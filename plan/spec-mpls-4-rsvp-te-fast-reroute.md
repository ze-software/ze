# Spec: mpls-4-rsvp-te-fast-reroute

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | mpls-3-rsvp-te (closed), mpls-1-kernel (closed) |
| Phase | - |
| Updated | 2026-06-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc4090.md` - Fast Reroute extensions (MUST CREATE)
4. `internal/component/rsvpte/engine.go` - signaling engine + `handleLinkDown` (the local-repair trigger point)
5. `internal/component/rsvpte/reroute.go` - make-before-break (`reroute`, `teardownLSP`) the head-end re-optimization reuses
6. `internal/component/rsvpte/wire.go` / `build.go` - object codecs the new FRR objects extend
7. `plan/learned/921-mpls-rsvp-te.md` - what the base RSVP-TE engine does and its known limitations

## Task

Implement RSVP-TE Fast Reroute (RFC 4090) so an LSP survives a link or node
failure with sub-second local repair instead of waiting for head-end
re-signaling. This is the mpls-3 AC-13 deferral.

Today (mpls-3) a link failure tears the LSP down and reports a PathErr toward
the head-end (`handleLinkDown`), which re-establishes from scratch: seconds of
outage. Fast Reroute pre-signals a **backup path** around the protected
resource so the Point of Local Repair (PLR) can redirect traffic onto it the
moment the failure is detected, then signal the head-end to re-optimize at
leisure.

Scope this spec to **facility backup (bypass tunnels)** as the primary mode
(RFC 4090 Section 3.2) — one bypass LSP protects every LSP crossing a link, the
common deployment. **One-to-one backup (detour LSPs)** (Section 3.1) is a
secondary phase. P2MP, refresh reduction, preemption and CSPF are out of scope
(see Known Limitations / Future Work).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/` and `docs/guide/rsvp-te.md` - existing RSVP-TE config/CLI surface to extend
  → Constraint: CLI uses `show rsvp-te <noun>`; config is the keyed-map tunnel/interface shape (see mpls-3 parser)
