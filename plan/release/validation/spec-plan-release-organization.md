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

## Implementation Summary

### What Was Implemented

- The six current queues derive from `internal/le/spec/path/specpath.go` (`Categories`, `Bucket`, `All`, `Find`). Current inventory refuses obsolete placement and duplicate identities; explicit historical readers preserve old classifications.
- `internal/le/spec/roadmap/roadmap.go` (`Collect`) reads one committed revision, carries priority and rationale, and distinguishes required work from proposals and post-release work. `internal/le/spec/roadmap/compare.go` (`Compare`) reports endpoint changes, never delivery. `internal/le/site/roadmap.go` (`renderRoadmap`) publishes JSON, HTML and Markdown from that same snapshot.
- `internal/le/commit/review.go` (`relocatedSpecs`, `closureStem`) recognizes same-stem old-to-new moves without closing work. `internal/le/spec/session.go` (`specOwner.Claim`) resolves stable filenames through `specpath.Find` while retaining locking, lifecycle and WIP checks.
- The migration preserves 450 source work items, adds 40 explicitly owned split work items, moves 20 records and two templates, and adds this governing spec separately. Independent portfolio reviews covered all source items and bounded corrections. Classification is not implementation, completion or RFC correctness.
- Implementation cutover commit: `4831b585d9`. The native deletion guard retained 145 old-path copies with preexisting uncommitted bytes; the integration owner preserved them non-destructively under scratch `preserved-relocation-inputs/`, with `preserved-relocation-inputs.json`. Their source hashes matched the migration inputs. A concurrent BGP-policy reference update was reconciled against the destination after citation normalization.

### Evidence Index

Evidence names below are relative to `tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/scratch/`, unless given as full paths.

| Evidence | What it establishes | Limit |
|----------|---------------------|-------|
| `migration-final.json` | Source-to-destination ownership for 450 source specs, 40 split additions, 20 records and two templates | This governing spec is an additional identity |
| `review-layout.diff`, `review-immediate.diff`, `review-prerelease.diff`, `review-backlog-a.diff`, `review-backlog-b.diff`, correction manifests | Reviewed implementation, classification and split obligations | Historical narrative is not a current product verdict |
| `plan-review-findings.txt` | Independent clean conclusions after three rounds | Does not claim whole-tree green |
| `plan-verification.json`, `job-plan-tools-check-3c53e983.log` | Twelve complete affected race packages; historical RFC-reference and site roadmap test passes | Broader run retained unrelated reds and the site package timeout |
| `job-plan-final-check-15a21df1.log` | Targeted hook catalog/digest checks PASS; native `le-spec-status-answers` PASS 1/1 | Does not exercise the real repository relocation or claim release |
| `plan-workflow-smoke.json` | Original fixture failure and exact causal analysis | Superseded only for the repaired expected-record comparison by the final targeted pass |
| `plan-held-docs.json` | Two citation edits belong to the active BGP owner's uncommitted pages | Neither absorbed nor reverted by this closure |
| `plan-cutover-evidence.json`, `plan-current-inventory.json`, `plan-current-roadmap.json`, `plan-migration-comparison.json` | Actual 491-item conservation, valid metadata, root cleanup and zero removed historical items | Counts are work inventory, not release readiness |
| `plan-roadmap-generation.json`, `plan-site-evidence.json`, `job-plan-site-smoke-96997f19.log` | Actual roadmap generation, full site build, served HTML inspection and site/CLI snapshot equality | Browser startup failed; no screenshot or narrow-viewport visual proof |
| `plan-review-check.json`, `plan-current-citations-fixed.json`, `plan-rules-lint.json` | Native review hashes clean, zero dangling citations, rules lint pass | Existing citation warnings and broader verification debt remain |

### Bugs Found/Fixed

- The status functional fixture compared equal-rank items by incidental position. Its producer, `internal/test/fixture/ui_fixture_le_spec_status_answers.go`, now matches independent expected records by name while preserving exact population and field assertions. The initial ordering failure is recorded in `plan-workflow-smoke.json`; `job-plan-final-check-15a21df1.log` records the repaired native pass.
- Hook fixture/source seals were updated for the changed contracts. `TestFixtureCatalogExactPopulationAndContent` and `TestEveryStoredDigestMatchesTheTreeItRatchets` passed in the final targeted log.
- Independent portfolio correction rounds restored explicit ownership of mixed proof, fixes, feature work, dependencies and file responsibilities. The correction manifests and final clean portfolio reviews, rather than whole-file bucket moves, establish that scope was retained.

