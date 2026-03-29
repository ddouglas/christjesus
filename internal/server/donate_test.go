package server

import (
	"testing"
)

func TestSmartPresetAmounts(t *testing.T) {
	tests := []struct {
		name              string
		amountNeededCents int
		amountRaisedCents int
		want              []int
	}{
		{
			name:              "fully funded returns static presets",
			amountNeededCents: 10000,
			amountRaisedCents: 10000,
			want:              []int{25, 50, 100, 250},
		},
		{
			name:              "overfunded returns static presets",
			amountNeededCents: 10000,
			amountRaisedCents: 12000,
			want:              []int{25, 50, 100, 250},
		},
		{
			name:              "remaining above largest preset returns static presets",
			amountNeededCents: 100000,
			amountRaisedCents: 0,
			want:              []int{25, 50, 100, 250},
		},
		{
			// remaining=$7 — replaces first slot
			name:              "remaining below first preset replaces first slot",
			amountNeededCents: 10000,
			amountRaisedCents: 9300,
			want:              []int{7, 50, 100, 250},
		},
		{
			// remaining=$35 — between $25 and $50, replaces second slot
			name:              "remaining between first and second preset replaces second slot",
			amountNeededCents: 10000,
			amountRaisedCents: 6500,
			want:              []int{25, 35, 100, 250},
		},
		{
			// remaining=$75 — between $50 and $100, replaces third slot
			name:              "remaining between second and third preset replaces third slot",
			amountNeededCents: 10000,
			amountRaisedCents: 2500,
			want:              []int{25, 50, 75, 250},
		},
		{
			// remaining=$150 — between $100 and $250, replaces fourth slot
			name:              "remaining between third and fourth preset replaces fourth slot",
			amountNeededCents: 20000,
			amountRaisedCents: 5000,
			want:              []int{25, 50, 100, 150},
		},
		{
			// remaining=$25 — exactly equals first preset, no substitution needed
			name:              "remaining exactly equal to a preset returns static presets",
			amountNeededCents: 10000,
			amountRaisedCents: 7500,
			want:              []int{25, 50, 100, 250},
		},
		{
			// remaining=$50 — exactly equals second preset, no substitution needed
			name:              "remaining exactly equal to second preset returns static presets",
			amountNeededCents: 10000,
			amountRaisedCents: 5000,
			want:              []int{25, 50, 100, 250},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := smartPresetAmounts(tt.amountNeededCents, tt.amountRaisedCents)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("presets[%d] = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}
