# 060 — Adj-RIB-Out Integration for Forward

## Objective

Persist forwarded routes in adj-rib-out so peers can replay them on reconnect, using the existing OutgoingRIB infrastructure.

## Decisions

- Routes converted from ReceivedUpdate to individual Route objects (parse once, reuse for all peers)
- Forwarding must check `updateSize > destMaxSize` BEFORE attempting zero-copy — a 65535-byte UPDATE from an Extended Message peer cannot be sent as-is to a non-Extended peer (max 4096 bytes)
- When splitting is required: parse and send routes individually (batching deferred to spec-061)
- Concurrent ForwardUpdate calls that announce then withdraw the same prefix are an API usage concern, not a bug — callers must sequence conflicting operations

## Patterns

- `ConvertToRoutes()` on ReceivedUpdate extracts NextHop and ASPath from AttributesWire, creates one Route per NLRI with wire cache attached
- `MarkSent()` after send, `RemoveFromSent()` for withdrawals — standard OutgoingRIB contract
- Reconnect replay already existed in `sendInitialRoutes()` — no new logic needed, just populate the RIB

## Gotchas

- **Known limitation removed later (spec-068):** adj-rib-out integration was subsequently removed from the router core and delegated to external API programs. The spec-060 code no longer exists in the form described here.
- Route batching not implemented in the split path — sends one route per UPDATE (O(routes) messages)
- `FlushAllPending` mid-loop failure cannot occur in practice until RFC 7606 is implemented; any send error tears down the session

## Files

- `internal/reactor/received_update.go` — `ConvertToRoutes()`
- `internal/reactor/reactor.go` — `ForwardUpdate()` with MarkSent integration, `sendSplitUpdates()`
- `internal/reactor/peer.go` — reconnect replay with size checking
