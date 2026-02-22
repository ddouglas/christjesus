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
	"github.com/go-playground/form/v4"
	"github.com/gorilla/securecookie"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/sirupsen/logrus"
	supauth "github.com/supabase-community/auth-go"
)

//go:embed templates static
var uiFS embed.FS
var decoder = form.NewDecoder()

type Service struct {
	logger       *logrus.Logger
	config       *types.Config
	needsRepo    *store.NeedRepository
	progressRepo *store.NeedProgressRepository
	templates    *template.Template

	supauth supauth.Client
	cookie  *securecookie.SecureCookie

	jwksCache *jwk.Cache
	jwksURL   string

	server *http.Server
}

func New(
	config *types.Config,
	logger *logrus.Logger,
	supauth supauth.Client,
	needsRepo *store.NeedRepository,
	progressRepo *store.NeedProgressRepository,
	jwkCache *jwk.Cache,
	jwksURL string,
) (*Service, error) {
	mux := flow.New()

	hashKey, _ := base64.StdEncoding.DecodeString(config.CookieHashKey)
	blockKey, _ := base64.StdEncoding.DecodeString(config.CookieBlockKey)

	s := &Service{
		logger:  logger,
		config:  config,
		supauth: supauth,
		cookie:  securecookie.New(hashKey, blockKey),

		needsRepo:    needsRepo,
		progressRepo: progressRepo,

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
		r.HandleFunc("/onboarding/need/:needID/details", s.handleGetOnboardingNeedDetails, http.MethodGet)
		r.HandleFunc("/onboarding/need/:needID/details", s.handlePostOnboardingNeedDetails, http.MethodPost)
		r.HandleFunc("/onboarding/need/:needID/story", s.handleGetOnboardingNeedStory, http.MethodGet)
		r.HandleFunc("/onboarding/need/:needID/story", s.handlePostOnboardingNeedStory, http.MethodPost)
		r.HandleFunc("/onboarding/need/:needID/documents", s.handleGetOnboardingNeedDocuments, http.MethodGet)
		r.HandleFunc("/onboarding/need/:needID/documents", s.handlePostOnboardingNeedDocuments, http.MethodPost)
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
	funcMap := template.FuncMap{
		"div": func(a, b int64) int64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mul": func(a, b int64) int64 {
			return a * b
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
