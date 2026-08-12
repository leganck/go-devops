// Package cmd 提供 DevOps 部署和查询工具的 CLI 命令实现。
//
// 该包包含以下子命令：
//   - deploy: 部署程序到指定服务器
//   - version: 查询程序可用版本
//   - servers: 查询程序可用服务器
//   - programs: 查询环境中的程序列表
//   - envs: 查询可用的环境列表
//   - sql: 通过 DevOps 数据源执行 SQL 查询与元数据浏览
//
// 全局配置选项：
//   - host (-H): DevOps API 基础 URL
//   - username (-u): DevOps 用户名
//   - password (-p): DevOps 密码
//   - debug: 启用调试模式
//
// 环境变量支持：
//   配置可以通过以下环境变量设置：
//   - DEVOPS_URL / URL: API 基础 URL
//   - DEVOPS_USERNAME / USERNAME: 用户名
//   - DEVOPS_PASSWORD / PASSWORD: 密码
//   - DEVOPS_DEBUG / DEBUG: 调试模式
//   - PLUGIN_ENV_FILE: 自定义 .env 文件路径
//
// 使用示例：
//
//	// 查询环境列表
//	go-devops envs -u username -p password
//
//	// 部署程序
//	go-devops deploy -a smartpos-svc -e dev2 -v 1.0.0 -s server1
package cmd

import (
	"go-devops/internal/logger"
	"os"

	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"
)

// Config 包含所有子命令共享的配置
type Config struct {
	BaseURL  string
	Username string
	Password string
	Debug    bool
}

// NewApp 创建并配置 CLI 应用程序
func NewApp() *cli.App {
	return &cli.App{
		Name:     "go-devops",
		Usage:    "DevOps 部署和查询工具",
		Copyright: "Copyright (c) 2025",
		Authors: []*cli.Author{
			{
				Name:  "leganck",
				Email: "leganck@outlook.com",
			},
		},
		Before: loadEnvironmentFiles,
		Flags:  globalFlags(),
		Commands: []*cli.Command{
			deployCommand(),
			versionCommand(),
			serversCommand(),
			programsCommand(),
			envsCommand(),
			sqlCommand(),
		},
	}
}

// loadEnvironmentFiles 从各种来源加载环境配置
func loadEnvironmentFiles(c *cli.Context) error {
	// 初始化日志记录器
	logger.InitLogger(c.Bool("debug"))

	// 如果指定了自定义 env 文件则加载
	if filename, found := os.LookupEnv("PLUGIN_ENV_FILE"); found {
		if err := godotenv.Load(filename); err != nil && !os.IsNotExist(err) {
			logger.Warningf("failed to load env file %s: %v", filename, err)
		}
	}

	// 如果可用则加载 Drone CI 环境
	if _, err := os.Stat("/run/drone/env"); err == nil {
		if err := godotenv.Overload("/run/drone/env"); err != nil {
			logger.Warningf("failed to load /run/drone/env: %v", err)
		}
	}

	return nil
}

// globalFlags 创建全局标志
func globalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "host",
			Aliases: []string{"H"},
			Usage:   "DevOps API 基础 URL",
			Value:   "https://devops.example.com",
			EnvVars: []string{"DEVOPS_URL", "URL"},
		},
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "DevOps 用户名",
			EnvVars: []string{"DEVOPS_USERNAME", "USERNAME"},
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p"},
			Usage:   "DevOps 密码",
			EnvVars: []string{"DEVOPS_PASSWORD", "PASSWORD"},
		},
		&cli.BoolFlag{
			Name:    "debug",
			Usage:   "启用调试模式显示详细信息",
			EnvVars: []string{"DEVOPS_DEBUG", "DEBUG"},
		},
	}
}

// getConfig 从 CLI 上下文获取共享配置
func getConfig(c *cli.Context) *Config {
	return &Config{
		BaseURL:  c.String("host"),
		Username: c.String("username"),
		Password: c.String("password"),
		Debug:    c.Bool("debug"),
	}
}
