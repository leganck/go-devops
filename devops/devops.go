package devops

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"go-devops/internal/logger"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// APIResponse is the common response envelope (like JenkinsResponse)
type APIResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// Auth holds username and password for login
type Auth struct {
	Username string
	Password string
}

// DevOps represents a DevOps client (Jenkins-style)
type DevOps struct {
	Auth    *Auth
	BaseURL string
	Client  *http.Client
	Debug   bool
	pubKey  *rsa.PublicKey // cached

	Authorities *map[string]Authority
}

// NewDevOps creates a new DevOps client
func NewDevOps(auth *Auth, baseURL string, debug bool) (*DevOps, error) {
	baseURL = strings.TrimRight(baseURL, "/")

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	client := &http.Client{
		Jar:       jar,
		Timeout:   60 * time.Second,
		Transport: &http.Transport{},
	}

	return &DevOps{
		Auth:    auth,
		BaseURL: baseURL,
		Client:  client,
		Debug:   debug,
	}, nil
}

// hasPermission checks if user has specified permission for given environment
func (d *DevOps) hasPermission(permission, env string) bool {
	if d.Authorities == nil {
		return false
	}

	for _, auth := range *d.Authorities {
		if auth.Permission != permission {
			continue
		}
		// Allow all environments if Envs is empty
		if len(auth.Envs) == 0 {
			return true
		}
		// Check if env is allowed
		for _, allowedEnv := range auth.Envs {
			if allowedEnv == env {
				return true
			}
		}
	}

	return false
}

// buildURL constructs full URL (like Jenkins)
func (d *DevOps) buildURL(path string) string {
	return d.BaseURL + path
}

// sendRequest centralizes HTTP execution
func (d *DevOps) sendRequest(req *http.Request) (*http.Response, error) {
	return d.Client.Do(req)
}

// get performs a GET request with independent timeout control
func (d *DevOps) get(ctx context.Context, path string) (*http.Response, error) {
	u := d.buildURL(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if d.Debug {
		logger.Debugf("[DEBUG] GET %s", u)
	}
	return d.sendRequest(req)
}

// postForm performs a POST with url.Values and independent timeout control
func (d *DevOps) postForm(ctx context.Context, path string, data url.Values) (*http.Response, error) {
	u := d.buildURL(path)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if d.Debug {
		masked := make(url.Values)
		for k, v := range data {
			if k == "password" {
				masked[k] = []string{"***MASKED***"}
			} else {
				masked[k] = v
			}
		}
		logger.Debugf("[DEBUG] POST %s ← %s", u, masked.Encode())
	}
	return d.sendRequest(req)
}
