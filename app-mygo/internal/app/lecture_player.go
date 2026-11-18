package app

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// lecture_player.dart: the players of a lecture and everything the player
// UI shows.

// VideoSource is one video source: a remote stream or the downloaded file.
type VideoSource struct {
	Label string
	URI   string
	Local bool
}

// LecturePlayer owns the video players of a lecture and its side data.
type LecturePlayer struct {
	lectureID string

	Player *ui.VideoPlayer
	Pip    *ui.VideoPlayer
	PipIdx int // -1 without a second camera

	// PanelTab asks the side panel to show a tab ("outline", "transcript"…).
	PanelTab string

	duration time.Duration
	rate     float64
	volume   float64 // 0 to 100, as media_kit's
	Boosting bool
	err      string
	seekMs   int // the position asked for until the player shows it
	seekAt   time.Time

	Lecture    *core.Lecture
	Course     *core.Course
	Media      *core.LectureMedia
	Sources    []VideoSource
	SourceIdx  int
	Slides     []core.Slide
	Segments   []core.Segment
	Bookmarks  []core.Bookmark
	Notes      []core.Note
	Chapters   []core.Chapter
	Courseware []core.CoursewareFile

	OutlineBusy  bool
	OutlineError string
	Loading      bool
	LoadError    error
	ResumedFrom  int

	watch     *core.ActivityClock
	watching  bool
	lastSaved int
	savedAt   time.Time
	ended     bool
	pipSync   time.Time
	disposed  bool
}

// NewLecturePlayer is a player of lecture id, not loaded yet.
func NewLecturePlayer(id string) *LecturePlayer {
	p := &LecturePlayer{lectureID: id, PipIdx: -1, rate: 1, volume: 100, Loading: true, lastSaved: -1, seekMs: -1}
	if core.Tracker != nil {
		p.watch = core.Tracker.Clock("watch", id, false)
	}
	return p
}

// LectureID is the lecture shown (PlayerHandle).
func (p *LecturePlayer) LectureID() string { return p.lectureID }

// PositionMs is the playhead (PlayerHandle).
func (p *LecturePlayer) PositionMs() int {
	if p.seekMs >= 0 {
		return p.seekMs
	}
	if p.Player == nil {
		return 0
	}
	return int(p.Player.Position().Milliseconds())
}

// DurationMs is the length of the video, the platform's until the player
// knows it.
func (p *LecturePlayer) DurationMs() int {
	if p.Player != nil {
		if d := p.Player.Duration(); d > 0 {
			return int(d.Milliseconds())
		}
	}
	return int(p.duration.Milliseconds())
}

// Playing, Buffering, Rate, Volume and Err are the main player's state.
func (p *LecturePlayer) Playing() bool { return p.Player != nil && p.Player.Playing() }
func (p *LecturePlayer) Buffering() bool {
	return p.Player == nil || !p.Player.Ready() || p.Player.Buffering()
}
func (p *LecturePlayer) Rate() float64   { return p.rate }
func (p *LecturePlayer) Volume() float64 { return p.volume }
func (p *LecturePlayer) Err() string {
	if p.err != "" {
		return p.err
	}
	if p.Player != nil && p.Player.Err() != nil {
		return p.Player.Err().Error()
	}
	return ""
}

// Source is the source playing.
func (p *LecturePlayer) Source() *VideoSource {
	if len(p.Sources) == 0 {
		return nil
	}
	return &p.Sources[p.SourceIdx]
}

func (p *LecturePlayer) lp() string { return "/api/lectures/" + core.Esc(p.lectureID) }

