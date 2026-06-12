# Spec: reactor-tuning-yang

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-06-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/yang/ze-bgp-conf.yang` - existing YANG schema
4. `internal/component/bgp/reactor/reactor.go` - env var usage

## Task

Promote operator-facing reactor env vars to YANG config leaves under
`protocols bgp reactor` (which already exists at ze-bgp-conf.yang:936).

Currently 17+ env vars control reactor behavior but only 4 are in YANG
(speed, cache-ttl, cache-max, update-groups). The rest require env vars,
meaning they are invisible in `show configuration`, not part of
commit/rollback, not validated, not discoverable, and require a restart.

Inspired by VyOS PR vyos/vyos-1x#5268 which exposed FRR's BGP queue
limits as proper config knobs.

### Env vars to promote (operator-tuning)

| Env var | Default | Purpose | YANG leaf name |
|---------|---------|---------|----------------|
| `ze.fwd.chan.size` | 64 | Per-destination forward channel capacity | `forward-queue-size` |
| `ze.fwd.batch.limit` | 1024 | Max items per drain batch | `forward-batch-limit` |
| `ze.fwd.pool.maxbytes` | 0 (auto) | Combined buffer pool byte cap | `forward-pool-max-bytes` |
| `ze.fwd.pool.headroom` | 0 | Extra bytes beyond auto-sized baseline | `forward-pool-headroom` |
| `ze.fwd.teardown.grace` | (code default) | Grace period before dropping on congestion | `forward-teardown-grace` |
| `ze.buf.read.size` | 65536 | TCP read buffer size | `read-buffer-size` |
| `ze.buf.write.size` | 16384 | TCP write buffer size | `write-buffer-size` |
| `ze.rs.chan.size` | 4096 | RS worker channel capacity | (under rs plugin YANG) |

### Env vars to keep as env-only (internal/debug)

| Env var | Reason |
|---------|--------|
| `ze.fwd.write.deadline` | Internal timeout, not operator-facing |
| `ze.fwd.dest.cap` | Safety cap, not tuning |
| `ze.fwd.pool.size` | Overlaps with maxbytes; legacy |
| `ze.cache.safety.valve` | Debug-only emergency override |
| `ze.bgp.announce.delay` | Debug/testing delay |
| `ze.bgp.openwait` | Rarely tuned, RFC-governed |
| `ze.metrics.interval` | Observability plumbing |
| `ze.bgp.reactor.coalesce` | Already registered, feature flag not tuning |

### Behavior

- YANG leaf is authoritative when set (non-default)
- Env var overrides YANG (escape hatch for emergencies)
- Precedence: env var > YANG config > hardcoded default
- Existing env vars remain functional, no breakage

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - reactor design
- [ ] `ai/patterns/config-option.md` - how to add YANG config leaves

### RFC Summaries (MUST for protocol work)
N/A - internal tuning, not protocol behavior.

**Key insights:** (to fill during research)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - reactor container lines 936-943
- [ ] `internal/component/bgp/reactor/reactor.go:330-390` - env var reads at startup
- [ ] `internal/component/bgp/reactor/forward_pool.go:290` - fwdPoolConfig struct
- [ ] `internal/component/bgp/reactor/peer.go:119-122` - DefaultOpQueueSize
- [ ] `internal/component/bgp/reactor/session_connection.go:272-273` - buffer sizes
- [ ] `internal/component/bgp/plugins/rs/server.go:38,193` - RS channel size

**Behavior to preserve:**
- All existing defaults remain the same
- Env vars continue to work (override YANG)
- Forward pool overflow + backpressure logic unchanged
- Auto-sizing from prefix maximums unchanged

**Behavior to change:**
- New YANG leaves under `reactor` container
- Config tree resolution feeds values into reactor/forward pool init

## Data Flow (MANDATORY)

### Entry Point
- Config file or CLI: `protocols bgp reactor forward-queue-size 128`

### Transformation Path
1. YANG schema validates the leaf value
2. Config tree stores it under `bgp/reactor/forward-queue-size`
3. `ResolveBGPTree()` maps tree leaf to reactor config struct field
4. `NewReactor()` reads config struct (falling back to env var, then default)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Reactor | via BGP config struct | [ ] |

### Integration Points
- `internal/component/bgp/reactor/reactor.go` NewReactor() - reads config
- `internal/component/bgp/reactor/forward_pool.go` fwdPoolConfig - receives values
- `internal/component/bgp/plugins/rs/server.go` - RS plugin reads its own config

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Reactor config struct already carries the existing YANG leaves (speed, cache-ttl, etc.) | ze-bgp-conf.yang + ResolveBGPTree | Need to find where it maps | grep ResolveBGPTree for reactor fields | unvalidated |
| A-2 | Adding leaves to reactor container does not affect peer-level config resolution | YANG structure | Wrong scope for config | unit test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Env var + YANG precedence confusion | Operator sets YANG, env var still active, gets unexpected value | Document precedence clearly in YANG description |
| R-2 | Changing defaults for existing deployments | Unlikely since defaults stay the same | Defaults match current env var defaults |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `set protocols bgp reactor forward-queue-size 128` | -> | `fwdPoolConfig.chanSize = 128` | TBD |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `set protocols bgp reactor forward-queue-size 128` in config | Forward pool uses chanSize=128 |
| AC-2 | `set protocols bgp reactor forward-batch-limit 512` in config | Batch limit is 512 |
| AC-3 | `set protocols bgp reactor read-buffer-size 131072` in config | TCP read buffer is 128K |
| AC-4 | `set protocols bgp reactor write-buffer-size 32768` in config | TCP write buffer is 32K |
| AC-5 | No YANG config set, no env var | Hardcoded defaults used (64, 1024, 65536, 16384) |
| AC-6 | YANG set to X, env var set to Y | Env var wins (Y used) |
| AC-7 | `show configuration` includes reactor tuning leaves | Leaves visible in config output |
| AC-8 | YANG validation rejects out-of-range values | e.g., forward-queue-size 0 rejected |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReactorConfigFromYANG` | `internal/component/bgp/reactor/reactor_test.go` | Config struct populated from tree | |
| `TestReactorEnvOverridesYANG` | `internal/component/bgp/reactor/reactor_test.go` | Env var takes precedence | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| forward-queue-size | 1-1000000 | 1000000 | 0 | 1000001 |
| forward-batch-limit | 0-1000000 | 1000000 | N/A (0=unlimited) | 1000001 |
| read-buffer-size | 4096-16777216 | 16777216 | 4095 | 16777217 |
| write-buffer-size | 4096-16777216 | 16777216 | 4095 | 16777217 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-reactor-tuning-config` | `test/decode/` or `test/plugin/` | Config with reactor leaves parses and applies | |

### Interop Tests (MANDATORY for protocol features)
N/A - internal tuning knobs, no wire protocol change.

### Future (if deferring any tests)
- RS plugin YANG leaf (`ze.rs.chan.size`) may be a separate spec since it's plugin-owned

## Files to Modify
- `internal/component/bgp/yang/ze-bgp-conf.yang` - add leaves to reactor container
- `internal/component/bgp/reactor/reactor.go` - read config tree, fall back to env
- `internal/component/bgp/reactor/forward_pool.go` - no change (receives values from reactor)
- `internal/component/bgp/reactor/session_connection.go` - read buffer sizes from config

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [x] | `internal/component/bgp/yang/ze-bgp-conf.yang` |
| YANG validation constraints | [x] | range constraints on each leaf |
| YANG custom validators | [ ] | N/A - native range sufficient |
| CLI commands/flags | [ ] | N/A - config editor handles YANG leaves |
| CLI grammar (action before identifier) | [ ] | N/A |
| Editor autocomplete | [ ] | automatic for numeric leaves |
| Functional test for new RPC/API | [x] | config parse test |
| Pipe completeness | [ ] | N/A |
| Env var registration | [ ] | Existing env vars stay; no new env vars |
| Doctor check for runtime dependencies | [ ] | N/A |
| Prometheus counters/metrics | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - reactor tuning via config |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` if reactor section exists |
| 3-17 | TBD during research | [ ] | |

