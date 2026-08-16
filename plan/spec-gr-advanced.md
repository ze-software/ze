# Spec: gr-advanced

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc4724.md`, `rfc/short/rfc9494.md` - GR/LLGR summaries already in the repo
4. `internal/component/bgp/plugins/gr/gr.go`, `gr_state.go` - current GR implementation
5. `internal/component/bgp/message/notification.go` - NOTIFICATION codes

## Task

Destination spec for the **GR advanced** work item deferred out of
`plan/spec-followup-bgp-feature.md` (umbrella work item "GR advanced (L81,L86)").
The umbrella bundled three sub-features under one line; research (2026-07-08) split
them by RFC and by subsystem. This spec captures the two that are genuinely
Graceful-Restart features. The third (VPN ATTR_SET, RFC 6368) is **not** a GR
feature and is deferred to a future L3VPN spec (see Known Limitations / Deferred).

ze today implements Graceful Restart **only as a Helper (Receiving Speaker)**: when a
GR-capable peer restarts, ze retains that peer's routes as stale and purges them on
End-of-RIB or timer expiry. The two in-scope features extend that:

- **Hard Reset (RFC 8538)** - Notification Message Support for BGP Graceful Restart.
  Two coupled pieces:
  1. **N-bit-gated retention on NOTIFICATION.** Today ze flushes on *any*
     NOTIFICATION-triggered down (pre-8538 / RFC 4724 behaviour). RFC 8538 says when
     both peers have negotiated the N-bit, a NOTIFICATION-triggered reset (other than a
     Hard Reset) drives the GR helper procedures (retain routes) instead of an
     immediate flush.
  2. **Hard Reset message (Cease, subcode 9).** Build and send a Cease/Hard-Reset that
     wraps the triggering NOTIFICATION's original [code][subcode][data]; on receipt,
     unwrap it, log the original, and **always** flush (never retain), regardless of
     N-bit.

- **Selection Deferral Timer (RFC 4724 Section 4.1)** - the Restarting-Speaker side.
  When ze itself restarts, defer running best-path selection and re-advertising routes
  to peers until either EOR has been received from every GR-negotiated peer or a
  configurable Selection Deferral Timer expires. ze has no restarting-speaker deferral
  today; GR is helper-only.

### Deferred out of this spec (see Known Limitations / Deferred)

- **VPN ATTR_SET (RFC 6368), attribute type 128** - an L3VPN PE-CE feature mis-bundled
  under "GR advanced (L86)" in the 2026-07-06 deferral triage. Not graceful restart.
- **AS-Confederation OTC (RFC 9234 Section 5, umbrella item 3, L88)** - already
  re-deferred separately in the umbrella; cross-referenced here only.

## Required Reading

### Architecture Docs
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `docs/features/rfc-status.md` - current GR/LLGR RFC support ledger
  → Constraint: any RFC behaviour newly implemented updates this ledger with source anchors.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4724.md` - Graceful Restart Mechanism for BGP (base GR + Section 4.1 deferral timer)
  → Constraint: Restarting Speaker MUST defer route selection until EOR-from-all-peers or the deferral timer fires (Section 4.1).
- [ ] `rfc/short/rfc9494.md` - Long-Lived Graceful Restart (already shipped; interacts with retention gating)
  → Constraint: N-bit / retention decisions must not regress the existing LLGR path.
- [ ] RFC 8538 - Notification Message Support for BGP Graceful Restart (N-bit, Cease subcode 9 Hard Reset)
  → Constraint: short summary is NOT yet in `rfc/short/`; generate it with the `ze-rfc` skill (`/ze-rfc rfc8538`) BEFORE this spec moves from `skeleton` to `ready`. Do not summarise from memory.
- [ ] RFC 6368 - Internal BGP as the PE-CE Protocol (ATTR_SET, attribute type 128) — DEFERRED
  → Constraint: out of scope here; its summary belongs with the future L3VPN spec, not this one.