// Load fetches the lecture and its media, then opens the first source at
// the saved position.
func (p *LecturePlayer) Load() {
	p.Loading, p.LoadError = true, nil
	type loaded struct {
		d     core.J
		media core.J
	}
	core.Async(func() (loaded, error) {
		d, err := api().GetJ(p.lp())
		if err != nil {
			return loaded{}, err
		}
		m, err := api().GetJ(p.lp() + "/media")
		return loaded{d, m}, err
	}, func(r loaded, err error) {
		if p.disposed {
			return
		}
		if err != nil {
			p.Loading, p.LoadError = false, err
			return
		}
		d := r.d
		l := core.Lecture{J: d.O("lecture")}
		p.Lecture = &l
		if c := d.O("course"); c != nil {
			p.Course = &core.Course{J: c}
		}
		p.Bookmarks = jsonList[core.Bookmark](d["bookmarks"])
		p.Notes = jsonList[core.Note](d["notes"])
		p.Chapters = jsonList[core.Chapter](d["timeline"])
		localLabel := ""
		for _, a := range d.Os("assets") {
			if a.S("kind") != "video" {
				continue
			}
			switch m := a["meta"].(type) {
			case map[string]any:
				localLabel = core.J(m).S("stream")
			case string:
				var mm core.J
				if json.Unmarshal([]byte(m), &mm) == nil {
					localLabel = mm.S("stream")
				}
			}
		}
		resumeMs := d.O("playback").I("position_ms")
		media := core.LectureMedia{J: r.media}
		p.Media = &media
		p.Sources = nil
		for _, s := range media.Streams() {
			u := s.URL()
			if media.Relayed() {
				u = api().URI(u, map[string]string{"token": api().Token})
			}
			p.Sources = append(p.Sources, VideoSource{Label: s.Label(), URI: u})
		}
		if lp := media.LocalPath(); lp != "" {
			if localLabel == "" {
				localLabel = l10n.CommonLocal
			}
			p.Sources = append([]VideoSource{{Label: localLabel, URI: lp, Local: true}}, p.Sources...)
		}
		if len(p.Sources) == 0 {
			p.Loading, p.LoadError = false, errors.New(l10n.PlayerNoVideo)
			return
		}
		p.SourceIdx = 0
		if media.Duration() > 0 {
			p.duration = time.Duration(media.Duration()) * time.Second
		}
		p.Loading = false
		durMs := int(p.duration.Milliseconds())
		start := 0
		if core.UI.AutoResume && resumeMs > 10000 && (durMs == 0 || resumeMs < durMs-30000) {
			start = resumeMs
		}
		p.ResumedFrom = start
		p.open(p.Sources[0], start)
		p.SetRate(core.UI.DefaultSpeed)
		p.loadSide()
	})
}

func (p *LecturePlayer) loadSide() {
	core.Async(func() (any, error) { return api().Get(p.lp() + "/slides") }, func(r any, err error) {
		if err != nil {
			return
		}
		p.Slides = jsonList[core.Slide](r)
		sort.SliceStable(p.Slides, func(a, b int) bool { return p.Slides[a].TSec() < p.Slides[b].TSec() })
	})
	p.ReloadTranscript()
	p.ReloadCourseware()
}

// ReloadTimeline fetches the outline again.
func (p *LecturePlayer) ReloadTimeline() {
	core.Async(func() (any, error) { return api().Get(p.lp() + "/timeline") }, func(r any, err error) {
		if err == nil {
			p.Chapters = jsonList[core.Chapter](r)
		}
	})
}

// ReloadCourseware fetches the uploaded courseware again.
func (p *LecturePlayer) ReloadCourseware() {
	core.Async(func() (any, error) { return api().Get(p.lp() + "/courseware") }, func(r any, err error) {
		if err == nil {
			p.Courseware = jsonList[core.CoursewareFile](r)
		}
	})
}

// ReloadTranscript fetches the transcript again.
func (p *LecturePlayer) ReloadTranscript() {
	core.Async(func() (any, error) { return api().Get(p.lp() + "/transcript") }, func(r any, err error) {
		if err == nil {
			p.Segments = jsonList[core.Segment](r)
		}
	})
}

// ReloadNotes fetches the lecture's notes again.
func (p *LecturePlayer) ReloadNotes() {
	core.Async(func() (any, error) { return api().Get("/api/notes", map[string]string{"lecture_id": p.lectureID}) }, func(r any, err error) {
		if err == nil {
			p.Notes = jsonList[core.Note](r)
		}
	})
}

// ReloadSyncedState fetches what another device may have changed.
func (p *LecturePlayer) ReloadSyncedState() {
	core.Async(func() (core.J, error) { return api().GetJ(p.lp()) }, func(d core.J, err error) {
		if err != nil || p.disposed {
			return
		}
		p.Bookmarks = jsonList[core.Bookmark](d["bookmarks"])
		p.ReloadNotes()
		p.ReloadTranscript()
		p.ReloadCourseware()
	})
}

// GenerateOutline asks the assistant host to write the outline (one model
// call over the whole transcript), by main stage or, when detailed, by
// topic. done runs after, with OutlineError set on failure.
func (p *LecturePlayer) GenerateOutline(detailed bool, done func()) {
	if p.OutlineBusy {
		return
	}
	p.OutlineBusy, p.OutlineError = true, ""
	detail := "concise"
	if detailed {
		detail = "detailed"
	}
	core.Async(func() (any, error) {
		return api().PostWait("/agent/timeline", core.J{"lecture_id": p.lectureID, "detail": detail}, 4*time.Minute)
	}, func(r any, err error) {
		p.OutlineBusy = false
		if err != nil {
			p.OutlineError = err.Error()
		} else {
			p.Chapters = jsonList[core.Chapter](core.Obj(r)["chapters"])
		}
		if done != nil && !p.disposed {
			done()
		}
	})
}

