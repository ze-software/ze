# Spec: fixit-learned-dead-paths

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; not started; the shard is listed under Files to Create and is not owed until a deferral is made. Create `plan/deferrals/fixit-learned-dead-paths.md` on the first deferral) |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`plan/learned/` still cites 389 file paths that do not exist, across 306
distinct paths. Down from 1,860 at the start of the closed knowledge-0 umbrella,
which retired band 1-400 and repaired 672 paths from git rename history.

`make ze-learned-staleness` holds the line with a zero-slack shrink-only
ceiling, so this cannot grow. It is not falling either.

### The composition, measured 2026-08-02

| Category | Distinct paths |
|----------|----------------|
| Basename exists NOWHERE in the tree: genuinely deleted code | **229** |
| Still resolvable in principle: a successor exists but automation could not pick it | **77** |
| Total distinct | 306 (389 references) |

That split decides the whole spec. The 77 are a repair job. The 229 are not:
the code is gone, so there is no path to repoint to, and deleting the reference
would destroy the sentence around it.

### What "fix" means for each half

- **The 77.** `scripts/dev/learned_repath.py` already resolves by git rename,
  learned directory rule, and unique three-segment suffix. These survived all
  three, so each needs a judgement its resolver refused to guess. Two were
  flagged AMBIGUOUS, meaning more than one candidate owned the suffix. The rest
  need a human-grade decision about which successor is meant.
- **The 229.** The honest options are to annotate (mark the path as referring to
  code that no longer exists, so a reader is not misled), or to accept them as a
  permanent floor and say so. **Deleting the reference is not on the table**: the
  summary's sentence still means something, and `learned_repath.py`'s
  `cites_a_deletion` guard already exists because repointing a line whose prose
  marks the file as deleted made the sentence self-contradictory.

### Why this is a fixit and not a continuation

`plan/spec-knowledge-routing.md` proposes routing summary content to its
canonical home, which would make many of these paths moot by moving the content
out of `plan/learned/` entirely. **If that spec runs first, this one shrinks or
disappears.** Check its status before starting. Doing both is waste.

## Required Reading

### Architecture Docs
- [ ] The closed knowledge-0 umbrella (record retired with the learned corpus) - what was already tried
  → Constraint: age-band retirement and git-rename repath are DONE and are not repeated. What remains resisted both.
  → Decision: the shrink-only ceiling asserts EQUALITY with the measured count. Any change here re-records it, never raises it.
- [ ] `plan/spec-knowledge-routing.md` - the overlapping proposal
  → Constraint: read its status FIRST. If routing is proceeding, most of this spec is redundant.
- [ ] `ai/rules/never-destroy-work.md` - governs any deletion
  → Constraint: deleting a path reference removes meaning from a sentence someone wrote. It needs explicit permission and is not assumed.
- [ ] `ai/rules/completion.md` - the opposing pull
  → Constraint: it forbids recording a defect instead of fixing it. The 229 are the case where no fix exists, which must be stated with evidence rather than asserted.

**Key insights:**
- 229 of 306 have no successor. That is a floor, not a backlog, unless annotation counts as a fix.
- The 77 resisted three automated resolvers, so each is a judgement call.
- The routing spec may make this moot.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/learned_staleness.py` - `check`, `candidate_paths`, `path_problem`, `enforce`, `write_baseline`; the gate and its zero-slack ceiling
- [ ] `scripts/dev/learned_repath.py` - `Resolver` (`by_rename`, `by_directory_rule`, `by_suffix`), `cites_a_deletion`; the three resolvers these 77 survived and the guard that protects deletion prose
- [ ] `plan/.learned-staleness-baseline` - the recorded ceiling and its comments

**Behavior to preserve:**
- The zero-slack ceiling. It may only tighten.
- `cites_a_deletion`: a line whose prose marks the file as deleted is never repointed.
- Every summary's sentence. A path may be annotated; the surrounding meaning is not removed.

**Behavior to change:**
- The 77 resolvable paths are repointed.
- The 229 unresolvable ones are either annotated or accepted, with the choice recorded.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `make ze-learned-staleness` walks every summary and reports dead references.

### Transformation Path
1. `check` collects every path in a `## Files` or `## Files <qualifier>` section.
2. `path_problem` classifies each: missing, traversal, or resolvable.
3. `enforce` compares the count against the ceiling and reports; it writes only
   under `--write-baseline`.
