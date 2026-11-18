package study

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

var nextID atomic.Int64

// NewID is studyId: unique within the session and across devices.
func NewID() string {
	return strconv.FormatInt(time.Now().UnixMicro(), 36) + "-" + strconv.FormatInt(nextID.Add(1)-1, 10)
}

// Layer is StudyLayer.
type Layer struct {
	ID, Name        string
	Visible, Locked bool
}

func (l Layer) JSON() M {
	return M{"id": l.ID, "name": l.Name, "visible": l.Visible, "locked": l.Locked}
}

// Card is StudyCard: a material, a note or a generated card on the canvas.
type Card struct {
	ID, Type, Layer, Title, Text, Path, Source string
	Rect                                       Rect
	DocumentID, Generation, ConversationID     string
	SourceLectureID, OriginalPath, PDFColumn   string
	Collapsed                                  bool
	ExpandedH                                  float64
	Page, PageCount, QuarterTurns              int
	PageAspect                                 float64
	TextBoxes                                  []any
	Blocks                                     []Block
	Sources                                    []Anchor
	Responses                                  map[string]string
}

// Generated reports a card the assistant wrote.
func (c *Card) Generated() bool {
	switch c.Type {
	case "note", "explanation", "knowledge", "practice":
		return true
	}
	return false
}

func blocksJSON(bs []Block) []any {
	out := make([]any, len(bs))
	for i, b := range bs {
		out[i] = b.JSON()
	}
	return out
}

func responsesJSON(r map[string]string) M {
	m := M{}
	for k, v := range r {
		m[k] = v
	}
	return m
}

// ContentStamp identifies the card's content, to tell whether it changed.
func (c *Card) ContentStamp() string {
	return Encode(M{"type": c.Type, "text": c.Text, "path": c.Path, "source": c.Source, "page": c.Page,
		"blocks": blocksJSON(c.Blocks), "responses": responsesJSON(c.Responses)})
}

// ContentText is the card's text for the assistant, with the answers.
func (c *Card) ContentText() string {
	value := c.Text
	if c.Generated() {
		parts := make([]string, len(c.Blocks))
		for i, b := range c.Blocks {
			parts[i] = b.PlainText()
		}
		value = join(parts, "\n")
	}
	if len(c.Responses) == 0 {
		return value
	}
	keys := make([]string, 0, len(c.Responses))
	for k := range c.Responses {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = k + ": " + c.Responses[k]
	}
	return value + "\n用户作答\n" + join(lines, "\n")
}

// CardFrom is StudyCard.fromJson.
func CardFrom(m M) *Card {
	doc := m["document_id"]
	if doc == nil {
		doc = m["source"]
	}
	c := &Card{
		ID: str(m["id"]), Type: str(m["type"]), Layer: str(m["layer"]),
		Rect:  RectLTWH(num(m["x"], 0), num(m["y"], 0), num(m["w"], 600), num(m["h"], 440)),
		Title: strOr(m["title"]), Text: strOr(m["text"]), Path: strOr(m["path"]), Source: strOr(m["source"]),
		DocumentID: strOr(doc), Page: toInt(m["page"]), PageCount: toInt(m["page_count"]),
		PageAspect: num(m["page_aspect"], .75), TextBoxes: list(m["text_boxes"]),
		Generation: strOr(m["generation"]), ConversationID: strOr(m["conversation_id"]),
		SourceLectureID: strOr(m["source_lecture_id"]), OriginalPath: strOr(m["original_path"]),
		PDFColumn: strOr(m["pdf_column"]), Collapsed: m["collapsed"] == true, ExpandedH: num(m["expanded_h"], 0),
		QuarterTurns: toInt(m["quarter_turns"]), Responses: map[string]string{},
	}
	for k, v := range obj(m["responses"]) {
		c.Responses[k] = str(v)
	}
	for _, b := range list(m["blocks"]) {
		c.Blocks = append(c.Blocks, BlockFrom(obj(b)))
	}
	for _, a := range list(m["sources"]) {
		c.Sources = append(c.Sources, AnchorFrom(obj(a)))
	}
	return c
}

