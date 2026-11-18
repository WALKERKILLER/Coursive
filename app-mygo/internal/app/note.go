package app

import (
	"errors"
	"io"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/assets"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// note_reader.dart: a note opened for reading, with contents on the left,
// the text in a readable column, and the sources a reference points to
// (recording, slide or courseware) in a supporting pane, a bottom sheet
// on narrow windows.

// NotePage reads a note, or writes a new one.
type NotePage struct {
	noteID   string
	note     *core.Note
	reader   *MdReader
	loading  bool
	editing  bool
	search   bool
	err      error
	ref      *CoursiveRef
	pane     *SourcePane
	editor   *NoteEditor
	width    float32
	activity ActivityRegion
}

// NewNotePage opens a note; an empty id writes a new one.
func NewNotePage(id string) m3.Page {
	p := &NotePage{noteID: id}
	if id == "" {
		p.editing = true
		p.editor = NewNoteEditor(nil, NoteEditorOptions{Edit: true, OnSaved: p.set})
	} else {
		p.loading = true
		p.load()
	}
	return p
}

// Dispose stops counting reading time and the editor's listening.
func (p *NotePage) Dispose() {
	p.activity.Stop()
	if p.editor != nil {
		p.editor.Dispose()
	}
}

func (p *NotePage) load() {
	core.Async(func() (any, error) { return State.Api().Get("/api/notes/" + core.Esc(p.noteID)) }, func(r any, err error) {
		p.loading = false
		if err != nil {
			p.err = err
			return
		}
		p.set(core.Note{J: core.Obj(r)})
	})
}

func (p *NotePage) set(n core.Note) {
	p.note = &n
	p.reader = NewMdReader(NewMdDoc(n.Body()), "")
	p.reader.LectureID = n.LectureID()
	p.reader.BaseDir = path.Dir(n.Path())
	p.reader.OnRef = func(r CoursiveRef) { p.open(r) }
	p.reader.Header = func(c *ui.Context) { noteHeader(c, n) }
}

func (p *NotePage) lectureID() string {
	if p.note == nil {
		return ""
	}
	return p.note.LectureID()
}

func (p *NotePage) open(ref CoursiveRef) {
	if ref.Kind == RefLecture || ref.Kind == RefFile {
		OpenRef(ref, "", nil)
		return
	}
	if p.width >= 1200 {
		p.ref, p.pane = &ref, NewSourcePane(ref, p.lectureID())
		return
	}
	pane := NewSourcePane(ref, p.lectureID())
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true, HeightFactor: .85}, func(c *ui.Context, r *m3.Route) {
		ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() { pane.Show(c, nil) })
	})
}

func (p *NotePage) delete() {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Icon: icons.DeleteOutlineRounded, Title: l10n.NoteDeleteTitle, Message: l10n.NoteDeleteBody, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Close()
			}
			if m3.FilledButton(c, l10n.CommonDelete).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) {
		if v != true {
			return
		}
		id := p.note.ID()
		core.Async(func() (any, error) { return State.Api().Delete("/api/notes/" + core.Esc(id)) }, func(_ any, err error) {
			if err != nil {
				widgets.ShowSnack(err.Error(), "", nil)
				return
			}
			m3.Nav().PopPage(p)
		})
	})
}

func (p *NotePage) showToc() {
	m3.ShowModalBottomSheet(m3.SheetOptions{HeightFactor: .6}, func(c *ui.Context, r *m3.Route) {
		ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() { p.reader.ShowToc(c, r.Close) })
	})
}

