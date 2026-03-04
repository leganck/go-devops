package cmd

import (
	"go-devops/devops"
)

// createClient 创建 DevOps 客户端
func createClient(cfg *Config) (*devops.DevOps, error) {
	return devops.NewDevOps(&devops.Auth{
		Username: cfg.Username,
		Password: cfg.Password,
	}, cfg.BaseURL, cfg.Debug)
}
