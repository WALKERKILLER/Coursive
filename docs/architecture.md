# Architecture

This is the module map for Coursive: the processes, their seams, and where each responsibility lives. Read it before changing structure. Feature behavior is described in [features.md](features.md); durable rationale lives in [decisions/](decisions/).

## Composition

```text
app (Flutter) ── HTTP + SSE on 127.0.0.1, per-run token ──▶ daemon (Go)
                                                             ├─ library files + rebuildable index
                                                             ├─ jobs: downloads, transcription, courseware parsing
                                                             ├─ credential vault (sealed secrets)
                                                             ├─ plugin store: index, mirrors, installs, consent
                                                             ├─ source providers ── JSON-RPC over stdio ──▶ provider executables
                                                             └─ sync, backup and export
agent host (Node.js) ── HTTP with a per-run token ──▶ daemon (its only data source)
```

The app starts the daemon and the daemon supervises everything else: the agent host when the assistant runtime is installed, and one process per enabled source provider. The app opens on the local library as soon as the daemon answers; no account, sign-in or provider is needed to start.

The app finds the daemon through `COURSIVE_DAEMON`, `COURSIVE_AGENT` and `COURSIVE_NODE`, then next to its executable, then the development tree ([daemon_service.dart](../app/lib/core/daemon_service.dart)). On Android the same daemon and agent host run in an Ubuntu workspace started with proot from the APK's native library folder ([workspace.dart](../app/lib/core/workspace.dart), [decision 0013](decisions/0013-android-runs-coursive-daemon-in-a-proot-ubuntu-workspace.md), [draft spec](specs/draft-android-app.md)).

## Modules

| Module | Responsibility |
|---|---|
| `app/` | Flutter UI for Linux, Windows, macOS and Android: library, player, study canvas, assistant, plugin market and settings |
| `daemon/` | Local HTTP API and event stream, library and index, jobs, credential vault, plugin store and source-provider supervision, sync |
| `agent/` | Assistant host: conversations, course-aware tools, scheduled tasks, and agent plugins loaded as Pi extensions |
| `assets/brand/` | Logo master and every platform icon, produced by [export-brand-icons.sh](../scripts/export-brand-icons.sh) |
| `scripts/` | Governance gates and repository tooling |

### Module map

