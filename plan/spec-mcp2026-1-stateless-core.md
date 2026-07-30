# Spec: mcp2026-1-stateless-core

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/4 |
| Deferral shard | `plan/deferrals/mcp2026-1-stateless-core.md` |
| Updated | 2026-07-30 |

Parent: `plan/spec-mcp2026-0-umbrella.md`. Siblings: `plan/spec-mcp2026-2-mrtr.md`,
`plan/spec-mcp2026-3-tasks-extension.md`, `plan/spec-mcp2026-4-caching-apps.md`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Cut Ze's MCP transport over from protocol revision `2025-06-18` to `2026-07-28`.
This phase is **atomic and not splittable**: the handshake, sessions, the GET
stream and per-request metadata are one interlocking mechanism, and removing any
one of them without the others leaves a server that cannot learn the client's
version or capabilities.

**Delete:**

| What | Where |
|------|-------|
| `initialize` and `notifications/initialized` handling | `streamable.go:428-451`, `:700-713`, `:777-798`; `streamable_tools.go:24-25` |
| The entire session layer | `session.go` (570L): `sessionRegistry`, TTL GC, `MaxSessions`, `MaxSessionLifetime`, the outbound queue, `streamActive` |
| `Mcp-Session-Id` minting, requirement and echo | `streamable.go:448`, `:457-483`, `:624`, `:683` |
| `handleGET` SSE stream endpoint | `streamable.go:618-678` |
| `handleDELETE` | `streamable.go:681-694` |
| Version negotiation from initialize params | `parseInitializeProtocolVersion` (`streamable_tools.go:338-368`) |
| The Provider-mode session-less special case | `streamable.go:463-475`. Deleted outright, not migrated: the session-less shape becomes the only shape, so the branch has nothing left to distinguish. D-2 records why this does not weaken `ze-chaos` auth |
| Elicitation transport and its `*session`-typed API | `elicit.go` (302L, `(*session).Elicit` at `:227`), `handleElicitResponse` (`streamable.go:562-605`), `TaskElicit` (`tasks.go:456-513`), the `ze_execute` elicit arm (`tools.go:740-766`), and the capability claim in the tool description (`tools.go:836`). Forced by A-4, not chosen |
| Task session coupling | The `sessionID` field (`tasks.go:36`, set `:120`), `CancelAllForSession` (`:277`), and `buildTaskStatusNotification` (`:517`). The registry, `byIdentity` and every cap stay |

**Add:**

| What | Requirement |
|------|-------------|
| Per-request `_meta` parsing | `io.modelcontextprotocol/protocolVersion`, `clientInfo`, `clientCapabilities` on every request. **Corrected 2026-07-29 from the specification text** (`tmp/mcp-2026-07-28-spec-extract.md`, PART E): `_meta` sits **inside `params`**, not at the JSON-RPC message top level; `protocolVersion` and `clientCapabilities` are **required**, `clientInfo` is not (clients only SHOULD send it) |
| `-32602` on a malformed `_meta` | **Added 2026-07-29.** "A request missing any required field is malformed; the server **MUST** reject it with JSON-RPC error code `-32602` (Invalid params). On HTTP, the response status **MUST** be `400 Bad Request`." This is a *different* failure from a header mismatch and must not be collapsed into `-32020` |
| `-32021 MissingRequiredClientCapabilityError` | **Added 2026-07-29.** "A server **MUST NOT** rely on capabilities the client has not declared. If processing a request requires a capability the client did not include in `io.modelcontextprotocol/clientCapabilities`, the server **MUST** return a `MissingRequiredClientCapabilityError` (`-32021`) whose `data.requiredCapabilities` lists the missing capabilities. On HTTP, the response status **MUST** be `400 Bad Request`." This replaces the pre-cutover `-32601` capability gate on resources and tasks |
| `io.modelcontextprotocol/serverInfo` on every result | **Added 2026-07-29.** "Servers **SHOULD** include the following `io.modelcontextprotocol/*` field in **every result's** `_meta`". Emit it from the shared `ok()` helper beside `resultType`, so one site covers every method rather than only `server/discover` |
| Per-request authentication | Today identity binds at initialize and later requests are trusted by session-id validity alone (`streamable.go:391-401`). Every request now authenticates. `authenticator.Authenticate` already takes an `*http.Request` (A-1, confirmed), so this is a call-site move, not an interface change |
| `MCP-Protocol-Version` header validation | MUST equal the body's `_meta` version; mismatch or absence returns HTTP 400 + `-32020` |
| `Mcp-Method` header validation | MUST equal `method` |
| `Mcp-Name` header validation | MUST equal `params.name` (`tools/call`) or `params.uri` (`resources/read`), compared **after** decoding the `=?base64?...?=` sentinel form |
| `Mcp-Param-{Name}` validation | Only if `tools.go` ever emits `x-mcp-header` annotations; it does not today, so the server side is the reject-on-mismatch half only |
| `-32022 UnsupportedProtocolVersionError` | HTTP 400 with `data.supported` listing exactly `["2026-07-28"]` and `data.requested` echoing the client's value |
| `server/discover` | Servers **MUST** implement it. Returns `supportedVersions`, `capabilities` (including the `extensions` map), `instructions`, and `serverInfo` under `_meta` |
| `resultType: "complete"` | On every successful result from every method |
| Unknown method | HTTP **404** (not 200) with JSON-RPC `-32601`, so a client can distinguish a modern server from a legacy HTTP+SSE one |
| GET / DELETE to `/mcp` | HTTP 405; `Mcp-Session-Id` and `Last-Event-ID` headers ignored, never echoed |

**Constraints carried from the parent:**

- `ProtocolVersion` becomes `2026-07-28` and `supportedProtocolVersions` (`streamable.go:49-53`) contains nothing else. No shim, alias or fallback for a dropped revision survives (`ai/rules/compatibility.md`).
- Elicitation is **not** reshaped here. **Decided:** Phase 1 fails closed through the branch that already exists, and deletes the elicit machinery outright. See "Elicitation in Phase 1" below.
- Tasks keep working through Phase 1 in reduced form. **Decided:** they are re-scoped to principal, because the registry is already principal-indexed. See "Tasks in Phase 1" below.
- `ze-chaos` (`internal/chaos/orchestrator/run.go:535`, `cli.go:635`) and the `ze-test mcp` client (`internal/test/cli/cmd_mcp.go`) are updated in this phase, not after it.
- `docs/architecture/mcp/overview.md` and `ai/digests/mcp.md` are Phase 1 deliverables (R-5): deleting `session.go` breaks their anchors and `make ze-digest-check`.

**Design decisions taken for this phase** (these were the open questions; all
three are now answered from source, and none remains open):

**D-1. Nothing replaces `MaxSessions`, because the object it bounded ceases to
exist.** `defaultMaxSessions = 1024` (`session.go:118`) bounded *sessions*, and
after the cutover Ze holds no long-lived per-client server state at all: the GET
stream is deleted, MRTR terminates each request at the response (Phase 2), tasks
are polled rather than streamed (Phase 3), and `subscriptions/listen` is not
implemented (umbrella A-4, confirmed). A session cap with nothing to count is not
a bound that was lost, it is a bound that no longer has a subject. Inventing a
replacement cap would be inventing state. The bounds that actually survive and
must be named in the Security Review are these:

| Surviving bound | Value | Producer | Enforced at | Scope |
|-----------------|-------|----------|-------------|-------|
| Request body size | 1 MB (`maxRequestBody = 1 << 20`, `tools.go:673`) | `streamable.go:152-155` selects it whenever `MaxBodyBytes` is zero | `streamable.go:412` (`http.MaxBytesReader`), HTTP 413 at `:415` | per request |
| Concurrent tasks | 8 (`defaultMaxConcurrentTasks`, `tasks.go:16`) | `activeCount` (`tasks.go:311-312`), called by `taskRegistry.Create` (`:103`) | `tasks.go:104-106`, returns `errTaskConcurrencyCap` | **per principal** |
| ~~Retained terminal tasks~~ **NO SUCH BOUND -- corrected 2026-07-29** | ~~128~~ | `defaultMaxTerminalTasks` (`tasks.go:17`) is assigned to `maxTerminal` (`tasks.go:98`) and **never read anywhere** | nothing: `sweep()` (`tasks.go:350-371`) reaps solely on TTL (`:357-358`) | -- |
| Task lifetime | default 5 min, clamped to 1 s .. 1 h (`tasks.go:18-20`) | `clampTTL` (`tasks.go:332`), called from `Create` (`:99`) | task GC (`taskGCInterval`, `tasks.go:21`) | per task |
| Concurrent in-flight requests | no Ze-level cap | Go `net/http` server accept loop | connection layer | process |

