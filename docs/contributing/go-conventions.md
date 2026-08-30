# Ze Go Conventions

The Ze-specific conventions a Go author needs: what to call a package, what goes
in a file header, how to reach an environment variable, and where a string is
allowed to cross a seam.

`docs/contributing/ze-go-style.md` is the style guide, and it says how every line
of Go here is written. `ai/rules/go-standards.md` states the obligations. This
page is the reference the two of them point at.

## Naming

### The project's own name

"Ze" is "The" with a French accent. Use "ze" wherever "the" works grammatically.

| Context | Use |
|---------|-----|
| CLI binary | `ze` |
| BGP config YANG | `ze-bgp-conf` |
| BGP JSON format | `ze-bgp` |
| Go variables | `ZeBGPConf*` |
| YANG suffixes | config `-conf`, API `-api` |

### Describe what, never the type

A variable or constant names what the value IS, not its Go type.

| Banned pattern | Why | Fix |
|---------------|-----|-----|
| A `*Str` suffix (`famStr`, `levelStr`, `addrStr`) | Encodes "string" into the name | `family`, `level`, `addr` |
| A `*Int`, `*Bool` or `*Bytes` suffix | The same problem | Name the concept |
| A field or type prefix on an enum constant (`SurfaceSSH`, `SurfaceWeb`) | Encodes the struct field into the name; the package already gives the context | `audit.SSH`, `audit.Web` |

When two variables hold one concept in two representations, separate them by
what they represent rather than by type: `afiName` for the name, `afi` for the
numeric code. Where the scope makes it unambiguous, use the shorter name.

Config-specific naming, for YANG leaves, env var keys and their Go struct
fields, is in `ai/rules/config.md`.

## The package-naming glossary

When you create a NEW package, pick the term below whose definition matches your
concern. Do not coin a synonym. The glossary documents the packages that exist;
it forces no rename, and an existing protocol keeps its own vocabulary.

| Term | Means, as a package name | Canonical examples |
|------|--------------------------|--------------------|
| `packet` | The protocol's wire codec: parse and encode its PDUs and TLVs at the serialization boundary. The preferred term for a new protocol codec | `component/bfd/packet`, `plugins/isis/packet` |
| `message` | The same role as `packet`, for a protocol whose RFC unit is the "message". BGP legacy vocabulary; a new protocol uses `packet` | `component/bgp/message` (OPEN, UPDATE, NOTIFICATION, KEEPALIVE, ROUTE-REFRESH) |
| `wire` | Wire-level primitives or raw-byte containers shared between layers, NOT a full codec: buffer writers, raw-packet handoff types | `core/bgp/wire` (zero-allocation buffer writing), `plugins/ospf/wire` (the AF-neutral raw-packet handoff from transport to engine). `component/ike/wire` is a full codec and predates this glossary |
| `session` | Per-peer protocol state: the state machine, the timers and the negotiation for ONE conversation | `component/bfd/session` |
| `fsm` | The RFC-defined state machine, when the RFC names it that | `component/bgp/fsm` (RFC 4271 Section 8) |
| `engine` | The protocol's runtime: the long-lived loop that owns sessions and executes the protocol. The preferred term for a new protocol runtime | `component/bfd/engine`, `component/ike/engine` |
| `transport` | Socket I/O delivering wire bytes to and from the engine. It may include an in-memory loopback for tests | `component/bfd/transport`, `plugins/isis/transport` |
| `reactor` | BGP-specific and historical: THE BGP event loop, owning peer sessions, wire events and plugin dispatch. Do not reuse it for a new protocol; use `engine` | `component/bgp/reactor` |
| `wireu` | "wire UPDATE": lazy-parsed BGP UPDATE messages with zero-copy iterators. A kept name; a new package with this concern would spell it out | `component/bgp/wireu` |

<!-- source: ai/PACKAGE-MAP.md -- generated package responsibilities backing each definition -->

The per-protocol layout these names compose into is
`docs/architecture/protocol-skeleton.md`.

