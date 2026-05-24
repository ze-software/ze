# Spec: Extended Doctor and Runtime Health Checks

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 12/12 |
| Updated | 2026-05-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/doctor/doctor.go` - existing doctor checks
4. `internal/core/health/registry.go` - health registry
5. `internal/core/report/report.go` - report bus (warnings/errors)
6. `internal/core/diagnostic/codes.go` - diagnostic code registry
7. `internal/component/bgp/reactor/session_prefix.go` - existing BGP report bus usage

## Task

Extend `ze doctor` with pre-start semantic checks and add runtime health checks
that surface operational anomalies via the report bus (`show warnings` / `show errors`)
and health registry (`show health`).

The current `ze doctor` covers infrastructure readiness (files, sockets, ports, certs,
kernel modules). It does not validate config semantics or check runtime protocol health.
Runtime health exists only for BGP prefix staleness, prefix limits, and session drops.

This spec adds two categories:

1. **Offline doctor checks** (extend `ze doctor`): config-level semantic validation,
   environment probes (DNS, disk, clock, VPP version).
2. **Runtime health checks** (report bus + health registry): BGP session anomalies,
   RIB-FIB divergence, firewall drift, plugin process liveness, interface errors.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - report bus, health registry, component isolation
  -> Decision: report bus uses (Source, Code, Subject) dedup for warnings, ring buffer for errors
  -> Constraint: health checks must not block >1 second
- [ ] `docs/features/ai-first.md` - diagnostic codes, `ze explain`
  -> Constraint: every new code registered in codes.go with title, description, examples

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - BGP FSM states, hold timer
  -> Constraint: session stuck detection must respect normal FSM transition times
- [ ] `rfc/short/rfc4486.md` - BGP cease notification subcodes
  -> Constraint: prefix limit teardown uses cease/max-prefix-reached

**Key insights:**
- Doctor is offline (no daemon). Runtime checks use report bus + health registry.
- Report bus warnings are state-based (raise/clear). Errors are event-based (raise once).
- Health registry aggregates component status to `/health` endpoint.
- BGP already has prefix-stale, prefix-threshold, notification-sent/received, session-dropped.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/doctor/doctor.go` - 13 checks: config, storage, TLS, VPP, modules, interfaces, plugins, SSH, listeners
- [ ] `cmd/ze/doctor/checks_linux.go` - VPP socket, kernel modules, interface state (Linux-only)
- [ ] `internal/core/diagnostic/codes.go` - 13 doctor codes registered. `doctor-store-integrity` used but NOT registered (bug)
- [ ] `internal/core/health/registry.go` - 3 components: l2tp, report-bus, vpp
- [ ] `internal/core/report/report.go` - state-based warnings, event-based errors, dedup on (Source, Code, Subject)
- [ ] `internal/component/bgp/reactor/session_prefix.go` - prefix-stale, prefix-threshold, notification-*, session-dropped

**Behavior to preserve:**
- Doctor exit codes: 0 = ready (no errors), 1 = not ready
- Doctor output modes: `--json` and text
- Diagnostic severity semantics: error = cannot start, warning = degraded
- Report bus dedup on (Source, Code, Subject)
- Health registry: healthy/degraded/down aggregation
- All existing 13 doctor checks unchanged

**Behavior to change:**
- Add new offline doctor checks (config semantics, environment probes)
- Add new runtime health checks (BGP, FIB, firewall, plugins, interfaces)
- Register missing `doctor-store-integrity` diagnostic code

## Data Flow (MANDATORY)

### Entry Point: Offline Doctor Checks
- `ze doctor` CLI command -> `runChecks()` in `cmd/ze/doctor/doctor.go`
- Config tree already parsed and available

### Entry Point: Runtime Health Checks
- Daemon startup -> component registration -> periodic/event-driven checks
- Results surface via report bus (warnings/errors) and health registry

### Transformation Path

**Offline (doctor):**
1. Parse config tree (existing)
2. New checks read config leaves and validate semantics (cross-references, reachability)
3. Return `[]diagnostic.Diagnostic`

