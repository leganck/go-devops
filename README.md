# Go DevOps CLI

<div align="center">

**一个现代化的 Go 语言 DevOps 部署工具**

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

</div>

---

## 简介

`go-devops` 是一个基于 Go 语言开发的命令行工具，用于与 smartpos.top DevOps API 交互。它提供了一套完整的部署解决方案，支持并发部署、SSH 失败自动重试、桌面通知、版本模糊匹配等功能。

### 核心特性

- **安全认证** - 用户名密码登录；本地会话文件跨命令复用，失效自动重登
- **并发部署** - 使用 goroutine 并发监听多个服务器部署任务
- **智能重试** - SSH 连接失败时自动重试失败的服务器（最多 2 次）
- **桌面通知** - 部署开始、成功、失败、重试时发送系统通知
- **版本匹配** - 支持精确匹配和模糊匹配版本号
- **SQL 查询** - 通过 DevOps `/dsourceDbexec` API 浏览数据源并执行 SQL（与 devops-jdbc-driver 同源）
- **程序日志** - 通过 `/programlog` API 检索 SLS/ES 程序日志（与 Web「程序日志」页同源）
- **AI 友好发现** - servers 对齐 groupName、SQL 可省略唯一数据源、版本自动回退、history 反查部署信息
- **结构化日志** - 详细的日志输出，支持调试模式
- **环境变量** - 灵活的配置方式，支持 `.env` 文件
- **高性能** - 基于 Go 语言的协程实现，部署效率高

---

## 安装

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/your-username/go-devops.git
cd go-devops

# 构建可执行文件
go build -o go-devops

# (可选) 安装到 GOPATH/bin
go install
```

### 使用预编译二进制

下载对应平台的预编译二进制文件，添加到系统 PATH 即可。

---

## 快速开始

### 基本用法

```bash
# 部署程序到指定服务器
go-devops deploy \
  -u admin \
  -p password \
  -a smartpos-svc-erp-chain \
  -e dev2 \
  -v 1.0.0 \
  -s dev2-zd1-erp-chain \
  --wait
```

### 使用环境变量

```bash
# 创建 .env 文件
cat > .env << EOF
DEVOPS_USERNAME=admin
DEVOPS_PASSWORD=your_password
DEVOPS_URL=https://devops.example.com
DEVOPS_PROGRAM_ALIAS=smartpos-svc-erp-chain
DEVOPS_ENV=dev2
EOF

# 运行命令
go-devops deploy -e dev2 -v 1.0.0 --wait
```

---

## 命令说明

### 全局选项

| 选项 | 简写 | 描述 | 默认值 | 环境变量 |
|------|------|------|--------|----------|
| `--host` | `-H` | DevOps API 地址 | `https://devops.example.com` | `DEVOPS_URL`, `URL` |
| `--username` | `-u` | 登录用户名 | | `DEVOPS_USERNAME`, `USERNAME` |
| `--password` | `-p` | 登录密码 | | `DEVOPS_PASSWORD`, `PASSWORD` |
| `--debug` | | 启用调试模式 | `false` | `DEVOPS_DEBUG`, `DEBUG` |
| `--fresh-login` | | 忽略本地会话，强制重新登录 | `false` | `DEVOPS_FRESH_LOGIN=1` |

**本地会话（默认开启）：** Cookie 与权限缓存写入 `%USERPROFILE%\.go-devops\sessions\`（Linux/macOS：`$HOME/.go-devops/sessions/`），按 `host+username` 隔离，TTL **8 小时**，不保存密码。跨命令复用会话；API 判定会话失效时自动重登并重试一次。`logout` 可清除当前会话文件。

### 子命令

#### `logout` - 清除本地登录会话

删除当前 `--host` + `--username` 对应的本地会话文件（下次命令会重新登录）。

```bash
go-devops logout
```

#### 1. `deploy` - 部署程序

部署指定版本的程序到目标服务器。

```bash
go-devops deploy [选项]
```

| 选项 | 简写 | 描述 | 默认值 | 环境变量 |
|------|------|------|--------|----------|
| `--program-alias` | `-a` | 程序别名 | | `DEVOPS_PROGRAM_ALIAS` |
| `--program-type` | `-t` | 程序类型 (snapshots/releases) | `snapshots` | `DEVOPS_PROGRAM_TYPE` |
| `--env` | `-e` | 环境名称 | | `DEVOPS_ENV` |
| `--version` | `-v` | 部署版本 | | `DEVOPS_PROJECT_VERSION` |
| `--server` | `-s` | 目标服务器别名 | | `DEVOPS_SERVER` |
| `--notify` | | 通知用户（逗号分隔） | | `DEVOPS_NOTIFY_USER` |
| `--wait` | | 等待部署完成 | `false` | `DEVOPS_WAIT` |

**示例：**

```bash
# 部署到单个服务器
go-devops deploy -a my-app -e dev2 -v 1.0.0 -s server-1 --wait

