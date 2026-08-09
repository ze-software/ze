# Workbench Domain Pages

The workbench shell and its reusable components give the operator a table,
detail and form framework. The domain pages are the content: purpose-built Go
builders that join operational state with configuration. `web-interface.md`
covers the shell and the URL scheme, `web-components.md` covers the component
library.

<!-- source: internal/component/web/workbench_pages.go -- page dispatch -->
<!-- source: internal/component/web/page_workbench_generic.go -- generic system and service dispatch -->

## Why builders and not a generic YANG renderer

Every domain page merges data a generic renderer cannot reach: link state and
counters from the iface backend, applied firewall state from the kernel
backend, config from the YANG tree. A generic list renderer can only show one
of those sources.

Each section has a dispatch function that owns its own path stripping and its
default page, rather than one flat switch in the workbench handler.

## Interfaces and IP

<!-- source: internal/component/web/page_interfaces.go -- interface table, detail, counters, type filter -->
<!-- source: internal/component/web/page_ip_addresses.go -- cross-interface address table, NetworkFromCIDR -->
<!-- source: internal/component/web/page_ip_routes.go -- kernel route table -->
<!-- source: internal/component/web/page_ip_dns.go -- DNS form -->
<!-- source: internal/component/web/page_traffic.go -- traffic table -->

- One `BuildInterfaceTableData` serves every type view through a `?type=`
  query parameter and `matchesTypeFilter`, instead of one handler per type.
  The left nav still shows a link per type.
- A VLAN is matched by `VlanID > 0`, not by a type string. A VLAN
  subinterface reports its parent's type, so the type string is never "vlan".
  Matching on a property is more reliable than matching on a name.
- `matchesTypeFilter` lists every tunnel encapsulation explicitly (gre,
  gretap, ip6gre, ip6gretap, ipip, sit, ip6tnl, wireguard). A new tunnel kind
  in the iface package must be added here, or it disappears from the Tunnel
  view.
- Routes are capped at 1000 rows and the cap is passed to `ListKernelRoutes`,
  so the limit is applied server-side. A full-table box holds 900k routes and
  rendering them as HTML exhausts the browser.
- Counters refresh with HTMX polling (`hx-trigger="every 3s"`), not SSE. The
  refresh is a GET returning a small HTML fragment, and SSE would add
  connection management for no gain.
- DNS resolver config is one object, so it renders as a singleton form rather
  than a table.
- Interface statistics can be nil, for example a loopback on some platforms.
  The counter table renders "not available" instead of zeros.

## BGP

<!-- source: internal/component/web/page_bgp_peers.go -- collectPeers, table, detail, actions -->
<!-- source: internal/component/web/page_bgp_groups.go -- collectGroups -->
<!-- source: internal/component/web/page_bgp_families.go -- cross-peer family aggregation -->
<!-- source: internal/component/web/page_bgp_policy.go -- policy collection -->
<!-- source: internal/component/web/page_bgp_summary.go -- summary table -->

- `collectPeers` walks both `bgp/peer` and `bgp/group/*/peer` and merges them
  into one table with a Group column. A third peer location, for example
  dynamic peers, must be added to this function explicitly.
- Row actions post `tool_id` and `context_path` to `/tools/related/run`. The
  server resolves the command from the YANG `ze:related` annotation, so the
  browser never builds a command string.
- The policy page checks a hardcoded list of list names (`filter`,
  `community`, `prefix-list`, `as-path`, `route-map`). `config.Tree` exposes no
  list enumeration at a level. A new policy list type added by a plugin augment
  must be added here or it stays invisible.
- Operational columns (state, uptime, prefix counts, message counters) render
  a placeholder. `peerFlag` colors from config alone: green for configured,
  grey for disabled. When reactor state is wired in, that function changes to
  FSM state and most peers stop being green.
- `buildPeerActionsHTML` embeds `hx-vals` JSON inside single-quoted HTML
  attributes. `context_path` is escaped, and a peer name carrying a single
  quote would break the JSON. YANG validation excludes the quote, which is what
  makes this safe.

## Firewall

<!-- source: internal/component/web/page_firewall.go -- tables, chains, rules, sets, connections -->

- Five sub-pages sit behind a path dispatcher, not tabs on one page, which
  matches the left nav and lets a table row link into a filtered chain or rule
  view. Drill-down uses query parameters (`?table=X&chain=Y`), so the back
  button and bookmarks work.
- The pages read `firewall.LastApplied()`, an immutable atomic snapshot, and
  never re-parse the config tree. That state is the applied one, not the
  operator's draft: a pending edit appears after commit. Display names strip
  the kernel `ze_` prefix.
