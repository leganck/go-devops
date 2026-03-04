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
		return fmt.Errorf("auth not configured")
	}
	if d.Auth.Username == "" {
		return fmt.Errorf("username is required")
	}
	if d.Auth.Password == "" {
		return fmt.Errorf("password is required")
	}

	pubKeyStr, err := d.fetchLoginPage(ctx)
	if err != nil {
		return fmt.Errorf("fetch login page: %w", err)
	}

	if d.pubKey == nil {
		pubKey, err := parseRSAPublicKey(pubKeyStr)
		if err != nil {
			return fmt.Errorf("parse public key: %w", err)
		}
		d.pubKey = pubKey
	}

	encPass, err := rsaEncrypt(d.Auth.Password, d.pubKey.(*rsa.PublicKey))
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}

	params := url.Values{
		"username": {d.Auth.Username},
		"password": {encPass},
	}

	resp, err := d.postForm(ctx, "/auth/form", params)
	if err != nil {
		return fmt.Errorf("POST /auth/form: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read login response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("parse login JSON: %w (raw: %.200s)", err, string(body))
	}

	if !apiResp.IsSuccess() {
		return fmt.Errorf("login failed: code=%d, msg=%q", apiResp.Code, apiResp.Msg)
	}

	var loginData LoginData
	if err := apiResp.WithData(&loginData); err != nil {
		return fmt.Errorf("parse login data: %w", err)
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

	logger.Debugf("[DEBUG] Login success: redirect=%q, menus=%d, authorities=%d",
		data.RedirectURL,
		len(data.UserMenu),
		len(data.Authorities))

	logger.Debug("=== Debug Mode: Login Data ===")
	if err := godump.Dump(data); err != nil {
		logger.Warningf("failed to dump login data: %v", err)
	}
	logger.Debug("==============================")
}

// fetchLoginPage 获取登录页面 HTML 并提取 RSA 公钥
func (d *DevOps) fetchLoginPage(ctx context.Context) (string, error) {
	resp, err := d.get(ctx, "/public/login")
	if err != nil {
		return "", fmt.Errorf("GET /public/login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET /public/login: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read login page: %w", err)
	}

	pubKeyStr, err := extractRSAPublicKey(string(body))
	if err != nil {
		return "", fmt.Errorf("extract public key: %w", err)
	}

	return pubKeyStr, nil
}

// extractRSAPublicKey 从 HTML 登录页面提取 RSA 公钥
func extractRSAPublicKey(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("parse HTML: %w", err)
	}

	var script string
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		t := s.Text()
		if strings.Contains(t, "rsaPlublic=") && s.AttrOr("src", "") == "" {
			script = t
		}
	})

	if script == "" {
		return "", fmt.Errorf("rsaPlublic script not found in login page")
	}

	re := regexp.MustCompile(rsaPublicKeyPattern)
	matches := re.FindStringSubmatch(script)
	if len(matches) < 2 {
		return "", fmt.Errorf("rsaPlublic value not matched by regex")
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
		return nil, fmt.Errorf("invalid PEM block: expected PUBLIC KEY")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKIX public key: %w", err)
	}

	pk, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return pk, nil
}

// rsaEncrypt 使用 RSA PKCS#1 v1.5 加密明文
func rsaEncrypt(plain string, pub *rsa.PublicKey) (string, error) {
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(plain))
	if err != nil {
		return "", fmt.Errorf("RSA encrypt: %w", err)
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}
