# Spec: port-defaults

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-31 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/config-design.md` - env var requirements
4. `internal/core/env/registry.go` - env registration struct and helpers
5. `cmd/ze-chaos/main.go` - chaos flag definitions

## Task

All port values displayed in help text and used as defaults across ze programs are currently hardcoded in CLI flag definitions. They should instead be derived from YANG schema defaults (via env var registrations) so that:

1. Every port has an env var (single runtime source of truth)
2. Every env var for a port carries the YANG default as its Default field
3. Help text shows both the default value and the env var name
4. When the env var is set to a non-default value, help text shows the configured value too
5. This applies to ALL ports in ALL programs (ze, ze-chaos, ze-test, ze-perf)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config pipeline and env var integration
  → Constraint: env vars are the runtime interface to YANG defaults
- [ ] `.claude/rules/config-design.md` - every YANG leaf under environment/ needs env var
  → Constraint: env vars registered via env.MustRegister()
- [ ] `.claude/rules/go-standards.md` - env var access patterns
  → Constraint: use env.Get/env.GetInt, never os.Getenv for ze vars

**Key insights:**
- YANG schemas define canonical defaults for ze service ports
- env.MustRegister captures defaults at package init time
- Several env registrations are missing their YANG defaults (e.g., ze.web.port has no Default but YANG says 3443)
- Tool-specific ports (chaos, test, perf) have no env vars at all
- Tool-specific ports need YANG leaves under appropriate containers

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/env/registry.go` - EnvEntry struct with Key, Type, Default, Description fields
- [ ] `internal/component/config/environment.go` - ze.bgp.* env registrations (ze.bgp.tcp.port default "179")
- [ ] `cmd/ze/hub/main.go` - ze.web.port, ze.looking-glass.port, ze.mcp.port (all missing Default field)
- [ ] `cmd/ze-chaos/main.go` - port flags with hardcoded defaults (1850, 1950, 0, etc.)
- [ ] `cmd/ze-test/bgp.go` - --port 1790 hardcoded
- [ ] `cmd/ze-test/peer.go` - --port reads from env ze_bgp_tcp_port (legacy naming)
- [ ] `cmd/ze-perf/run.go` - --dut-port 179 hardcoded
- [ ] `internal/component/lg/schema/ze-lg-conf.yang` - port default 8443
- [ ] `internal/component/web/schema/ze-web-conf.yang` - port default 3443
- [ ] `internal/component/telemetry/schema/ze-telemetry-conf.yang` - port default 9273
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` - listen leaf-list, no default port
- [ ] `internal/component/mcp/schema/ze-mcp-conf.yang` - no default port
- [ ] `internal/component/hub/schema/ze-hub-conf.yang` - chaos container, debug.pprof, no port defaults for chaos
- [ ] `cmd/ze/signal/main.go` - SSH port default 2222 hardcoded in help text

**Behavior to preserve:**
- Flag names unchanged (--port, --web, --lg, --web-ui, --pprof, etc.)
- Numeric defaults unchanged (just sourced from env instead of hardcoded)
- Programs that don't load YANG (ze-chaos, ze-test) still work standalone

**Behavior to change:**
- Help text shows env var name and default/configured values
- Flag defaults read from env when env var is set
- Missing env var registrations added with YANG defaults
- Missing YANG leaves added for tool-specific ports
- Startup validates no two services bind to the same host:port (conflict detection)

## Data Flow (MANDATORY)

### Entry Point
- Package init: env.MustRegister() captures default from YANG
- Flag definition: reads env.Get() to resolve current value (default or override)
- Help text: helper function formats description with default/configured info

### Transformation Path
1. YANG schema defines canonical default (e.g., `default 8443`)
2. env.MustRegister captures it (e.g., `Default: "8443"`)
3. At flag definition time, helper reads env.Get() (returns configured or default)
4. Helper formats description: `"Looking glass port (default: 8443, env: ze.looking-glass.port)"`
5. If env overrides: `"Looking glass port (default: 8443, configured: 9000 via ze.looking-glass.port)"`
6. Flag's Go default is set to the resolved value (env override or YANG default)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema to env registry | Default value copied at registration | [ ] |
| Env registry to CLI flags | env.Get() at flag definition time | [ ] |
| Env registry to help text | Helper formats description string | [ ] |

### Integration Points
- `internal/core/env/registry.go` - add helper to format port description and resolve default
- All `cmd/*/` flag definitions - use new helper instead of hardcoded defaults
- YANG schemas - add missing leaves for tool ports
- `internal/core/env/port.go` - conflict detection: `CheckListenConflicts()`

### Architectural Verification
- [ ] No bypassed layers (env is the single runtime source of truth)
- [ ] No unintended coupling (helper is in env package, generic)
- [ ] No duplicated functionality (replaces hardcoded defaults, doesn't layer on top)
- [ ] Zero-copy preserved where applicable (N/A - string formatting only)

## Port Conflict Detection

### Problem

Multiple ze services can be configured to the same host:port. Currently there is no validation. ze-chaos has `checkPortFree()` but only checks the BGP port against external processes, not internal conflicts between its own services.

### Rules

Conflicts are per (host, port) pair, not per port alone:
- `127.0.0.1:8443` and `10.0.0.1:8443` do NOT conflict (different host)
- `127.0.0.1:8443` and `127.0.0.1:8443` DO conflict (same host:port)
- `0.0.0.0:8443` and `127.0.0.1:8443` DO conflict (0.0.0.0 binds all interfaces)
- `:::8443` and `::1:8443` DO conflict (:: binds all interfaces)
- `0.0.0.0:8443` and `:::8443` DO conflict (both bind all interfaces, same port)

### Design

A `ListenAddr` registry collects all intended listen endpoints before any service starts. Each entry is a (host, port, service-name) triple. After all services declare their endpoints, `CheckListenConflicts()` returns an error listing all conflicts.

| Function | Signature | Purpose |
|----------|-----------|---------|
| `RegisterListen` | `(service, host string, port int)` | Declare intent to bind |
| `CheckListenConflicts` | `() error` | Detect overlapping endpoints |
| `ResetListens` | `()` | Clear registry (for tests) |

### Conflict Matrix

Two entries conflict when they share a port AND their hosts overlap:

| Host A | Host B | Conflict? |
|--------|--------|-----------|
| same IP | same IP | Yes |
| `0.0.0.0` | any IPv4 | Yes |
| `::` | any IPv6 | Yes |
| `0.0.0.0` | `::` | Yes (same port, both wildcard) |
| different IPs | different IPs | No |
| empty (unset) | any | No (service disabled) |

### Where to Call

| Program | When | What to check |
|---------|------|---------------|
| `cmd/ze/hub/main.go` | After resolving all listen addresses, before starting servers | web, LG, MCP, SSH, pprof, telemetry |
| `cmd/ze-chaos/main.go` | After flag parsing, before starting | BGP port, web dashboard, metrics, pprof, SSH, web-ui, LG |
| `cmd/ze/main.go` | After resolving --web, --mcp, --pprof | web, MCP, pprof |

### Error Format

```
error: port conflicts detected:
  web (0.0.0.0:8443) and looking-glass (0.0.0.0:8443): same host:port
  pprof (0.0.0.0:6060) and metrics (0.0.0.0:6060): same host:port
```

## Port Inventory

### Ze Service Ports (already in YANG)

| Service | YANG leaf | YANG default | Env var | Env Default | Gap |
|---------|-----------|-------------|---------|-------------|-----|
| BGP | `bgp.tcp.port` | 179 | `ze.bgp.tcp.port` | "179" | None |
| Web UI | `environment.web.port` | 3443 | `ze.web.port` | (none) | Missing Default |
| Web UI host | `environment.web.host` | 0.0.0.0 | `ze.web.host` | (none) | Missing Default |
| Looking Glass | `environment.looking-glass.port` | 8443 | `ze.looking-glass.port` | (none) | Missing Default |
| LG host | `environment.looking-glass.host` | 0.0.0.0 | `ze.looking-glass.host` | (none) | Missing Default |
| Prometheus | `telemetry.prometheus.port` | 9273 | (none) | - | Missing env var |
| Prometheus host | `telemetry.prometheus.address` | 0.0.0.0 | (none) | - | Missing env var |
| MCP | `environment.mcp.port` | (none) | `ze.mcp.port` | (none) | No YANG default (required) |
| MCP host | `environment.mcp.host` | 127.0.0.1 | `ze.mcp.host` | (none) | Missing Default |
| SSH | `environment.ssh.listen` | (none) | `ze.ssh.port` | (none) | No YANG default (leaf-list) |
| pprof | `environment.debug.pprof` | (none) | `ze.bgp.debug.pprof` | (none) | No default (opt-in) |

### Tool-Specific Ports (need YANG + env)

| Tool | Flag | Current default | Proposed env var | Proposed YANG location |
|------|------|----------------|------------------|----------------------|
| ze-chaos | `--port` | 1850 | `ze.chaos.bgp.port` | `environment.chaos.bgp-port` (ze-hub-conf.yang) |
| ze-chaos | `--listen-base` | 1950 | `ze.chaos.listen.port` | `environment.chaos.listen-port` (ze-hub-conf.yang) |
| ze-chaos | `--web` | (disabled) | `ze.chaos.web` | `environment.chaos.web` (ze-hub-conf.yang) |
| ze-chaos | `--ssh` | 0 (disabled) | `ze.chaos.ssh.port` | `environment.chaos.ssh-port` (ze-hub-conf.yang) |
| ze-chaos | `--pprof` | (disabled) | `ze.chaos.pprof` | `environment.chaos.pprof` (ze-hub-conf.yang) |
| ze-chaos | `--metrics` | (disabled) | `ze.chaos.metrics` | `environment.chaos.metrics` (ze-hub-conf.yang) |
| ze-chaos | `--ze-pprof` | (disabled) | (reuse ze.bgp.debug.pprof) | (already in ze-hub-conf.yang) |
| ze-chaos | `--web-ui` | 0 (disabled) | (reuse ze.web.port) | (already in ze-web-conf.yang) |
| ze-chaos | `--lg` | 0 (disabled) | (reuse ze.looking-glass.port) | (already in ze-lg-conf.yang) |
| ze-test | `--port` | 1790 | `ze.test.bgp.port` | `environment.test.bgp-port` (ze-hub-conf.yang) |
| ze-perf | `--dut-port` | 179 | (reuse ze.bgp.tcp.port) | (already in ze-bgp-conf.yang) |
| ze signal | `--port` | 2222 | (reuse ze.ssh.port) | (already in ze-ssh-conf.yang) |

### SSH Default Port

The SSH YANG uses a leaf-list for listen addresses (e.g., "0.0.0.0:2222"). There is no single "port" leaf. The env var `ze.ssh.port` exists but has no default. Add Default "2222" to match the hardcoded fallback in `cmd/ze/signal/main.go`.

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-chaos --help` | -> | `env.PortDefault()` formats description | `TestPortDefaultHelp` |
| `ze.chaos.bgp.port=1900 ze-chaos --help` | -> | Help shows "configured: 1900" | `TestPortDefaultEnvOverride` |
| `ze-chaos --port 0` (no flag, env set) | -> | Flag default from env | `TestPortDefaultFlagFromEnv` |
| hub startup with web=8443 and lg=8443 | -> | `CheckListenConflicts()` returns error | `TestListenConflict_SameHostPort` |
| ze-chaos with --web :8443 --lg 8443 | -> | Startup fails with conflict error | chaos port conflict .ci test |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze-chaos --help` | Port flags show env var name and YANG default |
| AC-2 | `ze.chaos.bgp.port=1900` then `ze-chaos --help` | Help shows `default: 1850, configured: 1900 via ze.chaos.bgp.port` |
| AC-3 | `ze.chaos.bgp.port=1900` then `ze-chaos` (no --port flag) | ze-chaos uses port 1900 |
| AC-4 | `ze.chaos.bgp.port=1900` then `ze-chaos --port 2000` | CLI flag wins, port 2000 used |
| AC-5 | Every port env var has Default matching YANG | `env.Registered()` defaults match YANG schemas |
| AC-6 | `ze-test bgp --help` | Shows env var and default for --port |
| AC-7 | `ze-perf --help` | Shows env var and default for --dut-port |
| AC-8 | Every tool port has a YANG leaf | YANG schemas contain leaves for chaos.*, test.* ports |
| AC-9 | `ze env list` shows all port env vars | All new env vars visible in output |
| AC-10 | Two services on same host:port (e.g., web and LG both on 0.0.0.0:8443) | Startup fails with clear error naming both services |
| AC-11 | Two services on same port but different hosts (127.0.0.1:8443 and 10.0.0.1:8443) | No conflict, both start |
| AC-12 | One service on 0.0.0.0:8443 and another on 127.0.0.1:8443 | Conflict detected (0.0.0.0 binds all interfaces including 127.0.0.1) |
| AC-13 | One service on :::8443 and another on 0.0.0.0:8443 | Conflict detected (both wildcard, same port) |
| AC-14 | Disabled service (port 0 or empty addr) | Not registered, no conflict possible |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPortDefault_NoEnv` | `internal/core/env/port_test.go` | Returns YANG default, description includes env var name | |
| `TestPortDefault_EnvOverride` | `internal/core/env/port_test.go` | Returns env value, description shows "configured: X" | |
| `TestPortDefault_Disabled` | `internal/core/env/port_test.go` | Ports with no default (disabled) show "(disabled)" | |
| `TestPortAddrDefault_NoEnv` | `internal/core/env/port_test.go` | addr:port style shows default and env var | |
| `TestPortAddrDefault_EnvOverride` | `internal/core/env/port_test.go` | addr:port with env override shows configured value | |
| `TestAllPortEnvVarsHaveDefaults` | `internal/core/env/port_test.go` | Every ze.*.port env var has Default matching YANG | |
| `TestListenConflict_SameHostPort` | `internal/core/env/listen_test.go` | Same host:port returns error naming both services | |
| `TestListenConflict_DifferentHost` | `internal/core/env/listen_test.go` | Different hosts, same port: no conflict | |
| `TestListenConflict_WildcardIPv4` | `internal/core/env/listen_test.go` | 0.0.0.0:P conflicts with 127.0.0.1:P | |
| `TestListenConflict_WildcardIPv6` | `internal/core/env/listen_test.go` | :::P conflicts with ::1:P | |
| `TestListenConflict_DualWildcard` | `internal/core/env/listen_test.go` | 0.0.0.0:P conflicts with :::P | |
| `TestListenConflict_Disabled` | `internal/core/env/listen_test.go` | Empty addr or port 0 not registered, no conflict | |
| `TestListenConflict_Multiple` | `internal/core/env/listen_test.go` | Three services on same port: error lists all pairs | |
| `TestListenConflict_NoConflict` | `internal/core/env/listen_test.go` | All different endpoints: nil error | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Port values | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-port-env` | `test/chaos/port-env.ci` | Set env var, verify ze-chaos uses it | |

### Future
- Verify all YANG defaults match env defaults (can be a lint check later)

## Files to Modify

### Env package - add port helpers
- `internal/core/env/registry.go` - add `PortDefault()` and `AddrPortDefault()` helpers

### Env registrations - add Default values
- `cmd/ze/hub/main.go` - add Default to ze.web.port ("3443"), ze.web.host ("0.0.0.0"), ze.looking-glass.port ("8443"), ze.looking-glass.host ("0.0.0.0"), ze.mcp.host ("127.0.0.1")
- `cmd/ze/internal/ssh/client/client.go` - add Default "2222" to ze.ssh.port

### New env registrations
- `internal/component/config/environment.go` or `cmd/ze-chaos/main.go` - register ze.chaos.bgp.port, ze.chaos.listen.port, ze.chaos.web, ze.chaos.ssh.port, ze.chaos.pprof, ze.chaos.metrics
- `cmd/ze-test/bgp.go` - register ze.test.bgp.port
- `internal/component/telemetry/` - register ze.telemetry.prometheus.port, ze.telemetry.prometheus.address

### YANG schemas - add tool port leaves
- `internal/component/hub/schema/ze-hub-conf.yang` - expand chaos container with port leaves, add test container

### CLI flag updates
- `cmd/ze-chaos/main.go` - use PortDefault() for all port flags
- `cmd/ze-test/bgp.go` - use PortDefault() for --port
- `cmd/ze-test/peer.go` - use PortDefault() for --port
- `cmd/ze-perf/run.go` - use PortDefault() for --dut-port
- `cmd/ze/signal/main.go` - use PortDefault() for --port
- `cmd/ze/main.go` - use env for --pprof, --web, --mcp

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaves) | [x] | `internal/component/hub/schema/ze-hub-conf.yang` |
| CLI commands/flags | [x] | All cmd/ files listed above |
| Editor autocomplete | [ ] | YANG-driven (automatic) |
| Functional test for new feature | [x] | `test/chaos/port-env.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - port defaults from env/YANG |
| 2 | Config syntax changed? | [ ] | - |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - updated help text format |
| 4 | API/RPC added/changed? | [ ] | - |
| 5 | Plugin added/changed? | [ ] | - |
| 6 | Has a user guide page? | [ ] | - |
| 7 | Wire format changed? | [ ] | - |
| 8 | Plugin SDK/protocol changed? | [ ] | - |
| 9 | RFC behavior implemented? | [ ] | - |
| 10 | Test infrastructure changed? | [ ] | - |
| 11 | Affects daemon comparison? | [ ] | - |
| 12 | Internal architecture changed? | [ ] | - |

## Files to Create
- `internal/core/env/port.go` - PortDefault() and AddrPortDefault() helpers
- `internal/core/env/port_test.go` - unit tests for port helpers
- `internal/core/env/listen.go` - RegisterListen(), CheckListenConflicts(), ResetListens()
- `internal/core/env/listen_test.go` - unit tests for conflict detection

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix all issues from review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: env helpers** -- Add PortDefault() and AddrPortDefault() to env package
   - Tests: TestPortDefault_NoEnv, TestPortDefault_EnvOverride, TestPortDefault_Disabled, TestPortAddrDefault_*
   - Files: internal/core/env/port.go, internal/core/env/port_test.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: YANG leaves** -- Add chaos and test port leaves to ze-hub-conf.yang
   - Files: internal/component/hub/schema/ze-hub-conf.yang
   - Verify: YANG parses correctly

3. **Phase: env registrations** -- Add missing env vars with YANG defaults, fix existing ones missing Default
   - Tests: TestAllPortEnvVarsHaveDefaults
   - Files: cmd/ze/hub/main.go, cmd/ze-chaos/main.go, cmd/ze-test/bgp.go, cmd/ze/internal/ssh/client/client.go, internal/component/telemetry/
   - Verify: tests pass

4. **Phase: CLI flag updates** -- Replace hardcoded defaults with PortDefault() calls
   - Files: cmd/ze-chaos/main.go, cmd/ze-test/bgp.go, cmd/ze-test/peer.go, cmd/ze-perf/run.go, cmd/ze/signal/main.go
   - Verify: help text shows env var info, flags still work

5. **Phase: conflict detection** -- Add listen conflict registry and checker
   - Tests: TestListenConflict_SameHostPort, _DifferentHost, _WildcardIPv4, _WildcardIPv6, _DualWildcard, _Disabled, _Multiple, _NoConflict
   - Files: internal/core/env/listen.go, internal/core/env/listen_test.go
   - Verify: tests fail -> implement -> tests pass

6. **Phase: wire conflict checks** -- Call CheckListenConflicts() at startup in hub and ze-chaos
   - Files: cmd/ze/hub/main.go, cmd/ze-chaos/main.go, cmd/ze/main.go
   - Verify: conflicting ports rejected with clear error message

7. **Phase: functional tests** -- Add .ci test for env var override
   - Files: test/chaos/port-env.ci
   - Verify: test passes

8. **Full verification** -- `make ze-verify`

9. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every port flag across all programs uses PortDefault() |
| Correctness | YANG defaults match env defaults match old hardcoded values |
| Naming | Env vars use ze.* dot notation consistently |
| Data flow | Flag default resolved at parse time, not cached stale |
| Conflict detection | 0.0.0.0 treated as overlapping all IPv4, :: as overlapping all IPv6 |
| Rule: no-layering | Old hardcoded defaults fully replaced, not layered |
| Rule: config-design | Every YANG environment leaf has matching env var |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| PortDefault() helper exists | `grep -r "func PortDefault" internal/core/env/` |
| All chaos port env vars registered | `grep "ze.chaos" internal/ cmd/` |
| All test port env vars registered | `grep "ze.test" cmd/ze-test/` |
| YANG chaos leaves exist | `grep "chaos" internal/component/hub/schema/ze-hub-conf.yang` |
| Help text shows env var | `go run ./cmd/ze-chaos --help 2>&1 \| grep "env:"` |
| Env defaults match YANG | TestAllPortEnvVarsHaveDefaults passes |
| CheckListenConflicts() exists | `grep -r "func CheckListenConflicts" internal/core/env/` |
| Hub calls CheckListenConflicts | `grep "CheckListenConflicts" cmd/ze/hub/main.go` |
| ze-chaos calls CheckListenConflicts | `grep "CheckListenConflicts" cmd/ze-chaos/main.go` |
| Conflict tests pass | `go test -run TestListenConflict ./internal/core/env/` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Port values still validated 1-65535 after env resolution |
| No privilege escalation | Env override of BGP port 179 still requires privilege for bind |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC |
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
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
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
- [ ] Write learned summary to plan/learned/NNN-port-defaults.md
- [ ] Summary included in commit
