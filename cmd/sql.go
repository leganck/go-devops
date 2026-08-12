package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"

	"github.com/urfave/cli/v2"
)

// sqlCommand 创建 SQL 查询子命令组
func sqlCommand() *cli.Command {
	return &cli.Command{
		Name:      "sql",
		Usage:     "通过 DevOps 数据源执行 SQL 查询与元数据浏览",
		UsageText: "go-devops sql <子命令> [选项]",
		Description: "调用 DevOps /dsourceDbexec API（与 devops-jdbc-driver 同源），支持数据源、表结构查询与 SQL 执行。\n\n" +
			"推荐工作流: envs → sql databases -e → sql tables/exec -e -d ...\n" +
			"环境仅 1 个数据源时可省略 -d。\n\n" +
			"示例:\n" +
			"  go-devops sql databases -e www_ali\n" +
			"  go-devops sql tables -e www_ali -d erp\n" +
			"  go-devops sql describe -e www_ali -d erp -t user\n" +
			"  go-devops sql exec -e www_ali -d erp --sql \"SELECT 1\" --all",
		Subcommands: []*cli.Command{
			sqlDatabasesCommand(),
			sqlTablesCommand(),
			sqlDescribeCommand(),
			sqlExecCommand(),
		},
	}
}

func sqlDatabasesCommand() *cli.Command {
	return &cli.Command{
		Name:      "databases",
		Usage:     "列出环境下的数据源",
		UsageText: "go-devops sql databases [选项]",
		Flags: []cli.Flag{
			sqlEnvFlag(),
			sqlJSONFlag(),
		},
		Action: sqlDatabasesAction,
	}
}

func sqlTablesCommand() *cli.Command {
	return &cli.Command{
		Name:      "tables",
		Usage:     "列出数据源下的表",
		UsageText: "go-devops sql tables [选项]",
		Flags: []cli.Flag{
			sqlEnvFlag(),
			sqlDatasourceFlag(),
			sqlJSONFlag(),
		},
		Action: sqlTablesAction,
	}
}

func sqlDescribeCommand() *cli.Command {
	return &cli.Command{
		Name:      "describe",
		Usage:     "查看表结构",
		UsageText: "go-devops sql describe [选项]",
		Flags: []cli.Flag{
			sqlEnvFlag(),
			sqlDatasourceFlag(),
			&cli.StringFlag{
				Name:    "table",
				Aliases: []string{"t"},
				Usage:   "表名",
				EnvVars: []string{"DEVOPS_TABLE", "TABLE"},
			},
			sqlJSONFlag(),
		},
		Action: sqlDescribeAction,
	}
}

func sqlExecCommand() *cli.Command {
	return &cli.Command{
		Name:      "exec",
		Usage:     "执行 SQL",
		UsageText: "go-devops sql exec [选项]",
		Description: "执行 SQL。select 结果默认单页；加 --all 自动翻页合并。\n" +
			"--sql 为空时从标准输入读取。",
		Flags: []cli.Flag{
			sqlEnvFlag(),
			sqlDatasourceFlag(),
			&cli.StringFlag{
				Name:    "sql",
				Usage:   "SQL 语句；为空时从 stdin 读取",
				EnvVars: []string{"DEVOPS_SQL", "SQL"},
			},
			&cli.IntFlag{
				Name:  "page",
				Usage: "页码（从 1 开始）",
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "每页行数（1-5000，默认 500）",
			},
			&cli.BoolFlag{
				Name:  "all",
				Usage: "select 自动翻页合并结果",
			},
			sqlJSONFlag(),
		},
		Action: sqlExecAction,
	}
}

func sqlEnvFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:    "env",
		Aliases: []string{"e"},
		Usage:   "环境名称",
		EnvVars: []string{"DEVOPS_ENV", "ENV"},
	}
}

func sqlDatasourceFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:    "datasource",
		Aliases: []string{"d"},
		Usage:   "数据源名称（支持 catalog name 或 datasourceName；环境仅 1 个时可省略）",
		EnvVars: []string{"DEVOPS_DATASOURCE", "DATASOURCE"},
	}
}

func sqlJSONFlag() *cli.BoolFlag {
	return &cli.BoolFlag{
		Name:  "json",
		Usage: "以 JSON 格式输出",
	}
}

