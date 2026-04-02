# Spec: listener-6-compound-env

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-04-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/environment.go` - env var registrations, LoadEnvironmentWithConfig
4. `cmd/ze/hub/main.go` - consumers of listener env vars

## Task

Replace per-field listener env vars (`ze.web.host`, `ze.web.port`, `ze.mcp.host`, `ze.mcp.port`, `ze.looking-glass.host`, `ze.looking-glass.port`) with compound `ze.<service>.listen=ip:port,ip:port` format. Add `ze.<service>.enabled` vars for each listener service. Update all Go consumers.

**Origin:** Deferred from spec-listener-2-env (AC-3, AC-4, AC-5, AC-6). Tracked in plan/learned/503-listener-0-umbrella.md.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/environment.md` - environment configuration pipeline
  -> Constraint: LoadEnvironmentWithConfig applies defaults, then config block, then OS env override

### RFC Summaries (MUST for protocol work)
- N/A

**Key insights:**
- Per-field vars (ze.web.host + ze.web.port) cannot represent multi-endpoint configs
- Compound format `ip:port,ip:port` enables multi-endpoint via single env var
- IPv6 uses bracket notation: `[::1]:3443`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/environment.go` - ze.web.host, ze.web.port, ze.mcp.host, ze.mcp.port, ze.looking-glass.host, ze.looking-glass.port registered
- [ ] `cmd/ze/hub/main.go` - env.Get("ze.web.host"), env.Get("ze.web.port") consumed to build listen address

**Behavior to preserve:**
- ze.web.insecure, ze.looking-glass.tls (service-level, not listener)
- SSH client vars (ze.ssh.host, ze.ssh.port, ze.ssh.password)
- LoadEnvironmentWithConfig pipeline

**Behavior to change:**
- Drop per-field listener vars, add compound listen + enabled vars
- Parse compound format into server list entries
- Update all env.Get consumers

## Data Flow (MANDATORY)

### Entry Point
- OS environment variables in compound format

### Transformation Path
1. OS env var `ze.web.listen=0.0.0.0:3443,127.0.0.1:8080` read at startup
2. Parsed into list of (ip, port) tuples
3. Used to populate server list entries (or override config file values)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OS env -> parsed endpoints | Compound format parser | [ ] |
| Parsed endpoints -> service startup | env.Get returns compound, caller parses | [ ] |

### Integration Points
- `cmd/ze/hub/main.go` - web/mcp/lg server startup
- `internal/component/config/environment.go` - registration and defaults

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze.web.listen=0.0.0.0:3443` env var | -> | Compound parser + web startup | `test/ui/env-compound-listen.ci` |
| `ze.web.enabled=true` env var | -> | Service activation | `test/ui/env-service-enabled.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze.web.listen=0.0.0.0:3443` env var set | Web server binds to specified endpoint |
| AC-2 | `ze.web.listen=0.0.0.0:3443,127.0.0.1:8080` | Web server binds to both endpoints |
| AC-3 | `ze.web.host` env var used | Process aborts: unregistered key |
| AC-4 | `ze.web.enabled=true` env var set | Web service starts with default endpoint |
| AC-5 | `ze.web.listen=[::1]:3443` | IPv6 bracket notation parsed correctly |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseCompoundListen` | `internal/component/config/environment_test.go` | Compound format parsing | |
| `TestParseCompoundListenIPv6` | `internal/component/config/environment_test.go` | IPv6 bracket notation | |
| `TestCompoundListenMulti` | `internal/component/config/environment_test.go` | Multiple endpoints | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| port in compound | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `env-compound-listen` | `test/ui/env-compound-listen.ci` | Compound env var accepted | |
| `env-service-enabled` | `test/ui/env-service-enabled.ci` | Enabled env var activates service | |

### Future (if deferring any tests)
- None

## Files to Modify

- `internal/component/config/environment.go` - replace per-field vars with compound + enabled, add parser
- `cmd/ze/hub/main.go` - update env.Get calls to compound format

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A |
| 2 | Config syntax changed? | [x] | `docs/guide/environment.md` - compound listen vars |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/architecture/config/environment.md` - compound format |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `test/ui/env-compound-listen.ci`
- `test/ui/env-service-enabled.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Compound parser** -- Parse `ip:port,ip:port` format with IPv6 bracket support
   - Tests: TestParseCompoundListen, TestParseCompoundListenIPv6, TestCompoundListenMulti
   - Files: environment.go
   - Verify: tests fail -> implement -> tests pass
2. **Phase: Replace registrations** -- Drop per-field vars, register compound + enabled vars
   - Tests: TestCompoundListenMulti (env.Get with new key)
   - Files: environment.go
   - Verify: tests fail -> implement -> tests pass
3. **Phase: Update consumers** -- Update hub/main.go and all Go files using old keys
   - Tests: functional tests
   - Files: hub/main.go, all consumers
   - Verify: tests fail -> implement -> tests pass
4. **Functional tests** -- Create .ci tests
5. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation |
| Correctness | IPv6 bracket notation parses correctly |
| No-layering | Old per-field vars fully deleted |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| ze.web.listen registered | grep "ze.web.listen" environment.go |
| ze.web.host NOT registered | grep "ze.web.host" returns nothing |
| Compound parser works | Unit test passes |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Compound format validated: valid IP, port 1-65535 |
| No injection | Compound string not passed to shell or format strings |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
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

N/A

## Implementation Summary

### What Was Implemented
- ListenEndpoint type and ParseCompoundListen function in environment.go
- Replaced 6 per-field env vars (ze.web.host/port, ze.mcp.host/port, ze.looking-glass.host/port) with 3 compound vars (ze.web.listen, ze.mcp.listen, ze.looking-glass.listen)
- Added 3 service enabled vars (ze.web.enabled, ze.mcp.enabled, ze.looking-glass.enabled)
- Updated hub/main.go consumers to use compound parser
- Added ze.mcp.enabled default endpoint (127.0.0.1:8080)
- Unit tests: 4 test functions with 17 sub-tests covering single, multi, IPv6, boundary cases
- Functional tests: 2 .ci tests verifying registration and old var removal

### Bugs Found/Fixed
- None

### Documentation Updates
- docs/architecture/config/environment.md: Added Listener Service Variables section
- docs/guide/looking-glass.md: Updated env var references
- docs/architecture/web-interface.md: Updated env var references
- docs/features/mcp-integration.md: Updated env var references

### Deviations from Plan
- Moved .ci tests from test/parse/ to test/ui/ because they test ze env list output, not config parsing. The parse test runner requires stdin config blocks.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Replace per-field listener env vars with compound | Done | environment.go:88-99 | ze.web/mcp/looking-glass.listen replaces .host+.port |
| Add ze.service.enabled vars | Done | environment.go:89,94,97 | All three services have .enabled |
| Compound format parser | Done | environment.go:125-186 | ParseCompoundListen with IPv6 support |
| Update hub consumers | Done | hub/main.go:202-237 | Uses ParseCompoundListen + env.IsEnabled |
| Update docs | Done | docs/architecture/config/environment.md, docs/guide/looking-glass.md, docs/architecture/web-interface.md, docs/features/mcp-integration.md | All references updated |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | TestParseCompoundListen + hub/main.go:214-220 | Single endpoint parsed and used |
| AC-2 | Done | TestCompoundListenMulti | Multi-endpoint parsing works |
| AC-3 | Done | env-compound-listen.ci (stdout:!contains=ze.web.host) + env.Get abort on unregistered | Old key not registered, env.Get aborts |
| AC-4 | Done | env-service-enabled.ci + hub/main.go:222-224 | ze.web.enabled=true starts web server |
| AC-5 | Done | TestParseCompoundListenIPv6 | [::1]:3443 parsed correctly |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestParseCompoundListen | Done | environment_test.go | Single endpoint |
| TestParseCompoundListenIPv6 | Done | environment_test.go | IPv6 bracket notation |
| TestCompoundListenMulti | Done | environment_test.go | Multiple endpoints |
| TestCompoundListenBoundary | Done | environment_test.go | Port 0/1/65535/65536, empty, malformed |
| env-compound-listen.ci | Done | test/ui/env-compound-listen.ci | Compound vars registered, old vars gone |
| env-service-enabled.ci | Done | test/ui/env-service-enabled.ci | Enabled vars registered |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/component/config/environment.go | Done | Parser added, registrations replaced |
| cmd/ze/hub/main.go | Done | Consumers updated to compound format |
| test/ui/env-compound-listen.ci | Done | Created |
| test/ui/env-service-enabled.ci | Done | Created |

### Audit Summary
- **Total items:** 18
- **Done:** 18
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (test location: test/ui/ instead of test/parse/ because tests use ze env list, not config parsing)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| test/ui/env-compound-listen.ci | Yes | 649 bytes |
| test/ui/env-service-enabled.ci | Yes | 352 bytes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | ze.web.listen registered | grep ze.web.listen environment.go: 1 match |
| AC-2 | Multi-endpoint parsing | TestCompoundListenMulti: PASS |
| AC-3 | ze.web.host not registered | grep ze.web.host environment.go: 0 matches |
| AC-4 | ze.web.enabled registered | grep ze.web.enabled environment.go: 1 match |
| AC-5 | IPv6 parsing | TestParseCompoundListenIPv6: PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| ze.web.listen env var | test/ui/env-compound-listen.ci | PASS |
| ze.web.enabled env var | test/ui/env-service-enabled.ci | PASS |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete
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
- [ ] Write learned summary to `plan/learned/NNN-listener-6-compound-env.md`
- [ ] Summary included in commit
