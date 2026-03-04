package cmd

import (
	"context"
	"go-devops/devops"
	"go-devops/internal/errors"
)

// createClient 创建 DevOps 客户端实例。
//
// 该函数使用提供的配置创建一个新的 DevOps 客户端。
// 配置包括认证信息、API 基础 URL 和调试模式设置。
//
// 参数：
//   - cfg: 包含连接和认证信息的配置结构体
//
// 返回：
//   - *devops.DevOps: 初始化后的 DevOps 客户端实例
//   - error: 创建客户端失败时返回错误
func createClient(cfg *Config) (*devops.DevOps, error) {
	return devops.NewDevOps(&devops.Auth{
		Username: cfg.Username,
		Password: cfg.Password,
	}, cfg.BaseURL, cfg.Debug)
}

// createAndLoginClient 创建 DevOps 客户端并执行登录。
//
// 这是一个辅助函数，封装了客户端创建和登录的常见操作模式，
// 用于减少各子命令中的重复代码。
//
// 该函数会：
//  1. 使用提供的配置创建 DevOps 客户端
//  2. 执行登录操作以获取认证令牌
//  3. 缓存用户的权限信息
//
// 参数：
//   - ctx: 用于控制请求生命周期的上下文
//   - cfg: 包含连接和认证信息的配置结构体
//
// 返回：
//   - *devops.DevOps: 已登录的 DevOps 客户端实例
//   - error: 创建客户端或登录失败时返回错误（已包装）
//
// 错误处理：
//   - 客户端创建失败返回 APIError
//   - 登录失败返回 LoginError
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
