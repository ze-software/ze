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
            server { ip 127.0.0.1; port 8081; }
        }
    }
}
```

REST uses plaintext HTTP and only binds loopback addresses. Use authenticated
gRPC with TLS for management from a non-loopback address.
<!-- source: internal/component/api/rest/server.go -- NewRESTServer loopback check -->

Or via environment variable:

```
ze.api-server.rest.enabled=true
ze.api-server.rest.listen=127.0.0.1:8081
```

Query the API:

```
curl http://localhost:8081/api/v1/commands
curl -X POST http://localhost:8081/api/v1/execute \
    -H "Content-Type: application/json" \
    -d '{"command":"show bgp summary"}'
```
<!-- source: internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang -- module ze-peer-cmd -->

Open interactive docs: <http://localhost:8081/api/v1/docs>

## Authentication

Ze selects one of three API authentication modes:

| Mode | User or token source | Client credentials |
|------|----------------------|--------------------|
| Per-user | A zefs user or a `system.authentication.user` entry | `Authorization: Bearer username:password` |
| Single token | `ze.api-server.token` or YANG `api-server { token "secret"; }` | `Authorization: Bearer <secret>` |
| No auth | No users and no token | No header, with read-only authority |

Per-user mode has precedence. If at least one user exists, the shared token does
not act as a fallback for a failed per-user login.
<!-- source: cmd/ze/hub/main.go -- runYANGConfig API auth mode -->
<!-- source: internal/component/api/rest/auth.go -- RESTServer.withAuth -->
<!-- source: internal/component/api/grpc/server.go -- GRPCServer.checkAuth -->

The per-user source merges the zefs users with `system.authentication.user`
entries from the running config. A config user replaces a zefs user with the
same name. Only zefs users that survive this merge keep the zefs recovery
profile. A successful login carries its authorization view with the
authenticated request. Concurrent requests with the same username cannot
replace each other's resolved profiles.
<!-- source: cmd/ze/hub/main_servers.go -- liveLocalUsers, mergeAuthUsers, usersFromZefsDB -->
<!-- source: cmd/ze/hub/api.go -- buildAPIAuthentication -->
<!-- source: internal/component/aaa/login_profiles.go -- WithProfileAuthorizer, AuthorizerForResult -->

At boot, Ze reads the merged user source once after it populates the config
provider. Ze refuses startup before management listeners bind if that read
fails. An `environment.ssh` block is not required for API users or no-BGP AAA.
<!-- source: cmd/ze/hub/main.go -- runYANGConfig boot user resolution and no-BGP AAA installation -->

Each request uses the current accepted authentication generation. A successful
reload publishes one generation that adds, changes, or removes API users and
authorization policy atomically. The generation includes profile definitions
and user-profile assignments. A failed reload keeps the prior credentials and
policy. The zefs user list stays the boot snapshot.
<!-- source: cmd/ze/hub/api.go -- buildAPIAuthentication -->
<!-- source: cmd/ze/hub/mgmt_auth_reload.go -- apiAuthReloader -->
<!-- source: cmd/ze/hub/aaa_lifecycle.go -- acceptedLocalIdentity, publishAcceptedLocalIdentity -->
<!-- source: cmd/ze/hub/main_reload.go -- runReloadContext -->

Per-user requests keep the authenticated username and the authorizer from that
authentication result. Single-token and no-auth requests use a reserved
server-injected identity. A valid shared token has write authority. A no-auth
request stays read-only, so Ze denies writes before command authorization.
<!-- source: internal/component/aaa/reserved.go -- ReservedSharedAPIUsername -->
<!-- source: internal/component/authz/authz.go -- Store.Authorize, Store.AuthorizeWithProfiles -->
<!-- source: internal/component/api/rest/auth.go -- RESTServer.withAuth, RESTServer.callerIdentity -->
<!-- source: internal/component/api/grpc/server.go -- GRPCServer.checkAuth -->

### Startup output and remedies

Ze writes one of these exact lines when an API server is configured:

```
API auth mode: per-user (1 users)
```

```
API auth mode: single-token (shared bearer)
```

```
warning: API auth mode: NONE (no users, no token) -- set ze.api-server.token or initialize zefs
```

The per-user count includes surviving zefs users and config users. To leave
single-token or NONE mode, add a config user or initialize zefs. To keep shared
credentials, set `ze.api-server.token`.
<!-- source: cmd/ze/hub/main.go -- runYANGConfig API auth mode output -->

## REST Endpoints

### Command execution

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/commands` | List all commands with metadata |
| `GET` | `/api/v1/commands/{path}` | Describe one command (e.g., `/bgp/summary`) |
| `POST` | `/api/v1/execute` | Execute any command |
| `GET` | `/api/v1/execute/stream?command=...` | Stream a registered streaming command, for example `monitor event`, as Server-Sent Events. It uses the same handler registry, authorization, and accounting path as SSH monitor commands. |
| `GET` | `/api/v1/complete?partial=...` | Tab completion (future) |

