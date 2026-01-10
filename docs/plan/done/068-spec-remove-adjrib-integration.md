# Remove Adj-RIB-Out Integration from Router Core

**Status:** ✅ Completed

## Problem

Router integrated Adj-RIB-Out for tracking sent routes. Delegated to external API program (`zebgp-rr`).

## Decisions

1. **Transactions:** Option A - removed per-peer transactions entirely
   - CommitManager (`commit <name> start/end/rollback`) remains for batching
   - Per-peer OutgoingRIB transactions deprecated

2. **opQueue:** Kept - unrelated to Adj-RIB-Out
   - Handles pre-session buffering (different concern)

3. **Re-announcement on reconnect:** Removed
   - External API program must re-send routes via refresh capability

4. **zebgp-rr:** TODO, not a prerequisite
   - Capability contract spec exists: `docs/plan/spec-api-capability-contract.md`

5. **API crash:** Acceptable failure mode
   - Session goes down anyway, clean restart

## Changes Made

### pkg/reactor/peer.go
- Removed `adjRIBOut` field and `AdjRIBOut()` accessor
- Updated doc comments
- Removed `adjRIBOut.MarkSent()`, `RemoveFromSent()`, `GetSentRoutes()`, `FlushAllPending()` calls
- Simplified `sendInitialRoutes()` - no longer re-sends Adj-RIB-Out routes
- opQueue processing no longer tracks sent state

### pkg/reactor/reactor.go
- Removed `ribOut` field from Reactor struct
- Simplified `AnnounceRoute()`, `WithdrawRoute()` - removed transaction branches, sent tracking
- Simplified `AnnounceLabeledUnicast()`, `WithdrawLabeledUnicast()` - same
- `RIBOutRoutes()` returns nil (deprecated)
- `RIBStats()` - OutPending/OutWithdrawls/OutSent always 0
- `ClearRIBOut()`, `FlushRIBOut()` return 0 (deprecated)
- `BeginTransaction()`, `CommitTransaction()`, `RollbackTransaction()` return error (deprecated)
- `InTransaction()` returns false, `TransactionID()` returns "" (deprecated)
- Removed `convertRIBError()` (unused)
- `ForwardUpdate()` no longer tracks in Adj-RIB-Out

### pkg/plugin/handler.go
- Removed `rib show out`, `rib clear out`, `rib flush out` commands
- Updated help text

### pkg/plugin/types.go
- Added deprecation notices to interface methods

### pkg/plugin/handler_test.go
- Removed `TestRIBClearOut`, `TestRIBFlushOut`
- Updated `TestRIBCommandsRegistered` to check only `rib show in`, `rib clear in`

### Deleted
- `pkg/reactor/adjribout_forward_test.go`

## What Stays

| Item | Reason |
|------|--------|
| `pkg/rib/` package | Used by external `zebgp-rr` program |
| `pkg/rib/outgoing.go` | Adj-RIB-Out implementation for external use |
| `pkg/plugin/commit.go` | CommitManager for batching |
| `pkg/plugin/commit_manager.go` | Transaction batching via `commit <name>` |

## Commit Batching

Routes can still be batched via CommitManager:

```
commit txn-1 start
commit txn-1 announce route 10.0.0.0/24 next-hop 192.168.1.1
commit txn-1 announce route 10.0.1.0/24 next-hop 192.168.1.1
commit txn-1 end                    # flush (no EOR)
commit txn-1 eor                    # flush + send EOR
commit txn-1 rollback               # discard
```

This is independent of the per-peer Adj-RIB-Out transactions that were removed.

## Related Specs

- `docs/plan/spec-api-capability-contract.md` - defines route-refresh handling
- TODO: `zebgp-rr` reference program
