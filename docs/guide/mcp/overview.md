---
title: MCP Integration (Guide)
---
# MCP Integration

<!-- source: internal/component/mcp/tools.go -- MCP tool dispatch primitives -->
<!-- source: cmd/ze/help_ai.go -- ze help ai output -->

Ze includes an MCP (Model Context Protocol) server that lets AI assistants
control BGP operations. The server runs inside the daemon and wraps the
same command dispatcher used by the CLI and web interface.

## Starting the MCP Server

**CLI flag:**
```bash
ze start --mcp 9718
ze --mcp 9718 config.conf
```

**Config file:**
```
environment {
    mcp {
        enabled true;
        server main {
            ip 127.0.0.1;
            port 9718;
        }
    }
}
```

**Environment variable:**
```bash
export ze_mcp_listen=127.0.0.1:9718
# or simply:
export ze_mcp_enabled=true  # defaults to 127.0.0.1:8080
```

Precedence: CLI > environment variable > config file.

The MCP server binds to `127.0.0.1` only. See
[remote-access.md](remote-access.md) for accessing it from other machines.

## Authentication

<!-- source: internal/component/mcp/auth.go -- AuthMode enum + Identity -->
<!-- source: internal/component/mcp/bearer.go -- bearer / bearer-list strategies -->
<!-- source: internal/component/mcp/oauth.go -- OAuth 2.1 resource-server strategy -->

MCP supports four authentication modes selected by `environment.mcp.auth-mode`:

| Mode | Use case | Config |
|------|----------|--------|
| `none` | Loopback dev / tunnel-only deployments | No extra leaves |
| `bearer` | Single shared secret, one trusted caller | `token` leaf |
| `bearer-list` | Per-identity tokens, many callers, per-identity scopes | `identity[]` list |
| `oauth` | OAuth 2.1 resource server, external AS manages identities | `oauth` container + TLS |

Identity is established per request: every POST presents its own credential and
every POST is checked. There is no session and no session id, so a revoked
token stops working on the very next request, and there is no long-lived
identifier that would act as a bearer credential in its own right.

<!-- source: internal/component/mcp/streamable.go -- authenticate, called from handlePOST -->

`auth-mode none` is not a bypass but an authenticator that accepts every caller
as an anonymous identity, so even an unauthenticated deployment runs the same
uniform code path.

<!-- source: internal/component/mcp/bearer.go -- noneAuthenticator.Authenticate -->

### bearer (legacy single token)

```
environment {
    mcp {
        enabled true;
        auth-mode bearer;
        token my-secret-token;
        server main { ip 127.0.0.1; port 9718; }
    }
}
```

Env var `ze.mcp.token` and CLI flag `--mcp-token` still work. The token leaf
is `ze:sensitive` -- masked in `show config` output. A token set without an
explicit `auth-mode` infers `bearer` for operators upgrading from pre-Phase-2
configs.

### bearer-list (per-identity tokens)

```
environment {
    mcp {
        enabled true;
        auth-mode bearer-list;
        identity alice { token alice-token; scope [ mcp.read mcp.write ]; }
        identity bob   { token bob-token;   scope [ mcp.read ]; }
        server main { ip 127.0.0.1; port 9718; }
    }
}
```

Each identity's token is compared constant-time. The matching entry's name and
scopes become the authenticated identity for that one request. Add, remove, or
rotate identities independently; a rotation takes effect on the next request.

### oauth (OAuth 2.1 resource server)

```
environment {
    mcp {
        enabled true;
        bind-remote true;
        auth-mode oauth;
        oauth {
            authorization-server https://auth.example/;
            audience             https://mcp.example/;
            required-scopes      [ mcp.admin ];
        }
        tls {
            cert /etc/ze/mcp.pem;
            key  /etc/ze/mcp.key;
        }
        server main { ip 0.0.0.0; port 443; }
    }
}
```

Tokens are validated locally: RS256 / RS384 / RS512 / ES256 / ES384 signatures
are verified against JWKS fetched from the authorization server's RFC 8414
metadata document. HS* (HMAC) and `alg: none` are always rejected.
`iss` / `aud` / `exp` / `nbf` / scope claims are validated with 60 s leeway.