4. This spec adds a pass that resolves the 77 and disposes of the 229.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Gate ↔ corpus | `check` reads the tree, never a generated index | Yes, read in `learned_staleness.py` |
| Repair ↔ gate | any repair re-runs the gate and re-records the ceiling downward | Yes, that is the existing contract |

### Integration Points
- `learned_staleness.py` supplies the dead set; a repair pass must reuse `check`
  rather than reimplementing the walk, so the two cannot disagree.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | The repair reuses `learned_staleness.check` |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, no wire path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | N-A |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The 77 have a determinable successor a human can pick | They survived three automated resolvers but a successor basename exists in the tree | They are unresolvable too, and the floor is 306 rather than 229 | Sample 10 and attempt resolution by hand before committing to the pass | unvalidated |
| A-2 | The 229 genuinely have no successor | Their basename exists nowhere in `git ls-files` | Some are renamed beyond basename recognition and ARE repairable | Sample 10 and check `git log --diff-filter=D` for each | unvalidated |
| A-3 | Annotation is worth the churn | A reader meeting a dead path currently cannot tell deleted-code from a stale reference | It edits hundreds of files for a marginal reader benefit | Ask the owner before the annotation pass; it is a judgement, not a defect |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A hand repair repoints to the wrong successor | A path resolves but names unrelated code | Every hand repair states the evidence (the git rename, or the reason this successor is the one meant). No guesses |
| R-2 | The annotation pass touches hundreds of summaries for little gain | The diff is large and the reader benefit is thin | A-3 gates it. Ask before doing it |
| R-3 | This duplicates `plan/spec-knowledge-routing.md` | Both specs edit the same summaries | Check the routing spec's status first. If it is running, close this one as superseded |
| R-4 | The ceiling is raised to accommodate a partial repair | `plan/.learned-staleness-baseline` grows | The ceiling only tightens. A partial repair lowers it or leaves it alone |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. A wrong repair points a reader at unrelated code, which is worse than a dead path because it is not detectable |
| How is it reverted? | Single commit revert; git history holds every summary |
| Who else touches this path? | Any session closing a spec writes to `plan/learned/`. The routing spec would touch the same files |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-learned-staleness` | → | `learned_staleness.check` after the repair | `test_dead_count_fell_and_ceiling_tightened` in `scripts/dev/learned_staleness_test.py` |
| A summary citing deleted code | → | the annotation form, if adopted | `test_annotated_path_is_not_counted_dead` in `scripts/dev/learned_staleness_test.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `plan/spec-knowledge-routing.md` status is checked before any work | Recorded in this spec. If routing is proceeding, this spec closes as superseded and AC-2 onward do not apply |
| AC-2 | A sample of 10 from the 77 is resolved by hand | Each names its evidence: the git rename, or why this successor is the one meant. A-1 confirmed or broken |
| AC-3 | A sample of 10 from the 229 is checked against `git log --diff-filter=D` | Confirms the code is genuinely gone, or finds a rename basename matching missed. A-2 confirmed or broken |
| AC-4 | The 77 are repaired | `make ze-learned-staleness` reports a lower count and the ceiling is re-recorded downward, never raised |
| AC-5 | The disposition of the 229 | Either annotated, or accepted as a floor with the reasoning recorded in this spec and in `ai/rules/` if it changes guidance |
| AC-6 | No reference is deleted to improve the number | Every summary sentence that survived before survives after, verified by diff review |
| AC-7 | `make ze-verify` after the change | Green, including `ze-doc-test` and the staleness gate |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_dead_count_fell_and_ceiling_tightened` | `scripts/dev/learned_staleness_test.py` | AC-4 | |
| `test_annotated_path_is_not_counted_dead` | `scripts/dev/learned_staleness_test.py` | AC-5, only if annotation is adopted | |
| `test_ceiling_never_rises_during_repair` | `scripts/dev/learned_staleness_test.py` | R-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Recorded ceiling | 0-389 | the measured count after repair | N/A | any value above the measured count |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `learned_staleness_test.py` | `scripts/dev/learned_staleness_test.py` | An agent runs `make ze-learned-staleness` and the count is lower with the ceiling tightened to match | |

### Interop Tests (Scope: protocol)
N-A. Scope is tooling. No wire-visible behavior changes.

## Files to Modify

- `plan/learned/[0-9]*.md` - the summaries carrying the 77 repairable paths, and the 229 if annotation is adopted
- `plan/.learned-staleness-baseline` - re-recorded downward after repair
- `ai/rules/` - only if the disposition of the 229 changes guidance for future summaries

## Files to Create

- `plan/deferrals/fixit-learned-dead-paths.md` - deferral shard

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface |
| YANG validation constraints | N-A | No YANG leaf |
| YANG custom validators | N-A | No YANG leaf |
| CLI commands/flags | N-A | No `ze` subcommand; existing make targets only |
| CLI grammar (keyword before value) | N-A | No CLI command |
| Editor autocomplete | N-A | No YANG leaf |
| Functional test for new RPC/API | N-A | No RPC; covered by `scripts/dev/*_test.py` |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | Build-time only |
| Prometheus counters/metrics | N-A | No daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Agent tooling |
| 2 | Config syntax changed? | No | No config |
| 3 | CLI command added/changed? | No | No command |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | No | Contributor tooling |
| 7 | Wire format changed? | No | No wire path |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol behavior |
| 10 | Test infrastructure changed? | No | The gate exists; only its ceiling moves |
| 11 | Affects daemon comparison? | No | No capability change |
| 12 | Internal architecture changed? | No | No architecture change |
| 13 | Route metadata keys added/changed? | No | No metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration |
| 16 | Any changed source file referenced by existing doc source anchors? | No | Only `plan/learned/` content changes |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No examples touched |

## Implementation Steps

1. **Phase: Check for supersession (MANDATORY FIRST)** -- read
   `plan/spec-knowledge-routing.md` status. **If routing is proceeding, close
   this spec as superseded and stop.** Record the decision either way (AC-1)
2. **Phase: Sample before committing** -- resolve 10 of the 77 by hand and check
   10 of the 229 against `git log --diff-filter=D`. This validates A-1 and A-2
   and sizes the work honestly (AC-2, AC-3)
3. **Phase: Repair the 77** -- each with its evidence stated. Re-run the gate and
   re-record the ceiling downward (AC-4)
4. **Phase: Dispose of the 229** -- ask the owner whether annotation is worth the
   churn (A-3). Either annotate, or record acceptance as a floor with reasoning
   (AC-5)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation, or AC-1 closed the spec as superseded |
| Correctness | Every hand repair names its evidence. A repair without evidence is a guess, and a wrong path is worse than a dead one |
| Correctness | The ceiling only ever tightened |
| Evidence | AC-6: no summary sentence lost meaning. Verified by reading the diff, not by the count falling |
| Rule: `ai/rules/never-destroy-work.md` | No reference deleted to improve the number |
| Rule: `ai/rules/completion.md` | The 229 are recorded as unfixable only with the per-sample evidence from AC-3 |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Supersession decision | recorded in this spec |
| The 77 repaired | `make ze-learned-staleness` count before and after |
| Ceiling tightened | `cat plan/.learned-staleness-baseline` |
| The 229 disposition | recorded in this spec with its reasoning |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Any repair tooling resolves paths from markdown. It must never resolve outside the repository root, and a `..` token is reported rather than resolved |

### Failure Routing
| Failure | Route To |
|---------|----------|
| The routing spec is proceeding | Close this as superseded. That is AC-1, not a failure |
| The 77 sample proves unresolvable | A-1 broken. The floor is 306, and the spec becomes a disposition decision only |
| The 229 sample finds repairable renames | A-2 broken. Widen the repair set and say what the resolver missed |
| A repair cannot be evidenced | Leave it. An unevidenced repair is a guess |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- 229 of 306 have no successor, so most of the remaining number is a floor rather
  than a backlog. Saying that plainly is worth more than churning the corpus.
- The paths that survived three automated resolvers are exactly the ones needing
  judgement, so this work does not automate.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Check for supersession first | Start repairing immediately | The routing spec would move this content out of `plan/learned/` entirely, making most of the repair waste |
| Sample before committing to the pass | Trust the 77/229 split | The split came from a basename heuristic. Ten hand checks per side cost little and could halve or double the work |
| Deleting a reference is off the table | Delete dead paths to reach zero | The sentence around the path still means something, and the count is not the goal |

## Known Limitations

- The 77/229 split rests on whether a basename exists in `git ls-files`. A file
  renamed AND moved beyond basename recognition would be misfiled as
  unresolvable, which AC-3 samples for.
- Annotation, if adopted, tells a reader the code is gone. It cannot tell them
  what replaced it.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated, or AC-1 closed the spec as superseded
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
