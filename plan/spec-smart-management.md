# Spec: SMART disk health management

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-05-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/host/smart_linux.go` - existing ATA/NVMe ioctl read path (to be moved)
4. `internal/component/host/metrics.go` - ticker+stopCh goroutine pattern
5. `internal/core/report/report.go` - RaiseWarning/RaiseError API
6. `internal/component/cmd/show/host.go` - show RPC registration pattern

## Task

Add YANG-modeled SMART disk health management to Ze: auto-enable SMART on detected
devices, schedule periodic self-tests (short and long), monitor temperature with
three-tier alerting via the report bus, and expose live SMART status through an
online `show storage smart` RPC.

Move the SMART ioctl code from `internal/component/host/` to the new
`internal/component/storage/` component. The host package keeps the SmartInfo type
and StorageDevice struct; the storage component owns the ioctl implementation and
management logic. The host detector calls into the storage component for SMART reads.

No NOS vendor (VyOS, SONiC, Cumulus, Arista) has YANG-modeled SMART management.
Ze would be the first NOS with configurable SMART management.

### Industry Reference

| Platform | SMART Config | Self-Test Scheduling | Alerting |
|----------|-------------|---------------------|----------|
| VyOS | None (show only) | None | None |
| SONiC | Hardcoded | None | None |
| TrueNAS | Full (global + per-test tasks) | Cron-based | Email + threshold |
| smartd | Full (smartd.conf) | Regex schedule | Email + exec |
| **Ze (proposed)** | YANG-modeled | Interval + time-of-day | Report bus (3-tier temp) |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - registration pattern, component lifecycle
  -> Constraint: new component registers via init() in register.go
  -> Constraint: hub creates and starts component, calls Stop() on shutdown
- [ ] `ai/rules/goroutine-lifecycle.md` - long-lived workers only
  -> Constraint: ticker + stopCh pattern, no per-event goroutines
- [ ] `ai/rules/config-design.md` - YANG config conventions
  -> Constraint: fail on unknown keys, augment for system containers
- [ ] `ai/rules/plugin-design.md` - show RPC registration
  -> Constraint: pluginserver.RegisterRPCs() in init(), WireMethod naming

### RFC Summaries
N/A - no protocol work.

**Key insights:**
- `smart_linux.go` has ATA (`HDIO_DRIVE_CMD`) and NVMe (`NVME_IOCTL_ADMIN_CMD`) ioctl infrastructure. Write commands use same structs with different feature register values.
- `host/metrics.go` has the exact goroutine+ticker pattern: `StartRefresh(interval)` with `stopCh`, `Stop()` via `close(stopCh)`.
- `report.RaiseWarning(source, code, subject, message)` for alerting. Warnings are state-based (deduped). Errors are event-based (ring).
- `detectSMART` is only called from `storage_linux.go:66`. Moving the SMART files to the storage component requires updating that single call site.
- The `parseSMARTJSON` function in `smart.go` is dead code after the ioctl rewrite and can be removed during the move.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/host/smart_linux.go` - detectSMARTATA, detectSMARTNVMe, parseNVMeSMARTBuf, nvmeNamespace, isPermissionError, permissionDenied. Only caller: storage_linux.go:66
  -> Constraint: write ioctls use same HDIO_DRIVE_CMD with feature 0xD8 (enable), 0xD9 (disable), 0xD4 (self-test), 0xDB (auto-offline)
  -> Decision: move to internal/component/storage/smart_linux.go, export as package functions
- [ ] `internal/component/host/smart.go` - SmartInfo type, parseSMARTJSON (dead code after ioctl rewrite)
  -> Decision: SmartInfo moves to storage package. host.StorageDevice.Smart becomes *storage.SmartInfo. host imports storage (no circular dep). parseSMARTJSON removed.
- [ ] `internal/component/host/storage_linux.go` - calls d.detectSMART(name) at line 66
  -> Decision: after move, calls storage.DetectSMART(name) instead
