package pluginfabric

// This file owns the versioned CanvasObject contract plugins write persistent
// canvas output through, the transaction and undo boundary that output enters,
// and the fallback the canvas shows for an object whose plugin is missing.
//
// Contract: docs/specs/0032-ui-extension-protocol.md (Canvas plugin objects).
// The bounds mirror the study canvas document limits (docs/study-canvas.md), so
// an object needs no conversion when the real document adopts it. Nothing here
// reads a file, opens the database or starts a runtime: a batch is validated
// and committed in memory, and the study canvas stays the owner of storage,
// input and gesture behavior until PF-10's integration step.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	// CanvasSchemaVersion is the object shape this build reads and writes.
	CanvasSchemaVersion = 1

	// CanvasMaxText caps content text, in bytes, as the study canvas does.
	CanvasMaxText = 800_000
	// CanvasMaxData caps the encoded structured content of one object.
	CanvasMaxData = 256 << 10
	// CanvasMaxAnchors caps the source anchors of one object.
	CanvasMaxAnchors = 32
	// CanvasMaxObjects caps the objects one batch may write or delete.
	CanvasMaxObjects = 200
	// CanvasMaxCoordinate bounds a geometry value, as the study canvas does.
	CanvasMaxCoordinate = 1e9
	// CanvasMinSize and CanvasMaxSize bound an object's width and height.
	CanvasMinSize = 24
	CanvasMaxSize = 100_000
	// CanvasMaxLectureMs bounds a lecture interval or transcript segment.
	CanvasMaxLectureMs int64 = 7 * 24 * 3600 * 1000
	// CanvasMaxPages bounds a PDF page range, as the PDF reader does.
	CanvasMaxPages = 5000
	// CanvasMaxStrokes caps the handwriting strokes one anchor names.
	CanvasMaxStrokes = 500
	// CanvasFallbackSummary is how many runes of text the fallback shows.
	CanvasFallbackSummary = 280
)

var (
	// ErrCanvasConflict means the batch was built on a revision the canvas no
	// longer has. Nothing is written; the caller rereads and rebuilds.
	ErrCanvasConflict = errors.New("画布已被另一个窗口修改，请重新读取后再试")
	// ErrCanvasCanceled means the caller cancelled before the batch committed.
	ErrCanvasCanceled = errors.New("画布操作已取消")
	// ErrCanvasLimit means the batch is larger than the canvas accepts.
	ErrCanvasLimit = errors.New("画布对象超过限制")
	// ErrCanvasNothingToUndo means the undo stack is empty.
	ErrCanvasNothingToUndo = errors.New("没有可撤销的画布操作")

	// canvasIDRE matches an object, lecture or stroke id.
	canvasIDRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,99}$`)
	// canvasTypeRE matches a namespaced object type such as review-lab.card.
	canvasTypeRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}(?:\.[a-z][a-z0-9-]{0,31})+$`)
)

// CanvasAnchorKind is the material an object came from.
type CanvasAnchorKind string

// The anchor kinds of spec 0032.
const (
	// AnchorLecture is a lecture interval, in milliseconds.
	AnchorLecture CanvasAnchorKind = "lecture"
	// AnchorPDF is a page or page range of a PDF courseware file.
	AnchorPDF CanvasAnchorKind = "pdf"
	// AnchorCourseware is a courseware file in the course folder.
	AnchorCourseware CanvasAnchorKind = "courseware"
	// AnchorTranscript is a transcript segment, in milliseconds.
	AnchorTranscript CanvasAnchorKind = "transcript"
	// AnchorStroke is a set of handwriting strokes.
	AnchorStroke CanvasAnchorKind = "stroke"
)

