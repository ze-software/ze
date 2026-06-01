# Spec: verify-debugging-protocol

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/7 |
| Updated | 2026-06-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `ai/rules/planning.md` - spec lifecycle and completion rules.
3. `docs/functional-tests.md` - current `ze-verify` stage list and documented log workflow.
4. `docs/architecture/testing/ci-format.md` - current functional runner model and debug surfaces.
5. `Makefile` - `ze-verify`, `ze-verify-changed`, `ze-lint`, `ze-vet-evidence`, `ze-exabgp-test`.
6. `mk/test-unit.mk` - unit test two-pass strategy and changed-group race pass.
7. `mk/test-functional.mk` - functional suite list and suite-level continue-on-failure behavior.
8. `cmd/ze-test/ci_runner.go` - non-BGP `.ci` suite wiring.
9. `internal/test/runner/report.go` - detailed failure reports and current rerun commands.
10. `internal/test/runner/display.go` - suite headers, summaries, and rerun hints.
11. `test/exabgp-compat/bin/functional` - ExaBGP live status and summary output.

## Task

Establish a verify debugging protocol for `make ze-verify` and every directly invoked stage tool. The protocol must let an AI or human read one compact failure index first, decide which failures are related, rerun the right scope, and open full logs only for the chosen group.

The protocol must satisfy four constraints:

| Constraint | Meaning |
|-----------|---------|
| Minimal first read | The first artifact contains only the information needed to route and reproduce failures. |
| Conservative grouping | Related failures stay together inside a stage, unrelated failures are split, and the first version never merges across stage boundaries. |
| Exact reproducers | Every failure group points to an exact rerun command for the smallest useful scope. |
| Full evidence preserved | Compact summaries never replace the full stage log. They point to it. |

This work stays as one spec. Splitting the orchestration layer from the stage emitters would create an unstable intermediate protocol where the top level wrapper expects structured group information that the runners do not yet produce. The first cut must land the top level artifact contract and the largest stage emitters together.

## Required Reading

### Architecture Docs and Rules
- [ ] `docs/functional-tests.md` - current public workflow for `ze-verify`, functional suites, and expected log files.
  -> Decision: preserve the current `ze-verify` stage set and stage order as the public contract.
  -> Constraint: existing documented log names remain valid, or the doc update must change every workflow that points at them.
- [ ] `docs/architecture/testing/ci-format.md` - `.ci` runner data model, test nick identity, and current debug modes.
  -> Decision: use suite label, test nick, CI file path, failure type, and expected/received evidence as the canonical functional failure identity.
  -> Constraint: existing single-test rerun and `--server` / `--client` debug modes remain usable.
- [ ] `docs/architecture/debugging/plugin-testing.md` - current expectations for concrete debugging guidance.
  -> Decision: prefer exact reproducers and exact artifact paths over generic advice.
  -> Constraint: when the protocol suggests a next step, it must be a command that actually reproduces the failure scope.
- [ ] `ai/rules/discovery-updates.md` - discoverability requirements for new tooling and verification flow.
  -> Decision: the protocol rollout updates docs, AI rules, and index entries in the same change.
  -> Constraint: any new log artifact or workflow must be documented where future agents look first.
- [ ] `ai/rules/testing.md` - testing expectations for infrastructure changes.
  -> Decision: verification uses fixture-driven mixed-failure tests and end-to-end runner behavior, not parser-only unit tests.
  -> Constraint: tests assert visible grouping, rerun hints, and artifact paths, not internal helper names.

### RFC Summaries
- [ ] Not applicable - this spec changes internal verification and debugging output, not wire protocol behavior.
  -> Constraint: no RFC summaries or interop protocol tests are required for the orchestration layer itself.

