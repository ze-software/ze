# Spec: knowledge-0-umbrella

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 4/4 |
| Deferral shard | `plan/deferrals/knowledge-0-umbrella.md` |
| Updated | 2026-08-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's recorded-knowledge system has grown without a retirement path and is now
mostly unread. Measured on 2026-08-01 against the working tree:

| Measure | Value |
|---|---|
| Numbered summaries in `plan/learned/` | 1,289 (73,403 lines, 7.2 MB) |
| Added in the last 90 days | 626, about 7 per day |
| Summaries ever deleted | 0 |
| Rule files in `ai/rules/` | 99, grown from 43 in April, none ever deleted |
| Summaries cited by any rule, skill, doc, spec, or script | 183 (14%) |
| Summaries appearing only as a generated index row | about 800 (62%) |
| `## Files` paths cited that no longer exist | 1,860 of 7,683 (24%) |
| Summaries carrying no gotcha | 229 (17%); 77 say so in words |
| Summaries using the retired `## Objective` heading | 425 (33%) |
| Summaries carrying all five prescribed sections | 48% |
| `ai/rules/CONDENSED.md`, loaded into every session | 400 KB, about 100,000 tokens |

Decay is an age function, and the gradient is steep. Dead-path rate by band:
1-200 is 78%, 201-400 is 44%, 401-600 is 18%, 601-800 is 22%, 801-1000 is 16%,
1001-1200 is 7%, 1201-1400 is 5%. The first 400 summaries hold 856 of the 1,860
dead paths: 46% of the rot in 24% of the files. A 45-file full-read sample puts
53% of the corpus in the durable-knowledge bucket and finds no stale entry above
number 900.

So the corpus is not mostly worthless. It is a good recent corpus with a rotted
tail, an index that inverted into a second corpus, three hand-maintained
meta-documents that stopped tracking reality, and a per-session rule digest that
costs about 100,000 tokens whether or not any of it applies.

The goal of this spec set is to make recorded knowledge bounded: written only
when there is something to record, mechanically checkable for decay, and
retired on a defined trigger rather than never.

### Root cause

`ai/skills/ze-close.md` step 6a makes `plan/learned/NNN-<stem>.md` an
unconditional artifact of every spec closure, and step 6e passes
`--lesson-required` to `commit_helper.py` on commit A.
`plan/learned/METHODOLOGY.md` supplies the boilerplate for the empty case
("Mechanical refactor, no design decisions"). One spec closed is one file,
whether or not anything was learned. The volume is the design working as
written, not a discipline failure.

## Children

| Spec | Covers | Status |
|------|--------|--------|
| `plan/spec-knowledge-1-corpus.md` | Conditional creation, decay gate, age-banded retirement of 1-400, section-format normalisation | implemented, awaiting closure |
| `plan/spec-knowledge-2-meta-docs.md` | `ai/LEARNED-INDEX.md` pointer budget, dead-name lint for `HOOK-FRICTION.md`, rebuild and gate `DESIGN-HISTORY.md`, prune gated `RECURRING-PATTERNS.md` entries | implemented, awaiting closure |
| `plan/spec-knowledge-3-rule-digest.md` | Route `ai/rules/CONDENSED.md` by `**When:**` trigger instead of shipping all 97 sections eagerly | implemented, awaiting closure |

Execution order is 1, then 2, then 3. Child 1 WRITES the consolidated knowledge
into `plan/learned/DESIGN-HISTORY.md`, because that is where the extraction
happens. Child 2 GATES that file, because that is where the checkers live. Child
3 is independent of both and is the largest single lever, but also the highest
blast radius, so it lands last.

## Owner decisions (recorded 2026-08-01)

| Question | Answer |
|----------|--------|
| Spec shape | Umbrella plus three children, each closing independently |
| Fate of summaries 1-400 | Merge surviving knowledge into a consolidated file, then delete the originals. Git history keeps them recoverable |
| Destination for the consolidated knowledge | `plan/learned/DESIGN-HISTORY.md`, rebuilt and gated. NOT `ai/digests/`: its own README says "The historical record lives in `plan/learned/`", and `digest_check.py` `check_digest` requires every anchor to resolve today, so history about deleted code cannot be anchored there at all |
| Consolidation fidelity | Read the 330 summaries in band 1-400 that carry a real gotcha or a durable citation (9,619 lines). Delete the other 67 unread, after spot-reading 10 of them |
| `ai/rules/model-selection.md` Opus 4.8 / Opus 5 split | Out of scope. Not touched by this spec set |

