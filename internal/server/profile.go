package server

import (
	"christjesus/pkg/types"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (s *Service) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	userEmail, _ := ctx.Value(contextKeyEmail).(string)
	userName, _ := ctx.Value(contextKeyUserName).(string)
	if strings.TrimSpace(userName) == "" {
		userName = displayNameFromEmail(userEmail)
	}

	userType := ""
	user, err := s.userRepo.User(ctx, userID)
	if err != nil {
		if !errors.Is(err, types.ErrUserNotFound) {
			s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch user for profile")
			s.internalServerError(w)
			return
		}
	} else if user.UserType != nil {
		userType = strings.TrimSpace(*user.UserType)
	}

	myNeeds := make([]*types.Need, 0)
	needSummaries := make([]types.ProfileNeedSummary, 0)
	if userType == string(types.UserTypeNeed) {
		needs, err := s.needsRepo.NeedsByUser(ctx, userID)
		if err != nil {
			if !errors.Is(err, types.ErrNeedNotFound) {
				s.logger.WithError(err).WithField("user_id", userID).Error("failed to fetch needs for profile")
				s.internalServerError(w)
				return
			}
		} else {
			myNeeds = needs
			for _, need := range needs {
				primaryCategoryName := "Uncategorized"

				assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, need.ID)
				if err != nil {
					s.logger.WithError(err).WithField("need_id", need.ID).Error("failed to fetch need category assignments for profile")
					s.internalServerError(w)
					return
				}

				for _, assignment := range assignments {
					if !assignment.IsPrimary {
						continue
					}

					category, err := s.categoryRepo.CategoryByID(ctx, assignment.CategoryID)
					if err != nil {
						s.logger.WithError(err).WithField("category_id", assignment.CategoryID).Error("failed to fetch primary category for profile")
						s.internalServerError(w)
						return
					}

					if category != nil {
						primaryCategoryName = category.Name
					}

					break
				}

				needSummaries = append(needSummaries, types.ProfileNeedSummary{
					NeedID:              need.ID,
					PrimaryCategoryName: primaryCategoryName,
					RequestedAmount:     formatUSDFromCents(need.AmountNeededCents),
					CurrentStep:         formatNeedStepLabel(need.CurrentStep),
					Status:              need.Status,
					CanDelete:           need.Status == types.NeedStatusDraft,
				})
			}
		}
	}

	donatedNeeds := make([]*types.Need, 0)

	data := &types.ProfilePageData{
		BasePageData:   types.BasePageData{Title: "My Profile"},
		UserID:         userID,
		UserEmail:      userEmail,
		WelcomeName:    userName,
		UserType:       userType,
		Notice:         strings.TrimSpace(r.URL.Query().Get("notice")),
		Error:          strings.TrimSpace(r.URL.Query().Get("error")),
		SidebarItems:   buildProfileSidebar(userType),
		Needs:          myNeeds,
		NeedSummaries:  needSummaries,
		DonatedNeeds:   donatedNeeds,
		HasNeeds:       len(myNeeds) > 0,
		HasDonatedNeed: len(donatedNeeds) > 0,
	}

	err = s.renderTemplate(w, r, "page.profile", data)
	if err != nil {
		s.logger.WithError(err).Error("failed to render profile page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handlePostProfileNeedDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.WithError(err).Error("user id not found in context")
		s.internalServerError(w)
		return
	}

	needID := strings.TrimSpace(r.PathValue("needID"))
	if needID == "" {
		s.redirectProfileWithError(w, r, "Need not found.")
		return
	}

	need, err := s.needsRepo.Need(ctx, needID)
	if err != nil {
		if errors.Is(err, types.ErrNeedNotFound) {
			s.redirectProfileWithError(w, r, "Need not found.")
			return
		}
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to load need for profile delete")
		s.internalServerError(w)
		return
	}

	if need.UserID != userID {
		s.redirectProfileWithError(w, r, "You do not have permission to delete that need.")
		return
	}

	if need.Status != types.NeedStatusDraft {
		s.redirectProfileWithError(w, r, "Only draft needs can be deleted.")
		return
	}

	docs, err := s.documentRepo.DocumentsByNeedID(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to load need documents before profile delete")
		s.internalServerError(w)
		return
	}

	for _, doc := range docs {
		storageKey := strings.TrimSpace(doc.StorageKey)
		if storageKey == "" {
			continue
		}

		_, err = s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.config.S3BucketName),
			Key:    aws.String(storageKey),
		})
		if err != nil {
			s.logger.WithError(err).
				WithField("need_id", needID).
				WithField("document_id", doc.ID).
				WithField("storage_key", storageKey).
				Error("failed to delete need document from S3 during profile delete")
			s.redirectProfileWithError(w, r, "Could not delete uploaded files from storage. Please try again.")
			return
		}
	}

	err = s.needsRepo.DeleteNeed(ctx, needID)
	if err != nil {
		s.logger.WithError(err).WithField("need_id", needID).Error("failed to delete draft need from profile")
		s.internalServerError(w)
		return
	}

	s.redirectProfileWithNotice(w, r, "Draft need deleted.")
}

func (s *Service) redirectProfileWithNotice(w http.ResponseWriter, r *http.Request, notice string) {
	v := url.Values{}
	v.Set("notice", notice)
	http.Redirect(w, r, "/profile?"+v.Encode(), http.StatusSeeOther)
}

func (s *Service) redirectProfileWithError(w http.ResponseWriter, r *http.Request, msg string) {
	v := url.Values{}
	v.Set("error", msg)
	http.Redirect(w, r, "/profile?"+v.Encode(), http.StatusSeeOther)
}

func formatUSDFromCents(cents int) string {
	dollars := float64(cents) / 100.0
	return fmt.Sprintf("$%.2f", dollars)
}

func formatNeedStepLabel(step types.NeedStep) string {
	switch step {
	case types.NeedStepWelcome:
		return "Welcome"
	case types.NeedStepLocation:
		return "Location"
	case types.NeedStepCategories:
		return "Categories"
	case types.NeedStepStory:
		return "Need Story"
	case types.NeedStepDocuments:
		return "Documents"
	case types.NeedStepReview:
		return "Review"
	case types.NeedStepComplete:
		return "Complete"
	default:
		return "Unknown"
	}
}

func buildProfileSidebar(userType string) []types.ProfileNavItem {
	items := []types.ProfileNavItem{
		{Label: "Profile Overview", Href: "#overview", Active: true, Section: "overview", ShowItem: true},
		{Label: "My Needs", Href: "#my-needs", Active: false, Section: "my-needs", ShowItem: userType == string(types.UserTypeNeed)},
		{Label: "Need Status", Href: "#need-status", Active: false, Section: "need-status", ShowItem: userType == string(types.UserTypeNeed)},
		{Label: "Needs I've Donated To", Href: "#donations", Active: false, Section: "donations", ShowItem: userType == string(types.UserTypeDonor)},
	}

	filtered := make([]types.ProfileNavItem, 0, len(items))
	for _, item := range items {
		if item.ShowItem {
			filtered = append(filtered, item)
		}
	}

	return filtered
}
