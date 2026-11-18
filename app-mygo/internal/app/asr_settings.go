package app

import (
	"fmt"
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

// asrMark is a brand icon: the asset under app/assets and whether it is
// single-colour, tinted with onSurface.
type asrMark struct {
	asset string
	mono  bool
}

// asrFamilies is the maker of each local model family, matched by catalog
// id prefix so new sizes of a family keep their icon.
var asrFamilies = []struct {
	prefix string
	mark   asrMark
}{
	{"qwen", asrMark{"providers/qwen-color.svg", false}},
	{"sensevoice", asrMark{"providers/alibaba-color.svg", false}},
	{"paraformer", asrMark{"providers/alibaba-color.svg", false}},
	{"fun-asr", asrMark{"providers/alibaba-color.svg", false}},
	{"firered", asrMark{"brands/firered.png", false}},
	{"whisper", asrMark{"providers/openai.svg", true}},
}

func asrModelMark(model any) *asrMark {
	id := core.Str(model)
	for _, f := range asrFamilies {
		if strings.HasPrefix(id, f.prefix) {
			m := f.mark
			return &m
		}
	}
	return nil
}

// asrChannelMarks is the service icon of each cloud channel.
var asrChannelMarks = map[string]*asrMark{
	"bcut":     {"brands/bcut.png", false},
	"jianying": {"brands/jianying.png", false},
}

// asrMarkImage is the icon alone, size square; bitmaps get rounded
// corners.
func asrMarkImage(c *ui.Context, m asrMark, size float32) {
	r := float32(0)
	if !strings.HasSuffix(m.asset, ".svg") {
		r = size * .25
	}
	brandIcon(c, m.asset, size, r, m.mono)
}

// AsrSettings is the free cloud transcription channels: enable/disable,
// strategy, concurrency and live status from the daemon's ASR pool.
type AsrSettings struct {
	chans, events []core.J
	poll          poller
	logOpen       bool
}

// NewAsrSettings loads the pool.
func NewAsrSettings() *AsrSettings {
	a := &AsrSettings{}
	a.load()
	return a
}

func (a *AsrSettings) load() {
	core.Async(func() (any, error) { return api().Get("/api/asr") }, func(v any, err error) {
		if err == nil {
			r := core.Obj(v)
			a.chans, a.events = r.Os("channels"), r.Os("events")
		}
	})
}

func (a *AsrSettings) toggle(s *ServerSettings, id string, on bool) {
	var d []any
	for _, x := range core.Strs(s.Values["asr.disabled"]) {
		if x != id {
			d = append(d, x)
		}
	}
	if !on {
		d = append(d, id)
	}
	if d == nil {
		d = []any{}
	}
	s.Put("asr.disabled", d)
	a.load()
}

func (a *AsrSettings) slotsHint() string {
	var parts []string
	for _, ch := range a.chans {
		slots := any(1)
		if ch.Has("slots") {
			slots = ch.I("slots")
		}
		parts = append(parts, l10n.AsrChannelSlots(ch.S("name"), slots))
	}
	if len(parts) == 0 {
		return l10n.AsrPerChannelLimits
	}
	return l10n.AsrChannelLimits(strings.Join(parts, l10n.ListSeparator))
}

// Show builds the pane; it polls every 4 seconds while it shows.
func (a *AsrSettings) Show(c *ui.Context, s *ServerSettings) {
	a.poll.Every(4*time.Second, a.load)
	cs := design.S()
	tt := design.TT()
	strategy := core.Str(s.Values["asr.strategy"])
	if strategy == "" {
		strategy = "balance"
	}
	conc := s.Num("asr.concurrency", 2)
	var local core.J
	for _, ch := range a.chans {
		if ch.IsTrue("local") {
			local = ch
			break
		}
	}
	ui.Column(c).FillWidth().Children(func() {
		m3.Text(c, l10n.AsrIntro, tt.BodySmall.C(cs.OnSurfaceVariant).H(1.6)).Margin(design.SpaceSM, 0)
		bg, fg := cs.PrimaryContainer, cs.OnPrimaryContainer
		title, sub, ic := l10n.AsrTryLocal, l10n.AsrLocalHint, icons.ComputerRounded
		if local != nil {
			bg, fg = cs.SecondaryContainer, cs.OnSecondaryContainer
			title, sub, ic = l10n.AsrLocalEnabled, l10n.AsrLocalUnlimited(local.S("name")), icons.OfflineBoltRounded
		}
		mark := asrModelMark(local["model"])
		ui.Box(c.Key("local")).FillWidth().Margin(design.SpaceSM, 0, 0, 0).Background(bg).Radius(design.CornerMD).Clip().Children(func() {
			t := m3.Tile{Title: title, Subtitle: sub, TrailingIcon: icons.ChevronRightRounded, IconColor: fg, TextColor: fg, Tap: true,
				Pad: true, PadX: design.SpaceLG, PadY: design.SpaceXS, Radius: design.CornerLG}
			if mark != nil {
				t.Leading = func() { asrMarkImage(c, *mark, 24) }
			} else {
				t.LeadingIcon = ic
			}
			if t.Show(c).Clicked() {
				m3.Nav().Push(NewLocalAsrPage(a.load))
			}
		})
		for _, ch := range a.chans {
			ch := ch
			id := ch.S("id")
			ui.Box(c.Key("chan:"+id)).FillWidth().Margin(design.SpaceSM, 0, 0, 0).Children(func() {
				if on, ok := asrChannelCard(c, ch); ok {
					a.toggle(s, id, on)
				}
			})
		}
		m3.VGap(c, design.SpaceLG)
		m3.Text(c, l10n.AsrDistribution, tt.TitleSmall)
		m3.VGap(c, design.SpaceSM)
		segs := []m3.Segment[string]{{Value: "balance", Icon: icons.BalanceRounded, Label: l10n.AsrBalanced}}
		for _, ch := range a.chans {
			segs = append(segs, m3.Segment[string]{Value: ch.S("id"), Label: l10n.AsrPrefer(ch.S("name"))})
		}
		if m3.Segmented(c, segs, &strategy, m3.SegOpts{NoCheck: true}) {
			s.Put("asr.strategy", strategy)
		}
		m3.VGap(c, design.SpaceLG)
		n := int(conc + .5)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				m3.Text(c, l10n.AsrConcurrency, tt.TitleSmall)
				m3.Text(c, a.slotsHint(), tt.BodySmall.C(cs.OnSurfaceVariant))
			})
			m3.Text(c, l10n.AsrSegments(n), tt.LabelLarge.C(cs.Primary))
		})
		v := min(max(conc, 1), 4)
		r := (m3.Slider{Min: 1, Max: 4, Divisions: 3, Label: strconv.Itoa(n)}).Show(c, &v)
		if int(v+.5) != n {
			s.Set("asr.concurrency", float64(int(v+.5)))
		}
		if r.Ended {
			s.Put("asr.concurrency", int(v+.5))
		}
		if len(a.events) > 0 {
			a.eventLog(c)
		}
		if ch, on := switchTile(c, "delete", m3.Tile{LeadingIcon: icons.AutoDeleteOutlined, Title: l10n.AsrDeleteVideo, Subtitle: l10n.AsrDeleteVideoHint},
			s.Values["transcript.delete_video"] == true); ch {
			s.Put("transcript.delete_video", on)
		}
	})
}

