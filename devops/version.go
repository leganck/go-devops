package devops

import (
	"context"
	"fmt"
	"net/url"
)

// VersionItem 表示来自 API 的部署版本条目。
//
// 每个版本条目包含版本标识符、文件路径、修改时间和文件大小等信息。
type VersionItem struct {
	RelativePath string `json:"relativePath"` // 版本文件的相对路径
	Version      string `json:"version"`      // 版本标识符（例如 "1.0.0"）
	ModifyTime   string `json:"modifyTime"`   // 最后修改时间戳
	Size         string `json:"size"`         // 文件大小
}

// VersionRequest 包含查询程序版本的参数。
//
// 该结构体指定要查询的程序、程序类型和环境。
// 程序类型可以是 "snapshots"（快照版本）或 "releases"（发布版本）。
type VersionRequest struct {
	ProgramAliasName string `json:"programAliasName"` // 程序别名标识符
	ProgramType      string `json:"programType"`      // 程序类型（例如 "snapshots"、"releases"）
	EnvName          string `json:"envName"`          // 环境名称（例如 "dev2"、"test"）
}

// GetVersion 查询程序的可用版本列表。
//
// 该方法返回指定程序在指定环境中的所有可用版本。
//
// 权限要求：
//   需要对指定环境具有 "deployProgram:page" 权限。
//
// 参数：
//   - ctx: 用于控制请求生命周期的上下文
//   - req: 包含程序别名、程序类型和环境名称的查询参数
//
// 返回：
//   - []VersionItem: 版本信息切片，包含版本号、文件路径、大小和修改时间
//   - error: 权限不足或 API 请求失败时返回错误
func (d *DevOps) GetVersion(ctx context.Context, req *VersionRequest) ([]VersionItem, error) {
	if !d.hasPermission("deployProgram:page", req.EnvName) {
		return nil, fmt.Errorf("权限被拒绝：缺少环境 %s 的 deployProgram:page 权限", req.EnvName)
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
