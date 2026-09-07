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
