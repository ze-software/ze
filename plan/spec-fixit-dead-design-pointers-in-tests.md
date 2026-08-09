# Spec: fixit-dead-design-pointers-in-tests

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling |
| Depends | - |
| Phase | 3/3 |
| Deferral shard | `plan/deferrals/fixit-dead-design-pointers-in-tests.md` |
| Updated | 2026-08-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

133 `// Design:` pointers in `_test.go` files name a `plan/spec-*.md` that no
longer exists. The gate cannot see them: `go_files()` in
`scripts/dev/check_doc_links.py` ends with
`[f for f in out if f.endswith(".go") and not f.endswith("_test.go")]`, and
`check_design_refs()` iterates only that list.

The count is structural, not an accident. Spec closure is two commits, and
commit B is `git rm plan/spec-<stem>.md` (`ai/rules/planning.md`, "Spec
Closure"). Every closed spec therefore kills every pointer to it, and nothing
repoints the tests that cited it. Non-test files carry zero dead spec pointers,
which is the control: the gate works where it looks.

Reproduction:

```
git grep -n '// Design: plan/spec-' -- '*.go' ':!vendor' \
  | while IFS= read -r l; do
      t=$(printf '%s' "$l" | grep -o 'plan/spec-[A-Za-z0-9._-]*\.md')
      [ -n "$t" ] && [ ! -f "$t" ] && printf '%s\n' "$l"
    done | wc -l
```

Surfaced by `plan/spec-problem-journal.md`, which moved 1155 `// Design:`
pointers off `plan/learned/` and onto `docs/architecture/`. A sweep agent found
`internal/component/iface/registry_integration_linux_test.go` pointing at a
deleted `plan/spec-as112-1-iface-address-registry.md`, which is how the class
was discovered.

The two questions this spec must answer, in order:

1. Should a test carry a `// Design:` pointer to a spec at all? A spec is
   deliberately temporary. `docs/architecture/` is the durable home, and
   `plan/spec-problem-journal.md` established that route for the learned corpus.
2. Whatever the answer, `check_design_refs()` must stop being blind to test
   files, or the class regrows silently.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/planning.md` - "Spec Closure" is why the targets die
  → Constraint: commit B removes the spec, and that is correct behavior to keep

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/check_doc_links.py` - `go_files()` excludes `_test.go`; `check_design_refs()` and `path_resolves()` do the checking

**Behavior to preserve:**
- Spec closure keeps removing the spec file. The dead pointer is the defect, not the removal.

**Behavior to change:**
- `check_design_refs()` reads `_test.go` files, and REFUSES a `plan/spec-` target
  in one. A test must cite a durable document.
- Every `plan/spec-` pointer in a test file is repointed at the durable document
  for its subject. Where no such document exists, the document is written.

## Data Flow (MANDATORY)

### Entry Point
- `make ze-doc-test` runs `scripts/dev/check_doc_links.py`, which iterates `go_files()`.

### Transformation Path
1. `go_files()` lists tracked Go, minus `_test.go`.
2. `check_design_refs()` extracts the first token after `// Design:`.
3. `path_resolves()` is a plain existence check, so a dead target WOULD be reported if the file were in the list.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Gate ↔ test sources | `go_files()` filter | Yes: the filter is the whole mechanism |

### Integration Points
- `scripts/dev/check_doc_links_test.py` - covers the gate today

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The refusal is one branch inside `check_design_refs()`, ahead of the existing `path_resolves()` call. Findings still leave through `drop_generated()` |
| No unintended coupling (components stay isolated) | N-A | one script |
| No duplicated functionality (extends existing, does not recreate) | Yes | `go_files()` is the one file list and `DESIGN` is the one parser. No second checker was added. `check_design_refs()` remains the sole caller of `go_files()` |
| Zero-copy preserved where applicable | N-A | no wire path |
| Registration over hardcoding | N-A | no plugin surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Including `_test.go` in the gate reports 133 findings and no more | the reproduction command above | the fix is larger than one filter change | run the gate with the filter removed, before changing anything else | broken: the true population is 145. The reproduction matched `// Design: plan/`, so a slash-free stem was invisible to it. See the Mistake Log |
| A-2 | Every `// Design:` line in a test file sits inside the 4096-byte head window `check_design_refs()` reads | the function reads `fh.read(4096)` and breaks at the first match | widening the file list would still miss pointers, and the gate would report a false zero | replay the gate's own parse over every `_test.go` | confirmed: 144 of 144 in the head window, 0 deeper |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Widening the gate turns 133 silent references into a hard red nobody can land through | `make ze-doc-test` goes red on unrelated commits | retired: all 145 are repointed in the same change that widens the gate, and `make ze-doc-test` exits 0 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. A doc gate and test-file comments |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Every spec closure, which is why the class regrows |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-doc-test` | → | `check_doc_links.py` `check_design_refs()` | `test_design_ref_in_a_test_file_is_checked` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `_test.go` carries a `// Design:` pointer to a file that does not exist | `check_doc_links.py` reports it as a broken Design reference |
| AC-2 | The repo as it stands | the gate reports zero Design findings, because all 145 pointers in test files were repointed first: the 144 `plan/spec-` ones, plus `forward_update_bench_test.go`'s slash-free `rs-fastpath-3` (a closed spec's stem, invisible to a `plan/` grep) |
| AC-3 | A `_test.go` carries a `// Design:` pointer to a `plan/spec-*.md` that DOES exist | the gate refuses it, naming the durable-document rule. Existence is not enough |
| AC-4 | A non-test `.go` carries a `// Design:` pointer to a `plan/spec-*.md` that exists | the gate accepts it. The ban is scoped to test files |
| AC-5 | Every target a repointed test now cites | the document exists and describes that test's subject |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_design_ref_in_a_test_file_is_checked` | `scripts/dev/check_doc_links_test.py` | AC-1 | passing (failed first against the unwidened gate) |
| `test_design_ref_to_a_live_spec_is_refused_in_a_test_file` | `scripts/dev/check_doc_links_test.py` | AC-3 | passing (failed first against the unwidened gate) |
| `test_design_ref_to_a_live_spec_is_allowed_outside_a_test_file` | `scripts/dev/check_doc_links_test.py` | AC-4 | passing (fails against an unscoped refusal) |
| `test_design_ref_outside_a_test_file_still_reports_a_dead_target` | `scripts/dev/check_doc_links_test.py` | the gate keeps its original job | passing (fails against a mutant whose `go_files()` returns only `_test.go`: real gate exits 1, mutant exits 0) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-doc-test` | `Makefile` | an agent cannot land a dead Design pointer in a test | passing: exit 0. Before the repoint the same target reported 145 findings |

## Files to Modify
- `scripts/dev/check_doc_links.py` - `go_files()` keeps `_test.go`; `check_design_refs()` refuses `SPEC_PREFIX` inside one
- `scripts/dev/check_doc_links_test.py` - four cases (three new ACs, plus the regression control)
- 145 `_test.go` files - one `// Design:` line each, repointed at a durable document
- `docs/architecture/ike/ipsec-10-cli-diag.md` - a stale claim the new IPsec document would have contradicted

