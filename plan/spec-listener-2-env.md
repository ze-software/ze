# Spec: listener-2-env

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-listener-1-yang |
| Phase | - |
| Updated | 2026-04-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/environment.go` - env struct, MustRegister calls, envOptions table
4. `internal/component/config/environment_extract.go` - ExtractEnvironment section list
5. `cmd/ze/hub/main.go` - 13 scattered MustRegister calls to centralize

## Task

Centralize all env var registrations into the config package, fix registration gaps (ReactorEnv.UpdateGroups, LogEnv.Backend, SSH server leaves, web/ssh/dns/mcp extraction), replace per-field listener vars with compound `ze.<service>.listen` vars and `ze.<service>.enabled` vars, and update all Go consumers.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/environment.md` - environment configuration pipeline
  -> Constraint: LoadEnvironmentWithConfig applies defaults, then config block, then OS env override
  -> Decision: environment.go is the single authority for env var definitions
- [ ] `docs/architecture/config/syntax.md` - YANG-driven config parsing
  -> Constraint: every YANG `environment/<name>` leaf must have a matching env var registration

### RFC Summaries (MUST for protocol work)
- N/A -- no protocol work in this spec

**Key insights:**
- MustRegister calls are scattered across 4+ files; config package already has 48 ze.bgp.* registrations as the pattern to follow
- ExtractEnvironment covers 9 sections but is missing web, ssh, dns, mcp -- values from those config sections are silently lost
- ReactorEnv.UpdateGroups has a MustRegister call in reactor.go but no struct field to receive the value
- LogEnv has no Backend field despite the YANG schema defining a `backend` leaf under `environment > log`
- SSH server YANG leaves (host-key, idle-timeout, max-sessions) exist in the schema but have no env var registrations
- SSH client env vars (ze.ssh.host, ze.ssh.port, ze.ssh.password) are outbound connection settings and must not be renamed or moved

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/environment.go` - Environment struct with 9 section sub-structs (DaemonEnv, LogEnv, TCPEnv, BGPEnv, CacheEnv, APIEnv, ReactorEnv, DebugEnv, ChaosEnv), 48 ze.bgp.* MustRegister calls in init(), envOptions lookup table mapping field names to env keys
  -> Constraint: LoadEnvironmentWithConfig is the central pipeline: apply defaults, overlay config block values, overlay OS env values
  -> Constraint: envOptions table drives which struct fields are populated from which env keys
- [ ] `internal/component/config/environment_extract.go` - ExtractEnvironment walks the config tree and extracts values for 9 sections: daemon, log, tcp, bgp, cache, api, reactor, debug, chaos. Web, ssh, dns, mcp are missing
  -> Constraint: each section produces a map[string]string consumed by LoadEnvironmentWithConfig
- [ ] `cmd/ze/hub/main.go` - 13 MustRegister calls: ze.ready.file, ze.web.{host,port,insecure}, ze.mcp.{host,port}, ze.looking-glass.{host,port,tls}, ze.dns.{server,timeout,cache-size,cache-ttl}
  -> Constraint: non-listener registrations (ze.ready.file, ze.web.insecure, ze.looking-glass.tls, ze.dns.*) move to config package; listener per-field vars (ze.web.host/port, ze.mcp.host/port, ze.looking-glass.host/port) are replaced by compound listen vars
- [ ] `cmd/ze/internal/ssh/client/client.go` - 4 MustRegister calls: ze.config.dir, ze.ssh.{host,port,password} (client-side, NOT server)
  -> Constraint: these are SSH client vars and must remain unchanged in their current location
- [ ] `internal/component/bgp/reactor/reactor.go` - 11 MustRegister calls: ze.fwd.*, ze.cache.*, ze.buf.*, ze.metrics.*
  -> Constraint: key names stay the same, only the registration location moves
- [ ] `internal/component/bgp/plugins/rs/server.go` - 2 MustRegister calls: ze.rs.*
  -> Constraint: key names stay the same, only the registration location moves
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` - SSH server YANG defines host-key, idle-timeout, max-sessions leaves but these have no env var registrations
- [ ] `internal/component/web/schema/ze-web-conf.yang` - web section uses `host` leaf name (will be `ip` after spec-listener-1-yang)
- [ ] `internal/component/mcp/schema/ze-mcp-conf.yang` - MCP section uses `host` leaf name (will be `ip` after spec-listener-1-yang)
- [ ] `internal/component/lg/schema/ze-lg-conf.yang` - looking-glass section uses `host` leaf name (will be `ip` after spec-listener-1-yang)

