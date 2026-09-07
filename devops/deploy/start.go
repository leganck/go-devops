package deploy

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/leganck/go-devops/devops"
)

// StartRequest starts a deployment. Env, alias and version are required.
type StartRequest struct {
	Env          string
	ProgramAlias string
	ProgramType  string
	Version      string
	ServerAlias  string
	NotifyUser   string
	NotifyMemo   string
	TypeFallback bool
}

func (c *Client) StartDeploy(ctx context.Context, req StartRequest, idempotencyKey string) (*Handle, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "idempotency key is required", nil)
	}
	if req.Env == "" || req.ProgramAlias == "" || req.Version == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "env, program alias and version are required", nil)
	}
	if req.ProgramType == "" {
		req.ProgramType = "snapshots"
		req.TypeFallback = true
	}

	id := idempotencyID(c.core.BaseURL(), c.core.Username(), req.Env, req.ProgramAlias, req.Version, req.ServerAlias, idempotencyKey)
	if existing, err := c.lookupIdempotent(ctx, id); err != nil {
		return nil, err
	} else if existing != nil {
		c.core.Logger().Info("idempotent deploy reuse", "key", idempotencyKey, "env", req.Env)
		return existing, nil
	}

	if err := c.ensureProgram(ctx, req.Env, req.ProgramAlias); err != nil {
		return nil, err
	}
	path, actualVersion, programType, err := c.resolveVersion(ctx, req)
	if err != nil {
		return nil, err
	}
	req.ProgramType = programType
	info, err := c.resolveServers(ctx, req.Env, req.ProgramAlias, req.ServerAlias)
	if err != nil {
		return nil, err
	}

	memo := req.NotifyMemo
	if memo == "" && req.NotifyUser != "" {
		if req.ServerAlias == "" {
			memo = fmt.Sprintf("Deploy %s version %s to all servers", req.ProgramAlias, actualVersion)
		} else {
			memo = fmt.Sprintf("Deploy %s version %s to server %s", req.ProgramAlias, actualVersion, req.ServerAlias)
		}
	}
	params := url.Values{
		"programAliasName": {req.ProgramAlias},
		"programType":      {req.ProgramType},
		"relativePath":     {path},
		"regularTime":      {""},
		"servers":          {strings.Join(info.IDs, ",")},
		"envName":          {req.Env},
		"programVersion":   {actualVersion},
	}
	if req.NotifyUser != "" {
		params.Set("notifyUser", req.NotifyUser)
		params.Set("notifyChecked", "1")
		params.Set("notifyMemo", memo)
	}

	if err := c.core.RequirePermission(permDeploy, req.Env); err != nil {
		return nil, err
	}
	if err := c.core.DoPost(ctx, "/deployProgram/start", params, nil); err != nil {
		return nil, err
	}

	h, err := c.collectHandle(ctx, req, info, path, actualVersion, idempotencyKey)
	if err != nil {
		return nil, err
	}
	_ = c.store.Put(ctx, id, h)
	return h, nil
}

func (c *Client) ensureProgram(ctx context.Context, env, alias string) error {
	programs, err := c.Programs(ctx, env)
	if err != nil {
		return err
	}
	for _, p := range programs {
		if p == alias {
			return nil
		}
	}
	return devops.NewError(devops.KindNotFound, fmt.Sprintf("program %s not found in env %s", alias, env), nil)
}

func (c *Client) resolveVersion(ctx context.Context, req StartRequest) (path, version, programType string, err error) {
	versions, err := c.Versions(ctx, VersionRequest{
		ProgramAliasName: req.ProgramAlias,
		ProgramType:      req.ProgramType,
		EnvName:          req.Env,
	})
	if err != nil {
		return "", "", "", err
	}
	if len(versions) == 0 && req.TypeFallback && req.ProgramType == "snapshots" {
		alt, altErr := c.Versions(ctx, VersionRequest{
			ProgramAliasName: req.ProgramAlias,
			ProgramType:      "releases",
			EnvName:          req.Env,
		})
		if altErr != nil {
			return "", "", "", altErr
		}
		if len(alt) > 0 {
			versions = alt
			req.ProgramType = "releases"
		}
	}
	for _, v := range versions {
		if v.Version == req.Version {
			return v.RelativePath, v.Version, req.ProgramType, nil
		}
	}
	for _, v := range versions {
		if FuzzyMatchVersion(req.Version, v.Version) {
			return v.RelativePath, v.Version, req.ProgramType, nil
		}
	}
	return "", "", "", devops.NewError(devops.KindVersionNotFound, fmt.Sprintf("version %s not found in env %s", req.Version, req.Env), nil)
}

