package usps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	prodBaseURL  = "https://apis.usps.com"
	tokenPath    = "/oauth2/v3/token"
	addressPath  = "/addresses/v3/address"
	tokenRefresh = 5 * time.Minute // refresh token this long before expiry
)

// Client is a USPS API client that handles OAuth2 token management
// and address validation.
type Client struct {
	httpClient   *http.Client
	baseURL      string
	consumerKey  string
	consumerSecret string

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// NewClient creates a new USPS API client.
func NewClient(consumerKey, consumerSecret string) *Client {
	return &Client{
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		baseURL:        prodBaseURL,
		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,
	}
}

// tokenResponse represents the OAuth2 token response from USPS.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// getToken returns a valid access token, fetching a new one if necessary.
func (c *Client) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.consumerKey},
		"client_secret": {c.consumerSecret},
		"scope":         {"addresses"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+tokenPath, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("usps: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("usps: token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("usps: read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("usps: token request returned %d: %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("usps: parse token response: %w", err)
	}

	c.accessToken = tok.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn)*time.Second - tokenRefresh)

	return c.accessToken, nil
}

// AddressInput is the input for address validation.
type AddressInput struct {
	StreetAddress    string
	SecondaryAddress string
	City             string
	State            string
	ZIPCode          string
}

// StandardizedAddress is the result of a successful address validation.
type StandardizedAddress struct {
	StreetAddress string
	SecondaryAddress string
	City          string
	State         string
	ZIPCode       string
	ZIPPlus4      string
}

// addressResponse represents the USPS address validation API response.
type addressResponse struct {
	Address struct {
		StreetAddress    string `json:"streetAddress"`
		SecondaryAddress string `json:"secondaryAddress"`
		City             string `json:"city"`
		State            string `json:"state"`
		ZIPCode          string `json:"ZIPCode"`
		ZIPPlus4         string `json:"ZIPPlus4"`
	} `json:"address"`
}

// errorResponse represents a USPS API error.
type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ValidationError is returned when USPS cannot validate the address.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ValidateAddress validates and standardizes an address using the USPS API.
// Returns a ValidationError if the address cannot be verified.
func (c *Client) ValidateAddress(ctx context.Context, input AddressInput) (*StandardizedAddress, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	params := url.Values{
		"streetAddress": {input.StreetAddress},
		"state":         {input.State},
	}
	if input.SecondaryAddress != "" {
		params.Set("secondaryAddress", input.SecondaryAddress)
	}
	if input.City != "" {
		params.Set("city", input.City)
	}
	if input.ZIPCode != "" {
		params.Set("ZIPCode", input.ZIPCode)
	}

	reqURL := c.baseURL + addressPath + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("usps: build address request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("usps: address request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("usps: read address response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var addrResp addressResponse
		if err := json.Unmarshal(body, &addrResp); err != nil {
			return nil, fmt.Errorf("usps: parse address response: %w", err)
		}

		return &StandardizedAddress{
			StreetAddress:    addrResp.Address.StreetAddress,
			SecondaryAddress: addrResp.Address.SecondaryAddress,
			City:             addrResp.Address.City,
			State:            addrResp.Address.State,
			ZIPCode:          addrResp.Address.ZIPCode,
			ZIPPlus4:         addrResp.Address.ZIPPlus4,
		}, nil
	}

	// Parse error response for user-friendly messages
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound {
		var errResp errorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, &ValidationError{Message: mapUSPSError(errResp.Error.Message)}
		}
		return nil, &ValidationError{Message: "We couldn't verify this address. Please check your entry and try again."}
	}

	return nil, fmt.Errorf("usps: address validation returned %d: %s", resp.StatusCode, string(body))
}

// mapUSPSError converts USPS error messages to user-friendly strings.
func mapUSPSError(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "address not found") || strings.Contains(lower, "no match"):
		return "We couldn't find this address. Please check the street address, city, and state."
	case strings.Contains(lower, "multiple addresses") || strings.Contains(lower, "more than one address"):
		return "Multiple addresses match your entry. Please add more detail, such as an apartment or unit number."
	case strings.Contains(lower, "insufficient"):
		return "Not enough address information provided. Please fill in all required fields."
	case strings.Contains(lower, "invalid delivery"):
		return "This doesn't appear to be a valid delivery address. Please check your entry."
	case strings.Contains(lower, "invalid") && strings.Contains(lower, "state"):
		return "The state code is invalid. Please use a two-letter state abbreviation (e.g., NC, TX)."
	case strings.Contains(lower, "invalid") && strings.Contains(lower, "city"):
		return "The city name is invalid. Please check your entry."
	case strings.Contains(lower, "city and state") && strings.Contains(lower, "unverifiable"):
		return "We couldn't verify this city and state combination. Please check your entry."
	default:
		return "We couldn't verify this address. Please check your entry and try again."
	}
}
