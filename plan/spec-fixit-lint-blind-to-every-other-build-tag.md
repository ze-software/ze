# Spec: fixit-lint-blind-to-every-other-build-tag

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 4/4, closed. Review round 4 is clean. The two open symbols were ruled on by Thomas on 2026-08-24 and fixed at `89c35d8ed` and `d580c0870`, so AC-7 is met and every flavor row reports 0 |
| Deferral shard | `-` |
| Updated | 2026-08-29 |

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

`ZE_LINT_PKGS := ./cmd/ze/... ./internal/... ./pkg/... ./test/...` (`internal/le/` native action tables).
Every file below carries NO build constraint at all. It is ordinary Go that the
linter is simply never pointed at.

| Root | Files | What lives there |
|------|-------|------------------|
| `internal/le/` | 44 | `internal/le/` 15, `internal/le/` 13, `internal/le/` 8, `internal/le/` 3, and five smaller dirs. These are the repository's own GATES |
| `cmd/ze-gok/` | 3 | the gokrazy image build wrapper |
| `cmd/ze-serial-shell/` | 2 | the appliance serial console |
| `api/proto/` | 2 | generated protobuf surface |
| `examples/plugin/go/` | 1 | the plugin SDK example an integrator copies |

`internal/le/verify/engine/verifyengine_test.go` is in that set. It holds
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
compiled, vetted and unit-tested: `internal/le/testunit/groups.go` runs
`GOOS=linux go test -tags 'ze_core ze_installer' ./internal/install/...` on
Linux and `go vet` with the same tags elsewhere, `internal/le/repository/trackedbuild/repositorytrackedbuild.go`
builds a `ze_installer` flavor, and `internal/le/integration/gates.go` executes the suite
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

`./le verify lint run ZE_LINT_PKGS=./cmd/ze-installer/...` exits 5 with
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
  → Constraint: `.golangci.yml` `build-tags` is generated by `internal/le/featuretags/featuretags.go`. Change the generator, never the YAML
- [ ] `ai/rules/quality.md` - MUST FIX lint issues, MUST NOT disable linters
  → Constraint: the burn-down each new pass produces is fixed, never excluded. Only `fieldalignment` and the named test-file exclusions are allowed
- [ ] `ai/rules/commands.md` - the lint gate and what it costs
  → Constraint: whatever lands must reach `./le changed scope` too, or a new file still lands unlinted. The point file is `ai/rules/points/commands/lint-gate/what-native-changed-lint-covers-and-costs.md`
- [ ] `ai/rules/plugins.md` - `feature-gates.txt` holds feature gates only
  → Constraint: none of the tags above is a feature gate. None goes in that manifest
- [ ] `docs/architecture/core-design.md` - the small core plus registration pattern, declared by the `// Design:` header of files this spec changes
  → Constraint: a new lint flavor is a row in a registered table, never a branch in the runner
- [ ] `docs/architecture/api/process-protocol.md` - the plugin process protocol, declared by the same headers
  → Constraint: named because this spec's own population declares it. It states no obligation the lint matrix depends on
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
- [ ] `internal/le/` native action tables - `ZE_LINT_PKGS` at `:650`, the `./le verify lint run` / `_./le verify lint run-impl` pair, the `./le changed scope` / `_./le changed scope` pair and its `&&` chaining
- [ ] `internal/le/verify/engine/verifyengine_test.go` - `integrationLintPass`, `TestLintCoversIntegrationTaggedFiles`, `TestChangedLintCoversIntegrationTaggedFiles`, `recipeBody` and `withDelegatedRecipes` (a guard for a new pass extends these)
- [ ] `internal/le/featuretags/featuretags.go` - owns the generated `build-tags` list; `rewriteGolangci`
- [ ] `internal/le/testunit/groups.go` - the `ze_installer` unit-test flavor, which is the model for what a per-personality pass must select
- [ ] `internal/le/repository/trackedbuild/repositorytrackedbuild.go` - already enumerates six build flavors. A lint pass per flavor may belong beside it rather than in the the native action tables under `internal/le/`

**Behavior to preserve:**
- `.golangci.yml` stays generated and hand-untouched.
- `feature-gates.txt` keeps holding feature gates only.
- No lint exclusion is added anywhere (`ai/rules/quality.md`).
- The two existing passes and their two guard tests keep working unchanged.

**Behavior to change:**
- Files outside `ZE_LINT_PKGS` are linted. **Done**: the pattern is `./...`.
- Personality-tagged files are linted, each under a build that selects them. **Done**: 17 flavor rows, each scoped to the packages it alone reaches.
- Compile-out stubs are linted under a build where their feature is OFF. **Done**: the `compile-out` row keeps `ze_core` alone and drops every gate, and each symbol the resulting `unused` findings named now carries its consumer's build constraint (AC-3).
- A guard test refuses the removal of each new pass, as the two existing ones do. **Done**, and at the level that matters: one guard refuses the removal of the DRIVER, and the driver's own coverage assertion refuses the removal of any row in it, because the files that row alone reaches become blind and the run fails.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A developer runs `./le verify lint run` or `./le changed scope`, or the verify pipeline runs the lint stage (`stagesForMode`, `internal/le/verify/engine/run.go`).

### Transformation Path
1. `_./le verify lint run-impl` runs pass 1 with the host GOOS and `.golangci.yml` tags.
2. It runs pass 2 with `GOOS=linux --build-tags integration`.
3. golangci-lint loads, per pass, only the packages under `ZE_LINT_PKGS` that build under that GOOS and tag set.
4. Everything else exits 0 and is reported clean.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Manifest ↔ generator | `feature-gates.txt` read by `feature_tags.go` | No |
| Generator ↔ config | `rewriteGolangci` writes `.golangci.yml` | No |
| Config ↔ linter | `--build-tags` ADDS to the config list | **Yes**, probe recorded in Task |
| Recipe ↔ guard test | `recipeBody` follows `$(MAKE)` delegation into `_./le verify lint run-impl` | **Yes**, `withDelegatedRecipes` |

