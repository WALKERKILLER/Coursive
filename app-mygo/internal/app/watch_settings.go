package app

import (
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

// WatchSettings is the background refresh of signed-in sources with
// in-app notifications. Contract: docs/specs/0010-live-refresh.md
type WatchSettings struct {
	st      core.J
	running bool
}

// NewWatchSettings loads the watch status.
func NewWatchSettings() *WatchSettings {
	w := &WatchSettings{}
	w.load()
	return w
}

func (w *WatchSettings) load() {
	core.Async(func() (any, error) { return api().Get("/api/watch") }, func(v any, err error) {
		if err == nil {
			w.st = core.Obj(v)
		}
	})
}

func (w *WatchSettings) runNow() {
	w.running = true
	core.Async(func() (any, error) { return api().Post("/api/watch/run", nil) }, func(_ any, err error) {
		if err != nil {
			widgets.ShowSnack(apiMessage(err), "", nil)
		} else {
			widgets.ShowSnack(l10n.WatchCheckStarted, "", nil)
		}
		time.AfterFunc(2*time.Second, func() {
			core.Post(func() {
				w.load()
				w.running = false
			})
		})
	})
}

// Show builds the switch, the status line and 立即检查.
func (w *WatchSettings) Show(c *ui.Context, s *ServerSettings) {
	cs := design.S()
	tt := design.TT()
	on := s.Values["watch.enabled"] != false
	awaiting, next := 0, 0
	for _, co := range w.st.Os("courses") {
		awaiting += co.I("awaiting")
		if t := co.I("next_at"); t > 0 && (next == 0 || t < next) {
			next = t
		}
	}
	last := w.st.I("last_run")
	errText := w.st.S("last_error")
	ui.Column(c).FillWidth().Children(func() {
		if ch, v := switchTile(c, "watch", m3.Tile{LeadingIcon: icons.SyncRounded, Title: l10n.WatchTitle, Subtitle: l10n.WatchBody}, on); ch {
			s.Put("watch.enabled", v)
		}
		if on && w.st != nil {
			parts := []string{l10n.WatchNoneAwaiting}
			if awaiting > 0 {
				parts[0] = l10n.WatchAwaiting(awaiting)
			}
			if next > 0 {
				parts = append(parts, l10n.WatchNextCheck(core.FmtUntil(time.Unix(int64(next), 0))))
			}
			if last > 0 {
				parts = append(parts, l10n.CommonLastUsed(core.FmtRelative(time.Unix(int64(last), 0))))
			}
			m3.Text(c, strings.Join(parts, " · "), tt.BodySmall.C(cs.OnSurfaceVariant)).Margin(0, 0, design.SpaceSM, 56)
		}
		if errText != "" {
			m3.Clamp(m3.Text(c, l10n.WatchLastError(errText), tt.BodySmall.C(cs.Error)), 2).Margin(0, 0, design.SpaceSM, 56)
		}
		ui.Row(c).Padding(0, 0, 0, 48).Children(func() {
			if (m3.Button{Kind: m3.TextKind, Icon: icons.RefreshRounded, Label: l10n.WatchCheckNow, Disabled: !on || w.running, Busy: w.running}).Show(c).Clicked() {
				w.runNow()
			}
		})
	})
}
