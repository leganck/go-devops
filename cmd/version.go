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
			"推荐工作流: envs → programs -e → version -e -a\n" +
			"未显式指定 -t 且 snapshots 无版本时，会自动尝试 releases。\n\n" +
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
				Usage:   "程序类型 (snapshots 或 releases)；未指定且 snapshots 为空时自动尝试 releases",
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
				Name:  "json",
				Usage: "以 JSON 格式输出",
			},
		},
		Action: versionAction,
	}
}

// versionAction 执行版本查询操作
func versionAction(c *cli.Context) error {
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

	programType := c.String("program-type")
	versions, usedType, err := fetchVersionsWithFallback(
		c.Context,
		client,
		programAlias,
		env,
		programType,
		!c.IsSet("program-type"),
	)
	if err != nil {
		return err
	}

	if c.Bool("json") {
		printVersionsJSON(versions, usedType)
	} else {
		if usedType != programType {
			fmt.Printf("programType=%s（已从 %s 回退）\n\n", usedType, programType)
		} else {
			fmt.Printf("programType=%s\n\n", usedType)
		}
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
func printVersionsJSON(versions []devops.VersionItem, programType string) {
	type versionOutput struct {
		Version     string `json:"version"`
		Size        string `json:"size"`
		ModifyTime  string `json:"modifyTime"`
		Path        string `json:"path"`
		ProgramType string `json:"programType"`
	}

	var result []versionOutput
	for _, v := range versions {
		result = append(result, versionOutput{
			Version:     v.Version,
			Size:        v.Size,
			ModifyTime:  v.ModifyTime,
			Path:        v.RelativePath,
			ProgramType: programType,
		})
	}

	data, err := json.Marshal(result)
	if err != nil {
		logger.Errorf("JSON 序列化失败: %v", err)
		return
	}
	fmt.Println(string(data))
}