`ze config validate` rejects internally inconsistent configurations (oauth
without TLS on a remote bind, oauth without authorization-server, bind-remote
without auth, etc.) before the daemon starts. See
[`rules/exact-or-reject.md`](../../../ai/rules/exact-or-reject.md) for the
contract.

RFC 9728 metadata: when `auth-mode oauth`, the server publishes
`/.well-known/oauth-protected-resource` listing the authorization server(s)
and supported scopes. Clients discover the AS through this URL when they hit
a 401.

### Constant-time comparison

Bearer tokens (both `bearer` and `bearer-list`) use `subtle.ConstantTimeCompare`
so response timing does not reveal which entry matched (or whether any did).
The bearer-list scan visits every entry regardless of early match.

## Protocol

<!-- source: internal/component/mcp/streamable.go -- ProtocolVersion, Endpoint, ServeHTTP, handlePOST -->
<!-- source: internal/component/mcp/meta.go -- parseRequestMeta and the reserved _meta keys -->
<!-- source: internal/component/mcp/headers.go -- validateStandardHeaders -->

The MCP server speaks JSON-RPC 2.0 over HTTP at protocol revision
`2026-07-28`, and accepts no other revision. Each message is its own HTTP POST
to `/mcp`. The profile is stateless: there is no `initialize` handshake, no
session, no `Mcp-Session-Id`, no GET stream, and no server-initiated request.

A typical exchange is simply:

1. (Optional) `server/discover` to learn the supported versions and capabilities
2. `tools/list` to discover available tools
3. `tools/call` with a tool name and arguments

Every POST carries three standard headers plus a `_meta` block **inside
`params`**:

| Header | Required | Must equal |
|--------|----------|------------|
| `MCP-Protocol-Version` | Always | `params._meta["io.modelcontextprotocol/protocolVersion"]` |
| `Mcp-Method` | Always | the body's `method` |
| `Mcp-Name` | `tools/call`, `resources/read`, `prompts/get` | `params.name` (`tools/call`, `prompts/get`) or `params.uri` (`resources/read`) |

| `params._meta` key | Required | Purpose |
|--------------------|----------|---------|
| `io.modelcontextprotocol/protocolVersion` | Yes | The revision this request speaks |
| `io.modelcontextprotocol/clientCapabilities` | Yes | What the client supports; send `{}` for none |
| `io.modelcontextprotocol/clientInfo` | No | Client name and version, for logs and display only |

A minimal conformant request:

```bash
curl -s http://127.0.0.1:9718/mcp \
  -H 'Content-Type: application/json' \
  -H 'MCP-Protocol-Version: 2026-07-28' \
  -H 'Mcp-Method: tools/list' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{
        "io.modelcontextprotocol/protocolVersion":"2026-07-28",
        "io.modelcontextprotocol/clientCapabilities":{}}}}'
```

Capabilities are declared per request, not once per session. Task support is
declared with the `io.modelcontextprotocol/tasks` identifier under
`clientCapabilities.extensions`, and only there: the bare `tasks` member that
MCP 2025-11-25 used is no longer accepted, because that spelling opted into a
model where the client asked for each task individually.

The declaration does two things. It gates the three `tasks/*` methods, which are
refused with `-32021` without it, and it governs whether the server may hand
back a task handle at all: a client that has not declared the extension still
gets its answer, synchronously, as an ordinary `resultType: "complete"` result.
That is the only gate. `resources/list` and `resources/read` are served to
everyone, because `resources` is a *server* capability and a conformant client
has no way to declare it.

<!-- source: internal/component/mcp/meta.go -- parseClientCapabilities -->
<!-- source: internal/component/mcp/streamable_tools.go -- callTool, failMissingTasksCapability -->

Every successful result carries `resultType: "complete"` and
`_meta["io.modelcontextprotocol/serverInfo"]`.

<!-- source: internal/component/mcp/streamable_tools.go -- ok, resultMeta -->

### Errors

