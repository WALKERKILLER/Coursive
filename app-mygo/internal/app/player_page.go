package app

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// player_page.dart: a lecture's player, with the side panel beside the
// video on wide windows and below it on narrow ones.

// SetFullScreen enters or leaves the window's full screen; main sets it.
var SetFullScreen func(bool)

// playerPages are the open players' pages, so a seek can bring one back
// on top.
var playerPages = map[*LecturePlayer]*PlayerPage{}

// revealPlayer closes the pages that cover p's player (a note or
// practice opened from it).
func revealPlayer(p *LecturePlayer) {
	if page := playerPages[p]; page != nil {
		m3.RootNav.PopUntil(page)
	}
}

// SeekOrOpen seeks the open player when it shows lectureID (any lecture
// when empty) and brings it on top; otherwise it opens that lecture at
// seconds.
func SeekOrOpen(lectureID string, seconds int) {
	if p, ok := Players.Value.(*LecturePlayer); ok && p != nil && (lectureID == "" || p.LectureID() == lectureID) {
		p.SeekTo(seconds * 1000)
		revealPlayer(p)
		return
	}
	if lectureID == "" {
		widgets.ShowSnack(l10n.PlayerNoLectureAtTime, "", nil)
		return
	}
	m3.RootNav.Push(NewPlayerPage(lectureID, seconds*1000))
}

// PlayerPage is a lecture's player page.
type PlayerPage struct {
	lectureID  string
	startMs    int
	ctl        *LecturePlayer
	view       *PlayerView
	panel      *PlayerSidePanel
	assistant  *AssistantPanel
	fullscreen bool
	panelOpen  bool
	started    bool
	// canvas is the lecture's study canvas while the page shows it.
	canvas *StudyCanvas
	cancel func()
	// An arrow key held boosts the speed (Kazumi style).
	rightAt time.Time
	held    bool
}

// NewPlayerPage plays lecture id, from startMs when not negative.
func NewPlayerPage(id string, startMs int) m3.Page {
	ctl := NewLecturePlayer(id)
	p := &PlayerPage{lectureID: id, startMs: startMs, ctl: ctl, view: NewPlayerView(ctl), panel: NewPlayerSidePanel(ctl), panelOpen: true}
	p.assistant = NewAssistantPanel(true, true)
	p.assistant.Leave = func() { m3.RootNav.PopPage(p) }
	p.panel.Assistant = p.assistant.View
	p.panel.OnAsk = func(text string) { Assistant.Send(text) }
	playerPages[ctl] = p
	ctl.Load()
	p.cancel = State.Events.Listen(func(e core.J) {
		d := e.O("data")
		if e.S("type") == "lecture_changed" && d.S("reason") == "sync" {
			ctl.ReloadSyncedState()
			return
		}
		if d.S("lecture_id") != id {
			return
		}
		switch e.S("type") {
		case "lecture_changed":
			switch d.S("what") {
			case "timeline":
				ctl.ReloadTimeline()
			case "courseware":
				ctl.ReloadCourseware()
			default:
				ctl.ReloadTranscript()
				ctl.ReloadCourseware()
			}
		case "notes_changed":
			ctl.ReloadNotes()
		}
	})
	return p
}

// Dispose saves the position and stops playing.
func (p *PlayerPage) Dispose() {
	if p.canvas != nil {
		p.canvas.Dispose()
	}
	p.assistant.Dispose()
	if p.cancel != nil {
		p.cancel()
	}
	if Players.Value == core.PlayerHandle(p.ctl) {
		Players.Value = nil
	}
	if p.fullscreen {
		p.setFullscreen(false)
	}
	delete(playerPages, p.ctl)
	p.panel.Dispose()
	p.ctl.Close()
}

// CanPop leaves full screen before the page; on the canvas, the system
// back action first returns from a followed citation, and leaving saves
// the canvas.
func (p *PlayerPage) CanPop() bool {
	if p.fullscreen {
		p.setFullscreen(false)
		return false
	}
	if p.canvas != nil {
		if !p.canvas.ReturnFromSource() {
			p.leave()
		}
		return false
	}
	return true
}

// openCanvas shows the lecture on its study canvas.
func (p *PlayerPage) openCanvas() {
	if p.canvas != nil {
		return
	}
	var panelView func(c *ui.Context)
	panelView = func(c *ui.Context) { p.panel.Show(c) }
	p.canvas = NewStudyCanvas(p.ctl, func(c *ui.Context) { p.view.Show(c, p.canvasViewOpts()) }, panelView, p.classic, p.leave, func(text string) {
		p.ctl.PanelTab = "assistant"
		Assistant.Send(text)
	})
}

