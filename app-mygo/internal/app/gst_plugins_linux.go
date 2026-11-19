package app

import (
	"sync"

	"github.com/ebitengine/purego"
)

// gstNeeds are the elements lectures play through, by the plugin package
// that brings them, each met by any of its elements: the Flutter client
// brought its own decoders (libmpv), MyGo plays through the system's
// GStreamer.
var gstNeeds = []struct {
	pkg string
	any []string
}{
	{"gst-plugins-good", []string{"qtdemux"}},
	{"gst-plugins-good", []string{"matroskademux"}},
	{"gst-plugins-bad", []string{"hlsdemux", "hlsdemux2"}},
	{"gst-plugins-bad", []string{"tsdemux"}},
	{"gst-libav", []string{"avdec_h264", "vah264dec", "nvh264dec", "openh264dec"}},
	{"gst-libav", []string{"avdec_aac", "fdkaacdec", "faad"}},
}

// missingGstPackages names the packages of gstNeeds the system lacks. It
// asks GStreamer's registry once a video failed, when the player has
// loaded and initialised GStreamer; the registry is read as it starts, so a
// package installed later counts after a restart.
var missingGstPackages = sync.OnceValue(func() []string {
	lib, err := purego.Dlopen("libgstreamer-1.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil
	}
	var isInit func() bool
	var find func(name string) uintptr
	var unref func(obj uintptr)
	purego.RegisterLibFunc(&isInit, lib, "gst_is_initialized")
	purego.RegisterLibFunc(&find, lib, "gst_element_factory_find")
	purego.RegisterLibFunc(&unref, lib, "gst_object_unref")
	if !isInit() {
		return nil
	}
	var out []string
	for _, n := range gstNeeds {
		found := false
		for _, el := range n.any {
			if f := find(el); f != 0 {
				unref(f)
				found = true
				break
			}
		}
		if !found && (len(out) == 0 || out[len(out)-1] != n.pkg) {
			out = append(out, n.pkg)
		}
	}
	return out
})
