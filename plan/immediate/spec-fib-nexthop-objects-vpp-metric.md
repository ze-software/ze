# Spec: fib-nexthop-objects-vpp-metric -- Linux nexthop objects, VPP metric, TableID production and table install

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | spec-vrf-0-umbrella.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Split out of `plan/immediate/spec-fib-depth.md` by the owner on 2026-10-08.

-> Decision (owner, 2026-10-08): AC-4 (Linux nexthop objects) and AC-8 (VPP metric), plus the TableID production they need, move out of `spec-fib-depth` into this spec, which depends on `plan/spec-vrf-0-umbrella.md`.

Three pieces of FIB programming differ from what `spec-fib-depth` originally
required, and the owner moved them here:

1. **Linux nexthop objects (AC-4).** The kernel backend's `buildRichRoute`
   (`internal/plugins/fib/kernel/`) emits per-route `MultiPath` for an ECMP
   change. The requirement is a Linux nexthop group, with the route pointing
   at the group ID. Existing per-route multipath does not prove it.
2. **VPP metric (AC-8).** The VPP backend's `richRouteAddDel`
   (`internal/plugins/fib/vpp/backend.go`) uses explicit next-hop weights and
   does not read `r.Metric`. Explicit path weight and route metric are separate
   inputs today.
3. **TableID production.** `fibChange` (`internal/component/sysrib/fibimport.go`)
   leaves `BestChangeEntry.TableID` zero, although both backends can consume a
   supplied value. Populating the table dimension end to end, including safe
   per-table route identity, waits on the VRF design in
   `plan/spec-vrf-0-umbrella.md`.

The source facts above are the parent spec's 2026-09-19 reading, carried as
claims; the design phase re-reads each producer before writing Current
Behavior.

## Acceptance Criteria

Moved verbatim from `plan/immediate/spec-fib-depth.md`:

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-4 | ECMP change emitted to kernel backend | Linux nexthop group created, route points to nhg ID |
| AC-8 | BestChangeEntry with Metric set | Kernel: route.Priority = metric, VPP: route weight |
| AC-9 | BestChangeEntry with TableID != 0 | Kernel: route installed in table N, VPP: route in table N |

-> Decision (main thread, 2026-10-08): AC-9 moves here with the TableID producer. A route installed in table N cannot be proven end to end while `BestChangeEntry.TableID` is never populated, so the AC follows the producer it depends on.

The parent's wording on TableID production, moved verbatim: "Populate the
table dimension end to end under the VRF design, including safe per-table
route identity; backend TableID consumption alone does not satisfy AC-9."

## Related

| Spec | Relation |
|------|----------|
| `plan/immediate/spec-fib-depth.md` | Parent; it keeps every other AC, including AC-14 |
| `plan/spec-vrf-0-umbrella.md` | Dependency; the VRF design decides how a table reaches `BestChangeEntry.TableID` |

<!-- Skeleton: every section below is written at design time (plan/TEMPLATE.md). -->

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [chosen at design time]

## Current Behavior (MANDATORY)

**Source files read:** (the design phase reads each before writing this section)
- [ ] `internal/plugins/fib/kernel/backend_linux.go` - [`buildRichRoute` per-route MultiPath; read at design time]
- [ ] `internal/plugins/fib/vpp/backend.go` - [`richRouteAddDel` weights; read at design time]
- [ ] `internal/component/sysrib/fibimport.go` - [`fibChange` TableID; read at design time]

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
| BestChangeEntry with TableID | → | kernel backend route in table N | `TestKernelVRFTable` (AC-9, carried from the parent; confirmed at design time) |
| [rest written at design time] | → | | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [written at design time] | | | |

### Functional Tests
Moved verbatim from `plan/immediate/spec-fib-depth.md` with AC-9 (2026-10-09):

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-fib-vrf-table` | `test/bgp/fib-vrf-table.ci` | Route installed in non-default table | |

## Files to Modify
- `test/bgp/fib-vrf-table.ci` -- functional test (moved from `spec-fib-depth` with AC-9)
- [rest written at design time]

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
