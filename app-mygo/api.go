package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Course is one row of the daemon's course list. The fields are the daemon's
// JSON (daemon/internal/store/store.go, type Course); the client reads them as
// they come and never renames them.
type Course struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Teacher       string  `json:"teacher,omitempty"`
	College       string  `json:"college,omitempty"`
	Term          string  `json:"term,omitempty"`
	Thumb         string  `json:"thumb,omitempty"`
	LectureTotal  int     `json:"lecture_total,omitempty"`
	Progress      float64 `json:"progress,omitempty"`
	LastLectureID string  `json:"last_lecture_id,omitempty"`
	LastLearnAt   string  `json:"last_learn_at,omitempty"`
	URL           string  `json:"url,omitempty"`

	// The fields the card draws from, which the daemon sends only when they
	// differ from their zero value.
	RawTitle      string `json:"raw_title,omitempty"`
	Color         string `json:"color,omitempty"`
	Hidden        bool   `json:"hidden,omitempty"`
	Pinned        bool   `json:"pinned,omitempty"`
	LocalLectures int    `json:"local_lectures,omitempty"`
}

// client speaks the daemon's HTTP API with the token the daemon was started
// with, as the Flutter client's Api does: JSON in, JSON out, a bearer header,
// and an {"error": "..."} body on a failed status.
type client struct {
	base  string
	token string
	http  *http.Client
}

func newClient(port int, token string) *client {
	return &client{
		base:  "http://127.0.0.1:" + strconv.Itoa(port),
		token: token,
		http:  &http.Client{Timeout: 90 * time.Second},
	}
}

// get decodes the daemon's JSON answer into out.
func (c *client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return c.do(ctx, http.MethodGet, u, nil, out)
}

// post sends body as JSON and decodes the answer into out.
func (c *client) post(ctx context.Context, path string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, c.base+path, payload, out)
}

func (c *client) do(ctx context.Context, method, u string, payload []byte, out any) error {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &e) == nil && e.Error != "" {
			return fmt.Errorf("%s: %s", res.Status, e.Error)
		}
		return fmt.Errorf("%s", res.Status)
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}

// courses is GET /api/courses, the library's course list.
func (c *client) courses(ctx context.Context) ([]Course, error) {
	var cs []Course
	err := c.get(ctx, "/api/courses", nil, &cs)
	return cs, err
}

// createLocalCourse is POST /api/local-courses: a course that is not from a
// source platform, which is how the prototype fills a throwaway library.
func (c *client) createLocalCourse(ctx context.Context, title, teacher string) (Course, error) {
	var out Course
	body := map[string]string{"title": title, "teacher": teacher}
	err := c.post(ctx, "/api/local-courses", body, &out)
	return out, err
}

// health is GET /api/health, the liveness check the app shows while it waits.
func (c *client) health(ctx context.Context) error {
	return c.get(ctx, "/api/health", nil, nil)
}
