package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// hosts stands in for the network: each host's handler answers in
// process, and every request is recorded.
type hosts struct {
	mu   sync.Mutex
	by   map[string]http.HandlerFunc
	seen []*http.Request
}

func (h *hosts) RoundTrip(r *http.Request) (*http.Response, error) {
	h.mu.Lock()
	h.seen = append(h.seen, r)
	fn := h.by[r.URL.Host]
	h.mu.Unlock()
	w := httptest.NewRecorder()
	if fn == nil {
		http.Error(w, "unknown host", http.StatusBadGateway)
	} else {
		fn(w, r)
	}
	return w.Result(), nil
}

func (h *hosts) hostsSeen() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for _, r := range h.seen {
		out = append(out, r.URL.Host)
	}
	return out
}

const (
	testIndex   = "https://raw.githubusercontent.com/o/plugins/index/index.json"
	testRelease = "https://github.com/o/plugins/releases/download/fake-v1.0.0/fake.tgz"
)

// github serves the index, its signature and the artifact at their
// GitHub URLs; a mirror serves them under its prefix, slowly, or
// tampered.
func github(files map[string][]byte) func(prefix string, delay time.Duration, tamper bool) http.HandlerFunc {
	return func(prefix string, delay time.Duration, tamper bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(delay)
			u := strings.TrimPrefix(r.URL.String(), prefix)
			b, ok := files[u]
			if !ok {
				http.NotFound(w, r)
				return
			}
			if tamper {
				b = append([]byte(nil), b...)
				b[len(b)/2] ^= 1
			}
			w.Write(b)
		}
	}
}

func mirrorMarket(t *testing.T) (*Market, *hosts, *MirrorConfig) {
	t.Helper()
	r, root := newRegistry(t)
	t.Cleanup(r.Stop)
	s := newSigner(t)
	art := tgz(t, fakePackage(t, "1.0.0"))
	sum := sha256.Sum256(art)
	ix, _ := json.Marshal(map[string]any{"schemaVersion": 1, "generatedAt": "2026-11-19T00:00:00Z", "plugins": []any{map[string]any{
		"id": "fake", "title": "Fake", "summary": "s", "author": "a", "homepage": "https://example.org", "kinds": []string{"sources"}, "official": true,
		"versions": []any{map[string]any{"version": "1.0.0", "engine": "^2.0.0", "artifacts": []any{
			map[string]any{"platform": Platform(), "url": testRelease, "sha256": hex.EncodeToString(sum[:]), "size": len(art)}}}},
	}}})
	serve := github(map[string][]byte{testIndex: ix, testIndex + ".sig": s.sign(ix, "t"), testRelease: art})
	h := &hosts{by: map[string]http.HandlerFunc{
		// GitHub itself is unreachable.
		"raw.githubusercontent.com": func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "reset", http.StatusBadGateway) },
		"github.com":                func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "reset", http.StatusBadGateway) },
		"fast.test":                 serve("https://fast.test/", 0, false),
		"slow.test":                 serve("https://slow.test/", 150*time.Millisecond, false),
		"evil.test":                 serve("https://evil.test/", 0, true),
	}}
	cfg := &MirrorConfig{Mode: MirrorAuto}
	m := &Market{IndexURL: testIndex, Keys: []string{s.key}, Cache: filepath.Join(root, "index"), Staging: filepath.Join(root, "staging"),
		Registry: r, HTTP: &http.Client{Transport: h},
		Builtin: []Mirror{
			{ID: DirectMirror, Name: "direct", Kinds: allKinds},
			{ID: "evil", Name: "evil", Template: "https://evil.test/{url}", Kinds: allKinds},
			{ID: "slow", Name: "slow", Template: "https://slow.test/{url}", Kinds: allKinds},
			{ID: "fast", Name: "fast", Template: "https://fast.test/{url}", Kinds: allKinds},
		},
		Config: func(context.Context) MirrorConfig { return *cfg }}
	return m, h, cfg
}

func ids(ms []Mirror) string {
	var out []string
	for _, m := range ms {
		out = append(out, m.ID)
	}
	return strings.Join(out, " ")
}

