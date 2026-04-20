# MCP Integration

<!-- source: internal/component/mcp/handler.go -- MCP HTTP handler -->
<!-- source: cmd/ze/help_ai.go -- ze help --ai output -->

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
| `bearer-list` | Per-identity tokens, many callers, scope-scoped sessions | `identity[]` list |
| `oauth` | OAuth 2.1 resource server, external AS manages identities | `oauth` container + TLS |

Identity is bound at `initialize` and carried on the session for its lifetime;
subsequent requests with the assigned `Mcp-Session-Id` header are trusted by
session-id validity alone.

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

Each identity's token is compared constant-time. The matching entry's name +
scopes ride on the session. Add / remove / rotate identities independently.

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
[`rules/exact-or-reject.md`](../../.claude/rules/exact-or-reject.md) for the
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

The MCP server speaks JSON-RPC 2.0 over HTTP. Each request is a POST with
a JSON-RPC body. The server implements the MCP tool-calling protocol:

1. Client sends `initialize` to start a session
2. Client calls `tools/list` to discover available tools
3. Client calls `tools/call` with tool name and arguments

## Tools

<!-- source: internal/component/mcp/handler.go -- handcraftedTools variable -->

These tools have hand-tuned parameter schemas:

| Tool | Purpose |
|------|---------|
| `ze_announce` | Announce BGP routes to peers |
| `ze_withdraw` | Withdraw BGP routes from peers |
| `ze_peers` | List peers with state (Idle, Established, ...) |
| `ze_peer_control` | Teardown, pause, resume, or flush a peer |
| `ze_execute` | Run any Ze command (escape hatch) |
| `ze_commands` | List all available daemon commands |

<!-- source: internal/component/mcp/tools.go -- auto-generated tools -->

**Auto-generated tools:** In addition to the tools above, `tools/list`
dynamically generates tools from all registered YANG commands and plugin
commands. Each command group (e.g. `rib`, `show config`, `metrics`) becomes
a tool with an `action` enum listing its subcommands. When a new YANG
command is registered, it appears as an MCP tool automatically without
code changes.

### ze_announce

Announce routes with typed parameters. Builds the UPDATE text command internally.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `peer` | string | No | Peer selector (address, name, or `*`). Default: `*` |
| `origin` | string | No | `igp`, `egp`, or `incomplete` |
| `next-hop` | string | No | Next-hop IP address |
| `local-preference` | integer | No | LOCAL_PREF value |
| `as-path` | array | No | List of ASNs |
| `community` | array | No | List of communities (e.g. `"65000:100"`) |
| `family` | string | Yes | Address family (e.g. `ipv4/unicast`) |
| `prefixes` | array | Yes | Prefixes to announce (e.g. `"10.0.0.0/24"`) |

Example:

```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{
  "name":"ze_announce",
  "arguments":{
    "origin":"igp",
    "next-hop":"1.1.1.1",
    "local-preference":100,
    "family":"ipv4/unicast",
    "prefixes":["10.0.0.0/24","10.0.1.0/24"]
  }
}}
```

### ze_withdraw

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `peer` | string | No | Peer selector. Default: `*` |
| `family` | string | Yes | Address family |
| `prefixes` | array | Yes | Prefixes to withdraw |

### ze_peers

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `peer` | string | No | Peer selector for detailed view. Omit for summary. |

### ze_peer_control

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `peer` | string | Yes | Peer selector |
| `action` | string | Yes | `teardown`, `pause`, `resume`, or `flush` |

### ze_execute

Run any command the CLI supports. Use `ze_commands` to discover available commands.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | Yes | Full command string |

### ze_commands

No parameters. Returns the list of all registered daemon commands.

## AI Help Reference

`ze help --ai` generates a machine-readable reference from the running binary.
All data comes from the plugin registry, YANG schemas, and RPC registrations,
so it is never out of date.

| Flag | Content |
|------|---------|
| `--ai` | Summary with counts and quick start |
| `--ai --cli` | CLI subcommands (ze bgp, ze config, ...) |
| `--ai --api` | Daemon API commands with parameters (YANG RPCs) |
| `--ai --mcp` | MCP tools with parameters and examples |
| `--ai --all` | Everything |

## Testing

<!-- source: cmd/ze-test/mcp.go -- MCP test client -->
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

Special stdin directives:

| Directive | Description |
|-----------|-------------|
| `# comment` | Ignored |
| `wait <duration>` | Pause (e.g. `wait 2s`) |
| `wait-established` | Poll until a BGP peer is Established |
| `@tool_name {json}` | Call a specific MCP tool with JSON arguments |
| `<command>` | Run via `ze_execute` |

Example using typed tools:

```
wait-established
@ze_announce {"family":"ipv4/unicast","origin":"igp","next-hop":"1.1.1.1","prefixes":["10.0.0.0/24"]}
@ze_peers {}
```
