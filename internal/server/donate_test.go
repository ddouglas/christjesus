package server

import (
	"testing"
)

func TestBuildSmartPresets(t *testing.T) {
	t.Parallel()

	standardPresets := []int{25, 50, 100, 250}

	tests := []struct {
		name           string
		remainingCents int
		standardPresets []int
		expected       []int
	}{
		{
			name:            "remaining $73 prepends 73 to presets",
			remainingCents:  7300,
			standardPresets: standardPresets,
			expected:        []int{73, 25, 50, 100, 250},
		},
		{
			name:            "remaining $0 returns standard presets",
			remainingCents:  0,
			standardPresets: standardPresets,
			expected:        standardPresets,
		},
		{
			name:            "negative remaining returns standard presets",
			remainingCents:  -500,
			standardPresets: standardPresets,
			expected:        standardPresets,
		},
		{
			name:            "remaining exactly matches existing preset $50",
			remainingCents:  5000,
			standardPresets: standardPresets,
			expected:        []int{25, 50, 100, 250},
		},
		{
			name:            "remaining exactly matches existing preset $25",
			remainingCents:  2500,
			standardPresets: standardPresets,
			expected:        []int{25, 50, 100, 250},
		},
		{
			name:            "remaining exactly matches existing preset $100",
			remainingCents:  10000,
			standardPresets: standardPresets,
			expected:        []int{25, 50, 100, 250},
		},
		{
			name:            "remaining exactly matches existing preset $250",
			remainingCents:  25000,
			standardPresets: standardPresets,
			expected:        []int{25, 50, 100, 250},
		},
		{
			name:            "fractional cents rounds up to next dollar",
			remainingCents:  7350,
			standardPresets: standardPresets,
			expected:        []int{74, 25, 50, 100, 250},
		},
		{
			name:            "1 cent remaining rounds up to $1",
			remainingCents:  1,
			standardPresets: standardPresets,
			expected:        []int{1, 25, 50, 100, 250},
		},
		{
			name:            "99 cents remaining rounds up to $1",
			remainingCents:  99,
			standardPresets: standardPresets,
			expected:        []int{1, 25, 50, 100, 250},
		},
		{
			name:            "large remaining amount $500",
			remainingCents:  50000,
			standardPresets: standardPresets,
			expected:        []int{500, 25, 50, 100, 250},
		},
		{
			name:            "empty standard presets with remaining",
			remainingCents:  7300,
			standardPresets: []int{},
			expected:        []int{73},
		},
		{
			name:            "empty standard presets with zero remaining",
			remainingCents:  0,
			standardPresets: []int{},
			expected:        []int{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := buildSmartPresets(tt.remainingCents, tt.standardPresets)

			if len(actual) != len(tt.expected) {
				t.Fatalf("buildSmartPresets() returned %d items %v, want %d items %v",
					len(actual), actual, len(tt.expected), tt.expected)
			}

			for i := range tt.expected {
				if actual[i] != tt.expected[i] {
					t.Fatalf("buildSmartPresets()[%d] = %d, want %d (full result: %v)",
						i, actual[i], tt.expected[i], actual)
				}
			}
		})
	}
}

func TestParseDonationAmountCents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		raw         string
		wantCents   int
		wantErr     bool
	}{
		{name: "simple integer", raw: "50", wantCents: 5000, wantErr: false},
		{name: "with dollar sign", raw: "$100", wantCents: 10000, wantErr: false},
		{name: "with commas", raw: "1,000", wantCents: 100000, wantErr: false},
		{name: "with dollar sign and commas", raw: "$1,500", wantCents: 150000, wantErr: false},
		{name: "with whitespace", raw: "  25  ", wantCents: 2500, wantErr: false},
		{name: "empty string", raw: "", wantCents: 0, wantErr: true},
		{name: "whitespace only", raw: "   ", wantCents: 0, wantErr: true},
		{name: "zero amount", raw: "0", wantCents: 0, wantErr: true},
		{name: "negative amount", raw: "-10", wantCents: 0, wantErr: true},
		{name: "non-numeric", raw: "abc", wantCents: 0, wantErr: true},
		{name: "decimal value", raw: "25.50", wantCents: 0, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cents, err := parseDonationAmountCents(tt.raw)
			if tt.wantErr {
				if err == nil && cents > 0 {
					t.Fatalf("parseDonationAmountCents(%q) = %d, nil; want error", tt.raw, cents)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDonationAmountCents(%q) unexpected error: %v", tt.raw, err)
			}
			if cents != tt.wantCents {
				t.Fatalf("parseDonationAmountCents(%q) = %d, want %d", tt.raw, cents, tt.wantCents)
			}
		})
	}
}

func TestDonationStatusLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{name: "finalized", status: "finalized", expected: "Payment Complete"},
		{name: "failed", status: "failed", expected: "Payment Failed"},
		{name: "canceled", status: "canceled", expected: "Payment Canceled"},
		{name: "pending", status: "pending", expected: "Payment Processing"},
		{name: "empty string", status: "", expected: "Payment Processing"},
		{name: "unknown status", status: "refunded", expected: "Payment Processing"},
		{name: "with whitespace", status: "  finalized  ", expected: "Payment Complete"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := donationStatusLabel(tt.status)
			if actual != tt.expected {
				t.Fatalf("donationStatusLabel(%q) = %q, want %q", tt.status, actual, tt.expected)
			}
		})
	}
}

func TestDonationStatusTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    string
		ownerName string
		expected  string
	}{
		{name: "finalized", status: "finalized", ownerName: "John", expected: "Thank you for supporting John"},
		{name: "failed", status: "failed", ownerName: "Jane", expected: "We couldn't complete your donation"},
		{name: "canceled", status: "canceled", ownerName: "Bob", expected: "Donation was canceled"},
		{name: "pending", status: "pending", ownerName: "Alice", expected: "Thanks — we captured your donation"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := donationStatusTitle(tt.status, tt.ownerName)
			if actual != tt.expected {
				t.Fatalf("donationStatusTitle(%q, %q) = %q, want %q", tt.status, tt.ownerName, actual, tt.expected)
			}
		})
	}
}

func TestDonationStatusDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   string
		contains string
	}{
		{name: "finalized", status: "finalized", contains: "finalized and has been applied"},
		{name: "failed", status: "failed", contains: "did not complete"},
		{name: "canceled", status: "canceled", contains: "exited checkout"},
		{name: "pending", status: "pending", contains: "still processing"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := donationStatusDescription(tt.status)
			if len(actual) == 0 {
				t.Fatalf("donationStatusDescription(%q) returned empty string", tt.status)
			}
		})
	}
}

func TestDonationStatusGuidance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
	}{
		{name: "finalized", status: "finalized"},
		{name: "failed", status: "failed"},
		{name: "canceled", status: "canceled"},
		{name: "pending", status: "pending"},
		{name: "unknown", status: "something_else"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := donationStatusGuidance(tt.status)
			if len(actual) == 0 {
				t.Fatalf("donationStatusGuidance(%q) returned empty string", tt.status)
			}
		})
	}
}
