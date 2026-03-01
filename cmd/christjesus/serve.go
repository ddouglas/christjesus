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

	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/sirupsen/logrus"
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

	config, err := loadConfig()
	if err != nil {
		return err
	}

	awsConfig, err := loadAWSConfig(ctx)
	if err != nil {
		return err
	}

	cognitoClient := cognitoidentityprovider.NewFromConfig(awsConfig)
	s3Client := s3.NewFromConfig(awsConfig)

	pool, err := db.Connect(ctx, config)
	if err != nil {
		return err
	}
	defer pool.Close()

	needsRepo := store.NewNeedRepository(pool)
	progressRepo := store.NewNeedProgressRepository(pool)
	categoryRepo := store.NewCategoryRepository(pool)
	needCategoryAssignmentsRepo := store.NewAssignmentRepository(pool)
	storyRepo := store.NewStoryRepository(pool)
	documentRepo := store.NewDocumentRepository(pool)

	jwkCache, err := jwk.NewCache(context.Background(), httprc.NewClient())
	if err != nil {
		return fmt.Errorf("failed to initilaize jwk cache: %w", err)
	}

	jwksURL := fmt.Sprintf("%s/.well-known/jwks.json", config.CognitoIssuerURL)

	err = jwkCache.Register(context.Background(), jwksURL)
	if err != nil {
		return fmt.Errorf("failed to register supabase jwk with cache: %w", err)
	}

	srv, err := server.New(
		config,
		logger,
		cognitoClient,
		s3Client,
		needsRepo,
		progressRepo,
		categoryRepo,
		needCategoryAssignmentsRepo,
		storyRepo,
		documentRepo,
		jwkCache,
		jwksURL,
	)
	if err != nil {
		return err
	}

	go func() {
		logger.WithField("port", config.ServerPort).Infof("server starting http://localhost:%d", config.ServerPort)
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
