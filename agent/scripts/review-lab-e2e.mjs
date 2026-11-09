#!/usr/bin/env node
// review-lab-e2e.mjs — the Review Lab example end to end against a real
// coursive-daemon and its assistant host: install and grant, page and slot,
// the agent tool, bridge actions with daemon-held state, the canvas tool's
// batch, a save that needs the user's approval and writes one note, disable,
// and restore.
//
//   go build -o /tmp/coursive-daemon ./cmd/coursive-daemon   (in daemon/)
//   node agent/scripts/review-lab-e2e.mjs --daemon /tmp/coursive-daemon
//
// Everything runs in temporary directories with a random token; nothing
// outside them is read or written. Exit 0 when every step passes.
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const agentDir = fileURLToPath(new URL("..", import.meta.url));
const plugin = join(agentDir, "plugins", "_examples", "review-lab");

function arg(name) {
	const i = process.argv.indexOf(name);
	return i > 0 ? process.argv[i + 1] : "";
}

async function main() {
	const bin = arg("--daemon");
	if (!bin) throw new Error("usage: review-lab-e2e.mjs --daemon <coursive-daemon binary>");
	const root = await mkdtemp(join(tmpdir(), "review-lab-e2e-"));
	const token = randomBytes(16).toString("hex");
	const daemon = spawn(resolve(bin), ["-data", join(root, "data"), "-library", join(root, "lib"), "-node", process.execPath, "-agent", join(agentDir, "src", "host.mjs")], {
		env: { ...process.env, COURSIVE_TOKEN: token },
		stdio: ["ignore", "pipe", "pipe"],
	});
	let logs = "";
	daemon.stderr.on("data", (d) => (logs = (logs + d).slice(-4000)));
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
		await run(`http://127.0.0.1:${port}`, token);
		console.log("review-lab e2e: OK");
	} catch (e) {
		console.error(`review-lab e2e: ${e.message}\n--- daemon log tail ---\n${logs}`);
		process.exitCode = 1;
	} finally {
		daemon.kill("SIGINT");
		await new Promise((r) => daemon.once("exit", r).once("error", r));
		await rm(root, { recursive: true, force: true });
	}
}

async function run(base, token) {
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
	let n = 0;
	const action = (actionId, extra = {}) => ok("POST", "/api/plugin-actions", { pluginId: "review-lab", actionId, requestId: `e2e-${++n}`, ...extra });
	const step = (name, cond, detail) => {
		if (!cond) throw new Error(`${name}: ${JSON.stringify(detail)}`);
		console.log(`✓ ${name}`);
	};

	for (let i = 0; i < 90; i++) {
		if ((await call("GET", "/agent/health").catch(() => ({}))).status === 200) break;
		await new Promise((r) => setTimeout(r, 500));
	}
	await ok("PUT", "/agent/plugins/third-party", { allow: true });
	const preview = await ok("POST", "/agent/plugins/install", { source: `path:${plugin}` });
	step("install previews the permissions it asks for", preview.manifest.permissions.map((p) => p.id).join() === "library.read,library.write", preview.manifest.permissions);
	const installed = await ok("POST", "/agent/plugins/install/confirm", { token: preview.token });
	step("confirming grants them and enables the plugin", installed.state === "enabled" && installed.needs_permissions.length === 0, installed);

	const listed = (await ok("GET", "/agent/plugins")).plugins.find((p) => p.id === "review-lab");
	step("the page and the course slot are published", listed.contributes.pages[0]?.document?.schemaVersion === 1 && listed.contributes.slots[0]?.target === "course.detail.sections", listed.contributes);
	step("the canvas tool is published", listed.contributes.slots[1]?.target === "canvas.toolbar.tools", listed.contributes);
	step("the agent sees its tool", listed.tools.some((t) => t.name === "review-lab__make_cards"), listed.tools);

	const opened = await action("open");
	step("the slot button opens the page", opened.result?.navigation?.route === "deck", opened);
	const pinned = await action("pin", { viewId: "canvas-pin", args: { revision: 0, lecture_id: "lecture-1", x: 0, y: 0 } });
	const batch = pinned.result?.viewPatch?.patch?.canvas;
	step("the canvas tool answers one canvas batch", batch?.revision === 0 && batch.objects?.length === 1 && batch.objects[0].createdBy?.pluginId === "review-lab", pinned);
	const unread = await action("pin", { viewId: "canvas-pin", args: {} });
	step("a canvas tool needs the revision it read", unread.error?.code === "invalid_args", unread);
	const started = await action("start", { viewId: "deck" });
	step("start fills the page state", started.result?.statePatch?.revision === 1 && started.result.statePatch.state.progress === "1 / 3", started);
	const revealed = await action("reveal", { viewId: "deck", contextRevision: 1 });
	step("reveal patches the same view", revealed.result?.statePatch?.state?.revealed === true, revealed);
	const stale = await action("next", { viewId: "deck", contextRevision: 1 });
	step("a stale revision is refused", stale.error?.code === "revision_conflict", stale);
	const view = await ok("GET", "/api/plugin-actions/state?pluginId=review-lab&viewId=deck");
	step("a remounted page reads the current revision", view.revision === 2, view);

	const proposed = await action("save", { viewId: "deck", contextRevision: 2 });
	const proposal = proposed.result?.proposal;
	step("save becomes a pending proposal", proposal?.status === "pending" && proposal.approvalMode === "auto", proposed);
	step("no note exists before approval", (await ok("GET", "/api/notes")).length === 0);
	const forged = await call("POST", "/agent/plugins/review-lab/bridge/save", { approved: proposal.id });
	step("the app cannot run the write through the proxy", forged.status === 404, forged);
	const applied = await ok("POST", `/api/plugin-actions/proposals/${proposal.id}/apply`);
	step("approving it writes", applied.ok && /已存为笔记/.test(applied.result?.notification?.text ?? ""), applied);
	const notes = await ok("GET", "/api/notes");
	step("one note holds the deck", notes.length === 1 && notes[0].title === "复习卡", notes);
	const again = await ok("POST", `/api/plugin-actions/proposals/${proposal.id}/apply`);
	step("an applied proposal never runs twice", again.error?.code === "proposal_settled" && (await ok("GET", "/api/notes")).length === 1, again);

	await ok("PUT", "/agent/plugins/review-lab", { enabled: false });
	const off = await action("next", { viewId: "deck", contextRevision: 2 });
	step("disabling cuts its actions off", off.error?.code === "plugin_inactive", off);
	await ok("PUT", "/agent/plugins/review-lab", { enabled: true });
	const back = await ok("GET", "/api/plugin-actions/state?pluginId=review-lab&viewId=deck");
	const resumed = await action("next", { viewId: "deck", contextRevision: back.revision });
	step("re-enabling restores the page where it was", back.revision === 2 && resumed.result?.statePatch?.state?.index === 1, { back, resumed });
}

await main();
