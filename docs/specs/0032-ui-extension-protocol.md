# UI Extension Protocol: navigation, slots, declarative UI, canvas, and Web UI

Artifact-Version: 1
Status: Approved
Approved-By: maintainer, explicit request on 2026-11-09 to adopt the Plugin Fabric RFC

## Intent

Plugins need to add complete pages, decorate existing surfaces and render rich results without forking the Flutter shell or shipping arbitrary Dart code. This spec owns the observable UI contract: navigation and page registration, typed UI slots, the declarative UI DSL, the action bridge, canvas objects and fallbacks, and the optional isolated Web UI renderer. The core capability and lifecycle contract is [0031](0031-plugin-fabric-core.md); the design rationale is in [plugin-fabric.md](../plugin-fabric.md); the durable choice is [decision 0033](../decisions/0033-plugin-fabric-and-ui-extension-protocol.md).

Non-goals: injecting third-party Flutter widgets or Dart source; replacing the application shell or its authorization rules; exposing arbitrary `BuildContext`, app state or database handles to a plugin; making WebView the default renderer; requiring a WebView on a platform that cannot support one safely.

## Contract

### Navigation and pages

- A page contribution declares `id`, `pluginId`, `titleKey`/`title`, `icon`, `route`, `renderer`, `minHostVersion`, `availableOn`, `requiredServices`, `requiredPermissions`, `order`, `keepAlivePolicy` and `fallbackRoute`.
- Desktop NavigationRail may pin a small number of plugin pages and folds the rest into an extension area. Mobile keeps its fixed bottom navigation and places plugin pages in one extension center with a small number of user-pinned shortcuts.
- New navigation uses stable string route ids. The existing integer `Tabs` indices remain readable for compatibility; plugin pages are never inserted into fixed indices in a way that shifts existing callers.
- Disabling, uninstalling or losing a dependency exits the current plugin page with its normal animation, returns to the most recent valid host page, cancels subscriptions and actions, and optionally restores a bookmark. It never leaves a blank or unclosable Navigator.
- A page that fails to render shows an isolated error page, not a white screen.

### Typed UI slots

- Slot modes are `list`, `single`, `keyed`, `chain` and `hoc/decorate`. `list` sorts by `order`, `priority` and `pluginId`; `single` resolves conflicts explicitly and keeps a host fallback; `keyed` rejects duplicate keys instead of silently overwriting; `chain` falls back to the original component on timeout.
- The first slot catalog is `app.navigation.entries`, `app.main.pages`, `course.detail.sections`, `course.header.actions`, `player.controls.trailing`, `player.transcript.after`, `canvas.toolbar.tools`, `canvas.card.actions`, `canvas.card.renderer`, `canvas.overlay`, `assistant.message.after`, `assistant.tool.result`, `assistant.composer.actions` and `settings.sections`.
- A slot scope is global, course, lecture, canvas document, conversation or selection. The host passes only the context that slot needs, such as `courseId`, `lectureId`, `objectIds`, `selectionRange` and `sourceRefs`; it never passes a database connection, credentials or the whole app state.
- Every registration returns a `dispose()` handle. Disabling or upgrading a plugin revokes its slots, subscriptions, timers and in-flight work. All slot declarations from one activation commit atomically after schema, version, permission and resource checks.

### Declarative UI DSL v1

- The host renders a data component tree; plugins do not submit Dart source. The versioned whitelist is `column`, `row`, `card`, `text`, `button`, `input`, `tabs`, `list`, `table`, `chart`, `timeline`, `math`, `mediaRef`, `sourceRef` and `canvasGraph`.
- Styles reference host design tokens such as `surface`, `primary` and `spacing.md`. The renderer accepts no arbitrary CSS, expressions or JavaScript `eval`; data binding uses structured object-path references.
- An action node sends a structured action descriptor to the action bridge. The renderer supports theme, locale, accessibility semantics, reduced motion, multi-size layouts, input methods and stylus input.
- The contract defines limits for tree depth, node count, text length, payload size, render frequency, virtual-list items and subscriptions. An over-limit payload is rejected with a diagnosable error; it is never silently truncated into a misleading view.
- An invalid DSL, unknown component, recursion overflow or bad action returns an isolated error boundary and leaves the host page usable.

