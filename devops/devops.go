package devops

import (
	"context"
	"crypto/rsa"
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
	Auth        *Auth
	BaseURL     string
	Client      *http.Client
	Debug       bool
	pubKey      *rsa.PublicKey // cached
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
	// Create a cache key combining permission and environment
	// If not cached, compute the result
	result := d.computePermission(permission, env)
	return result
}

// computePermission performs the actual permission computation
func (d *DevOps) computePermission(permission, env string) bool {
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

// PostRequest performs a POST request with common error handling
func (d *DevOps) PostRequest(ctx context.Context, path string, params url.Values, result interface{}) error {
	resp, err := d.postForm(ctx, path, params)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

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

	// Optional: Debug dump full response
	if d.Debug {
		logger.Debugf("=== Debug Mode: %s Response ===", path)
		logger.Debugf("Response: %s", string(body))
		logger.Debug("==============================")
	}

	return nil
}

// GetRequest performs a GET request with common error handling
func (d *DevOps) GetRequest(ctx context.Context, path string, result interface{}) error {
	resp, err := d.get(ctx, path)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

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

	// Optional: Debug dump full response
	if d.Debug {
		logger.Debugf("=== Debug Mode: %s Response ===", path)
		logger.Debugf("Response: %s", string(body))
		logger.Debug("==============================")
	}

	return nil
}
