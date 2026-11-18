package app

import (
	"net/url"
	"runtime"
	"sort"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

// uiSlotCatalog is the slot targets this host renders and how
// contributions combine there. It matches SLOT_TARGETS in
// agent/src/plugins.mjs, which refuses any other target at install
// (docs/specs/0032-ui-extension-protocol.md).
var uiSlotCatalog = map[string]string{
	"course.detail.sections": "list",
	"canvas.toolbar.tools":   "list",
}

// pluginPage is one page a plugin contributes. The route id is stable
// across plugin upgrades; ID is only unique inside the plugin.
type pluginPage struct {
	Entry                               core.PluginEntry
	ID, PluginID, Title, Icon, Renderer string
	Order                               int
	// View is the contribution payload, as a declarative page document.
	View core.J
}

func newPluginPage(e core.PluginEntry) pluginPage {
	return pluginPage{Entry: e, ID: e.ID, PluginID: e.PluginID(), Title: e.Title, Icon: e.Icon, Renderer: e.Renderer(), Order: e.Order(), View: e.Raw}
}

// RouteID is the page's route.
func (p pluginPage) RouteID() string { return p.Entry.Route() }

// HostRendered is whether the host draws the page itself. A v1 page
// carries no document: it is an action whose result the plugin shows
// (spec 0004).
func (p pluginPage) HostRendered() bool { return p.View.O("document") != nil }

// slotContribution is a slot entry the host can render: a declarative
// document on a catalog target, in the target's mode.
type slotContribution struct {
	Entry    core.PluginEntry
	Target   string
	Document core.J
}

// pluginContributions is PluginContributionBinder: the host-rendered pages
// and the slots of active plugins. A plugin whose route or keyed slot
// collides with one already taken is left out whole; the others stay.
func pluginContributions() (map[string]pluginPage, []slotContribution) {
	pages := map[string]pluginPage{}
	var slots []slotContribution
	keys := map[string]bool{}
	for _, plugin := range Plugins.Active() {
		var own []pluginPage
		var ownSlots []slotContribution
		for _, e := range plugin.O("contributes").Os("pages") {
			if e.O("document") != nil {
				own = append(own, newPluginPage(core.NewPluginEntry(plugin, "pages", e)))
			}
		}
		for _, e := range plugin.O("contributes").Os("slots") {
			en := core.NewPluginEntry(plugin, "slots", e)
			mode, ok := uiSlotCatalog[en.Target()]
			if doc := e.O("document"); ok && mode == en.Mode() && doc != nil {
				ownSlots = append(ownSlots, slotContribution{Entry: en, Target: en.Target(), Document: doc})
			}
		}
		clash := false
		for _, p := range own {
			_, taken := pages[p.RouteID()]
			clash = clash || taken
		}
		for _, s := range ownSlots {
			if s.Entry.Mode() == "keyed" {
				clash = clash || keys[s.Target+"/"+s.Entry.SlotKey()]
			}
		}
		if clash {
			continue
		}
		for _, p := range own {
			pages[p.RouteID()] = p
		}
		for _, s := range ownSlots {
			if s.Entry.Mode() == "keyed" {
				keys[s.Target+"/"+s.Entry.SlotKey()] = true
			}
		}
		slots = append(slots, ownSlots...)
	}
	return pages, slots
}

func pluginPageByRoute(route string) (pluginPage, bool) {
	pages, _ := pluginContributions()
	p, ok := pages[route]
	return p, ok
}

// UiSlotOutlet renders one slot target; an empty slot renders nothing, so
// adding a slot never changes a page no plugin contributes to.
func UiSlotOutlet(c *ui.Context, target string, ctx core.J) {
	_, all := pluginContributions()
	var list []slotContribution
	for _, s := range all {
		if s.Target == target {
			list = append(list, s)
		}
	}
	if len(list) == 0 {
		return
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i].Entry, list[j].Entry
		if a.Order() != b.Order() {
			return a.Order() < b.Order()
		}
		if a.Priority() != b.Priority() {
			return a.Priority() > b.Priority()
		}
		if a.PluginID() != b.PluginID() {
			return a.PluginID() < b.PluginID()
		}
		return a.ID < b.ID
	})
	ui.Column(c).FillWidth().Children(func() {
		for _, s := range list {
			PluginDeclarativeView(c, PluginDeclarative{PluginID: s.Entry.PluginID(), ViewID: s.Entry.ID, Document: s.Document, HostContext: ctx})
		}
	})
}

