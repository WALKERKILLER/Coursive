package library

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/walkerkiller/coursive/daemon/internal/store"
)

func TestCourseFilesOrganizeUndoAndReindex(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	if err := l.Store.UpsertLectures(ctx, []store.Lecture{{ID: "second", CourseID: "os", Title: "第二节"}}); err != nil {
		t.Fatal(err)
	}
	course, _, dir, err := l.EnsureLecture(ctx, "second")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.WriteLectureJSON(ctx, "second"); err != nil {
		t.Fatal(err)
	}
	asset, err := l.ImportCourseware(ctx, "one", "旧 讲义.pdf", strings.NewReader("original courseware"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	parsed := ParsedDir(asset.Path) + "/full.md"
	if err := os.MkdirAll(filepath.Dir(l.Abs(parsed)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(l.Abs(parsed), []byte("# 讲义\n这里讲的是虚拟地址。"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.SetParse(ctx, asset, &ParseInfo{State: "done", MD: parsed, Pages: 1}); err != nil {
		t.Fatal(err)
	}
	note, err := l.SaveNote(ctx, store.Note{CourseID: "os", Title: "我的笔记", Body: "可替换的笔记正文。", Source: "user"})
	if err != nil {
		t.Fatal(err)
	}
	oldNote, err := os.ReadFile(l.Abs(note.Path))
	if err != nil {
		t.Fatal(err)
	}
	inbox := course.Dir + "/" + courseInbox
	if err := os.Mkdir(l.Abs(inbox), 0o700); err != nil {
		t.Fatal(err)
	}
	intake := inbox + "/补充.pdf"
	if err := os.WriteFile(l.Abs(intake), []byte("supplement"), 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := l.ReadCourseFile(ctx, "os", asset.Path, 0)
	if err != nil || read.(map[string]any)["version"] != materialHash([]byte("original courseware")) {
		t.Fatalf("read = %v, %v", read, err)
	}
	source, err := l.ReadStudyCourseSource(ctx, "one", StudySourceRead{Source: StudyCourseSource{Path: asset.Path}})
	if err != nil || !strings.Contains(source.Text, "虚拟地址") {
		t.Fatalf("source = %+v, %v", source, err)
	}
	batch, err := l.PreviewCourseFiles(ctx, "os", []CourseFileEdit{
		{Op: "move", Path: asset.Path, Version: materialHash([]byte("original courseware")), LectureID: "second", Name: "新讲义.pdf"},
		{Op: "edit_markdown", Path: note.Path, Version: materialHash(oldNote), Old: "可替换", New: "已经修改"},
		{Op: "import", Path: intake, Version: materialHash([]byte("supplement")), LectureID: "one"},
	})
	if err != nil || batch.State != "pending" || len(batch.Lines) != 3 {
		t.Fatalf("preview = %+v, %v", batch, err)
	}
	if _, err := os.Stat(l.Abs(asset.Path)); err != nil {
		t.Fatal("preview changed the source", err)
	}
	moved := dir + "/courseware/新讲义.pdf"
	if got, err := l.CommitCourseFileBatch(ctx, batch.ID, false); err != nil || got.State != "applied" {
		t.Fatalf("apply = %+v, %v", got, err)
	}
	if _, err := os.Stat(l.Abs(asset.Path)); !os.IsNotExist(err) {
		t.Fatal("old file was not moved", err)
	}
	if result, err := l.Store.Note(ctx, note.ID); err != nil || result.Body != "已经修改的笔记正文。" || result.Source != "user" {
		t.Fatalf("updated note = %+v, %v", result, err)
	}
	if _, err := l.CommitCourseFileBatch(ctx, batch.ID, false); err != nil {
		t.Fatal("apply must be idempotent", err)
	}
	checkAlias := func(lib *Library) {
		t.Helper()
		result, err := lib.ResolveStudyCourseSource(ctx, "one", StudySourceRead{Source: source})
		if err != nil || result.Path != moved || result.Text != source.Text {
			t.Fatalf("source link = %+v, %v", result, err)
		}
		f, err := lib.OpenFormerCourseware(ctx, parsed)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(f)
		f.Close()
		if err != nil || !strings.Contains(string(b), "虚拟地址") {
			t.Fatalf("parsed old path = %s, %v", b, err)
		}
	}
	checkAlias(l)
	reindexed, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reindexed.Close()
	second, err := Open(l.Root, reindexed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Reindex(ctx); err != nil {
		t.Fatal(err)
	}
	checkAlias(second)
	if got, err := l.CommitCourseFileBatch(ctx, batch.ID, true); err != nil || got.State != "undone" {
		t.Fatalf("undo = %+v, %v", got, err)
	}
	restored, err := os.ReadFile(l.Abs(note.Path))
	if err != nil || string(restored) != string(oldNote) {
		t.Fatal("undo did not restore the note bytes", err)
	}
	if b, err := os.ReadFile(l.Abs(intake)); err != nil || string(b) != "supplement" {
		t.Fatal("undo did not return the import to the inbox", err)
	}
	if active, ok := l.FormerCourseware(ctx, moved); !ok || active.Path != asset.Path {
		t.Fatal("references created after the move stopped resolving", active)
	}
	if _, err := second.Reindex(ctx); err != nil {
		t.Fatal(err)
	}
	if assets, err := second.Courseware(ctx, "one"); err != nil || len(assets) != 1 || assets[0].Path != asset.Path {
		t.Fatal("peer kept a stale asset after undo", assets, err)
	}
	archive, err := l.PreviewCourseFiles(ctx, "os", []CourseFileEdit{{Op: "archive", Path: asset.Path, Version: materialHash([]byte("original courseware"))}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.CommitCourseFileBatch(ctx, archive.ID, false); err != nil {
		t.Fatal(err)
	}
	if active, ok := l.FormerCourseware(ctx, asset.Path); !ok || !strings.Contains(active.Path, "/归档/") {
		t.Fatal("archive did not preserve the old reference", active)
	}
}

func TestCourseFileBatchRefusesStaleAndOutOfScopeChanges(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	asset, err := l.ImportCourseware(ctx, "one", "课件.pdf", strings.NewReader("initial"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	change := CourseFileEdit{Op: "rename", Path: asset.Path, Name: "改名.pdf", Version: materialHash([]byte("initial"))}
	err = l.Store.UpsertCourse(ctx, store.Course{ID: "other", Title: "另一门课"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.PreviewCourseFiles(ctx, "other", []CourseFileEdit{change}); !errors.Is(err, ErrStudyInvalid) {
		t.Fatal("course escape", err)
	}
	lec, _ := l.Store.Lecture(ctx, "one")
	bad := CourseFileEdit{Op: "edit_markdown", Path: lec.Dir + "/transcript.md", Version: "wrong", Old: "abc", New: "def"}
	if _, err := l.PreviewCourseFiles(ctx, "os", []CourseFileEdit{change, bad}); !errors.Is(err, ErrStudyConflict) {
		t.Fatal("invalid batch was prepared", err)
	}
	batch, err := l.PreviewCourseFiles(ctx, "os", []CourseFileEdit{change})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(l.Abs(asset.Path), []byte("externally updated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.CommitCourseFileBatch(ctx, batch.ID, false); !errors.Is(err, ErrStudyConflict) {
		t.Fatal("stale preview applied", err)
	}
	change.Version = materialHash([]byte("externally updated"))
	batch, err = l.PreviewCourseFiles(ctx, "os", []CourseFileEdit{change})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.CommitCourseFileBatch(ctx, batch.ID, false); err != nil {
		t.Fatal(err)
	}
	active, _ := l.FormerCourseware(ctx, asset.Path)
	if err := os.WriteFile(l.Abs(active.Path), []byte("new edit after move"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.CommitCourseFileBatch(ctx, batch.ID, true); !errors.Is(err, ErrStudyConflict) {
		t.Fatal("undo overwrote the newer edit", err)
	}
	if b, err := os.ReadFile(l.Abs(active.Path)); err != nil || string(b) != "new edit after move" {
		t.Fatal("new content was lost", err)
	}
}
