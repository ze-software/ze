# Spec: mcp-0-umbrella — MCP Protocol Modernization

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` — workflow rules
3. `docs/guide/mcp/overview.md`, `docs/guide/mcp/remote-access.md` — current MCP docs
4. `internal/component/mcp/handler.go`, `tools.go`, `schema/ze-mcp-conf.yang` — current impl
5. Sibling child specs (once written): `spec-mcp-1-streamable-http.md` .. `spec-mcp-5-apps.md`

## Task

Bring ze's MCP server from the minimal `2024-11-05` tools-only handshake to a modern,
remotely-reachable, authenticated, interactive MCP surface. Five functional areas:

| # | Area | Child spec |
|---|------|-----------|
| 1 | Streamable HTTP transport + protocol version `2025-06-18` | `spec-mcp-1-streamable-http.md` |
| 2 | Remote binding + OAuth 2.1 resource-server auth | `spec-mcp-2-remote-oauth.md` |
| 3 | Server-initiated elicitation (`elicitation/create`) | `spec-mcp-3-elicitation.md` |
| 4 | Task-augmented requests, durable task registry (`2025-11-25`) | `spec-mcp-4-tasks.md` |
| 5 | MCP Apps: UI resources with `ui://` scheme (ext, `2026-01-26`) | `spec-mcp-5-apps.md` |

The umbrella defines target state, cross-cutting concerns, phase ordering,
and the shared surfaces (YANG config, capabilities declaration, session
state). Child specs own their own ACs, TDD plan, and wiring.

Both the loopback-only guarantee (`docs/guide/mcp/remote-access.md` line 3)
and the single shared bearer token (YANG `token` leaf) are explicitly
superseded. Docs updated in Phase 2.

## Required Reading

### Architecture Docs

- [ ] `docs/guide/mcp/overview.md` — current MCP daemon surface, auth, tools
  → Constraint: bearer token auth today is a single shared secret; Phase 2 replaces it
  → Constraint: MCP JSON-RPC lives on one POST endpoint; Phase 1 adds GET+SSE on the same path
- [ ] `docs/guide/mcp/remote-access.md` — loopback-only rationale
  → Decision: Phase 2 adds an opt-in `bind-remote` flag; docs page rewritten to explain TLS + OAuth model rather than SSH tunnels
- [ ] `docs/architecture/core-design.md` — small-core + registration pattern
  → Constraint: MCP is a component under `internal/component/mcp/`; it registers on start via existing pattern, must not import any plugin directly
- [ ] `docs/architecture/api/commands.md` — command dispatch the MCP tools wrap
  → Constraint: tools auto-generate from `CommandLister` result; adding task-augmented `tools/call` MUST not break the existing unbundled `tools/call`

### MCP Spec Pages (external, no rfc/ summary today)

- [ ] `modelcontextprotocol.io/specification/2025-06-18/basic/transports` — Streamable HTTP
  → Constraint: single MCP endpoint handles POST (client-initiated) + GET (server-to-client stream); SSE response Content-Type is `text/event-stream`
  → Constraint: `Mcp-Session-Id` header assigned at init, required on subsequent calls; server may 404 expired sessions
  → Constraint: `MCP-Protocol-Version` header on every non-initialize request; invalid value → 400
  → Constraint: `Origin` header MUST be validated to defeat DNS-rebinding
- [ ] `modelcontextprotocol.io/specification/2025-06-18/basic/authorization` — OAuth 2.1 profile
  → Constraint: MCP server is an OAuth 2.1 resource server; 401 on missing/bad token; WWW-Authenticate names the protected-resource-metadata URL
  → Constraint: RFC 9728 `/.well-known/oauth-protected-resource` MUST be served; MUST declare `authorization_servers`
  → Constraint: token audience validation per RFC 8707; no token-passthrough
- [ ] `modelcontextprotocol.io/specification/2025-06-18/client/elicitation` — elicitation/create
  → Constraint: server-initiated request on the SSE stream opened by the originating client POST; response lands as client POST correlated by JSON-RPC id
  → Constraint: `requestedSchema` is restricted to flat primitives (string/number/integer/boolean/enum); no nested objects, no arrays of objects
  → Constraint: response `action` is one of `accept`/`decline`/`cancel`
