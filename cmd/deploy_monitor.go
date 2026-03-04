package cmd

import (
	"context"
	"errors"
	"fmt"
	"go-devops/devops"
	"go-devops/internal/logger"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	// maxSSHRetry 是 SSH 部署失败的最大重试次数
	maxSSHRetry = 2
	// sshRetryDelay 是 SSH 部署重试之间的延迟
	sshRetryDelay = 5 * time.Second
)

// RetryConfig 包含单个服务器重试部署所需的配置
type RetryConfig struct {
	Ctx           context.Context
	Client        *devops.DevOps
	Opts          *DeployOptions
	VersionPath   string
	ServerID      string
	ActualVersion string
	TaskID        string
	ServerName    string
}

// deployWithRetry 使用重试逻辑执行部署
//
// 部署完成后，使用并发协程监听每个部署任务
// 当某个服务器部署失败是因为 SSH 错误时，重新部署当前服务器
func deployWithRetry(ctx context.Context, client *devops.DevOps, opts *DeployOptions, versionPath string, serverInfo *ServerInfo, actualVersion string) error {
	// 1. 初始部署所有服务器
	taskMap, err := doDeploy(ctx, client, opts, versionPath, serverInfo.IDs, actualVersion)
	if err != nil {
		sendFailureNotification(opts.ProgramAlias, opts.ProjectVersion, opts.Server, opts.Env)
		return err
	}

	if !opts.Wait {
		sendNotification("部署已启动", fmt.Sprintf("%s v%s 已部署到 %d 个服务器", opts.ProgramAlias, actualVersion, len(serverInfo.IDs)))
		return nil
	}

	// 2. 并发监听所有任务，对失败的服务器单独重试
	return watchAndRetryTasks(ctx, client, opts, versionPath, serverInfo, actualVersion, taskMap)
}

// watchAndRetryTasks 并发监听所有部署任务
//
// 当某个服务器的任务因 SSH 错误失败时，单独重新部署该服务器
func watchAndRetryTasks(ctx context.Context, client *devops.DevOps, opts *DeployOptions, versionPath string, serverInfo *ServerInfo, actualVersion string, initialTaskMap map[string]string) error {
	logger.Infof("开始并发监听 %d 个服务器的部署任务...", len(serverInfo.IDs))

	// 使用 errgroup 管理并发任务
	g, gCtx := errgroup.WithContext(ctx)

	// 限制并发数以避免过多 goroutine
	maxConcurrent := runtime.NumCPU()
	if maxConcurrent < 4 {
		maxConcurrent = 4
	}
	if maxConcurrent > len(serverInfo.IDs) {
		maxConcurrent = len(serverInfo.IDs)
	}

	sem := make(chan struct{}, maxConcurrent)

	// 用于收集失败的服务器
	var failedServersMu sync.Mutex
	var failedServers []string

	// 为每个服务器启动一个监听协程
	for _, serverID := range serverInfo.IDs {
		serverID := serverID // 闭包捕获

		g.Go(func() error {
			sem <- struct{}{}        // 获取信号量
			defer func() { <-sem }() // 释放信号量

			// 获取初始任务 ID
			taskID, exists := initialTaskMap[serverID]
			if !exists {
				logger.Warningf("服务器 %s 没有初始任务，跳过", serverID)
				return nil
			}

			// 获取服务器名称，如果不存在则 fallback 到 serverID
			serverName := serverInfo.Names[serverID]
			if serverName == "" {
				serverName = serverID
			}

			// 监听并重试该服务器的部署
			retryCfg := RetryConfig{
				Ctx:           gCtx,
				Client:        client,
				Opts:          opts,
				VersionPath:   versionPath,
				ServerID:      serverID,
				ActualVersion: actualVersion,
				TaskID:        taskID,
				ServerName:    serverName,
			}
			if err := deploySingleServerWithRetry(retryCfg); err != nil {
				failedServersMu.Lock()
				failedServers = append(failedServers, serverID)
				failedServersMu.Unlock()
				return err
			}
			return nil
		})
	}

	// 等待所有服务器部署完成
	if err := g.Wait(); err != nil {
		// 检查是否有部分服务器成功
		failedServersMu.Lock()
		if len(failedServers) > 0 && len(failedServers) < len(serverInfo.IDs) {
			logger.Warningf("部分服务器部署失败: %v", failedServers)
			sendPartialFailureNotification(len(serverInfo.IDs)-len(failedServers), len(serverInfo.IDs))
			return fmt.Errorf("部分服务器部署失败 (失败 %d/%d): %w", len(failedServers), len(serverInfo.IDs), err)
		}
		failedServersMu.Unlock()
		sendFailureNotification(opts.ProgramAlias, opts.ProjectVersion, "所有服务器", opts.Env)
		return err
	}

	logger.Infof("所有 %d 个服务器部署完成", len(serverInfo.IDs))
	sendSuccessNotification(opts.ProgramAlias, actualVersion, opts.Env, len(serverInfo.IDs))
	return nil
}

