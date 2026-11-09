package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/walkerkiller/coursive/daemon/internal/pluginbridge"
	"github.com/walkerkiller/coursive/daemon/internal/pluginfabric"
)

// webRegistry is one enabled plugin with a read action, a write action and an
// action whose permission the user did not grant.
type webRegistry struct{ state string }

func (r webRegistry) Plugin(_ context.Context, id string) (pluginbridge.Plugin, bool, error) {
	if id != "board" {
		return pluginbridge.Plugin{}, false, nil
	}
	state := r.state
	if state == "" {
		state = pluginbridge.StateEnabled
	}
	return pluginbridge.Plugin{ID: "board", State: state, Permissions: []string{"library.read", "library.write"},
		Actions: []pluginbridge.Action{
			{ID: "cards", Permission: "library.read"},
			{ID: "save", Title: "保存复习卡", Permission: "library.write", Write: true},
			{ID: "launch", Permission: "exec"},
		}}, true, nil
}

type webPages struct {
	bundle pluginfabric.WebUIBundle
	err    error
}

func (p webPages) WebPage(context.Context, string, string) (pluginfabric.WebUIBundle, error) {
	return p.bundle, p.err
}

func boardBundle() pluginfabric.WebUIBundle {
	file := func(path, typ, data string) pluginfabric.WebUIFile {
		sum := sha256.Sum256([]byte(data))
		return pluginfabric.WebUIFile{Path: path, Type: typ, SHA256: hex.EncodeToString(sum[:]), Data: []byte(data)}
	}
	return pluginfabric.WebUIBundle{PluginID: "board", PageID: "main", Title: "面板", Entry: "web/app.js",
		Hosts: []string{"example.org"}, Files: []pluginfabric.WebUIFile{file("web/app.js", "text/javascript", "coursive.call('cards');")}}
}

func webServer(exec *actionExecutor, preview bool) *Server {
	return &Server{Token: "test", PluginWebPreview: preview, PluginWebPages: webPages{bundle: boardBundle()},
		PluginActions: &pluginbridge.Bridge{Registry: webRegistry{}, Executor: exec, Approvals: actionApprovals{}}}
}

func webCall(t *testing.T, s *Server, method, path string, body any, out any) int {
	t.Helper()
	var payload io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		payload = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, payload)
	req.Header.Set("Authorization", "Bearer test")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if out != nil && rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: %v (%s)", method, path, err, rec.Body.String())
		}
	}
	return rec.Code
}

func openBoard(t *testing.T, s *Server, platform string) webOpenResponse {
	t.Helper()
	var out webOpenResponse
	if code := webCall(t, s, "POST", "/api/plugin-web/sessions", map[string]string{"pluginId": "board", "pageId": "main", "platform": platform}, &out); code != http.StatusOK {
		t.Fatalf("open = %d", code)
	}
	return out
}

func TestPluginWebAdmissionAndFallbacks(t *testing.T) {
	var adm struct {
		Platforms []pluginfabric.WebUIAdmission `json:"platforms"`
		Preview   bool                          `json:"preview"`
	}
	if webCall(t, webServer(&actionExecutor{}, false), "GET", "/api/plugin-web/admission", nil, &adm); len(adm.Platforms) != 4 || adm.Preview {
		t.Fatalf("admission = %+v", adm)
	}
	for name, c := range map[string]struct {
		s        *Server
		platform string
		reason   string
	}{
		"linux needs its engine review": {webServer(&actionExecutor{}, true), "linux", pluginfabric.FallbackPlatformNeedsReview},
		"unknown platform":              {webServer(&actionExecutor{}, true), "plan9", pluginfabric.FallbackPlatformUnsupported},
		"android before its review":     {webServer(&actionExecutor{}, false), "android", pluginfabric.FallbackReviewPending},
		"plugin disabled": {&Server{Token: "test", PluginWebPreview: true, PluginWebPages: webPages{bundle: boardBundle()},
			PluginActions: &pluginbridge.Bridge{Registry: webRegistry{state: pluginbridge.StateDisabled}}}, "android", pluginfabric.FallbackPluginInactive},
		"no registry":    {&Server{Token: "test", PluginWebPreview: true, PluginWebPages: webPages{bundle: boardBundle()}}, "android", pluginfabric.FallbackPluginInactive},
		"no page source": {&Server{Token: "test", PluginWebPreview: true, PluginActions: &pluginbridge.Bridge{Registry: webRegistry{}}}, "android", pluginfabric.FallbackResourcesMissing},
		"host refuses":   {&Server{Token: "test", PluginWebPreview: true, PluginWebPages: webPages{err: errors.New("网页资源 web/app.js 与安装时不一致")}, PluginActions: &pluginbridge.Bridge{Registry: webRegistry{}}}, "android", pluginfabric.FallbackResourcesMissing},
		"tampered bundle": {func() *Server {
			b := boardBundle()
			b.Files[0].Data = []byte("fetch('/')")
			s := webServer(&actionExecutor{}, true)
			s.PluginWebPages = webPages{bundle: b}
			return s
		}(), "android", pluginfabric.FallbackResourcesMissing},
	} {
		out := openBoard(t, c.s, c.platform)
		if out.Fallback == nil || out.Fallback.Reason != c.reason || out.Session != nil || out.HTML != "" {
			t.Errorf("%s: %+v", name, out)
		}
	}
	if code := webCall(t, webServer(&actionExecutor{}, true), "POST", "/api/plugin-web/sessions", map[string]any{"pluginId": "board", "token": "x"}, nil); code != http.StatusBadRequest {
		t.Fatalf("an unknown field was accepted: %d", code)
	}
}

