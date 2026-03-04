# Go DevOps CLI Plugin

A Go-based CLI tool for interacting with the smartpos.top DevOps API. This plugin allows you to login to DevOps, query deploy versions, and deploy programs to servers.

## Features

- **Authentication**: Secure login to DevOps API using username and password
- **Program Management**: Check if programs exist in specific environments
- **Version Management**: Query available versions with fuzzy matching support
- **Server Management**: List servers and validate server existence
- **Concurrent Deployment**: Deploy to multiple servers concurrently using goroutines
- **SSH Auto-Retry**: Automatically retry deployment when SSH connection fails (up to 2 retries)
- **Deployment Waiting**: Wait for deployment tasks to complete with configurable timeout (10 minutes default)
- **Desktop Notifications**: Get notified on deployment start, success, failure, and retry via system notifications
- **Structured Logging**: Detailed logging with different log levels
- **Custom Error Handling**: Consistent error messages with error codes
- **Environment Variable Support**: Configure the plugin using environment variables

## Installation

1. **Clone the repository**:
   ```bash
   git clone https://github.com/your-username/go-devops.git
   cd go-devops
   ```

2. **Build the plugin**:
   ```bash
   go build -o go-devops
   ```

3. **Run the plugin**:
   ```bash
   ./go-devops --help
   ```

## Usage

### Basic Command Structure

```bash
./go-devops [flags]
./go-devops deploy [flags]
```

### Global Flags

| Flag | Short | Description | Default | Environment Variables |
|------|-------|-------------|---------|----------------------|
| `--host` | `-H` | DevOps base URL | `https://devops.example.com` | `DEVOPS_URL`, `URL` |
| `--username` | `-u` | DevOps username | | `DEVOPS_USERNAME`, `USERNAME` |
| `--password` | `-p` | DevOps password | | `DEVOPS_PASSWORD`, `PASSWORD` |
| `--debug` | | Enable debug mode | `false` | `DEVOPS_DEBUG`, `DEBUG` |

### Deploy Command Flags

| Flag | Short | Description | Default | Environment Variables |
|------|-------|-------------|---------|----------------------|
| `--program-alias` | `-a` | Program alias name | | `DEVOPS_PROGRAM_ALIAS`, `PROGRAM_ALIAS` |
| `--program-type` | `-t` | Program type (e.g. snapshots, releases) | `snapshots` | `DEVOPS_PROGRAM_TYPE`, `PROGRAM_TYPE` |
| `--env` | `-e` | Environment name (e.g. dev2, test) | | `DEVOPS_ENV`, `ENV` |
| `--version` | `-v` | Project version to deploy | | `DEVOPS_PROJECT_VERSION`, `PROJECT_VERSION` |
| `--server` | `-s` | Server alias to deploy to | | `DEVOPS_SERVER`, `SERVER` |
| `--notify` | | Users to notify on deployment | | `DEVOPS_NOTIFY_USER`, `NOTIFY_USER` |
| `--wait` | | Wait for deployment to complete | `false` | `DEVOPS_WAIT`, `WAIT` |

## Examples

### Deploy to Single Server and Wait

```bash
./go-devops deploy \
  -u admin \
  -p password123 \
  -a smartpos-svc-erp-chain \
  -e dev2 \
  -v 1.0.0 \
  -s dev2-zd1-erp-chain \
  --wait
```

### Deploy to Multiple Servers Concurrently

```bash
# Deploy to all servers for the program
./go-devops deploy \
  -a smartpos-svc-erp-chain \
  -e dev2 \
  -v 1.0.0 \
  --wait
```

### Deploy with Desktop Notifications

```bash
# Deployment notifications are sent automatically via system notifications
./go-devops deploy \
  -a my-app \
  -e dev2 \
  -v 1.0.0 \
  --notify user1,user2 \
  --wait
```

### Using Environment Variables

```bash
export DEVOPS_USERNAME=admin
export DEVOPS_PASSWORD=password123
export DEVOPS_PROGRAM_ALIAS=smartpos-svc-erp-chain
export DEVOPS_ENV=dev2
export DEVOPS_PROJECT_VERSION=1.0.0

./go-devops deploy --wait
```

## Project Structure

```
go-devops/
├── cmd/                      # CLI command implementations
│   ├── root.go              # Root command and global flags
│   ├── deploy.go            # Deploy command main flow
│   ├── deploy_validate.go   # Deployment validation logic
│   ├── deploy_execute.go    # Deployment execution logic
│   ├── deploy_monitor.go    # Concurrent monitoring and retry
│   ├── deploy_helpers.go    # Deployment helper functions
│   ├── deploy_notification.go # Desktop notifications
│   ├── envs.go              # Environment list command
│   ├── programs.go          # Program list command
│   ├── servers.go           # Server list command
│   ├── version.go           # Version query command
│   └── client.go            # Client creation
├── devops/                  # DevOps API client
│   ├── devops.go            # Client core
│   ├── login.go             # Authentication
│   ├── deploy.go            # Deployment API
│   ├── deploy_history.go    # Deployment history
│   ├── program_alias.go     # Program alias
│   ├── server.go            # Server management
│   ├── version.go           # Version management
│   └── wait.go              # Wait for deployment completion
├── internal/                # Internal packages
│   ├── errors/              # Custom error types
│   ├── logger/              # Logging management
│   └── version/             # Version matching
├── main.go                  # Entry point
├── go.mod                   # Go module file
├── README.md                # English documentation
└── README.zh-CN.md          # Chinese documentation
```