## Deletion blast radius (measured, not assumed)

| Consumer | Effect of deleting 1-400 |
|----------|--------------------------|
| Go `// Design: plan/learned/NNN` refs | **None.** 1,134 refs exist and the lowest number cited is 415 |
| Summary numbering | **None.** Gaps are already legal and 29 exist today. No renumbering |
| `ai/digests/*.md` | 5 citations to repoint (173, 294, 374, 386, 390). `digest_check.py` fails closed on a missing target, so they must land in the same commit |
| `scripts/docvalid/check_doc_links.py` | 56 citations to summaries ≤400, 44 of them in `ai/LEARNED-INDEX.md` |
| `scripts/dev/learned_index.py --check` | Exits 3 "is stale" until `make ze-discovery-index` regenerates |
| `commit_helper.py` `discovery_index_problems` | BLOCKS the commit unless `ai/LEARNED-FULL-INDEX.md` is staged in the same commit |
| `scripts/dev/learned_numbers.py` `check` | Passes. It enforces uniqueness and H1-versus-filename only |


## Implementation complete (2026-08-01), closure outstanding

| Measure | Before | After |
|---------|--------|-------|
| Numbered summaries | 1,286 | 889 |
| Dead-path rate | 24% | 4.79% by the gate, 6.17% by the session-start script |
| `ai/LEARNED-INDEX.md` | 148 KB, 211 entries over budget | 51 KB, 0 over budget, gate blocking |
| Dead hook and check names | 31 | 0 |
| `DESIGN-HISTORY.md` | exempt from the link checker, 3 months stale | scanned, rebuilt, 1,504 lines |
| Per-session rule payload | 107,548 tokens | 21,356 tokens, 80% saving |
| `CLAUDE.md` | about 423 KB | 24 KB |

Six gates and nine Python suites green. Nothing committed.
`ai/rules/CONDENSED.md` is excluded: a concurrent session owns it.

**What this spec set learned about itself.** It found three defects in its OWN
specs, each caught by implementing rather than by review: a checker path that
did not exist (and would have produced a test that ran nowhere), an acceptance
criterion requiring a token its target file never contained, and an acceptance
criterion that never named its measuring instrument so that two defensible
readings straddle its threshold. The pattern is worth more than any single fix:
a plan is a hypothesis, and the gate is what tests it.

Closure is `/ze-close` on the review model: Review Gate, Implementation Audit,
Goal Validation, Pre-Commit Verification, then the two commits.


## Commit completeness: three files whose omission breaks everyone

`ai/rules/TRIGGERS.md`, `ai/rules/CORE.md` and `plan/.learned-staleness-baseline`
are new and untracked. None is gitignored. All three MUST be in commit A.

| File | If omitted |
|------|------------|
| `ai/rules/TRIGGERS.md` | `ai/INSTRUCTIONS.md` imports a file that does not exist. Every session and subagent loses the rule index |
| `ai/rules/CORE.md` | Same, and the always-on rules vanish, including every rung-1 and rung-2 rule from `rule-precedence.md` |
| `plan/.learned-staleness-baseline` | The decay gate has no ceiling, so it measures and never enforces. It looks armed and is not |

`ai/rules/CONDENSED.md` is TRACKED and is deliberately EXCLUDED: a concurrent
session owns it. It stays generated and unimported so the switch reverts in one
line, and its committed content is that session's to update.

The asymmetry is the trap. The file to exclude is tracked and the files to
include are not, so a commit script built from `git status` habits gets both
wrong.

## Out of scope

- `ai/rules/model-selection.md` and the three gates keyed on `REVIEW_TIER` in
  `scripts/dev/running_model.py`. Recorded as a live finding in the review,
  deliberately excluded here by owner decision.
- Deleting or rewriting rules under `ai/rules/`. Child 3 changes how the digest
  is ASSEMBLED and loaded, never the rule files themselves.
