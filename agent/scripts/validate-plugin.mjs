#!/usr/bin/env node
// validate-plugin.mjs — validate one plugin package against the canonical
// manifest. The rules live in src/plugin-manifest.mjs, so this CLI, the shared
// fixture test and the daemon report the same codes and paths.
//
// Contract: docs/schemas/plugin-manifest.v3.schema.json and
// docs/specs/0031-plugin-fabric-core.md. Usage:
//
//   node agent/scripts/validate-plugin.mjs <plugin-dir | package.json> [options]
//
//   --json          machine-readable report on stdout, for CI
//   --files         also check that declared entry, skill and source files exist
//                   inside the package and do not resolve outside it
//   --host <x.y.z>  also check that the host API version satisfies coursive.api
//   -h, --help      show this usage
//
// Zero dependencies; Node >= 22. Exit 0 when the manifest is acceptable, 1 when
// it has problems, 2 on a usage or unreadable-input error.
import { readFile, realpath, stat } from "node:fs/promises";
import { dirname, join, relative, resolve, sep } from "node:path";
import process from "node:process";
import { pathToFileURL } from "node:url";
import { SCHEMA_ID, entryPaths, insidePackage, problems, satisfiesApi, trimDotSlash } from "../src/plugin-manifest.mjs";

const USAGE = `Usage: node agent/scripts/validate-plugin.mjs <plugin-dir | package.json> [options]

  --json          machine-readable report on stdout, for CI
  --files         also check that declared entry, skill and source files exist
                  inside the package and do not resolve outside it
  --host <x.y.z>  also check that the host API version satisfies coursive.api
  -h, --help      show this usage

Exit codes: 0 valid, 1 problems, 2 usage or unreadable input.`;

/** A usage error: the command line, not the plugin, is wrong. */
export class UsageError extends Error {}

/** An input error: the target cannot be read or is not JSON. */
export class InputError extends Error {}

/** Parses the command line; throws [UsageError] on an unknown or missing value. */
export function parseArgs(argv) {
	const opts = { target: "", json: false, files: false, host: "", help: false };
	for (let i = 0; i < argv.length; i++) {
		const arg = argv[i];
		switch (arg) {
			case "--json":
				opts.json = true;
				break;
			case "--files":
				opts.files = true;
				break;
			case "--host":
				opts.host = argv[++i] ?? "";
				if (!/^\d+\.\d+\.\d+$/.test(opts.host)) throw new UsageError("--host needs a version, for example 3.0.0");
				break;
			case "-h":
			case "--help":
				opts.help = true;
				break;
			default:
				if (arg.startsWith("-")) throw new UsageError(`unknown option ${arg}`);
				if (opts.target) throw new UsageError("validate one package at a time");
				opts.target = arg;
		}
	}
	if (!opts.target && !opts.help) throw new UsageError("name a plugin directory or a package.json");
	return opts;
}

/** A path as the user typed it: relative to the working directory when inside it. */
export const displayPath = (target, cwd = process.cwd()) => {
	const rel = relative(cwd, resolve(target));
	return rel && !rel.startsWith("..") ? rel : resolve(target);
};

/** Reads the manifest of a package directory or of a package.json path. */
export async function load(target) {
	const resolved = resolve(target);
	const info = await stat(resolved).catch(() => null);
	if (!info) throw new InputError(`${displayPath(target)}: no such file or directory`);
	const manifestPath = info.isDirectory() ? join(resolved, "package.json") : resolved;
	const packageDir = info.isDirectory() ? resolved : dirname(resolved);
	const text = await readFile(manifestPath, "utf8").catch(() => {
		throw new InputError(`${displayPath(manifestPath)}: cannot read the manifest`);
	});
	let pkg;
	try {
		pkg = JSON.parse(text);
	} catch (error) {
		throw new InputError(`${displayPath(manifestPath)}: not valid JSON: ${error.message}`);
	}
	return { manifestPath, packageDir, pkg };
}

