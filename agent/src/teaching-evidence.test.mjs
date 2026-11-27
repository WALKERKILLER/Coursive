import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { digest, locateQuote, teachingCache } from "./teaching-evidence.mjs";
import { verifiedLookup } from "./teaching-lookup.mjs";

test("exact quotes retain their own interval, avoid ambiguous repeats and survive a restarted cache", async (t) => {
	const root = await mkdtemp(join(tmpdir(), "coursive-evidence-"));
	t.after(() => rm(root, { recursive: true, force: true }));
	const quote = "保存寄存器";
	const item = {
		id: "A",
		kind: "transcript",
		link: "coursive://lecture/l?t=0",
		text: "开头\n" + quote,
		fingerprint: digest("source"),
		segments: [
			{ at: 0, seconds: 2, startMs: 2412, endMs: 3510 },
			{ at: 3, seconds: 5, startMs: 5310, endMs: 6850 },
		],
	};
	const url = new URL(locateQuote(item, quote));
	assert.equal(url.searchParams.get("start_ms"), "5310");
	assert.equal(url.searchParams.get("end_ms"), "6850");
	assert.equal(locateQuote({ ...item, text: quote + quote }, quote), null);
	let calls = 0;
	const model = {
		currentModel: async () => ({ provider: "fixture", id: "1" }),
		runtime: {
			completeSimple: async (_, ctx) => {
				calls++;
				const data = JSON.parse(ctx.messages[0].content);
				const result = data.sources
					? {
							refs: [
								{ id: "A", quote },
								{ id: "A", quote: "编造的位置" },
							],
						}
					: { supported: [0] };
				return { content: [{ type: "text", text: JSON.stringify(result) }] };
			},
		},
	};
	const run = (items) =>
		verifiedLookup(model, "保存什么", items, undefined, teachingCache(root));
	assert.equal((await run([item]))[0].text, quote);
	assert.equal((await run([item]))[0].position, 5);
	assert.equal(calls, 2);
	const updated = { ...item, fingerprint: digest("new") };
	await run([updated]);
	assert.equal(calls, 4);
});