- `plan/known-failures/` and `plan/deferrals/`. Those are defect records
  governed by `ai/rules/no-parking.md` and `ai/rules/fix-dont-record.md`, and
  nothing here licenses pruning them.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/planning.md` - owns "Writing Learned Summaries" and spec closure
  → Constraint: the two-commit closure sequence is fixed. Commit A carries code
    plus spec plus learned summary, commit B removes the spec. Any change to
    whether a summary exists must keep both commits valid.
  → Decision: the five prescribed sections are Context, Decisions, Consequences,
    Gotchas, Files. Only 48% of the corpus carries all five, so any gate reading
    a named section must state what it does when the section is absent.
- [ ] `ai/rules/discovery-updates.md` - owns the discovery-surface obligation
  → Constraint: a new gate or make target must be added to `ai/INDEX.md` Dev
    Tools and to the rule that enforces it, in the same change.
- [ ] `ai/rules/detail-budget.md` - owns the pointer budget the index violates
  → Constraint: an index or pointer line is under 120 characters after the link.
    `ai/LEARNED-INDEX.md` has a median of 190, so the rule already governs; child
    2 adds enforcement, it does not invent the budget.
- [ ] `ai/rules/no-parking.md`, `ai/rules/fix-dont-record.md` - the opposing pull
  → Constraint: those rules govern DEFECT records and forbid recording instead of
    fixing. This spec set governs records of COMPLETED work. The distinction must
    be written into whichever rule owns retirement, or a later session will read
    retirement as licence to delete a known-failure shard.
- [ ] `ai/rules/never-destroy-work.md` - governs the deletion in child 1
  → Constraint: deletion of user-visible files needs explicit permission. Recorded
    for this spec set on 2026-08-01 (owner decision table above). The permission
    covers summaries 1-400 merged into consolidated files, nothing wider.

**Key insights:** (minimal context to resume after compaction)
- The volume is caused by `ze-close.md` step 6a, not by authors over-recording.
- Decay is an age function: below 400 the corpus is mostly dead references,
  above 1000 it is essentially current.
- `plan/learned/` costs nothing per session because nothing loads it. The
  standing per-session cost is `ai/rules/CONDENSED.md`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `ai/skills/ze-close.md` - step 6a allocates the number and mandates the
  summary for every closure; step 6b updates `ai/LEARNED-INDEX.md` on honour
  system; step 6e passes `--lesson-required` on commit A
- [ ] `plan/learned/METHODOLOGY.md` - the five-section format and the quality
  checks; also supplies the empty-case boilerplate that lets a contentless
  summary satisfy the format
- [ ] `plan/TEMPLATE.md` - the design-time spec scaffold this spec set uses
- [ ] `.claude/hooks/validate-spec.sh` - `REQUIRED_SECTIONS`, `REQUIRED_CHECKLIST`,
  and the tooling-only carve-out that accepts a `.py`/`.sh` functional surface
  when no daemon code is touched
- [ ] `internal/plugins/skills/main.go` - `inventory` embeds six user-facing
  skills only, so editing `ai/skills/ze-close.md` does not touch the binary
- [ ] `scripts/dev/running_model.py` - `REVIEW_TIER` is `("opus-5",)`; read to
  confirm the model gate is out of scope, not to change it

**Behavior to preserve:**
- The two-commit closure sequence in `ai/rules/planning.md`.
- `ai/LEARNED-FULL-INDEX.md` remains generated and remains a correct pointer
  index (median 27 characters, 0% over the 120-character budget).
- Summary numbering stays unique and stays allocated by `commit_helper.py`
  `learned_next`, so concurrent sessions cannot collide.
- Every gate that currently blocks a commit keeps blocking it.

**Behavior to change:**
- A learned summary becomes conditional on having content, not on a spec closing.
- Dead `## Files` paths and dead `plan/learned/NNN` citations become detectable.
- Summaries 1-400 are consolidated and removed.
- `ai/rules/CONDENSED.md` stops being shipped whole to every session.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A spec closes. `/ze-close` step 6a is the entry point that creates knowledge.
- A session starts. `CLAUDE.md` imports `ai/rules/CONDENSED.md`, which is the
  entry point that consumes knowledge.

