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
4. `ai/rules/documentation.md` - source anchors and doc validation rules
5. `docs/contributing/documentation-testing.md` - doc validation workflow
6. `docs/guide/quickstart.md` - first-user onboarding path
7. `scripts/docvalid/doc_drift.go` - documentation drift checker
8. `scripts/dev/code_to_docs.py` - source anchor reverse index checker

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
- [ ] `ai/rules/documentation.md` - source anchors and validation targets
  -> Decision: audit factual docs against source and tooling rather than memory
  -> Constraint: every factual docs claim should be source-backed, and `make ze-doc-test` is the documented validation target for docs changes
- [ ] `ai/rules/no-fabrication.md` - line-backed factual claims
  -> Constraint: findings cite exact source or tool output, and avoid inferred status
- [ ] `ai/rules/bash-output.md` - no pipes for build/test/verify commands
  -> Constraint: validation commands were run cleanly, without piping their output
- [ ] `docs/contributing/documentation-testing.md` - documentation drift and command validator workflow
  -> Decision: `make ze-doc-test`, `make ze-doc-drift`, `make ze-validate-commands`, `make ze-consistency`, and `make ze-doc-check-stale` are separate evidence tools
  -> Constraint: `make ze-doc-test` is not part of `make ze-verify` today, so docs drift can exist outside the default gate

### Source and Documentation Files

- [ ] `README.md` - release landing page, quickstart, Go version, test-count claims, docs links
  -> Decision: first release readiness needs README claims to match live source and current guide docs
- [ ] `docs/guide/README.md` - user guide index and feature routing
  -> Decision: guide index is an onboarding map and must not link users to stale feature paths
- [ ] `docs/guide/quickstart.md` - build, init, minimal config, validate, start, verify path
  -> Decision: quickstart must be executable from a clean checkout with current command output
- [ ] `docs/guide/ze-install.md` - local install, systemd, remote provisioning onboarding path
  -> Decision: install docs belong in the first-user docs surface even when not tested by `ze-doc-test`
- [ ] `docs/functional-tests.md` - release gate and evidence documentation
  -> Decision: release engineers depend on this file for what is and is not gated
- [ ] `docs/DESIGN.md` - shipped plugin table, test philosophy, interop count
  -> Decision: design doc feature inventory must match registered plugins and live interop scenario count
- [ ] `docs/features/interoperability-testing.md` - public interop feature page
  -> Constraint: scenario count and list must match `test/interop/scenarios/`
- [ ] `docs/architecture/testing/interop.md` - interop architecture and scenario inventory
  -> Constraint: negative or not-covered lists must be checked against current scenario tree
- [ ] `Makefile` and `mk/test-functional.mk` - split Makefile include and functional gate suites
  -> Decision: documentation tooling must follow included Makefiles or derive suites through `make`, not only the root file body
- [ ] `mk/test-fuzz.mk` - current fuzz duration and target list
  -> Constraint: fuzz documentation must not stale relative to `ze-fuzz-test`
- [ ] `scripts/docvalid/doc_drift.go` - docs drift checker implementation
  -> Decision: doc validation is itself a release surface because it decides whether docs are trustworthy
- [ ] `scripts/dev/code_to_docs.py` - source-anchor reverse index and stale-reference check
  -> Decision: source-anchor validation must be safe to run in audit mode and agree with documented anchor format

**Key insights:**
- The first-user quickstart and README already diverge on the Go version and validation output.
- `make ze-doc-test` currently fails, so documentation drift is confirmed by project tooling.
- Interop documentation is stale in multiple places: one file says 32 scenarios, one says 33, while the live tree and doc-test report 37.
- Documentation validation has tooling gaps: the Makefile split broke functional-suite discovery, and `ze-doc-check-stale` writes the generated index before reporting stale references.
- Local markdown links are not covered by the existing doc validation target.

## Current Behavior (MANDATORY)