<!-- source: internal/component/mcp/streamable_tools.go -- rpc* codes, failUnsupportedVersion, failMissingTasksCapability -->
<!-- source: internal/component/mcp/streamable.go -- httpStatusForDispatch -->

| Code | HTTP | When |
|------|------|------|
| `-32020` | 400 | A required header is missing, or disagrees with the body |
| `-32021` | 400 | The request needs a capability its `_meta` did not declare; `data.requiredCapabilities` names it |
| `-32022` | 400 | The declared version is not one this server implements; `data.supported` lists what is |
| `-32602` | 400 | A required `params._meta` field is missing or malformed |
| `-32601` | 404 | Unknown method. The 404 lets a client tell a modern server from a legacy one that does not host the endpoint |
| — | 405 | GET or DELETE to `/mcp` |

Only `-32601` and `-32021` carry a mandated HTTP status at dispatch time, plus
the three pre-dispatch rejections above (`-32020`, `-32022`, and a malformed
`_meta`) which are all 400. Every other JSON-RPC error, including a `-32602` for
an unknown tool or a bad `taskId`, rides a 200 as JSON-RPC intends.

A legacy client that POSTs `initialize` receives a `-32020` naming the protocol
version this server does speak, because header validation runs before dispatch
and the legacy request carries none of the required headers.

<!-- source: internal/component/mcp/headers.go -- initializeEraError -->

### server/discover

<!-- source: internal/component/mcp/discover.go -- serverDiscover, serverCapabilities, instructions -->

`server/discover` is the one call a client can make to learn what the server
speaks before committing to anything else. It returns `supportedVersions`,
`capabilities`, and an `instructions` string, with `serverInfo` in the result's
`_meta`.

`capabilities.extensions` names two extensions, each with an empty settings
object: `io.modelcontextprotocol/ui`, the MCP Apps extension, and
`io.modelcontextprotocol/tasks`, the Tasks extension. Advertising the second is
not bookkeeping. A client is entitled to reject a `resultType` it does not
recognize, and it recognizes `task` only because this map says the extension is
supported.

### Result caching

<!-- source: internal/component/mcp/caching.go -- cacheTTLByMethod, cacheScopePrivate -->

Four methods return caching hints, so a client can hold a result instead of
re-fetching it every turn:

| Method | `ttlMs` | `cacheScope` |
|--------|---------|--------------|
| `tools/list` | `60000` (60 s) | `private` |
| `server/discover` | `60000` (60 s) | `private` |
| `resources/list` | `3600000` (1 h) | `private` |
| `resources/read` | `3600000` (1 h) | `private` |

`tools/call` and the three `tasks/*` methods (`tasks/get`, `tasks/update`,
`tasks/cancel`) carry no hints; their results are not cacheable.

There is nothing to configure. The two lifetimes follow how the underlying data
changes: the tool list is rebuilt from the command registry on every call, while
the UI assets are compiled into the binary. `cacheScope` is always `private`,
which tells shared gateways and caching proxies never to serve one caller's
response to another.

#### Known limitation: a reload takes up to 60 seconds to reach clients

Ze has no push invalidation for the tool list, so `ttlMs` is the only signal a
client gets. **For up to 60 seconds after a config reload, a client may still
offer a command the reload removed.**

This is a supported mode rather than a defect: the protocol explicitly allows a
server to provide `ttlMs` without also promising change notifications, in which
case the client relies on TTL freshness alone. It is also self-correcting. If a
client does call a command that no longer exists, the error it gets back is one
of the signals the protocol names as a reason to re-fetch early, so the stale
entry is normally gone after a single failed call rather than lingering for the
full minute.

If you need a removed command to disappear from a client immediately, restart
the client's MCP connection after the reload.

## Tools

<!-- source: internal/component/mcp/tools.go -- auto-generated tools from command registry -->

All MCP tools are **auto-generated** from the YANG command registry at
`tools/list` time. Each command group (e.g. `rib`, `show config`, `metrics`)
becomes a tool with an `action` enum listing its subcommands. When a new
YANG command is registered, it appears as an MCP tool automatically without
code changes.

Run `tools/list` against a live daemon to see the current tool inventory.

