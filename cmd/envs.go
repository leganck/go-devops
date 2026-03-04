package cmd

import (
	"encoding/json"
	"fmt"
	"go-devops/internal/errors"
	"go-devops/internal/logger"

	"github.com/urfave/cli/v2"
)

// envsCommand 创建环境查询子命令
func envsCommand() *cli.Command {
	return &cli.Command{
		Name:      "envs",
		Usage:     "查询可用的环境列表",
		UsageText: "go-devops envs [选项]",
		Description: "查询当前用户具有 deployProgram:page 权限的所有环境。\n\n" +
			"示例:\n" +
			"  go-devops envs",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "以 JSON 格式输出",
			},
		},
		Action: envsAction,
	}
}

// envsAction 执行环境查询操作
func envsAction(c *cli.Context) error {
	cfg := getConfig(c)

	// 创建 DevOps 客户端并登录
	client, err := createAndLoginClient(c.Context, cfg)
	if err != nil {
		return err
	}

	// 查询环境列表
	envs := client.GetEnvs()

	// 输出结果
	if c.Bool("json") {
		printEnvsJSON(envs)
	} else {
		printEnvsList(envs)
	}

	return nil
}

// printEnvsList 打印环境列表
func printEnvsList(envs []string) {
	if len(envs) == 0 {
		logger.Infof("没有找到可用环境")
		return
	}

	fmt.Printf("找到 %d 个可用环境:\n\n", len(envs))
	for _, e := range envs {
		fmt.Printf("  • %s\n", e)
	}
	fmt.Printf("\n共 %d 个环境\n", len(envs))
}

// printEnvsJSON 以 JSON 格式打印环境列表
func printEnvsJSON(envs []string) {
	data, err := json.Marshal(envs)
	if err != nil {
		logger.Errorf("JSON 序列化失败: %v", err)
		return
	}
	fmt.Println(string(data))
}
