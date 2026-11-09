package pluginbridge

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRegistry map[string]Plugin

func (r fakeRegistry) Plugin(_ context.Context, id string) (Plugin, bool, error) {
	p, ok := r[id]
	return p, ok, nil
}

type fakeExecutor struct {
	mu    sync.Mutex
	calls int
	run   func(context.Context) (Result, error)
}

func (e *fakeExecutor) Execute(ctx context.Context, _ Call) (Result, error) {
	e.mu.Lock()
	e.calls++
	run := e.run
	e.mu.Unlock()
	if run == nil {
		return Result{}, nil
	}
	return run(ctx)
}

func (e *fakeExecutor) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls
}

type fakeApprovals struct {
	mode string
	err  error
}

func (a fakeApprovals) ApprovalMode(context.Context) (string, error) { return a.mode, a.err }

// testBridge is one enabled plugin with a read action, a write action and a
// UI-only action, next to a disabled plugin and one without granted rights.
func testBridge(exec *fakeExecutor) *Bridge {
	return &Bridge{
		Registry: fakeRegistry{
			"review": {ID: "review", State: StateEnabled, Permissions: []string{"views", "library.write"},
				Actions: []Action{
					{ID: "refresh", Permission: "views",
						Params: &Schema{Required: []string{"courseId"}, Properties: map[string]string{"courseId": TypeString}}},
					{ID: "save", Title: "保存复习卡", Permission: "library.write", Write: true},
					{ID: "open", Permission: "views"},
					{ID: "undeclared", Permission: "library.write", Write: true},
				}},
			"limited": {ID: "limited", State: StateReady, Permissions: []string{"views"},
				Actions: []Action{{ID: "save", Title: "保存", Permission: "library.write", Write: true}}},
			"off": {ID: "off", State: StateDisabled},
		},
		Executor:  exec,
		Approvals: fakeApprovals{mode: ApprovalAsk},
	}
}

func call(pluginID, actionID string) Request {
	return Request{PluginID: pluginID, ActionID: actionID, ViewID: "review.main", RequestID: "req-1", DeadlineMs: 1000}
}

func patch(viewID string, values map[string]any) Result {
	return Result{StatePatch: &StatePatch{ViewID: viewID, Patch: values}}
}

// A request that cannot be replayed, resolved or trusted is refused before
// any action runs.
func TestRequestValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Request)
	}{
		{"no requestId", func(r *Request) { r.RequestID = "" }},
		{"requestId with a space", func(r *Request) { r.RequestID = "req 1" }},
		{"plugin id", func(r *Request) { r.PluginID = "Review" }},
		{"action id", func(r *Request) { r.ActionID = "Refresh!" }},
		{"view id", func(r *Request) { r.ViewID = "../main" }},
		{"instance id", func(r *Request) { r.InstanceID = "page 1" }},
		{"negative revision", func(r *Request) { r.ContextRevision = -1 }},
		{"deadline over the cap", func(r *Request) { r.DeadlineMs = MaxDeadlineMs + 1 }},
		{"negative deadline", func(r *Request) { r.DeadlineMs = -5 }},
		{"trace id", func(r *Request) { r.TraceID = "trace id" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec := &fakeExecutor{}
			b := testBridge(exec)
			req := call("review", "refresh")
			req.Args = map[string]any{"courseId": "c1"}
			tc.mutate(&req)
			resp := b.Handle(context.Background(), req)
			if resp.OK || resp.Error == nil || resp.Error.Code != CodeInvalidRequest {
				t.Fatalf("want invalid_request, got %+v", resp)
			}
			if exec.count() != 0 {
				t.Fatalf("executor ran %d times for an invalid request", exec.count())
			}
		})
	}
}

