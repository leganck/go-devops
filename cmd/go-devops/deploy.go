package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/leganck/go-devops/devops/deploy"
	"github.com/leganck/go-devops/internal/logger"
)

func deployCommand() *cli.Command {
	return &cli.Command{
		Name:      "deploy",
		Usage:     "deploy a program",
		UsageText: "go-devops deploy [options]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "program-alias", Aliases: []string{"a"}, EnvVars: []string{"DEVOPS_PROGRAM_ALIAS"}},
			&cli.StringFlag{Name: "program-type", Aliases: []string{"t"}, Value: "snapshots", EnvVars: []string{"DEVOPS_PROGRAM_TYPE"}},
			&cli.StringFlag{Name: "env", Aliases: []string{"e"}, Usage: "deploy env", EnvVars: []string{"DEVOPS_DEPLOY_ENV"}},
			&cli.StringFlag{Name: "version", Aliases: []string{"v"}, EnvVars: []string{"DEVOPS_PROJECT_VERSION"}},
			&cli.StringFlag{Name: "server", Aliases: []string{"s"}, EnvVars: []string{"DEVOPS_SERVER"}},
			&cli.StringFlag{Name: "notify", EnvVars: []string{"DEVOPS_NOTIFY_USER"}},
			&cli.StringFlag{Name: "idempotency-key", Aliases: []string{"k"}, Usage: "resume key; generated if empty"},
			&cli.BoolFlag{Name: "wait", EnvVars: []string{"DEVOPS_WAIT"}},
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
			if err := requireFlag("version", c.String("version")); err != nil {
				return err
			}
			api, err := deployAPI(c.Context, cfg)
			if err != nil {
				return err
			}
			key := c.String("idempotency-key")
			if key == "" {
				key = newKey()
			}
			h, err := api.StartDeploy(c.Context, deploy.StartRequest{
				Env:          c.String("env"),
				ProgramAlias: c.String("program-alias"),
				ProgramType:  c.String("program-type"),
				Version:      c.String("version"),
				ServerAlias:  c.String("server"),
				NotifyUser:   c.String("notify"),
				TypeFallback: !c.IsSet("program-type"),
			}, key)
			if err != nil {
				return err
			}
			printHandle(h, c.Bool("json"))
			if !c.Bool("wait") {
				sendNotification("部署已启动", fmt.Sprintf("%s v%s key=%s", h.Alias, h.Version, h.IdempotencyKey))
				return nil
			}
			st, err := api.WaitDeploy(c.Context, h)
			if err != nil {
				sendNotification("部署失败", err.Error())
				return err
			}
			if c.Bool("json") {
				return printJSON(st)
			}
			if st.OK {
				sendNotification("部署完成", fmt.Sprintf("%s v%s ok servers=%d", h.Alias, h.Version, len(st.Servers)))
				logger.Infof("deploy ok servers=%d", len(st.Servers))
			}
			return nil
		},
	}
}

func newKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func printHandle(h *deploy.Handle, asJSON bool) {
	if asJSON {
		_ = printJSON(h)
		return
	}
	fmt.Fprintf(os.Stderr, "handle env=%s alias=%s version=%s key=%s history=%v\n",
		h.Env, h.Alias, h.Version, h.IdempotencyKey, h.HistoryIDs)
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
