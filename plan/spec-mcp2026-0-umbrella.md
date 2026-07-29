# Spec: mcp2026-0-umbrella

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | 0/4 |
| Deferral shard | `plan/deferrals/mcp2026-0-umbrella.md` |
| Updated | 2026-07-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's MCP server speaks protocol revision `2025-06-18` (`internal/component/mcp/streamable.go:30`)
and additionally accepts `2025-03-26` and `2024-11-05` at initialize
(`streamable.go:49-53`). The MCP specification published revision `2026-07-28`
on 2026-07-28 (release candidate locked 2026-05-21, ten-week validation window
now closed). Ze skipped `2025-11-25` entirely, so the server is two revisions
behind.

`2026-07-28` is not an additive release. It removes the four mechanisms Ze's
implementation is built on:

| Removed by the spec | Ze code that depends on it |
|---------------------|----------------------------|
| The `initialize` / `notifications/initialized` handshake | `doInitialize` + `buildInitializeResult` (`streamable.go:700-713`, `:777-798`) |
| Protocol-level sessions and the `Mcp-Session-Id` header | `sessionRegistry` and `session` (`session.go`, 570 lines), minted at `streamable.go:448`, terminated by `handleDELETE` (`:681-694`) |
| The GET stream endpoint | `handleGET` (`streamable.go:618-678`) |
| Server-initiated JSON-RPC requests on SSE streams | `session.Elicit` (`elicit.go:227`) emitting `elicitation/create` (`elicit.go:295`) through the POST-to-SSE sink upgrade (`reply_sink.go`), with the client's JSON-RPC response consumed by `handleElicitResponse` (`streamable.go:562-605`) |

The goal is full conformance with `2026-07-28` as a **clean cutover**: revisions
`2024-11-05` through `2025-11-25` are dropped, not maintained alongside. Ze has
never been released and the MCP server protocol is not the plugin API, so
`ai/rules/compatibility.md` applies and no backwards-compatibility surface is
owed. Owner decision, 2026-07-28.

This umbrella coordinates four child specs and is never implemented directly.

| Phase | Spec | Delivers |
|-------|------|----------|
| 1 | `plan/spec-mcp2026-1-stateless-core.md` | Sessions and handshake deleted, per-request `_meta`, per-request auth, header validation, `server/discover`, `resultType` |
| 2 | `plan/spec-mcp2026-2-mrtr.md` | Multi Round-Trip Requests replacing server-initiated elicitation, integrity-protected `requestState` |
| 3 | `plan/spec-mcp2026-3-tasks-extension.md` | Tasks as the `io.modelcontextprotocol/tasks` extension, server-directed, principal-scoped |
| 4 | `plan/spec-mcp2026-4-caching-apps.md` | `ttlMs` / `cacheScope` cache hints, deterministic tool order, MCP Apps as the `io.modelcontextprotocol/ui` extension |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/mcp/overview.md` - the subsystem design doc every MCP source file names in its `// Design:` line
  → Constraint: every `.go` file under `internal/component/mcp/` points here, so a structural change to the transport requires this doc to be rewritten in the same work (`ai/rules/design-doc-references.md`).
- [ ] `ai/digests/mcp.md` - living data-flow digest for the MCP subsystem
  → Constraint: anchors are validated by `make ze-digest-check`, so deleting `session.go` breaks the digest unless it is updated in the same commit.
- [ ] `ai/rules/compatibility.md` - no backwards compatibility pre-release
  → Decision: the cutover deletes the legacy revisions rather than running dual-era. The only protected surface is the plugin API, which MCP is not.
- [ ] `ai/rules/rfc-compliance.md` - conformance is not negotiable
  → Constraint: partial conformance is not selectable by the implementing session. Any requirement that cannot be met in full is escalated to Thomas as a "which way do I fix it" question, never as "may I skip it".

### Protocol Specification (Scope: protocol)
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/changelog` - the authoritative list of breaking changes
  → Constraint: 9 major changes, 12 minor. Every one is either implemented by a child spec or recorded in Known Limitations with a reason.
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http` - the binding Ze implements
  → Constraint: "Servers **MUST NOT** send independent JSON-RPC requests on this stream." "The client **MUST NOT** send JSON-RPC *responses*." Both are violated by the current elicitation design.
  → Constraint: GET or DELETE to the MCP endpoint **SHOULD** return `405 Method Not Allowed`; `Mcp-Session-Id` and `Last-Event-ID` headers **SHOULD** be ignored.
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning` - version negotiation and the era compatibility matrix
  → Constraint: "Servers **MUST** implement `server/discover`."
  → Decision: the matrix records Legacy client against Modern server as **Fails**, with no fall-forward for the client. This is the accepted cost of the cutover, and drives R-1 below.
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr` - the replacement for server-initiated requests
  → Constraint: servers **MUST** treat `requestState` as attacker-controlled and **MUST** protect its integrity when it influences authorization, resource access, or business logic.
- [ ] `https://modelcontextprotocol.io/extensions/tasks/overview` - the Tasks extension
  → Constraint: `tasks/list` and `tasks/result` no longer exist; task creation is server-directed, not client-flagged.

