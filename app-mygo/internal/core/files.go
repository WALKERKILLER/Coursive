package core

import (
	"github.com/egoist/mygo"
)

// FileFilter limits what a file dialog shows.
type FileFilter = mygo.FileFilter

// ImageFilter is the image files the app opens.
var ImageFilter = FileFilter{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "webp", "gif", "bmp"}, MIMETypes: []string{"image/*"}}

// PickFiles shows the system's open dialog off the UI thread and hands the
// chosen paths, none when canceled, to done on it.
func PickFiles(title string, multiple, dir bool, filters []FileFilter, done func([]string)) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: title, Filters: filters, Multiple: multiple, Directory: dir, CreateDirectories: dir})
		if err != nil {
			paths = nil
		}
		Post(func() { done(paths) })
	}()
}

// PickFile is PickFiles for one file; done gets "" when canceled.
func PickFile(title string, filters []FileFilter, done func(string)) {
	PickFiles(title, false, false, filters, func(p []string) {
		if len(p) == 0 {
			done("")
			return
		}
		done(p[0])
	})
}

// PickDir chooses a directory; done gets "" when canceled.
func PickDir(title string, done func(string)) {
	PickFiles(title, false, true, nil, func(p []string) {
		if len(p) == 0 {
			done("")
			return
		}
		done(p[0])
	})
}

// SaveFile asks where to save a file; done gets "" when canceled.
func SaveFile(title, name string, filters []FileFilter, done func(string)) {
	go func() {
		p, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Title: title, DefaultPath: name, Filters: filters, CreateDirectories: true})
		if err != nil {
			p = ""
		}
		Post(func() { done(p) })
	}()
}
