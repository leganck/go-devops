package logs

import (
	"strings"
	"testing"
	"time"
)

func TestProjectName(t *testing.T) {
	name := BuildProjectName("www_ali", "z0", "smartpos-svc-erp")
	if name != "www_ali-z0-smartpos-svc-erp" {
		t.Fatal(name)
	}
	env, group, alias, err := ParseProjectName(name)
	if err != nil || env != "www_ali" || group != "z0" || alias != "smartpos-svc-erp" {
		t.Fatalf("%s/%s/%s %v", env, group, alias, err)
	}
}

func TestParseLogProjectsHTML(t *testing.T) {
	htmlBody := `<li data-project-name="www_ali-z0-smartpos-svc-erp"></li><li data-project-name="www_ali-zd1-smartpos-svc-erp"></li>`
	got := parseLogProjectsHTML(htmlBody)
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
}

func TestFilterByAlias(t *testing.T) {
	matches := FilterByAlias([]string{
		"www_ali-z0-smartpos-svc-erp",
		"www_ali-zd1-smartpos-svc-erp",
		"dev2-z0-smartpos-svc-erp",
	}, "www_ali", "smartpos-svc-erp")
	if len(matches) != 2 {
		t.Fatalf("%v", matches)
	}
}

func TestFormatTime(t *testing.T) {
	if FormatTime("2024-01-02 03:04:05") != "2024-01-02 03:04:05" {
		t.Fatal("passthrough")
	}
	if FormatTime("1710000000") == "1710000000" {
		t.Fatal("expected formatted")
	}
}

func TestBuildTimeRange(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 30, 0, 0, time.Local)
	got := BuildTimeRange(2*time.Hour, now)
	if !strings.Contains(got, "~") {
		t.Fatal(got)
	}
}