// Copy is a deep copy through JSON.
func (c *Card) Copy() *Card { return CardFrom(obj(Clone(c.JSON()))) }

func (c *Card) JSON() M {
	sources := make([]any, len(c.Sources))
	for i, a := range c.Sources {
		sources[i] = a.JSON()
	}
	m := M{
		"id": c.ID, "type": c.Type, "layer": c.Layer,
		"x": c.Rect.L, "y": c.Rect.T, "w": c.Rect.W(), "h": c.Rect.H(),
		"title": c.Title, "text": c.Text, "path": c.Path, "source": c.Source, "document_id": c.DocumentID,
		"page": c.Page, "page_count": c.PageCount, "page_aspect": c.PageAspect, "text_boxes": nonNil(c.TextBoxes),
		"generation": c.Generation, "conversation_id": c.ConversationID, "source_lecture_id": c.SourceLectureID,
		"original_path": c.OriginalPath, "collapsed": c.Collapsed, "expanded_h": c.ExpandedH,
		"responses": responsesJSON(c.Responses), "blocks": blocksJSON(c.Blocks), "sources": sources,
	}
	if c.PDFColumn != "" {
		m["pdf_column"] = c.PDFColumn
	}
	if c.QuarterTurns != 0 {
		m["quarter_turns"] = c.QuarterTurns
	}
	return m
}

// InkPoint is a sample of a stroke with its pressure.
type InkPoint struct {
	P        Pt
	Pressure float64
}

// Stroke is StudyStroke: ink, immutable once made, so that undo snapshots
// share it.
type Stroke struct {
	ID, Layer, Tool, Anchor, Gesture string
	Page                             int
	Points                           []InkPoint
	Color                            uint32
	Width                            float64
	Brush                            *Brush
	bounds                           *Rect
	// Cache is the view's: what it drew the stroke into, if anything.
	Cache any
}

// Bounds is the stroke's points grown by its width.
func (s *Stroke) Bounds() Rect {
	if s.bounds == nil {
		r := RectCircle(s.Points[0].P, s.Width)
		for _, p := range s.Points {
			r = r.Union(RectCircle(p.P, s.Width))
		}
		s.bounds = &r
	}
	return *s.bounds
}

// With is a new stroke of s's style and gesture with other points.
func (s *Stroke) With(id string, points []InkPoint) *Stroke {
	return &Stroke{ID: id, Layer: s.Layer, Tool: s.Tool, Anchor: s.Anchor, Gesture: s.Gesture, Page: s.Page,
		Points: points, Color: s.Color, Width: s.Width, Brush: s.Brush}
}

func (s *Stroke) JSON() M {
	points := make([]any, len(s.Points))
	for i, p := range s.Points {
		points[i] = []float64{p.P.X, p.P.Y, p.Pressure}
	}
	m := M{"id": s.ID, "layer": s.Layer, "anchor": s.Anchor, "page": s.Page, "gesture": s.Gesture,
		"color": s.Color, "width": s.Width, "tool": s.Tool, "points": points}
	if s.Brush != nil {
		m["brush"] = s.Brush.JSON()
	}
	return m
}

// StrokeFrom is StudyStroke.fromJson.
func StrokeFrom(m M) (*Stroke, error) {
	color, ok := m["color"].(float64)
	if !ok {
		return nil, errors.New("stroke color")
	}
	points, ok := m["points"].([]any)
	if !ok {
		return nil, errors.New("stroke points")
	}
	s := &Stroke{ID: str(m["id"]), Layer: str(m["layer"]), Anchor: strOr(m["anchor"]), Page: toInt(m["page"]),
		Color: uint32(int64(color)), Width: num(m["width"], 3), Tool: str(m["tool"])}
	if g, ok := m["gesture"].(string); ok {
		s.Gesture = g
	} else {
		s.Gesture = s.ID
	}
	if b, ok := m["brush"].(M); ok {
		br := BrushFrom(b)
		s.Brush = &br
	}
	for _, p := range points {
		v := list(p)
		if len(v) < 2 {
			return nil, errors.New("stroke point")
		}
		pr := .5
		if len(v) > 2 {
			pr = num(v[2], .5)
		}
		s.Points = append(s.Points, InkPoint{Pt{num(v[0], 0), num(v[1], 0)}, pr})
	}
	return s, nil
}

