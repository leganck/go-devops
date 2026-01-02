package devops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"

	"github.com/yassinebenaid/godump"
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

	resp, err := d.postForm(ctx, "/deployProgram/server", params)
	if err != nil {
		return nil, fmt.Errorf("POST /deployProgram/server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read server response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse server JSON: %w (raw: %.200s)", err, string(body))
	}

	if !apiResp.IsSuccess() {
		return nil, fmt.Errorf("server query error: code=%d, msg=%q", apiResp.Code, apiResp.Msg)
	}

	var servers [][]Server
	if err := apiResp.WithData(&servers); err != nil {
		return nil, fmt.Errorf("parse server data: %w", err)
	}

	// Optional: Debug dump full response
	if d.Debug && len(servers) > 0 {
		log.Println("=== Debug Mode: Server Data ===")
		if err := godump.Dump(servers); err != nil {
			log.Printf("warning: failed to dump servers: %v", err)
		}
		log.Println("=============================")
	}

	return servers, nil
}
