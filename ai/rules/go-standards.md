# Go Standards

**When:** writing Go in Ze: naming, env access, context, logging, imports, errors, API contracts, typed-vs-string choices, file layout and cross-references, compatibility shims, or a Go compiler bump
**Severity:** blocking
**Related:** config, cli, performance, repo-maintenance, architecture

## Directives

### Required

- Go 1.21+ features (slog, generics)
- `golangci-lint` MUST pass
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Context as first param: `context.Context`
- Code MUST NOT strip `ctx context.Context` parameters from function signatures. "Clean unused context" means remove dead `import "context"` lines only. Parameters stay even if the current body doesn't use ctx (propagation, cancellation, future use).
- Fail-early: code MUST propagate parse/config errors immediately, and MUST NOT silently default

### Logging: `log/slog` only

- Engine: `slogutil.Logger("subsystem")`
- Plugins: `slogutil.PluginLogger("name", level)`
- Per-subsystem: `ze.log.<path>=<level>` env vars (hierarchical, most-specific wins)
- Levels: `disabled`, `debug`, `info`, `warn`, `err`
- Config: `environment { log { level warn; bgp.routes debug; } }`
- Priority: CLI flag > env var > config > default (WARN)
- Debug logging is permanent: code MUST use `logger.Debug()`, and MUST NOT use `fmt.Printf`

### Dependencies

Never add new third-party imports (not already in `go.mod`) without asking the user first.

### Environment Variables: `internal/core/env` only

All Ze environment variable access MUST use `env.Get()` / `env.Set()` or typed helpers. Never use `os.Getenv()` or `os.Setenv()` for Ze-specific vars.

Before adding an env var, read `ai/rules/config.md` (should this be YANG config instead?) and `ai/rules/config.md` (naming conventions, env var path must mirror YANG path).

| Getters | Use |
|---------|-----|
| `env.Get("ze.foo.bar")` | String lookup (case-insensitive, dot/underscore agnostic) |
| `env.GetInt("ze.foo", 0)` | Integer with default |
| `env.GetInt64("ze.foo", 0)` | Int64 with default |
| `env.GetBool("ze.foo", false)` | Boolean (true/false/1/0) with default |
| `env.IsEnabled("ze.foo")` | Enabling check (1/true/yes/on/enable/enabled) |
| `env.GetDuration("ze.foo", 5*time.Second)` | Duration with default |

| Setters | Use |
|---------|-----|
| `env.Set("ze.foo", "val")` | String (updates cache + os env) |
| `env.SetInt("ze.foo", 42)` | Integer |
| `env.SetBool("ze.foo", true)` | Boolean ("true"/"false") |

**Cache:** Built once from `os.Environ()` on first `Get()`. `Set*()` updates both cache and os env. Tests that use `os.Setenv` directly MUST call `env.ResetCache()`.

**Registration required:** every env var MUST be registered via package-level `var _ = env.MustRegister(...)`. Calling `env.Get()` with an unregistered key aborts the process.

**Registration flags:**

| Flag | Meaning |
|------|---------|
| `Private: true` | Hidden from `ze env list` and autocomplete |
| `Secret: true` | Cleared from OS environment after first `Get()` (value stays in cache) |

**`os.Getenv` MAY be used for:** System env vars (`HOME`, `PATH`, `XDG_*`, `NO_COLOR`, `USER`, `SSH_*`).

### Aliased Imports

- When two packages in the module share the same name (e.g., `internal/component/iface/cli/` and `internal/component/iface/`), goimports cannot resolve which to use and silently removes the import. You MUST use an aliased import in this case: `ifacepkg "github.com/ze-software/ze/internal/component/iface"`.
- You MUST add import + usage in the same Edit call to prevent goimports from removing an "unused" import between edits.

### Scripts: Python Only

Do not use shell/bash for scripts. Use Python. Shell scripts are fragile and hard to debug for complex orchestration. Precedent: `test/interop/run.py`, `test/interop/interop.py`.

### Style patterns to prefer

Advisory, not a sweep. Adopt opportunistically the next time you are in the relevant code.

