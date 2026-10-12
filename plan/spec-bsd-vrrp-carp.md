# Spec: bsd-vrrp-carp

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | `plan/spec-bsd-family-interface-backend.md` (P1, P2, P3 raw sockets), `plan/spec-bsd-qemu-guest-kind.md` |
| Phase | 0/0 (Do not start until the owner approves) |
| Handoff | - |
| Updated | 2026-10-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's VRRP (`internal/plugins/vrrp/`) owns the virtual MAC through a Linux macvlan
and procfs ARP sysctls (`dataplane_linux.go`); off Linux `dataplane_other.go` makes
the dataplane no-ops because the whole path is unreachable (no macvlan, no raw
sockets). The BSD systems ship carp(4) in the kernel, and carp uses IP protocol 112,
the same protocol number as VRRP, so a host running both sees each other's
advertisements. This spec decides how VRRP works on FreeBSD and OpenBSD.

**Owner decision, 2026-10-12:** "CARP vs VRRP - two different protocols, you
should be able to use one or the other on the interface (if/when we add CARP
support)". So:

| Protocol | Shape |
|----------|-------|
| VRRP | Ze's VRRP over raw proto 112 on every OS that can carry it; on BSD it needs a virtual-MAC mechanism without macvlan (interface `lladdr` alias or promiscuous answer) |
| CARP | A separate protocol, configured from Ze through carp(4) where the kernel has it; not VRRP on the wire, never presented as VRRP |
| Per interface | An interface runs VRRP or CARP, never both: the config refuses a second one on the same interface with an error naming the one already there |

CARP support itself is not yet in scope ("if/when we add CARP support"): until it
is, a CARP request is refused by name, and VRRP on BSD follows the VRRP row.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` - Linux virtual-MAC dataplane
  → Constraint: the virtual MAC is owned through macvlan on Linux; BSD has no macvlan

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5798.md` - VRRPv3
  → Constraint: protocol 112 and the virtual MAC `00-00-5E-00-01-{VRID}` are fixed by the RFC

**Key insights:**
- [to be filled at design]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/vrrp/dataplane_other.go` - no-op dataplane off Linux

**Behavior to preserve:**
- Linux VRRP unchanged.

**Behavior to change:**
- On BSD, VRRP either works or is refused by name; never silently inert.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: VRRP config tree, advertisements on proto 112]
- [Format at entry]

### Transformation Path
1. [to be designed]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to wire | [raw socket proto 112] | No |

### Integration Points
- [to be designed]

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A BSD interface can answer for the VRRP virtual MAC without macvlan | [to be checked per OS] | [VRRP refused on that OS] | [guest test] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | carp on the same segment misreads Ze's VRRP adverts | [interop with carp present] | [doctor check warns when carp interfaces exist] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| VRRP config on FreeBSD | → | [BSD VRRP dataplane or refusal] | [test name at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | VRRP config on BSD before a design is approved | Refused by name at verify |
| AC-2 | After design: VRRP against FRR/keepalived from a BSD guest | Master election and virtual MAC ownership as on Linux |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to be designed] | `internal/plugins/vrrp/` | [refusal off Linux] | |

## Files to Modify
- `internal/plugins/vrrp/dataplane_other.go` - refusal or BSD dataplane
- `docs/guide/vrrp.md` - platform section

## Files to Create
- [BSD dataplane files, if chosen]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| [to be answered at design] | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| [to be answered at design] | | | |

## Implementation Steps

1. **Phase: Wiring** -- [to be designed]

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
