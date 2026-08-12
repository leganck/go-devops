package devops

import (
	"context"
	"fmt"
	"net/url"
)

// ServerRequest 包含查询可用服务器的参数。
//
// 该结构体指定要查询的环境和程序，返回的结果包含该程序在该环境中可部署的所有服务器。
type ServerRequest struct {
	EnvName          string `json:"envName"`          // 环境名称（例如 "dev2"、"test"）
	ProgramAliasName string `json:"programAliasName"` // 程序别名标识符
}

// Server 表示来自 API 的部署服务器。
//
// 每个服务器有一个人类可读的别名和一个唯一的 ID。
// 服务器 ID 用于部署操作中指定目标服务器。
type Server struct {
	ServerAlias string `json:"serverAlias"` // 人类可读的服务器别名
	ServerID    string `json:"serverId"`    // 唯一的服务器标识符
}

// ServerGroup 表示一组部署服务器，可附带从日志项目对齐得到的组名。
type ServerGroup struct {
	GroupName   string   `json:"groupName,omitempty"`
	ProjectName string   `json:"projectName,omitempty"` // env-group-alias
	Servers     []Server `json:"servers"`
}

// GetServers 查询可用于部署程序的服务器列表。
//
// 该方法返回指定程序在指定环境中可部署的所有服务器。
// 服务器以组的形式返回，每组表示一个逻辑分组。
//
// 权限要求：
//
//	需要对指定环境具有 "deployProgram:page" 权限。
//
// 参数：
//   - ctx: 用于控制请求生命周期的上下文
//   - req: 包含环境名称和程序别名的查询参数
//
// 返回：
//   - [][]Server: 服务器组的二维数组，每个内层数组代表一组服务器
//   - error: 权限不足或 API 请求失败时返回错误
func (d *DevOps) GetServers(ctx context.Context, req *ServerRequest) ([][]Server, error) {
	if !d.hasPermission("deployProgram:page", req.EnvName) {
		return nil, fmt.Errorf("权限被拒绝：缺少环境 %s 的 deployProgram:page 权限", req.EnvName)
	}

	params := url.Values{
		"envName":          {req.EnvName},
		"programAliasName": {req.ProgramAliasName},
	}

	var servers [][]Server
	if err := d.PostRequest(ctx, "/deployProgram/server", params, &servers); err != nil {
		return nil, err
	}

	return servers, nil
}

// GetServersWithGroups 查询服务器并尝试对齐 groupName/projectName。
//
// /deployProgram/server 不返回组名；通过 /programlog/shortCutPage 的
// env-group-alias 列表按序对齐（数量一致时才填充，否则留空）。
func (d *DevOps) GetServersWithGroups(ctx context.Context, req *ServerRequest) ([]ServerGroup, error) {
	servers, err := d.GetServers(ctx, req)
	if err != nil {
		return nil, err
	}

	projects, err := d.ListLogProjects(ctx, req.EnvName)
	if err != nil {
		// 发现失败时仍返回服务器列表，只是没有组名
		return AlignServerGroups(servers, nil), nil
	}
	matches := filterLogProjectsByAlias(projects, req.EnvName, req.ProgramAliasName)
	return AlignServerGroups(servers, matches), nil
}

// AlignServerGroups 将 [][]Server 与已按 group 排序的 projectName 列表对齐。
// 仅当两侧长度一致时填充 GroupName/ProjectName；否则不伪造组名。
func AlignServerGroups(serverGroups [][]Server, projectMatches []string) []ServerGroup {
	result := make([]ServerGroup, 0, len(serverGroups))
	canAlign := len(projectMatches) > 0 && len(projectMatches) == len(serverGroups)
	for i, servers := range serverGroups {
		g := ServerGroup{Servers: servers}
		if canAlign {
			g.ProjectName = projectMatches[i]
			if _, group, _, err := ParseProjectName(projectMatches[i]); err == nil {
				g.GroupName = group
			}
		}
		result = append(result, g)
	}
	return result
}
