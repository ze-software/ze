# Spec: bgp-update-delay

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-09-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - reactor/plugin startup model
4. `internal/component/bgp/plugin/register.go` - peer startup wiring
5. `internal/component/bgp/reactor/` - `StartPeers`, outbound advertisement path

## Task

On BGP process startup (boot or reactor restart), Ze begins advertising routes to
peers the instant each session reaches Established, before the RIB has learned the
routes it will eventually receive from other peers. This causes premature, churny
advertisement: neighbours see an initial route set, then a rapid sequence of
add/withdraw as the local best-path stabilises.

Add a **convergence-hold (read-only) startup mode** for the BGP engine:

- `max-delay` (seconds): after startup, hold the engine's OUTBOUND side. While
  held, sessions negotiate, reach Established and receive UPDATEs normally, but
  the engine sends no UPDATE and no End-of-RIB marker. The hold ends when every
  *expected* peer has sent its own End-of-RIB marker or the `max-delay` timer
  expires, whichever comes first.
- `establish-wait` (seconds, optional, `≤ max-delay`): how long to wait before
  releasing on the peers that ARE up, whether or not they have finished. This
  lets a router that boots faster than its neighbours still wait for them.

**AMENDED 2026-09-08, by Thomas.** The paragraph above originally said the engine
"does not run best-path". It does. Ze has no reactor-side best-path scheduler to
gate: route selection lives in `internal/component/bgp/plugins/rib`, which the
reactor must not reach into (`ai/rules/plugins.md`), so deferring it would need a
new reactor-to-plugin seam and a second "held" fact that can disagree with the
first. Thomas decided on 2026-09-08 to AMEND the wording rather than build that
seam. The consequence is stated rather than hidden: **best-path selection still
runs during the hold, so the CPU saving the original wording implied is not
delivered.** What the hold delivers is the wire behaviour a neighbour observes,
which is the churn the feature exists to remove. This is his decision, not an
author's scope call.

This is a local-process startup feature. It is distinct from graceful restart
(which concerns a *remote* peer restarting) and from per-peer reconnect backoff.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. -->
- [ ] `docs/architecture/core-design.md` - startup ordering, plugin coordinator
  → Constraint: peer startup is already deferred behind `coord.OnPostStartup` until all tiers finish their 5-stage handshake; the convergence hold layers on top of this, it does not replace it.
- [ ] `docs/architecture/cli/color-system.md` - only if a `show` command exposes read-only state (semantic roles)
  → Constraint: any new operational output uses the 7 semantic roles.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 4271 (BGP-4) baseline UPDATE/best-path semantics. `update-delay` is a
  widely-implemented operational behaviour, not a wire-protocol change; no new
  capability or attribute is negotiated. (Create an `rfc/short/` summary during
  implementation only if design surfaces a normative requirement.)
  → Constraint: read-only mode must not alter OPEN/negotiation; only outbound Adj-RIB-Out population and best-path scheduling are gated.

**Key insights:**
- Nothing is sent on the wire that a peer can detect as "read-only"; the hold is purely a local decision to defer best-path + outbound UPDATE generation.
- The hold must still allow inbound UPDATEs to be received and stored (Adj-RIB-In), otherwise convergence can never be detected.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugin/register.go` - `coord.OnPostStartup` calls `bgpReactor.StartPeers()` with no convergence timer and no advertisement/best-path suppression (register.go). Peers are started unconditionally once plugin startup completes.
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` - graceful restart is per-remote-peer only (stale marking, restart timer, EOR-driven purge); no local-process startup deferral of best-path or UPDATE.
- [ ] `internal/component/bgp/reactor/` - outbound advertisement / best-path path (exact function to be located during design; `internal/component/bgp/rib/` holds the outbound side).

**Behavior to preserve:**
- Peer startup remains gated behind `coord.OnPostStartup` (register.go); the hold is additive.
- When no `update-delay` is configured, behaviour is byte-for-byte unchanged: peers start and advertise immediately.
- Inbound UPDATE processing and RIB population are unaffected while held.
- Graceful restart semantics (gr plugin) are untouched.

**Behavior to change:**
- When `update-delay max-delay` is set, the engine defers each peer's initial routing update, and the End-of-RIB marker that closes it, until the hold releases. Best-path is NOT deferred (AMENDED 2026-09-08 by Thomas; see Task).

## Data Flow (MANDATORY)

### Entry Point
- Config: new leaves under the BGP `parameters` container, e.g. `parameters update-delay max-delay <sec>` and `parameters update-delay establish-wait <sec>`, defined in the BGP component/plugin YANG.
- Resolved through the standard config path: File → Tree → `ResolveBGPTree()` → `map[string]any` → `reactor.PeersFromTree()` / reactor settings.

### Transformation Path
1. YANG leaves parsed into reactor/global BGP settings (a new `UpdateDelay` settings struct: `MaxDelay`, `EstablishWait`).
2. On `StartPeers()` (register.go), if `MaxDelay > 0`, the reactor enters read-only mode and arms the hold timer.
3. Sessions negotiate and reach Established normally; inbound UPDATEs populate Adj-RIB-In / RIB, but best-path scheduling and outbound Adj-RIB-Out generation are suppressed.
4. Hold-release condition evaluated on each peer Established transition and on timer expiry: release when expected peers are all Established or `max-delay` elapses.
5. On release: run best-path once, then generate and send the initial outbound UPDATE set to all peers; resume normal steady-state advertisement.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ Reactor | YANG leaves → `UpdateDelay` settings via `PeersFromTree` | [ ] |
| Reactor ↔ RIB/best-path | read-only flag gates best-path + Adj-RIB-Out generation | [ ] |
| FSM ↔ Reactor | Established transitions drive hold-release evaluation | [ ] |

### Integration Points
- `bgpReactor.StartPeers()` (register.go) - arm the hold here.
- Outbound advertisement / best-path scheduler in `internal/component/bgp/rib/` - honour the read-only flag.
- Per-peer FSM Established callback - notify the hold evaluator.