A plugin command registered with `Hidden` true is absent from
`/api/v1/commands`, as it is from completion, from help, and from the MCP tool
list. `buildCommandMeta` is the one source those surfaces read, so the flag has
one meaning everywhere.
<!-- source: cmd/ze/hub/command_meta.go -- buildCommandMeta: the Hidden skip -->

POST `/api/v1/execute` body:
```json
{
  "command": "show bgp rib",
  "params": {"family": "ipv4/unicast"}
}
```
<!-- source: internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang -- module ze-rib-cmd -->

Response:
```json
{
  "status": "done",
  "data": { ... }
}
```

### Convenience routes

Most convenience routes map to the generic Execute endpoint internally. The
refresh row gives the required generic Execute command instead.

| Method | Path | Maps to |
|--------|------|---------|
| `GET` | `/api/v1/peers` | `show bgp summary` |
| `GET` | `/api/v1/peers/{name}` | `show bgp peer {name} detail` |
| `DELETE` | `/api/v1/peers/{name}` | `request peer {name} teardown` |
| `POST` | `/api/v1/peers/{name}/refresh` | Use `/api/v1/execute` with `request peer {name} refresh {family}` |
| `GET` | `/api/v1/rib/{family}` | `show bgp rib family {family}` |
| `GET` | `/api/v1/rib/{family}/best` | `show bgp rib best family {family}` |
| `GET` | `/api/v1/system/version` | `show version` |
| `GET` | `/api/v1/system/status` | `show status` |
| `POST` | `/api/v1/system/reload` | `request reload` |
<!-- source: internal/component/api/rest/server.go -- RESTServer.handlePeerRefresh -->
<!-- source: internal/component/bgp/plugins/route_refresh/yang/ze-refresh-cmd.yang -- module ze-refresh-cmd; internal/component/bgp/plugins/route_refresh/handler/refresh.go -- handleRefreshMarker -->

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

No-auth REST/gRPC callers cannot create config sessions. Configure a token or
per-user authentication for API-driven config changes.

<!-- source: internal/component/api/config_session.go -- ConfigSessionManager -->

### Documentation

| Path | Description |
|------|-------------|
| `/api/v1/openapi.json` | OpenAPI 3.1 specification (auto-generated from YANG) |
| `/api/v1/docs` | Interactive Swagger UI (assets vendored, offline-capable) |

The OpenAPI spec is generated lazily on first request so it captures all
plugin commands registered during startup. Documentation routes use the same
Bearer authentication policy as the API when auth is configured.
<!-- source: internal/component/api/rest/server.go -- registerRoutes documentation handlers use withAuth -->

## gRPC Services

Proto definitions: `api/proto/ze.proto`, package `ze.api.v1`.

Enable gRPC:

```
environment {
    api-server {
        grpc {
            enabled true;
            server { ip 127.0.0.1; port 50051; }
        }
    }
}
```

Loopback gRPC can run plaintext for local tooling. Non-loopback gRPC listeners
must be authenticated and must configure TLS with both `tls-cert` and `tls-key`.
<!-- source: internal/component/api/grpc/server.go -- NewGRPCServer non-loopback TLS/auth checks -->

### ZeService

Generic command execution and discovery.

| RPC | Type | Purpose |
|-----|------|---------|
| `Execute` | unary | Run a command, get result |
| `Stream` | server-stream | Stream a registered streaming command, for example `monitor event`, over a gRPC server stream. It uses the same handler registry, authorization, and accounting path as SSH monitor commands. |
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
stub.Execute(CommandRequest(command='show bgp summary'), metadata=metadata)
```

### gRPC reflection

Reflection is enabled by default. Discover the schema with `grpcurl` on a
plaintext loopback listener:

```
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 describe ze.api.v1.ZeService
grpcurl -plaintext -d '{"command":"show bgp summary"}' \
    -H "authorization: Bearer alice:password123" \
    localhost:50051 ze.api.v1.ZeService/Execute
```
<!-- source: internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang -- module ze-peer-cmd -->

### TLS

Configure TLS via YANG before binding gRPC outside loopback:

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

Both fields must be set together. Minimum TLS version is 1.2. Startup fails if
an authenticated non-loopback gRPC listener is configured without TLS.

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
| `ze.api-server.rest.listen` | `0.0.0.0:8081` | REST listen address schema default; only loopback is accepted at server startup, so set this to `127.0.0.1:8081` or `::1:<port>` when enabling REST |
| `ze.api-server.grpc.enabled` | false | Enable gRPC API server |
| `ze.api-server.grpc.listen` | `0.0.0.0:50051` | gRPC listen address; non-loopback requires authentication and TLS |
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
| Streaming | Internal SSE hook, production hub returns `streaming not supported` | Internal server-stream RPC, production hub returns `streaming not supported` |
| Browser support | Yes (with CORS) | Needs grpc-web proxy |
| Tooling | curl, any HTTP client | grpcurl, any gRPC client |
| Overhead | JSON | Protobuf (smaller wire format) |
| TLS | via reverse proxy today | built-in |
