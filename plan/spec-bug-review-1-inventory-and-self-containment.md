# Spec: bug-review-1 -- Inventory and Self-Containment

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-bug-review-0-umbrella.md |
| Phase | - |
| Updated | 2026-06-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-bug-review-0-umbrella.md` - umbrella scope and child order
3. `internal/component/plugin/all/all.go` - generated import inventory
4. `ai/rules/plugin-self-containment.md` - removal invariant
5. `ai/rules/plugin-design.md` - plugin registration, lifecycle, dependencies, YANG ownership
6. `ai/patterns/registration.md` - all registration mechanisms
7. `plan/spec-cross-plugin-switch-audit.md` - overlapping ownership/switch audit

## Task

Build the authoritative inventory for the bug review. The inventory must identify every authored plugin surface compiled into Ze, every generated or schema-only surface that exposes commands/config, every RPC command package, and every core-engine package included in the review. It must also mark exclusions explicitly so later child specs do not silently skip code.

This child is a prerequisite for all other bug-review children. Do not start area review from a hand-written plugin list.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-bug-review-0-umbrella.md` - parent scope and review contract
  → Decision: this child produces the source-of-truth inventory used by children 2 through 5.
  → Constraint: review remains read-only; defects discovered here become findings, not direct edits.
- [ ] `internal/component/plugin/all/all.go` - generated blank-import aggregator
  → Decision: import groups define four coverage classes: infrastructure schema packages, plugin packages, event namespace packages, and RPC command packages.
  → Constraint: generated files are evidence only; never edit this file by hand.
- [ ] `ai/rules/plugin-self-containment.md` - full user-facing removal test
  → Decision: inventory records the owning package for command schema, RPC handler, offline CLI, help/completion, doctor check, metrics, and web/LG routes.
  → Constraint: central verb packages may own only generic roots or cross-system commands.
- [ ] `ai/rules/plugin-design.md` - plugin registration and lifecycle
  → Decision: inventory marks each package as system plugin, BGP plugin, component-owned plugin, schema-only plugin surface, RPC command package, or generated glue.
  → Constraint: infrastructure imports plugin implementation only through blank imports; test imports are tolerated but not production ownership.
- [ ] `ai/patterns/registration.md` - registry sources
  → Decision: inventory must reconcile generated imports with `registry.Register`, `pluginserver.RegisterRPCs`, YANG module registration, command registry registration, env registration, and doctor registration.
  → Constraint: a surface with two authorities is a candidate bug unless one is generated from the other.
- [ ] `plan/learned/RECURRING-PATTERNS.md` - recurring inventory defects
  → Decision: explicitly hunt hardcoded registry counts, unwired features, duplicate sources of truth, and package-level registry contamination.
  → Constraint: a test that asserts a literal plugin/RPC count is suspect unless it documents a floor and derives the real registry.

### RFC Summaries (MUST for protocol work)
- [ ] N/A for inventory creation
  → Constraint: protocol RFCs are read by child specs when a candidate touches wire or state-machine behavior.

