package logs

import "github.com/leganck/go-devops/devops"

// Client is a program-log API. It cannot deploy or execute SQL.
type Client struct {
	core *devops.Client
}

func New(core *devops.Client) *Client {
	return &Client{core: core}
}
