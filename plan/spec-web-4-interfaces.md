# Spec: Web 4 -- Interfaces and IP Pages

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-web-3-foundation |
| Phase | 7/7 |
| Updated | 2026-04-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/design/web-interface-design.md` - page layouts and behavior
4. `internal/component/iface/iface.go` - shared types (InterfaceInfo, InterfaceStats, KernelRoute, AddrInfo)
5. `internal/component/iface/schema/ze-iface-conf.yang` - interface YANG schema
6. `internal/component/web/handler_workbench.go` - workbench handler pattern
7. `internal/component/web/workbench_sections.go` - left nav sections
8. `internal/component/web/workbench_enrich.go` - table enrichment

## Task

Build the **Interfaces** and **IP** pages for the Ze operator workbench. These are
purpose-built pages (not YANG tree browsers) that present interface configuration,
status, and traffic as tables with inline actions. IP pages cover addresses, routes,
and DNS configuration. All pages use the reusable table, detail panel, and form
components from spec-web-3-foundation.

The pages cover seven distinct views:
1. Interfaces > All Interfaces (table with detail panel)
2. Interfaces > Filtered Views (ethernet, bridge, VLAN, tunnel)
3. Interfaces > Traffic (real-time counters)
4. IP > Addresses (cross-interface address table)
5. IP > Routes (routing table + static route management)
6. IP > DNS (singleton form for resolver config)

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `plan/design/web-interface-design.md` - Page layouts, column definitions, empty states, actions
  -> Decision: Tables are the primary view pattern; every table row has actions.
  -> Decision: Monitor and configure in the same place (config + counters together).
  -> Constraint: Every empty table shows column headers + specific message + Add button.
- [ ] `docs/architecture/web-interface.md` - Web component architecture
  -> Constraint: Server-rendered Go templates + HTMX partials. No client-side framework.
- [ ] `docs/architecture/web-components.md` - Fragment and OOB patterns
  -> Constraint: HTMX partial requests return OOB response fragments.
- [ ] `docs/features/interfaces.md` - Interface feature documentation
  -> Decision: JunOS-style two-layer model: physical interface + logical units.
  -> Constraint: Interface types: ethernet, dummy, veth, bridge, tunnel (8 encap kinds), wireguard.

### RFC Summaries (MUST for protocol work)
- Not applicable (no protocol wire work).

**Key insights:**
- InterfaceInfo has Name, Index, Type, State, MTU, MAC, Addresses, Stats, ParentIndex, VlanID.
- InterfaceStats has RxBytes/Packets/Errors/Dropped and TxBytes/Packets/Errors/Dropped (uint64).
- KernelRoute has Destination, NextHop, Device, Protocol, Metric, Family, Source.
- AddrInfo has Address, PrefixLength, Family.
- Interface types from YANG: ethernet, dummy, veth, bridge, tunnel, wireguard, loopback.
- Tunnel encapsulation kinds: gre, gretap, ip6gre, ip6gretap, ipip, sit, ip6tnl, ipip6.
- DNS resolver config in `internal/component/resolve/dns/resolver.go`: Server, CacheSize, CacheTTL, Timeout.
- Backend API: ListInterfaces(), GetInterface(name), GetStats(name), ListKernelRoutes(filter, limit), ResetCounters(name).
- Unit fields: ID, VLANID, Addresses, VRF, Description, Disable, SysctlProfiles.
- Show commands: `show interface`, `show interface <name>`, `clear interface [<name>] counters`.
- Routes CLI: `ze interface routes [cidr] [--limit N]`.
- RPCs available: create-dummy, create-veth, create-bridge, delete, addr-add, addr-del, unit-add, unit-del, up, down, mtu, mac, migrate.
- WorkbenchSections already has "interfaces" keyed to `/show/iface/`.
- The web component uses Renderer.RenderFragment, HTMX OOB swaps, SSE via sse.go.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/component/iface/iface.go` - Shared types: InterfaceInfo, InterfaceStats, AddrInfo, KernelRoute
  -> Constraint: All JSON field names use kebab-case.
- [ ] `internal/component/iface/backend.go` - Backend interface with all operations
  -> Constraint: Backend is a pluggable interface; dispatch.go wraps with baseline subtraction.
- [ ] `internal/component/iface/dispatch.go` - Package-level functions delegating to backend
  -> Constraint: ListInterfaces/GetInterface/GetStats apply baseline subtraction before returning.
- [ ] `internal/component/iface/counters.go` - Counter baseline store for clear-counters
  -> Constraint: ResetCounters uses baseline-delta when backend cannot physically reset.
