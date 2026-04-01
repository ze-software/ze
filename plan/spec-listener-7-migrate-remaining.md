# Spec: listener-7-migrate-remaining

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/migration/migrate.go` - migration pipeline
4. `internal/component/config/migration/listener.go` - existing listener transformations
5. `cmd/ze/exabgp/main.go` - ExaBGP migrate subcommand

## Task

Implement remaining listener migration transformations and ExaBGP env file migration:

1. **Log booleans to subsystems** -- `ze config migrate` converts `environment { log { packets true; } }` to `environment { log { bgp.packets debug; } }` (true->debug, false->disabled)
2. **Listener structural migration** -- `ze config migrate` converts old flat `host`+`port` format to `enabled true; server main { ip ...; port ...; }` for web, ssh, mcp, looking-glass, telemetry containers
3. **ExaBGP env file migration** -- `ze exabgp migrate --env` reads ExaBGP INI env files and produces Ze config. Listener keys (tcp.bind, tcp.port) emit comments. Environment keys map to Ze sections. Per-topic booleans map to subsystem levels.

**Origin:** Deferred from spec-listener-4-migrate (AC-5, AC-6-9) and spec-listener-0-umbrella. Tracked in plan/learned/503-listener-0-umbrella.md.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config parsing pipeline, migration transformations
  -> Constraint: migrations operate on the parsed config tree, not raw text
- [ ] `plan/learned/503-listener-0-umbrella.md` - ExaBGP topic-to-Ze subsystem mapping table

### RFC Summaries (MUST for protocol work)
- N/A

**Key insights:**
- Migration transformations are detect+apply pairs registered in migrate.go
- ExaBGP env is INI format (Python configparser): `[exabgp.<section>]` sections, `key = value`
- Only non-default ExaBGP values should be emitted (Ze has own YANG defaults)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/migration/migrate.go` - existing pipeline with 5 listener transformations
- [ ] `internal/component/config/migration/listener.go` - remove-bgp-listen, remove-tcp-port, remove-env-bgp-connect, remove-env-bgp-accept, hub-server-host-to-ip
- [ ] `cmd/ze/exabgp/main.go` - ExaBGP migrate subcommand, config file only (no --env support)

**Behavior to preserve:**
- Existing migration transformations continue to work
- `ze exabgp migrate` config file conversion unchanged when `--env` not provided
- Migration output format (Ze config syntax)

**Behavior to change:**
- Add log-booleans-to-subsystems transformation
- Add listener-to-list structural transformation (host+port -> server list)
- Add `--env` flag to `ze exabgp migrate` for INI env file parsing

## Data Flow (MANDATORY)

### Entry Point
- `ze config migrate`: Ze config file parsed into tree
- `ze exabgp migrate --env`: ExaBGP INI env file (raw text)

### Transformation Path
1. ze config migrate: tree -> detect old patterns -> apply transformations -> serialized output
2. ze exabgp migrate --env: INI parse -> mapping tables -> Ze config entries + comments

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| INI file -> Ze config tree | ExaBGP env parser | [ ] |
| Migration pipeline -> config tree | Transformations modify tree | [ ] |

