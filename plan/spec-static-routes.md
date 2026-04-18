# Spec: static-routes -- Kernel static routes with multi-table support

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/fib/kernel/backend_linux.go` -- netlink route programming (current, single-table)
4. `internal/plugins/fib/kernel/register.go` -- plugin registration pattern for FIB plugins
5. `internal/component/iface/register.go` -- component registration pattern (registry.Register)

## Task

Add a static route component that programs user-configured routes into the Linux kernel via
netlink. Unlike the existing fibkernel plugin (which programs routes from the sysRIB pipeline,
always into the main table), this component manages config-driven routes with multi-table
support, interface-based next-hops, and blackhole routes.

The immediate need is the VyOS LNS Surfprotect setup:

```
set protocols static table 100 route 0.0.0.0/0 interface tun100
```

This is a default route in routing table 100 pointing to the GRE tunnel. Policy routing rules
(spec-policy-routing) direct Surfprotect HTTP/HTTPS traffic into table 100, which forwards it
through tun100 to the content filter.

### Why not extend fibkernel?

fibkernel reacts to EventBus messages from the sysRIB (protocol-learned routes selected by
admin distance). Static kernel routes are config-driven, owned by the user, not subject to
admin distance selection. They go directly to the kernel. Different lifecycle, different
source of truth.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/fib/kernel/backend_linux.go` -- netlink route programming
  --> Constraint: uses netlink.Route with Dst, Gw, Protocol fields; does NOT set Table field
  --> Decision: buildRoute creates route with rtprotZE=250
- [ ] `internal/plugins/fib/kernel/register.go` -- plugin registration pattern
  --> Constraint: registry.Registration with Name, YANG, ConfigRoots, RunEngine
  --> Decision: SDK 5-stage protocol (verify/apply/rollback/started/command)
- [ ] `internal/plugins/fib/kernel/fibkernel.go` -- crash recovery pattern
  --> Constraint: stale-mark-then-sweep using custom rtm_protocol
  --> Decision: RTPROT_ZE=250 identifies ze-installed routes
- [ ] `vendor/github.com/vishvananda/netlink/route.go` -- netlink.Route struct
  --> Constraint: Route.Table field (int) controls routing table; Route.LinkIndex for interface-based routes
- [ ] `internal/component/iface/register.go` -- component pattern
  --> Constraint: registry.Register in init(), ConfigRoots for YANG config delivery
- [ ] `plan/learned/522-fib-0-umbrella.md` -- FIB pipeline design decisions
  --> Decision: Bus-only communication between FIB components
  --> Decision: admin distance for sysRIB selection
  --> Decision: build-tag OS selection for backends

### RFC Summaries (MUST for protocol work)

Not protocol work. Linux routing tables are a kernel concept, not an RFC feature.

