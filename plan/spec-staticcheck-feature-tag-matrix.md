# Spec: type-check feature-tag combinations with Staticcheck

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 4/4 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-17 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/planning.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze checks the full feature set and selected build flavors, but it does not type-check each supported feature-off combination. A symbol can therefore be available only under one feature tag while code selected by another tag uses it. Add a structural Staticcheck matrix gate that derives configurations from the feature-gate manifest, includes test files, and reports the failing configuration. Keep the tracked build gate as the final-link check for shipped binaries.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/repo-maintenance.md` - requirements for a new verification gate and its discovery path
  → Constraint: add the gate to the live verification pipeline, developer setup, CI setup, agent index, documentation, and structural-gate inventory in the same change.
- [ ] `ai/rules/testing.md` - requirements for behavioral tests of guards and developer tooling
  → Constraint: drive the gate from its executable entry point with fixture modules that prove both the red and green verdicts.
- [ ] `ai/rules/evidence.md` - fail-closed guard and non-vacuity requirements
  → Constraint: an empty manifest, zero generated rows, a missing executable, or a package selection that checks nothing must fail with a corrective diagnostic.
- [ ] `ai/rules/simplicity.md` - smallest fully correct design
  → Decision: derive the matrix directly from `feature-gates.txt`; do not add an authored tag list or a generated matrix artifact.
- [ ] `ai/rules/plugins.md` - feature-gate registration and dependent-gate behavior
  → Constraint: check the bare-core and single-feature-off configurations. Preserve valid dependent constraints such as BGP with BMP and L2TP with RADIUS.
- [ ] `docs/architecture/testing/tracked-build-gate.md` - committed-tree build and final-link contract
  → Constraint: keep `ze-repository-tracked-build-check` unchanged. The Staticcheck gate checks working-tree package and test variants, not final linking or committed-tree isolation.
- [ ] `docs/contributing/testing.md` - verification-stage discovery and rerun guidance
  → Decision: document the matrix gate as a structural type-check stage distinct from lint and tracked builds.
- [ ] `docs/guide/developer-setup.md` - required development tools
  → Constraint: pin standalone Staticcheck 2026.1 in every setup path that runs verification.
- [ ] `https://staticcheck.dev/docs/running-staticcheck/cli/build-tags/` - official matrix behavior
  → Decision: use standalone Staticcheck matrix mode so each row has an independent tag configuration and row-named diagnostics.
- [ ] `https://staticcheck.dev/changes/2026.1/` - Go 1.26 compatibility
  → Constraint: Staticcheck 2026.1 supports Go 1.26 syntax and ignores patch-level toolchain differences.

### RFC Summaries (Scope: protocol)
- [ ] N-A - this tooling change does not implement or change a wire protocol.

**Key insights:**
- `feature-gates.txt` is the only feature inventory. Existing generators and gates derive from it.
- Existing lint and unit-test passes select all features. The bare-core unit pass covers only `cmd/ze/hub`.
- Staticcheck matrix mode includes test variants by default and reports type errors even when analyzer checks are disabled.
- Staticcheck does not final-link binaries. The tracked-build gate must remain the shipped-build proof.
- Staticcheck accepts a zero-row matrix and Staticcheck 2026.1 ignores a final row without a newline. The Ze producer must reject zero rows and always terminate the matrix.
- The current compile-out rules establish bare-core and direct single-feature-off configurations. They do not promise the full feature-tag power set.
- Every non-race first-party Go compilation, including binaries, tests, benchmarks, fuzzing, `go run`, nested helpers, and installed project tools, MUST explicitly use `CGO_ENABLED=0`. A test-only command that actually uses `-race` may explicitly use `CGO_ENABLED=1` on Linux and Darwin. Race binaries never ship or serve as release/build evidence.
## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `feature-gates.txt` - declares each compile-out tag and owned package once.
- [ ] `Makefile` - derives `ZE_FEATURES`, runs all-on lint and tests, exposes build and verification targets, and provides repository cache settings.
- [ ] `.golangci.yml` - generated all-on tag list for the Staticcheck analyzer inside golangci-lint.
- [ ] `scripts/codegen/plugin_imports.go` - generates per-tag and compound dependent registration files.
- [ ] `scripts/codegen/feature_tags.go` - generates static all-on tag consumers that cannot read the manifest at runtime.
- [ ] `scripts/dev/dep_audit.py` - rejects an importer that does not require the tag of a gated package and checks generated lint-tag drift.
- [ ] `scripts/dev/rfc_requirements.py` - uses a narrow `go vet` analyzer to type-check RFC-tag carrier tests under the all-on configuration.
- [ ] `scripts/checks/tracked_build.go` - builds six representative configurations from extracted HEAD and proves package and anchor selection is non-vacuous.
- [ ] `scripts/checks/tracked_build_test.go` - provides fixture patterns for executable gates, manifest failures, and vacuity guards.
- [ ] `scripts/status/verify_run.go` - defines the only live full and changed verification-stage lists.
- [ ] `scripts/status/verify_run_test.go` - pins both stage lists and structural-stage liveness.
- [ ] `scripts/dev/commit_helper.py` - prevents an unverified commit from bypassing a red structural gate.
- [ ] `scripts/dev/dev-setup.py` - installs required development tools; it pins golangci-lint but not standalone Staticcheck.
- [ ] `.github/workflows/verify.yml` - installs golangci-lint before the pre-commit verification pipeline.
- [ ] `scripts/dev/setup_claude_server.sh` - installs Go development tools for the agent workstation.
- [ ] `ai/INDEX.md` - routes verification-gate work to repository-maintenance rules and target discovery.
- [ ] `plan/spec-fixit-lint-blind-to-integration-tag.md` - related work that added the separate Linux and integration lint population.
- [ ] `plan/journal/gate-excludes-part-of-its-population.md` - records the recurring class of green gates that omit selected files.
- [ ] `plan/journal/feature-test-missing-build-tag.md` - records test files excluded by a tag configuration.
- [ ] `plan/journal/test-gate-repeats-expensive-work.md` - records verification work duplicated by new gates.
- [ ] `plan/journal/gate-verdict-depends-on-the-machine.md` - records host-dependent gate verdicts.
- [ ] `ai/rules/points/commands/cgo-free-builds/ze-is-cgo-free.md` - blocks every non-race first-party Go compilation unless its process environment explicitly sets `CGO_ENABLED=0`, with a narrow test-only `-race` exception.
- [ ] First-party producer population - `Makefile`, `mk/*.mk`, Go `exec.Cmd` builds, Python subprocesses, the functional `.ci` command runner, CI workflows, setup scripts, and independently runnable usage commands.