**Key insights:**
- `ze-verify` is documented as a single pre-commit gate, but the documented log capture workflow is not actually wired.
- The functional runner already has the richest structured failure data in memory. It should emit native group summaries instead of forcing the top level protocol to scrape its own text.
- Stage boundary is the safest first routing unit. False aggregation across stages is more dangerous than asking for one extra agent.
- The current biggest debugging bugs are not missing data, but missing routing: fail-fast at the top level, wrong rerun commands for non-BGP suites, missing suite headers, and no compact index.
- Compact summaries must cap repeated detail. Full logs stay on disk and remain the authority.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `Makefile` - `ze-verify` is a dependency chain under `verify-lock.sh`; `ZE_VERIFY_LOG` exists as a variable but is not used.
- [ ] `mk/test-unit.mk` - unit tests run as a cacheable full pass plus a changed-group race pass; both use raw `go test` text output.
- [ ] `mk/test-functional.mk` - functional suites continue across suite failures and already print suite-level rerun commands.
- [ ] `scripts/dev/verify-lock.sh` - lock ownership, wait/break behavior, and duration tracking are implemented here.
- [ ] `scripts/dev/verify-summary.sh` - intended failure index script exists but is not wired and does not match `TEST FAILURE:` blocks.
- [ ] `scripts/dev/verify-status.sh` - documented freshness file helper exists, but current verify flow does not write it.
- [ ] `scripts/dev/changed-groups.sh` - changed `.go` files are mapped to race-test package groups.
- [ ] `scripts/dev/verify_wiring_docs.py` - selects changed-file-aware wiring, docs, command, inventory, and plugin-import checks, then shells out to the chosen subchecks.
- [ ] `mk/inventory.mk` - `ze-verify-wiring-docs`, `ze-doc-test`, `ze-validate-commands`, `ze-doc-check-stale`, and inventory targets.
- [ ] `cmd/ze-test/bgp.go` - BGP suite runner prints suite headers and uses `runner.Report` plus `runner.Display`.
- [ ] `cmd/ze-test/ci_runner.go` - non-BGP `.ci` suites reuse `EncodingTests` and `Runner`, but do not print suite headers before running.
- [ ] `cmd/ze-test/editor.go` - editor suite uses the generic `ParallelRunner` and only prints per-failure detail in verbose mode.
- [ ] `internal/test/runner/runner.go` - encode, plugin, reload, and top-level `.ci` suites print full failure reports only after the stage completes.
- [ ] `internal/test/runner/report.go` - detailed failure block is strong, but debug commands are hard-coded to `ze-test bgp <suite>`.
- [ ] `internal/test/runner/display.go` - suite summaries are compact and parseable, but rerun hints also hard-code BGP commands except for `editor`.
- [ ] `internal/test/runner/parallel.go` - parse, decode, and editor suites summarize counts, but concise per-failure lines only appear in verbose mode.
- [ ] `internal/test/runner/decoding.go` - decode failures have useful diff detail, but only with verbose failure callbacks.
- [ ] `internal/test/runner/parsing.go` - parse failures print file path only in verbose mode.
- [ ] `scripts/docvalid/commands.go` - command validator prints a readable failure summary, then a full inventory table even on failure.
- [ ] `scripts/docvalid/doc_drift.go` - drift checker prints issue lines only and exits non-zero.
- [ ] `scripts/codegen/plugin_imports.go` - plugin import check reports stale/current state with a single line.
- [ ] `test/exabgp-compat/bin/functional` - ExaBGP compat runner uses carriage-return live status, prints a summary, and points to `./qa/bin/functional` debug commands.
- [ ] `failure.txt` - observed mixed plugin and reload failures from a real run.
- [ ] `tmp/baseline-fail.log` - observed raw multi-package `go test` failure output.
- [ ] `tmp/133.log` - observed wrong rerun command for `ze-test ui`.
- [ ] `tmp/exabgp-verify.log` - observed carriage-return status artifacts in a saved verify log.

### Current execution tree

| Stage | Current entry | Current output contract | AI-friendliness gap |
|------|---------------|-------------------------|---------------------|
| Verify lock | `scripts/dev/verify-lock.sh` | Owner wait banner, previous duration, hard lock around verify-class runs | Good lock context, but no artifact directory or stage manifest handoff |
| Lint | `Makefile` -> `golangci-lint run` | Raw file:line diagnostics | No compact stage envelope, no grouped rerun scope |
| Wiring/docs | `mk/inventory.mk` -> `verify_wiring_docs.py` -> subchecks | Router prints `Running <target>...`, then raw subcheck output | No compact per-subcheck failure blocks, no aggregated rerun guidance |
| Vet evidence | `Makefile` -> `GOOS=linux go vet ./scripts/evidence/...` | Raw `go vet` text | No stage envelope, but usually already compact |
| Unit cached | `mk/test-unit.mk` -> raw `go test` | Standard `go test` package and assertion lines | No package-level failure index, no rerun commands, top level verify stops here today |
| Unit race changed | `mk/test-unit.mk` + `changed-groups.sh` | Prints changed package patterns, then raw `go test -race` text | Good package scope selection, but no grouped failures inside the stage |
| Functional BGP suites | `cmd/ze-test/bgp.go` + `internal/test/runner/*` | Strong suite header, summary, per-test detailed failure block, rerun hints | No compact group index before full detail, no per-group parallelization hints |
| Functional top-level `.ci` suites | `cmd/ze-test/ci_runner.go` + `Runner` | Same runner as BGP suites, but without explicit suite header | Saved logs can misattribute failures to the previous suite |
| Parse / decode / editor | `ParallelRunner`-based suites | Summary and rerun hints, detailed output only in verbose mode | Default verify failure often lacks enough local detail |
| ExaBGP compat | `test/exabgp-compat/bin/functional` | Live status with carriage returns, failure detail, debug hints, final summary | Saved logs contain status-line artifacts, commands point at `./qa/bin/functional` instead of the Ze invocation path |
| Verify status / summary | `verify-status.sh`, `verify-summary.sh` | Intended freshness and failure index helpers | Both are operationally incomplete in the current flow |

