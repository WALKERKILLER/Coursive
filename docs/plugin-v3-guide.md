# Writing a v3 plugin

This guide takes a v3 plugin from an empty folder to a package that validates, runs in the simulated host, installs and works in the app. The contracts are [spec 0031](specs/0031-plugin-fabric-core.md) (manifest, identity, services) and [spec 0032](specs/0032-ui-extension-protocol.md) (pages, slots, declarative documents, the action bridge). The worked example is [Review Lab](../agent/plugins/_examples/review-lab/README.md); the smallest starting point is [_template](../agent/plugins/_template/README.md). v1 plugins keep [spec 0004](specs/0004-plugin-platform.md) and the [v1 guide](../agent/plugins/README.md).

## 1. The package

A plugin is an npm-style folder whose `package.json` carries a `coursive` key:

```json
{
  "name": "coursive-plugin-review-lab",
  "version": "0.1.0",
  "type": "module",
  "files": ["index.mjs", "README.md"],
  "coursive": {
    "id": "review-lab",
    "title": "复习卡实验室",
    "summary": "把课程要点做成复习卡",
    "api": "^3.0.0",
    "runtime": [{ "kind": "pi", "entry": "index.mjs" }],
    "permissions": ["library.read", "library.write"],
    "provides": [{ "service": "review.generate", "version": "1.0.0" }],
    "contributes": { "navigation": [], "pages": [], "slots": [], "actions": [] }
  }
}
```

