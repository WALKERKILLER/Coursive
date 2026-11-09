package pluginbridge

import (
	"context"
	"time"
)

// proposalRec is a stored proposal with the request that made it, so an
// approval runs exactly the write the user saw.
type proposalRec struct {
	p        Proposal
	req      Request
	applying bool
}

// Apply runs a pending write after the user approved it. The plugin must still
// be usable and hold the permission; the Executor runs the write once, with
// Call.Approved set. A timeout, a cancellation or a recoverable refusal leaves
// the proposal pending for another try; any other failure settles it as
// failed. A settled proposal answers proposal_settled and never runs again.
func (b *Bridge) Apply(ctx context.Context, id string) Response {
	rec, perr := b.claim(id)
	if perr != nil {
		return Response{Error: perr}
	}
	req := rec.req
	resp := Response{RequestID: req.RequestID, TraceID: req.TraceID}
	result, err := b.applyWrite(ctx, rec)
	b.mu.Lock()
	defer b.mu.Unlock()
	rec.applying = false
	switch {
	case err == nil:
		rec.p.Status, rec.p.SettledAt = ProposalApplied, b.now().Unix()
		return succeed(resp, result)
	case err.Recoverable:
	default:
		rec.p.Status, rec.p.SettledAt, rec.p.Error = ProposalFailed, b.now().Unix(), err.Message
	}
	return fail(resp, err)
}

// Reject settles a pending proposal without running it.
func (b *Bridge) Reject(id string) Response {
	b.mu.Lock()
	defer b.mu.Unlock()
	rec, err := b.pendingLocked(id)
	if err != nil {
		return Response{Error: err}
	}
	rec.p.Status, rec.p.SettledAt = ProposalRejected, b.now().Unix()
	p := rec.p
	return succeed(Response{RequestID: rec.req.RequestID, TraceID: rec.req.TraceID}, Result{Proposal: &p})
}

// claim marks a pending proposal as being applied, so two approvals of one
// proposal cannot both run it.
func (b *Bridge) claim(id string) (*proposalRec, *Error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	rec, err := b.pendingLocked(id)
	if err != nil {
		return nil, err
	}
	rec.applying = true
	return rec, nil
}

func (b *Bridge) pendingLocked(id string) (*proposalRec, *Error) {
	rec, ok := b.proposals[id]
	switch {
	case !ok:
		return nil, &Error{Code: CodeUnknownProposal, Message: "没有这个待审批的修改：" + id,
			Details: map[string]any{"proposalId": id}}
	case rec.applying:
		return nil, &Error{Code: CodeProposalSettled, Message: "这个修改正在写入", Recoverable: true,
			Details: map[string]any{"proposalId": id, "status": "applying"}}
	case rec.p.Status != ProposalPending:
		return nil, &Error{Code: CodeProposalSettled, Message: "这个修改已经处理过（" + rec.p.Status + "）",
			Details: map[string]any{"proposalId": id, "status": rec.p.Status}}
	}
	return rec, nil
}

// applyWrite checks the plugin and the action again, then runs the write.
func (b *Bridge) applyWrite(ctx context.Context, rec *proposalRec) (Result, *Error) {
	req := rec.req
	plugin, action, err := b.resolve(ctx, req)
	if err != nil {
		return Result{}, err
	}
	if !action.Write {
		return Result{}, &Error{Code: CodeUnknownAction, Message: "动作 " + action.ID + " 已不再是写操作",
			Details: map[string]any{"pluginId": req.PluginID, "actionId": action.ID}}
	}
	if b.Executor == nil {
		return Result{}, &Error{Code: CodeUnavailable, Message: "插件动作桥没有执行器，无法写入"}
	}
	deadlineMs := req.deadlineMs(b.maxDeadline())
	deadline := b.now().Add(time.Duration(deadlineMs) * time.Millisecond)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	call := Call{Plugin: plugin, Action: action, Request: req, Deadline: deadline,
		IdempotencyKey: req.PluginID + ":" + req.RequestID, Approved: rec.p.ID}
	if req.ViewID != "" {
		view := b.snapshot(req.PluginID, req.ViewID)
		call.State = view.State
		// A state patch from the write lands on the view as it is now.
		req.ContextRevision = view.Revision
	}
	result, xerr := b.Executor.Execute(ctx, call)
	if ctx.Err() != nil {
		return Result{}, contextErr(ctx, deadlineMs)
	}
	if xerr != nil {
		return Result{}, execErr(xerr)
	}
	if err := b.checkResult(req, &result); err != nil {
		return Result{}, err
	}
	return result, nil
}
