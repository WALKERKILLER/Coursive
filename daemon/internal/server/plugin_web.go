package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/walkerkiller/coursive/daemon/internal/pluginbridge"
	"github.com/walkerkiller/coursive/daemon/internal/pluginfabric"
)

// Plugin Web UI pages (L3): the daemon publishes the platform admission, builds
// the page a renderer loads, answers the renderer's navigation questions and
// verifies every page message before the action bridge sees it.
// Contract: docs/specs/0032-ui-extension-protocol.md (Isolated Web UI).
//
// Like the action routes, a handled outcome is HTTP 200 with a typed body, so a
// fallback or a refusal reaches the app with its code. The page itself never
// calls these routes: it has no daemon address and no token, and the app relays
// what its `CoursiveBridge` channel receives.

// WebUIPages serves the resources of a web-ui page.
type WebUIPages interface {
	WebPage(ctx context.Context, pluginID, pageID string) (pluginfabric.WebUIBundle, error)
}

// maxWebSessions bounds open pages; the oldest is closed first.
const maxWebSessions = 16

type webSession struct {
	session pluginfabric.WebUISession
	pageID  string
	hosts   []string
}

type webSessions struct {
	mu     sync.Mutex
	byID   map[string]*webSession
	order  []string
	bridge pluginfabric.WebUIBridge
}

func (ws *webSessions) open(s *webSession) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.byID == nil {
		ws.byID = map[string]*webSession{}
	}
	ws.byID[s.session.InstanceID] = s
	ws.order = append(ws.order, s.session.InstanceID)
	for len(ws.order) > maxWebSessions {
		delete(ws.byID, ws.order[0])
		ws.order = ws.order[1:]
	}
}

func (ws *webSessions) get(id string) *webSession {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.byID[id]
}

func (ws *webSessions) close(id string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	delete(ws.byID, id)
	for i, at := range ws.order {
		if at == id {
			ws.order = append(ws.order[:i], ws.order[i+1:]...)
			break
		}
	}
}

// pluginWebAdmission publishes the admission matrix and whether this daemon
// runs the experimental preview.
func (s *Server) pluginWebAdmission(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"platforms": pluginfabric.WebUIAdmissions(), "preview": s.PluginWebPreview})
}

type webOpenRequest struct {
	PluginID string `json:"pluginId"`
	PageID   string `json:"pageId"`
	Platform string `json:"platform"`
}

