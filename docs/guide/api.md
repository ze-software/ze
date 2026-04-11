# REST and gRPC API

Ze exposes a programmatic API over REST (HTTP/JSON) and gRPC. Both transports
share one engine -- they produce identical command output and support the same
commands.

<!-- source: internal/component/api/engine.go -- APIEngine -->
<!-- source: internal/component/api/rest/server.go -- RESTServer -->
<!-- source: internal/component/api/grpc/server.go -- GRPCServer -->

## Quick Start

Enable REST in your config:

```
environment {
    api-server {
        rest {
            enabled true;
            server { ip 0.0.0.0; port 8081; }
        }
    }
}
```

Or via environment variable:

```
ze.api-server.rest.enabled=true
ze.api-server.rest.listen=0.0.0.0:8081
```

Query the API:

```
curl http://localhost:8081/api/v1/commands
curl -X POST http://localhost:8081/api/v1/execute \
    -H "Content-Type: application/json" \
    -d '{"command":"bgp summary"}'
```

Open interactive docs: <http://localhost:8081/api/v1/docs>

## Authentication

Three modes, in order of precedence:

| Mode | How to enable | How clients authenticate |
|------|--------------|-------------------------|
| Per-user (recommended) | Run `ze init` to create the zefs user database | `Authorization: Bearer username:password` |
| Single token | Set `ze.api-server.token=<secret>` or YANG `api-server { token "secret"; }` | `Authorization: Bearer <secret>` |
| No auth | Leave both unset | (no header required) |

Per-user mode uses the same user list as SSH and the web UI. Each request
authenticates as a specific user, and the engine enforces per-user
authorization via the existing dispatcher.

On startup, ze prints the active auth mode:
```
API auth mode: per-user (1 users from zefs)
```

If neither per-user nor token is configured, ze warns:
```
warning: API auth mode: NONE (no users, no token) -- set ze.api-server.token or initialize zefs
```

## REST Endpoints

### Command execution

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/commands` | List all commands with metadata |
| `GET` | `/api/v1/commands/{path}` | Describe one command (e.g., `/bgp/summary`) |
| `POST` | `/api/v1/execute` | Execute any command |
| `GET` | `/api/v1/execute/stream?command=...` | Stream command output via Server-Sent Events |
| `GET` | `/api/v1/complete?partial=...` | Tab completion (future) |

POST `/api/v1/execute` body:
```json
{
  "command": "bgp rib routes",
  "params": {"family": "ipv4/unicast"}
}
```

Response:
```json
{
  "status": "done",
  "data": { ... }
}
```

### Convenience routes

These map to the generic Execute endpoint internally. The data returned is
identical to calling `execute` directly.

| Method | Path | Maps to |
|--------|------|---------|
| `GET` | `/api/v1/peers` | `bgp summary` |
| `GET` | `/api/v1/peers/{name}` | `peer {name} detail` |
| `DELETE` | `/api/v1/peers/{name}` | `peer {name} teardown` |
| `POST` | `/api/v1/peers/{name}/refresh` | `peer {name} refresh` |
| `GET` | `/api/v1/rib/{family}` | `rib routes {family}` |
| `GET` | `/api/v1/rib/{family}/best` | `rib best {family}` |
| `GET` | `/api/v1/system/version` | `show version` |
| `GET` | `/api/v1/system/status` | `daemon status` |
| `POST` | `/api/v1/system/reload` | `daemon reload` |

### Config editing

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/config/running` | Current running config |
| `POST` | `/api/v1/config/sessions` | Start a candidate session, returns `{"session-id": "..."}` |
| `PUT` | `/api/v1/config/sessions/{id}` | Set a value: `{"path":"bgp.router-id","value":"10.0.0.1"}` |
| `DELETE` | `/api/v1/config/sessions/{id}/{path}` | Delete a config path |
| `GET` | `/api/v1/config/sessions/{id}/diff` | Preview pending changes |
| `POST` | `/api/v1/config/sessions/{id}/commit` | Apply changes |
| `DELETE` | `/api/v1/config/sessions/{id}` | Discard session |

Sessions are owned by the authenticated user. Another user cannot access a
session they did not create (returns 403 Forbidden). Idle sessions expire
after 30 minutes.

<!-- source: internal/component/api/config_session.go -- ConfigSessionManager -->

