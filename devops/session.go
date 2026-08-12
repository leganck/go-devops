package devops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	sessionTTL         = 8 * time.Hour
	sessionDirName     = ".go-devops"
	sessionSubDirName  = "sessions"
)

// persistedCookie 是可序列化的 Cookie 子集。
type persistedCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"httpOnly"`
}

// SessionFile 是落盘的会话内容（不含密码）。
type SessionFile struct {
	BaseURL     string                `json:"baseURL"`
	Username    string                `json:"username"`
	SavedAt     time.Time             `json:"savedAt"`
	ExpiresAt   time.Time             `json:"expiresAt"`
	Cookies     []persistedCookie     `json:"cookies"`
	Authorities map[string]Authority  `json:"authorities"`
}

// SessionDir 返回会话目录路径。
func SessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, sessionDirName, sessionSubDirName), nil
}

// SessionFilePath 按 baseURL+username 生成会话文件路径。
func SessionFilePath(baseURL, username string) (string, error) {
	dir, err := SessionDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.TrimRight(baseURL, "/") + "|" + username))
	name := hex.EncodeToString(sum[:8]) + ".json"
	return filepath.Join(dir, name), nil
}

// LoadSessionFile 读取会话文件；过期或不匹配则返回 nil。
func LoadSessionFile(baseURL, username string) (*SessionFile, error) {
	path, err := SessionFilePath(baseURL, username)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sess SessionFile
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	if !strings.EqualFold(sess.Username, username) {
		return nil, nil
	}
	if strings.TrimRight(sess.BaseURL, "/") != strings.TrimRight(baseURL, "/") {
		return nil, nil
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, nil
	}
	if len(sess.Cookies) == 0 {
		return nil, nil
	}
	return &sess, nil
}

// SaveSessionFile 写入会话文件。
func SaveSessionFile(sess *SessionFile) error {
	if sess == nil {
		return fmt.Errorf("session is nil")
	}
	path, err := SessionFilePath(sess.BaseURL, sess.Username)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ClearSessionFile 删除指定用户的会话文件。
func ClearSessionFile(baseURL, username string) error {
	path, err := SessionFilePath(baseURL, username)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func exportCookies(jar http.CookieJar, rawURL string) ([]persistedCookie, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	cookies := jar.Cookies(u)
	out := make([]persistedCookie, 0, len(cookies))
	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}
		pc := persistedCookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
		}
		if c.Domain != "" {
			pc.Domain = c.Domain
		} else {
			pc.Domain = u.Hostname()
		}
		if pc.Path == "" {
			pc.Path = "/"
		}
		if !c.Expires.IsZero() {
			pc.Expires = c.Expires
		}
		out = append(out, pc)
	}
	return out, nil
}

func applyCookies(jar http.CookieJar, rawURL string, cookies []persistedCookie) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	list := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		hc := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		}
		if hc.Path == "" {
			hc.Path = "/"
		}
		if !c.Expires.IsZero() {
			hc.Expires = c.Expires
		}
		list = append(list, hc)
	}
	jar.SetCookies(u, list)
	return nil
}

func newEmptyCookieJar() (http.CookieJar, error) {
	return cookiejar.New(nil)
}

// IsSessionExpiredError 判断错误是否像会话失效。
func IsSessionExpiredError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	markers := []string{
		"session expired",
		"unauthorized",
		"未登录",
		"请登录",
		"重新登录",
		"登录超时",
		"login page",
		"authentication",
		"sys_user_resource",
		"http 401",
		"http 403",
	}
	for _, m := range markers {
		if strings.Contains(msg, strings.ToLower(m)) {
			return true
		}
	}
	return false
}