**Behavior to preserve:**
- `feature-gates.txt` remains the only authored feature-tag inventory.
- golangci-lint keeps its all-on Staticcheck lint pass and Linux integration pass.
- `ze-repository-tracked-build-check` keeps extracted-HEAD isolation, six flavor anchors, package-count checks, and final-link coverage.
- Generated dependent registrations continue to permit valid partial builds, including RADIUS without L2TP and BGP without BMP.
- Both full and changed verification modes continue to use `scripts/status/verify_run.go` as their stage source.
- Linux and Darwin execute the same race-instrumented test package population.

**Behavior to change:**
- Add one structural gate that type-checks production and test sources under all-on, bare-core, and each single-feature-off configuration.
- Report the configuration name with an unavailable-symbol diagnostic.
- Install pinned standalone Staticcheck 2026.1 with `CGO_ENABLED=0` in local, agent, evidence, and CI setup paths.
- Make the new structural gate live in full and changed verification and non-bypassable in commit preparation.
- Make every non-race first-party Go producer set `CGO_ENABLED=0` in its command or child process environment, including tests, benchmarks, fuzzing, `go run`, nested helpers, and installed project tools.
- Permit `CGO_ENABLED=1` only for a test-only producing command/process that contains `-race`; race binaries must remain outside shipping and release/build evidence paths.
## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A developer runs the matrix Make target directly or through full or changed pre-commit verification.
- The gate reads the whitespace-separated tag and package rows in `feature-gates.txt`.

### Transformation Path
1. The gate reads, validates, de-duplicates, and sorts the manifest tags.
2. It creates named all-on, bare-core, and one-feature-off rows.
3. It verifies that the row count is the unique tag count plus two and that every row has a unique name.
4. It writes a newline-terminated matrix to standalone Staticcheck.
5. Staticcheck selects package and test variants for each row and type-checks them with analyzer findings disabled.
6. The gate preserves Staticcheck diagnostics and exit status, including the failing row name.
7. The Make target enters both verification-stage lists. Commit preparation treats its red result as structural.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Manifest to gate | The gate reads `feature-gates.txt` directly | Fixture with duplicate, malformed, and empty manifests |
| Gate to Staticcheck | Newline-terminated matrix on standard input | Entry-point fixture that checks all named rows |
| Staticcheck to developer | Standard output, standard error, and exit status | Broken-symbol fixture asserts the row and symbol |
| Make to verification | Named Make target in both `stagesForMode` lists | Verification-stage inventory test |
| Verification to commit preparation | Target name in `STRUCTURAL_GATES` | Commit-helper structural-red test |

### Integration Points
- `feature-gates.txt` - sole matrix inventory.
- `scripts/status/verify_run.go:stagesForMode` - live full and changed pipelines.
- `scripts/dev/commit_helper.py:STRUCTURAL_GATES` - deterministic red classification.
- `scripts/dev/dev-setup.py:REQUIRED_TOOLS` and `.github/workflows/verify.yml` - pinned executable installation.
- `ai/INDEX.md` and testing documentation - agent and developer discovery.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Manifest to gate to Make to `stagesForMode` to commit helper |
| No unintended coupling (components stay isolated) | Yes | Tooling reads source metadata and does not import runtime components |
| No duplicated functionality (extends existing, does not recreate) | Yes | Staticcheck performs selection and type-checking; the Ze gate only derives and validates rows |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Offline developer tooling is not a packet or event hot path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Matrix rows derive from the manifest; no feature name is hardcoded |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Bare-core and each single-feature omission are the supported structural population; arbitrary multi-feature omissions are not promised | User selected the single-feature-off population at the research gate | The gate name or ACs would overstate coverage | User confirmation on 2026-08-16 | confirmed |
| A-2 | Omitting one provider tag while retaining all other features selects the consumer in the requested accidental-dependency class | The requested case is one feature using a symbol supplied only by another feature | The fixture would not discriminate the defect | `TestStaticcheckFeatureMatrixReportsBrokenFeatureDependency` keeps `ze_consumer` selected in `without_ze_provider`, observes `ProviderOnly` undefined, then corrects the provider constraint and passes all rows | confirmed |
| A-3 | Staticcheck 2026.1 works with Ze's Go 1.26 toolchain | Official 2026.1 release notes state Go 1.26 support | CI cannot run the new gate | Staticcheck 2026.1 (v0.7.0) checked all 38 live rows with Go 1.26 in controlled warm and isolated cold runs | confirmed |
| A-4 | The unique feature count plus two rows fits the verification time budget | Controlled live-manifest runs use the pinned tool on an uncontended host | The gate makes each verification run too slow | Warm target: 38 rows in 183.51s; isolated empty `GOCACHE` and `STATICCHECK_CACHE`: 38 rows in 246.85s | confirmed |

