package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStudyConversationIsolationCASAndUpgradeBackup(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	before, e := l.StudyCanvas(ctx, "one")
	if e != nil {
		t.Fatal(e)
	}
	before.Version = 2
	before, e = l.SaveStudyCanvas(ctx, "one", before)
	if e != nil {
		t.Fatal(e)
	}
	lec, _ := l.Store.Lecture(ctx, "one")
	file := filepath.Join(l.Root, lec.Dir, "study-canvas.json")
	original, _ := os.ReadFile(file)
	for _, id := range []string{"first", "second"} {
		c, e := l.StudyConversation(ctx, "one", id, nil)
		if e != nil {
			t.Fatal(e)
		}
		c.Draft = "only " + id
		c.Turns = []StudyTurn{{ID: "turn", Question: "question " + id, Answer: "answer", State: "done", ContextKey: id}}
		saved, e := l.StudyConversation(ctx, "one", id, &c)
		if e != nil || saved.Revision != 1 {
			t.Fatal(e, saved)
		}
		c.Revision = 0
		if _, e = l.StudyConversation(ctx, "one", id, &c); !errors.Is(e, ErrStudyConflict) {
			t.Fatal("stale chat overwrote history", e)
		}
	}
	first, e := l.StudyConversation(ctx, "one", "first", nil)
	if e != nil || first.Draft != "only first" {
		t.Fatal(e, first)
	}
	if _, e = l.StudyConversation(ctx, "one", "../escape", nil); e == nil {
		t.Fatal("accepted traversal")
	}
	before.Version = 3
	before.Objects = append(before.Objects, StudyObject{ID: "chat", Type: "conversation", ConversationID: "first", Layer: "main", W: 520, H: 620, Title: "AI"})
	if _, e = l.SaveStudyCanvas(ctx, "one", before); e != nil {
		t.Fatal(e)
	}
	backup, e := os.ReadFile(filepath.Join(l.Root, lec.Dir, "study-canvas.pre-v3.json"))
	if e != nil || string(backup) != string(original) {
		t.Fatal("upgrade did not preserve original bytes", e)
	}
	reopened, e := l.StudyCanvas(ctx, "one")
	if e != nil || reopened.Objects[1].ConversationID != "first" {
		t.Fatal(e, reopened)
	}
}
func TestCourseMaterialScopeAndOriginalBytes(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	lec, _ := l.Store.Lecture(ctx, "one")
	source := filepath.Join(l.Root, lec.Dir, "material.txt")
	if e := os.MkdirAll(filepath.Dir(source), 0700); e != nil {
		t.Fatal(e)
	}
	original := []byte("Original UTF-8 learning material")
	if e := os.WriteFile(source, original, 0600); e != nil {
		t.Fatal(e)
	}
	rel := filepath.ToSlash(filepath.Join(lec.Dir, "material.txt"))
	catalog, e := l.CourseCatalog(ctx, lec.CourseID)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, it := range catalog {
		if it.Path == rel {
			found = true
		}
		if it.CourseID != lec.CourseID {
			t.Fatal("cross-course leak")
		}
	}
	if !found {
		t.Fatal("missing material", lec)
	}
	out, e := l.ImportStudyMaterial(ctx, "one", rel)
	if e != nil {
		t.Fatal(e)
	}
	if out.Objects[0].Text != string(original) || out.Objects[0].SourceLectureID != "one" {
		t.Fatal(out)
	}
	b, _ := os.ReadFile(source)
	if string(b) != string(original) {
		t.Fatal("original modified")
	}
	for _, p := range []string{"../../secret.txt", "courses/not-in-course/material.txt"} {
		if _, e = l.ImportStudyMaterial(ctx, "one", p); e == nil {
			t.Fatal("accepted foreign reference")
		}
	}
}
func TestStudyContextIndependentTextSelections(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	d, _ := l.StudyCanvas(ctx, "one")
	d.Objects = append(d.Objects, StudyObject{ID: "a", Type: "text", Layer: "main", W: 400, H: 300, Text: "one two"}, StudyObject{ID: "b", Type: "text", Layer: "main", W: 400, H: 300, Text: "three four"})
	d, e := l.SaveStudyCanvas(ctx, "one", d)
	if e != nil {
		t.Fatal(e)
	}
	out, e := l.StudyContext(ctx, "one", StudyContextRequest{Revision: d.Revision, Objects: []string{"a", "b"}, Selections: map[string]StudySelection{"a": {ObjectID: "a", Start: 4, End: 7}, "b": {ObjectID: "b", Start: 0, End: 5}}})
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Sources) != 2 || out.Sources[0].Text != "two" || out.Sources[1].Text != "three" {
		t.Fatal(out)
	}
	for _, s := range out.Sources {
		if strings.Contains(s.ID, "lecture") {
			t.Fatal("unselected video was included")
		}
	}
}

