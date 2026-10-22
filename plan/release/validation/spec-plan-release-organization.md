# Spec: plan-release-organization

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Preserve a complete, correctly classified release queue during the folder cutover |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Apply owner decisions M30-M33. Organize all work items below human-readable subfolders, separate release correctness from absent features, and split mixed scopes without losing work. Update the tools that discover, report, claim and close specs so relocation never hides a work item or claims completion. This work changes planning, not product behavior or RFC conformance verdicts.

## Required Reading

- [ ] `plan/README.md` and `docs/contributing/spec-workflow.md`.
  → Decision: retain one spec identity, metadata lifecycle and relocation-not-closure semantics; replace the directory-based classification.
- [ ] `internal/le/spec/path/specpath.go`.
  → Constraint: `Dirs`, `Globs`, `Bucket`, `All` and `Find` currently derive from one three-row declaration. Extend that authority rather than create independent inventories.
- [ ] `internal/le/spec/roadmap/roadmap.go`.
  → Constraint: committed snapshots, historical comparisons and public rendering must retain their revision boundary and distinguish absent metadata from an empty population.
- [ ] `internal/le/spec/session.go`.
  → Constraint: claims use stable filenames; the cap is 12, with M33 admitting this reorganization at 16 only.
- [ ] `docs/contributing/committing.md`, `ai/doors/system.md`, `ai/doors/le-tooling.md`, `docs/contributing/ze-go-style.md`.
  → Constraint: preserve concurrent edits, commit through explicit native scripts, and leave tracked sources in place until the relocation commit removes them.

## Current Behavior

`specpath.Bucket` accepts root, immediate and pre-release specs only. `specpath.All` searches their direct children. `roadmap.Collect` treats the after bucket as optional and everything else as required. Proposals in `to-review` are absent from that population. The root mixes work items, templates, generated inventory and historical records.

Preserve spec stems, status, dependencies, acceptance criteria, evidence and every unrelated edit. Change locations and release classification. Historical records remain historical; do not rewrite their account of old paths as if they described the current tree.

## Data Flow

Entry points: `le spec status`, `roadmap list/update/compare`, `claim`, `wip`, citation checks, hook checks, commit relocation detection and site roadmap rendering. A single path declaration identifies current categories. Metadata supplies priority and rationale. Filesystem inventory and committed snapshots feed their existing renderers. Historical snapshots and migration detection explicitly recognize historical layouts; current work has no fallback to obsolete folders.

| Boundary | Contract |
|----------|----------|
| Filesystem to inventory | Every current spec is visible exactly once; obsolete placements fail visibly rather than disappear |
| Git tree to historical report | Old revisions retain their actual paths and classifications |
| Prepared relocation to commit | Same stem at a new location remains a move, never completion |
| Spec to release queue | Correctness obligations and absent features have separate owners when mixed |

## Target Organization

| Directory | Meaning |
|-----------|---------|
| `plan/release/fixes/` | Incorrect shipped behavior and required behavioral repairs |
| `plan/release/validation/` | Proof of shipped behavior and release support claims |
| `plan/release/delivery/` | Artifacts, installation, appliance boot and essential onboarding |
| `plan/post-release/features/` | Additional capabilities and absent RFC features |
| `plan/post-release/engineering/` | Release-independent refactors, tooling and optimizations |
| `plan/proposals/` | Unapproved work, visible but not a release commitment |
| `plan/records/` | Loose historical records formerly at the root |
| `plan/templates/` | Spec templates |

Existing named journal, verification-debt, known-failure, learned and handover directories remain explicit supporting subfolders: their live obligations are not reclassified as historical. Root README is the navigation entry point. Generated roadmap moves below `plan/release/`. A folder does not encode lifecycle status.

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | Shared path authority reaches most inventory consumers | Read `specpath` and LSP references to `Bucket` | A consumer hides moved work | Inspect references and residual path literals | confirmed |
| A-2 | Existing specs mix fixes and expansion | Three triage reports and PKI Task | Whole-file moves misclassify release work | Read Task and remaining criteria per spec | confirmed |

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Concurrent session changes a source after its destination is prepared | Source content differs from recorded input | Refresh destination while preserving classification; do not commit a stale snapshot |
| R-2 | Migration is counted as closure | Commit helper requests closure proof for a pure move | Test old-to-new relocation recognition before moving |
| R-3 | Historical inventory becomes empty | Old revision loses work items | Keep explicit historical reading semantics and exercise comparison |
| R-4 | Split drops an obligation | Source criterion has no owning destination | Record criterion ownership in both split specs and independently review |
| R-5 | Tool build depends on another session's unfinished BGP edit | Named launcher compilation fails outside tooling | Preserve peer work; prove scoped packages and report blocked full-entry proof until reachable |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if wrong? | Every session's spec inventory, claims, closure guards and release reports |
| Recovery | Preserve source blobs and migration mapping; correct paths in place, never destructive reset |
| Other writers | All sessions editing active specs; refresh their source content before committing a relocation |

