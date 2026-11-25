// The assistant operating the app: navigation, the player, settings,
// downloads, courses, scheduled tasks and plugins. Operations run at once and
// are recorded with their undo; deleting downloaded files, which cannot be
// undone, waits for the user to confirm it on a card.
// Contract: docs/specs/0007-resident-bot-and-course-agents.md, amended by
// docs/specs/0016-agent-acts.md
import { Type } from "typebox";
import { validateTask } from "./tasks.mjs";
import { courseFileTools } from "./course-files.mjs";

const bad = (message, status = 400) => Object.assign(new Error(message), { status });
const text = (t, details = {}) => ({ content: [{ type: "text", text: t }], details });
const enc = encodeURIComponent;
const lit = (values) => Type.Union(values.map((v) => Type.Literal(v)));


// Hex values of the app's seed palette (kSeedPalette in app/lib/theme/prefs.dart).
export const COURSE_COLORS = {
	none: "",
	blue: "#3F6CB4",
	teal: "#2E6E6A",
	green: "#4F7A3A",
	amber: "#B0782A",
	red: "#B4443A",
	rose: "#B0507A",
	violet: "#6E5AA8",
	graphite: "#5D6670",
};
export const DOWNLOAD_KINDS = ["video", "slides", "transcript"];
const KIND_LABEL = { video: "视频", slides: "PPT 快照", transcript: "转写", courseware: "课件", parse: "课件解析" };
const JOB_STATE = { queued: "排队中", running: "下载中", paused: "已暂停", failed: "失败", done: "已完成", canceled: "已取消" };
export const MAX_DOWNLOAD = 50;

/**
 * Settings the bot may read and change. where: app (UiPrefs JSON keys, applied
 * by the app), daemon (GET/PUT /api/settings) or agent (assistant prefs). No
 * secret is listed, so none is readable or writable. Every change has an undo.
 */
export const SETTINGS = {
	themeMode: { where: "app", label: "主题模式", enum: ["system", "light", "dark"] },
	contrast: { where: "app", label: "对比度", enum: ["standard", "medium", "high"] },
	textScale: { where: "app", label: "界面字号", min: 0.8, max: 1.5 },
	chatTextScale: { where: "app", label: "对话字号", min: 0.8, max: 1.5 },
	chatBubbles: { where: "app", label: "对话气泡", type: "boolean" },
	showThinking: { where: "app", label: "显示思考过程", type: "boolean" },
	reduceMotion: { where: "app", label: "减少动效", type: "boolean" },
	defaultSpeed: { where: "app", label: "默认倍速", min: 0.5, max: 3 },
	boostSpeed: { where: "app", label: "长按加速倍速", min: 1, max: 4 },
	seekStepSec: { where: "app", label: "快进快退步长（秒）", min: 5, max: 60, int: true },
	showSubtitles: { where: "app", label: "显示字幕", type: "boolean" },
	subtitleScale: { where: "app", label: "字幕大小", min: 0.6, max: 2 },
	lecturesNewestFirst: { where: "app", label: "课程页最新一节在前", type: "boolean" },
	model: { where: "agent", label: "助手默认模型（provider/model）", type: "string" },
	thinking: { where: "agent", label: "思考强度", enum: ["off", "minimal", "low", "medium", "high", "xhigh"] },
};

/** The value checked against the registry entry; throws a message listing what is allowed. */
export function checkSetting(key, value) {
	const s = SETTINGS[key];
	if (!s) throw bad(`不能修改 ${key}。可修改的设置：${Object.keys(SETTINGS).join("、")}`);
	if (s.enum) {
		if (!s.enum.includes(value)) throw bad(`${key}（${s.label}）只能是 ${s.enum.map((v) => JSON.stringify(v)).join("、")}`);
		return value;
	}
	if (s.type === "boolean") {
		if (typeof value !== "boolean") throw bad(`${key}（${s.label}）应为 true 或 false`);
		return value;
	}
	if (s.type === "string") {
		if (typeof value !== "string" || !value.trim() || value.length > 120) throw bad(`${key}（${s.label}）应为非空字符串`);
		return value.trim();
	}
	const n = Number(value);
	if (typeof value !== "number" || !Number.isFinite(n) || n < s.min || n > s.max || (s.int && !Number.isInteger(n))) {
		throw bad(`${key}（${s.label}）应为 ${s.min}–${s.max} 之间的${s.int ? "整数" : "数"}`);
	}
	return Math.round(n * 100) / 100;
}