// PluginDeclarative is a plugin's declarative document bound to its view
// state in the daemon.
type PluginDeclarative struct {
	PluginID, ViewID string
	Document         core.J
	// HostContext is readable as context.* (a slot's course_id).
	HostContext core.J
	// OnViewPatch takes the view patches of a host surface, as the study
	// canvas committing a canvas batch; without one they are ignored.
	OnViewPatch func(pluginID string, patch PluginViewPatch)
}

type declarativeView struct {
	client   PluginActionClient
	state    core.J
	renderer DeclarativeRenderer
	onPatch  func(string, PluginViewPatch)
}

// PluginDeclarativeView reads the view's state and revision when it
// mounts, sends each button through the action bridge, applies a state
// patch, opens a page of the same plugin for a navigation result, hands a
// view patch to OnViewPatch and shows a notification or a refusal as a
// snackbar.
func PluginDeclarativeView(c *ui.Context, d PluginDeclarative) {
	e := ui.Column(c.Key("pdv:" + d.PluginID + "/" + d.ViewID)).FillWidth()
	v := ui.Local(e, "pdv", func() *declarativeView {
		v := &declarativeView{state: core.J{}}
		v.client = PluginActionClient{
			OnStatePatch: func(p PluginStatePatch) { v.state = p.State },
			OnNavigation: func(n PluginNavigation) { openPluginNavigation(d.PluginID, n) },
			OnViewPatch: func(p PluginViewPatch) {
				if v.onPatch != nil {
					v.onPatch(d.PluginID, p)
				}
			},
			OnNotification: func(n PluginNotification) { say(n.Text) },
			OnProposal:     func(p PluginProposal) { v.confirm(d, p) },
		}
		v.reload(d)
		return v
	})
	(*v).onPatch = d.OnViewPatch
	view := *v
	state := core.J{}
	for k, x := range view.state {
		state[k] = x
	}
	ctx := d.HostContext
	if ctx == nil {
		ctx = core.J{}
	}
	state["context"] = map[string]any(ctx)
	e.Children(func() {
		view.renderer.Show(c, d.Document, state, func(action string, args core.J) { view.act(d, action, args) })
	})
}

func say(text string) {
	if text != "" {
		widgets.ShowSnack(text, "", nil)
	}
}

func (v *declarativeView) reload(d PluginDeclarative) {
	v.client.ReadState(d.PluginID, d.ViewID, func(state core.J, err error) {
		// On failure the document still renders; its bindings stay empty
		// until an action.
		if err == nil {
			v.state = state
		}
	})
}

func (v *declarativeView) act(d PluginDeclarative, action string, args core.J) {
	v.client.Invoke(PluginActionRequest{PluginID: d.PluginID, ActionID: action, ViewID: d.ViewID, Args: args,
		ContextRevision: v.client.RevisionOf(d.PluginID, d.ViewID)}, func(o PluginActionOutcome, err error) {
		if err != nil {
			say(l10n.PluginActionFailed(pluginActionTitle(d.PluginID, action), err.Error()))
			return
		}
		if f := o.Failure; f != nil {
			if f.Code == "revision_conflict" {
				v.reload(d)
			}
			say(l10n.PluginActionFailed(pluginActionTitle(d.PluginID, action), f.Message))
		}
	})
}

