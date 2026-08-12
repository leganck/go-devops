package devops

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	defaultProgramLogLimit = 50
	maxProgramLogLimit     = 500
)

var programLogLevels = map[string]struct{}{
	"INFO":  {},
	"ERROR": {},
	"DEBUG": {},
	"WARN":  {},
	"TRACE": {},
}

// ProgramLogRequest 包含程序日志查询参数。
type ProgramLogRequest struct {
	ProjectName string
	Page        int
	Limit       int
	Time        string
	Level       string
	ThreadName  string
	Query       string
	Unquery     string
}

// ProgramLogEntry 表示单条程序日志。
type ProgramLogEntry struct {
	Time     string `json:"time"`
	Level    string `json:"level"`
	Thread   string `json:"thread"`
	Location string `json:"location"`
	Message  string `json:"message"`
	Topic    string `json:"topic"`
}

// ProgramLogResult 是程序日志列表结果。
type ProgramLogResult struct {
	Count int               `json:"count"`
	Data  []ProgramLogEntry `json:"data"`
}

type programLogTableResponse struct {
	Code  string            `json:"code"`
	Msg   string            `json:"msg"`
	Count int               `json:"count"`
	Data  []ProgramLogEntry `json:"data"`
}

// ListProgramLogs 查询程序日志（GET /programlog/list）。
func (d *DevOps) ListProgramLogs(ctx context.Context, req *ProgramLogRequest) (*ProgramLogResult, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}
	if strings.TrimSpace(req.ProjectName) == "" {
		return nil, fmt.Errorf("projectName 不能为空")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	limit := normalizeProgramLogLimit(req.Limit)

	level := strings.TrimSpace(req.Level)
	if level != "" {
		level = strings.ToUpper(level)
		if _, ok := programLogLevels[level]; !ok {
			return nil, fmt.Errorf("无效的日志级别: %s（支持 INFO|ERROR|DEBUG|WARN|TRACE）", req.Level)
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

	var table programLogTableResponse
	if err := d.GetRequest(ctx, "/programlog/list?"+params.Encode(), &table); err != nil {
		return nil, err
	}

	entries := make([]ProgramLogEntry, 0, len(table.Data))
	for _, item := range table.Data {
		item.Time = FormatProgramLogTime(item.Time)
		item.Message = UnescapeLogMessage(item.Message)
		entries = append(entries, item)
	}

	return &ProgramLogResult{
		Count: table.Count,
		Data:  entries,
	}, nil
}

// ListLogProjects 从程序日志快捷页解析环境下的 projectName 列表。
func (d *DevOps) ListLogProjects(ctx context.Context, envName string) ([]string, error) {
	if envName == "" {
		return nil, fmt.Errorf("环境名称不能为空")
	}

	path := "/programlog/shortCutPage?" + url.Values{"envName": {envName}}.Encode()
	htmlBody, err := d.getHTML(ctx, path)
	if err != nil {
		return nil, err
	}
	return parseLogProjectsHTML(htmlBody), nil
}

// ResolveLogProject 解析最终的 projectName。
// 优先级：project > env+group+alias > env+alias（自动匹配）。
func (d *DevOps) ResolveLogProject(ctx context.Context, env, group, alias, project string) (string, error) {
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
		return "", fmt.Errorf("请提供 --project，或提供 -e/-g/-a，或提供 -e/-a 以自动解析服务器组")
	}

	projects, err := d.ListLogProjects(ctx, env)
	if err != nil {
		return "", err
	}

	matches := filterLogProjectsByAlias(projects, env, alias)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("未找到程序 %q 在环境 %s 下的日志项目；请先执行: go-devops logs projects -e %s", alias, env, env)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("程序 %q 匹配到多个服务器组，请指定 -g 或 --project。候选: %s", alias, strings.Join(matches, ", "))
	}
}

// BuildProjectName 构建 env-group-alias 形式的 projectName。
func BuildProjectName(env, group, alias string) string {
	return env + "-" + group + "-" + alias
}

// ParseProjectName 解析 projectName 为 env/group/alias。
// 规则与后端一致：parts[0]=env, parts[1]=group, join(parts[2:], "-")=alias。
func ParseProjectName(projectName string) (env, group, alias string, err error) {
	parts := strings.Split(projectName, "-")
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("无效的 projectName: %s（期望 env-group-alias）", projectName)
	}
	return parts[0], parts[1], strings.Join(parts[2:], "-"), nil
}

// FormatProgramLogTime 将 Unix 秒字符串格式化为 yyyy-MM-dd HH:mm:ss；否则原样返回。
func FormatProgramLogTime(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	sec, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return raw
	}
	// 毫秒级时间戳容错
	if sec > 1_000_000_000_000 {
		sec = sec / 1000
	}
	return time.Unix(sec, 0).Format("2006-01-02 15:04:05")
}

// UnescapeLogMessage 还原后端 HTML escape 后的日志正文。
func UnescapeLogMessage(msg string) string {
	return html.UnescapeString(msg)
}

// BuildProgramLogTimeRange 根据相对时长生成后端 time 参数。
// 格式：MM-dd HH:mm:ss ~ MM-dd 23:59:59（结束为当天末，对齐 Web 默认习惯）。
func BuildProgramLogTimeRange(since time.Duration, now time.Time) string {
	if since <= 0 {
		since = 2 * time.Hour
	}
	start := now.Add(-since)
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	return fmt.Sprintf("%s ~ %s", start.Format("01-02 15:04:05"), end.Format("01-02 15:04:05"))
}

// ParseSinceDuration 解析 2h / 30m / 1h30m 等相对时长。
func ParseSinceDuration(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("since 不能为空")
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("无效的 since 值 %q（示例: 2h, 30m）: %w", raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("since 必须为正数")
	}
	return d, nil
}

func (d *DevOps) getHTML(ctx context.Context, path string) (string, error) {
	resp, err := d.get(ctx, path)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: 意外的状态码 %d", path, resp.StatusCode)
	}
	return string(body), nil
}

func parseLogProjectsHTML(htmlBody string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlBody))
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
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

func filterLogProjectsByAlias(projects []string, env, alias string) []string {
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

func normalizeProgramLogLimit(limit int) int {
	if limit <= 0 {
		return defaultProgramLogLimit
	}
	if limit > maxProgramLogLimit {
		return maxProgramLogLimit
	}
	return limit
}
