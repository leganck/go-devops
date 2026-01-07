package devops

import (
	"context"
	"fmt"
	"net/url"
)

// ProgramAliasRequest for program alias query
type ProgramAliasRequest struct {
	EnvName string `json:"envName"`
}

// GetProgramAliases queries program alias list
func (d *DevOps) GetProgramAliases(ctx context.Context, req *ProgramAliasRequest) ([]string, error) {
	// Check permission
	if !d.hasPermission("deployProgram:page", req.EnvName) {
		return nil, fmt.Errorf("permission denied: missing deployProgram:page permission for environment %s", req.EnvName)
	}

	params := url.Values{
		"envName": {req.EnvName},
	}

	var programAliases []string
	err := d.PostRequest(ctx, "/deployProgram/programAliasName", params, &programAliases)
	if err != nil {
		return nil, err
	}

	return programAliases, nil
}
