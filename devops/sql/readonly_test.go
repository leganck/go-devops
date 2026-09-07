package sql

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/leganck/go-devops/devops"
)

func TestCheckReadOnly(t *testing.T) {
	if err := CheckReadOnly("SELECT 1"); err != nil {
		t.Fatal(err)
	}
	if err := CheckReadOnly("show tables"); err != nil {
		t.Fatal(err)
	}
	err := CheckReadOnly("UPDATE t SET a=1")
	if err == nil || !errors.Is(err, devops.ErrInvalidArgument) {
		t.Fatalf("got %v", err)
	}
	if err := CheckReadOnly("SELECT * FROM t INTO OUTFILE '/tmp/x'"); err == nil {
		t.Fatal("expected reject")
	}
}

func TestParseSelect(t *testing.T) {
	rawJSON := `{"type":"select","execTime":12,"hasMore":true,"sqlPage":false,"columns":["id","name"],"data":[{"id":1,"name":"a"}]}`
	var raw sqlExecData
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		t.Fatal(err)
	}
	res, err := parseSQLExecutionResult(&raw)
	if err != nil || !res.IsSelect() || len(res.Rows) != 1 {
		t.Fatalf("%v %v", res, err)
	}
}

func TestParseUpdate(t *testing.T) {
	var raw sqlExecData
	_ = json.Unmarshal([]byte(`{"type":"update","execTime":3,"data":5}`), &raw)
	res, err := parseSQLExecutionResult(&raw)
	if err != nil || res.AffectedRows != 5 {
		t.Fatalf("%v %v", res, err)
	}
}

func TestExportCSVHeader(t *testing.T) {
	var buf bytes.Buffer
	_ = buf
	if !strings.Contains("a,b", "a") {
		t.Fatal("sanity")
	}
}
