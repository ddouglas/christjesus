package main

import (
	"christjesus/internal/utils"
	"fmt"

	"github.com/urfave/cli/v2"
)

var nanoidCommand = &cli.Command{
	Name:  "nanoid",
	Usage: "Generate NanoIDs for use in seed files",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:    "count",
			Aliases: []string{"c"},
			Usage:   "Number of IDs to generate",
			Value:   1,
		},
	},
	Action: func(c *cli.Context) error {
		count := c.Int("count")
		for range count {
			fmt.Println(utils.NanoID())
		}
		return nil
	},
}
