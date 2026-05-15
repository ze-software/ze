# Spec: gap-2-static-route-enhancements

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 7/8 |
| Updated | 2026-05-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/static/schema/ze-static-conf.yang` - static route schema
4. `internal/plugins/static/config.go` - Route config extraction
5. `internal/plugins/static/backend_linux.go` - netlink route programming
6. `internal/plugins/static/model.go` - Route and NextHop model types

## Task

Two related gaps in the static route plugin needed for VyOS config parity:

**Gap A: Routing table selection.** VyOS uses `protocols static table 100 route 0.0.0.0/0 interface tun100` to install routes in specific routing tables (used for policy-based routing). Ze's static plugin only installs routes in the main table. Netlink RouteAdd already supports setting a Table field; the YANG and config extraction need to expose it.

**Gap B: Interface-only next-hop.** VyOS uses `route 0.0.0.0/0 { interface pppoe0 { distance 230 } }` for routes with no gateway address (next-hop is the interface itself). Ze's static plugin requires `next-hop { address ... }` with a gateway IP. The YANG needs a sibling `list interface-next-hop` inside the `forward` case for interface-only forwarding, and the backend needs to handle routes with only an output interface.

**Motivation:** VyOS lns.conf uses `protocols static table 100 route 0.0.0.0/0 interface tun100` for PBR via a GRE tunnel. VyOS home.conf uses `route 0.0.0.0/0 { interface pppoe0 { distance 230 } }` for a default route out the PPPoE interface with no explicit gateway. Ze config equivalents: `routing-table { lns { id 100 } } static { table lns { route 0.0.0.0/0 { interface-next-hop tun100 { } } } }` and `static { table default { route 0.0.0.0/0 { metric 230; interface-next-hop pppoe0 { } } } }`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - plugin pattern, config flow
  → Constraint: Static plugin reads config tree, programs kernel via netlink
- [ ] `internal/plugins/static/schema/ze-static-conf.yang` - current route schema with choice action (forward/discard/unreachable)
  → Decision: Add top-level `routing-table` registry (`list routing-table` keyed by `name`, with `leaf id` uint32 range `1..252 | 256..4294967295`) defining name-to-table-ID mapping. Replace top-level `list route` with `list table` (keyed by `name`, string) containing `list route`; table name resolved via registry (`default` is built-in, maps to table 0 / RT_TABLE_MAIN 254). Add `list interface-next-hop` (keyed by `interface`) as sibling inside the `forward` case, not a separate choice case (YANG choice is mutually exclusive; a separate case would block mixed ECMP). No parser enhancement needed: each table has its own `list route` with independent `key "prefix"` uniqueness. Forward-compatible: VRF spec (`spec-vrf-0`) absorbs routing-table registry into `vrf <name> { table N }` context delivery
- [ ] `internal/plugins/static/backend_linux.go` - netlink route programming via buildRoute, applyRoute, removeRoute
  → Constraint: rtnetlink Route struct has Table field; Table 0 = RT_TABLE_UNSPEC, kernel defaults to main (254)

**Key insights:**
- Top-level `routing-table` registry maps names to kernel table IDs: `routing-table { surfprotect { id 100 } }`. `default` is built-in (table 0, kernel RT_TABLE_MAIN 254). Registry is the single source of name-to-ID mapping, shared by static routes and policy routing. VRF spec later absorbs it into `vrf <name> { table N }`
- All routes live under `list table` (keyed by `name`): `table default { route ... }` for main table, `table surfprotect { route ... }` for named PBR tables. Table name resolved via routing-table registry at parse time
- No parser enhancement needed: each table has its own `list route` with independent `key "prefix"` uniqueness, so same prefix in different tables (AC-8) works naturally
- Interface-only next-hop reuses `actionForward` with zero Address in the Go model; YANG adds `list interface-next-hop` (keyed by `interface`) as a sibling list inside the `forward` case alongside `list next-hop`; both lists can coexist in one route for mixed ECMP
- Metric on interface-only routes maps to kernel route priority
- Mixed ECMP (gateway + interface-only in same group) is allowed; kernel handles it deterministically
- BFD profiles are disallowed on interface-only next-hops (BFD requires a peer address)
- Redistribution events only emitted for table 0 (default); PBR table routes are not redistributed into BGP
- `sortedNextHops` in diff.go must use Interface as tiebreaker when Address compares equal (multiple zero-addr entries)
- `watchBFD` goroutine closure must capture table ID (nested map lookup needs both table and prefix)
- Config tree shape verified: `yang_schema.go` `flattenChildren` / `flattenChoiceCases` makes choice/case transparent; sibling lists inside a case appear as sibling keys in `map[string]any`
- VPP backend (`vpp/backend.go`) has `tableID uint32` on Backend struct set at construction time; this is a single-table binding, not per-route table selection; VPP per-route table support is future work (no changes needed now)
- `installedStaticRoute` needs `table` field for listRoutes
- `showRoutes` sort needs Table as secondary key for stable ordering when same prefix appears in different tables; NOTE: existing string-based prefix comparison (`a.Prefix < b.Prefix`) is pre-existing and incorrect for IP ordering but out of scope for this spec

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/static/schema/ze-static-conf.yang` - route list with choice action: forward (next-hop keyed by address), discard (blackhole), unreachable (reject)
- [ ] `internal/plugins/static/config.go` - parseStaticConfig returns []staticRoute; parseRoute, parseNextHop; dedup via `seen[Prefix]` map; requires Address on all next-hops
- [ ] `internal/plugins/static/model.go` - staticRoute (Prefix, Description, Metric, Tag, Action actionType, NextHops []nextHop); nextHop (Address, Interface, Weight, BFDProfile)
- [ ] `internal/plugins/static/backend_linux.go` - buildRoute constructs netlink.Route; applyRoute calls RouteReplace; removeRoute calls RouteDel; listRoutes filters by rtprotStatic
- [ ] `internal/plugins/static/inject.go` - routeManager with `routes map[netip.Prefix]*routeState`; emitRouteChange; watchBFD captures prefix in closure; showRoutes returns []showRoute (no Table field)

