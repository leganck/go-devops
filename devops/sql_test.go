package devops

import (
	"encoding/json"
	"testing"
)

func TestParseSQLExecutionResult_Select(t *testing.T) {
	rawJSON := `{
		"type": "select",
		"execTime": 12,
		"hasMore": true,
		"sqlPage": false,
		"columns": ["id", "name"],
		"data": [
			{"id": 1, "name": "a"},
			{"id": 2, "name": "b"}
		]
	}`

	var raw sqlExecData
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	result, err := parseSQLExecutionResult(&raw)
	if err != nil {
		t.Fatalf("parseSQLExecutionResult: %v", err)
	}
	if !result.IsSelect() {
		t.Fatalf("expected select, got %q", result.Type)
	}
	if result.ExecTime != 12 {
		t.Fatalf("execTime=%d", result.ExecTime)
	}
	if !result.HasMore || result.SQLPage {
		t.Fatalf("hasMore/sqlPage mismatch: %+v", result)
	}
	if len(result.Columns) != 2 || result.Columns[0] != "id" {
		t.Fatalf("columns=%v", result.Columns)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("rows=%d", len(result.Rows))
	}
	if result.Rows[0]["name"] != "a" {
		t.Fatalf("row0=%v", result.Rows[0])
	}
}

func TestParseSQLExecutionResult_Update(t *testing.T) {
	rawJSON := `{
		"type": "update",
		"execTime": 3,
		"data": 5
	}`

	var raw sqlExecData
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	result, err := parseSQLExecutionResult(&raw)
	if err != nil {
		t.Fatalf("parseSQLExecutionResult: %v", err)
	}
	if result.IsSelect() {
		t.Fatal("expected non-select")
	}
	if result.AffectedRows != 5 {
		t.Fatalf("affectedRows=%d", result.AffectedRows)
	}
	if len(result.Rows) != 0 {
		t.Fatalf("rows should be empty, got %d", len(result.Rows))
	}
}

func TestParseSQLExecutionResult_UpdateStringAffected(t *testing.T) {
	rawJSON := `{"type":"delete","execTime":1,"data":"7"}`
	var raw sqlExecData
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	result, err := parseSQLExecutionResult(&raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.AffectedRows != 7 {
		t.Fatalf("affectedRows=%d", result.AffectedRows)
	}
}

func TestParseTableColumns(t *testing.T) {
	raw := `[
		{"column_name":"id","column_type":"bigint","is_nullable":"NO","ordinal_position":2},
		{"columnName":"name","columnType":"varchar(64)","nullable":"YES"}
	]`
	cols, err := parseTableColumns(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parseTableColumns: %v", err)
	}
	if len(cols) != 2 {
		t.Fatalf("len=%d", len(cols))
	}
	if cols[0].ColumnName != "id" || cols[0].OrdinalPosition != 2 {
		t.Fatalf("col0=%+v", cols[0])
	}
	if cols[1].ColumnName != "name" || cols[1].OrdinalPosition != 2 {
		// second row has no ordinal; defaults to index+1 == 2
		if cols[1].ColumnName != "name" || cols[1].OrdinalPosition != 2 {
			t.Fatalf("col1=%+v", cols[1])
		}
	}
}

func TestParseTableMeta(t *testing.T) {
	raw := `{"Create Table":"CREATE TABLE t (id int)","comment":"demo"}`
	createSQL, comment := parseTableMeta(json.RawMessage(raw))
	if createSQL != "CREATE TABLE t (id int)" {
		t.Fatalf("createSQL=%q", createSQL)
	}
	if comment != "demo" {
		t.Fatalf("comment=%q", comment)
	}
}

func TestNormalizePageSize(t *testing.T) {
	if got := normalizePageSize(0); got != defaultSQLPageSize {
		t.Fatalf("default=%d", got)
	}
	if got := normalizePageSize(100); got != 100 {
		t.Fatalf("100=%d", got)
	}
	if got := normalizePageSize(99999); got != maxSQLPageSize {
		t.Fatalf("max=%d", got)
	}
}

func TestFormatDatabaseCandidates(t *testing.T) {
	dbs := []DatabaseInfo{
		{Name: "z0-erp", DatasourceName: "erp_ds"},
		{Name: "pos", DatasourceName: "pos"},
	}
	got := FormatDatabaseCandidates(dbs, 8)
	if got != "z0-erp(erp_ds), pos" {
		t.Fatalf("got=%q", got)
	}
	if FormatDatabaseCandidates(nil, 8) != "(无)" {
		t.Fatal("empty")
	}
}

func TestResolveDatasourceLogic(t *testing.T) {
	dbs := []DatabaseInfo{
		{Name: "erp", DatasourceName: "erp_ds"},
		{Name: "pos", DatasourceName: "pos_ds"},
	}

	find := func(name string) *DatabaseInfo {
		var byName, byDatasource *DatabaseInfo
		for i := range dbs {
			db := &dbs[i]
			if db.Name == name && byName == nil {
				byName = db
			}
			if db.DatasourceName == name && byDatasource == nil {
				byDatasource = db
			}
		}
		if byName != nil {
			return byName
		}
		return byDatasource
	}

	if got := find("erp"); got == nil || got.DatasourceName != "erp_ds" {
		t.Fatalf("by name: %+v", got)
	}
	if got := find("pos_ds"); got == nil || got.Name != "pos" {
		t.Fatalf("by datasource: %+v", got)
	}
	if got := find("missing"); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