### Documentation Updates

| Category | Updated? | Evidence and disposition |
|----------|----------|--------------------------|
| Feature list and maturity declarations | No product level change | `docs/contributing/feature-maturity.md` explains release-fix inventory; its anchor names `internal/le/feature/check.go` (`releaseFixSpecs`, `criterionDefects`). The layout diff changes discovery, not a feature promotion |
| User guide | No behavioral update | No product capability changed. Two citation-only edits in `docs/functional-tests.md` and `docs/guide/route-reflection.md` stay with their active owner, as `plan-held-docs.json` records |
| Config syntax and examples | No | The reviewed layout diff changes no YANG/parser contract |
| CLI reference | Yes | `docs/contributing/spec-workflow.md`, work queues and committed roadmap sections, documents categories, priorities, refusal and historical semantics; anchors name `specpath.Categories/Bucket/All/Find`, `roadmap.Collect`, `Update` and `indexCurrent` |
| API/RPC docs | No public API change | Existing registered actions and shared envelopes remain; the planning snapshot carries the new category/priority data |
| Plugin SDK | No | No plugin boundary or SDK contract changed |
| Wire format | No | No protocol encoder, decoder or wire behavior changed |
| RFC compliance | No conformance update | Historical planning-reference resolution is tooling, not newly proved RFC behavior; no support claim is earned |
| Comparison table | No | No capability or support-level change |
| Test infrastructure | Yes | Existing planning path/status/roadmap/citation/commit/hook tests and the native UI fixture cover the new layout; no protocol test changed |
| Architecture/design and discovery | Yes | `docs/contributing/spec-workflow.md`, `docs/contributing/committing.md`, `docs/contributing/gh-pages.md`, `plan/README.md`, `plan/templates/`, live rules/skills/index references; edits are in `review-layout.diff` and `reference-edits.json` |
| Runtime dependency / doctor | No | No new product path, socket, port, kernel module, binary or certificate dependency; no doctor registration owed |

`./le doc check verify` and full worktree verification are not recorded as green. Passing affected documentation packages are narrower evidence; the observed broader reds remain verification debt under M27 and `ai/rules/pre-release.md`.

### Deviations from Plan

