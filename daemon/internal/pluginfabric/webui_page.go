package pluginfabric

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"path"
	"regexp"
	"strings"
)

// The resources one Web UI page may ship, by extension, with the size the page
// inlines; the assistant host applies the same table when it indexes a package
// (agent/src/plugins.mjs, WEB_TYPES).
var webUITypes = map[string]string{
	".js":    "text/javascript",
	".css":   "text/css",
	".json":  "application/json",
	".txt":   "text/plain",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".webp":  "image/webp",
	".woff2": "font/woff2",
}

const (
	maxWebUIFiles = 64
	maxWebUIBytes = 512 << 10
)

// WebUIBundle is one web-ui page's resources as the assistant host serves them
// after checking them against the index it recorded at install.
type WebUIBundle struct {
	PluginID string      `json:"pluginId"`
	PageID   string      `json:"pageId"`
	Title    string      `json:"title"`
	Entry    string      `json:"entry"`
	Styles   []string    `json:"styles"`
	Hosts    []string    `json:"hosts"`
	Files    []WebUIFile `json:"files"`
}

// WebUIFile is one package file; Data travels as base64.
type WebUIFile struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	SHA256 string `json:"sha256"`
	Data   []byte `json:"data"`
}

var webUIPathRE = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)

// Check verifies a bundle before a page is built from it: it is the page the
// daemon asked for, every path is a clean package path of a known type, every
// file matches its hash, the entry is a script and the styles are style sheets,
// and the whole fits the inline budget.
func (b WebUIBundle) Check(pluginID, pageID string) error {
	if b.PluginID != pluginID || b.PageID != pageID {
		return fmt.Errorf("网页资源属于 %s/%s，不是 %s/%s", b.PluginID, b.PageID, pluginID, pageID)
	}
	if len(b.Files) > maxWebUIFiles {
		return fmt.Errorf("网页资源超过 %d 个文件", maxWebUIFiles)
	}
	seen := map[string]string{}
	total := 0
	for _, f := range b.Files {
		if !webUIPathRE.MatchString(f.Path) || path.Clean(f.Path) != f.Path || strings.Contains("/"+f.Path+"/", "/../") {
			return fmt.Errorf("网页资源路径无效：%q", f.Path)
		}
		if _, dup := seen[f.Path]; dup {
			return fmt.Errorf("网页资源 %s 重复", f.Path)
		}
		if want := webUITypes[strings.ToLower(path.Ext(f.Path))]; want == "" || f.Type != want {
			return fmt.Errorf("网页资源 %s 的类型 %q 不受支持", f.Path, f.Type)
		}
		sum := sha256.Sum256(f.Data)
		if hex.EncodeToString(sum[:]) != f.SHA256 {
			return fmt.Errorf("网页资源 %s 与安装记录的哈希不一致", f.Path)
		}
		total += len(f.Data)
		seen[f.Path] = f.Type
	}
	if total > maxWebUIBytes {
		return fmt.Errorf("网页资源合计超过 %d KB", maxWebUIBytes>>10)
	}
	if seen[b.Entry] != "text/javascript" {
		return fmt.Errorf("找不到页面脚本 %s", b.Entry)
	}
	for _, s := range b.Styles {
		if seen[s] != "text/css" {
			return fmt.Errorf("找不到页面样式 %s", s)
		}
	}
	return nil
}

func (b WebUIBundle) file(p string) []byte {
	for _, f := range b.Files {
		if f.Path == p {
			return f.Data
		}
	}
	return nil
}

// A script or style sheet that could close its own element would let package
// text become markup; the author escapes it (`<\/script>`) instead.
var (
	scriptCloseRE = regexp.MustCompile(`(?i)</script|<!--`)
	styleCloseRE  = regexp.MustCompile(`(?i)</style`)
)

