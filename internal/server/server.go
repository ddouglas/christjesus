package server

import (
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"christjesus/internal/store"
	"christjesus/pkg/types"

	"github.com/alexedwards/flow"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-playground/form/v4"
	"github.com/gorilla/securecookie"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v84"
)

//go:embed templates static
var uiFS embed.FS
var decoder = form.NewDecoder()

const authOutboundTimeout = 10 * time.Second

type Service struct {
	config *types.Config
	logger *logrus.Logger

	s3Client     *s3.Client
	stripeClient *stripe.Client

	needsRepo                   *store.NeedRepository
	progressRepo                *store.NeedProgressRepository
	categoryRepo                *store.CategoryRepository
	needCategoryAssignmentsRepo *store.AssignmentRepository
	storyRepo                   *store.StoryRepository
	documentRepo                *store.DocumentRepository
	userAddressRepo             *store.UserAddressRepository
	userRepo                    *store.UserRepository
	donorPreferenceRepo         *store.DonorPreferenceRepository
	donorPreferenceAssignRepo   *store.DonorPreferenceAssignmentRepository
	donationIntentRepo          *store.DonationIntentRepository

	cookie     *securecookie.SecureCookie
	jwksCache  *jwk.Cache
	jwksURL    string
	httpClient *http.Client

	server    *http.Server
	templates *template.Template
}

func New(
	config *types.Config,
	logger *logrus.Logger,

	s3Client *s3.Client,
	stripeClient *stripe.Client,

	needsRepo *store.NeedRepository,
	progressRepo *store.NeedProgressRepository,
	categoryRepo *store.CategoryRepository,
	needCategoryAssignmentsRepo *store.AssignmentRepository,
	storyRepo *store.StoryRepository,
	documentRepo *store.DocumentRepository,
	userAddressRepo *store.UserAddressRepository,
	userRepo *store.UserRepository,
	donorPreferenceRepo *store.DonorPreferenceRepository,
	donorPreferenceAssignRepo *store.DonorPreferenceAssignmentRepository,
	donationIntentRepo *store.DonationIntentRepository,

	jwkCache *jwk.Cache,
	jwksURL string,
) (*Service, error) {
	mux := flow.New()

	hashKey, _ := base64.StdEncoding.DecodeString(config.CookieHashKey)
	blockKey, _ := base64.StdEncoding.DecodeString(config.CookieBlockKey)

	s := &Service{
		config: config,
		logger: logger,

		s3Client:     s3Client,
		stripeClient: stripeClient,

		needsRepo:                   needsRepo,
		progressRepo:                progressRepo,
		storyRepo:                   storyRepo,
		categoryRepo:                categoryRepo,
		needCategoryAssignmentsRepo: needCategoryAssignmentsRepo,
		documentRepo:                documentRepo,
		userAddressRepo:             userAddressRepo,
		userRepo:                    userRepo,
		donorPreferenceRepo:         donorPreferenceRepo,
		donorPreferenceAssignRepo:   donorPreferenceAssignRepo,
		donationIntentRepo:          donationIntentRepo,

		cookie:     securecookie.New(hashKey, blockKey),
		jwksCache:  jwkCache,
		jwksURL:    jwksURL,
		httpClient: &http.Client{Timeout: authOutboundTimeout},

		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", config.ServerPort),
			Handler:           mux,
			ReadTimeout:       time.Duration(config.ReadTimeoutSec) * time.Second,
			ReadHeaderTimeout: time.Duration(config.ReadTimeoutSec) * time.Second,
			WriteTimeout:      time.Duration(config.WriteTimeoutSec) * time.Second,
			MaxHeaderBytes:    1 << 20,
		},
	}

	templates, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	s.templates = templates

	s.buildRouter(mux)

	return s, nil
}

func (s *Service) Start() error {
	return s.server.ListenAndServe()
}

