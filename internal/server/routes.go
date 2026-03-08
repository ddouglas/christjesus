package server

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type RouteName string

const (
	RouteHome                   RouteName = "home"
	RouteRegister               RouteName = "register"
	RouteRegisterConfirm        RouteName = "register.confirm"
	RouteRegisterConfirmResend  RouteName = "register.confirm.resend"
	RouteLogin                  RouteName = "login"
	RouteLogout                 RouteName = "logout"
	RouteProfile                RouteName = "profile"
	RouteProfileNeedDelete      RouteName = "profile.need.delete"
	RouteProfileDonationReceipt RouteName = "profile.donation.receipt"

	RouteOnboarding                    RouteName = "onboarding"
	RouteOnboardingDonorWelcome        RouteName = "onboarding.donor.welcome"
	RouteOnboardingDonorPreferences    RouteName = "onboarding.donor.preferences"
	RouteOnboardingDonorConfirmation   RouteName = "onboarding.donor.confirmation"
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
	RouteLogout:                        "/logout",
	RouteProfile:                       "/profile",
	RouteProfileNeedDelete:             "/profile/needs/:needID/delete",
	RouteProfileDonationReceipt:        "/profile/donations/:intentID/receipt",
	RouteOnboarding:                    "/onboarding",
	RouteOnboardingDonorWelcome:        "/onboarding/donor/welcome",
	RouteOnboardingDonorPreferences:    "/onboarding/donor/preferences",
	RouteOnboardingDonorConfirmation:   "/onboarding/donor/confirmation",
	RouteOnboardingSponsorIndividual:   "/onboarding/sponsor/individual/welcome",
	RouteOnboardingSponsorOrganization: "/onboarding/sponsor/organization/welcome",
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
	RouteNeedDetail:                    "/need/:id",
	RouteNeedDonate:                    "/need/:id/donate",
	RouteNeedDonateConfirmation:        "/need/:id/donate/confirmation",
	RouteStripeWebhook:                 "/webhooks/stripe",
}

var routeTokenRE = regexp.MustCompile(`:([a-zA-Z0-9_]+)`)

func RoutePattern(name RouteName) string {
	if pattern, ok := routePatterns[name]; ok {
		return pattern
	}
	panic(fmt.Sprintf("unknown route pattern: %s", name))
}

func BuildRoute(name RouteName, params map[string]string) (string, error) {
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

func BuildRouteWithQuery(name RouteName, params map[string]string, query url.Values) (string, error) {
	path, err := BuildRoute(name, params)
	if err != nil {
		return "", err
	}
	if len(query) == 0 {
		return path, nil
	}
	return path + "?" + query.Encode(), nil
}

func (s *Service) route(name RouteName, params map[string]string) string {
	path, err := BuildRoute(name, params)
	if err == nil {
		return path
	}

	s.logger.WithError(err).WithField("route_name", name).Error("failed to build route")
	return RoutePattern(RouteHome)
}

func (s *Service) routeWithQuery(name RouteName, params map[string]string, query url.Values) string {
	path, err := BuildRouteWithQuery(name, params, query)
	if err == nil {
		return path
	}

	s.logger.WithError(err).WithField("route_name", name).Error("failed to build route with query")
	fallback, fbErr := BuildRouteWithQuery(RouteHome, nil, query)
	if fbErr == nil {
		return fallback
	}
	return RoutePattern(RouteHome)
}

func (s *Service) absoluteRoute(name RouteName, params map[string]string, query url.Values) string {
	path := s.routeWithQuery(name, params, query)
	baseURL := strings.TrimRight(strings.TrimSpace(s.config.AppBaseURL), "/")
	if baseURL == "" {
		return path
	}
	return baseURL + path
}
