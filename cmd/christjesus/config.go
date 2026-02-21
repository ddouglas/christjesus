package main

import (
	"fmt"

	"christjesus/pkg/types"

	"github.com/kelseyhightower/envconfig"
)

func loadConfig(prefix string) (*types.Config, error) {
	c := new(types.Config)
	if err := envconfig.Process(prefix, c); err != nil {
		return nil, fmt.Errorf("process environment config: %w", err)
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("set DATABASE_URL")
	}

	if c.ServerPort == 0 {
		c.ServerPort = 8080
	}

	if c.ReadTimeoutSec == 0 {
		c.ReadTimeoutSec = 10
	}

	if c.WriteTimeoutSec == 0 {
		c.WriteTimeoutSec = 15
	}

	return c, nil
}