// confirm asks before a plugin writes to the library; nothing is written
// unless the user approves.
func (v *declarativeView) confirm(d PluginDeclarative, p PluginProposal) {
	title := d.PluginID
	if pl := Plugins.ByID(d.PluginID); pl != nil {
		title = pl.S("title")
	}
	askProposal(p, title, func(approved bool) {
		settle := v.client.Reject
		if approved {
			settle = v.client.Apply
		}
		settle(p, func(o PluginActionOutcome, err error) {
			switch {
			case err != nil:
				say(l10n.PluginActionFailed(p.Summary, err.Error()))
			case o.Failure != nil:
				say(l10n.PluginActionFailed(p.Summary, o.Failure.Message))
			}
		})
	})
}

// askProposal shows a pending write and answers whether to apply it.
func askProposal(p PluginProposal, pluginTitle string, done func(approved bool)) {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: l10n.PluginProposalTitle(p.Summary), Message: l10n.PluginProposalBody(pluginTitle), Actions: func() {
			if m3.TextButton(c, l10n.PluginProposalReject).Clicked() {
				r.Pop(false)
			}
			if m3.FilledButton(c, l10n.PluginProposalApply).Clicked() {
				r.Pop(true)
			}
		}})
	}, func(v any) { done(v == true) })
}

func pluginActionTitle(pluginID, action string) string {
	for _, a := range Plugins.ByID(pluginID).O("contributes").Os("actions") {
		if a.S("id") == action {
			return a.S("title")
		}
	}
	return action
}

// openPluginNavigation opens a page of the plugin for a navigation result.
func openPluginNavigation(pluginID string, n PluginNavigation) {
	page, ok := pluginPageByRoute(n.Route)
	if !ok {
		page, ok = pluginPageByRoute("plugin:" + pluginID + ":" + n.Route)
	}
	if !ok {
		t := n.Title
		if t == "" {
			t = n.Route
		}
		say(l10n.PluginRouteMissing(t))
		return
	}
	if n.Replace {
		m3.RootNav.Replace(newPluginPageOutlet(page))
	} else {
		m3.RootNav.Push(newPluginPageOutlet(page))
	}
}

// pluginPageOutlet is the host-owned frame for a plugin page: the app
// keeps the route, app bar and fallback; the content comes from the
// page's renderer.
type pluginPageOutlet struct {
	page   pluginPage
	gone   bool
	web    *pluginWebPage
	native bool
}

func newPluginPageOutlet(p pluginPage) *pluginPageOutlet { return &pluginPageOutlet{page: p} }

// Dispose ends a web page's session.
func (o *pluginPageOutlet) Dispose() {
	if o.web != nil {
		o.web.close()
	}
}

func (o *pluginPageOutlet) View(c *ui.Context) {
	// A page whose plugin was disabled, removed or lost the page leaves
	// with the normal pop instead of staying on screen (spec 0032, NAV-02).
	if _, ok := pluginPageByRoute(o.page.RouteID()); !ok && o.page.HostRendered() {
		if !o.gone {
			o.gone = true
			core.Post(func() { m3.RootNav.PopPage(o) })
		}
		ui.Box(c).Fill().Background(design.S().Surface)
		return
	}
	switch {
	case o.native || o.page.Renderer == "declarative":
		o.declarative(c)
	case o.page.Renderer == "web-ui":
		if o.web == nil {
			o.web = openPluginWebPage(o.page)
		}
		o.web.view(c, func() { o.native = true })
	default:
		pluginFallbackPage(c, o.page.Title, l10n.PluginViewUnsupported)
	}
}

// declarative is the declarative page renderer: the page document wired
// to the action bridge.
func (o *pluginPageOutlet) declarative(c *ui.Context) {
	m3.Scaffold{AppBar: &m3.AppBar{Title: o.page.Title, OnBack: func() {
		if o.native && o.page.Renderer == "web-ui" {
			o.native = false
			return
		}
		m3.RootNav.PopPage(o)
	}}, Body: func() {
		doc := o.page.View.O("document")
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(design.SpaceMD).Children(func() {
				if doc == nil {
					DeclarativeErrorView(c)
					return
				}
				PluginDeclarativeView(c, PluginDeclarative{PluginID: o.page.PluginID, ViewID: o.page.ID, Document: doc})
			})
		})
	}}.Show(c)
}

