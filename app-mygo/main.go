// Command coursive-mygo is the Coursive client built with MyGo: the Flutter
// client's screens, design system and behaviour, ported one to one to Go.
//
// It starts the Go daemon as the Flutter client does, speaks its HTTP API
// with the token it started it with, and keeps the same shared preferences.
// See docs/specs/0033-mygo-client-migration.md.
package main

import (
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/app"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/icons"
	"github.com/walkerkiller/coursive/app-mygo/internal/widgets"
)

func main() {
	icons.Register()
	core.Prefs = core.OpenStore(core.DefaultStorePath())
	core.LoadUiPrefs()
	d := core.NewDaemon()
	app.Init(d)
	widgets.Headers = func(url string) map[string]string {
		if d.Port() != 0 && strings.HasPrefix(url, d.BaseURL()+"/") {
			return map[string]string{"Authorization": "Bearer " + d.Token()}
		}
		return nil
	}
	mygo.App.OnOpenURL(func(url string) { core.Post(func() { app.ReceiveLink(url) }) })
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     "Coursive",
			Width:     1320,
			Height:    860,
			MinWidth:  380,
			MinHeight: 560,
			Content:   ui.View(app.Root),
		})
		core.Post = win.Update
		app.ShowWindow = func() { win.Show(); win.Focus() }
		app.SetFullScreen = win.SetFullScreen
		d.OnChange = func() { core.Post(app.State.OnDaemon) }
		d.Start()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	core.Tracker.Flush()
	app.State.Dispose()
	d.Dispose()
}
