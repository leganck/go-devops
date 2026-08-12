package devops

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildAndParseProjectName(t *testing.T) {
	name := BuildProjectName("www_ali", "z0", "smartpos-svc-erp")
	if name != "www_ali-z0-smartpos-svc-erp" {
		t.Fatalf("BuildProjectName=%q", name)
	}

	env, group, alias, err := ParseProjectName(name)
	if err != nil {
		t.Fatal(err)
	}
	if env != "www_ali" || group != "z0" || alias != "smartpos-svc-erp" {
		t.Fatalf("parsed=%s/%s/%s", env, group, alias)
	}

	if _, _, _, err := ParseProjectName("a-b"); err == nil {
		t.Fatal("expected error for short projectName")
	}
}

func TestParseLogProjectsHTML(t *testing.T) {
	htmlBody := `
<html><body>
<li data-project-name="www_ali-z0-smartpos-svc-erp"><div class="program-title" data-project-name="www_ali-z0-smartpos-svc-erp">x</div></li>
<li data-project-name="www_ali-zd1-smartpos-svc-erp"></li>
<li data-project-name="www_ali-z0-smartpos-svc-erp"></li>
</body></html>`

	got := parseLogProjectsHTML(htmlBody)
	if len(got) != 2 {
		t.Fatalf("len=%d got=%v", len(got), got)
	}
	if got[0] != "www_ali-z0-smartpos-svc-erp" || got[1] != "www_ali-zd1-smartpos-svc-erp" {
		t.Fatalf("got=%v", got)
	}
}

func TestFilterLogProjectsByAlias(t *testing.T) {
	projects := []string{
		"www_ali-z0-smartpos-svc-erp",
		"www_ali-zd1-smartpos-svc-erp",
		"www_ali-z0-other-app",
		"dev2-z0-smartpos-svc-erp",
	}
	matches := filterLogProjectsByAlias(projects, "www_ali", "smartpos-svc-erp")
	if len(matches) != 2 {
		t.Fatalf("matches=%v", matches)
	}
}

func TestFormatProgramLogTime(t *testing.T) {
	formatted := FormatProgramLogTime("1710000000")
	if !strings.Contains(formatted, "2024") && !strings.Contains(formatted, "2023") {
		// timezone dependent year around this epoch; just ensure not raw
		if formatted == "1710000000" {
			t.Fatalf("expected formatted time, got %q", formatted)
		}
	}
	if FormatProgramLogTime("2024-01-02 03:04:05") != "2024-01-02 03:04:05" {
		t.Fatal("non-unix should pass through")
	}
}

func TestUnescapeLogMessage(t *testing.T) {
	got := UnescapeLogMessage("a &lt;b&gt; &amp; c")
	if got != "a <b> & c" {
		t.Fatalf("got=%q", got)
	}
}

func TestBuildProgramLogTimeRange(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 30, 0, 0, time.Local)
	got := BuildProgramLogTimeRange(2*time.Hour, now)
	wantPrefix := "08-12 08:30:00 ~ 08-12 23:59:59"
	if got != wantPrefix {
		t.Fatalf("got=%q want=%q", got, wantPrefix)
	}
}

func TestParseSinceDuration(t *testing.T) {
	d, err := ParseSinceDuration("2h")
	if err != nil || d != 2*time.Hour {
		t.Fatalf("d=%v err=%v", d, err)
	}
	if _, err := ParseSinceDuration("-1h"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseProgramLogTableJSON(t *testing.T) {
	raw := `{
		"code": "0",
		"msg": "",
		"count": 2,
		"data": [
			{"time":"1710000000","level":"ERROR","thread":"main","location":"A.java","message":"fail &amp; retry","topic":"t1"},
			{"time":"2024-01-01 00:00:00","level":"INFO","thread":"t","location":"B","message":"ok","topic":"t2"}
		]
	}`
	var table programLogTableResponse
	if err := json.Unmarshal([]byte(raw), &table); err != nil {
		t.Fatal(err)
	}
	if table.Count != 2 || len(table.Data) != 2 {
		t.Fatalf("table=%+v", table)
	}
	entry := table.Data[0]
	entry.Time = FormatProgramLogTime(entry.Time)
	entry.Message = UnescapeLogMessage(entry.Message)
	if entry.Message != "fail & retry" {
		t.Fatalf("message=%q", entry.Message)
	}
	if entry.Time == "1710000000" {
		t.Fatal("time should be formatted")
	}
}

func TestNormalizeProgramLogLimit(t *testing.T) {
	if normalizeProgramLogLimit(0) != defaultProgramLogLimit {
		t.Fatal("default")
	}
	if normalizeProgramLogLimit(999) != maxProgramLogLimit {
		t.Fatal("max")
	}
}
