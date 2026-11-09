// Canonical Coursive plugin manifest rules, version 3.
//
// The accepted shape is docs/schemas/plugin-manifest.v3.schema.json and the
// contract is docs/specs/0031-plugin-fabric-core.md. The daemon parser
// daemon/internal/pluginfabric/manifest.go reports the same codes and paths;
// daemon/internal/pluginfabric/testdata/cases.json is the shared fixture list
// every validator is tested against. The author-facing entry point is
// scripts/validate-plugin.mjs.
//
// problems(pkg) returns every defect with a stable code, the JSON path, a
// message for the plugin author and a recovery hint, in the order the daemon
// reports them. The first entry is the one a caller shows first.

/** Manifest shape this module accepts; a new shape is a new schema file. */
export const SCHEMA_ID = "coursive/plugin-manifest@3";

/** Executors a manifest may name. */
export const RUNTIME_KINDS = ["pi", "moejs", "process", "web-ui", "go"];

/** Permission names a manifest may declare. */
export const PERMISSIONS = ["library.read", "library.write", "views", "localhost", "exec", "workspace", "network", "folder", "source"];

/** Marketplace categories a manifest may name. */
export const CATEGORIES = ["notes", "visualize", "review", "programming", "connector", "tutor", "assignment"];

/** Contribution groups `contributes` may hold. */
export const GROUPS = ["navigation", "pages", "slots", "tools", "views", "actions", "settings", "sources"];

/** Fields `coursive` may hold; anything else was written for another version. */
export const PLUGIN_FIELDS = ["id", "title", "summary", "description", "icon", "categories", "api", "runtime",
	"requires", "provides", "permissions", "exec", "hosts", "settings", "examples", "contributes"];

/** Fields `coursive` must hold. */
export const REQUIRED_FIELDS = ["id", "title", "summary", "api"];

/**
 * A v1 manifest field and its v3 home, so an unmigrated package gets an
 * instruction instead of "unknown field".
 */
export const V1_HOMES = {
	engine: "coursive.api",
	agent: 'coursive.runtime[{kind: "pi", entry}]',
	skills: 'coursive.runtime[{kind: "pi", skills}]',
	views: "coursive.contributes.views",
};

const ID = /^[a-z][a-z0-9-]{1,31}$/;
const VERSION = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;
const API = /^(?:\^|>=)?\d+\.\d+\.\d+$/;
const SERVICE = /^[a-z][a-z0-9]*(?:\.[a-z][a-z0-9]*)+$/;
const SERVICE_RANGE = /^(?:\^|>=)?\d+(?:\.\d+){0,2}$/;
const CONTRIBUTION = /^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$/;

/** Contribution groups whose entries carry an `id`. */
const ENTRY_GROUPS = ["navigation", "pages", "slots", "views", "actions", "settings"];

const list = (v) => (Array.isArray(v) ? v : []);
const text = (v) => (typeof v === "string" ? v : "");
const length = (v) => [...text(v)].length;
const json = (v) => JSON.stringify(v);

/**
 * A path inside the plugin package: no absolute path, drive letter,
 * backslash, empty, dot or dot-dot segment. A leading ./ is normalized away
 * by [trimDotSlash] before this runs, the way the install does.
 */
export function insidePackage(p) {
	if (typeof p !== "string" || !p || p.startsWith("/") || p.includes("\\") || /^[A-Za-z]:/.test(p)) return false;
	return p.split("/").every((segment) => segment !== "" && segment !== "." && segment !== "..");
}

/** Removes a leading ./ that v1 manifests write. */
export const trimDotSlash = (p) => (typeof p === "string" ? p.replace(/^(?:\.\/)+/, "") : p);

/** Entry paths of a runtime entry: one string, or one path per platform. */
export function entryPaths(entry) {
	if (typeof entry === "string") return [trimDotSlash(entry)];
	if (entry && typeof entry === "object" && !Array.isArray(entry)) return Object.values(entry).map(trimDotSlash);
	return null;
}

/**
 * Reports whether a host version satisfies a manifest's api range: `^x.y.z`
 * is the same major at or above it, `>=x.y.z` is at or above it, and a bare
 * version must match exactly. Both arguments are complete versions.
 */
export function satisfiesApi(range, host) {
	const r = text(range).trim();
	const want = /^(?:\^|>=)?(\d+)\.(\d+)\.(\d+)$/.exec(r);
	const have = /^(\d+)\.(\d+)\.(\d+)$/.exec(text(host).trim());
	if (!want || !have) return false;
	const w = [+want[1], +want[2], +want[3]];
	const h = [+have[1], +have[2], +have[3]];
	const cmp = h[0] - w[0] || h[1] - w[1] || h[2] - w[2];
	if (r.startsWith(">=")) return cmp >= 0;
	if (r.startsWith("^")) return h[0] === w[0] && cmp >= 0;
	return cmp === 0;
}

