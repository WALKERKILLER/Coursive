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
	{ID: "c1", Title: "数据结构与算法", RawTitle: "数据结构与算法", Teacher: "示例教师", Term: "2026 秋",
		College: "示例学院", LectureTotal: 32, Progress: 0.42, LastLearnAt: "2026-11-17"},
	{ID: "c2", Title: "编译原理", RawTitle: "编译原理", Teacher: "示例教师乙", Term: "2026 秋",
		LectureTotal: 24, Progress: 0.08},
	{ID: "c3", Title: "操作系统", RawTitle: "操作系统", Teacher: "示例教师丙", Term: "2026 秋",
		LectureTotal: 40},
	{ID: "c4", Title: "计算机网络", RawTitle: "计算机网络", Teacher: "示例教师丁", Term: "2026 秋",
		LectureTotal: 28, Progress: 0.75},
	{ID: "c5", Title: "离散数学", RawTitle: "离散数学", Teacher: "示例教师戊", Term: "2026 春",
		LectureTotal: 36},
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

// TestLibraryPageListsCourses checks the page shows the library of the term it
// opened on, not a picture of one.
func TestLibraryPageListsCourses(t *testing.T) {
	a := readyApp()
	tt := ui.NewTester(a.view, 1180, 760)
	for _, title := range []string{"数据结构与算法", "编译原理", "操作系统", "计算机网络"} {
		if !tt.HasText(title) {
			t.Errorf("%q missing from %q", title, tt.Texts())
		}
	}
	// The newest term is 2026 秋, so the spring course is behind its chip.
	if tt.HasText("离散数学") {
		t.Error("a course of another term is on the page")
	}
}

// TestTermChipsFilter checks the chips switch the list.
func TestTermChipsFilter(t *testing.T) {
	a := readyApp()
	if got := a.selectedTerm(); got != "2026 秋" {
		t.Fatalf("opened on term %q", got)
	}
	if got := len(a.filtered()); got != 4 {
		t.Fatalf("the autumn term holds %d courses", got)
	}
	a.term = "2026 春"
	if got := a.filtered(); len(got) != 1 || got[0].Title != "离散数学" {
		t.Fatalf("spring term holds %+v", got)
	}
	a.term = "all"
	if got := len(a.filtered()); got != 5 {
		t.Fatalf("all terms hold %d courses", got)
	}
}

// TestSearchFiltersCourses checks the search field narrows the list.
func TestSearchFiltersCourses(t *testing.T) {
	a := readyApp()
	a.term = "all"
	a.query = "编译"
	if got := a.filtered(); len(got) != 1 || got[0].Title != "编译原理" {
		t.Fatalf("filtered %+v", got)
	}
	a.query = "示例教师丙"
	if got := a.filtered(); len(got) != 1 || got[0].Title != "操作系统" {
		t.Fatalf("filter by teacher %+v", got)
	}
	a.query = "   "
	if got := a.filtered(); len(got) != 5 {
		t.Fatalf("blank query filtered to %d", len(got))
	}
}

// TestGridMatchesTheFlutterLayout checks the geometry the design system fixes:
// the page inset, the column count at a given width, the card pitch and the
// cover height. It runs everywhere, unlike the golden, because it does not
// depend on a font.
func TestGridMatchesTheFlutterLayout(t *testing.T) {
	const width, height = 1180, 760
	a := readyApp()
	tt := ui.NewTester(a.view, width, height)

	// The content is capped at MaxContentWidth and centred.
	contentW := float32(width) - 2*Page
	if contentW > MaxContentWidth {
		contentW = MaxContentWidth
	}
	cols := int(contentW / 280)
	if cols > 4 {
		cols = 4
	}
	if cols != 3 {
		t.Fatalf("1180 wide gives %d columns, want 3", cols)
	}
	cardW := (contentW - SpaceMD*float32(cols-1)) / float32(cols)
	// The rail takes its 80, and the content column is centred in the rest.
	pageLeft := RailWidth + (float32(width)-RailWidth-contentW)/2

	boxes := make([]ui.Rect, 0, len(sampleCourses))
	for _, course := range sampleCourses[:4] {
		r, ok := tt.Find(course.Title)
		if !ok {
			t.Fatalf("%q not laid out", course.Title)
		}
		boxes = append(boxes, r)
	}

	// The first three cards share a row; the fourth starts the next one.
	if !near(boxes[0].Y, boxes[1].Y) || !near(boxes[1].Y, boxes[2].Y) {
		t.Errorf("the first row is not one row: %v %v %v", boxes[0].Y, boxes[1].Y, boxes[2].Y)
	}
	if boxes[3].Y <= boxes[0].Y {
		t.Errorf("the fourth card did not wrap: %v then %v", boxes[0].Y, boxes[3].Y)
	}

	// The card pitch is the card width plus the gap.
	if got, want := boxes[1].X-boxes[0].X, cardW+SpaceMD; !near(got, want) {
		t.Errorf("card pitch %v, want %v", got, want)
	}
	// The title sits inside the card's 16 dp padding, after the page inset.
	if got, want := boxes[0].X, pageLeft+Page+SpaceLG; !near(got, want) {
		t.Errorf("first title x %v, want %v", got, want)
	}
}

// TestCardCoverHeight checks the cover area the card reserves above its text.
func TestCardCoverHeight(t *testing.T) {
	if cardCoverHeight != 108 {
		t.Fatalf("cover height %v, the Flutter card's is 108", cardCoverHeight)
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