// CanvasAnchor points at the material an object came from. Only the fields its
// kind uses may be set: a field belonging to another kind is a diagnostic, so a
// wrong anchor cannot look right.
type CanvasAnchor struct {
	Kind CanvasAnchorKind `json:"kind"`
	// LectureID is the lecture of a lecture interval, transcript segment or
	// stroke set.
	LectureID string `json:"lectureId,omitempty"`
	// StartMs and EndMs bound a lecture interval; a transcript segment needs
	// both and a nonzero length.
	StartMs int64 `json:"startMs,omitempty"`
	EndMs   int64 `json:"endMs,omitempty"`
	// File is a courseware path relative to the course folder. Page and PageEnd
	// bound a PDF page or page range.
	File    string `json:"file,omitempty"`
	Page    int    `json:"page,omitempty"`
	PageEnd int    `json:"pageEnd,omitempty"`
	// Text is the transcript text the object quotes.
	Text string `json:"text,omitempty"`
	// StrokeIDs are the handwriting strokes the object refers to.
	StrokeIDs []string `json:"strokeIds,omitempty"`
}

// CanvasGeometry is where an object sits on the canvas, in canvas units.
type CanvasGeometry struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
	// Rotation is quarter turns clockwise, 0–3, for media that can be rotated.
	Rotation int `json:"rotation,omitempty"`
}

// CanvasContent is what the object shows. The host renders Title and Text in the
// fallback and never interprets Data; only the owning plugin's renderer does.
type CanvasContent struct {
	Title string         `json:"title"`
	Text  string         `json:"text,omitempty"`
	Data  map[string]any `json:"data,omitempty"`
}

// CanvasOrigin is who wrote the object.
type CanvasOrigin struct {
	PluginID      string `json:"pluginId"`
	PluginVersion string `json:"pluginVersion,omitempty"`
}

// CanvasInteractionMode is how much input an object accepts. An overlay never
// takes stylus hit-testing unless the user switched tools (spec 0032).
type CanvasInteractionMode string

// The interaction modes a plugin may ask for.
const (
	// CanvasFixed accepts no pointer or stylus interaction.
	CanvasFixed CanvasInteractionMode = "fixed"
	// CanvasSelect can be selected but not moved.
	CanvasSelect CanvasInteractionMode = "select"
	// CanvasMove can be selected and moved.
	CanvasMove CanvasInteractionMode = "move"
	// CanvasEdit can be moved and its content edited through the plugin.
	CanvasEdit CanvasInteractionMode = "edit"
)

// CanvasInteractionPolicy is how the object reacts to input.
type CanvasInteractionPolicy struct {
	Mode CanvasInteractionMode `json:"mode"`
	// StylusHitTest lets a stylus stroke start on the object instead of the
	// layer below. A fixed object never takes one.
	StylusHitTest bool `json:"stylusHitTest,omitempty"`
}

// CanvasObject is one persistent plugin object on the canvas. It is the only
// shape plugin output is stored in; a plugin extends the canvas through tools,
// renderers, actions, overlays and edges, never with a document of its own.
type CanvasObject struct {
	ID                string                  `json:"id"`
	Type              string                  `json:"type"`
	SchemaVersion     int                     `json:"schemaVersion"`
	Geometry          CanvasGeometry          `json:"geometry"`
	Content           CanvasContent           `json:"content"`
	SourceAnchors     []CanvasAnchor          `json:"sourceAnchors,omitempty"`
	CreatedBy         CanvasOrigin            `json:"createdBy"`
	Revision          int64                   `json:"revision"`
	InteractionPolicy CanvasInteractionPolicy `json:"interactionPolicy"`
}

// Validate reports every problem in the object; it returns nil when the object
// may be committed.
func (o CanvasObject) Validate() error {
	p := &problems{}
	validateCanvasObject(o, "", p)
	if len(p.list) > 0 {
		return &Error{Subject: canvasSubject(), Problems: p.list}
	}
	return nil
}

// canvasSubject names a canvas error in the same shape a manifest error uses.
func canvasSubject() string {
	return fmt.Sprintf("画布对象不符合 coursive/canvas-object@%d", CanvasSchemaVersion)
}

// field joins an object path with a field path, so one validator serves a bare
// object and an object inside a batch.
func field(at, name string) string {
	if at == "" {
		return name
	}
	return at + "." + name
}

