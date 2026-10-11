# Spec: bgp-fsm-delayopen-and-automatic-start -- DelayOpen and Event 3 in the BGP FSM

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The BGP child of the rfc-verdict pass (`plan/pre-release/spec-rfc-verdict-fix-bgp.md`,
"Gap accounting", around line 975) closed with requirement rows that no spec owned.
One spec took them (`plan/immediate/spec-bgp-unowned-rfc-rows.md`). On 2026-10-11 the
owner put DelayOpen and Event 3 IN and split the work: "Split into three". This
spec owns the two RFC 4271 FSM rows. The defects stay in the immediate spec; the
EPE rows are `plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md`.

Row numbers are the numbers the rows had in the parent spec.

| # | Row id | Document, section | Class | What Ze does today |
|---|--------|-------------------|-------|--------------------|
| 6 | `RFC4271-8.2.2-13` | RFC 4271 §8.2.2 (Active) | gap: OPTIONAL feature absent, owner IN | No DelayOpen, so Active never holds a connection and the Event 18 action list is reached by no layer |
| 7 | `RFC4271-8.2.2-20` | RFC 4271 §8.2.2 (Idle) | gap: optional event absent, owner IN | No Event 3; automatic restart is Event 6, which keeps the ConnectRetryCounter |

The class column follows `ai/rules/rfc-compliance.md`: "a capability Ze does not
provide is an implementation gap". The owner scoped both in, so both are
implemented here.

### Bucket

`plan/`. Both rows are OPTIONAL FSM behavior: the release goes out without
DelayOpen and with Event 6 as its automatic start, and no operator meets either
as a bug. `plan/README.md` puts that at this level. The owner's split put them here.

### The RFC text, quoted from the source

| # | Quote | Read at |
|---|-------|---------|
| 6 | "If the local system receives a TcpConnectionFails event (Event 18), the local system: - restarts the ConnectRetryTimer (with the initial value), - stops and clears the DelayOpenTimer (sets the value to zero), - releases all BGP resource, - increments the ConnectRetryCounter by 1," and §8.2.1.3: "If the flag indicating support for an optional timer (DelayOpen or DampPeerOscillations) cannot be set to TRUE, the timers and events supporting that option do not have to be supported." | `rfc/full/rfc4271.txt` §8.2.2 Active, §8.2.1.3 |
| 7 | "In response to a ManualStart event (Event 1) or an AutomaticStart event (Event 3), the local system: - initializes all BGP resources for the peer connection, - sets ConnectRetryCounter to zero," and §8.1.2 Event 3 "Status: Optional, depending on local system" | `rfc/full/rfc4271.txt` §8.2.2 Idle, §8.1.2 |

### How FRR and BIRD handle it

| # | FRR (`/Users/thomas/Code/github.com/FRRouting/frr`) | BIRD (`/Users/thomas/Code/gitlab.nic.cz/labs/bird`) |
|---|------|------|
| 6 | DelayOpen implemented (`bgpd/bgp_fsm.c`, `bgp_delayopen_timer`); configured per neighbor as `neighbor X timers delayopen (1-240)` (`bgpd/bgp_vty.c` line 8204) | No DelayOpen; restart pacing is `error_delay_time_min/max` backoff (`proto/bgp/bgp.c` around line 1018), the same shape as Ze's Event 6 |
| 7 | No RFC ConnectRetryCounter tracked; FRR counts `peer->dropped` | No RFC ConnectRetryCounter |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/behavior/fsm.md` - names DelayOpen and the timers Ze does not implement
  → Constraint: rows 6 and 7 are IN (owner, 2026-10-11), so this page changes with the FSM phase: DelayOpen, the DelayOpenTimer and Event 3 move from "not implemented" to implemented, in the same commit as the FSM code
- [ ] `docs/architecture/testing/interop.md` - scenario naming and the revert-and-red walk
  → Constraint: `bgp-delayopen-frr` is named (no numeric prefix) and owes a recorded red with the fix reverted and the artifact rebuilt

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - rows 8.2.2-13 (line 811), 8.2.2-20 (line 805)
  → Constraint: main-thread rulings R47(a) and R53(a) (`plan/handover/rfc-verdict-test-fix-pass/RULINGS.md`) hold 8.2.2-20 as a counted gap because Ze has an automatic start, and 8.2.2-13 as a `{gap}` stamped unimplemented because the gate refused `{feature-declined}` beside existing tags. Both are now implemented (owner: IN), so both rows lose `{gap}` and owe tags in both polarities; R50 found the HEAD-tagged 8.2.2-13 units drive a synthetic Event 18 no real Peer reached, so those tags are re-pointed at a test that reaches Active with DelayOpen through a real Peer

**Key insights:**
- Row 7's Event 3 and the existing Event 6 must be chosen from the backoff state Ze already keeps, not from a second timer.
- Row 6 is the only wire-visible change: the moment Ze sends its OPEN.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/fsm/fsm.go`, `internal/component/bgp/fsm/state.go` - Event 6 implemented as the reactor's exponential backoff (state.go line 162); Events 3 and DelayOpen absent by recorded design (fsm.go header lines 16-18, 77)
- [ ] `internal/component/bgp/reactor/session.go` - line 1006-1011 fires Event 6 for an automatic restart
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - grouping `peer-fields`, container `timer` (`receive-hold-time`, `send-hold-time`, `keepalive`, `connect-retry`); no DelayOpen leaf