- Native preservation of 145 changed old-path inputs required a non-destructive scratch move after the cutover; the committed destinations and input manifest retain the work.
- The broad scoped run did not finish green. Unrelated native-rule source-pin, RFC cache-path, numberparse allowlist and derived-RFC-link failures, plus the site package's ten-minute timeout, remain explicit. They do not remove any agreed feature or acceptance criterion.
- No new protocol interop or RFC verdict was commissioned. No claim is released by this independent closure worker; the shared native identity also carries BGP work.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Functional fixture expected incidental tie order | Stable sorting retains lexicographic path order for equal rank/date | Native smoke failed on delivery versus fixes | Compare expected records by name, preserving exact fields and population; targeted native pass |
| approach | Moving a mixed spec alone did not establish ownership | Correctness, proof and absent feature obligations can require different queues | Independent portfolio reviews | Explicit split ownership and dependency/file corrections; final portfolio reviews clean |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Human-readable work, record and template folders | Done | `specpath.go:52`, `migration-final.json`, `plan-cutover-evidence.json` | Actual root files: README.md and hidden .citation-baseline only |
| Separate release correctness from absent features | Done | `specpath.go:52`; portfolio review diffs | Required fixes/validation/delivery remain distinct from optional features/engineering and proposals |
| Split mixed scope without losing obligations | Done | `migration-final.json`; correction manifests | 40 additions, each with ownership review |
| Discover, report, claim and close without hidden work or false completion | Done; site visual proof recorded separately | `specpath.go:173`, `roadmap.go:75`, `compare.go:36`, `session.go:90`, `review.go:61` | Actual CLI counts 491 unique identities; comparison removes none |
| Preserve concurrent work and avoid product behavior changes | Done | `preserved-relocation-inputs.json`, `plan-held-docs.json`, `plan-cutover-evidence.json` | Zero source changes at cutover; product source citation changes do not change product behavior |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `plan-cutover-evidence.json`: root files are `README.md` and `.citation-baseline` | Records/templates moved and obsolete directories removed; generated roadmap now below release |
| AC-2 | Done | `plan-current-inventory.json`, `plan-current-roadmap.json`: 491 unique mapped identities | 131 fixes, 89 validation, 6 delivery, 156 features, 97 engineering, 12 proposals |
| AC-3 | Done | `plan-cutover-evidence.json`: no invalid metadata; reviewed priority/rationale assignments; `roadmap.Collect` sorts priority first | Dependencies stay explicit; the report is not an automatic dependency scheduler |
| AC-4 | Done | All four portfolio reviewers clean after corrections; 450 source identities and 40 split additions mapped | No moved or split item is declared delivered |
| AC-5 | Done | `plan-migration-comparison.json`: 447 changed/moved, 44 added, zero removed; named historical/relocation tests passed | Added = 40 split children + this governing spec + three proposals historically excluded from reports |
| AC-6 | Done | `TestClaimFindsTheMovedIdentity`, concurrent-cap test and complete spec package pass; actual `spec current` resolves this identity, `plan-current-wip.json` retains cap 12 and WIP 16 | M33 admits this work once; standing cap unchanged; shared claim not released |
| AC-7 | Done, with visual-verification limit | `plan-roadmap-generation.json`: exit 0 at `plan/release/roadmap.md`; `plan-site-evidence.json`: full build exit 0, served HTTP 200 HTML has all six categories/counts and priority/reason columns; site JSON equals actual CLI snapshot at `4831b585d973366d347a3d3a79868b92e6718f8b` | Generation/rendered content observed; browser startup failed, so no screenshot or narrow-viewport claim |
| AC-8 | Done | `plan-cutover-evidence.json`: identity conservation true, zero source changes at cutover; 145 originals preserved | Concurrent BGP-policy reference reconciled; held BGP docs remain their owner's work; no product behavior changed |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Category and obsolete-placement boundaries | Passed | `internal/le/spec/path/specpath_test.go`: `TestCategoriesOwnClassificationAndCopies`, `TestObsoletePlacementFailsCurrentInventory` | Complete race package pass |
| Current/historical parsing and status answer | Passed | `internal/le/spec/status/specstatus_test.go`: `TestParseSeparatesCurrentAndHistoricalPlacement`; `test/ui/le-spec-status-answers.ci` | Complete status package and repaired native fixture pass |
| Historical comparison and no closure from moves | Passed | `internal/le/spec/roadmap/roadmap_test.go`: `TestRoadmapOldToNewRelocationKeepsHistory`; `internal/le/commit/relocation_test.go`: `TestCurrentCategoryRelocationsNeverCloseWork` | Complete roadmap/commit package passes |
| Claims and WIP | Passed | `internal/le/spec/session_test.go`: `TestClaimFindsTheMovedIdentity`, `TestConcurrentClaimsCannotExceedTheWIPCap` | Complete spec package pass |
| Citation, hook and documentation consumers | Passed within named packages | `plan-verification.json`, final targeted hook log | Not equivalent to whole-tree docs verification |
| Site roadmap | Targeted tests and actual build/render passed | `internal/le/site/roadmap_test.go`; `job-plan-site-smoke-96997f19.log`; `plan-site-evidence.json` | Earlier whole-package timeout remains; browser visual proof unavailable |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/le/spec/path/`, `status/`, `roadmap/` | Done | Central declaration and typed metadata/reporting |
| Commit, hook, documentation and site consumers | Done | Reviewed layout diff and hash-pinned review artifact |
| Planning README/templates and live guidance | Done | Reviewed layout/reference edits; actual generated roadmap at the new path and cutover revision |
| Existing work items, records and templates | Done | `migration-final.json`, portfolio corrections and preserved-input manifest |

### Audit Summary

- Implemented scope: all five Task requirements, all four planned file groups and all six test-plan groups are accounted for.
- Acceptance evidence: all eight conditions accounted for; AC-7 generation and rendered content observed through actual build, HTTP HTML and matching JSON, not a browser screenshot.
- Partial or skipped feature scope: none. Browser/narrow-viewport inspection remains an explicit verification limit, not a reduced feature or acceptance condition.
- Changed execution: native preserved-input handling and explicitly qualified verification, described above.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Readable folders and complete inventory | Source, behavioral tests and actual native inventory | `plan-cutover-evidence.json`: six categories, 491 unique mapped identities, valid metadata; root README and hidden baseline only |
| Separate correctness from absent features without losing work | Independent scope review | Four portfolio reviews and correction manifests account for every source obligation and split |
| Relocation never hides work or claims completion | Behavioral tests and actual native workflow | Historical roadmap/commit tests and native status fixture pass; actual comparison has 447 moves, 44 explained additions, zero removals |
| Planning change, not product/RFC completion | Scope and source review | Product files carry reference changes only; endpoint counts explicitly disclaim release readiness; no support status or wire verdict promoted |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| No agreed implementation scope omitted | This is a planning/tooling cutover; portfolio obligations remain in their mapped owner specs | All existing and split owners are enumerated in `migration-final.json`; none is closed by this migration |

Full verification remains debt, not omitted feature scope. The known red list is the source-pin/RFC cache-path/numberparse/derived-link failures and site timeout recorded above. M27/pre-release forbids repeating an observed run to reconfirm it; later targeted repairs and eventual quiet full verification are still owed before push. This closure makes no release-readiness or product RFC-correctness claim.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/plan-release-organization-644a4e78-0122-4eeb-8642-eb50638a1641.md` |
| `./le spec review check` | Exit 0: `review_gate: OK (62 code files, clean, hashes match tmp/review/plan-release-organization-644a4e78-0122-4eeb-8642-eb50638a1641.md)`; `plan-review-check.json` |
| Rounds | 3 |
| Reviewer lenses used | History/layout, guards/security, workflow, immediate scope, pre-release scope, backlog A and B; 62 Go files pinned |
| Final independent verdict | CLEAN, 0 BLOCKER and 0 ISSUE, as `plan-review-findings.txt` and the machine artifact record |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Exact-record fixture depended on incidental tie ordering | `internal/test/fixture/ui_fixture_le_spec_status_answers.go` | Name-based expected-record match retaining all assertions; native fixture PASS |
| 2 | ISSUE | Mixed scope needed explicit proof/fix/feature and file/dependency ownership | Portfolio work-item destinations | Bounded correction manifests; independent scope rechecks CLEAN |

