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

// VersionItem represents a deploy version entry
type VersionItem struct {
	RelativePath string `json:"relativePath"`
	Version      string `json:"version"`
	ModifyTime   string `json:"modifyTime"`
	Size         string `json:"size"`
}

// VersionRequest for version query
type VersionRequest struct {
	ProgramAliasName string `json:"programAliasName"`
	ProgramType      string `json:"programType"`
	EnvName          string `json:"envName"`
}

// GetVersion queries version list
func (d *DevOps) GetVersion(ctx context.Context, req *VersionRequest) ([]VersionItem, error) {
	// Check permission
	if !d.hasPermission("deployProgram:page", req.EnvName) {
		return nil, fmt.Errorf("permission denied: missing deployProgram:page permission for environment %s", req.EnvName)
	}

	params := url.Values{
		"programAliasName": {req.ProgramAliasName},
		"programType":      {req.ProgramType},
		"envName":          {req.EnvName},
	}

	resp, err := d.postForm(ctx, "/deployProgram/version", params)
	if err != nil {
		return nil, fmt.Errorf("POST /deployProgram/version: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read version response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse version JSON: %w (raw: %.200s)", err, string(body))
	}

	if !apiResp.IsSuccess() {
		return nil, fmt.Errorf("version query error: code=%d, msg=%q", apiResp.Code, apiResp.Msg)
	}

	var versions []VersionItem
	if err := apiResp.WithData(&versions); err != nil {
		return nil, fmt.Errorf("parse version  %w", err)
	}

	// Optional: Debug dump full response
	if d.Debug && len(versions) > 0 {
		log.Println("=== Debug Mode: Version Data ===")
		if err := godump.Dump(versions); err != nil {
			log.Printf("warning: failed to dump versions: %v", err)
		}
		log.Println("==============================")
	}

	return versions, nil
}
