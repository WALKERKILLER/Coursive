import assert from "node:assert/strict";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { bundledPluginsDir } from "../src/plugins.mjs";
import { matches, parseArgs, simulate } from "./plugin-sim.mjs";
import { UsageError } from "./validate-plugin.mjs";

const reviewLab = join(bundledPluginsDir, "_examples", "review-lab");

async function scratch(t, files) {
	const dir = await mkdtemp(join(tmpdir(), "plugin-sim-test-"));
	t.after(() => rm(dir, { recursive: true, force: true }));
	for (const [name, body] of Object.entries(files)) {
		await mkdir(join(dir, name, ".."), { recursive: true });
		await writeFile(join(dir, name), typeof body === "string" ? body : JSON.stringify(body));
	}
	return dir;
}

const manifest = (contributes, extra = {}) => ({
	name: "p-sim",
	version: "1.0.0",
	coursive: { id: "sim-demo", title: "测试", summary: "测试插件", api: "^3.0.0", runtime: [{ kind: "pi", entry: "index.mjs" }], contributes, ...extra },
});

test("the Review Lab example passes every scripted case and packs its entry", async (t) => {
	const out = await mkdtemp(join(tmpdir(), "plugin-sim-pack-"));
	t.after(() => rm(out, { recursive: true, force: true }));
	const r = await simulate(reviewLab, { pack: out });
	assert.deepEqual(r.problems, []);
	assert.ok(r.cases.length >= 9);
	assert.deepEqual(r.cases.filter((c) => !c.ok), []);
	assert.ok(r.pack.files.includes("index.mjs"));
	assert.equal(r.ok, true);
});

test("a failed case, a missing handler and a missing packed entry are reported", async (t) => {
	const dir = await scratch(t, {
		"package.json": { ...manifest({ actions: [{ id: "go", title: "走", action: "go" }, { id: "gone", title: "没了", action: "gone" }] }), files: ["README.md"] },
		"index.mjs": `export function activate(c) { c.action("go", async () => ({ notification: { text: "went" } })); }`,
		"README.md": "x",
	});
	const missing = await simulate(dir);
	assert.equal(missing.ok, false);
	assert.match(missing.problems.join("\n"), /gone.*never registers/);

	await writeFile(join(dir, "package.json"), JSON.stringify({ ...manifest({ actions: [{ id: "go", title: "走", action: "go" }] }), files: ["README.md"] }));
	await writeFile(join(dir, "cases.json"), JSON.stringify({ cases: [
		{ name: "right", action: "go", expect: { notification: { text: "went" } } },
		{ name: "wrong", action: "go", expect: { notification: { text: "stayed" } } },
		{ name: "refused", action: "nope", expectError: "没有动作" },
	] }));
	const run = await simulate(dir, { cases: join(dir, "cases.json") });
	assert.deepEqual(run.cases.map((c) => [c.name, c.ok]), [["right", true], ["wrong", false], ["refused", true]]);
	assert.equal(run.ok, false);

	await writeFile(join(dir, "cases.json"), JSON.stringify({ cases: [] }));
	const out = await mkdtemp(join(tmpdir(), "plugin-sim-pack-"));
	t.after(() => rm(out, { recursive: true, force: true }));
	const packed = await simulate(dir, { cases: join(dir, "cases.json"), pack: out });
	assert.equal(packed.ok, false);
	assert.match(packed.problems.join("\n"), /missing index\.mjs/);
});

test("a manifest the host refuses never runs", async (t) => {
	const dir = await scratch(t, {
		"package.json": manifest({}, { permissions: ["camera"] }),
		"index.mjs": "export function activate() { globalThis.__simRan = true; }",
	});
	const r = await simulate(dir);
	assert.equal(r.ok, false);
	assert.ok(r.problems.length > 0);
	assert.equal(globalThis.__simRan, undefined);
});

test("matches and argument parsing", () => {
	assert.equal(matches({ a: 1, b: { c: [1, { d: 2, e: 3 }] } }, { b: { c: [1, { d: 2 }] } }), true);
	assert.equal(matches({ a: [1, 2] }, { a: [1] }), false);
	assert.equal(matches({ a: null }, { a: 0 }), false);
	assert.deepEqual(parseArgs(["dir", "--pack", "out", "--json"]), { target: "dir", cases: "", pack: "out", json: true, help: false });
	assert.throws(() => parseArgs([]), UsageError);
	assert.throws(() => parseArgs(["a", "--pack"]), UsageError);
	assert.throws(() => parseArgs(["a", "--wat"]), UsageError);
});