### Action bridge and UI state

- The app sends `{pluginId, actionId, viewId, instanceId, args, contextRevision, requestId}` with `deadlineMs` and `traceId`. The daemon verifies plugin state, permission, target revision, parameter schema and the user's approval mode before executing.
- A response carries `ok` plus `result`, or `error:{code,message,details}`. State changes carry a `revision` and an idempotency key. The allowed result forms are `statePatch`, `navigation`, `viewPatch`, `notification` and `proposal`.
- Local temporary UI state belongs to Flutter; reusable plugin data belongs to the daemon's plugin namespace; course, note and canvas data stay in the existing domain APIs.
- Optimistic UI is allowed, but a write commits only after the daemon response. Repeated clicks, retries and replays do not double-write. Cancel and timeout leave no partial result. A stale target revision is rejected with a recoverable error.

### Canvas plugin objects

- Persistent plugin output uses one versioned `CanvasObject` contract: `id`, `type`, `schemaVersion`, `geometry`, `content`, `sourceAnchors`, `createdBy`, `revision` and `interactionPolicy`.
- A source anchor can point at a lecture interval, a PDF page and range, a courseware file, a transcript segment or a handwriting stroke.
- Plugins extend the canvas through tools, renderers, actions, overlays and edges. Overlays cannot steal stylus hit-testing unless the user explicitly switches tools.
- Every persistent change enters the existing canvas transaction and undo stack. Overwriting, deleting or editing provenance requires a preview and the host approval mode. A failure, cancel or revision mismatch commits nothing.
- An object whose plugin is missing renders through a fallback that shows its title, source, text summary and exportable data. Uninstalling a plugin never deletes the object or its references.

### Isolated Web UI (L3)

- A Web UI plugin ships static HTML/CSS/JS resources. Install verifies the signed index and artifact hash; resources live in a read-only package directory with no path escape.
- The page runs in a plugin-specific origin (a controlled scheme or an isolated local read-only origin). It never receives the daemon token, cookies, storage endpoints or credentials.
- A strict CSP disables unneeded window opening, navigation, downloads, clipboard and device permissions. External resources are denied by default; approved network requests go through a daemon broker.
- URL schemes, external links, iframes, WebSockets, mixed content and deep links are restricted. `127.0.0.1` is never trusted as a whole.
- The native bridge verifies `origin + pluginId + instanceId + nonce + schemaVersion + requestId`; every command checks permission independently. The bridge never exposes a wildcard proxy to arbitrary daemon HTTP routes.
- A write returns a pending Proposal and commits only after user authorization. A forged, replayed or cross-origin message is rejected.
- Platform admission is explicit: Android and macOS are validated first; Windows and Linux require a measured WebView engine, packaging, hardware-acceleration, input-method, stylus and window-layer review. A platform without stable WebView shows a native fallback page and keeps L1/L2 working.

## Plan

- PF-03 adds navigation and page registration. PF-04 adds the slot registry and lifecycle. PF-05 adds the declarative renderer. PF-06 adds the action bridge. PF-10 adds canvas objects and fallback rendering. PF-11 prototypes the isolated Web UI. PF-12 adds the SDK, templates and QA.
- Start with the Review Lab MVP: a declarative page, a course-detail slot, a note-toolbar action, an optional service dependency and a saved review card. Add MoeJS and Web UI only after the native loop is proven.
- A daemon route or JSON-field change updates both the app and the agent host in the same change. Keep the existing plugin view catalog working during the migration.

## Verification

