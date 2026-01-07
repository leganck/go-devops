package devops

import (
	"context"
	"fmt"
	"net/url"
)

// ServerRequest for server list query
type ServerRequest struct {
	EnvName          string `json:"envName"`
	ProgramAliasName string `json:"programAliasName"`
}

// Server represents a server item
type Server struct {
	ServerAlias string `json:"serverAlias"`
	ServerID    string `json:"serverId"`
}

// GetServers queries server list for a program
func (d *DevOps) GetServers(ctx context.Context, req *ServerRequest) ([][]Server, error) {
	// Check permission
	if !d.hasPermission("deployProgram:page", req.EnvName) {
		return nil, fmt.Errorf("permission denied: missing deployProgram:page permission for environment %s", req.EnvName)
	}

	params := url.Values{
		"envName":          {req.EnvName},
		"programAliasName": {req.ProgramAliasName},
	}

	var servers [][]Server
	err := d.PostRequest(ctx, "/deployProgram/server", params, &servers)
	if err != nil {
		return nil, err
	}

	return servers, nil
}