### Transformation Path
1. `/ze-close` step 6a calls `commit_helper.py learned-next <stem>`, which
   allocates `max(existing) + 1` and creates the file immediately.
2. The agent writes the five sections per `plan/learned/METHODOLOGY.md`.
3. Step 6b optionally adds a row to `ai/LEARNED-INDEX.md`, on honour system.
4. `commit_helper.py` gates commit A on `--lesson-required` and on
   `discovery_index_problems`, which requires the generated full index to agree
   with the tree the commit produces.
5. `make ze-discovery-index` regenerates `ai/LEARNED-FULL-INDEX.md`.
6. Nothing reads any of it at session start. Discovery is a manual grep
   prescribed by `ai/INDEX.md`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Skill ↔ commit helper | `/ze-close` step 6 invokes `commit_helper.py learned-next` and `create --lesson-required` | Yes, read in `ai/skills/ze-close.md` |
| Commit helper ↔ generated index | `discovery_index_problems` materialises the commit view and re-runs the generators | No, delegated to research |
| Doc validator ↔ corpus | `check_doc_links.py` resolves the `plan/learned/NNN` shorthand by glob | No, delegated to research |
| Session start ↔ rules | `CLAUDE.md` imports `ai/rules/CONDENSED.md` whole | Yes, visible in the loaded context |

### Integration Points
- `commit_helper.py` gates: any change to whether a summary exists must keep
  `lesson_comment` and `discovery_index_problems` satisfiable.
- `mk/inventory.mk`: declares the discovery-index and learned-number targets and
  is where a new decay gate joins.
