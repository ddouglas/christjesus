package types

type Config struct {
	Environment     string `envconfig:"ENVIRONMENT" default:"development"`
	ServerPort      uint   `envconfig:"SERVER_PORT" default:"8080"`
	DatabaseURL     string `envconfig:"DATABASE_URL"`
	ReadTimeoutSec  uint   `envconfig:"READ_TIMEOUT_SEC" default:"10"`
	WriteTimeoutSec uint   `envconfig:"WRITE_TIMEOUT_SEC" default:"15"`
}
