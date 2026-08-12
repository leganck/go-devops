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
		Description: "查询指定程序在指定环境中的可用服务器列表。\n" +
			"会尝试对齐日志项目中的服务器组名（groupName / projectName）。\n\n" +
			"推荐工作流: envs → programs -e → servers -e -a\n\n" +
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
				EnvVars: []string{"DEVOPS_DEPLOY_ENV", "DEVOPS_ENV", "ENV"},
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "以 JSON 格式输出",
			},
		},
		Action: serversAction,
	}
}

// serversAction 执行服务器查询操作
func serversAction(c *cli.Context) error {
	cfg := getConfig(c)

	programAlias := c.String("program-alias")
	env := c.String("env")
	if programAlias == "" {
		return errors.NewValidationError("程序别名不能为空", nil)
	}
	if env == "" {
		return errors.NewValidationError("环境名称不能为空", nil)
	}

	client, err := createAndLoginClient(c.Context, cfg)
	if err != nil {
		return err
	}

	groups, err := client.GetServersWithGroups(c.Context, &devops.ServerRequest{
		EnvName:          env,
		ProgramAliasName: programAlias,
	})
	if err != nil {
		return errors.NewAPIError("获取服务器失败", err)
	}

	if c.Bool("json") {
		printServersJSON(groups)
	} else {
		printServersTable(groups)
	}

	return nil
}

func printServersTable(groups []devops.ServerGroup) {
	if len(groups) == 0 {
		logger.Infof("没有找到服务器")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "GROUP\tPROJECT\tALIAS\tID")
	total := 0
	for _, g := range groups {
		groupName := g.GroupName
		if groupName == "" {
			groupName = "-"
		}
		project := g.ProjectName
		if project == "" {
			project = "-"
		}
		for _, server := range g.Servers {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", groupName, project, server.ServerAlias, server.ServerID)
			total++
		}
	}
	_ = w.Flush()
	fmt.Printf("\n共 %d 个服务器（分 %d 组）\n", total, len(groups))
}

func printServersJSON(groups []devops.ServerGroup) {
	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		logger.Errorf("JSON 序列化失败: %v", err)
		return
	}
	fmt.Println(string(data))
}
