package devops

import (
	"context"
	"crypto/rsa"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"
)

// Client is the shared HTTP + session kernel. Capability packages
// (deploy/sql/logs) wrap a Client; they do not share a unified default env.
type Client struct {
	baseURL     string
	creds       Credentials
	http        *http.Client
	store       SessionStore
	log         Logger
	clock       Clock
	fresh       bool
	pubKey      *rsa.PublicKey
	mu          sync.RWMutex
	authorities map[string]Authority
}

// New constructs a Client. BaseURL and credentials are required; there is no default host.
func New(opts ...Option) (*Client, error) {
	cfg := options{
		timeout: 60 * time.Second,
		logger:  nopLogger{},
		clock:   systemClock{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if strings.TrimSpace(cfg.baseURL) == "" {
		return nil, invalidArg("base URL is required")
	}
	if strings.TrimSpace(cfg.creds.Username) == "" {
		return nil, invalidArg("username is required")
	}
	if cfg.creds.Password == "" {
		return nil, invalidArg("password is required")
	}

	baseURL := strings.TrimRight(cfg.baseURL, "/")
	store := cfg.store
	if store == nil {
		store = NewFileStore("")
	}
	lg := cfg.logger
	if lg == nil {
		lg = nopLogger{}
	}
	clk := cfg.clock
	if clk == nil {
		clk = systemClock{}
	}

	httpClient := cfg.httpClient
	if httpClient == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, serverErr("create cookie jar", err)
		}
		transport := cfg.transport
		if transport == nil {
			transport = &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  true,
				MaxIdleConnsPerHost: 10,
			}
		}
		httpClient = &http.Client{
			Jar:       jar,
			Timeout:   cfg.timeout,
			Transport: transport,
		}
	} else if httpClient.Jar == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, serverErr("create cookie jar", err)
		}
		httpClient.Jar = jar
	}

	return &Client{
		baseURL: baseURL,
		creds:   cfg.creds,
		http:    httpClient,
		store:   store,
		log:     lg,
		clock:   clk,
		fresh:   cfg.freshLogin,
	}, nil
}

func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) Username() string { return c.creds.Username }

func (c *Client) HTTPClient() *http.Client { return c.http }

func (c *Client) Logger() Logger { return c.log }

func (c *Client) Clock() Clock { return c.clock }

func (c *Client) setAuthorities(a map[string]Authority) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authorities = a
}

func (c *Client) copyAuthorities() map[string]Authority {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.authorities == nil {
		return nil
	}
	out := make(map[string]Authority, len(c.authorities))
	for k, v := range c.authorities {
		out[k] = v
	}
	return out
}

// HasPermission reports whether the cached session has permission in env.
func (c *Client) HasPermission(permission, env string) bool {
	auths := c.copyAuthorities()
	if auths == nil {
		return false
	}
	for _, auth := range auths {
		if auth.Permission != permission {
			continue
		}
		if len(auth.Envs) == 0 {
			return true
		}
		for _, allowed := range auth.Envs {
			if allowed == env {
				return true
			}
		}
	}
	return false
}

// Environments returns distinct envs for a permission whose Envs list is non-empty.
func (c *Client) Environments(permission string) []string {
	auths := c.copyAuthorities()
	if auths == nil {
		return nil
	}
	var envs []string
	seen := map[string]bool{}
	for _, auth := range auths {
		if auth.Permission != permission {
			continue
		}
		for _, env := range auth.Envs {
			if env == "" || seen[env] {
				continue
			}
			seen[env] = true
			envs = append(envs, env)
		}
	}
	return envs
}

func (c *Client) requirePermission(permission, env string) error {
	if env == "" {
		return invalidArg("environment is required")
	}
	if !c.HasPermission(permission, env) {
		return forbidden("missing " + permission + " for environment " + env)
	}
	return nil
}

func (c *Client) saveSession(ctx context.Context) error {
	if c.http == nil || c.http.Jar == nil {
		return invalidArg("client not ready for save session")
	}
	cookies, err := exportCookies(c.http.Jar, c.baseURL)
	if err != nil {
		return err
	}
	now := c.clock.Now()
	sess := &Session{
		BaseURL:     c.baseURL,
		Username:    c.creds.Username,
		SavedAt:     now,
		ExpiresAt:   sessionTTLExpiry(now),
		Cookies:     cookies,
		Authorities: c.copyAuthorities(),
	}
	if sess.Authorities == nil {
		sess.Authorities = map[string]Authority{}
	}
	if err := dumpHasPassword(sess); err != nil {
		return err
	}
	return c.store.Save(ctx, sess)
}

func (c *Client) resetJar() {
	if jar, err := newEmptyCookieJar(); err == nil {
		c.http.Jar = jar
	}
	c.setAuthorities(nil)
}

// EnsureSession restores a persisted session or logs in.
func (c *Client) EnsureSession(ctx context.Context) error {
	if c.fresh {
		if err := c.Login(ctx); err != nil {
			return err
		}
		return c.saveSession(ctx)
	}
	sess, err := c.store.Load(ctx, c.baseURL, c.creds.Username)
	if err != nil {
		c.log.Warn("load session failed", "err", err)
	}
	if sess != nil {
		if applyErr := applyCookies(c.http.Jar, c.baseURL, sess.Cookies); applyErr != nil {
			c.log.Warn("apply session cookies failed", "err", applyErr)
		} else {
			c.setAuthorities(sess.Authorities)
			c.log.Debug("restored session", "user", c.creds.Username, "expires", sess.ExpiresAt.Format(time.RFC3339))
			return nil
		}
	}
	if err := c.Login(ctx); err != nil {
		return err
	}
	return c.saveSession(ctx)
}

// Relogin clears the stored session and logs in again.
func (c *Client) Relogin(ctx context.Context) error {
	_ = c.store.Clear(ctx, c.baseURL, c.creds.Username)
	c.resetJar()
	if err := c.Login(ctx); err != nil {
		return err
	}
	return c.saveSession(ctx)
}

// ClearSession removes persisted cookies for this host+user.
func (c *Client) ClearSession(ctx context.Context) error {
	c.resetJar()
	return c.store.Clear(ctx, c.baseURL, c.creds.Username)
}
