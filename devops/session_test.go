package devops

import (
	"context"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewRequiresBaseURL(t *testing.T) {
	_, err := New(WithCredentials(Credentials{Username: "a", Password: "b"}))
	if err == nil || !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	_, err := New(WithBaseURL("https://devops.example.com"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	baseURL := "https://devops.example.com"
	username := "alice"
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(baseURL)
	jar.SetCookies(u, []*http.Cookie{{Name: "SESSION", Value: "abc", Path: "/"}})
	cookies, err := exportCookies(jar, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	sess := &Session{
		BaseURL:   baseURL,
		Username:  username,
		SavedAt:   now,
		ExpiresAt: now.Add(sessionTTL),
		Cookies:   cookies,
		Authorities: map[string]Authority{
			"1": {Permission: "deployProgram:page", Envs: []string{"dev2"}},
		},
	}
	if err := store.Save(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	path, _ := store.SessionPath(baseURL, username)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background(), baseURL, username)
	if err != nil || loaded == nil {
		t.Fatalf("load=%v err=%v", loaded, err)
	}
	if loaded.Cookies[0].Value != "abc" {
		t.Fatalf("cookie=%v", loaded.Cookies)
	}
	if err := store.Clear(context.Background(), baseURL, username); err != nil {
		t.Fatal(err)
	}
}

func TestFileStoreExpired(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	baseURL := "https://devops.example.com"
	path, _ := store.SessionPath(baseURL, "bob")
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	data := []byte(`{"baseURL":"https://devops.example.com","username":"bob","savedAt":"2020-01-01T00:00:00Z","expiresAt":"2020-01-01T01:00:00Z","cookies":[{"name":"S","value":"x","path":"/"}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(context.Background(), baseURL, "bob")
	if err != nil || loaded != nil {
		t.Fatalf("expected nil expired session got %v %v", loaded, err)
	}
}

func TestSanitizeHidesPassword(t *testing.T) {
	got := sanitizeErrorText("password=supersecret leftover")
	if containsSecret(got, "supersecret") {
		t.Fatalf("leaked: %s", got)
	}
}

func TestIsSessionExpiredError(t *testing.T) {
	if !IsSessionExpiredError(ErrUnauthorized) {
		t.Fatal("expected true")
	}
	if IsSessionExpiredError(ErrNotFound) {
		t.Fatal("expected false")
	}
}
