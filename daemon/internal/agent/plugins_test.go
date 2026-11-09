package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/walkerkiller/coursive/daemon/internal/pluginbridge"
)

// fakeHost answers the host routes the bridge uses: one v3 plugin with a read
// action, a write action and a slow one, one v1 plugin and one awaiting consent.
func fakeHost(t *testing.T) (*Supervisor, *[]map[string]any) {
	t.Helper()
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /plugins":
			w.Write([]byte(`{"plugins":[
				{"id":"cards","schema":3,"state":"enabled","permissions":[{"id":"library.read"},{"id":"library.write"}],"needs_permissions":[],
				 "contributes":{"actions":[
					{"id":"next","title":"下一张","action":"next","write":false,"params":{"properties":{"step":"integer"}}},
					{"id":"save","title":"保存","action":"save","permission":"library.write","write":true},
					{"id":"slow","title":"慢","action":"slow","write":false}]}},
				{"id":"legacy","schema":1,"state":"enabled","permissions":[],"needs_permissions":[],
				 "contributes":{"actions":[{"id":"hello","title":"你好","surface":"composer.plus","action":"hello"}]}},
				{"id":"pending","schema":3,"state":"needs_consent","permissions":[{"id":"library.read"}],"needs_permissions":[{"id":"library.read"}],
				 "contributes":{"actions":[{"id":"go","title":"走","action":"go","write":false}]}}]}`))
		case "POST /plugins/cards/bridge/next":
			var in map[string]any
			json.NewDecoder(r.Body).Decode(&in)
			bodies = append(bodies, in)
			state, _ := in["state"].(map[string]any)
			index, _ := state["index"].(float64)
			step, _ := in["args"].(map[string]any)["step"].(float64)
			json.NewEncoder(w).Encode(map[string]any{"statePatch": map[string]any{"viewId": in["viewId"], "patch": map[string]any{"index": index + step}}})
		case "POST /plugins/cards/bridge/slow":
			w.WriteHeader(http.StatusGatewayTimeout)
			w.Write([]byte(`{"error":"插件动作超时"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	s := &Supervisor{Token: "secret"}
	s.set(Status{State: "ready", Port: port})
	return s, &bodies
}

func bridgeFor(s *Supervisor) *pluginbridge.Bridge {
	p := &Plugins{Agent: s}
	return &pluginbridge.Bridge{Registry: p, Executor: p}
}

func TestPluginsBridgeRunsHostActionsWithViewState(t *testing.T) {
	sup, bodies := fakeHost(t)
	b := bridgeFor(sup)
	ctx := context.Background()
	next := func(id string, rev int64) pluginbridge.Response {
		return b.Handle(ctx, pluginbridge.Request{PluginID: "cards", ActionID: "next", ViewID: "deck",
			Args: map[string]any{"step": 2}, ContextRevision: rev, RequestID: id})
	}
	first := next("r1", 0)
	if !first.OK || first.Result.StatePatch.Revision != 1 || first.Result.StatePatch.State["index"] != 2.0 {
		t.Fatalf("first = %+v %+v", first, first.Error)
	}
	second := next("r2", 1)
	if !second.OK || second.Result.StatePatch.State["index"] != 4.0 {
		t.Fatalf("second = %+v %+v", second, second.Error)
	}
	// The host saw the state the caller read, and a deadline.
	if got := (*bodies)[1]["state"].(map[string]any)["index"]; got != 2.0 {
		t.Fatalf("host state = %v", got)
	}
	if ms, _ := (*bodies)[1]["deadlineMs"].(float64); ms <= 0 || ms > pluginbridge.DefaultDeadline {
		t.Fatalf("deadlineMs = %v", (*bodies)[1]["deadlineMs"])
	}
	if stale := next("r3", 0); stale.OK || stale.Error.Code != pluginbridge.CodeRevisionConflict {
		t.Fatalf("stale = %+v", stale.Error)
	}
	if bad := b.Handle(ctx, pluginbridge.Request{PluginID: "cards", ActionID: "next", ViewID: "deck",
		Args: map[string]any{"step": "two"}, ContextRevision: 2, RequestID: "r4"}); bad.Error.Code != pluginbridge.CodeInvalidArgs {
		t.Fatalf("typed params = %+v", bad.Error)
	}
	if len(*bodies) != 2 {
		t.Fatalf("host ran %d times, want 2", len(*bodies))
	}
}

func TestPluginsBridgeRefusals(t *testing.T) {
	sup, bodies := fakeHost(t)
	b := bridgeFor(sup)
	ctx := context.Background()
	call := func(plugin, action string) *pluginbridge.Error {
		return b.Handle(ctx, pluginbridge.Request{PluginID: plugin, ActionID: action, RequestID: plugin + "-" + action}).Error
	}
	for _, c := range []struct{ plugin, action, code string }{
		// No approval source is wired, so a write never runs.
		{"cards", "save", pluginbridge.CodeApprovalUnavailable},
		{"cards", "slow", pluginbridge.CodeDeadlineExceeded},
		{"cards", "nope", pluginbridge.CodeUnknownAction},
		// A v1 plugin declares no bridge actions.
		{"legacy", "hello", pluginbridge.CodeUnknownAction},
		{"pending", "go", pluginbridge.CodePluginInactive},
		{"absent", "go", pluginbridge.CodeUnknownPlugin},
	} {
		if err := call(c.plugin, c.action); err == nil || err.Code != c.code {
			t.Errorf("%s/%s = %+v, want %s", c.plugin, c.action, err, c.code)
		}
	}
	if len(*bodies) != 0 {
		t.Fatalf("host ran a refused action: %v", *bodies)
	}
	sup.set(Status{State: "crashed"})
	if err := call("cards", "next"); err == nil || err.Code != pluginbridge.CodeUnavailable || !err.Recoverable {
		t.Fatalf("host down = %+v", err)
	}
}
