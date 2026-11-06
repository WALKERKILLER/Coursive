package sources

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tgz packs files (name → content) the way pack-sources does; a name
// ending in "@" is a symlink.
func tgz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if strings.HasSuffix(name, "@") {
			tw.WriteHeader(&tar.Header{Name: strings.TrimSuffix(name, "@"), Typeflag: tar.TypeSymlink, Linkname: body})
			continue
		}
		mode := int64(0o644)
		if strings.HasPrefix(name, "bin/") {
			mode = 0o755
		}
		tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// market writes an index listing one artifact for this device.
func market(t *testing.T, r *Registry, root string, art []byte, listed []byte, engine string) *Market {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fake.tgz"), art, 0o644)
	sum := sha256.Sum256(listed)
	ix := map[string]any{"schemaVersion": 1, "plugins": []any{map[string]any{
		"id": "fake", "title": "Fake", "summary": "s", "author": "a", "homepage": "https://example.org", "kinds": []string{"sources"}, "official": true,
		"versions": []any{
			map[string]any{"version": "9.0.0", "engine": "^3.0.0", "artifacts": []any{map[string]any{"platform": Platform(), "url": "x.tgz", "sha256": strings.Repeat("0", 64), "size": 1}}},
			map[string]any{"version": "1.0.0", "engine": engine, "artifacts": []any{map[string]any{"platform": Platform(), "url": "fake.tgz", "sha256": hex.EncodeToString(sum[:]), "size": len(listed)}}},
		},
	}}}
	b, _ := json.Marshal(ix)
	os.WriteFile(filepath.Join(dir, "index.json"), b, 0o644)
	return &Market{Index: filepath.Join(dir, "index.json"), Staging: filepath.Join(root, "plugins", ".staging"), Registry: r}
}

func fakePackage(t *testing.T, version string) map[string]string {
	exe, _ := os.ReadFile(fakeExe)
	return map[string]string{
		"package.json": `{"name":"fake","version":"` + version + `","coursive":{"id":"fake","title":"Fake","permissions":["source"],
"contributes":{"sources":[{"id":"main","title":"Fake Lectures","protocol":"1","executable":{"` + Platform() + `":"bin/provider"},"hosts":["media.example"]}]}}}`,
		"bin/provider": string(exe),
	}
}

func TestMarketInstallsVerifiedArtifactAndUninstalls(t *testing.T) {
	r, root := newRegistry(t)
	defer r.Stop()
	art := tgz(t, fakePackage(t, "1.0.0"))
	m := market(t, r, root, art, art, "^2.0.0")

	ls, err := m.List()
	if err != nil || len(ls) != 1 || ls[0].Version != "1.0.0" || !ls[0].Available || ls[0].Installed != "" {
		t.Fatalf("list = %+v, %v", ls, err)
	}
	st, err := m.Stage(t.Context(), "fake")
	if err != nil {
		t.Fatal(err)
	}
	if st.Plugin.ID != "fake" || !st.Plugin.Available || st.Plugin.Hosts[0] != "media.example" {
		t.Fatalf("staged = %+v", st.Plugin)
	}
	if _, err := m.Install(st.Token); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(r.Dir, "fake", "bin", "provider")); err != nil || fi.Mode().Perm()&0o100 == 0 {
		t.Fatalf("provider not installed executable: %v", err)
	}
	if entries, _ := os.ReadDir(m.Staging); len(entries) != 0 {
		t.Fatalf("staging left behind: %d entries", len(entries))
	}
	if ls, _ := m.List(); ls[0].Installed != "1.0.0" || ls[0].Update {
		t.Fatalf("after install = %+v", ls[0])
	}
	if _, err := m.Install(st.Token); err == nil {
		t.Fatal("a token installs once")
	}
	if err := r.Uninstall("fake"); err != nil {
		t.Fatal(err)
	}
	if ls, _ := m.List(); ls[0].Installed != "" {
		t.Fatalf("after uninstall = %+v", ls[0])
	}
}

func TestMarketRefusesBadArtifacts(t *testing.T) {
	r, root := newRegistry(t)
	defer r.Stop()
	good := tgz(t, fakePackage(t, "1.0.0"))
	for name, tc := range map[string]struct {
		art, listed []byte
		engine      string
		want        string
	}{
		"tampered":      {append(append([]byte{}, good[:len(good)-1]...), good[len(good)-1]^1), good, "^2.0.0", "校验失败"},
		"escape":        {tgz(t, map[string]string{"../evil": "x"}), nil, "^2.0.0", "越出"},
		"link":          {tgz(t, map[string]string{"package.json@": "/etc/passwd"}), nil, "^2.0.0", "链接"},
		"wrong version": {tgz(t, fakePackage(t, "1.0.1")), nil, "^2.0.0", "目录登记的是"},
		"engine":        {good, good, "^1.0.0", "没有适用于本机"},
	} {
		t.Run(name, func(t *testing.T) {
			listed := tc.listed
			if listed == nil {
				listed = tc.art
			}
			m := market(t, r, root, tc.art, listed, tc.engine)
			if _, err := m.Stage(t.Context(), "fake"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
			if entries, _ := os.ReadDir(m.Staging); len(entries) != 0 {
				t.Fatal("refused artifact left staging behind")
			}
			if len(r.Plugins()) != 0 {
				t.Fatal("refused artifact was installed")
			}
		})
	}
}

func TestEngineAndVersionOrder(t *testing.T) {
	for _, tc := range []struct {
		rng, v string
		ok     bool
	}{{"^2.0.0", "2.3.1", true}, {"^2.4.0", "2.3.1", false}, {"^1.0.0", "2.0.0", false}, {">=1.2.0", "2.0.0", true}, {"*", "0.1.0", true}} {
		if engineMatches(tc.rng, tc.v) != tc.ok {
			t.Errorf("engineMatches(%q, %q) != %v", tc.rng, tc.v, tc.ok)
		}
	}
	if compareVersions("1.10.0", "1.9.0") <= 0 || compareVersions("1.0.0-rc.1", "1.0.0") >= 0 {
		t.Fatal("version order")
	}
}
