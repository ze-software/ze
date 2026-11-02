# Spec: lint-backlog-4-plugin-infra-and-core

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | tooling |
| Priority | P2 |
| Release reason | 14 findings of the full lint in the plugin infrastructure, the public SDK and statestore; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 2/3 |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in the plugin
infrastructure and the public SDK (door `ai/doors/plugin-infra.md`) and in
`internal/core/statestore` (door `ai/doors/core.md`). By linter: errcheck 10,
unparam 1, modernize 1, gocritic 1, misspell 1. All but one are in test files.
Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/component/plugin/ipc/filter_batch_test.go` | 24 | unparam |
| `internal/component/plugin/ipc/tls_test.go` | 455 | errcheck |
| `internal/component/plugin/process/process_stop_test.go` | 87 | errcheck |
| `internal/component/plugin/process/recovery_delivery_integration_test.go` | 242 | errcheck |
| `internal/component/plugin/registry/filter_declaration.go` | 47 | modernize |
| `internal/component/plugin/server/local_bridge_test.go` | 121 | gocritic |
| `internal/component/plugin/server/startup_chain_refused_test.go` | 94 | misspell |
| `internal/component/plugin/sysproc_linux_test.go` | 54 | errcheck |
| `internal/core/statestore/statestore_reader_test.go` | 95 | errcheck |
| `pkg/plugin/sdk/sdk_callbacks_test.go` | 67 | errcheck |
| `pkg/plugin/sdk/sdk_callbacks_test.go` | 100 | errcheck |
| `pkg/plugin/sdk/sdk_callbacks_test.go` | 124 | errcheck |
| `pkg/plugin/sdk/sdk_callbacks_test.go` | 151 | errcheck |
| `pkg/plugin/sdk/sdk_callbacks_test.go` | 182 | errcheck |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/plugin-infra.md` - the SDK, the registry and the process protocol
  → Constraint: `pkg/plugin/sdk` is a published contract; only its tests change here, and no exported name moves
  → Constraint: a test that mutates the registry takes `registry.Snapshot()` and restores it
- [ ] `ai/doors/core.md` - statestore
  → Constraint: `statestore.Put` answers `false, nil` with no store; a test checking an error checks the acknowledging call
- [ ] `docs/contributing/ze-go-style.md` - "Every error is handled"
  → Constraint: an ignored error in a test is checked or carries `//nolint:errcheck // <reason>`

**Key insights:**
- The only non-test finding is a modernize rewrite in `registry/filter_declaration.go`; it must not change the Stage 1 declaration a plugin sends.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/filter_declaration.go` - the modernize site
- [ ] `pkg/plugin/sdk/sdk_callbacks_test.go` - five ignored errors
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- The process protocol, the SDK's exported surface, every test's assertion.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| Style rule table | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |
| Scoped plugin tests | `./le job run label plugin command go test -race ./internal/component/plugin/... ./pkg/plugin/...` (plugin-infra door, Gates) | Runs the plugin packages under race | Applies: AC-2 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/component/plugin/... ./pkg/plugin/... ./internal/core/statestore/..."`

### Transformation Path
1. Read each finding; check the error, or annotate with the reason it cannot matter.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Test and one source edit | No |

### Integration Points
- None beyond the packages listed.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | Source audit against the saved Task list; parent reserves CPU jobs | broken: eleven listed sites already addressed; three remained |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Checking a previously ignored error makes a test fail on a teardown race | A flaky red in the changed test | Read why the call can fail; assert only what the test owns, annotate the rest with its reason |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the three trees | Zero findings |
| AC-2 | The scoped plugin and statestore tests under `-race` | Pass |
| AC-3 | Any `//nolint` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | the changed packages | AC-2 | |

## Files to Modify
- The files in the Task table.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, CLI, env, doctor, metrics, BGP family surface | N-A | Lint fixes; no surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-15, 17 | Feature, config, CLI, API, plugin, wire, SDK, RFC, test infrastructure, comparison, architecture, metadata, metrics, inventory, examples | No | Lint fixes change no documented behavior |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-4-plugin-infra-and-core.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. Fix the test findings, then the registry modernize.
3. Scoped lint zero; tests pass.

