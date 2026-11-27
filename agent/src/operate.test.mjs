import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { Bot } from "./bot.mjs";
import { checkSetting, MAX_DOWNLOAD, operateTools, runOp, SETTINGS } from "./operate.mjs";

async function setup(over = {}) {
	const dir = await mkdtemp(join(tmpdir(), "coursive-operate-"));
	const bot = new Bot(dir);
	await bot.load();
	const calls = [];
	const daemon = {
		"GET /api/settings": { "download.video_parallel": 2, "asr.concurrency": 2 },
		"GET /api/jobs": [
			{ id: "j1", state: "running", title: "第 1 节", kind: "video" },
			{ id: "j2", state: "paused", title: "第 2 节", kind: "video" },
			{ id: "j3", state: "queued", title: "第 3 节", kind: "slides" },
		],
		"GET /api/courses": [{ id: "c1", title: "高等数学", alias: "", pinned: false, hidden: false, color: "" }],
		"GET /api/lectures/l1": { lecture: { id: "l1", title: "第 1 节", date: "2026-09-01", has_playback: true, assets: [] } },
		"GET /api/courses/c1/lectures": {
			course: { id: "c1", title: "高等数学" },
			lectures: [
				{ id: "l4", title: "第 4 节", date: "2026-09-22", has_playback: true, assets: [], segments: 0 },
				{ id: "l2", title: "第 2 节", date: "2026-09-08", has_playback: true, assets: ["video"], segments: 0 },
				{ id: "l1", title: "第 1 节", date: "2026-09-01", has_playback: true, assets: ["video"], segments: 120 },
				{ id: "l3", title: "第 3 节", date: "2026-09-15", has_playback: true, assets: [], segments: 0 },
				{ id: "l5", title: "第 5 节", date: "2026-09-29", has_playback: false, assets: [], segments: 0 },
			],
		},
		...over.daemon,
	};
	const api = async (method, path, body) => {
		calls.push({ method, path, body });
		const k = `${method} ${path}`;
		if (k in daemon) return daemon[k];
		if (k === "POST /api/jobs") return body.lecture_ids.flatMap((id) => body.kinds.map((kind) => ({ id: `${id}-${kind}`, lecture_id: id, kind, state: "queued" })));
		if (method === "GET") throw Object.assign(new Error("not found"), { status: 404 });
		return { ok: true, changed: body?.ids?.length ?? 0 };
	};
	const prefs = { provider: "p", model: "m1", thinking: "medium", approval: over.approval ?? "auto" };
	const deps = {
		bot,
		api,
		appState: () => ({ textScale: 1, themeMode: "system" }),
		pipeline: {
			prefs,
			describe: async () => ({ providers: [{ id: "p", models: [{ id: "m1", available: true }, { id: "m2", available: true }, { id: "m3", available: false }] }] }),
			setPrefs: async (patch) => Object.assign(prefs, patch),
		},
		tasks: { create: async (t) => ({ id: "t1", ...t }), remove: async () => {} },
		plugins: { list: async () => [{ id: "demo", name: "Demo", enabled: true }], update: async () => {} },
	};
	const tools = Object.fromEntries(operateTools(deps, over.conv).map((t) => [t.name, t]));
	return { bot, calls, deps, tools, prefs, done: () => rm(dir, { recursive: true, force: true }) };
}

test("material batches reject unrequested writes and use one approval and undo", async () => {
	const conv = { course_id: "c1", lastUser: "这份课件写了什么？" };
	const { bot, tools, deps, calls, prefs, done } = await setup({
		approval: "ask",
		conv,
		daemon: {
			"POST /api/courses/c1/material-files/batches": { id: "b1", lines: ["旧.pdf → 新.pdf"] },
		},
	});
	const input = { edits: [{ op: "rename", path: "旧.pdf", name: "新.pdf", version: "a".repeat(64) }] };
	try {
		await assert.rejects(tools.organize_course_files.execute("1", input), /本轮用户消息/);
		assert.equal(calls.length, 0);
		conv.lastUser = "把课件改名";
		const pending = await tools.organize_course_files.execute("2", input);
		assert.equal(pending.details.proposal.state, "pending");
		assert.deepEqual(calls.map((r) => r.path), ["/api/courses/c1/material-files/batches"]);
		const proposal = bot.proposal(pending.details.proposal.id);
		await runOp(proposal.op, deps);
		await runOp(proposal.undo, deps);
		assert.deepEqual(calls.slice(-2).map((r) => r.path), [
			"/api/material-file-batches/b1/apply", "/api/material-file-batches/b1/undo",
		]);
		prefs.approval = "auto";
		const applied = await tools.organize_course_files.execute("3", input);
		assert.equal(calls.at(-1).path, "/api/material-file-batches/b1/apply");
		assert.equal(bot.proposal(applied.details.app.proposal_id).state, "applied");
		const update = { ...input, edits: [{ ...input.edits[0], op: "edit_markdown" }] };
		const change = await tools.organize_course_files.execute("4", update);
		assert.equal(change.details.proposal.state, "pending");
		await bot.transition(change.details.proposal.id, ["pending"], "dismissed");
		assert.equal(bot.proposal(change.details.proposal.id).state, "dismissed");
		assert.equal(calls.at(-1).path, "/api/courses/c1/material-files/batches");
		await assert.rejects(tools.organize_course_files.execute("4", { ...input, course_id: "c2" }), /明确指定课程/);
	} finally {
		await done();
	}
});

