# Spec: ppp-ra-refinements

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Updated | 2026-08-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/l2tp/ppp/ra_linux.go` - RA sender loop + stop path
4. `internal/component/l2tp/ppp/ra.go` - `BuildRA` message builder
5. `plan/spec-router-advertisement.md` - the LAN RA spec (ready) that extracts encoding to `internal/core/ndp`

## Task

**Implemented 2026-08-29. The two refinements below are done; the functional and
interop rows of the test plan are not (see Known Limitations).**

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
- [ ] RFC 4861 (Neighbor Discovery) - Section 6.2.5 (ceasing to advertise), Section 6.2.4 (unsolicited RA intervals vs lifetime). Read from `rfc/full/rfc4861.txt`, which this work adds to the repository.
  -> Decision: no `rfc/short/rfc4861.md` summary and no `rfc/enrolled.txt` entry. Enrolling an RFC gates every MUST it declares and needs its own extraction sign-off, which is separate work (`ai/rules/rfc-compliance.md`, "Extraction Completeness").

**Key insights:**
- Both changes are confined to `ra_linux.go` (send loop + stop path) plus the
  constants; `BuildRA` already accepts an arbitrary `RouterLifetime`, so the
  cease packet needs no encoder change (RAConfig at ra.go).

## Current Behavior (MANDATORY)

**Source files read:** (re-verified 2026-08-29 against the working tree)
- [ ] `internal/component/l2tp/ppp/ra_linux.go` - held `raInitialCount=5`, `raInitialInterval=3s`, `raPeriodicInterval=600s` and `raSenderLoop`, which sent every RA with a hardcoded `RouterLifetime: 1800`. `startRASender` returned a cancel closure that cancelled the context and closed the socket; NO final RA was sent.
- [ ] `internal/component/l2tp/ppp/ra.go` - `BuildRA` takes `RAConfig.RouterLifetime`; a zero value already encodes a valid cease RA, pinned by `TestBuildRAHeader` in `internal/core/ndp/ra_test.go` ("router lifetime zero is not a default router").
- [ ] `internal/component/l2tp/ppp/ipv6_service_linux.go` - `startIPv6Service` wires `startRASender` and puts its cancel closure in `svc.stop`, which `(*IPv6Service).Stop` (`ipv6_service.go`) calls.
- [ ] `internal/component/l2tp/ppp/ncp.go` - `teardownNCPResources` calls `s.ipv6Svc.Stop()` FIRST, and `internal/component/l2tp/ppp/session_run.go`'s `run` defer calls `teardownNCPResources` BEFORE `s.chanFile.Close()`. The pppN interface and the RA socket are therefore both live at cease time on a normal teardown (validates A-1).
- [ ] `rfc/full/rfc4861.txt` - Section 6.2.5 (ceasing to be an advertising interface), Section 6.2.1 (AdvDefaultLifetime defaults to 3 * MaxRtrAdvInterval), Section 10 (MAX_FINAL_RTR_ADVERTISEMENTS is 3, MIN_DELAY_BETWEEN_RAS is 3 s). Added to the repository by this work; RFC 4861 is NOT enrolled and has no `rfc/short/` summary, so `./le rfc check` neither gates nor reports it.

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
| A-1 | The socket is still usable at cancel time for the cease send (teardown ordering) | cancel closure owns the conn and closes it itself (ra_linux.go) | move cease send earlier in teardown | unit test + read teardown callers of the cancel func | confirmed: `run`'s defer (`session_run.go`) calls `teardownNCPResources` before `s.chanFile.Close()`, and that function calls `ipv6Svc.Stop()` first of all |
| A-2 | On abrupt session death the PPP interface may already be gone; best-effort cease is acceptable | kernel removes pppN on channel close | log-and-continue semantics | design review of teardown paths | confirmed: `raSender.send` logs a write failure at Debug and returns nothing, and `TestStopRASenderIsBestEffortWhenTheInterfaceIsGone` drives the failing-send path |
| A-3 | Lifetime/3 remains the right margin | current constants embody it; osvbng uses the same | pick a different divisor | RFC 4861 Section 6.2.4 bounds check at design | confirmed: RFC 4861 Section 6.2.1 gives AdvDefaultLifetime a default of 3 * MaxRtrAdvInterval, so lifetime/3 is the RFC's own ratio, not a coincidence |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Cease RA delays teardown if the send blocks | slow session teardown under churn | accepted without a write deadline: the stop path performs one unconnected datagram write with no retry, and a raw ICMPv6 write to a dead interface returns an error rather than blocking. The wait it added, `<-senderDone`, is bounded because every branch of `raSenderLoop` selects on `ctx.Done()` |
| R-2 | Interval derivation changes cadence and surprises subscribers' RA-based accounting of the route | interop test diff | keep 1800/600 defaults; only the derivation mechanism changes |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| session with IPv6 up is torn down | → | final RA with RouterLifetime 0 observed on the session interface | `test/plugin/ppp-ra-cease.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-1 | RA sender stopped normally | exactly one final RA, RouterLifetime 0, same M/O/RDNSS shape | `TestStopRASenderSendsAZeroLifetimeRABeforeClose` and `TestStopRASenderWaitsForTheSenderGoroutine`. The shape is unchanged because `raSender.send` builds every RA from one `RAConfig` and varies only the lifetime |
| AC-2 | abrupt teardown (interface already gone) | stop completes without error spam; cease is best-effort | `TestStopRASenderIsBestEffortWhenTheInterfaceIsGone` |
| AC-3 | periodic interval | computed from router lifetime (default preserves today's 1800s/600s) | `TestRAPeriodicIntervalDerivesFromTheRouterLifetime` |
| AC-4 | steady state | at least 3 unsolicited RAs per router lifetime window | `TestRAPeriodicIntervalDerivesFromTheRouterLifetime` proves 3 * interval equals the lifetime; `TestRASenderLoopAdvertisesTheRouterLifetime` proves the loop advertises that lifetime |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | subscriber disconnects | teardown → cease RA → host drops default route immediately | `test/plugin/ppp-ra-cease.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildRAHeader` ("router lifetime zero is not a default router") | `internal/core/ndp/ra_test.go` | zero-lifetime RA encoding | already present, green |
| `TestStopRASenderSendsAZeroLifetimeRABeforeClose` | `internal/component/l2tp/ppp/ra_send_test.go` | the stop path sends one RA with Router Lifetime 0, and sends it before the close | green; reddens when `sender.send(raCeaseLifetime)` is removed |
| `TestStopRASenderWaitsForTheSenderGoroutine` | `internal/component/l2tp/ppp/ra_send_test.go` | the final RA cannot be overtaken by a periodic one | green |
| `TestStopRASenderIsBestEffortWhenTheInterfaceIsGone` | `internal/component/l2tp/ppp/ra_send_test.go` | AC-2: a failing send and a failing close still complete the stop path | green |
| `TestRASenderLoopAdvertisesTheRouterLifetime` | `internal/component/l2tp/ppp/ra_send_test.go` | steady-state RAs carry `raRouterLifetime`, and the loop closes `senderDone` | green |
| `TestRAPeriodicIntervalDerivesFromTheRouterLifetime` | `internal/component/l2tp/ppp/ra_send_test.go` | AC-3, AC-4: interval = lifetime/3, so three advertisements fit in one lifetime window | green |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| router lifetime | 0-65535 s (uint16, ra.go) | 65535 | N/A | N/A (type-bounded) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ppp-ra-cease` | `test/plugin/ppp-ra-cease.ci` | teardown withdraws the subscriber default route | NOT written. See Known Limitations |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| l2tp-ppp-ipv6-ra-cease | `test/interop-l2tp/scenarios/` | xl2tpd/pppd peer | real client kernel drops the route on cease RA | NOT written. See Known Limitations |

### Future (if deferring any tests)
- The functional and interop rows above are open, and the spec stays open with them.

## Files to Modify
- `internal/component/l2tp/ppp/ra_linux.go` - builds the `raSender`, starts `raSenderLoop` with its done channel, and returns a cancel closure that calls `stopRASender`. Keeps only the socket setup
- `internal/component/l2tp/ppp/ipv6_service_linux.go` - unchanged: A-1 held, so the stop-path ordering needed no edit

## Files to Create
- `internal/component/l2tp/ppp/ra_send.go` - the advertised lifetimes, the `raSender`, `raSenderLoop`, and `stopRASender`. No build tag, so the ordering is testable off Linux
- `internal/component/l2tp/ppp/ra_send_test.go` - the unit tests above
- `rfc/full/rfc4861.txt` - the source text this spec quotes
- `test/plugin/ppp-ra-cease.ci` - functional test, NOT written

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |

### Implementation Phases
1. **Cease and derivation (done, 2026-08-29)** - teardown ordering confirmed (A-1, A-2), `internal/core/ndp` extraction already landed so `BuildRA` needed no change, cease send and interval derivation implemented with unit tests.
2. **Functional and interop proof (not started)** - `test/plugin/ppp-ra-cease.ci` and an IPv6 L2TP interop scenario. See Known Limitations.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- LAN RA (prefixes, SLAAC) is `plan/spec-router-advertisement.md`; this spec touches only the PPP/BNG sender.
- **The functional and interop rows of the test plan are NOT discharged, so this spec stays open.** `test/plugin/ppp-ra-cease.ci` needs a live L2TP session with IPv6CP open, which no existing `.ci` test establishes: the `test/plugin/l2tp-*.ci` suite exercises show commands only. `test/interop-l2tp/scenarios/` has four scenarios and every one of them is IPv4 PPP, so proving a real client kernel drops its default route needs a new IPv6 scenario with an IPv6CP-capable pppd. Both are their own work package, and `ai/rules/interop-and-goal-validation.md` requires the interop one for a wire-visible change.
- RFC 4861 is not enrolled and has no `rfc/short/` summary, so no gate holds this behaviour to the RFC. Enrolment is separate work.
- `raSenderLoop` does not meet RFC 4861 Section 6.2.6: a solicited RA is sent with no random delay and consecutive multicast RAs are not rate limited. Recorded in `plan/journal/reply-sent-per-request-with-no-rate-limit.md`, raised with the owner, not fixed here.

## Implementation Summary
### What Was Implemented
- `stopRASender` (`internal/component/l2tp/ppp/ra_send.go`) cancels the sender goroutine, waits for it to leave, sends ONE Router Advertisement with a Router Lifetime of zero, and only then closes the socket. RFC 4861 Section 6.2.5 permits up to MAX_FINAL_RTR_ADVERTISEMENTS (3, Section 10); Ze sends one because Section 6.2.6 rate limits consecutive multicast RAs to one every MIN_DELAY_BETWEEN_RAS (3 s), so three would hold teardown for six seconds.
- The wait is what makes the cease advertisement final. Sending it from the cancel closure without waiting leaves a periodic or solicited RA carrying `raRouterLifetime` able to overtake it and restore the subscriber's default route for another 1800 s.
- `raPeriodicInterval` is now `raRouterLifetime * time.Second / 3` rather than an independent 600 s constant. RFC 4861 Section 6.2.1 gives AdvDefaultLifetime a default of 3 * MaxRtrAdvInterval, so the derivation is the RFC's own ratio and the wire values are unchanged (1800 s and 600 s).
- The send path moved from `ra_linux.go` into the untagged `ra_send.go` behind a one-method `raWriter` seam that `*ipv6.PacketConn` already satisfies. `ra_linux.go` keeps the socket setup. Without the seam the ordering above is provable only from a raw ICMPv6 socket, which needs Linux and root, so no developer machine could run the test.

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
- [ ] `./le verify worktree` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)

### Quality Gates (SHOULD pass)
- [ ] RFC 4861 Section 6.2.5 constraint comment above the cease send (`stopRASender` doc comment quotes it)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
