# 691 -- web-7-system-services

## Context

The operator workbench had table and form components (spec-web-3-foundation) and domain-specific pages for interfaces, BGP, and firewall, but lacked pages for the System, Services, and L2TP navigation sections. Operators needed web-based forms for 7 service configurations (SSH, Web, Telemetry, TACACS, MCP, Looking Glass, API), system pages (identity, users, resources, host hardware, sysctl profiles), and L2TP session management (sessions, configuration, health). The existing `handler_l2tp.go` already provided session list, detail, samples, SSE streaming, and disconnect functionality that had to be preserved alongside the new workbench pages.

## Decisions

- **`getConfigValue` / `getConfigListItems` helpers** over per-page tree traversal. A common helper reads arbitrary slash-separated config paths from the Tree, so every service form reads its YANG leaves with one call each. Keeps service forms declarative (just field definitions and paths).
- **One page file per navigation section** (`page_system.go` for 5 system pages, `page_services.go` for 7 service forms, `page_l2tp.go` for 3 L2TP pages) over one file per page. The pages are small forms/tables with no complex logic; grouping by section keeps related code together.
- **Dispatch functions per section** (`renderSystemPageContent`, `renderL2TPPageContent`, `renderServicePageContent`) over a flat switch in the workbench handler. Each section's dispatch is self-contained with its own path stripping and default-page logic.
- **Preserve `handler_l2tp.go` unchanged** and add new `page_l2tp.go` for workbench table views. The existing handler serves JSON content negotiation, SSE streaming, and disconnect commands under `/l2tp/` paths. The workbench pages render at `/show/l2tp/` and use the same `l2tp.LookupService().Snapshot()` for data.
- **Resources page uses `runtime.ReadMemStats` directly** over dispatching show RPCs. The page runs in the web server process, so Go runtime stats are available locally. Show RPC dispatch (for uptime, date) is deferred with placeholder values.
- **Host hardware as placeholder sections** over show-command dispatch. The hardware inventory requires dispatching `show host/*` RPCs which needs the command dispatcher wired into the page handler. Placeholder sections mirror the `host.Inventory` struct shape so the layout is correct when real data lands.
- **Password type for sensitive fields** (shared secrets, tokens, TLS keys) over plain text. The `WorkbenchFormField.Type = "password"` renders masked input in the browser. Values are still read from the config tree, so existing secrets display as masked dots.

## Consequences

- All 16 pages across System, Services, and L2TP are rendered in the workbench. Every service has a complete form matching its YANG schema.
- The `getConfigValue` helper became a shared utility used by all service form builders, reducing boilerplate.
- L2TP has two rendering paths: the original `handler_l2tp.go` for JSON/SSE/disconnect at `/l2tp/`, and `page_l2tp.go` workbench tables at `/show/l2tp/`. Both use `l2tp.LookupService().Snapshot()`.
- Resources and Host Hardware pages have partial operational data (Go runtime stats present, show-RPC-sourced data deferred). These show placeholder text where RPC dispatch is needed.

## Gotchas

- `getConfigValue` walks containers, not lists. Paths like `telemetry/prometheus/basic-auth/password` traverse four container levels. If any container is missing, it returns empty string silently. This is correct for optional config but means a typo in the path produces no error, just empty values.
- Users page reads profiles via both `GetList` and `GetListOrdered` because YANG leaf-lists may be stored as either map or ordered-list depending on the config loader. The `GetListOrdered` result takes priority if non-empty.
- L2TP sessions page URLs point to `/l2tp/<session-id>` (the original handler's detail path), not `/show/l2tp/<session-id>`, because the existing handler has rich session detail, samples, and disconnect functionality that the workbench page does not duplicate.
- The `renderServicePageContent` dispatch uses top-level path segments (`ssh`, `web`, `telemetry`, etc.) directly, not sub-paths under a `/services/` prefix, because service YANG schemas live at different config tree locations (some under `environment/`, some under `system/authentication/`, some under `telemetry/`).

## Files

- `internal/component/web/page_system.go` -- System section: identity form, users table, resources property list, host hardware, sysctl profiles table, section dispatch
- `internal/component/web/page_services.go` -- Services section: SSH, Web, Telemetry, TACACS, MCP, Looking Glass, API forms, config path helpers, section dispatch
- `internal/component/web/page_l2tp.go` -- L2TP section: sessions table, config form, health table, section dispatch
