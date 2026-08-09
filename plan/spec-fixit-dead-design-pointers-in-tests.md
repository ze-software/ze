# Spec: fixit-dead-design-pointers-in-tests

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
- To be decided by this spec: whether test `// Design:` pointers are repointed at `docs/architecture/`, dropped, or checked and required to resolve.

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
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | N-A | one script |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable | N-A | no wire path |
| Registration over hardcoding | N-A | no plugin surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Including `_test.go` in the gate reports 133 findings and no more | the reproduction command above | the fix is larger than one filter change | run the gate with the filter removed, before changing anything else | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Widening the gate turns 133 silent references into a hard red nobody can land through | `make ze-doc-test` goes red on unrelated commits | repoint or drop all 133 in the same change that widens the gate |

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
| AC-2 | The repo as it stands | the gate reports zero broken Design references, because all 133 were repointed or dropped first |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_design_ref_in_a_test_file_is_checked` | `scripts/dev/check_doc_links_test.py` | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-doc-test` | `Makefile` | an agent cannot land a dead Design pointer in a test | |

## Files to Modify
- `scripts/dev/check_doc_links.py` - `go_files()`, or a separate list for the Design check
- `scripts/dev/check_doc_links_test.py` - the new case
- 133 `_test.go` files - repointed or the pointer dropped, per this spec's answer to question 1

## Files to Create
- none expected

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

1. **Phase: Wiring (MANDATORY FIRST)** -- the gate can see test files
   - Tests: `test_design_ref_in_a_test_file_is_checked`
   - Files: `scripts/dev/check_doc_links.py`, `check_doc_links_test.py`
   - Verify: the new test fails, then passes, and the repo-wide run reports the full 133
2. **Phase: Resolve the 133** -- repoint or drop, per the answer to question 1
   - Files: the `_test.go` files
   - Verify: `make ze-doc-test` green

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
| (to be filled at design time) | | |

## Known Limitations
- None recorded yet. This is a skeleton.

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
