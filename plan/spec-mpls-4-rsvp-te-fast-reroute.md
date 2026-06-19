# Spec: mpls-4-rsvp-te-fast-reroute

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | mpls-3-rsvp-te (closed), mpls-1-kernel (closed) |
| Phase | 11/11 (review) |
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
| A-1 | The kernel MPLS backend can program a 2-label push (bypass + protected) in one entry | `mplsentry_linux.go` already encodes a label stack on push | Facility backup needs a different data-plane (e.g. recursive route) | `validateMPLSLabels` permits up to `maxLabelStack=16` (`mpls.go:17,27`); `addMPLSSwap(inLabel, outLabels []uint32, nextHop)` programs `MPLSDestination{Labels}` with an arbitrary-length stack (`mplsentry_linux.go:21-58`); the `OpSwap` FIB path validates + programs a multi-element `OutLabels` and is the sole caller of `addMPLSSwap` (`mplsentry.go:108-124`, LSP findReferences). No new primitive: facility backup = `addMPLSSwap(protectedInLabel, [bypassLabel, swappedProtectedLabel], bypassNextHop)`. | **CONFIRMED** — live-kernel proven: `TestMPLSIntegration_FacilityBackupSwap` programs `OutLabels:[5000,200]` and reads back `MPLSDestination{Labels:[5000,200]}` on a real kernel in QEMU (passed); no new primitive needed |
| A-2 | `handleLinkDown` runs fast enough off the iface-event worker for sub-second repair | mpls-3 already wires `EventDown` → worker → `handleLinkDown` | Need a faster failure-detection path (BFD) | `handleLinkDown` runs synchronously on the iface-event goroutine, matches the affected LSP by `admissionInterface(nextHop)`, acts inline (`engine.go:585-628`); the local-repair branch replaces the `tearLSPLocal(key)` call at line 626 with a single in-worker FIB swap reprogram — no signaling round-trip. | **validated (design)** — absolute sub-second still bounded by netlink link-down detection latency (BFD out of scope per fallback) |
| A-3 | A bypass LSP can be modeled as an ordinary LSP in `lspTable` keyed by a distinct SESSION | mpls-3 LSP table keys by SESSION+SENDER_TEMPLATE | Need a separate bypass table/abstraction | `lspKey = (TunnelEndpoint, TunnelID, ExtTunnelID, SenderAddr, LSPID)` (`fsm.go:70-76`); a bypass LSP (PLR as ingress sender, MP as endpoint, own TunnelID/LSPID) gets a distinct key in the same `map[lspKey]*LSP`. Only new `LSP` backup-association fields needed (`fsm.go:122-159`). | **validated** — no separate table |

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
| AC-10 | One-to-one backup requested | A DETOUR LSP is signaled per protected LSP and merged at the MP. **Split to `spec-mpls-9-rsvp-te-one-to-one-backup` (user-approved 2026-06-19).** One-to-one detour backup is a distinct mechanism (DETOUR object, per-LSP detour LSPs, detour merging) from the facility backup this spec delivers; it is the secondary mode per the Task scope and is tracked as its own spec rather than shipped partially. |

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

**From assumption validation (2026-06-19, pre-implementation gate):**

- **Local-repair insertion point is exact.** `handleLinkDown` (`engine.go:585-628`)
  already iterates `e.table.All()`, matches the affected LSP by
  `admissionInterface(nextHop)`, and for `RoleTransit`/`RoleEgress` sends a
  teardown PathErr (code 24/5) then calls `tearLSPLocal(key)` (line 626). FRR
  inserts a branch *before* the tear-down: if the matched LSP is protected and has
  a ready bypass, reprogram the FIB swap, set RRO "in use", send a Notify PathErr
  (code 25/3) instead of 24/5, and **skip `tearLSPLocal`**. Unprotected LSPs keep
  today's behavior unchanged. This is a localized edit, not a rewrite.

- **Facility-backup data plane is a single existing primitive.** At the PLR the
  protected LSP is a transit swap (`InLabel → OutLabel via NextHop`). Local repair
  becomes `addMPLSSwap(protectedInLabel, []uint32{bypassLabel, swappedProtectedLabel}, bypassNextHop)`.
  `netlink.MPLSDestination{Labels}` imposes labels[0] as outermost (top of stack),
  so the bypass label must be element 0 (carries the packet over the bypass) and the
  swapped protected label element 1 (the merge point continues the protected LSP).
  `validateMPLSLabels` caps depth at 16, so the 2-label stack is well within range.
  → Constraint: verify the labels[0]=outermost ordering on a live kernel in Phase 4
  (the QEMU 2-label readback) — it is the one empirical unknown left in A-1.