### Architectural Verification
- [ ] No bypassed layers (config flows through `PeersFromTree`, not a side channel)
- [ ] No unintended coupling (gr plugin untouched; read-only is a reactor concern)
- [ ] No duplicated functionality (reuse existing best-path scheduling; add a gate, not a parallel path)
- [ ] Zero-copy preserved where applicable
- [ ] Registration over hardcoding — the read-only state and any `show` view register through existing registries; no new per-feature switch case added to a core struct.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Outbound UPDATE generation has a single choke point that can be gated | `internal/component/bgp/rib/` outbound path | If dispersed, gating is invasive | grep/read the Adj-RIB-Out generation path during audit | broken -- there is no single outbound choke point, and none was needed |
| A-2 | Inbound processing continues while held so convergence is observable | reactor FSM independent of outbound | Convergence never detected → always times out (still safe) | unit test with held reactor receiving UPDATEs | confirmed -- the hold withholds only the sendInitialRoutes spawn; the session read loop is untouched |
| A-3 | "Expected peers" = configured peers admin-enabled at startup | operator intent | Wrong set → premature or delayed release | design confirmation with user | confirmed -- `len(peersToStart)` in `Reactor.StartPeers`, fixed at arm time |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Read-only mode deadlocks steady-state advertisement if release never fires | peers Established but no routes advertised | max-delay timer is a hard backstop; unit test asserts release-by-timer |
| R-2 | Held reactor delays legitimate fast convergence in small topologies | operator reports slow first advertisement | feature is opt-in (max-delay default 0 = off) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set protocols bgp parameters update-delay max-delay 30` | → | reactor enters read-only mode in `StartPeers` | `test/plugin/bgp-update-delay.ci` |
| peers reach Established before timer | → | hold releases, initial UPDATE sent | `test/plugin/bgp-update-delay.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `max-delay 30`, one expected peer | engine holds; no outbound UPDATE and no End-of-RIB until that peer's own End-of-RIB arrives or 30s elapse |
| AC-2 | every expected peer sends its End-of-RIB at t<max-delay | hold releases at that moment; the initial UPDATE set and this speaker's own End-of-RIB are sent. Best-path is NOT deferred and is not re-run here (AMENDED 2026-09-08 by Thomas; see Task) |
| AC-3 | no peer reaches Established | hold releases at `max-delay`; engine advertises whatever it has |
| AC-4 | `establish-wait 40` with `max-delay 30` | config rejected at verify: establish-wait must be ≤ max-delay |
| AC-5 | `max-delay 0` or leaf absent | behaviour unchanged: immediate advertisement |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures `update-delay max-delay 30` and boots with a slow neighbour | config → reactor read-only → hold until Established/timer → single converged UPDATE | `test/plugin/bgp-update-delay.ci` |
| 2 | sets `establish-wait 40 max-delay 30` | config verify rejects with a clear error | `test/plugin/bgp-update-delay-validation.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUpdateDelayHoldsUntilEstablished` | `internal/component/bgp/reactor/update_delay_test.go` | read-only until expected peers Established | pass |
| `TestUpdateDelayReleasesOnTimer` | `internal/component/bgp/reactor/update_delay_test.go` | release at max-delay with no peers up | pass (and `TestUpdateDelayReleasesOnTimerWithNoPeerEstablished`) |
| `TestUpdateDelaySuppressesOutbound` | `internal/component/bgp/reactor/update_delay_test.go` | no Adj-RIB-Out generation while held | pass |
| `TestUpdateDelayEstablishWaitOverMaxDelayRefused` | `internal/component/bgp/config/update_delay_test.go` | establish-wait ≤ max-delay | pass (with `...RefusedAtLoad` and `TestUpdateDelayBoundaries`) |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| max-delay | 0-3600 | 3600 | N/A (0 = disable) | 3601 |
| establish-wait | 1-3600 (≤ max-delay) | max-delay | 0 | max-delay+1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-update-delay` | `test/plugin/bgp-update-delay.ci` | configured hold releases on the timer, and the route plus End-of-RIB follow | pass; discrimination RED observed |
| `bgp-update-delay-validation` | `test/plugin/bgp-update-delay-validation.ci` | invalid establish-wait rejected, equal accepted | pass |
| `bgp-update-delay-converges` | `test/plugin/bgp-update-delay-converges.ci` | the hold releases on CONVERGENCE: one graceful-restart peer sends its End-of-RIB marker and ze's route plus its own marker follow | pass; discrimination RED observed (round 6) |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-update-delay-frr` | `test/interop/scenarios/bgp-update-delay-frr/` | FRR | FRR reaches Established while Ze holds, its table shows the prefix ABSENT, and the prefix arrives after the release line | PASS. Discrimination walk run: `UpdateDelay.Enabled` forced false, the `ze-interop` image rebuilt from that source, scenario RED on `assertion 2: peer output unexpectedly contains "10.10.0.0/24"`; restored, image rebuilt, GREEN |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/bgp/plugin/register.go` - arm hold in `StartPeers`
- `internal/component/bgp/reactor/` - read-only state, hold timer, release evaluator, outbound gate
- `internal/component/bgp/rib/` - honour read-only flag in Adj-RIB-Out / best-path scheduling
- BGP config resolution (`internal/component/bgp/config/`) - parse new leaves, validate establish-wait ≤ max-delay

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | BGP component/plugin `yang/` - `update-delay { max-delay, establish-wait }`; read `ai/rules/config.md`, `ai/rules/config.md` |
| YANG validation constraints | [ ] yes | `range 0..3600` / `1..3600`; custom validator for establish-wait ≤ max-delay |
| CLI grammar | [ ] yes | `ai/rules/cli.md` |
| Functional test for new behaviour | [ ] yes | `test/plugin/bgp-update-delay.ci` |
| Prometheus counters/metrics | [ ] maybe | a gauge for "read-only active" and a counter for release-reason (converged/timeout) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` |

## Files to Create
- `internal/component/bgp/reactor/update_delay.go` - hold state machine
- `internal/component/bgp/reactor/update_delay_test.go` - unit tests
- `test/plugin/bgp-update-delay.ci` - functional test
- `test/plugin/bgp-update-delay-validation.ci` - validation test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add YANG leaves + settings struct, arm a no-op hold in `StartPeers`, write failing `test/plugin/bgp-update-delay.ci`.
   - Verify: config accepted, entry point reached, wiring test fails on stubbed behaviour.
2. **Phase: Read-only gate** — suppress best-path + Adj-RIB-Out while held.
   - Tests: `TestUpdateDelaySuppressesOutbound`
3. **Phase: Release logic** — evaluate on Established + timer; run best-path once and flush initial UPDATEs.
   - Tests: `TestUpdateDelayHoldsUntilEstablished`, `TestUpdateDelayReleasesOnTimer`
4. **Phase: Validation** — establish-wait ≤ max-delay at config verify.
   - Tests: `TestUpdateDelayValidation`
