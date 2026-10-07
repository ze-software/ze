# Graceful Listener Migration

## Meta

| Field | Value |
|-------|-------|
| Name | Graceful Listener Migration |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | cmd/ze/hub/listener_migrate.go |
| Docs | docs/architecture/web-interface.md |
| Doc review | 2026-10-07: listener_migrate.go names web, lg, mcp, rest and grpc; web Reconfigure binds new addresses before closing old ones |
| Defect review | 2026-10-07: no open spec or journal row found naming listener migration |
| Extra criteria | supported: a functional test moving the web and LG and MCP listeners on reload = none yet; supported: a user page = none yet |

## Description

Hot reload of listener endpoints (web, LG, REST, gRPC, MCP). New listener starts before old one stops; in-flight connections are drained.
