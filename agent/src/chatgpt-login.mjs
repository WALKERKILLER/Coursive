// A sign-in belongs to this installation. The SDK handles PKCE, loopback
// callback validation, credential storage and refresh. Only its URL reaches UI.
import { randomUUID } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";

export function chatGPTLogin(pipeline, directory) {
	let active;
	let state = { status: "idle" };
	async function start() {
		if (active) return active;
		const controller = new AbortController();
		const timer = setTimeout(() => controller.abort(), 5 * 60_000);
		state = { status: "waiting" };
		let opened, rejected;
		let exchangeStarted = false;
		active = new Promise((resolve, reject) => { opened = resolve; rejected = reject; });
		const started = active;
		(async () => {
			const file = join(directory, "oauth-device-id");
			let id;
			try { id = (await readFile(file, "utf8")).trim(); }
			catch (error) {
				if (error.code !== "ENOENT") throw error;
				id = randomUUID();
				await writeFile(file, id, { mode: 0o600, flag: "wx" });
			}
			await pipeline.runtime.login("openai", "oauth", {
				signal: controller.signal,
				notify(event) {
					if (event.type === "progress" && event.message?.startsWith("Exchanging")) exchangeStarted = true;
					if (event.type === "auth_url") {
						const url = new URL(event.url);
						if (url.origin !== "https://auth.openai.com") throw new Error("未知的登录地址");
						opened({ url: url.href });
					}
				},
				prompt({signal}) {
					return new Promise((_, reject) => {
						if (signal.aborted) reject(new Error("登录已结束"));
						else signal.addEventListener("abort", () => reject(new Error("登录已结束")), { once: true });
					});
				}
			}, { agentName: "Coursive", getDeviceId: () => id });
			const available = (await pipeline.runtime.getAvailable()).filter((model) => model.provider === "openai");
			const model = available.find((model) => model.id === pipeline.prefs.model) ?? available[0];
			if (!model) throw new Error("登录后没有可用的 OpenAI 模型");
			await pipeline.setPrefs({ provider: model.provider, model: model.id });
			state = { status: "done" };
			opened(state);
		})().catch(() => {
			state = { status: "error", phase: exchangeStarted ? "token_exchange" : "authorization", error: "登录未完成，请确认网络可访问 OpenAI 后重试。" };
			rejected(new Error(state.error));
		}).finally(() => { clearTimeout(timer); active = null; });
		return started;
	}
	return { start, status: () => state };
}