**Behavior to preserve:**
- Event 6 backoff semantics and `TestRFC4271ConnectRetryCounterSurvivesDampedStart`.
- With `timer delay-open` unset (0), the OPEN timing and every existing FSM test unchanged.

**Behavior to change:** rows 6 and 7, as the ACs state.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- FSM: the peer run loop's automatic restart and TCP events (rows 6, 7).
- Config: `bgp peer <address> timer delay-open` (row 6).

### Transformation Path
1. Config parse of `timer delay-open` into the peer settings.
2. The reactor's run loop picks Event 3 or Event 6 from the backoff state on an automatic restart (row 7).
3. The FSM runs the DelayOpenTimer in Connect and Active, holding Ze's OPEN until the peer's OPEN or expiry (row 6).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to reactor | peer settings carry the DelayOpen value | No |
| Reactor to FSM | Event 3 / Event 6 and the DelayOpen events | No |

### Integration Points
- `ze-bgp-conf.yang` grouping `peer-fields` - the new leaf reaches peer, group and bgp levels.
- `session.go` automatic restart - the Event 3 / Event 6 choice.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be shown at implementation |
| No unintended coupling (components stay isolated) | No | to be shown at implementation |
| No duplicated functionality (extends existing, does not recreate) | No | Event 3 reads the existing backoff state, no second timer |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire encoding change |
| Registration over hardcoding, outbound | N-A | no registry involved |
| Registration over hardcoding, inbound | N-A | no registry involved |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | FRR's `neighbor X timers delayopen` lets an interop scenario run DelayOpen on both sides | `bgpd/bgp_vty.c` line 8204, read in the parent spec | `bgp-delayopen-frr` covers Ze-side DelayOpen only, and the scenario README says so | build the FRR scenario with both sides set | holds by source reading (2026-10-11) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Row 7 changes restart pacing: zeroing the counter on Event 3 hides oscillation the counter used to show | a flapping peer restarts faster after the change | Event 3 only when the backoff is not damping; a damped restart keeps Event 6 and its counter (AC-2) |
| R-2 | Row 6 delays Ze's OPEN; a peer that also waits never establishes | a session stuck in Active or Connect | The DelayOpenTimer always expires and Ze then sends its OPEN; the interop scenario sets DelayOpen on both sides |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Row 6: session establishment timing with `timer delay-open` set (OPEN held, Event 18 in Active). Row 7: restart pacing and the ConnectRetryCounter |
| How is it reverted? | Each row lands as its own commit, revertible alone |
| Who else touches this path? | `plan/pre-release/spec-rfc-verdict-fix-bgp.md` (ledger rows, the R50 re-pointed tags), `plan/immediate/spec-bgp-open-session-rfc-defects.md` (OPEN and session handling) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `bgp peer <address> timer delay-open 5`, peer connects | → | FSM DelayOpenTimer, OPEN held, Event 18 in Active | `test/plugin/bgp-delay-open.ci` |
| Peer session restarts automatically with no oscillation | → | FSM Event 3 zeroes ConnectRetryCounter | `TestRFC4271AutomaticStartEvent3ZeroesCounter` (`internal/component/bgp/reactor/`, through a real `Peer`; not wire-visible) |

