# Spec: mpls-9-rsvp-te-one-to-one-backup

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | mpls-4-rsvp-te-fast-reroute (facility backup), mpls-3-rsvp-te (closed), mpls-1-kernel (closed) |
| Phase | - |
| Updated | 2026-06-19 |

PATH RELOCATION (2026-07-22 plan review; citations corrected in-body
2026-07-22): the package moved from `internal/component/rsvpte/` to
`internal/plugins/rsvpte/` (the tiers reorg). Every source citation in this
spec now uses the new path. Line cites re-verified at the new path and still
hold -- `wire.go` `ClassDetour = 63` `:61`, `CTypeDetourIPv4 = 7` `:78`,
`FRRFlagOneToOneBackup = 0x01` `:87`; `frr.go` handles the one-to-one flag
(`:179`, `:201`) with no DETOUR codec, exactly the gap this spec fills.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `rfc/short/rfc4090.md` - Fast Reroute extensions (DETOUR object, Section 3.1 one-to-one)
3. `internal/plugins/rsvpte/frr.go` - the facility-backup FRR engine this extends (protectionRequest, selectBypass, tryLocalRepair, FAST_REROUTE/SESSION_ATTRIBUTE codecs)
4. `internal/plugins/rsvpte/engine.go` - `handlePathTransit` (PLR arming), `handleLinkDown` (local repair), `handleResvTransit` (RRO/label recording)
5. `plan/learned/NNN-mpls-rsvp-te-fast-reroute.md` - what the facility-backup mode does and why one-to-one was split out

## Task

Implement RSVP-TE **one-to-one backup** (RFC 4090 Section 3.1), the secondary
fast-reroute mode that mpls-4 split out (mpls-4 AC-10). Where facility backup
(mpls-4) shares one bypass LSP across every protected LSP crossing a resource,
one-to-one backup signals a **separate detour LSP per protected LSP**, each
carrying a DETOUR object, that routes around the protected resource and merges
back into the protected LSP at a downstream node.

This reuses the mpls-4 facility-backup engine wherever possible: the
FAST_REROUTE object already carries the "one-to-one backup desired" flag
(`FRRFlagOneToOneBackup`), the config already parses `backup one-to-one`
(`frrTunnelConfig.OneToOne`), and the local-repair / Notify / head-end
re-optimization machinery is unchanged. What is new is the DETOUR object codec,
per-protected-LSP detour LSP signaling, and detour **merging** (RFC 4090 Section
3.1: detours for the same protected LSP that meet at a node merge into one).

## Required Reading

### RFC Summaries
- [ ] `rfc/short/rfc4090.md` - DETOUR object (class 63, C-Type 7 IPv4); Section 3.1 one-to-one backup; Section 6.2 detour merging
  → Constraint: DETOUR carries (PLR_ID, Avoid_Node_ID) pairs; merging combines detours protecting the same LSP that arrive at a common node
  → Constraint: the FAST_REROUTE "one-to-one backup desired" flag (0x01) selects this mode; "facility backup desired" (0x02) selects mpls-4's mode

### Source files (read BEFORE implementing)
- [ ] `internal/plugins/rsvpte/frr.go` - facility-backup engine to extend; `protectionRequest.Facility` already distinguishes the modes
- [ ] `internal/plugins/rsvpte/wire.go` - object codecs; `ClassDetour`/`CTypeDetourIPv4` constants already declared (mpls-4)
- [ ] `internal/plugins/rsvpte/engine.go` - `handlePathTransit` arming, `handleLinkDown` repair, `reroute.go` make-before-break
- [ ] `internal/plugins/rsvpte/register.go` - `reconcileTunnels`/`setupBypass`; detour LSPs are signaled similarly but per protected LSP

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE implementing this spec)
- [ ] `internal/plugins/rsvpte/frr.go` - facility-backup engine: `protectionRequest` (`.Facility` distinguishes the modes), `selectBypass`, `tryLocalRepair`, FAST_REROUTE/SESSION_ATTRIBUTE codecs. One-to-one extends this.
- [ ] `internal/plugins/rsvpte/wire.go` - `ClassDetour`/`CTypeDetourIPv4` constants are declared but have no codec or caller yet.
- [ ] `internal/plugins/rsvpte/engine.go` - `handlePathTransit` (arming), `handleLinkDown` (local repair), `handleResvTransit` (RRO/label recording).
- [ ] `internal/plugins/rsvpte/register.go` - `setupBypass`/`reconcileTunnels`; detour LSPs signal similarly but per protected LSP.

