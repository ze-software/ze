# MCP Integration

<!-- source: internal/component/mcp/tools.go -- MCP tool dispatch primitives -->
<!-- source: internal/test/cli/cmd_mcp.go -- MCP test client -->

Ze includes an MCP (Model Context Protocol) server that makes the BGP daemon **AI-ready**. Any AI assistant (Claude, GPT, or custom agents) can connect via MCP and fully control Ze -- the same operations available through the CLI are accessible programmatically through typed tools.

## AI-Ready BGP Operations

The MCP server exposes typed tools with structured parameters, so AI assistants can manage BGP without parsing CLI output:

| Tool | Description |
|------|-------------|
| `ze_execute` | Run **any** CLI command -- full daemon control (the escape hatch) |
| `ze_reference` | Full machine-readable reference for this daemon (commands, RPC endpoints, dispatch keys, plugins, families, services); same JSON as `ze help ai --json`. Call first to discover capabilities. |
| `ze_announce` | Announce routes with typed parameters (origin, next-hop, communities, prefixes) |
| `ze_withdraw` | Withdraw routes |
| `ze_show_bgp` | BGP peer state, ASN, uptime, and summary views (auto-generated from `show bgp ...`) |
| `ze_request_peer` | Peer lifecycle: teardown, pause, resume, flush (auto-generated from `request peer ...`) |

`ze_execute` and `ze_reference` are the two handcrafted tools. Every other tool
is auto-generated from the command registry at runtime: a command prefix becomes
a tool name (`show bgp rib` becomes `ze_show_bgp_rib`), with an `action` enum and
optional `arguments` and `peer` parameters. New commands are exposed
automatically without code changes.

A plugin command registered with `Hidden` true reaches no tool list. The flag
already removed the command from completion and from help, and `buildCommandMeta`
is the one source both the MCP tool list and the API command list read, so the
skip covers both surfaces.

<!-- source: internal/component/mcp/tools.go -- toolName, generateTools -->
<!-- source: cmd/ze/hub/command_meta.go -- buildCommandMeta: the Hidden skip -->

The `ze_execute` tool is the key to full control: anything you can do in `ze cli` (interactive or `ze cli -c` for one-shot commands), an AI can do via MCP. This includes:

- **Route management:** `peer * update text origin set igp nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24`
- **RIB queries:** `show bgp rib received`, `show bgp rib sent`, `clear bgp rib in`
- **Peer lifecycle:** `show bgp peer list`, `request peer 10.0.0.1 teardown 6`, `delete bgp peer <sel>`
- **Configuration:** `request commit start window1`, route changes, `request commit end window1`
- **Cache operations:** `show cache`, `request cache forward`
- **Event subscription:** `request subscribe bgp/update`
- **Schema discovery:** `command-list`, `command-help <name>`

## Starting the MCP Server

```
ze start --mcp 8080            # start from stored (blob) config; run `ze init` first
ze --mcp 8080 config.conf      # start from a config file
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
ze help ai
ze help ai api          # daemon API endpoints (ze-show:*, ze-set:*, ...)
```

Generates a machine-readable command reference from code, suitable for feeding to an AI as context. Lists all available commands with their parameters, descriptions, and examples. The legacy `ze help --ai` flag form is still accepted.

## Example: AI-Driven Route Announcement

An AI assistant connected via MCP can:

1. Check peer state: `ze_show_bgp` returns structured JSON with all peer status
2. Announce a route: `ze_announce` with origin=igp, next-hop=10.0.0.1, prefixes=[10.0.0.0/24]
3. Verify propagation: `ze_execute` with command `show bgp rib sent peer peer1 family ipv4/unicast`
4. Withdraw if needed: `ze_withdraw` with the same prefixes

All without parsing text output -- each tool returns structured data.

## Protocol (2026-07-28)

Ze speaks MCP protocol revision `2026-07-28` and no other. The profile is
stateless, and every message is its own HTTP POST to `/mcp`. Each POST carries
its own protocol version, client capabilities and credential. Three standard
headers and a `_meta` block inside `params` hold them. There is no `initialize`
handshake, no session, no `Mcp-Session-Id`, no GET stream, and no
server-initiated request.

<!-- source: internal/component/mcp/streamable.go -- ProtocolVersion, supportedProtocolVersions, handlePOST -->
<!-- source: internal/component/mcp/meta.go -- parseRequestMeta -->

Two consequences are worth stating plainly:

- **Authentication runs on every request**, so a revoked token stops working on
  the very next call rather than at session expiry, and there is no long-lived
  identifier that acts as a bearer credential in its own right.
  <!-- source: internal/component/mcp/streamable.go -- authenticate -->
