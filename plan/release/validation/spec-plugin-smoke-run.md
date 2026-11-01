# Spec: plugin-smoke-run

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release validation: no run today starts every registered plugin on a config that wakes its workers, so a plugin that panics, races or exits once its timers fire can ship with every suite green |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas approved "a smoke run of every plugin: each plugin starts on a realistic
config and runs for a few seconds, and any crash fails the run". It comes from
the smoke step of the BAMA skill set: start every background timer and observer
for a few seconds, and fail on a crash. He also agreed that each plugin carries
its own small config file, and that a check rejects a registered plugin without
one, rather than picking configs out of the existing functional tests. This
spec is second in that order and is implemented in its own later session.
On 2026-11-01 Thomas approved this spec for implementation ("ok we want to fix
this gap, make sure the specs are ready"), and the open decisions D-1, D-3 and
D-4 were resolved from his words and Ze's rules (Key Design Decisions). The same
day he set the owner rule that every operator command runs through the daemon
on every command surface (MCP, REST, gRPC, SSH, web) with the same output as
the local `ze` command, so a remote operator or AI can debug the box
(`ai/META-DECISIONS.md`, Goal and scope). D-4 follows it: what the harness
reads to know a plugin is running is a daemon command, not a log line.

The problem: coverage of plugins by the functional suites is incidental and
recorded nowhere. 389 `.ci` files opt in to a `reject=stderr` panic pattern, no
run checks for a crash by default, and several recover sites keep the daemon
running after a panic, so an exit code alone says nothing. Nothing starts each
registered plugin and holds it long enough for its workers to run.

The goal: `./le test smoke` starts each registered plugin from its own fixture,
holds it a few seconds, stops it, and fails the case on any crash, recovered
panic, race report, startup refusal, unclean exit, or a plugin that never
started. Day to day it runs only the cases a change reaches; the run of every
case is release-only and no agent starts it on the Linux host.

The spec also fixes one defect it owns: `bridgeCallbackPanicError`
(`pkg/plugin/sdk/sdk_dispatch.go`) drops the panic value and stack, so a crash
the smoke run catches there could not be debugged from its report.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/system-plugins.md`, `ai/doors/bgp-plugins.md`, `ai/doors/le-tooling.md`, `ai/doors/core.md`, `ai/doors/hub-engine.md` - the doors this work crosses
  → Constraint: a plugin runs internal or forked, so each case runs both ways; code that only works internal is broken
  → Constraint: the engine never enumerates plugins, so the harness enumerates the registry, never a hand list
  → Constraint: le may link `internal/component/plugin/all`; product code never imports `internal/le`
  → Constraint: core imports nothing upward; a shared crash-signature declaration read only by le stays under `internal/le` or `internal/test`
- [ ] `docs/architecture/testing/runner-architecture.md` - the `.ci` Runner, child process lifetime, port leases, binaries a fixture reaches
  → Decision: every fork takes its attributes and cancellation from `plugin.KillGroupOnCancel`; a case owes a defer, context cancellation and `Pdeathsig`
  → Constraint: a harness command is `le test <name>` with its package at `internal/le/test/<name>`
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - change-set selection and the suite map
  → Decision: the suite map is a recorded artifact (`tmp/ze-suite-map.json`) that only a `ZE_COVER` gating run writes; a suite joins it by being listed in `Gating`, never by an edit to the map
  → Constraint: a verify run publishes its package scope as `ZE_VERIFY_SCOPE_PACKAGES`; the full population publishes `./...`
- [ ] `docs/architecture/diagnostics/crash-capture.md` - "Recovered panics"
  → Decision: `taskguard.Task.Contain` logs one ERROR line, increments `ze_task_panics_total` and writes `crash-<time>-recovered.log`; the crash directory comes from `ze.crash.dir`
- [ ] `docs/architecture/plugin/plugin-system.md` - registration fields, startup phases, the process boundary
- [ ] `docs/architecture/api/commands.md` - where a command is served (daemon dispatcher, offline fallback, local data) and `system subsystem list`
  → Decision: the started check reads a daemon-dispatched command, so it is the same answer MCP, REST, gRPC, SSH and web give; a local-data command (`show plugin list`) cannot answer it
  → Constraint: the row keys are kebab-case and additive; the existing `name`, `stage`, `running`, `command-count` keys keep their spelling
- [ ] `plan/learned/DESIGN-HISTORY.md` - Plugin system, load-bearing invariants
  → Constraint: any plugin failure aborts the whole startup tier, by owner ruling; one case per plugin is what lets a failure name the plugin
- [ ] `ai/rules/testing.md`, `ai/rules/goroutine-lifecycle.md`, `ai/rules/commands.md`
  → Constraint: the selftest must go red before the harness that satisfies it exists; no per-event goroutine in the harness

**Key insights:**
- The verdict rests on evidence the product already writes (crash records, the exit status, the runtime's own crash lines), not on a new list of log strings.
- Four recover sites sit outside `taskguard` and write no crash record; the harness cannot judge them without a string list unless they move onto the boundary.
- `show plugin list` reads the compiled registry and answers with no daemon, so it cannot say what runs. The daemon already has the running-set answer: `system subsystem list` and `show system subsystem list` list every process the process manager started, with its stage and a running flag. They do not say whether a plugin runs internal or forked, and two handlers build the same row (D-4).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` - `All` (line 833) and `Names` (851) enumerate registrations; `Register` refuses a nil `RunEngine`; `PluginsForConfigPaths` (1056) selects plugins whose `ConfigRoots` the config holds
- [ ] `internal/component/plugin/server/startup_autoload.go` - `getConfigPathPlugins` (96) autoloads on config roots and resolves `Dependencies`; `getUnclaimedFamilyPlugins`, `getUnclaimedEventTypePlugins`, `getUnclaimedSendTypePlugins` autoload beside it; a plugin already named in a `plugin { internal | external }` block is skipped there
- [ ] `internal/component/plugin/yang/ze-plugin-conf.yang` - `list internal` (181) needs `use`; `list external` (214) takes `run`, forked with `/bin/sh -c` and TLS connect-back, the ze directory on PATH so `ze plugin <name>` resolves
- [ ] `internal/component/plugin/process/process.go` - `startInternal` defers `runnerPanics().run.Contain` around the runner goroutine (`panic_guard.go`), so a panic in a plugin's own runner goroutine is contained, counted and crash-recorded and does not end the daemon; a non-zero return logs "internal plugin exited with non-zero code"
- [ ] `internal/core/taskguard/guard.go` - `Task.Contain`, `record`: crash record through `crashlog.RecordRecoveredPanic`, then the ERROR line "an internal Ze defect ended this unit; the rest of Ze keeps running"; 59 non-test files use the package
- [ ] `internal/core/crashlog/crashlog.go`, `persist.go`, `list.go` - `Init` (called first by `dispatchMain`, `cmd/ze/dispatch.go`) resolves the directory from `ze.crash.dir` first and arms `.pending/` runtime crash output; every `ze` process, a forked `ze plugin <name>` included, runs it
- [ ] `internal/component/plugin/process/delivery.go` - `safeBridgeCall` (28) recovers and logs "DirectBridge panic" with value and stack; `safeDeliverBatch` (127) logs "deliveryLoop panic recovered"; neither writes a crash record
- [ ] `internal/component/plugin/server/engine_event.go` - `invokeEngineHandler` (124) logs "engine event handler panicked" with the value and no stack, and no crash record
- [ ] `pkg/plugin/sdk/sdk_dispatch.go` - `handleBridgeCallback` (154) and `handleBridgeExecuteCommand` recover and call `bridgeCallbackPanicError` (123), which ignores its argument; nothing logs the value, and the error ends the bridge loop
- [ ] `pkg/plugin/sdk/sdk_source_policy.go` - `sourcePolicyWorker.call` (209) turns a panic into an error that carries the value but not the stack, and the worker stops
- [ ] `internal/component/plugin/startup_coordinator.go` - `PluginFailed` (240) records the first failure and fails the tier
- [ ] `internal/test/runner/runner_exec_util.go` - `childEnv` (201) adds `GOTRACEBACK=all`; `runner.go` sets it for the runner too
- [ ] `internal/component/plugin/sysproc.go` - `KillGroupOnCancel` (30)
- [ ] `internal/le/repo/inventory/plugins.go` - `pluginPackageDir` (88) derives a plugin's package from its `RunEngine` symbol
- [ ] `internal/le/test/functional/suites.go` - `Gating` (132) and `Suites`; a suite is `le test <args>`
- [ ] `internal/le/test/functional/suitemap.go` - `selectSuites` (180) intersects the published package scope with the recorded map (there is no `suitesFor` function)
- [ ] `internal/le/test/functional/binaries.go` - the functional DUT builds with `ze_core ze_distro ze_setup zetest` and the feature tags, `-cover` under `ZE_COVER`, no `-race`
- [ ] `internal/le/test/stressrepro/run.go` - `crashSignatures` (32) already declares the stderr crash signatures
- [ ] `internal/component/plugin/yang/ze-plugin-show-cmd.yang` - `show plugin list` answers the compiled registry, not the running set; `internal/component/plugin/register.go` registers it with `command.MustRegisterLocalData`, so no daemon serves it today
- [ ] `internal/core/ipc/yang/ze-system-cmd.yang` - `system subsystem list` (`ze-plugin:system-subsystem-list`): "A subsystem is a plugin process the daemon started, and the process manager owns that set"; each row carries the process name, its stage, a running flag and a command count
- [ ] `internal/component/plugin/server/system.go` - `handleSystemSubsystemList` (326) builds one row per `ProcessManager().AllProcesses()` entry from `Name`, `Stage`, `Running` and the dispatcher's `CommandCountsByProcess`
- [ ] `internal/component/cmd/show/system.go` - `handleShowSystemSubsystemList` (72), the handler of `show system subsystem list` (`internal/component/cmd/show/yang/ze-cli-show-cmd.yang`), builds the same row a second time, line for line
- [ ] `internal/component/plugin/process/process.go` - `Name` (393), `Stage` (150), `Running` (245); `config.Internal` decides `startInternal` against the fork (515); a forked plugin's pid is held in its `pluginGroup` (714), and no accessor exposes either the mode or the pid
- [ ] `cmd/ze/plugins_zetest.go`, `internal/test/plugins/` - test-only plugins under the `zetest` tag (`fakeas112`, `fakeddos`, `fakedrain`, `fakeenrich`, `fakefib`, `fakel2tp`, `fakeredist`)

**Behavior to preserve:**
- Fail-closed startup: one plugin failure still aborts the tier
- Every existing suite, `Gating` order and the suite-map semantics
- The `crash-*` file names, rotation and `show crashes` output
- `rpc.ErrBridgeFailed` stays the sentinel the bridge error wraps, so `errors.Is` callers keep working

**Behavior to change:**
- `bridgeCallbackPanicError` keeps the panic value in the error and the stack in a log line or crash record
- The internal recover sites outside `taskguard` produce the same evidence the boundary produces (Key Design Decisions, D-3)
- `crashSignatures` moves to one declaration that stress-repro and the smoke run both read
- Each `system subsystem list` and `show system subsystem list` row also carries the process mode (`internal` or `external`) and, for a forked plugin, its process id; both commands build the row through one shared builder, so they cannot drift (D-4)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Plugin registry enumeration | `internal/component/plugin/registry/registry.go` `All`, `Names` | Lists every registration the binary linked | Extends: the population of cases and of the fixture check, read from le's own link of `plugin/all`, never a hand list |
| Plugin source location | `internal/le/repo/inventory/plugins.go` `pluginPackageDir` | Derives a plugin's package from its engine symbol, for the inventory | Extends: locates `<package>/testdata/` for the fixture; no `SourceDir` field is added to `Registration` |
| Config-root autoload and plugin blocks | `startup_autoload.go` `getConfigPathPlugins`, the family, event and send-type autoload; `ze-plugin-conf.yang` `internal`, `external` | Starts plugins the config selects, or that a block names | Extends: a fixture starts its plugin the way an operator does; the forked case adds an `external` block running `ze plugin <name>`. No smoke-only start path |
| `.ci` Runner | `internal/test/runner` `Runner.runTest`, `childEnv`, `await=stderr`, `reject=stderr`, port leases, `option=needs-linux`, background `le mock` steps | Runs one daemon per `.ci` with its side processes and judges stderr and exit | Extends: the fixture is a `.ci` the Runner executes (D-1); the harness adds discovery from the registry and the crash verdict, not a second runner |
| Process-group lifetime | `internal/component/plugin/sysproc.go` `KillGroupOnCancel` | One place every fork takes its group and cancel | Extends: each case's daemon and mocks fork through it, as the Runner already does |
| Recovered-panic boundary | `internal/core/taskguard` `Task.Contain`; `internal/core/crashlog` `RecordRecoveredPanic`, `ze.crash.dir` | Logs, counts and writes `crash-*-recovered.log`; the runtime crash file is armed in `.pending/` | Extends: each case gets its own `ze.crash.dir`, and any record in it fails the case. This is the verdict source |
| Ad-hoc recover sites | `process/delivery.go` `safeBridgeCall`, `safeDeliverBatch`; `server/engine_event.go` `invokeEngineHandler`; `sdk_dispatch.go` `handleBridgeCallback`, `handleBridgeExecuteCommand`; `sdk_source_policy.go` `sourcePolicyWorker.call` | Recover without a crash record; the SDK pair drops the value | Extends: each produces the boundary's evidence (D-3), so the verdict needs no string list |
| Crash signatures | `internal/le/test/stressrepro/run.go` `crashSignatures` | The stderr substrings stress-repro treats as a crash | Extends: moved to one shared declaration both harnesses read |
| Fail-closed startup | `startup_coordinator.go` `PluginFailed`; DESIGN-HISTORY owner ruling | One failure fails the tier | Does not apply as a change: preserved. One plugin per case keeps the failure attributable |
| Functional suite list and suite map | `internal/le/test/functional/suites.go` `Suites`, `Gating`; `suitemap.go` `selectSuites`; `verify/engine/scope.go` `publishChangeScope` | Decides which suites a gating run starts from recorded reach | Extends: `smoke` becomes a `Suites` entry in `Gating`; the map learns its reach on the next `ZE_COVER` recording; inside the suite, cases are chosen from `ZE_VERIFY_SCOPE_PACKAGES` |
| Change-set selection | `./le repo changed packages`, `internal/le/repo/changed` `Selector` | Answers the packages a change reaches | Extends: a run outside verify with no plugin named selects its cases from this answer |
| Test-only plugins | `cmd/ze/plugins_zetest.go`, `internal/test/plugins/all` | Fake plugins linked into the DUT under `zetest` | Extends: the selftest plugin `fakecrash` lives here, outside the shipped population |
| Selftest of a gate | `./le test weakened selftest` | Proves a detector refuses what it must | Extends the pattern: `./le test smoke selftest` proves each failure class goes red |
| Stand-in servers | `internal/le/mock/*` (`rtr`, `radius`, `tacacs`, `syslog`, `irr`, ...) | Mock peers a `.ci` starts as background steps | Extends: fixtures for plugins that need a peer start one, on loopback |
| External engine runner | `./le mock plugin-external <name>` | Runs one engine through real TLS connect-back with no daemon config | Does not apply: the forked case goes through the daemon's own `external` block, so the real fork, token and reload path run |
| Chaos run | `./le chaos run` | Fault injection for the BGP route server | Does not apply: one plugin, one role |
| Doctor checks | `ze doctor`, `DoctorCheckPlugins` | Readiness probes on a running daemon | Does not apply: they start nothing. A doctor check can read the extended running-set rows later; this spec adds none |
| Running-set command | `ze-system-cmd.yang` `system subsystem list`; `server/system.go` `handleSystemSubsystemList`; `cmd/show/system.go` `handleShowSystemSubsystemList` | The daemon lists every plugin process it started with stage, running flag and command count, through the shared dispatcher, so SSH, web, MCP, REST and gRPC all reach it | Extends (D-4): the row gains the process mode and a forked plugin's pid, built once for both commands; the harness reads it to judge "started" and "really forked". No new command beside it |
| Compiled-plugin list | `ze-plugin-show-cmd.yang` `show plugin list`; `plugin/register.go` `MustRegisterLocalData` | Answers what the binary carries, in the caller's process | Does not apply: it cannot answer what runs. Its daemon form under the new owner rule is owned by `plan/release/fixes/spec-command-surface-parity.md` (gap G1), not by this spec |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test smoke` with plugin names, with `all`, with `check`, with `selftest`, or with no word (the change set)
- From verify: the gating run starts the `smoke` suite when the suite map selects it, with `ZE_VERIFY_SCOPE_PACKAGES` set

### Transformation Path
1. Population: le enumerates `registry.All()` from its link of `plugin/all` and locates each fixture under `pluginPackageDir(reg)/testdata/`.
2. Selection: named plugins; or `all`; or the published scope; or `./le repo changed packages`. A plugin's case is selected when a selected package is in its package's `go list -deps` closure, under the tag set the DUT builds with. A selected package under `internal/component/plugin/` or `cmd/ze` selects every case, because every case runs through it and no plugin's closure need contain it.
3. Build: one `ze` with the DUT tags plus `-race` into the run's scratch, once per run.
4. Per case, per mode (internal, then forked): a scratch directory, a fresh `ze.crash.dir` under it, the Runner executes the fixture; the harness asks the case's daemon `show system subsystem list | json` (with `ze cli -c`, through the daemon's dispatcher) until the plugin's row reads stage running with the running flag set, and in forked mode `external` with a live pid, or the start bound passes; then it holds for the hold time, asks once more (a plugin gone from the running set fails AC-9), sends SIGTERM, waits for exit within a bound.
5. Verdict per case: the failure classes in AC-3 to AC-9, each named in the failure line with the plugin, the mode and the evidence path.
6. One combined case: every selected fixture's config together, in the full run only (D-5).
7. Report through the shared `Display` and failure index, so verify's failure routing reads it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le to ze | Fork of the built `ze` through the Runner, process group, `childEnv` | No |
| Engine to plugin, internal | In-memory pipe and DirectBridge | No |
| Engine to plugin, forked | `external { run "ze plugin <name>" }`, TLS connect-back | No |
| Daemon to harness evidence | Exit status, stderr, crash directory files | No |
| Harness to daemon command | `show system subsystem list | json` through the daemon dispatcher, the same path MCP, REST, gRPC, SSH and web use | No |

