// Package assets holds the Flutter client's app/assets that the MyGo
// client shows: brand and provider marks, and the reward code. The fonts
// stay with the PDF export that embeds them.
package assets

import (
	"embed"
	"io/fs"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed brands providers reward.png
var files embed.FS

var (
	mu     sync.Mutex
	images = map[string]ui.ImageSource{}
)

// Bytes returns an asset's bytes by its path under app/assets.
func Bytes(name string) ([]byte, error) { return fs.ReadFile(files, name) }

// Image returns an asset as an image, decoded once: a bitmap, or an SVG.
// nil when it is missing or does not decode.
func Image(name string) ui.ImageSource {
	mu.Lock()
	defer mu.Unlock()
	if img, ok := images[name]; ok {
		return img
	}
	var img ui.ImageSource
	if b, err := Bytes(name); err == nil {
		if strings.HasSuffix(name, ".svg") {
			if s, err := ui.ParseSVG(b); err == nil {
				img = s
			}
		} else if bmp, err := ui.DecodeBitmap(b); err == nil {
			img = bmp
		}
	}
	images[name] = img
	return img
}