- [ ] `modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks` — tasks primitive
  → Constraint: task status state machine `working` → `{input_required, completed, failed, cancelled}`; terminal states are sinks
  → Constraint: `tools/list` tool entries declare `execution.taskSupport` = `required`/`optional`/`forbidden`
  → Constraint: task IDs MUST be cryptographically secure when no auth context exists; TTL bounded
  → Constraint: `_meta.io.modelcontextprotocol/related-task` is the correlation key for all messages that belong to a task
- [ ] `modelcontextprotocol.io/extensions/apps/overview` — MCP Apps
  → Constraint: server returns tools with `_meta.ui.resourceUri` → `ui://` scheme; the HTML/CSS/JS lives as a resource the host fetches
  → Constraint: UI resource `_meta.ui` carries `permissions` + `csp`; server MUST serve content over existing resource primitives (resources capability)
  → Constraint: app ↔ host JSON-RPC dialect rides over postMessage between iframe and host — server never handles postMessage; server only SERVES the HTML

### RFC / Standards

- [ ] RFC 9728 — OAuth 2.0 Protected Resource Metadata
  → Constraint: metadata document at `/.well-known/oauth-protected-resource`; JSON body names authorization_servers, resource, scopes_supported
- [ ] RFC 8414 — OAuth 2.0 Authorization Server Metadata (client-facing concern; resource server merely points at the AS)
- [ ] RFC 8707 — Resource Indicators
  → Constraint: `resource` parameter must canonically identify this MCP endpoint; server rejects tokens with wrong `aud`
- [ ] RFC 2119 / 8174 — the usual MUST/SHOULD vocabulary

**Key insights:**
- Streamable HTTP is the only transport change that unlocks Phases 3–5. It is not optional.
- OAuth 2.1 is the MCP-preferred scheme; implementing it as resource server is bounded (we do NOT run an AS). Existing single-token mode remains for unauth/dev.
- Tasks are orthogonal to elicitation EXCEPT for `input_required` → the server MAY elicit while a task is in `input_required`. Capability table handles this cleanly: clients declare `tasks.requests.elicitation.create` if they support it.
- Apps are the narrowest addition protocol-wise: server advertises `resources` capability, tool descriptions carry `_meta.ui.resourceUri`, UI resources are served through `resources/read`. The iframe / sandbox work is entirely client-side.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/mcp/handler.go` (307 LOC) — single HTTP POST endpoint; JSON-RPC 2.0 req/resp; four methods (`initialize`, `notifications/initialized`, `tools/list`, `tools/call`); constant-time bearer compare; CSRF guard via `application/json`
- [ ] `internal/component/mcp/tools.go` (415 LOC) — tools auto-derived from `CommandLister` return value; depth-1 and depth-2 grouping; typed YANG params surfaced as JSON Schema properties; handcrafted `ze_execute` escape hatch
- [ ] `internal/component/mcp/schema/ze-mcp-conf.yang` — single `server` list entry, `ip` defaulted to `127.0.0.1`, `port` defaulted to `8080`, `token` leaf (sensitive, optional)
- [ ] `internal/component/mcp/tools_test.go` — auto-generation tests: prefix grouping, conflict with handcrafted names, typed YANG params, invalid-action rejection

**Behavior to preserve:**
- Auto-generated tools derived from command registry at each `tools/list` call (no cache, always current)
- Handcrafted `ze_execute` remains for arbitrary dispatch
- Constant-time bearer compare preserved as one of the supported auth modes (see Phase 2 table)
- `application/json` Content-Type CSRF guard preserved on all POST endpoints
- Existing YANG `mcp.server[].ip` + `mcp.server[].port` shape preserved; new leaves added, existing leaves untouched
- Existing handler mount point (`Handler` returning `http.Handler`) stays as the public surface; internals refactor behind it

**Behavior to change:**
- Protocol version advertised in `initialize` bumps from `2024-11-05` to `2025-06-18` (Phase 1). Task-augmented requests require the `2025-11-25` version (Phase 4) which is negotiated separately via capability declaration.
- Loopback-only binding is no longer hard-coded. Operator opt-in via YANG; default remains loopback.
- Single shared token is no longer the only auth mode. Bearer-per-identity table and OAuth 2.1 resource-server mode added.

## Data Flow (MANDATORY)

### Entry Point (current)

Operator config `environment.mcp.server[]` → reactor mounts `mcp.Handler()` on the configured listener → client HTTP POST with JSON-RPC body → `handler.go` dispatcher → tool implementation (either `ze_execute` or auto-generated → `dispatch(cmd)` into reactor).

### Entry Point (target)

Operator config `environment.mcp.server[]` + new leaves (`auth-mode`, `oauth`, `bind-remote`, `tls`) → reactor mounts Streamable HTTP handler → three paths:

| Path | Source | Target |
|------|--------|--------|
| POST `/mcp` with JSON-RPC request | Client-initiated request | SSE stream (if the response requires server-initiated sub-requests or task progress) or single JSON response |
| GET `/mcp` with `Accept: text/event-stream` | Client asking for a push channel | SSE stream (server-initiated requests / notifications only) |
| GET `/.well-known/oauth-protected-resource` | Any client | RFC 9728 metadata JSON |

### Transformation Path

1. Request enters HTTP layer — Origin + Content-Type + MCP-Protocol-Version validation; fast-reject before any JSON parse.
2. Auth layer — one of: `none` (localhost + no token), `bearer` (single shared token, legacy), `bearer-list` (per-identity token table), `oauth` (validate Bearer against AS + audience binding).
3. Session layer — `Mcp-Session-Id` header → session registry lookup. Create on initialize, expire on TTL, 404 on missing.
4. Method dispatch — same shape as today (switch on `method`), expanded for `elicitation/create` (server-initiated, not dispatched here), `tasks/{list,get,result,cancel}`, `resources/{list,read}`.
5. For task-augmented `tools/call`: enqueue task in task registry, return `CreateTaskResult`; worker goroutine runs dispatch; status updates published to session's SSE stream.
6. For elicitation mid-tool-call: server sends `elicitation/create` on the originating stream; blocks pending session-scoped channel; resumes when matching client POST response lands.

### Boundaries Crossed

| Boundary | How | Verified (per phase) |
|----------|-----|----------------------|
| HTTP layer ↔ JSON-RPC layer | Raw body → parsed `request` struct | [ ] Phase 1 |
| JSON-RPC layer ↔ session registry | `Mcp-Session-Id` header → session object | [ ] Phase 1 |
| Session ↔ SSE sender | `session.Send(message)` → stream frame | [ ] Phase 1 |
| Auth layer ↔ rest | `Authorization: Bearer …` validated once, identity attached to session | [ ] Phase 2 |
| Task registry ↔ worker goroutine | Enqueue create → worker runs dispatch → pub to session stream | [ ] Phase 4 |
| Elicitation dispatch ↔ session stream | Server writes server-initiated request, blocks on client response channel | [ ] Phase 3 |
| Resources capability ↔ UI resource store | `resources/read` for `ui://…` → bundled asset fetch | [ ] Phase 5 |