func validateCanvasObject(o CanvasObject, at string, p *problems) {
	switch {
	case o.ID == "":
		p.add("id-missing", field(at, "id"), "缺少对象 id")
	case !canvasIDRE.MatchString(o.ID):
		p.add("id-invalid", field(at, "id"), "对象 id %q 无效：应以字母或数字开头，只含字母、数字和 . _ : -，最长 100 字符", o.ID)
	}
	switch {
	case o.Type == "":
		p.add("type-missing", field(at, "type"), "缺少对象类型")
	case !canvasTypeRE.MatchString(o.Type):
		p.add("type-invalid", field(at, "type"), "对象类型 %q 应为带命名空间的小写标识，例如 review-lab.card", o.Type)
	}
	switch {
	case o.SchemaVersion == 0:
		p.add("schema-missing", field(at, "schemaVersion"), "缺少 schemaVersion")
	case o.SchemaVersion < 0 || o.SchemaVersion > CanvasSchemaVersion:
		p.add("schema-unsupported", field(at, "schemaVersion"), "对象版本 %d 不受支持：本版本读取 1–%d，请更新应用", o.SchemaVersion, CanvasSchemaVersion)
	}

	g := o.Geometry
	for _, axis := range []struct {
		name      string
		v, lo, hi float64
	}{
		{"x", g.X, -CanvasMaxCoordinate, CanvasMaxCoordinate},
		{"y", g.Y, -CanvasMaxCoordinate, CanvasMaxCoordinate},
		{"w", g.W, CanvasMinSize, CanvasMaxSize},
		{"h", g.H, CanvasMinSize, CanvasMaxSize},
	} {
		if math.IsNaN(axis.v) || math.IsInf(axis.v, 0) || axis.v < axis.lo || axis.v > axis.hi {
			p.add("geometry-invalid", field(at, "geometry."+axis.name), "geometry.%s 应在 %g 到 %g 之间", axis.name, axis.lo, axis.hi)
		}
	}
	if g.Rotation < 0 || g.Rotation > 3 {
		p.add("geometry-invalid", field(at, "geometry.rotation"), "geometry.rotation 应为 0–3 个四分之一圈")
	}

	switch {
	case strings.TrimSpace(o.Content.Title) == "":
		p.add("content-invalid", field(at, "content.title"), "缺少标题：缺失插件的降级视图需要标题")
	case utf8.RuneCountInString(o.Content.Title) > 240:
		p.add("content-invalid", field(at, "content.title"), "标题不应超过 240 字")
	}
	if !utf8.ValidString(o.Content.Text) {
		p.add("content-invalid", field(at, "content.text"), "文字应为有效的 UTF-8")
	} else if len(o.Content.Text) > CanvasMaxText {
		p.add("content-invalid", field(at, "content.text"), "文字不应超过 %d 字节", CanvasMaxText)
	}
	if len(o.Content.Data) > 0 {
		if b, err := json.Marshal(o.Content.Data); err != nil {
			p.add("content-invalid", field(at, "content.data"), "结构化内容无法序列化为 JSON")
		} else if len(b) > CanvasMaxData {
			p.add("content-invalid", field(at, "content.data"), "结构化内容不应超过 %d 字节", CanvasMaxData)
		}
	}

	switch {
	case o.CreatedBy.PluginID == "":
		p.add("origin-missing", field(at, "createdBy.pluginId"), "缺少 createdBy.pluginId")
	case !idRE.MatchString(o.CreatedBy.PluginID):
		p.add("origin-invalid", field(at, "createdBy.pluginId"), "插件 id %q 无效", o.CreatedBy.PluginID)
	}
	if v := o.CreatedBy.PluginVersion; v != "" && !manifestVersionRE.MatchString(v) {
		p.add("origin-invalid", field(at, "createdBy.pluginVersion"), "插件版本 %q 应为 semver，例如 1.0.0", v)
	}

	if o.Revision < 1 {
		p.add("revision-invalid", field(at, "revision"), "对象 revision 从 1 开始")
	}

	switch o.InteractionPolicy.Mode {
	case CanvasFixed, CanvasSelect, CanvasMove, CanvasEdit:
	case "":
		p.add("interaction-missing", field(at, "interactionPolicy.mode"), "缺少 interactionPolicy.mode")
	default:
		p.add("interaction-invalid", field(at, "interactionPolicy.mode"), "未知的交互模式 %q：应为 fixed、select、move 或 edit", o.InteractionPolicy.Mode)
	}
	if o.InteractionPolicy.StylusHitTest && o.InteractionPolicy.Mode == CanvasFixed {
		p.add("interaction-invalid", field(at, "interactionPolicy.stylusHitTest"), "fixed 对象不接受触控笔命中")
	}

	if len(o.SourceAnchors) > CanvasMaxAnchors {
		p.add("anchor-limit", field(at, "sourceAnchors"), "来源锚点不应超过 %d 个", CanvasMaxAnchors)
	}
	for i, a := range o.SourceAnchors {
		validateCanvasAnchor(a, fmt.Sprintf("%s[%d]", field(at, "sourceAnchors"), i), p)
	}
}

