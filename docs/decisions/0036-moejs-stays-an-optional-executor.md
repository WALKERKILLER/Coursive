# MoeJS stays an optional executor outside the daemon build

## Status

Accepted
Class: architecture

## Context and Problem Statement

[Spec 0031](../specs/0031-plugin-fabric-core.md) names MoeJS ([github.com/MoeForge/moejs](https://github.com/MoeForge/moejs), a pure-Go JavaScript engine, Apache-2.0, v0.1.0, with the MIT `moego` as its one runtime dependency) as a candidate executor for short plugin hooks inside the daemon, and [the Plugin Fabric plan](../plugin-fabric.md) asks for a measured answer before any default changes: is running light plugin logic on MoeJS in the daemon better than the Node assistant host that runs Pi extensions today, a Go native function, or a process plugin over RPC? [internal/moejsexec](../../daemon/internal/moejsexec/factory.go) is the real engine behind `pluginfabric.RuntimeFactory`, and [cmd/moejs-bench](../../daemon/cmd/moejs-bench/main.go) measures all four on the same pilot tasks.

## Decision Drivers

- Light plugin tasks (subtitle normalization, JSON transforms, event hooks) must not get slower or heavier for the user.
- Untrusted script code must be stoppable and bounded: deadline, memory, no file, network or process access, no credentials.
- The daemon's default attack surface and binary must not grow for an experiment.
- Existing Pi plugins must keep working unchanged.

## Considered Options

- A. Keep MoeJS as an optional executor: the adapter, its tests and the benchmark live in the repository, and the shipped daemon does not link the engine.
- B. Make MoeJS the default executor for light plugin actions and tools in the daemon.
- C. Drop MoeJS and run every plugin script in the Node assistant host.

## Decision Outcome

Chosen option: A. The measurements below show no speed gain: on the same work MoeJS is about 9× slower per call than the module inline in Node, as a Pi extension runs it, and about 4× slower than Go native code. Through a daemon route it is slower than the host's inline call for every task. Its gains are isolation and footprint. It starts in under a millisecond, uses about 8 MB per plugin instead of a 90 MB process, and stops an endless loop within a millisecond of its deadline under a hard memory cap; Node cannot stop a synchronous loop in the host thread at all. Those gains matter only for untrusted code that must run without the assistant host, and no plugin needs that yet.

- `moejsexec.Factory` compiles a plugin's `moejs` entry once and links its imports when the plugin is prepared. Each prepared plugin gets a pool of runtimes, bounded by the adapter's concurrency.
- Imports may be package-relative paths, resolved through `os.Root` so no `..` or link leaves the package, or bare names of allowlisted host modules. Any other import fails at prepare.
- `eval` and `new Function` are disabled. The global `coursive` carries only the host functions the adapter allowlists and, when a function needs a permission, only ones the plugin declares and holds. A runtime has no file, network, timer or process API and never holds the daemon token, the vault or a database handle.
- A deadline or a cancellation interrupts the script. Output past the bound fails the call.
- A memory-limit hit fails the call and discards the runtime, even when the engine surfaced the hit inside `String.prototype.repeat` as a `RangeError` the script caught.
- Nothing on the daemon's default paths imports the package. CI fails if `coursive-daemon` links MoeJS.
- Revisit this decision when a plugin must run untrusted logic in the daemon without the assistant host, or when a MoeJS release comes within 2× of Node inline on the subtitle task.

### Consequences

- Good: the experiment is reproducible (`go run ./cmd/moejs-bench`), parity with Node is a test, and users get neither a slower path nor a larger daemon.
- Bad: the repository carries an alpha-API dependency and its tests. MoeJS releases may change the API, which the adapter pins at v0.1.0.

## Pros and Cons of the Options

### A. Optional, outside the daemon build
- Good: no regression, no new default attack surface, the isolation evidence stays available.
- Bad: plugin authors cannot ship `moejs` components to users yet.

### B. MoeJS by default for light actions
- Good: per-plugin isolation and limits without a process; under 1 ms startup.
- Bad: 4–9× slower per call than the paths it would replace, about 12× the CPU of Node inline for the same work, and an alpha engine in the shipped binary.

### C. Node only
- Good: fastest for script logic; one JavaScript runtime.
- Bad: no way to stop a synchronous loop in the host thread, and no per-plugin memory cap.

## Links

- Executor seam and adapter: [executor.go](../../daemon/internal/pluginfabric/executor.go)
- Engine factory and isolation tests: [factory.go](../../daemon/internal/moejsexec/factory.go), [factory_test.go](../../daemon/internal/moejsexec/factory_test.go)
- Node parity: [moejs-parity.test.mjs](../../agent/scripts/moejs-parity.test.mjs)
- Benchmark: [cmd/moejs-bench](../../daemon/cmd/moejs-bench/main.go)
- Core contract: [spec 0031](../specs/0031-plugin-fabric-core.md)

## Measurements

Run `cd daemon && go run ./cmd/moejs-bench -node "$(command -v node)" -n 2000` on Linux amd64 (20 CPUs) with Go 1.27.0, Node 26.13.0 and MoeJS v0.1.0. Two runs agreed within noise; the first is shown.

The input is the committed synthetic lecture fixture: 448 cues and 6243 characters, with fillers, repeated cues, full-width Latin and pauses, the shape an ASR pass produces. `-subtitles file.srt` runs the same report on a real local subtitle file, and the report prints no subtitle text. Each backend runs in its own child process. `startup_ms` is the setup inside a host that is already running:
- Go native: none.
- MoeJS: compile and link.
- Node inline: import of the module.
- Process RPC: starting the worker.

`first_action_ms` includes the first runtime's creation. `rss_delta_mb` and `cpu_ms` cover setup plus all calls, and include the worker process for RPC.

| backend | startup_ms | rss_delta_mb | cpu_ms | task | first_action_ms | invoke_p50_ms | invoke_p95_ms | errors |
|---|---:|---:|---:|---|---:|---:|---:|---:|
| go-native | 0.00 | 4.9 | 1425 | subtitles | 1.080 | 0.477 | 0.866 | 0/2001 |
| | | | | json-transform | 0.019 | 0.008 | 0.015 | 0/2001 |
| | | | | event-hook | 0.011 | 0.001 | 0.002 | 0/2001 |
| moejs | 0.34 | 8.9 | 6507 | subtitles | 3.036 | 2.119 | 3.021 | 0/2001 |
| | | | | json-transform | 0.083 | 0.027 | 0.065 | 0/2001 |
| | | | | event-hook | 0.020 | 0.005 | 0.009 | 0/2001 |
| process-rpc | 18.90 | 94.9 | 1465 | subtitles | 6.461 | 0.593 | 0.930 | 0/2001 |
| | | | | json-transform | 0.340 | 0.016 | 0.021 | 0/2001 |
| | | | | event-hook | 0.132 | 0.007 | 0.014 | 0/2001 |
| node-inline | 0.47 | 24.0 | 534 | subtitles | 5.012 | 0.233 | 0.299 | 0/2001 |
| | | | | json-transform | 0.306 | 0.004 | 0.006 | 0/2001 |
| | | | | event-hook | 0.086 | 0.0004 | 0.0005 | 0/2001 |

An agent tool call, timed from the assistant host, takes one of four paths:
- A Pi tool inline, with arguments and result as JSON.
- A Node process plugin over JSON lines.
- A loopback daemon route (bearer token, JSON in and out) backed by Go native code.
- The same route backed by MoeJS.

| path | task | first_ms | p50_ms | p95_ms | errors |
|---|---|---:|---:|---:|---:|
| pi-inline | subtitles | 5.507 | 0.345 | 0.425 | 0/2001 |
| pi-inline | json-transform | 0.249 | 0.009 | 0.013 | 0/2001 |
| pi-inline | event-hook | 0.090 | 0.001 | 0.001 | 0/2001 |
| process-rpc | subtitles | 6.467 | 0.483 | 0.792 | 0/2001 |
| process-rpc | json-transform | 0.456 | 0.019 | 0.024 | 0/2001 |
| process-rpc | event-hook | 0.145 | 0.009 | 0.023 | 0/2001 |
| daemon route → go-native | subtitles | 24.976 | 1.720 | 2.355 | 0/2001 |
| daemon route → go-native | json-transform | 0.587 | 0.114 | 0.366 | 0/2001 |
| daemon route → go-native | event-hook | 0.205 | 0.086 | 0.299 | 0/2001 |
| daemon route → moejs | subtitles | 5.052 | 3.330 | 4.518 | 0/2001 |
| daemon route → moejs | json-transform | 0.375 | 0.145 | 0.439 | 0/2001 |
| daemon route → moejs | event-hook | 0.467 | 0.085 | 0.313 | 0/2001 |

An endless loop under a 50 ms deadline:

| backend | stopped | max_overrun_ms | recover_ms | note |
|---|---:|---:|---:|---|
| moejs | 30/30 | 0.56 | 0.13 | the next call on the same plugin |
| process-rpc | 10/10 | 3.11 | 25.11 | stopped by killing the worker; recovery is a fresh worker's start |
| node-inline | – | – | – | a synchronous loop blocks the host's event loop; only a worker thread or a process can stop it |

Not measured here: Android under proot (cold start and resident memory there), battery, and 100 registered hooks. The parity, isolation and limit behavior are tests; these numbers are one machine's.
