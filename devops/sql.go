package devops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	defaultSQLPageSize = 500
	maxSQLPageSize     = 5000
	maxSQLAutoPages    = 50
)

// DatabaseInfo 表示 DevOps 环境下的一个数据源（对应 JDBC catalog）。
type DatabaseInfo struct {
	Name           string `json:"name"`
	DatasourceName string `json:"datasourceName"`
}

// TableColumnInfo 表示表字段元数据。
type TableColumnInfo struct {
	OrdinalPosition int    `json:"ordinalPosition"`
	ColumnName      string `json:"columnName"`
	ColumnType      string `json:"columnType"`
	IsNullable      string `json:"isNullable"`
	ColumnComment   string `json:"columnComment"`
	ColumnDefault   string `json:"columnDefault"`
	ColumnKey       string `json:"columnKey"`
	Extra           string `json:"extra"`
}

// TableInfo 表示表结构查询结果。
type TableInfo struct {
	Columns        []TableColumnInfo `json:"columns"`
	CreateTableSQL string            `json:"createTableSql,omitempty"`
	TableComment   string            `json:"tableComment,omitempty"`
}

// ExecuteSQLRequest 包含执行 SQL 的参数。
type ExecuteSQLRequest struct {
	EnvName        string
	DatasourceName string
	SQL            string
	Page           int    // 页码，从 1 开始；<=0 表示不传
	Limit          int    // 每页行数；<=0 使用默认值
	UniqueID       string // 为空时自动生成
}

// SQLExecutionResult 是 DevOps SQL 执行接口的规范化结果。
type SQLExecutionResult struct {
	Type         string                   `json:"type"`
	ExecTime     int64                    `json:"execTime"`
	HasMore      bool                     `json:"hasMore"`
	SQLPage      bool                     `json:"sqlPage"`
	AffectedRows int                      `json:"affectedRows"`
	Columns      []string                 `json:"columns"`
	Rows         []map[string]interface{} `json:"rows"`
}

// IsSelect 当结果类型为 select 时返回 true。
func (r *SQLExecutionResult) IsSelect() bool {
	return r != nil && strings.EqualFold(r.Type, "select")
}

type sqlExecData struct {
	Type     string          `json:"type"`
	ExecTime int64           `json:"execTime"`
	HasMore  bool            `json:"hasMore"`
	SQLPage  bool            `json:"sqlPage"`
	Columns  []string        `json:"columns"`
	Data     json.RawMessage `json:"data"`
}

type tableInfoRaw struct {
	Column  json.RawMessage `json:"column"`
	Columns json.RawMessage `json:"columns"`
	Table   json.RawMessage `json:"table"`
}

type tableColumnRaw map[string]interface{}

// ListDatabases 查询指定环境下的数据源列表。
func (d *DevOps) ListDatabases(ctx context.Context, envName string) ([]DatabaseInfo, error) {
	if envName == "" {
		return nil, fmt.Errorf("环境名称不能为空")
	}

	path := "/dsourceDbexec/database?" + url.Values{"envName": {envName}}.Encode()
	var items []DatabaseInfo
	if err := d.GetRequest(ctx, path, &items); err != nil {
		return nil, err
	}

	result := make([]DatabaseInfo, 0, len(items))
	for _, item := range items {
		if item.DatasourceName == "" {
			continue
		}
		if item.Name == "" {
			item.Name = item.DatasourceName
		}
		result = append(result, item)
	}
	return result, nil
}

// ResolveDatasource 按 catalog name 或 datasourceName 解析数据源。
func (d *DevOps) ResolveDatasource(ctx context.Context, envName, nameOrDatasource string) (*DatabaseInfo, error) {
	if nameOrDatasource == "" {
		return nil, fmt.Errorf("数据源名称不能为空")
	}

	dbs, err := d.ListDatabases(ctx, envName)
	if err != nil {
		return nil, err
	}

	var byName, byDatasource *DatabaseInfo
	for i := range dbs {
		db := &dbs[i]
		if db.Name == nameOrDatasource && byName == nil {
			byName = db
		}
		if db.DatasourceName == nameOrDatasource && byDatasource == nil {
			byDatasource = db
		}
	}

	if byName != nil {
		return byName, nil
	}
	if byDatasource != nil {
		return byDatasource, nil
	}
	return nil, fmt.Errorf("未找到数据源: %s", nameOrDatasource)
}

// ListTables 查询指定数据源下的表名列表。
func (d *DevOps) ListTables(ctx context.Context, envName, datasourceName string) ([]string, error) {
	if envName == "" {
		return nil, fmt.Errorf("环境名称不能为空")
	}
	if datasourceName == "" {
		return nil, fmt.Errorf("数据源名称不能为空")
	}

	params := url.Values{
		"envName":        {envName},
		"datasourceName": {datasourceName},
	}
	path := "/dsourceDbexec/table?" + params.Encode()

	var items []struct {
		Name string `json:"name"`
	}
	if err := d.GetRequest(ctx, path, &items); err != nil {
		return nil, err
	}

	tables := make([]string, 0, len(items))
	for _, item := range items {
		if item.Name != "" {
			tables = append(tables, item.Name)
		}
	}
	return tables, nil
}

