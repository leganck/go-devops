package main

import (
	"context"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"
	"go-devops/internal/logger"
	"os"
)

// Version set at compile-time (e.g., go build -ldflags "-X main.Version=v1.2.3")
var Version = "dev"

func main() {
	// Initialize logger first
	logger.InitLogger(false) // Default to info level, will be updated based on flags

	// Load env-file if it exists
	if filename, found := os.LookupEnv("PLUGIN_ENV_FILE"); found {
		if err := godotenv.Load(filename); err != nil && !os.IsNotExist(err) {
			logger.Warningf("failed to load env file %s: %v", filename, err)
		}
	}
	if _, err := os.Stat("/run/drone/env"); err == nil {
		if err := godotenv.Overload("/run/drone/env"); err != nil {
			logger.Warningf("failed to load /run/drone/env: %v", err)
		}
	}

	app := cli.NewApp()
	app.Usage = "login to DevOps, query deploy versions, and deploy programs"
	app.Copyright = "Copyright (c) 2025"
	app.Authors = []*cli.Author{
		{
			Name:  "leganck",
			Email: "leganck@outlook.com",
		},
	}
	app.Action = run
	app.Version = Version
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "host",
			Usage:   "DevOps base URL",
			Value:   "https://devops.example.com",
			EnvVars: []string{"PLUGIN_URL", "DEVOPS_URL", "URL"},
		},
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "DevOps username",
			EnvVars: []string{"PLUGIN_USERNAME", "DEVOPS_USERNAME", "USERNAME"},
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p"},
			Usage:   "DevOps password",
			EnvVars: []string{"PLUGIN_PASSWORD", "DEVOPS_PASSWORD", "PASSWORD"},
		},
		&cli.StringFlag{
			Name:    "program-alias",
			Usage:   "Program alias name (e.g. smartpos-svc-erp-chain)",
			Value:   "smartpos-svc-erp-chain",
			EnvVars: []string{"PLUGIN_PROGRAM_ALIAS", "DEVOPS_PROGRAM_ALIAS", "PROGRAM_ALIAS"},
		},
		&cli.StringFlag{
			Name:    "program-type",
			Usage:   "Program type (e.g. snapshots, releases)",
			Value:   "snapshots",
			EnvVars: []string{"PLUGIN_PROGRAM_TYPE", "DEVOPS_PROGRAM_TYPE", "PROGRAM_TYPE"},
		},
		&cli.StringFlag{
			Name:    "env",
			Usage:   "Environment name (e.g. dev2, test)",
			Value:   "dev2",
			EnvVars: []string{"PLUGIN_ENV", "DEVOPS_ENV", "ENV"},
		},
		&cli.StringFlag{
			Name:    "project-version",
			Usage:   "Project version to check or deploy (e.g. 1.0.0)",
			EnvVars: []string{"PLUGIN_PROJECT_VERSION", "DEVOPS_PROJECT_VERSION", "PROJECT_VERSION"},
		},
		&cli.StringFlag{
			Name:    "server",
			Usage:   "Server alias to check or deploy to (e.g. dev2-zd1-erp-chain)",
			EnvVars: []string{"PLUGIN_SERVER", "DEVOPS_SERVER", "SERVER"},
		},

		&cli.StringFlag{
			Name:    "notify-user",
			Usage:   "Users to notify on deployment",
			EnvVars: []string{"PLUGIN_NOTIFY_USER", "DEVOPS_NOTIFY_USER", "NOTIFY_USER"},
		},
		&cli.BoolFlag{
			Name:    "debug",
			Usage:   "enable debug mode to show detailed information",
			EnvVars: []string{"PLUGIN_DEBUG", "DEVOPS_DEBUG", "DEBUG"},
		},
		&cli.BoolFlag{
			Name:    "wait",
			Usage:   "wait for deployment to complete",
			EnvVars: []string{"PLUGIN_WAIT", "DEVOPS_WAIT", "WAIT"},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Fatalf("%v", err)
	}
}

// run is the CLI entrypoint
func run(c *cli.Context) error {
	// Reinitialize logger with debug flag from CLI
	logger.InitLogger(c.Bool("debug"))

	plugin := &Plugin{
		BaseURL:        c.String("host"),
		Username:       c.String("username"),
		Password:       c.String("password"),
		ProgramAlias:   c.String("program-alias"),
		ProgramType:    c.String("program-type"),
		Env:            c.String("env"),
		ProjectVersion: c.String("project-version"),
		Server:         c.String("server"),
		NotifyUser:     c.String("notify-user"),
		Debug:          c.Bool("debug"),
		Wait:           c.Bool("wait"),
	}

	// Create a context that listens for system signals to allow graceful shutdown
	ctx, cancel := context.WithCancel(c.Context)
	defer cancel()

	return plugin.Exec(ctx)
}
