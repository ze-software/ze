# Spec: crash-capture

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/crash-capture.md` |
| Handoff | - |
| Updated | 2026-09-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze captures a Go panic today and captures nothing when the kernel dies.

`internal/core/crashlog` redirects fd 2, forwards panics to syslog, persists a
crash file carrying the last 64 log ring entries, and `show crashes` reads them
back after the restart. That subsystem is complete and this spec does not change
its behavior.

A kernel panic is the uncovered case. The box goes down, the running kernel is
gone, and nothing on the appliance survives to say why: root is read-only
SquashFS, there is no shell, and there is no busybox to run a post-mortem from.
The operator gets a reboot and no evidence.

This spec adds kernel crash capture in two phases. Phase 1 records the panic
message and backtrace into a small reserved memory region that survives a warm
reboot, and copies it into the existing crash directory at next boot. Phase 2
adds a full memory image behind an opt-in leaf, for the rare fault a backtrace
does not explain.

Prompted by VyOS T8868, which added `system option kdump`, `show system kdump`,
and folded kdump artifacts into `show tech-support report`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/diagnostics/crash-capture.md` - the userspace half, already shipped
  → Decision: crash capture already owns a directory, a retention count, a listing API and a CLI noun. A kernel panic record is a second ARTIFACT KIND in that system, not a second system.
  → Constraint: the crash directory is autodetected by probe order (explicit override, config dir `crash`, `/perm/ze/crash`, `/var/lib/ze/crash`, `/tmp/ze-crash`), each candidate creatable and writable. A kernel artifact MUST resolve through the same probe, never a second path policy.
  → Constraint: `HandlePanic` and `os.Exit(2)` live in `cmd/ze/main.go`, not in the library, because the native write hook blocks `panic()` and `os.Exit()` outside a main file.
  → Constraint: a crash file carries the last 64 log ring entries for context, so a kernel artifact should carry an equivalent context block rather than the bare kernel text.
- [ ] `docs/architecture/appliance/kernel-profiles.md` - a kernel profile is a pair of files in an open registry
  → Constraint: kernel capability is declared as a `CONFIG_` symbol floor verified in Go, never assumed from a built image.
- [ ] `docs/guide/appliance.md` - appliance storage and update model
  → Constraint: root is read-only SquashFS; the only writable store is the ext4 `/perm` partition, which is the last partition and is grown to fill the disk at build. Artifact capacity is a property of the deployed disk, never of the image.
  → Decision: gokrazy A/B updates replace the root partition and leave `/perm`, so an artifact written there survives the update that follows a crash.
- [ ] `docs/functional-tests.md` - suite layout and the QEMU runner
  → Constraint: Linux-only kernel behavior is proven inside `./le qemu`, registered in `internal/le/qemu/actions.go` and run by the nightly runtime-kernel labs. A unit test cannot prove a reserved region survives a reboot.

### Rules
- [ ] `ai/rules/cli.md` - grammar and output
  → Constraint: one concept carries one name. `show crashes` exists, so a kernel artifact extends that noun; `show kdump` and `show crash` MUST NOT be added beside it.
  → Constraint: the verb follows the effect on live state. Listing and reporting readiness is `show`. Arming a reservation changes what the machine does at next boot, so it belongs in the config tree under `set`, never as an operational RPC.
  → Constraint: the response payload is structured data routed through `ApplyPipes`; a rendered table MUST NOT be returned as the payload.
  → Constraint: JSON keys are lowercase kebab-case matching the YANG leaf.
- [ ] `ai/rules/config.md` and `ai/patterns/config-option.md` - the config surface
  → Constraint: an operator-set value that must appear in `show configuration` and in a backup is a YANG leaf; env-only is reserved for debug, bootstrap and safety caps.
  → Constraint: toggles are positive `enabled` booleans; no `type empty`, no enable/disable enumeration.
  → Constraint: a dimensioned value carries a `units` statement and the leaf name stays unit-free, so the reservation leaf is `reserve` with `units megabytes`, never `reserve-mb`.
  → Constraint: containers take a singular noun with no `-config` or `-settings` suffix.
- [ ] `ai/rules/architecture.md` - placement and state
  → Constraint: placement follows dependency direction. A config-driven engine nothing else depends on is an edge plugin, which is where `internal/plugins/crashes` already sits.
  → Constraint: runtime state persists through `statestore`, but reads and writes to `/proc`, `/sys` and `/dev` for kernel and device control are explicitly exempt, which covers `/sys/fs/pstore` and `/proc/vmcore`.
- [ ] `ai/rules/repo-maintenance.md` - discovery
  → Constraint: a new runtime dependency owes a registered `ze doctor` check with a diagnostic code. A reserved region, a mounted pstore filesystem and a writable crash directory are three such dependencies.
- [ ] `ai/rules/platform-linux.md` - appliance and kernel work
  → Constraint: kernel-device behavior is proven in QEMU, not asserted from config symbols.

