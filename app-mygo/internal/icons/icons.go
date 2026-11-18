// Package icons is the Flutter client's Material icons: Flutter's own
// MaterialIcons font, cut to the glyphs the client uses, and their
// codepoints from Flutter's icons.dart. A view draws an icon as Flutter's
// Icon does, a glyph of that font in a square of its size.
package icons

//go:generate go run gen.go

import (
	_ "embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

// Icon is a codepoint of the MaterialIcons font.
type Icon rune

// Family is the font family the glyphs are registered under.
const Family = "MaterialIcons"

//go:embed MaterialIcons-Subset.otf
var font []byte

var once sync.Once

// Register adds the font to the window's text; call it before the first frame.
func Register() {
	once.Do(func() {
		if err := ui.RegisterFont(font, Family); err != nil {
			panic("icons: " + err.Error())
		}
	})
}

// String is the glyph's text.
func (i Icon) String() string { return string(rune(i)) }

// Mirrored reports whether Flutter flips the icon in right-to-left text.
func (i Icon) Mirrored() bool { return mirrored[i] }
