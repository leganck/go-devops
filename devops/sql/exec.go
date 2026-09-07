package sql

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/leganck/go-devops/devops"
)

type DatabaseInfo struct {
	Name           string `json:"name"`
	DatasourceName string `json:"datasourceName"`
}

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

type TableInfo struct {
	Columns        []TableColumnInfo `json:"columns"`
	CreateTableSQL string            `json:"createTableSql,omitempty"`
	TableComment   string            `json:"tableComment,omitempty"`
}

type ExecRequest struct {
	EnvName        string
	DatasourceName string
	SQL            string
	Page           int
	Limit          int
	UniqueID       string
}

type Result struct {
	Type         string                   `json:"type"`
	ExecTime     int64                    `json:"execTime"`
	HasMore      bool                     `json:"hasMore"`
	SQLPage      bool                     `json:"sqlPage"`
	AffectedRows int                      `json:"affectedRows"`
	Columns      []string                 `json:"columns"`
	Rows         []map[string]interface{} `json:"rows"`
}

func (r *Result) IsSelect() bool {
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

func (c *Client) ListDatabases(ctx context.Context, envName string) ([]DatabaseInfo, error) {
	if envName == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "environment is required", nil)
	}
	path := "/dsourceDbexec/database?" + url.Values{"envName": {envName}}.Encode()
	var items []DatabaseInfo
	if err := c.core.DoGet(ctx, path, &items); err != nil {
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

func (c *Client) ResolveDatasource(ctx context.Context, envName, nameOrDatasource string) (*DatabaseInfo, error) {
	if nameOrDatasource == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "datasource is required", nil)
	}
	dbs, err := c.ListDatabases(ctx, envName)
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
	return nil, devops.NewError(devops.KindNotFound, "datasource not found: "+nameOrDatasource+"; candidates: "+FormatDatabaseCandidates(dbs, 8), nil)
}

func (c *Client) ResolveOrDefaultDatasource(ctx context.Context, envName, nameOrDatasource string) (*DatabaseInfo, error) {
	nameOrDatasource = strings.TrimSpace(nameOrDatasource)
	if nameOrDatasource != "" {
		return c.ResolveDatasource(ctx, envName, nameOrDatasource)
	}
	dbs, err := c.ListDatabases(ctx, envName)
	if err != nil {
		return nil, err
	}
	switch len(dbs) {
	case 0:
		return nil, devops.NewError(devops.KindNotFound, "no datasource in env "+envName, nil)
	case 1:
		return &dbs[0], nil
	default:
		return nil, devops.NewError(devops.KindInvalidArgument, "multiple datasources, specify -d; candidates: "+FormatDatabaseCandidates(dbs, 8), nil)
	}
}

func FormatDatabaseCandidates(dbs []DatabaseInfo, max int) string {
	if len(dbs) == 0 {
		return "(none)"
	}
	if max <= 0 {
		max = 8
	}
	parts := make([]string, 0, max)
	for i, db := range dbs {
		if i >= max {
			parts = append(parts, "...")
			break
		}
		if db.Name != "" && db.Name != db.DatasourceName {
			parts = append(parts, fmt.Sprintf("%s(%s)", db.Name, db.DatasourceName))
		} else {
			parts = append(parts, db.DatasourceName)
		}
	}
	return strings.Join(parts, ", ")
}

func (c *Client) ListTables(ctx context.Context, envName, datasourceName string) ([]string, error) {
	if envName == "" || datasourceName == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "env and datasource are required", nil)
	}
	params := url.Values{"envName": {envName}, "datasourceName": {datasourceName}}
	var items []struct {
		Name string `json:"name"`
	}
	if err := c.core.DoGet(ctx, "/dsourceDbexec/table?"+params.Encode(), &items); err != nil {
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

func (c *Client) GetTableInfo(ctx context.Context, envName, datasourceName, tableName string) (*TableInfo, error) {
	if envName == "" || datasourceName == "" || tableName == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "env, datasource and table are required", nil)
	}
	params := url.Values{
		"envName":        {envName},
		"datasourceName": {datasourceName},
		"tableName":      {tableName},
	}
	var raw tableInfoRaw
	if err := c.core.DoGet(ctx, "/dsourceDbexec/tableInfo?"+params.Encode(), &raw); err != nil {
		return nil, err
	}
	columnsNode := raw.Column
	if len(columnsNode) == 0 || string(columnsNode) == "null" {
		columnsNode = raw.Columns
	}
	columns, err := parseTableColumns(columnsNode)
	if err != nil {
		return nil, devops.NewError(devops.KindServer, "parse table columns", err)
	}
	createSQL, comment := parseTableMeta(raw.Table)
	return &TableInfo{Columns: columns, CreateTableSQL: createSQL, TableComment: comment}, nil
}

