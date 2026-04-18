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

## Where Strings Are OK

External surfaces where the string IS the contract. Convert once at the
boundary, use the numeric identity internally.

| Surface | Why strings are OK |
|---------|-------------------|
| Log / diagnostic output | Humans read it; `String()` at emit time |
| YANG leaf values | YANG type is string enum; parser converts on load |
| CLI tokens | User-typed; parser converts on dispatch |
| JSON wire format | External contract; serialiser converts at the boundary |
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
