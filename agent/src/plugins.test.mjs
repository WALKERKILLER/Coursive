import assert from "node:assert/strict";
import { mkdir, mkdtemp, readFile, rm, stat, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { DefaultResourceLoader } from "@earendil-works/pi-coding-agent";
import { applySettings, bundledPluginsDir, engineSatisfied, daemonRoute, PluginHost, validateManifest } from "./plugins.mjs";

const pkg = (coursive, extra = {}) => ({ name: `p-${coursive.id}`, version: "1.0.0", ...extra, coursive: { title: "测试", summary: "测试插件", engine: "^1.0.0", ...coursive } });

const AGENT = `
export function activate(coursive) {
	coursive.effect(() => { globalThis.__disposed = (globalThis.__disposed ?? 0) + 1; });
	coursive.action("hello", async (ctx) => ({ toast: "hi " + (ctx.name ?? ""), state: { n: 1 } }));
	coursive.action("show", async () => ({ view: { kind: "checklist", data: { items: [] } } }));
	coursive.action("bad_view", async () => ({ view: { kind: "graph", data: {} } }));
	coursive.action("slow", async (_ctx, { signal }) => {
		globalThis.__slowSignal = signal;
		await new Promise((r) => (globalThis.__slowGo = r));
		try {
			await coursive.daemon.get("/api/courses");
		} catch (e) {
			globalThis.__slowError = e;
			throw e;
		}
		return { toast: "late" };
	});
}
export default function (pi, coursive) {
	pi.registerTool({
		name: "ping",
		label: "Ping",
		description: "Answers pong. Second sentence.",
		parameters: { type: "object", properties: {} },
		execute: async () => ({ content: [{ type: "text", text: "pong" }], details: {} }),
	});
	pi.registerTool({
		name: "read",
		label: "Read",
		description: "Reads a course.",
		parameters: { type: "object", properties: {} },
		execute: async () => coursive.daemon.get("/api/courses"),
	});
	pi.registerTool({
		name: "secret",
		label: "Secret",
		description: "Reads settings.",
		parameters: { type: "object", properties: {} },
		execute: async () => coursive.daemon.get("/api/settings"),
	});
}
`;

async function fixture(t) {
	const root = await mkdtemp(join(tmpdir(), "coursive-plugins-"));
	t.after(() => rm(root, { recursive: true, force: true }));
	const bundled = join(root, "bundled");
	const write = async (dir, files) => {
		await mkdir(dir, { recursive: true });
		for (const [name, body] of Object.entries(files)) {
			await mkdir(join(dir, name, ".."), { recursive: true });
			await writeFile(join(dir, name), typeof body === "string" ? body : JSON.stringify(body));
		}
	};
	await write(join(bundled, "demo"), {
		"package.json": pkg({
			id: "demo",
			agent: "index.mjs",
			skills: ["skills"],
			views: ["checklist"],
			permissions: ["views", "library.read"],
			settings: { type: "object", properties: { port: { type: "integer", title: "端口", format: "port", default: 27121 } } },
			contributes: { actions: [{ id: "hello", title: "你好", surface: "composer.plus", action: "hello" }] },
		}),
		"index.mjs": AGENT,
		"skills/demo/SKILL.md": "---\nname: demo-skill\ndescription: A demo\n---\n# Demo\nDo the demo.",
	});
	await write(join(bundled, "broken"), {
		"package.json": pkg({ id: "broken", agent: "index.mjs" }),
		"index.mjs": "export default function () { throw new Error('boom'); }",
	});
	await write(bundled, { "catalog.json": { name: "official", plugins: [{ source: "demo", featured: true }, { source: "broken" }] } });
	const calls = [];
	const events = [];
	const host = new PluginHost({
		dataDir: join(root, "data"),
		libraryRoot: join(root, "lib"),
		bundledDir: bundled,
		daemon: async (method, path) => (calls.push(`${method} ${path}`), { content: [{ type: "text", text: "ok" }] }),
		notify: (e) => events.push(e),
	});
	await host.init();
	return { root, bundled, host, calls, events, write };
}

const install = async (host, source) => host.confirm((await host.preview(source)).token);

test("manifest validation", () => {
	assert.equal(validateManifest(pkg({ id: "ok-id" })).id, "ok-id");
	const bad = (coursive, re) => assert.throws(() => validateManifest(pkg({ id: "x1", ...coursive })), re);
	bad({ id: "Bad" }, /id/);
	bad({ id: "install" }, /保留名/);
	bad({ engine: "^2.0.0" }, /插件接口/);
	bad({ permissions: ["root"] }, /未知权限/);
	bad({ exec: ["cph"] }, /exec 权限/);
	bad({ permissions: ["network"] }, /hosts/);
	bad({ views: ["graph"] }, /views 权限/);
	bad({ agent: "../x.mjs" }, /相对路径/);
	bad({ settings: { type: "object", properties: { token: { type: "string", secret: true } } } }, /secret/);
	bad({ contributes: { actions: [{ id: "a", title: "A", action: "a", surface: "sidebar" }] } }, /surface/);
	assert.ok(engineSatisfied("^1.0.0", "1.4.2"));
	assert.ok(!engineSatisfied("^1.5.0", "1.4.2"));
	assert.ok(engineSatisfied(">=0.9.0", "1.0.0"));
});

test("settings get defaults and are checked", () => {
	const schema = { type: "object", properties: { port: { type: "integer", format: "port", default: 27121 }, mode: { type: "string", enum: ["a", "b"] } } };
	assert.deepEqual(applySettings(schema), { port: 27121 });
	assert.deepEqual(applySettings(schema, { mode: "b" }, { port: 8080 }), { port: 8080, mode: "b" });
	assert.throws(() => applySettings(schema, { port: 70000 }), /端口/);
	assert.throws(() => applySettings(schema, { mode: "c" }), /之一/);
});

test("install, tools, groups, actions and uninstall", async (t) => {
	const { host, calls, root } = await fixture(t);
	const catalog = await host.catalog();
	assert.deepEqual(catalog.plugins.map((p) => p.id), ["demo", "broken"]);
	const preview = await host.preview("bundled:demo");
	assert.deepEqual(preview.manifest.permissions.map((p) => p.id), ["views", "library.read"]);
	const view = await host.confirm(preview.token);
	assert.equal(view.state, "enabled");
	assert.deepEqual(view.tools.map((x) => x.name), ["demo__ping", "demo__read", "demo__secret"]);
	assert.deepEqual(view.settings_values, { port: 27121 });
	const state = await stat(join(root, "data", "plugins.json"));
	assert.equal(state.mode & 0o777, 0o600);

	const g = host.groups()["plugin:demo"];
	assert.deepEqual(g.tools, ["demo__ping", "demo__read", "demo__secret"]);
	assert.match(g.playbook, /## demo-skill\nDo the demo|# Demo/);
	assert.match(host.promptLines()[0], /plugin:demo/);

	assert.deepEqual(await host.runAction("demo", "hello", { name: "A" }), { toast: "hi A", state: { n: 1 } });
	assert.equal((await host.runAction("demo", "show")).view.plugin, "demo");
	await assert.rejects(host.runAction("demo", "bad_view"), /没有声明视图/);
	await assert.rejects(host.runAction("demo", "nope"), /没有动作/);

	// Real Pi loader: tools are prefixed, deferred and namespaced; permissions gate the Coursive API.
	const loader = new DefaultResourceLoader({ cwd: root, agentDir: root, noExtensions: true, noSkills: true, noPromptTemplates: true, noThemes: true, noContextFiles: true, extensionFactories: host.extensionFactories() });
	await loader.reload();
	const ext = loader.getExtensions();
	assert.deepEqual(ext.errors, []);
	const tools = ext.extensions[0].tools;
	const ping = tools.get("demo__ping").definition;
	assert.equal(ping.exposure, "deferred");
	assert.equal(ping.namespace.name, "demo");
	await tools.get("demo__read").definition.execute("1", {}, undefined, undefined, {});
	assert.deepEqual(calls, ["GET /api/courses"]);
	await assert.rejects(tools.get("demo__secret").definition.execute("1", {}, undefined, undefined, {}), /不能读取/);

	await host.uninstall("demo");
	assert.deepEqual(await host.list(), []);
	assert.deepEqual(host.groups(), {});
});

test("an action that times out is aborted and its later Coursive API calls are refused", async (t) => {
	const { host, calls } = await fixture(t);
	host.actionTimeoutMs = 50;
	await install(host, "bundled:demo");
	await assert.rejects(host.runAction("demo", "slow"), { status: 504 });
	assert.equal(globalThis.__slowSignal.aborted, true);
	globalThis.__slowGo(); // the handler carries on after the caller got 504
	await new Promise((r) => setImmediate(r));
	assert.match(globalThis.__slowError?.message ?? "", /已超时/);
	assert.deepEqual(calls, []); // its daemon read never ran
	assert.equal((await host.runAction("demo", "hello", { name: "B" })).toast, "hi B"); // other actions unaffected
});

test("close runs every plugin's cleanup", async (t) => {
	const { host } = await fixture(t);
	await install(host, "bundled:demo");
	globalThis.__disposed = 0;
	await host.close();
	assert.equal(globalThis.__disposed, 1);
	assert.equal(host.loaded.size, 0);
});

test("tool calls count against the conversation budget", async (t) => {
	const { host } = await fixture(t);
	const conv = { calls: 0 };
	host.convOf = (id) => (id === "s1" ? conv : null);
	host.guard = (c, tool) => ({ ...tool, execute: (...a) => (c.calls++, tool.execute(...a)) });
	await install(host, "bundled:demo");
	const loader = new DefaultResourceLoader({ cwd: host.root, agentDir: host.root, noExtensions: true, noSkills: true, noPromptTemplates: true, noThemes: true, noContextFiles: true, extensionFactories: host.extensionFactories() });
	await loader.reload();
	const ping = loader.getExtensions().extensions[0].tools.get("demo__ping").definition;
	await ping.execute("1", {}, undefined, undefined, { sessionManager: { getSessionId: () => "s1" } });
	assert.equal(conv.calls, 1);
});

test("scheduled task runs only get read-only plugin tools", async (t) => {
	const { host } = await fixture(t);
	const task = { calls: 0, writes: [] };
	host.convOf = (id) => (id === "task" ? task : { calls: 0 });
	const ctx = (id) => ({ sessionManager: { getSessionId: () => id } });
	const ok = async () => ({ content: [{ type: "text", text: "ok" }] });
	const writer = host.guardedExecute({ manifest: { permissions: ["library.write"] } }, "w__save", ok);
	await assert.rejects(writer("1", {}, undefined, undefined, ctx("task")), /定时任务里不能使用/);
	assert.equal((await writer("1", {}, undefined, undefined, ctx("chat"))).content[0].text, "ok");
	const reader = host.guardedExecute({ manifest: { permissions: ["library.read", "views"] } }, "r__read", ok);
	assert.equal((await reader("1", {}, undefined, undefined, ctx("task"))).content[0].text, "ok");

	// A read-only plugin's tool cannot write its own storage in a run either.
	await install(host, "bundled:demo");
	const { coursive } = host.loaded.get("demo");
	const saver = host.guardedExecute(host.loaded.get("demo"), "demo__save", async () => {
		await new Promise((r) => setImmediate(r));
		await coursive.storage.set("k", 1);
		return ok();
	});
	await assert.rejects(saver("1", {}, undefined, undefined, ctx("task")), /不能写入自己的存储/);
	assert.equal(await coursive.storage.get("k"), undefined);
	await saver("1", {}, undefined, undefined, ctx("chat"));
	assert.equal(await coursive.storage.get("k"), 1);
});

test("plugin daemon paths must be normal and on the route list", () => {
	assert.ok(daemonRoute("GET", "/api/courses"));
	assert.ok(daemonRoute("GET", "/api/courses/c%201/lectures"));
	assert.ok(daemonRoute("GET", "/api/search?q=%E6%A0%88&limit=5"));
	assert.ok(daemonRoute("PUT", "/api/notes"));
	assert.ok(daemonRoute("DELETE", "/api/schedule/s1"));
	assert.ok(daemonRoute("POST", "/api/lectures/l1/bookmarks"));
	for (const path of [
		"/api/notes/../library/export",
		"/api/notes/%2e%2e/library/export",
		"/api/notes/%2E%2E/settings",
		"/api/notes/./x",
		"/api/notes%2fx",
		"/api/notes\\..\\settings",
		"/api//notes",
		"//daemon/api/notes",
		"/api/notes/",
		"/api/settings",
		"/api/lectures/l1/media",
		"/api/lectures/l1/slides/0/image",
		"/api/notes/n1#x",
		"http://daemon/api/notes",
		"/api/notes/\u6808",
	]) {
		assert.equal(daemonRoute("GET", path), false, path);
		assert.equal(daemonRoute("DELETE", path), false, path);
	}
	assert.equal(daemonRoute("DELETE", "/api/courses"), false); // a read route is not writable
	assert.equal(daemonRoute("PATCH", "/api/notes"), false); // nor with another method
	assert.equal(daemonRoute("GET", "/api/notes/a/b"), false); // * is one segment
});

test("user catalogs are capped in bytes and entries", async (t) => {
	const { host } = await fixture(t);
	await host.addCatalog("https://example.com/big.json");
	let pulls = 0;
	host.fetch = async () =>
		new Response(
			new ReadableStream({
				pull(c) {
					pulls++;
					c.enqueue(new Uint8Array(64 * 1024));
				},
			}),
		);
	const big = await host.catalog();
	assert.match(big.errors[0].error, /目录过大/);
	assert.ok(pulls < 40); // stopped reading after the cap
	const plugins = Array.from({ length: 600 }, (_, i) => ({ id: `p-${i}`, source: `npm:p-${i}` }));
	host.fetch = async () => new Response(JSON.stringify({ name: "many", plugins }));
	const many = await host.catalog();
	assert.deepEqual(many.errors, []);
	assert.equal(many.plugins.filter((p) => p.catalog === "many").length, 500);
});

test("a failing plugin is isolated and effects run on disable", async (t) => {
	const { host } = await fixture(t);
	globalThis.__disposed = 0;
	await install(host, "bundled:demo");
	const broken = await install(host, "bundled:broken");
	assert.equal(broken.state, "error");
	assert.match(broken.error, /boom/);
	assert.equal((await host.view("demo")).state, "enabled");
	assert.deepEqual(host.extensionFactories().map((f) => f.name), ["demo"]);
	globalThis.__disposed = 0; // every reload disposes the previous generation
	const off = await host.update("demo", { enabled: false });
	assert.equal(off.state, "disabled");
	assert.equal(globalThis.__disposed, 1);
	await assert.rejects(host.runAction("demo", "hello"), /未启用/);
});

test("third-party sources need the opt-in, and new permissions need consent", async (t) => {
	const { host, root, write } = await fixture(t);
	const src = join(root, "third");
	await write(src, { "package.json": pkg({ id: "third", permissions: [] }) });
	await assert.rejects(host.preview(`path:${src}`), /第三方插件未开启/);
	await host.setAllowThirdParty(true);
	assert.equal((await install(host, `path:${src}`)).state, "enabled");

	await write(src, { "package.json": pkg({ id: "third", permissions: ["views"], views: ["checklist"] }, { version: "1.1.0" }) });
	// The installed copy is replaced only through preview/confirm; simulate an
	// updated package on disk to check the consent gate.
	await write(join(root, "data", "plugins", "pkgs", "third"), { "package.json": pkg({ id: "third", permissions: ["views"], views: ["checklist"] }, { version: "1.1.0" }) });
	await host.reload();
	const v = await host.view("third");
	assert.equal(v.state, "needs_consent");
	assert.deepEqual(v.needs_permissions.map((p) => p.id), ["views"]);
	assert.equal((await host.update("third", { grant: true })).state, "enabled");

	const preview = await host.preview(`path:${src}`);
	assert.deepEqual(preview.update, { from: "1.1.0", added_permissions: [] });
	await write(join(root, "official"), { "package.json": pkg({ id: "demo" }) });
	await assert.rejects(host.preview(`path:${join(root, "official")}`), /官方插件/);
});

test("a v3 package installs and publishes its page, slot and action", async (t) => {
	const { host } = await fixture(t);
	await host.setAllowThirdParty(true);
	const v = await install(host, `path:${join(bundledPluginsDir, "_template")}`);
	assert.equal(v.state, "enabled");
	assert.equal(v.schema, 3);
	assert.deepEqual(v.requires, [{ service: "transcript.read", version: "^1", optional: true }]);
	const [page] = v.contributes.pages;
	assert.equal(page.renderer, "declarative");
	assert.equal(page.document.body.type, "column");
	assert.deepEqual(v.contributes.slots.map((s) => [s.id, s.target, s.mode]), [["course-summary", "course.detail.sections", "list"]]);
	assert.deepEqual(v.contributes.navigation.map((n) => n.page), ["home"]);
	// A v3 action has no v1 surface, so the v1 readers in the app skip it.
	assert.deepEqual(v.contributes.actions.map((a) => [a.id, a.surface ?? null]), [["hello", null]]);
	assert.deepEqual(await host.bridgeAction("template", "hello", { args: { from: "page" } }), { notification: { text: "Hello from the template (page)" } });
});

test("the action bridge runs v3 read actions with their view state and nothing else", async (t) => {
	const { host, root, write } = await fixture(t);
	await host.setAllowThirdParty(true);
	const src = join(root, "cards");
	const contributes = {
		actions: [
			{ id: "next", title: "下一张", action: "next", params: { properties: { step: "integer" } } },
			{ id: "save", title: "保存", action: "save", permission: "library.write", write: true },
			{ id: "bad", title: "坏结果", action: "bad" },
			{ id: "slow", title: "慢", action: "slow" },
		],
	};
	await write(src, {
		"package.json": { name: "p-cards", version: "1.0.0", coursive: { id: "cards", title: "卡片", summary: "测试插件", api: "^3.0.0", runtime: [{ kind: "pi", entry: "index.mjs" }], permissions: ["library.read", "library.write"], contributes } },
		"index.mjs": `export function activate(coursive) {
	coursive.action("next", async (args, { state, viewId }) => ({ statePatch: { viewId, patch: { index: (state.index ?? 0) + (args.step ?? 1) } } }));
	coursive.action("save", async (_args, { approved }) => { globalThis.__cardsSaved = approved; return { notification: { text: "saved" } }; });
	coursive.action("bad", async () => ({ toast: "v1 shape" }));
	coursive.action("slow", (_args, { signal }) => new Promise((_r, rej) => signal.addEventListener("abort", () => rej(signal.reason))));
}`,
	});
	await install(host, `path:${src}`);
	assert.deepEqual(await host.bridgeAction("cards", "next", { args: { step: 2 }, state: { index: 3 }, viewId: "deck" }), { statePatch: { viewId: "deck", patch: { index: 5 } } });
	// A Go nil map arrives as null.
	assert.deepEqual(await host.bridgeAction("cards", "next", { args: null, state: null, viewId: "deck" }), { statePatch: { viewId: "deck", patch: { index: 1 } } });
	await assert.rejects(host.bridgeAction("cards", "save"), { status: 409 });
	assert.equal(globalThis.__cardsSaved, undefined);
	// The daemon passes the proposal the user approved; only then does a write run.
	assert.deepEqual(await host.bridgeAction("cards", "save", { approved: "proposal-1" }), { notification: { text: "saved" } });
	assert.equal(globalThis.__cardsSaved, "proposal-1");
	await assert.rejects(host.bridgeAction("cards", "next", { approved: "proposal-1" }), { status: 409 });
	await assert.rejects(host.bridgeAction("cards", "bad"), { status: 502 });
	await assert.rejects(host.bridgeAction("cards", "slow", { deadlineMs: 30 }), { status: 504 });
	await assert.rejects(host.bridgeAction("cards", "missing"), { status: 404 });
	// A v1 plugin's actions keep their own route.
	await install(host, "bundled:demo");
	await assert.rejects(host.bridgeAction("demo", "hello"), { status: 404 });
	await host.update("cards", { enabled: false });
	await assert.rejects(host.bridgeAction("cards", "next"), /未启用/);
});

test("web-ui resources are indexed at install and re-verified before they are served", async (t) => {
	const { host, root, write } = await fixture(t);
	await host.setAllowThirdParty(true);
	const src = join(root, "board");
	const manifest = (extra = {}) => ({
		name: "p-board",
		version: "1.0.0",
		coursive: {
			id: "board",
			title: "面板",
			summary: "测试插件",
			api: "^3.0.0",
			runtime: [{ kind: "pi", entry: "index.mjs" }, { kind: "web-ui", entry: "web", optional: true }],
			hosts: ["example.org"],
			contributes: { pages: [{ id: "board", title: "面板", renderer: "web-ui", entry: "web/app.js", styles: ["web/app.css"] }] },
			...extra,
		},
	});
	await write(src, { "package.json": manifest(), "index.mjs": "export function activate() {}", "web/app.js": "document.title = 'board';", "web/app.css": "body { margin: 0 }", "web/img/dot.svg": "<svg/>" });
	await install(host, `path:${src}`);
	const page = await host.webPage("board", "board");
	assert.equal(page.entry, "web/app.js");
	assert.deepEqual(page.styles, ["web/app.css"]);
	assert.deepEqual(page.hosts, ["example.org"]);
	assert.deepEqual(page.files.map((f) => [f.path, f.type]), [["web/app.css", "text/css"], ["web/app.js", "text/javascript"], ["web/img/dot.svg", "image/svg+xml"]]);
	assert.equal(Buffer.from(page.files[1].data, "base64").toString(), "document.title = 'board';");
	await assert.rejects(host.webPage("board", "missing"), { status: 404 });

	// The installed package changes on disk: a changed, an added and a removed file are all refused.
	const pkgs = join(root, "data", "plugins", "pkgs", "board");
	await writeFile(join(pkgs, "web", "app.js"), "fetch('http://127.0.0.1')");
	await assert.rejects(host.webPage("board", "board"), { status: 409, message: /web\/app\.js 与安装时不一致/ });
	await writeFile(join(pkgs, "web", "app.js"), "document.title = 'board';");
	await writeFile(join(pkgs, "web", "extra.js"), "1");
	await assert.rejects(host.webPage("board", "board"), /web\/extra\.js 与安装时不一致/);
	await rm(join(pkgs, "web", "extra.js"));
	await rm(join(pkgs, "web", "img", "dot.svg"));
	await assert.rejects(host.webPage("board", "board"), /dot\.svg 与安装时不一致/);
	// Re-consent indexes the package again.
	await host.update("board", { grant: true });
	assert.equal((await host.webPage("board", "board")).files.length, 2);

	// Install refuses a symlink, an unknown type, an oversized file and a missing directory.
	const refuse = async (files, re) => {
		const dir = await mkdtemp(join(root, "bad-"));
		await write(dir, { "package.json": manifest(), "index.mjs": "export function activate() {}", "web/app.js": "1", ...files });
		await assert.rejects(host.preview(`path:${dir}`), re);
	};
	const dir = await mkdtemp(join(root, "link-"));
	await write(dir, { "package.json": manifest(), "index.mjs": "export function activate() {}", "web/app.js": "1" });
	await symlink(join(root, "data", "plugins.json"), join(dir, "web", "leak.json"));
	await assert.rejects(host.preview(`path:${dir}`), /符号链接/);
	await refuse({ "web/page.html": "<script></script>" }, /类型不受支持/);
	await refuse({ "web/big.js": "x".repeat(256 * 1024 + 1) }, /超过 256 KB/);
	const none = await mkdtemp(join(root, "none-"));
	await write(none, { "package.json": manifest(), "index.mjs": "export function activate() {}" });
	await assert.rejects(host.preview(`path:${none}`), /找不到网页资源目录 web/);
});

test("a v3 manifest is checked by the shared rules and then by this host", () => {
	const v3 = (coursive, contributes) => ({
		name: "p-v3",
		version: "1.0.0",
		coursive: { id: "v3", title: "测试", summary: "测试插件", api: "^3.0.0", runtime: [{ kind: "pi", entry: "index.mjs" }], ...coursive, ...(contributes ? { contributes } : {}) },
	});
	const doc = (body = { type: "text", text: "x" }) => ({ schemaVersion: 1, body });
	assert.equal(validateManifest(v3({})).agent, "index.mjs");
	const bad = (pkg, re) => assert.throws(() => validateManifest(pkg), re);
	bad(v3({ engine: "^1.0.0" }), /coursive\.engine.*coursive\.api/);
	bad(v3({ api: "^4.0.0" }), /插件接口/);
	bad(v3({ runtime: [{ kind: "moejs", entry: "x.js" }] }), /moejs/);
	const webRuntime = { runtime: [{ kind: "pi", entry: "index.mjs" }, { kind: "web-ui", entry: "./ui", optional: true }] };
	const webPage = (extra) => ({ pages: [{ id: "p", title: "P", renderer: "web-ui", entry: "ui/app.js", ...extra }] });
	assert.equal(validateManifest(v3(webRuntime)).agent, "index.mjs");
	assert.equal(validateManifest(v3(webRuntime)).web, "ui");
	assert.deepEqual(validateManifest(v3(webRuntime, webPage({ styles: ["./ui/app.css"] }))).contributes.pages[0], {
		id: "p", title: "P", icon: "extension", renderer: "web-ui", order: 100, entry: "ui/app.js", styles: ["ui/app.css"],
	});
	assert.ok(validateManifest(v3(webRuntime, webPage({ document: doc() }))).contributes.pages[0].document);
	bad(v3(webRuntime, webPage({ entry: "app.js" })), /ui\/ 目录内的 \.js/);
	bad(v3(webRuntime, webPage({ entry: "ui/../../index.mjs" })), /相对路径/);
	bad(v3(webRuntime, webPage({ entry: "ui/../index.js" })), /ui\/ 目录内/);
	bad(v3(webRuntime, webPage({ entry: "ui/app.html" })), /\.js 文件/);
	bad(v3(webRuntime, webPage({ styles: ["ui/app.js"] })), /\.css 文件/);
	bad(v3({ runtime: [{ kind: "pi", entry: "index.mjs" }, { kind: "web-ui", entry: "a", optional: true }, { kind: "web-ui", entry: "b", optional: true }] }), /一个 web-ui/);
	bad(v3({}, { pages: [{ id: "p", title: "P", entry: "ui/app.js", document: doc() }] }), /只属于 web-ui/);
	bad(v3({ permissions: ["source"] }), /来源插件/);
	bad(v3({}, { pages: [{ id: "p", title: "P" }] }), /document/);
	bad(v3({}, { pages: [{ id: "p", title: "P", renderer: "web-ui", document: doc() }] }), /web-ui/);
	bad(v3({}, { pages: [{ id: "p", title: "P", route: "/x", document: doc() }] }), /不支持字段 route/);
	bad(v3({}, { pages: [{ id: "p", title: "P", document: doc({ type: "button", text: "go", action: "missing" }) }] }), /未声明的动作 missing/);
	bad(v3({}, { slots: [{ id: "s", target: "player.controls.trailing", document: doc() }] }), /target/);
	bad(v3({}, { slots: [{ id: "s", target: "course.detail.sections", mode: "single", document: doc() }] }), /模式是 list/);
	bad(v3({}, { navigation: [{ id: "n", title: "N", page: "nowhere" }] }), /不存在的页面/);
	bad(v3({}, { actions: [{ id: "a", title: "A", permission: "library.write" }] }), /没有在 permissions 中声明/);
	bad(v3({ permissions: ["library.write"] }, { actions: [{ id: "a", title: "A", write: true }] }), /library\.write/);
	bad(v3({}, { actions: [{ id: "a", title: "A", params: { properties: { n: "date" } } }] }), /类型/);
	bad(v3({}, { sources: [{ id: "s", title: "S", protocol: "1", executable: { "linux-x64": "bin/s" } }] }), /来源/);
});

test("the bundled catalog's plugins are valid", async () => {
	const c = JSON.parse(await readFile(join(bundledPluginsDir, "catalog.json"), "utf8"));
	for (const e of c.plugins) {
		const m = validateManifest(JSON.parse(await readFile(join(bundledPluginsDir, e.source, "package.json"), "utf8")));
		assert.equal(m.id, e.source);
	}
});

test("socratic toggles a mode that extends the system prompt", async (t) => {
	const root = await mkdtemp(join(tmpdir(), "coursive-socratic-"));
	t.after(() => rm(root, { recursive: true, force: true }));
	const host = new PluginHost({ dataDir: root, libraryRoot: root, bundledDir: bundledPluginsDir, daemon: async () => ({}) });
	await host.init();
	const v = await install(host, "bundled:socratic");
	assert.deepEqual(v.status, { active: false });
	assert.equal(host.groups()["plugin:socratic"].tools.length, 0);
	assert.match(host.groups()["plugin:socratic"].playbook, /提示梯度/);
	const loader = new DefaultResourceLoader({ cwd: root, agentDir: root, noExtensions: true, noSkills: true, noPromptTemplates: true, noThemes: true, noContextFiles: true, extensionFactories: host.extensionFactories() });
	await loader.reload();
	const [hook] = loader.getExtensions().extensions[0].handlers.get("before_agent_start");
	assert.equal(await hook({ systemPrompt: "BASE" }), undefined);
	assert.deepEqual((await host.runAction("socratic", "toggle")).state, { active: true });
	assert.deepEqual((await host.view("socratic")).status, { active: true });
	const r = await hook({ systemPrompt: "BASE" });
	assert.match(r.systemPrompt, /^BASE\n\n## 引导式提问模式/);
	assert.match(r.systemPrompt, /2 次/);
});
