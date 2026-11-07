package library

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)

// Conversation history is independent of layout undo; deleting a card retains its recoverable sidecar.
type StudyTurn struct {
	ID         string          `json:"id"`
	Task       string          `json:"task,omitempty"`
	Question   string          `json:"question"`
	Answer     string          `json:"answer"`
	ContextKey string          `json:"context_key"`
	State      string          `json:"state"`
	Error      string          `json:"error,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
}

// StudyEvidence is one citation the assistant host issued for a passage,
// transcript line or inspected region it read in a run (spec 0027).
type StudyEvidence struct {
	ID       string      `json:"id"`
	SourceID string      `json:"source_id"`
	Kind     string      `json:"kind"`
	Status   string      `json:"status"`
	Label    string      `json:"label,omitempty"`
	Anchor   StudyAnchor `json:"anchor"`
}

// StudySuggestion is a next step the learner may choose; it runs nothing.
type StudySuggestion struct {
	Kind       string `json:"kind"`
	EvidenceID string `json:"evidence_id,omitempty"`
	Text       string `json:"text,omitempty"`
}

// StudyTurnResult is the part of a turn result the daemon understands.
// Older turns carry only sources and keep loading unchanged.
type StudyTurnResult struct {
	SchemaVersion   int                   `json:"schema_version,omitempty"`
	Evidence        []StudyEvidence       `json:"evidence,omitempty"`
	Suggestions     []StudySuggestion     `json:"suggestions,omitempty"`
	OutlineProposal *StudyOutlineProposal `json:"outline_proposal,omitempty"`
}

// StudyOutlineProposal is a review outline drafted from bound material
// (spec 0028). It stays a draft in the conversation until the learner saves
// it as a lecture note; only then is it accepted with that note's ID.
type StudyOutlineProposal struct {
	Version             int                   `json:"version"`
	ProposalID          string                `json:"proposal_id"`
	LectureID           string                `json:"lecture_id"`
	ConversationID      string                `json:"conversation_id"`
	TurnID              string                `json:"turn_id"`
	SourceRevision      int64                 `json:"source_revision"`
	SourceFingerprints  []string              `json:"source_fingerprints,omitempty"`
	State               string                `json:"state"`
	Title               string                `json:"title"`
	Sections            []StudyOutlineSection `json:"sections"`
	Conflicts           []StudyOutlineNote    `json:"conflicts,omitempty"`
	UnresolvedQuestions []string              `json:"unresolved_questions,omitempty"`
	MissingSources      []string              `json:"missing_sources,omitempty"`
	NoteID              string                `json:"note_id,omitempty"`
	NoteHash            string                `json:"note_hash,omitempty"`
}
type StudyOutlineSection struct {
	ID      string             `json:"id"`
	Heading string             `json:"heading"`
	Level   int                `json:"level"`
	Items   []StudyOutlineItem `json:"items"`
}
type StudyOutlineItem struct {
	ID          string       `json:"id"`
	Kind        string       `json:"kind"`
	Text        string       `json:"text"`
	EvidenceIDs []string     `json:"evidence_ids,omitempty"`
	Review      string       `json:"review"`
	SourceID    string       `json:"source_id,omitempty"`
	Anchor      *StudyAnchor `json:"anchor,omitempty"`
	Choice      string       `json:"choice,omitempty"`
	ChoiceText  string       `json:"choice_text,omitempty"`
}
type StudyOutlineNote struct {
	Text        string   `json:"text"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

func validStudyTurnResult(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return true
	}
	var r StudyTurnResult
	if json.Unmarshal(raw, &r) != nil || r.SchemaVersion < 0 || r.SchemaVersion > 2 || len(r.Suggestions) > 2 {
		return false
	}
	// An outline cites more of the bound material than one answer does.
	limit := 24
	if r.OutlineProposal != nil {
		limit = 120
	}
	if len(r.Evidence) > limit {
		return false
	}
	issued := map[string]bool{}
	for _, e := range r.Evidence {
		if !studyEvidenceID.MatchString(e.ID) || issued[e.ID] || len(e.SourceID) > 120 || len(e.Label) > 400 || !validStudyAnchor(e.Anchor) {
			return false
		}
		issued[e.ID] = true
		switch e.Kind {
		case "exact_text", "timestamped_transcript", "visual_observation":
		default:
			return false
		}
		switch e.Status {
		case "verified", "stale", "unavailable":
		default:
			return false
		}
	}
	for _, s := range r.Suggestions {
		switch {
		case s.Kind == "open_source" && issued[s.EvidenceID] && s.Text == "":
		case s.Kind == "ask_followup" && s.EvidenceID == "" && s.Text != "" && len(s.Text) <= 600:
		case s.Kind == "propose_outline" && s.EvidenceID == "" && len(s.Text) <= 600:
		default:
			return false
		}
	}
	return r.OutlineProposal == nil || validStudyOutline(*r.OutlineProposal, issued)
}

