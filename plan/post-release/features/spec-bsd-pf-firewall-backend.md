# Spec: bsd-pf-firewall-backend

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Adds a BSD platform or backend; release correctness of existing Linux support is not deferred. |
| Scope | plugin |
| Depends | `plan/post-release/features/spec-bsd-family-interface-backend.md` (P1), `plan/post-release/features/spec-bsd-qemu-guest-kind.md` |
| Phase | 0/0 (Do not start until the owner approves) |
| Handoff | - |
| Updated | 2026-10-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's firewall has two backends, `nft` (`internal/plugins/firewall/nft/`) and `vpp`
(`internal/plugins/firewall/vpp/`). Off Linux `internal/component/firewall/default_other.go`
sets an empty default, so any firewall section rejects at verify. This spec adds a
firewall backend for pf on FreeBSD and OpenBSD (and possibly macOS pf), lowering
Ze's firewall model into pf rules and tables.

Open design points, from the BSD research:

| Point | Notes |
|-------|-------|
| pf ABI differs per OS | FreeBSD pf (with its own `DIOC*` ioctls and, on recent releases, a netlink interface) and OpenBSD pf diverged long ago; macOS pf is an old OpenBSD fork. One backend per OS or a shared core with per-OS lowering |
| ioctl versus `pfctl` | The repository bans shelling out where a kernel interface exists (`ai/rules/go-standards.md`, external commands); the ioctl route needs hand-defined structs per OS |
| Anchors | Ze owns one pf anchor so the operator's own ruleset stays untouched, as Ze owns its nft tables on Linux |
| ipfw (FreeBSD) | Alternative to pf on FreeBSD; recommendation: pf only |
| Conntrack view | `show conntrack` equivalents read pf states |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - ownership of Ze's tables and flush on shutdown
  → Constraint: Ze must own a pf anchor the way it owns its nft tables, and flush only that anchor
- [ ] `docs/architecture/firewall/backend-command-dispatch.md` - how commands reach a backend
  → Constraint: the pf backend registers through the existing firewall backend registry

**Key insights:**
- [to be filled at design]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/default_other.go` - empty default backend off Linux; firewall config rejects at verify
- [ ] `internal/component/firewall/backend.go` - backend interface and registry

**Behavior to preserve:**
- Linux `nft` and `vpp` backends unchanged; off-Linux rejection stays until a pf backend registers.

**Behavior to change:**
- FreeBSD and OpenBSD (macOS if the owner chooses) default to a `pf` backend.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: firewall config tree]
- [Format at entry]

### Transformation Path
1. [to be designed]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to kernel | [pf ioctl on `/dev/pf`, per-OS ABI] | No |

### Integration Points
- [to be designed]

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze's firewall model lowers into pf without losing semantics | [to be checked against `model.go`] | [features refused per OS] | [mapping table at design] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | pf ioctl structs change between releases | [golden test on the guest] | [pin per minimum version] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Firewall config on FreeBSD | → | [pf backend apply] | [test name at design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Firewall config on FreeBSD/OpenBSD | Rules installed in Ze's pf anchor; traffic matched as configured |
| AC-2 | Ze shutdown | Only Ze's anchor flushed; operator rules untouched |
| AC-3 | Model feature pf cannot express | Refused by name at verify |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to be designed] | [pf backend package] | [rule lowering golden bytes] | |

## Files to Modify
- `internal/component/firewall/default_other.go` - narrow build tags
- `docs/guide/firewall.md` - platform section

## Files to Create
- [pf backend package under `internal/plugins/firewall/`]

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
