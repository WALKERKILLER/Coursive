//go:build !linux

package app

// missingGstPackages is GStreamer's on Linux; elsewhere the system's own
// media framework plays the video.
func missingGstPackages() []string { return nil }
