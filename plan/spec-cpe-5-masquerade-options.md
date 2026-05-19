# Spec: Masquerade Port Mapping and Flags

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/9 |
| Updated | 2026-05-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - registration pattern
4. `internal/plugins/firewall/nft/lower_linux.go` - nft backend lowering
5. `internal/component/firewall/model.go` - action types
6. `internal/component/firewall/config.go` - config parsing

## Task

The nft backend explicitly rejects masquerade with port mapping (`to-ports`) and
masquerade flags (`random`, `fully-random`, `persistent`). The model already has
the fields (`Masquerade.Port`, `PortEnd`, `Flags`), the config parser ignores them,
and the nft lowering returns an error. This spec fills the gap across config parsing,
YANG schema, nft backend lowering, CLI display, and VPP backend verification.

### What nftables supports

nftables masquerade accepts:
- **Port mapping**: `masquerade to :1024-65535` restricts the source port range.
  The kernel uses `NFTA_MASQ_REG_PROTO_MIN` / `NFTA_MASQ_REG_PROTO_MAX`.
- **Flags**: `random`, `fully-random`, `persistent`. These map to kernel
  `NF_NAT_RANGE_PROTO_RANDOM`, `NF_NAT_RANGE_PROTO_RANDOM_FULLY`,
  `NF_NAT_RANGE_PERSISTENT`. Port mapping and flags are mutually exclusive in the
  `google/nftables` library (`expr.Masq.ToPorts` gates the branch).

### What VPP supports

VPP NAT44 masquerade marks interfaces as output interfaces. It has no per-rule
port mapping or flag control. The VPP backend should reject these fields (as it
does for SNAT destination matches) rather than silently ignoring them.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - registration pattern, component isolation
  -> Constraint: firewall is a component + plugin pair; model in component, backend in plugin
- [ ] `ai/rules/exact-or-reject.md` - no silent approximation
  -> Constraint: VPP backend must reject unsupported fields, not silently drop them

### RFC Summaries (MUST for protocol work)
N/A (nftables kernel interface, not protocol)

