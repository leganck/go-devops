package main

import (
	"context"
	stdErrors "errors"
	"fmt"
	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"
	"go-devops/internal/version"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/yassinebenaid/godump"
)

const (
	// maxSSHRetry 是 SSH 部署失败的最大重试次数
	maxSSHRetry = 2
	// sshRetryDelay 是 SSH 部署重试之间的延迟
	sshRetryDelay = 5 * time.Second
)

// Plugin 保存 DevOps 部署自动化的配置
// 它包含与 DevOps API 交互所需的所有参数
type Plugin struct {
	BaseURL        string         // DevOps API 的基础 URL
	Username       string         // 用于身份验证的用户名
	Password       string         // 用于身份验证的密码
	ProgramAlias   string         // 程序别名
	ProgramType    string         // 程序类型（例如 snapshots、releases）
	Env            string         // 环境名称（例如 dev2、test）
	ProjectVersion string         // 用户输入的版本
	ActualVersion  string         // 找到的实际版本（用于模糊匹配）
	Server         string         // 要部署到的服务器别名
	NotifyUser     string         // 部署时要通知的用户
	Debug          bool           // 启用调试模式
	Wait           bool           // 等待部署完成
	client         *devops.DevOps // 内部 DevOps 客户端（不对外暴露）
}

// Exec 执行自动化部署的插件逻辑
// 它执行以下步骤：
// 1. 验证参数
// 2. 初始化 DevOps 客户端
// 3. 与 API 进行身份验证
// 4. 检查程序是否存在
// 5. 查询并匹配版本
// 6. 获取服务器信息
// 7. 使用重试逻辑执行部署
func (p *Plugin) Exec(ctx context.Context) error {
	// 验证必需的参数
	if err := p.validateParams(); err != nil {
		return p.notifyAndReturnError("参数验证失败", err)
	}

	// 初始化 DevOps 客户端
	if err := p.initClient(); err != nil {
		return p.notifyAndReturnError("初始化客户端失败", err)
	}

	// 登录到 DevOps API
	if err := p.login(ctx); err != nil {
		return p.notifyAndReturnError("登录失败", err)
	}

	// 检查程序是否存在
	if err := p.checkProgramExists(ctx); err != nil {
		return p.notifyAndReturnError("检查程序存在性失败", err)
	}

	// 查询版本并获取路径
	versionPath, err := p.queryVersion(ctx)
	if err != nil {
		return p.notifyAndReturnError("查询版本失败", err)
	}

	// 获取服务器 ID
	serverID, err := p.getServers(ctx)
	if err != nil {
		return p.notifyAndReturnError("获取服务器ID失败", err)
	}

	// 使用重试逻辑执行部署
	if err := p.executeDeploymentWithRetry(ctx, versionPath, serverID); err != nil {
		return p.notifyAndReturnError("执行部署失败", err)
	}

	// 成功通知
	p.sendSuccessNotification()
	return nil
}

// validateParams 验证是否提供了所有必需的参数
func (p *Plugin) validateParams() error {
	if p.Username == "" {
		return errors.NewValidationError("username is required", nil)
	}
	if p.Password == "" {
		return errors.NewValidationError("password is required", nil)
	}
	if p.ProgramAlias == "" {
		return errors.NewValidationError("program-alias is required", nil)
	}
	if p.Env == "" {
		return errors.NewValidationError("env is required", nil)
	}
	if p.ProjectVersion == "" {
		return errors.NewValidationError("project-version is required", nil)
	}
	return nil
}

// initClient 使用身份验证凭据初始化 DevOps 客户端
func (p *Plugin) initClient() error {
	client, err := devops.NewDevOps(&devops.Auth{
		Username: p.Username,
		Password: p.Password,
	}, p.BaseURL, p.Debug)
	if err != nil {
		return errors.NewAPIError("create DevOps client", err)
	}

	p.dumpConfigIfDebug()
	p.client = client
	return nil
}

// dumpConfigIfDebug 在调试模式下记录配置，敏感数据被屏蔽
func (p *Plugin) dumpConfigIfDebug() {
	if !p.Debug {
		return
	}

	logger.Debugf("Plugin Configuration:")
	display := struct {
		BaseURL      string
		Username     string
		Password     string
		ProgramAlias string
		ProgramType  string
		Env          string
		Debug        bool
	}{
		BaseURL:      p.BaseURL,
		Username:     p.Username,
		Password:     MaskToken(p.Password),
		ProgramAlias: p.ProgramAlias,
		ProgramType:  p.ProgramType,
		Env:          p.Env,
		Debug:        p.Debug,
	}

	if err := godump.Dump(display); err != nil {
		logger.Warningf("Failed to dump config: %v", err)
	}
}

// login 与 DevOps API 进行身份验证
func (p *Plugin) login(ctx context.Context) error {
	logger.Infof("Logging in to DevOps...")
	if err := p.client.Login(ctx); err != nil {
		return errors.NewLoginError("login failed", err)
	}
	logger.Infof("Login successful")
	return nil
}

