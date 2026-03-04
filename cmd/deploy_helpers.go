package cmd

import (
	"context"
	"go-devops/devops"
	"go-devops/internal/logger"
)

// findTaskUUIDs 查找指定服务器的未完成任务 UUID
//
// 返回服务器 ID 到任务 UUID 的映射
// 时间复杂度: O(n+m) 而非 O(n*m)，其中 n 是服务器数量，m 是历史记录数量
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

	// 先构建 map: serverID -> 未完成的任务项
	// 时间复杂度: O(m)，其中 m 是历史记录数量
	pendingTasks := make(map[string]*devops.DeployHistoryItem)
	for i := range historyResult.Data {
		item := &historyResult.Data[i]
		// 只记录未完成的任务
		if item.DeployStatus != devops.DeployStatusSuccess {
			// 如果该服务器还没有未完成任务，或者这是更新的任务（这里简单使用第一个找到的）
			if _, exists := pendingTasks[item.ServerId]; !exists {
				pendingTasks[item.ServerId] = item
			}
		}
	}

	// 构建最终的 taskMap，只包含我们关心的服务器
	// 时间复杂度: O(n)，其中 n 是服务器数量
	taskMap := make(map[string]string, len(serverIDs))
	for _, serverID := range serverIDs {
		if item, found := pendingTasks[serverID]; found {
			logger.Infof("找到未完成任务: ServerID=%s, TaskUUID=%s, Status=%d",
				item.ServerId, item.TaskUuid, item.DeployStatus)
			taskMap[serverID] = item.TaskUuid
		} else {
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