- [ ] `internal/component/iface/config.go` - Config parsing: ifaceConfig, ifaceEntry, unitEntry
  -> Constraint: Config model has Ethernet, Dummy, Veth, Bridge, Tunnel, Wireguard, Loopback.
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` - Full interface YANG schema
  -> Constraint: MTU range 68..16000. Unit ID range 0..16385. VLAN ID range 1..4094.
  -> Constraint: interface-l2 grouping includes mac-address; interface-common does not.
  -> Constraint: Tunnel and wireguard use interface-common (no list-level mac-address).
- [ ] `internal/component/iface/schema/ze-iface-api.yang` - Interface RPC definitions
  -> Constraint: RPCs use typed input leaves (name, address, vlan-id).
- [ ] `internal/component/iface/schema/ze-iface-cmd.yang` - Interface command tree
  -> Constraint: Commands: create-dummy, create-veth, create-bridge, delete, up, down, mtu, mac, addr-add, addr-del, unit-add, unit-del, migrate.
- [ ] `internal/component/iface/cmd/cmd.go` - RPC handlers for interface lifecycle
  -> Constraint: RPCs registered via pluginserver.RegisterRPCs in init().
- [ ] `internal/component/iface/cmd/clear.go` - Clear counters RPC handler
  -> Constraint: Wire method `ze-clear:interface-counters`. Handles single and all-interface clears.
- [ ] `internal/component/web/handler_workbench.go` - Workbench handler pattern
  -> Constraint: HandleWorkbench builds FragmentData, enriches, renders workbench template.
- [ ] `internal/component/web/workbench_sections.go` - Left navigation taxonomy
  -> Constraint: WorkbenchSections returns ordered sections. "interfaces" already mapped to `/show/iface/`.
- [ ] `internal/component/web/workbench_enrich.go` - Table enrichment (row tools, pending changes)
  -> Constraint: enrichWorkbenchTable attaches row tools and pending-change markers.
- [ ] `internal/component/web/render.go` - Template rendering infrastructure
  -> Constraint: Renderer has layout, workbench, fragments, l2tp template sets. Fragment templates in templates/component/.
- [ ] `internal/component/web/fragment.go` - FieldMeta type for form fields
  -> Constraint: FieldMeta has Leaf, Path, Type, Value, Default, Description, Options, Min, Max, Pattern.
- [ ] `internal/component/resolve/dns/resolver.go` - DNS resolver with config
  -> Constraint: ResolverConfig has Server, ResolvConfPath, Timeout, CacheSize, CacheTTL.
- [ ] `cmd/ze/iface/show.go` - Offline `ze interface show` command
  -> Constraint: Tabwriter output: NAME, INDEX, TYPE, STATE, MTU, MAC, ADDRESSES.
- [ ] `cmd/ze/iface/routes.go` - Offline `ze interface routes` command
  -> Constraint: ListKernelRoutes returns KernelRoute with Destination, NextHop, Device, Protocol, Metric, Family.

**Behavior to preserve:**
- All existing web handlers, templates, and the Finder mode continue unchanged.
- WorkbenchSections ordering and selection logic.
- Backend dispatch wraps with baseline subtraction for counters.
- Existing RPC wire methods and their argument formats.
- SSE event stream functionality.
- The workbench handler's dual-mode: HTMX partial vs full-page render.

**Behavior to change:**
- WorkbenchSections needs sub-entries for Interfaces (All, Ethernet, Bridge, VLAN, Tunnel, Traffic) and IP (Addresses, Routes, DNS).
- New purpose-built page handlers for each view.
- New templates for interface table, interface detail, traffic, IP addresses, IP routes, DNS form.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- HTTP request to `/workbench/interfaces/`, `/workbench/interfaces/ethernet/`, `/workbench/ip/addresses/`, etc.
- HTMX partial requests from navigation clicks and auto-refresh polling.

### Transformation Path
1. HTTP router dispatches to the page-specific handler (e.g., `HandleInterfacesPage`).
2. Handler calls `iface.ListInterfaces()` (or `iface.ListKernelRoutes()`, etc.) to get operational data.
3. Handler reads config tree for YANG-modeled configuration (units, addresses, VRF).
4. Handler merges operational and config data into a page-specific view model (e.g., `InterfaceTableData`).
5. Handler passes view model to `Renderer.RenderFragment()` or `Renderer.RenderWorkbench()`.
6. Template produces HTML; for HTMX partials, response uses OOB swap targeting the content area.
7. Auto-refresh: SSE events or HTMX polling trigger content area updates for counters.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Web handler -> iface backend | `iface.ListInterfaces()`, `iface.GetStats()` via dispatch.go | [ ] |
| Web handler -> config tree | `config.Tree.Walk()` to read unit/address config | [ ] |
| Web handler -> RPC | Form actions POST to handlers that call `pluginserver.DispatchRPC()` | [ ] |
| Browser -> Server | HTMX `hx-get`/`hx-post` for navigation, actions, auto-refresh | [ ] |
| SSE -> Browser | Server-sent events for traffic counter auto-refresh | [ ] |

### Integration Points
- `iface.ListInterfaces()` - Returns []InterfaceInfo with stats for the interface table.
- `iface.GetInterface(name)` - Returns single InterfaceInfo for detail panel.
- `iface.GetStats(name)` - Returns InterfaceStats for counter refresh.
- `iface.ListKernelRoutes(filter, limit)` - Returns []KernelRoute for routes page.
- `iface.ResetCounters(name)` - Clears counters (baseline-delta fallback on Linux).
- `config.Tree` / `config.Schema` - YANG-modeled interface config data.
- `pluginserver.DispatchRPC()` - Executes interface RPCs (create, delete, addr-add, etc.).
- `WorkbenchSections()` - Extended with sub-navigation entries.
- `Renderer` - Extended with new fragment templates.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `GET /workbench/interfaces/` | -> | `HandleInterfacesPage` renders interface table | `TestInterfacesPageRendersTable` |
| `GET /workbench/interfaces/?type=ethernet` | -> | `HandleInterfacesPage` filters by type | `TestInterfacesPageFiltersByType` |
| `GET /workbench/interfaces/detail/{name}` (HTMX) | -> | `HandleInterfaceDetail` renders detail panel | `TestInterfaceDetailPanelLoads` |
| `GET /workbench/interfaces/traffic/` | -> | `HandleTrafficPage` renders traffic table | `TestTrafficPageRendersCounters` |
| `GET /workbench/ip/addresses/` | -> | `HandleAddressesPage` renders address table | `TestAddressesPageRendersTable` |
| `GET /workbench/ip/routes/` | -> | `HandleRoutesPage` renders route table | `TestRoutesPageRendersTable` |
| `GET /workbench/ip/dns/` | -> | `HandleDNSPage` renders DNS form | `TestDNSPageRendersForm` |
| `POST /workbench/interfaces/create` | -> | `HandleInterfaceCreate` dispatches RPC | `TestInterfaceCreateDispatchesRPC` |
| `POST /workbench/interfaces/{name}/clear-counters` | -> | `HandleClearCounters` calls ResetCounters | `TestClearCountersCallsReset` |
| `POST /workbench/interfaces/{name}/unit/add` | -> | `HandleUnitAdd` dispatches unit-add RPC | `TestUnitAddDispatchesRPC` |
| `POST /workbench/ip/addresses/add` | -> | `HandleAddressAdd` dispatches addr-add RPC | `TestAddressAddDispatchesRPC` |
| `POST /workbench/ip/routes/add` | -> | `HandleRouteAdd` dispatches route-add config | `TestRouteAddCreatesEntry` |
| `POST /workbench/ip/dns/save` | -> | `HandleDNSSave` saves resolver config | `TestDNSSavePersistsConfig` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Navigate to Interfaces > All Interfaces | Table renders all interfaces with columns: Flags, Name, Type, OS Name, MTU, MAC, Link State, TX Rate, RX Rate, Description. |
| AC-2 | Click "Add Interface" in toolbar | Dropdown offers interface types available in Ze: ethernet, bridge, dummy, veth, tunnel (with subtypes: GRE, GRETAP, IPIP, SIT, IP6GRE, IP6GRETAP, IP6TNL, IPIP6), wireguard. |
| AC-3 | Click an interface row in the table | Detail panel opens showing: Configuration section (editable: name, type, description, MTU, MAC, admin state), Units sub-table (Unit ID, VLAN ID, Addresses, VRF, Description, State), Status section (read-only: link state, actual MTU, running flag), Traffic Counters section (read-only, auto-refresh: RX/TX bytes, packets, errors, drops). |
| AC-4 | Click "Add Unit" button in detail panel units sub-table | Form appears to create a new unit with fields for unit ID, VLAN ID; successful submission adds unit to the interface config. |
| AC-5 | View detail panel traffic counters | Counters auto-refresh via SSE or HTMX polling every 2-5 seconds showing current RX/TX bytes, packets, errors, drops. |
| AC-6 | Click "Clear Counters" action in detail panel | Confirmation dialog appears; on confirm, calls ResetCounters for that interface; counters display resets to zero. |
| AC-7 | Navigate to Interfaces > Ethernet (or Bridge, VLAN, Tunnel) | Same table as All Interfaces but pre-filtered to show only matching type. |
| AC-8 | Navigate to Interfaces > Traffic | Table shows all interfaces sorted by traffic rate (descending) with columns: Interface, TX bps, RX bps, TX pps, RX pps, TX Errors, RX Errors, TX Drops, RX Drops. Auto-refreshes every 2-5 seconds. |
| AC-9 | Navigate to IP > Addresses | Table shows all IP addresses from all interfaces with columns: Flags, Address (CIDR), Network, Interface.unit, VRF, Protocol (v4/v6), Description. |
| AC-10 | Click "Add Address" on IP > Addresses page | Form appears with fields: address (CIDR input), interface selector dropdown (listing all interfaces), VRF (optional). |
| AC-11 | Navigate to IP > Routes | Table shows routing table entries with columns: Flags (A/S/B/C/D), Destination, Gateway, Distance, Metric, Protocol, Interface, Table. Flag indicators visible per row. |
| AC-12 | Click "Add Static Route" on IP > Routes page | Form appears with fields: destination, gateway, distance, metric, table; submission creates a static route config entry. |
| AC-13 | Navigate to IP > DNS | Singleton form renders with fields: Upstream Servers (list with add/remove), Cache enabled (toggle), Cache size (number). Save persists the resolver config. |
| AC-14 | Navigate to any page with no data | Empty state message specific to that page displays with column headers visible and a prominent Add button. Messages match design doc. |
| AC-15 | All pages | Every page uses the table, detail panel, and form components from spec-web-3-foundation. No custom layout that bypasses the shared components. |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInterfaceTableData_Build` | `internal/component/web/page_interfaces_test.go` | Builds InterfaceTableData from []InterfaceInfo and config tree | |
| `TestInterfaceTableData_Flags` | `internal/component/web/page_interfaces_test.go` | R/X/D flags computed correctly from state, disable, managed | |
| `TestInterfaceTableData_FilterByType` | `internal/component/web/page_interfaces_test.go` | Filters entries by type parameter | |
| `TestInterfaceDetailData_Build` | `internal/component/web/page_interfaces_test.go` | Builds detail data merging config and operational info | |
| `TestInterfaceDetailData_Units` | `internal/component/web/page_interfaces_test.go` | Units sub-table populated from config tree | |
| `TestTrafficTableData_Build` | `internal/component/web/page_traffic_test.go` | Builds traffic rows from InterfaceInfo with stats | |
| `TestTrafficTableData_SortByRate` | `internal/component/web/page_traffic_test.go` | Rows sorted by total rate descending | |
| `TestAddressTableData_Build` | `internal/component/web/page_ip_addresses_test.go` | Collects addresses across all interfaces into flat table | |
| `TestAddressTableData_Protocol` | `internal/component/web/page_ip_addresses_test.go` | IPv4 and IPv6 addresses tagged correctly | |
| `TestAddressTableData_FilterByInterface` | `internal/component/web/page_ip_addresses_test.go` | Filters by interface name | |
| `TestAddressTableData_FilterByProtocol` | `internal/component/web/page_ip_addresses_test.go` | Filters by v4 or v6 | |
| `TestRouteTableData_Build` | `internal/component/web/page_ip_routes_test.go` | Builds route rows from []KernelRoute | |
| `TestRouteTableData_Flags` | `internal/component/web/page_ip_routes_test.go` | Route flags (A/S/B/C/D) set from protocol field | |
| `TestRouteTableData_FilterByProtocol` | `internal/component/web/page_ip_routes_test.go` | Filters by protocol (static, bgp, connected) | |
| `TestDNSFormData_Build` | `internal/component/web/page_ip_dns_test.go` | Builds DNS form data from config tree | |
| `TestDNSFormData_Defaults` | `internal/component/web/page_ip_dns_test.go` | Empty config produces default form values | |
| `TestWorkbenchSections_InterfaceSubNav` | `internal/component/web/workbench_sections_test.go` | Interfaces section has sub-entries: All, Ethernet, Bridge, VLAN, Tunnel, Traffic | |
| `TestWorkbenchSections_IPSubNav` | `internal/component/web/workbench_sections_test.go` | IP section has sub-entries: Addresses, Routes, DNS | |
| `TestInterfaceTypeDropdown` | `internal/component/web/page_interfaces_test.go` | Add Interface dropdown lists all Ze interface types | |
| `TestComputeTrafficRate` | `internal/component/web/page_traffic_test.go` | Rate calculation from two stat snapshots with time delta | |
| `TestNetworkFromCIDR` | `internal/component/web/page_ip_addresses_test.go` | Network address computed from CIDR (10.0.0.5/24 -> 10.0.0.0/24) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Interface MTU (edit form) | 68-16000 | 16000 | 67 | 16001 |
| Unit ID (add unit form) | 0-16385 | 16385 | N/A (0 is valid) | 16386 |
| VLAN ID (add unit form) | 1-4094 | 4094 | 0 | 4095 |
| DNS cache size (form) | 0-1000000 | 1000000 | N/A (0 disables) | 1000001 |
| Route limit (display cap) | 1-10000 | 10000 | 0 | N/A (capped server-side) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-web-iface-table` | `test/web/iface-table.ci` | Navigates to Interfaces page, verifies table columns and interface rows | |
| `test-web-iface-detail` | `test/web/iface-detail.ci` | Clicks interface row, verifies detail panel opens with config and counters | |
| `test-web-iface-filter` | `test/web/iface-filter.ci` | Navigates to Ethernet filtered view, verifies only ethernet interfaces shown | |
| `test-web-iface-traffic` | `test/web/iface-traffic.ci` | Opens Traffic page, verifies rate columns and auto-refresh behavior | |
| `test-web-ip-addresses` | `test/web/ip-addresses.ci` | Opens IP Addresses page, verifies addresses from multiple interfaces listed | |
| `test-web-ip-routes` | `test/web/ip-routes.ci` | Opens IP Routes page, verifies route entries with protocol flags | |
| `test-web-ip-dns` | `test/web/ip-dns.ci` | Opens DNS page, modifies cache size, saves, verifies persistence | |
| `test-web-iface-empty` | `test/web/iface-empty.ci` | Fresh config with no interfaces, verifies empty state messages | |
| `test-web-iface-add` | `test/web/iface-add.ci` | Uses Add Interface to create a dummy interface, verifies it appears in table | |
| `test-web-ip-addr-add` | `test/web/ip-addr-add.ci` | Uses Add Address to assign an address, verifies it appears in address table | |

### Future (if deferring any tests)
- Full SSE streaming test for traffic page (requires SSE test infrastructure from web component).
- Multi-VRF address filtering test (requires VRF test fixtures).

## Files to Modify
- `internal/component/web/workbench_sections.go` - Add sub-navigation entries for Interfaces and IP sections
- `internal/component/web/render.go` - Register new fragment templates for interface/IP pages
- `internal/component/web/handler_workbench.go` - Add route registration for new page endpoints
- `internal/component/web/handler.go` - Register new page routes in the HTTP mux
- `internal/component/web/assets/style.css` - CSS for interface table, detail panel, traffic page, form layouts
- `internal/component/web/workbench_enrich.go` - Extend enrichment for interface-specific row tools if needed

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No (uses existing RPCs) | N/A |
| CLI commands/flags | [ ] No (web pages use existing show/RPC commands) | N/A |
| Editor autocomplete | [ ] No | N/A |
| Functional test for new RPC/API | [ ] No (web functional tests cover UI) | `test/web/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] Yes | `docs/features.md` - Add Interfaces and IP web pages |
| 2 | Config syntax changed? | [ ] No | N/A |
| 3 | CLI command added/changed? | [ ] No | N/A |
| 4 | API/RPC added/changed? | [ ] No | N/A |
| 5 | Plugin added/changed? | [ ] No | N/A |
| 6 | Has a user guide page? | [x] Yes | `docs/guide/web-interface.md` - Document interface and IP pages |
| 7 | Wire format changed? | [ ] No | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] No | N/A |
| 9 | RFC behavior implemented? | [ ] No | N/A |
| 10 | Test infrastructure changed? | [ ] No | N/A |
| 11 | Affects daemon comparison? | [ ] No | N/A |
| 12 | Internal architecture changed? | [ ] No | N/A |