Controlled evidence (2026-08-17): Staticcheck 2026.1 (v0.7.0) with Go 1.26
checked all 38 live rows in 183.51s warm. An isolated cold run with empty
`GOCACHE` and `STATICCHECK_CACHE` checked the same 38 rows in 246.85s. These
runs confirm A-3 and A-4. The two earlier 1200-second attempts occurred under
shared CPU contention and remain invalid, so they provide no performance
evidence. The controlled results did not require a change to the 25-minute
checker deadline.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The matrix is empty or its final row is lost, and Staticcheck returns a vacuous success | Row-count mismatch or missing final newline | Validate a non-zero row count, require unique rows, and write the final newline |
| R-2 | The new pass duplicates all Staticcheck lint diagnostics | Diagnostics unrelated to compilation appear | Disable analyzer checks and retain golangci-lint as the lint owner |
| R-3 | A feature dependency needs two simultaneous omissions to fail | Constraint inventory finds a compound negative or disjunction | Add the explicit supported row or reject that constraint shape |
| R-4 | Host-only rows omit Linux-selected source | A Linux-only fixture is not selected | Retain Linux lint and tracked-build rows; add a Linux matrix axis only with measured need |
| R-5 | Standalone Staticcheck is missing or built with an incompatible Go version | Tool probe or version fixture fails | Pin 2026.1 in all setup paths and fail with the install command |
| R-6 | Package selection is empty under a row | Staticcheck exits without a package diagnostic | Check the selected package count or require a known anchor package per row |
| R-7 | The row count makes verification too slow | Cold or warm duration exceeds the accepted gate budget | Measure before integration; use Staticcheck and Go caches without reducing the supported population |
| R-8 | Replacing the tracked build loses final-link failures | Matrix passes while a shipped binary does not link | Keep the tracked-build stage unchanged and document the boundary |
| R-9 | A standalone or centrally launched producer inherits or enables cgo outside a race test | A first-party binary or non-race test links host C dependencies | Derive shell, workflow, Make, Docker, documentation, Python, nested Go, and functional `.ci` populations in structural tests; require explicit zero except when the same derived test-only process contains `-race`, and exclude race binaries from shipping/evidence |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Pre-commit verification can become slow, reject a valid feature combination, or pass without checking the promised population. Runtime binaries do not change. |
| How is it reverted? | Remove the new structural stage and tool installation in one focused change. The existing lint, test, and tracked-build gates remain intact. |
| Who else touches this path? | `plan/spec-fixit-lint-blind-to-integration-tag.md` changes lint populations; verification and setup work can modify the same Make, workflow, and tool-inventory files. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-staticcheck-feature-matrix-check` | → | Staticcheck matrix checker executable | `TestStaticcheckFeatureMatrixMakeTargetRunsExecutable` |
| Full and changed pre-commit verification | → | Matrix Make target through `stagesForMode` | `TestVerifyStagesIncludeStaticcheckFeatureMatrix` |
| Commit preparation after a red matrix stage | → | Structural-red refusal | `test_staticcheck_feature_matrix_is_structural` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The manifest contains N unique feature tags | The gate checks exactly N+2 named configurations: distro with all features, bare core, and distro with each feature omitted once |
| AC-2 | A file selected without feature A uses a symbol defined only when feature A is selected | The gate exits red and identifies both the unavailable symbol and the `without_ze_<feature>` configuration |
| AC-3 | The unavailable symbol exists only in a selected test file | The gate exits red for that configuration; test files are part of the checked population |
| AC-4 | All production and test sources type-check in every promised configuration | The gate exits zero and reports the checked row count |
| AC-5 | The manifest is missing, empty, malformed, or produces duplicate matrix row names | The gate fails closed and states the manifest or matrix defect; repeated tags on different package rows remain valid |
| AC-6 | Staticcheck is missing, cannot start, times out, or checks no package | The gate reports that it could not judge the matrix and never reports success |
| AC-7 | The matrix reaches Staticcheck | Input ends with a newline, contains unique row names, and disables analyzer findings without disabling compile diagnostics or tests |
| AC-8 | Full or changed pre-commit verification runs | The matrix target is a live structural stage in both modes and a red result cannot pass through normal unverified commit preparation |
| AC-9 | A developer or CI worker installs project tools | Standalone Staticcheck 2026.1 is installed from a pinned module version with `CGO_ENABLED=0` in every verification bootstrap |
| AC-10 | The matrix gate lands | Existing golangci lint and tracked-build final-link coverage remain live and unchanged in purpose |
| AC-11 | A developer or agent looks for feature-tag validation | The target, population, direct rerun command, and boundary with lint and tracked builds are discoverable from setup, testing, architecture, and agent indexes |
| AC-12 | Any first-party Go compilation runs, including a binary, test, benchmark, fuzz target, `go run`, nested helper, or installed project tool | Every non-race process explicitly sets `CGO_ENABLED=0`. A test-only process may explicitly set `CGO_ENABLED=1` only when its same derived command contains `-race`. Linux and Darwin retain race instrumentation. Race binaries do not ship or serve as release/build evidence |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStaticcheckFeatureMatrixDerivesManifestTags` | `scripts/checks/staticcheck_feature_matrix_test.go` | Unique sorted tags come only from the manifest | passing |
| `TestStaticcheckFeatureMatrixEmitsPromisedRows` | `scripts/checks/staticcheck_feature_matrix_test.go` | Distro all-on, bare-core, and each single-feature-off row exist once | passing |
| `TestStaticcheckFeatureMatrixRejectsVacuousInput` | `scripts/checks/staticcheck_feature_matrix_test.go` | Missing, empty, malformed, zero-row, duplicate-row, and unterminated matrices fail; repeated tags on different package rows remain valid | passing |
| `TestStaticcheckFeatureMatrixReportsBrokenFeatureDependency` | `scripts/checks/staticcheck_feature_matrix_test.go` | A selected consumer cannot use a provider-only symbol after the provider tag is omitted | passing |
| `TestStaticcheckFeatureMatrixIncludesTestFiles` | `scripts/checks/staticcheck_feature_matrix_test.go` | A test-only unavailable symbol fails its row | passing |
| `TestStaticcheckFeatureMatrixAcceptsValidConfigurations` | `scripts/checks/staticcheck_feature_matrix_test.go` | Coherent sources and valid dependent tags pass every promised row | passing |
| `TestStaticcheckFeatureMatrixClassifiesToolFailures` and `TestStaticcheckFeatureMatrixCompletedVerdictWinsExpiredContext` | `scripts/checks/staticcheck_feature_matrix_test.go` | Missing executable, timeout, descendant-held pipe closure, and no-package results are incomplete; built-checker stdout backpressure crosses the deadline after completed exit 0 and exit 1 without replacing either verdict | passing |
| `TestStaticcheckFeatureMatrixMakeTargetRunsExecutable` | `scripts/checks/checks_test.go` | The public Make target reaches the checker | passing |
| `TestVerifyStagesIncludeStaticcheckFeatureMatrix` | `scripts/status/verify_run_test.go` | Full and changed stage inventories contain the gate | passing |
| `test_staticcheck_feature_matrix_is_structural` | `scripts/dev/commit_helper_test.py` | Normal unverified commit preparation cannot bypass a red matrix stage | passing |
| `TestDevSetupRequiresPinnedStaticcheck` | `scripts/dev/dev_setup_test.py` | Developer setup keeps one 2026.1 pin, rejects stale, missing, failing, or timed-out Staticcheck probes, and proves that every Go-tool child receives the parent environment with `CGO_ENABLED=0` | passing |
| `TestVerifyInstallsPinnedStaticcheck` and `TestPinnedStaticcheckInstallIgnoresCommentedCommands` | `scripts/dev/github_workflows_test.go` | CI, agent, and evidence setup use one exact active cgo-free 2026.1 install; commented, duplicate, or unpinned installs cannot satisfy the guard | passing |
| `TestFirstPartyGoProducerCommandsDisableCGO` | `scripts/checks/checks_test.go` | Token-matched shell, workflow, Make, Docker, documentation, Python, functional `.ci`, compiled-test, and install producers are non-vacuously derived; non-race producers require explicit zero; cgo enablement requires `-race` in the same derived test-only command/process | passing |
| `TestNestedGoCompilationCommandsDisableCGO` | `scripts/checks/checks_test.go` | Each first-party Go `exec.Cmd` build or run sets a CGO-free child environment | passing |
| `TestChildEnvDisablesCGO` | `internal/test/runner/runner_exec_util_test.go` | The functional `.ci` producer copies the parent environment, preserves unrelated entries, forces the child to `CGO_ENABLED=0`, and does not mutate the parent | passing |
| `TestEveryExecSiteUsesChildEnv` and `TestEveryExecSiteUsesChildEnvChecksAllSyntax` | `internal/test/runner/runner_exec_util_test.go` | Type-resolved `os.Environ()` calls through default, aliased, dot, or parenthesized import expressions are checked across the whole production AST; local shadowing is ignored and only the canonical receiver-free producer is exempt | passing |
| `TestCodeQLBuildUsesShippedTags` | `scripts/status/verify_run_test.go` | All three generated CodeQL build commands set `CGO_ENABLED=0` and retain shipped tags | passing |
| `TestFeatureTagsCheckRuns` and `TestFeatureTagsCoverManifest` | `scripts/codegen/feature_tags_test.go` | Generated CodeQL and quickstart command anchors include `CGO_ENABLED=0` | passing |
| `TestBuildUsesGokBuildFn` | `internal/appliance/cmd_build_test.go` | The embedded gokrazy producer overrides a parent `CGO_ENABLED=1` with zero | passing |

