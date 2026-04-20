# Prefer Typed Numeric Over String

**BLOCKING:** On hot paths, represent discrete values as typed numeric
identities (enum, registered ID, bitset, packed integer), not strings.
Across component/engine boundaries, the same rule holds plus pointer
restrictions (see `rules/memory.md`).

Rationale: `.claude/rationale/enum-over-string.md`
Related: `rules/memory.md` (no pointers across component/engine seams),
`rules/buffer-first.md` (no per-event allocation on wire paths),
`spec-bgp-redistribute` payload design (numeric `ProtocolID`).

"Enum" in this rule means any typed numeric identity -- `iota` const
blocks, registered IDs (`ProtocolID uint16`), bitsets, packed
`uint64`s. Not only Go-style `iota` groups.

## Two Reasons, Different Scope

| Reason | Where it applies | What it costs |
|--------|------------------|---------------|
| Performance | Any hot path (within a component or across) | String equality is O(n) memcmp; integer equality is one CPU instruction. `fmt.Sprintf`, `string([]byte)`, `strconv.Itoa` escape to heap. `[]string` backing holds pointers per element |
| Contract clarity + isolation | Across component/engine boundaries (`internal/component/<X>/` ↔ `<Y>/`) | Strings are opaque contracts: both sides must agree on the literal. Typos compile. Registry-mediated numeric IDs enforce uniqueness + give a translation layer at each end |

Within a single component (e.g. `internal/component/bgp/**`), only the
performance reason applies. Strings are allowed as values; pointers
within the component are allowed. Across component seams, both reasons
stack on top of `rules/memory.md`'s pointer restriction.

## Rule

| Surface | Prefer | Reject |
|---------|--------|--------|
| Event / IPC payload fields crossing component seams | typed `uint8`/`uint16` enum, registered numeric ID, `netip.Prefix`, `family.Family` | `string` fields for kinds (protocol, family, action, direction, state) |
| Hot-path dispatch key | integer const / typed enum | string switch |
| Map keys on hot paths | integer or struct | string |
| Internal state flags | typed enum with zero-invalid | magic strings (`"up"`/`"down"`/`"replaying"`) |
| Comparison on hot paths | `x == FooAdd` | `x == "add"` |

## Pattern

```go
type FooAction uint8

const (
    FooUnspecified FooAction = 0 // zero invalid -- surfaces corruption
    FooAdd         FooAction = 1
    FooRemove      FooAction = 2
)

func (a FooAction) String() string {
    switch a {
    case FooAdd:
        return "add"
    case FooRemove:
        return "remove"
    default:
        return "unspecified"
    }
}
```

Rules:
- `0` is always `Unspecified` / invalid -- uninitialised values surface immediately.
- Enum type is a distinct `uint8`/`uint16` so it is not assignable from a bare integer literal.
- String form lives in `String()` for diagnostics -- never for comparison.

For plugin-extensible sets (set not known at compile time), use a numeric
ID registered at init, following the `spec-bgp-redistribute` pattern:

```go
type ProtocolID uint16
var _ = redistevents.RegisterProtocol("bgp") // returns a ProtocolID
```

Consumers compare on the ID. Name lookup is a separate call used only
for diagnostics.

## Minimize Conversions; Intern When You Must

Every `value.String()` is a potential allocation; every `parse(s)` on the
consumer side undoes the typed representation. Both are waste unless the
caller is a true output sink. Only two sink categories exist:

| Sink | Mechanism |
|------|-----------|
| External wire (JSON, text command) | `MarshalText` / `UnmarshalText` on the typed value -- both sides hold typed, wire stays string |
| Human output (log, CLI display, error) | `String()` returning an interned literal or registry-backed name |

**Canonical implementation:** `internal/core/family/family.go` --
`Family.String`, `Family.AppendTo`, `Family.MarshalText`,
`Family.UnmarshalText`. Registered-name path is zero-alloc via a packed
back-store + `unsafe.String`. Every new typed enum should add these four
methods at declaration time. Compile-time enums use const-literal
switches (Go interns literals); plugin-extensible sets use a packed
registry like `family`.