**Behavior to preserve:**
- Existing gateway next-hop routes work unchanged (AC-5)
- Blackhole and reject routes work unchanged
- ECMP with weighted next-hops unchanged
- BFD-backed next-hops unchanged
- Redistribution events for static routes unchanged

**Behavior to change:**
- New `routing-table` plugin owns the name-to-ID registry; exposes `Resolve(name string) (uint32, error)` for other plugins; `default` is built-in (0)
- Static plugin YANG replaces top-level `list route` with `list table` (keyed by `name`, string) containing `list route`
- staticRoute gains Table uint32 field (0 = default/main, kernel resolves to RT_TABLE_MAIN 254)
- routeManager.routes becomes `map[uint32]map[netip.Prefix]*routeState` (nested by table, then prefix)
- parseStaticConfig receives the routing-table registry; iterates table entries, resolves names to IDs, then parses routes within each
- parseNextHop validation relaxed: requires Address OR Interface (not both mandatory)
- parseNextHop rejects BFD profile on interface-only next-hops (BFD needs peer address)
- Mixed ECMP (gateway + interface-only in same group) allowed
- buildRoute passes Table to netlink Route struct
- buildRoute checks Address.IsValid() before setting Gw (single-path and multipath)
- listRoutes includes table info in returned installedStaticRoute
- showRoute and showNH gain Table field for display
- sortedNextHops uses Interface as tiebreaker when Address compares equal
- watchBFD closure captures table ID for nested map lookup
- emitRouteChange skips emission for non-zero table (PBR routes not redistributed)
- RouteChangeEntry gains Table uint32 field (for future use; currently always 0)

## Data Flow (MANDATORY)

### Entry Point
- Config commit with `routing-table { surfprotect { id 100 } } static { table surfprotect { route 0.0.0.0/0 { interface-next-hop tun100 { } } } }`

### Transformation Path
1. `routing-table` plugin parses registry: `surfprotect` -> 100; `default` built-in -> 0. Exposes `Resolve(name) (uint32, error)`
2. Static plugin YANG validates: `list table` (keyed by `name`) containing `list route` (keyed by `prefix`); action choice `forward` case contains both `list next-hop` and sibling `list interface-next-hop`
3. parseStaticConfig resolves table names via registry (`default`=0, `surfprotect`=100); within each table, parseRoute merges both next-hop lists into unified `[]nextHop` with zero Address for interface-only entries; dedup keyed on `Prefix` per table
4. Diff engine compares old/new routes per-table (nested map), then per-prefix; sortedNextHops uses Interface as tiebreaker
5. buildRoute programs route via netlink with Table field set; checks Address.IsValid() before setting Gw on both single-path and multipath entries

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree -> Route model | config.go parseStaticConfig / parseRoute | [ ] |
| Route model -> netlink | backend_linux.go buildRoute / applyRoute | [ ] |
| Route model -> redistribution | inject.go emitRouteChange | [ ] |

