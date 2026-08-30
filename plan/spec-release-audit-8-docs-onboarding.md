# Spec: release-audit-8-docs-onboarding

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-release-audit-1-surface-inventory.md |
| Phase | - |
| Updated | 2026-05-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-release-audit-0-umbrella.md` - release audit blocker policy and finding schema
3. `plan/spec-release-audit-1-surface-inventory.md` - release surface inventory and existing docs/test finding
4. `ai/rules/writing.md` - source anchors and doc validation rules
5. `docs/contributing/documentation-testing.md` - doc validation workflow
6. `docs/guide/quickstart.md` - first-user onboarding path
7. `internal/le/docvalid/drift.go` - documentation drift checker
8. `internal/le/docstocode/codetodocs.go` - source anchor reverse index checker

## Task

Audit Ze's release-facing documentation and onboarding path before first user-facing release. The audit covers README, quickstart, guide index, feature docs, command/config references, documentation validation tools, local markdown links, and release evidence documentation.

This spec documents findings only. It does not fix product code, tests, schemas, generated files, Makefiles, or documentation. Future fix work must be approved separately and must include the verification requested by each finding.

## Audit Scope Boundary

This child audit records documentation, onboarding, and documentation-tooling issues. It may run read-only or validation commands to prove findings, but it does not edit source, tests, schemas, docs, generated files, or Makefiles.

Every docs/onboarding finding must include:
- The observed documentation, onboarding, or evidence issue.
- The user, contributor, or release impact.
- Source, doc, command, or validation evidence proving the issue exists.
- The future owner area.
- Suggested fix direction, without implementing it here.
- Verification required from the future fix.

## Required Reading

### Architecture Docs and Rules

- [ ] `plan/spec-release-audit-0-umbrella.md` - release audit method and blocker policy
  -> Decision: use the umbrella finding schema and keep audit work separate from fixes
  -> Constraint: stale or invalid first-user docs are release blockers until future fix evidence exists
- [ ] `plan/spec-release-audit-1-surface-inventory.md` - docs/onboarding surface row and existing RA-DOC-002 routing
  -> Decision: consume README, guide pages, feature docs, help/docs tooling, install, examples, and release evidence rows
  -> Constraint: docs findings should map to a first-user or release-engineer path
- [ ] `ai/rules/writing.md` - source anchors and validation targets
  -> Decision: audit factual docs against source and tooling rather than memory
  -> Constraint: every factual docs claim should be source-backed, and `./le doc check verify` is the documented validation target for docs changes
- [ ] `ai/rules/evidence.md` - line-backed factual claims
  -> Constraint: findings cite exact source or tool output, and avoid inferred status
- [ ] `ai/rules/commands.md` - no pipes for build/test/verify commands
  -> Constraint: validation commands were run cleanly, without piping their output
- [ ] `docs/contributing/documentation-testing.md` - documentation drift and command validator workflow
  -> Decision: `./le doc check verify`, `./le docvalid doc-drift`, `./le docvalid command-contract`, `./le consistency`, and `./le docs-to-code index-check` are separate evidence tools
  -> Constraint: `./le doc check verify` is not part of `./le verify current mode full` today, so docs drift can exist outside the default gate

### Source and Documentation Files

- [ ] `README.md` - release landing page, quickstart, Go version, test-count claims, docs links
  -> Decision: first release readiness needs README claims to match live source and current guide docs
- [ ] `docs/guide/README.md` - user guide index and feature routing
  -> Decision: guide index is an onboarding map and must not link users to stale feature paths
- [ ] `docs/guide/quickstart.md` - build, init, minimal config, validate, start, verify path
  -> Decision: quickstart must be executable from a clean checkout with current command output
- [ ] `docs/guide/ze-install.md` - local install, systemd, remote provisioning onboarding path
  -> Decision: install docs belong in the first-user docs surface even when not tested by `./le doc check verify`
- [ ] `docs/functional-tests.md` - release gate and evidence documentation
  -> Decision: release engineers depend on this file for what is and is not gated
- [ ] `docs/DESIGN.md` - shipped plugin table, test philosophy, interop count
  -> Decision: design doc feature inventory must match registered plugins and live interop scenario count
- [ ] `docs/features/interoperability-testing.md` - public interop feature page
  -> Constraint: scenario count and list must match `test/interop/scenarios/`
- [ ] `docs/architecture/testing/interop.md` - interop architecture and scenario inventory
  -> Constraint: negative or not-covered lists must be checked against current scenario tree
- [ ] `internal/le/` native action tables and `internal/le/functional/suites.go` - split the native action tables under `internal/le/` include and functional gate suites
  -> Decision: documentation tooling must follow included Makefiles or derive suites through `make`, not only the root file body
- [ ] `internal/le/fuzz/actions.go` - current fuzz duration and target list
  -> Constraint: fuzz documentation must not stale relative to `ze-fuzz-test`
- [ ] `internal/le/docvalid/drift.go` - docs drift checker implementation
  -> Decision: doc validation is itself a release surface because it decides whether docs are trustworthy
- [ ] `internal/le/docstocode/codetodocs.go` - source-anchor reverse index and stale-reference check
  -> Decision: source-anchor validation must be safe to run in audit mode and agree with documented anchor format

**Key insights:**
- Local markdown links are not covered by the existing doc validation target.
- (Four insight bullets tied to findings RA-DOC-001/002/003/005/006 removed 2026-07-10 as no longer true; see Post-wave corrections for the re-verified state.)

## Current Behavior (MANDATORY)

**Source files and docs read:**
- [ ] `go.mod` - declares `go 1.26` at line 3
- [ ] `docs/contributing/documentation-testing.md` - says `./le doc check verify` runs all documentation tests at lines 8-14 and is not part of `./le verify current mode full` at lines 42-44
- [ ] `internal/le/docvalid/drift.go` - checks docs against registry, filesystem, the native action tables under `internal/le/`, README, features, and functional docs at lines 73-90
- (Observation bullets for the removed findings RA-DOC-001 and RA-DOC-005 deleted 2026-07-10; see Post-wave corrections.)
- [ ] `internal/le/` native action tables - includes split the native action tables under `internal/le/` modules at lines 50-59, including `internal/le/functional/suites.go`
- [ ] `internal/le/functional/suites.go` - owns `./le functional` and lists 12 gated suites at lines 40-70
- [ ] `docs/functional-tests.md` - claims the functional target runs 12 suites at lines 17-21, and documents fuzz count/duration at lines 1112-1140
- [ ] `internal/le/fuzz/actions.go` - `ze-fuzz-test` uses 10s fuzz time at lines 4 and 14-70
- (Observation bullets for the removed findings RA-DOC-003, RA-DOC-006, and RA-DOC-007 deleted 2026-07-10; see Post-wave corrections for the re-verified current state.)

**Validation commands run:**
- the retired `ze-consistency-check` (current: `./le consistency`) failed with 42 errors and 712 warnings; doc-relevant output includes stale source refs in `internal/component/bgp/plugins/rib/storage/pathset.go` and missing plugin command package documentation/schema markers. (2026-07-10 re-run: 84 errors, 1021 warnings, same pathset refs; see Post-wave corrections.)
- (Command-run bullets for the removed findings RA-DOC-002-as-filed, RA-DOC-006, and RA-DOC-007 deleted 2026-07-10; the 2026-07-10 `./le doc check verify` state is recorded in Post-wave corrections.)

**Behavior to preserve:**
- Documentation remains source-backed and should not rely on memory.
- `./le doc check verify` remains the main documentation drift and command-contract target.
- `./le verify current mode full` remains the fast code gate, while docs-specific checks stay explicit until a separate release-gate design changes that policy.
- Source anchors remain invisible comments in rendered Markdown.
- The release audit does not edit docs or tooling directly.

**Audit documentation goal:**
- Record first-user blockers and docs tooling gaps before release.
- Route each docs issue to a future fix with concrete verification.
- Make docs validation failures visible in the release audit even when the default verification gate omits them.

## Data Flow (MANDATORY)

### Entry Point

- New users enter through `README.md`, `docs/guide/quickstart.md`, `docs/guide/README.md`, install docs, feature pages, and command reference pages.
- Release engineers enter through `docs/functional-tests.md`, `docs/contributing/documentation-testing.md`, Make targets, and generated code-to-doc reverse indexes.
- Documentation validation enters through `./le doc check verify`, `./le docvalid doc-drift`, `./le docvalid command-contract`, `./le consistency`, `./le docs-to-code index-check`, and local markdown link scans.

### Transformation Path

1. Docs state a factual claim, command, config, example output, scenario count, plugin inventory, or local link.
2. Source anchors, registries, Makefiles, and filesystem inventories should make the claim verifiable.
3. Documentation tools compare claims against live source, registry, and file inventory.
4. Users or release engineers follow the docs and either reach a working command or hit stale output, broken links, or misleading evidence.
5. Confirmed issues become audit findings with future verification requests.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Docs -> live registry/filesystem | `./le doc check verify` and `doc_drift.go` | RA-DOC-002, RA-DOC-004, RA-DOC-005 |
| Consistency docs -> code refs | `./le consistency` output | RA-DOC-008 |
| Gate docs -> live stage list | `docs/functional-tests.md` vs `stagesForMode` | RA-DOC-009 |
| Interop docs -> scenario tree | documented counts vs `test/interop/scenarios/` | RA-DOC-010 |

(Boundary rows solely about the removed findings RA-DOC-001/003/006/007 deleted 2026-07-10; the retained registry/filesystem row still names removed RA-DOC-004/005 for history.)

### Integration Points

- `spec-release-audit-2-bgp-protocol.md` owns protocol implications of stale interop and malformed-wire coverage gaps.
- `spec-release-audit-5-plugins-rib.md` owns plugin registry and shipped-plugin table correctness when future fixes touch plugin inventory.
- `spec-release-evidence-gate.md` owns future release-gate wiring if documentation checks become required release evidence.
- `docs/contributing/documentation-testing.md` owns the documented workflow for interpreting doc validation failures.

### Architectural Verification

- [ ] No bypassed layers: findings cite the user doc, source/tool evidence, and future validation path.
- [ ] No unintended coupling: suggested fixes keep docs, codegen, the native action tables under `internal/le/` parsing, and product code changes separate.
- [ ] No duplicated functionality: future fix should extend existing doc validation where possible rather than adding parallel ad hoc checks.
- [ ] Zero-copy preserved where applicable: not applicable to this docs audit.

## Docs Audit Matrix

| Surface | Current Evidence | Release Risk | Finding |
|---------|------------------|--------------|---------|
| Documentation drift target | `./le doc check verify` output | Release docs are known-stale before release | RA-DOC-002 |
| Consistency backlog | `./le consistency` output | Release engineers cannot use one consistency gate as clean docs signal | RA-DOC-008 |
| Release gate documentation | `docs/functional-tests.md`, `internal/le/verify/engine/run.go` | Documented ./le verify current mode full order omits five live gates | RA-DOC-009 |
| Interop inventory docs | interop docs, scenario tree | Scenario counts re-drifted across three docs | RA-DOC-010 |

(Matrix rows for the removed findings RA-DOC-001/003/004/005/006/007 deleted 2026-07-10; see Post-wave corrections.)

## Initial Findings

| ID | Severity | Surface | File/line | User Impact | Reproduction | Expected | Actual | Missing Test | Suggested Direction | Owner | Verification Requested |
|----|----------|---------|-----------|-------------|--------------|----------|--------|--------------|---------------------|-------|------------------------|
| RA-DOC-002 | Major | documentation gate and release evidence docs | `docs/contributing/documentation-testing.md`, `:42-44`; `docs/functional-tests.md`, `:1112-1124`; the retired `Makefile:180-184` (current producers: `internal/le/` native action tables); `internal/le/fuzz/actions.go`, `:14-70`; `internal/le/docvalid/drift.go`; `./le doc check verify` output | Release engineers cannot trust docs to be release-ready while the repository's own doc target fails and release-evidence docs omit or stale-check current targets | Run `./le doc check verify`; compare `docs/functional-tests.md` with the retired `Makefile` (current producers: `internal/le/` native action tables) and `internal/le/fuzz/actions.go` | Documentation drift target passes or reports only explicitly deferred issues, and release evidence docs match current verification and fuzz targets | `./le doc check verify` fails with 11 drift issues before command validation; command validation itself reports `All commands validated`; `docs/functional-tests.md` omits `ze-evidence-vet` from `./le verify current mode full` and says fuzz runs 15s each while `internal/le/fuzz/actions.go` uses 10s | No release blocker check currently requires a clean doc target before release; doc-test does not catch the `ze-evidence-vet` or fuzz-duration drift | Future fix should clear or route every doc-test issue, update release evidence docs against Makefiles, then decide whether release evidence requires `./le doc check verify` clean output | docs/tooling and release evidence | Passing `./le doc check verify`; source-backed `docs/functional-tests.md` updates for `ze-evidence-vet` and fuzz duration; if any issue is deferred, it is tied to an existing spec and not counted as clean release evidence |
| RA-DOC-008 | Minor | consistency and doc-ref backlog | `./le consistency` output; `internal/component/bgp/plugins/rib/storage/pathset.go`; `internal/component/bgp/plugins/cmd/cache`; `internal/component/bgp/plugins/cmd/commit` | Release engineers cannot treat `./le consistency` as a clean documentation consistency gate because doc-relevant errors are mixed into a large backlog | Run `./le consistency` | Consistency output is clean or split into actionable release-gate categories with doc failures visible | Command fails with 42 errors and 712 warnings, including stale refs to non-existent storage files and missing plugin command package docs/schema markers | No focused docs-consistency gate separates source-anchor/link/plugin-doc failures from broader code size and style backlog | Future fix should either clean doc-relevant consistency errors or split a narrower docs consistency target from broad code health checks | docs/tooling plus relevant subsystem owners | Passing focused docs consistency target, or `./le consistency` clean enough that docs failures are actionable |
| RA-DOC-009 | Major | release gate documentation | `docs/functional-tests.md`; `internal/le/verify/engine/run.go`; `internal/le/` native action tables | Release engineers reading the documented ./le verify current mode full order miss five live gates and cannot route their failures | Compare `docs/functional-tests.md` with `stagesForMode` (`internal/le/verify/engine/run.go`) | Docs list the live stage order | Docs omit `./le tier check`, `ze-iface-resolution-check`, `./le plugin boundary check`, `./le port-defaults check`, `ze-platform-vet`, which the live producer runs between ./le verify lint run and ./le doc wiring (`verify_run.go`); the dead `_ze-verify-impl` target (`internal/le/` native action tables, zero callers per `internal/le/` native action tables) additionally lists `./le cli-grammar`, absent from the live list | `doc_drift.go` does not compare the documented order sentence against `stagesForMode` | Future fix should update the order sentence from `stagesForMode` and consider deriving the check in `doc_drift.go` | docs/onboarding plus docs/tooling | `docs/functional-tests.md` ./le verify current mode full order matches `stagesForMode`; ideally a drift check guards it |
| RA-DOC-010 | Minor | interop inventory docs | `docs/features/interoperability-testing.md`; `docs/architecture/testing/interop.md`; `docs/DESIGN.md`; `test/interop/scenarios/` | Users and release engineers see three different scenario counts | Compare documented counts with the scenario directory count | All docs state the live count or a generated-list policy | Features doc says 96, interop architecture doc says 97, DESIGN.md says 101; the tree has 101 scenario directories (2026-07-10) | No inventory check keeps counts in sync (successor to removed RA-DOC-003) | Future fix should derive or validate scenario counts from the tree | docs/onboarding plus BGP protocol audit | Counts consistent across docs or generated from the tree |

## Wiring Test (MANDATORY)

This audit spec has no runtime product code. Its wiring test is that each documentation surface maps to a source path and future validation path.

| Entry Point | -> | Feature Code or Tool | Test |
|-------------|----|----------------------|------|
| Documentation drift target | -> | `internal/le/docvalid/drift.go` | `./le doc check verify`, currently failing RA-DOC-002 |
| ./le verify current mode full order documentation | -> | `internal/le/verify/engine/run.go` stagesForMode | Future drift check requested by RA-DOC-009 |
| Interop scenario counts | -> | `test/interop/scenarios/` | Future inventory check requested by RA-DOC-010 |

(Wiring rows for the removed findings RA-DOC-001/003/004/005/006/007 deleted 2026-07-10; N/A otherwise -- this audit ships no runtime code.)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Docs/onboarding audit starts | README, quickstart, guide index, feature docs, test docs, and docs tooling surfaces are mapped |
| AC-2 | Confirm a docs finding | Finding includes source/tool evidence, user or release impact, suggested future direction, and requested verification |
| AC-3 | Review first-user path | Audit checks build requirements, minimal config validation output, start/verify commands, and install docs for obvious source-backed drift |
| AC-4 | Review documentation validation | Audit runs documented doc validation targets and records failures without changing product code or docs |
| AC-5 | Review local docs navigation | Audit performs a local link scan or records why no link evidence exists |
| AC-6 | Review source-anchor tooling | Audit checks whether source-anchor validation is safe and aligned with the documented format |
| AC-7 | Keep audit-only scope | No production source, tests, schemas, docs, generated files, or Makefiles are modified by this spec |

## 🧪 TDD Test Plan

This docs/onboarding audit records evidence expected from future fix work. It does not add or change tests itself.

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|

(Suggested-test rows deleted 2026-07-10: their findings RA-DOC-005/006 are resolved; see Post-wave corrections.)

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Interop scenario count | Live scenario directory count | 101 (2026-07-10) | Any lower stale count | Any higher stale count |

(Boundary rows for the removed findings RA-DOC-001/006/007 deleted 2026-07-10; the scenario-count row is retained for RA-DOC-010 with the current count.)

### Functional Tests

N/A for this audit itself -- it adds no tests; `.ci` coverage obligations live in the fix specs routed from findings. (Rows for removed findings RA-DOC-001 and RA-DOC-007 deleted 2026-07-10; see Post-wave corrections.)

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Docs validation umbrella | `./le doc check verify` | Release engineer checks docs drift and command contract | Currently failing RA-DOC-002 |

### Interop Tests

This docs audit does not add protocol behavior. Interop evidence remains owned by `spec-release-audit-2-bgp-protocol.md`; docs fixes should validate that interop inventory documentation matches scenario names and counts.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Interop inventory docs check | `test/interop/scenarios/` | N/A | Docs state the live scenario count (101 directories, 2026-07-10) | Suggested for RA-DOC-010 (replaces removed RA-DOC-003 row) |

### Future

- Add local Markdown link validation to `./le doc check verify` or document a separate docs link gate.
- Add source-anchor validation tests before requiring `./le docs-to-code index-check` in release evidence.

(The docs-smoke-test bullet was deleted 2026-07-10 with its finding RA-DOC-001; see Post-wave corrections.)

## Files to Modify

This audit spec does not modify product files. Future fix work is expected to touch some of these files:
- `docs/features/interoperability-testing.md` - scenario count and list (RA-DOC-010).
- `docs/architecture/testing/interop.md` - scenario inventory (RA-DOC-010).
- `docs/functional-tests.md` - ./le verify current mode full order sentence (RA-DOC-009).
- `docs/contributing/documentation-testing.md` - update workflow if new doc checks are added.

(File bullets for the removed findings RA-DOC-001/003-partial/004/005/006/007 deleted 2026-07-10; see Post-wave corrections.)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | - |
| CLI commands/flags | No | - |
| CLI grammar | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |
| Doctor check for runtime dependencies | No | - |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | Future fixes should update exact docs named in findings |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | Yes | Future docs/tooling fixes should update `docs/contributing/documentation-testing.md` if validation targets change |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create

- `plan/spec-release-audit-8-docs-onboarding.md` - this audit spec.

## Implementation Steps

This audit spec has no implementation phase. Future fix specs should be created per finding or closely related finding group.

### Suggested Future Fix Grouping

| Group | Findings | Suggested Future Scope |
|-------|----------|------------------------|
| Documentation validation tooling | RA-DOC-002, RA-DOC-005, RA-DOC-006, RA-DOC-008 | Doc-test cleanup, the native action tables under `internal/le/` include parsing, source-anchor check mode, consistency split |
| Release gate and inventory docs | RA-DOC-009, RA-DOC-010 | ./le verify current mode full order sentence from stagesForMode; interop scenario counts from the tree |

(Group rows whose findings were all removed on 2026-07-10 were deleted; the tooling row retains removed RA-DOC-005/006 for history, with RA-DOC-002/008 still live.)

### Failure Routing

| Failure | Route To |
|---------|----------|
| `./le doc check verify` fails | RA-DOC-002 owner until every issue is fixed or explicitly routed |
| ./le verify current mode full order docs drift | RA-DOC-009 docs/tooling future fix |
| Interop inventory drift | RA-DOC-010 and BGP protocol audit |

(Routing rows for the removed findings RA-DOC-001/003/004/006/007 deleted 2026-07-10; see Post-wave corrections.)

## Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Findings cover README, quickstart, docs validation, interop docs, plugin docs, source anchors, and links |
| Correctness | Every finding cites exact source lines or command output from this audit |
| Naming | Findings use stable `RA-DOC-NNN` IDs |
| Data flow | Each docs claim maps to a user/release entry point and validation path |
| Scope | Audit spec does not fix docs or tooling directly |
| Rule: no fabrication | Findings do not infer status without line or command evidence |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Docs/onboarding child audit exists | `plan/spec-release-audit-8-docs-onboarding.md` present |
| Findings use umbrella schema | Inspect Initial Findings table |
| Audit remains docs-only | `git diff -- plan/spec-release-audit-8-docs-onboarding.md` shows only spec creation |
| No em dash in spec | Search generated spec for the em dash character |
| Spec status visible | `./le spec status` lists `release-audit-8-docs-onboarding` |

## Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Unsafe docs commands | Quickstart and install docs should not encourage credential leakage or destructive setup without warning |
| Source-anchor trust | Source anchors should point to real files and not silently false-positive as stale paths |
| Link safety | Local Markdown link checker should reject paths outside the repository unless explicitly allowed |
| Release evidence integrity | Docs validation must not mutate generated files during a read-only check |

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

<!-- Added 2026-07-10: this audit spec predates the validator's required-section list
     (`internal/le/hookruntime/lifecycle.go`). Audit specs produce documentation, not code;
     the TDD items bind the future fix work routed from findings, not this spec. -->

### Goal Gates (MUST pass)

- [ ] Every docs surface mapped with evidence (AC-1)
- [ ] Findings carry source/tool evidence and requested verification (AC-2)
- [ ] `./le verify worktree` evidence requested from future fix work where findings demand it

### TDD (applies to future fix specs routed from findings)

- [ ] Tests written (in the owning fix spec)
- [ ] Tests FAIL (paste output) (in the owning fix spec)
- [ ] Tests PASS (paste output) (in the owning fix spec)

### Post-wave corrections (2026-07-10)

USER APPROVAL (2026-07-10): findings resolved by the followup implementation wave are REMOVED from this spec rather than struck. Each removal is recorded below with re-verified evidence; the matrix, boundary, wiring, TDD, files-to-modify, grouping, and routing rows belonging to those findings were removed in the same pass.

Removed findings:

- Removed 2026-07-10 per user instruction: resolved by followup wave -- RA-DOC-001: `README.md` now says "Go 1.26+", matching `go.mod:3`; `docs/guide/quickstart.md` says Go 1.26+ and its expected output "configuration valid: example.conf" (`quickstart.md`) matches the producer `internal/component/config/cli/cmd_validate.go` (the validate command moved there from `cmd/ze/config/cmd_validate.go`).
- Removed 2026-07-10 per user instruction: resolved by followup wave -- RA-DOC-003 (as filed): no doc says 32 or 33 scenarios any more; `docs/DESIGN.md` says "101 interop scenarios" (the tree has 101 scenario directories); `docs/architecture/testing/interop.md` lists `bfd-frr` in its core table and its coverage text names BFD failover (`interop.md`). Residual count drift re-filed as RA-DOC-010 below.
- Removed 2026-07-10 per user instruction: resolved by followup wave -- RA-DOC-004: all nine plugins the finding named (`bgp-filter-aspath-length`, `bgp-filter-remove-private-as`, `dhcpserver`, `flowspec-firewall`, `ike`, `imageserver`, `kernel`, `routing-table`, `tftpserver`) now appear in `docs/DESIGN.md`, and the doc-drift stage of `./le doc check verify` reports "No documentation drift detected" (run 2026-07-10).
- Removed 2026-07-10 per user instruction: resolved by followup wave -- RA-DOC-005: `functionalGateSuites` (`internal/le/docvalid/drift.go`) now follows `include`/`-include`/`sinclude` directives (`doc_drift.go`, `:221`); the "could not derive ./le functional suites" error no longer appears in the 2026-07-10 `./le doc check verify` run.
- Removed 2026-07-10 per user instruction: resolved by followup wave -- RA-DOC-006: check mode is read-only (`internal/le/docstocode/codetodocs.go` writes `ai/CODE-TO-DOCS.md` only in non-check mode) and the description separator accepts `--`, single hyphen, and the long dash (`code_to_docs.py`); the 2026-07-10 run reports "checked 1366 code paths, 429 packages ... all references valid".
- Removed 2026-07-10 per user instruction: resolved by followup wave -- RA-DOC-007: all 8 cited broken links now resolve (`docs/guide/benchmarking.md`, `docs/guide/config-archive.md`, `docs/guide/looking-glass.md`, `docs/guide/healthcheck.md`, `docs/guide/rpki.md`, `docs/guide/web-interface.md` exist for the `docs/features/*` links; `docs/guide/mcp/overview.md` -> `ai/rules/protocol.md` resolves; `docs/architecture/config/environment.md` links at `:115`/`:126` resolve). A local Markdown link checker is still absent from `./le doc check verify`, so the Future item for it is retained.

Retained findings, status re-verified 2026-07-10:

- RA-DOC-002 (retained, narrowed): the drift checker now passes ("No documentation drift detected"); `docs/functional-tests.md` includes `ze-evidence-vet` in the documented ./le verify current mode full order; fuzz docs match the the native action tables under `internal/le/` (54 targets at `functional-tests.md`, "10s each" at `:1503`; `internal/le/fuzz/actions.go` uses 10s per target). `./le doc check verify` still exits non-zero, but only on the discovery-index freshness stage: `ai/DOCS-TO-CODE.md` and `ai/LEARNED-FULL-INDEX.md` are stale against the wave (fix: `./le discovery-index update`). The finding's remaining substance is that index regeneration plus RA-DOC-009 below.
- RA-DOC-008 (retained, still reproducible): the retired `ze-consistency-check` (current: `./le consistency`) re-run 2026-07-10 fails with 84 errors and 1021 warnings (was 42/712; the backlog grew). The cited stale refs persist (`internal/component/bgp/plugins/rib/storage/pathset.go` -> `familyrib_bart.go`, `:3` -> `familyrib_map.go`, both missing), as do the `cmd/cache`/`cmd/commit` missing-schema errors.

Fresh instances (added as rows in Initial Findings above):

- RA-DOC-009: the documented `./le verify current mode full` order (`docs/functional-tests.md`) omits five gates the live producer runs: `stagesForMode` (`internal/le/verify/engine/run.go`, default branch `:135-148`) executes `./le tier check`, `ze-iface-resolution-check`, `./le plugin boundary check`, `./le port-defaults check`, `ze-platform-vet` between ./le verify lint run and ./le doc wiring. The dead `_ze-verify-impl` target (`internal/le/` native action tables; zero callers per the comment at `internal/le/` native action tables) additionally lists `./le cli-grammar`, which is not in the live stage list.
- RA-DOC-010: interop scenario counts re-drifted: `docs/features/interoperability-testing.md` says 96, `docs/architecture/testing/interop.md` says 97 scenario directories, `docs/DESIGN.md` says 101, and the tree has 101 scenario directories (2026-07-10). Successor to the removed RA-DOC-003.

Housekeeping from this correction pass: the `## TDD Test Plan` heading was renamed to `## 🧪 TDD Test Plan` and this `## Checklist` section added to satisfy the blocking spec validator; neither edit changed audit content.
