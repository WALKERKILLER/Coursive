// The tool-call pass of moejs-bench, timed from the assistant host: a Pi tool
// running the pilot inline (arguments and result as JSON, as a tool call
// carries them), a Node process plugin over JSON lines, and the daemon's
// loopback tool route backed by Go native code or by MoeJS.
import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";
import { createInterface } from "node:readline";
import { fileURLToPath, pathToFileURL } from "node:url";
import { measure } from "./measure.mjs";

const [planPath, base] = process.argv.slice(2);
const plan = JSON.parse(readFileSync(planPath, "utf8"));
const pilot = await import(pathToFileURL(`${plan.pilot}/index.js`).href);
const rows = [];
const add = async (path, call) => {
	for (const t of plan.tasks) rows.push({ path, ...(await measure(t.name, plan.n, () => call(t))) });
};

await add("pi-inline", (t) => JSON.parse(JSON.stringify(pilot[t.action](JSON.parse(JSON.stringify(t.input))))));

const worker = spawn(process.execPath, [fileURLToPath(new URL("rpc-worker.mjs", import.meta.url)), plan.pilot], { stdio: ["pipe", "pipe", "inherit"] });
const lines = createInterface({ input: worker.stdout })[Symbol.asyncIterator]();
await lines.next();
let id = 0;
await add("process-rpc", async (t) => {
	worker.stdin.write(`${JSON.stringify({ id: ++id, action: t.action, input: t.input })}\n`);
	const reply = JSON.parse((await lines.next()).value);
	if (reply.error || reply.id !== id) throw new Error(reply.error ?? "out of order");
	return reply.output;
});
worker.kill();

for (const backend of ["go-native", "moejs"]) {
	await add(`daemon route → ${backend}`, async (t) => {
		const res = await fetch(`${base}/tools/${backend}`, {
			method: "POST",
			headers: { authorization: `Bearer ${process.env.BENCH_TOKEN}`, "content-type": "application/json" },
			body: JSON.stringify({ action: t.action, input: t.input }),
		});
		if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
		return (await res.json()).output;
	});
}
process.stdout.write(`${JSON.stringify(rows)}\n`);