**No conversion roundtrips.** If the consumer on the other side of an
event/RPC parses the string back to a typed value, the field should be
typed with `MarshalText` attached. Wire format is unchanged; Go-to-Go
roundtrip cost disappears.

**Banned in `String()`:** `fmt.Sprintf`, `strconv.Itoa`,
`strconv.FormatUint`, `string([]byte{...})`, `strings.Builder`, `+`
concatenation. Use const literals (known values) or registry +
`unsafe.String` (plugin-extensible). Fallback for unregistered values
uses `strconv.AppendUint` via `AppendTo`, never a fresh allocation.

**`fmt.Sprintf` is not an escape hatch.** It bypasses `AppendTo` and
`WriteTo` -- even when the type provides a zero-alloc append path,
`fmt.Sprintf` ignores it and materialises a new string via reflection.
This applies everywhere, not just in `String()`: anywhere you reach for
`fmt.Sprintf` to build a string from typed values, you are discarding
the append path. Build into a caller's buffer with `AppendTo`, or return
an interned literal. `fmt.Sprintf` is only acceptable for genuinely
one-shot cold paths (errors, fatal logs, setup code).

## Where Strings Are OK

External surfaces where the string IS the contract. Convert once at the
boundary, use the numeric identity internally.

| Surface | Why strings are OK |
|---------|-------------------|
| Log / diagnostic output | Humans read it; `String()` at emit time |
| YANG leaf values | YANG type is string enum; parser converts on load |
| CLI tokens | User-typed; parser converts on dispatch |
| JSON wire format | External contract; attach `MarshalText`/`UnmarshalText` to the typed value so Go holds typed on both sides, wire stays string |
| Config file tokens | File syntax is string; parser converts on load |
| Error messages | Human-readable context |

Strings carried as values (not pointers) across component seams are not
forbidden by `rules/memory.md`; the performance argument above is why
this rule still pushes toward numeric identity on anything that fires
per-event.

## Anti-Patterns

| Anti-pattern | Why wrong | Fix |
|-------------|-----------|-----|
| `Protocol string` on an event payload crossing components | Per-emit alloc; contract enforced by convention only | `Protocol ProtocolID` via registry |
| `Action string // "add" or "remove"` | No compile-time enforcement; typo silent | typed `uint8` enum |
| `map[string]bool` on the hot path | String rehash per lookup, heap per key | `map[FamilyID]bool` or bitset |
| `switch state { case "up": ... }` | Compiler cannot detect missing case | typed enum + `exhaustive` lint |
| `fmt.Sprintf("%s:%d", proto, port)` as a key | Alloc per lookup | struct key or packed `uint64` |

## Mechanical Check

Before adding a `string` field on a struct crossing a component seam OR
sitting on a hot path:

1. Is the set of values finite and known at compile time? -> Typed enum.
2. Is the set extensible by plugins? -> Numeric ID + registry.
3. Is the string the external contract (YANG / JSON / CLI / log)? -> OK at the boundary only; convert to an internal numeric identity.
4. None of the above? Still suspect; ask why a string.
5. Does `String()` call `fmt.Sprintf`, `strconv.Itoa`, or allocate a new string? -> Replace with const literals or a registry lookup backed by `unsafe.String` on a packed store.
6. Does any consumer parse the emitted string back to typed form? -> Emit typed with `MarshalText`/`UnmarshalText`; don't round-trip through strings.

## Legacy Call-Outs

Pre-existing string-on-hot-path cases the rule targets for future
conversion. These are WITHIN the BGP component (no cross-seam concern);
the performance argument is what makes them legacy.

| Location | Today | Intended shape |
|----------|-------|---------------|
| `rs.peers map[string]*PeerState` | string peer addr key | `map[netip.AddrPort]*PeerState` (matches reactor `fwdKey`) |
| `rs.withdrawals map[string]map[string]withdrawalInfo` | nested string | integer peer ID + structured route key |
| `rs.peers[addr].Families map[string]bool` | family name string | `map[family.Family]bool` or bitset |
| `PeerState.Replaying bool` combined with scattered string state switches | string compares | typed `PeerState` enum |

Converting legacy sites is not required with this rule landing. The rule
blocks new regressions.
