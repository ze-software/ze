# Host observability: cache, metrics, hardware-change events, trends

Four pieces sit on top of the inventory: a cache so a scrape does not re-read
sysfs, a Prometheus gauge set, a diff engine that names hardware changes, and a
trend monitor that warns when a resource grows toward exhaustion.

<!-- source: internal/component/host/cached.go -- CachedDetector, Detect, Invalidate -->
<!-- source: internal/component/host/metrics.go -- RegisterMetrics, CollectOnce, StartRefresh, Stop -->
<!-- source: internal/component/host/diff.go -- DiffInventory, DiffEvent -->

## Decisions

- **The cache refreshes lazily, on the next call after the TTL expires.** No
  background goroutine, so there are no timers to mock in a test. A zero TTL
  disables caching.
- **The diff engine returns values, it does not publish.** `DiffInventory`
  returns events and the caller turns them into report-bus issues. The engine
  stays pure and testable with no bus.
- **Three change categories, chosen for a network OS**: carrier change, ECC
  error, and throttle. A carrier flip affects convergence, an ECC error signals
  memory degradation, and a rising throttle count signals thermal pressure on
  forwarding.
- The metrics type does run a refresh goroutine, because Prometheus gauges must
  stay fresh between scrapes even though the cache itself is lazy.

## Traps

- **ECC counters are exported as gauges, not Prometheus counters.** The value
  comes from EDAC hardware counters that survive a process restart. A Prometheus
  counter would lose the absolute value every time Ze restarts.
- **The NIC carrier gauge is 1.0 or 0.0.** Prometheus has no boolean type.
- **The first snapshot returns nil, not an empty slice.** That is what
  distinguishes "no changes" from "no baseline yet". A caller that treats them
  alike alerts on every start.

## The trend monitor

A resource that runs out slowly, such as kernel memory a driver leaks, fails
the box days after it starts growing. No single reading shows it. The trend
monitor keeps a short history of such resources and warns while there is still
time to act.

**What starts it.** The hub's system start, beside the SMART manager, starts
one monitor on the real clock over the real filesystem root, and the daemon's
stop stops it. It runs whatever the configuration holds: no `telemetry` block
and no `bgp` block is needed. It has no configuration, so no reload step
touches it, and a reload keeps its history. When it cannot start, the hub logs
`doctor-host-trend-not-running`.
<!-- source: cmd/ze/hub/main_system.go -- startTrendMonitor, stopTrendMonitor -->
<!-- source: internal/component/host/trend/monitor.go -- Start, Stop, Monitor -->

**Where it lives.** The monitor is the package `internal/component/host/trend`,
under the host component rather than in it, because `internal/core/diagnostic`
imports the host component for the platform type, and the monitor registers
doctor checks. The readers stay in the host component and read under
`Detector.Root`, so a test feeds the monitor a fixture `/proc`.

**The rule registry.** A rule is a value its own file registers from `init()`:
its name, its doctor code, its sample period, its bucket period and count, the
consecutive rising buckets it compares, its horizon, its confirm and clear
counts, its reader and the function that words its warning. The same file
registers the rule's code, its event code and its runtime doctor check. The
monitor, the doctor checks and the published state iterate the registry and
name no rule, so a new rule is a new file.
<!-- source: internal/component/host/trend/trend.go -- rule, registerRule, validateRule -->

**One goroutine, one ticker.** The ticker ticks at the greatest common divisor
of the rules' sample periods, and each rule is read when its own period comes.
A reader answers one of three things, kept apart by type: a level with its
limit and the least rise that counts, "not present" (the resource does not
exist on this box), or an error naming the path. Each level goes into the
rule's level history (`stats.LevelHistory`). At each bucket close the monitor
judges the growth pattern and feeds the rule's confirm-and-clear machine
(`stats.Hysteresis`): true for "holds" and for "limit reached", which is the
projected exhaustion already arrived; false for "not rising", "rise too small"
and "beyond the horizon". A history too short to judge feeds nothing, so a rule
neither confirms nor clears on a window it has not seen.
<!-- source: internal/component/host/trend/monitor.go -- sample, sampleRule, fold -->

**What the operator sees.** On confirm the monitor raises a warning on the
report bus, source `host`, the rule's code, the rule's name as subject, and the
fix `show host trend`. It raises it again at each bucket close while it stands,
which refreshes the message and keeps the time of the first raise. It clears
that one key when the machine clears, and at stop it clears only its own keys:
the carrier and throttle warnings of the same source are not its own. A reader
that fails raises `doctor-host-trend-unreadable` for that rule until a read
succeeds. Each rule's runtime doctor check reports its warning while it stands;
the monitor's check reports a monitor that is not running and every rule that
cannot read its resource.
<!-- source: internal/component/host/trend/register.go -- checkMonitor, ruleCheck -->

The published state is one snapshot per rule, replaced under one lock after
each tick and read by `trend.Rules`, which answers `trend.ErrNotRunning`, never
an empty list, when no monitor runs. A rule's state is one of not-present,
warming-up, quiet, confirming, active and unreadable. The machine's clearing
state reads active: the warning is still on the bus until the clear, and a row
that said otherwise would disagree with `show warnings` and `show doctor`.
<!-- source: internal/component/host/trend/trend.go -- Rules, RuleStatus, State -->

**`show host trend`.** The host command plugin answers it from `trend.Rules`,
one object per rule under `rules`: `name`, `state`, `unit`, `observed-seconds`,
and, only when the rule has them, `level` and `limit` (numbers in the unit),
`growth` and `growth-window-seconds` (the rise the latest judgment measured
and the window it compared), `exhaustion-seconds` (the projection), `warning`
(the standing warning's message) and `failure-source` and `failure-cause` (what
the latest sample could not read). A figure the rule lacks is absent, never
zero, so a rule still warming up shows its observed duration and no
projection. The read is a plain call: host-cmd's handlers are builtin RPCs
that the daemon's own dispatcher serves (`pluginserver.RegisterRPCs`, loaded by
`NewServer` through `LoadBuiltins`), so they run in the process that holds the
monitor, as `show storage smart` reads the SMART manager. With no monitor
running the command refuses with an operator error whose fix is
`ze signal restart`, and with no daemon reachable the offline fallback of
`show host trend` says the daemon is needed, rather than reading `trend` as an
unknown inventory section.
<!-- source: internal/plugins/host-cmd/cmd/show_trend.go -- handleShowTrend, trendRuleOf, trendReadRefusal -->
<!-- source: internal/plugins/host/trend_offline.go -- runShowTrendOffline, trendOfflineRefusal -->

**The kernel-memory rule.** It watches `SUnreclaim`, the kernel memory that
cannot be reclaimed, against a limit of that level plus `MemAvailable`. It
samples every 60 seconds into hourly buckets, keeps 168 of them, and warns
when the hourly floor rose for 6 consecutive hours, by at least 1% of
`MemTotal`, on a line that reaches the limit within 7 days; it confirms and
clears after 3 judgments. The floor is the minimum of each hour: a leak raises
it, a burst of traffic raises only the peak. `DetectKernelMemory` reads the
three counters through the host's one meminfo parser, so `show host memory`
keeps its answer.
<!-- source: internal/component/host/trend/register_kernel_memory.go -- readKernelMemory, describeKernelMemory -->
<!-- source: internal/component/host/memory_linux.go -- DetectKernelMemory, scanMeminfo -->

## Related

- `inventory.md` for the detection library underneath
- `../traffic/traffic-analysis-layers.md` for the level history and the confirm-and-clear machine
