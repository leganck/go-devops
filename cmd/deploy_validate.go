package cmd

import (
	"context"
	"fmt"
	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"
	"go-devops/internal/version"
)

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
//
// 先尝试精确匹配，如果失败则尝试模糊匹配
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

// getServerIDs 获取服务器 ID 列表
//
// 如果 serverAlias 为空，返回所有服务器的 ID
// 如果指定了 serverAlias，返回匹配服务器的 ID
func getServerIDs(ctx context.Context, client *devops.DevOps, programAlias, env, serverAlias string) ([]string, error) {
	logger.Infof("获取程序 %s 在环境 %s 中的服务器...", programAlias, env)

	servers, err := client.GetServers(ctx, &devops.ServerRequest{
		EnvName:          env,
		ProgramAliasName: programAlias,
	})
	if err != nil {
		return nil, errors.NewAPIError("获取服务器失败", err)
	}

	// 展平所有服务器
	var allServers []devops.Server
	for _, group := range servers {
		allServers = append(allServers, group...)
	}

	logger.Infof("找到 %d 个服务器", len(allServers))

	// 如果 serverAlias 为空，返回所有服务器 ID
	if serverAlias == "" {
		var serverIDs []string
		for _, server := range allServers {
			serverIDs = append(serverIDs, server.ServerID)
		}
		logger.Infof("返回所有服务器 ID (共 %d 个服务器)", len(serverIDs))
		return serverIDs, nil
	}

	// 如果只有一个服务器，使用它
	if len(allServers) == 1 {
		serverID := allServers[0].ServerID
		logger.Infof("注意: 只有一个服务器，使用服务器 %s (ID: %s)", allServers[0].ServerAlias, serverID)
		return []string{serverID}, nil
	}

	// 查找指定的服务器
	for _, server := range allServers {
		if server.ServerAlias == serverAlias {
			logger.Infof("找到服务器 ID: %s，服务器别名: %s", server.ServerID, server.ServerAlias)
			return []string{server.ServerID}, nil
		}
	}

	return nil, errors.NewServerError(fmt.Sprintf("未找到服务器 %s 的 ID", serverAlias), nil)
}
