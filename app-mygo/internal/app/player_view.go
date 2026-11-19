package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// player_view.dart and seek_bar.dart: the video with custom MD3 controls,
// gestures and overlays.

var playerSpeeds = []float64{0.5, 0.75, 1.0, 1.25, 1.5, 1.75, 2.0, 2.5, 3.0}

// SpeedLabel is "1.5×".
func SpeedLabel(s float64) string {
	prec := 2
	if s == float64(int(s)) {
		prec = 1
	}
	t := strconv.FormatFloat(s, 'f', prec, 64)
	return strings.TrimSuffix(t, "0") + "×"
}

var (
	pWhite   = ui.RGB(255, 255, 255)
	pWhite60 = ui.RGBA(255, 255, 255, .6)
	pWhite70 = ui.RGBA(255, 255, 255, .7)
	// playerScrim is the eased scrim: several stops so the fade has no
	// visible band.
	playerScrim = []ui.GradientStop{
		{Color: ui.RGBA(0, 0, 0, 0xC2/255.0), At: 0}, {Color: ui.RGBA(0, 0, 0, 0x8A/255.0), At: .25},
		{Color: ui.RGBA(0, 0, 0, 0x3D/255.0), At: .5}, {Color: ui.RGBA(0, 0, 0, 0x0F/255.0), At: .75},
		{Color: ui.Transparent, At: 1},
	}
)

// PlayerViewOpts are what the page hands the view each frame.
type PlayerViewOpts struct {
	Fullscreen   bool
	OnFullscreen func()
	OnBack       func()
	// PanelOpen is nil where the panel sits below the video.
	PanelOpen     *bool
	OnTogglePanel func()
	// OnCanvas opens the lecture on the study canvas; nil inside it.
	OnCanvas func()
}

// PlayerView is the video surface's state.
type PlayerView struct {
	ctl        *LecturePlayer
	shownUntil time.Time
	px, py     float32
	menuOpen   bool
	menus      bool

	toast      string
	toastIcon  icons.Icon
	toastUntil time.Time

	// The "resumed from" prompt, scoped to the player and self-dismissing.
	resumeFrom  int
	resumeStart time.Time

	// A click waits for a double click before it plays or pauses.
	tapAt time.Time
	// A held press boosts the speed.
	pressAt   time.Time
	boostHeld bool

	// Touch drags seek (sideways) or set the volume (up and down).
	dragging            bool
	dragDecided, dragH  bool
	dragSeek, dragVol   float64
	dragStartMs         int
	dragLastX, dragLast float32

	pipRight, pipBottom bool
	pipDrag             bool
	pipX, pipY          float32
	pipGrabX, pipGrabY  float32

	seek      seekBar
	volHover  bool
	volBefore float64
}

// NewPlayerView is the view of ctl.
func NewPlayerView(ctl *LecturePlayer) *PlayerView {
	v := &PlayerView{ctl: ctl, resumeFrom: -1, pipRight: true, volBefore: 100, dragSeek: -1, dragVol: -1}
	v.bump()
	return v
}

const resumeFor = 8 * time.Second

func mobilePlatform() bool { p := core.Platform(); return p == "android" || p == "ios" }

// bump shows the controls and restarts the auto-hide timer.
func (v *PlayerView) bump() { v.shownUntil = time.Now().Add(3 * time.Second) }

// Toast shows text with icon in the middle for a moment.
func (v *PlayerView) Toast(text string, ic icons.Icon) {
	v.toast, v.toastIcon, v.toastUntil = text, ic, time.Now().Add(900*time.Millisecond)
}

// ShowResume offers to start over after resuming from ms.
func (v *PlayerView) ShowResume(ms int) { v.resumeFrom, v.resumeStart = ms, time.Now() }

func (v *PlayerView) dismissResume() { v.resumeFrom = -1 }

// SeekBy seeks by s seconds with a toast.
func (v *PlayerView) SeekBy(s int) {
	v.ctl.SeekBy(s)
	sign, ic := "", icons.FastRewindRounded
	if s > 0 {
		sign, ic = "+", icons.FastForwardRounded
	}
	v.Toast(l10n.PlayerSeekToast(sign, s), ic)
}

// VolumeBy changes the volume by d.
func (v *PlayerView) VolumeBy(d float64) {
	n := min(max(v.ctl.Volume()+d, 0), 100)
	v.ctl.SetVolume(n)
	ic := icons.VolumeUpRounded
	if n == 0 {
		ic = icons.VolumeOffRounded
	}
	v.Toast(l10n.PlayerVolumeToast(int(n+.5)), ic)
}

// CycleSpeed steps through the speeds.
func (v *PlayerView) CycleSpeed(dir int) {
	i := -1
	for k, s := range playerSpeeds {
		if s >= v.ctl.Rate()-0.01 {
			i = k
			break
		}
	}
	if i < 0 {
		i = 2
	}
	n := playerSpeeds[min(max(i+dir, 0), len(playerSpeeds)-1)]
	v.ctl.SetRate(n)
	v.Toast(l10n.PlayerSpeedToast(SpeedLabel(n)), icons.SpeedRounded)
}

