package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"

	"github.com/leganck/go-devops/internal/logger"
)

type config struct {
	BaseURL    string
	Username   string
	Password   string
	Debug      bool
	FreshLogin bool
}

func newApp() *cli.App {
	return &cli.App{
		Name:      "go-devops",
		Usage:     "DevOps SDK CLI",
		Version:   version,
		Copyright: "Copyright (c) 2026",
		Authors:   []*cli.Author{{Name: "leganck", Email: "leganck@outlook.com"}},
		Before:    loadEnv,
		Flags:     globalFlags(),
		Commands: []*cli.Command{
			deployCommand(),
			versionCommand(),
			serversCommand(),
			programsCommand(),
			envsCommand(),
			sqlCommand(),
			logsCommand(),
			historyCommand(),
			logoutCommand(),
		},
	}
}

func loadEnv(c *cli.Context) error {
	logger.InitLogger(c.Bool("debug"))
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
	return nil
}

func globalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "host",
			Aliases: []string{"H"},
			Usage:   "DevOps API base URL (required)",
			EnvVars: []string{"DEVOPS_URL"},
		},
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "username",
			EnvVars: []string{"DEVOPS_USERNAME"},
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p"},
			Usage:   "password",
			EnvVars: []string{"DEVOPS_PASSWORD"},
		},
		&cli.BoolFlag{
			Name:    "debug",
			Usage:   "debug logs",
			EnvVars: []string{"DEVOPS_DEBUG"},
		},
		&cli.BoolFlag{
			Name:    "fresh-login",
			Usage:   "ignore local session",
			EnvVars: []string{"DEVOPS_FRESH_LOGIN"},
		},
	}
}

func getConfig(c *cli.Context) (*config, error) {
	cfg := &config{
		BaseURL:    c.String("host"),
		Username:   c.String("username"),
		Password:   c.String("password"),
		Debug:      c.Bool("debug"),
		FreshLogin: c.Bool("fresh-login"),
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("DEVOPS_URL / --host is required")
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	return cfg, nil
}

type sdkLog struct{}

func (sdkLog) Debug(msg string, kv ...any) { logger.Debugf("%s%s", msg, kvFmt(kv)) }
func (sdkLog) Info(msg string, kv ...any)  { logger.Infof("%s%s", msg, kvFmt(kv)) }
func (sdkLog) Warn(msg string, kv ...any)  { logger.Warningf("%s%s", msg, kvFmt(kv)) }
func (sdkLog) Error(msg string, kv ...any) { logger.Errorf("%s%s", msg, kvFmt(kv)) }

func kvFmt(kv []any) string {
	if len(kv) == 0 {
		return ""
	}
	s := ""
	for i := 0; i+1 < len(kv); i += 2 {
		s += fmt.Sprintf(" %v=%v", kv[i], kv[i+1])
	}
	return s
}
