package cmd

import (
	"fmt"
	"go-devops/internal/logger"

	"github.com/gen2brain/beeep"
)

// sendNotification 发送桌面通知
func sendNotification(title, body string) {
	beeep.AppName = "Go DevOps 插件"
	if err := beeep.Notify(title, body, ""); err != nil {
		logger.Warningf("Failed to send notification: %v", err)
	}
}

// sendSuccessNotification 发送成功通知
func sendSuccessNotification(programAlias, actualVersion, env string, serverCount int) {
	sendNotification(
		"部署完成",
		fmt.Sprintf("程序 %s 版本 %s 已成功部署到 %d 个服务器（环境 %s）",
			programAlias, actualVersion, serverCount, env),
	)
}

// sendRetryNotification 发送重试通知
func sendRetryNotification(programAlias, actualVersion, serverName, env string, attempt int) {
	sendNotification(
		"重新部署开始",
		fmt.Sprintf("开始重新部署程序 %s 版本 %s 到服务器 %s（环境 %s），第 %d 次重试...",
			programAlias, actualVersion, serverName, env, attempt),
	)
}

// sendSSHRetryNotification 发送 SSH 失败重试通知
func sendSSHRetryNotification(programAlias, actualVersion, serverName, env string, attempt int) {
	sendNotification(
		"部署重试",
		fmt.Sprintf("程序 %s 版本 %s 部署到服务器 %s（环境 %s）SSH失败，正在进行第 %d 次重试...",
			programAlias, actualVersion, serverName, env, attempt),
	)
}

// sendFailureNotification 发送失败通知
func sendFailureNotification(programAlias, projectVersion, server, env string) {
	sendNotification(
		"部署失败",
		fmt.Sprintf("程序 %s 版本 %s 部署到服务器 %s（环境 %s）失败",
			programAlias, projectVersion, server, env),
	)
}

// sendPartialFailureNotification 发送部分失败通知
func sendPartialFailureNotification(successCount, totalCount int) {
	sendNotification(
		"部署部分失败",
		fmt.Sprintf("成功部署 %d/%d 个服务器", successCount, totalCount),
	)
}