## Files to Create
- `internal/component/web/page_interfaces.go` - Interface table page handler and data model
- `internal/component/web/page_interfaces_test.go` - Unit tests for interface page data building
- `internal/component/web/page_traffic.go` - Traffic monitoring page handler and data model
- `internal/component/web/page_traffic_test.go` - Unit tests for traffic page data
- `internal/component/web/page_ip_addresses.go` - IP Addresses page handler and data model
- `internal/component/web/page_ip_addresses_test.go` - Unit tests for address page data
- `internal/component/web/page_ip_routes.go` - IP Routes page handler and data model
- `internal/component/web/page_ip_routes_test.go` - Unit tests for routes page data
- `internal/component/web/page_ip_dns.go` - IP DNS form page handler and data model
- `internal/component/web/page_ip_dns_test.go` - Unit tests for DNS form data
- `internal/component/web/templates/page/interfaces.html` - Interface table page template
- `internal/component/web/templates/page/interface_detail.html` - Interface detail panel template
- `internal/component/web/templates/page/traffic.html` - Traffic monitoring page template
- `internal/component/web/templates/page/ip_addresses.html` - IP Addresses page template
- `internal/component/web/templates/page/ip_routes.html` - IP Routes page template
- `internal/component/web/templates/page/ip_dns.html` - DNS form page template
- `test/web/iface-table.ci` - Functional test: interface table
- `test/web/iface-detail.ci` - Functional test: interface detail panel
- `test/web/iface-filter.ci` - Functional test: filtered interface views
- `test/web/iface-traffic.ci` - Functional test: traffic page
- `test/web/ip-addresses.ci` - Functional test: IP addresses page
- `test/web/ip-routes.ci` - Functional test: IP routes page
- `test/web/ip-dns.ci` - Functional test: DNS form page
- `test/web/iface-empty.ci` - Functional test: empty state rendering
- `test/web/iface-add.ci` - Functional test: Add Interface workflow
- `test/web/ip-addr-add.ci` - Functional test: Add Address workflow

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. /ze-review gate | Review Gate section -- run `/ze-review`; fix every BLOCKER/ISSUE; re-run until only NOTEs remain (BEFORE full verification) |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Interface Table Page** -- Handler, data model, template, CSS, navigation
   - Tests: `TestInterfaceTableData_Build`, `TestInterfaceTableData_Flags`, `TestInterfaceTableData_FilterByType`, `TestInterfaceTypeDropdown`, `TestWorkbenchSections_InterfaceSubNav`, `TestWorkbenchSections_IPSubNav`
   - Files: `page_interfaces.go`, `page_interfaces_test.go`, `templates/page/interfaces.html`, `workbench_sections.go`, `handler.go`, `style.css`
   - Verify: tests fail -> implement -> tests pass
   - Details:
     - Define `InterfaceRow` struct: Flags, Name, Type, OSName, MTU, MAC, LinkState, TXRate, RXRate, Description.
     - Define `InterfaceTableData` struct: Rows []InterfaceRow, FilterType string, EmptyMessage string.
     - Build data from `iface.ListInterfaces()` merged with config tree for description, OS name, disable flag.
     - Flags: R = state "up", X = disable set, D = zeManageable type.
     - Handler at `/workbench/interfaces/` accepts `?type=` query parameter for filtered views.
     - Extend `WorkbenchSections` with sub-navigation for Interfaces and IP.
     - Template uses table component from spec-web-3 with specific columns.
     - Add Interface dropdown: derive types from a registry constant, not hardcoded list.
     - Empty state: "No interfaces configured. Add Interface to begin."

