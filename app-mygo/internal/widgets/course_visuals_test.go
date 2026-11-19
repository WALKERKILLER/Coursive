package widgets

import (
	"math"
	"testing"
)

// The recipes are those CourseArtSpec.of gives in the Flutter client, so a
// course keeps its cover across the two.
func TestCourseArtSpecMatchesDart(t *testing.T) {
	for _, w := range []struct {
		key                   string
		main, partner, accent OrganicShape
		layout, texture, tone int
		rot, jitter           float64
	}{
		{"k0", ShapeGem, ShapeSoftBurst, ShapeCircle, 1, 1, 0, 0.6182, 0.5182},
		{"k9", ShapeSoftBurst, ShapeCircle, ShapeCookie4, 0, 0, 2, 2.1315, 0.9301},
	} {
		s := CourseArtSpec(w.key)
		if s.main != w.main || s.partner != w.partner || s.accent != w.accent || s.layout != w.layout ||
			s.texture != w.texture || s.tone != w.tone || math.Abs(s.rot-w.rot) > 1e-4 || math.Abs(s.jitter-w.jitter) > 1e-4 {
			t.Errorf("%s: got %+v", w.key, *s)
		}
	}
}
