package pluginbridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Bridge answers declarative UI actions. Its zero value is safe: with no
// Registry it reports "unavailable" instead of running anything, so the route
// can be registered before the Plugin Fabric registry exists.
type Bridge struct {
	// Registry is the daemon-owned plugin state and the actions a plugin declared.
	Registry Registry
	// Executor runs read actions. Write actions never reach it.
	Executor Executor
	// Approvals reports the user's approval mode for write actions.
	Approvals Approvals
	// MaxDeadlineMs overrides MaxDeadlineMs for this bridge (tests use a short one).
	MaxDeadlineMs int64
	// Now is the clock; nil uses time.Now.
	Now func() time.Time

	mu        sync.Mutex
	views     map[string]viewState
	responses map[string]Response
	order     []string
	proposals map[string]*proposalRec
	inflight  map[string]*pending
}

type viewState struct {
	revision int64
	state    map[string]any
}

// pending is one action in flight, so two requests with the same requestId
// wait for the first answer instead of running the action twice.
type pending struct {
	done chan struct{}
	resp Response
}

// Handle validates and answers one action. A repeated requestId replays the
// first answer without running the action again.
func (b *Bridge) Handle(ctx context.Context, req Request) Response {
	if err := req.validate(); err != nil {
		return Response{RequestID: req.RequestID, TraceID: req.TraceID, Error: err}
	}
	key := req.PluginID + "\x00" + req.RequestID

	b.mu.Lock()
	if resp, ok := b.responses[key]; ok {
		b.mu.Unlock()
		return resp
	}
	if p, ok := b.inflight[key]; ok {
		b.mu.Unlock()
		select {
		case <-p.done:
			return p.resp
		case <-ctx.Done():
			return Response{RequestID: req.RequestID, TraceID: req.TraceID,
				Error: contextErr(ctx, req.deadlineMs(b.maxDeadline()))}
		}
	}
	p := &pending{done: make(chan struct{})}
	if b.inflight == nil {
		b.inflight = map[string]*pending{}
	}
	b.inflight[key] = p
	b.mu.Unlock()

	resp := b.run(ctx, req)

	b.mu.Lock()
	delete(b.inflight, key)
	b.rememberLocked(key, resp)
	b.mu.Unlock()
	p.resp = resp
	close(p.done)
	return resp
}

// run resolves the plugin, checks the declared permission and the revision,
// then runs a read action or turns a write into a pending proposal.
func (b *Bridge) run(ctx context.Context, req Request) Response {
	resp := Response{RequestID: req.RequestID, TraceID: req.TraceID}
	plugin, action, err := b.resolve(ctx, req)
	if err != nil {
		return fail(resp, err)
	}
	if err := req.validateArgs(action); err != nil {
		return fail(resp, err)
	}
	if err := b.checkRevision(req); err != nil {
		return fail(resp, err)
	}
	deadlineMs := req.deadlineMs(b.maxDeadline())
	deadline := b.now().Add(time.Duration(deadlineMs) * time.Millisecond)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	call := Call{Plugin: plugin, Action: action, Request: req, Deadline: deadline,
		IdempotencyKey: req.PluginID + ":" + req.RequestID}
	if req.ViewID != "" {
		call.State = b.State(req.PluginID, req.ViewID)
	}

	if action.Write {
		proposal, err := b.propose(ctx, call)
		if err != nil {
			return fail(resp, err)
		}
		return succeed(resp, Result{Proposal: &proposal})
	}
	if b.Executor == nil {
		return fail(resp, &Error{Code: CodeUnavailable,
			Message: "插件动作桥没有执行器，无法运行 " + req.ActionID})
	}
	result, xerr := b.Executor.Execute(ctx, call)
	// A deadline or cancellation commits nothing: the result is dropped even
	// when the executor answered anyway.
	if ctx.Err() != nil {
		return fail(resp, contextErr(ctx, deadlineMs))
	}
	if xerr != nil {
		return fail(resp, execErr(xerr))
	}
	if err := b.checkResult(req, &result); err != nil {
		return fail(resp, err)
	}
	return succeed(resp, result)
}

