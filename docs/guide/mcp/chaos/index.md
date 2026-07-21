# Chaos MCP Server

ze-chaos includes an MCP (Model Context Protocol) server that lets AI assistants
query chaos test state, detect problems, and control the chaos scheduler.

## Starting

```bash
ze-chaos --mcp :8001 --web :8000 --peers 4 --routes 1000
```

The `--mcp` flag starts an HTTP MCP server on the given address. The `--web`
flag is required because the MCP server reads from the web dashboard's shared
state.

Use `--ai-help` to print the tool definitions as JSON (for configuring AI
clients):

```bash
ze-chaos --ai-help
```

## Ze MCP Injection

To inject an MCP server into the Ze daemon started by ze-chaos:

```bash
ze-chaos --ze-mcp 9718 ...
```

This adds `environment { mcp { port 9718; } }` to the generated Ze config.

## Tools

### chaos_status

Full snapshot of the chaos test state: peer counts, route counts, convergence
statistics (min/avg/max/p50/p90/p99 with histogram), per-family convergence,
throughput, property results.

### chaos_problems

Filtered list of actionable issues. Returns an empty JSON array when healthy.
Problem types: missing-routes, peer-stuck-down, route-plateau, slow-convergence,
dropped-events, property-violation.

### chaos_peers

Per-peer detail. Omit the `peer` argument for a summary of all peers. Provide
a peer index for full detail including recent chaos history (last 5 actions).

### chaos_scenario

Static scenario metadata: seed, peer count.

### chaos_control

Control chaos scheduling. Actions: pause, resume, trigger, rate, stop.
The description steers AI to check status before changing anything.

### chaos_execute

Escape hatch for arbitrary orchestrator commands. Prefer the specific tools
when possible.

## Watchdog

The Watchdog is a report consumer that prints structured `PROBLEM:` lines to
stderr for real-time anomaly detection. It runs regardless of whether the MCP
server is enabled.

### Stateful anomalies (tracked over time)

| Anomaly | Detection |
|---------|-----------|
| Peer not reconnecting | No Established within 30s after Disconnected |
| Route count plateau | recv < sent AND no change for 10s |
| Route regression | recv decreased without chaos withdrawal |
| Convergence stall | No EOR within 2x warmup after Established |

### Instant anomalies (single event)

| Anomaly | Trigger |
|---------|---------|
| Error | EventError |
| Dropped events | EventDroppedEvents |
| Property violation | Property transitions pass to fail |

All problems are rate-limited: the same (anomaly-type, peer-index) pair prints
at most once per 10 seconds.

## Protocol

The chaos MCP server speaks JSON-RPC 2.0 over the MCP 2025-06-18 Streamable
HTTP transport at the `/mcp` endpoint, sharing the same implementation as the
ze daemon's MCP server (`NewStreamable` with the chaos `ToolProvider`).
Because the tool surface comes from a provider, session-less POSTs are
accepted: each request below works standalone, no `Mcp-Session-Id` threading
required.
<!-- source: internal/chaos/orchestrator/run.go -- NewStreamable(Provider) mount -->

```bash
curl -s http://127.0.0.1:8001/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"chaos_status","arguments":{}}}'
```
