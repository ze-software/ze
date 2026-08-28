# Spec: hardware-watchdog

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `ai/rules/platform-linux.md` - `/dev/watchdog` is a kernel device; QEMU tests mandatory
4. `internal/appliance/kernelreq.go` - the kernel requirement floor
5. `internal/component/host/doc.go` - the host layer (read-only today)

## Task

Ze ships as a gokrazy appliance and owns its own process lifecycle. gokrazy
restarts the ze process if it *crashes* (exits), but a process that *hangs*
(deadlock, livelock, wedged kernel path) never exits and is never restarted, so
the box stays dark until someone power-cycles it. A hardware watchdog closes
that gap: the kernel `/dev/watchdog` device reboots the machine unless it is
periodically written to ("petted") within a configured timeout, so a hung
system self-recovers.

Ze has no hardware watchdog today: nothing opens `/dev/watchdog`, the kernel
requirement floor does not even mandate a watchdog driver, and the only thing
named "watchdog" in the tree is an unrelated BGP route-management plugin.

Add a hardware watchdog:
- A config surface to enable it and set the timeout.
- A host-layer component that opens `/dev/watchdog`, sets the timeout, and pets
  it on a timer at a safe fraction of the timeout, armed from the daemon
  lifecycle.
- Extend the kernel requirement floor to guarantee a watchdog driver is present.
- A doctor check for device availability.

## Required Reading

### Architecture Docs
- [ ] `internal/appliance/kernelreq.go` - the runtime kernel requirement floor (`runtimeKernelRequirements`).
  -> Constraint: add the watchdog kernel option(s) to the floor so the built kernel actually exposes `/dev/watchdog`.
- [ ] `internal/component/host/doc.go` - the host layer is documented as read-only, stateless inventory.
  -> Decision: the watchdog is stateful (owns a device fd + a pet timer), so it is a NEW sibling component, not an addition to the read-only host inventory.
- [ ] `ai/rules/platform-linux.md` - device behaviour is kernel-level.
  -> Constraint: the watchdog arm/pet/reboot behaviour is validated in QEMU, never skipped for "needs hardware".
- [ ] `ai/rules/config.md`, `ai/rules/config.md` - YANG vs env var, kebab-case.
  -> Constraint: enable + timeout are YANG leaves under a system container.

**Key insights:**
- gokrazy handles crash-restart; the watchdog handles hang-reboot. They are complementary, not redundant.
- The pet interval must be well below the timeout (a safe fraction) so scheduling jitter never trips a false reboot.
- On a clean shutdown the watchdog must be disarmed (magic-close) so an orderly stop does not cause a reboot.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/appliance/kernelreq.go` - `runtimeKernelRequirements` (kernelreq.go) lists MODULES/PPP/PPPOE/L2TP options; there is NO `CONFIG_WATCHDOG` or `*_WDT` entry, so the built kernel is not guaranteed to expose `/dev/watchdog`. `enforceKernelRequirements` (kernelreq.go) checks the floor.
- [ ] `internal/component/host/doc.go` - the host package is "read-only, stateless" hardware inventory (doc.go top comment); it never opens a device fd or runs a timer.
- [ ] `internal/component/bgp/plugins/watchdog/register.go` - the only "watchdog" in the tree registers plugin `bgp-watchdog`, "Watchdog route management plugin" (register.go). This is BGP route health, NOT `/dev/watchdog`; it is unrelated and untouched.

**Behavior to preserve:**
- gokrazy crash-restart of the ze process is unchanged.
- The BGP `bgp-watchdog` route plugin is unaffected (different concept, different package).
- The host inventory stays read-only and stateless.

**Behavior to change:**
- When configured, ze opens `/dev/watchdog`, sets its timeout, and pets it on a timer for the daemon's lifetime; a clean shutdown disarms it.
- The kernel floor guarantees a watchdog driver so the device exists on the appliance.

## Data Flow (MANDATORY)

### Entry Point
- Config: a system `watchdog` container with `enabled` (boolean) and `timeout` (seconds), plus an optional device path.
- Runtime: the daemon lifecycle starts the watchdog component after config resolve.

### Transformation Path
1. Config resolve produces the watchdog settings (enabled, timeout, device).
2. On start, the component opens the watchdog device and issues the set-timeout ioctl, reading back the effective timeout.
3. A ticker pets the device at a safe fraction of the effective timeout (for example half), for the lifetime of the daemon.
4. On config change, the component reconfigures (new timeout) or stops (disarm).
5. On clean shutdown, the component writes the magic-close disarm and closes the fd so no reboot follows an orderly stop.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> watchdog component | enabled/timeout/device resolved into settings | [ ] |
| Component <-> kernel | open `/dev/watchdog`, set-timeout ioctl, periodic write (pet) | [ ] |
| Daemon lifecycle <-> component | start after resolve, stop with disarm on shutdown | [ ] |

### Integration Points
- New host-layer component (sibling to `internal/component/host/`) owning the fd + pet timer.
- `internal/appliance/kernelreq.go` - watchdog driver in the kernel floor.
- System YANG - the `watchdog` container.
- Doctor - device-presence / open check.

### Architectural Verification
- [ ] No bypassed layers (device access isolated in the one component)
- [ ] No unintended coupling (independent of host inventory and of the BGP watchdog plugin)
- [ ] No duplicated functionality (single pet loop; no second device opener)
- [ ] Registration over hardcoding - the component registers and is driven by config, not a hardcoded call site.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The appliance kernel can expose a watchdog driver (softdog at minimum) | Linux ships `softdog` + platform WDTs | no driver available on a target | add the option to the floor and boot in QEMU | unvalidated |
| A-2 | The set-timeout ioctl + periodic write pet model works on the softdog device in QEMU | standard Linux watchdog API | need a platform-specific device | QEMU functional test with softdog | unvalidated |
| A-3 | Magic-close disarm prevents a reboot on clean shutdown | Linux watchdog `nowayout=0` semantics | orderly stop causes a reboot | QEMU stop-without-reboot test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A too-tight timeout with scheduler jitter causes a false reboot | spurious reboots under load | pet at a safe fraction of the effective timeout; floor the timeout to a sane minimum |
| R-2 | `nowayout` builds prevent disarm, so a clean stop reboots | reboot on every config removal | detect/avoid `nowayout`; document; keep petting through a graceful restart window |
| R-3 | Hardware-only WDT unavailable in CI | QEMU lacks a platform WDT | test against `softdog` in QEMU |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `watchdog { enabled true; timeout 30 }` present | -> | component opens the device and sets the timeout | `TestWatchdogArmsAndSetsTimeout` |
| the daemon runs | -> | the device is petted on a timer | `test/qemu/hardware-watchdog.ci` |
| clean shutdown | -> | the device is disarmed, no reboot | `test/qemu/hardware-watchdog.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `enabled true`, timeout 30 | the device opens and the effective timeout is set |
| AC-2 | daemon running | the device is petted below the timeout; no reboot occurs |
| AC-3 | pets stop (simulated hang) | the system reboots after the timeout (proven in QEMU) |
| AC-4 | clean shutdown | the watchdog is disarmed; no reboot |
| AC-5 | `enabled false` / not configured | no device is opened (default off) |
| AC-6 | timeout below the safe minimum | config verify rejects or clamps to the minimum |
| AC-7 | kernel lacks a watchdog driver | doctor check reports it |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables the watchdog so a hung box self-recovers | config -> component -> `/dev/watchdog` pet loop -> reboot-on-hang | `test/qemu/hardware-watchdog.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWatchdogConfigParse` | (new component)`_test.go` | enabled/timeout/device parsed | |
| `TestWatchdogPetInterval` | (new component)`_test.go` | pet interval is a safe fraction of the timeout | |
| `TestKernelFloorRequiresWatchdog` | `internal/appliance/kernelreq_test.go` | the floor rejects a config without the watchdog option | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| timeout (s) | min..max (design cap) | max | below the safe minimum | above the device max |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `hardware-watchdog` | `test/qemu/hardware-watchdog.ci` | arm + pet keeps the box alive; stopping pets reboots it; clean stop does not | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - local kernel-device feature; validated by QEMU, not a network peer | - | - | watchdog is a host/kernel behaviour | - |