// Plugin state, the declared permission and the action catalog gate every call.
func TestResolution(t *testing.T) {
	for _, tc := range []struct {
		name   string
		bridge func(*fakeExecutor) *Bridge
		req    Request
		code   string
		recov  bool
	}{
		{"no registry", func(e *fakeExecutor) *Bridge { return &Bridge{Executor: e} }, call("review", "refresh"), CodeUnavailable, false},
		{"unknown plugin", testBridge, call("ghost", "refresh"), CodeUnknownPlugin, false},
		{"disabled plugin", testBridge, call("off", "refresh"), CodePluginInactive, true},
		{"unknown action", testBridge, call("review", "ghost"), CodeUnknownAction, false},
		{"permission denied", testBridge, call("limited", "save"), CodePermissionDenied, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec := &fakeExecutor{}
			resp := tc.bridge(exec).Handle(context.Background(), tc.req)
			if resp.OK || resp.Error == nil || resp.Error.Code != tc.code {
				t.Fatalf("want %s, got %+v", tc.code, resp)
			}
			if resp.Error.Recoverable != tc.recov {
				t.Fatalf("recoverable = %v, want %v", resp.Error.Recoverable, tc.recov)
			}
			if exec.count() != 0 {
				t.Fatalf("executor ran for a refused action")
			}
		})
	}
}

// A read action with no executor cannot run, and its request is not cached as
// a success.
func TestNoExecutor(t *testing.T) {
	b := &Bridge{Registry: fakeRegistry{"review": {ID: "review", State: StateEnabled,
		Actions: []Action{{ID: "refresh"}}}}}
	resp := b.Handle(context.Background(), call("review", "refresh"))
	if resp.OK || resp.Error.Code != CodeUnavailable {
		t.Fatalf("want unavailable, got %+v", resp)
	}
}

// Arguments must match the declared shape, including undeclared keys.
func TestArgumentShape(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		code string
	}{
		{"missing required", map[string]any{}, CodeInvalidArgs},
		{"wrong type", map[string]any{"courseId": 7.0}, CodeInvalidArgs},
		{"undeclared key", map[string]any{"courseId": "c1", "extra": true}, CodeInvalidArgs},
		{"args too large", map[string]any{"courseId": strings.Repeat("x", maxArgs)}, CodeInvalidArgs},
		{"accepted", map[string]any{"courseId": "c1"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec := &fakeExecutor{}
			req := call("review", "refresh")
			req.Args = tc.args
			resp := testBridge(exec).Handle(context.Background(), req)
			if tc.code == "" {
				if !resp.OK {
					t.Fatalf("want ok, got %+v", resp.Error)
				}
				return
			}
			if resp.OK || resp.Error.Code != tc.code {
				t.Fatalf("want %s, got %+v", tc.code, resp)
			}
		})
	}
}

// A state patch lands once and moves the revision; a caller that read an older
// revision gets a recoverable conflict instead of overwriting.
func TestRevisionConflict(t *testing.T) {
	exec := &fakeExecutor{run: func(context.Context) (Result, error) {
		return patch("review.main", map[string]any{"page": 2.0}), nil
	}}
	b := testBridge(exec)
	first := call("review", "refresh")
	first.Args = map[string]any{"courseId": "c1"}
	resp := b.Handle(context.Background(), first)
	if !resp.OK || resp.Result.StatePatch == nil || resp.Result.StatePatch.Revision != 1 {
		t.Fatalf("first patch: %+v", resp)
	}
	if got := resp.Result.StatePatch.State["page"]; got != 2.0 {
		t.Fatalf("state page = %v", got)
	}

	stale := call("review", "refresh")
	stale.Args = map[string]any{"courseId": "c1"}
	stale.RequestID = "req-2"
	staleResp := b.Handle(context.Background(), stale)
	if staleResp.OK || staleResp.Error.Code != CodeRevisionConflict || !staleResp.Error.Recoverable {
		t.Fatalf("want a recoverable revision conflict, got %+v", staleResp)
	}
	if got := staleResp.Error.Details["current"]; got != int64(1) {
		t.Fatalf("conflict details current = %v", got)
	}
	if b.Revision("review", "review.main") != 1 {
		t.Fatalf("a refused patch moved the revision")
	}

	fresh := stale
	fresh.RequestID = "req-3"
	fresh.ContextRevision = 1
	if resp := b.Handle(context.Background(), fresh); !resp.OK || resp.Result.StatePatch.Revision != 2 {
		t.Fatalf("patch at the current revision: %+v", resp)
	}
}

