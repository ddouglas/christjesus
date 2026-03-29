package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"

	"christjesus/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const censusZCTAURL = "https://www2.census.gov/geo/docs/maps-data/data/gazetteer/2024_Gazetteer/2024_Gaz_zcta_national.zip"

var importZipsCommand = &cli.Command{
	Name:  "import-zips",
	Usage: "Download US Census ZCTA gazetteer and populate the zip_centroids table",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "url",
			Value: censusZCTAURL,
			Usage: "URL of the Census ZCTA gazetteer ZIP archive",
		},
	},
	Action: importZips,
}

func importZips(cCtx *cli.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	ctx := cCtx.Context

	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	url := cCtx.String("url")
	logrus.WithField("url", url).Info("downloading ZCTA gazetteer archive")

	rows, err := downloadAndParseZCTA(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to download and parse ZCTA data: %w", err)
	}

	logrus.WithField("rows", len(rows)).Info("parsed ZCTA centroids")

	if err := loadZipCentroids(ctx, pool, rows); err != nil {
		return fmt.Errorf("failed to load zip centroids: %w", err)
	}

	logrus.WithField("rows", len(rows)).Info("zip_centroids table populated")
	return nil
}

type zipCentroid struct {
	ZipCode   string
	Latitude  string
	Longitude string
}

func downloadAndParseZCTA(ctx context.Context, url string) ([]zipCentroid, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}

	for _, f := range zipReader.File {
		if !strings.HasSuffix(f.Name, ".txt") {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s in archive: %w", f.Name, err)
		}
		defer rc.Close()

		return parseZCTAFile(rc)
	}

	return nil, fmt.Errorf("no .txt file found in archive")
}

func parseZCTAFile(r io.Reader) ([]zipCentroid, error) {
	reader := csv.NewReader(r)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colIndex := make(map[string]int, len(header))
	for i, name := range header {
		colIndex[strings.TrimSpace(name)] = i
	}

	geoidIdx, ok := colIndex["GEOID"]
	if !ok {
		return nil, fmt.Errorf("missing GEOID column")
	}
	latIdx, ok := colIndex["INTPTLAT"]
	if !ok {
		return nil, fmt.Errorf("missing INTPTLAT column")
	}
	lngIdx, ok := colIndex["INTPTLONG"]
	if !ok {
		return nil, fmt.Errorf("missing INTPTLONG column")
	}

	var rows []zipCentroid
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}

		zipCode := strings.TrimSpace(record[geoidIdx])
		lat := strings.TrimSpace(record[latIdx])
		lng := strings.TrimSpace(record[lngIdx])

		if zipCode == "" || lat == "" || lng == "" {
			continue
		}

		rows = append(rows, zipCentroid{
			ZipCode:   zipCode,
			Latitude:  lat,
			Longitude: lng,
		})
	}

	return rows, nil
}

func loadZipCentroids(ctx context.Context, pool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}, rows []zipCentroid) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "TRUNCATE christjesus.zip_centroids"); err != nil {
		return fmt.Errorf("truncate zip_centroids: %w", err)
	}

	copyCount, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"christjesus", "zip_centroids"},
		[]string{"zip_code", "latitude", "longitude"},
		pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
			return []any{rows[i].ZipCode, rows[i].Latitude, rows[i].Longitude}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("copy into zip_centroids: %w", err)
	}

	logrus.WithField("copied", copyCount).Info("COPY complete, populating geography column")

	if _, err := tx.Exec(ctx, `
		UPDATE christjesus.zip_centroids
		SET geog = ST_SetSRID(ST_MakePoint(longitude::float8, latitude::float8), 4326)::geography
		WHERE geog IS NULL
	`); err != nil {
		return fmt.Errorf("populate geog column: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	logrus.WithField("copied", copyCount).Info("zip_centroids table populated with geography data")
	return nil
}