test("the registry validates values and lists no secret", () => {
	assert.equal(checkSetting("textScale", 1.2), 1.2);
	assert.throws(() => checkSetting("textScale", 3), /0.8–1.5/);
	assert.throws(() => checkSetting("seekStepSec", 7.5), /整数/);
	assert.throws(() => checkSetting("themeMode", "blue"), /system/);
	assert.throws(() => checkSetting("mineru.token", "x"), /不能修改/);
	assert.throws(() => checkSetting("download.rate_limit", 3), /不能修改/);
	for (const k of Object.keys(SETTINGS)) assert.doesNotMatch(k, /token|key|password|secret|auth|root/i);
});

test("ordinary app preferences apply with undo; batches wait for confirmation", async () => {
	const { tools, bot, deps, calls, prefs, done } = await setup();
	try {
		const direct = await tools.change_settings.execute("1", { changes: [{ key: "textScale", value: 1.2 }] });
		assert.deepEqual(direct.details.app.set, { textScale: 1.2 });
		assert.equal(bot.proposal(direct.details.app.proposal_id).undo.app.textScale, 1);
		const batch = await tools.change_settings.execute("2", { changes: [{ key: "contrast", value: "high" }, { key: "model", value: "p/m2" }] });
		assert.equal(batch.details.proposal.state, "pending");
		assert.equal(prefs.model, "m1");
		const applied = await runOp(bot.proposal(batch.details.proposal.id).op, deps);
		assert.deepEqual(applied, { app: { contrast: "high" } });
		assert.equal(prefs.model, "m2");
		await assert.rejects(tools.change_settings.execute("3", { changes: [{ key: "model", value: "p/m3" }] }), /不可用/);
		await assert.rejects(runOp({ type: "settings", daemon: { "transcript.delete_video": true } }, deps), /运行配置/);
		await assert.rejects(runOp({ type: "settings", agent: { approval: "auto" } }, deps), /不能修改/);
		assert.equal(calls.some((c) => c.path === "/api/settings"), false);
	} finally {
		await done();
	}
});

test("ask mode holds application actions until the user applies them", async () => {
	const { tools, bot, calls, deps, done } = await setup({ approval: "ask" });
	try {
		const r = await tools.open_page.execute("1", { target: "lecture", lecture_id: "l1" });
		assert.equal(r.details.proposal.state, "pending");
		assert.equal(r.details.app, undefined);
		assert.deepEqual(await runOp(bot.proposal(r.details.proposal.id).op, deps), {
			app_op: { op: "open", lecture_id: "l1", target: "lecture" },
		});
		const d = await tools.download.execute("2", { course_id: "c1", latest: 1, kinds: ["video"] });
		assert.equal(d.details.proposal.state, "pending");
		assert.equal(calls.filter((c) => c.method === "POST").length, 0);
		assert.match(d.content[0].text, /不要声称/);
	} finally {
		await done();
	}
});

