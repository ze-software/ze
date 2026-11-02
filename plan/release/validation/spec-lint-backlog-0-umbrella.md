# Spec: lint-backlog-0-umbrella

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | The release check runs the full `./le go lint run`, and a red there holds the release (`ai/META-DECISIONS.md`, "A red prevents a release, never a commit"); it reports 220 findings today |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

### Owner's one-off WIP exception

On 2026-11-02, Thomas authorized this set: "Do not let spec open limit stop you from starting."
This exception covers the umbrella and its nine children. For each claim, use
`ZE_SPEC_WIP_CAP=<current in-progress count plus one> ./le spec claim <spec-file>`.
Read the count with `./le spec wip report`. Set the override only on that command.
The permanent cap stays unchanged.

### Lint backlog

CI's full `./le go lint run` can finish since the Staticcheck and cache fixes,
and on 2026-11-02 it reported 228 unique findings across the tree: goconst 50,
exhaustive 36, errcheck 30, gosec 24, misspell 23, modernize 19, gocritic 17,
goimports 10, unused 8, unparam 6, nilerr 2, staticcheck 1, nilnil 1, godot 1.
Eight of the listed locations, six findings, are already fixed by 752d8f19ef
(`internal/le/go/staticcheck/judge.go`, `internal/le/hookruntime/timestamp.go`,
`internal/le/hookruntime/timestamp_test.go`, and the goconst in
`internal/le/test/qemu`, which the list reports at its three occurrences), so
220 listed locations remain.

Lint is a red that blocks the release, never a commit: no commit waits for this
work, and the release check owes it green. Acceptance for the whole set: the
full `./le go lint run` reports zero findings. That full run is a whole-tree
check, so no agent runs it (`ai/META-DECISIONS.md`, "Test selectively"): each
child proves its own packages with a scoped run, and the full run is the CI
`verify` workflow on a pushed commit, cited as the evidence.

