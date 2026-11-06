package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNamespaceSealsAndEnforcesLimits(t *testing.T) {
	dir := t.TempDir()
	KeyPath = filepath.Join(dir, "seal.key")
	n := &Namespace{Path: filepath.Join(dir, "p", "vault.json")}

	if err := n.Put("session", "s3cret-cookie"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(n.Path)
	if strings.Contains(string(raw), "s3cret-cookie") {
		t.Fatal("value stored unsealed")
	}
	if info, _ := os.Stat(n.Path); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, want 0600", info.Mode().Perm())
	}
	if v, ok, err := n.Get("session"); err != nil || !ok || v != "s3cret-cookie" {
		t.Fatalf("Get = %q %v %v", v, ok, err)
	}
	if err := n.Put("big", strings.Repeat("x", MaxValueLen+1)); err == nil {
		t.Fatal("oversized value accepted")
	}
	for i := 1; i < MaxKeys; i++ {
		if err := n.Put(strings.Repeat("k", i), "v"); err != nil {
			t.Fatal(err)
		}
	}
	if err := n.Put("one-too-many", "v"); err == nil {
		t.Fatal("key past the limit accepted")
	}
	if err := n.Put("session", "replaced"); err != nil {
		t.Fatalf("replacing an existing key at the limit: %v", err)
	}
	if err := n.Delete("session"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := n.Get("session"); ok {
		t.Fatal("deleted key still present")
	}
}