- [ ] `internal/component/iface/` event model - `EventDown` is the failure signal `handleLinkDown` already consumes
  → Constraint: the iface-event handler MUST NOT block (it enqueues; a worker does I/O) — the local-repair switch runs on that worker

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4090.md` - Fast Reroute Extensions for RSVP-TE LSP Tunnels (MUST CREATE)
  → Constraint: FAST_REROUTE object (class 205), DETOUR object (class 63), SESSION_ATTRIBUTE "local protection desired" / "node protection desired" / "bandwidth protection desired" flags
  → Constraint: RRO subobject flags - "Local protection available" (0x01), "Local protection in use" (0x02), "Node protection" (0x04)
  → Constraint: on local repair the PLR sends a PathErr code 25 (Notify) value 3 ("Tunnel locally repaired") toward the head-end; it MUST NOT tear the LSP down
  → Constraint: facility backup uses label stacking - the PLR pushes the bypass LSP's label under the protected LSP's (swapped) label
- [ ] `rfc/short/rfc3209.md` - base RSVP-TE (from mpls-3): SESSION_ATTRIBUTE, RRO, ERO
- [ ] `rfc/short/rfc2205.md` - base RSVP soft-state (from mpls-3)

**Key insights:**
- A **bypass tunnel** is itself an ordinary RSVP-TE LSP (reuse the mpls-3 ingress/transit/egress engine) whose ingress is the PLR and egress is the merge point (NHOP for link protection, NNHOP for node protection).
- The PLR must learn the protected LSP's downstream label and the MP address to build the backup forwarding (push bypass label on top of the swapped protected label).
- Local repair is a **data-plane** switch (re-point the FIB swap/push to the bypass next-hop with an extra label) plus a **control-plane** notify (PathErr Notify upstream); the protected LSP's soft-state keeps refreshing through the bypass.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE implementing this spec)
- [ ] `internal/component/rsvpte/engine.go` - `handlePath*`/`handleResv*` install state and program push/swap/pop; `handleLinkDown` currently TEARS DOWN affected LSPs and PathErrs the head-end
  → Constraint: `handleLinkDown` matches affected LSPs by the interface facing `lsp.NextHop` (`admissionInterface(nextHop)`); FRR reuses that match to find which LSPs a failed link protects
- [ ] `internal/component/rsvpte/reroute.go` - `reroute` (make-before-break) + `teardownLSP`; the head-end re-optimization after a local repair reuses `reroute`
- [ ] `internal/component/rsvpte/wire.go` - object encode/decode; FAST_REROUTE and DETOUR objects and the SESSION_ATTRIBUTE/RRO flags are NOT implemented
- [ ] `internal/component/rsvpte/build.go` - whole-message encoders; need FAST_REROUTE in PATH and the Notify PathErr
- [ ] `internal/component/rsvpte/fsm.go` - `LSP`/`lspTable`; needs backup-association fields (which bypass protects this LSP, protection state)
- [ ] `internal/component/rsvpte/register.go` - `reconcileTunnels` config path; bypass tunnels are configured/auto-computed here
- [ ] `internal/plugins/fib/kernel/mplsentry_linux.go` - MPLS FIB entries; facility backup needs a 2-label stack (bypass + protected) on local repair

**Behavior to preserve:**
- Base PATH/RESV/PathTear/PathErr signaling and the ze-to-ze interop suite (`interop_test.go`) stay green.
- Non-protected LSPs keep the current `handleLinkDown` behavior (tear down + PathErr) when no backup exists.
- The kernel MPLS data plane (push/swap/pop) is unchanged for unprotected LSPs.

**Behavior to change:**
- `handleLinkDown` for a PROTECTED LSP with an available backup: redirect to the bypass (data-plane switch + label stack) and send PathErr Notify (code 25/3) upstream, instead of tearing down.
- PATH carries a FAST_REROUTE object when "local protection desired"; RRO records protection-availability/in-use flags.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config: a tunnel with `fast-reroute { ... }` (protection desired) and, for facility backup, bypass tunnel definitions (or auto-computed bypass).
- Wire: PATH with FAST_REROUTE object (transit nodes learn protection is desired and become candidate PLRs); RESV RRO flags reporting protection availability.
- Event: iface `EventDown` (the local-repair trigger, already consumed by `handleLinkDown`).

### Transformation Path
1. **Head-end**: tunnel config with protection → PATH includes FAST_REROUTE + SESSION_ATTRIBUTE protection flags.
2. **PLR (transit)**: on PATH with protection desired, select/establish a bypass LSP to the MP (NHOP/NNHOP), record the association, set RRO "local protection available".
3. **Failure**: iface `EventDown` → `handleLinkDown` finds protected LSPs on the failed link that have a ready bypass → reprogram the FIB to push the bypass label under the protected label and forward via the bypass next-hop (local repair), set RRO "local protection in use".
4. **Notify**: PLR sends PathErr code 25 value 3 upstream; the head-end receives it and triggers `reroute` (make-before-break) onto a fresh optimal path.
5. **Cleanup**: once the head-end's re-optimized LSP is up, the old (locally-repaired) LSP is torn down (`teardownLSP`), releasing the bypass usage.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ engine | FAST_REROUTE/DETOUR object codecs, RRO/SESSION_ATTRIBUTE flags | [ ] |
| iface event ↔ engine | `EventDown` → local-repair switch (worker goroutine) | [ ] |
| engine ↔ FIB | 2-label stack program on local repair (`mplsfibevents`) | [ ] |
| PLR ↔ head-end | PathErr Notify (code 25/3) triggers re-optimization | [ ] |

### Integration Points
- `handleLinkDown` - extend to attempt local repair before tear-down.
- `reroute` - the head-end re-optimization after a Notify.
- `reconcileTunnels` - bypass tunnel lifecycle (configured or auto-computed) alongside protected tunnels.
- `mplsfibevents` / kernel `mplsentry_linux.go` - 2-label push for the backup forwarding.

### Architectural Verification
- [ ] No bypassed layers (local repair flows iface-event → engine → FIB)
- [ ] No unintended coupling (bypass LSP is an ordinary LSP in the same table)
- [ ] No duplicated functionality (reuses the mpls-3 signaling engine for the bypass LSP)
- [ ] Zero-copy preserved where applicable (wire objects use the buffer-first encoders)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The kernel MPLS backend can program a 2-label push (bypass + protected) in one entry | `mplsentry_linux.go` already encodes a label stack on push | Facility backup needs a different data-plane (e.g. recursive route) | grep `mplsentry_linux.go` for multi-label encap + a QEMU integration test | unvalidated |
| A-2 | `handleLinkDown` runs fast enough off the iface-event worker for sub-second repair | mpls-3 already wires `EventDown` → worker → `handleLinkDown` | Need a faster failure-detection path (BFD) | bench the event-to-FIB latency | unvalidated |
| A-3 | A bypass LSP can be modeled as an ordinary LSP in `lspTable` keyed by a distinct SESSION | mpls-3 LSP table keys by SESSION+SENDER_TEMPLATE | Need a separate bypass table/abstraction | design review against `fsm.go` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Facility backup label-stacking interacts badly with the kernel MPLS FIB (max stack depth, encap limits) | QEMU integration test for a 2-label push fails | Start with one-to-one (detour) backup, which uses a single swapped label, then add facility |
| R-2 | No open-source RSVP-TE peer to interop-test FRR against | (known from mpls-3) | ze-to-ze interop in the fabric (the established substitute); cross-vendor against a proprietary lab container is optional |
| R-3 | Re-optimization storm if many LSPs locally repair onto one bypass and all Notify at once | head-end CPU spike on a link failure | rate-limit head-end re-optimization; the bypass keeps forwarding meanwhile |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config `tunnel { fast-reroute { facility } }` | → | PATH carries FAST_REROUTE object | `TestBuildPathIncludesFastReroute` |
| PATH with FAST_REROUTE at a transit | → | PLR selects/establishes a bypass, sets RRO "protection available" | `TestPLRArmsBypass` |
| iface `EventDown` on a protected link | → | local repair: FIB switched to bypass, PathErr Notify sent | `TestLocalRepairOnLinkDown` |
| PathErr Notify at the head-end | → | `reroute` re-optimization triggered | `TestHeadEndReoptimizesOnNotify` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Tunnel configured with `fast-reroute` (facility) | PATH carries FAST_REROUTE object + SESSION_ATTRIBUTE "local protection desired" |
| AC-2 | Transit node (PLR) receives a protection-desired PATH | Selects/establishes a bypass LSP to the NHOP merge point; RRO records "local protection available" |
| AC-3 | Protected link fails at the PLR (iface EventDown) | Traffic redirected onto the bypass (2-label push) within the event-handling path; RRO marks "local protection in use" |
| AC-4 | Local repair performed | PLR sends PathErr code 25 ("Notify") value 3 ("Tunnel locally repaired") upstream; LSP NOT torn down |
| AC-5 | Head-end receives the Notify | Triggers make-before-break `reroute` onto a fresh path; tears the repaired LSP once the new one is up |
| AC-6 | Node protection requested | Bypass merge point is the NNHOP (next-next-hop), protecting against the next node's failure |
| AC-7 | One bypass protects multiple LSPs over the same link | A single bypass LSP carries all protected LSPs (facility backup), each as a label-stacked sub-flow |
| AC-8 | `show rsvp-te session` / `show rsvp-te fast-reroute` | Displays protection state per LSP (desired/available/in-use), bypass association |
| AC-9 | Protected link recovers before re-optimization | LSP either stays on the bypass until head-end re-optimizes, or reverts cleanly (no black hole, no duplicate forwarding) |
| AC-10 | One-to-one backup requested (Phase 2) | A DETOUR LSP is signaled per protected LSP and merged at the MP |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a tunnel with fast-reroute | config → reconcileTunnels → setupTunnel → buildPath (FAST_REROUTE) → wire | `TestBuildPathIncludesFastReroute` + ze-to-ze interop |
| 2 | A protected link fails | iface EventDown → handleLinkDown → local repair (FIB switch) + PathErr Notify | `TestLocalRepairOnLinkDown` (ze-to-ze fabric, 3+ nodes) |
| 3 | Operator inspects protection | `show rsvp-te fast-reroute` → engine state → display | `test/rsvpte/rsvpte-frr.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEncodeDecodeFastReroute` | `wire_test.go` | FAST_REROUTE object round-trip | |
| `TestEncodeDecodeDetour` | `wire_test.go` | DETOUR object round-trip (Phase 2) | |
| `TestSessionAttributeProtectionFlags` | `wire_test.go` | local/node/bandwidth protection flags | |
| `TestRROProtectionFlags` | `rro_test.go` | "available"/"in-use"/"node-protection" RRO subobject flags | |
| `TestBuildPathIncludesFastReroute` | `build_test.go` | PATH carries FAST_REROUTE when protection desired | |
| `TestPLRArmsBypass` | `frr_test.go` | PLR associates a bypass on a protection-desired PATH | |
| `TestLocalRepairSwitchesFIB` | `frr_test.go` | link-down on a protected LSP programs the 2-label push | |
| `TestLocalRepairSendsNotify` | `frr_test.go` | PathErr code 25/3 sent, LSP retained | |
| `TestHeadEndReoptimizesOnNotify` | `frr_test.go` | Notify triggers `reroute` | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Setup/hold priority | 0-7 | 7 | N/A | 8 |
| Label stack depth (protected+bypass) | 2 | 2 | 1 (no backup) | MaxLabelStack |
| Bypass bandwidth | 0-max reservable | max | negative | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rsvpte-frr` | `test/rsvpte/rsvpte-frr.ci` | configure fast-reroute, `show rsvp-te fast-reroute` reports protection state | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| ze-to-ze FRR | `internal/component/rsvpte/interop_test.go` | ze (fabric) | PLR arms a bypass, local repair on a fabric link-down redirects + Notifies, head-end re-optimizes | |
| (optional) cRPD/XRd FRR | lab container | proprietary | cross-vendor FRR interop (no open-source RSVP-TE peer exists; see mpls-3) | |

