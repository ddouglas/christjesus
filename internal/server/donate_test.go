package server

import (
	"testing"
)

func TestSmartPresetAmounts(t *testing.T) {
	tests := []struct {
		name              string
		amountNeededCents int
		amountRaisedCents int
		wantPresets       []int
		wantRemaining     int
	}{
		{
			name:              "fully funded returns static presets, no CTA",
			amountNeededCents: 10000,
			amountRaisedCents: 10000,
			wantPresets:       []int{25, 50, 100, 250},
			wantRemaining:     0,
		},
		{
			name:              "overfunded returns static presets, no CTA",
			amountNeededCents: 10000,
			amountRaisedCents: 12000,
			wantPresets:       []int{25, 50, 100, 250},
			wantRemaining:     0,
		},
		{
			name:              "remaining above largest preset returns static presets, no CTA",
			amountNeededCents: 100000,
			amountRaisedCents: 0,
			wantPresets:       []int{25, 50, 100, 250},
			wantRemaining:     0,
		},
		{
			// remaining=$7 — below all presets; grid empty, CTA only
			name:              "remaining below first preset shows no grid presets",
			amountNeededCents: 10000,
			amountRaisedCents: 9300,
			wantPresets:       nil,
			wantRemaining:     7,
		},
		{
			// remaining=$35 — only $25 fits; $50/$100/$250 removed, CTA=$35
			name:              "remaining between first and second preset filters overages",
			amountNeededCents: 10000,
			amountRaisedCents: 6500,
			wantPresets:       []int{25},
			wantRemaining:     35,
		},
		{
			// remaining=$75 — $25 and $50 fit; $100/$250 removed, CTA=$75
			name:              "remaining between second and third preset filters overages",
			amountNeededCents: 10000,
			amountRaisedCents: 2500,
			wantPresets:       []int{25, 50},
			wantRemaining:     75,
		},
		{
			// remaining=$150 — $25/$50/$100 fit; $250 removed, CTA=$150
			name:              "remaining between third and fourth preset filters overages",
			amountNeededCents: 20000,
			amountRaisedCents: 5000,
			wantPresets:       []int{25, 50, 100},
			wantRemaining:     150,
		},
		{
			// remaining=$25 exactly — matches preset, no CTA needed
			name:              "remaining exactly equal to a preset returns that preset, no CTA",
			amountNeededCents: 10000,
			amountRaisedCents: 7500,
			wantPresets:       []int{25},
			wantRemaining:     0,
		},
		{
			// remaining=$50 exactly — matches preset, no CTA needed
			name:              "remaining exactly equal to second preset, no CTA",
			amountNeededCents: 10000,
			amountRaisedCents: 5000,
			wantPresets:       []int{25, 50},
			wantRemaining:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPresets, gotRemaining := smartPresetAmounts(tt.amountNeededCents, tt.amountRaisedCents)

			if len(gotPresets) != len(tt.wantPresets) {
				t.Fatalf("presets len = %d, want %d (got %v, want %v)", len(gotPresets), len(tt.wantPresets), gotPresets, tt.wantPresets)
			}
			for i := range tt.wantPresets {
				if gotPresets[i] != tt.wantPresets[i] {
					t.Errorf("presets[%d] = %d, want %d", i, gotPresets[i], tt.wantPresets[i])
				}
			}

			if gotRemaining != tt.wantRemaining {
				t.Errorf("remainingPreset = %d, want %d", gotRemaining, tt.wantRemaining)
			}
		})
	}
}
