# 766 — JSON-Safe String Append (Skip Per-Byte Escape)

## Context

Every JSON-formatted BGP message called `appendJSONString` for direction
strings, peer names, and group names. That function does a per-byte switch
to escape `\`, `"`, and control chars. For strings that are guaranteed safe
by a prior boundary (config validation or bounded enum), the per-byte
dispatch is pure waste.

## Key Decisions

### Validate once at boundary, append raw on hot path

Added `IsJSONSafe(s) bool` for boundary validation and
`appendJSONSafeString(buf, s) []byte` that does a raw `append(buf, s...)`.
The safety contract is: callers must guarantee input is clean, either by
construction (enum `.String()`) or by prior validation (`IsJSONSafe` at
config load).

### naming.ValidateNodeName already guarantees JSON safety

Peer names and group names pass through `naming.ValidateNodeName` which
restricts to `[a-zA-Z0-9_.-]`. This character set is a strict subset of
JSON-safe bytes. Wired `IsJSONSafe` into `validatePeerName` and
`validateGroupName` as defense-in-depth, not as the primary guarantee.

### NLRI fallback stays with full escaping

The `n.String()` path in `appendNLRIJSON` is a fallback for NLRI types
that don't implement `JSONAppender` (EVPN, FlowSpec). Their `.String()`
output is not validated at any boundary, so it must use `appendJSONString`.

## Results

| Input | appendJSONString | appendJSONSafeString | Speedup |
|-------|-----------------|---------------------|---------|
| "received" (8B) | 24ns | 2.5ns | 10x |
| "core-rr1.fra.example" (20B) | 55ns | 2.5ns | 22x |

## Traps

- Dynamic peers set `Name: "dyn-" + addr.String()`. IP address strings
  contain only `[0-9a-f.:]`, all JSON-safe. No validation needed.
- The `parsePeersFromTree` fallback in `reactor_api.go` (used in tests
  only) does not call `validatePeerName`. Production path goes through
  `reloadFunc` which uses the full config pipeline.

## Files

None recorded.