5. **Functional + interop tests**
6. **Full verification** → `./le verify current mode full`
7. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | timer never leaks; release is idempotent; no double initial-flush |
| Data flow | inbound processing unaffected while held |
| Registration over hardcoding | read-only `show` view (if any) registered, not hardcoded into the CLI Model |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| read-only hold in reactor | `go test ./internal/component/bgp/reactor -run UpdateDelay` |
| config validation | `test/plugin/bgp-update-delay-validation.ci` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | max-delay/establish-wait range enforced in YANG + verify |
| Resource exhaustion | held reactor still bounds Adj-RIB-In growth |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-1: outbound UPDATE generation has a single choke point in `internal/component/bgp/rib/` that the hold can gate | It has none. Outbound UPDATEs leave through `Peer.SendUpdate`, `SendAnnounce`, `sendWithdraw`, `sendRawUpdateBody` and `SendRawMessage`, and the route-server fast path writes into the destination's `bufWriter` without passing any of them (`forward_rs.go`, `tryDirectWriteNoFlush`). `internal/component/bgp/rib/` holds the UPDATE BUILDER, not a scheduler | Read `send_permission.go` (which gates plugin permission, not advertisement) and `peer_send.go` during the audit | None on scope. The design does not need the assumption: it withholds the `sendInitialRoutes` SPAWN instead, and inherits `shouldQueue`, `forwardOrderHold` and the withheld End-of-RIB from establishment |
| The engine defers BEST-PATH, as the Task section says | Ze has no reactor-side best-path scheduler to defer. Route selection lives in `internal/component/bgp/plugins/rib`, which the reactor must not reach into (`ai/rules/plugins.md`) | Located the Established transition and its one spawn site (`peer_run.go:518`) | The hold defers the initial routing UPDATE, which is what the neighbour observes. Deferring selection as well would have needed a new reactor-to-plugin seam and a second "held" fact that can disagree with this one |
| The interop scenario could assert on ze's own log lines | The scenario's ze runs at the WARN default and the hold's lines are INFO, and the lab has no per-scenario env mechanism, so `opWaitLogContains` timed out rather than asserting | First interop run | The checker reads FRR alone, which is the better witness for an interop scenario anyway: the peer's table says the prefix is absent while ze holds and present after |
| A new `show bgp` child needs only a column order | It needs an ANSWER SHAPE as well, and the loop that registers the children declares neither. `registerColumns` carried a special case for it and `registerShapes` did not, so the command published every row operator over an answer with no rows | `TestEveryShowBgpPathDeclaresAShape` RED at closure, after four review rounds read the diff | One BLOCKER, fixed at closure. It also stales three hand-typed population counts, which were corrected in the same edit |
| The build and interop runs would be available throughout | Two other sessions were mid-edit in `internal/component/config/transaction`, `internal/component/bgp/plugin` and `internal/le/interoplab/bgp`, so `bin/ze` and a session-private `le` each failed to compile for roughly 25 minutes | Five build attempts, each failing on a different missing symbol of theirs | Delay only. Both compiled once those sessions landed their symbols, and every run below is against a clean build |

## Design Insights
<!-- LIVE -->

**The hold needed no new gate on the outbound path, and that is the whole
design.** `Peer.setState` closes the initial-sync gate in the same call that
publishes `PeerStateEstablished`, and the gate already produces the three
behaviors a read-only mode wants: `shouldQueue` queues a route operation,
`forwardOrderHold` parks a forwarded UPDATE in the destination worker's overflow
buffer, and the End-of-RIB is not sent because `sendInitialRoutes` has not run.
So the hold withholds ONE thing, the spawn of that goroutine, and the release
runs it. A second gate on the send path would have been a second copy of the
"held" fact with its own bugs, which is what `filterPermittedPeers`
(`send_permission.go`) already records the cost of.

**Withholding the End-of-RIB is the conformance argument, not a side effect.**
RFC 4724 Section 4.1: "Once the initial update is complete for an address family
(including the case that there is no routing update to send), the End-of-RIB
marker MUST be sent." A marker sent from under the hold would claim a completion
that had not happened. The same section is where the deferral itself comes from,
for a Restarting Speaker: it MUST defer route selection until it has every
peer's marker or its Selection_Deferral_Timer expires, and "an implementation
MUST support a (configurable) timer that imposes this upper bound". `max-delay`
is that timer, offered for a cold start as well.

**Operators must keep `max-delay` under the Restart Time Ze advertises.** The two
timers are independent, and a neighbour whose restart timer expires first
discards the routes it was holding for Ze, which is the outcome the hold exists
to avoid. This is documented in the leaf's `ze:help` and in
`docs/guide/configuration.md` rather than enforced: the RFC requires no relation
between them, and enforcing one would be inventing policy.

## Implementation Summary
### What Was Implemented

| AC | Producer | Test |
|----|----------|------|
| AC-1 | `updateDelayHold.arm` + `Peer.startInitialRoutes` (`internal/component/bgp/reactor/update_delay.go`), armed from `Reactor.StartPeers` (`reactor.go`) | `TestUpdateDelayHoldsUntilEstablished`, `TestUpdateDelaySuppressesOutbound`, `test/plugin/bgp-update-delay.ci` |
| AC-2 | `updateDelayHold.holdInitialUpdate` releases with `updateDelayConverged` when `len(held) == expected` | `TestUpdateDelayHoldsUntilEstablished` |
| AC-3 | the `maxDelay` arm of the timer goroutine in `updateDelayHold.startDeadlinesLocked` | `TestUpdateDelayReleasesOnTimer`, `TestUpdateDelayReleasesOnTimerWithNoPeerEstablished`, `test/plugin/bgp-update-delay.ci` |
| AC-4 | `ParseUpdateDelay` (`internal/component/bgp/config/update_delay.go`), called from `CreateReactorFromTree` for the value and from `peersAndDynamicGroups` for the error | `TestUpdateDelayEstablishWaitOverMaxDelayRefused`, `...RefusedAtLoad`, `TestUpdateDelayBoundaries`, `test/plugin/bgp-update-delay-validation.ci` |
| AC-5 | `UpdateDelay.Enabled`, the single reading of the off switch, checked in `arm` | `TestUpdateDelayNotArmedWithoutMaxDelay`, `TestUpdateDelayAbsentContainerDisablesTheHold`, `TestUpdateDelayExplicitZeroDisablesTheHold` |

Beyond the acceptance criteria, two guards are named and tested because a zero
value would otherwise answer for them (`ai/rules/principles.md`):

- `arm` refuses `expected < 1`. "Every expected peer is Established" is true of
  an empty set, so a hold armed on zero would release the instant it started and
  report that it converged. `TestUpdateDelayNotArmedWithNoExpectedPeers`.
- `releaseIfAnyEstablished` refuses an empty held set. An establish-wait deadline
  that fires with no peer up would otherwise make the leaf a plain shorter
  max-delay. `TestUpdateDelayEstablishWaitHoldsWhenNoPeerEstablished`.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `max-delay` holds the OUTBOUND side: sessions negotiate, establish and receive UPDATEs, but no UPDATE and no End-of-RIB leave this speaker | Done | `updateDelayHold.holdInitialUpdate` and `Peer.startInitialRoutes` (`internal/component/bgp/reactor/update_delay.go`) | The hold withholds ONE thing, the spawn of `Peer.sendInitialRoutes`. Establishment already closed the initial-sync gate, so `shouldQueue`, `forwardOrderHold` and the withheld marker follow from it |