# 并发部署到所有服务器
go-devops deploy -a my-app -e dev2 -v 1.0.0 --wait

# 部署并通知相关人员
go-devops deploy -a my-app -e dev2 -v 1.0.0 --notify user1,user2 --wait
```

**并发部署与重试机制：**

- 部署任务会并发执行到所有目标服务器
- 使用 goroutine 并发监听每个服务器的部署状态
- 当某个服务器因 **SSH 连接失败** 时，自动重试该服务器（最多 2 次）
- 重试间隔：5 秒
- 单个任务超时：10 分钟

#### 2. `envs` - 列出环境

列出当前用户具有 `deployProgram:page` 且权限 `Envs` 非空的环境。若为全局权限（Envs 为空），列表可能为空，需显式传 `-e`。

```bash
go-devops envs
```

**示例输出：**

```
可用环境列表:
- dev (开发环境)
- test (测试环境)
- prod (生产环境)
```

#### 3. `programs` - 列出程序

列出指定环境中的所有程序。

```bash
go-devops programs -e <环境名>
```

| 选项 | 简写 | 描述 | 环境变量 |
|------|------|------|----------|
| `--env` | `-e` | 环境名称 | `DEVOPS_ENV` |

**示例：**

```bash
# 列出 dev2 环境的所有程序
go-devops programs -e dev2
```

#### 4. `servers` - 列出服务器

列出指定程序和环境的服务器。会尝试对齐日志项目中的 `groupName` / `projectName`（`env-group-alias`）；数量对不上时 GROUP/PROJECT 显示为 `-`。

```bash
go-devops servers -a <程序别名> -e <环境名>
```

| 选项 | 简写 | 描述 | 环境变量 |
|------|------|------|----------|
| `--program-alias` | `-a` | 程序别名 | `DEVOPS_PROGRAM_ALIAS` |
| `--env` | `-e` | 环境名称 | `DEVOPS_ENV` |

**示例：**

```bash
# 列出程序的所有服务器
go-devops servers -a my-app -e dev2 --json
```

#### 5. `version` - 查询版本

查询指定程序的可用版本。未显式指定 `-t` 且 `snapshots` 无版本时，自动尝试 `releases`。

```bash
go-devops version -a <程序别名> -e <环境名> [-t <程序类型>]
```

| 选项 | 简写 | 描述 | 默认值 | 环境变量 |
|------|------|------|--------|----------|
| `--program-alias` | `-a` | 程序别名 | | `DEVOPS_PROGRAM_ALIAS` |
| `--env` | `-e` | 环境名称 | | `DEVOPS_ENV` |
| `--program-type` | `-t` | 程序类型 | `snapshots` | `DEVOPS_PROGRAM_TYPE` |

**示例：**

```bash
# 查询 snapshots（为空则自动尝试 releases）
go-devops version -a my-app -e dev2

