// Package sources runs source plugins: native provider programs that sign in
// to an external course platform, list its courses and lectures, and resolve
// media, slides, courseware and learning-platform data. Providers speak
// JSON-RPC 2.0 over their stdin and stdout, one message per line; the daemon
// keeps their secrets (see vault.Namespace) and performs every download.
//
// The contract is the source-provider spec in docs/specs.
package sources

import "fmt"

// ProtocolVersion is the provider protocol this daemon speaks.
const ProtocolVersion = "1"

// Capability groups a provider may announce in its initialize result.
const (
	CapSlides     = "slides"
	CapCourseware = "courseware"
	CapLMS        = "lms"
)

// Field is one input of a provider's sign-in form.
type Field struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	// Kind is "text", "password" or "otp".
	Kind string `json:"kind"`
	// Remember false means the value is passed to the provider and never stored.
	Remember *bool `json:"remember,omitempty"`
}

// InitResult is the provider's answer to initialize.
type InitResult struct {
	Protocol     string   `json:"protocol"`
	Capabilities []string `json:"capabilities"`
	SignIn       struct {
		Fields []Field `json:"fields"`
	} `json:"sign_in"`
}

func (r *InitResult) has(capability string) bool {
	for _, c := range r.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// Sign-in states a provider reports.
const (
	SignedOut  = "signed_out"
	SignedIn   = "signed_in"
	NeedsInput = "needs_input"
	Expired    = "expired"
)

// Challenge is an extra sign-in step, such as a captcha image.
type Challenge struct {
	ID string `json:"id"`
	// Kind is "image", "text" or "qr".
	Kind   string `json:"kind"`
	Prompt string `json:"prompt"`
	// Image is a data: URL for image and qr challenges.
	Image string `json:"image,omitempty"`
}

// Auth is a provider's sign-in state.
type Auth struct {
	State     string     `json:"state"`
	Account   *Account   `json:"account,omitempty"`
	Challenge *Challenge `json:"challenge,omitempty"`
}

// Account identifies who is signed in, for display only.
type Account struct {
	Display string `json:"display"`
}

// Teacher is one teacher of a course.
type Teacher struct {
	Username string `json:"username"`
	Realname string `json:"realname"`
}

// Course is a platform course as the provider lists it.
type Course struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Teacher       string    `json:"teacher,omitempty"`
	Teachers      []Teacher `json:"teachers,omitempty"`
	College       string    `json:"college,omitempty"`
	Term          string    `json:"term,omitempty"`
	Thumb         string    `json:"thumb,omitempty"`
	StartAt       int64     `json:"start_at,omitempty"`
	EndAt         int64     `json:"end_at,omitempty"`
	LectureTotal  int       `json:"lecture_total,omitempty"`
	Progress      float64   `json:"progress,omitempty"`
	LastLectureID string    `json:"last_lecture_id,omitempty"`
	LastLearnAt   string    `json:"last_learn_at,omitempty"`
	// URL opens the course on the platform.
	URL string `json:"url,omitempty"`
}

// Lecture is one recorded class of a course.
type Lecture struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Date        string `json:"date,omitempty"`
	BeginAt     int64  `json:"begin_at,omitempty"`
	EndAt       int64  `json:"end_at,omitempty"`
	Lecturer    string `json:"lecturer,omitempty"`
	Room        string `json:"room,omitempty"`
	HasPlayback bool   `json:"has_playback"`
	URL         string `json:"url,omitempty"`
}

// Address is where the daemon fetches something. Headers never leave the daemon.
type Address struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	// ExpiresAt is a Unix time after which the address must be resolved again.
	ExpiresAt int64 `json:"expires_at,omitempty"`
}

// Stream is one playable view of a lecture.
type Stream struct {
	ID string `json:"id"`
	// Kind is "camera", "screen" or "audio".
	Kind  string `json:"kind,omitempty"`
	Label string `json:"label"`
	Address
	Thumb    string `json:"thumb,omitempty"`
	Duration int64  `json:"duration,omitempty"`
}

// Media is a lecture's streams.
type Media struct {
	Streams  []Stream `json:"streams"`
	Duration int64    `json:"duration,omitempty"`
	Seekable bool     `json:"seekable"`
}

// Slide is one slide snapshot detected in a lecture.
type Slide struct {
	TSec     int64             `json:"t_sec"`
	URL      string            `json:"url"`
	ThumbURL string            `json:"thumb_url,omitempty"`
	Key      bool              `json:"key,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
}

// File is a courseware or learning-platform file. A list may carry its
// address directly; otherwise the matching resolve method returns it.
type File struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Folder    string `json:"folder,omitempty"`
	Size      int64  `json:"size,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Address
}

// Error kinds a provider reports in error.data.kind.
const (
	KindAuthRequired = "auth_required"
	KindRateLimited  = "rate_limited"
	KindUnavailable  = "unavailable"
	KindInvalid      = "invalid"
)

// Error is a provider or protocol failure. Message is user-facing.
type Error struct {
	Message       string
	Kind          string
	RetryAfterSec int
}

func (e *Error) Error() string { return e.Message }

func errorf(kind, format string, args ...any) *Error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}