- **Bypass LSP keying needs no new abstraction.** `lspKey` already includes
  `SenderAddr` + `TunnelID`/`LSPID`, so a bypass (PLR-sourced, MP-terminated) is a
  distinct key in the same table. The `LSP` struct gains backup-association fields
  (which bypass protects this LSP; protection desired/available/in-use state),
  mutated under the existing `lsp.mu`.

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
- **Facility backup (RFC 4090 Section 3.2)** end to end. Wire: FAST_REROUTE
  (class 205), SESSION_ATTRIBUTE (class 207, newly emitted) with protection +
  label-recording flags, RRO protection flags (`frr.go`, `wire.go`). Config: a
  `fast-reroute` container on a tunnel and a `bypass` list (`yang`,
  `register.go`). Engine: a transit PLR arms a configured bypass matched by
  merge point (`selectBypass`), records RRO available; `handleLinkDown` redirects
  a protected LSP onto the bypass with a 2-label swap (`tryLocalRepair` +
  `ProgramBackup`), sends a Notify (code 25/3) and keeps the LSP up; the head-end
  re-optimizes make-before-break on the Notify (`reoptimizeOnNotify`).
- **Node protection (AC-6)**: `selectBypass` picks an NNHOP bypass; RRO label
  recording (RFC 3209 4.4.3) lets the PLR resolve the NNHOP's label
  (`labelForAddr`, `LSP.BackupLabel`) so the backup pushes the correct inner label.
- **CLI/metrics** (AC-8): `show rsvp-te fast-reroute`; counters
  `ze_rsvpte_local_repairs_total`, gauges `ze_rsvpte_protected_lsps` /
  `ze_rsvpte_bypass_lsps`.
- **One-to-one backup (AC-10)** split to `spec-mpls-9-rsvp-te-one-to-one-backup`
  with explicit user approval (2026-06-19).

### Bugs Found/Fixed
- **RouteAdd → RouteReplace for AF_MPLS swaps** (`mplsentry_linux.go`). Local
  repair re-programs the protected LSP's existing swap entry on the same in-label;
  the kernel `addMPLSSwap` used `RouteAdd`, which fails `EEXIST` on a live kernel,
  so the repair would silently not take effect. The fake FIB and the
  isolated-program QEMU test missed it; found in review, fixed to `RouteReplace`,
  and the QEMU `TestMPLSIntegration_FacilityBackupSwap` now programs the original
  swap THEN the backup over the same in-label to prove the replace.

### Documentation Updates
- `docs/guide/rsvp-te.md` (Fast Reroute section, config, show, metrics),
  `docs/guide/configuration.md`, `docs/guide/command-reference.md` (new
  `### show rsvp-te` section), `docs/features.md`, `rfc/short/rfc4090.md`.
- `docs/comparison.md`: no MPLS-TE/FRR comparison row exists (the tables are
  BGP-feature-focused), so no change — a full TE comparison is out of scope.
- `docs/plugin-development/metrics.md`: a pattern guide, not a metric catalog;
  the new counters follow the documented pattern (the catalog is the rsvp-te guide).

