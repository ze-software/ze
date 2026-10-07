# Local Audit Trail

## Meta

| Field | Value |
|-------|-------|
| Name | Local Audit Trail |
| Page | docs/guide/audit.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/core/audit, cmd/ze/hub/audit.go |
| Real-path tests | test/plugin/audit-config-commit.ci, test/plugin/audit-auth-fail.ci, test/plugin/audit-persistence.ci |
| Docs | docs/guide/audit.md |
| Doc review | 2026-10-07: row prose read against cmd/ze/hub/audit.go defaultAuditPath (a config file gets <name>.audit.jsonl beside it, stdin '-' gets no path) and the auth-fail producers in ssh, web, REST, gRPC and MCP |
| Defect review | 2026-10-07: no journal row or immediate spec names internal/core/audit or cmd/ze/hub/audit.go |
| Extra criteria | supported: an auth-fail row per non-SSH surface (REST, gRPC, MCP, web) in show audit = test/plugin/audit-auth-fail-api-surfaces.ci |

## Description

Structured local audit log for config commit/discard, daemon reload, and failed authentication across SSH, web, REST, gRPC, MCP, CLI, and system surfaces. `show audit` filters by action, actor, surface, time range, and count. Disk-backed JSON-lines storage is used when Ze starts from a config file; stdin configs use memory-only storage.
