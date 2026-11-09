// Web UI (L3) of the UI Extension Protocol: the plugin-specific origin, the
// content security policy and the URL policy of a page, the native bridge that
// validates its messages, and the platform admission that decides whether a
// page may render at all.
//
// Contract: docs/specs/0032-ui-extension-protocol.md (Isolated Web UI). This
// file is policy only: it starts no browser engine and holds no credential.
// webui_page.go builds the page a renderer loads; the renderer itself is the
// app's WebView, and a platform opens it only once the admission record says
// so — until then the app keeps the native L1/L2 surfaces usable.
package pluginfabric

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// WebUIScheme is the controlled scheme a plugin page runs on. A page never runs
// on a loopback HTTP origin: 127.0.0.1 is not a plugin identity.
const WebUIScheme = "coursive-plugin"

const (
	// WebUIBridgeSchemaVersion is the message shape this bridge accepts.
	WebUIBridgeSchemaVersion = 1
	// WebUIDefaultDeadlineMs bounds a message that names no deadline.
	WebUIDefaultDeadlineMs int64 = 5_000
	// WebUIMaxDeadlineMs is the longest deadline a page may ask for.
	WebUIMaxDeadlineMs int64 = 15_000
	// maxWebUIMessageBytes caps the argument payload of one message.
	maxWebUIMessageBytes = 128 << 10
	// webUIReplayMemory bounds remembered request ids.
	webUIReplayMemory = 512
	// maxWebUIArgDepth bounds the argument walk that looks for credentials.
	maxWebUIArgDepth = 8
)

// WebUIOrigin is the plugin-specific origin "coursive-plugin://<pluginId>".
// Each plugin gets its own origin, so one page cannot read another plugin's
// document, storage or bridge session.
func WebUIOrigin(pluginID string) (string, error) {
	if !idRE.MatchString(pluginID) {
		return "", fmt.Errorf("插件 id %q 无效，无法分配 Web UI Origin", pluginID)
	}
	return WebUIScheme + "://" + pluginID, nil
}

// NormalizeWebUIOrigin lower-cases the scheme and host of a controlled origin
// and refuses everything such an origin cannot carry: user info, a port, a
// path, a query or a fragment. It returns false for any other origin, including
// http://127.0.0.1:<port>.
func NormalizeWebUIOrigin(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" || u.Opaque != "" {
		return "", false
	}
	if u.User != nil || u.Port() != "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	if scheme != WebUIScheme || !idRE.MatchString(host) {
		return "", false
	}
	return scheme + "://" + host, true
}

// WebUICSP is the Content-Security-Policy of one plugin page. The host inlines
// the plugin's own script and styles under the page nonce, so nothing else
// runs: no other inline script, no eval, no external source. Images and fonts
// are the package's own, inlined as data URLs; an approved network request goes
// through the daemon broker instead of connect-src. Frames, workers, objects,
// forms and a base change do not exist. The policy is delivered in a meta
// element, which cannot carry frame-ancestors; the page is the WebView's top
// document, so nothing frames it.
func WebUICSP(nonce string) string {
	return strings.Join([]string{
		"default-src 'none'",
		"script-src 'nonce-" + nonce + "'",
		"style-src 'nonce-" + nonce + "'",
		"img-src data:",
		"font-src data:",
		"media-src 'none'",
		"connect-src 'none'",
		"frame-src 'none'",
		"worker-src 'none'",
		"child-src 'none'",
		"object-src 'none'",
		"manifest-src 'none'",
		"base-uri 'none'",
		"form-action 'none'",
	}, "; ")
}

// What the host does with a URL a page asks to open.
const (
	// WebUIURLSameOrigin stays inside the plugin's own package.
	WebUIURLSameOrigin = "same-origin"
	// WebUIURLExternal opens in the system browser, after the user confirms.
	WebUIURLExternal = "external"
	// WebUIURLDeny refuses the target.
	WebUIURLDeny = "deny"
)

