// Validates the shared plugin-manifest v3 fixtures with a small standalone
// validator, so the daemon's Go parser, the assistant host and the app agree
// on the accepted shape. Contract: docs/schemas/plugin-manifest.v3.schema.json
// and docs/specs/0031-plugin-fabric-core.md.
import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const FIXTURES = fileURLToPath(new URL("../../daemon/internal/pluginfabric/testdata/", import.meta.url));
const SCHEMA = fileURLToPath(new URL("../../docs/schemas/plugin-manifest.v3.schema.json", import.meta.url));

export const RUNTIME_KINDS = ["pi", "moejs", "process", "web-ui", "go"];
export const PERMISSIONS = ["library.read", "library.write", "views", "localhost", "exec", "workspace", "network", "folder", "source"];
export const CATEGORIES = ["notes", "visualize", "review", "programming", "connector", "tutor", "assignment"];
export const GROUPS = ["navigation", "pages", "slots", "tools", "views", "actions", "settings", "sources"];
const REQUIRED = ["id", "title", "summary", "api"];

const ID = /^[a-z][a-z0-9-]{1,31}$/;
const VERSION = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;
const API = /^(?:\^|>=)?\d+\.\d+\.\d+$/;
const SERVICE = /^[a-z][a-z0-9]*(?:\.[a-z][a-z0-9]*)+$/;
const SERVICE_RANGE = /^(?:\^|>=)?\d+(?:\.\d+){0,2}$/;
const CONTRIBUTION = /^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$/;

const list = (v) => (Array.isArray(v) ? v : []);
const text = (v) => typeof v === "string" ? v : "";

/** A path inside the plugin package: no absolute path, no dot segment. */
export function insidePackage(p) {
	if (typeof p !== "string" || !p || p.startsWith("/") || p.includes("\\") || /^[A-Za-z]:/.test(p)) return false;
	return p.split("/").every((segment) => segment !== "" && segment !== "." && segment !== "..");
}

const trimDotSlash = (p) => typeof p === "string" ? p.replace(/^(?:\.\/)+/, "") : p;

/** Entry paths of a runtime entry: one string, or one path per platform. */
function entryPaths(entry) {
	if (typeof entry === "string") return [trimDotSlash(entry)];
	if (entry && typeof entry === "object") return Object.values(entry).map(trimDotSlash);
	return null;
}

/**
 * Validates a manifest and returns every problem it finds, in the order the
 * daemon reports them. The first entry is the one a caller shows first.
 */