func pill(c *ui.Context, ic icons.Icon, text string, big bool) ui.Element {
	px, py, is, fs := float32(14), float32(6), float32(18), float32(14)
	if big {
		px, py, is, fs = 18, 10, 22, 16
	}
	return ui.Row(c).AlignItems(ui.Center).Gap(8).Padding(py, px).Radius(design.CornerFull).Background(ui.RGBA(0, 0, 0, .66)).Children(func() {
		m3.Icon(c, ic, is, pWhite)
		m3.Text(c, text, design.TextStyle{Size: fs, Weight: 500, Color: pWhite}).FontFeatures("tnum").NoWrap()
	})
}

// Show builds the view filling its parent.
func (v *PlayerView) Show(c *ui.Context, o PlayerViewOpts) ui.Element {
	ctl := v.ctl
	now := time.Now()
	playing := ctl.Playing()
	showUI := now.Before(v.shownUntil) || !playing || v.menuOpen
	root := ui.Box(c).Fill().Background(ui.RGB(0, 0, 0)).Clip()
	if x, y, over := root.PointerPosition(); over && (x != v.px || y != v.py) {
		v.px, v.py = x, y
		v.bump()
		showUI = true
	}
	if showUI && playing && !v.menuOpen {
		c.After(time.Until(v.shownUntil) + 10*time.Millisecond)
	}
	if !showUI {
		root = root.Cursor(ui.CursorNone)
	}
	v.menuOpen = false
	chrome := root.AnimateTo("chrome", map[bool]float32{true: 1, false: 0}[showUI], design.T().Motion(map[bool]time.Duration{true: design.Short4, false: design.Medium2}[showUI], design.Standard))
	root.Children(func() {
		if ctl.Player != nil {
			ui.Video(c, ctl.Player).Fit(ui.Contain).Absolute().Top(0).Left(0).Right(0).Bottom(0)
		}
		v.surface(c, o)
		if ctl.Pip != nil && ctl.PipIdx >= 0 {
			v.pipWindow(c, root)
		}
		if core.UI.ShowSubtitles && len(ctl.Segments) > 0 {
			bottom := float32(28)
			if showUI {
				bottom = 108
			}
			scale := float32(core.UI.SubtitleScale)
			if o.Fullscreen {
				scale *= 1.25
			}
			ui.Row(c).Absolute().Left(24).Right(24).Bottom(bottom).Justify(ui.Center).PassThrough().Children(func() { v.subtitle(c, scale) })
		}
		if err := ctl.Err(); err != "" {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Children(func() { v.errorBox(c, err) })
		} else if ctl.Buffering() {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().PassThrough().Children(func() {
				widgets.Loader{Size: 56}.Show(c)
			})
		}
		ui.Row(c).Absolute().Top(72).Left(0).Right(0).Justify(ui.Center).PassThrough().Children(func() {
			if ctl.Boosting {
				pill(c, icons.FastForwardRounded, l10n.PlayerBoosting(SpeedLabel(core.UI.BoostSpeed)), false).
					Transition(design.Switcher(design.Short4, nil, nil, ui.Motion{Opacity: 0, Scale: .8}))
			}
		})
		v.centerPill(c, now)
		resume := false
		if v.resumeFrom >= 0 {
			if now.Sub(v.resumeStart) >= resumeFor {
				v.dismissResume()
			} else {
				resume = true
			}
		}
		// AnimatedPositioned around the AnimatedSwitcher of the prompt.
		bottom := float32(design.SpaceLG)
		if showUI {
			bottom = 112
		}
		ui.Box(c).Absolute().Left(design.SpaceLG).Bottom(bottom).
			Transition(ui.ElementTransition{Duration: design.T().Motion(design.Medium2, nil).Duration, Ease: design.Emphasized, Position: true}).Children(func() {
			if resume {
				ui.Box(c.Key(v.resumeFrom)).
					Transition(design.Switcher(design.Medium2, design.EmphasizedDecelerate, design.EmphasizedAccelerate, ui.Motion{Opacity: 0, XPercent: -15})).
					Children(func() { v.resumePrompt(c, now) })
			}
		})
		if chrome > 0.01 {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Opacity(chrome).Children(func() { v.topBar(c, o) })
			ui.Box(c).Absolute().Bottom(0).Left(0).Right(0).Opacity(chrome).Children(func() { v.bottomBar(c, o) })
		}
	})
	return root
}

