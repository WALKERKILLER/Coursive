package server

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMediaToolsReportsPartialInstallation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	for name, found := range mediaTools() {
		if found {
			t.Fatalf("empty PATH directory reports %s available", name)
		}
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
		t.Setenv("PATHEXT", ".EXE")
	}
	for _, name := range []string{"ffmpeg", "pdfinfo", "pdftoppm"} {
		if err := os.WriteFile(filepath.Join(dir, name+suffix), []byte("test fixture"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	tools := mediaTools()
	if !tools["ffmpeg"] || !tools["pdfinfo"] || !tools["pdftoppm"] || tools["ffprobe"] || tools["pdftotext"] {
		t.Fatalf("partial installation: %#v", tools)
	}
}
