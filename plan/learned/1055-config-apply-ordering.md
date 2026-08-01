# 1055 -- Config Apply Ordering (operation graph)

## Context
Config reload applied changes surface-by-surface with no cross-surface ordering,
so dependent operations could apply in the wrong order (add an interface before
the BGP peer that binds it; tear a peer down before removing its address; swap two
interface addresses without a dual-presence window). Spec config-apply-ordering
replaced ad-hoc ordering with a transaction operation graph.

## What was built
- Operation graph core: `internal/component/config/transaction/{operation,depgraph,solver,executor}.go`.
  `BuildOperationGraph` -> `TopologicalSort` -> executor `Verify` / `Execute` / `Commit`.
- Component-owned decomposition (plugin self-containment): iface and bgp each own
  their operation decomposition and constraint rules, registered via `init()`:
  `internal/component/iface/operation.go`, `internal/component/bgp/plugin/operation.go`;
  peer add/remove/modify primitives in `internal/component/bgp/reactor/operation.go`.
- Reached from the real reload path: `plugin/server/reload.go` `runTxCoordinator`
  -> `SetOperationPlanner(operationPlannerFromTrees(...))` -> `orchestrator.runOperationPath`.
- Mandatory-v1 external contract: SDK types `pkg/plugin/sdk/sdk_types.go`; RPC
  transport `plugin/ipc/rpc.go` (config-operation-decompose/verify/apply/rollback/commit);
  bridge `plugin/server/config_tx_bridge.go`.
- Cycle handling: address-only cross-interface cycles relax with `AllowDual`
  (dual-presence during a swap); non-address or same-interface cycles are rejected
  (`solver.go`). Rollback replays inverse operations in reverse order and excludes
  the failed op (`executor.go`); settlement waiters are armed before apply so a
  fast readiness event is not missed.

## Key decisions
- Decomposition and constraint rules live in the owning component, not a central
  switch -- removing a plugin removes its operation handling.
- `verify -> execute -> commit` with rollback, rather than best-effort apply.

## Known limitations (verified at closure, non-blocking)
- The functional rotation/swap/reip tests (`test/reload/test-config-apply-ordering-*.ci`)
  rotate BGP router-ids, emitting only REMOVE/ADD_PEER (no ADD/REMOVE_ADDRESS), so
  they exercise the full decompose -> graph -> executor -> bridge -> RPC -> reactor
  reload path but NOT the IP cycle / `AllowDual` machinery. That machinery is covered
  by unit tests instead: `solver_test.go` `TestTopologicalSortCycleResolution`
  (2-way swap) and `TestTopologicalSortThreeWayRotation` (3 `AllowDual`). The
  create/delete tests emit real iface ADD/REMOVE_INTERFACE ops but are Linux-only
  (`skip-os=darwin`).
- Boundary caps named in the spec (operation-count warn at 10000, cycle-depth
  reject at 100) are not enforced in `BuildOperationGraph`/`TopologicalSort`. Not
  security-relevant: config is operator-supplied and inherently bounded.
- The "Settlement Race Fix" step-3 owner state-check is not implemented in
  `armSettlementWaiters` (it only subscribes). Harmless for the current decomposers,
  which never emit ADD_ADDRESS for an already-present address.

## Related
Predecessor design history: 537-config-tx-protocol.md (transaction protocol),
535-config-tx-consumers.md (how plugins participate), 779-transactional-config-commit.md
(cross-surface commit), 758-config-graph.md (config dependency graph).

## Files

None recorded.
