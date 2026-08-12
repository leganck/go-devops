package cmd

import (
	"encoding/json"
	"fmt"
	"go-devops/internal/logger"

	"github.com/urfave/cli/v2"
)

// envsCommand 创建环境查询子命令
func envsCommand() *cli.Command {
	return &cli.Command{
		Name:      "envs",
		Usage:     "查询可用的环境列表",
		UsageText: "go-devops envs [选项]",
		Description: "查询当前用户具有 deployProgram:page 权限、且权限 Envs 非空的环境。\n" +
			"若权限 Envs 为空（全局权限），列表可能为空，需显式传 -e。\n\n" +
			"推荐工作流起点: envs → programs/sql/logs/history\n\n" +
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

	client, err := createAndLoginClient(c.Context, cfg)
	if err != nil {
		return err
	}

	envs := client.GetEnvs()

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
		fmt.Println("提示: 仅列出 deployProgram:page 且 Envs 非空的环境；全局权限请显式传 -e")
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
