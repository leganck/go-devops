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

// Auth 保存身份验证凭据
type Auth struct {
	Username string
	Password string
}

// Authority 表示来自 API 的权限项
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

// DevOps 表示与 DevOps API 交互的客户端
type DevOps struct {
	Auth        *Auth
	BaseURL     string
	Client      *http.Client
	Debug       bool
	pubKey      interface{}      // 缓存的公钥
	Authorities *map[string]Authority
}

// NewDevOps 使用给定的凭据和配置创建新的 DevOps 客户端
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

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("parse JSON response: %w (raw: %.200s)", err, string(body))
	}

	if !apiResp.IsSuccess() {
		return fmt.Errorf("API error: code=%d, msg=%q", apiResp.Code, apiResp.Msg)
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

// PostRequest 使用表单编码数据执行 POST 请求并处理响应
func (d *DevOps) PostRequest(ctx context.Context, path string, params url.Values, result interface{}) error {
	resp, err := d.postForm(ctx, path, params)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	return d.handleResponse(resp, path, result)
}

// GetRequest 执行 GET 请求并处理响应
func (d *DevOps) GetRequest(ctx context.Context, path string, result interface{}) error {
	resp, err := d.get(ctx, path)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	return d.handleResponse(resp, path, result)
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
