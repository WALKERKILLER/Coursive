package widgets

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// PickDirectory is pickDirectory: asks for a folder with the system's own
// picker (the GTK or portal dialog on Linux, the shell dialog on Windows,
// the open panel on macOS); where there is none or it fails to open, it
// asks for a path instead. done gets "" when cancelled.
func PickDirectory(initial, confirm, title string, done func(string)) {
	if confirm == "" {
		confirm = l10n.CommonChoose
	}
	if title == "" {
		title = l10n.PickerChooseFolder
	}
	start := ""
	if st, err := os.Stat(initial); initial != "" && err == nil && st.IsDir() {
		start = initial
	}
	// Android's picker answers with content:// addresses, not paths the
	// daemon could use.
	if runtime.GOOS == "android" {
		askPath(initial, confirm, title, done)
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: title, DefaultPath: start, ButtonLabel: confirm, Directory: true, CreateDirectories: true})
		core.Post(func() {
			switch {
			case err != nil:
				askPath(initial, confirm, title, done)
			case len(paths) == 0:
				done("")
			default:
				done(paths[0])
			}
		})
	}()
}

// PickSavePath is pickSavePath: asks where to save a file with the system's
// save dialog; where there is none, it asks for a folder and keeps name.
// The path ends in one of filter's extensions; done gets "" when cancelled.
func PickSavePath(name string, filter core.FileFilter, confirm string, done func(string)) {
	if confirm == "" {
		confirm = l10n.CommonSave
	}
	finish := func(path string) {
		if path == "" {
			done("")
			return
		}
		exts := filter.Extensions
		ok := len(exts) == 0
		for _, e := range exts {
			ok = ok || strings.HasSuffix(strings.ToLower(path), "."+strings.ToLower(e))
		}
		if !ok {
			path += "." + exts[0]
		}
		done(path)
	}
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{DefaultPath: name, ButtonLabel: confirm, Filters: []core.FileFilter{filter}, CreateDirectories: true})
		core.Post(func() {
			if err == nil {
				finish(path)
				return
			}
			PickDirectory("", confirm, l10n.PickerSaveToFolder, func(dir string) {
				if dir == "" {
					done("")
					return
				}
				finish(filepath.Join(dir, name))
			})
		})
	}()
}

func askPath(initial, confirm, title string, done func(string)) {
	v := initial
	m3.ShowDialog(func(c *ui.Context, r *m3.Route) {
		m3.AlertDialog(c, m3.Alert{Title: title, Width: 440, Content: func() {
			if (m3.Field{Label: l10n.PickerFolderPath, AutoFocus: true}).Show(c, &v).Submitted() {
				r.Pop(strings.TrimSpace(v))
			}
		}, Actions: func() {
			if m3.TextButton(c, l10n.CommonCancel).Clicked() {
				r.Close()
			}
			if m3.FilledButton(c, confirm).Clicked() {
				r.Pop(strings.TrimSpace(v))
			}
		}})
	}, func(res any) {
		s, _ := res.(string)
		done(s)
	})
}

// FolderField is a path text field with a button that opens PickDirectory.
type FolderField struct {
	Label, Hint, Helper string
	AutoFocus           bool
	// OnPicked runs with a folder picked with the button.
	OnPicked func(string)
}

// Show builds the field; it reports a submit.
func (f FolderField) Show(c *ui.Context, value *string) bool {
	e := m3.Field{Label: f.Label, Hint: f.Hint, Helper: f.Helper, AutoFocus: f.AutoFocus, SuffixFn: func() {
		ui.Box(c).Padding(0, 4, 0, 0).Children(func() {
			if m3.IconBtn(c, icons.FolderOpenRounded, l10n.PickerChooseFolder).Clicked() {
				PickDirectory(strings.TrimSpace(*value), "", f.Label, func(dir string) {
					if dir == "" {
						return
					}
					*value = dir
					if f.OnPicked != nil {
						f.OnPicked(dir)
					}
				})
			}
		})
	}}.Show(c, value)
	return e.Submitted()
}
