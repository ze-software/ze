# Spec: listener-5-log

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
3. `internal/component/hub/schema/ze-hub-conf.yang` - YANG log leaves to remove
4. `internal/component/config/environment.go` - LogEnv struct, envOptions, MustRegister calls
5. `internal/core/slogutil/slogutil.go` - subsystem logging infrastructure

## Task

Remove 14 ExaBGP legacy boolean log leaves from YANG and Go. Verify that Ze's `ze.log.<subsystem>=<level>` system provides equivalent control for all 12 ExaBGP log topics. Ensure all topics are controllable via environment variables and CLI.

ExaBGP had per-topic boolean controls (`log.packets = true/false`). Ze uses hierarchical subsystem log levels (`ze.log.bgp.packets=debug`). This spec ensures Ze has a subsystem for every ExaBGP topic, removes the legacy boolean leaves, and updates the Go structs.

**Parent spec:** `spec-listener-0-umbrella.md`
**Sibling specs:** `spec-listener-1-yang`, `spec-listener-2-env`, `spec-listener-3-conflict`, `spec-listener-4-migrate`

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config parsing, YANG-driven schema
  -> Constraint: YANG extensions drive parser behavior; `ze:allow-unknown-fields` on log container allows subsystem-specific levels
- [ ] `docs/architecture/config/environment.md` - environment configuration pipeline
  -> Constraint: LoadEnvironmentWithConfig pipeline: defaults -> config block -> OS env override

### RFC Summaries (MUST for protocol work)
N/A -- no RFC behavior involved. This is configuration cleanup.

