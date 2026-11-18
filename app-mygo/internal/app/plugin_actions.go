package app

import (
	"math/rand/v2"
	"net/url"
	"strconv"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/core"
)

// The typed action bridge of the UI Extension Protocol: a declarative view
// sends {pluginId, actionId, viewId, instanceId, args, contextRevision,
// requestId} with a deadline and a trace id, and the daemon answers with
// one of statePatch, navigation, viewPatch, notification or a pending
// proposal (docs/specs/0032-ui-extension-protocol.md).
//
// This client only moves the typed envelope and hands each result form to
// a callback; how a view is drawn stays with the renderer.

// PluginActionRequest is one action a view sends.
type PluginActionRequest struct {
	PluginID, ActionID string
	// ViewID is the declarative view the action came from; its revision is
	// the one ContextRevision carries.
	ViewID string
	// InstanceID is one mounted instance of the view.
	InstanceID string
	Args       core.J
	// ContextRevision is the revision the caller read; the daemon refuses
	// a stale one.
	ContextRevision int
	// RequestID is reused by retries and repeated clicks, so the daemon
	// answers once; empty takes a fresh one.
	RequestID  string
	DeadlineMs int
	TraceID    string
}

// pluginActionDeadlineMs is the daemon's default deadline.
const pluginActionDeadlineMs = 10000

// JSON is the request body.
func (r PluginActionRequest) JSON() core.J {
	args := r.Args
	if args == nil {
		args = core.J{}
	}
	return core.J{
		"pluginId": r.PluginID, "actionId": r.ActionID, "viewId": r.ViewID, "instanceId": r.InstanceID,
		"args": args, "contextRevision": r.ContextRevision, "requestId": r.RequestID,
		"deadlineMs": r.DeadlineMs, "traceId": r.TraceID,
	}
}

// newPluginActionRequestID is a fresh request id; the daemon replays an
// answer for a repeated one.
func newPluginActionRequestID() string {
	return "req-" + strconv.FormatInt(time.Now().UnixMicro(), 36) + "-" + strconv.FormatUint(uint64(rand.Uint32()), 36)
}

// PluginStatePatch is a view state patch the daemon applied; State is the
// merged result at Revision.
type PluginStatePatch struct {
	ViewID         string
	Revision       int
	IdempotencyKey string
	Patch, State   core.J
}

// PluginNavigation asks to open a route the host already knows.
type PluginNavigation struct {
	Route, Title string
	Replace      bool
}

// PluginViewPatch is a partial replacement of a rendered view.
type PluginViewPatch struct {
	ViewID   string
	Revision int
	Patch    core.J
}

// PluginNotification is a short message for the user; Level is info,
// warning or error.
type PluginNotification struct{ Text, Level string }

// PluginProposal is a write the user has not authorized yet. It is shown,
// never applied: the existing approval path commits it.
type PluginProposal struct {
	ID, PluginID, ActionID, ViewID string
	Summary, ApprovalMode, Status  string
	Preview                        core.J
}

// PluginActionFailure is a refused action. Recoverable marks the cases a
// view can retry after reloading its state.
type PluginActionFailure struct {
	Code, Message string
	Details       core.J
	Recoverable   bool
}

func (f *PluginActionFailure) Error() string {
	if f.Message == "" {
		return f.Code
	}
	return f.Message
}

// PluginActionOutcome is the daemon's answer: a result form or a typed
// error, never both.
type PluginActionOutcome struct {
	RequestID, TraceID string
	StatePatch         *PluginStatePatch
	Navigation         *PluginNavigation
	ViewPatch          *PluginViewPatch
	Notification       *PluginNotification
	Proposal           *PluginProposal
	// Failure is set exactly when the action was refused.
	Failure *PluginActionFailure
}

// OK is whether the daemon accepted the action.
func (o PluginActionOutcome) OK() bool { return o.Failure == nil }

func objOrEmpty(v any) core.J {
	if m := core.Obj(v); m != nil {
		return m
	}
	return core.J{}
}

