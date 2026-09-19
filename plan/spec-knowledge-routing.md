# Spec: knowledge-routing

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Find durable knowledge that still lacks a governing home, route it there, and
measure the documentation gaps that prevent routing. New lessons follow the
direct-routing rule in `ai/rules/planning.md`: update the governing surface and
use a journal row only when no surface governs the lesson. This spec must not
make closure write a summary before routing it.

The 889-summary population below is an August measurement. The current
`plan/learned/` directory contains numbered records 001 through 026 and the
learned indexes; it is not that backlog. Before a pilot, inventory the current
eligible records, check whether their lessons already reached their homes, and
record the population and the instrument used. The original 30-to-50-record
pilot, five routed examples and agreement threshold remain requirements.
If the eligible population cannot meet them, obtain an owner decision on the
pilot scope; do not manufacture records or lower the thresholds.

The residual proposal is a documentation-gap audit and, if an unrouted
population exists, an age report and guard for it. The routing taxonomy and
AC-1 through AC-10 remain below. Their implementation starts only after the
population and the meaning of "unrouted" are defined under direct routing.

### Historical population, August 2026

The closed knowledge-0 umbrella retired summaries 1-400 on an age band.
The previous pass measured 78% dead paths in band 1-200 and recorded 889
remaining summaries. Those figures motivated this proposal; they do not
describe the current corpus or authorize another retirement pass.

### The routing taxonomy

The following taxonomy identifies candidate homes. A routing verdict must
follow the current governing rule and be verified at its destination.

| Content | Canonical home |
|---------|----------------|
| A design decision, why the code is shaped this way | `docs/architecture/<subsystem>.md` |
| A recurring trap an agent must avoid | a rule under `ai/rules/` |
| An invariant governing ONE function | a comment at that function (`ai/rules/protocol.md`) |
| A protocol obligation | `rfc/short/rfcNNNN.md` |
| How data flows through a subsystem today | `ai/digests/<subsystem>.md` |
| An abandoned approach and why it failed | `plan/learned/DESIGN-HISTORY.md` |
| Hook or tooling friction | `plan/learned/HOOK-FRICTION.md` |
| Nothing anyone will need again | deleted; git history keeps it |

### The finding that matters more than the pruning

A record whose durable content has no suitable destination identifies a
documentation gap. The pilot must report each such item and the document
needed to hold it, rather than force it into a nearby page.

### Evidence that routing beats storing

Verified 2026-08-02. Summary 290 (buffered TCP read) was deleted in the age-band
retirement. Its knowledge survives, because someone had copied it into
`internal/component/bgp/reactor/session_connection.go` as an `INVARIANT` comment
naming the lock ordering, the readers that depend on the triple, and the crash
that follows from splitting the assignments. The routed copy outlived the stored
one by five months, and the stored one was about to be deleted unread.

## What the previous session learned, and what it changes here

These are not anecdotes. Each one changes a design decision in this spec.

| Lesson | What it changes |
|--------|-----------------|
| **A gate can assert its own bug.** `test_core_is_derived_not_hardcoded` asserted the fail-open behaviour of a ladder parser, so a header reword would silently have dropped `git-safety` and `never-destroy-work` from the always-on rule core, forever green | Every test for a guard in this spec asserts the REFUSAL, never the fallback. Stated as a Critical Review row, not left to judgement |
| **A ratchet with slack looks armed and is not.** A shrink-only baseline recorded at 1,011 against a real count of 341 left 670 references of room to regrow | The routing gate asserts its ceiling EQUALS the measured count, never merely bounds it |
| **Three spec defects were found by IMPLEMENTING, none by reviewing**: a checker path that did not exist (a test written there would have run nowhere), an AC requiring a token its target file never contained, an AC that never named its measuring instrument so two readings straddled the threshold | Every path this spec names is checked to exist before the spec is marked `ready`. Every numeric AC names its instrument |
| **An agent report is a claim.** A reviewer reported an RFC 4724 conformance violation from a stale summary and a vacuous test; reading the producing function showed the code conformant | Routing verdicts are verified against the TARGET file, never inferred. "Already documented" must be proven by opening the doc |
| **Discovery decays first.** The previous session built tools to make knowledge discoverable and left three of them undiscoverable | Every artifact this spec creates lands in `ai/INDEX.md` in the same phase that creates it |
| **The tool that writes must restore what it found.** `tempfile.mkstemp` creates at 0600; 228 summaries shipped with narrowed permissions, invisible to `git diff` | Any tool that rewrites a file preserves its mode, with a test |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
- [ ] The closed knowledge-0 umbrella (record retired with the learned corpus) - the previous pass and its measurements
  → Constraint: the August dead-path measurements and zero-slack guard describe that pass. Current `journal.ValidateFile` validates row structure, dates and spec cells; it does not enforce that old citation ceiling.
  → Decision: age-band retirement is historical completed work and is not repeated here.