// surface takes the picture's gestures, below the controls.
func (v *PlayerView) surface(c *ui.Context, o PlayerViewOpts) {
	ctl := v.ctl
	mobile := mobilePlatform()
	s := ui.ButtonBase(c).FocusRing(false).Absolute().Top(0).Left(0).Right(0).Bottom(0)
	now := time.Now()
	// A held press boosts; its release is no click.
	if s.Pressed() {
		if v.pressAt.IsZero() {
			v.pressAt = now
		}
		if !v.boostHeld && !v.dragging && now.Sub(v.pressAt) >= 500*time.Millisecond {
			v.boostHeld = true
			ctl.StartBoost()
		} else if !v.boostHeld {
			c.After(500*time.Millisecond - now.Sub(v.pressAt))
		}
	} else if !v.pressAt.IsZero() {
		v.pressAt = time.Time{}
		if v.boostHeld {
			ctl.EndBoost()
		}
	}
	held := v.boostHeld
	if !s.Pressed() {
		v.boostHeld = false
	}
	if mobile {
		v.touchDrags(c, s)
	}
	if s.DoubleClicked() {
		v.tapAt = time.Time{}
		if mobile {
			x, _, _ := s.PointerPosition()
			w := s.Bounds().W
			switch f := x / max(w, 1); {
			case f < .35:
				v.SeekBy(-core.UI.SeekStepSec)
			case f > .65:
				v.SeekBy(core.UI.SeekStepSec)
			default:
				ctl.PlayOrPause()
			}
		} else if o.OnFullscreen != nil {
			o.OnFullscreen()
		}
	} else if s.Clicked() && !held {
		// Wait out a double click.
		v.tapAt = now
		c.After(300 * time.Millisecond)
	}
	if !v.tapAt.IsZero() && now.Sub(v.tapAt) >= 280*time.Millisecond {
		v.tapAt = time.Time{}
		if mobile {
			if now.Before(v.shownUntil) {
				v.shownUntil = time.Time{}
			} else {
				v.bump()
			}
		} else {
			ctl.PlayOrPause()
			v.bump()
		}
	}
}

// touchDrags seeks with a sideways drag and sets the volume with an
// upright one.
func (v *PlayerView) touchDrags(c *ui.Context, s ui.Element) {
	ctl := v.ctl
	s.HandleInput(func(ev ui.InputEvent) bool {
		w, h := s.Bounds().W, s.Bounds().H
		switch ev.Kind {
		case ui.InputPointerDown:
			v.dragging, v.dragDecided = true, false
			v.dragLastX, v.dragLast = ev.X, ev.Y
			v.dragStartMs = ctl.PositionMs()
		case ui.InputPointerMove:
			if !v.dragging || ev.Button < 0 {
				return false
			}
			dx, dy := ev.X-v.dragLastX, ev.Y-v.dragLast
			if !v.dragDecided {
				if dx*dx+dy*dy < 64 {
					return false
				}
				v.dragDecided, v.dragH = true, abs32(dx) >= abs32(dy)
				if v.dragH {
					v.dragSeek = float64(v.dragStartMs)
				} else {
					v.dragVol = ctl.Volume()
				}
			}
			v.dragLastX, v.dragLast = ev.X, ev.Y
			dur := float64(ctl.DurationMs())
			if v.dragH {
				span := min(max(dur, 60000), 600000)
				v.dragSeek = min(max(v.dragSeek+float64(dx/max(w, 1))*span, 0), dur)
			} else {
				v.dragVol = min(max(v.dragVol-float64(dy/max(h, 1))*150, 0), 100)
				ctl.SetVolume(v.dragVol)
			}
			return v.dragDecided
		case ui.InputPointerUp:
			if v.dragSeek >= 0 {
				ctl.SeekTo(int(v.dragSeek + .5))
			}
			decided := v.dragDecided
			v.dragging, v.dragDecided, v.dragSeek, v.dragVol = false, false, -1, -1
			return decided
		}
		return false
	})
}

func abs32(f float32) float32 {
	if f < 0 {
		return -f
	}
	return f
}

// centerPill shows drag feedback or a toast.
func (v *PlayerView) centerPill(c *ui.Context, now time.Time) {
	ctl := v.ctl
	var build func() ui.Element
	key := ""
	switch {
	case v.dragSeek >= 0:
		key = "ds"
		build = func() ui.Element {
			return pill(c, icons.SwapHorizRounded, core.FmtClock(int(v.dragSeek))+" / "+core.FmtClock(ctl.DurationMs()), true)
		}
	case v.dragVol >= 0:
		key = "dv"
		build = func() ui.Element { return pill(c, icons.VolumeUpRounded, strconv.Itoa(int(v.dragVol+.5))+"%", true) }
	case v.toast != "" && now.Before(v.toastUntil):
		c.After(time.Until(v.toastUntil))
		key = "t" + v.toast
		build = func() ui.Element { return pill(c, v.toastIcon, v.toast, true) }
	}
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().PassThrough().Children(func() {
		if build != nil {
			ui.Box(c.Key(key)).Transition(design.Switcher(design.Short3, nil, nil, ui.Motion{Opacity: 0})).Children(func() { build() })
		}
	})
}

func (v *PlayerView) subtitle(c *ui.Context, scale float32) {
	ctl := v.ctl
	pos := ctl.PositionMs()
	i := ctl.SegmentAt(pos)
	if i < 0 || ctl.Segments[i].EndMs()+600 < pos {
		return
	}
	seg := ctl.Segments[i]
	ui.Box(c).Key(seg.Idx()).Padding(5, 12).Radius(design.CornerSM).Background(ui.RGBA(0, 0, 0, .62)).
		Transition(design.Switcher(design.Short3, nil, nil, ui.Motion{Opacity: 0})).Children(func() {
		m3.Text(c, seg.Text(), design.TextStyle{Size: 17 * scale, Height: 1.35, Weight: 500, Color: pWhite}).TextAlign(ui.Center)
	})
}

