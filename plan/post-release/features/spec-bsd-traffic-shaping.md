# Spec: bsd-traffic-shaping

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Adds a BSD platform or backend; release correctness of existing Linux support is not deferred. |
| Scope | plugin |
| Depends | `plan/post-release/features/spec-bsd-family-interface-backend.md` (P1), `plan/post-release/features/spec-bsd-pf-firewall-backend.md` (pf queues share the pf ABI), `plan/post-release/features/spec-bsd-qemu-guest-kind.md` |
| Phase | 0/0 (Do not start until the owner approves) |
| Handoff | - |
| Updated | 2026-10-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's traffic component lowers its QoS model into Linux `tc` (default backend `tc`)
or VPP. Off Linux `internal/component/traffic/default_other.go` sets no default, so
traffic config rejects. This spec adds a BSD shaping backend:

| OS | Candidates |
|----|------------|
| FreeBSD | dummynet (through ipfw or pf `dnpipe`/`dnqueue`), ALTQ |
| OpenBSD | pf queues (HFSC), the only shaper in base |
| macOS | dummynet through pf; recommendation: refuse, dev-only platform |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/traffic/` backend pages (`fw-7-traffic-vpp.md`, `tc-original-qdisc-restore.md`)
  → Constraint: Ze restores the original shaping state on removal, as `tc-original-qdisc-restore.md` requires on Linux

**Key insights:**
- [to be filled at design]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/traffic/default_other.go` - empty default backend off Linux

**Behavior to preserve:**
- Linux `tc` and VPP backends unchanged.

**Behavior to change:**
- FreeBSD and OpenBSD gain a shaping backend.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: traffic config tree]
- [Format at entry]

### Transformation Path
1. [to be designed]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to kernel | [pf queue ioctl / dummynet sockopt] | No |

### Integration Points
- [to be designed]

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze's QoS model maps onto HFSC queues and dummynet pipes | [to be checked] | [features refused per OS] | [mapping table at design] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two backends per FreeBSD (dummynet, ALTQ) double the work | [design] | [pick one at design] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Traffic config on OpenBSD | → | [pf queue backend apply] | [test name at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Rate limit on an interface on FreeBSD/OpenBSD | Measured throughput within the configured rate |
| AC-2 | Config removed | Original shaping state restored |
| AC-3 | Model feature the OS shaper cannot express | Refused by name |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to be designed] | [BSD traffic backend package] | [queue lowering] | |

## Files to Modify
- `internal/component/traffic/default_other.go` - narrow build tags
- `docs/guide/` traffic pages - platform section

## Files to Create
- [BSD traffic backend package]

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
