# Spec: iface-4 — DHCP, Migration, Mirroring, SLAAC

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-iface-2, spec-iface-3 |
| Phase | - |
| Updated | 2026-03-30 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-iface-0-umbrella.md` — migration protocol, DHCP topics, YANG
3. `internal/component/iface/` — plugin code from Phases 1-2
4. `internal/component/bgp/reactor/reactor_iface.go` — BGP reactions from Phase 3

## Task

Add advanced interface features: DHCP client (v4/v6 with Prefix Delegation), make-before-break IP migration protocol, traffic mirroring via tc mirred, and IPv6 SLAAC control via sysctl. These features depend on both interface management (Phase 2) and BGP reactions (Phase 3).

## Required Reading

### Architecture Docs
- [ ] `plan/spec-iface-0-umbrella.md` — migration protocol, DHCP topics, mirroring YANG
  → Decision: 5-phase migration, Phase 4 blocked until BGP confirms on new address
  → Decision: DHCP via `insomniacslk/dhcp`, mirroring via `vishvananda/netlink` tc
  → Constraint: DHCP addresses also trigger standard `addr/added`/`addr/removed` events
- [ ] `.claude/rules/goroutine-lifecycle.md` — long-lived workers
  → Constraint: DHCP client is a long-lived goroutine per interface, not per-lease

### RFC Summaries (MUST for protocol work)
- [ ] DHCPv4 and DHCPv6 RFCs — summaries to be created via `/rfc-summarisation` before implementation
  → Constraint: client must respect lease time and renew at T1, rebind at T2
  → Constraint: IA_PD option for prefix delegation

**Key insights:**
- Migration ordering is critical: BGP must confirm ready before old IP removed
- DHCP-acquired addresses trigger both DHCP-specific and standard addr events
- tc mirred requires ingress qdisc + matchall filter (all via netlink, no shell)
- SLAAC is kernel-native — Ze just controls sysctl knobs

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/iface_linux.go` — interface management from Phase 2
- [ ] `internal/component/bgp/reactor/reactor_iface.go` — BGP reactions from Phase 3
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` — YANG schema from Phase 2

**Behavior to preserve:**
- Phase 1 monitoring continues
- Phase 2 management continues
- Phase 3 BGP reactions continue

**Behavior to change:**
- No DHCP client capability — this spec adds it
- No migration orchestration — this spec adds it
- No traffic mirroring — this spec adds it

## Data Flow (MANDATORY)

### Entry Points
- DHCP: config enables DHCP on interface → DHCP client starts → lease events
- Migration: config reload or `ze interface migrate` CLI → orchestrated sequence
- Mirror: config specifies mirror → tc qdisc + filter created
- SLAAC: config enables autoconf → sysctl written

### Transformation Path (DHCP)
1. **Config** — DHCP enabled on interface
2. **Client start** — long-lived goroutine runs DISCOVER/SOLICIT
3. **Lease acquired** — address added via netlink
4. **Monitor detects** — publishes `interface/addr/added` (standard) + `interface/dhcp/lease-acquired` (specific)
5. **Renewal** — client renews at T1, publishes `interface/dhcp/lease-renewed`
6. **Expiry** — address removed, publishes `interface/dhcp/lease-expired` + `interface/addr/removed`

### Transformation Path (Migration)
Migration operates at the unit level -- moving an IP from one unit to another:
1. **Trigger** -- config reload or CLI command
2. **Phase 1-2** -- create new interface/unit, add IP (Phase 2 management)
3. **Phase 3** -- BGP confirms on new address (Phase 3 reactions publish `bgp/listener/ready`)
4. **Phase 4** -- orchestrator removes old IP from old unit (Phase 2 management)
5. **Phase 5** -- orchestrator removes old interface/unit (Phase 2 management)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ DHCP server | UDP (v4) / UDP over link-local (v6) | [ ] |
| Plugin ↔ OS | Netlink for addr add/remove, tc for mirroring | [ ] |
| Plugin ↔ Bus | Publish DHCP events + standard addr events | [ ] |
| Orchestrator ↔ Bus | Listen for `bgp/listener/ready` confirmation | [ ] |

### Integration Points
- Phase 2 management — migration uses create/delete/addr operations
- Phase 3 BGP reactions — migration waits for BGP readiness signal
- Phase 1 monitor — detects all changes (DHCP, migration, mirroring)

### Architectural Verification
- [ ] No bypassed layers (DHCP → netlink → monitor → Bus)
- [ ] No unintended coupling (migration orchestrator uses Bus events, not direct calls)
- [ ] No duplicated functionality (extends Phase 1-3 work)
- [ ] Zero-copy preserved where applicable

## DHCP Configuration (from umbrella)

DHCP is configured per-unit (a unit acquires addresses, not the physical interface):

| YANG Path | Description |
|-----------|-------------|
| `<type>/<name>/unit/<id>/dhcp/enabled` | Enable DHCPv4 (default: false) |
| `<type>/<name>/unit/<id>/dhcp/client-id` | Optional client identifier |
| `<type>/<name>/unit/<id>/dhcp/hostname` | Hostname in DHCP requests |
| `<type>/<name>/unit/<id>/dhcpv6/enabled` | Enable DHCPv6 (default: false) |
| `<type>/<name>/unit/<id>/dhcpv6/pd/length` | Requested prefix length |
| `<type>/<name>/unit/<id>/dhcpv6/duid` | Optional DUID override |

Dependency: `github.com/insomniacslk/dhcp` (BSD-3-Clause, 815+ stars)

## Traffic Mirroring (from umbrella)

Mirroring is per-unit. For VLAN units, tc targets the VLAN subinterface. For unit 0, tc targets the parent.

| YANG Path | Description |
|-----------|-------------|
| `<type>/<name>/unit/<id>/mirror/ingress` | Mirror ingress to this unit's OS interface |
| `<type>/<name>/unit/<id>/mirror/egress` | Mirror egress to this unit's OS interface |

Implementation: ingress/egress qdisc + matchall filter with `MirredAction` via netlink. Optional -- omitting `mirror` means no mirroring.

## Migration Failure Handling (from umbrella)

| Failure | Recovery |
|---------|----------|
| New interface creation fails | Abort, old unchanged |
| IP add fails | Abort, clean up new interface |
| BGP fails to bind new address | Timeout → abort, remove IP from new, old unchanged |
| Old IP removal fails | Log warning, continue |
| Old interface removal fails | Log warning (stale, no IP) |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with DHCP enabled | → | DHCP client obtains lease | `test/plugin/iface-dhcp.ci` |
| Config reload with migration | → | Make-before-break sequence | `TestMakeBeforeBreakMigration` |
| Config with mirror | → | tc mirred created | `test/plugin/iface-mirror.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-6 | Make-before-break migration via config reload | New interface created, IP added, BGP binds, old IP removed, old interface deleted — no unreachable period |
| AC-7 | DHCP enabled on interface | DHCPv4 client sends DISCOVER, obtains lease, adds address, publishes `interface/dhcp/lease-acquired` |
| AC-8 | DHCPv6 with PD enabled | DHCPv6 client sends SOLICIT, obtains prefix delegation, publishes event |
| AC-9 | DHCP lease expires | Address removed via netlink, `interface/dhcp/lease-expired` published, `interface/addr/removed` fires |
| AC-10 | IPv6 autoconf enabled on interface | sysctl `autoconf=1` set, kernel SLAAC addresses detected by monitor, published as `interface/addr/added` |
| AC-11 | Traffic mirror configured | tc ingress qdisc + matchall filter with mirred action created |
| AC-12 | Traffic mirror removed | tc filter and qdisc removed, no traffic duplication |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDHCPLeaseToEvent` | `internal/component/iface/dhcp_linux_test.go` | DHCP lease publishes correct Bus events | |
| `TestDHCPv6PDToEvent` | `internal/component/iface/dhcp_linux_test.go` | DHCPv6 PD publishes correct Bus events | |
| `TestDHCPLeaseExpiry` | `internal/component/iface/dhcp_linux_test.go` | Lease expiry removes address and publishes events | |
| `TestMirrorSetup` | `internal/component/iface/mirror_linux_test.go` | tc qdisc + mirred filter created | |
| `TestMirrorTeardown` | `internal/component/iface/mirror_linux_test.go` | tc filter and qdisc removed | |
| `TestMigrationOrdering` | `internal/component/bgp/reactor/reactor_iface_test.go` | Old IP not removed until new confirmed | |
| `TestMigrationAbortOnFailure` | `internal/component/iface/iface_linux_test.go` | Failed migration cleans up new interface | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PD prefix length | 48-64 | 64 | 47 | 65 |
| DHCP lease time | 1-4294967295 | 4294967295 | 0 | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-iface-dhcp` | `test/plugin/iface-dhcp.ci` | DHCP client obtains lease, address appears | |
| `test-iface-mirror` | `test/plugin/iface-mirror.ci` | Traffic mirroring configured, packets duplicated | |
| `test-iface-migrate` | `test/plugin/iface-migrate.ci` | Full make-before-break migration | |

