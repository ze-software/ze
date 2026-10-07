# Graceful Listener Migration

## Meta

| Field | Value |
|-------|-------|
| Name | Graceful Listener Migration |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | cmd/ze/hub/listener_migrate.go |
| Real-path tests | test/reload/listener-migration-web-lg-mcp.ci |
| Docs | docs/architecture/web-interface.md |
| Doc review | 2026-10-07: listener_migrate.go names web, lg, mcp, rest and grpc; WebServer, LGServer and mcpServerHandle Reconfigure each bind the new addresses before closing the removed listeners, and close only listeners, so accepted connections keep being served; matches the row. No user page describes moving a listen port on reload |
| Defect review | 2026-10-07: no open spec or journal row found naming listener migration |
| Extra criteria | supported: a functional test moving the web and LG and MCP listeners on reload = test/reload/listener-migration-web-lg-mcp.ci; supported: a user page = none yet |

## Description

Hot reload of listener endpoints (web, LG, REST, gRPC, MCP). New listener starts before old one stops; in-flight connections are drained.
