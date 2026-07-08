# Spec: improve-6 -- YANG Coverage Report

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-improve-0-umbrella.md` -- set context
4. `internal/component/config/yang/loader.go` -- module loading
5. `scripts/checks/command_ownership.go` -- precedent for ownership checks

## Task

Ze's YANG surface is registry-driven: plugins and domains register their modules, a
generator produces embed/register glue, and the loader resolves whatever registered.
Nothing reports what that adds up to. There is no single view answering: how many nodes
does each module define, which config roots own them, which leaves lack native
constraints (`range`/`length`/`pattern`/`enum`), which registered nodes no code
consumes, and which modules a build tag compiles out. A cheap per-module node-count
report keeps schema honesty visible as the surface grows.

Ze implements its own modules, not IETF ones, so "coverage" here means
schema-vs-implementation consistency, not standards coverage: a `ze yang coverage`
report (and a scripts/checks-style verifier where rules are mechanical) producing
per-module node counts, per-leaf constraint status, config-root ownership mapping, and
orphan detection. Wire the mechanical subset into the existing check infrastructure;
the report itself is a developer/agent tool.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - YANG handling design
  → Constraint: (fill during design)
- [ ] `ai/patterns/config-option.md` - constraint expectations per leaf
  → Constraint: every leaf must carry maximum native validation; the report measures exactly this
- [ ] `ai/rules/derive-not-hardcode.md` - report derives from loaded modules, no second list
  → Constraint: node inventory comes from the goyang-resolved tree, never a parallel list
- [ ] `ai/rules/feature-gate-registration.md` - build-tag-gated module groups the report must label
  → Constraint: (fill during design)

### RFC Summaries (MUST for protocol work)
- Not protocol work; RFC 7950 (YANG) semantics come via goyang.

**Key insights:**
- `scripts/checks/` already hosts mechanical schema/ownership checks
  (`command_ownership.go`); this extends that family, not a new infrastructure.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/config/yang/loader.go` - `DefaultLoader` loads embedded bootstrap modules then registered modules, best-effort resolve (:20-28); `LoadEmbedded` covers ze-extensions/ze-types only (:48-52)
- [ ] `internal/component/plugin/registry/registry.go` - Registration carries per-plugin `YANG` schema content (:64) and `ConfigRoots` (:50)
- [ ] `scripts/codegen/yang_glue.go` - generates embed/register glue for YANG files (verify exact behavior during design)
- [ ] `scripts/checks/command_ownership.go` - existing ownership-check precedent (read during design)
- [ ] `internal/component/gnmi/capabilities.go` - `loadModels` walks `loader.ModuleNames()` (:39-52): existing module enumeration the report can share

**Behavior to preserve:** (unless user explicitly said to change)
- Loader semantics (best-effort resolve, bootstrap vs registered split) unchanged; the
  report is read-only over the resolved module set.
- Existing checks in `scripts/checks/` and their exit-code contract unchanged.

**Behavior to change:** (only if user explicitly requested)
- None; purely additive tooling. (If the constraint check lands in verify as blocking,
  pre-existing violations are burned down or explicitly waived during design.)

## Data Flow (MANDATORY)

### Entry Point
- `ze yang coverage` CLI command (developer/agent tool) and/or `make ze-yang-coverage`;
  mechanical subset runs as a check inside the existing verify stage family.

### Transformation Path
1. Load the full resolved module set via the existing loader (`DefaultLoader`).
2. Walk each module's schema tree via goyang: count containers/lists/leaves; record each leaf's type and native constraints.
3. Join with registry data: which plugin registered the module, which ConfigRoots claim its subtrees, which build tags gate it.
4. Emit per-module report rows (nodes, constrained %, owner, gating) and violation lists (unconstrained bare-string leaves, unowned subtrees, registered-but-unresolvable modules).
5. Mechanical violations exit non-zero in check mode; report mode prints tables/JSON.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Loader ↔ report | read-only walk of resolved goyang modules | [ ] |
| Registry ↔ report | module-to-plugin/ConfigRoots join | [ ] |
| Report ↔ verify | check-mode exit code, same contract as scripts/checks | [ ] |

### Integration Points
- `DefaultLoader` (`loader.go:20`) - module source.
- `registry` module/ConfigRoots metadata - ownership join.
- `scripts/checks/` + `mk/` targets - check-mode wiring.