// deploySingleServerWithRetry 监听并重试单个服务器的部署
//
// 当部署因 SSH 错误失败时，自动重新部署该服务器
func deploySingleServerWithRetry(cfg RetryConfig) error {
	for attempt := 0; attempt <= maxSSHRetry; attempt++ {

		if attempt > 0 {
			logger.Infof("服务器 %s: 开始第 %d 次重试...", cfg.ServerName, attempt)
			sendRetryNotification(cfg.Opts.ProgramAlias, cfg.ActualVersion, cfg.ServerName, cfg.Opts.Env, attempt)
		}

		// 等待当前任务完成
		logger.Infof("服务器 %s: 等待任务 %s 完成 (尝试 %d/%d)...", cfg.ServerName, cfg.TaskID, attempt+1, maxSSHRetry+1)
		waitErr := cfg.Client.WaitForDeployCompletion(cfg.Ctx, cfg.TaskID, cfg.ServerName, 10*time.Second, 10*time.Minute)

		if waitErr == nil {
			logger.Infof("服务器 %s: 部署成功完成", cfg.ServerName)
			return nil
		}

		// 检查是否为 SSH 错误且可以重试
		if isSSHErrorFromWait(waitErr) && attempt < maxSSHRetry {
			logger.Infof("服务器 %s: SSH 部署失败，%v 后重试...", cfg.ServerName, sshRetryDelay)
			sendSSHRetryNotification(cfg.Opts.ProgramAlias, cfg.ActualVersion, cfg.ServerName, cfg.Opts.Env, attempt+1)

			// 等待重试延迟，但响应 context 取消
			select {
			case <-time.After(sshRetryDelay):
				// 继续重试
			case <-cfg.Ctx.Done():
				return fmt.Errorf("服务器 %s: 在重试延迟期间被取消: %w", cfg.ServerName, cfg.Ctx.Err())
			}

			// 重新部署该服务器
			logger.Infof("服务器 %s: 重新部署...", cfg.ServerName)
			newTaskID, deployErr := redeploySingleServer(cfg.Ctx, cfg.Client, cfg.Opts, cfg.VersionPath, cfg.ServerID, cfg.ActualVersion)
			if deployErr != nil {
				return fmt.Errorf("服务器 %s 重新部署失败: %w", cfg.ServerName, deployErr)
			}
			cfg.TaskID = newTaskID
			continue
		}

		// 其他错误或已达最大重试次数
		return fmt.Errorf("服务器 %s 部署失败: %w", cfg.ServerName, waitErr)
	}

	return fmt.Errorf("服务器 %s: 达到最大重试次数 (%d)", cfg.ServerName, maxSSHRetry)
}

// isSSHErrorFromWait 检查等待任务时的错误是否为 SSH 错误
func isSSHErrorFromWait(err error) bool {
	if err == nil {
		return false
	}

	// 使用类型安全的错误检查
	if errors.Is(err, devops.ErrSSHDeploymentFailed) {
		return true
	}

	// 作为后备方案，检查错误消息中是否包含 SSH 关键字
	// 这可以捕获包装过的错误
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "ssh")
}
