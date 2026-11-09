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
	bridge := s.PluginActions
	if bridge == nil {
		// Until the Plugin Fabric registry and the Pi action executor are
		// wired in, the route says so instead of trusting the request.
		bridge = &pluginbridge.Bridge{}
	}
	writeJSON(w, http.StatusOK, bridge.Handle(r.Context(), req))
}
