package main

import (
	"christjesus/internal/db"
	"christjesus/internal/store"
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var debugBrowseCommand = &cli.Command{
	Name:  "debug-browse",
	Usage: "Test browse needs queries against the database",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "zip",
			Value: "",
			Usage: "Zip code for geo search",
		},
		&cli.Float64Flag{
			Name:  "radius",
			Value: 25,
			Usage: "Radius in miles",
		},
	},
	Action: func(c *cli.Context) error {
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		ctx := context.Background()
		pool, err := db.Connect(ctx, cfg)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer pool.Close()

		logrus.Info("Connected to database")

		// Test 1: Check PostGIS availability
		var postgisVersion string
		err = pool.QueryRow(ctx, "SELECT PostGIS_Version()").Scan(&postgisVersion)
		if err != nil {
			logrus.WithError(err).Error("PostGIS not available via unqualified call")
		} else {
			logrus.WithField("version", postgisVersion).Info("PostGIS version")
		}

		// Test 1b: Try schema-qualified calls to find where PostGIS lives
		for _, schema := range []string{"christjesus", "extensions", "public"} {
			var v string
			q := fmt.Sprintf("SELECT %s.PostGIS_Version()", schema)
			err = pool.QueryRow(ctx, q).Scan(&v)
			if err != nil {
				logrus.WithField("schema", schema).WithError(err).Debug("PostGIS not in this schema")
			} else {
				logrus.WithField("schema", schema).WithField("version", v).Info("PostGIS found")
			}
		}

		// Test 1c: Check where the extension is registered
		var extSchema, extVersion string
		err = pool.QueryRow(ctx,
			"SELECT n.nspname, e.extversion FROM pg_extension e JOIN pg_namespace n ON n.oid = e.extnamespace WHERE e.extname = 'postgis'",
		).Scan(&extSchema, &extVersion)
		if err != nil {
			logrus.WithError(err).Error("PostGIS extension not found in pg_extension")
		} else {
			logrus.WithFields(logrus.Fields{"schema": extSchema, "version": extVersion}).Info("PostGIS extension registered in")
		}

		// Test 2: Check search_path
		var searchPath string
		err = pool.QueryRow(ctx, "SHOW search_path").Scan(&searchPath)
		if err != nil {
			logrus.WithError(err).Error("Failed to get search_path")
		} else {
			logrus.WithField("search_path", searchPath).Info("Current search_path")
		}

		// Test 3: Count browsable needs
		var activeCount int
		err = pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM christjesus.needs WHERE status IN ('ACTIVE','FUNDED') AND deleted_at IS NULL",
		).Scan(&activeCount)
		if err != nil {
			logrus.WithError(err).Error("Failed to count active needs")
		} else {
			logrus.WithField("count", activeCount).Info("Active/funded needs")
		}

		// Test 4: Count zip centroids
		var zipCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM christjesus.zip_centroids").Scan(&zipCount)
		if err != nil {
			logrus.WithError(err).Error("Failed to count zip centroids")
		} else {
			logrus.WithField("count", zipCount).Info("Zip centroids loaded")
		}

		// Test 5: Test raw ST_Distance query
		zip := c.String("zip")
		if zip != "" {
			logrus.WithField("zip", zip).Info("Testing geo query")

			var testDist *float64
			err = pool.QueryRow(ctx,
				"SELECT ST_Distance(zc.geog, (SELECT geog FROM christjesus.zip_centroids WHERE zip_code = $1)) / 1609.344 FROM christjesus.zip_centroids zc WHERE zc.zip_code = $2",
				zip, "28202",
			).Scan(&testDist)
			if err != nil {
				logrus.WithError(err).Error("Raw ST_Distance query failed")
			} else if testDist != nil {
				logrus.WithField("distance_miles", fmt.Sprintf("%.2f", *testDist)).Info("Distance from zip to 28202")
			}
		}

		// Test 6: Run the actual browse query via the repository
		needRepo := store.NewNeedRepository(pool)
		radius := c.Float64("radius")
		filter := store.BrowseNeedsFilter{
			ZipCode:    zip,
			FundingMax: 100,
			Page:       1,
			PageSize:   10,
		}
		if zip != "" {
			filter.RadiusMiles = &radius
		}

		logrus.WithField("filter", fmt.Sprintf("%+v", filter)).Info("Running BrowseNeedsCount")
		count, err := needRepo.BrowseNeedsCount(ctx, filter)
		if err != nil {
			logrus.WithError(err).Error("BrowseNeedsCount failed")
		} else {
			logrus.WithField("count", count).Info("BrowseNeedsCount result")
		}

		// Test 7: Check if zip centroids join resolves for browsable needs
		type joinCheck struct {
			NeedID  string  `db:"need_id"`
			ZipCode *string `db:"zip_code"`
			HasGeog bool    `db:"has_geog"`
		}
		checkRows, err := pool.Query(ctx, `
			SELECT n.id AS need_id,
			       COALESCE(sa.zip_code, pa.zip_code) AS zip_code,
			       zc.geog IS NOT NULL AS has_geog
			FROM christjesus.needs n
			LEFT JOIN christjesus.user_addresses sa ON sa.id = n.user_address_id
			LEFT JOIN christjesus.user_addresses pa ON pa.user_id = n.user_id AND pa.is_primary = true AND n.user_address_id IS NULL
			LEFT JOIN christjesus.zip_centroids zc ON zc.zip_code = COALESCE(sa.zip_code, pa.zip_code)
			WHERE n.status IN ('ACTIVE','FUNDED') AND n.deleted_at IS NULL
			LIMIT 5
		`)
		if err != nil {
			logrus.WithError(err).Error("Join check query failed")
		} else {
			defer checkRows.Close()
			for checkRows.Next() {
				var needID string
				var zipCode *string
				var hasGeog bool
				if err := checkRows.Scan(&needID, &zipCode, &hasGeog); err != nil {
					logrus.WithError(err).Error("Scan failed")
					break
				}
				zc := "NULL"
				if zipCode != nil {
					zc = *zipCode
				}
				logrus.WithFields(logrus.Fields{
					"need_id":  needID,
					"zip_code": zc,
					"has_geog": hasGeog,
				}).Info("Join check")
			}
		}

		logrus.Info("Running BrowseNeedsPage")
		rows, err := needRepo.BrowseNeedsPage(ctx, filter)
		if err != nil {
			logrus.WithError(err).Error("BrowseNeedsPage failed")
		} else {
			logrus.WithField("rows", len(rows)).Info("BrowseNeedsPage result")
			for i, row := range rows {
				dist := "N/A"
				if row.DistanceMiles != nil {
					dist = fmt.Sprintf("%.2f mi", *row.DistanceMiles)
				}
				logrus.WithFields(logrus.Fields{
					"#":        i + 1,
					"id":       row.ID,
					"status":   row.Status,
					"amount":   row.AmountNeededCents,
					"distance": dist,
				}).Info("Need")
			}
		}

		return nil
	},
}
