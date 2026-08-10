# Spec: fixit-spec-closure-leaves-dangling-spec-citations

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | `plan/deferrals/fixit-dead-design-pointers-in-tests.md` |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`make ze-spec-citation-check` is RED at HEAD. 15 citations of a `plan/spec-*.md`
name a spec that no longer exists and is not in the grandfathering baseline
`plan/.citation-baseline`. The check is a stage of `make ze-verify-wiring-docs`,
so that target cannot go green until the 15 are cleared.

The cause is the closure convention, the same one
`plan/journal/gate-excludes-part-of-its-population.md` records for the
`_test.go` Design pointers.
Spec closure is two commits, and commit B is `git rm plan/spec-<stem>.md`
(`ai/rules/planning.md`, "Spec Closure"). `/ze-close` does not add the removed
stem to `plan/.citation-baseline` and does not repoint the specs that cite it,
so every closure that lands while a sibling cites it adds a red row.

Found while implementing phase 3 of
`spec-fixit-dead-design-pointers-in-tests`. Phase 3 repointed 145
`// Design:` pointers and took the Design gate to zero. It touched no `plan/`
file, so it neither caused nor cleared these rows. That spec then closed on
2026-08-10 (`90082fb08`), which is what took the count from 12 to 15.

Reproduction, re-measured 2026-08-10:

| Command | Result |
|---------|--------|
| `python3 scripts/dev/spec-citation-check.py` | exit 1, 15 dangling reference(s) |
| `make ze-verify-wiring-docs` | exit 2, same stage |

Every one of the 15 is dangling at HEAD as well as in the working tree: for each
row, `git show HEAD:<citing-file>` holds the citation and
`git show HEAD:<cited-spec>` fails.

The producing function is `find_dangling` in
`scripts/dev/spec-citation-check.py`. It scans `spec_files()` and
`learned_files()` for `SPEC_REF_RE`, drops anything in `load_baseline()`, and
reports the rest. The check is correct; the closure workflow that feeds it is
the defect.

Seven distinct dead targets carry the 15 rows, measured 2026-08-10. Their stems
are written here without the `plan/` prefix and the `.md` suffix on purpose:
spelled in full they are themselves dangling citations, and this file would add
seven rows to the count it exists to clear.

| Dead stem | Rows |
|-----------|------|
| `spec-fixit-dead-design-pointers-in-tests` | 6 |
| `spec-wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal` | 4 |
| `spec-problem-journal` | 1 |
| `spec-fixit-vacuous-eor-family-tests` | 1 |
| `spec-fixit-rs-community-strip-arity-deferred-removevalues-quality` | 1 |
| `spec-gokrazy-init-bump` | 1 |
| `spec-rfc7606-5-1-2-relay-shape` | 1 |

Four of the 15 sit in this file. This spec's own commit B removes it, so those
four would clear at its closure. They are repaired now regardless. A spec that
carries the class it exists to clear cannot show the gate green while it is
open. The repair is the bare-stem restatement the table above uses.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/planning.md` - "Spec Closure", the two commits that remove the target
  → Constraint: commit B keeps removing the spec. The removal is correct; leaving the citers is the defect
- [ ] `ai/rules/repo-maintenance.md` - which gate owns which surface
  → Constraint: a new step in a closure skill is a workflow change and belongs in the discovery surfaces

**Key insights:**
- A repair is one of three moves: repoint the citation at the durable document
  that replaced the spec, restate the fact inline, or add the stem to
  `plan/.citation-baseline` when the citation is a historical record.
- The baseline already carries 46 stems, so grandfathering is the sanctioned
  route. Nothing adds to it automatically, which is why it drifts.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/spec-citation-check.py` - `find_dangling` reports a cited `plan/spec-*.md` that is absent and unbaselined; `load_baseline` reads `plan/.citation-baseline`; `write_baseline` regenerates it wholesale
- [ ] `mk/inventory.mk` - the `ze-spec-citation-check` target runs the script with no arguments
- [ ] `ai/skills/ze-close.md` - the closure steps that write commit B. This is the canonical source. `scripts/dev/skill_sync.sh` generates `.claude/skills/ze-close/SKILL.md` from it, and that copy is gitignored

**Behavior to preserve:**
- Commit B keeps removing the spec file.
- `--write-baseline` stays a deliberate operator action, never a step a failing
  run takes for itself.

**Behavior to change:**
- Closing a spec that a sibling cites stops leaving a red row behind.
- The 15 rows measured on 2026-08-10 are cleared.

## Data Flow (MANDATORY)

### Entry Point
- `make ze-verify-wiring-docs`, and `make ze-spec-citation-check` on its own.

### Transformation Path
1. `spec_files()` and `learned_files()` list the citing corpus.
2. `SPEC_REF_RE` extracts every `plan/spec-*.md` token.
3. `find_dangling` reports a token whose file is absent and whose stem is not baselined.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Closure workflow ↔ the citation gate | nothing connects them; the baseline is hand-edited | Yes: `write_baseline` has no caller in `/ze-close` |

