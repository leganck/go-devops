# Go DevOps CLI Plugin

A Go-based CLI tool for interacting with the smartpos.top DevOps API. This plugin allows you to login to DevOps, query deploy versions, and deploy programs to servers.

## Features

- **Authentication**: Secure login to DevOps API using username and password
- **Program Management**: Check if programs exist in specific environments
- **Version Management**: Query available versions with fuzzy matching support
- **Server Management**: List servers and validate server existence
- **Deployment**: Deploy programs to specified servers with automatic notification memo generation
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
   go build -o devops-plugin
   ```

3. **Run the plugin**:
   ```bash
   ./devops-plugin --help
   ```

## Usage

### Basic Command Structure

```bash
./devops-plugin [flags]
```

### Flags

| Flag | Short | Description | Default | Environment Variables |
|------|-------|-------------|---------|----------------------|
| `--host` | | DevOps base URL | `https://devops.example.com` | `PLUGIN_URL`, `DEVOPS_URL`, `URL` |
| `--username` | `-u` | DevOps username | | `PLUGIN_USERNAME`, `DEVOPS_USERNAME`, `USERNAME` |
| `--password` | `-p` | DevOps password | | `PLUGIN_PASSWORD`, `DEVOPS_PASSWORD`, `PASSWORD` |
| `--program-alias` | | Program alias name | `smartpos-svc-erp-chain` | `PLUGIN_PROGRAM_ALIAS`, `DEVOPS_PROGRAM_ALIAS`, `PROGRAM_ALIAS` |
| `--program-type` | | Program type (e.g. snapshots, releases) | `snapshots` | `PLUGIN_PROGRAM_TYPE`, `DEVOPS_PROGRAM_TYPE`, `PROGRAM_TYPE` |
| `--env` | | Environment name (e.g. dev2, test) | `dev2` | `PLUGIN_ENV`, `DEVOPS_ENV`, `ENV` |
| `--project-version` | | Project version to check or deploy | | `PLUGIN_PROJECT_VERSION`, `DEVOPS_PROJECT_VERSION`, `PROJECT_VERSION` |
| `--server` | | Server alias to deploy to | | `PLUGIN_SERVER`, `DEVOPS_SERVER`, `SERVER` |
| `--notify-user` | | Users to notify on deployment | | `PLUGIN_NOTIFY_USER`, `DEVOPS_NOTIFY_USER`, `NOTIFY_USER` |
| `--debug` | | Enable debug mode | `false` | `PLUGIN_DEBUG`, `DEVOPS_DEBUG`, `DEBUG` |
| `--version` | `-v` | Show version information | | |
| `--help` | `-h` | Show help information | | |

## Examples

### Deploy a Program

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

### Using Environment Variables

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

## Project Structure

```
├── devops/             # DevOps API client implementation
│   ├── deploy.go       # Deployment API methods
│   ├── devops.go       # DevOps client core
│   ├── login.go        # Authentication methods
│   ├── program_alias.go # Program alias methods
│   ├── server.go       # Server methods
│   └── version.go      # Version methods
├── internal/           # Internal packages (only accessible within the project)
│   ├── errors/         # Custom error types
│   │   └── errors.go   # Error definitions and creation functions
│   ├── logger/         # Logging management
│   │   └── logger.go   # Logging configuration and wrapper functions
│   ├── utils/          # Utility functions
│   │   └── utils.go    # Common utility functions
│   └── version/        # Version handling
│       └── version_matcher.go # Version matching logic
├── go.mod              # Go module file
├── go.sum              # Go dependency checksums
├── main.go             # CLI entry point
├── plugin.go           # Plugin core logic
├── README.md           # English documentation
├── README.zh-CN.md     # Chinese documentation
```

## Version Matching

The plugin supports fuzzy version matching, which means it can match versions like:
- `4.32.0` with `4.32`
- `1.2.3` with `1.2`
- `5.0.0` with `5`

This allows for more flexible version specification when deploying programs.

## Logging

The plugin uses structured logging with different log levels:
- **Info**: General information about the plugin's execution
- **Debug**: Detailed debugging information (enabled with `--debug` flag)
- **Warning**: Warnings about potential issues
- **Error**: Error messages

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

- [github.com/urfave/cli/v2](https://github.com/urfave/cli/v2) - CLI framework
- [github.com/sirupsen/logrus](https://github.com/sirupsen/logrus) - Structured logging
- [github.com/joho/godotenv](https://github.com/joho/godotenv) - Environment variable loading

### Running Tests

```bash
go test ./...
```

### Building with Version Info

```bash
go build -ldflags "-X main.Version=v1.0.0" -o devops-plugin
```

## Environment File Support

The plugin supports loading environment variables from a `.env` file. You can specify the path to the env file using the `PLUGIN_ENV_FILE` environment variable:

```bash
export PLUGIN_ENV_FILE=.env
./devops-plugin
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
