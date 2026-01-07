package devops

import (
	"context"
	"fmt"
	"net/url"
)

// DeployRequest for deployment
type DeployRequest struct {
	ProgramAliasName string `json:"programAliasName"`
	ProgramType      string `json:"programType"`
	RelativePath     string `json:"relativePath"`
	RegularTime      string `json:"regularTime"`
	NotifyUser       string `json:"notifyUser"`
	NotifyMemo       string `json:"notifyMemo"`
	Servers          string `json:"servers"`
	EnvName          string `json:"envName"`
	ProgramVersion   string `json:"programVersion"`
}

// DeployResponse represents deployment response
type DeployResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// Deploy deploys a program to specified servers
func (d *DevOps) Deploy(ctx context.Context, req *DeployRequest) error {
	// Check permission
	if !d.hasPermission("deployProgram:page", req.EnvName) {
		return fmt.Errorf("permission denied: missing deployProgram:page permission for environment %s", req.EnvName)
	}

	params := url.Values{
		"programAliasName": {req.ProgramAliasName},
		"programType":      {req.ProgramType},
		"relativePath":     {req.RelativePath},
		"regularTime":      {req.RegularTime},
		"servers":          {req.Servers},
		"envName":          {req.EnvName},
		"programVersion":   {req.ProgramVersion},
	}

	// Only add notify fields if NotifyUser is specified
	if req.NotifyUser != "" {
		params.Add("notifyUser", req.NotifyUser)
		params.Add("notifyChecked", "1")
		params.Add("notifyMemo", req.NotifyMemo)
	}

	err := d.PostRequest(ctx, "/deployProgram/start", params, nil)
	if err != nil {
		return err
	}

	return nil
}
