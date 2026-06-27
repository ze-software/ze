# Spec: VPP Interface Rate Stats — Dataplane-Agnostic iface Signal (detector prerequisite)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-cp-survival-5-detect-0-umbrella |
| Phase | 1/5 |
| Updated | 2026-06-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/spec-cp-survival-5-detect-1-detector.md` (the consumer that needs a dataplane-agnostic rate signal)
3. `.claude/rules/planning.md`, `ai/rules/qemu-testing.md` (VPP changes need QEMU integration tests)
4. `internal/plugins/iface/vpp/query.go` (the gap), `internal/component/vpp/telemetry.go` (the stats source), `internal/plugins/iface/netlink/show_linux.go` (the netlink analogue to mirror)

## Task

Make Ze's per-interface rate signal (`iface.RegisterCollectNotify` / `iface.GetRate` / `ze_interface_*`
gauges) work on a **VPP dataplane**, not only netlink. Today the VPP iface backend leaves
`InterfaceInfo.Stats = nil`, so `rate.go` skips every VPP interface and no rate is produced — which makes
the DDoS detector (child 1) blind on VPP boxes (umbrella R-8, detector A-7).

The fix is **at the source** (per `ai/rules/no-workarounds-for-missing-behavior.md` and the VPP-parity
rule): populate `InterfaceInfo.Stats` for VPP interfaces from VPP's stats segment, which Ze already
reads. The detector then needs zero VPP-specific code, and the whole rate-tracking subsystem
(`show interface rate`, the 12 `ze_interface_*` gauges) gains VPP parity as a side effect.

This is a prerequisite for detection on VPP; it touches only the iface VPP backend and its access to the
VPP stats provider. It is NOT DDoS-specific — it is a general iface-component completeness fix.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/qemu-testing.md` - VPP is Linux/dataplane code
  → Constraint: the VPP rate path MUST have a QEMU integration test; "needs hardware" is not an excuse.
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - fix at source
  → Constraint: populate `InterfaceInfo.Stats` in the iface VPP backend; do not special-case VPP in `rate.go` or in the detector.
- [ ] `internal/plugins/iface/netlink/show_linux.go` - the netlink analogue
  → Constraint: mirror how netlink fills `info.Stats` from `link.Attrs().Statistics`; the VPP backend fills the same struct from VPP counters.

### RFC Summaries
- N/A — no wire protocol.

