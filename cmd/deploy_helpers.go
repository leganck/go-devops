package cmd

import (
	"context"
	"go-devops/devops"
	"go-devops/internal/logger"
)

// findTaskUUIDs 查找指定服务器的未完成任务 UUID
//
// 返回服务器 ID 到任务 UUID 的映射
func findTaskUUIDs(ctx context.Context, client *devops.DevOps, env, programAlias string, serverIDs []string) (map[string]string, error) {
	logger.Infof("查询部署历史以查找未完成的任务...")

	historyResult, err := client.GetDeployHistory(ctx, &devops.DeployHistoryRequest{
		Page:         1,
		Limit:        50, // 增加限制以获取更多历史记录
		EnvName:      env,
		Condition:    programAlias,
		DeployStatus: "",
	})
	if err != nil {
		logger.Warningf("获取部署历史失败: %v，继续部署", err)
		return make(map[string]string), nil
	}

	taskMap := make(map[string]string)

	// 为每个服务器查找未完成的任务
	for _, serverID := range serverIDs {
		for _, item := range historyResult.Data {
			if item.ServerId == serverID && item.DeployStatus != devops.DeployStatusSuccess {
				if _, exists := taskMap[serverID]; !exists {
					logger.Infof("找到未完成任务: ServerID=%s, TaskUUID=%s, Status=%d",
						item.ServerId, item.TaskUuid, item.DeployStatus)
					taskMap[serverID] = item.TaskUuid
					break
				}
			}
		}
	}

	// 记录没有未完成任务的服务器
	for _, serverID := range serverIDs {
		if _, found := taskMap[serverID]; !found {
			logger.Infof("服务器 %s 没有未完成的任务", serverID)
		}
	}

	return taskMap, nil
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