Two consequences worth stating rather than leaving implicit. First,
`StreamableConfig.MaxBodyBytes` (`streamable.go:73`) is never set by any caller
(no `MaxBodyBytes:` literal exists outside the field declaration), so the
effective body cap is always the 1 MB default; it survives the cutover as the
only per-request size bound and must not be deleted alongside the session fields.
Second, the umbrella's Security Review worry that `byIdentity` (`tasks.go:52`)
"becomes an unbounded map keyed by attacker-supplied identity" does not
materialize: the key is `sess.Identity().Name`
(`streamable_tools.go:179`, `:198`, `:211`), which after the cutover comes from
the per-request authenticator, never from the request body. Under `auth-mode
none` every caller collapses to the single empty-name key, because
`noneAuthenticator.Authenticate` returns a zero `Identity`
(`bearer.go:41-43`); under `bearer` and `bearer-list` the key space is exactly
the configured identity list; under `oauth` it is bounded by what the external
authorization server will issue. The map is bounded by the credential surface,
not by request content, and the per-principal concurrency cap of 8 bounds each
key's contents.

**D-2. The Provider-mode branch is not special-cased, it disappears, and
`ze-chaos` ends up on exactly the same per-request auth path as everything
else, with no observable change.** Today `streamable.go:463-475` runs `runMethod`
without calling `s.authenticate(r)`. That reads like a bypass, but `ze-chaos` is
unauthenticated *by configuration*, not by that branch: it constructs the server
with `Provider` set and no `Token` and no `AuthMode`
(`internal/chaos/orchestrator/run.go:535`, `internal/chaos/orchestrator/cli.go:635`),
so `NewStreamable`'s mode inference (`streamable.go:167-176`) selects `AuthNone`,
whose authenticator accepts every request with a zero `Identity`
(`bearer.go:41-43`). After the cutover every request authenticates uniformly, and
under `AuthNone` that means every `ze-chaos` request still succeeds. The branch is
therefore deleted, not migrated, and the Security Review row is satisfied by
construction rather than by a carve-out: there is no code path left that can
reach `runMethod` without passing through `authenticate`.

**D-3. No env var and no YANG leaf is removed. Three YANG `description` strings
are reworded, and three dead Go config fields are deleted.** The MCP env vars are
registered at `internal/component/config/environment.go:63-72`
(`ze.mcp.listen`, `enabled`, `token`, `bind-remote`, `auth-mode`,
`oauth.authorization-server`, `oauth.audience`, `oauth.required-scopes`,
`tls.cert`, `tls.key`) and not one of them is a session TTL, session cap, or
lifetime knob. `internal/component/mcp/yang/ze-mcp-conf.yang` likewise declares
no session, TTL, or lifetime leaf: the only three occurrences of the word
"session" in that schema are prose inside `description` strings, at lines 74, 79
and 90, each saying that scopes and principal names are "attached to the session"
or "carried on session". Those are user-facing text that becomes wrong the moment
identity is per-request, so all three are reworded to say the scopes and name ride
the authenticated request. The Go side is where the deletion happens:
`StreamableConfig.SessionTTL` (`streamable.go:72`), `MaxSessions` (`:76`) and
`MaxSessionLifetime` (`:81`) are already dead (no caller sets any of them), and
they go with the session layer, taking the `MaxSessionLifetime >= SessionTTL`
consistency guard at `streamable.go:148-151` with them.

**Elicitation in Phase 1: fail closed on the branch that already exists, then
delete the machinery.** Phase 2 research established there is exactly one
`session.Elicit` call site, `tools.go:747`, inside the `ze_execute` handler
(`tools.go:722`). It is already guarded: `tools.go:737-738` returns
`ErrResult("missing required argument: command")` when `s.session == nil` or the
client did not declare the elicitation capability. So "fail closed with a clear
error until Phase 2" is not a new branch, it is the branch that already runs, and
Phase 1 makes it unconditional by deleting the elicit arm below it. The rest of
the elicit machinery cannot survive the cutover for a compile reason rather than a
policy one: `Elicit` is a method on `*session` (`elicit.go:227`), `TaskElicit`
takes a `*session` (`tasks.go:464`), and `handleElicitResponse` takes a
`*session` (`streamable.go:562`), so none of them can compile once the type is
gone. All three are deleted here, which is why `elicit.go` joins Files to Delete
rather than waiting for Phase 2. `TaskElicit` is exported and has no non-test
caller today, so removing it loses no reachable behavior. The tool description at
`tools.go:836` advertises the elicitation prompt to clients and is reworded in the
same change, because leaving it would advertise a capability that no longer
exists.

**Tasks in Phase 1: kept, and re-scoped to principal, because they already are.**
The registry is indexed by identity, not by session: `byIdentity`
(`tasks.go:52`), populated at `tasks.go:128-131`, read by `List` (`:198-200`) and
by `activeCount` (`:311-312`), which is what the per-identity concurrency cap
counts (called from `Create` at `:103`).
The four handlers use the session for exactly two things, and both have a
per-request replacement: the capability gate `ClientSupportsTasks()`
(`streamable_tools.go:176`, `:189`, `:204`, `:219`), which becomes the
per-request `_meta.clientCapabilities`, and `sess.Identity().Name`
(`:179`, `:198`, `:211`), which becomes the per-request authenticated identity.
So `tasks/list`, `tasks/get`, `tasks/result` and `tasks/cancel` all keep working
through Phase 1 and Phase 3 reshapes them into the extension. What does go is
the session coupling: the `sessionID` field on the task entry (`tasks.go:36`,
set at `:120`) and `CancelAllForSession` (`tasks.go:277`), whose only trigger
was session expiry, plus the `notifications/tasks/status` push
(`buildTaskStatusNotification`, `tasks.go:517`), which rode the deleted GET
stream and has no successor before Phase 3's polling model.

Closes with a conformant `2026-07-28` server minus MRTR, the Tasks extension and
cache hints. Umbrella ACs covered: AC-1, AC-2, AC-3, AC-4, AC-5, AC-9, AC-10.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/mcp/overview.md` - the subsystem design doc this phase rewrites
  → Constraint: every `.go` file under `internal/component/mcp/` names this file in its `// Design:` line, so deleting `session.go` and `elicit.go` requires this doc rewritten in the same commit (`ai/rules/design-doc-references.md`, R-5).
- [ ] `ai/rules/compatibility.md` - no shim for a dropped revision
  → Decision: `supportedProtocolVersions` (`streamable.go:49-53`) drops `2025-03-26` and `2024-11-05` and the `LegacyProtocolVersion` constant (`streamable.go:39`) is deleted rather than retained as a rejection label.
- [ ] `ai/rules/fail-closed-guards.md` - header and version validation are guards; a missing header must deny, never default
  → Constraint: absence and mismatch are the same verdict. A missing `MCP-Protocol-Version` may not fall back to a default revision the way `LegacyProtocolVersion` does today; it returns `-32020`.
  → Constraint: ~~the pre-cutover code carries a live instance of the zero-value trap this rule names, at `resources.go:136-137` and `:143-144` (see Design Insights).~~ **Superseded 2026-07-29:** the nil dereference was fixed on 2026-07-29 by commit `e53e2f24f` ("fix(mcp): resources handlers deny a nil session, not panic"), one day after this spec was written. `resources.go:142` and `:151` now read `if sess == nil || !sess.ClientSupportsResources()`, and Go's `||` short-circuit means the method is never called on a nil receiver. The rewrite must still not reintroduce an "optional session" shape under a new name; that obligation is unchanged. What changed is that it is now a regression guard, not a bug fix.