// A repeated requestId replays the first answer without running the action
// again, so a double click or a retry cannot write twice.
func TestIdempotentReplay(t *testing.T) {
	exec := &fakeExecutor{run: func(context.Context) (Result, error) {
		return patch("review.main", map[string]any{"clicks": 1.0}), nil
	}}
	b := testBridge(exec)
	req := call("review", "refresh")
	req.Args = map[string]any{"courseId": "c1"}
	first := b.Handle(context.Background(), req)
	second := b.Handle(context.Background(), req)
	if exec.count() != 1 {
		t.Fatalf("executor ran %d times, want 1", exec.count())
	}
	if first.Result.StatePatch.Revision != second.Result.StatePatch.Revision {
		t.Fatalf("replay returned another revision: %d then %d",
			first.Result.StatePatch.Revision, second.Result.StatePatch.Revision)
	}
	if b.Revision("review", "review.main") != 1 {
		t.Fatalf("replay applied the patch again")
	}
	if first.Result.StatePatch.IdempotencyKey != "review:req-1" {
		t.Fatalf("idempotency key = %q", first.Result.StatePatch.IdempotencyKey)
	}
}

// Two concurrent requests with one requestId share one execution.
func TestConcurrentReplay(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	exec := &fakeExecutor{run: func(context.Context) (Result, error) {
		once.Do(func() { close(entered) })
		<-release
		return patch("review.main", map[string]any{"clicks": 1.0}), nil
	}}
	b := testBridge(exec)
	req := call("review", "refresh")
	req.Args = map[string]any{"courseId": "c1"}

	var wg sync.WaitGroup
	results := make([]Response, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = b.Handle(context.Background(), req)
		}(i)
	}
	<-entered
	time.Sleep(10 * time.Millisecond) // let the second caller join the first
	close(release)
	wg.Wait()
	if exec.count() != 1 {
		t.Fatalf("executor ran %d times, want 1", exec.count())
	}
	for i, resp := range results {
		if !resp.OK || resp.Result.StatePatch.Revision != 1 {
			t.Fatalf("caller %d: %+v", i, resp)
		}
	}
}

// A deadline or a cancellation drops the answer even when the executor returns
// one, so a timed-out action leaves no partial write. The failure is not
// remembered, so a retry with the same requestId runs the action again.
func TestDeadlineAndCancel(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  func() context.Context
		code string
	}{
		{"deadline", func() context.Context { return context.Background() }, CodeDeadlineExceeded},
		{"cancel", func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			go func() { time.Sleep(5 * time.Millisecond); cancel() }()
			return ctx
		}, CodeCanceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec := &fakeExecutor{run: func(ctx context.Context) (Result, error) {
				<-ctx.Done()
				return patch("review.main", map[string]any{"partial": true}), nil
			}}
			b := testBridge(exec)
			req := call("review", "refresh")
			req.Args = map[string]any{"courseId": "c1"}
			req.DeadlineMs = 20
			resp := b.Handle(tc.ctx(), req)
			if resp.OK || resp.Error.Code != tc.code || !resp.Error.Recoverable {
				t.Fatalf("want a recoverable %s, got %+v", tc.code, resp)
			}
			if b.Revision("review", "review.main") != 0 {
				t.Fatalf("a timed-out action wrote state")
			}
		})
	}
}