### Round-1 review corrections

- Local setup probes the bounded `staticcheck -version` argument vector and accepts only 2026.1.
- Bootstrap pin guards inspect active workflow and shell content, not comments.
- The runner AST guard checks package initializers and methods while exempting only the canonical top-level `childEnv`.
- Matrix timeout cancellation kills the Staticcheck process group and reaps the direct process; its fixture records and checks a descendant PID.
- QEMU documentation states that in-VM unit and integration Go tests are non-race, and tracked-build documentation limits final-link proof to six tracked configurations.

These corrections and their regression tests are written but were not executed in the round-1 edit pass.
Subsequent main-thread focused verification passed the modified Python setup,
active bootstrap-pin, runner AST, and Staticcheck failure-classification tests,
including descendant cleanup. Documentation drift and changed-source lint also
passed. The round-1 findings are resolved at their producers. This is not an
independent-review-clean or closure claim.

### Round-2 review corrections

- The runner guard resolves `Environ` through `go/types` and the standard importer, so alias and dot imports cannot bypass it and locally shadowed names do not create false positives.
- Matrix verdict classification now rejects no-package output first, then preserves completed exit 0 and Staticcheck verdict exit 1 before treating only non-verdict process errors as deadline failures.
- Injected runner fixtures cover aliases, dot-import package initializers and methods, and local shadowing. Normal-package matrix fixtures run the built checker, hold its `StdoutPipe` undrained past the deadline with bounded output backpressure, then require completed exit 0 and exit 1 to survive.

These round-2 source and regression-test edits are written but unverified. No
independent-review-clean or closure claim is made.

### Round-3 review corrections

- `isOSEnvironCall` unwraps every parenthesized callee layer before type-resolved identifier or selector dispatch. Injected default, aliased, and dot-import parenthesized calls remain visible outside canonical `childEnv`.
- Matrix descendant cleanup is proven by pipe lifetime rather than PID liveness. A bounded background sleep inherits the fake tool's output pipes; process-group timeout must close them and return with headroom before the sleep duration, while an outer deadline bounds the broken case.

Main-thread focused verification passed `TestEveryExecSiteUsesChildEnv*` in
3.314s, both affected Staticcheck matrix tests in 3.436s, and
`ze-lint-changed` with zero issues. Review rounds 1 through 3 are resolved at
their producers. The formal Review Gate and closure remain owned by the closure
phase.


### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Unique manifest tags | At least one | One tag produces three rows | Zero tags | N-A; runtime is measured instead of imposing an arbitrary maximum |
| Generated row count | N+2 | Exactly N+2 | N+1 | N+3 |
| Gate deadline | Positive duration | Small positive fixture deadline | Zero or negative | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | N-A | This is developer verification tooling with no daemon, CLI, API, or RPC behavior | N-A |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No wire behavior changes | N-A |