func validStudyOutline(p StudyOutlineProposal, issued map[string]bool) bool {
	cited := func(ids []string) bool {
		if len(ids) > 24 {
			return false
		}
		for _, id := range ids {
			if !issued[id] {
				return false
			}
		}
		return true
	}
	if p.Version != 1 || !studyProposalID.MatchString(p.ProposalID) || len(p.LectureID) > 100 || len(p.ConversationID) > 100 || len(p.TurnID) > 100 || p.SourceRevision < 0 ||
		len(p.SourceFingerprints) > 24 || strings.TrimSpace(p.Title) == "" || len(p.Title) > 400 || len(p.Sections) > 50 ||
		len(p.Conflicts) > 20 || len(p.UnresolvedQuestions) > 20 || len(p.MissingSources) > 24 {
		return false
	}
	// A draft is never linked to a note; an accepted outline always is.
	switch {
	case p.State == "draft" && p.NoteID == "" && p.NoteHash == "":
	case p.State == "accepted" && studyOutlineID.MatchString(p.NoteID) && studyNoteHash.MatchString(p.NoteHash):
	default:
		return false
	}
	for _, f := range p.SourceFingerprints {
		if len(f) > 128 {
			return false
		}
	}
	ids := map[string]bool{}
	fresh := func(id string) bool {
		if !studyOutlineID.MatchString(id) || ids[id] {
			return false
		}
		ids[id] = true
		return true
	}
	for _, s := range p.Sections {
		if !fresh(s.ID) || len(s.Heading) > 400 || s.Level < 1 || s.Level > 3 || len(s.Items) > 20 {
			return false
		}
		for _, it := range s.Items {
			if !fresh(it.ID) || len(it.Text) > 4000 || !cited(it.EvidenceIDs) || len(it.SourceID) > 120 || len(it.ChoiceText) > 400 {
				return false
			}
			switch it.Review {
			case "pending", "confirmed", "to_check", "needs_interpretation":
			default:
				return false
			}
			switch it.Kind {
			case "fact", "summary", "question":
				if it.Choice != "" || it.ChoiceText != "" || it.Anchor != nil || it.Review == "needs_interpretation" {
					return false
				}
			case "unresolved_mark":
				// Only the learner gives a mark its meaning.
				switch it.Choice {
				case "", "mastered", "key", "review", "custom", "keep":
				default:
					return false
				}
				if (it.Choice == "custom") != (strings.TrimSpace(it.ChoiceText) != "") || (it.Choice == "") != (it.Review == "needs_interpretation") || it.Anchor != nil && !validStudyAnchor(*it.Anchor) {
					return false
				}
			default:
				return false
			}
		}
	}
	for _, c := range p.Conflicts {
		if strings.TrimSpace(c.Text) == "" || len(c.Text) > 2000 || !cited(c.EvidenceIDs) {
			return false
		}
	}
	for _, q := range p.UnresolvedQuestions {
		if len(q) > 1000 {
			return false
		}
	}
	for _, m := range p.MissingSources {
		if len(m) > 400 {
			return false
		}
	}
	return true
}

var (
	studyEvidenceID = regexp.MustCompile(`^e[0-9]{1,3}$`)
	studyProposalID = regexp.MustCompile(`^[0-9a-f][0-9a-f-]{7,63}$`)
	studyOutlineID  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
	studyNoteHash   = regexp.MustCompile(`^[0-9a-f]{16,64}$`)
)

type StudyConversation struct {
	Version     int                 `json:"version"`
	Revision    int64               `json:"revision"`
	ID          string              `json:"id"`
	Draft       string              `json:"draft"`
	Query       StudyContextRequest `json:"query"`
	AllowImages bool                `json:"allow_images"`
	Turns       []StudyTurn         `json:"turns"`
}