func validateCanvasAnchor(a CanvasAnchor, at string, p *problems) {
	// set maps a field name to whether the anchor uses it, so a field that
	// belongs to another kind is reported rather than silently ignored.
	set := map[string]bool{
		"lectureId": a.LectureID != "",
		"startMs":   a.StartMs != 0,
		"endMs":     a.EndMs != 0,
		"file":      a.File != "",
		"page":      a.Page != 0,
		"pageEnd":   a.PageEnd != 0,
		"text":      a.Text != "",
		"strokeIds": len(a.StrokeIDs) > 0,
	}
	reject := func(names ...string) {
		for _, name := range names {
			if set[name] {
				p.add("anchor-invalid", field(at, name), "%s 锚点不使用 %s", a.Kind, name)
			}
		}
	}
	span := func(minLength int64) {
		if a.StartMs < 0 || a.EndMs < a.StartMs || a.EndMs > CanvasMaxLectureMs || a.EndMs-a.StartMs < minLength {
			p.add("anchor-invalid", field(at, "startMs"), "时间区间应为 0 ≤ startMs ≤ endMs ≤ %d 毫秒", CanvasMaxLectureMs)
		}
	}

	switch a.Kind {
	case AnchorLecture:
		if !canvasIDRE.MatchString(a.LectureID) {
			p.add("anchor-invalid", field(at, "lectureId"), "课程视频锚点需要有效的 lectureId")
		}
		span(0)
		reject("file", "page", "pageEnd", "text", "strokeIds")
	case AnchorTranscript:
		if !canvasIDRE.MatchString(a.LectureID) {
			p.add("anchor-invalid", field(at, "lectureId"), "字幕锚点需要有效的 lectureId")
		}
		span(1)
		if strings.TrimSpace(a.Text) == "" {
			p.add("anchor-invalid", field(at, "text"), "字幕锚点需要该句的文字")
		} else if utf8.RuneCountInString(a.Text) > 8000 {
			p.add("anchor-invalid", field(at, "text"), "字幕文字不应超过 8000 字")
		}
		reject("file", "page", "pageEnd", "strokeIds")
	case AnchorPDF:
		if !validCanvasPath(a.File) {
			p.add("anchor-invalid", field(at, "file"), "PDF 锚点需要课程文件夹内的相对路径")
		}
		if a.Page < 1 || a.Page > CanvasMaxPages {
			p.add("anchor-invalid", field(at, "page"), "PDF 页码从 1 开始，不超过 %d", CanvasMaxPages)
		}
		if a.PageEnd != 0 && (a.PageEnd < a.Page || a.PageEnd > CanvasMaxPages) {
			p.add("anchor-invalid", field(at, "pageEnd"), "PDF 结束页应不小于起始页且不超过 %d", CanvasMaxPages)
		}
		reject("lectureId", "startMs", "endMs", "text", "strokeIds")
	case AnchorCourseware:
		if !validCanvasPath(a.File) {
			p.add("anchor-invalid", field(at, "file"), "课件锚点需要课程文件夹内的相对路径")
		}
		reject("lectureId", "startMs", "endMs", "page", "pageEnd", "text", "strokeIds")
	case AnchorStroke:
		if a.LectureID != "" && !canvasIDRE.MatchString(a.LectureID) {
			p.add("anchor-invalid", field(at, "lectureId"), "lectureId %q 无效", a.LectureID)
		}
		switch {
		case len(a.StrokeIDs) == 0:
			p.add("anchor-invalid", field(at, "strokeIds"), "笔迹锚点需要至少一笔")
		case len(a.StrokeIDs) > CanvasMaxStrokes:
			p.add("anchor-invalid", field(at, "strokeIds"), "笔迹锚点不应超过 %d 笔", CanvasMaxStrokes)
		}
		for i, id := range a.StrokeIDs {
			if !canvasIDRE.MatchString(id) {
				p.add("anchor-invalid", fmt.Sprintf("%s[%d]", field(at, "strokeIds"), i), "笔迹 id %q 无效", id)
			}
		}
		reject("file", "page", "pageEnd", "startMs", "endMs", "text")
	default:
		p.add("anchor-kind", field(at, "kind"), "未知的来源锚点类型 %q：应为 lecture、pdf、courseware、transcript 或 stroke", a.Kind)
	}
}