### Future (if deferring any tests)
- One-to-one (detour) backup interop is Phase 2.

## Files to Modify
- `internal/component/rsvpte/wire.go` - FAST_REROUTE (class 205), DETOUR (class 63) object codecs; SESSION_ATTRIBUTE + RRO protection flags
- `internal/component/rsvpte/build.go` - include FAST_REROUTE in PATH; build the Notify PathErr (code 25/3)
- `internal/component/rsvpte/engine.go` - PLR arming on protection-desired PATH; `handleLinkDown` local-repair branch; head-end Notify handling
- `internal/component/rsvpte/reroute.go` - head-end re-optimization on Notify (reuse `reroute`)
- `internal/component/rsvpte/fsm.go` - LSP backup-association + protection-state fields
- `internal/component/rsvpte/register.go` - bypass tunnel lifecycle in `reconcileTunnels`; FRR config parse
- `internal/component/rsvpte/cmd_show.go` - `show rsvp-te fast-reroute`
- `internal/component/rsvpte/yang/*.yang` - `fast-reroute` config container (facility/one-to-one, node-protection, bandwidth-protection)
- `internal/core/diagnostic/codes.go` - any new doctor/diagnostic codes

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (fast-reroute container) | [x] | `internal/component/rsvpte/yang/` |
| YANG validation constraints | [x] | enums for protection mode; priority ranges |
| CLI commands/flags | [x] | `show rsvp-te fast-reroute` |
| CLI grammar (action before identifier) | [x] | `ai/rules/cli-grammar.md` |
| Functional test for new RPC/API | [x] | `test/rsvpte/rsvpte-frr.ci` |
| Prometheus counters/metrics | [x] | `ze_rsvpte_local_repairs_total`, `ze_rsvpte_protected_lsps`, `ze_rsvpte_bypass_lsps` |
| Doctor check for runtime dependencies | [ ] | none new (reuses the raw-socket + kernel MPLS deps from mpls-1/3) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` |
| 6 | Has a user guide page? | [x] | `docs/guide/rsvp-te.md` (fast-reroute section) |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc4090.md` |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` |
| 14 | Prometheus counters added/changed? | [x] | `docs/plugin-development/metrics.md` |

## Files to Create
- `internal/component/rsvpte/frr.go` - PLR/MP local-protection logic (arm bypass, local repair, Notify)
- `internal/component/rsvpte/frr_test.go` - unit tests for PLR arming, local repair, Notify, re-optimization
- `rfc/short/rfc4090.md` - Fast Reroute RFC summary
- `test/rsvpte/rsvpte-frr.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - `fast-reroute` YANG container + config parse; PATH includes a (stub) FAST_REROUTE object; failing wiring tests.
   - Tests: `TestBuildPathIncludesFastReroute`
   - Files: `yang/`, `register.go`, `build.go`
