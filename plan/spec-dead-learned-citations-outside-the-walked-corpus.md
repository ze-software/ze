# Spec: dead-learned-citations-outside-the-walked-corpus

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | arm the widened sweep |
| Deferral shard | `plan/deferrals/doc-claims-are-checked-not-just-resolved.md` |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

451 citations of `plan/learned/<id>-*.md` survive across the tree, over 172
distinct paths, and not one of them resolves. Commit `2cff2050a` retired the
numbered corpus. `plan/learned/` now holds three aggregate documents and nothing
else.

Found while implementing spec-doc-claims-are-checked-not-just-resolved (closed
2026-08-10): three of these citations were hiding behind the unreasoned
`doc-links: ignore` markers that spec's change 2 armed against. Those three were repaired there. The
other 448 were never in reach of that phase.

Reproduction, run 2026-08-09:

```
git grep -oh 'plan/learned/[0-9][0-9a-zA-Z_.-]*' | sort -u > tmp/refs.txt
wc -l < tmp/refs.txt                                       # 172 distinct paths
git grep -o 'plan/learned/[0-9][0-9a-zA-Z_.-]*' | wc -l    # 451 citations
while read -r p; do [ -e "$p" ] || echo "$p"; done < tmp/refs.txt | wc -l   # 172 dead
```

They survived a link gate that runs on every verify, and the reason is scope.
`check_markdown` in `scripts/dev/check_doc_links.py` walks `MD_GLOBS`, a
fourteen-entry list. It does not include `docs/`, `plan/deferrals/`,
`plan/spec-*.md`, the `Makefile`, `mk/`, or `.claude/hooks/`. A citation in any
of those is checked by nothing.

This spec is the inverse of `plan/spec-fixit-learned-dead-paths.md`, which is
about paths cited FROM the learned corpus. This one is about citations TO it.

### Measured 2026-08-10, and what it changed

Commit `26684a4bc` re-homed the pointers. Under the gate's own line grammar
(`suppressed`, `BACKTICK`, `MD_LINK`, `candidate_paths`) **3 dead
`plan/learned/` citations survive outside `MD_GLOBS`**, in
`plan/spec-perf-next-1-ebgp-wire-lockfree.md` and
`scripts/dev/commit_helper_test.py`. AC-3's 172 are down to 3.

A-2 was then measured by running `check_markdown`'s logic over every tracked
file outside `MD_GLOBS`. It is BROKEN, and by two orders of magnitude:

| Bucket | Findings | Distinct dead paths |
|--------|---------:|--------------------:|
| `plan/spec-*.md` | 2085 | 816 |
| rest of `plan/` (audits, to-review, deferrals, handover, reviews) | 422 | ~260 |
| `docs/` | 266 | 205 |
| `scripts/dev/*.py` | 84 | 44 |
| `ai/digests/`, `.claude/`, `rfc/`, `internal/`, `test/` | 96 | 75 |
| **Total** | **2953** | **1373** |

So the widening cannot be paid for by repair: the population is not this
spec's 451 learned citations, it is every dead path of every class, and 1373
distinct targets is a multi-session job this spec never scoped. Thomas chose
the ratchet on 2026-08-10: **widen tree-wide, publish today's survivors as a
shrink-only baseline, and refuse anything new.** That is the shape
`plan/.citation-baseline` and `make ze-learned-staleness` already have.

Three classes in the 2953 are NOT dead references and are fixed at the source
rather than baselined, because a baseline of false positives can never shrink:

| Class | Count | Fix |
|-------|------:|-----|
| `ai/digests/` line-number notation: `attrparse.go,62,152,169` | ~20 | `candidate_paths` strips a trailing `,<digits>` run, as `LINE_SUFFIX` already strips `:12` |
| MCP method names `tools/call`, `tools/list` (`tools` is a real root) | 76 | reasoned `doc-links: ignore` on the citing line | <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
| Placeholder paths inside hook error messages (`internal/x/y.go`, `scripts/dev/foo.py`) | ~25 | reasoned `doc-links: ignore` on the citing line | <!-- doc-links: ignore (placeholder paths quoted from a hook message, not real files) -->