**Key insights:**
- The data and machinery already exist: `internal/component/vpp/telemetry.go` (`statsProvider.GetInterfaceStats(*api.InterfaceStats)`, `statsPoller`) reads per-interface rx/tx packets/bytes/drops/errors from the VPP stats segment and computes deltas — but only to emit `ze_vpp_interface_*` Prometheus counters. The fix routes the same counters into `iface.InterfaceInfo.Stats`.
- `rate.go` needs NO change: once `Stats` is non-nil for VPP interfaces, `rateDelta` (rate.go:181) computes RxPps/RxBps identically to netlink. The `if info.Stats == nil { continue }` skip (rate.go:140) simply stops skipping them.
- Mapping is by sw_if_index → interface name (the VPP backend already lists interfaces with both).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/rate.go:140` - `if info.Stats == nil { continue }` skips interfaces without stats; `rateDelta` (rate.go:181) computes Rx/Tx Pps/Bps from cumulative counters.
  → Constraint: this is the shared, backend-agnostic rate engine; the fix is to make Stats non-nil for VPP, not to branch here.
- [ ] `internal/component/iface/iface.go:76` - `type InterfaceStats` (RxBytes/RxPackets/TxBytes/TxPackets/Rx*Errors/…); `iface.go:103` - `type InterfaceInfo` (Stats field).
  → Constraint: the VPP backend must fill this exact struct.
- [ ] `internal/plugins/iface/vpp/query.go:178-197` - `detailsToInfo()` builds `InterfaceInfo` from VPP interface details but leaves `Stats` nil; `query.go:79-89` `ListInterfaces()`.
  → Constraint: this is the change site — populate `Stats` here (or via `GetStats`).
- [ ] `internal/plugins/iface/vpp/ifacevpp.go:575-577` - `GetStats()` returns "not supported (pending GoVPP stats API wiring)".
  → Constraint: wire this to the VPP stats segment (or fold the stats read into the listing path).
- [ ] `internal/component/vpp/telemetry.go:38-219` - `statsProvider.GetInterfaceStats(*api.InterfaceStats)`, `statsPoller` reads per-interface counters from the stats segment and computes deltas (`ifaceSnapshot`).
  → Constraint: reuse this stats provider/connection; do NOT open a second stats-segment connection.
- [ ] `internal/component/vpp/stats_conn.go` - connects to the VPP stats segment.
  → Constraint: the iface VPP backend obtains the stats provider from here / the VPP component, not a new connection.

**Behavior to preserve:**
- Netlink backend rate behavior is unchanged (no regression on Linux dataplane).
- `ze_vpp_interface_*` counters keep being published (this fix is additive to the rate path).
- `rate.go` semantics, `rateDelta`, and the `ze_interface_*` gauge names are unchanged.

**Behavior to change:** The VPP iface backend now populates `InterfaceInfo.Stats`; `GetStats()` stops
returning "not supported".

## Data Flow (MANDATORY)

### Entry Point
- `iface` collection tick (~1 Hz) calls the active backend's interface listing (`ListInterfaces`/`GetStats`).

### Transformation Path
1. On a VPP dataplane, the iface VPP backend lists interfaces (sw_if_index ↔ name) via GoVPP.
2. It reads per-interface cumulative counters from the VPP stats segment (`statsProvider.GetInterfaceStats`).
3. `detailsToInfo` fills `InterfaceInfo.Stats` (RxBytes/RxPackets/TxBytes/TxPackets/errors) keyed by name.
4. `rate.go` no longer skips these interfaces; `rateDelta` computes RxPps/RxBps and publishes `ze_interface_*` gauges, exactly as for netlink.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface VPP backend ↔ VPP stats segment | `statsProvider.GetInterfaceStats` via the VPP component | [ ] |
| iface backend ↔ rate engine | `InterfaceInfo.Stats` populated → `rate.go` | [ ] |
| rate engine ↔ telemetry | existing `ze_interface_*` gauge publication | [ ] |

### Integration Points
- `internal/plugins/iface/vpp/query.go` `detailsToInfo` / `ifacevpp.go` `GetStats` - change site
- `internal/component/vpp` stats provider (`statsProvider.GetInterfaceStats`, `stats_conn.go`) - stats source
- `internal/component/iface/rate.go` - unchanged consumer (just stops skipping)

### Architectural Verification
- [ ] No bypassed layers (stats via the existing VPP stats provider, not a new connection)
- [ ] No unintended coupling (rate.go and the detector stay dataplane-agnostic; no VPP branch leaks upward)
- [ ] No duplicated functionality (reuses the existing stats poller/connection)
- [ ] Zero-copy preserved (counter reads only; no wire encoding)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The VPP stats provider exposes per-interface rx/tx packet+byte counters keyed by sw_if_index | LSP: `statsProvider.GetInterfaceStats(*api.InterfaceStats)`, `ifaceSnapshot` fields (telemetry.go) | cannot fill Stats; need another VPP API | trace `api.InterfaceStats` fields + a unit test with a fake provider | unvalidated |
| A-2 | The iface VPP backend can reach the VPP stats provider without a second stats-segment connection | `internal/component/vpp/stats_conn.go` already connects | double-connect or contention | wire through the VPP component; integration test | unvalidated |
| A-3 | sw_if_index ↔ interface name mapping is available in the VPP backend listing path | `query.go ListInterfaces` lists both | counters cannot be keyed by name | confirm in `query.go`; unit test the mapping | unvalidated |
| A-4 | `rate.go` needs no change once Stats is non-nil | rate.go:140 skip + rate.go:181 rateDelta are backend-agnostic | rate.go branch required | functional test: VPP iface shows non-zero rate with rate.go unchanged | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | VPP stats counters are cumulative and may reset on interface flap, skewing the delta | negative or huge rate spike | clamp negative deltas to 0 (same guard the netlink path needs); test flap |
| R-2 | Double-counting if both the existing `ze_vpp_interface_*` poller and the iface path read stats | metric mismatch | share one stats provider; the iface path adds the `ze_interface_*` gauges, the VPP poller keeps `ze_vpp_interface_*` (distinct names, same source) |
| R-3 | sw_if_index reuse after interface delete maps counters to the wrong name | rate attributed to wrong iface | key by name resolved per poll, not cached index |
| R-4 | Test requires a running VPP, easy to skip | "tested on netlink only" | QEMU integration test is mandatory (`ai/rules/qemu-testing.md`) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| iface collection tick on a VPP dataplane | → | VPP backend fills `InterfaceInfo.Stats` → `rate.go` produces RxPps | `TestVPPDetailsToInfoPopulatesStats` (unit) + QEMU integration |
| `show interface rate` on VPP | → | rate gauges populated for VPP interface | `show-interface-rate.ci` (extended) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Traffic on an active VPP interface | `iface.GetRate(name)` returns non-zero RxPps/RxBps for that interface |
| AC-2 | VPP backend lists an interface | `InterfaceInfo.Stats` is non-nil and carries the VPP counters (sw_if_index → name) |
| AC-3 | rate tick on VPP | `ze_interface_rx_packets_per_second` etc. are published for VPP interfaces (parity with netlink) |
| AC-4 | netlink dataplane | rate behavior is unchanged (no regression) |
| AC-5 | counters increment across two polls | the computed delta/rate is correct and never negative (flap guard) |
| AC-6 | `show interface rate` on a VPP box | shows non-zero rates for active VPP interfaces |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs Ze on a VPP dataplane and generates traffic | VPP stats → iface Stats → rate.go → `ze_interface_*` / `show interface rate` | QEMU integration + `show-interface-rate.ci` |
| 2 | enables the DDoS detector on a VPP box | detector reads the now-populated iface rate → triggers | child-1 functional path (cross-ref) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVPPDetailsToInfoPopulatesStats` | `internal/plugins/iface/vpp/query_test.go` | `detailsToInfo` fills Stats from a fake stats provider (AC-2) | |
| `TestVPPStatsIndexToNameMapping` | `internal/plugins/iface/vpp/query_test.go` | sw_if_index → name keying (A-3, R-3) | |
| `TestVPPRateDeltaNonNegative` | `internal/plugins/iface/vpp/query_test.go` | cumulative-counter delta clamps negatives (AC-5, R-1) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| counter delta (per poll) | 0-… | n/a | negative (clamp to 0) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-interface-rate` | `test/ui/show-interface-rate.ci` | `show interface rate` renders rates (netlink no-regression; VPP path covered by QEMU) | |

### Interop Tests
N/A — no wire protocol.

### QEMU / Linux-only Tests (MANDATORY — `ai/rules/qemu-testing.md`)
| Test | Validates |
|------|-----------|
| `TestVPPInterfaceRateQEMU` | on a real VPP dataplane in QEMU, traffic produces non-zero `iface.GetRate` and `ze_interface_*` gauges (AC-1, AC-3, AC-6) |

### Future (deferred tests)
- None deferred.

## Files to Modify
- `internal/plugins/iface/vpp/query.go` - `detailsToInfo` populates `InterfaceInfo.Stats` from VPP stats
- `internal/plugins/iface/vpp/ifacevpp.go` - wire `GetStats()` to the VPP stats provider (remove the "not supported" stub)
- `internal/component/vpp/stats_conn.go` / `telemetry.go` - expose the stats provider to the iface backend if not already reachable (no second connection)
- `docs/features.md` - update "per-interface rate tracking" to note VPP parity
- `docs/guide/monitoring.md` - note `ze_interface_*` now covers VPP interfaces

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| QEMU integration test | Yes | VPP rate path (`ai/rules/qemu-testing.md`) |
| Functional test | Yes | `test/ui/show-interface-rate.ci` |
| Prometheus counters | No new names | reuses existing `ze_interface_*` gauges, now populated for VPP |
| Doctor check | No | no new runtime dependency (VPP already required for the VPP backend) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes (VPP rate parity) | `docs/features.md` |
| 11 | Affects daemon comparison? | Maybe | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes (iface backend stats) | `docs/architecture` iface/VPP doc |
| 14 | Prometheus counters added/changed? | Yes (now populated on VPP) | `docs/plugin-development/metrics.md` |
| 16 | Changed source referenced by doc anchors? | Yes | grep `docs/` for `iface/vpp` / `rate.go` anchors |

## Files to Create
- `internal/plugins/iface/vpp/query_test.go` - unit tests above (if not present)
- the QEMU integration test for the VPP rate path

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |

### Implementation Phases
1. **Phase: Wiring (FIRST)** — a failing `TestVPPDetailsToInfoPopulatesStats` asserting `Stats != nil`; backend still returns nil.
   - Verify: test fails for the right reason.
2. **Phase: Stats wiring** — give the iface VPP backend access to the VPP stats provider; read per-interface counters; map sw_if_index → name.
   - Tests: `TestVPPStatsIndexToNameMapping`.
3. **Phase: Populate Stats + delta guard** — fill `InterfaceInfo.Stats` in `detailsToInfo`/`GetStats`; clamp negative deltas.
   - Tests: `TestVPPDetailsToInfoPopulatesStats`, `TestVPPRateDeltaNonNegative`; `show-interface-rate.ci` (netlink no-regression).
4. **Phase: QEMU integration** — `TestVPPInterfaceRateQEMU` on a real VPP dataplane.
5. **Full verification** → `make ze-verify-changed` + `make ze-qemu-integration-test`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | each AC-N has implementation with file:line |
| Correctness | delta never negative; counters keyed by name, not stale index |
| Data flow | no VPP branch in `rate.go` or the detector; fix confined to the iface VPP backend |
| No duplicate connection | reuses the existing VPP stats provider |
| Rule: VPP parity | netlink unchanged; VPP now reaches parity |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Stats populated on VPP | `TestVPPDetailsToInfoPopulatesStats` passes |
| rate works on VPP | `TestVPPInterfaceRateQEMU` passes |
| netlink no-regression | existing iface rate tests + `show-interface-rate.ci` pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | per-interface map bounded by interface count; no unbounded growth on flap |
| Input validation | counter deltas clamped; index→name resolved safely |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- tap/memif/af_packet VPP interfaces that ALSO appear to the kernel are not double-counted because the
  active backend is VPP (single source of truth per dataplane).

## Design Insights
- The rate engine was already backend-agnostic; the only gap was the VPP backend not filling `Stats`.
  Fixing it at the backend gives every rate consumer (telemetry, CLI, the DDoS detector) VPP parity at once.

## Implementation Audit

| AC | Evidence |
|----|----------|
| AC-1 | `GetStats` wired to VPP stats provider (`ifacevpp.go:575`); `TestGetStatsReturnsVPPCounters` passes |
| AC-2 | `ListInterfaces` populates `InterfaceInfo.Stats` from `readVPPIfaceStats` (`query.go:88-91`); `TestVPPDetailsToInfoPopulatesStats` passes |
| AC-3 | `rate.go` unchanged; `rateDelta` (`rate.go:181`) computes Rx/Tx Pps/Bps and publishes `ze_interface_*` gauges once Stats is non-nil |
| AC-4 | netlink backend untouched; all existing `iface` tests pass |
| AC-5 | `rateDelta` (`rate.go:181-185`) clamps `cur < prev` to 0; `readVPPIfaceStats` maps cumulative counters |
| AC-6 | `show interface rate` reads from `rateTracker.snapshot()` which now has VPP data |

### Files changed
- `internal/component/vpp/vpp.go` -- `IfaceStatsReader` interface, `GetActiveStatsProvider`/`setActiveStatsProvider`, wired in `runOnce`
- `internal/plugins/iface/vpp/ifacevpp.go` -- `getActiveStatsProvider` var, `GetStats` wired
- `internal/plugins/iface/vpp/query.go` -- `readVPPIfaceStats` helper, `ListInterfaces`/`GetInterface` populate Stats
- `internal/plugins/iface/vpp/query_test.go` -- 6 new tests (stats populated, name mapping, nil provider, error, GetStats, unknown)

## Review Gate
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/iface/vpp`)
- [ ] QEMU integration test passes
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-5-detect-1a-vpp-iface-rate.md`
