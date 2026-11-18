package app

import (
	"sort"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/core"
)

// IsoDate is a day as YYYY-MM-DD.
func IsoDate(d time.Time) string { return d.Format("2006-01-02") }

// DayOf is the midnight starting d.
func DayOf(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
}

// ScheduleStore holds every schedule item, kept in sync with the daemon's
// schedule_changed events.
type ScheduleStore struct {
	Items     []core.ScheduleItem
	Loaded    bool
	listening bool
	debounce  *time.Timer
}

func (s *ScheduleStore) watch() {
	if !State.Connected() || s.listening {
		return
	}
	s.listening = true
	State.Events.Listen(func(e core.J) {
		if t := e.S("type"); t == "schedule_changed" || t == "hello" {
			if s.debounce != nil {
				s.debounce.Stop()
			}
			s.debounce = time.AfterFunc(150*time.Millisecond, func() { core.Post(s.Load) })
		}
	})
	s.Load()
}

// Load reads the items again.
func (s *ScheduleStore) Load() {
	if !State.Connected() {
		return
	}
	core.Async(func() ([]core.J, error) {
		r, err := api().Get("/api/schedule")
		return core.Objs(r), err
	}, func(l []core.J, err error) {
		if err != nil {
			return
		}
		s.Items = s.Items[:0]
		for _, j := range l {
			s.Items = append(s.Items, core.ScheduleItem{J: j})
		}
		s.Loaded = true
	})
}

func (s *ScheduleStore) ByID(id string) (core.ScheduleItem, bool) {
	for _, i := range s.Items {
		if i.ID() == id {
			return i, true
		}
	}
	return core.ScheduleItem{}, false
}

func (s *ScheduleStore) ForLecture(id string) []core.ScheduleItem {
	var out []core.ScheduleItem
	for _, i := range s.Items {
		if i.LectureID() == id {
			out = append(out, i)
		}
	}
	return out
}

func (s *ScheduleStore) OnDay(d time.Time) []core.ScheduleItem {
	k := IsoDate(d)
	var out []core.ScheduleItem
	for _, i := range s.Items {
		if i.DueDate() == k {
			out = append(out, i)
		}
	}
	return out
}

// DueSoon counts open items overdue or due within two days.
func (s *ScheduleStore) DueSoon() int {
	limit := IsoDate(DayOf(time.Now()).AddDate(0, 0, 2))
	n := 0
	for _, i := range s.Items {
		if !i.Done() && i.DueDate() <= limit {
			n++
		}
	}
	return n
}

// Upcoming is what is open in the next days, and what is overdue.
func (s *ScheduleStore) Upcoming(days int) []core.ScheduleItem {
	today := IsoDate(DayOf(time.Now()))
	limit := IsoDate(DayOf(time.Now()).AddDate(0, 0, days))
	var out []core.ScheduleItem
	for _, i := range s.Items {
		if !i.Done() && i.DueDate() <= limit && (i.DueDate() >= today || i.Overdue()) {
			out = append(out, i)
		}
	}
	return out
}

func (s *ScheduleStore) replace(it core.ScheduleItem) {
	found := false
	for k, x := range s.Items {
		if x.ID() == it.ID() {
			s.Items[k], found = it, true
		}
	}
	if !found {
		s.Items = append(s.Items, it)
	}
	sort.SliceStable(s.Items, func(a, b int) bool {
		x, y := s.Items[a], s.Items[b]
		if x.DueDate() != y.DueDate() {
			return x.DueDate() < y.DueDate()
		}
		return x.DueTime() < y.DueTime()
	})
}

// Add creates an item.
func (s *ScheduleStore) Add(fields core.J, done func(core.ScheduleItem, error)) {
	core.Async(func() (core.J, error) {
		r, err := api().Post("/api/schedule", fields)
		return core.Obj(r), err
	}, func(r core.J, err error) {
		var it core.ScheduleItem
		if err == nil {
			it = core.ScheduleItem{J: r.O("item")}
			s.replace(it)
		}
		if done != nil {
			done(it, err)
		}
	})
}

// Patch changes an item, at once and then as the daemon has it.
func (s *ScheduleStore) Patch(it core.ScheduleItem, fields core.J, done func(error)) {
	next := it.J.Copy()
	for k, v := range fields {
		next[k] = v
	}
	s.replace(core.ScheduleItem{J: next})
	core.Async(func() (core.J, error) {
		r, err := api().Patch("/api/schedule/"+core.Esc(it.ID()), fields)
		return core.Obj(r), err
	}, func(r core.J, err error) {
		if err != nil {
			s.replace(it)
		} else {
			s.replace(core.ScheduleItem{J: r})
		}
		if done != nil {
			done(err)
		}
	})
}

// Remove deletes an item.
func (s *ScheduleStore) Remove(it core.ScheduleItem, done func(error)) {
	out := s.Items[:0]
	for _, x := range s.Items {
		if x.ID() != it.ID() {
			out = append(out, x)
		}
	}
	s.Items = out
	core.Go(func() error { _, err := api().Delete("/api/schedule/" + core.Esc(it.ID())); return err }, func(err error) {
		if err != nil {
			s.replace(it)
		}
		if done != nil {
			done(err)
		}
	})
}

// Restore puts back an item removed by mistake (same fields, new id); a
// learning-platform assignment is only hidden, so it shows again.
func (s *ScheduleStore) Restore(it core.ScheduleItem, done func(error)) {
	if !it.IsLms() {
		f := it.J.Copy()
		delete(f, "id")
		s.Add(f, func(_ core.ScheduleItem, err error) {
			if done != nil {
				done(err)
			}
		})
		return
	}
	core.Async(func() (core.J, error) {
		r, err := api().Patch("/api/schedule/"+core.Esc(it.ID()), core.J{"hidden": false})
		return core.Obj(r), err
	}, func(r core.J, err error) {
		if err == nil {
			s.replace(core.ScheduleItem{J: r})
		}
		if done != nil {
			done(err)
		}
	})
}

// ScheduleFocus is the day the schedule page should show, set by pages
// that open it.
type ScheduleFocus struct{ Day *time.Time }