### Integration Points
- `internal/component/config/migration/migrate.go` - register new transformations
- `cmd/ze/exabgp/main.go` - add --env flag

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config migrate` on config with boolean log leaves | -> | log-booleans-to-subsystems | `test/parse/config-migrate-log-booleans.ci` |
| `ze config migrate` on config with old listener format | -> | listener-to-list | `test/parse/config-migrate-listener-to-list.ci` |
| `ze exabgp migrate --env` with env file | -> | ExaBGP env parser | `test/parse/exabgp-migrate-env.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze config migrate` on config with `log { packets true; }` | Converted to `log { bgp.packets debug; }` |
| AC-2 | `ze config migrate` on config with `log { packets false; }` | Converted to `log { bgp.packets disabled; }` |
| AC-3 | `ze config migrate` on config with `web { host 0.0.0.0; port 3443; }` | Converted to `web { enabled true; server main { ip 0.0.0.0; port 3443; } }` |
| AC-4 | `ze exabgp migrate --env exabgp.env` | Env file parsed, tcp.port/bind emit comments |
| AC-5 | `ze exabgp migrate --env` with `log.packets = true` | Output contains `bgp.packets debug` |
| AC-6 | `ze exabgp migrate --env` with `debug.pdb = true` | Output contains comment about Python-only |
| AC-7 | `ze config migrate --list` | Shows new transformations |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLogBooleansToSubsystems` | `internal/component/config/migration/listener_test.go` | Boolean log topics converted to subsystem level syntax | |
| `TestListenerToList` | `internal/component/config/migration/listener_test.go` | Flat host+port converted to server list | |
| `TestParseExaBGPEnv` | `internal/exabgp/migration/env_test.go` | INI file parsed into section/key/value triples | |
| `TestEnvListenerMapping` | `internal/exabgp/migration/env_test.go` | tcp.bind/port produce comments | |
| `TestEnvLogMapping` | `internal/exabgp/migration/env_test.go` | Per-topic booleans mapped to subsystem levels | |
| `TestEnvCommentOnly` | `internal/exabgp/migration/env_test.go` | Unsupported keys produce comments | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| tcp.port value from INI | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-migrate-log-booleans` | `test/parse/config-migrate-log-booleans.ci` | Boolean logs converted to subsystem levels | |
| `config-migrate-listener-to-list` | `test/parse/config-migrate-listener-to-list.ci` | Flat format converted to server list | |
| `exabgp-migrate-env` | `test/parse/exabgp-migrate-env.ci` | ExaBGP env file parsed and mapped | |

### Future (if deferring any tests)
- Property test: round-trip migration (migrate old config, parse result)

## Files to Modify

- `internal/component/config/migration/listener.go` - add log-booleans and listener-to-list transformations
- `internal/component/config/migration/migrate.go` - register new transformations
- `cmd/ze/exabgp/main.go` - add --env flag

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A |
| CLI commands/flags | [x] | `cmd/ze/exabgp/main.go` -- --env flag |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- ExaBGP env migration |
| 2 | Config syntax changed? | [ ] | N/A (migration handles old syntax) |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` -- ze exabgp migrate --env |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/migration.md` -- ExaBGP env file migration |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/exabgp/migration/env.go` - ExaBGP env file parser and Ze config mapper
- `internal/exabgp/migration/env_test.go` - unit tests
- `test/parse/config-migrate-log-booleans.ci`
- `test/parse/config-migrate-listener-to-list.ci`
- `test/parse/exabgp-migrate-env.ci`

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

1. **Phase: Log boolean migration** -- detect boolean log leaves, convert true->debug, false->disabled
   - Tests: TestLogBooleansToSubsystems
   - Files: migration/listener.go, migrate.go
   - Verify: tests fail -> implement -> tests pass
2. **Phase: Listener structural migration** -- detect flat host+port, wrap in server list with enabled true
   - Tests: TestListenerToList
   - Files: migration/listener.go, migrate.go
   - Verify: tests fail -> implement -> tests pass
3. **Phase: ExaBGP INI parser** -- parse INI format into section/key/value triples
   - Tests: TestParseExaBGPEnv
   - Files: internal/exabgp/migration/env.go
   - Verify: tests fail -> implement -> tests pass
4. **Phase: ExaBGP env mapper** -- map INI entries to Ze config output
   - Tests: TestEnvListenerMapping, TestEnvLogMapping, TestEnvCommentOnly
   - Files: internal/exabgp/migration/env.go
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Wire --env flag** -- add flag to ze exabgp migrate
   - Tests: functional test
   - Files: cmd/ze/exabgp/main.go
   - Verify: tests fail -> implement -> tests pass
6. **Functional tests** -- Create .ci tests
7. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation |
| Correctness | Boolean mapping correct (true->debug, false->disabled). INI parsing handles edge cases. |
| Naming | Transformation names match spec |
| No-layering | Old formats transformed, not kept alongside new |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| log-booleans transformation registered | grep "log-booleans" migrate.go |
| listener-to-list transformation registered | grep "listener-to-list" migrate.go |
| --env flag on ze exabgp migrate | grep "env" cmd/ze/exabgp/main.go |
| ExaBGP env parser exists | ls internal/exabgp/migration/env.go |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Port values from INI validated 1-65535; malformed lines rejected |
| Path traversal | --env flag path: standard file open, no special concern |
| Resource exhaustion | Large INI files: set reasonable size limit |

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
- [ ] AC-1..AC-7 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-listener-7-migrate-remaining.md`
- [ ] Summary included in commit