### Documentation

| Path | Description |
|------|-------------|
| `/api/v1/openapi.json` | OpenAPI 3.1 specification (auto-generated from YANG) |
| `/api/v1/docs` | Interactive Swagger UI (assets vendored, offline-capable) |

The OpenAPI spec is generated lazily on first request so it captures all
plugin commands registered during startup.

## gRPC Services

Proto definitions: `api/proto/ze.proto`, package `ze.api.v1`.

Enable gRPC:

```
environment {
    api-server {
        grpc {
            enabled true;
            server { ip 0.0.0.0; port 50051; }
        }
    }
}
```

### ZeService

Generic command execution and discovery.

| RPC | Type | Purpose |
|-----|------|---------|
| `Execute` | unary | Run a command, get result |
| `Stream` | server-stream | Run a streaming command (monitor, subscribe) |
| `ListCommands` | unary | Enumerate all commands |
| `DescribeCommand` | unary | Metadata for one command |
| `Complete` | unary | Tab completion (future) |

`CommandResponse.data` is JSON-encoded bytes for identical content with REST.

### ZeConfigService

Typed config session management (same semantics as REST config sessions).

| RPC | Purpose |
|-----|---------|
| `GetRunningConfig` | Current running config |
| `EnterSession` | Start a candidate session |
| `SetConfig` / `DeleteConfig` | Modify the candidate |
| `DiffSession` | Preview pending changes |
| `CommitSession` | Apply changes |
| `DiscardSession` | Throw away changes |

### gRPC authentication

Pass the same `Bearer username:password` or `Bearer <token>` as REST, via
the `authorization` metadata key:

```python
metadata = [('authorization', 'Bearer alice:password123')]
stub.Execute(CommandRequest(command='bgp summary'), metadata=metadata)
```

### gRPC reflection

Reflection is enabled by default. Discover the schema with `grpcurl`:

```
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 describe ze.api.v1.ZeService
grpcurl -plaintext -d '{"command":"bgp summary"}' \
    -H "authorization: Bearer alice:password123" \
    localhost:50051 ze.api.v1.ZeService/Execute
```

### TLS

Configure TLS via YANG:

```
environment {
    api-server {
        grpc {
            enabled true;
            tls-cert "/etc/ze/server.pem";
            tls-key "/etc/ze/server.key";
        }
    }
}
```

Both fields must be set together. Minimum TLS version is 1.2.

## CORS

For browser-based clients, set an allowed origin:

```
environment {
    api-server {
        rest {
            enabled true;
            cors-origin "https://dashboard.example.com";
        }
    }
}
```

Preflight `OPTIONS` requests are handled automatically.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ze.api-server.rest.enabled` | false | Enable REST API server |
| `ze.api-server.rest.listen` | `0.0.0.0:8081` | REST listen address |
| `ze.api-server.grpc.enabled` | false | Enable gRPC API server |
| `ze.api-server.grpc.listen` | `0.0.0.0:50051` | gRPC listen address |
| `ze.api-server.token` | (empty) | Single bearer token (if per-user auth not wanted) |

Precedence: env > YANG config. Values set in env override YANG.

<!-- source: cmd/ze/hub/main.go -- runYANGConfig API block -->
<!-- source: internal/component/config/environment.go -- env var registrations -->

## Input Validation

Both transports validate command input against shell injection:

- URL path segments (peer name, RIB family) reject whitespace and control chars
- Execute `params` map keys and values reject whitespace
- Config session IDs must match the hex format from `EnterSession`

These checks prevent command tokenizer confusion when user input flows into
dispatcher command strings.

<!-- source: internal/component/api/rest/server.go -- validatePathSegment, validateSessionID -->

## Differences Between Transports

The transports are functionally equivalent. Pick based on client needs:

| Feature | REST | gRPC |
|---------|------|------|
| Discovery | OpenAPI 3.1 + Swagger UI | gRPC reflection + grpcurl |
| Streaming | Server-Sent Events | server-stream RPC |
| Browser support | Yes (with CORS) | Needs grpc-web proxy |
| Tooling | curl, any HTTP client | grpcurl, any gRPC client |
| Overhead | JSON | Protobuf (smaller wire format) |
| TLS | via reverse proxy today | built-in |