**Source files and docs read:**
- [ ] `README.md` - says Go 1.25+ is required at line 86, while the project module and guide now require Go 1.26+
- [ ] `go.mod` - declares `go 1.26` at line 3
- [ ] `docs/guide/quickstart.md` - says Go 1.26+ at line 13, and expects `configuration valid (1 peer, 1 plugin)` at lines 93-97
- [ ] `cmd/ze/config/cmd_validate.go` - current text output prints `configuration valid: <path>` at lines 569-571, and peer/plugin counts are verbose-only at lines 572-586
- [ ] `docs/contributing/documentation-testing.md` - says `make ze-doc-test` runs all documentation tests at lines 8-14 and is not part of `make ze-verify` at lines 42-44
- [ ] `scripts/docvalid/doc_drift.go` - checks docs against registry, filesystem, Makefile, README, features, and functional docs at lines 73-90
- [ ] `scripts/docvalid/doc_drift.go` - derives functional suites by reading only `Makefile` at lines 95-127
- [ ] `Makefile` - includes split Makefile modules at lines 50-59, including `mk/test-functional.mk`
- [ ] `mk/test-functional.mk` - owns `ze-functional-test` and lists 12 gated suites at lines 40-70
- [ ] `docs/functional-tests.md` - claims the functional target runs 12 suites at lines 17-21, and documents fuzz count/duration at lines 1112-1140
- [ ] `mk/test-fuzz.mk` - `ze-fuzz-test` uses 10s fuzz time at lines 4 and 14-70
- [ ] `docs/DESIGN.md` - shipped plugin table starts at lines 297-356 and interop count says 33 at lines 731-735
- [ ] `docs/features/interoperability-testing.md` - says there are 32 scenarios at lines 10-14 and lists only 01 through 32 at lines 17-52
- [ ] `docs/architecture/testing/interop.md` - lists only scenarios 01 through 32 at lines 99-135 and says BFD is not covered at lines 240-244
- [ ] `test/interop/scenarios/*/check.py` - live scenario tree includes `33-bfd-frr`, `34-ecmp-frr`, `35-srv6-frr`, `36-remove-private-as-frr`, and `37-remove-private-as-as4path-frr`
- [ ] `scripts/dev/code_to_docs.py` - source-anchor parser strips descriptions only after `--` or a long dash at lines 21-24 and 40-42
- [ ] `scripts/dev/code_to_docs.py` - `--check` still writes `ai/CODE-TO-DOCS.md` before reporting stale references at lines 162-181
- [ ] `docs/architecture/decisions/001-pull-model-metrics.md` - uses single-hyphen source-anchor separators at lines 17-65 and 120-147
- [ ] `docs/architecture/config/environment.md`, `docs/features/*.md`, and `docs/guide/mcp/overview.md` - contain local markdown links that do not resolve from their current directories

**Validation commands run:**
- `make ze-doc-test` failed with 11 documentation drift issues, then reported command validation as `All commands validated` with 11 local handlers outside YANG.
- `make ze-consistency` failed with 42 errors and 712 warnings; doc-relevant output includes stale source refs in `internal/component/bgp/plugins/rib/storage/pathset.go:2-3` and missing plugin command package documentation/schema markers.
- `make ze-doc-check-stale` failed after writing `ai/CODE-TO-DOCS.md`; the generated-file change was restored immediately after the command.
- A read-only local markdown link scan found 8 broken local links under `docs/`.

**Behavior to preserve:**
- Documentation remains source-backed and should not rely on memory.
- `make ze-doc-test` remains the main documentation drift and command-contract target.
- `make ze-verify` remains the fast code gate, while docs-specific checks stay explicit until a separate release-gate design changes that policy.
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
- Documentation validation enters through `make ze-doc-test`, `make ze-doc-drift`, `make ze-validate-commands`, `make ze-consistency`, `make ze-doc-check-stale`, and local markdown link scans.

### Transformation Path

1. Docs state a factual claim, command, config, example output, scenario count, plugin inventory, or local link.
2. Source anchors, registries, Makefiles, and filesystem inventories should make the claim verifiable.
3. Documentation tools compare claims against live source, registry, and file inventory.
4. Users or release engineers follow the docs and either reach a working command or hit stale output, broken links, or misleading evidence.
5. Confirmed issues become audit findings with future verification requests.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| README -> build requirements | README Go version vs `go.mod` and guide | RA-DOC-001 |
| Quickstart -> CLI output | `docs/guide/quickstart.md` expected output vs `cmd_validate.go` | RA-DOC-001 |
| Docs -> live registry/filesystem | `make ze-doc-test` and `doc_drift.go` | RA-DOC-002, RA-DOC-004, RA-DOC-005 |
| Interop docs -> scenario tree | docs scenario lists vs `test/interop/scenarios/*/check.py` | RA-DOC-003 |
| Source anchors -> reverse index | `code_to_docs.py --check` and anchor format | RA-DOC-006 |
| Markdown links -> file paths | read-only local link scan | RA-DOC-007 |
| Consistency docs -> code refs | `make ze-consistency` output | RA-DOC-008 |

