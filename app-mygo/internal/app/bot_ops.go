package app

import (
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
	"github.com/walkerkiller/coursive/app-mygo/internal/m3"
)

// bot_ops.dart: the app side of the bot operating the app (details.app of
// its tools, docs/specs/0007-resident-bot-and-course-agents.md):
// navigation, the open player and preferences.

// OpenTargets are kOpenTargets: the pages the bot can open, by name.
var OpenTargets = map[string]string{
	"home": l10n.ScreenHome, "courses": l10n.CommonCourses, "course": l10n.CommonCourses,
	"lecture": l10n.CommonLecture, "note": l10n.CommonNotes, "schedule": l10n.CommonSchedule,
	"downloads": l10n.CommonDownload, "library": l10n.ScreenLibrary, "sources": l10n.SourcesTitle,
	"lms": l10n.LmsTitle, "tasks": l10n.CommonScheduledTasks, "plugins": l10n.CommonPlugins,
	"settings": l10n.CommonSettings, "activity": l10n.CommonActivity, "usage": l10n.CommonUsage, "bot": "Bot",
}

var tabTargets = map[string]int{
	"home": TabHome, "courses": TabCourses, "schedule": TabSchedule, "downloads": TabDownloads,
	"library": TabLibrary, "tasks": TabTasks, "plugins": TabPlugins, "settings": TabSettings,
}

// AppOpLabel is a short label for an operation's chip.
func AppOpLabel(op core.J) string {
	switch op.S("op") {
	case "open":
		t := OpenTargets[op.S("target")]
		if t == "" {
			t = op.S("target")
		}
		return l10n.BotOpened(t)
	case "player":
		switch op.S("action") {
		case "play":
			return l10n.BotResumed
		case "pause":
			return l10n.BotPaused
		case "seek":
			return l10n.BotSeeked
		}
		return l10n.BotRateSet(op["rate"])
	}
	return op.Or("label", l10n.CommonDone)
}

// botOps remembers where the user was before the bot opened a page, for
// 返回.
var botOps struct {
	backTab   int
	hasBack   bool
	pushedTab int
	pushed    bool
	rootPage  m3.Page
	lastOpen  string
}

// CanGoBackFrom tells whether op opened the page BotBack would leave
// (older chips cannot).
func CanGoBackFrom(op core.J) bool { return botOps.hasBack && core.Encode(op) == botOps.lastOpen }

// RunBotOp runs an operation the bot asked for.
func RunBotOp(op core.J) {
	switch op.S("op") {
	case "open":
		botOpen(op)
	case "player":
		h, ok := Players.Value.(*LecturePlayer)
		if !ok || h == nil {
			return
		}
		switch op.S("action") {
		case "play":
			if !h.Playing() {
				h.PlayOrPause()
			}
		case "pause":
			if h.Playing() {
				h.PlayOrPause()
			}
		case "seek":
			h.SeekTo(int(op.F("seconds")+.5) * 1000)
		case "speed":
			r := op.F("rate")
			if r == 0 {
				r = 1
			}
			h.SetRate(r)
		}
	case "prefs":
		if set := op.O("set"); len(set) > 0 {
			core.UI.ApplyJSON(set)
		}
	}
}

func botOpen(op core.J) {
	target := op.S("target")
	botOps.lastOpen = core.Encode(op)
	botOps.backTab, botOps.hasBack = CurrentTab(), true
	botOps.pushed, botOps.rootPage = false, nil
	tab, isTab := tabTargets[target]
	// Tabs sit under full-screen routes (the compact assistant, a sheet,
	// the player): close them so the page shows, as the chat's buttons do.
	if isTab || target == "course" || target == "note" || target == "sources" || target == "lms" {
		m3.CloseAll()
		m3.RootNav.PopUntilRoot()
	}
	if isTab {
		GoTo(tab)
		if day, err := time.ParseInLocation("2006-01-02", op.S("day"), time.Local); target == "schedule" && err == nil {
			Focus.Day = &day
		}
		if s := op.S("section"); target == "settings" && s != "" {
			OpenSettingsSection(s)
		}
		return
	}
	pushTab := func(t int, page m3.Page) {
		PushTab(t, page)
		botOps.pushedTab, botOps.pushed = t, true
	}
	pushRoot := func(page m3.Page) {
		m3.RootNav.Push(page)
		botOps.rootPage = page
	}
	switch target {
	case "course":
		pushTab(TabCourses, NewCoursePage(op.S("course_id")))
	case "note":
		pushTab(TabLibrary, NewNotePage(op.S("note_id")))
	case "sources":
		pushTab(TabCourses, NewSourcesPage())
	case "lms":
		pushTab(TabCourses, NewLmsPage())
	case "lecture":
		start := -1
		if _, ok := op["t"].(float64); ok {
			start = int(op.F("t")+.5) * 1000
		}
		pushRoot(NewPlayerPage(op.S("lecture_id"), start))
	case "activity":
		pushRoot(NewActivityPage())
	case "usage":
		pushRoot(NewUsagePage())
	case "bot":
		pushRoot(NewBotPage())
	}
}

// BotBack leaves the page the bot opened and returns to the tab the user
// was on.
func BotBack() {
	if !botOps.hasBack {
		return
	}
	if botOps.rootPage != nil {
		m3.RootNav.PopPage(botOps.rootPage)
	}
	if botOps.pushed {
		PopTab(botOps.pushedTab)
	}
	if botOps.backTab != CurrentTab() {
		GoTo(botOps.backTab)
	}
	botOps.hasBack, botOps.pushed, botOps.rootPage, botOps.lastOpen = false, false, nil, ""
}
