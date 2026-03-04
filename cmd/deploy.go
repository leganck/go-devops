package cmd

import (
	"context"
	"fmt"
	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"
	"go-devops/internal/version"
	"time"

	"github.com/urfave/cli/v2"
)

const (
	// maxSSHRetry 是 SSH 部署失败的最大重试次数
	maxSSHRetry = 2
	// sshRetryDelay 是 SSH 部署重试之间的延迟
	sshRetryDelay = 5 * time.Second
)

// deployCommand 创建部署子命令
func deployCommand() *cli.Command {
	return &cli.Command{
		Name:      "deploy",
		Usage:     "部署程序到指定服务器",
		UsageText: "go-devops deploy [选项]",
		Description: "部署指定版本的程序到目标服务器。支持 SSH 失败自动重试。\n\n" +
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
				Usage:   "环境名称 (例如: dev2, test)",
				EnvVars: []string{"DEVOPS_ENV", "ENV"},
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
		"程序别名":      c.String("program-alias"),
		"环境名称":       c.String("env"),
		"版本":         c.String("version"),
		"目标服务器":      c.String("server"),
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

	// 3. 获取服务器 ID
	serverID, err := getServerID(ctx, client, opts.ProgramAlias, opts.Env, opts.Server)
	if err != nil {
		return err
	}

	// 4. 执行部署
	if err := deployWithRetry(ctx, client, opts, versionPath, serverID, actualVersion); err != nil {
		return err
	}

	return nil
}

// checkProgramExists 检查程序是否存在
func checkProgramExists(ctx context.Context, client *devops.DevOps, programAlias, env string) error {
	logger.Infof("检查程序 %s 在环境 %s 中是否存在...", programAlias, env)

	programs, err := client.GetProgramAliases(ctx, &devops.ProgramAliasRequest{
		EnvName: env,
	})
	if err != nil {
		return errors.NewAPIError("获取程序别名失败", err)
	}

	if !contains(programs, programAlias) {
		return errors.NewValidationError(
			fmt.Sprintf("程序 %s 在环境 %s 中不存在", programAlias, env),
			nil,
		)
	}

	logger.Infof("程序 %s 存在于环境 %s", programAlias, env)
	return nil
}

// queryVersion 查询版本并返回匹配的版本路径
func queryVersion(ctx context.Context, client *devops.DevOps, opts *DeployOptions) (string, string, error) {
	logger.Infof("查询 %s (%s/%s) 的版本...", opts.ProgramAlias, opts.ProgramType, opts.Env)

	versions, err := client.GetVersion(ctx, &devops.VersionRequest{
		ProgramAliasName: opts.ProgramAlias,
		ProgramType:      opts.ProgramType,
		EnvName:          opts.Env,
	})
	if err != nil {
		return "", "", errors.NewAPIError("获取版本失败", err)
	}

	logger.Infof("找到 %d 个版本", len(versions))

	// 尝试精确匹配
	for _, v := range versions {
		if v.Version == opts.ProjectVersion {
			logger.Infof("版本 %s 找到", opts.ProjectVersion)
			logger.Infof("文件路径: %s", v.RelativePath)
			return v.RelativePath, v.Version, nil
		}
	}

	// 尝试模糊匹配
	for _, v := range versions {
		if version.FuzzyMatchVersion(opts.ProjectVersion, v.Version) {
			logger.Infof("注意: 模糊匹配版本 %s 与 %s", v.Version, opts.ProjectVersion)
			logger.Infof("文件路径: %s", v.RelativePath)
			return v.RelativePath, v.Version, nil
		}
	}

	return "", "", errors.NewVersionError(
		fmt.Sprintf("版本 %s 在环境 %s 中不存在", opts.ProjectVersion, opts.Env),
		nil,
	)
}

// getServerID 获取服务器 ID
func getServerID(ctx context.Context, client *devops.DevOps, programAlias, env, serverAlias string) (string, error) {
	logger.Infof("获取程序 %s 在环境 %s 中的服务器...", programAlias, env)

	servers, err := client.GetServers(ctx, &devops.ServerRequest{
		EnvName:          env,
		ProgramAliasName: programAlias,
	})
	if err != nil {
		return "", errors.NewAPIError("获取服务器失败", err)
	}

	// 展平所有服务器
	var allServers []devops.Server
	for _, group := range servers {
		allServers = append(allServers, group...)
	}

	logger.Infof("找到 %d 个服务器", len(allServers))

	// 如果只有一个服务器，使用它
	if len(allServers) == 1 {
		serverID := allServers[0].ServerID
		logger.Infof("注意: 只有一个服务器，使用服务器 %s (ID: %s)", allServers[0].ServerAlias, serverID)
		return serverID, nil
	}

	// 查找指定的服务器
	for _, server := range allServers {
		if server.ServerAlias == serverAlias {
			logger.Infof("找到服务器 ID: %s，服务器别名: %s", server.ServerID, server.ServerAlias)
			return server.ServerID, nil
		}
	}

	return "", errors.NewServerError(fmt.Sprintf("未找到服务器 %s 的 ID", serverAlias), nil)
}

// deployWithRetry 使用重试逻辑执行部署
func deployWithRetry(ctx context.Context, client *devops.DevOps, opts *DeployOptions, versionPath, serverID, actualVersion string) error {
	for attempt := 0; attempt <= maxSSHRetry; attempt++ {
		if attempt > 0 {
			logger.Infof("开始第 %d 次重试...", attempt)
		}

		taskUUID, err := doDeploy(ctx, client, opts, versionPath, serverID, actualVersion)
		if err != nil {
			return err
		}

		if !opts.Wait {
			return nil
		}

		waitErr := client.WaitForDeployCompletion(ctx, taskUUID, 10*time.Second, 10*time.Minute)
		if waitErr == nil {
			return nil
		}

		if isSSHError(waitErr) && attempt < maxSSHRetry {
			logger.Infof("SSH 部署失败，重试中... (%d/%d)", attempt+1, maxSSHRetry)
			time.Sleep(sshRetryDelay)
			continue
		}

		return waitErr
	}

	return nil
}

// doDeploy 执行实际的部署操作
func doDeploy(ctx context.Context, client *devops.DevOps, opts *DeployOptions, versionPath, serverID, actualVersion string) (string, error) {
	logger.Infof("准备部署...")

	if versionPath == "" {
		return "", errors.NewValidationError("未找到部署的版本路径", nil)
	}

	deployReq := &devops.DeployRequest{
		ProgramAliasName: opts.ProgramAlias,
		ProgramType:      opts.ProgramType,
		RelativePath:     versionPath,
		NotifyUser:       opts.NotifyUser,
		NotifyMemo:       buildNotifyMemo(opts.ProgramAlias, actualVersion, opts.Server),
		Servers:          serverID,
		EnvName:          opts.Env,
		ProgramVersion:   actualVersion,
	}

	logger.Infof("部署版本 %s 到服务器 %s...", actualVersion, opts.Server)
	if err := client.Deploy(ctx, deployReq); err != nil {
		return "", errors.NewDeploymentError("部署失败", err)
	}

	logger.Infof("部署已启动")
	return findTaskUUID(ctx, client, opts.Env, opts.ProgramAlias, serverID)
}

// buildNotifyMemo 构建通知备注
func buildNotifyMemo(programAlias, version, server string) string {
	return fmt.Sprintf("Deploy %s version %s to server %s", programAlias, version, server)
}

// findTaskUUID 查找未完成的任务 UUID
func findTaskUUID(ctx context.Context, client *devops.DevOps, env, programAlias, serverID string) (string, error) {
	logger.Infof("查询部署历史以查找未完成的任务...")

	historyResult, err := client.GetDeployHistory(ctx, &devops.DeployHistoryRequest{
		Page:         1,
		Limit:        10,
		EnvName:      env,
		Condition:    programAlias,
		DeployStatus: "",
	})
	if err != nil {
		logger.Warningf("获取部署历史失败: %v，继续部署", err)
		return "", nil
	}

	for _, item := range historyResult.Data {
		if item.ServerId == serverID && item.DeployStatus != devops.DeployStatusSuccess {
			logger.Infof("找到未完成任务: TaskUUID=%s, Status=%d, ServerID=%s",
				item.TaskUuid, item.DeployStatus, item.ServerId)
			return item.TaskUuid, nil
		}
	}

	logger.Infof("服务器 %s 没有未完成的任务", serverID)
	return "", nil
}

// isSSHError 检查是否为 SSH 错误
func isSSHError(err error) bool {
	return err.Error() == "ssh deployment failed"
}

// contains 检查字符串是否存在于切片中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
