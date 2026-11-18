package app

import (
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

// ServerSettings is the daemon's settings (/api/settings), shared by the
// settings panes and the pages they open so both stay in step.
type ServerSettings struct {
	Values core.J
}

// Load reads the settings.
func (s *ServerSettings) Load() {
	core.Async(func() (any, error) { return api().Get("/api/settings") }, func(v any, err error) {
		if err == nil {
			s.Values = objOrEmpty(v)
		}
	})
}

// Put shows value at once, then takes what the daemon saved; a failure
// shows as a snackbar.
func (s *ServerSettings) Put(key string, value any) {
	s.Set(key, value)
	core.Async(func() (any, error) { return api().Put("/api/settings", core.J{key: value}) }, func(v any, err error) {
		if err != nil {
			widgets.ShowSnack(l10n.CommonSaveFailed(apiMessage(err)), "", nil)
			return
		}
		if m := core.Obj(v); m != nil {
			s.Values = m
		}
	})
}

// Set changes a value locally, as while a slider is dragged.
func (s *ServerSettings) Set(key string, value any) {
	if s.Values == nil {
		s.Values = core.J{}
	}
	s.Values[key] = value
}

// Num is a numeric setting, or def.
func (s *ServerSettings) Num(key string, def float64) float64 {
	if v, ok := s.Values[key].(float64); ok {
		return v
	}
	if v, ok := s.Values[key].(int); ok {
		return float64(v)
	}
	return def
}

// SettingsGroup is a titled group of settings on a tonal container.
// leading replaces the icon, as with a brand mark.
func SettingsGroup(c *ui.Context, title string, ic icons.Icon, leading func(), children func()) {
	settingsGroupPad(c, title, ic, leading, [4]float32{design.SpaceMD, design.SpaceXL, design.SpaceLG, design.SpaceXL}, children)
}

func settingsGroupPad(c *ui.Context, title string, ic icons.Icon, leading func(), pad [4]float32, children func()) {
	cs := design.S()
	ui.Column(c.Key("group:"+title)).FillWidth().Padding(design.SpaceXL, 0, 0, 0).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Padding(0, 0, design.SpaceSM, design.SpaceXS).Children(func() {
			switch {
			case leading != nil:
				leading()
				m3.HGap(c, design.SpaceSM)
			case ic != 0:
				m3.Icon(c, ic, 18, cs.Primary)
				m3.HGap(c, design.SpaceSM)
			}
			m3.Text(c, title, design.TT().TitleSmall.C(cs.Primary))
		})
		ui.Column(c).FillWidth().Background(cs.SurfaceContainer).Radius(design.CornerXL).Clip().
			Padding(pad[0], pad[1], pad[2], pad[3]).Children(children)
	})
}

// brandImage is a mark from app/assets/brands (see its NOTICE), or a
// provider's from app/assets/providers when name has that folder.
func brandImage(name string) ui.ImageSource {
	if !strings.Contains(name, "/") {
		name = "brands/" + name
	}
	return assets.Image(name)
}

// brandIcon draws a mark size square; a mono SVG takes the theme's text
// colour, a bitmap gets rounded corners (radius).
func brandIcon(c *ui.Context, name string, size, radius float32, mono bool) {
	img := brandImage(name)
	if img == nil {
		ui.Box(c).Size(size, size)
		return
	}
	if s, ok := img.(*ui.SVG); ok && mono {
		ui.Icon(c, s).Size(size, size).TextColor(design.S().OnSurface)
		return
	}
	e := ui.Image(c, img).Size(size, size).Fit(ui.Cover)
	if radius > 0 {
		e.Radius(radius).Clip()
	}
}

// BrandMark is a third-party mark on a neutral tile.
func BrandMark(c *ui.Context, name string, size float32, mono bool) {
	if size == 0 {
		size = 40
	}
	ui.Box(c).Size(size, size).Shrink(0).Center().Background(design.S().SurfaceContainerHighest).Radius(size * .3).Children(func() {
		r := float32(design.CornerXS)
		if strings.HasSuffix(name, ".svg") {
			r = 0
		}
		brandIcon(c, name, size*.6, r, mono)
	})
}

