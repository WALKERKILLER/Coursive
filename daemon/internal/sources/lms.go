package sources

import (
	"context"
	"time"
)

// LMSCourse is a course on a source's learning platform.
type LMSCourse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Code  string `json:"code,omitempty"`
	Term  string `json:"term,omitempty"`
}

// Assignment is a learning-platform assignment.
type Assignment struct {
	ID        string     `json:"id"`
	Course    string     `json:"course"`
	Title     string     `json:"title"`
	DueAt     *time.Time `json:"due_at,omitempty"`
	URL       string     `json:"url,omitempty"`
	Points    *float64   `json:"points,omitempty"`
	Submitted bool       `json:"submitted"`
}

// Announcement is a learning-platform announcement; HTML is the platform's body.
type Announcement struct {
	ID       string     `json:"id"`
	Course   string     `json:"course"`
	Title    string     `json:"title"`
	HTML     string     `json:"html"`
	PostedAt *time.Time `json:"posted_at,omitempty"`
	URL      string     `json:"url,omitempty"`
	Author   string     `json:"author,omitempty"`
}

// LMSCourses lists the learning platform's courses.
func (p *Provider) LMSCourses(ctx context.Context) ([]LMSCourse, error) {
	if err := p.require(ctx, CapLMS); err != nil {
		return nil, err
	}
	var out struct {
		Courses []LMSCourse `json:"courses"`
	}
	err := p.Call(ctx, "lms.courses", struct{}{}, &out)
	return out.Courses, err
}

// Assignments lists one course's assignments.
func (p *Provider) Assignments(ctx context.Context, course string) ([]Assignment, error) {
	var out struct {
		Assignments []Assignment `json:"assignments"`
	}
	err := p.Call(ctx, "lms.assignments", map[string]any{"course": course}, &out)
	return out.Assignments, err
}

// Announcements lists announcements of the given courses posted since a time.
func (p *Provider) Announcements(ctx context.Context, courses []string, since time.Time) ([]Announcement, error) {
	var out struct {
		Announcements []Announcement `json:"announcements"`
	}
	err := p.Call(ctx, "lms.announcements", map[string]any{"courses": courses, "since": since.UTC().Format(time.RFC3339)}, &out)
	return out.Announcements, err
}

// LMSFiles lists one course's files.
func (p *Provider) LMSFiles(ctx context.Context, course string) ([]File, error) {
	if err := p.require(ctx, CapLMS); err != nil {
		return nil, err
	}
	var out struct {
		Files []File `json:"files"`
	}
	err := p.Call(ctx, "lms.files", map[string]any{"course": course}, &out)
	return out.Files, err
}

// LMSFileAddress returns where to fetch one file.
func (p *Provider) LMSFileAddress(ctx context.Context, f File) (Address, error) {
	if f.URL != "" {
		return f.Address, nil
	}
	var a Address
	err := p.Call(ctx, "lms.file", map[string]any{"file": f.ID}, &a)
	return a, err
}