### Step 2 source audit and handoff (2026-11-02)

The parent reserves all tests, lint and compilation until the parallel edits
settle. The saved CI list above is the baseline; no baseline was repeated.
Step 2 has written edits, with AC-1 and AC-2 still unverified.

| Finding | Current source evidence | Step 2 action |
|---------|-------------------------|---------------|
| unparam | `ipc/filter_batch_test.go` `sendQueued`: all six callers pass zero for the queue offset | Removed the constant parameter and updated all six callers, including `filter_batch_metrics_test.go` |
| modernize | `registry/filter_declaration.go` `DeclaredFilter.ReadsRaw`: a string membership loop | Replaced the loop with `slices.Contains`; nil, empty and matching lists keep their result |
| misspell | `server/startup_chain_refused_test.go` `TestStartupKeepsThePolicyForARefusedDeclaration`: fixture says colour | Changed the fixture to color; refusal assertions stay identical |
| gocritic | `server/local_bridge_test.go` `TestDeclarationsConfigRefusedReadOnly` already uses `strings.Fields` | No edit |
| TLS errcheck | `ipc/tls_test.go` `TestAuthenticateWrongMethod` already checks the request write | No edit |
| Process-stop errcheck | `process/process_stop_test.go` `TestStopReachesThePluginTheShellStarted` already checks `proc.Wait` | No edit |
| Recovery errcheck | `process/recovery_delivery_integration_test.go` `recoveryDeliveryScenario` already annotates unused pipe cleanup and joins the command result | No edit |
| Child-process errcheck | `sysproc_linux_test.go` `TestStartChildSurvivesForkingThreadExit` already checks `KillProcessGroup` | No edit |
| Store errcheck | `statestore_reader_test.go` `TestReadDoesNotWaitForAMutation` already captures and checks the joined increment error | No edit |
| Five SDK errcheck sites | `sdk_callbacks_test.go` `executeCommand` checks `CallAnswer`, collapse and terminator errors for the five listed callback tests | No edit |

Diagnosis: the remaining findings come from a constant helper argument, a
manual membership loop and a fixture spelling. The owning layers are the test
helper, registry predicate and test fixture. A [workaround] suppression would
leave each reported form intact; the [source] edits remove those forms without
changing assertions, declarations or protocol bytes. No product defect or new
behavior was found that needs a new red-first test. No suppression was added
(AC-3).

Read documentation: `docs/architecture/api/process-protocol.md` (filter
declarations and shutdown), `docs/guide/bgp-policy.md` (plugin filter chains),
`docs/architecture/plugin/rib-storage-design.md` (source-DOWN ordering),
`docs/architecture/zefs-format.md` (statestore), and
`docs/plugin-development/metrics.md` (filter batch histogram). These edits
change no documented contract or source-anchor claim.

Parent evidence owed: the scoped lint and race-test commands in the Checklist.
Changed packages are `internal/component/plugin/ipc`,
`internal/component/plugin/registry`, and `internal/component/plugin/server`.
Focused tests include `TestFilterBatchOrderAndVerdicts`,
`TestFilterBatchFailureAppliesOnError`, `TestFilterBatchSizeObserved`,
`TestCompileFilterDeclarations`, and
`TestStartupKeepsThePolicyForARefusedDeclaration`.

### Mistake Log: Wrong Assumptions

