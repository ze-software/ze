# Prefer Typed Numeric Over String

**BLOCKING:** Hot paths use typed numeric identity (enum, registered
ID, bitset, packed integer), not strings. Across component/engine
seams the rule holds plus pointer restrictions (`rules/memory.md`).

Detail: `.claude/rationale/enum-over-string.md`. See also
`rules/buffer-first.md`.

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

## Mechanical Check

Before adding a `string` field crossing a component seam OR on a hot
path:

1. Finite set, compile-time? -> typed enum.
2. Plugin-extensible? -> numeric ID + registry.
3. External contract (YANG/JSON/CLI/log)? -> OK at boundary; convert internally.
4. None of the above? Ask why a string.
5. Does `String()` allocate? -> const literals, or registry + `unsafe.String` on packed store.
6. Consumer parses back to typed? -> emit typed with `MarshalText`; no roundtrip.
