package app

import (
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// study_shelf_ui.dart: the lecture's material, out of the player's tabs.
// Subtitles, files, practice, notes and conversations sit on a shelf
// beside the canvas and drag onto it as ordinary cards; files fan out like
// a small stack of paper. Contract: docs/study-canvas.md (课次素材).

// studyAsset is something on the shelf that becomes a card where it is
// dropped: segments, file, quiz, note, thread or new-thread.
type studyAsset struct {
	kind, title string
	data        any
	// size is the card's world size, for the drop outline.
	size study.Pt
}

func newAsset(kind, title string, data any, w, h float64) *studyAsset {
	return &studyAsset{kind: kind, title: title, data: data, size: study.Pt{X: w, Y: h}}
}

// shelfParts is the shelf's state; shown is the tab last open, kept so the
// shelf can slide away with its content.
type shelfParts struct {
	shelfShown shelfTab
	shelfData  shelfState
	// dropHint is where a dragged asset would land, in view coordinates.
	dropHint *study.Rect
}

type shelfState struct {
	files              []core.J
	quizzes            []Quiz
	filesErr, quizErr  string
	filesBusy, quizBus bool
	openGroup          string
	transcript         ui.ListState
	fanOpen            map[string]bool
	fanLift            map[string]int
}

var shelfTabs = []struct {
	tab   shelfTab
	label string
	icon  icons.Icon
	name  string
}{
	{shelfTranscript, l10n.StudyShelfTranscript, icons.SubtitlesOutlined, "transcript"},
	{shelfFiles, l10n.StudyShelfFiles, icons.FolderCopyOutlined, "files"},
	{shelfPractice, l10n.StudyShelfPractice, icons.QuizOutlined, "practice"},
	{shelfNotes, l10n.StudyShelfNotes, icons.StickyNote2Outlined, "notes"},
	{shelfThreads, l10n.StudyShelfThreads, icons.ForumOutlined, "threads"},
}

// shelfSide puts the shelf beside the canvas on wide windows and above the
// bottom edge on phones.
func (s *StudyCanvas) shelfSide() bool { return s.viewport.X >= 720 }

// toggleShelf opens tab (the last one for shelfNone), or closes the shelf
// when it already shows it.
func (s *StudyCanvas) toggleShelf(tab shelfTab) {
	if s.shelfTab == shelfNone || (tab != shelfNone && tab != s.shelfTab) {
		switch {
		case tab != shelfNone:
			s.shelfTab = tab
		case s.shelfShown != shelfNone:
			s.shelfTab = s.shelfShown
		default:
			s.shelfTab = shelfTranscript
		}
	} else {
		s.shelfTab = shelfNone
	}
	if s.shelfTab != shelfNone {
		s.shelfShown = s.shelfTab
	}
}

func (s *StudyCanvas) shelf(c *ui.Context) {
	tab := s.shelfTab
	if tab == shelfNone {
		return
	}
	s.loadShelf(tab)
	side := s.shelfSide()
	top := math.Max(float64(design.SpaceXXXL+design.SpaceXL+studyDockMargin), s.topBarsBottom()+8)
	e := ui.Column(c.Key("study-shelf")).Absolute().AlignItems(ui.Stretch)
	if side {
		e = e.Left(design.SpaceMD).Top(f32(top)).Bottom(design.SpaceXXXL + design.SpaceLG).Width(340).
			Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, ExitDuration: design.Medium2, ExitEase: design.Emphasized,
				Enter: &ui.Motion{Opacity: 1, X: -360}, Exit: &ui.Motion{Opacity: 1, X: -360}, Shadows: true})
	} else {
		e = e.Left(design.SpaceSM).Right(design.SpaceSM).Bottom(design.SpaceSM).Height(f32(math.Min(480, s.viewport.Y*.62))).
			Transition(ui.ElementTransition{Duration: design.Medium2, Ease: design.Emphasized, ExitDuration: design.Medium2, ExitEase: design.Emphasized,
				Enter: &ui.Motion{Opacity: 1, YPercent: 110}, Exit: &ui.Motion{Opacity: 1, YPercent: 110}, Shadows: true})
	}
	widgets.Glass{Radius: design.CornerXL, Depth: design.GlassPanel}.Apply(e)
	s.recordControl(e)
	cs, tt := design.S(), design.TT()
	e.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Padding(design.SpaceMD, design.SpaceXS, 0, design.SpaceLG).Children(func() {
			m3.Text(c, l10n.StudyShelf, tt.TitleMedium.C(cs.OnSurface)).Grow(1).Basis(0)
			if (m3.IconButton{Icon: icons.ViewSidebarOutlined, Size: 20, Tooltip: l10n.StudyShelfPanel}).Show(c).Clicked() {
				s.openPlayerPanel()
			}
			if (m3.IconButton{Icon: icons.CloseRounded, Size: 20, Tooltip: l10n.CommonClose}).Show(c).Clicked() {
				s.shelfTab = shelfNone
			}
		})
		hint := l10n.StudyShelfHintTouch
		if side {
			hint = l10n.StudyShelfHint
		}
		m3.Text(c, hint, tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(0, design.SpaceLG)
		// Five sections side by side, styled like the navigation dock.
		ui.Row(c).Padding(design.SpaceMD, design.SpaceMD, design.SpaceSM, design.SpaceMD).Children(func() {
			for _, t := range shelfTabs {
				on := tab == t.tab
				b := ui.ButtonBase(c.Key("study-shelf-" + t.name)).Column().Grow(1).Basis(0).AlignItems(ui.Center).Checked(on).Label(t.label).Cursor(ui.CursorPointer)
				m3.Ink(b, cs.OnSurface, design.CornerMD)
				b.Children(func() {
					pill := ui.Box(c).Size(48, 30).Radius(15).Center()
					pill.Background(pill.AnimateColor("bg", map[bool]ui.Color{true: cs.SecondaryContainer, false: cs.SecondaryContainer.Alpha(0)}[on], ui.Animation{Duration: design.Short4}))
					fg := cs.OnSurfaceVariant
					if on {
						fg = cs.OnSecondaryContainer
					}
					pill.Children(func() { m3.Icon(c, t.icon, 20, fg) })
					ui.Box(c).Height(design.SpaceXXS)
					style := tt.LabelSmall.C(cs.OnSurfaceVariant).W(500)
					if on {
						style = tt.LabelSmall.C(cs.OnSurface).W(600)
					}
					m3.Text(c, t.label, style)
				})
				if b.Clicked() {
					s.shelfTab, s.shelfShown = t.tab, t.tab
				}
			}
		})
		ui.Box(c.Key(int(tab))).FillWidth().Grow(1).Basis(0).
			Transition(design.Switcher(design.Short4, nil, nil, ui.Motion{Opacity: 0})).Children(func() {
			switch tab {
			case shelfTranscript:
				s.transcriptList(c)
			case shelfFiles:
				s.fileStacks(c)
			case shelfPractice:
				s.practiceList(c)
			case shelfNotes:
				s.noteList(c)
			case shelfThreads:
				s.threadList(c)
			}
		})
	})
}

