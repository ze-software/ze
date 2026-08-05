# Spec: wire-edit-4-api-origin-deferred-bird-interop -- a live BIRD peer accepts an API-originated route

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/4 |
| Deferral shard | `plan/deferrals/wire-edit-4-api-origin-deferred-bird-interop.md` |
| Updated | 2026-08-05 |

Deferral holder created at the closure of `plan/learned/1320-wire-edit-4-api-origin.md` on 2026-08-02
(`ai/rules/planning.md`, "Creating the Deferral Spec"). The source spec
was removed by its closure commit, so the work below lives here.

## Task

`plan/learned/1320-wire-edit-4-api-origin.md` converged the two announce rails on one
writer. Its interop row was not reached in the implementation session.

The property is currently proven by unit tests over both rails and by
`test/plugin/wire-edit-api-origin-order.ci`, which pins the exact wire bytes
through the daemon. A live peer is stronger evidence and is still owed: an
interop scenario in which BIRD accepts an API-originated route and installs the
attributes in the expected order.

`ai/rules/interop-and-goal-validation.md` makes an interop test mandatory for a
protocol feature, so this is an owed deliverable, not an optional extra.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `ai/rules/interop-and-goal-validation.md` - required interop assertion per feature type
- [ ] `test/interop/scenarios/` - the scenario layout and `check.py` contract

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` (the announce rail under test)
- [ ] `internal/component/bgp/reactor/announce_build.go` (the writer that owns attribute ORDER)
- [ ] `test/plugin/wire-edit-api-origin-order.ci` (the byte-level property this must confirm against a real peer)

**Behavior to preserve:** the emitted bytes are already pinned; the scenario must confirm them against BIRD, never relax them.

### Read at design time, 2026-08-05

→ Constraint: the ordering property is produced by `(*announceAttrs).emit`
(`internal/component/bgp/reactor/announce_build.go`), not by the API rail. It calls
`sortByCode()` and then merge-inserts each contributed attribute at the first base
position whose code sorts after it. `(*reactorAPIAdapter).buildBatchAnnounceUpdate`
(`internal/component/bgp/reactor/reactor_api_batch.go`) only assembles the plan and
hands it to that writer. A scenario that proves order proves `emit`.

→ Constraint: the announce rail decides three attributes on the SESSION KIND, so an
eBGP scenario does not carry the attribute set the existing `.ci` pins.

| Attribute | iBGP, as the `.ci` pins it | eBGP, what BIRD would see |
|-----------|---------------------------|---------------------------|
| AS_PATH (2) | empty, synthesized by `announceASPathASNs` | carries the local AS; `prependApplies` gates the prepend |
| LOCAL_PREF (5) | synthesized at 100, `localPrefAllowedTo(isIBGP)` true | STRIPPED; `localPrefAllowedTo` returns false and `emit` drops it |
| COMMUNITIES (8), LARGE_COMMUNITIES (32) | caller-supplied, unchanged | caller-supplied, unchanged |

→ Decision: this makes the iBGP-versus-eBGP choice a design decision, not a detail.
`test/plugin/wire-edit-api-origin-order.ci` runs iBGP (`asn local 65000 remote 65000`)
and pins the block `1,2,3,5,8,32`. The four existing BIRD scenarios are eBGP, where
the same route emits `1,2,3,8,32` instead.

→ Constraint: the property the byte-level `.ci` pins is the INJECTED attributes
landing before the caller's, not merely "ascending". The rail's old defect emitted
`1,8,32,2,3,5` on one rail and `1,2,3,5,8,32` on the other. A scenario whose route
carries only caller attributes that already sort last cannot discriminate, which is
what R-1 is about.

## Data Flow (MANDATORY)

### Entry Point
A process plugin in the ze container writes TWO `peer * update text ...` commands,
each carrying COMMUNITIES and LARGE_COMMUNITIES on its own prefix. The two take
the two different announce rails (A-6). Which rail each one takes is fixed by
configuration and asserted before the announce, never left to timing.

### Transformation Path

1. The plugin script imports `ready`, `flush`, `wait_for_shutdown` from `ze_api`
   (`test/scripts/ze_api.py`, staged at `/usr/lib/ze/ze_api.py` by
   `test/interop/Dockerfile.ze`).
2. Each text command reaches `(*reactorAPIAdapter).AnnounceNLRIBatch`
   (`internal/component/bgp/reactor/reactor_api_batch.go`), which routes on
   `Peer.ShouldQueue`.
3a. QUEUE rail, `10.55.0.0/24`, announced before the session establishes. That
   ordering is CONFIGURED, not timed: `connect false` makes ze passive
   (`internal/component/bgp/reactor/config.go` reads it into
   `PeerSettings.Connection`) and `connect delay time 30` holds BIRD's dial, so
   the only route to Established opens 28s after the announce (measured). The
   route is stored in the RIB and replayed by `buildRIBRouteUpdate`
   (`internal/component/bgp/reactor/peer_rib_routes.go`), which passes a `nil`
   base and contributes the caller's attributes as plan entries.
3b. BATCH rail, `10.55.1.0/24`, announced behind `wait_peer_eor_sent`:
   `buildBatchAnnounceUpdate` assembles the plan with the caller's block as the
   base, and the rail contributes ORIGIN, AS_PATH and NEXT_HOP.
4. `(*announceAttrs).emit` (`internal/component/bgp/reactor/announce_build.go`)
   sorts by type code and merge-inserts each contribution into the base.
5. BIRD 2.15.1 receives both UPDATEs and installs both routes.
6. `check.py` reads them back with `birdc show route for <prefix> all`.

### Boundaries Crossed
| From | To | Format |
|------|----|--------|
| Plugin script | ze daemon | `ze_api` text command over the process-plugin pipe |
| ze daemon | BIRD | BGP UPDATE, eBGP 65001 to 65002 |
| BIRD | `check.py` | `birdc` text output, parsed by the scenario |

### Integration Points
| Point | Component |
|-------|-----------|
| Scenario discovery | `run.py`, which lists `test/interop/scenarios/` and requires `check.py` |
| BIRD container start | `Scenario.setup` (`test/interop/interop.py`), keyed on `bird.conf` being present |
| Check invocation | `Scenario.run_check`, which calls `mod.check()` with no arguments |
| BIRD query | `BIRD._birdc_quiet`, the raw escape hatch six scenarios already use |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | BIRD's route dump exposes attribute values in a form the check can assert. | Existing scenarios under `test/interop/scenarios/` assert against BIRD output. | Assert on session survival plus the installed prefix and next-hop only, and say so. | Ran BIRD 2.15.1 from `test/interop/Dockerfile.bird` with a route carrying both community types, 2026-08-05. | **confirmed** |
| A-2 | The two community types print in DIFFERENT punctuation, so one literal form cannot serve both. | Same probe: `BGP.community: (65001,100)` has no space, `BGP.large_community: (65001, 0, 1)` has spaces after each comma. `52-send-community-suppress-frr` `_check_bird_keeps_standard_only` asserts the no-space form and never asserts a large-community value. | A large-community assertion written `(65001,0,1)` never matches, and the check is silently vacuous. | The probe above; the scenario itself re-confirms on every run. | **confirmed** |
| A-3 | An eBGP BIRD peer accepts an API-originated route and the announce rail needs no code change. | `31-multihop-ebgp-bird/announce-routes.py` already originates through `ze_api` to a BIRD peer. | The spec grows a code change and stops being test-only. | `python3 test/interop/run.py 55-wire-edit-api-origin-bird` passed on 2026-08-05 with `git diff internal/` empty. | **confirmed** |
| A-4 | The caller's attribute block reaches `(*announceAttrs).emit` as the BASE, so ignoring it drops the caller's communities. | `parsedAttrs.snapshot` (`internal/component/bgp/plugins/cmd/update/update_text.go`) encodes the caller's attributes with `Builder.Build()` and wraps them with `attribute.NewAttributesWire`; that value becomes `NLRIGroup.Wire`, then `NLRIBatch.Wire`, and `buildBatchAnnounceUpdate` takes `case batch.Wire != nil` FIRST, setting `base = batch.Wire.Packed()`. | The AC-3 mutation is inert and the discrimination evidence is vacuous. | True for the BATCH rail only, and measured there: `rail=batch/buildBatchAnnounceUpdate base-bytes=26 base-codes=[8 32] plans=3 plan-codes=[1 2 3]` (instrumented containerised run, 2026-08-05). See A-6 for the rail it is false on. | **confirmed, batch rail only** |
| A-5 | The `Builder.SetWire` / `RawWire` reasoning governs this rail. | Design-time reading of the `case batch.Attrs != nil` branch of `buildBatchAnnounceUpdate`. | Any conclusion drawn from it about THIS scenario is void. | `SetWire` genuinely has no non-test caller, but the text rail never reaches that branch: it sets `Wire`, never `Attrs` (`DispatchNLRIGroups`, same file). The fact is true and the branch is unreachable here. | **broken** |
| A-6 | The scenario reaches `buildBatchAnnounceUpdate`, so one mutation of `emit` falsifies the whole check. | The Data Flow written at design time, which names that function and no other. | The recorded discrimination is rail-specific and the spec's Wiring Test names a function the scenario never calls. | FALSE as first written. `Peer.ShouldQueue` (`internal/component/bgp/reactor/peer.go`) decides the rail, and an announce one second after `ready()` lands before establishment: the route is queued and replayed by `buildRIBRouteUpdate` (`internal/component/bgp/reactor/peer_rib_routes.go`), which passes `nil` base at its `plan.emit` call. Measured `rail=queue/buildRIBRouteUpdate base-bytes=0 plans=5 plan-codes=[1 2 3 8 32]`. Resolved by announcing a SECOND prefix behind the `wait_peer_eor_sent` barrier, which reaches the batch rail; both rails are now measured in one run. | **broken, then resolved** |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A scenario that only asserts the session stays up proves nothing about attribute order. | The check passes against a deliberately broken encoder. | **Realized, and resolved by scoping.** Attribute ORDER is not observable from BIRD's dump at all (see Key Design Decisions), so this spec does not claim it. The claim is acceptance and value delivery, whose mutation test is AC-3. Order is homed at `plan/spec-interop-wire-capture.md`. |
| R-2 | An ABSENCE assertion reads green whether or not the behavior holds. | The check asserts "BIRD does not show X". | Do not write one. `54-local-pref-strip-gobgp` measured this: FRR skips LOCAL_PREF during parse, so a leaked attribute is invisible in its RIB and an absence assertion is green either way. Every assertion here is a positive one over an exact value. |
| R-3 | Announcing before the session establishes loses the UPDATE, and the check reads a missing route as a broken encoder. | Intermittent reds with no route at all. | Follow the existing BIRD scenarios: the plugin sleeps after `ready()`, and the check calls `wait_session("ze_peer")` before `wait_route`. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| An operator announces a route with communities through the API, before the session establishes | -> | `AnnounceNLRIBatch` -> `QueueAnnounce` -> `buildRIBRouteUpdate` -> `(*announceAttrs).emit`; BIRD installs `10.55.0.0/24` | `test/interop/scenarios/55-wire-edit-api-origin-bird/check.py` |
| The same, on an established peer whose initial sync has drained | -> | `AnnounceNLRIBatch` -> `buildBatchAnnounceUpdate` -> `(*announceAttrs).emit`; BIRD installs `10.55.1.0/24` | same file |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two API-originated routes, one per announce rail, each carrying COMMUNITIES and LARGE_COMMUNITIES on ONE prefix | BIRD installs both prefixes and reports both values on each, each in its own punctuation (A-2), read from the `BGP.community:` and `BGP.large_community:` LINES |
| AC-2 | The same routes | BIRD's append-only stderr carries exactly ONE `ze_peer: State changed to up`, so the peer held one session across both announces and sent no NOTIFICATION over either attribute block. The check reads it with `docker logs --tail 2000` (`docker_logs`, `test/interop/interop.py`); BIRD writes about five lines per run |
| AC-3 | Two mutations of `(*announceAttrs).emit`, one per rail: nil `base` on entry, and keep only the plan entries with code < 8 | Each FAILS the check on the community assertions of ITS rail's prefix and leaves the other prefix green, proving the check discriminates on both rails (`ai/rules/interop-and-goal-validation.md`). Evidence is a RUN of each build |
| AC-4 | The scenario announces two prefixes BIRD did not previously hold | `wait_route` finds both, so the pass is not inherited from another scenario's state |

~~**AC-3's mutation SITE changed at implementation time, 2026-08-05, and the reason
is A-4.** The row asked for `emit` to ignore `spans`. `base` is
`batch.Attrs.RawWire()`, which is the Builder's raw-wire escape hatch, and
`Builder.SetWire` has no non-test caller. An API-originated batch therefore
reaches `emit` with an EMPTY base, and every attribute -- the caller's included --
arrives as a plan contribution. A build that nils `base` logged
`base-bytes=0 plans=5` and the scenario stayed GREEN, so that mutation proves
nothing.~~

**VOID, corrected at the round 1 review gate, 2026-08-05.** The paragraph above
reasons over the `case batch.Attrs != nil` branch, which THIS rail never reaches.
`Builder.SetWire` having no non-test caller is true and irrelevant here.

The producer chain, read end to end at the gate: `parsedAttrs.snapshot`
(`internal/component/bgp/plugins/cmd/update/update_text.go`) encodes the caller's
attributes with `Builder.Build()` and wraps them with
`attribute.NewAttributesWire`. That value becomes `NLRIGroup.Wire`, and
`DispatchNLRIGroups` in the same file sets `NLRIBatch{Wire: group.Wire}` and never
sets `Attrs`. `buildBatchAnnounceUpdate` takes `case batch.Wire != nil` FIRST, so
`base = batch.Wire.Packed()`.

**The caller's COMMUNITIES and LARGE_COMMUNITIES are therefore BASE SPANS, not plan
entries.** The plan holds ORIGIN, AS_PATH and NEXT_HOP only, every code already
below 8, so "keep the plan entries with code < 8" removes nothing and the scenario
stays green under it. That mutation is inert and its recorded RED cannot have come
from it. Every AC-3 measurement taken before this correction is VOID and is being
re-run.

**The paragraph above is itself HALF RIGHT, and the missing half is A-6.** It reads
`buildBatchAnnounceUpdate` correctly: on that function `case batch.Wire != nil`
does come first and the caller's block IS the base. What it assumes without
measuring is that the scenario CALLS that function. It did not. The one-prefix
scenario announced one second after `ready()`, before the session established, so
`Peer.ShouldQueue` sent the route to `QueueAnnounce` and the initial-sync drain
emitted it through `buildRIBRouteUpdate`, whose `plan.emit` call passes a literal
`nil` base.

Instrumented containerised run, 2026-08-05, one line per rail:

```
rail=queue/buildRIBRouteUpdate       base-bytes=0  base-codes=[]     plans=5 plan-codes=[1 2 3 8 32]
rail=batch/buildBatchAnnounceUpdate  base-bytes=26 base-codes=[8 32] plans=3 plan-codes=[1 2 3]
```

So both earlier MEASUREMENTS were right and both EXPLANATIONS were wrong: nil
`base` was inert and code < 8 was RED, because the scenario ran the queue rail.
The scenario now announces a second prefix behind the `wait_peer_eor_sent`
barrier, which reaches the batch rail, and each mutation falsifies its own rail's
prefix.

**Lesson, and it is the reason this survived two verification passes:** reading a
producer is not enough when the code has two branches and only one is on the path,
and reading the right branch is not enough when the caller never reaches that
function. `ai/rules/evidence.md` asks which function PRODUCES the behavior. Which
function that is was decided here by SCHEDULING, not by the command text, and only
an instrumented run of the real scenario could say which one ran.

**AC-3 replaces the row this spec was created with.** The original asked the check
to fail against the pre-convergence encoder, whose defect was attribute ORDER
alone. That is unreachable from BIRD, and the reason is recorded under Key Design
Decisions. ~~AC-3: The scenario reverted against the pre-convergence encoder / The
check fails, proving it discriminates~~ (superseded 2026-08-05, Thomas ruled to
land acceptance now and home the order proof at `plan/spec-interop-wire-capture.md`).

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (interop scenario, not a Go unit test) | `test/interop/scenarios/55-wire-edit-api-origin-bird/check.py` | AC-1, AC-2, AC-4 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing `test/plugin/wire-edit-api-origin-order.ci` | `test/plugin/` | the daemon emits the attribute block in ascending type-code order (unchanged by this spec, and still the only proof of ORDER) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `55-wire-edit-api-origin-bird` | `test/interop/scenarios/` | BIRD 2.15.1 | a live peer accepts an API-originated route on EACH announce rail and installs both community types on each | PASS 2026-08-05 |

## Files to Modify
- `internal/component/bgp/reactor/reactor_api_batch.go` - re-read only; the scenario must not need a code change (A-3)
- `test/interop/interop.py` - added at the round 3 fix pass. `docker_run` gained an
  `env=` parameter and the ze container receives `SESSION_TIMEOUT`, so a process
  plugin derives its barrier from the harness budget. `wait_containers_healthy`
  reports a `ZE-OBSERVER-FAIL` sentinel through the new `raise_if_observer_failed`,
  which is where the queue-rail guard's own failure lands
- `docs/architecture/testing/interop.md` - documents both, per checklist row 10
- ~~`docs/functional-tests.md` - test infrastructure gained a scenario (Documentation Update Checklist row 10)~~
  (struck 2026-08-05, round 1 review) Row 10 reads "New test **tools or patterns**"
  (`ai/rules/writing.md`). This scenario introduces neither: it uses the existing
  scenario pattern unchanged. `docs/functional-tests.md` names BGP interop as a
  suite and enumerates no scenario, and `docs/architecture/testing/interop.md`
  carries no row for any scenario past 37. No doc update is owed.

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The scenario proves existing behavior; it adds none |
| 4 | API/RPC added/changed? | No | The `ze_api` text command is unchanged; the scenario is a new caller of it |
| 7 | Wire format changed? | No | A-3: no daemon code changes, so no emitted byte changes |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 4271 Section 5 ORDER is explicitly NOT proven here (Known Limitations). No `RFC requirement:` tag is added, so no `rfc/short/` or `docs/features/rfc-status.md` row moves |
| 10 | Test infrastructure changed? | **Yes** (round 3) | `docs/architecture/testing/interop.md`. The scenario itself still uses the existing pattern, but the round 3 fix pass added two harness capabilities: `docker_run(env=)` with `SESSION_TIMEOUT` reaching the ze container, and `raise_if_observer_failed`, which reports a process plugin's `ZE-OBSERVER-FAIL` message instead of the symptom it causes |
| 16 | Any changed source file referenced by existing doc source anchors? | No | No source file changed |
| 2, 3, 5, 6, 8, 11-15, 17 | - | N-A | No config, CLI, plugin, guide page, SDK, comparison, arch, metadata, metric, inventory or example surface is touched |

## Files to Create
- `test/interop/scenarios/55-wire-edit-api-origin-bird/ze.conf` - eBGP 65001 to BIRD 65002, plus the `plugin` and `process` blocks, plus `connect false` (round 3) so ze listens and never dials
- `test/interop/scenarios/55-wire-edit-api-origin-bird/bird.conf` - the `ze_peer` protocol, `import all`, matching the four existing BIRD scenarios, plus `debug { states }` so BIRD writes one `State changed to up` trace per establishment (AC-2), plus `connect delay time 30` (round 3) so the establishment is a configured barrier rather than a race
- `test/interop/scenarios/55-wire-edit-api-origin-bird/announce-api-origin.py` - the `ze_api` plugin that announces ONE prefix per rail, each with both community types
- `test/interop/scenarios/55-wire-edit-api-origin-bird/check.py` - defines `check()`, taking no arguments

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- create the scenario directory with all four
   files and confirm the runner discovers it.
   - Tests: `python3 test/interop/run.py 55-wire-edit-api-origin-bird`
   - Verify: the runner starts a ze container and a BIRD container, and reaches
     `check.py`. It is expected to FAIL at the assertions, not at setup.
2. **Phase: The announce** -- the plugin announces one prefix carrying both
   community types, and the check asserts the session plus the installed prefix.
   - Verify: AC-2 and AC-4 pass.
3. **Phase: The value assertions** -- assert each community value in ITS OWN
   punctuation (A-2). Standard has no space after the comma, large has a space
   after each.
   - Verify: AC-1 passes. Deliberately write the large-community literal in the
     no-space form ONCE and confirm the check FAILS; that is what proves the
     assertion is reading real output rather than matching anything.
4. **Phase: Discrimination (AC-3, BLOCKING)** -- instrument `(*announceAttrs).emit`
   and both of its production call sites, run the scenario, and record which rail
   each prefix took with its base and plan codes. Then run BOTH mutations, one per
   rail, rebuilding the ze image for each, and confirm each turns ITS prefix red.
   REVERT everything and confirm the scenario passes again. Paste every output.
   - Verify: `ai/rules/interop-and-goal-validation.md` is satisfied by evidence,
     not by assertion. `git diff internal/` carries nothing from this spec.

### Critical Review Checklist

<!-- Added 2026-08-05. Omitted at design time; the implementation agent flagged
     the gap. Feature-specific only: the generic checks in ai/rules/quality.md
     always apply and are not repeated. -->

| Check | What to verify for this spec |
|-------|------------------------------|
| Discrimination is measured, not asserted | AC-3's evidence is a RUN of the mutated build, with output, and a second run after the revert. A note saying "this would fail" is not evidence |
| The mutation site is the real one | There is no single one: A-6 measured that the site depends on WHICH RAIL the announce took, and the rail is decided by `Peer.ShouldQueue`, not by the command. Both mutations are owed, one per rail, and the instrumented run naming each rail is the evidence that the scenario reaches both |
| No absence assertions | Every assertion is positive over an exact value. An assertion that BIRD does NOT show something is green whether or not the behavior holds (R-2, and `54-local-pref-strip-gobgp` measured it). AC-2 counts `State changed to up` at exactly one rather than asserting no error line: BIRD CLEARS `Last error` on re-establishment (measured 2026-08-05), so the absence form would carry the same defect as `session_established` |
| The empty dump fails closed | If BIRD holds no route, the value checks must not pass by matching nothing (`ai/rules/evidence.md`) |
| Both community punctuations | Standard has no space after the comma, large has a space after each. One literal form cannot serve both (A-2) |
| The daemon is untouched | `git diff internal/` carries nothing from this spec. The scenario proves existing behavior; it does not change it (A-3) |
| The prefix is unique to this scenario | A pass must not be inherited from another scenario's leftover state (AC-4) |

## Review Gate

**Round 1 scope, written before the round ran (2026-08-05):** the whole diff.
`test/interop/scenarios/55-wire-edit-api-origin-bird/` (all four files), this spec,
`plan/spec-interop-wire-capture.md`, and
`plan/deferrals/wire-edit-4-api-origin-deferred-bird-interop.md`. Two lenses:
vacuity and discrimination; harness wiring and blast radius.

| Round | Lens | Severity | Finding | Resolution |
|-------|------|----------|---------|------------|
| 1 | vacuity | **BLOCKER** | AC-3's recorded mutation is inert: the caller's communities are BASE spans on this rail, not plan entries, so filtering plan entries with code < 8 removes nothing. AC-3 had no valid discrimination evidence | Verified independently at the producer chain, which the reviewer got right. A-4 corrected to `confirmed`, new A-5 records the false branch, AC-3's site restored to nilling `base`, and the evidence is being re-measured |
| 1 | vacuity | ISSUE | AC-2 asserts `bird.session_established` (`test/interop/interop.py`), which samples the CURRENT state. A NOTIFICATION bounces the session, ze re-announces, and both `wait_route` and the final sample pass. The property survives its own falsification | Fix: assert on `show protocols all ze_peer` with an empty `Last error`, or on session uptime against the announce age |
| 1 | vacuity | NOTE | `_check_communities` matches against the whole dump rather than the `BGP.community:` / `BGP.large_community:` lines. Safe while the prefix carries one path | Tighten to the attribute lines; cheap and removes the future trap |
| 1 | wiring | ISSUE | `docs/functional-tests.md` named in Files to Modify but never touched | Row struck with the verified reason: checklist row 10 is "new test tools or patterns", and this uses the existing pattern |
| 1 | wiring | ISSUE | Neither spec carried a Documentation Update Checklist | Added to both, every row answered |
| 1 | wiring | NOTE | `plan/spec-interop-wire-capture.md` named a deferral shard that does not exist | Set to `-`; nothing is deferred from it yet |
| 1 | wiring | NOTE | "nine scenarios already use `_birdc_quiet`" was wrong; the real count is six | Corrected. The number came from a research report and was repeated without checking |
| 1 | wiring | - | No BLOCKER. Runner discovery, BIRD start, bind-mount, `check()` signature, file modes, config shape, deferral-gate compliance and `__pycache__` ignoring all verified against source | - |

**The BLOCKER's real cause, settled by measurement after the fix pass.** Both
earlier measurements were correct and both explanations were wrong, the main
thread's included. `AnnounceNLRIBatch` routes on `Peer.ShouldQueue`
(`internal/component/bgp/reactor/reactor_api_batch.go`), and the plugin announced
one second after `ready()`, before the session established. The route therefore
took the QUEUE rail and was replayed by `buildRIBRouteUpdate`
(`internal/component/bgp/reactor/peer_rib_routes.go`), which passes a LITERAL nil
base to `emit`. So the caller's attributes were plan entries after all, on that
rail: "plan code < 8" reds it and nilling `base` is inert, exactly as first
measured. On the BATCH rail the opposite holds, exactly as the reviewer measured.

**The scenario was proving a rail this spec never named.** The Wiring Test and Data
Flow both named `buildBatchAnnounceUpdate`, which the original one-prefix scenario
never reached. Rather than narrow the claim, the scenario now announces one prefix
per rail: `10.55.0.0/24` before establishment (queue) and `10.55.1.0/24` behind
`wait_peer_eor_sent` plus `quiesce` (batch). Each mutation falsifies its own rail
and leaves the other green, which is stronger evidence than the spec originally
asked for.

**Round 2 scope, written before the round ran (2026-08-05):** only round 1's fixes
and what they touched. `check.py` (the DISCRIMINATION block, the rewritten session
assertion, `_attr_line`), `announce-api-origin.py` (the two-rail announce),
`bird.conf` (`debug { states }`), and the spec rows corrected at the gate (A-4,
A-5, A-6, AC-1 to AC-4, Data Flow, Wiring Test). The eight always-in-scope classes
apply at any round.

| Round | Lens | Severity | Finding | Resolution |
|-------|------|----------|---------|------------|
| 2 | rail-selection | **BLOCKER** | Rail selection is an unasserted race. Changing only `time.sleep(1)` to `time.sleep(25)` made the scenario PASS with a daemon provably broken on the queue rail, because both prefixes then take the batch rail. `Peer.ShouldQueue` (`internal/component/bgp/reactor/peer.go`) is true while state is not Established, or `sendingInitialRoutes != 0`, or `opQueue` is non-empty, and none of that is observable from a scenario. The ~6s cushion is an accident of BIRD starting after ze plus connect-retry backoff | Fixed in round 3: `announce-api-origin.py` now fails unless `peer_counter("eor-sent")` is still 0 before the queue announce, so losing the race is loud instead of silent |
| 2 | rail-selection | ISSUE | `api.quiesce()`'s return value was discarded, and it is the ONLY guarantee of the batch rail: `wait_peer_eor_sent` does not imply `ShouldQueue` is false, because `sendingInitialRoutes.Store(0)` happens later in the post-EOR drain. A silent False puts the batch prefix on the queue rail and makes mutation A inert | Fixed in round 3: `runtime_fail` on False, matching `API.assert_no_leak` |
| 2 | rail-selection | ISSUE | The plugin's 60s EOR barrier (`attempts=240, delay=0.25`) is shorter than the harness `SESSION_TIMEOUT` of 90s. Establishment taking 60-90s makes the plugin shut ze down while the check still waits, so the red surfaces two hops away at `wait_route` | Fixed in round 3: barrier derived from `SESSION_TIMEOUT` |
| 2 | discrimination | - | **No BLOCKER, and AC-3 is now proven.** Both mutations run with in-binary probes at each `emit` call site: mutation A reds only the batch prefix, mutation B reds only the queue prefix, and the probe lines confirm `base-bytes=0 plans=[1 2 3 8 32]` on the queue rail against `base-bytes=26 plans=[1 2 3]` on the batch rail. A-4, A-5, A-6, Data Flow and the Wiring Test all verified accurate | - |
| 2 | discrimination | ISSUE | The AC-3 evidence procedure can produce a false green. `build_images` tags a SHARED `ze-interop` image while containers are per-PID, so a concurrent build swaps the daemon under a run. The reviewer's first mutation run PASSED for exactly this reason | Procedure fixed in round 3: every recorded mutation result names its image digest. The harness defect itself is homed at `plan/spec-interop-image-tag-race.md` with a deferral row |
| 2 | discrimination | NOTE | `bird.check_route` after `bird.wait_route` is tautological; `wait_route` only returns when the route is present. It printed a green tick reading as independent verification | Fixed in round 3 |
| 2 | discrimination | NOTE | `_attr_line`'s docstring overstates: it returns the FIRST matching line, so two paths for one prefix would let path 1 satisfy it. Fail-closed on a missing line is correct and measured | Docstring corrected in round 3 |
| 2 | both | NOTE | AC-2, `bird.conf` and `_check_session_never_bounced` all call the BIRD log "cumulative"; `docker_logs` runs `docker logs --tail N`. Measured volume is ~5 lines so truncation is unreachable, but the wording claims a guarantee the code does not give | Wording corrected in round 3 |

**Round 3 scope, written before the round ran (2026-08-05):** only round 2's fixes
and what they touched. `announce-api-origin.py` (the `eor-sent` guard, the
`quiesce` check, the barrier), `check.py` (the tautological call, two docstrings),
`bird.conf` (one comment). The eight always-in-scope classes apply at any round.

| Round | Lens | Severity | Finding | Resolution |
|-------|------|----------|---------|------------|
| 3 | guard correctness | **BLOCKER** | The `eor-sent` guard added in round 3 FAILS OPEN. `API.peer_counter` (`test/scripts/ze_api.py`) ends `return total if seen else default`, and `default` is 0. Any miss -- a dispatch error, a selector matching no row, a missing `peers` key -- returns 0, which is exactly the value the guard reads as "the sync has not drained". Demonstrated: `sleep(25)` plus a selector matching no row PASSES again, both prefixes on the batch rail, queue rail untested. This is round 2's BLOCKER restored through the guard meant to close it | Pending round 4. `ai/rules/evidence.md`: a zero value must never be a valid-looking answer. Pass a sentinel default so a miss denies |
| 3 | guard correctness | ISSUE | The barrier is still a constant, not derived. `SESSION_TIMEOUT` is read from the environment (`test/interop/interop.py`), so a host setting it above 120 puts the plugin's 120s barrier back under the harness budget and the red surfaces two hops away. `docker_run` passes no `-e` but does accept `extra_args` | Pending round 4 |
| 3 | guard correctness | ISSUE | The plugin comment says `eor-sent` is "cleared per session by `ClearStats`". `ClearStats` (`internal/component/bgp/reactor/peer_stats.go`) has one non-test caller, `(*Peer).cleanup`, and `IncrEORSent`'s own contract calls it a per-peer LIFETIME counter that accumulates across flaps. Harm direction is fail-closed, so the guard survives, but a reader trusting "per session" mis-derives | Pending round 4 |
| 3 | guard correctness | ISSUE | The comment claims exhaustiveness it does not have: two more paths reach `sendingInitialRoutes.Store(0)` with `eor-sent` untouched (the EOR-send failure `break`, and `ClaimInitialSyncEOR` false for every family). Neither yields a silent green, because `wait_peer_eor_sent` then spins out, but the stated implication is not exact | Pending round 4 |
| 3 | guard correctness | NOTE | `ZE-OBSERVER-FAIL` has no consumer in the interop harness; the implicit reject lives in the `.ci` runner's `validateLogging`. The run reds either way, so the guard stays falsifiable, but the sentinel surfaces two hops from the printed failure | Pending round 4 |
| 3 | guard correctness | - | Verified clean: `quiesce()` IS the right batch-rail observable (`PendingSync` is exactly the non-state half of `ShouldQueue`); removing `bird.check_route` lost nothing, since `wait_route` raises on absence; the corrected docstrings match the code | - |
| 3 | CI flakiness | **BLOCKER** | The scenario reds about 1 run in 6 on an IDLE machine, and the failing case is the COLD run. Six consecutive runs: run 1 failed, 2 to 6 passed. The whole timing cushion is ~5.7s, most of it BIRD's default connect delay; `ready()` (`test/scripts/ze_api.py`) can spend 20s in `wait_for_config` plus `wait_for_registry`, both of which return silently on timeout. `.github/workflows/evidence-nightly.yml` is the only automated caller of the interop suite and starts fresh nightly, so the cold path IS the CI path | Fixed in round 4 by removing the race rather than widening it. The suggested BIRD-side delay alone was insufficient, because ze dials too (`(*Peer).run` retries 5s to 60s): `ze.conf` now sets `connect false` so ze is passive, and `bird.conf` sets `connect delay time 30`, making BIRD's dial the only route to Established. Measured barrier 28.4s against ~5.7s of luck, 11 consecutive passes |
| 3 | CI flakiness | ISSUE | All three plugin-side failure modes surfaced as `BIRD route not found`. `runtime_fail` gets its loudness from the `.ci` runner's `validateLogging`, and the interop harness has no such check | Fixed in round 4, in the shared harness rather than per scenario: `observer_fail_line` / `raise_if_observer_failed` (`test/interop/interop.py`) |
| 3 | CI flakiness | ISSUE | 120s barrier is correct against the default `SESSION_TIMEOUT` and wrong against the knob, which is environment-read | Fixed in round 4: `docker_run` gained `env=`, the ze container receives `SESSION_TIMEOUT`, and `EOR_ATTEMPTS` derives from it |
| 3 | CI flakiness | NOTE | `time.sleep(1)` bought nothing the guard did not and spent ~1s of a ~5.7s budget | Removed in round 4 |
| 3 | CI flakiness | NOTE | Cleanup is sound; startup ordering is structural but the cushion was luck | Superseded: the cushion is now configured |

**A reviewer destroyed another agent's work during round 3** (`ai/rules/never-destroy-work.md`).
Cleaning its own scratch, it ran a recursive delete on `tmp/review55`, a directory it
had entered with `mkdir -p` but which already held sixteen measurement logs belonging
to a different agent. It declared the violation itself. Damage assessed: the
load-bearing AC-3 evidence survives in `tmp/interop-55-*.log` (both mutations, both
two-rail runs, the final clean runs); what was lost was that reviewer's own
falsification and flake logs, whose measurements are preserved in its written report
and are re-derivable by re-running. Every later agent was told explicitly not to clean
any `tmp/` directory it did not create.

**Round 4 scope, written before the round ran (2026-08-05):** round 3's fixes and the
shared-harness change they introduced. `announce-api-origin.py` (the `_peer_row_or_fail`
guard, the derived barrier), `ze.conf` (`connect false`), `bird.conf` (`connect delay
time 30`), `check.py`, and `test/interop/interop.py` plus
`docs/architecture/testing/interop.md`. Attributing every red in the interop suite was
made a BLOCKING deliverable of this round, because a shared-harness change cannot ship
on unexplained reds.

| Round | Lens | Severity | Finding | Resolution |
|-------|------|----------|---------|------------|
| 4 | blast radius | - | **No BLOCKER, and the shipping condition is met: every red attributed.** `05-routes-from-frr`, `06-routes-from-bird`, `09-route-withdrawal-frr`, `10-ipv6-ebgp-frr` and `11-addpath-frr` all fail IDENTICALLY with HEAD's `interop.py`. The harness change caused none of them. `raise_if_observer_failed` sits after the loop's `return`, on a path that already ended in `raise`, so it cannot convert a PASS; `docker_run(env=)` is genuinely optional | - |
| 4 | blast radius | ISSUE | The new `wait_containers_healthy` call site did NOT fire in the one real case available to test it. On `11-addpath-frr` ze stopped 6s in with the sentinel in its log, but the waiter had already returned green, and the run still spent 90s reporting the wrong cause. Whether it fires is a race | Pending round 5. Move or add the call so it actually catches the fast mode: the generic waiters, or `run.py`'s `except BaseException` |
| 4 | blast radius | ISSUE | The shared-home ARGUMENT is falsified by that measurement, though the shared home itself is still right: `env=` cannot work anywhere else, and 26 scenarios can emit the sentinel via `sys.excepthook`. Only the scenario-local `check.py` call caught the fast mode | Pending round 5. Correct the comment, which states an implication that did not hold |
| 4 | blast radius | ISSUE | `observer_fail_line` is SILENT on an unreadable log. `docker_logs` returns `"(docker logs timed out)"` on timeout and `""` on any other docker failure, neither of which holds the sentinel, so the helper returns None without a word. Round 3's BLOCKER shape, again | Pending round 5. Fail closed, and say so (`ai/rules/evidence.md`) |
| 4 | blast radius | ISSUE | `lines=2000` is an unstated truncation bound; the doc says the helper "reads the sentinel from Ze's log" with no bound | Pending round 5 |
| 4 | blast radius | NOTE | `Ze.rib_count` returns 0 when `ze show bgp rib status` fails, which is why 05/06 print "0 received routes" instead of naming the real fault. The shared-harness twin of round 3's `peer_counter` BLOCKER, and its own docstring records the identical failure on 2026-08-04 | Pre-existing, not caused here. Homed with the five reds |
| 4 | guard / flake | - | **No BLOCKER.** The guard fails CLOSED in every enumerated unreadable state (dispatch error, non-`done` status, missing `peers` key, selector miss, peer absent, `state` outside the vocabulary, non-integer `eor-sent`), and was FALSIFIED live in both directions: round 3's fail-open demo now reds, and a deliberately-late variant reds. `connect false` traced into the FSM. 10 consecutive passes including a throttled-CPU run. `SESSION_TIMEOUT` injection verified by `docker inspect` | - |
| 4 | guard / flake | ISSUE | The establishment barrier is the one constant round 3 did not derive, and it has an undocumented floor: `SESSION_TIMEOUT=20` fails 100% naming no cause, and the `bird.conf` comment asserted a relation it did not read | Fixed in round 5: `_check_session_budget` (`check.py`) READS the delay out of `bird.conf` and fails early naming the floor, so the floor cannot drift from the config it guards |
| 4 | guard / flake | NOTE | A millisecond-wide fail-open window exists in the guard by construction, between `PeerStateEstablished` and `sendingInitialRoutes` being set (`internal/component/bgp/reactor/peer_run.go`). Unreachable behind the 28s barrier | Recorded in round 5 as the reason the guard is a BACKSTOP and `connect false` plus `connect delay` is the mechanism |
| 4 | guard / flake | NOTE | `wait_route(BATCH_PREFIX)` succeeding is itself positive proof that prefix did NOT take the queue rail: `opQueue` is drained only inside `sendInitialRoutes` and nothing re-drains it in a live session. A rail proof the scenario was not claiming | Written into `check.py` in round 5 |
| 4 | guard / flake | NOTE | The guard is single-shot where sibling helpers poll; failure direction is a false RED | Recorded in round 5 |

**Round 5 fixes applied (2026-08-05), NOT YET REVIEWED.** All five round 4 ISSUEs
and the NOTEs are fixed: `observer_fail_line` fails closed and says so when the log
cannot be read; the observer-fail recovery moved to `run.py`'s failure path, which is
the one site that sees EVERY scenario failure while the containers are still up; the
truncation bound is stated; the `SESSION_TIMEOUT` floor is read from `bird.conf` and
checked; line-number citations converted to symbols (`ai/rules/writing.md`).

**Scenario verified PASSING by the main thread after those edits**, all ten
assertions, `NO_BUILD=1 python3 test/interop/run.py 55-wire-edit-api-origin-bird`.

**Round 5's ISSUE 2 fix is PROVEN, on the exact case round 4 said the old placement
missed.** Round 4 measured `11-addpath-frr` failing with only
`FRR session ... not Established` after 90s, two hops from its cause. The same
scenario now reports both:

```
✗ FAIL: FRR session with 172.30.2.2 not Established
✗ the cause is a process plugin: ... ZE-OBSERVER-FAIL: uncaught observer
  exception: RuntimeError: RPC error ...: unexpected token 'path-information'
