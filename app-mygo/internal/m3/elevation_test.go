package m3

import "testing"

func TestElevationShadowsFollowTheFittedLevels(t *testing.T) {
	if s := elevationShadows(0); s != nil {
		t.Fatalf("level 0 has shadows %v", s)
	}
	for _, l := range elevations {
		if s := elevationShadows(l.level); s[0] != l.shadows[0] || s[1] != l.shadows[1] {
			t.Fatalf("level %v is %v, not the fitted %v", l.level, s, l.shadows)
		}
	}
	half := elevationShadows(.5)
	if want := elevations[0].shadows[0][3] / 2; half[0][3] != want {
		t.Fatalf("level .5 opacity %v, want %v", half[0][3], want)
	}
	if s := elevationShadows(24); s[0] != elevations[len(elevations)-1].shadows[0] {
		t.Fatalf("level 24 is %v, not the highest fitted level", s)
	}
}