2. **Phase: Interface Detail Panel** -- Config section, units sub-table, status, counters
   - Tests: `TestInterfaceDetailData_Build`, `TestInterfaceDetailData_Units`
   - Files: `page_interfaces.go`, `page_interfaces_test.go`, `templates/page/interface_detail.html`
   - Verify: tests fail -> implement -> tests pass
   - Details:
     - Define `InterfaceDetailData` struct: Config fields (Name, Type, Description, MTU, MAC, AdminState), Units []UnitRow, Status (LinkState, ActualMTU, Running), Counters (RX/TX stats).
     - Define `UnitRow` struct: ID, VLANID, Addresses []string, VRF, Description, State.
     - Handler at `/workbench/interfaces/detail/{name}` returns HTMX partial.
     - Config section: inline editable fields using form component from spec-web-3.
     - Units sub-table: read from config tree under `interface/<type>/<name>/unit/`.
     - Add Unit button: form with unit ID and VLAN ID fields.
     - Status section: read-only from `iface.GetInterface(name)`.
     - Traffic Counters: auto-refresh via HTMX `hx-trigger="every 3s"` targeting counter section.
     - Actions: Enable/Disable (calls up/down RPC), Clear Counters (with confirm).

3. **Phase: Filtered Views and Traffic Page** -- Type filters, real-time traffic monitoring
   - Tests: `TestTrafficTableData_Build`, `TestTrafficTableData_SortByRate`, `TestComputeTrafficRate`
   - Files: `page_traffic.go`, `page_traffic_test.go`, `templates/page/traffic.html`
   - Verify: tests fail -> implement -> tests pass
   - Details:
     - Filtered views: same handler as All Interfaces with `?type=ethernet`, `?type=bridge`, `?type=vlan`, `?type=tunnel`.
     - VLAN filter: match interfaces whose Type is "vlan" or that have units with VLANID > 0.
     - Tunnel filter: match tunnel + wireguard types.
     - Traffic page: define `TrafficRow` struct: Interface, TXbps, RXbps, TXpps, RXpps, TXErrors, RXErrors, TXDrops, RXDrops.
     - Rate computation: store previous snapshot in server-side per-session state, compute delta/time.
     - Sort by total rate (TXbps + RXbps) descending.
     - Auto-refresh via HTMX `hx-trigger="every 3s"` on table body.
     - Empty state: "No interfaces to monitor."

