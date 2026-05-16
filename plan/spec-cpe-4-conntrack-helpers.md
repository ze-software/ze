# Spec: cpe-4-conntrack-helpers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/7 |
| Updated | 2026-05-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/sysctl/sysctl.go` - store.setDefault (default layer), store.applyConfig (config layer), three-layer precedence
4. `internal/plugins/sysctl/events/events.go` - EventDefault is the mechanism for external sysctl contributions
5. `internal/component/config/system/schema/ze-system-conf.yang` - system config schema
6. `internal/component/config/system/system.go` - SystemConfig struct, extraction patterns
7. `internal/component/config/system/console.go` - ExtractConsoleFromMap (reload-path pattern)
8. `internal/component/telemetry/collector/conntrack_linux.go` - existing conntrack telemetry
9. `cmd/ze/hub/main.go` - startup wiring (applyConsole/applyHostTuning at L516-517, reload at L889-890)

## Task

Add comprehensive connection tracking management to Ze. This covers three areas:

1. **Helper modules** -- declarative loading of kernel conntrack helpers (ftp, sip, h323, etc.)
2. **Tuning** -- user-friendly config for table sizing, protocol timeouts, and TCP behavior, mapped internally to sysctl keys
3. **Observability** -- `show system conntrack` CLI command and telemetry integration

Config lives under `system conntrack`. The backend maps user-friendly names to sysctl keys and uses the sysctl plugin's config layer for writes (preventing dual-setting conflicts if a user also configures the same keys via the `sysctl` config block). Helper module loading uses `modprobe` on Linux. Modules are load-only: once loaded, they are never unloaded at runtime (unloading breaks active connections).

**Motivation:** VyOS home.conf uses `system { conntrack { modules { ftp; h323; nfs; pptp; sip; sqlnet; tftp } } }`. Ze exposes a broader feature set with user-friendly naming.

### Design Decisions

| Decision | Detail |
|----------|--------|
| Global scope | Conntrack is per-netns. Ze uses one netns (VRF devices, not namespaces). All VRFs share one conntrack table. Per-VRF isolation via conntrack zones is future work (spec-vrf-8) |
| User-friendly names | Config uses readable names (`table-size`, `tcp established`) mapped to sysctl keys internally. Users never write raw sysctl paths for conntrack |
| Sysctl plugin integration | Table-size and timeout values are written through the sysctl plugin's config layer, not directly. This prevents conflicts if the same key appears in both `system conntrack` and `sysctl` config blocks. On conflict: reject at config validation with a message pointing to the friendly name |
| Load-only helpers | `modprobe nf_conntrack_<name>` on commit. Never `modprobe -r`. Unloading breaks active connections. Removing a helper from config stops loading it on next boot but does not unload at runtime |
| Nested timeouts | `timeout tcp { established 7200; }` not `timeout tcp-established 7200`. Groups related knobs, easier to extend |
| Validated module set | Module names checked against a compile-time allowlist of real kernel modules. Unknown names rejected at config validation |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component isolation, registration pattern
  -> Constraint: conntrack is system-level config, not a plugin
- [ ] `internal/plugins/sysctl/register.go` - sysctl plugin registration and apply pattern
  -> Decision: conntrack routes sysctl values through EventBus (sysctl, default) with source "system-conntrack"; sysctl plugin's setDefault() applies them at the default layer
  -> Constraint: EventBus subscription happens in runSysctlPlugin(); conntrack emits via eb.Emit(sysctlevents.Namespace, sysctlevents.EventDefault, jsonPayload)
- [ ] `internal/plugins/sysctl/sysctl.go` - store.setDefault() for plugin-contributed values, applyConfig() for YANG config layer
  -> Constraint: setDefault takes key/value/source JSON; default layer is lowest priority (config > transient > default); conntrack values use default layer since dual-setting prevention blocks config-layer conflicts
  -> Constraint: No managed key set registration exists yet; must be added for dual-setting prevention
- [ ] `internal/component/config/system/schema/ze-system-conf.yang` - existing system schema (host, dns, tuning, archive)
  -> Decision: add conntrack container alongside existing system children
- [ ] `internal/component/telemetry/collector/conntrack_linux.go` - existing conntrack telemetry collector
  -> Constraint: already reads /proc/net/stat/nf_conntrack for per-CPU counters
- [ ] `plan/spec-vrf-0-umbrella.md` - VRF umbrella, Q7 resolved: conntrack zones for per-VRF isolation
  -> Constraint: this spec is global only. Per-VRF zones deferred to spec-vrf-8

**Key insights:**

Helper modules (real kernel modules, validated set):
- ftp, h323, sip, pptp, tftp, sane, irc, amanda, netbios-ns, snmp (10 modules)
- `nf_conntrack_broadcast` is an infrastructure dependency (netbios-ns, snmp), not user-facing
- NOT real modules: sqlnet, nfs, broadcast (userspace-only or infrastructure)
- SIP ALG is notorious for breaking VoIP; document the risk

Global sysctl tunables mapped to user-friendly config:
- `nf_conntrack_max` -> `table-size`
- `nf_conntrack_buckets` -> `hash-size`
- `nf_conntrack_expect_max` -> `expect-max`
- `nf_conntrack_generic_timeout` -> `timeout generic`
- `nf_conntrack_acct` -> `accounting`
- `nf_conntrack_timestamp` -> `timestamp`
- `nf_conntrack_checksum` -> `checksum`
- `nf_conntrack_log_invalid` -> `log-invalid`

TCP timeouts (nested under `timeout tcp {}`):
- established (432000s), syn-sent (120s), syn-recv (60s), fin-wait (120s),
  close-wait (60s), last-ack (30s), time-wait (120s), close (10s),
  unacknowledged (300s), max-retrans (300s)

TCP behavior (nested under `tcp {}`):
- be-liberal (false), loose (true), max-retrans (3), ignore-invalid-rst (false)

UDP timeouts: timeout (30s), stream (120s)
ICMP timeouts: timeout (30s)
ICMPv6 timeouts: timeout (30s)
GRE timeouts: timeout (30s), stream (180s)
SCTP timeouts: closed, cookie-wait, cookie-echoed, established, shutdown-sent, shutdown-recd, shutdown-ack-sent, heartbeat-sent
DCCP timeouts: request, respond, partopen, open, closereq, closing, timewait

Monitoring interfaces:
- `/proc/net/nf_conntrack` - per-connection state (L3/L4 proto, TTL, state, src/dst/port, mark, zone)
- `/proc/net/stat/nf_conntrack` - per-CPU counters (entries, found, new, invalid, drop, early_drop, insert_failed, search_restart)
- `nf_conntrack_count` - current entry count (read-only sysctl)

Platform notes:
- Module load via `modprobe nf_conntrack_<name>`. Load-only, never unload.
- On gokrazy, modules are built-in (no modprobe needed), only sysctl tuning applies

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/sysctl/register.go` - sysctl plugin registration and EventBus wiring
- [ ] `internal/plugins/sysctl/sysctl.go` - store.applyConfig(), setDefault(), conflict detection
- [ ] `internal/plugins/sysctl/schema/ze-sysctl-conf.yang` - sysctl key/value model
- [ ] `internal/component/config/system/schema/ze-system-conf.yang` - system config (host, dns, tuning)
- [ ] `internal/component/config/system/system.go` - ExtractSystemConfig, ExtractTuningFromMap pattern
- [ ] `internal/component/config/system/console.go` - ExtractConsoleFromMap (reload-path extractor pattern)
- [ ] `internal/component/telemetry/collector/conntrack_linux.go` - existing conntrack telemetry (per-CPU counters from procfs)
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - firewall uses conntrack states in rules