### Closure-record corrections

Two one-line citation corrections were independently read after the cutover. `plan/post-release/engineering/spec-journal-resolution.md` AC-2 now describes a spec path absent at HEAD without accidentally citing a concrete nonexistent file; its negative-case obligation and expected unresolved-pointer report are unchanged. `plan/proposals/spec-cgnat.md` Commit B now names its actual proposals path and the native closure script. Both travel with this record in closure commit A. `plan-current-citations-fixed.json` records exit 0 across 491 specs, zero dangling citations, and 18 existing source-citation warnings retained. `plan-rules-lint.json` records exit 0 for 34 rules and 247 points; neither result is full verification.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| Governing spec and `plan/templates/TEMPLATE-CLOSURE.md` | Yes | Independently read from their new paths during closure preparation |
| `internal/le/spec/path/specpath.go`, `internal/le/spec/roadmap/roadmap.go`, `internal/le/site/roadmap.go` | Yes | Independently read producing declarations and functions during closure |
| Migrated work-item destinations | Yes, committed and inventoried | Cutover `4831b585d9`, `migration-final.json`, `plan-cutover-evidence.json`: 491 exact unique mapped paths |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Root has only human-authored README | `plan-cutover-evidence.json` root files: `.citation-baseline`, `README.md` |
| AC-2 | Six categories and exact unique population | Actual inventory/roadmap exit 0; 491 identities; bucket counts 131/89/6/156/97/12 |
| AC-3 | Priority and rationale preserved | `plan-cutover-evidence.json` metadata-invalid list empty; reviewed priority producer and portfolio rationale |
| AC-4 | Every obligation retained | Independent portfolio clean conclusions and correction manifests; no completion inferred |
| AC-5 | Old history and same-stem relocation preserved | Actual cutover-parent comparison: 447 moves, 44 explained additions, zero removed; old classifications preserved |
| AC-6 | Stable identity and lifecycle/WIP | Source `specOwner.Claim`, passed moved-identity/concurrent-cap tests, actual current identity and WIP 16/cap 12 |
| AC-7 | New roadmap and live guidance | Actual generation exit 0; full site build exit 0 writes 1012 pages from 22 producers; served HTTP 200 roadmap includes six category headings/counts and priority/reason columns; JSON matches CLI. No browser visual pass |
| AC-8 | Concurrent content survives | Final cutover record: identity conservation true, source changes empty; 145 guarded inputs backed up, no work discarded |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `le spec status` and JSON pipe | `test/ui/le-spec-status-answers.ci` | Final native log PASS 1/1 through named launcher and compiled fixture |
| `le spec roadmap list/compare` | No new .ci; existing committed-Git behavioral tests | Complete roadmap race package passed; actual native list/comparison exit 0 with 491 current, 447 historical, zero removed |
| `le commit create` relocation classification | No new .ci; existing native commit fixtures | Complete commit race package passed; cutover committed 450 relocations without treating them as closure |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Shared `specpath` declaration is consumed by current inventory, claims, roadmap, citation and closure guards; reviewed consumer/reference edits and package passes |
| A-2 | confirmed | Four independent portfolio reviews required 40 split additions and bounded corrections to preserve mixed obligations |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Current queues, refusal and stable identity | `specpath.go`: `Categories`, `All`, `Find`; workflow source anchor | Yes, source read |
| Committed roadmap, priority and historical boundary | `roadmap.go`: `Collect`, `snapshotCategories`; `compare.go`: `Compare` | Yes, source read; dependencies are displayed, not scheduled |
| Site shares one snapshot | `site/roadmap.go`: `renderRoadmap`; `plan-site-evidence.json` snapshot matches actual CLI | Yes, source and actual artifacts; browser rendering geometry not verified |
| Commit relocation is not completion | `commit/review.go`: `relocatedSpecs`, `closureStem`; commit behavioral tests | Yes, reviewed producer and passing package evidence |
| No config/API/SDK/wire/support change | Reviewed layout diff and `plan-held-docs.json`; categories accounted for above | No behavioral documentation change owed; no product correctness claim |
| Whole-tree documentation verification | Broader scoped log retains derived-link failures | Not green; debt is explicit, not silently waived |

