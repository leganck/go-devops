package main

import (
	"context"

	"github.com/leganck/go-devops/devops"
	"github.com/leganck/go-devops/devops/deploy"
	"github.com/leganck/go-devops/devops/logs"
	"github.com/leganck/go-devops/devops/sql"
)

func newCore(cfg *config) (*devops.Client, error) {
	opts := []devops.Option{
		devops.WithBaseURL(cfg.BaseURL),
		devops.WithCredentials(devops.Credentials{Username: cfg.Username, Password: cfg.Password}),
		devops.WithLogger(sdkLog{}),
		devops.WithFreshLogin(cfg.FreshLogin),
	}
	return devops.New(opts...)
}

func loginCore(ctx context.Context, cfg *config) (*devops.Client, error) {
	core, err := newCore(cfg)
	if err != nil {
		return nil, err
	}
	if err := core.EnsureSession(ctx); err != nil {
		return nil, err
	}
	return core, nil
}

func deployAPI(ctx context.Context, cfg *config) (*deploy.Client, error) {
	core, err := loginCore(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return deploy.New(core, deploy.WithHandleStore(deploy.NewFileHandleStore(""))), nil
}

func sqlAPI(ctx context.Context, cfg *config) (*sql.Client, error) {
	core, err := loginCore(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return sql.New(core), nil
}

func logsAPI(ctx context.Context, cfg *config) (*logs.Client, error) {
	core, err := loginCore(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return logs.New(core), nil
}

func requireFlag(name, value string) error {
	if value == "" {
		return fmtRequire(name + " is required")
	}
	return nil
}

type flagError string

func (e flagError) Error() string { return string(e) }

func fmtRequire(msg string) error { return flagError(msg) }
