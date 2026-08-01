# 1142: Enum-Over-String for Text Event Pipeline

## What

Wired existing typed enums (`bgptypes.RouteAction`, `rpc.EventKind`) into the text-format event pipeline. Previously only the structured event path used typed enums; the text/JSON path compared raw strings (`op.Action == "add"`, `eventType == "sent"`). Added `EventKindSent` and `EventKindNegotiated` to the `rpc.EventKind` enum. Changed `FamilyOperation.Action` from `string` to `bgptypes.RouteAction`, `MessageInfo.Type` from `string` to `rpc.EventKind`, and `GetEventType()` from returning `string` to returning `rpc.EventKind`.

## Key Decisions

- **`Event.Type` stays `string`.** It carries non-BGP event types like `"cache"` and `"request"` that don't belong in `EventKind`. `MessageInfo.Type` (always a BGP message type) was changed to `EventKind`. `GetEventType()` converts at the boundary.
- **Lazy cache for `TypeKind`.** `ParseEvent` eagerly populates `Event.TypeKind` from the parsed string. For events constructed directly (not via `ParseEvent`), `GetEventType()` lazily parses on first call. This avoids a per-call `[]byte` allocation while keeping test construction simple.
- **`jsonKey` split from event kind.** `ParseEvent` needs the event type as a string for JSON key lookup (`rawPayload[jsonKey]`). The typed `EventKind` is stored separately. The `"sent"` event type maps to `"update"` for JSON key lookup but `EventKindSent` for dispatch.
- **`UnmarshalText` returns error for unknowns.** Keeps typo detection. `ParseEvent` intentionally ignores the error via `_` for non-BGP types. `MarshalText` rejects `EventKindUnspecified`, preserving the asymmetry: you can't marshal what you can't unmarshal.

## Gotchas

- **Local `familyOperation` types are independent.** `format/text_json.go`, `plugins/rs/server_text.go`, `plugins/rr/withdrawal.go`, and `plugins/persist/server.go` each have their own `familyOperation` or `persistFamilyOp` struct with `Action string`. These are internal to their respective packages and parse from their own data sources. Only `bgp.FamilyOperation` (the exported type in `event.go`) was changed.
- **NLRI index `map[string]` keys are not improvable.** The RIB maps use `string([]byte)` as map keys (Go's canonical `map[[]byte]V` idiom). The compiler optimizes `m[string(b)]` lookups to avoid allocation. Replacing with numeric handles would add complexity and indirection for no gain.
- **Attrpool dedup index is already zero-copy.** The `map[string]Handle` in `attrpool/pool.go` uses `unsafe.String` pointing into the data buffer. Further optimization would require a custom hash table.
- **`EventKindCount` changed from 10 to 12.** Arrays sized by `EventKindCount` auto-adjust. No external plugin uses the numeric value directly.

## Pattern: Boundary-Only String, Typed Internally

When a JSON field carries a fixed set of values but also needs to handle unknown extensions:
1. Keep the JSON struct field as `string` if it must accept values outside the enum.
2. Add a `json:"-"` typed field alongside for internal dispatch.
3. Parse string to enum once at the boundary (`ParseEvent`), cache in the typed field.
4. All internal dispatch uses the typed field, never string comparison.

This avoids the all-or-nothing choice between "change the JSON type" (breaks unknown values) and "keep everything as strings" (no type safety). `Event.Type` (string, any value) plus `Event.TypeKind` (EventKind, BGP values only) is the pattern.

## Files

None recorded.