func asrHM(unix int) string {
	t := time.Unix(int64(unix), 0)
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}

func asrDur(s float64) string {
	m := int(s/60 + .5)
	if m >= 60 {
		d := 1
		if m%60 == 0 {
			d = 0
		}
		return l10n.DurationHoursOnly(core.Fixed(float64(m)/60, d))
	}
	return l10n.DurationMinutes(m)
}

// asrChannelCard is a channel's status, quota and switch; it reports a
// toggle.
func asrChannelCard(c *ui.Context, ch core.J) (toggled, changed bool) {
	cs := design.S()
	tt := design.TT()
	state := ch.S("state")
	enabled := ch.IsTrue("enabled")
	used := ch.F("used_secs")
	quota := 1.0
	if ch.Has("quota_secs") {
		quota = ch.F("quota_secs")
	}
	label, color := l10n.CommonDisabled, cs.Outline
	switch state {
	case "ready":
		label, color = l10n.AsrReady, cs.Primary
	case "busy":
		label, color = l10n.AsrBusy(ch["busy"]), cs.Tertiary
	case "cooling":
		label, color = l10n.AsrCooling(asrHM(ch.I("cool_until"))), cs.Error
	case "quota":
		label, color = l10n.AsrQuotaFull, cs.Error
	}
	local := ch.IsTrue("local")
	op := float32(.6)
	if enabled {
		op = 1
	}
	card := ui.Column(c).FillWidth().Padding(design.SpaceMD, design.SpaceSM, design.SpaceMD, design.SpaceLG).
		Background(cs.SurfaceContainerHigh).Radius(design.CornerLG)
	card = card.Opacity(card.AnimateTo("asr.op", op, design.T().Motion(design.Medium1, design.Standard)))
	card.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			name := ch.S("name")
			parts := strings.Split(name, " · ")
			text := ""
			if r := []rune(parts[len(parts)-1]); len(r) > 0 {
				text = string(r[0])
			}
			mark := asrChannelMarks[ch.S("id")]
			if local {
				mark = asrModelMark(ch["model"])
			}
			asrBadge(c, text, mark, state == "busy")
			m3.HGap(c, design.SpaceMD)
			ui.Column(c).Grow(1).Basis(0).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					m3.Clamp(m3.Text(c, name, tt.TitleSmall), 1).Shrink(1)
					m3.HGap(c, design.SpaceSM)
					ui.Box(c).Padding(2, 8).Radius(design.CornerFull).Background(design.WithOpacity(color, .12)).Shrink(0).Children(func() {
						m3.Text(c, label, tt.LabelSmall.C(color))
					})
				})
				m3.VGap(c, 2)
				m3.Text(c, ch.S("note"), tt.BodySmall.C(cs.OnSurfaceVariant))
			})
			v := enabled
			if m3.Switch(c, &v, false).Changed() {
				toggled, changed = v, true
			}
		})
		m3.VGap(c, design.SpaceMD)
		if !local {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(0, design.SpaceMD, 0, 0).Children(func() {
				col := cs.Primary
				if used >= quota {
					col = cs.Error
				}
				ui.Box(c).Grow(1).Basis(0).Children(func() {
					m3.Linear(c, m3.Progress{Value: m3.Of(min(max(used/quota, 0), 1)), Color: col})
				})
				m3.HGap(c, design.SpaceMD)
				m3.Text(c, l10n.AsrQuotaUsage(asrDur(used), asrDur(quota)), tt.LabelSmall.C(cs.OnSurfaceVariant))
			})
		}
		m3.VGap(c, design.SpaceXS)
		parts := []string{l10n.AsrOkSegments(ch.I("ok")), l10n.AsrFailedSegments(ch.I("fail"))}
		if lu := ch.I("last_used"); lu != 0 {
			parts = append(parts, l10n.CommonLastUsed(core.FmtRelative(time.Unix(int64(lu), 0))))
		}
		m3.Text(c, strings.Join(parts, " · "), tt.LabelSmall.C(cs.OnSurfaceVariant))
		if e := ch.S("last_error"); e != "" && state == "cooling" {
			m3.Clamp(m3.Text(c, e, tt.LabelSmall.C(cs.Error)), 2).Margin(2, design.SpaceMD, 0, 0)
		}
	})
	return
}