- **Early return over nested else.** Handle the error / edge case and return immediately rather than wrapping the happy path in an else block. Deep nesting makes control flow hard to follow; a sequence of guard clauses followed by the main logic reads linearly. Applies to `if err != nil { return }`, validation guards, and nil checks alike.
- **Drain `time.NewTimer` on Stop.** If you use `time.NewTimer` directly and call `Stop()`, drain `.C` in a non-blocking select before `Reset()`, otherwise the next `Reset()` sees a stale firing and behaves wrong. The BGP FSM uses `clock.Timer` + `AfterFunc` (callback-based, no channel to drain) and needs no helper. The only `time.NewTimer` site in the BGP tree, `internal/component/bgp/plugins/rs/worker.go`, already has a `drainTimer` helper: promote it or copy the pattern when adding a new `time.NewTimer` site. Prefer `AfterFunc` where possible.
- **Slice type with methods beats a wrapping struct.** When the only state is `[]*T`, declare `type Foo []*T` with methods on the slice rather than `type Foo struct { items []*T }`. `Foo{}` is the empty value, `append(foo, x)` works, iteration works, JSON marshaling works, and you never write a `NewFoo`/`Items()` pair.
- **Family of narrow constructors beats a god `New(Config)`.** When most callers care about one axis of a type, give each common construction pattern its own named constructor (`NewFooWithPrefixes`, `NewFooWithProtocols`) rather than a functional-options pile or a config struct with half the fields nil.
- **Table-driven tests with `t.Run(name, ...)`.** When a test file has more than two similar cases, prefer a single `[]struct { name string; ... }` table iterated with `t.Run(tt.name, ...)`. Each case gets its own `-v` line, each case is self-contained, and you can focus one failing case.
- **Honest TODO comments over silent gaps.** When a function is incomplete (especially deep-comparison `equal()` methods that can pass superficially and silently mis-match on edge cases), write a visible `// TODO:` rather than a panic, a `//nolint`, or nothing. A reviewer SHOULD see the gap in a diff.

### JSON Struct Tags (MANDATORY)

Every exported struct field that reaches JSON output **must** have a `json:"kebab-name"` tag. Keys are lowercase kebab-case, matching the YANG leaf or config tree key. Full rules and attribute table: `ai/rules/cli.md`.

### Forbidden

**Code MUST NOT write these forbidden Go patterns:**
- `panic()` for error handling. Allowed prefixes (enforced by `block-panic-error.sh`): `panic("BUG: ...")`, `panic("unreachable: ...")`, `panic("not implemented")`, `panic("unimplemented")`, `panic("TODO: ...")`, `panic("impossible: ...")`. Use `panic("BUG: <what>")` for programmer-error guards that MUST never fire at runtime. Any other `panic()` call is rejected at Write/Edit time (test files and `scripts/` excepted)
- `f, _ := func()` and `_, _ = func()` (ignoring errors). If you genuinely MUST discard, use `//nolint:errcheck // <why>` with a specific reason
- Global mutable state
- `init()` except registry patterns
- `log.Printf` (legacy log package)
- Silent defaults: `if x == "" { x = "0.0.0.0/0" }`
- `os.Getenv("ZE_*")` or `os.Getenv("ze.*")` -- use `env.Var()` instead
- `if end > x { end = x }` when clamping an int, use `end = min(end, x)` (Go 1.21+ built-in)
- `for i := 0; i < N; i++` when the body does not use `i` as anything but a counter, use `for range N` (Go 1.22+)

## Naming

"Ze" = "The" with a French accent. Use "ze" where "the" works grammatically.

| Context | Use |
|---------|-----|
| CLI binary | `ze` |
| BGP config YANG | `ze-bgp-conf` |
| BGP JSON format | `ze-bgp` |
| Go variables | `ZeBGPConf*` |
| YANG suffixes | config `-conf`, API `-api` |

Config-specific naming (YANG leaves, env var keys, Go struct fields): `ai/rules/config.md`

### Naming: describe what, not the type

Names of variables and constants must describe what the value IS, not its Go type.