// View builds the page.
func (p *NotePage) View(c *ui.Context) {
	cs, tt := design.S(), design.TT()
	n := p.note
	screen := map[string]string{"screen": l10n.CommonNotes}
	if n != nil {
		screen["note_id"], screen["note_title"] = n.ID(), n.Title()
		if n.LectureID() != "" {
			screen["lecture_id"] = n.LectureID()
		}
	}
	ReportScreen(screen)
	tocPane, sidePane := p.width >= 1040, p.width >= 1200
	reading := !p.loading && p.err == nil && !p.editing && n != nil
	title := l10n.NoteNew
	if n != nil {
		title = n.Title()
	}
	bar := &m3.AppBar{Title: title, BG: ui.Transparent, Actions: func() {
		if reading {
			if (m3.IconButton{Icon: icons.SearchRounded, Tooltip: l10n.CommonFind, Toggle: true, Selected: p.search}).Show(c).Clicked() {
				p.search = !p.search
				if !p.search {
					p.reader.CloseSearch()
				}
			}
			if !tocPane && len(p.reader.Doc.Headings) > 0 && m3.IconBtn(c, icons.TocRounded, l10n.CommonContents).Clicked() {
				p.showToc()
			}
			if m3.IconBtn(c, icons.EditOutlined, l10n.CommonEdit).Clicked() {
				p.edit()
			}
			ctx := core.J{"note_id": n.ID()}
			if n.LectureID() != "" {
				ctx["lecture_id"] = n.LectureID()
			}
			if n.CourseID() != "" {
				ctx["course_id"] = n.CourseID()
			}
			PluginMenuButton(c, "note.toolbar", ctx, true, "note")
		}
		if n != nil && n.LectureID() != "" && m3.IconBtn(c, icons.OndemandVideoRounded, l10n.NoteOpenLecture).Clicked() {
			m3.RootNav.Push(NewPlayerPage(n.LectureID(), -1))
		}
		if n != nil {
			m3.MenuButton(c, "more", icons.MoreVertRounded, l10n.CommonMore, m3.Below, func() []m3.MenuItem {
				return []m3.MenuItem{
					{Icon: icons.CopyRounded, Label: l10n.NoteCopyMarkdown, OnTap: func() {
						c.WriteClipboard(n.Body())
						widgets.ShowSnack(l10n.NoteCopiedMarkdown, "", nil)
					}},
					{Icon: icons.DeleteOutlineRounded, Label: l10n.CommonDelete, OnTap: p.delete},
				}
			})
		}
		m3.HGap(c, design.SpaceSM)
	}}
	if reading {
		bar.Bottom = func() {
			m3.Linear(c, m3.Progress{Value: m3.Of(float64(p.reader.Progress)), Height: 2, NoTrack: true})
		}
	}
	m3.Scaffold{BG: ui.Transparent, AppBar: bar, Body: func() {
		body := ui.Row(c).Fill()
		if w := body.Bounds().W; w > 0 {
			p.width = w
		} else if p.width == 0 {
			p.width = WindowWidth
		}
		mark := any(nil)
		if p.reader != nil {
			mark = p.reader.Current
		}
		p.activity.Show(c, body, "read", p.lectureID(), mark)
		body.Children(func() {
			switch {
			case p.loading:
				widgets.CenteredLoader(c, "")
			case p.err != nil:
				widgets.ErrorState(c, p.err, nil)
			case p.editing || n == nil:
				ui.Column(c).Fill().Children(func() {
					widgets.ContentWidth(c, 860, func() {
						ui.Column(c).FillWidth().Grow(1).Padding(0, design.SpaceXL, design.SpaceXL, design.SpaceXL).Children(func() {
							p.editor.Show(c)
						})
					})
				})
			default:
				if tocPane && len(p.reader.Doc.Headings) > 0 {
					ui.Column(c).Width(272).Shrink(0).FillHeight().Children(func() {
						m3.Text(c, l10n.CommonContents, tt.TitleSmall.C(cs.OnSurfaceVariant)).Padding(design.SpaceLG, design.SpaceLG, design.SpaceXS, design.SpaceXL)
						p.reader.ShowToc(c, nil)
					})
				}
				ui.Column(c).Grow(1).Basis(0).FillHeight().Children(func() {
					if p.search {
						ui.Box(c).FillWidth().Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, Enter: &ui.Motion{Opacity: 0, Y: -12}}).Children(func() {
							p.reader.ShowSearchBar(c, func() { p.search = false; p.reader.CloseSearch() })
						})
					}
					ui.Column(c).FillWidth().Grow(1).Basis(0).Key(n.UpdatedAt()).Children(func() { p.reader.Show(c) })
				})
				if sidePane && p.ref != nil {
					ui.Box(c).Width(400).Shrink(0).FillHeight().Padding(design.SpaceSM, design.SpaceLG, design.SpaceLG, 0).
						Transition(ui.ElementTransition{Duration: design.Medium4, Ease: design.Emphasized, Enter: &ui.Motion{Opacity: 0, X: 40}}).Children(func() {
						ui.Column(c).Fill().Background(cs.SurfaceContainerLow).Radius(design.CornerXL).Clip().Children(func() {
							p.pane.Show(c, func() { p.ref, p.pane = nil, nil })
						})
					})
				}
			}
		})
	}}.Show(c)
}

