# MCP Integration

<!-- source: internal/component/mcp/handler.go -- MCP HTTP handler -->
<!-- source: cmd/ze-test/mcp.go -- MCP test client -->

Ze includes an MCP (Model Context Protocol) server for AI-assisted BGP operations. The server runs inside the daemon and exposes typed tools for route management and peer control.

| Feature | Description |
|---------|-------------|
| Route announcement | `ze_announce` tool with typed parameters (origin, next-hop, communities, prefixes) |
| Route withdrawal | `ze_withdraw` tool |
| Peer monitoring | `ze_peers` tool shows state, ASN, uptime |
| Peer control | `ze_peer_control` for teardown, pause, resume, flush |
| Generic commands | `ze_execute` runs any CLI command via MCP |
| AI reference | `ze help --ai` generates machine-readable command reference from code |
| Testing | `ze-test mcp` client for functional tests with `wait-established` synchronization |

Start with `ze start --mcp <port>` or `ze --mcp <port> config.conf`. Binds to 127.0.0.1 only.

See [MCP Guide](guide/mcp/overview.md) for details and [MCP Remote Access](guide/mcp/remote-access.md) for tunneling.
