package server

import (
	"net/url"
	"testing"

	"christjesus/pkg/types"
)

func TestParseBrowseFilters_EmptyQuery(t *testing.T) {
	t.Parallel()

	filters := parseBrowseFilters(url.Values{})

	if len(filters.CategoryIDs) != 0 {
		t.Fatalf("parseBrowseFilters(empty) CategoryIDs = %v, want empty map", filters.CategoryIDs)
	}
	if filters.Search != "" {
		t.Fatalf("parseBrowseFilters(empty) Search = %q, want empty", filters.Search)
	}
	if filters.City != "" {
		t.Fatalf("parseBrowseFilters(empty) City = %q, want empty", filters.City)
	}
	if filters.FundingMax != 100 {
		t.Fatalf("parseBrowseFilters(empty) FundingMax = %d, want 100", filters.FundingMax)
	}
	if filters.ViewMode != "grid" {
		t.Fatalf("parseBrowseFilters(empty) ViewMode = %q, want grid", filters.ViewMode)
	}
	if filters.SortBy != "urgency" {
		t.Fatalf("parseBrowseFilters(empty) SortBy = %q, want urgency", filters.SortBy)
	}
	if filters.Page != 1 {
		t.Fatalf("parseBrowseFilters(empty) Page = %d, want 1", filters.Page)
	}
	if filters.PageSize != browseDefaultPageSize {
		t.Fatalf("parseBrowseFilters(empty) PageSize = %d, want %d", filters.PageSize, browseDefaultPageSize)
	}
}

func TestParseBrowseFilters_WithCategories(t *testing.T) {
	t.Parallel()

	query := url.Values{}
	query.Add("category", "cat_001")
	query.Add("category", "cat_002")
	query.Add("category", "cat_003")

	filters := parseBrowseFilters(query)

	if len(filters.CategoryIDs) != 3 {
		t.Fatalf("parseBrowseFilters() CategoryIDs length = %d, want 3", len(filters.CategoryIDs))
	}
	for _, id := range []string{"cat_001", "cat_002", "cat_003"} {
		if !filters.CategoryIDs[id] {
			t.Fatalf("parseBrowseFilters() CategoryIDs missing %q", id)
		}
	}
}

func TestParseBrowseFilters_EmptyCategoriesIgnored(t *testing.T) {
	t.Parallel()

	query := url.Values{}
	query.Add("category", "cat_001")
	query.Add("category", "")
	query.Add("category", "   ")

	filters := parseBrowseFilters(query)

	if len(filters.CategoryIDs) != 1 {
		t.Fatalf("parseBrowseFilters() CategoryIDs length = %d, want 1 (got %v)", len(filters.CategoryIDs), filters.CategoryIDs)
	}
	if !filters.CategoryIDs["cat_001"] {
		t.Fatal("parseBrowseFilters() CategoryIDs should contain cat_001")
	}
}

func TestParseBrowseFilters_FundingMax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		expected int
	}{
		{name: "valid 50", raw: "50", expected: 50},
		{name: "zero", raw: "0", expected: 0},
		{name: "negative clamped to 0", raw: "-10", expected: 0},
		{name: "above 100 clamped", raw: "150", expected: 100},
		{name: "exactly 100", raw: "100", expected: 100},
		{name: "non-numeric falls back to 100", raw: "abc", expected: 100},
		{name: "empty falls back to 100", raw: "", expected: 100},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			query := url.Values{}
			if tt.raw != "" {
				query.Set("funding_max", tt.raw)
			}

			filters := parseBrowseFilters(query)
			if filters.FundingMax != tt.expected {
				t.Fatalf("parseBrowseFilters(funding_max=%q) FundingMax = %d, want %d", tt.raw, filters.FundingMax, tt.expected)
			}
		})
	}
}

func TestParseBrowseFilters_ViewMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{name: "list mode", raw: "list", expected: "list"},
		{name: "list uppercase", raw: "LIST", expected: "list"},
		{name: "grid mode", raw: "grid", expected: "grid"},
		{name: "unknown defaults to grid", raw: "table", expected: "grid"},
		{name: "empty defaults to grid", raw: "", expected: "grid"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			query := url.Values{}
			if tt.raw != "" {
				query.Set("view", tt.raw)
			}

			filters := parseBrowseFilters(query)
			if filters.ViewMode != tt.expected {
				t.Fatalf("parseBrowseFilters(view=%q) ViewMode = %q, want %q", tt.raw, filters.ViewMode, tt.expected)
			}
		})
	}
}

