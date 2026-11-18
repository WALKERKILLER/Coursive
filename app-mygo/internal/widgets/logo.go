package widgets

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

//go:embed assets/logo.png
var logoPNG []byte

var logo *ui.Bitmap

// Logo is CoursiveLogo: the launcher icon, a rounded tile from the brand
// master, at size.
func Logo(c *ui.Context, size float32) ui.Element {
	if logo == nil {
		logo, _ = ui.DecodeBitmap(logoPNG)
	}
	return ui.Image(c, logo).Size(size, size).Shrink(0).Label("Coursive")
}
