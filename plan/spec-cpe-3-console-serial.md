# Spec: cpe-3-console-serial

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-05-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/system/schema/ze-system-conf.yang` - system config (host, DNS, tuning)
4. `internal/component/config/system/system.go` - SystemConfig struct, ExtractSystemConfig, extractTuning pattern
5. `cmd/ze/hub/main.go:504-516` - startup wiring (ExtractSystemConfig -> WriteResolvConf -> applyHostTuning)
6. `internal/component/config/system/resolv_linux.go` - linux build-tag pattern for platform-specific code

## Task

Add console/serial port configuration to Ze's system block. Headless CPE devices need serial console for out-of-band management: configuring which serial device to use (e.g., ttyS0) and at what baud rate (e.g., 115200).

Must work on both deployment targets:
- **gokrazy** (no systemd, no getty, read-only rootfs): Ze owns the serial port entirely via termios.
- **Standard Linux** (Ubuntu/Debian with systemd): Ze configures the port via termios. At apply time, checks whether a `serial-getty@<device>.service` is active; if so, logs a warning and skips that device (the user must stop the competing getty first).

Single code path: termios on all Linux hosts. No systemd unit management by ze. The only platform difference is the getty-conflict check (queries systemd on hosts that have it, skipped when systemd is absent).

The config extends the existing `system { ... }` YANG container with a `console` sub-block.

**Motivation:** VyOS lns.conf uses `system console device ttyS0 speed 115200`.

## Required Reading

### Architecture Docs
- [ ] `internal/component/config/system/schema/ze-system-conf.yang` - system container with host, domain, name-server, dns, peeringdb, tuning, archive
  → Decision: Add `console` container as sibling to `tuning` inside `system`
- [ ] `internal/component/config/system/schema/register.go` - YANG module registration via init()
  → Constraint: Console schema extends the existing ze-system-conf.yang, no separate module needed
- [ ] `internal/component/config/system/system.go` - SystemConfig struct, ExtractSystemConfig, extractTuning pattern
  → Decision: Add ConsoleDevices field to SystemConfig, extractConsole follows extractTuning pattern
- [ ] `cmd/ze/hub/main.go:504-516` - system config wiring (ExtractSystemConfig -> WriteResolvConf -> applyHostTuning)
  → Decision: Add applyConsole call after applyHostTuning, same pattern
- [ ] `docs/architecture/core-design.md` - gokrazy appliance model

**Key insights:**
- termios on all Linux hosts (single code path), no systemd unit management
- gokrazy has no getty; ze owns the port. Standard Linux may have serial-getty competing.
- At apply time: check `systemctl is-active serial-getty@<device>` if systemd is present. If active, warn and skip (do not fight for the port).
- Speed must be one of the standard baud rates (9600, 19200, 38400, 57600, 115200)
- Device path validated at apply time (device must exist in /dev/)
- Applied at daemon startup and on config reload
- Data bits fixed at 8N1 (industry standard for console, not configurable)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/schema/ze-system-conf.yang` - system container with host, domain, name-server, dns, peeringdb, tuning (cpu, irq-affinity, ethtool), archive
- [ ] `internal/component/config/system/schema/register.go` - YANG module registration for ze-system-conf
- [ ] `internal/component/config/system/system.go` - SystemConfig struct, ExtractSystemConfig, extractTuning, sanitizeResolvConfPath
- [ ] `internal/component/config/system/resolv_linux.go` - WriteResolvConf (linux build tag, atomic write pattern)
- [ ] `internal/component/config/system/resolv_other.go` - non-linux stub
- [ ] `cmd/ze/hub/main.go:504-516` - startup wiring: ExtractSystemConfig -> WriteResolvConf -> applyHostTuning

**Behavior to preserve:**
- Existing system container fields (host, domain, name-server, dns, peeringdb, tuning, archive) unchanged
- YANG module registration pattern in register.go (single module, no new module for console)
- System config application order at startup
- ExtractSystemConfig / extractTuning pattern in system.go

**Behavior to change:**
- Add `console` container with `device` list to ze-system-conf.yang
- Add ConsoleDevices to SystemConfig, extractConsole to system.go
- New `console_linux.go`: termios apply + getty-conflict detection
- New `console_other.go`: no-op stub (serial console is Linux-only)
- Wire applyConsole in `cmd/ze/hub/main.go` after applyHostTuning

