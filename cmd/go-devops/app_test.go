package main

import (
	"io"
	"strings"
	"testing"
)

func TestHostRequired(t *testing.T) {
	app := newApp()
	app.Writer = io.Discard
	app.ErrWriter = io.Discard
	err := app.Run([]string{"go-devops", "envs"})
	if err == nil || !strings.Contains(err.Error(), "DEVOPS_URL") {
		t.Fatalf("expected DEVOPS_URL required, got %v", err)
	}
}

func TestCLIVersion(t *testing.T) {
	var buf strings.Builder
	app := newApp()
	app.Writer = &buf
	app.ErrWriter = io.Discard
	if err := app.Run([]string{"go-devops", "--version"}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "go-devops") || !strings.Contains(got, version) {
		t.Fatalf("expected CLI version output, got %q", got)
	}
}
