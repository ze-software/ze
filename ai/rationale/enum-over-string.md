# Typed-Numeric-Over-String Rationale

Why: `ai/rules/enum-over-string.md` (mechanical reference)
Related: `ai/rationale/memory.md`, `ai/rationale/buffer-first.md`

## Two concerns, different scope

The rule pushes in the same direction for two different reasons. Keep
them separate so you apply the right one.

### Performance (any hot path, within or across components)

| Cost | What it is | Where it bites |
|------|-----------|----------------|
| Allocation | Every non-literal string conversion (`fmt.Sprintf`, `string([]byte)`, `strconv.Itoa`) may escape to heap | Event payload fields populated per-UPDATE, per-route |
| Slow equality | String `==` is byte-wise compare, O(n); integer `==` is one CPU instruction | Switch on direction/action/state fired per message |
| Heap fragmentation in slices | `[]string{a, b, c}` backing array holds pointers; each string's backing bytes scattered | Batched event payloads and logs |
| No compile-time enforcement | Typo `"sett"` instead of `"set"` compiles; switch misses a new variant silently | State machines, protocol handlers |

These costs are present whether the string lives inside one component or
crosses component seams. Hot-path code in either place pays.

### Contract clarity and isolation (across component seams)

Component seams are the `internal/component/<X>/` boundaries. Crossing
those brings a second concern in addition to performance:

- Strings carried as contracts require both sides to agree on a literal
  that the compiler cannot check. Change one end, break the other silently.
- Registered numeric IDs mediated through a core registry
  (`internal/core/redistevents/`, `internal/core/family/`) give a single
  source of truth for the identity. Each side imports the same type
  definition and looks up its local IDs from the registry.
- Ownership is explicit: the registry stores value-typed metadata
  (IDs, name copies, bits), not pointers into producer-allocated state.
  See `rules/memory.md`.

Within a component, pointers and strings can flow freely -- performance
arguments still apply to hot paths, but the contract concern does not.

## Why this rule exists

The `spec-bgp-redistribute` design crystallised three prior decisions
scattered across learned summaries:

1. Event payloads crossing component seams must be value-typed
   (`memory_feedback_no_cross_boundary_pointers`).
2. Wire-facing paths must not allocate (`rules/buffer-first.md`).
3. Registered names used for dispatch must be constants, not literals
   (`memory.md` "Constants for Command/Status Names").

A string field on a cross-component struct tripped all three at once.
This rule generalises the lesson: pick typed numeric identity for any
discrete value on a hot path, and always for discrete values crossing
component seams.

## The ID pattern (extensible sets)

Concrete example from `spec-bgp-redistribute`:

- Producer side: `events.Register[*RouteChangeBatch]("l2tp", "route-change")` at init; assigns a local typed handle.
- Producer also calls `redistevents.RegisterProducer(ProtocolL2TP)` where `ProtocolL2TP` came from `RegisterProtocol("l2tp")`.
- Consumer side: enumerates `redistevents.Producers()` at `OnStarted`. For each `ProtocolID`, looks up the name via `redistevents.ProtocolName(id)` once, then calls `events.Register[*RouteChangeBatch](name, "route-change")` in its own package to obtain its own typed handle.
- Hot path compare: `batch.Protocol == ProtocolL2TP` -- integer eq, no alloc.

Registration identity is by `(namespace, eventType, T)` contract, not by
pointer. Two packages calling `events.Register` with the same tuple get
equivalent handles. The registry carries only value-typed metadata --
no pointers cross component seams.

## Where strings remain OK, explicitly

Strings at boundaries map cleanly to numeric identity internally:

```
YANG "redistribute { import l2tp { family ipv4/unicast; } }"
  -> parser -> config.ImportRule{ Source: ProtocolL2TP, Families: {family.IPv4Unicast} }
```

The parser is the single place string-to-numeric conversion happens.
Every downstream call compares integers.

JSON output reverses at the edge:

```
enum FooAdd (uint8 = 1)
  -> Stringer "add" in a Prometheus label OR a JSON response
```

If a struct field holds a `string "add"` through three hops of code, any
one of those hops is the wrong place to compare bytes.

## Why zero is invalid

Zero values surface corruption. A struct from a pool or deserialised
from a partial payload starts with zero fields. If `FooUnspecified == 0`
is a valid enum value, bugs are silent. If it is invalid, the first
compare against `FooAdd` or `FooRemove` fails loud and the bug
surfaces at its actual source.

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

For plugin-extensible sets (not known at compile time), use a numeric
ID registered at init (see the ID-pattern section above for the full
producer/consumer flow):

```go
type ProtocolID uint16
var _ = redistevents.RegisterProtocol("bgp") // returns a ProtocolID
```

## Anti-Patterns

| Anti-pattern | Why wrong | Fix |
|-------------|-----------|-----|
| `Protocol string` on an event payload crossing components | Per-emit alloc; contract enforced by convention only | `Protocol ProtocolID` via registry |
| `Action string // "add" or "remove"` | No compile-time enforcement; typo silent | typed `uint8` enum |
| `map[string]bool` on the hot path | String rehash per lookup, heap per key | `map[FamilyID]bool` or bitset |
| `switch state { case "up": ... }` | Compiler cannot detect missing case | typed enum + `exhaustive` lint |
| `fmt.Sprintf("%s:%d", proto, port)` as a key | Alloc per lookup | struct key or packed `uint64` |

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

Converting legacy sites is not required with this rule landing. The
rule blocks new regressions.

## Counter-pressure: YAGNI

Not every string needs numeric identity. The rule targets:

- Cross-component-seam payloads (events, IPC).
- Hot-path comparisons (per-UPDATE, per-route).
- State variables with a small fixed vocabulary.

A one-shot startup string (plugin name in a log line, config file path)
is not a hot path and does not need typed identity. Judgement applies;
the mechanical check in the rule file is the decision aid.