func parsePluginActionOutcome(j core.J) PluginActionOutcome {
	o := PluginActionOutcome{RequestID: j.S("requestId"), TraceID: j.S("traceId")}
	if r := j.O("result"); r != nil {
		if m := r.O("statePatch"); m != nil {
			o.StatePatch = &PluginStatePatch{ViewID: m.S("viewId"), Revision: m.I("revision"), IdempotencyKey: m.S("idempotencyKey"),
				Patch: objOrEmpty(m["patch"]), State: objOrEmpty(m["state"])}
		}
		if m := r.O("navigation"); m != nil {
			o.Navigation = &PluginNavigation{Route: m.S("route"), Title: m.S("title"), Replace: m.IsTrue("replace")}
		}
		if m := r.O("viewPatch"); m != nil {
			o.ViewPatch = &PluginViewPatch{ViewID: m.S("viewId"), Revision: m.I("revision"), Patch: objOrEmpty(m["patch"])}
		}
		if m := r.O("notification"); m != nil {
			o.Notification = &PluginNotification{Text: m.S("text"), Level: m.Or("level", "info")}
		}
		if m := r.O("proposal"); m != nil {
			o.Proposal = parsePluginProposal(m)
		}
	}
	if m := j.O("error"); m != nil {
		o.Failure = &PluginActionFailure{Code: m.S("code"), Message: m.S("message"), Details: objOrEmpty(m["details"]), Recoverable: m.IsTrue("recoverable")}
	}
	return o
}

func parsePluginProposal(m core.J) *PluginProposal {
	return &PluginProposal{ID: m.S("id"), PluginID: m.S("pluginId"), ActionID: m.S("actionId"), ViewID: m.S("viewId"),
		Summary: m.S("summary"), ApprovalMode: m.S("approvalMode"), Status: m.S("status"), Preview: objOrEmpty(m["preview"])}
}

// PluginActionClient sends typed actions through the daemon and applies
// each result form to its callbacks. A request id is sent at most once: a
// repeated click or a retry gets the first answer, and a settled answer is
// replayed without a second dispatch. Use it on the UI thread.
type PluginActionClient struct {
	OnStatePatch   func(PluginStatePatch)
	OnNavigation   func(PluginNavigation)
	OnViewPatch    func(PluginViewPatch)
	OnNotification func(PluginNotification)
	OnProposal     func(PluginProposal)
	// Keep is how many settled answers are remembered for replay.
	Keep int

	inFlight  map[string][]func(PluginActionOutcome, error)
	settled   map[string]PluginActionOutcome
	order     []string
	revisions map[string]int
}

// RevisionOf is the last revision the daemon reported for a plugin view;
// send it as the next request's ContextRevision.
func (cl *PluginActionClient) RevisionOf(pluginID, viewID string) int {
	return cl.revisions[pluginID+"/"+viewID]
}

func (cl *PluginActionClient) setRevision(pluginID, viewID string, rev int) {
	if cl.revisions == nil {
		cl.revisions = map[string]int{}
	}
	cl.revisions[pluginID+"/"+viewID] = rev
}

// ReadState reads a view's state and revision from the daemon and keeps
// the revision, so a view that mounts again is not refused as stale.
func (cl *PluginActionClient) ReadState(pluginID, viewID string, done func(state core.J, err error)) {
	a := api()
	core.Async(func() (any, error) {
		return a.Get("/api/plugin-actions/state", map[string]string{"pluginId": pluginID, "viewId": viewID})
	}, func(v any, err error) {
		if err != nil {
			done(nil, err)
			return
		}
		m := objOrEmpty(v)
		cl.setRevision(pluginID, m.Or("viewId", viewID), m.I("revision"))
		done(objOrEmpty(m["state"]), nil)
	})
}

// Apply runs a pending write the user approved; the daemon checks the
// plugin and its permission again and answers like an action.
func (cl *PluginActionClient) Apply(p PluginProposal, done func(PluginActionOutcome, error)) {
	cl.settle(p, "apply", done)
}