### Integration Points

- `reactor.ExecuteCommand` — unchanged; still the sole dispatch into the engine
- `CommandLister` — unchanged; tool generation layer extended to attach `execution.taskSupport` in Phase 4
- Web component listener plumbing — MCP continues to mount its own handler on its own port; it does NOT share the web component's HTTP mux. Listener setup code in `cmd/ze` references today are preserved.

### Architectural Verification

- [ ] No bypassed layers — dispatcher goes through session registry before touching tools
- [ ] No unintended coupling — MCP remains a component; no new imports from `internal/plugin/` or `internal/component/web/`
- [ ] No duplicated functionality — SSE sender is the first SSE sender in MCP, but the lg / web components have their own; we do not share one (different domain, different session model, different framing)
- [ ] Buffer-first where applicable — JSON-RPC layer is not wire-encoding; buffer-first rule does not apply. JSON marshaling via `encoding/json` remains allowed per `rules/json-format.md`

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| POST /mcp with `MCP-Protocol-Version: 2025-06-18` | → | `streamable.POST()` returns SSE stream for request requiring sub-messages | `test/mcp/streamable-post-sse.ci` |
| GET /mcp with `Accept: text/event-stream` | → | `streamable.GET()` opens server-to-client stream | `test/mcp/streamable-get-stream.ci` |
| MCP request without token against `auth-mode=oauth` | → | 401 with WWW-Authenticate header | `test/mcp/oauth-401.ci` |
| GET /.well-known/oauth-protected-resource | → | RFC 9728 metadata JSON returned | `test/mcp/oauth-metadata.ci` |
| tools/call on a tool that requires elicitation | → | server sends `elicitation/create`; client replies; tool completes | `test/mcp/elicitation-accept.ci` |
| tools/call with `task: {}` against a task-capable tool | → | `CreateTaskResult` returned immediately; `tasks/get` polls to `completed`; `tasks/result` returns CallToolResult | `test/mcp/task-tools-call.ci` |
| tools/list where a tool has `_meta.ui.resourceUri=ui://panel.html` | → | `resources/read` on `ui://panel.html` returns HTML with `_meta.ui.permissions` | `test/mcp/apps-ui-resource.ci` |

