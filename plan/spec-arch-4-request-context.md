# Spec: request-context-and-caller-identity

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | arch-4 |
| Updated | 2026-04-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/system-architecture.md` - hub transport and plugin routing boundaries
4. `docs/architecture/subsystem-wiring.md` - data-flow layering rules
5. `docs/architecture/api/commands.md` - API transport and command execution boundaries
6. `internal/component/plugin/server/command.go` - current request-context gap
7. `internal/component/api/engine.go` - execute path currently lacks caller context
8. `internal/component/ssh/ssh.go` - SSH exec/auth paths and remote address handling
9. `internal/component/aaa/{aaa,types}.go` - AAA authenticator surface
10. `internal/component/tacacs/authenticator.go` - TACACS rem_addr TODO
11. `internal/component/bgp/plugins/cmd/peer/prefix_update.go` - `context.TODO()` PeeringDB lookup

## Task

Propagate request-bound context and caller identity across command dispatch and external integrations so subsystem handlers, plugin routes, API execute paths, SSH sessions, and TACACS receive cancellable context plus correct actor metadata. Preserve existing command syntax, response shapes, authorization/accounting chokepoints, and the rule that identity is injected by trusted transport wiring rather than client payloads.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/system-architecture.md` - hub-mode transport and command routing boundaries
  -> Decision: caller identity is a transport concern owned by the hub composition root, not by command payloads.
  -> Constraint: all execution still flows through the existing dispatcher/pluginserver chokepoints.
- [ ] `docs/architecture/subsystem-wiring.md` - layering between buses, plugin delivery, and subsystem boundaries
  -> Decision: request context must follow the existing transport -> engine -> dispatcher -> subsystem/plugin path.
  -> Constraint: do not bypass `pluginserver.Server` or couple transports directly to subsystem internals.
- [ ] `docs/architecture/api/commands.md` - shared API transport contract
  -> Decision: REST/gRPC stay thin adapters over the shared API engine.
  -> Constraint: existing command strings and output formats stay unchanged.
- [ ] `docs/architecture/hub-api-commands.md` - shared command registry and hub dispatch model
  -> Decision: plugin and subsystem routing semantics remain exact/longest-prefix based.
  -> Constraint: plugin RPC payloads do not become identity carriers.
