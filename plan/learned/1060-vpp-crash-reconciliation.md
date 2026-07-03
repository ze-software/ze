# 1060 -- VPP Crash Reconciliation

## Context

After a VPP crash and restart, `ifacevpp` retained stale in-memory state (dead GoVPP channel, stale name map and bridge domain IDs, fired `sync.Once` preventing repopulation) from the pre-crash VPP instance. The iface component already subscribed to `EventReconnected` and ran config reconciliation via `reconcileOnVPPReady`, but this handler operated against the stale backend, so reconciliation silently failed. Same bug class as VyOS T8979 (defunct interface retaining IPs after crash).

## Decisions

- Chose reloading the backend via `LoadBackend(vppBackendName)` in `reconcileOnVPPReady` over adding a `Resettable` interface with a `ResetForReconnect()` method, because LoadBackend is one line, creates a guaranteed-clean instance, and follows the fib/vpp pattern of full reinitialization on reconnect.
- Kept LoadBackend failure non-fatal (log + continue) over aborting reconciliation, so tests that don't register a "vpp" factory and production edge cases degrade to the previous behavior rather than breaking.

## Consequences

- VPP crash recovery now fully reinitializes the iface backend: fresh channel, fresh name map, fresh bridge domains, fresh `sync.Once`, fresh monitor.
- Any future VPP-dependent iface backend state added to `vppBackendImpl` is automatically reset by the LoadBackend path (factory creates a new instance).
- Concurrent `GetBackend()` callers may briefly see the old (dead) backend during the reload window; this is harmless because the old channel was already dead.

## Gotchas

- Existing reconcile tests register fake backends under test-specific names, not "vpp". The new `LoadBackend("vpp")` call fails silently in those tests (no "vpp" factory registered) and falls through to the existing backend. New tests must explicitly register under `vppBackendName` and clean up after themselves.

## Files

- `internal/component/iface/config_apply.go` -- added LoadBackend call in reconcileOnVPPReady
- `internal/component/iface/config_test.go` -- added TestReconcileOnVPPReady_ReloadsBackend, TestReconcileOnVPPReady_ReconcilesToNewBackend
