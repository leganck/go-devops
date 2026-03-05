package devops

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"go-devops/internal/logger"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/yassinebenaid/godump"
)

const (
	// rsaPublicKeyPattern 是从 HTML 中提取 RSA 公钥的正则表达式模式
	rsaPublicKeyPattern = `let rsaPlublic="([\s\S]*?)\n?"`
)

// UserMenu 表示登录响应中的菜单项
type UserMenu struct {
	MenuID   StringOrNumber `json:"menuId"`
	MenuSort interface{}    `json:"menuSort"`
}

// LoginData 包含成功登录后返回的数据
type LoginData struct {
	UserMenu    map[StringOrNumber]UserMenu `json:"usermenu"`
	RedirectURL string                      `json:"redirctUrl"` // 保留 API 拼写错误
	Authorities map[string]Authority        `json:"authorities"`
}

// Login 使用 RSA 加密与 DevOps API 进行身份验证
// 它从登录页面获取 RSA 公钥，加密密码，并发送登录请求。权限被缓存用于权限检查。
func (d *DevOps) Login(ctx context.Context) error {
	if d.Auth == nil {
		return fmt.Errorf("认证未配置")
	}
	if d.Auth.Username == "" {
		return fmt.Errorf("用户名为必填项")
	}
	if d.Auth.Password == "" {
		return fmt.Errorf("密码为必填项")
	}

	pubKeyStr, err := d.fetchLoginPage(ctx)
	if err != nil {
		return fmt.Errorf("获取登录页失败: %w", err)
	}

	if d.pubKey == nil {
		pubKey, err := parseRSAPublicKey(pubKeyStr)
		if err != nil {
			return fmt.Errorf("解析公钥失败: %w", err)
		}
		d.pubKey = pubKey
	}

	encPass, err := rsaEncrypt(d.Auth.Password, d.pubKey.(*rsa.PublicKey))
	if err != nil {
		return fmt.Errorf("加密密码失败: %w", err)
	}

	params := url.Values{
		"username": {d.Auth.Username},
		"password": {encPass},
	}

	resp, err := d.postForm(ctx, "/auth/form", params)
	if err != nil {
		return fmt.Errorf("POST /auth/form 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取登录响应失败: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("解析登录 JSON 失败: %w (原始: %.200s)", err, string(body))
	}

	if !apiResp.IsSuccess() {
		return fmt.Errorf("登录失败: code=%d, msg=%q", apiResp.Code, apiResp.Msg)
	}

	var loginData LoginData
	if err := apiResp.WithData(&loginData); err != nil {
		return fmt.Errorf("解析登录数据失败: %w", err)
	}

	d.Authorities = &loginData.Authorities

	d.logLoginSuccess(&loginData)

	return nil
}

// logLoginSuccess 记录成功登录的调试信息
func (d *DevOps) logLoginSuccess(data *LoginData) {
	if !d.Debug {
		return
	}

	logger.Debugf("[DEBUG] 登录成功: 重定向=%q, 菜单=%d, 权限=%d",
		data.RedirectURL,
		len(data.UserMenu),
		len(data.Authorities))

	logger.Debug("=== 调试模式: 登录数据 ===")
	if err := godump.Dump(data); err != nil {
		logger.Warningf("转储登录数据失败: %v", err)
	}
	logger.Debug("==============================")
}

// fetchLoginPage 获取登录页面 HTML 并提取 RSA 公钥
func (d *DevOps) fetchLoginPage(ctx context.Context) (string, error) {
	resp, err := d.get(ctx, "/public/login")
	if err != nil {
		return "", fmt.Errorf("GET /public/login 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET /public/login: 意外的状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取登录页失败: %w", err)
	}

	pubKeyStr, err := extractRSAPublicKey(string(body))
	if err != nil {
		return "", fmt.Errorf("提取公钥失败: %w", err)
	}

	return pubKeyStr, nil
}

// extractRSAPublicKey 从 HTML 登录页面提取 RSA 公钥
func extractRSAPublicKey(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("解析 HTML 失败: %w", err)
	}

	var script string
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		t := s.Text()
		if strings.Contains(t, "rsaPlublic=") && s.AttrOr("src", "") == "" {
			script = t
		}
	})

	if script == "" {
		return "", fmt.Errorf("在登录页面中未找到 rsaPlublic 脚本")
	}

	re := regexp.MustCompile(rsaPublicKeyPattern)
	matches := re.FindStringSubmatch(script)
	if len(matches) < 2 {
		return "", fmt.Errorf("rsaPlublic 值未被正则表达式匹配")
	}

	return processEscapedPublicKey(matches[1]), nil
}

// processEscapedPublicKey 处理转义的公钥字符串
func processEscapedPublicKey(s string) string {
	s = strings.ReplaceAll(s, "\\/", "/")
	s = strings.ReplaceAll(s, "\\n", "\n")
	return strings.TrimSuffix(s, "\n")
}

// parseRSAPublicKey 解析 PEM 编码的 RSA 公钥
func parseRSAPublicKey(s string) (*rsa.PublicKey, error) {
	pemBlock := fmt.Sprintf("-----BEGIN PUBLIC KEY-----\n%s\n-----END PUBLIC KEY-----", s)
	block, _ := pem.Decode([]byte(pemBlock))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("无效的 PEM 块：期望为 PUBLIC KEY")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析 PKIX 公钥失败: %w", err)
	}

	pk, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("公钥不是 RSA 类型")
	}

	return pk, nil
}

// rsaEncrypt 使用 RSA PKCS#1 v1.5 加密明文
func rsaEncrypt(plain string, pub *rsa.PublicKey) (string, error) {
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(plain))
	if err != nil {
		return "", fmt.Errorf("RSA 加密失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}
