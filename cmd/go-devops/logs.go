package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/leganck/go-devops/devops/logs"
)

func logsCommand() *cli.Command {
	return &cli.Command{
		Name:  "logs",
		Usage: "query program logs (keyword then time+thread context)",
		Description: "Two-step workflow:\n" +
			"  1. logs query --query KEYWORD\n" +
			"  2. logs query --time \"MM-dd HH:mm:ss ~ MM-dd HH:mm:ss\" --thread NAME",
		Subcommands: []*cli.Command{
			{Name: "projects", Flags: []cli.Flag{logsEnvFlag(), jsonFlag()}, Action: logsProjects},
			{Name: "query", Flags: []cli.Flag{
				logsEnvFlag(),
				&cli.StringFlag{Name: "group", Aliases: []string{"g"}},
				&cli.StringFlag{Name: "program-alias", Aliases: []string{"a"}, EnvVars: []string{"DEVOPS_PROGRAM_ALIAS"}},
				&cli.StringFlag{Name: "project", EnvVars: []string{"DEVOPS_LOG_PROJECT"}},
				&cli.StringFlag{Name: "time"},
				&cli.StringFlag{Name: "since"},
				&cli.StringFlag{Name: "level"},
				&cli.StringFlag{Name: "thread"},
				&cli.StringFlag{Name: "query"},
				&cli.StringFlag{Name: "unquery"},
				&cli.IntFlag{Name: "limit"},
				jsonFlag(),
				&cli.BoolFlag{Name: "verbose"},
			}, Action: logsQuery},
		},
	}
}

func logsEnvFlag() cli.Flag {
	return &cli.StringFlag{Name: "env", Aliases: []string{"e"}, Usage: "log env", EnvVars: []string{"DEVOPS_LOG_ENV"}}
}

func logsProjects(c *cli.Context) error {
	cfg, err := getConfig(c)
	if err != nil {
		return err
	}
	if err := requireFlag("env", c.String("env")); err != nil {
		return err
	}
	api, err := logsAPI(c.Context, cfg)
	if err != nil {
		return err
	}
	projects, err := api.ListProjects(c.Context, c.String("env"))
	if err != nil {
		return err
	}
	if c.Bool("json") {
		return printJSON(projects)
	}
	for _, p := range projects {
		fmt.Println(p)
	}
	return nil
}

func logsQuery(c *cli.Context) error {
	cfg, err := getConfig(c)
	if err != nil {
		return err
	}
	api, err := logsAPI(c.Context, cfg)
	if err != nil {
		return err
	}
	thread := c.String("thread")
	timeRange := strings.TrimSpace(c.String("time"))
	keyword := c.String("query")
	var res *logs.Result
	if thread != "" && timeRange != "" {
		res, err = api.Context(c.Context, logs.ContextRequest{
			ProjectName: c.String("project"),
			Env:         c.String("env"),
			Group:       c.String("group"),
			Alias:       c.String("program-alias"),
			TimeRange:   timeRange,
			Thread:      thread,
			Limit:       c.Int("limit"),
			Unquery:     c.String("unquery"),
		})
	} else {
		if keyword == "" {
			return flagError("use --query KEYWORD first, then --time + --thread for context")
		}
		if timeRange == "" && c.String("since") != "" {
			d, err := logs.ParseSince(c.String("since"))
			if err != nil {
				return err
			}
			timeRange = logs.BuildTimeRange(d, time.Now())
		}
		res, err = api.Search(c.Context, logs.SearchRequest{
			ProjectName: c.String("project"),
			Env:         c.String("env"),
			Group:       c.String("group"),
			Alias:       c.String("program-alias"),
			Query:       keyword,
			Unquery:     c.String("unquery"),
			Level:       c.String("level"),
			Time:        timeRange,
			Limit:       c.Int("limit"),
		})
	}
	if err != nil {
		return err
	}
	if c.Bool("json") {
		return printJSON(res)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tLEVEL\tTHREAD\tMESSAGE")
	for _, e := range res.Data {
		msg := strings.ReplaceAll(e.Message, "\n", "\\n")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.Time, e.Level, e.Thread, msg)
	}
	return w.Flush()
}
