package server

import (
	"encoding/json"
	"net/http"

	"github.com/walkerkiller/coursive/daemon/internal/pluginbridge"
)

// Plugin actions: a declarative view sends one action, and the bridge answers
// with at most one of statePatch, navigation, viewPatch, notification or a
// pending proposal.
// Contract: docs/specs/0032-ui-extension-protocol.md (Action bridge and UI state).
//
// A handled outcome is HTTP 200 with the typed envelope, so a stale revision
// or a denied permission reaches the app with its code and details instead of
// collapsing into an HTTP status. A malformed body is the only 4xx.
func (s *Server) pluginAction(w http.ResponseWriter, r *http.Request) {
	var req pluginbridge.Request
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeErr(w, badRequest("动作请求无效: "+err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, s.pluginBridge().Handle(r.Context(), req))
}

// pluginViewState answers a view's state and revision:
// GET /api/plugin-actions/state?pluginId=…&viewId=….
func (s *Server) pluginViewState(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	view, err := s.pluginBridge().View(q.Get("pluginId"), q.Get("viewId"))
	if err != nil {
		writeErr(w, badRequest(err.Message))
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// pluginBridge is the configured bridge; without one (tests, or a daemon
// built without the assistant host) the zero bridge refuses every action
// instead of trusting the request.
func (s *Server) pluginBridge() *pluginbridge.Bridge {
	if s.PluginActions != nil {
		return s.PluginActions
	}
	return &pluginbridge.Bridge{}
}