// resolve finds the plugin and action, refusing a plugin that is not usable
// and an action whose declared permission the user did not grant.
func (b *Bridge) resolve(ctx context.Context, req Request) (Plugin, Action, *Error) {
	if b.Registry == nil {
		return Plugin{}, Action{}, &Error{Code: CodeUnavailable,
			Message: "插件动作桥未配置：守护进程还没有插件状态来源"}
	}
	plugin, ok, err := b.Registry.Plugin(ctx, req.PluginID)
	if err != nil {
		return Plugin{}, Action{}, &Error{Code: CodeUnavailable, Recoverable: true,
			Message: "无法读取插件状态：" + err.Error(), Details: map[string]any{"pluginId": req.PluginID}}
	}
	if !ok {
		return Plugin{}, Action{}, &Error{Code: CodeUnknownPlugin, Message: "没有安装插件 " + req.PluginID,
			Details: map[string]any{"pluginId": req.PluginID}}
	}
	switch plugin.State {
	case StateEnabled, StateReady:
	default:
		return Plugin{}, Action{}, &Error{Code: CodePluginInactive,
			Message: fmt.Sprintf("插件 %s 当前不可用（%s）", req.PluginID, plugin.State), Recoverable: true,
			Details: map[string]any{"pluginId": req.PluginID, "state": plugin.State}}
	}
	for _, action := range plugin.Actions {
		if action.ID != req.ActionID {
			continue
		}
		if action.Permission != "" && !granted(plugin.Permissions, action.Permission) {
			return Plugin{}, Action{}, &Error{Code: CodePermissionDenied,
				Message: fmt.Sprintf("插件 %s 没有获得 %s 权限", req.PluginID, action.Permission),
				Details: map[string]any{"pluginId": req.PluginID, "permission": action.Permission, "actionId": action.ID}}
		}
		return plugin, action, nil
	}
	return Plugin{}, Action{}, &Error{Code: CodeUnknownAction,
		Message: fmt.Sprintf("插件 %s 没有动作 %s", req.PluginID, req.ActionID),
		Details: map[string]any{"pluginId": req.PluginID, "actionId": req.ActionID}}
}

// checkRevision refuses an action whose view moved on since the caller read it.
func (b *Bridge) checkRevision(req Request) *Error {
	if req.ViewID == "" {
		return nil
	}
	b.mu.Lock()
	cur := b.views[viewKey(req.PluginID, req.ViewID)].revision
	b.mu.Unlock()
	if cur == req.ContextRevision {
		return nil
	}
	return revisionErr(req.ViewID, req.ContextRevision, cur)
}

// propose stores a pending proposal. Nothing is written: the existing
// approval path applies it, and the approval mode is recorded, never assumed.
func (b *Bridge) propose(ctx context.Context, call Call) (Proposal, *Error) {
	if b.Approvals == nil {
		return Proposal{}, &Error{Code: CodeApprovalUnavailable,
			Message: "写操作需要审批模式，但守护进程没有审批来源"}
	}
	mode, err := b.Approvals.ApprovalMode(ctx)
	if err != nil {
		return Proposal{}, &Error{Code: CodeApprovalUnavailable, Recoverable: true,
			Message: "无法读取审批模式：" + err.Error()}
	}
	if mode != ApprovalAuto && mode != ApprovalAsk {
		return Proposal{}, &Error{Code: CodeApprovalUnavailable,
			Message: "未知的审批模式：" + mode, Details: map[string]any{"mode": mode}}
	}
	summary := call.Action.Title
	if summary == "" {
		summary = call.Action.ID
	}
	proposal := Proposal{ID: proposalID(call.Request.PluginID, call.Request.RequestID),
		PluginID: call.Request.PluginID, ActionID: call.Action.ID, ViewID: call.Request.ViewID,
		InstanceID: call.Request.InstanceID, Summary: summary, Preview: call.Request.Args,
		ApprovalMode: mode, Status: ProposalPending, RequestID: call.Request.RequestID,
		TraceID: call.Request.TraceID, CreatedAt: b.now().Unix()}
	b.mu.Lock()
	if b.proposals == nil {
		b.proposals = map[string]*proposalRec{}
	}
	if prev, ok := b.proposals[proposal.ID]; ok {
		// A replay after the answer was forgotten names the same proposal.
		b.mu.Unlock()
		return prev.p, nil
	}
	b.proposals[proposal.ID] = &proposalRec{p: proposal, req: call.Request}
	b.mu.Unlock()
	return proposal, nil
}

