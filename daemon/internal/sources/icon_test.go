package sources

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withIcon rewrites a plugin folder's manifest to name icon.
func withIcon(t *testing.T, dir, icon string) {
	t.Helper()
	p := filepath.Join(dir, "package.json")
	b, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Replace(string(b), `"title":"Fake",`, `"title":"Fake","icon":"`+icon+`",`, 1)), 0o644)
}

func TestPackageIconBecomesDataURL(t *testing.T) {
	r, _ := newRegistry(t)
	defer r.Stop()
	dir := pluginFolder(t, "bin/provider")
	os.WriteFile(filepath.Join(dir, "icon.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644)
	withIcon(t, dir, "icon.svg")
	pre, err := r.Preview(dir)
	if err != nil || !strings.HasPrefix(pre.Icon, "data:image/svg+xml;base64,") {
		t.Fatalf("preview icon = %.40q, %v", pre.Icon, err)
	}
	in, err := r.Install(dir)
	if err != nil || in.Icon != pre.Icon {
		t.Fatalf("installed icon differs: %v", err)
	}
	r.Load()
	if r.Plugins()[0].Icon != pre.Icon {
		t.Fatal("icon lost after reload")
	}
}

func TestBadIconsAreRefused(t *testing.T) {
	r, _ := newRegistry(t)
	defer r.Stop()
	for name, icon := range map[string]string{
		"escape":  "../icon.png",
		"absent":  "missing.png",
		"type":    "icon.gif",
		"too big": "big.png",
	} {
		t.Run(name, func(t *testing.T) {
			dir := pluginFolder(t, "bin/provider")
			os.WriteFile(filepath.Join(dir, "icon.gif"), []byte("GIF89a"), 0o644)
			os.WriteFile(filepath.Join(dir, "big.png"), make([]byte, MaxIcon+1), 0o644)
			withIcon(t, dir, icon)
			if _, err := r.Install(dir); err == nil || !strings.Contains(err.Error(), "图标") {
				t.Fatalf("want icon error, got %v", err)
			}
		})
	}
	dir := pluginFolder(t, "bin/provider")
	withIcon(t, dir, "school")
	if in, err := r.Install(dir); err != nil || in.Icon != "school" {
		t.Fatalf("symbol icon = %q, %v", in.Icon, err)
	}
}

func TestListedIcon(t *testing.T) {
	for icon, ok := range map[string]bool{
		"school":                                                  true,
		"data:image/png;base64,iVBORw0=":                          true,
		"data:image/svg+xml;base64,PHN2Zz4=":                      true,
		"https://example.org/icon.png":                            false,
		"data:text/html;base64,PGI+":                              false,
		"data:image/png;base64," + strings.Repeat("A", MaxIcon*2): false,
	} {
		if (listedIcon(icon) != "") != ok {
			t.Errorf("listedIcon(%.40q) kept = %v", icon, !ok)
		}
	}
}