### Future (if deferring any tests)
- NDP/RA sender functionality — deferred to future spec (Ze is BGP daemon, not RA daemon)
- Chaos test: DHCP server failure during lease — defer to chaos framework

## Files to Modify

- `internal/component/iface/register.go` — add DHCP dependency on `insomniacslk/dhcp`
- `internal/component/iface/schema/ze-iface-conf.yang` — DHCP, mirror, SLAAC YANG sections
- `go.mod` — add `github.com/insomniacslk/dhcp`

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (extend) | [x] | `internal/component/iface/schema/ze-iface-conf.yang` — DHCP + mirror |
| CLI commands/flags | [x] | `cmd/ze/interface/migrate.go` |
| Functional test | [x] | `test/plugin/iface-dhcp.ci`, `test/plugin/iface-mirror.ci`, `test/plugin/iface-migrate.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — DHCP, migration, mirroring |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` — DHCP + mirror stanzas |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — `ze interface migrate` |
| 4 | API/RPC added/changed? | No | — |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — DHCP + mirroring capabilities |
| 6 | Has a user guide page? | Yes | `docs/guide/interfaces.md` — DHCP + migration sections |
| 7 | Wire format changed? | No | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented? | Yes | DHCPv4 + DHCPv6 RFCs — summaries created before implementation |
| 10 | Test infrastructure changed? | No | — |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — DHCP + migration |
| 12 | Internal architecture changed? | No | — |