function ordinaryPrefs(value, where) {
	return Object.fromEntries(Object.entries(value ?? {}).map(([key, val]) => {
		if (SETTINGS[key]?.where !== where) throw bad(`不能修改 ${key}`);
		return [key, checkSetting(key, val)];
	}));
}

const show = (v) => (typeof v === "boolean" ? (v ? "开" : "关") : v === "" ? "（自动）" : String(v));

/**
 * Runs a proposal's op (built by the host, never by the model). deps: api,
 * tasks, plugins, pipeline, changed() after plugin or prefs changes. Returns a
 * result; app preferences come back as {app: {...}} for the app to apply.
 */
export async function runOp(op, deps) {
	switch (op.type) {
		case "app":
			return { app: ordinaryPrefs(op.set, "app") };
		case "native_command":
			return { app_op: op.command };
		case "save_note":
			return { note: await deps.api("PUT", "/api/notes", op.note) };
		case "new_schedule":
			return deps.api("POST", "/api/schedule", op.item);
		case "new_quiz":
			return { quiz: await deps.api("POST", "/api/quizzes", op.quiz) };
		case "sync_courses":
			return { courses: ((await deps.api("GET", "/api/courses?sync=1")) ?? []).length };
		case "delete_schedule": {
			const item = (await deps.api("GET", "/api/schedule")).find((i) => i.id === op.id);
			if (!item || item.source !== "agent" || JSON.stringify(item) !== op.base) throw bad("这条日程已修改，请重新查看");
			return deps.api("DELETE", `/api/schedule/${enc(op.id)}`);
		}
		case "edit_schedule":
			return deps.api("PATCH", `/api/schedule/${enc(op.id)}`, op.patch);
		case "save_outline": {
			const path = `/api/lectures/${enc(op.lecture_id)}/timeline`;
			if (JSON.stringify(await deps.api("GET", path)) !== op.base) throw bad("大纲已修改，请重新读取");
			return { timeline: await deps.api("PUT", path, { text: op.text }), lecture_id: op.lecture_id };
		}
		case "course_review":
			return deps.api("POST", `/api/courses/${enc(op.course_id)}/review-handout`, { lecture_id: op.lecture_id, body: op.body });
		case "course_folder":
			return deps.api("POST", `/api/courses/${enc(op.course_id)}/material-folder`, { name: op.name });
		case "canvas_note":
			return deps.api("POST", `/api/courses/${enc(op.course_id)}/canvas-note`, { lecture_id: op.lecture_id, note_id: op.note_id });
		case "settings": {
			if (op.daemon && Object.keys(op.daemon).length) throw bad("运行配置只能由用户在设置页调整");
			const app = ordinaryPrefs(op.app, "app");
			const patch = ordinaryPrefs(op.agent, "agent");
			if (Object.keys(patch).length) {
				if (patch.model) {
					const at = patch.model.indexOf("/");
					patch.provider = patch.model.slice(0, at);
					patch.model = patch.model.slice(at + 1);
				}
				await deps.pipeline.setPrefs(patch);
			}
			return Object.keys(app).length ? { app } : { ok: true };
		}
		case "jobs":
			return deps.api("POST", "/api/jobs/bulk", { action: op.action, ids: op.ids });
		case "download": {
			// groups: lectures by the kinds each still needs; older cards carry one group.
			const ids = [];
			for (const g of op.groups ?? [op]) {
				for (const j of (await deps.api("POST", "/api/jobs", { lecture_ids: g.lecture_ids, kinds: g.kinds })) ?? []) ids.push(j.id);
			}
			return { jobs: ids.length, ids };
		}
		case "requeue": {
			// Undo of a cancel: the canceled jobs queued again, grouped by kind.
			const byKind = new Map();
			for (const { lecture_id, kind } of op.items) byKind.set(kind, [...(byKind.get(kind) ?? []), lecture_id]);
			let jobs = 0;
			for (const [kind, ids] of byKind) jobs += ((await deps.api("POST", "/api/jobs", { lecture_ids: ids, kinds: [kind] })) ?? []).length;
			return { jobs };
		}
		case "delete_files": {
			let freed = 0;
			for (const id of op.lecture_ids ?? [op.lecture_id]) {
				for (const kind of op.kinds) freed += (await deps.api("DELETE", `/api/lectures/${enc(id)}/assets/${kind}`))?.freed ?? 0;
			}
			return { freed };
		}
		case "course":
			await deps.api("PATCH", `/api/courses/${enc(op.course_id)}`, op.patch);
			return { ok: true };
		case "task": {
			const t = await deps.tasks.create(op.task);
			return { task_id: t.id };
		}
		case "delete_memory": {
			const change = await deps.memory.remove(op.path, op.base_hash);
			deps.memoryChanged(change);
			return { memory: change };
		}
		case "delete_task":
			await deps.tasks.remove(op.id);
			return { ok: true };
		case "course_files":
			return deps.api("POST", `/api/material-file-batches/${enc(op.id)}/apply`, {});
		case "undo_course_files":
			return deps.api("POST", `/api/material-file-batches/${enc(op.id)}/undo`, {});
		default:
			throw bad(`未知操作 ${op.type}`);
	}
}

