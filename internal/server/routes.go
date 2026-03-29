package server

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type RouteName string

const (
	RouteHome                      RouteName = "home"
	RouteRegister                  RouteName = "register"
	RouteRegisterConfirm           RouteName = "register.confirm"
	RouteRegisterConfirmResend     RouteName = "register.confirm.resend"
	RouteLogin                     RouteName = "login"
	RouteAuthCallback              RouteName = "auth.callback"
	RouteLogout                    RouteName = "logout"
	RouteProfile                   RouteName = "profile"
	RouteAdmin                     RouteName = "admin.dashboard"
	RouteAdminNeeds                RouteName = "admin.needs"
	RouteAdminNeedExplorer         RouteName = "admin.need.explorer"
	RouteAdminNeedReview           RouteName = "admin.need.review"
	RouteAdminNeedModerate         RouteName = "admin.need.moderate"
	RouteAdminNeedDocument         RouteName = "admin.need.document"
	RouteAdminNeedDelete           RouteName = "admin.need.delete"
	RouteAdminNeedRestore          RouteName = "admin.need.restore"
	RouteAdminNeedMessage          RouteName = "admin.need.message"
	RouteAdminUsers                RouteName = "admin.users"
	RouteAdminUserDetail           RouteName = "admin.user.detail"
	RouteProfileNeedDelete         RouteName = "profile.need.delete"
	RouteProfileNeedReview         RouteName = "profile.need.review"
	RouteProfileNeedReviewPost     RouteName = "profile.need.review.post"
	RouteProfileNeedReviewSetReady RouteName = "profile.need.review.set.ready"
	RouteProfileNeedReviewPullBack RouteName = "profile.need.review.pull.back"
	RouteProfileNeedDocumentView   RouteName = "profile.need.document.view"
	RouteProfileNeedEdit           RouteName = "profile.need.edit"
	RouteProfileNeedEditLocation   RouteName = "profile.need.edit.location"
	RouteProfileNeedEditCategories RouteName = "profile.need.edit.categories"
	RouteProfileNeedEditStory      RouteName = "profile.need.edit.story"
	RouteProfileNeedEditDocs       RouteName = "profile.need.edit.documents"
	RouteProfileNeedEditUpload     RouteName = "profile.need.edit.documents.upload"
	RouteProfileNeedEditMeta       RouteName = "profile.need.edit.documents.meta"
	RouteProfileNeedEditDelete     RouteName = "profile.need.edit.documents.delete"
	RouteProfileNeedEditReview     RouteName = "profile.need.edit.review"
	RouteProfileDonationReceipt    RouteName = "profile.donation.receipt"
	RouteProfileUpdateName             RouteName = "profile.update.name"
	RouteProfileUpdateEmail            RouteName = "profile.update.email"
	RouteProfileSendPasswordReset      RouteName = "profile.send.password.reset"
	RouteProfileDonorPreferences       RouteName = "profile.donor.preferences"

	RouteOnboarding              RouteName = "onboarding"
	RouteOnboardingAboutYou      RouteName = "onboarding.about.you"
	RouteOnboardingHowWeServeYou RouteName = "onboarding.how.we.serve.you"

	RouteOnboardingDonorWelcome      RouteName = "onboarding.donor.welcome"
	RouteOnboardingDonorPreferences  RouteName = "onboarding.donor.preferences"
	RouteOnboardingDonorConfirmation RouteName = "onboarding.donor.confirmation"

	RouteOnboardingSponsorIndividual   RouteName = "onboarding.sponsor.individual.welcome"
	RouteOnboardingSponsorOrganization RouteName = "onboarding.sponsor.organization.welcome"

	RouteOnboardingNeedWelcome         RouteName = "onboarding.need.welcome"
	RouteOnboardingNeedLocation        RouteName = "onboarding.need.location"
	RouteOnboardingNeedCategories      RouteName = "onboarding.need.categories"
	RouteOnboardingNeedDetails         RouteName = "onboarding.need.details"
	RouteOnboardingNeedStory           RouteName = "onboarding.need.story"
	RouteOnboardingNeedDocuments       RouteName = "onboarding.need.documents"
	RouteOnboardingNeedDocumentsUpload RouteName = "onboarding.need.documents.upload"
	RouteOnboardingNeedDocumentsMeta   RouteName = "onboarding.need.documents.meta"
	RouteOnboardingNeedDocumentDelete  RouteName = "onboarding.need.documents.delete"
	RouteOnboardingNeedReview          RouteName = "onboarding.need.review"
	RouteOnboardingNeedConfirmation    RouteName = "onboarding.need.confirmation"

	RouteBrowse                 RouteName = "browse"
	RouteCategories             RouteName = "categories"
	RouteCategoryNeeds          RouteName = "category.needs"
	RouteMap                    RouteName = "map"
	RouteGuidelines             RouteName = "guidelines"
	RouteAbout                  RouteName = "about"
	RouteShare                  RouteName = "share"
	RouteTerms                  RouteName = "terms"
	RoutePrivacy                RouteName = "privacy"
	RouteNeedDetail             RouteName = "need.detail"
	RouteNeedDonate             RouteName = "need.donate"
	RouteNeedDonateConfirmation RouteName = "need.donate.confirmation"
	RouteStripeWebhook          RouteName = "stripe.webhook"
)

