package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"christjesus/internal/db"
	"christjesus/internal/server"
	"christjesus/internal/store"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/sirupsen/logrus"
	supaauth "github.com/supabase-community/auth-go"
	"github.com/urfave/cli/v2"
)

var serveCommand = &cli.Command{
	Name:   "serve",
	Usage:  "Start the HTTP server",
	Action: serve,
}

func serve(cCtx *cli.Context) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	config, err := loadConfig(cCtx.String("env-prefix"))
	if err != nil {
		return err
	}

	supabaseAuthClient := supaauth.New(config.SupabaseProjectID, config.SupabaseAPIKey)

	pool, err := db.Connect(ctx, config)
	if err != nil {
		return err
	}
	defer pool.Close()

	formsStore := store.NewFormsRepository(pool)

	jwkCache, err := jwk.NewCache(context.Background(), httprc.NewClient())
	if err != nil {
		return fmt.Errorf("failed to initilaize jwk cache: %w", err)
	}

	supabaseJWKUrl := fmt.Sprintf("https://%s.supabase.co/auth/v1/.well-known/jwks.json", config.SupabaseProjectID)

	err = jwkCache.Register(context.Background(), supabaseJWKUrl)
	if err != nil {
		return fmt.Errorf("failed to register supabase jwk with cache: %w", err)
	}

	srv, err := server.New(
		config,
		logger,
		supabaseAuthClient,
		formsStore,
		jwkCache,
		supabaseJWKUrl,
	)
	if err != nil {
		return err
	}

	go func() {
		logger.WithField("port", config.ServerPort).Info("server starting")
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.WithError(err).Fatal("server failed")
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return srv.Stop(shutdownCtx)
}