| Banned pattern | Why | Fix |
|---------------|-----|-----|
| `*Str` suffix (`famStr`, `levelStr`, `addrStr`) | Encodes "string" type into the name | `family`, `level`, `addr` |
| `*Int`, `*Bool`, `*Bytes` suffixes | Same problem | Name the concept |
| Field/type-as-prefix on enum constants (`SurfaceSSH`, `SurfaceWeb`) | Encodes the struct field into the name; use the package for context | `audit.SSH`, `audit.Web` |

When two variables hold the same concept in different representations (typed value and its string form), distinguish by what they represent, not by type: `afiName` (the name) vs `afi` (the numeric code). When scope makes it unambiguous, just use the shorter name.

### Package-Naming Glossary

What the recurring package names mean, verified against each package's own doc comment (`ai/PACKAGE-MAP.md`). When creating a NEW package, pick the term whose definition matches your concern; do not coin a new synonym. The glossary documents existing packages: it does not force renames (existing protocols keep their vocabulary; renames need an explicit user-approved shortlist).

| Term | Means (as a package name) | Canonical examples |
|------|---------------------------|--------------------|
| `packet` | The protocol's wire codec: parse + encode its PDUs/TLVs at the serialization boundary. Preferred term for new protocol codecs. | `component/bfd/packet` (BFD Control codec), `plugins/isis/packet` (PDU/TLV codec, "the protocol's serialization boundary") |
| `message` | Same role as `packet` for protocols whose RFC unit is the "message"; BGP legacy vocabulary. New protocols use `packet`. | `component/bgp/message` (OPEN/UPDATE/NOTIFICATION/KEEPALIVE/ROUTE-REFRESH) |
| `wire` | Wire-level primitives or raw-byte containers shared between layers -- NOT a full codec: buffer writers, raw-packet handoff types. Exception: `ike/wire` is a full codec (predates this glossary). | `core/bgp/wire` (zero-allocation buffer writing), `plugins/ospf/wire` (AF-neutral RawPacket transport->engine handoff) |
| `session` | Per-peer/per-neighbor protocol state: state machine, timers, negotiation for ONE conversation. | `component/bfd/session` (per-session FSM, timer arithmetic, Poll/Final) |
| `fsm` | The RFC-defined state machine when the RFC names it that. | `component/bgp/fsm` (RFC 4271 Section 8) |
| `engine` | The protocol's runtime: the long-lived loop that owns sessions and executes the protocol. Preferred term for new protocol runtimes. | `component/bfd/engine` (express-loop runtime), `component/ike/engine` |
| `transport` | Socket I/O delivering wire bytes to/from the engine; may include an in-memory loopback for tests. | `component/bfd/transport` (UDP I/O + loopback), `plugins/isis/transport` |
| `reactor` | BGP-specific, historical: THE BGP event loop (peer sessions, wire events, plugin dispatch). Do not reuse for new protocols -- use `engine`. | `component/bgp/reactor` |
| `wireu` | "wire UPDATE": lazy-parsed BGP UPDATE messages with zero-copy iterators. Kept name (user decision 2026-07-08, spec-layout-3); a new package with this concern would spell it out. | `component/bgp/wireu` |

<!-- source: ai/PACKAGE-MAP.md -- generated package responsibilities backing each definition -->

### The `cli` / `cmd` / `command` trio (`internal/component/`)

| Package | Is | Use it when |
|---------|----|-------------|
| `component/cli` | The unified interactive TUI: config editing, CLI, and SSH sessions. | Adding an interactive surface or TUI behavior |
| `component/cmd` | A namespace of top-level CLI VERB implementations -- one subpackage per verb (`clear`, `delete`, `log`, `meta`, `metrics`, `monitor`, `set`, `show`, `subscribe`, `update`). | Adding or extending a top-level verb: `component/cmd/<verb>` |
| `component/command` | Shared types and logic for operational command execution (grammar, registry) consumed by the other two. | Adding command plumbing that more than one verb or surface needs |

### The four rib-named packages

