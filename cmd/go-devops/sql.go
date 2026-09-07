package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v2"

	"github.com/leganck/go-devops/devops/sql"
)

func sqlCommand() *cli.Command {
	return &cli.Command{
		Name:  "sql",
		Usage: "read-only SQL via DevOps datasource",
		Subcommands: []*cli.Command{
			{Name: "databases", Flags: []cli.Flag{sqlEnvFlag(), jsonFlag()}, Action: sqlDatabases},
			{Name: "tables", Flags: []cli.Flag{sqlEnvFlag(), sqlDSFlag(), jsonFlag()}, Action: sqlTables},
			{Name: "describe", Flags: []cli.Flag{sqlEnvFlag(), sqlDSFlag(), &cli.StringFlag{Name: "table", Aliases: []string{"t"}}, jsonFlag()}, Action: sqlDescribe},
			{Name: "exec", Flags: []cli.Flag{
				sqlEnvFlag(), sqlDSFlag(),
				&cli.StringFlag{Name: "sql", EnvVars: []string{"DEVOPS_SQL"}},
				&cli.IntFlag{Name: "page"},
				&cli.IntFlag{Name: "limit"},
				&cli.BoolFlag{Name: "all"},
				jsonFlag(),
			}, Action: sqlExec},
			{Name: "export", Flags: []cli.Flag{
				sqlEnvFlag(), sqlDSFlag(),
				&cli.StringFlag{Name: "sql", EnvVars: []string{"DEVOPS_SQL"}},
				&cli.StringFlag{Name: "format", Value: "csv"},
				&cli.StringFlag{Name: "output", Aliases: []string{"o"}},
			}, Action: sqlExport},
		},
	}
}

func sqlEnvFlag() cli.Flag {
	return &cli.StringFlag{Name: "env", Aliases: []string{"e"}, Usage: "SQL env", EnvVars: []string{"DEVOPS_SQL_ENV"}}
}

func sqlDSFlag() cli.Flag {
	return &cli.StringFlag{Name: "datasource", Aliases: []string{"d"}, EnvVars: []string{"DEVOPS_DATASOURCE"}}
}

func jsonFlag() cli.Flag { return &cli.BoolFlag{Name: "json"} }

func sqlDatabases(c *cli.Context) error {
	cfg, err := getConfig(c)
	if err != nil {
		return err
	}
	if err := requireFlag("env", c.String("env")); err != nil {
		return err
	}
	api, err := sqlAPI(c.Context, cfg)
	if err != nil {
		return err
	}
	dbs, err := api.ListDatabases(c.Context, c.String("env"))
	if err != nil {
		return err
	}
	if c.Bool("json") {
		return printJSON(dbs)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDATASOURCE")
	for _, db := range dbs {
		fmt.Fprintf(w, "%s\t%s\n", db.Name, db.DatasourceName)
	}
	return w.Flush()
}

func sqlTables(c *cli.Context) error {
	cfg, err := getConfig(c)
	if err != nil {
		return err
	}
	if err := requireFlag("env", c.String("env")); err != nil {
		return err
	}
	api, err := sqlAPI(c.Context, cfg)
	if err != nil {
		return err
	}
	db, err := api.ResolveOrDefaultDatasource(c.Context, c.String("env"), c.String("datasource"))
	if err != nil {
		return err
	}
	tables, err := api.ListTables(c.Context, c.String("env"), db.DatasourceName)
	if err != nil {
		return err
	}
	if c.Bool("json") {
		return printJSON(tables)
	}
	for _, t := range tables {
		fmt.Println(t)
	}
	return nil
}

func sqlDescribe(c *cli.Context) error {
	cfg, err := getConfig(c)
	if err != nil {
		return err
	}
	if err := requireFlag("env", c.String("env")); err != nil {
		return err
	}
	if err := requireFlag("table", c.String("table")); err != nil {
		return err
	}
	api, err := sqlAPI(c.Context, cfg)
	if err != nil {
		return err
	}
	db, err := api.ResolveOrDefaultDatasource(c.Context, c.String("env"), c.String("datasource"))
	if err != nil {
		return err
	}
	info, err := api.GetTableInfo(c.Context, c.String("env"), db.DatasourceName, c.String("table"))
	if err != nil {
		return err
	}
	if c.Bool("json") {
		return printJSON(info)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ORD\tFIELD\tTYPE\tNULL\tKEY")
	for _, col := range info.Columns {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", col.OrdinalPosition, col.ColumnName, col.ColumnType, col.IsNullable, col.ColumnKey)
	}
	return w.Flush()
}

func sqlExec(c *cli.Context) error {
	cfg, err := getConfig(c)
	if err != nil {
		return err
	}
	if err := requireFlag("env", c.String("env")); err != nil {
		return err
	}
	text, err := readSQL(c)
	if err != nil {
		return err
	}
	api, err := sqlAPI(c.Context, cfg)
	if err != nil {
		return err
	}
	db, err := api.ResolveOrDefaultDatasource(c.Context, c.String("env"), c.String("datasource"))
	if err != nil {
		return err
	}
	req := sql.ExecRequest{EnvName: c.String("env"), DatasourceName: db.DatasourceName, SQL: text, Page: c.Int("page"), Limit: c.Int("limit")}
	var res *sql.Result
	if c.Bool("all") {
		res, err = api.ExecAll(c.Context, req)
	} else {
		res, err = api.Exec(c.Context, req)
	}
	if err != nil {
		return err
	}
	if c.Bool("json") {
		return printJSON(res)
	}
	if res.IsSelect() {
		fmt.Printf("type=select rows=%d\n", len(res.Rows))
		cols := res.Columns
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, strings.Join(cols, "\t"))
		for _, row := range res.Rows {
			cells := make([]string, len(cols))
			for i, col := range cols {
				cells[i] = fmt.Sprint(row[col])
			}
			fmt.Fprintln(w, strings.Join(cells, "\t"))
		}
		return w.Flush()
	}
	fmt.Printf("type=%s affected=%d\n", res.Type, res.AffectedRows)
	return nil
}

func sqlExport(c *cli.Context) error {
	cfg, err := getConfig(c)
	if err != nil {
		return err
	}
	if err := requireFlag("env", c.String("env")); err != nil {
		return err
	}
	out := c.String("output")
	if out == "" {
		return flagError("--output is required")
	}
	text, err := readSQL(c)
	if err != nil {
		return err
	}
	api, err := sqlAPI(c.Context, cfg)
	if err != nil {
		return err
	}
	db, err := api.ResolveOrDefaultDatasource(c.Context, c.String("env"), c.String("datasource"))
	if err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	return api.Export(c.Context, sql.ExportRequest{
		EnvName: c.String("env"), DatasourceName: db.DatasourceName, SQL: text, Format: c.String("format"),
	}, f)
}

func readSQL(c *cli.Context) (string, error) {
	text := strings.TrimSpace(c.String("sql"))
	if text != "" {
		return text, nil
	}
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		return "", flagError("SQL is required (--sql or stdin)")
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(string(data))
	if text == "" {
		return "", flagError("SQL is empty")
	}
	return text, nil
}
