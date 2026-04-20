# MCP Integration

<!-- source: internal/component/mcp/handler.go -- MCP HTTP handler -->
<!-- source: cmd/ze-test/mcp.go -- MCP test client -->

Ze includes an MCP (Model Context Protocol) server that makes the BGP daemon **AI-ready**. Any AI assistant (Claude, GPT, or custom agents) can connect via MCP and fully control Ze -- the same operations available through the CLI are accessible programmatically through typed tools.

## AI-Ready BGP Operations

The MCP server exposes typed tools with structured parameters, so AI assistants can manage BGP without parsing CLI output:

| Tool | Description |
|------|-------------|
| `ze_announce` | Announce routes with typed parameters (origin, next-hop, communities, prefixes) |
| `ze_withdraw` | Withdraw routes |
| `ze_peers` | Monitor peer state, ASN, uptime |
| `ze_peer_control` | Teardown, pause, resume, flush peers |
| `ze_execute` | Run **any** CLI command -- full daemon control |
| `ze_commands` | List all available daemon commands |

Additional tools are auto-generated from the command registry at runtime.
Every registered YANG command and plugin command appears as a typed MCP tool
with an `action` enum and optional `arguments` and `peer` parameters. New
commands are exposed automatically without code changes.

The `ze_execute` tool is the key to full control: anything you can do in `ze cli` (interactive or `ze cli -c` for one-shot commands), an AI can do via MCP. This includes:

- **Route management:** `bgp peer * update text origin set igp nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24`
- **RIB queries:** `rib routes received`, `rib routes sent`, `rib clear-in`
- **Peer lifecycle:** `bgp peer * show`, `bgp peer 10.0.0.1 teardown 6`, `set bgp peer new-peer with ...`
- **Configuration:** `commit start window1`, route changes, `commit end window1`
- **Cache operations:** `cache list`, `cache forward`
- **Event subscription:** `subscribe bgp/update`
- **Schema discovery:** `command-list`, `command-help <name>`

## Starting the MCP Server

```
ze start --mcp 8080 config.conf
```

Or via config:

```
environment {
    mcp {
        enabled true
        server main {
            ip 127.0.0.1
            port 8080
        }
    }
}
```

Environment variable overrides: `ze.mcp.listen=ip:port`, `ze.mcp.enabled=true`, `ze.mcp.token=<secret>`. Defaults to `127.0.0.1:8080` (security: local-only unless explicitly overridden via `ze.mcp.listen`). Bearer token auth available via `--mcp-token` flag, `ze.mcp.token` env var, or the config `token` leaf.

## AI Command Reference

```
ze help --ai
```

Generates a machine-readable command reference from code, suitable for feeding to an AI as context. Lists all available commands with their parameters, descriptions, and examples.

## Example: AI-Driven Route Announcement

An AI assistant connected via MCP can:

1. Check peer state: `ze_peers` returns structured JSON with all peer status
2. Announce a route: `ze_announce` with origin=igp, next-hop=10.0.0.1, prefixes=[10.0.0.0/24]
3. Verify propagation: `ze_execute` with command `rib routes sent peer1 ipv4/unicast`
4. Withdraw if needed: `ze_withdraw` with the same prefixes

All without parsing text output -- each tool returns structured data.

## Elicitation (2025-06-18)

Tool handlers may ask the client for missing input mid-dispatch via
`session.Elicit`. The server upgrades the POST reply to
`text/event-stream`, sends an `elicitation/create` request over the
same HTTP body, and resumes the handler when the client POSTs a response
correlated by id. The client must advertise `capabilities.elicitation`
at `initialize` for the server to prompt; otherwise handlers fail fast
with a deterministic error. `ze_execute` illustrates the pattern:
calling it with `command=""` on an elicit-capable client produces an
`elicitation/create` asking which ze command to run.

See [MCP Elicitation](../guide/mcp/elicitation.md) for the full flow.

<!-- source: internal/component/mcp/elicit.go -- session.Elicit, schema validator -->
<!-- source: internal/component/mcp/handler.go -- ze_execute missing-command branch -->

## Testing

`ze-test mcp` provides a functional test client with `wait-established`
synchronization for CI pipelines and -- with `--elicit` plus
`elicit-accept`/`elicit-decline`/`elicit-cancel` stdin directives --
covers the server-initiated elicitation flow.

See [MCP Guide](../guide/mcp/overview.md) for details and
[MCP Remote Access](../guide/mcp/remote-access.md) for tunneling.
