# Spec: radius-acct-timewheel

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
3. `internal/component/l2tp/plugins/authradius/acct.go` - per-session interim goroutines (`interimLoop`)

## Task

**Investigation skeleton created from the osvbng comparison refresh (2026-07-10).
The first deliverable is a DECISION, not code.**

Ze schedules RADIUS interim accounting with one goroutine plus one `time.Ticker`
per session, started at session-IP-assigned time. Two scale concerns:

1. **Thundering herd.** After an LNS restart or a LAC reconnect storm, thousands
   of sessions come up within seconds, so their tickers stay phase-aligned
   forever: every interim interval produces a burst of simultaneous RADIUS
   Accounting-Requests (plus `iface.GetStats` reads) instead of a smooth rate.
2. **Per-session goroutine cost.** N sessions = N goroutines + N tickers doing
   nothing between firings.

Investigate whether interim scheduling should move to a time-wheel: sessions
hashed (e.g. FNV-1a of the accounting session ID) into fixed buckets, one bucket
swept per tick, so load spreads deterministically and one goroutine serves all
sessions. Reference: osvbng 34da0d4 uses a 12-bucket 5-second wheel (bucket =
`fnv32a(sessionID) % 12`), giving each session a deterministic, evenly spread,
once-per-minute slot.

Design constraints Ze has that osvbng does not:

- Ze honours a PER-SESSION `Acct-Interim-Interval` override (RFC 2866 Section
  5.18) clamped to [60, 3600]s; a single-period wheel does not directly support
  heterogeneous intervals. The design must either support per-session periods on
  the wheel (next-due tracking per slot) or justify simplification.
- Investigation may conclude the simplest fix wins (e.g. keep per-session tickers
  but add initial jitter to de-phase them). That outcome closes this spec with a
  decision record instead of a rewrite; "no change needed" is also a valid
  outcome if measurement shows the burst is harmless at target scale.

## Required Reading

### Architecture Docs
- [ ] `docs/research/l2tpv2-ze-integration.md` - RADIUS accounting design context.
  → Constraint: accounting failures MUST NOT tear down sessions (RFC 2866).
- [ ] `ai/rules/performance.md` - if a wheel is built, its slot storage should be allocation-conscious.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 2866 (RADIUS accounting) - interim semantics; Section 5.18 override handling must survive any redesign.

**Key insights:**
- The per-session override (clamped at acct.go, clamp at :318-326) is the
  main design constraint distinguishing Ze from the osvbng reference.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-07-10; re-read at design time)
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `onSessionIPAssigned` spawns a goroutine per session (:137-140) running `interimLoop` (:266-287), a plain `time.Ticker` at the session's interval (default 300s, :61; per-session override :113-116). No jitter, no phase spreading. `onSessionDown` cancels the loop (:143-162).
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - counters read per interim via `acctGetStats(sess.pppInterface)` (:216), i.e. one kernel interface-stats read per session per firing; bursts multiply these reads.

**Behavior to preserve:**
- Per-session `Acct-Interim-Interval` override honoured within clamp [60, 3600]s.
- Start/Stop semantics and packet content unchanged (packet content evolution is `plan/spec-radius-subscriber-attributes.md`).
- Interims for a session keep firing at (approximately) the configured interval; spreading may shift phase, not rate.

**Behavior to change:**
- Interim firing times de-phased across sessions; scheduling cost decoupled from session count (exact mechanism = the investigation outcome).

## Data Flow (MANDATORY)

### Entry Point
- `SessionIPAssigned` event registers the session for interim scheduling; `SessionDown` deregisters.

### Transformation Path
1. Session registered with its resolved interval.
2. Scheduler (wheel or jittered ticker) decides the next firing.
3. Firing reads interface stats and sends the interim packet (existing path).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| l2tp events ↔ authradius | existing subscription | [ ] |
| scheduler ↔ send path | existing `sendAcctInterimUpdate` | [ ] |

