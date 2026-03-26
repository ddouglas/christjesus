package usps_test

import (
	"christjesus/internal/usps"
	"context"
	"fmt"
	"os"
	"testing"
)

// Run with: go test ./internal/usps/ -run TestLiveValidation -v
// Requires USPS_CONSUMER_KEY and USPS_CONSUMER_SECRET in environment.
func TestLiveValidation(t *testing.T) {
	key := os.Getenv("USPS_CONSUMER_KEY")
	secret := os.Getenv("USPS_CONSUMER_SECRET")
	if key == "" || secret == "" {
		t.Skip("USPS_CONSUMER_KEY and USPS_CONSUMER_SECRET not set, skipping live test")
	}

	client := usps.NewClient(key, secret)

	t.Run("valid address", func(t *testing.T) {
		result, err := client.ValidateAddress(context.Background(), usps.AddressInput{
			StreetAddress: "1600 Pennsylvania Ave",
			City:          "Washington",
			State:         "DC",
			ZIPCode:       "20500",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fmt.Printf("Standardized: %s, %s, %s %s-%s\n",
			result.StreetAddress, result.City, result.State, result.ZIPCode, result.ZIPPlus4)
	})

	t.Run("bad address", func(t *testing.T) {
		_, err := client.ValidateAddress(context.Background(), usps.AddressInput{
			StreetAddress: "99999 Nonexistent Blvd",
			City:          "Faketown",
			State:         "ZZ",
		})
		if err == nil {
			t.Fatal("expected error for bad address, got nil")
		}
		fmt.Printf("Error (expected): %v\n", err)
	})
}
