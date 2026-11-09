// Text helpers shared by the pilot tasks. Only ECMAScript built-ins are used,
// and none whose result depends on the engine (Intl, locale, Date, random).

const FILLERS = ["就是说", "那个", "然后", "嗯", "呃", "啊"];

/** Full-width ASCII to ASCII, ideographic space to space, runs of space to one. */
export function fold(text) {
	let out = "";
	for (const ch of text) {
		const code = ch.codePointAt(0);
		if (code >= 0xff01 && code <= 0xff5e) out += String.fromCodePoint(code - 0xfee0);
		else if (code === 0x3000 || ch === "\t" || ch === "\n" || ch === "\r") out += " ";
		else out += ch;
	}
	return out.replace(/ {2,}/g, " ").trim();
}

/** Drops leading fillers (嗯，那个 …) an ASR transcript starts sentences with. */
export function dropFillers(text) {
	let t = text;
	for (let changed = true; changed; ) {
		changed = false;
		for (const f of FILLERS) {
			if (t.startsWith(f)) {
				t = t.slice(f.length).replace(/^[，,、 ]+/, "");
				changed = true;
			}
		}
	}
	return t;
}

const WORD = /[A-Za-z0-9]$/;
const WORD_START = /^[A-Za-z0-9]/;

/** Joins two pieces with a space only between two Latin words or numbers. */
export function join(a, b) {
	if (!a) return b;
	return WORD.test(a) && WORD_START.test(b) ? `${a} ${b}` : a + b;
}

export const SENTENCE_END = /[。！？.!?]$/;