# 仅查询 releases
go-devops version -a my-app -e dev2 -t releases
```

#### 6. `sql` - SQL 查询

通过 DevOps 数据源 API 浏览元数据并执行 SQL（与 DataGrip 使用的 `devops-jdbc-driver` 同一套后端接口，非直连数据库）。

环境仅 1 个数据源时可省略 `-d`；解析失败时会附带候选列表。

```bash
go-devops sql <子命令> [选项]
```

| 子命令 | 说明 |
|--------|------|
| `databases` | 列出环境下的数据源 |
| `tables` | 列出数据源下的表 |
| `describe` | 查看表结构 |
| `exec` | 执行 SQL |

| 选项 | 简写 | 描述 | 环境变量 |
|------|------|------|----------|
| `--env` | `-e` | 环境名称 | `DEVOPS_ENV` |
| `--datasource` | `-d` | 数据源（支持 name 或 datasourceName；唯一时可省略） | `DEVOPS_DATASOURCE` |
| `--table` | `-t` | 表名（`describe`） | `DEVOPS_TABLE` |
| `--sql` | | SQL 语句；为空时从 stdin 读取（`exec`） | `DEVOPS_SQL` |
| `--page` | | 页码，从 1 开始（`exec`） | |
| `--limit` | | 每页行数，1–5000，默认 500（`exec`） | |
| `--all` | | select 自动翻页合并结果（`exec`） | |
| `--json` | | JSON 格式输出 | |

**示例：**

```bash
# 列出数据源
go-devops sql databases -e www_ali

# 列出表
go-devops sql tables -e www_ali -d erp

# 查看表结构
go-devops sql describe -e www_ali -d erp -t user

# 执行查询（单页）
go-devops sql exec -e www_ali -d erp --sql "SELECT id, name FROM user LIMIT 10"

# 自动翻页合并
go-devops sql exec -e www_ali -d erp --sql "SELECT * FROM user" --all

# 管道传入 SQL，JSON 输出
echo "SELECT 1 AS n" | go-devops sql exec -e www_ali -d erp --json
```

#### 7. `logs` - 程序日志查询

通过 DevOps `/programlog` API 查询程序日志（与 Web「程序日志」页同源，非直连 SLS/ES）。

`projectName` 规则：`{环境}-{服务器组}-{程序别名}`，例如 `www_ali-z0-smartpos-svc-erp`。

```bash
go-devops logs <子命令> [选项]
```

| 子命令 | 说明 |
|--------|------|
| `projects` | 列出环境下的日志项目（`env-group-alias`） |
| `query` | 查询程序日志 |

| 选项 | 简写 | 描述 | 环境变量 |
|------|------|------|----------|
| `--env` | `-e` | 环境名称 | `DEVOPS_ENV` |
| `--group` | `-g` | 服务器组（可省略，自动解析） | `DEVOPS_GROUP` |
| `--program-alias` | `-a` | 程序别名 | `DEVOPS_PROGRAM_ALIAS` |
| `--project` | | 完整 projectName（优先） | `DEVOPS_LOG_PROJECT` |
| `--time` | | 时间范围 `MM-dd HH:mm:ss ~ MM-dd HH:mm:ss` | |
| `--since` | | 相对时长（如 `2h`）；未指定 `--time` 时生效 | |
| `--level` | | `INFO\|ERROR\|DEBUG\|WARN\|TRACE` | |
| `--thread` | | 线程名前缀 | |
| `--query` / `--unquery` | | message 包含 / 排除关键字 | |
| `--page` / `--limit` | | 分页（limit 1–500，默认 50） | |
| `--json` | | JSON 输出 | |
| `--verbose` | | 表格增加 LOCATION/TOPIC | |

**推荐工作流（尤其适合 AI）：**

```bash
# 1. 先发现可用 projectName
go-devops logs projects -e www_ali --json

# 2. 再查询（可省略 -g；多组命中时会提示候选）
go-devops logs query -e www_ali -a smartpos-svc-erp --level ERROR --since 2h

