package app

import (
	"runtime"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/design"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// ShowMediaToolsHelp is showMediaToolsHelp. Installation is explicit: this
// dialog never runs a package manager.
func ShowMediaToolsHelp(pdf bool) {
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		mac, linux := runtime.GOOS == "darwin", runtime.GOOS == "linux"
		m3.AlertDialog(c, m3.Alert{Title: l10n.SetupInstallHelp, Width: 520, Scrollable: true, Content: func() {
			ui.Column(c).FillWidth().Children(func() {
				m3.Body(c, l10n.SetupInstallIntro)
				m3.VGap(c, design.SpaceMD)
				how := l10n.SetupOtherInstall
				switch {
				case mac && pdf:
					how = l10n.SetupPdfHelperMissing
				case mac:
					how = l10n.SetupMacInstall
				case runtime.GOOS == "windows":
					how = l10n.SetupWindowsInstall
				case linux:
					how = l10n.SetupLinuxInstall
				}
				m3.Body(c, how)
				m3.VGap(c, design.SpaceMD)
				switch {
				case mac && !pdf:
					m3.Body(c, "brew install ffmpeg").Selectable()
				case linux && pdf:
					m3.Body(c, "sudo apt install poppler-utils").Selectable()
				case linux:
					m3.Body(c, "sudo apt install ffmpeg").Selectable()
				}
				m3.VGap(c, design.SpaceMD)
				links := "https://ffmpeg.org/download.html\nhttps://poppler.freedesktop.org/\nhttps://brew.sh/"
				if pdf && mac {
					links = "https://github.com/WALKERKILLER/Coursive/releases"
				}
				m3.Body(c, links).Selectable()
			})
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonClose).Clicked() {
				r.Close()
			}
		}})
	}, nil)
}