### Integration Points

- `spec-release-audit-2-bgp-protocol.md` owns protocol implications of stale interop and malformed-wire coverage gaps.
- `spec-release-audit-5-plugins-rib.md` owns plugin registry and shipped-plugin table correctness when future fixes touch plugin inventory.
- `spec-release-evidence-gate.md` owns future release-gate wiring if documentation checks become required release evidence.
- `docs/contributing/documentation-testing.md` owns the documented workflow for interpreting doc validation failures.

### Architectural Verification

- [ ] No bypassed layers: findings cite the user doc, source/tool evidence, and future validation path.
- [ ] No unintended coupling: suggested fixes keep docs, codegen, Makefile parsing, and product code changes separate.
- [ ] No duplicated functionality: future fix should extend existing doc validation where possible rather than adding parallel ad hoc checks.
- [ ] Zero-copy preserved where applicable: not applicable to this docs audit.

## Docs Audit Matrix

| Surface | Current Evidence | Release Risk | Finding |
|---------|------------------|--------------|---------|
| First-user README and quickstart | `README.md`, `go.mod`, `quickstart.md`, `cmd_validate.go` | Clean-checkout instructions can fail or teach stale output | RA-DOC-001 |
| Documentation drift target | `make ze-doc-test` output | Release docs are known-stale before release | RA-DOC-002 |
| Interop inventory docs | `docs/DESIGN.md`, interop docs, scenario tree | Protocol evidence is under-reported and inconsistent | RA-DOC-003 |
| Shipped plugin docs | `docs/DESIGN.md`, plugin registry via doc-test | Feature inventory omits registered production plugins | RA-DOC-004 |
| Functional-suite drift checker | `doc_drift.go`, `Makefile`, `mk/test-functional.mk` | Doc validation reports tool failure instead of actual suite drift | RA-DOC-005 |
| Source-anchor reverse index | `code_to_docs.py`, source anchors | Check mode mutates files and reports false stale refs | RA-DOC-006 |
| Local markdown links | read-only link scan, docs lines | Users hit broken internal docs navigation | RA-DOC-007 |
| Consistency backlog | `make ze-consistency` output | Release engineers cannot use one consistency gate as clean docs signal | RA-DOC-008 |

## Initial Findings