Two handcrafted tools (`ze_execute`, `ze_reference`) provide escape-hatch and
discovery capabilities alongside the generated tools.

<!-- source: internal/component/mcp/tools.go -- handcraftedTools, toolHandlers -->

### ze_execute

Run any command the CLI supports. Use `ze_reference` to discover available commands.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | Conditional: yes, unless the request declared **form-mode** elicitation | Full command string |

The published `inputSchema` says the same thing, and says it per request: the
`required` array names `command` for a client that declared no form-mode
elicitation, and is absent for one that did. Ze cannot advertise a single answer
here, because the two clients get different behavior and a schema-validating
host would otherwise refuse to make the very call that reaches the prompt.

<!-- source: internal/component/mcp/mrtr.go -- gateExecuteCommandRequired -->
<!-- source: internal/component/mcp/streamable_tools.go -- allTools -->

So a call that omits `command` is not always an error. When the request declared
**form-mode** elicitation, Ze answers with the Multi Round-Trip interim result
and asks for one:

```json
{
  "resultType": "input_required",
  "inputRequests": {
    "ze_execute_command": {
      "method": "elicitation/create",
      "params": { "mode": "form", "message": "Which ze command should be run? ..." }
    }
  }
}
```

The client then retries the same `tools/call`, under a new id, with
`params.inputResponses` carrying the answer. A client that declared no
elicitation capability -- or declared `url` mode only, which Ze does not
implement -- is never prompted and gets a tool error naming the missing
argument. See [MCP Elicitation](elicitation.md).

<!-- source: internal/component/mcp/tools.go -- ze_execute handler -->
<!-- source: internal/component/mcp/mrtr.go -- askForCommand, resolveExecuteCommand -->

### ze_reference

No parameters. Returns the full machine-readable reference for this daemon
(CLI commands, daemon API endpoints with dispatch keys, plugins, address
families, config services) as JSON. This is the same data as `ze help ai --json`,
assembled from `internal/component/aihelp` so the CLI and MCP never diverge.
An MCP client sees this tool in `tools/list` on connect, so it can discover the
instance's capabilities without out-of-band documentation.

<!-- source: internal/component/mcp/tools.go -- ze_reference handcrafted tool -->
<!-- source: internal/component/aihelp/aihelp.go -- Build assembles the reference -->

## AI Help Reference

`ze help ai` generates a machine-readable reference from the running binary.
All data comes from the plugin registry, YANG schemas, and RPC registrations,
so it is never out of date.

| Command | Content |
|---------|---------|
| `ze help ai` | Summary with counts and quick start |
| `ze help ai --json` | Machine-readable JSON with commands, RPCs, plugins, families, services |
| `ze help ai cli` | CLI subcommands (ze bgp, ze config, ...) |
| `ze help ai api` | Daemon API commands with parameters (YANG RPCs) |
| `ze help ai mcp` | MCP tools with parameters and examples |
| `ze help ai dispatch` | Dispatch keys for daemon commands |
| `ze help ai all` | Everything |

The legacy flag form (`ze help --ai --api`) is still accepted as a hidden alias.

## Agent Tooling

Offline commands for agent-driven config validation and repair:

| Command | Purpose |
|---------|---------|
| `ze config validate --json <file>` | Structured diagnostics with stable codes |
| `ze explain [--json] <code>` | Explain a diagnostic code |
| `ze config fix --plan --json <file>` | Plan-only repair candidates |
| `ze skills list [--json]` | List bundled agent skills |
| `ze skills get <name> [--full]` | Load version-matched skill content |

These commands do not require a running daemon. Agents use them to validate
and fix config before committing.

## Testing

<!-- source: internal/test/cli/cmd_mcp.go -- MCP test client -->
<!-- source: test/plugin/mcp-announce.ci -- MCP functional test -->

`ze-test mcp` is an MCP client for functional tests. It reads commands from
stdin and sends them to the MCP endpoint.

```bash
# Start daemon with MCP
ze --mcp 8080 config.conf &

# Send commands
echo 'wait-established
peer * update text origin igp next-hop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24' | ze-test mcp --port 8080
```

