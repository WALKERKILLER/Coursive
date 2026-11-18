package study

// Anchor is StudyAnchor (study_content.dart): UTF-16 ranges match
// Dart/JavaScript selection offsets; regions are object-normalized.
type Anchor struct {
	ID, ObjectID, Kind, WorkspaceID, DocumentID, SourceLectureID, OriginalPath string
	Fingerprint, Quote                                                         string
	Page, Start, End, TimeStartMs, TimeEndMs                                   int
	Region                                                                     *Rect
	StrokeIDs                                                                  []string
}

func (a Anchor) JSON() M {
	m := M{
		"id": a.ID, "object_id": a.ObjectID, "kind": a.Kind, "workspace_id": a.WorkspaceID,
		"document_id": a.DocumentID, "source_lecture_id": a.SourceLectureID, "original_path": a.OriginalPath,
		"page": a.Page, "start": a.Start, "end": a.End, "time_start_ms": a.TimeStartMs, "time_end_ms": a.TimeEndMs,
		"fingerprint": a.Fingerprint, "quote": a.Quote, "stroke_ids": nonNil(a.StrokeIDs),
	}
	if a.Region != nil {
		m["region"] = []float64{a.Region.L, a.Region.T, a.Region.W(), a.Region.H()}
	}
	return m
}

// AnchorFrom is StudyAnchor.fromJson.
func AnchorFrom(m M) Anchor {
	kind := str(m["kind"])
	if kind == "pdfdoc" {
		kind = "pdf"
	}
	a := Anchor{
		ID: str(m["id"]), ObjectID: str(m["object_id"]), Kind: kind,
		WorkspaceID: strOr(m["workspace_id"]), DocumentID: strOr(m["document_id"]),
		SourceLectureID: strOr(m["source_lecture_id"]), OriginalPath: strOr(m["original_path"]),
		Page: toInt(m["page"]), Start: toInt(m["start"]), End: toInt(m["end"]),
		TimeStartMs: toInt(m["time_start_ms"]), TimeEndMs: toInt(m["time_end_ms"]),
		Fingerprint: strOr(m["fingerprint"]), Quote: strOr(m["quote"]), StrokeIDs: strs(m["stroke_ids"]),
	}
	if r := list(m["region"]); len(r) == 4 {
		rr := RectLTWH(num(r[0], 0), num(r[1], 0), num(r[2], 0), num(r[3], 0))
		a.Region = &rr
	}
	return a
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// Block is StudyBlock: one block of a generated card.
type Block struct {
	Type, Text, Language, Hint, Answer string
	Items, Columns                     []string
	Rows                               [][]string
}

func (b Block) JSON() M {
	rows := make([]any, len(b.Rows))
	for i, r := range b.Rows {
		rows[i] = nonNil(r)
	}
	return M{"type": b.Type, "text": b.Text, "items": nonNil(b.Items), "columns": nonNil(b.Columns),
		"rows": rows, "language": b.Language, "hint": b.Hint, "answer": b.Answer}
}

// BlockFrom is StudyBlock.fromJson.
func BlockFrom(m M) Block {
	b := Block{Type: str(m["type"]), Text: strOr(m["text"]), Items: strs(m["items"]), Columns: strs(m["columns"]),
		Language: strOr(m["language"]), Hint: strOr(m["hint"]), Answer: strOr(m["answer"])}
	for _, r := range list(m["rows"]) {
		b.Rows = append(b.Rows, strs(r))
	}
	return b
}

// PlainText is the block's text, one part a line.
func (b Block) PlainText() string {
	parts := []string{b.Text}
	parts = append(parts, b.Items...)
	parts = append(parts, join(b.Columns, " | "))
	for _, r := range b.Rows {
		parts = append(parts, join(r, " | "))
	}
	parts = append(parts, b.Hint, b.Answer)
	var out []string
	for _, s := range parts {
		if s != "" {
			out = append(out, s)
		}
	}
	return join(out, "\n")
}

func join(s []string, sep string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += sep
		}
		out += v
	}
	return out
}

// Connection is StudyConnection: a labelled link between two objects.
type Connection struct{ ID, From, To, Label string }

func (c Connection) JSON() M { return M{"id": c.ID, "from": c.From, "to": c.To, "label": c.Label} }

// ConnectionFrom is StudyConnection.fromJson.
func ConnectionFrom(m M) Connection {
	return Connection{str(m["id"]), str(m["from"]), str(m["to"]), str(m["label"])}
}

// Selection is StudySelection: a selected range or region of one object
// and the ink in it.
type Selection struct {
	ObjectID   string
	Start, End int
	Region     *Rect
	StrokeIDs  map[string]bool
}

// Clear empties the selection.
func (s *Selection) Clear() {
	s.ObjectID, s.Start, s.End, s.Region = "", 0, 0, nil
	s.StrokeIDs = map[string]bool{}
}

func (s *Selection) JSON() M {
	ids := []string{}
	for id := range s.StrokeIDs {
		ids = append(ids, id)
	}
	m := M{"object_id": s.ObjectID, "start": s.Start, "end": s.End, "stroke_ids": ids}
	if s.Region != nil {
		m["region"] = []float64{s.Region.L, s.Region.T, s.Region.W(), s.Region.H()}
	}
	return m
}
