# Go DevOps CLI 插件

一个基于 Go 语言开发的 CLI 工具，用于与 smartpos.top DevOps API 交互。该插件允许您登录 DevOps、查询部署版本以及将程序部署到服务器。

## 功能特性

- **认证**：使用用户名和密码安全登录 DevOps API
- **程序管理**：检查程序是否存在于特定环境中
- **版本管理**：查询可用版本，支持模糊匹配
- **服务器管理**：列出服务器并验证服务器存在性
- **部署**：将程序部署到指定服务器，并自动生成通知备忘录
- **结构化日志**：不同日志级别的详细日志记录
- **自定义错误处理**：带有错误代码的一致错误消息
- **环境变量支持**：使用环境变量配置插件

## 安装

1. **克隆仓库**：
   ```bash
   git clone https://github.com/your-username/go-devops.git
   cd go-devops
   ```

2. **构建插件**：
   ```bash
   go build -o devops-plugin
   ```

3. **运行插件**：
   ```bash
   ./devops-plugin --help
   ```

## 使用方法

### 基本命令结构

```bash
./devops-plugin [flags]
```

### 命令行参数

| 参数 | 简写 | 描述 | 默认值 | 环境变量 |
|------|------|------|--------|----------|
| `--host` | | DevOps 基础 URL | `https://devops.example.com` | `PLUGIN_URL`, `DEVOPS_URL`, `URL` |
| `--username` | `-u` | DevOps 用户名 | | `PLUGIN_USERNAME`, `DEVOPS_USERNAME`, `USERNAME` |
| `--password` | `-p` | DevOps 密码 | | `PLUGIN_PASSWORD`, `DEVOPS_PASSWORD`, `PASSWORD` |
| `--program-alias` | | 程序别名 | `smartpos-svc-erp-chain` | `PLUGIN_PROGRAM_ALIAS`, `DEVOPS_PROGRAM_ALIAS`, `PROGRAM_ALIAS` |
| `--program-type` | | 程序类型（如 snapshots, releases） | `snapshots` | `PLUGIN_PROGRAM_TYPE`, `DEVOPS_PROGRAM_TYPE`, `PROGRAM_TYPE` |
| `--env` | | 环境名称（如 dev2, test） | `dev2` | `PLUGIN_ENV`, `DEVOPS_ENV`, `ENV` |
| `--project-version` | | 要检查或部署的项目版本 | | `PLUGIN_PROJECT_VERSION`, `DEVOPS_PROJECT_VERSION`, `PROJECT_VERSION` |
| `--server` | | 要部署到的服务器别名 | | `PLUGIN_SERVER`, `DEVOPS_SERVER`, `SERVER` |
| `--notify-user` | | 部署时要通知的用户 | | `PLUGIN_NOTIFY_USER`, `DEVOPS_NOTIFY_USER`, `NOTIFY_USER` |
| `--debug` | | 启用调试模式 | `false` | `PLUGIN_DEBUG`, `DEVOPS_DEBUG`, `DEBUG` |
| `--version` | `-v` | 显示版本信息 | | |
| `--help` | `-h` | 显示帮助信息 | | |

## 示例

### 部署程序

```bash
./devops-plugin \
  --username admin \
  --password password123 \
  --program-alias smartpos-svc-erp-chain \
  --program-type snapshots \
  --env dev2 \
  --project-version 4.32 \
  --server dev2-zd1-erp-chain \
  --notify-user user1,user2 \
  --debug
```

### 使用环境变量

```bash
export PLUGIN_USERNAME=admin
export PLUGIN_PASSWORD=password123
export PLUGIN_PROGRAM_ALIAS=smartpos-svc-erp-chain
export PLUGIN_PROGRAM_TYPE=snapshots
export PLUGIN_ENV=dev2
export PLUGIN_PROJECT_VERSION=4.32
export PLUGIN_SERVER=dev2-zd1-erp-chain
export PLUGIN_DEBUG=true

./devops-plugin
```

## 项目结构

```
├── devops/             # DevOps API 客户端实现
│   ├── deploy.go       # 部署 API 方法
│   ├── devops.go       # DevOps 客户端核心
│   ├── login.go        # 认证方法
│   ├── program_alias.go # 程序别名方法
│   ├── server.go       # 服务器方法
│   └── version.go      # 版本方法
├── internal/           # 内部包（只能被项目内其他包访问）
│   ├── errors/         # 自定义错误类型
│   │   └── errors.go   # 错误定义和创建函数
│   ├── logger/         # 日志管理
│   │   └── logger.go   # 日志配置和包装函数
│   ├── utils/          # 工具函数
│   │   └── utils.go    # 通用工具函数
│   └── version/        # 版本处理
│       └── version_matcher.go # 版本匹配逻辑
├── go.mod              # Go 模块文件
├── go.sum              # Go 依赖校验和
├── main.go             # CLI 入口点
├── plugin.go           # 插件核心逻辑
├── README.md           # 英文文档
├── README.zh-CN.md     # 中文文档（本文件）
```

## 版本匹配

插件支持模糊版本匹配，这意味着它可以匹配如下版本：
- `4.32.0` 与 `4.32`
- `1.2.3` 与 `1.2`
- `5.0.0` 与 `5`

这允许在部署程序时更灵活地指定版本。

## 日志记录

插件使用结构化日志，具有不同的日志级别：
- **Info**：插件执行的一般信息
- **Debug**：详细的调试信息（通过 `--debug` 标志启用）
- **Warning**：关于潜在问题的警告
- **Error**：错误消息

## 自定义错误类型

插件使用带有错误代码的自定义错误类型，以实现一致的错误处理：
- **VALIDATION_ERROR**：无效参数
- **LOGIN_ERROR**：登录失败
- **API_ERROR**：DevOps API 失败
- **DEPLOYMENT_ERROR**：部署失败
- **VERSION_ERROR**：版本相关错误
- **SERVER_ERROR**：服务器相关错误

## 开发

### 依赖项

- [github.com/urfave/cli/v2](https://github.com/urfave/cli/v2) - CLI 框架
- [github.com/sirupsen/logrus](https://github.com/sirupsen/logrus) - 结构化日志
- [github.com/joho/godotenv](https://github.com/joho/godotenv) - 环境变量加载

### 运行测试

```bash
go test ./...
```

### 构建带有版本信息

```bash
go build -ldflags "-X main.Version=v1.0.0" -o devops-plugin
```

## 环境文件支持

插件支持从 `.env` 文件加载环境变量。您可以使用 `PLUGIN_ENV_FILE` 环境变量指定 env 文件的路径：

```bash
export PLUGIN_ENV_FILE=.env
./devops-plugin
```

## 许可证

本项目采用 MIT 许可证。详情请参阅 [LICENSE](LICENSE) 文件。