| Package | Is |
|---------|----|
| `core/rib/` | Namespace, no root package: `locrib` (unified sharded Loc-RIB arbitrating best paths across protocols), `routeinstall` (RouteSink used by forked route-installing plugins in place of a direct Loc-RIB write), `store` (generic prefix-keyed route store on a BART trie) |
| `component/bgp/rib` | BGP's own RIB with per-attribute-type deduplication (the BGP Adj-RIB layer, distinct from the protocol-neutral Loc-RIB) |
| `core/routingtable` | Maps routing-table NAMES to kernel table IDs (the mapping types) |
| `plugins/routingtable` | The named-routing-table registry engine; wraps and re-exports `core/routingtable` so consumers keep a single import path |

<!-- source: internal/plugins/routingtable/registry.go -- re-export of core/routingtable -->

## Prefer Typed Numeric Over String

Hot paths use typed numeric identity (enum, registered ID, bitset, packed integer), not strings. Across component/engine seams the rule holds plus pointer restrictions (`ai/rules/repo-maintenance.md`).

### Rule

| Surface | Prefer | Reject |
|---------|--------|--------|
| Event/IPC payload crossing component seams | typed `uint8`/`uint16`, registered ID, `netip.Prefix`, `family.Family` | `string` for kinds (protocol, family, action, direction, state) |
| Hot-path dispatch key | integer const / typed enum | string switch |
| Hot-path map key | integer or struct | string |
| Internal state flags | typed enum, zero-invalid | magic strings |
| Hot-path comparison | `x == FooAdd` | `x == "add"` |

- Zero MUST mean `Unspecified` / invalid. The enum type MUST be a distinct `uint8`/`uint16` (not assignable from bare integer literal). `String()` is for diagnostics; code MUST NOT use it for comparison.
- Plugin-extensible sets: numeric ID registered at init (see `spec-bgp-redistribute`, `internal/core/family/family.go`).

### Where Strings Are OK (boundaries only)

| Surface | Why |
|---------|-----|
| Log / diagnostic output | Humans read; `String()` at emit |
| YANG leaf values | Parser converts on load |
| CLI tokens | Parser converts on dispatch |
| JSON wire format | `MarshalText`/`UnmarshalText` on typed value; wire string, Go typed |
| Config file tokens | Parser converts on load |
| Error messages | Human-readable |

### Minimize Conversions

- Code MUST convert to string only at two sinks: external wire (`MarshalText`/`UnmarshalText`) and human output (`String()` returning interned literal or registry name).
- `String()` MUST NOT use: `fmt.Sprintf`, `strconv.Itoa`, `strconv.FormatUint`, `string([]byte{...})`, `strings.Builder`, `+`.
- `fmt.Sprintf` bypasses `AppendTo`/`WriteTo`, so it MUST stay on cold paths only.
- Canonical impl: `internal/core/family/family.go`.

### Map Keys: Prefer Numeric

**BLOCKING on hot paths.** When a map is keyed by a value from a known set, code MUST use a numeric key type (`uint8`, `uint16`, `int`, typed enum), not `string`.

**Pattern: Registry Maps Name to ID at Init, All Lookups Use ID.** Code MUST parse the string once at the boundary (config load, CLI parse, JSON unmarshal), convert to the numeric type, and pass the numeric value everywhere internally. The string exists only at the boundary for human readability.

#### When `map[string]V` Is Acceptable

| Situation | Why |
|-----------|-----|
| Config tree (`map[string]any`) | YANG-parsed, accessed once at load |
| CLI dispatch table (built at init) | Looked up once per command, not per-UPDATE |
| JSON marshal/unmarshal | External format requires string keys |
| User-facing display | One-shot, cold path |
| Map built and discarded in a test | Not production code |

#### Anti-Patterns

| Anti-pattern | Fix |
|-------------|-----|
| `map[string]*Peer` keyed by peer address string | `map[netip.Addr]*Peer` or `map[uint32]*Peer` with `Addr.As4()` |
| Re-parsing the peer string at every map access | Parse ONCE where the JSON IPC event or text command enters the plugin, pass `netip.Addr` to every internal helper (see the RIB plugins for the reference conversion) |
| `map[string]Handler` keyed by command name, looked up per-message | Register commands to `map[uint16]Handler` by numeric code |
| `map[string]bool` as a set of known values | `map[uint8]bool` or bitfield |
| `switch s { case "add": ... case "remove": ... }` on every UPDATE | Parse to enum once, `switch e { case ActionAdd: ... }` |

