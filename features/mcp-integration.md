# MCP Integration

## Meta

| Field | Value |
|-------|-------|
| Name | MCP Integration |
| Page | docs/features/mcp-integration.md |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/mcp |
| Real-path tests | test/plugin/mcp-mrtr-elicit-roundtrip.ci, test/plugin/mcp-header-mismatch-rejected.ci |
| Docs | docs/features/mcp-integration.md |
| Doc review | 2026-10-07: revision 2026-07-28 and ttlMs/cacheScope read in internal/component/mcp/caching.go; other claims taken from the source anchors |
| Defect review | 2026-10-07: plan/journal/unwired-feature.md (2026-08-15) tools/list readiness race fails mcp-header-mismatch-rejected.ci, unfixed; plan/pre-release/spec-rfc-verdict-fix-services.md ready with MCP verdicts open |
| Extra criteria | supported: a third-party MCP client completes initialize and tools/call = test/interop/mcp-third-party-client.md |

## Description

AI-assisted BGP operations through Model Context Protocol. The transport is stateless Streamable HTTP, revision 2026-07-28: per-request `_meta`, standard request headers, per-request authentication, and `server/discover`. Ze is an OAuth 2.1 resource server. Background tasks are server-directed through the `io.modelcontextprotocol/tasks` extension: the daemon reads each command's `ze:task-support` annotation and decides whether to return a task handle. The client polls a task, and the server never pushes one. MCP Apps UI resources carry embedded panels, and Ze gates them on the `io.modelcontextprotocol/ui` extension. Cacheable results carry `ttlMs` and a `private` `cacheScope` on `server/discover`, `tools/list`, `resources/list` and `resources/read`. Elicitation runs as a Multi Round-Trip Request. The server RETURNS `resultType: "input_required"` with an `inputRequests` map, in form mode. Ze sends this result only when the client declares form-mode support. The client then retries the original call with `inputResponses`, because the revision forbids a server to send an independent request on any stream. Ze mints no `requestState`, so Ze holds nothing between the two requests. The service is default-on and compile-out-able with the `ze_mcp` build tag. normal and appliance builds include it, and `ze-stripped` and bare `ze_core` builds drop `internal/component/mcp` and its schema. An omitted MCP exposes no endpoint, and Ze rejects its `environment { mcp {} }` config block as unknown. <!-- source: feature-gates.txt -- ze_mcp --> <!-- source: cmd/ze/hub/service_mcp.go -- ze_mcp service factory --> <!-- source: cmd/ze/hub/register_mcp.go -- ze_mcp registration --> <!-- source: internal/component/mcp/streamable.go -- Streamable HTTP transport --> <!-- source: internal/component/mcp/meta.go -- per-request protocol metadata --> <!-- source: internal/component/mcp/headers.go -- standard request header validation --> <!-- source: internal/component/mcp/discover.go -- server/discover --> <!-- source: internal/component/mcp/tasks.go -- task registry and workers --> <!-- source: internal/component/mcp/resources.go -- MCP Apps UI resources --> <!-- source: internal/component/mcp/apps.go -- io.modelcontextprotocol/ui extension gate --> <!-- source: internal/component/mcp/caching.go -- cacheable result hints --> <!-- source: internal/component/mcp/tools.go -- ze_execute handcrafted tool --> <!-- source: internal/component/mcp/mrtr.go -- multi round-trip requests --> <!-- source: internal/component/mcp/elicit.go -- form-mode ElicitRequest -->
