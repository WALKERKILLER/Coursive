#!/usr/bin/env node
// check-brand-assets.mjs — the brand exports match assets/brand/exports.json.
// Fails when the master changed without a re-export, an export was edited by hand,
// or a file was added or removed outside scripts/export-brand-icons.sh.
// Zero dependencies; Node >= 18.
import { createHash } from 'node:crypto';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

const brand = path.resolve(process.argv[2] ?? 'assets/brand');
const sha256 = async (rel) => createHash('sha256').update(await readFile(path.join(brand, rel))).digest('hex');

async function walk(rel) {
  const out = [];
  for (const entry of await readdir(path.join(brand, rel), { withFileTypes: true })) {
    const child = path.posix.join(rel, entry.name);
    if (entry.isDirectory()) out.push(...await walk(child));
    else out.push(child);
  }
  return out;
}

const errors = [];
const record = JSON.parse(await readFile(path.join(brand, 'exports.json'), 'utf8'));
if (await sha256(record.source.path) !== record.source.sha256) {
  errors.push(`${record.source.path} changed; run scripts/export-brand-icons.sh`);
}
const listed = new Map(record.files.map((file) => [file.path, file.sha256]));
for (const [rel, hash] of listed) {
  try {
    if (await sha256(rel) !== hash) errors.push(`${rel} differs from the export; regenerate instead of editing it`);
  } catch {
    errors.push(`${rel} is missing`);
  }
}
for (const dir of ['logo', 'platforms', 'social']) {
  for (const rel of await walk(dir)) if (!listed.has(rel)) errors.push(`${rel} is not produced by the export script`);
}
if (errors.length) {
  for (const error of errors) console.error(`check-brand-assets: ${error}`);
  process.exit(1);
}
console.log(`check-brand-assets: OK (${listed.size} exports)`);
