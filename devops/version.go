package devops

import (
	"context"
	"fmt"
	"net/url"
)

// VersionItem 表示来自 API 的部署版本条目
type VersionItem struct {
	RelativePath string `json:"relativePath"` // 版本文件的相对路径
	Version      string `json:"version"`      // 版本标识符（例如 "1.0.0"）
	ModifyTime   string `json:"modifyTime"`   // 最后修改时间戳
	Size         string `json:"size"`         // 文件大小
}

// VersionRequest 包含查询程序版本的参数
type VersionRequest struct {
	ProgramAliasName string `json:"programAliasName"` // 程序别名标识符
	ProgramType      string `json:"programType"`      // 程序类型（例如 "snapshots"、"releases"）
	EnvName          string `json:"envName"`          // 环境名称（例如 "dev2"、"test"）
}

// GetVersion 查询程序的可用版本列表
// 需要对指定环境具有 "deployProgram:page" 权限。
// 返回包含版本信息的 VersionItem 切片。
func (d *DevOps) GetVersion(ctx context.Context, req *VersionRequest) ([]VersionItem, error) {
	if !d.hasPermission("deployProgram:page", req.EnvName) {
		return nil, fmt.Errorf("permission denied: missing deployProgram:page permission for environment %s", req.EnvName)
	}

	params := url.Values{
		"programAliasName": {req.ProgramAliasName},
		"programType":      {req.ProgramType},
		"envName":          {req.EnvName},
	}

	var versions []VersionItem
	if err := d.PostRequest(ctx, "/deployProgram/version", params, &versions); err != nil {
		return nil, err
	}

	return versions, nil
}