Every message it sends is its own POST to `/mcp` with the required headers and
`_meta` block; there is no handshake or session to set up.

Flags:

| Flag | Purpose |
|------|---------|
| `--port <port>` | MCP server port (required) |
| `--token <token>` | Bearer token, sent on every request |
| `--timeout <duration>` | Connection timeout (default 10s) |
| `--tasks` | Declare the `io.modelcontextprotocol/tasks` extension in every request's `_meta.clientCapabilities` |

<!-- source: internal/test/cli/cmd_mcp.go — cmdMcp flag set -->

There is no `--resources` flag. `resources` is a `ServerCapabilities` member,
not one of the five `ClientCapabilities` members, so a conformant client never
declares it and the server serves resources to every caller.

<!-- source: internal/component/mcp/resources.go — resourcesList, resourcesRead -->


Special stdin directives:

| Directive | Description |
|-----------|-------------|
| `# comment` | Ignored |
| `<command>` | Run via `ze_execute` |
| `@tool_name {json}` | Call a specific MCP tool with JSON arguments |
| `wait <duration>` | Pause (e.g. `wait 2s`) |
| `wait-established` | Poll until a BGP peer is Established |
| `wait-peers` | Poll until at least one peer exists |
| `wait-tool <name>` | Poll `tools/list` until the named tool appears |
| `task-call <tool> [<json>]` | Ordinary `tools/call` the server must answer with `resultType: "task"`; prints the taskId. There is no client-side opt-in: the server decides from `ze:task-support` |
| `call-sync <tool> [<json>]` | Ordinary `tools/call` the server must answer synchronously (`resultType: "complete"`, no taskId); prints the result text |
| `task-get <id>` | Get task status |
| `task-result <id>` | Print the result a terminal task carries. Reads it off `tasks/get`, since `tasks/result` no longer exists |
| `task-update <id> [<json>]` | Call `tasks/update` with optional `inputResponses`; requires an empty acknowledgement |
| `task-cancel <id>` | Cancel a task; requires an empty acknowledgement |
| `task-wait <id> <state>` | Poll until task reaches state |

<!-- source: internal/test/cli/cmd_mcp.go -- taskDirective -->

Deliberately-malformed requests, for conformance tests. Each `probe-*` directive
queues one deviation; the next `probe` applies every queued deviation, prints
one result line, then clears the queue. A `probe` with nothing queued sends a
fully conformant request, which is how a test asserts the success shape.

| Directive | Description |
|-----------|-------------|
| `probe-header <name> <value\|->` | Set a request header verbatim, with no sentinel encoding; `-` omits it |
| `probe-meta <key> <value\|->` | Set a `params._meta` field; `-` omits it. The short keys `protocolVersion`, `clientInfo` and `clientCapabilities` expand to their `io.modelcontextprotocol/` names |
| `probe-method <verb>` | HTTP verb for the next probe (default POST), for asserting the 405 on GET and DELETE |
| `probe-body <json\|->` | Send this exact request body; `-` sends an empty body |
| `probe <method> [<json params>]` | Send one request and print `probe status=<http> code=<jsonrpc\|ok\|none> [data=<json>] [result=<json>] message=<text>` |

`MCP-Protocol-Version` is derived from the `_meta` protocolVersion value, so
`probe-meta protocolVersion 2025-06-18` sends a consistent pair that tests
version rejection (`-32022`), while
`probe-header MCP-Protocol-Version 2025-06-18` sends a header/body mismatch that
tests `-32020`.

`$LAST` substitutes the most recent directive output (e.g., taskId from
`task-call`). `task-update` and `task-cancel` deliberately do not update it:
both return an empty acknowledgement rather than an identifier, so `$LAST` keeps naming the
taskId that `task-call` produced.

<!-- source: internal/test/cli/cmd_mcp.go -- taskDirective -->

Example using typed tools:

```
wait-established
@ze_announce {"family":"ipv4/unicast","origin":"igp","next-hop":"1.1.1.1","prefixes":["10.0.0.0/24"]}
@ze_peers {}
```