func TestStudyConversationValidatesGroundedResults(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	anchor := `{"id":"object-pdf","object_id":"pdf","kind":"pdf","page":2,"start":4,"end":12}`
	save := func(id, result string) error {
		c, e := l.StudyConversation(ctx, "one", id, nil)
		if e != nil {
			t.Fatal(e)
		}
		c.Turns = []StudyTurn{{ID: "turn", Question: "q", Answer: "a", State: "done", Result: json.RawMessage(result)}}
		_, e = l.StudyConversation(ctx, "one", id, &c)
		return e
	}
	good := `{"schema_version":2,"sources":[],"evidence":[{"id":"e1","source_id":"object-pdf","kind":"exact_text","status":"verified","label":"第 2 页","anchor":` + anchor + `}],"suggestions":[{"kind":"open_source","evidence_id":"e1"},{"kind":"ask_followup","text":"再举一个例子"}]}`
	if e := save("good", good); e != nil {
		t.Fatal("valid grounded result rejected", e)
	}
	if e := save("legacy", `{"sources":[{"anything":true}]}`); e != nil {
		t.Fatal("legacy result rejected", e)
	}
	for name, result := range map[string]string{
		"unknown kind":      `{"schema_version":2,"evidence":[{"id":"e1","kind":"guess","status":"verified","anchor":` + anchor + `}]}`,
		"unknown status":    `{"schema_version":2,"evidence":[{"id":"e1","kind":"exact_text","status":"likely","anchor":` + anchor + `}]}`,
		"duplicate id":      `{"schema_version":2,"evidence":[{"id":"e1","kind":"exact_text","status":"verified","anchor":` + anchor + `},{"id":"e1","kind":"exact_text","status":"verified","anchor":` + anchor + `}]}`,
		"bad id":            `{"schema_version":2,"evidence":[{"id":"x1","kind":"exact_text","status":"verified","anchor":` + anchor + `}]}`,
		"bad anchor":        `{"schema_version":2,"evidence":[{"id":"e1","kind":"exact_text","status":"verified","anchor":{"id":"object-pdf","object_id":"pdf","kind":"pdf","start":9,"end":2}}]}`,
		"unissued jump":     `{"schema_version":2,"suggestions":[{"kind":"open_source","evidence_id":"e9"}]}`,
		"action suggestion": `{"schema_version":2,"suggestions":[{"kind":"delete_card","text":"x"}]}`,
		"too many":          `{"schema_version":2,"suggestions":[{"kind":"ask_followup","text":"a"},{"kind":"ask_followup","text":"b"},{"kind":"ask_followup","text":"c"}]}`,
		"future schema":     `{"schema_version":3}`,
	} {
		if e := save("bad", result); !errors.Is(e, ErrStudyInvalid) {
			t.Fatal(name, "accepted", e)
		}
	}
}