func (p *NotePage) edit() {
	if p.editor != nil {
		p.editor.Dispose()
	}
	p.editing = true
	p.editor = NewNoteEditor(p.note, NoteEditorOptions{Edit: true, OnSaved: p.set, OnClose: func() { p.editing = false }})
}

func noteHeader(c *ui.Context, n core.Note) {
	cs, tt := design.S(), design.TT()
	agent := n.Source() == "agent"
	courseTitle := ""
	if co, ok := State.Course(n.CourseID()); ok {
		courseTitle = co.Title()
	}
	ui.Column(c).FillWidth().Padding(design.SpaceSM, design.SpaceSM, design.SpaceXL, design.SpaceSM).Children(func() {
		m3.Text(c, n.Title(), tt.HeadlineSmall)
		m3.VGap(c, design.SpaceSM)
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).GapX(design.SpaceSM).GapY(design.SpaceXS).Children(func() {
			if courseTitle != "" {
				m3.Text(c, courseTitle, tt.BodyMedium.C(cs.OnSurfaceVariant))
			}
			if n.UpdatedAt() > 0 {
				m3.Text(c, l10n.NoteUpdated(core.FmtRelative(time.Unix(int64(n.UpdatedAt()), 0))), tt.BodyMedium.C(cs.OnSurfaceVariant))
			}
			bg, fg, ic, label := cs.SecondaryContainer, cs.OnSecondaryContainer, icons.EditNoteRounded, l10n.NoteMine
			if agent {
				bg, fg, ic, label = cs.TertiaryContainer, cs.OnTertiaryContainer, icons.AutoAwesomeOutlined, l10n.NoteByAssistant
			}
			ui.Row(c).AlignItems(ui.Center).Padding(2, 8).Background(bg).Radius(design.CornerSM).Gap(4).Children(func() {
				m3.Icon(c, ic, 14, fg)
				m3.Text(c, label, tt.LabelSmall.C(fg))
			})
		})
		m3.VGap(c, design.SpaceLG)
		m3.Divider(c)
	})
}

// SourcePane is what a reference points to: the moment in the recording
// (slide on screen and what was said), a slide, or a courseware passage.
type SourcePane struct {
	ref       CoursiveRef
	lectureID string
	// The moment's data.
	loaded bool
	slide  *core.Slide
	segs   []core.Segment
	ms     int
	cw     *CoursewareSource
}

// NewSourcePane loads what ref points to; lectureID is the lecture
// references without one belong to.
func NewSourcePane(ref CoursiveRef, lectureID string) *SourcePane {
	s := &SourcePane{ref: ref, lectureID: lectureID}
	switch ref.Kind {
	case RefTime:
		if ref.LectureID != "" {
			s.lectureID = ref.LectureID
		}
		s.ms = ref.Seconds * 1000
		s.loadMoment(-1)
	case RefSlide:
		s.lectureID = ref.LectureID
		s.loadMoment(ref.Idx)
	case RefCourseware:
		s.cw = NewCoursewareSource(ref.LectureID, ref.File, ref.Query)
	}
	return s
}

func (s *SourcePane) loadMoment(slideIdx int) {
	id, ms := s.lectureID, s.ms
	type moment struct {
		slide *core.Slide
		segs  []core.Segment
		ms    int
	}
	core.Async(func() (moment, error) {
		api := State.Api()
		var slides []core.Slide
		if r, err := api.Get("/api/lectures/" + core.Esc(id) + "/slides"); err == nil {
			for _, j := range core.Objs(r) {
				slides = append(slides, core.Slide{J: j})
			}
		}
		slices.SortStableFunc(slides, func(a, b core.Slide) int { return a.TSec() - b.TSec() })
		keys := slides
		if slices.ContainsFunc(slides, func(s core.Slide) bool { return s.IsKey() }) {
			keys = nil
			for _, sl := range slides {
				if sl.IsKey() {
					keys = append(keys, sl)
				}
			}
		}
		var m moment
		m.ms = ms
		if slideIdx >= 0 {
			for i := range slides {
				if slides[i].Idx() == slideIdx {
					m.slide = &slides[i]
				}
			}
			if m.slide != nil {
				m.ms = m.slide.TSec() * 1000
			} else {
				m.ms = 0
			}
		} else {
			for i := range keys {
				if keys[i].TSec()*1000 <= ms+5000 {
					m.slide = &keys[i]
				}
			}
		}
		from, to := max(m.ms-30000, 0), m.ms+90000
		if r, err := api.Get("/api/lectures/"+core.Esc(id)+"/transcript", map[string]string{"from_ms": strconv.Itoa(from), "to_ms": strconv.Itoa(to)}); err == nil {
			for _, j := range core.Objs(r) {
				m.segs = append(m.segs, core.Segment{J: j})
			}
		}
		return m, nil
	}, func(m moment, _ error) {
		s.loaded, s.slide, s.segs, s.ms = true, m.slide, m.segs, m.ms
	})
}