func (v *PlayerView) errorBox(c *ui.Context, text string) {
	tt := design.TT()
	if pk := missingGstPackages(); len(pk) > 0 {
		text = l10n.PlayerMissingGstPlugins(strings.Join(pk, " "))
	}
	ui.Column(c).MaxWidth(420).AlignItems(ui.Center).Padding(design.SpaceXL).Radius(design.CornerXL).Background(ui.RGBA(0, 0, 0, .7)).Children(func() {
		m3.Icon(c, icons.ErrorOutlineRounded, 36, pWhite)
		m3.VGap(c, design.SpaceMD)
		m3.Text(c, l10n.PlayerError, tt.TitleMedium.C(pWhite))
		m3.VGap(c, design.SpaceXS)
		m3.Clamp(m3.Text(c, text, tt.BodyMedium.C(pWhite70)).TextAlign(ui.Center), 4)
		m3.VGap(c, design.SpaceLG)
		if (m3.Button{Kind: m3.Tonal, Icon: icons.RefreshRounded, Label: l10n.PlayerReload}).Show(c).Clicked() {
			v.ctl.SwitchSource(v.ctl.SourceIdx)
		}
	})
}

// resumePrompt is "Resumed from mm:ss · 从头播放", with a hairline that
// runs out as the prompt times out.
func (v *PlayerView) resumePrompt(c *ui.Context, now time.Time) {
	cs, tt := design.S(), design.TT()
	left := 1 - float32(now.Sub(v.resumeStart))/float32(resumeFor)
	c.After(50 * time.Millisecond)
	ui.Column(c).Radius(design.CornerMD).Clip().Background(ui.RGBA(0x20, 0x21, 0x24, 0xE6/255.0)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Padding(design.SpaceXS, design.SpaceXS, design.SpaceXS, design.SpaceLG).Children(func() {
			m3.Icon(c, icons.HistoryRounded, 18, pWhite70)
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, l10n.PlayerResumedFrom(core.FmtClock(v.resumeFrom)), tt.BodyMedium.C(pWhite)).MaxLines(1).Shrink(1)
			m3.HGap(c, design.SpaceSM)
			if (m3.Button{Kind: m3.TextKind, Label: l10n.PlayerStartOver, FG: cs.InversePrimary, Height: 32}).Show(c).Clicked() {
				v.ctl.SeekTo(0)
				v.dismissResume()
			}
			if (m3.IconButton{Icon: icons.CloseRounded, Tooltip: l10n.CommonClose, Size: 18, Box: 32, Color: pWhite70}).Show(c).Clicked() {
				v.dismissResume()
			}
		})
		ui.Box(c).FillWidth().Height(2).Children(func() {
			ui.Box(c).WidthPercent(left * 100).Height(2).Background(design.WithOpacity(cs.InversePrimary, .7))
		})
	})
}

func (v *PlayerView) topBar(c *ui.Context, o PlayerViewOpts) {
	ctl, tt := v.ctl, design.TT()
	bar := ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(design.SpaceSM, design.SpaceSM, design.SpaceXL, design.SpaceSM)
	bar.Draw(func(p *ui.Painter, r ui.Rect) {
		p.FillGradient(r, ui.LinearGradient{Angle: 180, Stops: playerScrim}, 0)
	})
	bar.Children(func() {
		if (m3.IconButton{Icon: icons.ArrowBackRounded, Tooltip: l10n.CommonBack, Color: pWhite}).Show(c).Clicked() && o.OnBack != nil {
			o.OnBack()
		}
		m3.HGap(c, design.SpaceXS)
		ui.Column(c).Grow(1).Basis(0).Children(func() {
			title := ""
			if ctl.Lecture != nil {
				title = ctl.Lecture.ShortTitle()
			}
			m3.Text(c, title, tt.TitleMedium.C(pWhite)).MaxLines(1)
			if ctl.Course != nil {
				m3.Text(c, ctl.Course.Title(), tt.BodySmall.C(pWhite70)).MaxLines(1)
			}
		})
		if len(ctl.Sources) > 1 {
			v.sourcePicker(c)
		}
		if o.OnCanvas != nil && !o.Fullscreen {
			if (m3.Button{Kind: m3.TextKind, Icon: icons.DrawOutlined, Label: l10n.StudyOpenCanvas, FG: pWhite, BG: ui.RGBA(255, 255, 255, .14), Height: 32}).Show(c).
				Margin(0, design.SpaceXS).Key("player-open-canvas").Clicked() {
				o.OnCanvas()
			}
		}
		if o.OnTogglePanel != nil {
			open := o.PanelOpen != nil && *o.PanelOpen
			tip, ic := l10n.PlayerExpandPanel, icons.ViewSidebarOutlined
			if open {
				tip, ic = l10n.PlayerCollapsePanel, icons.ViewSidebarRounded
			}
			if (m3.IconButton{Icon: ic, Tooltip: tip, Color: pWhite}).Show(c).Clicked() {
				o.OnTogglePanel()
			}
		}
	})
}

