import { Type } from "typebox";

export const COURSE_TOOLS = [
	{
		name: "search_course_materials",
		description: "检索当前课程本地材料，最多12项；搜索摘要不能作为已读证据。",
		parameters: Type.Object({ query: Type.String({ maxLength: 500 }) }, { additionalProperties: false }),
	},
	{
		name: "read_course_source",
		description: "读取搜索候选的原文，最多12000字符；offset是该文字载体的rune偏移。只有返回的citation可用于引用。",
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
		this.evidence = new Map();
	}

	async execute(name, args, signal) {
		signal?.throwIfAborted();
		if (name === "search_course_materials") {
			const items = await this.call("POST", this.base + "/sources/search", { query: args.query });
			signal?.throwIfAborted();
			if (!Array.isArray(items) || items.length > 12) throw new Error("课程检索结果无效");
			for (const item of items) this.candidates.set(item.id, item);
			return { candidates: items };
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
		if (result.availability !== "readable" || !result.text) return result;
		if (result.id !== source.id || result.path !== source.path || result.offset !== offset ||
			result.end <= offset || result.end - offset > limit ||
			[...result.text].length !== result.end - offset || !result.fingerprint) {
			throw new Error("来源证据范围无效");
		}

		const number = this.evidence.size + 1;
		const id = "e" + number;
		const location = result.page ? "第" + result.page + "页"
			: result.time_start_ms ? Math.floor(result.time_start_ms / 1000) + "秒" : "文本摘录";
		const label = [result.lecture, result.label, location].filter(Boolean).join(" · ");
		this.evidence.set(id, { id, number, label, source: result });
		return { ...result, citation: "[" + number + "](coursive://citation/" + id + ")" };
	}

	citations(answer) {
		const used = new Set([...answer.matchAll(/\[[^\]\n]+\]\(coursive:\/\/citation\/(e\d+)\)/g)].map(m => m[1]));
		return [...this.evidence.values()].filter(e => used.has(e.id)).map(e => {
			// Persist an identity and range. Re-read the text on navigation instead
			// of duplicating up to 12000 characters per citation in every turn.
			const { text, snippet, ...source } = e.source;
			return { ...e, source };
		});
	}

	sanitize(answer) {
		return answer.replace(/\[([^\]\n]+)\]\(coursive:\/\/citation\/([^\s)]+)\)/g,
			(match, label, id) => this.evidence.has(id) ? match : "[" + label + "]");
	}
}
