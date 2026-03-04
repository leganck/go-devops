package cmd

import (
	"context"
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

// deployWithRetry 使用重试逻辑执行部署
//
// 部署完成后，使用并发协程监听每个部署任务
// 当某个服务器部署失败是因为 SSH 错误时，重新部署当前服务器
func deployWithRetry(ctx context.Context, client *devops.DevOps, opts *DeployOptions, versionPath string, serverIDs []string, actualVersion string) error {
	// 1. 初始部署所有服务器
	taskMap, err := doDeploy(ctx, client, opts, versionPath, serverIDs, actualVersion)
	if err != nil {
		sendFailureNotification(opts.ProgramAlias, opts.ProjectVersion, opts.Server, opts.Env)
		return err
	}

	if !opts.Wait {
		sendNotification("部署已启动", fmt.Sprintf("%s v%s 已部署到 %d 个服务器", opts.ProgramAlias, actualVersion, len(serverIDs)))
		return nil
	}

	// 2. 并发监听所有任务，对失败的服务器单独重试
	return watchAndRetryTasks(ctx, client, opts, versionPath, serverIDs, actualVersion, taskMap)
}

// watchAndRetryTasks 并发监听所有部署任务
//
// 当某个服务器的任务因 SSH 错误失败时，单独重新部署该服务器
func watchAndRetryTasks(ctx context.Context, client *devops.DevOps, opts *DeployOptions, versionPath string, serverIDs []string, actualVersion string, initialTaskMap map[string]string) error {
	logger.Infof("开始并发监听 %d 个服务器的部署任务...", len(serverIDs))

	// 使用 errgroup 管理并发任务
	g, gCtx := errgroup.WithContext(ctx)

	// 限制并发数以避免过多 goroutine
	maxConcurrent := runtime.NumCPU()
	if maxConcurrent < 4 {
		maxConcurrent = 4
	}
	if maxConcurrent > len(serverIDs) {
		maxConcurrent = len(serverIDs)
	}

	sem := make(chan struct{}, maxConcurrent)

	// 用于收集失败的服务器
	var failedServersMu sync.Mutex
	var failedServers []string

	// 为每个服务器启动一个监听协程
	for _, serverID := range serverIDs {
		serverID := serverID // 闭包捕获

		g.Go(func() error {
			sem <- struct{}{}        // 获取信号量
			defer func() { <-sem }() // 释放信号量

			// 获取初始任务 UUID
			taskUUID, exists := initialTaskMap[serverID]
			if !exists {
				logger.Warningf("服务器 %s 没有初始任务，跳过", serverID)
				return nil
			}

			// 监听并重试该服务器的部署
			if err := deploySingleServerWithRetry(gCtx, client, opts, versionPath, serverID, actualVersion, taskUUID); err != nil {
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
		if len(failedServers) > 0 && len(failedServers) < len(serverIDs) {
			logger.Warningf("部分服务器部署失败: %v", failedServers)
			sendPartialFailureNotification(len(serverIDs)-len(failedServers), len(serverIDs))
			return fmt.Errorf("部分服务器部署失败 (失败 %d/%d): %w", len(failedServers), len(serverIDs), err)
		}
		failedServersMu.Unlock()
		sendFailureNotification(opts.ProgramAlias, opts.ProjectVersion, "所有服务器", opts.Env)
		return err
	}

	logger.Infof("所有 %d 个服务器部署完成", len(serverIDs))
	sendSuccessNotification(opts.ProgramAlias, actualVersion, opts.Env, len(serverIDs))
	return nil
}

// deploySingleServerWithRetry 监听并重试单个服务器的部署
//
// 当部署因 SSH 错误失败时，自动重新部署该服务器
func deploySingleServerWithRetry(ctx context.Context, client *devops.DevOps, opts *DeployOptions, versionPath string, serverID string, actualVersion string, taskUUID string) error {
	for attempt := 0; attempt <= maxSSHRetry; attempt++ {
		if attempt > 0 {
			logger.Infof("服务器 %s: 开始第 %d 次重试...", serverID, attempt)
			sendRetryNotification(opts.ProgramAlias, actualVersion, serverID, opts.Env, attempt)
		}

		// 等待当前任务完成
		logger.Infof("服务器 %s: 等待任务 %s 完成 (尝试 %d/%d)...", serverID, taskUUID, attempt+1, maxSSHRetry+1)
		waitErr := client.WaitForDeployCompletion(ctx, taskUUID, 10*time.Second, 10*time.Minute)

		if waitErr == nil {
			logger.Infof("服务器 %s: 部署成功完成", serverID)
			return nil
		}

		// 检查是否为 SSH 错误且可以重试
		if isSSHErrorFromWait(waitErr) && attempt < maxSSHRetry {
			logger.Infof("服务器 %s: SSH 部署失败，%v 后重试...", serverID, sshRetryDelay)
			sendSSHRetryNotification(opts.ProgramAlias, actualVersion, serverID, opts.Env, attempt+1)
			time.Sleep(sshRetryDelay)

			// 重新部署该服务器
			logger.Infof("服务器 %s: 重新部署...", serverID)
			newTaskUUID, deployErr := redeploySingleServer(ctx, client, opts, versionPath, serverID, actualVersion)
			if deployErr != nil {
				return fmt.Errorf("服务器 %s 重新部署失败: %w", serverID, deployErr)
			}
			taskUUID = newTaskUUID
			continue
		}

		// 其他错误或已达最大重试次数
		return fmt.Errorf("服务器 %s 部署失败: %w", serverID, waitErr)
	}

	return fmt.Errorf("服务器 %s: 达到最大重试次数 (%d)", serverID, maxSSHRetry)
}

// isSSHErrorFromWait 检查等待任务时的错误是否为 SSH 错误
func isSSHErrorFromWait(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否为 devops.ErrSSHDeploymentFailed
	if err.Error() == "ssh deployment failed" {
		return true
	}

	// 检查错误消息中是否包含 SSH 关键字
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "ssh")
}
