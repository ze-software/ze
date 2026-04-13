# Spec: vpp-1-lifecycle — VPP Lifecycle Management

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-vpp-0-umbrella |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/spec-vpp-0-umbrella.md` — parent spec
4. `internal/plugins/fibkernel/register.go` — registration pattern
5. `internal/component/iface/` — component pattern

## Task

Create ze's VPP component that manages VPP as a supervised process. The component generates
startup.conf from YANG config, binds NICs to DPDK (vfio-pci), starts/stops VPP, monitors
health via GoVPP connection state, and exposes a shared GoVPP connection for dependent plugins
(fibvpp, ifacevpp).

This is the foundation: everything else in the VPP spec set requires a running VPP instance
with a healthy GoVPP connection.

### Reference

- IPng.ch blog: VPP deployment patterns, startup.conf templates, DPDK binding, LCP configuration
- VyOS: `control_host.py` for DPDK NIC driver binding logic (ported to Go)
- GoVPP documentation (go.fd.io/govpp): AsyncConnect, stats client, socket transport

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component architecture
  → Constraint: components under `internal/component/`
  → Decision: VPP is a component (lifecycle), not a plugin (feature)
- [ ] `internal/plugins/fibkernel/register.go` - plugin registration pattern
  → Constraint: registry.Registration with Name, Description, Features, YANG, Dependencies, RunEngine
- [ ] `internal/component/iface/iface.go` - component lifecycle pattern
  → Constraint: OnStarted, OnConfigVerify, OnConfigApply callbacks
- [ ] `.claude/patterns/registration.md` - registration pattern
  → Constraint: init() + registry.Register()
- [ ] `.claude/patterns/config-option.md` - config option pattern
  → Constraint: YANG leaf + env.MustRegister()
- [ ] `rules/config-design.md` - YANG structure rules
  → Constraint: fail on unknown keys, no version numbers

### RFC Summaries (MUST for protocol work)

Not protocol work. No RFCs apply.

**Key insights:**
- VPP component registers as a plugin (registry.Register) so other plugins can declare dependency on it
- GoVPP AsyncConnect provides event channel for connection state changes
- DPDK NIC binding follows VyOS control_host.py pattern: save driver, load vfio, unbind, rebind
- startup.conf is a template rendered from YANG config leaves

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/fibkernel/register.go` — Registration with Dependencies, YANG, RunEngine
  → Constraint: vpp component follows same registration pattern
- [ ] `internal/plugins/fibkernel/fibkernel.go` — Plugin run loop with EventBus subscription
  → Constraint: vpp component provides connection, dependents subscribe to its ready state
- [ ] `internal/component/iface/iface.go` — Component lifecycle callbacks
  → Constraint: OnStarted for initial setup, OnConfigApply for changes
- [ ] No existing VPP code in ze

**Behavior to preserve:**
- All existing components and plugins unaffected
- Plugin registration patterns unchanged
- EventBus event formats unchanged

**Behavior to change:**
- New `internal/component/vpp/` package
- New YANG module `ze-vpp-conf`
- New dependency: go.fd.io/govpp

## Data Flow (MANDATORY)

### Entry Point
- YANG config file contains `vpp { enabled true; ... }` section
- Config component parses YANG tree at startup

### Transformation Path
1. Config component parses `vpp` subtree from YANG config
2. VPP component's OnConfigVerify validates config values (PCI addresses, core counts, paths)
3. VPP component's OnStarted triggers startup sequence:
   a. Parse YANG config into VPP settings struct
   b. Generate startup.conf from settings
   c. Bind configured NICs to vfio-pci (DPDK)
   d. Start VPP process (or verify externally managed VPP is running)
   e. Connect via GoVPP AsyncConnect to api-socket
   f. Wait for Connected event
   g. Notify dependents (fibvpp, ifacevpp) that VPP is ready
4. On config reload: regenerate startup.conf, signal VPP to reload (or restart)
5. On shutdown: disconnect GoVPP, unbind NICs from vfio-pci, restore original drivers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → VPP component | YANG tree parsing, OnConfigure callbacks | [ ] |
| VPP component → VPP process | startup.conf file, process management | [ ] |
| VPP component → GoVPP | Unix socket (/run/vpp/api.sock), binary API | [ ] |
| VPP component → dependents | Shared VPPConn object, dependency ordering | [ ] |
| VPP component → sysfs | DPDK NIC binding via /sys/bus/pci/ | [ ] |

