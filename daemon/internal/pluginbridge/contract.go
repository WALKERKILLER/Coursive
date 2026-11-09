// Package pluginbridge is the daemon side of the Plugin Fabric action bridge.
// A declarative UI action arrives from the app, the bridge checks the plugin
// state, the declared permission, the argument shape, the target revision, the
// deadline and the approval mode, and answers with at most one of statePatch,
// navigation, viewPatch, notification or proposal.
// Contract: docs/specs/0032-ui-extension-protocol.md (Action bridge and UI state).
package pluginbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

const (
	// DefaultDeadline bounds an action that names no deadline.
	DefaultDeadline = 10_000
	// MaxDeadlineMs is the longest deadline a request may ask for.
	MaxDeadlineMs int64 = 60_000
	// maxArgs caps the argument payload a declarative action may carry.
	maxArgs = 256 << 10
	// keep bounds remembered responses and pending proposals.
	keep = 256
)

// Plugin states (docs/specs/0031-plugin-fabric-core.md, Plugin identity and state).
const (
	StateInstalled = "installed"
	StateEnabled   = "enabled"
	StateResolving = "resolving"
	StateReady     = "ready"
	StateDegraded  = "degraded"
	StateFailed    = "failed"
	StateDisabled  = "disabled"
)

// Approval modes (docs/specs/0016-agent-acts.md).
const (
	ApprovalAuto = "auto"
	ApprovalAsk  = "ask"
)

// Proposal status; the bridge only ever stores pending proposals, because a
// write commits through the existing approval path, never here.
const ProposalPending = "pending"

// Error codes a response may carry.
const (
	CodeUnavailable         = "unavailable"
	CodeInvalidRequest      = "invalid_request"
	CodeUnknownPlugin       = "unknown_plugin"
	CodePluginInactive      = "plugin_inactive"
	CodeUnknownAction       = "unknown_action"
	CodePermissionDenied    = "permission_denied"
	CodeInvalidArgs         = "invalid_args"
	CodeRevisionConflict    = "revision_conflict"
	CodeDeadlineExceeded    = "deadline_exceeded"
	CodeCanceled            = "canceled"
	CodeApprovalUnavailable = "approval_unavailable"
	CodeInvalidResult       = "invalid_result"
	CodeInternal            = "internal"
)

// Parameter types an action schema may declare.
const (
	TypeString  = "string"
	TypeNumber  = "number"
	TypeInteger = "integer"
	TypeBoolean = "boolean"
	TypeObject  = "object"
	TypeArray   = "array"
)

var (
	pluginIDRE = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)
	actionIDRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	tokenRE    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	routeRE    = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,63}$`)
	traceIDRE  = regexp.MustCompile(`^[A-Za-z0-9._:-]{0,128}$`)
	viewIDRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

// Request is one action invocation from a declarative view.
type Request struct {
	PluginID        string         `json:"pluginId"`
	ActionID        string         `json:"actionId"`
	ViewID          string         `json:"viewId"`
	InstanceID      string         `json:"instanceId"`
	Args            map[string]any `json:"args"`
	ContextRevision int64          `json:"contextRevision"`
	RequestID       string         `json:"requestId"`
	DeadlineMs      int64          `json:"deadlineMs"`
	TraceID         string         `json:"traceId"`
}

// Response is the bridge's answer: result and error are exclusive, and a
// handled outcome is always HTTP 200 so the app reads the code, not a status.
type Response struct {
	OK        bool    `json:"ok"`
	RequestID string  `json:"requestId,omitempty"`
	TraceID   string  `json:"traceId,omitempty"`
	Result    *Result `json:"result,omitempty"`
	Error     *Error  `json:"error,omitempty"`
}

// Error is a diagnosable failure. Recoverable marks the cases the view can
// retry after reloading: a stale revision, a timeout or a cancellation.
type Error struct {
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	Details     map[string]any `json:"details,omitempty"`
	Recoverable bool           `json:"recoverable,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Result is what an action produced; at most one form is set.
