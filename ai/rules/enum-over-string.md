# Prefer Typed Numeric Over String

**When:** Hot paths use typed numeric identity (enum, registered
**Severity:** blocking

## Directives

Hot paths use typed numeric identity (enum, registered
ID, bitset, packed integer), not strings. Across component/engine
seams the rule holds plus pointer restrictions (`ai/rules/project-knowledge.md`).

Detail: `ai/rationale/enum-over-string.md`. See also
`ai/rules/buffer-first.md`.

## Rule

| Surface | Prefer | Reject |
|---------|--------|--------|
| Event/IPC payload crossing component seams | typed `uint8`/`uint16`, registered ID, `netip.Prefix`, `family.Family` | `string` for kinds (protocol, family, action, direction, state) |
| Hot-path dispatch key | integer const / typed enum | string switch |
| Hot-path map key | integer or struct | string |
| Internal state flags | typed enum, zero-invalid | magic strings |
| Hot-path comparison | `x == FooAdd` | `x == "add"` |

Zero = `Unspecified` / invalid. Enum type distinct `uint8`/`uint16`
(not assignable from bare integer literal). `String()` is for
diagnostics, never comparison.

Plugin-extensible sets: numeric ID registered at init (see
`spec-bgp-redistribute`, `internal/core/family/family.go`).

## Where Strings Are OK (boundaries only)

| Surface | Why |
|---------|-----|
| Log / diagnostic output | Humans read; `String()` at emit |
| YANG leaf values | Parser converts on load |
| CLI tokens | Parser converts on dispatch |
| JSON wire format | `MarshalText`/`UnmarshalText` on typed value; wire string, Go typed |
| Config file tokens | Parser converts on load |
| Error messages | Human-readable |

## Minimize Conversions

- Two sinks only: external wire (`MarshalText`/`UnmarshalText`) and
  human output (`String()` returning interned literal or registry
  name).
- Banned in `String()`: `fmt.Sprintf`, `strconv.Itoa`,
  `strconv.FormatUint`, `string([]byte{...})`, `strings.Builder`, `+`.
- `fmt.Sprintf` bypasses `AppendTo`/`WriteTo` -- cold paths only.
- Canonical impl: `internal/core/family/family.go`.

## Map Keys: Prefer Numeric

**BLOCKING on hot paths.** When a map is keyed by a value from a known
set, use a numeric key type (`uint8`, `uint16`, `int`, typed enum), not
`string`.

### Why

String map keys cost more than numeric keys at every operation:

| Operation | `map[string]V` | `map[uint16]V` |
|-----------|----------------|-----------------|
| Hash | hash the string bytes (length-dependent) | hash the integer (constant) |
| Compare | byte-by-byte comparison | single integer comparison |
| Key storage | allocates string header + backing bytes per key | inline in map bucket |
| GC scan | GC must scan string pointers | no pointers, no GC scan |

A `map[string]V` with 1000 entries stores 1000 string headers the GC
must scan on every collection cycle. A `map[uint16]V` stores inline
integers the GC ignores entirely.

### Pattern: Registry Maps Name to ID at Init, All Lookups Use ID

```go
// At init time (cold path): string -> ID
var familyByName = map[string]family.Family{}

// At runtime (hot path): ID -> data
var ribByFamily = map[family.Family]*RIB{}
```

Parse the string once at the boundary (config load, CLI parse, JSON
unmarshal), convert to the numeric type, and pass the numeric value
everywhere internally. The string exists only at the boundary for
human readability.

### When `map[string]V` Is Acceptable

| Situation | Why |
|-----------|-----|
| Config tree (`map[string]any`) | YANG-parsed, accessed once at load |
| CLI dispatch table (built at init) | Looked up once per command, not per-UPDATE |
| JSON marshal/unmarshal | External format requires string keys |
| User-facing display | One-shot, cold path |
| Map built and discarded in a test | Not production code |

### Anti-Patterns

| Anti-pattern | Fix |
|-------------|-----|
| `map[string]*Peer` keyed by peer address string | `map[netip.Addr]*Peer` or `map[uint32]*Peer` with `Addr.As4()` |
| Re-parsing the peer string at every map access | Parse ONCE where the JSON IPC event or text command enters the plugin, pass `netip.Addr` to every internal helper (see the RIB plugins for the reference conversion) |
| `map[string]Handler` keyed by command name, looked up per-message | Register commands to `map[uint16]Handler` by numeric code |
| `map[string]bool` as a set of known values | `map[uint8]bool` or bitfield |
| `switch s { case "add": ... case "remove": ... }` on every UPDATE | Parse to enum once, `switch e { case ActionAdd: ... }` |

## Mechanical Check

Before adding a `string` field crossing a component seam OR on a hot
path:

1. Finite set, compile-time? -> typed enum.
2. Plugin-extensible? -> numeric ID + registry.
3. External contract (YANG/JSON/CLI/log)? -> OK at boundary; convert internally.
4. None of the above? Ask why a string.
5. Does `String()` allocate? -> const literals, or registry + `unsafe.String` on packed store.
6. Consumer parses back to typed? -> emit typed with `MarshalText`; no roundtrip.
7. Map key? -> use numeric type; parse string to numeric at the boundary.
