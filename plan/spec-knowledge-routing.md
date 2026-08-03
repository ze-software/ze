# Spec: knowledge-routing

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/knowledge-routing.md` |
| Updated | 2026-08-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make `plan/learned/` a **staging queue** rather than an archive. A summary is
written at closure, routed to the permanent home its content belongs in, and
removed. Steady state is near zero, not 889.

### Why the previous pass was not enough

`plan/learned/1316-knowledge-0-umbrella.md` (closed, see
`plan/learned/1316-knowledge-0-umbrella.md`) retired summaries 1-400 on an AGE
band, because decay correlated with age: band 1-200 was 78% dead paths. That
worked and 889 remain. But age was a PROXY. The real question for every summary
is whether its content belongs somewhere else, and for most of them it does.

### The routing taxonomy

Ze already has a canonical home for every kind of durable knowledge. The
corpus exists because closure wrote to a queue and nobody drained it.

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

**A summary that cannot be routed reveals a MISSING DOCUMENT.** The size of the
un-routable set is a measure of documentation debt, and nobody has ever read the
corpus that way. Producing that list is a deliverable in its own right, and it
is worth more than the disk space.

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
- [ ] `plan/learned/1316-knowledge-0-umbrella.md` - the previous pass and its measurements
  → Constraint: dead-path rate is now about 5% and `make ze-learned-staleness` holds it with a zero-slack ceiling. Routing must not raise it.
  → Decision: age-band retirement is DONE and is not repeated. This spec routes by content.
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

**Source files read:**
- [ ] `internal/component/bgp/reactor/session_connection.go` - the `INVARIANT` comment at line 330 is the exemplar of a routed invariant: it names the rule, the dependent readers, and the failure mode
- [ ] `scripts/dev/learned_staleness.py` - `check`, `enforce`, `write_baseline`; the zero-slack ceiling routing must not disturb
- [ ] `ai/skills/ze-close.md` - step 6a writes a summary only when the work produced a lesson; this spec adds the routing step after it
- [ ] `scripts/dev/check_doc_links.py` - `check_index_budget`, `check_hook_names`; the closest sibling for a new markdown-corpus gate
- [ ] `plan/learned/DESIGN-HISTORY.md` - one of the routing destinations, rebuilt and now gated

**Behavior to preserve:**
- `make ze-learned-staleness` and its zero-slack ceiling.
- The conditional-creation rule: a summary is written only when there is a lesson.
- Every existing gate that blocks a commit.

**Behavior to change:**
- A summary acquires a lifecycle: written, routed, removed.
- `plan/learned/` acquires a target size and a gate that notices when it grows.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A spec closes and `/ze-close` writes a summary, when there is a lesson.
- A routing pass runs over the existing 889.

### Transformation Path
1. The summary is written as today.
2. Its content is classified against the taxonomy: one destination per item.
3. Each item is merged into its destination, one to three lines, with a source anchor where `docs/` receives it.
4. The summary is removed once every item has a home.
5. An item with no home is recorded in the documentation-gap list rather than forced.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Closure ↔ routing | `/ze-close` gains a routing step after the summary is written | No |
| Summary ↔ `docs/` | routed lines carry source anchors, gated by `check_doc_links.py` | Yes, the anchor format is enforced today |
| Summary ↔ code comment | an invariant lands at its function | No |
| Queue depth ↔ gate | a new check counts unrouted summaries | No |

### Integration Points
- `ai/skills/ze-close.md` step 6, where the summary is written.
- `mk/inventory.mk` `ze-doc-test`, where a queue-depth gate would join.
- `ai/INDEX.md` Dev Tools, where any new tool must appear in the same phase.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | A routing gate joins `check_doc_links.py` rather than becoming a new target |
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
| R-4 | Routing raises the dead-path count, because `docs/` anchors rot too | `make ze-learned-staleness` or `check_doc_links.py` reddens | Both gates run before and after each wave |
| R-5 | The queue-depth gate becomes a reason to skip writing a summary at all | Closures stop producing summaries entirely | The gate counts UNROUTED age, never total count. A summary routed the same week never trips it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible and no daemon behavior. The failure is lost design rationale, or bloated architecture docs |
| How is it reverted? | Per wave. Each wave is one commit, and git history holds every deleted summary |
| Who else touches this path? | Every concurrent session writes `plan/learned/` at closure. Waves must be short and land promptly |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-learned-queue` | → | the queue-depth reporter | `test_queue_reports_unrouted_age` in `scripts/dev/learned_queue_test.py` |
| `make ze-doc-test` | → | the gate declared in `mk/inventory.mk` | `test_target_declared_in_doc_test` in `scripts/dev/learned_queue_test.py` |
| `/ze-close` on a spec that produced a lesson | → | the routing step in `ai/skills/ze-close.md` | `test_close_skill_names_the_routing_step` in `scripts/dev/learned_queue_test.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The pilot runs over one subsystem of 30 to 50 summaries | A ratio table reports every summary as exactly one destination, with `ALREADY-THERE` proven by a quoted line from the target |
| AC-2 | The pilot completes | A documentation-gap list names every un-routable item and the document that should exist to hold it |
| AC-3 | Five summaries are routed for real, spanning at least three destinations | Each target file carries the merged content with a source anchor where `docs/` received it, and `check_doc_links.py` stays green |
| AC-4 | `docs/architecture/` is measured before and after the pilot | Growth is under 3 lines per routed item, proving merge rather than append (A-3, R-1) |
| AC-5 | Two independent agents route the same 5 summaries | Their destinations agree on at least 4 of 5, or the disagreement is reported as a taxonomy defect (A-4) |
| AC-6 | `make ze-learned-queue` runs | It reports how many summaries are unrouted and how old the oldest is, and names the destination taxonomy in its help |
| AC-7 | A summary sits unrouted past the agreed age | `make ze-doc-test` reports it. The gate counts unrouted AGE, never total count (R-5) |
| AC-8 | The pilot's numbers do not justify the full pass | The spec records that plainly and the full pass is not done. A pilot that says no is a successful pilot |
| AC-9 | Any tool this spec adds | Appears in `ai/INDEX.md` Dev Tools in the same phase that creates it |
| AC-10 | Any tool this spec adds that rewrites a file | Preserves the file's mode, with a test that fails when the restoration is removed |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_queue_reports_unrouted_age` | `scripts/dev/learned_queue_test.py` | AC-6 | |
| `test_gate_counts_age_not_total` | `scripts/dev/learned_queue_test.py` | AC-7, R-5 | |
| `test_target_declared_in_doc_test` | `scripts/dev/learned_queue_test.py` | wiring | |
| `test_close_skill_names_the_routing_step` | `scripts/dev/learned_queue_test.py` | wiring | |
| `test_rewrite_preserves_mode` | `scripts/dev/learned_queue_test.py` | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Unrouted age, days | 0-N | the agreed threshold | N/A | one day past it |
| Routed lines per item | 1-3 | 3 | 0 | 4 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `learned_queue_test.py` | `scripts/dev/learned_queue_test.py` | An agent runs `make ze-learned-queue` and sees what is unrouted and how stale | |

