# Spec: ppp-ra-refinements

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/3 |
| Updated | 2026-08-30 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/l2tp/ppp/ra_linux.go` - RA sender loop + stop path
4. `internal/component/l2tp/ppp/ra.go` - `BuildRA` message builder
5. `plan/spec-router-advertisement.md` - the LAN RA spec (ready) that extracts encoding to `internal/core/ndp`

## Task

**Phase 1 implemented 2026-08-29: the two refinements below. Phase 2 implemented
2026-08-29: the RFC 4861 Section 6.2.4 and Section 6.2.6 send schedule (AC-5 to
AC-9). The functional and interop rows of the test plan are not done (see Known
Limitations).**

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

Phase 2 adds the send schedule those two refinements were built on top of, after
`plan/journal/reply-sent-per-request-with-no-rate-limit.md` recorded that
`raSenderLoop` met neither MUST of RFC 4861 Section 6.2.6:

3. **Delay and rate limit every multicast advertisement.** Section 6.2.6:
   "Router Advertisements sent in response to a Router Solicitation MUST be
   delayed by a random time between 0 and MAX_RA_DELAY_TIME seconds. (If a
   single advertisement is sent in response to multiple solicitations, the delay
   is relative to the first solicitation.) In addition, consecutive Router
   Advertisements sent to the all-nodes multicast address MUST be rate limited
   to no more than one advertisement every MIN_DELAY_BETWEEN_RAS seconds."
   Section 10 gives MAX_RA_DELAY_TIME 0.5 s and MIN_DELAY_BETWEEN_RAS 3 s.
4. **Randomize the unsolicited interval.** Section 6.2.4: the interval timer is
   reset to a uniformly distributed random value between MinRtrAdvInterval and
   MaxRtrAdvInterval after every multicast advertisement, and the first
   MAX_INITIAL_RTR_ADVERTISEMENTS are capped at MAX_INITIAL_RTR_ADVERT_INTERVAL.

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
| session with IPv6 up is torn down | → | final RA with RouterLifetime 0 observed on the session interface | `stopRASender` is unreachable in the shipped daemon, so no entry point exists to drive. Proven at the seam instead by `TestStopRASenderSendsAZeroLifetimeRABeforeClose` and `TestStopRASenderWaitsForTheSenderGoroutine`. The `.ci` is deferred with the feature to `plan/future/spec-l2tp-ipv6-subscriber.md` (owner ruling, 2026-08-30) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-1 | RA sender stopped normally | exactly one final RA, RouterLifetime 0, same M/O/RDNSS shape | `TestStopRASenderSendsAZeroLifetimeRABeforeClose` and `TestStopRASenderWaitsForTheSenderGoroutine`. The shape is unchanged because `raSender.send` builds every RA from one `RAConfig` and varies only the lifetime |
| AC-2 | abrupt teardown (interface already gone) | stop completes without error spam; cease is best-effort | `TestStopRASenderIsBestEffortWhenTheInterfaceIsGone` |
| AC-3 | periodic interval | computed from router lifetime (default preserves today's 1800s/600s) | `TestRAScheduleIntervalsFitTheRouterLifetime` (`internal/component/l2tp/ppp/ra_schedule_test.go`), which proves 3 * raMaxRtrAdvInterval equals the lifetime. The cell named `TestRAPeriodicIntervalDerivesFromTheRouterLifetime` until 2026-08-30; no such symbol was ever written |
| AC-4 | steady state | at least 3 unsolicited RAs per router lifetime window | `TestRAScheduleIntervalsFitTheRouterLifetime` proves 3 * raMaxRtrAdvInterval equals the lifetime; `TestRASenderLoopAdvertisesTheRouterLifetime` proves the loop advertises that lifetime |
| AC-5 | a Router Solicitation arrives outside the rate-limit window | the answer is delayed by a random time in [0, MAX_RA_DELAY_TIME], and the delay varies between draws | `TestRAScheduleDelaysASolicitedAdvertisement` (200 draws), `TestRASenderLoopAnswersASolicitationWithinMaxRADelayTime` |
| AC-6 | a burst of Router Solicitations coalesces onto `rsCh` | ONE advertisement leaves, and its delay is measured from the FIRST solicitation | `TestRAScheduleTakesTheDelayFromTheFirstSolicitation` (a burst and a single solicitation reach the same send time over 50 seeds), `TestRASenderLoopAnswersABurstOfSolicitationsOnce` |
| AC-7 | a Router Solicitation arrives anywhere inside MIN_DELAY_BETWEEN_RAS of the previous multicast advertisement | the answer is scheduled at MIN_DELAY_BETWEEN_RAS plus the random delay after that advertisement, so two consecutive multicast advertisements are never closer than 3 s | `TestRAScheduleRateLimitsConsecutiveAdvertisements` (every millisecond offset in the window), `TestRASenderLoopRateLimitsAnAnswerToASolicitation` |
| AC-8 | steady-state unsolicited advertisements | each interval is drawn uniformly from [raMinRtrAdvInterval, raMaxRtrAdvInterval] rather than a fixed ticker, and the first MAX_INITIAL_RTR_ADVERTISEMENTS are capped at MAX_INITIAL_RTR_ADVERT_INTERVAL | `TestRAScheduleRandomizesTheUnsolicitedInterval`, `TestRAScheduleCapsTheInitialAdvertisements` |
| AC-9 | teardown within MIN_DELAY_BETWEEN_RAS of an advertisement | the final zero-lifetime advertisement waits out the remainder of the window, and waits for nothing once the window has closed | `TestRAScheduleCeaseWait`, `TestStopRASenderWaitsOutTheRateLimit` |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | subscriber disconnects | teardown → cease RA → host drops default route immediately | Not provable end to end: the shipped daemon never opens IPv6CP, so no subscriber ever reaches this story. Deferred with the feature to `plan/future/spec-l2tp-ipv6-subscriber.md` (owner ruling, 2026-08-30). The teardown half is proven at the seam by `TestStopRASenderSendsAZeroLifetimeRABeforeClose` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildRAHeader` ("router lifetime zero is not a default router") | `internal/core/ndp/ra_test.go` | zero-lifetime RA encoding | already present, green |
| `TestStopRASenderSendsAZeroLifetimeRABeforeClose` | `internal/component/l2tp/ppp/ra_send_test.go` | the stop path sends one RA with Router Lifetime 0, and sends it before the close | green; reddens when `sender.send(raCeaseLifetime)` is removed |
| `TestStopRASenderWaitsForTheSenderGoroutine` | `internal/component/l2tp/ppp/ra_send_test.go` | the final RA cannot be overtaken by a periodic one | green |
| `TestStopRASenderIsBestEffortWhenTheInterfaceIsGone` | `internal/component/l2tp/ppp/ra_send_test.go` | AC-2: a failing send and a failing close still complete the stop path | green |
| `TestRASenderLoopAdvertisesTheRouterLifetime` | `internal/component/l2tp/ppp/ra_send_test.go` | steady-state RAs carry `raRouterLifetime`, and the loop closes `senderDone` | green |
| `TestRAScheduleIntervalsFitTheRouterLifetime` | `internal/component/l2tp/ppp/ra_schedule_test.go` | AC-3, AC-4: raMaxRtrAdvInterval = lifetime/3, and all three values sit inside the RFC 4861 Section 6.2.1 bounds | green |
| `TestRAScheduleDelaysASolicitedAdvertisement` | `internal/component/l2tp/ppp/ra_schedule_test.go` | AC-5 | green; reddens when the random delay is replaced by zero |
| `TestRAScheduleTakesTheDelayFromTheFirstSolicitation` | `internal/component/l2tp/ppp/ra_schedule_test.go` | AC-6 | green; reddens when the `solicited` guard is removed |
| `TestRAScheduleRateLimitsConsecutiveAdvertisements` | `internal/component/l2tp/ppp/ra_schedule_test.go` | AC-7 | green; reddens when the MIN_DELAY_BETWEEN_RAS branch is removed |
| `TestRAScheduleRandomizesTheUnsolicitedInterval` | `internal/component/l2tp/ppp/ra_schedule_test.go` | AC-8 | green; reddens when the interval becomes fixed |
| `TestRAScheduleCapsTheInitialAdvertisements` | `internal/component/l2tp/ppp/ra_schedule_test.go` | AC-8 | green |
| `TestRAScheduleCeaseWait` | `internal/component/l2tp/ppp/ra_schedule_test.go` | AC-9 | green |
| `TestRAScheduleWaitIsNeverNegative` | `internal/component/l2tp/ppp/ra_schedule_test.go` | an overdue schedule asks for no wait, never a negative one | green |
| `TestStopRASenderWaitsOutTheRateLimit` | `internal/component/l2tp/ppp/ra_send_test.go` | AC-9: the stop path sleeps the remainder of the window, and nothing once it closed | green; reddens when the sleep is removed |
| `TestRASenderLoopRateLimitsAnAnswerToASolicitation` | `internal/component/l2tp/ppp/ra_send_test.go` | AC-7 through the real loop on a fake clock | green; reddens when the rate limit is removed and when the loop answers each solicitation at once |
| `TestRASenderLoopAnswersASolicitationWithinMaxRADelayTime` | `internal/component/l2tp/ppp/ra_send_test.go` | AC-5 through the real loop | green; reddens when the loop stops consulting the schedule |
| `TestRASenderLoopAnswersABurstOfSolicitationsOnce` | `internal/component/l2tp/ppp/ra_send_test.go` | AC-6 through the real loop | green; reddens when the loop answers each solicitation at once |
| `TestRAIntervalBounds`, `TestRASolicitedDelayBounds`, `TestRARateLimit` | `internal/core/ndp/schedule_test.go` | the shared arithmetic, moved with it out of `internal/plugins/iface/ra` | green |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| router lifetime | 0-65535 s (uint16, ra.go) | 65535 | N/A | N/A (type-bounded) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ppp-ra-cease` | `test/plugin/ppp-ra-cease.ci` | teardown withdraws the subscriber default route | DEFERRED to `plan/future/spec-l2tp-ipv6-subscriber.md`. The shipped daemon cannot reach IPv6CP Opened, so it sends no Router Advertisement for a test to observe. See Known Limitations |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| l2tp-ppp-ipv6-ra-cease | `test/interop-l2tp/scenarios/` | xl2tpd/pppd peer | real client kernel drops the route on cease RA | DEFERRED to `plan/future/spec-l2tp-ipv6-subscriber.md`. The same daemon blocker applies, and the L2TP lab needs PPPoL2TP in the Docker host kernel. See Known Limitations |

### Future (if deferring any tests)
- Both rows above are deferred WITH the feature they depend on, to
  `plan/future/spec-l2tp-ipv6-subscriber.md`. That spec's "What this spec owes
  when it runs" carries them by name. This spec closes on its nine acceptance
  criteria (owner ruling, 2026-08-30: the first release ships L2TP and PPPoE
  subscribers IPv4-only).

## Files to Modify
- `internal/component/l2tp/ppp/ra_linux.go` - builds the `raSender` and the `raSchedule`, starts `raSenderLoop` with its done channel, and returns a cancel closure that calls `stopRASender`. Keeps only the socket setup
- `internal/component/l2tp/ppp/ipv6_service_linux.go` - unchanged: A-1 held, so the stop-path ordering needed no edit
- `internal/component/l2tp/ppp/ra_send.go` - phase 2 replaced the initial burst and the fixed ticker with one loop that asks `raSchedule` how long to wait
- `internal/plugins/iface/ra/ifacera.go`, `sender_linux.go` - phase 2 moved the timing arithmetic to `internal/core/ndp` and added the first-solicitation guard the same RFC sentence requires
- `docs/features/interfaces.md` - the source pointers follow the arithmetic to `internal/core/ndp`

## Files to Create
- `internal/component/l2tp/ppp/ra_send.go` - the `raSender`, `raSenderLoop`, and `stopRASender`. No build tag, so the ordering is testable off Linux
- `internal/component/l2tp/ppp/ra_send_test.go` - the unit tests above
- `internal/component/l2tp/ppp/ra_schedule.go` - the advertised lifetimes and `raSchedule`, the RFC 4861 Sections 6.2.4 and 6.2.6 send schedule on an injected `clock.Clock`
- `internal/component/l2tp/ppp/ra_schedule_test.go` - the schedule unit tests
- `internal/core/ndp/schedule.go`, `schedule_test.go` - the RFC 4861 Section 10 router constants and the three timing functions, shared by the PPP sender and the LAN sender
- `rfc/full/rfc4861.txt` - the source text this spec quotes
- `test/plugin/ppp-ra-cease.ci` - functional test, NOT written and NOT writable here. Deferred with the IPv6 subscriber feature to `plan/future/spec-l2tp-ipv6-subscriber.md` (owner ruling, 2026-08-30)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |

### Implementation Phases
1. **Cease and derivation (done, 2026-08-29)** - teardown ordering confirmed (A-1, A-2), `internal/core/ndp` extraction already landed so `BuildRA` needed no change, cease send and interval derivation implemented with unit tests.
2. **Send schedule (done, 2026-08-29)** - RFC 4861 Section 6.2.6's random solicited delay, first-solicitation rule and MIN_DELAY_BETWEEN_RAS rate limit, and Section 6.2.4's randomized interval with the initial-advertisement cap. The arithmetic moved to `internal/core/ndp` rather than being written a second time, and the LAN sender in `internal/plugins/iface/ra` now calls it and gained the first-solicitation guard it was also missing.
3. **Functional and interop proof (blocked, 2026-08-30)** - `test/plugin/ppp-ra-cease.ci` and an IPv6 L2TP interop scenario. Neither can be written: the shipped daemon never starts the RA sender, because no pool handler accepts an IPv6 address request. See Known Limitations.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A PPP session can reach IPv6CP Opened in the shipped daemon, so a functional or an interop test can observe the cease Router Advertisement | `poolPlugin.handle` (`internal/component/l2tp/plugins/pool/register.go`, the `req.Family != ppp.AddressFamilyIPv4` guard) answers every IPv6 address request with `Accept: false`. It is the only `l2tp.RegisterPoolHandler` caller in the tree, so no build answers IPv6 differently. `runNCPPhase` (`internal/component/l2tp/ppp/ncp.go`) reads that decline as `declined`, sets `s.disableIPv6CP`, and leaves `s.ipv6cpState` at Initial, so the `s.ipv6cpState == LCPStateOpened` guard in `afterLCPOpen` (`internal/component/l2tp/ppp/session_run.go`) is never true | phase 3 traced the caller chain backward from `startRASender` while writing the functional test | `startRASender` and `stopRASender` have no reachable production caller. AC-1, AC-2 and AC-9 stay proven by unit tests alone, and the two open test rows cannot be written until the daemon reaches IPv6CP Opened |
| Moving a symbol out of a package updates the docs that describe the behaviour, and that is the whole documentation obligation | A `<!-- source: -->` anchor names a FILE and a SYMBOL, so a symbol that moves invalidates every anchor pointing at its old home even when the prose beside it is still true. `docs/features/interfaces.md` was corrected during implementation; the `IPv6 Router Advertisements` row of `docs/features.md` still named `unsolicitedInterval` in `internal/plugins/iface/ra/ifacera.go` | `./le doc check verify` at closure, reporting 37 stale anchors of which exactly one was this work's | One stale anchor shipped for a day. The lesson is to grep the OLD symbol name across `docs/` when a symbol moves, rather than only re-reading the page the change was about |

## Known Limitations
- LAN RA (prefixes, SLAAC) is `plan/spec-router-advertisement.md`; this spec touches only the PPP/BNG sender.
- **Owner ruling, 2026-08-30: the first release ships L2TP and PPPoE subscribers IPv4-only, so the two blocked rows are deferred with the feature they depend on.** They move to `plan/future/spec-l2tp-ipv6-subscriber.md`, which owes both when it runs. This spec closes on its nine acceptance criteria, every one implemented and mutation-proven; its two test rows are unreachable by design rather than unwritten by omission. The question put to the owner was whether the first release carries L2TP IPv6 subscribers at all, since wiring the path is a feature slice and not a fix.
- **The functional and interop rows of the test plan were BLOCKED before that ruling.** The 2026-08-29 text said only that no existing `.ci` test establishes a session with IPv6CP open. That understated the obstacle. No test can, because the shipped daemon never opens IPv6CP:
  - `poolPlugin.handle` (`internal/component/l2tp/plugins/pool/register.go`) answers every address request whose family is not IPv4 with `Accept: false` and the reason "IPv6 not supported by static pool". `l2tp.RegisterPoolHandler` has exactly one caller, that same file, so no configuration and no plugin changes the answer.
  - `runNCPPhase` (`internal/component/l2tp/ppp/ncp.go`) reads the decline, sets `s.disableIPv6CP`, and leaves `s.ipv6cpState` at Initial. `afterLCPOpen` (`internal/component/l2tp/ppp/session_run.go`) calls `startIPv6Service` only under `s.ipv6cpState == LCPStateOpened`, so `startRASender` never runs and no Router Advertisement, initial or final, reaches the wire.
  - `plan/journal/silent-fall-through.md` (row 2026-08-13) records the same path observed from outside: pppd 2.5.1 retransmitted its IPv6CP Configure-Request nine times and gave up, because Ze had already refused IPv6.
  - The second half of the same feature is disconnected too, and `plan/spec-radius-subscriber-attributes.md` (limitation L-1) records it: `session_run.go` passes a nil prefix allocator to `startIPv6Service`, and `l2tp.GetPrefixHandler` has no caller, so the DHCPv6-PD server the Managed and Other flags direct the subscriber to cannot delegate a prefix.
  - The repair is therefore a feature slice rather than a test: accept an IPv6 address request when the operator configured an IPv6 pool, and wire the prefix handler and releaser into `startIPv6Service`. It needs its own acceptance criteria, its own tests and its own interop scenario, so it belongs in its own spec. This spec's two open rows follow it.
- **A second obstacle sits under the interop row alone, and it is about the machine rather than the code.** `./le deployment docker-l2tp-ppp-test` runs a preflight probe that requires `/dev/ppp`, the `l2tp_ppp` or `pppol2tp` module, and `ip l2tp`. On the 2026-08-30 development machine the Docker host is a colima Linux VM whose module tree carries no L2TP module at all, so the probe reports "host kernel missing PPPoL2TP requirements" and all four existing scenarios refuse to start. A new scenario cannot be proven red under mutation there. The `.ci` row has no such obstacle: `option=needs-linux:caps=net-admin` runs in the QEMU guest, which loads `ppp_generic`, `l2tp_ppp` and `l2tp_netlink` at boot (`internal/le/qemu/run.go`), and `test/l2tp/radius-acct-wire.ci` already reaches a real kernel PPP session with IPCP negotiated there.
- RFC 4861 is not enrolled and has no `rfc/short/` summary, so no gate holds this behaviour to the RFC. Enrolment is separate work.
- The initial burst changed shape. It was five advertisements three seconds apart, chosen before MinRtrAdvInterval existed in this code. Section 6.2.6 states that "unsolicited multicast advertisements MUST NOT be sent more frequently than indicated by MinRtrAdvInterval", and Section 6.2.4 carves out only the first MAX_INITIAL_RTR_ADVERTISEMENTS, at MAX_INITIAL_RTR_ADVERT_INTERVAL. Defining MinRtrAdvInterval at 200 s made advertisements four and five of the old burst a violation, so the burst is now the RFC's own: three advertisements, each wait capped at 16 s. A subscriber that misses the first one waits 16 s rather than 3 s, or solicits and is answered inside 3.5 s.

## Implementation Summary
### What Was Implemented
- `stopRASender` (`internal/component/l2tp/ppp/ra_send.go`) cancels the sender goroutine, waits for it to leave, sends ONE Router Advertisement with a Router Lifetime of zero, and only then closes the socket. RFC 4861 Section 6.2.5 permits up to MAX_FINAL_RTR_ADVERTISEMENTS (3, Section 10); Ze sends one because Section 6.2.6 rate limits consecutive multicast RAs to one every MIN_DELAY_BETWEEN_RAS (3 s), so three would hold teardown for six seconds.
- The wait is what makes the cease advertisement final. Sending it from the cancel closure without waiting leaves a periodic or solicited RA carrying `raRouterLifetime` able to overtake it and restore the subscriber's default route for another 1800 s.
- `raMaxRtrAdvInterval` is now `raRouterLifetime * time.Second / 3` rather than an independent 600 s constant, and `raMinRtrAdvInterval` is one third of that (200 s). RFC 4861 Section 6.2.1 gives AdvDefaultLifetime a default of 3 * MaxRtrAdvInterval and MinRtrAdvInterval a default of 0.33 * MaxRtrAdvInterval, so both derivations are the RFC's own ratios.
- `raSchedule` (`internal/component/l2tp/ppp/ra_schedule.go`) holds the whole Section 6.2.6 algorithm in four methods on an injected `clock.Clock`: `wait` says how long the sender sleeps, `solicit` applies the three-step algorithm to an arriving Router Solicitation, `advertised` records a send and draws the next randomized interval, and `ceaseWait` says what the final advertisement owes the rate limit. Holding it apart from the loop is what lets every RFC bound be asserted on a fake clock in microseconds.
- The final teardown advertisement obeys the rate limit. Section 6.2.6's rate limit covers "consecutive Router Advertisements sent to the all-nodes multicast address" with no exception for a final one, and Section 6.2.5 contemplates the cost by permitting three of them. `stopRASender` therefore sleeps `sched.ceaseWait()`, which is zero in steady state (advertisements are at least 200 s apart) and at most 3 s when a session is torn down straight after it advertised. The sleep runs on the session's own goroutine (`run`'s defer in `session_run.go`), so it delays no other session.
- The timing arithmetic that both senders share now lives in `internal/core/ndp/schedule.go`. Writing it a second time in the PPP package would have left two copies of one RFC section, and the copy in `internal/plugins/iface/ra` was already missing the parenthetical of Section 6.2.6.
- The send path moved from `ra_linux.go` into the untagged `ra_send.go` behind a one-method `raWriter` seam that `*ipv6.PacketConn` already satisfies. `ra_linux.go` keeps the socket setup. Without the seam the ordering above is provable only from a raw ICMPv6 socket, which needs Linux and root, so no developer machine could run the test.

### Bugs Found/Fixed
- The LAN Router Advertisement sender in `internal/plugins/iface/ra` answered
  every Router Solicitation of a burst with its own random delay, because it
  carried its own copy of the RFC 4861 Section 6.2.6 arithmetic and that copy
  omitted the parenthetical "(If a single advertisement is sent in response to
  multiple solicitations, the delay is relative to the first solicitation.)".
  Moving the arithmetic to `internal/core/ndp/schedule.go` and giving that
  sender the first-solicitation guard fixed it. Covered by
  `TestRASolicitedDelayBounds` and `TestRARateLimit`
  (`internal/core/ndp/schedule_test.go`) plus the LAN sender's own tests.
- `TestStopRASenderWaitsOutTheRateLimit` asserted only the duration handed to
  `Sleep`, so it could not tell a compliant teardown from one that sends the
  final advertisement before waiting. Repaired in `7805d5162`: `sleepClock`
  now advances the fake clock, and the test asserts the gap between the last
  periodic advertisement and the final one.

### Documentation Updates
- `docs/features/interfaces.md`: the two source anchors under "Prefix list" and
  the RA sender section follow the arithmetic to
  `internal/core/ndp/schedule.go` (`UnsolicitedInterval`,
  `MaxInitialAdvertisements`, `SolicitedDelay`, `SolicitedSendTime`,
  `MinDelayBetweenRAs`), and the prose now states that a burst of solicitations
  draws one answer timed from the first of them.
- `docs/features.md`: the "IPv6 Router Advertisements" row's anchor named
  `unsolicitedInterval` in `internal/plugins/iface/ra/ifacera.go`, where the
  symbol no longer lives. Split into a `SetMetricsRegistry` anchor and an
  `internal/core/ndp/schedule.go -- UnsolicitedInterval` anchor. Found by
  `./le doc check verify` during closure, not by the implementation phase; a
  Mistake Log row records that.
- `docs/features/rfc-status.md`: no row is owed. RFC 4861 is not in
  `rfc/enrolled.txt` and has no `rfc/short/` summary, and
  `check_status_completeness` requires a row only for an enrolled RFC.
- `docs/features.md` also carried `RaisePrefixStale` where the tree declares
  `raisePrefixStale` (`3c9644e15` unexported it). Not this spec's defect, but the
  file was open for the anchor above, so it was repaired on touch
  (`internal/le/doc/check/links.go`, `citationExcludePrefixes`: "repair a stale
  path in a file you are already editing for another reason").
- `./le doc check verify` went 37 -> 35 stale source anchors across the two
  repairs, and `docs/features.md` now has none. The 35 that remain name other
  work's packages in this shared checkout.
- **`docs/features.md` is NOT in either closure commit.** `./le commit create`
  refuses it: `structuralGateReds` (`internal/le/commit/verification.go`) reads
  `tmp/ze-verify-failures.json`, the 2026-08-29 verify snapshot, whose `doc
  wiring` group still names `docs/features.md` among its related paths. Only a
  full verify rewrites that snapshot, and `./le verify worktree` judges HEAD,
  which does not carry the repair. The file is clean and its two hunks wait in
  the working tree for an owner-authorised `structural-red-ok`.

### Deviations from Plan
- The spec planned to change `ra_linux.go` in place. The send path moved into an
  untagged `ra_send.go` instead, behind the one-method `raWriter` seam, because
  the ordering guarantee is otherwise provable only from a raw ICMPv6 socket
  that needs Linux and root.
- The initial burst changed from five advertisements three seconds apart to the
  RFC's own three, each wait capped at MAX_INITIAL_RTR_ADVERT_INTERVAL. Defining
  MinRtrAdvInterval at 200 s made the fourth and fifth of the old burst a
  Section 6.2.6 violation.
- The two test rows of the plan are deferred with the feature they depend on
  (owner ruling, 2026-08-30), not written here.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Cease RA on teardown | Done | `stopRASender` (`internal/component/l2tp/ppp/ra_send.go`) | One advertisement with Router Lifetime 0, after the sender goroutine has left and after the rate-limit window |
| 2. Derive the periodic interval from RouterLifetime | Done | `raMaxRtrAdvInterval`, `raMinRtrAdvInterval` (`internal/component/l2tp/ppp/ra_schedule.go`) | `raRouterLifetime * time.Second / 3`, and one third of that. One source of truth |
| 3. Delay and rate limit every multicast advertisement | Done | `raSchedule.solicit` (`ra_schedule.go`), `ndp.SolicitedDelay`, `ndp.SolicitedSendTime` (`internal/core/ndp/schedule.go`) | The Section 6.2.6 three-step algorithm, in the section's own order |
| 4. Randomize the unsolicited interval | Done | `raSchedule.advertised`, `ndp.UnsolicitedInterval` | Uniform in [MinRtrAdvInterval, MaxRtrAdvInterval], capped at MAX_INITIAL_RTR_ADVERT_INTERVAL for the first three |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestStopRASenderSendsAZeroLifetimeRABeforeClose`, `TestStopRASenderWaitsForTheSenderGoroutine` | Order and lifetime both asserted through `raRecorder` |
| AC-2 | Done | `TestStopRASenderIsBestEffortWhenTheInterfaceIsGone` | A failing send and a failing close still complete the stop path |
| AC-3 | Done | `TestRAScheduleIntervalsFitTheRouterLifetime` | 3 * raMaxRtrAdvInterval equals raRouterLifetime |
| AC-4 | Done | `TestRAScheduleIntervalsFitTheRouterLifetime`, `TestRASenderLoopAdvertisesTheRouterLifetime` | At least three advertisements per lifetime window |
| AC-5 | Done | `TestRAScheduleDelaysASolicitedAdvertisement`, `TestRASenderLoopAnswersASolicitationWithinMaxRADelayTime` | 200 draws, all inside [0, MAX_RA_DELAY_TIME], and not all equal |
| AC-6 | Done | `TestRAScheduleTakesTheDelayFromTheFirstSolicitation`, `TestRASenderLoopAnswersABurstOfSolicitationsOnce` | A burst and a single solicitation reach the same send time over 50 seeds |
| AC-7 | Done | `TestRAScheduleRateLimitsConsecutiveAdvertisements`, `TestRASenderLoopRateLimitsAnAnswerToASolicitation` | Every millisecond offset in the window |
| AC-8 | Done | `TestRAScheduleRandomizesTheUnsolicitedInterval`, `TestRAScheduleCapsTheInitialAdvertisements` | Randomized interval and the initial cap |
| AC-9 | Done | `TestRAScheduleCeaseWait`, `TestStopRASenderWaitsOutTheRateLimit` | The second asserts the gap on the clock, so the order of sleep and send is observable |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The 17 unit tests of the TDD plan | Done | `internal/component/l2tp/ppp/ra_send_test.go`, `ra_schedule_test.go`, `internal/core/ndp/schedule_test.go` | Every symbol the plan names exists; `grep '^func Test'` over the three files returns all of them |
| `test/plugin/ppp-ra-cease.ci` | Deferred | `plan/future/spec-l2tp-ipv6-subscriber.md` | Unreachable: `poolPlugin.handle` refuses every non-IPv4 family, so IPv6CP never opens and no advertisement reaches the wire. Owner ruling, 2026-08-30 |
| `l2tp-ppp-ipv6-ra-cease` interop scenario | Deferred | `plan/future/spec-l2tp-ipv6-subscriber.md` | Same blocker, plus a Docker host kernel with no L2TP module |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/l2tp/ppp/ra_send.go` | Done | Created |
| `internal/component/l2tp/ppp/ra_send_test.go` | Done | Created |
| `internal/component/l2tp/ppp/ra_schedule.go` | Done | Created |
| `internal/component/l2tp/ppp/ra_schedule_test.go` | Done | Created |
| `internal/core/ndp/schedule.go`, `schedule_test.go` | Done | Created |
| `rfc/full/rfc4861.txt` | Done | Added |
| `internal/component/l2tp/ppp/ra_linux.go` | Done | Keeps the socket setup only |
| `internal/plugins/iface/ra/ifacera.go`, `sender_linux.go` | Done | Call the shared arithmetic; gained the first-solicitation guard |
| `docs/features/interfaces.md` | Done | Anchors and prose follow the arithmetic |
| `internal/component/l2tp/ppp/ipv6_service_linux.go` | Changed | Unchanged after all: A-1 held, so the stop-path ordering needed no edit |
| `test/plugin/ppp-ra-cease.ci` | Deferred | See above |

### Audit Summary
- **Total items:** 27 (4 requirements, 9 acceptance criteria, 3 test-plan groups, 11 files)
- **Done:** 24
- **Partial:** 0
- **Skipped:** 0
- **Deferred with owner approval:** 3 (the `.ci`, the interop scenario, and their shared blocker), all homed at `plan/future/spec-l2tp-ipv6-subscriber.md`
- **Changed:** 1 (`ipv6_service_linux.go` needed no edit), recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A ceasing PPP subscriber interface tells the host to drop its default route at once, rather than after up to 1800 s | Unit test at the socket seam | `TestStopRASenderSendsAZeroLifetimeRABeforeClose` asserts the recorder saw `send` then `close`, and that the advertisement carried Router Lifetime 0 read from octets 6..8 of the wire bytes. `TestStopRASenderWaitsForTheSenderGoroutine` proves no periodic advertisement can follow it. An end-to-end proof is unreachable: `startRASender` has no production caller, which is the finding this closure defers with the feature |
| Refresh cadence cannot drift away from the router lifetime | Unit test on the constants | `TestRAScheduleIntervalsFitTheRouterLifetime` proves 3 * raMaxRtrAdvInterval equals raRouterLifetime and that all three values sit inside the RFC 4861 Section 6.2.1 bounds. The constants derive from one another in `ra_schedule.go`, so the invariant is structural rather than checked |
| Ze meets both MUSTs of RFC 4861 Section 6.2.6 on the subscriber path | Unit tests over the algorithm and through the real loop | Random solicited delay: `TestRAScheduleDelaysASolicitedAdvertisement` (200 draws) and `TestRASenderLoopAnswersASolicitationWithinMaxRADelayTime`. Rate limit: `TestRAScheduleRateLimitsConsecutiveAdvertisements` (every millisecond offset in the window) and `TestRASenderLoopRateLimitsAnAnswerToASolicitation`. The final advertisement is covered too, by `TestStopRASenderWaitsOutTheRateLimit` |
| One reading of the RFC governs both Router Advertisement senders | Shared producer plus the LAN sender's tests | `internal/core/ndp/schedule.go` is the only place the Section 6.2.4 and 6.2.6 arithmetic exists. `internal/plugins/iface/ra` calls it and gained the first-solicitation guard it was missing |
| Interop with a peer daemon | NOT PROVEN, deferred | No interop scenario exists and none can run: the daemon never opens IPv6CP, and the development machine's Docker host kernel carries no L2TP module. Homed at `plan/future/spec-l2tp-ipv6-subscriber.md` under the owner's 2026-08-30 ruling that the first release ships subscribers IPv4-only |

## Deferrals Resolved

This spec has no deferral shard (`plan/deferrals/ppp-ra-refinements.md` does not
exist), so there is nothing to remove at closure. The two undischarged test rows
are homed in a spec, not in a deferral row.

| Row | Final Status | Destination or evidence |
|-----|--------------|-------------------------|
| `test/plugin/ppp-ra-cease.ci` (functional proof of the cease advertisement) | deferred | `plan/future/spec-l2tp-ipv6-subscriber.md`, "What this spec owes when it runs", third bullet |
| `l2tp-ppp-ipv6-ra-cease` (interop proof that a client kernel drops the route) | deferred | The same bullet of the same spec |
| ISSUE-2 of the review: `sendFinal` in the LAN sender sends three final advertisements back to back | homed elsewhere | `plan/spec-router-advertisement.md`, which owns that sender. A journal row is written in `plan/journal/guard-added-to-one-half-of-a-pair.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ppp-ra-refinements-bae6e1b4-738f-4436-9754-92603923b680.md` (12 files, verdict=clean) |
| `./le spec session review check` | OK (clean, hashes match) |
| Rounds | 2 |
| Reviewer lenses used | Round 1: logic and wiring, RFC conformance against `rfc/full/rfc4861.txt`, test quality by mutation (eleven mutations applied, ten died). Round 2: the fixes of round 1 and what they touched, plus the Ze Go style pass over every changed Go file |

### Round 1 scope
The whole diff of `dfe8e8427`, `94d0a4470` and `82a580a33`: the PPP sender and
schedule, the shared `internal/core/ndp` arithmetic, the LAN sender that now
calls it, and every test of the three.

### Round 2 scope
Written before the round ran: the `ra_send_test.go` change of `7805d5162`
(`sleepClock.Sleep` and `TestStopRASenderWaitsOutTheRateLimit`), the AC-3
evidence-cell correction, and the sibling call sites of `sleepClock`, which are
`TestStopRASenderWaitsOutTheRateLimit` alone.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| ISSUE-1 | ISSUE | The eleventh mutation survived: swapping the cease send and the rate-limit sleep in `stopRASender` left the whole package green. `sleepClock.Sleep` recorded the duration and returned without moving the fake clock, so both orderings produced identical send timestamps. The test could not tell a compliant teardown from one that breaks Section 6.2.6 on the way out | `internal/component/l2tp/ppp/ra_send_test.go`, `sleepClock.Sleep` and `TestStopRASenderWaitsOutTheRateLimit` | `7805d5162`. `Sleep` now advances the clock, and the test asserts the gap between the previous advertisement and the final one. Re-proven at closure: with the sleep and the send swapped, `TestStopRASenderWaitsOutTheRateLimit` fails with "the final advertisement left 1s after the previous one, want at least 3s"; restored, the package is green |
| ISSUE-2 | ISSUE | `sendFinal` (`internal/plugins/iface/ra/sender_linux.go`) sends three zero-lifetime advertisements in a bare loop with no delay, which breaks the same Section 6.2.6 MUST the PPP path now enforces | `internal/plugins/iface/ra/sender_linux.go` | NOT fixed here. It is the LAN sender, not this spec's code, and the two RFC-conformant repairs trade against each other, so the choice is the owner's. Homed at `plan/spec-router-advertisement.md`, which owns that sender; recorded in `plan/journal/guard-added-to-one-half-of-a-pair.md` |
| ISSUE-3 | ISSUE | AC-3's evidence cell cited `TestRAPeriodicIntervalDerivesFromTheRouterLifetime`, a symbol that was never written | `plan/spec-ppp-ra-refinements.md`, Acceptance Criteria | `7805d5162`. The cell now names `TestRAScheduleIntervalsFitTheRouterLifetime`, which exists and proves the derivation |

### NOTEs
Round 1 also returned 5 NIT-level findings. None blocks
(`ai/rules/planning.md`, "Review Gate"). Their texts were not carried into the
closure context, so they are recorded here as a count rather than restated from
memory; inventing them would be worse than counting them.

### Final status
- `/ze-review` round 2 shows 0 BLOCKER, 0 ISSUE.
- Round 2 found nothing in its own scope, and nothing in the always-in-scope
  classes anywhere. The one always-in-scope finding of round 1 was ISSUE-1, a
  vacuous test, and it is fixed and mutation-proven.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/l2tp/ppp/ra_send.go` | yes | `ls -la` on 2026-08-30: 5.6K |
