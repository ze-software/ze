# Spec: fixit-lint-blind-to-every-other-build-tag

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 4/4 (burn-down done; review round 1 closed; AC-3 waits on an owner ruling) |
| Deferral shard | `-` |
| Updated | 2026-08-24 |

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
- **The 144 is a DARWIN number. Re-measured on Linux, 2026-08-24: 245.** Pass 1 uses the HOST GOOS, so on a Linux machine both passes are `GOOS=linux` and every `!linux`, `darwin`, `freebsd` and `!unix` file is blind: 101 more files. Cause 4 named GOARCH and did not name GOOS.
- The subtractive mechanism cause 2 needs EXISTS: `-c <config>` with a copy of `.golangci.yml` whose `build-tags` list is empty, plus the whole tag set on the command line. `relative-path-mode: gitroot` keeps its reported paths equal to the base passes'. `setup-standalone` (`ze_setup && !ze_core`) proves it, at 0 findings.
- What blocks cause 2 is NOT the mechanism. It is `unused`: over `./cmd/ze/hub` a bare-`ze_core` build reports 67, of which 20 are real and 47 are symbols whose only consumers are compiled out.

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
- Files outside `ZE_LINT_PKGS` are linted. **Done**: the pattern is `./...`.
- Personality-tagged files are linted, each under a build that selects them. **Done**: 14 flavor rows, each scoped to the packages it alone reaches.
- Compile-out stubs are linted under a build where their feature is OFF. **NOT done**: the mechanism exists and `unused` blocks it (AC-3).
- A guard test refuses the removal of each new pass, as the two existing ones do. **Done**, and at the level that matters: one guard refuses the removal of the DRIVER, and the driver's own coverage assertion refuses the removal of any row in it, because the files that row alone reaches become blind and the run fails.

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
| No bypassed layers (data flows through the intended path) | Yes | Every golangci-lint run still goes through the Makefile: the base passes through `ZE_LINT_RUN`, the flavor passes through `ZE_LINT_FLAVOR_RUN`, which hands the driver the same `-j` and GOMEMLIMIT. `TestLintFlavorDriverCarriesTheLinterCeilings` pins that |
| No unintended coupling (components stay isolated) | Yes | The driver reads `.golangci.yml`, `git ls-files` and `go list`. It writes only under `tmp/`, and removes what it writes |
| No duplicated functionality (extends existing, does not recreate) | Yes | The two existing passes stay where they are and are declared, not re-implemented (`BASE_PASSES`). The flavor table does not copy `buildMatrix` (`scripts/checks/tracked_build.go`): it is derived from the tree, which is what R-4 asked for |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Developer tooling, no wire path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No feature is named in a core package by this change. The one Go edit that touches dispatch, `chaosRun` in `cmd/ze/ze_chaos_main_test.go`, replaces a direct call with `registry.LookupRoot("chaos")`, which is the registration path |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `--build-tags` adds to the config list rather than replacing it | measured 2026-08-23, two probes recorded in Task | a new pass loses every gate tag and reports `no go files to analyze` | re-run both probes on the golangci-lint in use | confirmed |
| A-2 | A pass cannot UNSET a tag the config sets, so cause 2 needs a different mechanism from causes 1 and 3 | `--build-tags` is additive (A-1); golangci-lint v2.10.1 has no `--no-build-tags` | one extra pass fixes cause 2 too and the spec is smaller | search `golangci-lint run -h` and the v2 config schema for a subtractive form | **broken, and in the direction that helps**. `run -h` offers no subtractive flag, but `-c PATH` takes a whole config, so a copy of `.golangci.yml` with an EMPTY `build-tags` makes the command line the whole tag set. `tagless_config` (`scripts/dev/lint_flavors.py`) derives that copy on every run instead of tracking a second file, and adds `relative-path-mode: gitroot` so its findings name the same paths the base passes do. The mechanism is proven by the `setup-standalone` flavor, which reaches `ze_setup && !ze_core` and reports 0 |
| A-3 | Every file in the 144 COMPILES under the build that would select it | not checked | a pass has to fix compile errors before a lint finding is visible | `go vet` per flavor, as `mk/test-unit.mk` already does for `ze_installer` | **broken, twice**. `-tags live` did not compile: `rpki_live_test.go` and `rtr_downgrade_test.go` called `NewROACache`, `NewASPACache`, `NewRTRSession` and `CheckPair`, which were unexported at some point and have not existed since. `-tags tinygo` did not compile either: `cmd/ze/main_test.go` carries `//go:build ze_core` and NOT `!tinygo`, while the `isLocalhostPprof` it tests is defined only by the `!tinygo` file, so `-tags 'ze_core tinygo'` selected the test and dropped the function. Both fixed here; both are the same shape, a file nothing ever built |
| A-4 | The finding count over the 144 is comparable to the 132 the integration tag hid | not checked | the burn-down is much larger and needs staging against a recorded baseline | run each candidate pass uncapped and count | **confirmed, and far smaller**. Uncapped: installer 10, distro 1, appliance 1, capability 1 (the typecheck above), setup 0, setup-standalone 0, `./scripts/...` 0. No baseline was needed |
| A-5 | Adding passes does not push the verify wall time past what a session will tolerate | two passes measured at ~18 minutes of a 20-minute full verify (`Makefile` comment) | the gate is disabled by whoever waits for it | measure each new pass warm and cold before choosing the shape | **confirmed, because each flavor is SCOPED**. A full-tree pass is 286s cold (`GOOS=darwin`, measured); a flavor pass over the packages it alone reaches is 8-39s. The whole 13-flavor stage, including its 13 `go list` calls, is about 36s of derivation plus the passes |
| A-6 | `scripts/` is unlinted because of the package set alone, not because it fails to lint | 55 files carry no build constraint | `scripts/` carries findings that block the simplest fix | run one pass over `./scripts/...` and count | **confirmed**. `./scripts/... ./cmd/ze-gok/... ./cmd/ze-serial-shell/... ./api/... ./examples/...` uncapped: 8s, 0 findings. `ZE_LINT_PKGS := ./...` was the whole fix, and it immediately caught a misspelling in this spec's own new test comment |
| A-7 | The blind population is a property of the TREE | assumed by the spec's own 144, measured on darwin | the number, and which files are covered, differ per machine, and a Linux CI is blind where a darwin dev machine is not | re-derive the population on Linux | **broken**. 245 on Linux against 144 on darwin, because pass 1 takes the host GOOS and pass 2 pins `GOOS=linux`. Fixed by naming darwin, windows and freebsd as flavor rows, so the union is the same on every machine |

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
| `make ze-lint` | → | `$(ZE_LINT_FLAVOR_RUN)` in `_ze-lint-impl`, which runs `FLAVORS` and then asserts coverage | `TestLintRunsEveryBuildFlavor` (`scripts/status/verify_run_test.go`), which also pins `ZE_LINT_PKGS` at `./...` |
| `make ze-lint-changed` | → | the same driver with `--scope "$pkgs"`, `&&`-chained after both existing passes | `TestChangedLintRunsEveryBuildFlavor`, which also refuses a `;` anywhere in the chain |
| `make ze-lint-changed`, on an edit under `cmd/ze-installer` | → | `uncompiledTreeReaders` (`scripts/checks/verify_scope_selector.go`) rules that directory no longer, so the change set widens to `./...` and the driver's `installer` flavor selects the package | `TestSelectorWidensForTheInstallerInitrd` (`scripts/checks/verify_scope_selector_test.go`) |
| `make ze-lint` | → | the driver carries the linter's two ceilings, because it runs golangci-lint itself | `TestLintFlavorDriverCarriesTheLinterCeilings` |
| `_ze-lint-impl` | → | `BASE_PASSES` (`scripts/dev/lint_flavors.py`), the two passes every flavor's scope subtracts | `test_base_passes_match_the_lint_recipe` (`scripts/dev/lint_flavors_test.py`) |
| `make generate` | → | `rewriteGolangci` tag list, UNCHANGED by this spec | `make ze-feature-tags-check` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-1 | A deliberate lint violation is added to a file under `scripts/`, `cmd/ze-gok/`, `cmd/ze-serial-shell/`, `api/proto/` or `examples/plugin/go/` | `make ze-lint` reports it | **MET, unplanned.** The first `make ze-lint` after `ZE_LINT_PKGS := ./...` failed on a `misspell` finding in `scripts/status/verify_run_test.go`, in a comment this spec had just written in a file no pass had ever loaded. No violation had to be planted. `examples/plugin/go/` is NOT covered: see Known Limitations |
| AC-2 | A deliberate lint violation is added to a `//go:build ze_installer` file | `make ze-lint` reports it | **MET.** `behaviour` added to a comment in `internal/install/disk/bootstrap_linux.go`, restored from a pristine copy afterwards (`git diff` clean). `make ze-lint` reports the `misspell` finding from both the `installer` and `installer-nofault` flavors and exits 2 |
| AC-3 | A deliberate lint violation is added to a `//go:build !ze_<feature>` stub | `make ze-lint` reports it, or the spec states the mechanism that makes this unreachable and the owner has ruled on it | **NOT MET. The owner's ruling is owed.** The mechanism is stated and proven (`tagless_config`, and the `setup-standalone` flavor that uses it), so the row that covers all 34 stubs is one entry carrying `"without": <every gate>`. What blocks it is `unused`: over `./cmd/ze/hub` that build reports 67, of which 20 are real and 47 name symbols whose only consumers it compiles out. The two ways to fix it are in `COMPILE_OUT_REASON` (`scripts/dev/lint_flavors.py`) |
| AC-4 | Each violation above, with only that file changed | `make ze-lint-changed` reports it | **MET, at the case that was failing.** The first evidence used `--scope "./internal/install/disk"`, a package the HOST build reports, so it never exercised the entry point. `cmd/ze-installer` is the one package the host build cannot see at all (`go list -e ./cmd/ze-installer` answers "build constraints exclude all Go files", and it is the only such directory in the tree), and `scripts/checks/verify_scope_selector.go` used to answer an installer-only edit with `./scripts/checks ./scripts/dev` -- a scope the initrd's PID 1 is not in, so `ze-lint-changed` linted none of it and exited 0. `uncompiledTreeReaders` no longer rules that directory, so it widens. Measured with a `behaviour` misspelling planted in `cmd/ze-installer/main.go` and restored from a pristine copy afterwards (`git status` clean): the selector answers `./...`, and `python3 scripts/dev/lint_flavors.py --scope "./..." -j 8` -- the exact call `_ze-lint-changed-impl` then makes -- reports the `misspell` finding in `main.go` from the `installer` flavor and exits 1. `TestSelectorWidensForTheInstallerInitrd` pins the widening; `TestChangedLintRunsEveryBuildFlavor` pins the `&&`-chained driver call |
| AC-5 | `make generate` runs after the change | `.golangci.yml` still lists `ze_core` plus every feature-gate tag, in manifest order, and `git diff --exit-code .golangci.yml` is clean | **MET.** `make generate` then `git diff --exit-code .golangci.yml` is clean. Nothing in this spec writes that file: the tagless copy is derived under `tmp/` on each run |
| AC-6 | `feature-gates.txt` is read after the change | It contains none of the tags this spec adds | **MET.** Every tag in `FLAVORS` (`debug`, `race`, `live`, `stress`, `maprib`, `fleetperf`, `zetest`, `gokrazy`, `tinygo`, `ze_test`, `ze_perf`, `ze_analyze`, `ze_chaos`, `ze_installer`, `ze_installer_fault`, `ze_distro`, `ze_appliance`, `ze_setup`, `integration`, and `ze_core` under `without`) checked against the manifest: no entry. `stress` and `gokrazy` appear in its prose only. `tools` was listed here in error and is no flavor tag at all: it gates `tools.go`, which stays in `RESIDUE` |
| AC-7 | The full lint runs against the tree | Zero findings remain in the newly reached files, measured UNCAPPED (`--max-issues-per-linter=0 --max-same-issues=0`), or the remainder is a recorded baseline that is burning down. No exclusion is added to `.golangci.yml` | **MET for every flavor that runs.** Uncapped counts before the burn-down: installer 10, capability 15, distro 1, appliance 1, `scripts/` 0, setup 0, setup-standalone 0, darwin 0, freebsd 0, openbsd 0, dragonfly 0, wasip1 0, arm64 0, riscv64 0, tinygo 0. The capability 15 only became visible after three TYPECHECK failures in builds nothing had ever compiled were fixed (`live`, `stress`, `ze_chaos`): 12 `noctx`, 1 `errorlint`, 1 `modernize`, 1 `staticcheck`. All fixed, `git diff .golangci.yml` empty, no `//nolint` added except four that state why the finding is wrong for that line. The 34 stubs are not measured against zero because no pass runs them yet (AC-3) |
| AC-8 | The blind-population script from this spec is re-run after the change | The 144 has fallen to the files this spec explicitly leaves (the GOARCH axis, and anything the owner ruled out), and the spec names that number | **MET, with a corrected starting number: 245 on Linux, not 144 (A-7).** 245 to 36, and the 36 are stated by name or by class on every full run: `examples/plugin/go/main.go`, `tools.go`, and the 34 compile-out stubs AC-3 owes, whose count `COMPILE_OUT_STUBS` now caps in both directions so a 35th cannot join them in silence. Four files left the residue in review round 1. `internal/core/privilege/drop_other.go` is `!linux && !darwin && !freebsd && !openbsd && !netbsd`, which dragonfly satisfies, so the `dragonfly` row lints it; the reason recorded against it, that no unix GOOS selects it, did not hold. The three `//go:build !unix` fallbacks in `internal/core/crashlog` and `pkg/zefs` are reached by the `wasip1` row: `log/syslog` has no windows or plan9 implementation and `internal/core/slogutil` imports it unconditionally, but wasip1 is not unix and does build it, so the whole import graph type-checks and all three lint at 0. The GOARCH axis is IN and covered. Re-derive with `python3 scripts/dev/lint_flavors.py --coverage`, about 36s |

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
- `Makefile` - `ZE_LINT_PKGS` is `./...`, `ZE_LINT_FLAVOR_RUN` carries the ceilings, and both lint recipes run the driver
- `scripts/checks/verify_scope_selector.go` - `uncompiledTreeReaders` no longer rules `cmd/ze-installer`, so an edit to the initrd's PID 1 widens instead of answering with a scope no lint pass loads it under
- `scripts/checks/verify_scope_selector_test.go` - `TestSelectorWidensForTheInstallerInitrd`, and the installer case out of `TestSelectorMapsGoTreesTheUnitBuildNeverCompiles`
- `scripts/status/verify_run_test.go` - `flavorLintPass`, `lintPassLines`, `TestLintRunsEveryBuildFlavor`, `TestChangedLintRunsEveryBuildFlavor`, `TestLintFlavorDriverCarriesTheLinterCeilings`
- `mk/test-chaos.mk` - `CHAOS_CLI_TAGS` and the `./cmd/ze` run in `_ze-chaos-unit-test-impl`. The eleven `//go:build ze_chaos` tests in `cmd/ze/ze_chaos_main_test.go` were executed by nothing, and `chaosRun` now depends at RUNTIME on the `chaos` root handler being registered
- `cmd/ze/hub/fleet_perf_test.go` - the auth check reports a parse failure and a non-ok verb apart. One `%w` for both rendered `%!w(<nil>)` on the branch a real refusal takes
- `cmd/ze/pprof_test.go`, `cmd/ze/main_test.go`, `test/weakened.md` - the move's stated reason. `main_test.go` carries `//go:build ze_core`, not no constraint; what it lacks is `!tinygo`
- `ai/rules/points/commands/lint-gate/what-ze-lint-changed-covers-and-what-it-costs.md` - the pass count and the cost, and the one directory whose edit widens the gate to the whole tree
- `docs/contributing/testing.md` - "The builds the linter reads"
- `docs/architecture/testing/verify-freshness-scope.md` - "Which packages a scoped run judges": what `uncompiledTreeReaders` still rules, and why `cmd/ze-installer` left it
- Burn-down, all of it uncapped: `internal/install/disk/{bootstrap,console,rescue,fault}_linux.go`, `internal/install/disk/{fault,initrd}_linux_test.go` (10), `internal/component/bgp/plugins/rpki/{rpki_live,rtr_downgrade}_test.go` (13, mostly `noctx` -- every `exec.Command` is `exec.CommandContext` now, and `dockerRM` carries its own bound because a cleanup runs after `t.Context()` is canceled), `cmd/ze/hub/fleet_perf_test.go` (2), `internal/component/config/system/selfupdate.go` (1), `internal/plugins/flowexport/conntrack_setup_appliance_linux.go` (1), `scripts/status/verify_run_test.go` (1, this spec's own new comment)
- Compile fixes for builds nothing had ever compiled: `internal/component/bgp/plugins/rpki/{rpki_live,rtr_downgrade}_test.go` (`live`), `internal/component/web/stress_test.go` (`stress`), `cmd/ze/ze_chaos_main_test.go` (`ze_chaos`, now dispatching through `registry.LookupRoot`), `cmd/ze/main_test.go` (`tinygo`)
- `examples/plugin/go/main.go` - `pluginserver.CommandContext` is `plugin.CommandContext`; the example did not compile

## Files to Create
- `scripts/dev/lint_flavors.py` - the flavor table, the derived scopes, the passes, and the coverage assertion
- `scripts/dev/lint_flavors_test.py` - its unit tests, run by `go test ./scripts/dev/` through `python_tests_test.go`
- `cmd/ze/pprof_test.go` - `TestIsLocalhostPprof`, moved so it carries pprof.go's `!tinygo` constraint

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
| 10 | Test infrastructure changed? | Yes | `ai/rules/commands.md` point file, `docs/contributing/testing.md` where it enumerates the build flavors a gate covers, and `docs/architecture/testing/verify-freshness-scope.md` where it says which packages a scoped run judges |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/testing/verify-freshness-scope.md`, updated: it is the design doc `docs/` holds for the change-set selector, and "Which packages a scoped run judges" now says what `uncompiledTreeReaders` still rules and why `cmd/ze-installer` left it. `scripts/checks/verify_scope_selector.go` ALSO declares `plan/spec-verify-scope-2-change-set-selector.md` as its `// Design:`, and that spec is UNAFFECTED and not edited here: it owns the shape of the change set (one selector, a tag-aware reverse graph, depth 2, fail open on a seed it cannot classify), and this change adds no rule and removes no property -- it deletes one entry from the table of directories exempted from that spec's own fail-open path, so the selector now behaves as that spec already specifies. It is also another session's open spec, which this one must not edit |
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
| `ZE_LINT_PKGS := ./...` | naming the missing roots one by one (`./scripts/...`, `./cmd/ze-gok/...`, ...) | the four roots were the whole of cause 1, and a named list has the same defect one directory later. `ZE_PACKAGES`, the unit-test population, has always been `./...`, so the two now agree. 0 findings and 8s, so nothing was traded for it |
| One pass for each build, driven by `scripts/dev/lint_flavors.py`, with the package set DERIVED from the tree | a tag list in the Makefile; a manifest file both make and the guard test read; a lint row beside each `buildMatrix` flavor in `scripts/checks/tracked_build.go` | a written list drifts silently the moment a `//go:build debug` file lands in a new package, which is this spec's own defect class. Deriving it costs one `go list` for each flavor, about 36s in total, and makes drift impossible rather than detectable. R-4 is answered the same way: the lint table is derived from the TREE, so it cannot disagree with `buildMatrix` about anything that matters |
| Each flavor lints only the packages holding a file the base passes do not load | a full `./...` pass for each flavor | measured: a full-tree pass is 286s and a scoped flavor pass is 8-39s. Thirteen full passes would have made the lint stage the whole verify (R-1) |
| The driver ASSERTS coverage at the end of a whole-tree run | a separate `make ze-lint-population-check` target and verify stage | the driver already holds every `go list` answer the assertion needs, so the check is free where a separate stage would pay for its own derivation. It is skipped for a scoped run, where a missing file is missing because the caller said so |
| A flavor that must turn a tag OFF gets a DERIVED tagless config (`tagless_config`) | a second GENERATED `.golangci-*.yml` beside the first; hand-maintaining a second config | `--build-tags` only adds (A-1), so the only subtractive route is a whole config. Deriving it on every run from `.golangci.yml` keeps ONE linter set: a tracked second file drifts, and the linter set is exactly the thing that must not differ between two passes. It is written under `tmp/`, so `relative-path-mode: gitroot` is needed for its findings to name the same paths |
| The two existing passes stay written in the Makefile recipe | moving them into the driver as rows | "Behavior to preserve" asks for it, and the two existing guard tests read the recipe. `BASE_PASSES` declares them for subtraction only, and `test_base_passes_match_the_lint_recipe` fails when the recipe and that declaration disagree |
| Python, not Go, for the driver | a `//go:build ignore` program beside `scripts/checks/tracked_build.go` | `docs/contributing/ze-style.md` puts Ze's tooling in Python, and `scripts/dev/python_tests_test.go` already runs a `<tool>_test.py` inside `make ze-unit-test` with no make target of its own |
| An edit under `cmd/ze-installer` WIDENS the change set to `./...`, by deleting the rule that exempted it | naming `./cmd/ze-installer` in the narrow answer; a second print mode on the selector for a lint-only package list; a lint-only scope computed beside the shared one | the narrow answer is not available: the same list drives `ze-unit-test-changed`, and `go test ./cmd/ze-installer` under the unit tag set fails with "build constraints exclude all Go files". Both other routes add a channel and a second change-set consumer to carry one directory, and the rule being deleted had a stated reason -- "./... does not compile it either" -- that this spec's own work made false. Deleting it cuts machinery and puts the directory back on the selector's own documented fail-open path. Five commits have ever touched it, so the wide answer is paid about that often |
| A residue reason is a claim that gets CHECKED, not a note | leaving the four `!unix`-class entries with the reason first written for them | review round 1 found the `internal/core/privilege/drop_other.go` reason false: dragonfly, solaris, illumos and aix all satisfy `!linux && !darwin && !freebsd && !openbsd && !netbsd`, and every one is unix and buildable here. Checking the other three the same way found windows and plan9 are not the whole non-unix set either: wasip1 is not unix and DOES build `log/syslog`, so it type-checks the whole graph. Two rows, `dragonfly` and `wasip1`, cost one `go list` and three packages each and took the residue from 40 to 36 |

## Known Limitations
- **Cause 2 is NOT closed.** 34 compile-out stubs (`//go:build !ze_<feature>`) are still reached by no pass. The mechanism exists and is proven (`tagless_config`); what blocks it is `unused`, which over `./cmd/ze/hub` reports 47 symbols whose only consumers that build compiles out, beside 20 real findings. The two answers are in `COMPILE_OUT_REASON` (`scripts/dev/lint_flavors.py`) and the owner picks. Until then `--coverage` counts and prints them on every full run.
- `examples/plugin/go/` is a separate Go module with no tracked `go.sum`, so `./...` cannot reach it and a run in that directory needs `go mod tidy`, which this repository's vendored dependencies do not provide for. It is named in `RESIDUE` with what would have to change. Its one file also did not COMPILE (`undefined: pluginserver`); that is fixed here, and proved in a scratch copy of the module.
- `tools.go` cannot be type-checked by anything: the tools.go idiom imports PROGRAMS so `go mod tidy` pins them. It leaves the residue when those pins move to go.mod's `tool` directives.
- The GOARCH axis IS in scope and covered, by two rows: `linux-arm64` for the filename-suffixed netlink files, and `linux-other-arch` (riscv64) for `linux && !amd64 && !arm64`.
- The GOOS axis now runs to six rows. `darwin`, `freebsd` and `openbsd` were there from the first round; `dragonfly` and `wasip1` were added in review round 1, and each removed a residue entry whose stated reason did not hold. `windows` and `plan9` stay unreachable, and the reason is exact: `internal/core/slogutil` imports `log/syslog` with no build constraint and that package has no implementation on either, so every package reaching the logger fails to type-check. `wasip1` is not a target claim -- Ze ships no WASI binary -- it is the build under which the `!unix` fallbacks type-check.
- An edit to `cmd/ze-installer` widens the changed-file gate to `./...`, and that is the design rather than a fallback. Every file there is `linux && ze_installer`, so no `go list` under the unit tag set reports the package, and no narrower answer names anything a lint pass loads it under. `./cmd/ze-installer` cannot be put in the narrow answer instead: the same list drives `ze-unit-test-changed`, where `go test` on it fails with "build constraints exclude all Go files". Five commits have ever touched that directory.
- `gokrazy/modcache/` (25 tracked third-party files) is excluded from the population on purpose. It is a vendored module cache, not Ze's source.
- A `//go:build ignore` file (29 of them, every program under `scripts/checks/` among them) belongs to no build by its own declaration and can never be linted by any pass. `population` states that.

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