4. **Phase: IP Addresses Page** -- Cross-interface address table
   - Tests: `TestAddressTableData_Build`, `TestAddressTableData_Protocol`, `TestAddressTableData_FilterByInterface`, `TestAddressTableData_FilterByProtocol`, `TestNetworkFromCIDR`
   - Files: `page_ip_addresses.go`, `page_ip_addresses_test.go`, `templates/page/ip_addresses.html`
   - Verify: tests fail -> implement -> tests pass
   - Details:
     - Define `AddressRow` struct: Flags, Address (CIDR), Network, InterfaceUnit, VRF, Protocol, Description.
     - Collect from `iface.ListInterfaces()` -> iterate addresses -> build rows.
     - Also read config tree to get unit-level VRF and description.
     - Network address: compute from CIDR using `netip.Prefix.Masked()`.
     - Protocol: detect from address format (contains ":" -> IPv6, otherwise IPv4).
     - Filters: `?interface=`, `?protocol=v4|v6`, `?vrf=`.
     - Add Address form: address (CIDR text input), interface selector (dropdown from ListInterfaces), VRF (optional text).
     - Submit dispatches `ze-iface:interface-addr-add` RPC.
     - Row actions: Edit, Delete (addr-del RPC), Go to Interface (link to detail panel).
     - Empty state: "No IP addresses configured. Add an address to an interface to enable L3 connectivity."