**Key insights:**
- netlink.Route.Table accepts any int (table ID 0-4294967295 in kernel, commonly 0-255 for compat)
- netlink.Route.LinkIndex specifies interface-based routes (resolve interface name to index via netlink.LinkByName)
- netlink.Route.Type controls route type (RTN_UNICAST, RTN_BLACKHOLE, RTN_UNREACHABLE, RTN_PROHIBIT)
- RTPROT_ZE=250 identifies ze routes; static routes should also use this protocol ID
- fibkernel manages sysRIB routes; static routes are a parallel path to the kernel
- Static routes must be reconciled at boot (add missing, remove orphaned) and on config reload

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/fib/kernel/backend_linux.go` -- buildRoute does NOT set Table, so routes go to main table. Uses net.ParseCIDR for prefix, net.ParseIP for next-hop. No interface-based route support.
  --> Constraint: routeBackend interface: addRoute(prefix, nextHop string), delRoute(prefix), replaceRoute(prefix, nextHop), listZeRoutes(), close()
- [ ] `internal/plugins/fib/kernel/register.go` -- ConfigRoots: ["fib.kernel"], RunEngine: SDK 5-stage protocol
  --> Constraint: OnConfigVerify/OnConfigApply/OnConfigRollback/OnStarted/OnExecuteCommand
- [ ] `internal/plugins/fib/kernel/schema/ze-fib-conf.yang` -- minimal config: flush-on-stop (bool), sweep-delay (uint16)
- [ ] No static route component exists anywhere in ze

**Behavior to preserve:**
- fibkernel continues to program sysRIB routes into the main table unchanged
- RTPROT_ZE=250 shared by both fibkernel and static routes (all ze routes identifiable)
- No collision: fibkernel and static routes manage different prefixes (fibkernel from sysRIB, static from config)

**Behavior to change:**
- Add new static route component that programs config-driven routes
- Support multi-table routing (table ID on each route)
- Support interface-based next-hops (LinkIndex instead of Gw)
- Support blackhole/unreachable routes

## Data Flow (MANDATORY)

### Entry Point
- YANG config file parsed at startup or reload by config component
- Config tree contains `static { ... }` section

### Transformation Path
1. Config file: `static { table 100 { route 0.0.0.0/0 { interface tun100; } } }`
2. YANG validation
3. Config parser: builds []StaticRoute from config JSON
4. Component calls backend.Apply(desired []StaticRoute)
5. Backend resolves interface names to link indices via netlink.LinkByName
6. Backend programs netlink.Route with Table, Dst, Gw/LinkIndex, Protocol=RTPROT_ZE
7. Reconciliation: list existing RTPROT_ZE routes in specified tables, add missing, remove orphaned, replace changed

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config --> Component | YANG tree JSON via SDK OnConfigure | [ ] |
| Component --> Backend | Apply([]StaticRoute) | [ ] |
| Backend --> Kernel | netlink RouteAdd/RouteDel/RouteReplace with Table field | [ ] |

### Integration Points
- `internal/component/plugin/registry/` -- Register() for component lifecycle
- `pkg/plugin/sdk/` -- SDK 5-stage protocol
- `vendor/github.com/vishvananda/netlink` -- Route programming (already vendored)
- `internal/component/iface/` -- interface names managed by iface component (static routes reference them)

### Architectural Verification
- [ ] No bypassed layers (config --> component --> backend --> kernel)
- [ ] No unintended coupling (static routes independent of fibkernel)
- [ ] No duplicated functionality (different source of truth from fibkernel)
- [ ] Zero-copy not applicable (config-driven, small data)

## Data Model

### StaticRoute

| Field | Type | Description |
|-------|------|-------------|
| Prefix | netip.Prefix | Destination prefix (0.0.0.0/0, 10.0.0.0/8, etc.) |
| Table | uint32 | Routing table ID (0 = main table, 100 = custom, etc.) |
| NextHop | netip.Addr | Gateway IP address (mutually exclusive with Interface) |
| Interface | string | Output interface name (mutually exclusive with NextHop) |
| RouteType | RouteType | unicast (default), blackhole, unreachable, prohibit |
| Metric | uint32 | Route metric/priority (optional, default 0) |

### RouteType enum

| Value | Kernel constant | Description |
|-------|----------------|-------------|
| RouteUnicast | RTN_UNICAST | Normal forwarding route (default) |
| RouteBlackhole | RTN_BLACKHOLE | Silently discard |
| RouteUnreachable | RTN_UNREACHABLE | ICMP unreachable |
| RouteProhibit | RTN_PROHIBIT | ICMP prohibited |

### Backend interface

| Method | Signature | Semantics |
|--------|-----------|-----------|
| Apply | `Apply(desired []StaticRoute) error` | Full reconciliation: list current ze static routes in targeted tables, add missing, remove orphaned, replace changed |
| List | `List() ([]StaticRoute, error)` | Read current ze static routes for CLI show |
| Close | `Close() error` | Release netlink handle |

## Config Syntax

```
static {
    # Routes in the main table (table 0 / default)
    route 10.0.0.0/8 {
        next-hop 192.168.1.1
    }
    route 192.168.100.0/24 {
        next-hop 10.0.0.1
        metric 100
    }
    route 198.51.100.0/24 {
        blackhole
    }

    # Routes in a named/numbered table
    table 100 {
        route 0.0.0.0/0 {
            interface tun100
        }
    }
    table 200 {
        route 0.0.0.0/0 {
            next-hop 10.0.0.1
        }
    }
}
```

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | --> | Feature Code | Test |
|-------------|-----|--------------|------|
| Config with `static { route ... }` at boot | --> | registry.Register, OnConfigure, Apply, RouteAdd | `test/static/001-boot-apply.ci` |
| Config with `static { table 100 { route ... } }` | --> | Apply with Table=100 | `test/static/002-table-route.ci` |
| Config reload adds route | --> | OnConfigReload, Apply reconciliation | `test/static/003-reload.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `route 10.0.0.0/8 { next-hop 192.168.1.1; }` | Route added to main table with RTPROT_ZE |
| AC-2 | Config with `table 100 { route 0.0.0.0/0 { interface tun100; } }` | Default route in table 100 via tun100 with RTPROT_ZE |
| AC-3 | Config with `route X { blackhole; }` | Blackhole route (RTN_BLACKHOLE) installed |
| AC-4 | Config reload removes a route | Route deleted from kernel |
| AC-5 | Config reload adds a route to table 100 | Route added to table 100, other tables untouched |
| AC-6 | ze boots with static config | All static routes programmed before ze is "running" |
| AC-7 | Interface name in config does not exist | Apply returns error, logged, other routes still applied |
| AC-8 | `ze static show` | Displays all ze static routes with table, prefix, next-hop/interface |
| AC-9 | Route with metric | Route installed with specified metric |
| AC-10 | ze restarts, stale routes exist | Stale-mark-sweep removes orphaned routes |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStaticRouteModel` | `internal/component/staticroute/model_test.go` | StaticRoute struct, RouteType enum, validation | |
| `TestParseStaticConfig` | `internal/component/staticroute/config_test.go` | Config JSON to []StaticRoute, table ID, next-hop vs interface | |
| `TestParseStaticConfigTable` | `internal/component/staticroute/config_test.go` | Table 100 route parsing | |
| `TestParseStaticConfigBlackhole` | `internal/component/staticroute/config_test.go` | Blackhole route parsing | |
| `TestParseStaticConfigMetric` | `internal/component/staticroute/config_test.go` | Metric field parsing | |
| `TestStaticRouteRegistration` | `internal/component/staticroute/register_test.go` | registry.Register succeeds | |
| `TestFormatStaticRoutes` | `internal/component/staticroute/cmd/show_test.go` | CLI show output formatting | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Table ID | 0-4294967295 | 4294967295 | N/A (0 is main table) | N/A (uint32) |
| Metric | 0-4294967295 | 4294967295 | N/A (0 is valid) | N/A (uint32) |
| Prefix bits | 0-32 (IPv4), 0-128 (IPv6) | 32/128 | N/A (0 is default route) | 33/129 (parse error) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Boot apply | `test/static/001-boot-apply.ci` | Config with static routes, verify in kernel | |
| Table route | `test/static/002-table-route.ci` | Route in table 100, verify with `ip route show table 100` | |
| Reload | `test/static/003-reload.ci` | Add/remove routes on reload | |

### Future (if deferring any tests)
- None. All tests required for LNS replacement.

## Files to Modify

No existing files modified. This is a new component.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/component/staticroute/schema/ze-static-conf.yang` |
| CLI commands/flags | Yes | `internal/component/staticroute/cmd/show.go` |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test | Yes | `test/static/*.ci` |
| Plugin import generation | Yes | `scripts/gen-plugin-imports.go` scans internal/component/ |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- static routes with multi-table support |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- static route config section |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- `ze static show` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- staticroute component |
| 6 | Has a user guide page? | Yes | `docs/guide/static-routes.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- static routes with multi-table |
| 12 | Internal architecture changed? | No | Follows existing patterns |

## Files to Create

- `internal/component/staticroute/model.go` -- StaticRoute struct, RouteType enum, validation
- `internal/component/staticroute/config.go` -- Config JSON to []StaticRoute parsing
- `internal/component/staticroute/backend.go` -- Backend interface (Apply, List, Close)
- `internal/component/staticroute/backend_linux.go` -- netlink backend (RouteAdd/Del/Replace with Table field)
- `internal/component/staticroute/backend_other.go` -- noop backend for non-Linux
- `internal/component/staticroute/register.go` -- registry.Register in init(), RunEngine
- `internal/component/staticroute/schema/ze-static-conf.yang` -- YANG schema
- `internal/component/staticroute/schema/register.go` -- YANG module registration
- `internal/component/staticroute/schema/embed.go` -- embed YANG file
- `internal/component/staticroute/cmd/show.go` -- CLI show formatting
- `internal/component/staticroute/model_test.go` -- model tests
- `internal/component/staticroute/config_test.go` -- config parsing tests
- `internal/component/staticroute/register_test.go` -- registration tests
- `internal/component/staticroute/cmd/show_test.go` -- CLI formatting tests
- `test/static/001-boot-apply.ci` -- functional test
- `test/static/002-table-route.ci` -- functional test
- `test/static/003-reload.ci` -- functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6-12 | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Data model** -- StaticRoute struct, RouteType, validation
   - Tests: TestStaticRouteModel
   - Files: model.go, model_test.go
   - Verify: tests fail --> implement --> tests pass

2. **Phase: YANG + config parsing** -- schema and JSON parser
   - Tests: TestParseStaticConfig, TestParseStaticConfigTable, TestParseStaticConfigBlackhole, TestParseStaticConfigMetric
   - Files: ze-static-conf.yang, config.go, config_test.go, schema/*.go
   - Verify: tests fail --> implement --> tests pass

3. **Phase: Backend** -- netlink route programming with table support
   - Tests: integration tests (Linux only)
   - Files: backend.go, backend_linux.go, backend_other.go
   - Verify: tests fail --> implement --> tests pass

4. **Phase: Component reactor** -- registration and lifecycle
   - Tests: TestStaticRouteRegistration
   - Files: register.go, register_test.go
   - Verify: tests fail --> implement --> tests pass

5. **Phase: CLI** -- show command
   - Tests: TestFormatStaticRoutes
   - Files: cmd/show.go, cmd/show_test.go
   - Verify: tests fail --> implement --> tests pass

6. **Functional tests** --> Boot apply, table routing, reload
7. **Full verification** --> `make ze-verify`
8. **Complete spec** --> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1 through AC-10) has implementation with file:line |
| Correctness | Table ID correctly set on netlink.Route; interface resolved to LinkIndex |
| Naming | Config keys use lowercase (static, table, route, next-hop, interface, blackhole) |
| Data flow | Config --> StaticRoute --> backend.Apply --> netlink.RouteAdd with Table |
| Reconciliation | Apply lists existing ze routes per table, adds missing, removes orphaned |
| RTPROT_ZE | All static routes use Protocol=250 (same as fibkernel) |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| staticroute component exists | `ls internal/component/staticroute/` |
| YANG schema exists | `ls internal/component/staticroute/schema/ze-static-conf.yang` |
| Backend supports table ID | `grep "Table" internal/component/staticroute/backend_linux.go` |
| Backend supports interface route | `grep "LinkIndex" internal/component/staticroute/backend_linux.go` |
| register.go registers with engine | `grep "registry.Register" internal/component/staticroute/register.go` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Table ID validated (uint32); prefix validated (netip.ParsePrefix) |
| Input validation | Interface name validated (non-empty, exists on system at Apply time) |
| Privilege | Requires CAP_NET_ADMIN for netlink route operations |
| Route isolation | Only manages RTPROT_ZE routes; never touches non-ze routes |
| Table isolation | Reconciliation scoped per table (only touch tables mentioned in config) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior --> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural --> DESIGN phase |
| Functional test fails | Check AC; if AC wrong --> DESIGN; if AC correct --> IMPLEMENT |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Separate component from fibkernel | Different source of truth (config vs sysRIB), different lifecycle (boot-time vs convergence-driven), different scope (user routes vs protocol routes). Mixing them would conflate two concerns. |
| 2 | Same RTPROT_ZE=250 as fibkernel | All ze-managed routes should be identifiable by the same protocol ID for crash recovery and `ip route show proto ze`. No collision risk since they manage different prefixes. |
| 3 | Apply does full reconciliation | Same pattern as firewall Apply: receive desired state, diff against kernel, add/remove/replace. Simple, correct, idempotent. |
| 4 | Interface resolution at Apply time, not config parse time | Interfaces may not exist yet at parse time (e.g., tun100 created by another component). Resolve name to LinkIndex in the backend when programming the route. |
| 5 | Component in internal/component/ not internal/plugins/ | Static routes are config-driven with their own YANG, like iface and firewall. Plugins in internal/plugins/ are protocol-reactive (fibkernel, sysrib, bfd). |
| 6 | Per-table reconciliation | Only touch tables mentioned in config. If table 100 is in config, reconcile table 100 ze routes. Never touch tables not in config (avoids destroying fibkernel's main table routes). |

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

## Implementation Summary

### What Was Implemented
- (To be filled after implementation)

### Bugs Found/Fixed
- (To be filled)

### Documentation Updates
- (To be filled)

### Deviations from Plan
- (To be filled)

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
- **Partial:**
- **Skipped:**
- **Changed:**

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-static-routes.md`
- [ ] Summary included in commit