### The `cli`, `cmd` and `command` trio

All three sit under `internal/component/`, and the difference is load-bearing.

| Package | Is | Use it when |
|---------|----|-------------|
| `component/cli` | The unified interactive TUI: config editing, the CLI, and SSH sessions | Adding an interactive surface or TUI behavior |
| `component/cmd` | A namespace of top-level CLI VERB implementations, one subpackage per verb: `clear`, `delete`, `log`, `meta`, `metrics`, `monitor`, `set`, `show`, `subscribe`, `update` | Adding or extending a top-level verb, at `component/cmd/<verb>` |
| `component/command` | Shared types and logic for operational command execution: the grammar and the registry, consumed by the other two | Adding command plumbing that more than one verb or surface needs |

### The rib-named packages

| Package | Is |
|---------|----|
| `core/rib/` | A namespace with no root package. It holds `locrib` (the unified sharded Loc-RIB, arbitrating best paths across protocols), `routeinstall` (the `RouteSink` a forked route-installing plugin uses in place of a direct Loc-RIB write), `store` (a generic prefix-keyed route store on a BART trie), `igpcost`, and `routetype` |
| `component/bgp/rib` | BGP's own RIB, with per-attribute-type deduplication. This is the BGP Adj-RIB layer, distinct from the protocol-neutral Loc-RIB |
| `core/routingtable` | The mapping types: routing-table NAMES to kernel table IDs |
| `plugins/routingtable` | The named-routing-table registry engine. It wraps and re-exports `core/routingtable` so consumers keep one import path |

<!-- source: internal/plugins/routingtable/registry.go -- re-export of core/routingtable -->

## File headers

Every non-test, non-generated `.go` file opens with a `// Design:` line naming
the document that governs its surface. Cross-reference lines follow it.

### The order

```
//go:build linux

// Design: docs/architecture/core-design.md -- topic annotation
// Related: sibling.go -- description
package foo
```

| Line | Content |
|------|---------|
| 1 | `// Design: docs/architecture/config/syntax.md -- BGP config types` |
| 2 | `// Detail: bgp_routes.go -- route extraction and NLRI parsers` |
| 3 | `// Related: bgp_peer.go -- peer parsing and process bindings` |

The `// Design:` line is the first comment in the file, after any build
constraint.

### When to add a Design line

| Situation | Action |
|-----------|--------|
| New file | Add it before writing code |
| A file split off another | Inherit the original's |
| Touching a file that has none | Add it for the parts you understand |
| No design doc exists | `// Design: (none -- predates documentation)` |

Exempt: `*_test.go`, `*_gen.go`, `register.go`, `embed.go`, `doc.go`, any file
opening with `// Code generated`, and everything under `vendor/`. The line points
at a Ze design document, and an upstream file has none to point at.

### Cross-references

A cross-reference lets a reader load only the files a task needs, instead of
scanning the package. It complements `// Design:` by pointing at sibling source
rather than at architecture documentation.

| Keyword | Direction | Meaning | Example |
|---------|-----------|---------|---------|
| `// Detail:` | Hub to leaf | "the details of this topic are in X" | `reactor.go` to `reactor_api.go` |
| `// Overview:` | Leaf to hub | "the broader context is in X" | `reactor_api.go` to `reactor.go` |
| `// Related:` | Peer to peer | "a sibling at the same level" | `reactor_api_batch.go` and `reactor_api_forward.go` |

A **hub** file is the orchestrator: core types and dispatch, usually with the
shortest name (`server.go`, `decode.go`, `peer.go`). A **leaf** file is one
concern split out of a hub, carrying a suffix (`_text`, `_routes`, `_batch`) or
a prefix (`cmd_`). **Peers** are siblings at the same level of abstraction, where
neither contains the other.

Every cross-reference is bidirectional:

| A says | B must say |
|--------|-----------|
| `// Detail: B.go -- topic` | `// Overview: A.go -- topic` |
| `// Overview: B.go -- topic` | `// Detail: A.go -- topic` |
| `// Related: B.go -- topic` | `// Related: A.go -- topic` |