5. **Phase: IP Routes Page** -- Routing table with static route management
   - Tests: `TestRouteTableData_Build`, `TestRouteTableData_Flags`, `TestRouteTableData_FilterByProtocol`
   - Files: `page_ip_routes.go`, `page_ip_routes_test.go`, `templates/page/ip_routes.html`
   - Verify: tests fail -> implement -> tests pass
   - Details:
     - Define `RouteRow` struct: Flags, Destination, Gateway, Distance, Metric, Protocol, Interface, Table.
     - Build from `iface.ListKernelRoutes("", routeDisplayLimit)`.
     - Flags: A = active (always for kernel routes), S = protocol "static", B = protocol "bgp", C = protocol "kernel" (connected), D = dynamic (non-static).
     - Distance: not in KernelRoute; show "-" (kernel FIB does not expose admin distance).
     - Table: not in KernelRoute; show "main" (Linux default, future: read rt_tables).
     - Filters: `?protocol=`, `?table=`, `?prefix=` (server-side filter via ListKernelRoutes).
     - Add Static Route form: destination (CIDR), gateway (IP), metric (number), interface selector.
     - Submit: create static route config entry in YANG tree (not a kernel route add -- goes through config commit).
     - Row actions for static routes only: Edit, Enable/Disable (toggle disable leaf), Delete.
     - Non-static rows: read-only (BGP, connected routes cannot be edited from this page).
     - Empty state: "No static routes. Connected routes appear automatically when interfaces have addresses."

