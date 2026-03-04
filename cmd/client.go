package cmd

import (
	"context"
	"go-devops/devops"
	"go-devops/internal/errors"
)

// createClient 创建 DevOps 客户端
func createClient(cfg *Config) (*devops.DevOps, error) {
	return devops.NewDevOps(&devops.Auth{
		Username: cfg.Username,
		Password: cfg.Password,
	}, cfg.BaseURL, cfg.Debug)
}

// createAndLoginClient 创建 DevOps 客户端并执行登录
// 这是一个辅助函数，用于减少子命令中的重复代码
func createAndLoginClient(ctx context.Context, cfg *Config) (*devops.DevOps, error) {
	// 创建 DevOps 客户端
	client, err := createClient(cfg)
	if err != nil {
		return nil, errors.NewAPIError("创建 DevOps 客户端失败", err)
	}

	// 登录
	if err := client.Login(ctx); err != nil {
		return nil, errors.NewLoginError("登录失败", err)
	}

	return client, nil
}