| Situation | Action |
|-----------|--------|
| Splitting a file | The hub gets `// Detail:` lines to the leaves; each leaf gets `// Overview:` back |
| A tightly coupled new file | Add the reference and its matching back-reference |
| Touching a file with stale references | Fix them: remove the deleted, add the missing, correct the direction |
| Renaming or deleting a `.go` file | Search for `// Detail:`, `// Overview:` and `// Related:` naming it, and update or remove each |

Exempt: the same set as `// Design:`.

### A header is not a directory listing

A `// Detail:` line points at a relationship a reader would otherwise miss. It
does not reproduce `ls`.

Good, from `reactor.go`, which has 15 files in its package and lists the five
with non-trivial roles:

```
// Detail: reactor_wire.go -- zero-allocation wire UPDATE builders
// Detail: reactor_connection.go -- TCP accept, collision detection (RFC 4271 Section 6.8)
// Detail: forward_pool.go -- per-peer forward worker pool
```

Bad, listing every file and duplicating `ls`:

```
// Detail: config.go -- config parsing
// Detail: validators.go -- validation
// Detail: logger.go -- logging
// Detail: types.go -- type definitions
```

## Environment variables

Every Ze environment variable is reached through `internal/core/env`, never
through `os.Getenv` or `os.Setenv`. `os.Getenv` stays correct for a genuine
system variable such as `PATH` or `HOME`.

| Getter | Use |
|---------|-----|
| `env.Get("ze.foo.bar")` | String lookup. Case-insensitive, and dot and underscore are interchangeable |
| `env.GetInt("ze.foo", 0)` | Integer with a default |
| `env.GetInt64("ze.foo", 0)` | Int64 with a default |
| `env.GetBool("ze.foo", false)` | Boolean (`true`/`false`/`1`/`0`) with a default |
| `env.IsEnabled("ze.foo")` | Enabling check: `1`, `true`, `yes`, `on`, `enable`, `enabled` |
| `env.GetDuration("ze.foo", 5*time.Second)` | Duration with a default |

| Setter | Use |
|---------|-----|
| `env.Set("ze.foo", "val")` | String. Updates the cache and the OS environment |
| `env.SetInt("ze.foo", 42)` | Integer |
| `env.SetBool("ze.foo", true)` | Boolean, written as `"true"` or `"false"` |

<!-- source: internal/core/env/env.go -- Get, Set, GetInt, GetInt64, GetBool, IsEnabled, GetDuration, ResetCache -->

A direct `os.Setenv` bypasses the cache, so `env.ResetCache()` follows it.

Every variable is registered with `MustRegister`, and the registration carries
two flags:

| Flag | Meaning |
|------|---------|
| `Private: true` | Hidden from `ze env list` and from autocomplete |
| `Secret: true` | Cleared from the OS environment after the first `Get()`. The value stays in the cache |

<!-- source: internal/core/env/registry.go -- Private, Secret -->

Before adding a variable at all, read `ai/rules/config.md`: the question is
whether it should be YANG config instead, and the env var path mirrors the YANG
path.

## Typed numeric over string

A hot path carries typed numeric identity: an enum, a registered ID, a bitset, or
a packed integer. A string identity on a hot path or across a component seam is
the thing this section exists to prevent.

| Surface | Prefer | Reject |
|---------|--------|--------|
| An event or IPC payload crossing a component seam | a typed `uint8` or `uint16`, a registered ID, `netip.Prefix`, `family.Family` | a `string` for a kind: protocol, family, action, direction, state |
| A hot-path dispatch key | an integer constant or a typed enum | a string switch |
| A hot-path map key | an integer or a struct | a string |
| An internal state flag | a typed enum, with zero invalid | a magic string |
| A hot-path comparison | `x == FooAdd` | `x == "add"` |

Make the zero value invalid, so an unset field cannot pass for a real one.

