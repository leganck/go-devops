package devops

import (
	"context"
	"fmt"
	"net/url"
)

// ServerRequest 包含查询可用服务器的参数
type ServerRequest struct {
	EnvName          string `json:"envName"`          // 环境名称（例如 "dev2"、"test"）
	ProgramAliasName string `json:"programAliasName"` // 程序别名标识符
}

// Server 表示来自 API 的部署服务器
type Server struct {
	ServerAlias string `json:"serverAlias"` // 人类可读的服务器别名
	ServerID    string `json:"serverId"`    // 唯一的服务器标识符
}

// GetServers 查询可用于部署程序的服务器列表
// 服务器以组的形式返回，每组表示一个逻辑分组。
// 需要对指定环境具有 "deployProgram:page" 权限。
func (d *DevOps) GetServers(ctx context.Context, req *ServerRequest) ([][]Server, error) {
	if !d.hasPermission("deployProgram:page", req.EnvName) {
		return nil, fmt.Errorf("permission denied: missing deployProgram:page permission for environment %s", req.EnvName)
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
