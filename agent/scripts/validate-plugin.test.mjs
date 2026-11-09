// The author-facing entry point: one package in, a report and an exit code
// out. Contract: docs/schemas/plugin-manifest.v3.schema.json and
// docs/specs/0031-plugin-fabric-core.md; the rules themselves are tested in
// src/plugin-schema.test.mjs against the shared fixtures.
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdir, mkdtemp, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";

const CLI = fileURLToPath(new URL("./validate-plugin.mjs", import.meta.url));
const REPO = fileURLToPath(new URL("../..", import.meta.url));
const FIXTURES = join(REPO, "daemon/internal/pluginfabric/testdata");
const TEMPLATE = join(REPO, "agent/plugins/_template");

const scratch = await mkdtemp(join(tmpdir(), "coursive-validate-"));
after(() => rm(scratch, { recursive: true, force: true }));

const run = (...args) => spawnSync(process.execPath, [CLI, ...args], { cwd: REPO, encoding: "utf8" });
const json = (result) => JSON.parse(result.stdout);

/** Writes a package directory and returns its path. */
async function pack(name, pkg, files = []) {
	const dir = join(scratch, name);
	await mkdir(dir, { recursive: true });
	await writeFile(join(dir, "package.json"), JSON.stringify(pkg, null, 2));
	for (const file of files) {
		await mkdir(join(dir, file, ".."), { recursive: true });
		await writeFile(join(dir, file), "// fixture\n");
	}
	return dir;
}

const manifest = (overrides = {}) => ({
	version: "1.0.0",
	coursive: { id: "demo", title: "Demo", summary: "A demo plugin", api: "^3.0.0", ...overrides },
});

test("the template package passes with --files and --host", () => {
	const result = run(TEMPLATE, "--files", "--host", "3.0.0", "--json");
	assert.equal(result.status, 0, result.stdout + result.stderr);
	const report = json(result);
	assert.equal(report.ok, true);
	assert.equal(report.schema, "coursive/plugin-manifest@3");
	assert.equal(report.plugin.id, "template");
	assert.equal(report.checks.files, true);
	assert.equal(report.checks.host, "3.0.0");
	assert.deepEqual(report.problems, []);
	assert.match(report.manifest, /agent\/plugins\/_template\/package\.json$/);
});

test("a valid manifest prints a readable OK line", () => {
	const result = run(TEMPLATE);
	assert.equal(result.status, 0);
	assert.match(result.stdout, /OK — template@0\.1\.0, api \^3\.0\.0/);
	assert.equal(result.stderr, "");
});

test("an invalid shared fixture fails with its code, path and a hint", () => {
	const result = run(join(FIXTURES, "invalid-contribution-duplicate.json"), "--json");
	assert.equal(result.status, 1);
	const report = json(result);
	assert.equal(report.ok, false);
	assert.equal(report.problems[0].code, "contribution-duplicate");
	assert.equal(report.problems[0].path, "coursive.contributes.actions[1].id");
	assert.ok(report.problems.every((p) => p.hint), "every problem carries a recovery hint");

	const human = run(join(FIXTURES, "invalid-contribution-duplicate.json"));
	assert.equal(human.status, 1);
	assert.match(human.stdout, /contributes\.actions\[1\]\.id \[contribution-duplicate\]/);
	assert.match(human.stdout, /hint: /);
});

test("a path that leaves the package is rejected", async () => {
	const dir = await pack("escape", {
		version: "1.0.0",
		coursive: {
			id: "escape",
			title: "Escape",
			summary: "A path that leaves the package",
			api: "^3.0.0",
			runtime: [{ kind: "moejs", entry: "../shared/index.js" }],
		},
	});
	const report = json(run(dir, "--json"));
	assert.equal(report.ok, false);
	assert.equal(report.problems[0].code, "runtime-path");
	assert.equal(report.problems[0].path, "coursive.runtime[0].entry");
});

