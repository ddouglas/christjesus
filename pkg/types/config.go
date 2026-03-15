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

	// Auth settings
	AuthIssuerURL  string `envconfig:"AUTH_ISSUER_URL"`
	AuthClientID   string `envconfig:"AUTH_CLIENT_ID"`
	AuthAdminClaim string `envconfig:"AUTH_ADMIN_CLAIM" default:"https://christjesus.app/claims/roles"`
	AuthAdminValue string `envconfig:"AUTH_ADMIN_VALUE" default:"admin"`

	// Auth0 settings
	Auth0Domain       string `envconfig:"AUTH0_DOMAIN"`
	Auth0ClientID     string `envconfig:"AUTH0_CLIENT_ID"`
	Auth0ClientSecret string `envconfig:"AUTH0_CLIENT_SECRET"`
	Auth0Audience     string `envconfig:"AUTH0_AUDIENCE"`
	Auth0CallbackURL  string `envconfig:"AUTH0_CALLBACK_URL" default:"http://localhost:8080/auth/callback"`
	Auth0LogoutURL    string `envconfig:"AUTH0_LOGOUT_URL" default:"http://localhost:8080/"`

	// AWS S3
	S3BucketName string `envconfig:"S3_BUCKET_NAME" required:"true"`

	// Auth Configuration
	CookieName       string `envconfig:"SESSION_COOKIE_NAME" default:"session_id"`
	SessionMaxAgeSec int    `envconfig:"SESSION_MAX_AGE_SEC" default:"604800"` // 7 days

	// Cookie encryption keys (base64 encoded)
	// openssl rand -base64 32
	// to generate values
	CookieHashKey  string `envconfig:"COOKIE_HASH_KEY" required:"true"`  // 32 or 64 bytes
	CookieBlockKey string `envconfig:"COOKIE_BLOCK_KEY" required:"true"` // 16, 24, or 32 bytes
}
