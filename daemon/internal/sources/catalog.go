package sources

import (
	"context"
	"net/http"
)

// maxPages bounds cursor paging, so a provider cannot keep a sync running forever.
const maxPages = 50

// Header returns the address's headers as an http.Header.
func (a Address) Header() http.Header {
	h := http.Header{}
	for k, v := range a.Headers {
		h.Set(k, v)
	}
	return h
}

// require starts the provider if needed and checks it announced a capability.
func (p *Provider) require(ctx context.Context, capability string) error {
	if _, err := p.ensure(ctx); err != nil {
		return err
	}
	if !p.Has(capability) {
		return errorf(KindInvalid, "来源「%s」不提供这项功能", p.Title)
	}
	return nil
}

// offers starts the provider if needed and reports whether it announced a capability.
func (p *Provider) offers(ctx context.Context, capability string) (bool, error) {
	if _, err := p.ensure(ctx); err != nil {
		return false, err
	}
	return p.Has(capability), nil
}

// Courses lists every course, following cursors.
func (p *Provider) Courses(ctx context.Context) ([]Course, error) {
	var all []Course
	cursor := ""
	for page := 0; page < maxPages; page++ {
		var out struct {
			Courses []Course `json:"courses"`
			Cursor  string   `json:"cursor"`
		}
		if err := p.Call(ctx, "courses.list", map[string]any{"cursor": cursor}, &out); err != nil {
			return nil, err
		}
		all = append(all, out.Courses...)
		if out.Cursor == "" || out.Cursor == cursor {
			break
		}
		cursor = out.Cursor
	}
	return all, nil
}

// Lectures lists a course's lectures, following cursors.
func (p *Provider) Lectures(ctx context.Context, course string) ([]Lecture, error) {
	var all []Lecture
	cursor := ""
	for page := 0; page < maxPages; page++ {
		var out struct {
			Lectures []Lecture `json:"lectures"`
			Cursor   string    `json:"cursor"`
		}
		if err := p.Call(ctx, "lectures.list", map[string]any{"course": course, "cursor": cursor}, &out); err != nil {
			return nil, err
		}
		all = append(all, out.Lectures...)
		if out.Cursor == "" || out.Cursor == cursor {
			break
		}
		cursor = out.Cursor
	}
	return all, nil
}

// Media resolves a lecture's streams.
func (p *Provider) Media(ctx context.Context, course, lecture string) (*Media, error) {
	var m Media
	if err := p.Call(ctx, "media.resolve", map[string]any{"course": course, "lecture": lecture}, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// RefreshStream returns a fresh address for one stream.
func (p *Provider) RefreshStream(ctx context.Context, course, lecture, stream string) (Address, error) {
	var a Address
	err := p.Call(ctx, "media.refresh", map[string]any{"course": course, "lecture": lecture, "stream": stream}, &a)
	return a, err
}

// Slides lists a lecture's slide snapshots; a provider without slides has none.
func (p *Provider) Slides(ctx context.Context, course, lecture string) ([]Slide, error) {
	if ok, err := p.offers(ctx, CapSlides); !ok {
		return nil, err
	}
	var out struct {
		Slides []Slide `json:"slides"`
	}
	err := p.Call(ctx, "slides.list", map[string]any{"course": course, "lecture": lecture}, &out)
	return out.Slides, err
}

// Courseware lists a lecture's courseware; a provider without courseware has none.
func (p *Provider) Courseware(ctx context.Context, course, lecture string) ([]File, error) {
	if ok, err := p.offers(ctx, CapCourseware); !ok {
		return nil, err
	}
	var out struct {
		Files []File `json:"files"`
	}
	err := p.Call(ctx, "courseware.list", map[string]any{"course": course, "lecture": lecture}, &out)
	return out.Files, err
}

// CoursewareAddress returns where to fetch one courseware file.
func (p *Provider) CoursewareAddress(ctx context.Context, course, lecture string, f File) (Address, error) {
	if f.URL != "" {
		return f.Address, nil
	}
	var a Address
	err := p.Call(ctx, "courseware.resolve", map[string]any{"course": course, "lecture": lecture, "file": f.ID}, &a)
	return a, err
}
