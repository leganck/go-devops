package devops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) buildURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.baseURL + path
}

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	u := c.buildURL(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, classifyTransport(err)
	}
	c.log.Debug("GET", "url", u)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, classifyTransport(err)
	}
	return resp, nil
}

func (c *Client) postForm(ctx context.Context, path string, data url.Values) (*http.Response, error) {
	u := c.buildURL(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, classifyTransport(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.log.Debug("POST", "url", u, "form", maskForm(data).Encode())
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, classifyTransport(err)
	}
	return resp, nil
}

func (c *Client) handleResponse(resp *http.Response, path string, result any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return serverErr("read response body", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return classifyStatus(resp.StatusCode, path, "")
	}

	trimmed := strings.TrimSpace(string(body))
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "<html") && (strings.Contains(lower, "login") || strings.Contains(lower, "/auth/")) {
		return unauthorized("login page returned for " + path)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return classifyStatus(resp.StatusCode, path, "")
	}
	if resp.StatusCode >= 500 {
		return classifyStatus(resp.StatusCode, path, "")
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		raw := string(body)
		if len(raw) > 200 {
			raw = raw[:200]
		}
		return serverErr("parse JSON response: "+raw, err)
	}

	if !apiResp.IsSuccess() {
		msg := fmt.Sprintf("API error: code=%d, msg=%q", apiResp.Code, sanitizeErrorText(apiResp.Msg))
		if looksLikeAuthFailure(apiResp.Msg) {
			return unauthorized(msg)
		}
		return serverErr(msg, nil)
	}

	if result != nil {
		if err := apiResp.WithData(result); err != nil {
			return serverErr("parse result data", err)
		}
	}
	return nil
}

func (c *Client) withSessionRetry(ctx context.Context, fn func() error) error {
	err := fn()
	if err == nil || !IsSessionExpiredError(err) {
		return err
	}
	c.log.Debug("session expired, relogin and retry")
	if reloginErr := c.Relogin(ctx); reloginErr != nil {
		return wrap(ErrUnauthorized, "relogin failed", reloginErr)
	}
	return fn()
}

// DoPost posts form-encoded data and decodes the API envelope into result.
func (c *Client) DoPost(ctx context.Context, path string, params url.Values, result any) error {
	return c.withSessionRetry(ctx, func() error {
		resp, err := c.postForm(ctx, path, params)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return c.handleResponse(resp, path, result)
	})
}

// DoGet performs GET and decodes the API envelope into result.
func (c *Client) DoGet(ctx context.Context, path string, result any) error {
	return c.withSessionRetry(ctx, func() error {
		resp, err := c.get(ctx, path)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return c.handleResponse(resp, path, result)
	})
}

// GetHTML fetches a HTML page with session retry (used by logs project discovery).
func (c *Client) GetHTML(ctx context.Context, path string) (string, error) {
	var htmlBody string
	err := c.withSessionRetry(ctx, func() error {
		resp, err := c.get(ctx, path)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return serverErr("read response", err)
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return classifyStatus(resp.StatusCode, path, "")
		}
		if resp.StatusCode != http.StatusOK {
			return classifyStatus(resp.StatusCode, path, "")
		}
		trimmed := strings.TrimSpace(string(body))
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "<html") && (strings.Contains(lower, "login") || strings.Contains(lower, "/auth/form")) &&
			!strings.Contains(lower, "data-project-name") {
			return unauthorized("login page returned for " + path)
		}
		htmlBody = string(body)
		return nil
	})
	return htmlBody, err
}

// RequirePermission returns ErrForbidden when the session lacks permission in env.
func (c *Client) RequirePermission(permission, env string) error {
	return c.requirePermission(permission, env)
}

// SetAuthoritiesForTest injects cached permissions (tests only).
func (c *Client) SetAuthoritiesForTest(a map[string]Authority) {
	c.setAuthorities(a)
}