/**
 * Operate tools for the bot and course agents. deps: bot (operation records
 * and cards), api, appState() (the app's last pushed preferences), pipeline
 * (prefs.approval: "ask" turns changes into cards), tasks, plugins, changed().
 */
export function operationAct(deps) {
	const { bot } = deps;
	const card = async (p) => {
		const saved = await bot.propose(p);
		return { proposal: bot.view(saved) };
	};
	const done = async (p, result) => {
		const saved = await bot.propose({ ...p, applied: true, result });
		return bot.view(saved);
	};
	/** The approval mode the user picked in the composer. */
	const asking = () => deps.pipeline.prefs.approval === "ask";

	/**
	 * Runs op now and records it with its undo (a 撤销 chip), or makes a card
	 * the user applies: in ask mode, and always when confirm is set. title
	 * names the change ("下载 3 节的视频"), doneTitle the outcome; undo may be
	 * a function of the result; app replaces the chip's operation; said(result)
	 * is what the model reads.
	 */
	return async ({ doneTitle, said, app, confirm = asking(), forceConfirm, undo, ...p }) => {
		if (confirm || forceConfirm) {
			const details = await card({ ...p, undo: typeof undo === "function" ? undefined : undo });
			return text(`已在聊天里生成卡片「${p.title}」，用户点「应用」后才会执行，不要声称已经完成。`, details);
		}
		const result = await runOp(p.op, deps);
		const view = await done({ ...p, title: doneTitle, undo: typeof undo === "function" ? undo(result) : undo }, result);
		const chip = { ...(result.app_op ?? app ?? { op: "done" }), label: doneTitle, ...(view.undoable ? { proposal_id: view.id } : {}) };
		return text(`${said(result)}${view.undoable ? "（用户可以撤销）" : ""}。`, { app: chip, result });
	};
}