const HINTS = {
	"manifest-missing": 'Add a "coursive" object to package.json; its shape is docs/schemas/plugin-manifest.v3.schema.json.',
	"manifest-invalid": "The coursive manifest must be a JSON object.",
	"version-missing": 'Set the package version, for example "1.0.0".',
	"version-invalid": 'Write version as semver, for example "1.0.0" or "1.0.0-beta.1".',
	"id-missing": "Give the plugin a stable id; it is also the install directory name and the tool-name prefix.",
	"id-invalid": 'Use 2-32 lowercase letters, digits or dashes, starting with a letter, for example "review-lab".',
	"title-invalid": "Give the plugin a title of 1-24 characters.",
	"summary-invalid": "Summarise the plugin in 1-80 characters.",
	"api-missing": 'Declare the plugin API range you build against, for example "^3.0.0".',
	"api-invalid": "Write api as ^x.y.z, >=x.y.z or an exact x.y.z version.",
	"category-unknown": `Pick a category from: ${CATEGORIES.join(", ")}.`,
	"runtime-kind": `Use one of: ${RUNTIME_KINDS.join(", ")}.`,
	"runtime-entry": "Point entry at a path inside the plugin package, or at one path per platform.",
	"runtime-path": 'Keep entry and skill paths inside the plugin package; remove "..", a leading "/" or a drive letter.',
	"runtime-skills": "Skills belong to a pi runtime entry; drop them from another kind.",
	"requires-invalid": "Write the service as domain.verb (for example transcript.read) and the version as ^1, ^1.2.3, >=1.0.0 or an exact version.",
	"provides-invalid": "Write the service as domain.verb and the version as an exact semver, for example 1.0.0.",
	"permission-unknown": `Declare only: ${PERMISSIONS.join(", ")}.`,
	"permission-duplicate": "Remove the repeated permission.",
	"exec-undeclared": "List the commands the exec permission allows under coursive.exec.",
	"hosts-undeclared": "List the host names the network permission allows under coursive.hosts.",
	"contribution-unknown": `Move the entries into one of: ${GROUPS.join(", ")}.`,
	"contribution-invalid": "Start the id with a lowercase letter and use only lowercase letters, digits and . _ - .",
	"contribution-duplicate": "Give every contribution in a group its own id.",
	"source-invalid": "A source needs id, title, protocol and an executable map keyed by platform.",
	"source-path": 'Keep the executable inside the plugin package; remove "..", a leading "/" or a drive letter.',
	"api-incompatible": "Raise the host version or lower the api range the plugin declares.",
	"file-missing": "Add the file, or point the manifest at a path that exists in the package.",
	"file-escape": "Keep the file inside the plugin package; a symlink may not resolve outside it.",
};

/**
 * Validates a manifest and returns every problem it finds. Each problem is
 * `{code, path, message, hint}`; the list is empty when the manifest is
 * acceptable.
 */