// BuildWebUIPage builds the document a renderer loads for one session. The host
// owns it whole: the CSP meta element with a fresh nonce, the plugin's styles
// and entry script inlined under that nonce, and a bootstrap that exposes
// `window.coursive` — `call(action, args, {deadlineMs})` posts one structured
// message through the renderer's `CoursiveBridge` channel and resolves with
// the daemon's answer, and `asset(path)` returns a package image, font or text
// file as a data URL. The page holds no daemon token and no daemon URL; the
// bridge nonce in it binds messages to this session, not to a secret.
func BuildWebUIPage(b WebUIBundle, session WebUISession) (string, error) {
	nonce := randomHex(16)
	var sb strings.Builder
	sb.WriteString("<!doctype html>\n<html lang=\"zh\"><head><meta charset=\"utf-8\">\n")
	fmt.Fprintf(&sb, "<meta http-equiv=\"Content-Security-Policy\" content=\"%s\">\n", html.EscapeString(WebUICSP(nonce)))
	sb.WriteString("<meta name=\"referrer\" content=\"no-referrer\">\n")
	sb.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&sb, "<title>%s</title>\n", html.EscapeString(b.Title))
	for _, s := range b.Styles {
		css := b.file(s)
		if styleCloseRE.Match(css) {
			return "", fmt.Errorf("页面样式 %s 不能包含 </style", s)
		}
		fmt.Fprintf(&sb, "<style nonce=\"%s\">\n%s\n</style>\n", nonce, css)
	}
	boot, err := webUIBootstrap(b, session)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&sb, "<script nonce=\"%s\">%s</script>\n</head><body>\n", nonce, boot)
	entry := b.file(b.Entry)
	if scriptCloseRE.Match(entry) {
		return "", fmt.Errorf("页面脚本 %s 不能包含 </script 或 <!--；请写成 <\\/script", b.Entry)
	}
	fmt.Fprintf(&sb, "<script nonce=\"%s\">\n%s\n</script>\n</body></html>\n", nonce, entry)
	return sb.String(), nil
}

// webUIBootstrap is the host script that runs before the plugin's: the session
// identity it stamps on every message, the pending calls by request id, and the
// package assets. It reads no storage and opens no connection.
func webUIBootstrap(b WebUIBundle, session WebUISession) (string, error) {
	assets := map[string]string{}
	for _, f := range b.Files {
		if f.Type == "text/javascript" || f.Type == "text/css" {
			continue
		}
		assets[f.Path] = "data:" + f.Type + ";base64," + base64.StdEncoding.EncodeToString(f.Data)
	}
	// encoding/json escapes <, > and &, so neither value can close the element.
	sess, err := json.Marshal(map[string]any{"schemaVersion": WebUIBridgeSchemaVersion, "origin": session.Origin,
		"pluginId": session.PluginID, "instanceId": session.InstanceID, "nonce": session.Nonce, "pageId": b.PageID})
	if err != nil {
		return "", err
	}
	files, err := json.Marshal(assets)
	if err != nil {
		return "", err
	}
	return strings.NewReplacer("__SESSION__", string(sess), "__ASSETS__", string(files)).Replace(webUIBootstrapJS), nil
}

const webUIBootstrapJS = `
(() => {
	"use strict";
	const session = __SESSION__;
	const assets = __ASSETS__;
	const channel = window.CoursiveBridge;
	const pending = new Map();
	let seq = 0;
	const failure = (code, message, details = {}) => Object.assign(new Error(message), { code, details });
	const receive = (raw) => {
		let answer;
		try {
			answer = typeof raw === "string" ? JSON.parse(raw) : raw;
		} catch {
			return;
		}
		const waiting = answer && pending.get(answer.requestId);
		if (!waiting) return;
		pending.delete(answer.requestId);
		if (answer.ok) waiting.resolve(answer.result ?? {});
		else waiting.reject(failure(answer.error?.code ?? "unavailable", answer.error?.message ?? "", answer.error?.details ?? {}));
	};
	const call = (action, args = {}, { deadlineMs = 0 } = {}) =>
		new Promise((resolve, reject) => {
			if (!channel || typeof channel.postMessage !== "function") return reject(failure("unavailable", "Web UI bridge is not available"));
			const requestId = session.instanceId + "." + ++seq;
			pending.set(requestId, { resolve, reject });
			channel.postMessage(JSON.stringify({ schemaVersion: session.schemaVersion, origin: session.origin, pluginId: session.pluginId, instanceId: session.instanceId, nonce: session.nonce, requestId, action, args, deadlineMs }));
		});
	Object.defineProperty(window, "__coursiveReceive", { value: receive });
	Object.defineProperty(window, "coursive", { value: Object.freeze({ pluginId: session.pluginId, pageId: session.pageId, call, asset: (p) => assets[p] ?? "" }) });
})();
`