func TestStudyConversationValidatesOutlineProposals(t *testing.T) {
	l, _ := reviewFixture(t)
	ctx := context.Background()
	evidence := `"evidence":[{"id":"e1","source_id":"object-pdf","kind":"exact_text","status":"verified","anchor":{"id":"object-pdf","object_id":"pdf","kind":"pdf","page":6,"start":0,"end":4}}]`
	ink := `{"id":"selected-ink","object_id":"pdf","kind":"ink","stroke_ids":["s1"]}`
	outline := func(state, items string) string {
		return `{"schema_version":2,` + evidence + `,"outline_proposal":{"version":1,"proposal_id":"0f8c2d3e-1a2b-4c5d-8e9f-0a1b2c3d4e5f","lecture_id":"one","conversation_id":"c","turn_id":"turn",` +
			`"source_revision":3,"state":` + state + `,"title":"复习提纲","sections":[{"id":"s1","heading":"概念","level":1,"items":[` + items + `]}]}}`
	}
	fact := `{"id":"i1","kind":"fact","text":"矩阵乘法不交换","evidence_ids":["e1"],"review":"pending"}`
	mark := `{"id":"i2","kind":"unresolved_mark","text":"第 6 页旁的勾","review":"needs_interpretation","source_id":"selected-ink","anchor":` + ink + `}`
	save := func(id string, turn StudyTurn) error {
		c, e := l.StudyConversation(ctx, "one", id, nil)
		if e != nil {
			t.Fatal(e)
		}
		c.Turns = []StudyTurn{turn}
		_, e = l.StudyConversation(ctx, "one", id, &c)
		return e
	}
	turn := func(result string) StudyTurn {
		return StudyTurn{ID: "turn", Task: "propose_outline", Question: "整理", Answer: "提纲草案", State: "done", Result: json.RawMessage(result)}
	}
	hash := "af63dc4c8601ec8c"
	for name, result := range map[string]string{
		"draft":    outline(`"draft"`, fact+","+mark),
		"resolved": outline(`"draft"`, strings.Replace(strings.Replace(mark, `"needs_interpretation"`, `"confirmed"`, 1), `"review"`, `"choice":"custom","choice_text":"考前再看","review"`, 1)),
		"accepted": strings.Replace(outline(`"accepted"`, fact), `"title"`, `"note_id":"outline-0f8c2d3e","note_hash":"`+hash+`","title"`, 1),
	} {
		if e := save("good-"+name, turn(result)); e != nil {
			t.Fatal(name, "rejected", e)
		}
	}
	// The assistant host writes this fixture; the app renders it.
	shared, e := os.ReadFile(filepath.Join("testdata", "outline-turn-result.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = save("good-shared", turn(string(shared))); e != nil {
		t.Fatal("host outline rejected", e)
	}
	c, e := l.StudyConversation(ctx, "one", "good-draft", nil)
	if e != nil || c.Turns[0].Task != "propose_outline" || !strings.Contains(string(c.Turns[0].Result), `"unresolved_mark"`) {
		t.Fatal("outline turn not kept", e, c.Turns)
	}
	sections := make([]string, 51)
	for i := range sections {
		sections[i] = fmt.Sprintf(`{"id":"s%d","heading":"h","level":1,"items":[]}`, i)
	}
	items := make([]string, 21)
	for i := range items {
		items[i] = fmt.Sprintf(`{"id":"i%d","kind":"summary","text":"t","review":"pending"}`, i)
	}
	for name, result := range map[string]string{
		"unissued evidence":     outline(`"draft"`, strings.Replace(fact, `"e1"`, `"e2"`, 1)),
		"mark given a meaning":  outline(`"draft"`, strings.Replace(mark, `"review"`, `"choice":"mastered","review"`, 1)),
		"fact with a choice":    outline(`"draft"`, strings.Replace(fact, `"review"`, `"choice":"key","review"`, 1)),
		"fact needs reading":    outline(`"draft"`, strings.Replace(fact, `"pending"`, `"needs_interpretation"`, 1)),
		"unknown review":        outline(`"draft"`, strings.Replace(fact, `"pending"`, `"done"`, 1)),
		"unknown item kind":     outline(`"draft"`, strings.Replace(fact, `"fact"`, `"command"`, 1)),
		"custom without text":   outline(`"draft"`, strings.Replace(strings.Replace(mark, `"needs_interpretation"`, `"confirmed"`, 1), `"review"`, `"choice":"custom","review"`, 1)),
		"duplicate item":        outline(`"draft"`, fact+","+fact),
		"accepted without note": outline(`"accepted"`, fact),
		"draft with a note":     strings.Replace(outline(`"draft"`, fact), `"title"`, `"note_id":"n1","title"`, 1),
		"unknown state":         outline(`"saved"`, fact),
		"too many items":        outline(`"draft"`, strings.Join(items, ",")),
		"too many sections":     strings.Replace(outline(`"draft"`, fact), `[{"id":"s1","heading":"概念","level":1,"items":[`+fact+`]}]`, "["+strings.Join(sections, ",")+"]", 1),
		"bad level":             strings.Replace(outline(`"draft"`, fact), `"level":1`, `"level":4`, 1),
		"empty title":           strings.Replace(outline(`"draft"`, fact), `"复习提纲"`, `" "`, 1),
		"bad proposal id":       strings.Replace(outline(`"draft"`, fact), `0f8c2d3e-1a2b-4c5d-8e9f-0a1b2c3d4e5f`, `../x`, 1),
	} {
		if e := save("bad", turn(result)); !errors.Is(e, ErrStudyInvalid) {
			t.Fatal(name, "accepted", e)
		}
	}
	bad := turn("")
	bad.Task = "rewrite_canvas"
	if e := save("bad", bad); !errors.Is(e, ErrStudyInvalid) {
		t.Fatal("unknown task accepted", e)
	}
	// Answers keep their 24-citation limit; outlines may cite up to 120.
	many := make([]string, 30)
	for i := range many {
		many[i] = fmt.Sprintf(`{"id":"e%d","kind":"exact_text","status":"verified","anchor":{"id":"a","object_id":"pdf","kind":"pdf"}}`, i+1)
	}
	list := `"evidence":[` + strings.Join(many, ",") + `]`
	if e := save("bad", StudyTurn{ID: "turn", Question: "q", Answer: "a", State: "done", Result: json.RawMessage(`{"schema_version":2,` + list + `}`)}); !errors.Is(e, ErrStudyInvalid) {
		t.Fatal("answer with 30 citations accepted", e)
	}
	if e := save("many", turn(strings.Replace(outline(`"draft"`, fact), evidence, list, 1))); e != nil {
		t.Fatal("outline with 30 citations rejected", e)
	}
}
