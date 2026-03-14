package main

import (
	"context"
	"fmt"
	"strings"

	"christjesus/pkg/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/kelseyhightower/envconfig"
)

func loadConfig() (*types.Config, error) {
	c := new(types.Config)
	if err := envconfig.Process("", c); err != nil {
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

	if strings.TrimSpace(c.Auth0Domain) == "" {
		return nil, fmt.Errorf("set AUTH0_DOMAIN")
	}
	if strings.TrimSpace(c.Auth0ClientID) == "" {
		return nil, fmt.Errorf("set AUTH0_CLIENT_ID")
	}
	if strings.TrimSpace(c.Auth0ClientSecret) == "" {
		return nil, fmt.Errorf("set AUTH0_CLIENT_SECRET")
	}
	domain := strings.TrimSpace(c.Auth0Domain)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	c.AuthIssuerURL = "https://" + domain + "/"
	c.AuthClientID = strings.TrimSpace(c.Auth0ClientID)

	return c, nil
}

func loadAWSConfig(ctx context.Context) (aws.Config, error) {
	config, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load aws config: %w", err)
	}

	return config, nil
}