export function problems(pkg) {
	const out = [];
	const add = (code, path, message, hint = HINTS[code]) => out.push({ code, path, message, ...(hint ? { hint } : {}) });

	if (!pkg || typeof pkg !== "object" || Array.isArray(pkg)) {
		add("manifest-missing", "coursive", "package.json is not a JSON object");
		return out;
	}
	if (!("coursive" in pkg) || pkg.coursive === null || pkg.coursive === undefined) {
		add("manifest-missing", "coursive", "package.json has no coursive manifest");
		return out;
	}
	if (typeof pkg.coursive !== "object" || Array.isArray(pkg.coursive)) {
		add("manifest-invalid", "coursive", "the coursive manifest is not a JSON object");
		return out;
	}
	const c = pkg.coursive;

	if (!pkg.version) add("version-missing", "version", "version is missing");
	else if (!VERSION.test(pkg.version)) add("version-invalid", "version", `version ${json(pkg.version)} is not semver`);

	if (!c.id) add("id-missing", "coursive.id", "id is missing");
	else if (!ID.test(c.id)) add("id-invalid", "coursive.id", `id ${json(c.id)} is not 2-32 lowercase letters, digits or dashes`);

	const title = length(c.title);
	if (title === 0 || title > 24) add("title-invalid", "coursive.title", `title has ${title} characters; it must have 1-24`);
	const summary = length(c.summary);
	if (summary === 0 || summary > 80) add("summary-invalid", "coursive.summary", `summary has ${summary} characters; it must have 1-80`);

	if (!c.api) add("api-missing", "coursive.api", "api is missing");
	else if (!API.test(c.api)) add("api-invalid", "coursive.api", `api ${json(c.api)} is not a version range`);

	list(c.categories).forEach((category, i) => {
		if (!CATEGORIES.includes(category)) add("category-unknown", `coursive.categories[${i}]`, `unknown category ${json(category)}`);
	});

	list(c.runtime).forEach((r, i) => {
		const at = `coursive.runtime[${i}]`;
		if (!RUNTIME_KINDS.includes(r?.kind)) add("runtime-kind", `${at}.kind`, `unknown runtime ${json(r?.kind)}`);
		const paths = entryPaths(r?.entry);
		if (!paths || !paths.length) add("runtime-entry", `${at}.entry`, "entry is not a package-relative path or a platform map");
		for (const p of paths ?? []) if (!insidePackage(p)) add("runtime-path", `${at}.entry`, `path ${json(p)} leaves the plugin package`);
		if (list(r?.skills).length && r?.kind !== "pi") add("runtime-skills", `${at}.skills`, `only a pi runtime declares skills, not ${json(r?.kind)}`);
		for (const p of list(r?.skills)) if (!insidePackage(trimDotSlash(p))) add("runtime-path", `${at}.skills`, `path ${json(p)} leaves the plugin package`);
	});

	list(c.requires).forEach((r, i) => {
		const at = `coursive.requires[${i}]`;
		if (!SERVICE.test(text(r?.service))) add("requires-invalid", `${at}.service`, `service ${json(r?.service)} is not domain.verb`);
		if (!SERVICE_RANGE.test(text(r?.version))) add("requires-invalid", `${at}.version`, `version ${json(r?.version)} is not a service range`);
	});
	list(c.provides).forEach((r, i) => {
		const at = `coursive.provides[${i}]`;
		if (!SERVICE.test(text(r?.service))) add("provides-invalid", `${at}.service`, `service ${json(r?.service)} is not domain.verb`);
		if (!VERSION.test(text(r?.version))) add("provides-invalid", `${at}.version`, `version ${json(r?.version)} is not an exact semver`);
	});

	const declared = new Set();
	list(c.permissions).forEach((permission, i) => {
		const at = `coursive.permissions[${i}]`;
		if (!PERMISSIONS.includes(permission)) add("permission-unknown", at, `unknown permission ${json(permission)}`);
		if (declared.has(permission)) add("permission-duplicate", at, `permission ${json(permission)} is declared twice`);
		declared.add(permission);
	});
	if (declared.has("exec") && !list(c.exec).length) add("exec-undeclared", "coursive.exec", "the exec permission needs the commands it allows");
	if (declared.has("network") && !list(c.hosts).length) add("hosts-undeclared", "coursive.hosts", "the network permission needs the hosts it allows");

	const contributes = c.contributes && typeof c.contributes === "object" && !Array.isArray(c.contributes) ? c.contributes : {};
	for (const name of Object.keys(contributes).sort()) {
		if (!GROUPS.includes(name)) add("contribution-unknown", `coursive.contributes.${name}`, `unknown contribution group ${json(name)}`);
	}
	for (const name of ENTRY_GROUPS) {
		const seen = new Set();
		list(contributes[name]).forEach((e, i) => {
			const at = `coursive.contributes.${name}[${i}].id`;
			if (!CONTRIBUTION.test(text(e?.id))) {
				add("contribution-invalid", at, `contribution id ${json(e?.id)} is not a lowercase dotted or dashed name`);
				return;
			}
			if (seen.has(e.id)) add("contribution-duplicate", at, `contribution id ${json(e.id)} is used twice in ${name}`);
			seen.add(e.id);
		});
	}
	const seenSources = new Set();
	list(contributes.sources).forEach((s, i) => {
		const at = `coursive.contributes.sources[${i}]`;
		if (!CONTRIBUTION.test(text(s?.id))) add("contribution-invalid", `${at}.id`, `source id ${json(s?.id)} is not a lowercase dotted or dashed name`);
		else if (seenSources.has(s.id)) add("contribution-duplicate", `${at}.id`, `source id ${json(s.id)} is declared twice`);
		seenSources.add(s?.id);
		if (!text(s?.title).trim()) add("source-invalid", `${at}.title`, "source title is missing");
		if (!text(s?.protocol).trim()) add("source-invalid", `${at}.protocol`, "source protocol version is missing");
		if (!s?.executable || typeof s.executable !== "object" || Array.isArray(s.executable) || !Object.keys(s.executable).length) {
			add("source-invalid", `${at}.executable`, "executable is not a platform-keyed path map");
		}
		for (const p of entryPaths(s?.executable) ?? []) if (!insidePackage(p)) add("source-path", `${at}.executable`, `path ${json(p)} leaves the plugin package`);
	});
	const seenTools = new Set();
	list(contributes.tools).forEach((tool, i) => {
		const at = `coursive.contributes.tools[${i}]`;
		if (!SERVICE.test(text(tool))) {
			add("contribution-invalid", at, `tool ${json(tool)} is not the domain.verb service it exposes`);
			return;
		}
		if (seenTools.has(tool)) add("contribution-duplicate", at, `tool ${json(tool)} is declared twice`);
		seenTools.add(tool);
	});

	for (const field of Object.keys(c).sort()) {
		if (PLUGIN_FIELDS.includes(field)) continue;
		const home = V1_HOMES[field];
		add("field-unknown", `coursive.${field}`,
			home ? `the v1 field ${json(field)} is now ${home}` : `the v3 manifest has no ${json(field)} field`,
			home ? `Rename ${json(field)} to ${home}.` : `Remove ${json(field)}; the v3 manifest does not define it.`);
	}
	return out;
}

/** The first problem of a manifest, or null when it is acceptable. */
export const firstProblem = (pkg) => problems(pkg)[0] ?? null;