**Behavior to preserve:**
- Facility backup (mpls-4) stays unchanged: a PATH with the facility flag arms a
  shared bypass and `tryLocalRepair` programs the 2-label stack exactly as today.
- Non-protected LSPs and the base PATH/RESV/PathErr/PathTear signaling stay green.

**Behavior to change:**
- A PATH with the one-to-one flag is already parsed (`frrTunnelConfig.OneToOne` →
  `protectionRequest.Facility == false`) but the PLR only arms **facility**
  bypasses today, so a one-to-one request finds no backup and the LSP falls back
  to tear-down on failure. One-to-one adds per-LSP detour signaling and the DETOUR
  object codec (`ClassDetour` currently has no codec/caller).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a tunnel with `fast-reroute { backup one-to-one }`.
- Wire: PATH FAST_REROUTE with the one-to-one flag; the PLR's detour PATH carries
  a DETOUR object.
- Event: iface `EventDown` triggers the switch to the detour LSP (as for facility).

### Transformation Path
1. **Head-end**: one-to-one config → PATH FAST_REROUTE one-to-one flag set.
2. **PLR (transit)**: on a one-to-one PATH, signal a detour LSP (DETOUR object,
   PLR_ID + Avoid_Node_ID) along a configured detour path to the MP; record it.
3. **Merge**: detours for the same protected LSP that meet at a node merge into one
   (RFC 4090 Section 6.2).
4. **Failure**: iface `EventDown` → redirect the protected LSP onto its detour LSP.
5. **Notify + re-optimize**: reuse mpls-4's PathErr Notify and `reoptimizeOnNotify`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ engine | DETOUR object codec; FAST_REROUTE one-to-one flag | [ ] |
| iface event ↔ engine | `EventDown` → detour switch | [ ] |
| engine ↔ FIB | redirect the protected LSP's swap to the detour next hop | [ ] |