// GetTableInfo 查询指定表的字段与建表信息。
func (d *DevOps) GetTableInfo(ctx context.Context, envName, datasourceName, tableName string) (*TableInfo, error) {
	if envName == "" {
		return nil, fmt.Errorf("环境名称不能为空")
	}
	if datasourceName == "" {
		return nil, fmt.Errorf("数据源名称不能为空")
	}
	if tableName == "" {
		return nil, fmt.Errorf("表名不能为空")
	}

	params := url.Values{
		"envName":        {envName},
		"datasourceName": {datasourceName},
		"tableName":      {tableName},
	}
	path := "/dsourceDbexec/tableInfo?" + params.Encode()

	var raw tableInfoRaw
	if err := d.GetRequest(ctx, path, &raw); err != nil {
		return nil, err
	}

	columnsNode := raw.Column
	if len(columnsNode) == 0 || string(columnsNode) == "null" {
		columnsNode = raw.Columns
	}

	columns, err := parseTableColumns(columnsNode)
	if err != nil {
		return nil, fmt.Errorf("解析表字段失败: %w", err)
	}

	createSQL, comment := parseTableMeta(raw.Table)
	return &TableInfo{
		Columns:        columns,
		CreateTableSQL: createSQL,
		TableComment:   comment,
	}, nil
}

// ExecuteSQL 执行单页 SQL。
func (d *DevOps) ExecuteSQL(ctx context.Context, req *ExecuteSQLRequest) (*SQLExecutionResult, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}
	if req.EnvName == "" {
		return nil, fmt.Errorf("环境名称不能为空")
	}
	if req.DatasourceName == "" {
		return nil, fmt.Errorf("数据源名称不能为空")
	}
	if strings.TrimSpace(req.SQL) == "" {
		return nil, fmt.Errorf("SQL 不能为空")
	}

	limit := normalizePageSize(req.Limit)
	uniqueID := req.UniqueID
	if uniqueID == "" {
		uniqueID = newUniqueID()
	}

	params := url.Values{
		"envName":        {req.EnvName},
		"datasourceName": {req.DatasourceName},
		"sql":            {req.SQL},
		"uniqueId":       {uniqueID},
	}
	if req.Page > 0 {
		params.Set("page", strconv.Itoa(req.Page))
	}
	if req.Limit > 0 || req.Page > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	var raw sqlExecData
	if err := d.PostRequest(ctx, "/dsourceDbexec/exec", params, &raw); err != nil {
		return nil, err
	}

	return parseSQLExecutionResult(&raw)
}

// ExecuteSQLAll 执行 SQL；对 select 且 hasMore 时自动翻页合并（有上限）。
func (d *DevOps) ExecuteSQLAll(ctx context.Context, req *ExecuteSQLRequest) (*SQLExecutionResult, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}

	pageSize := normalizePageSize(req.Limit)
	firstReq := *req
	if firstReq.Page <= 0 {
		firstReq.Page = 1
	}
	firstReq.Limit = pageSize
	if firstReq.UniqueID == "" {
		firstReq.UniqueID = newUniqueID()
	}

	first, err := d.ExecuteSQL(ctx, &firstReq)
	if err != nil {
		return nil, err
	}
	if !first.IsSelect() || first.SQLPage || !first.HasMore {
		return first, nil
	}

	rows := append([]map[string]interface{}{}, first.Rows...)
	desiredRows := pageSize * 20
	if desiredRows < 5000 {
		desiredRows = 5000
	}

	page := firstReq.Page
	for first.HasMore && len(rows) < desiredRows && page < maxSQLAutoPages {
		page++
		next, err := d.ExecuteSQL(ctx, &ExecuteSQLRequest{
			EnvName:        req.EnvName,
			DatasourceName: req.DatasourceName,
			SQL:            req.SQL,
			Page:           page,
			Limit:          pageSize,
			UniqueID:       newUniqueID(),
		})
		if err != nil {
			return nil, err
		}
		if len(next.Rows) == 0 {
			break
		}
		rows = append(rows, next.Rows...)
		first.HasMore = next.HasMore
		first.ExecTime += next.ExecTime
	}

	if len(rows) > desiredRows {
		rows = rows[:desiredRows]
	}
	first.Rows = rows
	first.HasMore = false
	return first, nil
}

