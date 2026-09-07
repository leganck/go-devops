package devops_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leganck/go-devops/devops"
)

func TestLoginAndSessionRetry(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	body := strings.TrimSpace(strings.TrimPrefix(string(pubPEM), "-----BEGIN PUBLIC KEY-----"))
	body = strings.TrimSuffix(body, "-----END PUBLIC KEY-----")
	body = strings.TrimSpace(strings.ReplaceAll(body, "\n", "\\n"))

	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		switch r.URL.Path {
		case "/public/login":
			w.Write([]byte(`<html><script>let rsaPlublic="` + body + `\n"</script></html>`))
		case "/auth/form":
			_ = r.ParseForm()
			if r.Form.Get("username") != "alice" || r.Form.Get("password") == "" {
				w.Write([]byte(`{"code":1,"msg":"bad login"}`))
				return
			}
			if _, err := base64.StdEncoding.DecodeString(r.Form.Get("password")); err != nil {
				w.Write([]byte(`{"code":1,"msg":"not encrypted"}`))
				return
			}
			w.Header().Set("Set-Cookie", "SESSION=tok; Path=/")
			w.Write([]byte(`{"code":0,"data":{"authorities":{"1":{"permission":"deployProgram:page","envs":["dev2"]}}}}`))
		default:
			if cookie, _ := r.Cookie("SESSION"); cookie == nil || cookie.Value != "tok" {
				w.WriteHeader(401)
				w.Write([]byte(`{"code":401,"msg":"未登录"}`))
				return
			}
			w.Write([]byte(`{"code":0,"data":["ok"]}`))
		}
	}))
	defer srv.Close()

	cli, err := devops.New(
		devops.WithBaseURL(srv.URL),
		devops.WithCredentials(devops.Credentials{Username: "alice", Password: "s3cret"}),
		devops.WithSessionStore(devops.NewMemoryStore()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.EnsureSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !cli.HasPermission("deployProgram:page", "dev2") {
		t.Fatal("missing permission")
	}
	var got []string
	if err := cli.DoGet(context.Background(), "/ping", &got); err != nil {
		t.Fatal(err)
	}
}

func TestErrorDoesNotContainPassword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<html><script>let rsaPlublic="nope\n"</script></html>`)
	}))
	defer srv.Close()
	cli, err := devops.New(
		devops.WithBaseURL(srv.URL),
		devops.WithCredentials(devops.Credentials{Username: "alice", Password: "super-secret-pass"}),
		devops.WithSessionStore(devops.NopStore()),
	)
	if err != nil {
		t.Fatal(err)
	}
	err = cli.Login(context.Background())
	if err == nil {
		t.Fatal("expected login error")
	}
	if strings.Contains(err.Error(), "super-secret-pass") {
		t.Fatalf("password leaked: %v", err)
	}
}

func TestAPIEnvelope(t *testing.T) {
	raw := []byte(`{"code":0,"data":{"x":1}}`)
	var resp devops.APIResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	var data map[string]int
	if err := resp.WithData(&data); err != nil || data["x"] != 1 {
		t.Fatalf("%v %v", data, err)
	}
}
