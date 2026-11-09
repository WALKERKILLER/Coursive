// Package server exposes the daemon over a token-protected local HTTP API.
package server

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/agent"
	"github.com/walkerkiller/coursive/daemon/internal/agentenv"
	"github.com/walkerkiller/coursive/daemon/internal/asr"
	"github.com/walkerkiller/coursive/daemon/internal/datasync"
	"github.com/walkerkiller/coursive/daemon/internal/dl"
	"github.com/walkerkiller/coursive/daemon/internal/events"
	"github.com/walkerkiller/coursive/daemon/internal/jobs"
	"github.com/walkerkiller/coursive/daemon/internal/library"
	"github.com/walkerkiller/coursive/daemon/internal/lms"
	"github.com/walkerkiller/coursive/daemon/internal/localasr"
	"github.com/walkerkiller/coursive/daemon/internal/mineru"
	"github.com/walkerkiller/coursive/daemon/internal/notify"
	"github.com/walkerkiller/coursive/daemon/internal/notion"
	"github.com/walkerkiller/coursive/daemon/internal/pluginbridge"
	"github.com/walkerkiller/coursive/daemon/internal/sources"
	"github.com/walkerkiller/coursive/daemon/internal/store"
)

// Version is set at release builds with -ldflags "-X github.com/walkerkiller/coursive/daemon/internal/server.Version=…".
var Version = "0.1.0"