// pluginFallbackPage is a safe page for a missing renderer, an
// unavailable plugin or a failed contribution. It keeps the plugin title
// and never deletes user data.
func pluginFallbackPage(c *ui.Context, title, message string) {
	m3.Scaffold{AppBar: &m3.AppBar{Title: title}, Body: func() {
		widgets.EmptyState{Icon: icons.ExtensionOffOutlined, Title: title, Body: message}.Show(c)
	}}.Show(c)
}

// PluginPageLauncher lists the pages contributed by active plugins and
// opens them through the host outlet.
func PluginPageLauncher(c *ui.Context) {
	entries := Plugins.Entries("pages", "", "")
	if len(entries) == 0 {
		return
	}
	cs := design.S()
	ui.Column(c).FillWidth().Children(func() {
		m3.Text(c, l10n.PluginPages, design.TT().LabelLarge.C(cs.OnSurfaceVariant)).Margin(design.SpaceMD, 0, design.SpaceSM, design.SpaceMD)
		for i, e := range entries {
			e := e
			page := newPluginPage(e)
			if (m3.Tile{Leading: func() { m3.Icon(c.Key(i), widgets.PluginIcon(page.Icon), 24, cs.Primary) }, Title: page.Title,
				Subtitle: page.PluginID, SubtitleLines: 1, TrailingIcon: icons.ChevronRightRounded, Tap: true,
				Pad: true, PadX: design.SpaceMD}).Show(c.Key(i)).Clicked() {
				if page.HostRendered() || page.Renderer == "web-ui" {
					m3.RootNav.Push(newPluginPageOutlet(page))
				} else {
					runPluginEntry(e, core.J{}, nil)
				}
			}
		}
	})
}

// pluginWebPlatform is the platform name of the admission matrix, or the
// OS name for one outside it (which the daemon reports as unsupported).
func pluginWebPlatform() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return runtime.GOOS
}

// pluginWebPage is the Web UI (L3) renderer of a plugin page
// (docs/specs/0032-ui-extension-protocol.md, Isolated Web UI). The daemon
// decides the platform admission; where it does not admit this platform,
// or where this client has no bridged web view, the page shows the native
// fallback with the daemon's reason.
type pluginWebPage struct {
	page    pluginPage
	opening core.J
	err     error
	session string
	closed  bool
}

func openPluginWebPage(p pluginPage) *pluginWebPage {
	w := &pluginWebPage{page: p}
	a := api()
	body := core.J{"pluginId": p.PluginID, "pageId": p.ID, "platform": pluginWebPlatform()}
	core.Async(func() (any, error) { return a.Post("/api/plugin-web/sessions", body) }, func(v any, err error) {
		if err != nil {
			w.err = err
			return
		}
		w.opening = objOrEmpty(v)
		w.session = w.opening.O("session").S("id")
		if w.closed {
			w.close()
		}
	})
	return w
}

// close ends the session; its later messages are refused.
func (w *pluginWebPage) close() {
	w.closed = true
	if w.session == "" {
		return
	}
	id := w.session
	w.session = ""
	a := api()
	core.Go(func() error { _, err := a.Delete("/api/plugin-web/sessions/" + url.PathEscape(id)); return err }, func(error) {})
}

func (w *pluginWebPage) view(c *ui.Context, openNative func()) {
	var native func()
	if w.page.HostRendered() {
		native = openNative
	}
	switch {
	case w.err != nil:
		pluginWebFallback(c, w.page.Title, "needs-review", "resources-missing", l10n.PluginWebOpenFailed(w.err.Error()), native)
	case w.opening == nil:
		m3.Scaffold{AppBar: &m3.AppBar{Title: w.page.Title}, Body: func() {
			ui.Box(c).Fill().Center().Children(func() { m3.Text(c, l10n.PluginWebLoading, design.TT().BodyMedium) })
		}}.Show(c)
	default:
		reason := "renderer-missing"
		if f := w.opening.O("fallback"); f != nil {
			reason = f.S("reason")
		}
		// This client has no bridged web view yet, so an admitted page
		// falls back as a missing renderer.
		pluginWebFallback(c, w.page.Title, w.opening.O("admission").S("status"), reason, w.opening.O("fallback").S("detail"), native)
	}
}

