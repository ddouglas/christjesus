package server

import "testing"

func TestSubtleCompare_CallbackSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		a      string
		b      string
		match  bool
		reason string
	}{
		{
			name:   "state exact match",
			a:      "oauthed-state-value",
			b:      "oauthed-state-value",
			match:  true,
			reason: "callback state should pass only on exact equality",
		},
		{
			name:   "state mismatch",
			a:      "oauthed-state-value",
			b:      "oauthed-state-value-x",
			match:  false,
			reason: "callback state mismatch must reject login",
		},
		{
			name:   "nonce exact match",
			a:      "nonce-123456",
			b:      "nonce-123456",
			match:  true,
			reason: "id token nonce should match cookie nonce",
		},
		{
			name:   "nonce same prefix but different tail",
			a:      "nonce-123456",
			b:      "nonce-123457",
			match:  false,
			reason: "near matches are still invalid",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := subtleCompare(tt.a, tt.b)
			if got != tt.match {
				t.Fatalf("subtleCompare(%q, %q) = %v, want %v (%s)", tt.a, tt.b, got, tt.match, tt.reason)
			}
		})
	}
}
