package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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

// ObsidianPage is the one-way export of the library into an Obsidian
// vault (POST /api/library/obsidian, settings obsidian.*).
// Contract: docs/specs/0012-obsidian-export.md
type ObsidianPage struct {
	settings      *ServerSettings
	vault, folder string
	filled, busy  bool
	scroll        ui.ScrollState
}

// NewObsidianPage opens the Obsidian page.
func NewObsidianPage(s *ServerSettings) m3.Page { return &ObsidianPage{settings: s} }

// MainScroll is the page's scroll.
func (p *ObsidianPage) MainScroll() *ui.ScrollState { return &p.scroll }

// fill takes the saved values once they arrive, without clobbering
// typing.
func (p *ObsidianPage) fill() {
	if p.filled || p.settings.Values == nil {
		return
	}
	p.filled = true
	if p.vault == "" {
		p.vault = p.settings.Values.S("obsidian.vault")
	}
	if p.folder == "" {
		p.folder = p.settings.Values.S("obsidian.folder")
	}
}

// save saves vault and folder in one request.
func (p *ObsidianPage) save(vault, folder string, done func(error)) {
	core.Go(func() error {
		_, err := api().Put("/api/settings", core.J{"obsidian.vault": vault, "obsidian.folder": folder})
		return err
	}, func(err error) {
		if err == nil {
			p.settings.Set("obsidian.vault", vault)
			p.settings.Set("obsidian.folder", folder)
		}
		done(err)
	})
}

func (p *ObsidianPage) saveAndTell(quiet bool) {
	p.save(strings.TrimSpace(p.vault), strings.TrimSpace(p.folder), func(err error) {
		switch {
		case err != nil:
			widgets.ShowSnack(l10n.CommonSaveFailed(apiMessage(err)), "", nil)
		case !quiet:
			widgets.ShowSnack(l10n.CommonSaved, "", nil)
		}
	})
}

func (p *ObsidianPage) export() {
	vault, folder := strings.TrimSpace(p.vault), strings.TrimSpace(p.folder)
	if vault == "" {
		widgets.ShowSnack(l10n.ObsidianChooseVaultFirst, "", nil)
		return
	}
	p.busy = true
	fail := func(err error) {
		p.busy = false
		widgets.ShowSnack(l10n.CommonExportFailed(apiMessage(err)), "", nil)
	}
	p.save(vault, folder, func(err error) {
		if err != nil {
			fail(err)
			return
		}
		// Export exactly what is on screen, not whatever was saved before.
		core.Async(func() (any, error) {
			return api().PostWait("/api/library/obsidian", core.J{"vault": vault, "folder": folder}, 5*time.Minute)
		}, func(v any, err error) {
			if err != nil {
				fail(err)
				return
			}
			p.busy = false
			r := core.Obj(v)
			parts := []string{l10n.ObsidianExported(r["written"], r["unchanged"], r["removed"])}
			if n := len(r.L("conflicts")); n > 0 {
				parts = append(parts, l10n.ObsidianConflicts(n))
			}
			if !r.IsTrue("vault") {
				parts = append(parts, l10n.ObsidianNotVault)
			}
			widgets.ShowSnack(strings.Join(parts, l10n.ListSemicolon), "", nil)
		})
	})
}

// registerLinks makes coursive-app:// links open this app: a user desktop
// entry that handles the scheme, set as the default with xdg-mime.
func registerLinks() {
	const name = "io.github.walkerkiller.coursive-handler.desktop"
	type result struct {
		ok     bool
		stderr string
	}
	core.Async(func() (result, error) {
		dir := filepath.Join(os.Getenv("HOME"), ".local", "share", "applications")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return result{}, err
		}
		exe, err := os.Executable()
		if err != nil {
			return result{}, err
		}
		exe = regexp.MustCompile("[\"`$\\\\]").ReplaceAllStringFunc(exe, func(m string) string { return "\\" + m })
		entry := "[Desktop Entry]\nType=Application\nName=Coursive\nExec=\"" + exe + "\" %u\nNoDisplay=true\nMimeType=x-scheme-handler/coursive-app;\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(entry), 0o644); err != nil {
			return result{}, err
		}
		var stderr strings.Builder
		cmd := exec.Command("xdg-mime", "default", name, "x-scheme-handler/coursive-app")
		cmd.Stderr = &stderr
		runErr := cmd.Run()
		if _, ok := runErr.(*exec.Error); ok {
			return result{}, runErr
		}
		// Optional; xdg-mime alone is enough on most desktops.
		_ = exec.Command("update-desktop-database", dir).Run()
		return result{runErr == nil, stderr.String()}, nil
	}, func(r result, err error) {
		switch {
		case err != nil:
			widgets.ShowSnack(l10n.LinkRegisterFailed(err.Error()), "", nil)
		case r.ok:
			widgets.ShowSnack(l10n.LinkRegistered, "", nil)
		default:
			widgets.ShowSnack(l10n.LinkRegisterPartial(name, r.stderr), "", nil)
		}
	})
}

