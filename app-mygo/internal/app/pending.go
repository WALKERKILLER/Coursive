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

func NewPlayerPage(id string, t int) m3.Page                                { return &todoPage{"player " + id} }
func NewTasksPage() m3.Page                                                 { return &todoPage{"tasks"} }
func NewPluginsPage() m3.Page                                               { return &todoPage{"plugins"} }
func NewSettingsPage() m3.Page                                              { return &todoPage{"settings"} }
func RunBotOp(op core.J)                                                    {}
func OpenAssistantPanel(c *ui.Context)                                      {}
func AssistantPanelView(c *ui.Context, d bool)                              {}
func runStatus(s string) (string, ui.Color, bool)                           { return s, ui.Color{}, s == "failed" }
func openRunConversation(session string)                                    {}
func SeekOrOpen(id string, t int)                                           { m3.RootNav.Push(NewPlayerPage(id, t)) }
func workspaceSetup(c *ui.Context)                                          { m3.Text(c, "setup", design.TT().TitleLarge) }
func OpenPlugins()                                                          {}
func PluginMenuButton(c *ui.Context, surface string, ctx map[string]string) {}
func OpenLink(u string)                                                     {}
func OpenSettingsSection(id string)                                         {}
func UiSlotOutlet(c *ui.Context, target string, ctx map[string]string)      {}
