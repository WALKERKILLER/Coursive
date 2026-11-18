package core

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// CanOpenFolders is whether folders can be shown in a file manager.
func CanOpenFolders() bool { return runtime.GOOS != "android" && runtime.GOOS != "ios" }

// OpenPath opens a file or folder with the system's default handler.
func OpenPath(path string) bool {
	if u, err := url.Parse(path); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		return openWith(path)
	}
	return openWith(filepath.Clean(path))
}

func openWith(target string) bool {
	switch runtime.GOOS {
	case "windows":
		// explorer.exe exits with 1 even when it opened the path.
		_ = exec.Command("explorer", target).Start()
		_, err := os.Stat(target)
		return err == nil
	case "darwin":
		return exec.Command("open", target).Run() == nil
	default:
		return exec.Command("xdg-open", target).Run() == nil
	}
}

// RevealPath shows a file selected in the system file manager, falling
// back to opening its folder.
func RevealPath(path string) bool {
	path = filepath.Clean(path)
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Run() == nil
	case "windows":
		_ = exec.Command("explorer", "/select,", path).Start()
		_, err := os.Stat(path)
		return err == nil
	case "linux":
		u := (&url.URL{Scheme: "file", Path: path}).String()
		if exec.Command("dbus-send", "--session", "--print-reply", "--dest=org.freedesktop.FileManager1",
			"--type=method_call", "/org/freedesktop/FileManager1", "org.freedesktop.FileManager1.ShowItems",
			"array:string:"+u, "string:").Run() == nil {
			return true
		}
	}
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		return OpenPath(path)
	}
	return OpenPath(filepath.Dir(path))
}
