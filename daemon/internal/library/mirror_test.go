package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/store"
)

// pendingActivity starts a mirrored write of activity.json and stops before
// the index commits, as a crash would.
func pendingActivity(t *testing.T, l *Library, seconds int64) *mirrorTx {
	t.Helper()
	m := l.beginMirror()
	if _, err := m.replace(filepath.Join(l.Root, "activity.json"), map[string]any{"rows": []store.ActivityRow{{Day: "2026-10-13", Kind: "watch", Seconds: seconds}}}); err != nil {
		t.Fatal(err)
	}
	return m
}

func journals(t *testing.T, l *Library) int {
	t.Helper()
	ents, _ := os.ReadDir(filepath.Join(l.Root, journalDir))
	return len(ents)
}

func TestMirrorFailedCommitRestoresFile(t *testing.T) {
	ctx := context.Background()
	l, _ := openLib(t)
	path := filepath.Join(l.Root, "activity.json")
	if err := l.AddActivity(ctx, "", "2026-10-13", "watch", "", "", 300, 0); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	m := pendingActivity(t, l, 999)
	m.finish(ctx, errors.New("disk I/O error")) // its token never committed
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatalf("previous file not restored:\n%s", after)
	}
	if n := journals(t, l); n != 0 {
		t.Fatalf("%d journal(s) left", n)
	}
}

func TestMirrorRecoveryAfterCrash(t *testing.T) {
	ctx := context.Background()
	l, st := openLib(t)
	path := filepath.Join(l.Root, "activity.json")
	if err := l.AddActivity(ctx, "", "2026-10-13", "watch", "", "", 300, 0); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)

	// Crash before the commit: reopening restores the previous file.
	pendingActivity(t, l, 999)
	if _, err := Open(l.Root, st); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatalf("uncommitted file kept:\n%s", after)
	}

	// Crash after the commit: the new file stays and the token is dropped.
	m := pendingActivity(t, l, 777)
	if _, err := st.DB.Exec(`INSERT INTO mirror_commits(token, at) VALUES(?, ?)`, m.token(), time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	committed, _ := os.ReadFile(path)
	if _, err := Open(l.Root, st); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(committed) {
		t.Fatalf("committed file replaced:\n%s", after)
	}
	if ok, _ := st.MirrorCommitted(ctx, m.token()); ok || journals(t, l) != 0 {
		t.Fatalf("journal or token left after recovery")
	}
}

func TestReindexIgnoresUnfinishedMirror(t *testing.T) {
	ctx := context.Background()
	l, st := openLib(t)
	if err := l.AddActivity(ctx, "", "2026-10-13", "watch", "", "", 300, 0); err != nil {
		t.Fatal(err)
	}
	pendingActivity(t, l, 999) // left by a crash; the index never saw it
	st.DB.Exec(`DELETE FROM activity`)
	if _, err := l.Reindex(ctx); err != nil {
		t.Fatal(err)
	}
	if rows, _ := st.ActivityRows(ctx); len(rows) != 1 || rows[0].Seconds != 300 {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestReindexWaitsForMirroredWrites(t *testing.T) {
	ctx := context.Background()
	l, _ := openLib(t)
	l.mirrorMu.Lock() // a write in progress
	done := make(chan struct{})
	go func() {
		l.Reindex(ctx)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("Reindex ran during a mirrored write")
	case <-time.After(50 * time.Millisecond):
	}
	l.mirrorMu.Unlock()
	<-done
}

func TestAmbiguousMoveRetainsBothFilesAndRecoversOtherWrites(t *testing.T) {
	l, _ := openLib(t)
	ctx := context.Background()
	root, err := os.OpenRoot(l.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.MkdirAll("courses/c/courseware", 0700); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("courses/c/courseware/old.pdf", []byte("classroom"), 0600); err != nil {
		t.Fatal(err)
	}
	m := l.beginMirror()
	m.j.BatchID = NewID()
	if err := m.move(root, "courses/c/courseware/old.pdf", "courses/c/courseware/new.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("courses/c/courseware/old.pdf", []byte("peer edit"), 0600); err != nil {
		t.Fatal(err)
	}
	separate := l.beginMirror()
	if err := separate.replaceRooted(root, "courses/c/temporary.md", nil, []byte("pending")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := l.recoverMirrors(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for name, want := range map[string]string{"courses/c/courseware/old.pdf": "peer edit", "courses/c/courseware/new.pdf": "classroom"} {
		if b, e := root.ReadFile(name); e != nil || string(b) != want {
			t.Fatalf("%s: %q %v", name, b, e)
		}
	}
	if _, err := root.Stat("courses/c/temporary.md"); !os.IsNotExist(err) {
		t.Fatal("unrelated rollback", err)
	}
	if _, err := root.Stat(journalDir + "/" + m.j.BatchID + ".conflict"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.CommitCourseFileBatch(ctx, m.j.BatchID, false); !errors.Is(err, ErrStudyConflict) {
		t.Fatal(err)
	}
}

func TestRootedRollbackAfterPartialLink(t *testing.T) {
	l, _ := openLib(t)
	root, err := os.OpenRoot(l.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	const oldName, newName = "courses/demo/original.md", "courses/demo/copy.md"
	if err := root.MkdirAll("courses/demo", 0700); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile(oldName, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	m := l.beginMirror()
	if err := m.replaceRooted(root, oldName, []byte("before"), []byte("updated")); err != nil {
		t.Fatal(err)
	}
	if err := m.record(journalEntry{Path: oldName, MoveTo: newName}); err != nil {
		t.Fatal(err)
	}
	if err := root.Link(oldName, newName); err != nil {
		t.Fatal(err)
	}
	if err := l.recoverMirrors(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b, err := root.ReadFile(oldName); err != nil || string(b) != "before" {
		t.Fatal(string(b), err)
	}
	if _, err := root.Stat(newName); !os.IsNotExist(err) {
		t.Fatal("duplicate link survived", err)
	}
}
