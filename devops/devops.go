// Package devops 提供 DevOps API 的 Go 客户端实现。
//
// 该包封装了与 DevOps API 交互的所有功能，包括：
//   - 用户认证和会话管理
//   - 程序、服务器、版本查询
//   - 程序部署和部署状态监控
//   - 权限检查
//
// 主要类型：
//   - DevOps: API 客户端，包含所有与 API 交互的方法
//   - Auth: 认证凭据（用户名和密码）
//   - Authority: 用户权限信息
//
// 使用示例：
//
//	// 创建客户端
//	client, err := devops.NewDevOps(&devops.Auth{
//	    Username: "user@example.com",
//	    Password: "secret",
//	}, "https://devops.example.com", false)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 登录
//	if err := client.Login(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// 查询环境列表
//	envs := client.GetEnvs()
//
// 权限系统：
// 该包实现了基于权限的访问控制。每个操作都需要特定的权限：
//   - deployProgram:page: 部署和查询权限
//
// 客户端会自动缓存登录后的权限信息，后续操作会检查权限。
package devops

import (
	"context"
	"encoding/json"
	"fmt"
	"go-devops/internal/logger"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// StringOrNumber 是一个自定义类型，可以同时解析 JSON 字符串和数字
type StringOrNumber string

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (s *StringOrNumber) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = StringOrNumber(str)
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(data, &num); err != nil {
		return fmt.Errorf("cannot unmarshal %q as string or number", data)
	}
	*s = StringOrNumber(num.String())
	return nil
}

// APIResponse 是 DevOps API 的通用响应封装
type APIResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// IsSuccess 当响应码表示成功(0)时返回 true
func (r *APIResponse) IsSuccess() bool {
	return r.Code == 0
}

// WithData 将 Data 字段解析到提供的指针中
func (r *APIResponse) WithData(v interface{}) error {
	if len(r.Data) == 0 || string(r.Data) == "null" {
		return nil
	}
	return json.Unmarshal(r.Data, v)
}

// Auth 保存身份验证凭据。
//
// 该结构体包含连接到 DevOps API 所需的用户认证信息。
type Auth struct {
	Username string
	Password string
}

// Authority 表示来自 API 的权限项。
//
// 每个权限项包含一个标识符、关联的环境列表和权限字符串。
// 如果 Envs 为空，则表示该权限适用于所有环境。
type Authority struct {
	ID         StringOrNumber `json:"id"`
	ParentID   StringOrNumber `json:"parentId"`
	MenuName   string         `json:"menuName"`
	Permission string         `json:"permission"`
	MultiEnv   int            `json:"multiEnv"`
	PageHref   string         `json:"pageHref"`
	Envs       []string       `json:"envs"`
	Child      interface{}    `json:"child"`
}

// DevOps 表示与 DevOps API 交互的客户端。
//
// 该结构体封装了 HTTP 客户端、认证信息和会话状态。
// 成功登录后，权限信息会被缓存到 Authorities 字段中。
type DevOps struct {
	Auth        *Auth
	BaseURL     string
	Client      *http.Client
	Debug       bool
	pubKey      interface{} // 缓存的公钥
	Authorities *map[string]Authority
	// FreshLogin 为 true 时忽略本地会话，强制重新登录
	FreshLogin bool
}

// NewDevOps 使用给定的凭据和配置创建新的 DevOps 客户端。
//
// 该函数创建并初始化一个 DevOps 客户端实例，包括：
//   - 验证必需的参数（auth、username、password、baseURL）
//   - 创建 HTTP 客户端，配置连接池和超时
//   - 初始化 Cookie 管理器以维护会话状态
//
// 参数：
//   - auth: 认证凭据，包含用户名和密码
//   - baseURL: DevOps API 的基础 URL（会自动去除末尾的斜杠）
//   - debug: 是否启用调试模式（启用后会打印详细日志）
//
// 返回：
//   - *DevOps: 初始化后的 DevOps 客户端实例
//   - error: 参数验证失败或创建 HTTP 客户端失败时返回错误
//
// 示例：
//
//	client, err := devops.NewDevOps(&devops.Auth{
//	    Username: "user@example.com",
//	    Password: "secret",
//	}, "https://devops.example.com", false)
func NewDevOps(auth *Auth, baseURL string, debug bool) (*DevOps, error) {
	if auth == nil {
		return nil, fmt.Errorf("auth cannot be nil")
	}
	if auth.Username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if auth.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}

	baseURL = strings.TrimRight(baseURL, "/")
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	client := &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  true,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 10,
		},
	}

	return &DevOps{
		Auth:    auth,
		BaseURL: baseURL,
		Client:  client,
		Debug:   debug,
	}, nil
}

// buildURL 通过将路径追加到基础 URL 来构建完整 URL
func (d *DevOps) buildURL(path string) string {
	return d.BaseURL + path
}

// get 使用上下文支持执行 HTTP GET 请求
func (d *DevOps) get(ctx context.Context, path string) (*http.Response, error) {
	u := d.buildURL(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create GET request: %w", err)
	}
	if d.Debug {
		logger.Debugf("[DEBUG] GET %s", u)
	}
	return d.Client.Do(req)
}