export function operateTools(deps, conv) {
	const { api } = deps;
	const act = operationAct(deps);
	const asking = () => deps.pipeline.prefs.approval === "ask";

	const command = (op, title, lines = []) => act({
		kind: "native_command", title, doneTitle: `已发送：${title}`, lines,
		op: { type: "native_command", command: op }, said: () => `已发送应用指令「${title}」`,
	});

	const currentSettings = async () => {
		const app = deps.appState() ?? {};
		const agent = deps.pipeline.prefs;
		return Object.fromEntries(Object.entries(SETTINGS).map(([key, s]) => [key,
			s.where === "app" ? app[key] : key === "model" && agent.provider && agent.model ? `${agent.provider}/${agent.model}` : agent[key],
		]));
	};
	/** Lectures by id, or a course's recorded lectures narrowed by date or the latest n; oldest first. */
	const pickLectures = async ({ lecture_ids, course_id, from, to, latest }) => {
		for (const d of [from, to]) if (d !== undefined && !/^\d{4}-\d{2}-\d{2}$/.test(d)) throw bad("from 和 to 的格式应为 YYYY-MM-DD");
		let list;
		if (lecture_ids?.length) {
			list = [];
			for (const id of new Set(lecture_ids)) {
				const l = (await api("GET", `/api/lectures/${enc(id)}`).catch(() => null))?.lecture;
				if (!l) throw bad(`没有节次 ${id}。用 list_lectures 查节次 id，或改用 course_id`);
				list.push(l);
			}
		} else if (course_id) {
			const r = await api("GET", `/api/courses/${enc(course_id)}/lectures`).catch(() => null);
			if (!r?.lectures) throw bad(`没有课程 ${course_id}。用 list_courses 查课程 id`);
			list = r.lectures.filter((l) => l.has_playback !== false).sort((a, b) => String(a.date ?? "").localeCompare(String(b.date ?? "")));
		} else {
			throw bad("需要 lecture_ids，或 course_id（可加 from、to、latest 缩小范围）");
		}
		const day = (l) => String(l.date ?? "").slice(0, 10);
		if (from) list = list.filter((l) => day(l) >= from);
		if (to) list = list.filter((l) => day(l) <= to);
		if (latest) list = list.slice(-latest);
		return list;
	};
	const local = (l, kind) => (l.assets ?? []).includes(kind) || (kind === "transcript" && l.segments > 0);
	const lectureLines = (ls, max = 12) => [...ls.slice(0, max).map((l) => `${l.date ?? ""} ${l.title ?? l.id}`.trim()), ...(ls.length > max ? [`…共 ${ls.length} 节`] : [])];

	return [
		{
			name: "open_page",
			label: "打开页面",
			description:
				"Navigate the app for the user. targets: home, courses, course (course_id), lecture (lecture_id, optional t seconds), canvas (lecture_id), note (note_id), schedule (optional day YYYY-MM-DD), downloads, library, sources, lms, tasks, plugins, settings (optional section), activity, usage, bot.",
			promptSnippet: "open_page(target, …): navigate the app (course, lecture, note, schedule, downloads, settings, …)",
			parameters: Type.Object({
				target: lit(["home", "courses", "course", "lecture", "canvas", "note", "schedule", "downloads", "library", "sources", "lms", "tasks", "plugins", "settings", "activity", "usage", "bot"]),
				course_id: Type.Optional(Type.String()),
				lecture_id: Type.Optional(Type.String()),
				note_id: Type.Optional(Type.String()),
				t: Type.Optional(Type.Number({ description: "Seconds into the lecture" })),
				day: Type.Optional(Type.String({ description: "YYYY-MM-DD" })),
				section: Type.Optional(lit(["appearance", "player", "assistant", "models", "plugins", "downloads", "transcript", "courseware", "knowledge", "storage", "obsidian", "notion", "sync", "backup", "notifications", "diagnostics"])),
			}),
			execute: async (_id, p) => {
				const need = { course: "course_id", lecture: "lecture_id", canvas: "lecture_id", note: "note_id" }[p.target];
				if (need && !p[need]) throw bad(`打开 ${p.target} 需要 ${need}`);
				if (p.t != null && (!Number.isFinite(p.t) || p.t < 0 || p.t > 7 * 86400)) throw bad("播放时间超出有效范围");
				if (p.day && !/^\d{4}-\d{2}-\d{2}$/.test(p.day)) throw bad("day 格式应为 YYYY-MM-DD");
				const app = { op: "open", ...p };
				return command(app, `打开 ${p.target}`, [p.course_id ?? p.lecture_id ?? p.note_id ?? p.target]);
			},
		},
		{
			name: "player",
			label: "控制播放",
			description: "Control the open player: play, pause, seek (seconds) or speed (rate 0.5–3). Only when the context says a lecture is playing.",
			parameters: Type.Object({
				action: lit(["play", "pause", "seek", "speed"]),
				seconds: Type.Optional(Type.Number()),
				rate: Type.Optional(Type.Number()),
			}),
			execute: async (_id, p) => {
				if (p.action === "seek" && (!Number.isFinite(p.seconds) || !(p.seconds >= 0 && p.seconds <= 7 * 86400))) throw bad("seek 需要 seconds");
				if (p.action === "speed" && !(p.rate >= 0.5 && p.rate <= 3)) throw bad("speed 需要 0.5–3 之间的 rate");
				const lecture_id = conv?.context?.player_lecture_id || conv?.context?.lecture_id;
				if (conv && !lecture_id) throw bad("请先打开要操作的播放器");
				const title = { play: "播放视频", pause: "暂停视频", seek: `跳到 ${p.seconds} 秒`, speed: `设为 ${p.rate} 倍速` }[p.action];
				return command({ op: "player", ...p, lecture_id }, title);
			},
		},
		{
			name: "canvas_control",
			label: "操作画布与 PDF",
			description: "Use the open lecture canvas. List cards first. Move or reshape one card, turn to a PDF page, or change PDF reading view. Use coordinates in canvas units. Requires the student's current request.",
			parameters: Type.Object({
				action: lit(["list", "move", "resize", "pdf_page", "pdf_view"]),
				card_id: Type.Optional(Type.String()),
				x: Type.Optional(Type.Number()), y: Type.Optional(Type.Number()),
				width: Type.Optional(Type.Number()), height: Type.Optional(Type.Number()),
				page: Type.Optional(Type.Integer({ minimum: 1, maximum: 5000 })),
				view: Type.Optional(lit(["scroll", "paged", "column"])),
			}),
			execute: async (_id, p) => {
				const lecture_id = conv?.context?.lecture_id || conv?.context?.player_lecture_id;
				if (!lecture_id || conv?.context?.study_canvas !== "yes") throw bad("请先打开要操作的课程画布");
				const d = await api("GET", `/api/lectures/${enc(lecture_id)}/study-canvas`);
				const view = ({ id, title, type, x, y, w, h, page, page_count, position_locked }) => ({ id, title, type, x, y, w, h, page, page_count, position_locked });
				if (p.action === "list") return text(JSON.stringify((d.objects ?? []).slice(0, 60).map(view)));
				if (!/移动|位置|放到|拖到|缩放|调整|大小|放大|缩小|页|滚动|展开|视图|move|resize|page|view/i.test(conv.lastUser ?? "")) throw bad("需要用户请求操作画布或 PDF");
				const target = d.objects?.find((c) => c.id === p.card_id);
				if (!target) throw bad("找不到目标卡片");
				const geometry = p.action === "move" ? [p.x, p.y] : p.action === "resize" ? [p.width, p.height] : [];
				if (geometry.some((v) => !Number.isFinite(v))) throw bad("缺少卡片位置或尺寸");
				if (["move", "resize"].includes(p.action) && target.position_locked) throw bad("卡片已锁定位置");
				if (p.action.startsWith("pdf_") && target.type !== "pdfdoc") throw bad("目标不是 PDF 文档");
				if (p.action === "pdf_page" && (!Number.isInteger(p.page) || p.page < 1 || p.page > target.page_count)) throw bad("页码超出文档范围");
				if (p.action === "pdf_view" && !["scroll", "paged", "column"].includes(p.view)) throw bad("阅读视图无效");
				const action = { move: "移动", resize: "调整大小", pdf_page: `翻到第 ${p.page} 页：`, pdf_view: "切换 PDF 视图：" }[p.action];
				return command({ op: "canvas", ...p, lecture_id, base_hash: d.base_hash ?? "" }, `${action}「${target.title}」`);
			},
		},
		{
			name: "read_settings",
			label: "读取设置",
			description: "Read the settings you can change, with their current and allowed values, and the approval mode the user picked.",
			parameters: Type.Object({}),
			execute: async () => {
				const cur = await currentSettings();
				const lines = Object.entries(SETTINGS).map(([k, s]) => {
					const allowed = s.enum ? s.enum.map((v) => JSON.stringify(v)).join("|") : s.type ?? `${s.min}–${s.max}`;
					return `- ${k}（${s.label}）= ${JSON.stringify(cur[k] ?? null)}；可选 ${allowed}`;
				});
				const mode = asking() ? "询问（改动先生成卡片，用户应用后执行）" : "自动（改动立即生效，用户可以撤销）";
				return text(`操作模式：${mode}\n当前设置：\n${lines.join("\n")}`);
			},
		},
		{
			name: "change_settings",
			label: "修改设置",
			description: "Change ordinary appearance, playback, default model and thinking settings by registry key (see read_settings).",
			parameters: Type.Object({
				changes: Type.Array(Type.Object({ key: Type.String(), value: Type.Any() }), { minItems: 1, maxItems: 10 }),
			}),
			execute: async (_id, { changes }) => {
				const clean = changes.map(({ key, value }) => ({ key, value: checkSetting(key, value) }));
				const cur = await currentSettings();
				if (clean.some((c) => c.key === "model")) {
					const desc = await deps.pipeline.describe();
					const ok = new Set(desc.providers.flatMap((p) => p.models.filter((m) => m.available).map((m) => `${p.id}/${m.id}`)));
					for (const c of clean) if (c.key === "model" && !ok.has(c.value)) throw bad(`模型 ${c.value} 不可用。可用：${[...ok].slice(0, 20).join("、")}`);
				}
				const op = { type: "settings", app: {}, daemon: {}, agent: {} };
				const undo = { type: "settings", app: {}, daemon: {}, agent: {} };
				for (const c of clean) {
					const where = SETTINGS[c.key].where;
					op[where][c.key] = c.value;
					// An unset default model has nothing to return to.
					if (cur[c.key] !== undefined && !(c.key === "model" && !cur[c.key])) undo[where][c.key] = cur[c.key];
				}
				const lines = clean.map((c) => `${SETTINGS[c.key].label}：${show(cur[c.key])} → ${show(c.value)}`);
				const names = clean.map((c) => SETTINGS[c.key].label).join("、");
				return act({
					kind: "settings",
					title: `修改${names}`,
					doneTitle: `已调整${names}`,
					forceConfirm: clean.length > 1,
					lines,
					op,
					undo,
					app: Object.keys(op.app).length ? { op: "prefs", set: op.app } : undefined,
					said: () => `已调整 ${lines.join("；")}`,
				});
			},
		},
		{
			name: "list_downloads",
			label: "查看下载",
			description: "List download jobs with state, progress and errors. state: active (queued, running, paused), failed, done or all; without it, active and failed.",
			parameters: Type.Object({ state: Type.Optional(lit(["active", "failed", "done", "all"])), course_id: Type.Optional(Type.String()) }),
			execute: async (_id, { state, course_id }) => {
				const want = { active: ["queued", "running", "paused"], failed: ["failed"], done: ["done"], all: null }[state] ?? ["queued", "running", "paused", "failed"];
				let jobs = (await api("GET", "/api/jobs")) ?? [];
				if (course_id) jobs = jobs.filter((j) => j.course_id === course_id);
				const counts = new Map();
				for (const j of jobs) counts.set(j.state, (counts.get(j.state) ?? 0) + 1);
				const head = [...counts].map(([s, n]) => `${JOB_STATE[s] ?? s} ${n}`).join(" · ") || "没有下载任务";
				const rows = jobs.filter((j) => !want || want.includes(j.state));
				const line = (j) =>
					`- ${j.title || j.lecture_id} · ${KIND_LABEL[j.kind] ?? j.kind} · ${JOB_STATE[j.state] ?? j.state}` +
					(j.state === "running" || j.state === "paused" ? ` ${Math.floor((j.progress ?? 0) * 100)}%` : "") +
					(j.error ? ` · ${j.error}` : "") +
					` (id ${j.id})`;
				return text([head, ...rows.slice(0, 40).map(line), ...(rows.length > 40 ? [`…还有 ${rows.length - 40} 个`] : [])].join("\n"));
			},
		},
		{
			name: "manage_downloads",
			label: "管理下载",
			description: "Pause, resume, retry or cancel download jobs (every matching job without job_ids). The user can undo.",
			parameters: Type.Object({ action: lit(["pause", "resume", "retry", "cancel"]), job_ids: Type.Optional(Type.Array(Type.String(), { maxItems: 200 })) }),
			execute: async (_id, { action, job_ids }) => {
				const from = { pause: ["queued", "running"], resume: ["paused"], retry: ["failed"], cancel: ["queued", "running", "paused"] }[action];
				const jobs = ((await api("GET", "/api/jobs")) ?? []).filter((j) => from.includes(j.state) && (!job_ids?.length || job_ids.includes(j.id)));
				if (!jobs.length) return text("没有符合条件的下载任务。");
				const ids = jobs.map((j) => j.id);
				const verb = { pause: "暂停", resume: "继续", retry: "重试", cancel: "取消" }[action];
				const undo =
					action === "cancel" ? { type: "requeue", items: jobs.map((j) => ({ lecture_id: j.lecture_id, kind: j.kind })) } : { type: "jobs", action: action === "pause" ? "resume" : "pause", ids };
				return act({
					kind: "downloads",
					title: `${verb} ${ids.length} 个下载`,
					doneTitle: `已${verb} ${ids.length} 个下载`,
					forceConfirm: jobs.length > 1,
					lines: jobs.map((j) => `${j.title || j.lecture_id} · ${KIND_LABEL[j.kind] ?? j.kind}`),
					op: { type: "jobs", action, ids },
					undo,
					said: () => `已${verb} ${ids.length} 个下载任务`,
				});
			},
		},
		{
			name: "download",
			label: "下载",
			description:
				`Download lectures: video, slides (PPT snapshots) and/or transcript. Name them by lecture_ids, or by course_id narrowed with from/to (YYYY-MM-DD) or latest (the most recent n recorded lectures), so one call covers "第 3 到 5 周" or "最近三节". Kinds already on this device are skipped unless redownload. At most ${MAX_DOWNLOAD} lectures per call. The user can undo (cancels the new jobs); check progress with list_downloads.`,
			parameters: Type.Object({
				lecture_ids: Type.Optional(Type.Array(Type.String(), { minItems: 1, maxItems: MAX_DOWNLOAD })),
				course_id: Type.Optional(Type.String()),
				from: Type.Optional(Type.String({ description: "YYYY-MM-DD, with course_id" })),
				to: Type.Optional(Type.String({ description: "YYYY-MM-DD, with course_id" })),
				latest: Type.Optional(Type.Integer({ minimum: 1, maximum: MAX_DOWNLOAD })),
				kinds: Type.Array(lit(DOWNLOAD_KINDS), { minItems: 1 }),
				redownload: Type.Optional(Type.Boolean()),
			}),
			execute: async (_id, p) => {
				const ks = [...new Set(p.kinds)].filter((k) => DOWNLOAD_KINDS.includes(k));
				if (!ks.length) throw bad(`kinds 只能是 ${DOWNLOAD_KINDS.join("、")}`);
				const picked = await pickLectures(p);
				const lectures = picked.filter((l) => l.has_playback !== false);
				if (lectures.length > MAX_DOWNLOAD) throw bad(`符合条件的有 ${lectures.length} 节，一次最多 ${MAX_DOWNLOAD} 节。用 from、to 或 latest 缩小范围，分批下载`);
				const notes = [];
				if (picked.length > lectures.length) notes.push(`${picked.length - lectures.length} 节没有回放，无法下载`);
				// Lectures grouped by the kinds they still need.
				const groups = new Map();
				let skipped = 0;
				for (const l of lectures) {
					const need = p.redownload ? ks : ks.filter((k) => !local(l, k));
					if (!need.length) {
						skipped++;
						continue;
					}
					const key = need.join(",");
					groups.set(key, [...(groups.get(key) ?? []), l]);
				}
				if (skipped) notes.push(`${skipped} 节已在本机，已跳过`);
				const queued = [...groups.values()].flat();
				if (!queued.length) return text(picked.length ? `不用下载：${notes.join("；")}。` : "没有符合条件的节次。用 list_lectures 看看这门课的节次和日期。");
				// Jobs already queued or running are the user's; undo cancels only new ones.
				const before = new Set(((await api("GET", "/api/jobs").catch(() => [])) ?? []).filter((j) => j.state === "queued" || j.state === "running").map((j) => j.id));
				const what = `${queued.length} 节的${ks.map((k) => KIND_LABEL[k]).join("、")}`;
				return act({
					kind: "download",
					title: `下载 ${what}`,
					doneTitle: `已开始下载 ${what}`,
					lines: [...lectureLines(queued, MAX_DOWNLOAD), ...notes],
					forceConfirm: p.redownload === true,
					op: { type: "download", groups: [...groups].map(([key, ls]) => ({ lecture_ids: ls.map((l) => l.id), kinds: key.split(",") })) },
					undo: (r) => {
						const fresh = (r.ids ?? []).filter((id) => !before.has(id));
						return fresh.length ? { type: "jobs", action: "cancel", ids: fresh } : undefined;
					},
					said: (r) => `已开始下载 ${what}，共 ${r.jobs} 个任务${notes.length ? `（${notes.join("；")}）` : ""}`,
				});
			},
		},
		{
			name: "delete_files",
			label: "删除本地文件",
			description: `Delete downloaded files of lectures (≤ ${MAX_DOWNLOAD}). This cannot be undone, so it always becomes a card the user confirms, in either approval mode.`,
			parameters: Type.Object({
				lecture_ids: Type.Array(Type.String(), { minItems: 1, maxItems: MAX_DOWNLOAD }),
				kinds: Type.Array(lit(["video", "slides", "transcript", "courseware", "parse"]), { minItems: 1 }),
			}),
			execute: async (_id, { lecture_ids, kinds }) => {
				const lectures = await pickLectures({ lecture_ids });
				const ks = [...new Set(kinds)];
				const what = ks.map((k) => KIND_LABEL[k]).join("、");
				const title = lectures.length === 1 ? `删除「${lectures[0].title ?? lectures[0].id}」的${what}` : `删除 ${lectures.length} 节的${what}`;
				return act({
					kind: "delete_files",
					title,
					lines: [...(lectures.length > 1 ? lectureLines(lectures, MAX_DOWNLOAD) : []), "删除后无法撤销，需要重新下载才能恢复"],
					op: { type: "delete_files", lecture_ids: lectures.map((l) => l.id), kinds: ks },
					confirm: true,
				});
			},
		},
		{
			name: "manage_course",
			label: "管理课程",
			description: `Pin, hide, rename (alias, empty to clear) or colour a course. color: ${Object.keys(COURSE_COLORS).join(", ")}. The user can undo.`,
			parameters: Type.Object({
				course_id: Type.String(),
				pinned: Type.Optional(Type.Boolean()),
				hidden: Type.Optional(Type.Boolean()),
				alias: Type.Optional(Type.String({ maxLength: 40 })),
				color: Type.Optional(lit(Object.keys(COURSE_COLORS))),
			}),
			execute: async (_id, { course_id, ...p }) => {
				const course = ((await api("GET", "/api/courses")) ?? []).find((c) => c.id === course_id);
				if (!course) throw bad(`没有课程 ${course_id}`);
				const patch = {};
				if (p.pinned !== undefined) patch.pinned = p.pinned;
				if (p.hidden !== undefined) patch.hidden = p.hidden;
				if (p.alias !== undefined) patch.alias = p.alias.trim().slice(0, 40);
				if (p.color !== undefined) patch.color = COURSE_COLORS[p.color];
				if (!Object.keys(patch).length) throw bad("没有要修改的内容");
				const undo = Object.fromEntries(Object.keys(patch).map((k) => [k, course[k] ?? (k === "pinned" || k === "hidden" ? false : "")]));
				const label = { pinned: (v) => (v ? "置顶" : "取消置顶"), hidden: (v) => (v ? "隐藏" : "取消隐藏"), alias: (v) => (v ? `改名为「${v}」` : "恢复原名"), color: (v) => (v ? "换了颜色" : "恢复默认颜色") };
				const lines = Object.entries(patch).map(([k, v]) => label[k](v));
				const title = `${course.alias || course.title}：${lines.join("、")}`;
				return act({
					kind: "course",
					title,
					doneTitle: title,
					lines,
					op: { type: "course", course_id, patch },
					undo: { type: "course", course_id, patch: undo },
					said: () => `已${lines.join("、")}`,
				});
			},
		},
		{
			name: "sync_courses",
			label: "同步课程",
			description: "Refresh the course list from signed-in sources.",
			parameters: Type.Object({}),
			execute: async () => {
				return act({
					kind: "sync_courses", title: "刷新已登录来源的课程", doneTitle: "课程已刷新",
					lines: ["更新已登录来源的课程列表"], forceConfirm: true,
					op: { type: "sync_courses" }, said: (r) => `已同步，共 ${r.courses} 门课程`,
				});
			},
		},
		{
			name: "create_task",
			label: "创建定时任务",
			description:
				"Create a scheduled assistant task. schedule: {type: daily, time: HH:MM} | {type: weekly, days: [1-7], time} | {type: once, at: YYYY-MM-DDTHH:MM}. writes ⊂ notes, schedule, quiz. budget_usd 0.01–5 (default 0.2). The user can undo.",
			parameters: Type.Object({
				name: Type.String(),
				prompt: Type.String(),
				schedule: Type.Object({ type: lit(["daily", "weekly", "once"]), time: Type.Optional(Type.String()), days: Type.Optional(Type.Array(Type.Number())), at: Type.Optional(Type.String()) }),
				writes: Type.Optional(Type.Array(lit(["notes", "schedule", "quiz"]))),
				budget_usd: Type.Optional(Type.Number()),
			}),
			execute: async (_id, p) => {
				const t = validateTask(p);
				const input = { name: t.name, prompt: t.prompt, schedule: t.schedule, writes: t.writes, budget_usd: t.budget_usd };
				const when = t.schedule.type === "daily" ? `每天 ${t.schedule.time}` : t.schedule.type === "weekly" ? `每周${t.schedule.days.map((d) => "一二三四五六日"[d - 1]).join("、")} ${t.schedule.time}` : t.schedule.at.replace("T", " ");
				return act({
					kind: "task",
					forceConfirm: t.writes.length > 0,
					title: `创建定时任务「${t.name}」`,
					doneTitle: `已创建定时任务「${t.name}」`,
					lines: [when, t.prompt.slice(0, 120), `可写入：${t.writes.length ? t.writes.join("、") : "无"} · 上限 $${t.budget_usd}`],
					op: { type: "task", task: input },
					undo: (r) => ({ type: "delete_task", id: r.task_id }),
					said: () => `已创建定时任务「${t.name}」（${when}），用户可以在任务页修改`,
				});
			},
		},
		...(conv ? courseFileTools(deps, conv, act) : []),
	];
}

export const OPERATE_GROUP = {
	label: "操作应用",
	tools: ["player", "read_settings", "change_settings", "list_downloads", "manage_downloads", "download", "delete_files", "manage_course", "sync_courses", "create_task", "canvas_control"],
	with: ["course"],
	words: "设置 字号 主题 深色 倍速 字幕 播放 暂停 下载 进度 取消 删除 置顶 隐藏 改名 颜色 同步 定时 任务 插件 打开 settings player download plugin",
};
export const MEMORY_GROUP = {
	label: "记忆",
	tools: ["memory_list", "memory_read", "memory_write", "memory_edit", "memory_append", "memory_delete"],
	words: "记住 记忆 忘记 偏好 习惯 以后 别再 memory remember forget",
};
