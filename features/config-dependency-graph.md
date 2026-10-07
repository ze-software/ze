# Config Dependency Graph

## Meta

| Field | Value |
|-------|-------|
| Name | Config Dependency Graph |
| Kind | daemon |
| Scope | complete |
| Level | supported |
| Components | internal/component/config/cli/cmd_graph.go |
| Real-path tests | test/parse/cli-config-graph.ci |
| Docs | docs/features/configuration.md |
| Doc review | 2026-10-07: re-read at promotion; config/cli/main.go maps graph to cmdGraph, which parses the file against the YANG schema and prints the dependency graph as JSON nodes and edges; matches the row and configuration.md "Dependency Graph" |
| Defect review | 2026-10-07: no open spec or journal row found naming config graph |

## Description

`ze config graph` visualizes config dependency relationships.