- [ ] `ai/rules/writing.md` - governs anything written into `docs/`
  → Constraint: every factual claim carries a source anchor and is verified against code BEFORE it is written. A routed line is a factual claim.
- [ ] `ai/rules/writing.md` - governs the size of a routed line
  → Constraint: one to three lines, merged into an existing section. Appending a section per summary moves the pile rather than draining it.
- [ ] `ai/rules/protocol.md` - governs the code-comment destination
  → Constraint: an invariant governing one function belongs at that function. `session_connection.go` line 330 is the exemplar to match.
- [ ] `ai/rules/never-destroy-work.md` - governs every deletion
  → Constraint: deletion needs explicit owner permission. The pilot deletes nothing.

**Key insights:**
- The un-routable set is the deliverable, not a by-product.
- "Already documented" is the most common verdict and the cheapest win, but it must be proven by opening the target.
- Routing into `docs/` risks giving the architecture docs the disease the corpus has. One to three lines merged, never a section appended.

## Current Behavior (MANDATORY)

**Current surfaces:**
- `ai/rules/planning.md`, "Spec Lifecycle", governs direct lesson routing.
- `plan/learned/` contains the current records to inventory; their existence
  alone says nothing about whether each lesson is already routed.
- `internal/le/journal/journal.go` `Check` reads problem-class rows at HEAD.
  `Report.Text` in `internal/le/journal/report.go` reports recurring classes.
- `internal/le/journal/validate.go` `ValidateFile` validates one journal file.
- `internal/le/doc/check/actions.go` owns the documentation-check command
  surface; integration of an age guard remains design work.

**Behavior to preserve:**
- Direct routing, conditional journal creation and every existing gate.
- The current journal recurrence report and its committed-tree population.
- Historical records stay intact during the pilot; later deletion requires
  explicit permission.

**Behavior to change:**
- Audit the current eligible records for missing homes and already-routed
  content. Add an unrouted-age report and guard only against a defined residual
  population, without requiring closure summaries or a target archive size.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A closure routes a lesson directly to its governing surface.
- A pilot reads the current inventory of eligible records.

### Transformation Path
1. Classify each durable item against the taxonomy and the current routing rule.
2. Verify an `ALREADY-THERE` verdict by reading and quoting the destination.
3. Merge unrouted content into its destination, within the original prose budget
   and with source anchors where `docs/` receives it.
4. Keep pilot records intact. Later removal requires proof that every item has
   a home and the owner's explicit permission.
5. An item with no home is recorded in the documentation-gap list rather than forced.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Closure ↔ routing | Preserve direct routing under `ai/rules/planning.md`; no write-summary prerequisite | Rule read; closure integration remains to check |
| Record ↔ `docs/` | routed factual text carries source anchors checked by `./le doc check verify` | Existing documentation contract; pilot proof owed |
| Record ↔ code comment | an invariant belongs at its producing function | Pilot proof owed |
| Unrouted age ↔ gate | proposed check uses the current inventory and a defined routing state | Not designed |