## Acceptance Criteria

AC numbers changed in the split: AC-1 and AC-2 were AC-6 and AC-7 in the parent
spec; AC-3 is the parent's ledger AC-11 for the `rfc4271` stem.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 (row 6) | `bgp peer <address> timer delay-open <seconds>` is set (owner: IN) | The DelayOpen attribute is TRUE and Ze holds its OPEN after the TCP connection until the peer's OPEN arrives or the DelayOpenTimer expires (RFC 4271 §8.2.2 Connect and Active); `0`, the default, leaves DelayOpen FALSE and today's behavior unchanged. A TcpConnectionFails (Event 18) in Active, reached through a real `Peer` with DelayOpen running, restarts the ConnectRetryTimer, stops and clears the DelayOpenTimer, releases the connection's resources and increments the ConnectRetryCounter, the four quoted actions. A value outside the leaf's range is refused at commit with the leaf, the range and the fix |
| AC-2 (row 7) | Ze restarts a peer session automatically and the peer has not been oscillating (owner: IN) | The FSM receives Event 3 (AutomaticStart) and the ConnectRetryCounter is set to zero; a restart while the backoff is damping keeps today's Event 6 behavior and keeps the counter (`TestRFC4271ConnectRetryCounterSurvivesDampedStart` stays green). The choice between Event 3 and Event 6 is read from the backoff state, never from a second timer |
| AC-3 (ledger) | Each row once its AC is met | The row in `rfc/short/rfc4271.md` loses `{gap}`, carries tags in both polarities, the HEAD-tagged 8.2.2-13 units are re-pointed at the real-`Peer` test (R50), and each carries a discrimination record (`./le rfc discriminate-record`); `./le rfc check` exits clean for `rfc4271` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `timer delay-open` on a peer facing FRR | config -> FSM DelayOpenTimer -> OPEN held -> session up | `bgp-delayopen-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4271DelayOpenHoldsOpen` | `internal/component/bgp/fsm/` | AC-1: OPEN held until peer OPEN or timer expiry; 0 leaves DelayOpen off | |
| `TestRFC4271ActiveEvent18WithDelayOpen` | `internal/component/bgp/reactor/` | AC-1: Event 18 in Active through a real `Peer`, four actions, both polarities | |
| `TestRFC4271AutomaticStartEvent3ZeroesCounter` | `internal/component/bgp/reactor/` | AC-2 both polarities (undamped zeroes, damped keeps) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `timer delay-open` (seconds) | 0 (off) or 1..240, FRR's range | 240 | N/A (0 is valid: off) | 241 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-delay-open` | `test/plugin/` | AC-1 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-delayopen-frr` | `test/interop/scenarios/` | FRR (`neighbor X timers delayopen (1-240)`) | With `timer delay-open` set on Ze, the capture shows Ze's OPEN only after FRR's OPEN (or after the DelayOpenTimer), and the session establishes; with FRR's own delayopen set as well, both hold and the session still establishes | |

Row 7 is not wire-visible; its proof is the reactor unit test through a real `Peer`. The scenario owes the revert, rebuild, red, restore, green walk with the red recorded.

## Files to Modify
- `internal/component/bgp/fsm/fsm.go`, `internal/component/bgp/fsm/state.go`, `internal/component/bgp/reactor/session.go` - rows 6, 7 (the fsm.go header comment that records DelayOpen and Event 3 as absent changes with the code)
- `internal/component/bgp/yang/ze-bgp-conf.yang` - row 6: grouping `peer-fields`, container `timer`, new leaf `delay-open` (uint16 seconds, range `0 | 1..240`, default 0 = DelayOpen FALSE), so `bgp peer <address> timer delay-open <seconds>` and the group and bgp levels through the grouping
- `docs/architecture/behavior/fsm.md` - rows 6, 7
- `rfc/short/rfc4271.md` - AC-3

