package study

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// CanvasSchemaVersion is the object shape this build reads and writes
// (pluginfabric.CanvasSchemaVersion).
const CanvasSchemaVersion = 1

// The limits of pluginfabric.CanvasObject, kept equal so an object the
// canvas commits is an object the daemon saves.
const (
	maxText         = 800000
	maxData         = 256 << 10
	maxAnchors      = 32
	maxCoordinate   = 1e9
	minSize         = 24.0
	maxSize         = 100000.0
	maxLectureMs    = 7 * 24 * 3600 * 1000
	maxPages        = 5000
	maxStrokes      = 500
	fallbackSummary = 280
	// CanvasMaxBatch is how many objects one plugin batch may write or
	// delete.
	CanvasMaxBatch = 200
)

var (
	idRE      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,99}$`)
	typeRE    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}(?:\.[a-z][a-z0-9-]{0,31})+$`)
	pluginRE  = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)
	versionRE = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)
)

// CanvasProblem is one rule a canvas object breaks, with the code and path
// of the daemon's pluginfabric.Problem.
type CanvasProblem struct{ Code, Path string }

func (p CanvasProblem) String() string { return p.Code + " at " + p.Path }

func isNumber(v any) (float64, bool) {
	switch v := v.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}

// ValidateCanvasObject lists every problem in a raw CanvasObject, in the
// order the daemon reports them (pluginfabric.CanvasObject.Validate).
func ValidateCanvasObject(raw any, at string) []CanvasProblem {
	var out []CanvasProblem
	add := func(code, name string) {
		if at != "" {
			name = at + "." + name
		}
		out = append(out, CanvasProblem{code, name})
	}
	o := obj(raw)
	if id, ok := o["id"]; !ok || id == nil || id == "" {
		add("id-missing", "id")
	} else if s, ok := id.(string); !ok || !idRE.MatchString(s) {
		add("id-invalid", "id")
	}
	if t, ok := o["type"]; !ok || t == nil || t == "" {
		add("type-missing", "type")
	} else if s, ok := t.(string); !ok || !typeRE.MatchString(s) {
		add("type-invalid", "type")
	}
	schema := o["schemaVersion"]
	if n, _ := isNumber(schema); schema == nil || n == 0 && isZero(schema) {
		add("schema-missing", "schemaVersion")
	} else if v, ok := isInt(schema); !ok || v < 0 || v > CanvasSchemaVersion {
		add("schema-unsupported", "schemaVersion")
	}
	g := obj(o["geometry"])
	for _, f := range []struct {
		name   string
		lo, hi float64
	}{{"x", -maxCoordinate, maxCoordinate}, {"y", -maxCoordinate, maxCoordinate}, {"w", minSize, maxSize}, {"h", minSize, maxSize}} {
		v, ok := g[f.name]
		if !ok || v == nil {
			v = 0.0
		}
		n, isNum := isNumber(v)
		if !isNum || !finite(n) || n < f.lo || n > f.hi {
			add("geometry-invalid", "geometry."+f.name)
		}
	}
	rot, ok := g["rotation"]
	if !ok || rot == nil {
		rot = 0.0
	}
	if r, ok := isInt(rot); !ok || r < 0 || r > 3 {
		add("geometry-invalid", "geometry.rotation")
	}
	content := obj(o["content"])
	if title, ok := content["title"].(string); !ok || strings.TrimSpace(title) == "" || utf8.RuneCountInString(title) > 240 {
		add("content-invalid", "content.title")
	}
	text, ok := content["text"]
	if !ok || text == nil {
		text = ""
	}
	if s, ok := text.(string); !ok || len(s) > maxText {
		add("content-invalid", "content.text")
	}
	if data, ok := content["data"]; ok && data != nil {
		m, isMap := data.(M)
		if b, _ := json.Marshal(m); !isMap || len(b) > maxData {
			add("content-invalid", "content.data")
		}
	}
	origin := obj(o["createdBy"])
	if p, ok := origin["pluginId"]; !ok || p == nil || p == "" {
		add("origin-missing", "createdBy.pluginId")
	} else if s, ok := p.(string); !ok || !pluginRE.MatchString(s) {
		add("origin-invalid", "createdBy.pluginId")
	}
	version, ok := origin["pluginVersion"]
	if !ok || version == nil {
		version = ""
	}
	if s, ok := version.(string); !ok || (s != "" && !versionRE.MatchString(s)) {
		add("origin-invalid", "createdBy.pluginVersion")
	}
	if r, ok := isInt(o["revision"]); !ok || r < 1 {
		add("revision-invalid", "revision")
	}
	policy := obj(o["interactionPolicy"])
	mode := policy["mode"]
	switch mode {
	case nil, "":
		add("interaction-missing", "interactionPolicy.mode")
	case "fixed", "select", "move", "edit":
	default:
		add("interaction-invalid", "interactionPolicy.mode")
	}
	if policy["stylusHitTest"] == true && mode == "fixed" {
		add("interaction-invalid", "interactionPolicy.stylusHitTest")
	}
	anchorsRaw, ok := o["sourceAnchors"]
	if !ok || anchorsRaw == nil {
		anchorsRaw = []any{}
	}
	anchors, ok := anchorsRaw.([]any)
	if !ok {
		add("anchor-kind", "sourceAnchors")
		return out
	}
	if len(anchors) > maxAnchors {
		add("anchor-limit", "sourceAnchors")
	}
	for i, a := range anchors {
		validateAnchor(obj(a), func(code, name string) { add(code, fmt.Sprintf("sourceAnchors[%d].%s", i, name)) })
	}
	return out
}

