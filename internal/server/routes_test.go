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
		params   map[string]string
		expected string
	}{
		{
			name:     "no parameters",
			route:    RouteBrowse,
			params:   nil,
			expected: "/browse",
		},
		{
			name:     "single parameter",
			route:    RouteNeedDetail,
			params:   map[string]string{"id": "need_123"},
			expected: "/need/need_123",
		},
		{
			name:     "single parameter trims spaces",
			route:    RouteNeedDetail,
			params:   map[string]string{"id": " need_123 "},
			expected: "/need/need_123",
		},
		{
			name:  "multiple parameters",
			route: RouteOnboardingNeedDocumentDelete,
			params: map[string]string{
				"needID":     "need_123",
				"documentID": "doc_456",
			},
			expected: "/onboarding/need/need_123/documents/doc_456/delete",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := BuildRoute(tt.route, tt.params)
			if err != nil {
				t.Fatalf("BuildRoute() unexpected error: %v", err)
			}

			if actual != tt.expected {
				t.Fatalf("BuildRoute() = %q, want %q", actual, tt.expected)
			}
		})
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
	route, ok := funcs["route"].(func(string, map[string]string) (string, error))
	if !ok {
		t.Fatalf("route helper has unexpected type")
	}

	_, err := route("category.needs", map[string]string{"slug": ""})
	if err == nil {
		t.Fatal("expected error from route helper")
	}
	if !strings.Contains(err.Error(), "template route") {
		t.Fatalf("err = %v, want message containing template route", err)
	}
}

func TestTemplateFuncMapRouteQReturnsErrorOnInvalidParams(t *testing.T) {
	t.Parallel()

	funcs := templateFuncMap()
	routeq, ok := funcs["routeq"].(func(string, map[string]string, map[string]string) (string, error))
	if !ok {
		t.Fatalf("routeq helper has unexpected type")
	}

	_, err := routeq("category.needs", map[string]string{"slug": ""}, map[string]string{"city": "Nashville"})
	if err == nil {
		t.Fatal("expected error from routeq helper")
	}
	if !strings.Contains(err.Error(), "template routeq") {
		t.Fatalf("err = %v, want message containing template routeq", err)
	}
}

func TestTemplateFuncMapDictReturnsErrorOnOddArgs(t *testing.T) {
	t.Parallel()

	funcs := templateFuncMap()
	dict, ok := funcs["dict"].(func(...string) (map[string]string, error))
	if !ok {
		t.Fatalf("dict helper has unexpected type")
	}

	_, err := dict("key-only")
	if err == nil {
		t.Fatal("expected error from dict helper")
	}
	if !strings.Contains(err.Error(), "dict expects even") {
		t.Fatalf("err = %v, want message containing dict expects even", err)
	}
}

func TestTemplateFuncMapRouteQBuildsQueryString(t *testing.T) {
	t.Parallel()

	funcs := templateFuncMap()
	routeq, ok := funcs["routeq"].(func(string, map[string]string, map[string]string) (string, error))
	if !ok {
		t.Fatalf("routeq helper has unexpected type")
	}

	actual, err := routeq("browse", nil, map[string]string{"city": "New York", "": "ignored"})
	if err != nil {
		t.Fatalf("routeq() unexpected error: %v", err)
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