## Wiring Test

| Entry Point | Feature Code | Proof |
|-------------|--------------|-------|
| `le spec status` | `specpath.All`, status parser | Current-tree CLI inventory includes every migrated identity |
| `le spec roadmap list/compare` | `roadmap.Collect`, comparison | Before/after snapshots show moves and explicit split additions, not delivered work |
| `le commit create` | relocation classification | Scoped relocation fixture does not become closure |

## Acceptance Criteria

| ID | Condition | Expected behavior |
|----|-----------|-------------------|
| AC-1 | Inspect plan root | No work-item, template or historical-record files at root; README only among human-authored documents |
| AC-2 | Enumerate current specs | Six declared categories, proposals included, every stem counted exactly once |
| AC-3 | Inspect release classification | Every migrated spec has a priority and concise release rationale; release priority orders security/config correctness, routing, recovery and proof by dependencies |
| AC-4 | Mixed fix and feature scope | Split wherever classification differs; every acceptance criterion, dependency and remaining obligation has an explicit owner; no closure is inferred |
| AC-5 | Query old commit and compare migration | Historical inventory preserved; relocation recognized; no false delivery claim |
| AC-6 | Resolve or claim an existing stem | New path resolves, lifecycle and WIP semantics preserved |
| AC-7 | Generate roadmap and render site | New categories and priority are visible; no stale root roadmap or old category instruction remains on live guidance surfaces |
| AC-8 | Other sessions edited source plans | Their latest content survives relocation; no product code is changed by this migration |

## Test Plan

Update existing path, status, roadmap, citation, hook and commit behavioral fixtures. Add only missing consumer-visible boundaries: new categories, obsolete placement refusal, historical queries, and old-to-new relocation. Run scoped packages once after edits, then failed selectors only. Smoke actual inventory and roadmap commands on a named launcher. Compare pre/post identity sets, inspecting explicit split additions separately. No protocol tests change.

## Files to Modify

- `internal/le/spec/path/`, `internal/le/spec/status/`, `internal/le/spec/roadmap/`: authoritative layout, metadata and output.
- Consumers in `internal/le/commit/`, `internal/le/hookruntime/`, documentation checks and site rendering, plus affected behavioral tests.
- `plan/README.md`, spec templates, `docs/contributing/spec-workflow.md`, live rules/skills/index links and relevant source-anchored pages.
- Existing work-item files: relocation and bounded classification/split edits only.

## Integration and Documentation Checklists

| Surface | Applies | Action |
|---------|---------|--------|
| CLI output, pipes, inventory | Yes | Preserve registered actions and envelopes; add categories, priority and rationale through shared models |
| YANG, config, RPC, wire, plugins, SDK, env, metrics, runtime dependencies | No | No product behavior or new runtime dependency |
| Tests | Yes | Existing planning-tool behavioral fixtures and real CLI smoke |
| Architecture | Yes | Update source-anchored planning workflow and layout references |
| Discovery | Yes | Update live README, templates, rules/skills and indexes; regenerate canonical derivatives |
| Product feature declarations, comparison, user guides | No | No product capability changes or new conformance claim |

## Implementation Steps

1. Central layout and metadata: current six categories, priority/rationale, historical snapshot support, current obsolete-placement refusal.
2. Consumers and guidance: preserve claims, closure detection, citations, generation, site and template contracts.
3. Portfolio migration: classify all specs, create new destinations, separate mixed obligations and move loose root records/templates. Preserve originals until native relocation commit.
4. Verification and independent review: scoped tests, CLI smoke, population conservation and split ownership review; commit only explicit paths.

## Critical Review Checklist

| Check | Required evidence |
|-------|-------------------|
| Population | No missing or duplicate identity, including proposals and malformed metadata |
| Classification | No incorrect shipped behavior hidden as an optional feature |
| Splits | Every former criterion has exactly one implementation owner |
| History | Old commits remain queryable and moves are not closures |
| Concurrency | Destination content includes source changes made during migration |

## Deliverables Checklist

| Deliverable | Proof |
|-------------|-------|
| Readable subfolders | Root listing and metadata-aware inventory |
| Working tooling | Scoped behavioral tests and CLI smoke |
| Preserved work | Identity/content comparison and independent split review |

## Security Review Checklist

| Check | Required evidence |
|-------|-------------------|
| Path handling | No traversal acceptance, duplicate-stem ambiguity or silent excluded population |
| Release safety | Security fixes remain required when they affect shipped behavior |

## Owner Decisions

M30-M33 authorize the release boundary, readable subfolders, mixed-scope splits and one-time WIP admission at 16. The earlier proposed layout is approved. No product implementation is commissioned by this reorganization.