// URL policy reason codes, stable so the app can explain a refusal.
const (
	WebUIURLReasonRelative     = "relative"
	WebUIURLReasonSameOrigin   = "same-origin"
	WebUIURLReasonExternal     = "external-https"
	WebUIURLReasonEmpty        = "empty"
	WebUIURLReasonInvalid      = "url-invalid"
	WebUIURLReasonScheme       = "scheme-not-allowed"
	WebUIURLReasonInsecure     = "insecure-scheme"
	WebUIURLReasonCrossOrigin  = "cross-origin"
	WebUIURLReasonLoopback     = "loopback-not-trusted"
	WebUIURLReasonCredentials  = "credentials-in-url"
	WebUIURLReasonPathEscape   = "path-escape"
	WebUIURLReasonOriginBroken = "origin-invalid"
	WebUIURLReasonHostDenied   = "host-not-allowed"
)

// WebUIURLDecision is the policy answer for one target, with the reason code
// that explains it.
type WebUIURLDecision struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// Allowed reports whether the page may open the target at all.
func (d WebUIURLDecision) Allowed() bool { return d.Decision != WebUIURLDeny }

// CheckWebUIURL decides a page's navigation, link and download target. A
// relative target and the plugin's own origin stay in the page; an https target
// on a host the plugin declared may be opened in the system browser after the
// user confirms; everything else — http, file, data, blob, javascript,
// websockets, another plugin's origin, an undeclared host, a loopback host and a
// target carrying credentials — is refused. An empty host list allows no
// external target at all.
func CheckWebUIURL(origin string, hosts []string, raw string) WebUIURLDecision {
	target := strings.TrimSpace(raw)
	if target == "" {
		return denyURL(WebUIURLReasonEmpty)
	}
	u, err := url.Parse(target)
	if err != nil {
		return denyURL(WebUIURLReasonInvalid)
	}
	if u.User != nil {
		return denyURL(WebUIURLReasonCredentials)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme == "" {
		// A relative target stays inside the page's own origin; "//host/path"
		// borrows the page's scheme but names another host, so it is refused.
		if strings.HasPrefix(target, "//") || strings.HasPrefix(target, `\\`) {
			return denyURL(WebUIURLReasonScheme)
		}
		return WebUIURLDecision{Decision: WebUIURLSameOrigin, Reason: WebUIURLReasonRelative}
	}
	switch scheme {
	case WebUIScheme:
		own, ok := NormalizeWebUIOrigin(origin)
		if !ok {
			return denyURL(WebUIURLReasonOriginBroken)
		}
		host := strings.ToLower(u.Hostname())
		if u.Port() != "" || host == "" {
			return denyURL(WebUIURLReasonInvalid)
		}
		if WebUIScheme+"://"+host != own {
			return denyURL(WebUIURLReasonCrossOrigin)
		}
		if strings.Contains(u.Path, "..") {
			return denyURL(WebUIURLReasonPathEscape)
		}
		return WebUIURLDecision{Decision: WebUIURLSameOrigin, Reason: WebUIURLReasonSameOrigin}
	case "https":
		if isLoopbackHost(u.Hostname()) {
			return denyURL(WebUIURLReasonLoopback)
		}
		if !hostAllowed(hosts, u.Hostname()) {
			return denyURL(WebUIURLReasonHostDenied)
		}
		return WebUIURLDecision{Decision: WebUIURLExternal, Reason: WebUIURLReasonExternal}
	case "http":
		if isLoopbackHost(u.Hostname()) {
			return denyURL(WebUIURLReasonLoopback)
		}
		return denyURL(WebUIURLReasonInsecure)
	}
	return denyURL(WebUIURLReasonScheme)
}

func denyURL(reason string) WebUIURLDecision {
	return WebUIURLDecision{Decision: WebUIURLDeny, Reason: reason}
}

// hostAllowed reports an exact match against the hosts the plugin declared. A
// wildcard entry matches nothing: the allowlist is read literally, so a manifest
// cannot turn itself into an open proxy.
func hostAllowed(hosts []string, host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, declared := range hosts {
		if strings.ToLower(strings.TrimSpace(declared)) == host {
			return true
		}
	}
	return false
}

// isLoopbackHost reports a host the page may not reach: a loopback or
// unspecified address, "localhost" and its subdomains.
func isLoopbackHost(host string) bool {
	host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	if host == "" {
		return false
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsUnspecified()
}

// Bridge error codes. A page reads the code, not a message.
const (
	WebUICodeInvalidRequest      = "invalid_request"
	WebUICodeInvalidSession      = "invalid_session"
	WebUICodeSchemaVersion       = "schema_version"
	WebUICodeForgedIdentity      = "forged_identity"
	WebUICodeCrossOrigin         = "cross_origin"
	WebUICodeNonceMismatch       = "nonce_mismatch"
	WebUICodeReplayed            = "replayed_request"
	WebUICodeWildcardDenied      = "wildcard_denied"
	WebUICodeUnknownAction       = "unknown_action"
	WebUICodePermissionDenied    = "permission_denied"
	WebUICodeSecretInMessage     = "secret_in_message"
	WebUICodeInvalidArgs         = "invalid_args"
	WebUICodeDeadlineExceeded    = "deadline_exceeded"
	WebUICodeCanceled            = "canceled"
	WebUICodeUnavailable         = "unavailable"
	WebUICodeApprovalUnavailable = "approval_unavailable"
)

// Approval modes the bridge records on a proposal.
const (
	WebUIApprovalAuto = "auto"
	WebUIApprovalAsk  = "ask"
)

// ProposalStatusPending is the only status a bridge proposal has: a write is
// committed by the existing approval path, never here.
const ProposalStatusPending = "pending"

// WebUIBridgeMessage is one structured message from a plugin page. Every field
// is checked against the host's own session, so a page cannot claim an identity
// by writing it into the payload.
type WebUIBridgeMessage struct {
	SchemaVersion int            `json:"schemaVersion"`
	Origin        string         `json:"origin"`
	PluginID      string         `json:"pluginId"`
	InstanceID    string         `json:"instanceId"`
	Nonce         string         `json:"nonce"`
	RequestID     string         `json:"requestId"`
	Action        string         `json:"action"`
	Args          map[string]any `json:"args"`
	DeadlineMs    int64          `json:"deadlineMs"`
	TraceID       string         `json:"traceId"`
}

// WebUISession is the trusted side of the bridge: the host opened this page for
// one plugin instance with one origin and one nonce, and identity comes from
// here rather than from the message.
type WebUISession struct {
	PluginID   string
	InstanceID string
	Origin     string
	Nonce      string
}

// WebUIAction is one action a plugin declared for its page.
type WebUIAction struct {
	ID string
	// Title is what the user reads on the proposal.
	Title string
	// Permission is the declared permission this action needs; empty means the
	// action only touches plugin UI state. Every action checks its own
	// permission, so a page cannot borrow another action's grant.
	Permission string
	// Write marks an action that changes domain data; it always answers with a
	// pending proposal and never reaches the executor.
	Write bool
}

// WebUIPlugin is the daemon-owned state a page is checked against: the
// permissions the user granted and the actions the plugin declared. It comes
// from the plugin registry, never from the page.
type WebUIPlugin struct {
	ID          string
	Permissions []string
	Actions     []WebUIAction
}

// WebUIProposal is a write the user has not authorized yet.
type WebUIProposal struct {
	ID           string         `json:"id"`
	PluginID     string         `json:"pluginId"`
	InstanceID   string         `json:"instanceId"`
	ActionID     string         `json:"actionId"`
	Summary      string         `json:"summary"`
	Preview      map[string]any `json:"preview,omitempty"`
	ApprovalMode string         `json:"approvalMode"`
	Status       string         `json:"status"`
	RequestID    string         `json:"requestId"`
	TraceID      string         `json:"traceId"`
	CreatedAt    int64          `json:"createdAt"`
}

// WebUIResult is what the page receives: a read action's payload or a pending
// proposal, never both and never a direct commit.
type WebUIResult struct {
	Payload  map[string]any `json:"payload,omitempty"`
	Proposal *WebUIProposal `json:"proposal,omitempty"`
}

// WebUIError is a diagnosable refusal.
type WebUIError struct {
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	Details     map[string]any `json:"details,omitempty"`
	Recoverable bool           `json:"recoverable,omitempty"`
}

func (e *WebUIError) Error() string { return e.Code + ": " + e.Message }

// WebUIOutcome is the bridge's answer: result and error are exclusive.
type WebUIOutcome struct {
	OK        bool         `json:"ok"`
	RequestID string       `json:"requestId,omitempty"`
	TraceID   string       `json:"traceId,omitempty"`
	Result    *WebUIResult `json:"result,omitempty"`
	Error     *WebUIError  `json:"error,omitempty"`
}

// WebUICall is a validated message handed to the executor.
type WebUICall struct {
	Session  WebUISession
	Plugin   WebUIPlugin
	Action   WebUIAction
	Message  WebUIBridgeMessage
	Deadline time.Time
}

// WebUIExecutor runs a read action for a page. A write never reaches it.
type WebUIExecutor interface {
	Execute(ctx context.Context, call WebUICall) (map[string]any, error)
}

// WebUIApprovals reports the user's approval mode. A nil one refuses writes
// instead of assuming 自动.
type WebUIApprovals interface {
	ApprovalMode(ctx context.Context) (string, error)
}

// WebUIBridge validates and answers page messages. Its zero value refuses
// everything it cannot verify: with no executor it answers "unavailable" and
// with no approvals it refuses every write.
type WebUIBridge struct {
	Executor  WebUIExecutor
	Approvals WebUIApprovals
	// Now is the clock; nil uses time.Now.
	Now func() time.Time

	mu        sync.Mutex
	seen      map[string]time.Time
	order     []string
	proposals map[string]WebUIProposal
}

// Handle validates one message and answers it. A repeated request id from the
// same page instance is refused instead of answered twice, so an injected or
// replayed message cannot act a second time; a retry uses a new request id.
func (b *WebUIBridge) Handle(ctx context.Context, session WebUISession, plugin WebUIPlugin, msg WebUIBridgeMessage) WebUIOutcome {
	out := WebUIOutcome{RequestID: msg.RequestID, TraceID: msg.TraceID}
	action, err := b.Verify(session, plugin, msg)
	if err != nil {
		return refuse(out, err)
	}
	deadlineMs := msg.DeadlineMs
	if deadlineMs == 0 {
		deadlineMs = WebUIDefaultDeadlineMs
	}
	deadline := b.now().Add(time.Duration(deadlineMs) * time.Millisecond)
	call := WebUICall{Session: session, Plugin: plugin, Action: action, Message: msg, Deadline: deadline}
	if action.Write {
		proposal, err := b.propose(ctx, call)
		if err != nil {
			return refuse(out, err)
		}
		out.OK, out.Result = true, &WebUIResult{Proposal: &proposal}
		return out
	}
	if b.Executor == nil {
		return refuse(out, &WebUIError{Code: WebUICodeUnavailable,
			Message: "守护进程没有 Web UI 动作执行器，无法运行 " + msg.Action})
	}
	runCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	payload, xerr := b.Executor.Execute(runCtx, call)
	// A deadline or a cancellation commits nothing: the payload is dropped even
	// when the executor answered anyway.
	if runCtx.Err() != nil {
		return refuse(out, deadlineErr(runCtx, deadlineMs))
	}
	if xerr != nil {
		var typed *WebUIError
		if errors.As(xerr, &typed) {
			return refuse(out, typed)
		}
		return refuse(out, &WebUIError{Code: WebUICodeUnavailable, Message: xerr.Error()})
	}
	out.OK, out.Result = true, &WebUIResult{Payload: payload}
	return out
}

// Verify runs every check that comes before an action: the session, the
// envelope, the identity fields against the session, credentials in the
// arguments, the declared action and its own permission. It then claims the
// request id, so a replayed message is refused. The daemon's page route
// forwards a verified message to the action bridge, which runs a read or turns
// a write into a pending proposal.
func (b *WebUIBridge) Verify(session WebUISession, plugin WebUIPlugin, msg WebUIBridgeMessage) (WebUIAction, *WebUIError) {
	if err := checkSession(session, plugin); err != nil {
		return WebUIAction{}, err
	}
	if err := checkEnvelope(msg); err != nil {
		return WebUIAction{}, err
	}
	if err := checkIdentity(session, plugin, msg); err != nil {
		return WebUIAction{}, err
	}
	if err := checkNoSecrets(msg.Args); err != nil {
		return WebUIAction{}, err
	}
	action, err := findWebUIAction(plugin, msg.Action)
	if err != nil {
		return WebUIAction{}, err
	}
	if action.Permission != "" && !hasPermission(plugin.Permissions, action.Permission) {
		return WebUIAction{}, &WebUIError{Code: WebUICodePermissionDenied,
			Message: fmt.Sprintf("插件 %s 没有获得 %s 权限", plugin.ID, action.Permission),
			Details: map[string]any{"pluginId": plugin.ID, "permission": action.Permission, "action": action.ID}}
	}
	if err := b.claimRequest(session, msg); err != nil {
		return WebUIAction{}, err
	}
	return action, nil
}

// NewWebUISession opens a page session for one plugin: its controlled origin
// and a fresh instance id and nonce the page carries in every message.
func NewWebUISession(pluginID string) (WebUISession, error) {
	origin, err := WebUIOrigin(pluginID)
	if err != nil {
		return WebUISession{}, err
	}
	return WebUISession{PluginID: pluginID, InstanceID: "web-" + randomHex(16), Origin: origin, Nonce: randomHex(16)}, nil
}

// WebUIFrameOrigin is the controlled origin of the URL a renderer reports for
// the frame a message came from; any other URL has none.
func WebUIFrameOrigin(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User != nil || u.Port() != "" || u.Opaque != "" {
		return "", false
	}
	return NormalizeWebUIOrigin(u.Scheme + "://" + u.Host)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Proposal returns a stored proposal. This is the seam the existing approval
// path uses; the bridge itself never applies one.
func (b *WebUIBridge) Proposal(id string) (WebUIProposal, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	p, ok := b.proposals[id]
	return p, ok
}

// propose records a pending proposal with the approval mode the user set. It
// writes nothing, so a page cannot commit through the bridge.
func (b *WebUIBridge) propose(ctx context.Context, call WebUICall) (WebUIProposal, *WebUIError) {
	if b.Approvals == nil {
		return WebUIProposal{}, &WebUIError{Code: WebUICodeApprovalUnavailable,
			Message: "写操作需要审批模式，但守护进程没有审批来源"}
	}
	mode, err := b.Approvals.ApprovalMode(ctx)
	if err != nil {
		return WebUIProposal{}, &WebUIError{Code: WebUICodeApprovalUnavailable, Recoverable: true,
			Message: "无法读取审批模式：" + err.Error()}
	}
	if mode != WebUIApprovalAuto && mode != WebUIApprovalAsk {
		return WebUIProposal{}, &WebUIError{Code: WebUICodeApprovalUnavailable,
			Message: "未知的审批模式：" + mode, Details: map[string]any{"mode": mode}}
	}
	summary := call.Action.Title
	if summary == "" {
		summary = call.Action.ID
	}
	proposal := WebUIProposal{ID: webUIProposalID(call.Session.PluginID, call.Session.InstanceID, call.Message.RequestID),
		PluginID: call.Session.PluginID, InstanceID: call.Session.InstanceID, ActionID: call.Action.ID,
		Summary: summary, Preview: call.Message.Args, ApprovalMode: mode, Status: ProposalStatusPending,
		RequestID: call.Message.RequestID, TraceID: call.Message.TraceID, CreatedAt: b.now().Unix()}
	b.mu.Lock()
	if b.proposals == nil {
		b.proposals = map[string]WebUIProposal{}
	}
	b.proposals[proposal.ID] = proposal
	b.mu.Unlock()
	return proposal, nil
}

// claimRequest records one request id per page instance. It refuses a repeated
// id, so a replayed message is rejected rather than answered twice; a message
// whose nonce is not the session's never gets this far.
func (b *WebUIBridge) claimRequest(session WebUISession, msg WebUIBridgeMessage) *WebUIError {
	key := session.InstanceID + "\x00" + session.Nonce + "\x00" + msg.RequestID
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.seen == nil {
		b.seen = map[string]time.Time{}
	}
	if _, ok := b.seen[key]; ok {
		return &WebUIError{Code: WebUICodeReplayed,
			Message: "这条消息已经处理过，重试请使用新的 requestId",
			Details: map[string]any{"requestId": msg.RequestID, "instanceId": session.InstanceID}}
	}
	b.seen[key] = b.now()
	b.order = append(b.order, key)
	for len(b.order) > webUIReplayMemory {
		delete(b.seen, b.order[0])
		b.order = b.order[1:]
	}
	return nil
}

// checkSession refuses a session the host could not have issued: the plugin id,
// instance id and nonce must be present and the origin must be the plugin's own
// controlled origin.
func checkSession(session WebUISession, plugin WebUIPlugin) *WebUIError {
	switch {
	case !idRE.MatchString(plugin.ID):
		return &WebUIError{Code: WebUICodeInvalidSession, Message: "插件 id 无效：" + plugin.ID}
	case !bridgeTokenRE.MatchString(session.InstanceID):
		return &WebUIError{Code: WebUICodeInvalidSession, Message: "页面实例 id 无效：" + session.InstanceID}
	case !bridgeTokenRE.MatchString(session.Nonce):
		return &WebUIError{Code: WebUICodeInvalidSession, Message: "页面会话缺少 nonce"}
	}
	want, err := WebUIOrigin(plugin.ID)
	if err != nil {
		return &WebUIError{Code: WebUICodeInvalidSession, Message: err.Error()}
	}
	got, ok := NormalizeWebUIOrigin(session.Origin)
	if !ok || got != want {
		return &WebUIError{Code: WebUICodeInvalidSession,
			Message: fmt.Sprintf("页面会话的 Origin %q 不是插件 %s 的 Origin %s", session.Origin, plugin.ID, want),
			Details: map[string]any{"origin": session.Origin, "expected": want}}
	}
	return nil
}

// checkEnvelope refuses a message the bridge cannot key on.
func checkEnvelope(msg WebUIBridgeMessage) *WebUIError {
	switch {
	case msg.SchemaVersion != WebUIBridgeSchemaVersion:
		return &WebUIError{Code: WebUICodeSchemaVersion,
			Message: fmt.Sprintf("不支持的 schemaVersion %d：本机只接受 %d", msg.SchemaVersion, WebUIBridgeSchemaVersion),
			Details: map[string]any{"schemaVersion": msg.SchemaVersion, "accepted": WebUIBridgeSchemaVersion}}
	case !bridgeTokenRE.MatchString(msg.RequestID):
		return &WebUIError{Code: WebUICodeInvalidRequest,
			Message: "requestId 必填且只能包含字母、数字和 . _ : -"}
	case msg.TraceID != "" && !bridgeTokenRE.MatchString(msg.TraceID):
		return &WebUIError{Code: WebUICodeInvalidRequest, Message: "traceId 无效：" + msg.TraceID}
	case strings.Contains(msg.Action, "*"):
		return &WebUIError{Code: WebUICodeWildcardDenied,
			Message: "Web UI 桥不接受通配动作：每条命令都要对应插件声明的一个动作",
			Details: map[string]any{"action": msg.Action}}
	case !webUIActionIDRE.MatchString(msg.Action):
		return &WebUIError{Code: WebUICodeInvalidRequest, Message: "动作 id 无效：" + msg.Action}
	case msg.DeadlineMs < 0 || msg.DeadlineMs > WebUIMaxDeadlineMs:
		return &WebUIError{Code: WebUICodeInvalidRequest,
			Message: fmt.Sprintf("deadlineMs 必须在 0 到 %d 之间：%d", WebUIMaxDeadlineMs, msg.DeadlineMs)}
	}
	if msg.Args != nil {
		b, err := json.Marshal(msg.Args)
		if err != nil {
			return &WebUIError{Code: WebUICodeInvalidArgs, Message: "args 无法编码：" + err.Error()}
		}
		if len(b) > maxWebUIMessageBytes {
			return &WebUIError{Code: WebUICodeInvalidArgs,
				Message: fmt.Sprintf("args 超过 %d 字节", maxWebUIMessageBytes),
				Details: map[string]any{"bytes": len(b), "limit": maxWebUIMessageBytes}}
		}
	}
	return nil
}

// checkIdentity compares the message with the trusted session. The message may
// not name another plugin, another page instance or another origin, and it must
// carry the nonce the host issued for this session.
func checkIdentity(session WebUISession, plugin WebUIPlugin, msg WebUIBridgeMessage) *WebUIError {
	if msg.PluginID != session.PluginID || msg.PluginID != plugin.ID {
		return &WebUIError{Code: WebUICodeForgedIdentity,
			Message: fmt.Sprintf("消息的 pluginId %q 与页面会话的 %q 不一致", msg.PluginID, session.PluginID),
			Details: map[string]any{"pluginId": msg.PluginID, "session": session.PluginID}}
	}
	if msg.InstanceID != session.InstanceID {
		return &WebUIError{Code: WebUICodeForgedIdentity,
			Message: "消息的 instanceId 与页面会话不一致",
			Details: map[string]any{"instanceId": msg.InstanceID, "session": session.InstanceID}}
	}
	msgOrigin, ok := NormalizeWebUIOrigin(msg.Origin)
	if !ok {
		return &WebUIError{Code: WebUICodeCrossOrigin,
			Message: fmt.Sprintf("Origin %q 不是受控的插件 Origin", msg.Origin),
			Details: map[string]any{"origin": msg.Origin}}
	}
	sessionOrigin, _ := NormalizeWebUIOrigin(session.Origin)
	if msgOrigin != sessionOrigin {
		return &WebUIError{Code: WebUICodeCrossOrigin,
			Message: fmt.Sprintf("Origin %q 不是这个页面会话的 Origin %q", msg.Origin, sessionOrigin),
			Details: map[string]any{"origin": msgOrigin, "session": sessionOrigin}}
	}
	if msg.Nonce != session.Nonce {
		return &WebUIError{Code: WebUICodeNonceMismatch,
			Message: "nonce 与页面会话不一致", Recoverable: true}
	}
	return nil
}

// findWebUIAction resolves the declared action; an undeclared action is refused
// rather than forwarded.
func findWebUIAction(plugin WebUIPlugin, id string) (WebUIAction, *WebUIError) {
	for _, action := range plugin.Actions {
		if action.ID == id {
			return action, nil
		}
	}
	return WebUIAction{}, &WebUIError{Code: WebUICodeUnknownAction,
		Message: fmt.Sprintf("插件 %s 没有动作 %s", plugin.ID, id),
		Details: map[string]any{"pluginId": plugin.ID, "action": id}}
}

// checkNoSecrets refuses a message that carries a credential. The page never
// receives the daemon token, and it may not hand one back either: a bridge that
// accepted "token" or "authorization" arguments would be a proxy for the vault.
func checkNoSecrets(args map[string]any) *WebUIError {
	if key, ok := findSecretKey(args, "", 0); ok {
		return &WebUIError{Code: WebUICodeSecretInMessage,
			Message: "Web UI 消息不能携带凭据：" + key,
			Details: map[string]any{"argument": key}}
	}
	return nil
}

func findSecretKey(value any, path string, depth int) (string, bool) {
	if depth > maxWebUIArgDepth {
		return "", false
	}
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			at := key
			if path != "" {
				at = path + "." + key
			}
			if secretKeyRE.MatchString(strings.ToLower(key)) {
				return at, true
			}
			if found, ok := findSecretKey(v[key], at, depth+1); ok {
				return found, true
			}
		}
	case []any:
		for i, item := range v {
			if found, ok := findSecretKey(item, fmt.Sprintf("%s[%d]", path, i), depth+1); ok {
				return found, true
			}
		}
	}
	return "", false
}

// Web UI platform admission.
const (
	// AdmissionSupported means the platform's engine is the accepted one.
	AdmissionSupported = "supported"
	// AdmissionNeedsReview means the platform needs its own measured review.
	AdmissionNeedsReview = "needs-review"
	// AdmissionUnsupported means the platform is not in the admission matrix.
	AdmissionUnsupported = "unsupported"
)

// Fallback reason codes the app shows instead of a Web UI page.
const (
	FallbackPlatformUnsupported = "platform-unsupported"
	FallbackPlatformNeedsReview = "platform-needs-review"
	FallbackRendererMissing     = "renderer-missing"
	FallbackReviewPending       = "review-pending"
	FallbackPluginInactive      = "plugin-inactive"
	FallbackResourcesMissing    = "resources-missing"
)

// Security review states of a platform renderer.
const (
	ReviewPending = "pending"
	ReviewPassed  = "passed"
)

// WebUIAdmission is one platform's admission record: what the platform policy
// allows, whether the app has a renderer there, and whether that renderer
// passed its separate security review. A page renders for users only after the
// review passed; before that it opens only in the daemon's experimental preview,
// and the app shows the native fallback while L1/L2 stay usable.
type WebUIAdmission struct {
	Platform string `json:"platform"`
	Status   string `json:"status"`
	Engine   string `json:"engine,omitempty"`
	Renderer bool   `json:"renderer"`
	Review   string `json:"review"`
	Reason   string `json:"reason"`
}

// Available reports whether a plugin Web UI page may render on this platform
// for users.
func (a WebUIAdmission) Available() bool {
	return a.Previewable() && a.Review == ReviewPassed
}

// Previewable reports whether the experimental preview (COURSIVE_WEBUI_PREVIEW)
// may open a page here before the review passed: the platform is admitted and
// the app has a renderer for it.
func (a WebUIAdmission) Previewable() bool {
	return a.Status == AdmissionSupported && a.Renderer
}

// FallbackReason is the stable code the app explains when a page cannot render.
func (a WebUIAdmission) FallbackReason() string {
	switch {
	case a.Status == AdmissionUnsupported:
		return FallbackPlatformUnsupported
	case a.Status == AdmissionNeedsReview:
		return FallbackPlatformNeedsReview
	case !a.Renderer:
		return FallbackRendererMissing
	case a.Review != ReviewPassed:
		return FallbackReviewPending
	}
	return ""
}

// WebUIAdmissions is the platform admission matrix, sorted by platform.
func WebUIAdmissions() []WebUIAdmission {
	out := make([]WebUIAdmission, 0, len(webUIAdmissions))
	for _, a := range webUIAdmissions {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Platform < out[j].Platform })
	return out
}

