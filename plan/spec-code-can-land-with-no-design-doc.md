# Spec: code-can-land-with-no-design-doc

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

A new package, component, plugin, or subsystem can land in this repository with
zero `docs/architecture/` coverage while every gate reports green. No rule
requires the page, and no check looks for its absence.

Found by the documentation-currency audit commissioned by
spec-doc-claims-are-checked-not-just-resolved (closed 2026-08-10), row 2 of its
deferral shard, on 2026-08-09.

Reproduction, run on 2026-08-09 at commit 1359f3324:

```
make ze-doc-drift-check                             # "No documentation drift detected"
python3 scripts/dev/code_to_docs.py --check   # "all references valid"
grep -rn 'internal/plugins/explain' docs/     # no output
```

85 non-yang package directories under `internal/` carry no `<!-- source: -->`
anchor. Four are whole plugin directories: `connect`, `diag`, `explain`,
`skills`. `explain` and `skills` are named nowhere under `docs/`.

The direction matters. Every doc gate in the tree answers "does this doc point
at code that still exists". This spec is about the opposite direction: code that
never gets a doc at all. One check covers that direction today, and its reach is
one registry against one table.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/go-standards.md` - "Design Document References", the "When to Add" table
  → Constraint: the rule accepts `// Design: (none -- predates documentation)`, and 79 tracked files use it
- [ ] `ai/rules/repo-maintenance.md` - the gate list a new check registers in
  → Constraint: a gate nobody registers is a gate nobody discovers

**Key insights:**
- The one class-b check that exists reads `internal/component/plugin/registry.All`.
  A plugin registering a CLI root through `internal/component/command/registry.RegisterRoot`
  is invisible to it.
- A table ROW in `docs/DESIGN.md` is what that check demands. An architecture
  page is not what it demands.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/docvalid/doc_drift.go` - `checkDesignMD` compares `registryPluginNames()` against the Shipped Plugins table; `registryPluginNames` reads `registry.All()`; a missing `docs/DESIGN.md` returns nil, so the check fails open
- [ ] `scripts/dev/code_to_docs.py` - `main()` check mode reports an anchor whose path is gone, never a package with no anchor
- [ ] `.claude/hooks/pretool-writeedit.py` - `c_require_design_ref` returns None once the literal `// Design:` appears, whatever it points at
- [ ] `scripts/dev/check_doc_links.py` - `check_design_refs` resolves the target path and never opens the target

**Behavior to preserve:**
- Every existing check keeps its coverage. This adds a direction, it relaxes nothing.
- `extractFloorCount` in `doc_drift.go` stays: a floor claim needs no edit when a
  scenario is added, and an exact claim is still checked exactly.
- The `(none -- predates documentation)` escape stays readable for the 79 files
  that carry it. Whether it stays ACCEPTED for a NEW file is this spec's question.

**Behavior to change:**
- A new package directory must have a documentation home, or a recorded reason
  it has none.
- The class-b check must see a plugin whatever registry it registers in.

## Data Flow (MANDATORY)

### Entry Point
- `make ze-doc-verify` runs `scripts/docvalid/doc_drift.go`, and `ze-doc-verify` is a
  stage of `make ze-precommit-verify` (`stagesForMode` in `scripts/status/verify_run.go`).

### Transformation Path
1. A check enumerates package directories from `git ls-files`.
2. It resolves each against the anchor index `code_to_docs.py` already builds.
3. A directory with no documentation home is a finding, unless it carries a recorded reason.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Gate ↔ package inventory | `git ls-files`, never a hardcoded list | No |
| Gate ↔ both plugin registries | `registry.All` and `command/registry.RegisterRoot` | No |

