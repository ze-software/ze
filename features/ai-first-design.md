# AI-First Design

## Meta

| Field | Value |
|-------|-------|
| Name | AI-First Design |
| Page | docs/features/ai-first.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/mcp |
| Real-path tests | test/plugin/mcp-discover-daemon.ci, test/plugin/mcp-tools-list-stateless.ci, test/plugin/mcp-tools-call-stateless.ci, test/ui/help-ai.ci, test/ui/help-ai-json.ci |
| RFCs | rfc9728 |
| Docs | docs/features/ai-first.md |
| Doc review | 2026-11-04: independently compared the Description and docs/features/ai-first.md against reference/help producers, MCP tool generation and dispatch, command-surface restrictions, diagnostics, bundled skills, commit preparation, and builtin/plugin discovery; registry-derived listings are distinguished from authored guidance, without universal CLI/MCP parity or freshness claims. Source review only; runtime evidence remains separately bounded. |
| Defect review | 2026-10-07: open: plan/release/fixes/spec-mcp-protected-resource-metadata-path.md (RFC 9728 metadata path) |

## Description

Self-describing CLI-as-API with MCP transport for AI assistants