### Protocol Specification (Scope: protocol)
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http` - the binding, header table, `-32020` server validation, Base64 sentinel encoding
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning` - `-32022`, the `server/discover` MUST, the era matrix
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/server/discover` - `DiscoverResult` shape
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/basic/index#meta` - `_meta` key naming rules and the reserved `io.modelcontextprotocol/*` prefix

**Key insights:** (minimal context to resume after compaction)

- **Per-request auth is a call-site move, not an interface change.** A-1 is
  confirmed: `authenticator.Authenticate(r *http.Request) (Identity, *authError)`
  (`auth.go:108-112`) and all four implementations already take the request
  (`bearer.go:41`, `:57`, `:97`; `oauth.go:42`). The only thing that changes is
  *when* `s.authenticate(r)` (`streamable.go:396`) is called: today only on the
  `initialize` branch (`streamable.go:428`), afterwards on every POST before
  dispatch. Do not reopen this as an auth refactor.
- **The blast radius outside the component is one file.** A-2 is confirmed: the
  only non-test production references to session semantics outside
  `internal/component/mcp/` are in `internal/test/cli/cmd_mcp.go` at lines 461,
  476, 499, 579 and 613. `cmd/ze/hub/service_mcp.go` and the two `ze-chaos` call
  sites never mention a session.
- **Nothing needs to replace `MaxSessions`, and nothing needs removing from the
  config surface.** D-1 and D-3 in the Task section carry the reasoning and the
  table of bounds that actually survive. The short form: sessions were the only
  long-lived per-client state, the surviving caps are per-request (1 MB body) and
  per-principal (8 concurrent tasks), and no env var or YANG leaf is a session
  knob, so only three YANG `description` strings (lines 74, 79, 90) are reworded.
- **Elicitation already fails closed.** `tools.go:737-738` returns
  `ErrResult("missing required argument: command")` when `s.session == nil`, so
  the "fail closed until Phase 2" requirement is satisfied by the branch that
  already exists. `elicit.go` is nonetheless deleted in Phase 1, because
  `(*session).Elicit` (`elicit.go:227`), `TaskElicit` (`tasks.go:464`) and
  `handleElicitResponse` (`streamable.go:562`) all take `*session` and cannot
  compile without it.
- **Tasks are already principal-scoped.** `byIdentity` (`tasks.go:52`) and the
  per-identity concurrency count (`tasks.go:103`) mean `tasks/*` survives Phase 1
  unchanged in substance; only the `sessionID` field (`tasks.go:36`),
  `CancelAllForSession` (`tasks.go:277`) and the status-notification push
  (`tasks.go:517`) are session-coupled and go.
