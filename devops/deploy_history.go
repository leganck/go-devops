package devops

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// DeployHistoryRequest 包含查询部署历史的参数
type DeployHistoryRequest struct {
	Page         int    // 页码（从 1 开始）
	Limit        int    // 每页的项目数量
	EnvName      string // 环境名称过滤器（例如 "dev2"、"test"）
	DeployStatus string // 部署状态过滤器（为空表示所有状态）
	Condition    string // 搜索条件（例如程序别名）
}

// DeployHistoryItem 表示单个部署历史条目
type DeployHistoryItem struct {
	ID               string `json:"id"`               // 唯一标识符
	CreatedAt        int64  `json:"createdAt"`        // 创建时间戳（Unix）
	UpdatedAt        int64  `json:"updatedAt"`        // 最后更新时间戳（Unix）
	ServerId         string `json:"serverId"`         // 服务器标识符
	UserId           string `json:"userId"`           // 发起部署的用户
	DeployStatus     int    `json:"deployStatus"`     // 部署状态码
	DeployDesc       string `json:"deployDesc"`       // 状态描述
	DeployTime       int64  `json:"deployTime"`       // 部署时间戳（Unix）
	TaskUuid         string `json:"taskUuid"`         // 任务 UUID，用于跟踪
	Notify           string `json:"notify"`           // 通知状态
	RelativePath     string `json:"relativePath"`     // 部署文件的路径
	ProgramType      string `json:"programType"`      // 程序类型
	ProgramVersion   string `json:"programVersion"`   // 部署的版本
	EnvName          string `json:"envName"`          // 环境名称
	GroupName        string `json:"groupName"`        // 服务器组名称
	ProgramAliasName string `json:"programAliasName"` // 程序别名
	ServerAlias      string `json:"serverAlias"`      // 服务器别名
	Nickname         string `json:"nickname"`         // 用户昵称
}

// DeployHistoryResponse 是来自部署历史 API 的原始响应
type DeployHistoryResponse struct {
	Code     string              `json:"code"`     // 响应码
	Msg      string              `json:"msg"`      // 响应消息
	Count    int                 `json:"count"`    // 当前页的项目数量
	Data     []DeployHistoryItem `json:"data"`     // 部署历史项目
	TotalRow interface{}         `json:"totalRow"` // 总行数（根据 API 变化）
}

// DeployHistoryResult 是 GetDeployHistory 返回的简化结果
type DeployHistoryResult struct {
	Count int                 // 返回的项目数量
	Data  []DeployHistoryItem // 部署历史项目
}

// GetDeployHistory 使用可选过滤器检索部署历史
// 需要对指定环境具有 "deployHistory:list" 权限。
// 结果支持分页，并可按状态和程序过滤。
func (d *DevOps) GetDeployHistory(ctx context.Context, req *DeployHistoryRequest) (*DeployHistoryResult, error) {
	if !d.hasPermission("deployHistory:list", req.EnvName) {
		return nil, fmt.Errorf("权限被拒绝：缺少环境 %s 的 deployHistory:list 权限", req.EnvName)
	}

	params := d.buildHistoryParams(req)
	path := "/deployHistory/list?" + params.Encode()

	var historyResp DeployHistoryResponse
	if err := d.GetRequest(ctx, path, &historyResp); err != nil {
		return nil, err
	}

	return &DeployHistoryResult{
		Count: historyResp.Count,
		Data:  historyResp.Data,
	}, nil
}

// buildHistoryParams 构建历史请求的 url.Values
func (d *DevOps) buildHistoryParams(req *DeployHistoryRequest) url.Values {
	params := url.Values{}

	// 添加分页参数
	params.Add("page", strconv.Itoa(req.Page))
	params.Add("limit", strconv.Itoa(req.Limit))

	// 添加可选过滤器
	if req.EnvName != "" {
		params.Add("envName", req.EnvName)
	}
	if req.DeployStatus != "" {
		params.Add("deployStatus", req.DeployStatus)
	}
	if req.Condition != "" {
		params.Add("condition", req.Condition)
	}

	return params
}