func (v *PlayerView) sourcePicker(c *ui.Context) {
	ctl := v.ctl
	m3.Menu(c, "source", m3.Below, func(t ui.Element, open bool) {
		if open {
			v.menuOpen = true
		}
		label := ""
		if s := ctl.Source(); s != nil {
			label = s.Label
		}
		t.Row().AlignItems(ui.Center).Gap(8).Height(40).Padding(0, 12, 0, 16).Radius(design.CornerFull).Cursor(ui.CursorPointer)
		m3.Ink(t, pWhite, design.CornerFull)
		t.Children(func() {
			m3.Icon(c, icons.VideocamOutlined, 18, pWhite)
			m3.Text(c, label, design.TT().LabelLarge.C(pWhite)).NoWrap()
		})
	}, func() []m3.MenuItem {
		var items []m3.MenuItem
		for i, s := range ctl.Sources {
			ic := icons.RadioButtonOffRounded
			if i == ctl.SourceIdx {
				ic = icons.RadioButtonCheckedRounded
			}
			label := s.Label
			var trailing icons.Icon
			if s.Local {
				label += l10n.PlayerLocalSuffix
				trailing = icons.DownloadDoneRounded
			}
			items = append(items, m3.MenuItem{Icon: ic, Label: label, TrailingIcon: trailing, OnTap: func() { ctl.SwitchSource(i) }})
		}
		checked := func(on bool) *bool { return &on }
		sub := []m3.MenuItem{{Checked: checked(ctl.PipIdx < 0), Label: l10n.CommonClose, OnTap: func() { ctl.SetPip(-1) }}}
		for i, s := range ctl.Sources {
			if i != ctl.SourceIdx && !s.Local {
				sub = append(sub, m3.MenuItem{Checked: checked(ctl.PipIdx == i), Label: s.Label, OnTap: func() { ctl.SetPip(i) }})
			}
		}
		return append(items, m3.MenuItem{Divider: true}, m3.MenuItem{Icon: icons.PictureInPictureAltRounded, Label: l10n.PlayerPip, Sub: sub})
	})
}

func (v *PlayerView) bottomBar(c *ui.Context, o PlayerViewOpts) {
	ctl, tt := v.ctl, design.TT()
	compact := WindowWidth < 600
	step := core.UI.SeekStepSec
	bar := ui.Column(c).FillWidth().Padding(design.SpaceXXL, design.SpaceLG, design.SpaceSM, design.SpaceLG)
	bar.Draw(func(p *ui.Painter, r ui.Rect) { p.FillGradient(r, ui.LinearGradient{Angle: 0, Stops: playerScrim}, 0) })
	bar.Children(func() {
		if len(ctl.Chapters) > 0 {
			v.chapterChip(c, o)
		}
		v.seek.Show(c, ctl, v.bump)
		m3.VGap(c, design.SpaceXS)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			v.playButton(c)
			m3.HGap(c, design.SpaceXS)
			if !compact {
				if (m3.IconButton{Icon: icons.Replay10Rounded, Tooltip: l10n.PlayerSeekBack(step), Color: pWhite}).Show(c).Clicked() {
					v.SeekBy(-step)
				}
				if (m3.IconButton{Icon: icons.Forward10Rounded, Tooltip: l10n.PlayerSeekForward(step), Color: pWhite}).Show(c).Clicked() {
					v.SeekBy(step)
				}
				v.volumeControl(c)
			}
			m3.HGap(c, design.SpaceSM)
			ui.Row(c).Shrink(0).Children(func() {
				st := tt.LabelLarge.C(pWhite)
				m3.Text(c, core.FmtClock(ctl.PositionMs()), st).FontFeatures("tnum").NoWrap()
				m3.Text(c, "  /  "+core.FmtClock(ctl.DurationMs()), st.C(pWhite60)).FontFeatures("tnum").NoWrap()
			})
			ui.Box(c).Grow(1)
			v.speedMenu(c)
			if len(ctl.Segments) > 0 {
				ic := icons.ClosedCaptionOffOutlined
				if core.UI.ShowSubtitles {
					ic = icons.ClosedCaptionRounded
				}
				if (m3.IconButton{Icon: ic, Tooltip: l10n.PlayerSubtitlesTooltip, Color: pWhite}).Show(c).Clicked() {
					core.UI.Update(func(p *design.Prefs) { p.ShowSubtitles = !p.ShowSubtitles })
				}
			}
			if (m3.IconButton{Icon: icons.BookmarkAddOutlined, Tooltip: l10n.PlayerBookmarkTooltip, Color: pWhite}).Show(c).Clicked() {
				v.addBookmark()
			}
			tip, ic := l10n.PlayerFullscreen, icons.FullscreenRounded
			if o.Fullscreen {
				tip, ic = l10n.PlayerExitFullscreen, icons.FullscreenExitRounded
			}
			if (m3.IconButton{Icon: ic, Tooltip: tip, Color: pWhite}).Show(c).Clicked() && o.OnFullscreen != nil {
				o.OnFullscreen()
			}
		})
	})
}

func (v *PlayerView) addBookmark() {
	v.ctl.AddBookmark("", func(b *core.Bookmark) {
		if b != nil {
			v.Toast(l10n.PlayerBookmarkAdded(core.FmtClock(b.TMs())), icons.BookmarkAddedRounded)
		}
	})
}

// chapterChip is the chapter under the playhead; it opens the outline.
func (v *PlayerView) chapterChip(c *ui.Context, o PlayerViewOpts) {
	ctl, tt := v.ctl, design.TT()
	i := ctl.ChapterAt(ctl.PositionMs())
	if i < 0 {
		return
	}
	ch := ctl.Chapters[i]
	b := m3.Tappable(c, design.CornerSM).Row().AlignItems(ui.Center).Height(30).Padding(0, 8, 0, 4).MaxWidth(420).Margin(0, 0, 2, 0).AlignSelf(ui.Start)
	b.Children(func() {
		ui.Row(c).Key(i).AlignItems(ui.Center).Shrink(1).
			Transition(design.Switcher(design.Medium2, design.EmphasizedDecelerate, nil, ui.Motion{Opacity: 0, YPercent: 40})).Children(func() {
			m3.Text(c, strconv.Itoa(i+1)+"/"+strconv.Itoa(len(ctl.Chapters)), tt.LabelMedium.C(pWhite60)).FontFeatures("tnum").NoWrap()
			m3.HGap(c, design.SpaceSM)
			m3.Text(c, ch.Title(), tt.LabelLarge.C(pWhite)).MaxLines(1).Shrink(1)
			m3.Icon(c, icons.ChevronRightRounded, 18, pWhite70)
		})
	})
	if b.Clicked() {
		// The outline lives in the side panel: bring it into view first.
		if o.Fullscreen && o.OnFullscreen != nil {
			o.OnFullscreen()
		}
		if o.PanelOpen != nil && !*o.PanelOpen && o.OnTogglePanel != nil {
			o.OnTogglePanel()
		}
		ctl.PanelTab = "outline"
	}
}

// playButton morphs: a circle while paused, a rounded square while
// playing (M3 Expressive shape morph).
func (v *PlayerView) playButton(c *ui.Context) {
	playing := v.ctl.Playing()
	tip, ic := l10n.PlayerPlayTooltip, icons.PlayArrowRounded
	target := float32(24)
	if playing {
		tip, ic, target = l10n.PlayerPauseTooltip, icons.PauseRounded, 14
	}
	b := ui.ButtonBase(c).FocusRing(false).Size(48, 48).Shrink(0).Center().Background(pWhite).Cursor(ui.CursorPointer)
	r := b.AnimateTo("radius", target, design.T().Motion(design.Medium2, design.Emphasized))
	b = b.Radius(r)
	m3.Ink(b, ui.RGB(0, 0, 0), r)
	m3.Tip(c, b.Label(tip), tip)
	b.Children(func() {
		ui.Box(c).Key(playing).Transition(design.Switcher(design.Short4, nil, nil, ui.Motion{Opacity: 0, Scale: .6})).Children(func() {
			m3.Icon(c, ic, 28, ui.RGBA(0, 0, 0, .87))
		})
	})
	if b.Clicked() {
		v.ctl.PlayOrPause()
	}
}

func (v *PlayerView) volumeControl(c *ui.Context) {
	ctl := v.ctl
	vol := ctl.Volume()
	row := ui.Row(c).AlignItems(ui.Center).Shrink(0)
	_, _, over := row.PointerPosition()
	v.volHover = over
	row.Children(func() {
		ic := icons.VolumeUpRounded
		if vol == 0 {
			ic = icons.VolumeOffRounded
		} else if vol < 50 {
			ic = icons.VolumeDownRounded
		}
		if (m3.IconButton{Icon: ic, Tooltip: l10n.PlayerMute, Color: pWhite}).Show(c).Clicked() {
			if vol > 0 {
				v.volBefore = vol
				ctl.SetVolume(0)
			} else {
				ctl.SetVolume(v.volBefore)
			}
		}
		box := ui.Box(c).Height(40).Clip().AlignItems(ui.Center).Justify(ui.Center)
		w := box.AnimateTo("w", map[bool]float32{true: 96, false: 0}[v.volHover], design.T().Motion(design.Medium1, design.Emphasized))
		box.Width(w).Children(func() {
			if w > 1 {
				val := vol
				res := (m3.Slider{Max: 100, TrackHeight: 4}).Show(c, &val)
				res.Width(96).Padding(0, 8).Shrink(0)
				if val != vol {
					ctl.SetVolume(val)
				}
			}
		})
	})
}

func (v *PlayerView) speedMenu(c *ui.Context) {
	ctl := v.ctl
	m3.Menu(c, "speed", m3.Over, func(t ui.Element, open bool) {
		if open {
			v.menuOpen = true
		}
		bg := ui.RGBA(255, 255, 255, .24)
		if abs64(ctl.Rate()-1) < 0.01 {
			bg = ui.RGBA(255, 255, 255, .12)
		}
		t.Height(32).MinWidth(56).Padding(0, 12).Margin(8, 0).Center().Radius(design.CornerFull).Background(bg).Cursor(ui.CursorPointer)
		t.Label(l10n.PlayerSpeedTooltip)
		if !open {
			m3.Tip(c, t, l10n.PlayerSpeedTooltip)
		}
		m3.Ink(t, pWhite, design.CornerFull)
		t.Children(func() {
			m3.Text(c, SpeedLabel(ctl.Rate()), design.TT().LabelLarge.C(pWhite).W(600)).FontFeatures("tnum").NoWrap()
		})
	}, func() []m3.MenuItem {
		var items []m3.MenuItem
		for _, s := range playerSpeeds {
			on := abs64(ctl.Rate()-s) < 0.01
			items = append(items, m3.MenuItem{Checked: &on, Label: SpeedLabel(s), OnTap: func() { ctl.SetRate(s) }})
		}
		return items
	})
}