### Integration Points
- `internal/le/test/functional` `Suites` and `Gating` - one new entry
- `internal/test/runner` - executes each fixture; the harness supplies discovery and the crash verdict
- `internal/le/repo/inventory` `pluginPackageDir` - exported or moved so the smoke package can call it
- `internal/core/crashlog` - the per-case crash directory the harness reads
- `internal/component/plugin/server` - one subsystem row builder, called by `handleSystemSubsystemList` and by `internal/component/cmd/show` `handleShowSystemSubsystemList`; `process.Process` gains read accessors for its mode and its forked pid

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes, by design | Plugins start through config autoload or plugin blocks, as an operator starts them |
| No unintended coupling (components stay isolated) | Yes, by design | New code sits in `internal/le/test/smoke` and test-only `internal/test/plugins/fakecrash`; the product changes are the recover sites and two added fields on the existing running-set rows |
| No duplicated functionality (extends existing, does not recreate) | Yes, by design | Runner, crash records, crash signatures and change selection reused; no second runner or signature list |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling, no wire path |
| Registration over hardcoding, outbound | Yes, by design | A new plugin adds only its own `testdata/` fixture |
| Registration over hardcoding, inbound | Yes, by design | The case list derives from `registry.All()`; `suites.go`, `suitemap.go` and `inventory/plugins.go` were searched for plugin names, and none is added to any list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A forked `ze plugin <name>` inherits `ze.crash.dir` from the daemon's environment | `dispatchMain` calls `crashlog.Init` first; the env given to `external` children is not yet read | Forked crashes land elsewhere; the forked mode passes the key explicitly | Read the external spawn env in `plugin/process`; a `fakecrash` forked case | unvalidated |
| A-2 | Every in-tree plugin can run forked through `external { run "ze plugin <name>" }` | Every plugin can run in another process (`ai/doors/system.md`) | A plugin that cannot is a defect the run reports, never an exemption | The first full run | unvalidated |
| A-3 | An `external` block for a plugin its config root would autoload replaces the autoload rather than starting it twice | `getConfigPathPlugins` skips `hasConfiguredPlugin` names | Two copies start and claim the same roots | Unit test over `getConfigPathPlugins` with both present | unvalidated |
| A-4 | le's link of `plugin/all` registers the shipped population and no `zetest` plugin | `ai/doors/le-tooling.md`; `plugins_zetest.go` is under `cmd/ze` | The fake plugins demand fixtures, or gated plugins are missing | `./le test smoke check` population compared with the plugin inventory | unvalidated |
| A-5 | A few seconds wake most workers when the fixture sets the shortest timers the YANG allows | Owner's words: "runs for a few seconds" | A worker whose period exceeds the hold never runs | Per-fixture review, recorded in the fixture's comment | unvalidated |
| A-6 | A started plugin is observable through an existing daemon command | `system subsystem list` answers `ProcessManager().AllProcesses()` with stage and running flag (`server/system.go` `handleSystemSubsystemList`); D-4 decided | The harness cannot tell started from absent, and AC-5 is vacuous | `TestSubsystemListRowCarriesMode`; the `fakecrash` mode C case goes red through it | unvalidated |
| A-7 | Every plugin the daemon starts, internal ones included, is a `Process` in `AllProcesses` | `Process.Start` branches on `config.Internal` to `startInternal` inside the same type (`process.go` 515) | An internal plugin is missing from the running set and every internal case fails as absent | The internal-mode `fakecrash` clean case passes (AC-10), and `test/plugin/subsystem-list-mode.ci` lists an internal plugin | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A full run of about 110 plugins, two modes each, under `-race`, is long | Case time times count | Full run is release-only; day to day only the reached cases run; cases run in parallel under the Runner's concurrency and port leases |
| R-2 | Fixtures conflict when merged for the combined case | Combined case fails at parse or bind | Port leases; a fixture marks itself not combinable only with a written reason (D-5) |
| R-3 | A Linux-only plugin cannot run on macOS | Visible SKIP | `option=needs-linux`; it runs under `./le test qemu` |
| R-4 | A fixture that never wakes the plugin's workers passes vacuously | Coverage of the plugin package near its `init` only | Each fixture names the workers it wakes; review reads the plugin and checks the claim |
| R-5 | The first full run finds many crashes at once | Many red cases | Each is a defect fixed under `ai/rules/completion.md`, never a fixture opt-out |
| R-6 | The two added row fields change an answer every command surface serves, and the two handlers drift again | A surface or test that pins the old row keys; the two commands answer different rows | Fields are additive, kebab-case and declared in the YANG description; one row builder serves both commands, and `TestShowSystemSubsystemListMatchesSystemSubsystemList` pins that they agree |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Tooling only, except the recover-site changes (a wrong change there could end a unit that used to continue, or lose a log line) and the running-set rows (a wrong mode or pid misleads an operator or AI debugging the box remotely) |
| How is it reverted? | Single commit revert per phase |
| Who else touches this path? | `internal/test/runner` (every `.ci` suite), `internal/le/test/functional` (verify), `pkg/plugin/sdk` (every plugin, third-party included) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test smoke selftest` | → | `internal/le/test/smoke` case runner and verdict over `fakecrash` | `TestSmokeSelftestEveryModeRed` |
| `./le test smoke check` | → | fixture check over `registry.All()` | `TestSmokeCheckRefusesPluginWithoutFixture` |
| verify gating run | → | `smoke` in `Suites` and `Gating`, cases from `ZE_VERIFY_SCOPE_PACKAGES` | `TestSmokeSelectsCasesFromPublishedScope` |
| A panic in a bridge callback | → | `bridgeCallbackPanicError` | `TestBridgeCallbackPanicKeepsValueAndStack` |
| `show system subsystem list | json` on a running daemon | → | the shared subsystem row builder, mode and pid | `test/plugin/subsystem-list-mode.ci`, `TestSubsystemListRowCarriesMode` |
| The harness's started check | → | `internal/le/test/smoke` reading the running-set rows | `TestSmokeVerdictFailsOnAbsentPlugin`, `TestSmokeSelftestEveryModeRed` (mode C) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A registered plugin in le's population with no fixture and no opt-out | `./le test smoke check` exits non-zero naming the plugin, its package and the fixture path it expects; the smoke run reports it as a failed case |
| AC-2 | A fixture whose opt-out reason is empty | Refused by the check, naming the file |
| AC-3 | `fakecrash` mode A: a goroutine the plugin starts panics after 1 s | The case fails, names the plugin and mode, and quotes the `panic:` line |
| AC-4 | `fakecrash` mode B: the plugin's event handler panics and is recovered | The case fails on the crash record in its `ze.crash.dir`, and the report prints the record's panic value |
| AC-5 | `fakecrash` mode C: the plugin's engine returns at once and starts nothing | The case fails: the plugin is absent from the started set |
| AC-6 | `fakecrash` mode D: two goroutines write one variable unsynchronised | The case fails on `WARNING: DATA RACE` |
| AC-7 | A fixture whose config the plugin refuses at configure | The case fails naming the `PluginFailed` message |
| AC-8 | The daemon exits non-zero after SIGTERM, or does not exit within the stop bound | The case fails, naming which |
| AC-9 | A plugin that exits on its own during the hold | The case fails on the unexpected exit |
| AC-10 | `fakecrash` clean mode | The case passes in both process modes |
| AC-11 | Each case | Runs once internal and once forked, and the report names both |
| AC-12 | A change set whose packages are in one plugin's dependency closure only | Only that plugin's cases run |
| AC-13 | A change set touching `internal/component/plugin/` or `cmd/ze` | Every case runs |
| AC-14 | Verify's full population (`./...`) | Every case runs, plus the combined case |
| AC-15 | `./le test smoke` with no word, no published scope and a docs-only change | No case runs, and the output says the change set reaches no plugin |
| AC-16 | A panic recovered by `handleBridgeCallback` | The error carries the panic value, still matches `rpc.ErrBridgeFailed` under `errors.Is`, and the stack is logged |
| AC-17 | A panic recovered by `safeBridgeCall`, `safeDeliverBatch`, `invokeEngineHandler` or `sourcePolicyWorker.call` | A crash record is written in the case's crash directory (D-3) |
| AC-18 | A Linux-only plugin's fixture on a non-Linux host | A visible SKIP naming the requirement, never a pass |
| AC-19 | `show system subsystem list | json` on a daemon running one internal and one forked plugin | Each row carries `mode` (`internal` or `external`); the forked row carries `pid`, a positive integer naming a live process; the internal row carries no `pid`; the existing keys (`name`, `stage`, `running`, `command-count`) are unchanged |
| AC-20 | The same daemon asked `system subsystem list` and `show system subsystem list` | Both answers carry the same rows, built by one function |
| AC-21 | The same command sent through a remote command surface (the daemon dispatcher a `ze cli -c` session reaches, which SSH, web, MCP, REST and gRPC share) | The same rows as AC-19; the command is not a local-data command and needs no `ze` binary on the caller's side |
| AC-22 | A smoke case | Its "started" judgement comes only from the running-set rows of AC-19: a plugin with no row, a row not at stage running, or (forked mode) a row not `external` with a live pid fails the case naming which, and no log line is read for it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A developer runs `./le test smoke <plugin>` after changing a plugin | le registry population, fixture under the plugin's `testdata/`, Runner, `ze` daemon with `-race`, running-set rows, crash directory, verdict, `Display` | `./le test smoke selftest` over `fakecrash`, `TestSmokeSelftestEveryModeRed` |
| 2 | A remote operator or AI asks the box which plugins run and how, over MCP, SSH or the web | surface, daemon dispatcher, `show system subsystem list`, shared row builder, `ProcessManager().AllProcesses()` | `test/plugin/subsystem-list-mode.ci` |
| 3 | Verify runs a gating pass over a change that reaches one plugin | `publishChangeScope`, `ZE_VERIFY_SCOPE_PACKAGES`, `smoke` in `Gating`, closure selection, that plugin's cases | `TestSmokeSelectsCasesFromPublishedScope` |

## Goal Validation (BLOCKING)

Planned evidence for each goal in the Task section; `/ze-close` fills the
Concrete Evidence column with the outputs.

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Each registered plugin starts from its own fixture, held a few seconds, stopped | functional | `./le test smoke check` exits 0 over the whole population; one named real plugin's case run through `./le test smoke <plugin>`, output pasted |
| Any crash, recovered panic, race, refusal, unclean exit or never-started plugin fails the case | functional, with a forced red per class | `./le test smoke selftest` output: every `fakecrash` mode red with its own reason in both process modes, the clean mode green (AC-3 to AC-11), and `TestSmokeSelftestEveryModeRed` |
| Day to day only the reached cases run; every case only on release | unit | `TestSmokeSelectsCasesFromPublishedScope`, `TestSmokeSelectsEveryCaseForPluginHost`, `TestSmokeSelectsNothingForDocsOnlyChange` outputs |
| A crash caught in the SDK bridge can be debugged from its report | unit plus functional | `TestBridgeCallbackPanicKeepsValueAndStack`; the `fakecrash` mode B report quoting the panic value |
| A remote operator or AI can see which plugins run, and how, through the daemon | functional | `test/plugin/subsystem-list-mode.ci` with explicit JSON assertions on `mode` and `pid`, run red with the fields removed and green with them |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSmokeCheckRefusesPluginWithoutFixture` | `internal/le/test/smoke/check_test.go` | AC-1 | |
| `TestSmokeCheckRefusesEmptyOptOutReason` | `internal/le/test/smoke/check_test.go` | AC-2 | |
| `TestSmokeSelectsCasesFromPublishedScope` | `internal/le/test/smoke/select_test.go` | AC-12, AC-14 | |
| `TestSmokeSelectsEveryCaseForPluginHost` | `internal/le/test/smoke/select_test.go` | AC-13 | |
| `TestSmokeSelectsNothingForDocsOnlyChange` | `internal/le/test/smoke/select_test.go` | AC-15 | |
| `TestSmokeVerdictFailsOnCrashRecord` | `internal/le/test/smoke/verdict_test.go` | AC-4 | |
| `TestSmokeVerdictFailsOnCrashSignature` | `internal/le/test/smoke/verdict_test.go` | AC-3, AC-6 | |
| `TestSmokeVerdictFailsOnUncleanStop` | `internal/le/test/smoke/verdict_test.go` | AC-8, AC-9 | |
| `TestSmokeVerdictFailsOnAbsentPlugin` | `internal/le/test/smoke/verdict_test.go` | AC-5 | |
| `TestBridgeCallbackPanicKeepsValueAndStack` | `pkg/plugin/sdk/sdk_dispatch_test.go` | AC-16 | |
| `TestRecoveredPanicSitesWriteCrashRecord` | one test per site's package | AC-17 | |
| `TestStressReproReadsSharedCrashSignatures` | `internal/le/test/stressrepro/` | the crash signatures have one source | |
| `TestSubsystemListRowCarriesMode` | `internal/component/plugin/server/system_test.go` | AC-19: `mode` for an internal and a forked process, `pid` only on the forked one, old keys unchanged | |
| `TestShowSystemSubsystemListMatchesSystemSubsystemList` | `internal/component/cmd/show/system_test.go` | AC-20 | |
| `TestSmokeStartedCheckReadsRunningSet` | `internal/le/test/smoke/verdict_test.go` | AC-22: no row, a row not running, and a forked row without `external` and a pid each fail with their own reason | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| hold time (fixture override) | 1 s to the case budget less the stop bound | the case budget less the stop bound | 0 s | above the case budget less the stop bound |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le test smoke selftest` | `internal/test/plugins/fakecrash/testdata/` | Each failure mode goes red with its named reason, the clean mode green, in both process modes (AC-3 to AC-11) | |
| `TestSmokeSelftestEveryModeRed` | `internal/le/test/smoke/selftest_test.go` | The selftest itself, so a selftest that stops discriminating fails the build, as `TestCIMustFailFixturesAllFail` does for the runner | |
| `./le test smoke <plugin>` over one real fixture | the plugin's `testdata/` | A developer runs the case their change reaches | |
| `subsystem-list-mode` | `test/plugin/subsystem-list-mode.ci` | An operator asks a daemon running one internal and one external plugin `show system subsystem list | json` and `system subsystem list | json`, and reads `mode` and `pid` (AC-19 to AC-21) | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol peer.

## Files to Modify
- `internal/le/test/functional/suites.go` - `smoke` in `Suites` and `Gating`
- `internal/le/test/stressrepro/run.go` - read the shared crash signatures
- `internal/le/repo/inventory/plugins.go` - make `pluginPackageDir` callable from the smoke package
- `pkg/plugin/sdk/sdk_dispatch.go` - `bridgeCallbackPanicError` keeps value and stack
- `pkg/plugin/sdk/sdk_source_policy.go` - the stack reaches the report (D-3)
- `internal/component/plugin/process/delivery.go`, `internal/component/plugin/server/engine_event.go` - recover with the boundary's evidence (D-3)
- `internal/component/plugin/process/process.go` - read accessors for the mode and the forked pid (D-4)
- `internal/component/plugin/server/system.go` - the shared subsystem row builder with `mode` and `pid`; `handleSystemSubsystemList` calls it (D-4)
- `internal/component/cmd/show/system.go` - `handleShowSystemSubsystemList` calls the shared builder instead of its own copy (D-4)
- `internal/core/ipc/yang/ze-system-cmd.yang`, `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` - the subsystem list descriptions name the two new fields, with a revision
- `docs/architecture/api/commands.md`, `docs/architecture/api/ipc_protocol.md`, `docs/architecture/hub-api-commands.md` - the subsystem list rows and their fields
- `docs/guide/command-catalogue.md` - declared by `internal/component/cmd/show/system.go`; checked at implementation, and edited only if it describes the subsystem list rows (it does not name them on 2026-11-01)
- `docs/functional-tests.md`, `docs/contributing/testing.md` - the new suite and when to run it
- `docs/architecture/testing/ci-format.md` - declared by `cmd/ze/plugins_zetest.go`; names the smoke fixtures as a `.ci` population that lives in plugin `testdata/` rather than under `test/`, and the `fakecrash` test-only plugin
- `docs/architecture/diagnostics/crash-capture.md` - the recover sites now covered
- `docs/plugin-development/testing.md`, `ai/patterns/plugin.md` - a new in-tree plugin owes a smoke fixture
- `ai/doors/le-tooling.md`, `ai/doors/plugin-infra.md`, `ai/doors/system-plugins.md`, `ai/doors/bgp-plugins.md` - gate rows

## Files to Create
- `internal/le/test/smoke/` - `register.go`, discovery, selection, case runner, verdict, check, selftest
- `internal/test/plugins/fakecrash/` - the selftest plugin with its modes and fixtures, linked through `internal/test/plugins/all`
- `<plugin package>/testdata/smoke.ci` - one per registered plugin (103 non-test source files call `registry.Register` today)
- `docs/architecture/testing/smoke-run.md` - the design page
- `test/plugin/subsystem-list-mode.ci` - the running-set rows through the daemon (AC-19 to AC-21)
- one shared crash-signature declaration that stress-repro and smoke both read, placed by the implementing session under `internal/le` or `internal/test`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | No new RPC (D-4 extends an existing command); `ze-system-cmd.yang` and `ze-cli-show-cmd.yang` subsystem list descriptions name `mode` and `pid`, with a revision |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | Yes | `le test smoke` and its keywords, through `leroot.Register` |
| CLI grammar (keyword before value) | Yes | `./le cli grammar`; keywords `all`, `check`, `selftest`, plugin names |
| Editor autocomplete | N-A | le only |
| Functional test for new RPC/API | Yes | `test/plugin/subsystem-list-mode.ci` for the extended answer |
| Pipe completeness | Yes | The extended rows through `| json` and `| table` in `subsystem-list-mode.ci`; the harness output goes through the shared `Display` |
| Env var registration | N-A | Reads the existing `ze.crash.dir` and `ZE_VERIFY_SCOPE_PACKAGES` |
| Doctor check for runtime dependencies | N-A | Tooling |
| Prometheus counters/metrics | N-A | Reads none; `ze_task_panics_total` already exists |
| BGP family surface | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Tooling |
| 2 | Config syntax changed? | No | None |
| 3 | CLI command added/changed? | Yes | `show system subsystem list` and `system subsystem list` gain `mode` and `pid`: `docs/architecture/api/commands.md`; the new le command is documented by its help and `docs/functional-tests.md` |
| 4 | API/RPC added/changed? | Yes | The same answer on every surface: `docs/architecture/api/ipc_protocol.md`, `docs/architecture/hub-api-commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/plugin-development/testing.md` (fixture owed); `fakecrash` is test-only |
| 6 | Has a user guide page? | No | None |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | Yes | The bridge panic error text; `docs/architecture/api/process-protocol.md` if it names it |
| 9 | RFC behavior? | No | None |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md`, `docs/architecture/testing/smoke-run.md` |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/diagnostics/crash-capture.md` |
| 13 | Route metadata keys? | No | None |
| 14 | Prometheus counters? | No | None |
| 15 | Registered plugin or inventory changed? | No | `fakecrash` is under `zetest` only |
| 16 | Changed source files with doc anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/release/validation/spec-plugin-smoke-run.md` |
| 17 | Existing docs show examples for this area? | Yes | `docs/functional-tests.md` suite table; `docs/architecture/api/commands.md` `system subsystem list` example |

