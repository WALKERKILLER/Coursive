package study

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func applySet(base M, set M) M {
	out := obj(Clone(base))
	for key, value := range set {
		parts := strings.Split(key, ".")
		at := out
		for _, p := range parts[:len(parts)-1] {
			at = at[p].(M)
		}
		if value == nil {
			delete(at, parts[len(parts)-1])
		} else {
			at[parts[len(parts)-1]] = value
		}
	}
	return out
}

func TestCanvasObjectSharedFixtures(t *testing.T) {
	b, err := os.ReadFile("../../../daemon/internal/pluginfabric/testdata/canvas/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var f M
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if f["schema"] != "coursive/canvas-object@1" {
		t.Fatalf("fixtures are for %v", f["schema"])
	}
	for _, c := range list(f["cases"]) {
		c := obj(c)
		problems := ValidateCanvasObject(applySet(obj(f["base"]), obj(c["set"])), "")
		if c["code"] == nil {
			if len(problems) > 0 {
				t.Errorf("%v: %v", c["name"], problems)
			}
			continue
		}
		if len(problems) == 0 {
			t.Errorf("%v: no problem", c["name"])
			continue
		}
		if got, want := problems[0].String(), str(c["code"])+" at "+str(c["path"]); got != want {
			t.Errorf("%v: %s, want %s", c["name"], got, want)
		}
	}
}

func pluginObject(id, plugin string, revision int, title string) M {
	return obj(Clone(M{
		"id": id, "type": plugin + ".card", "schemaVersion": 1,
		"geometry":      M{"x": 280, "y": 320, "w": 300, "h": 120},
		"content":       M{"title": title, "text": "先展开定义，再代入。", "data": M{"cards": []any{M{"front": "Q1", "back": "A1"}}}},
		"sourceAnchors": []any{M{"kind": "lecture", "lectureId": "fixture", "startMs": 12000, "endMs": 34000}},
		"createdBy":     M{"pluginId": plugin, "pluginVersion": "0.1.0"},
		"revision":      revision, "interactionPolicy": M{"mode": "select"},
	}))
}

func withObjects(t *testing.T, objects ...M) *Document {
	t.Helper()
	m := NewDocument().JSON()
	m["version"] = 5
	var objs []any
	for _, o := range objects {
		objs = append(objs, o)
	}
	m["plugin_objects"] = objs
	d, err := DocumentFrom(obj(Clone(m)))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestPluginObjectsSaveInFormat5(t *testing.T) {
	if NewDocument().JSON()["version"] != 3 {
		t.Fatal("an empty canvas is not format 3")
	}
	d := withObjects(t, pluginObject("deck-1", "review-lab", 1, "复习卡"))
	saved := obj(Clone(d.JSON()))
	if num(saved["version"], 0) != 5 {
		t.Fatal("not format 5")
	}
	if !reflect.DeepEqual(list(saved["plugin_objects"])[0], pluginObject("deck-1", "review-lab", 1, "复习卡")) {
		t.Fatal("the object changed on save")
	}
	bad := pluginObject("bad", "review-lab", 1, "复习卡")
	bad["revision"] = 0.0
	saved["plugin_objects"] = []any{bad}
	if _, err := DocumentFrom(saved); err == nil {
		t.Fatal("loaded an invalid object")
	}
}

func TestPluginBatchUndoMovesTheRevision(t *testing.T) {
	d := withObjects(t)
	b, err := d.CheckPluginBatch("review-lab", obj(Clone(M{"revision": 0, "objects": []any{
		pluginObject("deck-1", "review-lab", 1, "复习卡"), pluginObject("deck-2", "review-lab", 1, "复习卡")}, "label": "钉到画布"})))
	if err != nil || b.NeedsApproval() {
		t.Fatal(err)
	}
	if err := d.CommitPluginBatch(b); err != nil {
		t.Fatal(err)
	}
	if len(d.PluginObjects) != 2 || d.PluginRevision != 1 || !d.CanUndo() {
		t.Fatal("the batch did not commit as one entry")
	}
	d.Undo()
	if len(d.PluginObjects) != 0 || d.PluginRevision != 2 {
		t.Fatal("undo")
	}
	d.Redo()
	if len(d.PluginObjects) != 2 || d.PluginRevision != 3 {
		t.Fatal("redo")
	}
}

func TestRefusedBatchesCommitNothing(t *testing.T) {
	d := withObjects(t, pluginObject("mine", "review-lab", 1, "复习卡"), pluginObject("theirs", "lab-two", 1, "复习卡"))
	d.Cards = append(d.Cards, &Card{ID: "card-1", Type: "note", Layer: "main"})
	before := Encode(d.JSON())
	o := func(id, plugin string, rev int) M { return pluginObject(id, plugin, rev, "复习卡") }
	broken := o("broken", "review-lab", 1)
	broken["type"] = "card"
	for why, batch := range map[string]M{
		"empty":                     {"revision": 0},
		"stale":                     {"revision": 7, "objects": []any{o("new", "review-lab", 1)}},
		"invalid":                   {"revision": 0, "objects": []any{o("new", "review-lab", 1), broken}},
		"revision mismatch":         {"revision": 0, "objects": []any{o("mine", "review-lab", 3)}},
		"create past 1":             {"revision": 0, "objects": []any{o("new", "review-lab", 2)}},
		"another plugin object":     {"revision": 0, "objects": []any{o("theirs", "lab-two", 2)}},
		"deleting another's object": {"revision": 0, "delete": []any{"theirs"}},
		"writing as another plugin": {"revision": 0, "objects": []any{o("lab", "lab-two", 1)}},
		"taken id":                  {"revision": 0, "objects": []any{o("card-1", "review-lab", 1)}},
		"write and delete":          {"revision": 0, "objects": []any{o("mine", "review-lab", 2)}, "delete": []any{"mine"}},
		"deleting what is gone":     {"revision": 0, "delete": []any{"gone"}},
	} {
		b, err := d.CheckPluginBatch("review-lab", obj(Clone(batch)))
		if err == nil {
			err = d.CommitPluginBatch(b)
		}
		if err == nil {
			t.Errorf("%s: committed", why)
		}
		if Encode(d.JSON()) != before || d.CanUndo() {
			t.Errorf("%s: changed the canvas", why)
		}
	}
}

func TestOverwriteNeedsApprovalAndGoesStale(t *testing.T) {
	d := withObjects(t, pluginObject("mine", "review-lab", 1, "复习卡"))
	d.Connections = append(d.Connections, Connection{"edge", "mine", "mine-2", ""})
	overwrite, err := d.CheckPluginBatch("review-lab", obj(Clone(M{"revision": 0, "objects": []any{pluginObject("mine", "review-lab", 2, "新标题")}})))
	if err != nil || !overwrite.NeedsApproval() {
		t.Fatal("an overwrite needs no approval")
	}
	del, err := d.CheckPluginBatch("review-lab", obj(Clone(M{"revision": 0, "delete": []any{"mine"}})))
	if err != nil || !del.NeedsApproval() {
		t.Fatal("a delete needs no approval")
	}
	if err := d.CommitPluginBatch(overwrite); err != nil || d.PluginObjects[0].Title() != "新标题" {
		t.Fatal("overwrite")
	}
	if d.CommitPluginBatch(del) == nil || len(d.PluginObjects) != 1 {
		t.Fatal("a stale delete committed")
	}
	del, err = d.CheckPluginBatch("review-lab", obj(Clone(M{"revision": d.PluginRevision, "delete": []any{"mine"}})))
	if err != nil || d.CommitPluginBatch(del) != nil {
		t.Fatal(err)
	}
	if len(d.PluginObjects) != 0 || len(d.Connections) != 0 {
		t.Fatal("the delete left the object or its link")
	}
	d.Undo()
	if d.PluginObjects[0].Title() != "新标题" || d.Connections[0].ID != "edge" {
		t.Fatal("undo")
	}
}

func TestPluginFallback(t *testing.T) {
	o := withObjects(t, pluginObject("deck-1", "review-lab", 1, "复习卡")).PluginObjects[0]
	f := o.Fallback()
	if f.Title != "复习卡" || f.PluginID != "review-lab" || f.Source != "课程视频 00:12–00:34" || f.Summary != "先展开定义，再代入。" {
		t.Fatalf("%+v", f)
	}
	if !reflect.DeepEqual(f.Data["cards"], []any{M{"front": "Q1", "back": "A1"}}) {
		t.Fatal("data")
	}
}