**Key insights:** (minimal context to resume after compaction)
- The spec finalised on 2026-07-28. There is no pending churn to wait for.
- The cutover is a net deletion. Sessions, TTL sweep, session caps, the GET stream, the correlation map, the POST-to-SSE upgrade and the DELETE handler all go, and the operational requirement for sticky load-balancer routing goes with them.
- Ze is an MCP **server** only. Five of the six authorization SEPs in this revision (`iss` validation, DCR `application_type`, credential binding, token refresh, scope accumulation) are client-side MUSTs and do not apply. The RFC 9728 protected-resource metadata endpoint and JWKS validation are unchanged.
- Ze also owns an MCP **client**: the `ze-test mcp` driver (`internal/test/cli/cmd_mcp.go`, 785 lines), which threads `Mcp-Session-Id` (`:460-461`, `:578-579`, `:612-613`) and calls `initialize` (`:360-377`). Each phase updates it for what that phase changed.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [x] `internal/component/mcp/streamable.go` (808L) - HTTP dispatcher. `ServeHTTP` routes RFC 9728 metadata, then Origin check, then POST/GET/DELETE/OPTIONS. `handlePOST` special-cases `initialize` (auth + session mint), then requires `Mcp-Session-Id` for everything else.
- [x] `internal/component/mcp/streamable_tools.go` (368L) - `runMethod` switch over 9 methods: `notifications/initialized`, `tools/list`, `tools/call`, `tasks/{list,get,result,cancel}`, `resources/{list,read}`. `parseInitializeProtocolVersion` negotiates the version from initialize params.
- [x] `internal/component/mcp/session.go` (570L) - `sessionRegistry` with TTL GC, max-session cap, absolute lifetime cap; `session` holds identity, three client-capability bits, the elicit correlation map, the outbound SSE queue, and the active POST sink.
- [x] `internal/component/mcp/elicit.go` (302L) - `session.Elicit` validates a flat-primitive schema, upgrades the POST sink to SSE, writes an `elicitation/create` request frame, and blocks on a per-elicit channel until the client POSTs a correlated JSON-RPC response.
- [x] `internal/component/mcp/tasks.go` (530L) - `taskRegistry` keyed by task id, indexed `byIdentity`, each entry carrying `sessionID`; worker goroutines; `CancelAllForSession` on session expiry; `notifications/tasks/status` pushed to the session outbound queue (`:520`).
- [x] `internal/component/mcp/resources.go` (166L) - embedded-FS UI asset server for the `ui://` scheme, gated on the session's `clientResources` bit.
- [x] `internal/component/mcp/streamable_auth.go` (399L) - `buildAuthForMode` builds a per-mode `authenticator`; OAuth mode fetches AS metadata and primes the JWKS cache at startup.
- [x] `internal/component/mcp/tools.go` (855L) - tool descriptor generation from the YANG command registry; emits `_meta.ui` for UI-annotated groups (`:378`, `:386`).
- [x] `cmd/ze/hub/service_mcp.go` - daemon wiring; `NewStreamable` at `:202`.
- [x] `internal/chaos/orchestrator/run.go:535`, `internal/chaos/orchestrator/cli.go:635` - `ze-chaos` wiring in Provider mode.
- [x] `internal/test/cli/cmd_mcp.go` (785L) - the `ze-test mcp` client driver.

**Behavior to preserve:** (unless the user explicitly said to change it)
- Tool descriptor generation from the YANG command registry. Tool names, argument shapes and the `ze:task-support` / `ze:ui-resource` YANG extension walkers keep working; only the tool-call *result envelope* changes. `inputSchema` constraints loosen to full JSON Schema 2020-12, which is a superset of what `tools.go` emits today, so no generation change is forced.
- Origin validation (`streamable.go:371-385`) and the loopback-only default. Still a MUST in `2026-07-28`.
- All four auth modes (`none`, `bearer`, `bearer-list`, `oauth`) and their YANG config surface (`internal/component/mcp/yang/ze-mcp-conf.yang`). The `authenticator` interface already takes an `*http.Request` and returns an `Identity` (`streamable.go:396-401`), so it is already per-request shaped.
- The RFC 9728 protected-resource metadata endpoint and its CORS behaviour (`streamable.go:335-364`).
- Provider mode for `ze-chaos` (`StreamableConfig.Provider`). Note that Provider mode already accepts session-less POSTs (`streamable.go:463-475`), so it is the closest thing Ze has to a stateless path today.
- Resource-not-found already returns `-32602` (`resources.go:158-163`), which is what this revision requires. No change.

**Behavior to change:**
- Every item in the Task table above, plus the per-phase scope in each child spec.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- HTTP POST to `/mcp` (`Endpoint`, `streamable.go:42`) carrying a single JSON-RPC request or notification.
- After the cutover, every POST additionally carries `MCP-Protocol-Version`, `Mcp-Method`, and (for `tools/call`, `resources/read`, `prompts/get`) `Mcp-Name` headers, mirroring body fields.