Phase-level ACs decompose into child-spec ACs. Child specs own the per-phase AC tables and TDD plan.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Owning phase |
|-------|-------------------|-------------------|--------------|
| AC-1 | Client sends `initialize` with protocolVersion `2025-06-18` | Server echoes `2025-06-18`, issues `Mcp-Session-Id` header, returns updated capabilities declaration | 1 |
| AC-2 | Client POST with missing `MCP-Protocol-Version` header (post-initialize) | Server assumes `2025-03-26` per spec; or 400 if version invalid | 1 |
| AC-3 | Client POST a request where the server wants to stream notifications | Response Content-Type = `text/event-stream`, terminal frame = final JSON-RPC response | 1 |
| AC-4 | Client GET /mcp with `Accept: text/event-stream` and a valid session | SSE stream opens and remains until server or client closes | 1 |
| AC-5 | Client POST with `Origin: http://evil.example.com` | 403 (Origin not allowed) | 1 |
| AC-6 | Operator sets `environment.mcp.server.bind-remote true` + `auth-mode oauth` without `oauth.authorization-server` | `ze config verify` fails with a specific message | 2 |
| AC-7 | Remote client POST without Bearer against `auth-mode oauth` | 401 with `WWW-Authenticate: Bearer resource_metadata="https://…"` | 2 |
| AC-8 | GET /.well-known/oauth-protected-resource | JSON per RFC 9728 including `authorization_servers` and `resource` | 2 |
| AC-9 | Token whose `aud` is not this MCP endpoint | 401 (audience mismatch) | 2 |
| AC-10 | `auth-mode bearer-list` + invalid identity token | 401 | 2 |
| AC-11 | `auth-mode bearer-list` + valid identity token | Request proceeds; `identity.name` attached to session | 2 |
| AC-12 | Tool handler calls `session.Elicit()` mid-dispatch | Client receives `elicitation/create` on the originating POST's SSE stream | 3 |
| AC-13 | Client replies `{"action":"accept","content":{…}}` | Tool handler `Elicit()` returns parsed content; tool completes | 3 |
| AC-14 | Client replies `{"action":"decline"}` | Tool handler `Elicit()` returns typed "declined" sentinel; handler chooses fallback | 3 |
| AC-15 | Elicitation requestedSchema contains nested objects | Rejected at registration time (server-side sanity) | 3 |
| AC-16 | Client posts `tools/call` with `task: {"ttl": 60000}` against a task-capable tool | Server returns `CreateTaskResult` with status `working`; task registered | 4 |
| AC-17 | Client posts `tasks/get` for a `working` task | Task state returned, still `working` or `completed` | 4 |
| AC-18 | Client posts `tasks/result` for a `completed` task | Returns the underlying `CallToolResult` (same shape as unbundled `tools/call`) | 4 |
| AC-19 | Client posts `tasks/cancel` for a `working` task | Status transitions to `cancelled`; worker goroutine signalled | 4 |
| AC-20 | Task exceeds TTL | Server garbage-collects task; subsequent `tasks/get` returns `-32602` with "expired" | 4 |
| AC-21 | `tasks/list` under auth context A does not return tasks owned by context B | List scoped to caller identity | 4 |
| AC-22 | `tools/list` includes a tool with `_meta.ui.resourceUri: "ui://dashboard.html"` | Client can `resources/read` that URI and receive HTML + `_meta.ui` metadata | 5 |
| AC-23 | UI resource is served with declared `csp` in `_meta.ui.csp` | Resource content and metadata reach the client; UI metadata payload is well-formed | 5 |
| AC-24 | Concurrent sessions: two clients each initialize, each gets a unique `Mcp-Session-Id` | Sessions are isolated; one's requests never leak to the other | 1 |

## 🧪 TDD Test Plan

### Unit Tests (phase-owned; listed here for cross-reference)

