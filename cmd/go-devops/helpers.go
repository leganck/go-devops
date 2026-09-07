package main

import (
	"github.com/urfave/cli/v2"

	"github.com/leganck/go-devops/devops/deploy"
	"github.com/leganck/go-devops/devops/logs"
)

func versionReq(alias, pt, env string) deploy.VersionRequest {
	return deploy.VersionRequest{ProgramAliasName: alias, ProgramType: pt, EnvName: env}
}

func serverReq(env, alias string) deploy.ServerRequest {
	return deploy.ServerRequest{EnvName: env, ProgramAliasName: alias}
}

func histReq(c *cli.Context) deploy.HistoryRequest {
	return deploy.HistoryRequest{
		Page:         c.Int("page"),
		Limit:        c.Int("limit"),
		EnvName:      c.String("env"),
		DeployStatus: c.String("status"),
		Condition:    c.String("condition"),
	}
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func alignOptional(c *cli.Context, d *deploy.Client, groups [][]deploy.Server, env, alias string) []deploy.ServerGroup {
	core := d.Core()
	lc := logs.New(core)
	projects, err := lc.ListProjects(c.Context, env)
	if err != nil {
		return deploy.AlignServerGroups(groups, nil)
	}
	return deploy.AlignServerGroups(groups, logs.FilterByAlias(projects, env, alias))
}