var routePatterns = map[RouteName]string{
	RouteHome:                          "/",
	RouteRegister:                      "/register",
	RouteRegisterConfirm:               "/register/confirm",
	RouteRegisterConfirmResend:         "/register/confirm/resend",
	RouteLogin:                         "/login",
	RouteAuthCallback:                  "/auth/callback",
	RouteLogout:                        "/logout",
	RouteProfile:                       "/profile",
	RouteAdmin:                         "/admin",
	RouteAdminNeeds:                    "/admin/needs",
	RouteAdminNeedExplorer:             "/admin/needs/explorer",
	RouteAdminNeedReview:               "/admin/needs/:needID",
	RouteAdminNeedModerate:             "/admin/needs/:needID/moderate",
	RouteAdminNeedDocument:             "/admin/needs/:needID/documents/:documentID",
	RouteAdminNeedDelete:               "/admin/needs/:needID/delete",
	RouteAdminNeedRestore:              "/admin/needs/:needID/restore",
	RouteAdminNeedMessage:              "/admin/needs/:needID/messages",
	RouteAdminUsers:                    "/admin/users",
	RouteAdminUserDetail:               "/admin/users/:userID",
	RouteProfileNeedDelete:             "/profile/needs/:needID/delete",
	RouteProfileNeedReview:             "/profile/needs/:needID/review",
	RouteProfileNeedReviewPost:         "/profile/needs/:needID/review/messages",
	RouteProfileNeedReviewSetReady:     "/profile/needs/:needID/review/set-ready",
	RouteProfileNeedReviewPullBack:     "/profile/needs/:needID/review/pull-back",
	RouteProfileNeedDocumentView:       "/profile/needs/:needID/documents/:documentID",
	RouteProfileNeedEdit:               "/profile/needs/:needID/edit",
	RouteProfileNeedEditLocation:       "/profile/needs/:needID/edit/location",
	RouteProfileNeedEditCategories:     "/profile/needs/:needID/edit/categories",
	RouteProfileNeedEditStory:          "/profile/needs/:needID/edit/story",
	RouteProfileNeedEditDocs:           "/profile/needs/:needID/edit/documents",
	RouteProfileNeedEditUpload:         "/profile/needs/:needID/edit/documents/upload",
	RouteProfileNeedEditMeta:           "/profile/needs/:needID/edit/documents/metadata",
	RouteProfileNeedEditDelete:         "/profile/needs/:needID/edit/documents/:documentID/delete",
	RouteProfileNeedEditReview:         "/profile/needs/:needID/edit/review",
	RouteProfileDonationReceipt:        "/profile/donations/:intentID/receipt",
	RouteProfileUpdateName:             "/profile/update/name",
	RouteProfileUpdateEmail:            "/profile/update/email",
	RouteProfileSendPasswordReset:      "/profile/send-password-reset",
	RouteProfileDonorPreferences:       "/profile/preferences",
	RouteOnboarding:                    "/onboarding",
	RouteOnboardingAboutYou:            "/onboarding/about-you",
	RouteOnboardingHowWeServeYou:       "/onboarding/how-we-serve-you",
	RouteOnboardingDonorWelcome:        "/onboarding/donor/welcome",
	RouteOnboardingDonorPreferences:    "/onboarding/donor/preferences",
	RouteOnboardingDonorConfirmation:   "/onboarding/donor/confirmation",
	RouteOnboardingNeedWelcome:         "/onboarding/need/:needID/welcome",
	RouteOnboardingNeedLocation:        "/onboarding/need/:needID/location",
	RouteOnboardingNeedCategories:      "/onboarding/need/:needID/categories",
	RouteOnboardingNeedDetails:         "/onboarding/need/:needID/details",
	RouteOnboardingNeedStory:           "/onboarding/need/:needID/story",
	RouteOnboardingNeedDocuments:       "/onboarding/need/:needID/documents",
	RouteOnboardingNeedDocumentsUpload: "/onboarding/need/:needID/documents/upload",
	RouteOnboardingNeedDocumentsMeta:   "/onboarding/need/:needID/documents/metadata",
	RouteOnboardingNeedDocumentDelete:  "/onboarding/need/:needID/documents/:documentID/delete",
	RouteOnboardingNeedReview:          "/onboarding/need/:needID/review",
	RouteOnboardingNeedConfirmation:    "/onboarding/need/:needID/confirmation",
	RouteBrowse:                        "/browse",
	RouteCategories:                    "/categories",
	RouteCategoryNeeds:                 "/category/:slug",
	RouteMap:                           "/map",
	RouteGuidelines:                    "/guidelines",
	RouteAbout:                         "/about",
	RouteShare:                         "/share",
	RouteTerms:                         "/terms",
	RoutePrivacy:                       "/privacy",
	RouteNeedDetail:                    "/need/:needID",
	RouteNeedDonate:                    "/need/:needID/donate",
	RouteNeedDonateConfirmation:        "/need/:needID/donate/confirmation",
	RouteStripeWebhook:                 "/webhooks/stripe",

	// RouteOnboardingSponsorIndividual:   "/onboarding/sponsor/individual/welcome",
	// RouteOnboardingSponsorOrganization: "/onboarding/sponsor/organization/welcome",
}