// loadShelf reads files and practice the first time their tab shows.
func (s *StudyCanvas) loadShelf(tab shelfTab) {
	st := &s.shelfData
	api := State.Api()
	if tab == shelfFiles && st.files == nil && st.filesErr == "" && !st.filesBusy {
		if s.ctl.Lecture == nil || s.ctl.Lecture.CourseID() == "" {
			return
		}
		st.filesBusy = true
		course := s.ctl.Lecture.CourseID()
		core.Async(func() (any, error) { return api.LibraryCatalog(course) }, func(v any, err error) {
			st.filesBusy = false
			if err != nil {
				st.filesErr = err.Error()
				return
			}
			st.files = core.Objs(plainJSON(v))
			if st.files == nil {
				st.files = []core.J{}
			}
		})
	}
	if tab == shelfPractice && st.quizzes == nil && st.quizErr == "" && !st.quizBus {
		st.quizBus = true
		id := s.lectureID()
		core.Async(func() (any, error) { return api.Get("/api/quizzes", map[string]string{"lecture_id": id}) }, func(v any, err error) {
			st.quizBus = false
			if err != nil {
				st.quizErr = err.Error()
				return
			}
			st.quizzes = []Quiz{}
			for _, q := range core.Objs(v) {
				st.quizzes = append(st.quizzes, ParseQuiz(q))
			}
		})
	}
}

func (s *StudyCanvas) shelfEmpty(c *ui.Context, text string, retry func()) {
	cs, tt := design.S(), design.TT()
	ui.Column(c).Fill().Center().Padding(design.SpaceXL).Children(func() {
		m3.Text(c, text, tt.BodyMedium.C(cs.OnSurfaceVariant)).TextAlign(ui.Center)
		if retry != nil && m3.TextButton(c, l10n.CommonRetry).Clicked() {
			retry()
		}
	})
}