### Integration Points
- `internal/plugin/registry.go` — VPP registers as plugin so fibvpp/ifacevpp can depend on it
- `internal/component/config/` — YANG tree parsing
- `internal/core/env/` — env var registration for VPP config leaves
- `pkg/ze/eventbus.go` — VPP emits connection state events

### Architectural Verification
- [ ] No bypassed layers (config → component → startup.conf → VPP process → GoVPP)
- [ ] No unintended coupling (dependents access VPP through shared connection only)
- [ ] No duplicated functionality (new capability, not recreating existing)
- [ ] Zero-copy preserved where applicable (GoVPP handles binary encoding internally)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| VPP YANG config with enabled=true | → | VPP component generates startup.conf | `test/vpp/001-boot.ci` |
| VPP YANG config with dpdk interfaces | → | NIC binding to vfio-pci | `test/vpp/001-boot.ci` |
| VPP component OnStarted | → | GoVPP AsyncConnect to api-socket | `test/vpp/001-boot.ci` |
| VPP process crash | → | GoVPP disconnect detection, reconnect | `test/vpp/001-boot.ci` (reconnect scenario) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG config with `vpp { enabled true }` and DPDK interfaces | startup.conf generated with correct cpu, memory, dpdk, plugins sections |
| AC-2 | startup.conf generated | VPP process starts successfully |
| AC-3 | VPP process running | GoVPP connects via api-socket, connection state is Connected |
| AC-4 | DPDK interface configured with PCI address | NIC unbound from kernel driver, bound to vfio-pci |
| AC-5 | VPP component shutting down | NICs restored to original kernel drivers |
| AC-6 | VPP process crashes | GoVPP detects disconnect, attempts reconnect with backoff |
| AC-7 | VPP reconnected after crash | Dependents notified, emit replay-request for FIB repopulation |
| AC-8 | LCP enabled in config | startup.conf includes linux-cp and linux-nl sections |
| AC-9 | LCP disabled in config | startup.conf omits linux-cp and linux-nl sections |
| AC-10 | Invalid PCI address in config | OnConfigVerify rejects with clear error message |
| AC-11 | YANG config env vars | Every `vpp.*` config leaf has matching `ze.vpp.*` env var via env.MustRegister() |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStartupConfGeneration` | `internal/component/vpp/startupconf_test.go` | Config struct to startup.conf template rendering | |
| `TestStartupConfCPU` | `internal/component/vpp/startupconf_test.go` | CPU section: main-core, corelist-workers | |
| `TestStartupConfDPDK` | `internal/component/vpp/startupconf_test.go` | DPDK section: dev entries with PCI, name, queues | |
| `TestStartupConfLCPEnabled` | `internal/component/vpp/startupconf_test.go` | LCP sections present when enabled | |
| `TestStartupConfLCPDisabled` | `internal/component/vpp/startupconf_test.go` | LCP sections absent when disabled | |
| `TestStartupConfBuffers` | `internal/component/vpp/startupconf_test.go` | Buffer count and hugepage size | |
| `TestParsePCIAddress` | `internal/component/vpp/dpdk_test.go` | PCI address format validation | |
| `TestConfigParse` | `internal/component/vpp/config_test.go` | YANG tree to VPP settings struct | |
| `TestConfigValidation` | `internal/component/vpp/config_test.go` | Invalid configs rejected with clear errors | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| main-core | 0-255 | 255 | N/A (0 = auto) | 256 |
| workers | 0-255 | 255 | N/A (0 = auto) | 256 |
| hugepage-size | enum (2M, 1G) | 1G | invalid string | invalid string |
| buffers | 1-4294967295 | 4294967295 | 0 | N/A (uint32) |
| rx-queues | 1-255 | 255 | 0 | 256 |
| tx-queues | 1-255 | 255 | 0 | 256 |
| PCI address | DDDD:DD:DD.D format | valid addr | malformed | malformed |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-vpp-boot` | `test/vpp/001-boot.ci` | VPP config present, ze starts, startup.conf generated, VPP connected | |
| `test-vpp-reconnect` | `test/vpp/001-boot.ci` | VPP crashes, ze detects, reconnects (connection-level, not FIB replay) | |

