package library

import (
	"context"
	"errors"
	"testing"

	"github.com/walkerkiller/coursive/daemon/internal/pluginfabric"
)

func pluginCard(id string) pluginfabric.CanvasObject {
	return pluginfabric.CanvasObject{
		ID: id, Type: "review-lab.card", SchemaVersion: 1,
		Geometry:          pluginfabric.CanvasGeometry{X: 900, Y: 0, W: 320, H: 200},
		Content:           pluginfabric.CanvasContent{Title: "TCP 三次握手", Text: "SYN、SYN-ACK、ACK", Data: map[string]any{"front": "TCP 三次握手"}},
		SourceAnchors:     []pluginfabric.CanvasAnchor{{Kind: pluginfabric.AnchorLecture, LectureID: "one", StartMs: 60_000, EndMs: 90_000}},
		CreatedBy:         pluginfabric.CanvasOrigin{PluginID: "review-lab", PluginVersion: "0.1.0"},
		Revision:          1,
		InteractionPolicy: pluginfabric.CanvasInteractionPolicy{Mode: pluginfabric.CanvasSelect},
	}
}

// Plugin objects live in the lecture's canvas file, so they survive the
// plugin being disabled or uninstalled; they need version 5.
func TestStudyCanvasKeepsPluginObjects(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	d, err := l.StudyCanvas(ctx, "one")
	if err != nil {
		t.Fatal(err)
	}
	d.PluginObjects = []pluginfabric.CanvasObject{pluginCard("plugin-card-1")}
	if _, err = l.SaveStudyCanvas(ctx, "one", d); !errors.Is(err, ErrStudyInvalid) {
		t.Fatalf("plugin objects below version 5 = %v", err)
	}
	d.Version = 5
	// A knowledge edge may join a plugin object and a card.
	d.Connections = []StudyConnection{{ID: "edge-1", From: "lecture", To: "plugin-card-1", Label: "来源"}}
	saved, err := l.SaveStudyCanvas(ctx, "one", d)
	if err != nil {
		t.Fatal(err)
	}
	again, err := l.StudyCanvas(ctx, "one")
	if err != nil || len(again.PluginObjects) != 1 {
		t.Fatalf("reload = %v %+v", err, again.PluginObjects)
	}
	got := again.PluginObjects[0]
	if got.CreatedBy.PluginID != "review-lab" || got.SourceAnchors[0].StartMs != 60_000 || got.Content.Data["front"] != "TCP 三次握手" {
		t.Fatalf("object = %+v", got)
	}

	bad := saved
	bad.PluginObjects = []pluginfabric.CanvasObject{pluginCard("plugin-card-1")}
	bad.PluginObjects[0].SourceAnchors[0].EndMs = 10 // ends before it starts
	if _, err = l.SaveStudyCanvas(ctx, "one", bad); !errors.Is(err, ErrStudyInvalid) {
		t.Fatalf("invalid anchor = %v", err)
	}
	dup := saved
	dup.PluginObjects = []pluginfabric.CanvasObject{pluginCard("lecture")}
	if _, err = l.SaveStudyCanvas(ctx, "one", dup); !errors.Is(err, ErrStudyInvalid) {
		t.Fatalf("id shared with a card = %v", err)
	}
}