// validCanvasPath reports whether p is a relative path inside the course folder.
func validCanvasPath(p string) bool {
	if p == "" || len(p) > 512 || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") {
		return false
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// CanvasBatch is one atomic change a plugin proposes: the objects it writes,
// each already at its new revision, and the ids it deletes.
type CanvasBatch struct {
	// Revision is the canvas revision the caller read. A mismatch commits
	// nothing.
	Revision int64 `json:"revision"`
	// Objects are the objects to write; a create starts at revision 1 and an
	// update moves the object exactly one step.
	Objects []CanvasObject `json:"objects,omitempty"`
	// Delete names the objects to remove, by id.
	Delete []string `json:"delete,omitempty"`
	// Label is what the existing undo stack shows for this batch.
	Label string `json:"label,omitempty"`
}

// CanvasCommit is what a committed batch produced.
type CanvasCommit struct {
	Revision int64          `json:"revision"`
	Objects  []CanvasObject `json:"objects,omitempty"`
	Deleted  []string       `json:"delete,omitempty"`
	Label    string         `json:"label,omitempty"`
}

// Canvas is the transaction and undo boundary a plugin writes through. It holds
// the document revision, the objects and one undo entry per committed batch, and
// nothing else: no file, database or runtime handle. The study canvas document
// implements the same boundary when PF-10 wires it in.
type Canvas struct {
	mu       sync.Mutex
	revision int64
	objects  map[string]CanvasObject
	undo     []canvasUndo
}

// canvasUndo is one undo entry: what was there before a batch, so the undo can
// restore it. A nil object means the id did not exist before the batch.
type canvasUndo struct {
	label  string
	before map[string]*CanvasObject
}

// NewCanvas returns an empty canvas at revision 0.
func NewCanvas() *Canvas {
	return &Canvas{objects: map[string]CanvasObject{}}
}

// Revision is the current canvas revision.
func (c *Canvas) Revision() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.revision
}

// Object returns one object by id.
func (c *Canvas) Object(id string) (CanvasObject, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	o, ok := c.objects[id]
	return o, ok
}

