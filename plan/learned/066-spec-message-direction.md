# 066 — Message Direction

## Objective

Add `direction` ("sent"/"received") to BGP message API output for OPEN, NOTIFICATION, KEEPALIVE, and UPDATE messages, enabling external programs to distinguish sent from received messages.

## Decisions

- Mechanical wiring of direction string through the callback chain: session.go → peer.go → reactor.go → RawMessage.Direction → format functions
- OPEN exchange special case: when receiving peer's OPEN, fire callback for localOpen with "sent" FIRST (chronological order), then for received OPEN with "received"

## Patterns

- `MessageCallback` signature extended to include `direction string` — all call sites updated as a chain

## Gotchas

- Prior output for NOTIFICATION and KEEPALIVE had no direction at all (format: `peer <ip> asn <asn> notification ...`) — the "sent"/"received" distinction was entirely absent

## Files

- `internal/reactor/session.go` — MessageCallback signature, pass "sent"/"received" at call sites
- `internal/component/plugin/types.go` — `Direction string` field in RawMessage
- `internal/component/plugin/text.go` — `FormatOpen`, `FormatNotification`, `FormatKeepalive` use direction
- `internal/component/plugin/server.go` — pass `msg.Direction` to formatters