// KillSQL 终止指定 uniqueId 的 SQL 执行。
func (d *DevOps) KillSQL(ctx context.Context, envName, datasourceName, uniqueID string) error {
	if uniqueID == "" {
		return nil
	}
	if envName == "" {
		return fmt.Errorf("环境名称不能为空")
	}
	if datasourceName == "" {
		return fmt.Errorf("数据源名称不能为空")
	}

	params := url.Values{
		"envName":        {envName},
		"datasourceName": {datasourceName},
		"uniqueId":       {uniqueID},
	}
	return d.PostRequest(ctx, "/dsourceDbexec/kill", params, nil)
}

func parseSQLExecutionResult(raw *sqlExecData) (*SQLExecutionResult, error) {
	if raw == nil {
		return nil, fmt.Errorf("SQL 执行返回空数据")
	}

	result := &SQLExecutionResult{
		Type:     raw.Type,
		ExecTime: raw.ExecTime,
		HasMore:  raw.HasMore,
		SQLPage:  raw.SQLPage,
		Columns:  raw.Columns,
		Rows:     []map[string]interface{}{},
	}
	if result.Columns == nil {
		result.Columns = []string{}
	}

	if strings.EqualFold(raw.Type, "select") {
		if len(raw.Data) == 0 || string(raw.Data) == "null" {
			return result, nil
		}
		var rows []map[string]interface{}
		if err := json.Unmarshal(raw.Data, &rows); err != nil {
			return nil, fmt.Errorf("解析 select 结果行失败: %w", err)
		}
		result.Rows = rows
		return result, nil
	}

	if len(raw.Data) == 0 || string(raw.Data) == "null" {
		return result, nil
	}
	var affected int
	if err := json.Unmarshal(raw.Data, &affected); err != nil {
		// 兼容字符串数字
		var asString string
		if err2 := json.Unmarshal(raw.Data, &asString); err2 == nil {
			n, convErr := strconv.Atoi(asString)
			if convErr != nil {
				return nil, fmt.Errorf("解析影响行数失败: %w", err)
			}
			affected = n
		} else {
			return nil, fmt.Errorf("解析影响行数失败: %w", err)
		}
	}
	result.AffectedRows = affected
	return result, nil
}

func parseTableColumns(node json.RawMessage) ([]TableColumnInfo, error) {
	if len(node) == 0 || string(node) == "null" {
		return []TableColumnInfo{}, nil
	}

	var rawColumns []tableColumnRaw
	if err := json.Unmarshal(node, &rawColumns); err != nil {
		return nil, err
	}

	columns := make([]TableColumnInfo, 0, len(rawColumns))
	for i, raw := range rawColumns {
		col := TableColumnInfo{
			OrdinalPosition: i + 1,
			ColumnName:      firstString(raw, "column_name", "columnName", "field"),
			ColumnType:      firstString(raw, "column_type", "columnType", "type"),
			IsNullable:      firstString(raw, "is_nullable", "isNullable", "nullable", "null"),
			ColumnComment:   firstString(raw, "column_comment", "columnComment", "comment"),
			ColumnDefault:   firstString(raw, "column_default", "columnDefault", "default"),
			ColumnKey:       firstString(raw, "column_key", "columnKey", "key"),
			Extra:           firstString(raw, "extra"),
		}
		if pos := firstInt(raw, "ordinal_position", "ordinalPosition"); pos > 0 {
			col.OrdinalPosition = pos
		}
		columns = append(columns, col)
	}
	return columns, nil
}

func parseTableMeta(tableNode json.RawMessage) (createSQL, comment string) {
	if len(tableNode) == 0 || string(tableNode) == "null" {
		return "", ""
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(tableNode, &obj); err != nil {
		return "", ""
	}

	for k, v := range obj {
		normalized := normalizeFieldName(k)
		if normalized == "createtable" {
			createSQL = fmt.Sprint(v)
		}
		if normalized == "comment" || normalized == "tablecomment" {
			comment = fmt.Sprint(v)
		}
	}
	return createSQL, comment
}

func normalizePageSize(limit int) int {
	if limit <= 0 {
		return defaultSQLPageSize
	}
	if limit > maxSQLPageSize {
		return maxSQLPageSize
	}
	return limit
}

func newUniqueID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", len(b))
	}
	hexStr := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hexStr[0:8], hexStr[8:12], hexStr[12:16], hexStr[16:20], hexStr[20:32])
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				return strconv.FormatFloat(t, 'f', -1, 64)
			case json.Number:
				return t.String()
			default:
				return fmt.Sprint(t)
			}
		}
	}
	return ""
}

func firstInt(m map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			switch t := v.(type) {
			case float64:
				return int(t)
			case json.Number:
				n, _ := t.Int64()
				return int(n)
			case string:
				n, err := strconv.Atoi(t)
				if err == nil {
					return n
				}
			}
		}
	}
	return 0
}

func normalizeFieldName(value string) string {
	var b strings.Builder
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			if ch >= 'A' && ch <= 'Z' {
				b.WriteRune(ch + ('a' - 'A'))
			} else {
				b.WriteRune(ch)
			}
		}
	}
	return b.String()
}
