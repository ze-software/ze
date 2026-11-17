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
| Updated | 2026-11-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

The design below is not approved yet. Thomas approves it before the status moves to `ready` (`ai/META-DECISIONS.md`, "How agents work"). The decisions that are his are listed in "Owner decisions pending".

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
| Telemetry OS collectors | `internal/component/telemetry/collector` `Manager`, `Collector` | Sample 31 OS sources into Prometheus gauges while the exporter is enabled | Does not apply as the source: gated by `ze_telemetry` and by the exporter being enabled, and write-only. Always-on code cannot import it. It stays the exporter's source, unchanged |
| Traffic statistics primitives | `internal/core/stats` `Window`, `EWMA`, `Mean`, `StdDev` | Rolling byte-rate window, moving average, basic statistics; no clock inside | Extends: the level history and its growth and projection functions join the package under the same contract (pure, caller passes time, caller serializes) |
| ddos-detect state machine | `internal/plugins/ddos/detect/state.go` `stateMachine` | Confirm after N consecutive evaluations above, clear after M below | Replaces: the type moves to `internal/core/stats`, exported, and ddos-detect uses it from there. No copy remains in ddos-detect |
| anomaly-detect inline hysteresis | `internal/plugins/anomaly/detect/detector.go` `stepEntity` | The same confirm and clear logic with two counters | Replaces: converted to the shared type, so the tree holds one implementation instead of a third |
| anomaly-detect baseline scoring | `internal/plugins/anomaly/detect/score.go` | Scores deviation from a learned per-entity baseline | Does not apply: its entities are a closed set of traffic kinds, and a baseline learns a slow ramp instead of flagging it |
| ddos-detect baseline persistence | `internal/plugins/ddos/detect/persist.go` `saveBaselines`, `loadBaselines` | Saves a versioned snapshot through statestore and validates it on load | Extends the pattern: the monitor saves its history the same way under its own key, and additionally refuses a snapshot from another boot |
| Report bus | `internal/core/report/report.go` `RaiseWarningFix`, `ClearWarning` | Holds active warnings keyed by (source, code, subject), shown by `show warnings` | Extends: the monitor is a new producer; no change to the bus |
| Health probe from the bus | `internal/core/report/health_probe.go` `HealthProbeDegraded` | Derives a health row from active warning codes | Extends: one probe registered for the trend codes, so the bus stays the one source of the alert |
| Runtime doctor checks | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck` | A package registers a check that reads its live state | Extends: each rule registers its own runtime check declaring its code |
| SMART `temp-rising` | `internal/component/storage/manager.go` `checkTemperature` | Compares two consecutive polls | Does not apply: a two-sample delta is not a trend, flaps, and cannot see a slow rise. Left as it is; its codes are `spec-doctor-coverage-15-system` work |
| Static disk-space check | `internal/component/doctor/checks_storage.go` `checkDiskSpace` | Reports a config partition that is nearly full at the time doctor runs | Extends: the disk rule judges growth on the same partitions and leaves the fill level to this check |
| Fake clock | `internal/test/sim/sim.go` `FakeClock` | Drives `clock.Clock` timers in tests | Extends its use: the monitor takes a `clock.Clock`, so a test runs days of samples at once |

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
| Kernel to host component | File reads under the detector root, `statfs` | No |
| Host component to report bus | `report.RaiseWarningFix` and `report.ClearWarning`, same process | No |
| Host component to statestore | Acknowledged write and read under one registered key | No |
| Host component to `host-cmd` | The command handler reads the monitor's published state. This is a same-process read (assumption A-3) | No |
| Daemon to operator | `show host trend`, `show warnings`, `show doctor`, `show health`, the metrics endpoint | No |

### Changed-Wire Census

**Search evidence:** No existing wire changes its format: the spec adds four formats and alters none. Scope searched: `internal/`, `cmd/`, `pkg/`, `test/`, `docs/`, non-Go files included, for `ze_host_`, `show host`, `meta/` state keys in `pkg/zefs/keys.go`, and the report-bus codes raised by source `host`. Positive control: the search finds the existing `ze_host_memory_available_bytes` gauge at `internal/component/host/metrics.go` and the `show-memory` command at `internal/plugins/host-cmd/yang/ze-host-cmd.yang`, so a reader of an existing host format would have been found. No reader of a `show host trend` answer, a `ze_host_trend_` metric, a host trend state key or a `doctor-host-*-growth` code exists yet, because none of them exists.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Trend history snapshot | writer | `internal/component/host`, the trend monitor's save | New key registered in `pkg/zefs/keys.go`; no writer exists today | New, versioned; nothing to migrate |
| Trend history snapshot | reader | `internal/component/host`, the trend monitor's load at start | No reader exists today; `ze data registered` lists the key to an operator | New; a snapshot of an unknown version or another boot is discarded, never guessed at |
| `show host trend` answer | writer | `internal/plugins/host-cmd/cmd`, the new handler | `ze-host-cmd.yang` has no `trend` node today | New |
| `show host trend` answer | reader | The shared renderers, and API, MCP and web clients outside the repository | Command answers reach every interface through the command registry (`ai/doors/system.md`) | New; declared answer shape documented in the command reference |
| `ze_host_trend_*` metrics | writer | `internal/component/host/metrics.go` | No such name in the tree | New names only; existing names untouched |
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

## Owner decisions pending

| # | Decision | Recommendation | What the other answer changes |
|---|----------|----------------|-------------------------------|
| D-1 | Release bucket | `release/validation`, P2: the doctor rule makes an unreported problem class owed | `post-release/features`, P3: the same spec, scheduled after the first release |
| D-2 | The four rules and their figures (table above) | As written; they are deliberately slow to alarm | Tighter figures alarm earlier and more often on a box that is only warming up |
| D-3 | Severity | Every trend finding is a warning: the box still reads ready | Making the fill pattern an error makes `show doctor` report the box not ready at 90% |
| D-4 | No off switch | None, per the built-in ruling; a false alarm is fixed by correcting the rule | A per-rule disable leaf under the host config, which is operator configuration |
| D-5 | The hysteresis conversion of ddos-detect and anomaly-detect rides in this spec | Yes: otherwise the tree gains a third copy | Leave both as they are and add the shared type beside them, which `ai/rules/no-layering.md` argues against |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The host component is always linked and the hub may start it directly | `feature-gates.txt` has no row for `internal/component/host` | The monitor needs a registration seam like a gated service | `./le arch tier check` after the wiring phase | unvalidated |
| A-2 | A leak raises the hourly minimum of unreclaimable slab for hours on end, and legitimate growth (a full routing table, connection load) settles within six hours | A kernel leak grows without bound by nature; reasoning about route and connection load, not a measurement | The kernel-memory rule alarms on a healthy converging router, or misses a leak slower than 1% of memory in six hours | A lab run: load a full table and hold it for eight hours with the rule quiet; then a kernel module that leaks, with the rule confirming | unvalidated |
| A-3 | `host-cmd` handlers run in the daemon process, so the handler can read the monitor's state by a call | `show host storage` already reaches the SMART manager through `SetStorageManager` in `internal/component/storage/show.go` | `show host trend` answers from an empty copy when the plugin is forked; the state must then travel by a typed request to the daemon | Reading how `host-cmd` is launched, and `./le plugin boundary check` | unvalidated |
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
| `TestLevelHistoryBucketsMinimumAndLast` | `internal/core/stats/level_test.go` | AC-1: bucket contents | |
| `TestLevelHistoryRecordsMissingBucket` | `internal/core/stats/level_test.go` | AC-1: a period with no sample | |
| `TestGrowthHoldsOnSustainedRise` | `internal/core/stats/level_test.go` | AC-2: true with rise, span and projection | |
| `TestGrowthFalseWhenRiseSettles` | `internal/core/stats/level_test.go` | AC-2: the survivor | |
| `TestGrowthRunEndsAtMissingBucket` | `internal/core/stats/level_test.go` | AC-3 | |
| `TestFillAgainstLimit` | `internal/core/stats/level_test.go` | AC-4, zero limit included | |
| `TestHysteresisConfirmAndClear` | `internal/core/stats/hysteresis_test.go` | AC-5: the moved machine's contract | |
| `TestRuleRegisteredIsSampledAndListed` | `internal/component/host/trend_test.go` | AC-7 | |
| `TestReaderErrorAndNotPresentAreDistinctStates` | `internal/component/host/trend_test.go` | AC-8 | |
| `TestTrendMonitorRaisesAndClearsThroughRunLoop` | `internal/component/host/trend_test.go` | AC-12, AC-17: the run loop over a fixture `/proc` and a fake clock | |
| `TestTrendMonitorRestoresHistoryAcrossRestart` | `internal/component/host/trend_persist_test.go` | AC-10: same boot | |
| `TestTrendMonitorDiscardsSnapshotFromAnotherBoot` | `internal/component/host/trend_persist_test.go` | AC-10: other boot, bad version, invalid value | |
| `TestTrendMonitorStopClearsOnlyItsOwnWarnings` | `internal/component/host/trend_test.go` | AC-11 | |
| `TestKernelMemoryRuleQuietOnGrowthThatSettles` | `internal/component/host/trend_memory_test.go` | AC-12: the survivor | |
| `TestKernelMemoryRuleNamesGrowingCaches` | `internal/component/host/trend_memory_test.go` | AC-13: readable | |
| `TestKernelMemoryRuleReportsUnreadableSlab` | `internal/component/host/trend_memory_test.go` | AC-13: unreadable | |
| `TestFileDescriptorRuleFillAndGrowth` | `internal/component/host/trend_fd_test.go` | AC-14 | |
| `TestConntrackRuleFillAndAbsentModule` | `internal/component/host/trend_conntrack_test.go` | AC-15 | |
| `TestDiskRuleGrowthOnly` | `internal/component/host/trend_disk_test.go` | AC-16 | |
| `TestTrendDoctorCheckRowFollowsRuleState` | `internal/component/host/trend_doctor_test.go` | AC-18 | |
| `TestTrendMonitorNotRunningIsReported` | `internal/component/host/trend_doctor_test.go` | AC-19 | |
| `TestTrendHealthRowDerivesFromBus` | `internal/component/host/trend_doctor_test.go` | AC-20 | |
| `TestTrendMetricsFollowRuleState` | `internal/component/host/trend_metrics_test.go` | AC-22 | |
| `TestRecentEventsActiveWarningWording` | `internal/component/doctor/checks_recent_events_test.go` | AC-24 | |
| `TestReportSiteJudgesFixVariants` | `internal/le/arch/doctorcoverage/sites_test.go` | AC-25 | |
| `TestRateCollectorUsesItsOwnInterval` | `internal/component/telemetry/collector/collector_test.go` | AC-26 | |
| `TestStatusDuringPollIsRaceFree` | `internal/component/storage/manager_test.go` | AC-27, under the race detector | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Consecutive rising buckets for growth | 1 to the bucket count | the bucket count | 0 | the bucket count plus 1 |
| Fill share | above 0 to 100% | 100% | 0 | above 100% |
| Limit | 1 and above | any positive value | 0 (answers "cannot judge") | N/A |
| Confirm and clear counts | 1 and above | any positive value | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `host-trend-runs-without-telemetry` | `test/plugin/host-trend-runs-without-telemetry.ci` | An operator with no telemetry and no BGP configured runs `show host trend` and sees every rule observed for a growing duration | |
| `host-trend-show` | `test/plugin/host-trend-show.ci` | `show host trend` answers the declared shape for each rule from the real `/proc` | |
| `host-trend-doctor` | `test/plugin/host-trend-doctor.ci` | `show doctor` runs each rule's check and the monitor check on a healthy box and reports none of them as a problem | |

### Interop Tests (Scope: protocol)
N-A: Scope is plugin. No wire protocol is spoken to a peer.

## Files to Modify
- `internal/core/stats/` - the level history, growth and fill patterns, and the shared confirm-and-clear state machine
- `internal/plugins/ddos/detect/state.go` and its users - use the shared state machine; the local type is removed
- `internal/plugins/anomaly/detect/detector.go` - use the shared state machine in place of the inline counters
- `internal/component/host/memory_linux.go` - the slab fields of `/proc/meminfo`
- `internal/component/host/metrics.go` - the trend gauges
- `internal/component/host/register.go` - registration of the monitor's codes and checks
- `cmd/ze/hub/main_system.go`, `cmd/ze/hub/main.go` - start and stop the trend monitor at system start and stop
- `pkg/zefs/keys.go` - the history snapshot key
- `internal/plugins/host-cmd/yang/ze-host-cmd.yang`, `internal/plugins/host-cmd/cmd/show_host.go`, `internal/plugins/host-cmd/cmd/register.go` - the `show host trend` command
- `internal/component/doctor/checks_recent_events.go` - wording for an active warning
- `internal/le/arch/doctorcoverage/sites.go` - judge the `*Fix` raise variants
- `internal/component/telemetry/collector/collector.go` and the rate collectors - the effective interval reaches the collector
- `internal/component/storage/manager.go` - the poll's writes under the manager's lock
- `docs/architecture/host/observability.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/diagnostics/production-diagnostics.md`, `docs/architecture/traffic/traffic-analysis-layers.md`, `docs/guide/monitoring.md`, `docs/guide/command-reference.md`, `docs/plugin-development/metrics.md`, `ai/digests/observation-telemetry.md`, `ai/INDEX.md`, `features/host-inventory.md`

## Files to Create
- `internal/core/stats/level.go`, `internal/core/stats/hysteresis.go` and their tests
- `internal/component/host/trend.go` (the monitor and the rule registry), `trend_persist.go`, `trend_doctor.go`, and one file per rule: `trend_memory_linux.go`, `trend_fd_linux.go`, `trend_conntrack_linux.go`, `trend_disk_linux.go`, with their tests and a non-Linux file answering that trend monitoring is Linux-only
- `test/plugin/host-trend-runs-without-telemetry.ci`, `test/plugin/host-trend-show.ci`, `test/plugin/host-trend-doctor.ci`
- `features/host-resource-trends.md`

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

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the monitor starts from the hub and answers `show host trend` with no rule
   - Tests: `host-trend-runs-without-telemetry`, `host-trend-show`, `TestTrendMonitorNotRunningIsReported`
   - Files: `internal/component/host/trend.go`, `cmd/ze/hub/main_system.go`, `cmd/ze/hub/main.go`, the `host-cmd` YANG and handler
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
   - Files: `internal/component/host/trend.go`
   - Verify: a test rule registered from the test drives every state
5. **Phase: Kernel-memory rule** -- the slab fields, the rule, the cache detail
   - Tests: the `trend_memory_test.go` rows, `TestTrendMonitorRaisesAndClearsThroughRunLoop`
   - Files: `internal/component/host/memory_linux.go`, `trend_memory_linux.go`
   - Verify: the fixture `/proc` tree drives a leak and a settling load
6. **Phase: The other three rules** -- file descriptors, conntrack, disk, each with its fix command read at its handler (A-5, A-6)
   - Tests: the `trend_fd`, `trend_conntrack` and `trend_disk` rows
   - Files: one file per rule
   - Verify: each rule is one file and no other file changes
7. **Phase: Persistence** -- the snapshot key, save, load, boot identifier
   - Tests: the `trend_persist_test.go` rows
   - Files: `pkg/zefs/keys.go`, `internal/component/host/trend_persist.go`
   - Verify: a restart keeps a growth run; another boot discards it
8. **Phase: Operator surfaces** -- the doctor checks and codes, the health row, the metrics, the full `show host trend` answer
   - Tests: the `trend_doctor_test.go` and `trend_metrics_test.go` rows, `host-trend-doctor`
   - Files: `trend_doctor.go`, `metrics.go`, `register.go`, the docs of the Documentation Checklist as each surface lands
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
| Four rules, each one file | `ls internal/component/host/trend_*_linux.go`; a grep for each rule name finds it in its own file and its test only |
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
- There is no operator setting: the rules and their figures are built in (decision D-4)

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
