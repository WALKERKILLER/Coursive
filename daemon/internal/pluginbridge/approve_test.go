package pluginbridge

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func propose(t *testing.T, b *Bridge, requestID string) Proposal {
	t.Helper()
	req := call("review", "save")
	req.RequestID = requestID
	req.Args = map[string]any{"cardId": "c-1"}
	resp := b.Handle(context.Background(), req)
	if !resp.OK || resp.Result.Proposal == nil {
		t.Fatalf("want a proposal, got %+v", resp.Error)
	}
	return *resp.Result.Proposal
}

// An approved write runs once with the proposal named, and a settled proposal
// never runs again.
func TestApplyRunsTheApprovedWriteOnce(t *testing.T) {
	exec := &fakeExecutor{run: func(context.Context) (Result, error) {
		return Result{Notification: &Notification{Text: "已保存"}}, nil
	}}
	b := testBridge(exec)
	p := propose(t, b, "req-save")
	resp := b.Apply(context.Background(), p.ID)
	if !resp.OK || resp.Result.Notification.Text != "已保存" || resp.RequestID != "req-save" {
		t.Fatalf("apply = %+v %+v", resp, resp.Error)
	}
	if exec.count() != 1 || exec.last.Approved != p.ID || exec.last.Request.Args["cardId"] != "c-1" || !exec.last.Action.Write {
		t.Fatalf("executor call = %d %+v", exec.count(), exec.last)
	}
	if got, _ := b.Proposal(p.ID); got.Status != ProposalApplied || got.SettledAt == 0 {
		t.Fatalf("proposal = %+v", got)
	}
	if again := b.Apply(context.Background(), p.ID); again.Error == nil || again.Error.Code != CodeProposalSettled {
		t.Fatalf("second apply = %+v", again.Error)
	}
	if exec.count() != 1 {
		t.Fatalf("a settled proposal ran again")
	}
}

func TestRejectSettlesWithoutRunning(t *testing.T) {
	exec := &fakeExecutor{}
	b := testBridge(exec)
	p := propose(t, b, "req-save")
	resp := b.Reject(p.ID)
	if !resp.OK || resp.Result.Proposal.Status != ProposalRejected {
		t.Fatalf("reject = %+v", resp)
	}
	if apply := b.Apply(context.Background(), p.ID); apply.Error == nil || apply.Error.Code != CodeProposalSettled {
		t.Fatalf("apply after reject = %+v", apply.Error)
	}
	if exec.count() != 0 {
		t.Fatalf("a rejected write ran")
	}
	if r := b.Reject("proposal-none"); r.Error == nil || r.Error.Code != CodeUnknownProposal {
		t.Fatalf("unknown = %+v", r.Error)
	}
}

// The plugin state and the permission are checked again at approval time.
func TestApplyChecksThePluginAgain(t *testing.T) {
	exec := &fakeExecutor{}
	b := testBridge(exec)
	p := propose(t, b, "req-save")
	reg := b.Registry.(fakeRegistry)
	review := reg["review"]
	review.State = StateDisabled
	reg["review"] = review
	resp := b.Apply(context.Background(), p.ID)
	if resp.Error == nil || resp.Error.Code != CodePluginInactive {
		t.Fatalf("disabled = %+v", resp.Error)
	}
	// A recoverable refusal leaves the proposal pending for later.
	if got, _ := b.Proposal(p.ID); got.Status != ProposalPending {
		t.Fatalf("status = %s", got.Status)
	}
	review.State, review.Permissions = StateEnabled, []string{"views"}
	reg["review"] = review
	resp = b.Apply(context.Background(), p.ID)
	if resp.Error == nil || resp.Error.Code != CodePermissionDenied {
		t.Fatalf("revoked = %+v", resp.Error)
	}
	if got, _ := b.Proposal(p.ID); got.Status != ProposalFailed || got.Error == "" {
		t.Fatalf("status = %+v", got)
	}
	if exec.count() != 0 {
		t.Fatalf("the executor ran")
	}
}

// A timeout keeps the proposal pending; an executor failure settles it.
func TestApplyFailures(t *testing.T) {
	b := testBridge(&fakeExecutor{run: func(ctx context.Context) (Result, error) {
		<-ctx.Done()
		return Result{}, ctx.Err()
	}})
	b.MaxDeadlineMs = 20
	p := propose(t, b, "req-slow")
	if resp := b.Apply(context.Background(), p.ID); resp.Error == nil || resp.Error.Code != CodeDeadlineExceeded {
		t.Fatalf("timeout = %+v", resp.Error)
	}
	if got, _ := b.Proposal(p.ID); got.Status != ProposalPending {
		t.Fatalf("status after timeout = %s", got.Status)
	}
	b.Executor = &fakeExecutor{run: func(context.Context) (Result, error) { return Result{}, errors.New("disk full") }}
	if resp := b.Apply(context.Background(), p.ID); resp.Error == nil || resp.Error.Code != CodeInternal {
		t.Fatalf("failure = %+v", resp.Error)
	}
	if got, _ := b.Proposal(p.ID); got.Status != ProposalFailed || got.Error != "disk full" {
		t.Fatalf("status after failure = %+v", got)
	}
}

// Concurrent approvals of one proposal run the write once.
func TestConcurrentApply(t *testing.T) {
	release := make(chan struct{})
	exec := &fakeExecutor{run: func(context.Context) (Result, error) {
		<-release
		return Result{Notification: &Notification{Text: "ok"}}, nil
	}}
	b := testBridge(exec)
	p := propose(t, b, "req-save")
	var wg sync.WaitGroup
	codes := make(chan string, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := b.Apply(context.Background(), p.ID)
			if resp.Error != nil {
				codes <- resp.Error.Code
				return
			}
			codes <- "ok"
		}()
	}
	for exec.count() == 0 {
	}
	close(release)
	wg.Wait()
	close(codes)
	ok := 0
	for c := range codes {
		if c == "ok" {
			ok++
		} else if c != CodeProposalSettled {
			t.Fatalf("code = %s", c)
		}
	}
	if ok != 1 || exec.count() != 1 {
		t.Fatalf("ok = %d, runs = %d", ok, exec.count())
	}
}