- **None of the new wire surface exists yet.** A tree-wide grep for `32020`,
  `32022`, `resultType` and `server/discover` over `internal/component/mcp/`
  excluding tests returns nothing. Every one of them is new code, not an edit.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/mcp/streamable.go` - the transport being rewritten
- [ ] `internal/component/mcp/session.go` - the layer being deleted
- [ ] `internal/component/mcp/streamable_tools.go` - method dispatch
- [ ] `internal/component/mcp/streamable_auth.go` - the authenticator the call site moves for
- [ ] `internal/component/mcp/auth.go` - `authenticator` interface, to validate A-2
- [ ] `internal/test/cli/cmd_mcp.go` - the client being rewritten
- [ ] `cmd/ze/hub/service_mcp.go` - daemon wiring and dead config fields

**Behavior to preserve:**
- Origin validation and the loopback-only default (`streamable.go:371-385`).
- All four auth modes and their YANG surface. `noneAuthenticator` returning a zero `Identity` with no error (`bearer.go:41-43`) is load-bearing for `ze-chaos` (D-2) and must not become a rejection.
- The RFC 9728 protected-resource metadata endpoint (`streamable.go:335-364`).
- Tool descriptor generation from the YANG command registry (`tools.go`).
- Resource-not-found returning `-32602` (`resources.go:158-163`).
- The 1 MB request-body cap: `maxRequestBody` (`tools.go:673`), defaulted at `streamable.go:152-155`, enforced at `streamable.go:412`. `StreamableConfig.MaxBodyBytes` (`streamable.go:73`) is unset by every caller but is the knob for it and stays.
- Per-principal task scoping and its caps: `byIdentity` (`tasks.go:52`), `activeCount(identity)` against `maxConcurrent` (`tasks.go:103-106`), `maxTerminal` (`tasks.go:85`) and the TTL clamp (`clampTTL`, `tasks.go:332`).

**Behavior to change:**
- Everything in the Task tables above.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- HTTP POST to `/mcp` carrying one JSON-RPC request or notification, with `MCP-Protocol-Version`, `Mcp-Method`, and conditionally `Mcp-Name` headers mirroring body fields.

### Transformation Path
1. `ServeHTTP`: RFC 9728 metadata path, Origin allowlist, method dispatch (POST only; GET/DELETE become 405).
2. Header validation against the body, before dispatch.
3. Version check against the supported set.
4. Authentication, on every request.
5. `runMethod` dispatch.
6. Result assembly with `resultType`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| MCP client ↔ Ze transport | JSON-RPC over HTTP POST | No |
| Ze MCP ↔ authenticator | `Authenticate(*http.Request) (Identity, *authError)`, call site moved to per-request | No |
| Ze MCP ↔ `ze-chaos` Provider | `ToolProvider`; the session-less branch becomes the only branch | No |

### Integration Points
- `cmd/ze/hub/service_mcp.go:192` (`startMCPServer`)
- `internal/chaos/orchestrator/run.go:535`, `internal/chaos/orchestrator/cli.go:635`
- `internal/test/cli/cmd_mcp.go`
- `cmd/ze/help_ai.go:388-401`, `:551`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `authenticator` needs no signature change to run per-request | `streamable.go:396-401` | An auth refactor lands inside the transport cutover | Read 2026-07-28: the interface declares `Authenticate(r *http.Request) (Identity, *authError)` (`auth.go:108-112`) and all four implementations already take the request: `noneAuthenticator` (`bearer.go:41`), `bearerAuthenticator` (`bearer.go:57`), `bearerListAuthenticator` (`bearer.go:97`), `oauthAuthenticator` (`oauth.go:42`). Moving auth to per-request is a call-site move only | confirmed |
| A-2 | No code outside `internal/component/mcp/` and the three wiring sites depends on session semantics | Umbrella A-3 | Phase estimate is wrong | Run 2026-07-28: `grep -rn "Mcp-Session-Id\|sessionRegistry\|ClientSupportsTasks\|ClientSupportsResources\|ClientSupportsElicit" --include=*.go internal/ cmd/ pkg/`, minus `_test.go` and minus `internal/component/mcp/`, returns hits in exactly one file: `internal/test/cli/cmd_mcp.go` at lines 461, 476, 499, 579, 613. No production code outside the component depends on session semantics | confirmed |
| A-3 | Tasks can be re-scoped from session to principal without a registry redesign | `tasks.go:52` indexes `byIdentity`; `tasks.go:103` counts the concurrency cap per identity; the handlers already pass `sess.Identity().Name` (`streamable_tools.go:179`, `:198`, `:211`) | Phase 1 must delete `tasks/*` and Phase 3 reintroduces it from scratch, widening both phases | Read 2026-07-28 (see "Tasks in Phase 1"). The session supplies only the capability bit and the identity string, and both have per-request replacements | confirmed |
| A-4 | `elicit.go`, `TaskElicit` and `handleElicitResponse` cannot be deferred to Phase 2 | `(*session).Elicit` (`elicit.go:227`), `TaskElicit(..., sess *session, ...)` (`tasks.go:464`) and `handleElicitResponse(..., sess *session, ...)` (`streamable.go:562`) all name the `session` type in their signatures | Phase 1 does not compile after `session.go` is deleted, and the phase is not atomic after all | Read 2026-07-28. All three are deleted in Phase 1; `TaskElicit` is exported with no non-test caller, so nothing reachable is lost | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | ~~Removing session caps removes the only bound on concurrent client state~~ **RESOLVED at design time by D-1.** The premise was wrong: `defaultMaxSessions` (`session.go:118`) bounded sessions, and after the cutover there is no long-lived per-client state to bound. The surviving bounds are the 1 MB body cap (`tools.go:673`, enforced `streamable.go:412`) and the per-principal task caps (`tasks.go:16-17`, enforced `tasks.go:104`) | n/a, closed | n/a, closed. The residual obligation is a review one, carried into the Security Review Checklist: confirm no *new* unbounded per-client structure is introduced by the rewrite |
| R-2 | Header validation runs after dispatch, leaving the header/body confusion attack open | Review traces dispatch before validation | Validation is a `ServeHTTP`-level guard, not a per-handler check |
| R-3 | The rewrite reintroduces optional-ness under a new name: a nil-able request context, an "anonymous" identity sentinel, or a capability struct whose zero value reads as "supported" | A handler dereferences a per-request struct without the compiler forcing it to exist, exactly as `resources.go:136-137` dereferences a nil `*session` today | Per-request identity and capabilities are non-pointer values built before dispatch and passed by value. A zero `Identity` means "authenticated as anonymous under `auth-mode none`", never "unauthenticated"; unauthenticated is an early return, not a value (`ai/rules/fail-closed-guards.md`) |
| R-4 | `elicit.go` and `TaskElicit` are deleted in Phase 1, and Phase 2 rewrites the flat-primitive schema validator (`elicit.go:102-194`) from scratch instead of recovering it | Phase 2 opens with a fresh schema validator and no reference to the deleted one | Commit A of this spec preserves the file in history by design (`ai/rules/spec-preservation.md`). Phase 2's Required Reading must name the deleted path so the validator is recovered rather than reinvented |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The MCP server stops answering or answers non-conformantly. No dataplane impact; the listener is off by default |
| How is it reverted? | Single revert of the phase |
| Who else touches this path? | `ze-chaos`, `ze-test mcp` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Client POSTs `server/discover` to a running daemon | → | `startMCPServer` → `Streamable.ServeHTTP` → discover handler | `test/plugin/mcp-discover-daemon.ci` |
| Client POSTs `tools/list` with conformant headers and `_meta` | → | header validation → per-request auth → `runMethod` | `test/plugin/mcp-tools-list-stateless.ci` |
| Client POSTs `initialize` | → | ~~unknown-method path~~ **header validation** (`headers.go`), which runs before dispatch and rejects the header-less legacy request first (corrected 2026-07-29, delta 6) | `test/plugin/mcp-legacy-initialize-rejected.ci` |
| Client sends HTTP GET to `/mcp` | → | `ServeHTTP` method dispatch | `test/plugin/mcp-get-method-not-allowed.ci` |
| `ze-chaos` Provider-mode listener answers `tools/list` | → | `run.go:535` → `Streamable.ServeHTTP` | ~~`test/chaos/chaos-mcp-tools-list.ci`~~ **corrected 2026-07-29:** `test/chaos-web/mcp-status.ci` step 2, which already asserts `tools/list`. Both existing MCP chaos tests live in `test/chaos-web/`, not `test/chaos/` (which holds four simulation smoke tests and no HTTP checks). **`test/chaos-web/` is NOT a `ze-verify` stage** — `ze-functional-test` does not run it and `stagesForMode` has no chaos stage — so `make ze-chaos-test` must be run explicitly to close this row. This is umbrella R-6, confirmed |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Conformant `2026-07-28` request | Served with no handshake and no `Mcp-Session-Id` minted, required or echoed |
| AC-2 | Request declaring `2025-06-18` | HTTP 400, `-32022`, `data.supported == ["2026-07-28"]` |
| AC-3 | `server/discover` | Returns `supportedVersions`, `capabilities`, `serverInfo` in `_meta`, `resultType: "complete"` |
| AC-4 | HTTP GET or DELETE to `/mcp` | 405 |
| AC-5 | Any successful result | Carries `resultType` |
| AC-6 | `Mcp-Method` disagrees with body `method` | HTTP 400, `-32020` |
| AC-7 | `Mcp-Name` absent on `tools/call` | HTTP 400, `-32020` |
| AC-8 | `Mcp-Name` carries `=?base64?...?=` matching the body value | Accepted after decoding |
| AC-9 | `MCP-Protocol-Version` header disagrees with `_meta` version | HTTP 400, `-32020` |
| AC-10 | Unknown method | HTTP 404 with `-32601` |
| AC-11 | Request with no credential under `auth-mode bearer` | 401, on **every** method, not only the first |
| AC-12 | `grep -rn "sessionRegistry\|Mcp-Session-Id" --include=*.go internal/ cmd/` excluding tests | No production hit outside a rejection path |
| AC-13 | `resources/list` and `resources/read` sent by a client whose `_meta.clientCapabilities` is `{}` (the conformant shape every spec example uses) | **RESTATED 2026-07-29 after independent review.** Both are **served normally**, with `resultType: "complete"`. The previous wording -- a `-32021` capability gate -- was wrong and was a functional break: `2026-07-28` `ClientCapabilities` has exactly five members (`experimental`, `roots`, `sampling`, `elicitation`, `extensions`) and **`resources` is not one of them**; it is a *ServerCapabilities* member. Gating on it meant no conformant client could ever read a resource, while `server/discover` advertised `capabilities.resources` and `tools/list` published `_meta.ui.resourceUri` pointing at `ui://` assets -- so Ze instructed clients to fetch what it then refused. `data.requiredCapabilities` is additionally typed `ClientCapabilities` in the schema, so `{"resources":{}}` was not even a legal value of that field. The gate is deleted; `resourcesList`/`resourcesRead` no longer take a capability argument at all, so it cannot be reintroduced by accident |
| AC-14 | Two different principals each start 8 concurrent tasks under `auth-mode bearer-list` | Both reach 8; the 9th for either is refused with the concurrency-cap error. Proves the surviving bound is per principal (`tasks.go:103-106`), not per session and not global, which is what D-1 relies on |
| AC-15 | `ze_execute` called with no `command` argument, from any client | HTTP 200 with a tool error naming the missing argument (`tools.go:738`), and no `elicitation/create` frame is ever produced. Proves elicitation fails closed rather than half-wired (`ai/rules/no-parking.md`). **PARTLY SUPERSEDED BY PHASE 2, 2026-07-30.** "from any client" and the unconditional half of this row no longer hold: Phase 2 (`spec-mcp2026-2-mrtr`) restored elicitation as Multi Round-Trip Requests, so a client declaring **form-mode elicitation** that omits `command` now gets `resultType: "input_required"` rather than an error. The row survives unchanged for every OTHER client -- one declaring nothing, or declaring `url` mode only -- which is the branch `askForCommand` (`mrtr.go`) still takes and which `test/plugin/mcp-execute-missing-command.ci` and `TestExecuteWithoutCommandFailsClosed` still gate. **What Phase 2 missed and this correction fixes:** the *published* `inputSchema` kept `"required": ["command"]` and a description asserting the unconditional error, written for this AC. That contract made Phase 2's only user story unreachable through the advertised interface -- a schema-validating host will not construct a call its own schema rejects. The descriptor is now derived from the same capability the handler branches on (`gateExecuteCommandRequired`, `mrtr.go`), gated by `TestExecuteDescriptorMatchesElicitationCapability` |
| AC-16 | A request whose `params._meta` omits `io.modelcontextprotocol/protocolVersion`, or omits `io.modelcontextprotocol/clientCapabilities` | HTTP 400 with JSON-RPC `-32602`. **Added 2026-07-29**, delta 1. Distinct from AC-9: a missing `_meta` field is not a header mismatch, and collapsing the two into `-32020` would emit a reserved-range code with a meaning the specification does not give it (`basic/index`: implementations "**MUST** use defined codes only with their specified meanings"). `clientInfo` absent is **not** an error |
| AC-17 | Any successful result from any method | Carries `_meta["io.modelcontextprotocol/serverInfo"]` with `name` and `version`. **Added 2026-07-29**, delta 4. Asserted over the same table as AC-5 so one test proves both, since both are emitted from the shared `ok()` helper |
| AC-18 | A legacy client POSTs `initialize` with no MCP headers and no `_meta` | HTTP 400 with `-32020`, whose message names the supported protocol version. **Added 2026-07-29**, delta 6: header validation runs before dispatch, so this request never reaches the unknown-method path the Wiring Test row describes. The versioning page makes naming the version a **SHOULD** — "legacy clients have no fall-forward mechanism, and this message may be the only diagnostic they can surface to users" |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Points a `2026-07-28` client at the daemon and lists tools | POST `tools/list` → headers → auth → registry descriptors | `test/plugin/mcp-tools-list-stateless.ci` |
| 2 | Runs a `ze` command through `tools/call` with bearer auth | POST → `Mcp-Name` validation → auth → dispatcher | `test/plugin/mcp-tools-call-stateless.ci` |
| 3 | Drives `ze-chaos` over MCP | Provider mode → `tools/call` → orchestrator | `test/chaos/chaos-mcp-tools-call.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestProtocolVersionIsCurrentRevision` | `internal/component/mcp/streamable_test.go` | Constant and supported set | |
| `TestHeaderMismatchRejected` | `internal/component/mcp/headers_test.go` | `-32020` per validation failure condition | |
| `TestMcpNameBase64Sentinel` | `internal/component/mcp/headers_test.go` | Decode before compare | |
| `TestUnsupportedVersionListsSupported` | `internal/component/mcp/streamable_test.go` | `-32022` payload shape | |
| `TestServerDiscoverShape` | `internal/component/mcp/discover_test.go` | `DiscoverResult` fields | |
| `TestEveryResultCarriesResultType` | `internal/component/mcp/streamable_test.go` | Table over all methods | |
| `TestAuthRunsOnEveryRequest` | `internal/component/mcp/streamable_test.go` | Second request with no credential still 401 | |
| `TestProviderModeAuthenticatesLikeEveryOtherPath` | `internal/component/mcp/streamable_test.go` | D-2: with `Provider` set and `AuthBearer` configured, a credential-less request is 401. Guards against the deleted bypass returning | |
| `TestBodyLimitBoundary` | `internal/component/mcp/streamable_test.go` | Boundary table row 1: 1048576 bytes served, 1048577 returns 413, empty body returns `-32700` | |
| `TestTaskConcurrencyCapIsPerPrincipal` | `internal/component/mcp/tasks_test.go` | Boundary table row 2 and AC-14: 8 per identity, 9th refused, a second identity still reaches 8 | |
| ~~`TestResourcesRejectWithoutCapability`~~ **`TestResourcesServedWithoutClientCapability` + `TestResourcesServedForEveryCapabilityShape`** | `internal/component/mcp/resources_test.go` | AC-13 as restated: resources are served to a `{}`-capability client, and list/read are byte-identical across `{}`, tasks-only, and a stray `{"resources":{}}` -- so any gate keyed on any declared value makes a row diverge. Replaces the `-32021` assertion, which locked in the functional break | |
| `TestMalformedMetaRejected` | `internal/component/mcp/meta_test.go` | AC-16: `_meta` missing `protocolVersion` → 400 + `-32602`; missing `clientCapabilities` → 400 + `-32602`; missing `clientInfo` → **accepted**, since it is only a SHOULD | |
| `TestEveryResultCarriesServerInfo` | `internal/component/mcp/streamable_test.go` | AC-17: same method table as `TestEveryResultCarriesResultType`, asserting `result._meta["io.modelcontextprotocol/serverInfo"]` | |
| `TestLegacyInitializeNamesSupportedVersion` | `internal/component/mcp/headers_test.go` | AC-18: a header-less POST whose body method is `initialize` returns 400 + `-32020` and the message contains `2026-07-28` | |
| `TestExecuteWithoutCommandFailsClosed` | `internal/component/mcp/tools_test.go` | AC-15: missing `command` returns the argument error, and no elicit frame is produced | |

### Boundary Tests (numeric inputs)

Three real numeric bounds survive the cutover, and one countable wire assertion
behaves like a bound. `MaxSessions`, `SessionTTL` and `MaxSessionLifetime` are
absent from this table on purpose: D-1 records that they are deleted, not
re-homed, so there is no boundary left to test.

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Request body size, bytes (`maxRequestBody`, `tools.go:673`; enforced `streamable.go:412`) | 1 .. 1048576 | 1048576 body bytes, request served normally | 0 bytes, which is a distinct failure: `json.Unmarshal` fails and the server returns JSON-RPC `-32700` (`streamable.go:420-426`), not a size rejection. Assert the code, not a 413 | 1048577 bytes, `http.MaxBytesReader` errors and the server returns HTTP 413 (`streamable.go:414-417`) |
| Concurrent tasks per principal (`defaultMaxConcurrentTasks`, `tasks.go:16`; enforced `tasks.go:104`) | 0 .. 8 | the 8th concurrent `tools/call` task for one identity is created | n/a, zero active tasks is the normal idle state | the 9th concurrent task for the same identity is refused with `errTaskConcurrencyCap` (`tasks.go:106`). The boundary is per identity, so a second principal must still reach 8 in the same test, which is what proves the cap is scoped and not global |
| Task TTL, seconds (`clampTTL`, `tasks.go:323-334`; bounds `tasks.go:19-20`) | 1 .. 3600 | a requested TTL of 3600 s is kept | ~~a requested TTL of 0 s clamps up to `minTaskTTL` = 1 s~~ **CORRECTED 2026-07-29:** `clampTTL` returns `r.ttl`, the registry-configured DEFAULT, for any `requested <= 0` (`tasks.go:324-326`); the `minTaskTTL` floor applies only to *positive* values below it (`:327-329`). `TestTaskRegistry_TTLExpiry` depends on that sentinel, so the code is right and this row was wrong. Assert: 0 and negative -> registry default; 1 ns and 999 ms -> `minTaskTTL` | a requested TTL of 3601 s clamps down to `maxTaskTTL` = 3600 s, it does not error. Both directions are silent clamps, so the assertion is on the clamped value |
| `data.supported` length in a `-32022` response | exactly 1 | the array is `["2026-07-28"]`, length 1 | length 0 would mean the server advertises no version it speaks, which fails closed into unreachability | length 2 or more would mean a dropped revision survived as an advertised version, contradicting `ai/rules/compatibility.md` and the umbrella cutover decision. Assert the exact array, not a `contains` |

Two candidate bounds were considered and rejected as not real. There is no
header-length bound to test: Ze reads MCP headers through `r.Header.Get` and
imposes no length of its own, so the only limit is Go's
`http.Server.MaxHeaderBytes` default, which is not Ze code and not this spec's
behavior. And the protocol version is a fixed-format date string checked by set
membership against `supportedProtocolVersions` (`streamable.go:49-53`), not a
range: its tests are equality and rejection cases (AC-2), not boundaries.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mcp-discover-daemon` | `test/plugin/*.ci` | Client discovers capabilities before any other call | |
| `mcp-tools-list-stateless` | `test/plugin/*.ci` | Tools listed with no handshake | |
| `mcp-tools-call-stateless` | `test/plugin/*.ci` | Command executed with per-request auth | |
| `mcp-legacy-initialize-rejected` | `test/plugin/*.ci` | Legacy client gets a diagnostic naming the supported version | |
| `mcp-get-method-not-allowed` | `test/plugin/*.ci` | GET stream attempt refused cleanly | |
| `mcp-header-mismatch-rejected` | `test/plugin/*.ci` | Forged `Mcp-Name` refused | |
| `mcp-execute-missing-command` | `test/plugin/*.ci` | AC-15: a tool call with no `command` gets a clear argument error instead of hanging on a prompt that can never arrive | |
| `mcp-resources-no-capability` | ~~`test/chaos/*.ci`~~ **`test/chaos-web/*.ci`** (corrected 2026-07-29 — that is where both existing MCP chaos tests live and the suite `ze-chaos-web-test` drives) | AC-13: a Provider-mode listener answers `resources/list` with `-32021` and `data.requiredCapabilities`, and the daemon stays up | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | No third-party MCP peer in-tree; `ze-test mcp` is rewritten from the specification text as the independent reading (umbrella Key Design Decisions) | |

## Files to Modify
- `internal/component/mcp/streamable.go` - transport rewrite
- `internal/component/mcp/streamable_tools.go` - dispatch
- `internal/component/mcp/streamable_auth.go` - per-request auth call site
- `internal/component/mcp/resources.go` - capability gate moves off the session onto the per-request capability value. ~~and the unguarded nil dereference at `:136-137` and `:143-144` goes with it~~ **Corrected 2026-07-29:** there is no unguarded dereference left to remove; `e53e2f24f` added the `sess == nil ||` guard at `:142` and `:151`. The gate still moves off the session, and the nil case disappears with the type rather than with a bug fix
- `internal/component/mcp/tasks.go` - session scoping removed: the `sessionID` field (`:36`, `:120`), `CancelAllForSession` (`:277`), `TaskElicit` (`:456-513`) and `buildTaskStatusNotification` (`:517`). `byIdentity` and every cap stay
- `internal/component/mcp/tools.go` - the elicit arm under `ze_execute` (`:740-766`) and the tool description advertising it (`:836`)
- `internal/component/mcp/yang/ze-mcp-conf.yang` - **no leaf is removed** (D-3). Reword the three `description` strings at lines 74, 79 and 90 that say scopes and principal names are attached to or carried on the session
- `cmd/ze/hub/service_mcp.go` - no config field it sets is removed (it sets none of the session fields); confirm it still compiles against the narrowed `StreamableConfig`
- `internal/chaos/orchestrator/run.go`, `cli.go` - Provider wiring
- `internal/test/cli/cmd_mcp.go` - client rewrite
- `cmd/ze/help_ai.go` - agent-facing wire shape
- `docs/architecture/mcp/overview.md`, `ai/digests/mcp.md`

## Files to Create
- `internal/component/mcp/meta.go` - `_meta` parsing
- `internal/component/mcp/headers.go` - standard header validation
- `internal/component/mcp/discover.go` - `server/discover`
- `test/plugin/*.ci` - the functional suite above

## Files to Delete
- `internal/component/mcp/session.go`, `session_test.go`
- `internal/component/mcp/reply_sink.go`, `reply_sink_test.go` (the POST-to-SSE upgrade has no successor)
- `internal/component/mcp/elicit.go`, `elicit_test.go` (A-4: `(*session).Elicit` at `:227` cannot compile once the type is gone. Phase 2's Required Reading must name this path so the flat-primitive schema validator at `:102-194` is recovered from commit A rather than reinvented, per R-4)
- `internal/component/mcp/task_elicit_test.go` (the only caller of `TaskElicit`, which is itself deleted from `tasks.go`)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-mcp-conf.yang`, but **description text only**: D-3 verified the schema declares no session, TTL, or lifetime leaf. The three `description` strings at lines 74, 79 and 90 say scopes and principal names are "attached to the session" / "carried on session" and become wrong under per-request identity |
| YANG validation constraints | N-A | No new leaves |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | Yes | `cmd_mcp.go` capability flags reshape to per-request |
| CLI grammar (keyword before value) | N-A | No operator-facing command surface |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | Yes | `test/plugin/*.ci` |
| Pipe completeness | N-A | JSON-RPC over HTTP, not a CLI display surface |
| Env var registration | No | D-3: the ten MCP entries at `internal/component/config/environment.go:63-72` are listen, enabled, token, bind-remote, auth-mode, the two oauth settings plus required-scopes, and the two TLS paths. None is a session TTL, cap, or lifetime knob, and no leaf is removed, so no `env.MustRegister` entry is removed |
| Doctor check for runtime dependencies | No | No new dependency |
| Prometheus counters/metrics | No | Verified 2026-07-28: `grep -rn "metrics\.\|prometheus" --include=*.go internal/component/mcp/` excluding tests returns nothing. The MCP component registers no counter or gauge at all, so there is no session gauge to remove and none is added here |
| BGP family surface | N-A | Not a BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | No | D-3: no leaf and no env var is added or removed, so the config *syntax* is unchanged. The reworded YANG `description` strings (lines 74, 79, 90) are schema help text, covered by row 6's guide pages, not a syntax change |
| 3 | CLI command added/changed? | Yes | `docs/functional-tests.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | MCP is a component |
| 6 | Has a user guide page? | Yes | All five pages under `docs/guide/mcp/`: `overview.md`, `remote-access.md`, `chaos.md`, `tasks.md`, and `elicitation.md`. `elicitation.md` documents a feature this phase deletes, so it is removed or reduced to a pointer at Phase 2 rather than left describing `elicitation/create`; `tasks.md` loses the session-scoping and status-notification text. Eight files across `docs/` mention elicitation and must be swept: the five listed in the umbrella plus `docs/guide/mcp/tasks.md` and `docs/guide/mcp/elicitation.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/mcp/overview.md` |
| 8 | Plugin SDK/protocol changed? | No | Untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/mcp-integration.md` names the revision |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/mcp/overview.md`, `ai/digests/mcp.md` |
| 13 | Route metadata keys added/changed? | N-A | Not routing |
| 14 | Prometheus counters added/changed? | No | Check performed 2026-07-28 and recorded in the Integration Checklist above: the MCP component has no metrics registration at all, so nothing is added, removed, or documented |
| 15 | Registered plugin/event/command/capability changed? | No | No registry entries |
| 16 | Changed source referenced by doc source anchors? | Yes | Every `internal/component/mcp/` file is anchored from the overview |
| 17 | Docs show config/CLI/API examples for this area? | Yes | `cmd/ze/help_ai.go:388-401`, `:551` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - add the `server/discover` route and a failing wiring test; add header-validation and `_meta` entry points as stubs that reject everything.
   - Tests: `mcp-discover-daemon`, `mcp-get-method-not-allowed`
   - Verify: entry points reachable, wiring tests fail because handlers are stubs
2. **Phase: Per-request metadata and headers** - `meta.go`, `headers.go`, `-32020`, `-32022`
3. **Phase: Delete the session layer** - remove `session.go`, `reply_sink.go`, `elicit.go`, GET/DELETE, initialize, `handleElicitResponse` (`streamable.go:562`), `TaskElicit` (`tasks.go:456-513`), the task `sessionID` field and `CancelAllForSession`, and `buildTaskStatusNotification` (`tasks.go:517`); move auth to per-request; move the tasks and resources capability gates from session bits to per-request `_meta.clientCapabilities`, passing identity and capabilities **by value** so no handler can run without them (R-3)
   - Nothing in this step is optional or splittable: A-4 records that `Elicit`, `TaskElicit` and `handleElicitResponse` all name the `session` type and cannot compile without it
   - Tests: `TestProviderModeAuthenticatesLikeEveryOtherPath`, `TestResourcesServedWithoutClientCapability` (renamed 2026-07-29 with AC-13's restatement), `TestExecuteWithoutCommandFailsClosed`
4. **Phase: Result envelope** - `resultType` on every method
5. **Phase: Consumers** - `ze-chaos`, `ze-test mcp` client (drop `-elicit`, `internal/test/cli/cmd_mcp.go:33`, and the `elicitation/create` handling at `:543`; drop the five `Mcp-Session-Id` sites at `:461`, `:476`, `:499`, `:579`, `:613`), `help_ai.go`, the three YANG `description` strings (D-3), `tools.go:836`, docs, digest

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Header validation runs before dispatch, and compares after Base64 decoding |
| Correctness | No path mints, requires or echoes `Mcp-Session-Id`; no path writes a JSON-RPC request frame to a client |
| Naming | MCP wire keys stay camelCase (documented exemption, `streamable.go:775-776`); `_meta` keys use `io.modelcontextprotocol/*` verbatim |
| Data flow | Authentication happens once, in one place, before dispatch; no handler re-authenticates and none skips it |
| Rule: `ai/rules/compatibility.md` | No shim for a dropped revision. A rejection path naming the supported version is not a shim |
| Rule: `ai/rules/no-parking.md` | Elicitation fails closed with a clear error (`tools.go:738`) and its machinery is deleted, not stubbed. No `*session`-typed signature survives anywhere, so nothing is half-wired for Phase 2 |
| Rule: `ai/rules/fail-closed-guards.md` (R-3) | Per-request identity and capabilities are values, not pointers, and no handler can run without them. Specifically: nothing reproduces `resources.go:136-137`, where a guard dereferences a possibly-absent value and a zero state reads as an answer |
| Rule: `ai/rules/wiring-completeness.md` | Every new exported symbol in `meta.go`, `headers.go` and `discover.go` has a non-test caller. Note `TaskElicit` (`tasks.go:464`) is a pre-existing violation this phase clears by deleting it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Version constant advanced | `grep -n 'ProtocolVersion = ' internal/component/mcp/streamable.go` |
| Session layer gone | `ls internal/component/mcp/session.go` returns no such file |
| Elicit machinery gone | `ls internal/component/mcp/elicit.go` returns no such file, and `grep -rn "TaskElicit\|handleElicitResponse\|elicitation/create" --include=*.go internal/ cmd/` is empty |
| No `*session` type survives | `grep -rn "\*session\b" --include=*.go internal/component/mcp/` is empty, including tests |
| No config leaf or env var was removed (D-3) | `git diff internal/component/mcp/yang/ze-mcp-conf.yang` touches only `description` text; `git diff internal/component/config/environment.go` is empty |
| No session symbols in production | `grep -rn "sessionRegistry\|Mcp-Session-Id" --include=*.go internal/ cmd/ \| grep -v _test.go` is empty |
| Chaos suites green | `make ze-chaos-test` |
| Full gate | `make ze-verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Per-request auth is not weakened | Confirm the former Provider-mode bypass (`streamable.go:463-475`) does not become an unauthenticated path. D-2 answers *why* it is safe (`ze-chaos` is `AuthNone` by configuration, `streamable.go:167-176` + `bearer.go:41-43`); the review must still confirm the branch is gone rather than moved, and that no path reaches `runMethod` without passing `authenticate` |
| Header/body confusion | Validation before dispatch, comparison after decoding |
| Resource exhaustion | Answered by D-1: session caps bounded an object that no longer exists, and the surviving bounds are the 1 MB body cap (`tools.go:673`, enforced `streamable.go:412`) and the per-principal task caps (`tasks.go:16-17`, enforced `tasks.go:104`). The residual review obligation is the inverse of the original question: confirm the rewrite introduces **no new** per-client structure that outlives a request. Any new map, cache, or registry keyed by client, connection, or identity is a finding |
| Identity-keyed map growth | `byIdentity` (`tasks.go:52`) must stay keyed by the authenticator's `Identity().Name` and never by a client-supplied field from `_meta.clientInfo`. Confirm the per-request identity is the only thing that reaches the registry |
| No optional-ness reintroduced (R-3) | ~~The pre-cutover code has a live fail-open instance: `resourcesList`/`resourcesRead` (`resources.go:136-137`, `:143-144`) dereference a possibly-nil `*session` ... so a Provider-mode `resources/list` panics rather than degrading.~~ **Corrected 2026-07-29:** fixed by `e53e2f24f`; the guard is present at `:142`/`:151`. The review obligation is unchanged and is the important half: confirm the replacement passes per-request identity and capabilities **by value**, so the compiler forces every handler to have them and no handler can be reached with a "maybe absent" context. The lesson the deleted example taught still stands — optional-ness the compiler does not force you to handle is what produced that bug — it is now a historical example rather than a live one |
| Error leakage | `-32020` messages must not echo unvalidated header bytes |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Wrong AC → DESIGN, correct AC → IMPLEMENT |
| A spec MUST cannot be met as designed | STOP. Escalate to Thomas per `ai/rules/rfc-compliance.md` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

What the code does not say, found while designing this phase.

- **The cutover is smaller than the file count suggests, because the hard parts
  are already per-request.** Three things that look like they need designing turn
  out to be already done: the authenticator is request-shaped
  (`auth.go:108-112`), the task registry is principal-indexed (`tasks.go:52`,
  `:103`), and the elicit path already fails closed on a nil session
  (`tools.go:737-738`). What remains is genuinely deletion plus new wire surface,
  and none of the new surface (`-32020`, `-32022`, `resultType`,
  `server/discover`) exists anywhere in the component today, so it is all new
  code rather than edits to code with existing callers and tests.
- **"Session-less" is currently modelled as a nil pointer, and that shape is what
  R-3 exists to stop the rewrite from recreating.** Provider mode passes a nil
  `*session` into `runMethod` (`streamable.go:468`), and the comment above it
  (`streamable.go:459-462`) claims "runMethod handlers are nil-session aware
  (tasks/resources degrade to method-not-found)". ~~That claim is true for the four
  task handlers ... and **false for resources**: `resourcesList`
  (`resources.go:136-137`) and `resourcesRead` (`:143-144`) call
  `sess.ClientSupportsResources()` with no nil check ... A `resources/list` sent to
  a Provider-mode listener therefore panics rather than degrading.~~
  **Superseded 2026-07-29.** That was true when this spec was written and is not
  true now: commit `e53e2f24f` ("fix(mcp): resources handlers deny a nil session,
  not panic") landed on 2026-07-29 and added the guard, so `resources.go:142` and
  `:151` now read `if sess == nil || !sess.ClientSupportsResources()` and the
  short-circuit prevents the dereference. Every handler reached with a nil session
  now degrades to method-not-found, and the comment at `streamable.go:459-462` is
  accurate as written.
  The insight the example was carrying survives its own repair, and is the reason
  to keep the paragraph rather than delete it: optional-ness that the compiler does
  not force you to handle is what produced the bug in the first place, and the fix
  was a hand-written guard at each of two call sites rather than a shape that makes
  the mistake impossible. A nil-able per-request context, or a capability struct
  whose zero value reads as "supported", would reproduce exactly this under a new
  name and would again be caught only by whoever remembers to write the guard.
  Phase 1 removes the class rather than the instance: per-request identity and
  capabilities are **values**, so there is no nil case for a handler to forget.
- **`MaxSessions` was never reachable from config, so removing it changes no
  deployed behavior.** `SessionTTL`, `MaxSessions` and `MaxSessionLifetime` are
  `StreamableConfig` fields (`streamable.go:72`, `:76`, `:81`) that no caller ever
  sets, so every deployment has always run on the built-in defaults. The same is
  true of `MaxBodyBytes` (`:73`), which is why the effective body cap is always
  exactly 1 MB. The distinction that matters at implementation time: the three
  session fields are deleted, `MaxBodyBytes` is kept, and the difference is not
  visible from usage (both are unused) but from whether the thing they bound
  still exists.
- **`TaskElicit` is exported, unwired, and has only test callers.** `tasks.go:464`
  declares it; the only references are three calls in `task_elicit_test.go`
  (`:41`, `:101`, `:158`). It is a pre-existing `ai/rules/wiring-completeness.md`
  violation that this phase happens to clear. Worth knowing so its deletion is not
  mistaken for a behavior removal that needs a replacement.
- **The elicitation deletion is forced by the type system, not chosen by policy.**
  It would have been reasonable to leave `elicit.go` in place until Phase 2 and
  simply stop calling it. That option does not exist: `Elicit` is a method on
  `*session` (`elicit.go:227`), and `TaskElicit` (`tasks.go:464`) and
  `handleElicitResponse` (`streamable.go:562`) both take `*session` in their
  signatures. Deleting `session.go` breaks the build unless all three go in the
  same change. This is the strongest argument for the umbrella's "Phase 1 is
  atomic and not splittable" decision, and it is stronger than the argument the
  umbrella actually gives (which is about learning the client's version).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| **Per-request authentication replaces session-bound identity, as a call-site move with no interface change.** `s.authenticate(r)` (`streamable.go:396`) moves from the `initialize` branch only (`streamable.go:428`) to every POST, before dispatch | (a) Keep identity bound once and carry it in a server-side map keyed by something other than a session id, which is a session under another name; (b) change `authenticator` to take a pre-extracted credential struct so the transport does the parsing | Rejected (a) because it recreates the exact state the revision removes, and would need its own TTL, cap and eviction, reintroducing everything D-1 deletes. Rejected (b) because it is unnecessary: A-1 confirmed all four implementations already take `*http.Request` (`bearer.go:41`, `:57`, `:97`; `oauth.go:42`). The chosen form is also **strictly stronger security**, not merely equivalent: today a stolen or guessed `Mcp-Session-Id` is a bearer credential in its own right, since `streamable.go:479-483` looks the session up and every later request is trusted by session-id validity alone. After the cutover there is no credential-equivalent identifier to steal, and revoking a token takes effect on the next request rather than at session expiry |
| **Nothing replaces `MaxSessions` or `MaxSessionLifetime`. They are deleted, and no new cap is invented** (D-1) | (a) A per-remote-address in-flight request cap; (b) a global concurrent-request semaphore; (c) keep a lightweight "client record" purely for accounting | All three were rejected for the same reason: they would be new state added to a change whose entire point is removing state, and none of them bounds anything that is currently unbounded. `defaultMaxSessions` (`session.go:118`) bounded sessions, an object that ceases to exist. The bounds that matter after the cutover already exist and are per-request or per-principal, not per-client: 1 MB body (`tools.go:673`, enforced `streamable.go:412`), 8 concurrent tasks per identity (`tasks.go:16`, enforced `tasks.go:104`), 128 retained terminal tasks (`tasks.go:17`), and a 1 s .. 1 h task TTL clamp (`tasks.go:19-20`). Adding (a) or (b) would also be the first Ze-level cap on plain HTTP concurrency in a component that has never had one, which is a policy change smuggled into a conformance change |
| **The Provider-mode branch disappears rather than being special-cased or explicitly exempted** (D-2). Every request, `ze-chaos` included, passes through the same per-request `authenticate` | (a) Keep a `Provider != nil` short-circuit that skips auth, preserving today's shape; (b) keep the branch but add an explicit `AuthNone`-only assertion so the exemption is at least checked | Rejected (a) because it is the only thing that could make the cutover *weaken* auth, and the Security Review row exists precisely to catch it. Rejected (b) because the exemption is not needed at all: `ze-chaos` is unauthenticated by configuration, not by the branch. It sets `Provider` with no `Token` and no `AuthMode` (`run.go:535`, `cli.go:635`), `NewStreamable` infers `AuthNone` (`streamable.go:167-176`), and `noneAuthenticator` accepts every request with a zero `Identity` (`bearer.go:41-43`). So running `ze-chaos` through the uniform path is observably identical to today, while removing the only code shape from which an unauthenticated path could later be reached by accident. A carve-out that is not needed is a carve-out that will outlive its reason |
| **Elicitation fails closed on the branch that already exists, and its machinery is deleted in this phase rather than Phase 2** | (a) Add a new explicit guard returning a "not supported until MRTR" error; (b) leave `elicit.go` compiled but unreachable until Phase 2; (c) stub `Elicit` to return an error and keep the file | Rejected (a) because `tools.go:737-738` already returns `ErrResult("missing required argument: command")` on `s.session == nil`, so the fail-closed behavior is the existing default and a second guard would be dead code. Rejected (b) and (c) because they are not available: A-4 confirmed `(*session).Elicit` (`elicit.go:227`), `TaskElicit` (`tasks.go:464`) and `handleElicitResponse` (`streamable.go:562`) all name the `session` type in their signatures and cannot compile once it is gone. (c) additionally leaves precisely the half-wired path `ai/rules/no-parking.md` forbids. The tool description at `tools.go:836` is reworded in the same change so no client is told a deleted capability exists |
| **`tasks/list`, `tasks/get`, `tasks/result` and `tasks/cancel` all survive Phase 1, re-scoped to principal** | (a) Delete all four in Phase 1 and have Phase 3 reintroduce them in extension shape; (b) keep only `tasks/get` and drop the other three early, since the revision removes `tasks/list` and `tasks/result` | Rejected (a) because A-3 confirmed the re-scope is nearly free: the registry is already keyed `byIdentity` (`tasks.go:52`) with the concurrency cap counted per identity (`tasks.go:103`), and the handlers already read `sess.Identity().Name` (`streamable_tools.go:179`, `:198`, `:211`). Deleting working, principal-scoped code to rebuild it two phases later is churn, and it would leave Phase 1 with a method table that matches neither revision. Rejected (b) because partial removal is the R-4 failure mode from the umbrella: a server that is neither revision. Phase 1 keeps the method set it has and Phase 3 makes the removals as one deliberate change, with the session coupling (`sessionID` at `tasks.go:36`, `CancelAllForSession` at `:275`, the status push at `:516`) removed here because it cannot compile either |
| **Absence and mismatch of a required header are the same verdict, and no header has a default** | Treat a missing `MCP-Protocol-Version` as the current revision, which is what the pre-cutover code does for the legacy header via `LegacyProtocolVersion` (`streamable.go:39`) | The existing default-on-absence behavior is exactly the fail-open shape `ai/rules/fail-closed-guards.md` bans, and it is only tolerable today because the version was pinned at initialize. With no handshake there is nothing to fall back to, and a server that guesses the version defeats the header/body confusion defense that `-32020` exists to provide. `LegacyProtocolVersion` is deleted rather than repurposed as a rejection label, so no code path can read a version Ze does not speak as a valid value |

## Known Limitations

Scope boundaries this phase accepts. Each is either owned by a later phase, or
owned by the umbrella and recorded there.

- **The server is incomplete between Phase 1 and Phase 4, and for one item it is
  NOT conformant.** ~~That is a conformant subset, not a violation.~~
  **Corrected 2026-07-29 after independent review.** Two of the three gaps really
  are additive and a server that does not advertise them owes nothing: MRTR
  (`resultType: "input_required"` is never produced, Phase 2) and the Tasks
  extension shape (Phase 3). The third is not: `ttlMs` and `cacheScope` are
  **non-optional** fields on `CacheableResult`, which `DiscoverResult` extends,
  and changelog minor #5 requires them on `tools/list`, `resources/list`,
  `resources/read` and `server/discover`. Ze emits neither, and
  `discover_test.go` currently asserts their absence. So the intermediate state
  omits fields the schema makes mandatory, which is a conformance gap and is
  recorded as such in `plan/deferrals/mcp2026-1-stateless-core.md` with
  `plan/spec-mcp2026-4-caching-apps.md` as its destination -- a spec that has
  already done the design. The intermediate state never ships: phases 1-4 land as
  one series. Calling it "a conformant subset, not a violation" was wrong, and the
  distinction matters because `ai/rules/rfc-compliance.md` treats phasing a MUST
  as an escalation trigger rather than a scheduling decision.
- **Elicitation is gone with no replacement until Phase 2.** `ze_execute` invoked
  without a `command` argument returns "missing required argument: command"
  (`tools.go:738`) instead of prompting. That is the pre-existing behavior for
  every client that did not declare the capability, so it is a narrowing of an
  optional path rather than a regression of the default one, but any client that
  did rely on the prompt loses it for the duration of Phase 2.
- **No task status is pushed. Polling is the only way to observe a task.**
  `buildTaskStatusNotification` (`tasks.go:517`) rode the GET stream this phase
  deletes, and its successor is Phase 3's polling model. Between the two phases
  a client must call `tasks/get`. The spec's stated default for tasks is polling,
  so this is the target behavior arriving early rather than a gap.
- **No shipping MCP client is known to speak `2026-07-28` when this lands.**
  Umbrella R-1, accepted by owner decision. `ze-test mcp` is the only client that
  will drive the new server, and by the umbrella's Key Design Decision it is
  rewritten from the specification text rather than from Ze's implementation so
  it is an independent reading. Client availability is a release-readiness
  question, not a design one.
- **`subscriptions/listen` is not implemented**, and this phase does not change
  that. Umbrella A-4 is confirmed: the method carries a closed four-field opt-in
  filter and no server-side MUST obliges Ze to offer it. Ze has nothing to
  advertise on a stream once the GET endpoint is gone.
- **Prompts, Roots, Sampling and Logging remain unimplemented**, unchanged by
  this phase. `runMethod` has never handled `prompts/*` (`streamable_tools.go:22-45`),
  and the other three are Deprecated in this revision. Recorded in the umbrella;
  no deferral row, because none is a regression introduced here.
- **This phase does not add metrics.** The MCP component registers no Prometheus
  counter or gauge today and gains none here, so there is no observability of
  per-request auth failures, header rejections, or version rejections beyond the
  existing audit-recorder call on auth failure (`recordMCPAuthFailure`, `streamable.go:431`). Adding MCP
  metrics is a separate, genuinely new feature and is deliberately outside a
  conformance cutover.

## RFC Documentation (Scope: protocol)

Add `// MCP 2026-07-28 <page> Section X: "<quoted requirement>"` above each
enforcing path: header validation and `-32020`, version rejection and `-32022`,
the 405 on GET/DELETE, the 404 on unknown method, and the `server/discover` MUST.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved

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
- [ ] **Commit B:** `git rm plan/<spec>` only
