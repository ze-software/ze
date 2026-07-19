# 1182 -- Firewall reconcile serialization (registry `reconcileMu`)

## Context

`plan/spec-fixit-firewall-concurrency-deadlock.md`. A QEMU run (2026-07-12) saw
`ze-plugin-engine:dispatch-command` stop responding ~255s with `firewall { backend nft }`
configured while ddos-local mitigated under a flood. The spec's research established the
title is a MISNOMER: **there is no lock cycle / deadlock.** The lock order is strictly
`r.mu`(plugin) -> `tableRegistry.mu` -> `backendsMu` -> backend-internal on every path,
with no reverse edge. What exists instead are real, separately-shippable defects, of which
this change fixes the one that lives in the shared firewall layer.

`firewall.ApplyAll` (`internal/component/firewall/registry.go`) deliberately dropped BOTH
registry locks before calling `b.Apply(all)` (comment: "don't hold a registry lock across
a kernel call"). Correct premise, wrong conclusion: it held NO lock, so two owners (the
firewall engine + a plugin like ddos-local) could be inside `Backend.Apply` at once. The
nft backend (`internal/plugins/firewall/nft/backend_linux.go`) stages every command into a
single shared `*nftables.Conn` batch and holds an un-synchronized `applied` map; concurrent
`Apply` therefore causes (a) lost updates -- the loser's `Flush` finds the batch already
drained and returns nil having sent nothing -- and (b) a genuine `applied`-map data race.
Because the `ze_*` sweep deletes any owned table in `applied`, an interleaved apply can
silently DELETE the other owner's live drop rule while the registry believes it is installed.

## Scope of THIS change

This agent implemented ONLY the `internal/component/firewall/*` slice (disjoint from sibling
agents landing the plugin-side fixes concurrently on `main`):
- **D-1** registry serialization (`registry.go`).
- **D-5, in-scope portion**: the single-writer + must-not-block-indefinitely contract on the
  `Backend.Apply` interface doc, and the lock-order note on `backendsMu` (`backend.go`).

Handled by sibling agents, NOT here: D-2 nft netlink deadline (`plugins/firewall/nft`),
D-3 ddos-local atomic status snapshot (`plugins/ddos/local`), D-4 anomaly-shape
(`plugins/anomaly/shape`), the `core-design.md` note, and the QEMU `.ci` reproduction.

## Decisions

- **Serialize the WHOLE `ApplyAll` body under a new package-level `reconcileMu`, not just
  `b.Apply`.** The desired-state snapshot is taken under `tableRegistry.mu` and released
  before `b.Apply`. Locking only around `b.Apply` would still let two callers apply STALE
  snapshots out of order and converge the kernel to a superseded state. Snapshot and apply
  must be atomic together, so `reconcileMu` spans the entire function (acquire at top,
  `defer` unlock).
- **`reconcileMu` is the OUTERMOST firewall lock**: order is
  `reconcileMu -> tableRegistry.mu -> backendsMu -> backend-internal`. Verified (spec A-8)
  that no `ApplyAll` call site holds `tableRegistry.mu` or `backendsMu` on entry --
  `FlushAllTables` unlocks `tableRegistry.mu` before calling `ApplyAll`; `engine.go` releases
  `backendsMu` after `LoadBackend` before `ApplyAll` -- so making it outermost inverts no
  existing order and cannot self-deadlock. `ApplyAll` never calls itself, so non-reentrancy
  is safe.
- **The fix belongs in the registry, not any plugin.** `registry.go` is the single
  production caller of `Backend.Apply` (spec Finding 5; every other `firewall.GetBackend()`
  consumer uses read-only paths with their own transient netlink conn). Serializing inside
  `ApplyAll` therefore protects ALL callers (firewall engine, copp, policyroute,
  flowspec-firewall, anomaly-shape, ddos-local, irr) with one change -- satisfies AC-4,
  AC-5, AC-8 at the owning layer. Putting a mutex in each backend would fix the race but not
  the stale-snapshot lost-update, which lives in the registry.
- **State the concurrency contract in the interface doc, not only in prose.** `Backend.Apply`
  now documents the single-writer guarantee (implementers may keep un-synchronized state)
  AND the must-not-block-indefinitely obligation (a wedged kernel would hold the outermost
  `reconcileMu` forever and stall every firewall owner plus any command handler needing a
  lock a plugin holds across `ApplyAll`). The `EngineEventHandler` "MUST NOT block on
  external I/O" contract already existed and was violated by two of two plugins that had the
  opportunity -- a documented contract with no mechanical gate; candidate for
  `ai/rules/friction-reporting.md`.

## Consequences

- Tests (`internal/component/firewall/registry_concurrency_test.go`), all `-race` clean:
  - `TestApplyAllSerialisesBackendApply` (AC-4): 8 goroutines call the real `ApplyAll`; an
    overlap-counting fake backend asserts max concurrent `Apply` == 1. Deterministically RED
    without the lock (observed max = 8).
  - `TestApplyAllConcurrentOwnersConverge` (AC-4): N goroutines each register a distinct
    owner then `ApplyAll`; the last `Apply` to complete carries every owner's tables.
  - `TestApplyAllStaleSnapshotNotApplied` (D-1 rationale): owner A applies (blocked
    mid-flight via an `atomic.Bool` CAS gate so only the first `Apply` blocks), B registers
    during the window, a second `ApplyAll` must wait on `reconcileMu`; the kernel ends on
    {A,B}. Deterministically RED without the lock (ended on [ze_a] only).
  - The gate uses `atomic.Bool.CompareAndSwap`, NOT `sync.Once`: `Once.Do` blocks concurrent
    callers and would hide the missing lock, making the test pass for the wrong reason. This
    was caught during the RED-check and is the reason the test discriminates.
- The three existing registry contract tests (`TestApplyAllAutoLoadsDefaultBackend`,
  `TestApplyAllNoBackendNoTablesIsNoOp`, `TestApplyAllNoDefaultKeepsNotLoadedError`) and
  `TestFlushAllTablesClearsRegistryAndReconciles` stay green: `reconcileMu` adds ordering,
  not new failure modes.
- **Do NOT claim "the deadlock is fixed" anywhere.** It is not a deadlock, and the link from
  these code-read findings to the 2026-07-12 observation remains a hypothesis (spec A-6/R-9):
  no goroutine dump was captured. This change fixes the concurrent-apply corruption on its
  own merits (which Scope admits as IN); the observed stall's keystone fact (which command
  was dispatched) stays open for the plugin-side agents' `.ci` reproduction.
