package server

import (
	"net/http"
	"net/url"
	"strings"

	"christjesus/pkg/types"
)

type HomePageData struct {
	Title        string
	Notice       string
	Error        string
	FeaturedNeed *types.Need
	Needs        []*types.Need
	Categories   []types.CategoryData
	Stats        types.StatsData
	Steps        []types.StepData
}

type BrowsePageData struct {
	Title string
	Needs []*types.Need
}

type NeedDetailPageData struct {
	Title string
	Need  *types.Need
}

func (s *Service) handleHome(w http.ResponseWriter, r *http.Request) {
	var _ = r.Context()

	needs := sampleNeeds()
	
	data := HomePageData{
		Title:        "",
		Notice:       r.URL.Query().Get("notice"),
		Error:        r.URL.Query().Get("error"),
		FeaturedNeed: needs[0], // First need is featured
		Needs:        needs[1:], // Rest are in the grid
		Categories:   sampleCategories(),
		Stats:        getStats(),
		Steps:        getSteps(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "page.home", data); err != nil {
		s.logger.WithError(err).Error("failed to render home page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Service) redirectWithNotice(w http.ResponseWriter, r *http.Request, notice string) {
	v := url.Values{}
	v.Set("notice", notice)
	http.Redirect(w, r, "/?"+v.Encode(), http.StatusSeeOther)
}

func (s *Service) redirectWithError(w http.ResponseWriter, r *http.Request, msg string) {
	v := url.Values{}
	v.Set("error", msg)
	http.Redirect(w, r, "/?"+v.Encode(), http.StatusSeeOther)
}

func required(v string) bool {
	return strings.TrimSpace(v) != ""
}

func (s *Service) internalServerError(w http.ResponseWriter) {
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (s *Service) handleBrowse(w http.ResponseWriter, r *http.Request) {
	data := BrowsePageData{
		Title: "Browse Needs",
		Needs: sampleNeeds(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "page.browse", data); err != nil {
		s.logger.WithError(err).Error("failed to render browse page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleNeedDetail(w http.ResponseWriter, r *http.Request) {
	needID := r.PathValue("id")

	var need *types.Need
	for _, n := range sampleNeeds() {
		if n.ID == needID {
			need = n
			break
		}
	}

	if need == nil {
		http.NotFound(w, r)
		return
	}

	data := NeedDetailPageData{
		Title: need.Name + " - Need Details",
		Need:  need,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "page.need-detail", data); err != nil {
		s.logger.WithError(err).Error("failed to render need detail page")
		s.internalServerError(w)
		return
	}
}

func sampleNeeds() []*types.Need {
	return []*types.Need{
		{
			ID:           "need-001",
			Name:         "Maria",
			City:         "Charlotte, NC",
			Neighborhood: "Uptown Charlotte",
			Category:     "Health Condition",
			Urgency:      "high",
			Verified:     true,
			Verification: "gold",
			Description:  "Recovering from surgery and facing mounting bills while searching for stable work",
			RaisedCents:  214000,
			GoalCents:    320000,
			DonorsCount:  24,
		},
		{
			ID:           "need-002",
			Name:         "James T.",
			City:         "Charlotte, NC",
			Neighborhood: "West Charlotte",
			Category:     "Food",
			Urgency:      "medium",
			Verified:     true,
			Verification: "silver",
			Description:  "Elderly veteran needs groceries and meal support for three weeks while recovering from surgery",
			RaisedCents:  45000,
			GoalCents:    75000,
			DonorsCount:  12,
		},
		{
			ID:           "need-003",
			Name:         "Sarah K.",
			City:         "Charlotte, NC",
			Neighborhood: "South Charlotte",
			Category:     "Medical",
			Urgency:      "high",
			Verified:     true,
			Verification: "gold",
			Description:  "Medical bills assistance needed for unexpected emergency room visit and follow-up care",
			RaisedCents:  80000,
			GoalCents:    100600,
			DonorsCount:  18,
		},
		{
			ID:           "need-004",
			Name:         "David M.",
			City:         "Charlotte, NC",
			Neighborhood: "North Charlotte",
			Category:     "Utility & Basic Needs",
			Urgency:      "medium",
			Verified:     true,
			Verification: "bronze",
			Description:  "Family needs help with electricity bills and winter heating costs after job transition",
			RaisedCents:  35000,
			GoalCents:    60000,
			DonorsCount:  8,
		},
		{
			ID:           "need-005",
			Name:         "Lisa R.",
			City:         "Charlotte, NC",
			Neighborhood: "University Area",
			Category:     "Family & Children",
			Urgency:      "high",
			Verified:     true,
			Verification: "silver",
			Description:  "Single parent needs childcare assistance to maintain part-time employment",
			RaisedCents:  80000,
			GoalCents:    100000,
			DonorsCount:  15,
		},
	}
}

func sampleCategories() []types.CategoryData {
	return []types.CategoryData{
		{Name: "Unhoused", Slug: "unhoused", Count: 18, Icon: "home"},
		{Name: "Unbanked", Slug: "unbanked", Count: 9, Icon: "wallet"},
		{Name: "Malnourished", Slug: "malnourished", Count: 12, Icon: "utensils"},
		{Name: "Health Condition", Slug: "health-condition", Count: 15, Icon: "heart-pulse"},
		{Name: "Unemployment", Slug: "unemployment", Count: 22, Icon: "briefcase"},
		{Name: "Utility & Basic Needs", Slug: "utility-basic-needs", Count: 14, Icon: "lightbulb"},
		{Name: "Family & Children", Slug: "family-children", Count: 11, Icon: "users"},
		{Name: "Legal Documentation", Slug: "legal-documentation", Count: 7, Icon: "file-text"},
	}
}

func getStats() types.StatsData {
	return types.StatsData{
		TotalRaised:  7824000, // $78,240
		NeedsFunded:  214,
		LivesChanged: 389,
	}
}

func getSteps() []types.StepData {
	return []types.StepData{
		{
			Number:      1,
			Title:       "Share your verified need",
			Description: "Complete a simple form and connect with our verification team to share your story.",
		},
		{
			Number:      2,
			Title:       "Connect with sponsors & organizations",
			Description: "We match your need with caring individuals and local organizations ready to help.",
		},
		{
			Number:      3,
			Title:       "Receive support & transform",
			Description: "Get the assistance you need and join our community of hope and transformation.",
		},
	}
}
