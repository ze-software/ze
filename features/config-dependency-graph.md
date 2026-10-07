# Config Dependency Graph

## Meta

| Field | Value |
|-------|-------|
| Name | Config Dependency Graph |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/cli/cmd_graph.go |
| Real-path tests | test/parse/cli-config-graph.ci |
| Docs | docs/features/configuration.md |
| Doc review | 2026-10-07: config/cli/main.go maps graph to cmdGraph; matches the row |
| Defect review | 2026-10-07: no open spec or journal row found naming config graph |

## Description

`ze config graph` visualizes config dependency relationships.