test("download jobs pause and cancel with their undo", async () => {
	const { tools, bot, deps, calls, done } = await setup();
	try {
		const r = await tools.manage_downloads.execute("1", { action: "pause" });
		assert.equal(r.details.proposal.state, "pending");
		assert.equal(calls.filter((c) => c.method === "POST").length, 0);
		await runOp(bot.proposal(r.details.proposal.id).op, deps);
		assert.deepEqual(calls.at(-1), { method: "POST", path: "/api/jobs/bulk", body: { action: "pause", ids: ["j1", "j3"] } });
		assert.deepEqual(bot.proposal(r.details.proposal.id).undo, { type: "jobs", action: "resume", ids: ["j1", "j3"] });
		const none = await tools.manage_downloads.execute("2", { action: "retry" });
		assert.match(none.content[0].text, /没有符合条件/);
		const c = await tools.manage_downloads.execute("3", { action: "cancel", job_ids: ["j2"] });
		const undo = bot.proposal(c.details.app.proposal_id).undo;
		assert.equal(undo.type, "requeue");
		await runOp(undo, deps);
		assert.deepEqual(calls.at(-1), { method: "POST", path: "/api/jobs", body: { lecture_ids: [undefined], kinds: ["video"] } });
	} finally {
		await done();
	}
});

test("download picks a course's lectures by date or latest and skips what is local", async () => {
	const { tools, bot, calls, done } = await setup();
	try {
		const r = await tools.download.execute("1", { course_id: "c1", from: "2026-09-01", to: "2026-09-15", kinds: ["video", "transcript"] });
		const posts = calls.filter((c) => c.method === "POST");
		// l1 has both, l2 needs only the transcript, l3 needs both.
		assert.deepEqual(
			posts.map((c) => c.body),
			[
				{ lecture_ids: ["l2"], kinds: ["transcript"] },
				{ lecture_ids: ["l3"], kinds: ["video", "transcript"] },
			],
		);
		assert.match(r.content[0].text, /1 节已在本机/);
		const p = bot.proposal(r.details.app.proposal_id);
		assert.equal(p.state, "applied");
		assert.deepEqual(p.undo, { type: "jobs", action: "cancel", ids: ["l2-transcript", "l3-video", "l3-transcript"] });
		await tools.download.execute("2", { course_id: "c1", latest: 2, kinds: ["slides"] });
		assert.deepEqual(calls.at(-1).body, { lecture_ids: ["l3", "l4"], kinds: ["slides"] }, "latest counts recorded lectures only");
		const none = await tools.download.execute("3", { course_id: "c1", to: "2026-09-01", kinds: ["video", "transcript"] });
		assert.match(none.content[0].text, /不用下载/);
		await assert.rejects(tools.download.execute("4", { lecture_ids: ["nope"], kinds: ["video"] }), /没有节次/);
		await assert.rejects(tools.download.execute("5", { kinds: ["video"] }), /course_id/);
		await assert.rejects(tools.download.execute("6", { course_id: "c1", from: "9月1日", kinds: ["video"] }), /YYYY-MM-DD/);
	} finally {
		await done();
	}
});

test("download undo leaves jobs that were already running", async () => {
	const { tools, bot, done } = await setup({ daemon: { "GET /api/jobs": [{ id: "l3-video", state: "running", kind: "video" }] } });
	try {
		const r = await tools.download.execute("1", { lecture_ids: ["l1"], kinds: ["video"] });
		assert.deepEqual(bot.proposal(r.details.app.proposal_id).undo, { type: "jobs", action: "cancel", ids: ["l1-video"] });
		assert.ok(MAX_DOWNLOAD >= 50);
	} finally {
		await done();
	}
});

test("deleting files always waits for a card", async () => {
	const { tools, bot, deps, calls, done } = await setup();
	try {
		const d = await tools.delete_files.execute("1", { lecture_ids: ["l1"], kinds: ["video"] });
		assert.equal(d.details.proposal.state, "pending");
		assert.equal(calls.filter((c) => c.method === "DELETE").length, 0);
		await runOp(bot.proposal(d.details.proposal.id).op, deps);
		assert.deepEqual(calls.at(-1), { method: "DELETE", path: "/api/lectures/l1/assets/video", body: undefined });
		await runOp({ type: "delete_files", lecture_id: "l1", kinds: ["slides"] }, deps);
		assert.equal(calls.at(-1).path, "/api/lectures/l1/assets/slides", "older cards still apply");
	} finally {
		await done();
	}
});

test("course changes apply at once and undo to the previous values", async () => {
	const { tools, bot, deps, calls, done } = await setup();
	try {
		const r = await tools.manage_course.execute("1", { course_id: "c1", pinned: true, color: "teal" });
		assert.deepEqual(calls.at(-1), { method: "PATCH", path: "/api/courses/c1", body: { pinned: true, color: "#2E6E6A" } });
		const p = bot.proposal(r.details.app.proposal_id);
		await runOp(p.undo, deps);
		assert.deepEqual(calls.at(-1).body, { pinned: false, color: "" });
		await assert.rejects(tools.manage_course.execute("2", { course_id: "c9", pinned: true }), /没有课程/);
	} finally {
		await done();
	}
});