| `internal/component/l2tp/ppp/ra_send_test.go` | yes | 16K |
| `internal/component/l2tp/ppp/ra_schedule.go` | yes | 6.1K |
| `internal/component/l2tp/ppp/ra_schedule_test.go` | yes | 10K |
| `internal/core/ndp/schedule.go` | yes | 3.0K |
| `internal/core/ndp/schedule_test.go` | yes | 5.3K |
| `internal/component/l2tp/ppp/ra_linux.go` | yes | 3.6K |
| `rfc/full/rfc4861.txt` | yes | 230K |
| `test/plugin/ppp-ra-cease.ci` | NO | `ls: cannot access 'test/plugin/ppp-ra-cease.ci': No such file or directory`. Deferred, see Deferrals Resolved |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-9 | the stop path sends one zero-lifetime advertisement, best effort, after the rate limit | `go test -race -count=1` over `./internal/component/l2tp/ppp/`: `ok ... 11.013s`. Every named test is declared in `ra_send_test.go` (`grep '^func Test'`) |
| AC-3, AC-4 | the intervals derive from the router lifetime | Same run. `TestRAScheduleIntervalsFitTheRouterLifetime` is declared at `ra_schedule_test.go:178` |
| AC-5 to AC-8 | the Section 6.2.6 and 6.2.4 send schedule | Same run, plus `ok github.com/ze-software/ze/internal/core/ndp 2.001s` and `ok github.com/ze-software/ze/internal/plugins/iface/ra 1.529s` |
| AC-9 discrimination | the AC-9 test would fail if the code were wrong | Mutation applied at closure: sleep and send swapped in `stopRASender`. `TestStopRASenderWaitsOutTheRateLimit` went red ("the final advertisement left 1s after the previous one, want at least 3s"), then the file was restored and `git status` reports the package clean |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| session with IPv6 up is torn down | none | NO, and it cannot be. `poolPlugin.handle` (`internal/component/l2tp/plugins/pool/register.go`) refuses every non-IPv4 family, `l2tp.RegisterPoolHandler` has one caller, and `afterLCPOpen` (`internal/component/l2tp/ppp/session_run.go`) starts the IPv6 service only when `ipv6cpState` is `LCPStateOpened`. Read at each producer, not inferred. Deferred with the feature under the owner's 2026-08-30 ruling |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `run`'s defer (`session_run.go`) calls `teardownNCPResources` before `s.chanFile.Close()`, and that function calls `ipv6Svc.Stop()` first, so the socket is live at cease time |
| A-2 | confirmed | `raSender.send` logs a write failure at Debug and returns nothing; `TestStopRASenderIsBestEffortWhenTheInterfaceIsGone` drives the failing-send and failing-close path |
| A-3 | confirmed | RFC 4861 Section 6.2.1 gives AdvDefaultLifetime a default of 3 * MaxRtrAdvInterval, so lifetime/3 is the RFC's own ratio |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features/interfaces.md` RA anchors point at the arithmetic | `UnsolicitedInterval`, `MaxInitialAdvertisements`, `SolicitedDelay`, `SolicitedSendTime` and `MinDelayBetweenRAs` are all declared in `internal/core/ndp/schedule.go` | yes |
| `docs/features.md` RA row anchor | Was naming `unsolicitedInterval` in `ifacera.go`, where it no longer exists; repaired at closure, along with a `RaisePrefixStale` anchor repaired on touch. `./le doc check verify` went from 37 stale anchors to 35, and `docs/features.md` now has none | yes, but the file is NOT committed here: the commit gate reads a 2026-08-29 verify snapshot that still names it. See Documentation Updates |
| RFC status page | RFC 4861 is in neither `rfc/enrolled.txt` nor `rfc/short/`, so `check_status_completeness` owes no row | yes, no update needed |
| CLI, config syntax, YANG, plugin SDK, wire format | This work adds no CLI verb, no config leaf, no YANG node and no new wire field: the cease advertisement uses the existing `BuildRA` encoder with a lifetime of zero | yes, no update needed |
| Doctor checks | No new runtime dependency. The RA socket and its ICMP filter predate this work, and `doctor-iface-ra-forwarding` already covers the LAN sender | yes, no update needed |

### Verification state at commit time
`./le verify status check` reports STALE (the last full run failed on
2026-08-29T14:01:28Z, before any of this spec's commits). The gate runs against
a commit in a throwaway worktree and is periodic, not per-commit
(`ai/rules/git-safety.md`), so the closure commits record verification-debt rows
rather than waiting for it. The debt file `plan/verification-debt/a7b3c9d2.md`
already carries open rows for every commit this shared checkout made since that
run, including this spec's four. Closure commits A and B carry no Go, no `.ci`
and no `.et`: they change Markdown only.

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
