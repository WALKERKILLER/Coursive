package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// Pages and flows not yet ported; each is replaced by its port.

type todoPage struct{ name string }

func (p *todoPage) View(c *ui.Context) { m3.Text(c, p.name, design.TT().TitleLarge) }

func workspaceSetup(c *ui.Context) { m3.Text(c, "setup", design.TT().TitleLarge) }