- [ ] `internal/component/host/inventory.go` - StorageDevice struct with Smart *SmartInfo
- [ ] `internal/component/host/metrics.go` - StartRefresh(interval), Stop(), ticker+stopCh
  -> Constraint: same lifecycle pattern for storage manager
- [ ] `internal/core/report/report.go` - RaiseWarning(source, code, subject, message, detail...)

**Behavior to preserve:**
- `ze host show storage` continues to work (calls storage.DetectSMART instead of local method)
- SmartInfo struct and JSON format unchanged
- `ze support` host module includes SMART data as before

**Behavior to change:**
- SMART ioctl code moves from host to storage component
- New YANG config under `storage { smart { ... } }`
- New background scheduler goroutine for health polling and self-tests
- New `show storage smart` online RPC
- New report bus warnings for temperature/failure
- New `ze doctor` check for SMART status

## Data Flow (MANDATORY)

### Entry Point
- Config: YANG `storage { smart { ... } }` parsed at startup
- Runtime: hub creates `storage.Manager` from config, calls `manager.Start()`

### Transformation Path
1. YANG config parsed into config tree at startup
2. Hub extracts `storage.smart` subtree, builds `storage.Config` struct
3. `storage.Manager.Start()` spawns a ticker goroutine
4. Each tick: discover block devices via sysfs, read SMART via ioctl
5. Compare temperature against thresholds, raise warnings via report bus
6. Check self-test schedule, start test if due (ioctl write)
7. On shutdown: hub calls `manager.Stop()`, goroutine exits via stopCh

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Manager | Hub extracts config subtree, passes to NewManager() | [ ] |
| Manager -> ioctl | Calls storage.DetectSMART, EnableSMART, StartSelfTest | [ ] |
| Manager -> report bus | report.RaiseWarning() for threshold crossings | [ ] |
| Show RPC -> Manager | Handler calls manager.Status() for current state | [ ] |
| host -> storage | storage_linux.go calls storage.DetectSMART (replaces d.detectSMART) | [ ] |

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (storage imports host for types, host imports storage for detection)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `storage { smart { enabled true } }` config | -> | `storage.NewManager(config)` | `TestManagerCreatedFromConfig` |
| Hub startup | -> | `manager.Start()` spawns goroutine | `TestManagerStartStop` |
| Ticker fires | -> | health poll reads SMART, raises warnings | `TestHealthPollRaisesWarning` |
| Self-test interval elapsed | -> | `StartSelfTest()` ioctl | `TestSelfTestScheduling` |
| `show storage smart` RPC | -> | `manager.Status()` | `TestShowStorageSmart` |
| `ze doctor` | -> | SMART enabled check | `TestDoctorSmartEnabled` |
| `ze host show storage` | -> | `storage.DetectSMART(name)` | existing host tests |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `storage { smart { enabled true } }` | SMART auto-enabled on all detected ATA/NVMe devices at startup |
| AC-2 | Config `self-test { short { interval 24h } }` | Short self-test started on each device every 24 hours |
| AC-3 | Config `self-test { long { interval 7d; day sunday } }` | Long self-test started weekly on Sunday |
| AC-4 | Device temperature exceeds `informational` threshold | `report.RaiseWarning` emitted with source "storage", device name as subject |
| AC-5 | Device temperature exceeds `critical` threshold | `report.RaiseError` emitted |
| AC-6 | Temperature drops below threshold | Warning cleared |
| AC-7 | SMART health status changes to unhealthy | `report.RaiseError` with "smart-failing" code |
| AC-8 | `show storage smart` RPC | Returns JSON with per-device SMART status, temperature, hours, schedule |
| AC-9 | Self-test already in progress on device | Scheduler skips, does not start duplicate test |
| AC-10 | Device does not support SMART | Logged once, excluded from polling |
| AC-11 | `ze doctor` with SMART config | Verifies SMART enabled on detected devices |
| AC-12 | Daemon shutdown | Manager.Stop() called, goroutine exits cleanly |
| AC-13 | Config reload changes interval | Manager.Reconfigure() updates ticker |
| AC-14 | gokrazy (no smartctl) | Works identically (pure ioctl, no shell-outs) |
| AC-15 | `ze host show storage` after SMART move | Still works, SMART data present |