### Integration Points
- `scripts/docvalid/doc_drift.go` - widen `checkDesignMD`, or add a sibling check
- `ai/rules/go-standards.md` via its point files - the rule that owes the page

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable | N-A | no wire path |
| Registration over hardcoding | No | the inventory must be derived, never a list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 85 undocumented directories can each be given a page or a recorded reason | the count is the whole population, measured 2026-08-09 | the gate cannot be armed until a large writing job finishes | classify all 85 before arming | unvalidated |
| A-2 | A derived package inventory is stable enough to gate on | `code_to_docs.py` derives its index today and is gated | the gate churns on every new directory | run the derivation across a package addition and diff | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The gate demands a page for an internal leaf package nobody navigates to | findings on `internal/core/*` helpers | gate on the unit a reader navigates by (component, plugin), not on every directory |
| R-2 | The recorded reason becomes the default answer, as `(none` did | the reason count grows faster than the page count | count both and publish the ratio, as `rfc/extraction` does for exclusions |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A doc gate reds on a package that needs no page. Nothing user-visible |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Every session that adds a package |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-doc-verify` | → | the package-without-a-doc check in `doc_drift.go` | `TestPackageWithNoDocumentationHomeIsReported` |
| `make ze-doc-verify` | → | plugin discovery across both registries | `TestPluginRegisteringACLIRootIsSeen` |

This is agent tooling with no user-facing surface, so the existing test suite
carries the proof: the Go test beside the check, plus the gate over the real tree.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A package directory has no documentation home and no recorded reason | the gate names the directory and fails |
| AC-2 | A plugin registers through `command/registry.RegisterRoot` | the class-b check sees it |
| AC-3 | A package carries a recorded reason for having no page | no finding, and the reason is counted |
| AC-4 | The 85 directories measured on 2026-08-09 | each classified, and the whole tree green under the armed gate |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPackageWithNoDocumentationHomeIsReported` | `scripts/docvalid/doc_drift_test.go` | AC-1 | |
| `TestPluginRegisteringACLIRootIsSeen` | `scripts/docvalid/doc_drift_test.go` | AC-2 | |
| `TestRecordedReasonSuppressesTheFinding` | `scripts/docvalid/doc_drift_test.go` | AC-3 | |

### Functional Tests

No `.ci` applies: the subject is a documentation gate with no user-facing
surface and no daemon code. It drives no `ze` process, so a `.ci` could assert
nothing about it. The end-to-end proof is the gate running inside its make
target over the real tree.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-doc-verify` | `mk/inventory.mk` | an agent cannot land a package with no documentation home | |

## Files to Modify
- `scripts/docvalid/doc_drift.go` - the check
- `scripts/docvalid/doc_drift_test.go` - its tests
- `ai/rules/go-standards.md` via `ai/rules/points/go-standards/` - the obligation
- `ai/rules/repo-maintenance.md` via its point files - register the gate

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
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md` and the `ai/rules/repo-maintenance.md` gate list |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on `doc_drift.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Classify the 85** -- every undocumented directory gets a page, a reason, or an exclusion with a stated property
   - Verify: A-1 answered with numbers
2. **Phase: Wiring** -- the check exists, reachable from `make ze-doc-verify`, reporting nothing
   - Tests: `TestRecordedReasonSuppressesTheFinding`
3. **Phase: Both registries** -- the class-b check sees a CLI-root plugin
   - Tests: `TestPluginRegisteringACLIRootIsSeen`
4. **Phase: Arm** -- the finding fails the gate
   - Tests: `TestPackageWithNoDocumentationHomeIsReported`
5. **Phase: The rule** -- the obligation in `ai/rules/points/go-standards/`, then `make ze-rules-condensed-update`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | The inventory is derived from `git ls-files`, never a hardcoded list |
| Naming | The finding names the directory and what would satisfy it |
| Data flow | One inventory, shared with the anchor index, not a second walk |
| Rule: `ai/rules/evidence.md` | The gate fails closed: a missing input is a finding, never a nil return |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The check exists and reports a package with no doc | `go test ./scripts/docvalid/` |
| The tree is clean under it | `make ze-doc-verify` |
| The escape is counted, not silent | the published reason count |

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

- Every doc gate in the tree runs in one direction: from the doc to the code.
  Nothing runs from the code to the doc. That is why 85 directories are invisible
  while 1611 anchors are checked.
- The `(none -- predates documentation)` escape was written for files that
  predate the convention. 79 files carry it, and nothing distinguishes a file
  that predates the convention from a file written last week.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Gate on the navigation unit, not every directory | every package directory | A reader navigates by component and plugin. A finding on every leaf helper is the flood that gets a gate switched off |

## Known Limitations
- A page that exists and says nothing useful still passes. This closes the
  absence, not the quality.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional coverage: the gate runs inside its make target
- [ ] Interop tests N-A: no protocol surface

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Journal row written for anything this teaches
- [ ] **Commit A:** code + tests + docs + spec + journal row
- [ ] **Commit B:** `git rm plan/spec-code-can-land-with-no-design-doc.md` only