// shelfDrag makes e drag asset onto the canvas.
func (s *StudyCanvas) shelfDrag(e ui.Element, asset *studyAsset) ui.Element {
	e = e.Drag(asset)
	if e.Dragging() {
		e.Opacity(.35)
	}
	return e
}

func (s *StudyCanvas) shelfPlace(c *ui.Context, asset *studyAsset) {
	if (m3.IconButton{Icon: icons.AddRounded, Size: 20, Box: 36, Tooltip: l10n.StudyShelfPlace}).Show(c).Clicked() {
		s.placeAsset(asset, nil)
	}
}

type shelfRow struct {
	chapter *core.Chapter
	segment int
}

func (s *StudyCanvas) transcriptList(c *ui.Context) {
	segments := s.ctl.Segments
	if len(segments) == 0 {
		s.shelfEmpty(c, l10n.StudyShelfNoTranscript, nil)
		return
	}
	chapters := s.ctl.Chapters
	// Chapter headers interleave with their lines.
	var rows []shelfRow
	k := 0
	for i, seg := range segments {
		for k < len(chapters) && chapters[k].StartMs() <= seg.StartMs() {
			rows = append(rows, shelfRow{chapter: &chapters[k], segment: -1})
			k++
		}
		rows = append(rows, shelfRow{segment: i})
	}
	cs, tt := design.S(), design.TT()
	now := s.ctl.SegmentAt(s.ctl.PositionMs())
	st := &s.shelfData
	st.transcript.Key = func(i int) any {
		if i >= len(rows) {
			return i
		}
		if r := rows[i]; r.chapter != nil {
			return "c" + r.chapter.Title() + core.FmtClock(r.chapter.StartMs())
		}
		return rows[i].segment
	}
	ui.Box(c).Fill().Children(func() {
		ui.List(c, &st.transcript, len(rows), func(i int) {
			row := rows[i]
			ui.Box(c).Padding(0, design.SpaceSM).FillWidth().Children(func() {
				if ch := row.chapter; ch != nil {
					var inside []core.Segment
					for _, seg := range segments {
						if seg.StartMs() >= ch.StartMs() && (ch.EndMs() <= 0 || seg.StartMs() < ch.EndMs()) {
							inside = append(inside, seg)
						}
					}
					asset := newAsset("segments", ch.Title(), inside, 420, 320)
					e := ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceSM, design.SpaceXXS, design.SpaceSM, design.SpaceSM)
					s.shelfDrag(e, asset).Children(func() {
						ui.Column(c).Grow(1).Basis(0).Children(func() {
							m3.Text(c, ch.Title(), tt.TitleSmall.C(cs.OnSurface)).MaxLines(2)
							m3.Text(c, l10n.StudyShelfChapterLines(len(inside)), tt.LabelSmall.C(cs.OnSurfaceVariant))
						})
						if len(inside) > 0 {
							s.shelfPlace(c, asset)
						}
					})
					return
				}
				seg := segments[row.segment]
				playing := row.segment == now
				asset := newAsset("segments", seg.Text(), []core.Segment{seg}, 420, 220)
				e := ui.ButtonBase(c).FillWidth().Row().AlignItems(ui.Start).Radius(design.CornerMD).
					Padding(design.SpaceSM, design.SpaceXXS, design.SpaceSM, design.SpaceSM)
				if playing {
					e = e.Background(cs.SecondaryContainer)
				}
				m3.Ink(e, cs.OnSurface, design.CornerMD)
				s.shelfDrag(e, asset).Children(func() {
					m3.Text(c, core.FmtClock(seg.StartMs()), tt.LabelMedium.C(cs.Primary)).FontFeatures("tnum").Width(48).Shrink(0)
					col := cs.OnSurface
					if playing {
						col = cs.OnSecondaryContainer
					}
					m3.Text(c, seg.Text(), tt.BodyMedium.C(col).H(1.45)).MaxLines(3).Grow(1).Basis(0)
					s.shelfPlace(c, asset)
				})
				if e.Clicked() {
					s.ctl.SeekTo(seg.StartMs())
				}
			})
		}).Fill()
		fab := ui.Box(c).Absolute().Right(design.SpaceMD).Bottom(design.SpaceMD)
		fab.Children(func() {
			if (m3.FAB{Icon: icons.MyLocationRounded, Small: true, Tooltip: l10n.StudyShelfNow}).Show(c).Clicked() {
				at := s.ctl.SegmentAt(s.ctl.PositionMs())
				for i, r := range rows {
					if r.chapter == nil && r.segment == max(0, at) {
						st.transcript.ScrollTo(i, ui.Center)
						break
					}
				}
			}
		})
	})
	if s.ctl.Playing() {
		c.After(500 * time.Millisecond)
	}
}

