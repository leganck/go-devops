package cmd

import (
	"encoding/json"
	"fmt"
	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"
	"os"
	"text/tabwriter"

	"github.com/urfave/cli/v2"
)

// serversCommand 创建服务器查询子命令
func serversCommand() *cli.Command {
	return &cli.Command{
		Name:      "servers",
		Usage:     "查询程序可用服务器",
		UsageText: "go-devops servers [选项]",
		Description: "查询指定程序在指定环境中的可用服务器列表。\n\n" +
			"示例:\n" +
			"  go-devops servers -a smartpos-svc-erp-chain -e dev2",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "program-alias",
				Aliases: []string{"a"},
				Usage:   "程序别名 (例如: smartpos-svc-erp-chain)",
				EnvVars: []string{"DEVOPS_PROGRAM_ALIAS", "PROGRAM_ALIAS"},
			},
			&cli.StringFlag{
				Name:    "env",
				Aliases: []string{"e"},
				Usage:   "环境名称 (例如: dev2, test)",
				EnvVars: []string{"DEVOPS_ENV", "ENV"},
			},
			&cli.BoolFlag{
				Name:    "json",
				Usage:   "以 JSON 格式输出",
			},
		},
		Action: serversAction,
	}
}

// serversAction 执行服务器查询操作
func serversAction(c *cli.Context) error {
	cfg := getConfig(c)

	// 验证必需参数
	programAlias := c.String("program-alias")
	env := c.String("env")
	if programAlias == "" {
		return errors.NewValidationError("程序别名不能为空", nil)
	}
	if env == "" {
		return errors.NewValidationError("环境名称不能为空", nil)
	}

	// 创建 DevOps 客户端并登录
	client, err := createAndLoginClient(c.Context, cfg)
	if err != nil {
		return err
	}

	// 查询服务器
	servers, err := client.GetServers(c.Context, &devops.ServerRequest{
		EnvName:          env,
		ProgramAliasName: programAlias,
	})
	if err != nil {
		return errors.NewAPIError("获取服务器失败", err)
	}

	// 输出结果
	if c.Bool("json") {
		printServersJSON(servers)
	} else {
		printServersTable(servers)
	}

	return nil
}

// printServersTable 以表格形式打印服务器列表
func printServersTable(servers [][]devops.Server) {
	if len(servers) == 0 {
		logger.Infof("没有找到服务器")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	for groupIndex, group := range servers {
		if len(group) > 0 {
			fmt.Fprintf(w, "组 %d:\n", groupIndex+1)
			fmt.Fprintln(w, "服务器别名\t\t服务器ID")
			fmt.Fprintln(w, "----------\t\t-------")
			for _, server := range group {
				fmt.Fprintf(w, "%s\t\t%s\n", server.ServerAlias, server.ServerID)
			}
			fmt.Fprintln(w)
		}
	}

	w.Flush()

	// 统计总数
	total := 0
	for _, group := range servers {
		total += len(group)
	}
	fmt.Printf("共 %d 个服务器（分 %d 组）\n", total, len(servers))
}

// printServersJSON 以 JSON 格式打印服务器列表
func printServersJSON(servers [][]devops.Server) {
	// 创建简化的 JSON 输出结构
	type serverOutput struct {
		Alias string `json:"alias"`
		ID    string `json:"id"`
	}

	var result [][]serverOutput
	for _, group := range servers {
		var outputGroup []serverOutput
		for _, server := range group {
			outputGroup = append(outputGroup, serverOutput{
				Alias: server.ServerAlias,
				ID:    server.ServerID,
			})
		}
		result = append(result, outputGroup)
	}

	data, err := json.Marshal(result)
	if err != nil {
		logger.Errorf("JSON 序列化失败: %v", err)
		return
	}
	fmt.Println(string(data))
}
