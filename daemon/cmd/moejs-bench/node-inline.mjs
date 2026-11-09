// The Node inline backend of moejs-bench: the pilot module imported into a
// running Node process, as a Pi extension runs inside the assistant host.
import { readFileSync } from "node:fs";
import { pathToFileURL } from "node:url";
import { measure } from "./measure.mjs";

const plan = JSON.parse(readFileSync(process.argv[2], "utf8"));
const cpu0 = process.cpuUsage();
const rss0 = process.memoryUsage.rss();
const t0 = performance.now();
const pilot = await import(pathToFileURL(`${plan.pilot}/index.js`).href);
const report = { backend: "node-inline", startup_ms: performance.now() - t0, tasks: [] };
for (const t of plan.tasks) report.tasks.push(await measure(t.name, plan.n, () => JSON.stringify(pilot[t.action](t.input))));
const cpu = process.cpuUsage(cpu0);
report.cpu_ms = (cpu.user + cpu.system) / 1000;
report.rss_delta_mb = (process.memoryUsage.rss() - rss0) / 1048576;
process.stdout.write(`${JSON.stringify(report)}\n`);