export function problems(pkg) {
	const out = [];
	const add = (code, path, message) => out.push({ code, path, message });

	if (!pkg || typeof pkg !== "object" || !pkg.coursive || typeof pkg.coursive !== "object") {
		add("manifest-missing", "coursive", "插件包缺少 coursive 清单");
		return out;
	}
	const c = pkg.coursive;

	if (!pkg.version) add("version-missing", "version", "缺少 version");
	else if (!VERSION.test(pkg.version)) add("version-invalid", "version", "version 应为 semver，例如 1.0.0");

	if (!c.id) add("id-missing", "coursive.id", "缺少 id");
	else if (!ID.test(c.id)) add("id-invalid", "coursive.id", "id 应为 2–32 位小写字母、数字或 -，以字母开头");

	const title = text(c.title);
	if ([...title].length === 0 || [...title].length > 24) add("title-invalid", "coursive.title", "title 应为 1–24 字");
	const summary = text(c.summary);
	if ([...summary].length === 0 || [...summary].length > 80) add("summary-invalid", "coursive.summary", "summary 应为 1–80 字");

	if (!c.api) add("api-missing", "coursive.api", "缺少 api：插件需要的接口范围，例如 ^3.0.0");
	else if (!API.test(c.api)) add("api-invalid", "coursive.api", "api 应为 ^x.y.z、>=x.y.z 或精确版本");

	list(c.categories).forEach((category, i) => {
		if (!CATEGORIES.includes(category)) add("category-unknown", `coursive.categories[${i}]`, `未知分类 ${JSON.stringify(category)}`);
	});

	list(c.runtime).forEach((r, i) => {
		const at = `coursive.runtime[${i}]`;
		if (!RUNTIME_KINDS.includes(r?.kind)) add("runtime-kind", `${at}.kind`, `未知的运行时 ${JSON.stringify(r?.kind)}`);
		const paths = entryPaths(r?.entry);
		if (!paths || !paths.length) add("runtime-entry", `${at}.entry`, "entry 应为插件目录内的相对路径，或按平台给出的路径表");
		for (const p of paths ?? []) if (!insidePackage(p)) add("runtime-path", `${at}.entry`, `路径 ${JSON.stringify(p)} 越出插件目录`);
		if (list(r?.skills).length && r?.kind !== "pi") add("runtime-skills", `${at}.skills`, "只有 pi 运行时可以声明 skills");
		for (const p of list(r?.skills)) if (!insidePackage(trimDotSlash(p))) add("runtime-path", `${at}.skills`, `路径 ${JSON.stringify(p)} 越出插件目录`);
	});

	list(c.requires).forEach((r, i) => {
		const at = `coursive.requires[${i}]`;
		if (!SERVICE.test(text(r?.service))) add("requires-invalid", `${at}.service`, "服务 id 应为 domain.verb，例如 transcript.read");
		if (!SERVICE_RANGE.test(text(r?.version))) add("requires-invalid", `${at}.version`, "服务版本应为 ^1、^1.2.3、>=1.0.0 或精确版本");
	});
	list(c.provides).forEach((r, i) => {
		const at = `coursive.provides[${i}]`;
		if (!SERVICE.test(text(r?.service))) add("provides-invalid", `${at}.service`, "服务 id 应为 domain.verb");
		if (!VERSION.test(text(r?.version))) add("provides-invalid", `${at}.version`, "提供的服务版本应为精确的 semver，例如 1.0.0");
	});

	const declared = new Set();
	list(c.permissions).forEach((permission, i) => {
		const at = `coursive.permissions[${i}]`;
		if (!PERMISSIONS.includes(permission)) add("permission-unknown", at, `未知权限 ${JSON.stringify(permission)}`);
		if (declared.has(permission)) add("permission-duplicate", at, `权限 ${JSON.stringify(permission)} 重复声明`);
		declared.add(permission);
	});
	if (declared.has("exec") && !list(c.exec).length) add("exec-undeclared", "coursive.exec", "exec 权限需要在 exec 中列出允许的命令");
	if (declared.has("network") && !list(c.hosts).length) add("hosts-undeclared", "coursive.hosts", "network 权限需要在 hosts 中列出允许的域名");

	const contributes = c.contributes && typeof c.contributes === "object" ? c.contributes : {};
	for (const name of Object.keys(contributes).sort()) {
		if (!GROUPS.includes(name)) add("contribution-unknown", `coursive.contributes.${name}`, `未知的贡献组 ${JSON.stringify(name)}`);
	}
	for (const name of ["navigation", "pages", "slots", "views", "actions", "settings"]) {
		const seen = new Set();
		list(contributes[name]).forEach((e, i) => {
			const at = `coursive.contributes.${name}[${i}].id`;
			if (!CONTRIBUTION.test(text(e?.id))) {
				add("contribution-invalid", at, `贡献 id ${JSON.stringify(e?.id)} 无效`);
				return;
			}
			if (seen.has(e.id)) add("contribution-duplicate", at, `贡献 id ${JSON.stringify(e.id)} 在同一组内重复`);
			seen.add(e.id);
		});
	}
	const seenSources = new Set();
	list(contributes.sources).forEach((s, i) => {
		const at = `coursive.contributes.sources[${i}]`;
		if (!CONTRIBUTION.test(text(s?.id))) add("contribution-invalid", `${at}.id`, `来源 id ${JSON.stringify(s?.id)} 无效`);
		else if (seenSources.has(s.id)) add("contribution-duplicate", `${at}.id`, `来源 id ${JSON.stringify(s.id)} 重复声明`);
		seenSources.add(s?.id);
		if (!text(s?.title).trim()) add("source-invalid", `${at}.title`, "来源需要 title");
		if (!text(s?.protocol).trim()) add("source-invalid", `${at}.protocol`, "来源需要它实现的协议版本");
		const executable = s?.executable;
		if (!executable || typeof executable !== "object" || Array.isArray(executable) || !Object.keys(executable).length) {
			add("source-invalid", `${at}.executable`, "executable 应为按平台给出的程序路径表");
		}
		for (const p of entryPaths(executable) ?? []) {
			if (!insidePackage(p)) add("source-path", `${at}.executable`, `路径 ${JSON.stringify(p)} 越出插件目录`);
		}
	});
	const seenTools = new Set();
	list(contributes.tools).forEach((tool, i) => {
		const at = `coursive.contributes.tools[${i}]`;
		if (!SERVICE.test(text(tool))) {
			add("contribution-invalid", at, "工具应由它提供的服务标识，例如 review.generate");
			return;
		}
		if (seenTools.has(tool)) add("contribution-duplicate", at, `工具 ${JSON.stringify(tool)} 重复声明`);
		seenTools.add(tool);
	});

	for (const field of Object.keys(c).sort()) {
		if (["id", "title", "summary", "description", "icon", "categories", "api", "runtime", "requires", "provides", "permissions", "exec", "hosts", "settings", "examples", "contributes"].includes(field)) continue;
		add("field-unknown", `coursive.${field}`, `v3 清单没有 ${JSON.stringify(field)} 字段`);
	}
	return out;
}

/** The first problem of a manifest, or null when it is acceptable. */
export const firstProblem = (pkg) => problems(pkg)[0] ?? null;

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
	}
	const listed = new Set(index.cases.map((c) => c.file));
	for (const name of await readdir(FIXTURES)) {
		if (name !== "cases.json") assert.ok(listed.has(name), `testdata/${name} is not listed in cases.json`);
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
	assert.deepEqual(enums(defs.plugin.required).sort(), [...REQUIRED].sort());
});

test("a manifest with an unknown field names the field", async () => {
	const pkg = { version: "1.0.0", coursive: { id: "demo", title: "演示", summary: "演示插件", api: "^3.0.0", engine: "^1.0.0" } };
	const first = firstProblem(pkg);
	assert.equal(first.code, "field-unknown");
	assert.equal(first.path, "coursive.engine");
});

test("a valid manifest has no problems", async () => {
	const pkg = await readJSON(`${FIXTURES}valid-full.json`);
	assert.deepEqual(problems(pkg), []);
	assert.equal(pkg.coursive.runtime[0].entry, "./agent/index.mjs");
	assert.equal(insidePackage("agent/index.mjs"), true);
	assert.equal(insidePackage("./agent/index.mjs"), false);
});
