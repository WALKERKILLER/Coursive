package app

import (
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
)

// TaskInbox counts finished task runs not yet seen on the 定时 tab, for
// the rail badge. Only the newest seen finish time is kept on this device.
type TaskInbox struct {
	Unread int
	// LastVisit: runs that finished after it were new when the tab was last
	// opened.
	LastVisit int
	seenAt    int
	newest    int
	viewing   bool
	listening bool
}

const inboxKey = "tasks.seenAt"

func (t *TaskInbox) watch() {
	if t.listening || !State.AgentReady() {
		return
	}
	t.listening = true
	State.Events.Listen(func(e core.J) {
		if e.S("type") == "agent_task" && e.O("data").S("phase") == "finished" {
			t.Load()
		}
	})
	t.Load()
}

// Load reads the runs again.
func (t *TaskInbox) Load() {
	core.Async(func() ([]core.J, error) {
		r, err := api().Get("/agent/tasks/runs", map[string]string{"limit": "200"})
		return core.Objs(r), err
	}, func(runs []core.J, err error) {
		if err != nil {
			return
		}
		// Aborted runs were stopped by the user, so they are not news.
		var finished []int
		for _, r := range runs {
			if r.S("status") != "aborted" {
				if _, ok := r["finished"].(float64); ok {
					finished = append(finished, r.I("finished"))
				}
			}
		}
		t.newest = 0
		for _, f := range finished {
			t.newest = max(t.newest, f)
		}
		if t.seenAt < 0 {
			// The first time, history from before this version counts as seen.
			t.seenAt = t.newest
			if v, ok := core.Prefs.Int(inboxKey); ok {
				t.seenAt = v
			}
			t.LastVisit = t.seenAt
		}
		if t.viewing {
			t.markSeen()
		} else {
			t.Unread = 0
			for _, f := range finished {
				if f > t.seenAt {
					t.Unread++
				}
			}
		}
	})
}

// SetViewing is set by the shell while the 定时 tab is on screen.
func (t *TaskInbox) SetViewing(v bool) {
	if v == t.viewing {
		return
	}
	t.viewing = v
	if !v {
		return
	}
	if t.seenAt >= 0 {
		t.LastVisit = t.seenAt
	}
	t.markSeen()
}

func (t *TaskInbox) markSeen() {
	t.Unread = 0
	if t.newest <= t.seenAt {
		return
	}
	t.seenAt = t.newest
	core.Prefs.SetInt(inboxKey, t.seenAt)
}