### Observed behavior from real logs

| Observed file | Observation | Why it matters |
|--------------|-------------|----------------|
| `failure.txt` | Plugin failures cluster naturally by suite, failure type, and test-name prefix, for example BFD timeouts, FIB observer failures, RIB graph failures, role OTC timeouts, and L2TP show/teardown timeouts | The grouping information is already present, but the runner does not print it as a compact routing index |
| `failure.txt` | `ui` failures appear after the `reload` section without their own suite header | Line-oriented readers can route UI failures to the wrong subsystem |
| `tmp/133.log` | `ze-test ui` failure suggests `ze-test bgp ui 133` and BGP decode hints | Non-BGP suites currently emit incorrect reproducers |
| `tmp/baseline-fail.log` | Multi-package `go test` failures show package names and assertions, but no package summary block | An agent must scan the full log to decide package boundaries |
| `tmp/exabgp-verify.log` | Live status updates collapse into a single line with carriage-return artifacts | Saved logs are harder to read and harder to parse |
| `docs/functional-tests.md` and `ai/rules/git-safety.md` | Both tell readers to open `tmp/ze-verify-failures.log` first | The protocol must make that file real and trustworthy |

**Behavior to preserve:**
- The current `ze-verify` stage order: lint, wiring/docs, evidence vet, cacheable unit pass, changed-group race pass, functional suites, ExaBGP compat.
- The current `ze-verify` failure semantics: any failed stage returns a non-zero final exit code.
- The current `ze-unit-test-cached` plus `ze-unit-test-race-changed` strategy.
- The current `ze-functional-test` suite list and suite-level continue-on-failure behavior.
- The current detailed `TEST FAILURE` blocks for encode, plugin, reload, and top-level `.ci` suites, including CI file path, failure type, expected/received evidence, and likely-cause hints.
- Existing `ze-test` single-test rerun by nick and `--server` / `--client` debug modes.
- Existing timing baselines in `tmp/test-timings.json`.
- Existing changed-file-aware selection in `ze-verify-wiring-docs`.
- Existing `verify-lock.sh` ownership and wait behavior.

**Behavior to change:**
- `make ze-verify` must always write a real compact failure index and per-stage full logs.
- The top level verify driver must continue across top-level stages, collect every failing stage, and summarize them at the end.
- Non-BGP `.ci` suites must print their own suite header and correct rerun commands.
- Parse, decode, and editor failures must emit enough default detail to debug without re-running in verbose mode just to learn the basic error.
- ExaBGP verify-mode logs must be newline-safe in saved logs.
- The first routing unit is a stage. Within a stage, the protocol groups related failures conservatively. The first version does not merge groups across stage boundaries.

## Data Flow (MANDATORY)

### Entry Point
- Entry point: `make ze-verify` and `make ze-verify-changed`.
- Input format: ordered stage commands plus their stdout, stderr, exit codes, and any native failure metadata they emit.
- Output format: full stage logs, compact failure index, machine-readable failure index, and the existing status fingerprint file.

### Transformation Path
1. `Makefile` invokes `scripts/dev/verify-lock.sh` with the verify label.
2. The lock wrapper launches one verify protocol runner with `ZE_VERIFY_MODE=1` and a run artifact directory under `tmp/verify/`.
3. The verify runner executes each stage in fixed order, mirrors output to the console, and writes a per-stage full log.
4. When a stage passes, the runner records stage metadata only. When a stage fails, the runner gathers a compact stage summary:
   - native failure groups for `ze-test` suites,
   - package and test groups for unit stages,
   - subcheck groups for wiring/docs,
   - line-based groups for lint and vet,
   - verify-mode summary groups for ExaBGP.