func pluginWebReason(code string) string {
	switch code {
	case "platform-unsupported":
		return l10n.PluginWebReasonPlatformUnsupported
	case "platform-needs-review":
		return l10n.PluginWebReasonNeedsReview
	case "review-pending":
		return l10n.PluginWebReasonReviewPending
	case "plugin-inactive":
		return l10n.PluginWebReasonPluginInactive
	case "resources-missing":
		return l10n.PluginWebReasonResourcesMissing
	}
	return l10n.PluginWebReasonRendererMissing
}

// pluginWebFallback is the native fallback for a page whose renderer is
// web-ui: it says why the page cannot render and keeps the plugin's
// native pages, slots and actions usable. An unknown admission is read as
// still under review, never as supported.
func pluginWebFallback(c *ui.Context, title, admission, reason, detail string, openNative func()) {
	cs := design.S()
	tt := design.TT()
	m3.Scaffold{AppBar: &m3.AppBar{Title: title}, Body: func() {
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(design.SpaceXXL, design.SpaceXL).Children(func() {
				ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
					ui.Box(c).Size(72, 72).Radius(36).Center().Background(cs.SecondaryContainer).Children(func() {
						m3.Icon(c, icons.WebAssetOffOutlined, 32, cs.OnSecondaryContainer)
					})
				})
				m3.VGap(c, design.SpaceXL)
				m3.Text(c, l10n.PluginWebUnavailableTitle, tt.TitleLarge).FillWidth().TextAlign(ui.Center)
				m3.VGap(c, design.SpaceSM)
				m3.Text(c, pluginWebReason(reason), tt.BodyMedium.C(cs.OnSurfaceVariant)).FillWidth().TextAlign(ui.Center)
				if detail != "" {
					m3.VGap(c, design.SpaceXS)
					m3.Text(c, detail, tt.BodySmall.C(cs.OnSurfaceVariant)).FillWidth().TextAlign(ui.Center)
				}
				m3.VGap(c, design.SpaceXL)
				label := l10n.PluginWebAdmissionNeedsReview
				switch admission {
				case "supported":
					label = l10n.PluginWebAdmissionSupported
				case "unsupported":
					label = l10n.PluginWebAdmissionUnsupported
				}
				ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
					ui.Box(c).Padding(design.SpaceXS, design.SpaceMD).Border(1, cs.OutlineVariant).Radius(design.CornerFull).Children(func() {
						m3.Text(c, label, tt.LabelMedium.C(cs.OnSurfaceVariant))
					})
				})
				m3.VGap(c, design.SpaceXL)
				ui.Column(c).FillWidth().Padding(design.SpaceLG).Background(cs.SurfaceContainerHighest).Radius(design.CornerMD).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Children(func() {
						m3.Icon(c, icons.ExtensionOutlined, 18, cs.OnSurfaceVariant)
						m3.HGap(c, design.SpaceSM)
						m3.Text(c, l10n.PluginWebNativeKeptTitle, tt.TitleSmall)
					})
					m3.VGap(c, design.SpaceSM)
					m3.Text(c, l10n.PluginWebNativeKept, tt.BodySmall.C(cs.OnSurfaceVariant))
				})
				m3.VGap(c, design.SpaceXL)
				if openNative != nil && (m3.Button{Kind: m3.Tonal, Icon: icons.OpenInNewRounded, Label: l10n.PluginWebOpenNative}).Show(c).FillWidth().Clicked() {
					openNative()
				}
				m3.VGap(c, design.SpaceSM)
				if (m3.Button{Kind: m3.TextKind, Label: l10n.CommonBack}).Show(c).FillWidth().Clicked() {
					m3.RootNav.MaybePop()
				}
			})
		})
	}}.Show(c)
}
