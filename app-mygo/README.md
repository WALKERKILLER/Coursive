# app-mygo

The MyGo native-UI client for Coursive, in Go. It draws its own window with `github.com/egoist/mygo/ui` and the Material 3 components of [Mayu](https://github.com/harukizmoe/mayu), and speaks the daemon's existing HTTP API: the same routes, the same JSON, the same bearer token. The daemon keeps owning the library, the downloads and the credentials, exactly as it does for the Flutter client in `app/`.

This is stage 0 of [the migration spec](../docs/specs/0033-mygo-client-migration.md); [the checklist](../docs/mygo-migration-todo.md) orders the rest. The Flutter client stays the shipped one until a platform passes its acceptance.

## Run it

The daemon must be built first, and the prototype never opens the real library unless you point it there:

```sh
(cd ../daemon && go build -o bin/coursive-daemon ./cmd/coursive-daemon)
COURSIVE_DATA=/tmp/coursive-proto/data COURSIVE_LIBRARY=/tmp/coursive-proto/library go run .
```

Without `COURSIVE_DATA` and `COURSIVE_LIBRARY` the daemon uses its own defaults, which is the same library the installed app reads. Point both somewhere else when trying the prototype.

## Test it

```sh
go test ./...
```

`TestDaemonHandshakeAndCourses` starts the real daemon against a temporary library and walks the contract end to end: handshake, token, an empty list, a created course, and a rejected token. The gesture tests replay pointer events through the canvas's input rules, so gesture switches, cancels and the pressure-eraser gate are checked without a device. `TestLibraryPageGolden` compares the window with `testdata/library-light.png` and `testdata/library-dark.png`; rewrite them with `MYGO_UPDATE_GOLDEN=1`.

## What is here

- `daemon.go` — the daemon child process: token, `COURSIVE_READY` handshake, restart backoff, clean shutdown. The contract is the Flutter client's `app/lib/core/daemon_service.dart`.
- `api.go` — the HTTP client: `GET /api/courses`, `POST /api/local-courses`, `GET /api/health`.
- `theme.go` — Coursive's seed colour, 湛蓝 `#3F6CB4`, through Mayu into MyGo's own theme.
- `main.go` — the window: navigation rail, course library, the setup/starting/ready/failed states.
- `gesture.go` — the canvas's input rules in plain Go, with no UI types, so they replay in a test.