### Future (if deferring any tests)
- Integration tests requiring real VPP + DPDK NICs deferred to lab environment

## Files to Modify

- `go.mod` — add go.fd.io/govpp dependency
- `go.sum` — updated
- `vendor/` — GoVPP vendored

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/component/vpp/schema/ze-vpp-conf.yang` |
| CLI commands/flags | Yes | VPP status command (show vpp connection state, NIC bindings) |
| Editor autocomplete | Yes | YANG-driven (automatic) |
| Functional test | Yes | `test/vpp/001-boot.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — add VPP lifecycle management |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` — add VPP section |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — VPP status |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — add VPP component |
| 6 | Has a user guide page? | Yes | `docs/guide/vpp.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — VPP integration |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — VPP component |

## Files to Create

- `internal/component/vpp/vpp.go` — Component: startup, shutdown, health monitoring
- `internal/component/vpp/config.go` — Parse YANG config into VPP settings struct
- `internal/component/vpp/startupconf.go` — Generate VPP startup.conf from config
- `internal/component/vpp/dpdk.go` — NIC driver binding: save driver, load vfio, bind vfio-pci
- `internal/component/vpp/conn.go` — GoVPP connection management (AsyncConnect, retry, health check)
- `internal/component/vpp/register.go` — Component registration
- `internal/component/vpp/schema/ze-vpp-conf.yang` — YANG module
- `internal/component/vpp/startupconf_test.go` — startup.conf generation tests
- `internal/component/vpp/dpdk_test.go` — DPDK binding tests
- `internal/component/vpp/config_test.go` — Config parsing tests
- `test/vpp/001-boot.ci` — VPP boot functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG + config parsing** — YANG module, config struct, parser
   - Tests: `TestConfigParse`, `TestConfigValidation`
   - Files: schema/ze-vpp-conf.yang, config.go, config_test.go
   - Verify: tests fail → implement → tests pass

2. **Phase: startup.conf generation** — template rendering from config
   - Tests: `TestStartupConfGeneration`, `TestStartupConfCPU`, `TestStartupConfDPDK`, `TestStartupConfLCP*`, `TestStartupConfBuffers`
   - Files: startupconf.go, startupconf_test.go
   - Verify: tests fail → implement → tests pass

3. **Phase: DPDK NIC binding** — sysfs driver management
   - Tests: `TestParsePCIAddress`
   - Files: dpdk.go, dpdk_test.go
   - Verify: tests fail → implement → tests pass

4. **Phase: GoVPP connection** — AsyncConnect, state machine, health check
   - Tests: connection lifecycle tests
   - Files: conn.go
   - Verify: tests fail → implement → tests pass

5. **Phase: Component wiring** — registration, lifecycle callbacks, dependency exposure
   - Tests: registration test
   - Files: vpp.go, register.go
   - Verify: tests fail → implement → tests pass