func abs64(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// pipWindow is the draggable floating second camera that snaps to the
// nearest corner.
func (v *PlayerView) pipWindow(c *ui.Context, root ui.Element) {
	ctl := v.ctl
	b := root.Bounds()
	W, H := b.W, b.H
	w := min(max(W*0.26, 160), 360)
	h := w * 9 / 16
	const m = 16
	bx, by := float32(m), float32(72)
	if v.pipRight {
		bx = W - w - m
	}
	if v.pipBottom {
		by = H - h - 112
	}
	x, y := bx, by
	if v.pipDrag {
		x, y = v.pipX, v.pipY
	}
	win := ui.Box(c).Absolute().Left(x).Top(y).Size(w, h).Radius(design.CornerLG).Clip().Background(ui.RGB(0, 0, 0))
	if !v.pipDrag {
		win = win.Transition(ui.ElementTransition{Duration: design.Medium4, Ease: design.Emphasized, Position: true})
	}
	m3.Elevation(win, 6)
	win.HandleInput(func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			v.pipDrag, v.pipX, v.pipY, v.pipGrabX, v.pipGrabY = true, x, y, ev.X, ev.Y
			return true
		case ui.InputPointerMove:
			if v.pipDrag && ev.Button >= 0 {
				v.pipX += ev.X - v.pipGrabX
				v.pipY += ev.Y - v.pipGrabY
				return true
			}
		case ui.InputPointerUp:
			if v.pipDrag {
				v.pipRight = v.pipX+w/2 >= W/2
				v.pipBottom = v.pipY+h/2 >= H/2
				v.pipDrag = false
				return true
			}
		}
		return false
	})
	win.Children(func() {
		ui.Video(c, ctl.Pip).Fit(ui.Contain).Absolute().Top(0).Left(0).Right(0).Bottom(0)
		label := ctl.Sources[ctl.PipIdx].Label
		m3.Text(c, label, design.TextStyle{Size: 12, Color: pWhite}).Absolute().Left(8).Top(6).TextShadow(0, 0, 4, ui.RGBA(0, 0, 0, .8))
		if (m3.IconButton{Icon: icons.CloseRounded, Size: 18, Color: pWhite}).Show(c).Absolute().Right(0).Top(0).Clicked() {
			ctl.SetPip(-1)
		}
	})
}

// ---------------------------------------------------------------- seek bar

// seekBar is the progress bar split into outline chapters, with slide
// ticks, bookmark and schedule marks, and a hover preview of the slide
// and chapter at that moment.
type seekBar struct {
	hover    float32 // -1 when away
	dragFrac float32 // -1 when not dragging
	width    float32
}

func (s *seekBar) frac(x float32) float32 { return min(max(x/max(s.width, 1), 0), 1) }