| ID | Severity | Surface | File/line | User Impact | Reproduction | Expected | Actual | Missing Test | Suggested Direction | Owner | Verification Requested |
|----|----------|---------|-----------|-------------|--------------|----------|--------|--------------|---------------------|-------|------------------------|
| RA-DOC-001 | Major | first-user onboarding | `README.md:86`; `go.mod:3`; `docs/guide/quickstart.md:13`, `:93-97`; `cmd/ze/config/cmd_validate.go:569-586` | New users can start with the wrong Go requirement or distrust the quickstart when the expected validation output does not match the binary | Compare README and quickstart against `go.mod` and `outputValidateText()` | README, quickstart, and command output examples match current source | README says Go 1.25+, `go.mod` and quickstart require Go 1.26+; quickstart expects `configuration valid (1 peer, 1 plugin)`, while current non-verbose output is `configuration valid: <path>` | No first-run doc test extracts quickstart commands and verifies expected output against the current binary | Future fix should update README and quickstart, then add a first-run docs smoke test or doctest-style validation for core commands and output snippets | docs/onboarding | Passing quickstart validation from a clean checkout or scripted docs smoke test, plus `make ze-doc-test` remains clean |
| RA-DOC-002 | Major | documentation gate and release evidence docs | `docs/contributing/documentation-testing.md:8-14`, `:42-44`; `docs/functional-tests.md:17-21`, `:1112-1124`; `Makefile:180-184`; `mk/test-fuzz.mk:4`, `:14-70`; `scripts/docvalid/doc_drift.go:73-90`; `make ze-doc-test` output | Release engineers cannot trust docs to be release-ready while the repository's own doc target fails and release-evidence docs omit or stale-check current targets | Run `make ze-doc-test`; compare `docs/functional-tests.md` with `Makefile` and `mk/test-fuzz.mk` | Documentation drift target passes or reports only explicitly deferred issues, and release evidence docs match current verification and fuzz targets | `make ze-doc-test` fails with 11 drift issues before command validation; command validation itself reports `All commands validated`; `docs/functional-tests.md` omits `ze-vet-evidence` from `ze-verify` and says fuzz runs 15s each while `mk/test-fuzz.mk` uses 10s | No release blocker check currently requires a clean doc target before release; doc-test does not catch the `ze-vet-evidence` or fuzz-duration drift | Future fix should clear or route every doc-test issue, update release evidence docs against Makefiles, then decide whether release evidence requires `make ze-doc-test` clean output | docs/tooling and release evidence | Passing `make ze-doc-test`; source-backed `docs/functional-tests.md` updates for `ze-vet-evidence` and fuzz duration; if any issue is deferred, it is tied to an existing spec and not counted as clean release evidence |
| RA-DOC-003 | Minor | interop inventory docs | `docs/DESIGN.md:731-735`; `docs/features/interoperability-testing.md:10-14`, `:17-52`; `docs/architecture/testing/interop.md:99-135`, `:240-244`; `test/interop/scenarios/*/check.py` | Users and release engineers see inconsistent protocol evidence, including a claim that BFD is not covered despite a BFD interop scenario | Compare documented scenario counts/lists with live scenario tree and `make ze-doc-test` output | Interop docs list the current 37 scenarios and accurately state covered and not-covered protocol areas | Docs say 32 or 33 scenarios, list only through 32, and one doc says BFD is not covered while `33-bfd-frr` exists | No inventory check currently keeps all interop docs in sync, only selected claims fail doc-test | Future fix should derive or validate interop scenario lists from the scenario tree and update not-covered text | docs/onboarding plus BGP protocol audit | Passing inventory check for all interop docs, with `33` through `37` represented or an explicit generated-list policy |
| RA-DOC-004 | Major | shipped plugin docs | `docs/DESIGN.md:297-356`; `make ze-doc-test` output | Feature inventory omits registered production plugins, so users cannot discover shipped capability from docs | Run `make ze-doc-test` and inspect the shipped plugin table | Every registered production plugin appears in the shipped plugin table or is intentionally hidden with source-backed reason | Doc-test reports missing table entries for `bgp-filter-aspath-length`, `bgp-filter-remove-private-as`, `dhcpserver`, `flowspec-firewall`, `ike`, `imageserver`, `kernel`, `routing-table`, and `tftpserver` | Shipped plugin table is not generated, and current doc-test failure is not release-clean | Future fix should reconcile `docs/DESIGN.md` with the registry or generate the table from the registry with stable descriptions | docs/onboarding and plugins/RIB | Passing `make ze-doc-test`; plugin table contains all registered plugins or a validated hidden/internal classification |
| RA-DOC-005 | Major | documentation drift tooling | `scripts/docvalid/doc_drift.go:95-127`; `Makefile:50-59`; `mk/test-functional.mk:40-70`; `docs/functional-tests.md:17-21` | The docs drift checker cannot verify the functional gate after Makefile modularization, so it reports a tool failure instead of checking actual docs drift | Run `make ze-doc-test` or inspect `functionalGateSuites()` | Drift checker derives `ze-functional-test` suites from included Makefiles or another live source | `functionalGateSuites()` reads only root `Makefile`, while `ze-functional-test` lives in `mk/test-functional.mk`; doc-test reports `could not derive ze-functional-test suites from Makefile` | No unit test covers `doc_drift.go` Makefile include parsing or suite derivation after the Makefile split | Future fix should teach the checker to follow included makefiles or use a generated inventory source, then add a regression test for the split target | docs/tooling | `make ze-doc-test` no longer emits the derivation error and can detect real suite list drift |
| RA-DOC-006 | Major | source anchor tooling | `scripts/dev/code_to_docs.py:21-24`, `:40-42`, `:162-181`; `docs/architecture/decisions/001-pull-model-metrics.md:17-65`, `:120-147`; `docs/architecture/decisions/001-pull-model-metrics.md` anchor style | Source-anchor validation is unsafe for audit mode and can report false stale references, reducing trust in documentation source links | Run `make ze-doc-check-stale` | A check target reports stale source anchors without modifying generated files and parses the anchor format actually used in docs | `--check` writes `ai/CODE-TO-DOCS.md` before failing, and the parser strips descriptions only after `--` or a long-dash separator, while many anchors use a single hyphen separator | No test proves `--check` is read-only; no parser test covers the source-anchor separator style used by docs | Future fix should make check mode read-only, align the documented and accepted separator formats, and add parser tests for valid anchor styles | docs/tooling | `make ze-doc-check-stale` leaves `git diff -- ai/CODE-TO-DOCS.md` empty and reports only true missing paths |
| RA-DOC-007 | Minor | local markdown links | `docs/architecture/config/environment.md:99`; `docs/features/benchmarking.md:22`; `docs/features/cli-commands.md:25`; `docs/features/looking-glass.md:23`; `docs/features/plugins.md:12`, `:45`; `docs/features/web-interface.md:35`; `docs/guide/mcp/overview.md:127` | Users following internal docs navigation hit missing pages from rendered Markdown or repository browsing | Run the read-only local markdown link scan from this audit | Every relative Markdown link under `README.md` and `docs/` resolves inside the repository or is explicitly external | 8 local links are broken, mostly `docs/features/*` links missing `../` and one guide page link pointing to `../../ai/rules/...` instead of the repository root path | No link checker is part of `make ze-doc-test` today | Future fix should correct the broken links and add a local Markdown link checker to doc validation | docs/onboarding | Link checker passes for README and docs; `make ze-doc-test` includes or documents the link check |
| RA-DOC-008 | Minor | consistency and doc-ref backlog | `make ze-consistency` output; `internal/component/bgp/plugins/rib/storage/pathset.go:2-3`; `internal/component/bgp/plugins/cmd/cache`; `internal/component/bgp/plugins/cmd/commit` | Release engineers cannot treat `make ze-consistency` as a clean documentation consistency gate because doc-relevant errors are mixed into a large backlog | Run `make ze-consistency` | Consistency output is clean or split into actionable release-gate categories with doc failures visible | Command fails with 42 errors and 712 warnings, including stale refs to non-existent storage files and missing plugin command package docs/schema markers | No focused docs-consistency gate separates source-anchor/link/plugin-doc failures from broader code size and style backlog | Future fix should either clean doc-relevant consistency errors or split a narrower docs consistency target from broad code health checks | docs/tooling plus relevant subsystem owners | Passing focused docs consistency target, or `make ze-consistency` clean enough that docs failures are actionable |

