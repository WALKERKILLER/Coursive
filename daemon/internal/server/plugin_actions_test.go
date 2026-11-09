package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/walkerkiller/coursive/daemon/internal/pluginbridge"
)

// actionRegistry is one enabled plugin whose read action patches view state
// and whose write action becomes a proposal.
type actionRegistry struct{}

func (actionRegistry) Plugin(_ context.Context, id string) (pluginbridge.Plugin, bool, error) {
	if id != "review" {
		return pluginbridge.Plugin{}, false, nil
	}
	return pluginbridge.Plugin{ID: "review", State: pluginbridge.StateEnabled,
		Permissions: []string{"views", "library.write"},
		Actions: []pluginbridge.Action{
			{ID: "refresh", Permission: "views"},
			{ID: "save", Title: "保存复习卡", Permission: "library.write", Write: true},
		}}, true, nil
}

type actionExecutor struct {
	calls  int
	result pluginbridge.Result
}

func (e *actionExecutor) Execute(context.Context, pluginbridge.Call) (pluginbridge.Result, error) {
	e.calls++
	return e.result, nil
}

type actionApprovals struct{}

func (actionApprovals) ApprovalMode(context.Context) (string, error) {
	return pluginbridge.ApprovalAsk, nil
}

func postAction(t *testing.T, s *Server, body string) (*httptest.ResponseRecorder, pluginbridge.Response) {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/plugin-actions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var resp pluginbridge.Response
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("body is not the typed envelope: %v (%s)", err, rec.Body.String())
		}
	}
	return rec, resp
}

// Without a registry the route answers a diagnosable refusal rather than
// trusting the request.
func TestPluginActionRouteUnavailable(t *testing.T) {
	s := &Server{Token: "test"}
	rec, resp := postAction(t, s, `{"pluginId":"review","actionId":"refresh","viewId":"review.main","requestId":"req-1","deadlineMs":1000}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if resp.OK || resp.Error == nil || resp.Error.Code != pluginbridge.CodeUnavailable {
		t.Fatalf("want unavailable, got %s", rec.Body.String())
	}
	if resp.RequestID != "req-1" {
		t.Fatalf("the refusal lost its request id: %s", rec.Body.String())
	}
}

// A configured bridge answers through the real route: a patch lands at the
// read revision, a stale revision is refused, and a write becomes a proposal.
func TestPluginActionRoute(t *testing.T) {
	exec := &actionExecutor{result: pluginbridge.Result{
		StatePatch: &pluginbridge.StatePatch{ViewID: "review.main", Patch: map[string]any{"page": 2.0}}}}
	s := &Server{Token: "test", PluginActions: &pluginbridge.Bridge{
		Registry: actionRegistry{}, Executor: exec, Approvals: actionApprovals{}}}

	rec, resp := postAction(t, s, `{"pluginId":"review","actionId":"refresh","viewId":"review.main","requestId":"req-1","deadlineMs":1000}`)
	if !resp.OK || resp.Result == nil || resp.Result.StatePatch == nil {
		t.Fatalf("want a state patch, got %s", rec.Body.String())
	}
	if resp.Result.StatePatch.Revision != 1 || resp.Result.StatePatch.State["page"] != 2.0 {
		t.Fatalf("patch = %+v", resp.Result.StatePatch)
	}
	if resp.Result.StatePatch.IdempotencyKey != "review:req-1" {
		t.Fatalf("idempotency key = %q", resp.Result.StatePatch.IdempotencyKey)
	}

	_, stale := postAction(t, s, `{"pluginId":"review","actionId":"refresh","viewId":"review.main","requestId":"req-2","contextRevision":0}`)
	if stale.OK || stale.Error.Code != pluginbridge.CodeRevisionConflict || !stale.Error.Recoverable {
		t.Fatalf("want a recoverable revision conflict, got %+v", stale.Error)
	}

	_, written := postAction(t, s, `{"pluginId":"review","actionId":"save","viewId":"review.main","requestId":"req-3","contextRevision":1,"args":{"cardId":"c-1"}}`)
	if !written.OK || written.Result.Proposal == nil || written.Result.Proposal.Status != pluginbridge.ProposalPending {
		t.Fatalf("want a pending proposal, got %+v", written.Result)
	}
	if exec.calls != 1 {
		t.Fatalf("executor ran %d times, want 1 (the write must not run)", exec.calls)
	}
	if _, ok := s.PluginActions.Proposal(written.Result.Proposal.ID); !ok {
		t.Fatalf("the proposal was not stored")
	}

	// A view that mounts again reads the revision its next action must name.
	req := httptest.NewRequest("GET", "/api/plugin-actions/state?pluginId=review&viewId=review.main", nil)
	req.Header.Set("Authorization", "Bearer test")
	got := httptest.NewRecorder()
	s.Handler().ServeHTTP(got, req)
	var view pluginbridge.ViewSnapshot
	if err := json.Unmarshal(got.Body.Bytes(), &view); err != nil || got.Code != http.StatusOK {
		t.Fatalf("state = %d %s", got.Code, got.Body.String())
	}
	if view.ViewID != "review.main" || view.Revision != 1 || view.State["page"] != 2.0 {
		t.Fatalf("view = %+v", view)
	}
	req = httptest.NewRequest("GET", "/api/plugin-actions/state?pluginId=review&viewId=", nil)
	req.Header.Set("Authorization", "Bearer test")
	got = httptest.NewRecorder()
	s.Handler().ServeHTTP(got, req)
	if got.Code != http.StatusBadRequest {
		t.Fatalf("a missing viewId = %d, want 400", got.Code)
	}
}

// A replayed requestId answers once: the executor runs a single time.
func TestPluginActionRouteReplay(t *testing.T) {
	exec := &actionExecutor{result: pluginbridge.Result{Notification: &pluginbridge.Notification{Text: "已同步"}}}
	s := &Server{Token: "test", PluginActions: &pluginbridge.Bridge{Registry: actionRegistry{}, Executor: exec}}
	body := `{"pluginId":"review","actionId":"refresh","viewId":"review.main","requestId":"req-1"}`
	for range 2 {
		rec, resp := postAction(t, s, body)
		if !resp.OK {
			t.Fatalf("want ok, got %s", rec.Body.String())
		}
	}
	if exec.calls != 1 {
		t.Fatalf("executor ran %d times, want 1", exec.calls)
	}
}

// A body outside the contract is a transport error: 400, not a typed answer.
func TestPluginActionRouteRejectsBadBodies(t *testing.T) {
	s := &Server{Token: "test"}
	for _, body := range []string{
		`{"pluginId":"review","actionId":"refresh","requestId":"req-1","extra":true}`,
		`not json`,
		`{"pluginId":"review","actionId":"refresh","requestId":42}`,
	} {
		rec, _ := postAction(t, s, body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", body, rec.Code)
		}
	}
}

// The route stays behind the daemon token like every other API.
func TestPluginActionRouteNeedsToken(t *testing.T) {
	s := &Server{Token: "test"}
	req := httptest.NewRequest("POST", "/api/plugin-actions", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
