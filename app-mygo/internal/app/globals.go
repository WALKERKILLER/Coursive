// Package app is the Coursive client's features, ported one to one from the
// Flutter client's lib/features, with the shell and the root gate.
package app

import (
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
)

// The app's long-lived state, which the Flutter client provides through
// Provider at its root (main.dart).
var (
	State     *core.AppState
	Profile   *core.UserProfile
	Plugins   *core.PluginStore
	Screen    = core.NewScreenContext()
	Players   = &core.PlayerHub{}
	Assistant *AssistantController
	Schedule  *ScheduleStore
	Inbox     *TaskInbox
	Focus     = &ScheduleFocus{}
)

// Init builds the state around a started daemon.
func Init(d *core.Daemon) {
	State = core.NewAppState(d)
	Profile = core.LoadProfile()
	core.Tracker = core.NewActivityTracker(State)
	Plugins = core.NewPluginStore(State)
	Assistant = NewAssistantController()
	Schedule = &ScheduleStore{}
	Inbox = &TaskInbox{seenAt: -1}
}

// Tick lets the stores start once the daemon answers, as their listeners on
// AppState do in Flutter. The root calls it every frame.
func Tick() {
	Plugins.Watch()
	Assistant.watch()
	Schedule.watch()
	Inbox.watch()
}