**Behavior to preserve:**
- SSH client env vars unchanged: ze.ssh.host, ze.ssh.port, ze.ssh.password remain in client.go
- LoadEnvironmentWithConfig pipeline: defaults -> config block -> OS env override
- ze.fwd.*, ze.rs.*, ze.dns.* key names unchanged
- All ze.bgp.* registrations stay in config package (already there)
- env.Get() semantics: registered key returns value, unregistered key causes abort

**Behavior to change:**
- Move MustRegister calls from hub/main.go, reactor.go, rs/server.go into config package (environment.go)
- Drop per-field listener vars: remove ze.web.host, ze.web.port, ze.mcp.host, ze.mcp.port, ze.looking-glass.host, ze.looking-glass.port
- Add compound listen vars: ze.web.listen, ze.mcp.listen, ze.looking-glass.listen, ze.ssh.listen, ze.telemetry.listen (format: `ip:port,ip:port`, IPv6 bracket notation `[::1]:3443`)
- Add enabled vars: ze.web.enabled, ze.mcp.enabled, ze.looking-glass.enabled, ze.ssh.enabled, ze.telemetry.enabled
- Keep service-level vars unchanged: ze.web.insecure, ze.looking-glass.tls
- Register ze.ssh.host-key, ze.ssh.idle-timeout, ze.ssh.max-sessions
- Add ReactorEnv.UpdateGroups bool field + envOptions entry (fix dead registration)
- Add LogEnv.Backend string field + MustRegister + envOptions entry (fix YANG gap)
- Add web, ssh, dns, mcp to ExtractEnvironment sections list
- Update all Go files doing env.Get with old key names to new compound/enabled vars

**Sequencing:** This spec must run before spec-listener-5-log (both modify environment.go/LogEnv).

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config file with `environment` block containing web, ssh, dns, mcp sections
- OS environment variables (ze.web.ip, ze.ssh.server.ip, etc.)

### Transformation Path
1. Config file parsed into Tree via YANG-driven schema
2. Tree -> ExtractEnvironment() -> map per section (now including web, ssh, dns, mcp)
3. map values -> LoadEnvironmentWithConfig() -> Environment struct fields populated
4. Environment struct consumed by hub, reactor, SSH server, web server, etc.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree -> Environment extraction | ExtractEnvironment reads new sections | [ ] |
| Environment struct -> Component consumers | env.Get() with new key names | [ ] |
| Registration -> Validation | Unregistered old key causes abort | [ ] |

### Integration Points
- `config.ExtractEnvironment()` - add web, ssh, dns, mcp sections
- `config.LoadEnvironmentWithConfig()` - consume new section data via envOptions
- `cmd/ze/hub/main.go` - remove MustRegister calls, update env.Get calls to new key names
- `internal/component/bgp/reactor/reactor.go` - remove MustRegister calls
- `internal/component/bgp/plugins/rs/server.go` - remove MustRegister calls
- All Go files referencing ze.web.host, ze.mcp.host, ze.looking-glass.host - update to .ip

