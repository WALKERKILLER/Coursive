package pluginfabric

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
)

func webFile(path, typ, data string) WebUIFile {
	sum := sha256.Sum256([]byte(data))
	return WebUIFile{Path: path, Type: typ, SHA256: hex.EncodeToString(sum[:]), Data: []byte(data)}
}

func testBundle() WebUIBundle {
	return WebUIBundle{PluginID: "virtual-lab", PageID: "board", Title: "面板 <b>", Entry: "web/app.js", Styles: []string{"web/app.css"},
		Files: []WebUIFile{
			webFile("web/app.js", "text/javascript", "document.body.dataset.ready = '1';"),
			webFile("web/app.css", "text/css", "body { margin: 0 }"),
			webFile("web/dot.svg", "image/svg+xml", "<svg/>"),
		}}
}

func TestWebUIBundleCheck(t *testing.T) {
	if err := testBundle().Check("virtual-lab", "board"); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		edit func(*WebUIBundle)
		want string
	}{
		"another page":  {func(b *WebUIBundle) { b.PageID = "other" }, "不是"},
		"escape":        {func(b *WebUIBundle) { b.Files[2].Path = "web/../../etc/passwd" }, "路径无效"},
		"absolute":      {func(b *WebUIBundle) { b.Files[2].Path = "/etc/passwd" }, "路径无效"},
		"backslash":     {func(b *WebUIBundle) { b.Files[2].Path = `web\dot.svg` }, "路径无效"},
		"html":          {func(b *WebUIBundle) { b.Files[2] = webFile("web/x.html", "text/html", "<script>") }, "不受支持"},
		"type mismatch": {func(b *WebUIBundle) { b.Files[2].Type = "text/javascript" }, "不受支持"},
		"hash":          {func(b *WebUIBundle) { b.Files[0].Data = []byte("fetch('http://127.0.0.1')") }, "哈希不一致"},
		"duplicate":     {func(b *WebUIBundle) { b.Files = append(b.Files, b.Files[0]) }, "重复"},
		"no entry":      {func(b *WebUIBundle) { b.Entry = "web/missing.js" }, "页面脚本"},
		"style is css":  {func(b *WebUIBundle) { b.Styles = []string{"web/app.js"} }, "页面样式"},
		"too large": {func(b *WebUIBundle) {
			b.Files[2] = webFile("web/big.txt", "text/plain", strings.Repeat("x", maxWebUIBytes))
		}, "合计超过"},
	} {
		b := testBundle()
		b.Files = append([]WebUIFile(nil), b.Files...)
		c.edit(&b)
		if err := b.Check("virtual-lab", "board"); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}

func TestBuildWebUIPage(t *testing.T) {
	session, err := NewWebUISession("virtual-lab")
	if err != nil {
		t.Fatal(err)
	}
	if session.Origin != "coursive-plugin://virtual-lab" || !bridgeTokenRE.MatchString(session.InstanceID) || !bridgeTokenRE.MatchString(session.Nonce) {
		t.Fatalf("session = %+v", session)
	}
	other, _ := NewWebUISession("virtual-lab")
	if other.InstanceID == session.InstanceID || other.Nonce == session.Nonce {
		t.Fatal("two sessions share an instance id or a nonce")
	}
	page, err := BuildWebUIPage(testBundle(), session)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`script-src &#39;nonce-([0-9a-f]{32})&#39;`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no CSP nonce in the page:\n%s", page)
	}
	if m[1] == session.Nonce {
		t.Fatal("the CSP nonce is the bridge nonce")
	}
	// Every script and style element carries the CSP nonce; nothing else runs.
	for _, tag := range regexp.MustCompile(`<(script|style)[^>]*>`).FindAllString(page, -1) {
		if tag != `<script nonce="`+m[1]+`">` && tag != `<style nonce="`+m[1]+`">` {
			t.Fatalf("element without the page nonce: %s", tag)
		}
	}
	if strings.Count(page, "<script") != 2 || strings.Count(page, "<style") != 1 {
		t.Fatalf("unexpected elements:\n%s", page)
	}
	for _, want := range []string{
		`<meta http-equiv="Content-Security-Policy" content="default-src &#39;none&#39;`,
		`<title>面板 &lt;b&gt;</title>`,
		`"instanceId":"` + session.InstanceID + `"`,
		`"nonce":"` + session.Nonce + `"`,
		`"web/dot.svg":"data:image/svg+xml;base64,PHN2Zy8+"`,
		"document.body.dataset.ready = '1';",
		"body { margin: 0 }",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("page is missing %q:\n%s", want, page)
		}
	}
	// The page names no daemon address and no credential.
	for _, unwanted := range []string{"127.0.0.1", "localhost", "Authorization", "Bearer", "token"} {
		if strings.Contains(page, unwanted) {
			t.Fatalf("page contains %q", unwanted)
		}
	}

	for name, c := range map[string]struct {
		path, data string
	}{
		"script close": {"web/app.js", "x = '</SCRIPT><script>alert(1)</script>';"},
		"html comment": {"web/app.js", "x = '<!--';"},
		"style close":  {"web/app.css", "body{}</style><script>alert(1)</script>"},
	} {
		b := testBundle()
		b.Files = append([]WebUIFile(nil), b.Files...)
		for i := range b.Files {
			if b.Files[i].Path == c.path {
				b.Files[i] = webFile(c.path, b.Files[i].Type, c.data)
			}
		}
		if _, err := BuildWebUIPage(b, session); err == nil {
			t.Errorf("%s: a resource that closes its element was inlined", name)
		}
	}
}

func TestWebUIFrameOrigin(t *testing.T) {
	for raw, want := range map[string]string{
		"coursive-plugin://virtual-lab/":          "coursive-plugin://virtual-lab",
		"coursive-plugin://virtual-lab/index#top": "coursive-plugin://virtual-lab",
		"COURSIVE-PLUGIN://Virtual-Lab":           "coursive-plugin://virtual-lab",
		"http://127.0.0.1:4173/":                  "",
		"https://virtual-lab.example/":            "",
		"coursive-plugin://virtual-lab:80/":       "",
		"coursive-plugin://u@virtual-lab/":        "",
		"about:blank":                             "",
		"data:text/html,hi":                       "",
		"":                                        "",
	} {
		got, ok := WebUIFrameOrigin(raw)
		if got != want || ok != (want != "") {
			t.Errorf("WebUIFrameOrigin(%q) = %q, %v; want %q", raw, got, ok, want)
		}
	}
}

func TestWebUIVerifyClaimsTheRequest(t *testing.T) {
	plugin := testPagePlugin()
	session := testSession(t, plugin)
	var bridge WebUIBridge
	msg := testMessage(session, "curve.sample")
	if action, err := bridge.Verify(session, plugin, msg); err != nil || action.ID != "curve.sample" {
		t.Fatalf("Verify = %+v, %v", action, err)
	}
	if _, err := bridge.Verify(session, plugin, msg); err == nil || err.Code != WebUICodeReplayed {
		t.Fatalf("a verified request id was accepted twice: %v", err)
	}
	forged := testMessage(session, "curve.sample")
	forged.RequestID, forged.PluginID = "req-2", "other-lab"
	if _, err := bridge.Verify(session, plugin, forged); err == nil || err.Code != WebUICodeForgedIdentity {
		t.Fatalf("a forged plugin id was verified: %v", err)
	}
}
