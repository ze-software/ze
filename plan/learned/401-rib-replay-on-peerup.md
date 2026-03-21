# 401 -- rib-replay-on-peerup

## Objective

Auto-replay all known adj-rib-in routes to a peer when it reaches Established state, so new or reconnecting peers receive routes without waiting for fresh UPDATEs from other peers.

## Decisions

- Replay runs after releasing the write lock to avoid deadlock (buildReplayCommands takes RLock, updateRoute does I/O)
- Reuses existing buildReplayCommands + updateRoute -- no new abstractions
- Full replay (fromIndex=0) on every peer-up; incremental replay deferred to future optimization

## Patterns

- Lock release before I/O: split state mutation (under lock) from side effects (after unlock)
- Self-exclusion already handled by buildReplayCommands (skips targetPeer)
- .ci test uses Python plugin to poll adj-rib-in status and verify replay count

## Gotchas

- Cannot test auto-replay with single peer reconnect: adj-rib-in clears routes on peer-down, so reconnecting peer has nothing to replay from itself
- Unit tests verify data correctness (buildReplayCommands output) not the trigger; .ci test covers wiring

## Files

- `internal/component/bgp/plugins/adj_rib_in/rib.go` -- handleState modified
- `internal/component/bgp/plugins/adj_rib_in/rib_test.go` -- 3 tests added
- `test/plugin/adj-rib-in-replay-on-peerup.ci` -- functional test
- `docs/guide/plugins.md` -- updated description
