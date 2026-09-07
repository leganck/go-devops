package devops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	sessionTTL        = 8 * time.Hour
	sessionDirName    = ".go-devops"
	sessionSubDirName = "sessions"
)

// Cookie is a persistable cookie subset (never includes passwords).
type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"httpOnly"`
}

// Session is a persisted login session. JSON is compatible with older files.
type Session struct {
	BaseURL     string               `json:"baseURL"`
	Username    string               `json:"username"`
	SavedAt     time.Time            `json:"savedAt"`
	ExpiresAt   time.Time            `json:"expiresAt"`
	Cookies     []Cookie             `json:"cookies"`
	Authorities map[string]Authority `json:"authorities"`
}

// SessionStore persists sessions. Implementations must not store passwords.
type SessionStore interface {
	Load(ctx context.Context, baseURL, username string) (*Session, error)
	Save(ctx context.Context, sess *Session) error
	Clear(ctx context.Context, baseURL, username string) error
}

// NopStore disables persistence.
func NopStore() SessionStore { return nopStore{} }

type nopStore struct{}

func (nopStore) Load(context.Context, string, string) (*Session, error) { return nil, nil }
func (nopStore) Save(context.Context, *Session) error                   { return nil }
func (nopStore) Clear(context.Context, string, string) error            { return nil }

// MemoryStore is an in-memory session store (tests, short-lived processes).
type MemoryStore struct {
	mu    sync.Mutex
	clock Clock
	data  map[string]*Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{clock: systemClock{}, data: map[string]*Session{}}
}

func (s *MemoryStore) key(baseURL, username string) string {
	return strings.TrimRight(baseURL, "/") + "|" + username
}

func (s *MemoryStore) Load(_ context.Context, baseURL, username string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.data[s.key(baseURL, username)]
	if sess == nil {
		return nil, nil
	}
	now := s.clock.Now()
	if now.After(sess.ExpiresAt) {
		return nil, nil
	}
	cp := *sess
	return &cp, nil
}

func (s *MemoryStore) Save(_ context.Context, sess *Session) error {
	if sess == nil {
		return invalidArg("session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = map[string]*Session{}
	}
	cp := *sess
	s.data[s.key(sess.BaseURL, sess.Username)] = &cp
	return nil
}

func (s *MemoryStore) Clear(_ context.Context, baseURL, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, s.key(baseURL, username))
	return nil
}

// FileStore persists sessions under ~/.go-devops/sessions with 0700/0600,
// atomic writes, and a per-file mutex. Isolated by host+username hash.
type FileStore struct {
	Dir   string
	Clock Clock
	mu    sync.Mutex
	locks sync.Map
}

// NewFileStore uses dir if non-empty, otherwise ~/.go-devops/sessions.
func NewFileStore(dir string) *FileStore {
	return &FileStore{Dir: dir, Clock: systemClock{}}
}

func DefaultSessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, sessionDirName, sessionSubDirName), nil
}

func sessionFileName(baseURL, username string) string {
	sum := sha256.Sum256([]byte(strings.TrimRight(baseURL, "/") + "|" + username))
	return hex.EncodeToString(sum[:8]) + ".json"
}

func (s *FileStore) dir() (string, error) {
	if s.Dir != "" {
		return s.Dir, nil
	}
	return DefaultSessionDir()
}

func (s *FileStore) path(baseURL, username string) (string, error) {
	d, err := s.dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, sessionFileName(baseURL, username)), nil
}

// SessionPath returns the file path for a host+username pair.
func (s *FileStore) SessionPath(baseURL, username string) (string, error) {
	return s.path(baseURL, username)
}

func (s *FileStore) lock(path string) *sync.Mutex {
	v, _ := s.locks.LoadOrStore(path, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func (s *FileStore) Load(_ context.Context, baseURL, username string) (*Session, error) {
	path, err := s.path(baseURL, username)
	if err != nil {
		return nil, err
	}
	mu := s.lock(path)
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	if !strings.EqualFold(sess.Username, username) {
		return nil, nil
	}
	if strings.TrimRight(sess.BaseURL, "/") != strings.TrimRight(baseURL, "/") {
		return nil, nil
	}
	now := time.Now()
	if s.Clock != nil {
		now = s.Clock.Now()
	}
	if now.After(sess.ExpiresAt) {
		return nil, nil
	}
	if len(sess.Cookies) == 0 {
		return nil, nil
	}
	return &sess, nil
}

func (s *FileStore) Save(_ context.Context, sess *Session) error {
	if sess == nil {
		return invalidArg("session is nil")
	}
	path, err := s.path(sess.BaseURL, sess.Username)
	if err != nil {
		return err
	}
	mu := s.lock(path)
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *FileStore) Clear(_ context.Context, baseURL, username string) error {
	path, err := s.path(baseURL, username)
	if err != nil {
		return err
	}
	mu := s.lock(path)
	mu.Lock()
	defer mu.Unlock()
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func exportCookies(jar http.CookieJar, rawURL string) ([]Cookie, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	cookies := jar.Cookies(u)
	out := make([]Cookie, 0, len(cookies))
	for _, c := range cookies {
		if c == nil || c.Name == "" {
			continue
		}
		pc := Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
		}
		if c.Domain != "" {
			pc.Domain = c.Domain
		} else {
			pc.Domain = u.Hostname()
		}
		if pc.Path == "" {
			pc.Path = "/"
		}
		if !c.Expires.IsZero() {
			pc.Expires = c.Expires
		}
		out = append(out, pc)
	}
	return out, nil
}

func applyCookies(jar http.CookieJar, rawURL string, cookies []Cookie) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	list := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		hc := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		}
		if hc.Path == "" {
			hc.Path = "/"
		}
		if !c.Expires.IsZero() {
			hc.Expires = c.Expires
		}
		list = append(list, hc)
	}
	jar.SetCookies(u, list)
	return nil
}

func newEmptyCookieJar() (http.CookieJar, error) {
	return cookiejar.New(nil)
}

func sessionTTLExpiry(now time.Time) time.Time {
	return now.Add(sessionTTL)
}

func dumpHasPassword(sess *Session) error {
	if sess == nil {
		return nil
	}
	raw, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(string(raw)), `"password"`) {
		return fmt.Errorf("session must not contain password")
	}
	return nil
}
