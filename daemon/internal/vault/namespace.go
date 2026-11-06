package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Limits on one namespace, from the source-provider contract.
const (
	MaxKeys     = 64
	MaxValueLen = 16 << 10
	maxKeyLen   = 128
)

// Namespace keeps one source plugin's values, each sealed, in a JSON file
// readable only by the user.
type Namespace struct {
	Path string
	mu   sync.Mutex
}

type namespaceFile struct {
	Version int               `json:"version"`
	Values  map[string]string `json:"values"`
}

func (n *Namespace) load() (namespaceFile, error) {
	f := namespaceFile{Version: 1, Values: map[string]string{}}
	b, err := os.ReadFile(n.Path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return f, fmt.Errorf("vault %s: %w", filepath.Base(filepath.Dir(n.Path)), err)
	}
	if f.Values == nil {
		f.Values = map[string]string{}
	}
	return f, nil
}

func (n *Namespace) save(f namespaceFile) error {
	if err := os.MkdirAll(filepath.Dir(n.Path), 0o700); err != nil {
		return err
	}
	b, _ := json.Marshal(f)
	tmp := n.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, n.Path)
}

// Get returns the value stored under key, unsealed.
func (n *Namespace) Get(key string) (string, bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	f, err := n.load()
	if err != nil {
		return "", false, err
	}
	sealed, ok := f.Values[key]
	if !ok {
		return "", false, nil
	}
	v, err := Unseal(sealed)
	return v, err == nil, err
}

// Put seals value under key, refusing keys and values past the limits.
func (n *Namespace) Put(key, value string) error {
	if key == "" || len(key) > maxKeyLen {
		return fmt.Errorf("vault key must be 1 to %d bytes", maxKeyLen)
	}
	if len(value) > MaxValueLen {
		return fmt.Errorf("vault value exceeds %d bytes", MaxValueLen)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	f, err := n.load()
	if err != nil {
		return err
	}
	if _, ok := f.Values[key]; !ok && len(f.Values) >= MaxKeys {
		return fmt.Errorf("vault holds at most %d keys", MaxKeys)
	}
	sealed, err := Seal(value)
	if err != nil {
		return err
	}
	f.Values[key] = sealed
	return n.save(f)
}

// Delete removes key; a missing key is not an error.
func (n *Namespace) Delete(key string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	f, err := n.load()
	if err != nil {
		return err
	}
	if _, ok := f.Values[key]; !ok {
		return nil
	}
	delete(f.Values, key)
	return n.save(f)
}

// Values returns every unsealed value, so logs can redact them.
func (n *Namespace) Values() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	f, err := n.load()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(f.Values))
	for _, sealed := range f.Values {
		if v, err := Unseal(sealed); err == nil && v != "" {
			out = append(out, v)
		}
	}
	return out
}