## Concurrent Deployment and Auto-Retry

### How It Works

1. **Concurrent Deployment**: Deployment tasks are executed concurrently to all target servers using goroutines
2. **Parallel Monitoring**: Each server deployment is monitored in a separate goroutine
3. **SSH Auto-Retry**: When SSH connection fails, the deployment is automatically retried (up to 2 retries)
4. **Retry Delay**: 5 seconds between retry attempts
5. **Timeout**: 10 minutes per task

### Notification Types

- **Deployment Started**: When deployment begins
- **Deployment Success**: When all servers complete successfully
- **Deployment Failure**: When deployment fails
- **Partial Failure**: When some servers fail
- **SSH Retry**: When SSH connection fails and retry is scheduled
- **Redeploy**: When redeploying to a failed server

## Version Matching

The plugin supports fuzzy version matching, which means it can match versions like:
- `4.32.0` with `4.32`
- `1.2.3` with `1.2`
- `5.0.0` with `5`

**Matching Priority:**
1. **Exact Match**: First attempts to match the exact version
2. **Fuzzy Match**: If exact match fails, uses fuzzy matching

## Logging

The plugin uses structured logging with different log levels:
- **INFO**: General information about the plugin's execution
- **DEBUG**: Detailed debugging information (enabled with `--debug` flag)
- **WARNING**: Warnings about potential issues
- **ERROR**: Error messages

## Custom Error Types

The plugin uses custom error types with error codes for consistent error handling:
- **VALIDATION_ERROR**: Invalid parameters
- **LOGIN_ERROR**: Login failures
- **API_ERROR**: DevOps API failures
- **DEPLOYMENT_ERROR**: Deployment failures
- **VERSION_ERROR**: Version-related errors
- **SERVER_ERROR**: Server-related errors

## Development

### Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/urfave/cli/v2` | v2.27.7 | CLI framework |
| `github.com/sirupsen/logrus` | v1.9.3 | Structured logging |
| `github.com/joho/godotenv` | v1.5.1 | Environment variable loading |
| `golang.org/x/sync` | v0.10.0 | Concurrency control |
| `github.com/gen2brain/beeep` | v0.11.2 | Desktop notifications |
| `github.com/PuerkitoBio/goquery` | v1.11.0 | HTML parsing |

### Running Tests

```bash
go test ./...
```

### Building with Version Info

```bash
go build -ldflags "-X main.Version=v1.0.0" -o go-devops
```

### Cross-Platform Build

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o go-devops-linux

# Windows
GOOS=windows GOARCH=amd64 go build -o go-devops.exe

# macOS
GOOS=darwin GOARCH=amd64 go build -o go-devops-mac
```

## Environment File Support

The plugin supports loading environment variables from a `.env` file. You can specify the path to the env file using the `DEVOPS_ENV_FILE` environment variable:

```bash
export DEVOPS_ENV_FILE=.env
./go-devops deploy -a my-app -e dev2 -v 1.0.0 --wait
```

### Example .env File

```bash
# .env
DEVOPS_URL=https://devops.example.com
DEVOPS_USERNAME=admin
DEVOPS_PASSWORD=your_password
DEVOPS_PROGRAM_ALIAS=smartpos-svc-erp-chain
DEVOPS_ENV=dev2
```

## Best Practices

### 1. Use Environment Variables for Configuration

Create a `.env` file to manage common configurations and avoid exposing sensitive information in the command line:

```bash
# .env (do not commit to version control)
DEVOPS_USERNAME=admin
DEVOPS_PASSWORD=your_password
```

### 2. Use --wait Flag for Critical Deployments

When deploying critical services, use the `--wait` flag to ensure deployment completion:

```bash
./go-devops deploy -a critical-service -e prod -v 1.0.0 --wait
```

### 3. Leverage Fuzzy Version Matching

Use simplified version numbers for deployment:

```bash
# Deploy the latest version of 4.32.x
./go-devops deploy -a my-app -e dev2 -v 4.32 --wait
```

### 4. Enable Debug Mode for Troubleshooting

Enable `--debug` to view detailed logs when encountering issues:

```bash
./go-devops deploy -a my-app -e dev2 -v 1.0.0 --wait --debug
```

### 5. Use Notification Feature

Notify relevant personnel when deploying important updates:

```bash
./go-devops deploy -a my-app -e prod -v 2.0.0 --notify devops,qa --wait
```

## Troubleshooting

### Common Issues

**1. Login Failed: Authentication Error**

- **Cause**: Incorrect username or password
- **Solution**:
  - Check `DEVOPS_USERNAME` and `DEVOPS_PASSWORD` environment variables
  - Verify the account exists and is active in the DevOps system

**2. Deployment Failed: Version Not Found**

- **Cause**: The specified version does not exist in the target environment
- **Solution**:
  - Use `./go-devops version -a <program> -e <env>` to view available versions
  - Check if the program type (snapshots/releases) is correct

**3. SSH Deployment Failed**

- **Cause**: SSH connection to target server failed
- **Solution**:
  - The tool will automatically retry failed servers (up to 2 retries)
  - Check the SSH service status on the target server
  - Verify network connectivity

**4. Server Not Found**

- **Cause**: The specified server alias does not exist
- **Solution**:
  - Use `./go-devops servers -a <program> -e <env>` to view available servers
  - Verify the server alias spelling

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Contact

For issues or suggestions, please submit an [Issue](https://github.com/your-username/go-devops/issues).