One spec over every finding would need eleven subsystem doors, so the work is
split by door (`ai/META-DECISIONS.md`, "a spec whose edits need many doors is
split into narrower child specs"). Each child carries its own finding list.

| Child | Doors | Findings | Packages |
|-------|-------|----------|----------|
| `plan/release/validation/spec-lint-backlog-1-bgp-engine.md` | bgp-engine | 53 | `internal/component/bgp/{reactor,policychain,config,cli,redistribute}` |
| `plan/release/validation/spec-lint-backlog-2-bgp-plugins.md` | bgp-plugins | 22 | `internal/component/bgp/plugins/{filter_*,gr,role}` |
| `plan/release/validation/spec-lint-backlog-3-test-tooling.md` | le-tooling (system for `internal/chaos`, `tools/`) | 58 | `internal/test/...`, `internal/le/...`, `internal/chaos/...`, `tools/freertr-entrypoint` |
| `plan/release/validation/spec-lint-backlog-4-plugin-infra-and-core.md` | plugin-infra, core | 14 | `internal/component/plugin/...`, `pkg/plugin/sdk`, `internal/core/statestore` |
| `plan/release/validation/spec-lint-backlog-5-hub-and-policy.md` | hub-engine (system for `internal/component/policy`) | 18 | `cmd/ze`, `cmd/ze/hub`, `internal/component/policy` |
| `plan/release/validation/spec-lint-backlog-6-access-and-appliance.md` | access-security, appliance | 25 | `internal/component/ike/...`, `internal/component/l2tp`, `internal/appliance`, `internal/component/config/system` |
| `plan/release/validation/spec-lint-backlog-7-system-and-routing-plugins.md` | system-plugins, routing-protocols | 19 | `internal/plugins/{debug,help,redistribute,skills}`, `internal/plugins/{ospf,isis}/...` |
| `spec-lint-backlog-8-api-and-commands` (closed; implementation `3895eccf22`, Linux lint clean) | api-surfaces, commands-cli | 6 | `internal/component/{web,mcp,command}` |
| `plan/release/validation/spec-lint-backlog-9-config-and-forwarding.md` | config-yang, forwarding | 5 | `internal/component/config/{storage,yang/cli}`, `internal/plugins/iface/netlink` |

Line numbers are those of the CI run; a child regenerates its list with its
scoped run before it starts, because other sessions edit these files.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/ze-go-style.md` - "The rules a tool enforces", the `exhaustive` C/O/P classification, "Every error is handled"
  → Constraint: a `//nolint` names its linter and its reason (`nolintlint`); a blanket `nolint:exhaustive`, a file exemption or a cast to dodge coverage is refused
  → Constraint: a `default` never substitutes for a missing named enum member
- [ ] `ai/META-DECISIONS.md` - "Ze is pre-release", "Test changes", "Code and dependencies"
  → Constraint: an RFC sentence quoted in a comment stays byte-exact and is never edited to satisfy a linter
  → Constraint: a full lint is a whole-tree check that no agent starts; scoped runs are the evidence
  → Decision: lint holds the release, never a commit

**Key insights:**
- Most findings are mechanical (goconst, misspell, modernize, goimports, unused, godot); errcheck, nilerr, nilnil, gosec and exhaustive can each hide a real defect, so each is read before it is silenced.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `.golangci.yml` - the enabled linters and their settings (`exhaustive` `default-signifies-exhaustive: false`, errcheck `check-blank: false`, nolintlint)
- [ ] `docs/contributing/ze-go-style.md` - the rule table each linter enforces

**Behavior to preserve:**
- Every behavior of the shipped binary, unless a finding is a real defect, which is then fixed with a red test first.

**Behavior to change:**
- The 220 findings, each fixed at its source or annotated with the reason the linter is wrong there.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint configuration | `.golangci.yml` | Declares the linters and their settings | Does not change: the findings are fixed, not the configuration loosened |
| Lint runner | `./le go lint run` (`scope <packages>`, `changed`) | Lints the tree, a package scope or the change set over every build flavor | Applies: each child's evidence is a scoped run; the full run is CI's |
| Style rule table | `docs/contributing/ze-go-style.md`, "The rules a tool enforces" | Names the rule each linter enforces | Applies: the fix follows the rule, never a bypass |
| Journal class for lint contracts | `plan/journal/lint-contract-not-applied.md` | Open rows owned by `plan/post-release/engineering/spec-gates-report-cause-and-progress.md` and `plan/post-release/engineering/spec-tooling-one-off-defects.md` | Does not change ownership: a child whose fix clears a finding such a row records marks that row `fixed:` in the same commit |
| Lint ratchet for discards and the clock | `plan/post-release/engineering/spec-lint-ratchet-clock-and-discarded-errors.md` | Plans new checks for blank discards and direct `time` calls | Does not apply: it adds checks; this set clears the findings of the checks that exist |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run` over the tree (CI) or a scope (a child).

### Transformation Path
1. golangci-lint runs each enabled linter over each build flavor the matrix declares.
2. Each child clears its packages' findings and proves them with a scoped run.
3. CI's full run on a pushed commit reports zero.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Source edits inside each package | No |

### Integration Points
- The children listed in the Task table.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The CI list is the whole population | The full run finished on CI on 2026-11-02 | A finding outside the children survives | CI's full run after the last child | unvalidated |
| A-2 | 752d8f19ef cleared the eight excluded locations | Its body: a scoped lint over the three packages reports 0 issues | A child misses them | A scoped run over `internal/le/go/staticcheck`, `internal/le/hookruntime`, `internal/le/test/qemu` in child 3 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | New findings land while the children run | CI's full run reports findings no child lists | Each is fixed by the commit's author under the lint contract, or added to the child of its door |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CI `verify` workflow on a pushed commit | → | `./le go lint run` over the tree | the workflow's lint step, zero findings |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Every child closed | Each child's scoped lint reports zero |
| AC-2 | CI's full `./le go lint run` after the last child | Zero findings, cited by its run |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Each child's scoped lint and package tests | the children | AC-1 | |

## Files to Modify
- None of its own: each child names its files.

## Implementation Steps

1. Run the children; they are independent and can run in any order.
2. When the last closes, cite CI's full run (AC-2) and close this umbrella.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Population | Each child's refreshed findings are resolved, including child 3's check of the excluded locations (A-2). |
| Scoped evidence | Each closed child records zero scoped lint findings and its required package tests (AC-1). |
| Full-run evidence | Cite the CI lint run after the last child for AC-2, including any new findings under R-1. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Nine completed children | Cite each child's zero scoped lint result and required package-test evidence (AC-1). |
| Full backlog cleared | Cite the CI full lint run after the last child, including the excluded locations and new findings (AC-2, A-1–2). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Hidden defects | Child reviews distinguish real errcheck, nilerr, nilnil and gosec defects from justified annotations. |
| Unsafe suppression | No child weakens lint configuration or changes unknown-value handling merely to clear exhaustive findings. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Each child's scoped `./le go lint run scope "<packages>"` reports zero and its package tests pass. Cite CI's full run for AC-2.