**Key insights:**
- Log container has `ze:allow-unknown-fields` allowing subsystem paths like `bgp.routes debug;`
- Ze subsystem logging uses hierarchical lookup: `ze.log.bgp=debug` sets all `bgp.*`
- The 14 ExaBGP boolean leaves and Ze subsystem levels coexist but serve the same purpose
- Removing boolean leaves does not break subsystem level control (different mechanism)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/hub/schema/ze-hub-conf.yang` - defines 14 boolean log leaves: enable, all, configuration, reactor, daemon, processes, network, statistics, packets, rib, message, timers, routes, parser, short
- [ ] `internal/component/config/environment.go` - LogEnv struct has 17 fields (Enable, Level, Destination, All, Configuration, Reactor, Daemon, Processes, Network, Statistics, Packets, RIB, Message, Timers, Routes, Parser, Short); envOptions has entries for all 14 booleans; 14 MustRegister calls for boolean fields; 6 SchemaDefaultBool calls in loadDefaults
- [ ] `internal/core/slogutil/slogutil.go` - subsystem logging infrastructure, hierarchical subsystem lookup

**Behavior to preserve:**
- `ze.log` (base level), `ze.log.backend`, `ze.log.destination`, `ze.log.relay` -- these are Ze-native, not ExaBGP legacy
- `ze:allow-unknown-fields` on log container (for subsystem-specific levels)
- Hierarchical subsystem lookup (`ze.log.bgp=debug` sets all `bgp.*`)
- `log.level`, `log.short` -- these have meaning in Ze beyond ExaBGP compat

**Behavior to change:**

### YANG changes (ze-hub-conf.yang)

| Remove leaf | Reason |
|-------------|--------|
| enable | Ze uses `ze.log=disabled` instead of a boolean |
| all | Ze uses `ze.log=debug` instead of a boolean |
| configuration | Replaced by `ze.log.bgp.configuration=<level>` |
| reactor | Replaced by `ze.log.bgp.reactor=<level>` |
| daemon | Replaced by `ze.log.bgp.daemon=<level>` |
| processes | Replaced by `ze.log.bgp.processes=<level>` |
| network | Replaced by `ze.log.bgp.network=<level>` |
| statistics | Replaced by `ze.log.bgp.statistics=<level>` |
| packets | Replaced by `ze.log.bgp.packets=<level>` |
| rib | Replaced by `ze.log.bgp.rib=<level>` |
| message | Replaced by `ze.log.bgp.message=<level>` |
| timers | Replaced by `ze.log.bgp.timers=<level>` |
| routes | Replaced by `ze.log.bgp.routes=<level>` |
| parser | Replaced by `ze.log.bgp.parser=<level>` |

Keep in YANG: level, backend, destination, relay, short, `ze:allow-unknown-fields`

### Go changes

| Change | Detail |
|--------|--------|
| Remove 14 fields from LogEnv struct | Enable, All, Configuration, Reactor, Daemon, Processes, Network, Statistics, Packets, RIB, Message, Timers, Routes, Parser |
| Remove 14 envOptions entries | Same keys |
| Remove 14 MustRegister calls | `ze.bgp.log.enable` through `ze.bgp.log.parser` |
| Remove loadDefaults for boolean log fields | 6 SchemaDefaultBool calls |
| Verify subsystem registration | Each ExaBGP topic must have a `ze.log.bgp.<topic>` subsystem in slogutil |

### ExaBGP topic to Ze subsystem verification

| ExaBGP topic | Expected Ze subsystem | Verify exists |
|-------------|----------------------|---------------|
| configuration | bgp.configuration | check slogutil |
| reactor | bgp.reactor | check slogutil |
| daemon | bgp.daemon | check slogutil |
| processes | bgp.processes | check slogutil |
| network | bgp.network | check slogutil |
| statistics | bgp.statistics | check slogutil |
| packets | bgp.packets | check slogutil |
| rib | bgp.rib | check slogutil |
| message | bgp.message | check slogutil |
| timers | bgp.timers | check slogutil |
| routes | bgp.routes | check slogutil |
| parser | bgp.parser | check slogutil |

Any missing subsystems must be created. The implementation phase must grep for all Logger/LazyLogger calls to find actual subsystem names and verify coverage.

**Actual slogutil subsystem names differ from ExaBGP topic names.** Only `bgp.reactor` and `bgp.routes` currently exist as exact matches. Examples: `bgp.config` (not `bgp.configuration`), `bgp.server` (not `bgp.network`). This spec must:
1. Map each ExaBGP topic to the closest existing Ze subsystem
2. Decide whether to create new subsystems for unmapped topics or map to existing ones
3. Finalize the mapping table before spec-listener-4-migrate can implement the log migration

**Sequencing:** This spec must run after spec-listener-2-env (both modify environment.go/LogEnv).

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config file with `environment { log { ... } }` block
- OS environment variables (`ze.log.*`)

### Transformation Path
1. Config file parsed via YANG schema; boolean log leaves currently accepted; after removal, only `level`, `short`, `backend`, `destination`, `relay` are named leaves; subsystem levels enter via `ze:allow-unknown-fields`
2. ExtractEnvironment() reads log section into `map[string]string`
3. LoadEnvironmentWithConfig() populates LogEnv struct (after removal: Level, Backend, Destination, Short only)
4. OS env vars override via `ze.log.*` registrations
5. slogutil consumes subsystem levels for hierarchical log control

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema -> Parser | Boolean leaves removed; `ze:allow-unknown-fields` preserved for subsystem paths | [ ] |
| Config tree -> Environment extraction | ExtractEnvironment handles reduced log section | [ ] |
| Environment -> slogutil | Subsystem levels passed via `ze.log.<subsystem>=<level>` env vars | [ ] |

### Integration Points
- `config.ExtractEnvironment()` - log section extraction (reduced fields)
- `config.LoadEnvironmentWithConfig()` - LogEnv struct consumption (reduced fields)
- `slogutil` - subsystem registration and hierarchical lookup
- `env.MustRegister()` - env var registration (14 boolean registrations removed)

### Architectural Verification
- [ ] No bypassed layers (subsystem levels flow through same env var mechanism)
- [ ] No unintended coupling (removing boolean leaves does not affect subsystem level handling)
- [ ] No duplicated functionality (removing the duplicate boolean mechanism)
- [ ] Zero-copy preserved where applicable (N/A -- config parsing, not wire encoding)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `environment { log { packets true; } }` (old boolean syntax) | -> | YANG parse rejects unknown leaf | `test/parse/log-boolean-removed.ci` |
| Config with `environment { log { bgp.packets debug; } }` (subsystem level) | -> | allow-unknown-fields accepts, slogutil uses level | `test/parse/log-subsystem-level.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 14 ExaBGP log boolean leaves removed from YANG | grep for legacy leaves in ze-hub-conf.yang returns nothing |
| AC-2 | Config with `environment { log { packets true; } }` | Parse error (leaf removed); use `bgp.packets debug;` instead |
| AC-3 | Config with `environment { log { bgp.packets debug; } }` | Parsed via allow-unknown-fields, sets subsystem level |
| AC-4 | `ze.log.bgp.packets=debug` env var | Packets subsystem logs at debug level |
| AC-5 | All 12 ExaBGP topics have Ze subsystem | `ze env registered` shows `ze.log.<subsystem>` for each |
| AC-6 | LogEnv struct has no boolean fields for topics | Only Level, Backend, Destination, Short remain |
| AC-7 | `log.level` and `log.short` still work | These are Ze-native settings, unchanged |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLogEnvNoBooleanFields` | `internal/component/config/environment_test.go` | LogEnv struct has no boolean topic fields | |
| `TestLogEnvOptionsReduced` | `internal/component/config/environment_test.go` | envOptions for log section has no boolean entries | |
| `TestSubsystemExists` | `internal/core/slogutil/slogutil_test.go` | All 12 ExaBGP topics have Ze subsystem registrations | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no numeric inputs in this spec. Log levels are string enums, not numeric.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-log-boolean-removed` | `test/parse/log-boolean-removed.ci` | Old boolean syntax rejected at parse time | |
| `test-log-subsystem-level` | `test/parse/log-subsystem-level.ci` | New subsystem level syntax accepted and applied | |