### Integration Points
- `radiusAcct.sessions` map and `interimLoop` (`acct.go`) - replaced or augmented by the chosen scheduler.

### Architectural Verification
- [ ] No bypassed layers (send path unchanged)
- [ ] No unintended coupling (scheduler local to the plugin)
- [ ] No duplicated functionality (one scheduling mechanism after the change, not two)
- [ ] Registration over hardcoding - no core surface changes

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Interim bursts at target session counts are large enough to matter | osvbng judged it worth fixing; Ze target scale unmeasured | close as "no change needed" | benchmark: N synthetic sessions, measure send/stat-read clustering | unvalidated |
| A-2 | A wheel can honour per-session intervals via per-entry next-due timestamps | standard timing-wheel technique | fall back to jittered per-session tickers | design spike | unvalidated |
| A-3 | Deterministic bucketing (stable session→slot) has operational value (predictable per-session cadence) | osvbng rationale | random jitter is simpler and sufficient | decision at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Spreading shifts an interim past a billing-system staleness window | interop/billing expectations | keep max added phase shift below one interval; document |
| R-2 | Wheel rewrite introduces missed/duplicate interims on session churn | unit test with churn | churn-focused unit tests; keep Stop-path send outside the wheel |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| many sessions established near-simultaneously | → | interim packets spread across the interval, not clustered | `test/plugin/radius-acct-interim-spread.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | investigation complete | decision recorded here (wheel / jitter / no change) with measurement evidence |
| AC-2 | (if change adopted) K sessions brought up in the same second | interim sends spread over the interval within the chosen bound |
| AC-3 | (if change adopted) session with Acct-Interim-Interval override | fires at its own clamped interval |
| AC-4 | (if change adopted) session churn during a sweep | no missed and no duplicate interim for surviving sessions |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | LAC storm reconnects thousands of subscribers | events → scheduler → paced interim stream to RADIUS | `test/plugin/radius-acct-interim-spread.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInterimSpread` | `internal/component/l2tp/plugins/authradius/acct_test.go` | firings de-phased for same-time registrations | |
| `TestInterimPerSessionOverride` | `internal/component/l2tp/plugins/authradius/acct_test.go` | override interval survives the scheduler change | |
| `TestInterimChurn` | `internal/component/l2tp/plugins/authradius/acct_test.go` | register/deregister races lose no session | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| interim interval | 60-3600 s (existing clamp) | 3600 | 59 (clamped up) | 3601 (clamped down) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-acct-interim-spread` | `test/plugin/radius-acct-interim-spread.ci` | mass bring-up produces paced interims | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - scheduling only, wire content unchanged | - | - | covered by functional test | - |

### Future (if deferring any tests)
- None planned (skeleton; refine at design).

## Files to Modify
- `internal/component/l2tp/plugins/authradius/acct.go` - scheduling mechanism (per investigation outcome)

## Files to Create
- `test/plugin/radius-acct-interim-spread.ci` - functional pacing test (if change adopted)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton - run `/ze-spec` RESEARCH/DESIGN first) |

### Implementation Phases
1. **INVESTIGATE (not started)** - measure burst behaviour at target scale (synthetic sessions), compare wheel vs jitter vs status quo, record the decision in this spec. STOP for user review of the decision before any implementation phase.
2. **(conditional) IMPLEMENT** - only if the decision adopts a change; fill phases at design time.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests above are provisional placeholders to be refined during DESIGN.
- Packet content (attributes) is out of scope: `plan/spec-radius-subscriber-attributes.md`.

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
- [ ] Investigation decision recorded with measurement evidence and user sign-off
- [ ] (if change adopted) `make ze-standard-test` passes
- [ ] (if change adopted) feature code integrated (`internal/*`)

### Quality Gates (SHOULD pass)
- [ ] RFC 2866 interim semantics re-checked against the final design

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