### Transformation Path
1. `ServeHTTP` (`streamable.go:219`): RFC 9728 metadata path, then Origin allowlist, then method dispatch.
2. **New:** header validation. `MCP-Protocol-Version` must equal `_meta["io.modelcontextprotocol/protocolVersion"]`; `Mcp-Method` must equal `method`; `Mcp-Name` must equal `params.name` or `params.uri` after Base64-sentinel decoding. Mismatch or absence returns HTTP 400 with JSON-RPC `-32020`.
3. **New:** version check. An unsupported version returns HTTP 400 with `-32022` and a `supported` list.
4. **Changed:** authentication runs on **every** request, not only `initialize`. Today identity is bound at initialize and later requests are trusted by session-id validity alone (`streamable.go:391-401`).
5. `runMethod` dispatch (`streamable_tools.go:22`), with `initialize`, `notifications/initialized`, `tasks/list` and `tasks/result` removed and `server/discover`, `tasks/update` added.
6. Result assembly gains a mandatory `resultType` discriminator and, for list and read results, `ttlMs` + `cacheScope`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| MCP client ↔ Ze HTTP transport | JSON-RPC over HTTP POST; SSE only as a per-request response stream | No |
| Ze MCP ↔ command registry | `CommandDispatcher` / `CommandLister` interfaces; unchanged by this work | No |
| Ze MCP ↔ authenticator | `authenticator.Authenticate(*http.Request) (Identity, *authError)`; call site moves from initialize-only to per-request | No |
| Ze MCP ↔ `ze-chaos` Provider | `ToolProvider` interface; Provider mode loses its session-less special case because every path becomes session-less | No |
| Server ↔ client across an MRTR retry | Opaque `requestState` string, integrity-protected, carried through the client | No |

### Integration Points
- `cmd/ze/hub/service_mcp.go:202` (`startMCPServer`) - daemon wiring; `StreamableConfig` fields for session TTL, max sessions and max session lifetime become dead and are removed.
- `internal/chaos/orchestrator/run.go:535` and `cli.go:635` - `ze-chaos` wiring.
- `internal/test/cli/cmd_mcp.go` - the test client, rewritten per phase.
- `cmd/ze/help_ai.go:388-401`, `:551` - agent-facing documentation of the MCP wire shape.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Dropping revisions `2024-11-05` through `2025-11-25` is acceptable, accepting that clients speaking those revisions cannot talk to Ze at all | Owner decision 2026-07-28 ("clean cutover"); `ai/rules/compatibility.md` | Phase 1 would have to carry a dual-era code path, roughly doubling the transport surface permanently | User confirmation (obtained) | confirmed |
| A-2 | The `authenticator` interface needs no signature change to run per-request | `streamable.go:396-401` shows `Authenticate(r *http.Request) (Identity, *authError)`, already request-scoped | Phase 1 grows an auth refactor it did not budget for | Confirmed 2026-07-28: the interface is declared at `auth.go:108-112`, and all four implementations already take `*http.Request` (`bearer.go:41` none, `bearer.go:57` bearer, `bearer.go:97` bearer-list, `oauth.go:42` oauth). Per-request auth is a call-site move | confirmed |
| A-3 | No Ze code outside `internal/component/mcp/` and the three wiring sites depends on MCP session semantics | `grep` for `NewStreamable` returned only `service_mcp.go:202`, `run.go:535`, `cli.go:635` | Additional call sites need migration and the phase estimates are wrong | Confirmed 2026-07-28: a tree-wide grep for `Mcp-Session-Id\|sessionRegistry\|ClientSupports*` over `internal/ cmd/ pkg/`, excluding tests and the component itself, hits exactly one file, the test client `internal/test/cli/cmd_mcp.go` (lines 461, 476, 499, 579, 613). No production consumer outside the component | confirmed |
| A-4 | `subscriptions/listen` is not required for conformance, because Ze pushes no change notifications that survive the cutover | The subscriptions pattern page defines an opt-in filter with exactly four fields (`toolsListChanged`, `promptsListChanged`, `resourcesListChanged`, `resourceSubscriptions`) and no server-side MUST to implement the method, unlike `server/discover`. Ze's only pushed notification is `notifications/tasks/status` (`tasks.go:520`), which rides the deleted GET stream; UI resources are immutable (`plan/learned/682-mcp-5-apps.md`) | Phase 3 or a fifth phase must implement `subscriptions/listen` | Subscriptions pattern page read in full 2026-07-28: the filter is a closed four-field set, the server "**MUST NOT** send notification types the client has not explicitly requested", and no clause obliges a server to offer the method. Ze's tool list is fixed per config reload and its UI resources are immutable, so it has nothing to advertise | confirmed |
| A-7 | Surfacing Ze's internal event bus to MCP clients is a new feature, not a conformance obligation, and belongs outside this umbrella | `pkg/ze/eventbus.go` is an in-process typed pub/sub with ~25 registered `(namespace, eventType)` pairs; none of it is reachable from `internal/component/mcp/` today | If it were in scope, Phase 4 would grow a resource-modelling design and the cutover would stop being a pure conformance change | Owner question 2026-07-28, answered in Key Design Decisions below | confirmed |
| A-5 | Every tool handler that currently elicits can be restructured as a resumable, stateless function of (arguments + `inputResponses`) | `elicit.go:227` is called mid-dispatch from tool handlers; MRTR requires the retry to be processable with nothing beyond the retry request | Phase 2 needs server-side state after all, contradicting the stateless model, and the design has to change | Confirmed 2026-07-28, and more strongly than expected: a tree-wide grep for `\.Elicit(` finds exactly ONE call site, `tools.go:747`, inside the handcrafted `ze_execute` handler (`tools.go:722`). It elicits a single string named `command` and assigns the answer straight into `input.Command` (`tools.go:766`). The entire state suspended across the round trip is one tool argument that the retry carries anyway | confirmed |
| A-6 | `ttlMs` and `cacheScope` can be derived from static server knowledge without new config | The tool list changes only on daemon config reload; UI resources are immutable | Phase 4 needs a YANG config surface for cache lifetimes | Confirmed 2026-07-28: tool descriptors are generated from the YANG command registry at request time (`streamable_tools.go:49-64`), so they change only when the command surface does, and UI assets are served from an `//go:embed` filesystem (`ui/embed.go`) that is immutable for the process lifetime. Static constants are honest; Phase 4 fixes the values | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | No shipping MCP client speaks `2026-07-28` when the cutover lands, leaving Ze's MCP server unreachable by real clients even though it is conformant | `ze-test mcp` passes but a real client (Claude Code, Claude Desktop) cannot connect | Accepted, by owner decision. The spec's own compatibility matrix records Legacy client + Modern server as Fails. Mitigation is timing, not code: land the cutover, and treat client availability as a release-readiness question rather than a design question. Escalate to Thomas if it blocks a release |
| R-2 | The MRTR rewrite silently loses elicitation coverage, because the current tests assert on the SSE frame shape that no longer exists | Phase 2 deletes `elicit_test.go` (676L) and `task_elicit_test.go` (185L) and the replacement suite is thinner | Mutation-verify per `ai/rules/functional-test-gate.md`: disable the `InputRequiredResult` producer and confirm the new `.ci` tests go red. Count assertions before and after |
| R-3 | A future elicitation flow needs real cross-round-trip state, and someone adds `requestState` without the integrity design | A `requestState` field appears in an `InputRequiredResult` with no keyed MAC, principal binding, expiry or request digest behind it | Superseded in its original form. A-5 established that Ze's one elicitation carries no state, so Phase 2 emits `inputRequests` alone and omits `requestState`, and the cross-user replay primitive this row was written about cannot exist. The residual risk is the future case: Phase 2 records the trigger condition and the obligation that the full binding design (principal, expiry, originating-request digest, all inside the protected payload) is mandatory the moment the field is introduced, not optional |
| R-4 | The four phases land out of order or partially, leaving a server that is neither `2025-06-18` nor `2026-07-28` conformant | `make ze-verify` green with the MCP `.ci` tests in `test/plugin` disabled or thinned | Phase 1 is a single atomic cutover of the transport: it is not splittable, and no phase closes with the protocol version constant advanced but its methods unimplemented |
| R-5 | Deleting `session.go` breaks `ai/digests/mcp.md` anchors and the `// Design:` references in every MCP file | `make ze-digest-check` fails, or `make ze-doc-test` reports stale source anchors | Treat `docs/architecture/mcp/overview.md` and `ai/digests/mcp.md` as Phase 1 deliverables, not Phase 4 cleanup |
| R-6 | `ze-chaos` regresses, because its Provider mode is wired separately from the daemon and its `.ci` coverage lives in `test/chaos` and `test/chaos-web` rather than beside the other MCP tests in `test/plugin` | `test/chaos*` reds after Phase 1 | Both `ze-chaos` call sites are listed in Integration Points for every phase, and Phase 1's Deliverables Checklist names the chaos suites explicitly |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The MCP server stops answering, or answers non-conformantly. No routing, forwarding, or dataplane impact: the MCP listener is off by default (`ze-mcp-conf.yang`, `leaf enabled` default false) and is an operator-facing control surface, not a protocol Ze speaks to peers. `ze-chaos` loses its tool interface. |
| How is it reverted? | Single revert per phase while unreleased; the phases are sequenced so each leaves a conformant server for exactly one revision. Once a phase lands, the previous revision is gone by design, so a revert is a revert of the whole phase, not a config rollback. |
| Who else touches this path? | No other open spec names `internal/component/mcp/`. `ze-chaos` (`internal/chaos/orchestrator/`) and the `ze-test mcp` driver (`internal/test/cli/cmd_mcp.go`) are the two consumers outside the component. |

