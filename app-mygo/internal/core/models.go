package core

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// The models are views over the daemon's JSON, as models.dart's classes
// are: each wraps the object and reads its fields leniently.

// SourcePlugin is an installed source plugin, as GET /api/sources lists it.
type SourcePlugin struct{ J }

func (p SourcePlugin) ID() string      { return p.S("id") }
func (p SourcePlugin) Title() string   { return p.S("title") }
func (p SourcePlugin) Summary() string { return p.S("summary") }

// Icon is a Material Symbols name or an image data: URL.
func (p SourcePlugin) Icon() string    { return p.S("icon") }
func (p SourcePlugin) Version() string { return p.S("version") }
func (p SourcePlugin) Enabled() bool   { return p.B("enabled") }

// Available is false when the plugin has no executable for this device.
func (p SourcePlugin) Available() bool { return p.B("available") }
func (p SourcePlugin) Hosts() []string { return p.Ss("hosts") }
func (p SourcePlugin) Sources() []Source {
	var out []Source
	for _, s := range p.Os("sources") {
		out = append(out, Source{s})
	}
	return out
}

// Source is one source of a plugin and its sign-in state.
type Source struct{ J }

func (s Source) auth() J {
	if a := s.O("auth"); a != nil {
		return a
	}
	return J{}
}

// ID is "<plugin>/<source>".
func (s Source) ID() string     { return s.S("id") }
func (s Source) Plugin() string { return s.S("plugin") }
func (s Source) Title() string  { return s.S("title") }

// Process is the provider process: stopped, starting, ready or error.
func (s Source) Process() string { return s.S("process") }
func (s Source) Error() string   { return s.S("error") }

// State is signed_out, signed_in, needs_input or expired.
func (s Source) State() string  { return s.auth().S("state") }
func (s Source) SignedIn() bool { return s.State() == "signed_in" }
func (s Source) Expired() bool  { return s.State() == "expired" }
func (s Source) Account() string {
	if a := s.auth().O("account"); a != nil {
		return a.S("display")
	}
	return ""
}
func (s Source) Challenge() *SignInChallenge {
	if c := s.auth().O("challenge"); c != nil {
		return &SignInChallenge{c}
	}
	return nil
}
func (s Source) Capabilities() []string { return s.Ss("capabilities") }
func (s Source) HasLms() bool           { return slices.Contains(s.Capabilities(), "lms") }
func (s Source) Fields() []SignInField {
	var out []SignInField
	for _, f := range s.Os("fields") {
		out = append(out, SignInField{f})
	}
	return out
}

// SignInField is one input of a source's sign-in form.
type SignInField struct{ J }

func (f SignInField) Key() string   { return f.S("key") }
func (f SignInField) Title() string { return f.S("title") }

// Kind is text, password or otp.
func (f SignInField) Kind() string { return f.S("kind") }

// Remember is false when the value is passed once and never stored.
func (f SignInField) Remember() bool { return f.J["remember"] != false }

// SignInChallenge is an extra sign-in step, such as a captcha image.
type SignInChallenge struct{ J }

func (c SignInChallenge) ID() string { return c.S("id") }

// Kind is image, text or qr.
func (c SignInChallenge) Kind() string   { return c.S("kind") }
func (c SignInChallenge) Prompt() string { return c.S("prompt") }

// Image is a data: URL for image and qr challenges.
func (c SignInChallenge) Image() string { return c.S("image") }

// Course is a course of the library.
type Course struct{ J }

func (c Course) IsLocal() bool { return strings.HasPrefix(c.ID(), "local-") }
func (c Course) ID() string    { return c.S("id") }

// Source is "<plugin>/<source>"; empty for local courses.
func (c Course) Source() string { return c.S("source") }

