package main

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/leganck/go-devops/internal/logger"
)

func envsCommand() *cli.Command {
	return &cli.Command{
		Name:  "envs",
		Usage: "list deployProgram environments (not SQL/log envs)",
		Flags: []cli.Flag{&cli.BoolFlag{Name: "json"}},
		Action: func(c *cli.Context) error {
			cfg, err := getConfig(c)
			if err != nil {
				return err
			}
			api, err := deployAPI(c.Context, cfg)
			if err != nil {
				return err
			}
			envs := api.Environments()
			if c.Bool("json") {
				return printJSON(envs)
			}
			if len(envs) == 0 {
				logger.Infof("没有找到可用环境")
				fmt.Println("提示: 仅列出 deployProgram:page 且 Envs 非空的环境")
				return nil
			}
			for _, e := range envs {
				fmt.Printf("  • %s\n", e)
			}
			return nil
		},
	}
}

func programsCommand() *cli.Command {
	return &cli.Command{
		Name:  "programs",
		Usage: "list programs in a deploy env",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "env", Aliases: []string{"e"}, EnvVars: []string{"DEVOPS_DEPLOY_ENV"}},
			&cli.BoolFlag{Name: "json"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := getConfig(c)
			if err != nil {
				return err
			}
			if err := requireFlag("env", c.String("env")); err != nil {
				return err
			}
			api, err := deployAPI(c.Context, cfg)
			if err != nil {
				return err
			}
			list, err := api.Programs(c.Context, c.String("env"))
			if err != nil {
				return err
			}
			if c.Bool("json") {
				return printJSON(list)
			}
			for _, p := range list {
				fmt.Printf("  • %s\n", p)
			}
			return nil
		},
	}
}

func versionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "list program versions",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "program-alias", Aliases: []string{"a"}, EnvVars: []string{"DEVOPS_PROGRAM_ALIAS"}},
			&cli.StringFlag{Name: "program-type", Aliases: []string{"t"}, Value: "snapshots", EnvVars: []string{"DEVOPS_PROGRAM_TYPE"}},
			&cli.StringFlag{Name: "env", Aliases: []string{"e"}, EnvVars: []string{"DEVOPS_DEPLOY_ENV"}},
			&cli.BoolFlag{Name: "json"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := getConfig(c)
			if err != nil {
				return err
			}
			if err := requireFlag("env", c.String("env")); err != nil {
				return err
			}
			if err := requireFlag("program-alias", c.String("program-alias")); err != nil {
				return err
			}
			api, err := deployAPI(c.Context, cfg)
			if err != nil {
				return err
			}
			pt := c.String("program-type")
			vers, err := api.Versions(c.Context, versionReq(c.String("program-alias"), pt, c.String("env")))
			if err != nil {
				return err
			}
			if len(vers) == 0 && !c.IsSet("program-type") && pt == "snapshots" {
				alt, altErr := api.Versions(c.Context, versionReq(c.String("program-alias"), "releases", c.String("env")))
				if altErr != nil {
					return altErr
				}
				if len(alt) > 0 {
					vers = alt
					pt = "releases"
				}
			}
			if c.Bool("json") {
				return printJSON(vers)
			}
			fmt.Printf("programType=%s\n", pt)
			for _, v := range vers {
				fmt.Printf("%s\t%s\t%s\n", v.Version, v.Size, v.ModifyTime)
			}
			return nil
		},
	}
}

func serversCommand() *cli.Command {
	return &cli.Command{
		Name:  "servers",
		Usage: "list deploy servers",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "program-alias", Aliases: []string{"a"}, EnvVars: []string{"DEVOPS_PROGRAM_ALIAS"}},
			&cli.StringFlag{Name: "env", Aliases: []string{"e"}, EnvVars: []string{"DEVOPS_DEPLOY_ENV"}},
			&cli.BoolFlag{Name: "json"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := getConfig(c)
			if err != nil {
				return err
			}
			if err := requireFlag("env", c.String("env")); err != nil {
				return err
			}
			if err := requireFlag("program-alias", c.String("program-alias")); err != nil {
				return err
			}
			api, err := deployAPI(c.Context, cfg)
			if err != nil {
				return err
			}
			groups, err := api.Servers(c.Context, serverReq(c.String("env"), c.String("program-alias")))
			if err != nil {
				return err
			}
			aligned := alignOptional(c, api, groups, c.String("env"), c.String("program-alias"))
			if c.Bool("json") {
				return printJSON(aligned)
			}
			for _, g := range aligned {
				for _, s := range g.Servers {
					fmt.Printf("%s\t%s\t%s\t%s\n", dash(g.GroupName), dash(g.ProjectName), s.ServerAlias, s.ServerID)
				}
			}
			return nil
		},
	}
}

func historyCommand() *cli.Command {
	return &cli.Command{
		Name:  "history",
		Usage: "list deploy history",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "env", Aliases: []string{"e"}, EnvVars: []string{"DEVOPS_DEPLOY_ENV"}},
			&cli.StringFlag{Name: "condition", Aliases: []string{"c"}},
			&cli.StringFlag{Name: "status"},
			&cli.IntFlag{Name: "page", Value: 1},
			&cli.IntFlag{Name: "limit", Value: 20},
			&cli.BoolFlag{Name: "json"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := getConfig(c)
			if err != nil {
				return err
			}
			if err := requireFlag("env", c.String("env")); err != nil {
				return err
			}
			api, err := deployAPI(c.Context, cfg)
			if err != nil {
				return err
			}
			res, err := api.History(c.Context, histReq(c))
			if err != nil {
				return err
			}
			if c.Bool("json") {
				return printJSON(res)
			}
			for _, item := range res.Data {
				fmt.Printf("%d\t%s\t%s\t%s\n", item.DeployStatus, item.ProgramAliasName, item.ServerAlias, item.ProgramVersion)
			}
			return nil
		},
	}
}

func logoutCommand() *cli.Command {
	return &cli.Command{
		Name:  "logout",
		Usage: "clear local session",
		Action: func(c *cli.Context) error {
			cfg, err := getConfig(c)
			if err != nil {
				return err
			}
			core, err := newCore(cfg)
			if err != nil {
				return err
			}
			if err := core.ClearSession(c.Context); err != nil {
				return err
			}
			fmt.Println("logout ok")
			return nil
		},
	}
}
