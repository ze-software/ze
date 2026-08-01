# 808 - SMART Disk Health Management

Spec: `plan/spec-smart-management.md` (closed 2026-06-10 via two-commit flow)

## What Was Built

YANG-modeled SMART disk health management for Ze: auto-enable, periodic health
polling, three-tier temperature alerting, self-test scheduling, and live status
via `show storage smart`. Ze is the first NOS with configurable SMART management
(VyOS, SONiC, Cumulus, Arista have none or hardcoded-only).

### Core Library (`internal/core/smart/`)
- `smart.go`: `Info` type (health, temperature, power-on hours, error count,
  NVMe percent-used/available-spare), `ParseNVMeBuf`, `NvmeNamespace`
- `smart_linux.go`: ATA (`HDIO_DRIVE_CMD`) and NVMe admin passthrough ioctls for
  Detect, Enable, StartSelfTest, IsSelfTestInProgress. Pure ioctl, no smartctl
  dependency (gokrazy-safe).
- `smart_other.go`: non-Linux stubs
- `smart_test.go`: NvmeNamespace partition stripping, ParseNVMeBuf field extraction

### Storage Component (`internal/component/storage/`)
- `manager.go`: Manager with Start/Stop/Reconfigure, ticker+stopCh lifecycle.
  Poll loop discovers devices via sysfs, reads SMART, checks thresholds, schedules
  self-tests. Cleans up removed devices (clears warnings).
- `config.go`: Config, TemperatureConfig, SelfTestConfig, DefaultConfig
- `discover_linux.go`: `/sys/class/block/` enumeration, partition filtering
- `schema/ze-storage-conf.yang`: full YANG schema with range constraints on all
  numeric leaves, pattern-constrained time/interval strings, day enumeration
  (actual path `internal/component/storage/yang/ze-storage-conf.yang`)
- `manager_test.go`: 16 unit tests covering lifecycle, temperature alerting
  (informational, critical, rate-of-change, clearing), health status dedup,
  reconfigure, time-of-day matching, day-of-week matching

### Show RPC (`internal/component/storage/show.go`)
- `ze-show:storage-smart` RPC registered via `pluginserver.RegisterRPCs`
- `atomic.Pointer[storage.Manager]` set by hub, handler calls `Manager.Status()`;
  returns the stable error `storage SMART management not configured` when the
  manager pointer is nil (no `storage { smart { ... } }` in config)
- CLI verb declared in `internal/plugins/storage-cmd/yang/ze-storage-cmd.yang`

### Hub Wiring (`cmd/ze/hub/main_system.go`)
- `startSmartManager`: extract config from tree, create Manager, Start, wire show RPC
- `stopSmartManager`: Stop on shutdown
- `reloadSmartManager`: live reconfigure or create/destroy on config change
- Pattern matches archive scheduler (not plugin registration)

### Doctor Check (`internal/component/doctor/checks_linux.go`)
- `checkSmartEnabled`: when config has `storage.smart.enabled=true`, enumerates
  sysfs devices and warns if none are SMART-accessible

### Functional Tests
- `test/parse/smart-config.ci`: full SMART config validates cleanly
- `test/plugin/smart-show.ci`: configured daemon -> `show storage smart` returns
  `done` with a `devices` list (proves the RPC is wired through the plugin
  dispatch entry point; `devices` is empty on a non-Linux/no-block-device host)
- `test/plugin/smart-show-unconfigured.ci`: daemon with no SMART config ->
  `show storage smart` returns the stable not-configured error (proves the
  error contract and that an unconfigured daemon does not panic)

## Key Decisions

- **Core library, not component-internal.** Spec planned SMART ioctls in
  `component/storage/smart_linux.go`. Placed in `core/smart/` instead because the
  host detector also needs Detect (for `ze host show storage`). Core avoids
  component-to-component import coupling.
- **Host files kept as thin wrappers.** Spec planned to delete
  `host/smart_linux.go`. Kept as one-liner delegating to `core/smart.Detect`,
  preserving the `Detector.detectSMART(name)` method for testdata-mode routing.
  Zero duplication.
- **No register.go for storage.** Storage is a system service, not a plugin.
  Hub creates and manages its lifecycle directly, matching the archive scheduler
  pattern. Plugin-style registration would add indirection for no benefit.
- **Fail-open scheduling.** Invalid time-of-day or unrecognized day names match
  always rather than silently blocking tests. A user who types an invalid time
  gets tests at any time rather than never.
- **Health dedup via `healthReported` flag.** SMART-failing error is raised once
  per failure episode (ring-based), cleared on recovery. Prevents flood on
  persistent failure.

## Traps and Pitfalls

- **ATA self-test in-progress detection**: data offset 363 in the SMART data
  page, bits 7:4. Value 0x0F means in progress. Other non-zero values indicate
  completed-with-error, not in-progress.
- **NVMe self-test log page is 0x06, not 0x02.** Log page 0x02 is SMART/Health.
  The self-test log is a separate Get Log Page call.
- **Critical clears informational.** When temperature crosses the critical
  threshold, the informational `temp-high` warning must be explicitly cleared
  to avoid stale warnings at two severity levels simultaneously.
- **Device removal must clear warnings.** Poll tracks seen devices; removed
  devices get their warnings cleared to avoid phantom alerts.

## What Would Change Next Time

- Add `TestSelfTestSkipsInProgress` with a mock for `IsSelfTestInProgress`.
  The in-progress guard is code-level obvious but untested.
- Consider a `storage.Register()` function if other components need to discover
  the storage manager (currently only show RPC needs it, wired via atomic pointer).

## Closure Lesson (2026-06-10)

The first closure attempt set the spec to `Status: done` and wrote this summary
*before* the AC-8 functional test existed: the audit tables claimed
`test/plugin/smart-show.ci` was "Created" when no `.ci` exercised
`ze-show:storage-smart`, and the Review Gate was never filled. The two-commit
closure flow was also never run (the spec was never `git rm`'d). The closure
audit caught this, reverted to `in-progress`, and the gap was closed by actually
writing the two functional tests above and the Review Gate. Lesson: a claimed
test in an audit table is not evidence; verify the file exists and runs before
marking an AC done, and do not write the learned summary until closure actually
proceeds.

## Metrics

- 16 unit tests, 3 functional tests
- ~650 lines of new code across core/smart + component/storage
- Zero external dependencies (pure ioctl + sysfs)

## Files

None recorded.