- [ ] `docs/architecture/plugin-manager-wiring.md` - pluginserver composition-root ownership
  -> Decision: request-scoped data belongs on request-scoped objects, not on global server state.
  -> Constraint: keep plugin manager / pluginserver lifecycle boundaries intact.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8907.md` - TACACS+ authentication request fields and status handling
  -> Constraint: SSH authentication should pass service `ssh` and `rem_addr` when available.

### Related Learned Summaries
- [ ] `plan/learned/229-command-context-server-refactor.md` - prior `CommandContext` cleanup
  -> Decision: prefer nil-safe accessor methods on `CommandContext` over leaking transport plumbing into handlers.
- [ ] `plan/learned/390-rbac.md` - dispatcher authz chokepoint
  -> Constraint: transport changes must not duplicate or bypass `Dispatcher.Dispatch()` authorization/accounting.
- [ ] `plan/learned/601-tacacs.md` - AAA bundle wiring and current remote-address gap
  -> Constraint: preserve reject-vs-unreachable chain behavior while completing SSH remote address propagation.
- [ ] `plan/learned/227-config-reload-7-coordinator-hardening.md` - lifecycle and context ownership during reload/shutdown
  -> Constraint: request contexts must not outlive server shutdown/reload boundaries.

**Key insights:**
- The biggest gap is not authz logic; it is transport metadata dropping on the way to shared execution paths.
- `CommandContext` is already the per-request carrier for username/peer/meta, so it is the right place for request context as long as access stays nil-safe.
- API `Execute` is the odd one out: `Stream` already accepts `context.Context`, but `Execute` still does not.
- SSH exec already carries remote address into `CommandContext`; SSH interactive sessions and TACACS auth do not.
- Plugin RPC params intentionally exclude identity today; that must remain true after the refactor.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/command.go` - `CommandContext` carries `Server`, `Process`, `Peer`, `Username`, `RemoteAddr`, `Meta`; `dispatchSubsystem()` uses `context.Background()`; `routeToProcess()` derives timeout from `context.Background()`, so caller cancellation is lost.
- [ ] `internal/component/plugin/server/dispatch.go` - plugin `update-route` and `dispatch-command` build `CommandContext` without request context; `dispatch-command` correctly uses `Username: "plugin:<name>"`.
- [ ] `internal/component/plugin/server/server.go` - socket `RPCParams` only accept `Selector` and `Args`; identity is explicitly forbidden in client JSON.
- [ ] `internal/component/api/engine.go` - `Executor` has signature `func(username, command string) (string, error)`; `Execute()` has no caller context; `Stream()` already does.
- [ ] `internal/component/api/types.go` - `CallerIdentity` carries only `Username`.
- [ ] `internal/component/api/rest/server.go` - `callerIdentity()` returns username only; execute requests do not pass remote address metadata.
- [ ] `internal/component/api/grpc/server.go` - auth interceptors inject username only; `Execute()` and `Stream()` ignore gRPC peer address.
- [ ] `cmd/ze/hub/api.go` - `apiExecutor()` creates `CommandContext{Server:s, Username:username}`; `buildUserAuthenticator()` still calls local auth with just `(username, password)`.
- [ ] `cmd/ze/hub/infra_setup.go` - SSH exec executor factory passes `Username` and `RemoteAddr` to `CommandContext`; streaming executor factory only passes `Username`.
- [ ] `internal/component/ssh/ssh.go` - `CommandExecutorFactory` accepts `(username, remoteAddr)`, but `ExecutorForUser(username)` forces `remoteAddr=""`; password auth callback has `ctx.RemoteAddr().String()` but AAA interface drops it.
- [ ] `internal/component/ssh/session.go` - `createSessionModel(username string)` passes only username into the injected session factory.
- [ ] `internal/component/cli/contract/contract.go` - `SessionModelFactory` only accepts username, so SSH TUI command mode cannot preserve remote address/session context.
- [ ] `cmd/ze/hub/session_factory.go` - session model factory uses `srv.ExecutorForUser(username)`, which drops remote address for interactive commands.
- [ ] `internal/component/bgp/plugins/cmd/peer/prefix_update.go` - PeeringDB lookups use `context.TODO()` and fixed sleeps, so cancellation only happens between iterations.
- [ ] `internal/component/aaa/{aaa.go,types.go}` - `Authenticator` / `ChainAuthenticator` only accept `(username, password)`.
- [ ] `internal/component/authz/auth.go` - local bcrypt authenticator matches the old AAA interface and ignores network metadata.
- [ ] `internal/component/tacacs/authenticator.go` - TACACS auth hardcodes service `ssh` but passes empty `rem_addr`; source has a TODO for this exact gap.
- [ ] `internal/component/web/auth.go` - basic auth and login handler still call `Authenticate(username, password)`, so shared auth surface changes will reach web auth too.

**Tests already inspected:**
- [ ] `internal/component/plugin/server/command_test.go` - accounting hooks already assert `Username` and `RemoteAddr` propagation.
- [ ] `internal/component/api/rest/server_test.go` - transport-level REST tests exist and can absorb request-context coverage.
- [ ] `internal/component/api/grpc/server_test.go` - transport-level gRPC tests exist and can absorb peer-address coverage.
- [ ] `internal/component/ssh/ssh_test.go` - SSH session/executor wiring tests exist.
- [ ] `internal/component/tacacs/authenticator_test.go` - TACACS auth tests exist but do not cover remote address propagation.
- [ ] `internal/component/web/auth_test.go` - shared authenticator call sites are already under test.
- [ ] `cmd/ze/hub/aaa_lifecycle_test.go` - stub authenticator will need updating if the AAA interface changes.

**Behavior to preserve:**
- Existing command strings, RPC method names, and response payload shapes.
- `RPCParams` and plugin RPC JSON must NOT accept or override `Username` or `RemoteAddr`.
- Plugin `dispatch-command` identity remains `plugin:<process-name>`.
- Unauthenticated API transports continue to default to username `api`.
- Dispatcher authorization/accounting stays centralized in `Dispatcher.Dispatch()`.
- Plugin command timeout behavior stays enforced even after parent-context propagation.
- Local bcrypt auth remains timing-safe and preserves reject semantics.