// A timeout leaves nothing behind, so the same requestId may try again.
func TestRetryAfterTimeout(t *testing.T) {
	calls := 0
	exec := &fakeExecutor{run: func(ctx context.Context) (Result, error) {
		calls++
		if calls == 1 {
			<-ctx.Done()
			return Result{}, ctx.Err()
		}
		return patch("review.main", map[string]any{"page": 2.0}), nil
	}}
	b := testBridge(exec)
	req := call("review", "refresh")
	req.Args = map[string]any{"courseId": "c1"}
	req.DeadlineMs = 20
	if resp := b.Handle(context.Background(), req); resp.Error == nil || resp.Error.Code != CodeDeadlineExceeded {
		t.Fatalf("want a timeout, got %+v", resp)
	}
	if resp := b.Handle(context.Background(), req); !resp.OK || resp.Result.StatePatch.Revision != 1 {
		t.Fatalf("the retry did not run: %+v", resp)
	}
	if calls != 2 {
		t.Fatalf("executor ran %d times, want 2", calls)
	}
}

// A write action answers with a pending proposal and reaches no executor; the
// existing approval path applies it later.
func TestProposalFlow(t *testing.T) {
	exec := &fakeExecutor{}
	b := testBridge(exec)
	req := call("review", "save")
	req.Args = map[string]any{"cardId": "c-1"}
	req.TraceID = "trace-1"
	resp := b.Handle(context.Background(), req)
	if !resp.OK || resp.Result.Proposal == nil {
		t.Fatalf("want a proposal, got %+v", resp)
	}
	p := resp.Result.Proposal
	if p.Status != ProposalPending || p.ApprovalMode != ApprovalAsk || p.Summary != "保存复习卡" {
		t.Fatalf("proposal = %+v", p)
	}
	if p.Preview["cardId"] != "c-1" || p.TraceID != "trace-1" || p.CreatedAt == 0 {
		t.Fatalf("proposal lost its context: %+v", p)
	}
	if exec.count() != 0 {
		t.Fatalf("a write action reached the executor")
	}
	if _, ok := b.Proposal(p.ID); !ok {
		t.Fatalf("proposal %s was not stored", p.ID)
	}

	replay := b.Handle(context.Background(), req)
	if replay.Result.Proposal.ID != p.ID {
		t.Fatalf("replay named another proposal: %s", replay.Result.Proposal.ID)
	}
}

// Writes are refused, never auto-approved, without a usable approval mode.
func TestApprovalRefused(t *testing.T) {
	for _, tc := range []struct {
		name      string
		approvals Approvals
		recov     bool
	}{
		{"no approvals", nil, false},
		{"unknown mode", fakeApprovals{mode: "sometimes"}, false},
		{"approvals error", fakeApprovals{err: errors.New("no prefs")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := testBridge(&fakeExecutor{})
			b.Approvals = tc.approvals
			resp := b.Handle(context.Background(), call("review", "save"))
			if resp.OK || resp.Error.Code != CodeApprovalUnavailable {
				t.Fatalf("want approval_unavailable, got %+v", resp)
			}
			if resp.Error.Recoverable != tc.recov {
				t.Fatalf("recoverable = %v, want %v", resp.Error.Recoverable, tc.recov)
			}
		})
	}
}