### Integration Points
- `ai/skills/ze-close.md`, lesson routing: preserve the governing rule rather than adding a summary-writing stage.
- `internal/le/doc/check/actions.go` `./le doc check verify`, where a queue-depth gate would join.
- `ai/INDEX.md` Dev Tools, where any new tool must appear in the same phase.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | Not yet established | Reuse the native journal and documentation-check surfaces; preserve their existing answers |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, no wire path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | N-A, no daemon registration |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Most summary content has an existing home | The taxonomy maps every kind Ze produces, and summary 290's knowledge reached a code comment unaided | The corpus is mostly un-routable, which means Ze's documentation is far thinner than believed. That is a bigger finding, not a failure | The pilot's ratio table | unvalidated |
| A-2 | A large share is ALREADY documented and deletes cleanly | Summaries were written at closure alongside the doc updates the same closure required | Routing is much more expensive than estimated | The pilot's `ALREADY-THERE` count, proven by opening each target | unvalidated |
| A-3 | Routing into `docs/` does not bloat it | The one-to-three-line budget and merge-not-append rule | The architecture docs acquire the corpus's disease | Measure `docs/architecture/` size before and after the pilot | unvalidated |
| A-4 | The routing judgement is reproducible enough to delegate | Eight agents produced consistent extractions in the previous pass | Two agents route the same summary differently and the result is arbitrary | Route 5 summaries with two independent agents and compare | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Routed prose is appended rather than merged, moving the pile into `docs/` | `docs/architecture/` grows by roughly the size of what left `plan/learned/` | Measure both sides. The budget is one to three lines per item |
| R-2 | "Already documented" is asserted without opening the target | A routed summary is deleted and its knowledge is in neither place | The verdict requires a quoted line from the target. Spot-audit a sample |
| R-3 | The un-routable set is quietly forced into the nearest file rather than recorded | The gap list comes back suspiciously short | The gap list is a deliverable with its own AC. A short list is a finding to challenge, not a success |
| R-4 | Routing introduces dead live references | `./le doc check links` or `./le doc check verify` reports a new finding | Run the current checks before and after each batch; historical record citations retain their policy |
| R-5 | An age gate suppresses lesson recording or makes every closure create a record | Closures omit needed governing updates or create empty journal rows | Count age only for a defined unrouted population; preserve direct routing and conditional journal creation |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible and no daemon behavior. The failure is lost design rationale, or bloated architecture docs |
| How is it reverted? | Per wave. Each wave is one commit, and git history holds every deleted summary |
| Who else touches this path? | Concurrent sessions update governing surfaces and problem-class journals; coordinate ownership before routing into a shared destination |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le journal report` | → | proposed unrouted-age reporting alongside the existing recurrence answer | functional report test over a fixture with both populations; concrete test location is a design prerequisite |
| `./le doc check verify` | → | proposed age guard registered in the native action surface | refusal test for an over-age unrouted item and admission for an already-routed item |
| A closure that produced a lesson | → | direct routing under `ai/rules/planning.md` | inspect the governing update and absence of a mandatory intermediate summary |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The pilot runs over one subsystem of 30 to 50 eligible records from the current inventory | A ratio table reports every record as exactly one destination, with `ALREADY-THERE` proven by a quoted line from the target. An insufficient population requires an owner scope decision before the pilot |
| AC-2 | The pilot completes | A documentation-gap list names every un-routable item and the document that should exist to hold it |
| AC-3 | Five records are routed for real, spanning at least three destinations | Each target carries merged content with a source anchor where `docs/` received it, the pilot deletes nothing, and `./le doc check verify` and `./le doc check links` stay green |
| AC-4 | `docs/architecture/` is measured before and after the pilot | Growth is under 3 lines per routed item, proving merge rather than append (A-3, R-1) |
| AC-5 | Two independent agents route the same 5 summaries | Their destinations agree on at least 4 of 5, or the disagreement is reported as a taxonomy defect (A-4) |
| AC-6 | `./le journal report` runs | It retains its existing recurrence report, adds the count and oldest age of the defined unrouted population, and names the destination taxonomy in its help |
| AC-7 | A record in the defined residual population sits unrouted past the agreed age | `./le doc check verify` reports it. The gate counts unrouted age, preserves direct routing and never requires a closure summary (R-5) |
| AC-8 | The pilot's numbers do not justify the full pass | The spec records that plainly and the full pass is not done. A pilot that says no is a successful pilot |
| AC-9 | Any tool this spec adds | Appears in `ai/INDEX.md` Dev Tools in the same phase that creates it |
| AC-10 | Any tool this spec adds that rewrites a file | Preserves the file's mode, with a test that fails when the restoration is removed |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_queue_reports_unrouted_age` | `internal/le/` | AC-6 | |
| `test_gate_counts_age_not_total` | `internal/le/` | AC-7, R-5 | |
| `test_target_declared_in_doc_test` | `internal/le/` | wiring | |
| Direct-routing closure check | current closure skill and governing destination | a needed lesson reaches its home without a mandatory summary | |
| `test_rewrite_preserves_mode` | `internal/le/` | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Unrouted age, days | 0-N | the agreed threshold | N/A | one day past it |
| Routed lines per item | 1-3 | 3 | 0 | 4 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Unrouted-age report fixture | native journal test surface, exact location to be designed | An agent sees both recurrence data and the unrouted population without confusing the two | |

