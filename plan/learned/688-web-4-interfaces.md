# 688 -- web-4-interfaces

## Context

The workbench shell and foundation components (spec-web-2, spec-web-3) gave Ze a table/detail/form UI framework but no domain pages. This spec built the first domain pages: Interfaces (all, filtered by type, traffic monitor) and IP (addresses, routes, DNS). These are purpose-built pages, not YANG tree browsers, that merge operational data from the iface backend with config data from the YANG tree.

## Decisions

- **Per-page Go builder functions** (`BuildInterfaceTableData`, `BuildAddressTableData`, `BuildRouteTableData`, `BuildTrafficTableData`, `BuildDNSWorkbenchForm`) over generic YANG-to-table rendering. Each page joins operational state (link state, counters, kernel routes) with config, which a generic renderer cannot express.
- **Type-based URL filtering** (`?type=ethernet`) over separate handler functions per type. One `BuildInterfaceTableData` function handles all views via `matchesTypeFilter`, keeping the page count down while the left nav shows dedicated links per type.
- **VLAN filter by VlanID > 0** over matching type string. VLAN sub-interfaces appear as their parent type with a VlanID, so the type string is not "vlan". This match-by-property approach is more reliable than string matching.
- **Tunnel filter includes wireguard** alongside GRE/IPIP/SIT variants. The "Tunnel" nav entry shows all tunnel-like encapsulations in one view.
- **HTMX polling for counter auto-refresh** (`hx-trigger="every 3s"`) over SSE. Counter refresh is a simple GET that returns a small HTML table fragment. SSE would be overkill for this use case and add connection management complexity.
- **Route display cap at 1000 rows** (`routeDisplayLimit`) over unbounded rendering. Full-DFZ boxes have 900K+ routes; rendering all as HTML would cause browser OOM. The cap is passed to `ListKernelRoutes` which handles the limit server-side.
- **DNS as a singleton form** (`WorkbenchFormData`) over a table. DNS resolver config is a single object, not a list, so the form component is the natural fit.
- **Addresses cross-interface table** collecting all addresses from all interfaces into one flat table. Each row links back to its parent interface detail. Dual filter by interface name and protocol family.
- **Traffic table sorted by total bytes descending** (RxBytes + TxBytes) so the busiest interfaces appear first. Uses `sort.Slice` at render time.
- **NetworkFromCIDR via `netip.ParsePrefix.Masked()`** for computing network addresses from host CIDRs. Standard library, no manual bit manipulation.

## Consequences

- Seven views (all interfaces, 4 type filters, traffic, addresses, routes, DNS) are reachable from the left nav under Interfaces and IP sections.
- Interface detail panel has three tabs (Configuration, Status, Traffic Counters) with auto-refreshing counters and a Clear Counters action.
- The `iface.ListInterfaces()` and `iface.ListKernelRoutes()` calls use the dispatch layer, which applies baseline subtraction for counters.
- Pages degrade gracefully: if the iface backend is not loaded, tables render empty with the standard empty-state message.

## Gotchas

- `matchesTypeFilter` for "tunnel" must list all tunnel encapsulation types explicitly (gre, gretap, ip6gre, ip6gretap, ipip, sit, ip6tnl). If new tunnel types are added to the iface package, this filter must be updated.
- Interface stats can be nil (e.g., loopback on some platforms). `formatCountersTable` handles nil stats with a "not available" fallback.
- `capitalizeFirst` is a single-word helper replacing `strings.Title` (deprecated). It only uppercases the first byte, so it does not handle multi-byte first runes correctly, though all type names are ASCII.
- The DNS form uses `htmxRequestTrue` (a package constant for the string "true") for the cache toggle value, coupling the form builder to the HTMX request detection constant.

## Files

- `internal/component/web/page_interfaces.go` -- interface table, detail, counters, type filter
- `internal/component/web/page_ip_addresses.go` -- cross-interface address table, NetworkFromCIDR
- `internal/component/web/page_ip_dns.go` -- DNS singleton form builder
- `internal/component/web/page_ip_routes.go` -- kernel route table with display limit
- `internal/component/web/page_traffic.go` -- traffic monitoring table, sorted by total bytes
- `internal/component/web/page_interfaces_test.go` -- interface table and detail tests
- `internal/component/web/page_ip_addresses_test.go` -- address table tests
- `internal/component/web/page_ip_dns_test.go` -- DNS form tests
- `internal/component/web/page_ip_routes_test.go` -- route table tests