func TestSpeedTestRanksMirrorsAndChecksTheirContent(t *testing.T) {
	m, h, _ := mirrorMarket(t)
	ls, notice, err := m.List(t.Context())
	if err != nil || len(ls) != 1 || notice != "" {
		t.Fatalf("list = %v %q %v", ls, notice, err)
	}
	test := m.lastTest()
	if test == nil {
		t.Fatal("auto ran no speed test")
	}
	ok := map[string]bool{}
	for _, r := range test.Results {
		ok[r.ID] = r.OK
		if r.OK && (r.Speed <= 0 || r.Latency < 0) {
			t.Errorf("%s: %+v", r.ID, r)
		}
	}
	if !ok["fast"] || !ok["slow"] || ok["evil"] || ok[DirectMirror] {
		t.Fatalf("results = %+v", test.Results)
	}
	// Fastest first; a mirror whose index did not verify and the
	// unreachable GitHub last.
	if got := ids(m.routes(t.Context(), KindRaw)); !strings.HasPrefix(got, "fast slow ") {
		t.Fatalf("routes = %s", got)
	}
	if st := m.Mirrors(t.Context()); st.Using != "fast" || st.Mode != MirrorAuto {
		t.Fatalf("state = %+v", st)
	}
	// A restart reads the test back rather than testing again.
	fresh := &Market{IndexURL: m.IndexURL, Cache: m.Cache, Builtin: m.Builtin, Config: m.Config}
	if got := ids(fresh.routes(t.Context(), KindRaw)); !strings.HasPrefix(got, "fast slow ") {
		t.Fatalf("restart routes = %s", got)
	}
	for _, r := range h.seen {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatalf("credentials sent to %s", r.URL.Host)
		}
	}
}

func TestDownloadsFallBackPastTamperedMirrors(t *testing.T) {
	m, h, cfg := mirrorMarket(t)
	// Chosen by the user, the tampering mirror goes first: its index fails
	// the signature and its artifact the hash, and the next route serves.
	cfg.Mode = "evil"
	if got := ids(m.routes(t.Context(), KindRelease)); !strings.HasPrefix(got, "evil ") {
		t.Fatalf("routes = %s", got)
	}
	if _, notice, err := m.List(t.Context()); err != nil || notice != "" {
		t.Fatalf("list: %q %v", notice, err)
	}
	h.mu.Lock()
	h.seen = nil
	h.mu.Unlock()
	st, err := m.Stage(t.Context(), "fake")
	if err != nil {
		t.Fatal(err)
	}
	if st.Plugin.ID != "fake" {
		t.Fatalf("staged %+v", st.Plugin)
	}
	if seen := h.hostsSeen(); len(seen) < 2 || seen[0] != "evil.test" {
		t.Fatalf("artifact routes = %v", seen)
	}
}

func TestMirrorTemplates(t *testing.T) {
	raw, _ := url.Parse(testIndex)
	rel, _ := url.Parse(testRelease)
	for _, c := range []struct {
		m    Mirror
		u    *url.URL
		want string
	}{
		{Mirror{Template: "https://p.test/{url}", Kinds: allKinds}, rel, "https://p.test/" + testRelease},
		{Mirror{Template: "https://p.test/gh/{path}", Kinds: allKinds}, raw, "https://p.test/gh/o/plugins/index/index.json"},
		{Mirror{Template: "https://cdn.test/gh/{owner}/{repo}@{ref}/{file}", Kinds: []string{KindRaw}}, raw, "https://cdn.test/gh/o/plugins@index/index.json"},
		{Mirror{Template: "https://cdn.test/gh/{owner}/{repo}@{ref}/{file}", Kinds: []string{KindRaw}}, rel, ""},
		{Mirror{Kinds: allKinds}, rel, testRelease},
	} {
		if got, _ := c.m.rewrite(c.u); got != c.want {
			t.Errorf("%s on %s = %q, want %q", c.m.Template, c.u, got, c.want)
		}
	}
	for _, bad := range []Mirror{
		{ID: "x", Name: "x", Template: "http://p.test/{url}", Kinds: allKinds},
		{ID: "x", Name: "x", Template: "https://p.test/", Kinds: allKinds},
		{ID: "direct", Name: "x", Template: "https://p.test/{url}", Kinds: allKinds},
		{ID: "x", Name: "x", Template: "https://cdn.test/{owner}/{repo}/{file}", Kinds: allKinds},
		{ID: "x", Name: "", Template: "https://p.test/{url}", Kinds: allKinds},
	} {
		if ValidateMirror(bad) == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if err := ValidateMirror(Mirror{ID: "mine", Name: "Mine", Template: "https://p.test/{url}", Kinds: allKinds}); err != nil {
		t.Error(err)
	}
}

func TestProbeErrorsLeaveOutTheURL(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{&url.Error{Op: "Get", URL: "https://x.test/a", Err: &net.DNSError{Err: "no such host", Name: "x.test", IsNotFound: true}}, "找不到服务器"},
		{&url.Error{Op: "Get", URL: "https://x.test/a", Err: context.DeadlineExceeded}, "超时"},
		{&url.Error{Op: "Get", URL: "https://x.test/a", Err: &net.OpError{Op: "dial", Err: errors.New("refused")}}, "无法连接"},
		{&url.Error{Op: "Get", URL: "https://x.test/a", Err: errors.New("tls: bad certificate")}, "tls: bad certificate"},
		{errors.New("HTTP 502"), "HTTP 502"},
	} {
		if got := probeError(c.err); got != c.want {
			t.Errorf("%v = %q, want %q", c.err, got, c.want)
		}
	}
}