### Mechanical Check

Before adding a `string` field crossing a component seam OR on a hot path:

**You MUST run these checks before adding a string:**
1. Finite set, compile-time? -> typed enum.
2. Plugin-extensible? -> numeric ID + registry.
3. External contract (YANG/JSON/CLI/log)? -> OK at boundary; convert internally.
4. None of the above? Ask why a string.
5. Does `String()` allocate? -> const literals, or registry + `unsafe.String` on packed store.
6. Consumer parses back to typed? -> emit typed with `MarshalText`; no roundtrip.
7. Map key? -> use numeric type; parse string to numeric at the boundary.

## API Contracts in Comments

When authoring functions with caller obligations, document them in the godoc.

### When to Document

Any function where skipping a step causes a resource leak, deadlock, panic, or silent misbehavior.

| Pattern | Required Comment |
|---------|-----------------|
| Start/Stop/Wait lifecycle | Type doc: full sequence. Stop: "MUST call Wait after". Wait: "Must be called after Stop". |
| Close/cleanup required | "Caller MUST call Close when done" on the constructor |
| Init before use | "MUST call Init before first use" on the type or constructor |
| Call ordering | "MUST be called before/after X" on the dependent function |
| Concurrency safety | "Safe for concurrent use" or "NOT safe for concurrent use" |
| Paired operations (Lock/Unlock, Acquire/Release) | "Caller MUST call Y after X" on X |

### Format

- Use "MUST" (not "SHOULD") for obligations that cause bugs when violated.
- Place the obligation on both sides of the pair: the function that creates the obligation AND the function that fulfills it.

### Checklist (before merging new API)

- [ ] Every resource-acquiring function MUST name how to release it
- [ ] Every multi-step lifecycle MUST be documented on the type
- [ ] Every "call B after A" MUST appear in both A's and B's comments

## File Modularity

### One Concern Per File

Each `.go` source file contains exactly one concern: a cohesive group of types and functions serving a single responsibility.

The line threshold exists for **context economy**. Any task that touches a file must be able to load that file's whole concern, and no unrelated code. The corollary (per Thomas): a split is only worth doing when the separation is RIGHT. A forced mechanical split that scatters one concern across files is worse than one large cohesive file. The post-edit size warning is deliberately non-blocking for this reason. Read it as a prompt to check cohesion, not as an order to cut.

| Lines | Action |
|-------|--------|
| < 1000 | Fine |
| > 1000 | Check for a second concern. Split only when the separation is right |

**You MUST judge file size against 1000 lines, the only threshold** (Thomas, 2026-08-01). A 600-line tier existed before. It fired on cohesive single-concern files. It is gone from this rule, from the post-edit hook, and from `scripts/lint/consistency.go`.

**Before creating a file, you MUST ask "one concern?" Before adding to one, you MUST ask "belongs to this file's concern?" Past 1000 lines, you MUST check for multiple concerns.**

### Splitting

- **Tool:** `go build -o bin/go_extract ./scripts/dev/go_extract.go && bin/go_extract <source.go> <dest.go> <symbol1> [symbol2 ...]` moves named declarations (with doc comments) to dest, runs `goimports` on both. Note: `goimports` cannot resolve aliased imports; you MUST add those manually to the new file.
- Zero semantic effect: Go compiles all files in a package together
- File-local types move with their functions
- Shared test helpers stay in base `_test.go`
- `goimports` handles import cleanup
- You MUST name the file after its concern: `reactor_announce.go`, `session_handlers.go`
- New files: you MUST copy `// Design:` from original, and review the topic annotation (see "Design Document References" below)
- All resulting files: you MUST add `// Related:` to siblings (see "File Cross-References" below)

### Exempt: Test Files

`_test.go` files are not subject to line-count thresholds. Tests grow with coverage and table-driven cases; splitting them adds navigation cost without improving production code clarity.

### NOT a Reason to Split

