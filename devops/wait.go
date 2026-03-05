package devops

import (
	"context"
	"errors"
	"fmt"
	"go-devops/internal/logger"
	"strings"
	"time"
)

// 部署状态常量
const (
	DeployStatusInProgress = 1 // 进行中
	DeployStatusPending    = 2 // 等待中
	DeployStatusFailed     = 3 // 失败
	DeployStatusSuccess    = 4 // 成功
	DeployStatusCancelled  = 5 // 已取消
)

// 部署相关的错误
var (
	ErrTaskInProgress      = errors.New("task is still in progress")
	ErrSSHDeploymentFailed = errors.New("ssh deployment failed")
)

// checkDeployStatus 检查部署任务的当前状态
// 如果任务成功完成返回 nil，如果仍在运行返回 ErrTaskInProgress，
// 如果是 SSH 相关失败返回 ErrSSHDeploymentFailed，或其他错误返回相应的错误。
func (d *DevOps) checkDeployStatus(ctx context.Context, taskID string) error {
	historyReq := &DeployHistoryRequest{
		Page:  1,
		Limit: 10,
	}
	historyResult, err := d.GetDeployHistory(ctx, historyReq)
	if err != nil {
		return fmt.Errorf("获取部署历史失败: %w", err)
	}

	// 在结果中查找 ID 匹配的任务
	var targetTask *DeployHistoryItem
	for i := range historyResult.Data {
		// 这里使用 history.ID 作为任务唯一标识
		if historyResult.Data[i].ID == taskID {
			targetTask = &historyResult.Data[i]
			break
		}
	}

	if targetTask == nil {
		return fmt.Errorf("在部署历史中未找到任务 %s", taskID)
	}

	return d.evaluateDeployStatus(targetTask, taskID)
}

// evaluateDeployStatus 评估部署状态并返回相应的错误或 nil
func (d *DevOps) evaluateDeployStatus(task *DeployHistoryItem, taskID string) error {
	serviceName := task.ServerAlias
	switch task.DeployStatus {
	case DeployStatusSuccess:
		logger.Infof("[%s] 任务 %s 已成功完成（状态: %d, 描述: %s）",
			serviceName, taskID, task.DeployStatus, task.DeployDesc)
		return nil

	case DeployStatusFailed:
		logger.Errorf("[%s] 任务 %s 失败（状态: %d, 描述: %s）",
			serviceName, taskID, task.DeployStatus, task.DeployDesc)
		// 检查是否为 SSH 错误以便重试
		if d.isSSHError(task.DeployDesc) {
			logger.Errorf("[%s] 任务 %s 因 SSH 错误失败，将返回 SSH 特定错误以便可能的重试", serviceName, taskID)
			return ErrSSHDeploymentFailed
		}
		return fmt.Errorf("部署任务 %s 失败: %s", taskID, task.DeployDesc)

	case DeployStatusCancelled:
		logger.Errorf("[%s] 任务 %s 已取消（状态: %d, 描述: %s）",
			serviceName, taskID, task.DeployStatus, task.DeployDesc)
		return fmt.Errorf("部署任务 %s 已取消: %s", taskID, task.DeployDesc)

	default:
		logger.Infof("[%s] 任务 %s 仍在进行中（状态: %d, 描述: %s），等待中...",
			serviceName, taskID, task.DeployStatus, task.DeployDesc)
		return ErrTaskInProgress
	}
}

// isSSHError 检查部署描述是否指示 SSH 相关错误
func (d *DevOps) isSSHError(description string) bool {
	return strings.Contains(strings.ToLower(description), "ssh")
}

// WaitForDeployCompletion 轮询部署状态直到完成或超时
// pollInterval 决定检查状态的频率。
// timeout 是等待的最大超时持续时间。
// 如果部署成功完成返回 nil。
// 如果由于 SSH 问题导致部署失败返回 ErrSSHDeploymentFailed（用于重试逻辑）。
// 其他失败或超时返回错误。
func (d *DevOps) WaitForDeployCompletion(ctx context.Context, taskID string, serverName string, pollInterval, timeout time.Duration) error {
	logger.Infof("服务器 %s: 开始等待部署任务 %s 完成（轮询间隔: %v, 超时时间: %v）...", serverName, taskID, pollInterval, timeout)

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := d.checkDeployStatus(waitCtx, taskID)
			if err == nil {
				logger.Infof("服务器 %s: 部署任务 %s 已成功完成", serverName, taskID)
				return nil
			}
			if errors.Is(err, ErrTaskInProgress) {
				continue
			}
			if errors.Is(err, ErrSSHDeploymentFailed) {
				return ErrSSHDeploymentFailed
			}
			return fmt.Errorf("服务器 %s 的任务 %s: 等待过程中出现异常: %w", serverName, taskID, err)

		case <-waitCtx.Done():
			logger.Errorf("服务器 %s 的任务 %s: 等待因超时或上下文取消而终止", serverName, taskID)
			return fmt.Errorf("服务器 %s 的任务 %s: 等待被中止: %w", serverName, taskID, waitCtx.Err())
		}
	}
}