**Behavior to preserve:**
- Existing system config schema children unchanged (host, domain, dns, tuning, archive, peeringdb, console)
- Sysctl plugin operates independently (generic key/value), conntrack config routes through its config layer
- Firewall conntrack state matching works without explicit helper loading
- Existing telemetry conntrack collector continues to work

**Behavior to change:**
- Add `conntrack` container to system YANG schema
- New backend applies modprobe on config commit (Linux only, load-only)
- Conntrack sysctl values written through sysctl plugin's applyConfig (table-size, timeouts, behavior)
- Sysctl plugin rejects direct config of keys that conntrack manages (dual-setting prevention)
- New `show system conntrack` CLI command
- Telemetry collector gains configured-max gauge alongside existing counters

## Config Syntax

```
system {
    conntrack {
        module ftp
        module sip
        module h323
        module tftp
        module pptp

        table-size 262144
        hash-size 65536
        expect-max 1024

        accounting
        timestamp
        log-invalid tcp

        timeout {
            generic 600

            tcp {
                established 432000
                syn-sent 120
                syn-recv 60
                fin-wait 120
                close-wait 60
                last-ack 30
                time-wait 120
                close 10
                unacknowledged 300
                max-retrans 300
            }

            udp {
                timeout 30
                stream 120
            }

            icmp {
                timeout 30
            }

            icmpv6 {
                timeout 30
            }

            gre {
                timeout 30
                stream 180
            }

            sctp {
                closed 10
                cookie-wait 3
                cookie-echoed 3
                established 210
                shutdown-sent 3
                shutdown-recd 3
                shutdown-ack-sent 3
                heartbeat-sent 30
            }

            dccp {
                request 240
                respond 480
                partopen 480
                open 43200
                closereq 64
                closing 64
                timewait 240
            }
        }

        tcp {
            be-liberal false
            loose true
            max-retrans 3
            ignore-invalid-rst false
        }
    }
}
```

