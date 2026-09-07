# go-devops

Go SDK and thin CLI for a DevOps HTTP API. The library has no default host: callers must pass a base URL.

## Install

GitHub Releases（推荐，打 `v*` tag 后由 GoReleaser 产出多架构二进制和 checksums）：

https://github.com/leganck/go-devops/releases

```bash
go install github.com/leganck/go-devops/cmd/go-devops@v0.2.1
```

容器镜像：`ghcr.io/leganck/go-devops`

Module: `github.com/leganck/go-devops`

## SDK

```go
core, err := devops.New(
    devops.WithBaseURL(os.Getenv("DEVOPS_URL")),
    devops.WithCredentials(devops.Credentials{
        Username: os.Getenv("DEVOPS_USERNAME"),
        Password: os.Getenv("DEVOPS_PASSWORD"),
    }),
)
if err != nil { log.Fatal(err) }
if err := core.EnsureSession(ctx); err != nil { log.Fatal(err) }

d := deploy.New(core)
h, err := d.StartDeploy(ctx, deploy.StartRequest{
    Env: "dev2", ProgramAlias: "my-app", Version: "1.0.0",
}, "idempotency-key")
st, err := d.GetDeploy(ctx, h)

s := sql.New(core)
rows, err := s.Exec(ctx, sql.ExecRequest{EnvName: sqlEnv, DatasourceName: "db", SQL: "SELECT 1"})

l := logs.New(core)
hits, err := l.Search(ctx, logs.SearchRequest{Env: logEnv, Alias: "my-app", Query: "timeout"})
ctxRows, err := l.Context(ctx, logs.ContextRequest{Env: logEnv, Alias: "my-app", TimeRange: hits.Data[0].Time + " ~ ...", Thread: hits.Data[0].Thread})
```

Import only the capability you need (`devops/deploy`, `devops/sql`, `devops/logs`). SQL is read-only. Logs are two-step: keyword search, then time+thread context. There is no shared default environment.

## CLI

`--host` / `DEVOPS_URL` is required. SQL uses `DEVOPS_SQL_ENV`, logs use `DEVOPS_LOG_ENV`, deploy uses `DEVOPS_DEPLOY_ENV`.

```bash
export DEVOPS_URL=https://<your-devops-host>
export DEVOPS_USERNAME=...
export DEVOPS_PASSWORD=...

go-devops envs --json
go-devops programs -e dev2
go-devops deploy -a my-app -e dev2 -v 1.0.0 --wait --json
go-devops sql exec -e sql-env --sql "SELECT 1"
go-devops sql export -e sql-env --sql "SELECT * FROM t" -o "$HOME/tmp/out.csv"
go-devops logs query --project env-group-alias --query timeout --since 2h --json
go-devops logs query --project env-group-alias --time "01-02 15:04:05 ~ 01-02 23:59:59" --thread http-nio
```

`deploy --wait` prints a handle (`historyIds`, `idempotencyKey`) on stderr so a restarted process can call `GetDeploy` / `WaitDeploy` with the same key.
