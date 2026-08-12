package cmd

import (
	"encoding/json"
	"fmt"
	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"

	"github.com/urfave/cli/v2"
)

// programsCommand 创建程序查询子命令
func programsCommand() *cli.Command {
	return &cli.Command{
		Name:      "programs",
		Usage:     "查询环境中的程序列表",
		UsageText: "go-devops programs [选项]",
		Description: "查询指定环境中的所有可用程序别名。\n\n" +
			"推荐工作流: envs → programs -e → version/servers/deploy\n\n" +
			"示例:\n" +
			"  go-devops programs -e dev2",
		Flags: []cli.Flag{
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

	// 创建 DevOps 客户端并登录
	client, err := createAndLoginClient(c.Context, cfg)
	if err != nil {
		return err
	}

	// 查询程序列表
	programs, err := client.GetProgramAliases(c.Context, &devops.ProgramAliasRequest{
		EnvName: env,
	})
	if err != nil {
		return errors.NewAPIError("获取程序列表失败", err)
	}

	// 输出结果
	if c.Bool("json") {
		printProgramsJSON(programs)
	} else {
		printProgramsList(programs)
	}

	return nil
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
	data, err := json.Marshal(programs)
	if err != nil {
		logger.Errorf("JSON 序列化失败: %v", err)
		return
	}
	fmt.Println(string(data))
}