// DeleteOutline drops the outline.
func (p *LecturePlayer) DeleteOutline() {
	p.Chapters = nil
	core.Async(func() (any, error) { return api().Delete(p.lp() + "/timeline") }, nil)
}

// RestoreOutline puts chapters back (undo).
func (p *LecturePlayer) RestoreOutline(old []core.Chapter) {
	chapters := make([]any, len(old))
	for i, c := range old {
		chapters[i] = map[string]any(c.J)
	}
	core.Async(func() (any, error) { return api().Put(p.lp()+"/timeline", core.J{"chapters": chapters}) }, func(any, error) { p.ReloadTimeline() })
}

// ChapterAt is the chapter at ms, -1 without an outline.
func (p *LecturePlayer) ChapterAt(ms int) int {
	for i := len(p.Chapters) - 1; i >= 0; i-- {
		if p.Chapters[i].StartMs() <= ms {
			return i
		}
	}
	if len(p.Chapters) == 0 {
		return -1
	}
	return 0
}

// open plays s from start ms.
func (p *LecturePlayer) open(s VideoSource, start int) {
	p.err = ""
	if p.Player != nil {
		p.Player.Close()
		p.Player = nil
	}
	var v *ui.VideoPlayer
	var err error
	if s.Local {
		v, err = video.Open(s.URI)
	} else {
		v, err = video.OpenURL(s.URI)
	}
	if err != nil {
		p.err = err.Error()
		return
	}
	v.SetRate(p.rate).SetVolume(p.volume / 100)
	if start > 0 {
		v.Seek(time.Duration(start) * time.Millisecond)
	}
	p.Player, p.ended = v, false
}

// SwitchSource plays source i from where the video stands.
func (p *LecturePlayer) SwitchSource(i int) {
	if i >= len(p.Sources) {
		return
	}
	if i == p.SourceIdx && p.Player != nil && p.Err() == "" {
		return
	}
	at, wasPlaying := p.PositionMs(), p.Playing() || p.Player == nil
	p.SourceIdx = i
	if p.PipIdx == i {
		p.SetPip(-1)
	}
	p.open(p.Sources[i], at)
	if p.Player != nil && !wasPlaying {
		p.Player.Pause()
	}
}

// SetPip shows a second camera in a floating window, kept in sync with
// the main one; -1 closes it.
func (p *LecturePlayer) SetPip(i int) {
	if i == p.PipIdx {
		return
	}
	if p.Pip != nil {
		p.Pip.Close()
		p.Pip = nil
	}
	p.PipIdx = -1
	if i < 0 || i >= len(p.Sources) {
		return
	}
	v, err := video.OpenURL(p.Sources[i].URI)
	if err != nil {
		return
	}
	v.SetVolume(0).SetRate(p.rate)
	v.Seek(time.Duration(p.PositionMs()) * time.Millisecond)
	if !p.Playing() {
		v.Pause()
	}
	p.Pip, p.PipIdx = v, i
}

// SeekTo moves the playhead to ms (PlayerHandle).
func (p *LecturePlayer) SeekTo(ms int) {
	if d := p.DurationMs(); d > 0 {
		ms = min(ms, d)
	}
	ms = max(ms, 0)
	p.seekMs, p.seekAt = ms, time.Now()
	d := time.Duration(ms) * time.Millisecond
	if p.Player != nil {
		p.Player.Seek(d)
	}
	if p.Pip != nil {
		p.Pip.Seek(d)
	}
}

// SeekBy moves the playhead by seconds.
func (p *LecturePlayer) SeekBy(seconds int) { p.SeekTo(p.PositionMs() + seconds*1000) }

// PlayOrPause toggles playing.
func (p *LecturePlayer) PlayOrPause() {
	if p.Player == nil {
		return
	}
	if p.Player.Playing() {
		p.Player.Pause()
	} else {
		p.Player.Play()
	}
}

// SetRate plays at r.
func (p *LecturePlayer) SetRate(r float64) {
	p.rate = r
	if p.Player != nil && !p.Boosting {
		p.Player.SetRate(r)
	}
	if p.Pip != nil {
		p.Pip.SetRate(r)
	}
}

// StartBoost plays at the boost speed while held.
func (p *LecturePlayer) StartBoost() {
	if p.Boosting || !p.Playing() {
		return
	}
	p.Boosting = true
	p.Player.SetRate(core.UI.BoostSpeed)
}

// EndBoost goes back to the chosen speed.
func (p *LecturePlayer) EndBoost() {
	if !p.Boosting {
		return
	}
	p.Boosting = false
	if p.Player != nil {
		p.Player.SetRate(p.rate)
	}
}

