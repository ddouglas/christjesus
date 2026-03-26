package server

import (
	"christjesus/internal/usps"
	"christjesus/pkg/types"
	"context"
	"errors"
	"strings"
)

// validateAndStandardizeAddress calls the USPS API to validate and standardize
// the address fields on the given UserAddress. If USPS credentials are not
// configured, it silently skips validation.
// Returns a user-facing error message if the address is invalid, or empty string on success.
func (s *Service) validateAndStandardizeAddress(ctx context.Context, addr *types.UserAddress) string {
	if addr.Address == nil || addr.State == nil {
		return ""
	}

	input := usps.AddressInput{
		StreetAddress: strings.TrimSpace(*addr.Address),
		State:         strings.TrimSpace(*addr.State),
	}
	if addr.AddressExt != nil {
		input.SecondaryAddress = strings.TrimSpace(*addr.AddressExt)
	}
	if addr.City != nil {
		input.City = strings.TrimSpace(*addr.City)
	}
	if addr.ZipCode != nil {
		input.ZIPCode = strings.TrimSpace(*addr.ZipCode)
	}

	result, err := s.uspsClient.ValidateAddress(ctx, input)
	if err != nil {
		var valErr *usps.ValidationError
		if errors.As(err, &valErr) {
			return valErr.Message
		}
		s.logger.WithError(err).Error("USPS address validation failed unexpectedly")
		// Don't block the user on USPS outages — skip validation
		return ""
	}

	// Overwrite with standardized values
	addr.Address = &result.StreetAddress
	if result.SecondaryAddress != "" {
		addr.AddressExt = &result.SecondaryAddress
	}
	addr.City = &result.City
	addr.State = &result.State
	zip := result.ZIPCode
	if result.ZIPPlus4 != "" {
		zip = result.ZIPCode + "-" + result.ZIPPlus4
	}
	addr.ZipCode = &zip

	return ""
}
