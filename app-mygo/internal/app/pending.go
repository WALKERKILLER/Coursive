package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// Pages and flows not yet ported; each is replaced by its port.

type todoPage struct{ name string }

func (p *todoPage) View(c *ui.Context) { m3.Text(c, p.name, design.TT().TitleLarge) }

func NewPluginsPage() m3.Page                                               { return &todoPage{"plugins"} }
func NewSettingsPage() m3.Page                                              { return &todoPage{"settings"} }
func workspaceSetup(c *ui.Context)                                          { m3.Text(c, "setup", design.TT().TitleLarge) }
func OpenPlugins(installed bool)                                            {}
func PluginMenuButton(c *ui.Context, surface string, ctx map[string]string) {}
func OpenLink(u string)                                                     {}
func OpenSettingsSection(id string)                                         {}
func UiSlotOutlet(c *ui.Context, target string, ctx map[string]string)      {}
func NewUsagePage() m3.Page                                                 { return &todoPage{"usage"} }
func NewModelsPage() m3.Page                                                { return &todoPage{"models"} }
func ShowModelPicker()                                                      {}
func runPluginEntry(e core.PluginEntry, ctx core.J, onPrompt func(string))  {}
func PluginView(c *ui.Context, view core.J)                                 {}
