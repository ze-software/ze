# Spec: dead-learned-citations-outside-the-walked-corpus

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/doc-claims-are-checked-not-just-resolved.md` |
| Updated | 2026-08-09 |

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
- [ ] `scripts/dev/learned_staleness.py` - `path_problem` measures the OTHER direction, paths cited from inside the corpus

**Behavior to preserve:**
- `check_markdown` keeps skipping `plan/handover/`: a historical record describes
  the tree as it was, which the module docstring already states.
- The `doc-links: ignore` grammar landed by
  spec-doc-claims-are-checked-not-just-resolved stays as it is. A citation whose
  path is deliberately unresolvable states its reason and is audited.

**Behavior to change:**
- A dead citation in a file the corpus does not walk becomes visible.
- The 451 citations are repaired: repointed, restated, or deleted.

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
| A-1 | Each of the 172 dead paths has a repair: a journal row, an inline restatement, or a deletion | 3 of 3 repaired so far were deletions | the corpus cannot be cleaned and the gate cannot be widened | classify all 172 before widening | unvalidated |
| A-2 | Widening the walked corpus reds only on dead `plan/learned/` citations | unmeasured; `docs/` alone holds thousands of backticked paths | the widening is a tree-wide red nobody can clear | run the widened check before arming and count by root | unvalidated |

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
| AC-1 | A dead citation in a file outside `MD_GLOBS` | the gate names the file, the line and the path |
| AC-2 | A live citation in the same file | no finding |
| AC-3 | The 172 dead paths measured on 2026-08-09 | each repointed, restated, or deleted, with the choice recorded |
| AC-4 | The whole tree after the work | `make ze-doc-links` green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_dead_citation_outside_md_globs_is_reported` | `scripts/dev/check_doc_links_test.py` | AC-1 | |
| `test_live_citation_outside_md_globs_passes` | `scripts/dev/check_doc_links_test.py` | AC-2 | |
| `test_real_corpus_has_no_dead_learned_citation` | `scripts/dev/check_doc_links_test.py` | AC-4 | |

## Files to Modify
- `scripts/dev/check_doc_links.py` - the walked corpus
- `scripts/dev/check_doc_links_test.py` - its tests
- the citing files, across `docs/`, `plan/`, `mk/`, the `Makefile` and `.claude/hooks/`
- `ai/rules/repo-maintenance.md` via its point files - the gate inventory

## Files to Create
- none expected

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

1. **Phase: Classify the 172** -- each dead path gets a repointing target, an inline restatement, or a deletion
   - Verify: A-1 answered with numbers
2. **Phase: Repair** -- the 451 citations are corrected
   - Verify: the reproduction command returns zero dead paths
3. **Phase: Measure the widening** -- run the widened corpus check and count findings by root
   - Verify: A-2 answered with numbers
4. **Phase: Arm** -- the widened sweep fails the gate
   - Tests: `test_dead_citation_outside_md_globs_is_reported`
5. **Phase: The inventory** -- the gate list records what the corpus now covers, then `make ze-rules-condensed`

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
