package m3

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
)

// AppBar is Material 3's small top app bar with the app's AppBarTheme:
// 64 high, surface, the title in titleLarge, a 56 DIP leading slot (the
// back button when the navigator can pop), and the surface tint at
// elevation 3 once content scrolls under it.
type AppBar struct {
	Title   string
	TitleFn func()
	// Leading replaces the back button; NoLeading drops it
	// (automaticallyImplyLeading: false).
	Leading   func()
	NoLeading bool
	Actions   func()
	// Bottom is built below the toolbar (a TabBar).
	Bottom   func()
	Scrolled bool
	BG       ui.Color
	// CenterTitle centres the title.
	CenterTitle bool
	// OnBack replaces popping the navigator.
	OnBack func()
	// Height replaces 64.
	Height float32
	// TitleSpacing replaces 16.
	TitleSpacing float32
	HasSpacing   bool
}

// Show builds the bar.
func (a AppBar) Show(c *ui.Context) ui.Element {
	s := cs()
	nav := Nav()
	bg := s.Surface
	if a.BG.A != 0 {
		bg = a.BG
	}
	lvl := float32(0)
	if a.Scrolled {
		lvl = 3
	}
	lvl = ui.Box(c).Size(0, 0).AnimateTo("m3.appbar.lvl", lvl, design.T().Motion(200*time.Millisecond, ui.Linear))
	if lvl > 0 {
		bg = Tinted(bg, lvl)
	}
	h := a.Height
	if h == 0 {
		h = 64
	}
	spacing := float32(16)
	if a.HasSpacing {
		spacing = a.TitleSpacing
	}
	return ui.Column(c).FillWidth().Background(bg).Shrink(0).Children(func() {
		ui.Row(c).Height(h).AlignItems(ui.Center).Children(func() {
			lead := a.Leading
			if lead == nil && !a.NoLeading && (a.OnBack != nil || (nav != nil && nav.CanPop())) {
				lead = func() {
					if (IconButton{Icon: icons.ArrowBack, Tooltip: backLabel, Color: s.OnSurface}).Show(c).Clicked() {
						if a.OnBack != nil {
							a.OnBack()
						} else if nav != nil {
							nav.MaybePop()
						}
					}
				}
			}
			if lead != nil {
				ui.Row(c).Width(56).FillHeight().Center().Shrink(0).Children(lead)
			}
			pad := spacing
			if lead == nil {
				pad = 16
			}
			ui.Row(c).Grow(1).Basis(0).PaddingStart(pad).PaddingEnd(spacing).AlignItems(ui.Center).Children(func() {
				if a.CenterTitle {
					ui.Spacer(c)
				}
				if a.TitleFn != nil {
					a.TitleFn()
				} else {
					Clamp(Text(c, a.Title, tt().TitleLarge.C(s.OnSurface)), 1)
				}
				if a.CenterTitle {
					ui.Spacer(c)
				}
			})
			if a.Actions != nil {
				ui.Row(c).AlignItems(ui.Center).Shrink(0).Children(a.Actions)
			}
		})
		if a.Bottom != nil {
			a.Bottom()
		}
	})
}

// backLabel is MaterialLocalizations' backButtonTooltip in Chinese.
const backLabel = "返回"

// Scaffold lays out a page: an app bar, the body filling the rest, a
// floating action button at the end corner 16 DIPs in, and a bottom bar.
type Scaffold struct {
	AppBar *AppBar
	Body   func()
	FAB    func()
	Bottom func()
	BG     ui.Color
	// FABInset lifts the button, as the phone dock's clearance does.
	FABInset float32
}

// Show builds the page.
func (sc Scaffold) Show(c *ui.Context) ui.Element {
	bg := sc.BG
	if bg.A == 0 {
		bg = cs().Surface
	}
	return ui.Column(c).Fill().Background(bg).Children(func() {
		if sc.AppBar != nil {
			sc.AppBar.Show(c)
		}
		ui.Column(c).Grow(1).Basis(0).FillWidth().Children(func() {
			if sc.Body != nil {
				sc.Body()
			}
			if sc.FAB != nil {
				ui.Box(c).Absolute().Right(16).Bottom(16 + sc.FABInset).Children(sc.FAB)
			}
		})
		if sc.Bottom != nil {
			sc.Bottom()
		}
	})
}