func paneHeader(c *ui.Context, ic icons.Icon, title string, onClose func(), trailing func()) {
	cs := design.S()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceSM, design.SpaceXS, design.SpaceXS, design.SpaceLG).Children(func() {
		m3.Icon(c, ic, 20, cs.Primary)
		m3.HGap(c, design.SpaceSM)
		m3.Text(c, title, design.TT().TitleMedium).MaxLines(1).Grow(1).Basis(0)
		if trailing != nil {
			trailing()
		}
		if onClose != nil && m3.IconBtn(c, icons.CloseRounded, l10n.CommonClose).Clicked() {
			onClose()
		}
	})
}

// Show builds the pane; onClose, when set, adds a close button.
func (s *SourcePane) Show(c *ui.Context, onClose func()) {
	if s.cw != nil {
		s.cw.Show(c, onClose)
		return
	}
	if s.ref.Kind != RefTime && s.ref.Kind != RefSlide {
		return
	}
	cs, tt := design.S(), design.TT()
	slide := s.ref.Kind == RefSlide
	ic, title := icons.GraphicEqRounded, l10n.NoteAudioRef(core.FmtClock(s.ms))
	if slide {
		ic, title = icons.SlideshowOutlined, l10n.NoteSlideRef(s.ref.Idx, core.FmtClock(s.ms))
	}
	paneHeader(c, ic, title, onClose, nil)
	if !s.loaded {
		ui.Column(c).FillWidth().Grow(1).Children(func() { widgets.CenteredLoader(c, "") })
		return
	}
	ui.Scroll(c).FillWidth().Grow(1).Basis(0).Children(func() {
		ui.Column(c).FillWidth().Padding(design.SpaceXS, design.SpaceLG, design.SpaceLG, design.SpaceLG).Children(func() {
			if s.slide != nil {
				u := SlideImageURL(s.lectureID, s.slide.Idx())
				img := ui.ButtonBase(c).FocusRing(false).Cursor(ui.CursorZoomIn).FillWidth().AspectRatio(16.0 / 9).Radius(design.CornerLG).Clip().Background(cs.SurfaceContainerHighest)
				if img.Clicked() {
					ShowImageViewer(u)
				}
				img.Children(func() {
					bmp, failed := widgets.NetBitmap(u)
					switch {
					case failed:
						ui.Box(c).Fill().Center().Children(func() { m3.Icon(c, icons.BrokenImageOutlined, 24, cs.OnSurfaceVariant) })
					case bmp != nil:
						ui.Image(c, bmp).Fill().Fit(ui.Cover)
					}
				})
				m3.Text(c, l10n.NoteSlideAt(s.slide.Idx(), core.FmtClock(s.slide.TSec()*1000)), tt.LabelMedium.C(cs.OnSurfaceVariant)).Padding(design.SpaceXS, 0, design.SpaceLG, 0)
			}
			if (m3.Button{Kind: m3.Tonal, Icon: icons.PlayArrowRounded, Label: l10n.NotePlayFrom(core.FmtClock(s.ms))}).Show(c).FillWidth().Clicked() {
				SeekOrOpen(s.lectureID, s.ms/1000)
			}
			m3.VGap(c, design.SpaceLG)
			if len(s.segs) == 0 {
				m3.Text(c, l10n.NoteNoTranscriptHere, tt.BodyMedium.C(cs.OnSurfaceVariant))
			}
			for _, seg := range s.segs {
				segmentLine(c, seg, seg.StartMs() <= s.ms && s.ms < seg.EndMs(), s.lectureID)
			}
		})
	})
}