// Reject discards a pending write without running it.
func (cl *PluginActionClient) Reject(p PluginProposal, done func(PluginActionOutcome, error)) {
	cl.settle(p, "reject", done)
}

func (cl *PluginActionClient) settle(p PluginProposal, verb string, done func(PluginActionOutcome, error)) {
	a := api()
	core.Async(func() (any, error) {
		return a.Post("/api/plugin-actions/proposals/"+url.PathEscape(p.ID)+"/"+verb, nil)
	}, func(v any, err error) {
		if err != nil {
			done(PluginActionOutcome{}, err)
			return
		}
		o := parsePluginActionOutcome(objOrEmpty(v))
		if o.OK() && verb == "apply" {
			cl.dispatch(PluginActionRequest{PluginID: p.PluginID, ActionID: p.ActionID, ViewID: p.ViewID, RequestID: o.RequestID}, o)
		}
		done(o, nil)
	})
}

// Invoke sends req once. The answer is dispatched to the callbacks; a
// transport failure (no answer at all) is passed on, not remembered, so a
// retry can still reach the daemon.
func (cl *PluginActionClient) Invoke(req PluginActionRequest, done func(PluginActionOutcome, error)) {
	if req.RequestID == "" {
		req.RequestID = newPluginActionRequestID()
	}
	if req.DeadlineMs == 0 {
		req.DeadlineMs = pluginActionDeadlineMs
	}
	if o, ok := cl.settled[req.RequestID]; ok {
		done(o, nil)
		return
	}
	if cl.inFlight == nil {
		cl.inFlight = map[string][]func(PluginActionOutcome, error){}
	}
	if waiting, ok := cl.inFlight[req.RequestID]; ok {
		cl.inFlight[req.RequestID] = append(waiting, done)
		return
	}
	cl.inFlight[req.RequestID] = []func(PluginActionOutcome, error){done}
	a := api()
	body := req.JSON()
	core.Async(func() (any, error) { return a.Post("/api/plugin-actions", body) }, func(v any, err error) {
		waiting := cl.inFlight[req.RequestID]
		delete(cl.inFlight, req.RequestID)
		var o PluginActionOutcome
		if err == nil {
			o = parsePluginActionOutcome(objOrEmpty(v))
			// A timeout or a cancellation wrote nothing, so it is not settled:
			// a retry with the same requestId may still reach the daemon.
			if f := o.Failure; f == nil || (f.Code != "deadline_exceeded" && f.Code != "canceled") {
				cl.remember(req.RequestID, o)
			}
			if o.OK() {
				cl.dispatch(req, o)
			}
		}
		for _, w := range waiting {
			w(o, err)
		}
	})
}

func (cl *PluginActionClient) remember(id string, o PluginActionOutcome) {
	if cl.settled == nil {
		cl.settled = map[string]PluginActionOutcome{}
	}
	keep := cl.Keep
	if keep == 0 {
		keep = 64
	}
	cl.settled[id] = o
	cl.order = append(cl.order, id)
	if len(cl.order) > keep {
		delete(cl.settled, cl.order[0])
		cl.order = cl.order[1:]
	}
}

// dispatch applies the one result form the daemon allows, keeping the
// view revision current so the next action is not refused as stale.
func (cl *PluginActionClient) dispatch(req PluginActionRequest, o PluginActionOutcome) {
	if p := o.StatePatch; p != nil {
		cl.setRevision(req.PluginID, p.ViewID, p.Revision)
		if cl.OnStatePatch != nil {
			cl.OnStatePatch(*p)
		}
	}
	if p := o.ViewPatch; p != nil {
		cl.setRevision(req.PluginID, p.ViewID, p.Revision)
		if cl.OnViewPatch != nil {
			cl.OnViewPatch(*p)
		}
	}
	if n := o.Navigation; n != nil && cl.OnNavigation != nil {
		cl.OnNavigation(*n)
	}
	if n := o.Notification; n != nil && cl.OnNotification != nil {
		cl.OnNotification(*n)
	}
	if p := o.Proposal; p != nil && cl.OnProposal != nil {
		cl.OnProposal(*p)
	}
}