## Wiring Test (MANDATORY -- NOT deferrable)

The umbrella's own wiring assertion is that the daemon and `ze-chaos` both serve
the new revision end to end. Per-feature wiring rows live in the child specs.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze` daemon with `environment { mcp { enabled true; } }`, client POSTs `server/discover` | → | `startMCPServer` (`cmd/ze/hub/service_mcp.go:192`) → `Streamable.ServeHTTP` → discover handler | `test/plugin/mcp-discover-daemon.ci` |
| `ze-chaos` Provider-mode MCP listener, client POSTs `tools/list` with `2026-07-28` headers | → | `internal/chaos/orchestrator/run.go:535` → `Streamable.ServeHTTP` → `runMethod` | `test/chaos/chaos-mcp-tools-list.ci` |
| Client POSTs `initialize` (legacy client, modern server) | → | `Streamable.handlePOST` unknown-method path | `test/plugin/mcp-legacy-initialize-rejected.ci` |
| Client sends HTTP GET to `/mcp` | → | `Streamable.ServeHTTP` method dispatch | `test/plugin/mcp-get-method-not-allowed.ci` |

## Acceptance Criteria

Umbrella-level, aggregate. Each child spec carries its own AC table for the
mechanism it delivers.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any conformant `2026-07-28` request | Served without an `initialize` handshake and without any `Mcp-Session-Id` header being minted, required, or echoed |
| AC-2 | A request declaring protocol version `2025-06-18`, `2025-03-26`, or `2024-11-05` | Rejected with HTTP 400 and JSON-RPC `-32022`, whose `data.supported` lists exactly `["2026-07-28"]` |
| AC-3 | `server/discover` | Returns supported versions, capabilities including the negotiated `extensions` map, and `serverInfo` in `_meta` |
| AC-4 | HTTP GET or DELETE to `/mcp` | `405 Method Not Allowed` |
| AC-5 | Any successful result from any method | Carries a `resultType` field |
| AC-6 | A tool call that needs operator input | Returns `resultType: "input_required"` with `inputRequests`, and no JSON-RPC request is ever sent from server to client |
| AC-7 | A long-running tool call from a client declaring the tasks extension | Returns `resultType: "task"`; `tasks/get` polls it to a terminal state; `tasks/list` and `tasks/result` are unknown methods |
| AC-8 | `tools/list`, `resources/list`, `resources/read`, `server/discover` | Each result carries `ttlMs` and `cacheScope` |
| AC-9 | A POST whose `Mcp-Method` or `Mcp-Name` header disagrees with the body, or omits a required header | HTTP 400 with JSON-RPC `-32020` |
| AC-10 | `grep -rn "Mcp-Session-Id\|sessionRegistry\|elicitation/create" --include=*.go internal/ cmd/` | Returns no non-test production hit outside a compatibility-rejection path |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Points a `2026-07-28` MCP client at a running `ze` daemon and lists tools | POST `tools/list` → header validation → per-request auth → `runMethod` → registry-generated descriptors + cache hints | `test/plugin/mcp-tools-list-stateless.ci` |
| 2 | Runs a `ze` command through `tools/call` with bearer auth | POST `tools/call` → `Mcp-Name` validation → `authenticate` → `CommandDispatcher` → result with `resultType: "complete"` | `test/plugin/mcp-tools-call-stateless.ci` |
| 3 | Invokes a tool that needs a missing argument, supplies it, and gets the result | `tools/call` → `InputRequiredResult` + signed `requestState` → client retry with `inputResponses` → final result | `test/plugin/mcp-mrtr-elicit-roundtrip.ci` |
| 4 | Starts a long-running operation and polls it to completion | `tools/call` → `CreateTaskResult` → `tasks/get` × N → terminal state with result | `test/plugin/mcp-task-poll-to-completion.ci` |
| 5 | Opens a tool's embedded UI panel | `tools/list` `_meta.ui` → `resources/read` on a `ui://` URI → HTML bundle with cache hints | `test/plugin/mcp-ui-resource-read.ci` |
| 6 | Drives `ze-chaos` fault injection over MCP | `ze-chaos` Provider mode → `tools/call` → orchestrator | `test/chaos/chaos-mcp-tools-call.ci` |