### Integration Points
- `.claude/skills/ze-close/SKILL.md` and `ai/skills/ze-close/SKILL.md` - the closure steps
- `plan/.citation-baseline` - the grandfathering list

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | N-A | one script and one skill |
| No duplicated functionality (extends existing, does not recreate) | No | the baseline mechanism exists; nothing feeds it |
| Zero-copy preserved where applicable | N-A | no wire path |
| Registration over hardcoding | N-A | no plugin surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each of the 15 rows has a repair: a repoint, an inline restatement, or a baseline row | 4 of them vanish at this spec's own closure | the gate cannot be cleared without dropping content | classify all 15 before you edit | confirmed 2026-08-10: every row is classified in "Citation repair decisions", 5 repoints and 10 restatements, 0 baseline rows and 0 sentences dropped |
| A-2 | Clearing the 15 takes `make ze-verify-wiring-docs` green | the log names one failing stage | a second red is hiding behind the first | run the target after the repairs | confirmed 2026-08-10: the target exits 0 over 13 stages, `ze-spec-citation-check PASSED` among them. No second red was hiding |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The class regrows at the next closure | a red row on an unrelated commit | the closure skill checks the gate before commit B, so the author who removes the target is the author who clears the citers |
| R-2 | A blanket `--write-baseline` hides a repointable citation | the baseline grows faster than the corpus closes | baseline only a citation that is a historical record, and say so per row |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. A documentation gate and spec prose |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Every spec closure |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-spec-citation-check` | → | `find_dangling` over the repaired tree | `test_real_corpus_has_no_dangling_spec_citation` |

### Functional Tests

No `.ci` applies: the subject is a documentation gate with no user-facing
surface and no daemon code.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test_real_corpus_has_no_dangling_spec_citation` | `scripts/dev/spec_citation_check_test.py` | an agent cannot close a spec and leave a dangling citation | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The repo as it stands | `make ze-spec-citation-check` reports zero dangling references |
| AC-2 | The repo as it stands | `make ze-verify-wiring-docs` is green |
| AC-3 | A spec is closed while a sibling cites it | the closure workflow names the citers before commit B |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_real_corpus_has_no_dangling_spec_citation` | `scripts/dev/spec_citation_check_test.py` | AC-1 | |

## Files to Modify
- the citing specs, for each citation that repoints or restates: 8 files, listed
  in "Citation repair decisions"
- `plan/.citation-baseline` - unchanged. No row needed grandfathering, so it
  stays at 46 entries
- `ai/skills/ze-close/SKILL.md` - the closure step that stops the regrowth

## Files to Create
- `scripts/dev/spec_citation_check_test.py` - if the script has no test today

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
| 16 | Any changed source file referenced by existing doc source anchors? | No | |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Classify the 15** -- each row gets a repoint, a restatement, or a baseline entry
   - Verify: A-1 answered per row
2. **Phase: Repair** -- apply the 15 decisions
   - Verify: `make ze-spec-citation-check` reports zero
3. **Phase: Stop the regrowth** -- the closure skill runs the gate before commit B
   - Verify: `make ze-verify-wiring-docs` green, and the step is in the skill

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | A repointed citation names a document that exists and covers the subject |
| Data flow | The closure skill reads the gate; no second copy of the citation parser |
| Rule: `ai/rules/simplicity.md` | The baseline mechanism already exists. Feed it, do not replace it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No dangling spec citation survives | `make ze-spec-citation-check` |
| The wiring-docs gate is green | `make ze-verify-wiring-docs` |

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

- Removing a document kills every pointer to it. This is the third corpus where
  the same closure convention produced the same class:
  `spec-fixit-dead-design-pointers-in-tests` for `_test.go` pointers,
  `spec-dead-learned-citations-outside-the-walked-corpus` for the
  retired learned corpus, and this one for spec-to-spec citations. The gate
  exists here and is red, so the missing part is the closure step that feeds it.
- A dead stem named in prose is not a pointer, and spelling it as one is what
  makes the class self-inflicted. `SPEC_REF_RE` in
  `scripts/dev/spec-citation-check.py` matches `plan/spec-<stem>.md` and nothing
  else. So `spec-<stem>` in backticks keeps the name a reader needs. It drops
  the resolution promise the file can no longer keep.

### Citation repair decisions

Every one of the 15 rows measured on 2026-08-10, and the move it took. No row
took the baseline move, so `plan/.citation-baseline` is unchanged at 46 entries.
Each dead stem either has a live document that covers its subject, or is named
in prose as history. R-2 says to restate the second kind rather than grandfather
it. The citing site is named by its section rather than by a line number,
because the lines move and the sections do not.

| Citing file (section) | Dead stem | Move | Reason |
|-----------------------|-----------|------|--------|
| `spec-dead-learned-citations-outside-the-walked-corpus` (untracked, another session's), Design Insights | `spec-fixit-dead-design-pointers-in-tests` | Repoint | The sentence points at the record of the `_test.go` class. That record is now `plan/journal/gate-excludes-part-of-its-population.md`, and the gate it names is `scripts/dev/check_doc_links.py`, whose `go_files` reads `_test.go` since commit `1ca0436e8` |
| `plan/spec-doc-claims-are-checked-not-just-resolved.md`, header `Depends` cell | `spec-fixit-dead-design-pointers-in-tests` | Restate | The dependency is satisfied, so the cell states the stem and its closing commit `90082fb08` rather than pointing at a file that closure removed |
| `plan/spec-doc-claims-are-checked-not-just-resolved.md`, Task | `spec-problem-journal` | Restate | "Measured while closing X" is a historical fact about when the measurement was taken. The stem is the fact |
| `plan/spec-fixit-relax-audit-reports-the-wrong-token.md`, Task, "How it was found" | `spec-fixit-vacuous-eor-family-tests` | Restate | Names the spec whose review found the defect. Provenance, not a pointer to knowledge |
| `plan/spec-fixit-rs-community-strip-arity.md`, Implementation Audit, RF-1 row | `spec-fixit-rs-community-strip-arity-deferred-removevalues-quality` | Repoint | A deferral destination. The live record is the shard `plan/deferrals/fixit-rs-community-strip-arity.md`, whose RF-1 row reads `done` against `newRemovalSet` and `TestRemovalSetIndexesOnlyAboveThreshold` |
| This file: Task twice, the dead-stem table, Design Insights | `spec-fixit-dead-design-pointers-in-tests` | Restate, 4 rows | This file already writes dead stems bare and says why. The first Task mention repoints as well, to the journal class file, because it names where the `_test.go` class is recorded |
| `plan/spec-gokrazy-builddir-tmp-deferred-build-flow-unification.md`, "Added 2026-08-03" | `spec-gokrazy-init-bump` | Repoint | The sentence hands the reader the knowledge behind the finding. `plan/deferrals/gokrazy-init-bump.md` carries the 2026-08-03 row that homes this exact work here |
| `plan/spec-perf-next-1-ebgp-wire-lockfree.md`: Task, Review Gate run 1, Deferrals Resolved | `spec-wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal` | Repoint, 3 rows | The dead spec was a DUPLICATE, removed by commit `56ddbcc63`. Its subject survives in `plan/spec-wire-edit-3-deferred-ac9-dead-code.md`, cited beside it at all three sites, so each one keeps the survivor and says the duplicate went |
| `plan/spec-perf-next-1-ebgp-wire-lockfree.md`, Bugs Found/Fixed | `spec-rfc7606-5-1-2-relay-shape` | Restate | The sentence says commit `632dcade1` removed that file. A resolving pointer is impossible by construction |
| `plan/spec-wire-edit-3-deferred-ac9-dead-code.md`, Task | `spec-wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal` | Restate | The subject IS the removed duplicate. The surviving owner is the file holding the sentence |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The closure workflow clears the citers before commit B | let the gate red on the next author | The author who removes the target knows why it went, and a red gate that stays red teaches everyone to bypass it |

## Known Limitations
- A citation that resolves and misdescribes what it cites still passes.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
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
- [ ] Interop tests N-A: no protocol surface

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Journal row written for anything this teaches
- [ ] **Commit A:** code + tests + docs + spec + journal row
- [ ] **Commit B:** `git rm plan/spec-fixit-spec-closure-leaves-dangling-spec-citations.md` only

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `scripts/dev/spec_citation_check_test.py` | Yes | `ls -la` shows it. It already existed, and gained `repo_root` plus `test_real_corpus_has_no_dangling_spec_citation` |
| `plan/journal/closure-deletes-a-cited-document.md` | Yes | `make ze-journal` exits 0 and parses its row |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | zero dangling references | `make ze-spec-citation-check` exits 0: `spec-citation-check OK (258 specs, 46 baselined dangling, 35 line-token WARN)` |
| AC-2 | `make ze-verify-wiring-docs` green | exits 0 over 13 stages, `ze-spec-citation-check PASSED` among them |
| AC-3 | closure names the citers before commit B | `ai/skills/ze-close.md` step 6d, bullet `Clear the citers first (BLOCKING)`, ahead of the Commit A and Commit B bullets |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-spec-citation-check` | no `.ci` applies. The fixture is `test_real_corpus_has_no_dangling_spec_citation` in `scripts/dev/spec_citation_check_test.py` | Yes. It runs the gate over the real tree through the same entry point. It asserts the corpus is non-empty first, so a wrong root fails rather than passes. It was red at 15 rows, then green |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | every one of the 15 rows is classified in "Citation repair decisions": 5 repoints, 10 restatements, 0 baseline rows, 0 sentences dropped |
| A-2 | confirmed | `make ze-verify-wiring-docs` exits 0. No second red was hiding behind the citation stage |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 15, registered workflow surface | `ai/INDEX.md` names closure as the feeder of `make ze-spec-citation-check` | Yes |
| Row 10, test infrastructure | no new check and no new make target, so `docs/contributing/documentation-testing.md` needs no row | Yes |