**Key insights:**
- `all.go` is generated but still the closest compiled truth for plugin/schema/RPC inclusion.
- Directory names alone are insufficient. Some command surfaces are schema-only directories, some handlers live in component packages, and some plugins own multiple subpackages.
- The inventory must carry owner, surface type, entry point, generated source, and child spec assignment.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/all/all.go:10-137` - infrastructure schema package imports include component schemas, central verb schemas, BGP plugin command schemas, and `internal/plugins/*/yang` schemas.
  → Constraint: schema-only directories such as `*-cmd/yang` must still appear in review scope because they expose user commands.
- [ ] `internal/component/plugin/all/all.go:138-229` - plugin package imports include BGP plugin packages, NLRI families, component plugin packages, system plugin backends, and command subpackages.
  → Constraint: child coverage must include BGP plugin packages, `internal/plugins/*` packages, and component-owned plugins such as LDP, RSVP-TE, iface, traffic, flowexport, and VPP.
- [ ] `internal/component/plugin/all/all.go:230-272` - event namespace and RPC command package imports.
  → Constraint: RPC handlers can be user-visible even when the owning package is not under `internal/plugins/`.
- [ ] `internal/plugins/` directory listing - current tree contains system plugins, backend plugins, command schema packages, and support plugins.
  → Constraint: directory listing is a candidate source but not sufficient by itself; compare to generated imports and registry calls.
- [ ] `internal/component/bgp/plugins/` directory listing - current tree contains BGP policy, RIB, route-server, route-reflector, RPKI, BMP, GR, capability, NLRI, and command plugin packages.
  → Constraint: BGP plugin review must include nested NLRI family packages under `nlri/`, not only direct children.
- [ ] `plan/spec-cross-plugin-switch-audit.md:18-115` - existing cross-plugin switch list across ten boundaries.
  → Decision: inventory marks switch-heavy boundaries so later review can reuse this pending audit rather than duplicate it.

**Behavior to preserve:**
- Generated import ordering remains unchanged.
- Existing plugin registration and command/YANG owner model remains unchanged during inventory.
- No production code is edited.

**Behavior to change:**
- A durable review inventory exists and becomes the child-spec scope source.
- Any plugin surface not covered by a child spec is explicit, with owner and reason.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Generated aggregator imports, directory listings, registry calls, YANG registrations, RPC registrations, command registry registrations, doctor registrations, env registrations, and active specs.

### Transformation Path
1. Parse generated import groups into raw package rows.
2. Categorize each row by surface type and owning subsystem.
3. Cross-check rows against directory candidates and registry calls.
4. Assign each row to child 2, child 3, child 4, or child 5.
5. Emit the inventory report with uncovered, duplicate-owner, generated-only, and excluded tables.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Generated imports -> review inventory | direct read of `all.go` import groups | [ ] every import row accounted for |
| Directory candidates -> compiled scope | compare directory names with generated imports | [ ] directory-only candidates classified |
| Registry calls -> owners | structural search for registration calls | [ ] each user-visible surface has owner |
| Inventory -> child specs | child assignment column | [ ] no row has empty child assignment unless excluded |

### Integration Points
- `internal/component/plugin/all/all.go`
- `internal/component/plugin/registry/`
- `internal/component/plugin/server/rpc_register.go`
- `internal/component/config/yang/`
- `internal/component/command/registry/`
- `internal/core/diagnostic/`
- `internal/core/env/`
- `docs/architecture/core-design.md`

### Architectural Verification
- [ ] No bypassed layers: generated imports and registries both checked.
- [ ] No unintended coupling: duplicate owners become findings.
- [ ] No duplicated functionality: repeated schema/handler ownership must be documented or reported.
- [ ] Zero-copy preserved where applicable: inventory does not propose implementation changes.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|----------------------------------|----------|--------------|--------|
| A-1 | `all.go` plus registries is sufficient to identify compiled in-tree plugin surfaces | generated file comment and registration pattern | Review misses code linked by a build tag or root command path | compare `all.go` with `find` output and command registry registrations | unvalidated |
| A-2 | Schema-only command directories are user-visible review scope | `all.go` schema imports and plugin self-containment rule | Command bugs hidden in YANG grammar are skipped | inventory has surface type `schema-only` and child assignment | unvalidated |
| A-3 | Component-owned protocol plugins such as LDP and RSVP-TE belong in plugin review even outside `internal/plugins/` | `all.go` plugin package imports and root AGENTS architecture | user-intended scope too narrow or too broad | owner table separates component plugin packages from system plugins | unvalidated |
| A-4 | Vendor, tmp, generated `yang/register.go`, generated `all.go`, and pure test helpers are excluded unless they expose a user-visible compiled surface | project generated-file rules | real defects in generated glue are ignored | generated source is traced back to canonical owner and generator | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | Inventory becomes a stale static list | new plugin appears but report not updated | derive from current source each time review starts |
| R-2 | Nested plugin subpackages are missed | package path under a plugin has no row | include subpackage rows when they register handlers, schemas, or families |
| R-3 | Ownership classification is subjective | same command/schema appears in two owner rows | cite handler dependency and self-containment rule to decide owner |
| R-4 | Active specs overlap and duplicate findings | finding references a file under an active spec | include active spec column and route finding to that spec |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `internal/component/plugin/all/all.go` import groups | -> | inventory raw package rows | `BugReviewInventoryAllImportsAccounted` |
| plugin/schema/RPC registries | -> | owner and surface tables | `BugReviewInventoryRegistriesAccounted` |
| inventory rows | -> | child spec assignments | `BugReviewInventoryNoUnassignedRows` |
| excluded candidates | -> | exclusion table with reason | `BugReviewInventoryExclusionsHaveReason` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Generated aggregator is read | Every import path from lines 10-273 is represented in the inventory report or excluded with a reason |
| AC-2 | Directory candidates are enumerated | Every direct child under `internal/plugins/`, `internal/component/bgp/plugins/`, and relevant component plugin paths is classified |
| AC-3 | A package registers a plugin, schema, RPC, command, env var, doctor check, event type, send type, family, capability, or metric | Inventory records the owner and review child assignment |
| AC-4 | A user-visible command/schema surface is central or generic | Inventory marks whether it is allowed generic scope or a self-containment candidate |
| AC-5 | A row belongs to an active spec | Inventory records the active spec so later findings do not conflict silently |
| AC-6 | Inventory is complete | Unassigned compiled package count is zero |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Starts the bug-review program | generated imports -> inventory report -> child assignments | `BugReviewInventoryNoUnassignedRows` |
| 2 | Checks why a package is or is not reviewed | package row -> surface type -> owner -> child assignment or exclusion reason | `BugReviewInventoryExclusionsHaveReason` |
| 3 | Adds a future plugin before review starts | generated import changes -> inventory derivation includes new row | `BugReviewInventoryAllImportsAccounted` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `BugReviewInventoryAllImportsAccounted` | `plan/review-bug-review-inventory.md` | every import row in `all.go` appears in inventory or exclusion table | |
| `BugReviewInventoryRegistriesAccounted` | `plan/review-bug-review-inventory.md` | registry call classes are represented | |
| `BugReviewInventoryNoUnassignedRows` | `plan/review-bug-review-inventory.md` | every in-scope row has child assignment | |
| `BugReviewInventoryExclusionsHaveReason` | `plan/review-bug-review-inventory.md` | exclusions are explicit and justified | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | Inventory review adds no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `BugReviewInventoryReadableByChildren` | `plan/review-bug-review-inventory.md` | child reviewers can scope themselves without re-deriving plugin lists | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | | | Inventory only | |

### Future (if deferring any tests)
- No deferral. If the inventory cannot be fully derived, stop before area review.

## Files to Modify

- No production code files.
- Read-only scope includes `internal/component/plugin/all/all.go`, registry packages, command/YANG registration packages, and active plan files.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Review only |
| CLI commands/flags | No | Review only |
| Functional test for new RPC/API | No | Review only |
| Env var registration | No | Review only |
| Doctor check for runtime dependencies | No | Review only |
| Prometheus counters/metrics | No | Review only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | Review only |
| 16 | Any changed source file is referenced by existing doc source anchors? | No | Review-only spec |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create

- `plan/review-bug-review-inventory.md` - inventory report used by later children.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file and umbrella |
| 2. Audit | Required Reading, generated imports, registry calls |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Produce inventory report, no production code |
| 5. /ze-review gate | Review inventory report for omissions |
| 6. Full verification | Report audits in TDD plan |
| 7-14 | Standard completion |

### Implementation Phases

1. **Phase: Import inventory** - derive package rows from `all.go` import groups.
   - Tests: `BugReviewInventoryAllImportsAccounted`.
   - Files: `internal/component/plugin/all/all.go`.
   - Verify: import row count in report matches source count.
2. **Phase: Directory reconciliation** - compare generated rows to plugin directories.
   - Tests: `BugReviewInventoryNoUnassignedRows`.
   - Files: `internal/plugins/`, `internal/component/bgp/plugins/`, relevant component plugin packages.
   - Verify: directory-only rows are explained.
3. **Phase: Registry reconciliation** - identify registry calls by category and owner.
   - Tests: `BugReviewInventoryRegistriesAccounted`.
   - Files: registry packages and owner packages.
   - Verify: owner/source tables complete.
4. **Phase: Child assignment** - assign each row to child review scope or exclusion.
   - Tests: `BugReviewInventoryExclusionsHaveReason`.
   - Files: `plan/review-bug-review-inventory.md`.
   - Verify: no unassigned in-scope rows.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every `all.go` import row accounted for |
| Feature completeness | BGP plugins, system plugins, component-owned plugins, schema-only surfaces, event namespaces, RPC command packages all represented |
| Correctness | Owner classifications cite dependency or registry evidence |
| Naming | Stable row IDs use prefixes SCHEMA, PLUG, EVT, RPC, DIR, EXCL |
| Data flow | Inventory derivation path is reproducible from source files |
| Rule: self-containment | Candidate ownership violations cite the removal test |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Inventory report | read `plan/review-bug-review-inventory.md` |
| Unassigned count zero | inventory summary table |
| Exclusions justified | exclusion table has reason per row |
| Child assignment complete | child assignment column has no blanks for in-scope rows |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|------------------|
| Sensitive data | Inventory must not include secrets from config or env |
| Resource use | Inventory derivation must not crawl vendor or tmp |
| Confusion | Package aliases and generated schema packages must not be conflated with owners |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Generated import row has no owner | classify as generated schema/RPC, directory candidate, or bug candidate |
| Directory exists but no import or registry | exclusion table or orphan candidate |
| Registry call has no generated import path | check build tags, root command imports, or report as wiring candidate |
| Child assignment disputed | resolve by user-visible owner and self-containment rule |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- Inventory is part of the review, not a preparatory note. It is the defense against skipped plugin surfaces.

## Core Insight

Generated imports tell us what is compiled, but ownership tells us what must be reviewed together. Both are required.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Inventory from generated imports plus registries | Directory-only inventory | Directory-only misses schema/RPC surfaces and build-tag/aggregator details |
| Record exclusions | Ignore obvious non-scope paths | Unrecorded exclusions become silent scope cuts |
| Assign child per row | Let children decide scope independently | Prevents overlap and omission |

## Known Limitations

- Inventory does not prove code correctness. It only proves review coverage.
- Generated schema glue is not reviewed line-by-line unless a bug points to the generator or canonical YANG.

## RFC Documentation

N/A for inventory. Protocol RFC checks happen in children 3 and 4.

## Implementation Summary

### What Was Implemented
- Created `plan/review-bug-review-inventory.md`.
- Accounted for 258 generated import rows from `internal/component/plugin/all/all.go`.
- Classified runtime plugin packages, schema-only command packages, event namespaces, RPC command packages, directory-only command roots, and exclusions.
- Assigned all in-scope rows to child 2, child 3, child 4, or child 5.

### Bugs Found/Fixed
- `INV-OBS-1`: Generated architecture inventory omits `internal/component/bgp/plugins/capa`, while the generated plugin aggregator imports it. Classified as a NOTE and routed to child 4 scope, not a production fix.
- `INV-OBS-2`: Several `internal/plugins/*` command roots are intentionally `codegen:skip`; classified as child 2 command-wiring scope, not missing imports.
- No production code was changed.

### Documentation Updates
- None. This review created a plan report only and did not change user-facing behavior, config, CLI, API, wire format, or architecture.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Build authoritative inventory | done | `plan/review-bug-review-inventory.md` | Import groups, directories, registry classes, and skipped command roots are covered |
| Mark exclusions explicitly | done | `plan/review-bug-review-inventory.md` Exclusions | Vendor, tmp, generated glue, pure test helpers, and build-tag roots have reasons |
| Assign child scope | done | `plan/review-bug-review-inventory.md` Child Scope Handoff | Unassigned count is zero |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `plan/review-bug-review-inventory.md` Generated Import Ledger | all.go import groups counted and mapped |
| AC-2 | done | Directory Reconciliation table | `internal/plugins`, BGP plugins, NLRI, and BGP cmd directories classified |
| AC-3 | done | Registry Reconciliation table | registry classes represented and assigned |
| AC-4 | done | Assignment Rules and Directory-Only Command Providers | central/generic and command roots classified |
| AC-5 | done | Active Spec Overlap table | selected active specs recorded |
| AC-6 | done | Summary and audit tests | unassigned compiled package count is zero |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| BugReviewInventoryAllImportsAccounted | pass | `plan/review-bug-review-inventory.md` | manual report audit |
| BugReviewInventoryRegistriesAccounted | pass | `plan/review-bug-review-inventory.md` | manual report audit |
| BugReviewInventoryNoUnassignedRows | pass | `plan/review-bug-review-inventory.md` | manual report audit |
| BugReviewInventoryExclusionsHaveReason | pass | `plan/review-bug-review-inventory.md` | manual report audit |
| BugReviewInventoryReadableByChildren | pass | `plan/review-bug-review-inventory.md` | manual report audit |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `plan/review-bug-review-inventory.md` | created | inventory report |

### Audit Summary
- **Total items:** 16
- **Done:** 16
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 report file created

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Authoritative plugin/core review inventory | Inventory report | `plan/review-bug-review-inventory.md` with zero unassigned in-scope rows |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `capa` is imported by generated plugin aggregator but absent from the generated architecture inventory in `AGENTS.md` | `internal/component/plugin/all/all.go#B835` line 144 | Routed as `INV-OBS-1`, child 4 treats it as in-scope |
| 2 | NOTE | `codegen:skip` command roots are expected directory-only rows | `codegen:skip` search and `cmd/ze/ze_core_dispatch.go#B708` | Routed as `INV-OBS-2`, child 2 treats them as command-wiring scope |

### Fixes applied
- None during review spec execution.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | No BLOCKER or ISSUE after report self-review | `plan/review-bug-review-inventory.md` | No action |

### Final status
- [x] Critical review of inventory artifact records no BLOCKER or ISSUE against the inventory deliverable
- [x] All NOTEs recorded above or explicitly none

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/review-bug-review-inventory.md` | yes | `read plan/review-bug-review-inventory.md:1-230` returned the report |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | every all.go import group represented | report lines 31-37 count all import groups |
| AC-2 | directory candidates classified | report lines 62-70 list directory reconciliation |
| AC-3 | registry surfaces represented | report lines 98-107 list registry reconciliation |
| AC-4 | generic/central and command-root surfaces classified | report lines 39-60 and 72-96 |
| AC-5 | active specs recorded | report lines 109-116 |
| AC-6 | unassigned count zero | report lines 14 and 37 |

### Wiring Verified (end-to-end)
| Entry Point | Report Audit | Verified |
|-------------|--------------|----------|
| `internal/component/plugin/all/all.go` import groups | `BugReviewInventoryAllImportsAccounted` | yes |
| plugin/schema/RPC registries | `BugReviewInventoryRegistriesAccounted` | yes |
| inventory rows | `BugReviewInventoryNoUnassignedRows` | yes |
| excluded candidates | `BugReviewInventoryExclusionsHaveReason` | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | report assumptions table and generated import ledger |
| A-2 | confirmed | schema-only rows assigned in report |
| A-3 | confirmed | component-owned rows assigned in report |
| A-4 | confirmed | exclusions identify generated glue and canonical owners |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Documentation updates needed | Documentation checklist says review-only, no user-facing changes | yes |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-N all demonstrated
- [x] End-to-End User Stories: every story has a working path and report audit
- [x] Wiring Test table complete
- [x] Critical review gate clean for inventory artifacts
- [x] `make ze-spec-status` passes
- [x] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass, defer with user approval)
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction
- [x] No speculative features
- [x] Single responsibility
- [x] Explicit behavior
- [x] Minimal coupling

### TDD
- [x] Report audits written
- [x] Boundary tests N/A documented
- [x] Goal Validation table filled

### Completion (BLOCKING, before ANY commit)
- [x] Critical Review passes
- [x] Partial/Skipped items have user approval or are not applicable
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Write learned summary to `plan/learned/939-bug-review-1-inventory-and-self-containment.md`
- [x] **Commit A script prepared:** spec + report + learned summary + counter bump in `tmp/commit-f32fa560.sh`
- [x] **Commit B script prepared:** remove `plan/spec-bug-review-1-inventory-and-self-containment.md` only after final state is preserved
