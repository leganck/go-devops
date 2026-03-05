package devops

import (
	"context"
	"fmt"
	"net/url"
)

// DeployRequest 包含启动部署的参数。
//
// 该结构体包含部署所需的所有信息，包括目标程序、版本、服务器和通知设置。
type DeployRequest struct {
	ProgramAliasName string `json:"programAliasName"` // 程序别名标识符
	ProgramType      string `json:"programType"`      // 程序类型（例如 "snapshots"、"releases"）
	RelativePath     string `json:"relativePath"`     // 版本文件的路径
	RegularTime      string `json:"regularTime"`      // 定时部署时间（为空表示立即部署）
	NotifyUser       string `json:"notifyUser"`       // 要通知的用户的逗号分隔列表
	NotifyMemo       string `json:"notifyMemo"`       // 通知消息
	Servers          string `json:"servers"`          // 要部署到的服务器 ID（逗号分隔）
	EnvName          string `json:"envName"`          // 目标环境名称
	ProgramVersion   string `json:"programVersion"`   // 版本标识符
}

// Deploy 启动程序版本到指定服务器的部署。
//
// 该方法向 DevOps API 发送部署请求，启动异步部署任务。
// 如果指定了 NotifyUser，部署完成后会通知相关用户。
//
// 权限要求：
//   需要对指定环境具有 "deployProgram:page" 权限。
//
// 参数：
//   - ctx: 用于控制请求生命周期的上下文
//   - req: 包含部署目标、版本和通知设置的部署请求
//
// 返回：
//   - error: 权限不足或 API 请求失败时返回错误
//
// 注意：
//   该方法仅启动部署任务，不等待部署完成。
//   使用 WaitForDeployCompletion 方法来监控部署进度。
func (d *DevOps) Deploy(ctx context.Context, req *DeployRequest) error {
	if !d.hasPermission("deployProgram:page", req.EnvName) {
		return fmt.Errorf("权限被拒绝：缺少环境 %s 的 deployProgram:page 权限", req.EnvName)
	}

	params := d.buildDeployParams(req)

	return d.PostRequest(ctx, "/deployProgram/start", params, nil)
}

// buildDeployParams 构建部署请求的 url.Values
func (d *DevOps) buildDeployParams(req *DeployRequest) url.Values {
	params := url.Values{
		"programAliasName": {req.ProgramAliasName},
		"programType":      {req.ProgramType},
		"relativePath":     {req.RelativePath},
		"regularTime":      {req.RegularTime},
		"servers":          {req.Servers},
		"envName":          {req.EnvName},
		"programVersion":   {req.ProgramVersion},
	}

	// 仅在指定了 NotifyUser 时添加通知字段
	if req.NotifyUser != "" {
		params.Add("notifyUser", req.NotifyUser)
		params.Add("notifyChecked", "1")
		params.Add("notifyMemo", req.NotifyMemo)
	}

	return params
}