## Data Flow (MANDATORY)

### Entry Point
- Config commit containing `system { console { device ttyS0 { speed 115200 } } }`

### Transformation Path
1. YANG schema validates config (device name is string, speed is valid enum)
2. `ExtractSystemConfig` extracts console device list into `SystemConfig.ConsoleDevices`
3. `applyConsole` iterates devices:
   a. Check /dev/<device> exists. If not: warn, skip.
   b. On systemd hosts: check `systemctl is-active serial-getty@<device>`. If active: warn, skip.
   c. Open /dev/<device>, configure baud rate via termios (tcsetattr with B<speed>).
4. Serial device ready for terminal I/O

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> system component | ExtractSystemConfig at startup/reload | [ ] |
| system component -> systemd (optional) | exec `systemctl is-active` (best-effort, no error if missing) | [ ] |
| system component -> kernel | termios ioctl on /dev/<device> | [ ] |

### Integration Points
- `internal/component/config/system/system.go` - ConsoleDevices in SystemConfig, extractConsole
- `internal/component/config/system/console_linux.go` - termios apply + getty check
- `cmd/ze/hub/main.go` - applyConsole called after applyHostTuning at startup

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config commit with console block | → | console config parse + apply | `test/system/console.ci` |
| device ttyS0 speed 115200 | → | termios baud rate set | `TestConsoleApplySpeed` |
| invalid speed value | → | config validation rejection | `TestConsoleInvalidSpeed` |
| device with active serial-getty | → | getty conflict detection, skip + warn | `TestConsoleGettyConflict` |
| device on gokrazy (no systemctl) | → | getty check skipped, termios applied | `TestConsoleNoSystemd` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `system { console { device ttyS0 { speed 115200 } } }` | Config parses and validates successfully |
| AC-2 | Speed set to valid enum (9600, 19200, 38400, 57600, 115200) | Accepted by YANG validation |
| AC-3 | Speed set to invalid value (e.g., 12345) | Rejected by YANG enum validation |
| AC-4 | Console apply on startup (device exists, no getty) | Serial device opened, baud rate configured via termios |
| AC-5 | Device does not exist (e.g., /dev/ttyS99) | Warning logged, startup continues (non-fatal) |
| AC-6 | Config reload with speed change | Serial device reconfigured with new baud rate |
| AC-7 | `serial-getty@ttyS0` is active on systemd host | Warning logged naming the conflicting service, device skipped |
| AC-8 | No systemd on host (gokrazy) | Getty check skipped silently, termios applied directly |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConsoleConfigParse` | `internal/component/config/system/console_test.go` | Config extraction: device name, speed | |
| `TestConsoleApplySpeed` | `internal/component/config/system/console_test.go` | Baud rate applied via termios (mock) | |
| `TestConsoleInvalidSpeed` | `internal/component/config/system/console_test.go` | Invalid speed rejected by validation | |
| `TestConsoleDeviceNotFound` | `internal/component/config/system/console_test.go` | Missing device logs warning, does not crash | |
| `TestConsoleMultipleDevices` | `internal/component/config/system/console_test.go` | Multiple devices each configured independently | |
| `TestConsoleGettyConflict` | `internal/component/config/system/console_test.go` | Active serial-getty detected, device skipped with warning | |
| `TestConsoleNoSystemd` | `internal/component/config/system/console_test.go` | systemctl not found, getty check skipped, termios applied | |
| `TestConsoleDevicePathTraversal` | `internal/component/config/system/console_test.go` | Device name with `../` rejected | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| speed | enum: 9600, 19200, 38400, 57600, 115200 | 115200 | N/A (enum) | N/A (enum) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-console-serial` | `test/system/console.ci` | Serial console configured at startup with correct baud rate | |

## Files to Modify
- `internal/component/config/system/schema/ze-system-conf.yang` - add console container with device list and speed enum

## Files to Create
- `internal/component/config/system/console.go` - ConsoleDeviceEntry type, extractConsole, device name validation (no build tag)
- `internal/component/config/system/console_linux.go` - ApplyConsole: getty-conflict check + termios apply (linux build tag)
- `internal/component/config/system/console_other.go` - ApplyConsole no-op stub (non-linux build tag)
- `internal/component/config/system/console_test.go` - unit tests
- `test/system/console.ci` - functional test