## Files to Modify
- `Makefile` - add the public matrix target, PHONY declaration, and help entry.
- `scripts/checks/checks_test.go` - prove the Make entry point reaches the checker.
- `scripts/status/verify_run.go` - add the target to full and changed verification.
- `scripts/status/verify_run_test.go` and its stage fixtures - pin stage order and liveness.
- `scripts/dev/commit_helper.py` - classify the new target as structural.
- `scripts/dev/commit_helper_test.py` - prove structural-red handling.
- `scripts/dev/dev-setup.py` and `scripts/dev/dev_setup_test.py` - install and pin Staticcheck.
- `.github/workflows/verify.yml` and `scripts/dev/github_workflows_test.go` - install and verify the CI pin.
- `scripts/dev/setup_claude_server.sh` - install the pin in the agent workstation.
- `scripts/evidence/effective-verify.sh` - install the pin in isolated verification evidence.
- `docs/guide/developer-setup.md` - list the required tool and target.
- `docs/contributing/testing.md` and `docs/functional-tests.md` - document the structural matrix and the cgo-enabled, test-only race path separately from non-race zero-CGO commands.
- `docs/architecture/testing/tracked-build-gate.md` - document the type-check versus final-link boundary.
- `ai/INDEX.md` - add the matrix target to developer-tool discovery.
- `README.md` and `docs/guide/quickstart.md` - make the Ze build and generated install examples explicitly CGO-free.
- `scripts/codegen/feature_tags.go` and `scripts/codegen/feature_tags_test.go` - own and check the generated CodeQL and quickstart command anchors.
- `Makefile`, `mk/appliance.mk`, `mk/gokrazy.mk`, `mk/test-functional.mk`, `mk/test-unit.mk`, `mk/test-integration.mk`, and `mk/test-chaos.mk` - keep every non-race compilation at zero and run test-only race targets with `CGO_ENABLED=1` and `-race` on Linux and Darwin.
- `.github/workflows/codeql.yml` and `.github/workflows/verify.yml` - set zero on CodeQL builds and Go-installed verification tools.
- Non-race Python Go producers under `scripts/dev/`, `scripts/evidence/`, and `test/` - copy the parent environment, force zero in the child, and leave the parent unchanged.
- `internal/test/runner/runner_exec_util.go` and `internal/test/runner/runner_exec_util_test.go` - make the functional `.ci` process boundary CGO-free and prove its behavior.
- First-party shell, workflow, Make, Docker, `go:generate`, and independently runnable usage commands - set zero directly or inherit the explicit Make/runner environment.
- `scripts/dev/stress-repro.py` - build its explicitly requested test-only race binary with `CGO_ENABLED=1` and `-race` on Linux and Darwin under `tmp/`, outside shipping and release/build evidence.
- `cmd/ze/hub/build_tag_*_absent_test.go`, `internal/exabgp/bridge/bridge_integration_test.go`, `scripts/checks/tracked_build.go`, and `scripts/checks/tracked_build_test.go` - set zero on nested Go builds.
- `internal/appliance/cmd_build.go`, `internal/appliance/cmd_build_test.go`, and `cmd/ze-gok/main.go` - force embedded and standalone gokrazy build processes to zero.