5. The runner appends compact stage/group blocks to `tmp/ze-verify-failures.log` and the same information to `tmp/ze-verify-failures.json`.
6. After all stages finish, the runner prints a final grouped summary, writes `tmp/ze-verify.status`, and exits non-zero if any stage failed.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Make -> lock wrapper | `scripts/dev/verify-lock.sh` remains the single verify-class lock entry | [ ] |
| Lock wrapper -> verify runner | one command invocation with label and artifact directory | [ ] |
| Verify runner -> stage command | `exec.Command`-style subprocess execution with streamed output and per-stage logs | [ ] |
| `ze-test` runner -> verify protocol | native failure groups exported from in-memory `Record` data | [ ] |
| External tool text -> verify protocol | parser/classifier reads raw stage log and emits groups | [ ] |
| Verify protocol -> docs/workflow | log file names and routing workflow updated in docs and AI rules | [ ] |

### Integration Points
- `Makefile` remains the public entry point.
- `verify-lock.sh` remains the lock authority.
- `cmd/ze-test/bgp.go`, `ci_runner.go`, `editor.go`, and `internal/test/runner/*` must expose suite label, rerun scope, and grouped failure metadata consistently.
- `mk/test-unit.mk` and the unit-test invocations must expose package boundaries cleanly to the protocol.
- `verify_wiring_docs.py` must expose failing subchecks as individual groups.
- `test/exabgp-compat/bin/functional` must expose a verify-mode summary that survives saved logs.
- `verify-status.sh` must stay aligned with the actual verify runner output.

### Protocol Contract

#### Artifact set

| Artifact | Producer | Purpose |
|----------|----------|---------|
| `tmp/ze-verify.log` | verify runner | concatenated full verify log, stage ordered |
| `tmp/verify/<nn>-<stage>.log` | verify runner | full log for one stage |
| `tmp/ze-verify-failures.log` | verify runner | compact stage and failure-group index, first file to read |
| `tmp/ze-verify-failures.json` | verify runner | machine-readable failure routing index |
| `tmp/ze-verify.status` | verify runner | freshness fingerprint written on both pass and fail |

#### Required compact fields

| Field | Meaning |
|-------|---------|
| Stage | failing verify stage or functional suite name |
| Group ID | stable stage-local group key |
| Kind | failure class, for example `package`, `lint`, `timeout`, `mismatch`, `subcheck`, `build`, `stderr-mismatch` |
| Related tests | test ids, package names, or subcheck names included in the group |
| Summary | shortest useful statement of the common failure signal |
| Rerun | smallest useful reproducer command |
| Detail log | exact full log path |
| Parallel | `stage`, `group`, or `manual`, depending on whether the group is safe to hand to a separate agent immediately |

#### Grouping rules

| Stage | Group key | Rerun scope | Parallelization rule |
|------|-----------|-------------|----------------------|
| `ze-lint` | nearest package directory plus linter name | `golangci-lint run <pkg>` or exact file set when narrower | each group is independent |
| `ze-vet-evidence` | package path | `GOOS=linux go vet <pkg>` | each package group is independent |
| `ze-unit-test-cached` | Go package, with compile/setup failures collapsed per package | `go test <pkg>` plus `-run` when one named test fails | package groups are independent |
| `ze-unit-test-race-changed` | Go package pattern returned by changed-group mapping | `go test -race <pkg>` | package groups are independent |
| `ze-verify-wiring-docs` | selected subcheck target name | `make <target>` or direct script command | subchecks are independent unless the same target reports multiple issues |
| `ze-functional-test` BGP suites | suite label, failure type, subsystem prefix from test name or explicit plugin signal | `ze-test bgp <suite> <nick...>` | groups inside one suite are independent; stage boundary stays primary |
| `ze-functional-test` top-level `.ci` suites | suite label, failure type, and test-name prefix | `ze-test <suite> <nick...>` | groups inside one suite are independent |
| parse / decode / editor | suite label plus test file or test name | suite-specific command for one test or one pattern | each group is independent |
| `ze-exabgp-test` | scenario type plus failure kind | `uv run --with ... ./test/exabgp-compat/bin/functional <scenario> <nick>` | each group is independent |

#### Detail caps

| Field | Value | Reason |
|-------|-------|--------|
| Full-detail samples per group | 1 | keep the compact index short and route readers to the full log for the rest |
| Group member ids shown inline | 8 | enough to see the cluster without flooding the index |
| Relevant log lines copied into compact summary | 20 | enough to identify the failure family without copying the full log |

