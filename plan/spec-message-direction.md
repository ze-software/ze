# Spec: message-direction

## Task
Add `direction` ("sent"/"received") to BGP message API output for OPEN, NOTIFICATION, KEEPALIVE, and UPDATE messages.

## Required Reading (MUST complete before implementation)

- [x] `.claude/zebgp/api/ARCHITECTURE.md` - Message flow: Session → Peer → Reactor → Server

**Key insights from docs:**
- `MessageCallback` defined in session.go, wired through peer.go to reactor.go
- `RawMessage` in types.go carries message data to Server
- Format functions in text.go output API strings
- Per-peer bindings control which messages go to which processes

## Files to Modify

| File | Changes |
|------|---------|
| `pkg/reactor/session.go:41` | Update `MessageCallback` signature to add `direction string` |
| `pkg/reactor/session.go:562-564` | Pass `"received"` in `processMessage()` |
| `pkg/reactor/session.go:1006` | Fire callback with `"sent"` after `sendOpen()` |
| `pkg/reactor/session.go:1011` | Fire callback with `"sent"` after `sendKeepalive()` |
| `pkg/reactor/session.go:1021` | Fire callback with `"sent"` after `sendNotification()` |
| `pkg/reactor/peer.go` | Signature update propagates automatically |
| `pkg/reactor/reactor.go:2415` | Update `notifyMessageReceiver` to accept/set direction |
| `pkg/api/types.go:448` | Add `Direction string` to `RawMessage` |
| `pkg/api/text.go` | Update `FormatOpen`, `FormatNotification`, `FormatKeepalive` to use direction |
| `pkg/api/server.go:452-488` | Pass `msg.Direction` to formatters |

## Current State
- Tests: `make test` PASS, `make lint` 0 issues, `make functional` 37/37
- Last commit: `ccc6f15` (API OPEN format improvements)

## Special Case: OPEN Exchange

When receiving peer's OPEN in `processMessage()`:
1. Fire callback for `localOpen` with `"sent"` FIRST (pack s.localOpen)
2. Fire callback for received OPEN with `"received"` (existing behavior)

This outputs both messages in chronological order.

## Output Format

**Before:**
```
peer 10.0.0.1 received open asn 65001 router-id 1.1.1.1 hold-time 90 ...
peer 10.0.0.1 asn 65001 notification code 6 subcode 2 ...
peer 10.0.0.1 asn 65001 keepalive
```

**After:**
```
peer 10.0.0.1 sent open asn 65000 router-id 2.2.2.2 hold-time 90 ...
peer 10.0.0.1 received open asn 65001 router-id 1.1.1.1 hold-time 90 ...
peer 10.0.0.1 sent notification code 6 subcode 2 ...
peer 10.0.0.1 received keepalive
```

## Implementation Steps

1. Write test for `FormatOpen` with direction param (TDD)
2. See test fail
3. Update `RawMessage.Direction` field
4. Update `MessageCallback` signature
5. Update all call sites in session.go
6. Update `notifyMessageReceiver` in reactor.go
7. Update format functions in text.go
8. Update `formatMessage` in server.go
9. Run `make test && make lint && make functional`

## Checklist
- [ ] Required docs read
- [ ] Test fails first
- [ ] Test passes after impl
- [ ] make test passes
- [ ] make lint passes
- [ ] make functional passes
