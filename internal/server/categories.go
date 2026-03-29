package server

import (
	"christjesus/pkg/types"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (s *Service) handleCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	categories, err := s.categoryRepo.Categories(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch categories for categories page")
		s.internalServerError(w)
		return
	}

	categoryIDs := make([]string, 0, len(categories))
	for _, category := range categories {
		if category == nil || strings.TrimSpace(category.ID) == "" {
			continue
		}
		categoryIDs = append(categoryIDs, category.ID)
	}

	countsByCategoryID, err := s.needCategoryAssignmentsRepo.PrimaryNeedCountsByCategoryIDs(ctx, categoryIDs)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch category need counts")
		s.internalServerError(w)
		return
	}

	city := strings.TrimSpace(r.URL.Query().Get("city"))
	cityQuery := ""
	if city != "" {
		cityQuery = "?city=" + url.QueryEscape(city)
	}

	items := make([]*types.CategoryListItem, 0, len(categories))
	for _, category := range categories {
		if category == nil {
			continue
		}

		slug := normalizedCategorySlug(category)
		if slug == "" {
			s.logger.WithField("category_id", category.ID).Warn("skipping category with empty slug")
			continue
		}

		items = append(items, &types.CategoryListItem{
			ID:          category.ID,
			Name:        category.Name,
			Slug:        slug,
			Description: category.Description,
			Icon:        category.Icon,
			NeedCount:   countsByCategoryID[category.ID],
			Href:        s.route(RouteCategoryNeeds, Param("slug", slug)) + cityQuery,
		})
	}

	browseHref := s.route(RouteBrowse)
	if cityQuery != "" {
		browseHref += cityQuery
	}

	data := &types.CategoriesPageData{
		BasePageData: types.BasePageData{Title: "Categories"},
		Categories:   items,
		BrowseHref:   browseHref,
	}

	if err := s.renderTemplate(w, r, "page.categories", data); err != nil {
		s.logger.WithError(err).Error("failed to render categories page")
		s.internalServerError(w)
		return
	}
}

func (s *Service) handleCategoryNeeds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := strings.TrimSpace(r.PathValue("slug"))
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	category, err := s.categoryRepo.CategoryBySlug(ctx, slug)
	if err != nil {
		s.logger.WithError(err).WithField("slug", slug).Error("failed to fetch category by slug")
		s.internalServerError(w)
		return
	}
	if category == nil {
		http.NotFound(w, r)
		return
	}

	filters := parseBrowseFilters(r.URL.Query())
	filters.CategoryIDs = map[string]bool{category.ID: true}

	browseData, err := s.buildBrowseResultsPageData(ctx, filters)
	if err != nil {
		s.logger.WithError(err).WithField("category_id", category.ID).Error("failed to build category needs results")
		s.internalServerError(w)
		return
	}

	city := strings.TrimSpace(r.URL.Query().Get("city"))
	cityQuery := ""
	if city != "" {
		cityQuery = "?city=" + url.QueryEscape(city)
	}

	backHref := s.route(RouteCategories)
	browseHref := s.route(RouteBrowse)
	if cityQuery != "" {
		backHref += cityQuery
		browseHref += cityQuery
	}

	data := &types.CategoryNeedsPageData{
		BasePageData: types.BasePageData{Title: fmt.Sprintf("%s Needs", category.Name)},
		Category:     category,
		Needs:        browseData.Needs,
		BackHref:     backHref,
		BrowseHref:   browseHref,
	}

	if err := s.renderTemplate(w, r, "page.category.needs", data); err != nil {
		s.logger.WithError(err).Error("failed to render category needs page")
		s.internalServerError(w)
		return
	}
}

func normalizedCategorySlug(category *types.NeedCategory) string {
	if category == nil {
		return ""
	}

	slug := strings.TrimSpace(category.Slug)
	if slug == "" {
		slug = slugifyCategoryName(category.Name)
	}

	return strings.TrimSpace(slug)
}

func slugifyCategoryName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, "&", "and")
	normalized = strings.ReplaceAll(normalized, " ", "-")
	for strings.Contains(normalized, "--") {
		normalized = strings.ReplaceAll(normalized, "--", "-")
	}
	return strings.Trim(normalized, "-")
}
