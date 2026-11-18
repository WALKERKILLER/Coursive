package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// AppID is the application id the Flutter client's support directory and
// preferences are named after.
const AppID = "io.github.walkerkiller.coursive"

// SupportDir is path_provider's getApplicationSupportDirectory.
func SupportDir() string {
	if runtime.GOOS == "linux" {
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, AppID)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, AppID)
}

// Store is SharedPreferences: small values the app keeps on the device, in
// the same file and form the Flutter client's shared_preferences_linux
// writes, a JSON object whose keys carry the "flutter." prefix. Keys the app
// does not know stay as they are.
type Store struct {
	mu   sync.Mutex
	path string
	m    map[string]any
}

const prefix = "flutter."

// OpenStore reads the preferences file, or starts empty when there is none.
func OpenStore(path string) *Store {
	s := &Store{path: path, m: map[string]any{}}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &s.m)
	}
	return s
}

// DefaultStorePath is where the preferences file lives.
func DefaultStorePath() string { return filepath.Join(SupportDir(), "shared_preferences.json") }

func (s *Store) String(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[prefix+key].(string)
	return v, ok
}

func (s *Store) Bool(key string) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[prefix+key].(bool)
	return v, ok
}

func (s *Store) Float(key string) (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[prefix+key].(float64)
	return v, ok
}

func (s *Store) Int(key string) (int, bool) {
	f, ok := s.Float(key)
	return int(f), ok
}

func (s *Store) Strings(key string) ([]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.m[prefix+key].([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(l))
	for _, e := range l {
		out = append(out, Str(e))
	}
	return out, true
}

func (s *Store) set(key string, v any) {
	s.mu.Lock()
	if v == nil {
		delete(s.m, prefix+key)
	} else {
		s.m[prefix+key] = v
	}
	b, _ := json.Marshal(s.m)
	path := s.path
	s.mu.Unlock()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		_ = os.Rename(tmp, path)
	}
}

func (s *Store) SetString(key, v string)        { s.set(key, v) }
func (s *Store) SetBool(key string, v bool)     { s.set(key, v) }
func (s *Store) SetFloat(key string, v float64) { s.set(key, v) }
func (s *Store) SetInt(key string, v int)       { s.set(key, float64(v)) }
func (s *Store) SetStrings(key string, v []string) {
	l := make([]any, len(v))
	for i, e := range v {
		l[i] = e
	}
	s.set(key, l)
}
func (s *Store) Remove(key string) { s.set(key, nil) }

// Prefs is the process's preferences store.
var Prefs *Store