**Key insights:**
- ze is a GR Helper only today; both in-scope features add Restarting-Speaker / negotiation behaviour that does not exist yet.
- Hard Reset is two coupled changes (retention gating + a new Cease subcode-9 wire message), not one.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-07-08 at the line numbers below)
- [ ] `internal/component/bgp/plugins/gr/gr.go` - `decodeGR` (:799) parses the *peer's* GR capability, extracting the R-bit (`Restarting`, :812) and N-bit (`Notification`, :813); the N-bit is stored (`grResult.Notification`, :791) but consumed **only cosmetically** in `formatGRText` (:841-842). `handleStructuredState` (:346) computes `wasNotification := reason == "notification"` (:358) and calls `onSessionDown` (:365); on activation it dispatches helper-side retention (`purge-stale`/`retain-routes`/`mark-stale`, :367-369). EOR handling (`handleEOREvent`, :567) purges stale routes for a family (RFC 4724 Section 4.2, receiving side).
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` - `onSessionDown` (:114) short-circuits `if cap == nil || wasNotification { clearPeerLocked; return false }` (:118): **any** NOTIFICATION-triggered down disables retention, ignoring the N-bit. `onEORReceived` (:241) purges stale per family.
- [ ] `internal/component/bgp/message/notification.go` - `NotifyCeaseHardReset uint8 = 9 // RFC 8538` (:127) and its `String()` case "Hard Reset" (:429) exist, but there is **no** Hard Reset build (wrap `[origCode][origSubcode][origData]`) or unwrap logic anywhere in `internal/component/bgp/`.
- [ ] `internal/core/bgp/attribute/attribute.go` - attribute codes stop at `AttrPrefixSID = 40` / `AttrLargeCommunity = 32` (plus internal `AttrTombstone = 252`); **attribute code 128 (ATTR_SET) is not defined**. L3VPN NLRI scaffolding exists (`family.SAFIVPN`; `internal/component/bgp/plugins/nlri/vpn/types.go`; `internal/component/bgp/plugins/rib/rib_nlri.go`) but the ATTR_SET path-attribute does not.

**Behavior to preserve:**
- Existing Helper-side GR/LLGR behaviour: stale-route retention on non-notification down, EOR-driven purge (RFC 4724 Section 4.2), LLGR long-lived retention (RFC 9494), and the restart-time=0+LLGR fast path in `onSessionDown`.
- The existing GR capability decode (`decodeGR`) wire layout and JSON field names (`restarting`, `notification`, `restart-flags`).
- Per-peer metric-label cleanup on peer removal (umbrella item 6, already shipped).

**Behavior to change:**
- `onSessionDown` retention decision: gate NOTIFICATION-triggered retention on negotiated N-bit instead of always flushing (RFC 8538).
- Add Hard Reset (Cease subcode 9) build/unwrap and wire it into the send/receive paths.
- Add a Restarting-Speaker Selection Deferral Timer (RFC 4724 Section 4.1) that holds best-path selection/advertisement.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A peer sends a NOTIFICATION (plain or Cease/subcode-9 Hard Reset) that tears the session down; delivered to GR as a structured `SessionStateDown` state event with a reason string (`internal/component/bgp/plugins/gr/gr.go`).
- ze itself restarts and re-establishes sessions; peers send UPDATEs then EOR markers, delivered to GR as `eor` events (`gr.go`).
- Operator/engine decides to hard-reset a peer; the NOTIFICATION send path emits Cease/subcode-9 instead of the raw code.

### Transformation Path
1. Session-down reason and the peer's negotiated GR capability (`peerCaps`) are read in `handleStructuredState` (gr.go).
2. `onSessionDown` (gr_state.go) decides retention. New: when `wasNotification` and both local+peer N-bits were negotiated (and it was not a Hard Reset), run helper retention instead of flushing.
3. A received Cease/subcode-9 is unwrapped in the message layer to recover the original [code][subcode][data]; GR treats it as a forced flush (never retain).
4. On ze's own restart, a Selection Deferral Timer in the reactor holds best-path selection until EOR-from-all-GR-peers or timer expiry, then selection runs once and routes are advertised.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| session/FSM → GR plugin | structured state + `eor` events over the plugin event bus | [ ] |
| GR plugin → RIB | `dispatchCommand` text commands (`retain-routes`, `purge-stale`, `mark-stale`) | [ ] |
| wire ↔ message | NOTIFICATION encode/decode incl. Cease subcode 9 wrapping | [ ] |
| reactor → RIB/advertisement | selection-deferral hold on the restarting speaker | [ ] |

