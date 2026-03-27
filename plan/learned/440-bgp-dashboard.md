# 440 -- bgp-dashboard

## Context

The `bgp monitor` command was a redirect to `event monitor` after the streaming monitor was renamed. Users had no live overview of peer status in the TUI. The goal was to repurpose `monitor bgp` (verb-first: `<action> <module>`) as a live peer dashboard that auto-refreshes every 2 seconds, showing a sortable color-coded peer table with update rates and drill-down detail.

## Decisions

- `monitor bgp` (verb-first) over `bgp monitor` (noun-first), matching the `show/set/del/update/monitor` CLI taxonomy established in learned/431.
- Polls via `commandExecutor("bgp summary")` over a streaming RPC, reusing existing SSH exec infrastructure. Acceptable latency for 2s refresh.
- `DashboardFactory` returns a poller function over reusing `MonitorSession` (channel-based). Polling is simpler for periodic refresh and avoids channel management.
- Direct lipgloss screen composition over Bubble Tea viewport widget, because the dashboard needs fixed header/footer with a scrollable middle section.
- Split into 3 source files (`model_dashboard.go`, `model_dashboard_render.go`, `model_dashboard_sort.go`) over a single file, keeping each under 420 lines.
- Removed the `bgp monitor` streaming handler redirect (no-layering) over keeping it alongside the dashboard detection.
- Detail view uses summary-level data over separate `peer <ip> detail` RPC call. Summary already contains essential fields; extended fields deferred.

## Consequences

- `monitor bgp` is now a verb-first command in the TUI, establishing the pattern for future `monitor` commands (e.g., `monitor rib`, `monitor metrics`).
- The verb taxonomy (`<action> <module>`) is documented in `docs/architecture/api/commands.md` as a formal convention.
- `event monitor` still uses noun-first syntax (legacy). A future rename to `monitor event` would be consistent but is not blocking.
- Detail view can be enhanced later to call `peer <ip> detail` for timer, connect/accept, local-ip fields.
- `model.go` grew to 1370 lines (from 1334). Dashboard additions are minimal (fields + dispatch) but the file remains over threshold.

## Gotchas

- `bgp monitor` was registered as a streaming handler, so `isMonitorCommand()` in `model.go` caught it before dashboard detection. Had to remove the streaming handler registration and add dashboard detection before the streaming check.
- The `commandExecutor` returns `formatResponseData(resp.Data)` (JSON-indented `Data` field only, not the full `plugin.Response`). Dashboard JSON parsing must expect the summary wrapper, not a response envelope.
- Auto-linter (`goimports`) removes imports when referenced symbols don't exist yet, making incremental file creation difficult. Had to create all three dashboard files in quick succession.
- `exhaustive` linter requires all enum cases in switch statements, including the `numSortColumns` sentinel.

## Files

- `internal/component/cli/model_dashboard.go` -- dashboard state, JSON parsing, rate computation, session lifecycle, key handling
- `internal/component/cli/model_dashboard_render.go` -- header, peer table, detail view, footer rendering with lipgloss
- `internal/component/cli/model_dashboard_sort.go` -- sort column enum, sort logic, column cycling
- `internal/component/cli/model_dashboard_test.go` -- 14 unit tests
- `internal/component/cli/model.go` -- dashboard fields, message dispatch, key interception, View() override
- `internal/component/cli/model_render.go` -- dashboard View() short-circuit
- `internal/component/bgp/plugins/cmd/monitor/monitor.go` -- removed `bgp monitor` streaming redirect
- `internal/component/ssh/session.go` -- DashboardFactory wiring in SSH sessions
- `cmd/ze/cli/main.go` -- DashboardFactory wiring in local CLI
- `docs/architecture/api/commands.md` -- verb taxonomy documentation, monitor bgp entry
- `docs/features.md` -- live peer dashboard feature
- `test/plugin/bgp-monitor-dashboard.ci` -- functional test
