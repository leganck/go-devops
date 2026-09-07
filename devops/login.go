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
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const rsaPublicKeyPattern = `let rsaPlublic="([\s\S]*?)\n?"`

type userMenu struct {
	MenuID   StringOrNumber `json:"menuId"`
	MenuSort any            `json:"menuSort"`
}

type loginData struct {
	UserMenu    map[StringOrNumber]userMenu `json:"usermenu"`
	RedirectURL string                      `json:"redirctUrl"`
	Authorities map[string]Authority        `json:"authorities"`
}

// Login authenticates with RSA-encrypted password. Does not persist the session.
func (c *Client) Login(ctx context.Context) error {
	if c.creds.Username == "" || c.creds.Password == "" {
		return invalidArg("username and password are required")
	}

	pubKeyStr, err := c.fetchLoginPage(ctx)
	if err != nil {
		return err
	}
	if c.pubKey == nil {
		pubKey, err := parseRSAPublicKey(pubKeyStr)
		if err != nil {
			return serverErr("parse public key", err)
		}
		c.pubKey = pubKey
	}

	encPass, err := rsaEncrypt(c.creds.Password, c.pubKey)
	if err != nil {
		return serverErr("encrypt password", err)
	}

	params := url.Values{
		"username": {c.creds.Username},
		"password": {encPass},
	}
	resp, err := c.postForm(ctx, "/auth/form", params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return serverErr("read login response", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		raw := string(body)
		if len(raw) > 200 {
			raw = raw[:200]
		}
		return serverErr("parse login JSON: "+raw, err)
	}
	if !apiResp.IsSuccess() {
		msg := sanitizeErrorText(apiResp.Msg)
		return unauthorized(fmt.Sprintf("login failed: code=%d, msg=%q", apiResp.Code, msg))
	}

	var data loginData
	if err := apiResp.WithData(&data); err != nil {
		return serverErr("parse login data", err)
	}
	c.setAuthorities(data.Authorities)
	c.log.Debug("login ok", "menus", len(data.UserMenu), "authorities", len(data.Authorities))
	return nil
}

func (c *Client) fetchLoginPage(ctx context.Context) (string, error) {
	resp, err := c.get(ctx, "/public/login")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", classifyStatus(resp.StatusCode, "/public/login", "")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", serverErr("read login page", err)
	}
	pubKeyStr, err := extractRSAPublicKey(string(body))
	if err != nil {
		return "", serverErr("extract public key", err)
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
	re := regexp.MustCompile(rsaPublicKeyPattern)
	matches := re.FindStringSubmatch(script)
	if len(matches) < 2 {
		return "", fmt.Errorf("rsaPlublic value not matched")
	}
	s := matches[1]
	s = strings.ReplaceAll(s, "\\/", "/")
	s = strings.ReplaceAll(s, "\\n", "\n")
	return strings.TrimSuffix(s, "\n"), nil
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
		return nil, fmt.Errorf("not RSA")
	}
	return pk, nil
}

func rsaEncrypt(plain string, pub *rsa.PublicKey) (string, error) {
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(plain))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}