### Integration Points
- `internal/le/` native action tables lint recipes, or a new `internal/le/` fragment if the pass count makes one recipe unreadable.
- `internal/le/verify/engine/run.go` stage list, if a new target is added rather than the two extended.
- `ai/rules/points/commands/lint-gate/`, which states what the changed-file lint covers and what it costs.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Every golangci-lint run still goes through the the native action tables under `internal/le/`: the base passes through `ZE_LINT_RUN`, the flavor passes through `ZE_LINT_FLAVOR_RUN`, which hands the driver the same `-j` and GOMEMLIMIT. `TestLintFlavorDriverCarriesTheLinterCeilings` pins that |
| No unintended coupling (components stay isolated) | Yes | The driver reads `.golangci.yml`, `git ls-files` and `go list`. It writes only under `tmp/`, and removes what it writes |
| No duplicated functionality (extends existing, does not recreate) | Yes | The two existing passes stay where they are and are declared, not re-implemented (`BASE_PASSES`). The flavor table does not copy `buildMatrix` (`internal/le/repository/trackedbuild/repositorytrackedbuild.go`): it is derived from the tree, which is what R-4 asked for |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Developer tooling, no wire path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No feature is named in a core package by this change. The one Go edit that touches dispatch, `chaosRun` in `cmd/ze/ze_chaos_main_test.go`, replaces a direct call with `registry.LookupRoot("chaos")`, which is the registration path |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `--build-tags` adds to the config list rather than replacing it | measured 2026-08-23, two probes recorded in Task | a new pass loses every gate tag and reports `no go files to analyze` | re-run both probes on the golangci-lint in use | confirmed |
| A-2 | A pass cannot UNSET a tag the config sets, so cause 2 needs a different mechanism from causes 1 and 3 | `--build-tags` is additive (A-1); golangci-lint v2.10.1 has no `--no-build-tags` | one extra pass fixes cause 2 too and the spec is smaller | search `golangci-lint run -h` and the v2 config schema for a subtractive form | **broken, and in the direction that helps**. `run -h` offers no subtractive flag, but `-c PATH` takes a whole config, so a copy of `.golangci.yml` with an EMPTY `build-tags` makes the command line the whole tag set. `tagless_config` (`internal/le/verify/lint/matrix.go`) derives that copy on every run instead of tracking a second file, and adds `relative-path-mode: gitroot` so its findings name the same paths the base passes do. The mechanism is proven by the `setup-standalone` flavor, which reaches `ze_setup && !ze_core` and reports 0 |
| A-3 | Every file in the 144 COMPILES under the build that would select it | not checked | a pass has to fix compile errors before a lint finding is visible | `go vet` per flavor, as `internal/le/testunit/groups.go` already does for `ze_installer` | **broken, twice**. `-tags live` did not compile: `rpki_live_test.go` and `rtr_downgrade_test.go` called `NewROACache`, `NewASPACache`, `NewRTRSession` and `CheckPair`, which were unexported at some point and have not existed since. `-tags tinygo` did not compile either: `cmd/ze/main_test.go` carries `//go:build ze_core` and NOT `!tinygo`, while the `isLocalhostPprof` it tests is defined only by the `!tinygo` file, so `-tags 'ze_core tinygo'` selected the test and dropped the function. Both fixed here; both are the same shape, a file nothing ever built |
| A-4 | The finding count over the 144 is comparable to the 132 the integration tag hid | not checked | the burn-down is much larger and needs staging against a recorded baseline | run each candidate pass uncapped and count | **confirmed, and far smaller**. Uncapped: installer 10, distro 1, appliance 1, capability 1 (the typecheck above), setup 0, setup-standalone 0, `./scripts/...` 0. No baseline was needed |
| A-5 | Adding passes does not push the verify wall time past what a session will tolerate | two passes measured at ~18 minutes of a 20-minute full verify (the retired `Makefile` (current producers: `internal/le/` native action tables) comment) | the gate is disabled by whoever waits for it | measure each new pass warm and cold before choosing the shape | **confirmed, because each flavor is SCOPED**. A full-tree pass is 286s cold (`GOOS=darwin`, measured); a flavor pass over the packages it alone reaches is 8-39s. The whole 13-flavor stage, including its 13 `go list` calls, is about 36s of derivation plus the passes |
| A-6 | the retired `scripts/` (current producer: `internal/le/`) is unlinted because of the package set alone, not because it fails to lint | 55 files carry no build constraint | the retired `scripts/` (current producer: `internal/le/`) carries findings that block the simplest fix | run one pass over `./scripts/...` and count | **confirmed**. `./scripts/... ./cmd/ze-gok/... ./cmd/ze-serial-shell/... ./api/... ./examples/...` uncapped: 8s, 0 findings. `ZE_LINT_PKGS := ./...` was the whole fix, and it immediately caught a misspelling in this spec's own new test comment |
| A-7 | The blind population is a property of the TREE | assumed by the spec's own 144, measured on darwin | the number, and which files are covered, differ per machine, and a Linux CI is blind where a darwin dev machine is not | re-derive the population on Linux | **broken**. 245 on Linux against 144 on darwin, because pass 1 takes the host GOOS and pass 2 pins `GOOS=linux`. Fixed by naming darwin, windows and freebsd as flavor rows, so the union is the same on every machine |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Five or six sequential passes make the lint stage the whole verify | the stage time doubles | measure first; consider one pass per flavor in parallel under the existing admission point, not five in series |
| R-2 | The burn-down is large enough to stall the gate landing | phase 3 runs long | land each pass against a recorded baseline and burn it down. Never add an exclusion |
| R-3 | Cause 2 has no mechanism and the spec quietly drops it | the design phase proposes passes for causes 1 and 3 only | A-2 is a named assumption with a validation method. Answering it is a phase-2 deliverable, not a footnote |
| R-4 | A per-flavor pass duplicates the flavor list already in `internal/le/repository/trackedbuild/repositorytrackedbuild.go` | two lists of build flavors drift | derive one from the other, or move the lint pass beside the build flavor it mirrors |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing at run time. This is lint configuration and build tooling. A wrong generator change breaks `./le repository generate` and `dep_audit.py --check`, which is loud and immediate. A wrong burn-down edit can break a test, which its own suite catches |
| How is it reverted? | Per-pass revert. No config migration, no wire-visible change |
| Who else touches this path? | Any spec adding a feature gate touches `feature_tags.go`. `spec-shared-machine-job-admission` owns the job slots the lint runs in. `spec-verify-scope-2-change-set-selector` derives its package graph from the same tag set |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le verify lint run` | → | `$(ZE_LINT_FLAVOR_RUN)` in `_./le verify lint run-impl`, which runs `FLAVORS` and then asserts coverage | `TestLintRunsEveryBuildFlavor` (`internal/le/verify/engine/verifyengine_test.go`), which also pins `ZE_LINT_PKGS` at `./...` |
| `./le changed scope` | → | the same driver with `--scope "$pkgs"`, `&&`-chained after both existing passes | `TestChangedLintRunsEveryBuildFlavor`, which also refuses a `;` anywhere in the chain |
| `./le changed scope`, on an edit under `cmd/ze-installer` | → | `uncompiledTreeReaders` (`internal/le/changed/selector.go`) rules that directory no longer, so the change set widens to `./...` and the driver's `installer` flavor selects the package | `TestSelectorWidensForTheInstallerInitrd` (`internal/le/changed/selector_test.go`) |
| `./le verify lint run` | → | the driver carries the linter's two ceilings, because it runs golangci-lint itself | `TestLintFlavorDriverCarriesTheLinterCeilings` |
| `_./le verify lint run-impl` | → | `BASE_PASSES` (`internal/le/verify/lint/matrix.go`), the two passes every flavor's scope subtracts | `test_base_passes_match_the_lint_recipe` (`internal/le/`) |
| `./le repository generate` | → | `rewriteGolangci` tag list, UNCHANGED by this spec | `./le feature-tags check` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-1 | A deliberate lint violation is added to a file under the retired `scripts/` (current producer: `internal/le/`), `cmd/ze-gok/`, `cmd/ze-serial-shell/`, `api/proto/` or `examples/plugin/go/` | `./le verify lint run` reports it | **MET, unplanned.** The first `./le verify lint run` after `ZE_LINT_PKGS := ./...` failed on a `misspell` finding in `internal/le/verify/engine/verifyengine_test.go`, in a comment this spec had just written in a file no pass had ever loaded. No violation had to be planted. `examples/plugin/go/` is NOT covered: see Known Limitations |
| AC-2 | A deliberate lint violation is added to a `//go:build ze_installer` file | `./le verify lint run` reports it | **MET.** `behaviour` added to a comment in `internal/install/disk/bootstrap_linux.go`, restored from a pristine copy afterwards (`git diff` clean). `./le verify lint run` reports the `misspell` finding from both the `installer` and `installer-nofault` flavors and exits 2 |
| AC-3 | A deliberate lint violation is added to a `//go:build !ze_<feature>` stub | `./le verify lint run` reports it | **MET.** The `compile-out` row derives its `without` from `.golangci.yml`'s own tag list, so the build is `ze_core` and nothing else, and one row selects every stub because the gates are independent. Measured with `behaviour` planted in a comment in `internal/plugins/static/backend_vpp_off_linux.go` (`linux && !ze_vpp`) and restored from a pristine copy afterwards (`git status` clean): `./le verify lint run --scope "./internal/plugins/static"` reports the `misspell` from the `compile-out` flavor and exits 1. The owner ruled on 2026-08-24 that `unused` stays enabled and each feature-only helper takes its consumer's build constraint; 46 of the 47 findings are gone that way, not by an exclusion, or with code no build could reach. The AC's own condition -- a planted violation IS reported -- is MET, and so is the burn-down behind it: the 47th was `forceExitOnSignal` and `bgpDecodeLinked` joined it when the decode tests stopped reading the constant, and both were fixed on 2026-08-24 rulings at `89c35d8ed` and `d580c0870`. AC-7 carries the measurement |
| AC-4 | Each violation above, with only that file changed | `./le changed scope` reports it | **MET, at the case that was failing.** The first evidence used `--scope "./internal/install/disk"`, a package the HOST build reports, so it never exercised the entry point. `cmd/ze-installer` is the one package the host build cannot see at all (`go list -e ./cmd/ze-installer` answers "build constraints exclude all Go files", and it is the only such directory in the tree), and `internal/le/changed/selector.go` used to answer an installer-only edit with `./scripts/checks ./scripts/dev` -- a scope the initrd's PID 1 is not in, so `./le changed scope` linted none of it and exited 0. `uncompiledTreeReaders` no longer rules that directory, so it widens. Measured with a `behaviour` misspelling planted in `cmd/ze-installer/main.go` and restored from a pristine copy afterwards (`git status` clean): the selector answers `./...`, and `./le verify lint run --scope "./..." -j 8` -- the exact call `_./le changed scope` then makes -- reports the `misspell` finding in `main.go` from the `installer` flavor and exits 1. `TestSelectorWidensForTheInstallerInitrd` pins the widening; `TestChangedLintRunsEveryBuildFlavor` pins the `&&`-chained driver call |
| AC-5 | `./le repository generate` runs after the change | `.golangci.yml` still lists `ze_core` plus every feature-gate tag, in manifest order, and `git diff --exit-code .golangci.yml` is clean | **MET.** `./le repository generate` then `git diff --exit-code .golangci.yml` is clean. Nothing in this spec writes that file: the tagless copy is derived under `tmp/` on each run |
| AC-6 | `feature-gates.txt` is read after the change | It contains none of the tags this spec adds | **MET.** Every tag in `FLAVORS` (`debug`, `race`, `live`, `stress`, `maprib`, `fleetperf`, `zetest`, `gokrazy`, `tinygo`, `ze_test`, `ze_perf`, `ze_analyze`, `ze_chaos`, `ze_installer`, `ze_installer_fault`, `ze_distro`, `ze_appliance`, `ze_setup`, `integration`, and `ze_core` under `without`) checked against the manifest: no entry. `stress` and `gokrazy` appear in its prose only. `tools` was listed here in error and is no flavor tag at all: it gates `tools.go`, which stays in `RESIDUE` |
| AC-7 | The full lint runs against the tree | Zero findings remain in the newly reached files, measured UNCAPPED (`--max-issues-per-linter=0 --max-same-issues=0`), or the remainder is a recorded baseline that is burning down. No exclusion is added to `.golangci.yml` | **MET. Every flavor row reports 0, measured 2026-08-25 at `d580c0870`.** `./le verify lint run` runs all 17 rows and exits 0, each printing `0 issues.`; `--scope "./cmd/ze/hub"` alone exits 0 for both the `capability` and the `compile-out` row. The two `unused` findings that held this AC open until 2026-08-24 were FIXED on Thomas's rulings rather than suppressed. `forceExitOnSignal` (`cmd/ze/hub/main.go`) gained a second caller in `run`, because the daemon shutdown path took the same second-signal watchdog `runWebOnly` already had (`89c35d8ed`); moving the function beside its one caller was what the `unused` finding implied, and it was wrong twice, since `c_os_exit` refuses `os.Exit` in `service_web.go` and the asymmetry was a MISSING FEATURE rather than a misplaced function. `bgpDecodeLinked` went with the whole of `cmd/ze/hub/bgp_decode_nolink_test.go` <!-- doc-links: ignore (deleted at d580c0870, which is what this row records) -->, which held that one const and no test function (`d580c0870`; Thomas ran the `rm` himself, because the test-deletion hook needs an interactive approval no agent can supply). Uncapped counts before the burn-down: installer 10, capability 15, distro 1, appliance 1, the retired `scripts/` (current producer: `internal/le/`) 0, setup 0, setup-standalone 0, darwin 0, freebsd 0, openbsd 0, dragonfly 0, wasip1 0, arm64 0, riscv64 0, tinygo 0. The capability 15 only became visible after three TYPECHECK failures in builds nothing had ever compiled were fixed (`live`, `stress`, `ze_chaos`): 12 `noctx`, 1 `errorlint`, 1 `modernize`, 1 `staticcheck`. All fixed and `git diff .golangci.yml` empty. The `compile-out` row, added 2026-08-24, reported 67 uncapped over its seven packages: 47 `unused`, 16 `noctx`, and one each of `errcheck`, `gocritic`, `misspell`, `modernize`, plus `goconst` and a second `modernize` outside `cmd/ze/hub`. All 67 fixed, and no exclusion was added to `.golangci.yml` anywhere in this spec. ELEVEN `//nolint` suppressions were added across the whole burn-down. The count is DERIVED from the commits rather than counted by hand, which is what the Mistake Log demands and what two earlier readings of this row got wrong: it said four, then ten. `git show --unified=0 -- '*.go'` over this spec's eight commits adds a `//nolint` line in exactly two of them, two in `d16046963` and nine in `587f86ad4`. Every one names its linter and states why the finding is wrong for that line: two `gosec` on a `uint16(port)` conversion in the `live` rpki tests (`d16046963`), four `gosec` appended to the `errcheck` directives already on the `os.MkdirAll` mount points in `internal/install/disk/bootstrap_linux.go`, two `gosec` on the `os.OpenFile` of a console path read from `/sys/class/tty/console/active` in `console_linux.go` and `rescue_linux.go`, one `gosec` on the `exec.CommandContext` modprobe call in `internal/plugins/flowexport/conntrack_setup_appliance_linux.go` (the one the ten-count omitted), one `staticcheck` SA5000 on `triggerRuntimeFault`, whose nil-map write IS the fault it injects, and one `staticcheck` QF1011 in `initrd_linux_test.go`, where the written type is the assertion (`587f86ad4`) |
| AC-8 | The blind-population script from this spec is re-run after the change | The 144 has fallen to the files this spec explicitly leaves (the GOARCH axis, and anything the owner ruled out), and the spec names that number | **MET, with a corrected starting number: 245 on Linux, not 144 (A-7).** 245 to 2, and both are named on every full run: `examples/plugin/go/main.go` and `tools.go`. It stood at 36 until the `compile-out` row landed on 2026-08-24; the 34 stubs it left are now linted, and `COMPILE_OUT_STUBS`, the ceiling that counted them while nothing did, is deleted with the predicate it used. Four files left the residue in review round 1. `internal/core/privilege/drop_other.go` is `!linux && !darwin && !freebsd && !openbsd && !netbsd`, which dragonfly satisfies, so the `dragonfly` row lints it; the reason recorded against it, that no unix GOOS selects it, did not hold. The three `//go:build !unix` fallbacks in `internal/core/crashlog` and `pkg/zefs` are reached by the `wasip1` row: `log/syslog` has no windows or plan9 implementation and `internal/core/slogutil` imports it unconditionally, but wasip1 is not unix and does build it, so the whole import graph type-checks and all three lint at 0. The GOARCH axis is IN and covered. Re-derive with `./le verify lint run-population-check`, about 36s |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Edits the installer initrd's PID 1 and runs the changed-file lint | edit → `./le changed scope` → a pass that selects `ze_installer` | the AC-2/AC-4 guard test |
| 2 | Edits a repository gate under `internal/le/` and runs the changed-file lint | edit → `./le changed scope` → a pass whose package set names `./scripts/...` | the AC-1/AC-4 guard test |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| one guard per new pass, named for the population it covers | `internal/le/verify/engine/verifyengine_test.go` | AC-1..AC-4 | not written |
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
- `internal/le/` native action tables - `ZE_LINT_PKGS` is `./...`, `ZE_LINT_FLAVOR_RUN` carries the ceilings, and both lint recipes run the driver
- `internal/le/changed/selector.go` - `uncompiledTreeReaders` no longer rules `cmd/ze-installer`, so an edit to the initrd's PID 1 widens instead of answering with a scope no lint pass loads it under
- `internal/le/changed/selector_test.go` - `TestSelectorWidensForTheInstallerInitrd`, and the installer case out of `TestSelectorMapsGoTreesTheUnitBuildNeverCompiles`
- `internal/le/verify/engine/verifyengine_test.go` - `flavorLintPass`, `lintPassLines`, `TestLintRunsEveryBuildFlavor`, `TestChangedLintRunsEveryBuildFlavor`, `TestLintFlavorDriverCarriesTheLinterCeilings`
- `internal/le/testchaos/actions.go` - `CHAOS_CLI_TAGS` and the `./cmd/ze` run in `_./le test-chaos unit`. The eleven `//go:build ze_chaos` tests in `cmd/ze/ze_chaos_main_test.go` were executed by nothing, and `chaosRun` now depends at RUNTIME on the `chaos` root handler being registered
- `cmd/ze/hub/fleet_perf_test.go` - the auth check reports a parse failure and a non-ok verb apart. One `%w` for both rendered `%!w(<nil>)` on the branch a real refusal takes
- `cmd/ze/pprof_test.go`, `cmd/ze/main_test.go`, `test/weakened.md` - the move's stated reason. `main_test.go` carries `//go:build ze_core`, not no constraint; what it lacks is `!tinygo`
- `ai/rules/points/commands/lint-gate/what-native-changed-lint-covers-and-costs.md` - the pass count and the cost, and the one directory whose edit widens the gate to the whole tree
- `docs/contributing/testing.md` - "The builds the linter reads"
- `docs/architecture/testing/verify-freshness-scope.md` - "Which packages a scoped run judges": what `uncompiledTreeReaders` still rules, and why `cmd/ze-installer` left it
- Burn-down, all of it uncapped: `internal/install/disk/{bootstrap,console,rescue,fault}_linux.go`, `internal/install/disk/{fault,initrd}_linux_test.go` (10), `internal/component/bgp/plugins/rpki/{rpki_live,rtr_downgrade}_test.go` (13, mostly `noctx` -- every `exec.Command` is `exec.CommandContext` now, and `dockerRM` carries its own bound because a cleanup runs after `t.Context()` is canceled), `cmd/ze/hub/fleet_perf_test.go` (2), `internal/component/config/system/selfupdate.go` (1), `internal/plugins/flowexport/conntrack_setup_appliance_linux.go` (1), `internal/le/verify/engine/verifyengine_test.go` (1, this spec's own new comment)
- Compile fixes for builds nothing had ever compiled: `internal/component/bgp/plugins/rpki/{rpki_live,rtr_downgrade}_test.go` (`live`), `internal/component/web/stress_test.go` (`stress`), `cmd/ze/ze_chaos_main_test.go` (`ze_chaos`, now dispatching through `registry.LookupRoot`), `cmd/ze/main_test.go` (`tinygo`)
- `examples/plugin/go/main.go` - `pluginserver.CommandContext` is `plugin.CommandContext`; the example did not compile
- AC-3, the `compile-out` row: `internal/le/verify/lint/matrix.go` (`feature_gate_tags`, the row, the deletion of `COMPILE_OUT_REASON`, `COMPILE_OUT_STUBS` and `compile_out_stub`, and `population()` skipping a tracked path the working tree no longer has -- `git ls-files` answers from the index, so deleting a `.go` file made the coverage assertion red with every pass at 0 issues), `internal/le/` (`test_the_compile_out_row_keeps_ze_core_alone` and `test_a_file_deleted_in_the_working_tree_leaves_the_population` replace `TestCompileOutStub` and `TestStubCeiling`)
- AC-3, the build constraint each feature-only helper now carries: `cmd/ze/hub/editor_adapter.go` (`ze_web`), `cmd/ze/hub/cert_store.go` (`ze_lg || ze_web`), and the six helpers moved into the gated file that calls them -- `setRESTInfra`, `setGRPCInfra` (`api_infra.go`), `setGNMIInfra` (`gnmi_infra.go`), `setSSHInfra` (`ssh_infra.go`), `setWebStandalone` (`web_infra.go`), each now the direct seam assignment its `register_<x>.go` init makes; `listenerMigrator.setMCP` into `register_mcp.go`; and `resolveConfigPath` (`main_servers.go`) into `service_web.go`. `forceExitOnSignal` did NOT move. It stays in `main.go` and gained a caller there instead: `89c35d8ed` starts it from `run`'s shutdown path, so the daemon honors a second signal as `runWebOnly` already did. See Known Limitations
- AC-3, the second source of truth left unread: every one of `bgpDecodeLinked`'s five readers now asks `pluginreg.GetPacketDecoder()`, which is the fact the constant mirrored, and the seam-is-nil assertion moved to `build_tag_bgp_absent_test.go`, where it RUNS -- the `!ze_bgp && ze_web` build that carried it runs in no suite. `cmd/ze/hub/bgp_decode_nolink_test.go` <!-- doc-links: ignore (deleted at d580c0870, which is what this row records) --> is deleted, at `d580c0870`, by Thomas himself: it held that constant and no test function, and the test-deletion hook needs an interactive approval no agent can supply. See Known Limitations
- AC-3, the same treatment outside the hub: `internal/plugins/cos/{session_state,enricher,enricher_test,handler_off,register}.go` (the per-session record and the two subscriber enrichers that read it take `handler.go`'s `ze_l2tp`, with the `show.MustRegister` pair), and the burn-down of what the new pass then reported -- 16 `noctx` in `cmd/ze/hub/build_tag_*_absent_test.go`, `errcheck` and `gocritic` in `build_tag_ssh_absent_test.go`, `misspell` in `build_tag_l2tp_absent_test.go`, `modernize` in `build_tag_web_absent_test.go` and `internal/component/config/infra/authz_no_ssh_test.go`, `goconst` in `internal/plugins/diag/cmd/capture_{,raw_}l2tp_off.go`

## Files to Create
- `internal/le/verify/lint/matrix.go` - the flavor table, the derived scopes, the passes, and the coverage assertion
- `internal/le/` - its unit tests, run by `go test ./scripts/dev/` through `python_tests_test.go`
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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/testing/verify-freshness-scope.md`, updated: it is the design doc `docs/` holds for the change-set selector, and "Which packages a scoped run judges" now says what `uncompiledTreeReaders` still rules and why `cmd/ze-installer` left it. `internal/le/changed/selector.go` ALSO declares `plan/spec-verify-scope-2-change-set-selector.md` as its `// Design:`, and that spec is UNAFFECTED and not edited here: it owns the shape of the change set (one selector, a tag-aware reverse graph, depth 2, fail open on a seed it cannot classify), and this change adds no rule and removes no property -- it deletes one entry from the table of directories exempted from that spec's own fail-open path, so the selector now behaves as that spec already specifies. It is also another session's open spec, which this one must not edit |
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
   - Files: the the native action tables under `internal/le/` recipes and `internal/le/verify/engine/verifyengine_test.go`
   - Verify: a deliberate violation in each newly reached population is reported by BOTH lint paths
4. **Phase: Burn down** - fix the findings each pass now sees
   - Tests: the existing suites stay green; `internal/le/testunit/groups.go`'s installer flavor stays green
   - Files: the files carrying findings
   - Verify: AC-7 and AC-8. No lint exclusion is added

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | Both the full lint AND the changed-file lint are covered, for every new pass |
| Correctness | `.golangci.yml` was never hand-edited. Confirm by running `./le repository generate` and seeing no diff |
| Population | The claim "X is now linted" is verified by ENUMERATING X and confirming the pass loads each file, not by observing that a pass exists |
| Data flow | `feature-gates.txt` still holds feature gates only |
| Rule: `ai/rules/repo-maintenance.md` | The generated file changed only as generator output |
| Rule: `ai/rules/quality.md` | No linter disabled and no exclusion added to reach green |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Each newly reached population is linted | Add a deliberate violation, run `./le verify lint run`, confirm it is reported, restore the file from a pristine copy saved first |
| Changed-file path covered | Same violation, run `./le changed scope` |
| Generator still correct | `./le repository generate` then `git diff --exit-code .golangci.yml` |
| Drift check intact | `./le verify lint run` |
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
| One pass for each build, driven by `internal/le/verify/lint/matrix.go`, with the package set DERIVED from the tree | a tag list in the the native action tables under `internal/le/`; a manifest file both make and the guard test read; a lint row beside each `buildMatrix` flavor in `internal/le/repository/trackedbuild/repositorytrackedbuild.go` | a written list drifts silently the moment a `//go:build debug` file lands in a new package, which is this spec's own defect class. Deriving it costs one `go list` for each flavor, about 36s in total, and makes drift impossible rather than detectable. R-4 is answered the same way: the lint table is derived from the TREE, so it cannot disagree with `buildMatrix` about anything that matters |
| Each flavor lints only the packages holding a file the base passes do not load | a full `./...` pass for each flavor | measured: a full-tree pass is 286s and a scoped flavor pass is 8-39s. Thirteen full passes would have made the lint stage the whole verify (R-1) |
| The driver ASSERTS coverage at the end of a whole-tree run | a separate `./le verify lint run-population-check` target and verify stage | the driver already holds every `go list` answer the assertion needs, so the check is free where a separate stage would pay for its own derivation. It is skipped for a scoped run, where a missing file is missing because the caller said so |
| A flavor that must turn a tag OFF gets a DERIVED tagless config (`tagless_config`) | a second GENERATED `.golangci-*.yml` beside the first; hand-maintaining a second config | `--build-tags` only adds (A-1), so the only subtractive route is a whole config. Deriving it on every run from `.golangci.yml` keeps ONE linter set: a tracked second file drifts, and the linter set is exactly the thing that must not differ between two passes. It is written under `tmp/`, so `relative-path-mode: gitroot` is needed for its findings to name the same paths |
| The two existing passes stay written in the the native action tables under `internal/le/` recipe | moving them into the driver as rows | "Behavior to preserve" asks for it, and the two existing guard tests read the recipe. `BASE_PASSES` declares them for subtraction only, and `test_base_passes_match_the_lint_recipe` fails when the recipe and that declaration disagree |
| Python, not Go, for the driver | a `//go:build ignore` program beside `internal/le/repository/trackedbuild/repositorytrackedbuild.go` | `docs/contributing/ze-go-style.md` puts Ze's tooling in Python, and `internal/le/` already runs a `<tool>_test.py` inside `./le test-unit` with no make target of its own |
| An edit under `cmd/ze-installer` WIDENS the change set to `./...`, by deleting the rule that exempted it | naming `./cmd/ze-installer` in the narrow answer; a second print mode on the selector for a lint-only package list; a lint-only scope computed beside the shared one | the narrow answer is not available: the same list drives `ze-unit-test-changed`, and `go test ./cmd/ze-installer` under the unit tag set fails with "build constraints exclude all Go files". Both other routes add a channel and a second change-set consumer to carry one directory, and the rule being deleted had a stated reason -- "./... does not compile it either" -- that this spec's own work made false. Deleting it cuts machinery and puts the directory back on the selector's own documented fail-open path. Five commits have ever touched it, so the wide answer is paid about that often |
| A residue reason is a claim that gets CHECKED, not a note | leaving the four `!unix`-class entries with the reason first written for them | review round 1 found the `internal/core/privilege/drop_other.go` reason false: dragonfly, solaris, illumos and aix all satisfy `!linux && !darwin && !freebsd && !openbsd && !netbsd`, and every one is unix and buildable here. Checking the other three the same way found windows and plan9 are not the whole non-unix set either: wasip1 is not unix and DOES build `log/syslog`, so it type-checks the whole graph. Two rows, `dragonfly` and `wasip1`, cost one `go list` and three packages each and took the residue from 40 to 36 |
| A feature-only helper gets its CONSUMER's build constraint | running the `compile-out` pass with every linter except `unused` | `docs/contributing/ze-go-style.md` refuses a disabled linter, and `unused` is not confused here: `cmd/ze/hub/editor_adapter.go` carried no constraint while its only callers sit in `service_web.go` (`ze_web`), so a bare-core daemon compiled 32 methods no code in it could reach. Owner ruling, 2026-08-24. The constraint is the disjunction where consumers differ (`cert_store.go` is `ze_lg \|\| ze_web`), and a helper whose only caller is one gated file MOVES into that file, which is what plugins.md already asks for |
| `COMPILE_OUT_STUBS` and `compile_out_stub()` are DELETED, not lowered to 0 | keeping the ceiling at 0 as a second guard | the ceiling existed because nothing lints a stub, so a 35th could land unseen. A pass now lints them, and `report_coverage` already fails on any file no pass loads. A second counter over the same population would report the same fact twice and drift when one of them is wrong (`ai/rules/no-layering.md`) |

## Known Limitations

**BOTH OPEN SYMBOLS WERE RULED ON BY THOMAS ON 2026-08-24 AND ARE FIXED. NOTHING KEEPS THE GATE RED.**

- **`forceExitOnSignal` (`cmd/ze/hub/main.go`) was `unused` in the bare-core build, and Thomas chose to give the daemon the feature rather than move the function.** Its only caller was `runWebOnly` in `service_web.go`, which carries `//go:build ze_web`, so a daemon built without the web listener compiled in a function nothing could call. Moving it beside that caller is what the `unused` finding implied, and it was wrong twice: `c_os_exit` (`.claude/hooks/pretool-writeedit.py` (retired; now `internal/le/hookruntime/writeedit.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->) allows `os.Exit` only in `main.go`, `register.go`, the retired `scripts/` (current producer: `internal/le/`) and `_test.go`, and the asymmetry was a MISSING FEATURE rather than a misplaced function, because shutdown stops plugins, gNMI, the API servers and the reactor with a grace period each, so a wedged component can hold the daemon past every one of them with only SIGKILL left. `89c35d8ed` starts the same watchdog from `run`'s shutdown path. **This is a user-visible behavior change for anyone scripting `ze hub`: a second signal now exits 1 where it was ignored before, and the shutdown line says `Ctrl+C again to force` before it happens.** The change is wider than Ctrl+C, and the record must say so: `run` builds one `sigCh`, `signal.Notify` delivers SIGINT, SIGTERM and SIGHUP to it, `monitorStdinEOF` injects one SIGTERM on stdin EOF, and `apiServer.SetShutdownFunc` injects a SIGTERM every time `request shutdown` is called. That was true of `89c35d8ed`, which read `sigCh` directly: a repeated `request shutdown`, a supervisor sending SIGTERM twice, and a SIGHUP arriving after shutdown had started each reached the same forced exit, because `forceExitOnSignal` reads the next VALUE and never asks which signal it is. `67d1e9cb4` closes that by giving the watchdog its own channel: `run` builds `forceCh` and calls `signal.Notify(forceCh, syscall.SIGINT, syscall.SIGTERM)`, so `request shutdown`, stdin EOF and SIGHUP cannot reach it. A supervisor's second SIGTERM still exits 1, which is the feature. Recorded in `plan/journal/field-carries-two-meanings.md`.
- **`bgpDecodeLinked` (`cmd/ze/hub/bgp_decode_nolink_test.go` <!-- doc-links: ignore (deleted at d580c0870, which is what this row records) -->) was `unused`, and Thomas ruled the deletion and ran the `rm` himself**, because the test-deletion hook needs an interactive approval no agent can supply (`d580c0870`). The file carried `//go:build !ze_bgp`, that one constant, and no test function. Its five readers had moved to `pluginreg.GetPacketDecoder()` in `448740364`, the seam the constant mirrored. No coverage left the suite: `build_tag_bgp_absent_test.go` asserts `GetPacketDecoder()` is nil in the same `!ze_bgp` build, and that build RUNS -- `_ze-unit-test-impl` ends with `$(GO_TEST_CORE_RACE) ./cmd/ze/hub`, whose tag set is `ze_core` alone. Verified by this closure: `go test -tags 'ze_core' -race ./cmd/ze/hub` exit 0, 117.5s. The row in `test/weakened.md` records the deletion.
- **Cause 2 is otherwise closed.** The `compile-out` row lints all 34 stubs. `unused` was not disabled for it: the owner ruled on 2026-08-24 that a feature-only helper carries its consumer's build constraint, so the symbols it named now do, or are gone. What the row cannot reach is a file whose constraint needs a gate ON and another OFF, because no pass runs a build with one gate on and the rest off. `ze_web && !ze_bgp` was the shape that existed, in the assertion `448740364` moved out of a build no suite ran, and no tracked file mixes one FEATURE gate on with another off today. Mixed polarity survives only among the PERSONALITY tags, and each such file has a row of its own: `cmd/ze/setup_dispatch.go` and `cmd/ze/build_tag_setup_test.go` (`ze_setup && !ze_core`) go to `setup-standalone`, `cmd/ze/build_tag_appliance_test.go` and `build_tag_distro_test.go` to `appliance` and `distro`, and `internal/install/disk/fault_stub_linux.go` (`ze_installer && !ze_installer_fault`) to `installer-nofault`. A feature-gate file of that shape landing later would need its own row, and `report_coverage` is what would say so: it fails a whole-tree run on any tracked Go file no pass loads.
- `examples/plugin/go/` is a separate Go module with no tracked `go.sum`, so `./...` cannot reach it and a run in that directory needs `go mod tidy`, which this repository's vendored dependencies do not provide for. It is named in `RESIDUE` with what would have to change. Its one file also did not COMPILE (`undefined: pluginserver`); that is fixed here, and proved in a scratch copy of the module.
- `tools.go` cannot be type-checked by anything: the tools.go idiom imports PROGRAMS so `go mod tidy` pins them. It leaves the residue when those pins move to go.mod's `tool` directives.
- The GOARCH axis IS in scope and covered, by two rows: `linux-arm64` for the filename-suffixed netlink files, and `linux-other-arch` (riscv64) for `linux && !amd64 && !arm64`.
- The GOOS axis now runs to six rows. `darwin`, `freebsd` and `openbsd` were there from the first round; `dragonfly` and `wasip1` were added in review round 1, and each removed a residue entry whose stated reason did not hold. `windows` and `plan9` stay unreachable, and the reason is exact: `internal/core/slogutil` imports `log/syslog` with no build constraint and that package has no implementation on either, so every package reaching the logger fails to type-check. `wasip1` is not a target claim -- Ze ships no WASI binary -- it is the build under which the `!unix` fallbacks type-check.
- An edit to `cmd/ze-installer` widens the changed-file gate to `./...`, and that is the design rather than a fallback. Every file there is `linux && ze_installer`, so no `go list` under the unit tag set reports the package, and no narrower answer names anything a lint pass loads it under. `./cmd/ze-installer` cannot be put in the narrow answer instead: the same list drives `ze-unit-test-changed`, where `go test` on it fails with "build constraints exclude all Go files". Five commits have ever touched that directory.
- `gokrazy/modcache/` (25 tracked third-party files) is excluded from the population on purpose. It is a vendored module cache, not Ze's source.
- A `//go:build ignore` file (29 of them, every program under `internal/le/` among them) belongs to no build by its own declaration and can never be linted by any pass. `population` states that.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Lessons ROUTED: a problem class to `plan/journal/<class>.md`, a rule to `ai/rules/`, a design decision to `docs/architecture/`. `plan/learned/` holds only `DESIGN-HISTORY.md`, `HOOK-FRICTION.md` and `RECURRING-PATTERNS.md`; no `NNN-<name>.md` is written
- [ ] **Commit A:** code + tests + docs + spec + journal rows
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

`./le verify lint run` analysed one build. It now analyses eighteen. `internal/le/verify/lint/matrix.go`
holds the flavor table, derives each flavor's package set from `go list` minus what the two
base passes load, runs one scoped golangci-lint pass per flavor, and then asserts that every
tracked Go file was loaded by some pass. `ZE_LINT_PKGS` went from four named roots to `./...`,
and `ZE_LINT_FLAVOR_RUN` carries the same `-j` and `GOMEMLIMIT` ceilings `ZE_LINT_RUN` does.
Both lint recipes call the driver, the changed-file one `&&`-chained with `--scope "$pkgs"`.

`internal/le/changed/selector.go` lost the rule that kept an edit under
`cmd/ze-installer` narrow: no `go list` under the unit tag set reports that package, so the
narrow answer named nothing a lint pass would load it under, and the edit now widens to `./...`.

The burn-down cleared every finding the new passes reported: installer 10, capability 15
(visible only after three builds that had never compiled were fixed), distro 1, appliance 1,
and 67 over the `compile-out` row. The last two, both `unused` in `cmd/ze/hub`, needed an
owner ruling and got one on 2026-08-24: `89c35d8ed` gives the daemon the second-signal
watchdog that gives `forceExitOnSignal` a caller in every build, and `d580c0870` deletes the
file holding `bgpDecodeLinked`. No linter was disabled and no exclusion was added to
`.golangci.yml`. Eleven `//nolint` suppressions were added, each naming its linter and saying
why the finding is wrong for that line. The count is DERIVED, not counted by hand: two in
`d16046963` and nine in `587f86ad4`, and no other commit of this spec adds one.

### Bugs Found/Fixed
- Three builds nothing had ever compiled: `live` (`rpki_live_test.go`, `rtr_downgrade_test.go`
  called four symbols that no longer exist), `stress` (`internal/component/web/stress_test.go`
  called `CommittedConfig`, unexported by an earlier sweep that could not see the file), and
  `ze_chaos` (`cmd/ze/ze_chaos_main_test.go`). Each is proven by the flavor pass that now
  type-checks it.
- Eleven `//go:build ze_chaos` tests in `cmd/ze/ze_chaos_main_test.go` were executed by nothing.
  `internal/le/testchaos/actions.go` now runs them (`CHAOS_CLI_TAGS`), and `chaosRun` dispatches through
  `registry.LookupRoot("chaos")`.
- `fleetInitialSync` (`cmd/ze/hub/fleet_perf_test.go`) wrapped two different failures with one
  `%w` and rendered `%!w(<nil>)` on the branch a real refusal takes.
- `examples/plugin/go/main.go` did not compile (`undefined: pluginserver`).
- A feature-only helper compiled into a build that could not call it: `editor_adapter.go`
  carried no constraint while its only callers are `ze_web`, and `cert_store.go` likewise for
  `ze_lg || ze_web`. Six seam setters were one indirection over a direct assignment their own
  `register_<x>.go` init could make, and are deleted.
- `bgpDecodeLinked` was a build-tag mirror of `pluginreg.GetPacketDecoder()`, a second source
  of truth for one fact. Its readers now ask the seam. Nothing asserted that a `ze_bgp` build
  FILLS that seam, so a broken `bgp/cli` init would have emptied three decode tests in silence;
  `TestBuildTag_BGP_PresentFillsTheDecoderSeam` is that assertion.
- `loadConntrackModules` (`internal/plugins/flowexport/conntrack_setup_appliance_linux.go`) ran
  `modprobe` with no bound. It carries `modprobeTimeout` now.

### Documentation Updates
- `docs/contributing/testing.md`, new section "The builds the linter reads", anchored
  `<!-- source: internal/le/verify/lint/matrix.go -- FLAVORS, scopes -->`.
- `docs/architecture/testing/verify-freshness-scope.md`, "Which packages a scoped run judges":
  what `uncompiledTreeReaders` still rules, and why `cmd/ze-installer` left it.
- `ai/rules/points/commands/lint-gate/what-native-changed-lint-covers-and-costs.md` and
  `ai/rules/points/precommit-verify/running-the-gate/what-each-changed-path-selects.md`, both
  regenerated into `ai/rules/commands.md` and `ai/rules/precommit-verify.md` by
  `./le rules render-update`.
- `ai/rules/points/commands/lint-gate/never-invoke-golangci-lint-directly.md` repointed at the
  journal row that is its evidence.
- `./le doc check verify` exits 0, re-run by this closure on 2026-08-25. The one anchor that
  failed on 2026-08-24 was never this spec's: `docs/guide/web-interface.md` named
  `liveAAABundleAuthenticator.Authenticate`, and another session landed the fix at
  `dcbb0efa6`. The Source anchors stage now reports "checked 2192 code paths, 517 packages,
  all references valid". `ai/DOCS-TO-CODE.md` was stale again on the re-run and was
  regenerated; it is gitignored and untracked, so it belongs in no commit.

### Deviations from Plan
- The spec sized the blind population at 144 from a darwin measurement. It is 245 on Linux,
  because base pass 1 takes the host GOOS (A-7, broken).
- Cause 2 was expected to have no mechanism. A derived tagless config gives it one (A-2,
  broken in the direction that helps).
- The GOARCH axis was written as "a floor, not a count" and is IN, covered by two rows.
- The burn-down needed two owner rulings before it finished. Both came on 2026-08-24, and each
  chose the fix over the suppression: the daemon gained a feature it was missing rather than
  hiding an `unused` function, and a `_test.go` holding one unread const was deleted rather
  than given a fabricated reader. AC-7 is met.
- **`89c35d8ed` is a user-visible behavior change this spec did not commission.** A second
  signal during `ze hub` shutdown now exits 1 where it was ignored. Known Limitations states
  the full trigger set, which is wider than the Ctrl+C the commit message names.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 assumed every file in the blind population compiles under the build that selects it | Three builds did not compile at all: `live`, `stress`, `ze_chaos`. A lint pass cannot report a finding it cannot type-check | the first run of each new flavor reported typecheck errors, not lint findings | fixed each build before measuring its findings; the capability flavor's 15 findings were invisible until then |
| assumption | A-7 assumed the blind population is a property of the TREE | It is a property of the tree AND the host: 245 on Linux against 144 on darwin, because pass 1 takes the host GOOS | re-derived the population on Linux | every GOOS a flavor needs is now NAMED in `FLAVORS`, so the answer no longer moves with the machine |
| approach | A residue entry was written with a reason nobody re-checked | `internal/core/privilege/drop_other.go` was excused as reachable by no unix GOOS. dragonfly satisfies its constraint | review round 1 read the constraint against `go list` per GOOS | `dragonfly` and `wasip1` rows added; four residue entries removed; a residue reason is now a claim the coverage assertion re-checks |
| approach | AC-7 and Goal Validation both stated that four `//nolint` were added in the whole burn-down, and the number was never counted against the diff | Ten were added, over ten lines in six files. Each is individually legitimate -- the closure re-read every one and checked its reason against the producing code, including that both console `os.OpenFile` paths are built from `/sys/class/tty/console/active` -- so the gate was not weakened. The RECORD was wrong by a factor of 2.5, and it was the sentence a reader would use to judge whether this spec bought its green with suppressions | review round 4 counted `^+.*//nolint` over each of the spec's eight commits | AC-7 now enumerates all ten by linter, file and reason. Transferable: a negative claim ("no X was added except N") is a COUNT, and a count is derivable from the diff, so it must be derived rather than remembered |
| approach | Three closure statements were written ahead of the work they describe | On 2026-08-24 `bgp_decode_nolink_test.go` was not deleted, `forceExitOnSignal` had not moved, and the `compile-out` row reported 2 rather than 0 | round 3 ran the row over `./cmd/ze/hub` and read exit 1 | all three corrected in the record on 2026-08-24, and made TRUE on 2026-08-25 by doing the work rather than by editing the claim again: `89c35d8ed` and `d580c0870`. `forceExitOnSignal` still did not move -- it gained a caller instead -- so that statement stays corrected rather than fulfilled. A record written from an intention rather than from a run is the class `plan/journal/stale-spec-claims-done.md` collects |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Cause 1: the package set names four roots | Done | `internal/le/` native action tables, `ZE_LINT_PKGS := ./...` | 55 files, 0 findings once loaded |
| Cause 2: every gate is ON, so no compile-out stub is loaded | Done | `internal/le/verify/lint/matrix.go`, `tagless_config`, the `compile-out` row | mechanism found (A-2 broken); burn-down 65 of 67 |
| Cause 3: personality and capability tags neither pass carries | Done | `internal/le/verify/lint/matrix.go`, `FLAVORS` | installer, installer-nofault, distro, appliance, setup, setup-standalone, capability, tinygo |
| Cause 4: GOARCH, which no pass varies | Done | `FLAVORS`, `linux-arm64` and `linux-other-arch` | promoted from "a floor" to covered |
| Zero findings remain in the newly reached files | Done | `internal/le/verify/lint/matrix.go`, all 17 rows | `./le verify lint run` exit 0 on 2026-08-25 at `d580c0870`, every row `0 issues.` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | an unplanted `misspell` in `internal/le/verify/engine/verifyengine_test.go` failed the first run after `ZE_LINT_PKGS := ./...` | |
| AC-2 | Done | planted `behaviour` in `internal/install/disk/bootstrap_linux.go`, reported by `installer` and `installer-nofault`, exit 2 | file restored from a pristine copy |
| AC-3 | Done | planted `behaviour` in `internal/plugins/static/backend_vpp_off_linux.go`, reported by `compile-out`, exit 1 | the AC's own condition, and the burn-down behind it finished on 2026-08-24 |
| AC-4 | Done | planted `behaviour` in `cmd/ze-installer/main.go`; the selector answers `./...` and the driver reports it | `TestSelectorWidensForTheInstallerInitrd`, `TestChangedLintRunsEveryBuildFlavor` |
| AC-5 | Done | `./le repository generate` then `git diff --exit-code .golangci.yml` clean | |
| AC-6 | Done | every `FLAVORS` tag checked against `feature-gates.txt`: no entry | |
| AC-7 | Done | `./le verify lint run` exit 0 at `d580c0870`, all 17 rows `0 issues.` | the last two were fixed on 2026-08-24 owner rulings (`89c35d8ed`, `d580c0870`), not suppressed |
| AC-8 | Done | `./le verify lint run-population-check` exit 0, "every tracked Go file is linted, except the 2 stated above" | 245 to 2 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLintRunsEveryBuildFlavor` | Done | `internal/le/verify/engine/verifyengine_test.go` | also pins `ZE_LINT_PKGS` at `./...` |
| `TestChangedLintRunsEveryBuildFlavor` | Done | `internal/le/verify/engine/verifyengine_test.go` | refuses a `;` anywhere in the `&&` chain |
| `TestLintFlavorDriverCarriesTheLinterCeilings` | Done | `internal/le/verify/engine/verifyengine_test.go` | pins `-j` and `GOMEMLIMIT` against `ZE_LINT_RUN` |
| `TestSelectorWidensForTheInstallerInitrd` | Done | `internal/le/changed/selector_test.go` | |
| `TestBuildTag_BGP_PresentFillsTheDecoderSeam` | Done | `cmd/ze/hub/bgp_decode_link_test.go` | proven to discriminate: red with the blank import removed |
| `lint_flavors_test.py` (8 cases) | Done | `internal/le/` | run by `go test ./scripts/dev/` through `python_tests_test.go` |
| `TestIsLocalhostPprof` | Done | `cmd/ze/pprof_test.go` | moved so it carries `pprof.go`'s `!tinygo` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/verify/lint/matrix.go` | Done | created, 600L |
| `internal/le/` | Done | created |
| `cmd/ze/pprof_test.go` | Done | created |
| `internal/le/` native action tables, `internal/le/testchaos/actions.go`, `internal/le/repository/repository.go`, `internal/le/repository/repository_test.go`, `internal/le/verify/engine/verifyengine_test.go` | Done | committed `d16046963`, `98f4297eb` |
| the hub build constraints | Done | committed `98f4297eb`, `448740364` |
| `cmd/ze/hub/bgp_decode_nolink_test.go` <!-- doc-links: ignore (deleted at d580c0870, which is what this row records) --> deletion | Done | `d580c0870`, run by Thomas: the hook needs an interactive approval |
| `forceExitOnSignal` move | **Changed** | not moved. `89c35d8ed` gives it a caller in `main.go` instead, which is the ruling Thomas made |
| the burn-down and the docs | Done | committed `587f86ad4`. The whole diff is landed; nothing of this spec is uncommitted but the spec itself |

### Audit Summary
- **Total items:** 31
- **Done:** 30
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5 (four in Deviations, plus the `forceExitOnSignal` move that became a feature)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A file outside the analysed build is no longer reported as clean | functional (planted violation, both lint paths) | AC-1, AC-2, AC-3, AC-4. Each plants a real violation in a real file of the population, runs the gate, reads the finding, and restores the file from a pristine copy. A planted violation is the only evidence that discriminates: a pass that EXISTS proves nothing about what it loads |
| The 245 blind files fall to a number this spec names | measurement, re-derived from the tree | `./le verify lint run-population-check`, exit 0: "every tracked Go file is linted, except the 2 stated above". The two are `examples/plugin/go/main.go` and `tools.go`, each with the change that would end its exclusion |
| The coverage claim cannot rot silently | guard | `report_coverage` fails a whole-tree run on ANY file no pass loads, AND on a `RESIDUE` entry that stopped being blind. Both directions, so an entry cannot outlive its cause and a new blind file cannot be added quietly |
| Both lint paths are covered, not just the full one | guard test, proven to discriminate | `TestLintRunsEveryBuildFlavor`, `TestChangedLintRunsEveryBuildFlavor`, `TestLintFlavorDriverCarriesTheLinterCeilings`. Each goes RED against a the native action tables under `internal/le/` with the driver line removed, with the four roots restored, or with the `&&` replaced by `;` |
| No linter is disabled and no exclusion is added to reach green | negative evidence, counted from the diff | `git diff .golangci.yml` empty. ELEVEN `//nolint` added in the whole burn-down: 2 in `d16046963` and 9 in `587f86ad4`, and none in the other six commits. Each names its linter and states why the finding is wrong for that line, and AC-7 enumerates all eleven. The command this row used to name, `git show \| grep -c '^+.*//nolint'`, is NOT a safe derivation and gave 2 and 8: it matches added lines in any file, so over `d16046963` it counts THIS SPEC's own AC-7 prose as a third suppression, and the eight it reported for `587f86ad4` was simply short by one. The command that answers is `git show <sha> --unified=0 -- '*.go' \| grep -c '^+.*//nolint'`. Two earlier drafts of this row said four and then ten, which is the defect the Mistake Log's first entry records, reached twice more |
| **Zero findings remain in the newly reached files** | measurement, re-run by this closure | ACHIEVED. `./le verify lint run` exit 0 on 2026-08-25 at `d580c0870`: all 17 flavor rows print `0 issues.`, and the run ends with the coverage assertion. The last two findings were fixed on 2026-08-24 owner rulings, not suppressed and not excluded |
| Interop | N-A | no protocol behavior changes. The diff is build tooling, build constraints, and a burn-down of lint findings |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | n-a | the spec metadata declares `Deferral shard: -`, and `plan/deferrals/fixit-lint-blind-to-every-other-build-tag.md` does not exist <!-- doc-links: ignore (the row states this shard was never created) --> (`ls plan/deferrals/ \| grep -i lint` is empty) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-lint-blind-to-every-other-build-tag-9ad8358c-695f-41be-8019-5d92ba08f8e6.md` |
| `review_gate.py check` | CLEAN. Round 4 verdict `clean`, 0 BLOCKER, 0 ISSUE |
| Rounds | 4 |
| Reviewer lenses used | rounds 3 and 4, one agent per round, every lens in each: gate-population (does the pass load what the record says), record-accuracy (each closure claim read against the producing file), style (`docs/contributing/ze-go-style.md` over every changed Go file), guard discrimination, test integrity, commit-hygiene (which of the diff is landed and which is not). Round 4 read the three commits that landed after round 3 -- `587f86ad4`, `89c35d8ed`, `d580c0870` -- and re-measured every claim round 3 had left open |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE (round 2) | a blocking rule still stated the selector behaviour the fix removed | `ai/rules/points/precommit-verify/running-the-gate/what-each-changed-path-selects.md` | the row now names only the module root, and a second row sends `cmd/ze-installer` to `./...`; rendered by `./le rules render-update` (committed `98f4297eb`) |
| 2 | NIT (round 2) | the file header said `uncompiledTreeReaders` maps two directories | `internal/le/changed/selector.go` | the header says one, and says where `gokrazy/modcache` is answered instead (committed `98f4297eb`) |
| 3 | BLOCKER (round 3) | AC-7 claimed the `compile-out` row reports 0. It reports 2, at HEAD | `plan/spec-...-build-tag.md`, AC-7 | AC-7 rewritten with the measurement. The FINDINGS are not fixed: both need Thomas |
| 4 | ISSUE (round 3) | the spec claimed `bgp_decode_nolink_test.go` is deleted and `forceExitOnSignal` moved. Neither happened | `plan/spec-...-build-tag.md`, Files to Modify; `test/weakened.md` | all three statements corrected, and Known Limitations now carries both open symbols with both options each |
| 5 | ISSUE (round 3) | 17 files of this spec's own diff are UNCOMMITTED while the record reads as landed | working tree | enumerated in Pre-Commit Verification below, with the diff that proves each one is this spec's |

### Round 4 -- clean

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 6 | ISSUE (round 4) | AC-7 and Goal Validation both said four `//nolint` were added in the whole burn-down. Ten were. Each is legitimate, so the gate is not weakened, but the sentence a reader uses to judge that is the one that was wrong | `plan/spec-...-build-tag.md`, AC-7 and Goal Validation | both rows now enumerate all ten by linter, file and reason, derived from `git show \| grep -c '^+.*//nolint'` over each commit. Mistake Log row added |
| 7 | ISSUE (round 4) | `89c35d8ed`'s message and the spec described the change as "a second Ctrl+C". `run` builds ONE `sigCh` that `signal.Notify` fills with SIGINT, SIGTERM and SIGHUP, and that `apiServer.SetShutdownFunc` also injects a SIGTERM into on every `request shutdown`, so a repeated `request shutdown` or a supervisor's second SIGTERM reaches the same forced exit | `cmd/ze/hub/main.go`, `run` | Known Limitations now states the full trigger set and names `monitorStdinEOF` as the one producer that cannot reach it, because it sends once and returns. Whether the watchdog reads a channel fed by `signal.Notify` alone is Thomas's call, journalled at `plan/journal/field-carries-two-meanings.md` |
| 8 | NOTE (round 4) | The new watchdog wiring in `run` has no test, and neither has the identical `runWebOnly` call since it was written. Deleting `go forceExitOnSignal(sigCh)` from `main.go` reddens nothing, and `unused` no longer fires either, because `service_web.go` still calls it | `cmd/ze/hub/main.go`, `run` | recorded, not fixed. A deterministic test needs a shutdown slow enough to signal into, which is a fault-injection point rather than an assertion. Journalled with finding 7 |
| 9 | NOTE (round 4) | `plan/journal/stale-spec-claims-done.md` cites this spec by full path, and commit B removes that file. The citation gate does not read `plan/journal/`, so it stays green either way | `plan/journal/stale-spec-claims-done.md` | restated as the bare stem, which keeps the name and drops a resolution promise the tree cannot keep |
| 10 | NOTE (round 4), escalated | Another session's uncommitted `test/weakened.md` DELETES the `bgp_decode_nolink_test` row `d580c0870` committed, while appending seven of its own. A lost update: that session wrote the file back from a copy taken before the row landed. If it commits as it stands, the record of why a test file was deleted is destroyed | `test/weakened.md`, working tree | RETRACTED, and the finding was wrong rather than late. `af92e8a04` did land the file with the row gone, but that is the file working as designed. `test/weakened.md`'s own header says "Each row accepts one test weakening in the commit that carries this file" and "A commit that weakens nothing carries the table with no rows". The ledger is PER-COMMIT and never accumulates: the `bgp_decode_nolink_test` row lives in `d580c0870`, the commit whose deletion it justifies, and `git log -p -- test/weakened.md` is where any past row is read. HEAD carrying `af92e8a04`'s own rows instead is correct. Appending the old row back would name a weakening the carrying commit does not perform, which `commit_helper.py` refuses. The reviewer read a per-commit ledger as an accumulating one; so did a later session, which recorded the same wrong conclusion before checking the header |

**Round 4 reports 0 BLOCKER and 0 ISSUE after those fixes.** Findings 6 and 7 are record
defects, fixed in one edit each, and `ai/rules/planning.md` says a round whose findings are
all record defects is the last round. Findings 8 and 9 are NOTEs and do not block.

Two reds were checked and charged elsewhere. `./le repository check` exits 1 on
`LinkPayload` and `StatePayload` in `internal/component/iface/iface.go`: both exist at HEAD,
none of this spec's eight commits touches `internal/component/iface`, and the file is another
session's uncommitted work. `internal/le/testweakened/audit.go` exits 1 over
`d580c08704c4..worktree`, and every row in it is another session's working-tree edit --
doctor MPLS, the QEMU kernel wiring test, and two RFC-tagged Python suites.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/verify/lint/matrix.go` | yes | `ls -la` 2026-08-24, 22352 bytes |
| `internal/le/` | yes | `ls -la` 2026-08-24, 7190 bytes |
| `cmd/ze/pprof_test.go` | yes | `ls -la` 2026-08-24, 1410 bytes |
| `cmd/ze/hub/bgp_decode_nolink_test.go` <!-- doc-links: ignore (deleted at d580c0870, which is what this row records) --> | no, correctly | deleted at `d580c0870`. `git ls-files` does not list it and `ls` answers "No such file or directory", so the index and the working tree agree |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-3 | a violation in a compile-out stub is reported | the row loads `cmd/ze/hub` and reports on it. On 2026-08-24 `./le verify lint run --scope "./cmd/ze/hub"` exited 1 with the `compile-out` row naming two real `unused` findings. Re-run by this closure on 2026-08-25 at `d580c0870`: exit 0, `0 issues.` from both the `capability` and the `compile-out` row, because those two findings were fixed |
| AC-7 | zero findings remain | **TRUE at `d580c0870`.** `./le verify lint run` exit 0, all 17 rows printing `0 issues.`, re-measured by this closure rather than carried from an earlier run |
| AC-8 | the blind count is 2 | `./le verify lint run-population-check` exit 0, re-run 2026-08-25: "every tracked Go file is linted, except the 2 stated above", the two being `examples/plugin/go/main.go` and `tools.go` |
| the deleted `_test.go` costs no coverage | the surviving assertion runs in a build a suite executes | `build_tag_bgp_absent_test.go` asserts `pluginreg.GetPacketDecoder()` is nil under `//go:build !ze_bgp`, and `_ze-unit-test-impl` ends with `$(GO_TEST_CORE_RACE) ./cmd/ze/hub`, whose tag set is `GO_TEST_CORE_TAGS = ze_core $(ZE_TAGS)` and carries no `ze_bgp`. Re-run by this closure: `go test -tags 'ze_core' -race ./cmd/ze/hub` exit 0, 117.5s |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le verify lint run` -> the driver -> the coverage assertion | `TestLintRunsEveryBuildFlavor` (`internal/le/verify/engine/verifyengine_test.go`) | `go test -race ./scripts/status` exit 0, 6.1s. Read: each test expands the make variables and reads the recipe lines, so it fails on a recipe that drops the driver |
| `./le changed scope` on an installer edit -> `./...` | `TestSelectorWidensForTheInstallerInitrd` (`internal/le/changed/selector_test.go`) | `go test -race ./scripts/checks` exit 0, 6.5s |
| `_./le verify lint run-impl` -> `BASE_PASSES` | `test_base_passes_match_the_lint_recipe` (`internal/le/`) | `./le verify lint run` exit 0, 8 tests. It reads the the native action tables under `internal/le/`, so a recipe change that leaves `BASE_PASSES` behind fails it |
| `./le verify lint run` -> the whole population | the coverage assertion itself | `--coverage` exit 0, residue 2 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `--build-tags` adds to the config list. Two probes recorded in Task; the `compile-out` row exists only because of the consequence |
| A-2 | broken, in the direction that helps | `golangci-lint run -h` offers no subtractive form, so a DERIVED tagless config is the mechanism (`tagless_config`). Cause 2 is reachable |
| A-3 | broken, twice | three builds did not compile: `live`, `stress`, `ze_chaos`. Each is fixed and its flavor now type-checks it |
| A-4 | confirmed, and far smaller than feared | uncapped: installer 10, capability 15, distro 1, appliance 1, compile-out 67, everything else 0 |
| A-5 | confirmed, because each flavor is SCOPED | a full-tree pass is 286s; a flavor pass is 8-39s over 1 to 59 packages. The the native action tables under `internal/le/` comment records the added minute |
| A-6 | confirmed | `./scripts/... ./cmd/ze-gok/... ./cmd/ze-serial-shell/... ./api/... ./examples/...` uncapped: 8s, 0 findings. `ZE_LINT_PKGS := ./...` was the whole fix for cause 1 |
| A-7 | broken | 245 on Linux against 144 on darwin. Fixed by NAMING every GOOS a flavor needs, so the answer stops moving with the machine |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| #10 test infrastructure changed -> `docs/contributing/testing.md` | the new section's pass table is read against `FLAVORS` and `BASE_PASSES` in `internal/le/verify/lint/matrix.go`; the `--coverage` and `--list` commands in it were both run | yes |
| #10 -> `docs/architecture/testing/verify-freshness-scope.md` | the paragraph is read against `uncompiledTreeReaders` in `internal/le/changed/selector.go`, which now holds one directory | yes |
| #10 -> `ai/rules/points/commands/lint-gate/...` and `.../precommit-verify/running-the-gate/...` | `./le doc check verify` reports "rules-points: 29 rules are fresh" and "all 29 rules round-trip byte-identical" | yes |
| #16 source anchors | `./le doc check verify` exits 0, re-run 2026-08-25. Its "Source anchors" stage reports "checked 2192 code paths, 517 packages, all references valid". The one anchor that failed on 2026-08-24 was `docs/guide/web-interface.md` naming `liveAAABundleAuthenticator.Authenticate`, never this spec's, and another session landed the fix at `dcbb0efa6` | yes |
| #1..#9, #11..#15, #17 answered No | no CLI command, RPC, YANG leaf, plugin registration, wire format, RFC row or metric is touched. `git diff` and `git show` over the whole diff name no file under `internal/component/*/yang/`, no `register.go` gaining a command, and no `docs/features/rfc-status.md` row | yes |
| `ai/DOCS-TO-CODE.md` | regenerated: `./le verify lint rundocstocode/docstocode.go --check` exit 0, "266 design docs, up to date". `git check-ignore` confirms it is gitignored and untracked, so it belongs in no commit | yes |

### Landed (round 3 found 17 files uncommitted; all have since landed)
<!-- Round 3 found the record reading as landed while 17 files were not.
     Each row below was re-attributed by `git log -1 -- <path>` on 2026-08-25,
     not by its path, and no file of this spec's diff is uncommitted now. -->
| Group | State | Evidence |
|-------|-------|----------|
| driver, the native action tables under `internal/le/`, selector, guard tests, chaos make, rpki/chaos/tinygo compile fixes, hub build constraints, rules points | committed `d16046963`, `2e0ebcf18`, `98f4297eb`, `448740364`, `f1153a3d5` | `git show --stat` on each |
| the 17 files round 3 found uncommitted -- both docs, the six `internal/install/disk` files, `conntrack_setup_appliance_linux.go`, `stress_test.go`, the five `internal/plugins/cos` files and the two `internal/plugins/diag/cmd` stubs | committed `587f86ad4` | `git log -1 --format='%h %s' -- <path>` answers `587f86ad4 fix(lint): clear what the new build-flavor passes found` for every one, and `git status --porcelain` names none of them |
| `cmd/ze/hub/main.go`, the second-signal watchdog | committed `89c35d8ed` | `git show --stat`. Ten lines in `run`, no other file but a verification-debt row |
| `cmd/ze/hub/bgp_decode_nolink_test.go` <!-- doc-links: ignore (deleted at d580c0870, which is what this row records) -->, deleted | committed `d580c0870` | `git show --stat`: the file, a verification-debt row, and the `test/weakened.md` row that records the deletion |
| nothing of this spec is uncommitted but the spec itself and `test/weakened.md` | verified | `git status --porcelain` over every path this spec names returns `test/weakened.md` alone. **That modification is NOT this closure's, and it DELETES this spec's committed row.** The working-tree diff removes the `bgp_decode_nolink_test` row `d580c0870` added and appends seven rows about the QEMU kernel-wiring test and the MPLS doctor check, which is another session's pending commit. It is a lost update: that session wrote the file back from a copy taken before `d580c0870` landed. This closure does not touch the file, because clobbering another session's in-flight edit is the worse failure, and the main thread carries the correction to whoever owns that commit. **It then committed as it stood, at `af92e8a04`, and that is CORRECT.** `test/weakened.md` is per-commit by its own header and never accumulates, so the row belongs in `d580c0870` and nowhere else. Nothing is owed. The lost-update reading was wrong, and the closure's decision not to touch an in-flight file was right for a different reason than it gave |
| the whole compile-out scope, `cmd/ze/hub` included | green at HEAD | `./le verify lint run` exit 0, all 17 rows `0 issues.` |
| `cmd/ze/hub`, `./scripts/status`, `./scripts/checks`, `internal/le/` | green at HEAD | `go test -race ./cmd/ze/hub` exit 0 (191.8s); the three flavor guards exit 0 (6.0s); the two selector guards exit 0 (6.2s); the driver's 8 Python cases exit 0 |

## Post-Migration Correction (2026-08-29, written at closure)

`eae282592` retired `make` and `scripts/` on 2026-08-28 and ported the lint
driver to Go. This spec was written against the retired producers, and a bulk
rewrite then edited its prose in place, so several names below read as paths
that no longer exist. What each one means today:

| The spec says | The producer today |
|---------------|--------------------|
| `scripts/dev/lint_flavors.py`, `FLAVORS`, `BASE_PASSES` | `flavorMatrix` and `basePasses` (`internal/le/verify/lint/matrix.go`) |
| the `ze-lint` recipe, `ZE_LINT_PKGS`, `ZE_LINT_FLAVOR_RUN` | `(*Runner).plan` and `packageRoot` (`internal/le/verify/lint/verifylint.go`). The full-tree scope is still `./...` |
| `report_coverage`, `RESIDUE`, `population()` | `(*Runner).coverage`, `residue` and `(*Runner).population`, same file. The proof still runs at the end of a full-tree run and is still skipped for a scoped one |
| `./le verify lint run-population-check` | no such action, under that spelling or any other. The bulk rewrite wrote that name into four evidence cells. The coverage proof is the last line of `./le verify lint run` |
| `TestLintRunsEveryBuildFlavor`, `TestChangedLintRunsEveryBuildFlavor`, `TestLintFlavorDriverCarriesTheLinterCeilings`, the Python `lint_flavors_test.py` cases | gone with the recipe they read. `internal/le/verify/lint/parity_test.go` pins the same properties: `TestFlavorMatrixPinsEveryCurrentBuildInOrder`, `TestPlanPinsEveryArgvEnvironmentScopeAndOrder`, `TestScopedRunParsesPackagesAndNeverBroadensToTheTree`, `TestProducerHeadingsAndCoverageOutputArePinned` |
| `uncompiledTreeReaders` (`internal/le/changed/selector.go`) | deleted. `TestSelectorWidensForTheInstallerInitrd` (`internal/le/changed/selector_test.go`) survives, runs against the live tree, and still requires the answer `./...` |
| 17 flavor rows | 18. The port added `linux-amd64`, which the base Linux pass covers only on an amd64 host |
| `docs/architecture/testing/verify-freshness-scope.md` names the installer exemption | the doc was rewritten for the native verifier. It states the general rule that deleting the exemption restored: every unresolved case widens to `./...` |

Three things the port did not change. `residue` still holds exactly
`examples/plugin/go/main.go` and `tools.go`. The `compile-out` row still derives
its tagless config on each run and removes it after. `.golangci.yml` is still
generated, and no exclusion was added to it.

The lint stage gained failure-group attribution on 2026-08-29
(`internal/le/verify/lint/failuregroup.go`), so a red run now ends with a
declared group naming the files its findings were about. A green run declares
nothing, which is why the output this spec describes is unchanged when it passes.

## Core Insight

A gate's population is a claim, and a gate cannot check its own. `golangci-lint` exits 0 over
a file it never loaded, and so does every tool that takes a pattern: the answer to "did you
find anything" is silent about "what did you look at". The fix that generalises is not another
pass. It is the ASSERTION at the end of the run that every member of the population was seen
by something, derived from the tree on each run, failing in BOTH directions -- on a file no
pass loads, and on a stated exclusion that stopped being one. Without it, a thirteen-pass gate
is thirteen chances to be silently wrong instead of one.
