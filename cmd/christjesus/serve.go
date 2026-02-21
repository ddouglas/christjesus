package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"christjesus/internal/db"
	"christjesus/internal/server"
	"christjesus/internal/store"

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

	config, err := loadConfig(cCtx.String("env-prefix"))
	if err != nil {
		return err
	}

	pool, err := db.Connect(ctx, config)
	if err != nil {
		return err
	}
	defer pool.Close()

	formsStore := store.NewFormsRepository(pool)
	srv, err := server.New(config, logger, formsStore)
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
