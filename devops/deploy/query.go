package deploy

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

const (
	StatusInProgress = 1
	StatusPending    = 2
	StatusFailed     = 3
	StatusSuccess    = 4
	StatusCancelled  = 5
)

type ProgramAliasRequest struct {
	EnvName string
}

func (c *Client) Programs(ctx context.Context, env string) ([]string, error) {
	if err := c.core.RequirePermission(permDeploy, env); err != nil {
		return nil, err
	}
	params := url.Values{"envName": {env}}
	var aliases []string
	if err := c.core.DoPost(ctx, "/deployProgram/programAliasName", params, &aliases); err != nil {
		return nil, err
	}
	return aliases, nil
}

type VersionItem struct {
	RelativePath string `json:"relativePath"`
	Version      string `json:"version"`
	ModifyTime   string `json:"modifyTime"`
	Size         string `json:"size"`
}

type VersionRequest struct {
	ProgramAliasName string
	ProgramType      string
	EnvName          string
}

func (c *Client) Versions(ctx context.Context, req VersionRequest) ([]VersionItem, error) {
	if err := c.core.RequirePermission(permDeploy, req.EnvName); err != nil {
		return nil, err
	}
	params := url.Values{
		"programAliasName": {req.ProgramAliasName},
		"programType":      {req.ProgramType},
		"envName":          {req.EnvName},
	}
	var versions []VersionItem
	if err := c.core.DoPost(ctx, "/deployProgram/version", params, &versions); err != nil {
		return nil, err
	}
	return versions, nil
}

type Server struct {
	ServerAlias string `json:"serverAlias"`
	ServerID    string `json:"serverId"`
}

type ServerGroup struct {
	GroupName   string   `json:"groupName,omitempty"`
	ProjectName string   `json:"projectName,omitempty"`
	Servers     []Server `json:"servers"`
}

type ServerRequest struct {
	EnvName          string
	ProgramAliasName string
}

func (c *Client) Servers(ctx context.Context, req ServerRequest) ([][]Server, error) {
	if err := c.core.RequirePermission(permDeploy, req.EnvName); err != nil {
		return nil, err
	}
	params := url.Values{
		"envName":          {req.EnvName},
		"programAliasName": {req.ProgramAliasName},
	}
	var servers [][]Server
	if err := c.core.DoPost(ctx, "/deployProgram/server", params, &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

func AlignServerGroups(serverGroups [][]Server, projectMatches []string) []ServerGroup {
	result := make([]ServerGroup, 0, len(serverGroups))
	canAlign := len(projectMatches) > 0 && len(projectMatches) == len(serverGroups)
	for i, servers := range serverGroups {
		g := ServerGroup{Servers: servers}
		if canAlign {
			g.ProjectName = projectMatches[i]
			parts := splitProject(projectMatches[i])
			if len(parts) >= 3 {
				g.GroupName = parts[1]
			}
		}
		result = append(result, g)
	}
	return result
}

func splitProject(name string) []string {
	return strings.Split(name, "-")
}

type HistoryRequest struct {
	Page         int
	Limit        int
	EnvName      string
	DeployStatus string
	Condition    string
}

type HistoryItem struct {
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

type HistoryResult struct {
	Count int           `json:"count"`
	Data  []HistoryItem `json:"data"`
}

type historyResponse struct {
	Code  string        `json:"code"`
	Msg   string        `json:"msg"`
	Count int           `json:"count"`
	Data  []HistoryItem `json:"data"`
}

func (c *Client) History(ctx context.Context, req HistoryRequest) (*HistoryResult, error) {
	if err := c.core.RequirePermission(permHistory, req.EnvName); err != nil {
		return nil, err
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("limit", strconv.Itoa(limit))
	if req.EnvName != "" {
		params.Set("envName", req.EnvName)
	}
	if req.DeployStatus != "" {
		params.Set("deployStatus", req.DeployStatus)
	}
	if req.Condition != "" {
		params.Set("condition", req.Condition)
	}
	var resp historyResponse
	if err := c.core.DoGet(ctx, "/deployHistory/list?"+params.Encode(), &resp); err != nil {
		return nil, err
	}
	return &HistoryResult{Count: resp.Count, Data: resp.Data}, nil
}