### Integration Points
- `internal/plugins/static/inject.go` - redistribution events only emitted for table 0 (main); non-main table routes are PBR-only and not redistributed
- `internal/core/redistevents/events.go` - RouteChangeEntry gains Table uint32 field (always 0 for now; field exists for future filtering)
- `internal/plugins/policyroute/` - PBR sets table, static routes can populate those tables
- `internal/component/iface/backend.go` - interface name to index resolution (used by buildRoute for interface-only next-hops)

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config commit with `table <name> { route ... }` | → | Route.Table (resolved via registry) passed to netlink | `test/static/table-routes.ci` |
| config commit with interface-next-hop | → | interface-only next-hop programmed (zero Gw, LinkIndex set) | `test/static/table-routes.ci` |
| config commit with mixed ECMP | → | multipath with both Gw and interface-only entries | `test/static/table-routes.ci` |
| config delete of table route | → | route removed from correct table | `test/static/table-routes.ci` |
| same prefix in two tables | → | both routes installed independently | `test/static/table-routes.ci` |
| show static routes with table | → | showRoutes includes Table field, sorted by (Prefix, Table) | `TestRouteManagerShowRoutesWithTable` |
| interface-only NH with BFD profile | → | parseNextHop rejects with error | `TestRejectBFDOnInterfaceOnly` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `routing-table { lns { id 100 } }` + `table lns { route ... }` | Route installed in routing table 100 instead of main |
| AC-2 | Interface-only next-hop (no gateway address) | Route created with output interface, no gateway IP |
| AC-3 | Interface-only next-hop with metric | Route priority set from metric value |
| AC-4 | Named table + interface-only combined | Route in specified table with interface-only forwarding |
| AC-5 | Existing gateway next-hop routes | No regression; gateway routes work exactly as before |
| AC-6 | Deletion of a table route | Route removed from the correct table (not main) |
| AC-7 | Mixed ECMP (gateway + interface-only in same group) | Route programmed with multipath containing both Gw and interface-only entries |
| AC-8 | Same prefix in `table default` and `table 100` | Both routes installed independently; no map key collision |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractRouteInTable` | `internal/plugins/static/config_table_test.go` | Route in `table 100 { ... }` gets Table=100; route in `table default { ... }` gets Table=0 | |
| `TestExtractInterfaceOnlyRoute` | `internal/plugins/static/config_table_test.go` | Interface-only next-hop: zero Address, non-empty Interface | |
| `TestExtractTableAndInterface` | `internal/plugins/static/config_table_test.go` | Combined table + interface-only | |
| `TestExtractMixedECMP` | `internal/plugins/static/config_table_test.go` | Gateway + interface-only next-hops in same route | |
| `TestExtractSamePrefixDifferentTables` | `internal/plugins/static/config_table_test.go` | Same prefix in table 0 and table 100 both accepted | |
| `TestRejectNextHopNoAddressNoInterface` | `internal/plugins/static/config_table_test.go` | Next-hop with neither Address nor Interface rejected | |
| `TestRejectBFDOnInterfaceOnly` | `internal/plugins/static/config_table_test.go` | Interface-only next-hop with bfd-profile rejected | |
| `TestRejectReservedTableID` | `internal/plugins/static/config_table_test.go` | Table IDs 253, 254, 255 rejected by range validation | |
| `TestExistingGatewayRouteUnchanged` | `internal/plugins/static/config_table_test.go` | No regression on gateway routes | |
| `TestRoutesEqualWithTable` | `internal/plugins/static/diff_test.go` | Routes differing only by Table are not equal | |
| `TestSortedNextHopsInterfaceTiebreak` | `internal/plugins/static/diff_test.go` | Two interface-only NHs sorted stably by Interface name | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| routing-table id | 1..252, 256..4294967295 | 252, 256, 4294967295 | 0 (use `default`), 253-255 (reserved) | 4294967296 |
| metric | 0-4294967295 | 4294967295 | N/A | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-table-route` | `test/static/table-routes.ci` | Route in table 100 programmed via netlink | |
| `test-interface-route` | `test/static/table-routes.ci` | Interface-only default route | |
| `test-mixed-ecmp` | `test/static/table-routes.ci` | ECMP group with gateway + interface-only entries | |
| `test-same-prefix-two-tables` | `test/static/table-routes.ci` | Same prefix in main and table 100 coexist | |