### Architectural Verification
- [ ] No bypassed layers (registrations move to config, consumers still use env.Get)
- [ ] No unintended coupling (config package registers on behalf of components, no import cycle)
- [ ] No duplicated functionality (extends existing registration pattern in environment.go)
- [ ] Zero-copy preserved where applicable (N/A -- config values, not wire encoding)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `environment { web { ip 10.0.0.1; } }` | -> | ExtractEnvironment web section | `test/parse/env-web-extract.ci` |
| `ze env registered` output | -> | Centralized MustRegister in config package | `test/parse/env-registered-centralized.ci` |
| ze.web.ip OS env var set | -> | LoadEnvironmentWithConfig populates WebEnv.IP | `test/parse/env-web-ip-override.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze env registered` output | All vars visible, centralized, correct descriptions |
| AC-2 | No MustRegister outside config package | grep returns nothing in hub/main.go, reactor.go, rs/server.go (except sdk, privilege, slogutil, main.go) |
| AC-3 | `ze.web.listen=0.0.0.0:3443` env var set | Web server binds to specified endpoint (compound format) |
| AC-4 | `ze.web.listen=0.0.0.0:3443,127.0.0.1:8080` env var set | Web server binds to both endpoints (multi-endpoint) |
| AC-5 | `ze.web.host` env var used | Process aborts: unregistered key (old var removed) |
| AC-6 | `ze.web.enabled=true` env var set | Web service starts with default endpoint |
| AC-7 | ReactorEnv.UpdateGroups | Config `environment { reactor { update-groups true; } }` populates field |
| AC-8 | LogEnv.Backend | Config `environment { log { backend stderr; } }` populates field |
| AC-9 | ExtractEnvironment with web section in config | Web section extracted into map |
| AC-10 | ExtractEnvironment with ssh section in config | SSH section extracted into map |
| AC-11 | SSH server env vars registered | ze.ssh.listen, ze.ssh.enabled, ze.ssh.host-key, ze.ssh.idle-timeout, ze.ssh.max-sessions visible in `ze env registered` |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractEnvironmentWeb` | `internal/component/config/environment_extract_test.go` | Web section extracted from config tree | |
| `TestExtractEnvironmentSSH` | `internal/component/config/environment_extract_test.go` | SSH section extracted from config tree | |
| `TestExtractEnvironmentDNS` | `internal/component/config/environment_extract_test.go` | DNS section extracted from config tree | |
| `TestExtractEnvironmentMCP` | `internal/component/config/environment_extract_test.go` | MCP section extracted from config tree | |
| `TestReactorEnvUpdateGroups` | `internal/component/config/environment_test.go` | UpdateGroups field populated from config | |
| `TestLogEnvBackend` | `internal/component/config/environment_test.go` | Backend field populated from config | |
| `TestEnvKeyRenameWebIP` | `internal/component/config/environment_test.go` | ze.web.ip registered, ze.web.host not registered | |
| `TestSSHServerEnvRegistered` | `internal/component/config/environment_test.go` | SSH server env vars registered with correct descriptions | |
| `TestNoMustRegisterOutsideConfig` | `internal/component/config/environment_test.go` | Verify centralization by checking all expected keys are registered | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A -- this spec moves registrations and adds struct fields; no new numeric inputs with validation | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `env-registered-centralized` | `test/parse/env-registered-centralized.ci` | `ze env registered` shows all vars centralized with correct names | |
| `env-web-extract` | `test/parse/env-web-extract.ci` | Config with web section values extracted correctly | |
| `env-web-ip-override` | `test/parse/env-web-ip-override.ci` | OS env var ze.web.ip overrides config value | |

### Future (if deferring any tests)
- None -- all tests are in scope

## Files to Modify

- `internal/component/config/environment.go` - centralize all MustRegister calls from hub/main.go, reactor.go, rs/server.go; add WebEnv/SSHServerEnv/DNSEnv/MCPEnv sub-structs; add ReactorEnv.UpdateGroups bool field; add LogEnv.Backend string field; rename host keys to ip keys in envOptions; register SSH server vars
- `internal/component/config/environment_extract.go` - add web, ssh, dns, mcp sections to ExtractEnvironment
- `cmd/ze/hub/main.go` - remove 13 MustRegister calls; update env.Get calls from ze.web.host to ze.web.ip and similar renames
- `internal/component/bgp/reactor/reactor.go` - remove 11 MustRegister calls (key names unchanged, registration moves to config)
- `internal/component/bgp/plugins/rs/server.go` - remove 2 MustRegister calls (key names unchanged, registration moves to config)
- All Go files referencing ze.web.host, ze.mcp.host, ze.looking-glass.host - update to ze.web.ip, ze.mcp.ip, ze.looking-glass.ip

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A -- no new RPCs |
| CLI commands/flags | [ ] | N/A -- `ze env registered` already exists |
| Editor autocomplete | [ ] | N/A -- YANG-driven, automatic |
| Functional test for new RPC/API | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A -- env centralization is internal |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- env var renames (ze.web.host to ze.web.ip, ze.mcp.host to ze.mcp.ip, ze.looking-glass.host to ze.looking-glass.ip) |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/environment.md` (if exists) -- env var renames, new SSH server vars, new extraction sections |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [x] | `docs/architecture/config/environment.md` -- centralized registrations, new section extraction, new struct fields |