| Path | Owns |
| --- | --- |
| `daemon/cmd/coursive-daemon` | Daemon entry point |
| `daemon/internal/server` | HTTP API for the app and the agent — the main seam; the lecture watcher (`watch.go`, [spec 0010](specs/0010-live-refresh.md)); source and plugin-market routes (`sources.go`) and learning-platform routes (`lms.go`); the media relay for streams only the daemon reaches (`relay.go`) |
| `daemon/internal/sources` | Source-provider supervision and JSON-RPC transport, the plugin registry, and market installs verified against the index ([spec 0024](specs/0024-source-provider-plugins.md), [spec 0025](specs/0025-plugin-marketplace.md)) |
| `daemon/internal/vault` | Sealed secrets under a per-install key, with one namespace per source plugin |
| `daemon/internal/lms` | Learning-platform data from sources with the `lms` capability: cache, notifications, schedule due dates, course links and file downloads |
| `daemon/internal/jobs` | Download, transcription and parse jobs with per-kind concurrency and persistence |
| `daemon/internal/dl` | Segmented, resumable downloader |
| `daemon/internal/asr` | Cloud ASR channels (bcut, JianYing), chunk pool with the optional `local` channel, cooldowns and retries |
| `daemon/internal/localasr` | Local recognition: model catalog with digests and benchmarks, verified resumable installs, sherpa-onnx runner and punctuation realignment ([spec 0009](specs/0009-local-asr-models.md), [decision 0007](decisions/0007-sherpa-onnx-command-line-for-local-asr.md)) |
| `daemon/internal/notify` | In-app notification feed (`notifications.json`), deduplicated by id and published as `notification` events |
| `daemon/internal/mineru` | MinerU courseware parsing client and sealed token |
| `daemon/internal/library` | On-disk library layout, courseware search, schedule, timeline, README for AI tools |
| `daemon/internal/library/study_*.go` | Validated canvas documents, bounded material/PDF imports, source context and scoped conversation persistence ([study workspace](study-canvas.md)) |
| `daemon/internal/portable` | Converts library Markdown (`coursive://` links, timestamps, math) to plain Markdown for exports |
| `daemon/internal/notion` | Notion API client (pacing, retries) and the one-way sync of `library.KnowledgeDocs` with edit detection ([spec 0013](specs/0013-notion-sync.md), [decision 0011](decisions/0011-notion-sync-through-an-internal-integration.md)) |
| `daemon/internal/datasync` | Backup archives, a WebDAV client and the three-way sync of the library and the assistant's portable data ([spec 0014](specs/0014-data-sync-and-backup.md), [decision 0012](decisions/0012-webdav-sync-with-a-shared-manifest.md)) |
| `daemon/internal/datasync/folder*.go` | Course-scoped folder transport, immutable objects, device manifests and portable record merging ([contract](specs/0023-course-folder-sync.md)) |
| `daemon/internal/store` | SQLite schema and FTS index ([schema.sql](../daemon/internal/store/schema.sql)); rebuildable from library files (schedule and activity via their JSON mirrors) |
| `daemon/internal/events` | SSE event fan-out |
| `daemon/internal/agent` | Supervises the Node agent host and proxies `/agent/*` |
| `daemon/internal/agentenv` | Installs Node.js and the agent's npm dependencies into the data directory and prunes what they never read at run time ([spec 0008](specs/0008-assistant-runtime-setup.md)) |
| `daemon/internal/rootfs` | Unpacks the Android workspace's Ubuntu base (`coursive-daemon unpack`), refusing entries that escape it |
| `agent/src/host.mjs` | Agent host: HTTP endpoints, system prompt, sessions |
| `agent/src/sessions.mjs` | Finds and opens conversations whatever library path they recorded, so ones synced from another device open here |
| `agent/src/http.mjs` | Request body reading with the 1 MiB limit and 30 s deadline, HTTP error replies |
| `agent/src/tools.mjs` | Tool groups, `find_tools`, note playbook, write guards |
| `agent/src/models.mjs`, `catalog.mjs` | Providers, model preferences, models.dev metadata |
| `agent/src/timeline.mjs`, `vision.mjs`, `daemon.mjs` | Outline generation, image bridge for text-only models, daemon client |
| `agent/src/canvas-agent.mjs`, `transcript-review.mjs` | Bounded, user-triggered canvas model runs and non-destructive transcript suggestions |
| `agent/src/usage.mjs` | Usage ledger (`usage.jsonl`): one line per model call, summaries and call paging |
| `agent/src/practice.mjs` | Quiz grounding checks and playbook for `create_quiz` |
| `agent/src/tasks.mjs` | Scheduled tasks: validation, next-run computation, queue and run history (`tasks.json`, `tasks-runs.jsonl`) |
| `agent/src/bot.mjs` | The resident bot: identity (`bot.json`), conversation kinds (`sessions-meta.json`), proposals, consolidation scheduling and transcripts |
| `agent/src/memory.mjs` | Bot memory files: paths, limits, sensitive-value guard, hash-checked writes, prompt injection and memory tools |
| `agent/src/operate.mjs` | The bot's app operations: settings registry, operate tools, proposal ops and undo |
| `agent/src/plugins.mjs` | Plugin host: manifests, install state (`plugins.json`), catalogs, loading plugins as Pi extensions, the permission-gated Coursive API, actions |
| `agent/plugins/` | Official plugins and their catalog; the [plugin author guide](../agent/plugins/README.md) |
| `app/lib/core` | daemon client (`api.dart`), daemon discovery and startup, the Android workspace and platform bridge (`workspace.dart`, `android_bridge.dart`, `background_work.dart`), app state, models, study-time tracker (`activity_tracker.dart`), installed plugins and their contributions (`plugins.dart`) |
| `app/lib/features/study_canvas` | Canvas rendering, materials, local ink, docked controls, source selection and scoped conversation UI ([controls](drawing-controls.md)) |
| `app/lib/features/<area>` | Screens: activity, assistant, courses, courseware, downloads, home, library, lms, notifications, player, plugins, practice, profile, schedule, settings, setup (Android workspace), sources, usage |
| `app/android/workspace/prepare.py` | Fetches the pinned Termux proot build and builds `libcoursive_daemon.so` and the agent's sources for the APK; [NOTICE](../app/android/workspace/NOTICE) for the shipped programs |
| `app/lib/widgets` | Shared widgets, including the Markdown/LaTeX renderer and reader, statistics charts (`charts.dart`), the activity heatmap (`heatmap.dart`) and the glass navigation of phones (`glass_dock.dart`) and of tablets and desktops (`glass_rail.dart`) |
| `app/lib/theme` | Material 3 theme, spacing/motion tokens and the glass material for floating chrome (`glass.dart`) |
| `app/lib/l10n` | Interface text: `app_zh.arb` and the `app_localizations*.dart` that `flutter gen-l10n` generates from it, read through the global `l10n` in `l10n.dart` |

## Seams

