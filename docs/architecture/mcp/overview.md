# MCP Architecture Overview

Ze exposes the [Model Context Protocol](https://modelcontextprotocol.io/) so that
AI assistants can drive the BGP daemon through the same command surface a human
operator uses at the CLI. The implementation lives in
`internal/component/mcp/` and is mounted by `cmd/ze/hub/` on an HTTP listener
configured under `environment.mcp.server`.

<!-- source: internal/component/mcp/tools.go — MCP component tool dispatch primitives -->
<!-- source: internal/component/mcp/streamable.go — Streamable HTTP transport -->
<!-- source: cmd/ze/hub/service_mcp.go — production mount point (ze_mcp) -->

## Protocol Profile

| Profile | Status | Used By |
|---------|--------|---------|
| 2026-07-28 Streamable HTTP | Current (only transport) | `internal/component/mcp/streamable.go` (`NewStreamable`) |

`ProtocolVersion` is `2026-07-28` and `supportedProtocolVersions` holds exactly
that one entry, so no earlier revision is accepted, aliased, or defaulted to.

<!-- source: internal/component/mcp/streamable.go — ProtocolVersion, supportedProtocolVersions, isSupportedProtocolVersion -->

`cmd/ze/hub/service_mcp.go:startMCPServer` mounts `NewStreamable` for all
production listeners, including the `ze-chaos` orchestrator, which supplies a
`ToolProvider` to replace the tool surface. Provider mode changes only which
tools the server offers. It takes the same header validation, the same
per-request metadata, and the same per-request authentication as every other
caller.

<!-- source: internal/component/mcp/streamable.go — StreamableConfig.Provider -->

## Files

| File | Concern |
|------|---------|
| `tools.go` | JSON-RPC 2.0 types (`request`, `response`, `rpcError`, `callParams`), handcrafted tool catalogue (`ze_execute`, `ze_reference`), tool runner helper (`server` struct), `ToolProvider` interface, command-registry -> MCP tool auto-generation: grouping, schema emission, dispatch |
| `streamable.go` | Streamable HTTP dispatcher: `ServeHTTP` (POST + OPTIONS, GET and DELETE answer 405), Origin gate, CORS, RFC 9728 metadata endpoint, `handlePOST` validation pipeline, `httpStatusForDispatch` |
| `meta.go` | Per-request `_meta` parsing: `parseRequestMeta`, the reserved `io.modelcontextprotocol/*` key names, and the `clientCapabilities` value type |
| `headers.go` | Standard request header validation: `validateStandardHeaders`, `mcpNameSource`, the `=?base64?...?=` sentinel decoder, `Mcp-Param-*` field-value checks |
| `discover.go` | `server/discover`: `serverCapabilities`, `instructions`, `serverInfo` |
| `streamable_tools.go` | `runMethod` dispatch switch, `requestScope`, `callTool` (the server-directed task decision), `createTask`, the `tasks/get`, `tasks/update` and `tasks/cancel` handlers, the `ok()`/`fail()` response builders and the MCP error codes |
| `resources.go` | MCP resources capability: `resources/list` (walks embedded FS), `resources/read` (serves `ui://` assets), MIME sniffer, URI validator |
| `caching.go` | Cacheable-result hints: the per-method `ttlMs` table, the unconditional `"private"` cache scope, and the single stamp site |
| `mrtr.go` | Multi Round-Trip Requests: the `InputRequiredResult` builder, the `inputRequests`/`inputResponses` keys, the form-mode capability gate, the unsolicited-`requestState` rejection, and `ze_execute`'s re-entrant command resolution |
| `elicit.go` | Form-mode elicitation: the flat-primitive `requestedSchema` validator, the action sentinels, and the `ElicitRequest` builder whose value goes into `inputRequests` |
| `apps.go` | The `io.modelcontextprotocol/ui` extension: its identifier, the client-settings gate, and the `_meta.ui` strip applied when the gate is closed |
| `ui/embed.go` | `//go:embed` directive exposing UI bundles as `embed.FS` |
| `task_state.go` | Typed `TaskState uint8` enum with `MarshalText`/`UnmarshalText` for wire serialization. Four producible states. `input_required` is deliberately absent |
| `tasks.go` | Task registry (`taskRegistry`) keyed by authenticated principal, worker goroutine orchestration, TTL GC, and the per-worker execution deadline |
| `yang/ze-mcp-conf.yang` | YANG configuration: server listeners, auth mode, bearer-list identities, OAuth, TLS |

## Transport Shape

<!-- source: internal/component/mcp/streamable.go — ServeHTTP -->

| Method | Path | Body | Purpose |
|--------|------|------|---------|
| POST | `/mcp` | JSON-RPC 2.0 request | Client-to-server call. The response is a single `application/json` object |
| POST | `/mcp` | JSON-RPC 2.0 notification (no `id`) | Acknowledged with `202 Accepted` and no body. This revision defines no client-to-server notification on this transport, so nothing is dispatched |
| OPTIONS | `/mcp` | — | CORS preflight. Answers `204` with the allowed method and header sets |
| GET, DELETE | `/mcp` | — | `405 Method Not Allowed` with `Allow: POST, OPTIONS`. Earlier revisions used GET for the server-to-client SSE stream and DELETE for session termination. Neither exists here |
| GET | `/.well-known/oauth-protected-resource` | — | RFC 9728 protected resource metadata. Served before the Origin allowlist and without authentication. Answers `404` unless `auth-mode oauth` |

There is no handshake, no session, and no server-to-client stream. Every request
carries its own protocol version, client identity, and client capabilities, and
authenticates on its own.

## Request Pipeline

<!-- source: internal/component/mcp/streamable.go — handlePOST -->

`handlePOST` runs a fixed order, and the order is the contract. Header
validation is a transport-level guard that must run before dispatch. If it runs
later, the header/body confusion it exists to prevent is already possible.

| # | Step | Failure |
|---|------|---------|
| 1 | Content-Type guard (`application/json` when present) | `415 Unsupported Media Type` |
| 2 | Read the body under `http.MaxBytesReader` | `413 Request Entity Too Large` |
| 3 | Parse the JSON-RPC envelope | `-32700`, HTTP 200 |
| 4 | Validate the standard headers against the body | `-32020`, HTTP 400 |
| 5 | Parse `params._meta` | `-32602`, HTTP 400 |
| 6 | Check the declared version against the supported set | `-32022`, HTTP 400 |
| 7 | Authenticate | `401` with `WWW-Authenticate` |
| 8 | Acknowledge a notification (no `id`) | — (`202`) |
| 9 | Dispatch through `runMethod` | see below |

`httpStatusForDispatch` maps the two dispatch-time codes that carry a mandated
status: `-32601` becomes HTTP 404 (so a client can tell a modern server from a
legacy HTTP+SSE one that does not host the endpoint) and `-32021` becomes HTTP
400. Every other result, success or failure, rides a 200.

<!-- source: internal/component/mcp/streamable.go — httpStatusForDispatch -->

## Headers

<!-- source: internal/component/mcp/headers.go — validateStandardHeaders, mcpNameSource, decodeSentinel -->

| Header | Direction | Semantics |
|--------|-----------|-----------|
| `MCP-Protocol-Version` | Client -> server | Required on every POST. Must equal `params._meta["io.modelcontextprotocol/protocolVersion"]` when the body declares one. Absence and mismatch are the same verdict: there is no handshake to fall back on, so a missing header never defaults to a revision |
| `Mcp-Method` | Client -> server | Required on every POST. Must equal the body's `method`. Header *names* are matched case-insensitively. Header *values* are compared exactly |
| `Mcp-Name` | Client -> server | Required for `tools/call` and `prompts/get` (mirror `params.name`) and for `resources/read` (mirrors `params.uri`). Which field applies follows the request type: `CallToolRequest` and `GetPromptRequest` carry `name`, only `ReadResourceRequest` carries `uri`. Ze decodes the `=?base64?...?=` sentinel, then compares |
| `Mcp-Param-{Name}` | Client -> server | Ze annotates no tool parameter with `x-mcp-header`, so no `Mcp-Param-*` header has a body field to compare against. The character half of the rule is still enforced: a value carrying octets RFC 9110 does not permit in a field value is rejected |
| `Content-Type: application/json` | Client -> server (POST) | Required when present. CSRF guard: rejects `text/plain` form submissions from browsers |
| `Origin` | Client -> server | Validated against `StreamableConfig.AllowedOrigins`. An empty allowlist accepts only loopback-shaped origins. A non-matching origin is rejected with 403 first, before every other check on the endpoint path |
| `Authorization: Bearer <token>` | Client -> server | Checked on **every** request, per the configured auth mode |
| `Mcp-Session-Id`, `Last-Event-ID` | — | Not minted, not required, not echoed. Neither appears in the CORS allow-list or expose-list |

The `=?base64?` prefix and `?=` suffix are case-sensitive, and they must appear
exactly as the specification prints them. The payload is standard Base64
**with** padding, not base64url. A value not in sentinel form is used verbatim.

Header validation also refuses a body that is not a request at all. A POST whose
body carries no `method` is a JSON-RPC **response** or **error** frame. Ze
rejects it by name, with `-32020` and HTTP 400, and reads nothing else first.
This server writes no JSON-RPC request to a client (MCP 2026-07-28 replaced
server-initiated elicitation with Multi Round-Trip Requests, so it asks
nothing). An arriving response therefore answers a question that was never put,
and it can be correlated with nothing.

<!-- source: internal/component/mcp/headers.go — validateStandardHeaders, errBodyCarriesNoMethod -->
<!-- source: internal/component/mcp/mrtr.go — the MRTR pattern that replaced server-initiated requests -->

## Per-Request Metadata

<!-- source: internal/component/mcp/meta.go — parseRequestMeta, parseClientCapabilities -->

The `_meta` block sits **inside `params`**, not at the JSON-RPC message top
level.

| Key | Required | Consumed as |
|-----|----------|-------------|
| `io.modelcontextprotocol/protocolVersion` | Yes | `requestMeta.ProtocolVersion`, checked against the supported set |
| `io.modelcontextprotocol/clientCapabilities` | Yes | `requestMeta.Capabilities`. Send `{}` to declare none |
| `io.modelcontextprotocol/clientInfo` | No | `requestMeta.ClientInfo`. Self-reported and unverified, so it is carried for display and logging only and never reaches an authorization or ownership decision |

A request missing either required field is rejected with `-32602` and HTTP 400.
That is deliberately a different failure from a header mismatch, so a client can
tell "you sent the wrong header" from "you omitted a required field".

## Capability Negotiation

<!-- source: internal/component/mcp/meta.go — clientCapabilities, parseClientCapabilities -->
<!-- source: internal/component/mcp/streamable_tools.go — requestScope, failMissingTasksCapability -->

Capabilities are declared per request, not once per session. `clientCapabilities`
is a value type. Every field is a bool that stays false until the client declares
the capability. A handler that holds a zero value therefore denies rather than
serves. There is no pointer and no third "unknown" state: absence and
non-declaration are the same verdict.

| Field | Source member | Consumer |
|-------|---------------|----------|
| `Tasks` | `extensions["io.modelcontextprotocol/tasks"]: {}` only. The bare `tasks` member the earlier revision used is no longer accepted. `tasks` is not a `ClientCapabilities` member. A server that honored it would push an unsolicited task handle at a client that only agreed to the client-directed model | `tasks/get`, `tasks/update`, `tasks/cancel`, and whether `tools/call` can answer with a task handle at all |
| `ElicitForm` | `clientCapabilities.elicitation`, resolved for FORM mode | whether a handler that needs input can emit an `inputRequests` entry, or must return the missing-argument error |

`ElicitForm` is the one field that is not a presence check. The capability is
`elicitation?: { form?: JSONObject; url?: JSONObject; }`, and an empty object
means form mode only. The specification says "Servers **MUST NOT** send
elicitation requests with modes that are not supported by the client". A client
that declares `{"url":{}}` therefore supports elicitation and reads `ElicitForm`
false, because url mode is not implemented.

`elicitationFormSupported` (`mrtr.go`) owns that reading. An object naming only
unrecognized modes reads false, like every other non-declaration.

Only a capability the CLIENT supplies can gate a request. `resources`, `tools`
and `prompts` are members of `ServerCapabilities`, not of `ClientCapabilities`,
whose five members are `experimental`, `roots`, `sampling`, `elicitation` and
`extensions`. A conformant client can therefore never declare one, and a demand
for one would refuse every conformant caller.

A capability counts as declared only when its value is a JSON object. A null, a
bool, or a string under the same key is not a declaration. Ze ignores anything
unrecognized, which is what keeps the zero value fail-closed.

A request needing a capability its `_meta` did not declare is answered with
`-32021 MissingRequiredClientCapability` and HTTP 400, carrying
`data.requiredCapabilities` in the `ClientCapabilities` shape. This is not
`-32601`: an undeclared capability is a method the client cannot be served, not
a method the server does not have.

## Per-Request State

<!-- source: internal/component/mcp/streamable_tools.go — requestScope, runMethod -->

`requestScope` is the per-request protocol state every handler runs against:
the authenticated `Identity`, the declared `clientCapabilities`, the checked
`ProtocolVersion`, and the unverified `ClientInfo`. It is built once in
`handlePOST` after authentication and copied into every handler by value.

It is deliberately not a pointer and deliberately not optional. The compiler
forces each handler to receive an identity and a capability set, so there is no
nil case a handler can forget to guard. A zero `Identity` means "anonymous under
`auth-mode none`". It never means "not authenticated", because an
unauthenticated request is rejected before a scope exists.

The server therefore holds no per-client state at all. The task registry is the
one long-lived structure, and it is keyed by authenticated principal with its
own concurrency, retention, and TTL caps.

<!-- source: internal/component/mcp/streamable.go — Streamable struct, authenticate -->

## Methods

<!-- source: internal/component/mcp/streamable_tools.go — runMethod -->

| Method | Purpose |
|--------|---------|
| `server/discover` | Advertise supported versions, capabilities, and instructions |
| `tools/list` | The tool inventory, derived at call time |
| `tools/call` | Run a tool. Answers `resultType: "task"` when the command is annotated `ze:task-support required` and the request declared the tasks extension |
| `tasks/get`, `tasks/update`, `tasks/cancel` | Task lifecycle, scoped to the authenticated principal. `tasks/list` and `tasks/result` were removed this revision and now answer `-32601` |
| `resources/list`, `resources/read` | Embedded UI assets under `ui://` |

`server/discover`, `tools/list`, `resources/list` and `resources/read` carry
caching hints. `tools/call` and every `tasks/*` method carry none (see Cacheable
Results).

Any other method returns `-32601` with HTTP 404. `initialize` is recognized for
one reason: a client that still sends it receives a diagnostic naming the
protocol version this server does speak. The versioning page asks a modern-only
server to name its versions in any error it returns to an `initialize` request.

Which of the two rejections that client receives depends on whether it sent
conformant headers. A header-less legacy `initialize` is rejected by header
validation with `-32020`, before dispatch. One that somehow carried the headers
reaches `runMethod` and gets the `-32601` 404.

<!-- source: internal/component/mcp/headers.go — initializeEraError -->

## server/discover

<!-- source: internal/component/mcp/discover.go — serverDiscover, serverCapabilities, instructions -->

Servers must implement `server/discover`. A client can call it first, to learn
what the server speaks. The result carries:

| Field | Value |
|-------|-------|
| `supportedVersions` | A clone of `supportedProtocolVersions`, so it cannot drift from what the version check accepts |
| `capabilities` | `tools`, `resources`, and an `extensions` map naming BOTH `io.modelcontextprotocol/ui` and `io.modelcontextprotocol/tasks`, each with an empty settings object. Advertising tasks is what makes `resultType: "task"` interpretable: a client's legal ResultType set is the core set plus the values of extensions advertised via capabilities |
| `instructions` | Natural-language guidance. In Provider mode it names the provider. Otherwise it points an LLM at `ze_reference` first, then `ze_execute` |
| `resultType`, `_meta` | Stamped by the shared `ok()` helper, like every other result |
| `ttlMs`, `cacheScope` | `DiscoverResult` extends `CacheableResult`, where both are non-optional. Stamped from the method table, not by this handler (see Cacheable Results) |

The UI extension's settings object is empty. The specification says:

> an empty object indicates support with no additional settings

And the extension defines `mimeTypes` as something a CLIENT declares, to
constrain what it can render. A server-side settings member would assert a
schema the extension does not specify.

<!-- source: internal/component/mcp/apps.go -- extensionUI -->

## Cacheable Results

<!-- source: internal/component/mcp/caching.go -- cacheTTLByMethod, stampCacheHints, cacheScopePrivate -->

MCP 2026-07-28 requires caching hints on results with `resultType: "complete"`
returned by six operations. Ze implements four of them. `prompts/list` and
`resources/templates/list` are not dispatched at all. An unimplemented method
returns a JSON-RPC error rather than a result, so it carries no hints and
breaches nothing.

| Surface | `ttlMs` | Mutability class |
|---------|---------|------------------|
| `tools/list`, `server/discover` | `60000` (60 s) | Registry-derived: the command list is re-read from the dispatcher on every call, so it changes when a plugin registers or a config reload lands |
| `resources/list`, `resources/read` | `3600000` (1 h) | Embedded asset: `cachedResources` is built once at construction and the bytes come from `//go:embed`, so they are fixed for the binary's lifetime |

Both are compile-time constants with no YANG leaf and no env var. The right
lifetime is a function of how the underlying data changes. The code knows that,
and an operator does not. Ze can neither detect nor reject a setting that
contradicted the server's real invalidation behavior.

`cacheScope` is `"private"` on every cacheable result, unconditionally. Ze's tool
list is currently identical for every principal, so `"public"` would be accurate
today. But the whole endpoint sits behind authentication, and the specification
warns that a `"public"` result:

> may be shared outside of the initial requests authorization context

The auth modes already carry per-identity scopes, and `Identity.HasScope` is the
exact hook a scope-filtered tool list would use. One unconditional value
therefore removes a branch that a later change would have to get right.

The hints are applied from ONE site, on the way out of dispatch, driven by
`cacheTTLByMethod`. They are deliberately not folded into the shared `ok()`
responder. `tools/call` rides that responder and must carry no hints in either
result shape. Interim `input_required` results "are not cacheable and carry no
caching hints", and MRTR retries "MUST NOT be cached".

<!-- source: internal/component/mcp/streamable_tools.go -- runMethod -->

### Known limitation: the 60 s reload window

Without `subscriptions/listen` there is no push invalidation, so `ttlMs` is the
only lever Ze has. For up to 60 seconds after a config reload, a client can
still offer a command that the reload removed.

That window is a sanctioned mode, not a gap. The specification says that a
server:

> MAY provide `ttlMs` without advertising `listChanged: true`

In that case:

> the client relies entirely on TTL-based freshness

The window is also self-correcting. A call to a removed tool returns an error.
And clients:

> MAY re-fetch before the TTL expires if they have reason to believe the data
> has changed (e.g., receiving an unexpected error on a tool call indicating the
> method was not found)

The 60 s constant is the bound on that window. Ze chose it over the
specification example's 300000 for exactly this reason.

## Result Envelope

<!-- source: internal/component/mcp/streamable_tools.go — ok, resultMeta, serverInfo -->

Every successful result from every method flows through one `ok()` helper, which
stamps two envelope fields:

| Field | Value |
|-------|-------|
| `resultType` | `"complete"`, unless the handler already set `"input_required"` (the Multi Round-Trip interim result), which `ok()` preserves rather than overwrites. A guard on the single path out of `runMethod` refuses `"input_required"` on any method other than `prompts/get`, `resources/read` and `tools/call`. Those are the three the specification permits it on |
| `_meta["io.modelcontextprotocol/serverInfo"]` | `{name, version}`. The name is `ze-mcp`, or the provider's own name in Provider mode |

The caller's map is copied, not stamped in place. `tasks/get` returns the map
the registry stored (as a terminal task's `result`), and a mutation of that map
would persist envelope fields into registry state.

## Error Codes

<!-- source: internal/component/mcp/streamable_tools.go — rpc* constants, failUnsupportedVersion, failMissingTasksCapability -->

| Code | Name | HTTP | Meaning |
|------|------|------|---------|
| `-32700` | Parse error | 200 | The body is not JSON |
| `-32601` | Method not found | 404 | The server does not implement the method |
| `-32602` | Invalid params | 200, or 400 for a malformed `_meta` | Bad params, an unknown tool, a resource that does not exist, or a required `_meta` field missing |
| `-32020` | `HeaderMismatch` | 400 | A required standard header is missing, or disagrees with the body. Carries no `data` |
| `-32021` | `MissingRequiredClientCapability` | 400 | `data.requiredCapabilities` names what the client must declare |
| `-32022` | `UnsupportedProtocolVersion` | 400 | `data.supported` lists this server's versions, `data.requested` echoes the client's |

`-32020` messages name which header disagreed with which body field, and they
echo neither value. A rejection must not reflect unvalidated header bytes back
to the client, and the body value is client-supplied too.

## Authentication

<!-- source: internal/component/mcp/streamable.go — authenticate -->
<!-- source: internal/component/mcp/auth.go — authenticator interface, Identity -->

Authentication runs on **every** request. With the handshake gone there is no
session id to stand in for a credential. Each POST therefore presents its own
credential, and each POST is checked.

Two consequences follow. A revoked token stops working on the next request, not
at session expiry. And no long-lived identifier exists that is a bearer
credential in its own right to steal.

Four modes are selected by `environment.mcp.auth-mode`: `none`, `bearer`,
`bearer-list`, and `oauth`. `none` is not a bypass. It is an authenticator that
accepts every request with a zero `Identity`. That is why `ze-chaos`, which
configures no token and no auth mode, reaches the same uniform path as every
other caller. It is not a carve-out.

## Mount Point

<!-- source: cmd/ze/hub/service_mcp.go — startMCPServer (ze_mcp) -->

`startMCPServer(addrs, dispatch, commands, mcpCfg, tlsCert, tlsKey)` is called
from the gated `buildMCPService` factory when `environment.mcp.server` has at
least one entry. Each listener address gets its own `net.Listener`. All are
served by a single `http.Server` whose handler is the `*zemcp.Streamable`. Bind
is all-or-nothing: if any listener fails, the already-bound listeners are
closed. Shutdown calls `http.Server.Shutdown`, and the caller must also call
`Close` on the handler so the task-registry GC goroutine exits.

## Auto-Generated Tools

<!-- source: internal/component/mcp/tools.go — groupCommands, buildToolDef -->

`CommandLister` returns every registered CLI command (`CommandInfo`). The
tool generator groups by common prefix (`show bgp rib status`, `show bgp rib`
-> `show bgp` group with `rib status` and `rib` actions), synthesises a JSON
Schema from each command's YANG RPC metadata, and emits an MCP tool named
`ze_<prefix_joined_with_underscores>`. The handcrafted `ze_execute` tool is
a raw dispatch escape hatch, and `ze_reference` returns the machine-readable
daemon reference.

Tools are derived at every `tools/list` call, so newly registered commands
become available without any MCP code changes (rule: `derive-not-hardcode`).

A command that a plugin registers with `Hidden` never becomes a tool.
`buildCommandMeta` skips it, so the tool list and the completion tree hide the
same commands.

<!-- source: cmd/ze/hub/command_meta.go -- buildCommandMeta hidden plugin command skip -->

## The Tasks Extension

<!-- source: internal/component/mcp/tasks.go -- task registry -->
<!-- source: internal/component/mcp/streamable_tools.go -- createTask, tasks/* dispatch -->

Background tasks are the `io.modelcontextprotocol/tasks` extension, not core
protocol. Creation is **server-directed**: there is no `task` member on
`tools/call` params, and the client cannot opt a call into background execution.
The server decides per tool from the YANG `ze:task-support` annotation, and the
client declares once per request that it can read a task handle.

<!-- source: internal/component/mcp/streamable_tools.go -- callTool eligibility decision -->

| Annotation | Effect |
|------------|--------|
| `required` | Always returns a task handle, to a client that declared the extension |
| `forbidden` | Never returns a task handle. The call runs synchronously |
| `optional` (default) | The call runs synchronously |

A group that mixes levels resolves to `forbidden`. The eligibility decision is
made per TOOL, and a tool folds several commands into one group. The precedence
is therefore a promotion rule under server-directed semantics. A required-wins
rule would auto-task an action its YANG explicitly marked forbidden, and the
four annotated commands are the mutating rib ones.

A closed failure costs one long command that runs synchronously. An open failure
costs an auto-tasked route injection.

<!-- source: internal/component/mcp/tools.go -- groupTaskSupport -->

A client that did NOT declare the extension still gets its answer, synchronously
and with `resultType: "complete"`. The extension is an optimization over a
synchronous call, never a precondition for the work. A refusal would make the
annotated commands unreachable to any client that has not adopted an optional
extension.

The `CreateTaskResult` carries `resultType: "task"`, a cryptographically random
`taskId`, `status`, `ttlMs` and `pollIntervalMs`. The poll hint is derived as
`min(1 s, ttlMs/2)` rather than fixed, so a client obeying it always polls at
least twice inside the retention window. The entry is registered before the
response is written, so an immediate poll always finds it.

<!-- source: internal/component/mcp/tasks.go -- retentionHints -->

A worker goroutine runs the tool dispatch and transitions the task through the
state machine:

    working -> completed | failed | cancelled

Nothing is pushed to the client. This revision has no server-to-client stream on
this transport, so a client observes a task with a poll of `tasks/get`. The
output arrives there too: a terminal task carries `result` when it completed,
and `error` when it failed. That payload rule is what let the blocking
`tasks/result` method be deleted. Ze stores the result BEFORE the state goes
terminal. A poll that sees a terminal status can therefore always read the
payload from the same response.

<!-- source: internal/component/mcp/tasks.go -- runTaskWorker, TaskInfo.toWire -->

`input_required`, the extension's fifth state, is not implemented and
`TaskInputRequired` does not exist. The task worker is handed a zero capability
set, so a handler that would elicit takes the missing-argument path instead. No
task can raise `inputRequests`.

`tasks/update` is implemented in full anyway. For a server that raises no input
requests, the extension's rule is exactly this: verify ownership, acknowledge
empty, and ignore unknown keys.

<!-- source: internal/component/mcp/task_state.go -- TaskState, the deliberately absent state -->

Task identity scoping: each task is bound to the authenticated principal of the
request that created it. `tasks/get`, `tasks/update` and `tasks/cancel` reject a
cross-identity lookup with a uniform "not found" error. That error is identical
to the answer an id that never existed gets. A caller therefore cannot use the
reply to probe for another principal's ids. The registry is indexed by identity,
so the concurrency cap, every lookup, and the visibility rule are all per
principal. There is no enumeration surface at all now that `tasks/list` is gone.

Concurrency is bounded per identity (default 8). Terminal tasks are retained for
a configurable TTL (default 5 minutes, clamped to 1 second .. 1 hour) and then
garbage collected, with at most 128 retained.

Every worker also runs under a server-side execution deadline (default 10
minutes), past which the GC sweep forces the entry terminal and releases its
concurrency slot. The deadline is a liveness bound, not a retention one, and it
replaces what the deleted session reaper used to provide. The TTL sweep deletes
only entries that ALREADY reached a terminal state, so it cannot see a worker
that never returns. And a canceled worker context is not sufficient either,
because a wedged dispatch can miss the cancellation.

<!-- source: internal/component/mcp/tasks.go -- sweep, defaultTaskExecDeadline -->

## Resources Capability and the MCP Apps Extension

<!-- source: internal/component/mcp/resources.go -- resources/list, resources/read -->
<!-- source: internal/component/mcp/ui/embed.go -- embedded UI assets -->
<!-- source: internal/component/mcp/apps.go -- clientSupportsUIApps, gateUIMeta -->

The server advertises `resources: {}` in its `server/discover` capabilities and
accepts `resources/list` and `resources/read` from every caller. There is no
client-capability gate. `resources` is a `ServerCapabilities` member, so no
conformant client can declare it. A gate on it would refuse every conformant
caller, while `tools/list` advertised `_meta.ui.resourceUri` that points at the
very assets refused.

UI assets are embedded at compile time via `//go:embed` in
`internal/component/mcp/ui/embed.go`. `resources/list` derives the asset
list from `fs.WalkDir` over the embedded FS (no hardcoded list).
`resources/read` resolves `ui://` URIs to embedded files, validates
against path traversal, and returns content as `text` (for text MIME
types) or base64-encoded `blob` (for binary).

Tool descriptors carry `_meta.ui.resourceUri` (plus optional `permissions`
and `csp` fields) when the tool's YANG command group has a `ze:ui-resource`
extension. Those three key names are the extension's own, verbatim, and did not
change when MCP Apps became a first-class extension in 2026-07-28.

The first shipped UI bundle is `bgp-peer/` (peer status panel).

### Negotiating `_meta.ui`

MCP Apps is negotiated as the `io.modelcontextprotocol/ui` extension, advertised
by the server in `server/discover` and declared by a client in each request's
`clientCapabilities.extensions`. The declaration gates whether `_meta.ui` is
emitted at all:

| Client's declared `io.modelcontextprotocol/ui` settings | `_meta.ui` emitted? |
|----------------------------------------------------------|---------------------|
| Extension absent from `extensions` | No |
| `{}` (support, no settings) | Yes |
| Present with no `mimeTypes` key | Yes |
| `mimeTypes` holding a media type whose base type is `text/html` | Yes |
| `mimeTypes` present and holding no `text/html` base type | No |

Matching is on the base media type, with parameters stripped. A client that
declares bare `text/html` is therefore served exactly as one that declares
`text/html;profile=mcp-app`. The profile is a media-type parameter, and an exact
string match would refuse a host that renders Ze's bundle perfectly well.
Anything malformed answers no.

When the gate says no the descriptor is emitted **without** `_meta` and is
otherwise identical: the tool is still listed and still callable. That is the
"revert to core protocol behavior" branch of the specification's two permitted
fallbacks. A rejection of a whole `tools/list`, because the host cannot render
HTML panels, would break every non-Apps client for no benefit.

The gate is applied to the assembled tool list, not inside the descriptor
builder. One site therefore covers descriptors from both origins: the ones
generated from the command registry, and the ones a `ToolProvider` supplies. A
provider owns its descriptor maps, so nothing is edited in place.

A `ui://` resource is a **separate** question, and Ze serves it ungated. The
assets are non-secret embedded files behind an authenticated endpoint, so a
non-UI client can still fetch one. Only the metadata that advertises a panel is
negotiated.

## Security Model

- Default binding is `127.0.0.1`. `bind-remote` is the opt-in for anything else.
- Authentication runs on every request, in all four modes.
- Origin allowlist defaults to loopback-shaped origins. A bad Origin is 403.
- `application/json` content-type is required on POST to defeat browser
  `text/plain` form CSRF.
- 1 MiB request body cap via `http.MaxBytesReader`. It is the only per-request
  size bound the transport enforces, and `StreamableConfig.MaxBodyBytes` is its
  knob.
- OAuth 2.1 resource-server semantics: RFC 9728 metadata, audience-bound tokens,
  `WWW-Authenticate` on 401, and a per-identity bearer list as alternatives to
  the shared token.

There is no session cap, session TTL, or session lifetime bound, because there
is no session to bound. The bounds that survive are per request (body size) and
per principal (concurrent tasks, retained terminal tasks, task TTL).

## History

| Landed | Delivered |
|--------|-----------|
| MCP 1 | The Streamable HTTP transport |
| MCP 2 | Remote binding, OAuth 2.1, per-identity bearer list |
| MCP 3 | Server-initiated `elicitation/create` (the push model, deleted by the 2026-07-28 cutover) |
| 2026-2 | Multi Round-Trip Requests: elicitation returns as an `InputRequiredResult` value, with no `requestState` |
| MCP 4 | The original task surface: client-directed `tools/call`, the `tasks/*` methods, the task registry. Superseded by the extension cutover below |
| MCP 5 | Resources capability, `ui://` UI-resource scheme |
| 2026-4 | Cacheable results (SEP-2549) and MCP Apps as the `io.modelcontextprotocol/ui` extension (SEP-1865) |
| Gate | The `ze_mcp` compile-out feature gate |
