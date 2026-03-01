package types

type Config struct {
	Environment     string `envconfig:"ENVIRONMENT" default:"development"`
	ServerPort      uint   `envconfig:"SERVER_PORT" default:"8080"`
	DatabaseURL     string `envconfig:"DATABASE_URL"`
	ReadTimeoutSec  uint   `envconfig:"READ_TIMEOUT_SEC" default:"10"`
	WriteTimeoutSec uint   `envconfig:"WRITE_TIMEOUT_SEC" default:"15"`

	// Cognito Auth
	CognitoUserPoolID string `envconfig:"COGNITO_USER_POOL_ID"`
	CognitoClientID   string `envconfig:"COGNITO_CLIENT_ID"`
	CognitoIssuerURL  string `envconfig:"COGNITO_ISSUER_URL"`

	// Supabase Auth
	// SupabaseJWTSecret string `envconfig:"SUPABASE_JWT_SECRET"`

	// Supabase Storage
	// StorageBucketName string `envconfig:"STORAGE_BUCKET_NAME" default:"need-documents"`

	// Auth Configuration
	CookieName       string `envconfig:"SESSION_COOKIE_NAME" default:"session_id"`
	SessionMaxAgeSec int    `envconfig:"SESSION_MAX_AGE_SEC" default:"604800"` // 7 days

	// Cookie encryption keys (base64 encoded)
	// openssl rand -base64 32
	// to generate values
	CookieHashKey  string `envconfig:"COOKIE_HASH_KEY"`  // 32 or 64 bytes
	CookieBlockKey string `envconfig:"COOKIE_BLOCK_KEY"` // 16, 24, or 32 bytes
}