**Size alone MUST NOT be a reason to split a file that is:**
- Large but single coherent concern (capability registry, pool internals)
- CLI file with one-function-per-subcommand
- Dependency chain where dispatcher references all implementations

## Design Document References

All `.go` source files (non-test, non-generated) MUST have `// Design:` comment.

### Format

- **Format:** you MUST write `// Design: docs/architecture/core-design.md -- topic annotation`
- Topic annotations SHOULD be used over section numbers (they survive restructuring).

### Line Ordering

- The `// Design:` line MUST be the first comment in every file. Only compiler directives (`//go:build`) MAY precede it.
- `// Package` doc comments MUST go after the header block, not before it.

### When to Add

| Situation | Action |
|-----------|--------|
| New file | Add before writing code |
| Split file | Inherit from original |
| Touching file without refs | Add for parts you understand |
| No design doc | `// Design: (none -- predates documentation)` |

### Exempt

`*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`, files starting with `// Code generated`.

## File Cross-References

### Purpose

Cross-reference comments let Claude load only needed files without scanning the whole package. Complements `// Design:` (architecture docs) by pointing to sibling source files.

### Keywords

Three directional keywords express the relationship between files:

| Keyword | Direction | Meaning | Example |
|---------|-----------|---------|---------|
| `// Detail:` | Hub -> Leaf | "details of this topic are in X" | `reactor.go` -> `reactor_api.go` |
| `// Overview:` | Leaf -> Hub | "broader context is in X" | `reactor_api.go` -> `reactor.go` |
| `// Related:` | Peer <-> Peer | "sibling at same level" | `reactor_api_batch.go` <-> `reactor_api_forward.go` |

**Hub file** = orchestrator, core types, dispatch (typically shortest name: `server.go`, `decode.go`, `peer.go`).
**Leaf file** = specific concern split from hub (has suffix: `_text`, `_routes`, `_batch`, or prefix: `cmd_`).
**Peer files** = siblings at same abstraction level, neither contains the other.

### Bidirectionality (BLOCKING)

Every cross-reference MUST have a back-reference. If A references B, B must reference A.

| A says | B must say |
|--------|-----------|
| `// Detail: B.go -- topic` | `// Overview: A.go -- topic` |
| `// Overview: B.go -- topic` | `// Detail: A.go -- topic` |
| `// Related: B.go -- topic` | `// Related: A.go -- topic` |

### Format

Place after `// Design:` at file top. One line per reference with topic annotation:

| Line | Content |
|------|---------|
| 1 | `// Design: docs/architecture/config/syntax.md -- BGP config types` |
| 2 | `// Detail: bgp_routes.go -- route extraction and NLRI parsers` |
| 3 | `// Related: bgp_peer.go -- peer parsing and process bindings` |

### When to Add

| Situation | Action |
|-----------|--------|
| Splitting a file | Hub gets `// Detail:` to leaves, leaves get `// Overview:` to hub |
| Tightly coupled new file | Add reference + matching back-reference |
| Touching file with stale refs | Fix (remove deleted, add missing, fix direction) |

### When NOT to Add

**A file MAY skip a cross-reference when it is:**
- Standalone in package (no strong coupling to siblings)
- Only related through package's public API
- Relationship is obvious from filename alone (see "Not a Directory Listing" below)

### Not a Directory Listing

**`// Detail:` lines SHOULD point to files with non-obvious relationships, not enumerate every file in the package.** If the relationship is self-evident from the filename (e.g., `config.go` has config, `validators.go` has validators), omit it.

**Rule of thumb:** if removing the `// Detail:` line would leave a reader unable to find important code, you SHOULD keep it. If they would find it anyway by scanning filenames, you SHOULD drop it. Aim for 3-5 references maximum per hub file.

### Maintenance

When renaming/deleting a `.go` file, search for `// Detail:`, `// Overview:`, and `// Related:` references to that filename and update/remove.

### Exempt

Same as `// Design:`: `*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`.

## No Backwards Compatibility

### Pre-release (current state)

Ze has never been released. No users. No compat code, comments, shims, or fallbacks anywhere, including the plugin API. If something needs to change, just change it.

### Post-release (future state)

