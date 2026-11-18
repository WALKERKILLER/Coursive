package widgets

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
)

// TestShapePathFitsItsRect is the invariant the covers depend on: a shape is
// scaled into the box it is given, never past it.
func TestShapePathFitsItsRect(t *testing.T) {
	rect := ui.Rect{X: 10, Y: 20, W: 200, H: 120}
	for s := ShapeCircle; s <= ShapeGhost; s++ {
		path := ShapePath(ShapeRadii(s), rect, 0.3)
		b := path.Bounds()
		if b.X < rect.X-0.5 || b.Y < rect.Y-0.5 ||
			b.X+b.W > rect.X+rect.W+0.5 || b.Y+b.H > rect.Y+rect.H+0.5 {
			t.Errorf("shape %d bounds %+v escape %+v", s, b, rect)
		}
		// The shape fills one axis: it is scaled to fit, not shrunk.
		if b.W < rect.W-1 && b.H < rect.H-1 {
			t.Errorf("shape %d bounds %+v do not fill %+v", s, b, rect)
		}
	}
}

// TestShapeRadiiAreNormalised checks every shape has a unit maximum radius.
func TestShapeRadiiAreNormalised(t *testing.T) {
	for s := ShapeCircle; s <= ShapeGhost; s++ {
		r := ShapeRadii(s)
		if len(r) != shapeSamples {
			t.Fatalf("shape %d has %d samples", s, len(r))
		}
		max := 0.0
		for _, v := range r {
			if v < 0 || math.IsNaN(v) {
				t.Fatalf("shape %d radius %v", s, v)
			}
			if v > max {
				max = v
			}
		}
		if math.Abs(max-1) > 1e-9 {
			t.Errorf("shape %d max radius %v", s, max)
		}
	}
}

// TestCourseArtSpecIsDeterministic is what makes a cover a course's signature:
// the same id always draws the same picture, and different ids do not.
func TestCourseArtSpecIsDeterministic(t *testing.T) {
	a := CourseArtSpec("course-a")
	b := CourseArtSpec("course-a")
	if a != b {
		t.Fatal("the same id gave two recipes")
	}
	before := *a
	seen := map[OrganicShape]bool{}
	for _, id := range []string{"c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8"} {
		spec := CourseArtSpec(id)
		seen[spec.main] = true
	}
	if len(seen) < 2 {
		t.Fatalf("eight courses share one hero shape: %v", seen)
	}
	// The recipe is stable across a fresh cache, so a restart redraws it.
	delete(artCache, "course-a")
	after := *CourseArtSpec("course-a")
	if after.main != before.main || after.layout != before.layout ||
		after.rot != before.rot || after.tone != before.tone ||
		len(after.cluster) != len(before.cluster) {
		t.Fatalf("the recipe changed after a cache reset: %+v then %+v", before, after)
	}
}

// TestCourseSchemeSeedsFromTheTitle checks two courses with different titles
// get different palettes, and that an explicit colour wins.
func TestCourseSchemeSeedsFromTheTitle(t *testing.T) {
	one := CourseScheme(core.Course{J: core.J{"title": "数据结构与算法"}})
	two := CourseScheme(core.Course{J: core.J{"title": "编译原理"}})
	if one.Primary == two.Primary {
		t.Error("two titles share a primary colour")
	}
	pinned := CourseScheme(core.Course{J: core.J{"title": "x", "color": "#00ADD8"}})
	if pinned.Primary == one.Primary {
		t.Error("an explicit colour did not change the palette")
	}
}
