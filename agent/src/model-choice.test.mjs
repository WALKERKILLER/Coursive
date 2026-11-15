import test from "node:test";
import assert from "node:assert/strict";
import { ModelPipeline } from "./models.mjs";

test("explicit selection never sends a prompt to the first available provider", async () => {
	const pipeline = new ModelPipeline();
	const deepseek = { provider: "deepseek", id: "chat" };
	const openai = { provider: "openai", id: "gpt" };
	let available = [deepseek, openai];
	pipeline.runtime = { getAvailable: async () => available };
	assert.equal(await pipeline.currentModel(), deepseek);
	pipeline.prefs = { provider: "openai", model: "gpt" };
	assert.equal(await pipeline.currentModel(), openai);
	available = [deepseek];
	assert.equal(await pipeline.currentModel(), undefined);
});