// WebUIAdmissionFor returns the admission record of one platform. A platform
// that is not in the matrix is unsupported rather than optimistically admitted.
func WebUIAdmissionFor(platform string) WebUIAdmission {
	key := strings.ToLower(strings.TrimSpace(platform))
	if a, ok := webUIAdmissions[key]; ok {
		return a
	}
	return WebUIAdmission{Platform: key, Status: AdmissionUnsupported, Review: ReviewPending,
		Reason: "这个平台不在 Web UI 准入矩阵中，插件页面改用原生渲染"}
}

// webUIAdmissions is the matrix spec 0032 requires: Android and macOS are the
// platforms validated first and the app renders pages there with the system
// WebView; Windows and Linux need their engine, packaging,
// hardware-acceleration, input-method, stylus and window-layer review and have
// no renderer. Review is the security review recorded in the spec's Web UI
// release gate; TestWebUIReleaseGate keeps the two in step.
var webUIAdmissions = map[string]WebUIAdmission{
	"android": {Platform: "android", Status: AdmissionSupported, Engine: "system-webview", Renderer: true, Review: ReviewPending,
		Reason: "Android 系统 WebView 渲染器等待单独的安全评审；评审通过前只在实验预览中打开"},
	"macos": {Platform: "macos", Status: AdmissionSupported, Engine: "wkwebview", Renderer: true, Review: ReviewPending,
		Reason: "macOS WKWebView 渲染器等待单独的安全评审；评审通过前只在实验预览中打开"},
	"windows": {Platform: "windows", Status: AdmissionNeedsReview, Engine: "webview2", Review: ReviewPending,
		Reason: "需要测评 WebView2 引擎、安装器、硬件加速、输入法与窗口层级"},
	"linux": {Platform: "linux", Status: AdmissionNeedsReview, Engine: "webkitgtk", Review: ReviewPending,
		Reason: "需要测评 WebKitGTK 动态库分发、Wayland/X11 输入法、触控笔与窗口层级"},
}

