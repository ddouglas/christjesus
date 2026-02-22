package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "christjesus",
		Usage: "Server-side rendered Go web app",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "env-prefix",
				Aliases: []string{"p"},
				Usage:   "Environment variable prefix",
				Value:   "APP",
			},
		},
		Commands: []*cli.Command{
			serveCommand,
			seedCommand,
			nanoidCommand,
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.WithError(err).Fatal("application failed")
	}
}