## Implementation Steps

1. **Phase: Wiring** -- `le test smoke` registered, `selftest` and `check` reachable, `fakecrash` linked under `zetest`
   - Tests: `TestSmokeSelftestEveryModeRed`, `TestSmokeCheckRefusesPluginWithoutFixture` (red: stubs)
   - Files: `internal/le/test/smoke/register.go`, `internal/test/plugins/fakecrash/`
   - Verify: the commands answer, the tests fail on the stubs
2. **Phase: Running-set rows (D-4)** -- one row builder for both subsystem list commands, `mode` and `pid` from new `Process` accessors, the YANG descriptions and the API pages
   - Tests: `TestSubsystemListRowCarriesMode`, `TestShowSystemSubsystemListMatchesSystemSubsystemList`, `test/plugin/subsystem-list-mode.ci` (red before the fields exist)
   - Verify: AC-19 to AC-21 through `./le test functional plugin` naming the one test; this phase lands as its own commit, before the harness reads it
3. **Phase: Verdict and case runner** -- build with `-race`, per-case crash directory, Runner execution, the started check over the running-set rows (AC-22), hold, stop, the failure classes
   - Tests: the verdict tests, the selftest
   - Verify: every `fakecrash` mode red with its own reason in both process modes, clean green
4. **Phase: Recover sites (D-3) and the journal row** -- value and stack kept, crash records written
   - Tests: `TestBridgeCallbackPanicKeepsValueAndStack`, `TestRecoveredPanicSitesWriteCrashRecord`
   - Verify: AC-4 and AC-17 pass through the selftest; the journal row marked `fixed:`
