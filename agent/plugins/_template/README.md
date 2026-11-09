# Plugin template (manifest v3)

The full author guide, including slots, the action bridge, services, the simulator and migration from v1, is [plugin-v3-guide.md](../../../docs/plugin-v3-guide.md); [Review Lab](../_examples/review-lab/README.md) is a complete example.

A copyable starting point for a Coursive plugin package: one declarative page, one course-detail slot, one action and one Pi runtime module, declaring only the permission it uses.

It is not an installed plugin. The host installs official plugins listed in [catalog.json](../catalog.json) and third-party packages from a folder, an npm package or a Git tag, so this directory is inert until you copy it out and install the copy. Keep the `_` prefix if you add other examples here: the catalog, not the directory listing, decides what exists.

- `package.json`: the `coursive` manifest — identity, API range, runtime, optional dependency, permission, settings form and contributions.
- `index.mjs`: the `pi` runtime entry — `activate(coursive)` registers the action; the default export runs per conversation and stays side-effect free.
- `README.md`: this guide.

Validate a copy before installing it:

```sh
node agent/scripts/validate-plugin.mjs agent/plugins/_template --files --host 3.0.0
```

## The manifest

| Field | Rule |
|---|---|
| `version` | Semver of the plugin itself, for example `0.1.0` |
| `coursive.id` | 2–32 lowercase letters, digits or dashes, starting with a letter; also the install directory name and the tool-name prefix |
| `coursive.title`, `coursive.summary` | 1–24 and 1–80 characters; the app shows them before installation |
| `coursive.api` | The plugin API range you build against: `^x.y.z`, `>=x.y.z` or an exact version |
| `coursive.runtime` | One entry per executor: `kind` is `pi`, `moejs`, `process`, `web-ui` or `go`, `entry` is a package-relative path or a platform map, `optional` keeps the other components when this one cannot run here |
| `coursive.requires` | Services that must resolve before contributions activate: `{service: "domain.verb", version: "^1"}`, `optional: true` for a documented degraded mode |
| `coursive.provides` | Services this plugin offers: `{service: "domain.verb", version: "1.0.0"}`, exact semver |
| `coursive.contributes` | Groups: `navigation`, `pages`, `slots`, `tools`, `views`, `actions`, `settings`, `sources` |

Every path is package-relative: no absolute path, drive letter, backslash, `..` or dot segment. A path that escapes the package is rejected at install, not at first call.

The shape is [plugin-manifest.v3.schema.json](../../../docs/schemas/plugin-manifest.v3.schema.json); the contract is [spec 0031](../../../docs/specs/0031-plugin-fabric-core.md). The daemon, the assistant host and the app validate the same shared fixtures, so a manifest one of them accepts is accepted by all three.

## Permissions

Ask for the least you use; the user sees every permission before installing, and a call outside them fails. A permission increase after installation is a new consent step.

| Permission | Allows |
|---|---|
| `library.read` | Read courses, lectures, notes and the library index |
| `library.write` | Write notes, practice records and the schedule through the domain write guard |
| `views` | Return interactive views in a conversation |
| `localhost` | Call a local program over loopback HTTP |
| `exec` | Start the commands listed in `coursive.exec` |
| `workspace` | Create a working directory inside the library |
| `network` | Reach the hosts listed in `coursive.hosts` |
| `folder` | Read and write a folder the user picks |
| `source` | Offer a content source through a provider executable |

`exec` and `network` are paired with their detail: `"permissions": ["exec"]` also needs `"exec": ["/usr/bin/ffmpeg"]`, and `"permissions": ["network"]` needs `"hosts": ["example.org"]`. This template declares only `library.read`.

## Page, slot and action

A page declares `id`, `title`, `renderer` and a `document`. The renderer is the host's declarative one: the plugin sends a data tree and ships no Dart or UI code. The versioned whitelist is `column`, `row`, `card`, `text`, `button`, `input`, `tabs`, `list`, `table`, `chart`, `timeline`, `math`, `mediaRef`, `sourceRef` and `canvasGraph`, and a document is `{"schemaVersion": 1, "body": {…}}`. A `button` carries `action` and optional `args`; the app sends them to the action bridge, which checks the plugin state, the permission and the approval mode before the runtime sees them.

A slot entry adds `target` (a host slot such as `course.detail.sections` or `canvas.toolbar.tools`), `mode` (`list`, `single`, `keyed`, `chain`) and `order`, plus the same document. Keyed slots reject duplicate keys instead of overwriting; the slot catalog is [spec 0032](../../../docs/specs/0032-ui-extension-protocol.md).

An action is the runtime function the page, slot or composer calls. `activate(coursive)` registers it with `coursive.action("hello", handler)`; the manifest lists it under `contributes.actions` so the app can offer it. The daemon's action bridge calls it as `handler(args, {state, viewId, instanceId, signal})` after checking the plugin state, the permission, `params` and the view revision, and it answers with exactly one of `notification`, `statePatch`, `navigation` or `viewPatch` ([spec 0032](../../../docs/specs/0032-ui-extension-protocol.md)); a `write` action never runs and becomes a pending proposal. Disabling the plugin disposes the registration.

## Migrating a v1 manifest

A v1 package is not silently accepted: each old field is rejected with its v3 home.

| v1 | v3 |
|---|---|
| `coursive.engine` | `coursive.api` (same range syntax) |
| `coursive.agent` | `coursive.runtime[{kind: "pi", entry}]` |
| `coursive.skills` | `coursive.runtime[{kind: "pi", skills}]` |
| `coursive.views` | `coursive.contributes.views` (strings become entries with an `id`) |
| `contributes.noteViews`, `exporters` | `contributes.actions` with the note surface |
| `contributes.blocks` | `contributes.views` |

The full field map and the reasons are in [plugin-fabric-audit.md](../../../docs/plugin-fabric-audit.md).

## Compatibility and QA

- **API Version 3.0.0**: the manifest shape `coursive/plugin-manifest@3`, which this template declares as `"api": "^3.0.0"`. A v3 reader accepts exactly this shape.
- **Minimum Host Version**: the first host release that reads v3 manifests. The assistant host reads them with API 3.0.0 ([decision 0034](../../../docs/decisions/0034-v3-manifests-install-in-the-assistant-host.md)): it runs the shared rules, then its own — one required `pi` runtime, declarative pages, slot targets from its catalog, button actions that name a declared action, and no `contributes.sources`. Source plugins still declare v2 `engine` (`2.0.0`). `--host <version>` in the validator answers "would this host accept this range" ahead of time.
- **coursive-plugins counterpart**: that repository owns the published templates and the index. Its `templates/agent-plugin` and `templates/source-plugin` still write v2 `engine`, and `scripts/validate.mjs` validates `index.json` and `mirrors.json`. When the v3 store lands, its templates move to `api`/`runtime`, and both validators run the shared fixtures ([cases.json](../../../daemon/internal/pluginfabric/testdata/cases.json)) so an index entry and a core install cannot disagree.

## Validation CLI

`agent/scripts/validate-plugin.mjs` takes a package directory or a `package.json`, checks the manifest rules and prints one line per problem with the JSON path, a stable code and a recovery hint.

| Option | Effect |
|---|---|
| `--json` | Print `{ok, manifest, schema, plugin, checks, problems}` on stdout for CI |
| `--files` | Also require every declared entry, skill and source executable to exist inside the package and resolve inside it |
| `--host x.y.z` | Also check that the host API version satisfies `coursive.api` |
| `--help` | Usage |

Exit codes: `0` valid, `1` problems, `2` usage or unreadable input. From `agent/`, `npm run validate:plugin -- plugins/_template --files` is the same check.
