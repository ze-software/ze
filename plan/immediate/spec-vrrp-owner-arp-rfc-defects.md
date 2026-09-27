# Spec: vrrp-owner-arp-rfc-defects

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Commit `850eb41b66` stopped the VRRP address owner's parent from answering ARP
and ND for the virtual address with its physical MAC. Two halves remain,
recorded in `plan/journal/guard-added-to-one-half-of-a-pair.md` on 2026-09-27
and confirmed open in HEAD the same day: the parent's own ARP requests still
carry the physical MAC for the owned address, and the vpp firewall backend
refuses the new ARP and ND matches with a generic error.

## Defects

| ID | Source | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|--------|----------------|----------|--------------------|------------------------|
| D1 | RFC 9568 Section 8.1.2 | "When a VRRP Router restarts or boots, it SHOULD NOT send any ARP messages using its physical MAC address for an IPv4 address for which it is the IPv4 address owner (as defined in Section 1.7), and it should only send ARP messages that include Virtual Router MAC addresses." | `internal/plugins/vrrp/ownerfilter.go` (`ownerARPReplyTerm`, the only ARP term) | Only ARP replies are dropped; an ARP request the parent sends with the owned address as sender still carries the physical MAC, and a host that learns from requests caches it | journal row, not fixed |
| D2 | journal row (no RFC sentence: backend parity) | (none) | `internal/plugins/firewall/vpp/verify.go::verifyMatch` | `MatchARPOperation`, `MatchARPSenderAddress` and `MatchNDTargetAddress` fall to "match type ... not recognized by backend vpp", so a VRRP owner on the vpp dataplane is refused without a reason naming the match | journal row, not fixed |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1: a QEMU test capturing the parent's ARP request for the owned address. D2: a verifier unit test per match |
| Both polarities | D1: requests for other addresses keep the physical MAC. D2: each match either lowers or refuses with its own reason |
| Discrimination | the QEMU red with the new term flipped to accept, as `850eb41b66` recorded for the reply term |
| Real entry point | the `vrrp-keepalived` QEMU scenarios and a new owner-request scenario |
| Interop | `vrrp-v2-owner-keepalived` and `vrrp-mastership-keepalived` stay green |

## Owner decisions

| Row | Question |
|-----|----------|
| D1 | Drop the parent's ARP requests whose sender is the owned address, or rewrite their sender MAC to the virtual MAC? The journal row names both |
| D2 | Lower each of the three matches on vpp, or refuse each with its own reason? |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9568.md`, `rfc/short/rfc5798.md`, `rfc/short/rfc3768.md`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/vrrp/ownerfilter.go` - owner ARP and ND terms
- [ ] `internal/plugins/firewall/vpp/verify.go` - `verifyMatch`

**Behavior to change:** the two rows.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- a VRRP instance whose virtual address is a real address of the parent (owner)

### Transformation Path
1. the VRRP plugin builds owner filter terms
2. the firewall component lowers them to nft (arp family) or verifies them for vpp

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VRRP plugin ↔ firewall backends | firewall model terms | No |

### Integration Points
- the firewall backend registry

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| VRRP owner config | → | owner ARP request term | [to fill in design: QEMU] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | owner Master sends an ARP request | no frame carries the physical MAC as sender for the owned address |
| AC-2 | owner filter on the vpp backend | lowered, or refused with a reason naming the match |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| verifier cases per match | `internal/plugins/firewall/vpp/` | D2 | [to fill in design] |

## Files to Modify

- the two producers, their tests, the journal rows' Fix cells at closure

## Implementation Steps

1. Owner picks D1's shape; 2. failing tests; 3. fixes; 4. QEMU and interop evidence.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