func (s *StudyCanvas) fileStacks(c *ui.Context) {
	st := &s.shelfData
	if s.ctl.Lecture == nil || s.ctl.Lecture.CourseID() == "" {
		s.shelfEmpty(c, l10n.StudyShelfNoCourse, nil)
		return
	}
	if st.filesErr != "" {
		s.shelfEmpty(c, l10n.StudyShelfLoadFailed(st.filesErr), func() { st.filesErr = "" })
		return
	}
	if st.files == nil {
		ui.Box(c).Fill().Center().Children(func() { m3.Circular(c, m3.Progress{}) })
		return
	}
	if len(st.files) == 0 {
		s.shelfEmpty(c, l10n.StudyShelfNoFiles, nil)
		return
	}
	// This lecture's files first, then one stack per kind of material.
	var order []string
	groups := map[string][]core.J{}
	for _, f := range st.files {
		key := l10n.StudyShelfFiles
		switch {
		case f.S("lecture_id") == s.lectureID():
			key = l10n.StudyShelfThisLecture
		case f.S("category") != "":
			key = f.S("category")
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], f)
	}
	if files, ok := groups[st.openGroup]; ok && st.openGroup != "" {
		s.openStack(c, st.openGroup, files)
		return
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Grid(c).Columns(2).Gap(design.SpaceMD).Padding(0, design.SpaceMD, design.SpaceLG, design.SpaceMD).Children(func() {
			for _, key := range order {
				s.fileFan(c, key, groups[key])
			}
		})
	})
}

func studyImportable(f core.J) bool {
	t := strings.ToLower(f.S("file_type"))
	for _, k := range studyImportTypes {
		if k == t {
			return true
		}
	}
	return false
}

func fileAsset(f core.J) *studyAsset {
	return newAsset("file", f.S("name"), f.S("path"), 320, 420)
}

// fileDrag makes e drag f, or dims it with why it cannot.
func (s *StudyCanvas) fileDrag(e ui.Element, f core.J) ui.Element {
	if !studyImportable(f) {
		return e.Opacity(.45).Tooltip(l10n.StudyShelfUnsupported)
	}
	return s.shelfDrag(e, fileAsset(f))
}

func (s *StudyCanvas) openStack(c *ui.Context, label string, files []core.J) {
	cs, tt := design.S(), design.TT()
	st := &s.shelfData
	ui.Column(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Padding(0, design.SpaceSM).Children(func() {
			if (m3.IconButton{Icon: icons.ArrowBackRounded, Size: 20, Tooltip: l10n.StudyShelfBack}).Show(c).Clicked() {
				st.openGroup = ""
			}
			m3.Text(c, label, tt.TitleSmall.C(cs.OnSurface)).MaxLines(1).Grow(1).Basis(0)
			m3.Text(c, l10n.StudyShelfFilesCount(len(files)), tt.LabelSmall.C(cs.OnSurfaceVariant))
			ui.Box(c).Width(design.SpaceMD)
		})
		ui.Scroll(c).FillWidth().Grow(1).Basis(0).Children(func() {
			ui.Grid(c).Columns(3).GapX(design.SpaceSM).GapY(design.SpaceMD).Padding(design.SpaceSM, design.SpaceMD, design.SpaceLG, design.SpaceMD).Children(func() {
				for _, f := range files {
					e := ui.ButtonBase(c.Key(f.S("path"))).Column().AlignItems(ui.Center).Radius(design.CornerMD)
					m3.Ink(e, cs.OnSurface, design.CornerMD)
					s.fileDrag(e, f).Children(func() {
						fileTile(c, f)
						ui.Box(c).Height(design.SpaceXS)
						m3.Text(c, f.S("name"), tt.LabelSmall.C(cs.OnSurface)).MaxLines(2).TextAlign(ui.Center)
					})
					if e.Clicked() && studyImportable(f) {
						s.placeAsset(fileAsset(f), nil)
					}
				}
			})
		})
	})
}