## Files to Modify
- `internal/plugins/static/schema/ze-static-conf.yang` - replace top-level `list route` with `list table` (keyed by `name`, string) containing `list route`; add `list interface-next-hop` (keyed by `interface`) as sibling inside `forward` case with leaves: `interface` (key, string), `weight` (uint16, range 1..65535, default 1)
- `internal/plugins/static/config.go` - rewrite parseStaticConfig to iterate table entries; resolve table names via routing-table registry; add `parseInterfaceNextHop` to handle `interface-next-hop` list entries (zero Address, weight); relax parseNextHop (Address OR Interface, not both mandatory); reject BFD profile on interface-only; merge both next-hop lists into unified `[]nextHop`
- `internal/plugins/static/model.go` - add Table uint32 to staticRoute (no new types)
- `internal/plugins/static/backend_linux.go` - pass Table to netlink Route struct; check Address.IsValid() before setting Gw in buildRoute (both single-path and multipath branches); add table field to installedStaticRoute in listRoutes
- `internal/plugins/static/backend.go` - add `table` field to installedStaticRoute
- `internal/plugins/static/backend_other.go` - no changes needed (staticRoute struct change is implicit)
- `internal/plugins/static/diff.go` - add Table to routesEqual comparison; add Interface tiebreaker to sortedNextHops
- `internal/plugins/static/inject.go` - change routes map to `map[uint32]map[netip.Prefix]*routeState` (nested by table, then prefix); update applyRoutes, setBFD, shutdown, showRoutes for nested iteration; add Table to showRoute struct; sort showRoutes by (Prefix, Table); watchBFD closure captures table; emitRouteChange skips non-zero table
- `internal/plugins/static/register.go` - update callers of parseStaticConfig (lines 60, 85, 101) to pass the routing-table registry; inject registry dependency into plugin startup
- `internal/core/redistevents/events.go` - add Table uint32 to RouteChangeEntry

## Existing Tests to Update
- `internal/plugins/static/config_test.go` - **ALL 21 tests** need JSON input updated: top-level `"route":[...]` becomes `"table":[{"name":"default","route":[...]}]` (or equivalent tree shape after YANG change). `TestParseStaticConfigDuplicatePrefix` additionally changes semantics: same prefix is only duplicate within the same table. Signature also changes if parseStaticConfig takes a registry argument (tests must pass a default-only registry or nil).
- `internal/plugins/static/diff_test.go` - existing tests pass unchanged (Table defaults to 0); add `TestRoutesEqualWithTable` and `TestSortedNextHopsInterfaceTiebreak`
- `internal/plugins/static/inject_test.go` - `TestRouteManagerApplyRoutes`, `TestRouteManagerRemoveOnReload`, `TestRouteManagerSkipsUnchangedRoutes`, `TestRouteManagerShutdownRemovesRoutes` all pass unchanged (Table 0, flat list input); `rm.routes` type changes but tests access through public methods only; verify mock still compiles