| Test | File | Validates | Phase |
|------|------|-----------|-------|
| `TestStreamableHTTP_InitializeSession` | `internal/component/mcp/streamable_test.go` | AC-1, AC-24 | 1 |
| `TestStreamableHTTP_ProtocolVersionGate` | `…/streamable_test.go` | AC-2 | 1 |
| `TestStreamableHTTP_POSTUpgradesToSSE` | `…/streamable_test.go` | AC-3 | 1 |
| `TestStreamableHTTP_GETServerStream` | `…/streamable_test.go` | AC-4 | 1 |
| `TestStreamableHTTP_OriginValidation` | `…/streamable_test.go` | AC-5 | 1 |
| `TestOAuth_ConfigValidate` | `…/oauth_test.go` | AC-6 | 2 |
| `TestOAuth_401Challenge` | `…/oauth_test.go` | AC-7 | 2 |
| `TestOAuth_ResourceMetadata` | `…/oauth_test.go` | AC-8 | 2 |
| `TestOAuth_AudienceValidation` | `…/oauth_test.go` | AC-9 | 2 |
| `TestBearerList_ValidAndInvalid` | `…/auth_test.go` | AC-10, AC-11 | 2 |
| `TestElicitation_SchemaRejectNested` | `…/elicit_test.go` | AC-15 | 3 |
| `TestElicitation_AcceptDeclineCancel` | `…/elicit_test.go` | AC-12–14 | 3 |
| `TestTasks_CreateGetResult` | `…/tasks_test.go` | AC-16–18 | 4 |
| `TestTasks_CancelAndTTL` | `…/tasks_test.go` | AC-19, AC-20 | 4 |
| `TestTasks_AuthContextScoping` | `…/tasks_test.go` | AC-21 | 4 |
| `TestApps_ToolMetaUIResourceURI` | `…/apps_test.go` | AC-22 | 5 |
| `TestApps_UIResourceMetadata` | `…/apps_test.go` | AC-23 | 5 |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| task ttl (ms) | 1 000 – 86 400 000 (configurable bound) | 86 400 000 | 999 | 86 400 001 |
| session TTL (s) | 60 – 86 400 | 86 400 | 59 | 86 400 + 1 |
| max concurrent tasks per identity | 1 – configurable | config-driven | 0 | over-limit |
| Mcp-Session-Id length | 22 – 128 ASCII | 128 | 21 | 129 |

### Functional Tests

