package app

import (
	"maps"
	"sort"

	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// WindowWidth and WindowHeight are the window's size in the frame being
// built (MediaQuery.sizeOf).
var WindowWidth, WindowHeight float32

// Compact is context.isCompact: the window is in the compact size class.
func Compact() bool { return design.SizeClassOf(WindowWidth) == design.SizeCompact }

// ReportScreen is report_screen.dart's: it tells the assistant what the
// user is looking at. Only the visible page of the selected tab is built,
// so a page calls it from its View.
func ReportScreen(v map[string]string) {
	if !maps.Equal(Screen.Value, v) {
		Screen.Set(v)
	}
}

func sortStrings(s []string) { sort.Strings(s) }