### Where a string is correct

Strings belong at the boundaries, and the conversion happens there once.

| Surface | Why |
|---------|-----|
| Log and diagnostic output | A human reads it. Call `String()` at the point of emission |
| YANG leaf values | The parser converts on load |
| CLI tokens | The parser converts on dispatch |
| JSON wire format | `MarshalText` and `UnmarshalText` on the typed value: a string on the wire, typed in Go |
| Config file tokens | The parser converts on load |
| Error messages | A human reads it |

| A `map[string]V` is acceptable when | Why |
|-----------------------------------|-----|
| It is the config tree (`map[string]any`) | YANG-parsed, and accessed once at load |
| It is a CLI dispatch table built at `init()` | Looked up once per command, not once per UPDATE |
| It is JSON marshalling | The external format requires string keys |
| It drives user-facing display | One-shot, on a cold path |
| It is built and discarded inside a test | Not production code |

An accepted string key is bound to a constant rather than spelled at each use.

### The anti-patterns

| Anti-pattern | Fix |
|-------------|-----|
| `map[string]*Peer` keyed by the peer address string | `map[netip.Addr]*Peer`, or `map[uint32]*Peer` with `Addr.As4()` |
| Re-parsing the peer string at every map access | Parse ONCE where the JSON IPC event or the text command enters the plugin, then pass `netip.Addr` to every internal helper |
| `map[string]Handler` keyed by command name, looked up per message | Register to `map[uint16]Handler` by numeric code |
| `map[string]bool` as a set of known values | `map[uint8]bool`, or a bitfield |
| `switch s { case "add": ... }` on every UPDATE | Parse to an enum once, then `switch e { case ActionAdd: ... }` |

The shape that resolves all of these is a registry that maps name to ID at
`init()`, after which every lookup uses the ID:

```go
// At init time, on the cold path: string -> ID
var familyByName = map[string]family.Family{}

// At runtime, on the hot path: ID -> data
var ribByFamily = map[family.Family]*RIB{}
```

### Why the numeric key is cheaper

| Operation | `map[string]V` | `map[uint16]V` |
|-----------|----------------|-----------------|
| Hash | Hash the string bytes, cost depending on length | Hash the integer, constant cost |
| Compare | Byte by byte | One integer comparison |
| Key storage | Allocates a string header plus backing bytes per key | Inline in the map bucket |
| GC scan | The GC scans the string pointers | No pointers, so no scan |

A `map[string]V` with 1000 entries gives the garbage collector 1000 string
headers to scan on every cycle. A `map[uint16]V` gives it none.

## API contracts in comments

When a function has a caller obligation, the godoc states it, and it states it
on both sides of a pair.

| Pattern | Required comment |
|---------|------------------|
| A Start, Stop, Wait lifecycle | The type doc carries the full sequence. `Stop` says "MUST call Wait after". `Wait` says "MUST be called after Stop" |
| Close or cleanup required | "Caller MUST call Close when done", on the constructor |
| Init before use | "MUST call Init before first use", on the type or the constructor |
| Call ordering | "MUST be called before X" or "after X", on the dependent function |
| Concurrency safety | "Safe for concurrent use", or "NOT safe for concurrent use" |
| A paired operation (Lock and Unlock, Acquire and Release) | "Caller MUST call Y after X", on X |

The trigger is any function where skipping a step causes a resource leak, a
deadlock, a panic, or silent misbehavior.

## The Go compiler upgrade check

`internal/core/textbuf/textbuf.go` uses a `noescape` function copying the
technique `strings.Builder` uses through `abi.NoEscape`. It stops a
self-referential slice escaping to the heap, which is what lets `var b Buffer`
stay on the stack.

After every Go update, compare `noescape` against the standard library's. If the
Go team removes `NoEscape`, or escape analysis learns to see through the
`uintptr` round trip, the inline-array optimization breaks and `var b Buffer`
reverts to a heap allocation. The code stays correct; the performance does not.
