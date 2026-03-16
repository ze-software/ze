# 068 — Remove Adj-RIB-Out from Router Core

## Objective

Remove per-peer Adj-RIB-Out tracking from the reactor and delegate route persistence to external API programs, simplifying the engine core.

## Decisions

- Route persistence delegated entirely to external programs (e.g., future `zebgp-rr`) via route-refresh capability
- CommitManager (batching via `commit <name> start/end/rollback`) kept — orthogonal concern, not Adj-RIB-Out
- opQueue kept — handles pre-session buffering, unrelated to sent-route tracking
- `internal/rib/` package and `internal/rib/outgoing.go` kept — used by external programs, not the engine
- Per-peer OutgoingRIB transactions (`BeginTransaction`, `CommitTransaction`, `RollbackTransaction`) deprecated; commit batching via CommitManager remains
- API crash is an acceptable failure mode — session goes down, clean restart

## Patterns

- None.

## Gotchas

- This reverses spec-060 (Adj-RIB-Out integration for ForwardUpdate) — `adjRIBOut` field, `MarkSent()`, `RemoveFromSent()`, `GetSentRoutes()`, `FlushAllPending()` calls all removed
- `RIBOutRoutes()`, `ClearRIBOut()`, `FlushRIBOut()` return nil/0 (deprecated, not removed from API)
- `ForwardUpdate()` no longer stores in Adj-RIB-Out — on peer reconnect, external program must re-send routes

## Files

- `internal/reactor/peer.go` — removed adjRIBOut field and all MarkSent/RemoveFromSent calls
- `internal/reactor/reactor.go` — simplified AnnounceRoute, WithdrawRoute, ForwardUpdate
- `internal/component/plugin/handler.go` — removed `rib show out`, `rib clear out`, `rib flush out` commands
- Deleted: `internal/reactor/adjribout_forward_test.go`