## 🧪 TDD Test Plan

Umbrella level only. Per-mechanism unit and functional tests live in the child
specs.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestProtocolVersionIsCurrentRevision` | `internal/component/mcp/streamable_test.go` | `ProtocolVersion == "2026-07-28"` and the supported set contains nothing else | |
| `TestNoLegacyTransportSymbolsRemain` | `internal/component/mcp/streamable_test.go` | Compile-time absence of session registry construction from the transport path | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A at umbrella level; `ttlMs`, `pollIntervalMs` and `requestState` TTL boundaries are specified in phases 4, 3 and 2 respectively | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mcp-discover-daemon` | `test/plugin/*.ci` | A client discovers the daemon's capabilities before any other call | |
| `mcp-legacy-initialize-rejected` | `test/plugin/*.ci` | A `2025-06-18` client gets a diagnostic naming the supported version, not a silent hang | |
| `mcp-get-method-not-allowed` | `test/plugin/*.ci` | A legacy client's GET stream attempt is refused cleanly | |
| `chaos-mcp-tools-list` | `test/chaos/*.ci` | `ze-chaos` still exposes its tools after the cutover | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | MCP has no peer-daemon interop lab. The counterpart is an MCP client, and the only one available in-tree is `ze-test mcp`, which Ze also owns. Per `ai/rules/interop-and-goal-validation.md` this is the "no protocol peer involved" case for third-party daemons; the substitute obligation is that `ze-test mcp` be rewritten to the spec from the specification text rather than from Ze's implementation, so it is an independent reading. Recorded as R-1's residual risk | |

## Files to Modify

Umbrella-level inventory. Each child spec narrows this to its own phase.

- `internal/component/mcp/streamable.go` - transport rewrite
- `internal/component/mcp/streamable_tools.go` - method dispatch
- `internal/component/mcp/streamable_auth.go` - per-request auth call site
- `internal/component/mcp/tasks.go`, `task_state.go` - extension rework, principal scoping
- `internal/component/mcp/resources.go` - cache hints, per-request capability gate
- `internal/component/mcp/tools.go` - `resultType`, cache hints, `_meta.ui` extension shape
- `cmd/ze/hub/service_mcp.go` - drop dead session config fields
- `internal/chaos/orchestrator/run.go`, `cli.go` - Provider-mode wiring
- `internal/test/cli/cmd_mcp.go` - the MCP test client
- `cmd/ze/help_ai.go` - agent-facing wire-shape documentation
- `internal/component/mcp/yang/ze-mcp-conf.yang` - remove session-lifetime leaves
- `docs/architecture/mcp/overview.md`, `ai/digests/mcp.md` - subsystem design and digest

## Files to Create