2. **Phase: Wire objects** - FAST_REROUTE + SESSION_ATTRIBUTE/RRO protection flags (DETOUR deferred to Phase 7).
   - Tests: `TestEncodeDecodeFastReroute`, `TestSessionAttributeProtectionFlags`, `TestRROProtectionFlags`
   - Files: `wire.go`, `build.go`
3. **Phase: PLR arming** - on a protection-desired PATH, a transit selects/establishes a bypass LSP to the NHOP and records the association + RRO "available".
   - Tests: `TestPLRArmsBypass`
   - Files: `frr.go`, `fsm.go`, `engine.go`, `register.go`
4. **Phase: Local repair (data plane)** - `handleLinkDown` redirects a protected LSP onto the bypass with a 2-label push; RRO "in use".
   - Tests: `TestLocalRepairSwitchesFIB`; QEMU integration test for the 2-label push
   - Files: `frr.go`, `engine.go`, kernel `mplsentry_linux.go` (if multi-label push needs work)
5. **Phase: Notify + head-end re-optimization** - PathErr code 25/3 upstream; head-end triggers `reroute`.
   - Tests: `TestLocalRepairSendsNotify`, `TestHeadEndReoptimizesOnNotify`
   - Files: `frr.go`, `engine.go`, `reroute.go`, `build.go`