### Integration Points
- `handlePathTransit` - arm a detour (vs a facility bypass) when one-to-one is requested.
- `handleLinkDown` / `tryLocalRepair` - switch to the detour LSP.
- `reconcileTunnels` - detour path lifecycle.

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | A detour LSP is an ordinary `lspTable` LSP (PLR ingress, MP egress) keyed distinctly, like a facility bypass | mpls-4 keys bypasses in the same table via `bypassTunnelIDBase` | Need a separate detour table | design review against `fsm.go`/`frr.go` | unvalidated |
| A-2 | Detour merging can be modeled by detour LSPs sharing a SESSION at the MP | RFC 4090 Section 6.2 | Need explicit merge state | RFC re-read + ze-to-ze interop | unvalidated |
| A-3 | Detour paths can be configured explicitly (ze has no CSPF), reusing the bypass-style ERO config | mpls-4 configures bypass EROs explicitly | Need auto-computed detours (CSPF, out of scope) | config-surface review | unvalidated |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config `tunnel { fast-reroute { backup one-to-one } }` | → | PATH FAST_REROUTE flags one-to-one; PLR signals a detour LSP with a DETOUR object | `TestPLRSignalsDetour` |
| iface `EventDown` on a protected link (one-to-one) | → | local repair switches to the detour LSP | `TestOneToOneLocalRepair` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Tunnel configured with `fast-reroute backup one-to-one` | PATH FAST_REROUTE sets the one-to-one flag; the PLR signals a detour LSP carrying a DETOUR object |
| AC-2 | DETOUR object round-trip | `TestEncodeDecodeDetour`: class 63, C-Type 7, (PLR_ID, Avoid_Node_ID) pairs |
| AC-3 | Protected link fails (one-to-one) | Traffic is redirected onto the detour LSP; RRO marks local protection in use; LSP NOT torn down |
| AC-4 | Two detours for one protected LSP meet at a node | They merge into a single detour (RFC 4090 Section 6.2) |
| AC-5 | Head-end receives the Notify | Re-optimizes (reuses mpls-4's `reoptimizeOnNotify`) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEncodeDecodeDetour` | `frr_test.go` | DETOUR object round-trip | |
| `TestPLRSignalsDetour` | `frr_test.go` | PLR signals a detour LSP on a one-to-one PATH | |
| `TestOneToOneLocalRepair` | `frr_test.go` | link-down switches to the detour | |
| `TestDetourMerge` | `frr_test.go` | two detours for one LSP merge | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rsvpte-detour` | `test/rsvpte/rsvpte-detour.ci` | configure `backup one-to-one`, `show rsvp-te fast-reroute` reports the detour protection state | |

### Interop Tests
| Scenario | Directory | Peer | What It Proves | Status |
|----------|-----------|------|----------------|--------|
| ze-to-ze one-to-one | `internal/plugins/rsvpte/interop_test.go` | ze (fabric) | PLR signals a detour, local repair switches to it, head-end re-optimizes | |

## Files to Modify
- `internal/plugins/rsvpte/frr.go` - DETOUR codec; detour selection/signaling; merge logic
- `internal/plugins/rsvpte/engine.go` - one-to-one arming in `handlePathTransit`; detour switch in `handleLinkDown`
- `internal/plugins/rsvpte/register.go` - detour path config (reuse bypass-style ERO or a `detour` list)
- `internal/plugins/rsvpte/yang/ze-rsvp-te-conf.yang` - detour path config if not reusing bypass

## Files to Create
- (none expected; extends the mpls-4 `frr.go`)

## Key Design Decisions
| Decision | Alternatives | Rationale |
|----------|--------------|-----------|
| Reuse the mpls-4 facility engine | a parallel one-to-one engine | the FAST_REROUTE/SESSION_ATTRIBUTE/RRO/Notify/re-optimization machinery is shared; only DETOUR + per-LSP signaling + merging are new |
| Explicit detour paths | auto-computed (CSPF) | ze has no IGP/CSPF; CSPF is its own out-of-scope spec (mpls-8) |

## Known Limitations
- Detour merging beyond the simple same-SESSION case (RFC 4090 Section 6.2 full
  merging rules) may be staged.
- CSPF-computed detour paths are out of scope (see `spec-mpls-8-rsvp-te-cspf`).

## Implementation Steps

1. **Validate assumptions** A-1..A-3 (cheap greps / design review against `frr.go`, `fsm.go`).
2. **DETOUR codec** - `encodeDetour`/`decodeDetour` (class 63, C-Type 7) + `TestEncodeDecodeDetour`.
3. **One-to-one arming** - in `handlePathTransit`, when `!protectionRequest.Facility`, signal a detour LSP carrying a DETOUR object along a configured detour path; record the association.
4. **Local repair** - extend `tryLocalRepair` (or a sibling) to switch a one-to-one protected LSP onto its detour LSP.
5. **Detour merging** - merge detours for the same protected LSP that meet at a node (RFC 4090 Section 6.2).
6. **Notify + re-optimization** - reuse mpls-4's `reoptimizeOnNotify` unchanged.
7. **ze-to-ze interop** - extend `interop_test.go` with a one-to-one local-repair scenario.
8. **Verify + close** - `make ze-verify`; learned summary; two-commit closure.

## Gate Before Implementation

This spec is `ready` but has NOT passed `/ze-spec` research/design or
`/ze-review-spec`. Validate A-1..A-3 and flesh out the Implementation Steps,
add a Review Gate and Pre-Commit Verification section, and complete the
End-to-End User Stories before `/ze-implement`.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated (no unused DETOUR codec)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop test (ze-to-ze one-to-one)

### Completion (BLOCKING)
- [ ] Write learned summary to `plan/learned/NNN-mpls-rsvp-te-one-to-one.md`
- [ ] Commit A (code + spec + learned); Commit B (`git rm` spec)
