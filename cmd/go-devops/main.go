package main

import (
	"fmt"
	"os"

	"github.com/leganck/go-devops/internal/logger"
	"github.com/urfave/cli/v2"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func init() {
	cli.VersionFlag = &cli.BoolFlag{
		Name:               "version",
		Usage:              "print CLI version",
		DisableDefaultText: true,
	}
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Fprintf(c.App.Writer, "go-devops %s (%s)\n", version, buildTime)
	}
}

func main() {
	app := newApp()
	if err := app.Run(os.Args); err != nil {
		logger.Errorf("%v", err)
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}
