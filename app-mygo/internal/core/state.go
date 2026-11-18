package core

import (
	"context"
	"log"
	"runtime"
	"sort"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// AppState is the global app state the daemon's event stream feeds, read
// and changed on the UI thread.
type AppState struct {
	Daemon *Daemon

	api *Api

	// Plugins are the installed source plugins with their sources' state.
	Plugins []SourcePlugin
	Agent   J
	Health  J

	Courses        []Course
	CoursesLoading bool
	CoursesError   error

	Jobs map[string]Job

	// UnreadNotifications counts unread items of the daemon's feed.
	UnreadNotifications int

	// Events are the daemon's raw events (lecture_changed, notes_changed…).
	Events Events

	sseCancel  context.CancelFunc
	connecting bool
}

// NewAppState is the state of an app whose daemon is d.
func NewAppState(d *Daemon) *AppState {
	return &AppState{Daemon: d, Agent: J{"state": "starting"}, Health: J{}, Jobs: map[string]Job{}}
}

// Api is the daemon client; valid once Connected.
func (a *AppState) Api() *Api       { return a.api }
func (a *AppState) Connected() bool { return a.api != nil }

// OnDaemon follows the daemon's state, on the UI thread: once it is ready
// it connects, loads what the app shows and listens to its events.
func (a *AppState) OnDaemon() {
	if a.Daemon.State() != DaemonReady {
		return
	}
	base := a.Daemon.BaseURL()
	if a.api != nil && a.api.BaseURL == base || a.connecting {
		return
	}
	a.connecting = true
	api := NewApi(base, a.Daemon.Token())
	go func() {
		var (
			health  J
			plugins []SourcePlugin
			jobs    []Job
			unread  int
		)
		func() {
			h, err := api.GetJ("/api/health")
			if err != nil {
				log.Printf("init failed: %v", err)
				return
			}
			health = h
			plugins = loadPlugins(api)
			js, err := api.Get("/api/jobs")
			if err != nil {
				log.Printf("init failed: %v", err)
				return
			}
			for _, j := range Objs(js) {
				jobs = append(jobs, Job{j})
			}
			n, err := api.GetJ("/api/notifications")
			if err != nil {
				log.Printf("init failed: %v", err)
				return
			}
			unread = n.I("unread")
		}()
		Post(func() {
			a.connecting = false
			a.api = api
			if health != nil {
				a.Health = health
			}
			if plugins != nil {
				a.Plugins = plugins
			}
			a.Jobs = map[string]Job{}
			for _, j := range jobs {
				a.Jobs[j.ID()] = j
			}
			a.UnreadNotifications = unread
			a.connectEvents()
			a.LoadCourses(false, false)
		})
	}()
}

func loadPlugins(api *Api) []SourcePlugin {
	r, err := api.GetJ("/api/sources")
	if err != nil {
		log.Printf("sources: %v", err)
		return nil
	}
	out := []SourcePlugin{}
	for _, p := range r.Os("plugins") {
		out = append(out, SourcePlugin{p})
	}
	return out
}

func (a *AppState) connectEvents() {
	if a.sseCancel != nil {
		a.sseCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.sseCancel = cancel
	api := a.api
	go func() {
		_ = api.SSE(ctx, "GET", "/api/events", nil, func(e J) { Post(func() { a.onEvent(e) }) })
		if ctx.Err() != nil {
			return
		}
		time.Sleep(time.Second)
		Post(func() {
			if ctx.Err() == nil && a.api == api && a.Daemon.State() == DaemonReady {
				a.connectEvents()
			}
		})
	}()
}

func (a *AppState) onEvent(e J) {
	data := e.O("data")
	switch e.S("type") {
	case "hello":
		if data != nil && data.O("agent") != nil {
			a.Agent = data.O("agent")
		}
	case "sources":
		a.LoadSources()
	case "agent":
		if data != nil {
			a.Agent = data
		}
	case "job":
		if data != nil {
			j := Job{data}
			a.Jobs[j.ID()] = j
		}
	case "job_removed":
		if data != nil {
			delete(a.Jobs, data.S("id"))
		}
	case "courses_changed":
		a.LoadCourses(false, true)
	case "notification":
		a.UnreadNotifications++
	case "notifications_read":
		if data != nil {
			a.UnreadNotifications = data.I("unread")
		}
	}
	a.Events.Emit(e)
}

// Resumed is didChangeAppLifecycleState(resumed): a folder sync runs when
// the app comes back.
func (a *AppState) Resumed() {
	if !a.Connected() {
		return
	}
	api := a.api
	go func() {
		s, err := api.GetJ("/api/sync")
		if err == nil && s.S("kind") == "folder" && s["configured"] == true {
			_, _ = api.Post("/api/sync/run", nil)
		}
	}()
}

// RefreshHealth reads the daemon's health again, e.g. after installing ffmpeg.
func (a *AppState) RefreshHealth() {
	if !a.Connected() {
		return
	}
	Async(func() (J, error) { return a.api.GetJ("/api/health") }, func(h J, err error) {
		if err == nil {
			a.Health = h
		}
	})
}

// LoadCourses reloads the course list; sync asks the sources first, quiet
// keeps the list on screen while it loads.
func (a *AppState) LoadCourses(sync, quiet bool) {
	a.LoadCoursesThen(sync, quiet, nil)
}

// LoadCoursesThen is LoadCourses with a callback once it is done.
func (a *AppState) LoadCoursesThen(sync, quiet bool, done func()) {
	if !a.Connected() {
		if done != nil {
			done()
		}
		return
	}
	if !quiet {
		a.CoursesLoading, a.CoursesError = true, nil
	}
	path := "/api/courses"
	if sync {
		path += "?sync=1"
	}
	api := a.api
	Async(func() (any, error) { return api.Get(path) }, func(v any, err error) {
		a.CoursesLoading = false
		if err != nil {
			a.CoursesError = err
		} else {
			var cs []Course
			for _, c := range Objs(v) {
				cs = append(cs, Course{c})
			}
			a.Courses, a.CoursesError = cs, nil
		}
		if done != nil {
			done()
		}
	})
}

// Course is a course by id.
func (a *AppState) Course(id string) (Course, bool) {
	for _, c := range a.Courses {
		if c.ID() == id {
			return c, true
		}
	}
	return Course{}, false
}

// LoadSources reloads the source plugins.
func (a *AppState) LoadSources() { a.LoadSourcesThen(nil) }

// LoadSourcesThen is LoadSources running done once they are in.
func (a *AppState) LoadSourcesThen(done func()) {
	if !a.Connected() {
		if done != nil {
			done()
		}
		return
	}
	api := a.api
	Async(func() ([]SourcePlugin, error) { return loadPlugins(api), nil }, func(p []SourcePlugin, _ error) {
		if p != nil {
			a.Plugins = p
		}
		if done != nil {
			done()
		}
	})
}

// Sources is every source of every installed plugin.
func (a *AppState) Sources() []Source {
	var out []Source
	for _, p := range a.Plugins {
		out = append(out, p.Sources()...)
	}
	return out
}

// Source is a source by id.
func (a *AppState) Source(id string) (Source, bool) {
	for _, s := range a.Sources() {
		if s.ID() == id {
			return s, true
		}
	}
	return Source{}, false
}

func (a *AppState) AnySignedIn() bool {
	for _, s := range a.Sources() {
		if s.SignedIn() {
			return true
		}
	}
	return false
}

// AccountName is the first signed-in account's name, a profile fallback.
func (a *AppState) AccountName() string {
	for _, s := range a.Sources() {
		if s.SignedIn() && s.Account() != "" {
			return s.Account()
		}
	}
	return ""
}

// PatchCourse changes a course and reloads the list quietly.
func (a *AppState) PatchCourse(id string, patch J, done func(error)) {
	api := a.api
	Go(func() error { _, err := api.Patch("/api/courses/"+id, patch); return err }, func(err error) {
		if err == nil {
			a.LoadCoursesThen(false, true, func() {
				if done != nil {
					done(nil)
				}
			})
			return
		}
		if done != nil {
			done(err)
		}
	})
}

// Enqueue queues jobs of kinds for lectures.
func (a *AppState) Enqueue(lectureIDs, kinds []string, done func([]Job, error)) {
	api := a.api
	Async(func() (any, error) {
		return api.Post("/api/jobs", J{"lecture_ids": lectureIDs, "kinds": kinds})
	}, func(v any, err error) {
		var js []Job
		for _, j := range Objs(v) {
			job := Job{j}
			a.Jobs[job.ID()] = job
			js = append(js, job)
		}
		if done != nil {
			done(js, err)
		}
	})
}

// JobAction pauses, resumes, retries or removes a job.
func (a *AppState) JobAction(id, action string, done func(error)) {
	api := a.api
	Go(func() error {
		var err error
		if action == "remove" {
			_, err = api.Delete("/api/jobs/" + id)
		} else {
			_, err = api.Post("/api/jobs/"+id+"/"+action, nil)
		}
		return err
	}, func(err error) {
		if err == nil && action == "remove" {
			delete(a.Jobs, id)
		}
		if done != nil {
			done(err)
		}
	})
}

// BulkJobs pauses, resumes or retries every job (or ids); done gets how
// many changed.
func (a *AppState) BulkJobs(action string, ids []string, done func(int, error)) {
	body := J{"action": action}
	if ids != nil {
		body["ids"] = ids
	}
	api := a.api
	Async(func() (any, error) { return api.Post("/api/jobs/bulk", body) }, func(v any, err error) {
		n := 0
		if m := Obj(v); m != nil {
			n = m.I("changed")
		}
		if done != nil {
			done(n, err)
		}
	})
}

// ClearFinished clears finished records, and with files their files;
// done gets the bytes freed.
func (a *AppState) ClearFinished(files bool, done func(int, error)) {
	path := "/api/jobs/clear"
	if files {
		path += "?files=1"
	}
	api := a.api
	Async(func() (any, error) { return api.Post(path, nil) }, func(v any, err error) {
		if err == nil {
			for id, j := range a.Jobs {
				if s := j.State(); s == "done" || s == "canceled" {
					delete(a.Jobs, id)
				}
			}
		}
		n := 0
		if m := Obj(v); m != nil {
			n = m.I("freed")
		}
		if done != nil {
			done(n, err)
		}
	})
}

// RemoveJobFiles removes a job with the files it produced.
func (a *AppState) RemoveJobFiles(id string, done func(int, error)) {
	api := a.api
	Async(func() (any, error) { return api.Delete("/api/jobs/" + id + "?files=1") }, func(v any, err error) {
		if err == nil {
			delete(a.Jobs, id)
		}
		n := 0
		if m := Obj(v); m != nil {
			n = m.I("freed")
		}
		if done != nil {
			done(n, err)
		}
	})
}

// RemoveAsset deletes one kind of local asset of a lecture.
func (a *AppState) RemoveAsset(lectureID, kind string, done func(int, error)) {
	api := a.api
	Async(func() (any, error) { return api.Delete("/api/lectures/" + lectureID + "/assets/" + kind) }, func(v any, err error) {
		if err == nil {
			delete(a.Jobs, lectureID+"-"+kind)
		}
		n := 0
		if m := Obj(v); m != nil {
			n = m.I("freed")
		}
		if done != nil {
			done(n, err)
		}
	})
}

// SortedJobs is running, queued, paused, failed, then the rest, newest
// first within each.
func (a *AppState) SortedJobs() []Job {
	l := make([]Job, 0, len(a.Jobs))
	for _, j := range a.Jobs {
		l = append(l, j)
	}
	rank := func(j Job) int {
		switch j.State() {
		case "running":
			return 0
		case "queued":
			return 1
		case "paused":
			return 2
		case "failed":
			return 3
		}
		return 4
	}
	sort.SliceStable(l, func(i, k int) bool {
		if ri, rk := rank(l[i]), rank(l[k]); ri != rk {
			return ri < rk
		}
		if l[i].UpdatedAt() != l[k].UpdatedAt() {
			return l[i].UpdatedAt() > l[k].UpdatedAt()
		}
		return l[i].ID() < l[k].ID()
	})
	return l
}

func (a *AppState) ActiveJobCount() int {
	n := 0
	for _, j := range a.Jobs {
		if j.Active() {
			n++
		}
	}
	return n
}

// JobFor is the job of one kind for a lecture.
func (a *AppState) JobFor(lectureID, kind string) (Job, bool) {
	j, ok := a.Jobs[lectureID+"-"+kind]
	return j, ok
}

func (a *AppState) AgentReady() bool { return a.Agent.S("state") == "ready" }

// Dispose stops listening to the daemon.
func (a *AppState) Dispose() {
	if a.sseCancel != nil {
		a.sseCancel()
	}
}

// ScreenContext is what the user is looking at, shared with the assistant.
type ScreenContext struct {
	Value map[string]string
}

// NewScreenContext starts on the home screen.
func NewScreenContext() *ScreenContext {
	return &ScreenContext{Value: map[string]string{"screen": l10n.ScreenHome}}
}

// Set replaces the context.
func (s *ScreenContext) Set(v map[string]string) { s.Value = v }

// PlayerHandle is the active player, which the assistant can seek.
type PlayerHandle interface {
	LectureID() string
	PositionMs() int
	SeekTo(ms int)
}

// PlayerHub holds the active player.
type PlayerHub struct{ Value PlayerHandle }

// Platform is defaultTargetPlatform's name: linux, macos, windows, android, ios.
func Platform() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	}
	return runtime.GOOS
}

// Desktop is whether the app runs on a desktop system.
func Desktop() bool {
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		return true
	}
	return false
}