Each row in the Wiring Test table is a BLOCKING functional test per `rules/integration-completeness.md`. Listed here for `/ze-implement` stage tracking:

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-mcp-streamable-post-sse` | `test/mcp/streamable-post-sse.ci` | Client POSTs a request that yields an SSE stream; terminal frame is the JSON-RPC response | — |
| `test-mcp-streamable-get-stream` | `test/mcp/streamable-get-stream.ci` | Client GETs the MCP endpoint and holds an SSE stream for server-pushed messages | — |
| `test-mcp-oauth-401` | `test/mcp/oauth-401.ci` | Missing/invalid Bearer → 401 with RFC 9728 `WWW-Authenticate` header | — |
| `test-mcp-oauth-metadata` | `test/mcp/oauth-metadata.ci` | `.well-known/oauth-protected-resource` returns JSON per RFC 9728 | — |
| `test-mcp-elicitation-accept` | `test/mcp/elicitation-accept.ci` | Tool handler elicits; client accepts; tool returns with user content | — |
| `test-mcp-task-tools-call` | `test/mcp/task-tools-call.ci` | `tools/call` with `task:{}` → `CreateTaskResult` → `tasks/get` poll → `tasks/result` returns tool output | — |
| `test-mcp-apps-ui-resource` | `test/mcp/apps-ui-resource.ci` | Tool with `_meta.ui.resourceUri`; `resources/read` on that URI yields HTML + `_meta.ui` | — |

## Files to Modify

- `internal/component/mcp/handler.go` — split: transport moves to `streamable.go`, dispatch table stays but grows
- `internal/component/mcp/tools.go` — add `taskSupport` annotation, add UI-resource reference for tools that declare UIs
- `internal/component/mcp/schema/ze-mcp-conf.yang` — add auth-mode, oauth block, bind-remote, tls block, per-identity bearer list, task limits, session TTL
- `cmd/ze/*/main.go` (wherever MCP lands) — new flags if any (Phase 2 may add `--mcp-bind-remote`, `--mcp-oauth-as`, `--mcp-auth-mode`)
- `docs/guide/mcp/overview.md` — capability matrix, new auth modes, new transport shape
- `docs/guide/mcp/remote-access.md` — rewrite to explain supported remote modes rather than "SSH only"

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaves + RPCs: none — MCP is external) | [ ] | `internal/component/mcp/schema/ze-mcp-conf.yang` |
| Env vars for each new YANG `environment/mcp/…` leaf | [ ] | `internal/core/env/*` via `env.MustRegister` |
| CLI commands/flags | [ ] | `cmd/ze/…` |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test per new capability | [ ] | `test/mcp/*.ci` (new subtree) |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (add MCP 2025-06-18 + tasks + apps + OAuth) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (MCP block), `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | Maybe | `docs/guide/command-reference.md` (if new `ze mcp …` surface) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (MCP dispatch table now has tasks + resources) |
| 5 | Plugin added/changed? | No | — |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/overview.md`, `docs/guide/mcp/remote-access.md` rewritten; new `docs/guide/mcp/oauth.md`, `docs/guide/mcp/elicitation.md`, `docs/guide/mcp/tasks.md`, `docs/guide/mcp/apps.md` |
| 7 | Wire format changed? | No (MCP is not BGP wire) | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc9728.md`, `rfc/short/rfc8707.md`, `rfc/short/rfc8414.md` — create summaries |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` — add the `test/mcp/` subtree and runner notes |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (MCP is a ze differentiator; update row if present) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` mentions MCP — update if MCP component is now larger; add `docs/architecture/mcp/overview.md` |

## Files to Create

Child specs, each with their own template:

- `plan/spec-mcp-1-streamable-http.md`
- `plan/spec-mcp-2-remote-oauth.md`
- `plan/spec-mcp-3-elicitation.md`
- `plan/spec-mcp-4-tasks.md`
- `plan/spec-mcp-5-apps.md`

New source files (anticipated; each child spec confirms):

- `internal/component/mcp/streamable.go` — Streamable HTTP dispatcher, session registry, SSE writer
- `internal/component/mcp/session.go` — session state, correlation map for server-initiated requests, identity binding
- `internal/component/mcp/oauth.go` — RFC 9728 metadata handler, Bearer validation, audience check
- `internal/component/mcp/auth.go` — unified auth dispatch (none / bearer / bearer-list / oauth)
- `internal/component/mcp/elicit.go` — `session.Elicit()` helper and server-side schema sanity
- `internal/component/mcp/tasks.go` — task registry, state machine, TTL GC, worker orchestration
- `internal/component/mcp/tasks_store.go` — in-memory task store with auth-context scoping
- `internal/component/mcp/resources.go` — MCP resources capability handler (enables UI resources for apps)
- `internal/component/mcp/apps.go` — UI resource bundling (`ui://` scheme resolver) and tool `_meta.ui` attachment
- Corresponding `_test.go` files
- `test/mcp/*.ci` — functional tests per wiring table

## Implementation Steps

Each phase below is its own child spec. Umbrella tracks ordering and hand-off.

| Phase | Child spec | Depends on | Delivers |
|-------|-----------|-----------|----------|
| 1 | `spec-mcp-1-streamable-http.md` | — | Streamable HTTP transport, session registry, protocol-version bump, SSE sender, Origin validation |
| 2 | `spec-mcp-2-remote-oauth.md` | 1 | YANG remote + auth-mode, OAuth 2.1 resource server, RFC 9728 metadata, bearer-list auth |
| 3 | `spec-mcp-3-elicitation.md` | 1 | `session.Elicit()` API; client elicitation capability negotiation; server-side schema sanity |
| 4 | `spec-mcp-4-tasks.md` | 1, 3 (input_required case) | Task registry, `tasks/*` methods, `tools/call` task-augmentation, TTL GC, auth scoping |
| 5 | `spec-mcp-5-apps.md` | 1 | Resources capability; `ui://` resolver; tool `_meta.ui.resourceUri` plumbing |

### /implement Stage Mapping

Umbrella does NOT go through `/implement` — child specs do. Umbrella is the
contract that keeps them consistent. When a child spec's `/implement` stage 2
(audit) runs, it cross-references this umbrella to confirm no cross-phase
surface was broken.

### Implementation Phases (umbrella-level)

1. **Phase 1 — Streamable HTTP** (`spec-mcp-1-streamable-http.md`)
   - Replace pure request/response handler with POST+GET endpoint
   - Add session registry with `Mcp-Session-Id` header
   - Add SSE writer with per-session message queue
   - Bump protocol version to `2025-06-18`; capability declaration updated
   - Origin header validation
2. **Phase 2 — Remote + OAuth** (`spec-mcp-2-remote-oauth.md`)
   - YANG leaves: `bind-remote`, `auth-mode`, `oauth.{authorization-server, audience, required-scopes}`, `tls.{cert, key}`, `identity[]` (per-identity bearer list)
   - `/.well-known/oauth-protected-resource` handler (RFC 9728)
   - Bearer validation (local JWT introspection against AS metadata)
   - Audience binding check per RFC 8707
   - Update `docs/guide/mcp/remote-access.md`
3. **Phase 3 — Elicitation** (`spec-mcp-3-elicitation.md`)
   - `session.Elicit(ctx, message, schema)` handler API
   - Negotiation: server honors `elicitation/create` only if client declared `elicitation: {}`
   - Server-side sanity on `requestedSchema` (flat primitives only)
   - Correlation on JSON-RPC id; response routed from client POST back to suspended handler
4. **Phase 4 — Tasks** (`spec-mcp-4-tasks.md`)
   - Task registry with auth-context scoping (Phase 2 identity plumbing required)
   - `tools/call` augmentation: `params.task` → `CreateTaskResult` return; worker goroutine runs dispatch
   - `tasks/list`, `tasks/get`, `tasks/result`, `tasks/cancel` methods
   - `notifications/tasks/status` on session stream
   - `execution.taskSupport` declared per tool (drive from YANG command metadata; `optional` by default, `required` for long-running commands)
5. **Phase 5 — Apps** (`spec-mcp-5-apps.md`)
   - Resources capability in `initialize` response
   - `resources/list`, `resources/read` methods
   - `ui://<tool>.html` resolver served from an embedded `fs.FS` in `internal/component/mcp/ui/`
   - Tool descriptors carry `_meta.ui.resourceUri` when the tool has a UI bundle

### Critical Review Checklist (umbrella)

| Check | What to verify across phases |
|-------|------------------------------|
| Completeness | Every AC-N has at least one phase that owns it |
| Correctness | `2025-06-18` handshake compatible with `2024-11-05` clients? Decide: reject with 400 after Phase 1, or keep legacy branch. Recorded in Phase 1 spec |
| Naming | YANG leaves use kebab-case; JSON keys use camelCase (MCP is an external spec; kebab-case rule exempts the MCP dialect) |
| Data flow | Auth identity flows through session registry; task registry uses identity as scope key |
| Rule: no-layering | Old bearer-only code path DELETED when auth-mode dispatcher lands (no feature flag toggling between them) |
| Rule: exact-or-reject | `auth-mode=oauth` without `oauth.authorization-server` rejects at verify (AC-6) |
| Rule: derive-not-hardcode | Tool list in `tools/list` always derived from `CommandLister`; UI resource bundle map likewise derived from embedded FS walk |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Five child specs exist | `ls plan/spec-mcp-*.md` |
| Protocol-version bump | `grep '2025-06-18' internal/component/mcp/handler.go` (or successor) |
| RFC 9728 well-known handler | `curl http://localhost:8080/.well-known/oauth-protected-resource` in a `.ci` test |
| Elicitation via `session.Elicit` | Grep `session.Elicit(` at an existing tool handler |
| Task registry | `ls internal/component/mcp/tasks.go` |
| UI resource served | `.ci` test reading a `ui://` resource via `resources/read` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | JSON-RPC body size bounded (still `maxRequestBody`); SSE stream size limits; Origin allowlist; TLS-strict when `auth-mode oauth`; reject HTTP (non-TLS) in `oauth` mode |
| Token leakage | Access tokens never logged; redacted in debug output; `log/slog` handlers scrub `Authorization:` |
| Audience binding | AC-9 (reject token whose `aud` is wrong) — must have a unit test AND a functional test |
| CSRF | `application/json` guard preserved; Origin allowlist in play for GET SSE too |
| DNS rebinding | Origin validation on all methods (MCP spec explicit warning) |
| Task ID guessing | Cryptographically secure task IDs (crypto/rand, ≥128 bits); shorter TTL when no auth context |
| Rate limiting | Per-identity limit on task creation and elicitation dispatch; 429 when exceeded |
| Resource exhaustion | Session TTL + max concurrent sessions; per-session queue size bound; UI resource max size |

### Failure Routing

Same as template. Phase-level failures escalate to umbrella when they touch
cross-cutting surfaces (auth table shape, session state contract, task
registry API). Phase 1 must land cleanly before Phase 3/4 start; Phase 2
must land before Phase 4 uses auth identity.

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| "Tasks are not in MCP spec" | Tasks are experimental in `2025-11-25` | WebFetch against spec site | Corrected before any code written; umbrella updated |
| "Apps are not in MCP spec" | Apps are an extension at `2026-01-26` | WebFetch against spec site | Corrected before any code written; umbrella updated |

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Implement all five features as one spec | Scope would force partial wiring; `make ze-verify` fails mid-turn | Umbrella + five child specs; land per-phase |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Claiming a spec feature "doesn't exist" without fetching the current spec | 2× in a single conversation | Add to `rules/design-context.md` a line: "External-protocol claims require a fresh WebFetch, never memory" | File once umbrella commits |

## Design Insights

- MCP is the first ze component with server-initiated requests. Every other JSON path in ze is request/response. The session registry IS the new abstraction — a long-lived, identity-tagged, ID-correlated state machine. It belongs in `internal/component/mcp/session.go`, not in the transport layer.
- Auth identity is a value-type carried on the session. It flows through the tasks registry (scoping key) and into elicitation (audit log). No pointers across component seams; per `rules/enum-over-string.md` the auth-mode enum is a typed `uint8`.
- The `2025-06-18` vs `2025-11-25` split forces a capability table, not a global version bump. We advertise `2025-06-18` in initialize but accept tasks-augmented requests when the client also declared `tasks.*` capability. Matches spec intent.
- MCP-JSON is not ze-JSON. MCP uses camelCase (`protocolVersion`, `taskId`); `rules/json-format.md` exempts external specs. Document the exemption explicitly in Phase 1's spec.

## RFC Documentation

Phase 2 adds RFC references inline at the handlers. Phase 1 adds protocol-version and origin-check references. Summaries to be created under `rfc/short/`:

- `rfc/short/rfc9728.md` — OAuth 2.0 Protected Resource Metadata
- `rfc/short/rfc8707.md` — Resource Indicators for OAuth 2.0
- `rfc/short/rfc8414.md` — OAuth 2.0 Authorization Server Metadata (client-facing, short)

## Implementation Summary

_To be filled as phases complete; each phase lands its own summary in `plan/learned/NNN-mcp-<phase>-<name>.md`. Umbrella summary lands after Phase 5 as `plan/learned/NNN-mcp-0-umbrella.md`._

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Remote binding | - | (Phase 2) | — |
| Proper authorization | - | (Phase 2) | OAuth 2.1 + bearer-list |
| Elicitation | - | (Phase 3) | — |
| Tasks | - | (Phase 4) | — |
| Apps | - | (Phase 5) | — |
| Streamable HTTP (prerequisite) | - | (Phase 1) | — |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 .. AC-24 | - | (phase-owned) | Each child spec fills this row |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (all) | - | (phase-owned) | — |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| child specs 1–5 | - | Phase 1 child lands in the same turn as this umbrella |

### Audit Summary
- **Total items:** 24 ACs + 5 phases + documentation rows
- **Done:** 0
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| — | — | (umbrella has no code yet; `/ze-review` run after Phase 1 lands) | — | — |

### Fixes applied
- None (pre-implementation)

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE (after each phase)
- [ ] All NOTEs recorded in phase specs

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `plan/spec-mcp-0-umbrella.md` | [ ] | ls will be run in Phase 1 completion |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| — | (umbrella has no verifiable ACs of its own; all delegate to phase specs) | — |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| (phase-owned) | `test/mcp/*.ci` | [ ] |

## Checklist

### Goal Gates

- [ ] All five child specs written
- [ ] All five phases land with `/ze-review` clean
- [ ] `make ze-verify-fast` passes after each phase
- [ ] Documentation updated per checklist
- [ ] Learned summary per phase + umbrella summary after Phase 5

### Quality Gates

- [ ] RFC 9728, 8707, 8414 summaries created
- [ ] Implementation Audit complete per phase
- [ ] Mistake Log promoted if a rule candidate emerges

### Design

- [ ] No premature abstraction (each phase delivers usable code standalone)
- [ ] No speculative features
- [ ] Single responsibility per new file
- [ ] Minimal coupling (MCP component stays isolated from plugins + web)

### TDD

- [ ] Tests written before code in each phase
- [ ] Tests FAIL (paste output per phase)
- [ ] Tests PASS (paste output per phase)
- [ ] Boundary tests per TDD Plan
- [ ] `.ci` functional tests per Wiring table
- [ ] `make ze-test` passes at end of each phase
