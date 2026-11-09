#!/usr/bin/env node
// plugin-sim.mjs — run a plugin package in a simulated host before publishing
// it. It validates the manifest, installs the package into a real PluginHost
// over a temporary data directory with a fake daemon, checks that every
// declared action has a handler, and runs the scripted cases in
// `sim/cases.json`: bridge actions (with the view state threaded between them
// the way the daemon's action bridge does), writes as proposals or approved,
// and agent tools through Pi's real loader. `--pack` then builds the npm
// tarball and checks that the runtime entry is in it.
//
// Guide: docs/plugin-v3-guide.md. Usage:
//
//   node agent/scripts/plugin-sim.mjs <plugin-dir> [options]
//
//   --cases <file>  cases to run instead of <plugin-dir>/sim/cases.json
//   --pack <dir>    after the cases pass, write the npm tarball into <dir>
//   --json          machine-readable report on stdout, for CI
//   -h, --help      show this usage
//
// Exit 0 when everything passes, 1 on a problem or a failed case, 2 on a usage
// or unreadable-input error.
import { execFile } from "node:child_process";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { isAbsolute, join, resolve } from "node:path";
import process from "node:process";
import { pathToFileURL } from "node:url";
import { promisify } from "node:util";
import { DefaultResourceLoader } from "@earendil-works/pi-coding-agent";
import { PluginHost } from "../src/plugins.mjs";
import { UsageError, validate } from "./validate-plugin.mjs";

const USAGE = `Usage: node agent/scripts/plugin-sim.mjs <plugin-dir> [options]

  --cases <file>  cases to run instead of <plugin-dir>/sim/cases.json
  --pack <dir>    after the cases pass, write the npm tarball into <dir>
  --json          machine-readable report on stdout, for CI
  -h, --help      show this usage`;

export function parseArgs(argv) {
	const opts = { target: "", cases: "", pack: "", json: false, help: false };
	for (let i = 0; i < argv.length; i++) {
		const a = argv[i];
		if (a === "-h" || a === "--help") opts.help = true;
		else if (a === "--json") opts.json = true;
		else if (a === "--cases" || a === "--pack") {
			if (!argv[i + 1]) throw new UsageError(`${a} needs a value`);
			opts[a.slice(2)] = argv[++i];
		} else if (a.startsWith("-")) throw new UsageError(`unknown option ${a}`);
		else if (opts.target) throw new UsageError("only one plugin directory");
		else opts.target = a;
	}
	if (!opts.target && !opts.help) throw new UsageError("missing plugin directory");
	return opts;
}

/** Whether `actual` holds everything `expected` names (arrays match by length and position). */
export function matches(actual, expected) {
	if (expected === null || typeof expected !== "object") return Object.is(actual, expected);
	if (Array.isArray(expected)) return Array.isArray(actual) && actual.length === expected.length && expected.every((e, i) => matches(actual[i], e));
	return actual !== null && typeof actual === "object" && Object.entries(expected).every(([k, v]) => matches(actual[k], v));
}

/**
 * Validates, installs and exercises one package. Returns
 * `{ok, plugin, problems, cases: [{name, ok, error?, result?}], pack?}`.
 */
export async function simulate(target, { cases: casesPath = "", pack = "" } = {}) {
	const dir = resolve(target);
	const report = await validate(dir, { files: true });
	const out = { ok: false, plugin: report.plugin, problems: report.problems.map((p) => `${p.path} [${p.code}] ${p.message}`), cases: [] };
	if (!report.ok) return out;

	const script = await readCases(casesPath ? resolve(casesPath) : join(dir, "sim", "cases.json"));
	const root = await mkdtemp(join(tmpdir(), "coursive-plugin-sim-"));
	const calls = [];
	try {
		await mkdir(join(root, "bundled"), { recursive: true });
		await writeFile(join(root, "bundled", "catalog.json"), JSON.stringify({ name: "sim", plugins: [] }));
		const host = new PluginHost({
			dataDir: join(root, "data"),
			libraryRoot: join(root, "lib"),
			bundledDir: join(root, "bundled"),
			daemon: async (method, path, body) => {
				calls.push({ call: `${method} ${path}`, body });
				const answer = script.daemon?.[`${method} ${path}`];
				return answer === undefined ? {} : structuredClone(answer);
			},
		});
		await host.init();
		await host.setAllowThirdParty(true);
		let view;
		try {
			view = await host.confirm((await host.preview(`path:${dir}`)).token);
		} catch (e) {
			out.problems.push(`install: ${e.message}`);
			return out;
		}
		const id = view.id;
		const rec = host.loaded.get(id);
		if (view.state !== "enabled" || rec?.error) out.problems.push(`load: ${rec?.error ?? view.error ?? view.state}`);
		for (const a of view.contributes?.actions ?? []) {
			if (!rec?.actions.has(a.action)) out.problems.push(`contributes.actions: 「${a.id}」 names action ${a.action}, which activate() never registers`);
		}
		if (out.problems.length) return out;

		const tools = await loadTools(host, root);
		const states = {};
		let proposals = 0;
		for (const [i, c] of (script.cases ?? []).entries()) {
			const name = c.name ?? `case ${i + 1}`;
			const before = calls.length;
			try {
				const result = await runCase(host, id, c, { tools, states, proposal: () => `sim-proposal-${++proposals}` });
				const made = calls.slice(before).map((x) => x.call);
				if (c.expectError !== undefined) throw new Error(`expected an error matching ${c.expectError}, got ${JSON.stringify(result)}`);
				if (c.expect !== undefined && !matches(result, c.expect)) throw new Error(`result ${JSON.stringify(result)} does not match ${JSON.stringify(c.expect)}`);
				if (c.calls !== undefined && !matches(made, c.calls)) throw new Error(`daemon calls ${JSON.stringify(made)}, expected ${JSON.stringify(c.calls)}`);
				out.cases.push({ name, ok: true, result });
			} catch (e) {
				const expected = c.expectError !== undefined && new RegExp(c.expectError).test(e.message) && !e.message.startsWith("expected an error");
				out.cases.push(expected ? { name, ok: true, error: e.message } : { name, ok: false, error: e.message });
			}
		}
		await host.close();
	} finally {
		await rm(root, { recursive: true, force: true });
	}
	out.ok = out.problems.length === 0 && out.cases.every((c) => c.ok);
	if (out.ok && pack) out.pack = await packPackage(dir, pack, report);
	if (out.pack?.error) {
		out.problems.push(`pack: ${out.pack.error}`);
		out.ok = false;
	}
	return out;
}