test("a symlink that resolves outside the package is rejected by --files", async (t) => {
	const outside = join(scratch, "outside.mjs");
	await writeFile(outside, "// outside the package\n");
	const dir = await pack("symlink", manifest({ runtime: [{ kind: "pi", entry: "index.mjs" }] }));
	try {
		await symlink(outside, join(dir, "index.mjs"));
	} catch {
		t.skip("symlinks are not available here");
		return;
	}
	const report = json(run(dir, "--files", "--json"));
	assert.equal(report.ok, false);
	assert.equal(report.problems[0].code, "file-escape");
	assert.equal(report.problems[0].path, "coursive.runtime[0].entry");
});

test("--files reports a missing entry once, not twice", async () => {
	const dir = await pack("missing-entry", manifest({ runtime: [{ kind: "pi", entry: "index.mjs" }] }));
	const report = json(run(dir, "--files", "--json"));
	assert.equal(report.ok, false);
	assert.deepEqual(report.problems.map((p) => p.code), ["file-missing"]);
	assert.equal(report.problems[0].path, "coursive.runtime[0].entry");
});

test("--files checks the source executables of a valid fixture", () => {
	const report = json(run(join(FIXTURES, "valid-source-provider.json"), "--files", "--json"));
	assert.equal(report.ok, false);
	assert.ok(report.problems.every((p) => p.code === "file-missing"));
	assert.ok(report.problems.some((p) => p.path === "coursive.contributes.sources[0].executable"));
});

test("an unknown api range for the given host is reported", () => {
	const report = json(run(TEMPLATE, "--host", "2.0.0", "--json"));
	assert.equal(report.ok, false);
	assert.equal(report.problems[0].code, "api-incompatible");
	assert.equal(report.problems[0].path, "coursive.api");
});

test("an unknown plugin version or runtime kind is rejected", async () => {
	const dir = await pack("unknown", {
		version: "v1",
		coursive: { id: "unknown", title: "Unknown", summary: "An unknown version and runtime", api: "^3.0.0", runtime: [{ kind: "wasm", entry: "logic/index.wasm" }] },
	});
	const report = json(run(dir, "--json"));
	assert.equal(report.ok, false);
	assert.deepEqual(report.problems.map((p) => p.code), ["version-invalid", "runtime-kind"]);
	assert.deepEqual(report.problems.map((p) => p.path), ["version", "coursive.runtime[0].kind"]);
});

test("a v1 package is rejected with its v3 home", async () => {
	const dir = await pack("v1", {
		version: "1.0.0",
		coursive: { id: "v1", title: "V1", summary: "A v1 manifest", api: "^3.0.0", engine: "^1.0.0", agent: "index.mjs" },
	});
	const report = json(run(dir, "--json"));
	assert.equal(report.ok, false);
	const unknown = report.problems.filter((p) => p.code === "field-unknown");
	assert.deepEqual(unknown.map((p) => p.path), ["coursive.agent", "coursive.engine"]);
	assert.match(unknown[0].hint, /coursive\.runtime/);
	assert.match(unknown[1].hint, /coursive\.api/);
});

test("a package without a coursive manifest is rejected", async () => {
	const dir = await pack("plain", { name: "plain", version: "1.0.0" });
	const report = json(run(dir, "--json"));
	assert.equal(report.ok, false);
	assert.equal(report.problems[0].code, "manifest-missing");
	assert.equal(report.problems[0].path, "coursive");
});

test("a usage or unreadable input exits 2 with a readable message", async () => {
	const noTarget = run();
	assert.equal(noTarget.status, 2);
	assert.match(noTarget.stderr, /name a plugin directory or a package\.json/);
	assert.match(noTarget.stderr, /Usage:/);

	const unknownOption = run(TEMPLATE, "--wat");
	assert.equal(unknownOption.status, 2);
	assert.match(unknownOption.stderr, /unknown option --wat/);

	const badHost = run(TEMPLATE, "--host", "3");
	assert.equal(badHost.status, 2);
	assert.match(badHost.stderr, /--host needs a version/);

	const missing = run(join(scratch, "nope"));
	assert.equal(missing.status, 2);
	assert.match(missing.stderr, /no such file or directory/);

	const dir = join(scratch, "broken");
	await mkdir(dir, { recursive: true });
	await writeFile(join(dir, "package.json"), "{ not json");
	const broken = run(dir);
	assert.equal(broken.status, 2);
	assert.match(broken.stderr, /not valid JSON/);

	assert.equal(run("--help").status, 0);
});
