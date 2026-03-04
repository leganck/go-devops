package devops

import (
	"context"
	"fmt"
	"net/url"
)

// ProgramAliasRequest 包含查询程序别名的参数
type ProgramAliasRequest struct {
	EnvName string `json:"envName"` // 环境名称（例如 "dev2"、"test"）
}

// GetProgramAliases 查询环境的可用程序别名列表
// 需要对指定环境具有 "deployProgram:page" 权限。
// 返回程序别名的名称切片。
func (d *DevOps) GetProgramAliases(ctx context.Context, req *ProgramAliasRequest) ([]string, error) {
	if !d.hasPermission("deployProgram:page", req.EnvName) {
		return nil, fmt.Errorf("permission denied: missing deployProgram:page permission for environment %s", req.EnvName)
	}

	params := url.Values{
		"envName": {req.EnvName},
	}

	var programAliases []string
	if err := d.PostRequest(ctx, "/deployProgram/programAliasName", params, &programAliases); err != nil {
		return nil, err
	}

	return programAliases, nil
}
