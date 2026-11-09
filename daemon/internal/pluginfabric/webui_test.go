package pluginfabric

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// testPagePlugin is one plugin with a read action, a write action and an action
// that only touches its own UI state.
func testPagePlugin() WebUIPlugin {
	return WebUIPlugin{
		ID:          "virtual-lab",
		Permissions: []string{"library.read", "library.write"},
		Actions: []WebUIAction{
			{ID: "curve.sample", Permission: "library.read"},
			{ID: "ui.toggle"},
			{ID: "simulation.save", Title: "保存模拟结果", Permission: "library.write", Write: true},
		},
	}
}

func testSession(t *testing.T, plugin WebUIPlugin) WebUISession {
	t.Helper()
	origin, err := WebUIOrigin(plugin.ID)
	if err != nil {
		t.Fatalf("WebUIOrigin(%q): %v", plugin.ID, err)
	}
	return WebUISession{PluginID: plugin.ID, InstanceID: "page-039", Origin: origin, Nonce: "nonce-7f3a"}
}

func testMessage(session WebUISession, action string) WebUIBridgeMessage {
	return WebUIBridgeMessage{
		SchemaVersion: WebUIBridgeSchemaVersion,
		Origin:        session.Origin,
		PluginID:      session.PluginID,
		InstanceID:    session.InstanceID,
		Nonce:         session.Nonce,
		RequestID:     "req-001",
		Action:        action,
		Args:          map[string]any{"courseId": "course-123"},
		DeadlineMs:    1000,
		TraceID:       "trace-9",
	}
}

type recordingExecutor struct {
	calls   int
	payload map[string]any
	err     error
	wait    time.Duration
}

