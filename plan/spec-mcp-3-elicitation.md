# Spec: mcp-3-elicitation — Server-Initiated Elicitation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/9 |
| Updated | 2026-04-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` — workflow rules
3. `plan/spec-mcp-0-umbrella.md` — umbrella; Phase 3 owns AC-12..AC-15
4. `plan/learned/636-mcp-1-streamable-http.md` — P1 (transport, session registry, SSE)
5. `plan/learned/638-mcp-2-remote-oauth.md` — P2 (Identity value type, session.Create)
6. `internal/component/mcp/session.go`, `streamable.go`, `handler.go`, `tools.go`, `auth.go`
7. `docs/architecture/mcp/overview.md` — transport shape, header semantics, roadmap
8. `modelcontextprotocol.io/specification/2025-06-18/client/elicitation` — authoritative

## Task

Deliver MCP 2025-06-18 server-initiated elicitation: a tool handler calls
`session.Elicit(ctx, message, schema)` mid-dispatch, the server sends an
`elicitation/create` request down the originating POST's SSE stream (upgrading
the POST response from `application/json` to `text/event-stream` on demand),
the client POSTs a JSON-RPC response whose id matches, and the suspended
handler resumes with the parsed content or a typed decline/cancel sentinel.

Phase 3 also ships **one concrete elicitor** so the wiring test exercises a
real user entry point rather than a synthetic harness
(`rules/integration-completeness.md`): the handcrafted `ze_execute` tool
grows a branch where a missing `command` argument triggers an elicitation
asking the client to provide it.

Scope: AC-12..AC-15 from the umbrella (with AC-15 phrasing corrected from
"registration time" to "call-site sanity"), plus AC-15a/b/c covering
gaps the umbrella did not name, plus the one-elicitor wiring so the `.ci`
functional tests have something real to call.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/mcp/overview.md` — transport shape, headers, session lifecycle, Phase 3 roadmap row
  → Constraint: POST response body may be `application/json` OR `text/event-stream`; server picks based on whether sub-messages are emitted during the call
  → Constraint: GET SSE stream exists independently; server-initiated notifications also ride it. Phase 3 adds a SECOND SSE surface scoped to the originating POST
  → Decision: Phase 3 updates this doc to add a "POST may upgrade mid-call" row under Transport Shape

- [ ] `plan/spec-mcp-0-umbrella.md` — cross-phase contract; AC-12..AC-15
  → Constraint: capability declaration is `elicitation: {}` on client's initialize; server MUST NOT emit `elicitation/create` without it
  → Constraint: `requestedSchema` is flat primitives only (string/number/integer/boolean/enum with formats)
  → Decision: AC-15 phrasing corrected from "registration time (server-side sanity)" to "call-site sanity at `session.Elicit()`"; elicitation schemas are per-call, not pre-registered
  → Decision: three new ACs (AC-15a/b/c) added to cover capability gate, unknown-id response, and ctx-cancel cleanup — gaps in umbrella

- [ ] `plan/learned/636-mcp-1-streamable-http.md` — P1 landed session registry + SSE
  → Constraint: `session.sendMu` already serialises producers (elicitation, tasks, transport); Send uses len/cap pre-check, no `default:` in select (blocked by `block-silent-ignore.sh`)
  → Constraint: `session.outbound chan []byte` is the long-lived GET-stream queue; P3 does NOT reuse it for POST-upgrade traffic (different lifetime)
  → Constraint: only ONE GET stream per session (409 Conflict on second). P3's POST-upgrade sink is scoped to the POST's goroutine and does not interact with the GET rule

- [ ] `plan/learned/638-mcp-2-remote-oauth.md` — P2 landed identity + auth dispatcher
  → Constraint: `Identity` is a value type on `*session` bound at `initialize`; P3 does NOT re-auth per frame. Correlation identity = session identity
  → Decision: new deps forbidden for schema validation — matches P2's stdlib-only JWT choice. Flat-primitive validator is ~80 LOC hand-rolled

- [ ] `.claude/rules/json-format.md` — external-spec exemption
  → Constraint: MCP uses camelCase; decode via `map[string]any` + key lookup, never struct tags (`check-json-kebab.sh` blocks `json:"camelCase"`)

- [ ] `.claude/rules/goroutine-lifecycle.md`
  → Constraint: no goroutine-per-Elicit. The handler's own POST goroutine blocks on the correlation channel; no new background goroutines

- [ ] `.claude/rules/integration-completeness.md`
  → Constraint: wiring test MUST exercise a real entry point. Drives the decision to ship `ze_execute` missing-command elicitor in this spec rather than deferring