// checkResult validates the answer's shape and applies a state patch at the
// revision the caller read.
func (b *Bridge) checkResult(req Request, result *Result) *Error {
	forms := 0
	for _, set := range []bool{result.StatePatch != nil, result.Navigation != nil, result.ViewPatch != nil,
		result.Notification != nil, result.Proposal != nil} {
		if set {
			forms++
		}
	}
	if forms > 1 {
		return &Error{Code: CodeInvalidResult, Message: "一个动作只能返回一种结果"}
	}
	if n := result.Navigation; n != nil {
		switch {
		case !routeRE.MatchString(n.Route):
			return &Error{Code: CodeInvalidResult, Message: "navigation.route 无效：" + n.Route}
		case len([]rune(n.Title)) > 120:
			return &Error{Code: CodeInvalidResult, Message: "navigation.title 过长"}
		}
	}
	if n := result.Notification; n != nil {
		switch {
		case n.Text == "":
			return &Error{Code: CodeInvalidResult, Message: "notification.text 不能为空"}
		case len([]rune(n.Text)) > 500:
			return &Error{Code: CodeInvalidResult, Message: "notification.text 过长"}
		case n.Level != "" && n.Level != "info" && n.Level != "warning" && n.Level != "error":
			return &Error{Code: CodeInvalidResult, Message: "notification.level 无效：" + n.Level}
		}
	}
	if result.Proposal != nil {
		return &Error{Code: CodeInvalidResult, Message: "写操作必须声明为 write，不能由执行器返回 proposal"}
	}
	if p := result.ViewPatch; p != nil {
		if p.ViewID == "" || len(p.Patch) == 0 {
			return &Error{Code: CodeInvalidResult, Message: "viewPatch 需要 viewId 和 patch"}
		}
	}
	if p := result.StatePatch; p != nil {
		viewID := p.ViewID
		if viewID == "" {
			viewID = req.ViewID
		}
		switch {
		case viewID == "":
			return &Error{Code: CodeInvalidResult, Message: "statePatch 需要 viewId"}
		case !viewIDRE.MatchString(viewID):
			return &Error{Code: CodeInvalidResult, Message: "statePatch.viewId 无效：" + viewID}
		case req.ViewID != "" && viewID != req.ViewID:
			return &Error{Code: CodeInvalidResult,
				Message: fmt.Sprintf("statePatch.viewId %s 与请求的 %s 不一致", viewID, req.ViewID)}
		}
		applied, err := b.applyPatch(req, viewID, p.Patch)
		if err != nil {
			return err
		}
		*result.StatePatch = applied
	}
	return nil
}

// applyPatch merges a patch into the view state and bumps the revision. The
// revision check repeats under the lock, so two patches from the same stale
// view cannot both land.
func (b *Bridge) applyPatch(req Request, viewID string, patch map[string]any) (StatePatch, *Error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.views == nil {
		b.views = map[string]viewState{}
	}
	key := viewKey(req.PluginID, viewID)
	cur := b.views[key]
	if cur.revision != req.ContextRevision {
		return StatePatch{}, revisionErr(viewID, req.ContextRevision, cur.revision)
	}
	state := make(map[string]any, len(cur.state)+len(patch))
	for k, v := range cur.state {
		state[k] = v
	}
	for k, v := range patch {
		if v == nil {
			delete(state, k)
			continue
		}
		state[k] = v
	}
	revision := cur.revision + 1
	b.views[key] = viewState{revision: revision, state: state}
	return StatePatch{ViewID: viewID, Patch: patch, Revision: revision,
		IdempotencyKey: req.PluginID + ":" + req.RequestID, State: state}, nil
}

