# 686 -- web-2-operator-workbench

## Context

Ze's web UI was a Finder-style column browser (macOS Finder metaphor) that showed config nodes as nested columns. Operators managing BGP peers had to navigate deep config paths, run operational commands on a separate admin page, and mentally correlate config with runtime state across multiple page loads. The workbench spec replaced the Finder UI with a RouterOS/WebFig-inspired operator workbench: persistent left nav, table-first data presentation, detail panels, tool overlays, and a change-verify loop that keeps operators in one workspace.

## Decisions

- **UIMode switch with env var rollback** over feature flags or per-user cookie switching. `ze.web.ui=workbench` activates V2; `ze.web.ui=finder` is the emergency rollback. One mode per process, read at startup. This avoids dual-UI maintenance and forces a promotion decision.
- **YANG `ze:related` metadata for tool discovery** over hardcoded tool lists per page. Related tools are declared in YANG schema nodes, extracted at schema load, and resolved server-side. Browser requests submit a tool key and config path, never arbitrary command strings. Prevents command injection and keeps authz consistent with CLI/API commands.
- **Overlay rendering for tool output** over replacing the detail panel. Tool results open in a dismissible overlay that preserves the underlying table/detail workspace. Supports rerun without losing context.
- **Server-side command resolution with placeholder substitution** (`related_resolver.go`) over client-side command construction. The resolver expands `{peer}`, `{group}` etc. from the current config path, so the browser never assembles command strings.
- **Workbench sections as a flat Go registry** (`workbench_sections.go`) over YANG-driven nav. V1 keeps the section taxonomy in one file for simplicity; schema-driven `ze:nav-section` is deferred.
- **Table enrichment pipeline** (`workbench_enrich.go`) over per-page enrichment. A reusable pipeline adds operational state (status flags, counters, session state) to table rows built from config data.
- **Per-page Go files** (`page_bgp_peers.go`, `page_firewall.go`, etc.) over a generic YANG-to-table renderer. Purpose-built pages can join config and operational data, add domain-specific columns, and present meaningful empty states.

## Consequences

- The workbench is the default web UI. Finder is preserved as rollback but not actively developed.
- Every new web page follows the same pattern: page file builds `WorkbenchTableData`/`WorkbenchDetailData`/`WorkbenchFormData`, handler renders via `workbench_table.html`/`workbench_detail.html`/`workbench_form.html`.
- Related tools are discoverable from YANG metadata, so adding a tool to a config node requires only a YANG annotation, not handler changes.
- The BGP change-verify loop (edit peer, commit, run peer detail, check state) works without full-page navigation.

## Gotchas

- UIMode is process-wide, not per-request. Switching requires hub restart. This is intentional (rollback is emergency, not preference).
- Related tool resolution trusts YANG metadata implicitly. If a `ze:related` annotation names a command that does not exist in the command registry, the tool button renders but execution fails with a clear error, not a crash.
- ANSI escape sequences from CLI command output must be stripped before rendering in the HTML overlay (`stripANSI` in handler_tools.go).
- Tool output truncation at 64KB prevents browser memory issues from large command results (e.g., full routing table dumps).

## Files

- `internal/component/web/ui_mode.go` -- UIMode type, ParseUIMode, env/cookie reading
- `internal/component/web/handler_workbench.go` -- workbench shell handler, page dispatch
- `internal/component/web/handler_tools.go` -- related tool POST handler, overlay rendering
- `internal/component/web/related_resolver.go` -- placeholder substitution for tool commands
- `internal/component/web/workbench_sections.go` -- two-level nav section registry
- `internal/component/web/workbench_enrich.go` -- table row enrichment pipeline
- `internal/component/web/workbench_table.go` -- WorkbenchTableData model
- `internal/component/web/workbench_detail.go` -- WorkbenchDetailData model
- `internal/component/web/workbench_form.go` -- WorkbenchFormData model
- `internal/component/web/workbench_pages.go` -- page registry and dispatch
- `internal/component/web/workbench_dashboard.go` -- dashboard data builder
- `internal/component/web/templates/component/workbench_*.html` -- all workbench templates
- `internal/component/web/templates/component/tool_overlay.html` -- tool output overlay
- `internal/component/web/templates/page/workbench.html` -- workbench page shell
