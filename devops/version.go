package devops

import (
	"context"
	"fmt"
	"net/url"
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

	var versions []VersionItem
	err := d.PostRequest(ctx, "/deployProgram/version", params, &versions)
	if err != nil {
		return nil, err
	}

	return versions, nil
}
