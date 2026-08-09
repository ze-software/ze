# Spec: fixit-spec-closure-leaves-dangling-spec-citations

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-dead-design-pointers-in-tests.md` |
| Updated | 2026-08-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`make ze-spec-citation-check` is RED at HEAD. 12 citations of a `plan/spec-*.md`
name a spec that no longer exists and is not in the grandfathering baseline
`plan/.citation-baseline`. The check is a stage of `make ze-verify-wiring-docs`,
so that target cannot go green until the 12 are cleared.

The cause is the closure convention, the same one
`plan/spec-fixit-dead-design-pointers-in-tests.md` records for `_test.go`.
Spec closure is two commits, and commit B is `git rm plan/spec-<stem>.md`
(`ai/rules/planning.md`, "Spec Closure"). `/ze-close` does not add the removed
stem to `plan/.citation-baseline` and does not repoint the specs that cite it,
so every closure that lands while a sibling cites it adds a red row.

Found while implementing phase 3 of
`plan/spec-fixit-dead-design-pointers-in-tests.md`. Phase 3 repointed 145
`// Design:` pointers and took the Design gate to zero. It touched no `plan/`
file, so it neither caused nor cleared these 12 rows.

Reproduction, run 2026-08-09:

```
python3 scripts/dev/spec-citation-check.py          # exit 1, 12 dangling reference(s)
make ze-verify-wiring-docs                          # exit 2, same stage
```

Every one of the 12 is dangling at HEAD as well as in the working tree: for each
row, `git show HEAD:<citing-file>` holds the citation and
`git show HEAD:<cited-spec>` fails.

The producing function is `find_dangling` in
`scripts/dev/spec-citation-check.py`. It scans `spec_files()` and
`learned_files()` for `SPEC_REF_RE`, drops anything in `load_baseline()`, and
reports the rest. The check is correct; the closure workflow that feeds it is
the defect.

Seven distinct dead targets carry the 12 rows. Their stems are written here
without the `plan/` prefix and the `.md` suffix on purpose: spelled in full they
are themselves dangling citations, and this file would add seven rows to the
count it exists to clear.

| Dead stem | Rows |
|-----------|------|
| `spec-problem-journal` | 3 |
| `spec-wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal` | 4 |
| `spec-as112-1-iface-address-registry` | 1 |
| `spec-fixit-vacuous-eor-family-tests` | 1 |
| `spec-fixit-rs-community-strip-arity-deferred-removevalues-quality` | 1 |
| `spec-gokrazy-init-bump` | 1 |
| `spec-rfc7606-5-1-2-relay-shape` | 1 |

Three of the 12 sit in `plan/spec-fixit-dead-design-pointers-in-tests.md`
itself. Its commit B removes that file, so those three clear at its closure and
need no separate repair.

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
- The baseline already carries 56 stems, so grandfathering is the sanctioned
  route. Nothing adds to it automatically, which is why it drifts.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/spec-citation-check.py` - `find_dangling` reports a cited `plan/spec-*.md` that is absent and unbaselined; `load_baseline` reads `plan/.citation-baseline`; `write_baseline` regenerates it wholesale
- [ ] `mk/inventory.mk` - the `ze-spec-citation-check` target runs the script with no arguments
- [ ] `.claude/skills/ze-close/SKILL.md` - the closure steps that write commit B

**Behavior to preserve:**
- Commit B keeps removing the spec file.
- `--write-baseline` stays a deliberate operator action, never a step a failing
  run takes for itself.

**Behavior to change:**
- Closing a spec that a sibling cites stops leaving a red row behind.
- The 12 rows measured on 2026-08-09 are cleared.

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
| A-1 | Each of the 12 rows has a repair: a repoint, an inline restatement, or a baseline row | 3 of them vanish at this spec's own closure | the gate cannot be cleared without dropping content | classify all 12 before editing | unvalidated |
| A-2 | Clearing the 12 takes `make ze-verify-wiring-docs` green | the log names one failing stage | a second red is hiding behind the first | run the target after the repairs | unvalidated |

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
- the citing specs, for each citation that repoints or restates
- `plan/.citation-baseline` - for each citation that is a historical record
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

1. **Phase: Classify the 12** -- each row gets a repoint, a restatement, or a baseline entry
   - Verify: A-1 answered per row
2. **Phase: Repair** -- apply the 12 decisions
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
  `plan/spec-fixit-dead-design-pointers-in-tests.md` for `_test.go` pointers,
  `plan/spec-dead-learned-citations-outside-the-walked-corpus.md` for the
  retired learned corpus, and this one for spec-to-spec citations. The gate
  exists here and is red, so the missing part is the closure step that feeds it.

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