var (
	webUIActionIDRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	bridgeTokenRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	secretKeyRE     = regexp.MustCompile(`^(token|access_?token|refresh_?token|id_?token|authorization|cookie|cookies|password|passwd|secret|client_?secret|api_?key|apikey|private_?key|credential|credentials|vault)$`)
)

// hasPermission reports an exact grant. There is no wildcard permission: a page
// cannot ask for "*" and receive every action.
func hasPermission(permissions []string, want string) bool {
	for _, p := range permissions {
		if p == want {
			return true
		}
	}
	return false
}

func refuse(out WebUIOutcome, err *WebUIError) WebUIOutcome {
	out.OK, out.Result, out.Error = false, nil, err
	return out
}

// deadlineErr names a deadline or cancellation failure.
func deadlineErr(ctx context.Context, deadlineMs int64) *WebUIError {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &WebUIError{Code: WebUICodeDeadlineExceeded, Message: "Web UI 动作超时", Recoverable: true,
			Details: map[string]any{"deadlineMs": deadlineMs}}
	}
	return &WebUIError{Code: WebUICodeCanceled, Message: "Web UI 动作已取消", Recoverable: true}
}

func (b *WebUIBridge) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

// webUIProposalID is derived from the session and the request, so a page names
// the same proposal for the same request.
func webUIProposalID(pluginID, instanceID, requestID string) string {
	sum := sha256.Sum256([]byte(pluginID + "\x00" + instanceID + "\x00" + requestID))
	return "proposal-" + hex.EncodeToString(sum[:8])
}