```

That also shows the recovery catches a sentinel from `sys.excepthook`
(`test/scripts/ze_api.py`), not only an explicit `runtime_fail`, which is the
26-scenario surface the shared home was chosen for. The `11-addpath-frr` red itself
is pre-existing and homed at `plan/spec-interop-suite-red.md`, where its root cause
is now recorded.

| Round | Lens | Severity | Finding | Resolution |
|-------|------|----------|---------|------------|
| 5 | (NOT RUN -- weekly usage budget exhausted 2026-08-05) | | Round 5's fixes are unreviewed. The gate is OPEN | Resume here |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The scenario claims ACCEPTANCE and value delivery, not attribute ORDER | Claim order anyway; add packet capture to the harness first | BIRD 2.15.1 accepts any order and `birdc show route ... all` prints in BIRD's own canonical order, so reverting the encoder leaves the dump identical. A check written over the dump would be green either way, which is the vacuity trap named in `ai/rules/interop-and-goal-validation.md`. Thomas ruled on 2026-08-05: land acceptance now, home capture at `plan/spec-interop-wire-capture.md` |
| eBGP, matching the four existing BIRD scenarios | iBGP, matching `test/plugin/wire-edit-api-origin-order.ci` | Every BIRD scenario in the lab is eBGP 65001/65002 and the containers are addressed for it. iBGP would need a new lab shape for no gain here. The consequence is recorded: LOCAL_PREF is absent from what BIRD sees, by RFC 4271 Section 5.1.5 |
| No assertion on LOCAL_PREF absence | Assert BIRD shows no `local_pref`, since the eBGP strip is wire-visible | `54-local-pref-strip-gobgp` measured the trap directly: FRR skips LOCAL_PREF during parse, so a LEAKED attribute is invisible in its RIB and the absence assertion passes either way. BIRD is a receiving daemon with the same RFC 4271 Section 5.1.5 receive obligation, so the assertion would be unfalsifiable here too. The strip already has a live-peer witness in scenario 54, which uses GoBGP precisely because it reports what ARRIVED |
| One prefix carries BOTH community types | Two prefixes, one per type, as `15-community-frr` does | AC-1 is about one route carrying both. Splitting them would leave the combined attribute block, which is the thing the announce rail assembles, untested against a live peer |
| One prefix per RAIL, both carrying the full attribute set | One prefix, whichever rail scheduling happened to pick | A-6: the rail is decided by `Peer.ShouldQueue`, so a single-prefix scenario silently covers one rail and the other's mutation reads as inert. Two prefixes make both rails measured in one run and give each rail its own falsifying mutation. The cost is one extra prefix and the `wait_peer_eor_sent` barrier |
| AC-2 counts `State changed to up` in BIRD's log | `bird.session_established`; an empty `Last error` in `show protocols all` | Both sample state that a bounce restores. `session_established` reads the CURRENT state, and BIRD CLEARS `Last error` when the session re-establishes (measured on BIRD 2.15.1, 2026-08-05). `docker logs` is append-only, so a second establishment cannot be taken back. Falsified by a forced `birdc restart ze_peer`, which turned the count to 2 while `session_established` still passed |

## Known Limitations

- Attribute ORDER on the wire is not proven here and cannot be, from BIRD's route
  dump. It stays proven by `test/plugin/wire-edit-api-origin-order.ci` at the byte
  level, and the live-peer half is homed at `plan/spec-interop-wire-capture.md`
  with a row in this spec's deferral shard.
- The scenario proves BIRD's behavior only. FRR and GoBGP reach the same rail
  through other scenarios; this one does not widen peer coverage.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] `make ze-verify` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
