package app

import (
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A row the list built glides to its alignment; one it did not, the list
// jumps to before the row goes the rest of the way.
func TestListRevealGlidesToTheAlignment(t *testing.T) {
	var l ui.ListState
	var v listReveal
	tt := ui.NewTester(func(c *ui.Context) {
		list := ui.List(c, &l, 100, v.Rows(c, func(i int) {
			ui.Text(c, "row"+strconv.Itoa(i)).Height(40)
		})).Size(300, 400)
		v.Track(c, list, &l)
	}, 300, 400)
	glide := ui.Animation{Duration: 300 * time.Millisecond, Ease: ui.Linear}
	v.Show(6, .3, &glide, &glide)
	tt.Frame()
	tt.Advance(150 * time.Millisecond)
	r, _ := tt.Find("row6")
	if r.Y <= 108 || r.Y >= 240 {
		t.Fatalf("half way: row6 at %v", r.Y)
	}
	tt.Advance(300 * time.Millisecond)
	if r, _ := tt.Find("row6"); r.Y < 107 || r.Y > 109 {
		t.Fatalf("there: row6 at %v, not 108", r.Y)
	}
	v.Show(80, .3, &glide, nil)
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if r, ok := tt.Find("row80"); !ok || r.Y < 107 || r.Y > 109 {
		t.Fatalf("far: row80 at %v (%v)", r.Y, ok)
	}
}

// End glides a list following its end back to it.
func TestListRevealGlidesToTheEnd(t *testing.T) {
	l := ui.ListState{FollowEnd: true}
	var v listReveal
	tt := ui.NewTester(func(c *ui.Context) {
		list := ui.List(c, &l, 100, v.Rows(c, func(i int) {
			ui.Text(c, "row"+strconv.Itoa(i)).Height(40)
		})).Size(300, 400)
		v.Track(c, list, &l)
	}, 300, 400)
	l.ScrollTo(0, ui.Start)
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	if !tt.HasText("row0") {
		t.Fatalf("at the top: %v", tt.Texts())
	}
	v.End(ui.Animation{Duration: 300 * time.Millisecond, Ease: ui.Linear})
	tt.Frame()
	tt.Advance(150 * time.Millisecond)
	if tt.HasText("row0") || tt.HasText("row99") {
		t.Fatalf("on the way: %v", tt.Texts())
	}
	tt.Advance(300 * time.Millisecond)
	if r, ok := tt.Find("row99"); !ok || r.Y+r.H < 399 {
		t.Fatalf("at the end: row99 at %v (%v)", r, ok)
	}
}