- `ai/INDEX.md`: the Dev Tools table is where a new make target becomes findable.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, no wire path touched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Deleting 400 summaries does not break a commit gate beyond regenerating the full index | `discovery_index_problems` regenerates from the commit view | The retirement commit cannot be prepared at all, and child 1 needs a different mechanism | Research agent report, then a dry-run commit script over a throwaway branch | unvalidated |
| A-2 | Summary numbers may be non-contiguous | `learned_numbers.py` is described as a duplicate-number and H1 check, not a contiguity check | Deleting 1-400 forces a mass renumber, which invalidates every citation in the tree | Read `learned_numbers.py` `summaries()` | unvalidated |
| A-3 | The 53% durable-knowledge rate generalises well enough that consolidating 1-400 loses nothing load-bearing | 45-file full-read sample | Real design rationale is lost with no way to know what | Per-file review during consolidation, not a bulk move | unvalidated |
| A-4 | `**When:**` triggers are precise enough to route rules without dropping one that should have fired | 98 of 99 rules carry the field; `ai/rules/rule-format.md` governs its shape | Child 3 silently stops delivering a blocking rule, which is worse than the token cost it saves | Child 3 builds a router and diffs its output against the full digest for a corpus of past tasks | unvalidated |
| A-5 | No gate asserts that a learned summary was read, so making creation conditional breaks no consumer | Research found number-uniqueness and link-integrity checks only | A consumer silently degrades | Research agent report | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Retirement is read by a later session as licence to prune defect records | A `plan/known-failures/` shard or a `plan/deferrals/` row is deleted citing this spec | Write the completed-work versus defect-record distinction into the rule that owns retirement, and name `no-parking.md` in it |
| R-2 | Child 3 drops a blocking rule from a session that needed it | A hook fires on something the digest would have prevented | Ship the router in report-only mode first, diff against the full digest, and keep every `blocking` rule eager until the diff is clean |
| R-3 | Consolidation becomes a second unread corpus, moving the problem rather than solving it | The consolidated files are themselves uncited three months on | The decay gate from child 1 applies to the consolidated files too, from the day they are created |
| R-4 | The conditional-creation change makes this spec set's own closure impossible | `/ze-close` refuses commit A for child 1 | Child 1's own summary must have real content, which it will; verify by closing child 1 through the changed path |
| R-5 | A concurrent session commits a learned summary mid-retirement, colliding with a number in the deleted range | `learned_next` allocates above the max, so a gap below is safe, but a citation may dangle | Run the retirement as one commit, and re-run the decay gate immediately after |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible and no daemon behavior. The failure modes are agent-facing: lost design rationale, a blocking rule that stops reaching sessions, or a commit gate that cannot be satisfied |
| How is it reverted? | Child 1 and 2 are single-commit reverts, and deleted summaries stay in git history. Child 3 reverts by restoring the whole-digest import in `ai/INSTRUCTIONS.md` |
| Who else touches this path? | Every concurrent session in this checkout writes to `plan/learned/` and `ai/LEARNED-INDEX.md`. Retirement must be one commit, not a long-running branch |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-verify` | → | the new decay gate declared in `mk/inventory.mk` | `test_staleness_gate_runs_in_verify` in `scripts/dev/learned_staleness_test.py` |
| `/ze-close` step 6a on a spec with no lesson | → | the conditional-creation path in `commit_helper.py` | `test_lesson_optional_when_no_content` in `scripts/dev/commit_helper_test.py` |
| A session start | → | the routed digest assembled from `ai/rules/*.md` | `test_router_emits_every_blocking_rule` in `scripts/dev/rules_router_test.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | All three children reach Status `done` and are closed per `ai/rules/planning.md` | `plan/spec-knowledge-1-corpus.md`, `-2-meta-docs.md`, `-3-rule-digest.md` are absent from `plan/` and their learned summaries exist |
| AC-2 | `make ze-verify` runs on the tree after all three children land | Exits 0, including the new decay gate |
| AC-3 | The corpus is measured again after child 1 | Dead `## Files` path rate is under 5% tree-wide, down from 24% |
| AC-4 | A spec closes with no reusable lesson after child 1 | No `plan/learned/NNN-*.md` is created, and commit A is still accepted |
| AC-5 | The always-loaded context is measured after child 3 | `ai/rules/CONDENSED.md` as imported is under 40,000 tokens, down from about 100,000, with every `blocking` rule still reaching the session |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_umbrella_children_exist` | `scripts/dev/knowledge_umbrella_test.py` | Each child named in the Children table exists in `plan/` or has a learned summary | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Retirement band upper bound | 1-1289 | 400 | N/A | 401 (a summary above the band must never be deleted by the retirement step) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `learned_staleness_test.py` | `scripts/dev/learned_staleness_test.py` | An agent runs `make ze-learned-staleness` and sees every dead path named | |
| `commit_helper_test.py` | `scripts/dev/commit_helper_test.py` | An agent closes a lessonless spec and the commit is accepted | |

### Interop Tests (Scope: protocol)
N-A. Scope is tooling. No wire-visible behavior changes.

## Files to Modify

- `ai/INDEX.md` - add the new gate to the Dev Tools table so it is discoverable
- `ai/rules/discovery-updates.md` - add the decay gate to the discovery-surface table

## Files to Create

- `plan/spec-knowledge-1-corpus.md` - child 1
- `plan/spec-knowledge-2-meta-docs.md` - child 2
- `plan/spec-knowledge-3-rule-digest.md` - child 3
- `plan/deferrals/knowledge-0-umbrella.md` - deferral shard for this spec set

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface. This spec set changes agent tooling and documentation only |
| YANG validation constraints | N-A | No YANG leaf added |
| YANG custom validators | N-A | No YANG leaf added |
| CLI commands/flags | N-A | No `ze` subcommand added. The new surfaces are make targets |
| CLI grammar (keyword before value) | N-A | No CLI command added |
| Editor autocomplete | N-A | No YANG leaf added |
| Functional test for new RPC/API | N-A | No RPC added. Tooling gates are covered by `scripts/dev/*_test.py` |
| Pipe completeness | N-A | No command output produced |
| Env var registration | N-A | No `environment/` leaf added |
| Doctor check for runtime dependencies | N-A | No runtime dependency added. The gates run at build and commit time, never in the daemon |
| Prometheus counters/metrics | N-A | No observable daemon state added |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Agent-facing tooling only; `docs/features.md` describes the product |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | No | Make targets only, not `ze` subcommands |
| 4 | API/RPC added/changed? | No | No RPC touched |
| 5 | Plugin added/changed? | No | No plugin touched |
| 6 | Has a user guide page? | No | Contributor tooling, not a user feature |
| 7 | Wire format changed? | No | No wire path touched |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface touched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol behavior touched |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/documentation-testing.md` gains the decay gate |
| 11 | Affects daemon comparison? | No | No daemon capability changes |
| 12 | Internal architecture changed? | No | No `internal/` architecture touched |
| 13 | Route metadata keys added/changed? | No | No route metadata touched |
| 14 | Prometheus counters added/changed? | No | No counters added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `scripts/dev/learned_index.py` and `commit_helper.py` before closing each child |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/INDEX.md` and `ai/rules/discovery-updates.md` list the current gates; both need the new one |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- create the three children as `ready`
   specs and the deferral shard, so each has a home before any code moves
   - Tests: `test_umbrella_children_exist`
   - Files: the three child specs, `plan/deferrals/knowledge-0-umbrella.md`
   - Verify: `make ze-spec-status` lists all four, and the shard exists
2. **Phase: Child 1** -- corpus: conditional creation, decay gate, retirement,
   format normalisation. Closes independently
3. **Phase: Child 2** -- meta-docs: index budget, HOOK-FRICTION, DESIGN-HISTORY,
   RECURRING-PATTERNS. Closes independently
4. **Phase: Child 3** -- rule digest routing. Ships report-only first, then
   switches the import once the diff against the full digest is clean

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation, and each child's ACs roll up to one here |
| Feature completeness | The three children together cover all eight review recommendations, with R7 and R8 explicitly out of scope |
| Correctness | The retirement band is 1-400 inclusive and the tooling cannot reach 401 or above |
| Naming | Child specs follow `spec-<prefix>-<N>-<name>.md` per `ai/rules/planning.md` Spec Sets |
| Data flow | Nothing in this spec set alters daemon behavior; every change is build-time or commit-time |
| Rule: `ai/rules/no-parking.md` | The retirement mechanism cannot reach `plan/known-failures/` or `plan/deferrals/` |
| Rule: `ai/rules/never-destroy-work.md` | Deletion is limited to the band the owner approved, and is recoverable from git history |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Three child specs exist and are `ready` | `make ze-spec-status \| grep knowledge-` |
| Deferral shard exists | `ls plan/deferrals/knowledge-0-umbrella.md` |
| All eight review recommendations are assigned or explicitly out of scope | Read the Children and Out of scope tables together |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The decay gate parses arbitrary markdown and resolves paths from it. It must never follow a path outside the repository root, and must not execute anything it reads |
| Destructive tooling | The retirement step deletes files. It must refuse a band outside 1-400 and must never take the band from an unvalidated argument |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A gate cannot be satisfied after retirement | Back to RESEARCH on child 1: the blast radius was mis-measured |
| The router drops a blocking rule | Back to DESIGN on child 3: keep every blocking rule eager |
| Consolidation loses knowledge a later session needs | Restore from git history and narrow the band |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The system already carries the metadata needed to fix it. Every rule has a
  `**When:**` trigger and a `**Severity:**`, and 98 of 99 are well-formed, so
  routing is an assembly change rather than a rewrite.
- The generated index obeys the pointer budget and the hand-maintained one does
  not. That is the whole argument for generating anything that can be generated.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Umbrella plus three children | One spec covering all eight recommendations | One review gate over four unrelated failure modes means a blocker in the digest work blocks the corpus work from closing |
| Merge then delete summaries 1-400 | Mark them stale in place; move them to an archive directory | Only deletion reduces what a future session must search. Git history keeps them recoverable, so the cost of being wrong is bounded |
| Retire by age band rather than uniformly | Prune every summary that is uncited | 53% of the corpus is durable knowledge, and uncited is not the same as worthless. The rot is concentrated in a band, so the band is the right unit |

## Known Limitations

- This spec set closes under the rules it changes. Each child produces a learned
  summary, and child 1 must not make its own closure impossible.
- The 53% durable-knowledge figure comes from a 45-file sample, not a full read.
  Consolidation must be reviewable per file rather than a bulk move.
- Child 3 can prove that every blocking rule still reaches a session. It cannot
  prove that a session which no longer sees an advisory rule would not have
  benefited from it.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
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
