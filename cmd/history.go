package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"go-devops/devops"
	"go-devops/internal/errors"

	"github.com/urfave/cli/v2"
)

// historyCommand 创建部署历史查询子命令
func historyCommand() *cli.Command {
	return &cli.Command{
		Name:      "history",
		Usage:     "查询部署历史",
		UsageText: "go-devops history [选项]",
		Description: "查询部署历史（需 deployHistory:list 权限）。\n" +
			"可用于反查 groupName / serverAlias / 最近版本。\n\n" +
			"推荐工作流: history -e <env> --condition <alias> --json\n\n" +
			"示例:\n" +
			"  go-devops history -e www_ali --condition smartpos-svc-erp --limit 20",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "env",
				Aliases: []string{"e"},
				Usage:   "环境名称",
				EnvVars: []string{"DEVOPS_DEPLOY_ENV", "DEVOPS_ENV", "ENV"},
			},
			&cli.StringFlag{
				Name:    "condition",
				Aliases: []string{"c"},
				Usage:   "搜索条件（程序别名/任务 ID 等）",
				EnvVars: []string{"DEVOPS_HISTORY_CONDITION"},
			},
			&cli.StringFlag{
				Name:  "status",
				Usage: "部署状态过滤（后端 deployStatus 枚举）",
			},
			&cli.IntFlag{
				Name:  "page",
				Usage: "页码（从 1 开始）",
				Value: 1,
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "每页条数",
				Value: 20,
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "以 JSON 格式输出",
			},
		},
		Action: historyAction,
	}
}

func historyAction(c *cli.Context) error {
	env := c.String("env")
	if env == "" {
		return errors.NewValidationError("环境名称不能为空", nil)
	}

	client, err := createAndLoginClient(c.Context, getConfig(c))
	if err != nil {
		return err
	}

	result, err := client.GetDeployHistory(c.Context, &devops.DeployHistoryRequest{
		Page:         c.Int("page"),
		Limit:        c.Int("limit"),
		EnvName:      env,
		DeployStatus: c.String("status"),
		Condition:    c.String("condition"),
	})
	if err != nil {
		return errors.NewAPIError("获取部署历史失败", err)
	}

	if c.Bool("json") {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return errors.NewAPIError("JSON 序列化失败", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if len(result.Data) == 0 {
		fmt.Println("没有找到部署历史")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tSTATUS\tGROUP\tPROGRAM\tSERVER\tVERSION\tTYPE")
	for _, item := range result.Data {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			formatHistoryTime(item.DeployTime),
			item.DeployStatus,
			emptyDash(item.GroupName),
			emptyDash(item.ProgramAliasName),
			emptyDash(item.ServerAlias),
			emptyDash(item.ProgramVersion),
			emptyDash(item.ProgramType),
		)
	}
	_ = w.Flush()
	fmt.Printf("\n本页 %d 条（count=%d）\n", len(result.Data), result.Count)
	return nil
}

func formatHistoryTime(ts int64) string {
	if ts <= 0 {
		return "-"
	}
	// 兼容秒/毫秒
	if ts > 1_000_000_000_000 {
		ts = ts / 1000
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