func segmentLine(c *ui.Context, seg core.Segment, current bool, lectureID string) {
	cs, tt := design.S(), design.TT()
	row := m3.Tappable(c.Key(seg.Idx()), design.CornerMD).FillWidth()
	if current {
		row = row.Background(cs.SecondaryContainer)
	}
	if row.Clicked() {
		SeekOrOpen(lectureID, seg.StartMs()/1000)
	}
	clock, text := cs.Primary, cs.OnSurface
	if current {
		clock, text = cs.OnSecondaryContainer, cs.OnSecondaryContainer
	}
	row.Padding(6, design.SpaceSM).Children(func() {
		ui.Row(c).FillWidth().Children(func() {
			m3.Text(c, core.FmtClock(seg.StartMs()), tt.LabelSmall.C(clock).H(1.9)).FontFeatures("tnum").Width(48).Shrink(0)
			m3.Text(c, seg.Text(), tt.BodyMedium.H(1.55).C(text)).Grow(1).Basis(0)
		})
	})
}

// LoadParsedCourseware loads a parsed courseware file's Markdown through
// the daemon.
func LoadParsedCourseware(lectureID, file string) (core.CoursewareFile, string, error) {
	api := State.Api()
	r, err := api.Get("/api/lectures/" + core.Esc(lectureID) + "/courseware")
	if err != nil {
		return core.CoursewareFile{}, "", err
	}
	var f *core.CoursewareFile
	for _, j := range core.Objs(r) {
		x := core.CoursewareFile{J: j}
		if x.Path() == file || x.Name() == file {
			f = &x
			break
		}
	}
	if f == nil {
		return core.CoursewareFile{}, "", errors.New(l10n.CoursewareGone)
	}
	if !f.Parsed() {
		return *f, "", errors.New(l10n.CoursewareNotParsed)
	}
	res, err := http.Get(api.FileURL(f.MdPath()))
	if err != nil {
		return *f, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return *f, "", errors.New(l10n.CoursewareReadFailed(res.StatusCode))
	}
	b, err := io.ReadAll(res.Body)
	return *f, string(b), err
}

// CoursewareSource is a courseware passage in the supporting pane, opened
// at its query.
type CoursewareSource struct {
	lectureID, file, query string
	f                      *core.CoursewareFile
	reader                 *MdReader
	err                    error
}

// NewCoursewareSource loads the file.
func NewCoursewareSource(lectureID, file, query string) *CoursewareSource {
	s := &CoursewareSource{lectureID: lectureID, file: file, query: query}
	type parsed struct {
		f  core.CoursewareFile
		md string
	}
	core.Async(func() (parsed, error) {
		f, md, err := LoadParsedCourseware(lectureID, file)
		return parsed{f, md}, err
	}, func(p parsed, err error) {
		if err != nil {
			s.err = err
			return
		}
		s.f = &p.f
		s.reader = NewMdReader(NewMdDoc(p.md), query)
		s.reader.BaseDir = path.Dir(p.f.MdPath())
		s.reader.LectureID = lectureID
		s.reader.OnRef = func(r CoursiveRef) { OpenRef(r, lectureID, nil) }
		s.reader.Padding = []float32{0, design.SpaceMD, design.SpaceXL, design.SpaceMD}
	})
	return s
}

// Show builds the pane.
func (s *CoursewareSource) Show(c *ui.Context, onClose func()) {
	title := l10n.AssetCourseware
	var trailing func()
	if s.f != nil {
		title = s.f.Name()
		f := *s.f
		trailing = func() {
			if m3.IconBtn(c, icons.OpenInFullRounded, l10n.ReaderFullscreen).Clicked() {
				OpenCoursewareReader(f, s.lectureID, s.query)
			}
		}
	}
	paneHeader(c, icons.DescriptionOutlined, title, onClose, trailing)
	ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() {
		switch {
		case s.err != nil:
			widgets.ErrorState(c, s.err, nil)
		case s.reader == nil:
			widgets.CenteredLoader(c, "")
		default:
			s.reader.Show(c)
		}
	})
}

// OpenCoursewareReader is openCoursewareReader: a full-window reader for a
// parsed courseware file, with contents and search.
func OpenCoursewareReader(file core.CoursewareFile, lectureID, query string) {
	p := &coursewareReaderPage{file: file, lectureID: lectureID, query: query}
	core.Async(func() (string, error) {
		_, md, err := LoadParsedCourseware(lectureID, file.Path())
		return md, err
	}, func(md string, err error) {
		if err != nil {
			p.err = err
			return
		}
		p.reader = NewMdReader(NewMdDoc(md), query)
		p.reader.LectureID = lectureID
		p.reader.BaseDir = path.Dir(file.MdPath())
		p.reader.OnRef = func(r CoursiveRef) { OpenRef(r, lectureID, nil) }
		p.reader.MaxWidth = 820
	})
	m3.RootNav.Push(p)
}