func isZero(v any) bool { n, ok := isNumber(v); return ok && n == 0 }

func validateAnchor(a M, add func(code, name string)) {
	// A field that belongs to another kind is reported rather than ignored.
	has := func(name string) bool {
		switch v := a[name].(type) {
		case nil:
			return false
		case string:
			return v != ""
		case float64:
			return v != 0
		case []any:
			return len(v) > 0
		}
		return true
	}
	reject := func(names ...string) {
		for _, n := range names {
			if has(n) {
				add("anchor-invalid", n)
			}
		}
	}
	validID := func(v any) bool { s, ok := v.(string); return ok && idRE.MatchString(s) }
	intOr0 := func(name string) (int, bool) {
		v, ok := a[name]
		if !ok || v == nil {
			return 0, true
		}
		return isInt(v)
	}
	span := func(minLength int) {
		start, ok1 := intOr0("startMs")
		end, ok2 := intOr0("endMs")
		if !ok1 || !ok2 || start < 0 || end < start || end > maxLectureMs || end-start < minLength {
			add("anchor-invalid", "startMs")
		}
	}
	page := func(name string) int {
		if v, ok := isInt(a[name]); ok {
			return v
		}
		if has(name) {
			return -1
		}
		return 0
	}
	switch a["kind"] {
	case "lecture":
		if !validID(a["lectureId"]) {
			add("anchor-invalid", "lectureId")
		}
		span(0)
		reject("file", "page", "pageEnd", "text", "strokeIds")
	case "transcript":
		if !validID(a["lectureId"]) {
			add("anchor-invalid", "lectureId")
		}
		span(1)
		if t, ok := a["text"].(string); !ok || strings.TrimSpace(t) == "" || utf8.RuneCountInString(t) > 8000 {
			add("anchor-invalid", "text")
		}
		reject("file", "page", "pageEnd", "strokeIds")
	case "pdf":
		if !validPath(a["file"]) {
			add("anchor-invalid", "file")
		}
		first, last := page("page"), page("pageEnd")
		if first < 1 || first > maxPages {
			add("anchor-invalid", "page")
		}
		if last != 0 && (last < first || last > maxPages) {
			add("anchor-invalid", "pageEnd")
		}
		reject("lectureId", "startMs", "endMs", "text", "strokeIds")
	case "courseware":
		if !validPath(a["file"]) {
			add("anchor-invalid", "file")
		}
		reject("lectureId", "startMs", "endMs", "page", "pageEnd", "text", "strokeIds")
	case "stroke":
		if has("lectureId") && !validID(a["lectureId"]) {
			add("anchor-invalid", "lectureId")
		}
		strokes, _ := a["strokeIds"].([]any)
		if len(strokes) == 0 || len(strokes) > maxStrokes {
			add("anchor-invalid", "strokeIds")
		}
		for i, s := range strokes {
			if !validID(s) {
				add("anchor-invalid", fmt.Sprintf("strokeIds[%d]", i))
			}
		}
		reject("file", "page", "pageEnd", "startMs", "endMs", "text")
	default:
		add("anchor-kind", "kind")
	}
}