6. **Phase: Node protection** - bypass to the NNHOP (AC-6).
   - Files: `frr.go`
7. **Phase: One-to-one backup** - DETOUR LSP per protected LSP (AC-10).
   - Tests: `TestEncodeDecodeDetour`
   - Files: `wire.go`, `frr.go`
8. **Phase: CLI/metrics/docs** - `show rsvp-te fast-reroute`, counters, guide.
9. **Phase: ze-to-ze interop** - extend `interop_test.go` with a PLR/MP local-repair scenario over the fabric.
10. **Full verification** - `make ze-verify`; QEMU integration for the data plane.
11. **Complete spec** - learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | FAST_REROUTE/DETOUR/RRO flag encodings match RFC 4090 exactly; Notify is code 25 value 3 |
| Data flow | Local repair: iface-event → engine → FIB; no bypassed layer |
| No-tear-down on repair | A protected LSP is NOT removed on link-down when a bypass is ready (regression vs mpls-3 `handleLinkDown`) |
| Soft-state | The protected LSP keeps refreshing through the bypass until head-end re-optimizes |
| Rule: no-layering | Bypass LSP reuses the mpls-3 engine, not a parallel signaling path |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `frr.go` exists with PLR/MP logic | `ls internal/component/rsvpte/frr.go` |
| RFC summary | `ls rfc/short/rfc4090.md` |
| Functional test | `ls test/rsvpte/rsvpte-frr.ci` |
| ze-to-ze local-repair interop | `grep -n "LocalRepair\|FastReroute" internal/component/rsvpte/interop_test.go` |
| Data-plane proven on a live kernel | QEMU integration test for the 2-label push passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | FAST_REROUTE/DETOUR object lengths validated before parsing (bounded, like the mpls-3 ERO/RRO) |
| Resource exhaustion | Bound the number of bypass LSPs and protected LSPs per bypass; cap re-optimization rate (R-3) |
| Label stack | Bound the imposed stack depth (protected + bypass ≤ MaxLabelStack) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |
| Data-plane reject on a live kernel | RESEARCH the kernel MPLS multi-label encap limits (R-1); fall back to one-to-one first |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Facility backup first, one-to-one second | one-to-one first | facility is the common deployment and reuses one bypass for many LSPs; one-to-one is simpler per-LSP but scales worse |
| Bypass LSP is an ordinary `lspTable` LSP | a separate bypass abstraction | reuses the whole mpls-3 signaling engine; the bypass is just another LSP with the PLR as ingress |
| Reuse `reroute` for head-end re-optimization | a new re-optimization path | make-before-break already does exactly the "new LSP up, old torn" dance the Notify needs |

