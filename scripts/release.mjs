#!/usr/bin/env node
// release.mjs — the version and promotion rules of docs/release-policy.md,
// shared by the release workflows and their tests.
//
//   node scripts/release.mjs prerelease [--version X.Y.Z-pre.N]
//       The next pre-release cut from dev: the given version, or the newest
//       pre-release with its last number raised.
//   node scripts/release.mjs promote --head <sha> (--base <ref> | --merged <sha>)
//       The stable release a promotion of <head> to main publishes. <head>
//       must carry a pre-release tag whose stable version is unreleased, and
//       main after the merge (--base: the merge GitHub would make; --merged:
//       the merge commit) must hold exactly <head>'s files.
//
// Both print one JSON object: tag, version, build (the build number, one more
// than the published v* tags) and, for promote, prerelease and previous (the
// stable tag before it, for the notes). Errors exit 1 with a message.
// Needs git and every tag fetched. Zero dependencies; Node >= 22.
import { execFileSync } from 'node:child_process';
import path from 'node:path';
import process from 'node:process';
import { pathToFileURL } from 'node:url';

const SEMVER = /^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/;

export class ReleaseError extends Error {}

/** Parses X.Y.Z or X.Y.Z-pre; null when the text is not one. */
export function parse(version) {
  const m = SEMVER.exec(version ?? '');
  return m ? { core: [+m[1], +m[2], +m[3]], pre: m[4] ? m[4].split('.') : [] } : null;
}

/** Semantic-versioning precedence: negative, zero or positive. */
export function compare(a, b) {
  const x = parse(a);
  const y = parse(b);
  for (let i = 0; i < 3; i++) if (x.core[i] !== y.core[i]) return x.core[i] - y.core[i];
  if (!x.pre.length || !y.pre.length) return y.pre.length - x.pre.length;
  for (let i = 0; i < Math.max(x.pre.length, y.pre.length); i++) {
    if (x.pre[i] === undefined) return -1;
    if (y.pre[i] === undefined) return 1;
    const [p, q] = [x.pre[i], y.pre[i]];
    const [pn, qn] = [/^\d+$/.test(p), /^\d+$/.test(q)];
    if (pn && qn && +p !== +q) return +p - +q;
    if (pn !== qn) return pn ? -1 : 1;
    if (p !== q) return p < q ? -1 : 1;
  }
  return 0;
}

const stableOf = (version) => version.replace(/-.*$/, '');
const versions = (tags) => tags.filter((t) => t.startsWith('v') && parse(t.slice(1))).map((t) => t.slice(1));
const newest = (list) => [...list].sort(compare).at(-1);

/** The next pre-release given every existing tag and an optional requested version. */
export function nextPrerelease(tags, requested = '') {
  const all = versions(tags);
  const released = (v) => all.includes(stableOf(v));
  let version = requested.replace(/^v/, '');
  if (version) {
    if (!parse(version)?.pre.length) throw new ReleaseError(`${version} is not a pre-release version such as 0.2.0-beta.1`);
  } else {
    const last = newest(all.filter((v) => parse(v).pre.length));
    if (!last) throw new ReleaseError('there is no pre-release to continue; name the version, for example 0.1.0-beta.1');
    if (released(last)) throw new ReleaseError(`${stableOf(last)} is already released; name the next version, for example ${bump(stableOf(last))}-beta.1`);
    const pre = parse(last).pre;
    if (!/^\d+$/.test(pre.at(-1))) throw new ReleaseError(`${last} does not end in a number; name the version`);
    version = `${stableOf(last)}-${[...pre.slice(0, -1), String(+pre.at(-1) + 1)].join('.')}`;
  }
  if (all.includes(version)) throw new ReleaseError(`v${version} already exists`);
  if (released(version)) throw new ReleaseError(`${stableOf(version)} is already released; a pre-release must precede its stable version`);
  return { tag: `v${version}`, version, build: all.length + 1 };
}

const bump = (stable) => {
  const [x, y] = parse(stable).core;
  return `${x}.${y + 1}.0`;
};

/** The stable release promoting a commit that carries headTags, given every existing tag. */
export function promotion(headTags, tags) {
  const prerelease = newest(versions(headTags).filter((v) => parse(v).pre.length));
  if (!prerelease) throw new ReleaseError('the promoted commit carries no pre-release tag; cut a pre-release from dev first');
  const all = versions(tags);
  const version = stableOf(prerelease);
  if (all.includes(version)) throw new ReleaseError(`v${version} is already released`);
  const previous = newest(all.filter((v) => !parse(v).pre.length && compare(v, version) < 0));
  return { tag: `v${version}`, version, build: all.length + 1, prerelease: `v${prerelease}`, previous: previous ? `v${previous}` : '' };
}

const git = (...args) => execFileSync('git', args, { encoding: 'utf8' }).trim();
const lines = (text) => text.split('\n').filter(Boolean);

function main(argv) {
  const option = (name) => (argv.includes(name) ? argv[argv.indexOf(name) + 1] ?? '' : undefined);
  const tags = lines(git('tag', '--list', 'v*'));
  if (argv[0] === 'prerelease') return nextPrerelease(tags, option('--version') ?? '');
  if (argv[0] === 'promote') {
    const head = option('--head');
    const base = option('--base');
    const merged = option('--merged');
    if (!head || (base === undefined) === (merged === undefined)) throw new ReleaseError('promote needs --head and one of --base or --merged');
    const tree = merged ? git('rev-parse', `${merged}^{tree}`) : git('merge-tree', '--write-tree', base, head);
    if (tree !== git('rev-parse', `${head}^{tree}`)) {
      throw new ReleaseError(`main after the merge would differ from ${head}; main may only change by promoting dev`);
    }
    return promotion(lines(git('tag', '--points-at', head, '--list', 'v*')), tags);
  }
  throw new ReleaseError('usage: release.mjs prerelease [--version V] | promote --head SHA (--base REF | --merged SHA)');
}

const isMain = process.argv[1] && pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url;
if (isMain) {
  try {
    console.log(JSON.stringify(main(process.argv.slice(2))));
  } catch (error) {
    console.error(`release: ${error.message}`);
    process.exit(1);
  }
}