func (p *PlayerPage) canvasViewOpts() PlayerViewOpts {
	return PlayerViewOpts{Fullscreen: p.fullscreen, OnFullscreen: func() { p.setFullscreen(!p.fullscreen) }, OnBack: p.leave}
}

// flushCanvas saves the canvas; when it cannot, it says so and offers a
// recovery file, and done gets false.
func (p *PlayerPage) flushCanvas(done func(bool)) {
	if p.canvas == nil {
		done(true)
		return
	}
	canvas := p.canvas
	canvas.Flush(func(ok bool) {
		if ok {
			done(true)
			return
		}
		m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
			m3.AlertDialog(c, m3.Alert{Title: l10n.StudyUnsaved, Message: l10n.StudyUnsavedBody, Actions: func() {
				if m3.TextButton(c, l10n.StudyKeepEditing).Clicked() {
					r.Pop(nil)
				}
				if m3.FilledButton(c, l10n.StudyExportRecovery).Clicked() {
					r.Pop(nil)
					canvas.ExportRecovery()
				}
			}})
		}, nil)
		done(false)
	})
}

// classic leaves the canvas for the player.
func (p *PlayerPage) classic() {
	p.flushCanvas(func(ok bool) {
		if ok && p.canvas != nil {
			p.canvas.Dispose()
			p.canvas = nil
		}
	})
}

func (p *PlayerPage) setFullscreen(v bool) {
	if SetFullScreen != nil {
		SetFullScreen(v)
	}
	p.fullscreen = v
}

func (p *PlayerPage) leave() {
	if p.fullscreen {
		p.setFullscreen(false)
		return
	}
	p.flushCanvas(func(ok bool) {
		if ok {
			if p.canvas != nil {
				p.canvas.Dispose()
				p.canvas = nil
			}
			m3.RootNav.PopPage(p)
		}
	})
}

// keys are the player's shortcuts, while no text field has the focus.
func (p *PlayerPage) keys(c *ui.Context) {
	if c.FocusTakesText() || m3.AnyRoute() {
		return
	}
	if p.canvas != nil && p.canvas.Keys(c) {
		return
	}
	ctl, v := p.ctl, p.view
	step := core.UI.SeekStepSec
	now := time.Now()
	if c.Shortcut(0, ui.KeyRight) && !c.ShortcutRepeated() && p.rightAt.IsZero() {
		p.rightAt, p.held = now, false
	}
	if !p.rightAt.IsZero() {
		if c.KeyDown(ui.KeyRight) {
			if !p.held && now.Sub(p.rightAt) >= 350*time.Millisecond {
				p.held = true
				ctl.StartBoost()
			}
			c.After(50 * time.Millisecond)
		} else {
			if p.held {
				ctl.EndBoost()
			} else {
				v.SeekBy(step)
			}
			p.rightAt, p.held = time.Time{}, false
		}
	}
	key := func(k ui.Key, repeat bool) bool { return c.Shortcut(0, k) && (repeat || !c.ShortcutRepeated()) }
	switch {
	case key(ui.KeySpace, false), key(ui.KeyK, false):
		ctl.PlayOrPause()
	case key(ui.KeyLeft, true), key(ui.KeyJ, true):
		v.SeekBy(-step)
	case key(ui.KeyL, true):
		v.SeekBy(step)
	case key(ui.KeyUp, true):
		v.VolumeBy(5)
	case key(ui.KeyDown, true):
		v.VolumeBy(-5)
	case key(ui.KeyBracketRight, true):
		v.CycleSpeed(1)
	case key(ui.KeyBracketLeft, true):
		v.CycleSpeed(-1)
	case key(ui.KeyF, false):
		p.setFullscreen(!p.fullscreen)
	case p.fullscreen && key(ui.KeyEscape, false):
		p.setFullscreen(false)
	case key(ui.KeyM, false):
		if ctl.Volume() > 0 {
			ctl.SetVolume(0)
		} else {
			ctl.SetVolume(100)
		}
	case key(ui.KeyC, false):
		core.UI.Update(func(pr *design.Prefs) { pr.ShowSubtitles = !pr.ShowSubtitles })
	case key(ui.KeyB, false):
		v.addBookmark()
	case key(ui.KeyP, false):
		p.panelOpen = !p.panelOpen
	}
}

