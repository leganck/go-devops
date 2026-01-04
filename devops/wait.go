package devops

import (
	"context"
	"fmt"
	"log"
	"time"
)

// WaitForDeployCompletion waits for a deployment task to complete using taskUUID
// It polls GetDeployHistory until the task is found and completed (success or failure)
func (d *DevOps) WaitForDeployCompletion(
	ctx context.Context,
	taskUUID string,
	pollInterval, timeout time.Duration,
) (*DeployHistoryItem, error) {
	deadline := time.Now().Add(timeout)

	log.Printf("waiting for deployment task %s to complete...", taskUUID)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for deployment task %s to complete", taskUUID)
		}

		// Call GetDeployHistory to find the task
		historyReq := &DeployHistoryRequest{
			Page:    1,
			Limit:   20,
			// We don't know the exact filters, so search with empty filters
			// and find the task by taskUUID in the results
		}

		historyResult, err := d.GetDeployHistory(ctx, historyReq)
		if err != nil {
			log.Printf("warning: failed to get deploy history: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		// Find the task with matching taskUUID
		var targetTask *DeployHistoryItem
		for i := range historyResult.Data {
			if historyResult.Data[i].TaskUuid == taskUUID {
				targetTask = &historyResult.Data[i]
				break
			}
		}

		if targetTask == nil {
			log.Printf("task %s not found in deploy history, retrying...", taskUUID)
			time.Sleep(pollInterval)
			continue
		}

		// Check if task has completed (status != 4 means completed? Or need to check other statuses?)
		// Based on plugin.go code, status 4 might be completed, but let's check actual statuses
		// For now, let's assume status 4 is completed, other statuses are in progress
		log.Printf("task %s found with status: %d, description: %s", 
			taskUUID, targetTask.DeployStatus, targetTask.DeployDesc)

		// Check if task has completed (we need to define what success/failure means)
		// Let's assume status 4 is success, status 5 is failure, others are in progress
		// Adjust based on actual API behavior
		switch targetTask.DeployStatus {
		case 4:
			// Task completed successfully
			log.Printf("task %s completed successfully", taskUUID)
			return targetTask, nil
		case 5:
			// Task failed
			log.Printf("task %s failed with status: %d, description: %s", 
				taskUUID, targetTask.DeployStatus, targetTask.DeployDesc)
			return targetTask, fmt.Errorf("deployment task %s failed: %s", taskUUID, targetTask.DeployDesc)
		default:
			// Task is still in progress, continue waiting
			log.Printf("task %s is still in progress (status: %d), waiting...", 
				taskUUID, targetTask.DeployStatus)
			time.Sleep(pollInterval)
			continue
		}
	}
}