| The hold ends on every expected peer's End-of-RIB, or on `max-delay` | Done | `updateDelayHold.releaseIfConvergedLocked` and the `maxDelay` arm of `startDeadlinesLocked` | Per negotiated family, struck off in `peerEndOfRIB` |
| `establish-wait` releases early on the peers that ARE up | Done | `updateDelayHold.releaseIfAnyHeld` | Refuses an empty held set, so the leaf is not a shorter `max-delay` |
| Best-path is NOT deferred | Changed | Task section, AMENDED 2026-09-08 by Thomas | Owner decision, recorded in the Task and in AC-2 |
| Distinct from graceful restart and from reconnect backoff | Done | `internal/component/bgp/plugins/gr/` untouched; the hold is a reactor field | `git diff --stat internal/component/bgp/plugins/gr/` is empty |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestUpdateDelayHoldsUntilEstablished`, `TestUpdateDelaySuppressesOutbound`, `test/plugin/bgp-update-delay.ci` | `arm` from `Reactor.StartPeers` (`reactor.go`) and from `Reactor.StartWithContext` |
| AC-2 | Done | `TestUpdateDelayHoldsUntilEstablished` | Releases with `updateDelayConverged` when every expected peer is settled. Best-path is not re-run: owner amendment |
| AC-3 | Done | `TestUpdateDelayReleasesOnTimer`, `TestUpdateDelayReleasesOnTimerWithNoPeerEstablished`, `test/plugin/bgp-update-delay.ci` | `.ci` asserts `reason=max-delay` and rejects `reason=converged` |
| AC-4 | Done | `TestUpdateDelayEstablishWaitOverMaxDelayRefused`, `...RefusedAtLoad`, `TestUpdateDelayBoundaries`, `test/plugin/bgp-update-delay-validation.ci` | `ParseUpdateDelay` is called twice, once for the value and once for the error `ze config validate` reaches |
| AC-5 | Done | `TestUpdateDelayNotArmedWithoutMaxDelay`, `TestUpdateDelayAbsentContainerDisablesTheHold`, `TestUpdateDelayExplicitZeroDisablesTheHold` | `UpdateDelay.Enabled` is the ONE reading of the off switch |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestUpdateDelayHoldsUntilEstablished` | Done | `internal/component/bgp/reactor/update_delay_test.go` | pass |
| `TestUpdateDelayReleasesOnTimer` | Done | same | pass, with the no-peer sibling |
| `TestUpdateDelaySuppressesOutbound` | Done | same | pass |
| `TestUpdateDelayEstablishWaitOverMaxDelayRefused` | Done | `internal/component/bgp/config/update_delay_test.go` | pass |
| `TestUpdateDelayReasonWords` | Changed | `internal/component/bgp/reactor/update_delay_test.go` | ADDED at closure. Every other reason assertion compares `String()` to `String()`, so a collapsed `String()` survived them all |
| `bgp-update-delay` | Done | `test/plugin/bgp-update-delay.ci` | pass; discrimination RED observed |
| `bgp-update-delay-validation` | Done | `test/plugin/bgp-update-delay-validation.ci` | pass |
| `bgp-update-delay-command` | Done | `test/ui/bgp-update-delay-command.ci` | pass; drives the SSH CLI |
| `bgp-update-delay-frr` | Done | `test/interop/scenarios/bgp-update-delay-frr/` | PASS with the discrimination walk recorded in the spec's Interop table |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/update_delay.go` | Done | the hold, its two entry points on `Peer`, and `Reactor.UpdateDelayStatus` |
| `internal/component/bgp/reactor/update_delay_test.go` | Done | |
| `internal/component/bgp/plugin/register.go` | Changed | The arm site is `Reactor.StartPeers` (`reactor.go`) and `Reactor.StartWithContext`, not the plugin's register file |
| `internal/component/bgp/rib/` | Changed | Not touched. A-1 was broken: there is no single outbound choke point and none was needed |
| `internal/component/bgp/config/` | Done | `update_delay.go`, called from `loader_create.go` and `peers.go` |
| `test/plugin/bgp-update-delay.ci` | Done | |
| `test/plugin/bgp-update-delay-validation.ci` | Done | |

### Audit Summary
- **Total items:** 26
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (best-path deferral, amended by Thomas; the arm site; `internal/component/bgp/rib/` untouched; one test added at closure)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A neighbour sees one settled route set instead of an initial set followed by add/withdraw churn | interop | `test/interop/scenarios/bgp-update-delay-frr/`: FRR reaches Established while Ze holds, FRR's own table shows `10.10.0.0/24` ABSENT, and the prefix arrives after the release. Discrimination walk run: `UpdateDelay.Enabled` forced false, `ze-interop` image rebuilt, scenario RED on `assertion 2: peer output unexpectedly contains "10.10.0.0/24"`; restored, rebuilt, GREEN |
| The hold ends on convergence, on establish-wait, or on max-delay, and never runs past max-delay | functional | `test/plugin/bgp-update-delay.ci` asserts `update-delay armed.*expected-peers=2` and `update-delay released.*reason=max-delay`, and REJECTS `reason=converged`. The wire assertions alone are vacuous and the file says so |
| An operator can tell a holding speaker from a wedged one | user workflow | `test/ui/bgp-update-delay-command.ci` types `show bgp update-delay` at a running daemon over SSH and reads `"configured": true`, `"holding": true`, `"reason": "not-released"`, `"expected-peers": 2` out of the answer |
| A config the daemon would refuse at startup is refused at verify | negative | `test/plugin/bgp-update-delay-validation.ci`; `ParseUpdateDelay` is reached from `peersAndDynamicGroups` for that reason alone |
| Nothing changes when the operator configures nothing | functional | `UpdateDelay.Enabled` is the one off switch, checked in `arm`; `TestUpdateDelayAbsentContainerDisablesTheHold` and `TestUpdateDelayExplicitZeroDisablesTheHold` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Deferring best-path selection during the hold | Thomas amended the Task on 2026-09-08 rather than build a reactor-to-plugin seam and a second "held" fact. The wire behaviour a neighbour observes is delivered; the CPU saving the original wording implied is not | None. The owner closed the item; it is not outstanding work |
| A Prometheus gauge for "hold active" and a counter for release reason | The Integration Checklist marked it `maybe`. `show bgp update-delay` answers the operational question, and no metric consumer asked for it | None. Not in scope and not owed |

### Bugs Found/Fixed

- **BLOCKER, found at closure.** `show bgp update-delay` declared no answer
  shape. `registerShapes` (`internal/component/bgp/plugins/cmd/peer/peer.go`)
  ends in a loop that declares an EMPTY shape for every entry of
  `cmdBgpChildren`, which is correct for the ten children a plugin process
  answers and wrong for the one this package answers. `validateDeclaredShape`
  skips its shape test on an undeclared command, so every row operator was
  published over an answer holding no rows. The pre-existing
  `TestEveryShowBgpPathDeclaresAShape` was RED on it. Fixed by special-casing
  the child to `command.ShapeDoc`, mirroring the special case `registerColumns`
  already carried. Journal row:
  `plan/journal/blanket-mechanism-hid-missing-cases.md`, 2026-09-09.
- **Lint, found at closure.** `Peer.QueueAnnounce` and `Peer.QueueWithdraw`
  gained an error return in this spec's diff, and eight existing test call
  sites did not check it (`errcheck`). Fixed in
  `internal/component/bgp/reactor/peer_test.go` and
  `forward_initial_sync_order_test.go`; the site past the cap now asserts
  `ErrOpQueueFull` rather than discarding it, which is what the new return is
  for.
- **Spelling, found at closure.** `cancelled` in `update_delay.go` and its
  test. The project language is US English (`ai/rules/writing.md`).

### Documentation Updates