## Files to Create
Three architecture documents, each written because the subject had no durable home:
- `docs/architecture/pki/tls-listeners.md` - `pki.ServerTLSMaterial` has six non-test callers and was undocumented
- `docs/architecture/ike/ipsec-dataplane-inspection.md` - the install side was documented, reading the kernel back was not
- `docs/architecture/iface/vlan-qos-map.md` - `cos-plugin.md` defers the low-level mechanism to iface, which had no page

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config surface |
| YANG validation constraints | N-A | no config surface |
| YANG custom validators | N-A | no config surface |
| CLI commands/flags | N-A | no CLI surface |
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
| 1 | New user-facing feature? | No | agent tooling only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the pointer convention changes |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/rules/repo-maintenance.md` gate list |
| 16 | Any changed source file referenced by existing doc source anchors? | No | |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase 1: Wiring (MANDATORY FIRST)** -- the gate sees test files and refuses `plan/spec-`
   - Tests: `test_design_ref_in_a_test_file_is_checked`, `test_design_ref_to_a_live_spec_is_refused_in_a_test_file`, `test_design_ref_to_a_live_spec_is_allowed_outside_a_test_file`
   - Files: `scripts/dev/check_doc_links.py`, `scripts/dev/check_doc_links_test.py`
   - Verify: the new tests fail, then pass, and the repo-wide run reports all 144
2. **Phase 2: Destinations** -- every one of the 31 targets gets a durable document
   - Files: `docs/architecture/**` (new documents only where none exists)
   - Verify: each destination exists and covers the subject its tests exercise
3. **Phase 3: Repoint** -- apply the 145 edits
   - Files: the 145 `_test.go` files (144 `plan/spec-` pointers, plus
     `internal/component/bgp/reactor/forward_update_bench_test.go`)
   - Verify: `make ze-doc-test` green, `go build` of every touched package clean

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The gate reports a dead target in a test file, and still reports one in a non-test file |
| Data flow | The fix is in the file list, not in a second copy of the Design parser |
| Rule: `ai/rules/simplicity.md` | One filter, one test. No new gate |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The gate sees test files | `python3 scripts/dev/check_doc_links_test.py` |
| No dead Design pointer remains | `python3 scripts/dev/check_doc_links.py` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | None. The gate reads repo-authored comments |

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

- The gate's blind spot and the closure convention compound: one removes the
  target, the other cannot see the reference. Either alone would be harmless.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A test file may not carry a `// Design:` pointer to `plan/spec-*.md` at all. The gate refuses the prefix, it does not only check existence | Existence check alone, the same rule non-test files carry | Existence alone fixes the 133 and leaves the 11 live pointers legal. Each one dies at its spec's closure commit, so the gate would go red later on an unrelated commit by an unrelated author. Refusing the prefix stops the class at authoring time, which is what question 2 asks for (owner decision, 2026-08-09) |
| Every pointer is repointed at the durable document. Where none exists, the document is written from the deleted spec's content in git history | Dropping the pointer where no document exists | A dropped pointer loses the design record the test was written against, and the missing document is the real gap the dead pointer was pointing at (owner decision, 2026-08-09) |
| The rule lives in `check_design_refs()`, over a widened file list | A second checker for test files | `ai/rules/simplicity.md`: one parser, one list. A second copy of the Design parser is the failure mode the Critical Review Checklist names |

## Known Limitations
- **The rule is asymmetric.** A non-test `.go` may still cite a `plan/spec-*.md`,
  and 21 pointers do. Each dies at its spec's closure commit, and the gate then
  goes red for whoever is committing at the time. The class is caught there
  rather than prevented, which is the weaker half of what this spec does for
  tests. Widening the ban would be a separate change: AC-4 pins the current
  scope, so it is a deliberate boundary rather than an oversight.
- **Five pointers land on `docs/architecture/core-design.md`, which is 87 KB.**
  The subject really is in that file for each of them, and each pointer names
  its section in trailing prose, because `path_resolves()` cannot resolve an
  `#anchor` fragment. A reader still opens a large document and searches.
- `plan/spec-fixit-appliance-evidence-config.md` has the weakest destination.
  Its subject, `bootstrapConfigFromTemplate` in `cmd/ze/ze_core_start.go`, is one
  table cell in `core-design.md`. The topical page `appliance/device-config.md`
  defers the mechanism to that section and never mentions the template.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-2 all demonstrated
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
- [ ] Journal row written for anything this spec teaches
- [ ] **Commit A:** code + tests + docs + spec + journal row
- [ ] **Commit B:** `git rm plan/spec-fixit-dead-design-pointers-in-tests.md` only

---

## Implementation Summary

### What Was Implemented
- `go_files()` in `scripts/dev/check_doc_links.py` keeps `_test.go`. The exclusion was the whole defect, and `check_design_refs()` is its only caller, so no other consumer changed meaning.
- `check_design_refs()` gained one branch ahead of the existence check: inside a `_test.go`, a target under `SPEC_PREFIX` is refused and the message names `ai/rules/planning.md`.
- 145 `// Design:` pointers in test files repointed at durable documents.
- Three architecture documents written where the subject had no durable home.

### Bugs Found/Fixed
- The gate found a pointer the spec's own reproduction command could not see: `internal/component/bgp/reactor/forward_update_bench_test.go` cites the slash-free stem `rs-fastpath-3`. The reproduction matched `// Design: plan/`, so the real count was 145, not 133. Covered by `test_design_ref_in_a_test_file_is_checked`.
- `docs/architecture/ike/ipsec-10-cli-diag.md` claimed byte counters are absent on purpose. `readSADCounters` in `internal/component/ike/cmd/show_ipsec.go` reads them from the kernel SAD. Corrected.
- Review round: nothing pinned the gate's original job. Added `test_design_ref_outside_a_test_file_still_reports_a_dead_target`.

### Documentation Updates
- Created `docs/architecture/pki/tls-listeners.md`, `docs/architecture/ike/ipsec-dataplane-inspection.md`, `docs/architecture/iface/vlan-qos-map.md`.
- Cross-links added to `pki-store.md` and `traffic/cos-plugin.md`. Source anchor added to `ipsec-10-cli-diag.md` naming `sadCounters, readSADCounters`.
- `make ze-doc-test` exit 0.

### Deviations from Plan
- The spec said 133 pointers "repointed or dropped". The owner ruled out dropping, so all 145 are repointed and three documents were written.
- The spec expected no new files. Three were needed.
- Phases went from two to three: the destination decision was separated from the mechanical repoint.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The reproduction command in the Task section was treated as the full population, giving 133 | 145. A `// Design:` target needs no `plan/` prefix and no slash to be dead | The widened gate reported one more finding than the audit predicted | Counted with the gate's own parser, not a grep. A-2 was added to record that the parse, not the grep, is the measurement |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Answer whether a test may cite a spec | Done | Key Design Decisions | Owner decision: it may not |
| `check_design_refs()` stops being blind to test files | Done | `scripts/dev/check_doc_links.py`, `go_files()` | |
| The class cannot regrow silently | Done | `check_design_refs()`, `SPEC_PREFIX` branch | Refused at authoring time, not caught later |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test_design_ref_in_a_test_file_is_checked` | |
| AC-2 | Done | `check_doc_links.py --design-only` exit 0 | 145 findings before the repoint |
| AC-3 | Done | `test_design_ref_to_a_live_spec_is_refused_in_a_test_file` | |
| AC-4 | Done | `test_design_ref_to_a_live_spec_is_allowed_outside_a_test_file` | 21 live non-test pointers stay green |
| AC-5 | Done | Existence machine-checked by the gate. Subject match is judgment, spot-checked on `wire/isis.md` and the `core-design.md` `rs-fastpath-3` section | Weakest row recorded in Known Limitations |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `test_design_ref_in_a_test_file_is_checked` | Done | `scripts/dev/check_doc_links_test.py` | Fails against the unwidened gate |
| `test_design_ref_to_a_live_spec_is_refused_in_a_test_file` | Done | same | Fails against the unwidened gate |
| `test_design_ref_to_a_live_spec_is_allowed_outside_a_test_file` | Done | same | Fails against an unscoped refusal |
| `test_design_ref_outside_a_test_file_still_reports_a_dead_target` | Done | same | Added at review. Fails against a `_test.go`-only mutant |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/check_doc_links.py` | Done | |
| `scripts/dev/check_doc_links_test.py` | Done | Four cases, not one |
| 145 `_test.go` | Done | Plan said 133 |
| 3 architecture documents | Changed | Plan expected none |

### Audit Summary
- **Total items:** 15
- **Done:** 14
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the three new documents, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The 133 dead pointers are gone | functional | `python3 scripts/dev/check_doc_links.py --design-only` exit 0. The same command reported 145 before the repoint |
| The gate is no longer blind to test files | functional | `make ze-doc-test` exit 0, and it now fails on a dead pointer in a `_test.go`: `test_design_ref_in_a_test_file_is_checked` |
| The class cannot regrow silently | functional | `test_design_ref_to_a_live_spec_is_refused_in_a_test_file`. A pointer at a spec that still exists is refused, so closure can no longer manufacture a dead one |
| The gate keeps its original job | functional | `test_design_ref_outside_a_test_file_still_reports_a_dead_target`, proven to fail against a mutant whose `go_files()` returns only `_test.go` (real gate exit 1, mutant exit 0) |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| 133 dead `// Design:` pointers behind the `go_files()` exclusion | done | This spec. Gate exit 0 |
| `ze-spec-citation-check` red with 12 dangling citations at HEAD | deferred | `plan/spec-fixit-spec-closure-leaves-dangling-spec-citations.md`. Pre-existing, in `plan/` prose, none from this work |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | none: owner override |
| `review_gate.py check` | not run |
| Rounds | 1 inline round by the supervising thread, which is NOT an independent review |
| Reviewer lenses used | the spec's Critical Review Checklist (completeness, correctness, data flow, simplicity) |

Thomas instructed closure without the independent review on 2026-08-09: several
sessions are editing this checkout, a clean pass is unreachable, and a follow-up
agent takes any problems found later. The one round below was run by the thread
that supervised the implementation, so it does not satisfy the independence
requirement in `ai/rules/planning.md`.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | No test pinned the gate's original job. A later change scoping the check to test files only would have passed every test in the file | `scripts/dev/check_doc_links_test.py`, `DesignRefTest` | `test_design_ref_outside_a_test_file_still_reports_a_dead_target`, proven against a mutant |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `docs/architecture/pki/tls-listeners.md` | Yes | 4000 bytes |
| `docs/architecture/ike/ipsec-dataplane-inspection.md` | Yes | 4557 bytes |
| `docs/architecture/iface/vlan-qos-map.md` | Yes | 3136 bytes |
| All 32 destinations in the mapping | Yes | Loop over `tmp/design-pointer-destinations.tsv` reported no missing target |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A dead target in a test file is reported | `check_doc_links_test.py`, 29 tests OK |
| AC-2 | The repo reports zero Design findings | `check_doc_links.py --design-only` exit 0, output `all corpus path references resolve` |
| AC-3 | A live spec target in a test file is refused | Same suite. The finding carries `durable` and `ai/rules/planning.md`, and NOT `broken Design reference` |
| AC-4 | A live spec target outside a test file is accepted | Same suite, and the repo-wide run reports zero findings from the 21 live non-test pointers |
| AC-5 | Every destination exists | Gate exit 0 is the machine check |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-doc-test` | none: this gate is Python, so the wiring test is `check_doc_links_test.py` driven through the real entry point by subprocess | Yes: `make ze-doc-test` exit 0, and it reported 145 findings before the repoint |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken, then corrected | Predicted 133 and no more. The true population is 145: the reproduction grep could not see a slash-free stem |
| A-2 | confirmed | 144 of 144 `// Design:` lines sit inside the 4096-byte head window, 0 deeper |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Item 10, test infrastructure changed | The pointer convention changed, and the rule is enforced by the gate rather than by prose | Yes |
| Item 15, verification surface changed | `check_design_refs()` refuses a new class. Journal row filed at `plan/journal/gate-excludes-part-of-its-population.md` | Yes |
| `ipsec-10-cli-diag.md` byte-counter claim | `readSADCounters` in `internal/component/ike/cmd/show_ipsec.go` reads `BytesCurrent` from the kernel SAD | Yes: read the producer |
| Items 1 to 9 and 11 to 14, 16, 17 | No user-facing feature, config, CLI, API, plugin, wire format, RFC behavior, or counter changed. The diff is 145 comment lines, one Python gate, and architecture prose | Yes |

## Core Insight

Two correct conventions manufactured the defect between them. Spec closure
deletes the spec, which is right. The gate skipped test files, which looked like
a scoping choice. Neither is wrong alone. Together one of them creates dead
references at a steady rate and the other guarantees nobody sees them. The fix
that only widens the gate would have caught the class one closure later, on an
unrelated author's commit. Refusing the pointer at authoring time is what stops
the production, rather than improving the detection.
