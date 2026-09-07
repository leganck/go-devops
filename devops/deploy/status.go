package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/leganck/go-devops/devops"
)

type State string

const (
	StatePending  State = "pending"
	StateRunning  State = "running"
	StateSuccess  State = "success"
	StateFailed   State = "failed"
	StateCanceled State = "canceled"
	StateUnknown  State = "unknown"
)

type ServerStatus struct {
	ServerID    string `json:"serverId"`
	ServerAlias string `json:"serverAlias"`
	HistoryID   string `json:"historyId"`
	TaskUUID    string `json:"taskUuid"`
	Code        int    `json:"code"`
	State       State  `json:"state"`
	Desc        string `json:"desc"`
	SSHFailed   bool   `json:"sshFailed,omitempty"`
}

type Status struct {
	Handle  *Handle        `json:"handle"`
	Servers []ServerStatus `json:"servers"`
	Done    bool           `json:"done"`
	OK      bool           `json:"ok"`
}

func mapState(code int) State {
	switch code {
	case StatusInProgress:
		return StateRunning
	case StatusPending:
		return StatePending
	case StatusFailed:
		return StateFailed
	case StatusSuccess:
		return StateSuccess
	case StatusCancelled:
		return StateCanceled
	default:
		return StateUnknown
	}
}

func isSSHError(desc string) bool {
	return strings.Contains(strings.ToLower(desc), "ssh")
}

func (c *Client) GetDeploy(ctx context.Context, h *Handle) (*Status, error) {
	if h == nil || h.Env == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "deploy handle is required", nil)
	}
	hist, err := c.History(ctx, HistoryRequest{
		Page:      1,
		Limit:     50,
		EnvName:   h.Env,
		Condition: h.Alias,
	})
	if err != nil {
		return nil, err
	}
	byID := map[string]HistoryItem{}
	byServer := map[string]HistoryItem{}
	for _, item := range hist.Data {
		byID[item.ID] = item
		if _, ok := byServer[item.ServerId]; !ok {
			byServer[item.ServerId] = item
		}
	}
	st := &Status{Handle: h.clone(), Done: true, OK: true}
	ids := h.ServerIDs
	if len(ids) == 0 {
		for sid := range h.HistoryIDs {
			ids = append(ids, sid)
		}
	}
	for _, sid := range ids {
		var item HistoryItem
		ok := false
		if hid := h.HistoryIDs[sid]; hid != "" {
			item, ok = byID[hid]
		}
		if !ok {
			item, ok = byServer[sid]
		}
		ss := ServerStatus{
			ServerID:    sid,
			ServerAlias: h.ServerNames[sid],
			HistoryID:   h.HistoryIDs[sid],
			TaskUUID:    h.TaskUUIDs[sid],
			State:       StateUnknown,
		}
		if ok {
			ss.Code = item.DeployStatus
			ss.State = mapState(item.DeployStatus)
			ss.Desc = item.DeployDesc
			ss.HistoryID = item.ID
			ss.TaskUUID = item.TaskUuid
			ss.ServerAlias = firstNonEmpty(item.ServerAlias, ss.ServerAlias)
			ss.SSHFailed = item.DeployStatus == StatusFailed && isSSHError(item.DeployDesc)
		}
		switch ss.State {
		case StateSuccess:
		case StateFailed, StateCanceled:
			st.OK = false
		default:
			st.Done = false
			st.OK = false
		}
		st.Servers = append(st.Servers, ss)
	}
	if len(st.Servers) == 0 {
		st.Done = false
		st.OK = false
	}
	return st, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

type RetryPolicy struct {
	MaxSSHRetries int
	SSHRetryDelay time.Duration
	PollInterval  time.Duration
	Timeout       time.Duration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxSSHRetries: 2,
		SSHRetryDelay: 5 * time.Second,
		PollInterval:  10 * time.Second,
		Timeout:       10 * time.Minute,
	}
}

func (c *Client) WaitDeploy(ctx context.Context, h *Handle) (*Status, error) {
	if h == nil {
		return nil, devops.NewError(devops.KindInvalidArgument, "deploy handle is required", nil)
	}
	p := c.retry
	if p.PollInterval <= 0 {
		p.PollInterval = 10 * time.Second
	}
	if p.Timeout <= 0 {
		p.Timeout = 10 * time.Minute
	}
	deadline := time.Now().Add(p.Timeout)
	retries := map[string]int{}
	ticker := time.NewTicker(p.PollInterval)
	defer ticker.Stop()

	for {
		st, err := c.GetDeploy(ctx, h)
		if err != nil {
			return nil, err
		}
		if st.Done {
			if st.OK {
				return st, nil
			}
			retried := false
			for i, ss := range st.Servers {
				if ss.SSHFailed && retries[ss.ServerID] < p.MaxSSHRetries {
					retries[ss.ServerID]++
					c.core.Logger().Info("ssh retry", "server", ss.ServerAlias, "attempt", retries[ss.ServerID])
					select {
					case <-ctx.Done():
						return st, ctx.Err()
					case <-time.After(p.SSHRetryDelay):
					}
					key := fmt.Sprintf("%s:retry:%s:%d", h.IdempotencyKey, ss.ServerID, retries[ss.ServerID])
					newID, err := c.startSingle(ctx, h, ss.ServerID, key)
					if err != nil {
						return st, err
					}
					h.HistoryIDs[ss.ServerID] = newID
					st.Servers[i].HistoryID = newID
					retried = true
				}
			}
			if !retried {
				return st, devops.NewError(devops.KindServer, "deploy failed", nil)
			}
		}
		if time.Now().After(deadline) {
			return st, devops.NewError(devops.KindTransient, "deploy wait timeout", ctx.Err())
		}
		select {
		case <-ctx.Done():
			return st, ctx.Err()
		case <-ticker.C:
		}
	}
}
