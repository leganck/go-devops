package logs

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/leganck/go-devops/devops"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

var levels = map[string]struct{}{
	"INFO": {}, "ERROR": {}, "DEBUG": {}, "WARN": {}, "TRACE": {},
}

type QueryRequest struct {
	ProjectName string
	Page        int
	Limit       int
	Time        string
	Level       string
	ThreadName  string
	Query       string
	Unquery     string
}

type Entry struct {
	Time     string `json:"time"`
	Level    string `json:"level"`
	Thread   string `json:"thread"`
	Location string `json:"location"`
	Message  string `json:"message"`
	Topic    string `json:"topic"`
}

type Result struct {
	Count int     `json:"count"`
	Data  []Entry `json:"data"`
}

type tableResponse struct {
	Code  string  `json:"code"`
	Msg   string  `json:"msg"`
	Count int     `json:"count"`
	Data  []Entry `json:"data"`
}

func (c *Client) Query(ctx context.Context, req QueryRequest) (*Result, error) {
	if strings.TrimSpace(req.ProjectName) == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "projectName is required", nil)
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	limit := normalizeLimit(req.Limit)
	level := strings.TrimSpace(req.Level)
	if level != "" {
		level = strings.ToUpper(level)
		if _, ok := levels[level]; !ok {
			return nil, devops.NewError(devops.KindInvalidArgument, "invalid log level", nil)
		}
	}
	params := url.Values{
		"projectName": {req.ProjectName},
		"page":        {strconv.Itoa(page)},
		"limit":       {strconv.Itoa(limit)},
	}
	if req.Time != "" {
		params.Set("time", req.Time)
	}
	if level != "" {
		params.Set("level", level)
	}
	if req.ThreadName != "" {
		params.Set("threadName", req.ThreadName)
	}
	if req.Query != "" {
		params.Set("query", req.Query)
	}
	if req.Unquery != "" {
		params.Set("unquery", req.Unquery)
	}
	var table tableResponse
	if err := c.core.DoGet(ctx, "/programlog/list?"+params.Encode(), &table); err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(table.Data))
	for _, item := range table.Data {
		item.Time = FormatTime(item.Time)
		item.Message = html.UnescapeString(item.Message)
		entries = append(entries, item)
	}
	return &Result{Count: table.Count, Data: entries}, nil
}

type SearchRequest struct {
	ProjectName string
	Env         string
	Group       string
	Alias       string
	Query       string
	Unquery     string
	Level       string
	Since       time.Duration
	Time        string
	Limit       int
	Now         time.Time
}

func (c *Client) Search(ctx context.Context, req SearchRequest) (*Result, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "keyword query is required", nil)
	}
	project, err := c.ResolveProject(ctx, req.Env, req.Group, req.Alias, req.ProjectName)
	if err != nil {
		return nil, err
	}
	timeRange := strings.TrimSpace(req.Time)
	if timeRange == "" && req.Since > 0 {
		now := req.Now
		if now.IsZero() {
			now = time.Now()
		}
		timeRange = BuildTimeRange(req.Since, now)
	}
	return c.Query(ctx, QueryRequest{
		ProjectName: project,
		Limit:       req.Limit,
		Time:        timeRange,
		Level:       req.Level,
		Query:       req.Query,
		Unquery:     req.Unquery,
	})
}

type ContextRequest struct {
	ProjectName string
	Env         string
	Group       string
	Alias       string
	TimeRange   string
	Thread      string
	Limit       int
	Unquery     string
}

func (c *Client) Context(ctx context.Context, req ContextRequest) (*Result, error) {
	if strings.TrimSpace(req.TimeRange) == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "time range is required", nil)
	}
	if strings.TrimSpace(req.Thread) == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "thread is required", nil)
	}
	project, err := c.ResolveProject(ctx, req.Env, req.Group, req.Alias, req.ProjectName)
	if err != nil {
		return nil, err
	}
	return c.Query(ctx, QueryRequest{
		ProjectName: project,
		Limit:       req.Limit,
		Time:        req.TimeRange,
		ThreadName:  req.Thread,
		Unquery:     req.Unquery,
	})
}

func (c *Client) ListProjects(ctx context.Context, envName string) ([]string, error) {
	if envName == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "environment is required", nil)
	}
	path := "/programlog/shortCutPage?" + url.Values{"envName": {envName}}.Encode()
	htmlBody, err := c.core.GetHTML(ctx, path)
	if err != nil {
		return nil, err
	}
	return parseLogProjectsHTML(htmlBody), nil
}

func (c *Client) ResolveProject(ctx context.Context, env, group, alias, project string) (string, error) {
	project = strings.TrimSpace(project)
	if project != "" {
		return project, nil
	}
	env = strings.TrimSpace(env)
	group = strings.TrimSpace(group)
	alias = strings.TrimSpace(alias)
	if env != "" && group != "" && alias != "" {
		return BuildProjectName(env, group, alias), nil
	}
	if env == "" || alias == "" {
		return "", devops.NewError(devops.KindInvalidArgument, "provide --project, or -e/-g/-a, or -e/-a", nil)
	}
	projects, err := c.ListProjects(ctx, env)
	if err != nil {
		return "", err
	}
	matches := FilterByAlias(projects, env, alias)
	switch len(matches) {
	case 0:
		return "", devops.NewError(devops.KindNotFound, fmt.Sprintf("no log project for %q in %s", alias, env), nil)
	case 1:
		return matches[0], nil
	default:
		return "", devops.NewError(devops.KindInvalidArgument, "multiple groups, specify -g or --project: "+strings.Join(matches, ", "), nil)
	}
}

func BuildProjectName(env, group, alias string) string {
	return env + "-" + group + "-" + alias
}

func ParseProjectName(projectName string) (env, group, alias string, err error) {
	parts := strings.Split(projectName, "-")
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("invalid projectName: %s", projectName)
	}
	return parts[0], parts[1], strings.Join(parts[2:], "-"), nil
}

func FormatTime(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	sec, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return raw
	}
	if sec > 1_000_000_000_000 {
		sec = sec / 1000
	}
	return time.Unix(sec, 0).Format("2006-01-02 15:04:05")
}

func BuildTimeRange(since time.Duration, now time.Time) string {
	if since <= 0 {
		since = 2 * time.Hour
	}
	start := now.Add(-since)
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	return fmt.Sprintf("%s ~ %s", start.Format("01-02 15:04:05"), end.Format("01-02 15:04:05"))
}

func ParseSince(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, devops.NewError(devops.KindInvalidArgument, "since is required", nil)
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, devops.NewError(devops.KindInvalidArgument, "invalid since (example: 2h, 30m)", err)
	}
	return d, nil
}

func parseLogProjectsHTML(htmlBody string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlBody))
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var projects []string
	doc.Find("[data-project-name]").Each(func(_ int, s *goquery.Selection) {
		name := strings.TrimSpace(s.AttrOr("data-project-name", ""))
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		projects = append(projects, name)
	})
	sort.Strings(projects)
	return projects
}

func FilterByAlias(projects []string, env, alias string) []string {
	var matches []string
	for _, p := range projects {
		e, _, a, err := ParseProjectName(p)
		if err != nil {
			continue
		}
		if e == env && a == alias {
			matches = append(matches, p)
		}
	}
	return matches
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}
