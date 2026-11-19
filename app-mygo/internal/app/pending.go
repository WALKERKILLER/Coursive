package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// workspaceSetup stands for workspace_setup_page.dart, Android's first run,
// which installs the Linux workspace the daemon runs in. The desktop never
// reaches it (DaemonSetup); it is ported with the Android build.
func workspaceSetup(c *ui.Context) { m3.Text(c, "setup", design.TT().TitleLarge) }
