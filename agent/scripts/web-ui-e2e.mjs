#!/usr/bin/env node
// web-ui-e2e.mjs — a hostile plugin Web UI page in a real browser engine
// against a real coursive-daemon and its assistant host (spec 0032, WEB-01/02).
//
//   go build -o /tmp/coursive-daemon ./cmd/coursive-daemon   (in daemon/)
//   node agent/scripts/web-ui-e2e.mjs --daemon /tmp/coursive-daemon [--browser /usr/bin/chromium]
//
// The script plays the app's renderer: it opens a page session, loads the page
// the daemon built into headless Chromium over the DevTools protocol, exposes a
// `CoursiveBridge` channel like the app's WebView, relays each message to the
// daemon with the frame URL, asks the daemon about every navigation and refuses
// it with 204 (the page stays), and declines the write proposal as a user
// would. Chromium cannot host the `coursive-plugin:` scheme, so the page loads
// on https://web-probe.coursive-plugin.test/ and the relay reports that frame
// as the session's base URL; any other frame is reported as itself. Every
// request that leaves the page — anything besides the first document — fails
// the run. The probe (fixtures/web-probe) attacks from inside the page.
//
// Everything runs in temporary directories with a random token. Exit 0 when
// every step passes.
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { existsSync } from "node:fs";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const agentDir = fileURLToPath(new URL("..", import.meta.url));
const plugin = join(agentDir, "scripts", "fixtures", "web-probe");
const pageURL = "https://web-probe.coursive-plugin.test/";

function arg(name) {
	const i = process.argv.indexOf(name);
	return i > 0 ? process.argv[i + 1] : "";
}

function findBrowser() {
	const named = arg("--browser") || process.env.CHROME_PATH;
	if (named) return named;
	for (const p of ["/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome", "/usr/bin/google-chrome-stable"]) if (existsSync(p)) return p;
	throw new Error("no Chromium found; pass --browser or set CHROME_PATH");
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function main() {
	const bin = arg("--daemon");
	if (!bin) throw new Error("usage: web-ui-e2e.mjs --daemon <coursive-daemon binary> [--browser <chromium>]");
	const root = await mkdtemp(join(tmpdir(), "web-ui-e2e-"));
	const token = randomBytes(16).toString("hex");
	const daemon = spawn(resolve(bin), ["-data", join(root, "data"), "-library", join(root, "lib"), "-node", process.execPath, "-agent", join(agentDir, "src", "host.mjs")], {
		env: { ...process.env, COURSIVE_TOKEN: token, COURSIVE_WEBUI_PREVIEW: "1" },
		stdio: ["ignore", "pipe", "pipe"],
	});
	let logs = "";
	daemon.stderr.on("data", (d) => (logs = (logs + d).slice(-4000)));
	let browser;
	try {
		const port = await new Promise((ok, fail) => {
			let out = "";
			daemon.stdout.on("data", (d) => {
				out += d;
				const m = /COURSIVE_READY (\{.*\})/.exec(out);
				if (m) ok(JSON.parse(m[1]).port);
			});
			daemon.on("exit", (code) => fail(new Error(`daemon exited with ${code}`)));
			setTimeout(() => fail(new Error("daemon did not start in 30 s")), 30_000);
		});
		browser = spawn(findBrowser(), ["--headless=new", "--no-first-run", "--no-default-browser-check", "--disable-gpu", "--block-new-web-contents",
			"--remote-debugging-port=0", `--user-data-dir=${join(root, "browser")}`, "about:blank"], { stdio: "ignore" });
		await run(`http://127.0.0.1:${port}`, port, token, join(root, "browser"));
		console.log("web-ui e2e: OK");
	} catch (e) {
		console.error(`web-ui e2e: ${e.message}\n--- daemon log tail ---\n${logs}`);
		process.exitCode = 1;
	} finally {
		if (browser && browser.exitCode === null) {
			browser.kill("SIGKILL");
			await new Promise((r) => browser.once("exit", r));
		}
		daemon.kill("SIGINT");
		await new Promise((r) => daemon.once("exit", r).once("error", r));
		await rm(root, { recursive: true, force: true });
	}
}

/** One DevTools page connection: commands by id, events by method. */
async function devtools(profile) {
	let active = "";
	for (let i = 0; i < 100 && !active; i++) {
		active = await readFile(join(profile, "DevToolsActivePort"), "utf8").catch(() => "");
		if (!active) await sleep(100);
	}
	const port = active.split("\n")[0];
	if (!port) throw new Error("Chromium did not open its DevTools port");
	const target = await (await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: "PUT" })).json();
	const ws = new WebSocket(target.webSocketDebuggerUrl);
	await new Promise((ok, fail) => ((ws.onopen = ok), (ws.onerror = () => fail(new Error("DevTools connection failed")))));
	let id = 0;
	const waiting = new Map();
	const handlers = new Map();
	ws.onmessage = (e) => {
		const m = JSON.parse(e.data);
		if (m.id && waiting.has(m.id)) {
			const w = waiting.get(m.id);
			waiting.delete(m.id);
			m.error ? w.fail(new Error(`${w.method}: ${m.error.message}`)) : w.ok(m.result);
		} else if (m.method) handlers.get(m.method)?.(m.params);
	};
	const send = (method, params = {}) => new Promise((ok, fail) => {
		waiting.set(++id, { ok, fail, method });
		ws.send(JSON.stringify({ id, method, params }));
	});
	return { send, on: (method, fn) => handlers.set(method, fn), close: () => ws.close() };
}

