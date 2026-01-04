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

// StringOrNumber unmarshals both JSON string and number into Go string
type StringOrNumber string

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

// LoginData is the "data" field of login response
type LoginData struct {
	UserMenu    map[StringOrNumber]UserMenu `json:"usermenu"`
	RedirectURL string                      `json:"redirctUrl"` // API typo preserved
	Authorities map[string]Authority        `json:"authorities"`
}

// UserMenu represents menu item in login response
type UserMenu struct {
	MenuID   StringOrNumber `json:"menuId"`
	MenuSort interface{}    `json:"menuSort"`
}

// Authority represents a permission item
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

// Login performs RSA-encrypted login and caches LoginData
func (d *DevOps) Login(ctx context.Context) error {
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

	encPass, err := rsaEncrypt(d.Auth.Password, d.pubKey)
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
		return fmt.Errorf("parse login  %w", err)
	}

	d.Authorities = &loginData.Authorities

	if d.Debug {
		logger.Debugf("[DEBUG] Login success: redirect=%q, menus=%d, authorities=%d",
			loginData.RedirectURL,
			len(loginData.UserMenu),
			len(loginData.Authorities))
		// Optional: Debug dump full response
		logger.Debug("=== Debug Mode: Login Data ===")
		if err := godump.Dump(loginData); err != nil {
			logger.Warningf("failed to dump login data: %v", err)
		}
		logger.Debug("==============================")
	}

	return nil
}

// fetchLoginPage extracts RSA public key from /public/login
func (d *DevOps) fetchLoginPage(ctx context.Context) (string, error) {
	resp, err := d.get(ctx, "/public/login")
	if err != nil {
		return "", fmt.Errorf("GET /public/login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET /public/login: %d", resp.StatusCode)
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

func extractRSAPublicKey(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}

	var script string
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		t := s.Text()
		if strings.Contains(t, "rsaPlublic=") && s.AttrOr("src", "") == "" {
			script = t
		}
	})
	if script == "" {
		return "", fmt.Errorf("rsaPlublic script not found")
	}

	re := regexp.MustCompile(`let rsaPlublic="([\s\S]*?)\n?"`)
	m := re.FindStringSubmatch(script)
	if len(m) < 2 {
		return "", fmt.Errorf("rsaPlublic value not matched")
	}
	return processEscapedPublicKey(m[1]), nil
}

func parseRSAPublicKey(s string) (*rsa.PublicKey, error) {
	pemBlock := fmt.Sprintf("-----BEGIN PUBLIC KEY-----\n%s\n-----END PUBLIC KEY-----", s)
	block, _ := pem.Decode([]byte(pemBlock))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("invalid PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pk, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not RSA key")
	}
	return pk, nil
}

func processEscapedPublicKey(s string) string {
	s = strings.ReplaceAll(s, "\\/", "/")
	s = strings.ReplaceAll(s, "\\n", "\n")
	return strings.TrimSuffix(s, "\n")
}

func rsaEncrypt(plain string, pub *rsa.PublicKey) (string, error) {
	ct, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(plain))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}

// IsSuccess checks if code == 0
func (r *APIResponse) IsSuccess() bool {
	return r.Code == 0
}

// WithData unmarshals Data field into given pointer
func (r *APIResponse) WithData(v interface{}) error {
	if len(r.Data) == 0 || string(r.Data) == "null" {
		return nil
	}
	return json.Unmarshal(r.Data, v)
}
