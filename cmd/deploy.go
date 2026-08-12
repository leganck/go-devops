package cmd

import (
	"context"
	"fmt"
	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"

	"github.com/urfave/cli/v2"
)

// deployCommand 创建部署子命令
func deployCommand() *cli.Command {
	return &cli.Command{
		Name:      "deploy",
		Usage:     "部署程序到指定服务器",
		UsageText: "go-devops deploy [选项]",
		Description: "部署指定版本的程序到目标服务器。支持 SSH 失败自动重试。\n\n" +
			"推荐工作流: envs → programs -e → version -e -a → servers -e -a → deploy ...\n" +
			"未显式指定 -t 且 snapshots 无版本时，会自动尝试 releases。\n\n" +
			"示例:\n" +
			"  go-devops deploy -a smartpos-svc-erp-chain -e dev2 -v 1.0.0 -s dev2-zd1-erp-chain",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "program-alias",
				Aliases: []string{"a"},
				Usage:   "程序别名 (例如: smartpos-svc-erp-chain)",
				EnvVars: []string{"DEVOPS_PROGRAM_ALIAS", "PROGRAM_ALIAS"},
			},
			&cli.StringFlag{
				Name:    "program-type",
				Aliases: []string{"t"},
				Usage:   "程序类型 (snapshots 或 releases)",
				Value:   "snapshots",
				EnvVars: []string{"DEVOPS_PROGRAM_TYPE", "PROGRAM_TYPE"},
			},
			&cli.StringFlag{
				Name:    "env",
				Aliases: []string{"e"},
				Usage:   "部署环境（deployProgram，与 SQL/日志环境不一定相同）",
				EnvVars: []string{"DEVOPS_DEPLOY_ENV", "DEVOPS_ENV", "ENV"},
			},
			&cli.StringFlag{
				Name:    "version",
				Aliases: []string{"v"},
				Usage:   "要部署的版本 (例如: 1.0.0)",
				EnvVars: []string{"DEVOPS_PROJECT_VERSION", "PROJECT_VERSION"},
			},
			&cli.StringFlag{
				Name:    "server",
				Aliases: []string{"s"},
				Usage:   "目标服务器别名 (例如: dev2-zd1-erp-chain)",
				EnvVars: []string{"DEVOPS_SERVER", "SERVER"},
			},
			&cli.StringFlag{
				Name:    "notify",
				Usage:   "部署后通知的用户 (逗号分隔)",
				EnvVars: []string{"DEVOPS_NOTIFY_USER", "NOTIFY_USER"},
			},
			&cli.BoolFlag{
				Name:    "wait",
				Usage:   "等待部署完成",
				EnvVars: []string{"DEVOPS_WAIT", "WAIT"},
			},
		},
		Action: deployAction,
	}
}

// deployAction 执行部署操作
func deployAction(c *cli.Context) error {
	ctx := c.Context
	cfg := getConfig(c)

	// 验证必需参数
	required := map[string]string{
		"程序别名": c.String("program-alias"),
		"环境名称": c.String("env"),
		"版本":     c.String("version"),
	}
	for name, value := range required {
		if value == "" {
			return errors.NewValidationError(fmt.Sprintf("%s 不能为空", name), nil)
		}
	}

	// 创建 DevOps 客户端并登录
	client, err := createAndLoginClient(ctx, cfg)
	if err != nil {
		return err
	}

	// 执行部署
	return executeDeploy(ctx, client, &DeployOptions{
		ProgramAlias:   c.String("program-alias"),
		ProgramType:    c.String("program-type"),
		Env:            c.String("env"),
		ProjectVersion: c.String("version"),
		Server:         c.String("server"),
		NotifyUser:     c.String("notify"),
		Wait:           c.Bool("wait"),
		TypeFallback:   !c.IsSet("program-type"),
	})
}

// DeployOptions 包含部署选项
type DeployOptions struct {
	ProgramAlias   string
	ProgramType    string
	Env            string
	ProjectVersion string
	Server         string
	NotifyUser     string
	Wait           bool
	// TypeFallback 为 true 表示 program-type 使用默认值，允许空结果时回退到另一类型
	TypeFallback bool
}

// executeDeploy 执行部署流程
func executeDeploy(ctx context.Context, client *devops.DevOps, opts *DeployOptions) error {
	logger.Infof("登录成功")

	// 1. 检查程序是否存在
	if err := checkProgramExists(ctx, client, opts.ProgramAlias, opts.Env); err != nil {
		return err
	}

	// 2. 查询版本并获取路径
	versionPath, actualVersion, err := queryVersion(ctx, client, opts)
	if err != nil {
		return err
	}

	// 3. 获取服务器信息（包含 ID 和名称映射）
	serverInfo, err := getServerInfo(ctx, client, opts.ProgramAlias, opts.Env, opts.Server)
	if err != nil {
		return err
	}

	// 4. 执行部署到所有服务器
	if err := deployToServers(ctx, client, opts, versionPath, serverInfo, actualVersion); err != nil {
		return err
	}

	return nil
}

// deployToServers 部署到多个服务器（支持重试）
func deployToServers(ctx context.Context, client *devops.DevOps, opts *DeployOptions, versionPath string, serverInfo *ServerInfo, actualVersion string) error {
	if len(serverInfo.IDs) == 0 {
		return errors.NewValidationError("没有可部署的服务器", nil)
	}

	return deployWithRetry(ctx, client, opts, versionPath, serverInfo, actualVersion)
}