## Files to Create
- `test/decode/reactor-tuning.ci` or equivalent functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | YANG leaves + config resolution skeleton |
| 4. Implement (TDD) | Phases below |
| 5-14 | Standard /implement flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG leaves + config struct
   - Tests: `TestReactorConfigFromYANG`
   - Files: `ze-bgp-conf.yang`, config resolution
   - Verify: YANG validates, config struct receives values

2. **Phase: Forward pool config** -- Wire YANG values into forward pool
   - Tests: `TestReactorEnvOverridesYANG`
   - Files: `reactor.go`
   - Verify: Config values flow through, env var override works

3. **Phase: Buffer sizes** -- Wire read/write buffer config
   - Tests: buffer size config test
   - Files: `session_connection.go`
   - Verify: buffer sizes configurable

4. **Functional tests** -- End-to-end config parse
5. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Precedence: env > YANG > default everywhere |
| Naming | YANG leaves use kebab-case, match reactor naming |
| Data flow | Config -> struct -> pool init, no shortcuts |
| YANG validation | Every leaf has range constraint |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| YANG leaves in ze-bgp-conf.yang | grep reactor ze-bgp-conf.yang |
| Config resolution in reactor.go | grep config.Forward reactor.go |
| Functional test | ls test/.../reactor-tuning.ci |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | YANG range constraints prevent overflow/underflow |
| Resource exhaustion | Unreasonably large buffer/queue values -- upper bounds in YANG |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Config resolution doesn't pick up leaf | Trace ResolveBGPTree path |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Env var overrides YANG | YANG-only (drop env vars), YANG overrides env | Env vars are the emergency escape hatch; removing them breaks existing deployments. Env > YANG matches the existing `environment/` pattern. |
| Scope to reactor container | Per-peer tuning, global container | These are engine-wide knobs, not per-peer. Reactor container already exists. |
| Exclude RS channel size | Include in same spec | RS plugin owns its own YANG; separate spec respects plugin-self-containment. |

## Known Limitations
- RS plugin channel size (`ze.rs.chan.size`) deferred to a plugin-owned spec
- `ze.fwd.pool.size` (legacy overlap with maxbytes) not promoted -- operators should use maxbytes

## RFC Documentation
N/A - internal tuning, no RFC requirements.

## Implementation Summary

### What Was Implemented
- [to fill]

### Bugs Found/Fixed
- [to fill]

### Documentation Updates
- [to fill]

### Deviations from Plan
- [to fill]

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

### Assumptions Resolved
| ID | Final Status | Evidence |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

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
- [ ] Interop tests N/A (internal tuning)

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/887-reactor-tuning-yang.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-reactor-tuning-yang.md`