### Future (if deferring any tests)
- None -- all tests are in scope

## Files to Modify
- `internal/component/hub/schema/ze-hub-conf.yang` - remove 14 boolean leaves from log container
- `internal/component/config/environment.go` - remove LogEnv boolean fields, envOptions entries, MustRegister calls, loadDefaults calls
- `internal/core/slogutil/slogutil.go` - verify/add subsystem registrations for all 12 ExaBGP topics

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (remove leaves) | [x] | `internal/component/hub/schema/ze-hub-conf.yang` |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [x] | Automatic (YANG-driven; removed leaves disappear from autocomplete) |
| Functional test for removed boolean syntax | [x] | `test/parse/log-boolean-removed.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A -- removal, not addition |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- 14 boolean log leaves removed; use subsystem levels instead |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/environment.md` (if exists) -- removed boolean env vars, subsystem level usage |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A -- using existing subsystem mechanism, just removing legacy |

## Files to Create
- `test/parse/log-boolean-removed.ci` - old boolean syntax rejected at parse time
- `test/parse/log-subsystem-level.ci` - new subsystem level syntax accepted and applied

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

1. **Phase: Verify subsystems** -- Grep all Logger/LazyLogger calls in the codebase; map actual subsystem names; verify all 12 ExaBGP topics have Ze subsystem equivalents in slogutil
   - Tests: `TestSubsystemExists`
   - Files: `internal/core/slogutil/slogutil.go`
   - Verify: identify any missing subsystems

2. **Phase: Create missing subsystems** -- Register any ExaBGP topics that lack Ze subsystem equivalents
   - Tests: `TestSubsystemExists` passes for all 12 topics
   - Files: `internal/core/slogutil/slogutil.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Remove YANG leaves** -- Delete 14 boolean leaves from ze-hub-conf.yang log container; keep level, backend, destination, relay, short, `ze:allow-unknown-fields`
   - Tests: `test-log-boolean-removed` functional test
   - Files: `internal/component/hub/schema/ze-hub-conf.yang`
   - Verify: old boolean syntax rejected at parse time

4. **Phase: Remove Go struct fields** -- Remove 14 boolean fields from LogEnv, 14 envOptions entries, 14 MustRegister calls, 6 SchemaDefaultBool calls in loadDefaults
   - Tests: `TestLogEnvNoBooleanFields`, `TestLogEnvOptionsReduced`
   - Files: `internal/component/config/environment.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Update tests** -- Fix any existing tests that reference boolean log settings; update .ci files that use old boolean syntax
   - Tests: all existing tests that touch log configuration
   - Files: test files referencing boolean log leaves
   - Verify: no test references removed boolean fields

6. **Phase: Functional tests** -- Create `test/parse/log-boolean-removed.ci` and `test/parse/log-subsystem-level.ci`
   - Tests: both .ci files
   - Files: `test/parse/log-boolean-removed.ci`, `test/parse/log-subsystem-level.ci`
   - Verify: functional tests pass end-to-end

7. **Phase: Full verification** -- `make ze-verify`
   - Verify: all tests pass (lint + unit + functional)

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | All 14 boolean leaves removed; all 12 subsystems verified/created |
| Naming | Subsystem names match expected pattern: `bgp.<topic>` |
| Data flow | Subsystem levels still flow through `ze:allow-unknown-fields` -> slogutil |
| Rule: no-layering | Boolean leaves fully deleted from YANG; no boolean fields remain in LogEnv; no fallback to old mechanism |
| Rule: config-design | No boolean log env vars remain registered |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| 14 boolean leaves removed from ze-hub-conf.yang | `grep -c "leaf enable\|leaf all\|leaf configuration\|leaf reactor\|leaf daemon\|leaf processes\|leaf network\|leaf statistics\|leaf packets\|leaf rib\|leaf message\|leaf timers\|leaf routes\|leaf parser" ze-hub-conf.yang` returns 0 |
| LogEnv has no boolean topic fields | grep for `Enable\|All\|Configuration\|Reactor\|Daemon\|Processes\|Network\|Statistics\|Packets\|RIB\|Message\|Timers\|Routes\|Parser` in LogEnv struct returns nothing |
| 12 subsystems registered in slogutil | Test `TestSubsystemExists` passes |
| Old boolean syntax rejected | `test/parse/log-boolean-removed.ci` passes |
| Subsystem level syntax works | `test/parse/log-subsystem-level.ci` passes |
| `log.level` and `log.short` still work | Existing tests for level and short pass |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Subsystem level values validated (debug, info, warn, error, disabled) |
| No information leak | Log level changes do not expose sensitive data in error messages |

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

N/A -- no RFC behavior involved. This is configuration cleanup removing ExaBGP legacy.

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
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-listener-5-log.md`
- [ ] Summary included in commit