## Required Reading

### Architecture Docs
- [ ] `ai/rules/planning.md` - what replaced the numbered corpus, and where a lesson lives now
  → Constraint: the successor is `plan/journal/<class>.md`, so a repair repoints rather than deletes wherever a successor exists
- [ ] `ai/rules/repo-maintenance.md` - which gate owns which surface
  → Constraint: widening a gate's corpus is a gate change and belongs in the inventory

**Key insights:**
- A citation is not always repointable. The numbered file is gone, so a repair
  is one of three moves: point at the journal row that replaced it, restate the
  fact inline, or delete the citation.
- The three repaired during the suppression phase were all deletions, because
  each named a file whose content had moved into a journal class with no
  one-to-one successor.
- `make ze-spec-citation-check` already gates dangling `plan/spec-*.md`
  citations, with a grandfathering baseline at `plan/.citation-baseline`. That
  is the shape a `plan/learned/` equivalent can copy.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/check_doc_links.py` - `check_markdown` iterates `MD_GLOBS` and checks backticked paths and markdown links; `candidate_paths` drops a token with no `/`; `MD_GLOBS` names fourteen globs and excludes `docs/`, `plan/deferrals/` and every `plan/spec-*.md`
- [ ] `mk/inventory.mk` - `ze-spec-citation-check`, the sibling gate for dangling spec citations, and its grandfathering baseline
- [ ] `scripts/dev/learned_staleness.py` - `path_problem` measures the OTHER direction, paths cited from inside the corpus <!-- doc-links: ignore (deleted with the learned corpus in 2cff2050a; named here as the prior art this spec read) -->

**Behavior to preserve:**
- `check_markdown` keeps skipping `plan/handover/`: a historical record describes
  the tree as it was, which the module docstring already states.
- The `doc-links: ignore` grammar landed by
  spec-doc-claims-are-checked-not-just-resolved stays as it is. A citation whose
  path is deliberately unresolvable states its reason and is audited.

**Behavior to change:**
- A dead citation in a file the corpus does not walk becomes visible.
- The 451 citations are repaired: repointed, restated, or deleted.
- A dead citation that predates the gate is grandfathered as a
  (citer, target) pair and can only be removed, never added to.

## Data Flow (MANDATORY)

### Entry Point
- `make ze-doc-links`, a stage of `make ze-verify` per `stagesForMode` in
  `scripts/status/verify_run.go`.

### Transformation Path
1. `check_markdown` enumerates files from `MD_GLOBS`.
2. It extracts backticked tokens and markdown links per line.
3. A file outside the fourteen globs contributes no tokens, so its dead citation is invisible.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Gate ↔ the corpus it walks | `MD_GLOBS`, a hand-written list | No |
| Gate ↔ non-markdown citers | the `Makefile`, `mk/`, `.claude/hooks/*.py` carry citations too | No |

### Integration Points
- `scripts/dev/check_doc_links.py` - `MD_GLOBS`, or a sweep like `check_ignore_reasons` which already reads every tracked file
- `ai/rules/repo-maintenance.md` via its point files - the gate inventory

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | `check_ignore_reasons` already sweeps every tracked file; reuse that walk |
| Zero-copy preserved where applicable | N-A | no wire path |
| Registration over hardcoding | No | `MD_GLOBS` is the hardcoding |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each of the 172 dead paths has a repair: a journal row, an inline restatement, or a deletion | 3 of 3 repaired so far were deletions | the corpus cannot be cleaned and the gate cannot be widened | classify all 172 before widening | CONFIRMED: `26684a4bc` repaired all but 3, which this spec repairs |
| A-2 | Widening the walked corpus reds only on dead `plan/learned/` citations | unmeasured; `docs/` alone holds thousands of backticked paths | the widening is a tree-wide red nobody can clear | run the widened check before arming and count by root | BROKEN 2026-08-10: 2953 findings over 1373 paths. The if-wrong branch is what happened, and the baseline ratchet is its answer |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Widening `check_markdown` to `docs/` floods on illustrative paths in prose | findings on example config paths | widen by root, arming one root at a time, each with its count |
| R-2 | A citation is deleted where the fact it carried is lost | a repaired line reads as an assertion with no source | prefer repointing to the journal class; delete only when the content moved with no successor |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A link gate reds on a correct citation. Nothing user-visible |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Every session that writes a doc or a rule |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-doc-links` | → | the widened citation sweep | `test_dead_citation_outside_md_globs_is_reported` |
| `make ze-doc-links` | → | the repaired tree | `test_real_corpus_has_no_dead_learned_citation` |

### Functional Tests

No `.ci` applies: the subject is a documentation gate with no user-facing
surface and no daemon code. A `.ci` drives a `ze` process and could assert
nothing here. The driving surface is `scripts/dev/check_doc_links_test.py`.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test_real_corpus_has_no_dead_learned_citation` | `scripts/dev/check_doc_links_test.py` | an agent cannot leave a citation to a retired document | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A NEW dead citation in a file outside `MD_GLOBS` | the gate names the file, the line and the path, and exits 1 |
| AC-2 | A live citation in the same file | no finding |
| AC-3 | The 3 dead `plan/learned/` citations that survive `26684a4bc` | each repointed, restated, or deleted, and none is baselined |
| AC-4 | The whole tree after the work | `make ze-doc-links` green |
| AC-5 | A (citing file, dead target) pair recorded in the baseline | no finding: it is the grandfathered population |
| AC-6 | A pair the baseline lists that no longer occurs | reported as drift, non-fatal, so the baseline shrinks deliberately |
| AC-7 | A NEW citer of a target another file already has baselined | reported. The baseline grandfathers pairs, never bare targets, so rot cannot spread to a new file under cover of an old entry |
| AC-8 | `ai/digests/` line-number notation `file.go,62,152,169` | the path is checked and the line-number run is stripped, so a live file is not reported |
| AC-9 | A baseline committed with MORE entries than the one at HEAD | the gate fails. Shrink-only is the property Thomas chose, so it is enforced against HEAD rather than written in a header comment. Without it, `--write-baseline` is a one-command escape from every other AC |

## 🧪 TDD Test Plan

### Unit Tests
Every row below was proved to DISCRIMINATE by mutation: the behaviour was
reverted one mutation at a time and the owning test went red each time
(`ai/rules/interop-and-goal-validation.md`). 45 tests pass.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_dead_citation_outside_md_globs_is_reported` | `scripts/dev/check_doc_links_test.py` | AC-1 | green |
| `test_live_citation_outside_md_globs_passes` | `scripts/dev/check_doc_links_test.py` | AC-2 | green |
| `test_real_corpus_has_no_dead_learned_citation` | `scripts/dev/check_doc_links_test.py` | AC-3 | green |
| `test_baselined_pair_is_not_reported` | `scripts/dev/check_doc_links_test.py` | AC-5 | green |
| `test_stale_baseline_entry_is_reported_as_drift` | `scripts/dev/check_doc_links_test.py` | AC-6 | green |
| `test_new_citer_of_baselined_target_is_reported` | `scripts/dev/check_doc_links_test.py` | AC-7 | green |
| `test_digest_line_number_run_is_stripped` | `scripts/dev/check_doc_links_test.py` | AC-8 | green |
| `test_unreadable_file_is_a_finding` | `scripts/dev/check_doc_links_test.py` | the sweep fails closed, as `check_ignore_reasons` does | green |
| `test_baseline_that_grew_against_head_is_refused` | `scripts/dev/check_doc_links_test.py` | AC-9 | green |
| `test_baseline_pair_absent_from_head_is_refused_at_equal_size` | `scripts/dev/check_doc_links_test.py` | AC-9, the swap a count comparison let through | green |
| `test_no_head_baseline_yet_passes` | `scripts/dev/check_doc_links_test.py` | the arming commit itself must run | green |
| `test_vendored_and_handover_citations_are_not_swept` | `scripts/dev/check_doc_links_test.py` | the three excluded roots | green |

## Files to Modify
- `scripts/dev/check_doc_links.py` - `check_tracked_citations`, the tree-wide sweep, and `candidate_paths` for the line-number run
- `scripts/dev/check_doc_links_test.py` - its tests
- the 3 files citing a dead `plan/learned/` path, and the ~10 carrying a false-positive class
- `ai/rules/repo-maintenance.md` via its point files - the gate inventory
- `docs/contributing/documentation-testing.md` - the new gate and how to shrink its baseline

## Files to Create
- `scripts/dev/doc_citation_baseline.txt` - the shrink-only grandfathered population, beside `core_import_baseline.txt` and `tier_migration_baseline.txt`, which are the two other tree-wide baselines kept there

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config surface |
| YANG validation constraints | N-A | no config surface |
| YANG custom validators | N-A | no config surface |
| CLI commands/flags | N-A | a make target |
| CLI grammar (keyword before value) | N-A | no CLI surface |
| Editor autocomplete | N-A | no config surface |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no route output |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | no daemon state |
| BGP family surface | N-A | no protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | agent tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/documentation-testing.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the `ai/rules/repo-maintenance.md` gate list |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on `check_doc_links.py` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Classify the 172** -- DONE by `26684a4bc`; 3 survive
   - Verify: A-1 answered with numbers
2. **Phase: Repair** -- the 3 survivors are corrected, and none is baselined
   - Verify: `test_real_corpus_has_no_dead_learned_citation`
3. **Phase: Measure the widening** -- DONE 2026-08-10: 2953 findings, 1373 distinct paths
   - Verify: A-2 answered with numbers, and it is BROKEN
4. **Phase: The false-positive classes** -- the parser gap is fixed and the two placeholder classes carry a reasoned marker
   - Tests: `test_digest_line_number_run_is_stripped`
5. **Phase: Arm** -- `check_tracked_citations` sweeps every tracked file against a shrink-only baseline
   - Tests: `test_dead_citation_outside_md_globs_is_reported`, `test_baselined_pair_is_not_reported`, `test_new_citer_of_baselined_target_is_reported`, `test_stale_baseline_entry_is_reported_as_drift`
6. **Phase: The inventory** -- the gate list records what the corpus now covers, then `make ze-rules-condensed`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | A deliberately unresolvable path keeps its reasoned marker and stays passing |
| Naming | The finding names the file, the line and the path |
| Data flow | One tracked-file sweep, shared with `check_ignore_reasons`, not a second walk |
| Rule: `ai/rules/evidence.md` | The sweep fails closed: an unreadable file is a finding |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No dead citation to the retired corpus survives | the reproduction command returns 0 |
| The gate sees a citation outside `MD_GLOBS` | `python3 scripts/dev/check_doc_links_test.py` |
| The tree is clean under it | `make ze-doc-links` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Repo-authored input only. An unreadable file is a finding, never a skip |

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

## Design Insights

- Retiring a document class kills every pointer to it. Spec closure has the same
  shape and the same consequence, which
  `plan/journal/gate-excludes-part-of-its-population.md` records for the
  `_test.go` Design pointers that `scripts/dev/check_doc_links.py` now reads.
  The retirement is correct; leaving the citers is the defect.
- A gate with a hand-written corpus list is a gate whose coverage nobody can
  state. `check_ignore_reasons` reads every tracked file, so the walk that
  answers this already exists in the same module.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Repair the citations before widening the corpus | widen first and let the gate list the work | A widening that reds hundreds of lines cannot land in one commit, and a red gate that stays red teaches everyone to bypass it |
| Widen tree-wide behind a shrink-only baseline (Thomas, 2026-08-10) | arm only the roots repairable today, leaving `plan/spec-*.md` and `docs/` ungated; or repair all 1373 dead paths first | The measurement made the first decision unpayable: repair-then-widen assumed a 451-citation population and the real one is 2953 over 1373 targets. The ratchet keeps both properties that mattered -- the tree is green today, and a NEW dead citation anywhere fails -- while the arithmetic backlog is published rather than hidden. Arming a subset instead leaves the two largest surfaces silent, which is the hole this spec exists to close |
| Grandfather (citer, target) PAIRS, not bare targets | list the dead target once, as `plan/.citation-baseline` does | A bare target lets rot spread: any new file could cite an already-dead path for free. The sibling gate can afford it because its population is one directory of specs; this one spans the tree |
| Fix the three false-positive classes at the source | baseline them with everything else | A baseline entry is a promise that someone can shrink it. A line-number notation the parser misreads is not shrinkable by any repair, so it would sit there forever and teach the reader that the baseline is noise |

## Known Limitations
- A citation that resolves and is wrong about what it cites still passes. This
  closes the resolution half.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional coverage: the gate runs inside `make ze-doc-links`
- [ ] Interop tests N-A: no protocol surface

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Journal row written for anything this teaches
- [ ] **Commit A:** code + tests + docs + spec + journal row
- [ ] **Commit B:** `git rm plan/spec-dead-learned-citations-outside-the-walked-corpus.md` only

## Implementation Summary

### What Was Implemented
- `scripts/dev/check_doc_links.py`: check 5, `check_tracked_citations` (`:866`).
  `sweep_tracked` (`:778`) is the one walk over every tracked file. It returns
  unreadable files, marker findings and dead `(citer, line, target)` triples.
  `check_ignore_reasons` (`:856`) and `check_tracked_citations` are thin wrappers
  over it, and `main` calls it once.
- `CITATION_EXCLUDE_PREFIXES` (`:572`) holds the three excluded roots:
  `vendor/`, `third_party/` and `plan/handover/`.
- `scripts/dev/doc_citation_baseline.txt`: 1426 `(citer, target)` pairs over 305
  citers and 1259 targets. `check_baseline_growth` (`:647`) compares the file
  against its own version at HEAD as a SET and names each pair HEAD does not
  hold.
- `candidate_paths` (`:243`) strips the digest line-number run through
  `LINE_RUN_SUFFIX` (`:140`), so `attrparse.go,62,152,169` is checked as
  `attrparse.go`.
- 164 lines across the tree gained a reasoned `doc-links: ignore`. They cover MCP
  method names, hook placeholder paths and format templates.
- `scripts/dev/check_doc_links_test.py`: 47 tests, 14 of them new for this spec.
- The inventory records the gate in `ai/rules/points/repo-maintenance/discovery-updates/the-discovery-surface-that-answers-each-need.md`,
  in `ai/rules/points/planning/spec-closure-blocking/what-each-reference-gate-reads.md`,
  in `ai/INDEX.md` and in `docs/contributing/documentation-testing.md`.

### Bugs Found/Fixed
- The shrink-only ratchet compared the baseline SIZE against HEAD. One repair plus
  one new dead pair kept the total and let the new pair through.
  `check_baseline_growth` now compares sets.
  `test_baseline_pair_absent_from_head_is_refused_at_equal_size` holds the size
  constant, so a count comparison fails it.
- `ruff format` moved a marker off its path's line in
  `scripts/dev/rfc_requirements_test.py`. The marker now sits on the argument
  line itself.
- The baseline header said the gate fails when the count grew. The header now
  states the set comparison.
- The header lived in TWO places: the file on disk and an inline string inside
  `write_baseline`. The correction reached the file and not the generator, so the
  next `--write-baseline` would have reverted it. No gate reads a comment, so the
  divergence was reported during the session rather than caught. `BASELINE_HEADER`
  (`:742`) is now the one home, and
  `test_baseline_header_is_what_the_generator_writes` (`check_doc_links_test.py:942`)
  compares it against the committed file.
- `parse_baseline` said a malformed entry announces itself. Nothing reported one.
  `baseline_format_problems` (`:621`) reports a baseline line with no TAB, `main`
  calls it beside `check_baseline_growth`, and
  `test_a_baseline_line_with_no_tab_is_reported` (`check_doc_links_test.py:965`)
  covers it.
- `MARKER_SWEEP_EXCLUDE` is renamed `SWEEP_EXCLUDE` (`:162`). It now excludes from
  checks 4 and 5, and no caller outside the module names it.

### Documentation Updates
- `docs/contributing/documentation-testing.md`: the check count, the source
  anchor naming `check_tracked_citations`, `sweep_tracked` and
  `check_baseline_growth`, the situation table, and a paragraph on the baseline.
- `ai/INDEX.md:203`: the `check_doc_links.py` row names five checks.
- Two rule point files, rendered into `ai/rules/repo-maintenance.md` and
  `ai/rules/planning.md` by `make ze-rules-condensed`.
- `make ze-doc-test`: exit 2 on two artifacts this change does not touch. See
  Pre-Commit Verification.

### Deviations from Plan
- A-2 is BROKEN. The spec planned to repair the citations and then widen. The
  measurement found 2953 findings over 1373 targets, which no single spec can
  repair. Thomas chose the shrink-only baseline on 2026-08-10, and the spec
  records that decision.
- The three false-positive classes are fixed at the source rather than
  baselined. A baseline of false positives can never shrink.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 said the widening reds only on dead `plan/learned/` citations | 2953 findings over 1373 distinct targets, `plan/spec-*.md` holding 2085 | ran the widened check before arming and counted by root | the shrink-only baseline, chosen by Thomas |
| approach | The ratchet compared the baseline size against HEAD | a swap of one pair for another keeps the size and defeats the check | writing the test that holds the size constant | `check_baseline_growth` compares sets |
| approach | The baseline header was corrected on disk and the generator kept the old text | one truth had two homes, and `--write-baseline` would have reverted the correction | the two artifacts were compared during the session, and no gate reads a comment | `BASELINE_HEADER` is the one home, and a test compares it against the committed file |
| approach | `parse_baseline`'s docstring said a malformed entry announces itself | nothing reported one | reading the docstring against the module's checks | `baseline_format_problems`, called by `main`, makes the claim true |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| No dead citation to the retired corpus survives | Done | `scripts/dev/check_doc_links_test.py:885` | `test_real_corpus_has_no_dead_learned_citation` |
| A citation outside `MD_GLOBS` is checked | Done | `check_doc_links.py:866` `check_tracked_citations` | one walk, `sweep_tracked:778` |
| Today's survivors are grandfathered and can only shrink | Done | `check_doc_links.py:647` `check_baseline_growth` | set comparison against HEAD |
| The false-positive classes are fixed at the source | Done | `candidate_paths:243`, 164 reasoned markers | none of the three is baselined |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test_dead_citation_outside_md_globs_is_reported:585` | names file, line and path |
| AC-2 | Done | `test_live_citation_outside_md_globs_passes:601` | |
| AC-3 | Done | `test_real_corpus_has_no_dead_learned_citation:885` | none of the 3 is baselined |
| AC-4 | Done | `make ze-doc-links` | one error, and it belongs to another session |
| AC-5 | Done | `test_baselined_pair_is_not_reported:612` | |
| AC-6 | Done | `test_stale_baseline_entry_is_reported_as_drift:622` | drift is a WARN and exit stays 0 |
| AC-7 | Done | `test_new_citer_of_baselined_target_is_reported:641` | pairs, never bare targets |
| AC-8 | Done | `test_digest_line_number_run_is_stripped:114` | |
| AC-9 | Done | `test_baseline_that_grew_against_head_is_refused:664`, `test_baseline_pair_absent_from_head_is_refused_at_equal_size:701` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| all 12 rows of the TDD table | Done | `scripts/dev/check_doc_links_test.py` | 47 tests OK, 36s |
| `test_baseline_header_is_what_the_generator_writes` | Done | `check_doc_links_test.py:942` | added after the TDD plan, mutation-proved by drifting `BASELINE_HEADER` |
| `test_a_baseline_line_with_no_tab_is_reported` | Done | `check_doc_links_test.py:965` | added after the TDD plan, mutation-proved by stubbing `baseline_format_problems` to `[]` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/check_doc_links.py` | Done | check 5 and `candidate_paths` |
| `scripts/dev/check_doc_links_test.py` | Done | 14 new tests |
| the 3 dead `plan/learned/` citers and the false-positive files | Done | 1 repair, 164 reasoned markers |
| `ai/rules/repo-maintenance.md` via its point file | Done | rendered, never hand-edited |
| `docs/contributing/documentation-testing.md` | Done | five checks, anchor, baseline |
| `scripts/dev/doc_citation_baseline.txt` | Done | created, 1426 pairs |

### Audit Summary
- **Total items:** 21
- **Done:** 21
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (A-2 broken, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A dead citation in a file the gate does not walk becomes visible | functional (gate test) | `test_dead_citation_outside_md_globs_is_reported`, proved by mutation: reverting the sweep to `MD_GLOBS` makes it fail |
| The dead `plan/learned/` citations are gone | functional over the real tree | `test_real_corpus_has_no_dead_learned_citation` reads the repository itself, not a fixture |
| Rot cannot spread under cover of an old baseline entry | functional (gate test) | `test_new_citer_of_baselined_target_is_reported` |
| The baseline can only shrink | functional (gate test) | `test_baseline_pair_absent_from_head_is_refused_at_equal_size` holds the total constant, so a count comparison passes and the set comparison refuses |
| The tree is green under the widened gate | gate run | `make ze-doc-links`: 1 error, on `plan/spec-fixit-mgmt-listener-auth-guard.md:545`, a line another session added and has not committed |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| 451 citations of the retired learned corpus, in files `check_markdown` does not walk | done | this spec. `26684a4bc` repaired all but 3, this spec repaired those 3, and `check_tracked_citations` sweeps every tracked file |
| Every doc-freshness gate verifies reference integrity and none verifies claim truth | done | closed by spec-doc-claims-are-checked-not-just-resolved on 2026-08-10 |
| Full audit of whether the rule corpus keeps design documentation current | resolved | 14 findings, 3 specs written |
| Code can land with no design doc: 85 package directories carry no anchor | open | homed at `plan/spec-code-can-land-with-no-design-doc.md`, which this spec does not close |

The shard `plan/deferrals/doc-claims-are-checked-not-just-resolved.md` keeps one
live row, so it is NOT removed. `deferral_shard_removal_problems` in
`scripts/dev/commit_helper.py` blocks the removal of a shard that still holds a
live row.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | none. `--review-override` on the owner's cost decision, 2026-08-10 |
| `review_gate.py check` | not run: the override replaces the artifact |
| Rounds | 1, in the main thread over the complete diff |
| Reviewer lenses used | logic and guards, generated-artifact drift, documentation accuracy |

The three-lens reviewer fan-out was estimated at 400-700k tokens against a
commit of about 40k. Thomas ruled on cost and the fan-out did not run. The main
thread read the complete diff instead: `scripts/dev/check_doc_links.py` and
`scripts/dev/check_doc_links_test.py` hunk by hunk, plus `ai/INDEX.md`,
`docs/contributing/documentation-testing.md` and the `ai/rules/points/` changes.
This review is NOT independent of the author, because the main thread supervised
the phases. Five defects were found and fixed, three of them by that read.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `write_baseline` held the baseline header inline while the committed file carried a corrected copy. `--write-baseline` would have reverted the correction | `scripts/dev/check_doc_links.py`, `write_baseline` | `BASELINE_HEADER` is the one home, and `test_baseline_header_is_what_the_generator_writes` compares it against the committed file |
| 2 | BLOCKER | `check_baseline_growth` compared COUNTS, so a repair plus a new dead citation in one commit passed at equal size | `scripts/dev/check_doc_links.py`, `check_baseline_growth` | compares the sets, with `test_baseline_pair_absent_from_head_is_refused_at_equal_size` |
| 3 | ISSUE | `parse_baseline`'s docstring claimed a malformed entry announces itself, and nothing reported one | `scripts/dev/check_doc_links.py`, `parse_baseline` | `baseline_format_problems`, called from `main`, with `test_a_baseline_line_with_no_tab_is_reported` |
| 4 | ISSUE | `ai/INDEX.md` described `check_baseline_growth` as refusing a baseline larger than the one at HEAD: the count semantics finding 2 removed | `ai/INDEX.md:203` | the row states the set comparison |
| 5 | ISSUE | The same false count sentence, and a `--write-baseline` instruction that did not say the command reads the WORKING TREE | `docs/contributing/documentation-testing.md` | both corrected, and the working-tree warning is written down |

Findings 4 and 5 are the same class as finding 1: a claim about behaviour, in a
surface no gate reads. Requirement 11 of `ai/rules/completion.md` is that class
written into the rule corpus, and it rides on commit C.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `scripts/dev/doc_citation_baseline.txt` | Yes | `grep -vc '^#\|^$'` prints 1426 pairs |
| `scripts/dev/check_doc_links_test.py` | Yes | `python3 scripts/dev/check_doc_links_test.py` ran 47 tests |
| No `.ci` applies | N-A | the subject is a documentation gate with no daemon surface |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-9 | every AC has a named test | `python3 scripts/dev/check_doc_links_test.py`: `Ran 47 tests in 35.518s`, `OK` |
| AC-4 | the tree is green | `make ze-doc-links`: 1 broken reference, on `plan/spec-fixit-mgmt-listener-auth-guard.md:545` |
| AC-3 | none of the 3 is baselined | `grep -c 'plan/learned' scripts/dev/doc_citation_baseline.txt` is 0 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-doc-links` | none, `scripts/dev/check_doc_links_test.py` drives it | Yes: `main` calls `sweep_tracked` and `check_baseline_growth`, both read the real tree |
| `make ze-doc-links` over the repaired tree | `test_real_corpus_has_no_dead_learned_citation` | Yes: the test opens the repository, not a fixture |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `26684a4bc` repaired all but 3 of the 172 paths, and this spec repaired those 3 |
| A-2 | broken | 2953 findings over 1373 distinct targets. The shrink-only baseline is the answer Thomas chose |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 10, test infrastructure | `docs/contributing/documentation-testing.md` names five checks and anchors `check_tracked_citations`, `sweep_tracked` and `check_baseline_growth`, all three present in `scripts/dev/check_doc_links.py` | Yes |
| Row 15, gate inventory | the discovery-surface point file carries the citation-sweep row, rendered into `ai/rules/repo-maintenance.md` | Yes |
| Row 16, source anchors | the anchor in `docs/contributing/documentation-testing.md` is the only one naming `check_doc_links.py` | Yes |
| Rows 1..9, 11..14, 17 | No: the change adds no user-facing feature, no config, no CLI, no RPC, no plugin, no wire format and no RFC behaviour | Yes |
| `make ze-doc-test` | exit 2 on `ai/rules/INDEX.md` and `ai/RFC-REQUIREMENTS.md`, both stale from other sessions. See the attribution below | Yes |

**Attribution of the reds this session did not cause.** A shared checkout never
gives a clean full gate (`ai/rules/git-safety.md`).

| Red | Attribution |
|-----|-------------|
| `make ze-doc-links`: `plan/spec-fixit-mgmt-listener-auth-guard.md:545` cites a deleted rule file | The backticked citation is a `+` line in that file's uncommitted diff. HEAD carries the name only as unbackticked prose at `:384`, so HEAD is green on this file |
| `make ze-doc-test`: `ai/rules/INDEX.md` stale | The one stale row is the `ai/rules/evidence.md` summary. Another session changed that rule's `**When:**` line |
| `make ze-doc-test`: `ai/RFC-REQUIREMENTS.md` stale | Its sources `rfc/enrolled.txt`, `rfc/not-enrolled.txt` and three `rfc/short/*.md` files are modified by another session |
| `make ze-lint-changed`: typecheck | `internal/component/ike/dataplane/vpp_extra_test.go` calls `vppCryptoAlg` with one return value. Another session changed that signature |
| `scripts/dev/rfc_requirements_test.py`: 1 of 763 | `TestRealTreeIsGreen` fails because `internal/component/ike/dataplane/vpp.go:683` says `undefined: spdID`. Same session, same package |

## Core Insight

A gate whose corpus is a hand-written list states a coverage claim nobody can
check. The walk that answers it already existed in the same module, and the
fourteen globs hid 2953 dead references from every reader.
