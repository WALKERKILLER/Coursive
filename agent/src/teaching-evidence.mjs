import { createHash, randomUUID } from "node:crypto";
import { mkdir, readFile, rename, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";

export const digest = (value) =>
	createHash("sha256").update(value).digest("hex");

// These entries are disposable derivatives. The source library and saved notes remain authoritative.
export function teachingCache(directory) {
	const pending = new Map();
	return async function cached(parts, produce) {
		const key = digest(JSON.stringify(parts));
		if (pending.has(key)) return pending.get(key);
		const file = join(directory, key + ".json");
		async function load() {
			try {
				return JSON.parse(await readFile(file, "utf8"));
			} catch (e) {
				if (e.code !== "ENOENT" && !(e instanceof SyntaxError)) throw e;
			}
			const value = await produce();
			if (value === undefined || value === null) return value;
			await mkdir(directory, { recursive: true, mode: 0o700 });
			const tmp = file + "." + randomUUID();
			try {
				await writeFile(tmp, JSON.stringify(value), { mode: 0o600 });
				await rename(tmp, file);
			} finally {
				await rm(tmp, { force: true });
			}
			return value;
		}
		pending.set(
			key,
			load().finally(() => pending.delete(key)),
		);
		return pending.get(key);
	};
}

const segmenter = new Intl.Segmenter("zh", { granularity: "word" });
const stop = new Set(
	"老师 课上 课堂 课程 学生 我们 你们 这里 为什么 如何 怎么 提到 什么 一个 这个 哪个 多少 哪些 时候 需要 他们 就是 是否 示例 例子 可以 怎样 哪儿 地方 解释 分别 讲到".split(
		" ",
	),
);
function words(text) {
	return [...segmenter.segment(text.normalize("NFKC").toLowerCase())]
		.filter(
			(s) => s.isWordLike && s.segment.length >= 2 && !stop.has(s.segment),
		)
		.map((s) => s.segment);
}

export function transcriptIndex(windows) {
	const df = new Map();
	const docs = windows.map((item) => {
		const rows = item.segments.map((row, i) => {
			const text = item.text.slice(row.at, item.segments[i + 1]?.at).trim();
			return { row, text, words: new Set(words(text)) };
		});
		const terms = new Set(rows.flatMap((r) => [...r.words]));
		for (const term of terms) df.set(term, (df.get(term) ?? 0) + 1);
		return { item, rows };
	});
	return function search(query) {
		const terms = [...new Set(words(query))];
		if (!terms.length) return [];
		const weight = (term) =>
			Math.log(1 + (1 + docs.length) / (1 + (df.get(term) ?? 0)));
		const total = terms.reduce((v, t) => v + weight(t), 0);
		const hits = [];
		for (const { item, rows } of docs) {
			let best;
			for (let i = 0; i < rows.length; i++) {
				const context = rows.slice(i, i + 5);
				const matched = terms.filter((t) =>
					context.some(
						(r) => r.words.has(t) || r.text.toLowerCase().includes(t),
					),
				);
				const score = matched.reduce((v, t) => v + weight(t), 0);
				if (
					!score ||
					score < total * 0.24 ||
					(terms.length > 2 && matched.length < 2)
				)
					continue;
				const first = context.find((r) =>
					matched.some(
						(t) => r.words.has(t) || r.text.toLowerCase().includes(t),
					),
				);
				if (!best || score > best.score)
					best = { item, seconds: first.row.seconds, score };
			}
			if (best) hits.push(best);
		}
		return hits.sort((a, b) => b.score - a.score).slice(0, 3);
	};
}

export function locateQuote(item, quote) {
	if (
		typeof quote !== "string" ||
		quote.trim().length < 2 ||
		quote.length > 400 ||
		!item.text.includes(quote)
	)
		return null;
	const q = quote.trim(),
		at = item.text.indexOf(q);
	if (item.text.indexOf(q, at + q.length) >= 0) return null;
	const url = new URL(item.link);
	if (item.kind === "transcript") {
		const start = item.segments?.findLast((s) => s.at <= at);
		if (!start) return null;
		url.searchParams.set("t", String(start.seconds));
		const last = item.segments.findLast((s) => s.at < at + q.length);
		if (Number.isFinite(start.startMs))
			url.searchParams.set("start_ms", String(start.startMs));
		if (Number.isFinite(last.endMs))
			url.searchParams.set("end_ms", String(last.endMs));
	}
	if (item.imageHash) url.searchParams.set("image_rev", item.imageHash);
	url.searchParams.set("q", q);
	url.searchParams.set("v", "1");
	url.searchParams.set("rev", item.fingerprint || digest(item.text));
	url.searchParams.set("ref", digest(q));
	return url.toString();
}
