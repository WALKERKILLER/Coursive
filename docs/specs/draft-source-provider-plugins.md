# Source-provider plugins

Artifact-Version: 1
Status: Draft

## Intent

Coursive must work for anyone with lecture recordings, so the core can neither assume an institution nor require an account. Today sign-in, course listing and media resolution are built into the daemon, and the app root blocks on a splash screen and a sign-in page before showing anything.

Outcome:

- The app always opens on the local library, whatever is installed.
- Each external course platform is a **source plugin**. Installing one adds its sign-in, course list and downloads; uninstalling it removes them, and the library stays.

Scope:

- The startup path.
- The source contribution in the plugin manifest.
- The provider protocol and its process supervision.
- The credential vault the daemon offers providers.
- The UI that appears when a source is installed.

Non-goals:

- Moving downloads, transcription or the library into plugins.
- Sandboxing provider executables beyond what the operating system gives a user process.
- Shipping any provider with the core.

Constraints:

- Secrets stay inside the daemon.
- A provider never blocks startup.
- The core names no institution or platform.

## Contract

### Startup

- The app root shows the workspace setup (Android, first run), a daemon failure page with 「重新启动」, or the shell. There is no splash route, no sign-in route and no local-mode switch.
- While the daemon is connecting, the shell is shown with its content areas in a loading state. The native launch screen of each platform shows the app icon until the first frame.
- With no source plugin installed, no sign-in entry, platform course list or platform download action exists anywhere in the UI.

### Manifest

A plugin's `package.json` `coursive` manifest may declare sources:

```json
"contributes": {
  "sources": [{
    "id": "example",
    "title": "Example Lectures",
    "protocol": "1",
    "executable": {
      "linux-x64": "bin/provider-linux-x64",
      "linux-arm64": "bin/provider-linux-arm64",
      "windows-x64": "bin/provider-windows-x64.exe",
      "macos-arm64": "bin/provider-macos-arm64",
      "macos-x64": "bin/provider-macos-x64"
    },
    "hosts": ["lectures.example.org"]
  }]
}
```

- `id` follows the plugin id rules and is unique across installed plugins.
- `executable` has one entry per platform. A plugin with no entry for this device is shown as unavailable here.
- The `source` permission is required. Consent shows it as 「登录 <title> 并下载课程」 together with the declared hosts.
- Consent also states that the provider runs as a native program with the user's rights.

### Process

- The daemon starts an enabled provider on first use: `<executable>` with an empty environment except `HOME`, `TMPDIR`, `LANG`, proxy variables and `COURSIVE_PROTOCOL=1`. The working directory is the provider's data directory (mode 0700).
- Transport: JSON-RPC 2.0, one message per line, UTF-8.
  - stdin and stdout carry protocol messages, at most 1 MiB per message.
  - stderr is diagnostics, capped at 64 KiB per minute and redacted before it reaches the daemon log.
- The provider must answer `initialize` within 10 s. Any other call times out after 60 s.
- A crash restarts the provider with backoff (1, 2, 4, 8, 16 s). After five failures in ten minutes the source is marked `error` with the last message, and the user restarts it from 插件.
- Idle providers exit after 10 minutes without calls.

### Protocol version 1

From the daemon to the provider:

| Method | Params → Result |
|---|---|
| `initialize` | `{protocol, daemonVersion, locale}` → `{protocol, capabilities: [...], signIn: {fields: [{key, title, kind: "text"\|"password"\|"otp", remember?}]}}` |
| `auth.status` | `{}` → `{state: "signed_out"\|"signed_in"\|"needs_input"\|"expired", account?: {display}}` |
| `auth.signIn` | `{fields: {key: value}}` → `{state, challenge?: {id, kind: "image"\|"text"\|"qr", prompt, image?}}` |
| `auth.answer` | `{challenge, answer}` → same as `auth.signIn` |
| `auth.signOut` | `{}` → `{}` |
| `courses.list` | `{cursor?}` → `{courses: [{id, title, term?, teachers?}], cursor?}` |
| `lectures.list` | `{course, cursor?}` → `{lectures: [{id, title, startsAt, durationSec?, streams: ["camera"\|"screen"\|"audio"], courseware?: bool}], cursor?}` |
| `media.resolve` | `{lecture, stream}` → `{url, headers?, expiresAt?}` |
| `courseware.list` | `{lecture}` → `{files: [{id, name, size?}]}` (capability `courseware`) |
| `courseware.resolve` | `{lecture, file}` → `{url, headers?, expiresAt?}` (capability `courseware`) |

From the provider to the daemon:

| Method | Params → Result |
|---|---|
| `vault.get` | `{key}` → `{value?}` |
| `vault.put` | `{key, value}` → `{}` |
| `vault.delete` | `{key}` → `{}` |
| `notify` | `{state}`: sign-in state changed (for example a session expired) |

- `capabilities` lists optional method groups. The daemon never calls a group that was not announced.
- Errors use JSON-RPC error objects with a user-facing `message` that starts with 无法, plus `data.kind`: `auth_required`, `rate_limited` (with `retryAfterSec`), `unavailable` or `invalid`.

### Vault and downloads

- `vault.*` stores per-provider values sealed with the daemon's existing seal key, at most 64 keys of 16 KiB each. Uninstalling the plugin deletes them.
- Sign-in field values marked `remember: false` are passed to the provider and never stored.
- The daemon performs every media and courseware download itself. Resolved URLs and headers live only in memory for the job, are never logged or returned by an API, and are resolved again when they expire.
- Library entries from a source carry `source: "<plugin>/<source>"` and the provider's ids. They stay readable after the plugin is removed and are marked as no longer refreshable.

### UI

- With a source installed, 课程 → 来源 lists each source with its state and 「登录」 or 「退出登录」.
- The sign-in sheet is rendered from `signIn.fields` and challenges. A challenge image is shown at its natural size.
- A source's courses appear in the library beside local courses, grouped by source. Lecture rows offer the download actions the source's capabilities allow.
- A source in state `expired` shows a banner on its courses with 「重新登录」. Downloads that need it wait with that reason instead of failing.

## Plan

1. Daemon:
   - provider supervisor and JSON-RPC transport;
   - vault namespacing;
   - source registry, and library fields for source entries;
   - routes `GET /api/sources`, `POST /api/sources/{id}/sign-in`, `/answer` and `/sign-out`, plus source-scoped course and lecture routes.
2. App:
   - a root without splash or sign-in gates;
   - the shell's loading states;
   - the 来源 page and sign-in sheet driven by `signIn.fields`.
3. Remove the built-in sign-in code and its routes from the core. Port it to a provider in a plugin repository chosen by the maintainer.
4. coursive-plugins: protocol schema, a provider template, and a fake provider used by daemon tests.

## Verification

- Daemon integration tests against a fake provider executable:
  - sign-in with a challenge;
  - expiry and re-sign-in;
  - crash and backoff;
  - a timeout;
  - oversized output;
  - redaction of stderr;
  - vault limits and deletion on uninstall.
- Widget tests:
  - the app reaches the shell with the daemon connecting and with no source installed;
  - no sign-in entry exists without a source;
  - the sign-in sheet renders each field kind and challenge kind.
- A search over the tree and the import history passes the checklist in [migration.md](../migration.md#de-identification-checklist).

## Evidence

Pending until implementation.

## Open questions

1. Where the ported providers live: a private repository, or a public one listed only through a user-added catalog. The official index lists no institution-specific source either way.
2. Whether the predecessor's library folder is imported once on first start, or the user imports it through the existing local-course import.