type Server struct {
	Token string
	// Sources runs the installed source plugins.
	Sources *sources.Registry
	// Market lists source plugins from the plugin index and installs them.
	Market *sources.Market
	St     *store.Store
	Lib    *library.Library
	Jobs   *jobs.Manager
	Eng    *dl.Engine
	Bus    *events.Bus
	Agent  *agent.Supervisor
	// Runtime installs Node.js and the agent's dependencies.
	Runtime *agentenv.Manager
	// Ctx lives as long as the daemon; background work started by a request uses it.
	Ctx context.Context
	// MinerU parses uploaded courseware.
	MinerU *mineru.Service
	// Local installs and runs on-device speech recognition models.
	Local *localasr.Manager
	// Notes is the in-app notification feed.
	Notes *notify.Feed
	// LMS keeps learning-platform data from sources that announce lms.
	LMS *lms.Service
	// Notion syncs notes and transcripts into a Notion workspace.
	Notion *notion.Service
	// Data makes backups and syncs the library over WebDAV.
	Data *datasync.Service
	// PluginActions answers declarative UI actions. A nil bridge answers
	// "unavailable" (docs/specs/0032-ui-extension-protocol.md).
	PluginActions *pluginbridge.Bridge

	syncMu        sync.Mutex
	lastCourse    time.Time
	relayed       relayCache
	slideHeaders  sync.Map // lecture id -> map[slide url]headers
	watchOnceInit sync.Once
	w             *watchState

	obsidianMu      sync.Mutex
	obsidianTimer   *time.Timer
	obsidianRunning bool // an automatic export is in progress
	obsidianPending bool // notes changed during it; export again after
	notionTimer     *time.Timer
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	h := func(pattern string, f func(http.ResponseWriter, *http.Request) (any, error)) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			v, err := f(w, r)
			if err != nil {
				writeErr(w, err)
				return
			}
			if s.Data != nil && r.Method != "GET" && !strings.HasPrefix(r.URL.Path, "/api/sync") && !strings.HasPrefix(r.URL.Path, "/api/data") {
				s.Data.Soon(s.Ctx)
			}
			if v != nil {
				writeJSON(w, http.StatusOK, v)
			}
		})
	}
	h("GET /api/health", s.health)
	mux.HandleFunc("POST /api/plugin-actions", s.pluginAction)
	mux.HandleFunc("GET /api/plugin-actions/state", s.pluginViewState)
	s.sourceRoutes(h)
	s.lmsRoutes(h)

	h("GET /api/courses", s.courses)
	h("POST /api/local-courses", s.createLocalCourse)
	h("POST /api/courses/{id}/local-videos", s.importLocalVideo)
	h("PATCH /api/courses/{id}", s.patchCourse)
	h("GET /api/courses/{id}/lectures", s.lectures)
	h("GET /api/lectures/{id}", s.lecture)
	h("GET /api/lectures/{id}/media", s.media)
	h("GET /api/lectures/{id}/slides", s.slides)
	h("GET /api/lectures/{id}/transcript", s.transcript)
	h("PUT /api/lectures/{id}/playback", s.playback)
	h("POST /api/lectures/{id}/bookmarks", s.addBookmark)
	h("DELETE /api/bookmarks/{id}", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		return ok(), s.St.DeleteBookmark(r.Context(), id)
	})
	h("GET /api/recent", func(_ http.ResponseWriter, r *http.Request) (any, error) { return s.St.RecentLectures(r.Context(), 20) })

	h("GET /api/jobs", func(_ http.ResponseWriter, r *http.Request) (any, error) { return s.Jobs.List(r.Context()) })
	h("POST /api/jobs", s.enqueue)
	h("POST /api/jobs/clear", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var freed int64
		if r.URL.Query().Get("files") == "1" {
			done, err := s.St.Jobs(r.Context(), "done")
			if err != nil {
				return nil, err
			}
			for _, j := range done {
				n, err := s.Jobs.RemoveFiles(r.Context(), j.ID)
				if err != nil {
					return map[string]any{"ok": false, "freed": freed}, err
				}
				freed += n
			}
		}
		return map[string]any{"ok": true, "freed": freed}, s.St.ClearFinishedJobs(r.Context())
	})
	h("POST /api/jobs/bulk", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			Action string   `json:"action"`
			IDs    []string `json:"ids"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if in.Action != "pause" && in.Action != "resume" && in.Action != "retry" && in.Action != "cancel" {
			return nil, badRequest("action 只能是 pause、resume、retry 或 cancel")
		}
		n, err := s.Jobs.Bulk(r.Context(), in.Action, in.IDs)
		return map[string]any{"changed": n}, err
	})
	h("POST /api/jobs/{id}/{action}", s.jobAction)
	h("DELETE /api/jobs/{id}", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		if r.URL.Query().Get("files") == "1" {
			freed, err := s.Jobs.RemoveFiles(r.Context(), r.PathValue("id"))
			return map[string]any{"ok": err == nil, "freed": freed}, err
		}
		return ok(), s.Jobs.Remove(r.Context(), r.PathValue("id"))
	})
	h("GET /api/local", func(_ http.ResponseWriter, r *http.Request) (any, error) { return s.St.LocalItems(r.Context()) })
	h("DELETE /api/lectures/{id}/assets/{kind}", s.removeAsset)

	h("GET /api/search", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		return s.St.Search(r.Context(), q.Get("q"), q.Get("course_id"), limit)
	})
	h("GET /api/notes", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		q := r.URL.Query()
		return s.St.Notes(r.Context(), q.Get("course_id"), q.Get("lecture_id"))
	})
	h("GET /api/notes/{id}", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		return s.St.Note(r.Context(), r.PathValue("id"))
	})
	h("PUT /api/notes", s.putNote)
	h("DELETE /api/notes/{id}", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		err := s.Lib.DeleteNote(r.Context(), r.PathValue("id"))
		if err == nil {
			s.Bus.Publish("notes_changed", map[string]string{"id": r.PathValue("id")})
			s.knowledgeChanged()
		}
		return ok(), err
	})

	h("GET /api/settings", func(_ http.ResponseWriter, r *http.Request) (any, error) { return s.St.Settings(r.Context()) })
	h("PUT /api/settings", s.putSettings)
	h("GET /api/library", s.library)
	h("GET /api/library/catalog", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		return s.Lib.CourseCatalog(r.Context(), r.URL.Query().Get("course_id"))
	})
	h("GET /api/lectures/{id}/transcript-review", s.transcriptReview)
	h("PUT /api/lectures/{id}/transcript-review", s.transcriptReview)
	h("PATCH /api/lectures/{id}/transcript-review", s.transcriptReview)
	h("GET /api/lectures/{id}/transcript-review/context", s.transcriptReviewContext)
	h("POST /api/library/reindex", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		n, err := s.Lib.Reindex(r.Context())
		return map[string]int{"indexed": n}, err
	})
	h("POST /api/library/export", s.export)
	h("POST /api/library/obsidian", s.exportObsidian)
	h("GET /api/notion", s.notionStatus)
	h("PUT /api/notion", s.notionConfigure)
	h("POST /api/notion/connect", s.notionConnect)
	h("POST /api/notion/disconnect", s.notionDisconnect)
	h("GET /api/notion/pages", s.notionPages)
	h("POST /api/notion/sync", s.notionSync)
	h("GET /api/asr", func(http.ResponseWriter, *http.Request) (any, error) {
		cfg, chans, events := s.Jobs.ASR.Status()
		return map[string]any{"config": cfg, "channels": chans, "events": events}, nil
	})
	h("GET /api/asr/local", s.localASRStatus)
	h("POST /api/asr/local/install", s.localASRInstall)
	h("POST /api/asr/local/cancel", func(http.ResponseWriter, *http.Request) (any, error) {
		s.Local.Cancel()
		return ok(), nil
	})
	h("DELETE /api/asr/local/models/{id}", s.localASRRemove)
	h("PUT /api/asr/local/location", s.localASRLocation)
	h("GET /api/watch", s.watchStatus)
	h("POST /api/watch/run", s.watchNow)
	h("GET /api/notifications", func(http.ResponseWriter, *http.Request) (any, error) {
		items, unread := s.Notes.List()
		return map[string]any{"items": items, "unread": unread}, nil
	})
	h("POST /api/notifications/read", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			IDs []string `json:"ids"`
			All bool     `json:"all"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if in.All {
			in.IDs = nil
		} else if len(in.IDs) == 0 {
			return nil, badRequest("ids 或 all 必填")
		}
		s.Notes.MarkRead(in.IDs)
		_, unread := s.Notes.List()
		s.Bus.Publish("notifications_read", map[string]int{"unread": unread})
		return map[string]int{"unread": unread}, nil
	})
	h("GET /api/activity", s.activity)
	h("GET /api/activity/day", s.activityDay)
	h("POST /api/activity", s.addActivity)
	h("POST /api/agent/event", s.agentEvent)
	h("GET /api/agent/runtime", func(_ http.ResponseWriter, r *http.Request) (any, error) { return s.Runtime.Status(r.Context()), nil })
	h("POST /api/agent/runtime/install", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			Mirror string `json:"mirror"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		if _, ok := agentenv.Mirrors[in.Mirror]; !ok {
			return nil, badRequest("mirror 只能是 npmmirror 或 official")
		}
		if err := s.Runtime.Start(s.Ctx, in.Mirror); errors.Is(err, agentenv.ErrRunning) {
			return nil, apiError{http.StatusConflict, err.Error()}
		} else if err != nil {
			return nil, err
		}
		return s.Runtime.Status(r.Context()), nil
	})
	h("GET /api/quizzes", s.quizzes)
	h("POST /api/quizzes", s.addQuiz)
	h("GET /api/quizzes/{id}", s.quiz)
	h("DELETE /api/quizzes/{id}", s.deleteQuiz)
	h("POST /api/quizzes/{id}/attempts", s.addQuizAttempt)
	h("GET /api/schedule", s.schedule)
	h("POST /api/schedule", s.addSchedule)
	h("PATCH /api/schedule/{id}", s.patchSchedule)
	h("DELETE /api/schedule/{id}", s.deleteSchedule)

	h("GET /api/lectures/{id}/timeline", s.timeline)
	h("PUT /api/lectures/{id}/timeline", s.putTimeline)
	h("DELETE /api/lectures/{id}/timeline", s.deleteTimeline)
	h("GET /api/lectures/{id}/study-canvas", s.studyCanvas)
	h("PUT /api/lectures/{id}/study-canvas", s.studyCanvas)
	h("POST /api/lectures/{id}/study-canvas/assets", s.studyCanvasImport)
	h("POST /api/lectures/{id}/study-canvas/context", s.studyCanvasContext)
	h("POST /api/lectures/{id}/study-canvas/attachments", s.studyAttachmentUpload)
	h("GET /api/lectures/{id}/study-canvas/attachments/{attachment}", s.studyAttachmentRead)
	h("GET /api/lectures/{id}/study-canvas/attachments/{attachment}/image", s.studyAttachmentRead)
	h("POST /api/lectures/{id}/study-canvas/sources/search", s.studyCourseSources)
	h("POST /api/lectures/{id}/study-canvas/sources/read", s.studyCourseSources)
	h("POST /api/lectures/{id}/study-canvas/sources/resolve", s.studyCourseSources)
	h("GET /api/lectures/{id}/study-canvas/conversations/{conversation}", s.studyConversation)
	h("PUT /api/lectures/{id}/study-canvas/conversations/{conversation}", s.studyConversation)
	h("POST /api/lectures/{id}/study-canvas/material", s.studyMaterial)
	h("GET /api/lectures/{id}/study-canvas/pdf-page", s.studyPDFPage)
	h("POST /api/export/pdf-preview", s.exportPDFPreview)
	h("GET /api/lectures/{id}/study-canvas/media-file/{object}", s.studyVideoFile)
	h("POST /api/lectures/{id}/study-canvas/media", s.studyMedia)
	h("GET /api/lectures/{id}/study-canvas/media/{media}", s.studyMedia)
	h("DELETE /api/lectures/{id}/study-canvas/media/{media}", s.studyMedia)
	h("GET /api/lectures/{id}/courseware", s.courseware)
	h("POST /api/lectures/{id}/courseware", s.uploadCourseware)
	h("DELETE /api/lectures/{id}/courseware", s.deleteCourseware)
	h("GET /api/lectures/{id}/courseware/text", s.coursewareText)
	h("GET /api/lectures/{id}/courseware/search", s.searchCourseware)
	mux.HandleFunc("GET /api/lectures/{id}/slides/{idx}/image", s.slideImage)
	mux.HandleFunc("GET /api/lectures/{id}/streams/{n}", s.relayStream)
	h("GET /api/storage", s.storage)
	h("POST /api/storage/clean", s.storageClean)
	mux.HandleFunc("GET /api/data/export", s.dataExport)
	h("GET /api/data/estimate", func(http.ResponseWriter, *http.Request) (any, error) { return s.Data.Estimate(), nil })
	h("POST /api/data/import", s.dataImport)
	h("GET /api/data/import/{id}", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		pv, err := s.Data.Inspect(r.PathValue("id"))
		return pv, dataErr(err)
	})
	h("POST /api/data/import/{id}/apply", s.dataRestore)
	h("DELETE /api/data/import/{id}", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		s.Data.Discard(r.PathValue("id"))
		return ok(), nil
	})
	h("GET /api/sync", func(http.ResponseWriter, *http.Request) (any, error) { return s.Data.Status(), nil })
	h("PUT /api/sync", s.syncConfigure)
	h("POST /api/sync/folder/preview", s.folderPreview)
	h("POST /api/lectures/{id}/study-canvas/draft", s.folderDraft)
	h("GET /api/sync/folder/conflicts", s.folderConflicts)
	h("GET /api/sync/folder/conflicts/{conflict}", s.folderConflict)
	h("POST /api/sync/folder/conflicts/{conflict}/resolve", s.folderConflict)
	h("POST /api/sync/auto", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			Auto bool `json:"auto"`
		}
		if err := decode(r, &in); err != nil {
			return nil, err
		}
		return s.Data.SetAuto(in.Auto), nil
	})
	h("POST /api/sync/run", s.syncRun)
	h("DELETE /api/sync", func(http.ResponseWriter, *http.Request) (any, error) {
		s.Data.Disconnect()
		return s.Data.Status(), nil
	})
	h("GET /api/mineru", s.mineruStatus)
	h("PUT /api/mineru", s.putMinerU)
	h("POST /api/mineru/test", s.testMinerU)
	h("GET /api/agent/status", func(http.ResponseWriter, *http.Request) (any, error) {
		return map[string]any{"status": s.Agent.Status(), "logs": s.Agent.Logs()}, nil
	})

	mux.HandleFunc("GET /api/events", s.events)
	mux.HandleFunc("GET /files/{rel...}", s.files)
	mux.Handle("/agent/", s.Agent)
	return s.auth(mux)
}

// auth accepts the token as a Bearer header or ?token= (for media players).
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if tok == "" {
			tok = r.URL.Query().Get("token")
		}
		if subtle.ConstantTimeCompare([]byte(tok), []byte(s.Token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

type apiError struct {
	Code int
	Msg  string
}

func (e apiError) Error() string { return e.Msg }

func badRequest(msg string) error { return apiError{http.StatusBadRequest, msg} }

func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	var ae apiError
	var se *sources.Error
	switch {
	case errors.As(err, &ae):
		code = ae.Code
	case errors.As(err, &se):
		code = map[string]int{sources.KindAuthRequired: http.StatusUnauthorized, sources.KindInvalid: http.StatusBadRequest,
			sources.KindRateLimited: http.StatusTooManyRequests, sources.KindUnavailable: http.StatusServiceUnavailable}[se.Kind]
		if code == 0 {
			code = http.StatusBadGateway
		}
		writeJSON(w, code, map[string]any{"error": se.Message, "kind": se.Kind})
		return
	case errors.Is(err, sql.ErrNoRows):
		code = http.StatusNotFound
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 8<<20)).Decode(v); err != nil {
		return badRequest("请求体无效: " + err.Error())
	}
	return nil
}

func ok() map[string]bool { return map[string]bool{"ok": true} }

func (s *Server) health(http.ResponseWriter, *http.Request) (any, error) {
	return map[string]any{"version": Version, "ffmpeg": asr.HasFFmpeg(), "agent": s.Agent.Status(),
		"sources": s.sourceState(), "library": s.Lib.Root, "media_tools": mediaTools(), "pdf_backend": pdfBackend()}, nil
}

// ---- courses ----

func (s *Server) courses(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx := r.Context()
	cs, err := s.St.Courses(ctx)
	if err != nil {
		return nil, err
	}
	if len(s.Sources.SignedIn()) == 0 {
		return s.markRefreshable(cs), nil
	}
	if r.URL.Query().Get("sync") == "1" || len(cs) == 0 {
		if err := s.syncCourses(ctx); err != nil {
			if len(cs) == 0 {
				return nil, err
			}
			slog.Warn("course sync failed, serving cache", "err", err)
		}
		cs, err = s.St.Courses(ctx)
		return s.markRefreshable(cs), err
	}
	if time.Since(s.lastCourse) > 10*time.Minute {
		go s.syncCourses(context.Background())
	}
	return s.markRefreshable(cs), nil
}

func (s *Server) markRefreshable(cs []store.Course) []store.Course {
	for i := range cs {
		cs[i].Refreshable = s.refreshable(cs[i])
	}
	return cs
}

func (s *Server) patchCourse(_ http.ResponseWriter, r *http.Request) (any, error) {
	var patch map[string]any
	if err := decode(r, &patch); err != nil {
		return nil, err
	}
	if err := s.St.PatchCourse(r.Context(), r.PathValue("id"), patch); err != nil {
		return nil, badRequest(err.Error())
	}
	s.Bus.Publish("courses_changed", nil)
	return s.St.Course(r.Context(), r.PathValue("id"))
}

func (s *Server) lectures(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, id := r.Context(), r.PathValue("id")
	ls, err := s.St.Lectures(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.URL.Query().Get("sync") == "1" || len(ls) == 0 {
		if err := s.syncLectures(ctx, id); err != nil && len(ls) == 0 {
			return nil, err
		}
		if ls, err = s.St.Lectures(ctx, id); err != nil {
			return nil, err
		}
	}
	c, err := s.St.Course(ctx, id)
	if err != nil {
		return nil, err
	}
	c.Refreshable = s.refreshable(c)
	return map[string]any{"course": c, "lectures": ls}, nil
}

func (s *Server) lecture(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, id := r.Context(), r.PathValue("id")
	l, err := s.St.Lecture(ctx, id)
	if err != nil {
		return nil, err
	}
	c, _ := s.St.Course(ctx, l.CourseID)
	assets, _ := s.St.Assets(ctx, id)
	pb, _ := s.St.Playback(ctx, id)
	bms, _ := s.St.Bookmarks(ctx, id)
	notes, _ := s.St.Notes(ctx, "", id)
	var js []store.Job
	all, _ := s.Jobs.List(ctx)
	for _, j := range all {
		if j.LectureID == id {
			js = append(js, j)
		}
	}
	tl, _ := s.St.Timeline(ctx, id)
	return map[string]any{"lecture": l, "course": c, "assets": assets, "playback": pb, "bookmarks": bms,
		"notes": notes, "jobs": js, "timeline": tl, "library_root": s.Lib.Root}, nil
}

// playStream is a stream as the player gets it: never with headers.
type playStream struct {
	ID       string `json:"id"`
	Kind     string `json:"kind,omitempty"`
	Label    string `json:"label"`
	URL      string `json:"url"`
	Thumb    string `json:"thumb,omitempty"`
	Duration int64  `json:"duration,omitempty"`
}

func (s *Server) media(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, id := r.Context(), r.PathValue("id")
	l, err := s.St.Lecture(ctx, id)
	if err != nil {
		return nil, err
	}
	localStream := s.localVideoStream(r, id)
	localResult := func() (any, error) {
		return map[string]any{"streams": []playStream{*localStream}, "duration": l.DurationS,
			"allow_drag": true, "local_path": "", "relayed": false}, nil
	}
	if store.IsLocalCourse(l.CourseID) {
		if localStream == nil {
			return nil, apiError{404, "无法播放：本地视频文件不存在，请重新导入"}
		}
		return localResult()
	}
	p, err := s.source(ctx, l.CourseID)
	if err != nil || !p.SignedIn() {
		if localStream != nil {
			return localResult()
		}
		if err != nil {
			return nil, err
		}
	}
	m, err := p.Media(ctx, l.CourseID, id)
	if err != nil {
		if localStream != nil {
			return localResult()
		}
		return nil, err
	}
	thumb := ""
	if len(m.Streams) > 0 {
		thumb = m.Streams[0].Thumb
	}
	s.St.SetLectureMedia(ctx, id, m.Duration, thumb)
	local := ""
	if as, err := s.St.Assets(ctx, id); err == nil {
		for _, a := range as {
			if a.Kind == "video" {
				if _, err := os.Stat(s.Lib.Abs(a.Path)); err == nil {
					local = s.Lib.Abs(a.Path)
				}
			}
		}
	}
	relay := relayed(m)
	if relay {
		// The streams need headers, which stay in the daemon: the player
		// gets daemon's relay instead (see relayStream).
		s.relayed.put(id, m)
	}
	streams := make([]playStream, len(m.Streams))
	for i, st := range m.Streams {
		streams[i] = playStream{ID: st.ID, Kind: st.Kind, Label: st.Label, URL: st.URL, Thumb: st.Thumb, Duration: st.Duration}
		if relay {
			streams[i].URL = fmt.Sprintf("/api/lectures/%s/streams/%d", id, i)
		}
	}
	return map[string]any{"streams": streams, "duration": m.Duration, "allow_drag": m.Seekable,
		"local_path": local, "relayed": relay}, nil
}

// Downloaded video remains readable after the source signs out. The existing
// authenticated /files endpoint confines it to the library and supports Range.
func (s *Server) localVideoStream(r *http.Request, id string) *playStream {
	assets, err := s.St.Assets(r.Context(), id)
	if err != nil {
		return nil
	}
	root, err := os.OpenRoot(s.Lib.Root)
	if err != nil {
		return nil
	}
	defer root.Close()
	for _, a := range assets {
		if a.Kind != "video" {
			continue
		}
		info, err := root.Stat(a.Path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		parts := strings.Split(filepath.ToSlash(a.Path), "/")
		for i := range parts {
			parts[i] = url.PathEscape(parts[i])
		}
		return &playStream{ID: "local", Label: "本机录像", URL: fmt.Sprintf("http://%s/files/%s?token=%s", r.Host, strings.Join(parts, "/"), s.Token)}
	}
	return nil
}

// slides returns downloaded slides, or the remote list (indexed without
// files) so the player can show slides while streaming.
func (s *Server) slides(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, id := r.Context(), r.PathValue("id")
	sl, err := s.lectureSlides(ctx, id)
	if err != nil {
		return nil, err
	}
	type out struct {
		store.Slide
		Abs string `json:"abs,omitempty"`
	}
	res := make([]out, len(sl))
	for i, x := range sl {
		res[i] = out{Slide: x}
		if x.Path != "" {
			res[i].Abs = s.Lib.Abs(x.Path)
		}
	}
	return res, nil
}

// lectureSlides returns the indexed slides, fetching the remote list (and
// indexing it without files) the first time.
func (s *Server) lectureSlides(ctx context.Context, id string) ([]store.Slide, error) {
	sl, err := s.St.Slides(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(sl) == 0 {
		l, err := s.St.Lecture(ctx, id)
		if err != nil {
			return nil, err
		}
		if store.IsLocalCourse(l.CourseID) {
			return sl, nil
		}
		snaps, err := s.remoteSlides(ctx, l.CourseID, id)
		if err != nil {
			return nil, err
		}
		for i, sn := range snaps {
			sl = append(sl, store.Slide{Idx: i + 1, TSec: sn.TSec, IsKey: sn.Key, RemoteURL: sn.URL})
		}
		if len(sl) > 0 {
			s.St.ReplaceSlides(ctx, id, sl)
		}
	}
	return sl, nil
}

// remoteSlides lists a lecture's slides from its source and keeps their
// headers in memory for slideImage.
func (s *Server) remoteSlides(ctx context.Context, course, lecture string) ([]sources.Slide, error) {
	p, err := s.source(ctx, course)
	if err != nil {
		return nil, err
	}
	snaps, err := p.Slides(ctx, course, lecture)
	if err != nil {
		return nil, err
	}
	hs := map[string]map[string]string{}
	for _, sn := range snaps {
		if len(sn.Headers) > 0 {
			hs[sn.URL] = sn.Headers
		}
	}
	s.slideHeaders.Store(lecture, hs)
	return snaps, nil
}

// slideHeader returns the headers a remote slide image needs.
func (s *Server) slideHeader(ctx context.Context, lecture, u string) http.Header {
	v, ok := s.slideHeaders.Load(lecture)
	if !ok {
		if l, err := s.St.Lecture(ctx, lecture); err == nil {
			s.remoteSlides(ctx, l.CourseID, lecture)
		}
		v, _ = s.slideHeaders.Load(lecture)
	}
	hs, _ := v.(map[string]map[string]string)
	return sources.Address{Headers: hs[u]}.Header()
}

var slideClient = &http.Client{Timeout: 30 * time.Second}

// slideImage serves one slide: the downloaded file, or the source's image
// fetched on the fly (for the assistant and for notes that embed slides).
func (s *Server) slideImage(w http.ResponseWriter, r *http.Request) {
	ctx, id := r.Context(), r.PathValue("id")
	idx, _ := strconv.Atoi(r.PathValue("idx"))
	sl, err := s.lectureSlides(ctx, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	for _, x := range sl {
		if x.Idx != idx {
			continue
		}
		w.Header().Set("Cache-Control", "private, max-age=86400")
		if x.Path != "" {
			if abs := s.Lib.Abs(x.Path); fileExists(abs) {
				http.ServeFile(w, r, abs)
				return
			}
		}
		if !strings.HasPrefix(x.RemoteURL, "https://") && !strings.HasPrefix(x.RemoteURL, "http://") {
			break
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, x.RemoteURL, nil)
		req.Header = s.slideHeader(ctx, id, x.RemoteURL)
		res, err := slideClient.Do(req)
		if err != nil {
			writeErr(w, apiError{http.StatusBadGateway, "无法获取 PPT 图片"})
			return
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			writeErr(w, apiError{http.StatusBadGateway, fmt.Sprintf("无法获取 PPT 图片：HTTP %d", res.StatusCode)})
			return
		}
		if ct := res.Header.Get("Content-Type"); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		io.Copy(w, io.LimitReader(res.Body, 20<<20))
		return
	}
	writeErr(w, apiError{http.StatusNotFound, "没有这张 PPT"})
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func (s *Server) transcript(_ http.ResponseWriter, r *http.Request) (any, error) {
	q := r.URL.Query()
	from, _ := strconv.ParseInt(q.Get("from_ms"), 10, 64)
	to, _ := strconv.ParseInt(q.Get("to_ms"), 10, 64)
	return s.St.Segments(r.Context(), r.PathValue("id"), from, to)
}

func (s *Server) playback(_ http.ResponseWriter, r *http.Request) (any, error) {
	var p store.Playback
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	return ok(), s.St.SavePlayback(r.Context(), r.PathValue("id"), p)
}

func (s *Server) addBookmark(_ http.ResponseWriter, r *http.Request) (any, error) {
	var b store.Bookmark
	if err := decode(r, &b); err != nil {
		return nil, err
	}
	b.LectureID = r.PathValue("id")
	id, err := s.St.AddBookmark(r.Context(), b)
	b.ID = id
	return b, err
}

// ---- jobs ----

func (s *Server) removeAsset(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, id, kind := r.Context(), r.PathValue("id"), r.PathValue("kind")
	valid := false
	for _, k := range jobs.Kinds {
		valid = valid || k == kind
	}
	if !valid {
		return nil, badRequest("未知类型 " + kind)
	}
	l, err := s.St.Lecture(ctx, id)
	if err != nil {
		return nil, err
	}
	jid := id + "-" + kind
	if j, err := s.St.Job(ctx, jid); err == nil && (j.State == "running" || j.State == "queued") {
		return nil, badRequest("该任务正在进行，请先取消")
	}
	freed, err := s.Lib.RemoveAsset(ctx, id, kind)
	if err != nil {
		return nil, err
	}
	if j, err := s.St.Job(ctx, jid); err == nil && j.State == "done" {
		s.Jobs.Remove(ctx, jid)
	}
	s.Bus.Publish("lecture_changed", map[string]string{"lecture_id": id, "course_id": l.CourseID})
	return map[string]any{"ok": true, "freed": freed}, nil
}

func (s *Server) enqueue(_ http.ResponseWriter, r *http.Request) (any, error) {
	var in struct {
		LectureIDs []string `json:"lecture_ids"`
		Kinds      []string `json:"kinds"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if len(in.LectureIDs) == 0 || len(in.Kinds) == 0 {
		return nil, badRequest("lecture_ids 和 kinds 不能为空")
	}
	var out []store.Job
	for _, id := range in.LectureIDs {
		js, err := s.Jobs.Enqueue(r.Context(), id, in.Kinds)
		if err != nil {
			return nil, badRequest(err.Error())
		}
		out = append(out, js...)
	}
	return out, nil
}