- `docs/features.md`, `docs/comparison.md`, `docs/guide/configuration.md`,
  `docs/features/bgp-protocol.md`,
  `docs/architecture/behavior/peer-lifecycle.md` and
  `docs/guide/command-reference.md` were edited by the implementation; each is
  verified against its producer in Pre-Commit Verification below.
- Added at closure, because this diff moved a published count. Three pages
  state the size of the Go-declared answer-shape population, hand-typed:
  `docs/architecture/api/commands.md`, `docs/features/cli-commands.md` and
  `docs/guide/command-reference.md`. `show bgp update-delay` is the twentieth,
  so all three moved from nineteen to twenty. The count was recomputed by
  enumerating every non-empty `command.RegisterShape` call under `show bgp`:
  6 in `cmd/rib/rib.go`, 5 in `cmd/peer/peer.go`, 1 in `cmd/peer/health.go`,
  2 in `cmd/peer/summary.go`, 3 in `filter_irr/cmd_irr.go` and 3 in
  `filter_path_asn/register_command.go`.
- `./le verify lint run` exit 0. The findings it reported against this spec's
  files are the three fixed above; the rest belong to other packages.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Round 1 scope (fixed before the round ran, 2026-09-08)

The WHOLE diff of this spec, with at least two lenses. In scope:
`internal/component/bgp/reactor/update_delay.go` and its test, the hold's arming
site in `reactor.go` `Reactor.StartPeers`, the gate in `Peer.startInitialRoutes`
and its call site in `peer_run.go`, `internal/component/bgp/config/update_delay.go`
`ParseUpdateDelay` and its two call sites (`loader_create.go` `CreateReactorFromTree`,
`peers.go` `peersAndDynamicGroups`), the `update-delay` container in
`internal/component/bgp/yang/ze-bgp-conf.yang`, `test/plugin/bgp-update-delay.ci`,
`test/plugin/bgp-update-delay-validation.ci`,
`test/interop/scenarios/bgp-update-delay-frr/` and its checker registration, and
the three doc pages edited (`docs/architecture/behavior/peer-lifecycle.md`,
`docs/guide/configuration.md`, `docs/features/bgp-protocol.md`).

Out of scope, because they belong to other sessions in flight in this shared
checkout: the as-notation work (`internal/core/bgp/asn/`, the ASN renderers, the
`as-notation` leaf), the BFD strict-mode work (`internal/component/bgp/fsm/`, the
capability packages, `session_bfd_strict.go`, `speaker_bfd.go`),
`internal/component/config/transaction/`, `internal/component/iface/`, and the
`plugin/yang` rename.

Always-in-scope classes apply anywhere they are found, per `ai/rules/planning.md`.

### Gate record

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-update-delay-828bf96b-ed79-41e5-bc2d-b82d08d4124a.md`, RE-RECORDED at closure over the current tree (51 files) |
| `./le spec session review check` | `review_gate: OK (0 code files, clean, hashes match)` |
| Rounds | 8 |
| `rounds-reason` | round 6 found the park-and-restore cycle had severed the ONLY production caller of `Peer.updateDelayEndOfRIB` in `reactor_notify.go`, so the hold could never converge and released on a timer alone; the same cycle had dropped the `bgp-update-delay-frr` scenario key from `checkers.go` |
| `owner-authorised` | Thomas authorised round 6 and round 7 individually on 2026-09-11, each because the park cycle altered files the recorded review covered |
| Reviewer lenses used | rounds 1-4: independent contexts, scopes fixed below. Round 5 (closure): wiring over the registration surface, claim over every doc comment. Rounds 6 and 7: independent contexts over what the park cycle changed. Round 8 (this closure): reachability to a production caller, and claim against producer |

The PREVIOUS artifact did not cover the round 6 and 7 work: three covered files'
hashes differed from the tree, and `bgp-update-delay-converges.ci` plus six mocks
were not listed at all. `./le spec session review check` printed `0 code files`
because no commit was pending, which is the absence of a comparison rather than
coverage. It is re-recorded over the tree this closure commits.

### Rounds 1 to 3

Each round ran in an independent context and recorded its FIXED SCOPE above. The
rounds themselves did not write a per-finding table, and this closure does not
invent one: what those rounds found is recoverable only from the scope paragraph
of the round that followed, which names the fixes the previous round produced.
Round 2's scope names round 1's fixes, round 3's names round 2's, and round 4's
names round 3's. Every fix they produced is in the diff this closure reviewed.

### Round 4
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `UpdateDelayStatus.Configured`'s doc said "When false every other field is the zero value", but both producers write `Reason = "not-released"` when Configured is false | `internal/component/plugin/types_bgp.go` | fixed in round 5 |
| 2 | NOTE | `// Stats returns reactor statistics for the API.` headed the newly inserted `UpdateDelayStatus`, leaving `Stats` with no doc comment | `internal/component/bgp/reactor/reactor_api.go` | fixed in round 5 |
| 3 | NOTE | Every reason assertion compared `String()` output to `String()` output, so a `String()` collapsed to one word survived them all | `internal/component/bgp/reactor/update_delay_test.go` | fixed in round 5 |

### Round 7 scope (owner-authorised, fixed before the round ran, 2026-09-11)

Thomas authorised this round on 2026-09-11. It covers ONLY the fixes round 6
produced, plus the sibling call sites they touched.

1. The restored production caller `peer.updateDelayEndOfRIB(eorFamily)` in the
   inbound End-of-RIB decode of `internal/component/bgp/reactor/reactor_notify.go`,
   with its new comment naming it as the only production caller.
2. The restored `bgp-update-delay-frr` registration in
   `internal/le/interoplab/bgp/checkers.go`, block and non-vacuity note.
3. The new functional test `test/plugin/bgp-update-delay-converges.ci` with its
   fixture, and the two costs it records: ze must declare graceful-restart
   because the peer harness mirrors ze's capability set, and a graceful-restart
   peer must attach a process permitted to put a route on the wire
   (`send [ update ]` or `send [ raw ]`).
4. The `cmd/peer` mock completing an unset record the way a producer does
   (`Reason` to `UpdateDelayReasonNotReleased`) and
   `TestBgpUpdateDelayReportsAnUnconfiguredDaemon` asserting that constant.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 6 cleared that these fixes did not touch,
including the two extended test populations and the 13 previously uncovered
files, both settled in round 6.

### Round 6 scope (owner-authorised, fixed before the round ran, 2026-09-11)

Thomas authorised this round on 2026-09-11. It exists because the park and
restore cycle altered files the recorded review covered, and added content it
never saw. The FEATURE is unchanged and passed four rounds plus closure's own
pass; this round covers only what the cycle changed.

In scope, and nothing else:

1. The two extended derived test populations in
   `internal/component/bgp/plugins/cmd/peer/peer_shape_test.go` and
   `summary_test.go`. During the park both tests passed because the command was
   absent; on restore `TestDeclaredShapesReachTheRegistry` and
   `TestChildCommandsDoNotInheritTheSummaryOrder` failed, and each table was
   extended to pin `ShapeDoc` and the nine-column order exactly rather than to
   assert the command declares nothing. This is the only genuinely new content.
