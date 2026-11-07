package library

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"
)

func TestStudyImageQuarterTurnsPersistAndRejectDowngrade(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	d, e := l.StudyCanvas(ctx, "one")
	if e != nil {
		t.Fatal(e)
	}
	png, e := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j8l8AAAAASUVORK5CYII=")
	if e != nil {
		t.Fatal(e)
	}
	asset, e := l.ImportStudyAsset(ctx, "one", "capture.png", bytes.NewReader(png))
	if e != nil {
		t.Fatal(e)
	}
	d.Objects = append(d.Objects, asset.Objects...)
	d.Version = 3
	d, e = l.SaveStudyCanvas(ctx, "one", d)
	if e != nil {
		t.Fatal(e)
	}
	d.Version = 4
	d.Objects[1].QuarterTurns = 1
	d, e = l.SaveStudyCanvas(ctx, "one", d)
	if e != nil {
		t.Fatal(e)
	}
	read, e := l.StudyCanvas(ctx, "one")
	if e != nil || read.Objects[1].QuarterTurns != 1 {
		t.Fatal("rotation lost", e, read)
	}
	read.Version = 3
	read.Objects[1].QuarterTurns = 0
	if _, e = l.SaveStudyCanvas(ctx, "one", read); !errors.Is(e, ErrStudyInvalid) {
		t.Fatal("downgrade accepted", e)
	}
	read.Version = 4
	read.Objects[1].QuarterTurns = 4
	if _, e = l.SaveStudyCanvas(ctx, "one", read); !errors.Is(e, ErrStudyInvalid) {
		t.Fatal("bad rotation accepted", e)
	}
	read.Objects[1].QuarterTurns = 0
	read.Objects[0].QuarterTurns = 1
	if _, e = l.SaveStudyCanvas(ctx, "one", read); !errors.Is(e, ErrStudyInvalid) {
		t.Fatal("video rotation accepted", e)
	}
}
