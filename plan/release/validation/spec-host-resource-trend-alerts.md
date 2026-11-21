# Spec: host-resource-trend-alerts

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md`, "Goal and scope": `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it, and a problem class doctor does not report is a gap owed a doctor check. A resource that runs out slowly (kernel memory, file descriptors, the conntrack table, a disk) is such a class, and today nothing in Ze reports it before the box fails |
| RFC gaps | - |
| Depends | `plan/release/validation/spec-doctor-coverage-15-system.md` AC-16 (the host monitor starts from the hub's system start beside the SMART manager, and the `ze_host_*` gauges register whenever a registry is published). Whichever spec lands first makes that move; the other builds on it |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

The design below is not approved yet. Thomas approves it before the status moves to `ready` (`ai/META-DECISIONS.md`, "How agents work"). His answers to the open questions are in "Design presentation", "Choices for Thomas". The first version he ordered is in "First version".

## Task

Thomas asked how an operator would detect, in Ze, a kernel memory leak: a slab cache such as `kmalloc-256` that grows without bound until the router panics, a problem an operator finds today only by watching `/proc/slabinfo` by hand. He then asked whether Ze has a subsystem that records key metrics over time and alerts when one follows a pattern of change, and ordered a spec for it, with this ruling: **the rules are built in per metric, not operator-configured.**

The answer today is no. Ze publishes the current value of each system metric to Prometheus when the exporter is enabled, and leaves history and alerting to an external system. `ze doctor` has no memory, file-descriptor, conntrack or disk-growth finding.

This spec makes Ze keep a short history of a few host resources and raise a warning when a built-in rule sees a resource heading for exhaustion. The operator meets the warning in `show warnings`, `show doctor`, `show health`, `show host trend` and the metrics endpoint, with the cause in plain words and the command to run. The warning clears itself when the pattern stops.

| # | Work | Rows |
|---|------|------|
| 1 | A history of a sampled level, and the two patterns judged on it (sustained growth that projects exhaustion; fill against a hard limit), as shared primitives beside the existing traffic ones | AC-1 to AC-4 |
| 2 | One confirm-and-clear state machine for every detector, replacing the two copies the tree has | AC-5 |
| 3 | The trend monitor in the host component: always on, rules registered, state kept across a daemon restart | AC-6 to AC-11 |
| 4 | The four built-in rules: kernel memory, file descriptors, conntrack table, persistent disk | AC-12 to AC-16 |
| 5 | The operator surfaces: report bus, doctor, health, `show host trend`, metrics | AC-17 to AC-23 |
| 6 | Small defects met during the research, fixed here | AC-24 to AC-29 |
| 7 | Two tooling defects met while taking this spec to approval from its worktree, fixed here (`ai/META-DECISIONS.md`, "How agents work", the small-problem rule): a design judge in a git worktree could not record a verdict, because its read records are kept in the holding checkout under the worktree's key (commit a2ee440e05); and the design-page derivation read only the first path of a Files to Modify or Files to Create bullet, so a judge refused rows the tool never derived (commit baf7b18b8c) | AC-30, AC-31 |

## First version

Thomas ordered a first, simple version before the whole spec: one rule working end to end, so he can watch it run on a real box. It is built in the worktree branch `worktree-host-trend-alerts` and lands on `main` from his laptop. The spec stays open for everything this version leaves owed; nothing is dropped from the spec.

| In the first version | Rows | Evidence |
|----------------------|------|----------|
| The level history and the growth pattern in `internal/core/stats` (`LevelHistory`, `Growth`); the fill pattern is not built | AC-1, AC-2, AC-3 | Commit 6cf4b12c26: `TestLevelHistoryBucketsMinimumAndLast`, `TestLevelHistoryKeepsFallingLevel`, `TestLevelHistoryRecordsMissingBucket`, `TestGrowthHoldsOnSustainedRise`, `TestGrowthFalseWhenRiseSettles`, `TestGrowthRunEndsAtMissingBucket`, `TestGrowthRunRestartsAfterFlatBucket`, `TestLevelParamsGrowthRisesBounds` and the rest of `internal/core/stats/level_test.go` |
| The confirm-and-clear state machine moved from ddos-detect to `internal/core/stats` as `stats.Hysteresis`, and ddos-detect converted to it, in a commit of its own | AC-5, ddos-detect half | Commit 2829eaf0da: `TestHysteresisConfirmAndClear` and the seven moved `state_test.go` tests in `internal/core/stats/hysteresis_test.go`, judged a move by ze-test-judge; the ddos-detect package tests pass with no assertion changed |
| The trend monitor in `internal/component/host/trend`, started from the hub's system start beside the SMART manager, running with no `telemetry` and no `bgp` block; the rule registers itself, its code and its check from its own file | AC-6, AC-7, AC-8 | Commit d2376b7e29: `TestRuleRegisteredIsSampledAndListed`, `TestReaderErrorAndNotPresentAreDistinctStates`, `TestValidateRuleRefusesEachMissingField`. Commit cfe98a9853: `host-trend-runs-without-telemetry.ci`, red with `startTrendMonitor` removed from the hub ("the host trend monitor is not running") and green restored |
| One rule, `kernel-memory`, without the slab cache names, read by `host.(*Detector).DetectKernelMemory` | AC-12 | Commit d2376b7e29: `TestTrendMonitorRaisesAndClearsThroughRunLoop` and `TestKernelMemoryRuleQuietOnGrowthThatSettles`, each red under a recorded break (clearing judgment dropped; not-rising counted as growth) and green restored; `TestDetectKernelMemoryReadsUnreclaimableSlab`, `TestDetectKernelMemoryRefusesMissingLine`, `TestDetectKernelMemoryNamesUnreadablePath` |
| The warning on the report bus with its fix, refreshed keeping its raise time; the rule's own runtime doctor check; the check that reports the monitor not running; stop clears only the monitor's own warnings | AC-17, AC-18, AC-19, and the warning half of AC-11 | Commit d2376b7e29: `TestTrendMonitorRaisesAndClearsThroughRunLoop`, `TestTrendDoctorCheckRowFollowsRuleState`, `TestTrendMonitorNotRunningIsReported`, `TestTrendMonitorStopClearsOnlyItsOwnWarnings`. Commit cfe98a9853: `host-trend-doctor.ci` |
| `show host trend` listing the rule and its state, answering an operator error when no monitor runs, and an offline fallback that says the command needs the daemon | AC-21, for `kernel-memory`, without the cache names; the command half of AC-19 | Commit cfe98a9853: `TestShowHostTrendListsWarmingUpRule`, `TestShowHostTrendListsProjectionOfActiveRule`, `TestShowHostTrendRefusesWhenMonitorNotRunning`, `TestShowHostTrendOfflineHasItsOwnFallback`, `host-trend-show.ci` through the json and yaml pipes |
| The design judge in a git worktree finds its session's read records | AC-30 | Commit a2ee440e05: `TestJudgeInAWorktreeReadsTheRecordsItsSessionKeeps`, red before the fix, and `TestJudgeInAWorktreeRefusesAReadOfTheHoldersCopy` |
| Every path of a file bullet owes its design pages | AC-31 | Commit baf7b18b8c: `TestBulletPathsStopsAtTheDescription`, `TestSpecSourceFilesTakesEveryPathOfABullet`, `TestDesignDocumentsTakeEveryPageOfABullet`, each red without the rule it pins |

The proof Thomas asked for is the monitor's run loop driven by a fake clock over a fixture `/proc`: a leak raises the warning and the warning clears when the growth stops, and memory that rises and then settles raises nothing (`TestTrendMonitorRaisesAndClearsThroughRunLoop`, `TestKernelMemoryRuleQuietOnGrowthThatSettles`). The wiring tests `host-trend-runs-without-telemetry`, `host-trend-show` and `host-trend-doctor` prove the operator reaches it through the daemon (`./le test bgp plugin --pattern host-trend-`, 3 of 3 pass at commit cfe98a9853).

The rule's figures (6 consecutive rising hours, a rise of at least 1% of total memory, exhaustion projected within 7 days, confirm and clear after 3 evaluations) are used as written. They stay unmeasured until the lab run of phase 10 (assumption A-2).

| Owed after the first version | Rows |
|------------------------------|------|
| The fill pattern | AC-4 |
| anomaly-detect converted to the shared state machine | AC-5, anomaly-detect half |
| A reload proven to leave the monitor and its history alone | AC-9 |
| The history kept across a daemon restart, and its save at stop | AC-10, and the save half of AC-11 |
| The slab cache names from `/proc/slabinfo` | AC-13, and the cache list of AC-21 |
| The other three rules: file descriptors, conntrack, disk, each in its own `register_<rule>.go` | AC-14, AC-15, AC-16, and their rows of AC-21 |
| The `host-trend` health row | AC-20 |
| The `ze_host_trend_*` metrics | AC-22 |
| The doctor-coverage acceptance of the host component | AC-23 |
| Work group 6, the adjacent defects | AC-24 to AC-29 |
| The lab run that measures the figures | Phase 10, A-2 |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - process boundary, registration, compile tags, shared machinery
  → Constraint: "A compile tag is a boundary, and code MUST NOT reach across it": always-on code cannot import `internal/component/telemetry/collector` (gated by `ze_telemetry`) or `internal/plugins/ddos/detect` (gated by `ze_ddos`). The sampler therefore cannot be the telemetry collectors, and the hysteresis machine cannot be imported from ddos-detect
  → Constraint: "Persist runtime state: In the daemon, `statestore` under a key in `pkg/zefs/keys.go`, never `os.WriteFile`"
  → Constraint: "A goroutine: A long-lived worker on a channel, never one per event"
  → Constraint: "A readiness check: A doctor check registered by the package that owns the dependency"
  → Constraint: "A feature registers from `init()` in its own `register.go` and is discovered": a new rule must not need an edit to a list in the monitor, the doctor check, the show handler or the metrics
- [ ] `ai/doors/core.md` - core is the leaf tier; statestore; registries
  → Constraint: `internal/core/` imports neither `internal/component/` nor `internal/plugins/`, so the shared primitives go in `internal/core/stats` and take no component type
  → Constraint: "`statestore.Put` reports nothing when there is no store": the monitor uses the acknowledged calls (`Write`, `Read`) so a failed save is seen
  → Constraint: "With no engine linked, it answers `ErrNotRegistered`, never an empty list": a reader of the monitor's state answers "not running" when the monitor never started, never an empty healthy list
- [ ] `ai/doors/hub-engine.md` - boot, reload, stop
  → Constraint: "Something built only at startup ignores every reload" and "a reload rebuilt and swapped a live bundle": the monitor has no config, so a reload neither restarts it nor discards its history
  → Constraint: `cmd/ze/hub` reaches gated services only through a registration seam; the host component is always linked (it has no row in `feature-gates.txt`), so the hub may start it directly, as it starts the SMART manager
- [ ] `ai/doors/api-surfaces.md` - the exporter and its metric naming
  → Constraint: Prometheus and Netdata dashboards depend on existing metric names and labels: no existing `netdata_*` or `ze_host_*` metric is renamed; this spec only adds names
- [ ] `ai/doors/system-plugins.md` - the `*-cmd` command providers
  → Constraint: "A command-only plugin (`host-cmd`...) owns the command YANG and the RPC handlers and delegates the work to a component": `show host trend` is declared in `internal/plugins/host-cmd`, and the trend state stays in `internal/component/host`
- [ ] `ai/doors/le-tooling.md` - the gates and the test trees
  → Constraint: a bare `go test` omits the feature tags; package tests run through `./le test unit pkg path <dir>/`
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases, the runtime phase, recent-events, codes, Fix
  → Constraint: a runtime check registers with `diagnostic.RegisterDoctorCheck` and `DoctorPhaseRuntime` from `init()`; only `show doctor` runs it, and the local `ze doctor` asks the daemon
  → Constraint: an active warning whose code a runtime check declares is skipped by `recent-events` (`ownedCodes`), so the owning check writes the row. Each rule therefore owns a runtime check that declares its code
  → Decision: the fix travels on the raise (`report.RaiseWarningFix`), not in `CodeMeta`
- [ ] `docs/architecture/host/observability.md` - hardware-change events from inventory diffs
  → Decision: the host component is the place that samples the host and raises report-bus warnings from what it sees; the trend monitor extends it
- [ ] `docs/architecture/traffic/traffic-analysis-layers.md` - the shared traffic-analysis primitives
  → Constraint: `internal/core/stats` holds "domain-NEUTRAL statistical primitives", "pure math and small stateful helpers with no component imports", none of which reads the clock: the caller passes time in
- [ ] `ai/rules/user-facing-errors.md` - every message says what is wrong, why, and the exact command
  → Constraint: each warning names the resource, the growth observed, the projected time to exhaustion and the command to run next

### RFC Summaries (Scope: protocol)
N-A: Scope is plugin. No RFC is implemented or changed.

**Key insights:** (minimal context to resume after compaction)
- System sampling is not always on today: the OS collectors run only when the Prometheus exporter is enabled, the host monitor additionally needs a `bgp` block, and every sample goes write-only into a gauge. No code in the daemon can read a sample back
- `internal/core/stats` has no primitive for a level: `Window` drops a falling value and stores only rates; nothing computes a slope or a run of growth
- A slow leak defeats a deviation detector: the anomaly detector's baseline learns the ramp, so the leak never reaches its threshold. The rule must be "sustained growth that projects exhaustion", not "deviation from a baseline"
- The vendored `github.com/prometheus/procfs` already parses `/proc/slabinfo` (`slab.go`); no Ze code calls it
- The report bus dedups on (source, code, subject); a re-raise refreshes the message and keeps the raise time; `ClearWarning` removes one key
- ddos-detect persists its baseline through statestore and validates it on load: that is the precedent for keeping history across a daemon restart

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/telemetry/exporter/exporter.go` - `Start` returns early when `cfg.Enabled` is false and calls `collector.StartOSCollectors` only when `cfg.Netdata.Enabled`; built only under `ze_telemetry`
- [ ] `internal/component/telemetry/collector/collector.go` - `Collector` is `Name`, `Init(reg, prefix)`, `Collect() error`; `Manager.Start` runs a one-second ticker and each collector at its own interval; values go into gauges only, no accessor, no hook after `Collect`
- [ ] `internal/component/telemetry/collector/platform_linux.go` - `registerPlatformCollectors` registers 31 collectors in one central list
- [ ] `internal/component/telemetry/collector/memory_linux.go` - reads `/proc/meminfo` through procfs `Meminfo`; exports `mem.slab` `unreclaimable` from `SUnreclaim` and `mem.available`; the parsed struct is not kept
- [ ] `internal/component/telemetry/collector/vmstat_linux.go` - `newVMStatCollector(interval)` keeps the manager's global interval and divides counter deltas by it
- [ ] `internal/component/bgp/config/loader_create.go` - `CreateReactorFromTree` is the only start of the host monitor: when `metrics.StartExporter` returns a registry it builds `host.NewCachedDetector` with a 60 second cache, calls `CollectOnce` and `StartRefresh(30 * time.Second)`; nothing stops it
- [ ] `cmd/ze/hub/main_system.go` - `startSmartManager` and `stopSmartManager` start and stop the SMART manager from the hub's system start; `startStandaloneTelemetry` starts the exporter without BGP and starts no host monitor
- [ ] `cmd/ze/hub/main.go` - calls `startSmartManager(loadResult.Tree)`
- [ ] `cmd/ze/hub/main_reload.go` - calls `reloadSmartManager(parsedTree)`
- [ ] `internal/component/host/memory_linux.go` - `readMeminfo` is the host's own `/proc/meminfo` parser (`meminfoFields`); it reads total, free, available, buffers, cached, swap and hugepages, and no slab field
- [ ] `internal/component/host/metrics.go` - `HostMetrics.CollectOnce`, `StartRefresh` (a raw goroutine around `time.NewTicker`), `emitDiffEvents` raising `report.RaiseWarning("host", ...)` and `report.RaiseError` for ECC; the package never calls `ClearWarning`
- [ ] `internal/component/host/diff.go` - `DiffInventory` compares two snapshots and returns carrier, ECC and throttle events
- [ ] `internal/component/host/fsroot_linux.go` - `Detector.procPath` and `sysfsPath` join under `Detector.Root`, so a test redirects the host readers at a fixture tree
- [ ] `internal/core/stats/window.go`, `ewma.go`, `math.go` - `Window` is a byte-counter accumulator (non-positive deltas ignored, decreases clamped to zero, history of rates); `EWMA`; `Mean`, `StdDev`, `Quantile`. No slope, no run length, no clock
- [ ] `internal/plugins/ddos/detect/state.go` - `stateMachine` with states idle, confirming, active, clearing: confirms after `confirmDuration` consecutive evaluations above, clears after `clearChecks` consecutive below, one evaluation above returns clearing to active
- [ ] `internal/plugins/anomaly/detect/detector.go` - `stepEntity` repeats the same confirm and clear logic inline with `above` and `below` counters
- [ ] `internal/plugins/ddos/detect/persist.go` - `saveBaselines` and `loadBaselines` under `zefs.KeyDDoSDetectBaseline`: versioned JSON, saved every 300 ticks and on stop, refused on load when too short or holding an invalid value
- [ ] `internal/core/report/report.go` - `RaiseWarning`, `RaiseWarningFix`, `ClearWarning`, `ClearSource`; key (source, code, subject); bounds on each field; an empty source, code or subject is refused with a Debug line only
- [ ] `internal/core/report/health_probe.go` - `HealthProbeDegraded(codes...)` turns active warnings into a health row
- [ ] `internal/component/doctor/checks_recent_events.go` - `checkRecentEvents` reports bus entries; every row, an active warning included, ends "It may have recovered, so the box still reads ready; check that it no longer happens."
- [ ] `internal/le/arch/doctorcoverage/sites.go` - `reportSite` recognizes `RaiseWarning` and `RaiseError` only; the `RaiseWarningFix`, `RaiseErrorFix` and `RaiseErrorFixOnce` variants are not judged
- [ ] `internal/component/storage/manager.go` - `checkTemperature` writes `prevTemp` and `checkHealth` writes `healthReported` outside `m.mu`, while `Status` copies the device under the read lock (read, not run under the race detector)
- [ ] `internal/core/health/registry.go` - the handler answers 200 for healthy and degraded and 503 for down
- [ ] `docs/architecture/diagnostics/production-diagnostics.md` - says the handler "answers 200 when every check passes and 503 otherwise"
- [ ] `internal/plugins/host-cmd/yang/ze-host-cmd.yang` - `show host`, `show host cpu`, `nic`, `dmi`, `memory`, `thermal`, `storage`, `kernel`, `platform`
- [ ] `internal/test/sim/sim.go` - `FakeClock` with `Add`, `Set` and ticker firing, implementing `internal/core/clock.Clock`
- [ ] `vendor/github.com/prometheus/procfs/slab.go` - the `/proc/slabinfo` parser, with per-cache object count and object size

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every existing metric name and label of the exporter and of `ze_host_*`
- The answer shapes of the existing `show host` commands, `show warnings`, `show errors`, `show health` and `show doctor`
- ddos-detect and anomaly-detect behavior: the same confirm and clear decisions on the same inputs, proven by their existing tests
- The telemetry collectors stay the source of the Prometheus metrics and keep their gating
- The host inventory diff events and their codes (their renaming is `spec-doctor-coverage-15-system` AC-2, not this spec)

**Behavior to change:** (only what the user asked for)
- The host component gains a trend monitor that samples four resources whether or not telemetry or BGP is configured
- New warnings, doctor rows, a health row, a `show host trend` command and metrics for those four resources
- The six small defects of work group 6

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Host monitor | `internal/component/host/metrics.go` `HostMetrics.CollectOnce`, `StartRefresh`, `emitDiffEvents` | Samples the hardware inventory on a timer and raises report-bus warnings from the difference between two snapshots | Extends: the trend monitor is the same component doing the same job (sample the host, judge, raise) for levels over a window, started and stopped on the same path |
| Host readers under a test root | `internal/component/host/fsroot_linux.go` `Detector.procPath`; `memory_linux.go` `readMeminfo` | Reads `/proc` and `/sys` under `Detector.Root` | Extends: `readMeminfo` gains the slab fields, and the new readers (file-nr, conntrack, slabinfo) use the same root so tests feed them a fixture tree |
| Telemetry OS collectors | `internal/component/telemetry/collector` `Manager`, `Collector` | Sample 31 OS sources into Prometheus gauges while the exporter is enabled; a rate collector such as `newVMStatCollector` divides its counter deltas by the manager's global interval, even when its own `interval` overrides it | Does not apply as the trend source: gated by `ze_telemetry` and by the exporter being enabled, and write-only. Always-on code cannot import it, and it stays the exporter's source. Extended by AC-26 only: the interval a rate collector actually runs at reaches it, so its per-second rates are divided by that interval |
| host-cmd `show host` RPCs | `internal/plugins/host-cmd/cmd/show_host.go` `registerShowHost`, `dispatchHostSection` | One `ze-host:show-<section>` RPC per `host.SectionNames` entry, registered with `pluginserver.RegisterRPCs`; each handler calls `host.DetectSection` in the daemon and answers JSON data | Extends: `show host trend` is one more host-cmd RPC (`ze-host:show-trend`), registered the same way from `show_trend.go`, answering data through the shared renderer. It is not an inventory section, so it is not in `host.SectionNames` |
| SMART published pointer | `internal/component/storage/show.go` `SetStorageManager`, `storageManagerPtr`, `handleShowStorageSmart` | The hub stores the running SMART manager in an `atomic.Pointer`; the show RPC loads it and answers an error when it is nil, never an empty device list | Extends the pattern: `trend.Start`, which the hub's `startTrendMonitor` calls, publishes the monitor in an `atomic.Pointer` (`published` in `internal/component/host/trend/monitor.go`) and its stop removes it; the `show host trend` handler reads it through `trend.Rules` and answers an operator error naming the fix when no monitor runs (AC-19) |
| Published metrics registry and the host gauges | `internal/component/bgp/config/loader_create.go` `CreateReactorFromTree`; `internal/component/plugin/registry` `SetMetricsRegistry`; `internal/component/host/metrics.go` `RegisterMetrics` | When `metrics.StartExporter` returns a registry, the loader publishes it to the reactor and to `registry.SetMetricsRegistry`, and `host.RegisterMetrics` registers the `ze_host_*` inventory gauges on it | Extends: the `ze_host_trend_*` gauges register on the same published registry when one exists (AC-22), from the trend package, because the host package cannot import the monitor. No existing `ze_host_*` name changes; with no registry the monitor still runs |
| Doctor recent events | `internal/component/doctor/checks_recent_events.go` `checkRecentEvents`, `ownedCodes`, `recentEventsDiagnostic` | One warning row per error event and per active warning no runtime check owns; every row ends "It may have recovered, so the box still reads ready; check that it no longer happens." | Extends: each trend rule's check declares its code, so `ownedCodes` keeps its warning out of this check (AC-18). AC-24 changes only the closing sentence of a row whose warning is still active, and every row stays a warning |
| Doctor-coverage gate | `internal/le/arch/doctorcoverage/sites.go` `reportSite` | Recognizes `report.RaiseWarning` and `report.RaiseError` as report-bus sites and judges their code; any other name returns no site | Extends (AC-25): `RaiseWarningFix`, `RaiseErrorFix` and `RaiseErrorFixOnce` are judged the same way, so the trend monitor's `RaiseWarningFix` sites are held to a declared code |
| Traffic statistics primitives | `internal/core/stats` `Window`, `EWMA`, `Mean`, `StdDev` | Rolling byte-rate window, moving average, basic statistics; no clock inside | Extends: the level history and its growth and projection functions join the package under the same contract (pure, caller passes time, caller serializes) |
| ddos-detect state machine | `internal/plugins/ddos/detect/state.go` `stateMachine` | Confirm after N consecutive evaluations above, clear after M below | Replaces: the type moves to `internal/core/stats`, exported, and ddos-detect uses it from there. No copy remains in ddos-detect |
| anomaly-detect inline hysteresis | `internal/plugins/anomaly/detect/detector.go` `stepEntity` | The same confirm and clear logic with two counters | Replaces: converted to the shared type, so the tree holds one implementation instead of a third |
| anomaly-detect baseline scoring | `internal/plugins/anomaly/detect/score.go` | Scores deviation from a learned per-entity baseline | Does not apply: its entities are a closed set of traffic kinds, and a baseline learns a slow ramp instead of flagging it |
| ddos-detect baseline persistence | `internal/plugins/ddos/detect/persist.go` `saveBaselines`, `loadBaselines` | Saves a versioned snapshot through statestore and validates it on load | Extends the pattern: the monitor saves its history the same way under its own key, and additionally refuses a snapshot from another boot |
| Report bus | `internal/core/report/report.go` `RaiseWarningFix`, `ClearWarning` | Holds active warnings keyed by (source, code, subject), shown by `show warnings` | Extends: the monitor is a new producer; no change to the bus |
| Health probe from the bus | `internal/core/report/health_probe.go` `HealthProbeDegraded` | Derives a health row from active warning codes | Extends: one probe registered for the trend codes, so the bus stays the one source of the alert |
| Runtime doctor checks | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck` | A package registers a check that reads its live state | Extends: each rule registers its own runtime check declaring its code |
| SMART `temp-rising` | `internal/component/storage/manager.go` `checkTemperature` | Compares two consecutive polls | Does not apply as a trend rule: a two-sample delta is not a trend, flaps, and cannot see a slow rise, so `checkTemperature` keeps its two-poll rule; its codes are `spec-doctor-coverage-15-system` work. Changed by AC-27 only: the poll's writes of `prevTemp` (in `checkTemperature`) and of `healthReported` (in `checkHealth`) move under the manager's lock, so `Status` copies a consistent device |
| Static disk-space check | `internal/component/doctor/checks_storage.go` `checkDiskSpace` | Reports a config partition that is nearly full at the time doctor runs | Extends: the disk rule judges growth on the same partitions and leaves the fill level to this check |
| Fake clock | `internal/test/sim/sim.go` `FakeClock` | Drives `clock.Clock` timers in tests | Extends its use: the monitor takes a `clock.Clock`, so a test runs days of samples at once |

## Design documents

Derived with `./le spec design documents spec plan/release/validation/spec-host-resource-trend-alerts.md` at commit baf7b18b8c, which reads every path of every file bullet (49 pages, 2 decision rows), plus rows added beyond it: a second section of `docs/architecture/host/inventory.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/core-design.md` and `docs/contributing/spec-workflow.md`, and `docs/features/ai-first.md`, the page of the registration this design relies on.

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | The door every change reads, and the door covering every file of the spec | "A compile tag is a boundary, and code MUST NOT reach across it." | agrees | The monitor lives in the always-linked host component and the shared primitives in `internal/core/stats`. Neither imports the `ze_telemetry` collectors or the `ze_ddos` detector, and each rule registers from its own file |
| `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` | Packages | The `// Design:` page of `internal/plugins/ddos/detect/state.go` | "The plugin packages are independent and individually removable." | agrees | The state machine leaves ddos-detect for the core leaf `internal/core/stats`, as the event contract sits in a core leaf, so ddos-detect stays removable and the host component reaches the machine without importing a gated plugin |
| `ai/doors/system-plugins.md` | Where it sits | The door covering `internal/plugins/ddos/detect/state.go`, `internal/plugins/anomaly/detect/detector.go` and `internal/plugins/host-cmd/yang/ze-host-cmd.yang` | "the RPC handlers and delegates the work to a component or core package." | agrees | `show host trend` is declared in host-cmd's YANG and its handler reads the trend state the host component owns. The read is a same-process call, as `show host <section>` already makes through `host.DetectSection`: host-cmd registers its handlers with `pluginserver.RegisterRPCs`, which the daemon's dispatcher serves (A-3) |
| `docs/architecture/anomaly/anomaly-1-detect.md` | Freeze-learn: score first, fold later | The `// Design:` page of `internal/plugins/anomaly/detect/detector.go`, anchored to it | "`onTick` folds them only when the entity is not anomalous" | agrees | The conversion replaces only the confirm and clear counters of `stepEntity`. The per-tick `above` answer that gates the baseline fold stays, so freeze-learn is unchanged (AC-5, A-7) |
| `docs/architecture/anomaly/anomaly-3-observe.md` | Why the detector's ring is not enough | Anchored to `internal/plugins/anomaly/detect/detector.go` | - | silent | The page is about the incident store and the ring `activate` fills. The conversion still calls `activate` once per confirmation, so no event and no ring entry changes |
| `docs/architecture/host/inventory.md` | Decisions | The `// Design:` page of `internal/component/host/memory_linux.go` and `metrics.go`, anchored to `memory_linux.go` | "A test-injectable filesystem root" | agrees | Every trend reader reads under `Detector.Root` through `procPath`, so the run-loop test feeds the monitor a fixture `/proc` |
| `docs/architecture/host/inventory.md` | Decisions | Added: the same page, the decision on parsers | "Standard library only." | agrees | The trend readers parse `/proc` with the host's own code, as `readMeminfo` does. The cache-name reader of AC-13 parses `/proc/slabinfo` the same way, and does not call the vendored procfs parser the Key insights name |
| `docs/architecture/appliance/gokrazy-build-pins.md` | Proving a boot, and three ways it lied | Anchored to `internal/component/host/memory_linux.go` | "A page count is not kilobytes." | agrees | `SUnreclaim` and `MemAvailable` are kB lines, so they join `meminfoFields` (kB to bytes) and never `meminfoCounts` |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Current Ze State | Anchored to `internal/component/telemetry/collector/collector.go` | "plugins and most components use the abstract registry rather than Prometheus metric types" | agrees | The trend gauges register on the `metrics.Registry` the loader publishes, like the `ze_host_*` gauges (AC-22). AC-26 changes only how a rate collector divides its deltas |
| `docs/architecture/host/observability.md` | Decisions | Anchored to `internal/component/host/memory_linux.go` and `cmd/ze/hub/main_system.go`; the `// Design:` page of the trend package files, `show_trend.go` and `trend_offline.go`; a page this spec modifies | "The diff engine returns values, it does not publish." | agrees | The growth pattern in `internal/core/stats` answers and publishes nothing; the monitor turns the answer into a report-bus warning, as the caller of `DiffInventory` does. The page gains the trend monitor, its start and its rules (AC-29) |
| `docs/features/ai-first.md` | Shared Registrations | Added: the `// Design:` page of `internal/component/host/register.go`, the host component's registration file, which the trend monitor's own `register.go` follows | "The core design point is broader than AI. A feature added to Ze should avoid" | agrees | The rule registers its doctor code and its runtime check, and `show host trend` is a YANG node, so every command surface reaches them with no glue of its own |
| `docs/architecture/hub-architecture.md` | The hub owns the shared singletons | The `// Design:` page of `cmd/ze/hub/main_system.go`, anchored to it | "Some resources must exist exactly once in the daemon, and the hub is where they" | agrees | The hub builds the one trend monitor at system start beside the SMART manager and publishes it to the `show host trend` handler and the doctor checks, as `SetStorageManager` publishes the SMART manager to its show RPC |
| `docs/architecture/core-design.md` | 20. System Update Backend | Anchored to `cmd/ze/hub/main_system.go`; the `// Design:` page of `internal/component/telemetry/collector/collector.go` | - | silent | The section is the update backend the hub starts beside the SMART manager; the monitor neither replaces nor reads it. The page's collector text is not changed by AC-26, which corrects how a rate is divided |
| `docs/architecture/diagnostics/path-mtu.md` | The reference address | Anchored to `cmd/ze/hub/main_system.go` | - | silent | The anchor is `publishNameServers`, which this spec does not touch; the monitor publishes no name server and adds no reload step |
| `docs/architecture/resolve.md` | Construction | Anchored to `cmd/ze/hub/main_system.go` | - | silent | The anchor is `newResolvers`, which this spec does not touch |
| `ai/doors/hub-engine.md` | Traps | The door covering `cmd/ze/hub/main_system.go` | "Something built only at startup ignores every reload." | agrees | The monitor has no config, so a reload has nothing to apply to it. The hub starts it once and no reload step rebuilds it, which keeps its history (AC-9) |
| `docs/architecture/zefs-format.md` | Runtime state through statestore | The `// Design:` page of `pkg/zefs/keys.go`, anchored to it | "`Put` retains its best-effort no-store result for unwired callers; operations" | agrees | The history snapshot is a daemon key registered in `pkg/zefs/keys.go`, written with the acknowledged `statestore.Write` and read with `Read`, never `Put`, so a failed save is seen (AC-10) |
| `docs/architecture/container/image-store.md` | State keys | Anchored to `pkg/zefs/keys.go` | - | silent | Those keys are granted to the container plugin; the trend key is a daemon key with no plugin grant |
| `docs/architecture/diagnostics/debug-filtering.md` | Profiles are one JSON value | Anchored to `pkg/zefs/keys.go` | "One key stores the complete state, so a save and a restore are" | agrees | The trend history is one versioned value under one key, so a save and a load are whole |
| `docs/architecture/fleet-config.md` | Roles | Anchored to `pkg/zefs/keys.go` | - | silent | The anchor is the managed hub keys; the trend key has nothing to do with fleet configuration |
| `docs/architecture/irr-owner.md` | Stored keys and notices | Anchored to `pkg/zefs/keys.go` | - | silent | The anchor is the IRR keys and notices; the trend key adds no notice and no plugin writer |
| `docs/architecture/storage-backends.md` | Content-addressed history | Anchored to `pkg/zefs/keys.go` | - | silent | The anchor is the config version keys; the trend snapshot is runtime state, not a config version |
| `ai/doors/core.md` | Where it sits | The door covering `pkg/zefs/keys.go` and `internal/core/stats/level.go` | "`internal/core/` is the leaf tier." | agrees | The level history, the growth pattern and the state machine take numbers and times and no component type |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | The `// Design:` page of `internal/component/doctor/checks_recent_events.go` and `internal/le/arch/doctorcoverage/sites.go`, anchored to both | "Only `show doctor` runs it:" | agrees | Each rule's check and the monitor-not-running check register through `diagnostic.RegisterDoctorCheck` in the `runtime` phase, because the monitor's state lives only in the daemon |
| `docs/architecture/doctor-and-health-checks.md` | Recent events: the report bus in show doctor | Added: the same page, the section AC-18 and AC-24 bear on | "The producer knows the command that fixes its subject, so the fix travels on" | agrees | The rule raises with `report.RaiseWarningFix`, fix `show host trend`, and its own runtime check declares its code, so `recent-events` adds no second row (AC-18). AC-24 keeps every row a warning and changes only the closing sentence of a row whose warning is still active. The page owes that edit under AC-24: its Severity row ("the event already happened and its owner may have recovered") holds then only for an error event, and gains the wording of a warning still active |
| `docs/architecture/behavior/fsm-open-sent.md` | Connection diagnostics | Anchored to `internal/component/doctor/checks_recent_events.go` | "They remain history if a later connection succeeds." | agrees | These are error events. AC-24 changes only the row of a warning still active, so their rows keep "It may have recovered" |
| `docs/architecture/diagnostics/production-diagnostics.md` | Health registry | Anchored to `internal/component/doctor/checks_recent_events.go`; a page this spec modifies (AC-28) | "handler answers 200 when every check passes and 503 otherwise" | departs | The design reads `show health` degraded while a trend warning is active, with the endpoint still answering 200 (AC-20). The page says 503 whenever a check does not pass. The code agrees with the design: `Registry.Handler` (`internal/core/health/registry.go`) answers 503 only for `StatusDown`. AC-28 corrects the page to the code |
| `ai/doors/le-tooling.md` | Boundaries | The door covering `internal/le/arch/doctorcoverage/sites.go` and `test/plugin/host-trend-runs-without-telemetry.ci` | "A bare `go test` omits the feature tags" | agrees | The package tests run through `./le test unit pkg path`, and the three `host-trend-*` tests are `.ci` files under `test/plugin/` that the functional runner drives |
| `ai/doors/api-surfaces.md` | Who depends on it from outside | The door covering `internal/component/telemetry/collector/collector.go` | "A metric or a label is renamed" | agrees | No existing `netdata_*` or `ze_host_*` name or label changes. The trend metrics are new names (AC-22), and AC-26 corrects a value, not a name |
| `docs/architecture/storage/smart-health.md` | The decisions | The `// Design:` page of `internal/component/storage/manager.go`, anchored to it | "Health errors dedup through a `healthReported` flag." | agrees | AC-27 keeps the flag and moves the poll's writes of it and of `prevTemp` under the manager's lock. The trend show handler reads a pointer the hub sets, as this page's show RPC does |
| `docs/architecture/traffic/traffic-analysis-layers.md` | The shared confirm-and-clear machine | The `// Design:` page of `internal/core/stats/window.go`, `level.go` and `hysteresis.go`, anchored to them and to `internal/plugins/ddos/detect/detector.go`; a page this spec modifies | "The machine counts answers and nothing else." | agrees | The kernel-memory rule feeds its growth answer to the same `stats.Hysteresis` ddos-detect drives, so the tree holds one confirm-and-clear machine. The level history joins the page's primitives under the same contract: pure, the caller passes the time |
| `docs/architecture/ddos/cp-survival-5-detect-5-characterization.md` | Characterization runs off the rate tick | Anchored to `internal/plugins/ddos/detect/detector.go` | - | silent | The anchor is `onAttackStart` and the characterization it starts. The conversion hands the same `onAttackStart` to `stats.Hysteresis` as its `OnConfirmed` callback, called once per confirmation as before, so characterization is untouched |
| `docs/architecture/ddos/ddos-detect-enhancements.md` | The bandwidth trigger is a second baseline instance | Anchored to `internal/plugins/ddos/detect/detector.go` | - | silent | The anchor is the bandwidth baseline and the trigger it feeds. The conversion changes only the confirm-and-clear machine the trigger's answer reaches, and no baseline |
| `docs/architecture/api/architecture.md` | Transport shutdown | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchors are the transport shutdown and the boot user resolution of `runYANGConfig`. The trend monitor serves no transport and resolves no user |
| `docs/architecture/api/commands.md` | Process Lifecycle Commands | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchors are the process lifecycle and plugin completion wiring of `runYANGConfig`. `show host trend` is a daemon command declared in host-cmd's YANG, added without touching either |
| `docs/architecture/api/commands.md` | Design decisions | The decision of 2026-08-21 on the page anchored to `cmd/ze/hub/main.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `show host trend` names the subject and takes no argument. Selecting one rule and choosing a format are pipe operations after it, and `host-trend-show.ci` reads the answer through the generic json and yaml pipes |
| `docs/architecture/api/commands.md` | Design decisions | The decision of 2026-11-20 on the page anchored to `cmd/ze/hub/main.go` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | `show host trend` declares no pipe operation of its own, so every generic operation answers it in the generic's shape |
| `docs/architecture/api/process-protocol.md` | The stop drain | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchors are the stop drain and the stop of the plugin server. The trend monitor is no plugin and declares no drain budget: it stops through a deferred call beside `stopSmartManager` in the teardown of `runYANGConfig` |
| `docs/architecture/appliance/remote-operations.md` | Live configuration upload | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchor is the mutation owner and shutdown of a live configuration upload. The monitor has no config, so an upload neither reaches nor restarts it |
| `docs/architecture/behavior/signals.md` | Stop sequence | Anchored to `cmd/ze/hub/main.go` | - | silent | The section's phases are the config fence, the stop drain and teardown. The monitor stops in teardown, among the deferred calls of `runYANGConfig`, and adds no phase and no signal handling |
| `docs/architecture/cli/command-completion.md` | Why web sources at request time | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchor is the wiring of the web completion source. `show host trend` completes from its YANG node, as every command does |
| `docs/architecture/config/syntax.md` | Downgrade Recovery | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchor is the branch taken when the startup config fails to load. The monitor starts after the config loaded and reads none of it |
| `docs/architecture/config/yang-config-design.md` | Joining a module that no `init()` registered | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchor is how an external plugin's module joins the schema. The trend command is a node of the compiled-in host-cmd module |
| `docs/architecture/diagnostics/crash-capture.md` | The decisions | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchor is `exitForced`. The monitor neither captures a crash nor changes how a forced exit is recorded |
| `docs/architecture/pki/pki-store.md` | Decisions | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchor is the opening of the PKI store in `run` and `runYANGConfig`. The monitor reads no certificate |
| `docs/architecture/pki/tls-listeners.md` | Decision: a named certificate fails closed | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchor is the certificate gates of the web and looking-glass listeners. The monitor opens no listener |
| `docs/architecture/system-architecture.md` | Capabilities without root | Anchored to `cmd/ze/hub/main.go` | - | silent | The section declares the capabilities a non-root daemon holds for networking and eBPF. The kernel-memory rule reads `/proc/meminfo`, which needs no capability. The owed cache names read `/proc/slabinfo`, which needs root; A-4 and AC-13 cover a daemon that cannot read it, so the design asks for no new capability |
| `docs/architecture/testing/ci-format.md` | The compiled observer API | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchor is the daemon shutdown function the observer API calls, which this design does not touch |
| `docs/architecture/web-interface.md` | Per-User Editor | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchor is the editor's late-bound reload holder. The monitor has no config to edit, and the web reaches `show host trend` through the shared dispatcher |
| `docs/architecture/command-ownership.md` | `plugins/` -- Self-Contained Feature Owners | Anchored to `internal/plugins/host-cmd/cmd/register.go` | "It delegates the operation to a `component/` or `core/` package." | agrees | host-cmd owns the `show host trend` YANG node and its handler, and the handler reads the state the monitor in `internal/component/host/trend` publishes. The monitor, its rules and their doctor checks stay in the component |
| `docs/architecture/overview.md` | 3. Directory Structure | Anchored to `internal/plugins/host-cmd/cmd/register.go` | - | silent | The anchor cites host-cmd's registration as an example of a command-only plugin. The trend handler adds a file to that plugin and changes no layout |
| `docs/architecture/testing/test-health.md` | One approval names one test | Anchored to `internal/le/hookruntime/bash.go` | - | silent | The anchor is `governedShellWrite`. The worktree fix changes only where the Bash hook finds a session's read records, not which shell write it governs or how an approval names its test |
| `docs/architecture/core-design.md` | Command engine and composition roots | Added: the `// Design:` page of the tooling files of AC-30 and AC-31, anchored to `internal/le/hookruntime/bash.go` | - | silent | The anchors are the hook's materialization of derived files. The two tooling fixes change where a read record is found and how a file bullet is read, and leave le's command engine and its composition roots as they are |
| `docs/architecture/core-design.md` | 19. Component Boundaries | Anchored to `cmd/ze/hub/main.go` | - | silent | The anchor is the boot publication of `runYANGConfig`. The trend monitor is a subpackage of the host component, so its import of the host package crosses no component boundary, and host-cmd reaches it as a command-only plugin reaches the component it delegates to |
| `docs/contributing/spec-workflow.md` | The two sections a design owes | The `// Design:` page of `internal/le/spec/design/specfile.go`; a page this spec modifies | "A file bullet names every back-quoted path of its path list, the text before its description, so a bullet listing two files separated by a comma owes the pages of both." | agrees | AC-31 makes the derivation read every path of a bullet, and this table is derived that way: the later paths of each Files bullet and their bare sibling names owe their pages. The page states the rule (commit baf7b18b8c) |
| `docs/contributing/spec-workflow.md` | The judge | Added: the `// Design:` page of `internal/le/spec/design/judge.go`; a page this spec modifies | "A read of the holding checkout's own copy carries another key, and covers no row of the worktree's spec." | agrees | AC-30 lets a judge in a worktree count the records its session keeps in the holding checkout under the worktree's key, and keeps this refusal: a read of the holder's own copy still covers no row (`TestJudgeInAWorktreeRefusesAReadOfTheHoldersCopy`). The page states it (commit a2ee440e05) |
| `docs/plugin-development/metrics.md` | Label Rules | A page this spec modifies | "Use labels for runtime dimensions. Never encode variable data in metric names." | agrees | The trend gauges carry the rule as a `rule` label, so a fifth rule adds a label value and no metric name. The page gains the four names (AC-22) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The kernel's own counters under `/proc`: `meminfo`, `slabinfo`, `sys/fs/file-nr`, `sys/net/netfilter/nf_conntrack_count` and `nf_conntrack_max`, and `statfs` on the persistent partition
- Format at entry: text files and one system call, read by the host component's readers on a timer

### Transformation Path
1. Sample: on each rule's sample period the monitor calls the rule's reader, which answers a level and a limit, or "not present" (the resource does not exist on this box), or an error
2. Bucket: the level is folded into the rule's current bucket of the history (minimum and last value of the period); when the period ends the bucket closes
3. Judge: at each sample the fill pattern is evaluated, and at each bucket close the growth pattern is evaluated; each answers true or false
4. Hysteresis: the answer feeds the rule's confirm-and-clear state machine
5. Raise or clear: on confirm the monitor raises a warning on the report bus with the message and the fix, and refreshes it at each bucket close while active; on clear it removes it
6. Detail: while the kernel-memory rule is confirming or active, the monitor reads the per-cache slab sizes at each bucket close and names the caches that grew most
7. Persist: the history is saved through statestore at most every five minutes and at stop, with the boot identifier
8. Read back: the rule's doctor check, the health probe, `show host trend` and the metrics read the monitor's state

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel to host component | File reads under the detector root, `statfs` | Yes for `/proc/meminfo` (`TestDetectKernelMemoryReadsUnreclaimableSlab` over a fixture root); the other readers owed |
| Host component to report bus | `report.RaiseWarningFix` and `report.ClearWarning`, same process | Yes (`TestTrendMonitorRaisesAndClearsThroughRunLoop`, `TestTrendMonitorStopClearsOnlyItsOwnWarnings`) |
| Host component to statestore | Acknowledged write and read under one registered key | No: owed with AC-10 |
| Host component to `host-cmd` | The command handler reads the monitor's published state. This is a same-process read (assumption A-3, confirmed) | Yes (`host-trend-show.ci` through the daemon) |
| Daemon to operator | `show host trend`, `show warnings`, `show doctor`, `show health`, the metrics endpoint | No |

### Changed-Wire Census

**Search evidence:** No existing wire changes its format: the spec adds four formats and alters none. Scope searched: `internal/`, `cmd/`, `pkg/`, `test/`, `docs/`, non-Go files included, for `ze_host_`, `show host`, `meta/` state keys in `pkg/zefs/keys.go`, and the report-bus codes raised by source `host`. Positive control: the search finds the existing `ze_host_memory_available_bytes` gauge at `internal/component/host/metrics.go` and the `show-memory` command at `internal/plugins/host-cmd/yang/ze-host-cmd.yang`, so a reader of an existing host format would have been found. No reader of a `show host trend` answer, a `ze_host_trend_` metric, a host trend state key or a `doctor-host-*-growth` code exists yet, because none of them exists.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Trend history snapshot | writer | `internal/component/host`, the trend monitor's save | New key registered in `pkg/zefs/keys.go`; no writer exists today | New, versioned; nothing to migrate |
| Trend history snapshot | reader | `internal/component/host`, the trend monitor's load at start | No reader exists today; `ze data registered` lists the key to an operator | New; a snapshot of an unknown version or another boot is discarded, never guessed at |
| `show host trend` answer | writer | `internal/plugins/host-cmd/cmd`, the new handler | `ze-host-cmd.yang` has no `trend` node today | New |
| `show host trend` answer | reader | The shared renderers, and API, MCP and web clients outside the repository | Command answers reach every interface through the command registry (`ai/doors/system.md`) | New; declared answer shape documented in the command reference |
| `ze_host_trend_*` metrics | writer | `internal/component/host/trend/metrics.go` (owed): the trend package imports the host package, so the host package's `metrics.go` cannot read the monitor | No such name in the tree | New names only; existing names untouched |
| `ze_host_trend_*` metrics | reader | Prometheus and dashboards outside the repository | `ai/doors/api-surfaces.md`, "Who depends on it from outside" | New; documented in the monitoring guide |
| Trend warning | writer | `internal/component/host`, the trend monitor's raise | No `doctor-host-*` trend code in `internal/core/diagnostic` | New codes registered by the rules |
| Trend warning | reader | `show warnings` handler, `checkRecentEvents`, the rule's runtime check, `HealthProbeDegraded` | `internal/component/cmd/show/show.go`, `internal/component/doctor/checks_recent_events.go` | Existing readers handle any code; the rule's check owns its row |

### Integration Points
- `cmd/ze/hub/main_system.go` system start and stop - the monitor starts and stops beside the SMART manager
- `internal/core/report` - the monitor raises and clears warnings
- `internal/core/diagnostic` `Register`, `RegisterEventCode`, `RegisterDoctorCheck` - each rule declares its code and its runtime check
- `internal/core/health` `Register` with `report.HealthProbeDegraded` - one health row
- `internal/core/statestore` and `pkg/zefs/keys.go` - the history snapshot
- `internal/core/stats` - the level history, the two patterns and the shared state machine
- `internal/plugins/host-cmd` - the `show host trend` command
- `internal/core/metrics` registry - the trend gauges, registered when a registry is published

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Filled at implementation |
| No unintended coupling (components stay isolated) | No | Filled at implementation |
| No duplicated functionality (extends existing, does not recreate) | No | Filled at implementation |
| Zero-copy preserved where applicable (refs, not copies) | No | Filled at implementation |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | Filled at implementation |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | Filled at implementation |

## Design

### The two patterns

A rule watches one **level** (how much of a resource is used) against one **limit** (the level at which the resource is exhausted). Both are read together at each sample.

| Pattern | Holds when | Why this shape |
|---------|-----------|----------------|
| Growth | The minimum of each of the last N closed buckets is higher than the minimum of the bucket before it, the level rose by at least a stated share of the limit over those buckets, and a straight line fitted to those minimums reaches the limit within the rule's horizon | The minimum of a period is the floor. A leak raises the floor; a burst of traffic raises the peak and leaves the floor alone. Requiring a projected exhaustion keeps a level that grows and then settles (a routing table loading) from alarming |
| Fill | The level is at or above a stated share of the limit | A hard kernel limit that is nearly reached is a problem whatever the trend |

A rule declares one pattern or both. A missing bucket (the daemon was down for a whole period) ends a growth run: growth is judged only over consecutive buckets that were observed.

### The built-in rules

| Rule | Level | Limit | Sample | Bucket | Buckets kept | Growth pattern | Fill pattern |
|------|-------|-------|--------|--------|--------------|----------------|--------------|
| `kernel-memory` | Kernel memory that cannot be reclaimed (`SUnreclaim`) | That level plus the memory still available (`MemAvailable`) | 60 s | 1 h | 168 (7 days) | 6 consecutive hours, rise of at least 1% of total memory, exhaustion within 7 days | none |
| `file-descriptors` | Open file handles system-wide (`file-nr`, allocated) | The system maximum (`file-nr`, maximum) | 30 s | 5 min | 288 (24 h) | 6 consecutive buckets, rise of at least 5% of the limit, exhaustion within 24 h | 90% |
| `conntrack` | Tracked connections (`nf_conntrack_count`) | `nf_conntrack_max` | 10 s | 1 min | 120 (2 h) | 5 consecutive buckets, rise of at least 5% of the limit, exhaustion within 1 h | 90% |
| `disk` | Bytes used on the persistent partition | Its size | 60 s | 1 h | 168 (7 days) | 6 consecutive hours, rise of at least 1% of the size, full within 3 days | none: `checkDiskSpace` owns the level |

Every pattern confirms after 3 consecutive true evaluations and clears after 3 consecutive false ones. These figures are the built-in defaults Thomas ruled for; they are constants of each rule's own file, with no config leaf.

### What a rule is

A rule is a value its own file registers from `init()` into the host component's rule registry. It carries: its name, its doctor code, its sample period, bucket period and bucket count, its reader, its pattern figures, and the function that writes its message and fix from the observed figures. The same file registers the rule's code, its event code and its runtime doctor check. Adding a fifth rule is adding a file: the monitor, `show host trend`, the health probe and the metrics iterate the registry and name no rule.

### What the operator sees

| Surface | Content |
|---------|---------|
| `show warnings` | One warning per active rule, source `host`, the rule's code, subject the rule name, with the fix |
| `show doctor` | One row per rule from the rule's own runtime check: nothing when the rule is quiet, a warning with the same message when it is active, and a warning when the rule's reader fails. One more check reports the monitor not running |
| `show health` | Row `host-trend`, degraded while any trend warning is active |
| `show host trend` | Every rule: its state (not present, warming up, quiet, confirming, active, unreadable), level, limit, how long it has been observed, the growth over the window, the projected time to exhaustion when one exists, and for `kernel-memory` the caches that grew most |
| Metrics | Per rule: active or not, level, limit, and the projected seconds to exhaustion, which reads positive infinity while no exhaustion is projected |

Message of the `kernel-memory` rule, as an example of the wording every rule follows: "Kernel memory that cannot be reclaimed grew by 412 MiB over the last 9 hours and is still growing. At this rate the box runs out of memory in about 2 days. The kernel caches that grew most: kmalloc-256 (+380 MiB). This is the pattern of a kernel memory leak: a kernel update fixes it, and a reboot frees the memory until it grows again." Fix: `show host trend`.

When `/proc/slabinfo` cannot be read (it needs root), the message says the caches could not be named and why, and still reports the growth.

### Lifetime

The monitor is one long-lived goroutine driven by a `clock.Clock` ticker. The hub starts it at system start beside the SMART manager and stops it at daemon stop, where it saves its history and clears its own warnings. It runs whether or not telemetry or BGP is configured. A config reload does not touch it. At start it loads the saved history when the snapshot's boot identifier matches the running kernel's and every value is valid; otherwise it starts empty and says so in `show host trend` (warming up).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The host component is always linked and the hub may start it directly | `feature-gates.txt` has no row for `internal/component/host` | The monitor needs a registration seam like a gated service | `./le arch tier check` after the wiring phase | confirmed: `feature-gates.txt` has no row for `internal/component/host` or its `trend` subpackage, and `cmd/ze/hub/main_system.go` `startTrendMonitor` calls `trend.Start` directly (commit d2376b7e29); `./le arch tier check` refuses only files outside this work, as on the base commit 0178ed7e22 |
| A-2 | A leak raises the hourly minimum of unreclaimable slab for hours on end, and legitimate growth (a full routing table, connection load) settles within six hours | A kernel leak grows without bound by nature; reasoning about route and connection load, not a measurement | The kernel-memory rule alarms on a healthy converging router, or misses a leak slower than 1% of memory in six hours | A lab run: load a full table and hold it for eight hours with the rule quiet; then a kernel module that leaks, with the rule confirming | unvalidated |
| A-3 | `host-cmd` handlers run in the daemon process, so the handler can read the monitor's state by a call | `show host storage` already reaches the SMART manager through `SetStorageManager` in `internal/component/storage/show.go` | `show host trend` answers from an empty copy when the plugin is forked; the state must then travel by a typed request to the daemon | Reading how `host-cmd` is launched, and `./le plugin boundary check` | confirmed: host-cmd has no runner of its own, and `pluginserver.RegisterRPCs` (called from `internal/plugins/host-cmd/cmd/show_trend.go`, as from the other host-cmd handlers) loads its handlers into the daemon's dispatcher, so the handler reads the monitor in the daemon process; `./le plugin boundary check` clean (commit cfe98a9853) |
| A-4 | The daemon can read `/proc/slabinfo` on the appliance | The appliance runs `ze` as root; the file is mode 0400 root | The cache names are never available; the growth warning still fires | The integration test on Ze's kernel reading the real file | unvalidated |
| A-5 | Each fix command named in a warning exists: `show host trend` (added here), and for the fill pattern the Ze command that raises the limit | `set system file-descriptors` exists but sets the daemon's own limit, not the system maximum; a conntrack limit is managed through `internal/core/sysctl` (`net.netfilter.nf_conntrack_max`) | A warning names a command that does not do what it says | Reading each command's handler before writing the message; the functional test runs the named command | unvalidated |
| A-6 | The persistent partition judged by the disk rule is the one `checkDiskSpace` judges | `internal/component/doctor/checks_storage.go` `checkDiskSpace` | The two disagree about which disk is filling | Reading `checkDiskSpace` and deriving the path from the same source | unvalidated |
| A-7 | Converting anomaly-detect's inline counters to the shared state machine changes no decision | `stepEntity` and `stateMachine` implement the same confirm and clear counts by reading | A traffic detector changes behavior | The existing ddos-detect and anomaly-detect tests, unchanged, stay green | unvalidated |
| A-8 | The `host-monitor` runtime check of `spec-doctor-coverage-15-system` AC-13 and this spec's monitor check do not collide | That spec's AC-13 text | Two checks report one state | Reading that spec's landed state at implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | False alarm on a healthy box: the rule figures are reasoned, not measured | A trend warning on a lab box that is only loading routes | A-2's lab run before closure; the figures are constants in one file per rule, so a correction is one line and one test |
| R-2 | A leak too slow for the window is never reported | A box that dies of memory with the rule quiet | The 7-day history and the 7-day horizon bound what is caught; stated in Known Limitations and in the guide |
| R-3 | The wall clock steps (NTP correction) and a bucket closes early or twice | A history with a bucket shorter than its period | A sample dated before the current bucket's start is folded into the current bucket; a step forward closes the skipped buckets as missing, which ends a growth run instead of inventing one |
| R-4 | A saved history from before a reboot is trusted and reports growth across the reboot | A warning minutes after boot | The snapshot carries the boot identifier and is discarded on mismatch; AC-10 |
| R-5 | The reader fails and the rule reads as quiet | A rule that never leaves "quiet" on a box where the file is unreadable | A reader error is a state of its own, reported by the rule's doctor check; "not present" is a third, separate answer; AC-8 |
| R-6 | Making the doctor-coverage gate judge the `*Fix` raise variants reveals many unowned sites across the tree | The gate goes red on packages this spec does not touch | Sites in the host component are fixed here; if the others exceed a small fix, that conversion becomes a child spec named here, and the gate change lands with it |
| R-7 | Moving the state machine out of ddos-detect breaks a build without `ze_ddos` or a ddos test | A compile failure in a flavor | The type moves to always-on core; the scoped ddos and anomaly package tests and `./le repo compiles check` |
| R-8 | The monitor's own cost: reading `/proc/slabinfo` on a box with thousands of caches every hour | Visible CPU at bucket close | Read only while the kernel-memory rule is confirming or active, once per bucket |
| R-9 | The monitor and a command handler read the same state concurrently | A race detector report | One owner goroutine; readers take a snapshot under one lock; unit test under `-race` |
| R-10 | The warning cap of the report bus evicts a trend warning | An active rule with no warning | Four keys at most, refreshed at each bucket close, and eviction drops the oldest refresh first |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A false warning in `show warnings` and `show doctor` (the box still reads ready), or a missed leak, which is today's behavior. The hysteresis move could change when a DDoS or anomaly detection confirms or clears |
| How is it reverted? | The monitor is a new start call and new files: reverting its commits removes it. The hysteresis move is its own commit and reverts alone |
| Who else touches this path? | `spec-doctor-coverage-15-system` (host monitor start, host codes, SMART), `spec-doctor-coverage-14-system-plugins` if it touches `host-cmd`, and any session editing `internal/plugins/ddos/detect` or `internal/plugins/anomaly/detect` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze start` with no `telemetry` and no `bgp` block | → | The hub's system start starts the trend monitor | `test/plugin/host-trend-runs-without-telemetry.ci` |
| `show host trend` on a running daemon | → | The `host-cmd` handler reading the monitor's state | `test/plugin/host-trend-show.ci` |
| `show doctor` on a running daemon | → | Each rule's runtime check | `test/plugin/host-trend-doctor.ci` |
| The monitor's own run loop with real readers over a fixture `/proc` and a fake clock | → | Sample, bucket, judge, confirm, raise, clear | `TestTrendMonitorRaisesAndClearsThroughRunLoop` |
| Daemon stop then start | → | Save and load of the history | `TestTrendMonitorRestoresHistoryAcrossRestart` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Samples of a level fed to the level history with their times | Each period's bucket holds the minimum and the last sample of that period, the history keeps the newest N buckets, and a period with no sample is recorded as missing | A falling level is kept as it is: unlike a byte counter, a level legitimately goes down |
| AC-2 | A history whose last N bucket minimums each exceed the one before, with a rise of at least the stated share and a fitted line reaching the limit within the horizon | The growth pattern answers true and reports the rise, the span and the projected time to exhaustion | A history that rises and then holds level answers false: growth that settles is healthy |
| AC-3 | The same rise interrupted by one missing bucket, or by one bucket whose minimum did not rise | The growth pattern answers false and the run restarts from the interruption | Growth observed after the interruption counts toward a new run |
| AC-4 | A level at or above the stated share of its limit; a limit of zero | The fill pattern answers true at or above the share and false below it; a zero limit answers "cannot judge" as an error, never a share | A level below the share stays quiet |
| AC-5 | The ddos-detect and anomaly-detect tests, unchanged | Both detectors use one exported confirm-and-clear state machine in `internal/core/stats`, their tests pass unchanged, and neither package keeps its own copy | The detectors keep their own thresholds and counts: only the counting logic is shared |
| AC-6 | A daemon started with no `telemetry` block and no `bgp` block | The trend monitor runs and `show host trend` lists every registered rule with a state and how long it has been observed | A rule whose resource is absent on the box (no conntrack module) is listed as not present |
| AC-7 | A rule registered from a new file, with no other edit | The monitor samples it, and `show host trend`, the health probe and the metrics include it | None: a list that must learn the rule's name is the defect |
| AC-8 | A reader that answers an error; a reader that answers "not present" | The error puts the rule in the unreadable state, and its doctor check reports a warning naming the path and the error; "not present" puts it in the not-present state and reports nothing | A rule that is not present stays silent in doctor: an absent resource is not a problem |
| AC-9 | A config reload while a rule holds three hours of history | The monitor keeps running and the history is still three hours long | None: a reload has nothing to reconfigure here |
| AC-10 | The daemon stops and starts on the same boot; the daemon starts after a reboot with a snapshot from the previous boot | Same boot: the history is restored and a growth run continues across the restart. Other boot, unknown version or an invalid value: the snapshot is discarded, the rules start warming up, and `show host trend` says so | Buckets older than the rule's window are dropped on load |
| AC-11 | Daemon stop while a trend warning is active | The history is saved, the monitor's goroutine ends, and its warnings are cleared from the bus | Other warnings of source `host` (carrier, throttle) are left as they are: they are not the monitor's |
| AC-12 | Unreclaimable kernel memory whose hourly minimum rises for 8 hours by more than 1% of memory, projecting exhaustion within 7 days | A warning with the rule's code states the growth, the span and the projected time, and clears after the growth stops for 3 evaluations | Unreclaimable memory that rises for 2 hours and then holds (a routing table loading) raises nothing |
| AC-13 | The kernel-memory rule confirming or active, with `/proc/slabinfo` readable; then unreadable | Readable: the warning names the caches that grew most, with the size each gained. Unreadable: the warning states that the caches could not be named and why, and still reports the growth | The slab file is not read while the rule is quiet |
| AC-14 | Open file handles at 92% of the system maximum; open file handles rising toward the maximum within 24 hours from 40% | Both raise the file-descriptor warning, the first from the fill pattern and the second from the growth pattern, each saying which | A level steady at 60% raises nothing |
| AC-15 | The conntrack table at 95% of its maximum; the conntrack module not loaded | The table at 95% raises the conntrack warning with the count and the maximum; without the module the rule is not present | A table that fills to 50% in a burst and drains raises nothing |
| AC-16 | The persistent partition's used bytes rising every hour toward full within 3 days | The disk warning states the growth and the projected time to full | A partition that is nearly full and not growing is reported by `checkDiskSpace` only: one finding per condition |
| AC-17 | A rule active | `show warnings` holds one warning with source `host`, the rule's code, the rule name as subject and a fix; a second evaluation refreshes it and keeps its raise time | One warning per rule, however long it stays active |
| AC-18 | A rule active; then quiet | `show doctor` holds one row from the rule's own check with the same message and fix while active, and no row from that check once quiet; the `recent-events` check adds no second row for the same warning | The row is a warning: the box still reads ready (decision D-3) |
| AC-19 | The monitor never started | The monitor check of `show doctor` reports that the trend monitor is not running, with the command that restarts it; `show host trend` answers the same, never an empty list | None |
| AC-20 | Any trend warning active | `show health` shows the `host-trend` row degraded, derived from the bus; with no trend warning it reads healthy | Degraded, not down: the health endpoint still answers 200 |
| AC-21 | `show host trend` | The answer lists, per rule: state, level, limit, observed duration, growth over the window, projected time to exhaustion when one exists, and for `kernel-memory` the growing caches; it reaches every command surface and takes the generic pipes | A rule still warming up shows its observed duration and no projection |
| AC-22 | A metrics registry published, exporter enabled | Per rule the endpoint exposes whether it is active, its level, its limit and the projected seconds to exhaustion, positive infinity while none is projected | With the exporter disabled no metric is exposed and the monitor still runs |
| AC-23 | Every Warn and Error line and every raise of the monitor | Each names a doctor code that a registered check declares, and `./le arch` doctor-coverage accepts the host component | None |
| AC-24 | An active warning reported by `recent-events` (any producer whose code no runtime check owns) | The row says the condition is still active; only an event that already happened keeps "It may have recovered" | Error events keep the existing wording: they are past by nature |
| AC-25 | A package raising through `RaiseWarningFix`, `RaiseErrorFix` or `RaiseErrorFixOnce` | The doctor-coverage gate judges those call sites the way it judges `RaiseWarning` and `RaiseError` | Sites outside this spec's packages that the gate newly refuses follow R-6 |
| AC-26 | `collector vmstat { interval 10; }` (any rate collector with its own interval) | The collector's per-second rates are computed over the interval it actually runs at, so the figure matches a one-second collector's | Collectors with no override behave as before |
| AC-27 | `show host storage` read while the SMART poll runs | The poll updates each device's previous temperature and health latch under the manager's lock, and the read returns a consistent copy, proven under the race detector | None |
| AC-28 | `docs/architecture/diagnostics/production-diagnostics.md` | The page states that the health handler answers 503 only when a check is down and 200 when healthy or degraded | None |
| AC-29 | `ai/digests/observation-telemetry.md`, `docs/architecture/host/observability.md`, `docs/guide/monitoring.md` | Each states what starts system sampling and under which config: the exporter's collectors only while the exporter is enabled, the host monitor and the trend monitor always | None |
| AC-30 | A design judge run from a git worktree under `.claude/worktrees/<name>`, after its session's Read hook recorded the reads in the holding checkout's `tmp/session` under the worktree's key | `./le spec design judge` counts those records and records the verdict (`TestJudgeInAWorktreeReadsTheRecordsItsSessionKeeps`, red before the fix with "has not read 2 section(s)"); a read of the holding checkout's own copy of a page still covers no row (`TestJudgeInAWorktreeRefusesAReadOfTheHoldersCopy`) | In the main checkout nothing changes: one store, the bare key, the same marker names |
| AC-31 | A Files to Modify or Files to Create bullet naming several back-quoted paths, a bare sibling name, a span in parentheses, and a description after ` - ` | The derivation takes every path before the description and each bare sibling name in the directory of the path before it, and skips a span in parentheses and every span of the description (`TestBulletPathsStopsAtTheDescription`, `TestSpecSourceFilesTakesEveryPathOfABullet`); `./le spec design documents` owes the design pages of every one of those files (`TestDesignDocumentsTakeEveryPageOfABullet`) | A file written inside a parenthesis is skipped, which leaves a spec owing fewer pages, as before |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `show doctor` on a box whose kernel leaks memory | `/proc/meminfo` -> trend monitor -> growth pattern -> state machine -> report bus -> the rule's runtime check | `TestTrendMonitorRaisesAndClearsThroughRunLoop`, `test/plugin/host-trend-doctor.ci` |
| 2 | Runs `show host trend` to see which kernel cache grows | `/proc/slabinfo` -> trend monitor detail -> `host-cmd` handler | `TestKernelMemoryRuleNamesGrowingCaches`, `test/plugin/host-trend-show.ci` |
| 3 | Restarts the daemon on a leaking box | history -> statestore -> load at start -> growth run continues | `TestTrendMonitorRestoresHistoryAcrossRestart` |
| 4 | Scrapes the metrics endpoint with an external Prometheus | trend monitor state -> `ze_host_trend_*` gauges -> exporter | `TestTrendMetricsFollowRuleState` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLevelHistoryBucketsMinimumAndLast` | `internal/core/stats/level_test.go` | AC-1: bucket contents | built, commit 6cf4b12c26 |
| `TestLevelHistoryRecordsMissingBucket` | `internal/core/stats/level_test.go` | AC-1: a period with no sample | built, commit 6cf4b12c26 |
| `TestGrowthHoldsOnSustainedRise` | `internal/core/stats/level_test.go` | AC-2: true with rise, span and projection | built, commit 6cf4b12c26 |
| `TestGrowthFalseWhenRiseSettles` | `internal/core/stats/level_test.go` | AC-2: the survivor | built, commit 6cf4b12c26 |
| `TestGrowthRunEndsAtMissingBucket` | `internal/core/stats/level_test.go` | AC-3 | built, commit 6cf4b12c26 |
| `TestGrowthRunRestartsAfterFlatBucket` | `internal/core/stats/level_test.go` | AC-3: a bucket whose minimum did not rise | built, commit 6cf4b12c26 |
| `TestLevelParamsGrowthRisesBounds` | `internal/core/stats/level_test.go` | The growth-run boundary row | built, commit 6cf4b12c26 |
| `TestFillAgainstLimit` | `internal/core/stats/level_test.go` | AC-4, zero limit included | owed |
| `TestHysteresisConfirmAndClear` | `internal/core/stats/hysteresis_test.go` | AC-5: the moved machine's contract | built, commit 2829eaf0da |
| `TestRuleRegisteredIsSampledAndListed` | `internal/component/host/trend/trend_test.go` | AC-7 | built, commit d2376b7e29 |
| `TestReaderErrorAndNotPresentAreDistinctStates` | `internal/component/host/trend/trend_test.go` | AC-8 | built, commit d2376b7e29 |
| `TestTrendMonitorRaisesAndClearsThroughRunLoop` | `internal/component/host/trend/trend_memory_linux_test.go` | AC-12, AC-17: the run loop over a fixture `/proc` and a fake clock | built, commit d2376b7e29, red under a recorded break |
| `TestTrendMonitorRestoresHistoryAcrossRestart` | `internal/component/host/trend/persist_test.go` | AC-10: same boot | owed |
| `TestTrendMonitorDiscardsSnapshotFromAnotherBoot` | `internal/component/host/trend/persist_test.go` | AC-10: other boot, bad version, invalid value | owed |
| `TestTrendMonitorStopClearsOnlyItsOwnWarnings` | `internal/component/host/trend/trend_test.go` | AC-11: the warnings | built, commit d2376b7e29; the save at stop owed |
| `TestKernelMemoryRuleQuietOnGrowthThatSettles` | `internal/component/host/trend/trend_memory_linux_test.go` | AC-12: the survivor | built, commit d2376b7e29, red under a recorded break |
| `TestDetectKernelMemoryReadsUnreclaimableSlab` | `internal/component/host/memory_kernel_linux_test.go` | AC-12: the reader; a missing line and an unreadable file are errors naming the path | built, commit d2376b7e29 |
| `TestKernelMemoryRuleNamesGrowingCaches` | `internal/component/host/trend/trend_memory_linux_test.go` | AC-13: readable | owed |
| `TestKernelMemoryRuleReportsUnreadableSlab` | `internal/component/host/trend/trend_memory_linux_test.go` | AC-13: unreadable | owed |
| `TestFileDescriptorRuleFillAndGrowth` | `internal/component/host/trend/trend_fd_linux_test.go` | AC-14 | owed |
| `TestConntrackRuleFillAndAbsentModule` | `internal/component/host/trend/trend_conntrack_linux_test.go` | AC-15 | owed |
| `TestDiskRuleGrowthOnly` | `internal/component/host/trend/trend_disk_linux_test.go` | AC-16 | owed |
| `TestTrendDoctorCheckRowFollowsRuleState` | `internal/component/host/trend/trend_doctor_linux_test.go` | AC-18 | built, commit d2376b7e29 |
| `TestTrendMonitorNotRunningIsReported` | `internal/component/host/trend/trend_doctor_test.go` | AC-19 | built, commit d2376b7e29 |
| `TestShowHostTrendListsWarmingUpRule`, `TestShowHostTrendListsProjectionOfActiveRule` | `internal/plugins/host-cmd/cmd/show_trend_linux_test.go` | AC-21, for `kernel-memory` | built, commit cfe98a9853 |
| `TestShowHostTrendRefusesWhenMonitorNotRunning` | `internal/plugins/host-cmd/cmd/show_trend_test.go` | AC-19: the command answers an operator error, never an empty list | built, commit cfe98a9853 |
| `TestTrendHealthRowDerivesFromBus` | `internal/component/host/trend/trend_doctor_test.go` | AC-20 | owed |
| `TestTrendMetricsFollowRuleState` | `internal/component/host/trend/metrics_test.go` | AC-22 | owed |
| `TestJudgeInAWorktreeReadsTheRecordsItsSessionKeeps`, `TestJudgeInAWorktreeRefusesAReadOfTheHoldersCopy` | `internal/le/spec/design/judge_record_test.go` | AC-30 | built, commit a2ee440e05 |
| `TestBulletPathsStopsAtTheDescription`, `TestSpecSourceFilesTakesEveryPathOfABullet` | `internal/le/spec/citation/bullets_test.go` | AC-31: the bullet reader | built, commit baf7b18b8c |
| `TestDesignDocumentsTakeEveryPageOfABullet` | `internal/le/spec/design/documents_test.go` | AC-31: the derivation | built, commit baf7b18b8c |
| `TestRecentEventsActiveWarningWording` | `internal/component/doctor/checks_recent_events_test.go` | AC-24 | |
| `TestReportSiteJudgesFixVariants` | `internal/le/arch/doctorcoverage/sites_test.go` | AC-25 | |
| `TestRateCollectorUsesItsOwnInterval` | `internal/component/telemetry/collector/collector_test.go` | AC-26 | |
| `TestStatusDuringPollIsRaceFree` | `internal/component/storage/manager_test.go` | AC-27, under the race detector | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Consecutive rising buckets for growth | 1 to the bucket count minus 1, because N rises compare N+1 buckets (`TestLevelParamsGrowthRisesBounds`) | the bucket count minus 1 | 0 | the bucket count |
| Fill share | above 0 to 100% | 100% | 0 | above 100% |
| Limit | 1 and above | any positive value | 0 (answers "cannot judge") | N/A |
| Confirm and clear counts of the trend rules | 1 and above; each rule's counts are constants of 3 in its own file | any positive value | 0 | N/A |
| ddos-detect `confirm-duration` through the shared `stats.Hysteresis`: the trend rules' row above does not bind it | 0 to 3600 (`ze-ddos-detect-conf.yang`, `uint16` with `range "0..3600"`): 0 or 1 confirms on the first evaluation, as before the move | 3600 | N/A: the type is unsigned | above 3600, refused by the YANG range |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `host-trend-runs-without-telemetry` | `test/plugin/host-trend-runs-without-telemetry.ci` | An operator with no telemetry and no BGP configured runs `show host trend` and sees every rule observed for a growing duration | built, commit cfe98a9853: red with `startTrendMonitor` removed, green restored |
| `host-trend-show` | `test/plugin/host-trend-show.ci` | `show host trend` answers the declared shape for each rule from the real `/proc` | built, commit cfe98a9853, for `kernel-memory` |
| `host-trend-doctor` | `test/plugin/host-trend-doctor.ci` | `show doctor` runs each rule's check and the monitor check on a healthy box and reports none of them as a problem | built, commit cfe98a9853 |

### Interop Tests (Scope: protocol)
N-A: Scope is plugin. No wire protocol is spoken to a peer.

## Files to Modify
- `internal/core/stats/window.go` - the package comment names the host trend monitor among the package's users (built)
- `internal/plugins/ddos/detect/detector.go` - drives `stats.Hysteresis`; the local machine and its tests are removed, with no alias left (built, commit 2829eaf0da)
- `internal/plugins/anomaly/detect/detector.go` - owed: use the shared state machine in place of the inline counters
- `internal/component/host/memory_linux.go`, `memory_other.go`, `inventory.go` - `Detector.DetectKernelMemory` and the `KernelMemory` it answers, read by the one `/proc/meminfo` parser `scanMeminfo` that `show host memory` also uses; off Linux it answers an error (built)
- `cmd/ze/hub/main_system.go`, `main.go` - `startTrendMonitor` and `stopTrendMonitor`, called beside the SMART manager at system start and stop (built)
- `pkg/zefs/keys.go` - owed: the history snapshot key
- `internal/plugins/host-cmd/yang/ze-host-cmd.yang`, `internal/plugins/host-cmd/cmd/register.go` - the `show host trend` command node and its handler (built)
- `internal/plugins/host/register.go` - the offline `show host trend` fallback, which says the command needs the daemon (built)
- `internal/component/plugin/all/testdata/wire-methods.snapshot` - gains `ze-host:show-trend` (built)
- `internal/component/doctor/checks_recent_events.go` - owed: wording for an active warning
- `internal/le/arch/doctorcoverage/sites.go` - owed: judge the `*Fix` raise variants
- `internal/component/telemetry/collector/collector.go` - owed: the effective interval reaches each rate collector
- `internal/component/storage/manager.go` - owed: the poll's writes under the manager's lock
- `internal/le/doc/readrecord/readrecord.go`, `internal/le/le/path/lepath.go`, `internal/le/hookruntime/readgate.go`, `bash.go`, `worktreecopy.go`, `worktreeplace.go`, `internal/le/spec/design/judge.go` - one resolver of a session's read records, so a design judge in a git worktree finds them (AC-30, built, commit a2ee440e05)
- `internal/le/spec/citation/anchors.go`, `internal/le/spec/design/specfile.go`, `internal/le/ai/hooks/fixtures.go` - every path of a file bullet owes its design pages, and the hook seal re-taken over both tooling fixes (AC-31, built, commit baf7b18b8c)
- `docs/architecture/host/observability.md`, `docs/architecture/host/inventory.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/diagnostics/production-diagnostics.md`, `docs/architecture/traffic/traffic-analysis-layers.md`, `docs/contributing/spec-workflow.md`, `docs/guide/monitoring.md`, `docs/guide/command-reference.md`, `docs/plugin-development/metrics.md`, `ai/digests/observation-telemetry.md`, `ai/digests/flow-ddos.md`, `ai/INDEX.md`, `features/host-inventory.md`

## Files to Create
- `internal/core/stats/level.go`, `hysteresis.go`, `level_test.go`, `hysteresis_test.go` - the level history and its growth pattern, the shared confirm-and-clear machine (built); the fill pattern joins `level.go` (owed)
- `internal/component/host/trend/trend.go`, `monitor.go`, `format.go`, `register.go` - the rule registry and the published state, the run loop on `clock.Clock`, the message wording, and the monitor's own codes and doctor checks (built). The monitor is a subpackage because `internal/core/diagnostic` imports `internal/component/host`, so the host package cannot register doctor checks without a cycle
- `internal/component/host/trend/register_kernel_memory.go` - the kernel-memory rule, which registers itself, its code and its runtime check (built; the cache names of AC-13 owed)
- `internal/component/host/trend/register_file_descriptors.go`, `register_conntrack.go`, `register_disk.go` - owed: one file per later rule, in the same form, each with its own reader
- `internal/component/host/trend/persist.go`, `metrics.go` - owed: the history snapshot, and the `ze_host_trend_*` gauges, which live in the trend package because the host package cannot import it
- `internal/component/host/trend/trend_test.go`, `trend_rule_test.go`, `trend_doctor_test.go`, `trend_doctor_linux_test.go`, `trend_memory_linux_test.go`, `internal/component/host/memory_kernel_linux_test.go` - the monitor, rule, doctor and reader tests (built)
- `internal/plugins/host-cmd/cmd/show_trend.go`, `show_trend_test.go`, `show_trend_linux_test.go` - the `show host trend` handler and its tests (built)
- `internal/plugins/host/trend_offline.go`, `trend_offline_test.go` - the offline fallback and its tests (built)
- `test/plugin/host-trend-runs-without-telemetry.ci`, `host-trend-show.ci`, `host-trend-doctor.ci` - the wiring tests (built)
- `internal/le/doc/readrecord/stores_test.go`, `internal/le/le/path/worktree_test.go`, `internal/le/spec/citation/bullets_test.go` - the tooling fixes' tests (built)
- `features/host-resource-trends.md` - the feature page (built)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/host-cmd/yang/ze-host-cmd.yang`: the `trend` command node under `show host`. No config schema: the rules are built in |
| YANG validation constraints | N-A | The command takes no argument |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | Yes | `show host trend`, through `host-cmd` |
| CLI grammar (keyword before value) | Yes | The path names the subject and stops; selecting one rule is a pipe operation |
| Editor autocomplete | Yes | Automatic from the YANG command node |
| Functional test for new RPC/API | Yes | `test/plugin/host-trend-show.ci` |
| Pipe completeness | Yes | The handler returns answer data through the shared renderer, so the generic pipes apply |
| Env var registration | No | No env var is added |
| Doctor check for runtime dependencies | Yes | New procfs reads (`/proc/slabinfo`, `/proc/sys/fs/file-nr`, the conntrack count and maximum): each rule's runtime check reports an unreadable source, with codes registered from the rule's file |
| Doctor code per problem class | Yes | Runtime phase, registered by `internal/component/host`: `doctor-host-kernel-memory-growth`, `doctor-host-file-descriptors-running-out`, `doctor-host-conntrack-running-out`, `doctor-host-disk-filling`, `doctor-host-trend-unreadable`, `doctor-host-trend-not-running` |
| Prometheus counters/metrics | Yes | `ze_host_trend_active{rule}`, `ze_host_trend_level{rule}`, `ze_host_trend_limit{rule}`, `ze_host_trend_exhaustion_seconds{rule}` |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP family work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/host-resource-trends.md` (new), `features/host-inventory.md` (the monitor's start) |
| 2 | Config syntax changed? | No | No config is added |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: `show host trend` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` if it lists the host commands; verified at implementation |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: `host-cmd` gains a command |
| 6 | Has a user guide page? | Yes | `docs/guide/monitoring.md`: a section on the trend rules, what each watches and what it cannot see |
| 7 | Wire format changed? | No | No protocol wire |
| 8 | Plugin SDK/protocol changed? | No | The SDK is not touched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC |
| 10 | Test infrastructure changed? | No | Existing runner and fake clock |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it compares built-in health detection; verified at implementation |
| 12 | Internal architecture changed? | Yes | `docs/architecture/host/observability.md` (the trend monitor, its start, its rules), `docs/architecture/traffic/traffic-analysis-layers.md` (the level primitives and the shared state machine) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md`, `docs/guide/monitoring.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/guide/status.md` and the command inventory regenerated by `./le repo update` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/release/validation/spec-host-resource-trend-alerts.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/monitoring.md` per-collector interval example (AC-26) and the Conntrack row |

## Design presentation

### What changes

Ze keeps a short history of four host resources inside the daemon (unreclaimable kernel memory, open file handles, the conntrack table, the persistent disk), sampled by the host component whether or not telemetry or BGP is configured, and judges each against a rule built into its own file. A rule that sees the floor of a resource rise without settling, on a line that reaches exhaustion within the rule's horizon, or a hard kernel limit nearly reached, confirms after three evaluations and raises one warning on the report bus. The warning names the resource, the growth observed, the projected time to exhaustion and the command to run, and it clears itself after three quiet evaluations. The operator meets it in `show warnings`, in `show doctor` through the rule's own runtime check, in the `host-trend` row of `show health`, in the new `show host trend` command and in `ze_host_trend_*` metrics. There is no operator setting. The history survives a daemon restart on the same boot and is discarded across a reboot. The shared pieces go to `internal/core/stats`: a level history with its growth and fill patterns, and the one confirm-and-clear state machine that ddos-detect and anomaly-detect move onto.

The first version Thomas ordered builds the `kernel-memory` rule end to end, from the hub's start to `show host trend` and `show doctor`, and leaves the rest owed (section "First version").

Two tooling defects met while taking this spec to approval from its worktree are fixed here, each with its acceptance criterion: a design judge in a git worktree now finds its session's read records (AC-30), and the design-page derivation reads every path of a file bullet (AC-31). The second fix is why this design's table grew from 28 derived pages to 49.

### Before and after

1. `show warnings` on a box whose unreclaimable kernel memory has grown for nine hours. Before: no warning; the operator learns of the leak when the box runs out of memory. After: one warning, source `host`, code `doctor-host-kernel-memory-growth`, subject `kernel-memory`, message "Kernel memory that cannot be reclaimed grew by 412 MiB over the last 9 hours and is still growing. At this rate the box runs out of memory in about 2 days. The kernel caches that grew most: kmalloc-256 (+380 MiB). This is the pattern of a kernel memory leak: a kernel update fixes it, and a reboot frees the memory until it grows again.", fix `show host trend`. In the first version the message carries no cache sentence.
2. `show doctor` on the same box. Before: no row about memory growth, and the box reads ready. After: one warning row from the `kernel-memory` check, with the same message and fix; `recent-events` adds no second row for it, and the box still reads ready. On a box where the rule is quiet the check adds nothing. On a daemon whose trend monitor never started, a row says the trend monitor is not running, with the command that restarts it.
3. `show host trend`. Before: no such command; `show host` offers cpu, nic, dmi, memory, thermal, storage, kernel, platform and all. After: one entry per registered rule with its state (not present, warming up, quiet, confirming, active, unreadable), level, limit, observed duration, growth over the window, the projected time to exhaustion when one exists, and for `kernel-memory` the caches that grew most. In the first version it lists `kernel-memory` alone, without the caches.
4. `show health` while a trend warning is active. Before: no `host-trend` row. After: `host-trend` degraded, healthy again once the warning clears; the `/health` endpoint still answers 200. Owed after the first version.
5. A Prometheus scrape with the exporter enabled. Before: no trend series. After: per rule `ze_host_trend_active`, `ze_host_trend_level`, `ze_host_trend_limit` and `ze_host_trend_exhaustion_seconds`, the last positive infinity while nothing is projected. Owed after the first version.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Sample through the telemetry collectors | They are compiled out without `ze_telemetry`, run only while the exporter is enabled, and write into gauges nothing reads back; always-on code cannot import them |
| A new always-on component for the sampling | It would be a second host sampler beside the host monitor, which already samples the host and raises warnings |
| Deviation from a learned baseline (the anomaly detector's method) | A baseline learns a slow ramp, so a leak never reaches its threshold |
| A fixed threshold on the level | Wrong for every box size, and it fires only when it is nearly too late |
| A config tree of rules | Thomas ruled for built-in rules |
| A table of rules in the monitor | A central list every new rule would have to edit; each rule registers from its own file instead |
| One doctor check listing every rule's code | The doctor-coverage gate wants each code declared by a check literal; one check per rule keeps that declaration in the rule's file |
| History in memory only | A daemon restart during a leak would blind the kernel-memory rule for six more hours |
| Read every slab cache at every sample | The per-cache file is large on a busy box and is needed only to name the culprit, so it is read only while the rule is confirming or active |
| A third copy of the confirm-and-clear machine in the host component | Two copies exist already; `ai/rules/no-layering.md` |

### Documents departed from

`docs/architecture/diagnostics/production-diagnostics.md`, section "Health registry": "The HTTP handler answers 200 when every check passes and 503 otherwise". After this work (AC-28) the page says that the handler answers 503 only when a check is down, and 200 when every check is healthy or degraded, which is what `Registry.Handler` in `internal/core/health/registry.go` does today. The design relies on it: the `host-trend` row reads degraded and the endpoint still answers 200 (AC-20).

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| D-1 | Release bucket | (a) `release/validation`, P2: the doctor rule makes an unreported problem class owed; (b) `post-release/features`, P3: the same spec, scheduled after the first release | (a): "keep it in release/validation", P2 stays |
| D-2 | The four rules and their figures (table "The built-in rules") | (a) as written, deliberately slow to alarm; (b) tighter figures, which alarm earlier and more often on a box that is only warming up | (a): the first version uses the figures as written; they stay unmeasured until the lab run the spec owes (phase 10, A-2) |
| D-3 | Severity | (a) every trend finding is a warning, so the box still reads ready; (b) the fill pattern an error, so `show doctor` reports the box not ready at 90% | (a): "every trend finding is a warning" |
| D-4 | An off switch | (a) none, per the built-in ruling: a false alarm is fixed by correcting the rule; (b) a per-rule disable leaf under the host config, which is operator configuration | (a): "no off switch" |
| D-5 | The hysteresis conversion of ddos-detect and anomaly-detect rides in this spec | (a) yes, otherwise the tree gains a third copy; (b) leave both and add the shared type beside them, which `ai/rules/no-layering.md` argues against | (a): yes; ddos-detect is converted in the first version, in a commit of its own, and anomaly-detect in the later pass |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the monitor starts from the hub and answers `show host trend` with no rule
   - Tests: `host-trend-runs-without-telemetry`, `host-trend-show`, `TestTrendMonitorNotRunningIsReported`
   - Files: `internal/component/host/trend/trend.go`, `cmd/ze/hub/main_system.go`, `cmd/ze/hub/main.go`, the `host-cmd` YANG and handler
   - Verify: the command is reachable and lists nothing; the wiring tests fail on the missing rules
2. **Phase: Shared primitives** -- the level history, the two patterns
   - Tests: the `internal/core/stats/level_test.go` rows
   - Files: `internal/core/stats/level.go`
   - Verify: tests fail, implement, tests pass
3. **Phase: One state machine** -- move the ddos-detect machine to core, convert ddos-detect and anomaly-detect
   - Tests: `TestHysteresisConfirmAndClear`, and the existing ddos-detect and anomaly-detect tests unchanged
   - Files: `internal/core/stats/hysteresis.go`, `internal/plugins/ddos/detect/`, `internal/plugins/anomaly/detect/detector.go`
   - Verify: the old type is deleted before the new one is used (`ai/rules/no-layering.md`); the detectors' tests stay green. A commit of its own
4. **Phase: Monitor core** -- rule registry, run loop on `clock.Clock`, states, raise and clear, stop
   - Tests: `TestRuleRegisteredIsSampledAndListed`, `TestReaderErrorAndNotPresentAreDistinctStates`, `TestTrendMonitorStopClearsOnlyItsOwnWarnings`
   - Files: `internal/component/host/trend/trend.go`
   - Verify: a test rule registered from the test drives every state
5. **Phase: Kernel-memory rule** -- the slab fields, the rule, the cache detail
   - Tests: the `trend_memory_linux_test.go` rows, `TestTrendMonitorRaisesAndClearsThroughRunLoop`
   - Files: `internal/component/host/memory_linux.go`, `internal/component/host/trend/register_kernel_memory.go`
   - Verify: the fixture `/proc` tree drives a leak and a settling load
6. **Phase: The other three rules** -- file descriptors, conntrack, disk, each with its fix command read at its handler (A-5, A-6)
   - Tests: the `trend_fd`, `trend_conntrack` and `trend_disk` rows
   - Files: one file per rule
   - Verify: each rule is one file and no other file changes
7. **Phase: Persistence** -- the snapshot key, save, load, boot identifier
   - Tests: the `persist_test.go` rows
   - Files: `pkg/zefs/keys.go`, `internal/component/host/trend/persist.go`
   - Verify: a restart keeps a growth run; another boot discards it
8. **Phase: Operator surfaces** -- the doctor checks and codes, the health row, the metrics, the full `show host trend` answer
   - Tests: the `trend_doctor_test.go` and `trend_metrics_test.go` rows, `host-trend-doctor`
   - Files: `internal/component/host/trend/register.go`, `internal/component/host/trend/metrics.go`, the docs of the Documentation Checklist as each surface lands
   - Verify: the doctor-coverage gate accepts the host component
9. **Phase: Adjacent defects** -- AC-24 to AC-29, each a commit of its own
   - Tests: `TestRecentEventsActiveWarningWording`, `TestReportSiteJudgesFixVariants`, `TestRateCollectorUsesItsOwnInterval`, `TestStatusDuringPollIsRaceFree`
   - Files: the five files of work group 6 and the three pages
   - Verify: each test is red before its fix
10. **Phase: Lab validation of the figures** -- A-2: a full table held for eight hours stays quiet; a leaking kernel module confirms the rule
    - Tests: the lab run, recorded in the spec with its figures
    - Files: none unless a figure is corrected
    - Verify: both runs on Ze's kernel

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | A reader error, "not present" and a real zero are three different answers at every step; no projection is computed from fewer buckets than the rule requires; a limit of zero never divides |
| Naming | Rule names and codes are kebab-case; metric names follow `ze_host_*`; JSON keys of `show host trend` are kebab-case |
| Data flow | Only the monitor's goroutine writes the history; every reader takes a snapshot; no rule name appears outside its own file |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line and raise names the code the Integration Checklist's doctor row lists, and each doctor check carries `Fix` at the phase that holds its state (`ai/rules/user-facing-errors.md`) |
| Rule: `ai/rules/no-layering.md` | The ddos-detect state machine and the anomaly-detect counters are gone, not kept beside the shared type |
| Rule: `ai/rules/goroutine-lifecycle.md` | One long-lived goroutine, stopped and waited for at daemon stop |
| Rule: `ai/rules/principles.md`, silent zero | The exhaustion metric is positive infinity when nothing is projected; a rule with too little history says "warming up", not "quiet" |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The monitor runs without telemetry or BGP | `./le test functional plugin` selecting `host-trend-runs-without-telemetry` |
| Four rules, each one file | `ls internal/component/host/trend/register_*.go`; a grep for each rule name finds it in its own file and its test only |
| One state machine in the tree | A grep for the confirm and clear counters finds `internal/core/stats/hysteresis.go` and no copy in ddos-detect or anomaly-detect |
| The doctor rows and codes | `TestTrendDoctorCheckRowFollowsRuleState`; `./le arch` doctor-coverage on `internal/component/host` |
| The history survives a restart | `TestTrendMonitorRestoresHistoryAcrossRestart` |
| The figures hold on a real box | The lab record of phase 10 in this spec |
| The pages match the code | `./le doc check all` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | Every value read from `/proc` and from the saved snapshot is parsed and bounded; a snapshot with an invalid value is discarded whole |
| Resource exhaustion | The history is a fixed number of buckets per rule; the slab read is bounded to once per bucket and only while a rule is confirming or active; the cache list in a message is capped |
| Information exposure | `show host trend` shows kernel cache names and sizes: the same authorization as the other `show host` commands |
| Fail open | A reader that fails must never read as a healthy rule (AC-8) |
| Message bounds | The report bus cuts a message at 1024 bytes and a fix at 256: the rule's message fits with the cache list capped |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| The lab run alarms on a healthy box | Correct the rule's figures with the measurement, record both in the spec |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The first sketch of this design named the telemetry collectors as the source of samples. Reading the code showed that is impossible: the collectors are compiled out without `ze_telemetry`, run only while the exporter is enabled, and write into gauges nothing can read back. The source is the host component, which already samples the host and raises warnings
- Two pages state that metric collection is always on. It is not: AC-29
- The monitor could not live in the host package: `internal/core/diagnostic` imports `internal/component/host`, so a doctor check registered from the host package would close an import cycle. It is the subpackage `internal/component/host/trend`, which imports the host package for its readers (`host.(*Detector).DetectKernelMemory`) and registers its codes and checks from its own `register*.go` files (commit d2376b7e29). The same direction puts the trend gauges in that subpackage, not in the host package's `metrics.go`
- The write hook takes an `init()` that registers only from a file named `register*.go`, so each rule's file is `register_<rule>.go`

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The host component samples for the trend rules | Read the telemetry collectors' samples; add a new always-on component | The collectors are gated and write-only, and an always-on file cannot import them. A new component would be a second host sampler beside the host monitor |
| Growth is judged on bucket minimums with a projected exhaustion | Deviation from a learned baseline (the anomaly detector's method); a fixed threshold on the level | A baseline learns a slow ramp and never fires. A fixed threshold on kernel memory is wrong for every box size, and fires only when it is nearly too late |
| Rules are constants of their own file, registered | A config tree of rules; a table in the monitor | Thomas ruled for built-in rules. A table in the monitor is a central list every new rule would have to edit |
| One runtime doctor check per rule | One check listing every rule's code | The doctor-coverage gate wants each code declared by a check literal; one check per rule keeps that declaration in the rule's file |
| The history is kept across a daemon restart, discarded across a reboot | Memory only | The kernel-memory rule needs six hours: a daemon restart during a leak would otherwise blind it for six more. Kernel memory starts again at boot, so an older history describes another kernel run |
| The slab caches are read only while the rule is confirming or active | Sample every cache always | The per-cache file is large on a busy box and is needed only to name the culprit |
| The state machine moves to core and both detectors use it | Add a third copy in the host component | Two copies already exist; `ai/rules/no-layering.md` |

## Known Limitations

- A leak slower than the rule's figures (under 1% of memory in six hours, or exhaustion further than seven days away) is not reported. The guide says so
- The rule names the kernel caches that grow. It does not name the feature whose use triggers the leak (for example flow offload together with tunnels going down): that needs knowledge of each kernel defect
- Only the four resources above are watched. A fifth is a new rule file
- A resource owned by a plugin that runs in another process cannot register a rule through this registry: the registry is in the daemon. No such rule is needed today
- Trend monitoring is Linux-only, like the readers it uses
- There is no operator setting: the rules and their figures are built in ("Choices for Thomas", D-4)

## RFC Documentation (Scope: protocol)

N-A: Scope is plugin. No RFC behavior is implemented.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
