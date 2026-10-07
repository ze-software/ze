# Graceful Listener Migration

## Meta

| Field | Value |
|-------|-------|
| Name | Graceful Listener Migration |
| Kind | daemon |
| Scope | complete |
| Level | supported |
| Components | cmd/ze/hub/listener_migrate.go |
| Real-path tests | test/reload/listener-migration-web-lg-mcp.ci |
| Docs | docs/architecture/web-interface.md, docs/guide/config-reload.md |
| Doc review | 2026-10-07: listener_migrate.go buildChanges names web, lg, mcp, rest and grpc; WebServer, LGServer and mcpServerHandle Reconfigure each bind the new addresses before closing the removed listeners, and close only listeners, so accepted connections keep being served; matches the row. docs/guide/config-reload.md "Moving a Management Listener" re-read against Reconfigure, migrateListeners, rollbackAppliedListeners, detectConflicts and checkReloadExposure: bind-before-close, bind failure fails the reload, rollback of moved services, conflicting services last, unauthenticated non-loopback refusal with the looking glass exempt |
| Defect review | 2026-10-07: no open spec or journal row found naming listener migration |
| Extra criteria | supported: a functional test moving the web and LG and MCP listeners on reload = test/reload/listener-migration-web-lg-mcp.ci; supported: a user page = docs/guide/config-reload.md |

## Description

Hot reload of listener endpoints (web, LG, REST, gRPC, MCP). New listener starts before old one stops; in-flight connections are drained.
