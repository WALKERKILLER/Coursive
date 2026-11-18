package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"time"
)

// ActivityTracker measures study time and reports it to the daemon for the
// activity heatmap (docs/specs/0001-learning-activity-heatmap.md), as
// activity_tracker.dart does. Every tick it credits running clocks, and
// every minute it sends the totals. It runs on the UI thread.
type ActivityTracker struct {
	app       *AppState
	clocks    map[*ActivityClock]bool
	pending   map[bucketKey]*bucket
	visible   bool
	focused   bool
	lastFlush time.Time
	flushing  bool
}

const (
	activityTick = 10 * time.Second
	flushEvery   = 60 * time.Second
	idleAfter    = 2 * time.Minute
)

type bucketKey struct{ kind, lecture, day string }

type bucket struct {
	at      time.Time
	seconds float64
	count   int
	unsent  J
}

// Tracker is the app's tracker; nil until main starts it.
var Tracker *ActivityTracker

// NewActivityTracker starts a tracker for app.
func NewActivityTracker(app *AppState) *ActivityTracker {
	t := &ActivityTracker{app: app, clocks: map[*ActivityClock]bool{}, pending: map[bucketKey]*bucket{}, visible: true, focused: true}
	go func() {
		for range time.Tick(activityTick) {
			Post(t.check)
		}
	}()
	return t
}

// SetWindow tells the tracker whether the window is shown and has focus,
// as the app lifecycle does; a change closes the intervals first.
func (t *ActivityTracker) SetWindow(visible, focused bool) {
	if visible == t.visible && focused == t.focused {
		return
	}
	now := time.Now()
	for c := range t.clocks {
		c.credit(now)
	}
	t.visible, t.focused = visible, focused
	if !visible {
		t.Flush()
	}
}

// Clock is a clock for one surface. Interactive clocks (reading, practice)
// need focus and recent interaction; others (watching) need visibility.
func (t *ActivityTracker) Clock(kind, lectureID string, interactive bool) *ActivityClock {
	return &ActivityClock{t: t, Kind: kind, LectureID: lectureID, Interactive: interactive}
}

// Count counts a discrete event, such as a question to the assistant.
func (t *ActivityTracker) Count(kind, lectureID string) {
	t.bucket(kind, lectureID, time.Now()).count++
}

func (t *ActivityTracker) bucket(kind, lecture string, at time.Time) *bucket {
	k := bucketKey{kind, lecture, fmt.Sprintf("%d-%d-%d", at.Year(), at.Month(), at.Day())}
	b := t.pending[k]
	if b == nil {
		b = &bucket{}
		t.pending[k] = b
	}
	b.at = at
	return b
}

func (t *ActivityTracker) check() {
	now := time.Now()
	for c := range t.clocks {
		c.credit(now)
	}
	if t.lastFlush.IsZero() || now.Sub(t.lastFlush) >= flushEvery {
		t.Flush()
	}
}

func reportID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Flush sends accumulated time and counts. A failed report stays queued for
// a day and keeps its id, so a retry after a lost reply counts once.
func (t *ActivityTracker) Flush() {
	if t.flushing || !t.app.Connected() {
		return
	}
	t.flushing = true
	now := time.Now()
	t.lastFlush = now
	type item struct {
		k bucketKey
		b *bucket
	}
	var items []item
	for k, b := range t.pending {
		if now.Sub(b.at) > 23*time.Hour {
			delete(t.pending, k)
			continue
		}
		items = append(items, item{k, b})
	}
	// Reports are made on the UI thread and sent on another goroutine one
	// after another; the buckets change only back on the UI thread.
	api := t.app.Api()
	var next func(i int)
	next = func(i int) {
		if i >= len(items) {
			t.flushing = false
			return
		}
		k, b := items[i].k, items[i].b
		r := b.unsent
		if r == nil {
			at := b.at.Unix()
			switch {
			case b.seconds >= 1:
				r = J{"kind": k.kind, "seconds": min(int(math.Floor(b.seconds)), 300), "at": at}
			case b.count > 0:
				r = J{"kind": k.kind, "count": min(b.count, 50), "at": at}
			default:
				if b.seconds < 1 && b.count == 0 && !t.clockRunning(k) {
					delete(t.pending, k)
				}
				next(i + 1)
				return
			}
			if k.lecture != "" {
				r["lecture_id"] = k.lecture
			}
			r["id"] = reportID()
			b.unsent = r
		}
		Go(func() error { _, err := api.Post("/api/activity", r); return err }, func(err error) {
			if err != nil {
				log.Printf("activity flush failed: %v", err)
				t.flushing = false
				return
			}
			b.unsent = nil
			if s, ok := r["seconds"].(int); ok {
				b.seconds -= float64(s)
			}
			if c, ok := r["count"].(int); ok {
				b.count -= c
			}
			next(i)
		})
	}
	next(0)
}

func (t *ActivityTracker) clockRunning(k bucketKey) bool {
	for c := range t.clocks {
		if c.Kind == k.kind && c.LectureID == k.lecture {
			return true
		}
	}
	return false
}

// ActivityClock is time on one surface (the player, a reader, a quiz).
type ActivityClock struct {
	t           *ActivityTracker
	Kind        string
	LectureID   string
	Interactive bool
	since       time.Time
	touched     time.Time
}

func (c *ActivityClock) Running() bool { return !c.since.IsZero() }

func (c *ActivityClock) Start() {
	if c == nil || c.Running() {
		return
	}
	c.since = time.Now()
	if c.Interactive {
		c.touched = time.Time{}
	} else {
		c.touched = c.since
	}
	c.t.clocks[c] = true
}

func (c *ActivityClock) Stop() {
	if c == nil || !c.Running() {
		return
	}
	c.credit(time.Now())
	c.since = time.Time{}
	delete(c.t.clocks, c)
}

// Touch marks user interaction; the first starts an interactive clock.
func (c *ActivityClock) Touch() {
	if c == nil {
		return
	}
	now := time.Now()
	if c.Interactive && c.touched.IsZero() && c.Running() {
		c.since = now
	}
	c.touched = now
}

func (c *ActivityClock) credit(now time.Time) {
	since := c.since
	if since.IsZero() {
		return
	}
	c.since = now
	end := now
	if c.Interactive {
		if !c.t.focused || c.touched.IsZero() {
			return
		}
		if idle := c.touched.Add(idleAfter); idle.Before(end) {
			end = idle
		}
	} else if !c.t.visible {
		return
	}
	s := end.Sub(since).Seconds()
	s = math.Min(s, 2*activityTick.Seconds())
	if s > 0 {
		c.t.bucket(c.Kind, c.LectureID, end).seconds += s
	}
}
