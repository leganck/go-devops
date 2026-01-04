package devops

import (
	"context"
	"encoding/json"
	"fmt"
	"go-devops/internal/logger"
	"io"
	"net/url"
	"strconv"
)

// DeployHistoryRequest represents a request to get deploy history
// It includes pagination and filtering parameters

type DeployHistoryRequest struct {
	Page         int    // Page number
	Limit        int    // Number of items per page
	EnvName      string // Environment name filter
	DeployStatus string // Deployment status filter
	Condition    string // Search condition
}

// DeployHistoryItem represents a single deploy history entry

type DeployHistoryItem struct {
	ID               string `json:"id"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
	ServerId         string `json:"serverId"`
	UserId           string `json:"userId"`
	DeployStatus     int    `json:"deployStatus"`
	DeployDesc       string `json:"deployDesc"`
	DeployTime       int64  `json:"deployTime"`
	TaskUuid         string `json:"taskUuid"`
	Notify           string `json:"notify"`
	RelativePath     string `json:"relativePath"`
	ProgramType      string `json:"programType"`
	ProgramVersion   string `json:"programVersion"`
	EnvName          string `json:"envName"`
	GroupName        string `json:"groupName"`
	ProgramAliasName string `json:"programAliasName"`
	ServerAlias      string `json:"serverAlias"`
	Nickname         string `json:"nickname"`
}

// DeployHistoryResponse represents the response from deploy history API

type DeployHistoryResponse struct {
	Code     string              `json:"code"`
	Msg      string              `json:"msg"`
	Count    int                 `json:"count"`
	Data     []DeployHistoryItem `json:"data"`
	TotalRow interface{}         `json:"totalRow"`
}

// DeployHistoryResult represents the complete result from GetDeployHistory method

type DeployHistoryResult struct {
	Count int                 `json:"count"`
	Data  []DeployHistoryItem `json:"data"`
}

// GetDeployHistory retrieves the deployment history with optional filters
// It returns the deploy history items and the total count

func (d *DevOps) GetDeployHistory(ctx context.Context, req *DeployHistoryRequest) (*DeployHistoryResult, error) {
	// Check permission
	// Note: We're using a default environment check since envName might be empty in the request
	// If envName is provided, we'll use that for permission check
	checkEnv := req.EnvName
	if checkEnv == "" {
		checkEnv = "*" // Use wildcard if no environment is specified
	}

	if !d.hasPermission("deployHistory:list", checkEnv) {
		return nil, fmt.Errorf("permission denied: missing deployHistory:list permission")
	}

	// Build query parameters
	params := url.Values{}

	// Add pagination parameters
	params.Add("page", strconv.Itoa(req.Page))
	params.Add("limit", strconv.Itoa(req.Limit))

	// Add optional filters
	if req.EnvName != "" {
		params.Add("envName", req.EnvName)
	}
	if req.DeployStatus != "" {
		params.Add("deployStatus", req.DeployStatus)
	}
	if req.Condition != "" {
		params.Add("condition", req.Condition)
	}

	// Make the GET request
	resp, err := d.get(ctx, "/deployHistory/list?"+params.Encode())
	if err != nil {
		return nil, fmt.Errorf("GET /deployHistory/list: %w", err)
	}
	defer resp.Body.Close()

	// Read and parse the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read deploy history response: %w", err)
	}

	// Parse the API response envelope
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse deploy history JSON: %w (raw: %.200s)", err, string(body))
	}

	if !apiResp.IsSuccess() {
		return nil, fmt.Errorf("deploy history error: code=%d, msg=%q", apiResp.Code, apiResp.Msg)
	}

	// Parse the inner data structure
	var historyResp DeployHistoryResponse
	if err := json.Unmarshal(apiResp.Data, &historyResp); err != nil {
		return nil, fmt.Errorf("parse deploy history data: %w (raw: %.200s)", err, string(apiResp.Data))
	}

	// Optional: Debug dump full response
	if d.Debug {
		logger.Debug("=== Debug Mode: Deploy History Response ===")
		logger.Debugf("Deploy history response: %s", string(body))
		logger.Debug("=============================================")
	}

	// Return the result
	return &DeployHistoryResult{
		Count: historyResp.Count,
		Data:  historyResp.Data,
	}, nil
}
