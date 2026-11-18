//go:build e2e

package main

// Built with -tags e2e, the client listens on the socket MYGO_AUTOMATION
// names, for driving and capturing its screens from outside (see
// scripts/mygo-drive.py). Release builds leave it out.
import _ "github.com/egoist/mygo/plugins/automation"
