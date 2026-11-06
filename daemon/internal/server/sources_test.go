package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/walkerkiller/coursive/daemon/internal/agent"
	"github.com/walkerkiller/coursive/daemon/internal/events"
	"github.com/walkerkiller/coursive/daemon/internal/library"
	"github.com/walkerkiller/coursive/daemon/internal/lms"
	"github.com/walkerkiller/coursive/daemon/internal/sources"
	"github.com/walkerkiller/coursive/daemon/internal/store"
	"github.com/walkerkiller/coursive/daemon/internal/vault"
)

// A source plugin installed from a folder signs in through a challenge,
// syncs its courses into the library with its id, and plays streams that
// need headers through the relay without the headers reaching the app.
func TestSourcePluginRoutes(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	exe := filepath.Join(tmp, "provider")
	if out, err := exec.Command("go", "build", "-o", exe, "../sources/testdata/fakeprovider").CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	plugin := filepath.Join(tmp, "plugin")
	os.MkdirAll(filepath.Join(plugin, "bin"), 0o755)
	b, _ := os.ReadFile(exe)
	os.WriteFile(filepath.Join(plugin, "bin", "provider"), b, 0o755)
	os.WriteFile(filepath.Join(plugin, "package.json"), []byte(`{"name":"fake","version":"1.0.0","coursive":{"id":"fake","title":"Fake",
"permissions":["source"],"contributes":{"sources":[{"id":"main","title":"Fake Lectures","protocol":"1",
"executable":{"`+sources.Platform()+`":"bin/provider"},"hosts":["media.example"]}]}}}`), 0o644)

	data := filepath.Join(tmp, "data")
	vault.KeyPath = filepath.Join(data, "seal.key")
	reg := &sources.Registry{Dir: filepath.Join(data, "plugins", "sources"), DataDir: filepath.Join(data, "sources")}
	if err := reg.Load(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reg.Stop)
	root := filepath.Join(tmp, "library")
	st, err := store.Open(filepath.Join(tmp, "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := library.Open(root, st)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Token: "test", Sources: reg, St: st, Lib: lib, Bus: events.New(), Agent: &agent.Supervisor{}, Ctx: ctx,
		LMS: &lms.Service{Path: filepath.Join(data, "lms.json")}}
	h := s.Handler()
	call := func(method, path string, body any) (int, string) {
		t.Helper()
		var rd *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		} else {
			rd = bytes.NewReader(nil)
		}
		r := httptest.NewRequest(method, path, rd)
		r.Header.Set("Authorization", "Bearer test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}

	if code, body := call("POST", "/api/sources/install", map[string]string{"path": plugin}); code != 200 || !strings.Contains(body, `"id":"fake"`) {
		t.Fatal(code, body)
	}
	code, body := call("POST", "/api/sources/fake/main/sign-in", map[string]any{"fields": map[string]string{"user": "u", "password": "pw-secret"}})
	if code != 200 || !strings.Contains(body, `"needs_input"`) || !strings.Contains(body, `"c1"`) {
		t.Fatal(code, body)
	}
	if code, body := call("POST", "/api/sources/fake/main/answer", map[string]string{"challenge": "c1", "answer": "0000"}); code != 400 || !strings.Contains(body, `"kind":"invalid"`) {
		t.Fatal(code, body)
	}
	if code, body := call("POST", "/api/sources/fake/main/answer", map[string]string{"challenge": "c1", "answer": "1234"}); code != 200 || !strings.Contains(body, `"signed_in"`) {
		t.Fatal(code, body)
	}

	code, body = call("GET", "/api/courses?sync=1", nil)
	var cs []store.Course
	json.Unmarshal([]byte(body), &cs)
	if code != 200 || len(cs) != 2 || cs[0].Source != "fake/main" || !cs[0].Refreshable {
		t.Fatal(code, body)
	}
	if code, body := call("GET", "/api/courses/c-1/lectures", nil); code != 200 || !strings.Contains(body, `"l-1"`) {
		t.Fatal(code, body)
	}
	code, body = call("GET", "/api/lectures/l-1/media", nil)
	if code != 200 || !strings.Contains(body, `"relayed":true`) || !strings.Contains(body, "/api/lectures/l-1/streams/0") ||
		strings.Contains(body, "k=v") || strings.Contains(body, "media.example") {
		t.Fatal(code, body)
	}
	if code, body := call("GET", "/api/health", nil); code != 200 || !strings.Contains(body, `"signed_in":1`) {
		t.Fatal(code, body)
	}

	if code, body := call("DELETE", "/api/sources/fake", nil); code != 200 {
		t.Fatal(code, body)
	}
	code, body = call("GET", "/api/courses", nil)
	json.Unmarshal([]byte(body), &cs)
	if code != 200 || len(cs) != 2 || cs[0].Refreshable {
		t.Fatal("courses stay readable but not refreshable", code, body)
	}
	if code, body := call("GET", "/api/courses/c-2/lectures", nil); code != 401 || !strings.Contains(body, "来源") {
		t.Fatal(code, body)
	}
}