type Result struct {
	StatePatch   *StatePatch   `json:"statePatch,omitempty"`
	Navigation   *Navigation   `json:"navigation,omitempty"`
	ViewPatch    *ViewPatch    `json:"viewPatch,omitempty"`
	Notification *Notification `json:"notification,omitempty"`
	Proposal     *Proposal     `json:"proposal,omitempty"`
}

// StatePatch replaces top-level keys of a view's plugin state; a nil value
// removes a key. The bridge fills Revision, IdempotencyKey and State, so a
// stale or repeated patch never lands twice.
type StatePatch struct {
	ViewID         string         `json:"viewId"`
	Patch          map[string]any `json:"patch"`
	Revision       int64          `json:"revision"`
	IdempotencyKey string         `json:"idempotencyKey"`
	State          map[string]any `json:"state"`
}

// ViewSnapshot is a view's plugin state at its current revision.
type ViewSnapshot struct {
	ViewID   string         `json:"viewId"`
	Revision int64          `json:"revision"`
	State    map[string]any `json:"state"`
}

// Navigation asks the host to open a route it already knows.
type Navigation struct {
	Route   string `json:"route"`
	Title   string `json:"title,omitempty"`
	Replace bool   `json:"replace,omitempty"`
}

// ViewPatch replaces part of a view the host renders.
type ViewPatch struct {
	ViewID   string         `json:"viewId"`
	Revision int64          `json:"revision"`
	Patch    map[string]any `json:"patch"`
}

// Notification is a short message for the user.
type Notification struct {
	Text  string `json:"text"`
	Level string `json:"level,omitempty"` // info | warning | error
}

// Proposal is a write the user has not authorized yet. Only the existing
// approval path applies it; the bridge stores it pending and never writes.
type Proposal struct {
	ID           string         `json:"id"`
	PluginID     string         `json:"pluginId"`
	ActionID     string         `json:"actionId"`
	ViewID       string         `json:"viewId,omitempty"`
	InstanceID   string         `json:"instanceId,omitempty"`
	Summary      string         `json:"summary"`
	Preview      map[string]any `json:"preview,omitempty"`
	ApprovalMode string         `json:"approvalMode"`
	Status       string         `json:"status"`
	RequestID    string         `json:"requestId"`
	TraceID      string         `json:"traceId"`
	CreatedAt    int64          `json:"createdAt"`
}

// Plugin is the daemon-owned state of one plugin: its registry state, the
// permissions the user granted and the actions its contributions declared.
type Plugin struct {
	ID          string
	State       string
	Permissions []string
	Actions     []Action
}

// Action is one declared action.
type Action struct {
	ID string
	// Title is what the user reads on the proposal.
	Title string
	// Permission is the declared permission the action needs; empty means the
	// action only touches plugin UI state.
	Permission string
	// Write marks an action that changes domain data; it always answers with a
	// pending proposal and never reaches the Executor.
	Write bool
	// Params is the argument shape; nil accepts any object.
	Params *Schema
}

