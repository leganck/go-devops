package cmd

import (
	"context"
	"fmt"
	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"
	"strings"
)

// doDeploy 执行实际的部署操作（支持多服务器）
//
// 返回服务器 ID 到任务 UUID 的映射
func doDeploy(ctx context.Context, client *devops.DevOps, opts *DeployOptions, versionPath string, serverIDs []string, actualVersion string) (map[string]string, error) {
	if versionPath == "" {
		return nil, errors.NewValidationError("未找到部署的版本路径", nil)
	}

	// 将服务器 ID 列表用逗号连接
	serversStr := strings.Join(serverIDs, ",")

	deployReq := &devops.DeployRequest{
		ProgramAliasName: opts.ProgramAlias,
		ProgramType:      opts.ProgramType,
		RelativePath:     versionPath,
		NotifyUser:       opts.NotifyUser,
		NotifyMemo:       buildNotifyMemo(opts.ProgramAlias, actualVersion, opts.Server),
		Servers:          serversStr,
		EnvName:          opts.Env,
		ProgramVersion:   actualVersion,
	}

	serverInfo := opts.Server
	if serverInfo == "" {
		serverInfo = fmt.Sprintf("%d servers", len(serverIDs))
	}
	logger.Infof("部署版本 %s 到 %s...", actualVersion, serverInfo)

	if err := client.Deploy(ctx, deployReq); err != nil {
		return nil, errors.NewDeploymentError("部署失败", err)
	}

	logger.Infof("部署已启动，查询任务 UUID...")

	// 查找所有任务 UUID
	return findTaskUUIDs(ctx, client, opts.Env, opts.ProgramAlias, serverIDs)
}

// redeploySingleServer 重新部署单个服务器
//
// 返回新的任务 UUID
func redeploySingleServer(ctx context.Context, client *devops.DevOps, opts *DeployOptions, versionPath string, serverID string, actualVersion string) (string, error) {
	// 单个服务器部署
	deployReq := &devops.DeployRequest{
		ProgramAliasName: opts.ProgramAlias,
		ProgramType:      opts.ProgramType,
		RelativePath:     versionPath,
		NotifyUser:       opts.NotifyUser,
		NotifyMemo:       fmt.Sprintf("Redeploy %s version %s to server %s (retry)", opts.ProgramAlias, actualVersion, serverID),
		Servers:          serverID,
		EnvName:          opts.Env,
		ProgramVersion:   actualVersion,
	}

	if err := client.Deploy(ctx, deployReq); err != nil {
		return "", errors.NewDeploymentError("重新部署失败", err)
	}

	// 查找新任务 UUID
	historyResult, err := client.GetDeployHistory(ctx, &devops.DeployHistoryRequest{
		Page:         1,
		Limit:        10,
		EnvName:      opts.Env,
		Condition:    opts.ProgramAlias,
		DeployStatus: "",
	})
	if err != nil {
		return "", fmt.Errorf("获取部署历史失败: %w", err)
	}

	// 查找该服务器的最新任务
	for _, item := range historyResult.Data {
		if item.ServerId == serverID && item.DeployStatus != devops.DeployStatusSuccess {
			logger.Infof("服务器 %s: 找到重试任务 history.id=%s, taskUuid=%s (状态: %d)", serverID, item.ID, item.TaskUuid, item.DeployStatus)
			// 监听重试任务时改为使用 history.ID 作为任务标识
			return item.ID, nil
		}
	}

	return "", fmt.Errorf("未找到服务器 %s 的重试任务", serverID)
}

// buildNotifyMemo 构建通知备注
func buildNotifyMemo(programAlias, version, server string) string {
	if server == "" {
		return fmt.Sprintf("Deploy %s version %s to all servers", programAlias, version)
	}
	return fmt.Sprintf("Deploy %s version %s to server %s", programAlias, version, server)
}
