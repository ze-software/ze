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

### RFC Summaries
Moved verbatim from `plan/immediate/spec-fib-depth.md` (2026-10-09):
- [ ] `rfc/short/rfc4364.md` -- BGP/MPLS IP VPNs (VRF route installation)
  → Constraint: VPN routes install into per-VRF tables identified by RD

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

Design decisions moved verbatim from `plan/immediate/spec-fib-depth.md` (2026-10-09):

| Decision | Detail | Rationale |
|----------|--------|-----------|
| Kernel backend uses nexthop objects | Linux 5.3+ `ip nexthop` API for NH groups rather than per-route multipath expansion | Atomic failover, shared NH state across routes, matches FRR/iproute2 direction |
| VRF table wired through BestChangeEntry.TableID | FIB backends use this to program into the correct kernel table or VPP table | Unblocks vrf-0-umbrella FIB programming without changing backend interfaces |

Security review rows moved verbatim from `plan/immediate/spec-fib-depth.md` (2026-10-09):

| Check | What to look for |
|-------|-----------------|
| Input validation | TableID from untrusted BGP peer must be ignored (only config/sysrib sets it) |
| Privilege | Nexthop object creation requires CAP_NET_ADMIN (already held by fib-kernel) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BestChangeEntry with TableID | → | kernel backend route in table N | `TestKernelVRFTable` (AC-9, carried from the parent; confirmed at design time) |
| [rest written at design time] | → | | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestKernelMetric` | `internal/plugins/fib/kernel/fibkernel_test.go` | Priority field set | |
| `TestKernelTable` | `internal/plugins/fib/kernel/fibkernel_test.go` | Route in specified table | |
| `TestKernelNexhopGroup` | `internal/plugins/fib/kernel/fibkernel_test.go` | ECMP via nexthop objects | |
| `TestVPPTable` | `internal/plugins/fib/vpp/fibvpp_test.go` | Per-change table override | |
| [rest written at design time] | | | |

The four rows above moved verbatim from `plan/immediate/spec-fib-depth.md` (2026-10-09).

### Boundary Tests
Moved verbatim from `plan/immediate/spec-fib-depth.md` (2026-10-09):

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| TableID | 0-4294967295 | 4294967295 | N/A (0=default) | N/A (uint32) |
| Metric | 0-4294967295 | 4294967295 | N/A (0=best) | N/A (uint32) |

### Functional Tests
Moved verbatim from `plan/immediate/spec-fib-depth.md` with AC-9 (2026-10-09):

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-fib-vrf-table` | `test/bgp/fib-vrf-table.ci` | Route installed in non-default table | |

## Files to Modify
- `test/bgp/fib-vrf-table.ci` -- functional test (moved from `spec-fib-depth` with AC-9)
- `internal/plugins/fib/kernel/nexthop_linux.go` -- Linux nexthop object management (moved verbatim from `spec-fib-depth` Files to Create, 2026-10-09)
- `internal/plugins/fib/kernel/backend_linux.go` -- netlink nexthop objects, table, metric (split from `spec-fib-depth`, which keeps route types and MPLS)
- `internal/plugins/fib/vpp/backend.go` -- table override (split from `spec-fib-depth`, which keeps multi-path FibPath)
- `internal/plugins/fib/kernel/fibkernel_test.go`, `internal/plugins/fib/vpp/fibvpp_test.go` -- `TestKernelMetric`, `TestKernelTable`, `TestKernelNexhopGroup`, `TestVPPTable`
- [rest written at design time]

Deliverable check moved verbatim from `spec-fib-depth` (2026-10-09):

| Deliverable | Verification method |
|-------------|---------------------|
| Kernel backend creates nexthop objects | `grep -rn "nexthop" internal/plugins/fib/kernel/` shows implementation |

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
