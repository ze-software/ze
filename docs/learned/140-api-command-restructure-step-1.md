# 140 — API Command Restructure Step 1: IPC Protocol JSON Format

## Objective

Add `"type"` field to all JSON messages and nest event data under a typed payload key (`"bgp"` or `"rib"`), nest path attributes under `"attributes"`, and nest NLRI families under `"nlri"`. Wrap responses under `{"type":"response","response":{...}}`.

## Decisions

- Chose `ResponseWrapper` struct wrapping at serialization time rather than modifying every `Response` creation call — only the 6 serialization points in `server.go` needed updating.
- Text format (non-JSON mode) left unchanged — the new JSON wrapper only applies to JSON IPC.
- `WrapResponse()`, `NewResponse()`, `NewErrorResponse()` added as helpers.

## Patterns

- Top-level `type` indicates which key contains the payload (`"bgp"`, `"rib"`, `"response"`). Consumers read `type`, then access the named key — eliminates format detection heuristics.
- `bgp.type` field carries the event type (`update`, `open`, `state`, etc.); `message` object carries wire metadata (id, direction).

## Gotchas

- `ResponseWrapper` was created but not integrated into actual code paths in first pass — 5 additional serialization points in `handleProcessCommand`, `handleRegisterCommand` (×2), `handleUnregisterCommand` (×2) were missed. Always grep for all serialization sites.

## Files

- `internal/plugin/types.go` — `ResponseWrapper`, `WrapResponse()`, `NewResponse()`, `NewErrorResponse()`
- `internal/plugin/server.go` — all 6 response serialization points wrapped
- `internal/plugin/json.go` — `marshal()` outputs IPC Protocol format
- `internal/plugin/rib/event.go` — `parseEvent` updated for IPC Protocol wrapper