// Schema is the small parameter shape an action declares. It is not JSON
// Schema: the bridge only checks presence, type and undeclared keys.
type Schema struct {
	Required   []string          `json:"required,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	Additional bool              `json:"additional,omitempty"`
}

// Registry is the plugin state the bridge trusts. Until the Plugin Fabric
// store lands it is the assistant host's (agent.Plugins, decision 0034); a nil
// Registry answers "unavailable" instead of trusting the request. An error
// means the state could not be read and is reported as recoverable.
type Registry interface {
	Plugin(ctx context.Context, id string) (Plugin, bool, error)
}

// Executor runs a read action.
type Executor interface {
	Execute(ctx context.Context, call Call) (Result, error)
}

// Approvals reports the user's approval mode. A nil Approvals refuses writes
// rather than assuming 自动.
type Approvals interface {
	ApprovalMode(ctx context.Context) (string, error)
}

// Call is a validated action handed to the Executor.
type Call struct {
	Plugin  Plugin
	Action  Action
	Request Request
	// State is the view state at the revision the caller read, so an action
	// can compute its patch; empty when the request names no view.
	State          map[string]any
	Deadline       time.Time
	IdempotencyKey string
}

// validate checks the request envelope. A missing requestId is refused
// because replay protection needs a key.
func (r Request) validate() *Error {
	switch {
	case !pluginIDRE.MatchString(r.PluginID):
		return invalid("pluginId 无效：%q", r.PluginID)
	case !actionIDRE.MatchString(r.ActionID):
		return invalid("actionId 无效：%q", r.ActionID)
	case r.ViewID != "" && !viewIDRE.MatchString(r.ViewID):
		return invalid("viewId 无效：%q", r.ViewID)
	case r.InstanceID != "" && !tokenRE.MatchString(r.InstanceID):
		return invalid("instanceId 无效：%q", r.InstanceID)
	case !tokenRE.MatchString(r.RequestID):
		return invalid("requestId 必填且只能包含字母、数字和 . _ : -：%q", r.RequestID)
	case !traceIDRE.MatchString(r.TraceID):
		return invalid("traceId 无效：%q", r.TraceID)
	case r.ContextRevision < 0:
		return invalid("contextRevision 不能为负：%d", r.ContextRevision)
	case r.DeadlineMs < 0 || r.DeadlineMs > MaxDeadlineMs:
		return invalid("deadlineMs 必须在 0 到 %d 之间：%d", MaxDeadlineMs, r.DeadlineMs)
	}
	return nil
}

// validateArgs checks the argument object against the declared shape.
func (r Request) validateArgs(a Action) *Error {
	args := r.Args
	if args == nil {
		args = map[string]any{}
	}
	if b, err := json.Marshal(args); err != nil {
		return invalid("args 无法编码：%v", err)
	} else if len(b) > maxArgs {
		return &Error{Code: CodeInvalidArgs, Message: fmt.Sprintf("args 超过 %d 字节", maxArgs),
			Details: map[string]any{"bytes": len(b), "limit": maxArgs}}
	}
	if a.Params == nil {
		return nil
	}
	for _, key := range a.Params.Required {
		if _, ok := args[key]; !ok {
			return &Error{Code: CodeInvalidArgs, Message: "缺少参数 " + key,
				Details: map[string]any{"missing": key, "actionId": a.ID}}
		}
	}
	for key, value := range args {
		want, declared := a.Params.Properties[key]
		if !declared {
			if a.Params.Additional {
				continue
			}
			return &Error{Code: CodeInvalidArgs, Message: "动作 " + a.ID + " 不接受参数 " + key,
				Details: map[string]any{"unexpected": key, "actionId": a.ID}}
		}
		if !matchesType(value, want) {
			return &Error{Code: CodeInvalidArgs, Message: fmt.Sprintf("参数 %s 应为 %s", key, want),
				Details: map[string]any{"parameter": key, "want": want, "actionId": a.ID}}
		}
	}
	return nil
}

// matchesType reports whether a decoded JSON value has the declared type.
func matchesType(v any, want string) bool {
	switch want {
	case TypeString:
		_, ok := v.(string)
		return ok
	case TypeBoolean:
		_, ok := v.(bool)
		return ok
	case TypeNumber:
		switch v.(type) {
		case float64, int, int64, json.Number:
			return true
		}
		return false
	case TypeInteger:
		switch n := v.(type) {
		case float64:
			return n == float64(int64(n))
		case int, int64, json.Number:
			return true
		}
		return false
	case TypeObject:
		_, ok := v.(map[string]any)
		return ok
	case TypeArray:
		_, ok := v.([]any)
		return ok
	}
	return false
}

// deadlineMs is the effective request deadline: the default when none is
// named, capped at the bridge's maximum.
func (r Request) deadlineMs(max int64) int64 {
	ms := r.DeadlineMs
	if ms == 0 {
		ms = DefaultDeadline
	}
	if ms > max {
		ms = max
	}
	return ms
}

func invalid(format string, args ...any) *Error {
	return &Error{Code: CodeInvalidRequest, Message: fmt.Sprintf(format, args...)}
}

// contextErr names a deadline or cancellation failure.
func contextErr(ctx context.Context, deadlineMs int64) *Error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &Error{Code: CodeDeadlineExceeded, Message: "插件动作超时", Recoverable: true,
			Details: map[string]any{"deadlineMs": deadlineMs}}
	}
	return &Error{Code: CodeCanceled, Message: "插件动作已取消", Recoverable: true}
}
