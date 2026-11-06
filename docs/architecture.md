# Architecture

This is the module map for Coursive: the processes, their seams, and where each responsibility lives. Modules join the repository as they migrate ([migration.md](migration.md)); a module listed here before it lands describes its contract, not files you can open yet.

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

## Modules

| Module | Responsibility |
|---|---|
| `app/` | Flutter UI for Linux, Windows, macOS and Android: library, player, study canvas, assistant, plugin market and settings |
| `daemon/` | Local HTTP API and event stream, library and index, jobs, credential vault, plugin store and source-provider supervision, sync |
| `agent/` | Assistant host: conversations, course-aware tools, scheduled tasks, and agent plugins loaded as Pi extensions |
| `assets/brand/` | Logo master and every platform icon, produced by [export-brand-icons.sh](../scripts/export-brand-icons.sh) |
| `scripts/` | Governance gates and repository tooling |

## Seams

- **Daemon HTTP API**: the only interface between the app or the agent host and user data. A route or JSON field change updates both clients in the same change.
- **Plugin manifest**: a plugin is a package whose manifest declares contributions — agent tools and skills, views, actions, and sources — and the permissions they need. The daemon owns install state and consent; each contribution runs in the runtime that owns it ([plugin marketplace](specs/draft-plugin-marketplace.md)).
- **Source-provider protocol**: versioned JSON-RPC 2.0 over a provider's stdin and stdout. Providers sign in, list courses and lectures, and resolve media; the daemon stores their secrets and performs every download ([source-provider plugins](specs/draft-source-provider-plugins.md)).
- **Plugin index and mirrors**: a signed `index.json` and a mirror list published from [coursive-plugins](https://github.com/WALKERKILLER/coursive-plugins); mirrors are untrusted transports.
- **Library on disk**: the daemon owns the layout; the index is rebuildable from the library files and is never the source of truth.

## Invariants

- Credentials never leave the daemon unsealed, and nothing logs or returns them.
- The core contains no knowledge of a specific institution or course platform.
- Long-running daemon work takes a `context.Context` and stops on cancellation; asynchronous Flutter callbacks check `mounted` before touching `BuildContext`.
