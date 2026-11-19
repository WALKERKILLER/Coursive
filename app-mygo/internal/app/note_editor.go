package app

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// NoteEditorOptions are NoteEditor's parameters besides the note.
type NoteEditorOptions struct {
	CourseID, LectureID string
	// Timestamp returns the current playback time in ms, enabling "insert
	// timestamp".
	Timestamp func() int
	OnSaved   func(core.Note)
	// OnSeek handles timestamp taps in the preview (the player seeks in
	// place).
	OnSeek  func(CoursiveRef)
	OnClose func()
	// Edit opens in writing mode even when the note has content.
	Edit bool
}

// NoteEditor is note_editor.dart's: the Markdown note editor with a live
// preview. It saves through the daemon, which writes the note as a
// Markdown file with front matter and indexes it.
type NoteEditor struct {
	o        NoteEditorOptions
	note     *core.Note
	title    string
	body     string
	preview  bool
	saving   bool
	dirty    bool
	id       string
	baseBody string
	cancel   func()
	sel      [2]int
	caret    int
	scroll   ui.ScrollState
}

// NewNoteEditor edits n, or a new note when nil.
func NewNoteEditor(n *core.Note, o NoteEditorOptions) *NoteEditor {
	e := &NoteEditor{o: o, note: n, caret: -1}
	if n != nil {
		e.title, e.body, e.id, e.baseBody = n.Title(), n.Body(), n.ID(), n.Body()
	}
	// Existing notes open in reading mode so timestamps are tappable.
	e.preview = !o.Edit && strings.TrimSpace(e.body) != ""
	e.cancel = State.Events.Listen(func(ev core.J) {
		if ev.S("type") == "notes_changed" && !e.dirty && e.id != "" {
			e.reload()
		}
	})
	return e
}

// Dispose stops following the note's changes.
func (e *NoteEditor) Dispose() { e.cancel() }

func (e *NoteEditor) reload() {
	id := e.id
	core.Async(func() (any, error) { return State.Api().Get("/api/notes/" + core.Esc(id)) }, func(r any, err error) {
		if err != nil || e.dirty {
			return
		}
		j := core.Obj(r)
		e.title, e.body, e.baseBody, e.dirty = j.S("title"), j.S("body"), j.S("body"), false
	})
}

func (e *NoteEditor) lectureID() string {
	if e.o.LectureID != "" {
		return e.o.LectureID
	}
	if e.note != nil {
		return e.note.LectureID()
	}
	return ""
}

func (e *NoteEditor) insertTimestamp() {
	s := e.o.Timestamp() / 1000
	h, m, sec := s/3600, s%3600/60, s%60
	tag := fmt.Sprintf("[%02d:%02d] ", m, sec)
	if h > 0 {
		tag = fmt.Sprintf("[%02d:%02d:%02d] ", h, m, sec)
	}
	text := []rune(e.body)
	start, end := e.sel[0], e.sel[1]
	if start < 0 || start > len(text) {
		start, end = len(text), len(text)
	}
	end = min(max(end, start), len(text))
	prefix := ""
	if start > 0 && text[start-1] != '\n' {
		prefix = "\n"
	}
	ins := []rune(prefix + tag)
	e.body = string(text[:start]) + string(ins) + string(text[end:])
	e.caret = start + len(ins)
	e.dirty = true
}

func (e *NoteEditor) save() {
	e.saving = true
	title := strings.TrimSpace(e.title)
	if title == "" {
		title = l10n.NoteUntitled
	}
	courseID, lectureID, source := e.o.CourseID, e.o.LectureID, "user"
	if e.note != nil {
		courseID, lectureID = e.note.CourseID(), e.note.LectureID()
		if e.note.Source() != "" {
			source = e.note.Source()
		}
	}
	body := core.J{"course_id": courseID, "lecture_id": lectureID, "title": title, "body": e.body, "source": source}
	if e.id != "" {
		body["id"], body["base_body"] = e.id, e.baseBody
	}
	text := e.body
	core.Async(func() (any, error) { return State.Api().Put("/api/notes", body) }, func(r any, err error) {
		e.saving = false
		if err != nil {
			var ae *core.ApiError
			if errors.As(err, &ae) && ae.Status == 409 {
				e.conflict()
				return
			}
			widgets.ShowSnack(l10n.CommonSaveFailed(err), "", nil)
			return
		}
		j := core.Obj(r)
		j["body"] = text
		n := core.Note{J: j}
		e.id, e.baseBody, e.dirty = n.ID(), text, false
		if e.o.OnSaved != nil {
			e.o.OnSaved(n)
		}
		widgets.ShowSnack(l10n.NoteSaved, "", nil)
	})
}

