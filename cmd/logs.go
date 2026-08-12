package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"

	"github.com/urfave/cli/v2"
)

// logsCommand 创建程序日志查询子命令组
func logsCommand() *cli.Command {
	return &cli.Command{
		Name:      "logs",
		Usage:     "查询 DevOps 程序日志（SLS/ES）",
		UsageText: "go-devops logs <子命令> [选项]",
		Description: "调用 /programlog API（与 Web「程序日志」页同源）。\n\n" +
			"AI/推荐工作流:\n" +
			"  1. go-devops logs projects -e www_ali --json\n" +
			"  2. 选中 projectName（或使用 -e/-g/-a）\n" +
			"  3. go-devops logs query --project ... --level ERROR --since 2h\n\n" +
			"示例:\n" +
			"  go-devops logs projects -e www_ali\n" +
			"  go-devops logs query -e www_ali -a smartpos-svc-erp --level ERROR\n" +
			"  go-devops logs query -e www_ali -g z0 -a smartpos-svc-erp --query timeout",
		Subcommands: []*cli.Command{
			logsProjectsCommand(),
			logsQueryCommand(),
		},
	}
}

func logsProjectsCommand() *cli.Command {
	return &cli.Command{
		Name:      "projects",
		Usage:     "列出环境下的日志项目（env-group-alias）",
		UsageText: "go-devops logs projects [选项]",
		Flags: []cli.Flag{
			logsEnvFlag(),
			logsJSONFlag(),
		},
		Action: logsProjectsAction,
	}
}

func logsQueryCommand() *cli.Command {
	return &cli.Command{
		Name:      "query",
		Usage:     "查询程序日志",
		UsageText: "go-devops logs query [选项]",
		Description: "projectName 规则: {env}-{group}-{alias}。\n" +
			"可省略 -g：将自动匹配；多个服务器组时会列出候选。",
		Flags: []cli.Flag{
			logsEnvFlag(),
			&cli.StringFlag{
				Name:    "group",
				Aliases: []string{"g"},
				Usage:   "服务器组（如 z0）；可省略以自动解析",
				EnvVars: []string{"DEVOPS_GROUP", "GROUP"},
			},
			&cli.StringFlag{
				Name:    "program-alias",
				Aliases: []string{"a"},
				Usage:   "程序别名",
				EnvVars: []string{"DEVOPS_PROGRAM_ALIAS", "PROGRAM_ALIAS"},
			},
			&cli.StringFlag{
				Name:    "project",
				Usage:   "完整 projectName（优先于 -e/-g/-a）",
				EnvVars: []string{"DEVOPS_LOG_PROJECT", "LOG_PROJECT"},
			},
			&cli.StringFlag{
				Name:  "time",
				Usage: "时间范围，格式: MM-dd HH:mm:ss ~ MM-dd HH:mm:ss",
			},
			&cli.StringFlag{
				Name:  "since",
				Usage: "相对时长（如 2h、30m）；在未指定 --time 时生成时间范围",
			},
			&cli.StringFlag{
				Name:  "level",
				Usage: "日志级别: INFO|ERROR|DEBUG|WARN|TRACE",
			},
			&cli.StringFlag{
				Name:  "thread",
				Usage: "线程名前缀",
			},
			&cli.StringFlag{
				Name:  "query",
				Usage: "message 包含关键字",
			},
			&cli.StringFlag{
				Name:  "unquery",
				Usage: "message 排除关键字",
			},
			&cli.IntFlag{
				Name:  "page",
				Usage: "页码（从 1 开始）",
				Value: 1,
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "每页行数（1-500，默认 50）",
			},
			logsJSONFlag(),
			&cli.BoolFlag{
				Name:  "verbose",
				Usage: "表格输出增加 LOCATION/TOPIC 列",
			},
		},
		Action: logsQueryAction,
	}
}

func logsEnvFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:    "env",
		Aliases: []string{"e"},
		Usage:   "日志环境（programlog，与部署/SQL 环境不一定相同）",
		EnvVars: []string{"DEVOPS_LOG_ENV", "DEVOPS_ENV", "ENV"},
	}
}

func logsJSONFlag() *cli.BoolFlag {
	return &cli.BoolFlag{
		Name:  "json",
		Usage: "以 JSON 格式输出",
	}
}

func logsProjectsAction(c *cli.Context) error {
	env := c.String("env")
	if env == "" {
		return errors.NewValidationError("环境名称不能为空", nil)
	}

	client, err := createAndLoginClient(c.Context, getConfig(c))
	if err != nil {
		return err
	}

	projects, err := client.ListLogProjects(c.Context, env)
	if err != nil {
		return errors.NewAPIError("获取日志项目列表失败", err)
	}

	if c.Bool("json") {
		return printLogsJSON(projects)
	}
	if len(projects) == 0 {
		logger.Infof("没有找到日志项目")
		return nil
	}
	fmt.Printf("环境 %s 共 %d 个日志项目:\n\n", env, len(projects))
	for _, p := range projects {
		fmt.Printf("  • %s\n", p)
	}
	return nil
}

func logsQueryAction(c *cli.Context) error {
	client, err := createAndLoginClient(c.Context, getConfig(c))
	if err != nil {
		return err
	}

	projectName, err := client.ResolveLogProject(
		c.Context,
		c.String("env"),
		c.String("group"),
		c.String("program-alias"),
		c.String("project"),
	)
	if err != nil {
		return errors.NewValidationError(err.Error(), nil)
	}

	timeRange := strings.TrimSpace(c.String("time"))
	if timeRange == "" && c.String("since") != "" {
		since, err := devops.ParseSinceDuration(c.String("since"))
		if err != nil {
			return errors.NewValidationError(err.Error(), nil)
		}
		timeRange = devops.BuildProgramLogTimeRange(since, time.Now())
	}

	result, err := client.ListProgramLogs(c.Context, &devops.ProgramLogRequest{
		ProjectName: projectName,
		Page:        c.Int("page"),
		Limit:       c.Int("limit"),
		Time:        timeRange,
		Level:       c.String("level"),
		ThreadName:  c.String("thread"),
		Query:       c.String("query"),
		Unquery:     c.String("unquery"),
	})
	if err != nil {
		return errors.NewAPIError("查询程序日志失败", err)
	}

	if c.Bool("json") {
		return printLogsJSON(map[string]interface{}{
			"projectName": projectName,
			"count":       result.Count,
			"data":        result.Data,
		})
	}

	fmt.Printf("project=%s  count=%d  rows=%d\n\n", projectName, result.Count, len(result.Data))
	return printProgramLogsTable(result.Data, c.Bool("verbose"))
}

func printLogsJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errors.NewAPIError("JSON 序列化失败", err)
	}
	fmt.Println(string(data))
	return nil
}

func printProgramLogsTable(entries []devops.ProgramLogEntry, verbose bool) error {
	if len(entries) == 0 {
		fmt.Println("(empty)")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if verbose {
		fmt.Fprintln(w, "TIME\tLEVEL\tTHREAD\tLOCATION\tTOPIC\tMESSAGE")
		for _, e := range entries {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				e.Time, e.Level, e.Thread, e.Location, e.Topic, compactMessage(e.Message))
		}
	} else {
		fmt.Fprintln(w, "TIME\tLEVEL\tTHREAD\tMESSAGE")
		for _, e := range entries {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				e.Time, e.Level, e.Thread, compactMessage(e.Message))
		}
	}
	return w.Flush()
}

func compactMessage(msg string) string {
	msg = strings.ReplaceAll(msg, "\r\n", "\\n")
	msg = strings.ReplaceAll(msg, "\n", "\\n")
	msg = strings.ReplaceAll(msg, "\t", " ")
	return msg
}
