package devops

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionFileRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	baseURL := "https://devops.example.com"
	username := "alice"

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(baseURL)
	jar.SetCookies(u, []*http.Cookie{
		{Name: "SESSION", Value: "abc", Path: "/"},
	})

	cookies, err := exportCookies(jar, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	sess := &SessionFile{
		BaseURL:   baseURL,
		Username:  username,
		SavedAt:   now,
		ExpiresAt: now.Add(sessionTTL),
		Cookies:   cookies,
		Authorities: map[string]Authority{
			"1": {Permission: "deployProgram:page", Envs: []string{"dev2"}},
		},
	}
	if err := SaveSessionFile(sess); err != nil {
		t.Fatal(err)
	}

	path, _ := SessionFilePath(baseURL, username)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file missing: %v", err)
	}

	loaded, err := LoadSessionFile(baseURL, username)
	if err != nil || loaded == nil {
		t.Fatalf("load=%v err=%v", loaded, err)
	}
	if loaded.Username != username || len(loaded.Cookies) != 1 || loaded.Cookies[0].Value != "abc" {
		t.Fatalf("loaded=%+v", loaded)
	}
	if loaded.Authorities["1"].Permission != "deployProgram:page" {
		t.Fatalf("authorities=%+v", loaded.Authorities)
	}

	jar2, _ := cookiejar.New(nil)
	if err := applyCookies(jar2, baseURL, loaded.Cookies); err != nil {
		t.Fatal(err)
	}
	got := jar2.Cookies(u)
	if len(got) != 1 || got[0].Value != "abc" {
		t.Fatalf("cookies=%v", got)
	}

	if err := ClearSessionFile(baseURL, username); err != nil {
		t.Fatal(err)
	}
	loaded2, err := LoadSessionFile(baseURL, username)
	if err != nil || loaded2 != nil {
		t.Fatalf("expected nil after clear, got %v err=%v", loaded2, err)
	}
}

func TestLoadSessionFileExpired(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	baseURL := "https://devops.example.com"
	username := "bob"
	path, err := SessionFilePath(baseURL, username)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	sess := &SessionFile{
		BaseURL:   baseURL,
		Username:  username,
		SavedAt:   time.Now().Add(-10 * time.Hour),
		ExpiresAt: time.Now().Add(-2 * time.Hour),
		Cookies:   []persistedCookie{{Name: "S", Value: "x", Path: "/"}},
	}
	if err := SaveSessionFile(sess); err != nil {
		// SaveSessionFile recomputes expires - write manually
		data := []byte(`{"baseURL":"https://devops.example.com","username":"bob","savedAt":"2020-01-01T00:00:00Z","expiresAt":"2020-01-01T01:00:00Z","cookies":[{"name":"S","value":"x","path":"/"}]}`)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	} else {
		_ = sess
		data := []byte(`{"baseURL":"https://devops.example.com","username":"bob","savedAt":"2020-01-01T00:00:00Z","expiresAt":"2020-01-01T01:00:00Z","cookies":[{"name":"S","value":"x","path":"/"}]}`)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	loaded, err := LoadSessionFile(baseURL, username)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Fatal("expected expired session to be ignored")
	}
}

func TestIsSessionExpiredError(t *testing.T) {
	if !IsSessionExpiredError(errString("session expired: HTTP 401")) {
		t.Fatal("expected true")
	}
	if IsSessionExpiredError(errString("API error: code=1, msg=\"not found\"")) {
		t.Fatal("expected false")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
