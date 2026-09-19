# Spec: a-test-passes-at-any-concurrency

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**A functional test MUST pass at every concurrency, and the `plugin` suite does
not (owner ruling, 2026-09-19).** A suite that is green at 6 and red at 20 has a
timing design fault in the TESTING. The concurrency number only chooses how
often it bites, so lowering it is a band-aid and was refused as one.

Measured on 2026-09-19 at `1f207727e`, sequential runs on an idle 16-core box,
the same 731-case suite each time:

| Concurrency | Runs | Cases lost | Wall clock |
|-------------|------|-----------|------------|
| 20 (`DefaultParallelConcurrent`) | 3 | 2, 2, 2 | ~170s |
| 10 | 2 | 0, 2 | ~190s |
| 6 | 4 | 0, 0, 0, 0 | ~235s |

Fifteen-plus distinct cases failed across the session and almost none twice, so
this is ONE cause with many faces rather than a backlog of broken tests. The
same suite at `-p 6` lost 1 to 2 cases per run while OTHER heavy jobs shared the
box, which says the variable is total machine load rather than the suite's own
concurrency setting.

**What this spec must find is the mechanism, not a setting.** Two candidate
shapes are already excluded by measurement, and the spec must not re-propose
either:

- Widening the fixtures' wait bounds by the runner's contention factor
  (`ParallelFactorEnv`) made it WORSE: 5, 3, 5 lost against 2, 2, 2. A fixture
  that waits three times longer holds its daemon, peers and ports three times
  longer, and feeds the contention it was meant to survive.
- Lowering the concurrency is the band-aid this spec exists to replace.

**The protocol boundary is part of the question.** There is a load at which a
BGP speaker legitimately cannot answer inside a timer a peer is entitled to
enforce, and a test asserting on that timer is then right to fail. The spec must
say WHERE that boundary is for each failing case: a test whose fixture invents a
deadline the protocol does not owe is a test defect, and a test that measures a
real protocol timer under a machine too loaded to serve it is a capacity
finding. These are different repairs and must not be merged.

## Required Reading

### Architecture Docs
- [ ] The functional runner's parallelism and budget contract, page named by `ai/CODE-TO-DOCS.md` for `internal/test/runner/parallel.go`
  → Decision: [fill during research]
  → Constraint: [fill during research]
- [ ] `docs/functional-tests.md` - what a `.ci` may assume about the machine it runs on
  → Constraint: [fill during research]

