package devops

import (
	"context"
	"encoding/json"
	"fmt"
	"go-devops/internal/logger"
	"io"
	"net/url"

	"github.com/yassinebenaid/godump"
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

	resp, err := d.postForm(ctx, "/deployProgram/programAliasName", params)
	if err != nil {
		return nil, fmt.Errorf("POST /deployProgram/programAliasName: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read program alias response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse program alias JSON: %w (raw: %.200s)", err, string(body))
	}

	if !apiResp.IsSuccess() {
		return nil, fmt.Errorf("program alias query error: code=%d, msg=%q", apiResp.Code, apiResp.Msg)
	}

	var programAliases []string
	if err := apiResp.WithData(&programAliases); err != nil {
		return nil, fmt.Errorf("parse program alias data: %w", err)
	}

	// Optional: Debug dump full response
	if d.Debug {
		logger.Debug("=== Debug Mode: Program Alias Data ===")
		if err := godump.Dump(programAliases); err != nil {
			logger.Warningf("failed to dump program aliases: %v", err)
		}
		logger.Debug("====================================")
	}

	return programAliases, nil
}