// fileFan is _FileFan: up to five files fanned like paper in a hand.
// Hovering opens the fan and lifts the file under the pointer, which drags
// straight onto the canvas; a click lays all its files out.
func (s *StudyCanvas) fileFan(c *ui.Context, label string, files []core.J) {
	cs, tt := design.S(), design.TT()
	st := &s.shelfData
	if st.fanOpen == nil {
		st.fanOpen, st.fanLift = map[string]bool{}, map[string]int{}
	}
	shown := files[:min(5, len(files))]
	mid := float32(len(shown)-1) / 2
	open := st.fanOpen[label]
	b := ui.ButtonBase(c.Key("study-fan-" + label)).Column().AlignItems(ui.Center).Label(label + " " + l10n.StudyShelfFilesCount(len(files)))
	b.Children(func() {
		stack := ui.Box(c).FillWidth().Height(120)
		w := stack.Bounds().W
		if w == 0 {
			w = 150
		}
		stack.Children(func() {
			for i, f := range shown {
				fi := float32(i)
				x, y, rot := (fi-mid)*3, -fi*1.5, (fi-mid)*.06
				if open {
					x, y, rot = (fi-mid)*w/7, -4, (fi-mid)*.2
				}
				if lift, ok := st.fanLift[label]; ok && lift == i && open {
					y = -16
				}
				sheet := ui.Box(c.Key(i)).Absolute().Bottom(0).Left(w/2-32).Size(64, 84).TransformOrigin(.5, 1)
				ax := sheet.AnimateTo("x", x, design.T().Motion(design.Medium1, design.Emphasized))
				ay := sheet.AnimateTo("y", y, design.T().Motion(design.Medium1, design.Emphasized))
				ar := sheet.AnimateTo("r", rot*180/math.Pi, design.T().Motion(design.Medium1, design.Emphasized))
				sheet = sheet.Translate(ax, ay).Rotate(ar)
				if sheet.Hovered() && st.fanLift[label] != i {
					st.fanLift[label] = i
				}
				s.fileDrag(sheet, f).Children(func() { fileTile(c, f) })
			}
		})
		ui.Box(c).Height(design.SpaceSM)
		m3.Text(c, label, tt.LabelLarge.C(cs.OnSurface)).MaxLines(1)
		m3.Text(c, l10n.StudyShelfFilesCount(len(files)), tt.LabelSmall.C(cs.OnSurfaceVariant))
	})
	if b.Hovered() != open {
		st.fanOpen[label] = b.Hovered()
		if !b.Hovered() {
			delete(st.fanLift, label)
		}
	}
	if b.Clicked() {
		st.openGroup = label
	}
}

// fileTile is _FileTile: a file as a small sheet of paper with a folded
// corner and its type.
func fileTile(c *ui.Context, f core.J) {
	cs, tt := design.S(), design.TT()
	kind := strings.ToLower(f.S("file_type"))
	fill, ink := cs.SurfaceContainerHighest, cs.OnSurfaceVariant
	switch kind {
	case "pdf":
		fill, ink = cs.TertiaryContainer, cs.OnTertiaryContainer
	case "png", "jpg", "jpeg", "gif", "webp":
		fill, ink = cs.SecondaryContainer, cs.OnSecondaryContainer
	case "mp4", "mov", "m4v", "webm", "mkv":
		fill, ink = cs.PrimaryContainer, cs.OnPrimaryContainer
	}
	dark := design.T().Dark
	paper, edge := cs.SurfaceContainerLowest, cs.OnSurface.Alpha(.1)
	if dark {
		paper, edge = cs.SurfaceContainerHighest, cs.OnSurface.Alpha(.16)
	}
	ui.Column(c).Size(64, 84).Shrink(0).Padding(22, 7, 7, 7).Draw(func(p *ui.Painter, r ui.Rect) {
		const corner, rad = 14, 6
		x, y, w, h := r.X, r.Y, r.W, r.H
		body := (&ui.Path{}).MoveTo(x+rad, y).LineTo(x+w-corner, y).LineTo(x+w, y+corner).LineTo(x+w, y+h-rad).
			QuadTo(x+w, y+h, x+w-rad, y+h).LineTo(x+rad, y+h).QuadTo(x, y+h, x, y+h-rad).LineTo(x, y+rad).QuadTo(x, y, x+rad, y).Close()
		if !dark {
			m3.PaintElevation(p, r, rad, 2)
		}
		p.FillPath(body, paper)
		p.StrokePath(body, .8, edge)
		flap := (&ui.Path{}).MoveTo(x+w-corner, y).LineTo(x+w-corner, y+corner-3).QuadTo(x+w-corner, y+corner, x+w-corner+3, y+corner).
			LineTo(x+w, y+corner).Close()
		p.FillPath(flap, fill)
	}).Children(func() {
		m3.Text(c, f.S("name"), tt.LabelSmall.S(9).H(1.25).C(cs.OnSurfaceVariant)).MaxLines(3).Grow(1).Basis(0).Clip()
		label := strings.ToUpper(kind)
		if label == "" {
			label = "?"
		}
		ui.Box(c).Radius(design.CornerFull).Background(fill).Padding(1, 5).AlignSelf(ui.Start).Children(func() {
			m3.Text(c, label, tt.LabelSmall.S(9).W(700).C(ink).L(.4))
		})
	})
}