func (s *Server) jobAction(_ http.ResponseWriter, r *http.Request) (any, error) {
	ctx, id := r.Context(), r.PathValue("id")
	switch r.PathValue("action") {
	case "pause":
		return ok(), s.Jobs.Pause(ctx, id)
	case "resume", "retry":
		return ok(), s.Jobs.Resume(ctx, id)
	case "cancel":
		return ok(), s.Jobs.Cancel(ctx, id)
	}
	return nil, badRequest("未知操作")
}

// ---- notes ----

func (s *Server) putNote(_ http.ResponseWriter, r *http.Request) (any, error) {
	var n store.Note
	if err := decode(r, &n); err != nil {
		return nil, err
	}
	n, err := s.Lib.SaveNote(r.Context(), n)
	if errors.Is(err, library.ErrStudyConflict) {
		return nil, apiError{409, "其他设备已修改此笔记，请另存本机编辑"}
	}
	if err == nil {
		s.Bus.Publish("notes_changed", map[string]string{"id": n.ID, "lecture_id": n.LectureID, "course_id": n.CourseID})
		s.knowledgeChanged()
	}
	return n, err
}

// ---- settings & library ----

// ApplySettings pushes persisted settings into running components.
func (s *Server) ApplySettings(ctx context.Context) {
	kv, err := s.St.Settings(ctx)
	if err != nil {
		return
	}
	get := func(k string, v any) bool { raw, ok := kv[k]; return ok && json.Unmarshal(raw, v) == nil }
	var n int64
	if get("download.max_conns", &n) {
		s.Eng.SetMaxConns(int(n))
	}
	if get("download.rate_limit", &n) {
		s.Eng.SetRateLimit(n)
	} else {
		s.Eng.SetRateLimit(0)
	}
	cfg := jobs.Config{}
	if get("download.video_parallel", &n) {
		cfg.VideoParallel = int(n)
	}
	get("download.stream", &cfg.Stream)
	get("transcript.delete_video", &cfg.DeleteVideoAfterTranscript)
	s.Jobs.SetConfig(cfg)
	var pc asr.PoolConfig
	get("asr.disabled", &pc.Disabled)
	get("asr.strategy", &pc.Strategy)
	if get("asr.concurrency", &n) {
		pc.Concurrency = int(n)
	}
	s.Jobs.ASR.SetConfig(pc)
	s.applyLocalASR(ctx)
	var tpl library.Templates
	if get("library.templates", &tpl) {
		s.Lib.SetTemplates(tpl)
	}
}