func (e *recordingExecutor) Execute(ctx context.Context, call WebUICall) (map[string]any, error) {
	e.calls++
	if e.wait > 0 {
		select {
		case <-time.After(e.wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return e.payload, e.err
}

type fixedApprovals struct {
	mode string
	err  error
}

func (a fixedApprovals) ApprovalMode(context.Context) (string, error) { return a.mode, a.err }

func TestWebUIOriginIsPluginSpecific(t *testing.T) {
	first, err := WebUIOrigin("virtual-lab")
	if err != nil {
		t.Fatalf("WebUIOrigin: %v", err)
	}
	second, err := WebUIOrigin("review-lab")
	if err != nil {
		t.Fatalf("WebUIOrigin: %v", err)
	}
	if first != "coursive-plugin://virtual-lab" {
		t.Fatalf("origin = %q", first)
	}
	if first == second {
		t.Fatalf("two plugins share the origin %q", first)
	}
	for _, bad := range []string{"", "Virtual-Lab", "../escape", "virtual lab"} {
		if _, err := WebUIOrigin(bad); err == nil {
			t.Fatalf("WebUIOrigin(%q) accepted an invalid plugin id", bad)
		}
	}
}

func TestNormalizeWebUIOriginRejectsForeignOrigins(t *testing.T) {
	ok, valid := NormalizeWebUIOrigin("COURSIVE-PLUGIN://Virtual-Lab/")
	if !valid || ok != "coursive-plugin://virtual-lab" {
		t.Fatalf("normalized = %q, %v", ok, valid)
	}
	for _, bad := range []string{
		"http://127.0.0.1:4173",
		"http://localhost:8080",
		"https://virtual-lab.example",
		"coursive-plugin://virtual-lab:8443",
		"coursive-plugin://user:pass@virtual-lab",
		"coursive-plugin://virtual-lab/page.html",
		"coursive-plugin://virtual-lab?x=1",
		"file:///tmp/index.html",
		"",
	} {
		if got, valid := NormalizeWebUIOrigin(bad); valid {
			t.Fatalf("NormalizeWebUIOrigin(%q) accepted %q", bad, got)
		}
	}
}

func TestCheckWebUIURL(t *testing.T) {
	origin := "coursive-plugin://virtual-lab"
	hosts := []string{"example.org", "docs.example.org"}
	cases := []struct {
		name     string
		hosts    []string
		target   string
		decision string
		reason   string
	}{
		{"relative", hosts, "assets/curve.js", WebUIURLSameOrigin, WebUIURLReasonRelative},
		{"own origin", hosts, "coursive-plugin://virtual-lab/index.html", WebUIURLSameOrigin, WebUIURLReasonSameOrigin},
		{"other plugin", hosts, "coursive-plugin://review-lab/index.html", WebUIURLDeny, WebUIURLReasonCrossOrigin},
		{"path escape", hosts, "coursive-plugin://virtual-lab/../../etc/passwd", WebUIURLDeny, WebUIURLReasonPathEscape},
		{"declared https", hosts, "https://example.org/docs", WebUIURLExternal, WebUIURLReasonExternal},
		{"undeclared host", hosts, "https://evil.example/x", WebUIURLDeny, WebUIURLReasonHostDenied},
		{"subdomain is not implied", hosts, "https://api.example.org/x", WebUIURLDeny, WebUIURLReasonHostDenied},
		{"no declared host", nil, "https://example.org/docs", WebUIURLDeny, WebUIURLReasonHostDenied},
		{"wildcard entry", []string{"*"}, "https://evil.example/x", WebUIURLDeny, WebUIURLReasonHostDenied},
		{"plain http", hosts, "http://example.org/docs", WebUIURLDeny, WebUIURLReasonInsecure},
		{"declared loopback", []string{"127.0.0.1"}, "https://127.0.0.1:8443/admin", WebUIURLDeny, WebUIURLReasonLoopback},
		{"loopback http", hosts, "http://127.0.0.1:4173/", WebUIURLDeny, WebUIURLReasonLoopback},
		{"localhost", hosts, "http://localhost:8080/", WebUIURLDeny, WebUIURLReasonLoopback},
		{"ipv6 loopback", hosts, "https://[::1]:9000/", WebUIURLDeny, WebUIURLReasonLoopback},
		{"unspecified", hosts, "https://0.0.0.0/", WebUIURLDeny, WebUIURLReasonLoopback},
		{"javascript", hosts, "javascript:alert(1)", WebUIURLDeny, WebUIURLReasonScheme},
		{"data", hosts, "data:text/html,<script>alert(1)</script>", WebUIURLDeny, WebUIURLReasonScheme},
		{"blob", hosts, "blob:coursive-plugin://virtual-lab/x", WebUIURLDeny, WebUIURLReasonScheme},
		{"file", hosts, "file:///etc/passwd", WebUIURLDeny, WebUIURLReasonScheme},
		{"websocket", hosts, "wss://example.org/socket", WebUIURLDeny, WebUIURLReasonScheme},
		{"protocol relative", hosts, "//example.org/x", WebUIURLDeny, WebUIURLReasonScheme},
		{"credentials", hosts, "https://user:secret@example.org/", WebUIURLDeny, WebUIURLReasonCredentials},
		{"empty", hosts, "   ", WebUIURLDeny, WebUIURLReasonEmpty},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CheckWebUIURL(origin, c.hosts, c.target)
			if got.Decision != c.decision || got.Reason != c.reason {
				t.Fatalf("CheckWebUIURL(%q) = %+v, want %s/%s", c.target, got, c.decision, c.reason)
			}
		})
	}
	// A relative target stays in the page; the origin itself is checked when
	// the host opens the page, not here.
	if got := CheckWebUIURL("http://127.0.0.1:4173", nil, "index.html"); got.Reason != WebUIURLReasonRelative {
		t.Fatalf("relative target = %+v", got)
	}
	if got := CheckWebUIURL("http://127.0.0.1:4173", nil, "coursive-plugin://virtual-lab/x"); got.Reason != WebUIURLReasonOriginBroken {
		t.Fatalf("a page without a controlled origin = %+v", got)
	}
}

func TestWebUICSPDeniesExternalResources(t *testing.T) {
	csp := WebUICSP("n0nce")
	for _, want := range []string{
		"default-src 'none'",
		"script-src 'nonce-n0nce'",
		"style-src 'nonce-n0nce'",
		"img-src data:",
		"connect-src 'none'",
		"frame-src 'none'",
		"worker-src 'none'",
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'none'",
	} {
		if !strings.Contains(csp, want) {
			t.Fatalf("CSP is missing %q: %s", want, csp)
		}
	}
	for _, unwanted := range []string{"*", "unsafe-inline", "unsafe-eval", "http:", "https:", "'self'", "blob:"} {
		if strings.Contains(csp, unwanted) {
			t.Fatalf("CSP contains %q: %s", unwanted, csp)
		}
	}
}

func TestWebUIBridgeAnswersDeclaredReadAction(t *testing.T) {
	plugin := testPagePlugin()
	session := testSession(t, plugin)
	executor := &recordingExecutor{payload: map[string]any{"points": 12.5}}
	bridge := WebUIBridge{Executor: executor}

	out := bridge.Handle(context.Background(), session, plugin, testMessage(session, "curve.sample"))
	if !out.OK || out.Error != nil {
		t.Fatalf("Handle refused a valid message: %+v", out.Error)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls = %d", executor.calls)
	}
	if out.Result == nil || out.Result.Payload["points"] != 12.5 || out.Result.Proposal != nil {
		t.Fatalf("result = %+v", out.Result)
	}
	if out.RequestID != "req-001" || out.TraceID != "trace-9" {
		t.Fatalf("answer lost its request identity: %+v", out)
	}
}

func TestWebUIBridgeRefusesForgedIdentity(t *testing.T) {
	plugin := testPagePlugin()
	session := testSession(t, plugin)
	bridge := WebUIBridge{Executor: &recordingExecutor{}}

	forgedPlugin := testMessage(session, "curve.sample")
	forgedPlugin.PluginID = "review-lab"
	if out := bridge.Handle(context.Background(), session, plugin, forgedPlugin); out.Error == nil ||
		out.Error.Code != WebUICodeForgedIdentity {
		t.Fatalf("a message naming another plugin was not refused: %+v", out.Error)
	}

	forgedInstance := testMessage(session, "curve.sample")
	forgedInstance.InstanceID = "page-001"
	if out := bridge.Handle(context.Background(), session, plugin, forgedInstance); out.Error == nil ||
		out.Error.Code != WebUICodeForgedIdentity {
		t.Fatalf("a message naming another page instance was not refused: %+v", out.Error)
	}

	// A page that rewrites the plugin id of the whole message still fails,
	// because the session, not the payload, decides the identity.
	otherPlugin := WebUIPlugin{ID: "review-lab", Actions: plugin.Actions}
	if out := bridge.Handle(context.Background(), session, otherPlugin, testMessage(session, "curve.sample")); out.Error == nil ||
		out.Error.Code != WebUICodeInvalidSession {
		t.Fatalf("a session of another plugin was accepted: %+v", out.Error)
	}
}

func TestWebUIBridgeRefusesCrossOriginAndNonceMismatch(t *testing.T) {
	plugin := testPagePlugin()
	session := testSession(t, plugin)
	bridge := WebUIBridge{Executor: &recordingExecutor{}}

	foreign := testMessage(session, "curve.sample")
	foreign.Origin = "coursive-plugin://review-lab"
	if out := bridge.Handle(context.Background(), session, plugin, foreign); out.Error == nil ||
		out.Error.Code != WebUICodeCrossOrigin {
		t.Fatalf("a cross-origin message was not refused: %+v", out.Error)
	}

	loopback := testMessage(session, "curve.sample")
	loopback.Origin = "http://127.0.0.1:4173"
	if out := bridge.Handle(context.Background(), session, plugin, loopback); out.Error == nil ||
		out.Error.Code != WebUICodeCrossOrigin {
		t.Fatalf("a loopback origin was not refused: %+v", out.Error)
	}

	// A host session that was somehow opened on a loopback origin is refused
	// before any message is read: loopback is not a plugin identity.
	loopbackSession := session
	loopbackSession.Origin = "http://127.0.0.1:4173"
	if out := bridge.Handle(context.Background(), loopbackSession, plugin, testMessage(loopbackSession, "curve.sample")); out.Error == nil ||
		out.Error.Code != WebUICodeInvalidSession {
		t.Fatalf("a loopback session was accepted: %+v", out.Error)
	}

	stale := testMessage(session, "curve.sample")
	stale.Nonce = "nonce-other"
	if out := bridge.Handle(context.Background(), session, plugin, stale); out.Error == nil ||
		out.Error.Code != WebUICodeNonceMismatch {
		t.Fatalf("a message with another nonce was not refused: %+v", out.Error)
	}
}

func TestWebUIBridgeRefusesReplay(t *testing.T) {
	plugin := testPagePlugin()
	session := testSession(t, plugin)
	executor := &recordingExecutor{payload: map[string]any{"ok": true}}
	bridge := WebUIBridge{Executor: executor}
	msg := testMessage(session, "curve.sample")

	if out := bridge.Handle(context.Background(), session, plugin, msg); !out.OK {
		t.Fatalf("first message refused: %+v", out.Error)
	}
	if out := bridge.Handle(context.Background(), session, plugin, msg); out.Error == nil ||
		out.Error.Code != WebUICodeReplayed {
		t.Fatalf("replayed message was not refused: %+v", out.Error)
	}
	if executor.calls != 1 {
		t.Fatalf("a replay ran the action again: calls = %d", executor.calls)
	}

	// A retry uses a new request id, which is a new request.
	retry := msg
	retry.RequestID = "req-002"
	if out := bridge.Handle(context.Background(), session, plugin, retry); !out.OK {
		t.Fatalf("a new request id was refused: %+v", out.Error)
	}

	// Two concurrent copies of one message act once: exactly one is answered.
	concurrent := msg
	concurrent.RequestID = "req-003"
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	replayed := 0
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out := bridge.Handle(context.Background(), session, plugin, concurrent)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case out.OK:
				ok++
			case out.Error != nil && out.Error.Code == WebUICodeReplayed:
				replayed++
			}
		}()
	}
	wg.Wait()
	if ok != 1 || replayed != 3 {
		t.Fatalf("concurrent replay: ok=%d replayed=%d", ok, replayed)
	}
}

