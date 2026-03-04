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

// versionCommand 创建版本查询子命令
func versionCommand() *cli.Command {
	return &cli.Command{
		Name:      "version",
		Usage:     "查询程序可用版本",
		UsageText: "go-devops version [选项]",
		Description: "查询指定程序在指定环境中的可用版本列表。\n\n" +
			"示例:\n" +
			"  go-devops version -a smartpos-svc-erp-chain -e dev2 -t snapshots",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "program-alias",
				Aliases: []string{"a"},
				Usage:   "程序别名 (例如: smartpos-svc-erp-chain)",
				EnvVars: []string{"DEVOPS_PROGRAM_ALIAS", "PROGRAM_ALIAS"},
			},
			&cli.StringFlag{
				Name:    "program-type",
				Aliases: []string{"t"},
				Usage:   "程序类型 (snapshots 或 releases)",
				Value:   "snapshots",
				EnvVars: []string{"DEVOPS_PROGRAM_TYPE", "PROGRAM_TYPE"},
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
		Action: versionAction,
	}
}

// versionAction 执行版本查询操作
func versionAction(c *cli.Context) error {
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

	// 查询版本
	versions, err := client.GetVersion(c.Context, &devops.VersionRequest{
		ProgramAliasName: programAlias,
		ProgramType:      c.String("program-type"),
		EnvName:          env,
	})
	if err != nil {
		return errors.NewAPIError("获取版本失败", err)
	}

	// 输出结果
	if c.Bool("json") {
		printVersionsJSON(versions)
	} else {
		printVersionsTable(versions)
	}

	return nil
}

// printVersionsTable 以表格形式打印版本列表
func printVersionsTable(versions []devops.VersionItem) {
	if len(versions) == 0 {
		logger.Infof("没有找到版本")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "版本\t\t大小\t\t修改时间")
	fmt.Fprintln(w, "----\t\t----\t\t--------")

	for _, v := range versions {
		fmt.Fprintf(w, "%s\t\t%s\t\t%s\n", v.Version, v.Size, v.ModifyTime)
	}

	w.Flush()
	fmt.Printf("\n共 %d 个版本\n", len(versions))
}

// printVersionsJSON 以 JSON 格式打印版本列表
func printVersionsJSON(versions []devops.VersionItem) {
	// 创建简化的 JSON 输出结构
	type versionOutput struct {
		Version    string `json:"version"`
		Size       string `json:"size"`
		ModifyTime string `json:"modifyTime"`
		Path       string `json:"path"`
	}

	var result []versionOutput
	for _, v := range versions {
		result = append(result, versionOutput{
			Version:    v.Version,
			Size:       v.Size,
			ModifyTime: v.ModifyTime,
			Path:       v.RelativePath,
		})
	}

	data, err := json.Marshal(result)
	if err != nil {
		logger.Errorf("JSON 序列化失败: %v", err)
		return
	}
	fmt.Println(string(data))
}