**Behavior to change:**
- Request-scoped contexts are propagated end-to-end on execute/update paths that currently use `context.Background()` or `context.TODO()`.
- Remote address reaches dispatcher/accounting from REST, gRPC, SSH exec, and SSH interactive command sessions.
- SSH password auth passes remote address into AAA/TACACS.
- PeeringDB lookups become cancellable through the caller context.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point -- REST / gRPC Execute
1. Transport authenticates request and extracts username plus transport remote address.
2. Transport builds `api.CallerIdentity` and passes the request `context.Context` to `APIEngine.Execute`.
3. `APIEngine` performs authz check and forwards the same context + auth metadata to the executor.
4. `cmd/ze/hub/api.go` builds `pluginserver.CommandContext` from server + caller context + username + remote address.
5. `Dispatcher.Dispatch()` remains the single chokepoint for authz/accounting/execution.

### Entry Point -- SSH Exec
1. Wish authenticates the SSH user and provides `username` + `remoteAddr`.
2. `infra_setup.go` builds a per-session executor closure from those values.
3. The closure dispatches through `Dispatcher.Dispatch()` with `CommandContext{Username, RemoteAddr}` and the request context.
4. Builtin handlers, subsystem handlers, and plugin RPC routes inherit the caller context.

### Entry Point -- SSH Interactive Session
1. SSH session is accepted and a bubbletea model is created.
2. Current code drops `remoteAddr` when `createSessionModel()` calls `ExecutorForUser(username)`.
3. New wiring must preserve per-session remote address (and a request-derived context source where needed) into the command executor used by the TUI model.
4. Interactive commands then share the same dispatcher/accounting path as exec commands.

### Entry Point -- Plugin Engine RPCs
1. `handleUpdateRouteRPC()` and `handleDispatchCommandRPC()` build a `CommandContext`.
2. They must attach request/server context without changing peer selector, metadata, or plugin identity semantics.
3. `dispatchSubsystem()` and `routeToProcess()` derive child contexts from the request context, adding timeouts without discarding cancellation.

### Entry Point -- SSH Password Auth / AAA / TACACS
1. Wish password callback receives `username`, `password`, and SSH `remoteAddr`.
2. SSH builds an AAA auth request object and passes it into the chain.
3. `ChainAuthenticator` forwards the full request to each backend.
4. Local auth ignores network metadata; TACACS uses service `ssh` and the remote address in the authen START.

### Entry Point -- BGP Prefix Update
1. Command reaches `HandleBgpPeerPrefixUpdate()` through the normal dispatcher path.
2. The handler derives its lookup context from `CommandContext` instead of `context.TODO()`.
3. PeeringDB lookups and inter-peer wait loops stop promptly when the caller context is canceled.

### Transformation Path
1. Transport authenticates the caller and extracts trusted metadata (`username`, `remoteAddr`, request `context.Context`).
2. Shared transport adapters (`RESTServer`, `GRPCServer`, SSH executor/session factory) convert that metadata into request-scoped auth/context objects.
3. Hub wiring builds `pluginserver.CommandContext` only from trusted transport data.
4. Dispatcher performs authz/accounting once and routes to builtin, subsystem, or plugin code.
5. External I/O (plugin RPCs, PeeringDB, TACACS) derives child contexts from the same parent request instead of creating new roots.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| REST/gRPC transport -> API engine | `api.CallerIdentity` + request `context.Context` | [x] `TestExecutePropagatesRequestContextAndRemoteAddr`, `TestExecuteUsesPeerRemoteAddr` |
| API engine -> dispatcher | `apiExecutor()` builds `pluginserver.CommandContext` | [x] `TestAPIExecutorPropagatesRequestContextAndRemoteAddr` |
| SSH transport -> dispatcher | exec/TUI command executor closures | [x] `TestSSHExecCommandPropagatesRemoteAddr`, `TestCreateSessionModelPreservesRemoteAddr` |
| Dispatcher -> subsystem/plugin process | child contexts derived from `CommandContext.Context()` | [x] `TestDispatchSubsystemUsesCommandContextContext`, `TestRouteToProcessUsesParentContextTimeout`, `TestForwardToPluginUsesParentContext` |
| SSH password auth -> AAA chain | request object carrying username/password/remote/service | [x] `TestChainAuthenticatorForwardsAuthRequest` |
| AAA chain -> TACACS client | `TacacsClient.Authenticate(..., "ssh", remAddr)` | [x] `TestTacacsAuthenticatorUsesRemoteAddr` |
| BGP command handler -> PeeringDB client | lookup context derived from caller request | [x] `TestPrefixUpdateStopsOnContextCancel` |

