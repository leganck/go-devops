package deploy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/leganck/go-devops/devops"
)

func TestFuzzyMatch(t *testing.T) {
	if !FuzzyMatchVersion("4.34", "4.34.0-SNAPSHOT.jar") {
		t.Fatal("expected match")
	}
	if FuzzyMatchVersion("4.34", "4.34.0-plat-SNAPSHOT.jar") {
		t.Fatal("group mismatch")
	}
}

func TestAlignServerGroups(t *testing.T) {
	got := AlignServerGroups([][]Server{
		{{ServerAlias: "a1", ServerID: "1"}},
		{{ServerAlias: "b1", ServerID: "2"}},
	}, []string{"www_ali-z0-app", "www_ali-zd1-app"})
	if got[0].GroupName != "z0" || got[1].GroupName != "zd1" {
		t.Fatalf("%+v", got)
	}
}

func TestStartDeployIdempotent(t *testing.T) {
	starts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/deployProgram/programAliasName":
			writeData(w, []string{"app"})
		case r.URL.Path == "/deployProgram/version":
			writeData(w, []VersionItem{{Version: "1.0.0", RelativePath: "/p/1.0.0"}})
		case r.URL.Path == "/deployProgram/server":
			writeData(w, [][]Server{{{ServerAlias: "s1", ServerID: "sid1"}}})
		case r.URL.Path == "/deployProgram/start":
			starts++
			writeData(w, map[string]any{})
		case strings.HasPrefix(r.URL.Path, "/deployHistory/list"):
			writeData(w, historyResponse{Count: 1, Data: []HistoryItem{{
				ID: "h1", ServerId: "sid1", DeployStatus: StatusInProgress,
				ProgramAliasName: "app", ProgramVersion: "1.0.0", EnvName: "dev2", ServerAlias: "s1",
			}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	core := authedClient(t, srv.URL)
	api := New(core, WithHandleStore(NewMemoryHandleStore()))
	req := StartRequest{Env: "dev2", ProgramAlias: "app", Version: "1.0.0"}
	h1, err := api.StartDeploy(context.Background(), req, "key-1")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := api.StartDeploy(context.Background(), req, "key-1")
	if err != nil {
		t.Fatal(err)
	}
	if starts != 1 {
		t.Fatalf("expected 1 start, got %d", starts)
	}
	if h1.HistoryIDs["sid1"] != "h1" || h2.IdempotencyKey != "key-1" {
		t.Fatalf("%+v %+v", h1, h2)
	}
}

func TestGetDeployCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeData(w, historyResponse{Count: 1, Data: []HistoryItem{{
			ID: "h1", ServerId: "sid1", DeployStatus: StatusInProgress, ServerAlias: "s1",
		}}})
	}))
	defer srv.Close()
	core := authedClient(t, srv.URL)
	api := New(core, WithRetryPolicy(RetryPolicy{PollInterval: time.Millisecond, Timeout: time.Second}))
	h := &Handle{Env: "dev2", Alias: "app", ServerIDs: []string{"sid1"}, HistoryIDs: map[string]string{"sid1": "h1"}}
	st, err := api.GetDeploy(context.Background(), h)
	if err != nil || st.Done {
		t.Fatalf("%+v %v", st, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = api.WaitDeploy(ctx, h)
	if err == nil {
		t.Fatal("expected cancel")
	}
}

func authedClient(t *testing.T, base string) *devops.Client {
	t.Helper()
	cli, err := devops.New(
		devops.WithBaseURL(base),
		devops.WithCredentials(devops.Credentials{Username: "u", Password: "p"}),
		devops.WithSessionStore(devops.NopStore()),
		devops.WithHTTPClient(&http.Client{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	cli.SetAuthoritiesForTest(map[string]devops.Authority{
		"1": {Permission: "deployProgram:page", Envs: []string{"dev2"}},
		"2": {Permission: "deployHistory:list", Envs: []string{"dev2"}},
	})
	return cli
}

func writeData(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(v)
	io.WriteString(w, `{"code":0,"data":`+string(b)+`}`)
}