// checkProgramExists 验证指定的程序在环境中是否存在
func (p *Plugin) checkProgramExists(ctx context.Context) error {
	logger.Infof("Checking if program %s exists in environment %s...", p.ProgramAlias, p.Env)

	programs, err := p.client.GetProgramAliases(ctx, &devops.ProgramAliasRequest{
		EnvName: p.Env,
	})
	if err != nil {
		return errors.NewAPIError("get program aliases failed", err)
	}

	if !contains(programs, p.ProgramAlias) {
		return errors.NewValidationError(
			fmt.Sprintf("program %s does not exist in environment %s", p.ProgramAlias, p.Env),
			nil,
		)
	}

	logger.Infof("Program %s exists in environment %s", p.ProgramAlias, p.Env)
	return nil
}

// getServers 获取程序的服务器并返回指定的服务器 ID
func (p *Plugin) getServers(ctx context.Context) (string, error) {
	logger.Infof("Getting servers for program %s in environment %s...", p.ProgramAlias, p.Env)

	servers, err := p.client.GetServers(ctx, &devops.ServerRequest{
		EnvName:          p.Env,
		ProgramAliasName: p.ProgramAlias,
	})
	if err != nil {
		return "", errors.NewAPIError("get servers failed", err)
	}

	return p.findServerID(servers)
}

// findServerID 从服务器组中查找服务器 ID
func (p *Plugin) findServerID(serverGroups [][]devops.Server) (string, error) {
	// 将所有服务器展平为单个列表
	var allServers []devops.Server
	for _, group := range serverGroups {
		allServers = append(allServers, group...)
	}

	logger.Infof("Found %d server(s)", len(allServers))
	p.displayServers(serverGroups)

	// 如果只有一个服务器，则使用它
	if len(allServers) == 1 {
		serverID := allServers[0].ServerID
		logger.Infof("Note: Only one server found, using server %s (ID: %s)", allServers[0].ServerAlias, serverID)
		return serverID, nil
	}

	// 查找指定的服务器
	for _, server := range allServers {
		if server.ServerAlias == p.Server {
			logger.Infof("Found server ID: %s for server %s", server.ServerID, server.ServerAlias)
			return server.ServerID, nil
		}
	}

	return "", errors.NewServerError(fmt.Sprintf("server ID not found for server %s", p.Server), nil)
}

// displayServers 以分组格式记录服务器信息
func (p *Plugin) displayServers(serverGroups [][]devops.Server) {
	for groupIndex, group := range serverGroups {
		logger.Infof("  Group %d:", groupIndex+1)
		for _, server := range group {
			logger.Infof("    - %s (ID: %s)", server.ServerAlias, server.ServerID)
		}
	}
}

// queryVersion 查询可用版本并返回匹配的版本路径
func (p *Plugin) queryVersion(ctx context.Context) (string, error) {
	logger.Infof("Querying versions for %s (%s/%s)...", p.ProgramAlias, p.ProgramType, p.Env)

	versions, err := p.client.GetVersion(ctx, &devops.VersionRequest{
		ProgramAliasName: p.ProgramAlias,
		ProgramType:      p.ProgramType,
		EnvName:          p.Env,
	})
	if err != nil {
		return "", errors.NewAPIError("get version failed", err)
	}

	logger.Infof("Found %d version(s)", len(versions))
	return p.findMatchingVersion(versions)
}

// findMatchingVersion 查找匹配的版本（精确或模糊）并返回其路径
func (p *Plugin) findMatchingVersion(versions []devops.VersionItem) (string, error) {
	logger.Infof("Checking if version %s exists...", p.ProjectVersion)

	// 先尝试精确匹配
	for _, v := range versions {
		if v.Version == p.ProjectVersion {
			p.ActualVersion = v.Version
			logger.Infof("Version %s found", p.ProjectVersion)
			logger.Infof("File path: %s", v.RelativePath)
			return v.RelativePath, nil
		}
	}

	// 尝试模糊匹配（例如 4.32.0 <-> 4.32）
	for _, v := range versions {
		if version.FuzzyMatchVersion(p.ProjectVersion, v.Version) {
			p.ActualVersion = v.Version
			logger.Infof("Note: Fuzzy matched version %s with %s", v.Version, p.ProjectVersion)
			logger.Infof("File path: %s", v.RelativePath)
			return v.RelativePath, nil
		}
	}

	return "", errors.NewVersionError(
		fmt.Sprintf("version %s does not exist for program %s in environment %s",
			p.ProjectVersion, p.ProgramAlias, p.Env),
		nil,
	)
}