6. **Phase: IP DNS Page** -- Singleton form for resolver config
   - Tests: `TestDNSFormData_Build`, `TestDNSFormData_Defaults`
   - Files: `page_ip_dns.go`, `page_ip_dns_test.go`, `templates/page/ip_dns.html`
   - Verify: tests fail -> implement -> tests pass
   - Details:
     - Define `DNSFormData` struct: Servers []string, CacheEnabled bool, CacheSize uint32.
     - Read from config tree (resolve/dns section if it exists in YANG; otherwise read ResolverConfig).
     - Upstream Servers: list field with add/remove buttons (uses list input pattern from spec-web-3).
     - Cache enabled: toggle (boolean field).
     - Cache size: number input with validation.
     - Save: POST to handler that writes config tree and commits.
     - Uses form component from spec-web-3.
     - Empty state: "No DNS resolvers configured. Add upstream DNS servers for name resolution."

7. **Phase: Integration and Full Verification** -- Wire everything together, functional tests
   - Tests: All functional tests from TDD Plan
   - Files: All `.ci` test files
   - Verify: `make ze-lint && make ze-unit-test && make ze-functional-test`
   - Details:
     - Verify all page routes registered in HTTP mux.
     - Verify all HTMX partials return valid HTML fragments.
     - Verify navigation highlighting works for all sub-pages.
     - Verify auto-refresh does not cause memory leaks (check SSE cleanup).
     - Run all functional tests.
     - Full `make ze-verify`.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-15 has implementation with file:line |