type webFallback struct {
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

type webSessionView struct {
	ID         string `json:"id"`
	PluginID   string `json:"pluginId"`
	PageID     string `json:"pageId"`
	InstanceID string `json:"instanceId"`
	Origin     string `json:"origin"`
	BaseURL    string `json:"baseUrl"`
}

type webOpenResponse struct {
	Admission pluginfabric.WebUIAdmission `json:"admission"`
	Preview   bool                        `json:"preview"`
	Fallback  *webFallback                `json:"fallback,omitempty"`
	Session   *webSessionView             `json:"session,omitempty"`
	HTML      string                      `json:"html,omitempty"`
}

// pluginWebOpen opens one page for the app's platform: a fallback reason when
// the platform, the plugin or its resources are not ready, otherwise a fresh
// session and the page built for it.
func (s *Server) pluginWebOpen(w http.ResponseWriter, r *http.Request) {
	var req webOpenRequest
	if err := decodeStrict(r.Body, 4<<10, &req); err != nil {
		writeErr(w, badRequest("页面请求无效: "+err.Error()))
		return
	}
	out := webOpenResponse{Admission: pluginfabric.WebUIAdmissionFor(req.Platform)}
	fallback := func(reason, detail string) {
		out.Fallback = &webFallback{Reason: reason, Detail: detail}
		writeJSON(w, http.StatusOK, out)
	}
	switch {
	case out.Admission.Available():
	case s.PluginWebPreview && out.Admission.Previewable():
		out.Preview = true
	default:
		fallback(out.Admission.FallbackReason(), "")
		return
	}
	plugin, ferr := s.webPlugin(r.Context(), req.PluginID)
	if ferr != nil {
		fallback(pluginfabric.FallbackPluginInactive, ferr.Message)
		return
	}
	if s.PluginWebPages == nil {
		fallback(pluginfabric.FallbackResourcesMissing, "守护进程没有网页资源来源")
		return
	}
	bundle, err := s.PluginWebPages.WebPage(r.Context(), plugin.ID, req.PageID)
	if err == nil {
		err = bundle.Check(plugin.ID, req.PageID)
	}
	if err != nil {
		fallback(pluginfabric.FallbackResourcesMissing, err.Error())
		return
	}
	session, err := pluginfabric.NewWebUISession(plugin.ID)
	if err == nil {
		out.HTML, err = pluginfabric.BuildWebUIPage(bundle, session)
	}
	if err != nil {
		fallback(pluginfabric.FallbackResourcesMissing, err.Error())
		return
	}
	s.web.open(&webSession{session: session, pageID: req.PageID, hosts: bundle.Hosts})
	out.Session = &webSessionView{ID: session.InstanceID, PluginID: plugin.ID, PageID: req.PageID,
		InstanceID: session.InstanceID, Origin: session.Origin, BaseURL: session.Origin + "/"}
	writeJSON(w, http.StatusOK, out)
}

// pluginWebNavigate answers what the renderer does with a URL the page asks to
// open: stay, open in the system browser after the user confirms, or refuse.
func (s *Server) pluginWebNavigate(w http.ResponseWriter, r *http.Request) {
	rec := s.web.get(r.PathValue("id"))
	if rec == nil {
		writeErr(w, apiError{http.StatusNotFound, "页面会话不存在或已关闭"})
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := decodeStrict(r.Body, 8<<10, &req); err != nil {
		writeErr(w, badRequest("导航请求无效: "+err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, pluginfabric.CheckWebUIURL(rec.session.Origin, rec.hosts, req.URL))
}

// pluginWebMessage verifies one message the renderer received from its page —
// the frame it came from, then origin, plugin, instance, nonce, schema version,
// request id and the action's own permission against the session — and hands
// a verified one to the action bridge, which runs a read or answers a write
// with a pending proposal. The answer carries the page's own request id.
func (s *Server) pluginWebMessage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FrameURL string `json:"frameUrl"`
		Message  string `json:"message"`
	}
	if err := decodeStrict(r.Body, 512<<10, &req); err != nil {
		writeErr(w, badRequest("页面消息无效: "+err.Error()))
		return
	}
	var msg pluginfabric.WebUIBridgeMessage
	refuse := func(e *pluginfabric.WebUIError) {
		writeJSON(w, http.StatusOK, pluginbridge.Response{RequestID: msg.RequestID, TraceID: msg.TraceID,
			Error: &pluginbridge.Error{Code: e.Code, Message: e.Message, Details: e.Details, Recoverable: e.Recoverable}})
	}
	rec := s.web.get(r.PathValue("id"))
	if rec == nil {
		refuse(&pluginfabric.WebUIError{Code: pluginfabric.WebUICodeInvalidSession, Message: "页面会话不存在或已关闭"})
		return
	}
	if origin, ok := pluginfabric.WebUIFrameOrigin(req.FrameURL); !ok || origin != rec.session.Origin {
		refuse(&pluginfabric.WebUIError{Code: pluginfabric.WebUICodeCrossOrigin,
			Message: "消息不是来自这个页面的 Origin", Details: map[string]any{"expected": rec.session.Origin}})
		return
	}
	if err := decodeStrict(strings.NewReader(req.Message), 256<<10, &msg); err != nil {
		refuse(&pluginfabric.WebUIError{Code: pluginfabric.WebUICodeInvalidRequest, Message: "消息无法解析: " + err.Error()})
		return
	}
	plugin, ferr := s.webPlugin(r.Context(), rec.session.PluginID)
	if ferr != nil {
		refuse(ferr)
		return
	}
	action, verr := s.web.bridge.Verify(rec.session, plugin, msg)
	if verr != nil {
		refuse(verr)
		return
	}
	// The bridge keys idempotency by request id; the forwarded id is bound to
	// this page instance, so two pages cannot collide on one id.
	sum := sha256.Sum256([]byte(rec.session.InstanceID + "\x00" + msg.RequestID))
	resp := s.pluginBridge().Handle(r.Context(), pluginbridge.Request{PluginID: plugin.ID, ActionID: action.ID,
		InstanceID: rec.session.InstanceID, Args: msg.Args, RequestID: "web-" + hex.EncodeToString(sum[:16]),
		DeadlineMs: msg.DeadlineMs, TraceID: msg.TraceID})
	resp.RequestID = msg.RequestID
	writeJSON(w, http.StatusOK, resp)
}

// pluginWebClose forgets a page session; its later messages are refused.
func (s *Server) pluginWebClose(w http.ResponseWriter, r *http.Request) {
	s.web.close(r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// webPlugin reads the plugin from the bridge's registry: it must be usable, and
// its granted permissions and declared actions are what each message is
// checked against.
func (s *Server) webPlugin(ctx context.Context, id string) (pluginfabric.WebUIPlugin, *pluginfabric.WebUIError) {
	inactive := func(msg string) *pluginfabric.WebUIError {
		return &pluginfabric.WebUIError{Code: pluginbridge.CodePluginInactive, Message: msg, Details: map[string]any{"pluginId": id}}
	}
	registry := s.pluginBridge().Registry
	if registry == nil {
		return pluginfabric.WebUIPlugin{}, inactive("守护进程还没有插件状态来源")
	}
	p, ok, err := registry.Plugin(ctx, id)
	switch {
	case err != nil:
		return pluginfabric.WebUIPlugin{}, inactive("无法读取插件状态：" + err.Error())
	case !ok:
		return pluginfabric.WebUIPlugin{}, inactive("没有安装插件 " + id)
	case p.State != pluginbridge.StateEnabled && p.State != pluginbridge.StateReady:
		return pluginfabric.WebUIPlugin{}, inactive("插件 " + id + " 当前不可用（" + p.State + "）")
	}
	out := pluginfabric.WebUIPlugin{ID: p.ID, Permissions: p.Permissions}
	for _, a := range p.Actions {
		out.Actions = append(out.Actions, pluginfabric.WebUIAction{ID: a.ID, Title: a.Title, Permission: a.Permission, Write: a.Write})
	}
	return out, nil
}

// decodeStrict decodes one JSON value of at most limit bytes and refuses
// unknown fields.
func decodeStrict(r io.Reader, limit int64, v any) error {
	dec := json.NewDecoder(io.LimitReader(r, limit+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return io.ErrUnexpectedEOF
	}
	return nil
}
