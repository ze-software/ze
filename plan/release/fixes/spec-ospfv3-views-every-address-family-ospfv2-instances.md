# Spec: ospfv3-views-every-address-family-ospfv2-instances

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Priority | P1 |
| Release reason | Shipped RFC 6549 OSPFv2 multi-instance: the neighbors, LSDB and routes of every instance but the base one are invisible to the operator |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Source: `plan/release/fixes/spec-ospfv3-views-every-address-family.md`, which
fixes the same shape for OSPFv3 address families and leaves OSPFv2 instances
to this spec (its Known Limitations).

OSPFv2 runs one engine per configured Instance ID (RFC 6549):
`instanceManager` (`internal/plugins/ospf/multi_instance.go`) holds the base
(Instance 0) engine and one more per non-zero Instance ID. `show ospf instance`
lists them all (`instanceSnapshot`), but every other OSPFv2 view in the
`OnExecuteCommand` switch of `internal/plugins/ospf/register.go`
(`show ospf neighbor`, `interface`, `database` and its subviews, `route`,
`spf`, `border-routers`, `virtual-links`, `te-database`, `graceful-restart`,
`segment-routing`, the detail views) reads `eng`, the base engine. A non-zero
instance's adjacencies, LSDB and routes have no view, the same defect the
source spec fixes for OSPFv3.

The fix follows the source spec's design: each view answers for every running
instance with rows tagged by `instance-id`, and a typed keyword narrows it to
one; the source spec's Key Design Decisions carry the reasoning. Design starts
when this spec moves to `design`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ospf/ospf-ext-12-multi-instance.md` - [fill during design]
  → Decision: [fill during design]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ospf/multi_instance.go` - `instanceManager`, `engineFor`, `instanceSnapshot`
- [ ] `internal/plugins/ospf/register.go` - the OSPFv2 view arms read `eng`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]

### Transformation Path
1. [fill during design]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon ↔ OSPF plugin | [fill during design] | No |

### Integration Points
- [fill during design]

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | [fill during design] | | | | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [fill during design] | → | [fill during design] | [fill during design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [fill during design] | [fill during design] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill during design] | `internal/plugins/ospf/` | | |

## Files to Modify
- `internal/plugins/ospf/register.go` - [fill during design]

## Implementation Steps

1. [fill during design]

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `view-reads-one-instance-of-several.md` | 2026-11-02 | OSPFv2 views read only the base instance |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] The targeted tests pass: the OSPF package tests, `./le test ospf` for the changed `.ci`, and `./le verify changed` over the closing commits