// RouteOption is a functional option for building a route path and query string.
type RouteOption func(*routeOptions)

type routeOptions struct {
	params map[string]string
	query  url.Values
}

// Param returns a RouteOption that sets a path parameter.
func Param(k, v string) RouteOption {
	return func(o *routeOptions) { o.params[k] = v }
}

// Query returns a RouteOption that adds a query string parameter.
func Query(k, v string) RouteOption {
	return func(o *routeOptions) { o.query.Set(k, v) }
}

var routeTokenRE = regexp.MustCompile(`:([a-zA-Z0-9_]+)`)

func RoutePattern(name RouteName) string {
	if pattern, ok := routePatterns[name]; ok {
		return pattern
	}
	panic(fmt.Sprintf("unknown route pattern: %s", name))
}

// buildRouteFromMap builds the URL path by substituting :param tokens from the given map.
func buildRouteFromMap(name RouteName, params map[string]string) (string, error) {
	pattern, ok := routePatterns[name]
	if !ok {
		return "", fmt.Errorf("unknown route name: %s", name)
	}

	missing := make([]string, 0)
	built := routeTokenRE.ReplaceAllStringFunc(pattern, func(token string) string {
		key := strings.TrimPrefix(token, ":")
		value, exists := params[key]
		if !exists {
			missing = append(missing, key)
			return token
		}

		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			missing = append(missing, key)
			return token
		}
		return url.PathEscape(trimmed)
	})

	if len(missing) > 0 {
		return "", fmt.Errorf("missing route params for %s: %s", name, strings.Join(missing, ", "))
	}

	return built, nil
}

// BuildRoute builds a URL path (with optional query string) for the named route.
func BuildRoute(name RouteName, opts ...RouteOption) (string, error) {
	o := &routeOptions{
		params: make(map[string]string),
		query:  make(url.Values),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	path, err := buildRouteFromMap(name, o.params)
	if err != nil {
		return "", err
	}

	if len(o.query) == 0 {
		return path, nil
	}
	return path + "?" + o.query.Encode(), nil
}

func (s *Service) route(name RouteName, opts ...RouteOption) string {
	path, err := BuildRoute(name, opts...)
	if err == nil {
		return path
	}

	s.logger.WithError(err).WithField("route_name", name).Error("failed to build route")
	return RoutePattern(RouteHome)
}

func (s *Service) routeWithQuery(name RouteName, query url.Values, opts ...RouteOption) string {
	o := &routeOptions{
		params: make(map[string]string),
		query:  make(url.Values),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	// Merge caller-provided query values on top of any Query() opts.
	for k, vs := range query {
		for _, v := range vs {
			o.query.Set(k, v)
		}
	}

	path, err := buildRouteFromMap(name, o.params)
	if err != nil {
		s.logger.WithError(err).WithField("route_name", name).Error("failed to build route with query")
		path = RoutePattern(RouteHome)
	}

	if len(o.query) == 0 {
		return path
	}
	return path + "?" + o.query.Encode()
}

func (s *Service) absoluteRoute(name RouteName, query url.Values, opts ...RouteOption) string {
	path := s.routeWithQuery(name, query, opts...)
	baseURL := strings.TrimRight(strings.TrimSpace(s.config.AppBaseURL), "/")
	if baseURL == "" {
		return path
	}
	return baseURL + path
}