func (s *Service) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Service) buildRouter(r *flow.Mux) {
	r.Use(s.StripTrailingSlash)
	r.Use(s.LoggingMiddleware)
	r.Use(s.AttachAuthContext)

	r.HandleFunc(RoutePattern(RouteHome), s.handleHome, http.MethodGet)

	r.HandleFunc(RoutePattern(RouteRegister), s.handleGetRegister, http.MethodGet)
	r.HandleFunc(RoutePattern(RouteRegister), s.handlePostRegister, http.MethodPost)
	r.HandleFunc(RoutePattern(RouteRegisterConfirm), s.handleGetRegisterConfirm, http.MethodGet)
	r.HandleFunc(RoutePattern(RouteRegisterConfirm), s.handlePostRegisterConfirm, http.MethodPost)
	r.HandleFunc(RoutePattern(RouteRegisterConfirmResend), s.handlePostRegisterConfirmResend, http.MethodPost)
	r.HandleFunc(RoutePattern(RouteLogin), s.handleGetLogin, http.MethodGet)
	r.HandleFunc(RoutePattern(RouteLogin), s.handlePostLogin, http.MethodPost)
	r.HandleFunc(RoutePattern(RouteAuthCallback), s.handleGetAuthCallback, http.MethodGet)
	r.HandleFunc(RoutePattern(RouteLogout), s.handlePostLogout, http.MethodGet)
	r.HandleFunc(RoutePattern(RouteLogout), s.handlePostLogout, http.MethodPost)

	r.Group(func(r *flow.Mux) {
		r.Use(s.RequireAuth)

		r.HandleFunc(RoutePattern(RouteProfile), s.handleGetProfile, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteProfileNeedDelete), s.handlePostProfileNeedDelete, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteProfileDonationReceipt), s.handleGetProfileDonationReceipt, http.MethodGet)

		r.HandleFunc(RoutePattern(RouteOnboarding), s.handleGetOnboarding, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteOnboarding), s.handlePostOnboarding, http.MethodPost)

		r.HandleFunc(RoutePattern(RouteOnboardingNeedWelcome), s.handleGetOnboardingNeedWelcome, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedWelcome), s.handlePostOnboardingNeedWelcome, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedLocation), s.handleGetOnboardingNeedLocation, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedLocation), s.handlePostOnboardingNeedLocation, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedCategories), s.handleGetOnboardingNeedCategories, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedCategories), s.handlePostOnboardingNeedCategories, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedStory), s.handleGetOnboardingNeedStory, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedStory), s.handlePostOnboardingNeedStory, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedDocuments), s.handleGetOnboardingNeedDocuments, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedDocuments), s.handlePostOnboardingNeedDocuments, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedDocumentsUpload), s.handlePostOnboardingNeedDocumentsUpload, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedDocumentsMeta), s.handlePostOnboardingNeedDocumentMetadata, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedDocumentDelete), s.handlePostOnboardingNeedDocumentDelete, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedReview), s.handleGetOnboardingNeedReview, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedReview), s.handlePostOnboardingNeedReview, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteOnboardingNeedConfirmation), s.handleGetOnboardingNeedConfirmation, http.MethodGet)

		// Donor onboarding flow
		r.HandleFunc(RoutePattern(RouteOnboardingDonorWelcome), s.handleGetOnboardingDonorWelcome, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteOnboardingDonorPreferences), s.handleGetOnboardingDonorPreferences, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteOnboardingDonorPreferences), s.handlePostOnboardingDonorPreferences, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteOnboardingDonorConfirmation), s.handleGetOnboardingDonorConfirmation, http.MethodGet)

		r.HandleFunc(RoutePattern(RouteNeedDonate), s.handleGetNeedDonate, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteNeedDonate), s.handlePostNeedDonate, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteNeedDonateConfirmation), s.handleGetNeedDonateConfirmation, http.MethodGet)
	})

	r.Group(func(r *flow.Mux) {
		r.Use(s.RequireAdmin)

		r.HandleFunc(RoutePattern(RouteAdmin), s.handleGetAdminDashboard, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteAdminNeeds), s.handleGetAdminNeeds, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteAdminNeedReview), s.handleGetAdminNeedReview, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteAdminNeedModerate), s.handlePostAdminNeedModerate, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteAdminNeedDocument), s.handleGetAdminNeedDocument, http.MethodGet)
		r.HandleFunc(RoutePattern(RouteAdminNeedDelete), s.handlePostAdminNeedDelete, http.MethodPost)
		r.HandleFunc(RoutePattern(RouteAdminNeedRestore), s.handlePostAdminNeedRestore, http.MethodPost)
	})

	r.HandleFunc(RoutePattern(RouteBrowse), s.handleBrowse, http.MethodGet)
	r.HandleFunc(RoutePattern(RouteCategories), s.handleCategories, http.MethodGet)
	r.HandleFunc(RoutePattern(RouteCategoryNeeds), s.handleCategoryNeeds, http.MethodGet)
	r.HandleFunc(RoutePattern(RouteNeedDetail), s.handleNeedDetail, http.MethodGet)
	r.HandleFunc(RoutePattern(RouteStripeWebhook), s.handlePostStripeWebhook, http.MethodPost)

	staticRoot, err := fs.Sub(uiFS, "static")
	if err != nil {
		s.logger.WithError(err).Fatal("failed to mount static assets")
	}
	r.Handle("/static/...", http.StripPrefix("/static/", http.FileServer(http.FS(staticRoot))), http.MethodGet)
}