// Refreshable is false when the course's source plugin is gone.
func (c Course) Refreshable() bool { return c.B("refreshable") }
func (c Course) URL() string       { return c.S("url") }
func (c Course) RawTitle() string  { return c.S("title") }
func (c Course) Alias() string     { return c.S("alias") }
func (c Course) Title() string {
	if a := c.Alias(); a != "" {
		return a
	}
	return c.RawTitle()
}
func (c Course) Teacher() string     { return c.S("teacher") }
func (c Course) College() string     { return c.S("college") }
func (c Course) Term() string        { return c.S("term") }
func (c Course) LectureTotal() int   { return c.I("lecture_total") }
func (c Course) Progress() float64   { return c.F("progress") }
func (c Course) Pinned() bool        { return c.B("pinned") }
func (c Course) Hidden() bool        { return c.B("hidden") }
func (c Course) Color() string       { return c.S("color") }
func (c Course) LastSubID() string   { return c.S("last_sub_id") }
func (c Course) LastLearnAt() string { return c.S("last_learn_at") }
func (c Course) LocalLectures() int  { return c.I("local_lectures") }
func (c Course) Teachers() []string {
	var out []string
	for _, t := range c.Os("teachers") {
		if n := t.S("realname"); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// TermLabel is "20261" → "2026 秋", "20252" → "2026 春".
func (c Course) TermLabel() string {
	if c.IsLocal() {
		return l10n.LocalCourse
	}
	t := c.Term()
	if len(t) != 5 {
		return t
	}
	y, _ := strconv.Atoi(t[:4])
	if strings.HasSuffix(t, "1") {
		return l10n.TermAutumn(y)
	}
	return l10n.TermSpring(y + 1)
}

// Lecture is one lecture of a course.
type Lecture struct{ J }

func (l Lecture) ID() string        { return l.S("id") }
func (l Lecture) CourseID() string  { return l.S("course_id") }
func (l Lecture) Title() string     { return l.S("title") }
func (l Lecture) Date() string      { return l.S("date") }
func (l Lecture) BeginAt() int      { return l.I("begin_at") }
func (l Lecture) EndAt() int        { return l.I("end_at") }
func (l Lecture) Lecturer() string  { return l.S("lecturer") }
func (l Lecture) Room() string      { return l.S("room") }
func (l Lecture) HasPlayback() bool { return l.B("has_playback") }
func (l Lecture) DurationS() int    { return l.I("duration_s") }
func (l Lecture) Thumb() string     { return l.S("thumb") }
func (l Lecture) Dir() string       { return l.S("dir") }
func (l Lecture) Assets() []string  { return l.Ss("assets") }
func (l Lecture) PositionMs() int   { return l.I("position_ms") }
func (l Lecture) Segments() int     { return l.I("segments") }
func (l Lecture) Has(kind string) bool {
	return slices.Contains(l.Assets(), kind)
}

// Begin is the lecture's start, or the zero time.
func (l Lecture) Begin() time.Time {
	if b := l.BeginAt(); b > 0 {
		return time.Unix(int64(b), 0)
	}
	return time.Time{}
}
func (l Lecture) Upcoming() bool {
	return l.BeginAt() > 0 && time.Now().Unix() < int64(l.BeginAt())
}

// ShortTitle is the title without a leading date the platform repeats.
func (l Lecture) ShortTitle() string {
	t := l.Title()
	if d := l.Date(); strings.HasPrefix(t, d) {
		t = strings.TrimSpace(t[len(d):])
	}
	if t == "" {
		return l.Title()
	}
	return t
}

// Job is a download or processing job.
type Job struct{ J }

func (j Job) ID() string        { return j.S("id") }
func (j Job) CourseID() string  { return j.S("course_id") }
func (j Job) LectureID() string { return j.S("lecture_id") }
func (j Job) Kind() string      { return j.S("kind") }
func (j Job) Title() string     { return j.S("title") }
func (j Job) State() string     { return j.S("state") }
func (j Job) DoneBytes() int    { return j.I("done_bytes") }
func (j Job) TotalBytes() int   { return j.I("total_bytes") }
func (j Job) Progress() float64 { return min(max(j.F("progress"), 0), 1) }
func (j Job) Error() string     { return j.S("error") }
func (j Job) Speed() int        { return j.I("speed") }
func (j Job) Conns() int        { return j.I("conns") }
func (j Job) Stage() string     { return j.S("stage") }
func (j Job) UpdatedAt() int    { return j.I("updated_at") }
func (j Job) Active() bool      { s := j.State(); return s == "running" || s == "queued" }

// RetryAt is when a queued job retries by itself, or the zero time.
func (j Job) RetryAt() time.Time {
	if r := j.I("retry_at"); r > 0 {
		return time.Unix(int64(r), 0)
	}
	return time.Time{}
}

type Slide struct{ J }

func (s Slide) Idx() int          { return s.I("idx") }
func (s Slide) TSec() int         { return s.I("t_sec") }
func (s Slide) IsKey() bool       { return s.B("is_key") }
func (s Slide) RemoteURL() string { return s.S("remote_url") }
func (s Slide) Abs() string       { return s.S("abs") }
func (s Slide) Path() string      { return s.S("path") }

type Segment struct{ J }

func (s Segment) Idx() int     { return s.I("idx") }
func (s Segment) StartMs() int { return s.I("start_ms") }
func (s Segment) EndMs() int   { return s.I("end_ms") }
func (s Segment) Text() string { return s.S("text") }

type Note struct{ J }

func (n Note) ID() string        { return n.S("id") }
func (n Note) Title() string     { return n.S("title") }
func (n Note) Body() string      { return n.S("body") }
func (n Note) CourseID() string  { return n.S("course_id") }
func (n Note) LectureID() string { return n.S("lecture_id") }
func (n Note) Source() string    { return n.S("source") }
func (n Note) Path() string      { return n.S("path") }
func (n Note) UpdatedAt() int    { return n.I("updated_at") }

type Bookmark struct{ J }

func (b Bookmark) ID() int       { return b.I("id") }
func (b Bookmark) TMs() int      { return b.I("t_ms") }
func (b Bookmark) Label() string { return b.S("label") }

// Hit is a search result.
type Hit struct{ J }

func (h Hit) Kind() string        { return h.S("kind") }
func (h Hit) CourseID() string    { return h.S("course_id") }
func (h Hit) CourseTitle() string { return h.S("course_title") }
func (h Hit) LectureID() string   { return h.S("lecture_id") }
func (h Hit) Lecture() string     { return h.S("lecture") }
func (h Hit) NoteID() string      { return h.S("note_id") }
func (h Hit) StartMs() int        { return h.I("start_ms") }
func (h Hit) Snippet() string     { return h.S("snippet") }

// Stream is one playable stream of a lecture (Stream_ in Dart).
type Stream struct{ J }

func (s Stream) Label() string { return s.S("label") }
func (s Stream) URL() string   { return s.S("url") }
func (s Stream) Thumb() string { return s.S("thumb") }

type LectureMedia struct{ J }

func (m LectureMedia) Streams() []Stream {
	var out []Stream
	for _, s := range m.Os("streams") {
		out = append(out, Stream{s})
	}
	return out
}
func (m LectureMedia) Duration() int { return m.I("duration") }
func (m LectureMedia) Headers() map[string]string {
	out := map[string]string{}
	for k, v := range m.O("headers") {
		out[k] = Str(v)
	}
	return out
}
func (m LectureMedia) LocalPath() string { return m.S("local_path") }

// Relayed: streams are daemon paths, needing headers that stay in the daemon.
func (m LectureMedia) Relayed() bool   { return m.B("relayed") }
func (m LectureMedia) Courseware() []J { return m.Os("courseware") }

// ScheduleItem is a dated item on the schedule.
type ScheduleItem struct{ J }

func (s ScheduleItem) ID() string          { return s.S("id") }
func (s ScheduleItem) Kind() string        { return s.S("kind") }
func (s ScheduleItem) Title() string       { return s.S("title") }
func (s ScheduleItem) Detail() string      { return s.S("detail") }
func (s ScheduleItem) DueDate() string     { return s.S("due_date") }
func (s ScheduleItem) DueTime() string     { return s.S("due_time") }
func (s ScheduleItem) CourseID() string    { return s.S("course_id") }
func (s ScheduleItem) CourseTitle() string { return s.S("course_title") }
func (s ScheduleItem) LectureID() string   { return s.S("lecture_id") }
func (s ScheduleItem) Lecture() string     { return s.S("lecture") }
func (s ScheduleItem) TMs() int {
	if s.J["t_ms"] == nil {
		return -1
	}
	return s.I("t_ms")
}
func (s ScheduleItem) Quote() string  { return s.S("quote") }
func (s ScheduleItem) Source() string { return s.S("source") }
func (s ScheduleItem) Done() bool     { return s.B("done") }

// IsLms: a learning-platform assignment, read-only except done.
func (s ScheduleItem) IsLms() bool { return s.Source() == "lms" }
func (s ScheduleItem) URL() string { return s.S("url") }

// Day is the due date, in local time.
func (s ScheduleItem) Day() time.Time { return ParseDay(s.DueDate()) }
func (s ScheduleItem) Overdue() bool {
	n := time.Now()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.Local)
	return !s.Done() && s.Day().Before(today)
}

// ParseDay is DateTime.tryParse for a "YYYY-MM-DD" date, 1970 when it fails.
func ParseDay(s string) time.Time {
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05", "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local)
}