- [ ] `.claude/rules/spec-no-code.md`
  → Constraint: this spec uses tables and prose only; no code snippets

### Source Files

- [ ] `internal/component/mcp/session.go` (369 L) — `sessionRegistry`, `session` struct, `Send`, `sendMu`, `Outbound`, `Identity`, `Touch`, TTL GC
  → Constraint: `session` fields are immutable after Create except `lastSeenAt`, `streamActive`, `closed`, `outbound` contents. P3 adds `clientElicit bool`, `correlations map[string]chan elicitResponse`, `activePostSink replySink` (request-scoped)
  → Constraint: new fields that are request-scoped (`activePostSink`) are set/cleared within `handlePOST`'s goroutine under a new `postMu` mutex; they are NOT fields a second concurrent POST can observe meaningfully because sessions process POSTs serially in practice (one session, one client)
  → Decision: `correlations` map protected by existing `session.mu` (already used for `lastSeenAt`)

- [ ] `internal/component/mcp/streamable.go` (1099 L) — `Streamable`, `ServeHTTP`, `handlePOST`, `handleGET`, `runMethod`, `callTool`, `doInitialize`, `buildInitializeResult`, `parseInitializeProtocolVersion`
  → Constraint: `runMethod(sess *session, req *request) *response` already receives the session but discards it (`_ = sess`). P3 stops discarding — passes `sess` into `callTool`
  → Constraint: `handlePOST` currently writes `application/json` unconditionally; P3 introduces a sink abstraction so the writer can be swapped to SSE mid-call
  → Constraint: `doInitialize` does not read client capabilities today; P3 parses `params.capabilities.elicitation` and stores on `*session` at create time
  → Decision: JSON-RPC response branch goes in `handlePOST` BEFORE the `req.ID == nil` notification branch, because a response has `id != nil` but no `method`
  → Decision: file is at 1099 L; exceeds 1000-L threshold in `rules/file-modularity.md`. P3 splits after adding: move elicitation dispatch helpers to `elicit.go`, and consider pulling `handlePOST`+`handleGET` into `streamable_http.go` if the total swells further

- [ ] `internal/component/mcp/handler.go` (311 L) — legacy 2024-11-05 handler, JSON-RPC types (`request`, `response`, `rpcError`, `callParams`), `server` struct, `toolHandlers`, `handcraftedTools`
  → Constraint: `server{dispatch, commands}` is the object passed to handlers today. P3 adds `session *session` field
  → Constraint: handcrafted `ze_execute` handler currently requires `args.Command`; P3 extends with a missing-command branch that calls `session.Elicit`
  → Decision: legacy `Handler(...)` factory stays untouched — P3 does NOT wire elicitation into the 2024-11-05 path. Only the Streamable path (2025-06-18) supports elicitation

- [ ] `internal/component/mcp/tools.go` (415 L) — auto-generation, `dispatchGenerated`
  → Constraint: auto-generated tools are structure-driven (YANG-typed params) and have no natural "missing input" axis to elicit on. Phase 3 leaves them alone
  → Decision: no changes to auto-generation in P3; `ze_execute` is the lone elicitor

- [ ] `internal/component/mcp/auth.go` (198 L) — `Identity` value type, `AuthMode`, `authenticator` interface, `authError`
  → Constraint: `Identity.Name` + `Scopes` are the only identity fields today; P3 does not extend. Correlation log / audit (if any) uses `session.Identity()` untouched
  → Constraint: `Identity` is zero-valued for AuthMode=None; P3 must gate elicitation by client-declared capability, NOT by identity (anonymous callers can still elicit if they declared the capability)

- [ ] `internal/component/mcp/streamable_test.go` (~800 L) — existing test helpers (`newTestStreamable`, `initializeResult`, `postMethod`, etc.)
  → Constraint: P3 tests reuse these helpers; add `postElicitResponse(t, hs, sid, id, body) int` for the JSON-RPC response POST and `openSSEStream(t, hs, sid)` for reading SSE frames inline
  → Constraint: httptest.Server-based; tests run with `-race` in `make ze-unit-test`

### MCP Spec Pages