# 或显式指定
go-devops logs query --project www_ali-z0-smartpos-svc-erp --query timeout --limit 100
```

#### 8. `history` - 部署历史

查询部署历史（需 `deployHistory:list` 权限），可用于反查 `groupName` / `serverAlias` / 最近版本。

```bash
go-devops history -e <环境名> [--condition <别名>] [--status <n>] [--page] [--limit] [--json]
```

| 选项 | 简写 | 描述 | 默认值 | 环境变量 |
|------|------|------|--------|----------|
| `--env` | `-e` | 环境名称 | | `DEVOPS_ENV` |
| `--condition` | `-c` | 搜索条件（程序别名等） | | `DEVOPS_HISTORY_CONDITION` |
| `--status` | | 部署状态过滤 | | |
| `--page` / `--limit` | | 分页 | `1` / `20` | |
| `--json` | | JSON 输出 | | |

**示例：**

```bash
go-devops history -e www_ali --condition smartpos-svc-erp --limit 20 --json
```

---

## 版本匹配

工具支持**模糊版本匹配**，可以灵活指定版本号：

| 指定版本 | 可匹配的实际版本 |
|----------|------------------|
| `4.32` | `4.32.0`, `4.32.1`, `4.32.2` |
| `1.2` | `1.2.0`, `1.2.3`, `1.2.10` |
| `5` | `5.0.0`, `5.1.2`, `5.2.0` |

**匹配优先级：**

1. **精确匹配** - 首先尝试完全匹配版本号
2. **模糊匹配** - 精确匹配失败时，使用模糊匹配

---

## 桌面通知

工具支持桌面通知功能，在以下情况下会自动发送通知：

| 通知类型 | 触发时机 |
|----------|----------|
| 部署已启动 | 部署任务开始执行 |
| 部署完成 | 所有服务器部署成功 |
| 部署失败 | 部署任务失败 |
| 部署部分失败 | 部分服务器部署失败 |
| SSH 重试 | SSH 连接失败，准备重试 |
| 重新部署开始 | 开始重新部署到失败的服务器 |

通知通过系统通知中心发送，无需额外配置。

---

## 配置

### 环境变量

支持通过环境变量配置所有选项：

```bash
# 认证信息
export DEVOPS_URL=https://devops.example.com
export DEVOPS_USERNAME=admin
export DEVOPS_PASSWORD=your_password

# 部署配置
export DEVOPS_PROGRAM_ALIAS=my-app
export DEVOPS_ENV=dev2
export DEVOPS_PROGRAM_TYPE=snapshots
export DEVOPS_PROJECT_VERSION=1.0.0

# SQL 查询（可选）
export DEVOPS_DATASOURCE=erp
export DEVOPS_TABLE=user

# 程序日志（可选）
export DEVOPS_GROUP=z0
export DEVOPS_LOG_PROJECT=www_ali-z0-smartpos-svc-erp

# 自定义 .env 文件路径
export PLUGIN_ENV_FILE=/path/to/custom.env

# 运行
go-devops deploy --wait
```

### .env 文件

创建 `.env` 文件加载配置：

```bash
# .env
DEVOPS_URL=https://devops.example.com
DEVOPS_USERNAME=admin
DEVOPS_PASSWORD=your_password
DEVOPS_ENV=dev2
DEVOPS_PROGRAM_ALIAS=my-app
```

---

## 日志说明

### 日志级别

| 级别 | 说明 |
|------|------|
| `INFO` | 常规操作信息 |
| `WARNING` | 警告信息 |
| `ERROR` | 错误信息 |
| `DEBUG` | 调试信息（需启用 `--debug`） |

### 启用调试模式

```bash
go-devops deploy -a my-app -e dev2 -v 1.0.0 --wait --debug
```

---

## 开发

### 项目结构

```
go-devops/
├── cmd/                      # CLI 命令实现
│   ├── root.go              # 根命令和全局选项
│   ├── deploy.go            # 部署命令主流程
│   ├── deploy_validate.go   # 部署验证逻辑
│   ├── deploy_execute.go    # 部署执行逻辑
│   ├── deploy_monitor.go    # 并发监听与重试
│   ├── deploy_helpers.go    # 部署辅助函数
│   ├── deploy_notification.go # 桌面通知
│   ├── envs.go              # 环境列表命令
│   ├── programs.go          # 程序列表命令
│   ├── servers.go           # 服务器列表命令
│   ├── version.go           # 版本查询命令
│   ├── sql.go               # SQL 查询命令
│   ├── logs.go              # 程序日志命令
│   ├── history.go           # 部署历史命令
│   └── client.go            # 客户端创建
├── devops/                  # DevOps API 客户端
│   ├── devops.go            # 客户端核心
│   ├── login.go             # 认证
│   ├── deploy.go            # 部署 API
│   ├── deploy_history.go    # 部署历史
│   ├── program_alias.go     # 程序别名
│   ├── server.go            # 服务器
│   ├── server_test.go       # 服务器组对齐单测
│   ├── version.go           # 版本
│   ├── sql.go               # 数据源 SQL API
│   ├── sql_test.go          # SQL 解析单测
│   ├── program_log.go       # 程序日志 API
│   ├── program_log_test.go  # 程序日志单测
│   └── wait.go              # 等待完成
├── internal/                # 内部包
│   ├── errors/              # 错误处理
│   ├── logger/              # 日志管理
│   └── version/             # 版本匹配
├── main.go                  # 入口文件
└── go.mod                   # Go 模块
```

### 依赖项

| 包 | 版本 | 用途 |
|---|------|------|
| `github.com/urfave/cli/v2` | v2.27.7 | CLI 框架 |
| `github.com/sirupsen/logrus` | v1.9.3 | 结构化日志 |
| `github.com/joho/godotenv` | v1.5.1 | 环境变量加载 |
| `golang.org/x/sync` | v0.10.0 | 并发控制 |
| `github.com/gen2brain/beeep` | v0.11.2 | 桌面通知 |
| `github.com/PuerkitoBio/goquery` | v1.11.0 | HTML 解析 |
| `github.com/yassinebenaid/godump` | v0.11.1 | 调试输出 |

### 构建命令

```bash
# 标准构建
go build -o go-devops