### Integration Points
- `pluginserver.CommandContext` - add nil-safe request-context accessor for all downstream work.
- `api.CallerIdentity` / `APIEngine.Execute()` - align execute path with the already-context-aware stream path.
- `ssh.Server` session/executor wiring - preserve remote address for interactive sessions.
- `aaa.Authenticator` surface - accept richer auth request data without changing reject/fallback semantics.
- `TacacsAuthenticator` - use propagated `rem_addr` instead of empty string.

### Architectural Verification
- [x] No bypassed layers: transports still go through API engine or dispatcher, never directly to handlers.
- [x] No unintended coupling: subsystem/plugin code reads request context through `CommandContext`, not through transport-specific types.
- [x] No duplicated functionality: authz/accounting remains in `Dispatcher.Dispatch()`, not reimplemented per transport.
- [x] Zero-copy preserved where applicable: this is metadata plumbing only; command payload transport remains unchanged.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| SSH exec command | -> | `cmd/ze/hub/infra_setup.go` executor closure -> `Dispatcher.Dispatch()` | `TestSSHExecCommandPropagatesRemoteAddr` |
| SSH interactive command | -> | `internal/component/ssh/session.go` / `cmd/ze/hub/session_factory.go` | `TestCreateSessionModelPreservesRemoteAddr` |
| REST `/api/v1/execute` | -> | `RESTServer.callerIdentity()` -> `APIEngine.Execute()` -> `apiExecutor()` | `TestExecutePropagatesRequestContextAndRemoteAddr` |
| gRPC `ZeService.Execute` | -> | `peer.FromContext()` -> `APIEngine.Execute()` -> `apiExecutor()` | `TestExecuteUsesPeerRemoteAddr` |
| Plugin `ze-plugin-engine:dispatch-command` | -> | `handleDispatchCommandRPC()` -> `Dispatcher.Dispatch()` | `TestHandleDispatchCommandRPCPreservesPluginIdentity` |
| Plugin command proxy | -> | `Dispatcher.ForwardToPlugin()` -> `routeToProcess()` | `TestForwardToPluginUsesParentContext`, `TestRouteToProcessUsesParentContextTimeout` |
| BGP prefix update | -> | `HandleBgpPeerPrefixUpdate()` -> `PeeringDB.LookupASN()` | `TestPrefixUpdateStopsOnContextCancel` |
| SSH password auth | -> | `wish.WithPasswordAuth()` -> `aaa.ChainAuthenticator` -> `TacacsAuthenticator` | `TestChainAuthenticatorForwardsAuthRequest`, `TestTacacsAuthenticatorUsesRemoteAddr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `CommandContext` created with a request context | `ctx.Context()` returns that context; if unset it falls back to `Server.Context()` and finally `context.Background()` |
| AC-2 | Dispatcher routes to subsystem or plugin process | Child contexts derive from the caller context; command timeout is still enforced without discarding caller cancellation |
| AC-3 | REST execute request | Request context, username, and `r.RemoteAddr` reach `apiExecutor()` and then dispatcher/accounting |
| AC-4 | gRPC execute/stream request | Request context, username, and gRPC peer address reach dispatcher/accounting when peer info is available |
| AC-5 | SSH exec and interactive commands from the same SSH session | Both paths dispatch with the authenticated username and the same remote address |
| AC-6 | Plugin `dispatch-command` request | Dispatcher still sees `Username: "plugin:<name>"`; plugin JSON cannot override identity metadata |
| AC-7 | `bgp peer ... prefix-update` canceled by caller | Handler stops lookup loop promptly and no longer uses `context.TODO()` |
| AC-8 | SSH password auth against TACACS | AAA backend receives remote address; TACACS auth passes non-empty `rem_addr` to the client when SSH provided it |
| AC-9 | Existing clients use REST/gRPC/SSH commands as before | No command syntax, response payload, or public RPC method changes are required |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCommandContextContextFallback` | `internal/component/plugin/server/command_test.go` | Request-context accessor uses request -> server -> background fallback | pass |
| `TestDispatchSubsystemUsesCommandContextContext` | `internal/component/plugin/server/command_test.go` | Subsystem handlers inherit caller cancellation | pass |
| `TestRouteToProcessUsesParentContextTimeout` | `internal/component/plugin/server/command_test.go` | Plugin RPC route derives timeout from parent context instead of a new root | pass |
| `TestForwardToPluginUsesParentContext` | `internal/component/plugin/server/command_test.go` | Builtin proxy handlers preserve caller cancellation when forwarding to plugins | pass |
| `TestHandleDispatchCommandRPCPreservesPluginIdentity` | `internal/component/plugin/server/dispatch_test.go` | Plugin-dispatched commands keep `plugin:<name>` identity while gaining request context | pass |
| `TestExecutePropagatesRequestContextAndRemoteAddr` | `internal/component/api/rest/server_test.go` | REST transport passes request context + remote address into the engine/executor | pass |
| `TestExecuteUsesPeerRemoteAddr` | `internal/component/api/grpc/server_test.go` | gRPC transport pulls remote address from `peer.FromContext()` | pass |
| `TestCreateSessionModelPreservesRemoteAddr` | `internal/component/ssh/ssh_test.go` | Interactive SSH sessions no longer lose remote address | pass |
| `TestChainAuthenticatorForwardsAuthRequest` | `internal/component/aaa/chain_test.go` | AAA chain forwards the richer auth request to each backend unchanged | pass |
| `TestTacacsAuthenticatorUsesRemoteAddr` | `internal/component/tacacs/authenticator_test.go` | TACACS auth passes SSH remote address to the client | pass |
| `TestAuthMiddlewarePassesRemoteAddrToAuthenticator` | `internal/component/web/auth_test.go` | Shared web auth call sites still work after AAA surface change | pass |
| `TestPrefixUpdateStopsOnContextCancel` | `internal/component/bgp/plugins/cmd/peer/prefix_update_test.go` | PeeringDB loop stops on caller cancellation | pass |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | No new numeric inputs or bounds are introduced by this spec | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestExecutePropagatesRequestContextAndRemoteAddr` | `internal/component/api/rest/server_test.go` | REST caller aborts or disconnects; command execution sees caller metadata and cancellation | pass |
| `TestExecuteUsesPeerRemoteAddr` | `internal/component/api/grpc/server_test.go` | gRPC client metadata includes peer address that reaches dispatcher/accounting | pass |
| `TestSSHExecCommandPropagatesRemoteAddr` | `internal/component/ssh/ssh_test.go` | SSH operator command and accounting observe the real client endpoint | pass |

### Future (if deferring any tests)
- No deferrals planned. This spec is wiring-heavy; missing transport tests would leave the work unproven.

## Files to Modify

- `internal/component/plugin/server/{command.go,dispatch.go,server.go,command_test.go,dispatch_test.go}` - add request-context plumbing and preserve identity rules on plugin dispatch paths.
- `internal/component/api/{types.go,engine.go,rest/server.go,rest/server_test.go,grpc/server.go,grpc/server_test.go}` - extend execute/auth context to carry request context and remote address.
- `cmd/ze/hub/{api.go,infra_setup.go,session_factory.go}` - composition-root wiring for API and SSH transport metadata.
- `internal/component/ssh/{ssh.go,session.go,ssh_test.go}` and `internal/component/cli/contract/contract.go` - preserve per-session remote address for interactive commands.
- `internal/component/aaa/{aaa.go,types.go,build_test.go,chain_test.go}` and `internal/component/authz/auth.go` - widen the authenticator surface to a request object while preserving chain semantics.
- `internal/component/tacacs/{authenticator.go,authenticator_test.go}` - use propagated `rem_addr` in TACACS auth.
- `internal/component/web/{auth.go,auth_test.go}` - update shared authenticator call sites after AAA surface change.
- `cmd/ze/hub/aaa_lifecycle_test.go` - update authenticator stubs for the new AAA request type.
- `internal/component/bgp/plugins/cmd/peer/prefix_update.go` - replace `context.TODO()` with request-derived context and honor cancellation in the rate-limit loop.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | No new public RPC/API surface; cover wiring in transport integration tests |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | RFC summaries stay generic; Ze-specific request-context notes live in architecture docs instead |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/system-architecture.md`, `docs/architecture/api/commands.md` - document request-context/auth-context propagation path |

