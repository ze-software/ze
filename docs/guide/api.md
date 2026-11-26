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
    -d '{"command":"show bgp"}'
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

The per-user source merges managed-store users with `system.authentication.user`
entries from the running config. A config user replaces a stored user with the
same name. Only stored users that survive this merge keep the recovery
profile. A successful login carries its authorization view with the
authenticated request. Concurrent requests with the same username cannot
replace each other's resolved profiles.
<!-- source: cmd/ze/hub/main_servers.go -- liveLocalUsers, mergeAuthUsers, usersFromStore -->
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
warning: API auth mode: NONE (no users, no token) -- set ze.api-server.token or run ze init
```

The per-user count includes surviving zefs users and config users. To leave
single-token or NONE mode, add a config user or run `ze init`. To keep shared
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

Each command in the list, and the object that `/api/v1/commands/{path}`
returns, carries the keys `name`, `short-help`, `description`, `read-only`
and `params`. Each entry in `params` carries `name`, `type`, `short-help`,
`description`, `required`, `anchor` and `flag`: `anchor` names the command-path
keyword the value follows, and `flag` is `true` when the keyword alone supplies
the argument. Empty `short-help`, `description`, `params` and `anchor` values,
and a `false` `flag`, are omitted. `read-only` and `required` are always sent.

```json
{
  "name": "show bgp rib",
  "short-help": "Show routes",
  "read-only": true,
  "params": [{"name": "family", "type": "string", "required": false}]
}
```
<!-- source: internal/component/api/types.go -- CommandMeta, ParamMeta json tags -->
<!-- source: cmd/ze/hub/command_meta.go -- anchoredParams -->

POST `/api/v1/execute` body:
```json
{
  "command": "show bgp rib",
  "params": {"family": "ipv4/unicast"}
}
```

The optional `pipe` list applies pipe operations to the answer, in order, as
if typed after the command. This body answers what
`show bgp rib | received | count` answers:
```json
{
  "command": "show bgp rib",
  "pipe": [{"name": "received"}, {"name": "count"}]
}
```
Each element holds a `name` and an optional `args` array, one word per
element (`{"name": "match", "args": ["link down"]}` matches the two words as
one). A name that is not one of the command's pipe operations is refused, and
the refusal lists the valid ones; `show command help "<command>"` lists them
too. An element key other than `name` and `args` is refused.
<!-- source: internal/component/command/pipe_list.go -- WritePipeList -->
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
| `GET` | `/api/v1/peers` | `show bgp` |
| `GET` | `/api/v1/peers/{name}` | `show bgp peer {name} detail` |
| `DELETE` | `/api/v1/peers/{name}` | `request peer {name} teardown` |
| `POST` | `/api/v1/peers/{name}/refresh` | Use `/api/v1/execute` with `request peer {name} refresh {family}` |
| `GET` | `/api/v1/rib/{family}` | `show bgp rib \| family {family}` |
| `GET` | `/api/v1/rib/{family}/best` | `show bgp rib best \| family {family}` |
| `GET` | `/api/v1/system/version` | `show version` |
| `GET` | `/api/v1/system/status` | `show status` |
| `POST` | `/api/v1/system/reload` | `request reload` |
<!-- source: internal/component/api/rest/server.go -- RESTServer.handlePeerRefresh -->
<!-- source: internal/component/bgp/plugins/route_refresh/yang/ze-refresh-cmd.yang -- module ze-refresh-cmd; internal/component/bgp/plugins/route_refresh/handler/refresh.go -- handleRefreshMarker -->

### Config editing

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/config/running` | The configuration the daemon applied, from memory (`show config running`). After a refused reload it still answers the config in force, not the file. Secret values are replaced with a placeholder |
| `POST` | `/api/v1/config/sessions` | Start a candidate session, returns `{"session-id": "..."}` |
| `PUT` | `/api/v1/config/sessions/{id}` | Set a value: `{"path":"bgp.router-id","value":"10.0.0.1"}` |
| `DELETE` | `/api/v1/config/sessions/{id}/{path}` | Delete a config path |
| `GET` | `/api/v1/config/sessions/{id}/diff` | Preview pending changes |
| `POST` | `/api/v1/config/sessions/{id}/commit` | Apply changes |
| `DELETE` | `/api/v1/config/sessions/{id}` | Discard session |

Sessions are owned by the authenticated user. Another user cannot access a
session they did not create (returns 403 Forbidden). Idle sessions expire
after 30 minutes.

While a `commit confirmed` window is open, a session commit is refused for
every user with the window's refusal naming its owner and the seconds left,
and the session stays open for a retry once the window is accepted or
aborted (`config-editor.md`, "Commit Confirmed").

Session commits apply one at a time. A commit that arrives while another is
being applied waits its turn, then applies as its own commit with its own
version in the config history, so two commits land in arrival order and both
answer `{"status":"committed"}`. Each session commits the whole candidate it
built, so the later commit replaces what the earlier one changed wherever the
two sessions edited the same value.

No-auth REST/gRPC callers cannot create config sessions. Configure a token or
per-user authentication for API-driven config changes.

<!-- source: internal/component/api/config_session.go -- ConfigSessionManager -->