# 带版本信息构建
go build -ldflags "-X main.Version=v1.0.0" -o go-devops

# 跨平台构建
GOOS=linux GOARCH=amd64 go build -o go-devops-linux
GOOS=windows GOARCH=amd64 go build -o go-devops.exe
GOOS=darwin GOARCH=amd64 go build -o go-devops-mac
```

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行测试并显示覆盖率
go test -cover ./...

# 运行特定包的测试
go test ./cmd/...
```

---

## 故障排查

### 常见问题

<details>
<summary><b>1. 登录失败：认证错误</b></summary>

**原因：** 用户名或密码错误

**解决：**
- 检查环境变量 `DEVOPS_USERNAME` 和 `DEVOPS_PASSWORD`
- 确认账号在 DevOps 系统中存在且状态正常
</details>

<details>
<summary><b>2. 部署失败：版本不存在</b></summary>

**原因：** 指定的版本在目标环境中不存在

**解决：**
- 使用 `go-devops version -a <程序> -e <环境>` 查看可用版本
- 检查程序类型（snapshots/releases）是否正确
</details>

<details>
<summary><b>3. SSH 部署失败</b></summary>

**原因：** 目标服务器 SSH 连接失败

**解决：**
- 工具会自动重试失败的服务器（最多 2 次）
- 检查目标服务器的 SSH 服务状态
- 确认网络连接正常
</details>

<details>
<summary><b>4. 服务器未找到</b></summary>

**原因：** 指定的服务器别名不存在

**解决：**
- 使用 `go-devops servers -a <程序> -e <环境>` 查看可用服务器
- 确认服务器别名拼写正确
</details>

---

## 最佳实践

### 1. 使用环境变量管理配置

创建 `.env` 文件管理常用配置，避免在命令行中暴露敏感信息：

```bash
# .env (不要提交到版本控制)
DEVOPS_USERNAME=admin
DEVOPS_PASSWORD=your_password
```

### 2. 使用 --wait 参数

部署关键服务时使用 `--wait` 参数确保部署完成：

```bash
go-devops deploy -a critical-service -e prod -v 1.0.0 --wait
```

### 3. 利用模糊版本匹配

使用简化版本号进行部署：

```bash
# 部署 4.32.x 的最新版本
go-devops deploy -a my-app -e dev2 -v 4.32 --wait
```

### 4. 启用调试模式排查问题

遇到问题时启用 `--debug` 查看详细日志：

```bash
go-devops deploy -a my-app -e dev2 -v 1.0.0 --wait --debug
```

### 5. 使用通知功能

部署重要更新时通知相关人员：

```bash
go-devops deploy -a my-app -e prod -v 2.0.0 --notify devops,qa --wait
```

---

## 许可证

本项目采用 [MIT 许可证](LICENSE)。

---

## 贡献

欢迎贡献代码！请随时提交 Pull Request。

---

## 联系方式

如有问题或建议，请提交 [Issue](https://github.com/your-username/go-devops/issues)。

---

<div align="center">

**用爱构建 | Powered by Go**

</div>