async function run(base, daemonPort, token, profile) {
	const call = async (method, path, body) => {
		const res = await fetch(base + path, {
			method,
			headers: { authorization: `Bearer ${token}`, "content-type": "application/json" },
			body: body === undefined ? undefined : JSON.stringify(body),
		});
		const text = await res.text();
		return { status: res.status, body: text ? JSON.parse(text) : null };
	};
	const ok = async (method, path, body) => {
		const r = await call(method, path, body);
		if (r.status !== 200) throw new Error(`${method} ${path} → ${r.status} ${JSON.stringify(r.body)}`);
		return r.body;
	};
	const step = (name, cond, detail) => {
		if (!cond) throw new Error(`${name}: ${JSON.stringify(detail)}`);
		console.log(`✓ ${name}`);
	};

	for (let i = 0; i < 90; i++) {
		if ((await call("GET", "/agent/health").catch(() => ({}))).status === 200) break;
		await sleep(500);
	}
	await ok("PUT", "/agent/plugins/third-party", { allow: true });
	const preview = await ok("POST", "/agent/plugins/install", { source: `path:${plugin}` });
	const installed = await ok("POST", "/agent/plugins/install/confirm", { token: preview.token });
	step("the probe installs with its web-ui page", installed.state === "enabled" && installed.contributes.pages[0]?.renderer === "web-ui", installed);

	const linux = await ok("POST", "/api/plugin-web/sessions", { pluginId: "web-probe", pageId: "probe", platform: "linux" });
	step("a platform without an admitted renderer gets the native fallback", linux.fallback?.reason === "platform-needs-review" && !linux.html, linux);
	const opened = await ok("POST", "/api/plugin-web/sessions", { pluginId: "web-probe", pageId: "probe", platform: "android" });
	const session = opened.session;
	step("the preview opens a session on the plugin's own origin", opened.preview === true && session?.origin === "coursive-plugin://web-probe", opened);
	step("the page carries no token and no daemon address", !opened.html.includes(token) && !opened.html.includes(`127.0.0.1:${daemonPort}`));

	const dt = await devtools(profile);
	const answers = new Map(); // request id → the daemon's answers, in order
	const leaks = [];
	const navigations = [];
	const proposals = [];
	let report = null;
	let documentServed = false;
	let frameURL = "about:blank";
	let mainFrame = "";

	dt.on("Page.frameNavigated", ({ frame }) => {
		if (!frame.parentId) [frameURL, mainFrame] = [frame.url, frame.id];
	});
	dt.on("Fetch.requestPaused", async ({ requestId, request, resourceType, frameId }) => {
		if (!documentServed && request.url.startsWith(pageURL) && resourceType === "Document") {
			documentServed = true;
			await dt.send("Fetch.fulfillRequest", { requestId, responseCode: 200, responseHeaders: [{ name: "Content-Type", value: "text/html; charset=utf-8" }], body: Buffer.from(opened.html).toString("base64") });
			return;
		}
		if (resourceType === "Document" && frameId === mainFrame) {
			// What the app's NavigationDelegate does: ask the daemon, never load.
			const decision = await ok("POST", `/api/plugin-web/sessions/${session.id}/navigate`, { url: request.url });
			navigations.push({ url: request.url, ...decision });
			await dt.send("Fetch.fulfillRequest", { requestId, responseCode: 204, responseHeaders: [] });
			return;
		}
		leaks.push({ url: request.url, resourceType });
		await dt.send("Fetch.failRequest", { requestId, errorReason: "BlockedByClient" });
	});
	dt.on("Runtime.bindingCalled", async ({ name, payload }) => {
		if (name !== "__coursiveBridgeOut") return;
		// The app reports the WebView's current URL; this frame stands for the session's base URL.
		const frame = frameURL.startsWith(pageURL) ? session.baseUrl : frameURL;
		let answer = await ok("POST", `/api/plugin-web/sessions/${session.id}/messages`, { frameUrl: frame, message: payload });
		let parsed = null;
		try {
			parsed = JSON.parse(payload);
		} catch {}
		if (answer.ok && answer.result?.proposal) {
			proposals.push(answer.result.proposal);
			// The user declines; the page learns the outcome of its call.
			const rejected = await ok("POST", `/api/plugin-actions/proposals/${answer.result.proposal.id}/reject`);
			answer = { ...rejected, requestId: answer.requestId };
		}
		if (parsed?.action === "report" && answer.ok) report = parsed.args?.results;
		const key = answer.requestId || parsed?.requestId || "";
		answers.set(key, [...(answers.get(key) ?? []), answer]);
		await dt.send("Runtime.evaluate", { expression: `window.__coursiveReceive(${JSON.stringify(JSON.stringify(answer))})` });
	});

	await dt.send("Page.enable");
	await dt.send("Runtime.enable");
	await dt.send("Fetch.enable", { patterns: [{ urlPattern: "*" }] });
	await dt.send("Runtime.addBinding", { name: "__coursiveBridgeOut" });
	await dt.send("Page.addScriptToEvaluateOnNewDocument", { source: "window.CoursiveBridge = Object.freeze({ postMessage: (m) => __coursiveBridgeOut(String(m)) });" });
	await dt.send("Page.navigate", { url: `${pageURL}?port=${daemonPort}` });

	const title = async () => (await dt.send("Runtime.evaluate", { expression: "document.title", returnByValue: true })).result.value;
	for (let i = 0; i < 100 && (await title().catch(() => "")) !== "probe-done"; i++) await sleep(100);
	step("the probe ran to the end", report && (await title()) === "probe-done", { report, title: await title().catch((e) => e.message) });

	const blocked = ["fetchLoopback", "fetchLocalhost", "fetchExternal", "xhrLoopback", "webSocket", "eventSource", "imageLoopback", "imageExternal",
		"stylesheetExternal", "iframeLoopback", "workerBlob", "eval", "newFunction", "injectedScript", "injectedHandler", "popup"];
	const through = blocked.filter((k) => report[k]?.reached);
	step("no request, frame, worker, popup, eval or injected script got through", through.length === 0, Object.fromEntries(through.map((k) => [k, report[k]])));
	step("nothing left the page at the network layer, the beacon included", leaks.length === 0, { leaks, beaconQueued: report.beaconQueued });
	step("the CSP named what it refused", ["connect-src", "img-src", "script-src-elem", "frame-src", "worker-src"].every((d) => report.violations.includes(d)), report.violations);
	step("the page's own assets load as data URLs", report.dataAsset?.reached === true, report.dataAsset);
	step("no cookie and no credential global in the page", report.documentCookie === "" && report.credentialGlobals.length === 0, report);

	const codes = (id) => (answers.get(id) ?? []).map((a) => a.error?.code ?? (a.ok ? "ok" : "none"));
	const code = (id) => codes(id).join();
	// The relay answers concurrently, so the refused copy may come back first.
	step("a message in the page's own session runs once and its replay is refused", codes("forge-own").sort().join() === "ok,replayed_request", codes("forge-own"));
	const forged = {
		"forge-plugin": "forged_identity",
		"forge-instance": "forged_identity",
		"forge-origin": "cross_origin",
		"forge-loopback-origin": "cross_origin",
		"forge-nonce": "nonce_mismatch",
		"forge-schema": "schema_version",
		"forge-wildcard": "wildcard_denied",
		"forge-route": "unknown_action",
		"forge-secret": "secret_in_message",
		"forge-field": "invalid_request",
	};
	const wrong = Object.entries(forged).filter(([id, want]) => code(id) !== want).map(([id, want]) => ({ id, want, got: code(id) }));
	step("forged, cross-origin, wildcard, credential and unknown-field messages are refused", wrong.length === 0, wrong);
	step("a malformed message is refused", code("") === "invalid_request", codes(""));
	step("the API answers a declared read", report.callRead?.value === "3 张卡片", report.callRead);
	step("a write is a pending proposal, and declining it commits nothing", proposals.length === 1 && proposals[0].status === "pending" && report.callWrite?.value === "rejected", { proposals, write: report.callWrite });

	const decided = Object.fromEntries(navigations.map((n) => [n.url, n.decision]));
	step("loopback and undeclared navigations are refused, a declared host is external", decided[`http://127.0.0.1:${daemonPort}/api/health`] === "deny" && decided["https://evil.example/steal"] === "deny" && decided["https://example.org/docs"] === "external", navigations);
	step("the page survived every navigation", (await dt.send("Runtime.evaluate", { expression: "window.coursive?.pluginId", returnByValue: true })).result.value === "web-probe");

	await ok("DELETE", `/api/plugin-web/sessions/${session.id}`);
	const closed = await ok("POST", `/api/plugin-web/sessions/${session.id}/messages`, { frameUrl: session.baseUrl, message: "{}" });
	step("a closed session answers nothing", closed.error?.code === "invalid_session", closed);
	dt.close();
}

await main();
