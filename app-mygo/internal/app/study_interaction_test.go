package app

import (
	"testing"

	"github.com/walkerkiller/coursive/app-mygo/internal/study"
)

// A loop selects a card only when it encloses all of it, as the card less
// the loop being empty in the Dart client.
func TestRectInPolygon(t *testing.T) {
	card := study.RectLTWH(40, 40, 40, 40)
	pts := func(v ...float64) []study.Pt {
		var out []study.Pt
		for i := 0; i < len(v); i += 2 {
			out = append(out, study.Pt{X: v[i], Y: v[i+1]})
		}
		return out
	}
	for _, c := range []struct {
		name string
		loop []study.Pt
		want bool
	}{
		{"around", pts(0, 0, 120, 0, 120, 120, 0, 120), true},
		{"the card itself", pts(40, 40, 80, 40, 80, 80, 40, 80), true},
		{"a corner out", pts(0, 0, 50, 0, 120, 120, 0, 120), false},
		// A U whose notch runs through the card: every corner is inside.
		{"a notch through it", pts(0, 0, 55, 0, 55, 100, 65, 100, 65, 0, 120, 0, 120, 120, 0, 120), false},
		// A spike into the card from below, no corner of the card out.
		{"a spike into it", pts(0, 0, 120, 0, 120, 120, 62, 120, 60, 60, 58, 120, 0, 120), false},
	} {
		if got := rectInPolygon(card, c.loop); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