### Architectural Verification
- [ ] No bypassed layers: the protocol wraps existing stage commands instead of reimplementing their test logic.
- [ ] No unintended coupling: stage grouping is stage-local first; no cross-stage semantic merger in the first cut.
- [ ] No duplicated functionality: `ze-test` groups come from existing `Record` data where possible, not from scraping its own rendered text.
- [ ] Low-copy output path: compact summaries reference stage logs and copy only capped excerpts.

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `make ze-verify` | -> | verify protocol runner orchestration and artifact writing | `TestVerifyRunWritesArtifactsAndContinuesAfterStageFailures` |
| `make ze-verify` | -> | final compact failure index | `TestVerifyRunProducesStageAndGroupSummaries` |
| `ze-test ui --all` | -> | non-BGP suite header and rerun command path | `TestCISubcommandPrintsHeaderAndTopLevelRerunHints` |
| `ze-test bgp plugin --all` | -> | functional native failure grouping | `TestFunctionalFailureGroupsUseSuiteTypeAndSubsystemPrefix` |
| `uv run ... functional encoding` with `ZE_VERIFY_MODE=1` | -> | ExaBGP newline-safe verify summary | `TestExabgpVerifyModeSummaryUsesNewlinesAndExactReproducers` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-verify` runs and at least one stage fails | The run still executes all top-level stages, then exits non-zero with a final stage-and-group summary. |
| AC-2 | `make ze-verify` finishes, pass or fail | `tmp/ze-verify.log`, per-stage logs under `tmp/verify/`, and `tmp/ze-verify.status` exist and describe that run. |
| AC-3 | `ze-unit-test-cached` fails in multiple packages | The compact failure index shows one group per failing package with exact rerun commands. |
| AC-4 | `ze-functional-test` fails in multiple unrelated suites | The compact failure index splits failures by suite before any inner grouping. |
| AC-5 | A functional suite fails with a cluster of related plugin tests, for example multiple BFD, FIB, or RIB cases | The compact failure index emits one related group with member ids and one group rerun command instead of one ungrouped block per test. |
| AC-6 | A non-BGP `.ci` suite fails, for example `ui`, `managed`, `web`, `firewall`, `policy`, `l2tp`, or `install` | The failure block and rerun hints use `ze-test <suite> ...`, not `ze-test bgp <suite> ...`, and the suite has its own header. |
| AC-7 | A parse, decode, or editor test fails under default verify mode | The stage log includes enough detail to identify the failing file or mismatch without re-running only to learn the basic error. |
| AC-8 | `ze-verify-wiring-docs` fails in one selected subcheck | The compact failure index names the failing subcheck target and its exact rerun command. |
| AC-9 | ExaBGP compat fails under verify mode | The saved log uses newline-delimited status and summary output, and its debug commands point at the actual Ze repo path. |
| AC-10 | A failure group has many repeated members | The compact failure index caps repeated detail, shows a bounded member list, and points to the full stage log for complete evidence. |
| AC-11 | All stages pass | `make ze-verify` preserves a green summary, writes a fresh status file, and exits zero. |
| AC-12 | `tmp/ze-verify-failures.log` is opened first | A reader can decide which failures can be debugged in parallel without opening `tmp/ze-verify.log`. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifyRunWritesArtifactsAndContinuesAfterStageFailures` | `scripts/status/verify_run_test.go` | AC-1 and AC-2 top-level orchestration, artifact paths, non-zero exit on failure | planned |
| `TestVerifyRunProducesStageAndGroupSummaries` | `scripts/status/verify_run_test.go` | AC-1, AC-3, AC-4, AC-8 final compact index shape | planned |
| `TestVerifyRunCapsInlineMembersAndExcerptLines` | `scripts/status/verify_run_test.go` | AC-10 group member and excerpt caps | planned |
| `TestGoTestFailuresGroupByPackage` | `scripts/status/verify_run_test.go` | AC-3 package grouping for cached and race unit stages | planned |
| `TestWiringDocFailuresGroupBySubcheck` | `scripts/status/verify_run_test.go` | AC-8 wiring/docs subcheck routing | planned |
| `TestCISubcommandPrintsHeaderAndTopLevelRerunHints` | `cmd/ze-test/ci_runner_test.go` | AC-6 correct header and rerun commands for non-BGP suites | planned |
| `TestDisplayDebugHintsUseSuiteSpecificCommands` | `internal/test/runner/display_test.go` | AC-6 suite-specific rerun hints | planned |
| `TestReportDebugCommandsUseSuiteSpecificCommands` | `internal/test/runner/report_test.go` | AC-6 detailed failure block rerun commands | planned |
| `TestFunctionalFailureGroupsUseSuiteTypeAndSubsystemPrefix` | `internal/test/runner/failure_group_test.go` | AC-5 conservative functional grouping by suite, failure type, and subsystem prefix | planned |
| `TestParallelRunnerFailureLinesAppearWithoutVerboseWhenVerifyMode` | `internal/test/runner/parallel_test.go` | AC-7 default detail for parse, decode, and editor failures | planned |
| `TestExabgpVerifyModeSummaryUsesNewlinesAndExactReproducers` | `scripts/status/verify_run_test.go` | AC-9 newline-safe ExaBGP summary and repo-root reproducer commands | planned |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Inline member ids shown per group | 1-8 displayed | 8 | 0 | 9 means fold into `+N more` text |
| Relevant log lines copied into compact summary | 1-20 displayed | 20 | 0 | 21 means truncate and point at full log |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestVerifyRunMixedFailureFixture` | `scripts/status/verify_run_test.go` | Developer runs `make ze-verify`, sees multiple failing stages grouped into a compact routing index | planned |
| `TestVerifyRunAllPassFixture` | `scripts/status/verify_run_test.go` | Developer runs `make ze-verify`, sees pass summary and fresh status artifact | planned |
| `TestVerifyRunFunctionalFixtureWithRelatedPluginFailures` | `scripts/status/verify_run_test.go` | Developer sees one grouped plugin cluster instead of dozens of isolated blocks | planned |

### Interop Tests (MANDATORY for protocol features)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Not applicable | - | - | This spec changes verification output, not a wire protocol | N/A |

### Future (if deferring any tests)
- No test deferrals are planned. If the first implementation skips native manifests for a stage and falls back to text parsing, that decision must still ship with fixture coverage for the fallback path.

## Files to Modify

- `Makefile` - route `ze-verify` and `ze-verify-changed` through the protocol runner instead of a fail-fast dependency chain.
- `mk/test-unit.mk` - expose unit-stage commands in a form the protocol runner can execute and classify cleanly.
- `mk/test-functional.mk` - keep suite ordering aligned with the new runner and pass verify-mode environment through stage invocations.
- `scripts/dev/verify-status.sh` - keep freshness file handling aligned with the real verify runner outputs.
- `scripts/dev/verify_wiring_docs.py` - expose stable per-subcheck failure routing and rerun guidance.
- `cmd/ze-test/ci_runner.go` - add non-BGP suite headers and correct suite-aware rerun command context.
- `cmd/ze-test/editor.go` - align default failure detail and rerun hints with the protocol.
- `internal/test/runner/runner.go` - export or hand off grouped failure metadata for encode, plugin, reload, and top-level `.ci` suites.
- `internal/test/runner/report.go` - correct suite-specific debug commands and emit bounded group-aware detail.
- `internal/test/runner/display.go` - fix suite-specific rerun hints and print group-aware failure indices.
- `internal/test/runner/parallel.go` - make concise failure lines available in verify mode without requiring `-v`.
- `internal/test/runner/decoding.go` - preserve JSON diff detail while surfacing a concise default failure line.
- `internal/test/runner/parsing.go` - preserve file-path detail in default verify-mode failures.
- `test/exabgp-compat/bin/functional` - add newline-safe verify mode summary and exact repo-root reproducer commands.
- `docs/functional-tests.md` - document the real artifact set and the “read failures first” workflow.
- `docs/architecture/testing/ci-format.md` - document verify-mode debug surfaces emitted by the `.ci` runner.
- `ai/rules/git-safety.md` - keep the prescribed verify triage workflow aligned with the actual artifacts.
- `ai/INDEX.md` - point future sessions at the new protocol artifacts and workflow.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | - |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | No | - |
| CLI grammar (action before identifier) | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |
| Pipe completeness | No | - |
| Env var registration | No | - |
| Doctor check for runtime dependencies | No | - |
| Prometheus counters/metrics | No | - |
| Test infrastructure changed | Yes | `Makefile`, `mk/test-unit.mk`, `mk/test-functional.mk`, `docs/functional-tests.md` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/functional-tests.md` |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/functional-tests.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/ci-format.md` or a new testing-architecture note if the protocol becomes large enough |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | - |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | `docs/`, `ai/rules/git-safety.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/functional-tests.md`, `ai/rules/git-safety.md` |

## Files to Create

- `scripts/status/verify_run.go` - top-level verify protocol runner, artifact writer, and final summary emitter.
- `scripts/status/verify_run_test.go` - fixture-driven orchestration, grouping, cap, and artifact tests.
- `scripts/status/testdata/verify/` - saved stage logs and expected group summaries for lint, go test, functional, wiring/docs, and ExaBGP cases.
- `internal/test/runner/failure_group.go` - native functional failure grouping for suite-local clusters.
- `internal/test/runner/failure_group_test.go` - grouping behavior tests for plugin, reload, and top-level `.ci` suites.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Critical Review Checklist |
| 6. Full verification | Deliverables Checklist and final verify commands |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | Relevant implementation phase |
| 9. Re-verify | Full verification commands |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | Full verification commands |
| 14. Present summary | Executive Summary per planning rule |

### Implementation Phases

1. **Phase: Wiring** - create the top-level verify protocol runner and route `ze-verify` through it
   - Tests: `TestVerifyRunWritesArtifactsAndContinuesAfterStageFailures`, `TestVerifyRunProducesStageAndGroupSummaries`
   - Files: `Makefile`, `scripts/status/verify_run.go`, `scripts/status/verify_run_test.go`, `scripts/dev/verify-status.sh`
   - Verify: `make ze-verify` writes the new artifact set, continues across synthetic stage failures, and still exits non-zero when any stage fails
2. **Phase: Native functional grouping** - fix suite headers and rerun commands, add suite-local failure groups
   - Tests: `TestCISubcommandPrintsHeaderAndTopLevelRerunHints`, `TestDisplayDebugHintsUseSuiteSpecificCommands`, `TestReportDebugCommandsUseSuiteSpecificCommands`, `TestFunctionalFailureGroupsUseSuiteTypeAndSubsystemPrefix`
   - Files: `cmd/ze-test/ci_runner.go`, `cmd/ze-test/editor.go`, `internal/test/runner/runner.go`, `report.go`, `display.go`, `parallel.go`, `failure_group.go`, `failure_group_test.go`
   - Verify: `ui`, `managed`, `web`, `firewall`, `policy`, `l2tp`, and `install` all emit the correct suite header and suite-specific reproducers; functional failures appear as groups in the compact index
3. **Phase: Unit, lint, and wiring-doc classifiers** - add compact grouping for stages that still emit text
   - Tests: `TestGoTestFailuresGroupByPackage`, `TestWiringDocFailuresGroupBySubcheck`, `TestVerifyRunCapsInlineMembersAndExcerptLines`
   - Files: `scripts/status/verify_run.go`, `mk/test-unit.mk`, `scripts/dev/verify_wiring_docs.py`, `Makefile`
   - Verify: package, linter, and subcheck boundaries are visible in `tmp/ze-verify-failures.log`
4. **Phase: ExaBGP verify mode** - make saved ExaBGP logs readable and routeable
   - Tests: `TestExabgpVerifyModeSummaryUsesNewlinesAndExactReproducers`
   - Files: `test/exabgp-compat/bin/functional`, `scripts/status/verify_run_test.go`
   - Verify: saved ExaBGP stage logs have no carriage-return artifacts and expose exact rerun commands
5. **Phase: Documentation and discovery updates** - update the public workflow and AI entry points
   - Tests: documentation and index verification through `make ze-verify-wiring-docs`
   - Files: `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md`, `ai/rules/git-safety.md`, `ai/INDEX.md`
   - Verify: docs match the actual artifact set and direct readers to the compact failure index first
6. **Full verification** - run `make ze-verify` and stage-focused tests used by this work
7. **Complete spec** - fill audit sections, write learned summary, and close the spec when implementation is done

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has a visible output artifact or summary block, not only helper code |
| Correctness | Stage grouping never mixes different stage names in one group |
| Conservative routing | Functional grouping keeps related plugin clusters together but does not merge different suites or different stages |
| Reproducers | Every group rerun command is executable from repo root and matches the smallest useful scope |
| Output minimization | Compact summary never copies full logs; it respects the configured sample and line caps |
| Header correctness | Every suite in `ze-functional-test`, including non-BGP suites, prints an explicit suite header |
| Wrong-command regression | No `ui`, `managed`, `web`, `firewall`, `policy`, `l2tp`, or `install` failure suggests `ze-test bgp <suite>` |
| ExaBGP log safety | Saved verify logs use newline-delimited progress/summary output |
| Discovery updates | Docs, AI rules, and index entries point to the new artifact workflow |
| Rule: no-layering | The verify runner orchestrates stages only; it does not reimplement the tests themselves |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `scripts/status/verify_run.go` exists and is wired from `Makefile` | `read Makefile` and `read scripts/status/verify_run.go` |
| `tmp/ze-verify-failures.log` is written by a real run | run `make ze-verify` with at least one failing fixture or real failure |
| `tmp/ze-verify-failures.json` exists and matches the log index | run the fixture tests and inspect the JSON artifact |
| Non-BGP suite headers exist | run `ze-test ui --all` or fixture harness and inspect stage log |
| Non-BGP rerun commands are correct | unit tests in `ci_runner_test.go`, `display_test.go`, `report_test.go` |
| Functional native grouping exists | `failure_group_test.go` and fixture-driven verify run test |
| ExaBGP verify-mode log is newline-safe | ExaBGP fixture test |
| Docs point to the compact failure index first | inspect `docs/functional-tests.md` and `ai/rules/git-safety.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Artifact path safety | All protocol artifacts stay under `tmp/` and do not accept arbitrary path traversal from environment variables or test names |
| Shell injection | Group rerun commands are assembled from known stage names, package paths, and test ids with safe quoting or direct argument vectors |
| Secret minimization | Compact summaries copy only capped relevant lines and point to full logs instead of duplicating large client output blocks |
| Log parser robustness | Text parsers handle missing markers, truncated logs, and mixed stdout/stderr without panicking or misrouting unrelated failures |
| Status file integrity | `tmp/ze-verify.status` reflects the actual run result and cannot report fresh success after a failing run |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Top-level verify runner does not execute stages in order | Phase 1 |
| Suite header or rerun commands wrong for non-BGP suites | Phase 2 |
| Functional grouping merges unrelated tests | Phase 2 |
| Unit, lint, or wiring-doc failures collapse into one unhelpful block | Phase 3 |
| ExaBGP saved log still contains carriage-return artifacts | Phase 4 |
| Docs still point to nonexistent artifacts | Phase 5 |
| Test fails for the wrong reason | Fix test setup or fixture, then re-run the relevant phase |
| Three failed fix attempts on the same grouping bug | Stop, record the attempted grouping heuristics, and ask the user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The documented `tmp/ze-verify.log` and `tmp/ze-verify-failures.log` workflow was already wired | The variable and helper scripts exist, but the verify chain does not produce those artifacts | Source review of `Makefile`, `verify-summary.sh`, `verify-status.sh`, and `tmp/` contents | The spec must include real artifact wiring, not only formatting changes |
| Non-BGP `.ci` suites inherited correct rerun commands from the shared runner | `report.go` and `display.go` hard-code `ze-test bgp <suite>` | Source review and `tmp/133.log` | Non-BGP suite debugging is currently misdirected |
| Saved ExaBGP verify logs would be readable because the runner prints a final summary | Carriage-return status updates still pollute saved logs before the summary | `tmp/exabgp-verify.log` | Verify mode must switch ExaBGP logging to newline-safe output |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Reuse `verify-summary.sh` as the main protocol implementation | It is not wired, is too weak for `TEST FAILURE:` blocks, and shell parsing is too limited for stage-local grouping | Add a dedicated tested verify runner and keep shell wrappers thin |
| Group across stage boundaries in the first version | A compile or setup error can cascade and would create false aggregation | Group by stage first, then by conservative stage-local heuristics |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Docs name debugging artifacts that are not actually produced | Seen in `docs/functional-tests.md` and `ai/rules/git-safety.md` | Verification workflow docs must have an executable source path and a test or fixture proving the artifact exists | Update docs and consider a future doc-anchor check |

## Design Insights
- Stage boundary is the first routing boundary. The first protocol version never merges across stages.
- Prefer native failure manifests where a runner already owns structured state. Only parse rendered text for external tools and simple one-line stages.
- Verify mode is an environment-controlled rendering mode. It adds machine-readable artifacts and log-safe rendering without making normal interactive runs verbose or awkward.
- The compact failure index is a routing artifact, not a replacement for the full stage log.

## Core Insight
The main missing feature is not more diagnostics. It is a stable routing layer between existing diagnostics and the person or agent reading them. The protocol should therefore spend most of its complexity budget on stage boundaries, group identity, exact reproducers, and bounded summaries, not on inventing new per-test detail.