**Key insights:** (minimal context to resume after compaction)
- The rate tracks total machine LOAD, not the `-p` value: the same `-p 6` lost cases while other jobs ran and lost none when idle.
- Widening fixture wait bounds is measured HARMFUL, not neutral.
- Two members were fixed at their producers and stayed fixed, so the population is not unfixable: `path-asn-show` (the daemon tore down with the End-of-RIB owed) and `ipv4-announce-withdraw` (the driver never sent the `plugin session ready` its peer's barrier waits for, so the asserted order held only while it beat a 2s timer).

**The mechanism, read at the producer on 2026-09-19.** An observer fixture runs
its scenario from `OnAllPluginsReady`, which reports that every PLUGIN is ready
and says nothing about any BGP session. It then fences with `request quiesce`
and asks the daemon to stop. That fence is `DrainPeerSync`, and `pendingSync`
is what it waits on: routes queued, an initial sync in flight, or the
End-of-RIB owed. A configured peer that has not yet established has none of the
three, so it is skipped and the quiesce answers `done`. The observer then tears
the daemon down while the peer is still in its OPEN exchange, and the marker
the `.ci` asserts is never sent. `show-bgp-bare-runs-summary` failed exactly
this way: its peer received OPEN, then NOTIFICATION (Cease), and no marker.

This is a zero-as-a-valid-answer guard (`ai/rules/principles.md`): "no work
pending" and "work not started" are the same reading. The exclusion is
DELIBERATE and correct for the quiescer's own job, because a peer that will
never come up must not hang a quiesce, and `setState` already names this hazard
where it closes the matching window on the other side of establishment. So the
defect is not in `pendingSync`. It is that the observer's shutdown turns on the
quiescer's answer, which was never a statement about session establishment.

**The population is 346 of 731.** That many `test/plugin/*.ci` cases assert the
End-of-RIB marker and drive a fixture, which is why 15-plus distinct cases
failed with almost no repeats: one hole, entered by whichever case lost the
race that run. Any repair measured on a handful of named cases is measuring
noise.

**The fact the fence needs is already declared, in the `.ci` itself.**
`option=tcp_connections:value=N` is the case stating how many BGP sessions it
expects, `internal/test/cli/cmd_exabgp.go` already publishes it to the daemon's
environment as `exabgp_tcp_connections`, and a fixture runs as a child of that
daemon. Nothing has to be invented or passed down by hand. What research still
owes is the terminal condition: N sessions reaching a settled outcome is not
the same as N markers, because a case that asserts a REFUSED session declares a
connection and owes no marker, and a fence written as "wait for N markers"
hangs those instead of failing them.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/test/runner/parallel.go` - `DefaultParallelConcurrent` is a fixed 20 whatever the machine has; `ParallelTimeoutHeadroom` widens each TEST's budget by 3 under concurrency; `ParallelFactorEnv` publishes that factor to children
- [ ] `internal/test/fixture/fixture.go` - `Poll` bounds a wait by an attempt count and a delay, and reads the contention factor NOWHERE
- [x] `internal/component/bgp/reactor/peer.go` - `pendingSync` is the fact `request quiesce` waits on. It reads three facts, and a peer that has not established yet has none of them, so it is reported settled
- [x] `internal/component/bgp/reactor/reactor_api.go` - `DrainPeerSync` and `peersSynced` are the quiescer behind `request quiesce`, and their own comments state the state-gating exclusion as deliberate
- [x] `internal/test/fixture/fixture.go` - `observeConfigured` runs the scenario from `OnAllPluginsReady`, quiesces, then asks the daemon to stop. This is the shared path all 346 marker-asserting cases take
- [x] `internal/test/cli/cmd_exabgp.go` - `exaBGPClientEnv` already publishes `exabgp_tcp_connections` into the daemon's environment

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every assertion each failing case makes today. This is a synchronisation repair, not a coverage reduction, and `test/weakened/` is not the route.

**Behavior to change:** (only what the user asked for)
- A functional test stops depending on how loaded the machine is

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le functional plugin`, and the same suite inside `functional/gating` where 27 suites share the box.

### Transformation Path
1. [fill during research] The runner's concurrency, each `.ci`'s own budget, the fixture's internal waits, and the daemon's own timers are four clocks over one run; name which of them each failing case actually depends on.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner ↔ fixture | the budget the runner measures against versus the deadlines the fixture enforces inside its own binary | No |
| Fixture ↔ daemon | `request quiesce`, `plugin session ready`, and the counters a fixture polls | No |

### Integration Points
- The failing population itself, which research must re-derive rather than inherit from this table

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | [fill during design] | [fill during design] |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | One mechanism explains most of the population | read at the producer 2026-09-19: the quiesce fence skips a not-yet-established peer, and 346 of 731 cases take that path | the repair is per-case after all, and the spec becomes a survey | the fix removing the loss at 20, not at 6 | mechanism confirmed, repair unvalidated |
| A-2 | Every failing case CAN be made load-independent | two were, at their producers | the residue is a capacity finding about the box, which is a different answer and must be stated as one | each case's own repair | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A repair trades one flake for another, as the bound widening did | the loss rate at 20 does not fall, or wall clock climbs | measure at 20 over at least three runs before and after, on an idle box |
| R-2 | The measurement is taken on a contended box and reads as a result | wall clock differs between runs of the same condition | run conditions sequentially, nothing else running; record the wall clock beside every count |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing an operator meets: this is the instrument. A wrong repair costs the gate its meaning, because a suite that loses a rotating 0.3% makes `debt-clear` a coin toss rather than a verdict |
| How is it reverted? | Per repair, each its own commit |
| Who else touches this path? | Every suite that runs under the same runner |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le functional plugin` at the default concurrency, three consecutive runs on an idle box | → | the repaired synchronisation | [fill during design: the measurement itself is the test, and it needs a home that runs it] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | the `plugin` suite at concurrency 20, three consecutive runs, idle box | 731 of 731 each run |
| AC-2 | the same suite at 6 and at 1 | the same result, so the assertion is on the fact and not on the schedule |
| AC-3 | each case repaired | its mechanism is named at the producer, and the repair states which of the two kinds it is: a fixture deadline the protocol never owed, or a real protocol timer the box could not serve |
| AC-4 | the population | every member is accounted for: repaired, or named as a capacity finding with the load it needs |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill during design] | `internal/test/fixture/` | a wait that cannot be satisfied by the passage of time alone | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the suite itself | `test/plugin/*.ci` | the population above, run at three concurrencies | |

## Files to Modify
- [fill during research] - the research phase names them, because naming them now would be the guess this spec exists to avoid

## Files to Create
- [fill during design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no leaf |
| CLI commands/flags | N-A | none |
| Functional test for new RPC/API | N-A | the tests exist; their synchronisation is the subject |
| Prometheus counters/metrics | No | no runtime surface changes |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, and the runner's parallelism page once research names it |
| 12 | Internal architecture changed? | [fill during design] | depends on where the repair lands |

## Implementation Steps

1. **Phase: Research** -- re-derive the population on an idle box (at least five runs at 20, recording every case and its wall clock), then read each failing case's fixture and name the deadline it depends on and who owns it
   - Verify: a table of every member, its mechanism at the producer, and which of the two kinds it is
2. **Phase: [fill during design]**

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | no assertion was dropped to reach green, and `test/weakened/` carries no row for this work |
| Data flow | each repair names the FACT its wait now turns on, never a duration |
| Rule: `ai/rules/testing.md` | a fence is a fence: the wait ends because the thing happened |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The suite is load-independent | three consecutive green runs at 20 on an idle box, and the same at 6 |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | none: this adds no external input |

### Failure Routing

| Failure | Route To |
|---------|----------|
| The loss rate at 20 does not fall | the mechanism was not the one found. Back to RESEARCH with the new population |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A fence widened is still a clock. Waiting longer under contention made the loss rate worse, because the wait itself is a resource.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Find the mechanism | lower `DefaultParallelConcurrent` to 6 | Measured green, and refused: a test that passes at 6 and fails at 20 is broken at both, and the number only sets the rate (owner, 2026-09-19) |

## Known Limitations

- The concurrency stays at its current value while this spec runs. Reaching a green sweep in the meantime uses 6, which is the band-aid this spec replaces and is not a result.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] Every assumption has a Basis and a validation method

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
