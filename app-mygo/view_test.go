package main

import (
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/golden"
)

// sampleCourses is a sanitised library: invented titles, teachers and terms,
// shaped like the daemon's JSON.
var sampleCourses = []Course{
	{ID: "c1", Title: "数据结构与算法", Teacher: "示例教师", Term: "2026 秋", College: "示例学院",
		LectureTotal: 32, Progress: 0.42, LastLearnAt: "2026-11-17"},
	{ID: "c2", Title: "编译原理", Teacher: "示例教师乙", Term: "2026 秋",
		LectureTotal: 24, Progress: 0.08},
	{ID: "c3", Title: "操作系统", Teacher: "示例教师丙", Term: "2026 春",
		LectureTotal: 40},
}

func readyApp() *app {
	a := &app{}
	a.state = stateReady
	a.port = 41234
	a.library = "/tmp/coursive-prototype-library"
	a.courses = sampleCourses
	return a
}

// TestLibraryPageGolden draws the stage-0 course page and compares it with the
// image beside the test.
//
// It is opt-in, because the images hold glyphs: a machine without Coursive's
// CJK fallback chain rasterises Chinese text differently, and that is a font
// difference, not a layout one. Run it where the fonts are, with
// COURSIVE_GOLDEN=1 (and MYGO_UPDATE_GOLDEN=1 to rewrite the images).
func TestLibraryPageGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(tc.name, func(t *testing.T) {
			a := readyApp()
			a.dark = tc.dark
			tt := ui.NewTester(a.view, 1180, 760)
			path := "testdata/library-" + tc.name + ".png"
			if os.Getenv("COURSIVE_GOLDEN") != "1" && os.Getenv("MYGO_UPDATE_GOLDEN") == "" {
				t.Skip("set COURSIVE_GOLDEN=1 to compare glyphs; see the comment above")
			}
			if _, err := os.Stat(path); err != nil && os.Getenv("MYGO_UPDATE_GOLDEN") == "" {
				t.Skipf("no golden at %s", path)
			}
			golden.Match(t, tt, path, golden.Tolerance{Channel: 3, PerMille: 5})
		})
	}
}

// TestLibraryPageListsCourses checks the page actually shows the library, not
// just a picture of one.
func TestLibraryPageListsCourses(t *testing.T) {
	a := readyApp()
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("数据结构与算法") || !tt.HasText("编译原理") {
		t.Fatalf("course titles missing from %q", tt.Texts())
	}
	if !tt.HasText("共 3 门课程") {
		t.Fatalf("count missing from %q", tt.Texts())
	}
}

// TestSearchFiltersCourses checks the search field narrows the list.
func TestSearchFiltersCourses(t *testing.T) {
	a := readyApp()
	a.query = "编译"
	if got := a.filtered(); len(got) != 1 || got[0].Title != "编译原理" {
		t.Fatalf("filtered %+v", got)
	}
	a.query = "示例教师丙"
	if got := a.filtered(); len(got) != 1 || got[0].Title != "操作系统" {
		t.Fatalf("filter by teacher %+v", got)
	}
	a.query = "   "
	if got := a.filtered(); len(got) != 3 {
		t.Fatalf("blank query filtered to %d", len(got))
	}
}

// TestFailureOffersRetry is the failure state the checklist asks for: a start
// that failed says why and offers to try again.
func TestFailureOffersRetry(t *testing.T) {
	a := &app{}
	a.state = stateFailed
	a.daemonErr = errString("coursive-daemon not found")
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("本地服务没有启动。") || !tt.HasText("重试") {
		t.Fatalf("failure state texts %q", tt.Texts())
	}
}

// TestStartingStateNamesTheStage checks the window says what it is waiting for.
func TestStartingStateNamesTheStage(t *testing.T) {
	a := &app{}
	a.state = stateStarting
	tt := ui.NewTester(a.view, 1180, 760)
	if !tt.HasText("正在启动本地服务…") {
		t.Fatalf("starting state texts %q", tt.Texts())
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// TestLibraryLayoutMatchesTheTokens checks the geometry the design system
// fixes, which is the part a machine can compare: the page inset, the rail,
// the card's inner padding, and the gaps between cards. It runs everywhere,
// unlike the golden, because it does not depend on a font.
func TestLibraryLayoutMatchesTheTokens(t *testing.T) {
	const width, height = 1180, 760
	a := readyApp()
	tt := ui.NewTester(a.view, width, height)
	w := float32(width)

	titles := []string{"数据结构与算法", "编译原理", "操作系统"}
	boxes := make([]ui.Rect, 0, len(titles))
	for _, title := range titles {
		r, ok := tt.Find(title)
		if !ok {
			t.Fatalf("%q not laid out", title)
		}
		boxes = append(boxes, r)
	}

	wantX := RailWidth + Page + SpaceLG
	if !near(boxes[0].X, wantX) {
		t.Errorf("first title x = %v, want %v (rail %v + page %v + card padding %v)",
			boxes[0].X, wantX, RailWidth, Page, SpaceLG)
	}
	wantW := w - RailWidth - 2*Page - 2*SpaceLG
	if !near(boxes[0].W, wantW) {
		t.Errorf("title width = %v, want %v", boxes[0].W, wantW)
	}

	// Cards stack with the small gap, so the titles are evenly spaced.
	step1 := boxes[1].Y - boxes[0].Y
	step2 := boxes[2].Y - boxes[1].Y
	if !near(step1, step2) {
		t.Errorf("card pitch %v then %v", step1, step2)
	}
	if step1 <= boxes[0].H {
		t.Errorf("cards overlap: pitch %v, title height %v", step1, boxes[0].H)
	}

	// The rail is its own column, so the page starts where it ends.
	if boxes[0].X <= RailWidth {
		t.Errorf("the page starts inside the rail: x = %v", boxes[0].X)
	}
}
