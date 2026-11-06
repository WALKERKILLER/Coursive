package sources

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/vault"
)

var fakeExe string

func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "fakeprovider")
	fakeExe = filepath.Join(dir, "provider")
	if out, err := exec.Command("go", "build", "-o", fakeExe, "./testdata/fakeprovider").CombinedOutput(); err != nil {
		panic(string(out))
	}
	initTimeout, callTimeout = 2*time.Second, 2*time.Second
	backoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond, time.Millisecond, time.Millisecond}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// pluginFolder lays out a source plugin around the fake provider.
func pluginFolder(t *testing.T, exe string) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "bin"), 0o755)
	b, _ := os.ReadFile(fakeExe)
	os.WriteFile(filepath.Join(dir, "bin", "provider"), b, 0o755)
	manifest := `{"name":"fake","version":"1.0.0","coursive":{"id":"fake","title":"Fake","permissions":["source"],
"contributes":{"sources":[{"id":"main","title":"Fake Lectures","protocol":"1","executable":{"` + Platform() + `":"` + exe + `"},"hosts":["media.example"]}]}}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(manifest), 0o644)
	return dir
}

func newRegistry(t *testing.T) (*Registry, string) {
	t.Helper()
	root := t.TempDir()
	vault.KeyPath = filepath.Join(root, "seal.key")
	r := &Registry{Dir: filepath.Join(root, "plugins", "sources"), DataDir: filepath.Join(root, "sources")}
	if err := r.Load(); err != nil {
		t.Fatal(err)
	}
	return r, root
}

func install(t *testing.T, r *Registry, mode string) *Provider {
	t.Helper()
	if _, err := r.Install(pluginFolder(t, "bin/provider")); err != nil {
		t.Fatal(err)
	}
	p, err := r.Provider("fake/main")
	if err != nil {
		t.Fatal(err)
	}
	if mode != "" {
		os.MkdirAll(p.DataDir, 0o700)
		os.WriteFile(filepath.Join(p.DataDir, "mode"), []byte(mode), 0o600)
	}
	t.Cleanup(r.Stop)
	return p
}

type logBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *logBuffer) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func TestSignInWithChallengeRemembersSessionInVault(t *testing.T) {
	logs := &logBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
	defer slog.SetDefault(prev)

	r, root := newRegistry(t)
	install(t, r, "")
	ctx := context.Background()

	a, err := r.SignIn(ctx, "fake/main", map[string]string{"user": "alice", "password": "hunter2-very-secret"})
	if err != nil || a.State != NeedsInput || a.Challenge == nil || a.Challenge.Kind != "image" {
		t.Fatalf("SignIn = %+v, %v", a, err)
	}
	var pe *Error
	if _, err := r.Answer(ctx, "fake/main", "c1", "0000"); !errors.As(err, &pe) || pe.Kind != KindInvalid || !strings.HasPrefix(pe.Message, "无法") {
		t.Fatalf("wrong answer: %v", err)
	}
	if a, err := r.Answer(ctx, "fake/main", "c1", "1234"); err != nil || a.State != SignedIn {
		t.Fatalf("Answer = %+v, %v", a, err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "sources", "fake", "main", "vault.json"))
	if len(raw) == 0 || strings.Contains(string(raw), "cookie-abcdef") {
		t.Fatalf("vault missing or unsealed: %s", raw)
	}
	time.Sleep(100 * time.Millisecond) // let stderr reach the log
	if out := logs.String(); strings.Contains(out, "hunter2-very-secret") || !strings.Contains(out, redactedMarker) {
		t.Fatalf("password not redacted from provider diagnostics:\n%s", out)
	}

	// A new daemon run finds the remembered session.
	r.Stop()
	r2 := &Registry{Dir: r.Dir, DataDir: r.DataDir}
	r2.Load()
	defer r2.Stop()
	r2.Refresh(ctx)
	if len(r2.SignedIn()) != 1 {
		t.Fatalf("session not remembered: %+v", r2.Plugins())
	}
	p, err := r2.ForCourse("")
	if err != nil {
		t.Fatal(err)
	}
	cs, err := p.Courses(ctx)
	if err != nil || len(cs) != 2 || cs[1].Title != "数据结构" {
		t.Fatalf("Courses = %+v, %v", cs, err)
	}
	m, err := p.Media(ctx, "c-1", "l-1")
	if err != nil || len(m.Streams) != 1 || m.Streams[0].Header().Get("Cookie") != "k=v" || !m.Seekable {
		t.Fatalf("Media = %+v, %v", m, err)
	}
	if sl, err := p.Slides(ctx, "c-1", "l-1"); err != nil || len(sl) != 1 || sl[0].TSec != 60 {
		t.Fatalf("Slides = %+v, %v", sl, err)
	}
	if cw, err := p.Courseware(ctx, "c-1", "l-1"); err != nil || cw != nil {
		t.Fatalf("Courseware without the capability = %+v, %v", cw, err)
	}
	if _, err := r2.SignOut(ctx, "fake/main"); err != nil || len(r2.SignedIn()) != 0 {
		t.Fatalf("SignOut: %v", err)
	}
}

func TestAuthRequiredMarksSourceExpired(t *testing.T) {
	r, _ := newRegistry(t)
	p := install(t, r, "")
	var pe *Error
	if err := p.Call(context.Background(), "expire", struct{}{}, nil); !errors.As(err, &pe) || pe.Kind != KindAuthRequired {
		t.Fatalf("err = %v", err)
	}
	if s := p.Snapshot(); s.Auth.State != Expired {
		t.Fatalf("state = %s, want expired", s.Auth.State)
	}
}

func TestCrashingProviderStopsAfterFiveFailuresAndRestarts(t *testing.T) {
	r, _ := newRegistry(t)
	p := install(t, r, "crash")
	ctx := context.Background()
	for i := 0; i < maxFailures; i++ {
		if err := p.Call(ctx, "courses.list", struct{}{}, nil); err == nil {
			t.Fatal("call to a crashing provider succeeded")
		}
	}
	if s := p.Snapshot(); s.Process != Failed || s.Error == "" {
		t.Fatalf("after %d crashes: %+v", maxFailures, s)
	}
	if err := p.Call(ctx, "courses.list", struct{}{}, nil); err == nil || !strings.Contains(err.Error(), "重新启动") {
		t.Fatalf("failed source should ask for a restart: %v", err)
	}
	os.WriteFile(filepath.Join(p.DataDir, "mode"), []byte("ok"), 0o600)
	p.Restart()
	if _, err := p.Courses(ctx); err != nil {
		t.Fatalf("after restart: %v", err)
	}
}

func TestMisbehavingProvidersAreRefused(t *testing.T) {
	for _, tc := range []struct{ mode, want string }{
		{"hang", "没有及时回应"},
		{"flood", "1 MiB"},
		{"oldproto", "协议版本"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			r, _ := newRegistry(t)
			p := install(t, r, tc.mode)
			err := p.Call(context.Background(), "courses.list", struct{}{}, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSlowCallTimesOut(t *testing.T) {
	r, _ := newRegistry(t)
	p := install(t, r, "")
	start := time.Now()
	err := p.Call(context.Background(), "sleep", struct{}{}, nil)
	if err == nil || !strings.Contains(err.Error(), "没有及时回应") || time.Since(start) > 5*time.Second {
		t.Fatalf("err = %v after %v", err, time.Since(start))
	}
}

func TestManifestValidationAndUninstall(t *testing.T) {
	r, root := newRegistry(t)
	if _, err := r.Install(pluginFolder(t, "../outside")); err == nil {
		t.Fatal("executable outside the plugin accepted")
	}
	dir := pluginFolder(t, "bin/provider")
	b, _ := os.ReadFile(filepath.Join(dir, "package.json"))
	os.WriteFile(filepath.Join(dir, "package.json"), bytes.Replace(b, []byte(`"permissions":["source"]`), []byte(`"permissions":[]`), 1), 0o644)
	if _, err := r.Install(dir); err == nil {
		t.Fatal("plugin without the source permission accepted")
	}

	p := install(t, r, "")
	if _, err := r.SignIn(context.Background(), "fake/main", map[string]string{"user": "a", "password": "b"}); err != nil {
		t.Fatal(err)
	}
	r.Answer(context.Background(), "fake/main", "c1", "1234")
	if err := r.Uninstall("fake"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "sources", "fake")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("vault kept after uninstall: %v", err)
	}
	if _, err := r.Provider(p.ID); err == nil {
		t.Fatal("provider kept after uninstall")
	}
}