## Security Review Results

| Concern | Evidence | Result |
|---------|----------|--------|
| Traversal and ambiguous identity | `specpath.bucketIn` refuses `..`; `Find` refuses path separators; `All` refuses duplicate stems and obsolete locations | Reviewed guards and complete path package pass; no silent inventory fallback |
| Release safety | Required categories remain fixes/validation/delivery; independent portfolio reviews and correction manifests keep shipped-behavior obligations required | No security obligation is demoted merely because it shared a spec with an absent feature |
| Concurrent writers and claim authority | `specOwner.Claim` retains repository lock, basename marker, atomic writes and standing cap; native retained inputs preserved, shared live claim untouched | No new claim bypass or discarded peer work |
| Rendering and untrusted metadata | Existing roadmap fixture covers script/markup-bearing metadata; shared Markdown/site renderer retained; `Collect` refuses duplicate or unreadable input | Reviewed history/workflow/guard lenses clean; HTML inspection is not a browser or security certification |
| Injection, memory bounds, cryptography, privilege and runtime resources | Scope is local planning paths, metadata and existing renderers, not a new protocol listener, credential boundary or runtime dependency | No newly commissioned product attack surface; no broader security certification claimed |

## Executive Summary

All agreed implementation work and all eight acceptance conditions are accounted for. The actual cutover preserves 491 unique work items before this governing spec closes, and comparison with its parent reports no removal. Actual generation, full site build and served HTML/JSON establish AC-7's rendered content; browser visual inspection remains unverified.

Risks and observations: R-1 concurrent inputs were reconciled and guarded originals preserved; R-2 moves are not closure; R-3 historical classifications remain intact; R-4 split ownership was independently reviewed; R-5 the named launcher and native smoke are reachable, but unrelated broad verification reds remain. The site package timeout and full verification debt are not erased by narrower passes.

### Final verification limits

The full site build succeeded (1012 pages, 22 producers, 559.28 seconds), separately from the earlier site test-package timeout. Its seven stale-supported feature rows and two unstamped talk pages were disclosed, not refreshed or promoted. Managed browser startup timed out; a direct Chrome attempt produced no stderr or CDP endpoint and was observed in kernel I/O wait (`D`, `folio_`). Only that owned browser process was terminated (exit -15). No screenshot, narrow-viewport or JavaScript-disabled visual result is claimed. `plan-site-evidence.json` records this limitation alongside successful served HTML inspection and exact site/CLI snapshot equality.

No lint-all, unit-all, functional-gating, full worktree verification or full documentation-verification green is claimed by this record. The independent closure worker ran no tests, gates, builds or formatters and released no claim. The integration owner retains native two-commit closure and claim-ownership responsibility.