// View builds the page.
func (p *ObsidianPage) View(c *ui.Context) {
	p.fill()
	cs := design.S()
	tt := design.TT()
	v := p.settings.Values
	vault := v.S("obsidian.vault")
	auto := v.IsTrue("obsidian.auto")
	SettingsSubpage(c, "Obsidian", &p.scroll, func() {
		status := l10n.ObsidianNoVault
		if vault != "" {
			suffix := ""
			if auto {
				suffix = l10n.CommonAutoSuffix
			}
			status = l10n.ObsidianStatus(filepath.Base(vault), suffix)
		}
		IntegrationHeader(c, func() { BrandMark(c, "obsidian.svg", 56, false) }, "Obsidian", status, l10n.ObsidianBody, vault != "")
		SettingsGroup(c, l10n.CommonLocation, 0, nil, func() {
			m3.VGap(c, design.SpaceSM)
			f := (m3.Field{Label: l10n.ObsidianVault, Hint: "/home/me/Notes", SuffixFn: func() {
				if m3.IconBtn(c, icons.FolderOpenRounded, l10n.PickerChooseFolder).Clicked() {
					widgets.PickDirectory(strings.TrimSpace(p.vault), "", l10n.ObsidianVault, func(dir string) {
						if dir != "" {
							p.vault = dir
							p.saveAndTell(false)
						}
					})
				}
			}}).Show(c.Key("vault"), &p.vault)
			if f.Submitted() {
				p.saveAndTell(true)
			}
			m3.VGap(c, design.SpaceMD)
			if (m3.Field{Label: l10n.ObsidianFolder, Hint: "Coursive", Helper: l10n.ObsidianFolderHelper}).Show(c.Key("folder"), &p.folder).Submitted() {
				p.saveAndTell(true)
			}
			m3.VGap(c, design.SpaceSM)
			ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
				if m3.TextButton(c, l10n.ObsidianSaveLocation).Clicked() {
					p.saveAndTell(false)
				}
			})
		})
		SettingsGroup(c, l10n.CommonExport, 0, nil, func() {
			if ch, on := switchTile(c, "auto", m3.Tile{Title: l10n.ObsidianAutoExport, Subtitle: l10n.ObsidianAutoExportHint}, auto); ch {
				p.settings.Put("obsidian.auto", on)
			}
			m3.VGap(c, design.SpaceSM)
			ui.Row(c).FillWidth().Children(func() {
				label := l10n.CommonExportNow
				if p.busy {
					label = l10n.CommonExporting
				}
				if (m3.Button{Kind: m3.Filled, Icon: icons.IosShareRounded, Label: label, Disabled: p.busy, Busy: p.busy}).Show(c).Clicked() {
					p.export()
				}
			})
		})
		if runtime.GOOS == "linux" {
			SettingsGroup(c, l10n.ObsidianBackLinks, 0, nil, func() {
				m3.Text(c, l10n.ObsidianBackLinksBody, tt.BodyMedium.C(cs.OnSurfaceVariant))
				m3.VGap(c, design.SpaceMD)
				ui.Row(c).FillWidth().Children(func() {
					if (m3.Button{Kind: m3.Outlined, Icon: icons.LinkRounded, Label: l10n.LinkRegister}).Show(c).Clicked() {
						registerLinks()
					}
				})
			})
		}
	})
}
