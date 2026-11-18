package widgets

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// CenteredLoader is states.dart's: the contained loader, with a label
// below it, in the middle of the space it fills.
func CenteredLoader(c *ui.Context, label string) ui.Element {
	return ui.Column(c).Fill().Center().Children(func() {
		Loader{Contained: true}.Show(c)
		if label != "" {
			m3.VGap(c, design.SpaceLG)
			m3.Text(c, label, design.TT().BodyMedium.C(design.S().OnSurfaceVariant))
		}
	})
}

// EmptyState is an icon in an eight-pointed rounded star, a title, a body
// and an action, centred and at most 360 wide.
type EmptyState struct {
	Icon   icons.Icon
	Title  string
	Body   string
	Action func()
}

// Show builds the state.
func (e EmptyState) Show(c *ui.Context) ui.Element {
	s := design.S()
	tt := design.TT()
	return ui.Column(c).Fill().Center().Children(func() {
		ui.Column(c).Padding(design.SpaceXXL).MaxWidth(360 + 2*design.SpaceXXL).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(88, 88).Shrink(0).Center().Draw(func(p *ui.Painter, r ui.Rect) {
				path := StarPath(r, 8, .82, .55, .45)
				p.FillPath(&path, s.SecondaryContainer)
			}).Children(func() { m3.Icon(c, e.Icon, 36, s.OnSecondaryContainer) })
			m3.VGap(c, design.SpaceXL)
			m3.Text(c, e.Title, tt.TitleLarge.C(s.OnSurface)).TextAlign(ui.Center)
			if e.Body != "" {
				m3.VGap(c, design.SpaceSM)
				m3.Text(c, e.Body, tt.BodyMedium.C(s.OnSurfaceVariant)).TextAlign(ui.Center)
			}
			if e.Action != nil {
				m3.VGap(c, design.SpaceXL)
				e.Action()
			}
		})
	})
}

// ErrorState is the empty state of a failed load, with a retry button when
// retry is set.
func ErrorState(c *ui.Context, err error, retry func()) ui.Element {
	title := l10n.StateLoadFailed
	var ae *core.ApiError
	if errors.As(err, &ae) && ae.Status == 401 {
		title = l10n.StateSessionExpired
	}
	st := EmptyState{Icon: icons.CloudOffRounded, Title: title, Body: fmt.Sprint(err)}
	if retry != nil {
		st.Action = func() {
			if (m3.Button{Kind: m3.Tonal, Icon: icons.RefreshRounded, Label: l10n.CommonRetry}).Show(c).Clicked() {
				retry()
			}
		}
	}
	return st.Show(c)
}

// ContentWidth keeps page content to a readable width on large windows,
// at the top centre.
func ContentWidth(c *ui.Context, max float32, child func()) ui.Element {
	if max == 0 {
		max = design.MaxContentWidth
	}
	return ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(max).Children(child)
	})
}

// ShowSnack is showSnack: brief feedback at the bottom of the window,
// replacing the current one, for 4 s, or 6 s with an action.
func ShowSnack(msg string, action string, onAction func()) {
	m3.HideSnackBar()
	sb := m3.SnackBar{Message: msg, Duration: 4 * time.Second}
	if action != "" {
		sb.Action, sb.OnAction, sb.Duration = action, onAction, 6*time.Second
	}
	m3.ShowSnackBar(sb)
}

// InlineError is a failure message next to the action that failed.
func InlineError(c *ui.Context, text string) ui.Element {
	s := design.S()
	return ui.Row(c).FillWidth().Padding(design.SpaceMD, design.SpaceLG).Background(s.ErrorContainer).Radius(design.CornerMD).
		AlignItems(ui.Start).Children(func() {
		m3.Icon(c, icons.ErrorOutlineRounded, 20, s.OnErrorContainer)
		m3.HGap(c, design.SpaceMD)
		m3.Text(c, text, design.TT().BodyMedium.C(s.OnErrorContainer)).Grow(1).Basis(0)
	})
}

// CircleAvatar is a 40 DIP circle of bg holding child in fg's colour.
func CircleAvatar(c *ui.Context, size float32, bg ui.Color, child func()) ui.Element {
	if size == 0 {
		size = 40
	}
	return ui.Box(c).Size(size, size).Shrink(0).Radius(size / 2).Background(bg).Center().Clip().Children(child)
}

// OpenPlainLink opens a link that is not a coursive:// reference: web and
// mail links in the system's handler; anything else says why nothing
// happens.
func OpenPlainLink(c *ui.Context, raw string) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "mailto") {
		ShowSnack(l10n.LinkCannotOpen, "", nil)
		return
	}
	c.OpenURLThen(u.String(), func(err error) {
		if err != nil {
			ShowSnack(l10n.LinkCannotOpen, "", nil)
		}
	})
}