## YANG Schema

```
storage {
    smart {
        enabled true
        check-interval 1800
        temperature {
            difference 4
            informational 45
            critical 55
        }
        self-test {
            short {
                interval 24h
                time 02:00
            }
            long {
                interval 7d
                time 03:00
                day sunday
            }
        }
    }
}
```

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestManagerCreatedFromConfig` | `internal/component/storage/manager_test.go` | Config parsing | |
| `TestManagerStartStop` | `internal/component/storage/manager_test.go` | Goroutine lifecycle | |
| `TestHealthPollRaisesWarning` | `internal/component/storage/manager_test.go` | Temp threshold alert | |
| `TestHealthPollClearsWarning` | `internal/component/storage/manager_test.go` | Temp recovery | |
| `TestSelfTestScheduling` | `internal/component/storage/manager_test.go` | Interval triggers test | |
| `TestSelfTestSkipsInProgress` | `internal/component/storage/manager_test.go` | No duplicate tests | |
| `TestShowStorageSmart` | `internal/component/storage/manager_test.go` | Status() output | |
| `TestReconfigure` | `internal/component/storage/manager_test.go` | Live interval change | |
| `TestDetectSMART_MovedFromHost` | `internal/component/storage/smart_linux_test.go` | Read path still works after move | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| check-interval | 60..86400 | 86400 | 59 | 86401 |
| temperature difference | 1..50 | 50 | 0 | 51 |
| temperature informational | 1..100 | 100 | 0 | 101 |
| temperature critical | 1..100 | 100 | 0 | 101 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-smart-config` | `test/parse/smart-config.ci` | Config parses | |
| `test-smart-show` | `test/plugin/smart-show.ci` | Show RPC returns data | |

## Files to Modify
- `internal/component/host/storage_linux.go` - change detectSMART call to storage.DetectSMART
- `internal/component/host/smart.go` - remove parseSMARTJSON (dead code), keep SmartInfo type
- `internal/component/cmd/show/show.go` - register show storage smart RPC (or separate file)
- `cmd/ze/hub/main.go` - create and start storage.Manager
- `cmd/ze/doctor/doctor.go` - add checkSmartEnabled()

## Files to Create
- `internal/component/storage/schema/ze-storage-conf.yang` - YANG module
- `internal/component/storage/smart_linux.go` - moved from host, add write ioctls
- `internal/component/storage/smart_other.go` - non-Linux stubs
- `internal/component/storage/manager.go` - Manager, Start/Stop/Reconfigure, poll loop
- `internal/component/storage/manager_test.go` - unit tests
- `internal/component/storage/register.go` - init() registration
- `internal/component/storage/config.go` - Config struct
- `internal/component/cmd/show/storage.go` - show handler
- `test/parse/smart-config.ci` - config test
- `test/plugin/smart-show.ci` - show RPC test

## Files to Delete
- `internal/component/host/smart_linux.go` - moved to storage component
- `internal/component/host/smart_linux_test.go` - moved to storage

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `ze-storage-conf.yang` |
| CLI commands/flags | Yes | `show storage smart` via YANG RPC |
| CLI grammar | Yes | action before noun |
| Editor autocomplete | Yes | YANG-driven |
| Functional test for new RPC | Yes | `test/plugin/smart-show.ci` |
| Doctor check | Yes | SMART enabled on devices |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` |
| 3 | CLI command added? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/storage-health.md` (new) |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK changed? | No | - |
| 9 | RFC behavior? | No | - |
| 10 | Test infra changed? | No | - |
| 11 | Affects comparison? | Yes | `docs/comparison.md` |
| 12 | Architecture changed? | No | - |

## Implementation Steps

### Implementation Phases

1. **Phase: Move SMART to storage component** - move files, update imports, verify host still works
   - Tests: existing host tests + `TestDetectSMART_MovedFromHost`
   - Files: move smart_linux.go, update storage_linux.go caller
   - Verify: `ze host show storage` still includes SMART data

