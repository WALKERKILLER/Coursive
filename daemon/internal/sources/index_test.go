package sources

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// A signature written by coursive-plugins' scripts/minisign.mjs with a
// throwaway key, so both sides agree on the format.
const (
	jsKey = "untrusted comment: minisign public key E391A010F366B048\nRWRIsGbzEKCR40A7hXroWTjXDUzgez24zws9VN/ce5KmM/aKZzaNRmdt\n"
	jsSig = "untrusted comment: signature from the coursive-plugins index key\nRWRIsGbzEKCR44IL2tx61ToAWubo3m2tDWJfZJm09vlrd5cLb8KEl6UAxKssMCXOsTw2DGj3v2f2mZhax+x2E2UUdkNeLuSQTAA=\ntrusted comment: timestamp:1794000000\tfile:index.json\nvxn8tVhzDVcFOQIIIXtVdiSKwFrK25JIh8Ae0Uiw++eU7eKMqeVDytKUBm342+wn5Mxpfv4/YB/XYXXm/s2xCw==\n"
	jsMsg = "{\"schemaVersion\":1}\n"
)

type signer struct {
	priv ed25519.PrivateKey
	id   []byte
	key  string
}

func newSigner(t *testing.T) signer {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(pub)
	id := sum[:8]
	key := base64.StdEncoding.EncodeToString(append(append([]byte("Ed"), id...), pub...))
	return signer{priv, id, key}
}

func (s signer) sign(msg []byte, trusted string) []byte {
	sig := ed25519.Sign(s.priv, msg)
	global := ed25519.Sign(s.priv, append(append([]byte{}, sig...), trusted...))
	return []byte(fmt.Sprintf("untrusted comment: test\n%s\ntrusted comment: %s\n%s\n",
		base64.StdEncoding.EncodeToString(append(append([]byte("Ed"), s.id...), sig...)), trusted,
		base64.StdEncoding.EncodeToString(global)))
}

func TestVerifyMinisign(t *testing.T) {
	if err := verifyMinisign([]byte(jsMsg), []byte(jsSig), []string{jsKey}); err != nil {
		t.Fatalf("minisign.mjs signature: %v", err)
	}
	s, other := newSigner(t), newSigner(t)
	msg := []byte("index")
	good := s.sign(msg, "timestamp:1\tfile:index.json")
	if err := verifyMinisign(msg, good, []string{other.key, s.key}); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		msg, sig []byte
		keys     []string
	}{
		"tampered message": {[]byte("indeX"), good, []string{s.key}},
		"other key":        {msg, good, []string{other.key}},
		"tampered trusted": {msg, []byte(strings.Replace(string(good), "timestamp:1", "timestamp:2", 1)), []string{s.key}},
		"not a signature":  {msg, []byte("{}"), []string{s.key}},
	} {
		if err := verifyMinisign(tc.msg, tc.sig, tc.keys); err == nil {
			t.Errorf("%s: verified", name)
		}
	}
}

// indexServer serves index.json and index.json.sig, which tests replace.
type indexServer struct {
	mu        sync.Mutex
	body, sig []byte
	down      bool
}

func (s *indexServer) set(body, sig []byte, down bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.body, s.sig, s.down = body, sig, down
}

func (s *indexServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.down:
		http.Error(w, "down", http.StatusBadGateway)
	case r.URL.Path == "/index.json":
		w.Write(s.body)
	case r.URL.Path == "/index.json.sig":
		w.Write(s.sig)
	default:
		http.NotFound(w, r)
	}
}

func indexJSON(t *testing.T, generated, url string) []byte {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"schemaVersion": 1, "generatedAt": generated, "plugins": []any{map[string]any{
		"id": "fake", "title": "Fake " + generated, "summary": "s", "author": "a", "homepage": "https://example.org", "kinds": []string{"sources"}, "official": true,
		"versions": []any{map[string]any{"version": "1.0.0", "engine": "^2.0.0", "artifacts": []any{
			map[string]any{"platform": Platform(), "url": url, "sha256": strings.Repeat("0", 64), "size": 1}}}},
	}}})
	return b
}

// list forgets the fetched index, as if indexFresh had passed, and lists.
func list(t *testing.T, m *Market) ([]Listing, string, error) {
	t.Helper()
	m.ixMu.Lock()
	m.ixAt = time.Time{}
	m.ixMu.Unlock()
	return m.List(t.Context())
}