### Deviations from Plan
- One-to-one backup (AC-10, Phase 7) split to a follow-up spec with user approval
  (the spec's secondary mode; a distinct mechanism).
- Bypass paths are explicitly configured (a `bypass` list), not auto-computed:
  ze has no IGP/CSPF (out of scope per Known Limitations). The PLR auto-associates
  a configured bypass by merge point.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestBuildPathIncludesFastReroute`, `TestParseRSVPTEConfigFastReroute` | PATH carries FAST_REROUTE + SESSION_ATTRIBUTE local-protection flag |
| AC-2 | Done | `TestPLRArmsBypass` | PLR selects the NHOP bypass, RRO records "available" |
| AC-3 | Done | `TestLocalRepairSwitchesFIB`, QEMU `TestMPLSIntegration_FacilityBackupSwap` | 2-label push within the link-down handler; live-kernel proven |
| AC-4 | Done | `TestLocalRepairSendsNotify` | PathErr code 25/3; LSP NOT torn down |
| AC-5 | Done | `TestHeadEndReoptimizesOnNotify` | make-before-break reroute; old LSP torn once new up (interop) |
| AC-6 | Done | `TestNodeProtectionLocalRepair` | NNHOP merge point + NNHOP recorded label pushed |
| AC-7 | Done | `TestFacilityBypassProtectsMultipleLSPs` | one bypass carries multiple protected LSPs |
| AC-8 | Done | `TestShowFastReroute`, `test/rsvpte/rsvpte-frr.ci` | `show rsvp-te fast-reroute` reports protection state |
| AC-9 | Done | `TestLocalRepairSwitchesFIB` (stays on bypass) + AC-5 re-optimization | LSP stays on the bypass until the head-end re-optimizes; one swap entry (replace), no duplicate forwarding |
| AC-10 | Split | `spec-mpls-9-rsvp-te-one-to-one-backup` | one-to-one detour backup; user-approved split 2026-06-19 |

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
| Local repair on link failure (no re-signaling round trip) | unit + ze-to-ze interop + QEMU | `TestLocalRepairSwitchesFIB` redirects the FIB to the 2-label backup stack in the link-down handler; `TestEngineZeToZeFRRLocalRepair` over the 4-node fabric; data-plane 2-label swap (with replace-over-existing) verified on a live kernel in QEMU (`TestMPLSIntegration_FacilityBackupSwap`) |
| Notify keeps the LSP up, head-end re-optimizes | unit | `TestLocalRepairSendsNotify` (code 25/3, LSP retained), `TestHeadEndReoptimizesOnNotify` (make-before-break LSP_ID 2) |
| Facility backup shares one bypass across LSPs | unit | `TestFacilityBypassProtectsMultipleLSPs` (two protected LSPs, one bypass, two label-stacked backups) |
| Node protection survives the next node | unit | `TestNodeProtectionLocalRepair` (NNHOP merge + NNHOP recorded label pushed) |

## Review Gate

### Run 1 (self-review against Critical Review Checklist)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | Local repair re-programs the protected LSP's existing swap on the same in-label, but `addMPLSSwap` used `RouteAdd` → fails `EEXIST` on a live kernel; repair silently no-ops | `internal/plugins/fib/kernel/mplsentry_linux.go:54` | Fixed: `RouteAdd` → `RouteReplace` (AF_MPLS in-label space is ze's own). QEMU test extended to swap-then-replace; passes on a live kernel |
| 2 | ISSUE | AC-7 (one bypass, multiple protected LSPs) had no explicit test | `frr_test.go` | Fixed: added `TestFacilityBypassProtectsMultipleLSPs` |

### Fixes applied
- RouteReplace for AF_MPLS swaps; re-verified in QEMU.
- Added the AC-7 multi-LSP facility test.

### Run 2 (independent adversarial review agent)
The reviewer verified the RFC-critical encodings as CORRECT: FAST_REROUTE (class
205/C-Type 1/len 24, byte layout), SESSION_ATTRIBUTE flags + C-Type 7 field
order, RRO protection flags, Notify (code 25/3, no teardown), facility label
stack order (bypass outermost), node-protection NNHOP-label resolution, link-vs-node
selection, RouteReplace, and that no send/emit happens under `lsp.mu`.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 3 | MEDIUM (race) | `handleResvTransit` read `lsp.PSB`/`lsp.InLabel` before taking `lsp.mu` (pre-existing mpls-3 lock-discipline violation) | `engine.go` handleResvTransit | Fixed: lock first, snapshot/allocate the in-label under the lock (AllocateLabel uses the table mutex, no inversion). `-race` interop/reconcile tests green |
| 4 | LOW (false claim) | `tryLocalRepair` claimed a repair (set in-use, Notify, skip teardown) even when `inLabel == 0` (transit LSP with no swap state yet) | `frr.go` tryLocalRepair | Fixed: `bypass == nil || inLabel == 0` falls back to base teardown |
| 5 | LOW (metric) | `localRepairs` Inc + Notify not atomic with the in-use check under hypothetical concurrent `handleLinkDown` | `frr.go` tryLocalRepair | Mitigated by design: `handleLinkDown` is driven by a single worker goroutine (one drains `linkDownCh`), so calls are serial; serial repeats return early at `if already` (no double Inc). Head-end dedups duplicate Notifies via `reoptimizeOnNotify`. No code change |
| 6 | NOTE (SHOULD) | SESSION_ATTRIBUTE did not set the SE-style flag (0x04); RFC 4090 §4.3 recommends it (reroute already reserves SE) | `frr.go` sessionAttr | Fixed: added `SessAttrSEStyle` to the advertised flags |

### Run 3 (/ze-review fresh pass on the post-fix state)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 7 | ISSUE | Node protection silently fell back to the NHOP's label when the NNHOP did not record its label (a peer ignoring label-recording), so the backup would push the wrong inner label and blackhole while still reporting protection in use | `engine.go` handleResvTransit | Fixed: disarm the bypass (clear `lsp.Bypass`) and warn when the NNHOP label is unresolvable, so a failure falls back to clean teardown. Regression test `TestNodeProtectionDisarmsWithoutNNHOPLabel` |
| 8 | NOTE | `ProgramBackup` (and pre-existing `ProgramSwap`/`RemoveSwap`) flagged by ze-validate as "no cross-package caller" -- wired intra-package via the private `fibProgrammer` interface | `fib.go` | Fixed: unexported the whole `fibProgrammer` interface + impl methods (`programPush`/`programSwap`/`programBackup`/`programPop`/`removePush`/`removeSwap`); ze-validate now reports no rsvpte findings |
| 9 | NOTE | Redundant `inLabel != 0` after the early-return guard | `frr.go` tryLocalRepair | Fixed: simplified to `if e.fib != nil` |

Test-relaxation audit: clean. Wiring verification: every new symbol has a non-test caller. Removed-behavior audit: clean (buildPathErr ttl drop and RouteAdd→RouteReplace preserve/fix behavior).

### Run 4 (/ze-review fresh pass — caught an incomplete prior fix)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 10 | ISSUE | The Run-3 node-protection disarm was undone by the next PATH refresh: `handlePathTransit` re-arms `lsp.Bypass` unconditionally, but `BackupLabel` was left at the NHOP-label fallback, so a link-down in the window between a PATH refresh and the next RESV would push the wrong label and blackhole | `engine.go` handleResvTransit + `frr.go` tryLocalRepair | Fixed: for node protection, leave `BackupLabel = 0` ("unresolved") unless the NNHOP label is in the RESV RRO, and have `tryLocalRepair` refuse when `BackupLabel == 0` (dropped the unsafe `OutLabel` fallback). A re-arm can no longer reintroduce a wrong-label repair. Regression test `TestNodeProtectionDisarmsWithoutNNHOPLabel` extended to re-arm via a PATH refresh before the link-down |

### Run 5 (full /ze-review: 4 parallel agents over the whole diff — 9 findings, user approved "fix all")
| # | Sev | Finding | Fix |
|---|-----|---------|-----|
| 11 | HIGH | FRR broke after a config reload (engine `e.cfg` frozen at startup; `selectBypass`/`admissionInterface` read stale bypasses/interfaces) | Engine config behind `atomic.Pointer`; `OnConfigApply` calls `eng.setConfig(cfg)`; reads via `e.cfg()`. Test `TestEngineConfigReloadPicksUpBypass` |
| 12 | HIGH | Unbounded wire ERO → `encodeERO` buffer overflow / remote panic on transit relay (pre-existing base codec) | Cap `decodeERO` at `maxExplicitRouteHops`; add the `off+20>len(buf)` guard to `encodeERO`. Tests `TestDecodeEROCapsHops`/`TestEncodeEROBounded`/`TestTransitRelayLargeERONoPanic` |
| 13 | MED | `handleResvTransit` read `lsp.RSB` after unlock (pre-existing race) + my Run-2 `AllocateLabel`-under-lock inversion | Restructured: allocate the in-label outside the lock (table→lsp order), build the RESV under the lock (like `sendResv`). `-race` green |
| 14 | MED | `bypassKey` index-derived → reorder re-keys live bypasses | Keyed by a stable FNV-1a name hash; config-time collision check |
| 15 | MED | SESSION_ATTRIBUTE C-Type not checked on decode (C-Type 1 misparsed) | Branch on C-Type (skip the 12-byte affinity prefix). Test `TestSessionAttributeCType1` |
| 16 | LOW | `destination`/ERO config swallowed parse errors (inconsistent with `merge-point`) | Both now error. Tests `TestParseRSVPTEConfigInvalidDestination`/`InvalidERO` |
| 17 | LOW | Reserved tunnel-id range not enforced; no bypass cap | `parseConfig` rejects tunnel-id ≥ `0xF000`; `maxBypasses` cap + collision check. Test `TestParseRSVPTEConfigReservedTunnelID` |
| 18 | LOW | Stale `protection-available` when a bypass goes down | `clearBypassReferences` clears referencing LSPs on bypass teardown. Test `TestBypassTeardownClearsProtection` |
| 19 | NOTE | decodeSessionAttr truncated-name leniency; `reoptimizeOnNotify` empty-ERO | truncated-name now errors; empty-ERO documented as a re-optimization-quality limit |

### Run 6 (full /ze-review #2: 3 parallel agents over the fix code — caught a HIGH regression in the fixes)
| # | Sev | Finding | Fix |
|---|-----|---------|-----|
| 20 | HIGH | `validateBypasses` (new) called `bypassKey` → `addrToUint32` → `As4()`, which panics on the zero/IPv6 router-id; router-id is not mandatory in YANG, so a `bypass` with no `router-id` crashed the plugin on config verify | Guard `validateBypasses` when `!RouterID.IsValid()`; reject a non-IPv4 router-id in `parseConfig` (also fixes the pre-existing `tunnelKey` IPv6 panic). Tests `TestParseRSVPTEConfigBypassNoRouterID`/`IPv6RouterID` |
| 21 | LOW | reserved tunnel-id check cast `float→uint16` before range-checking (65536 wraps to 0, aliasing) | Range-check `v` before the cast. Test `TestParseRSVPTEConfigTunnelIDOutOfRange` |
| 22 | LOW | `selectBypass`/`handlePathTransit` read `e.cfg()` multiple times (benign reload skew) | Snapshot `cfg := e.cfg()` once |
| 23 | LOW | `handlePathTear` did not clear bypass references | Defensive `clearBypassReferences` (a bypass shouldn't receive a PathTear, but consistent) |

Agents A/B/C otherwise verified the fix code correct: atomic config (Store/Load, escape, whole-object swap), the `handleResvTransit` restructure (lock order, label-leak path), `clearBypassReferences` (no inversion/re-lock), the ERO cap/guard, C-Type 1 decode bounds, name-hash keying, and the reload flow.

### Run 7 (full /ze-review #3: convergence pass — closed the As4()-panic-on-reload class)
| # | Sev | Finding | Fix |
|---|-----|---------|-----|
| 24 | HIGH | `OnConfigApply` (reload) lacked the invalid-router-id guard `OnStarted` has, so a reload to a config with a tunnel/bypass but no router-id reached `setupTunnel`/`tunnelKey`/`bypassKey` → `As4()` → **plugin crash** (router-id not mandatory in YANG; pre-existing for the tunnel path, the bypass path was new) | Guard `reconcileTunnels` itself (`if !cfg.RouterID.IsValid() { return prev }`) — covers every caller. Test `TestReconcileTunnelsNoRouterIDNoPanic` |
| 25 | HIGH | `setConfig` adopted a reloaded config wholesale, so a reload that removed/changed the router-id made runtime reads (`selectBypass`/`buildPath` → `As4()`) panic on the next PATH | `setConfig` preserves the engine's RouterID (LSR identity is restart-class). Test `TestSetConfigPreservesRouterID` |

Agent confirmed all other latest fixes correct and the remaining note (refresh/cleanup tickers capture the period at launch) is a benign pre-existing timing gap, not a crash.

### Run 8 (full /ze-review #4: convergence pass — caught a split-brain the router-id fix introduced)
| # | Sev | Finding | Fix |
|---|-----|---------|-----|
| 26 | MED | After `setConfig` preserves the router-id, `OnConfigApply` still reconciled with `activeCfg`'s NEW router-id, so a reload that *changed* router-id to a different valid IPv4 signaled LSPs under the new key while the engine's `selectBypass` resolved the old key → FRR silently off; PATHs carried mismatched SENDER_TEMPLATE/RSVP_HOP | Reconcile against the engine's effective config (`cfg = eng.cfg()`); warn that a router-id change needs a restart. Test `TestConfigReloadRouterIDChangeNoSplitBrain` |

Agent swept all other `As4()`/index/label-leak/state-hole paths and confirmed clean.

### Run 9 (full /ze-review #5: reconcile-diff deep pass — found a pre-existing oversubscription bug the reload exposed)
| # | Sev | Finding | Fix |
|---|-----|---------|-----|
| 27 | HIGH | `admission.setInterface` replaced the whole `*interfaceBandwidth`, zeroing `ReservedBandwidth` while live reservations remained in `sessions`; `OnConfigApply` calls it for every interface on every reload, so any `commit` with RSVP-TE LSPs up on a transit/egress node reset the reserved counter → admission control admits past `MaxReservable` (oversubscription). **Pre-existing** from `f3ac11eb8` (reconcile-on-reload); fixed here because it directly undermines FRR bandwidth-protection | `setInterface` is now read-modify-write: for an existing interface it updates only the limits, preserving `ReservedBandwidth` and `sessions`. Test `TestSetInterfaceReloadPreservesReservation` |

The reconcile prev/next diff, the effective-config (`cfg = eng.cfg()`), the verify/apply consistency, and the `setConfig` single-writer RMW were all confirmed converged.

**Known pre-existing limitations (out of FRR scope, documented not fixed):**
- The refresh/cleanup loops capture `cfg` by value at `OnStarted`, so a reloaded `refresh-period`/`refresh-multiplier` does not change the ticker cadence or `expiredPSBs` factor until restart (newly-built PSBs do carry the new period). Benign soft-state timing skew, not a crash.
- `admission` has no `removeInterface`, so an interface removed on reload leaves a stale `interfaces`/`sessions` entry. Bounded; no oversubscription.

### Run 10 (full /ze-review #6: FRR-scope convergence — display/metrics/re-opt/RRO/label-order)
Reviewed the less-trodden FRR code (show builders, gauges/counter, `reoptimizeOnNotify`, RRO self-flags/`labelForAddr`, the 2-label backup stack, FRR object encoding). **No FRR-introduced bugs.** The one flagged concern — fixed encoders write at offsets without a buffer check, and a giant ERO could in theory overflow — was proven a non-issue: the 64-hop cap plus every trailing object (incl. max-name SESSION_ATTRIBUTE + FAST_REROUTE) encodes to ≤1500. Pinned by `TestTransitRelayLargeERONoPanic` (now worst-case: 64 IPv6 hops + protection, asserts `len ≤ maxRSVPMessage`).

### Final status — CONVERGED
- [x] 10 review passes (self + multiple multi-agent rounds); findings drove 9 → 5 → 2 → 1 → 1 → 0 (Run 10 found no real FRR bug)
- [x] Runs 5–6: 14 findings in the FRR code (3 HIGH), all fixed + regression-tested
- [x] Runs 7–9: 4 findings the fixes/reload exposed (3 HIGH incl. 1 pre-existing oversubscription bug fixed as a bonus), all fixed + regression-tested
- [x] Run 10: FRR feature confirmed converged; the one buffer concern proven safe by a worst-case test
- [x] `-race` green, `golangci-lint` 0 issues, `ze-validate` no rsvpte findings, test-relaxation clean, 135 package tests pass
- [x] Two pre-existing LOW limitations documented (refresh-ticker staleness, no `removeInterface`)
- [x] All NOTEs recorded above

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
| A-1 | CONFIRMED | Live-kernel QEMU `TestMPLSIntegration_FacilityBackupSwap`: programs `OutLabels:[5000,200]` (replacing an existing single-label swap on the same in-label) and reads back `MPLSDestination{Labels:[5000,200]}`. No new data-plane primitive needed. |
| A-2 | CONFIRMED (design) | `handleLinkDown` runs inline on the iface-event goroutine; local repair is a single in-worker FIB swap (`tryLocalRepair`), no signaling round trip (`engine.go`). Absolute sub-second remains bounded by netlink link-down detection latency (BFD out of scope, as the fallback states). |
| A-3 | CONFIRMED | Bypass LSPs key distinctly in the same `map[lspKey]*LSP` via `bypassTunnelIDBase` (`frr.go`); `TestPLRArmsBypass` asserts the reserved range. New `LSP` backup-association fields added; no separate table. |

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
