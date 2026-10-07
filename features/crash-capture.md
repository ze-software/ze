# Crash Capture

## Meta

| Field | Value |
|-------|-------|
| Name | Crash Capture |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/core/crashlog, internal/plugins/crashes, internal/appliance/kernelargs.go |
| Real-path tests | test/ui/cli-crashes-offline-show.ci, test/ui/cli-crashes-readiness.ci, test/ui/cli-crashes-kernel-kind.ci, test/plugin/support-crashes-kernel.ci |
| Docs | docs/guide/operations.md, docs/guide/appliance.md |
| Doc review | 2026-10-07: ring.Recent(64) in internal/core/crashlog/crashlog.go; ze_crashes_artifacts and ze_crashes_kernel_capture_armed gauges in internal/plugins/crashes/readiness.go |
| Defect review | 2026-10-07: plan/spec-crash-capture.md in progress (QEMU kernel-panic and warm-reboot labs unwritten); journal 2026-09-13 crashdump test row unfixed |
| Extra criteria | supported: a kernel panic in a QEMU guest is harvested into show crashes after warm reboot = test/appliance/crash-kernel-panic-harvest.ci |

## Description

Automatic stderr redirect captures panic stack traces from any goroutine. Forwarded to syslog (via `ze.log.destination`) in real time and persisted to crash files on disk. Crash reports include ring buffer context (last 64 log entries before the panic), version, build date, uptime. Crash dir autodetected (`/perm/ze/crash/` on gokrazy, fallback chain for other platforms). CLI: `show crashes`, `show crashes latest`. Env vars: `ze.crash.dir`, `ze.crash.keep`. **Kernel panics are captured too**: `system crash-dump enabled true` plus an appliance built with `image.crash-dump` reserves a named memory region the kernel writes its own panic message and backtrace into, which a warm reboot does not clear. Ze harvests the record on the next boot as a `kernel`-kind report in the same directory, under the same retention count, listed by the same `show crashes`. No Ze code runs at the moment of the fault. `show crashes` reports `configured` and `armed` separately, because a reservation is a boot argument: three `ze doctor` checks report the reservation, the record store and the crash directory. A full memory image is a separate opt-in and is amd64-only. Prometheus: `ze_crashes_artifacts` and `ze_crashes_kernel_capture_armed`, so a fleet can alert on capture being silently unarmed. <!-- source: internal/core/crashlog/kernel.go -- HarvestKernelCrashes, CrashReadiness --> <!-- source: internal/plugins/crashes/readiness.go -- bindMetrics, the two gauges --> <!-- source: internal/appliance/kernelargs.go -- crashDumpKernelArgs --> <!-- source: internal/core/crashlog/ -- Init, HandlePanic, redirectStderr, stderrReader --> <!-- source: internal/core/report/report.go -- Issue, RaiseWarning, RaiseError, Warnings, Errors --> <!-- source: internal/component/cmd/show/show.go -- handleShowWarnings, handleShowErrors --> <!-- source: internal/component/bgp/reactor/session_prefix.go -- raisePrefixThreshold, raisePrefixStale, raiseNotificationError, raiseSessionDropped --> <!-- source: internal/component/bgp/config/loader.go -- collectPrefixWarnings reads from report bus for login banner -->
