# Spec: fixit-lint-blind-to-every-other-build-tag

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | 0/4 (research) |
| Deferral shard | `-` |
| Updated | 2026-08-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Provenance

`spec-fixit-lint-blind-to-integration-tag` closed on 2026-08-23 having fixed ONE
tag. Its assumption A-5 ("`integration` is the only test-capability tag the
linter is blind to") was recorded BROKEN with evidence, and the remainder was
scoped out of that spec deliberately: each remaining tag names a different,
mutually exclusive build, so they cannot share one tag list. The class row is
`plan/journal/gate-excludes-part-of-its-population.md`, 2026-08-15.

This spec owns that remainder. Every number below was measured against the
working tree on 2026-08-23, not copied from the closed spec.

## Task

**Two golangci-lint passes now run, and 144 tracked own-source Go files are still
reported on by nothing.** A file outside the analysed build is not merely
unchecked: the gate exits 0 and reads as clean over it.

Measured 2026-08-23. Population is `git ls-files '*.go'` minus `vendor/`, minus
the 25 files under `gokrazy/modcache/` (a tracked third-party module cache),
minus the 29 files carrying `//go:build ignore` (legitimately excluded). What
the two passes load was derived by running `go list` under each pass's exact
GOOS and tag set over `ZE_LINT_PKGS`, and subtracting.

The 144 fall into three causes, and they need three different fixes.

### Cause 1 -- the package set names four roots, and the tree has more (55 files)

`ZE_LINT_PKGS := ./cmd/ze/... ./internal/... ./pkg/... ./test/...` (`Makefile`).
Every file below carries NO build constraint at all. It is ordinary Go that the
linter is simply never pointed at.

| Root | Files | What lives there |
|------|-------|------------------|
| `scripts/` | 44 | `scripts/checks/` 15, `scripts/dev/` 13, `scripts/status/` 8, `scripts/codegen/` 3, and five smaller dirs. These are the repository's own GATES |
| `cmd/ze-gok/` | 3 | the gokrazy image build wrapper |
| `cmd/ze-serial-shell/` | 2 | the appliance serial console |
| `api/proto/` | 2 | generated protobuf surface |
| `examples/plugin/go/` | 1 | the plugin SDK example an integrator copies |

`scripts/status/verify_run_test.go` is in that set. It holds
`TestLintCoversIntegrationTaggedFiles`, the guard that pins the integration
pass. The gate that proves the lint reaches the integration files is itself
unlinted.

### Cause 2 -- the lint build sets every feature gate ON, so no compile-out stub is loaded (47 files)

A `//go:build !ze_<feature>` file is the stub an operator reaches when they build
WITHOUT that feature: the "not built" message, the no-op registration, the
alternative dispatch. `.golangci.yml` `build-tags` is `ze_core` plus every gate,
so every one of those stubs is excluded from both passes.

`cmd/ze/hub` alone holds 25 of them, one per feature, plus two multi-feature
stubs (`!ze_isis && !ze_ldp && !ze_ospf && !ze_rsvpte && !ze_vrrp && !ze_bgp`,
and a twelve-tag plugin one).

This cause was named nowhere before this spec. The closed spec's A-5 evidence
listed positive personality tags only.

### Cause 3 -- personality and capability tags neither pass carries (42 files)

| Tag | Files | What it is |
|-----|-------|-----------|
| `ze_installer` (with `ze_installer_fault`) | 13 | the appliance installer's PID 1 and its disk logic: `cmd/ze-installer/`, `internal/install/disk/` |
| `ze_distro` | 5 | the distro build's dispatch |
| `ze_setup` (four constraint shapes) | 4 | `ze appliance ...`, the host tool |
| `ze_appliance` | 3 | the appliance build's dispatch |
| `debug` | 3 | debug-only instrumentation |
| `ze_chaos`, `live`, `race` | 2 each | orchestrator, live-network tests, race-only code |
| `ze_test`, `ze_perf`, `ze_analyze`, `zetest`, `stress`, `maprib`, `fleetperf`, `tinygo`, `gokrazy`, `tools` | 1 each | program flavors and one-off capabilities |

**`ze_installer` is the sharpest instance, and the reason is precise.** It IS
compiled, vetted and unit-tested: `mk/test-unit.mk` runs
`GOOS=linux go test -tags 'ze_core ze_installer' ./internal/install/...` on
Linux and `go vet` with the same tags elsewhere, `scripts/checks/tracked_build.go`
builds a `ze_installer` flavor, and `mk/test-integration.mk` executes the suite
under QEMU. What has never happened is a LINT pass loading it. This is shipped
code that runs as PID 1 on an appliance, and `errcheck`, `noctx`, `staticcheck`
and `gosec` have never seen a line of it.

### Cause 4 -- GOARCH, which no pass varies (a floor, not a count)

Pass 2 sets `GOOS=linux` and leaves GOARCH at the host's.
`internal/plugins/policyroute/netlinkint_linux_amd64.go` is excluded on an arm64
dev machine by its filename alone. `//go:build linux && !amd64 && !arm64`,
`//go:build !amd64`, `freebsd`, `!unix` and `!linux && !darwin` are each in the
144 for the same reason. This spec does not commit to fixing the arch axis; it
must state what it leaves.

### The evidence that the exclusion is total, not partial

`make ze-lint ZE_LINT_PKGS=./cmd/ze-installer/...` exits 5 with
`Running error: context loading failed: no go files to analyze`. golangci-lint is
LOUD when a package has no buildable file, which is what makes the silence over
the 144 a package-set and tag problem rather than a linter one: those files sit
in packages that DO build, so their exclusion is invisible.

The same probe settles the design constraint the fix inherits. Running the second
pass over `./internal/component/telemetry/exporter/...`, whose four files all
require `//go:build ze_telemetry`, reports `0 issues` and exits 0. Under
replace semantics it would have reported `no go files to analyze` as the
installer probe did. So `--build-tags` ADDS to the `.golangci.yml` list
(golangci-lint v2.10.1, measured on this tree), and a new pass inherits every
gate tag whether it wants them or not.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/repo-maintenance.md` - never edit a generated file
  → Constraint: `.golangci.yml` `build-tags` is generated by `scripts/codegen/feature_tags.go`. Change the generator, never the YAML
- [ ] `ai/rules/quality.md` - MUST FIX lint issues, MUST NOT disable linters
  → Constraint: the burn-down each new pass produces is fixed, never excluded. Only `fieldalignment` and the named test-file exclusions are allowed
- [ ] `ai/rules/commands.md` - the lint gate and what it costs
  → Constraint: whatever lands must reach `ze-lint-changed` too, or a new file still lands unlinted. The point file is `ai/rules/points/commands/lint-gate/what-ze-lint-changed-covers-and-what-it-costs.md`
- [ ] `ai/rules/plugins.md` - `feature-gates.txt` holds feature gates only
  → Constraint: none of the tags above is a feature gate. None goes in that manifest
- [ ] `plan/spec-shared-machine-job-admission.md` - the lint is the heaviest job in the repo
  → Constraint: two passes are already about 18 minutes of a 20-minute full verify. A third, fourth and fifth pass is a cost this spec must measure before it chooses a shape

**Key insights:** (minimal context to resume after compaction)
- 144 files, three causes, three different fixes. A single extra tag list fixes none of them: `ze_distro` against `ze_appliance` are mutually exclusive, and a negated stub is excluded by the tag being PRESENT.
- The negated-gate stubs (cause 2) were never named before. 47 files, 25 of them in `cmd/ze/hub`.
- `--build-tags` ADDS to the config list. A new pass cannot UNSET `ze_core` or a gate tag from the command line, which is exactly what cause 2 needs. That is the hard part of this spec.
- `ze_installer` is compiled, vetted and unit-tested, and never linted. Say it that way; "no gate has ever seen it" is false and checkable.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `Makefile` - `ZE_LINT_PKGS` at `:650`, the `ze-lint` / `_ze-lint-impl` pair, the `ze-lint-changed` / `_ze-lint-changed-impl` pair and its `&&` chaining
- [ ] `scripts/status/verify_run_test.go` - `integrationLintPass`, `TestLintCoversIntegrationTaggedFiles`, `TestChangedLintCoversIntegrationTaggedFiles`, `recipeBody` and `withDelegatedRecipes` (a guard for a new pass extends these)
- [ ] `scripts/codegen/feature_tags.go` - owns the generated `build-tags` list; `rewriteGolangci`
- [ ] `mk/test-unit.mk` - the `ze_installer` unit-test flavor, which is the model for what a per-personality pass must select
- [ ] `scripts/checks/tracked_build.go` - already enumerates six build flavors. A lint pass per flavor may belong beside it rather than in the Makefile

**Behavior to preserve:**
- `.golangci.yml` stays generated and hand-untouched.
- `feature-gates.txt` keeps holding feature gates only.
- No lint exclusion is added anywhere (`ai/rules/quality.md`).
- The two existing passes and their two guard tests keep working unchanged.

**Behavior to change:**
- Files outside `ZE_LINT_PKGS` are linted.
- Personality-tagged files are linted, each under a build that selects them.
- Compile-out stubs are linted under a build where their feature is OFF.
- A guard test refuses the removal of each new pass, as the two existing ones do.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A developer runs `make ze-lint` or `make ze-lint-changed`, or the verify pipeline runs the lint stage (`stagesForMode`, `scripts/status/verify_run.go`).

### Transformation Path
1. `_ze-lint-impl` runs pass 1 with the host GOOS and `.golangci.yml` tags.
2. It runs pass 2 with `GOOS=linux --build-tags integration`.
3. golangci-lint loads, per pass, only the packages under `ZE_LINT_PKGS` that build under that GOOS and tag set.
4. Everything else exits 0 and is reported clean.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Manifest ↔ generator | `feature-gates.txt` read by `feature_tags.go` | No |
| Generator ↔ config | `rewriteGolangci` writes `.golangci.yml` | No |
| Config ↔ linter | `--build-tags` ADDS to the config list | **Yes**, probe recorded in Task |
| Recipe ↔ guard test | `recipeBody` follows `$(MAKE)` delegation into `_ze-lint-impl` | **Yes**, `withDelegatedRecipes` |

### Integration Points
- `Makefile` lint recipes, or a new `mk/` fragment if the pass count makes one recipe unreadable.
- `scripts/status/verify_run.go` stage list, if a new target is added rather than the two extended.
- `ai/rules/points/commands/lint-gate/`, which states what the changed-file lint covers and what it costs.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `--build-tags` adds to the config list rather than replacing it | measured 2026-08-23, two probes recorded in Task | a new pass loses every gate tag and reports `no go files to analyze` | re-run both probes on the golangci-lint in use | confirmed |
| A-2 | A pass cannot UNSET a tag the config sets, so cause 2 needs a different mechanism from causes 1 and 3 | `--build-tags` is additive (A-1); golangci-lint v2.10.1 has no `--no-build-tags` | one extra pass fixes cause 2 too and the spec is smaller | search `golangci-lint run -h` and the v2 config schema for a subtractive form | unvalidated |
| A-3 | Every file in the 144 COMPILES under the build that would select it | not checked | a pass has to fix compile errors before a lint finding is visible | `go vet` per flavor, as `mk/test-unit.mk` already does for `ze_installer` | unvalidated |
| A-4 | The finding count over the 144 is comparable to the 132 the integration tag hid | not checked | the burn-down is much larger and needs staging against a recorded baseline | run each candidate pass uncapped and count | unvalidated |
| A-5 | Adding passes does not push the verify wall time past what a session will tolerate | two passes measured at ~18 minutes of a 20-minute full verify (`Makefile` comment) | the gate is disabled by whoever waits for it | measure each new pass warm and cold before choosing the shape | unvalidated |
| A-6 | `scripts/` is unlinted because of the package set alone, not because it fails to lint | 55 files carry no build constraint | `scripts/` carries findings that block the simplest fix | run one pass over `./scripts/...` and count | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Five or six sequential passes make the lint stage the whole verify | the stage time doubles | measure first; consider one pass per flavor in parallel under the existing admission point, not five in series |
| R-2 | The burn-down is large enough to stall the gate landing | phase 3 runs long | land each pass against a recorded baseline and burn it down. Never add an exclusion |
| R-3 | Cause 2 has no mechanism and the spec quietly drops it | the design phase proposes passes for causes 1 and 3 only | A-2 is a named assumption with a validation method. Answering it is a phase-2 deliverable, not a footnote |
| R-4 | A per-flavor pass duplicates the flavor list already in `scripts/checks/tracked_build.go` | two lists of build flavors drift | derive one from the other, or move the lint pass beside the build flavor it mirrors |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing at run time. This is lint configuration and build tooling. A wrong generator change breaks `make generate` and `dep_audit.py --check`, which is loud and immediate. A wrong burn-down edit can break a test, which its own suite catches |
| How is it reverted? | Per-pass revert. No config migration, no wire-visible change |
| Who else touches this path? | Any spec adding a feature gate touches `feature_tags.go`. `spec-shared-machine-job-admission` owns the job slots the lint runs in. `spec-verify-scope-2-change-set-selector` derives its package graph from the same tag set |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-lint` | → | one pass per cause, shape TBD in phase 2 | a guard test per pass, beside `TestLintCoversIntegrationTaggedFiles` |
| `make ze-lint-changed` | → | the same passes over `scripts/dev/changed-pkgs.sh` output, `&&`-chained | a guard test per pass, beside `TestChangedLintCoversIntegrationTaggedFiles` |
| `make generate` | → | `rewriteGolangci` tag list, UNCHANGED by this spec | `make ze-feature-tags-check` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-1 | A deliberate lint violation is added to a file under `scripts/`, `cmd/ze-gok/`, `cmd/ze-serial-shell/`, `api/proto/` or `examples/plugin/go/` | `make ze-lint` reports it | |
| AC-2 | A deliberate lint violation is added to a `//go:build ze_installer` file | `make ze-lint` reports it | |
| AC-3 | A deliberate lint violation is added to a `//go:build !ze_<feature>` stub | `make ze-lint` reports it, or the spec states the mechanism that makes this unreachable and the owner has ruled on it | |
| AC-4 | Each violation above, with only that file changed | `make ze-lint-changed` reports it | |
| AC-5 | `make generate` runs after the change | `.golangci.yml` still lists `ze_core` plus every feature-gate tag, in manifest order, and `git diff --exit-code .golangci.yml` is clean | |
| AC-6 | `feature-gates.txt` is read after the change | It contains none of the tags this spec adds | |
| AC-7 | The full lint runs against the tree | Zero findings remain in the newly reached files, measured UNCAPPED (`--max-issues-per-linter=0 --max-same-issues=0`), or the remainder is a recorded baseline that is burning down. No exclusion is added to `.golangci.yml` | |
| AC-8 | The blind-population script from this spec is re-run after the change | The 144 has fallen to the files this spec explicitly leaves (the GOARCH axis, and anything the owner ruled out), and the spec names that number | |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Edits the installer initrd's PID 1 and runs the changed-file lint | edit → `make ze-lint-changed` → a pass that selects `ze_installer` | the AC-2/AC-4 guard test |
| 2 | Edits a repository gate under `scripts/checks/` and runs the changed-file lint | edit → `make ze-lint-changed` → a pass whose package set names `./scripts/...` | the AC-1/AC-4 guard test |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| one guard per new pass, named for the population it covers | `scripts/status/verify_run_test.go` | AC-1..AC-4 | not written |
| the blind-population derivation, as a check rather than a one-off script | phase 2 decides the home | AC-8 | not written |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| lint passes in one recipe | 2..N | N | N/A | the count at which the verify stage time is unacceptable (R-1, measure it) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | | Tooling-only spec: the driving surface is the make target and its guard test, not a `.ci` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | | | Tooling change, no wire-visible behavior and no peer involved | |

## Files to Modify
- `Makefile` - `ZE_LINT_PKGS` and the two lint recipes
- `scripts/status/verify_run_test.go` - a guard test per new pass
- `ai/rules/points/commands/lint-gate/what-ze-lint-changed-covers-and-what-it-costs.md` - the pass count and the cost
- the files carrying the burn-down findings, count unknown until phase 1

## Files to Create
- None planned. A `mk/` fragment only if the pass count makes the Makefile recipe unreadable

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Tooling change |
| YANG validation constraints | N-A | Tooling change |
| YANG custom validators | N-A | Tooling change |
| CLI commands/flags | N-A | No `ze` command changes |
| CLI grammar (keyword before value) | N-A | No command changes |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | N-A | No new RPC |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | No new counter |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Developer tooling only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `ai/rules/commands.md` point file, and `docs/contributing/testing.md` where it enumerates the build flavors a gate covers |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for `feature_tags.go`, `.golangci.yml` and `tracked_build.go` at design time |
| 17 | Existing docs show config/CLI/API examples for this area? | | Check `docs/contributing/testing.md` at design time |

## Implementation Steps

1. **Phase: Measure** - re-derive the blind population and size each cause's burn-down
   - Tests: none yet. This phase produces the baseline and answers A-3, A-4, A-6
   - Files: a recorded finding list per cause
   - Verify: the 144 is re-derived from the tree rather than read from this spec, and each cause carries an uncapped finding count
2. **Phase: Choose the shape** - answer A-2 first, because it decides whether cause 2 is reachable at all
   - Tests: none yet
   - Files: the decision lands in Key Design Decisions with the rejected alternatives
   - Verify: the choice cites the measured cost from phase 1 and the wall time from A-5. If cause 2 has no mechanism, that goes to the owner as "which way", never as "may I skip it"
3. **Phase: Wiring** - land one pass at a time, each with its guard test
   - Tests: a guard per pass
   - Files: the Makefile recipes and `scripts/status/verify_run_test.go`
   - Verify: a deliberate violation in each newly reached population is reported by BOTH lint paths
4. **Phase: Burn down** - fix the findings each pass now sees
   - Tests: the existing suites stay green; `mk/test-unit.mk`'s installer flavor stays green
   - Files: the files carrying findings
   - Verify: AC-7 and AC-8. No lint exclusion is added

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | Both the full lint AND the changed-file lint are covered, for every new pass |
| Correctness | `.golangci.yml` was never hand-edited. Confirm by running `make generate` and seeing no diff |
| Population | The claim "X is now linted" is verified by ENUMERATING X and confirming the pass loads each file, not by observing that a pass exists |
| Data flow | `feature-gates.txt` still holds feature gates only |
| Rule: `ai/rules/repo-maintenance.md` | The generated file changed only as generator output |
| Rule: `ai/rules/quality.md` | No linter disabled and no exclusion added to reach green |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Each newly reached population is linted | Add a deliberate violation, run `make ze-lint`, confirm it is reported, restore the file from a pristine copy saved first |
| Changed-file path covered | Same violation, run `make ze-lint-changed` |
| Generator still correct | `make generate` then `git diff --exit-code .golangci.yml` |
| Drift check intact | `python3 scripts/dev/dep_audit.py --check` |
| No exclusions added | `git diff .golangci.yml` shows no new entry under an exclusion key |
| The remainder is stated | AC-8: the new blind count, and what this spec deliberately leaves |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Newly linted shipped code | `internal/install/disk` runs as PID 1 with disk-write privileges. The first `gosec` and `errcheck` pass over it may report real findings; each is a defect to fix, not a directive to silence |
| Fail-open guard | A pass whose package set silently resolves to nothing exits 0. Every new pass must fail LOUDLY on an empty population, as `no go files to analyze` already does |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error under a new flavor | Fix in the phase that introduced it. A-3 exists to find these early |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A cause has no mechanism | Ask the owner which way to fix it. Never present dropping the cause as an option |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| (phase 2) | | |

## Known Limitations
- The GOARCH axis is named and not committed to. Phase 2 decides whether it is in scope.
- `gokrazy/modcache/` (25 tracked third-party files) is excluded from the 144 on purpose. It is a vendored module cache, not Ze's source.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