// validPath reports a relative path inside the course folder.
func validPath(v any) bool {
	p, ok := v.(string)
	if !ok || p == "" || Len16(p) > 512 || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || strings.Contains(p, "\x00") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." {
			return false
		}
	}
	return true
}

// PluginObject is a plugin object on the canvas (pluginfabric.CanvasObject).
// It keeps the JSON it was read from, so a field this build does not draw
// survives a save, and it is immutable, so undo snapshots share it.
type PluginObject struct{ raw M }

// PluginObjectFrom reads a stored or proposed object; a problem is an
// error naming the rule.
func PluginObjectFrom(raw any) (*PluginObject, error) {
	if p := ValidateCanvasObject(raw, ""); len(p) > 0 {
		return nil, errors.New("插件对象无效：" + p[0].String())
	}
	return &PluginObject{raw: obj(Clone(raw))}, nil
}

func (o *PluginObject) ID() string    { return o.raw["id"].(string) }
func (o *PluginObject) Type() string  { return o.raw["type"].(string) }
func (o *PluginObject) Revision() int { v, _ := isInt(o.raw["revision"]); return v }
func (o *PluginObject) content() M    { return obj(o.raw["content"]) }
func (o *PluginObject) Title() string { return strings.TrimSpace(o.content()["title"].(string)) }
func (o *PluginObject) Text() string {
	s, _ := o.content()["text"].(string)
	return strings.TrimSpace(s)
}
func (o *PluginObject) PluginID() string { return obj(o.raw["createdBy"])["pluginId"].(string) }
func (o *PluginObject) Mode() string {
	s, _ := obj(o.raw["interactionPolicy"])["mode"].(string)
	return s
}

// Anchors are the object's source anchors.
func (o *PluginObject) Anchors() []M {
	var out []M
	for _, a := range list(o.raw["sourceAnchors"]) {
		out = append(out, obj(a))
	}
	return out
}

// Rect is where the object sits on the canvas.
func (o *PluginObject) Rect() Rect {
	g := obj(o.raw["geometry"])
	return RectLTWH(num(g["x"], 0), num(g["y"], 0), num(g["w"], 0), num(g["h"], 0))
}

// QuarterTurns is the object's rotation.
func (o *PluginObject) QuarterTurns() int { return toInt(obj(o.raw["geometry"])["rotation"]) }

// Source is where the object came from, in one line
// (pluginfabric.FallbackFor).
func (o *PluginObject) Source() string {
	anchors := o.Anchors()
	var parts []string
	for i, a := range anchors {
		if i == 3 {
			break
		}
		parts = append(parts, anchorText(a))
	}
	text := strings.Join(parts, "；")
	if len(anchors) > 3 {
		return fmt.Sprintf("%s 等 %d 处", text, len(anchors))
	}
	return text
}

// Fallback is the view a canvas shows when the owning plugin is missing.
type Fallback struct {
	ID, Type, PluginID, Title, Source, Summary string
	SchemaVersion, Revision                    int
	Data                                       M
}

