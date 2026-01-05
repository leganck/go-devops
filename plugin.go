package main

import (
	"context"
	"fmt"
	"go-devops/devops"
	"go-devops/internal/errors"
	"go-devops/internal/logger"
	"go-devops/internal/version"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/yassinebenaid/godump"
)

// Plugin holds the configuration for the DevOps client
// It contains all the parameters needed to interact with the DevOps API
type Plugin struct {
	BaseURL        string         // Base URL of the DevOps API
	Username       string         // Username for authentication
	Password       string         // Password for authentication
	ProgramAlias   string         // Program alias name
	ProgramType    string         // Program type (e.g., snapshots, releases)
	Env            string         // Environment name (e.g., dev2, test)
	ProjectVersion string         // User input version
	ActualVersion  string         // Actual version found (for fuzzy matching)
	Server         string         // Server alias to deploy to
	NotifyUser     string         // Users to notify on deployment
	Debug          bool           // Enable debug mode
	Wait           bool           // Wait for deployment to complete
	client         *devops.DevOps // Internal DevOps client (not exposed)
}

// validateParams validates required parameters
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

// initClient initializes the DevOps client
func (p *Plugin) initClient() error {
	client, err := devops.NewDevOps(&devops.Auth{
		Username: p.Username,
		Password: p.Password,
	}, p.BaseURL, p.Debug)
	if err != nil {
		return errors.NewAPIError("create DevOps client", err)
	}

	// Debug: dump config (mask sensitive fields)
	if p.Debug {
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

	p.client = client
	return nil
}

// login authenticates with DevOps
func (p *Plugin) login(ctx context.Context) error {
	logger.Infof("Logging in to DevOps...")
	if err := p.client.Login(ctx); err != nil {
		return errors.NewLoginError("login failed", err)
	}
	logger.Infof("Login successful")
	return nil
}

// checkProgramExists checks if the specified program exists
func (p *Plugin) checkProgramExists(ctx context.Context) error {
	logger.Infof("Checking if program %s exists in environment %s...", p.ProgramAlias, p.Env)
	programs, err := p.client.GetProgramAliases(ctx, &devops.ProgramAliasRequest{
		EnvName: p.Env,
	})
	if err != nil {
		return errors.NewAPIError("get program aliases failed", err)
	}

	// Check if program exists
	programExists := false
	for _, program := range programs {
		if program == p.ProgramAlias {
			programExists = true
			break
		}
	}

	if !programExists {
		return errors.NewValidationError(fmt.Sprintf("program %s does not exist in environment %s", p.ProgramAlias, p.Env), nil)
	}
	logger.Infof("Program %s exists in environment %s", p.ProgramAlias, p.Env)
	return nil
}

// getServers retrieves and displays servers for the program, then finds the specified server ID
func (p *Plugin) getServers(ctx context.Context) (string, error) {
	logger.Infof("Getting servers for program %s in environment %s...", p.ProgramAlias, p.Env)
	servers, err := p.client.GetServers(ctx, &devops.ServerRequest{
		EnvName:          p.Env,
		ProgramAliasName: p.ProgramAlias,
	})
	if err != nil {
		return "", errors.NewAPIError("get servers failed", err)
	}

	// Flatten and display servers
	logger.Infof("Found %d server group(s)", len(servers))
	groupIndex := 1
	// Collect all servers in a flat list for easy checking
	var allServers []devops.Server
	for _, group := range servers {
		logger.Infof("  Group %d:", groupIndex)
		for _, server := range group {
			logger.Infof("    - %s (ID: %s)", server.ServerAlias, server.ServerID)
			allServers = append(allServers, server)
		}
		groupIndex++
	}

	// Find server ID for the specified server alias
	var serverID string

	// If only one server exists, use it regardless of the specified server alias
	if len(allServers) == 1 {
		serverID = allServers[0].ServerID
		logger.Infof("Note: Only one server found, using server %s (ID: %s) regardless of specified server alias", allServers[0].ServerAlias, allServers[0].ServerID)
	} else {
		// Multiple servers exist, find the specified one
		for _, server := range allServers {
			if server.ServerAlias == p.Server {
				serverID = server.ServerID
				break
			}
		}
	}

	if serverID == "" {
		return "", errors.NewServerError(fmt.Sprintf("server ID not found for server %s", p.Server), nil)
	}

	logger.Infof("Found server ID: %s for server %s", serverID, p.Server)
	return serverID, nil
}

// queryVersion queries and checks the specified version
func (p *Plugin) queryVersion(ctx context.Context) (string, error) {
	logger.Infof("Querying versions for %s (%s/%s)...",
		p.ProgramAlias, p.ProgramType, p.Env)

	versions, err := p.client.GetVersion(ctx, &devops.VersionRequest{
		ProgramAliasName: p.ProgramAlias,
		ProgramType:      p.ProgramType,
		EnvName:          p.Env,
	})
	if err != nil {
		return "", errors.NewAPIError("get version failed", err)
	}

	// Output result
	logger.Infof("Found %d version(s)", len(versions))

	logger.Infof("Checking if version %s exists...", p.ProjectVersion)
	var versionPath string
	versionFound := false
	var actualVersion string

	// First try exact match
	for _, v := range versions {
		if v.Version == p.ProjectVersion {
			versionFound = true
			versionPath = v.RelativePath
			actualVersion = v.Version
			break
		}
	}

	// If no exact match, try fuzzy matching (e.g., 4.32.0 <-> 4.32)
	if !versionFound {
		for _, v := range versions {
			if version.FuzzyMatchVersion(p.ProjectVersion, v.Version) {
				versionFound = true
				versionPath = v.RelativePath
				actualVersion = v.Version
				logger.Infof("Note: Fuzzy matched version %s with %s", v.Version, p.ProjectVersion)
				break
			}
		}
	}

	if !versionFound {
		return "", errors.NewVersionError(fmt.Sprintf("version %s does not exist for program %s in environment %s", p.ProjectVersion, p.ProgramAlias, p.Env), nil)
	}

	// Save actual version for later use
	p.ProjectVersion = actualVersion

	logger.Infof("Version %s found (actual: %s)", p.ProjectVersion, actualVersion)
	logger.Infof("File path: %s", versionPath)
	return versionPath, nil
}

// deploy deploys the program to the specified server and returns the task UUID
func (p *Plugin) deploy(ctx context.Context, versionPath string, serverID string) (string, error) {
	logger.Infof("Preparing deployment...")

	// Validate deployment parameters
	if versionPath == "" {
		return "", errors.NewValidationError("version path not found for deployment", nil)
	}

	// Prepare deploy request
	// Only generate notify memo if notifyUser is specified
	var notifyMemo string
	if p.NotifyUser != "" {
		notifyMemo = fmt.Sprintf("Deploy %s version %s to server %s", p.ProgramAlias, p.ProjectVersion, p.Server)
	}

	// Use actual version if available, otherwise use user input version
	programVersion := p.ProjectVersion

	deployReq := &devops.DeployRequest{
		ProgramAliasName: p.ProgramAlias,
		ProgramType:      p.ProgramType,
		RelativePath:     versionPath,
		NotifyUser:       p.NotifyUser,
		NotifyMemo:       notifyMemo,
		Servers:          serverID,
		EnvName:          p.Env,
		ProgramVersion:   programVersion,
	}

	// Execute deployment
	logger.Infof("Deploying version %s to server %s...", p.ProjectVersion, p.Server)
	if err := p.client.Deploy(ctx, deployReq); err != nil {
		return "", errors.NewDeploymentError("deploy failed", err)
	}

	logger.Infof("Deployment completed successfully")

	// After deployment, query deploy history to find first non-completed task
	logger.Infof("Querying deploy history to find non-completed tasks...")

	// Prepare deploy history request
	historyReq := &devops.DeployHistoryRequest{
		Page:         1,
		Limit:        10,
		EnvName:      p.Env,
		Condition:    p.ProgramAlias,
		DeployStatus: "", // No status filter - we'll check status in code
	}

	// Call GetDeployHistory API
	historyResult, err := p.client.GetDeployHistory(ctx, historyReq)
	if err != nil {
		logger.Warningf("Failed to get deploy history: %v", err)
		return "", nil // Continue even if history query fails
	}

	// Find first non-completed task for the specified serverID
	var targetTaskUUID string
	for _, item := range historyResult.Data {
		// Match the serverID
		if item.ServerId == serverID {
			// Check if status is not completed (4)
			if item.DeployStatus != 4 {
				targetTaskUUID = item.TaskUuid
				logger.Infof("Found non-completed task: TaskUUID=%s, Status=%d, ServerID=%s",
					targetTaskUUID, item.DeployStatus, item.ServerId)
				break // Found the first one, exit loop
			}
		}
	}

	if targetTaskUUID != "" {
		logger.Infof("First non-completed task UUID: %s", targetTaskUUID)
	} else {
		logger.Infof("No non-completed tasks found for server %s", serverID)
	}

	return targetTaskUUID, nil
}

// sendNotification sends a desktop notification with Chinese content
func (p *Plugin) sendNotification(title, body string) {
	// Set application name for notification
	beeep.AppName = "Go DevOps 插件"

	// Send desktop notification
	if err := beeep.Notify(title, body, ""); err != nil {
		logger.Warningf("Failed to send notification: %v", err)
	}
}

// waitForDeployment waits for a deployment task to complete using taskUUID
func (p *Plugin) waitForDeployment(ctx context.Context, taskUUID string) error {
	if taskUUID == "" {
		logger.Infof("No task UUID provided, skipping wait")
		return nil
	}

	// Wait for the task to complete
	logger.Infof("Waiting for task %s to complete...", taskUUID)
	// Let WaitForDeployCompletion handle timeout using the provided context
	err := p.client.WaitForDeployCompletion(ctx, taskUUID, 10*time.Second, 3*time.Minute)
	if err != nil {
		// Send failure notification
		notificationTitle := "部署等待失败"
		notificationBody := fmt.Sprintf("程序 %s 版本 %s 部署到服务器 %s（环境 %s）等待超时或失败\n错误信息：%v",
			p.ProgramAlias, p.ProjectVersion, p.Server, p.Env, err)
		p.sendNotification(notificationTitle, notificationBody)
		return errors.NewDeploymentError("failed to wait for task completion", err)
	}

	return nil
}

// Exec executes the plugin logic
// It follows a specific workflow:
// 1. Validate required parameters
// 2. Initialize DevOps client
// 3. Login to DevOps API
// 4. Check if program exists in the specified environment
// 5. Query available versions and find the matching version
// 6. Get server information and find the server ID
// 7. Deploy the program if all required parameters are provided
func (p *Plugin) Exec(ctx context.Context) error {
	// Validate required parameters
	if err := p.validateParams(); err != nil {
		return err
	}

	// Initialize DevOps client with authentication and configuration
	if err := p.initClient(); err != nil {
		return err
	}

	// Login to DevOps API to obtain authentication cookies
	if err := p.login(ctx); err != nil {
		return err
	}

	// Check if the specified program exists in the given environment
	if err := p.checkProgramExists(ctx); err != nil {
		return err
	}

	// Query available versions and find the matching version path
	versionPath, err := p.queryVersion(ctx)
	if err != nil {
		return err
	}

	// Get server ID from the server alias (this will fail if server doesn't exist)
	serverID, err := p.getServers(ctx)
	if err != nil {
		return err
	}

	// Deploy the program
	taskUUID, err := p.deploy(ctx, versionPath, serverID)
	if err != nil {
		return err
	}
	// Wait for deployment to complete if Wait flag is true
	if p.Wait {
		if err := p.waitForDeployment(ctx, taskUUID); err != nil {
			return err
		}
	}

	// Send success notification after deployment (or after waiting if enabled)
	notificationTitle := "部署完成"
	notificationBody := fmt.Sprintf("程序 %s 版本 %s 已成功部署到服务器 %s（环境 %s）",
		p.ProgramAlias, p.ProjectVersion, p.Server, p.Env)
	p.sendNotification(notificationTitle, notificationBody)

	return nil
}

func MaskToken(token string) string {
	if token == "" {
		return ""
	}
	return "***MASKED***"
}