## Files to Create
- `test/plugin/bgp-delay-open.ci`
- `test/interop/scenarios/bgp-delayopen-frr/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-bgp-conf.yang` leaf `timer delay-open` |
| YANG validation constraints | Yes | `delay-open` range 0 or 1..240 |
| YANG custom validators | N-A | the range is a YANG constraint |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | Yes | the new leaf comes from YANG |
| Functional test for new RPC/API | N-A | no new RPC; `bgp-delay-open.ci` covers the behavior |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | No | the FSM state metrics already exist; confirm at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/bgp-protocol.md` if its evidence names these rows (check `./le feature report`) |
| 2 | Config syntax changed? | Yes | `docs/config-reference.md` and `docs/guide/configuration.md`, which list the `timer` leaves (`connect-retry`) |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-peering.md` if it describes session timers |
| 7 | Wire format changed? | No | the OPEN is unchanged; only its timing |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` and `docs/features/rfc-status.md` (generated by `./le rfc index-update`) |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists DelayOpen |
| 12 | Internal architecture changed? | Yes | `docs/architecture/behavior/fsm.md` (rows 6, 7 implemented) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/behavior/fsm.md` (affected). Re-derive at implementation with `./le spec citation anchors spec plan/spec-bgp-fsm-delayopen-and-automatic-start.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `timer` examples in the pages of item 2 |

## Implementation Steps

1. **Phase: Wiring** -- `bgp-delay-open.ci` and the Event 3 reactor test written and red.
2. **Phase: DelayOpen (row 6)** -- YANG leaf, config, DelayOpenTimer in Connect and Active, Event 18 in Active; `fsm.md` edited. Own commit.
3. **Phase: Event 3 (row 7)** -- the automatic restart picks Event 3 or Event 6 from the backoff state. Own commit.
4. **Phase: interop** -- `bgp-delayopen-frr` with the recorded red.
5. **Phase: ledger** -- AC-3, discrimination records, `./le rfc check` clean for `rfc4271`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | `delay-open 0` leaves every existing FSM test green |
| Correctness | A damped restart still keeps the ConnectRetryCounter |
| Rule: rfc-compliance | Each new implementing function quotes its RFC sentence; each caller names the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `{gap}` left on rows 6, 7 | `grep -n` each id in `rfc/short/rfc4271.md` |
| Both polarities tagged | `grep -rn "RFC requirement: <id>" internal/ test/` |
| Ledger clean | `./le rfc check` exit status for `rfc4271` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `delay-open` range enforced at commit |
| Resource exhaustion | A held connection is bounded by the DelayOpenTimer and the hold timer |
| Error leakage | none: no peer-supplied bytes parsed differently |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Owner decisions (2026-10-11)

| Question | Owner's words | What it means here |
|----------|---------------|--------------------|
| Row 6: scope DelayOpen in, or decline it? | IN | AC-1: a `timer delay-open` leaf, the DelayOpenTimer, and `bgp-delayopen-frr` |
| Row 7: add Event 3, or record Event 6 as Ze's automatic start by design? | IN | AC-2: Event 3 when the backoff is not damping |
| One spec or several? | "Split into three" | This spec owns rows 6 and 7; the defects stay in `plan/immediate/spec-bgp-unowned-rfc-rows.md`, EPE is `plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md` |

## Open Questions (owner)

None open.

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `delay-open` range 0 or 1..240, 0 meaning off | A separate enable leaf | FRR's range; one leaf carries both the attribute and the timer |
| Event 3 vs Event 6 read from the backoff state | A second restart timer | Ze already decides damping there; a second timer would be a second declaration of the same fact |

## Known Limitations
- None beyond the FRR/BIRD comparison: BIRD has no DelayOpen, so the interop proof is against FRR only.

## RFC Documentation (Scope: protocol)

Add `// RFC 4271 Section 8.2.2: "<quoted requirement>"` above enforcing code, using
the quotes in the Task section.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-bgp-fsm-delayopen-and-automatic-start.md` only, in the same `./le commit create` script
