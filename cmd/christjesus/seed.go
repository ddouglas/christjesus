package main

import (
	"christjesus/internal/db"
	"christjesus/internal/seed"
	"christjesus/internal/store"
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var seedCommand = &cli.Command{
	Name:  "seed",
	Usage: "Seed the database with initial data",
	Action: func(c *cli.Context) error {
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		ctx := context.Background()

		// Connect to database
		pool, err := db.Connect(ctx, cfg)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer pool.Close()

		logrus.Info("Connected to database")

		// Create category repository
		categoryRepo := store.NewCategoryRepository(pool)

		// Seed categories
		logrus.Info("Seeding categories...")
		if err := seed.SeedCategories(ctx, categoryRepo); err != nil {
			return fmt.Errorf("failed to seed categories: %w", err)
		}

		logrus.Info("Categories seeded successfully")

		return nil
	},
}
