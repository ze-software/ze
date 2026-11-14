# Ze Go Style: where Ze differs from standard Go

The detail behind "Where Ze differs from standard Go" in
[the Go style core](../ze-go-style.md). The core keeps the first rule of each
section; these are the other rows. The buffer and pool rules are in "Performance"
([performance.md](performance.md)).

## Encoding and wire

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `bytes.Buffer`, `append`, or `make` in a helper | A pooled buffer of one fixed maximum size, passed down by the caller | `ai/rules/performance.md` | One allocation at the outermost scope, and block accounting can release a block whole |
| Parse into structs eagerly | Lazy iterators over raw byte slices (`Next()`) | `ai/rules/architecture.md` | N to zero-until-needed, not N to one |
| `fmt.Sprintf`, `strings.Join` | `textbuf.Buffer` (128-byte stack inline) or `strconv.Append*` | `ai/rules/performance.md` | Sprintf allocates two to three times; textbuf allocates once |

## Architecture and registration

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Constructor injection | Registry lookup at runtime, such as `Registration.InProcessNLRIDecoder` through the family index | `ai/rules/plugins.md` | A plugin is removable by dropping its blank import |
| `os.Getenv("FOO")` | `env.Get("ze.foo")` through `internal/core/env` | `ai/rules/go-standards.md` | Caching, registration, dot and underscore agnostic, secret clearing |
| `log.Printf` or `logrus` | `slog` through `slogutil.Logger("subsystem")` | `ai/rules/go-standards.md` | Per-subsystem levels set by env var |
| Shared types by direct import | Cross-boundary payloads are value types only | `ai/rules/plugins.md` | No pointer fields cross a plugin or component boundary |

<!-- source: internal/core/env/env.go -- Get -->
<!-- source: internal/core/slogutil/slogutil.go -- Logger -->
<!-- source: internal/component/plugin/registry/registry.go -- Registration.InProcessNLRIDecoder -->

## Config and schema

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| A config version field | No version numbers and no migration of Ze's own syntax | `ai/META-DECISIONS.md` | Pre-release: a syntax change converts the repository's configs in the same change |
| Silent defaults for missing fields | Fail on an unknown key and suggest the closest valid one | `ai/rules/config.md` | Explicit beats implicit |
| `interface{}` for flexible config | `map[string]any` through one canonical pipeline | `ai/rules/repo-maintenance.md` | File to Tree to `ResolveBGPTree` to `map[string]any` to `PeersFromTree` |

## Communication and IPC

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Direct function calls for synchronous work | DirectBridge for typed in-process calls | `ai/rules/plugins.md` | Skips JSON serialization for internal plugins |
| Channel-based pub/sub | EventBus with typed handles (`events.Register[T]`) | `ai/rules/plugins.md` | Type-safe registered event types, no raw `bus.Subscribe` |

<!-- source: internal/core/events/typed.go -- Register -->

## Testing

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `go test ./...` for verification | The scoped package tests and functional suites the change reaches, with its feature tags, run in the main checkout | `ai/rules/testing.md` | Day-to-day evidence covers what the change can reach; no agent runs `./le verify`, and the whole-tree run is the release check GitHub CI runs |
| `testify/assert` | The standard library `testing` package | (convention) | No test framework dependencies |
| `go test -race` once | `go test -race -count=20 ./internal/component/bgp/reactor/...` for reactor code | `ai/rules/testing.md` | A rare schedule needs repeated runs to surface |

## CLI and commands

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `cobra` or `flag` | YANG-modeled dispatch with RPC handlers | `ai/patterns/cli-command.md` | One schema serves CLI, web, config and completion |
| Format the output as a string | Return structured data and format through pipe operators | `ai/rules/cli.md` | `\| json`, `\| table`, `\| match`, `\| resolve` |
| Hardcode help text | Derive it from the registry or schema | `ai/rules/evidence.md` | One source of truth, no stale enumerations |

## Native tooling

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `/tmp` for scratch files | The per-session directory from `./le session scratch ensure` | `ai/rules/commands.md` | Concurrent sessions never share a name |
| A bare staging verb followed by a bare commit | `./le commit create`, then the generated script | `ai/rules/git-safety.md` | The declared file population is checked before staging |
