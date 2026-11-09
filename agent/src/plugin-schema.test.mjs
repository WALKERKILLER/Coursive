// Validates the shared plugin-manifest v3 fixtures with the agent's rule
// module, so the daemon's Go parser, the assistant host and the app agree on
// the accepted shape. Contract: docs/schemas/plugin-manifest.v3.schema.json
// and docs/specs/0031-plugin-fabric-core.md.
import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import {
	CATEGORIES,
	GROUPS,
	PERMISSIONS,
	REQUIRED_FIELDS,
	RUNTIME_KINDS,
	V1_HOMES,
	firstProblem,
	insidePackage,
	problems,
	satisfiesApi,
} from "./plugin-manifest.mjs";

const FIXTURES = fileURLToPath(new URL("../../daemon/internal/pluginfabric/testdata/", import.meta.url));
const SCHEMA = fileURLToPath(new URL("../../docs/schemas/plugin-manifest.v3.schema.json", import.meta.url));

const readJSON = async (path) => JSON.parse(await readFile(path, "utf8"));

test("shared fixtures validate the same way as the daemon", async () => {
	const index = await readJSON(`${FIXTURES}cases.json`);
	assert.equal(index.schema, "coursive/plugin-manifest@3");
	for (const c of index.cases) {
		const pkg = await readJSON(`${FIXTURES}${c.file}`);
		const first = firstProblem(pkg);
		if (c.code === undefined) {
			assert.equal(first, null, `${c.file}: ${JSON.stringify(first)}`);
			continue;
		}
		assert.ok(first, `${c.file}: expected ${c.code}, got a valid manifest`);
		assert.equal(first.code, c.code, c.file);
		assert.equal(first.path, c.path, c.file);
		assert.ok(first.hint, `${c.file}: ${first.code} has no recovery hint`);
	}
	const listed = new Set(index.cases.map((c) => c.file));
	for (const e of await readdir(FIXTURES, { withFileTypes: true })) {
		if (e.isFile() && e.name !== "cases.json") assert.ok(listed.has(e.name), `testdata/${e.name} is not listed in cases.json`);
	}
});

test("the schema and this validator agree on the closed sets", async () => {
	const schema = await readJSON(SCHEMA);
	const defs = schema.$defs;
	const enums = (v) => [...v];
	assert.deepEqual(enums(defs.plugin.properties.permissions.items.enum).sort(), [...PERMISSIONS].sort());
	assert.deepEqual(enums(defs.runtime.properties.kind.enum).sort(), [...RUNTIME_KINDS].sort());
	assert.deepEqual(enums(defs.plugin.properties.categories.items.enum).sort(), [...CATEGORIES].sort());
	assert.deepEqual(Object.keys(defs.contributes.properties).sort(), [...GROUPS].sort());
	assert.deepEqual(enums(defs.plugin.required).sort(), [...REQUIRED_FIELDS].sort());
});

test("a manifest with an unknown field names the field", async () => {
	const pkg = { version: "1.0.0", coursive: { id: "demo", title: "演示", summary: "演示插件", api: "^3.0.0", engine: "^1.0.0" } };
	const first = firstProblem(pkg);
	assert.equal(first.code, "field-unknown");
	assert.equal(first.path, "coursive.engine");
	assert.match(first.hint, /coursive\.api/);
});

test("a v1 manifest is pointed at its v3 homes", () => {
	const pkg = {
		version: "1.0.0",
		coursive: { id: "demo", title: "演示", summary: "演示插件", api: "^3.0.0", agent: "index.mjs", skills: ["skills"], views: ["checklist"] },
	};
	const hints = problems(pkg).filter((p) => p.code === "field-unknown");
	assert.deepEqual(hints.map((p) => p.path), ["coursive.agent", "coursive.skills", "coursive.views"]);
	for (const p of hints) assert.ok(p.hint.includes(V1_HOMES[p.path.slice("coursive.".length)]), p.hint);
});

test("the api range is checked against a host version", () => {
	assert.equal(satisfiesApi("^3.0.0", "3.0.0"), true);
	assert.equal(satisfiesApi("^3.0.0", "3.4.1"), true);
	assert.equal(satisfiesApi("^3.0.0", "4.0.0"), false);
	assert.equal(satisfiesApi("^3.0.0", "2.9.9"), false);
	assert.equal(satisfiesApi(">=3.1.0", "3.0.0"), false);
	assert.equal(satisfiesApi(">=3.1.0", "3.1.0"), true);
	assert.equal(satisfiesApi("3.0.0", "3.0.1"), false);
	assert.equal(satisfiesApi("^3", "3.0.0"), false);
});

test("a valid manifest has no problems", async () => {
	const pkg = await readJSON(`${FIXTURES}valid-full.json`);
	assert.deepEqual(problems(pkg), []);
	assert.equal(pkg.coursive.runtime[0].entry, "./agent/index.mjs");
	assert.equal(insidePackage("agent/index.mjs"), true);
	assert.equal(insidePackage("./agent/index.mjs"), false);
});
