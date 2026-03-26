package server

import (
	"christjesus/internal/usps"
	"christjesus/pkg/types"
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

// Run with: go test ./internal/server/ -run TestLiveAddressValidation -v
// Requires USPS_CONSUMER_KEY and USPS_CONSUMER_SECRET in environment.
func TestLiveAddressValidation(t *testing.T) {
	key := os.Getenv("USPS_CONSUMER_KEY")
	secret := os.Getenv("USPS_CONSUMER_SECRET")
	if key == "" || secret == "" {
		t.Skip("USPS_CONSUMER_KEY and USPS_CONSUMER_SECRET not set, skipping live test")
	}

	s := &Service{
		uspsClient: usps.NewClient(key, secret),
		logger:     logrus.New(),
	}

	t.Run("valid address gets standardized", func(t *testing.T) {
		street := "101 Randolph Rd"
		city := "Fort Mill"
		state := "SC"
		zip := "29715"

		addr := &types.UserAddress{
			Address: &street,
			City:    &city,
			State:   &state,
			ZipCode: &zip,
		}

		errMsg := s.validateAndStandardizeAddress(context.Background(), addr)
		if errMsg != "" {
			t.Fatalf("unexpected validation error: %s", errMsg)
		}

		t.Logf("Standardized: %s, %s, %s %s", *addr.Address, *addr.City, *addr.State, *addr.ZipCode)
	})

	t.Run("bad address returns error", func(t *testing.T) {
		street := "99999 Nonexistent Blvd"
		city := "Faketown"
		state := "NC"
		zip := "00000"

		addr := &types.UserAddress{
			Address: &street,
			City:    &city,
			State:   &state,
			ZipCode: &zip,
		}

		errMsg := s.validateAndStandardizeAddress(context.Background(), addr)
		if errMsg == "" {
			t.Fatal("expected validation error for bad address, got empty string")
		}

		t.Logf("Error (expected): %s", errMsg)
	})
}
