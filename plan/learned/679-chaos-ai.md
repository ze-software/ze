# 679: chaos-ai

AI-friendly chaos testing: MCP server for ze-chaos, Watchdog anomaly detector, per-family convergence tracking.

## Context

ze-chaos runs chaos tests against a Ze route server. This spec makes it AI-queryable so Claude can run tests, detect problems, and investigate issues programmatically through MCP.

## Decisions

| Decision | Rationale |
|----------|-----------|
| ToolProvider interface in mcp package | Shared JSON-RPC protocol for both ze and ze-chaos MCP servers; avoids duplicating HTTP/JSON-RPC plumbing |
| Handler accepts ToolProvider, not factory | Handler has no external callers (only Streamable used in production); per-request state not needed for chaos tools |
| Streamable path unchanged | Only Handler refactored; Streamable continues using server struct directly to avoid scope creep |
| Watchdog always enabled | PROBLEM lines to stderr are useful even without MCP; zero cost when no anomalies |
| --mcp requires --web | MCP reads DashboardState which is owned by the web dashboard; standalone state would duplicate tracking |
| Per-family convergence via variadic param | Backward compatible: existing callers without family arg unchanged; replay code unaffected |
| Problems list capped at 10,000 | Long-running chaos tests can generate thousands of problems; unbounded growth is a memory leak |
| Route regression uses high-water mark | Monotonically incrementing recv counter can't regress on EventRouteReceived; HWM tracks the peak and fires on EventRouteWithdrawn |
| sendfile= directive in test runner | .ci test format had no way to send POST request bodies; needed for MCP functional tests |

## Patterns

- **ToolProvider interface**: `ServerName()`, `Tools()`, `CallTool()`. Chaos provider reads DashboardState under RLock. Ze provider wraps CommandDispatcher and reuses existing server struct internally.
- **Rate-limit key**: `(anomaly-type, peer-index)` pair with last-printed time. Same anomaly on different peers prints independently.
- **sendfile= in .ci tests**: `tmpfs=file.json` creates the body content, `http=post:sendfile=file.json` sends it. Path resolved against TmpfsTempDir first, then ciDir.

## Mistakes

| Mistake | Impact | Fix |
|---------|--------|-----|
| Route regression check on EventRouteReceived (counter just incremented, can never regress) | Dead code, no regression detection | Moved to EventRouteWithdrawn with HWM comparison |
| --mcp without --web silently did nothing | User confusion | Explicit error message |
| Port conflict validation missing new flags | Conflicting ports undetected | Added mcpAddr and zeMCPPort to both validators |

## Files

None recorded.
