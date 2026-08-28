# Spec: ppp-ra-refinements

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/l2tp/ppp/ra_linux.go` - RA sender loop + stop path
4. `internal/component/l2tp/ppp/ra.go` - `BuildRA` message builder
5. `plan/spec-router-advertisement.md` - the LAN RA spec (ready) that extracts encoding to `internal/core/ndp`

## Task

**Skeleton created from the osvbng comparison refresh (2026-07-10). Full design not started.**

Two small refinements to the PPP/BNG subscriber RA sender (both verified against
current code 2026-07-10):

1. **Cease RA on teardown.** When the RA sender stops (session teardown, IPv6
   service stop), Ze just cancels the goroutine and closes the socket
   (`startRASender` cancel closure, ra_linux.go). RFC 4861 Section 6.2.5
   says a router ceasing to advertise SHOULD send a final RA with Router
   Lifetime 0 so hosts drop the default route immediately instead of holding it
   for up to the remaining lifetime (1800s today). Send one final
   RouterLifetime=0 RA on stop. Reference: osvbng dc2e34d/83b0330 ship the same
   cease behaviour (`ceaseSessionRA`) when a group's IPv6 is disabled.

2. **Derive the periodic interval from RouterLifetime.** Today the invariant
   "refresh well inside the router lifetime" holds only by coincidence of two
   unrelated constants: `raPeriodicInterval = 600s` (ra_linux.go) and
   `RouterLifetime: 1800` hardcoded at the send site (ra_linux.go). If
   either constant changes (or lifetime becomes configurable), nothing keeps
   refresh < lifetime, and a lost RA near expiry silently drops the subscriber's
   default route. Make the periodic interval a function of the lifetime (osvbng
   uses lifetime/3 so one lost RA cannot expire the route; Ze's current 600/1800
   is exactly that ratio) with a single source of truth for both values.

Coordination: `plan/spec-router-advertisement.md` (ready) plans to extract RA
encoding into `internal/core/ndp` with `ppp.BuildRA` delegating byte-identically,
and independently decided on a final zero-lifetime RA for the LAN sender. This
spec applies the same cease discipline to the PPP path. Whichever spec lands
second rebases on the other's encoding location; behaviour here is orthogonal to
the LAN feature.

## Required Reading

### Architecture Docs
- [ ] `docs/research/l2tpv2-ze-integration.md` - RA design context (referenced by ra.go).
  → Constraint: BNG RAs stay prefix-less (M+O direct subscribers to DHCPv6); the cease RA must keep that shape, only lifetime changes.
- [ ] `plan/spec-router-advertisement.md` - encoding extraction + LAN cease decision.
  → Constraint: do not fork the RA builder; reuse whatever `BuildRA`/`internal/core/ndp` state exists when this is picked up.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 4861 (Neighbor Discovery) - Section 6.2.5 (ceasing to advertise), Section 6.2.4 (unsolicited RA intervals vs lifetime). Summary `rfc/short/rfc4861.md` did not exist as of 2026-07-10 (per spec-router-advertisement research); create via `/ze-rfc` at design time if still missing.

**Key insights:**
- Both changes are confined to `ra_linux.go` (send loop + stop path) plus the
  constants; `BuildRA` already accepts an arbitrary `RouterLifetime`, so the
  cease packet needs no encoder change (RAConfig at ra.go).

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-07-10; re-read at design time)
- [ ] `internal/component/l2tp/ppp/ra_linux.go` - constants `raInitialCount=5`, `raInitialInterval=3s`, `raPeriodicInterval=600s` (:18-22). `raSenderLoop` (:108-146) sends RAs with hardcoded `RouterLifetime: 1800` (:115); initial burst then 600s ticker plus RS-triggered sends. `startRASender` returns a cancel closure (:79-84) that cancels the context and closes the socket; NO final RA is sent.
- [ ] `internal/component/l2tp/ppp/ra.go` - `BuildRA` (:37) takes `RAConfig.RouterLifetime` (ra.go); a zero value encodes a valid cease RA already.
- [ ] `internal/component/l2tp/ppp/ipv6_service_linux.go` - wires `startRASender` after IPv6CP; the stop path to extend (re-read at design).

**Behavior to preserve:**
- Prefix-less RA shape (M+O flags, RDNSS, no PIO).
- Initial burst + RS-solicited behaviour unchanged.
- Current effective cadence (refresh at lifetime/3 = 600s for lifetime 1800s) unless design decides otherwise.

**Behavior to change:**
- Stop path sends one final RouterLifetime=0 RA before closing the socket.
- Periodic interval computed from the router lifetime (single source of truth), not an independent constant.

## Data Flow (MANDATORY)

### Entry Point
- Session IPv6 service stop / session teardown reaches the RA sender's cancel path.

### Transformation Path
1. Stop requested → sender sends `BuildRA` with RouterLifetime 0 to ff02::1.
2. Context cancelled, socket closed (existing path).
3. At start, the periodic interval is derived from the configured/constant lifetime.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| session lifecycle ↔ RA sender | existing cancel closure gains a pre-close send | [ ] |
| wire | one additional RA per teardown | [ ] |

### Integration Points
- `startRASender` cancel closure (ra_linux.go) - cease send before close.
- `raSenderLoop` (ra_linux.go) - interval derivation.

### Architectural Verification
- [ ] No bypassed layers (cease uses the same builder + socket)
- [ ] No unintended coupling (change local to the ppp package)
- [ ] No duplicated functionality (one RA builder; coordinate with spec-router-advertisement extraction)
- [ ] Registration over hardcoding - N/A (no new registration surface)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The socket is still usable at cancel time for the cease send (teardown ordering) | cancel closure owns the conn and closes it itself (ra_linux.go) | move cease send earlier in teardown | unit test + read teardown callers of the cancel func | unvalidated |
| A-2 | On abrupt session death the PPP interface may already be gone; best-effort cease is acceptable | kernel removes pppN on channel close | log-and-continue semantics | design review of teardown paths | unvalidated |
| A-3 | Lifetime/3 remains the right margin | current constants embody it; osvbng uses the same | pick a different divisor | RFC 4861 Section 6.2.4 bounds check at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Cease RA delays teardown if the send blocks | slow session teardown under churn | non-blocking/best-effort send with short deadline |
| R-2 | Interval derivation changes cadence and surprises subscribers' RA-based accounting of the route | interop test diff | keep 1800/600 defaults; only the derivation mechanism changes |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| session with IPv6 up is torn down | → | final RA with RouterLifetime 0 observed on the session interface | `test/plugin/ppp-ra-cease.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RA sender stopped normally | exactly one final RA, RouterLifetime 0, same M/O/RDNSS shape |
| AC-2 | abrupt teardown (interface already gone) | stop completes without error spam; cease is best-effort |
| AC-3 | periodic interval | computed from router lifetime (default preserves today's 1800s/600s) |
| AC-4 | steady state | at least 3 unsolicited RAs per router lifetime window |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | subscriber disconnects | teardown → cease RA → host drops default route immediately | `test/plugin/ppp-ra-cease.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildRAZeroLifetime` | `internal/component/l2tp/ppp/ra_test.go` | zero-lifetime RA encoding | |
| `TestRASenderCeaseOnStop` | `internal/component/l2tp/ppp/` | stop path emits the cease RA before close | |
| `TestRAIntervalDerivation` | `internal/component/l2tp/ppp/` | interval = f(lifetime); defaults unchanged | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| router lifetime | 0-65535 s (uint16, ra.go) | 65535 | N/A | N/A (type-bounded) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ppp-ra-cease` | `test/plugin/ppp-ra-cease.ci` | teardown withdraws the subscriber default route | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| extend l2tp-interop teardown scenario | `test/interop-l2tp/` | xl2tpd/pppd peer | real client kernel drops the route on cease RA | |

### Future (if deferring any tests)
- None planned (skeleton; refine at design).

## Files to Modify
- `internal/component/l2tp/ppp/ra_linux.go` - cease send in the stop path; interval derivation
- `internal/component/l2tp/ppp/ipv6_service_linux.go` - stop-path ordering if A-1 breaks

## Files to Create
- `test/plugin/ppp-ra-cease.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton - run `/ze-spec` RESEARCH/DESIGN first) |

### Implementation Phases
1. **RESEARCH/DESIGN (not started)** - run the `/ze-spec` workflow: confirm teardown ordering (A-1/A-2), check spec-router-advertisement's encoding-extraction state and rebase accordingly, then fill ACs/tests above.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests above are provisional placeholders to be refined during DESIGN.
- LAN RA (prefixes, SLAAC) is `plan/spec-router-advertisement.md`; this spec touches only the PPP/BNG sender.

## Implementation Summary
### What Was Implemented
- Nothing yet (skeleton).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Full `/ze-spec` DESIGN completed and approved before implementation
- [ ] `./le verify current mode full` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)

### Quality Gates (SHOULD pass)
- [ ] RFC 4861 Section 6.2.5 constraint comment above the cease send

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
