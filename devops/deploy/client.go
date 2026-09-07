package deploy

import (
	"github.com/leganck/go-devops/devops"
)

const (
	permDeploy  = "deployProgram:page"
	permHistory = "deployHistory:list"
)

// Client exposes deploy-only APIs. It cannot execute SQL or query program logs.
type Client struct {
	core  *devops.Client
	store HandleStore
	retry RetryPolicy
}

// New wraps a kernel client. Session must already be established by the caller
// (or StartDeploy will call EnsureSession as needed via HTTP retry).
func New(core *devops.Client, opts ...Option) *Client {
	c := &Client{
		core:  core,
		store: NewMemoryHandleStore(),
		retry: DefaultRetryPolicy(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	if c.store == nil {
		c.store = NewMemoryHandleStore()
	}
	return c
}

type Option func(*Client)

func WithHandleStore(s HandleStore) Option {
	return func(c *Client) { c.store = s }
}

func WithRetryPolicy(p RetryPolicy) Option {
	return func(c *Client) { c.retry = p }
}

func (c *Client) Core() *devops.Client { return c.core }

// Environments lists deployProgram:page environments (not SQL/log envs).
func (c *Client) Environments() []string {
	return c.core.Environments(permDeploy)
}