// View builds the page.
func (p *PlayerPage) View(c *ui.Context) {
	cs := design.S()
	ctl := p.ctl
	ctl.Tick()
	if !ctl.Loading && ctl.LoadError == nil && !p.started {
		p.started = true
		if p.startMs >= 0 {
			ctl.SeekTo(p.startMs)
		} else if ctl.ResumedFrom > 0 {
			p.view.ShowResume(ctl.ResumedFrom)
		}
		Players.Value = ctl
	}
	screen := map[string]string{"screen": l10n.ScreenPlayer, "lecture_id": p.lectureID, "has_transcript": "no"}
	if l := ctl.Lecture; l != nil {
		screen["lecture"] = l.Date() + " " + l.ShortTitle()
	}
	if ctl.Course != nil {
		screen["course"], screen["course_id"] = ctl.Course.Title(), ctl.Course.ID()
	}
	if len(ctl.Segments) > 0 {
		screen["has_transcript"] = "yes"
	}
	ReportScreen(screen)
	p.keys(c)
	width := WindowWidth
	wide := width >= 900
	bg := cs.SurfaceContainer
	if p.fullscreen {
		bg = ui.RGB(0, 0, 0)
	}
	ui.Box(c).Fill().Background(bg).Children(func() {
		switch {
		case ctl.Loading:
			ui.Box(c).Fill().Background(ui.RGB(0, 0, 0)).Children(func() { widgets.CenteredLoader(c, l10n.PlayerPreparing) })
		case ctl.LoadError != nil:
			m3.Scaffold{AppBar: &m3.AppBar{}, Body: func() { widgets.ErrorState(c, ctl.LoadError, ctl.Load) }}.Show(c)
		default:
			opts := PlayerViewOpts{Fullscreen: p.fullscreen, OnFullscreen: func() { p.setFullscreen(!p.fullscreen) }, OnBack: p.leave, OnCanvas: p.openCanvas}
			if wide {
				opts.PanelOpen = &p.panelOpen
				if !p.fullscreen {
					opts.OnTogglePanel = func() { p.panelOpen = !p.panelOpen }
				}
			}
			switch {
			case p.canvas != nil:
				// StudyCanvasSurface: the canvas follows the app's theme;
				// printed pages keep their own light background.
				surface := cs.SurfaceContainerLow
				top, right, bottom, left := c.SafeArea()
				if p.fullscreen {
					surface, top, right, bottom, left = ui.RGB(0, 0, 0), 0, 0, 0, 0
				}
				p.canvas.fullscreen = p.fullscreen
				ui.Box(c).Fill().Background(surface).Padding(top, right, bottom, left).Children(func() { p.canvas.Show(c) })
			case p.fullscreen:
				p.view.Show(c, opts)
			case wide:
				ui.Row(c).Fill().Children(func() {
					right := float32(design.SpaceMD)
					if p.panelOpen {
						right = 0
					}
					ui.Box(c).Grow(1).Basis(0).FillHeight().Padding(design.SpaceMD, right, design.SpaceMD, design.SpaceMD).Children(func() {
						ui.Box(c).Fill().Radius(design.CornerXL).Clip().Children(func() { p.view.Show(c, opts) })
					})
					target := float32(0)
					if p.panelOpen {
						target = 380
						if width >= 1400 {
							target = 440
						}
					}
					side := ui.Box(c).FillHeight().Shrink(0).Clip()
					w := side.Animate("panel", target, design.Medium4)
					side.Width(w).Children(func() {
						if w < 1 {
							return
						}
						ui.Box(c).Width(max(target, w)).FillHeight().Padding(design.SpaceMD).Children(func() {
							ui.Column(c).Fill().Radius(design.CornerXL).Clip().Background(cs.SurfaceContainerLow).Children(func() { p.panel.Show(c) })
						})
					})
				})
			default:
				top, _, bottom, _ := c.SafeArea()
				ui.Column(c).Fill().Padding(top, 0, bottom, 0).Children(func() {
					ui.Box(c).FillWidth().AspectRatio(16.0 / 9).Shrink(0).Children(func() { p.view.Show(c, opts) })
					ui.Column(c).FillWidth().Grow(1).Basis(0).Background(cs.Surface).Children(func() { p.panel.Show(c) })
				})
			}
		}
	})
}