func (c *Client) Exec(ctx context.Context, req ExecRequest) (*Result, error) {
	if err := CheckReadOnly(req.SQL); err != nil {
		return nil, err
	}
	return c.execRaw(ctx, req)
}

func (c *Client) ExecAll(ctx context.Context, req ExecRequest) (*Result, error) {
	if err := CheckReadOnly(req.SQL); err != nil {
		return nil, err
	}
	pageSize := c.normalizePage(req.Limit)
	firstReq := req
	if firstReq.Page <= 0 {
		firstReq.Page = 1
	}
	firstReq.Limit = pageSize
	if firstReq.UniqueID == "" {
		firstReq.UniqueID = newUniqueID()
	}
	first, err := c.execRaw(ctx, firstReq)
	if err != nil {
		return nil, err
	}
	if !first.IsSelect() || first.SQLPage || !first.HasMore {
		return first, nil
	}
	rows := append([]map[string]interface{}{}, first.Rows...)
	desired := c.maxRows
	if desired <= 0 {
		desired = 5000
	}
	page := firstReq.Page
	maxPages := c.maxPages
	if maxPages <= 0 {
		maxPages = 50
	}
	for first.HasMore && len(rows) < desired && page < maxPages {
		page++
		next, err := c.execRaw(ctx, ExecRequest{
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
	if len(rows) > desired {
		rows = rows[:desired]
	}
	first.Rows = rows
	first.HasMore = false
	return first, nil
}

func (c *Client) execRaw(ctx context.Context, req ExecRequest) (*Result, error) {
	if req.EnvName == "" || req.DatasourceName == "" || strings.TrimSpace(req.SQL) == "" {
		return nil, devops.NewError(devops.KindInvalidArgument, "env, datasource and SQL are required", nil)
	}
	limit := c.normalizePage(req.Limit)
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
	if err := c.core.DoPost(ctx, "/dsourceDbexec/exec", params, &raw); err != nil {
		return nil, err
	}
	return parseSQLExecutionResult(&raw)
}

func (c *Client) normalizePage(limit int) int {
	def := c.pageSize
	if def <= 0 {
		def = 500
	}
	max := 5000
	if limit <= 0 {
		return def
	}
	if limit > max {
		return max
	}
	return limit
}

func parseSQLExecutionResult(raw *sqlExecData) (*Result, error) {
	if raw == nil {
		return nil, devops.NewError(devops.KindServer, "empty SQL result", nil)
	}
	result := &Result{
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
			return nil, devops.NewError(devops.KindServer, "parse select rows", err)
		}
		result.Rows = rows
		return result, nil
	}
	if len(raw.Data) == 0 || string(raw.Data) == "null" {
		return result, nil
	}
	var affected int
	if err := json.Unmarshal(raw.Data, &affected); err != nil {
		var asString string
		if err2 := json.Unmarshal(raw.Data, &asString); err2 == nil {
			n, convErr := strconv.Atoi(asString)
			if convErr != nil {
				return nil, devops.NewError(devops.KindServer, "parse affected rows", err)
			}
			affected = n
		} else {
			return nil, devops.NewError(devops.KindServer, "parse affected rows", err)
		}
	}
	result.AffectedRows = affected
	return result, nil
}

func parseTableColumns(node json.RawMessage) ([]TableColumnInfo, error) {
	if len(node) == 0 || string(node) == "null" {
		return []TableColumnInfo{}, nil
	}
	var rawColumns []map[string]interface{}
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
		n := normalizeFieldName(k)
		if n == "createtable" {
			createSQL = fmt.Sprint(v)
		}
		if n == "comment" || n == "tablecomment" {
			comment = fmt.Sprint(v)
		}
	}
	return createSQL, comment
}

func newUniqueID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", len(b))
	}
	hexStr := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexStr[0:8], hexStr[8:12], hexStr[12:16], hexStr[16:20], hexStr[20:32])
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				return strconv.FormatFloat(t, 'f', -1, 64)
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