- Commands: `cd app && flutter test && flutter analyze --no-fatal-infos`; `cd daemon && go test -count=1 -race ./... && test -z "$(gofmt -l .)" && go vet ./...`; `cd agent && npm test && npm run check`; `node scripts/run-gates.mjs`.
- Scenarios: NAV-01/02 (install adds a page, hot enable/disable, mobile extension center); SLOT-01/02 (stable ordering, independent unload, keyed/chain fallback); UI-01/02 (invalid DSL, depth limit, unknown component, theme, size, accessibility, reduced motion, input method); DATA-01/02 (canvas objects survive plugin removal, fallback readable, undo, no partial commit); WEB-01/02 (token and loopback isolation, forged/replayed messages, cross-site navigation); REG-01 (existing plugin, assistant and source-provider paths).
- A malicious Web UI cannot read the daemon token or reach loopback without authorization. A platform without a stable WebView still runs the local library and the native plugin surfaces.

## Evidence

Pending: the canvas plugin object contract and its transaction tests (PF-10), and the platform Web UI renderer with its device matrix (PF-11). Link them here when they exist.

- Action bridge: [bridge.go](../../daemon/internal/pluginbridge/bridge.go) and [contract.go](../../daemon/internal/pluginbridge/contract.go) answer a typed request with one result form or an error, checking plugin state, the declared permission, the argument shape, the view revision, the deadline and the approval mode; [bridge_test.go](../../daemon/internal/pluginbridge/bridge_test.go) covers validation, permission denial, a stale revision, an idempotent replay (sequential and concurrent), a deadline and a cancellation that write nothing, the result forms and the proposal flow. The route is `POST /api/plugin-actions` in [plugin_actions.go](../../daemon/internal/server/plugin_actions.go) with [plugin_actions_test.go](../../daemon/internal/server/plugin_actions_test.go), which also pins the refusal when no registry is wired. The app client is [plugin_actions.dart](../../app/lib/features/plugins/plugin_actions.dart) with [plugin_actions_test.dart](../../app/test/plugin_actions_test.dart): request encoding, dispatch of each result form, the error surface and one send per request id. The Plugin Fabric registry and the Pi action executor are not wired yet, so the route answers `unavailable`; Pi plugin actions keep their own `/agent/plugins/{id}/actions/{name}` path ([0004](0004-plugin-platform.md)).
- Isolated Web UI contract: [webui.go](../../daemon/internal/pluginfabric/webui.go) with [webui_test.go](../../daemon/internal/pluginfabric/webui_test.go) — the plugin-specific origin `coursive-plugin://<pluginId>`, so one plugin cannot read another page, and a loopback HTTP origin is refused as a page session rather than trusted; the strict CSP (no external source, `connect-src 'none'`, no frames, workers, objects, forms or base change); the URL policy (a relative or own-origin target stays in the page, an https target on a host the plugin declared may open in the system browser, and `http`, `file`, `data`, `blob`, `javascript`, websocket, cross-plugin, undeclared-host, wildcard-host, loopback, path-escaping and credentialed targets are refused); the native bridge, which checks `origin + pluginId + instanceId + nonce + schemaVersion + requestId` against the host session instead of the payload, checks every action's own permission, refuses a wildcard action and any credential argument, rejects a repeated request id (including four concurrent copies of one message), enforces the deadline, and answers a write with a pending proposal only — the executor is never called; and the platform admission matrix, where Android and macOS are `supported`, Windows and Linux are `needs-review` with their engine review, an unlisted platform is `unsupported`, and every platform reports `renderer: false` because this build ships no WebView. The native fallback is [plugin_web_fallback.dart](../../app/lib/features/plugins/plugin_web_fallback.dart) with [plugin_web_fallback_test.dart](../../app/test/plugin_web_fallback_test.dart): the three admission states and each fallback reason, the native page it offers, and the `web-ui` page renderer that shows the fallback instead of a blank page.
- No WebView is shipped in PF-11: the security contract, the admission record and the native fallback are in place, and every `web-ui` contribution renders the fallback while L1/L2 stays usable. Remaining for PF-11: a platform renderer that serves the read-only package on the controlled origin, injects the CSP and a per-session nonce, and forwards only bridge messages; the Android and macOS device matrix (origin isolation, token and loopback isolation, cross-site navigation, input method, stylus, window layer); the Windows and Linux engine review named in the admission table; the daemon route that publishes the admission record to the app; and the install-time artifact hash and path check for the page resources.