async function readCases(path) {
	try {
		return JSON.parse(await readFile(path, "utf8"));
	} catch (e) {
		if (e.code === "ENOENT") return { cases: [] };
		throw new Error(`cannot read ${path}: ${e.message}`);
	}
}

/** The plugin's Pi tools as the assistant loads them. */
async function loadTools(host, root) {
	const loader = new DefaultResourceLoader({ cwd: root, agentDir: root, noExtensions: true, noSkills: true, noPromptTemplates: true, noThemes: true, noContextFiles: true, extensionFactories: host.extensionFactories() });
	await loader.reload();
	const tools = new Map();
	for (const ext of loader.getExtensions().extensions) for (const [name, t] of ext.tools) tools.set(name, t.definition);
	return tools;
}

/**
 * One case: `{action, view?, args?, state?, approve?}` runs a bridge action and
 * merges a state patch into the simulated view state; a write without
 * `approve` answers `{proposal}` as the bridge would. `{tool, params}` runs an
 * agent tool.
 */
async function runCase(host, id, c, { tools, states, proposal }) {
	if (c.tool) {
		const tool = tools.get(`${id}__${c.tool}`);
		if (!tool) throw new Error(`no tool ${c.tool}; registered: ${[...tools.keys()].join(", ") || "none"}`);
		const r = await tool.execute("sim", c.params ?? {}, undefined, undefined, { sessionManager: { getSessionId: () => "sim" } });
		return { text: (r.content ?? []).map((x) => x.text ?? "").join(""), details: r.details };
	}
	const view = c.view ?? "";
	const declared = host.loaded.get(id).manifest.contributes.actions.find((a) => a.id === c.action);
	if (declared?.write && !c.approve) return { proposal: { actionId: c.action, args: c.args ?? {}, summary: declared.title } };
	const state = c.state ?? states[view] ?? {};
	const r = await host.bridgeAction(id, c.action, { args: c.args ?? {}, state, viewId: view, ...(c.approve ? { approved: proposal() } : {}) });
	if (r.statePatch) {
		const next = { ...state };
		for (const [k, v] of Object.entries(r.statePatch.patch ?? {})) v === null ? delete next[k] : (next[k] = v);
		states[r.statePatch.viewId || view] = next;
	}
	return r;
}

/** `npm pack` into dir; the runtime entry must be in the tarball. */
async function packPackage(dir, dest, report) {
	const outDir = isAbsolute(dest) ? dest : resolve(dest);
	await mkdir(outDir, { recursive: true });
	try {
		const { stdout } = await promisify(execFile)("npm", ["pack", "--json", "--pack-destination", outDir], { cwd: dir });
		// npm 10 and 11 print an array, npm 12 an object keyed by package name.
		const parsed = JSON.parse(stdout);
		const info = Array.isArray(parsed) ? parsed[0] : Object.values(parsed)[0];
		const files = new Set(info.files.map((f) => f.path));
		const pkg = JSON.parse(await readFile(join(dir, "package.json"), "utf8"));
		const entries = (pkg.coursive?.runtime ?? []).filter((r) => !r.optional).map((r) => String(r.entry).replace(/^\.\//, ""));
		const missing = entries.filter((e) => !files.has(e));
		if (missing.length) return { error: `${info.filename} is missing ${missing.join(", ")}; add them to "files"` };
		return { file: join(outDir, info.filename), files: [...files].sort(), plugin: report.plugin.id };
	} catch (e) {
		return { error: e.message };
	}
}

export function formatReport(r) {
	const lines = [`${r.plugin.id || "(no id)"}@${r.plugin.version || "(no version)"}: ${r.ok ? "OK" : "FAILED"}`];
	for (const p of r.problems) lines.push(`  problem: ${p}`);
	for (const c of r.cases) lines.push(`  ${c.ok ? "✓" : "✗"} ${c.name}${c.ok ? "" : ` — ${c.error}`}`);
	if (r.pack?.file) lines.push(`  packed ${r.pack.file}`);
	return lines.join("\n");
}

async function main(argv) {
	const opts = parseArgs(argv);
	if (opts.help) {
		console.log(USAGE);
		return 0;
	}
	const report = await simulate(opts.target, { cases: opts.cases, pack: opts.pack });
	console.log(opts.json ? JSON.stringify(report, null, 2) : formatReport(report));
	return report.ok ? 0 : 1;
}

const isMain = process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url;
if (isMain) {
	main(process.argv.slice(2)).then(
		(code) => {
			process.exitCode = code;
		},
		(error) => {
			console.error(`plugin-sim: ${error.message}`);
			if (error instanceof UsageError) console.error(`\n${USAGE}`);
			process.exitCode = 2;
		},
	);
}