// executeDeploymentWithRetry 使用 SSH 失败重试逻辑执行部署
func (p *Plugin) executeDeploymentWithRetry(ctx context.Context, versionPath, serverID string) error {
	for attempt := 0; attempt <= maxSSHRetry; attempt++ {
		if attempt > 0 {
			p.sendRetryNotification(attempt)
		}

		taskUUID, err := p.deploy(ctx, versionPath, serverID)
		if err != nil {
			return err
		}

		if !p.Wait {
			return nil
		}

		waitErr := p.client.WaitForDeployCompletion(ctx, taskUUID, 10*time.Second, 10*time.Minute)
		if waitErr == nil {
			return nil
		}

		if stdErrors.Is(waitErr, devops.ErrSSHDeploymentFailed) && attempt < maxSSHRetry {
			logger.Infof("SSH deployment failed, retrying... (%d/%d)", attempt+1, maxSSHRetry)
			p.sendSSHRetryNotification(attempt + 1)
			time.Sleep(sshRetryDelay)
			continue
		}

		return waitErr
	}

	return nil
}

// deploy 启动部署并返回任务 UUID 用于跟踪
func (p *Plugin) deploy(ctx context.Context, versionPath, serverID string) (string, error) {
	logger.Infof("Preparing deployment...")

	if versionPath == "" {
		return "", errors.NewValidationError("version path not found for deployment", nil)
	}

	deployReq := &devops.DeployRequest{
		ProgramAliasName: p.ProgramAlias,
		ProgramType:      p.ProgramType,
		RelativePath:     versionPath,
		NotifyUser:       p.NotifyUser,
		NotifyMemo:       p.buildNotifyMemo(),
		Servers:          serverID,
		EnvName:          p.Env,
		ProgramVersion:   p.ActualVersion,
	}

	logger.Infof("Deploying version %s to server %s...", p.ActualVersion, p.Server)
	if err := p.client.Deploy(ctx, deployReq); err != nil {
		return "", errors.NewDeploymentError("deploy failed", err)
	}

	logger.Infof("Deployment initiated successfully")
	return p.findTaskUUID(ctx, serverID)
}

// buildNotifyMemo 如果指定了通知用户，则构造通知备注
func (p *Plugin) buildNotifyMemo() string {
	if p.NotifyUser == "" {
		return ""
	}
	return fmt.Sprintf("Deploy %s version %s to server %s", p.ProgramAlias, p.ActualVersion, p.Server)
}

// findTaskUUID 查询部署历史以查找未完成的任务 UUID
func (p *Plugin) findTaskUUID(ctx context.Context, serverID string) (string, error) {
	logger.Infof("Querying deploy history to find non-completed tasks...")

	historyResult, err := p.client.GetDeployHistory(ctx, &devops.DeployHistoryRequest{
		Page:         1,
		Limit:        10,
		EnvName:      p.Env,
		Condition:    p.ProgramAlias,
		DeployStatus: "",
	})
	if err != nil {
		logger.Warningf("Failed to get deploy history: %v, continuing deployment", err)
		return "", nil
	}

	for _, item := range historyResult.Data {
		if item.ServerId == serverID && item.DeployStatus != devops.DeployStatusSuccess {
			logger.Infof("Found non-completed task: TaskUUID=%s, Status=%d, ServerID=%s",
				item.TaskUuid, item.DeployStatus, item.ServerId)
			return item.TaskUuid, nil
		}
	}

	logger.Infof("No non-completed tasks found for server %s", serverID)
	return "", nil
}

// 通知方法

func (p *Plugin) notifyAndReturnError(reason string, err error) error {
	p.sendNotification("部署失败", fmt.Sprintf("%s：%s", reason, p.buildFailureDesc()))
	return err
}

func (p *Plugin) buildFailureDesc() string {
	return fmt.Sprintf("程序 %s 版本 %s 部署到服务器 %s（环境 %s）",
		p.ProgramAlias, p.ProjectVersion, p.Server, p.Env)
}

func (p *Plugin) sendSuccessNotification() {
	p.sendNotification(
		"部署完成",
		fmt.Sprintf("程序 %s 版本 %s 已成功部署到服务器 %s（环境 %s）",
			p.ProgramAlias, p.ActualVersion, p.Server, p.Env),
	)
}

func (p *Plugin) sendRetryNotification(attempt int) {
	p.sendNotification(
		"重新部署开始",
		fmt.Sprintf("开始重新部署程序 %s 版本 %s 到服务器 %s（环境 %s）...",
			p.ProgramAlias, p.ActualVersion, p.Server, p.Env),
	)
}

func (p *Plugin) sendSSHRetryNotification(attempt int) {
	p.sendNotification(
		"部署重试",
		fmt.Sprintf("程序 %s 版本 %s 部署到服务器 %s（环境 %s）SSH失败，正在进行第 %d 次重试...",
			p.ProgramAlias, p.ActualVersion, p.Server, p.Env, attempt),
	)
}

// sendNotification 发送桌面通知
func (p *Plugin) sendNotification(title, body string) {
	beeep.AppName = "Go DevOps 插件"
	if err := beeep.Notify(title, body, ""); err != nil {
		logger.Warningf("Failed to send notification: %v", err)
	}
}

// MaskToken 屏蔽敏感令牌用于日志记录
func MaskToken(token string) string {
	if token == "" {
		return ""
	}
	return "***MASKED***"
}

// contains 检查字符串是否存在于切片中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
