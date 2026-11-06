package server

import (
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/sources"
)

// relayCache keeps each relayed lecture's resolved media for a while, so the
// player's many range requests do not resolve and sign it again each time.
type relayCache struct {
	mu sync.Mutex
	m  map[string]relayEntry
}

type relayEntry struct {
	media *sources.Media
	at    time.Time
}

const relayFresh = 10 * time.Minute

func (c *relayCache) put(id string, m *sources.Media) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]relayEntry{}
	}
	for k, e := range c.m {
		if time.Since(e.at) > relayFresh {
			delete(c.m, k)
		}
	}
	c.m[id] = relayEntry{m, time.Now()}
}

func (c *relayCache) get(id string) *sources.Media {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.m[id]; ok && time.Since(e.at) <= relayFresh {
		return e.media
	}
	return nil
}

func (c *relayCache) reset() {
	c.mu.Lock()
	c.m = nil
	c.mu.Unlock()
}

// relayClient has no overall timeout: a relayed video streams for as long as
// the player reads it.
var relayClient = &http.Client{Transport: http.DefaultTransport}

// relayed reports whether the player must get a lecture's streams through
// the daemon: a stream that needs headers keeps them in the daemon.
func relayed(m *sources.Media) bool {
	for _, st := range m.Streams {
		if len(st.Headers) > 0 {
			return true
		}
	}
	return false
}

// relayStream passes one stream of a lecture to the player when the stream
// needs headers (see relayed). Range requests pass through; a refused
// request resolves the media again once.
func (s *Server) relayStream(w http.ResponseWriter, r *http.Request) {
	ctx, id := r.Context(), r.PathValue("id")
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 0 {
		writeErr(w, badRequest("无效的视频流序号"))
		return
	}
	for attempt := 0; attempt < 2; attempt++ {
		m := s.relayed.get(id)
		if m == nil || attempt > 0 {
			l, err := s.St.Lecture(ctx, id)
			if err != nil {
				writeErr(w, err)
				return
			}
			p, err := s.source(ctx, l.CourseID)
			if err != nil {
				writeErr(w, err)
				return
			}
			if m, err = p.Media(ctx, l.CourseID, id); err != nil {
				writeErr(w, err)
				return
			}
			s.relayed.put(id, m)
		}
		if n >= len(m.Streams) {
			writeErr(w, apiError{http.StatusNotFound, "没有这个视频流"})
			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.Streams[n].URL, nil)
		if err != nil {
			writeErr(w, err)
			return
		}
		req.Header = m.Streams[n].Header()
		for _, k := range []string{"Range", "If-Range"} {
			if v := r.Header.Get(k); v != "" {
				req.Header.Set(k, v)
			}
		}
		res, err := relayClient.Do(req)
		if err != nil {
			writeErr(w, apiError{http.StatusBadGateway, "无法连接视频服务器"})
			return
		}
		if attempt == 0 && (res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden) {
			// The signature or the session lapsed; resolving the media again
			// lets the provider renew it.
			res.Body.Close()
			continue
		}
		defer res.Body.Close()
		for _, k := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Last-Modified", "ETag"} {
			if v := res.Header.Get(k); v != "" {
				w.Header().Set(k, v)
			}
		}
		w.WriteHeader(res.StatusCode)
		io.Copy(w, res.Body)
		return
	}
	writeErr(w, apiError{http.StatusBadGateway, "无法播放：视频服务器拒绝了请求，请稍后重试"})
}
