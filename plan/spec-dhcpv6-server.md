# Spec: dhcpv6-server

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/dhcpserver/` - current IPv4-only server
4. `ai/rules/planning.md` - this is a skeleton; full RESEARCH/DESIGN not yet done

## Task

**Large feature area — skeleton only. Full design not started.**

Ze's DHCP server is IPv4-only. Ze cannot serve DHCPv6: no address assignment
(IA_NA), no prefix delegation (IA_PD), no DUID identity, no v6 options, no v6-only
reservation model. Networks that need stateful IPv6 addressing or PD from Ze have
no path.

Add a DHCPv6 server. This is a substantial, multi-phase effort (RFC 8415 and
related) and must go through the full `/ze-spec` RESEARCH/DESIGN workflow before
implementation. Likely sub-features once designed:

- Stateful address assignment (IA_NA) and prefix delegation (IA_PD).
- DUID-based client identity and reservations that can carry multiple addresses
  and/or multiple delegated prefixes per host, with validation that a reservation
  has at least one address or prefix.
- DHCPv6 options (DNS, time-zone / RFC 4833, vendor options), Rapid Commit, and
  Information-Request handling.

This skeleton exists so the gap is tracked; it is NOT ready to implement.

Cross-reference (2026-07-10): the v6 options sub-feature should adopt the same
per-pool option config shape (code, encoding ascii|hex, value, plus a denylist of
auto-emitted codes) being designed for DHCPv4 in `plan/spec-dhcp-pool-options.md`,
so the two servers present one consistent operator surface.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/dhcpserver/` (current IPv4 code) - reuse packet/socket/lease scaffolding where possible.
  → Constraint: DHCPv6 uses a different transport (UDP 546/547, multicast) and message model; do not force it into the v4 handler.
- [ ] `ai/rules/architecture.md` - where a v6 server sits relative to the v4 plugin.
  → Constraint: decide during design whether v6 is a sibling plugin or a shared engine with v4.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 8415 (DHCPv6) - core protocol, IA_NA/IA_PD, DUID.
  → Constraint: create the `rfc/short/` summary during the DESIGN phase before coding.

**Key insights:**
- The v4 server's lease/pool abstractions may generalise, but the wire format and identity model are entirely different.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dhcpserver/config.go` - IPv4-only: addresses handled as 32-bit (`As4()`); reservations are one MAC → one IPv4; no v6 fields.
- [ ] `internal/plugins/dhcpserver/handler.go` - DHCPv4 op codes / magic cookie / v4 option codes only; no v6 message types.
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - v4 service schema only; no `dhcpv6-server` container.

**Behavior to preserve:**
- The existing IPv4 DHCP server is unchanged; DHCPv6 is additive (separate service container).

**Behavior to change:**
- Introduce a DHCPv6 service (design to decide plugin boundary).

## Data Flow (MANDATORY)

### Entry Point
- Config: a new `service dhcpv6-server` container in a DHCPv6 YANG schema (interfaces to listen on, address ranges, delegated-prefix pools, reservations).
- Wire: DHCPv6 Solicit/Request/Renew/Rebind/Release/Information-Request on UDP 547 (server) from clients on 546, over the interface's link-local scope.

### Transformation Path
1. Config parsed into a DHCPv6 server config (pools, PD pools, reservations by DUID).
2. Server binds v6 sockets on configured interfaces and joins the relay/server multicast group.
3. Incoming DHCPv6 messages parsed (DUID, IA_NA, IA_PD options).
4. Address/prefix selected from pools or reservation; lease recorded.
5. Reply built with assigned IA_NA/IA_PD and requested options.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ dhcpv6 server | new v6 YANG → v6 config struct | [ ] |
| Wire ↔ engine | v6 message parse/build (UDP 546/547) | [ ] |
| Lease store ↔ engine | v6 lease/PD state persisted | [ ] |

### Integration Points
- `internal/plugins/dhcpserver/` scaffolding (lease/pool) - candidate for reuse or generalisation (design decision).
- `internal/component/iface/` - interface enumeration / link-local addresses for binding.

### Architectural Verification
- [ ] No bypassed layers (config via the standard path)
- [ ] No unintended coupling (v6 does not distort the v4 handler)
- [ ] No duplicated functionality (reuse lease/pool abstractions where they generalise)
- [ ] Registration over hardcoding — DHCPv6 registers as its own plugin/service; no v6 special-case in core.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | v4 lease/pool abstractions partly generalise to v6 | `internal/plugins/dhcpserver/lease.go`, `pool.go` | more greenfield code than hoped | design spike during RESEARCH | unvalidated |
| A-2 | A separate v6 plugin is the right boundary | small-core/registration model | shared engine may be better | design decision with user | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope explosion (IA_NA + IA_PD + options + relay) | design keeps growing | phase into sub-specs (address-only first, PD later) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set service dhcpv6-server ...` + client Solicit | → | v6 server assigns IA_NA/IA_PD | `test/plugin/dhcpv6-server.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | client Solicit/Request on a configured link | server offers and assigns an IA_NA address |
| AC-2 | client requests IA_PD | server delegates a prefix from the PD pool |
| AC-3 | reservation with multiple addresses/prefixes for a DUID | all are offered to that client |
| AC-4 | reservation with neither address nor prefix | config verify rejects |
| AC-5 | existing IPv4 server | unaffected |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures a v6 pool and a client solicits | config → v6 engine → assign IA_NA → reply | `test/plugin/dhcpv6-server.ci` |
| 2 | configures a PD pool | client IA_PD → delegated prefix | `test/plugin/dhcpv6-pd.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDHCPv6SolicitAdvertise` | `internal/plugins/dhcpv6server/handler_test.go` | Solicit → Advertise with IA_NA | |
| `TestDHCPv6PrefixDelegation` | `internal/plugins/dhcpv6server/pd_test.go` | IA_PD assignment | |
| `TestDHCPv6ReservationMulti` | `internal/plugins/dhcpv6server/config_test.go` | multiple addresses/prefixes per DUID + validation | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PD delegated length | design (e.g. 48-64) | design | design | design |
| valid/preferred lifetime | design | design | design | design |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dhcpv6-server` | `test/plugin/dhcpv6-server.ci` | client obtains an IPv6 address and PD | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-dhcpv6-client` | `test/interop/scenarios/` | ISC/Kea client or dhclient -6 | real client obtains IA_NA/IA_PD from Ze | |

### Future (if deferring any tests)
- Phasing: address assignment first, PD and options in follow-up sub-specs (design to define).

## Files to Modify
- `internal/plugins/dhcpserver/` - possible generalisation of lease/pool for reuse (design decision)

## Files to Create
- `internal/plugins/dhcpv6server/` - new DHCPv6 server plugin (name/boundary decided in design)
- `internal/plugins/dhcpv6server/yang/` - DHCPv6 config schema
- `test/plugin/dhcpv6-server.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton — run `/ze-spec` RESEARCH/DESIGN first) |

### Implementation Phases
1. **RESEARCH/DESIGN (not started)** — run the full `/ze-spec` workflow: RFC 8415 summary, plugin-boundary decision, phasing into sub-specs. This skeleton is not implementable as-is.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests above are provisional placeholders to be refined during DESIGN.

## Implementation Summary
### What Was Implemented
- Nothing yet (skeleton).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Full `/ze-spec` DESIGN completed and approved before implementation
- [ ] `make ze-standard-test` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)

### Quality Gates (SHOULD pass)
- [ ] RFC 8415 summary created

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
