package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/walkerkiller/coursive/daemon/internal/pluginbridge"
)

// maxHostBody bounds an answer the daemon reads from the host.
const maxHostBody = 1 << 20

// ErrNotReady reports a host that is not running.
var ErrNotReady = errors.New("助手未就绪")

// HostError is a non-2xx answer from the host.
type HostError struct {
	Status  int
	Message string
}

func (e *HostError) Error() string { return fmt.Sprintf("助手返回 %d：%s", e.Status, e.Message) }

// Call sends one JSON request to the running host and decodes a 2xx answer
// into out (when non-nil).
func (s *Supervisor) Call(ctx context.Context, method, path string, in, out any) error {
	st := s.Status()
	if st.State != "ready" || st.Port == 0 {
		return ErrNotReady
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("http://127.0.0.1:%d%s", st.Port, path), body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxHostBody))
	if err != nil {
		return err
	}
	if res.StatusCode/100 != 2 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		return &HostError{Status: res.StatusCode, Message: e.Error}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// Plugins backs the plugin action bridge with the assistant host until the
// Plugin Fabric store owns plugin state (docs/decisions/0034): the host's
// plugin list is the registry, and its bridge route runs read actions. Only
// v3 plugins declare bridge actions; a v1 plugin keeps its own action route.
// Contract: docs/specs/0032-ui-extension-protocol.md (Action bridge and UI state).
type Plugins struct{ Agent *Supervisor }

// hostPlugin is the part of the host's plugin view the bridge reads.
type hostPlugin struct {
	ID          string `json:"id"`
	Schema      int    `json:"schema"`
	State       string `json:"state"`
	Permissions []struct {
		ID string `json:"id"`
	} `json:"permissions"`
	NeedsPermissions []struct {
		ID string `json:"id"`
	} `json:"needs_permissions"`
	Contributes struct {
		Actions []struct {
			ID         string               `json:"id"`
			Title      string               `json:"title"`
			Permission string               `json:"permission"`
			Write      bool                 `json:"write"`
			Params     *pluginbridge.Schema `json:"params"`
		} `json:"actions"`
	} `json:"contributes"`
}

// hostStates maps the host's plugin states to the bridge's; anything else is failed.
var hostStates = map[string]string{
	"enabled":       pluginbridge.StateEnabled,
	"disabled":      pluginbridge.StateDisabled,
	"needs_consent": pluginbridge.StateInstalled,
	"error":         pluginbridge.StateFailed,
}

// Plugin reads one plugin's state, granted permissions and v3 actions.
func (p *Plugins) Plugin(ctx context.Context, id string) (pluginbridge.Plugin, bool, error) {
	var list struct {
		Plugins []hostPlugin `json:"plugins"`
	}
	if err := p.Agent.Call(ctx, http.MethodGet, "/plugins", nil, &list); err != nil {
		return pluginbridge.Plugin{}, false, err
	}
	for _, h := range list.Plugins {
		if h.ID != id {
			continue
		}
		state, ok := hostStates[h.State]
		if !ok {
			state = pluginbridge.StateFailed
		}
		missing := map[string]bool{}
		for _, m := range h.NeedsPermissions {
			missing[m.ID] = true
		}
		out := pluginbridge.Plugin{ID: h.ID, State: state}
		for _, perm := range h.Permissions {
			if !missing[perm.ID] {
				out.Permissions = append(out.Permissions, perm.ID)
			}
		}
		if h.Schema == 3 {
			for _, a := range h.Contributes.Actions {
				out.Actions = append(out.Actions, pluginbridge.Action{ID: a.ID, Title: a.Title,
					Permission: a.Permission, Write: a.Write, Params: a.Params})
			}
		}
		return out, true, nil
	}
	return pluginbridge.Plugin{}, false, nil
}

// Execute runs a read action in the host with the arguments, the view state
// and what is left of the deadline.
func (p *Plugins) Execute(ctx context.Context, call pluginbridge.Call) (pluginbridge.Result, error) {
	state := call.State
	if state == nil {
		state = map[string]any{}
	}
	in := map[string]any{"args": call.Request.Args, "state": state, "viewId": call.Request.ViewID,
		"instanceId": call.Request.InstanceID, "deadlineMs": max(time.Until(call.Deadline).Milliseconds(), 1)}
	var result pluginbridge.Result
	path := "/plugins/" + url.PathEscape(call.Plugin.ID) + "/bridge/" + url.PathEscape(call.Action.ID)
	err := p.Agent.Call(ctx, http.MethodPost, path, in, &result)
	var he *HostError
	switch {
	case err == nil:
		return result, nil
	case errors.Is(err, ErrNotReady):
		return result, &pluginbridge.Error{Code: pluginbridge.CodeUnavailable, Message: err.Error(), Recoverable: true}
	case errors.As(err, &he):
		return result, hostActionErr(call, he)
	}
	return result, err
}

// hostActionErr names a host refusal with the bridge's codes.
func hostActionErr(call pluginbridge.Call, he *HostError) *pluginbridge.Error {
	details := map[string]any{"pluginId": call.Plugin.ID, "actionId": call.Action.ID, "status": he.Status}
	e := &pluginbridge.Error{Code: pluginbridge.CodeInternal, Message: he.Message, Details: details}
	switch he.Status {
	case http.StatusNotFound:
		e.Code = pluginbridge.CodeUnknownAction
	case http.StatusConflict:
		// The plugin was disabled or failed after the registry read.
		e.Code, e.Recoverable = pluginbridge.CodePluginInactive, true
	case http.StatusBadGateway:
		e.Code = pluginbridge.CodeInvalidResult
	case http.StatusGatewayTimeout:
		e.Code, e.Recoverable = pluginbridge.CodeDeadlineExceeded, true
	}
	if e.Message == "" {
		e.Message = he.Error()
	}
	return e
}