// Fallback is the object's fallback view model.
func (o *PluginObject) Fallback() Fallback {
	text := []rune(o.Text())
	summary := string(text)
	if len(text) > fallbackSummary {
		summary = string(text[:fallbackSummary]) + "…"
	}
	schema, _ := isInt(o.raw["schemaVersion"])
	data := M{}
	for k, v := range obj(o.content()["data"]) {
		data[k] = v
	}
	return Fallback{ID: o.ID(), Type: o.Type(), PluginID: o.PluginID(), Title: o.Title(), Source: o.Source(),
		Summary: summary, SchemaVersion: schema, Revision: o.Revision(), Data: data}
}

// JSON is the stored object with its anchors, as the user exports it; the
// caller gets its own copy.
func (o *PluginObject) JSON() M { return obj(Clone(o.raw)) }

func anchorText(a M) string {
	clock := func(ms int) string {
		s := max(0, ms/1000)
		if s >= 3600 {
			return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
		}
		return fmt.Sprintf("%02d:%02d", s/60, s%60)
	}
	span := func() string {
		start, end := toInt(a["startMs"]), toInt(a["endMs"])
		if start == 0 && end == 0 {
			return ""
		}
		if end == 0 {
			return " 从 " + clock(start)
		}
		return " " + clock(start) + "–" + clock(end)
	}
	page, pageEnd := toInt(a["page"]), toInt(a["pageEnd"])
	switch a["kind"] {
	case "lecture":
		return "课程视频" + span()
	case "transcript":
		return "字幕" + span()
	case "pdf":
		if pageEnd > page {
			return fmt.Sprintf("PDF 第 %d–%d 页", page, pageEnd)
		}
		return fmt.Sprintf("PDF 第 %d 页", page)
	case "courseware":
		parts := strings.Split(str(a["file"]), "/")
		return "课件 " + parts[len(parts)-1]
	case "stroke":
		return fmt.Sprintf("手写笔迹 %d 笔", len(list(a["strokeIds"])))
	}
	return str(a["kind"])
}

// PluginBatch is a plugin batch the canvas checked and can commit as one
// undo entry (pluginfabric.CanvasBatch). Revision is the plugin-object
// revision it was checked against; a commit after another change is
// refused.
type PluginBatch struct {
	PluginID, Label           string
	Revision                  int
	Created, Updated, Deleted []*PluginObject
}

// NeedsApproval reports a batch that overwrites or deletes what is on the
// canvas: the user approves it after a preview. A create commits directly.
func (b *PluginBatch) NeedsApproval() bool { return len(b.Updated) > 0 || len(b.Deleted) > 0 }

// CheckPluginBatch reads raw for pluginID against the current objects and
// revision. Any problem is an error and nothing is staged.
func CheckPluginBatch(pluginID string, raw any, objects map[string]*PluginObject, revision int, taken func(id string) bool) (*PluginBatch, error) {
	m, ok := raw.(M)
	if !ok {
		return nil, errors.New("画布批次应为对象")
	}
	orList := func(k string) ([]any, bool) {
		v, ok := m[k]
		if !ok || v == nil {
			return nil, true
		}
		l, ok := v.([]any)
		return l, ok
	}
	writes, ok1 := orList("objects")
	deletes, ok2 := orList("delete")
	if !ok1 || !ok2 {
		return nil, errors.New("画布批次的 objects 和 delete 应为列表")
	}
	if len(writes) == 0 && len(deletes) == 0 {
		return nil, errors.New("批次没有任何对象改动")
	}
	if len(writes) > CanvasMaxBatch || len(deletes) > CanvasMaxBatch {
		return nil, errors.New("画布对象超过限制")
	}
	if r, ok := isInt(m["revision"]); !ok || r != revision {
		return nil, fmt.Errorf("画布已改变：批次读到版本 %s，当前版本 %d，请重新读取后再试", str(m["revision"]), revision)
	}
	seen := map[string]bool{}
	b := &PluginBatch{PluginID: pluginID, Revision: revision}
	for _, w := range writes {
		o, err := PluginObjectFrom(w)
		if err != nil {
			return nil, err
		}
		if seen[o.ID()] {
			return nil, fmt.Errorf("对象 %s 在同一批次里重复", o.ID())
		}
		seen[o.ID()] = true
		if o.PluginID() != pluginID {
			return nil, fmt.Errorf("插件只能写入自己的对象：%s", o.ID())
		}
		old := objects[o.ID()]
		if old == nil {
			if taken(o.ID()) {
				return nil, fmt.Errorf("对象标识 %s 已被画布占用", o.ID())
			}
			if o.Revision() != 1 {
				return nil, fmt.Errorf("新对象 %s 的 revision 应为 1", o.ID())
			}
			b.Created = append(b.Created, o)
			continue
		}
		if old.PluginID() != pluginID {
			return nil, fmt.Errorf("插件只能修改自己的对象：%s", o.ID())
		}
		if o.Revision() != old.Revision()+1 {
			return nil, fmt.Errorf("对象 %s 已改变：读到版本 %d，当前版本 %d", o.ID(), o.Revision()-1, old.Revision())
		}
		b.Updated = append(b.Updated, o)
	}
	for _, v := range deletes {
		id := str(v)
		old := objects[id]
		if seen[id] {
			return nil, fmt.Errorf("对象 %s 在同一批次里既写入又删除", id)
		}
		seen[id] = true
		if old == nil {
			return nil, fmt.Errorf("要删除的对象 %s 已不存在", id)
		}
		if old.PluginID() != pluginID {
			return nil, fmt.Errorf("插件只能删除自己的对象：%s", id)
		}
		b.Deleted = append(b.Deleted, old)
	}
	b.Label, _ = m["label"].(string)
	return b, nil
}

