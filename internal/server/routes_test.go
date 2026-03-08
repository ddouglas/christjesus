package server

import "testing"

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