// SetVolume sets the volume, 0 to 100.
func (p *LecturePlayer) SetVolume(v float64) {
	p.volume = min(max(v, 0), 100)
	if p.Player != nil {
		p.Player.SetVolume(p.volume / 100)
	}
}

// Tick follows the players each frame the page builds: the watch clock,
// the second camera's drift, saving the position and the end.
func (p *LecturePlayer) Tick() {
	if p.Player == nil {
		return
	}
	if p.seekMs >= 0 {
		pos := int(p.Player.Position().Milliseconds())
		if abs(pos-p.seekMs) < 1500 || time.Since(p.seekAt) > 3*time.Second {
			p.seekMs = -1
		}
	}
	playing := p.Player.Playing()
	if playing != p.watching {
		p.watching = playing
		if p.watch != nil {
			if playing {
				p.watch.Start()
			} else {
				p.watch.Stop()
			}
		}
		if p.Pip != nil {
			if playing {
				p.Pip.Play()
			} else {
				p.Pip.Pause()
			}
		}
	}
	if p.Pip != nil && time.Since(p.pipSync) > 2*time.Second {
		p.pipSync = time.Now()
		main := p.Player.Position()
		if d := p.Pip.Position() - main; d > 1200*time.Millisecond || d < -1200*time.Millisecond {
			p.Pip.Seek(main)
		}
	}
	if ended := p.Player.Ended(); ended != p.ended {
		p.ended = ended
		if ended {
			p.Save()
		}
	}
	if time.Since(p.savedAt) >= 10*time.Second {
		p.savedAt = time.Now()
		p.Save()
	}
}

// jsonList reads a JSON list of objects as models.
func jsonList[T ~struct{ core.J }](v any) []T {
	js := core.Objs(v)
	out := make([]T, len(js))
	for i, j := range js {
		out[i] = T{J: j}
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// SlideAt is the slide showing at ms, -1 without slides.
func (p *LecturePlayer) SlideAt(ms int) int {
	if len(p.Slides) == 0 {
		return -1
	}
	s := ms / 1000
	i := sort.Search(len(p.Slides), func(i int) bool { return p.Slides[i].TSec() > s })
	return max(i-1, 0)
}

// SegmentAt is the transcript line at ms, -1 before the first.
func (p *LecturePlayer) SegmentAt(ms int) int {
	return sort.Search(len(p.Segments), func(i int) bool { return p.Segments[i].StartMs() > ms }) - 1
}

// SlideURL is where a slide's picture loads from: the library file, or
// the daemon, which fetches a remote slide with any headers it needs.
func (p *LecturePlayer) SlideURL(s core.Slide) string {
	if s.Path() != "" {
		return api().FileURL(s.Path())
	}
	return api().URI(p.lp()+"/slides/"+strconv.Itoa(s.Idx())+"/image", map[string]string{"token": api().Token})
}

// Save stores the position.
func (p *LecturePlayer) Save() {
	ms := p.PositionMs()
	if ms <= 0 || abs(ms-p.lastSaved) < 2000 {
		return
	}
	p.lastSaved = ms
	body := core.J{"position_ms": ms, "duration_ms": p.DurationMs(), "speed": p.rate}
	core.Async(func() (any, error) { return api().Put(p.lp()+"/playback", body) }, nil)
}

// AddBookmark marks the position; done gets the bookmark, nil on failure.
func (p *LecturePlayer) AddBookmark(label string, done func(*core.Bookmark)) {
	body := core.J{"t_ms": p.PositionMs(), "label": label}
	core.Async(func() (any, error) { return api().Post(p.lp()+"/bookmarks", body) }, func(r any, err error) {
		if err != nil {
			if done != nil {
				done(nil)
			}
			return
		}
		b := core.Bookmark{J: core.Obj(r)}
		p.Bookmarks = append(p.Bookmarks, b)
		sort.SliceStable(p.Bookmarks, func(i, j int) bool { return p.Bookmarks[i].TMs() < p.Bookmarks[j].TMs() })
		if done != nil {
			done(&b)
		}
	})
}

// DeleteBookmark removes b.
func (p *LecturePlayer) DeleteBookmark(b core.Bookmark) {
	var keep []core.Bookmark
	for _, x := range p.Bookmarks {
		if x.ID() != b.ID() {
			keep = append(keep, x)
		}
	}
	p.Bookmarks = keep
	core.Async(func() (any, error) { return api().Delete("/api/bookmarks/" + strconv.Itoa(b.ID())) }, nil)
}

// Close saves the position and stops the players.
func (p *LecturePlayer) Close() {
	if p.watch != nil {
		p.watch.Stop()
	}
	p.Save()
	p.disposed = true
	if p.Pip != nil {
		p.Pip.Close()
	}
	if p.Player != nil {
		p.Player.Close()
	}
}
