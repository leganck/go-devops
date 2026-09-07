package sql

import (
	"github.com/leganck/go-devops/devops"
)

// Client is a read-only SQL API. It cannot deploy or query program logs.
type Client struct {
	core     *devops.Client
	pageSize int
	maxPages int
	maxRows  int
}

func New(core *devops.Client, opts ...Option) *Client {
	c := &Client{core: core, pageSize: 500, maxPages: 50, maxRows: 5000}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

type Option func(*Client)

func WithPageSize(n int) Option {
	return func(c *Client) {
		if n > 0 {
			c.pageSize = n
		}
	}
}

func WithMaxPages(n int) Option {
	return func(c *Client) {
		if n > 0 {
			c.maxPages = n
		}
	}
}

func WithMaxRows(n int) Option {
	return func(c *Client) {
		if n > 0 {
			c.maxRows = n
		}
	}
}