func (s *StudyCanvas) practiceList(c *ui.Context) {
	st := &s.shelfData
	if st.quizErr != "" {
		s.shelfEmpty(c, l10n.StudyShelfLoadFailed(st.quizErr), func() { st.quizErr = "" })
		return
	}
	if st.quizzes == nil {
		ui.Box(c).Fill().Center().Children(func() { m3.Circular(c, m3.Progress{}) })
		return
	}
	if len(st.quizzes) == 0 {
		s.shelfEmpty(c, l10n.StudyShelfNoPractice, nil)
		return
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(0, design.SpaceSM, design.SpaceLG, design.SpaceSM).Children(func() {
			for _, q := range st.quizzes {
				asset := newAsset("quiz", q.Title, q.ID, 520, 560)
				e := ui.Box(c.Key(q.ID)).FillWidth()
				s.shelfDrag(e, asset).Children(func() {
					m3.Tile{LeadingIcon: icons.QuizOutlined, Title: q.Title, TitleLines: 2, Subtitle: q.Meta(), SubtitleLines: 1,
						Trailing: func() { s.shelfPlace(c, asset) }}.Show(c)
				})
			}
		})
	})
}

func (s *StudyCanvas) noteList(c *ui.Context) {
	notes := s.ctl.Notes
	if len(notes) == 0 {
		s.shelfEmpty(c, l10n.StudyShelfNoNotes, nil)
		return
	}
	space := regexp.MustCompile(`\s+`)
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(0, design.SpaceSM, design.SpaceLG, design.SpaceSM).Children(func() {
			for _, n := range notes {
				title := n.Title()
				if title == "" {
					title = l10n.StudyShelfNotes
				}
				asset := newAsset("note", title, n.ID(), 480, 420)
				e := ui.Box(c.Key(n.ID())).FillWidth()
				s.shelfDrag(e, asset).Children(func() {
					m3.Tile{LeadingIcon: icons.StickyNote2Outlined, Title: title, TitleLines: 1, Subtitle: space.ReplaceAllString(n.Body(), " "),
						SubtitleLines: 2, Trailing: func() { s.shelfPlace(c, asset) }}.Show(c)
				})
			}
		})
	})
}

func (s *StudyCanvas) threadList(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(0, design.SpaceSM, design.SpaceLG, design.SpaceSM).Children(func() {
			fresh := newAsset("new-thread", l10n.StudyThreadNew, nil, 300, 200)
			e := ui.Box(c.Key("new")).FillWidth()
			s.shelfDrag(e, fresh).Children(func() {
				m3.Tile{LeadingIcon: icons.AddCommentOutlined, Title: l10n.StudyThreadNew, Subtitle: l10n.StudyShelfThreadHint,
					Trailing: func() { s.shelfPlace(c, fresh) }}.Show(c)
			})
			for _, t := range s.threads() {
				turns := 0
				if cv := s.conversations[t.ID]; cv != nil && cv.data != nil {
					turns = len(cv.turns())
				}
				asset := newAsset("thread", t.Title, t.ID, t.Rect.W(), t.Rect.H())
				e := ui.Box(c.Key(t.ID)).FillWidth()
				var tile ui.Element
				s.shelfDrag(e, asset).Children(func() {
					col := ui.Color{}
					if t.ID == s.activeConversation {
						col = design.S().Primary
					}
					tile = m3.Tile{Leading: func() { m3.Icon(c, icons.ForumOutlined, 24, orColor(col, design.S().OnSurfaceVariant)) },
						Title: t.Title, Subtitle: l10n.StudyThreadTurns(turns), Tap: true}.Show(c)
				})
				if tile.Clicked() {
					s.activateThread(t.ID)
				}
			}
		})
	})
}

func orColor(c, fallback ui.Color) ui.Color {
	if c.A == 0 {
		return fallback
	}
	return c
}