## Data Flow (MANDATORY)

### Entry Point
- Config commit with `system { conntrack { ... } }` (see Config Syntax above)

### Transformation Path
1. YANG schema parsed, module names validated against compile-time allowlist
2. Config tree extracted by system config loader (ExtractConntrackConfig + ExtractConntrackFromMap)
3. Friendly names mapped to sysctl key/value pairs (e.g. `table-size 262144` -> `nf_conntrack_max=262144`)
4. Sysctl values sent to sysctl plugin via EventBus `(sysctl, default)` with source `"system-conntrack"`, one event per key
5. Sysctl plugin applies via setDefault(), captures originals; config layer (from `sysctl {}` block) wins if both exist (but dual-setting prevention blocks that)
6. On commit (Linux only): iterate module list, run `modprobe nf_conntrack_<name>` for each
7. On gokrazy: skip modprobe (modules built-in), sysctl tuning still applied
8. Module removal from config: no runtime action (load-only). Module will not be loaded on next boot

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> system loader | config tree extract at startup/reload | [ ] |
| System loader -> sysctl plugin | EventBus (sysctl, default) per key with source "system-conntrack" | [ ] |
| System loader -> kernel | modprobe exec (Linux only, load-only) | [ ] |
| CLI -> system loader | `show system conntrack` reads live state | [ ] |
| Telemetry -> procfs | conntrack collector reads /proc/net/stat/nf_conntrack | [ ] |

### Integration Points
- `internal/component/config/system/` - system config extraction and apply
- `internal/plugins/sysctl/` - sysctl plugin config layer (applyConfig, managed key registration)
- `internal/component/telemetry/collector/conntrack_linux.go` - telemetry integration
- `cmd/ze/hub/main.go` - startup wiring (applyConntrack/applyConntrackFromMap pattern)

### Architectural Verification
- [ ] No bypassed layers (sysctl writes go through sysctl plugin, not direct)
- [ ] No unintended coupling (conntrack config is extracted independently, sends to sysctl via EventBus)
- [ ] No duplicated functionality (reuses sysctl plugin for writes, telemetry collector for stats)
- [ ] Zero-copy preserved where applicable

## Dual-Setting Prevention

