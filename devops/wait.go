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
func (d *DevOps) checkDeployStatus(ctx context.Context, taskUUID string) error {
	historyReq := &DeployHistoryRequest{
		Page:  1,
		Limit: 10,
	}

	historyResult, err := d.GetDeployHistory(ctx, historyReq)
	if err != nil {
		return fmt.Errorf("failed to get deploy history: %w", err)
	}

	// 在结果中查找 UUID 匹配的任务
	var targetTask *DeployHistoryItem
	for i := range historyResult.Data {
		if historyResult.Data[i].TaskUuid == taskUUID {
			targetTask = &historyResult.Data[i]
			break
		}
	}

	if targetTask == nil {
		return fmt.Errorf("task %s not found in deploy history", taskUUID)
	}

	return d.evaluateDeployStatus(targetTask, taskUUID)
}

// evaluateDeployStatus 评估部署状态并返回相应的错误或 nil
func (d *DevOps) evaluateDeployStatus(task *DeployHistoryItem, taskUUID string) error {
	switch task.DeployStatus {
	case DeployStatusSuccess:
		logger.Infof("task %s completed successfully (status: %d, description: %s)",
			taskUUID, task.DeployStatus, task.DeployDesc)
		return nil

	case DeployStatusFailed:
		logger.Errorf("task %s failed with status: %d, description: %s",
			taskUUID, task.DeployStatus, task.DeployDesc)
		// 检查是否为 SSH 错误以便重试
		if d.isSSHError(task.DeployDesc) {
			logger.Errorf("task %s failed with SSH error, will return SSH-specific error for potential retry", taskUUID)
			return ErrSSHDeploymentFailed
		}
		return fmt.Errorf("deployment task %s failed: %s", taskUUID, task.DeployDesc)

	case DeployStatusCancelled:
		logger.Errorf("task %s was cancelled (status: %d, description: %s)",
			taskUUID, task.DeployStatus, task.DeployDesc)
		return fmt.Errorf("deployment task %s was cancelled: %s", taskUUID, task.DeployDesc)

	default:
		logger.Infof("task %s is still in progress (status: %d, description: %s), waiting...",
			taskUUID, task.DeployStatus, task.DeployDesc)
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
func (d *DevOps) WaitForDeployCompletion(ctx context.Context, taskUUID string, pollInterval, timeout time.Duration) error {
	logger.Infof("waiting for deployment task %s to complete (poll interval: %v, timeout: %v)...", taskUUID, pollInterval, timeout)

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := d.checkDeployStatus(waitCtx, taskUUID)
			if err == nil {
				logger.Infof("task %s completed successfully", taskUUID)
				return nil
			}
			if errors.Is(err, ErrTaskInProgress) {
				continue
			}
			if errors.Is(err, ErrSSHDeploymentFailed) {
				return ErrSSHDeploymentFailed
			}
			return fmt.Errorf("task %s: unexpected error: %w", taskUUID, err)

		case <-waitCtx.Done():
			logger.Errorf("task %s: wait aborted due to timeout or context cancellation", taskUUID)
			return fmt.Errorf("wait for task %s aborted: %w", taskUUID, waitCtx.Err())
		}
	}
}
