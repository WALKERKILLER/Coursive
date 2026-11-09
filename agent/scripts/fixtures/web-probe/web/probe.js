// A hostile page: every attempt below must fail inside the renderer. The page
// learns the daemon port from its URL (?port=), the best case for an attacker;
// it never learns the token. Results go back through the bridge's report action.
(async () => {
	const port = new URLSearchParams(location.search).get("port") ?? "0";
	const daemon = `http://127.0.0.1:${port}`;
	const results = {};
	const violations = [];
	document.addEventListener("securitypolicyviolation", (e) => violations.push(e.effectiveDirective));
	const settle = (p, ms = 1500) => Promise.race([p, new Promise((r) => setTimeout(() => r("timeout"), ms))]);
	const attempt = async (name, run) => {
		try {
			results[name] = { reached: true, value: String(await settle(Promise.resolve().then(run))).slice(0, 120) };
		} catch (e) {
			results[name] = { reached: false, error: String(e?.name ?? e) };
		}
	};
	const loads = (el) => new Promise((ok, fail) => ((el.onload = () => ok("loaded")), (el.onerror = () => fail(new Error("blocked")))));

	await attempt("fetchLoopback", () => fetch(`${daemon}/api/health`).then((r) => r.status));
	await attempt("fetchLocalhost", () => fetch(`http://localhost:${port}/api/health`).then((r) => r.status));
	await attempt("fetchExternal", () => fetch("https://example.org/").then((r) => r.status));
	await attempt("xhrLoopback", () => new Promise((ok, fail) => {
		const x = new XMLHttpRequest();
		x.onload = () => ok(x.status);
		x.onerror = () => fail(new Error("blocked"));
		x.open("GET", `${daemon}/api/health`);
		x.send();
	}));
	await attempt("webSocket", () => new Promise((ok, fail) => {
		const ws = new WebSocket(`ws://127.0.0.1:${port}/api/events`);
		ws.onopen = () => ok("open");
		ws.onerror = () => fail(new Error("blocked"));
	}));
	await attempt("eventSource", () => new Promise((ok, fail) => {
		const es = new EventSource(`${daemon}/api/events`);
		es.onopen = () => ok("open");
		es.onerror = () => (es.close(), fail(new Error("blocked")));
	}));
	// sendBeacon only queues; whether it left shows in the CSP report and the relay's network log.
	results.beaconQueued = navigator.sendBeacon(`${daemon}/api/health`, "x");
	await attempt("imageLoopback", () => { const i = new Image(); const p = loads(i); i.src = `${daemon}/files/x.png`; return p; });
	await attempt("imageExternal", () => { const i = new Image(); const p = loads(i); i.src = "https://example.org/x.png"; return p; });
	await attempt("stylesheetExternal", () => { const l = document.createElement("link"); l.rel = "stylesheet"; const p = loads(l); l.href = "https://example.org/x.css"; document.head.append(l); return p; });
	await attempt("iframeLoopback", () => { const f = document.createElement("iframe"); const p = loads(f); f.src = `${daemon}/api/health`; document.body.append(f); return settle(p, 800).then(() => f.contentDocument?.body?.textContent || Promise.reject(new Error("blocked"))); });
	await attempt("workerBlob", () => { const w = new Worker(URL.createObjectURL(new Blob(["postMessage(1)"], { type: "text/javascript" }))); return new Promise((ok, fail) => ((w.onmessage = () => ok("ran")), (w.onerror = () => fail(new Error("blocked"))))); });
	await attempt("eval", () => eval("6 * 7"));
	await attempt("newFunction", () => new Function("return 42")());
	await attempt("injectedScript", () => { const s = document.createElement("script"); s.textContent = "window.__injected = 1"; document.body.append(s); return window.__injected === 1 ? "ran" : Promise.reject(new Error("blocked")); });
	await attempt("injectedHandler", () => { document.body.insertAdjacentHTML("beforeend", "<img src=x onerror='window.__handler = 1'>"); return settle(new Promise((r) => setTimeout(r, 300))).then(() => (window.__handler === 1 ? "ran" : Promise.reject(new Error("blocked")))); });
	await attempt("popup", () => (window.open(`${daemon}/api/health`) ? "opened" : Promise.reject(new Error("blocked"))));
	await attempt("dataAsset", () => { const i = new Image(); const p = loads(i); i.src = coursive.asset("web/dot.svg"); return p; });
	results.documentCookie = document.cookie;
	results.credentialGlobals = Object.getOwnPropertyNames(window).filter((k) => /token|secret|daemon|authorization/i.test(k) && typeof window[k] !== "function");

	// Forged messages straight into the channel. The page can read its own
	// bootstrap, so it knows its session; it still cannot be another page.
	const boot = document.scripts[0].textContent;
	const session = JSON.parse(/const session = (\{.*?\});/.exec(boot)[1]);
	const base = { schemaVersion: 1, origin: session.origin, pluginId: session.pluginId, instanceId: session.instanceId, nonce: session.nonce, action: "cards", args: {}, deadlineMs: 0 };
	const post = (requestId, edit = {}) => CoursiveBridge.postMessage(JSON.stringify({ ...base, requestId, ...edit }));
	post("forge-own");
	post("forge-own");
	post("forge-plugin", { pluginId: "review-lab" });
	post("forge-instance", { instanceId: "web-0000" });
	post("forge-origin", { origin: "coursive-plugin://review-lab" });
	post("forge-loopback-origin", { origin: daemon });
	post("forge-nonce", { nonce: "guessed" });
	post("forge-schema", { schemaVersion: 2 });
	post("forge-wildcard", { action: "*" });
	post("forge-route", { action: "api.settings" });
	post("forge-secret", { args: { authorization: "Bearer stolen" } });
	post("forge-field", { url: "/api/settings" });
	CoursiveBridge.postMessage("not json");

	// The real API: a read answers, a write waits for the user.
	await attempt("callRead", () => coursive.call("cards").then((r) => r.notification?.text));
	await attempt("callWrite", () => coursive.call("save", { title: "复习卡" }).then((r) => r.proposal?.status ?? JSON.stringify(r)));
	results.violations = [...new Set(violations)].sort();
	await coursive.call("report", { results });

	// Navigation last: the renderer must keep this page whatever is asked.
	document.title = "probe-navigating";
	location.href = `${daemon}/api/health`;
	setTimeout(() => (location.href = "https://evil.example/steal"), 200);
	setTimeout(() => (location.href = "https://example.org/docs"), 400);
	setTimeout(() => (document.title = "probe-done"), 900);
})();