**Runtime (health):**
1. Component registers health check with `health.Register(name, checkFunc)`
2. Component raises/clears warnings on report bus via `report.Raise()`/`report.Clear()`
3. `show warnings` / `show health` surface results to operator

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Doctor | Config tree parsed, checks read leaves | [ ] |
| Component -> Report bus | `report.Raise(source, code, subject, message, detail)` | [ ] |
| Component -> Health registry | `health.Register(name, func)` | [ ] |

### Integration Points
- `cmd/ze/doctor/doctor.go:runChecks()` - add new check functions
- `internal/core/diagnostic/codes.go` - register new diagnostic codes
- `internal/core/report/report.go` - existing raise/clear API
- `internal/core/health/registry.go` - existing register API
- `internal/component/bgp/reactor/` - BGP session monitoring
- `internal/plugins/fib/kernel/` - FIB monitoring
- `internal/component/firewall/` - firewall reconciliation

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze doctor` CLI | -> | `checkConfigReferences()` | `TestDoctorConfigDanglingReference` |
| `ze doctor` CLI | -> | `checkDNS()` | `TestDoctorDNSUnreachable` |
| `ze doctor` CLI | -> | `checkDiskSpace()` | `TestDoctorDiskSpace` |
| BGP session FSM event | -> | `raiseSessionStuck()` | `TestBGPSessionStuckWarning` |
| BGP session state change | -> | `detectFlap()` | `TestBGPSessionFlapDetection` |
| FIB programming result | -> | `raiseFIBSyncFailure()` | `TestFIBSyncFailureWarning` |
| Firewall Apply completion | -> | `auditFirewallTables()` | `TestFirewallStaleTableWarning` |
| Plugin keepalive timeout | -> | plugin health check | `TestPluginLivenessCheck` |
| Interface counter poll | -> | `raiseIfaceErrors()` | `TestInterfaceErrorCounterWarning` |

## Acceptance Criteria

### Part 1: Offline Doctor Checks

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config policy references nonexistent community-list | `doctor-config-reference` error, names the dangling reference |
| AC-2 | Config peer references nonexistent policy | `doctor-config-reference` error, names the dangling reference |
| AC-3 | Configured DNS resolver does not respond | `doctor-dns-resolver` warning |
| AC-4 | zefs partition has <5% free space | `doctor-disk-space` warning with percentage |
| AC-5 | System clock >5 minutes from NTP | `doctor-clock-skew` warning |
| AC-6 | VPP version incompatible with expected API | `doctor-vpp-version` error (Linux only) |
| AC-7 | `doctor-store-integrity` code used but unregistered | Code registered in `codes.go`, `ze explain` works |

### Part 2: Runtime BGP Health

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-8 | Peer configured but stays in Active/Connect >5 minutes | `session-stuck` warning raised on report bus, cleared when peer reaches Established |
| AC-9 | Peer transitions Established->down->Established >3 times in 5 minutes | `session-flap` warning raised, auto-clears after 5 minutes of stability |
| AC-10 | Received prefix count drops >50% in one update cycle for a peer | `route-count-anomaly` error on report bus (event, not state) |
| AC-11 | Peer established but no End-of-RIB received within GR restart-time | `eor-timeout` warning raised, cleared when EOR received |

### Part 3: RIB-FIB Divergence

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-12 | FIB programming returns error for a route | `fib-sync-failure` error on report bus with route prefix and error detail |
| AC-13 | FIB has routes with no corresponding RIB entry after reconciliation sweep | `fib-orphan` warning with count of orphan entries |
| AC-14 | Routes pending FIB install for >30 seconds | `fib-programming-lag` warning with pending count |

### Part 4: Firewall Drift

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-15 | `ze_*` nftables tables exist that do not match current config | `firewall-stale-table` warning with table names |
| AC-16 | External tool modifies `ze_*` rules between Apply cycles | `firewall-drift` warning on next periodic audit |

### Part 5: Infrastructure Health

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-17 | External plugin process exits unexpectedly | `plugin-down` health status (down), `plugin-crash` error on report bus |
| AC-18 | VPP API call times out or returns error | `vpp` health degrades to `degraded` or `down` |
| AC-19 | Interface RX/TX error counters increase | `iface-errors` warning with interface name and counter type |

### Part 6: Health Registry Extensions

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-20 | `show health` | Reports status for all registered components including new ones (bgp, fib, firewall, plugins) |
| AC-21 | `/health` HTTP endpoint | JSON report includes new components, 503 if any down |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDoctorConfigDanglingReference` | `cmd/ze/doctor/doctor_test.go` | Dangling policy/community-list reference detected | |
| `TestDoctorDNSUnreachable` | `cmd/ze/doctor/doctor_test.go` | Unreachable DNS resolver produces warning | |
| `TestDoctorDiskSpace` | `cmd/ze/doctor/doctor_test.go` | Low disk space detected | |
| `TestDoctorStoreIntegrityCodeRegistered` | `internal/core/diagnostic/codes_test.go` | `doctor-store-integrity` code is registered | |
| `TestBGPSessionStuckWarning` | `internal/component/bgp/reactor/session_prefix_test.go` | Warning raised after 5 min non-Established | |
| `TestBGPSessionFlapDetection` | `internal/component/bgp/reactor/session_flap_test.go` | Flap detected after >3 transitions in 5 min | |
| `TestRouteCountAnomaly` | `internal/component/bgp/reactor/session_prefix_test.go` | >50% drop raises error | |
| `TestFIBSyncFailure` | `internal/plugins/fib/kernel/monitor_test.go` | Programming error raises report | |
| `TestFirewallStaleTable` | `internal/component/firewall/audit_test.go` | Orphan ze_* table produces warning | |
| `TestPluginHealthCheck` | `internal/plugin/health_test.go` | Crashed plugin reports down | |
| `TestIfaceErrorCounters` | `internal/component/iface/health_test.go` | Increasing error counters raise warning | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Flap count threshold | 1-100 | 3 (default) | 0 (no detection) | N/A (warn operator) |
| Flap window seconds | 60-3600 | 300 (default) | 59 | 3601 |
| Stuck timeout seconds | 60-3600 | 300 (default) | 59 | 3601 |
| Route drop percentage | 1-100 | 50 (default) | 0 | 101 |
| Disk space threshold % | 1-50 | 5 (default) | 0 | 51 |
| FIB lag timeout seconds | 5-300 | 30 (default) | 4 | 301 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-doctor-reference` | `test/config/*.ci` | `ze doctor` reports dangling config reference | |
| `test-health-bgp-stuck` | `test/bgp/*.ci` | Peer stuck non-Established shows in `show warnings` | |
| `test-health-fib-sync` | `test/bgp/*.ci` | FIB programming failure shows in `show errors` | |
| `test-health-endpoint` | `test/web/*.ci` | `/health` includes BGP component | |

### Interop Tests (MANDATORY for protocol features)
N/A -- this spec adds monitoring/observability, not wire protocol changes.
BGP session detection observes existing FSM; no new wire behavior.

### Future (if deferring any tests)
- RPKI validator reachability doctor check (depends on RPKI component maturity)
- gRPC API health check (depends on gRPC API stabilization)
- L2TP/subscriber runtime health (separate spec, in-progress work)

## Files to Modify
- `cmd/ze/doctor/doctor.go` - add `checkConfigReferences()`, `checkDNS()`, `checkDiskSpace()`
- `cmd/ze/doctor/checks_linux.go` - add `checkClockSkew()`, `checkVPPVersion()`
- `cmd/ze/doctor/doctor_test.go` - tests for new offline checks
- `internal/core/diagnostic/codes.go` - register new codes + fix `doctor-store-integrity`
- `internal/component/bgp/reactor/session_prefix.go` - session-stuck, route-count-anomaly warnings
- `internal/plugins/fib/kernel/monitor.go` - FIB sync failure reporting
- `internal/component/firewall/` - periodic nftables audit integration

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | Existing `show warnings`, `show errors`, `show health` suffice |
| CLI grammar (action before identifier) | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/config/*.ci`, `test/bgp/*.ci` |
| Doctor check for runtime dependencies | Yes | This spec IS the doctor check spec |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - extended doctor checks, runtime health |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | `ze doctor` unchanged interface, new checks documented via `ze explain` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/troubleshooting.md` - health checks section |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - health monitoring differentiator |
| 12 | Internal architecture changed? | No | - |

## Files to Create
- `internal/component/bgp/reactor/session_flap.go` - flap detection (sliding window)
- `internal/component/bgp/reactor/session_flap_test.go` - flap detection tests
- `internal/component/firewall/audit.go` - periodic nftables state audit
- `internal/component/firewall/audit_test.go` - audit tests
- `internal/plugin/health.go` - plugin process health check
- `internal/plugin/health_test.go` - plugin health tests
- `internal/component/iface/health.go` - interface error counter health
- `internal/component/iface/health_test.go` - interface health tests

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: all wiring test names from Wiring Test table
   - Files: register.go skeletons, diagnostic code registration
   - Verify: entry points exist and are reachable; wiring tests fail because feature logic is a stub

2. **Phase: Bugfix -- register doctor-store-integrity code** -- fix existing bug
   - Tests: `TestDoctorStoreIntegrityCodeRegistered`
   - Files: `internal/core/diagnostic/codes.go`
   - Verify: `ze explain doctor-store-integrity` returns metadata

3. **Phase: Offline doctor -- config reference validation** -- dangling policy/community-list detection
   - Tests: `TestDoctorConfigDanglingReference`
   - Files: `cmd/ze/doctor/doctor.go`
   - Verify: config with dangling reference produces `doctor-config-reference` error

4. **Phase: Offline doctor -- environment probes** -- DNS, disk, clock
   - Tests: `TestDoctorDNSUnreachable`, `TestDoctorDiskSpace`
   - Files: `cmd/ze/doctor/doctor.go`, `cmd/ze/doctor/checks_linux.go`
   - Verify: unreachable DNS, low disk, clock skew produce appropriate diagnostics

5. **Phase: Runtime -- BGP session stuck** -- non-Established peer detection
   - Tests: `TestBGPSessionStuckWarning`
   - Files: `internal/component/bgp/reactor/session_prefix.go`
   - Verify: peer in Active >5 min raises warning, reaching Established clears it

6. **Phase: Runtime -- BGP session flap** -- rapid state transition detection
   - Tests: `TestBGPSessionFlapDetection`
   - Files: `internal/component/bgp/reactor/session_flap.go`
   - Verify: >3 transitions in 5 min raises warning, stability clears it

7. **Phase: Runtime -- route count anomaly** -- sudden prefix drop
   - Tests: `TestRouteCountAnomaly`
   - Files: `internal/component/bgp/reactor/session_prefix.go`
   - Verify: >50% drop in received count raises error event

8. **Phase: Runtime -- FIB sync** -- programming failures and orphans
   - Tests: `TestFIBSyncFailure`
   - Files: `internal/plugins/fib/kernel/monitor.go`
   - Verify: FIB error raises report, orphans produce warning

9. **Phase: Runtime -- firewall audit** -- periodic nftables drift detection
   - Tests: `TestFirewallStaleTable`
   - Files: `internal/component/firewall/audit.go`
   - Verify: orphan `ze_*` table produces warning

10. **Phase: Runtime -- plugin liveness** -- crashed plugin detection
    - Tests: `TestPluginHealthCheck`
    - Files: `internal/plugin/health.go`
    - Verify: exited plugin process registers as `down` in health registry

11. **Phase: Runtime -- interface errors** -- counter monitoring
    - Tests: `TestIfaceErrorCounters`
    - Files: `internal/component/iface/health.go`
    - Verify: increasing RX/TX error counters raise warning

12. **Phase: Health registry integration** -- register new components
    - Tests: `TestHealthRegistryNewComponents`
    - Files: component register.go files
    - Verify: `show health` includes bgp, fib, firewall, plugins

13. **Functional tests** -- create after feature works
14. **Full verification** -- `make ze-verify`
15. **Complete spec** -- learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Warning/error severity correctly chosen (state vs event) |
| Naming | Diagnostic codes use `doctor-` prefix for offline, descriptive kebab-case for runtime |
| Data flow | Doctor checks use config tree only, runtime checks use report bus only |
| Report bus | Warnings use raise/clear pattern. Errors are fire-once events. |
| Health checks | All CheckFunc implementations return within 1 second |
| Thresholds | All numeric thresholds have boundary tests |
| Platform | Linux-only checks gated by build tags |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `doctor-store-integrity` code registered | `grep 'doctor-store-integrity' internal/core/diagnostic/codes.go` |
| New doctor codes registered | `grep -c 'doctor-' internal/core/diagnostic/codes.go` count increased |
| BGP session-stuck warning | `grep 'session-stuck' internal/component/bgp/reactor/` |
| BGP flap detection | `ls internal/component/bgp/reactor/session_flap.go` |
| FIB sync failure reporting | `grep 'fib-sync-failure' internal/plugins/fib/` |
| Firewall audit | `ls internal/component/firewall/audit.go` |
| Plugin health | `ls internal/plugin/health.go` |
| Interface health | `ls internal/component/iface/health.go` |
| Health registry components | `grep 'health.Register' internal/component/` |
| All tests pass | `make ze-unit-test` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Doctor DNS check must not follow redirects or accept untrusted responses |
| Resource exhaustion | Flap detection sliding window must be bounded (max entries) |
| DoS via warnings | Report bus dedup prevents warning flood, but verify new producers respect it |
| Timing attacks | Clock skew check must not leak NTP server addresses in diagnostics |
| Firewall audit | Must never modify nftables state, read-only audit |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
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

**Two systems, not one.** `ze doctor` is offline (no daemon, exit code). Runtime health
uses report bus + health registry (daemon running). They share diagnostic code conventions
but not execution paths. Do not try to unify them into one system.

**Report bus severity discipline.** Warnings are state-based (condition exists now, may clear).
Errors are event-based (something happened, no clear). BGP session stuck = warning (clears when
peer comes up). FIB programming failure = error (the failure already happened). Route count
anomaly = error (the drop already happened, even if count recovers).

**Flap detection needs a sliding window.** Cannot use a simple counter because the window
must roll. Use a ring buffer of timestamps. When len(ring) >= threshold and newest-oldest < window,
flap is active. Bounded at threshold+1 entries to prevent memory growth.

**Firewall audit is read-only.** The audit compares nftables state to config. It never
modifies nftables. The Apply path handles reconciliation. The audit detects drift between
Apply cycles.

## RFC Documentation

N/A -- this spec adds observability, not protocol behavior.
BGP session monitoring observes existing FSM; no new RFC requirements.

## Implementation Summary

### What Was Implemented (commit 95353888c)
- AC-7: registered missing `doctor-store-integrity` diagnostic code
- AC-1/AC-2: `checkConfigReferences()` with namespace-aware filter name resolution
- AC-3: `checkDNSResolvers()` with per-server probe and NXDOMAIN-counts-as-responded
- AC-4: `checkDiskSpace()` using syscall.Statfs, <5% threshold
- AC-8: `session-stuck` warning (5-min timer, raise/clear, stopped flag for race safety)
- AC-9: `session-flap` warning (ring buffer, 3 transitions in 5 min)
- 4 new diagnostic codes: doctor-store-integrity, doctor-config-reference, doctor-disk-space, doctor-dns-resolver
- sessionHealth wired into Peer (create, setState, all 3 removal paths)
- 17 new unit tests

### What Was Implemented (session 32579)
- AC-10: `route-count-anomaly` error on report bus when >50% prefix drop in single UPDATE
  - `totalCount()` on `prefixCounts`, anomaly check via defer in `checkPrefixLimits`
  - 3 tests: main case, zero-start, exact-50%-boundary
- AC-11: `eor-timeout` warning when End-of-RIB not received within GR restart-time
  - `startEORTimer(restartSeconds)` + `onEORReceived()` on `sessionHealth`
  - Wired: `peer_run.go` starts timer on Established+GR, `reactor_notify.go` clears on EOR
  - 4 tests: timeout fires, cleared on EOR, cancelled before firing, zero restart-time

### What Was Implemented (session 32579, cont.)
- AC-12: `fib-sync-failure` error on report bus for add/replace/delete failures in `processEvent`
  - 2 tests: add failure, replace failure
- AC-13: `fib-orphan` warning raised during `sweepStale` with orphan count, cleared when none
  - 2 tests: orphan detected, no orphans after refresh
- AC-14: `fib-programming-lag` warning when routes pending >30s (tracked via `pending` map)
  - `trackPendingLocked` records first-failure time, `checkPendingLagLocked` raises/clears
  - 2 tests: lag detected after 31s, cleared after successful install

### What Was Implemented (session 32579, cont.)
- AC-15: `firewall-stale-table` warning via `AuditTables()` comparing kernel ListTables vs LastApplied
  - 1 test: stale table detected
- AC-16: `firewall-drift` warning when chain count differs between kernel and config
  - 1 test: drift detected, 1 test: clean audit
- New file: `internal/component/firewall/audit.go`

### What Was Implemented (session 32579, cont.)
- AC-17: `plugin-crash` error + `plugin-down` warning in ProcessManager.Respawn
  - Error on every crash, warning when respawn limit exceeded, cleared on successful restart
  - 1 test: crash raises error, disabled raises warning
- AC-19: `iface-errors` warning via `CheckInterfaceErrors` / `checkErrorsFromStats`
  - Tracks per-interface error snapshots, raises warning on delta > 0
  - 2 tests: errors detected, cleared when stable
- New files: `internal/component/firewall/audit.go`, `internal/component/iface/health.go`

### What Was Implemented (session 32579, cont.)
- AC-20: health registry extended with bgp, fib, firewall, plugins components
  - `checkBGPHealth`, `checkFIBHealth`, `checkFirewallHealth`, `checkPluginHealth`
  - plugin-down produces StatusDown; others produce StatusDegraded
  - 3 tests: BGP degraded, plugin down, registry aggregation
- AC-21: `/health` endpoint includes new components (via existing handler)
  - Verified: 503 when any component is down
- New file: `internal/component/cmd/show/health_checks.go`

### What Was Implemented (session 32579, final)
- AC-5: `checkClockSkew()` SNTP query to pool.ntp.org, warns at >5 min skew
  - `doctor-clock-skew` diagnostic code registered

### What Was Implemented (session 32579, final cont.)
- AC-6: `checkVPPVersion()` runs `vppctl show version` when VPP backend configured (Linux-only)
  - Stub in checks_other.go for non-Linux. `doctor-vpp-version` diagnostic code registered.
- AC-18: `checkVPPHealth()` probes VPP API socket, returns down/degraded/healthy
  - Registered in health registry as "vpp" component

### What Remains
- All ACs implemented (AC-1 through AC-21)

### Bugs Found/Fixed
- `doctor-store-integrity` code was used in doctor.go but not registered in codes.go (AC-7)

### Documentation Updates
- [pending -- docs update deferred until remaining ACs complete]

### Deviations from Plan
- AC-5 (clock skew) and AC-6 (VPP version) not yet implemented, deferred to next phase

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

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Pre-start config semantic validation | functional test | `test-doctor-reference` |
| BGP session anomaly detection | functional test | `test-health-bgp-stuck` |
| FIB divergence detection | functional test | `test-health-fib-sync` |
| Health registry completeness | functional test | `test-health-endpoint` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

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
- [ ] AC-1..AC-21 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
