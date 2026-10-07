# Chaos MCP

## Meta

| Field | Value |
|-------|-------|
| Name | Chaos MCP |
| Page | docs/guide/mcp/chaos.md |
| Kind | dev-tool |
| Scope | complete |
| Level | supported |
| Components | internal/chaos/mcp, internal/chaos/watchdog |
| Real-path tests | test/chaos-web/mcp-problems.ci, test/chaos-web/mcp-resources-served.ci, test/chaos-web/mcp-status.ci |
| Docs | docs/guide/mcp/chaos.md |
| Doc review | 2026-10-07: description re-read against code: six tools chaos_status, chaos_problems, chaos_peers, chaos_scenario, chaos_control, chaos_execute in internal/chaos/mcp/tools.go; PROBLEM: lines emitted by internal/chaos/watchdog/watchdog.go; per-family convergence served by chaos_status (StatsByFamily) |
| Defect review | 2026-10-07: no journal row or immediate spec names internal/chaos/mcp |

## Description

AI-queryable chaos test state via MCP: 6 tools (status, problems, peers, scenario, control, execute), Watchdog anomaly detector with structured PROBLEM lines, per-family convergence tracking <!-- source: internal/chaos/mcp/tools.go -- chaos MCP tools --> <!-- source: internal/chaos/watchdog/watchdog.go -- Watchdog consumer -->
