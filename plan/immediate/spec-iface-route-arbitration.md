# Spec: iface-route-arbitration -- interface-layer routes join administrative-distance arbitration

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Split out of `spec-connected-static-reach-the-locrib` (closed 2026-10-08) by the owner
on 2026-10-08.

-> Decision (owner, 2026-10-08): interface-layer route arbitration (DHCP, RA and PPP routes) gets this spec. It does not block `spec-connected-static-reach-the-locrib`.

Routes the interface layer learns (DHCPv4, IPv6 RA, PPPoE, PPP NCPs) are stamped
`rtproto.Iface` (253) and programmed into the kernel directly, so they never reach
the Loc-RIB and are never arbitrated by administrative distance against static,
connected or dynamic-protocol routes. The parent spec moves connected and
main-table static routes into the Loc-RIB; its Known Limitations section records
this interface-layer remainder as a THIRD inert consumer of the same idea, and
`rib { distance { } }` declares no iface leaf.

The history is the 2026-08-10 row in `plan/journal/guard-added-to-one-half-of-a-pair.md`.
It names a destination spec, `spec-admin-distance-reaches-the-kernel`, that was never
written; this spec is the destination for the interface-layer part of that row. The
narrow fix it records (`route-priority` defaulting to 254 on a unit and a
pppoe-client, landed 2026-08-11) separates a learned default from an operator static
by kernel metric; it does not arbitrate by distance.

The design phase decides, with the owner where it changes behavior, whether
interface-layer routes become Loc-RIB producers, which distance they carry, and how
`rib { distance { } }` exposes it. These facts are the parent's and the journal's
claims; the design phase re-reads each producer.

Acceptance criteria: none moved from the parent, whose ACs cover the connected and
main-table static paths only. This spec writes its own at design time.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/rib/unified-locrib.md` - the Loc-RIB the routes would join

## Current Behavior (MANDATORY)

**Source files read:** (the design phase reads each before writing this section)
- [ ] `internal/plugins/iface/dhcp/dhcp_v4_linux.go` - [DHCPv4 route install; read at design time]
- [ ] `internal/component/l2tp/ppp/ipv6_service.go` - [PPP IPv6 route install; read at design time]
- [ ] `internal/component/sysrib/yang/ze-rib-conf.yang` - [`rib { distance { } }` leaves; read at design time]

## Data Flow (MANDATORY)

### Entry Point
- [Where data enters: written at design time]

### Transformation Path
1. [written at design time]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [written at design time] | | No |

### Integration Points
- [written at design time]

## Risks & Assumptions

[written at design time]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [written at design time] | → | | |

## Acceptance Criteria

[written at design time]

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [written at design time] | | | |

## Files to Modify
- [written at design time]

### Integration Checklist
- [written at design time]

### Documentation Update Checklist (BLOCKING)
- [written at design time]

## Implementation Steps
1. [written at design time]

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