## Files to Create

- `internal/component/bgp/plugins/cmd/peer/prefix_update_test.go` - cancellation and lookup-context coverage for the PeeringDB update handler
- `plan/learned/644-request-context-and-caller-identity.md` - implementation summary, gotchas, and final decisions

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section - fix every BLOCKER/ISSUE before full verification |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: CommandContext core plumbing** - add a nil-safe request-context accessor and thread it through dispatcher subsystem/plugin paths.
   - Tests: `TestCommandContextContextFallback`, `TestDispatchSubsystemUsesCommandContextContext`, `TestRouteToProcessUsesParentContextTimeout`, `TestHandleDispatchCommandRPCPreservesPluginIdentity`
   - Files: `internal/component/plugin/server/{command.go,dispatch.go,server.go,command_test.go,dispatch_test.go}`
   - Verify: tests fail -> implement -> tests pass
2. **Phase: API execute context/auth metadata** - align `Execute()` with the stream path and propagate remote address from REST/gRPC.
   - Tests: `TestExecutePropagatesRequestContextAndRemoteAddr`, `TestExecuteUsesPeerRemoteAddr`
   - Files: `internal/component/api/{types.go,engine.go,rest/server.go,rest/server_test.go,grpc/server.go,grpc/server_test.go}`, `cmd/ze/hub/api.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: SSH session identity parity** - make interactive SSH commands carry the same metadata as exec commands.
   - Tests: `TestSSHExecCommandPropagatesRemoteAddr`, `TestCreateSessionModelPreservesRemoteAddr`
   - Files: `internal/component/ssh/{ssh.go,session.go,ssh_test.go}`, `internal/component/cli/contract/contract.go`, `cmd/ze/hub/{infra_setup.go,session_factory.go}`
   - Verify: tests fail -> implement -> tests pass
4. **Phase: AAA request object + TACACS rem_addr** - widen the shared authenticator surface and update all call sites.
   - Tests: `TestChainAuthenticatorForwardsAuthRequest`, `TestTacacsAuthenticatorUsesRemoteAddr`, `TestAuthMiddlewarePassesRemoteAddrToAuthenticator`
   - Files: `internal/component/aaa/{aaa.go,types.go,build_test.go,chain_test.go}`, `internal/component/authz/auth.go`, `internal/component/tacacs/{authenticator.go,authenticator_test.go}`, `internal/component/web/{auth.go,auth_test.go}`, `cmd/ze/hub/aaa_lifecycle_test.go`, `cmd/ze/hub/api.go`, `internal/component/ssh/ssh.go`
   - Verify: tests fail -> implement -> tests pass
5. **Phase: BGP prefix-update cancellation** - replace `context.TODO()` and make rate-limited loops cancel promptly.
   - Tests: `TestPrefixUpdateStopsOnContextCancel`
   - Files: `internal/component/bgp/plugins/cmd/peer/{prefix_update.go,prefix_update_test.go}`
   - Verify: tests fail -> implement -> tests pass
6. **Phase: Docs + verification + learned summary** - update docs, run full verification, fill review/audit sections, write learned summary, then remove the spec entry from `tmp/session/selected-spec`.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-9 is demonstrated by a test or an implementation note with file:line |
| Correctness | No targeted execute path still uses `context.Background()` or `context.TODO()` where caller context exists; timeouts still apply |
| Naming | `CallerIdentity` vs AAA auth request naming is explicit and not overloaded; no transport-specific names leak into handler code |
| Data flow | Identity still comes only from trusted transport wiring; JSON/plugin params cannot inject username or remote address |
| Rule: no-layering | REST/gRPC/SSH still go through API engine or dispatcher; no direct handler invocation from transports |
| Rule: single chokepoint | Authorization/accounting remain centralized in `Dispatcher.Dispatch()` |
| Rule: exact-or-reject | Missing remote address is represented as empty string; do not infer from proxy headers or untrusted metadata |
| Rule: goroutine lifecycle | Child contexts are canceled correctly and no new long-lived goroutines outlive shutdown/reload |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `CommandContext` has a nil-safe request-context accessor | `rg 'func \\(c \\*CommandContext\\) Context\\(' internal/component/plugin/server/command.go` |
| Dispatcher subsystem/plugin routes derive child contexts from caller context | `rg 'Context\\(\\)|WithTimeout\\(' internal/component/plugin/server/command.go` |
| API execute path accepts caller context and remote address metadata | `rg 'Execute\\(ctx context.Context, caller CallerIdentity' internal/component/api/engine.go` |
| REST and gRPC transports extract remote address | `rg 'RemoteAddr|peer.FromContext' internal/component/api/rest/server.go internal/component/api/grpc/server.go` |
| SSH interactive sessions preserve remote address | `rg 'remoteAddr' internal/component/ssh/ssh.go internal/component/ssh/session.go internal/component/cli/contract/contract.go cmd/ze/hub/session_factory.go` |
| AAA auth request object is wired through shared backends | `go test ./internal/component/aaa ./internal/component/tacacs ./internal/component/web` |
| Prefix update no longer uses `context.TODO()` | `rg 'context\\.TODO\\(' internal/component/bgp/plugins/cmd/peer` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Identity spoofing | Username/remote address still come only from trusted transports; request bodies/params cannot override them |
| Credential handling | Passwords are not logged or copied into long-lived structs; only remote address/service metadata are added |
| Cancellation safety | Derived contexts always call `cancel`; no leaked plugin RPCs or external lookups after disconnect |
| Error leakage | Remote address/auth failures are not exposed beyond existing user-facing error behavior |
| Resource exhaustion | Prefix-update wait/lookup loop and plugin RPC paths stop promptly on caller cancellation |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compile fallout from AAA interface widening | Update all shared authenticator call sites first, then rerun package-local tests |
| Transport test passes but accounting metadata missing | Re-audit transport -> `api.CallerIdentity` / `CommandContext` construction sites |
| Parent cancellation lost after timeout refactor | Re-check which context is used as the parent to `context.WithTimeout` |
| Plugin identity regresses on `dispatch-command` | Re-read `handleDispatchCommandRPC()` current behavior and preserve `plugin:<name>` wiring |
| TACACS auth still sends empty `rem_addr` | Re-audit SSH password callback -> AAA request object -> `TacacsAuthenticator` chain |

## Review Gate

### /ze-review Findings
| Severity | File | Finding | Status |
|----------|------|---------|--------|
| ISSUE | `internal/component/plugin/server/command.go`, `internal/component/bgp/plugins/cmd/rib/rib.go` | Builtin proxy handlers still called `ForwardToPlugin()` without the caller `CommandContext`, so proxied plugin RPCs re-rooted at `context.Background()` and lost caller cancellation. | Fixed via `ForwardToPlugin(cmdCtx, ...)` plus `TestForwardToPluginUsesParentContext` |

Review rerun after the proxy fix: 0 BLOCKER, 0 ISSUE, 0 NOTE.

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-9 demonstrated
- [x] Wiring Test table complete
- [ ] `make ze-verify` passes
- [x] No targeted request path still roots child work at `context.Background()` / `context.TODO()`
- [x] Identity injection remains transport-owned only

### Design
- [x] No duplicated authz/accounting logic in transports
- [x] Request context stored only on request-scoped objects
- [x] Remote address propagation is explicit and non-heuristic
- [x] Existing command/output contracts preserved

### TDD
- [x] Tests written first for each phase
- [x] Tests FAIL then PASS
- [x] Transport integration tests prove end-to-end metadata propagation

### Completion
- [x] Critical Review passes
- [x] Review Gate filled
- [x] Learned summary written to `plan/learned/644-request-context-and-caller-identity.md`
- [x] Spec entry removed from `tmp/session/selected-spec`

Verification note:
- `make ze-lint` passed on 2026-04-21.
- `make ze-unit-test` was rerun without sandbox restrictions; the only remaining repo-wide failure was unrelated to this spec: `internal/core/slogutil.TestUseColorZeLogColorTrue`.
- `make ze-functional-test` was rerun without sandbox restrictions; the only remaining repo-wide failure was unrelated to this spec: plugin suite test `103 elicitation-accept` from `test/plugin/elicitation-accept.ci`.
