package usps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateAddress_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "test-token",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		})
	})
	mux.HandleFunc("/addresses/v3/address", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}

		q := r.URL.Query()
		if q.Get("streetAddress") != "1600 Pennsylvania Ave" {
			t.Errorf("unexpected streetAddress: %s", q.Get("streetAddress"))
		}

		resp := addressResponse{}
		resp.Address.StreetAddress = "1600 PENNSYLVANIA AVE NW"
		resp.Address.City = "WASHINGTON"
		resp.Address.State = "DC"
		resp.Address.ZIPCode = "20500"
		resp.Address.ZIPPlus4 = "0005"

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient("test-key", "test-secret")
	client.baseURL = srv.URL

	result, err := client.ValidateAddress(context.Background(), AddressInput{
		StreetAddress: "1600 Pennsylvania Ave",
		City:          "Washington",
		State:         "DC",
		ZIPCode:       "20500",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StreetAddress != "1600 PENNSYLVANIA AVE NW" {
		t.Errorf("expected standardized street, got %s", result.StreetAddress)
	}
	if result.City != "WASHINGTON" {
		t.Errorf("expected WASHINGTON, got %s", result.City)
	}
	if result.ZIPCode != "20500" {
		t.Errorf("expected 20500, got %s", result.ZIPCode)
	}
	if result.ZIPPlus4 != "0005" {
		t.Errorf("expected 0005, got %s", result.ZIPPlus4)
	}
}

func TestValidateAddress_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "test-token",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		})
	})
	mux.HandleFunc("/addresses/v3/address", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errorResponse{
			Error: struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "404",
				Message: "There is no match for the address requested.",
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient("test-key", "test-secret")
	client.baseURL = srv.URL

	_, err := client.ValidateAddress(context.Background(), AddressInput{
		StreetAddress: "99999 Fake Street",
		City:          "Nowhere",
		State:         "ZZ",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	valErr := assertValidationError(t, err)

	if !strings.Contains(valErr.Message, "couldn't find") {
		t.Errorf("expected user-friendly message, got: %s", valErr.Message)
	}
}

func TestValidateAddress_MultipleAddresses(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "test-token",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		})
	})
	mux.HandleFunc("/addresses/v3/address", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errorResponse{
			Error: struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{
				Code:    "404",
				Message: "More than one address was found matching the requested address.",
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient("test-key", "test-secret")
	client.baseURL = srv.URL

	_, err := client.ValidateAddress(context.Background(), AddressInput{
		StreetAddress: "123 Main St",
		City:          "Springfield",
		State:         "IL",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	valErr := assertValidationError(t, err)

	if !strings.Contains(valErr.Message, "Multiple addresses") {
		t.Errorf("expected multiple addresses message, got: %s", valErr.Message)
	}
}

func TestTokenCaching(t *testing.T) {
	tokenCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		tokenCalls++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "test-token",
			ExpiresIn:   3600,
			TokenType:   "Bearer",
		})
	})
	mux.HandleFunc("/addresses/v3/address", func(w http.ResponseWriter, r *http.Request) {
		resp := addressResponse{}
		resp.Address.StreetAddress = "123 MAIN ST"
		resp.Address.City = "ANYTOWN"
		resp.Address.State = "VA"
		resp.Address.ZIPCode = "22030"

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient("test-key", "test-secret")
	client.baseURL = srv.URL

	input := AddressInput{
		StreetAddress: "123 Main St",
		City:          "Anytown",
		State:         "VA",
	}

	// Make two calls — token should be fetched only once
	if _, err := client.ValidateAddress(context.Background(), input); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := client.ValidateAddress(context.Background(), input); err != nil {
		t.Fatalf("second call: %v", err)
	}

	if tokenCalls != 1 {
		t.Errorf("expected 1 token call, got %d", tokenCalls)
	}
}

func TestMapUSPSError(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"There is no match for the address requested.", "couldn't find"},
		{"Address not found.", "couldn't find"},
		{"More than one address was found matching the requested address.", "Multiple addresses"},
		{"The address information in the request is insufficient to match.", "Not enough"},
		{"The address requested is an invalid delivery address.", "valid delivery"},
		{"The state code in the request is missing or invalid.", "state code"},
		{"The city in the request is missing or invalid.", "city name"},
		{"The city and state are missing or together unverifiable.", "city and state"},
		{"Some unknown error.", "couldn't verify"},
	}

	for _, tt := range tests {
		result := mapUSPSError(tt.input)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("mapUSPSError(%q) = %q, expected to contain %q", tt.input, result, tt.contains)
		}
	}
}

func assertValidationError(t *testing.T, err error) *ValidationError {
	t.Helper()
	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	return valErr
}
