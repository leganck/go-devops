package devops

import (
	"context"
	"fmt"
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
	if !d.hasPermission("deployHistory:list", req.EnvName) {
		return nil, fmt.Errorf("permission denied: missing deployHistory:list permission for environment %s", req.EnvName)
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

	// Make the GET request with query parameters
	path := "/deployHistory/list?" + params.Encode()
	var historyResp DeployHistoryResponse
	err := d.GetRequest(ctx, path, &historyResp)
	if err != nil {
		return nil, err
	}

	// Return the result
	return &DeployHistoryResult{
		Count: historyResp.Count,
		Data:  historyResp.Data,
	}, nil
}