## Files to Wire
- `internal/component/config/system/system.go` - add ConsoleDevices []ConsoleDeviceEntry to SystemConfig, call extractConsole
- `cmd/ze/hub/main.go` - add applyConsole call after applyHostTuning (line ~516)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist |
| 7-9. Fix cycle | Fix issues, re-verify |
| 10. Deliverables | Deliverables Checklist |
| 11. Security | Security Review Checklist |

### Implementation Phases

1. **Phase: YANG schema** - Add console container with device list and speed enum to ze-system-conf.yang
   - Tests: `TestConsoleConfigParse`, `TestConsoleInvalidSpeed`
   - Files: `ze-system-conf.yang`
   - Verify: config parse test fails -> implement schema -> passes

2. **Phase: Config extraction** - Extract console device list from config tree, add to SystemConfig
   - Tests: `TestConsoleConfigParse`, `TestConsoleMultipleDevices`, `TestConsoleDevicePathTraversal`
   - Files: `console.go`, `system.go` (add ConsoleDevices field + extractConsole call)
   - Verify: tests fail -> implement -> pass

3. **Phase: Serial apply (Linux)** - Termios baud rate configuration + getty-conflict detection
   - Tests: `TestConsoleApplySpeed`, `TestConsoleDeviceNotFound`, `TestConsoleGettyConflict`, `TestConsoleNoSystemd`
   - Files: `console_linux.go`, `console_other.go`
   - Verify: tests fail -> implement -> pass

4. **Phase: Startup wiring** - Wire applyConsole into daemon startup
   - Files: `cmd/ze/hub/main.go`
   - Verify: grep confirms call site exists after applyHostTuning

5. **Phase: Functional tests** - End-to-end serial console configuration
   - Tests: `test/system/console.ci`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-1 through AC-8 have implementation with file:line |
| Correctness | Termios baud rate constants match kernel values (B9600, B115200, etc.) |
| Naming | Console config under `system { console { ... } }` matches YANG hierarchy |
| Data flow | Config extract -> getty check -> termios apply, no intermediate state |
| Rule: no-layering | Extends existing system YANG, does not create separate module |
| Dual-platform | Works on gokrazy (no systemd) and standard Linux (systemd present) |
| Getty safety | Active serial-getty detected and reported, not silently overridden |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG console container | `grep console internal/component/config/system/schema/ze-system-conf.yang` |
| Config extraction | `ls internal/component/config/system/console.go` |
| Linux termios + getty check | `ls internal/component/config/system/console_linux.go` |
| Non-linux stub | `ls internal/component/config/system/console_other.go` |
| Startup wiring | `grep applyConsole cmd/ze/hub/main.go` |
| SystemConfig ConsoleDevices | `grep ConsoleDevices internal/component/config/system/system.go` |
| Unit tests pass | `go test ./internal/component/config/system/ -run Console` |
| Functional test | `ls test/system/console.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Device name must not contain path traversal (../, /), must be bare name only |
| Privilege | Opening /dev/ttyS* requires appropriate permissions (root or dialout group) |
| Error handling | Missing device is warning not crash (headless device may not have serial) |
| Getty conflict | Must not silently override an active serial-getty (data corruption risk) |
| systemctl exec | Exec of systemctl must use exact path, no shell injection via device name |

### Documentation Update Checklist
| Category | Needed? | File + Section |
|----------|---------|----------------|
| Feature list | No | Feature is config-level, not user-facing UI |
| User guide | No | No user guide exists yet |
| Config syntax | Yes | `docs/architecture/config/syntax.md` - add `system { console { ... } }` example |
| CLI reference | No | No new CLI commands |
| API/RPC docs | No | No new API endpoints |
| Plugin SDK | No | Not a plugin |
| Wire format | No | No wire format changes |
| RFC compliance | No | No RFC involved |
| Comparison table | No | Not comparable to external features |
| Test infrastructure | No | Standard unit tests |
| Architecture design | No | Extends existing system config pattern |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read system YANG and source from Current Behavior |
| Lint failure | Fix inline |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/component/config/system/*`)
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling
