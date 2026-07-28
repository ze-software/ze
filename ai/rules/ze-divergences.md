# Ze Divergences from Standard Go

**When:** when a Go instinct formed outside Ze is about to drive a decision
**Severity:** advisory

## Directives

Ze differs from typical Go projects in specific, load-bearing ways.
An AI trained on standard Go patterns will default to the wrong
approach unless it reads this document. Each entry names the standard
approach, the Ze approach, the rule that governs it, and a one-line
reason.

## Encoding / Wire

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `func (t T) Marshal() ([]byte, error)` | `func (t T) WriteTo(buf []byte, off int) int` | `ai/rules/buffer-first.md` | Zero alloc on hot path; caller owns the buffer |
| `bytes.Buffer` / `append` in helpers | Pre-allocated pooled buffers, slice inward | `ai/rules/buffer-first.md` | Bounded memory, no GC pressure |
| `make([]byte, n)` for variable-length wire data | Pool-backed buffers of fixed MAX size | `ai/rules/memory-architecture.md` | Pool strategy by goroutine shape |
| Helper allocates its own scratch | Caller passes buffer down, callee writes into it | `ai/rules/memory-architecture.md` | One allocation at outermost scope, not N in sub-functions |
| `sync.Pool` only for "reuse" | `sync.Pool` is the default for multi-goroutine scratch, ring buffer for single-goroutine | `ai/rules/memory-architecture.md` | Pool shape must match goroutine shape |
| Parse into structs eagerly | Lazy iterators over raw byte slices (`Next()`) | `ai/rules/design-principles.md` (Lazy over eager) | N->0-until-needed, not N->1 |
| `fmt.Sprintf` for formatting | `textbuf.Buffer` (128B stack-inline) or `strconv.Append*` | `ai/rules/no-sprintf-alloc.md` | Sprintf allocates 2-3x; textbuf allocates once |
| `strings.Join(parts, " ")` | Single `textbuf.Buffer` with `.Byte(' ')` separators | `ai/rules/no-sprintf-alloc.md` | Eliminates intermediate `[]string` + final join |

## Architecture / Registration

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Direct imports between packages | `init()` + registry + blank import | `ai/patterns/registration.md` | Small core discovers components; never imports directly |
| Constructor injection | Registry lookup at runtime (`registry.NLRIDecoder(family)`) | `ai/rules/plugin-design.md` | Plugins are independently removable via blank import |
| `os.Getenv("FOO")` | `env.Get("ze.foo")` via `internal/core/env` | `ai/rules/go-standards.md` | Cache, registration, dot/underscore agnostic, secret clearing |
| `log.Printf` or `logrus` | `slog` via `slogutil.Logger("subsystem")` | `ai/rules/go-standards.md` | Hierarchical per-subsystem levels via env vars |
| Shared types via direct import | Cross-boundary payloads are value types only | `ai/rules/plugin-design.md` (Cross-Boundary Value Types) | No pointer fields across plugin/component boundaries |

## Config / Schema

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Struct tags + `json.Unmarshal` | YANG schema as sole source of truth | `ai/rules/config-design.md` | Schema-driven validation, migration, completion, diff |
| Config version field | No version numbers; machine-transformable migration | `ai/rules/config-design.md` | YANG evolution handles schema changes |
| Silent defaults for missing fields | Fail on unknown keys; suggest closest valid | `ai/rules/config-design.md` | Explicit > implicit |
| `interface{}` for flexible config | `map[string]any` through canonical pipeline | `ai/rules/project-knowledge.md` | File -> Tree -> ResolveBGPTree -> map[string]any -> PeersFromTree |

## Communication / IPC

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| gRPC or HTTP between services | JSON events down, text commands up, over pipes or net.Pipe | `ai/rules/plugin-design.md` | Plugin SDK is language-agnostic (Go/Python/Rust) |
| Direct function calls for sync | DirectBridge for typed in-process calls | `ai/rules/plugin-design.md` (DirectBridge) | Bypasses JSON serialization for internal plugins |
| Channel-based pub/sub | EventBus with typed handles (`events.Register[T]`) | `ai/rules/plugin-design.md` (EventBus) | Type-safe, registered event types, no raw `bus.Subscribe` |

## Testing

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `go test ./...` for verification | `make ze-verify` (two-pass + functional + exabgp) | `ai/rules/testing.md` | 349 packages; cached full + race on changed groups |
| Unit tests prove correctness | Unit tests + `.ci` functional tests (both required) | `ai/rules/integration-completeness.md` | Unit proves algorithm; `.ci` proves user can reach the feature |
| `testify/assert` | Standard library `testing` | (convention) | No test framework dependencies |
| `go test -race` once | `make ze-race-reactor` (`-race -count=20`) for reactor code | `ai/rules/testing.md` | Rare schedules need repeated runs to surface |

## CLI / Commands

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `cobra` or `flag` | YANG-modeled dispatch with RPC handlers | `ai/patterns/cli-command.md` | Unified schema for CLI, web, config, completion |
| `command <identifier> [flags]` | `<verb> <noun> <action> [<identifier>]` | `ai/rules/cli-grammar.md` | Identifier-keyword ambiguity elimination |
| Format output as string | Return structured JSON, format via pipe operators | `ai/rules/pipe-completeness.md` | `\| json`, `\| table`, `\| match`, `\| resolve`, etc. |
| Hardcode help text | Derive from registry/schema | `ai/rules/derive-not-hardcode.md` | Single source of truth; no stale enumerations |

## Scripts / Tooling

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Shell scripts for tooling | Python only | `ai/rules/go-standards.md` | Shell is fragile for complex orchestration |
| `/tmp` for scratch files | Project `tmp/` (gitignored) | `ai/rules/testing.md` | `go test ./...` walks `/tmp`; project tmp is isolated |
| `git add -A && git commit` | Commit via script the user triggers | `CLAUDE.md` prohibitions | Sessions share staging; cross-commits result |
