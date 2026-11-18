package core

import (
	"strings"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// PluginEntry is one place a plugin appears outside the chat: an action on
// a surface, a note view, an exporter, a page or a slot
// (docs/specs/0004-plugin-platform.md).
type PluginEntry struct {
	Plugin J
	// Kind is actions, noteViews, exporters, pages or slots.
	Kind                                    string
	Raw                                     J
	ID, Title, Icon, Action, Surface, Scope string
}

// NewPluginEntry reads an entry of a plugin's contributes.
func NewPluginEntry(plugin J, kind string, e J) PluginEntry {
	return PluginEntry{
		Plugin: plugin, Kind: kind, Raw: e,
		ID: Str(e["id"]), Title: Str(e["title"]), Icon: e.Or("icon", "extension"),
		Action: Str(e["action"]), Surface: e.Or("surface", ""), Scope: e.Or("scope", ""),
	}
}

func (e PluginEntry) PluginID() string    { return Str(e.Plugin["id"]) }
func (e PluginEntry) PluginTitle() string { return Str(e.Plugin["title"]) }
func (e PluginEntry) Target() string      { return e.Raw.Or("target", "") }
func (e PluginEntry) Renderer() string    { return e.Raw.Or("renderer", "declarative") }
func (e PluginEntry) Route() string {
	return e.Raw.Or("route", "plugin:"+e.PluginID()+":"+e.ID)
}
func (e PluginEntry) Order() int {
	if _, ok := e.Raw["order"].(float64); ok {
		return e.Raw.I("order")
	}
	return 100
}
func (e PluginEntry) Priority() int                 { return e.Raw.I("priority") }
func (e PluginEntry) Mode() string                  { return e.Raw.Or("mode", "list") }
func (e PluginEntry) SlotKey() string               { return e.Raw.Or("key", e.ID) }
func (e PluginEntry) RequiredServices() []string    { return e.Raw.Ss("required_services") }
func (e PluginEntry) RequiredPermissions() []string { return e.Raw.Ss("required_permissions") }
func (e PluginEntry) AvailableOn() []string         { return e.Raw.Ss("available_on") }
func (e PluginEntry) MinHostVersion() string        { return e.Raw.Or("min_host_version", "") }
func (e PluginEntry) FallbackRoute() string         { return e.Raw.Or("fallback_route", "") }

// PluginStore is the agent host's installed plugins and their routes.
type PluginStore struct {
	app             *AppState
	Installed       []J
	AllowThirdParty bool
	Loaded          bool
	Error           error
	listening       bool

	// OnNotice shows a plugin's short message (coursive.notify).
	OnNotice func(string)
	// OnChange runs after the installed set changes.
	OnChange func()
}

// NewPluginStore is the store of app's agent host.
func NewPluginStore(app *AppState) *PluginStore { return &PluginStore{app: app} }

// Ready is whether the agent host is up.
func (s *PluginStore) Ready() bool { return s.app.AgentReady() }

// Watch starts listening once the agent is ready; call it every frame.
func (s *PluginStore) Watch() {
	if s.listening || !s.app.AgentReady() || !s.app.Connected() {
		return
	}
	s.listening = true
	s.app.Events.Listen(func(e J) {
		if e.S("type") != "plugin_event" {
			return
		}
		d := e.O("data")
		if d == nil {
			d = J{}
		}
		switch d.S("kind") {
		case "status":
			s.SetStatus(d.S("plugin"), d["status"])
		case "notify":
			msg := d.S("text")
			if p := s.ByID(d.S("plugin")); p != nil {
				msg = l10n.PluginContextLine(p["title"], d["text"])
			}
			if s.OnNotice != nil {
				s.OnNotice(msg)
			}
		}
	})
	s.Load(nil)
}

func (s *PluginStore) changed() {
	if s.OnChange != nil {
		s.OnChange()
	}
}

func (s *PluginStore) ByID(id string) J {
	for _, p := range s.Installed {
		if Str(p["id"]) == id {
			return p
		}
	}
	return nil
}

// Active is the plugins that are loaded and working.
func (s *PluginStore) Active() []J {
	var out []J
	for _, p := range s.Installed {
		if p.S("state") == "enabled" {
			out = append(out, p)
		}
	}
	return out
}

// Attention counts plugins waiting for consent or failing to load.
func (s *PluginStore) Attention() int {
	n := 0
	for _, p := range s.Installed {
		if st := p.S("state"); st == "needs_consent" || st == "error" {
			n++
		}
	}
	return n
}

func (s *PluginStore) SetStatus(id string, status any) {
	if p := s.ByID(id); p != nil {
		p["status"] = status
	}
}

// Load reloads the installed plugins.
func (s *PluginStore) Load(done func()) {
	api := s.app.Api()
	Async(func() (J, error) { return api.GetJ("/agent/plugins") }, func(r J, err error) {
		if err == nil {
			s.Installed = r.Os("plugins")
			s.AllowThirdParty = r["allow_third_party"] == true
			s.Error = nil
		} else {
			s.Error = err
		}
		s.Loaded = true
		s.changed()
		if done != nil {
			done()
		}
	})
}

// Entries is the contributions of kind (a key of contributes) of active
// plugins, on a surface and in a scope when they are not empty.
func (s *PluginStore) Entries(kind, surface, scope string) []PluginEntry {
	var out []PluginEntry
	for _, p := range s.Active() {
		for _, e := range p.O("contributes").Os(kind) {
			if (surface == "" || e.S("surface") == surface) && (scope == "" || e.S("scope") == scope) {
				out = append(out, NewPluginEntry(p, kind, e))
			}
		}
	}
	return out
}

// ToolPlugin is the plugin owning a prefixed tool name (<id>__<tool>).
func (s *PluginStore) ToolPlugin(name string) J {
	i := strings.Index(name, "__")
	if i <= 0 {
		return nil
	}
	return s.ByID(name[:i])
}

func (s *PluginStore) ToolLabel(name string) string {
	for _, t := range s.ToolPlugin(name).Os("tools") {
		if t.S("name") == name {
			return t.S("label")
		}
	}
	return ""
}

// RendersBlock is whether an active plugin renders coursive-view blocks of kind.
func (s *PluginStore) RendersBlock(kind string) bool {
	for _, p := range s.Active() {
		for _, b := range p.O("contributes").Ss("blocks") {
			if b == kind {
				return true
			}
		}
	}
	return false
}

func (s *PluginStore) Catalog() (J, error) { return s.app.Api().GetJ("/agent/plugins/catalog") }

// Preview fetches and validates a package; nothing installs until Confirm.
func (s *PluginStore) Preview(source string) (J, error) {
	v, err := s.app.Api().PostWait("/agent/plugins/install", J{"source": source}, 5*time.Minute)
	return Obj(v), err
}

// Confirm installs a previewed package; call Replace with the result on
// the UI thread.
func (s *PluginStore) Confirm(token string) (J, error) {
	v, err := s.app.Api().PostWait("/agent/plugins/install/confirm", J{"token": token}, 6*time.Minute)
	return Obj(v), err
}

// Update changes a plugin (enabling, consent, settings).
func (s *PluginStore) Update(id string, body J) (J, error) {
	v, err := s.app.Api().Put("/agent/plugins/"+id, body)
	return Obj(v), err
}

func (s *PluginStore) Uninstall(id string) error {
	_, err := s.app.Api().Delete("/agent/plugins/" + id)
	return err
}

// Removed drops a plugin after Uninstall, on the UI thread.
func (s *PluginStore) Removed(id string) {
	var out []J
	for _, p := range s.Installed {
		if Str(p["id"]) != id {
			out = append(out, p)
		}
	}
	s.Installed = out
	s.changed()
}

func (s *PluginStore) SetThirdParty(allow bool) (bool, error) {
	v, err := s.app.Api().Put("/agent/plugins/third-party", J{"allow": allow})
	if err != nil {
		return false, err
	}
	return Obj(v)["allow_third_party"] == true, nil
}

func (s *PluginStore) AddCatalog(url string) error {
	_, err := s.app.Api().Post("/agent/plugins/catalogs", J{"url": url})
	return err
}

func (s *PluginStore) RemoveCatalog(url string) error {
	_, err := s.app.Api().Post("/agent/plugins/catalogs/remove", J{"url": url})
	return err
}

// Run runs a contributed action; the result holds one of view, prompt,
// open or toast, and maybe a new state.
func (s *PluginStore) Run(e PluginEntry, context J) (J, error) {
	v, err := s.app.Api().PostWait("/agent/plugins/"+e.PluginID()+"/actions/"+e.Action, J{"context": context}, 75*time.Second)
	return Obj(v), err
}

// Replace puts an updated plugin in place, on the UI thread.
func (s *PluginStore) Replace(p J) {
	if p == nil {
		return
	}
	for i, x := range s.Installed {
		if Str(x["id"]) == Str(p["id"]) {
			s.Installed[i] = p
			s.changed()
			return
		}
	}
	s.Installed = append(s.Installed, p)
	s.changed()
}

// PluginCategory is pluginCategories.
func PluginCategory(id string) string {
	switch id {
	case "notes":
		return l10n.CommonNotes
	case "visualize":
		return l10n.PluginCategoryVisualize
	case "review":
		return l10n.PluginCategoryReview
	case "programming":
		return l10n.PluginCategoryProgramming
	case "connector":
		return l10n.PluginCategoryConnector
	case "tutor":
		return l10n.PluginCategoryTutor
	case "assignment":
		return l10n.CommonAssignments
	}
	return ""
}

// PluginCategories lists the category ids in pluginCategories' order.
var PluginCategories = []string{"notes", "visualize", "review", "programming", "connector", "tutor", "assignment"}

// SensitivePermissions reach outside the app and show with emphasis.
var SensitivePermissions = map[string]bool{"exec": true, "network": true, "folder": true, "localhost": true}