2. **Phase: Wiring** - YANG schema, register show RPC, manager skeleton
   - Tests: `TestManagerCreatedFromConfig`, `TestShowStorageSmart`
   - Files: ze-storage-conf.yang, manager.go, register.go, show/storage.go

3. **Phase: Write ioctls** - enable, disable, start-test, abort-test
   - Tests: ioctl constant verification
   - Files: smart_linux.go (add exported write functions)

4. **Phase: Health poll loop** - ticker, temperature alerting
   - Tests: `TestManagerStartStop`, `TestHealthPollRaisesWarning/ClearsWarning`
   - Files: manager.go

5. **Phase: Self-test scheduler** - interval tracking, in-progress detection
   - Tests: `TestSelfTestScheduling`, `TestSelfTestSkipsInProgress`
   - Files: manager.go

6. **Phase: Hub + doctor integration**
   - Tests: `TestDoctorSmartEnabled`
   - Files: hub/main.go, doctor.go

7. **Phase: Reconfigure** - live interval changes
   - Tests: `TestReconfigure`

8. **Functional tests + full verification**

### Critical Review Checklist
| Check | What to verify |
|-------|---------------|
| Completeness | Every AC-N implemented |
| Correctness | Ioctl feature registers match ATA/NVMe spec |
| Naming | YANG kebab-case, JSON kebab-case, RPC `ze-show:storage-smart` |
| Data flow | Config -> Manager -> ioctl, no skipped layers |
| CLI grammar | `show storage smart` |
| Doctor checks | SMART enabled check registered |
| Goroutine lifecycle | Ticker + stopCh, clean shutdown |
| Move correctness | host tests still pass, no broken imports |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | YANG range constraints on all numeric leaves |
| Ioctl safety | Write ioctls use fixed constants, no user input in buffer |
| Privilege | Graceful error on permission denied |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in phase |
| Ioctl fails | Handle gracefully |
| Ticker doesn't fire | Check goroutine lifecycle |
| 3 fix attempts fail | STOP. Ask user. |

## Design Insights

### Component Placement
New `internal/component/storage/` owns all SMART code (reads + writes + scheduling).
The host package keeps `SmartInfo` type (part of `StorageDevice`) and calls
`storage.DetectSMART()` for detection. This is the same pattern as iface: types in
one package, backend implementation in another.

### Move Strategy
`smart_linux.go` and `smart_linux_test.go` move to `storage/`. `SmartInfo` type
moves from `host/inventory.go` to `storage/smart.go`. `parseSMARTJSON` removed
(dead code). `host.StorageDevice.Smart` field becomes `*storage.SmartInfo` (host
imports storage). The single caller in `storage_linux.go:66` changes from
`d.detectSMART(name)` to `storage.DetectSMART(name)`. Import direction:
host -> storage (clean, no cycle).

### Write Ioctl Pattern
Same `HDIO_DRIVE_CMD` as reads. Enable: `[0xB0, 0xD8, 0, 0]`. Self-test:
`[0xB0, 0xD4, 0, testType]`. NVMe: same `nvmePassthruCmd` with opcode 0x14.

### Self-Test In-Progress Detection
ATA data page offset 363 bits 7:4: non-zero and not 0x0 means test in progress.
NVMe: admin command Get Log Page 0x06 (Device Self-test) byte 4 bits 7:4.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Implementation Summary
- [to be filled]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| SMART auto-enabled | functional test | `test-smart-config` |
| Self-tests scheduled | unit test | `TestSelfTestScheduling` |
| Temperature alerting | unit test | `TestHealthPollRaisesWarning` |
| Show RPC live data | functional test | `test-smart-show` |
| Doctor verifies SMART | unit test | `TestDoctorSmartEnabled` |
| Host show still works after move | existing tests | `TestParseSMARTJSON_*` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Architecture docs updated
- [ ] Critical Review passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs
- [ ] Functional tests for end-to-end
- [ ] Goal Validation filled

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/790-smart-management.md`
- [ ] Summary included in commit