When `system conntrack` manages a sysctl key (e.g. `nf_conntrack_max`), the same key in the
`sysctl { setting { ... } }` config block must be rejected at validation time. Implementation:
conntrack registers its managed key set in `internal/core/sysctl/` (a leaf package safe to import
from both the system config verifier and the sysctl plugin verifier). The sysctl plugin's
`verifySysctlConfig` checks incoming keys against the managed set and rejects with:
`"nf_conntrack_max is managed by system conntrack table-size; remove it from sysctl config"`

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| config commit with conntrack block | -> | system conntrack apply | `test/system/conntrack.ci` |
| module list in config | -> | modprobe dispatch | `TestConntrackModuleLoad` |
| table-size in config | -> | sysctl plugin config-apply | `TestConntrackTableSize` |
| timeout values in config | -> | sysctl plugin config-apply | `TestConntrackTimeouts` |
| tcp behavior in config | -> | sysctl plugin config-apply | `TestConntrackTCPBehavior` |
| global flags (accounting, timestamp) | -> | sysctl plugin config-apply | `TestConntrackGlobalFlags` |
| `show system conntrack` CLI | -> | live state reader | `TestShowConntrack` |
| duplicate key in sysctl config | -> | config validation rejects | `TestConntrackDualSettingPrevention` |
| telemetry scrape | -> | conntrack collector | existing `conntrack_linux.go` tests |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `conntrack { module ftp; module sip; }` | Config parses; module names validated against compile-time allowlist (ftp, h323, sip, pptp, tftp, sane, irc, amanda, netbios-ns, snmp) |
| AC-2 | Config commit with module list | `modprobe nf_conntrack_ftp` (and each module) executed on Linux |
| AC-3 | Config with `table-size 262144` | `nf_conntrack_max=262144` written via sysctl plugin config layer |
| AC-4 | Config with `timeout tcp { established 432000; close-wait 60; }` | Corresponding sysctl keys written via sysctl plugin |
| AC-5 | Unknown module name in config | Config validation rejects with descriptive error |
| AC-6 | Module removed from config (diff) | No runtime action. Module remains loaded until reboot. Not loaded on next boot |
| AC-7 | On gokrazy (no modprobe) | Module load skipped gracefully, sysctl tuning still applied |
| AC-8 | Config with `tcp { be-liberal true; loose false; }` | Corresponding sysctl keys written via sysctl plugin |
| AC-9 | Config with `accounting` and `timestamp` | `nf_conntrack_acct=1` and `nf_conntrack_timestamp=1` written via sysctl plugin |
| AC-10 | `nf_conntrack_max` also set in `sysctl { setting { ... } }` | Config validation rejects with message: "managed by system conntrack table-size" |
| AC-11 | `show system conntrack` CLI command | Displays: current entry count, max table size, loaded modules, per-protocol timeout values, TCP behavior flags |
| AC-12 | Telemetry scrape with conntrack config active | Conntrack gauges include configured max alongside existing per-CPU counters |
| AC-13 | Config with timeout for all supported protocols (tcp, udp, icmp, icmpv6, gre, sctp, dccp) | All corresponding sysctl keys written correctly |
| AC-14 | Config with `hash-size` and `expect-max` | `nf_conntrack_buckets` and `nf_conntrack_expect_max` written via sysctl plugin |
| AC-15 | Config with `log-invalid tcp` | `nf_conntrack_log_invalid=6` (TCP protocol number) written via sysctl plugin |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConntrackConfigParse` | `internal/component/config/system/conntrack_test.go` | Config extraction from tree (modules, table-size, timeouts, flags) | |
| `TestConntrackConfigParseFromMap` | `internal/component/config/system/conntrack_test.go` | Reload-path extraction from map[string]any | |
| `TestConntrackModuleValidation` | `internal/component/config/system/conntrack_test.go` | Known module names accepted, unknown rejected | |
| `TestConntrackSysctlMapping` | `internal/component/config/system/conntrack_test.go` | Friendly names mapped to correct sysctl key/value pairs | |
| `TestConntrackTableSize` | `internal/component/config/system/conntrack_test.go` | table-size -> nf_conntrack_max mapping | |
| `TestConntrackTimeouts` | `internal/component/config/system/conntrack_test.go` | Nested timeout config -> sysctl keys for all protocols | |
| `TestConntrackTCPBehavior` | `internal/component/config/system/conntrack_test.go` | tcp { be-liberal; loose; } -> sysctl keys | |
| `TestConntrackGlobalFlags` | `internal/component/config/system/conntrack_test.go` | accounting, timestamp, log-invalid, checksum -> sysctl keys | |
| `TestConntrackDualSettingPrevention` | `internal/plugins/sysctl/sysctl_test.go` | Managed key set rejects duplicate config | |
| `TestShowConntrack` | `internal/component/config/system/conntrack_test.go` | Show command returns structured state | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-conntrack-modules` | `test/system/conntrack.ci` | Commit conntrack config, verify modules loaded and sysctl applied | |
| `test-conntrack-show` | `test/system/conntrack.ci` | Show command displays live conntrack state | |

## Files to Modify
- `internal/component/config/system/schema/ze-system-conf.yang` - add conntrack container (modules, sizing, timeouts, tcp behavior, global flags)
- `internal/component/config/system/system.go` - add ConntrackConfig to SystemConfig struct, call extractConntrack
- `internal/plugins/sysctl/sysctl.go` - add managed key set registration and conflict detection in applyConfig
- `internal/component/telemetry/collector/conntrack_linux.go` - add configured-max gauge
- `cmd/ze/hub/main.go` - startup wiring (applyConntrack/applyConntrackFromMap pattern) for conntrack apply

## Files to Create
- `internal/component/config/system/conntrack.go` - config extraction, sysctl key mapping, show command handler
- `internal/component/config/system/conntrack_linux.go` - Linux modprobe backend (load-only)
- `internal/component/config/system/conntrack_other.go` - non-Linux stub (no-op modprobe)
- `internal/component/config/system/conntrack_test.go` - unit tests
- `test/system/conntrack.ci` - functional test

## Implementation Steps

### Implementation Phases

