# Chaos MCP

## Meta

| Field | Value |
|-------|-------|
| Name | Chaos MCP |
| Page | docs/guide/mcp/chaos.md |
| Kind | dev-tool |
| Scope | complete |
| Level | experimental |
| Components | internal/chaos/mcp, internal/chaos/watchdog |
| Docs | docs/guide/mcp/chaos.md |
| Doc review | 2026-10-07: six tools chaos_status, chaos_problems, chaos_peers, chaos_scenario, chaos_control, chaos_execute in internal/chaos/mcp/tools.go |
| Defect review | 2026-10-07: no journal row or immediate spec names internal/chaos/mcp |
| Extra criteria | supported: the chaos-web MCP .ci run in a suite the functional runner declares = test/chaos-web/mcp-status.ci |

## Description

AI-queryable chaos test state via MCP: 6 tools (status, problems, peers, scenario, control, execute), Watchdog anomaly detector with structured PROBLEM lines, per-family convergence tracking <!-- source: internal/chaos/mcp/tools.go -- chaos MCP tools --> <!-- source: internal/chaos/watchdog/watchdog.go -- Watchdog consumer -->