func TestPluginWebSessionMessages(t *testing.T) {
	exec := &actionExecutor{result: pluginbridge.Result{Notification: &pluginbridge.Notification{Text: "3 张卡片"}}}
	s := webServer(exec, true)
	out := openBoard(t, s, "android")
	if out.Fallback != nil || out.Session == nil || !out.Preview {
		t.Fatalf("preview did not open the page: %+v", out)
	}
	sess := out.Session
	if sess.Origin != "coursive-plugin://board" || sess.BaseURL != "coursive-plugin://board/" || !strings.Contains(out.HTML, sess.InstanceID) {
		t.Fatalf("session = %+v", sess)
	}
	if strings.Contains(out.HTML, "test") || strings.Contains(out.HTML, "127.0.0.1") {
		t.Fatal("the page carries the daemon token or address")
	}
	nonce := between(out.HTML, `"nonce":"`, `"`)
	base := "/api/plugin-web/sessions/" + sess.ID
	msg := func(edit func(map[string]any)) string {
		m := map[string]any{"schemaVersion": 1, "origin": sess.Origin, "pluginId": "board", "instanceId": sess.InstanceID,
			"nonce": nonce, "requestId": sess.InstanceID + ".1", "action": "cards", "args": map[string]any{}, "deadlineMs": 0}
		if edit != nil {
			edit(m)
		}
		b, _ := json.Marshal(m)
		return string(b)
	}
	send := func(frame, message string) pluginbridge.Response {
		t.Helper()
		var resp pluginbridge.Response
		if code := webCall(t, s, "POST", base+"/messages", map[string]string{"frameUrl": frame, "message": message}, &resp); code != http.StatusOK {
			t.Fatalf("message = %d", code)
		}
		return resp
	}
	frame := sess.BaseURL

	// A read runs once through the action bridge and answers with the page's own request id.
	if resp := send(frame, msg(nil)); !resp.OK || resp.RequestID != sess.InstanceID+".1" || resp.Result.Notification.Text != "3 张卡片" || exec.calls != 1 {
		t.Fatalf("read = %+v (calls %d)", resp, exec.calls)
	}
	for name, c := range map[string]struct {
		frame, message, code string
	}{
		"replayed":            {frame, msg(nil), pluginfabric.WebUICodeReplayed},
		"loopback frame":      {"http://127.0.0.1:4173/", msg(func(m map[string]any) { m["requestId"] = "r2" }), pluginfabric.WebUICodeCrossOrigin},
		"another plugin":      {"coursive-plugin://other/", msg(func(m map[string]any) { m["requestId"] = "r3" }), pluginfabric.WebUICodeCrossOrigin},
		"forged plugin":       {frame, msg(func(m map[string]any) { m["requestId"], m["pluginId"] = "r4", "other" }), pluginfabric.WebUICodeForgedIdentity},
		"forged instance":     {frame, msg(func(m map[string]any) { m["requestId"], m["instanceId"] = "r5", "web-other" }), pluginfabric.WebUICodeForgedIdentity},
		"forged origin":       {frame, msg(func(m map[string]any) { m["requestId"], m["origin"] = "r6", "coursive-plugin://other" }), pluginfabric.WebUICodeCrossOrigin},
		"wrong nonce":         {frame, msg(func(m map[string]any) { m["requestId"], m["nonce"] = "r7", "guess" }), pluginfabric.WebUICodeNonceMismatch},
		"schema version":      {frame, msg(func(m map[string]any) { m["requestId"], m["schemaVersion"] = "r8", 2 }), pluginfabric.WebUICodeSchemaVersion},
		"undeclared action":   {frame, msg(func(m map[string]any) { m["requestId"], m["action"] = "r9", "library.delete" }), pluginfabric.WebUICodeUnknownAction},
		"wildcard":            {frame, msg(func(m map[string]any) { m["requestId"], m["action"] = "r10", "*" }), pluginfabric.WebUICodeWildcardDenied},
		"ungranted":           {frame, msg(func(m map[string]any) { m["requestId"], m["action"] = "r11", "launch" }), pluginfabric.WebUICodePermissionDenied},
		"token argument":      {frame, msg(func(m map[string]any) { m["requestId"], m["args"] = "r12", map[string]any{"authorization": "Bearer x"} }), pluginfabric.WebUICodeSecretInMessage},
		"daemon route field":  {frame, msg(func(m map[string]any) { m["requestId"], m["url"] = "r13", "/api/settings" }), pluginfabric.WebUICodeInvalidRequest},
		"not json":            {frame, "{", pluginfabric.WebUICodeInvalidRequest},
		"deadline over limit": {frame, msg(func(m map[string]any) { m["requestId"], m["deadlineMs"] = "r14", 60_000 }), pluginfabric.WebUICodeInvalidRequest},
	} {
		if resp := send(c.frame, c.message); resp.OK || resp.Error == nil || resp.Error.Code != c.code {
			t.Errorf("%s: %+v", name, resp.Error)
		}
	}
	if exec.calls != 1 {
		t.Fatalf("a refused message reached the executor (%d calls)", exec.calls)
	}

	// A write is a pending proposal on the existing approval path; nothing runs.
	resp := send(frame, msg(func(m map[string]any) { m["requestId"], m["action"] = "w1", "save" }))
	if !resp.OK || resp.Result.Proposal == nil || resp.Result.Proposal.Status != pluginbridge.ProposalPending || exec.calls != 1 {
		t.Fatalf("write = %+v (calls %d)", resp, exec.calls)
	}
	if _, ok := s.PluginActions.Proposal(resp.Result.Proposal.ID); !ok {
		t.Fatal("the proposal is not on the action bridge")
	}

	var nav pluginfabric.WebUIURLDecision
	for target, want := range map[string]string{
		"https://example.org/doc":        pluginfabric.WebUIURLExternal,
		"https://evil.example/":          pluginfabric.WebUIURLDeny,
		"http://127.0.0.1:4173/api":      pluginfabric.WebUIURLDeny,
		"coursive-plugin://other/":       pluginfabric.WebUIURLDeny,
		"javascript:alert(1)":            pluginfabric.WebUIURLDeny,
		"coursive-plugin://board/#cards": pluginfabric.WebUIURLSameOrigin,
	} {
		if webCall(t, s, "POST", base+"/navigate", map[string]string{"url": target}, &nav); nav.Decision != want {
			t.Errorf("navigate %s = %+v, want %s", target, nav, want)
		}
	}

	// A closed session answers nothing.
	webCall(t, s, "DELETE", base, nil, nil)
	if resp := send(frame, msg(func(m map[string]any) { m["requestId"] = "r20" })); resp.OK || resp.Error.Code != pluginfabric.WebUICodeInvalidSession {
		t.Fatalf("a closed session answered: %+v", resp)
	}
	if code := webCall(t, s, "POST", base+"/navigate", map[string]string{"url": "https://example.org/"}, nil); code != http.StatusNotFound {
		t.Fatalf("navigate on a closed session = %d", code)
	}
}

func between(s, start, end string) string {
	_, rest, _ := strings.Cut(s, start)
	v, _, _ := strings.Cut(rest, end)
	return v
}