## Files to Create
- `internal/plugins/routingtable/` - routing-table registry plugin: YANG schema, config parsing, `Resolve(name string) (uint32, error)`, registration. Exposes a package-level `Registry` (populated at config parse, queried by static plugin via direct import). Direct import is acceptable because the registry is a leaf package with no reverse dependency on static.
- `internal/plugins/routingtable/schema/ze-routing-table-conf.yang` - `list routing-table` keyed by `name`, with `leaf id` (uint32, mandatory, range `1..252 | 256..4294967295`)
- `internal/plugins/static/config_table_test.go` - unit tests for table, interface-only, mixed ECMP, same-prefix-different-tables, BFD rejection
- `test/static/table-routes.ci` - functional tests

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Routing-table registry plugin | [x] | `internal/plugins/routingtable/` (new plugin) |
| YANG schema (new fields) | [x] | `internal/plugins/static/schema/ze-static-conf.yang` |
| CLI commands/flags | [ ] | N/A - existing show/set commands cover new fields |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Redistribution event struct | [x] | `internal/core/redistevents/events.go` |
| VPP backend | [ ] | N/A - `vpp/backend.go` has `tableID` on Backend struct as single-table binding set at construction; does not read per-route Table. VPP per-route table selection is future work. Path.SwIfIndex already supports interface-only forwarding. No code changes needed now. |
| Functional test | [x] | `test/static/table-routes.ci` |
| Static plugin register.go | [x] | `internal/plugins/static/register.go` (inject registry, update 3 parseStaticConfig call sites) |
| Existing test updates | [x] | `config_test.go` (all 21 tests: JSON format + signature), `diff_test.go` (new tests), `inject_test.go` (verify compile) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - table routes + interface-only next-hop |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - static route table and interface-only syntax |
| 6 | Has a user guide page? | [x] | `docs/guide/static-routes.md` (if exists) |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |

### Implementation Phases

1. **Phase: Routing-table registry plugin** - New plugin at `internal/plugins/routingtable/`; YANG schema for `list routing-table` (keyed by `name`, with `leaf id` uint32); config parsing; `Resolve(name) (uint32, error)` with `default` built-in; registration
   - Tests: `TestResolveDefault`, `TestResolveNamed`, `TestRejectReservedTableID`, `TestRejectUnknownName`
   - Files: `internal/plugins/routingtable/`
   - Verify: tests fail -> implement -> tests pass
2. **Phase: YANG schema** - Replace top-level `list route` with `list table` (keyed by `name`) containing `list route`; add `list interface-next-hop` (keyed by `interface`) as sibling inside `forward` case (not a separate choice case)
   - Tests: `TestExtractRouteInTable`
   - Files: `ze-static-conf.yang`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: Model + config extraction** - Add Table to staticRoute; rewrite parseStaticConfig to resolve table names via registry; relax nextHop validation (Address OR Interface); reject BFD on interface-only; merge interface-next-hop list entries
   - Tests: `TestExtractInterfaceOnlyRoute`, `TestExtractTableAndInterface`, `TestExtractMixedECMP`, `TestExtractSamePrefixDifferentTables`, `TestRejectNextHopNoAddressNoInterface`, `TestRejectBFDOnInterfaceOnly`, `TestRejectReservedTableID`
   - Files: `model.go`, `config.go`, `diff.go`
   - Verify: tests fail -> implement -> tests pass
4. **Phase: Backend** - Pass Table to netlink; check Address.IsValid() before setting Gw (single-path and multipath); update listRoutes with table info
   - Tests: `TestExistingGatewayRouteUnchanged`
   - Files: `backend_linux.go`
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Route manager** - Change routes map to nested `map[uint32]map[netip.Prefix]*routeState`; update applyRoutes, setBFD, showRoutes (add Table), shutdown; watchBFD closure captures table; sortedNextHops adds Interface tiebreaker
   - Tests: `TestSortedNextHopsInterfaceTiebreak`
   - Files: `inject.go`, `diff.go`
   - Verify: existing tests still pass after restructuring
6. **Phase: Redistribution** - Add Table to RouteChangeEntry; emitRouteChange skips non-zero table
   - Files: `redistevents/events.go`, `inject.go`