// CheckPluginBatch checks a plugin's CanvasBatch against the current
// objects and PluginRevision without changing anything.
func (d *Document) CheckPluginBatch(pluginID string, raw any) (*PluginBatch, error) {
	objects := map[string]*PluginObject{}
	for _, o := range d.PluginObjects {
		objects[o.ID()] = o
	}
	b, err := CheckPluginBatch(pluginID, raw, objects, d.PluginRevision, func(id string) bool { return d.Card(id) != nil })
	if err != nil {
		return nil, err
	}
	if len(d.Cards)+len(d.PluginObjects)+len(b.Created)-len(b.Deleted) > 2000 {
		return nil, errors.New("画布对象数量超过限制")
	}
	return b, nil
}

// CommitPluginBatch commits a checked batch as one undo entry. A batch
// checked before another change to the plugin objects is refused whole.
func (d *Document) CommitPluginBatch(b *PluginBatch) error {
	if b.Revision != d.PluginRevision {
		return errors.New("画布已改变，请重新读取后再试")
	}
	updated := map[string]*PluginObject{}
	for _, o := range b.Updated {
		updated[o.ID()] = o
	}
	deleted := map[string]bool{}
	for _, o := range b.Deleted {
		deleted[o.ID()] = true
	}
	d.Checkpoint()
	var next []*PluginObject
	for _, o := range d.PluginObjects {
		if deleted[o.ID()] {
			continue
		}
		if u := updated[o.ID()]; u != nil {
			o = u
		}
		next = append(next, o)
	}
	d.PluginObjects = append(next, b.Created...)
	d.dropLinks(func(id string) bool { return deleted[id] })
	d.PluginRevision++
	d.Changed()
	return nil
}

func (d *Document) dropLinks(gone func(id string) bool) {
	var keep []Connection
	for _, c := range d.Connections {
		if !gone(c.From) && !gone(c.To) {
			keep = append(keep, c)
		}
	}
	d.Connections = keep
}

// RemovePluginObject removes one plugin object at the user's request, as
// one undo entry.
func (d *Document) RemovePluginObject(id string) {
	found := false
	for _, o := range d.PluginObjects {
		found = found || o.ID() == id
	}
	if !found {
		return
	}
	d.Checkpoint()
	var next []*PluginObject
	for _, o := range d.PluginObjects {
		if o.ID() != id {
			next = append(next, o)
		}
	}
	d.PluginObjects = next
	d.dropLinks(func(x string) bool { return x == id })
	d.PluginRevision++
	d.Changed()
}