6. **Functional tests** → `test/vpp/001-boot.ci`
7. **Full verification** → `make ze-verify`
8. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | startup.conf output matches IPng/VyOS proven templates |
| Naming | YANG leaves follow ze conventions (kebab-case) |
| Data flow | Config → settings struct → startup.conf → VPP process → GoVPP |
| Rule: no-layering | No abstraction between GoVPP and VPP (direct socket) |
| Rule: single-responsibility | vpp.go = lifecycle, config.go = parsing, startupconf.go = generation, dpdk.go = NIC, conn.go = connection |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| VPP component directory | `ls internal/component/vpp/` |
| YANG module | `grep "module ze-vpp-conf" internal/component/vpp/schema/` |
| Registration | `grep "registry.Register" internal/component/vpp/register.go` |
| startup.conf generation | `go test -run TestStartupConf internal/component/vpp/` |
| DPDK binding | `go test -run TestParsePCI internal/component/vpp/` |
| Env vars | `grep "env.MustRegister" internal/component/vpp/` |
| Functional test | `ls test/vpp/001-boot.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| PCI address validation | Strict format validation to prevent sysfs path traversal |
| Module loading | vfio module names hardcoded, never from config |
| Socket path | api-socket path from config. Validate it is a Unix socket path, no injection |
| Process management | VPP started with minimal privileges. No shell injection in command args. |
| Driver save/restore | Saved driver name validated against known driver patterns |
| Sysfs writes | All sysfs paths constructed from validated PCI addresses only |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## YANG Config Shape

| Container | Leaf | Type | Default | Description |
|-----------|------|------|---------|-------------|
| vpp | enabled | boolean | false | Enable VPP integration |
| vpp | api-socket | string | /run/vpp/api.sock | GoVPP API socket path |
| vpp/cpu | main-core | uint8 | - | CPU core for VPP main thread |
| vpp/cpu | workers | uint8 | - | Number of worker threads |
| vpp/cpu | isolate | leaf-list uint8 | - | CPU cores to isolate |
| vpp/memory | main-heap | string | 1G | Main heap size |
| vpp/memory | hugepage-size | enum (2M, 1G) | 2M | Hugepage size |
| vpp/memory | buffers | uint32 | 128000 | Buffer count |
| vpp/dpdk/interface (list, key: pci-address) | pci-address | string | - | PCI address (0000:03:00.0) |
| vpp/dpdk/interface | name | string | - | Interface name (xe0) |
| vpp/dpdk/interface | rx-queues | uint8 | - | Receive queues |
| vpp/dpdk/interface | tx-queues | uint8 | - | Transmit queues |
| vpp/stats | segment-size | string | 512M | Stats segment size |
| vpp/stats | socket-path | string | /run/vpp/stats.sock | Stats socket path |
| vpp/lcp | enabled | boolean | true | Enable LCP plugin |
| vpp/lcp | sync | boolean | true | Sync LCP state |
| vpp/lcp | auto-subint | boolean | true | Auto-create sub-interfaces |
| vpp/lcp | netns | string | dataplane | Network namespace for LCP |

## startup.conf Sections

| Section | Contents | Source |
|---------|----------|--------|
| unix | nodaemon, cli-listen socket, log path, coredump | Hardcoded defaults |
| cpu | main-core, corelist-workers | vpp/cpu YANG leaves |
| buffers | buffers-per-numa, default-data-size, page-size | vpp/memory YANG leaves |
| dpdk | dev entries with PCI address, name, rx/tx queues | vpp/dpdk/interface list |
| plugins | Enable dpdk, linux_cp, linux_nl; disable others | Hardcoded, LCP toggle from vpp/lcp/enabled |
| linux-cp | lcp-sync, lcp-auto-subint, default netns | vpp/lcp YANG leaves (only if LCP enabled) |
| linux-nl | rx-buffer-size (67108864) | Hardcoded (only if LCP enabled) |
| statseg | size, page-size | vpp/stats + vpp/memory YANG leaves |

Pattern follows IPng.ch blog and VyOS proven templates.

## DPDK NIC Driver Management

**Bind sequence** (per configured PCI address):
1. Read current driver from `/sys/bus/pci/devices/<addr>/driver` symlink
2. Save original driver name to persistent state file
3. Load vfio kernel modules: `vfio`, `vfio_pci`, `vfio_iommu_type1`
4. Write PCI address to `/sys/bus/pci/devices/<addr>/driver/unbind`
5. Write vendor:device to `/sys/bus/pci/drivers/vfio-pci/new_id`

**Unbind sequence** (teardown, reverse):
1. Unbind from vfio-pci
2. Trigger PCI rescan via `/sys/bus/pci/rescan`
3. Rebind to original saved driver

Ported from VyOS `control_host.py` to Go.

## GoVPP Connection State Machine

| State | Transition | Action |
|-------|-----------|--------|
| Disconnected | Connect requested | AsyncConnect to api-socket with retry (10 attempts, 1s interval) |
| Connecting | Connected event from GoVPP | Mark ready, notify dependent plugins |
| Connected | Disconnect event from GoVPP | Mark unavailable, dependents stop operations |
| Connected | Health check timeout | Attempt reconnect with exponential backoff |
| Reconnecting | Connected event from GoVPP | Mark ready, emit replay-request for FIB repopulation |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented
- (To be filled after implementation)

### Bugs Found/Fixed
- (To be filled)

### Documentation Updates
- (To be filled)

### Deviations from Plan
- (To be filled)

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-vpp-1-lifecycle.md`
- [ ] Summary included in commit