// InkOnDark is the colour ink takes on dark paper: dark ink turns light
// with its hue kept, so notes written on white paper stay legible in dark
// mode. Saved colours never change; highlighters and light ink stay.
func InkOnDark(color uint32, tool string) uint32 {
	if tool == "highlighter" {
		return color
	}
	h, s, l, a := hsl(color)
	if l >= .55 {
		return color
	}
	return fromHSL(h, s, clamp(1-l, .74, .92), a)
}

// hsl is HSLColor.fromColor.
func hsl(c uint32) (h, s, l, a float64) {
	a = float64(c>>24) / 255
	r, g, b := float64(c>>16&0xff)/255, float64(c>>8&0xff)/255, float64(c&0xff)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	d := mx - mn
	switch {
	case mx == 0:
	case mx == r:
		h = 60 * dmod((g-b)/d, 6)
	case mx == g:
		h = 60 * ((b-r)/d + 2)
	default:
		h = 60 * ((r-g)/d + 4)
	}
	if math.IsNaN(h) {
		h = 0
	}
	l = (mx + mn) / 2
	if l != 1 {
		s = clamp(d/(1-math.Abs(2*l-1)), 0, 1)
	}
	return
}

// dmod is Dart's % on doubles: never negative for a positive divisor.
func dmod(a, b float64) float64 {
	m := math.Mod(a, b)
	if m < 0 {
		m += b
	}
	return m
}

// fromHSL is HSLColor.toColor.
func fromHSL(h, s, l, a float64) uint32 {
	chroma := (1 - math.Abs(2*l-1)) * s
	second := chroma * (1 - math.Abs(dmod(h/60, 2)-1))
	match := l - chroma/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g = chroma, second
	case h < 120:
		r, g = second, chroma
	case h < 180:
		g, b = chroma, second
	case h < 240:
		g, b = second, chroma
	case h < 300:
		r, b = second, chroma
	default:
		r, b = chroma, second
	}
	ch := func(v float64) uint32 { return uint32(math.Round(v * 255)) }
	return ch(a)<<24 | ch(r+match)<<16 | ch(g+match)<<8 | ch(b+match)
}

// OnPrintedPage reports whether ink anchored to a card of type sits on a
// printed page (PDF, image) that keeps its light background in dark mode.
func OnPrintedPage(typ string) bool { return typ == "pdf" || typ == "pdfdoc" || typ == "image" }

type snapshot struct {
	appliedRuns   []string
	layers        []Layer
	cards         []*Card
	strokes       []*Stroke
	connections   []Connection
	pluginObjects []*PluginObject
}

func takeSnapshot(d *Document) snapshot {
	s := snapshot{appliedRuns: append([]string(nil), d.AppliedRuns...), layers: append([]Layer(nil), d.Layers...),
		strokes: append([]*Stroke(nil), d.Strokes...), connections: append([]Connection(nil), d.Connections...),
		pluginObjects: append([]*PluginObject(nil), d.PluginObjects...)}
	for _, c := range d.Cards {
		s.cards = append(s.cards, c.Copy())
	}
	return s
}