Code under `internal/` is not user-exposed. It follows the no-backwards-compatibility rule forever: change it freely, no shims, no deprecation layers, no "keep the old name working".

**The only exception is the plugin API**, the surface that external plugin authors compile against (`pkg/plugin/` SDK types, the JSON event / text command protocol between core and plugins, and anything re-exported for plugin consumption). Once released, that surface MUST NOT break. Everything else under `internal/` remains free to change.

To be clear: the plugin API's *implementation* can change freely. Only its *contract* (signatures, protocol shape, documented semantics) is frozen post-release.

### ExaBGP compat

External tools only (`ze exabgp plugin`, `ze config migrate`). Engine code: zero ExaBGP format awareness.

## Go Compiler Upgrade Checklist

### textbuf.noescape vs strings.Builder

`internal/core/textbuf/textbuf.go` uses a `noescape` function identical to the technique `strings.Builder` uses via `abi.NoEscape` to prevent self-referential slices from escaping to the heap.

On every Go update:

**After a Go update, you MUST:**
1. Read `$(go env GOROOT)/src/strings/builder.go`, find `copyCheck`.
2. Read `$(go env GOROOT)/src/internal/abi/escape.go`, find `NoEscape`.
3. Compare against `internal/core/textbuf/textbuf.go` `noescape` + `inlineSlice`.
4. If the stdlib changed technique, update ours to match.
5. Verify: `go build -gcflags='-m=2' -o bin/escape-test ./internal/core/textbuf/ 2>&1 | grep 'moved to heap'` SHOULD NOT show `b` escaping for stack-local Buffer usage.

If the Go team removes `NoEscape` or changes escape analysis to see through the `uintptr` round-trip, the inline array optimization breaks and `var b Buffer` reverts to heap allocation. This is not a correctness bug (the code still works), but a performance regression.

## Rationale

Per-topic rationale: `ai/rationale/go-standards.md`, `ai/rationale/naming.md`, `ai/rationale/enum-over-string.md`, `ai/rationale/api-contracts.md`, `ai/rationale/file-modularity.md`, `ai/rationale/design-doc-references.md`, `ai/rationale/related-refs.md`, `ai/rationale/compatibility.md`. See also `ai/rules/performance.md`.

### Why numeric map keys

String map keys cost more than numeric keys at every operation:

| Operation | `map[string]V` | `map[uint16]V` |
|-----------|----------------|-----------------|
| Hash | hash the string bytes (length-dependent) | hash the integer (constant) |
| Compare | byte-by-byte comparison | single integer comparison |
| Key storage | allocates string header + backing bytes per key | inline in map bucket |
| GC scan | GC must scan string pointers | no pointers, no GC scan |

A `map[string]V` with 1000 entries stores 1000 string headers the GC must scan on every collection cycle. A `map[uint16]V` stores inline integers the GC ignores entirely.

### Reference (file modularity and cross-references)

Learned: 363 (file modularity), and 221 before it (the first splitting round). Both were retired on 2026-08-01. Neither was carried into `plan/learned/DESIGN-HISTORY.md`, which records subsystem design and not agent workflow. The header of that file gives the git-recovery route for a retired summary.

## Examples

Registry maps name to ID at init, all lookups use ID:

```go
// At init time (cold path): string -> ID
var familyByName = map[string]family.Family{}

// At runtime (hot path): ID -> data
var ribByFamily = map[family.Family]*RIB{}
```

File header ordering:

```
//go:build linux

// Design: docs/architecture/core-design.md -- topic annotation
// Related: sibling.go -- description
package foo
```

Good hub header (reactor.go, 15 files in package, lists 5 with non-trivial roles):

```
// Detail: reactor_wire.go -- zero-allocation wire UPDATE builders
// Detail: reactor_connection.go -- TCP accept, collision detection (RFC 4271 Section 6.8)
// Detail: forward_pool.go -- per-peer forward worker pool
```

Bad hub header (lists every file, duplicating `ls`):

```
// Detail: config.go -- config parsing
// Detail: validators.go -- validation
// Detail: logger.go -- logging
// Detail: types.go -- type definitions
```