2. The 13 code files the recorded review never covered: mocks and test files
   under-listed when the artifact was written. Establish whether each is
   CORRECT, not merely present.
3. The 12 covered files that changed after the artifact was recorded: confirm
   which are byte-identical reflows of the park and restore, and name any that
   are not.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 5 cleared that the cycle did not touch.

### Round 5 (closure pass)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `show bgp update-delay` declared NO answer shape. `registerShapes` loops `cmdBgpChildren` and registers an EMPTY shape for every child, which is right for a branch a plugin answers and wrong for the one child this package answers. `validateDeclaredShape` skips its shape test on an undeclared command, so every row operator was published over an answer with no rows, and `ze help command --json` said "with-rows" for it. The pre-existing `TestEveryShowBgpPathDeclaresAShape` was RED: `"show bgp update-delay" declares no answer shape` | `internal/component/bgp/plugins/cmd/peer/peer.go` `registerShapes` | fixed: the child is special-cased to `command.ShapeDoc`, mirroring the special case `registerColumns` already carries |
| 2 | NOTE (round 4, 1) | as above | `internal/component/plugin/types_bgp.go`, `internal/component/plugin/coordinator.go` | fixed: three comments corrected. `Reason` is named as the one field that is not the Go zero value, and why |
| 3 | NOTE (round 4, 2) | as above | `internal/component/bgp/reactor/reactor_api.go` | fixed: the `Stats` comment moved back onto `Stats` |
| 4 | NOTE (round 4, 3) | as above | `internal/component/bgp/reactor/update_delay_test.go` | fixed: `TestUpdateDelayReasonWords` pins all four words plus the `unknown` fallback as literals |

### Fixes applied
- `internal/component/bgp/plugins/cmd/peer/peer.go`: `registerShapes` declares `command.ShapeDoc` for `cmdBgpUpdateDelay` instead of declaring none, and the function's doc comment names the exception. `TestEveryShowBgpPathDeclaresAShape` is green.
- `internal/component/plugin/types_bgp.go`: the `Configured` field doc and the `ReactorIntrospector.UpdateDelayStatus` doc now say what the producers write.
- `internal/component/plugin/coordinator.go`: the no-reactor branch's comment no longer calls its answer "the zero value".
- `internal/component/bgp/reactor/reactor_api.go`: the misplaced `Stats` comment restored to `Stats`.
- `internal/component/bgp/reactor/update_delay_test.go`: `TestUpdateDelayReasonWords` added.
- `internal/component/bgp/plugins/cmd/peer/peer_shape_test.go` and
  `summary_test.go`: the shape declaration owed two DERIVED-population tables an
  entry, and both say so in their own prose. `childOwnDeclarations` now names
  `show bgp update-delay` with `command.ShapeDoc`, and the `branches` table in
  `TestChildCommandsDoNotInheritTheSummaryOrder` now carries its nine-column
  order. Neither is a relaxation: before the entry each table asserted the
  command declares NOTHING, and after it each pins the exact value the command
  declares. Found by `./le test-unit bgp` after the parked half was restored,
  which is the run that links the plugin registrations a bare `go test` leaves
  out.


### Round 2 scope (fixed before the round ran, 2026-09-08)

ONLY the fixes round 1 produced, plus the sibling call sites they touched:
the `expected map[*Peer]struct{}` identity set in `updateDelayHold` and its
filling from `peerSlice` in `reactor.go`, `trackLocked`,
`releaseIfConvergedLocked` and the deleted `expected < 1` guard in `arm`,
`peerEndOfRIB` and its driver in `reactor_notify.go`, `peerDown` and its driver
in `peer_run.go`, the two RFC 4724 exclusions (no GR capability, Restart State
bit), the `clock.AfterFunc` deadlines replacing the timer goroutine and
`stoppingLocked`, `ErrOpQueueFull` with `Peer.raiseOpQueueFull` and the
`reactor_api_batch.go` accounting change, `plugin.UpdateDelayStatus` on
`ReactorIntrospector` with `Reactor.UpdateDelayStatus`, the adapter, the
Coordinator and `show bgp update-delay` (`cmd/peer/update_delay.go`, its YANG
rpc and column order), the standalone `StartWithContext` arming path, the
amended spec Task/AC-1/AC-2, and the doc edits (`docs/features.md`,
`docs/comparison.md`, `docs/guide/configuration.md`,
`docs/features/bgp-protocol.md`, `docs/architecture/behavior/peer-lifecycle.md`)
plus the interop scenario's retimed `max-delay`.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything round 1 already cleared and the fixes did not touch,
including the route-server fast path and the `opRequireAbsent` proof string.


### Round 3 scope (fixed before the round ran, 2026-09-08)

ONLY the fixes round 2 produced, plus the sibling call sites they touched:
the `container update-delay` command node moved from
`yang/ze-bgp-cmd-peer-api.yang` into `yang/ze-peer-cmd.yang`,
`update_delay_registration_test.go`, `updateDelayPeer.owed` as a family SET with
`peerEndOfRIB(p, fam)` and the `eorFamily` binding in `reactor_notify.go`,
`stoppingLocked` on `releaseIfConvergedLocked`, `test/ui/bgp-update-delay-command.ci`
with `internal/test/fixture/ui_fixture_update_delay.go`, the
`wire-methods.snapshot` entry, the frozen `finalExpected`/`finalHeld`/
`finalConverged` in `releaseLocked` with `report` reading them, the narrowed
return of `releaseIfConvergedLocked`, the six mock types that gained
`UpdateDelayStatus`, and the `docs/guide/command-reference.md` row.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 and 2 cleared that these fixes did not touch,
including `peerDown`, the two RFC 4724 exclusions, the queue verbs' callers and
the timer re-entrancy.


### Round 4 scope (fixed before the round ran, 2026-09-09)

ONLY the fixes round 3 produced, plus the sibling call sites they touched:
the removal of `updateDelayHold.holding` and `.releaseReason` with the two test
helpers that now read `Reactor.UpdateDelayStatus` instead, the narrowed
VALIDATES block on `TestShowBgpUpdateDelayIsRegistered`, the
`plugin.UpdateDelayReasonNotReleased` declaration in `types_bgp.go` with its two
readers (`Coordinator.UpdateDelayStatus`'s no-reactor branch and
`updateDelayReleaseReason.String()`), the corrected comment in
`internal/test/fixture/ui_fixture_update_delay.go`, and the new ordering
paragraph at `peerEndOfRIB`.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 3 cleared that these fixes did not touch.

