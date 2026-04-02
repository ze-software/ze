# AI-First Design

<!-- source: internal/component/mcp/handler.go -- MCP HTTP handler -->
<!-- source: cmd/ze/help_ai.go -- ze help --ai machine-readable reference -->
<!-- source: cmd/ze-test/mcp.go -- MCP test client -->

Ze is designed AI-first: the entire command surface is programmatically accessible and
self-describing. AI assistants do not need documentation -- the binary describes itself.

## Principle: The CLI Is the API

Every command available through `ze cli` (interactive or `ze cli -c` for one-shot) is exposed
programmatically via the MCP `ze_execute` tool. There is no separate "API surface" -- the CLI
and the API are the same thing. This means:

- No commands are CLI-only or API-only
- No translation layer between human and machine interfaces
- Any new CLI command is automatically available to AI assistants

## Self-Describing Command Reference

```
ze help --ai
```

Generates a machine-readable command reference from the live binary. The output is assembled
from the plugin registry, YANG schemas, and RPC registrations -- it cannot go stale because
it is generated from code, not written by hand.

## MCP Transport

The MCP (Model Context Protocol) server wraps the CLI command surface for AI consumption:

| Tool | Description |
|------|-------------|
| `ze_announce` | Announce routes with typed parameters (origin, next-hop, communities, prefixes) |
| `ze_withdraw` | Withdraw routes |
| `ze_peers` | Monitor peer state, ASN, uptime |
| `ze_peer_control` | Teardown, pause, resume, flush peers |
| `ze_execute` | Run **any** CLI command -- full daemon control |

The `ze_execute` tool is the key: anything a human can type, an AI can execute. Route
management, RIB queries, peer lifecycle, configuration changes, event subscription,
schema discovery -- all through one interface.

Start with `ze start --mcp <port>` or configure via YANG (`environment/mcp`).

## What Makes This Different

Other software adds an API endpoint and hopes someone wraps it for AI. Ze exposes its
entire command surface through a self-describing interface that AI can discover and use
without external documentation. `ze help --ai` is generated from code. The MCP tools
have typed parameters. The command list is queryable at runtime (`command-list`,
`command-help <name>`).

No other network daemon -- BGP or otherwise -- is designed this way.

See [MCP Guide](../guide/mcp/overview.md) for configuration and
[MCP Remote Access](../guide/mcp/remote-access.md) for tunneling.
