package server

import (
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"christjesus/internal/store"
	"christjesus/pkg/types"

	"github.com/alexedwards/flow"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-playground/form/v4"
	"github.com/gorilla/securecookie"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/sirupsen/logrus"
)

//go:embed templates static
var uiFS embed.FS
var decoder = form.NewDecoder()

type Service struct {
	config *types.Config
	logger *logrus.Logger

	cognitoClient *cognitoidentityprovider.Client
	s3Client      *s3.Client

	needsRepo                   *store.NeedRepository
	progressRepo                *store.NeedProgressRepository
	categoryRepo                *store.CategoryRepository
	needCategoryAssignmentsRepo *store.AssignmentRepository
	storyRepo                   *store.StoryRepository
	documentRepo                *store.DocumentRepository
	userAddressRepo             *store.UserAddressRepository

	cookie    *securecookie.SecureCookie
	jwksCache *jwk.Cache
	jwksURL   string

	server    *http.Server
	templates *template.Template
}

func New(
	config *types.Config,
	logger *logrus.Logger,

	cognitoClient *cognitoidentityprovider.Client,
	s3Client *s3.Client,

	needsRepo *store.NeedRepository,
	progressRepo *store.NeedProgressRepository,
	categoryRepo *store.CategoryRepository,
	needCategoryAssignmentsRepo *store.AssignmentRepository,
	storyRepo *store.StoryRepository,
	documentRepo *store.DocumentRepository,
	userAddressRepo *store.UserAddressRepository,

	jwkCache *jwk.Cache,
	jwksURL string,
) (*Service, error) {
	mux := flow.New()

	hashKey, _ := base64.StdEncoding.DecodeString(config.CookieHashKey)
	blockKey, _ := base64.StdEncoding.DecodeString(config.CookieBlockKey)

	s := &Service{
		config: config,
		logger: logger,

		cognitoClient: cognitoClient,
		s3Client:      s3Client,

		needsRepo:                   needsRepo,
		progressRepo:                progressRepo,
		storyRepo:                   storyRepo,
		categoryRepo:                categoryRepo,
		needCategoryAssignmentsRepo: needCategoryAssignmentsRepo,
		documentRepo:                documentRepo,
		userAddressRepo:             userAddressRepo,

		cookie:    securecookie.New(hashKey, blockKey),
		jwksCache: jwkCache,
		jwksURL:   jwksURL,

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

	r.HandleFunc("/", s.handleHome, http.MethodGet)

	r.HandleFunc("/register", s.handleGetRegister, http.MethodGet)
	r.HandleFunc("/register", s.handlePostRegister, http.MethodPost)
	r.HandleFunc("/register/confirm", s.handleGetRegisterConfirm, http.MethodGet)
	r.HandleFunc("/register/confirm", s.handlePostRegisterConfirm, http.MethodPost)
	r.HandleFunc("/login", s.handleGetLogin, http.MethodGet)
	r.HandleFunc("/login", s.handlePostLogin, http.MethodPost)

	r.Group(func(r *flow.Mux) {
		r.Use(s.RequireAuth)

		r.HandleFunc("/onboarding", s.handleGetOnboarding, http.MethodGet)
		r.HandleFunc("/onboarding", s.handlePostOnboarding, http.MethodPost)

		r.HandleFunc("/onboarding/need/:needID/welcome", s.handleGetOnboardingNeedWelcome, http.MethodGet)
		r.HandleFunc("/onboarding/need/:needID/welcome", s.handlePostOnboardingNeedWelcome, http.MethodPost)
		r.HandleFunc("/onboarding/need/:needID/location", s.handleGetOnboardingNeedLocation, http.MethodGet)
		r.HandleFunc("/onboarding/need/:needID/location", s.handlePostOnboardingNeedLocation, http.MethodPost)
		r.HandleFunc("/onboarding/need/:needID/categories", s.handleGetOnboardingNeedCategories, http.MethodGet)
		r.HandleFunc("/onboarding/need/:needID/categories", s.handlePostOnboardingNeedCategories, http.MethodPost)
		r.HandleFunc("/onboarding/need/:needID/story", s.handleGetOnboardingNeedStory, http.MethodGet)
		r.HandleFunc("/onboarding/need/:needID/story", s.handlePostOnboardingNeedStory, http.MethodPost)
		r.HandleFunc("/onboarding/need/:needID/documents", s.handleGetOnboardingNeedDocuments, http.MethodGet)
		r.HandleFunc("/onboarding/need/:needID/documents", s.handlePostOnboardingNeedDocuments, http.MethodPost)
		r.HandleFunc("/onboarding/need/:needID/documents/upload", s.handlePostOnboardingNeedDocumentsUpload, http.MethodPost)
		r.HandleFunc("/onboarding/need/:needID/documents/metadata", s.handlePostOnboardingNeedDocumentMetadata, http.MethodPost)
		r.HandleFunc("/onboarding/need/:needID/documents/:documentID/delete", s.handlePostOnboardingNeedDocumentDelete, http.MethodPost)
		r.HandleFunc("/onboarding/need/:needID/review", s.handleGetOnboardingNeedReview, http.MethodGet)
		r.HandleFunc("/onboarding/need/:needID/review", s.handlePostOnboardingNeedReview, http.MethodPost)
		r.HandleFunc("/onboarding/need/:needID/confirmation", s.handleGetOnboardingNeedConfirmation, http.MethodGet)

		// Donor onboarding flow
		r.HandleFunc("/onboarding/donor/welcome", s.handleGetOnboardingDonorWelcome, http.MethodGet)
		r.HandleFunc("/onboarding/donor/preferences", s.handleGetOnboardingDonorPreferences, http.MethodGet)
		r.HandleFunc("/onboarding/donor/preferences", s.handlePostOnboardingDonorPreferences, http.MethodPost)
	})

	// Sponsor onboarding flow
	// r.HandleFunc("/register/sponsor", s.handleRegisterSponsor, http.MethodGet)
	// r.HandleFunc("/onboarding/sponsor/individual/welcome", s.handleGetOnboardingSponsorIndividualWelcome, http.MethodGet)
	// r.HandleFunc("/onboarding/sponsor/organization/welcome", s.handleGetOnboardingSponsorOrganizationWelcome, http.MethodGet)

	r.HandleFunc("/browse", s.handleBrowse, http.MethodGet)
	r.HandleFunc("/need/:id", s.handleNeedDetail, http.MethodGet)
	// r.HandleFunc("/forms/prayer", s.handlePrayerRequestSubmit, http.MethodPost)
	// r.HandleFunc("/forms/signup", s.handleEmailSignupSubmit, http.MethodPost)
	// r.HandleFunc("/healthz", s.handleHealth, http.MethodGet)

	staticRoot, err := fs.Sub(uiFS, "static")
	if err != nil {
		s.logger.WithError(err).Fatal("failed to mount static assets")
	}
	r.Handle("/static/...", http.StripPrefix("/static/", http.FileServer(http.FS(staticRoot))), http.MethodGet)
}

func loadTemplates() (*template.Template, error) {
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

	funcMap := template.FuncMap{
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
	}

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

func (s *Service) userIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(contextKeyUserID).(string)
	if !ok {
		return "", fmt.Errorf("user id not found in context")
	}
	return userID, nil
}