// assetDrops takes shelf items anywhere on the canvas (input) and outlines
// where they land.
func (s *StudyCanvas) assetDrops(input ui.Element) {
	d := s.doc
	if a, ok := ui.DragOver[*studyAsset](input); ok {
		if x, y, in := input.PointerPosition(); in {
			r := study.RectCenter(study.Pt{X: float64(x), Y: float64(y)}, a.size.X*d.Zoom, a.size.Y*d.Zoom)
			s.dropHint = &r
		}
	} else {
		s.dropHint = nil
	}
	if a, ok := ui.Drop[*studyAsset](input); ok {
		var at *study.Pt
		if x, y, _ := input.PointerPosition(); x != 0 || y != 0 {
			w := s.world(study.Pt{X: float64(x), Y: float64(y)})
			at = &w
		}
		s.dropHint = nil
		s.placeAsset(a, at)
	}
}

func (s *StudyCanvas) dropOutline(c *ui.Context) {
	r := s.dropHint
	if r == nil {
		return
	}
	primary := design.S().Primary
	ui.Box(c.Key("study-drop-outline")).Absolute().Top(0).Left(0).Right(0).Bottom(0).IgnoreInput(true).Draw(func(p *ui.Painter, b ui.Rect) {
		rect := ui.Rect{X: b.X + f32(r.L), Y: b.Y + f32(r.T), W: f32(r.W()), H: f32(r.H())}
		path := design.RoundedRectPath(rect, design.CornerLG)
		p.FillPath(path, primary.Alpha(.07))
		p.StrokePath(design.RoundedRectPath(rect, design.CornerLG).Dash(0, 7, 5), 1.5, primary.Alpha(.7))
	})
}

// placeAsset places a centred on at (world coordinates), or in view when
// nil.
func (s *StudyCanvas) placeAsset(a *studyAsset, at *study.Pt) {
	d := s.doc
	if d == nil {
		return
	}
	if !d.Editable(d.ActiveLayer) {
		s.toast(l10n.StudyPickLayer)
		return
	}
	if at == nil {
		w := s.world(s.viewport.Mul(.5))
		at = &w
	}
	switch a.kind {
	case "file":
		s.importMaterial(a.data.(string), true, at)
		return
	case "new-thread":
		s.startConversation(conversationStart{at: at}, nil)
		return
	case "thread":
		card := d.Card(a.data.(string))
		if card == nil {
			return
		}
		d.Checkpoint()
		card.Rect = study.RectCenter(*at, card.Rect.W(), card.Rect.H())
		d.Changed()
		s.activateThread(card.ID)
		return
	}
	add := func(card *study.Card) {
		if card == nil || s.disposed || s.doc != d {
			return
		}
		if len(d.Cards) >= 2000 {
			s.toast(l10n.StudyCardLimit)
			return
		}
		d.Checkpoint()
		d.Cards = append(d.Cards, card)
		d.Changed()
		clear(s.selection)
		s.selection[card.ID] = true
	}
	where := *at
	switch a.kind {
	case "segments":
		add(s.transcriptCard(a.data.([]core.Segment), where))
	case "quiz", "note":
		id := a.data.(string)
		api := State.Api()
		core.Async(func() (any, error) {
			if a.kind == "quiz" {
				return api.Get("/api/quizzes/" + core.Esc(id))
			}
			return api.Get("/api/notes/" + core.Esc(id))
		}, func(v any, err error) {
			if err != nil {
				s.toast(l10n.StudyShelfLoadFailed(err.Error()))
				return
			}
			if a.kind == "quiz" {
				add(s.practiceCard(ParseQuiz(core.Obj(v)), where))
			} else {
				add(s.noteCard(core.Note{J: core.Obj(v)}, where))
			}
		})
	}
}

func (s *StudyCanvas) transcriptCard(all []core.Segment, at study.Pt) *study.Card {
	if len(all) == 0 {
		return nil
	}
	// Eight lines a paragraph, within the 32 blocks a card holds.
	segments := all[:min(256, len(all))]
	var blocks []study.Block
	for i := 0; i < len(segments); i += 8 {
		var lines []string
		for _, seg := range segments[i:min(i+8, len(segments))] {
			lines = append(lines, core.FmtClock(seg.StartMs())+"  "+seg.Text())
		}
		blocks = append(blocks, study.Block{Type: "paragraph", Text: strings.Join(lines, "\n")})
	}
	if len(all) > len(segments) {
		blocks = append(blocks, study.Block{Type: "paragraph", Text: l10n.StudyTranscriptExcerpt(len(segments))})
	}
	first, last := segments[0], segments[len(segments)-1]
	quote := []rune(first.Text())
	return &study.Card{ID: study.NewID(), Type: "note", Layer: s.doc.ActiveLayer, Rect: study.RectCenter(at, 420, 320),
		Title:  l10n.StudyTranscriptTitle(core.FmtClock(first.StartMs()) + "–" + core.FmtClock(last.EndMs())),
		Blocks: blocks, Responses: map[string]string{},
		Sources: []study.Anchor{{ID: study.NewID(), ObjectID: "lecture", Kind: "video", WorkspaceID: s.lectureID(),
			TimeStartMs: first.StartMs(), TimeEndMs: max(first.StartMs(), last.EndMs()), Quote: string(quote[:min(200, len(quote))])}}}
}