5. **Phase: Selection and the suite entry** -- named, `all`, published scope, change set; `smoke` in `Gating`
   - Tests: the selection tests
6. **Phase: Fixtures** -- one per registered plugin, in batches by door (system plugins, BGP plugins, routing protocols, components), each batch run as named cases only
   - Verify: `./le test smoke check` green; each batch's cases run by name; a crash found is fixed, never opted out
7. **Phase: Docs** -- the pages listed above, in the same commits as the behavior they describe

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | A case cannot pass when the plugin never started (AC-5); the crash directory is per case, never shared, so one case's record never fails another |
| Correctness | The forked mode really forks: the case asserts a child process, not only the config block |
| Data flow | The case list derives from `registry.All()`; no plugin name appears in harness code |
| Rule: testing | Selftest red before the verdict exists; no fixture opt-out for a plugin that crashes |
| Rule: owner rules | No path runs every case unless `all` or the full population asked for it |
| Rule: owner rules, daemon parity | The running-set answer is daemon-served on every surface; no local-data command and no log line decides "started" |
| Rule: principles, one declaration | One subsystem row builder; the two handlers keep no copy of the row |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `le test smoke` with `all`, `check`, `selftest`, names | `./le help test smoke` |
| A fixture for every registered plugin | `./le test smoke check` exits 0 |
| Each failure class discriminates | `./le test smoke selftest` output pasted |
| SDK panic carries its value | `TestBridgeCallbackPanicKeepsValueAndStack` |
| Running-set rows carry mode and pid on every surface | `test/plugin/subsystem-list-mode.ci` output |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Plugin names given on the command line are looked up in the registry, never used as paths |
| Resource exhaustion | Every case's daemon, mocks and forked plugins end with the case (defer, cancel, `Pdeathsig`) |
| Error leakage | A panic value in an error that reaches a third-party plugin says only what the value says; nothing secret is added to it |
| Information exposure | The running-set rows add only the mode and a pid, behind the same command authorization as the rest of the row; no argv, environment or token of a forked plugin is shown |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A real plugin's case goes red | A product defect: fix the plugin, never the fixture |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The research brief framed the verdict as a scan for four recovered-panic log lines. The code has a boundary (`taskguard`) that already writes a crash record per recovered panic, used by 59 files, and the internal plugin runner goroutine is already inside it (`process/panic_guard.go`). The crash directory is therefore the general verdict; a string list would be a second copy of messages declared in code.
- The brief said a suite is "registered in the suite map". The map is a derived artifact only a `ZE_COVER` gating run writes; the suite enters `Suites` and `Gating`, and the map learns it.
- `pkg/plugin/sdk` already imports `internal/` packages (`internal/core/env`, `internal/component/plugin/ipc`), so routing its recover through `internal/core/taskguard` adds no new kind of dependency.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `recovered-panic-loses-its-value.md` | 2026-11-01 | `bridgeCallbackPanicError` drops the panic value and stack |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1 (decided 2026-11-01): the fixture is `<plugin package>/testdata/smoke.ci`, a `.ci` whose config block is the plugin's own small config, run by the existing Runner | A bare `smoke.conf` with a new header format for needs, mocks and hold time | Thomas agreed that each plugin carries its own small config file, and approved this spec for implementation. The `.ci` format already carries needs (`option=needs-linux`), mocks, `await`, `reject`, timeouts and port leases, so a new header format would duplicate it |
| D-2: the population is le's link of `plugin/all`, the shipped set | Walk `register.go` files; a hand list | The registry is the one declaration; `zetest` plugins stay outside the population by construction |
| D-3 (decided 2026-11-01): the internal recover sites outside `taskguard` move onto it, and the SDK pair keeps the panic value and stack | Keep them and scan for their log strings | A recovered panic that loses its cause is a defect (the owned journal row, `recovered-panic-loses-its-value.md`), and Ze's principle is that code which cannot answer must say so (`ai/rules/principles.md`); fixing a defect is not a question (`ai/META-DECISIONS.md`). It also gives one evidence source for every recovered panic, where a string list would drift from the messages it copies |
| D-4 (decided 2026-11-01): the harness knows a plugin started by reading the daemon's read-only running-set command, `show system subsystem list` (and its twin `system subsystem list`), extended so each row also carries the process mode and a forked plugin's pid, with one row builder serving both. The harness reads that command and nothing else for "started" (AC-22) | A debug log line per plugin reaching the running stage; a brand-new command listing running plugins | The owner rule of 2026-11-01: every operator command runs through the daemon on every surface, so a remote operator or AI must be able to see what runs, which a log line does not give. The running-set command already exists and is daemon-served (`handleSystemSubsystemList` over `AllProcesses`), so a new command beside it would be a second declaration of one fact (`ai/rules/principles.md`; `ai/META-DECISIONS.md`, "A design starts from the mechanisms Ze already has"). What it lacks for the harness, and for a remote debugger, is the mode and the pid; the extension adds exactly those. `ze doctor` and operators reach the same rows |
| D-5: one case per plugin per mode, plus one combined case in the full run only | Only the combined case; only per-plugin cases | Fail-closed startup means a combined failure names no plugin; per-plugin cases name it, and the combined case catches interaction |
| D-6: an opt-out is a line in the plugin's own fixture with a non-empty reason, judged at review | An exemption list in the harness | A list in the harness is a central enumeration; `Register` already refuses a plugin with no engine, so an opt-out is rare and stays beside the plugin |
| D-7: day-to-day selection is by dependency closure of the plugin package; the plugin host and `cmd/ze` select all | The suite map alone | The map is per suite; within the suite, closure picks the cases the change reaches, as the owner rule requires |

## Known Limitations

- A worker whose period exceeds the hold time does not run unless its fixture shortens the period (A-5). The fixture names what it wakes, and review judges it.
- The run proves a plugin survives start, idle work and stop. It does not prove the plugin's function; the functional suites own that.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-22 all demonstrated
- [ ] Goal Validation table filled with the outputs it plans
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests and `./le verify changed` over the closing commits. The run of every smoke case belongs to the release check (`./le verify current`, run by Thomas or CI); no agent runs it on the Linux host
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for the hold time
- [ ] Functional proof: `./le test smoke selftest`
- [ ] Interop tests: N-A, tooling

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-plugin-smoke-run.md` only, in the same `./le commit create` script
