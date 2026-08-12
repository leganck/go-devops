package cmd

import (
	"context"
	"go-devops/devops"
	"go-devops/internal/errors"
)

// createClient 创建 DevOps 客户端实例。
func createClient(cfg *Config) (*devops.DevOps, error) {
	client, err := devops.NewDevOps(&devops.Auth{
		Username: cfg.Username,
		Password: cfg.Password,
	}, cfg.BaseURL, cfg.Debug)
	if err != nil {
		return nil, err
	}
	client.FreshLogin = cfg.FreshLogin
	return client, nil
}

// createAndLoginClient 创建客户端并确保会话（优先复用本地会话）。
func createAndLoginClient(ctx context.Context, cfg *Config) (*devops.DevOps, error) {
	client, err := createClient(cfg)
	if err != nil {
		return nil, errors.NewAPIError("创建 DevOps 客户端失败", err)
	}

	if err := client.EnsureSession(ctx); err != nil {
		return nil, errors.NewLoginError("登录失败", err)
	}

	return client, nil
}