func (s *StudyCanvas) practiceCard(q Quiz, at study.Pt) *study.Card {
	if len(q.Questions) == 0 {
		return nil
	}
	const letters = "ABCDEFGH"
	var blocks []study.Block
	for i, qq := range q.Questions[:min(32, len(q.Questions))] {
		lines := []string{itoa(i+1) + ". " + qq.Stem}
		for j, o := range qq.Options[:min(8, len(qq.Options))] {
			lines = append(lines, letters[j:j+1]+". "+o.Text)
		}
		answer := ""
		if qq.Answer >= 0 && qq.Answer < len(qq.Options) && qq.Answer < 8 {
			parts := []string{l10n.StudyAnswerLabel(letters[qq.Answer : qq.Answer+1])}
			if r := qq.Options[qq.Answer].Rationale; r != "" {
				parts = append(parts, r)
			}
			answer = strings.Join(parts, "\n")
		}
		blocks = append(blocks, study.Block{Type: "exercise", Text: strings.Join(lines, "\n"), Hint: qq.Hint, Answer: answer})
	}
	return &study.Card{ID: study.NewID(), Type: "practice", Layer: s.doc.ActiveLayer, Rect: study.RectCenter(at, 520, 560),
		Title: q.Title, Blocks: blocks, Responses: map[string]string{}}
}

func (s *StudyCanvas) noteCard(n core.Note, at study.Pt) *study.Card {
	blocks := studyBlocksFromMarkdown(n.Body())
	if len(blocks) == 0 {
		blocks = []study.Block{{Type: "paragraph"}}
	}
	title := n.Title()
	if title == "" {
		title = l10n.StudyShelfNotes
	}
	return &study.Card{ID: study.NewID(), Type: "note", Layer: s.doc.ActiveLayer, Rect: study.RectCenter(at, 480, 420),
		Title: title, Blocks: blocks, Responses: map[string]string{}}
}

var (
	mdHeading = regexp.MustCompile(`^#{1,6}\s+(.*)`)
	mdBullet  = regexp.MustCompile(`^\s*[-*+]\s+(.*)`)
	mdStep    = regexp.MustCompile(`^\s*\d+[.)]\s+(.*)`)
)

// studyBlocksFromMarkdown reads headings, lists, steps, code and
// paragraphs from a note's Markdown, within the 32 blocks a card holds;
// anything past that joins the last paragraph.
func studyBlocksFromMarkdown(text string) []study.Block {
	var out []study.Block
	var para, items []string
	listing := false
	kind := ""
	flush := func() {
		if len(para) > 0 {
			out = append(out, study.Block{Type: "paragraph", Text: strings.Join(para, "\n")})
			para = nil
		}
		if listing && len(items) > 0 {
			out = append(out, study.Block{Type: kind, Items: items[:min(30, len(items))]})
		}
		items, listing = nil, false
	}
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t\r")
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "```") {
			flush()
			language := strings.TrimSpace(strings.TrimLeft(line, " \t")[3:])
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimLeft(lines[i], " \t"), "```"); i++ {
				code = append(code, lines[i])
			}
			out = append(out, study.Block{Type: "code", Text: strings.Join(code, "\n"), Language: language})
			continue
		}
		heading, bullet, step := mdHeading.FindStringSubmatch(line), mdBullet.FindStringSubmatch(line), mdStep.FindStringSubmatch(line)
		switch {
		case heading != nil:
			flush()
			out = append(out, study.Block{Type: "heading", Text: heading[1]})
		case bullet != nil || step != nil:
			k, m := "list", bullet
			if bullet == nil {
				k, m = "steps", step
			}
			if len(para) > 0 || kind != k {
				flush()
			}
			kind, listing = k, true
			items = append(items, m[1])
		case strings.TrimSpace(line) == "":
			flush()
		default:
			if listing {
				flush()
			}
			para = append(para, line)
		}
	}
	flush()
	if len(out) > 32 {
		var rest []string
		for _, b := range out[31:] {
			rest = append(rest, b.PlainText())
		}
		out = append(out[:31], study.Block{Type: "paragraph", Text: strings.Join(rest, "\n\n")})
	}
	return out
}