func loadTemplates() (*template.Template, error) {
	funcMap := templateFuncMap()

	t := template.New("").Funcs(funcMap)
	err := fs.WalkDir(uiFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}

		data, err := fs.ReadFile(uiFS, path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", path, err)
		}

		if _, err := t.Parse(string(data)); err != nil {
			return fmt.Errorf("parse template %s: %w", path, err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return t, nil
}

func templateFuncMap() template.FuncMap {
	toInt64 := func(value any) int64 {
		switch v := value.(type) {
		case int:
			return int64(v)
		case int8:
			return int64(v)
		case int16:
			return int64(v)
		case int32:
			return int64(v)
		case int64:
			return v
		case uint:
			return int64(v)
		case uint8:
			return int64(v)
		case uint16:
			return int64(v)
		case uint32:
			return int64(v)
		case uint64:
			if v > ^uint64(0)>>1 {
				return 0
			}
			return int64(v)
		default:
			return 0
		}
	}

	return template.FuncMap{
		"div": func(a, b any) int64 {
			a64 := toInt64(a)
			b64 := toInt64(b)
			if b64 == 0 {
				return 0
			}
			return a64 / b64
		},
		"mul": func(a, b any) int64 {
			return toInt64(a) * toInt64(b)
		},
		"deref": func(s *string) string {
			if s == nil {
				return ""
			}
			return *s
		},
		"derefOr": func(s *string, defaultVal string) string {
			if s == nil {
				return defaultVal
			}
			return *s
		},
		"hasKey": func(values map[string]bool, key string) bool {
			if values == nil {
				return false
			}
			return values[key]
		},
		"route": func(name string, params map[string]string) (string, error) {
			trimmedName := strings.TrimSpace(name)
			path, err := BuildRoute(RouteName(trimmedName), params)
			if err != nil {
				return "", fmt.Errorf("template route(%q) failed: %w", trimmedName, err)
			}
			return path, nil
		},
		"routeq": func(name string, params map[string]string, query map[string]string) (string, error) {
			trimmedName := strings.TrimSpace(name)
			values := url.Values{}
			for key, value := range query {
				trimmedKey := strings.TrimSpace(key)
				if trimmedKey == "" {
					continue
				}
				values.Set(trimmedKey, value)
			}

			path, err := BuildRouteWithQuery(RouteName(trimmedName), params, values)
			if err != nil {
				return "", fmt.Errorf("template routeq(%q) failed: %w", trimmedName, err)
			}
			return path, nil
		},
		"dict": func(values ...string) (map[string]string, error) {
			result := make(map[string]string)
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict expects even number of arguments, got %d", len(values))
			}

			for i := 0; i < len(values); i += 2 {
				key := strings.TrimSpace(values[i])
				if key == "" {
					continue
				}
				result[key] = values[i+1]
			}

			return result, nil
		},
	}
}

func (s *Service) userIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(contextKeyUserID).(string)
	if !ok {
		return "", fmt.Errorf("user id not found in context")
	}
	return userID, nil
}