func sqlDatabasesAction(c *cli.Context) error {
	env, err := requireEnv(c)
	if err != nil {
		return err
	}

	client, err := createAndLoginClient(c.Context, getConfig(c))
	if err != nil {
		return err
	}

	dbs, err := client.ListDatabases(c.Context, env)
	if err != nil {
		return errors.NewAPIError("获取数据源列表失败", err)
	}

	if c.Bool("json") {
		return printJSON(dbs)
	}
	if len(dbs) == 0 {
		logger.Infof("没有找到数据源")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDATASOURCE")
	for _, db := range dbs {
		fmt.Fprintf(w, "%s\t%s\n", db.Name, db.DatasourceName)
	}
	_ = w.Flush()
	fmt.Printf("\n共 %d 个数据源\n", len(dbs))
	return nil
}

func sqlTablesAction(c *cli.Context) error {
	env, err := requireEnv(c)
	if err != nil {
		return err
	}

	client, err := createAndLoginClient(c.Context, getConfig(c))
	if err != nil {
		return err
	}

	db, err := client.ResolveOrDefaultDatasource(c.Context, env, c.String("datasource"))
	if err != nil {
		return errors.NewAPIError("解析数据源失败", err)
	}

	tables, err := client.ListTables(c.Context, env, db.DatasourceName)
	if err != nil {
		return errors.NewAPIError("获取表列表失败", err)
	}

	if c.Bool("json") {
		return printJSON(tables)
	}
	if len(tables) == 0 {
		logger.Infof("没有找到表")
		return nil
	}
	fmt.Printf("数据源 %s (%s) 共 %d 张表:\n\n", db.Name, db.DatasourceName, len(tables))
	for _, table := range tables {
		fmt.Printf("  • %s\n", table)
	}
	return nil
}

func sqlDescribeAction(c *cli.Context) error {
	env, err := requireEnv(c)
	if err != nil {
		return err
	}
	table := c.String("table")
	if table == "" {
		return errors.NewValidationError("表名不能为空", nil)
	}

	client, err := createAndLoginClient(c.Context, getConfig(c))
	if err != nil {
		return err
	}

	db, err := client.ResolveOrDefaultDatasource(c.Context, env, c.String("datasource"))
	if err != nil {
		return errors.NewAPIError("解析数据源失败", err)
	}

	info, err := client.GetTableInfo(c.Context, env, db.DatasourceName, table)
	if err != nil {
		return errors.NewAPIError("获取表结构失败", err)
	}

	if c.Bool("json") {
		return printJSON(info)
	}

	fmt.Printf("表 %s.%s", db.Name, table)
	if info.TableComment != "" {
		fmt.Printf(" (%s)", info.TableComment)
	}
	fmt.Println()
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ORD\tFIELD\tTYPE\tNULL\tKEY\tDEFAULT\tEXTRA\tCOMMENT")
	for _, col := range info.Columns {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			col.OrdinalPosition,
			col.ColumnName,
			col.ColumnType,
			col.IsNullable,
			col.ColumnKey,
			col.ColumnDefault,
			col.Extra,
			col.ColumnComment,
		)
	}
	_ = w.Flush()

	if info.CreateTableSQL != "" {
		fmt.Printf("\nCREATE TABLE:\n%s\n", info.CreateTableSQL)
	}
	return nil
}

func sqlExecAction(c *cli.Context) error {
	env, err := requireEnv(c)
	if err != nil {
		return err
	}

	sqlText, err := readSQLText(c)
	if err != nil {
		return err
	}

	client, err := createAndLoginClient(c.Context, getConfig(c))
	if err != nil {
		return err
	}

	db, err := client.ResolveOrDefaultDatasource(c.Context, env, c.String("datasource"))
	if err != nil {
		return errors.NewAPIError("解析数据源失败", err)
	}

	req := &devops.ExecuteSQLRequest{
		EnvName:        env,
		DatasourceName: db.DatasourceName,
		SQL:            sqlText,
		Page:           c.Int("page"),
		Limit:          c.Int("limit"),
	}

	var result *devops.SQLExecutionResult
	if c.Bool("all") {
		result, err = client.ExecuteSQLAll(c.Context, req)
	} else {
		result, err = client.ExecuteSQL(c.Context, req)
	}
	if err != nil {
		return errors.NewAPIError("执行 SQL 失败", err)
	}

	if c.Bool("json") {
		return printJSON(result)
	}
	return printSQLResult(result)
}

func requireEnv(c *cli.Context) (string, error) {
	env := c.String("env")
	if env == "" {
		return "", errors.NewValidationError("环境名称不能为空", nil)
	}
	return env, nil
}

func readSQLText(c *cli.Context) (string, error) {
	sqlText := strings.TrimSpace(c.String("sql"))
	if sqlText != "" {
		return sqlText, nil
	}

	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", errors.NewValidationError("读取标准输入失败", err)
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return "", errors.NewValidationError("SQL 不能为空（请使用 --sql 或通过管道传入）", nil)
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", errors.NewValidationError("读取标准输入失败", err)
	}
	sqlText = strings.TrimSpace(string(data))
	if sqlText == "" {
		return "", errors.NewValidationError("SQL 不能为空", nil)
	}
	return sqlText, nil
}

func printJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errors.NewAPIError("JSON 序列化失败", err)
	}
	fmt.Println(string(data))
	return nil
}

func printSQLResult(result *devops.SQLExecutionResult) error {
	if result == nil {
		return nil
	}

	if result.IsSelect() {
		fmt.Printf("type=select  rows=%d  execTime=%dms", len(result.Rows), result.ExecTime)
		if result.HasMore {
			fmt.Printf("  hasMore=true")
		}
		if result.SQLPage {
			fmt.Printf("  sqlPage=true")
		}
		fmt.Println()
		if len(result.Columns) == 0 && len(result.Rows) == 0 {
			fmt.Println("(empty)")
			return nil
		}

		columns := result.Columns
		if len(columns) == 0 && len(result.Rows) > 0 {
			for k := range result.Rows[0] {
				columns = append(columns, k)
			}
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, strings.Join(columns, "\t"))
		for _, row := range result.Rows {
			cells := make([]string, len(columns))
			for i, col := range columns {
				cells[i] = formatCell(row[col])
			}
			fmt.Fprintln(w, strings.Join(cells, "\t"))
		}
		return w.Flush()
	}

	fmt.Printf("type=%s  affectedRows=%d  execTime=%dms\n", result.Type, result.AffectedRows, result.ExecTime)
	return nil
}

func formatCell(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// JSON numbers decode as float64
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprint(t)
	}
}