func TestWebUIBridgeChecksPermissionPerAction(t *testing.T) {
	plugin := testPagePlugin()
	plugin.Permissions = []string{"library.read"}
	session := testSession(t, plugin)
	executor := &recordingExecutor{payload: map[string]any{"ok": true}}
	bridge := WebUIBridge{Executor: executor, Approvals: fixedApprovals{mode: WebUIApprovalAsk}}

	if out := bridge.Handle(context.Background(), session, plugin, testMessage(session, "curve.sample")); !out.OK {
		t.Fatalf("a granted read action was refused: %+v", out.Error)
	}

	write := testMessage(session, "simulation.save")
	write.RequestID = "req-010"
	if out := bridge.Handle(context.Background(), session, plugin, write); out.Error == nil ||
		out.Error.Code != WebUICodePermissionDenied {
		t.Fatalf("an action without its permission was not refused: %+v", out.Error)
	}
	if _, ok := bridge.Proposal(webUIProposalID(plugin.ID, session.InstanceID, "req-010")); ok {
		t.Fatal("a refused action still stored a proposal")
	}

	// A wildcard grant is not a grant: permissions match by name.
	plugin.Permissions = []string{"*"}
	if out := bridge.Handle(context.Background(), session, plugin, write); out.Error == nil ||
		out.Error.Code != WebUICodePermissionDenied {
		t.Fatalf("a wildcard permission granted an action: %+v", out.Error)
	}

	unknown := testMessage(session, "vault.export")
	unknown.RequestID = "req-011"
	if out := bridge.Handle(context.Background(), session, plugin, unknown); out.Error == nil ||
		out.Error.Code != WebUICodeUnknownAction {
		t.Fatalf("an undeclared action was not refused: %+v", out.Error)
	}
}

