package devops

import (
	"context"
	"errors"
	"fmt"
	"go-devops/internal/logger"
	"time"
)

func (d *DevOps) checkDeployStatus(ctx context.Context, taskUUID string) error {
	reqCtx, reqCancel := context.WithTimeout(ctx, 15*time.Second)
	defer reqCancel()

	historyReq := &DeployHistoryRequest{
		Page:  1,
		Limit: 20,
	}

	historyResult, err := d.GetDeployHistory(reqCtx, historyReq)
	if err != nil {
		return fmt.Errorf("failed to get deploy history: %w", err)
	}

	// Find task with matching UUID in results
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

	switch targetTask.DeployStatus {
	case 4:
		logger.Infof("task %s completed successfully (status: %d, description: %s)",
			taskUUID, targetTask.DeployStatus, targetTask.DeployDesc)
		return nil
	case 3:
	case 5:
		logger.Errorf("task %s failed with status: %d, description: %s",
			taskUUID, targetTask.DeployStatus, targetTask.DeployDesc)
		return fmt.Errorf("deployment task %s failed: %s", taskUUID, targetTask.DeployDesc)
	default:
		logger.Infof("task %s is still in progress (status: %d, description: %s), waiting...",
			taskUUID, targetTask.DeployStatus, targetTask.DeployDesc)
		return ErrTaskInProgress
	}
}

var ErrTaskInProgress = fmt.Errorf("task is still in progress")

func (d *DevOps) WaitForDeployCompletion(ctx context.Context, taskUUID string, pollInterval, timeout time.Duration) error {
	logger.Infof("waiting for deployment task %s to complete (poll interval: %v, timeout: %v)...", taskUUID, pollInterval, timeout)

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 🔹 Start periodic polling
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

			// 🟡 Non-in-progress error (e.g., API down, task failed, auth issue)
			logger.Warningf("task %s: encountered error during polling, will retry: %v", taskUUID, err)

		case <-waitCtx.Done():
			logger.Errorf("task %s: wait aborted due to timeout or context cancellation", taskUUID)
			return fmt.Errorf("wait for task %s aborted: %w", taskUUID, waitCtx.Err())
		}
	}
}