// Revision is the current revision of a plugin view's state.
func (b *Bridge) Revision(pluginID, viewID string) int64 {
	return b.snapshot(pluginID, viewID).Revision
}

// State is the current state of a plugin view.
func (b *Bridge) State(pluginID, viewID string) map[string]any {
	return b.snapshot(pluginID, viewID).State
}

// View is a view's state and revision read together, so a view that mounts
// again names the revision its next action must carry.
func (b *Bridge) View(pluginID, viewID string) (ViewSnapshot, *Error) {
	switch {
	case !pluginIDRE.MatchString(pluginID):
		return ViewSnapshot{}, invalid("pluginId 无效：%q", pluginID)
	case !viewIDRE.MatchString(viewID):
		return ViewSnapshot{}, invalid("viewId 无效：%q", viewID)
	}
	return b.snapshot(pluginID, viewID), nil
}

func (b *Bridge) snapshot(pluginID, viewID string) ViewSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	cur := b.views[viewKey(pluginID, viewID)]
	out := make(map[string]any, len(cur.state))
	for k, v := range cur.state {
		out[k] = v
	}
	return ViewSnapshot{ViewID: viewID, Revision: cur.revision, State: out}
}

// Proposal returns a stored proposal.
func (b *Bridge) Proposal(id string) (Proposal, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if rec, ok := b.proposals[id]; ok {
		return rec.p, true
	}
	return Proposal{}, false
}

// rememberLocked keeps the answer for a requestId, dropping the oldest entry
// past keep so a long session cannot grow without bound. A timeout or a
// cancellation is not remembered: nothing was written, so a retry with the
// same requestId may run the action again.
func (b *Bridge) rememberLocked(key string, resp Response) {
	if resp.Error != nil && (resp.Error.Code == CodeDeadlineExceeded || resp.Error.Code == CodeCanceled) {
		return
	}
	if b.responses == nil {
		b.responses = map[string]Response{}
	}
	if _, seen := b.responses[key]; !seen {
		b.order = append(b.order, key)
	}
	b.responses[key] = resp
	for len(b.order) > keep {
		delete(b.responses, b.order[0])
		b.order = b.order[1:]
	}
}

func (b *Bridge) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func (b *Bridge) maxDeadline() int64 {
	if b.MaxDeadlineMs > 0 && b.MaxDeadlineMs < MaxDeadlineMs {
		return b.MaxDeadlineMs
	}
	return MaxDeadlineMs
}

func succeed(resp Response, result Result) Response {
	resp.OK, resp.Result = true, &result
	return resp
}

func fail(resp Response, err *Error) Response {
	resp.OK, resp.Result, resp.Error = false, nil, err
	return resp
}

// revisionErr is the recoverable refusal of a view that moved on.
func revisionErr(viewID string, want, current int64) *Error {
	return &Error{Code: CodeRevisionConflict, Recoverable: true,
		Message: fmt.Sprintf("视图 %s 已更新到版本 %d，请重新读取后再试", viewID, current),
		Details: map[string]any{"viewId": viewID, "expected": want, "current": current}}
}

// execErr keeps a typed executor error and otherwise reports it as internal.
func execErr(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: CodeInternal, Message: err.Error()}
}

func granted(permissions []string, want string) bool {
	for _, p := range permissions {
		if p == want {
			return true
		}
	}
	return false
}

func viewKey(pluginID, viewID string) string { return pluginID + "\x00" + viewID }

// proposalID is derived from the request, so a replay names the same proposal.
func proposalID(pluginID, requestID string) string {
	sum := sha256.Sum256([]byte(pluginID + "\x00" + requestID))
	return "proposal-" + hex.EncodeToString(sum[:8])
}