func (s *Server) putSettings(_ http.ResponseWriter, r *http.Request) (any, error) {
	var kv map[string]json.RawMessage
	if err := decode(r, &kv); err != nil {
		return nil, err
	}
	if err := s.St.PutSettings(r.Context(), kv); err != nil {
		return nil, badRequest(err.Error())
	}
	s.ApplySettings(r.Context())
	s.Bus.Publish("settings_changed", kv)
	return s.St.Settings(r.Context())
}

func (s *Server) library(_ http.ResponseWriter, r *http.Request) (any, error) {
	st, err := s.St.Stats(r.Context())
	return map[string]any{"root": s.Lib.Root, "stats": st}, err
}

func (s *Server) export(_ http.ResponseWriter, r *http.Request) (any, error) {
	var in struct {
		Root string `json:"root"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.Root == "" || !filepath.IsAbs(in.Root) {
		return nil, badRequest("请提供绝对路径")
	}
	n, err := s.Lib.ExportSkill(r.Context(), in.Root)
	return map[string]any{"exported": n, "root": in.Root}, err
}

// obsidianTarget reads the saved Obsidian export settings.
func (s *Server) obsidianTarget(ctx context.Context) (vault, folder string, auto bool) {
	kv, err := s.St.Settings(ctx)
	if err != nil {
		return "", "", false
	}
	json.Unmarshal(kv["obsidian.vault"], &vault)
	json.Unmarshal(kv["obsidian.folder"], &folder)
	json.Unmarshal(kv["obsidian.auto"], &auto)
	return vault, folder, auto
}

// exportObsidian refreshes the Obsidian export, into the vault named in the
// body or else the saved one.
func (s *Server) exportObsidian(_ http.ResponseWriter, r *http.Request) (any, error) {
	var in struct {
		Vault  string `json:"vault"`
		Folder string `json:"folder"`
	}
	if err := decode(r, &in); err != nil {
		return nil, err
	}
	if in.Vault == "" {
		in.Vault, in.Folder, _ = s.obsidianTarget(r.Context())
	}
	if _, err := s.Lib.ObsidianRoot(in.Vault, in.Folder); err != nil {
		return nil, badRequest(err.Error())
	}
	return s.Lib.ExportObsidian(r.Context(), in.Vault, in.Folder)
}

// exportObsidianSoon refreshes the Obsidian export shortly after notes
// change, when automatic export is on. Bursts of saves export once; while
// an export runs, further changes queue a single follow-up instead of
// piling up goroutines. Settings are read when the export starts, so
// turning automatic export off or changing the vault takes effect at once.
func (s *Server) exportObsidianSoon() {
	if _, _, auto := s.obsidianTarget(context.Background()); !auto {
		return
	}
	s.obsidianMu.Lock()
	defer s.obsidianMu.Unlock()
	if s.obsidianRunning {
		s.obsidianPending = true
		return
	}
	if s.obsidianTimer != nil {
		s.obsidianTimer.Stop()
	}
	s.obsidianTimer = time.AfterFunc(3*time.Second, s.runObsidianExport)
}

func (s *Server) runObsidianExport() {
	s.obsidianMu.Lock()
	if s.obsidianRunning {
		s.obsidianPending = true
		s.obsidianMu.Unlock()
		return
	}
	s.obsidianRunning = true
	s.obsidianMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	if vault, folder, auto := s.obsidianTarget(ctx); auto && vault != "" {
		res, err := s.Lib.ExportObsidian(ctx, vault, folder)
		event := map[string]any{"result": res}
		if err != nil {
			slog.Warn("obsidian export", "err", err)
			event["error"] = err.Error()
		}
		s.Bus.Publish("obsidian_exported", event)
	}
	cancel()

	s.obsidianMu.Lock()
	s.obsidianRunning = false
	again := s.obsidianPending
	s.obsidianPending = false
	s.obsidianMu.Unlock()
	if again {
		s.exportObsidianSoon()
	}
}

// ---- events & files ----

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, okf := w.(http.Flusher)
	if !okf {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	ch, unsub := s.Bus.Subscribe()
	defer unsub()
	hello, _ := json.Marshal(map[string]any{"type": "hello", "data": map[string]any{"sources": s.sourceState(), "agent": s.Agent.Status()}})
	fmt.Fprintf(w, "data: %s\n\n", hello)
	fl.Flush()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			fl.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		}
	}
}

func (s *Server) files(w http.ResponseWriter, r *http.Request) {
	rel := path.Clean("/" + r.PathValue("rel"))
	abs := s.Lib.Abs(strings.TrimPrefix(rel, "/"))
	if !strings.HasPrefix(abs, filepath.Clean(s.Lib.Root)+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(abs, ".db") || strings.Contains(abs, "library.db") {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, abs)
}

// mediaTools reports executables available to the same process that runs jobs.
// Presence is not a decoder, model or network connectivity test.
func mediaTools() map[string]bool {
	tools := make(map[string]bool)
	for _, name := range []string{"ffmpeg", "ffprobe", "pdfinfo", "pdftoppm", "pdftotext"} {
		_, err := exec.LookPath(name)
		tools[name] = err == nil
	}
	tools["study_pdf"] = false
	switch pdfBackend() {
	case "pdfkit":
		if exe, err := os.Executable(); err == nil {
			_, err = exec.LookPath(filepath.Join(filepath.Dir(exe), "coursive-study-pdf"))
			tools["study_pdf"] = err == nil
		}
	case "poppler":
		tools["study_pdf"] = tools["pdfinfo"] && tools["pdftoppm"] && tools["pdftotext"]
	}
	return tools
}

func pdfBackend() string {
	switch runtime.GOOS {
	case "darwin":
		return "pdfkit"
	case "linux":
		return "poppler"
	default:
		return "unsupported"
	}
}