## Files to Create

- `test/parse/env-registered-centralized.ci` - functional test verifying `ze env registered` shows centralized vars
- `test/parse/env-web-extract.ci` - functional test for web section extraction
- `test/parse/env-web-ip-override.ci` - functional test for OS env override with new key name

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Centralize MustRegister calls** -- Move all MustRegister calls from hub/main.go, reactor.go, rs/server.go into config/environment.go
   - Tests: `TestNoMustRegisterOutsideConfig`
   - Files: environment.go, hub/main.go, reactor.go, rs/server.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Rename host to ip in env keys** -- Rename ze.web.host to ze.web.ip, ze.mcp.host to ze.mcp.ip, ze.looking-glass.host to ze.looking-glass.ip; update all Go consumers
   - Tests: `TestEnvKeyRenameWebIP`
   - Files: environment.go, hub/main.go, all files referencing old key names
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Add missing registrations** -- Add ReactorEnv.UpdateGroups bool field, LogEnv.Backend string field, SSH server env vars (ze.ssh.server.ip, ze.ssh.server.port, ze.ssh.host-key, ze.ssh.idle-timeout, ze.ssh.max-sessions)
   - Tests: `TestReactorEnvUpdateGroups`, `TestLogEnvBackend`, `TestSSHServerEnvRegistered`
   - Files: environment.go
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Add new sections to ExtractEnvironment** -- Add web, ssh, dns, mcp sections
   - Tests: `TestExtractEnvironmentWeb`, `TestExtractEnvironmentSSH`, `TestExtractEnvironmentDNS`, `TestExtractEnvironmentMCP`
   - Files: environment_extract.go
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Functional tests** -- Create .ci tests for end-user scenarios
   - Tests: `env-registered-centralized.ci`, `env-web-extract.ci`, `env-web-ip-override.ci`
   - Files: test/parse/*.ci
   - Verify: `make ze-functional-test`

6. **Phase: Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)

7. **Phase: Complete spec** -- Fill audit tables, write learned summary to `plan/learned/NNN-listener-2-env.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | All env.Get calls use new key names; no references to old key names remain |
| Naming | Env vars use ze.<service>.ip (not host); SSH server vars namespaced under ze.ssh.server.* |
| Data flow | ExtractEnvironment produces maps for all 13 sections; LoadEnvironmentWithConfig consumes them |
| Rule: no-layering | Old MustRegister calls fully deleted from hub/main.go, reactor.go, rs/server.go; old key names fully removed |
| Rule: compatibility | No compat shims for old env var names (Ze unreleased) |
| No MustRegister outside config | grep for MustRegister in hub/main.go, reactor.go, rs/server.go returns nothing |
| SSH client vars unchanged | ze.ssh.host, ze.ssh.port, ze.ssh.password still registered in client.go and working |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| No MustRegister in hub/main.go | `grep MustRegister cmd/ze/hub/main.go` returns nothing |
| No MustRegister in reactor.go | `grep MustRegister internal/component/bgp/reactor/reactor.go` returns nothing |
| No MustRegister in rs/server.go | `grep MustRegister internal/component/bgp/plugins/rs/server.go` returns nothing |
| ze.web.ip registered | `grep "ze.web.ip" internal/component/config/environment.go` |
| ze.web.host not registered | `grep "ze.web.host" internal/component/config/environment.go` returns nothing |
| ReactorEnv.UpdateGroups field | `grep "UpdateGroups" internal/component/config/environment.go` |
| LogEnv.Backend field | `grep "Backend" internal/component/config/environment.go` |
| SSH server env vars registered | `grep "ze.ssh.server" internal/component/config/environment.go` |
| Web section in ExtractEnvironment | `grep "web" internal/component/config/environment_extract.go` |
| Functional test exists | `ls test/parse/env-registered-centralized.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Env var values validated by YANG types (zt:ip-address, zt:port) -- no raw strings accepted for IP/port |
| No sensitive data in errors | Error messages for unregistered keys do not leak other env var values |
| SSH host-key path | Env var for host-key file path must not allow directory traversal |

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

## RFC Documentation

N/A -- no new RFC behavior in this spec.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-listener-2-env.md`
- [ ] Summary included in commit
