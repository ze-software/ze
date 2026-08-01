# 772 -- NH Resolution Cascade Wiring

## Context

The nhResolver already provided Track/Untrack/Dependents for dependency tracking between next-hops and prefixes, but these were never called from production code. When a covering route changed (IGP link down, metric change), prefixes using that next-hop stayed in the FIB with stale resolved addresses. This spec wired the cascade so covering-route changes automatically re-evaluate and re-emit all dependent prefixes.

## Decisions

- Chose async channel worker over synchronous OnChange processing: the locrib OnChange handler runs under a shard write lock, and processEvent calls resolveNextHop which calls LPM which iterates all shards and tries to read-lock them. Same-shard re-locking deadlocks. Moving all processing to an async worker eliminates the deadlock entirely.
- Chose inline cascade (in the same worker goroutine) over a separate cascade channel: since both change processing and cascade need sysRIB.mu and call resolveNextHop, combining them in one goroutine simplifies shutdown and ordering.
- Chose a `resolvedNH` map to track last-emitted resolved NH over modifying protocolRoute: the resolution is derived state that can change without the route changing. A separate map keeps the concern distinct.
- Chose `ecmpCollectResolved` (cascade-aware ECMP) over reusing `ecmpCollect`: cascade must filter unreachable ECMP members and resolve remaining members' NHs, which `ecmpCollect` (raw NHs) cannot do.

## Consequences

- All Loc-RIB change processing in the locrib path is now asynchronous. Tests that relied on synchronous dispatch from Insert/Remove must use waitFor. The existing TestSysRIBConsumesLocRIB already did this.
- The locrib LPM method had a pre-existing data race (PathGroup.best() called after releasing shard read lock, sharing the backing array). Fixed by moving best() inside the lock. This fix is load-bearing for the async worker.
- ECMP cascade via the locrib path requires routes injected through processEvent directly (multiple protocols), because locrib only reports the aggregate best per prefix, not per-source bests.

## Gotchas

- The deadlock was latent in the existing code: OnChange -> processEvent -> recomputeBest -> resolveNextHop -> LPM -> same-shard RLock. Existing tests did not hit it because prefix hashing distributed them to different shards. Any test with GOMAXPROCS=1 would deadlock.
- LPM returning a PathGroup copy that shares the Paths slice backing array with the stored value is a subtle race. The copy looks safe but the slice header shares memory with the original.
- locrib OnChange fires only for aggregate best changes. Two paths with identical AdminDistance where the first-registered one stays best produce no event for the second path. ECMP at the sysrib level only works when routes arrive from different protocol names with distinct locrib best changes.

## Files

- `internal/component/sysrib/sysrib.go` -- Track/Untrack, cascade worker, async OnChange, resolvedNH, cascadeRecompute, ecmpCollectResolved, processLocRIBChange
- `internal/component/sysrib/nhresolver.go` -- CoveredNHs, familyForPrefix
- `internal/component/sysrib/sysrib_test.go` -- TestNHCascadeWithdraw, TestNHCascadeCostChange, TestECMPMemberFail, TestNHCascadeRestore
- `internal/component/sysrib/nhresolver_test.go` -- TestNHResolver_CoveredNHs
- `internal/core/rib/locrib/manager.go` -- LPM race fix
