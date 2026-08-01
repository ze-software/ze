# 923 -- isis-8-dis-broadcast

## Context
Spec `isis-8-dis-broadcast` adds broadcast-LAN behaviour to the native IS-IS engine
(ISO/IEC 10589 clause 8.4.5): per-level DIS election, pseudo-node LSP origination by
the elected DIS, the own-LSP star encoding, DIS-loss re-election, and the LAN CSNP
cadence. It layers on isis-5 (circuit/adjacency), isis-6 (LSDB/origination), and
isis-7 (flooding/CSNP), which already existed (sibling agents) -- this was an
integration, not a from-scratch build, despite the task brief saying "from scratch".

## Decisions
- Election lives PURE in `circuit/dis.go` (`DISState.Elect(candidates, damp, now)`):
  compare `(priority desc, SNPA desc)`; per-level `DISState` so L1/L2 are independent.
  The circuit gathers candidates (local + each Up LAN neighbor) and the engine reacts
  to the role-transition flags. No live-table reads inside the election.
- Pseudo-node LSP reuses the isis-6 `Originator` verbatim: a new
  `OriginatePseudonode(level, info)` differs from `Originate` only by a non-zero
  pseudonode Source ID and a metric-0 member TLV-22 stream; same fragmenter, same
  `lastSeq`/`suspendUntil` wraparound state, same `LSDB.Insert`. `PurgePseudonode`
  mirrors the stale-fragment purge. No second store, no side channel.
- Engine cross-package glue lives in a dedicated root-package file `dis_wiring.go`
  (matching the existing `lsdb_wiring.go`/`flooding_wiring.go` split), NOT threaded
  through circuit/lsdb. Election runs on the adjacency transition hooks AND a 1s
  re-election tick (so a DIS lost via the hold-timer sweep -- which fires no Hello --
  is still re-elected promptly).
- Pseudonode ID is derived deterministically per (circuit,level) -- `cid*2+level`
  folded to a non-zero octet -- so a re-election reuses the same ID (no churn of
  distinct pseudo-node LSP IDs) and L1/L2 get distinct LAN IDs.
- Own-LSP star encoding is a small change in `levelState`: a broadcast circuit with a
  recorded pseudo-node and ≥1 Up neighbor at the level emits ONE TLV 22 entry at the
  pseudo-node (and `continue`, skipping per-peer) instead of per-peer entries.

## Consequences
- A broadcast LAN of N nodes appears as one pseudo-node with N spokes, not an N*(N-1)
  mesh, in every node's LSDB and SPF graph.
- Per-neighbor DIS priority required threading a new `Priority` field through
  `Adjacency` + `HelloInput` + `ReceiveHello` + the LAN-IIH receive path (the field
  existed on the wire `LANHello` but was being dropped before the adjacency record).
- Owned metrics: `ze_isis_dis_elections_total{level}`, `ze_isis_pseudonode_lsps{level}`.

## Gotchas
- **isis-5 `reconcile` does NOT re-advertise a runtime priority change to a LIVE
  circuit** (it marks the param changed but the `circuit.Circuit` keeps its build-time
  priority). So an engine-level priority-driven DIS role transfer (spec AC-5) is not
  testable through reconcile in v1. The election LOGIC handles a priority change
  (unit test); the engine-level runtime triggers that work are DIS LOSS and a
  higher-priority node JOINING. Documented with `// test-relax:` in dis_wiring_test.go.
- **An abruptly-departed DIS cannot purge its pseudo-node** -- it ages out over
  MaxLifetime (1200s) via the isis-6 aging path. A DIS-loss test must assert the NEW
  DIS comes up, NOT that the old pseudo-node is already gone. The purge-before-yielding
  path (R-2) only applies to a node that loses the role while still PRESENT (the
  `LostRole` flag); test it with a higher-priority JOIN, not a node shutdown.
- The shared `multiBackend`/`relWire` test harness (lossless in-memory L2 segment, in
  `flooding_wiring_test.go`) supports N engines on one wire -- ideal for a 3-node LAN
  DIS test on darwin (no raw socket). Reuse it; do not build a new harness.
- LAN DIS interop with FRR (`isis-lan-dis-frr`) is owned by isis-13, not this spec.
  Per-topology DIS (RFC 5120 MT) is out of scope.
- Learning a FOREIGN DIS's exact pseudonode octet from its LAN IIH `LANID` is a v1 gap
  (isis-5 does not surface the neighbor's advertised LANID through the adjacency
  table). A non-DIS Ze node uses a deterministic placeholder pseudonode ID for the
  star edge; this only affects cross-vendor pseudonode-ID matching for a non-Ze DIS,
  not Ze-to-Ze convergence (the DIS originates the authoritative pseudo-node LSP).
  Follow-up: isis-13.

## Files
- `internal/plugins/isis/circuit/dis.go` (+test): election state machine, damping,
  circuit API (RunElection/LocalIsDIS/DISLevels/MembersSnapshot/candidates).
- `internal/plugins/isis/lsdb/pseudonode.go` (+test): OriginatePseudonode/PurgePseudonode.
- `internal/plugins/isis/dis_wiring.go` (+test): engine election trigger, pseudonode
  originate/purge, elected-pseudonode record, LAN CSNP cadence, re-election tick.
- Modified (additive): `adjacency/adjacency.go` (+Priority), `adjacency/fsm.go`
  (+HelloInput.Priority, store in ReceiveHello), `circuit/runtime.go` (extract LAN
  priority), `circuit/circuit.go` (per-level DISState), `circuits.go` (election on
  transition + clearCircuitDIS on down), `server.go` (DIS state, metrics, startDISLoop),
  `lsdb_wiring.go` (own-LSP star encoding).
- `internal/plugins/isis/packet/pseudonode_ci_test.go`: pins `test/isis/isis-dis.ci`.
- `test/isis/isis-dis.ci`: pseudo-node LSP wire decode.
- Docs: `docs/architecture/wire/isis.md` (DIS/pseudo-node section),
  `docs/plugin-development/metrics.md` (2 metric rows), `docs/guide/isis.md` (created).