// Document is StudyDocument: the whole canvas of one lecture.
type Document struct {
	Revision, Edit, FormatVersion int
	// PluginRevision is the plugin-object revision a plugin batch is
	// checked against. It moves on every change to PluginObjects, undo and
	// redo included, and is not saved.
	PluginRevision int
	BaseHash       string
	// AppliedRuns are the assistant runs already applied, oldest first.
	AppliedRuns []string
	Camera      Pt
	Zoom        float64
	ActiveLayer string
	Layers      []Layer
	Cards       []*Card
	Strokes     []*Stroke
	Connections []Connection
	// PluginObjects are persistent plugin output, saved as plugin_objects
	// in format 5. The canvas draws them and never hit-tests them.
	PluginObjects []*PluginObject
	undo, redo    []snapshot
	// OnChange runs after every change (notifyListeners).
	OnChange func()
}

// NewDocument is an empty canvas.
func NewDocument() *Document {
	return &Document{FormatVersion: 3, Camera: Pt{40, 70}, Zoom: .75, ActiveLayer: "main",
		Layers: []Layer{{ID: "main", Name: l10n.StudyDefaultLayer, Visible: true}}}
}

func (d *Document) CanUndo() bool { return len(d.undo) > 0 }
func (d *Document) CanRedo() bool { return len(d.redo) > 0 }

// Layer is the layer of id.
func (d *Document) Layer(id string) *Layer {
	for i := range d.Layers {
		if d.Layers[i].ID == id {
			return &d.Layers[i]
		}
	}
	return nil
}

// Visible reports whether layer id shows.
func (d *Document) Visible(id string) bool {
	l := d.Layer(id)
	return l != nil && l.Visible
}

// Editable reports whether layer id shows and is not locked.
func (d *Document) Editable(id string) bool {
	l := d.Layer(id)
	return l != nil && l.Visible && !l.Locked
}