## Wiring Test (MANDATORY)

This audit spec has no runtime product code. Its wiring test is that each documentation surface maps to a source path and future validation path.

| Entry Point | -> | Feature Code or Tool | Test |
|-------------|----|----------------------|------|
| README Go version | -> | `go.mod` | Future docs smoke test requested by RA-DOC-001 |
| Quickstart validation output | -> | `cmd/ze/config/cmd_validate.go` | Future docs smoke test requested by RA-DOC-001 |
| Documentation drift target | -> | `scripts/docvalid/doc_drift.go` | `make ze-doc-test`, currently failing RA-DOC-002 |
| Interop scenario docs | -> | `test/interop/scenarios/` | Future inventory check requested by RA-DOC-003 |
| Shipped plugin table | -> | plugin registry via `internal/component/plugin/all` | `make ze-doc-test`, currently failing RA-DOC-004 |
| Functional-suite documentation | -> | `mk/test-functional.mk` | Future checker regression requested by RA-DOC-005 |
| Source-anchor reverse index | -> | `scripts/dev/code_to_docs.py` | Future read-only check requested by RA-DOC-006 |
| Local Markdown links | -> | docs filesystem | Future link checker requested by RA-DOC-007 |

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

## TDD Test Plan

This docs/onboarding audit records evidence expected from future fix work. It does not add or change tests itself.

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| Doc drift Makefile include parsing | `scripts/docvalid/doc_drift_test.go` or equivalent | `ze-functional-test` suites are derived after Makefile split | Suggested for RA-DOC-005 |
| Source anchor parser formats | `scripts/dev/code_to_docs` test harness | Accepted separators match documented source-anchor format | Suggested for RA-DOC-006 |
| Read-only source-anchor check mode | `scripts/dev/code_to_docs` test harness | Check mode does not write `ai/CODE-TO-DOCS.md` | Suggested for RA-DOC-006 |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Go version claim | Matches `go.mod` major.minor | `1.26` | `1.25` when module says `1.26` | Future unsupported version |
| Interop scenario count | Live scenario directory count | 37 | Any lower stale count | Any higher stale count |
| Markdown relative link | Existing repository path | Existing file/dir | Missing path | Outside workspace unless explicitly allowed |
| Source-anchor check mode | No worktree diff | empty diff | generated file modified | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Quickstart smoke | new docs smoke suite or `test/install` | User follows build/init/config validate/start path from docs | Suggested for RA-DOC-001 |
| Docs validation umbrella | `make ze-doc-test` | Release engineer checks docs drift and command contract | Currently failing RA-DOC-002 |
| Markdown link checker | new doc validation target | User navigation inside README and docs resolves | Suggested for RA-DOC-007 |

