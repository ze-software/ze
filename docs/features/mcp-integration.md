# MCP Integration

<!-- source: internal/component/mcp/tools.go -- MCP tool dispatch primitives -->
<!-- source: internal/test/cli/cmd_mcp.go -- MCP test client -->

Ze exposes daemon commands through an MCP (Model Context Protocol) server.
Clients discover tools from the running daemon and invoke commands with the
caller's permissions. Command availability depends on the registered surface;
an interactive CLI view is not automatically a one-shot MCP operation.
<!-- source: internal/component/command/surface.go -- AnswerSurface, answerView -->

## Discoverable command tools

Generated groups use typed inputs where YANG RPC metadata supplies them.
Groups without typed inputs expose an `arguments` string.
<!-- source: internal/component/mcp/tools.go -- buildToolDef -->

| Tool | Description |
|------|-------------|
| `ze_execute` | Dispatch a supported daemon command string with the caller's permissions. |
| `ze_reference` | Reference assembled from this binary's registries and schemas; the same JSON data as `ze help ai --json`. |
| Generated groups | Discover current names and action/input schemas through `tools/list`; do not assume a route-specific tool name. |

`ze_execute` and `ze_reference` are the fixed tools. Generated tool groups read
the live command list at `tools/list` time. Groups with named actions expose an
`action` enum; selector parameters appear only where the command supports them.
<!-- source: internal/component/mcp/tools.go -- toolHandlers, groupCommands, buildToolDef -->

A plugin command registered with `Hidden` true reaches no tool list. The flag
already removed the command from completion and from help, and `buildCommandMeta`
is the one source both the MCP tool list and the API command list read, so the
skip covers both surfaces.

<!-- source: internal/component/mcp/tools.go -- toolName, generateTools -->
<!-- source: cmd/ze/hub/command_meta.go -- buildCommandMeta: the Hidden skip -->

`ze_execute` uses the daemon command grammar for the operations that command
registrations offer on MCP. Offline commands may have a different daemon path;
terminal-only views and streaming-only operations are not automatically
equivalent one-shot calls. Discover current commands rather than copying an old
route, configuration or cache example.

Notification metadata in `ze_reference` describes the schema. Its presence
does not itself provide an MCP event subscription.
<!-- source: internal/component/command/surface.go -- AnswerSurface, answerOfflineRoot, answerView -->
<!-- source: internal/component/aihelp/aihelp.go -- Build -->

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
ze help ai              # summary
ze help ai all          # detailed listings and authored guidance
ze help ai --json       # registry-derived JSON reference
ze help ai api          # daemon RPCs, update syntax, families and plugins
```

The listings come from the binary's registrations and schemas. Recipes and
explanations are maintained text, not a guarantee that every example is current.
`ze_reference` returns the JSON reference inside MCP text content. The legacy
`ze help --ai` flag form is still accepted.
<!-- source: internal/plugins/help/help_ai.go -- renderAIHelpDocument, printSummary -->
<!-- source: internal/component/mcp/tools.go -- toolHandlers, TextResult -->

## Example: AI-Driven Route Announcement

An AI assistant connected via MCP can:

1. Read `tools/list` and call `ze_reference` to discover this daemon's commands.
2. Call `ze_execute` with `show bgp peer list` to inspect configured peers.
3. Announce through `ze_execute` with `send bgp * update text origin igp next-hop 10.0.0.1 nlri add 10.0.0.0/24`.
4. Call `ze_execute` with `show bgp rib sent peer peer1 family ipv4/unicast` to inspect that peer's sent routes.
5. If withdrawal is needed, use the current `send bgp` withdrawal grammar through `ze_execute`, rather than assuming a `ze_withdraw` tool exists.

Current results use MCP text-content envelopes. JSON references and rendered
command data are carried in that text; the envelope is not a promise of a
separate typed result object for every command.
<!-- source: internal/plugins/help/help_ai.go -- printRecipes -->
<!-- source: internal/component/mcp/tools.go -- toolHandlers, TextResult -->

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

### Client compatibility

A client connects only if it speaks revision `2026-07-28`. One that opens with
the `initialize` handshake of an earlier revision gets error `-32020`, which
names the revision Ze speaks, and cannot connect. The owner-run
[real-client lane](../contributing/mcp-client-verification.md) records each
client tried. In its first round, on 2026-11-11, Claude Code 2.1.322 connected
and called `ze_reference`. Codex 0.167.1 opened with the `2025-06-18`
`initialize` handshake and could not connect.

<!-- source: internal/component/mcp/headers.go -- initializeEraError -->

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

`le test mcp` provides a functional test client with `wait-established`
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

The `tools-size` directive measures the `tools/list` answer a running daemon <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
serves: tool count, total bytes, the largest tool, and each tool's input schema
size and depth. It prints one violation line for each tool that breaks a limit
strict clients enforce: an input schema above 13,312 bytes, nesting deeper
than 10 levels, an `enum` with no sibling `type`, or a `oneOf`, `allOf` or
`anyOf`. `test/plugin/mcp-tools-list-structural-limits.ci` refuses a catalog
that prints a violation.

<!-- source: internal/test/cli/cmd_mcp.go -- sizeDirective -->
<!-- source: internal/test/cli/cmd_mcp_calls.go -- measureToolsSize, toolSchemaBytesMax, toolSchemaDepthMax -->

See [MCP Guide](../guide/mcp/overview.md) for details and
[MCP Remote Access](../guide/mcp/remote-access.md) for tunneling.