### Related Specs
- [ ] `plan/spec-kernel-lockdown-hardening.md` (status: design, not scheduled)
  → Constraint: lockdown integrity mode blocks UNSIGNED kexec. Its C-1 correction establishes that cross-build kexec needs a stable, long-lived image-signing key, and it deliberately leaves adopting that key as a cost the owner may decline. Phase 1 avoids this entirely by using no kexec. Phase 2 inherits it and MUST NOT adopt the key on its own authority.
  → Constraint: its C-2 note records that gokrazy `reboot.go` never kexecs on `!amd64`, so Phase 2 is amd64-only. Phase 1 has no such limit.
- [ ] `plan/spec-hardware-watchdog.md` (status: design)
  → Decision: the watchdog covers the HANG case by rebooting a wedged box; crash capture covers the PANIC case by recording before the reboot. They are complements and both declare kernel requirements through `internal/appliance/kernelreq.go`.
  → Constraint: a watchdog reboot is not a panic and MUST NOT produce a crash artifact, or every wedge looks like a kernel fault.

**Key insights:** (minimal context to resume after compaction)
- The userspace crash path is DONE. Only the kernel side is missing.
- `CONFIG_KEXEC_FILE=y` is already set for gokrazy OTA. `CONFIG_PSTORE`, `CONFIG_PSTORE_RAM`, `CONFIG_CRASH_DUMP` and `CONFIG_PROC_VMCORE` are all absent from the 107-line runtime kernel config.
- Phase 1 costs megabytes and no kexec; Phase 2 costs gigabytes, capture-kernel code and the lockdown collision. Phase 1 removes four of the seven risks Phase 2 carries.
- `show crashes` is the extension point, and a `kind` field distinguishes the artifacts.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/core/crashlog/crashlog.go` - `Init()` runs first in `main()`, resolves the crash directory by probe order, parses a retention count, redirects stderr. Registers `ze.crash.dir` and `ze.crash.keep` (default 5).
- [ ] `internal/core/crashlog/list.go` - `ListCrashes()`, `ReadCrash()`, `CrashDir()`: the listing API the CLI and the support bundle both read.
- [ ] `internal/core/crashlog/persist.go` - crash file persistence and rotation against the retention count.
- [ ] `internal/core/crashlog/stderr.go` - stderr redirect and line-by-line syslog forwarding.
- [ ] `internal/plugins/crashes/register.go` - registers `show crashes` as an OFFLINE FALLBACK, never a plain local, so it does not shadow the daemon command while the daemon is up.
- [ ] `internal/plugins/crashes/yang/ze-crashes-cmd.yang` - the `show crashes` command tree, `config false`, owned by the plugin for self-containment. Carries a `name` leaf for selecting one report.
- [ ] `internal/plugins/crashes/cmd/show.go` - the daemon-served RPC half.
- [ ] `internal/component/support/modules.go` - `moduleRegistry` is the single source of truth for `ze support` modules; help text, `--list-modules`, validation and collection all derive from it. A `crashes` entry is already registered.
- [ ] `internal/component/support/support.go` - `collectCrashes` returns count, directory and full content per crash file.
- [ ] `internal/component/support/disk_unix.go` - `collectDiskInfo` already does `Statfs` and reports total, free, used and used-percent.
- [ ] `internal/appliance/kernelreq.go` - `universalKernelRequirements` and `runtimeKernelRequirements` are Go-verified `CONFIG_` floors. The `CONFIG_INET_ESP` comment records exactly the failure this catches: the kernel accepts the install, the feature is silently dead, and the image has no busybox to diagnose it with.
- [ ] `gokrazy/kernel/kernel.config` - 107 lines. `CONFIG_KEXEC_FILE=y` at line 98. No pstore, ramoops, crash-dump or vmcore symbol anywhere.
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - the `system` config container, holding `host`, `domain`, `dns`, `peeringdb` and `tuning`. `tuning` is the nearest precedent: hardware tuning, Linux only, applied at startup and on commit.
- [ ] `internal/appliance/cmd_build.go` - GPT image; `findLastPartition` then resize, so `/perm` is the last partition grown to fill the disk. ze owns the partition table, which is why a block-device backend was considered before being rejected on panic-write driver support.
- [ ] `internal/appliance/kernelargs.go` - declares itself the single kernel-argument assembly seam. `hugepageKernelArgs` reserves hugepages by SIZE from the appliance config, with nil meaning the built image's `/cmdline.txt` is unchanged. This is the precedent the crash reservation follows.
- [ ] `internal/appliance/kernel.version` - pins kernel 7.2, which is what makes size-named memory reservation available and settles A-2.

**Behavior to preserve:**
- `show crashes`, `show crashes latest` and `show crashes name <file>` keep their current output for userspace crash files, including the daemon-served path and the in-process fallback used when the daemon has died.
- The `crashes` support module keeps its existing keys for userspace crash files.
- The crash directory probe order and `ze.crash.keep` retention semantics are unchanged.
- gokrazy OTA continues to kexec normally on update. Phase 2 staging MUST NOT disturb the normal-reboot kexec path.
- A watchdog-initiated reboot produces no crash artifact.

**Behavior to change:**
- `show crashes` gains kernel panic records as a second artifact kind, and gains a readiness report.
- The `crashes` support module gains kernel artifact metadata and readiness.
- The runtime kernel gains pstore symbols in Phase 1 and crash-dump symbols in Phase 2, each with a requirement-floor row.
- A new `system crash-dump` config container.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Four entries, which do not share a trigger:
- Config commit records intent and reports that a reboot is owed, because a memory reservation is a boot argument.
- Image build writes the reservation onto the appliance kernel command line.
- Boot: the daemon reads whether the running kernel actually carries the reservation, harvests any record left by the previous boot, and records readiness.
- Kernel panic: the kernel writes its own record into the reserved region. No ze code runs at this instant.

### Transformation Path
1. Commit stores the `system crash-dump` subtree and answers that the setting is configured but not yet armed.
2. The appliance build renders the reservation onto the kernel command line for the next image.
3. At boot the daemon compares configured intent against the running kernel's actual reservation, producing the configured-versus-armed distinction.
4. On panic the kernel writes the message and backtrace into the reserved region, which is not cleared by a warm reboot.
5. At the next boot the daemon reads the record from the pstore filesystem, writes it into the crash directory as a kernel-kind artifact with a context block, and clears the source region so the next panic has somewhere to go.
6. The artifact is listed by the existing crash listing and carried by the existing support module.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon ↔ kernel | reads the reservation state and the pstore filesystem, clears consumed records | No |
| Build ↔ kernel command line | reservation rendered into the appliance cmdline | No |
| Build ↔ kernel image | `CONFIG_` floor verified against the built config | No |
| Daemon ↔ CLI | existing `show crashes` RPC and its offline fallback | No |
| Daemon ↔ support bundle | existing `crashes` module in `moduleRegistry` | No |
| Capture kernel ↔ storage (Phase 2 only) | writer reads `/proc/vmcore`, writes the resolved target | No |

### Integration Points
- `internal/core/crashlog` listing API - kernel artifacts enumerate through the same call, not a parallel lister.
- `internal/plugins/crashes` - owns the `show crashes` noun and gains the readiness answer.
- `internal/component/support` `moduleRegistry` - the `crashes` module gains kernel metadata; no new module name.
- `internal/appliance/kernelreq.go` - the new `CONFIG_` floor rows.
- `internal/component/config/system/yang/ze-system-conf.yang` - the new config container.
- `internal/core/diagnostic/codes.go` - diagnostic codes for the doctor checks.

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
| A-1 | The userspace crash path needs no change; only the kernel side is missing | `docs/architecture/diagnostics/crash-capture.md`; `internal/core/crashlog/*`; `show crashes` in `docs/guide/command-reference.md` | Scope is wrong and the spec duplicates shipped behavior | Owner confirmed at the RESEARCH gate, 2026-09-02 | confirmed |
| A-2 | A ramoops region can be reserved by SIZE, with no per-machine physical address | `internal/appliance/kernel.version` pins 7.2. Size-based named reservation (`reserve_mem`, bound by `ramoops.mem_name`) has been in the kernel since 6.12, so the address problem that made ramoops awkward on x86 does not apply at this version. It mirrors the hugepage precedent in `internal/appliance/kernelargs.go`, which also reserves by size | Phase 1 would need a per-machine address and the design falls back to the EFI-variable backend (see Key Design Decisions) | QEMU lab asserting the named region is reserved and ramoops binds to it; the version floor is asserted by the kernel requirement check | unvalidated |
| A-3 | The reserved region survives a warm reboot on the target firmware | pstore's ramoops backend depends on RAM contents persisting across a warm reset, which firmware may defeat by training or zeroing memory | Phase 1 captures nothing on real hardware while passing in QEMU | QEMU lab first, then one run on the N100 reference machine before the feature is documented as supported | unvalidated |
| A-4 | Adding pstore symbols is the whole Phase 1 kernel-config delta | `gokrazy/kernel/kernel.config` carries no pstore symbol in its 107 lines | The floor row is incomplete and the feature is silently dead, exactly the `CONFIG_INET_ESP` failure | Build the kernel with the symbols added and assert the floor check passes against the built config | unvalidated |
| A-5 | `/perm` is writable early enough in boot for the harvest to run | `docs/guide/appliance.md`: `/perm` is ext4 and the only writable store; the crash directory probe already prefers `/perm/ze/crash` | The harvest silently drops records, or writes to `/tmp` and loses them on the next reboot | QEMU lab asserting the artifact is present after the reboot, in the probed directory | unvalidated |
| A-6 | Phase 2 is amd64-only | `plan/spec-kernel-lockdown-hardening.md` C-2: gokrazy `reboot.go` never kexecs on `!amd64` | arm64 appliances accept a config that silently captures nothing | Assert the memory-image leaf is refused with a reason on a non-amd64 build | unvalidated |
| A-7 | Operators accept a full memory image sized to RAM when they opt into Phase 2 | Owner statement during design: full dumps accepted, gated on available space | Phase 2 should filter by default rather than capture everything | Owner confirmation before Phase 2 starts | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Lockdown integrity mode blocks unsigned kexec, so Phase 2 cannot stage a capture kernel on a hardened profile | The staging call is refused with a permission error on a lockdown build | Phase 1 is unaffected because it uses no kexec. For Phase 2, either sign the capture kernel with the stable image key in `plan/spec-kernel-lockdown-hardening.md` C-1, or ship the hardened profile without the memory image and say so on the profile page. This spec does NOT adopt that key |
| R-2 | A Phase 2 memory image does not fit: sized by RAM, written to a `/perm` that may be far smaller | Readiness reports insufficient space before any panic | Check free space against the estimate plus a reserve at commit, at boot, and again before the write opens. Report not-armed with the shortfall rather than failing at panic time |
| R-3 | Writing a multi-gigabyte image extends the outage while the router is down | Write duration measured in the QEMU lab | Phase 2 is opt-in and off by default; document the downtime cost on the guide page. Never begin a write that cannot complete |
| R-4 | Reserved memory costs RAM on every boot to serve a rare panic | Available memory drops after enabling | Phase 1 reserves megabytes and defaults off; the reservation is a leaf so it can be tuned down. Phase 2's far larger reservation is a separate opt-in |
| R-5 | Phase 2 staging disturbs the normal-reboot kexec path gokrazy uses for OTA | An OTA update fails to reboot after the feature lands | Crash staging targets the reserved region through the crash-entry path, a separate slot from the normal kexec image. A QEMU lab asserts an OTA-style reboot still works with crash staging active |
| R-6 | A truncated artifact reads as a complete one and misleads the analysis | Recorded expected size does not match actual size | Record both; never truncate to fit. Skip with the reason recorded instead |
| R-7 | Phase 2 runs code in the capture kernel, the least debuggable place, where a bug costs the reboot as well as the image | A box that panics and then does not come back | The writer does the minimum: read, write, reboot. It reboots unconditionally on every error path, so a failed capture still restores service. Phase 1 has no code on this path at all |
| R-8 | A stale record is harvested twice and looks like a second panic | The same backtrace appears under two timestamps | The harvest clears the source region after a successful write, and the artifact carries the boot identifier it was recovered from |
| R-9 | The harvest runs before the crash directory is writable and drops the only copy of the record | Readiness reports the directory unwritable while a record is pending | The harvest does not clear the source region unless the write succeeded, so a failed harvest retries on the next boot |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Phase 1: a small amount of RAM is reserved and nothing is captured, which is the status quo plus a few megabytes. Phase 2 worst case is a box that panics and then fails to reboot, which is worse than no capture at all. No routing, session or forwarding behavior is touched while the machine is healthy |
| How is it reverted? | Single commit revert, plus removing the config and one reboot; a reservation is only live after a boot that carries it |
| Who else touches this path? | `plan/spec-kernel-lockdown-hardening.md` (kexec signing, Phase 2 only), `plan/spec-hardware-watchdog.md` (kernel requirement floor, reboot on hang), `internal/plugins/crashes` and `internal/component/support` (the surfaces extended) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set system crash-dump enabled true` committed | → | config subtree stored and reported as configured | `TestCrashDumpConfigCommitReportsRebootOwed` |
| Daemon start with a record present in pstore | → | harvest writes a kernel-kind artifact into the crash directory | `TestHarvestWritesKernelArtifact` |
| `ze show crashes` | → | listing returns userspace and kernel artifacts with a kind field | `TestShowCrashesListsKernelKind` |
| `ze show crashes` with no daemon running | → | offline fallback returns the same artifacts | `TestOfflineShowCrashesListsKernelKind` |
| `ze support --module crashes` | → | `collectCrashes` returns kernel metadata and readiness | `TestSupportCrashesModuleCarriesKernelReadiness` |
| `ze doctor` | → | the three crash-capture checks run and report | `TestDoctorReportsCrashCaptureReadiness` |
| Appliance image build | → | reservation rendered onto the kernel command line | `TestApplianceBuildRendersCrashReservation` |
| Built runtime kernel | → | `CONFIG_` floor check | `TestRuntimeKernelRequirementsIncludePstore` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `system crash-dump enabled true` committed on a kernel whose running command line has no reservation | Commit succeeds; readiness reports configured true, armed false, and a reason naming the reboot that is owed |
| AC-2 | Same setting, on a kernel booted with the reservation present | Readiness reports configured true and armed true |
| AC-3 | Kernel panic, then a warm reboot | A kernel-kind artifact appears in the crash directory carrying the panic message and backtrace |
| AC-4 | `ze show crashes` after AC-3 | The kernel artifact is listed alongside userspace crash files, each carrying a kind field distinguishing them |
| AC-5 | `ze show crashes name <kernel artifact>` | The panic text is printed in full |
| AC-6 | Same harvest runs twice with no new panic | Exactly one artifact exists; the second harvest finds nothing to do |
| AC-7 | The crash directory is not writable when a record is pending | The record is left in place, readiness reports the directory unwritable, and the next boot retries the harvest |
| AC-8 | `ze support` with default modules | The `crashes` module payload carries kernel artifacts and the readiness block |
| AC-9 | `ze doctor` on a machine with the feature enabled but no reservation | A check fails with a registered diagnostic code naming the missing reservation |
| AC-10 | The runtime kernel is built without the pstore symbols | The kernel requirement floor check fails the build |
| AC-11 | A watchdog-initiated reboot | No kernel artifact is produced |
| AC-12 | `system crash-dump memory-image enabled true` on a non-amd64 build | The config is refused with a reason naming the architecture limit |
| AC-13 | `system crash-dump memory-image enabled true` where free space is below the estimate plus reserve | Readiness reports armed false with the shortfall in bytes; no capture is attempted |
| AC-14 | Every command in this feature run through `| json` | The payload renders as structured data with kebab-case keys |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables crash capture and reboots to arm it | commit → cmdline reservation → boot → readiness armed | `TestCrashDumpArmsAfterReboot` |
| 2 | Suffers a kernel panic and reads the backtrace after the box comes back | panic → reserved region → boot harvest → crash directory → `show crashes name` | `qemu-crash-capture-panic-harvest` |
| 3 | Sends a support bundle after a kernel panic | panic → harvest → `crashes` module → archive | `TestSupportArchiveContainsKernelArtifact` |
| 4 | Checks why capture is not working | `ze doctor` → three checks → diagnostic codes | `TestDoctorReportsCrashCaptureReadiness` |
| 5 | Inspects a crash with the daemon down | offline fallback → crash directory → listing | `TestOfflineShowCrashesListsKernelKind` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHarvestWritesKernelArtifact` | `internal/core/crashlog/kernel_test.go` | A pending record becomes an artifact in the probed directory | |
| `TestHarvestClearsSourceAfterSuccess` | `internal/core/crashlog/kernel_test.go` | R-8: the source region is cleared only after a successful write | |
| `TestHarvestRetriesWhenDirectoryUnwritable` | `internal/core/crashlog/kernel_test.go` | R-9 and AC-7: a failed write leaves the record in place | |
| `TestListCrashesCarriesKind` | `internal/core/crashlog/list_test.go` | AC-4: listing distinguishes userspace from kernel artifacts | |
| `TestKernelArtifactRetentionSharesKeep` | `internal/core/crashlog/persist_test.go` | Kernel artifacts rotate under the existing `ze.crash.keep` | |
| `TestReadinessConfiguredVersusArmed` | `internal/plugins/crashes/readiness_test.go` | AC-1 and AC-2: the two states are reported separately | |
| `TestReadinessReportsSpaceShortfall` | `internal/plugins/crashes/readiness_test.go` | AC-13: shortfall is reported in bytes | |
| `TestMemoryImageRefusedOnNonAmd64` | `internal/component/config/system/validate_test.go` | AC-12 and A-6 | |
| `TestRuntimeKernelRequirementsIncludePstore` | `internal/appliance/kernelreq_test.go` | AC-10 and A-4 | |
| `TestSupportCrashesModuleCarriesKernelReadiness` | `internal/component/support/support_test.go` | AC-8 | |
| `TestDoctorReportsCrashCaptureReadiness` | `internal/plugins/crashes/doctor_test.go` | AC-9 | |
| `TestWatchdogRebootProducesNoArtifact` | `internal/core/crashlog/kernel_test.go` | AC-11 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `system crash-dump reserve` (megabytes) | 4-256 | 256 | 3 | 257 |
| `system crash-dump memory-image reserve` (megabytes) | 64-1024 | 1024 | 63 | 1025 |
| `ze.crash.keep` (existing, unchanged) | 1-100 | 100 | 0 | 101 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `crash-dump-config-parse` | `test/parse/crash-dump-config-parse.ci` | The config subtree parses and round-trips | |
| `crash-dump-reserve-bounds` | `test/parse/crash-dump-reserve-bounds.ci` | Out-of-range reservations are refused with the bound named | |
| `cli-crashes-kernel-kind` | `test/ui/cli-crashes-kernel-kind.ci` | `show crashes` lists a kernel artifact with its kind | |
| `cli-crashes-readiness` | `test/ui/cli-crashes-readiness.ci` | Readiness reports configured versus armed | |
| `support-crashes-kernel` | `test/plugin/support-crashes-kernel.ci` | The support archive carries the kernel artifact | |
| `appliance-crash-reservation` | `test/appliance/appliance-crash-reservation.ci` | The build renders the reservation onto the kernel command line | |

### QEMU Tests (Linux kernel behavior)
| Test | Registration | What it proves | Status |
|------|--------------|----------------|--------|
| `qemu-crash-capture-panic-harvest` | `internal/le/qemu/actions.go`, runtime-kernel labs | A-2, A-3, A-5, AC-3: reserve, panic, warm reboot, artifact present in the probed directory | |
| `qemu-crash-capture-ota-unaffected` | `internal/le/qemu/actions.go` | R-5: an OTA-style kexec reboot still works with crash capture active | |

### Interop Tests (Scope: protocol)
N-A. This feature is not protocol-implementing and changes no wire-visible behavior.

## Files to Modify
- `gokrazy/kernel/kernel.config` - Phase 1 pstore symbols; Phase 2 crash-dump symbols
- `internal/appliance/kernelreq.go` - `runtimeKernelRequirements` floor rows, each with a comment naming the silent-failure mode
- `internal/appliance/kernelargs.go` - the reservation tokens. This file declares itself the single kernel-argument assembly seam, and hugepage reservation is the existing precedent: a size in `appliance.json`, tokens rendered onto the cmdline, nil meaning no reservation and `/cmdline.txt` left unchanged
- `internal/appliance/config.go` - the appliance-config field carrying the reservation, alongside the hugepage field
- `internal/component/config/system/yang/ze-system-conf.yang` - the `crash-dump` container
- `internal/core/crashlog/list.go` - listing gains the kind field
- `internal/core/crashlog/persist.go` - retention covers kernel artifacts
- `internal/plugins/crashes/yang/ze-crashes-cmd.yang` - readiness on the `show crashes` node
- `internal/plugins/crashes/cmd/show.go` - daemon-served readiness and kind
- `internal/plugins/crashes/crashes.go` - offline path returns the same shape
- `internal/component/support/support.go` - `collectCrashes` carries kernel metadata and readiness
- `internal/core/diagnostic/codes.go` - three diagnostic codes
- `internal/le/qemu/actions.go` - register the two QEMU labs
- `docs/architecture/diagnostics/crash-capture.md` - the kernel half, with source anchors
- `docs/architecture/testing/qemu-integration.md` - its registered-action anchor enumerates the actions by name, so two new labs make it stale
- `ai/INDEX.md` - discovery row for the feature

## Files to Create
- `internal/core/crashlog/kernel.go` - harvest: read a pending record, write the artifact, clear the source
- `internal/core/crashlog/kernel_linux.go` - the Linux-only pstore reader
- `internal/core/crashlog/kernel_other.go` - the non-Linux stub
- `internal/plugins/crashes/readiness.go` - configured versus armed, reservation, target, space
- `internal/plugins/crashes/doctor.go` - the three registered checks
- `test/parse/crash-dump-config-parse.ci`
- `test/parse/crash-dump-reserve-bounds.ci`
- `test/ui/cli-crashes-kernel-kind.ci`
- `test/ui/cli-crashes-readiness.ci`
- `test/plugin/support-crashes-kernel.ci`
- `test/appliance/appliance-crash-reservation.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/system/yang/ze-system-conf.yang` for the config; `internal/plugins/crashes/yang/ze-crashes-cmd.yang` for readiness on the show node |
| YANG validation constraints | Yes | `range` on both reserve leaves; `boolean` on both enabled leaves; `units megabytes` on both reserve leaves |
| YANG custom validators | Yes | `ze:validate` on the memory-image enable, refusing non-amd64 (AC-12) and refusing a reservation the target cannot hold (AC-13) |
| CLI commands/flags | Yes | No new command. `show crashes` gains fields; `internal/plugins/crashes/cmd/show.go` |
| CLI grammar (keyword before value) | Yes | No new grammar: `show crashes name <file>` already places the keyword before the value and is unchanged |
| Editor autocomplete | Yes | Automatic for the boolean and range leaves; no dynamic values need a `CompleteFn` |
| Functional test for new RPC/API | Yes | `test/plugin/support-crashes-kernel.ci`, `test/ui/cli-crashes-readiness.ci` |
| Pipe completeness | Yes | Readiness and listing are structured payloads through `ApplyPipes`; AC-14 covers rendering |
| Env var registration | N-A | No leaf under `environment/`. The existing `ze.crash.dir` and `ze.crash.keep` are reused unchanged and are already registered |
| Doctor check for runtime dependencies | Yes | `internal/plugins/crashes/doctor.go` plus three codes in `internal/core/diagnostic/codes.go`: reservation missing, pstore unavailable, crash directory unwritable |
| Prometheus counters/metrics | Yes | One gauge for artifacts present and one for armed state, so a fleet can alert on capture silently unarmed |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, the `show crashes` entry |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, the crashes plugin entry |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` gains the capture and reservation section |
| 7 | Wire format changed? | N-A | No wire format touched |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK or process-protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Not protocol work; no RFC requirement bound |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, the two new QEMU labs |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: VyOS and FRR crash-artifact behavior |
| 12 | Internal architecture changed? | Yes | `docs/architecture/diagnostics/crash-capture.md` gains the kernel half |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | The subsystem telemetry doc, for the two gauges |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/features/plugins.md` and `docs/guide/status.md` for the doctor checks |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED by `./le spec citation anchors spec plan/spec-crash-capture.md`, run 2026-09-02. **Declared (blocking), named as unaffected:** `docs/architecture/core-design.md` (declared by `crashes/cmd/show.go`, `support/support.go`, `le/qemu/actions.go`) describes the module registry and system data flow; this feature adds fields to the existing `crashes` module and registers no new module name, so the registry design is unchanged. `docs/features/ai-first.md` (declared by `core/diagnostic/codes.go`) describes the stable-diagnostic-code contract for agents; three new codes follow that scheme and change no contract. **Updated:** `docs/architecture/diagnostics/crash-capture.md`, `docs/architecture/testing/qemu-integration.md` (its anchor enumerates registered actions by name), `docs/architecture/appliance/builder.md` (declared by `internal/appliance/config.go`, which gains a reservation field). **Declared, named as unaffected:** `docs/architecture/vpp-host-tuning.md` (declared by `internal/appliance/kernelargs.go`) documents hugepage reservation for VPP; this feature adds a second, independent token at the same assembly seam and changes no VPP tuning behavior, so the page stays accurate. `docs/guide/vpp.md` mentions the same file for the same reason and is likewise unaffected. **Advisory, unaffected:** `docs/architecture/appliance/gokrazy-build-pins.md`, `docs/architecture/appliance/installer-initrd.md`, `docs/architecture/appliance/iso-installer.md`, `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` and `docs/guide/developer-setup.md` mention `le/qemu/actions.go` but describe unrelated labs, and adding two actions leaves their prose accurate; `docs/guide/ze-install.md` anchors `enforceKernelRequirements` but describes the mechanism ("rejects any required symbol that is not built in") without enumerating symbols, so new floor rows do not make it stale. Re-run the command at closure |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify the `show crashes` examples in `docs/guide/command-reference.md` still match after the payload gains a kind field |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: every row of the Wiring Test table, all failing against stubs
   - Files: `internal/core/crashlog/kernel.go` stub, `internal/plugins/crashes/readiness.go` stub, `internal/plugins/crashes/doctor.go` registration, the three diagnostic codes
   - Verify: each entry point is reachable and each wiring test fails because the feature is a stub, not because it is unregistered
2. **Phase: Kernel floor** -- declare the capability before depending on it
   - Tests: `TestRuntimeKernelRequirementsIncludePstore`
   - Files: `gokrazy/kernel/kernel.config`, `internal/appliance/kernelreq.go`
   - Verify: a kernel built without the symbols fails the floor check, and one built with them passes (AC-10)
3. **Phase: Config surface** -- schema, bounds, validators
   - Tests: `TestMemoryImageRefusedOnNonAmd64`, `test/parse/crash-dump-config-parse.ci`, `test/parse/crash-dump-reserve-bounds.ci`
   - Files: `ze-system-conf.yang`, the custom validator and its registration
   - Verify: the subtree parses, bounds are enforced with the bound named, and the architecture refusal states its reason
4. **Phase: Reservation** -- render onto the appliance kernel command line
   - Tests: `TestApplianceBuildRendersCrashReservation`, `test/appliance/appliance-crash-reservation.ci`
   - Files: `internal/appliance/kernelargs.go`, `internal/appliance/config.go`
   - Verify: the reservation appears on the built command line and nowhere else. Follow the hugepage precedent in the same file: a value in the appliance config, tokens assembled at the one seam, and nil meaning `/cmdline.txt` is left unchanged
   - The reservation is size-named, never an address: a named region sized from the config leaf, with ramoops bound to it by name. No per-machine tuning, and the assembly reads like the hugepage tokens beside it
   - The kernel requirement check asserts the version floor that size-named reservation needs, so an older pinned kernel fails the build rather than silently reserving nothing
5. **Phase: Harvest** -- read the pending record, write the artifact, clear the source
   - Tests: `TestHarvestWritesKernelArtifact`, `TestHarvestClearsSourceAfterSuccess`, `TestHarvestRetriesWhenDirectoryUnwritable`, `TestWatchdogRebootProducesNoArtifact`
   - Files: `internal/core/crashlog/kernel.go`, `kernel_linux.go`, `kernel_other.go`, `list.go`, `persist.go`
   - Verify: AC-3, AC-6, AC-7 and AC-11 hold, and retention shares the existing keep count
6. **Phase: Surfaces** -- listing, readiness, support module, metrics
   - Tests: `TestListCrashesCarriesKind`, `TestReadinessConfiguredVersusArmed`, `TestReadinessReportsSpaceShortfall`, `TestSupportCrashesModuleCarriesKernelReadiness`, `TestDoctorReportsCrashCaptureReadiness`, the four `.ci` files
   - Files: `crashes.go`, `cmd/show.go`, `ze-crashes-cmd.yang`, `support.go`
   - Verify: AC-1, AC-2, AC-4, AC-5, AC-8, AC-9, AC-14 hold on both the daemon path and the offline fallback
7. **Phase: QEMU proof** -- the assumptions that only a booted kernel can settle
   - Tests: `qemu-crash-capture-panic-harvest`, `qemu-crash-capture-ota-unaffected`
   - Files: `internal/le/qemu/actions.go`
   - Verify: A-2, A-3 and A-5 move to confirmed or the design changes. STOP and report if the reserved region does not survive the reboot
8. **Phase: Phase 2, memory image (opt-in)** -- only after 1 to 7 are closed and A-7 is confirmed
   - Tests: `TestReadinessReportsSpaceShortfall` extended to the image estimate, `qemu-crash-capture-ota-unaffected` re-run with staging active
   - Files: crash-dump kernel symbols, the capture writer, the memory-image leaves
   - Verify: AC-12, AC-13, R-5 and R-7 hold. The writer reboots unconditionally on every error path

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Configured and armed are computed from different sources, never one derived from the other; the harvest clears the source only after a successful write |
| Naming | The YANG leaf, the Go field, the JSON key and the CLI token agree; `reserve` carries `units megabytes` and is not named `reserve-mb` |
| Data flow | The harvest reads the kernel and writes the crash directory; no other package learns what a pstore is |
| One noun | No `show kdump` or `show crash` was added beside `show crashes` |
| Rule: `ai/rules/cli.md` | Readiness and listing are structured payloads, not rendered text, and answer every pipe operator the catalog offers |
| Rule: `ai/rules/repo-maintenance.md` | Three runtime dependencies, three doctor checks, three registered diagnostic codes |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Kernel symbols present in the built config | The floor check in `TestRuntimeKernelRequirementsIncludePstore` |
| Config subtree parses and enforces bounds | `./le functional parse` over the two new `.ci` files |
| Kernel artifact reaches the crash directory | `./le qemu qemu-crash-capture-panic-harvest` |
| OTA reboot still works | `./le qemu qemu-crash-capture-ota-unaffected` |
| Support archive carries the artifact | `./le functional plugin` over `support-crashes-kernel.ci` |
| Doctor checks registered with codes | `ze doctor` output plus a grep of `internal/core/diagnostic/codes.go` for the three codes |
| No second noun added | `grep -rn "show kdump\|show crash\b" internal/` returns nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The pstore record is kernel-authored but arbitrary-length; the reader bounds what it copies and never sizes an allocation from an untrusted length field |
| Information disclosure | A kernel backtrace can contain pointers and memory contents. It follows the existing crash-file sanitization path and the `--sensitive` default in `ze support`, so a bundle redacts unless asked |
| Resource exhaustion | Artifacts share the existing retention count; a panic loop cannot fill `/perm` because rotation applies to kernel artifacts too |
| Error leakage | A readiness failure names what is missing and what to do, without printing kernel addresses to an unauthenticated surface |
| Authorization fails closed | The memory-image opt-in is refused, with a reason, when the architecture or the space precondition does not hold |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| QEMU lab shows the region does not survive reboot | STOP. A-3 is broken; the mechanism must change before more code is written |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The expensive half of a crash-dump feature is not the dump, it is the code that runs after the machine has already failed. Phase 1 has none: the kernel writes the record itself and ze only reads it on the next healthy boot. That is why it removes four risks rather than mitigating them.
- `CONFIG_KEXEC_FILE=y` is already set for OTA, so the kexec half of a full kdump is paid for. The remaining cost of Phase 2 is entirely the reservation, the write and the capture-kernel code, which is what makes deferring it cheap.
- Configured versus armed is the state operators get wrong, because a reservation is a boot argument and a commit looks like it took effect. Reporting them as separate fields from separate sources is the single most useful thing this feature does.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| pstore/ramoops first, full memory image behind an opt-in | Full kdump only; ramoops only | A backtrace names the faulting subsystem, which is what a bug report needs, at megabytes instead of gigabytes and with no code on the reboot path. The memory image stays available for the fault a backtrace does not explain |
| Kernel artifacts are ordinary entries in `show crashes` | A separate `show kernel-dumps` noun with its own store | `ai/rules/cli.md`: one concept carries one name. Reuses the directory probe, the retention count, the support module and the offline fallback, all of which would otherwise be duplicated |
| Reservation is a YANG leaf, not an env var | Env-only knob | `ai/patterns/config-option.md` decision table: an operator sets it during capacity planning, it must appear in `show configuration` and in a backup, and it needs validation and rollback |
| The spec does not adopt the stable image-signing key | Making Phase 2 lockdown-compatible from the start | `plan/spec-kernel-lockdown-hardening.md` deliberately leaves that key as a cost the owner may decline. R-1 records the collision without pre-committing the decision |
| Harvest clears the source only after a successful write | Clear on read | R-9: clearing on read loses the only copy when the crash directory is not yet writable, which is exactly the early-boot case this runs in |
| pstore backend: ramoops over a size-named reserved region | EFI-variable backend; block-device backend; ACPI ERST; netconsole | ramoops keeps the record local, needs no firmware storage and has capacity for a full backtrace. The size-named reservation removes its one awkward requirement on x86, an explicit physical address, so it now mirrors the hugepage reservation this repository already does. **EFI variables** need no reservation and work on any EFI machine, but capacity is tens of kilobytes, writes wear firmware NVRAM the operator cannot service, and kernels ship the backend disabled by default because filling NVRAM has bricked machines. **Block device** is tempting because ze owns the GPT and could add a partition, but it needs a driver with panic-context write support, which is not universal across the NVMe and AHCI parts an appliance may boot from. **ACPI ERST** is firmware-provided and typical on servers, absent on the consumer mini-PCs this targets. **netconsole** is complementary rather than an alternative: it needs the network up and does not survive a panic in the network path, but it is worth offering later for operators who already run a collector |

## Known Limitations
- Phase 2 is amd64-only, because gokrazy `reboot.go` never kexecs on other architectures. arm64 appliances get Phase 1 only.
- Phase 2 is incompatible with kernel lockdown integrity mode unless the capture kernel is signed. That decision belongs to `plan/spec-kernel-lockdown-hardening.md`.
- A backtrace without kernel debug symbols names the subsystem but not the line. The symbols exist because ze builds the kernel, but pairing an artifact with its vmlinux is a manual step and is not automated here.
- Crash capture on a non-appliance Linux host is out of scope: the distro owns `crashkernel` and its own kdump tooling there, and ze reports readiness rather than configuring it.

## RFC Documentation (Scope: protocol)

N-A. Not protocol work.

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
- [ ] AC-1..AC-14 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
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
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
