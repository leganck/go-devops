package cmd

import (
	"fmt"
	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"
	"strings"

	"github.com/urfave/cli/v2"
)

// programsCommand 创建程序查询子命令
func programsCommand() *cli.Command {
	return &cli.Command{
		Name:      "programs",
		Usage:     "查询环境中的程序列表",
		UsageText: "go-devops programs [选项]",
		Description: "查询指定环境中的所有可用程序别名。\n\n" +
			"示例:\n" +
			"  go-devops programs -e dev2",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "env",
				Aliases: []string{"e"},
				Usage:   "环境名称 (例如: dev2, test)",
				EnvVars: []string{"DEVOPS_ENV", "ENV"},
			},
			&cli.StringFlag{
				Name:    "filter",
				Aliases: []string{"f"},
				Usage:   "过滤程序名称 (支持部分匹配)",
			},
			&cli.BoolFlag{
				Name:    "json",
				Usage:   "以 JSON 格式输出",
			},
		},
		Action: programsAction,
	}
}

// programsAction 执行程序查询操作
func programsAction(c *cli.Context) error {
	cfg := getConfig(c)

	// 验证必需参数
	env := c.String("env")
	if env == "" {
		return errors.NewValidationError("环境名称不能为空", nil)
	}

	// 创建 DevOps 客户端
	client, err := createClient(cfg)
	if err != nil {
		return errors.NewAPIError("创建 DevOps 客户端失败", err)
	}

	// 登录
	if err := client.Login(c.Context); err != nil {
		return errors.NewLoginError("登录失败", err)
	}

	// 查询程序列表
	programs, err := client.GetProgramAliases(c.Context, &devops.ProgramAliasRequest{
		EnvName: env,
	})
	if err != nil {
		return errors.NewAPIError("获取程序列表失败", err)
	}

	// 应用过滤过滤
	filter := c.String("filter")
	if filter != "" {
		programs = filterPrograms(programs, filter)
	}

	// 输出结果
	if c.Bool("json") {
		printProgramsJSON(programs)
	} else {
		printProgramsList(programs)
	}

	return nil
}

// filterPrograms 过滤程序列表
func filterPrograms(programs []string, filter string) []string {
	var result []string
	lowerFilter := strings.ToLower(filter)
	for _, p := range programs {
		if strings.Contains(strings.ToLower(p), lowerFilter) {
			result = append(result, p)
		}
	}
	return result
}

// printProgramsList 打印程序列表
func printProgramsList(programs []string) {
	if len(programs) == 0 {
		logger.Infof("没有找到程序")
		return
	}

	fmt.Printf("找到 %d 个程序:\n\n", len(programs))
	for _, p := range programs {
		fmt.Printf("  • %s\n", p)
	}
	fmt.Printf("\n共 %d 个程序\n", len(programs))
}

// printProgramsJSON 以 JSON 格式打印程序列表
func printProgramsJSON(programs []string) {
	fmt.Printf("[")
	for i, p := range programs {
		if i > 0 {
			fmt.Printf(", ")
		}
		fmt.Printf(`"%s"`, p)
	}
	fmt.Printf("]\n")
}