### Interop Tests (Scope: protocol)
N-A. Scope is tooling. No wire-visible behavior changes.

## Files to Modify

- `ai/skills/ze-close.md` - a routing step after the summary is written
- `ai/rules/planning.md` - "Writing Learned Summaries" states the lifecycle: written, routed, removed
- `mk/inventory.mk` - declare `ze-learned-queue`, add it to `ze-doc-test`
- `ai/INDEX.md` - Dev Tools row, in the same phase (AC-9)
- `ai/rules/repo-maintenance.md` - discovery-surface row

## Files to Create

- `scripts/dev/learned_queue.py` - the queue-depth reporter and gate
- `scripts/dev/learned_queue_test.py` - its tests
- `plan/deferrals/knowledge-routing.md` - deferral shard
- a documentation-gap list, location decided at the pilot (AC-2)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface; agent tooling only |
| YANG validation constraints | N-A | No YANG leaf |
| YANG custom validators | N-A | No YANG leaf |
| CLI commands/flags | N-A | Make targets only, no `ze` subcommand |
| CLI grammar (keyword before value) | N-A | No CLI command |
| Editor autocomplete | N-A | No YANG leaf |
| Functional test for new RPC/API | N-A | No RPC; covered by `scripts/dev/*_test.py` |
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
| 3 | CLI command added/changed? | No | Make targets only |
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

1. **Phase: The pilot (MANDATORY FIRST, and it may end the spec)** -- one
   subsystem of 30 to 50 summaries. Classify every one against the taxonomy,
   proving `ALREADY-THERE` by quoting the target. Route 5 for real across at
   least three destinations. Measure `docs/architecture/` before and after.
   - Tests: none yet; this phase produces numbers
   - Verify: the ratio table, the gap list, the growth measurement. **If the
     ratio does not justify the full pass, record that and STOP (AC-8)**
2. **Phase: Wiring** -- declare `ze-learned-queue` and its `ai/INDEX.md` row
   before the checker works, with failing tests
   - Tests: `test_target_declared_in_doc_test`, `test_close_skill_names_the_routing_step`
3. **Phase: The queue gate** -- implement the reporter, counting unrouted AGE
   rather than total count
   - Tests: `test_queue_reports_unrouted_age`, `test_gate_counts_age_not_total`, `test_rewrite_preserves_mode`
4. **Phase: The lifecycle** -- `ze-close.md` gains the routing step,
   `planning.md` records written-routed-removed
5. **Phase: Waves** -- route the remaining subsystems, one commit per wave,
   re-running both gates before and after each

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation |
| Correctness | Every test for a guard asserts the REFUSAL, never the fallback. This is the defect the previous pass shipped and the reviewer caught |
| Correctness | The queue gate counts unrouted AGE, so it can never become a reason to skip writing a summary |
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
| Five routed summaries | `git diff` over the five target files, plus `check_doc_links.py` green |
| `docs/architecture/` growth measured | line counts before and after, under 3 per routed item |
| `make ze-learned-queue` | runs and reports |

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

- The un-routable set measures documentation debt. That reading is available
  today and has never been taken.
- `plan/learned/` being large is a symptom of a queue nobody drains, not of
  people over-recording.

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
- The queue gate can prove a summary is unrouted. It cannot prove a routed one
  landed in the RIGHT home.
- `docs/architecture/` has no size gate today, so R-1 is measured rather than
  enforced.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