// Exactly one result form is accepted, and each form is checked.
func TestResultForms(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result Result
		code   string
	}{
		{"navigation", Result{Navigation: &Navigation{Route: "review.home", Title: "复习"}}, ""},
		{"notification", Result{Notification: &Notification{Text: "已同步", Level: "info"}}, ""},
		{"view patch", Result{ViewPatch: &ViewPatch{ViewID: "review.main", Patch: map[string]any{"title": "新"}}}, ""},
		{"no effect", Result{}, ""},
		{"two forms", Result{Navigation: &Navigation{Route: "review.home"},
			Notification: &Notification{Text: "hi"}}, CodeInvalidResult},
		{"bad route", Result{Navigation: &Navigation{Route: "Review Home"}}, CodeInvalidResult},
		{"empty notification", Result{Notification: &Notification{}}, CodeInvalidResult},
		{"bad level", Result{Notification: &Notification{Text: "x", Level: "loud"}}, CodeInvalidResult},
		{"executor proposal", Result{Proposal: &Proposal{ID: "p"}}, CodeInvalidResult},
		{"empty view patch", Result{ViewPatch: &ViewPatch{ViewID: "review.main"}}, CodeInvalidResult},
		{"patch for another view", Result{StatePatch: &StatePatch{ViewID: "other.main",
			Patch: map[string]any{"a": 1.0}}}, CodeInvalidResult},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.result
			b := testBridge(&fakeExecutor{run: func(context.Context) (Result, error) { return result, nil }})
			req := call("review", "open")
			resp := b.Handle(context.Background(), req)
			if tc.code == "" {
				if !resp.OK {
					t.Fatalf("want ok, got %+v", resp.Error)
				}
				return
			}
			if resp.OK || resp.Error.Code != tc.code {
				t.Fatalf("want %s, got %+v", tc.code, resp)
			}
		})
	}
}

// A patch sets keys, a nil value removes one, and each patch moves the
// revision once.
func TestPatchMerge(t *testing.T) {
	steps := []map[string]any{
		{"a": 1.0, "b": 2.0},
		{"a": nil, "c": 3.0},
	}
	exec := &fakeExecutor{run: func(context.Context) (Result, error) {
		step := steps[0]
		steps = steps[1:]
		return patch("", step), nil
	}}
	b := testBridge(exec)
	req := call("review", "refresh")
	req.Args = map[string]any{"courseId": "c1"}
	if resp := b.Handle(context.Background(), req); !resp.OK {
		t.Fatalf("first patch: %+v", resp.Error)
	}
	req.RequestID = "req-2"
	req.ContextRevision = 1
	resp := b.Handle(context.Background(), req)
	if !resp.OK {
		t.Fatalf("second patch: %+v", resp.Error)
	}
	state := b.State("review", "review.main")
	if _, ok := state["a"]; ok {
		t.Fatalf("nil did not remove a: %v", state)
	}
	if state["b"] != 2.0 || state["c"] != 3.0 || resp.Result.StatePatch.Revision != 2 {
		t.Fatalf("state = %v revision = %d", state, resp.Result.StatePatch.Revision)
	}
	if resp.Result.StatePatch.ViewID != "review.main" {
		t.Fatalf("the patch did not take the request's view: %q", resp.Result.StatePatch.ViewID)
	}
}

// The zero bridge is the route's safe default: it refuses instead of running.
func TestZeroBridge(t *testing.T) {
	var b Bridge
	resp := b.Handle(context.Background(), call("review", "refresh"))
	if resp.OK || resp.Error.Code != CodeUnavailable {
		t.Fatalf("want unavailable, got %+v", resp)
	}
	if resp.RequestID != "req-1" || resp.Error.Message == "" {
		t.Fatalf("the refusal lost its request id or message: %+v", resp)
	}
}

// An executor error keeps its own code, and an untyped one becomes internal.
func TestExecutorErrors(t *testing.T) {
	typed := &Error{Code: CodeInvalidArgs, Message: "插件说参数不对"}
	b := testBridge(&fakeExecutor{run: func(context.Context) (Result, error) { return Result{}, typed }})
	if resp := b.Handle(context.Background(), call("review", "open")); resp.Error.Code != CodeInvalidArgs {
		t.Fatalf("typed error lost: %+v", resp.Error)
	}
	b = testBridge(&fakeExecutor{run: func(context.Context) (Result, error) { return Result{}, errors.New("boom") }})
	resp := b.Handle(context.Background(), call("review", "open"))
	if resp.Error.Code != CodeInternal || resp.Error.Message != "boom" {
		t.Fatalf("want internal boom, got %+v", resp.Error)
	}
}