test("tasks are undoable and plugin authorization is user-only", async () => {
	const { tools, bot, done } = await setup();
	try {
		await assert.rejects(tools.create_task.execute("1", { name: "x", prompt: "y", schedule: { type: "daily", time: "25:00" } }), /HH:MM/);
		const r = await tools.create_task.execute("2", { name: "周复习", prompt: "复习本周课程", schedule: { type: "weekly", days: [7], time: "20:00" } });
		assert.deepEqual(bot.proposal(r.details.app.proposal_id).undo, { type: "delete_task", id: "t1" });
		assert.equal(tools.toggle_plugin, undefined);
	} finally {
		await done();
	}
});

test("list_downloads reports states, progress and errors", async () => {
	const { tools, done } = await setup({
		daemon: {
			"GET /api/jobs": [
				{ id: "a", state: "running", title: "第 1 节", kind: "video", progress: 0.426, course_id: "c1" },
				{ id: "b", state: "failed", title: "第 2 节", kind: "transcript", error: "网络中断", course_id: "c1" },
				{ id: "c", state: "done", title: "第 3 节", kind: "slides", course_id: "c2" },
			],
		},
	});
	try {
		const r = (await tools.list_downloads.execute("1", {})).content[0].text;
		assert.match(r, /下载中 1 · 失败 1 · 已完成 1/);
		assert.match(r, /第 1 节 · 视频 · 下载中 42%/);
		assert.match(r, /网络中断/);
		assert.doesNotMatch(r, /第 3 节/);
		assert.doesNotMatch((await tools.list_downloads.execute("2", { state: "all", course_id: "c1" })).content[0].text, /第 3 节/);
	} finally {
		await done();
	}
});

test("open_page needs the id its target names", async () => {
	const { tools, done } = await setup();
	try {
		await assert.rejects(tools.open_page.execute("1", { target: "lecture" }), /lecture_id/);
		const r = await tools.open_page.execute("2", { target: "lecture", lecture_id: "l1", t: 90 });
		assert.deepEqual(r.details.result.app_op, { op: "open", target: "lecture", lecture_id: "l1", t: 90 });
	} finally {
		await done();
	}
});

test("workspace tools disclose irreversible writes and preserve later card moves", async () => {
	const conv = {course_id:"c1", context:{lecture_id:"l1", study_canvas:"yes"}, lastUser:"新建文件夹，将笔记添加到画布，打开复习讲义，移动卡片"};
	const card = {id:"card",title:"笔记",x:10,y:20};
	const note = {id:"note",title:"笔记",course_id:"c1",lecture_id:"l1"};
	const {courseReviewId}=await import("./course-review.mjs");
	const {tools,bot,calls,deps,done}=await setup({conv, daemon:{
		"GET /api/lectures/l1":{lecture:{course_id:"c1"}},
		"GET /api/lectures/l1/study-canvas":{objects:[card],base_hash:"current"},
		"GET /api/notes/note":note,
		["GET /api/notes/"+courseReviewId("c1")]:note,
		"GET /api/courses/c1/lectures":{course:{id:"c1",title:"课程"},lectures:[{id:"l1",course_id:"c1"}]},
	}});
	try {
		for (const [name,args] of [["create_course_folder",{name:"习题"}],["prepare_course_review",{}],["pin_course_note",{note_id:"note",lecture_id:"l1"}]]) {
			const result=await tools[name].execute("1",args);
			assert.equal(result.details.proposal.state,"pending");
			assert.match(result.details.proposal.lines.join(" "),/无法.*撤销/);
		}
		assert.ok(calls.every(call=>call.method==="GET"));
		const result=await tools.canvas_control.execute("2",{action:"move",card_id:"card",x:30,y:40});
		const undo=bot.proposal(result.details.app.proposal_id).undo;
		await assert.rejects(runOp(undo,deps),/位置已变化/);
		Object.assign(card,{x:30,y:40});
		const reverse=(await runOp(undo,deps)).app_op;
		assert.deepEqual([reverse.x,reverse.y,reverse.base_hash],[10,20,"current"]);
	} finally {await done();}
});
