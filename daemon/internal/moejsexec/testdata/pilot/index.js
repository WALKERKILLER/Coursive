// Pilot tasks of the MoeJS executor (decision 0036). The same module runs in
// Node for the parity check and the benchmark, so it uses ES modules and
// ECMAScript built-ins only.
import { SENTENCE_END, dropFillers, fold, join } from "./text.js";

/**
 * Subtitle normalization: folds width and space, drops ASR fillers and
 * repeated cues, and merges cues into paragraphs at a pause, a length cap or a
 * finished sentence.
 * input: {cues: [{start, end, text}], gapMs?, maxChars?}
 */
export function normalizeSubtitles(input) {
	const gapMs = input.gapMs ?? 1500;
	const maxChars = input.maxChars ?? 120;
	const cues = [];
	let repeated = 0;
	for (const cue of input.cues) {
		const text = dropFillers(fold(cue.text));
		if (!text) continue;
		const last = cues[cues.length - 1];
		if (last && last.text === text) {
			last.end = cue.end;
			repeated++;
			continue;
		}
		cues.push({ start: cue.start, end: cue.end, text });
	}
	const paragraphs = [];
	for (const cue of cues) {
		const p = paragraphs[paragraphs.length - 1];
		const open = p && cue.start - p.end <= gapMs && p.text.length + cue.text.length <= maxChars && !(SENTENCE_END.test(p.text) && p.text.length >= 40);
		if (open) {
			p.text = join(p.text, cue.text);
			p.end = cue.end;
			p.cues++;
		} else {
			paragraphs.push({ start: cue.start, end: cue.end, text: cue.text, cues: 1 });
		}
	}
	return { paragraphs, stats: { cues: input.cues.length, kept: cues.length, repeated, paragraphs: paragraphs.length } };
}

/**
 * Pure JSON transform: notes grouped by normalized tag, newest first, with the
 * untagged ones listed apart.
 * input: {notes: [{id, title, tags, updated}]}
 */
export function groupNotes(input) {
	const groups = new Map();
	const untagged = [];
	const order = (a, b) => (b.updated === a.updated ? (a.id < b.id ? -1 : a.id > b.id ? 1 : 0) : b.updated - a.updated);
	for (const note of [...input.notes].sort(order)) {
		const tags = [...new Set((note.tags ?? []).map((t) => fold(String(t)).toLowerCase()).filter(Boolean))];
		if (!tags.length) untagged.push(note.id);
		for (const tag of tags) {
			if (!groups.has(tag)) groups.set(tag, []);
			groups.get(tag).push({ id: note.id, title: fold(note.title ?? "") });
		}
	}
	const byTag = [...groups.entries()].map(([tag, notes]) => ({ tag, count: notes.length, notes }));
	byTag.sort((a, b) => b.count - a.count || (a.tag < b.tag ? -1 : 1));
	return { byTag, untagged };
}

/**
 * Event hook: decides what a study event triggers.
 * input: {type, lectureId, payload}
 */
export function onEvent(input) {
	const p = input.payload ?? {};
	switch (input.type) {
		case "lecture.progress":
			if (p.percent >= 90) return { action: "tag", tags: ["watched"], reason: `看完 ${Math.round(p.percent)}%` };
			if (p.percent >= 50 && p.rewinds >= 3) return { action: "remind", tags: ["review"], reason: "反复回看，建议复习" };
			return { action: "ignore" };
		case "note.saved": {
			const todos = (p.text ?? "").split("\n").filter((l) => /^\s*(TODO|待办)[:：]/.test(l)).length;
			return todos ? { action: "remind", tags: ["todo"], reason: `${todos} 条待办` } : { action: "ignore" };
		}
		default:
			return { action: "ignore" };
	}
}

/** Never returns: the benchmark's cancellation probe. */
export function spin() {
	for (;;) {}
}
