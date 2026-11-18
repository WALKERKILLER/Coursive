package assets

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// The assets are the Flutter client's, byte for byte.
func TestParity(t *testing.T) {
	src := filepath.Join("..", "..", "..", "app", "assets")
	if _, err := os.Stat(src); err != nil {
		t.Skip("no Flutter client beside")
	}
	seen := map[string]bool{}
	err := fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		seen[p] = true
		want, err := os.ReadFile(filepath.Join(src, p))
		if err != nil {
			t.Errorf("%s: not in app/assets", p)
			return nil
		}
		if got, _ := Bytes(p); !bytes.Equal(got, want) {
			t.Errorf("%s differs from app/assets", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"brands", "providers"} {
		entries, _ := os.ReadDir(filepath.Join(src, dir))
		for _, e := range entries {
			if !seen[dir+"/"+e.Name()] {
				t.Errorf("%s/%s is missing", dir, e.Name())
			}
		}
	}
	if Image("brands/mineru.png") == nil || Image("brands/notion.svg") == nil {
		t.Error("brand marks do not decode")
	}
}
