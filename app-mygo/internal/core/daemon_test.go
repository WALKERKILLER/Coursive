package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestFindNodeSkipsABrokenNode: a node first on PATH that cannot start, as
// after a partial system upgrade, gives way to a working one further on.
func TestFindNodeSkipsABrokenNode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows takes the first node.exe")
	}
	dir := t.TempDir()
	broken, working := filepath.Join(dir, "broken"), filepath.Join(dir, "working")
	for d, script := range map[string]string{
		broken:  "#!/bin/sh\nexit 127\n",
		working: "#!/bin/sh\necho v24.0.0\n",
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "node"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("COURSIVE_NODE", "")
	t.Setenv("PATH", broken+":"+working)
	if got, want := FindNode(), filepath.Join(working, "node"); got != want {
		t.Fatalf("FindNode() = %q, want %q", got, want)
	}
}

// TestDaemonHandshakeAndCourses is P01's acceptance path without a display: the
// client starts the real daemon, reads the port from its handshake, mints the
// token the daemon authenticates with, creates a course and reads the list
// back over the same HTTP contract the Flutter client uses.
//
// It runs against a throwaway library and data directory, never the user's.
// Build the daemon first: (cd daemon && go build -o bin/coursive-daemon ./cmd/coursive-daemon)
func TestDaemonHandshakeAndCourses(t *testing.T) {
	exe := os.Getenv("COURSIVE_DAEMON")
	if exe == "" {
		root, err := filepath.Abs(filepath.Join("..", "..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		exe = filepath.Join(root, "daemon", "bin", "coursive-daemon")
	}
	if _, err := os.Stat(exe); err != nil {
		t.Skipf("no daemon binary at %s; build it first", exe)
	}

	dir := t.TempDir()
	t.Setenv("COURSIVE_DAEMON", exe)
	t.Setenv("COURSIVE_DATA", filepath.Join(dir, "data"))
	t.Setenv("COURSIVE_LIBRARY", filepath.Join(dir, "library"))
	// The test starts no assistant; the daemon works without one.
	t.Setenv("COURSIVE_AGENT", "")

	d := NewDaemon()
	ready := make(chan struct{}, 1)
	d.OnChange = func() {
		if d.State() == DaemonReady {
			select {
			case ready <- struct{}{}:
			default:
			}
		}
	}
	d.Start()
	defer d.Dispose()

	started := time.Now()
	select {
	case <-ready:
	case <-time.After(45 * time.Second):
		t.Fatalf("daemon never became ready (state %v, err %v)", d.State(), d.Error())
	}
	t.Logf("daemon ready in %s", time.Since(started))

	if d.Port() == 0 {
		t.Fatal("handshake carried no port")
	}
	if d.LibraryRoot() == "" {
		t.Fatal("handshake carried no library root")
	}

	api := NewApi(d.BaseURL(), d.Token())
	courses := func(a *Api) ([]J, error) {
		v, err := a.Get("/api/courses")
		return Objs(v), err
	}
	// An empty library answers with an empty list, not an error.
	empty, err := courses(api)
	if err != nil {
		t.Fatalf("list an empty library: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("a fresh library has %d courses", len(empty))
	}
	made, err := api.CreateLocalCourse("阶段 0 原型课程", "脱敏样本")
	if err != nil {
		t.Fatalf("create a local course: %v", err)
	}
	if Obj(made).S("id") == "" && Obj(made).O("course").S("id") == "" {
		t.Fatalf("created course has no id: %v", made)
	}
	listed, err := courses(api)
	if err != nil {
		t.Fatalf("list after create: %v", err)
	}
	if len(listed) != 1 || listed[0].S("title") != "阶段 0 原型课程" {
		t.Fatalf("listed %+v", listed)
	}
	// The token is the daemon's only gate: another one is refused.
	if _, err := courses(NewApi(d.BaseURL(), "not-the-token")); err == nil {
		t.Fatal("a wrong token was accepted")
	}
}