func TestParseBrowseFilters_SortBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{name: "newest", raw: "newest", expected: "newest"},
		{name: "closest", raw: "closest", expected: "closest"},
		{name: "nearest", raw: "nearest", expected: "nearest"},
		{name: "urgency", raw: "urgency", expected: "urgency"},
		{name: "uppercase newest", raw: "NEWEST", expected: "newest"},
		{name: "unknown defaults to urgency", raw: "random", expected: "urgency"},
		{name: "empty defaults to urgency", raw: "", expected: "urgency"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			query := url.Values{}
			if tt.raw != "" {
				query.Set("sort", tt.raw)
			}

			filters := parseBrowseFilters(query)
			if filters.SortBy != tt.expected {
				t.Fatalf("parseBrowseFilters(sort=%q) SortBy = %q, want %q", tt.raw, filters.SortBy, tt.expected)
			}
		})
	}
}

func TestParseBrowseFilters_SearchAndCity(t *testing.T) {
	t.Parallel()

	query := url.Values{}
	query.Set("search", "  housing  ")
	query.Set("city", "  Nashville  ")

	filters := parseBrowseFilters(query)

	if filters.Search != "housing" {
		t.Fatalf("parseBrowseFilters() Search = %q, want %q", filters.Search, "housing")
	}
	if filters.City != "Nashville" {
		t.Fatalf("parseBrowseFilters() City = %q, want %q", filters.City, "Nashville")
	}
}

func TestParseBrowseFilters_PageParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		expected int
	}{
		{name: "valid page 3", raw: "3", expected: 3},
		{name: "page 1", raw: "1", expected: 1},
		{name: "zero falls back to 1", raw: "0", expected: 1},
		{name: "negative falls back to 1", raw: "-1", expected: 1},
		{name: "non-numeric falls back to 1", raw: "abc", expected: 1},
		{name: "empty falls back to 1", raw: "", expected: 1},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			query := url.Values{}
			if tt.raw != "" {
				query.Set("page", tt.raw)
			}

			filters := parseBrowseFilters(query)
			if filters.Page != tt.expected {
				t.Fatalf("parseBrowseFilters(page=%q) Page = %d, want %d", tt.raw, filters.Page, tt.expected)
			}
		})
	}
}

func TestBrowseUrgency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		status            types.NeedStatus
		amountNeededCents int
		amountRaisedCents int
		expectedLabel     string
	}{
		{name: "zero needed returns LOW", status: types.NeedStatusActive, amountNeededCents: 0, amountRaisedCents: 0, expectedLabel: "LOW"},
		{name: "under 35 percent is HIGH", status: types.NeedStatusActive, amountNeededCents: 10000, amountRaisedCents: 3000, expectedLabel: "HIGH"},
		{name: "exactly 35 percent is MEDIUM", status: types.NeedStatusActive, amountNeededCents: 10000, amountRaisedCents: 3500, expectedLabel: "MEDIUM"},
		{name: "under 70 percent is MEDIUM", status: types.NeedStatusActive, amountNeededCents: 10000, amountRaisedCents: 6900, expectedLabel: "MEDIUM"},
		{name: "70 percent or above is LOW", status: types.NeedStatusActive, amountNeededCents: 10000, amountRaisedCents: 7000, expectedLabel: "LOW"},
		{name: "fully funded is LOW", status: types.NeedStatusActive, amountNeededCents: 10000, amountRaisedCents: 10000, expectedLabel: "LOW"},
		{name: "submitted status is URGENT", status: types.NeedStatusSubmitted, amountNeededCents: 10000, amountRaisedCents: 9000, expectedLabel: "URGENT"},
		{name: "under review is URGENT", status: types.NeedStatusUnderReview, amountNeededCents: 10000, amountRaisedCents: 9000, expectedLabel: "URGENT"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			label, dotClass := browseUrgency(tt.status, tt.amountNeededCents, tt.amountRaisedCents)
			if label != tt.expectedLabel {
				t.Fatalf("browseUrgency() label = %q, want %q", label, tt.expectedLabel)
			}
			if dotClass == "" {
				t.Fatal("browseUrgency() dotClass should not be empty")
			}
		})
	}
}

func TestUserDisplayName(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name     string
		user     *types.User
		expected string
	}{
		{name: "nil user", user: nil, expected: "Anonymous"},
		{name: "empty user", user: &types.User{}, expected: "Anonymous"},
		{name: "given name only", user: &types.User{GivenName: strPtr("John")}, expected: "John"},
		{name: "family name only", user: &types.User{FamilyName: strPtr("Doe")}, expected: "Doe"},
		{name: "full name", user: &types.User{GivenName: strPtr("John"), FamilyName: strPtr("Doe")}, expected: "John Doe"},
		{name: "email fallback", user: &types.User{Email: strPtr("john@example.com")}, expected: "john"},
		{name: "whitespace names fall to email", user: &types.User{GivenName: strPtr("  "), FamilyName: strPtr("  "), Email: strPtr("test@example.com")}, expected: "test"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := userDisplayName(tt.user)
			if actual != tt.expected {
				t.Fatalf("userDisplayName() = %q, want %q", actual, tt.expected)
			}
		})
	}
}
