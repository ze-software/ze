# Typed-Numeric-Over-String Rationale

Why: `.claude/rules/enum-over-string.md` (mechanical reference)
Related: `.claude/rationale/memory.md`, `.claude/rationale/buffer-first.md`

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

## Counter-pressure: YAGNI

Not every string needs numeric identity. The rule targets:

- Cross-component-seam payloads (events, IPC).
- Hot-path comparisons (per-UPDATE, per-route).
- State variables with a small fixed vocabulary.

A one-shot startup string (plugin name in a log line, config file path)
is not a hot path and does not need typed identity. Judgement applies;
the mechanical check in the rule file is the decision aid.