func TestWebUIBridgeRefusesWildcardAndSecrets(t *testing.T) {
	plugin := testPagePlugin()
	session := testSession(t, plugin)
	bridge := WebUIBridge{Executor: &recordingExecutor{}}

	wildcard := testMessage(session, "*")
	if out := bridge.Handle(context.Background(), session, plugin, wildcard); out.Error == nil ||
		out.Error.Code != WebUICodeWildcardDenied {
		t.Fatalf("a wildcard action was not refused: %+v", out.Error)
	}

	for name, args := range map[string]map[string]any{
		"daemon token":   {"token": "0123456789abcdef"},
		"authorization":  {"headers": map[string]any{"Authorization": "Bearer x"}},
		"cookie":         {"cookie": "session=1"},
		"vault secret":   {"vault": map[string]any{"apiKey": "sk-live"}},
		"nested in list": {"items": []any{map[string]any{"password": "hunter2"}}},
	} {
		msg := testMessage(session, "curve.sample")
		msg.RequestID = "req-secret"
		msg.Args = args
		out := bridge.Handle(context.Background(), session, plugin, msg)
		if out.Error == nil || out.Error.Code != WebUICodeSecretInMessage {
			t.Fatalf("%s was not refused: %+v", name, out.Error)
		}
		if strings.Contains(out.Error.Message, "hunter2") || strings.Contains(out.Error.Message, "sk-live") {
			t.Fatalf("%s echoed the secret in its message: %s", name, out.Error.Message)
		}
	}
}