// Show builds the bar; onInteract keeps the controls up.
func (s *seekBar) Show(c *ui.Context, ctl *LecturePlayer, onInteract func()) {
	cs := design.S()
	dur := ctl.DurationMs()
	bar := ui.Box(c).FillWidth().Height(28).Cursor(ui.CursorPointer)
	if w := bar.Bounds().W; w > 0 {
		s.width = w
	}
	s.hover = -1
	if x, _, over := bar.PointerPosition(); over {
		s.hover = s.frac(x)
	}
	if s.dragFrac == 0 && s.width == 0 {
		s.dragFrac = -1
	}
	bar.HandleInput(func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			onInteract()
			s.dragFrac = s.frac(ev.X)
			return true
		case ui.InputPointerMove:
			if s.dragFrac >= 0 && ev.Button >= 0 {
				onInteract()
				s.dragFrac = s.frac(ev.X)
				return true
			}
		case ui.InputPointerUp:
			if s.dragFrac >= 0 {
				if dur > 0 && !ev.Canceled {
					ctl.SeekTo(int(float64(s.frac(ev.X)) * float64(dur)))
				}
				s.dragFrac = -1
				return true
			}
		}
		return false
	})
	active := s.hover >= 0 || s.dragFrac >= 0
	played := float32(0)
	if s.dragFrac >= 0 {
		played = s.dragFrac
	} else if dur > 0 {
		played = float32(ctl.PositionMs()) / float32(dur)
	}
	var slides, bookmarks, chapters []float32
	type mark struct {
		at float32
		c  ui.Color
	}
	var marks []mark
	if dur > 0 {
		fd := float32(dur)
		for _, sl := range ctl.Slides {
			if sl.IsKey() {
				slides = append(slides, float32(sl.TSec())*1000/fd)
			}
		}
		for _, b := range ctl.Bookmarks {
			bookmarks = append(bookmarks, float32(b.TMs())/fd)
		}
		for i, ch := range ctl.Chapters {
			if i > 0 {
				chapters = append(chapters, float32(ch.StartMs())/fd)
			}
		}
		for _, it := range Schedule.ForLecture(ctl.LectureID()) {
			if it.TMs() >= 0 {
				marks = append(marks, mark{float32(it.TMs()) / fd, KindStyleOf(it.Kind()).Accent})
			}
		}
	}
	hover := s.hover
	bar.Draw(func(p *ui.Painter, r ui.Rect) {
		h := float32(4)
		if active {
			h = 6
		}
		y := r.Y + r.H/2
		w := r.W
		edges := []float32{0}
		for _, e := range chapters {
			if e > 0 && e < 1 {
				edges = append(edges, e)
			}
		}
		edges = append(edges, 1)
		px := r.X + w*min(max(played, 0), 1)
		const gap = 3
		for i := 0; i < len(edges)-1; i++ {
			x0 := r.X + w*edges[i]
			if i > 0 {
				x0 += gap / 2
			}
			x1 := r.X + w*edges[i+1]
			if i < len(edges)-2 {
				x1 -= gap / 2
			}
			if x1 <= x0 {
				continue
			}
			// The chapter under the pointer grows, so its extent reads at
			// a glance.
			sh := h
			if hover >= 0 && len(edges) > 2 && hover >= edges[i] && hover < edges[i+1] {
				sh = h + 3
			}
			seg := ui.Rect{X: x0, Y: y - sh/2, W: x1 - x0, H: sh}
			p.Clip(seg, sh/2, func() {
				p.Fill(seg, ui.RGBA(255, 255, 255, .22), 0)
				if hover >= 0 {
					p.Fill(ui.Rect{X: x0, Y: seg.Y, W: r.X + w*hover - x0, H: sh}, ui.RGBA(255, 255, 255, .18), 0)
				}
				p.Fill(ui.Rect{X: x0, Y: seg.Y, W: px - x0, H: sh}, cs.Primary, 0)
			})
		}
		for _, sl := range slides {
			p.Fill(ui.Rect{X: r.X + w*sl - .75, Y: y - h/2, W: 1.5, H: h}, ui.RGBA(0, 0, 0, .45), 0)
		}
		for _, b := range bookmarks {
			p.Fill(ui.Rect{X: r.X + w*b - 3, Y: y - h/2 - 8, W: 6, H: 6}, cs.Tertiary, 3)
		}
		// Schedule mentions: small rounded squares in the kind's colour.
		for _, m := range marks {
			sq := ui.Rect{X: r.X + w*m.at - 3.5, Y: y - h/2 - 9.5, W: 7, H: 7}
			p.Fill(sq, ui.RGBA(0, 0, 0, .5), 2)
			p.Fill(ui.Rect{X: sq.X + 1, Y: sq.Y + 1, W: 5, H: 5}, m.c, 1)
		}
		// Thumb: an M3 Expressive vertical handle.
		tw, th := float32(4), float32(16)
		if active {
			tw, th = 6, 22
		}
		p.Fill(ui.Rect{X: px - tw/2, Y: y - th/2, W: tw, H: th}, cs.Primary, tw/2)
	})
	if (s.dragFrac >= 0 || s.hover >= 0) && dur > 0 {
		bar.Children(func() { s.preview(c, ctl, dur) })
	}
}

func (s *seekBar) preview(c *ui.Context, ctl *LecturePlayer, dur int) {
	tt := design.TT()
	f := s.hover
	if s.dragFrac >= 0 {
		f = s.dragFrac
	}
	ms := int(f * float32(dur))
	si := ctl.SlideAt(ms)
	ci := ctl.ChapterAt(ms)
	chapter := ""
	if ci >= 0 {
		chapter = ctl.Chapters[ci].Title()
	}
	const w = 176
	left := min(max(f*s.width-w/2, 0), max(s.width-w, 0))
	ui.Column(c).Absolute().Left(left).Bottom(30).Width(w).AlignItems(ui.Center).PassThrough().Children(func() {
		if si >= 0 {
			ui.Box(c).FillWidth().AspectRatio(16.0 / 9).Radius(design.CornerMD).Clip().Background(ui.RGBA(0, 0, 0, .54)).Children(func() {
				widgets.NetImage(c, ctl.SlideURL(ctl.Slides[si]), nil)
			})
		}
		m3.VGap(c, design.SpaceXS)
		radius := float32(design.CornerFull)
		if chapter != "" {
			radius = design.CornerMD
		}
		ui.Column(c).AlignItems(ui.Center).Padding(4, 10).Radius(radius).Background(ui.RGBA(0, 0, 0, .78)).Children(func() {
			if chapter != "" {
				m3.Text(c, chapter, tt.LabelMedium.C(pWhite).H(1.3)).MaxLines(2).Ellipsis("…").TextAlign(ui.Center)
				m3.Text(c, core.FmtClock(ms), tt.LabelSmall.C(pWhite70)).FontFeatures("tnum")
			} else {
				m3.Text(c, core.FmtClock(ms), tt.LabelMedium.C(pWhite)).FontFeatures("tnum")
			}
		})
	})
}