- Counters are joined at collect time from `GetBackend().GetCounters()`, keyed
  by term name, and show "-" when no backend is loaded.
- `matchSummary` and `actionSummary` are exhaustive type switches over 15 match
  types and 19 action types, with a `%T` fallback. A new match or action type
  then shows its type name instead of vanishing from the rule display.
- Empty states differ by context: "no rules in chain X" against "no rules
  configured", depending on whether a filter is active.

## System, services and L2TP

<!-- source: internal/component/web/page_system.go -- identity, users, resources, host, sysctl -->
<!-- source: internal/component/web/page_services.go -- seven service forms and config path helpers -->
<!-- source: internal/component/web/page_l2tp.go -- L2TP workbench tables -->

- `getConfigValue` and `getConfigListItems` read a slash-separated config path,
  so a service form is a list of field definitions and paths. The helper walks
  containers, not lists, and returns an empty string for a missing container.
  That is correct for optional config and it means a typo in a path produces
  empty values rather than an error.
- One file per navigation section, not one file per page. These pages are small
  forms and tables with no complex logic.
- Sensitive fields (shared secrets, tokens, TLS keys) use the password field
  type, so the browser masks them. The values still come from the config tree.
- The users page reads profiles through both `GetList` and `GetListOrdered`,
  because a YANG leaf-list may be stored as a map or as an ordered list
  depending on the loader. The ordered result wins when it is non-empty.
- The service dispatch keys on the top-level path segment (`ssh`, `telemetry`),
  not on a `/services/` prefix, because the service schemas sit at different
  places in the config tree.
- The resources page reads `runtime.ReadMemStats` directly, because the page
  runs in the web server process.

## L2TP management UI

<!-- source: internal/component/web/handler_l2tp.go -- session list, detail, CQM samples, SSE, disconnect -->

The workbench L2TP tables at `/show/l2tp/` do not replace `handler_l2tp.go` at
`/l2tp/`, which serves JSON content negotiation, SSE streaming and the
disconnect action. Both read `l2tp.LookupService().Snapshot()`, and the
workbench session rows link to the original detail path because that page holds
the richer view.

- The CQM chart feed is a per-connection `time.Ticker` at the bucket interval,
  not an EventBroker broadcast. A CQM feed is per login, so broadcasting it
  would leak one subscriber's data to another and waste bandwidth.
- Disconnect runs `clear l2tp session teardown <sid> reason <text> cause
  <code>` through the existing command dispatcher. Authorization is enforced
  once at the CLI layer, and the `clear` prefix is already denied to the
  read-only profile, so the web path needed no authorization change.
- The destructive L2TP commands were moved from a top-level `l2tp` noun to an
  augment of the `clear` verb. ze CLI grammar puts the verb first and the noun
  second.
- The arguments are keyword-prefixed (`reason <text...>`, `cause <code>`), not
  positional, so a later argument does not break an existing caller.
- Chart colors are CSS custom properties read through `getComputedStyle`, so a
  theme change reaches the charts with no JavaScript change.

## Tools, logs and dashboard

<!-- source: internal/component/web/page_tools.go -- ping, BGP decode, metrics, capture forms -->
<!-- source: internal/component/web/page_logs.go -- live log, warnings, errors -->
<!-- source: internal/component/web/page_dashboard.go -- health and events sub-pages -->

- One handler per tool, not a generic tool page with a parameter. Each tool has
  its own fields, validation and command construction, so the shared code would
  be an abstraction over one case.
- GET renders the form and POST validates and dispatches, at one URL per tool,
  so HTMX can replace the result area alone.
- Every data read goes through the command dispatcher
  (`func(command, username, remoteAddr string) (string, error)`), so the web
  surface has the same authorization as the CLI and the API. No handler calls
  an RPC directly.
- Input is bounded before dispatch: ping count 1 to 100, timeout 1 to 30
  seconds, capture count 1 to 10000, hex-only for BGP decode, and the
  Prometheus name pattern for a metric query. Invalid input never reaches the
  dispatcher.
- All output passes through `normalizeOutput` (ANSI stripping and a 4 MiB cap)
  and then HTML escaping.
- The live log SSE handler delegates to `broker.ServeHTTP`, inheriting the
  connection limit of 100 and the per-client buffer of 16 events.
- Warnings, errors and events are parsed as JSON with a plain-text fallback per
  line. A change to the RPC response format degrades to line rendering instead
  of breaking the page.
- Dashboard health is derived from config presence, showing "Configured" or
  "Not configured". It is not runtime health. `knownComponents` is a hardcoded
  slice: a new component in that table means editing it.
