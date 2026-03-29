package server

import (
	"christjesus/pkg/types"
	"context"
	"errors"
	"net/http"
	"strings"
)

func (s *Service) handleGetOnboarding(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFromRequest(r)
	if !ok {
		http.Redirect(w, r, s.route(RouteLogin), http.StatusSeeOther)
		return
	}

	if strings.TrimSpace(session.GivenName) == "" {
		http.Redirect(w, r, s.route(RouteOnboardingAboutYou), http.StatusSeeOther)
		return
	}

	if strings.TrimSpace(session.UserType) == "" {
		http.Redirect(w, r, s.route(RouteOnboardingHowWeServeYou), http.StatusSeeOther)
		return
	}

	ctx := r.Context()
	switch session.UserType {
	case string(types.UserTypeRecipient):
		s.redirectNeedOnboarding(ctx, w, r, session.UserID)
	case string(types.UserTypeDonor):
		http.Redirect(w, r, s.route(RouteOnboardingDonorPreferences), http.StatusSeeOther)
	default:
		http.Redirect(w, r, s.route(RouteOnboardingHowWeServeYou), http.StatusSeeOther)
	}
}

func (s *Service) handleGetOnboardingHowWeServeYou(w http.ResponseWriter, r *http.Request) {
	// ctx := r.Context()

	// userID, err := s.userIDFromContext(ctx)
	// if err != nil {
	// 	s.logger.WithError(err).Error("user id not found in context")
	// 	s.internalServerError(w)
	// 	return
	// }

	// user, err := s.userRepo.User(ctx, userID)
	// if err != nil {
	// 	if !errors.Is(err, types.ErrUserNotFound) {
	// 		s.logger.WithError(err).WithField("user_id", userID).Error("failed to load user from datastore")
	// 		s.internalServerError(w)
	// 		return
	// 	}
	// } else if user.UserType != nil {
	// 	switch *user.UserType {
	// 	case string(types.UserTypeRecipient):
	// 		s.redirectNeedOnboarding(ctx, w, r, userID)
	// 		return
	// 	case string(types.UserTypeDonor):
	// 		http.Redirect(w, r, s.route(RouteOnboardingDonorPreferences), http.StatusSeeOther)
	// 		return
	// 	}
	// }

	data := &types.OnboardingPageData{BasePageData: types.BasePageData{Title: "Onboarding"}}

	err := s.renderTemplate(w, r, "page.onboarding", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need welcome page")
		s.internalServerError(w)
		return
	}
}

type onboardingDirector struct {
	Path string `form:"path"`
}