- **Daemon HTTP API**: the only interface between the app or the agent host and user data. Routes live in `daemon/internal/server`; a route or JSON field change updates both clients, [api.dart](../app/lib/core/api.dart) and [daemon.mjs](../agent/src/daemon.mjs), in the same change.
- **Plugin manifest**: a plugin is a package whose manifest declares contributions — agent tools and skills, views, actions, and sources — and the permissions they need. The daemon owns install state and consent; each contribution runs in the runtime that owns it ([spec 0025](specs/0025-plugin-marketplace.md)).
- **Source-provider protocol**: versioned JSON-RPC 2.0 over a provider's stdin and stdout. Providers sign in, list courses and lectures, and resolve media; the daemon stores their secrets and performs every download ([spec 0024](specs/0024-source-provider-plugins.md)).
- **Plugin index and mirrors**: a signed `index.json` and a mirror list published from [coursive-plugins](https://github.com/WALKERKILLER/coursive-plugins); mirrors are untrusted transports.
- **Library on disk**: the daemon owns the layout; the index is rebuildable from the library files and is never the source of truth.
- **`coursive://` links**: parsed by `CoursiveRef` in [coursive_markdown.dart](../app/lib/widgets/coursive_markdown.dart), produced by assistant tools, documented for AI tools in [readme.go](../daemon/internal/library/readme.go).
- **Markdown rendering**: every Markdown surface uses `CoursiveMarkdown` in [coursive_markdown.dart](../app/lib/widgets/coursive_markdown.dart).
- **Plugins**: a plugin is a package with a `coursive` manifest whose agent module is loaded as a Pi extension per conversation ([decision 0005](decisions/0005-plugins-as-pi-extensions-with-declarative-views.md), [spec 0004](specs/0004-plugin-platform.md)). Its tools reach the model through `find_tools` groups `plugin:<id>`; its results are data for views the app draws (`pluginViewBuilders` in [plugin_views.dart](../app/lib/features/plugins/plugin_views.dart)); its contributions appear through `PluginMenuButton` on the note toolbar, lecture side panel, course page and schedule items, and the composer's "+" menu.
- **Plugin Fabric (approved extension)**: plugin pages, typed UI slots, service capabilities and optional executors follow [spec 0031](specs/0031-plugin-fabric-core.md) and [spec 0032](specs/0032-ui-extension-protocol.md); the design rationale is [plugin-fabric.md](plugin-fabric.md) and the decision is [0033](decisions/0033-plugin-fabric-and-ui-extension-protocol.md). The Pi-based plugin path above remains the shipped baseline until the migration lands.
- **Bot and conversation kinds**: the agent host keeps one bot thread and course agents, told apart by `sessions-meta.json` ([decision 0006](decisions/0006-resident-bot-with-file-memory.md), [spec 0007](specs/0007-resident-bot-and-course-agents.md)). Bot memory lives in Markdown files the host injects into prompts; app operations come back as `details.app` (run by `BotOps` in [bot_ops.dart](../app/lib/features/assistant/bot_ops.dart)) or as proposals the host applies through `/agent/proposals/{id}/apply`.
- **`coursive-app://` deep links**: produced by `daemon/internal/portable` for exported Markdown; received by the app over the `coursive/links` method channel (Linux runner, Android `MainActivity`).
- **Release package**: [release.yml](../.github/workflows/release.yml) places `coursive-daemon` and `agent/` next to the app executable (in `Contents/MacOS` and `Contents/Resources` on macOS), matching the discovery order above.
- **Android guest**: every program in the workspace runs through `Workspace.guest` (proot, `env -i`, the files folder bound at its own path, native programs at `/opt/coursive`); paths the daemon reports are valid in the app.

## Invariants

- Credentials never leave the daemon unsealed, and nothing logs or returns them.
- The core contains no knowledge of a specific institution or course platform.
- Long-running daemon work takes a `context.Context` and stops on cancellation; asynchronous Flutter callbacks check `mounted` before touching `BuildContext`.

## Conventions

- New behavior goes on documented extension points; a decision record is required when a core-flow change chooses among meaningful alternatives.
- Explicit > implicit at boundaries: defaulting is an explicit step in the owning implementation, never a hidden fallback.
- UI spacing, shape and motion come from `app/lib/theme/tokens.dart` rather than literals.
- Glass (`GlassSurface`) is reserved for chrome that floats over content: the phone dock and its assistant button, the tablet and desktop rail, canvas docks, the canvas composer and shelf. Pages, cards and lists stay tonal Material 3 surfaces. On phones the dock floats over the tab's root page, shrinks to the current tab while the page scrolls down and hides on pushed pages and while the keyboard is open. Its selection is a lens inset 4 dp in the pill with the pill's corner less the inset, so the two stay concentric; pressing and sliding along the dock carries the lens under the finger with a tick per tab and selects on release. Root pages add `MediaQuery.paddingOf(context).bottom` to their last scroll padding so content clears the dock. The rail pins its leading and trailing controls; on short windows the brand hides, then labels give way to tooltips, then destinations scroll with a fade at the edge that has more.
- User-visible text comes from `app/lib/l10n/app_zh.arb` (`l10n.<key>`, placeholders for values) rather than string literals in widgets; edit the ARB file and run `flutter gen-l10n`. Copy names UI elements and paths in 「」, starts failures with 无法, and calls this device 本机.

## Change procedure

1. Trace the current owners of the flow you change.
2. For a risk-boundary change, obtain an Approved spec in [docs/specs/](../docs/specs/).
3. Record durable design choices in [docs/decisions/](../docs/decisions/) when genuine alternatives exist.
4. Update this document and the owning module docs in the same change.
5. Run the gates; add the smallest sufficient evidence for the behavior and risk, not tests for line count.