// eventLog is the recent chunk attempts, newest first, to see why
// transcription failed.
func (a *AsrSettings) eventLog(c *ui.Context) {
	cs := design.S()
	tt := design.TT()
	fails := 0
	for _, e := range a.events {
		if !e.IsTrue("ok") {
			fails++
		}
	}
	m3.Expansion{Tile: m3.Tile{LeadingIcon: icons.HistoryRounded, Title: l10n.AsrRecent, Subtitle: l10n.AsrRecentSummary(len(a.events), fails), Pad: true, PadX: 0},
		Open: &a.logOpen, Children: func() {
			for i, e := range a.events {
				if i == 12 {
					break
				}
				ui.Row(c.Key(i)).FillWidth().AlignItems(ui.Start).Padding(design.SpaceXS, 0).Children(func() {
					ic, col := icons.ErrorOutlineRounded, cs.Error
					if e.IsTrue("ok") {
						ic, col = icons.CheckCircleOutlineRounded, cs.Primary
					}
					m3.Icon(c, ic, 18, col)
					m3.HGap(c, design.SpaceMD)
					ui.Column(c).Grow(1).Basis(0).Children(func() {
						m3.Text(c, l10n.AsrEventLine(asrHM(e.I("at")), e["channel"], e["chunk"], asrDur(e.F("secs")), int(e.F("took")+.5)), tt.LabelMedium)
						if msg := e.S("error"); msg != "" {
							m3.Clamp(m3.Text(c, msg, tt.LabelSmall.C(cs.Error)), 2)
						}
					})
				})
			}
		}}.Show(c, "events")
}

// asrBadge is a channel's badge: a soft cookie shape that turns while the
// channel works, filled with the service's app icon, or holding the model
// maker's mark.
func asrBadge(c *ui.Context, text string, mark *asrMark, busy bool) {
	cs := design.S()
	b := ui.Box(c).Size(40, 40).Shrink(0).Center().Label(text)
	t := float64(b.AnimateTo("asr.busy", map[bool]float32{true: 1, false: 0}[busy], design.T().Motion(design.Long2, design.Emphasized)))
	path := func(r ui.Rect) ui.Path {
		return widgets.ShapePath(widgets.MorphRadii(widgets.ShapeCookie9, widgets.ShapeSoftBurst, t), r, t*.6)
	}
	switch {
	case mark != nil && !strings.HasSuffix(mark.asset, ".svg"):
		img := brandImage(mark.asset)
		b.Draw(func(p *ui.Painter, r ui.Rect) {
			if img == nil {
				return
			}
			clip := path(r)
			p.ClipPath(&clip, func() { p.Image(img, r, ui.Cover) })
		})
	case mark != nil:
		b.Draw(func(p *ui.Painter, r ui.Rect) { pp := path(r); p.FillPath(&pp, cs.SurfaceContainerLowest) }).Children(func() {
			asrMarkImage(c, *mark, 22)
		})
	default:
		b.Draw(func(p *ui.Painter, r ui.Rect) { pp := path(r); p.FillPath(&pp, cs.SecondaryContainer) }).Children(func() {
			m3.Text(c, text, design.TT().TitleSmall.C(cs.OnSecondaryContainer))
		})
	}
}