7. **Functional tests** - `test/static/table-routes.ci`
8. **Full verification** - `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 8 ACs have implementation with file:line |
| Correctness | Table 0 means unspecified (kernel resolves to RT_TABLE_MAIN 254); interface-only uses ifindex not gateway; mixed ECMP produces valid multipath |
| YANG structure | `list table` (keyed by `name`) containing `list route`; `interface-next-hop` is a sibling list inside `forward` case, NOT a separate choice case; both `next-hop` and `interface-next-hop` can coexist |
| Naming | YANG uses `list table` keyed by `name` (`default` or numeric), `list interface-next-hop` keyed by `interface` |
| Table range | `default`=0 (main); numeric range `1..252 \| 256..4294967295`; reserved 253-255 rejected at parse time |
| Data flow | Table flows from YANG table name -> config parse -> model -> netlink; RouteChangeEntry.Table always 0 (non-main routes skip emission) |
| Model unity | No new action types; no InterfaceNextHop struct; nextHop.Address zero for interface-only |
| Map structure | routes is nested map[uint32]map[Prefix]*routeState; same prefix in different tables coexists |
| Dedup key | parseStaticConfig uses Prefix per table (natural scoping from `list table`) |
| Diff stability | sortedNextHops uses Interface as tiebreaker; routesEqual includes Table |
| BFD guard | Interface-only next-hop with bfd-profile rejected at parse time |
| watchBFD | Closure captures table ID for nested map lookup |
| showRoutes | showRoute includes Table field; sort uses (Prefix, Table) for stable ordering |
| YANG leaf spec | `list interface-next-hop` has leaves: `interface` (key), `weight` (uint16, default 1) |
| Existing tests | config_test.go dedup test updated; inject_test.go compiles with nested map |
| Redistribution | emitRouteChange skips non-zero table; no PBR route leaking |
| Rule: no-layering | No duplication with PBR table handling |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Routing-table registry plugin | ls internal/plugins/routingtable/ |
| Routing-table YANG schema | grep "routing-table" internal/plugins/routingtable/schema/*.yang |
| Table list in YANG | grep "list table" ze-static-conf.yang |
| Interface-next-hop list in YANG (inside forward case) | grep "interface-next-hop" ze-static-conf.yang |
| Weight leaf in interface-next-hop | grep -A5 "interface-next-hop" ze-static-conf.yang \| grep "weight" |
| Table in Route model | grep "Table" model.go |
| Table in RouteChangeEntry | grep "Table" redistevents/events.go |
| Table in installedStaticRoute | grep "table" backend.go |
| Table in showRoute | grep "Table" inject.go |
| Nested route map | grep "map\[uint32\]" inject.go |
| Table name resolution | grep -n "Resolve\|registry\|routing.table" config.go |
| Interface tiebreaker in sort | grep "Interface" diff.go |
| BFD rejection on interface-only | grep "bfd\|BFD" config.go |
| Existing tests compile | go test ./internal/plugins/static/... |
| Functional test exists | ls test/static/table-routes.ci |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Table name: `default` or numeric `1..252 \| 256..4294967295`; reserved 253-255 rejected; interface name must exist |
| Privilege | Route table manipulation requires CAP_NET_ADMIN |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Netlink error on table route | Check kernel supports table ID; verify rtnetlink Table field |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| YANG compound key `key "prefix table"` | Ze config parser (`parseList`) only reads first word as key; secondary key leaves are inside the block. Compound YANG keys (e.g. BFD `key "peer vrf interface"`) do not affect duplicate detection in `addParsedListEntry`. | `list table` container: each table has own `list route` with independent prefix uniqueness |
| `leaf table` on route | Parser rejects duplicate prefix keys (`addParsedListEntry`); same prefix in different tables (AC-8) impossible without parser enhancement; `table` leaf also creates table-0-vs-254 semantic collision | `list table` container: routes grouped under named table, resolved via routing-table registry |
| Numeric table IDs in static config | Opaque numbers in config; not forward-compatible with VRF (which names routing domains) | Named tables via routing-table registry; VRF later absorbs the registry |
| Table 0-4294967295 unrestricted range | Table 0 (unspecified) and 254 (RT_TABLE_MAIN) both resolve to main table in kernel, creating semantic collision in dedup key and redistribution logic | `default` built-in = table 0; named tables via registry with ID range `1..252 \| 256..4294967295`; reserved 253-255 excluded |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

### Decision Log

| # | Decision | Resolved | Rationale |
|---|----------|----------|-----------|
| 1 | Route map key with table | Nested `map[uint32]map[Prefix]*routeState` | Enables per-table operations (lookup all routes in a table); mirrors kernel model |
| 2 | Action type for interface-only | Reuse `actionForward`, zero Address | Kernel is unified (Gw + LinkIndex independent fields); avoids FRR 6-variant enum pattern; no perf difference at LNS scale |
| 3 | Model shape | Single `nextHop`, `Address.IsValid() \|\| Interface != ""` | One struct, one parser, YANG lists normalize to same Go type |
| 4 | ECMP mixing | Allow gateway + interface-only in same group | Kernel handles it deterministically (rtnetlink(7)); rejecting valid config is wrong for a tool; allowing is cheaper (no validation pass) |
| 5 | RouteChangeEntry table field | Add `Table uint32` to entry, not batch | Value type, zero = main, no pool impact, no batch-splitting logic |
| 6 | YANG table structure | `list table` (keyed by `name`) containing `list route`; names resolved via `routing-table` registry | All routes under a named table container; each table has independent `list route` with own `key "prefix"` uniqueness, so AC-8 works naturally. `default` built-in (table 0, kernel RT_TABLE_MAIN 254). Named tables resolved via separate `routing-table` registry plugin. Forward-compatible: VRF spec absorbs registry into `vrf <name> { table N }`. |
| 7 | Routing-table registry | Separate plugin at `internal/plugins/routingtable/` | Single source of name-to-table-ID mapping shared by static routes and policy routing. Separate plugin because it's cross-cutting (not static-route-specific). VRF later absorbs the registry. |
| 8 | Backend wiring | Pass Table to netlink Route struct | Per spec |
| 9 | YANG interface-next-hop placement | Sibling `list` inside `forward` case, NOT separate choice case | YANG `choice` is mutually exclusive; a separate case would make mixed ECMP (AC-7) impossible at schema level |
| 10 | BFD on interface-only | Disallowed (rejected at parse time) | BFD requires a peer address; zero Address has no target to probe |
| 11 | Redistribution scope | Only emit for table 0 (main) | PBR table routes are policy-routed, not redistributed into BGP; avoids unexpected route leaking |
| 12 | Parser dedup key | `Prefix` per table | Each `list table` entry has its own `list route`; prefix uniqueness is per-table naturally. No parser changes needed |
| 13 | sortedNextHops tiebreaker | Compare Interface when Address compares equal | Multiple interface-only NHs have zero Address; without tiebreaker, diff produces wrong equality results |

### Research: other implementations

- **FRR staticd**: 6-variant `enum static_nh_type` (IFNAME, IPV4_GATEWAY, IPV4_GATEWAY_IFNAME, IPV6_GATEWAY, IPV6_GATEWAY_IFNAME, BLACKHOLE). C limitation: no sum types, AF-specific address unions. `table_id` on `static_path`, not route.
- **GoBGP (zebra)**: Same 6-type enum mirroring kernel. No static route plugin; zebra bridge only.
- **bio-routing**: `StaticPath` has only `NextHop *bnet.IP`. No interface-only support. `FIBPath.Table int` as flat field.
- **freeRtr**: Java. Routes interface-attached with optional gateway. Per-VRF routing tables.

Ze's `netip.Addr` is family-agnostic, eliminating the IPv4/IPv6 split that forces FRR to 6 variants. Unified `nextHop` with zero Address for interface-only is the natural Go model. The YANG schema uses a sibling `list interface-next-hop` inside the `forward` case (not a separate choice case) so that gateway and interface-only next-hops can coexist for mixed ECMP; the parser merges both lists into `[]nextHop`.

### Scope limitations

- **Per-next-hop distance (VyOS admin distance):** VyOS supports per-next-hop `distance` for primary/backup selection (lower distance = preferred, equal distance = ECMP). Ze models only per-route `metric`. A config like `{ next-hop 10.0.0.1 { distance 10 } interface pppoe0 { distance 230 } }` cannot express primary/backup semantics in Ze; both next-hops would form an ECMP group. This is acceptable for the stated use cases (single interface-only default route, PBR table routes) but worth noting for future VyOS parity work.
- **showRoutes prefix ordering:** The existing `showRoutes` sort uses string comparison (`a.Prefix < b.Prefix`), which gives incorrect IP ordering (e.g., "10.x" < "9.x"). This spec adds Table as a secondary sort key but does not fix the pre-existing primary comparison. A separate fix is recommended.
- **VPP `toFibPath` and zero Address:** `vpp/backend.go` `toFibPath` calls `p.NextHop.Is4()` on the address. A zero `netip.Addr` returns false for `Is4()`, falling through to the IPv6 branch and writing 16 zero bytes as the next-hop. This is benign now (VPP backend does not handle interface-only routes), but if VPP per-route table support is added later, `toFibPath` must check `NextHop.IsValid()` before setting the address.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/`
