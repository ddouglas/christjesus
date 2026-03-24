package types

type Config struct {
	Environment     string `envconfig:"ENVIRONMENT" default:"development"`
	ServerPort      uint   `envconfig:"SERVER_PORT" default:"8080"`
	AppBaseURL      string `envconfig:"APP_BASE_URL" default:"http://localhost:8080"`
	DatabaseURL     string `envconfig:"DATABASE_URL" required:"true"`
	ReadTimeoutSec  uint   `envconfig:"READ_TIMEOUT_SEC" default:"10"`
	WriteTimeoutSec uint   `envconfig:"WRITE_TIMEOUT_SEC" default:"15"`

	// Stripe Payments
	StripeSecretKey      string `envconfig:"STRIPE_SECRET_KEY"`
	StripePublishableKey string `envconfig:"STRIPE_PUBLISHABLE_KEY"`
	StripeWebhookSecret  string `envconfig:"STRIPE_WEBHOOK_SECRET"`

	// Auth settings (derived from Auth0Domain/Auth0ClientID in loadConfig)
	AuthIssuerURL  string `envconfig:"-"`
	AuthClientID   string `envconfig:"-"`
	AuthAdminClaim       string `envconfig:"AUTH_ADMIN_CLAIM" default:"https://christjesus.app/claims/roles"`
	AuthAdminValue       string `envconfig:"AUTH_ADMIN_VALUE" default:"admin"`
	AuthDisplayNameClaim string `envconfig:"AUTH_DISPLAY_NAME_CLAIM" default:"https://christjesus.app/claims/display_name"`

	// Auth0 settings
	Auth0Domain       string `envconfig:"AUTH0_DOMAIN"`
	Auth0ClientID     string `envconfig:"AUTH0_CLIENT_ID"`
	Auth0ClientSecret string `envconfig:"AUTH0_CLIENT_SECRET"`
	Auth0Audience     string `envconfig:"AUTH0_AUDIENCE"`
	Auth0CallbackURL  string `envconfig:"AUTH0_CALLBACK_URL" default:"http://localhost:8080/auth/callback"`
	Auth0LogoutURL    string `envconfig:"AUTH0_LOGOUT_URL" default:"http://localhost:8080/"`

	// Auth0 Management API (M2M application credentials for profile updates)
	Auth0MgmtClientID     string `envconfig:"AUTH0_MGMT_CLIENT_ID"`
	Auth0MgmtClientSecret string `envconfig:"AUTH0_MGMT_CLIENT_SECRET"`

	// S3-compatible object storage (Tigris)
	S3BucketName         string `envconfig:"S3_BUCKET_NAME" required:"true"`
	ObjectStoreEndpoint  string `envconfig:"OBJECT_STORE_ENDPOINT" default:"https://t3.storage.dev"`
	ObjectStoreRegion    string `envconfig:"OBJECT_STORE_REGION" default:"auto"`
	ObjectStorePathStyle bool   `envconfig:"OBJECT_STORE_PATH_STYLE" default:"false"`
	TigrisAccessKey      string `envconfig:"TIGRIS_ACCESS_KEY"`
	TigrisSecretKey      string `envconfig:"TIGRIS_SECRET_KEY"`

	// Auth Configuration
	CookieName       string `envconfig:"SESSION_COOKIE_NAME" default:"session_id"`
	SessionMaxAgeSec int    `envconfig:"SESSION_MAX_AGE_SEC" default:"604800"` // 7 days

	// Cookie encryption keys (base64 encoded)
	// openssl rand -base64 32
	// to generate values
	CookieHashKey  string `envconfig:"COOKIE_HASH_KEY" required:"true"`  // 32 or 64 bytes
	CookieBlockKey string `envconfig:"COOKIE_BLOCK_KEY" required:"true"` // 16, 24, or 32 bytes
}