## Files to Create
- `scripts/checks/staticcheck_feature_matrix.go` - derive and validate rows, invoke Staticcheck, and classify the verdict.
- `scripts/checks/staticcheck_feature_matrix_test.go` - executable fixtures for matrix derivation, unavailable symbols, test inclusion, valid rows, and failure classification.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No runtime configuration changes |
| YANG validation constraints | N-A | No YANG changes |
| YANG custom validators | N-A | No YANG changes |
| CLI commands/flags | N-A | A Make developer target is not a Ze CLI command |
| CLI grammar (keyword before value) | N-A | No Ze CLI surface |
| Editor autocomplete | N-A | No configuration or command values |
| Functional test for new RPC/API | N-A | No RPC or API |
| Pipe completeness | N-A | No command output pipeline |
| Env var registration | N-A | No runtime environment variable |
| Doctor check for runtime dependencies | N-A | Staticcheck is a development dependency checked by setup and the gate, not a daemon dependency |
| Prometheus counters/metrics | N-A | No runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP behavior |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | Developer tooling only |
| 2 | Config syntax changed? | N-A | No config change |
| 3 | CLI command added/changed? | N-A | No Ze CLI change |
| 4 | API/RPC added/changed? | N-A | No API change |
| 5 | Plugin added/changed? | N-A | No plugin change |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md` |
| 7 | Wire format changed? | N-A | No wire change |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No protocol change |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/testing.md` |
| 11 | Affects daemon comparison? | N-A | No daemon behavior |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/tracked-build-gate.md` |
| 13 | Route metadata keys added/changed? | N-A | No route data |
| 14 | Prometheus counters added/changed? | N-A | No metrics |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | Only verification-stage inventory changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Search and update every source anchor for the changed tooling files |
| 17 | Existing docs show config/CLI/API examples for this area? | N-A | No config, CLI, or API examples |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - add the checker entry point, Make target, and failing verification-stage tests.
   - Tests: `TestStaticcheckFeatureMatrixMakeTargetRunsExecutable`, `TestVerifyStagesIncludeStaticcheckFeatureMatrix`, and `test_staticcheck_feature_matrix_is_structural`.
   - Files: checker stub, `Makefile`, verification runner, and commit helper.
   - Verify: the public target and both pipelines reach the checker; fixture behavior is still red.
2. **Phase: Matrix contract** - derive and validate the N+2 row population from the manifest.
   - Tests: manifest derivation, promised rows, row boundaries, and vacuous-input failures.
   - Files: checker and checker tests.
   - Verify: tests fail before derivation, then pass with exact named rows and a final newline.
3. **Phase: Type-check verdict** - invoke Staticcheck and classify valid, broken, missing-tool, timeout, and empty-package outcomes.
   - Tests: broken production symbol, broken test symbol, valid configurations, and tool failures.
   - Files: checker and checker tests.
   - Verify: the broken fixtures name the symbol and row; coherent fixtures pass; incomplete runs never report green.
4. **Phase: Installation and discovery** - pin the tool in every verification bootstrap and document the new stage.
   - Tests: setup pin, workflow pin, generated-stage inventory, documentation and wiring gates.
   - Files: setup scripts, CI workflow, evidence script, docs, and `ai/INDEX.md`.
   - Verify: cold and warm gate duration is recorded; scoped package, lint, documentation, and wiring gates pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has an entry-point test or an exact inventory assertion |
| Feature completeness | Distro all-on, bare core, and every single-feature omission are present exactly once |
| Correctness | Compile diagnostics remain fatal with analyzer checks disabled and test variants remain selected |
| Naming | Row names identify `all_features`, `core_only`, or the exact omitted `ze_` tag |
| Data flow | The manifest is the only feature list and `stagesForMode` remains the only verification-stage list |
| Rule: evidence | Empty input, missing tools, timeouts, and no-package selection fail closed |
| Rule: simplicity | No generated matrix, second tag list, replacement lint pass, or replacement build gate exists |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Manifest-derived matrix checker | Checker fixture tests |
| Production unavailable-symbol detection | Broken production fixture |
| Test unavailable-symbol detection | Broken test fixture |
| Public Make target | Make entry-point test |
| Full and changed verification wiring | Stage inventory tests |
| Structural-red enforcement | Commit-helper test |
| Pinned local, agent, evidence, and CI installation | Setup and workflow tests |
| Developer and agent discovery | Documentation, link, and wiring checks |
| Preserved tracked-build final linking | Stage inventory plus existing tracked-build tests |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Manifest input | Reject malformed tags and never pass manifest package text as shell syntax |
| Process execution | Use argument vectors and standard input; do not invoke a shell |
| Resource exhaustion | Bound the row count to N+2 and the process with a deadline |
| Failure disclosure | Report actionable paths, tags, rows, and tool output without environment secrets |
| Fail closed | Missing tools, killed processes, empty rows, and empty package populations cannot return success |

## Design Insights
- Staticcheck matrix mode solves the type-selection problem but does not remove compiler work or final-link needs.
- The matrix parser's empty-input and final-newline behavior makes a validated wrapper necessary.
- A single-feature-off population directly covers the requested accidental dependency without claiming the full power set.
- Test-file inclusion is the main advantage over `go build`; `-checks=-all` keeps the new stage separate from lint ownership.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Use an executable Go checker | Generated matrix artifact; Make-time stream | It matches existing `scripts/checks` gates and makes vacuity, timeout, and diagnostics testable without a second artifact |
| Check N+2 configurations | Pairwise omissions; full power set | The user selected single-feature-off coverage; pairwise is expensive and still incomplete |
| Derive at execution | Authored or generated tag list | `feature-gates.txt` remains the single source of truth |
| Disable analyzer findings | Run full standalone Staticcheck lint | golangci-lint already owns lint; this gate owns compile diagnostics |
| Keep tracked build | Replace it with Staticcheck | Staticcheck does not final-link shipped binaries or judge extracted HEAD |
| Add a distinct structural stage | Fold into `ze-lint` | The population and failure contract differ from lint and must remain visible |

## Known Limitations
- The gate does not claim arbitrary combinations with two or more omitted features. The user selected this boundary on 2026-08-16.
- Host matrix rows do not replace Linux integration lint or Linux tracked-build coverage.
- Staticcheck type-checks test sources but does not run or final-link test binaries.

## RFC Documentation (Scope: protocol)

N-A. This tooling change does not implement protocol behavior.

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-12 all demonstrated
- [x] User stories are N-A because this tooling change has no user-facing operation
- [x] Wiring Test table complete: every row names a concrete test and none is deferred
- [x] Scoped package and structural verification passed; broad documentation and lint gates are red only on attributed foreign RFC/`forward_med_test.go` work under the shared-checkout rule
- [x] Tooling code integrated through the public Make and verification entry points
- [x] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [x] Architectural Verification table filled, including registration over hardcoding
- [x] Three-round critical Review Gate is clean and machine-recorded
- [x] Every A-N confirmed or broken, none `unvalidated`
- [x] Deferral shard resolved: metadata declares no shard and no deferred row exists

### TDD
- [x] Tests written
- [x] Tests failed before their producers existed, as recorded in the phase handoff
- [x] Focused tests and the live 38-row matrix pass
- [x] Boundary tests cover row count and positive deadline inputs
- [x] Functional `.ci` tests are N-A because no daemon behavior changes
- [x] Interop tests are N-A because no protocol behavior changes

### Closure
- [x] Appended `plan/TEMPLATE-CLOSURE.md` and completed every section
- [x] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [x] No additional closure journal row; this session's three foreign-red attribution rows are included
- [x] **Commit A:** implementation + tests + docs + spec + this session's three journal rows
- [x] **Commit B:** removes only `plan/spec-staticcheck-feature-tag-matrix.md`

---

## Implementation Summary

### What Was Implemented
- `scripts/checks/staticcheck_feature_matrix.go:runStaticcheckFeatureMatrix` derives and validates a manifest-owned N+2 matrix, sends it to standalone Staticcheck with tests enabled and analyzers disabled, preserves diagnostics, and fails closed on incomplete judgment.
- `Makefile:ze-staticcheck-feature-matrix-check`, `scripts/status/verify_run.go:stagesForMode`, and `scripts/dev/commit_helper.py:STRUCTURAL_GATES` wire the checker into direct, full, changed, and commit-preparation entry points while retaining the tracked-build stage.
- `scripts/dev/dev-setup.py:probe_tool` and the local, CI, agent, and evidence bootstraps pin cgo-free Staticcheck 2026.1.
- First-party Go producers and `internal/test/runner/runner_exec_util.go:childEnv` enforce explicit non-race `CGO_ENABLED=0`; test-only race producers retain `CGO_ENABLED=1` with `-race`.

### Bugs Found/Fixed
- Stale Staticcheck executables were accepted as present. `probe_tool` now requires exact 2026.1 output; `TestDevSetupRequiresPinnedStaticcheck` covers stale, missing, failing, and timed-out probes.
- Commented bootstrap commands satisfied the pin guard. `activeStaticcheckInstallCounts` strips comments; `TestPinnedStaticcheckInstallIgnoresCommentedCommands` covers the bypass.
- Runner environment construction escaped the guard through initializers, methods, aliases, dot imports, shadowing, and parenthesized callees. `unexpectedOSEnvironCalls` now type-resolves the complete AST; `TestEveryExecSiteUsesChildEnvChecksAllSyntax` covers those forms.
- Staticcheck timeout left descendants alive and deadline expiry could replace completed verdicts. `judgeStaticcheckFeatureMatrix` kills the process group, reaps the direct process, and classifies completed exits first; the failure and backpressure fixtures cover both defects.
- The public Make test's deterministic-shim rewrite reduced its syntactic assertion count. It now explicitly rejects any `could not be judged` output, so the preserved complete-verdict invariant is direct and `audit-test-relaxation.py` no longer reports this test.

### Security Review
- Manifest tokens are constrained by `featureTagPattern` and `validPackagePath`; package text never reaches a shell.
- `judgeStaticcheckFeatureMatrix` uses an argument vector and matrix stdin, a 25-minute deadline, isolated process-group cancellation, and explicit exit 0/1/2 classification.
- Empty inputs, empty rows, missing packages, missing tools, start failures, timeouts, malformed invocations, and output-write failures all deny success. The offline repository-owned manifest and bounded N+2 population introduce no untrusted allocation, path traversal, authorization, cryptographic, or secret-handling surface.

### Documentation Updates
- `docs/guide/developer-setup.md` names the required 2026.1 tool and direct target with a `scripts/dev/dev-setup.py -- REQUIRED_TOOLS, detect_os` source anchor.
- `docs/contributing/testing.md` documents the N+2 population, test inclusion, direct rerun, multi-off boundary, and tracked-build distinction with a `scripts/checks/staticcheck_feature_matrix.go -- buildFeatureMatrix, runStaticcheckFeatureMatrix` anchor.
- `docs/architecture/testing/tracked-build-gate.md` limits final-link proof to the six tracked configurations and anchors the matrix producer.
- `ai/INDEX.md` exposes the direct target to agents. `make ze-doc-verify` reached only foreign stale `rfc/requirements/rfc4271.md` and `rfc/requirements/rfc7947.md`; the matrix docs drift check and `make ze-repository-check` passed.

### Deviations from Plan
- No generated matrix artifact or additional feature inventory was added.
- Staticcheck timeout cleanup and exact tool-version probing were strengthened by review without changing the approved population or 25-minute deadline.
- The first-party CGO invariant was clarified to retain cgo-enabled race instrumentation only for test-only `-race` processes on Linux and Darwin.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | PID existence was used as descendant cleanup evidence | A killed zombie can still satisfy `kill(pid, 0)` | Round-3 review | Replaced PID probing with bounded descendant-held-pipe lifetime evidence |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Type-check supported feature-tag combinations | Done | `scripts/checks/staticcheck_feature_matrix.go:deriveFeatureMatrix` | Live target checked all 38 rows |
| Include production and test sources | Done | `judgeStaticcheckFeatureMatrix` | Staticcheck defaults retained; `TestStaticcheckFeatureMatrixIncludesTestFiles` passed |
| Report the failing configuration | Done | `judgeStaticcheckFeatureMatrix` | Broken provider fixture reports `without_ze_provider` and the missing symbol |
| Preserve tracked-build final linking | Done | `scripts/status/verify_run.go:stagesForMode` | Inventory tests keep tracked build after the matrix stage |
| Enforce first-party CGO process rules | Done | `TestFirstPartyGoProducerCommandsDisableCGO` | Derived producer population passed |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestStaticcheckFeatureMatrixDerivesManifestTags`, `TestStaticcheckFeatureMatrixEmitsPromisedRows` | Exact sorted N+2 population |
| AC-2 | Done | `TestStaticcheckFeatureMatrixReportsBrokenFeatureDependency` | Names omitted row and unavailable production symbol |
| AC-3 | Done | `TestStaticcheckFeatureMatrixIncludesTestFiles` | Test-only unavailable symbol is fatal |
| AC-4 | Done | `TestStaticcheckFeatureMatrixAcceptsValidConfigurations`; live target | Valid fixtures and 38 live rows pass |
| AC-5 | Done | `TestStaticcheckFeatureMatrixRejectsVacuousInput` | Manifest and matrix invariants fail closed |
| AC-6 | Done | `TestStaticcheckFeatureMatrixClassifiesToolFailures`, `TestStaticcheckFeatureMatrixCompletedVerdictWinsExpiredContext` | Missing, start, timeout, no-package, and incomplete outcomes return 2 |
| AC-7 | Done | `renderFeatureMatrix`, `judgeStaticcheckFeatureMatrix` | Final newline, unique names, `-checks=-all -matrix`, tests enabled |
| AC-8 | Done | `TestVerifyStagesIncludeStaticcheckFeatureMatrix`, `test_staticcheck_feature_matrix_is_structural` | Both modes and structural refusal covered |
| AC-9 | Done | `TestDevSetupRequiresPinnedStaticcheck`, `TestVerifyInstallsPinnedStaticcheck` | Exact active cgo-free 2026.1 pins |
| AC-10 | Done | `TestVerifyStagesIncludeStaticcheckFeatureMatrix`, `TestStructuralGatesAreLiveStages` | Existing lint purpose and tracked-build ordering retained |
| AC-11 | Done | `make ze-repository-check`; anchored setup, testing, architecture, and index entries | Discovery surfaces resolve |
| AC-12 | Done | CGO producer, nested compilation, runner, CodeQL, feature-tag, and appliance tests | Non-race zero and test-only race exception enforced |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Matrix derivation, rows, and vacuity tests | Done | `scripts/checks/staticcheck_feature_matrix_test.go` | Focused package command passed |
| Broken production, broken test, valid, and tool-failure fixtures | Done | `scripts/checks/staticcheck_feature_matrix_test.go` | Focused package command passed |
| Completed-verdict backpressure fixture | Done | `scripts/checks/staticcheck_feature_matrix_test.go` | Exit 0 and 1 precedence passed |
| Public Make wiring | Done | `scripts/checks/checks_test.go:TestStaticcheckFeatureMatrixMakeTargetRunsExecutable` | Deterministic shim and complete verdict passed |
| Verification-stage and CodeQL inventory | Done | `scripts/status/verify_run_test.go` | Focused package command passed |
| Structural-red classification | Done | `scripts/dev/commit_helper_test.py` | Python unit command passed |
| Local and bootstrap Staticcheck pins | Done | `scripts/dev/dev_setup_test.py`, `scripts/dev/github_workflows_test.go` | Python and Go focused commands passed |
| First-party and nested CGO producer guards | Done | `scripts/checks/checks_test.go` | Derived producer population passed |
| Functional runner child environment and syntax guard | Done | `internal/test/runner/runner_exec_util_test.go` | Focused package command passed |
| Generated feature-tag anchors and embedded gokrazy build | Done | `scripts/codegen/feature_tags_test.go`, `internal/appliance/cmd_build_test.go` | Both focused package commands passed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Matrix checker and direct tests | Done | Created under `scripts/checks/` |
| Make, verification, and commit-helper wiring | Done | Live targets and inventories retain tracked build |
| Setup, workflow, agent, and evidence bootstraps | Done | Exact 2026.1 cgo-free install |
| CGO producer population | Done | Make, shell, Python, Go subprocess, workflow, QEMU, generator, and functional boundaries covered |
| Developer, testing, architecture, and agent docs | Done | Source-anchored and discoverable |