| Correctness | Interface types derived from registry, not hardcoded; flags (R/X/D, A/S/B/C/D) computed correctly from data |
| Naming | URL paths use kebab-case (`/workbench/interfaces/`, `/workbench/ip/addresses/`); Go types use CamelCase; template names use snake_case |
| Data flow | Pages call iface.ListInterfaces() through dispatch.go (never backend directly); RPCs dispatched through pluginserver |
| Rule: no-layering | No duplicate interface-listing code (reuse iface package functions) |
| Rule: derive-not-hardcode | Interface type list in Add dropdown derived from YANG schema or a typed constant, never a literal string list |
| Rule: buffer-first | Counter data returned as-is from InterfaceStats (no re-serialization) |
| Rule: exact-or-reject | Form validation rejects out-of-range values (MTU, VLAN ID, unit ID) before dispatching RPC |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Interface table page handler exists | `grep 'HandleInterfacesPage' internal/component/web/page_interfaces.go` |
| Interface detail handler exists | `grep 'HandleInterfaceDetail' internal/component/web/page_interfaces.go` |
| Traffic page handler exists | `grep 'HandleTrafficPage' internal/component/web/page_traffic.go` |
| Addresses page handler exists | `grep 'HandleAddressesPage' internal/component/web/page_ip_addresses.go` |
| Routes page handler exists | `grep 'HandleRoutesPage' internal/component/web/page_ip_routes.go` |
| DNS page handler exists | `grep 'HandleDNSPage' internal/component/web/page_ip_dns.go` |
| Interface table template exists | `ls internal/component/web/templates/page/interfaces.html` |
| Detail panel template exists | `ls internal/component/web/templates/page/interface_detail.html` |
| Traffic template exists | `ls internal/component/web/templates/page/traffic.html` |
| Addresses template exists | `ls internal/component/web/templates/page/ip_addresses.html` |
| Routes template exists | `ls internal/component/web/templates/page/ip_routes.html` |
| DNS template exists | `ls internal/component/web/templates/page/ip_dns.html` |
| WorkbenchSections has Interface sub-nav | `grep -c 'ethernet\|bridge\|vlan\|tunnel\|traffic' internal/component/web/workbench_sections.go` |
| WorkbenchSections has IP sub-nav | `grep -c 'addresses\|routes\|dns' internal/component/web/workbench_sections.go` |
| Routes registered in HTTP mux | `grep '/workbench/interfaces/' internal/component/web/handler.go` |
| Unit tests pass | `go test ./internal/component/web/ -run 'TestInterface\|TestTraffic\|TestAddress\|TestRoute\|TestDNS'` |
| Functional tests exist | `ls test/web/iface-*.ci test/web/ip-*.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Interface name validated via `iface.ValidateIfaceName()` before any RPC dispatch. CIDR addresses validated via `netip.ParsePrefix()`. MTU/VLAN/unit ID checked against YANG ranges. |
| Path traversal | URL path segments validated (no `..` or arbitrary path injection). `ValidatePathSegments()` applied. |
| XSS | All user-provided values (interface names, descriptions, addresses) HTML-escaped by Go template engine. No `template.HTML` wrapping of user input. |
| CSRF | POST handlers require HTMX request header or appropriate CSRF token. |
| Authorization | Page handlers check authentication via `GetUsernameFromRequest()`. Write operations require authenticated session. |
| Resource exhaustion | Route table display capped at `routeDisplayLimit` (default 1000). Traffic polling rate server-controlled (not client-adjustable). ListInterfaces does not return unbounded results. |
| Information leakage | Error messages do not expose internal file paths or stack traces. Interface stats are operational data, not sensitive. |
| Injection | RPC arguments validated before dispatch. No shell command construction from user input. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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
<!-- LIVE -- write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem -> arch doc, process -> rules, knowledge -> memory.md -->

- DNS resolver config lives in `internal/component/resolve/dns/`, not under a YANG schema. The web page will need to read/write ResolverConfig fields directly or via config tree if a YANG module is added.
- KernelRoute does not carry admin distance or routing table name. The Routes page shows "-" for distance and "main" for table. Future: extend KernelRoute if the kernel provides these.
- Interface types for the Add dropdown should be derived from the YANG schema's list names under `/interface` (ethernet, dummy, veth, bridge, tunnel, wireguard), not hardcoded. Tunnel subtypes come from the encapsulation choice cases.
- Traffic rate computation requires maintaining a previous snapshot per interface. Server-side session state (or a recent-stats cache keyed by username) holds the previous readings. First request has no delta and shows "-" for rates.

## RFC Documentation

Not applicable (no RFC protocol work in this spec).

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

<!-- BLOCKING (rules/planning.md Completion Checklist step 7): -->
<!-- Run /ze-review BEFORE the final testing/verify step. Record the findings here. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns only NOTEs (or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block -- record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->
<!-- Hook pre-commit-spec-audit.sh (exit 2) checks this section exists and is filled. -->

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