### Integration Points
- `internal/component/bgp/plugins/gr/` - retention decision, N-bit negotiation state, EOR tracking
- `internal/component/bgp/message/notification.go` - Hard Reset build/unwrap
- `internal/core/bgp/capability/` - advertise ze's own N-bit in the GR capability
- `internal/component/bgp/reactor/` - Selection Deferral Timer on the restarting speaker
- `internal/component/bgp/fsm/` - hard-reset send path on teardown

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing GR, does not recreate)
- [ ] Registration over hardcoding - any new CLI/config surface registers and is core-discovered, not hardcoded into a core/shared package (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ze does not yet advertise its own N-bit in the GR capability it sends | `decodeGR` (gr.go) is decode-only; no GR encoder found in `internal/core/bgp/capability` during research | N-bit negotiation may be partly present; scope shrinks | grep/read the GR capability encoder at design time | unvalidated |
| A-2 | ze has no Restarting-Speaker restart tracking (only Helper) | grep for "deferral" in `gr/*.go` returns nothing; R-bit is parsed only from the peer's capability | Selection-deferral scope changes | LSP/grep at design time | unvalidated |
| A-3 | RFC 6368 ATTR_SET is absent and is an L3VPN (not GR) feature | attribute.go has no code 128; RFC 6368 is "iBGP as PE-CE" | If partly present, re-scope the deferral | grep at design time + RFC 6368 summary | unvalidated |
| A-4 | The Cease/subcode-9 constant is defined but unused for build/unwrap | notification.go/429 define the code + String only; grep found no wrap/unwrap | Less to build | grep at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | N-bit retention change regresses existing flush-on-NOTIFICATION expectations | existing GR/LLGR functional tests fail | gate strictly on negotiated N-bit; keep default behaviour when N-bit absent |
| R-2 | Selection Deferral Timer interacts badly with LLGR/EOR purge timing | stale routes linger or are advertised early | reuse existing EOR tracking; add explicit timer-expiry test |
| R-3 | Hard Reset unwrap on malformed/short data panics or mis-reports | fuzz/boundary tests on the NOTIFICATION parser | length-check before unwrap; boundary tests |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer with negotiated N-bit (both sides) sends a plain NOTIFICATION | → | `onSessionDown` runs helper retention instead of flushing | `gr.TestNotificationWithNegotiatedNBitRetainsRoutes` |
| Peer sends Cease/subcode-9 (Hard Reset) | → | message layer unwraps original [code][subcode][data]; GR flushes, never retains | `message.TestHardResetUnwrapYieldsOriginalNotification`, `gr.TestHardResetDownAlwaysFlushes` |
| Engine triggers a hard reset toward an N-bit peer | → | NOTIFICATION send path emits Cease/subcode-9 wrapping the original notification | `message.TestBuildHardResetWrapsNotification` |
| ze restarts; peers reconnect and EOR is still pending | → | reactor Selection Deferral Timer holds best-path selection until EOR-from-all or timer expiry | `reactor.TestSelectionDeferralHoldsUntilEOR`, `reactor.TestSelectionDeferralTimerExpiryProceeds` |
| Operator scrapes GR behaviour end to end | → | hard-reset + selection-deferral behave per RFC | `test/plugin/gr-hard-reset.ci`, `test/plugin/gr-selection-deferral.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Both peers negotiated the N-bit; peer tears the session with a plain (non-Hard-Reset) NOTIFICATION | ze runs the GR helper procedures (retains the peer's routes as stale) instead of the current immediate flush |
| AC-2 | ze receives a Cease with subcode 9 (Hard Reset) | ze unwraps and logs the original [code][subcode][data] and flushes the peer's routes immediately, regardless of N-bit negotiation |
| AC-3 | Engine/operator triggers a hard reset toward a peer that advertised the N-bit | ze sends a Cease/subcode-9 NOTIFICATION whose data wraps the triggering NOTIFICATION's [code][subcode][data]; a non-N-bit peer receives the ordinary NOTIFICATION |
| AC-4 | GR capability negotiation completes | ze advertises its own N-bit; the retention-on-NOTIFICATION behaviour (AC-1) applies only when both local and peer N-bits are set |
| AC-5 | ze restarts and re-establishes sessions with GR-negotiated peers | ze defers best-path selection and route advertisement until EOR is received from every GR-negotiated peer OR the Selection Deferral Timer expires, then selects once |
| AC-6 | Selection Deferral Timer configured (or default) and it expires before all EORs arrive | ze proceeds with best-path selection using whatever it has received; behaviour and default value are configurable via YANG |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildHardResetWrapsNotification` | `internal/component/bgp/message/notification_test.go` | Cease/subcode-9 build wraps original [code][subcode][data] | |
| `TestHardResetUnwrapYieldsOriginalNotification` | `internal/component/bgp/message/notification_test.go` | unwrap recovers the original notification; short/malformed data handled | |
| `TestNotificationWithNegotiatedNBitRetainsRoutes` | `internal/component/bgp/plugins/gr/gr_state_test.go` | N-bit-gated retention on plain NOTIFICATION | |
| `TestHardResetDownAlwaysFlushes` | `internal/component/bgp/plugins/gr/gr_test.go` | Hard Reset forces flush even with N-bit negotiated | |
| `TestSelectionDeferralHoldsUntilEOR` | `internal/component/bgp/reactor/selection_deferral_test.go` | selection deferred until all-EOR | |
| `TestSelectionDeferralTimerExpiryProceeds` | `internal/component/bgp/reactor/selection_deferral_test.go` | selection proceeds on timer expiry | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Hard Reset wrapped data length | >= 2 bytes (code+subcode) | 2 | 1 | N/A |
| Selection Deferral Timer (seconds) | 0-65535 | 65535 | N/A | N/A (define default at design time) |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gr-hard-reset` (`.ci`) | `test/plugin/` | operator observes a Hard Reset flushes routes while a plain NOTIFICATION under N-bit retains them | |
| `gr-selection-deferral` (`.ci`) | `test/plugin/` | after ze restart, advertisement is held until EOR/timer | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-gr-hard-reset-peer` | `test/interop/scenarios/` | FRR or GoBGP | Hard Reset (RFC 8538) interop: peer honours Cease/subcode-9 and ze unwraps a peer-sent one | |
| `NN-gr-selection-deferral-peer` | `test/interop/scenarios/` | FRR or GoBGP | restarting-speaker deferral does not blackhole during convergence | |

### Future (if deferring any tests)
- VPN ATTR_SET (RFC 6368) tests are out of scope here; they belong to the future L3VPN spec.

## Files to Modify
- `internal/component/bgp/message/notification.go` - Hard Reset (Cease subcode 9) build + unwrap
- `internal/component/bgp/plugins/gr/gr.go` - N-bit negotiation tracking; route the Hard-Reset-vs-plain-notification distinction into the retention decision
- `internal/component/bgp/plugins/gr/gr_state.go` - `onSessionDown` retention gate on negotiated N-bit
- `internal/core/bgp/capability/` - advertise ze's own N-bit in the GR capability (confirm A-1 first)
- `internal/component/bgp/reactor/` - Restarting-Speaker Selection Deferral Timer + advertisement hold
- `internal/component/bgp/fsm/` - hard-reset send path on teardown (confirm integration point at design time)
- `internal/component/bgp/yang/` - Selection Deferral Timer config leaf (validation constraints)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (Selection Deferral Timer leaf) | [ ] | `internal/component/bgp/yang/` — read `ai/rules/config.md` + `ai/rules/config.md` |
| YANG validation constraints (range on the timer) | [ ] | `range 0..65535` (default TBD at design) |
| CLI grammar (if a `request bgp ... hard-reset` verb is added) | [ ] | `ai/rules/cli.md` |
| Functional test for new behaviour | [ ] | `test/plugin/gr-hard-reset.ci`, `test/plugin/gr-selection-deferral.ci` |
| Prometheus counters (hard resets sent/received; deferral timer expiries) | [ ] | reuse GR metrics registry; list names at design time |
| RFC status ledger | [ ] | `docs/features/rfc-status.md` (RFC 8538 row; RFC 4724 Section 4.1 row) |

## Files to Create
- `internal/component/bgp/reactor/selection_deferral.go` - Selection Deferral Timer state (name TBD at design)
- `test/plugin/gr-hard-reset.ci` - functional test
- `test/plugin/gr-selection-deferral.ci` - functional test
- (before `ready`) RFC 8538 short summary via the `ze-rfc` skill

## Implementation Steps

1. **Phase: design** - re-verify A-1..A-4, generate the RFC 8538 summary (`/ze-rfc rfc8538`), fill AC/TDD/Data-Flow detail, move Status `skeleton` → `design` → `ready`.
2. **Phase: wiring** - register entry points; write the failing wiring tests from the Wiring Test table.
3. **Phase: Hard Reset message** - build/unwrap Cease subcode 9 in the message layer (TDD).
4. **Phase: N-bit negotiation + retention gate** - advertise local N-bit; gate `onSessionDown` retention on negotiated N-bit; Hard Reset always flushes (TDD).
5. **Phase: Selection Deferral Timer** - restarting-speaker deferral in the reactor; YANG config leaf (TDD).
6. **Functional + interop tests** - `.ci` + interop scenarios.
7. **Full verification** - `make ze-precommit-verify`.
8. **Complete spec** - fill audit tables, write `plan/learned/NNN-gr-advanced.md`, two-commit closure.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Hard Reset always flushes; N-bit retention only when both sides negotiated |
| Registration over hardcoding | New config/CLI surface registers and is core-discovered, not hardcoded into a core/shared package (`ai/rules/plugins.md`) |
| Data flow | Retention decision stays in the GR plugin; message layer owns wrap/unwrap; reactor owns deferral |
| Rule: no-fabrication | RFC claims cite the generated RFC 8538 summary + section, not memory |

## RFC Documentation

Add `// RFC 8538 Section X: "<quoted requirement>"` above the N-bit negotiation, the
retention gate, and the Hard Reset build/unwrap. Add `// RFC 4724 Section 4.1:
"<quoted requirement>"` above the Selection Deferral Timer. Generate the RFC 8538
short summary with the `ze-rfc` skill before quoting it.

## Known Limitations / Deferred

- **VPN ATTR_SET (RFC 6368), attribute type 128 — DEFERRED (out of GR scope).**
  RFC 6368 ("Internal BGP as the PE-CE Protocol") defines the ATTR_SET path attribute
  (type 128) that carries a CE's attributes inside a PE-PE VPN update. This is an L3VPN
  feature, orthogonal to graceful restart; it was mis-bundled under "GR advanced (L86)"
  in the 2026-07-06 deferral triage. attribute code 128 is not defined
  (`internal/core/bgp/attribute/attribute.go`), though L3VPN NLRI scaffolding exists
  (`family.SAFIVPN`, `internal/component/bgp/plugins/nlri/vpn/types.go`).
  **Destination:** a future L3VPN / PE-CE spec (to be created when that work is picked
  up). Not tracked as an AC in this spec.
- **AS-Confederation OTC (RFC 9234 Section 5, umbrella item 3, L88)** — already
  re-deferred in `plan/spec-followup-bgp-feature.md` ("Item 3 re-deferral"); ze is a
  single-AS speaker so the confederation OTC rules are vacuously satisfied. Cross-
  referenced here only; not in scope.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`), not just tests
- [ ] Registration over hardcoding respected
- [ ] Interop tests pass (or justified N/A)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (RFC 8538, RFC 4724 Section 4.1)
- [ ] RFC 8538 short summary generated before `ready`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for the timer and the Hard Reset length
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol behaviour

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/planning.md`). Moves to `design` when picked up.
- Created 2026-07-08 as the destination for umbrella item 1 ("GR advanced"). The umbrella's item-1 row points here.