// Chapter is one chapter of a lecture outline.
type Chapter struct{ J }

func (c Chapter) Idx() int      { return c.I("idx") }
func (c Chapter) StartMs() int  { return c.I("start_ms") }
func (c Chapter) EndMs() int    { return c.I("end_ms") }
func (c Chapter) Title() string { return c.S("title") }

// CoursewareFile is an uploaded courseware file and its MinerU parse state.
type CoursewareFile struct{ J }

func (f CoursewareFile) Path() string { return f.S("path") }
func (f CoursewareFile) Name() string { return f.S("name") }
func (f CoursewareFile) Bytes() int   { return f.I("bytes") }
func (f CoursewareFile) p() J {
	if p := f.O("parse"); p != nil {
		return p
	}
	return J{}
}
func (f CoursewareFile) ParseState() string { return f.p().S("state") }
func (f CoursewareFile) Pages() int         { return f.p().I("pages") }
func (f CoursewareFile) MdPath() string     { return f.p().S("md") }
func (f CoursewareFile) ParseError() string { return f.p().S("error") }
func (f CoursewareFile) Parsed() bool       { return f.ParseState() == "done" }
func (f CoursewareFile) Ext() string {
	n := f.Name()
	if i := strings.LastIndex(n, "."); i >= 0 {
		return strings.ToLower(n[i+1:])
	}
	return ""
}