A session refuses a new list key that its YANG type does not accept before
changing the candidate. The answer lists the accepted keys with their help
and ends with the complete corrected `set` command, including the setting
and value after the key. For example, setting
`ospf.redistribute.bogus.nssa-propagate` to `true` proposes
`fix: set ospf redistribute <connected|static|kernel|bgp|isis> nssa-propagate true`.
REST answers HTTP 400, and gRPC answers `InvalidArgument`. gNMI `Set` uses the
same editor and answers `InvalidArgument` with the same refusal. A commit
that refuses a value leaves the running configuration unchanged; the refusal
follows [Value refusals](../architecture/config/yang-config-design.md#value-refusals).
<!-- source: internal/component/cli/editor_commands.go -- SetValue -->
<!-- source: internal/component/api/config_session.go -- Set, SetSegments -->

### Documentation

| Path | Description |
|------|-------------|
| `/api/v1/openapi.json` | OpenAPI 3.1 specification (auto-generated from YANG) |
| `/api/v1/docs` | Interactive Swagger UI (assets vendored, offline-capable) |
| `/api/v1/docs/swagger-ui.LICENSE`, `/api/v1/docs/swagger-ui.NOTICE` | swagger-ui's licence and notice, as plain text |
| `/api/v1/docs/swagger-ui-bundle.js.LICENSE.txt` | The licence banners of the libraries in the bundle, which the bundle's first line points at |

Every documentation path, the licence files included, sits behind the same
authentication as the API it describes.
<!-- source: internal/component/api/rest/server.go -- the documentation routes, licenceHandler -->

The OpenAPI spec is generated lazily on first request so it captures all
plugin commands registered during startup. Documentation routes use the same
Bearer authentication policy as the API when auth is configured.

Every path the document publishes is a route the server answers. A command
has no path of its own: every command runs through `POST /api/v1/execute`, so
the document publishes each command as a schema under `components.schemas`,
keyed by its camel-case name (`show bgp rib` is `showBgpRib`). That schema is
the request body that runs the command: `command` fixed to the command's
name, its parameters under `params`, and the optional `pipe` list.

Each command schema carries the two help texts the command declares. The
`title` is the one-line summary from the YANG `ze:help` statement, and
the `description` is the long explanation from the YANG `description`. A command that
declares no explanation carries no `description` key. Each parameter property
under the schema's `params` carries the same pair from its YANG leaf:
`title` is the leaf's `ze:help` and `description` is the leaf's `description`,
and a text the leaf does not declare writes no key. The gRPC `CommandInfo` and
`ParamInfo` messages carry both texts as `short_help` and `description`.
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
| `GetRunningConfig` | The configuration the daemon applied, as one JSON object (`show config running`). A daemon with no running configuration yet answers `UNAVAILABLE` |
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
stub.Execute(CommandRequest(command='show bgp'), metadata=metadata)
```

`CommandRequest` carries the same optional pipe list as REST execute, as
`repeated PipeOperation pipe`, each with a `name` and `repeated string args`;
`Execute` and `Stream` both apply it:

```python
stub.Execute(CommandRequest(command='show bgp rib',
    pipe=[PipeOperation(name='received'), PipeOperation(name='count')]), metadata=metadata)
```
<!-- source: api/proto/ze.proto -- CommandRequest, PipeOperation -->

### gRPC reflection

Reflection is enabled by default. Discover the schema with `grpcurl` on a
plaintext loopback listener:

```
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 describe ze.api.v1.ZeService
grpcurl -plaintext -d '{"command":"show bgp"}' \
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

Both fields must be set together, and the minimum TLS version is 1.2. At
`ze start`, an authenticated non-loopback gRPC listener without TLS leaves only
gRPC down and reported under `doctor-service-not-started`, while REST and Ze
keep running. Reloads still refuse that configuration.
<!-- source: internal/component/api/grpc/server.go -- NewGRPCServer -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig, grpcBuild -->

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

## Startup and Reload Behavior

Ze always starts even when an API transport cannot be served. Only the failing
transport or breaching address stays down, while the other transport and Ze
keep running. Ze logs an error with the fix and reports it in `ze doctor` under
`doctor-service-not-started`.

| Startup problem | What `ze start` does |
|-----------------|----------------------|
| `ze.api-server.rest.listen` or `ze.api-server.grpc.listen` does not parse | That transport is not started, and the other transport still serves |
| The REST or gRPC server cannot be built (for example, a non-loopback REST address or a non-loopback gRPC address without TLS) | That transport is not started, and the other transport still serves |
| A non-loopback REST or gRPC address has no authentication | Only the breaching address is not started, while loopback addresses and other services keep running |
| A registered `rest` or `grpc` block fails schema or reference validation | `LoadConfigForStart` drops only that block in memory while leaving the stored file untouched |

A reload carrying any of these problems is refused as a whole, and every
running listener keeps what it serves.
<!-- source: cmd/ze/hub/api_infra.go -- planAPIListeners, resolveAPIListeners -->
<!-- source: cmd/ze/hub/mgmt_guard.go -- filterMgmtListener, filterAPIConfig -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig, restBuild, grpcBuild -->
<!-- source: cmd/ze/hub/service_not_started.go -- reportServiceNotStarted, buildFailed, listenUnreadable -->
<!-- source: internal/component/config/loader.go -- LoadConfigForStart -->

## Input Validation

Both transports validate command input against shell injection:

- URL path segments (peer name, RIB family) reject whitespace and control chars
- Execute `params` map keys and values reject whitespace
- Each `pipe` list name must be one of the command's pipe operations, and
  each argument word is written quoted when it holds a space, a pipe or a
  quote, so no element opens a stage of its own
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
