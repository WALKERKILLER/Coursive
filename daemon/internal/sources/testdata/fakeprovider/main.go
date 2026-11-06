// fakeprovider is a source provider for tests. A file named "mode" in its
// working directory selects a behavior: ok (default), crash, hang, flood,
// oldproto.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type msg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

var (
	out           = json.NewEncoder(os.Stdout)
	in            = bufio.NewScanner(os.Stdin)
	nextID  int64 = 1000
	pending       = map[int64]chan json.RawMessage{}
)

func reply(id *int64, result any) { out.Encode(msg{JSONRPC: "2.0", ID: id, Result: result}) }

func fail(id *int64, kind, text string) {
	out.Encode(msg{JSONRPC: "2.0", ID: id, Error: map[string]any{"code": -32000, "message": text, "data": map[string]any{"kind": kind}}})
}

// ask sends a request to the daemon and waits for its answer, reading and
// queueing nothing else (the tests never interleave).
func ask(method string, params any) json.RawMessage {
	nextID++
	id := nextID
	raw, _ := json.Marshal(params)
	out.Encode(msg{JSONRPC: "2.0", ID: &id, Method: method, Params: raw})
	for in.Scan() {
		var m struct {
			ID     *int64          `json:"id"`
			Result json.RawMessage `json:"result"`
		}
		json.Unmarshal(in.Bytes(), &m)
		if m.ID != nil && *m.ID == id {
			return m.Result
		}
	}
	return nil
}

func main() {
	mode := "ok"
	if b, err := os.ReadFile("mode"); err == nil {
		mode = strings.TrimSpace(string(b))
	}
	switch mode {
	case "crash":
		os.Exit(3)
	case "flood":
		fmt.Println(`{"jsonrpc":"2.0","id":1,"result":"` + strings.Repeat("x", 2<<20) + `"}`)
		select {}
	}
	in.Buffer(make([]byte, 64<<10), 1<<20)
	for in.Scan() {
		var m msg
		if json.Unmarshal(in.Bytes(), &m) != nil {
			continue
		}
		var p map[string]any
		json.Unmarshal(m.Params, &p)
		switch m.Method {
		case "initialize":
			if mode == "hang" {
				continue
			}
			proto := "1"
			if mode == "oldproto" {
				proto = "0"
			}
			reply(m.ID, map[string]any{"protocol": proto, "capabilities": []string{"slides"},
				"sign_in": map[string]any{"fields": []map[string]any{{"key": "user", "title": "账号", "kind": "text"}, {"key": "password", "title": "密码", "kind": "password"}}}})
		case "auth.status":
			var v struct {
				Value string `json:"value"`
			}
			json.Unmarshal(ask("vault.get", map[string]any{"key": "session"}), &v)
			if v.Value != "" {
				reply(m.ID, map[string]any{"state": "signed_in", "account": map[string]any{"display": "测试用户"}})
			} else {
				reply(m.ID, map[string]any{"state": "signed_out"})
			}
		case "auth.signIn":
			fields, _ := p["fields"].(map[string]any)
			fmt.Fprintf(os.Stderr, "signing in with password=%v\n", fields["password"])
			reply(m.ID, map[string]any{"state": "needs_input", "challenge": map[string]any{"id": "c1", "kind": "image", "prompt": "输入验证码", "image": "data:image/png;base64,AA=="}})
		case "auth.answer":
			if p["answer"] != "1234" {
				fail(m.ID, "invalid", "无法登录：验证码错误")
				continue
			}
			ask("vault.put", map[string]any{"key": "session", "value": "cookie-abcdef"})
			reply(m.ID, map[string]any{"state": "signed_in", "account": map[string]any{"display": "测试用户"}})
		case "auth.signOut":
			ask("vault.delete", map[string]any{"key": "session"})
			reply(m.ID, map[string]any{})
		case "courses.list":
			if p["cursor"] == "" {
				reply(m.ID, map[string]any{"courses": []map[string]any{{"id": "c-1", "title": "线性代数"}}, "cursor": "p2"})
			} else {
				reply(m.ID, map[string]any{"courses": []map[string]any{{"id": "c-2", "title": "数据结构"}}})
			}
		case "lectures.list":
			reply(m.ID, map[string]any{"lectures": []map[string]any{{"id": "l-1", "title": "第一讲", "has_playback": true}}})
		case "media.resolve":
			reply(m.ID, map[string]any{"streams": []map[string]any{{"id": "s1", "label": "屏幕", "url": "https://media.example/a.mp4", "headers": map[string]string{"Cookie": "k=v"}}}, "seekable": true})
		case "slides.list":
			reply(m.ID, map[string]any{"slides": []map[string]any{{"t_sec": 60, "url": "https://media.example/1.jpg"}}})
		case "expire":
			fail(m.ID, "auth_required", "无法继续：登录已过期")
		case "sleep":
			// never answers
		default:
			fail(m.ID, "invalid", "unknown method "+m.Method)
		}
	}
}