/**
 * Checks the files a manifest names: every runtime entry, pi skill and source
 * executable must exist in the package and resolve inside it. Paths the schema
 * rules already reject are skipped, so one defect is reported once.
 */
async function checkFiles(packageDir, pkg, report) {
	const root = await realpath(packageDir).catch(() => resolve(packageDir));
	const check = async (path, at) => {
		const rel = trimDotSlash(path);
		if (!insidePackage(rel)) return;
		const full = join(packageDir, rel);
		if (!(await stat(full).catch(() => null))) {
			report.push({
				code: "file-missing",
				path: at,
				message: `${JSON.stringify(rel)} does not exist in the package`,
				hint: "Add the file, or point the manifest at a path that exists in the package.",
			});
			return;
		}
		const real = await realpath(full).catch(() => full);
		if (real !== root && !real.startsWith(root + sep)) {
			report.push({
				code: "file-escape",
				path: at,
				message: `${JSON.stringify(rel)} resolves outside the package`,
				hint: "Keep the file inside the plugin package; a symlink may not resolve outside it.",
			});
		}
	};

	const c = pkg?.coursive ?? {};
	for (const [i, runtime] of (Array.isArray(c.runtime) ? c.runtime : []).entries()) {
		for (const path of entryPaths(runtime?.entry) ?? []) await check(path, `coursive.runtime[${i}].entry`);
		for (const path of Array.isArray(runtime?.skills) ? runtime.skills : []) await check(path, `coursive.runtime[${i}].skills`);
	}
	const sources = Array.isArray(c.contributes?.sources) ? c.contributes.sources : [];
	for (const [i, source] of sources.entries()) {
		for (const path of entryPaths(source?.executable) ?? []) await check(path, `coursive.contributes.sources[${i}].executable`);
	}
}

/** Validates one package and returns the report the CLI prints. */
export async function validate(target, { files = false, host = "" } = {}) {
	const { manifestPath, packageDir, pkg } = await load(target);
	const found = problems(pkg);
	if (files && !found.some((p) => p.code === "manifest-missing" || p.code === "manifest-invalid")) {
		await checkFiles(packageDir, pkg, found);
	}
	const api = typeof pkg?.coursive?.api === "string" ? pkg.coursive.api : "";
	if (host && api && !satisfiesApi(api, host)) {
		found.push({
			code: "api-incompatible",
			path: "coursive.api",
			message: `api ${JSON.stringify(api)} is not satisfied by host ${host}`,
			hint: "Raise the host version or lower the api range the plugin declares.",
		});
	}
	return {
		ok: found.length === 0,
		target: displayPath(target),
		manifest: displayPath(manifestPath),
		schema: SCHEMA_ID,
		plugin: {
			id: typeof pkg?.coursive?.id === "string" ? pkg.coursive.id : "",
			version: typeof pkg?.version === "string" ? pkg.version : "",
			api,
		},
		checks: { files, host: host || null },
		problems: found,
	};
}

/** Renders a report for a terminal. */
export function formatReport(report) {
	const name = `${report.plugin.id || "(no id)"}@${report.plugin.version || "(no version)"}`;
	if (report.ok) return `${report.manifest}: OK — ${name}, api ${report.plugin.api || "(none)"}`;
	const lines = [`${report.manifest}: ${report.problems.length} problem${report.problems.length === 1 ? "" : "s"} in ${name}`];
	for (const p of report.problems) {
		lines.push(`  ${p.path} [${p.code}] ${p.message}`);
		if (p.hint) lines.push(`    hint: ${p.hint}`);
	}
	return lines.join("\n");
}

async function main(argv) {
	const opts = parseArgs(argv);
	if (opts.help) {
		console.log(USAGE);
		return 0;
	}
	const report = await validate(opts.target, { files: opts.files, host: opts.host });
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
			console.error(`validate-plugin: ${error.message}`);
			if (error instanceof UsageError) console.error(`\n${USAGE}`);
			process.exitCode = 2;
		},
	);
}
