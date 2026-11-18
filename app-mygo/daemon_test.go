package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
		root, err := filepath.Abs("..")
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
	// The prototype starts no assistant; the daemon works without one.
	t.Setenv("COURSIVE_AGENT", "")

	d := newDaemon()
	ready := make(chan struct{}, 1)
	d.onChange = func() {
		if state, _, _, _ := d.snapshot(); state == stateReady {
			select {
			case ready <- struct{}{}:
			default:
			}
		}
	}
	d.start()
	defer d.stop()

	started := time.Now()
	select {
	case <-ready:
	case <-time.After(45 * time.Second):
		state, _, _, err := d.snapshot()
		t.Fatalf("daemon never became ready (state %s, err %v)", state, err)
	}
	handshake := time.Since(started)
	t.Logf("daemon ready in %s", handshake)

	state, port, library, err := d.snapshot()
	if state != stateReady || err != nil {
		t.Fatalf("state %s, err %v", state, err)
	}
	if port == 0 {
		t.Fatal("handshake carried no port")
	}
	if library == "" {
		t.Fatal("handshake carried no library root")
	}

	c := newClient(port, d.token)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// An empty library answers with an empty list, not an error.
	empty, err := c.courses(ctx)
	if err != nil {
		t.Fatalf("list an empty library: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("a fresh library has %d courses", len(empty))
	}

	made, err := c.createLocalCourse(ctx, "阶段 0 原型课程", "脱敏样本")
	if err != nil {
		t.Fatalf("create a local course: %v", err)
	}
	if made.ID == "" {
		t.Fatal("created course has no id")
	}

	listed, err := c.courses(ctx)
	if err != nil {
		t.Fatalf("list after create: %v", err)
	}
	if len(listed) != 1 || listed[0].Title != "阶段 0 原型课程" {
		t.Fatalf("listed %+v", listed)
	}

	// The token is the daemon's only gate: another one is refused.
	wrong := newClient(port, "not-the-token")
	if _, err := wrong.courses(ctx); err == nil {
		t.Fatal("a wrong token was accepted")
	}
}