// postForm 使用表单编码数据执行 HTTP POST 请求
func (d *DevOps) postForm(ctx context.Context, path string, data url.Values) (*http.Response, error) {
	u := d.buildURL(path)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if d.Debug {
		masked := maskSensitiveData(data)
		logger.Debugf("[DEBUG] POST %s ← %s", u, masked.Encode())
	}

	return d.Client.Do(req)
}

// maskSensitiveData 创建 url.Values 的副本，其中敏感字段被屏蔽
func maskSensitiveData(data url.Values) url.Values {
	masked := make(url.Values)
	for k, v := range data {
		if isSensitiveField(k) {
			masked[k] = []string{"***MASKED***"}
		} else {
			masked[k] = v
		}
	}
	return masked
}

// isSensitiveField 当字段名包含敏感关键字时返回 true
func isSensitiveField(field string) bool {
	sensitiveFields := []string{"password", "secret", "token", "key"}
	lowerField := strings.ToLower(field)
	for _, sensitive := range sensitiveFields {
		if strings.Contains(lowerField, sensitive) {
			return true
		}
	}
	return false
}

// doRequest 执行 HTTP 请求并处理通用响应处理
func (d *DevOps) doRequest(ctx context.Context, method, path string, body io.Reader, result interface{}) error {
	u := d.buildURL(path)
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return fmt.Errorf("create %s request: %w", method, err)
	}

	if body != nil && method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := d.Client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	return d.handleResponse(resp, path, result)
}

// handleResponse 处理 HTTP 响应，处理错误和解析 JSON
func (d *DevOps) handleResponse(resp *http.Response, path string, result interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("session expired: HTTP %d for %s", resp.StatusCode, path)
	}

	trimmed := strings.TrimSpace(string(body))
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "<html") && (strings.Contains(lower, "login") || strings.Contains(lower, "/auth/")) {
		return fmt.Errorf("session expired: login page returned for %s", path)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("parse JSON response: %w (raw: %.200s)", err, string(body))
	}

	if !apiResp.IsSuccess() {
		errMsg := fmt.Sprintf("API error: code=%d, msg=%q", apiResp.Code, apiResp.Msg)
		if looksLikeAuthFailure(apiResp.Msg) {
			return fmt.Errorf("session expired: %s", errMsg)
		}
		return fmt.Errorf("%s", errMsg)
	}

	if result != nil {
		if err := apiResp.WithData(result); err != nil {
			return fmt.Errorf("parse result data: %w", err)
		}
	}

	if d.Debug {
		logger.Debugf("=== Debug Mode: %s Response ===", path)
		logger.Debugf("Response: %s", string(body))
		logger.Debug("==============================")
	}

	return nil
}

func looksLikeAuthFailure(msg string) bool {
	lower := strings.ToLower(msg)
	for _, m := range []string{"未登录", "请登录", "重新登录", "登录超时", "unauthorized", "authentication", "sys_user_resource"} {
		if strings.Contains(lower, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

// PostRequest 使用表单编码数据执行 POST 请求并处理响应
func (d *DevOps) PostRequest(ctx context.Context, path string, params url.Values, result interface{}) error {
	return d.withSessionRetry(ctx, func() error {
		resp, err := d.postForm(ctx, path, params)
		if err != nil {
			return fmt.Errorf("POST %s: %w", path, err)
		}
		defer resp.Body.Close()
		return d.handleResponse(resp, path, result)
	})
}

// GetRequest 执行 GET 请求并处理响应
func (d *DevOps) GetRequest(ctx context.Context, path string, result interface{}) error {
	return d.withSessionRetry(ctx, func() error {
		resp, err := d.get(ctx, path)
		if err != nil {
			return fmt.Errorf("GET %s: %w", path, err)
		}
		defer resp.Body.Close()
		return d.handleResponse(resp, path, result)
	})
}

func (d *DevOps) withSessionRetry(ctx context.Context, fn func() error) error {
	err := fn()
	if err == nil || !IsSessionExpiredError(err) {
		return err
	}
	if d.Debug {
		logger.Debugf("session expired, relogin and retry: %v", err)
	}
	if reloginErr := d.Relogin(ctx); reloginErr != nil {
		return fmt.Errorf("%w (relogin failed: %v)", err, reloginErr)
	}
	return fn()
}

// hasPermission 检查已认证用户是否具有指定环境的指定权限
func (d *DevOps) hasPermission(permission, env string) bool {
	if d.Authorities == nil {
		return false
	}

	for _, auth := range *d.Authorities {
		if auth.Permission != permission {
			continue
		}
		// 如果 Envs 为空，则允许所有环境
		if len(auth.Envs) == 0 {
			return true
		}
		// 检查 env 是否在允许列表中
		for _, allowedEnv := range auth.Envs {
			if allowedEnv == env {
				return true
			}
		}
	}

	return false
}

// GetEnvs 获取具有 deployProgram:page 权限的所有环境列表
func (d *DevOps) GetEnvs() []string {
	if d.Authorities == nil {
		return []string{}
	}

	var envs []string
	seen := make(map[string]bool)

	for _, auth := range *d.Authorities {
		if auth.Permission != "deployProgram:page" {
			continue
		}

		// 如果 Envs 为空，没有特定环境限制，跳过
		if len(auth.Envs) == 0 {
			continue
		}

		// 添加所有环境，去重
		for _, env := range auth.Envs {
			if !seen[env] {
				seen[env] = true
				envs = append(envs, env)
			}
		}
	}

	return envs
}