- **Elicitation is inverted, not gone.** The revision forbids a server to send
  an independent JSON-RPC request on any stream, so a server can no longer push
  a prompt. The server RETURNS the prompt instead: `ze_execute` called without a
  `command` answers `resultType: "input_required"` with an `inputRequests` map,
  and that map carries an `elicitation/create` request. The client then retries
  the original call with `inputResponses`, which carries the value. Ze attaches
  no `requestState`, so it holds nothing between the two requests and
  authenticates each one on its own. A client that did not declare **form-mode**
  elicitation is never prompted, and it gets the missing-argument error instead.
  See [MCP Elicitation](../guide/mcp/elicitation.md) for the full round trip.
  <!-- source: internal/component/mcp/mrtr.go -- inputRequiredForMissingCommand, elicitationFormSupported -->
  <!-- source: internal/component/mcp/tools.go -- ze_execute handler -->

Capability discovery is a single optional call: `server/discover` returns the
supported versions, the server's capabilities, and natural-language
instructions. Its `capabilities.extensions` names two extensions, each with an
empty settings object: `io.modelcontextprotocol/ui`, the MCP Apps extension, and
`io.modelcontextprotocol/tasks`, the Tasks extension. The second extension is
what makes the `task` result type interpretable, because a client can reject a
`resultType` that no advertised extension defines.

<!-- source: internal/component/mcp/discover.go -- serverDiscover, serverCapabilities -->

### Cacheable results

`server/discover`, `tools/list`, `resources/list` and `resources/read` return <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
`ttlMs` and `cacheScope`, so a client can hold a result and does not re-fetch it
every turn. The tool inventory and the discovery result are fresh for 60
seconds. The embedded UI assets are fresh for one hour. Every one of them is
scoped `private`, so a shared gateway is not permitted to serve one caller's
response to another.

`tools/call` and the `tasks/*` methods return no hints. Their results are not <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
cacheable.

Ze has no push invalidation. That 60-second TTL is therefore also the window in
which a client can still offer a command that a config reload has removed. A
call to that command returns an error. The protocol names such an error as
grounds for an early re-fetch, so the stale entry usually clears after one
failed call.

<!-- source: internal/component/mcp/caching.go -- cacheTTLByMethod, cacheScopePrivate -->

### MCP Apps

Tool descriptors for command groups carrying a `ze:ui-resource` YANG annotation
include `_meta.ui`, pointing at a `ui://` asset the host renders in a sandboxed
panel. That metadata is emitted only when the request declared the
`io.modelcontextprotocol/ui` extension in a form compatible with the HTML
bundles Ze serves. A host without MCP Apps support gets the same tool list, the
same tools and the same behavior, minus the panel metadata. Ze rejects nothing.

<!-- source: internal/component/mcp/apps.go -- clientSupportsUIApps, gateUIMeta -->

### Resources

`resources/list` and `resources/read` serve every conformant caller, with no <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
client-capability gate. `resources` is a member of `ServerCapabilities`, not of
`ClientCapabilities`, whose complete member set in this revision is
`experimental`, `roots`, `sampling`, `elicitation` and `extensions`. A conformant
client therefore never declares `resources`, and a gate on it refused every
conformant caller while `server/discover` advertised the capability and
`tools/list` published `_meta.ui.resourceUri` pointing at those same assets. <!-- doc-links: ignore (JSON-RPC method name, not a path) -->

A `ui://` URI is validated before any read: the scheme must match, the cleaned
path must equal the given path, and the depth is capped at 8 segments. A URI that
fails validation gets `invalid uri`, and one that resolves to nothing gets
`resource not found`.

<!-- source: internal/component/mcp/resources.go -- resourcesList, resourcesRead, validateResourceURI -->

## Testing

`ze-test mcp` provides a functional test client with `wait-established`
synchronization for CI pipelines. It also provides a `probe-*` directive family,
which drives deliberately-malformed requests at the conformance surface (header
mismatch, unsupported version, malformed `_meta`, GET and DELETE).

Background execution is driven by `task-call` (an ordinary `tools/call` the <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
server must answer with a task handle), its twin `call-sync` (which requires a
synchronous answer and no taskId), then `task-get`, `task-result`,
`task-update`, `task-cancel` and `task-wait`. The `--tasks` flag declares the
`io.modelcontextprotocol/tasks` extension on every request. A run without the
flag is itself a test, because the server must still serve an undeclaring client
synchronously.

<!-- source: internal/test/cli/cmd_mcp.go -- taskDirective -->

See [MCP Guide](../guide/mcp/overview.md) for details and
[MCP Remote Access](../guide/mcp/remote-access.md) for tunneling.
