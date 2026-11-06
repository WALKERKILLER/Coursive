package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSearchAndNotes(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.UpsertCourse(ctx, Course{ID: "1", Title: "海洋地质学"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertLectures(ctx, []Lecture{{ID: "10", CourseID: "1", Title: "第一讲"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceSegments(ctx, "10", []Segment{{0, 0, 1000, "今天我们讲板块构造理论"}, {1, 1000, 2000, "洋中脊扩张"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutNote(ctx, Note{ID: "n1", CourseID: "1", LectureID: "10", Title: "板块笔记", Body: "板块构造理论要点"}); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search(ctx, "板块构造", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 hits, got %+v", hits)
	}
	hits, err = s.Search(ctx, "洋中", "1", 10)
	if err != nil || len(hits) != 1 {
		t.Fatalf("short query: %v %+v", err, hits)
	}
	// re-replace keeps FTS consistent
	if err := s.ReplaceSegments(ctx, "10", []Segment{{0, 0, 1000, "全新内容"}}); err != nil {
		t.Fatal(err)
	}
	hits, _ = s.Search(ctx, "板块构造", "", 10)
	if len(hits) != 1 || hits[0].Kind != "note" {
		t.Fatalf("after replace: %+v", hits)
	}
	cs, err := s.Courses(ctx)
	if err != nil || len(cs) != 1 {
		t.Fatal(err, cs)
	}
	ls, err := s.Lectures(ctx, "1")
	if err != nil || len(ls) != 1 || ls[0].Segments != 1 {
		t.Fatal(err, ls)
	}
}

func TestScheduleAndTimeline(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.UpsertCourse(ctx, Course{ID: "1", Title: "计算机视觉"})
	s.UpsertLectures(ctx, []Lecture{{ID: "10", CourseID: "1", Title: "第三讲"}})
	for i, d := range []string{"2026-10-20", "2026-10-15"} {
		if err := s.PutScheduleItem(ctx, ScheduleItem{ID: string(rune('a' + i)), Kind: "homework", Title: "作业", DueDate: d, CourseID: "1", LectureID: "10", Source: "agent"}); err != nil {
			t.Fatal(err)
		}
	}
	items, err := s.Schedule(ctx, ScheduleQuery{From: "2026-10-01"})
	if err != nil || len(items) != 2 || items[0].DueDate != "2026-10-15" || items[0].CourseTitle != "计算机视觉" {
		t.Fatalf("items %+v err %v", items, err)
	}
	items[0].Done = true
	s.PutScheduleItem(ctx, items[0])
	if open, _ := s.Schedule(ctx, ScheduleQuery{Open: true}); len(open) != 1 {
		t.Fatalf("open %d", len(open))
	}
	if err := s.ReplaceTimeline(ctx, "10", []Chapter{{StartMs: 0, EndMs: 1000, Title: "引言"}}); err != nil {
		t.Fatal(err)
	}
	if tl, _ := s.Timeline(ctx, "10"); len(tl) != 1 || tl[0].Title != "引言" {
		t.Fatalf("timeline %+v", tl)
	}
}

func TestSourceColumnsUpgradeAndKeep(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE courses (id TEXT PRIMARY KEY, title TEXT NOT NULL, teacher TEXT NOT NULL DEFAULT '',
 teachers_json TEXT NOT NULL DEFAULT '[]', college TEXT NOT NULL DEFAULT '', term TEXT NOT NULL DEFAULT '', thumb TEXT NOT NULL DEFAULT '',
 start_at INTEGER NOT NULL DEFAULT 0, end_at INTEGER NOT NULL DEFAULT 0, lecture_total INTEGER NOT NULL DEFAULT 0,
 progress REAL NOT NULL DEFAULT 0, last_sub_id TEXT NOT NULL DEFAULT '', last_learn_at TEXT NOT NULL DEFAULT '',
 pinned INTEGER NOT NULL DEFAULT 0, hidden INTEGER NOT NULL DEFAULT 0, alias TEXT NOT NULL DEFAULT '', color TEXT NOT NULL DEFAULT '',
 dir TEXT NOT NULL DEFAULT '', synced_at INTEGER NOT NULL DEFAULT 0);
 INSERT INTO courses(id,title) VALUES('c1','Old');`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	c, err := st.Course(ctx, "c1")
	if err != nil || c.Source != "" || c.Title != "Old" {
		t.Fatalf("legacy row: %+v %v", c, err)
	}
	if err := st.UpsertCourse(ctx, Course{ID: "c1", Source: "demo/rec", URL: "https://example.edu/c1", Title: "New"}); err != nil {
		t.Fatal(err)
	}
	// a later sync without the optional fields keeps them
	if err := st.UpsertCourse(ctx, Course{ID: "c1", Title: "New"}); err != nil {
		t.Fatal(err)
	}
	if c, _ = st.Course(ctx, "c1"); c.Source != "demo/rec" || c.URL != "https://example.edu/c1" {
		t.Fatalf("source kept: %+v", c)
	}
	if err := st.UpsertLectures(ctx, []Lecture{{ID: "l1", CourseID: "c1", Title: "L", URL: "https://example.edu/l1"}}); err != nil {
		t.Fatal(err)
	}
	if l, err := st.Lecture(ctx, "l1"); err != nil || l.URL != "https://example.edu/l1" {
		t.Fatalf("lecture url: %+v %v", l, err)
	}
}