### Round 6
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | `Peer.updateDelayEndOfRIB` had NO production caller. `29102e14f8` deleted the call from the inbound marker decode while the declaration was parked, and the restore did not put it back, so the hold could release only on `establish-wait` or `max-delay` and the convergence path was severed | `internal/component/bgp/reactor/reactor_notify.go` | fixed: the call is back, and its comment now names it as the ONLY production caller |
| 2 | BLOCKER | The `bgp-update-delay-frr` interop scenario was not registered. The same commit removed the key and the restore did not return it, so the scenario directory named a checker nothing dispatched | `internal/le/interoplab/bgp/checkers.go` | fixed: the key and its non-vacuity note are back, taken from the removed side of `29102e14f8` |
| 3 | ISSUE | Nothing could have caught finding 1. Seven test functions call the hook directly, at fourteen call sites, and `bgp-update-delay.ci` asserts `reason=max-delay` and REJECTS `reason=converged`, so AC-2 had no functional test driving convergence through a daemon | `test/plugin/` | fixed: `test/plugin/bgp-update-delay-converges.ci` added |
| 4 | NOTE | Nine mocks answered an empty `Reason` where both producers write `UpdateDelayReasonNotReleased`, and the `cmd/peer` mock's comment claimed the zero value says "never configured" | `internal/component/bgp/plugins/cmd/peer/mock_reactor_test.go` | fixed: the mock completes an unset record the way a producer would, and the comment says so |
| 5 | NOTE | `TestBgpUpdateDelayReportsAnUnconfiguredDaemon` exercised a state no producer emits | `internal/component/bgp/plugins/cmd/peer/update_delay_test.go` | fixed: it now asserts `plugin.UpdateDelayReasonNotReleased` |

### Round 6 discrimination walk

`bgp-update-delay-converges` was forced RED before it was believed
(`ai/rules/interop-and-goal-validation.md`). The production call in
`reactor_notify.go` was replaced with `_ = eorFamily`, the plugin suite was
re-run, and the case went `FAIL` at 61.7s: the peer's marker reaches a hold that
is not listening, so `reason=converged` never appears and both wire expectations
time out. The call was restored and the case went `PASS` at 2.7s. The sibling
`bgp-update-delay` and `bgp-update-delay-validation` passed in both runs, which
is what says the new file is the one that discriminates.

Two facts the file records, because each cost a run to find. `ze-peer` MIRRORS
ze's capability set and refuses to state a capability ze does not offer, so ze
must declare graceful-restart for the peer to advertise it. And a
graceful-restart peer must attach a process permitted to put a route on the
wire: `validatePeerProcessCaps` (`internal/component/bgp/config/peers.go`) reads
`ProcessBinding.MayPushRoutes`, which `send [ update ]` and `send [ raw ]` each
satisfy. The peer block attaches `bgp-rib` with `send [ update ]`.

### Round 7 (owner-authorised, 2026-09-11)

Round 7 ENDED the loop: 0 BLOCKER, 0 ISSUE, 6 NOTE. It read the round 6 fixes
and the sibling call sites they touched, and it found no third loss from the
park cycle. It verified that by rebuilding the pre-park tree out of
`backups/bgp-update-delay-tracked-20260911-0115.patch`'s own blobs and comparing
it with the restored tree, which is the only comparison that can see a loss INTO
HEAD.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | The restored comment claimed "This is the one site that decodes an inbound marker". False: `onMessageReceived` and `onMessageBatchReceived` each decode one too. The load-bearing half, "the ONLY production caller of `Peer.updateDelayEndOfRIB`", is true | `internal/component/bgp/reactor/reactor_notify.go` | fixed at closure. The false sentence is replaced by one naming the plugin server's two decodes and saying that path never reaches the hold; the true sentence stays |
| 2 | NOTE | The header said "thirteen unit cases call the hook directly". It is 7 test functions over 14 call sites, and the file declares no subtest | `test/plugin/bgp-update-delay-converges.ci` | fixed at closure |
| 3 | NOTE | The same header recorded the cost as an attached process with `send [ update ]`. `validatePeerProcessCaps` reads `ProcessBinding.MayPushRoutes`, so `send [ raw ]` satisfies it equally, and the function's own comment says both | `test/plugin/bgp-update-delay-converges.ci`, `internal/component/bgp/config/peers.go` | fixed at closure, in the `.ci` header, the `.ci` config comment and this spec's two copies of the claim |
| 4 | NOTE | `RESTORE.md` still inventoried seven `.parked` files that the restore MOVED rather than copied, so the directory holds only that file | `backups/parked-bgp-update-delay-20260911/RESTORE.md` | fixed at closure: a status header says the restore is complete and names the patch that must be kept. `backups/` is git-ignored, so the edit rides in no commit |
| 5 | NOTE | `TestCheckerPopulationMatchesProducer` is RED, on `bgp-paths-limit-frr` | `internal/le/interoplab/bgp` | NOT this spec's. Its population comparison PASSED, which is what independently confirms `bgp-update-delay-frr` is registered |
| 6 | NOTE | Six mocks and `bgp-update-delay-converges.ci` were absent from the recorded review artifact, and three covered files' hashes no longer matched the tree | `tmp/review/bgp-update-delay-828bf96b-...md` | fixed at closure: the artifact is RE-RECORDED over the current tree. `./le spec session review check` printing `0 code files` was the absence of a pending commit, not coverage |

### Round 8 (closure pass, 2026-09-11)

The closure context ran two lenses over the whole diff and spawned no reader.

**Lens 1, reachability.** Every producer this spec added was traced to a
production caller, because that is the class the park cycle broke. `arm` from
`Reactor.StartPeers` (`reactor.go:746`) and `StartWithContext` (`:1312`), both
BEFORE the peer-start loop. `startInitialRoutes` from the FSM Established
callback (`peer_run.go:501`), and it is the ONLY production spawn of
`sendInitialRoutes`: the two `go p.sendInitialRoutes()` sites in the tree are
both in `update_delay.go`. `updateDelayPeerDown` from the leave-Established rail
(`peer_run.go:514`). `updateDelayEndOfRIB` from `reactor_notify.go:300`.
`ParseUpdateDelay` from `loader_create.go:96` for the value and `peers.go:316`
for the error. `show bgp update-delay` from `pluginserver.RPCRegistration{
WireMethod: "ze-bgp:update-delay"}` (`cmd/peer/peer.go:151`) to
`handleBgpUpdateDelay`, `ctx.Reactor().UpdateDelayStatus()`,
`reactorAPIAdapter.UpdateDelayStatus` and `Reactor.UpdateDelayStatus`.
`bgp-update-delay-frr` registered at `checkers.go:333`.

**Lens 2, claim against producer.** Every doc comment and doc sentence this diff
added was read against the function that produces the behavior. It found the
round 7 NOTE 1 claim repeated in two more places nobody had named: the
`peerEndOfRIB` doc in `update_delay.go` and the convergence paragraph in
`docs/architecture/behavior/peer-lifecycle.md`. Both are corrected, because the
unit that gets fixed is the PROBLEM and not the file the finding named
(`ai/rules/principles.md`).

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | The same false uniqueness claim round 7 found in `reactor_notify.go` is repeated in the `peerEndOfRIB` doc comment and in the peer-lifecycle page | `internal/component/bgp/reactor/update_delay.go`, `docs/architecture/behavior/peer-lifecycle.md` | fixed |