| Assumption | Evidence | Correction |
|------------|----------|------------|
| A-1: all saved CI findings still need edits | The source audit above found eleven sites already addressed | Fix only the three remaining forms; the parent must still establish scoped lint and race results |

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Teardown errors | Error checks distinguish failures the test owns from teardown races, without weakening its assertions (R-1). |
| Published contracts | The registry rewrite preserves the Stage 1 declaration and SDK exports. Registry tests restore their snapshots. |
| Plugin and store evidence | All three trees are lint-clean, race tests pass, and ignored errors carry specific reasons (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Plugin and statestore scopes cleared | Record zero scoped lint findings and race-test results for the three trees (AC-1–2). |
| Preserved SDK and declaration contracts | Inspect the registry rewrite and SDK test assertions, including each ignored-error reason (AC-3). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| TLS and process failures | Error checks preserve TLS, process-stop and callback assertions and distinguish expected teardown failures. |
| Shared state | Registry tests restore snapshots, and statestore tests check the acknowledging operation without masking its error. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Final scoped verification (2026-11-02)

AC-1 and AC-2 pass after `f930c73ca9`, `ebbd06aba3`, `c5c69bf6fa`
and `7620e0dfc4`. This is scoped verification evidence, not the independent
Review Gate or closure. The parent's in-progress status is retained.

AC-1 command: `CGO_ENABLED=0 ./le job run label lint4-final-scoped-lint quiet command ./le go lint run scope "./internal/component/plugin/... ./pkg/plugin/... ./internal/core/statestore/..."`.
Exit 0; default, Linux integration, Darwin and capability flavors each
reported `0 issues.` No lint4 CPU job was running before launch.

AC-2 uses the retained race runs below. Tests use `CGO_ENABLED=1` and
`-tags "$tags"`, where tags are `ze_core` plus all unique first-column
`ze_` entries in `feature-gates.txt`, except the focused community producer
run, which used `ze_bgp`. Successful unchanged tests were not repeated.

| Package population | Observed evidence |
|--------------------|-------------------|
| plugin/ipc, plugin/registry, plugin/server, core/statestore, pkg/plugin/sdk | Package PASS/ok in `job-lint4-infra-policy-units-8fdf5b92.log`; the additional policy package failed and is not counted as an AC-2 pass |
| plugin root, plugin/cli, plugin/server after the query/lint fixes | Package PASS/ok in `job-lint4-fix-packages-f40cf343.log` |
| plugin/doctor, plugin/manager, plugin/yang, pkg/plugin, pkg/plugin/rpc | Package PASS/ok in `job-lint4-remaining-race-2ae0b12f.log` |
| plugin/all | 37 original top-level passes in the remaining-race log, plus `TestRegisteredWireMethods` in `job-lint4-wire-snapshot-bdf17cf0.log` and `TestNoIngressFilterGatesAPeerRouteOnRedistributionConfig` in `job-lint4-ingress-final-green-5ff53129.log`; all 39 source test names covered |
| plugin/process | 81 original top-level passes, seven in `job-lint4-process-unproven-28611362.log`, and `TestRecoveryQueuedDeliveryTransport` in `job-lint4-recovery-judge-fix-f2ee5a61.log`; all 89 source test names covered. Recovery has eight passing subtests, not eight top-level tests |

These are all 14 package directories containing Go source in the three AC
trees. Population checks compared source test declarations to named PASS
lines, not just wrapper exit status. The recovered fix-packages wrapper exit
was unavailable; its completed package PASS/ok lines are the evidence.
No explicit SKIP or race warning appears in these logs. Recovery retains
neighbor-watch EPERM and shutdown diagnostics; it is not log-silent.

The new product fix also reaches the community package:
`internal/component/bgp/plugins/filter_community/community_filter.go`
`evaluateCommunityFilter` passes both callback decisions to `rewriteVerdict`,
which rejects before deriving edits. The focused race run already passed
`TestCommunityRewritePreservesRejection`,
`TestCommunitySetsCarryWhatTheBodyChanged`, and
`TestCommunityFiltersPassConformance` in
`job-lint4-community-reject-green-d85f2666.log`.
The missing package coverage ran once with
`./le job run label lint4-community-unproven quiet command go test -race -tags "$tags" -count=1 -v -skip '^(TestCommunityRewritePreservesRejection|TestCommunitySetsCarryWhatTheBodyChanged|TestCommunityFiltersPassConformance)$' ./internal/component/bgp/plugins/filter_community`.
Exit 0; 109 additional top-level tests passed, with no explicit SKIP or race
warning. Together the two logs cover all 112 source test names. The selector
excludes only the three previously successful tests.

Retained navigation paths: all named logs except the initial infra-policy run
are under `tmp/session/2026-11-02-220832/scratch/`. The new lint log is
`job-lint4-final-scoped-lint-aae4a6b4.log`; the new community log is
`job-lint4-community-unproven-8d89175c.log`. The initial log is under
`tmp/session/2026-11-02-153314/scratch/`. Retain them through closure.
The commands, canonical producers and observed results here are the durable
record. No source or test was changed by this verifier; no whole-tree gate,
compile check, functional suite or push was run.

## Blocker

Independent closure on 2026-11-02 reviewed the five implementation commits
through `7620e0dfc4` and the completed evidence in `b0d8f62a88`. No product
finding was found in that owned history. Closure cannot record a clean review
of the current file population: a concurrent foreign edit deletes
`internal/component/bgp/plugins/filter_community/community_filter_test.go`
`TestCommunityIsNoEngineStep`, which asserted that neither legacy stage list
contains the community plugin. The retained 112-test evidence includes this
test; the working tree now has 111 community tests. No independent judgment
of this concurrent deletion was supplied to the closure.

Diagnosis: the symptom is a test deletion in a file this closure must pin.
`internal/le/spec/review.go` `recordedHash` hashes the working-tree bytes;
`CheckReview` compares those hashes, not the implementation commit's bytes.
The deletion arrived before the pin was recorded while review read the owned
history, so the initial machine CLEAN record included an unreviewed foreign
hunk. That verdict is withdrawn and replaced by FINDINGS. The owning layer is
the concurrent filter migration and its test proof. A [workaround] would
silently bless that deletion or overwrite another session's work. The [source]
resolution is for its owner to finish the migration's equivalent proof and
independent test judgment, then resume closure against the settled content.
No foreign hunk was changed or carried in this handback.

The retained scoped results remain valid for the implementation they tested:
four lint flavors clean, all 14 AC package directories covered, process 89/89,
composition root 39/39 and community 112/112 at verification. Closure ran no
new tests and repeated no successful check. The three inline lenses covered
logic/wiring, security/edge cases, and proof/docs/style. `core-design.md`'s
callback claim and `process-protocol.md`'s query-version contract match the
owned producers. Closure tables, the umbrella citation and the two closure
commits remain owed after this conflict clears.

## Historical blockers (resolved by the evidence above)

### Observed verification after restart (2026-11-02)

AC-1 and AC-2 now have observed failures, not only missing evidence. The
verification agent recovered the existing lint job, waited for PID 3187687
with a Linux pidfd, and did not start another lint run. Its retained exit
record reports 1. The command was `./le --name session job run label
lint4-scoped-verification quiet command ./le --name session go lint run scope
"./internal/component/plugin/... ./pkg/plugin/... ./internal/core/statestore/..."`.
The default and Linux integration flavors each reported these four findings;
the Darwin and capability flavors each reported the coordinator errcheck only.

| Canonical source | Finding | Required correction |
|------------------|---------|---------------------|
| `internal/component/plugin/coordinator_test.go`, startup calls in the no-reactor test | `SignalPluginStartupComplete` error unchecked | Assert the returned error; `Coordinator.SignalPluginStartupComplete` returns the reactor or post-startup error, or nil when neither exists |
| `internal/component/plugin/server/startup.go`, `registrationFromRPC` | `rangeValCopy`: copies a 160-byte filter declaration | Iterate by index and read the declaration through its address |
| `internal/component/plugin/server/filter_reference_alias_test.go`, Stage 1 alias test | `cancelled` misspelling | Use `canceled` |
| `internal/component/plugin/server/filter_reference_alias_test.go`, `aliasedSlotProcess` | `embedlit` on `DeclaredFilter` | Use the embedded-field literal form the linter requests |

The remaining race command was `./le --name session job run label
lint4-remaining-race quiet command go test -race -tags "$tags" -count=1 -v
./internal/component/plugin ./internal/component/plugin/all
./internal/component/plugin/cli ./internal/component/plugin/doctor
./internal/component/plugin/manager ./internal/component/plugin/process
./internal/component/plugin/yang ./pkg/plugin ./pkg/plugin/rpc`, with
`CGO_ENABLED=1` and tags consisting of `ze_core` plus all unique first-column
`ze_` entries in `feature-gates.txt`. It exited 1.

| Package | Observed result |
|---------|-----------------|
| plugin root, doctor, manager, yang; `pkg/plugin`, `pkg/plugin/rpc` | PASS |
| `internal/component/plugin/all` | FAIL: `TestRegisteredWireMethods` reports four additional methods: `ze-update:update-serve`, `ze-update:update-serve-start`, `ze-update:update-serve-stop`, `ze-update:update-serve-pause`. `TestNoIngressFilterGatesAPeerRouteOnRedistributionConfig` finds an empty ingress filter population |
| `internal/component/plugin/cli` | FAIL: `TestQueryAnswerIsTheSameLineFromBothWriters`; `answerQuery` omits protocol-version while `pkg/plugin/sdk/sdk_query.go` `runOrDeclare` sets it to 1. Align the actual writers, preserving the byte-equality assertion |
| `internal/component/plugin/process` | FAIL: `TestRecoveryQueuedDeliveryTransport/direct/newer-target-owner/control`; `registry.CheckFilterDeclarations` refuses bgp-rs because its declaration judge is nil, then `recoveryDeliveryScenario` calls `HasStructuredHandler` on the absent process. Restore the real judge in the fixture and assert process presence before dereferencing it; do not bypass declaration validation |

There were no explicit SKIP or race-detector warning lines in the new race
log. The process panic stopped that package before its remaining tests, so
it does not establish complete process coverage. The prior IPC, registry,
server, statestore, SDK and `pkg/ze` package passes were read from the prior
log and were not repeated. They remain historical evidence, not a fresh
verdict on concurrent shared-checkout edits. No Go source was edited by this
verification agent. Fixes and independent closure remain owed.

Diagnostic logs (retained until the parent consumes them):
`tmp/session/2026-11-02-220832/scratch/job-lint4-scoped-verification-08c4ecab.log`
and `tmp/session/2026-11-02-220832/scratch/job-lint4-remaining-race-2ae0b12f.log`.
The commands, canonical sources and observed results above are the durable
record; the scratch paths are navigation aids only.

### Earlier handback

Closure needs AC-1 and the remainder of AC-2. The implementation is committed
in `aaf2642dc1`: `ipc/filter_batch_test.go` `sendQueued`, its metrics-test
caller, `registry/filter_declaration.go` `DeclaredFilter.ReadsRaw`, and
`server/startup_chain_refused_test.go`
`TestStartupKeepsThePolicyForARefusedDeclaration`. No source defect is asserted
by this verification handback.

The retained tagged race run reports package passes for plugin IPC, registry,
server, statestore, SDK and `pkg/ze`. It does not establish the complete
`internal/component/plugin/...` and `pkg/plugin/...` population required by
AC-2. Its additional policy package failed
`TestPolicyFilterReferenceSource`; that result is not a plugin-package pass.
The previous coordinator records that scoped lint refused planning after
concurrent source edits, so AC-1 has no zero-findings result.

A fresh verification-only context must run the scoped lint command in Data
Flow and establish race evidence for the remaining packages, with the feature
tags from `feature-gates.txt`. The current editing context cannot start the
potentially long lint job under `ai/rules/commands.md`. Then resume this spec
as in-progress, complete the independent Review Gate and closure tables,
clear citations, and land both closure commits. Keep the existing acceptance
criteria intact.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the three trees>"` zero; `./le job run label plugin command go test -race ./internal/component/plugin/... ./pkg/plugin/... ./internal/core/statestore/...`. Full-run evidence belongs to the umbrella's CI run.