// Card is the card of id, or nil.
func (d *Document) Card(id string) *Card {
	for _, c := range d.Cards {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// StrokeVisible reports whether s shows: its layer visible, its card open
// on its page.
func (d *Document) StrokeVisible(s *Stroke) bool {
	if !d.Visible(s.Layer) {
		return false
	}
	c := d.Card(s.Anchor)
	return c == nil || (!c.Collapsed && (s.Page == 0 || c.Page == s.Page))
}

// CanTransformCard reports whether card and the ink on it may move.
func (d *Document) CanTransformCard(c *Card) bool {
	if !d.Editable(c.Layer) {
		return false
	}
	for _, s := range d.Strokes {
		if s.Anchor == c.ID {
			if l := d.Layer(s.Layer); l != nil && l.Locked {
				return false
			}
		}
	}
	return true
}

// DocumentFrom is StudyDocument.fromJson.
func DocumentFrom(m M) (d *Document, err error) {
	v, ok := isInt(m["version"])
	if !ok || v < 1 || v > 5 {
		return nil, errors.New(l10n.StudyUnknownVersion)
	}
	defer func() {
		if r := recover(); r != nil {
			d, err = nil, errors.New(l10n.StudyUnknownVersion)
		}
	}()
	d = NewDocument()
	d.FormatVersion = max(3, v)
	d.Revision = toInt(m["revision"])
	d.BaseHash, _ = m["base_hash"].(string)
	d.AppliedRuns = strs(m["applied_runs"])
	c := m["camera"].(M)
	d.Camera = Pt{num(c["x"], 0), num(c["y"], 0)}
	d.Zoom = clamp(num(c["z"], 1), .05, 8)
	d.Layers = nil
	for _, a := range m["layers"].([]any) {
		l := a.(M)
		d.Layers = append(d.Layers, Layer{ID: str(l["id"]), Name: str(l["name"]), Visible: l["visible"] == true, Locked: l["locked"] == true})
	}
	d.ActiveLayer = d.Layers[0].ID
	for _, a := range m["objects"].([]any) {
		d.Cards = append(d.Cards, CardFrom(a.(M)))
	}
	for _, a := range list(m["connections"]) {
		d.Connections = append(d.Connections, ConnectionFrom(obj(a)))
	}
	// Older files may contain long strokes; normalize their render and
	// erase work into bounded chunks.
	for _, a := range m["strokes"].([]any) {
		s, err := StrokeFrom(a.(M))
		if err != nil {
			return nil, err
		}
		d.Strokes = append(d.Strokes, Chunks(s)...)
	}
	for _, a := range list(m["plugin_objects"]) {
		o, err := PluginObjectFrom(a)
		if err != nil {
			return nil, err
		}
		d.PluginObjects = append(d.PluginObjects, o)
	}
	return d, nil
}

func (d *Document) JSON() M {
	version := 3
	for _, c := range d.Cards {
		if c.QuarterTurns != 0 {
			version = 4
		}
	}
	if len(d.PluginObjects) > 0 {
		version = 5
	}
	layers := make([]any, len(d.Layers))
	for i, l := range d.Layers {
		layers[i] = l.JSON()
	}
	cards := make([]any, len(d.Cards))
	for i, c := range d.Cards {
		cards[i] = c.JSON()
	}
	strokes := make([]any, len(d.Strokes))
	for i, s := range d.Strokes {
		strokes[i] = s.JSON()
	}
	links := make([]any, len(d.Connections))
	for i, c := range d.Connections {
		links[i] = c.JSON()
	}
	m := M{
		"version": max(d.FormatVersion, version), "revision": d.Revision, "applied_runs": nonNil(d.AppliedRuns),
		"camera": M{"x": d.Camera.X, "y": d.Camera.Y, "z": d.Zoom},
		"layers": layers, "objects": cards, "strokes": strokes, "connections": links,
	}
	if d.BaseHash != "" {
		m["base_hash"] = d.BaseHash
	}
	if len(d.PluginObjects) > 0 {
		objs := make([]any, len(d.PluginObjects))
		for i, o := range d.PluginObjects {
			objs[i] = o.JSON()
		}
		m["plugin_objects"] = objs
	}
	return m
}

// Chunks splits a long stroke into pieces of 64 segments, so drawing and
// erasing never redo a long prefix.
func Chunks(s *Stroke) []*Stroke {
	if len(s.Points) <= 65 {
		return []*Stroke{s}
	}
	var out []*Stroke
	for i := 0; i < len(s.Points)-1; i += 64 {
		out = append(out, s.With(s.ID+"-"+strconv.Itoa(i), s.Points[i:min(i+65, len(s.Points))]))
	}
	return out
}

// Checkpoint records the state for undo.
func (d *Document) Checkpoint() {
	d.undo = append(d.undo, takeSnapshot(d))
	if len(d.undo) > 40 {
		d.undo = d.undo[1:]
	}
	d.redo = nil
}

// Changed counts an edit and tells the listener.
func (d *Document) Changed() {
	d.Edit++
	if d.OnChange != nil {
		d.OnChange()
	}
}

func (d *Document) restore(s snapshot) {
	d.AppliedRuns = append([]string(nil), s.appliedRuns...)
	d.Layers = append([]Layer(nil), s.layers...)
	d.Cards = nil
	for _, c := range s.cards {
		d.Cards = append(d.Cards, c.Copy())
	}
	d.Strokes = append([]*Stroke(nil), s.strokes...)
	d.Connections = append([]Connection(nil), s.connections...)
	same := len(s.pluginObjects) == len(d.PluginObjects)
	for i := 0; same && i < len(s.pluginObjects); i++ {
		same = s.pluginObjects[i] == d.PluginObjects[i]
	}
	if !same {
		d.PluginRevision++
	}
	d.PluginObjects = append([]*PluginObject(nil), s.pluginObjects...)
	if d.Layer(d.ActiveLayer) == nil {
		d.ActiveLayer = d.Layers[0].ID
	}
	d.Changed()
}

// Undo steps back one checkpoint.
func (d *Document) Undo() {
	if !d.CanUndo() {
		return
	}
	d.redo = append(d.redo, takeSnapshot(d))
	s := d.undo[len(d.undo)-1]
	d.undo = d.undo[:len(d.undo)-1]
	d.restore(s)
}

// Redo steps forward one undone checkpoint.
func (d *Document) Redo() {
	if !d.CanRedo() {
		return
	}
	d.undo = append(d.undo, takeSnapshot(d))
	s := d.redo[len(d.redo)-1]
	d.redo = d.redo[:len(d.redo)-1]
	d.restore(s)
}

// AddStroke adds s as one undo entry.
func (d *Document) AddStroke(s *Stroke) {
	if len(s.Points) == 0 {
		return
	}
	d.Checkpoint()
	d.Strokes = append(d.Strokes, Chunks(s)...)
	d.Changed()
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// TransformCards maps the movable cards of ids, and the ink on them, from
// one rectangle to another.
func (d *Document) TransformCards(ids map[string]bool, from, to Rect) {
	movable := map[string]bool{}
	for _, c := range d.Cards {
		if ids[c.ID] && d.CanTransformCard(c) {
			movable[c.ID] = true
		}
	}
	sx, sy := to.W()/from.W(), to.H()/from.H()
	move := func(p Pt) Pt { return Pt{to.L + (p.X-from.L)*sx, to.T + (p.Y-from.T)*sy} }
	for _, c := range d.Cards {
		if movable[c.ID] {
			c.Rect = RectPts(move(c.Rect.TopLeft()), move(c.Rect.BottomRight()))
		}
	}
	for i, s := range d.Strokes {
		if movable[s.Anchor] {
			n := s.With(s.ID, make([]InkPoint, len(s.Points)))
			for j, p := range s.Points {
				n.Points[j] = InkPoint{move(p.P), p.Pressure}
			}
			n.Width = s.Width * math.Sqrt(sx*sy)
			d.Strokes[i] = n
		}
	}
	d.Changed()
}

// CanRotateSelection reports whether the selection turns: images only,
// open and movable, and editable ink. Quarter turns keep axis-aligned hit
// areas; images turn with their frame, ink about the same pivot.
func (d *Document) CanRotateSelection(cardIDs, inkIDs map[string]bool) bool {
	n := 0
	for _, c := range d.Cards {
		if cardIDs[c.ID] {
			n++
			if c.Type != "image" || c.Collapsed || !d.CanTransformCard(c) {
				return false
			}
		}
	}
	if n == 0 && len(inkIDs) == 0 {
		return false
	}
	for _, s := range d.Strokes {
		if (inkIDs[s.ID] || cardIDs[s.Anchor]) && !d.Editable(s.Layer) {
			return false
		}
	}
	return true
}

// RotateSelection turns the selection a quarter clockwise about pivot.
func (d *Document) RotateSelection(cardIDs, inkIDs map[string]bool, pivot Pt) {
	if !d.CanRotateSelection(cardIDs, inkIDs) {
		return
	}
	turn := func(p Pt) Pt { v := p.Sub(pivot); return pivot.Add(Pt{-v.Y, v.X}) }
	d.Checkpoint()
	for _, c := range d.Cards {
		if cardIDs[c.ID] {
			c.Rect = RectCenter(turn(c.Rect.Center()), c.Rect.H(), c.Rect.W())
			c.QuarterTurns = (c.QuarterTurns + 1) % 4
			d.FormatVersion = 4
		}
	}
	for i, s := range d.Strokes {
		if inkIDs[s.ID] || cardIDs[s.Anchor] {
			n := s.With(s.ID, make([]InkPoint, len(s.Points)))
			for j, p := range s.Points {
				n.Points[j] = InkPoint{turn(p.P), p.Pressure}
			}
			d.Strokes[i] = n
		}
	}
	d.Changed()
}

// Erase cuts the editable, visible ink along segment ab within radius,
// keeping the pieces either side.
func (d *Document) Erase(a, b Pt, radius float64) bool {
	hit := RectPts(a, b).Inflate(radius)
	touched := false
	var next []*Stroke
	for _, s := range d.Strokes {
		if !d.Editable(s.Layer) || !d.StrokeVisible(s) || !s.Bounds().Overlaps(hit) {
			next = append(next, s)
			continue
		}
		var fragments [][]InkPoint
		var fragment []InkPoint
		for i, p := range s.Points {
			erased := SegmentDistance(p.P, a, b) <= radius+s.Width/2
			cross := i > 0 && segmentsNear(s.Points[i-1].P, p.P, a, b, radius+s.Width/2)
			if erased || cross {
				if len(fragment) > 0 {
					fragments = append(fragments, fragment)
				}
				fragment = nil
				touched = true
			}
			if !erased {
				fragment = append(fragment, p)
			}
		}
		if len(fragment) > 0 {
			fragments = append(fragments, fragment)
		}
		for _, f := range fragments {
			next = append(next, s.With(NewID(), f))
		}
	}
	if touched {
		d.Strokes = next
		d.Changed()
	}
	return touched
}

func (d *Document) removeGestures(gestures map[string]bool) {
	var keep []*Stroke
	for _, s := range d.Strokes {
		if !(d.Editable(s.Layer) && gestures[s.Gesture]) {
			keep = append(keep, s)
		}
	}
	d.Strokes = keep
}

// EraseWhole removes whole physical strokes the segment ab touches.
func (d *Document) EraseWhole(a, b Pt, radius float64) bool {
	hit := RectPts(a, b).Inflate(radius)
	gestures := map[string]bool{}
	for _, s := range d.Strokes {
		if !d.Editable(s.Layer) || !d.StrokeVisible(s) || !s.Bounds().Overlaps(hit) {
			continue
		}
		if len(s.Points) == 1 && SegmentDistance(s.Points[0].P, a, b) <= radius+s.Width/2 {
			gestures[s.Gesture] = true
		}
		for i := 1; i < len(s.Points); i++ {
			if segmentsNear(s.Points[i-1].P, s.Points[i].P, a, b, radius+s.Width/2) {
				gestures[s.Gesture] = true
				break
			}
		}
	}
	if len(gestures) == 0 {
		return false
	}
	d.removeGestures(gestures)
	d.Changed()
	return true
}

// EraseLasso removes the whole strokes with a point inside polygon.
func (d *Document) EraseLasso(polygon []Pt) bool {
	if len(polygon) < 3 {
		return false
	}
	bounds := Bounds(polygon)
	hit := map[string]bool{}
	for _, s := range d.Strokes {
		if !d.Editable(s.Layer) || !d.StrokeVisible(s) || !s.Bounds().Overlaps(bounds) {
			continue
		}
		for _, p := range s.Points {
			if InPolygon(p.P, polygon) {
				hit[s.Gesture] = true
				break
			}
		}
	}
	if len(hit) == 0 {
		return false
	}
	d.removeGestures(hit)
	d.Changed()
	return true
}

// StrokesInPolygon selects whole physical strokes whose painted outline
// meets the loop, sparse segments crossing it included.
func (d *Document) StrokesInPolygon(polygon []Pt) map[string]bool {
	out := map[string]bool{}
	if len(polygon) < 3 {
		return out
	}
	region := Bounds(polygon)
	hit := map[string]bool{}
	for _, s := range d.Strokes {
		if d.StrokeVisible(s) && s.Bounds().Overlaps(region) && StrokeMeetsPolygon(s, polygon) {
			hit[s.Gesture] = true
		}
	}
	for _, s := range d.Strokes {
		if d.StrokeVisible(s) && hit[s.Gesture] {
			out[s.ID] = true
		}
	}
	return out
}

// SegmentRadius is the half width the painted segment ending at point i
// takes, as PaintInk draws it.
func (s *Stroke) SegmentRadius(i int) float64 {
	pressure := (s.Points[i-1].Pressure + s.Points[i].Pressure) / 2
	scale := .35 + .65*pressure
	if s.Tool == "highlighter" {
		scale = 1
	} else if s.Brush != nil {
		scale = s.Brush.RadiusScale(pressure)
	}
	return s.Width / 2 * scale
}

// StrokeMeetsPolygon reports whether the filled outline of s, as
// studyStrokeOutline traces it, shares area with polygon.
func StrokeMeetsPolygon(s *Stroke, polygon []Pt) bool {
	capsule := func(a, b Pt, r float64) bool {
		if InPolygon(a, polygon) || InPolygon(b, polygon) {
			return true
		}
		for i, j := 0, len(polygon)-1; i < len(polygon); j, i = i, i+1 {
			if segmentsNear(polygon[j], polygon[i], a, b, r) {
				return true
			}
		}
		return false
	}
	if len(s.Points) == 1 {
		return capsule(s.Points[0].P, s.Points[0].P, s.Width/2)
	}
	square := s.Brush != nil && s.Brush.SquareTip
	for i := 1; i < len(s.Points); i++ {
		a, b := s.Points[i-1].P, s.Points[i].P
		r := s.SegmentRadius(i)
		if delta := b.Sub(a); square && delta.Dist() > 0 {
			t := delta.Div(delta.Dist()).Mul(r)
			if s.Tool != "highlighter" || i == 1 {
				a = a.Sub(t)
			}
			if s.Tool != "highlighter" || i == len(s.Points)-1 {
				b = b.Add(t)
			}
		}
		if capsule(a, b, r) {
			return true
		}
	}
	return false
}

// TransformInk maps the editable strokes of ids from one rectangle to
// another, recoloured or resized when asked. The caller owns the gesture's
// checkpoint; stroke ids survive.
func (d *Document) TransformInk(ids map[string]bool, from, to Rect, color *uint32, width *float64) {
	sx, sy := to.W()/math.Max(.001, from.W()), to.H()/math.Max(.001, from.H())
	touched := false
	for i, s := range d.Strokes {
		if !ids[s.ID] || !d.Editable(s.Layer) {
			continue
		}
		touched = true
		n := s.With(s.ID, make([]InkPoint, len(s.Points)))
		for j, p := range s.Points {
			n.Points[j] = InkPoint{Pt{to.L + (p.P.X-from.L)*sx, to.T + (p.P.Y-from.T)*sy}, p.Pressure}
		}
		if color != nil {
			n.Color = *color
		}
		w := s.Width * math.Sqrt(sx*sy)
		if width != nil {
			w = *width
		}
		n.Width = clamp(w, .2, 80)
		d.Strokes[i] = n
	}
	if touched {
		d.Changed()
	}
}

// ContextText is the text of the visible cards of ids, or all, for the
// assistant: at most 12000 characters.
func (d *Document) ContextText(ids map[string]bool) string {
	out := []rune("画布材料（文字内容，不含手写识别）：\n")
	for _, c := range d.Cards {
		if !d.Visible(c.Layer) || (len(ids) > 0 && !ids[c.ID]) {
			continue
		}
		out = append(out, []rune(c.Title+"\n"+c.Text+"\n")...)
		if utf16Len(out) > 12000 {
			break
		}
	}
	return string(truncUTF16(out, 12000))
}

// SelectionKind is StudySelectionKind: what a selection holds.
type SelectionKind int

const (
	SelectInk SelectionKind = iota
	SelectText
	SelectDocument
	SelectVideo
	SelectOther
)

// SelectionKinds are every kind.
var SelectionKinds = []SelectionKind{SelectInk, SelectText, SelectDocument, SelectVideo, SelectOther}

// CardSelectionKind is studyCardSelectionKind.
func CardSelectionKind(c *Card) SelectionKind {
	switch c.Type {
	case "text", "note", "explanation", "knowledge", "practice":
		return SelectText
	case "pdfdoc", "pdf", "image":
		return SelectDocument
	case "lecture", "video":
		return SelectVideo
	}
	return SelectOther
}