// IntegrationHeader is the top of an integration's page: its mark, name,
// a status line and what it does; on tints the status.
func IntegrationHeader(c *ui.Context, mark func(), title, status, body string, on bool) {
	cs := design.S()
	tt := design.TT()
	ui.Column(c).FillWidth().Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			mark()
			m3.HGap(c, design.SpaceLG)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				m3.Text(c, title, tt.HeadlineSmall)
				m3.VGap(c, design.SpaceXXS)
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					dot, col := cs.Outline, cs.OnSurfaceVariant
					if on {
						dot, col = cs.Primary, cs.Primary
					}
					ui.Box(c).Size(8, 8).Radius(4).Background(dot).Shrink(0)
					m3.HGap(c, design.SpaceSM)
					m3.Clamp(m3.Text(c, status, tt.BodyMedium.C(col)), 1).Shrink(1)
				})
			})
		})
		m3.VGap(c, design.SpaceLG)
		m3.Text(c, body, tt.BodyMedium.C(cs.OnSurfaceVariant).H(1.6))
	})
}

// SettingsSteps is numbered how-to steps from start.
func SettingsSteps(c *ui.Context, steps []string, start int) {
	cs := design.S()
	tt := design.TT()
	if start == 0 {
		start = 1
	}
	ui.Column(c).FillWidth().Children(func() {
		for i, s := range steps {
			ui.Row(c.Key(i)).FillWidth().AlignItems(ui.Start).Padding(design.SpaceXS, 0).Children(func() {
				ui.Box(c).Size(22, 22).Radius(11).Center().Background(cs.SecondaryContainer).Shrink(0).Children(func() {
					m3.Text(c, strconv.Itoa(start+i), tt.LabelSmall.C(cs.OnSecondaryContainer))
				})
				m3.HGap(c, design.SpaceMD)
				m3.Text(c, s, tt.BodyMedium).Margin(1, 0, 0, 0).Grow(1).Basis(0)
			})
		}
	})
}

// SettingsSubpage is a full page opened from settings, with a readable
// column.
func SettingsSubpage(c *ui.Context, title string, scroll *ui.ScrollState, children func()) {
	pad := float32(design.SpaceXL)
	if Compact() {
		pad = design.SpaceLG
	}
	m3.Scaffold{AppBar: &m3.AppBar{Title: title, Scrolled: scroll != nil && scroll.Y > 0}, Body: func() {
		sc := ui.Scroll(c).Fill()
		if scroll != nil {
			sc = sc.TrackScroll(scroll)
		}
		sc.Children(func() {
			ui.Column(c).FillWidth().Padding(design.SpaceSM, pad, design.SpaceHuge, pad).Children(func() {
				widgets.ContentWidth(c, 760, children)
			})
		})
	}}.Show(c)
}

// sliderTile is a labelled slider with its value; onEnd runs when a drag
// ends (onChangeEnd).
type sliderTile struct {
	Title, Subtitle, Label string
	Min, Max               float64
	Divisions              int
}

func (s sliderTile) Show(c *ui.Context, value float64, onChanged, onEnd func(float64)) {
	cs := design.S()
	tt := design.TT()
	ui.Column(c.Key("slider:"+s.Title)).FillWidth().Padding(design.SpaceSM, 0).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			m3.Text(c, s.Title, tt.BodyLarge).Grow(1).Basis(0)
			m3.Text(c, s.Label, tt.LabelLarge.C(cs.Primary))
		})
		if s.Subtitle != "" {
			m3.Text(c, s.Subtitle, tt.BodySmall.C(cs.OnSurfaceVariant))
		}
		v := min(max(value, s.Min), s.Max)
		r := (m3.Slider{Min: s.Min, Max: s.Max, Divisions: s.Divisions, Label: s.Label}).Show(c, &v)
		if v != min(max(value, s.Min), s.Max) && onChanged != nil {
			onChanged(v)
		}
		if r.Ended && onEnd != nil {
			onEnd(v)
		}
	})
}

// settingsTile is a ListTile(contentPadding: zero) with a chevron, the
// row that opens a page.
func settingsTile(c *ui.Context, key any, t m3.Tile) bool {
	t.Pad, t.PadX, t.Tap = true, 0, true
	if t.Trailing == nil && t.TrailingIcon == 0 {
		t.TrailingIcon = icons.ChevronRightRounded
	}
	return t.Show(c.Key(key)).Clicked()
}

// switchTile is a SwitchListTile(contentPadding: zero); it reports a
// change and the new value.
func switchTile(c *ui.Context, key any, t m3.Tile, on bool) (bool, bool) {
	t.Pad, t.PadX = true, 0
	v := on
	changed := m3.SwitchTile(c.Key(key), t, &v)
	return changed, v
}

// poller reruns a refresh while the pane that calls Every keeps showing:
// each frame may arm one timer, so a pane that is gone stops polling.
type poller struct{ armed bool }

func (p *poller) Every(d time.Duration, fn func()) {
	if p.armed {
		return
	}
	p.armed = true
	time.AfterFunc(d, func() {
		core.Post(func() {
			p.armed = false
			fn()
		})
	})
}