### Interop Tests (Scope: protocol)
N-A. Scope is tooling. No wire-visible behavior changes.

## Files to Modify

- `ai/skills/ze-close.md` - reconcile lesson routing with the governing direct-routing rule if drift remains; do not add an intermediate summary
- `ai/rules/planning.md` - preserve direct routing and conditional journal creation; only add an approved residual-age contract
- `internal/le/doc/check/actions.go` - register the residual-age guard after its population is defined
- `ai/INDEX.md` - Dev Tools row, in the same phase (AC-9)
- `internal/le/journal/report.go` and its owning producer - extend the existing answer without replacing recurrence reporting (AC-6)
- `ai/rules/repo-maintenance.md` - discovery-surface row

## Files to Create

- native journal and documentation-check tests for the new behavior, at locations chosen during design
- no replacement deferral shard; any residual implementation scope needs its own spec under the current lifecycle
- a documentation-gap list, location decided at the pilot (AC-2)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface; agent tooling only |
| YANG validation constraints | N-A | No YANG leaf |
| YANG custom validators | N-A | No YANG leaf |
| CLI commands/flags | Yes | Proposed extension to `./le journal report` and a native documentation check |
| CLI grammar (keyword before value) | Yes | Preserve the native action grammar |
| Editor autocomplete | N-A | No YANG leaf |
| Functional test for new RPC/API | N-A | No RPC; covered by `internal/le/` |
| Pipe completeness | N-A | No `ze` command output |
| Env var registration | N-A | No `environment/` leaf |
| Doctor check for runtime dependencies | N-A | Build-time only, never in the daemon |
| Prometheus counters/metrics | N-A | No daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Agent tooling |
| 2 | Config syntax changed? | No | No config surface |
| 3 | CLI command added/changed? | Yes | Document the native journal report extension and age-check contract in `docs/contributing/documentation-testing.md` |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | No | Contributor tooling |
| 7 | Wire format changed? | No | No wire path |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol behavior |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/documentation-testing.md` gains the queue gate |
| 11 | Affects daemon comparison? | No | No daemon capability |
| 12 | Internal architecture changed? | Yes | This spec ROUTES content into `docs/architecture/`, which is the point |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Routing adds anchors; every one is verified against code first |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/INDEX.md` and `ai/rules/repo-maintenance.md` list current gates |

## Implementation Steps

1. **Phase: Current population and pilot design**: inventory eligible records
   and define evidence for routed versus unrouted state. Preserve the original
   30-to-50 pilot scope; an insufficient current population goes to the owner
   before execution. Decide the residual age threshold and exact native test
   surfaces without changing the direct-routing lifecycle.
2. **Phase: Pilot**: classify every selected record, prove `ALREADY-THERE` at
   the target, route five across at least three destinations, and measure
   architecture-document growth. Produce the ratio table and documentation-gap
   list. Delete nothing. If the ratio does not justify a full pass, record that
   result and stop the full pass under AC-8.