// Objects returns every object, sorted by id.
func (c *Canvas) Objects() []CanvasObject {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]CanvasObject, 0, len(c.objects))
	for _, o := range c.objects {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Apply validates the whole batch and commits it as one transaction. An invalid
// object, a stale document revision, an object revision mismatch or a cancelled
// context leaves the canvas and the undo stack untouched and reports why; a
// committed batch is exactly one undo entry.
func (c *Canvas) Apply(ctx context.Context, batch CanvasBatch) (CanvasCommit, error) {
	if err := ctx.Err(); err != nil {
		return CanvasCommit{}, canceled(err)
	}
	if len(batch.Objects)+len(batch.Delete) == 0 {
		return CanvasCommit{}, &Error{Subject: canvasSubject(), Problems: []Problem{
			{Code: "batch-empty", Path: "objects", Message: "批次没有任何对象改动"},
		}}
	}
	if len(batch.Objects) > CanvasMaxObjects || len(batch.Delete) > CanvasMaxObjects {
		return CanvasCommit{}, ErrCanvasLimit
	}

	p := &problems{}
	if batch.Revision < 0 {
		p.add("revision-invalid", "revision", "批次需要读取时的画布 revision")
	}
	seen := map[string]bool{}
	for i, o := range batch.Objects {
		validateCanvasObject(o, fmt.Sprintf("objects[%d]", i), p)
		if seen[o.ID] {
			p.add("duplicate-id", fmt.Sprintf("objects[%d].id", i), "对象 %q 在同一批次里重复", o.ID)
		}
		seen[o.ID] = true
		if err := ctx.Err(); err != nil {
			return CanvasCommit{}, canceled(err)
		}
	}
	for i, id := range batch.Delete {
		switch {
		case !canvasIDRE.MatchString(id):
			p.add("id-invalid", fmt.Sprintf("delete[%d]", i), "要删除的对象 id %q 无效", id)
		case seen[id]:
			p.add("duplicate-id", fmt.Sprintf("delete[%d]", i), "对象 %q 在同一批次里既写入又删除", id)
		}
		seen[id] = true
	}
	if len(p.list) > 0 {
		return CanvasCommit{}, &Error{Subject: canvasSubject(), Problems: p.list}
	}
	if err := ctx.Err(); err != nil {
		return CanvasCommit{}, canceled(err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if batch.Revision != c.revision {
		return CanvasCommit{}, fmt.Errorf("%w：批次读到版本 %d，当前版本 %d", ErrCanvasConflict, batch.Revision, c.revision)
	}
	for _, o := range batch.Objects {
		old, ok := c.objects[o.ID]
		if !ok {
			if o.Revision != 1 {
				return CanvasCommit{}, fmt.Errorf("%w：新对象 %s 的 revision 应为 1，收到 %d", ErrCanvasConflict, o.ID, o.Revision)
			}
			continue
		}
		if o.Revision != old.Revision+1 {
			return CanvasCommit{}, fmt.Errorf("%w：对象 %s 读到版本 %d，当前版本 %d", ErrCanvasConflict, o.ID, o.Revision-1, old.Revision)
		}
	}
	for _, id := range batch.Delete {
		if _, ok := c.objects[id]; !ok {
			return CanvasCommit{}, fmt.Errorf("%w：要删除的对象 %s 已不存在", ErrCanvasConflict, id)
		}
	}
	if err := ctx.Err(); err != nil {
		return CanvasCommit{}, canceled(err)
	}

	entry := canvasUndo{label: batch.Label, before: map[string]*CanvasObject{}}
	for _, o := range batch.Objects {
		if old, ok := c.objects[o.ID]; ok {
			previous := old
			entry.before[o.ID] = &previous
		} else {
			entry.before[o.ID] = nil
		}
		c.objects[o.ID] = o
	}
	for _, id := range batch.Delete {
		previous := c.objects[id]
		entry.before[id] = &previous
		delete(c.objects, id)
	}
	c.undo = append(c.undo, entry)
	c.revision++
	return CanvasCommit{
		Revision: c.revision,
		Objects:  append([]CanvasObject(nil), batch.Objects...),
		Deleted:  append([]string(nil), batch.Delete...),
		Label:    batch.Label,
	}, nil
}

// Undo restores the canvas as it was before the last committed batch. The
// restore is a new revision, so another window sees it as a change; a cancelled
// undo changes nothing.
func (c *Canvas) Undo(ctx context.Context) (CanvasCommit, error) {
	if err := ctx.Err(); err != nil {
		return CanvasCommit{}, canceled(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.undo) == 0 {
		return CanvasCommit{}, ErrCanvasNothingToUndo
	}
	if err := ctx.Err(); err != nil {
		return CanvasCommit{}, canceled(err)
	}
	entry := c.undo[len(c.undo)-1]
	c.undo = c.undo[:len(c.undo)-1]
	commit := CanvasCommit{Label: entry.label}
	for id, before := range entry.before {
		if before == nil {
			delete(c.objects, id)
			commit.Deleted = append(commit.Deleted, id)
			continue
		}
		c.objects[id] = *before
		commit.Objects = append(commit.Objects, *before)
	}
	sort.Slice(commit.Objects, func(i, j int) bool { return commit.Objects[i].ID < commit.Objects[j].ID })
	sort.Strings(commit.Deleted)
	c.revision++
	commit.Revision = c.revision
	return commit, nil
}

func canceled(err error) error {
	return fmt.Errorf("%w：%v", ErrCanvasCanceled, err)
}

// CanvasFallback is what the canvas shows for an object whose plugin is
// missing: its title, where it came from, a text summary and the data the user
// can still export. It is a view model, never a replacement object, and
// uninstalling a plugin deletes neither the object nor its references.
type CanvasFallback struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	SchemaVersion int            `json:"schemaVersion"`
	Revision      int64          `json:"revision"`
	PluginID      string         `json:"pluginId"`
	Title         string         `json:"title"`
	Source        string         `json:"source"`
	Summary       string         `json:"summary"`
	Data          map[string]any `json:"data,omitempty"`
	Anchors       []CanvasAnchor `json:"anchors,omitempty"`
}

// FallbackFor builds the fallback view model for o. It never fails: an object
// whose content is empty or unreadable still shows its id, type and revision, so
// a missing plugin loses nothing.
func FallbackFor(o CanvasObject) CanvasFallback {
	out := CanvasFallback{
		ID:            o.ID,
		Type:          o.Type,
		SchemaVersion: o.SchemaVersion,
		Revision:      o.Revision,
		PluginID:      o.CreatedBy.PluginID,
		Title:         strings.TrimSpace(o.Content.Title),
		Source:        canvasSourceText(o.SourceAnchors),
		Summary:       truncateRunes(strings.TrimSpace(o.Content.Text), CanvasFallbackSummary),
	}
	if len(o.Content.Data) > 0 {
		out.Data = make(map[string]any, len(o.Content.Data))
		for k, v := range o.Content.Data {
			out.Data[k] = v
		}
	}
	if len(o.SourceAnchors) > 0 {
		out.Anchors = append([]CanvasAnchor(nil), o.SourceAnchors...)
	}
	return out
}

// canvasSourceText describes the first few anchors in one line.
func canvasSourceText(anchors []CanvasAnchor) string {
	const shown = 3
	parts := make([]string, 0, shown)
	for i, a := range anchors {
		if i == shown {
			break
		}
		parts = append(parts, canvasAnchorText(a))
	}
	text := strings.Join(parts, "；")
	if len(anchors) > shown {
		text += fmt.Sprintf(" 等 %d 处", len(anchors))
	}
	return text
}

func canvasAnchorText(a CanvasAnchor) string {
	switch a.Kind {
	case AnchorLecture:
		return "课程视频" + canvasSpanText(a.StartMs, a.EndMs)
	case AnchorTranscript:
		return "字幕" + canvasSpanText(a.StartMs, a.EndMs)
	case AnchorPDF:
		if a.PageEnd > a.Page {
			return fmt.Sprintf("PDF 第 %d–%d 页", a.Page, a.PageEnd)
		}
		return fmt.Sprintf("PDF 第 %d 页", a.Page)
	case AnchorCourseware:
		return "课件 " + path.Base(a.File)
	case AnchorStroke:
		return fmt.Sprintf("手写笔迹 %d 笔", len(a.StrokeIDs))
	}
	return string(a.Kind)
}

func canvasSpanText(startMs, endMs int64) string {
	switch {
	case startMs == 0 && endMs == 0:
		return ""
	case endMs == 0:
		return " 从 " + canvasClock(startMs)
	}
	return " " + canvasClock(startMs) + "–" + canvasClock(endMs)
}

func canvasClock(ms int64) string {
	seconds := ms / 1000
	if seconds < 0 {
		seconds = 0
	}
	if hours := seconds / 3600; hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, seconds/60%60, seconds%60)
	}
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

// truncateRunes cuts s to max runes, marking that it was cut.
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}