// conflict offers to keep the text as a new note when the note changed
// elsewhere.
func (e *NoteEditor) conflict() {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: "笔记已在另一端更新", Message: "本机文字仍保留在编辑器中，可以另存一份，避免覆盖另一端。", Actions: func() {
			if m3.TextButton(c, "继续编辑").Clicked() {
				r.Pop(false)
			}
			if m3.FilledButton(c, "另存为新笔记").Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v == true {
			e.id = ""
			e.save()
			return
		}
		widgets.ShowSnack(l10n.CommonSaveFailed(errors.New("409")), "", nil)
	})
}

// Show builds the editor filling its parent.
func (e *NoteEditor) Show(c *ui.Context) {
	cs, tt := design.S(), design.TT()
	ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			if e.o.OnClose != nil && m3.IconBtn(c, icons.ArrowBackRounded, l10n.CommonBack).Clicked() {
				e.o.OnClose()
			}
			ui.Box(c).Grow(1).Basis(0).Children(func() {
				in := m3.Field{Hint: l10n.CommonTitle, Borderless: true, Dense: true, Style: &tt.TitleLarge}.Show(c, &e.title)
				if in.Changed() {
					e.dirty = true
				}
			})
			m3.Segmented(c, []m3.Segment[bool]{
				{Value: false, Icon: icons.EditOutlined, Tooltip: l10n.CommonEdit},
				{Value: true, Icon: icons.VisibilityOutlined, Tooltip: l10n.CommonPreview},
			}, &e.preview, m3.SegOpts{NoCheck: true})
		})
		m3.VGap(c, design.SpaceSM)
		area := ui.Column(c).FillWidth().Grow(1).Basis(0)
		area.Children(func() {
			if e.preview {
				data := e.body
				if data == "" {
					data = l10n.NoteEmptyPreview
				}
				baseDir := ""
				if e.note != nil {
					baseDir = path.Dir(e.note.Path())
				}
				st := tt.BodyMedium.H(1.7)
				lecture := e.lectureID()
				ui.Scroll(c.Key("p")).Fill().TrackScroll(&e.scroll).Padding(design.SpaceLG).Background(cs.SurfaceContainerLow).Radius(design.CornerLG).
					Transition(design.Switcher(design.Medium1, nil, nil, ui.Motion{Opacity: 0})).Children(func() {
					CoursiveMarkdown{Data: data, Style: &st, LectureID: lecture, BaseDir: baseDir,
						OnRef: func(r CoursiveRef) { OpenRef(r, lecture, e.o.OnSeek) }}.Show(c)
				})
				return
			}
			h := area.Bounds().H
			if h <= 0 {
				h = 320
			}
			hint := ""
			if e.o.Timestamp != nil {
				hint = l10n.NoteEditorHintTimestamp
			}
			st := tt.BodyMedium.H(1.6)
			in := m3.Field{Hint: l10n.NoteEditorHint(hint), Multiline: true, Height: h, Style: &st}.Show(c.Key("e"), &e.body).
				Transition(design.Switcher(design.Medium1, nil, nil, ui.Motion{Opacity: 0}))
			if in.Changed() {
				e.dirty = true
			}
			if e.caret >= 0 {
				in.Focus()
				in.SetTextSelection(e.caret, e.caret)
				e.caret = -1
			}
			e.sel[0], e.sel[1] = in.TextSelection()
		})
		m3.VGap(c, design.SpaceSM)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			if e.o.Timestamp != nil && (m3.Button{Kind: m3.TextKind, Icon: icons.MoreTimeRounded, Label: l10n.NoteInsertTime}).Show(c).Clicked() {
				e.insertTimestamp()
			}
			ui.Box(c).Grow(1)
			op := float32(0)
			if e.dirty {
				op = 1
			}
			t := m3.Text(c, l10n.NoteUnsaved, tt.LabelSmall.C(cs.OnSurfaceVariant))
			t.Opacity(t.AnimateTo("dirty", op, design.T().Motion(design.Short4, nil)))
			m3.HGap(c, design.SpaceSM)
			b := m3.Button{Kind: m3.Filled, Icon: icons.CheckRounded, Label: l10n.CommonSave, Disabled: e.saving || !e.dirty && e.id != ""}
			if e.saving {
				b.Icon = 0
				b.Child = func(fg ui.Color) {
					ui.Row(c).AlignItems(ui.Center).Gap(design.SpaceSM).Children(func() {
						m3.Spinner(c, 16, 2, fg)
						m3.Text(c, l10n.CommonSave, tt.LabelLarge.C(fg))
					})
				}
			}
			if b.Show(c).Clicked() {
				e.save()
			}
		})
	})
}