func TestWebUIBridgeDeadline(t *testing.T) {
	plugin := testPagePlugin()
	session := testSession(t, plugin)

	slow := &recordingExecutor{wait: 2 * time.Second, payload: map[string]any{"late": true}}
	bridge := WebUIBridge{Executor: slow}
	msg := testMessage(session, "curve.sample")
	msg.DeadlineMs = 20
	start := time.Now()
	out := bridge.Handle(context.Background(), session, plugin, msg)
	if out.Error == nil || out.Error.Code != WebUICodeDeadlineExceeded {
		t.Fatalf("a slow action was not timed out: %+v", out.Error)
	}
	if !out.Error.Recoverable {
		t.Fatal("a timeout should be recoverable")
	}
	if out.Result != nil {
		t.Fatal("a timed-out action returned a result")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("the bridge waited %s for a 20ms deadline", elapsed)
	}

	tooLong := testMessage(session, "curve.sample")
	tooLong.RequestID = "req-long"
	tooLong.DeadlineMs = WebUIMaxDeadlineMs + 1
	if out := bridge.Handle(context.Background(), session, plugin, tooLong); out.Error == nil ||
		out.Error.Code != WebUICodeInvalidRequest {
		t.Fatalf("an over-long deadline was accepted: %+v", out.Error)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cancelMsg := testMessage(session, "curve.sample")
	cancelMsg.RequestID = "req-canceled"
	if out := bridge.Handle(canceled, session, plugin, cancelMsg); out.Error == nil ||
		out.Error.Code != WebUICodeCanceled {
		t.Fatalf("a canceled action was not reported: %+v", out.Error)
	}
}

func TestWebUIBridgeWriteIsPendingProposalOnly(t *testing.T) {
	plugin := testPagePlugin()
	session := testSession(t, plugin)
	executor := &recordingExecutor{}
	bridge := WebUIBridge{Executor: executor, Approvals: fixedApprovals{mode: WebUIApprovalAsk}}
	msg := testMessage(session, "simulation.save")

	out := bridge.Handle(context.Background(), session, plugin, msg)
	if !out.OK || out.Result == nil || out.Result.Proposal == nil {
		t.Fatalf("a write did not answer with a proposal: %+v", out.Error)
	}
	if executor.calls != 0 {
		t.Fatalf("a write reached the executor %d times", executor.calls)
	}
	proposal := out.Result.Proposal
	if proposal.Status != ProposalStatusPending {
		t.Fatalf("proposal status = %q", proposal.Status)
	}
	if proposal.ApprovalMode != WebUIApprovalAsk || proposal.Summary != "保存模拟结果" {
		t.Fatalf("proposal = %+v", proposal)
	}
	if proposal.PluginID != plugin.ID || proposal.InstanceID != session.InstanceID || proposal.ActionID != "simulation.save" {
		t.Fatalf("proposal identity = %+v", proposal)
	}
	stored, ok := bridge.Proposal(proposal.ID)
	if !ok || stored.Status != ProposalStatusPending {
		t.Fatalf("stored proposal = %+v, %v", stored, ok)
	}

	// Without an approval source the bridge refuses instead of assuming 自动.
	noApprovals := WebUIBridge{Executor: executor}
	other := testMessage(session, "simulation.save")
	other.RequestID = "req-020"
	if out := noApprovals.Handle(context.Background(), session, plugin, other); out.Error == nil ||
		out.Error.Code != WebUICodeApprovalUnavailable {
		t.Fatalf("a write without approvals was accepted: %+v", out.Error)
	}

	brokenApprovals := WebUIBridge{Executor: executor, Approvals: fixedApprovals{err: errors.New("boom")}}
	third := testMessage(session, "simulation.save")
	third.RequestID = "req-021"
	if out := brokenApprovals.Handle(context.Background(), session, plugin, third); out.Error == nil ||
		out.Error.Code != WebUICodeApprovalUnavailable {
		t.Fatalf("a write with an unreadable approval mode was accepted: %+v", out.Error)
	}
}

func TestWebUIBridgeRefusesWrongSchemaVersion(t *testing.T) {
	plugin := testPagePlugin()
	session := testSession(t, plugin)
	bridge := WebUIBridge{Executor: &recordingExecutor{}}
	msg := testMessage(session, "curve.sample")
	msg.SchemaVersion = WebUIBridgeSchemaVersion + 1
	if out := bridge.Handle(context.Background(), session, plugin, msg); out.Error == nil ||
		out.Error.Code != WebUICodeSchemaVersion {
		t.Fatalf("an unknown schema version was accepted: %+v", out.Error)
	}
}

func TestWebUIAdmissionMatrix(t *testing.T) {
	byPlatform := map[string]WebUIAdmission{}
	for _, a := range WebUIAdmissions() {
		byPlatform[a.Platform] = a
		if a.Available() {
			t.Fatalf("%s is available before its security review passed", a.Platform)
		}
		if a.Reason == "" {
			t.Fatalf("%s has no admission reason", a.Platform)
		}
	}
	if len(byPlatform) != 4 {
		t.Fatalf("admission matrix has %d platforms", len(byPlatform))
	}
	for _, p := range []string{"android", "macos"} {
		if got := byPlatform[p]; got.Status != AdmissionSupported || !got.Renderer || !got.Previewable() ||
			got.FallbackReason() != FallbackReviewPending {
			t.Fatalf("%s = %+v", p, got)
		}
	}
	for _, p := range []string{"windows", "linux"} {
		if got := byPlatform[p]; got.Status != AdmissionNeedsReview || got.Renderer || got.Previewable() ||
			got.FallbackReason() != FallbackPlatformNeedsReview {
			t.Fatalf("%s = %+v", p, got)
		}
	}
	if got := WebUIAdmissionFor("plan9"); got.Status != AdmissionUnsupported ||
		got.FallbackReason() != FallbackPlatformUnsupported || got.Available() {
		t.Fatalf("unknown platform = %+v", got)
	}
	if got := WebUIAdmissionFor(" ANDROID "); got.Platform != "android" {
		t.Fatalf("platform lookup did not normalize: %+v", got)
	}

	// An admitted platform with a renderer that passed its review is the only
	// available combination; without a renderer there is nothing to preview.
	supported := WebUIAdmission{Platform: "android", Status: AdmissionSupported, Renderer: true, Review: ReviewPassed}
	if !supported.Available() || supported.FallbackReason() != "" {
		t.Fatalf("an admitted, reviewed platform with a renderer is not available: %+v", supported)
	}
	bare := WebUIAdmission{Platform: "android", Status: AdmissionSupported, Review: ReviewPassed}
	if bare.Available() || bare.Previewable() || bare.FallbackReason() != FallbackRendererMissing {
		t.Fatalf("a platform without a renderer = %+v", bare)
	}
}

// TestWebUIReleaseGate keeps the admission matrix and the security review
// record of spec 0032 (Web UI release gate) in step: every platform has one
// row, the row's review state is the matrix's, and a passed review names its
// record. A platform therefore becomes available to users only by a reviewed
// change to both.
func TestWebUIReleaseGate(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "specs", "0032-ui-extension-protocol.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, gate, ok := strings.Cut(string(spec), "### Web UI release gate")
	if !ok {
		t.Fatal("spec 0032 has no Web UI release gate section")
	}
	if next := strings.Index(gate, "\n## "); next >= 0 {
		gate = gate[:next]
	}
	rows := map[string][]string{}
	for _, line := range strings.Split(gate, "\n") {
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) < 4 || !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if _, known := webUIAdmissions[cells[0]]; known {
			rows[cells[0]] = cells
		}
	}
	for _, a := range WebUIAdmissions() {
		row, ok := rows[a.Platform]
		if !ok {
			t.Fatalf("the release gate has no row for %s", a.Platform)
		}
		if row[1] != a.Review {
			t.Fatalf("%s: the release gate records %q, the admission matrix %q", a.Platform, row[1], a.Review)
		}
		if a.Review == ReviewPassed && (row[3] == "" || row[3] == "—") {
			t.Fatalf("%s passed its review without a review record", a.Platform)
		}
	}
}
