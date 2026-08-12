package cmd

import (
	"fmt"
	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"

	"github.com/urfave/cli/v2"
)

func logoutCommand() *cli.Command {
	return &cli.Command{
		Name:      "logout",
		Usage:     "清除本地持久登录会话",
		UsageText: "go-devops logout [选项]",
		Description: "删除当前 host+username 对应的本地会话文件。\n" +
			"下次命令将重新执行 RSA 登录。",
		Action: logoutAction,
	}
}

func logoutAction(c *cli.Context) error {
	cfg := getConfig(c)
	if cfg.Username == "" {
		return errors.NewValidationError("用户名不能为空", nil)
	}
	if cfg.BaseURL == "" {
		return errors.NewValidationError("API 地址不能为空", nil)
	}

	path, err := devops.SessionFilePath(cfg.BaseURL, cfg.Username)
	if err != nil {
		return errors.NewAPIError("解析会话路径失败", err)
	}
	if err := devops.ClearSessionFile(cfg.BaseURL, cfg.Username); err != nil {
		return errors.NewAPIError("清除会话失败", err)
	}
	logger.Infof("已清除本地会话: %s", path)
	fmt.Println("logout ok")
	return nil
}