## Files to Create

- `internal/component/iface/dhcp_linux.go` — DHCPv4/v6 client using `insomniacslk/dhcp`
- `internal/component/iface/mirror_linux.go` — tc mirred setup via netlink
- `cmd/ze/interface/migrate.go` — `ze interface migrate` handler
- `internal/component/iface/dhcp_linux_test.go` — DHCP unit tests
- `internal/component/iface/mirror_linux_test.go` — Mirror unit tests
- `test/plugin/iface-dhcp.ci` — Functional test
- `test/plugin/iface-mirror.ci` — Functional test
- `test/plugin/iface-migrate.ci` — Functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
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

1. **Phase: DHCP client** — DHCPv4/v6 with PD, lease lifecycle, Bus events
   - Tests: `TestDHCPLeaseToEvent`, `TestDHCPv6PDToEvent`, `TestDHCPLeaseExpiry`
   - Files: `dhcp_linux.go`, YANG schema
   - Verify: tests fail → implement → tests pass
2. **Phase: Traffic mirroring** — tc qdisc + mirred filter via netlink
   - Tests: `TestMirrorSetup`, `TestMirrorTeardown`
   - Files: `mirror_linux.go`
   - Verify: tests fail → implement → tests pass
3. **Phase: Migration orchestration** — 5-phase make-before-break sequence
   - Tests: `TestMigrationOrdering`, `TestMigrationAbortOnFailure`
   - Files: `cmd/ze/interface/migrate.go`, integration with Phase 2+3
   - Verify: tests fail → implement → tests pass
4. **Functional tests** → `test/plugin/iface-dhcp.ci`, `test/plugin/iface-mirror.ci`, `test/plugin/iface-migrate.ci`
5. **Full verification** → `make ze-verify`
6. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-6 through AC-12 all have implementation with file:line |
| Correctness | Migration ordering: BGP confirms before old IP removed. DHCP respects lease timers. |
| Naming | DHCP Bus topics match umbrella table exactly |
| Data flow | DHCP → netlink → monitor → Bus (standard + DHCP-specific events) |
| Rule: goroutine-lifecycle | DHCP client is long-lived per-interface goroutine |
| Migration abort | Failure at any phase cleans up correctly |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/component/iface/dhcp_linux.go` exists | `ls -la` |
| `internal/component/iface/mirror_linux.go` exists | `ls -la` |
| `cmd/ze/interface/migrate.go` exists | `ls -la` |
| `test/plugin/iface-dhcp.ci` exists | `ls -la` |
| `test/plugin/iface-mirror.ci` exists | `ls -la` |
| `test/plugin/iface-migrate.ci` exists | `ls -la` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | DHCP server responses may be malicious — validate all fields |
| Resource exhaustion | Lease failure must not retry in tight loop — exponential backoff |
| Privilege | DHCP requires raw socket / `CAP_NET_RAW`. Mirror requires `CAP_NET_ADMIN`. |
| Migration safety | Abort path must never leave both old and new IPs removed simultaneously |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
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

DHCPv4 and DHCPv6 RFC comment references to be determined after `/rfc-summarisation` before implementation.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from plan and why]

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
- [ ] AC-6 through AC-12 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
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

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-iface-4-advanced.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