**Key insights:**
- `expr.Masq` has `ToPorts` bool that selects between flags path and port-mapping path
- Port mapping and flags are mutually exclusive at the kernel level
- SNAT/DNAT already handle ports and flags via `lowerNAT`; masquerade needs its own path

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` (L438-443) - `Masquerade{Port, PortEnd, Flags uint32}` already defined
- [ ] `internal/component/firewall/config.go` (L422-424) - parses `masquerade` presence only, ignores sub-fields
- [ ] `internal/component/firewall/schema/ze-firewall-conf.yang` (L283-286) - empty presence container
- [ ] `internal/plugins/firewall/nft/lower_linux.go` (L907-919) - `lowerMasquerade` rejects non-zero Port/Flags
- [ ] `internal/plugins/firewall/nft/lower_linux_test.go` (L510-523) - tests plain masquerade only
- [ ] `internal/plugins/firewall/vpp/nat_linux.go` (L76-81) - ignores Port/Flags on Masquerade action
- [ ] `internal/plugins/firewall/vpp/verify.go` (L126) - matches `firewall.Masquerade` but does not check fields
- [ ] `internal/component/firewall/cmd/show.go` (L179) - displays "masquerade" with no port/flag detail
- [ ] `vendor/github.com/google/nftables/expr/expr.go` (L331-338) - `Masq{Random, FullyRandom, Persistent, ToPorts, RegProtoMin, RegProtoMax}`

**Behavior to preserve:**
- Plain `masquerade {}` (no sub-fields) continues to produce `expr.Masq{}` with no flags
- SNAT/DNAT port and flag handling unchanged
- VPP masquerade still marks all interfaces as NAT44 output

**Behavior to change:**
- Config parser: read optional `to-ports` and `flags` from masquerade container
- YANG schema: add `to-ports` leaf and `random`/`fully-random`/`persistent` leaves
- nft backend: emit `expr.Masq` with port registers and/or flag bits
- VPP backend verify: reject masquerade with port/flags (not supported by VPP NAT44)
- CLI show: display port range and flags when present

## Data Flow (MANDATORY)

### Entry Point
- Config tree: `firewall.table.<name>.chain.<name>.term.<name>.then.masquerade`

### Transformation Path
1. YANG schema validates the config tree (new leaves accepted)
2. `config.go:parseThenActions` reads masquerade sub-map, populates `Masquerade{Port, PortEnd, Flags}`
3. Backend lowering: `lowerMasquerade` emits `expr.Masq` with port registers and/or flags
4. CLI show: `showAction` renders port range and flag names

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Model | `parseThenActions` populates `Masquerade` struct | [ ] |
| Model -> nft backend | `lowerMasquerade` reads struct fields | [ ] |
| Model -> VPP verify | `verifyNATChain` rejects non-zero fields | [ ] |
| Model -> CLI show | `showAction` switch on `Masquerade` | [ ] |

### Integration Points
- `parseThenActions` in `config.go` - existing masquerade presence check
- `lowerMasquerade` in `lower_linux.go` - existing stub rejection
- `verifyNATChain` in `verify.go` - existing Masquerade case
- `showAction` in `cmd/show.go` - existing Masquerade case

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config JSON with masquerade to-ports | -> | `parseThenActions` -> `Masquerade{Port: 1024, PortEnd: 65535}` | `TestParseMasqueradeWithPorts` |
| Config JSON with masquerade random | -> | `parseThenActions` -> `Masquerade{Flags: ...}` | `TestParseMasqueradeWithFlags` |
| `Masquerade{Port: 1024}` | -> | `lowerMasquerade` -> `expr.Masq{ToPorts: true, ...}` | `TestLowerMasqueradeWithPorts` |
| `Masquerade{Flags: random}` | -> | `lowerMasquerade` -> `expr.Masq{Random: true}` | `TestLowerMasqueradeWithFlags` |
| VPP verify with masquerade ports | -> | `verifyNATChain` -> error | `TestVerifyRejectsMasqueradePorts` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config: `masquerade { to-ports "1024-65535"; }` | `Masquerade{Port: 1024, PortEnd: 65535}` parsed |
| AC-2 | Config: `masquerade { to-ports "8080"; }` | `Masquerade{Port: 8080, PortEnd: 0}` (single port) |
| AC-3 | Config: `masquerade { random; }` | `Masquerade{Flags}` has random bit set |
| AC-4 | Config: `masquerade { fully-random; }` | `Masquerade{Flags}` has fully-random bit set |
| AC-5 | Config: `masquerade { persistent; }` | `Masquerade{Flags}` has persistent bit set |
| AC-6 | Config: masquerade with both to-ports and flags | Rejected at parse time (mutually exclusive) |
| AC-7 | nft lower: `Masquerade{Port: 1024, PortEnd: 65535}` | Emits `expr.Masq{ToPorts: true, RegProtoMin, RegProtoMax}` |
| AC-8 | nft lower: `Masquerade{Flags: random}` | Emits `expr.Masq{Random: true}` |
| AC-9 | nft lower: plain `Masquerade{}` | Still emits bare `expr.Masq{}` (regression guard) |
| AC-10 | VPP verify: `Masquerade{Port: 1024}` in NAT chain | Error: "masquerade port mapping not supported by backend vpp" |
| AC-11 | VPP verify: `Masquerade{Flags: random}` in NAT chain | Error: "masquerade flags not supported by backend vpp" |
| AC-12 | CLI show: `Masquerade{Port: 1024, PortEnd: 65535}` | Displays "masquerade to :1024-65535" |
| AC-13 | CLI show: `Masquerade{Flags: random+persistent}` | Displays "masquerade random persistent" |
| AC-14 | Config: `masquerade { to-ports "0"; }` | Rejected (port 0 invalid) |
| AC-15 | Config: `masquerade { to-ports "65535-1024"; }` | Rejected (inverted range) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseMasqueradeWithPorts` | `internal/component/firewall/config_test.go` | AC-1, AC-2 | |
| `TestParseMasqueradeWithFlags` | `internal/component/firewall/config_test.go` | AC-3, AC-4, AC-5 | |
| `TestParseMasqueradePortsAndFlagsMutuallyExclusive` | `internal/component/firewall/config_test.go` | AC-6 | |
| `TestParseMasqueradeInvalidPort` | `internal/component/firewall/config_test.go` | AC-14 | |
| `TestParseMasqueradeInvertedRange` | `internal/component/firewall/config_test.go` | AC-15 | |
| `TestLowerMasqueradeWithPorts` | `internal/plugins/firewall/nft/lower_linux_test.go` | AC-7 | |
| `TestLowerMasqueradeWithFlags` | `internal/plugins/firewall/nft/lower_linux_test.go` | AC-8 | |
| `TestLowerMasqueradePlain` | `internal/plugins/firewall/nft/lower_linux_test.go` | AC-9 (exists) | |
| `TestVerifyRejectsMasqueradePorts` | `internal/plugins/firewall/vpp/verify_test.go` | AC-10 | |
| `TestVerifyRejectsMasqueradeFlags` | `internal/plugins/firewall/vpp/verify_test.go` | AC-11 | |
| `TestShowMasqueradeWithPorts` | `internal/component/firewall/cmd/show_test.go` | AC-12 | |
| `TestShowMasqueradeWithFlags` | `internal/component/firewall/cmd/show_test.go` | AC-13 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Port | 1-65535 | 65535 | 0 | N/A (uint16) |
| PortEnd | 0 or Port..65535 | 65535 | port-1 (inverted) | N/A (uint16) |
| Flags | combination of 3 bits | all three set | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-masquerade-ports` | `test/firewall/masquerade-ports.ci` | Config with to-ports parsed and applied | |
| `test-masquerade-flags` | `test/firewall/masquerade-flags.ci` | Config with random flag parsed and applied | |

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/component/firewall/schema/ze-firewall-conf.yang` - add leaves to masquerade container
- `internal/component/firewall/config.go` - parse masquerade sub-fields
- `internal/component/firewall/config_test.go` - test parsing
- `internal/plugins/firewall/nft/lower_linux.go` - implement `lowerMasquerade` port/flag paths
- `internal/plugins/firewall/nft/lower_linux_test.go` - test lowering
- `internal/plugins/firewall/vpp/verify.go` - reject masquerade with port/flags
- `internal/plugins/firewall/vpp/verify_test.go` - test VPP rejection
- `internal/component/firewall/cmd/show.go` - display port range and flags
- `internal/component/firewall/cmd/show_test.go` - test display

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaves) | Yes | `internal/component/firewall/schema/ze-firewall-conf.yang` |
| CLI commands/flags | No | display only, no new commands |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new config | Yes | `test/firewall/masquerade-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Enhancement to existing masquerade |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (masquerade section) |
| 3 | CLI command added/changed? | No | Display enhancement only |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |

## Files to Create
- `test/firewall/masquerade-ports.ci` - functional test for port mapping config
- `test/firewall/masquerade-flags.ci` - functional test for flag config

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
   - Tests: `TestParseMasqueradeWithPorts`, `TestLowerMasqueradeWithPorts`, `TestVerifyRejectsMasqueradePorts`
   - Files: config.go (stub), lower_linux.go (stub), verify.go (stub)
   - Verify: entry point exists and is reachable; wiring test fails because feature logic is a stub

2. **Phase: YANG schema** -- add leaves to masquerade container
   - Tests: schema compilation (automatic via `all_schemas_test.go`)
   - Files: `ze-firewall-conf.yang`
   - Verify: schema compiles, editor shows new leaves

3. **Phase: Config parsing** -- parse masquerade sub-fields
   - Tests: `TestParseMasqueradeWithPorts`, `TestParseMasqueradeWithFlags`, `TestParseMasqueradePortsAndFlagsMutuallyExclusive`, `TestParseMasqueradeInvalidPort`, `TestParseMasqueradeInvertedRange`
   - Files: `config.go`, `config_test.go`
   - Verify: tests fail then pass

4. **Phase: nft backend lowering** -- emit expr.Masq with ports and flags
   - Tests: `TestLowerMasqueradeWithPorts`, `TestLowerMasqueradeWithFlags`, `TestLowerMasqueradePlain` (regression)
   - Files: `lower_linux.go`, `lower_linux_test.go`
   - Verify: tests fail then pass

5. **Phase: VPP backend rejection** -- reject unsupported fields
   - Tests: `TestVerifyRejectsMasqueradePorts`, `TestVerifyRejectsMasqueradeFlags`
   - Files: `verify.go`, `verify_test.go`
   - Verify: tests fail then pass

6. **Phase: CLI show** -- display ports and flags
   - Tests: `TestShowMasqueradeWithPorts`, `TestShowMasqueradeWithFlags`
   - Files: `cmd/show.go`, `cmd/show_test.go`
   - Verify: tests fail then pass

7. **Functional tests** -- Create after feature works
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- Fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Port byte order matches nftables expectation (big-endian) |
| Correctness | Flags bitmask matches kernel NF_NAT_RANGE_* constants |
| Correctness | Mutual exclusion enforced: ports vs flags cannot coexist |
| Naming | YANG leaves use kebab-case: `to-ports`, `fully-random` |
| Data flow | Config -> Model -> Backend only; no cross-layer shortcuts |
| Rule: exact-or-reject | VPP rejects unsupported fields, never silently drops |
| Rule: no-sprintf-alloc | Error messages use string concat or fmt.Errorf (not hot path) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| YANG masquerade container has to-ports and flag leaves | `grep 'to-ports\|random\|persistent' ze-firewall-conf.yang` |
| Config parser populates Masquerade fields | `go test -run TestParseMasquerade` |
| nft backend emits expr.Masq with ports | `go test -run TestLowerMasqueradeWith` |
| VPP backend rejects masquerade ports/flags | `go test -run TestVerifyRejectsMasquerade` |
| CLI show renders ports and flags | `go test -run TestShowMasquerade` |
| Functional tests exist | `ls test/firewall/masquerade-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Port range 1-65535, no zero; PortEnd >= Port when set |
| Input validation | Flags only accept known bits (random, fully-random, persistent) |
| Resource exhaustion | N/A (config-time only, no runtime allocation) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
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

N/A (kernel interface, not protocol)

## Implementation Summary

### What Was Implemented
- Config parsing: `parseMasquerade` + `parseMasqPorts` in config.go
- Model constants: `MasqFlagRandom`, `MasqFlagFullyRandom`, `MasqFlagPersistent` in model.go
- YANG schema: `to-ports`, `random`, `fully-random`, `persistent` leaves in masquerade container
- nft backend: `lowerMasquerade` with port register and flag boolean paths
- VPP backend: explicit rejection of masquerade ports/flags in `verifyNATChain`
- CLI show: `formatMasquerade` with port range and flag name rendering
- Functional tests: 015-masquerade-ports.ci, 016-masquerade-flags.ci

### Bugs Found/Fixed
- None

### Documentation Updates
- docs/guide/firewall.md: updated masquerade action entry with to-ports and flags syntax

### Deviations from Plan
- None

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
