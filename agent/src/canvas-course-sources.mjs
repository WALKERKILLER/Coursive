import { Type } from "typebox";
import { documentPassages, neutralize } from "./canvas-agent.mjs";

export const COURSE_TOOLS = [
	{
		name: "search_course_materials",
		description: "检索当前课程本地材料，最多12项；搜索摘要不能作为已读证据。",
		parameters: Type.Object({ query: Type.String({ maxLength: 500 }) }, { additionalProperties: false }),
	},
	{
		name: "read_course_source",
		description: "读取搜索候选的原文，最多12000字符；offset是该文字载体的rune偏移。只有返回段落的 evidence_id 可用于引用。",
		parameters: Type.Object({
			source_id: Type.String({ maxLength: 100 }),
			offset: Type.Optional(Type.Integer({ minimum: 0 })),
			limit: Type.Optional(Type.Integer({ minimum: 1, maximum: 12000 })),
		}, { additionalProperties: false }),
	},
];

// Candidate IDs are scoped to this run. A model cannot nominate another course,
// an arbitrary filesystem path, or reuse evidence from a previous answer.
export class CanvasCourseSources {
	constructor(call, base) {
		this.call = call;
		this.base = base;
		this.candidates = new Map();
	}

	async execute(name, args, signal, scope) {
		signal?.throwIfAborted();
		if (name === "search_course_materials") {
			const items = await this.call("POST", this.base + "/sources/search", { query: args.query });
			signal?.throwIfAborted();
			if (!Array.isArray(items) || items.length > 12) throw new Error("课程检索结果无效");
			for (const item of items) this.candidates.set(item.id, item);
			return { candidates: items.map(({ text, ...item }) => ({ ...item, snippet: neutralize(item.snippet) })) };
		}

		const source = this.candidates.get(args.source_id);
		if (!source) throw new Error("来源未经本轮搜索授权");
		const offset = args.offset ?? source.offset ?? 0;
		const limit = args.limit ?? 12000;
		if (!Number.isInteger(offset) || offset < 0 || !Number.isInteger(limit) || limit < 1 || limit > 12000) {
			throw new Error("来源读取范围无效");
		}
		const result = await this.call("POST", this.base + "/sources/read", { source, offset, limit });
		signal?.throwIfAborted();
		if (result.availability !== "readable" || !result.text) return { source_id: source.id, availability: result.availability, text: "" };
		if (result.id !== source.id || result.path !== source.path || result.offset !== offset ||
			result.end <= offset || result.end - offset > limit ||
			[...result.text].length !== result.end - offset || !result.fingerprint) {
			throw new Error("来源证据范围无效");
		}

		const { text, snippet, next_offset, ...identity } = result;
		const timed = result.time_end_ms > (result.time_start_ms ?? 0);
		const location = result.page ? "第" + result.page + "页"
			: timed ? Math.floor((result.time_start_ms ?? 0) / 1000) + "–" + Math.ceil(result.time_end_ms / 1000) + "秒" : "";
		const label = [result.lecture, result.label, location].filter(Boolean).join(" · ");
		const id = "course-" + result.id;
		scope.texts.set(id, (scope.texts.get(id) ?? "") + "\n" + text);
		scope.seen.add(id);
		// Each passage keeps its own rune range, so a click re-reads exactly the
		// cited sentence and the daemon can recover its page or transcript time.
		const passages = documentPassages(text).map(p => {
			const start = offset + [...text.slice(0, p.start)].length, end = start + [...p.text].length;
			const anchor = {
				id: result.id, workspace_id: "", object_id: "", kind: timed ? "video" : result.page ? "pdf" : "text",
				source_lecture_id: result.lecture_id, original_path: result.path, page: result.page || undefined,
				start, end, time_start_ms: timed ? result.time_start_ms ?? 0 : undefined, time_end_ms: timed ? result.time_end_ms : undefined,
				fingerprint: result.fingerprint, quote: p.text,
			};
			const course_source = { ...identity, offset: start, end };
			return { evidence_id: scope.issue({ id, label }, timed ? "timestamped_transcript" : "exact_text", anchor, { course_source }), text: neutralize(p.text) };
		});
		return {
			source_id: source.id, label, kind: result.kind, lecture: result.lecture, page: result.page || undefined,
			time_start_ms: timed ? result.time_start_ms ?? 0 : undefined, time_end_ms: timed ? result.time_end_ms : undefined,
			next_offset, passages,
		};
	}
}