func readStudyConversation(root *os.Root, name, id string) (StudyConversation, error) {
	out := StudyConversation{Version: 1, ID: id, Turns: []StudyTurn{}}
	b, e := root.ReadFile(name)
	if os.IsNotExist(e) {
		return out, nil
	}
	if e != nil {
		return out, e
	}
	if len(b) > 4<<20 {
		return out, ErrStudyLimit
	}
	e = json.Unmarshal(b, &out)
	return out, e
}
func (l *Library) StudyConversation(ctx context.Context, lecture, id string, next *StudyConversation) (StudyConversation, error) {
	if !validStudyID(id) {
		return StudyConversation{}, ErrStudyInvalid
	}
	l.studyMu.Lock()
	defer l.studyMu.Unlock()
	root, dir, e := l.studyRoot(ctx, lecture)
	if e != nil {
		return StudyConversation{}, e
	}
	defer root.Close()
	if e = checkedStudyDir(root, dir+"/study-conversations", true); e != nil {
		return StudyConversation{}, e
	}
	name := dir + "/study-conversations/" + id + ".json"
	old, e := readStudyConversation(root, name, id)
	if e != nil || next == nil {
		return old, e
	}
	if next.ID != id || next.Version != 1 || next.Revision < 0 || len(next.Draft) > 16000 || len(next.Turns) > 300 || len(next.Query.Objects) > 12 {
		return old, ErrStudyInvalid
	}
	if next.Revision != old.Revision {
		return old, ErrStudyConflict
	}
	seen := map[string]bool{}
	for _, t := range next.Turns {
		if !validStudyID(t.ID) || seen[t.ID] || len(t.Question) > 16000 || len(t.Answer) > 48000 || len(t.ContextKey) > 128 || len(t.Error) > 4000 || len(t.Result) > 400000 || !validStudyTurnResult(t.Result) {
			return old, ErrStudyInvalid
		}
		seen[t.ID] = true
		if t.Task != "" && t.Task != "propose_outline" {
			return old, ErrStudyInvalid
		}
		switch t.State {
		case "preparing", "running", "done", "error", "cancelled", "interrupted", "pending", "applied":
		default:
			return old, ErrStudyInvalid
		}
	}
	next.Revision++
	b, e := json.Marshal(next)
	if e != nil {
		return old, e
	}
	if len(b) > 4<<20 {
		return old, ErrStudyLimit
	}
	if e = writeStudyBytes(root, name, b); e != nil {
		return old, e
	}
	return *next, nil
}

// Course material selection must resolve an indexed file in the workspace's own course.
// Original bytes are never changed. External imports still use the existing managed asset boundary.
func (l *Library) ImportStudyMaterial(ctx context.Context, lecture, rel string) (StudyImport, error) {
	var out StudyImport
	lec, e := l.Store.Lecture(ctx, lecture)
	if e != nil {
		return out, e
	}
	items, e := l.CourseCatalog(ctx, lec.CourseID)
	if e != nil {
		return out, e
	}
	var item *CatalogItem
	for i := range items {
		if items[i].Path == rel {
			item = &items[i]
			break
		}
	}
	if item == nil {
		return out, fmt.Errorf("%w：资料不属于当前课程", ErrStudyInvalid)
	}
	root, e := os.OpenRoot(l.Root)
	if e != nil {
		return out, e
	}
	defer root.Close()
	if path.Clean(rel) != rel || strings.Contains(rel, "\\") || checkedStudyDir(root, path.Dir(rel), false) != nil {
		return out, ErrStudyInvalid
	}
	st, e := root.Lstat(rel)
	if e != nil || !st.Mode().IsRegular() {
		return out, ErrStudyInvalid
	}
	if item.FileType == "mp4" || item.FileType == "mov" || item.FileType == "mkv" || item.FileType == "webm" {
		return StudyImport{Objects: []StudyObject{{ID: studyID(), Type: "video", Layer: "main", W: 640, H: 440, Title: item.Name, OriginalPath: item.Path, SourceLectureID: item.LectureID}}}, nil
	}
	f, e := root.Open(rel)
	if e != nil {
		return out, e
	}
	defer f.Close()
	out, e = l.ImportStudyModule(ctx, lecture, path.Base(item.Path), f)
	if e == nil {
		for i := range out.Objects {
			out.Objects[i].SourceLectureID = item.LectureID
			out.Objects[i].OriginalPath = item.Path
		}
	}
	return out, e
}

// Material references stay confined to the indexed course and never relocate large recordings.
func (l *Library) StudyCourseSource(ctx context.Context, lecture, rel string) (*os.File, error) {
	lec, e := l.Store.Lecture(ctx, lecture)
	if e != nil {
		return nil, e
	}
	course, e := l.Store.Course(ctx, lec.CourseID)
	if e != nil {
		return nil, e
	}
	if course.Dir == "" || !strings.HasPrefix(rel, course.Dir+"/") || path.Clean(rel) != rel || strings.Contains(rel, "\\") {
		return nil, ErrStudyInvalid
	}
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") {
			return nil, ErrStudyInvalid
		}
	}
	root, e := os.OpenRoot(l.Root)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	if checkedStudyDir(root, path.Dir(rel), false) != nil {
		return nil, ErrStudyInvalid
	}
	st, e := root.Lstat(rel)
	if e != nil || !st.Mode().IsRegular() {
		return nil, ErrStudyInvalid
	}
	return root.Open(rel)
}
func (l *Library) StudyVideoSource(ctx context.Context, lecture, object string) (*os.File, error) {
	return l.studyMediaFile(ctx, lecture, object)
}