- `id`: 2–32 lowercase letters, digits or `-`, starting with a letter. It prefixes the plugin's tools (`review-lab__make_cards`) and namespaces its storage; it never changes across versions.
- `api`: the manifest API range the package is built against (see [Compatibility](#7-compatibility-and-deprecation)).
- `runtime`: exactly one required `pi` component with its `entry` module and optional `skills` folders. Other kinds (`moejs`, `web-ui`, `process`, `go`) must be marked `"optional": true`; this host does not run them.
- `files`: what `npm pack` ships. The runtime entry must be in it; the simulator's `--pack` refuses a tarball without it.
- `requires` / `provides`: services the plugin needs or offers, each `{service, version}` (`optional` on `requires`). The capability inspector shows them to the assistant; the daemon resolver picks providers.

The [schema](schemas/plugin-manifest.v3.schema.json) closes every field set. An unknown field, a v1 field (`engine`, `agent`, `skills`), a path outside the package, a duplicate contribution id or an undeclared permission is a problem with a JSON path, a code and a hint.

## 2. Permissions

| Permission | Grants |
|---|---|
| `library.read` | `coursive.daemon.get` on the course, lecture, note, quiz, activity and schedule routes |
| `library.write` | `coursive.daemon.put/post/patch/delete` on notes, quizzes, bookmarks, timeline and schedule; required by every `write` action |
| `views` | interactive views in chat results (`contributes.views`) |
| `localhost` | `coursive.fetch` to ports named in the plugin's settings |
| `network` | `coursive.fetch` to the hosts listed in `hosts` |
| `exec` | `coursive.spawn` for the commands listed in `exec` |
| `workspace` | a working folder under the library |
| `folder` | a folder the user picked in the plugin's settings |

The user grants all declared permissions at install; a version that adds one waits in 需要授权 until the user grants it again. The daemon's action bridge re-checks the permission an action names on every call, and on approval of a write.

## 3. Pages, navigation and slots

A page is a declarative document the app renders natively; the plugin ships no UI code.

```json
"pages": [{ "id": "deck", "title": "复习卡", "icon": "style", "renderer": "declarative",
  "document": { "schemaVersion": 1, "body": { "type": "column", "children": [
    { "type": "text", "variant": "title", "text": "${front}" },
    { "type": "button", "text": "下一张", "action": "next", "args": { "step": 1 } }
  ] } } }],
"navigation": [{ "id": "deck", "title": "复习卡", "page": "deck", "order": 50 }]
```

- Node types are the DSL v1 whitelist of spec 0032 (`column`, `row`, `card`, `text`, `button`, `input`, `list`, `table`, `tabs`, `timeline`, …). Limits: depth 24, 2000 nodes, 20 000 characters per text, 256 KB per document.
- `"${path}"` binds a value of the view state (and, in a slot, `context.*`); a binding with no value renders empty.
- Every `button.action` must name a declared action; the installer refuses one that does not.
- A slot puts a document on a host surface. The host catalog is `course.detail.sections` (mode `list`), with `context.course_id`, and `canvas.toolbar.tools` (mode `list`), shown in the study canvas's 插件 sheet with `context.lecture_id`, `context.canvas_revision`, `context.canvas_x`/`canvas_y` (the view centre in canvas units) and `context.canvas_objects` (`[{id, revision}]` of this plugin's objects only). Other targets are refused at install until the app ships their outlet.
- A button binds a context value as an argument with `{"ref": "context.canvas_revision"}`.

Disabling or uninstalling the plugin removes its pages, navigation and slots, and an open page closes itself.

## 4. Actions and the bridge

```json
"actions": [
  { "id": "next", "title": "下一张", "action": "next", "params": { "properties": { "step": "integer" } } },
  { "id": "save", "title": "存为笔记", "action": "save", "permission": "library.write", "write": true }
]
```

`activate(coursive)` registers the handler with `coursive.action(name, handler)`. A document button reaches it through the daemon's action bridge, which first checks the plugin state, the permission, `params` (`required`, `properties` typed `string | number | integer | boolean | object | array`, `additional`), the view revision and the deadline. The handler gets `(args, {state, viewId, instanceId, signal, approved})` and answers with exactly one of:

- `{statePatch: {viewId, patch}}`: top-level keys of the view state; `null` removes a key. The daemon keeps the state and its revision; a stale button is refused and the page reloads.
- `{navigation: {route, title?}}`: a page id of the same plugin, or a registered route.
- `{notification: {text, level?}}`: `info`, `warning` or `error`.
- `{viewPatch: {viewId, patch}}`. On the canvas, `patch.canvas` is a canvas batch `{revision, objects?, delete?, label?}`: `revision` is the `context.canvas_revision` the tool read, each object is a `coursive/canvas-object@1` created by this plugin (a create at `revision` 1, an update exactly one above the stored object), and `delete` names this plugin's objects. The canvas checks the batch whole, shows an overwrite or delete to the user first, and commits it as one undo entry; a stale revision, an invalid object or a foreign id commits nothing. Review Lab's 钉到画布 and 移除复习卡 are the example.

A `write` action never runs when the button is pressed. The bridge stores a pending proposal with the assistant's approval mode; the app asks 允许「…」吗？, and only 批准写入 runs the handler, with `approved` set to the proposal id. 放弃 runs nothing. A proposal is applied at most once.

An action with a v1 `surface` (`composer.plus`, `note.toolbar`, …) still runs through the v1 route from that menu and answers a v1 result (`toast`, `view`, `prompt`, `open`).

## 5. Services and tools

The default export of the `pi` entry runs for every new conversation and registers agent tools and hooks with Pi's extension API:

```js
export default function (pi, coursive) {
  pi.registerTool({
    name: "make_cards",
    label: "生成复习卡",
    description: "Replaces the deck with question/answer cards …",
    parameters: { type: "object", properties: { cards: { type: "array" } }, required: ["cards"] },
    execute: async (_id, { cards }) => { await coursive.storage.set("deck", cards); return { content: [{ type: "text", text: "…" }], details: {} }; },
  });
}
```

Tools are exposed as `<id>__<name>`, loaded on demand through `find_tools`, and count against the conversation's tool budget; scheduled tasks get only read-only plugin tools. `provides` lists the services the plugin implements so the capability inspector can name it as a provider; `contributes.tools` may name the services its tools expose.

## 6. Validate, simulate, pack

```sh
node agent/scripts/validate-plugin.mjs my-plugin --files          # the manifest rules
node agent/scripts/plugin-sim.mjs my-plugin                        # install in a simulated host, run sim/cases.json
node agent/scripts/plugin-sim.mjs my-plugin --pack dist            # then build the npm tarball
```

`plugin-sim.mjs` installs the package into a real `PluginHost` over a temporary folder with a fake daemon, checks that every declared action has a handler, and runs the cases in `sim/cases.json`:

```json
{
  "daemon": { "PUT /api/notes": { "id": "n-1", "title": "复习卡" } },
  "cases": [
    { "name": "start", "action": "start", "view": "deck", "expect": { "statePatch": { "patch": { "index": 0 } } } },
    { "name": "save asks first", "action": "save", "expect": { "proposal": { "actionId": "save" } }, "calls": [] },
    { "name": "approved save", "action": "save", "approve": true, "calls": ["PUT /api/notes"] },
    { "name": "tool", "tool": "make_cards", "params": { "cards": [{ "front": "Q", "back": "A" }] }, "expect": { "details": { "cards": 1 } } }
  ]
}
```

`expect` matches a subset of the result; `calls` lists the daemon routes the case must call, in order; state patches are carried between cases per view. CI runs the simulator and `--pack` on every v3 package in this repository, and [review-lab-e2e.mjs](../agent/scripts/review-lab-e2e.mjs) runs Review Lab against a real daemon and assistant host.

To try a package in the app, turn on 允许第三方插件 and install it from its folder, npm or Git.

## 7. Compatibility and deprecation

[Decision 0035](decisions/0035-plugin-api-versions-and-deprecation.md) is the policy:

- **API Version** `3.0.0` is the manifest shape this guide describes. A package declares the range it builds against in `coursive.api` (`^3.0.0`); a host refuses a package whose range it does not satisfy, with the reason.
- **Minimum Host Version** for v3 packages is the first release that reads v3 manifests in the assistant host (#56). `validate-plugin.mjs --host x.y.z` answers whether a host would accept a range.
- Within a major version, changes only add optional fields, node types, slot targets or permissions. Removing or changing the meaning of a field takes a new major.
- v1 (`engine: ^1.0.0`) packages are deprecated, not removed. They keep installing and running until v1 support is removed in a major host release announced at least one minor release ahead, with the validator naming the v3 home of every v1 field.

## 8. Migrating a v1 plugin

The field map is in the [audit](plugin-fabric-audit.md#compatibility). In short:

1. Replace `engine` with `api: "^3.0.0"`, and `agent`/`skills` with `runtime: [{kind: "pi", entry, skills}]`.
2. Move `views` to `contributes.views` entries with an `id`.
3. Keep surface actions as they are (`contributes.actions` with `surface`); their handlers still answer v1 results.
4. Turn a v1 page (`pages` with `action`) into a declarative `document` whose buttons name actions answering bridge results.
5. Mark every action that writes with `write: true` and `permission: library.write`, and make it check `approved`.
6. Keep `noteViews`, `exporters` and `blocks` on v1 for now: v3 has no host for them yet.
7. Run `validate-plugin.mjs`, then `plugin-sim.mjs` with cases for each action.