- `plan/spec-mcp2026-1-stateless-core.md` .. `plan/spec-mcp2026-4-caching-apps.md` - the four child specs
- `internal/component/mcp/discover.go` - `server/discover` handler (Phase 1)
- `internal/component/mcp/meta.go` - per-request `_meta` parsing (Phase 1)
- `internal/component/mcp/headers.go` - standard request header validation (Phase 1)
- `internal/component/mcp/mrtr.go`, `requeststate.go` - MRTR and integrity-protected state (Phase 2)
- `test/plugin/*.ci` - the functional suite, rewritten across phases

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/mcp/yang/ze-mcp-conf.yang`: session-lifetime leaves become dead and are removed. No new leaves expected; A-6 revisits this in Phase 4 |
| YANG validation constraints | N-A | No new leaves planned |
| YANG custom validators | N-A | No new leaves planned |
| CLI commands/flags | Yes | `internal/test/cli/cmd_mcp.go`: `-elicit` and `-tasks` flags describe initialize-time capability declaration and must be reshaped to per-request capabilities |
| CLI grammar (keyword before value) | N-A | No new operator-facing command surface |
| Editor autocomplete | N-A | No new YANG leaves |
| Functional test for new RPC/API | Yes | `test/plugin/*.ci`; 19 `.ci` files currently reference MCP |
| Pipe completeness | N-A | MCP output is JSON-RPC over HTTP, not a CLI display surface |
| Env var registration | Yes | Any `environment/mcp/` leaf removed must have its `env.MustRegister` entry removed with it |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module, or certificate. The existing MCP listener check is unchanged |
| Prometheus counters/metrics | No | No new observable state proposed; session-count gauges (if any) die with sessions and must be checked during Phase 1 |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (MCP row names the protocol revision) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` if session-lifetime leaves are removed |
| 3 | CLI command added/changed? | Yes | `docs/functional-tests.md` for the `ze-test mcp` driver flags |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (method table: remove `initialize`, `tasks/list`, `tasks/result`; add `server/discover`, `tasks/update`) |
| 5 | Plugin added/changed? | No | MCP is a component, not a plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/mcp/overview.md`, `remote-access.md`, `elicitation.md`, `tasks.md`, `chaos.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/mcp/overview.md` is the wire/transport doc for this subsystem |
| 8 | Plugin SDK/protocol changed? | No | The plugin SDK is untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | MCP has no `rfc/short/` entry. `docs/features/mcp-integration.md` carries the support statement and must name `2026-07-28` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (MCP and MCP Apps rows) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/mcp/overview.md`, `ai/digests/mcp.md` |
| 13 | Route metadata keys added/changed? | N-A | Not a routing change |
| 14 | Prometheus counters added/changed? | No | Pending the Phase 1 check above |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registry entries change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Every file under `internal/component/mcp/` is anchored from `docs/architecture/mcp/overview.md`; 8 files currently mention `2025-06-18` (`docs/comparison.md`, `docs/functional-tests.md`, `docs/features.md`, `docs/architecture/api/commands.md`, `docs/architecture/mcp/overview.md`, `docs/features/mcp-integration.md`, `docs/guide/mcp/chaos.md`, `docs/guide/mcp/elicitation.md`) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `cmd/ze/help_ai.go:388-401` and `:551` embed JSON-RPC examples in agent-facing help |

## Implementation Steps

The umbrella is never implemented. These are the phase gates it enforces.

1. **Phase 1: Stateless core** (`spec-mcp2026-1-stateless-core`) - the atomic transport cutover. Not splittable: the handshake, sessions, GET stream and per-request metadata are one interlocking change. Closes with a conformant `2026-07-28` server minus MRTR, tasks and cache hints.
   - Gate: AC-1, AC-2, AC-3, AC-4, AC-5, AC-9, AC-10 demonstrated; `ze-chaos` suites green.
2. **Phase 2: MRTR** (`spec-mcp2026-2-mrtr`) - replaces elicitation. Depends on Phase 1 for the stateless request model that `requestState` exists to serve.
   - Gate: AC-6 demonstrated; R-2 mutation check recorded; R-3 negative tests per property.
3. **Phase 3: Tasks extension** (`spec-mcp2026-3-tasks-extension`) - depends on Phase 2, because a task in `input_required` surfaces `inputRequests` and is answered by `tasks/update`, reusing the Phase 2 types.
   - Gate: AC-7 demonstrated; A-4 resolved before design starts.
4. **Phase 4: Caching and Apps** (`spec-mcp2026-4-caching-apps`) - additive, depends on Phase 1 only. May run concurrently with Phase 3 if capacity allows.
   - Gate: AC-8 demonstrated.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the changelog's 9 major and 12 minor changes is either implemented by a child spec or has a Known Limitations row naming why it does not apply to a server-only implementation |
| Feature completeness | Each of the 6 user stories has an unbroken path after the phase that owns it |
| Correctness | No code path can mint, require, or echo `Mcp-Session-Id`; no code path can write a JSON-RPC request frame to a client |
| Naming | MCP wire keys stay camelCase (the documented exemption from `ai/rules/json-format.md`, `streamable.go:775-776`); Go identifiers and `_meta` keys use the reverse-DNS `io.modelcontextprotocol/*` form verbatim |
| Data flow | Authentication happens on every request, in one place, before dispatch. No handler re-authenticates and none skips it |
| Rule: `ai/rules/compatibility.md` | No shim, alias, or fallback for a dropped protocol revision survives the cutover. A rejection path that names the supported version is not a shim |
| Rule: `ai/rules/rfc-compliance.md` | Every MUST in the transports, versioning and MRTR pages maps to an AC in some child spec, or to an escalation to Thomas |
| Rule: `ai/rules/no-parking.md` | No phase closes with a method stubbed, a capability advertised but unimplemented, or a `.ci` test thinned to reach green |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Four child specs exist and are individually implementable | `ls plan/spec-mcp2026-*.md` returns 5 files |
| Protocol version constant advanced | `grep -n 'ProtocolVersion = ' internal/component/mcp/streamable.go` shows `2026-07-28` |
| No session machinery remains in production code | `grep -rn "sessionRegistry\|Mcp-Session-Id" --include=*.go internal/ cmd/ \| grep -v _test.go` is empty |
| No server-initiated request frames remain | `grep -rn '"method": *"elicitation/create"' --include=*.go internal/` is empty |
| Legacy revisions unreachable | `grep -n '2024-11-05\|2025-03-26\|2025-06-18' internal/component/mcp/*.go` returns only rejection-path or comment hits |
| Docs carry the new revision | `grep -rln "2025-06-18" docs/` is empty |
| Full gate | `make ze-verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authentication is not weakened by the cutover | Today identity binds at initialize and later requests are trusted by session-id alone (`streamable.go:391-401`). Per-request auth is strictly stronger, but confirm no path (notably the Provider-mode branch at `streamable.go:463-475`, which currently bypasses the session requirement entirely) ends up unauthenticated |
| `requestState` is attacker-controlled input | Phase 2. Integrity protection bound to principal, expiry and originating request; verification failure rejects rather than falls back |
| Header/body confusion | A load balancer routing on `Mcp-Method` while the server dispatches on the body is the exact attack `-32020` exists to close. Validation must run before dispatch, and must compare after Base64-sentinel decoding |
| Resource exhaustion after sessions are gone | Session caps (`MaxSessions`, `MaxSessionLifetime`) were the bound on concurrent client state. Confirm what bounds task count and in-flight requests once that bound is removed, and that the task registry's per-principal indexing does not become an unbounded map keyed by attacker-supplied identity |
| Error leakage | `-32022` publishes the supported version list by design. Confirm `-32020` messages do not echo unvalidated header bytes back to the client |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A spec MUST cannot be met as designed | STOP. Escalate to Thomas as "which way do I fix it" per `ai/rules/rfc-compliance.md`. Never narrow the requirement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The cutover is a net deletion of operational complexity. Sessions were the reason Ze needed TTL garbage collection, an absolute-lifetime cap, a max-session cap, a single-stream-per-session guard, an outbound queue per session, and a documented requirement for sticky load-balancer routing. All of that is protocol machinery with no replacement, because the protocol no longer has the concept.
- Provider mode (`ze-chaos`) already runs session-less (`streamable.go:463-475`) with a comment noting the MCP spec makes sessions optional. That branch is the closest thing in-tree to the target model, and is worth reading as a reference before Phase 1 rather than being treated as a special case to delete.
- The single hardest piece looked like A-5: today a tool handler *blocks* mid-dispatch inside `session.Elicit` and resumes in the same goroutine with the client's answer, and MRTR requires that flow to be re-entrant across two independent HTTP requests with no server-side continuation. Design-phase research collapsed it. There is exactly **one** elicit call site in the tree (`tools.go:747`), it lives in the handcrafted `ze_execute` handler, and the only thing it suspends is a single `command` string that is itself a tool argument (`tools.go:766`). The control-flow inversion is one branch, not a refactor of many handlers.
- That finding cascades. Because the round trip carries no server-side state, Ze can satisfy the MRTR "at least one of `inputRequests` or `requestState`" rule with `inputRequests` alone and omit `requestState` entirely, which deletes the whole integrity-protection design (keyed MAC, principal binding, expiry, request digest) from Phase 2. Umbrella R-3 was written to guard exactly that design; with no `requestState`, the risk it describes cannot occur. Phase 2 carries the residual obligation instead: the moment any future flow needs real state, the full binding design becomes mandatory and is not optional to skip.
- Nothing bounded by the session layer needs a replacement. `defaultMaxSessions` (`session.go:118`) bounded sessions, and after the cutover there is no long-lived per-client server state left at all: no GET stream, no server-initiated requests, tasks polled rather than streamed, `subscriptions/listen` not implemented. The surviving bounds are `maxRequestBody` (1 MB, `tools.go:672-673`) and the task registry caps (`tasks.go:16-17`).
- The cutover is cheaper at the edges than expected. There are no MCP Prometheus metrics to migrate (a grep for `metrics.`/`prometheus` over the component returns nothing), no session-lifetime YANG leaf or env var to remove (the MCP env vars are `internal/component/config/environment.go:63-69` and none is a session knob), and `SessionTTL` / `MaxSessions` / `MaxSessionLifetime` are never set by any caller, so those `StreamableConfig` fields are already dead code. What does need care is prose: three `ze-mcp-conf.yang` descriptions (lines 74, 79, 90) tell operators that scopes are attached to the session.
- One requirement was hiding underneath the one that vanished, and it is a real MUST. The elicitation client capability is no longer a bare presence flag: `2026-07-28` declares it as `elicitation: {"form": {}, "url": {}}`, an empty object means form-mode only (backwards compatibility), and "Servers **MUST NOT** send elicitation requests with modes that are not supported by the client." `ElicitRequest` gained a matching `mode` parameter (`form` or `url`, optional for form). Ze's gate is a bare-presence check (`streamable.go:734`), which would emit a form request to a client that declared url mode only. Phase 2 owns the fix. Shrinking a phase is not the same as finishing it, and this is what re-reading the page rather than trusting the changelog surfaced.
- Ze's MCP Apps metadata did not drift. `_meta.ui` carries `resourceUri`, `permissions` and `csp` (`tools.go:376-386`), which is exactly what the official Apps extension documents, and that extension still references the `2026-01-26` draft Ze was built against. Phase 4's Apps work is therefore negotiation only, not a reshaping of the descriptor.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Clean cutover to `2026-07-28` only | Dual-era server (both `initialize` and per-request metadata on one endpoint, explicitly permitted by the versioning page) | Owner decision 2026-07-28. `ai/rules/compatibility.md`: Ze is unreleased and MCP is not the plugin API, so no compatibility is owed. Dual-era would preserve the session registry, the SSE sink upgrade and the correlation map permanently alongside their replacements |
| Four sequenced child specs rather than one | A single cutover spec | `plan/learned/683-mcp-0-umbrella-modernization.md` records the phased approach as what prevented the partial-wiring trap on the previous MCP modernization. Each phase is independently verifiable |
| Phase 1 is atomic and not splittable | Splitting header validation, `server/discover` and session removal into separate phases | The handshake, sessions and per-request metadata are one interlocking mechanism: removing `initialize` without per-request `_meta` leaves no way to learn the client's version or capabilities |
| `ze-test mcp` is rewritten from the specification text, not from Ze's implementation | Adapting the existing client alongside the server | MCP has no third-party interop lab in-tree, so the test client is the only independent reading of the spec available. Deriving it from the server implementation would make every test tautological (`ai/rules/interop-and-goal-validation.md`, vacuity traps) |
| If Ze ever surfaces live state to MCP clients, it does so by modelling that state as MCP **resources** and letting `resourceSubscriptions` carry the change signal, not by piping the event bus through a Ze-defined notification type | (a) a Ze extension (`io.ze-software/events`) adding its own filter field and notification type, which the extension mechanism does permit, as the Tasks extension does with `notifications/tasks`; (b) no push at all, exposing events through a polling tool | Three reasons. **Shape:** `notifications/resources/updated` carries only the changed URI, so the client re-reads. That is naturally coalescing, which is what an agent wants and what a raw event stream is not. **Volume:** the bus carries `*BestChangeBatch` (`internal/core/bgp/ribevents/ribevents.go:157`), which is BGP best-path churn and can be thousands of events per second; SSE to an LLM client is the wrong sink for it. **Coupling:** bus payloads are internal typed Go values, and a Ze-specific notification type would freeze them into a public wire contract that no third-party client understands anyway (`ai/rules/plugin-design.md`, cross-boundary value types). Option (a) stays available if a first-party client ever justifies it |

## Known Limitations

- **`subscriptions/listen` is not implemented.** A-4 is confirmed: the method carries a closed four-field opt-in filter and no server-side MUST obliges Ze to offer it. Ze's only pushed notification is `notifications/tasks/status` (`tasks.go:520`), which rides the GET stream that Phase 1 deletes; polling via `tasks/get` is the spec's stated default, the tool list changes only on config reload, and UI resources are immutable (`plan/learned/682-mcp-5-apps.md`). So there is nothing for Ze to advertise on the stream today, and omitting it is conformant rather than a gap.
- **Live Ze state (event bus) is not surfaced to MCP clients.** This is a genuine new feature and is deliberately outside a conformance cutover (A-7). When it is wanted, the design is fixed by the Key Design Decision above: model the interesting state as MCP resources under a `ze://` URI space, subscribe with `resourceSubscriptions`, and let the event bus fire `notifications/resources/updated` so the agent re-reads. The low-rate transitions are the useful ones (protocol session up/down, VRRP state change, interface state); the high-rate route batches are not. Recorded in `plan/deferrals/mcp2026-0-umbrella.md` with `plan/spec-mcp2026-5-state-resources.md` as its destination.
- **Prompts are not implemented**, before or after this work. `runMethod` has never handled `prompts/*` (`streamable_tools.go:22-45`). The cutover does not change that, and the spec does not require it: `prompts` is an advertised capability, not a mandatory one. Out of scope, no deferral row, because it is pre-existing and unrelated to the revision change.
- **Roots, Sampling and Logging remain unimplemented**, which is now the recommended position: all three are Deprecated in `2026-07-28` under the feature-lifecycle policy, and new implementations are told not to adopt them.

## RFC Documentation (Scope: protocol)

MCP is a specification rather than an IETF RFC, so `rfc/short/` carries no entry
and `make ze-rfc-check` does not gate it. The equivalent obligation still
applies: add `// MCP 2026-07-28 <page> Section X: "<quoted requirement>"` above
each enforcing code path. MUST document at minimum: header validation and
mismatch rejection, version rejection, the prohibition on server-initiated
requests, the prohibition on client-sent responses, `requestState` integrity
verification, and the client-capability precondition on `inputRequests`.

Whether MCP should be enrolled in the `rfc/` ledger machinery so its MUSTs get
the same coverage ratchet as protocol RFCs is a question for Thomas, raised in
`plan/deferrals/mcp2026-0-umbrella.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