func TestMarketVerifiesTheSignedIndex(t *testing.T) {
	r, root := newRegistry(t)
	defer r.Stop()
	s := newSigner(t)
	srv := &indexServer{}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	cache := filepath.Join(t.TempDir(), "index")
	m := &Market{IndexURL: ts.URL + "/index.json", Keys: []string{s.key}, Cache: cache, Staging: filepath.Join(root, "staging"), Registry: r}
	title := func(ls []Listing) string {
		if len(ls) != 1 {
			return fmt.Sprint(len(ls), " listings")
		}
		return ls[0].Title
	}

	// Nothing verified yet: a failed fetch is an error, a bad signature an
	// empty list with a notice.
	srv.set(nil, nil, true)
	if _, _, err := list(t, m); err == nil || !strings.Contains(err.Error(), "无法读取插件目录") {
		t.Fatalf("down, no cache: %v", err)
	}
	v2 := indexJSON(t, "2026-11-06T10:00:00Z", "https://github.com/o/r/releases/download/fake-v1.0.0/fake.tgz")
	srv.set(v2, newSigner(t).sign(v2, "t"), false)
	if ls, notice, err := list(t, m); err != nil || len(ls) != 0 || notice != NoticeUnverified {
		t.Fatalf("other key, no cache: %v %q %v", ls, notice, err)
	}

	srv.set(v2, s.sign(v2, "t"), false)
	if ls, notice, err := list(t, m); err != nil || title(ls) != "Fake 2026-11-06T10:00:00Z" || notice != "" {
		t.Fatalf("signed: %v %q %v", ls, notice, err)
	}
	// Within indexFresh the fetched index is reused.
	srv.set(nil, nil, true)
	if ls, notice, err := m.List(t.Context()); err != nil || len(ls) != 1 || notice != "" {
		t.Fatalf("reused: %v %q %v", ls, notice, err)
	}

	// The last verified copy stands in, with a notice.
	tampered := []byte(strings.Replace(string(v2), `"Fake `, `"Evil `, 1))
	srv.set(tampered, s.sign(v2, "t"), false)
	if ls, notice, _ := list(t, m); title(ls) != "Fake 2026-11-06T10:00:00Z" || notice != NoticeUnverified {
		t.Fatalf("tampered: %v %q", ls, notice)
	}
	srv.set(nil, nil, true)
	if ls, notice, _ := list(t, m); title(ls) != "Fake 2026-11-06T10:00:00Z" || notice != NoticeStale {
		t.Fatalf("down: %v %q", ls, notice)
	}
	// A daemon restart still has it.
	fresh := &Market{IndexURL: m.IndexURL, Keys: m.Keys, Cache: cache, Registry: r}
	if ls, notice, _ := list(t, fresh); title(ls) != "Fake 2026-11-06T10:00:00Z" || notice != NoticeStale {
		t.Fatalf("restart: %v %q", ls, notice)
	}

	// An older signed index does not replace a newer one.
	v1 := indexJSON(t, "2026-11-05T10:00:00Z", "https://github.com/o/r/releases/download/fake-v1.0.0/fake.tgz")
	srv.set(v1, s.sign(v1, "t"), false)
	if ls, notice, _ := list(t, m); title(ls) != "Fake 2026-11-06T10:00:00Z" || notice != "" {
		t.Fatalf("rollback: %v %q", ls, notice)
	}

	// A copy verified more than indexKeep ago is not shown.
	old := time.Now().Add(-indexKeep - time.Hour)
	os.Chtimes(filepath.Join(cache, "index.json"), old, old)
	srv.set(nil, nil, true)
	if _, _, err := list(t, m); err == nil {
		t.Fatal("expired cache shown")
	}
}

func TestRemoteIndexRefusesRelativeArtifacts(t *testing.T) {
	r, root := newRegistry(t)
	defer r.Stop()
	s := newSigner(t)
	body := indexJSON(t, "2026-11-06T10:00:00Z", "fake.tgz")
	srv := &indexServer{}
	srv.set(body, s.sign(body, "t"), false)
	ts := httptest.NewServer(srv)
	defer ts.Close()
	m := &Market{IndexURL: ts.URL + "/index.json", Keys: []string{s.key}, Staging: filepath.Join(root, "staging"), Registry: r}
	if _, err := m.Stage(t.Context(), "fake"); err == nil || !strings.Contains(err.Error(), "不是 GitHub 发布文件") {
		t.Fatalf("relative artifact from a remote index: %v", err)
	}
}

func TestOfficialKeysDecode(t *testing.T) {
	for _, k := range OfficialKeys {
		if _, ok := minisignKey(k); !ok {
			t.Fatalf("official key %q does not decode", k)
		}
	}
}