### Future (if deferring any tests)
- Platform-specific WDT drivers (beyond softdog) validated as hardware is added.

## Files to Modify
- `internal/appliance/kernelreq.go` - add the watchdog driver option(s) to the runtime floor
- System YANG (`internal/component/config/system/yang/ze-system-conf.yang`) - the `watchdog` container
- (daemon lifecycle) `cmd/ze/hub/` - start/stop the watchdog component

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |

## Files to Create
- `internal/component/watchdog/` - new host-layer component (device fd + pet loop + registration + doctor)
- `test/qemu/hardware-watchdog.ci` - QEMU functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - register the component + `watchdog` YANG (no-op) + a failing `test/qemu/hardware-watchdog.ci`.
2. **Phase: Kernel floor** - add the watchdog option to `runtimeKernelRequirements`.
   - Tests: `TestKernelFloorRequiresWatchdog`
3. **Phase: Device + pet loop** - open, set-timeout, pet at a safe fraction, disarm on stop.
   - Tests: `TestWatchdogConfigParse`, `TestWatchdogPetInterval`
4. **Functional (QEMU)** - arm/pet/reboot/clean-stop.
5. **Doctor** - device-presence check.
6. **Full verification** -> `./le verify current mode full`
7. **Complete spec** -> audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | pet below timeout; disarm on clean stop; default off |
| Registration over hardcoding | the component registers and is config-driven |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 demonstrated
- [ ] End-to-End User Stories: working path + passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify current mode full` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for the timeout
- [ ] Functional tests for end-to-end behavior (QEMU)

### Post-wave corrections (2026-07-10)

- New gate obligation: the followup wave added `ze-platform-vet` (`internal/le/` native action tables), which vets `./internal/component/host/...`, `./internal/component/iface/...`, and `./internal/plugins/iface/...` under GOOS=darwin and GOOS=freebsd; it runs in the live `./le verify current mode full` stage list in both branches (`internal/le/verify/run.go`, `:141`). The `/dev/watchdog` open/ioctl/pet code is Linux-only, so the new component MUST follow the `_linux.go`/`_other.go` split convention regardless (a macOS dev host builds the tree). Additionally, this spec plans the component as a sibling of `internal/component/host/` (Files to Create: `internal/component/watchdog/`), which is NOT in the gate's current package list -- the design must either extend the `ze-platform-vet` package list to the new tree or place the platform-split code under an already-vetted tree, so the non-Linux stubs cannot rot silently.
- `ze-system-conf.yang` (Files to Modify) was restructured by the wave: it gained resolver-related leaves including `dnssec-validation` (`internal/component/config/system/yang/ze-system-conf.yang`). No `watchdog` symbol exists anywhere in that file (grep 2026-07-10), so there is no conflict, but the planned `watchdog` container edit must rebase onto the current file layout at design time.