0 BLOCKER, 0 ISSUE. Every edit this pass made is prose; no product code changed,
which is why it earns no further round (`ai/rules/planning.md`, "A finding in the
record is not a finding in the product").

### Final status
- [ ] Round 5 re-check over its own fixes shows 0 BLOCKER, 0 ISSUE. The fix that
      earned round 5 is a one-line registration change, and the test that found
      the defect is what proves it: `go test -run TestEveryShowBgpPathDeclaresAShape
      ./internal/component/bgp/plugins/cmd/peer/` was RED before it and is green
      after, and the whole `cmd/peer` update-delay set runs green beside it.
- [ ] All NOTEs recorded above. The three round-4 NOTEs are FIXED rather than
      carried, and no NOTE is outstanding.
- [ ] Round 7 ended the loop: 0 BLOCKER, 0 ISSUE, 6 NOTE. Round 8, the closure
      pass, added one NOTE and fixed it. Every NOTE from both rounds is fixed
      except number 5, which is another session's red.
- [ ] Rounds 6 and 7 exist because of the PARK cycle, not because of the
      feature. Rounds 1 to 5 cleared the feature; the park reverted it from the
      tree and the restore lost two pieces, so the two extra rounds read what
      the cycle changed. Thomas authorised each of them individually on
      2026-09-11.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/update_delay.go` | yes | `ls -l` 2026-09-09 02:02, 27K |
| `internal/component/bgp/reactor/update_delay_test.go` | yes | `ls -l`, and `go test -run UpdateDelay ./internal/component/bgp/reactor/` is `ok` |
| `internal/component/bgp/config/update_delay.go` | yes | `ls -l` 2026-09-08 17:17, 4.1K |
| `internal/component/bgp/plugins/cmd/peer/update_delay.go` | yes | `ls -l` 2026-09-08 21:44, 2.9K |
| `test/plugin/bgp-update-delay.ci` | yes | `ls -l` 3.8K |
| `test/plugin/bgp-update-delay-validation.ci` | yes | `ls -l` 2.6K |
| `test/ui/bgp-update-delay-command.ci` | yes | `ls -l` 1.8K |
| `test/interop/scenarios/bgp-update-delay-frr/` | yes | `ls -l` holds `frr.conf` and `ze.conf` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the hold is armed before any peer can establish, and it owns the initial routing update | `Reactor.StartPeers` calls `r.updateDelay.arm(...)` BEFORE its `peer.StartWithContext` loop (read in `reactor.go`); `Peer.startInitialRoutes` returns without spawning when `holdInitialUpdate` answers true (`update_delay.go`) |
| AC-2 | convergence releases the hold | `releaseIfConvergedLocked` releases with `updateDelayConverged` once `settled == len(h.expected)`; `go test -run UpdateDelay ./internal/component/bgp/reactor/` `ok 0.874s` |
| AC-3 | `max-delay` releases with no peer up | `startDeadlinesLocked` arms `clk.AfterFunc(MaxDelay, release(updateDelayMaxDelay))`; the `.ci` asserts `reason=max-delay` and rejects `reason=converged` |
| AC-4 | establish-wait over max-delay is refused | `ParseUpdateDelay` returns the error at `establishWait > maxDelay`; `go test -run UpdateDelay ./internal/component/bgp/config/` `ok 2.143s` |
| AC-5 | absent or zero leaves the old path | `UpdateDelay.Enabled` is `MaxDelay > 0` and `arm` returns false on it; the three disable tests are in the same green run |
| shape | `show bgp update-delay` declares its answer shape | `go test -run TestEveryShowBgpPathDeclaresAShape ./internal/component/bgp/plugins/cmd/peer/` RED before the closure fix, `ok 0.563s` after |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `update-delay { max-delay 3 }` in a config a daemon loads | `test/plugin/bgp-update-delay.ci` | read: the file carries the config block, starts `ze -`, and asserts the two production log lines plus the two wire messages |
| a peer reaches Established while the hold runs | `test/plugin/bgp-update-delay.ci` | read: `ze-peer` connects on conn=1 and the route plus the End-of-RIB arrive only after the release |
| `establish-wait 40` with `max-delay 30` | `test/plugin/bgp-update-delay-validation.ci` | read: the file drives the refusal and the equal-value acceptance |
| an operator types `show bgp update-delay` | `test/ui/bgp-update-delay-command.ci` | read: it runs `ze-test fixture ui/bgp-update-delay`, which drives the SSH CLI and reads six fields out of the answer |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | There is no single outbound choke point. Read `peer_send.go` and `forward_rs.go`: `Peer.SendUpdate`, `SendAnnounce`, `sendWithdraw`, `sendRawUpdateBody`, `SendRawMessage` and the route-server fast path all reach the wire. The design does not need the assumption. Mistake Log row written |
| A-2 | confirmed | The hold withholds only the `sendInitialRoutes` spawn; the session read loop is untouched, and `reactor_notify.go` still decodes inbound markers while held, which is what settles a peer |
| A-3 | confirmed | The expected set is `peerSlice(peersToStart)` in `Reactor.StartPeers`, fixed at arm time and never grown (`updateDelayHold.expected`) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features.md`: the BGP row names `update-delay` and `show bgp update-delay` | anchor `<!-- source: internal/component/bgp/reactor/update_delay.go -- updateDelayHold -->`, and the type exists | yes |
| `docs/comparison.md`: a `Startup convergence hold (update-delay)` row | Ze `Yes`, every other daemon `?`. This diff also added the paragraph that DECLARES `?`, because the row is the page's first unsurveyed one: "A cell reads `?` when nobody has checked that daemon for that row. It is not a `No`." Claiming a Yes or a No for the other ten without reading their sources would be the fabrication the mark exists to avoid | yes |
| `docs/guide/configuration.md`: the container, both leaves, and the refusal | checked against `ze-bgp-conf.yang` (`range "0..3600"`, `range "1..3600"`) and against `ParseUpdateDelay`'s refusal text | yes |
| `docs/features/bgp-protocol.md`: the feature section and the startup-only note | `arm` returns false once `armed || released`, so a reload cannot re-arm; the page says the value takes effect at the next restart | yes |
| `docs/architecture/behavior/peer-lifecycle.md`: step 10 and the hold section | checked against `Peer.startInitialRoutes`, `trackLocked`'s two RFC 4724 exclusions and `updateDelayPeerDown` | yes |
| `docs/guide/command-reference.md`: the `show bgp update-delay` row and its three reason words | the words are `converged`, `establish-wait`, `max-delay` in `updateDelayReleaseReason.String()`, now pinned as literals by `TestUpdateDelayReasonWords` | yes |
| CLI reference for a new command | added; the row carries `<!-- source: ... -- handleBgpUpdateDelay -->` | yes |
| Doctor check for a new runtime dependency | not applicable: the feature adds no file path, socket, kernel module, port, binary or certificate. `grep -n "update-delay" internal/component/doctor/` returns nothing | yes |
| RFC status row | not applicable: RFC 4724 Section 4.1's deferral is offered for a COLD start here and no `rfc/short/rfc4724.md` support level changed. The RFC is cited in the code and in the pages, not claimed as newly proven | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
