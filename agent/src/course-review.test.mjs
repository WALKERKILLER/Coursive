import assert from "node:assert/strict";
import { test } from "node:test";
import { courseReviewId, reviewDraft, reviewLinks } from "./course-review.mjs";

test("course handout treats real class evidence as data and links only verified sources", async () => {
	const calls = [];
	const api = async (_method, path) => {
		calls.push(path);
		if (path.endsWith("/lectures")) return {
			course: { id: "os", title: "操作系统" },
			lectures: [{ id: "l1", course_id: "os", title: "进程" }, { id: "bad", course_id: "other" }],
		};
		if (path.startsWith("/api/notes/")) throw Object.assign(new Error("not found"), { status: 404 });
		if (path.endsWith("/transcript")) return [{ start_ms: 54000, text: "上下文切换需要保存寄存器。" }];
		return [];
	};
	const pipeline = {
		prefs: { reviewGuidance: "先举一个例子" },
		currentModel: () => ({ id: "m" }),
		runtime: {
			completeSimple: (_model, { systemPrompt, messages }) => {
				const data = JSON.parse(messages[0].content);
				const quote = "上下文切换需要保存寄存器。";
				let result;
				if (data.lectures) result={chapters:[{title:"上下文切换",query:"上下文切换 寄存器",lectures:["l1"]}]};
				else if (data.quotes) result={supported:[0]};
				else if (data.claims) result={supported:[0]};
				else if (data.query) result={refs:[{id:data.sources[0].id,quote}]};
				else {
					assert.equal(data.guidance,"先举一个例子");
					assert.equal(data.sources.length,1);
					result={paragraphs:[
						{markdown:"老师讲到保存当前线程的寄存器。",refs:[{id:data.sources[0].id,quote}]},
						{markdown:"老师强调月球调度。",refs:[{id:"fake",quote}]}
					]};
				}
				return {content:[{type:"text",text:JSON.stringify(result)}]};
			},
		},
	};
	const draft = await reviewDraft({ api, pipeline, reviewCache:(_key, produce)=>produce() }, "os");
	assert.match(draft.body, /coursive:\/\/lecture\/l1\?t=54/);
	const cachedDraft = await reviewDraft({
		api, pipeline, reviewCache: (key, produce) => key[3] === "chapter"
			? { cited: true, body: "原文\n\n课堂出处 · [课堂录音 · 54 秒](coursive://lecture/l1?t=98&start_ms=98000) · [课堂录音 · 54 秒](coursive://lecture/l1?t=59)" }
			: produce(),
	}, "os");
	assert.match(cachedDraft.body, /课堂录音 · 98 秒/);
	assert.match(cachedDraft.body, /课堂录音 · 59 秒/);
	assert.doesNotMatch(cachedDraft.body, /课堂录音 · 54 秒/);

	assert.match(draft.body, /课程资料索引/);
	assert.doesNotMatch(draft.body, /fake|月球/);
	assert.match(draft.body, /rev=[a-f0-9]{64}/);
	assert.ok(calls.every((path) => !path.includes("/bad/")));
	assert.match(reviewLinks({ note_id: "note", lecture_id: "l1", card_id: "card" }), /coursive:\/\/canvas\/l1\/card/);
});

test("retry reuses the existing note and canvas without calling the model", async () => {
	const api = async (_method, path) => path.endsWith("/notes/" + courseReviewId("os")) ? { course_id: "os", lecture_id: "initial" } : {
		course: { id: "os", title: "操作系统" }, lectures: [{ id: "new", course_id: "os" }],
	};
	const draft = await reviewDraft({ api, pipeline: {} }, "os");
	assert.equal(draft.existing, true);
	assert.equal(draft.lecture_id, "initial");
	assert.equal(draft.body, "");
});