### Architectural Verification
- [ ] No bypassed layers (report reads via loader, not raw .yang file parsing of its own)
- [ ] No unintended coupling (tool depends on loader + registry read APIs only)
- [ ] No duplicated functionality (shares module enumeration with capabilities' loadModels approach)
- [ ] Registration over hardcoding -- module inventory derives from registration; no hardcoded module list (`ai/rules/plugin-self-containment.md`, `ai/rules/derive-not-hardcode.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | goyang exposes enough resolved-tree detail (types, ranges, patterns) to grade constraints | goyang powers existing validation | Constraint grading incomplete; report ships counts only in v1 | Prototype walk over 2 modules during design | unvalidated |
| A-2 | Module-to-plugin ownership is derivable from Registration.YANG + ConfigRoots without new metadata | registry fields exist (:50, :64) | Need a module-name annotation in Registration | Cross-check 5 plugins' modules during design | unvalidated |
| A-3 | Existing schema violations (unconstrained leaves) are few enough to burn down before check mode blocks | config-option pattern has been enforced for a while | Check mode starts advisory with a waiver list | First full report run during implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Report becomes a stale second source of truth if it caches anything | drift between report and loader behavior | always compute live from the loader; no stored inventories |
| R-2 | "Coverage" naming misleads (suggests IETF-standards coverage) | user/docs confusion | name and docs say schema-consistency report; comparison docs handle standards claims separately |
| R-3 | Check mode blocks unrelated PRs on legacy violations (A-3) | verify failures on untouched modules | advisory first; blocking only for CHANGED modules (matches ze-lint-changed philosophy) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| ze yang coverage | → | loader walk + registry join + report | TestYANGCoverageReport |
| check mode on a module with an unconstrained leaf (fixture) | → | violation detection, non-zero exit | TestYANGCoverageCheckFailsOnBareLeaf |
| make ze-yang-coverage | → | make target runs the tool | test/plugin/yang-coverage.ci |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze yang coverage` | Per-module table: node counts, constrained-leaf %, owning plugin, build-tag gating |
| AC-2 | Module with a bare `type string` leaf (test fixture) | Listed as a violation with module, path, and what to add |
| AC-3 | Registered module that fails to resolve | Reported explicitly (today it is silently skipped by best-effort resolve) |
| AC-4 | Subtree with no owning ConfigRoot | Flagged as unowned |
| AC-5 | Check mode, violations present | Non-zero exit with the scripts/checks output contract |
| AC-6 | JSON output flag | Machine-readable per `ai/rules/json-format.md` |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Agent adds a YANG leaf and runs coverage before committing | loader -> walk -> constraint grade -> violation or clean | test/plugin/yang-coverage.ci |
| 2 | Maintainer audits what a build tag removes from the schema surface | report's gating column | TestYANGCoverageReport |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestYANGCoverageReport | coverage tool package test | counts, ownership join, gating labels | |
| TestYANGCoverageCheckFailsOnBareLeaf | same | AC-2, AC-5 | |
| TestYANGCoverageUnresolvedModule | same | AC-3 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (none numeric; N/A -- reporting tool) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| yang-coverage | `test/plugin/yang-coverage.ci` | command produces the report and supports pipes per `ai/rules/pipe-completeness.md` | |

### Interop Tests (MANDATORY for protocol features)
- N/A: developer tooling, no wire behavior.

## Files to Modify
- `cmd/ze/` command tree - `ze yang coverage` registration (per CLI grammar)
- `mk/` - `ze-yang-coverage` target; verify wiring per R-3 decision
- `docs/guide/command-reference.md` - document the command

## Files to Create
- coverage walker + report (location per module tiers during design; likely beside `internal/component/config/yang/`)
- check-mode entry in `scripts/checks/` family if design separates it
- `test/plugin/yang-coverage.ci` - functional test

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - command + make target registered, failing wiring tests
2. **Phase: module walk + counts** (A-1 prototype hardened)
3. **Phase: ownership/gating join + violations** (AC-2..AC-4)
4. **Phase: check mode + JSON output** (AC-5, AC-6; R-3 decision applied)
5. `make ze-verify`, learned summary, two-commit closure

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-6 with file:line |
| Correctness | inventory derives live from loader; zero hardcoded module lists |
| Naming | "coverage" framed as schema consistency, not standards conformance (R-2) |
| Registration over hardcoding | command registered via dispatch; derives from registry (`ai/rules/plugin-self-containment.md`) |
| Pipe completeness | report output supports all pipe operators |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | none from network; tool reads embedded/registered schema only |
| Resource exhaustion | walk bounded by schema size; no issue expected |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- (fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Schema-consistency report, not IETF coverage | a standards-coverage counter over IETF modules | Ze implements its own modules; the honest local guardrail is constraint/ownership consistency |

## Known Limitations
- Says nothing about standards conformance; `docs/comparison.md` and interop tests own
  that story.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