## Known Limitations
- **Out of scope here, each a candidate follow-up spec (the rest of the mpls-3 deferred work):**
  - **Refresh reduction (RFC 2961)** - bundle messages, summary refresh (Srefresh), MESSAGE_ID/ACK; scales soft-state on large LSP counts. (`spec-mpls-5-rsvp-te-refresh-reduction`)
  - **P2MP LSPs (RFC 4875)** - point-to-multipoint trees; new SESSION/S2L sub-LSP handling. (`spec-mpls-6-rsvp-te-p2mp`)
  - **Preemption** - setup/hold-priority-based displacement of lower-priority LSPs under contention; the admission controller already tracks priorities but does not preempt. (`spec-mpls-7-rsvp-te-preemption`)
  - **CSPF / automatic path computation** - constrained SPF over BGP-LS TE topology instead of explicit-only EROs. (`spec-mpls-8-rsvp-te-cspf`)
  - **Cross-vendor interop validation** - run FRR (and base RSVP-TE) against a proprietary lab container (Juniper cRPD / Cisco IOS XRd / Arista cEOS); no open-source RSVP-TE peer exists, so this needs a non-open image and is validation, not a code feature.
- BFD-driven failure detection (faster than iface netlink events) is out of scope; FRR here triggers on the existing `EventDown`.

## RFC Documentation

Add `// RFC 4090 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: FAST_REROUTE/DETOUR object formats, SESSION_ATTRIBUTE/RRO protection flags, the Notify (code 25 value 3) on local repair, the no-tear-down-on-repair rule.

## Implementation Summary

### What Was Implemented
- (to be filled during implementation)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Sub-second local repair on link failure | unit + ze-to-ze interop + QEMU | local-repair test redirects FIB before head-end re-signaling; data-plane 2-label push verified on a live kernel |
| Head-end re-optimizes after a Notify | unit | `TestHeadEndReoptimizesOnNotify` |
| Facility backup shares one bypass | unit | `TestPLRArmsBypass` (multiple protected LSPs, one bypass) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Data-plane proven on a live kernel (QEMU)
- [ ] Architecture docs and guides updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests (ze-to-ze; cross-vendor optional)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-mpls-rsvp-te-fast-reroute.md`
- [ ] Commit A (code + spec + learned); Commit B (`git rm` spec)