### Audit Summary
- **Total items:** 44 grouped requirements, ACs, planned tests, and file populations
- **Done:** 44
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 review-driven refinements recorded above

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Type-check every promised feature-tag configuration | Integrated gate | `make ze-staticcheck-feature-matrix-check` printed `checked 38 rows` |
| Detect accidental cross-feature symbol dependency | Discriminating fixture | `TestStaticcheckFeatureMatrixReportsBrokenFeatureDependency` observes `ProviderOnly` under `without_ze_provider`, then passes after correction |
| Include test files in the checked population | Discriminating fixture | `TestStaticcheckFeatureMatrixIncludesTestFiles` observes `ProviderTestOnly` in `consumer_test.go` |
| Keep shipped-binary final linking | Stage inventory | `TestVerifyStagesIncludeStaticcheckFeatureMatrix` requires tracked build after matrix in both modes |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| No deferral shard | done | Metadata declares `Deferral shard: -`; all AC-1 through AC-12 are complete |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/staticcheck-feature-tag-matrix-95ead384-f7b2-4a4a-9286-268f9021bd63.md` |
| `review_gate.py check` | `OK (85 code files, clean, hashes match)` |
| Rounds | 3 |
| Reviewer lenses used | matrix logic and process lifecycle; AC proof and guard vacuity; docs and final-fix edge cases |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Accepting stale Staticcheck | `scripts/dev/dev-setup.py:probe_tool` | Exact version probe and regression cases |
| 2 | ISSUE | Commented installs satisfy bootstrap guard | `scripts/dev/github_workflows_test.go:activeStaticcheckInstallCounts` | Comment-stripped active-command count |
| 3 | ISSUE | Runner guard misses valid syntax | `internal/test/runner/runner_exec_util_test.go:unexpectedOSEnvironCalls` | Full type-resolved AST, alias, dot, shadow, and parenthesis cases |
| 4 | ISSUE | Timeout leaves descendants | `scripts/checks/staticcheck_feature_matrix.go:judgeStaticcheckFeatureMatrix` | Process-group kill, direct wait, pipe-lifetime regression |
| 5 | ISSUE | Expired context replaces completed verdict | `judgeStaticcheckFeatureMatrix` | Completed exit 0/1 classified before deadline failures |
| 6 | ISSUE | QEMU and tracked-build docs overclaim coverage | `mk/test-integration.mk`; `docs/architecture/testing/tracked-build-gate.md` | Non-race wording and six-configuration boundary |
| 7 | ISSUE | PID liveness cannot distinguish zombies | `scripts/checks/staticcheck_feature_matrix_test.go` | Descendant-held-pipe evidence |
| 8 | ISSUE | Make-entry test rewrite reduced direct proof | `scripts/checks/checks_test.go:TestStaticcheckFeatureMatrixMakeTargetRunsExecutable` | Explicit rejection of incomplete verdict output |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `scripts/checks/staticcheck_feature_matrix.go` | Yes | Read and hashed by Review Gate |
| `scripts/checks/staticcheck_feature_matrix_test.go` | Yes | Focused package command passed |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-4 | Exact N+2 rows detect broken production and test symbols and accept valid inputs | Focused `scripts/checks` command passed; live target checked 38 rows |
| AC-5 to AC-7 | Inputs and tool outcomes fail closed; matrix syntax and verdict precedence hold | Vacuity, failure, and backpressure tests passed |
| AC-8 to AC-10 | Both verification modes, structural refusal, exact pin, and tracked-build ordering remain live | `scripts/status`, `scripts/dev`, and Python focused commands passed |
| AC-11 | Developer and agent discovery is source-aware | `make ze-repository-check` passed |
| AC-12 | First-party non-race zero-CGO and test-only race exception hold | CGO, runner, CodeQL, codegen, and appliance focused commands passed |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-staticcheck-feature-matrix-check` | N-A, developer gate | `TestStaticcheckFeatureMatrixMakeTargetRunsExecutable` passed |
| Full and changed verification | N-A, stage inventory | `TestVerifyStagesIncludeStaticcheckFeatureMatrix` passed |
| Commit preparation after matrix red | N-A, commit-helper guard | `test_staticcheck_feature_matrix_is_structural` passed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Owner-selected bare-core and single-feature-off population |
| A-2 | confirmed | Broken provider/consumer fixture discriminates retained consumer |
| A-3 | confirmed | Staticcheck 2026.1 with Go 1.26 checked all 38 live rows |
| A-4 | confirmed | Controlled warm 183.51s and isolated cold 246.85s runs fit the 25-minute deadline |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Developer setup | `scripts/dev/dev-setup.py:REQUIRED_TOOLS`, `probe_tool` | Exact 2026.1 tool and target documented |
| Test infrastructure | `buildFeatureMatrix`, `runStaticcheckFeatureMatrix` | N+2 population, tests, direct rerun, and multi-off boundary documented |
| Architecture | `scripts/status/verify_run.go:stagesForMode`, tracked-build checker | Working-tree type-check and six-configuration final-link boundary documented |
| Agent discovery | `Makefile:ze-staticcheck-feature-matrix-check` | `ai/INDEX.md` routes the target |
| Runtime feature/config/CLI/API/plugin/wire/RFC/metrics categories | No runtime, protocol, config, API, plugin, or metric producer changed | N-A |
| Documentation gates | Matrix doc drift and repository check passed | Broad doc gate remains red only for foreign RFC 4271 and RFC 7947 producers |
