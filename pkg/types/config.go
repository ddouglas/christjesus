package types

type Config struct {
	Environment     string `envconfig:"ENVIRONMENT" default:"development"`
	ServerPort      uint   `envconfig:"SERVER_PORT" default:"8080"`
	DatabaseURL     string `envconfig:"DATABASE_URL" required:"true"`
	ReadTimeoutSec  uint   `envconfig:"READ_TIMEOUT_SEC" default:"10"`
	WriteTimeoutSec uint   `envconfig:"WRITE_TIMEOUT_SEC" default:"15"`

	// Cognito Auth
	CognitoUserPoolID string `envconfig:"COGNITO_USER_POOL_ID" required:"true"`
	CognitoClientID   string `envconfig:"COGNITO_CLIENT_ID" required:"true"`
	CognitoIssuerURL  string `envconfig:"COGNITO_ISSUER_URL" required:"true"`

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