type coursewareReaderPage struct {
	file      core.CoursewareFile
	lectureID string
	query     string
	reader    *MdReader
	err       error
	search    bool
	width     float32
	activity  ActivityRegion
}

func (p *coursewareReaderPage) Dispose() { p.activity.Stop() }

func (p *coursewareReaderPage) View(c *ui.Context) {
	cs, tt := design.S(), design.TT()
	doc := (*MdDoc)(nil)
	if p.reader != nil {
		doc = p.reader.Doc
	}
	tocPane := p.width >= 1040 && doc != nil && len(doc.Headings) > 0
	pages := ""
	if p.file.Pages() > 0 {
		pages = l10n.CoursewarePages(p.file.Pages())
	}
	parsedBy := l10n.CoursewareParsedBy(pages)
	bar := &m3.AppBar{Title: p.file.Name(), Actions: func() {
		// On a phone the title needs the room; the logo keeps the credit.
		credit := ui.Row(c).AlignItems(ui.Center).Padding(0, design.SpaceSM, 0, 0)
		credit.Children(func() {
			ui.Image(c, assets.Image("brands/mineru.png")).Size(18, 18).Radius(4).Clip()
			if !Compact() {
				m3.HGap(c, 6)
				m3.Text(c, parsedBy, tt.LabelMedium.C(cs.OnSurfaceVariant))
			}
		})
		m3.Tip(c, credit, parsedBy)
		if doc != nil {
			if (m3.IconButton{Icon: icons.SearchRounded, Tooltip: l10n.CommonFind, Toggle: true, Selected: p.search}).Show(c).Clicked() {
				p.search = !p.search
				if !p.search {
					p.reader.CloseSearch()
				}
			}
			if !tocPane && len(doc.Headings) > 0 && m3.IconBtn(c, icons.TocRounded, l10n.CommonContents).Clicked() {
				m3.ShowModalBottomSheet(m3.SheetOptions{HeightFactor: .6}, func(c *ui.Context, r *m3.Route) {
					ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() { p.reader.ShowToc(c, r.Close) })
				})
			}
		}
		m3.HGap(c, design.SpaceSM)
	}}
	if doc != nil {
		bar.Bottom = func() { m3.Linear(c, m3.Progress{Value: m3.Of(float64(p.reader.Progress)), Height: 2, NoTrack: true}) }
	}
	m3.Scaffold{AppBar: bar, Body: func() {
		body := ui.Row(c).Fill()
		if w := body.Bounds().W; w > 0 {
			p.width = w
		} else if p.width == 0 {
			p.width = WindowWidth
		}
		mark := any(nil)
		if p.reader != nil {
			mark = p.reader.Current
		}
		p.activity.Show(c, body, "read", p.lectureID, mark)
		body.Children(func() {
			switch {
			case p.err != nil:
				widgets.ErrorState(c, p.err, nil)
			case doc == nil:
				widgets.CenteredLoader(c, "")
			default:
				if tocPane {
					ui.Column(c).Width(280).Shrink(0).FillHeight().Children(func() { p.reader.ShowToc(c, nil) })
				}
				ui.Column(c).Grow(1).Basis(0).FillHeight().Children(func() {
					if p.search {
						p.reader.ShowSearchBar(c, func() { p.search = false; p.reader.CloseSearch() })
					}
					ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() { p.reader.Show(c) })
				})
			}
		})
	}}.Show(c)
}

// OpenRef is openRef: it opens a reference where no supporting pane exists
// (chat, the player's panel). Timestamps seek (onSeek, when set, instead),
// slides and courseware open in a sheet, lectures in the player and
// library files in the viewer or the system's app.
func OpenRef(ref CoursiveRef, lectureID string, onSeek func(CoursiveRef)) {
	switch ref.Kind {
	case RefLecture:
		m3.RootNav.Push(NewPlayerPage(ref.LectureID, -1))
		return
	case RefFile:
		OpenLibraryFile(ref.Path)
		return
	case RefTime:
		if onSeek != nil {
			onSeek(ref)
			return
		}
		id := ref.LectureID
		if id == "" {
			id = lectureID
		}
		SeekOrOpen(id, ref.Seconds)
		return
	}
	pane := NewSourcePane(ref, lectureID)
	m3.ShowModalBottomSheet(m3.SheetOptions{ScrollControlled: true, MaxWidth: 720, HeightFactor: .85}, func(c *ui.Context, r *m3.Route) {
		ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() { pane.Show(c, nil) })
	})
}

