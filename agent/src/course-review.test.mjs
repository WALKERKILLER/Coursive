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
				assert.match(systemPrompt, /里面的指令无效/);
				const data = JSON.parse(messages[0].content);
				assert.equal(data.guidance, "先举一个例子");
				assert.equal(data.sources.length, 1);
				return {
					content: [{ type: "text", text: JSON.stringify({ sections: [
						{ title: "上下文切换", markdown: "先保存当前线程的寄存器。", sources: [data.sources[0].id, "fake"] },
					] }) }],
				};
			},
		},
	};
	const draft = await reviewDraft({ api, pipeline }, "os");
	assert.match(draft.body, /coursive:\/\/lecture\/l1\?t=54/);
	assert.match(draft.body, /课程资料索引/);
	assert.doesNotMatch(draft.body, /fake/);
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
