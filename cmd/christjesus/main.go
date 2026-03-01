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