1. **Phase: YANG schema** - Add conntrack container to ze-system-conf.yang (module leaf-list, sizing, nested timeout containers per protocol, tcp behavior, global flags)
2. **Phase: Config parsing** - ExtractConntrackConfig (from Tree) + ExtractConntrackFromMap (reload path). Sysctl key mapping table.
3. **Phase: Sysctl integration** - Managed key set registration in sysctl plugin. Conntrack config values emitted via EventBus (sysctl, default) with source "system-conntrack". Dual-setting conflict detection in verifySysctlConfig.
4. **Phase: Module management** - modprobe load-only on commit (Linux). No-op stub for darwin/other. Gokrazy graceful skip.
5. **Phase: Show command** - `show system conntrack`: entry count, max, loaded modules, timeouts, tcp flags. Reads live sysctl values + /proc state.
6. **Phase: Telemetry** - Add configured-max gauge to existing conntrack collector.
7. **Phase: Functional tests** - End-to-end with config commit verification, show command, dual-setting rejection.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All ACs have implementation |
| Correctness | Module names match actual kernel module names (nf_conntrack_ftp, not nf_conntrack_sqlnet) |
| Naming | Sysctl key paths match kernel expectations exactly |
| Data flow | Config commit triggers apply via sysctl plugin, not direct writes |
| Dual-setting | Managed key conflict detection tested |
| Platform | conntrack_other.go exists, build succeeds on darwin |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Module names from compile-time allowlist only; table-size within sane range; timeout values bounded |
| Command injection | Module names passed to modprobe must be validated against allowlist, no shell metacharacters possible |
| Privilege | modprobe requires root or CAP_SYS_MODULE |
| No unload | Verify modprobe -r is never called anywhere in the implementation |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| table-size | 1-2097152 | 2097152 | 0 | 2097153 |
| hash-size | 1-2097152 | 2097152 | 0 | 2097153 |
| expect-max | 1-65535 | 65535 | 0 | 65536 |
| timeout generic | 1-604800 | 604800 | 0 | 604801 |
| timeout tcp established | 1-604800 | 604800 | 0 | 604801 |
| timeout udp timeout | 1-604800 | 604800 | 0 | 604801 |
| tcp max-retrans | 1-255 | 255 | 0 | 256 |

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | `internal/component/config/system/schema/ze-system-conf.yang` |
| CLI commands/flags | Yes | Show command wired via plugin dispatch |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test | Yes | `test/system/conntrack.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add conntrack management |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - add system conntrack block, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - add show system conntrack |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | Sysctl plugin modified but interface unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/conntrack.md` - new page for conntrack management |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - conntrack management vs VyOS/FRR |
| 12 | Internal architecture changed? | No | Follows existing system config pattern |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG conntrack container | `grep -n 'container conntrack' internal/component/config/system/schema/ze-system-conf.yang` |
| ConntrackConfig struct in SystemConfig | `grep -n 'ConntrackConfig\|Conntrack' internal/component/config/system/system.go` |
| Config extraction (Tree + Map) | `grep -n 'ExtractConntrack\|extractConntrack' internal/component/config/system/conntrack.go` |
| Sysctl key mapping table | `grep -n 'nf_conntrack' internal/component/config/system/conntrack.go` |
| Module allowlist | `grep -n 'allowedModules\|validModules' internal/component/config/system/conntrack.go` |
| Linux modprobe backend | `ls internal/component/config/system/conntrack_linux.go` |
| Non-Linux stub | `ls internal/component/config/system/conntrack_other.go` |
| Managed key set in sysctl | `grep -n 'managed\|Managed' internal/plugins/sysctl/sysctl.go` |
| Dual-setting prevention | `grep -rn 'managed by system conntrack' internal/plugins/sysctl/` |
| Startup wiring | `grep -n 'applyConntrack' cmd/ze/hub/main.go` |
| Reload wiring | `grep -n 'applyConntrackFromMap' cmd/ze/hub/main.go` |
| Show command handler | `grep -n 'show system conntrack\|ShowConntrack' internal/component/config/system/conntrack.go` |
| Telemetry configured-max gauge | `grep -n 'configured.*max\|conntrack_max' internal/component/telemetry/collector/conntrack_linux.go` |
| Unit tests | `ls internal/component/config/system/conntrack_test.go` |
| Functional test | `ls test/system/conntrack.ci` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] Wiring Test table complete, every row has a concrete test name
- [ ] `/ze-review` gate clean (Review Gate section filled, 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass)
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Implementation Audit complete
- [ ] Boundary tests for all numeric inputs

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