- [ ] `modelcontextprotocol.io/specification/2025-06-18/client/elicitation`
  → Constraint: client MUST declare `capabilities.elicitation = {}` on initialize
  → Constraint: `elicitation/create` request has `params.message` (string) and `params.requestedSchema` (restricted JSON Schema); no other fields
  → Constraint: `requestedSchema` is a flat object — supported prop types are string (formats: email, uri, date, date-time), number/integer (min/max), boolean (default), enum (`enum`+`enumNames`). Nested objects, arrays, `oneOf`, `allOf`, `$ref` are explicitly not supported
  → Constraint: response `action` enum is `accept` (with `content`), `decline` (content typically omitted), `cancel` (content typically omitted). Three-state, not two
  → Constraint: server MUST NOT request sensitive information (documentation-level, not enforceable)
  → Constraint: client SHOULD rate-limit (client-side; server-side cap is prudent but not spec-mandated)

**Key insights:**
- MCP 2025-06-18 elicitation is strict: client must declare capability, schema is flat-primitive-only, response is three-state.
- Transport mechanism: server-initiated request rides the session's SSE channel; client response is a separate POST correlated by JSON-RPC id. Ze carries this with per-POST reply sinks, not the long-lived GET outbound queue.
- Schema sanity is at `session.Elicit()` call time (the spec's `requestedSchema` is a parameter to the server-initiated REQUEST, not a pre-registered thing).
- Elicitation interacts with Phase 4 tasks via the `input_required` task state. P3 designs the Elicit API with a signature that a future task-status publisher can also reuse (same correlation map), but P3 does NOT implement any task-related code.
- One concrete elicitor is `ze_execute` missing-command branch — reuses an already-handcrafted tool rather than inventing a synthetic one.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/component/mcp/session.go` (369 L) — session registry + session state; ttl GC; SSE outbound queue; `Send(frame []byte)` non-blocking with `sendMu`; `Outbound() <-chan []byte`; `Identity` set once via `Create(protocolVersion, identity)`
- [ ] `internal/component/mcp/streamable.go` (1099 L) — `NewStreamable`, `ServeHTTP` routes `/mcp` POST/GET/DELETE/OPTIONS and `/.well-known/oauth-protected-resource`; `handlePOST` unmarshals one request per POST and writes one `application/json` response; `handleGET` is the GET SSE stream drain; `runMethod` dispatch switch supports `notifications/initialized`, `tools/list`, `tools/call`; `callTool` → `toolHandlers[name]` or `dispatchGenerated`
- [ ] `internal/component/mcp/handler.go` (311 L) — legacy 2024-11-05 handler; JSON-RPC types (`request`, `response`, `rpcError`); `server{dispatch, commands}`; handcrafted `ze_execute`
- [ ] `internal/component/mcp/tools.go` (415 L) — `groupCommands`, `generateTools`, `dispatchGenerated`; `(s *server).run(command)` for text dispatch
- [ ] `internal/component/mcp/auth.go` (198 L) — `Identity` (Name, Scopes), `AuthMode` enum, `authenticator` interface

**Behavior to preserve:**

- `session.Send` stays non-blocking with len/cap pre-check; `sendMu` keeps serialising producers
- `handlePOST` still returns `application/json` when the handler does NOT elicit (no regression for non-eliciting tools)
- `handleGET` unchanged — GET stream drains `session.outbound` as today
- `doInitialize` still sets the `Mcp-Session-Id` response header and returns negotiated `protocolVersion`
- Auth gate (Phase 2) runs on initialize only — no re-auth for the response POST carrying an elicit reply (session-id validity is the per-request token per P2 decision)
- Auto-generated tools (`dispatchGenerated` path) unchanged — they do not elicit in P3
- Existing `TestStreamable*` tests pass unchanged

**Behavior to change:**

- `initialize` response still declares `capabilities.tools = {}`; no new SERVER capability key added. Elicitation is a CLIENT capability — the server merely honours it
- `handlePOST` gains a branch: if the body is a JSON-RPC response shape (no method, has id, has result or error), route to `session.resolveElicit(id, body)` and return 202 Accepted
- `handlePOST` introduces a sink abstraction so the tool handler can trigger an SSE upgrade mid-call; when upgraded, subsequent frames (elicitation/create AND the terminal tool result) go as SSE events
- `server` struct (`handler.go`) grows `session *session`; populated in `callTool` before handler dispatch
- Handcrafted `ze_execute` grows a missing-command branch that calls `session.Elicit`

## Data Flow (MANDATORY — see `rules/data-flow-tracing.md`)

### Entry Point

Client (MCP-speaking AI or test) sends POST to `/mcp` carrying a JSON-RPC request.
Format: `application/json` body with `jsonrpc`, `id`, `method`, `params` fields.

For the elicit-response leg, client POSTs a JSON-RPC *response* body (no `method`,
has `id`, has `result` or `error`) — Ze branches on absence of `method`.

### Transformation Path (outbound elicit)

1. `handlePOST` unmarshals body; present branch: `req.Method != "" && req.ID != nil`
2. Protocol-version + session header + session lookup (existing logic)
3. `callTool` pulls `server{dispatch, commands, session}`; dispatches handler
4. Handler sees `args.command == ""`; checks `s.session.ClientSupportsElicit()`; if no, returns typed "missing command" error (no frame, AC-15a path)
5. If yes: `s.session.Elicit(ctx, message, schema)` called
6. `Elicit` validates `schema` via `validateElicitSchema` (flat primitives only); on fail, returns `ErrElicitSchemaInvalid` with offending path (no frame, AC-15)
7. `Elicit` allocates a correlation id (base64url of 16 random bytes); registers `chan elicitResponse` under `session.correlations[id]`; pending cap check (default 32; `ErrElicitTooMany` if exceeded)
8. `Elicit` builds the JSON-RPC request frame carrying method `elicitation/create` and the message + schema params
9. `Elicit` calls `session.UpgradeToSSE()` which flips the current sink from JSON to SSE (writes the SSE response headers, flushes, replaces `session.activePostSink`)
10. `Elicit` writes the frame via `activePostSink.WriteFrame(frame)` — first SSE `data: ...\n\n` event
11. `Elicit` blocks on `<-chan elicitResponse` with ctx select

### Transformation Path (inbound response)

12. Client POSTs the JSON-RPC response. `handlePOST` unmarshals; detects `body.Method == "" && body.ID != nil && (body.Result != nil || body.Error != nil)`
13. Session lookup via `Mcp-Session-Id` header (required on the response POST too — MCP 2025-06-18 rule)
14. `session.ResolveElicit(id, body)` looks up `correlations[id]`; parses action enum from `result.action`; on valid, delivers typed `elicitResponse` to channel; deletes map entry
15. `handlePOST` returns 202 Accepted, empty body
16. Suspended `Elicit` in step 11 wakes; returns `(content, nil)` or typed decline/cancel/malformed error
17. Tool handler receives content; continues dispatch (calls `s.dispatch(content["command"])` for the `ze_execute` path)
18. Handler returns final tool result; runMethod serialises to JSON-RPC response; `handlePOST`'s sink (now SSE) writes it as the terminal `data: ...\n\n` frame, then closes the stream

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| HTTP layer ↔ JSON-RPC layer | POST body unmarshaled; request-vs-response branch by presence of `method` | [ ] |
| JSON-RPC layer ↔ session registry | `Mcp-Session-Id` header → session lookup on both legs | [ ] |
| Session ↔ per-POST reply sink | `session.activePostSink` is set on POST entry, swapped to SSE on Elicit, cleared on POST exit | [ ] |
| Session ↔ correlation map | `session.correlations[id] = chan elicitResponse`; register/resolve/cancel under `session.mu` | [ ] |
| Tool handler ↔ session | `server.session` carries the session pointer for handler's lifetime | [ ] |
| Elicit ↔ ctx cancellation | `ctx.Done()` → `session.CancelElicit(id)` → pending map entry deleted → Elicit returns wrapped ctx err | [ ] |

### Integration Points

- `reactor.ExecuteCommand` / dispatcher — unchanged; `ze_execute` still dispatches the string command after elicitation completes
- `CommandLister` — unchanged
- `Identity` (P2) — read-only; `session.Identity()` untouched by P3
- Phase 4 (future) task-status publisher will reuse `correlations` for `input_required` branches — P3 designs `elicitResponse` with enough shape for that extension (future-extensible but not over-engineered now)

### Architectural Verification

- [ ] No bypassed layers — elicitation goes through session.correlations; no direct goroutine handoff
- [ ] No unintended coupling — `session` remains unaware of HTTP specifics; the sink abstraction is the only seam; `streamable.go` owns the HTTP upgrade mechanics
- [ ] No duplicated functionality — reuses `session.sendMu` serialisation contract for any frame touching `outbound`; per-POST sink is a separate writer (different lifetime, different channel of existence)
- [ ] Zero-copy preserved — frames are built once per Elicit, written once; no intermediate buffer pools (JSON marshal is the convention here, not wire encoding)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| POST `tools/call` for `ze_execute` with empty `command`, client declared `elicitation: {}`, client POSTs elicit response with accept + command | → | `handlePOST` → `callTool` → `ze_execute` handler → `session.Elicit` (with POST→SSE upgrade) → frame on SSE → second POST detected as response, routed to correlation → handler resumes → dispatcher runs supplied command → terminal SSE frame | `test/mcp/elicitation-accept.ci` |
| Same first POST, client responds with decline action | → | `Elicit()` returns `ErrElicitDeclined`; handler returns error result to the SSE terminal frame | `test/mcp/elicitation-decline.ci` |
| POST `tools/call` for `ze_execute` with empty `command`, client did NOT declare `elicitation` | → | `Elicit()` returns `ErrElicitUnsupported`; no SSE upgrade; `application/json` response with error result (no frame sent) | `test/mcp/elicitation-no-capability.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-12 | Tool handler calls `session.Elicit(ctx, msg, schema)`; client declared `elicitation: {}` at initialize | The originating POST response upgrades to `text/event-stream`; an SSE frame carrying an `elicitation/create` JSON-RPC request with the given message + schema is delivered |
| AC-13 | Client POSTs an elicit response body with action `accept` and a content object matching the schema | Response POST returns 202 Accepted with empty body; suspended `Elicit()` returns `(content, nil)`; tool handler completes; final tool result is the terminal SSE frame on the original POST |
| AC-14 | Client POSTs an elicit response body with action `decline` or `cancel` | 202 returned; `Elicit()` returns `(nil, ErrElicitDeclined)` or `(nil, ErrElicitCancelled)` respectively; handler chooses fallback behaviour |
| AC-15 | Handler calls `Elicit()` with a `requestedSchema` containing a nested object, array-of-object, `oneOf`, `allOf`, `$ref`, or a non-object root | `Elicit()` returns `(nil, ErrElicitSchemaInvalid)` naming the offending path; no frame is sent; no SSE upgrade occurs; tool's error path runs |
| AC-15a | Handler calls `Elicit()` and client did NOT declare `elicitation: {}` at initialize | `Elicit()` returns `(nil, ErrElicitUnsupported)`; no frame sent; no SSE upgrade; tool's fallback path runs |
| AC-15b | Client POSTs a JSON-RPC response whose id matches no pending correlation | 202 Accepted; frame silently dropped (no server state to update); logged at debug |
| AC-15c | Client's POST context cancels (disconnect) while `Elicit()` is suspended | Correlation entry deleted; `Elicit()` returns `(nil, <wrapped ctx err>)`; no write to sink after cancellation |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestElicit_SchemaRejectsNestedObject` | `internal/component/mcp/elicit_test.go` | AC-15 (nested object) |
| `TestElicit_SchemaRejectsArrayOfObjects` | same | AC-15 (array-of-object) |
| `TestElicit_SchemaRejectsOneOf` | same | AC-15 (oneOf) |
| `TestElicit_SchemaAllowsString` | same | string primitives including format=email/uri/date/date-time pass |
| `TestElicit_SchemaAllowsNumber` | same | number/integer with min/max pass |
| `TestElicit_SchemaAllowsBoolean` | same | boolean pass |
| `TestElicit_SchemaAllowsEnum` | same | enum with enumNames pass |
| `TestElicit_NoCapabilityReturnsUnsupported` | same | AC-15a at session-level (mock sink) |
| `TestElicit_AcceptReturnsContent` | same | AC-13 at session level |
| `TestElicit_DeclineSentinel` | same | AC-14 decline |
| `TestElicit_CancelSentinel` | same | AC-14 cancel |
| `TestElicit_ContextCancelDropsPending` | same | AC-15c |
| `TestElicit_UnknownIDIgnored` | same | AC-15b resolve path: no panic, no write; map unchanged |
| `TestElicit_PendingCapRejects` | same | pending-cap guard returns `ErrElicitTooMany` |
| `TestStreamable_POSTUpgradesToSSEOnElicit` | `internal/component/mcp/streamable_test.go` | AC-12 wire: Content-Type switches, terminal frame is the tool result |
| `TestStreamable_JSONRPCResponseBranch` | same | response body routed to correlation; 202 returned |
| `TestStreamable_InitializeReadsClientCapabilities` | same | `clientElicit` flag set when `capabilities.elicitation = {}` present |
| `TestZeExecute_MissingCommandElicits` | `internal/component/mcp/handler_test.go` (or new `handcrafted_test.go`) | one-elicitor wiring — `ze_execute` handler actually invokes `session.Elicit` on missing command |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| pending elicitations per session | 1 – 32 (default cap) | 32 | N/A | 33 (returns ErrElicitTooMany) |
| elicit id charset | base64url 22-char | 22 | < 22 | > 22 |
| schema property count | 0 – no hard cap | any | N/A | N/A (flat shape bounds it organically) |

No numeric CONFIG inputs added in this spec (no new YANG leaves). Pending cap is a compiled constant; boundary covered via unit test.

### Functional Tests

| Test | Location | End-User Scenario |
|------|----------|-------------------|
| `test-mcp-elicitation-accept` | `test/mcp/elicitation-accept.ci` | Client initialize → tools/call ze_execute no command → receives elicitation/create via SSE → client accepts with a valid ze command → tool runs and returns output as terminal SSE frame |
| `test-mcp-elicitation-decline` | `test/mcp/elicitation-decline.ci` | Same setup → client declines → tool returns error result as terminal SSE frame |
| `test-mcp-elicitation-no-capability` | `test/mcp/elicitation-no-capability.ci` | Client initialize WITHOUT elicitation capability → tools/call ze_execute no command → plain JSON response with error result; no SSE upgrade, no elicit frame |

## Files to Modify

- `internal/component/mcp/session.go` — add `clientElicit bool`, `correlations map[string]chan elicitResponse`, `activePostSink replySink`, `postMu sync.Mutex`; add `RegisterElicit(id) (chan elicitResponse, error)`, `ResolveElicit(id, body) bool`, `CancelElicit(id)`, `ClientSupportsElicit() bool`, `SetActivePostSink(sink) func()`, `UpgradeToSSE() error`
- `internal/component/mcp/streamable.go` — extend `doInitialize` to parse `params.capabilities.elicitation`; stop discarding `sess` in `runMethod`; extend `callTool` to pass session into the `server` struct; add response-branch detection in `handlePOST`; implement `jsonReplySink` and `sseReplySink` and the upgrade handoff
- `internal/component/mcp/handler.go` — add `session *session` to `server` struct; extend `ze_execute` handler with the missing-command elicit branch
- `internal/component/mcp/tools.go` — ensure `dispatchGenerated` path still works with the enriched `server` struct (session is unused there)
- `internal/component/mcp/streamable_test.go` — new helpers: `postElicitResponse(t, hs, sid, id, bodyMap) int`, `readSSEFrames(resp, count) [][]byte`
- `docs/architecture/mcp/overview.md` — add row to Transport Shape describing POST→SSE upgrade on elicit; add paragraph explaining capability negotiation

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema changes | No — elicitation is an MCP protocol feature, not a ze config knob | — |
| Env vars | No — no new config | — |
| CLI commands/flags | No — feature is visible only through the MCP protocol | — |
| Editor autocomplete | No | — |
| Functional test per capability | Yes | `test/mcp/elicitation-*.ci` (3 scenarios above) |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — add "MCP elicitation (2025-06-18)" row |
| 2 | Config syntax changed? | No | — |
| 3 | CLI command added/changed? | No | — |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` — `elicitation/create` is a new server-initiated method |
| 5 | Plugin added/changed? | No | — |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/elicitation.md` (new file) — how a tool handler elicits + client-side capability requirement |
| 7 | Wire format changed? | No (MCP is not BGP wire) | — |
| 8 | Plugin SDK/protocol changed? | No | — |
| 9 | RFC behavior implemented? | No RFC — MCP spec is the reference | — |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` — document `test/mcp/elicitation-*.ci` runner flow |
| 11 | Affects daemon comparison? | Maybe | `docs/comparison.md` — if MCP row lists features, add elicitation |
| 12 | Internal architecture changed? | Yes | `docs/architecture/mcp/overview.md` — POST→SSE upgrade row; capability negotiation paragraph |

## Files to Create

- `internal/component/mcp/elicit.go` — `validateElicitSchema`, `elicitResponse`, `ErrElicit*` sentinels, `session.Elicit()` method body
- `internal/component/mcp/elicit_test.go` — unit tests listed above
- `test/mcp/elicitation-accept.ci` — functional test (accept path)
- `test/mcp/elicitation-decline.ci` — functional test (decline path)
- `test/mcp/elicitation-no-capability.ci` — functional test (no capability)
- `docs/guide/mcp/elicitation.md` — user guide page

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation Phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-verify-fast` |
| 6. Critical review | Critical Review Checklist |
| 7. Fix issues | Resulting fixes |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase 1 — Schema validator + typed errors**
   - Tests: `TestElicit_SchemaRejects*`, `TestElicit_SchemaAllows*`
   - Files: `elicit.go` (validator + error sentinels), `elicit_test.go`
   - Verify: tests fail → implement `validateElicitSchema` over `map[string]any` → tests pass
2. **Phase 2 — Correlation map + Elicit core (session-level, mock sink)**
   - Tests: `TestElicit_AcceptReturnsContent`, `TestElicit_DeclineSentinel`, `TestElicit_CancelSentinel`, `TestElicit_NoCapabilityReturnsUnsupported`, `TestElicit_ContextCancelDropsPending`, `TestElicit_UnknownIDIgnored`, `TestElicit_PendingCapRejects`
   - Files: `session.go` (new fields + register/resolve/cancel), `elicit.go` (Elicit method body), `elicit_test.go`
   - Verify: tests fail → add session fields + methods → Elicit implementation → tests pass
3. **Phase 3 — HTTP reply sinks + POST→SSE upgrade**
   - Tests: `TestStreamable_POSTUpgradesToSSEOnElicit`, `TestStreamable_JSONRPCResponseBranch`
   - Files: `streamable.go` (sink types, upgrade mechanics, response branch in handlePOST), `streamable_test.go` (new helpers)
   - Verify: tests fail → implement `jsonReplySink`/`sseReplySink`, the switcheroo, and the response branch → tests pass
4. **Phase 4 — Initialize capability parsing**
   - Tests: `TestStreamable_InitializeReadsClientCapabilities`
   - Files: `streamable.go` (`doInitialize` reads `params.capabilities.elicitation`), `session.go` (store the bit)
   - Verify: test fails → implement parse + store → test passes
5. **Phase 5 — ze_execute elicitor wiring**
   - Tests: `TestZeExecute_MissingCommandElicits`
   - Files: `handler.go` (extend `ze_execute` handler), `handler_test.go` or new `handcrafted_test.go`
   - Verify: test fails → add missing-command branch → test passes
6. **Phase 6 — Functional tests**
   - Files: `test/mcp/elicitation-accept.ci`, `test/mcp/elicitation-decline.ci`, `test/mcp/elicitation-no-capability.ci`
   - Verify: run `bin/ze-test mcp` scenarios locally; all three pass
7. **Phase 7 — Documentation**
   - Files: `docs/guide/mcp/elicitation.md` (new), `docs/architecture/mcp/overview.md` (amend), `docs/features.md`, `docs/architecture/api/commands.md`, `docs/functional-tests.md`
8. **Phase 8 — Full verification** → `make ze-verify-fast`
9. **Phase 9 — Spec completion** → fill audit tables, write `plan/learned/NNN-mcp-3-elicitation.md`, two-commit sequence

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-12..AC-15c each have a named test with a direct assertion on the AC's expected behavior |
| Correctness | POST→SSE upgrade is irreversible within a single POST; no path allows a downgrade back to JSON mid-call |
| Naming | `session.Elicit` capitalised (exported); `elicitResponse` lowercased (package-internal); error sentinels `ErrElicit*` (exported for callers who want to detect) |
| Data flow | tool handler → session.Elicit → sink.WriteFrame (per-POST) OR session.Send (outbound GET queue). No path crosses these two channels |
| Rule: no-layering | Old `handlePOST` path with only `application/json` branch is REPLACED by the sink-abstracted path, not retained as a parallel branch |
| Rule: derive-not-hardcode | Schema type enum derived from one map; validator loops the map, does not re-enumerate |
| Rule: exact-or-reject | Schema validator REJECTS on unsupported shapes (no silent simplification); pending cap REJECTS with typed error (no silent queuing) |
| Rule: json-format | Frame marshaling via `map[string]any` + `encoding/json`; no struct tags with camelCase |
| Rule: goroutine-lifecycle | No `go func()` per Elicit — handler goroutine blocks on channel |
| Rule: memory pointers | `elicitResponse` value type carried on channel — no pointer shared with client-controlled data |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `session.Elicit()` exists and is callable from tool handlers | `grep -rn 'session.Elicit(' internal/component/mcp/` returns at least the `ze_execute` call site |
| Schema validator rejects every forbidden shape | `go test -run 'TestElicit_Schema' ./internal/component/mcp/` passes |
| POST→SSE upgrade works end-to-end | `bin/ze-test mcp` runs `test/mcp/elicitation-accept.ci` and asserts SSE upgrade + terminal frame |
| `ze_execute` missing-command elicit path fires | Functional test asserts SSE frames, then dispatch, then terminal frame |
| Docs exist | `ls docs/guide/mcp/elicitation.md` |
| `make ze-verify-fast` passes | `tmp/ze-verify.log` shows PASS |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | `requestedSchema` rejected on disallowed shapes; client response body size bounded by `maxRequestBody` already; action enum is closed set, anything else → `ErrElicitMalformed` |
| Sensitive-info guard | MCP spec: server MUST NOT elicit sensitive information. Document in `docs/guide/mcp/elicitation.md`; not enforceable at runtime |
| Resource exhaustion | Pending cap per session (default 32) prevents unbounded `correlations` growth; per-elicit timeout via ctx (caller-supplied or a sensible default like 5 min if ctx has no deadline) |
| DoS via unresolved elicits | Cap + ctx + session TTL sweep — when the session expires, pending Elicits unblock via session close path |
| Information leakage | Error messages to client do not leak schema internals (path in message is fine; stack traces are not) |
| Correlation id unpredictability | 128-bit crypto/rand base64url-encoded — matches session-id rationale |
| Log hygiene | Message + schema could contain user-sensitive content; log at debug with redacted schema property values |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Phase that introduced it |
| Test fails wrong reason | Fix test assertion/setup |
| Test fails behavior mismatch | Re-read `Current Behavior`; if source misunderstood → RESEARCH |
| POST→SSE upgrade races the handler return | Re-read `Failure Mode Analysis`; add synchronisation via `session.postMu` |
| Functional test stalls | Check SSE read loop terminates on terminal frame; check client response POST arrives before timeout |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Reuse `session.outbound` chan for elicitation frames | `outbound` is the long-lived GET-stream queue; mixing per-POST frames into it would deliver them to the wrong stream | Per-POST reply sink scoped to `handlePOST`'s goroutine |
| `requestedSchema` validation at tool registration time (umbrella AC-15 phrasing) | Elicitation schemas are per-call parameters, never pre-registered | AC-15 phrasing corrected to call-site sanity |
| `context.WithValue` to pass session into handlers | Discouraged for non-request-scoped data; explicit field is cleaner and LSP-navigable | `server` struct gains `session *session` field |
| Import a JSON Schema library for validation | No new deps; flat-primitive validator is ~80 LOC | Hand-rolled validator in `elicit.go` |
| GET-stream-only for server-initiated messages | Requires client to hold a GET open; ordering vs terminal response undefined; diverges from spec | POST→SSE upgrade on first Elicit |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Umbrella ACs with imprecise timing wording (e.g., "registration time" for per-call data) | 1× (AC-15) | Add note to `rules/planning.md`: "Umbrella ACs for protocol features must anchor timing to the actual protocol event (request/response), not to lifecycle stages the protocol doesn't have" | File once spec lands |

## Design Insights

- **Per-POST sink vs long-lived outbound queue** is the structural split that makes elicitation clean. Mixing them was tempting but would fail as soon as a concurrent GET stream existed.
- **Capability negotiation lives on `*session`**, not on the Streamable. Each session carries its client-declared capability bits immutably after initialize. Matches the P2 identity pattern.
- **`elicitResponse` as a value type** carried on a channel (no pointer to client-controlled payload) preserves the memory rule about cross-component boundaries. Even though session and elicit live in the same package, this is the pattern Phase 4 will extend for task notifications.
- **The correlation map is extensible to tasks.** Phase 4's `tasks/status` notification could reuse the same pending-channel infrastructure keyed by task id. P3 designs Elicit to NOT depend on elicitation-specific detail in the register/resolve primitives — keeping them `RegisterPending(id)` / `ResolvePending(id, body)` would make P4's job trivial. But: per anti-premature-abstraction, P3 ships elicitation-specific names today; P4 renames when the second user arrives.
- **AC-15's "registration time" language from the umbrella was incorrect.** This spec records the correction and adds AC-15a/b/c for capability gate, unknown-id, and ctx-cancel — gaps in the umbrella's AC table.

## RFC Documentation

MCP 2025-06-18 is the authoritative spec, not an IETF RFC. Inline comments
at `session.Elicit` and the `elicitation/create` sender cite:

- `modelcontextprotocol.io/specification/2025-06-18/client/elicitation` — schema restriction, action enum, capability declaration
- `modelcontextprotocol.io/specification/2025-06-18/basic/transports` — POST response Content-Type flexibility

## Implementation Summary

_Filled after implementation per `/implement` stage 13._

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates
- [ ] AC-12..AC-15c all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] One concrete elicitor wired (`ze_execute` missing-command path)

### Quality Gates
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (correlation primitive kept elicitation-specific for now)
- [ ] No speculative features (no task integration in P3)
- [ ] Single responsibility per new file (elicit.go owns only elicitation)
- [ ] Minimal coupling (session does not import streamable; sink interface is the seam)

### TDD
- [ ] Tests written before code in each phase
- [ ] Tests FAIL (paste output per phase)
- [ ] Tests PASS (paste output per phase)
- [ ] Boundary test for pending-elicit cap
- [ ] Functional `.ci` tests for all three wiring-test rows

### Completion
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-mcp-3-elicitation.md`
