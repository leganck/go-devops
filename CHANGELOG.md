# Changelog

## v0.2.1

- GitHub Actions CI（`main` / PR：tidy、vet、fmt、test、build）和 GoReleaser 发布流水线（`v*` tag：多架构二进制 + GHCR 镜像）。

## v0.2.0

- Module path is `github.com/leganck/go-devops`.
- Library layout: `devops` kernel plus `devops/deploy`, `devops/sql`, `devops/logs`.
- Thin CLI lives in `cmd/go-devops`. `--host` / `DEVOPS_URL` is required; no default site.
- Deploy is recoverable: `StartDeploy` + `GetDeploy` + `WaitDeploy` with an idempotency key.
- SQL is read-only by default and supports streaming `Export`.
- Logs expose `Search` then `Context` (keyword, then time+thread).
- Session store, credentials, logger, clock, and HTTP client are injectable.
- Stable error kinds: unauthorized, forbidden, invalid argument, not found, version not found, rate limited, transient, server.