3. **Phase: Wiring and report**: extend the native journal report without
   replacing recurrence data, register the documentation-check entry point,
   and add discovery documentation in the same phase. Prove both entry points
   against the defined population.
4. **Phase: Age guard**: prove over-age refusal, already-routed admission and
   file-mode preservation where a tool rewrites content. The guard must not
   make a new summary a closure prerequisite.
5. **Phase: Remaining batches**: route only the justified current population,
   with destination proof and current gates before and after each batch.
   Later deletions require explicit permission; the pilot grants none.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation |
| Correctness | Every test for a guard asserts the REFUSAL, never the fallback. This is the defect the previous pass shipped and the reviewer caught |
| Correctness | The age gate cannot suppress a governing update, require a closure summary or turn a historical record's existence into proof that it is unrouted |
| Correctness | Any ceiling asserts EQUALITY with the measured count, never merely bounds it |
| Data flow | Routed content is merged into an existing section, never appended as a new one |
| Evidence | Every `ALREADY-THERE` verdict quotes a line from the target file |
| Rule: `ai/rules/writing.md` | Every routed line into `docs/` carries a source anchor and was verified against code first |
| Rule: `ai/rules/never-destroy-work.md` | The pilot deletes nothing; later waves delete only with per-wave permission |
| Rule: `ai/rules/repo-maintenance.md` | Every artifact reaches `ai/INDEX.md` in the phase that creates it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The pilot ratio table | present in this spec, one row per destination with counts |
| The documentation-gap list | a file, with one row per un-routable item and the document that should hold it |
| Five routed records | inspect the target edits and both native documentation checks; pilot records remain intact |
| `docs/architecture/` growth measured | line counts before and after, under 3 per routed item |
| `./le journal report` | reports the residual population while preserving recurrence output |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The queue tool parses arbitrary markdown and resolves paths. It must never resolve outside the repository root and never execute what it reads |
| Destructive tooling | Later waves delete summaries. Deletion is per-wave, permission-gated, and never inferred from a classification alone |

### Failure Routing
| Failure | Route To |
|---------|----------|
| The pilot ratio does not justify the pass | Record it and STOP. That is AC-8, not a failure |
| Two agents route the same summary differently | A-4 is broken; the taxonomy needs sharpening before any wave |
| `docs/architecture/` grows roughly as much as `plan/learned/` shrank | R-1 fired; the merge rule is not being followed. Back to DESIGN |
| A routed summary's knowledge is in neither place | R-2 fired. Restore from git, and make the quoted-line proof mandatory rather than expected |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The un-routable set measures missing documentation; its current size is
  unknown until the inventory and destination checks run.
- A historical record can remain after its lesson is routed. Record count
  alone cannot establish a queue or justify deletion.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Route by content | Prune by age again; prune by citation count | Age was a proxy and is spent. Citation count punishes knowledge nobody has needed YET |
| Pilot before planning the full pass | Spec the whole routing effort now | Four ratios are unknown and they change the size of the work by an order of magnitude. Speccing first bakes in the assumption the pilot exists to test |
| The gate counts unrouted AGE | Count total summaries | A total-count gate makes "write no summary" the cheapest way to stay green, which is the opposite of the goal |
| Deletion stays per-wave and permission-gated | One blanket grant | `ai/rules/never-destroy-work.md` treats permission as per-band, and the previous pass honoured that with a hardcoded ceiling |

## Known Limitations

- Routing is a judgement call and is not reproducible in the way a gate is.
  AC-5 measures the disagreement rate rather than assuming it is zero.
- The proposed age guard needs an explicit routed-state contract before it can
  classify records. It cannot prove that the chosen destination is correct.
- `docs/architecture/` has no size gate today, so R-1 is measured rather than
  enforced.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Route any lesson directly to its governing surface; write a journal row only when no surface governs it
- [ ] Commit A preserves the code, evidence, documentation and edited spec under the current closure workflow
- [ ] Commit B removes only the reviewed spec through the same generated commit script