### Interop Tests

This docs audit does not add protocol behavior. Interop evidence remains owned by `spec-release-audit-2-bgp-protocol.md`; docs fixes should validate that interop inventory documentation matches scenario names and counts.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Interop inventory docs check | `test/interop/scenarios/` | N/A | Docs list current scenario tree, including 33 through 37 | Suggested for RA-DOC-003 |

### Future

- Add a docs smoke test that extracts or mirrors quickstart commands and verifies source-backed expected output.
- Add local Markdown link validation to `make ze-doc-test` or document a separate docs link gate.
- Add source-anchor validation tests before requiring `make ze-doc-check-stale` in release evidence.

## Files to Modify

This audit spec does not modify product files. Future fix work is expected to touch some of these files:
- `README.md` - Go version and quickstart alignment.
- `docs/guide/quickstart.md` - expected output and first-run flow.
- `docs/DESIGN.md` - shipped plugin table and interop count.
- `docs/features/interoperability-testing.md` - scenario count and list.
- `docs/architecture/testing/interop.md` - scenario inventory and not-covered list.
- `docs/features/*.md`, `docs/architecture/config/environment.md`, `docs/guide/mcp/overview.md` - broken local links.
- `scripts/docvalid/doc_drift.go` - Makefile include-aware functional-suite derivation.
- `scripts/dev/code_to_docs.py` - read-only check mode and source-anchor separator parsing.
- `docs/contributing/documentation-testing.md` - update workflow if new doc checks are added.

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
| First-user docs | RA-DOC-001, RA-DOC-007 | README, quickstart, local links, docs smoke test |
| Interop and plugin inventory docs | RA-DOC-003, RA-DOC-004 | Scenario inventory, shipped plugin table, derived or checked inventories |
| Documentation validation tooling | RA-DOC-002, RA-DOC-005, RA-DOC-006, RA-DOC-008 | Doc-test cleanup, Makefile include parsing, source-anchor check mode, consistency split |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Docs smoke test fails | Update docs or source-backed expected output in first-user docs future spec |
| `make ze-doc-test` fails | RA-DOC-002 owner until every issue is fixed or explicitly routed |
| Interop inventory drift | RA-DOC-003 and BGP protocol audit |
| Plugin table drift | RA-DOC-004 and plugin/RIB audit |
| Source-anchor check modifies files | RA-DOC-006 tooling future fix |
| Link checker finds missing local target | RA-DOC-007 first-user docs future fix |

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
| Spec status visible | `make ze-spec-status` lists `release-audit-8-docs-onboarding` |

## Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Unsafe docs commands | Quickstart and install docs should not encourage credential leakage or destructive setup without warning |
| Source-anchor trust | Source anchors should point to real files and not silently false-positive as stale paths |
| Link safety | Local Markdown link checker should reject paths outside the repository unless explicitly allowed |
| Release evidence integrity | Docs validation must not mutate generated files during a read-only check |