// NoteInlineReader is a note read inside a narrow pane (the player's side
// panel): contents in a menu, timestamps seek the player in place.
type NoteInlineReader struct {
	note     core.Note
	reader   *MdReader
	OnBack   func()
	OnEdit   func()
	OnSeek   func(CoursiveRef)
	activity ActivityRegion
}

// NewNoteInlineReader reads n.
func NewNoteInlineReader(n core.Note) *NoteInlineReader {
	r := &NoteInlineReader{}
	r.Set(n)
	return r
}

// Set shows n, keeping the place when its text is the same.
func (r *NoteInlineReader) Set(n core.Note) {
	same := r.reader != nil && r.note.Body() == n.Body()
	r.note = n
	if same {
		return
	}
	r.reader = NewMdReader(NewMdDoc(n.Body()), "")
	r.reader.LectureID = n.LectureID()
	r.reader.BaseDir = path.Dir(n.Path())
	r.reader.OnRef = func(ref CoursiveRef) { OpenRef(ref, n.LectureID(), r.OnSeek) }
	r.reader.Padding = []float32{design.SpaceMD, design.SpaceSM, design.SpaceXXL, design.SpaceSM}
}

// Dispose stops counting reading time.
func (r *NoteInlineReader) Dispose() { r.activity.Stop() }

// Show builds the reader.
func (r *NoteInlineReader) Show(c *ui.Context) {
	n := r.note
	tt := design.TT()
	// Reading beside the player counts like the full note page.
	col := ui.Column(c).Fill()
	r.activity.Show(c, col, "read", n.LectureID(), r.reader.Current)
	col.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceXS, design.SpaceXS, 0, design.SpaceXS).Children(func() {
			if m3.IconBtn(c, icons.ArrowBackRounded, l10n.CommonBack).Clicked() && r.OnBack != nil {
				r.OnBack()
			}
			m3.Text(c, n.Title(), tt.TitleMedium).MaxLines(1).Grow(1).Basis(0)
			if hs := r.reader.Doc.Headings; len(hs) > 0 {
				m3.MenuButton(c, "toc", icons.TocRounded, l10n.CommonContents, m3.Below, func() []m3.MenuItem {
					var items []m3.MenuItem
					for _, h := range hs {
						indent := ""
						for i := 0; i < min(max(h.Level-2, 0), 3); i++ {
							indent += "    "
						}
						items = append(items, m3.MenuItem{Label: indent + h.Title, OnTap: func() { r.reader.JumpToSection(h.Section, false) }})
					}
					return items
				})
			}
			if m3.IconBtn(c, icons.EditOutlined, l10n.CommonEdit).Clicked() && r.OnEdit != nil {
				r.OnEdit()
			}
			if m3.IconBtn(c, icons.OpenInFullRounded, l10n.ReaderOpen).Clicked() {
				m3.RootNav.Push(NewNotePage(n.ID()))
			}
		})
		m3.Linear(c, m3.Progress{Value: m3.Of(float64(r.reader.Progress)), Height: 2, NoTrack: true})
		ui.Column(c).FillWidth().Grow(1).Basis(0).Children(func() { r.reader.Show(c) })
	})
}

var imageExts = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp"}

// OpenLibraryFile is openLibraryFile: images in the viewer, other files
// in the system's app.
func OpenLibraryFile(p string) {
	rel := path.Clean(p)
	if rel == ".." || len(rel) > 2 && rel[:3] == "../" || path.IsAbs(rel) {
		widgets.ShowSnack(l10n.LinkNotInLibrary, "", nil)
		return
	}
	if slices.Contains(imageExts, strings.ToLower(path.Ext(rel))) {
		ShowImageViewer(State.Api().FileURL(rel))
		return
	}
	root := State.Daemon.LibraryRoot()
	if root == "" || !core.OpenPath(root+"/"+rel) {
		widgets.ShowSnack(l10n.LinkCannotOpenFile(path.Base(rel)), "", nil)
	}
}
