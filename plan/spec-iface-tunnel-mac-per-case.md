# Spec: iface-tunnel-mac-per-case

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-04-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/component/iface/schema/ze-iface-conf.yang` -- grouping split and list tunnel
3. `plan/learned/557-iface-tunnel.md` -- original tunnel spec context
4. `plan/learned/566-iface-wireguard.md` -- where the grouping split was done and this deferral originated

## Task

Move the `mac-address` leaf from the `interface-l2` YANG grouping (which
is applied at the `list tunnel` level, making it visible to all 8 tunnel
encapsulation cases) into the per-case YANG containers for the two L2
tunnel kinds only: `gretap` and `ip6gretap`. L3 tunnel cases (gre,
ip6gre, ipip, sit, ip6tnl, ipip6) should not carry a `mac-address`
leaf because the Linux kernel does not assign them a MAC address and
setting one has no effect.

Today `list tunnel` uses `uses interface-l2;` which inherits
`mac-address` from `interface-l2`. After this change, `list tunnel`
would use `uses interface-common;` (no MAC) and the two bridgeable
cases would inline their own `mac-address` leaf. This narrows the
YANG schema so that only structurally valid leaves are expressible
per kind, matching the choice/case spirit that already restricts
`key` to GRE-family and `hoplimit`/`tclass`/`encaplimit` to
v6-underlay.

**Origin:** deferred from spec-iface-wireguard Phase 2 (D2 resolution).
The grouping split (interface-physical -> interface-common + interface-l2)
was done to support wireguard (L3, no MAC), but tunnel was left on
interface-l2 because per-case MAC handling would have expanded the D2
scope. Recorded in `plan/deferrals.md` 2026-04-12.

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` -- tunnel section, interface-l2 description

### Source Files Read
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` -- `grouping interface-l2`, `list tunnel`, the 8 `case` blocks under `choice kind`, `grouping tunnel-v4-endpoints`, `grouping tunnel-v6-endpoints`
- [ ] `internal/component/iface/config.go` -- `parseIfaceEntry` reads `mac-address`, `applyConfig` Phase 2 calls `SetMACAddress`
- [ ] `internal/component/iface/tunnel.go` -- `TunnelSpec`, `IsBridgeable()`
- [ ] `internal/plugins/ifacenetlink/tunnel_linux.go` -- `CreateTunnel`, per-kind builders

## Current Behavior (MANDATORY)

**Source files read:** see Required Reading above.

**Behavior to preserve:**
- gretap and ip6gretap interfaces accept a MAC address and the backend calls `SetMACAddress` on them during Phase 2
- `ze:required "mac-address"` is NOT set on `list tunnel` (it is set on ethernet/veth/bridge only), so MAC is optional on tunnels
- L3 tunnel creation and operation is unaffected

**Behavior to change:**
- `list tunnel` switches from `uses interface-l2;` to `uses interface-common;` (drops mac-address from the list level)
- `case gretap` and `case ip6gretap` containers each gain a `leaf mac-address` with the same type and validator as the one in `interface-l2`
- `parseTunnelEntry` or `parseIfaceEntry` is updated to read `mac-address` from either the list level (for non-tunnel kinds) or the case level (for gretap/ip6gretap)
- The YANG walker's alphabetical sort means leaf order in config dumps may shift for tunnel entries; since the walker alphabetizes regardless of declaration order (confirmed in spec-iface-wireguard /ze-review), this is cosmetically invisible

## Data Flow (MANDATORY)

### Entry Point
- Config file -> parser -> `parseTunnelEntry` reads mac-address from the case container (gretap/ip6gretap) instead of the list level

### Transformation Path
1. YANG schema change: mac-address moves from grouping to per-case container
2. Go parser: `parseTunnelEntry` checks for mac-address inside the matched case container
3. `applyConfig` Phase 2: `SetMACAddress` called only for bridgeable kinds (already gated by `IsBridgeable()` check -- verify)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema -> Go parser | Parser reads mac-address from case-level container | [ ] |

### Integration Points
- `parseIfaceEntry` currently reads mac-address at the list level for ALL kinds -- needs to be conditional for tunnels
- `applyConfig` Phase 2 `SetMACAddress` -- verify it is already gated on bridgeable kinds

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with gretap + mac-address | -> | parser reads mac-address from case level | `TestParseTunnelGretapMAC` |
| Config with gre + mac-address (invalid) | -> | parser rejects or YANG rejects | `TestParseTunnelGreNoMAC` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `tunnel foo { encapsulation { gretap { ... } } mac-address aa:...; }` | Rejected -- mac-address is no longer at the list level; must be inside the gretap block |
| AC-2 | Config with `tunnel foo { encapsulation { gretap { mac-address aa:...; ... } } }` | Accepted -- mac-address inside the case container |
| AC-3 | Config with `tunnel foo { encapsulation { gre { ... } } }` (no mac-address) | Accepted -- L3 kind, no mac-address available in schema |
| AC-4 | Config with `tunnel foo { encapsulation { gre { mac-address aa:...; ... } } }` | Rejected by YANG -- mac-address not in the gre case |
| AC-5 | Existing tunnel `.ci` tests | All still pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseTunnelGretapMAC` | `config_test.go` | AC-2 | |
| `TestParseTunnelGreNoMAC` | `config_test.go` | AC-3/AC-4 | |

