# Plugin marketplace and mirrors

Artifact-Version: 1
Status: Draft

## Intent

Plugins are how Coursive grows: assistant tools and skills, views, actions, and now content sources ([source-provider plugins](draft-source-provider-plugins.md)). Today the plugin store lives in the assistant host, so nothing can be installed until the Node.js runtime exists. Catalogs are plain JSON fetched straight from GitHub, which is slow or unreachable on many networks.

Outcome:

- One plugin store, owned by the daemon, that installs every contribution kind.
- A marketplace fed by a signed index from [coursive-plugins](https://github.com/WALKERKILLER/coursive-plugins).
- Downloads through several GitHub mirrors, with a speed test that picks the fastest one. A mirror can make a download faster but can never change its content.

Scope:

- Plugin store ownership and the manifest.
- The index format and its signature.
- Mirrors, the speed test and download fallback.
- The market and mirror UI.

Non-goals:

- Paid plugins or accounts.
- Automatic updates without the user's consent.
- Plugin sandboxing beyond the permission model.
- Hosting a mirror.

Constraints:

- GET requests only toward mirrors, with no credentials, cookies or tokens.
- Every artifact is verified before it is unpacked.

Comparable designs reviewed:

- LobeHub's plugin index: a single `index.json` with a schema version and per-plugin manifest URLs, validated in CI.
- Obsidian's community list: a JSON list in a central repository, files taken from GitHub releases whose tag equals the manifest version, compatibility in `versions.json`.
- MCP Bundles: a zip with a `manifest.json`, one-click install into a per-extension directory, optional signatures.
- Raycast: assistant tool calls ask for approval by default.

The links are listed under Sources.

## Contract

### Store

- The daemon owns plugin state (`plugins.json`, mode 0600), installs, consent, enablement and settings. The app and the assistant host read it through the daemon API.
- Each contribution runs in its owner's runtime:
  - agent tools, skills and hooks in the assistant host, which loads the enabled agent plugins the daemon lists;
  - sources in the daemon;
  - views and actions are rendered by the app.
- A plugin whose contributions need a missing runtime installs normally and shows 「需要先安装助手运行环境」 on those contributions.
- Manifest: the `coursive` key in `package.json`. It keeps the current plugin fields and permissions, adds `contributes.sources` and the `source` permission, and sets `engine` against plugin API 2.0.0. Version 1 manifests are not loaded.

### Index

`index.json` is published from coursive-plugins with a detached signature `index.json.sig` (Ed25519, minisign format):

```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-11-06T00:00:00Z",
  "plugins": [{
    "id": "mindmap",
    "title": "Mind map",
    "summary": "Turn a lecture outline into an interactive mind map",
    "author": "WALKERKILLER",
    "homepage": "https://github.com/WALKERKILLER/coursive-plugins/tree/main/plugins/mindmap",
    "categories": ["visualize"],
    "kinds": ["agent", "views"],
    "official": true,
    "versions": [{
      "version": "1.0.0",
      "engine": "^2.0.0",
      "published": "2026-11-06",
      "artifacts": [{
        "platform": "any",
        "url": "https://github.com/WALKERKILLER/coursive-plugins/releases/download/mindmap-v1.0.0/mindmap-1.0.0.tgz",
        "sha256": "<64 hex>",
        "size": 18342
      }]
    }]
  }]
}
```

- `platform` is `any` or one of the source executable keys. An install picks the newest version whose `engine` matches and whose artifacts cover this device.
- Artifact URLs point at `github.com` releases or `raw.githubusercontent.com`. Any other host is refused unless it comes from a user-added catalog.
- The app pins the index public keys. An index whose signature fails is ignored, and the last verified copy (kept up to 30 days) is shown with 「无法验证插件目录」.
- User-added catalogs (HTTPS URL or `owner/repo`) use the same format without a signature. Their plugins are marked third-party and installable only with 允许第三方插件 on.

### Mirrors

- A mirror is `{id, name, template, kinds}`:
  - `template` must be HTTPS and contain `{url}`, the full GitHub URL, or `{path}`, the URL without its scheme and host;
  - `kinds` is a subset of `raw`, `release` and `archive`.
- The built-in list is the mirror list published in coursive-plugins, signed like the index, plus `direct` (no rewrite). Users can add, disable and reorder mirrors in 插件 → 下载线路.
- Speed test (「测速」, on first use, and when the cached result is older than 24 h):
  - fetch the probe file `probe-256k.bin` through every enabled mirror, four at a time;
  - each probe has a 6 s deadline and stops after 256 KiB;
  - the score is the time to the full probe with the SHA-256 checked. A failure, a timeout or a hash mismatch puts the mirror last for this result.
- Selection is 自动 (best score first) or one mirror chosen by the user. Each download tries the selected order and falls back to the next mirror on a network error, an HTTP error or a hash mismatch, ending with `direct`.
- Results show each mirror's latency and throughput, the time of the test, and which one is in use.

### Install

1. Resolve the version and artifact.
2. Download through the mirror order into a staging directory, verify `size` and `sha256`, and unpack, refusing absolute paths, `..` and links that escape.
3. Validate the manifest against the index entry (`id`, `version`).
4. Show consent: permissions, hosts, kinds, publisher and source.
5. Move the plugin into place and record it.

Updates show the new version and any added permissions, and wait for the user. Uninstall removes the package, its storage and its vault entries.

## Plan

1. Daemon:
   - plugin store and state migration from the assistant host;
   - index fetch and signature check;
   - mirror list, speed test and download fallback;
   - install pipeline;
   - routes under `/api/plugins`.
2. Assistant host: load enabled agent plugins from the daemon's list; remove its own install and catalog code.
3. App:
   - market (browse, search, categories, detail);
   - consent and updates;
   - 下载线路 page with 测速.
4. coursive-plugins:
   - index and mirror schemas, validator and CI;
   - signing in the release workflow;
   - the probe file in a `probe` release.

## Verification

- Daemon tests with an `httptest` server standing in for GitHub and three mirrors:
  - ranking by speed;
  - fallback on error and on a tampered artifact;
  - no `Authorization` or `Cookie` header on any mirror request;
  - a bad index signature;
  - unpack escapes;
  - engine and platform selection.
- Assistant-host tests: only enabled agent plugins from the daemon list load.
- Widget tests: market list and detail, consent showing permissions and hosts, the 下载线路 page with results and manual selection.
- The coursive-plugins CI rejects an index entry whose artifact hash does not match the release asset.

## Evidence

Pending until implementation.

## Open questions

1. Signing key custody: a GitHub Actions secret in coursive-plugins (the maintainer creates it) or offline signing by the maintainer before each index publish.
2. Whether the built-in mirror list may be refreshed from the signed list at run time, or changes only with app releases.

## Sources

- LobeHub plugin index — https://github.com/lobehub/lobe-chat-plugins (accessed 2026-11-06)
- Obsidian community plugin releases and submission — https://github.com/obsidianmd/obsidian-releases and https://docs.obsidian.md/Plugins/Releasing/Submit+your+plugin (accessed 2026-11-06)
- MCP Bundles format and CLI — https://github.com/modelcontextprotocol/mcpb and https://blog.modelcontextprotocol.io/posts/2025-11-20-adopting-mcpb/ (accessed 2026-11-06)
- Raycast AI tool approval — https://aiagents.wiki/agents/raycast-ai (accessed 2026-11-06)
- GitHub mirror availability tests — https://zachthinking.github.io/posts/github-proxy-mirror/ and https://www.chonglangbiji.com/security/github-release-download-mirror-speed-comparison-may-2026/ (accessed 2026-11-06)
