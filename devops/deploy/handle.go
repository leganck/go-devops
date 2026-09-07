package deploy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/leganck/go-devops/devops"
)

// Handle is a recoverable deploy identifier. Persist it to resume after restart.
type Handle struct {
	HistoryIDs     map[string]string `json:"historyIds"` // serverID -> history.ID
	TaskUUIDs      map[string]string `json:"taskUuids"`
	Env            string            `json:"env"`
	Alias          string            `json:"alias"`
	Version        string            `json:"version"`
	ProgramType    string            `json:"programType"`
	RelativePath   string            `json:"relativePath"`
	ServerIDs      []string          `json:"serverIds"`
	ServerNames    map[string]string `json:"serverNames"`
	IdempotencyKey string            `json:"idempotencyKey"`
	BaseURL        string            `json:"baseURL"`
	CreatedAt      time.Time         `json:"createdAt"`
}

func (h *Handle) clone() *Handle {
	if h == nil {
		return nil
	}
	cp := *h
	cp.HistoryIDs = copyMap(h.HistoryIDs)
	cp.TaskUUIDs = copyMap(h.TaskUUIDs)
	cp.ServerNames = copyMap(h.ServerNames)
	cp.ServerIDs = append([]string(nil), h.ServerIDs...)
	return &cp
}

func copyMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func idempotencyID(baseURL, user, env, alias, version, servers, key string) string {
	raw := strings.Join([]string{
		strings.TrimRight(baseURL, "/"), user, env, alias, version, servers, key,
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:16])
}

// HandleStore remembers StartDeploy results by idempotency key.
type HandleStore interface {
	Get(ctx context.Context, id string) (*Handle, error)
	Put(ctx context.Context, id string, h *Handle) error
}

type MemoryHandleStore struct {
	mu   sync.Mutex
	data map[string]*Handle
}

func NewMemoryHandleStore() *MemoryHandleStore {
	return &MemoryHandleStore{data: map[string]*Handle{}}
}

func (s *MemoryHandleStore) Get(_ context.Context, id string) (*Handle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[id].clone(), nil
}

func (s *MemoryHandleStore) Put(_ context.Context, id string, h *Handle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = map[string]*Handle{}
	}
	s.data[id] = h.clone()
	return nil
}

// FileHandleStore persists handles under dir (default ~/.go-devops/deploys).
type FileHandleStore struct {
	Dir string
	mu  sync.Mutex
}

func NewFileHandleStore(dir string) *FileHandleStore {
	return &FileHandleStore{Dir: dir}
}

func (s *FileHandleStore) dir() (string, error) {
	if s.Dir != "" {
		return s.Dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".go-devops", "deploys"), nil
}

func (s *FileHandleStore) path(id string) (string, error) {
	d, err := s.dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, id+".json"), nil
}

func (s *FileHandleStore) Get(_ context.Context, id string) (*Handle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var h Handle
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

func (s *FileHandleStore) Put(_ context.Context, id string, h *Handle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func handleLooksAlive(st *Status) bool {
	if st == nil || len(st.Servers) == 0 {
		return false
	}
	for _, s := range st.Servers {
		if s.State == StatePending || s.State == StateRunning || s.State == StateSuccess {
			return true
		}
	}
	return false
}

func (c *Client) lookupIdempotent(ctx context.Context, id string) (*Handle, error) {
	if c.store == nil || id == "" {
		return nil, nil
	}
	h, err := c.store.Get(ctx, id)
	if err != nil || h == nil {
		return h, err
	}
	st, err := c.GetDeploy(ctx, h)
	if err != nil {
		if devops.IsSessionExpiredError(err) {
			return nil, err
		}
		return h, nil
	}
	if handleLooksAlive(st) {
		return h, nil
	}
	return h, nil
}