func (s *Service) handlePostOnboardingHowWeServeYou(w http.ResponseWriter, r *http.Request) {

	var ctx = r.Context()

	err := r.ParseForm()
	if err != nil {
		s.logger.WithError(err).Error("failed to parse form")
		return
	}

	var onboarding = new(onboardingDirector)
	err = decoder.Decode(onboarding, r.Form)
	if err != nil {
		s.logger.WithError(err).Error("failed to decode form")
		s.internalServerError(w)
		return
	}

	switch onboarding.Path {
	case "recipient":
		userType := string(types.UserTypeRecipient)
		err = s.setUserType(ctx, userType)
		if err != nil {
			s.logger.WithError(err).Error("failed to set user type")
			s.internalServerError(w)
			return
		}
		s.updateAuthUserTypeCookie(w, r, userType)
		s.handleCreateNeed(ctx, w, r)
		return
	case "donor":
		userType := string(types.UserTypeDonor)
		err = s.setUserType(ctx, userType)
		if err != nil {
			s.logger.WithError(err).Error("failed to set user type")
			s.internalServerError(w)
			return
		}
		s.updateAuthUserTypeCookie(w, r, userType)
		http.Redirect(w, r, s.route(RouteOnboardingDonorWelcome), http.StatusSeeOther)
		return
	}

	data := &types.OnboardingPageData{BasePageData: types.BasePageData{Title: "Onboarding"}}

	err = s.renderTemplate(w, r, "page.onboarding", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need welcome page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleCreateNeed(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("ctx doesn't contain user")
		s.internalServerError(w)
		return
	}

	need := &types.Need{
		UserID:      userID,
		Status:      types.NeedStatusDraft,
		CurrentStep: types.NeedStepWelcome,
	}

	err = s.needsRepo.CreateNeed(ctx, need)
	if err != nil {
		s.logger.WithError(err).Error("failed to create need in datastore")
		s.internalServerError(w)
		return
	}

	http.Redirect(w, r, s.route(RouteOnboardingNeedWelcome, Param("needID", need.ID)), http.StatusSeeOther)
}

func (s *Service) redirectNeedOnboarding(ctx context.Context, w http.ResponseWriter, r *http.Request, userID string) {
	need, err := s.needsRepo.DraftNeedsByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			s.handleCreateNeed(ctx, w, r)
			return
		}

		s.logger.WithError(err).WithField("user_id", userID).Error("failed to load draft need")
		s.internalServerError(w)
		return
	}

	if need == nil {
		s.handleCreateNeed(ctx, w, r)
		return
	}

	// Once a need leaves draft/onboarding workflow, send users to the need review portal.
	if need.Status != types.NeedStatusDraft {
		http.Redirect(w, r, s.route(RouteProfileNeedReview, Param("needID", need.ID)), http.StatusSeeOther)
		return
	}

	nextRouteByStep := map[types.NeedStep]RouteName{
		types.NeedStepWelcome:    RouteOnboardingNeedWelcome,
		types.NeedStepLocation:   RouteOnboardingNeedLocation,
		types.NeedStepCategories: RouteOnboardingNeedCategories,
		types.NeedStepStory:      RouteOnboardingNeedStory,
		types.NeedStepDocuments:  RouteOnboardingNeedDocuments,
		types.NeedStepReview:     RouteOnboardingNeedReview,
		types.NeedStepComplete:   RouteOnboardingNeedConfirmation,
	}
	nextRoute, ok := nextRouteByStep[need.CurrentStep]
	if !ok {
		nextRoute = RouteOnboardingNeedWelcome
	}
	http.Redirect(w, r, s.route(nextRoute, Param("needID", need.ID)), http.StatusSeeOther)
}

func (s *Service) setUserType(ctx context.Context, userType string) error {
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return err
	}

	existingUser, err := s.userRepo.User(ctx, userID)
	if err != nil {
		if !errors.Is(err, types.ErrUserNotFound) {
			return err
		}

		newUser := &types.User{
			ID:       userID,
			UserType: &userType,
		}
		return s.userRepo.Create(ctx, newUser)
	}

	existingUser.UserType = &userType
	return s.userRepo.Update(ctx, userID, existingUser)
}

func (s *Service) redirectIfNeedSubmitted(w http.ResponseWriter, r *http.Request, need *types.Need) bool {
	if need == nil {
		return false
	}

	if need.Status != types.NeedStatusDraft {
		http.Redirect(w, r, s.route(RouteProfileNeedReview, Param("needID", need.ID)), http.StatusSeeOther)
		return true
	}

	return false
}

func (s *Service) handleGetOnboardingNeedWelcome(w http.ResponseWriter, r *http.Request) {
	var ctx = r.Context()

	needID := r.PathValue("needID")

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	data := &types.NeedWelcomePageData{
		BasePageData: types.BasePageData{Title: "Need Onboarding"},
		Need:         need,
	}

	err = s.renderTemplate(w, r, "page.onboarding.need.welcome", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render need welcome page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostOnboardingNeedWelcome(w http.ResponseWriter, r *http.Request) {
	var ctx = r.Context()

	needID := r.PathValue("needID")
	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch need from datastore")
		s.internalServerError(w)
		return
	}

	if s.redirectIfNeedSubmitted(w, r, need) {
		return
	}

	s.recordNeedProgress(ctx, need.ID, types.NeedStepWelcome)
	http.Redirect(w, r, s.route(RouteOnboardingNeedLocation, Param("needID", need.ID)), http.StatusSeeOther)

}

func (s *Service) recordNeedProgress(ctx context.Context, needID string, step types.NeedStep) {
	err := s.progressRepo.RecordStepCompletion(ctx, needID, step)
	if err != nil {
		s.logger.WithError(err).
			WithField("need_id", needID).
			WithField("step", step).
			Warn("failed to record progress event")
	}
}
