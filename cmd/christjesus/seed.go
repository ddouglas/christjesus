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
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:  "fake-needs",
			Value: 40,
			Usage: "Number of fake needs to generate for browse/filter development",
		},
		&cli.BoolFlag{
			Name:  "reset-fake-needs",
			Value: true,
			Usage: "Delete previously seeded fake needs (short_description starts with [seed]) before inserting",
		},
	},
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
		needRepo := store.NewNeedRepository(pool)
		assignmentRepo := store.NewAssignmentRepository(pool)
		storyRepo := store.NewStoryRepository(pool)
		userRepo := store.NewUserRepository(pool)

		// Seed categories
		logrus.Info("Seeding categories...")
		if err := seed.SeedCategories(ctx, categoryRepo); err != nil {
			return fmt.Errorf("failed to seed categories: %w", err)
		}

		logrus.Info("Categories seeded successfully")

		fakeNeedsCount := c.Int("fake-needs")
		resetFakeNeeds := c.Bool("reset-fake-needs")

		if fakeNeedsCount > 0 {
			logrus.Info("Seeding fake users...")
			if err := seed.SeedFakeUsers(ctx, userRepo); err != nil {
				return fmt.Errorf("failed to seed fake users: %w", err)
			}
			logrus.Info("Fake users seeded successfully")

			logrus.WithField("count", fakeNeedsCount).WithField("reset", resetFakeNeeds).Info("Seeding fake needs...")
			if err := seed.SeedFakeNeeds(ctx, pool, needRepo, categoryRepo, assignmentRepo, storyRepo, fakeNeedsCount, resetFakeNeeds); err != nil {
				return fmt.Errorf("failed to seed fake needs: %w", err)
			}
			logrus.Info("Fake needs seeded successfully")
		} else {
			logrus.Info("Skipping fake needs seed (fake-needs <= 0)")
		}

		return nil
	},
}