type serverInfo struct {
	IDs   []string
	Names map[string]string
}

func (c *Client) resolveServers(ctx context.Context, env, alias, serverAlias string) (*serverInfo, error) {
	groups, err := c.Servers(ctx, ServerRequest{EnvName: env, ProgramAliasName: alias})
	if err != nil {
		return nil, err
	}
	var all []Server
	for _, g := range groups {
		all = append(all, g...)
	}
	info := &serverInfo{Names: map[string]string{}}
	if len(all) == 0 {
		return nil, devops.NewError(devops.KindNotFound, "no servers available", nil)
	}
	if serverAlias == "" {
		for _, s := range all {
			info.IDs = append(info.IDs, s.ServerID)
			info.Names[s.ServerID] = s.ServerAlias
		}
		return info, nil
	}
	if len(all) == 1 {
		info.IDs = []string{all[0].ServerID}
		info.Names[all[0].ServerID] = all[0].ServerAlias
		return info, nil
	}
	for _, s := range all {
		if s.ServerAlias == serverAlias {
			info.IDs = []string{s.ServerID}
			info.Names[s.ServerID] = s.ServerAlias
			return info, nil
		}
	}
	return nil, devops.NewError(devops.KindNotFound, "server not found: "+serverAlias, nil)
}

func (c *Client) collectHandle(ctx context.Context, req StartRequest, info *serverInfo, path, version, key string) (*Handle, error) {
	hist, err := c.History(ctx, HistoryRequest{
		Page:      1,
		Limit:     50,
		EnvName:   req.Env,
		Condition: req.ProgramAlias,
	})
	if err != nil {
		return nil, err
	}
	h := &Handle{
		HistoryIDs:     map[string]string{},
		TaskUUIDs:      map[string]string{},
		Env:            req.Env,
		Alias:          req.ProgramAlias,
		Version:        version,
		ProgramType:    req.ProgramType,
		RelativePath:   path,
		ServerIDs:      append([]string(nil), info.IDs...),
		ServerNames:    copyMap(info.Names),
		IdempotencyKey: key,
		BaseURL:        c.core.BaseURL(),
		CreatedAt:      time.Now(),
	}
	pending := map[string]HistoryItem{}
	for _, item := range hist.Data {
		if item.DeployStatus == StatusSuccess {
			continue
		}
		if _, ok := pending[item.ServerId]; !ok {
			pending[item.ServerId] = item
		}
	}
	for _, sid := range info.IDs {
		if item, ok := pending[sid]; ok {
			h.HistoryIDs[sid] = item.ID
			h.TaskUUIDs[sid] = item.TaskUuid
			if item.ServerAlias != "" {
				h.ServerNames[sid] = item.ServerAlias
			}
		}
	}
	return h, nil
}

func (c *Client) startSingle(ctx context.Context, h *Handle, serverID string, key string) (string, error) {
	params := url.Values{
		"programAliasName": {h.Alias},
		"programType":      {h.ProgramType},
		"relativePath":     {h.RelativePath},
		"regularTime":      {""},
		"servers":          {serverID},
		"envName":          {h.Env},
		"programVersion":   {h.Version},
	}
	if err := c.core.DoPost(ctx, "/deployProgram/start", params, nil); err != nil {
		return "", err
	}
	hist, err := c.History(ctx, HistoryRequest{Page: 1, Limit: 10, EnvName: h.Env, Condition: h.Alias})
	if err != nil {
		return "", err
	}
	for _, item := range hist.Data {
		if item.ServerId == serverID && item.DeployStatus != StatusSuccess {
			return item.ID, nil
		}
	}
	return "", devops.NewError(devops.KindNotFound, "retry task not found for server "+serverID, nil)
}
