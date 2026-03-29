package server

import (
	"net/url"
	"strings"
	"testing"

	"christjesus/pkg/types"
)

func TestBuildRoute_SelectedRouteShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		route    RouteName
		opts     []RouteOption
		expected string
	}{
		{
			name:     "no parameters",
			route:    RouteBrowse,
			opts:     nil,
			expected: "/browse",
		},
		{
			name:     "single parameter",
			route:    RouteNeedDetail,
			opts:     []RouteOption{Param("needID", "need_123")},
			expected: "/need/need_123",
		},
		{
			name:     "single parameter trims spaces",
			route:    RouteNeedDetail,
			opts:     []RouteOption{Param("needID", " need_123 ")},
			expected: "/need/need_123",
		},
		{
			name:  "multiple parameters",
			route: RouteOnboardingNeedDocumentDelete,
			opts: []RouteOption{
				Param("needID", "need_123"),
				Param("documentID", "doc_456"),
			},
			expected: "/onboarding/need/need_123/documents/doc_456/delete",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := BuildRoute(tt.route, tt.opts...)
			if err != nil {
				t.Fatalf("BuildRoute() unexpected error: %v", err)
			}

			if actual != tt.expected {
				t.Fatalf("BuildRoute() = %q, want %q", actual, tt.expected)
			}
		})
	}
}

func TestBuildRoute_NoOpts(t *testing.T) {
	t.Parallel()

	actual, err := BuildRoute(RouteBrowse)
	if err != nil {
		t.Fatalf("BuildRoute() unexpected error: %v", err)
	}
	if actual != "/browse" {
		t.Fatalf("BuildRoute() = %q, want %q", actual, "/browse")
	}
}

func TestBuildRoute_WithQueryOpt(t *testing.T) {
	t.Parallel()

	actual, err := BuildRoute(RouteBrowse, Query("city", "Nashville"), Query("sort", "urgency"))
	if err != nil {
		t.Fatalf("BuildRoute() unexpected error: %v", err)
	}

	parsed, err := url.Parse(actual)
	if err != nil {
		t.Fatalf("url.Parse() error: %v", err)
	}
	if parsed.Path != "/browse" {
		t.Fatalf("path = %q, want %q", parsed.Path, "/browse")
	}
	if parsed.Query().Get("city") != "Nashville" {
		t.Fatalf("city = %q, want %q", parsed.Query().Get("city"), "Nashville")
	}
	if parsed.Query().Get("sort") != "urgency" {
		t.Fatalf("sort = %q, want %q", parsed.Query().Get("sort"), "urgency")
	}
}

func TestBuildRoute_MissingRequiredParam(t *testing.T) {
	t.Parallel()

	_, err := BuildRoute(RouteNeedDetail)
	if err == nil {
		t.Fatal("expected error for missing required param")
	}
	if !strings.Contains(err.Error(), "missing route params") {
		t.Fatalf("err = %v, want message containing 'missing route params'", err)
	}
}

func TestBuildRoute_EmptyParamValue(t *testing.T) {
	t.Parallel()

	_, err := BuildRoute(RouteNeedDetail, Param("needID", ""))
	if err == nil {
		t.Fatal("expected error for empty param value")
	}
	if !strings.Contains(err.Error(), "missing route params") {
		t.Fatalf("err = %v, want message containing 'missing route params'", err)
	}
}

func TestBuildRoute_NilOptionHandledSafely(t *testing.T) {
	t.Parallel()

	actual, err := BuildRoute(RouteBrowse, nil, nil)
	if err != nil {
		t.Fatalf("BuildRoute() unexpected error with nil opts: %v", err)
	}
	if actual != "/browse" {
		t.Fatalf("BuildRoute() = %q, want %q", actual, "/browse")
	}
}

func TestRoutePattern_UnknownRoutePanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("expected RoutePattern to panic for unknown route")
		}
	}()

	_ = RoutePattern(RouteName("does.not.exist"))
}

func TestNormalizedCategorySlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category *types.NeedCategory
		expected string
	}{
		{name: "nil category", category: nil, expected: ""},
		{
			name: "uses explicit slug",
			category: &types.NeedCategory{
				Slug: "  urgent-needs  ",
				Name: "Ignored",
			},
			expected: "urgent-needs",
		},
		{
			name: "falls back to name slug",
			category: &types.NeedCategory{
				Slug: "",
				Name: "Housing Support",
			},
			expected: "housing-support",
		},
		{
			name: "empty when name slugifies empty",
			category: &types.NeedCategory{
				Slug: "   ",
				Name: "   ",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := normalizedCategorySlug(tt.category)
			if actual != tt.expected {
				t.Fatalf("normalizedCategorySlug() = %q, want %q", actual, tt.expected)
			}
		})
	}
}

func TestTemplateFuncMapRouteReturnsErrorOnInvalidParams(t *testing.T) {
	t.Parallel()

	funcs := templateFuncMap()
	route, ok := funcs["route"].(func(string, ...RouteOption) (string, error))
	if !ok {
		t.Fatalf("route helper has unexpected type")
	}

	_, err := route("category.needs", Param("slug", ""))
	if err == nil {
		t.Fatal("expected error from route helper")
	}
	if !strings.Contains(err.Error(), "template route") {
		t.Fatalf("err = %v, want message containing template route", err)
	}
}

func TestTemplateFuncMapRouteEmptyParamStillErrors(t *testing.T) {
	t.Parallel()

	funcs := templateFuncMap()
	route, ok := funcs["route"].(func(string, ...RouteOption) (string, error))
	if !ok {
		t.Fatalf("route helper has unexpected type")
	}

	// category.needs requires :slug; providing empty value should error
	_, err := route("category.needs", Param("slug", ""))
	if err == nil {
		t.Fatal("expected error from route helper for empty param value")
	}
	if !strings.Contains(err.Error(), "template route") {
		t.Fatalf("err = %v, want message containing 'template route'", err)
	}
}

func TestTemplateFuncMapQueryBuildsQueryString(t *testing.T) {
	t.Parallel()

	funcs := templateFuncMap()
	route, ok := funcs["route"].(func(string, ...RouteOption) (string, error))
	if !ok {
		t.Fatalf("route helper has unexpected type")
	}
	queryFn, ok := funcs["query"].(func(string, string) RouteOption)
	if !ok {
		t.Fatalf("query helper has unexpected type")
	}

	actual, err := route("browse", queryFn("city", "New York"))
	if err != nil {
		t.Fatalf("route() unexpected error: %v", err)
	}

	parsed, err := url.Parse(actual)
	if err != nil {
		t.Fatalf("url.Parse() error: %v", err)
	}

	if parsed.Path != "/browse" {
		t.Fatalf("path = %q, want %q", parsed.Path, "/browse")
	}

	if parsed.Query().Get("city") != "New York" {
		t.Fatalf("city query = %q, want %q", parsed.Query().Get("city"), "New York")
	}
}