### Functional Tests
| Test File | End-User Scenario | Status |
|-----------|-------------------|--------|
| Existing `test/reload/test-tx-iface-tunnel-*.ci` | AC-5 -- no regression | |

## Files to Modify

- `internal/component/iface/schema/ze-iface-conf.yang` -- move mac-address into gretap/ip6gretap cases; switch list tunnel from `uses interface-l2` to `uses interface-common`
- `internal/component/iface/config.go` -- `parseTunnelEntry` reads mac-address from the case container for bridgeable kinds
- `docs/features/interfaces.md` -- update `interface-l2` description to remove the "tunnel" mention

## Files to Create

- None expected

## Implementation Steps

### Implementation Phases

1. **Phase 1: YANG restructure.** Move mac-address into gretap and ip6gretap case containers. Switch `list tunnel` from `uses interface-l2;` to `uses interface-common;`. Verify YANG schema tests pass.
2. **Phase 2: Parser update.** Adjust `parseTunnelEntry` to read mac-address from the matched case container for bridgeable kinds. Update existing tunnel parser unit tests. Verify Phase 2 `SetMACAddress` in `applyConfig` is already gated on bridgeable kinds (if not, add the gate).
3. **Phase 3: Docs + verify.** Update `docs/features/interfaces.md` interface-l2 description. Full `make ze-verify`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| No regression | All existing tunnel tests and `.ci` files pass |
| Schema correctness | `ze config validate` rejects mac-address on L3 tunnel cases |
| Parser correctness | mac-address on gretap/ip6gretap round-trips through parse -> apply |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `list tunnel` uses `interface-common` not `interface-l2` | `grep 'uses interface' ze-iface-conf.yang` |
| mac-address inside gretap/ip6gretap cases only | `grep -A5 'gretap' ze-iface-conf.yang` |
| Existing `.ci` tests pass | `ze-test bgp reload -a` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| N/A | Pure schema refactor, no new inputs |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Existing tunnel test fails | Parser or config structure regression |
| 3 fix attempts fail | STOP. Report. Ask user |

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

N/A -- no protocol change.

## Implementation Summary

### What Was Implemented
(Fill at completion.)

### Bugs Found/Fixed
(Fill at completion.)

### Documentation Updates
(Fill at completion.)

### Deviations from Plan
(Fill at completion.)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| (filled at completion) | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| (filled at completion) | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (filled at completion) | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| (filled at completion) | | |

### Audit Summary
- **Total items:** TBD
- **Done:** TBD
- **Partial:** TBD (all require user approval)
- **Skipped:** TBD (all require user approval)
- **Changed:** TBD (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| (filled at completion) | | |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| (filled at completion) | | |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| (filled at completion) | | |

## Checklist

### Goal Gates
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-iface-tunnel-mac-per-case.md`
