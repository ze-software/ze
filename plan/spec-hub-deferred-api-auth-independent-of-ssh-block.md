# Spec: resolve API per-user authentication independently of the SSH block

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | `plan/deferrals/fixit-web-auth-deleted-user-survives-reload.md` |
| Handoff | verify |
| Updated | 2026-08-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

REST and gRPC decide whether to install per-user authentication from a boot
snapshot built with `ExtractSSHConfig(tree).Users`. That extractor returns
before it reads users when `environment.ssh` is absent. A config can therefore
declare `system.authentication.user`, enable an API listener, and still produce
no API user authenticator.

The hub already has one correct live user source. `localUsers`, created by
`liveLocalUsers`, merges the boot-time zefs power users with users in the
running `ConfigProvider`. API request authentication already receives that
closure as `apiBuildInputs.UsersLive`, but boot and reload use a second
snapshot to decide whether the authenticator exists.

Resolve the boot user snapshot once from `localUsers`. Use that snapshot for
API mode selection, no-BGP AAA installation, and standalone SSH construction.
At reload, use the existing `apiUsersLive` closure after the candidate tree
enters the provider. Remove the second zefs read, `apiZefsUsersOK`, and every
`main.go` use of `SSHExtractedConfig.Users`.

A boot user-source error must abort startup before any management listener
binds. A non-empty user list is real authentication, so the generic management
guard can permit an authenticated gRPC listener. REST remains loopback-only
because it has no TLS transport. When no BGP reactor exists, the hub must still
install the AAA bundle so API and MCP authorization does not depend on SSH.
Successful per-user authentication must bind its resolved profiles to
`AuthResult.Authorizer`. Command dispatch must carry that immutable view.
Shared-token and loopback no-auth callers must use one server-injected reserved
API identity. Authorization permits that identity only after the transport has
validated the token, while the existing read-only gate continues to deny
no-auth writes.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - guard and failed-read requirements
  → Constraint: a failed user-source read must say so and deny. It must not become a valid-looking empty list.
  → Constraint: tests must drive boot and reload entry points, not only `buildUserAuthenticator`.
- [ ] `ai/rules/simplicity.md` - smallest fully correct design
  → Decision: reuse `localUsers`, `apiUsersLive`, result-scoped AAA authorization, and the reserved identity namespace. Do not add a credential source, config leaf, environment variable, user snapshot, or username-keyed profile store.
  → Constraint: token and no-auth compatibility must not weaken per-user profile checks.
- [ ] `docs/architecture/config/syntax.md` - config root ownership
  → Constraint: `system` holds identities. `environment.ssh` decides only whether SSH runs.
- [ ] `docs/architecture/api/architecture.md` - REST/gRPC shared authentication
  → Decision: REST and gRPC continue to share one `apiShared` and one auth decision.
- [ ] `docs/guide/api.md` - operator-facing API authentication contract
  → Constraint: document both zefs and `system.authentication.user` as per-user sources, with the exact current auth-mode output.
- [x] `plan/spec-ssh-optional-composition.md` - dependent optional-transport composition
  → Decision: this spec owns the always-built authentication extraction/schema, accepted AAA/API generation, no-BGP startup, reload transaction, and response-completion contract. The SSH spec owns build tags, optional SSH transport registration, the SSH public-key schema augment, and SSH composition tests.
  → Constraint: pure SSH compile seams do not enter this closure. Shared SSH callers changed only where the accepted response-completion signature requires them.

### RFC Summaries (Scope: protocol)
- [ ] N-A - local credential resolution changes no HTTP, gRPC, or SSH wire format.

**Key insights:**
- `localUsers` is already created before API resolution and already passed to `apiBuildInputs.UsersLive` and `mgmtAuthInputs.apiUsersLive`.
- `liveLocalUsers` treats a missing `system` root as no configured users, but reports a missing provider or failed read as an error.
- `runYANGConfig` must drive zefs/config precedence tests. A hand-built merged list does not prove the boot producer or recovery-profile wiring.
- `buildUserAuthenticator` calls `authz.LocalAuthenticator` directly. It must use the same reusable AAA result-authorizing wrapper as `aaa.BackendRegistry.Build`, or zefs recovery profiles are discarded before authorization.
- REST `withAuth` and gRPC `checkAuth` give a non-nil per-user authenticator precedence over the shared token.
- Shared token and no-auth currently report username `api`. Installing a strict `authz.Store` denies that unassigned name unless the transports inject a reserved shared-API identity after their credential gate.
- `infraSetup` installs AAA only when a BGP reactor exists. The standalone path installs it only inside the SSH branch. No-BGP API and MCP therefore need an always-on `main.go` install.
- REST always rejects non-loopback binds because it has no TLS transport. Authenticated non-loopback proof belongs to gRPC with TLS.
- Way 1 from the prior draft is selected. Way 2 added an unrequested opt-in only to preserve a false refusal after real authentication exists.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/main.go` - creates `localUsers`, then loads API users a second time; it also installs no-BGP AAA only inside the standalone SSH branch.
- [ ] `cmd/ze/hub/main_servers.go` - `liveConfigUsers` reads the current `system` root; `liveLocalUsers` merges it with the zefs snapshot and reports source errors.
- [ ] `cmd/ze/hub/aaa_lifecycle.go` - `liveAAABundleAuthorizer` fails open when no bundle was installed.
- [ ] `cmd/ze/hub/infra_setup.go` - installs AAA when the BGP reactor invokes the infrastructure hook.
- [ ] `cmd/ze/hub/api.go` - `buildUserAuthenticator` returns nil for an empty decision list, uses `UsersLive` per request, and binds successful `AuthResult.Profiles` to the caller identity.
- [ ] `internal/component/aaa/login_profiles.go`, `types.go`, and `reserved.go` - the registry build owns result-scoped authorization and the untypeable reserved identity namespace.
- [ ] `internal/component/authz/authz.go` - strict store authorization evaluates explicit session profiles, recovery profiles, config assignments, and recognized server-injected identities.
- [ ] `internal/component/api/rest/auth.go` - per-user authentication precedes token authentication; token and no-auth callers currently use username `api`.
- [ ] `internal/component/api/grpc/server.go` - gRPC has the same precedence and default username.
- [ ] `cmd/ze/hub/mgmt_auth_reload.go` - carries `apiUsersLive` but re-reads zefs and `ExtractSSHConfig(tree).Users`; `apiZefsUsersOK` exists only for that second read.
- [ ] `cmd/ze/hub/main_reload.go` - installs the candidate tree into `ConfigProvider` before listener auth reloaders run and restores it on rollback.
- [ ] `cmd/ze/hub/mgmt_guard.go` - permits an authenticated surface and refuses unauthenticated non-loopback or unresolved listeners.
- [ ] `internal/component/api/rest/server.go` - rejects every non-loopback listener because REST has no TLS.
- [ ] `cmd/ze/hub/mgmt_auth_reload_test.go`, API transport tests, and `test/ui/api-user-removed-by-reload.ci` - current unit and running-path coverage.
- [ ] `docs/guide/api.md` - currently says per-user mode requires only `ze init`, attributes the list to SSH, and shows stale output.

**Behavior to preserve:**
- A configured API token keeps authenticating when no users exist. A non-empty per-user authenticator continues to take precedence over the token.
- No users and no token keeps the exact `API auth mode: NONE` warning, loopback read-only behavior, and management-guard refusal.
- Shared-token writes and no-auth reads keep working when unrelated authorization profiles exist.
- `buildUserAuthenticator` continues to use the live source per request, so deleted users stop authenticating.
- Removing the whole `api-server` block on reload leaves running credentials unchanged.
- Reload failure restores the previous credentials and listener state.
- The three existing `API auth mode:` messages keep their exact text.

**Behavior to change:**
- Boot resolves the merged user snapshot once from `localUsers`; an error aborts startup.
- API reload gets users by calling `mgmtAuthInputs.apiUsersLive`, not by loading zefs and reading `ExtractSSHConfig`.
- No-BGP startup installs AAA independently of SSH config or build presence.
- Standalone SSH construction and API mode selection consume the shared boot snapshot, not `sshCfg.Users`.
- Successful per-user authentication binds local and zefs recovery profiles to the returned caller identity before authorization.
- REST and gRPC inject the reserved shared-API identity only for validated shared-token requests or no-auth loopback requests. Per-user requests keep their authenticated username.
- A config user works with REST and gRPC when no SSH block exists.
- Authenticated non-loopback gRPC passes the generic guard and its own TLS check. REST remains loopback-only.
- `apiZefsUsersOK` and its obsolete tests are removed.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Boot: a config file or stdin config reaches `runYANGConfig` and populates the shared `ConfigProvider`.
- Reload: SIGHUP reaches `runReload`, which loads a candidate tree and refreshes the same provider.

### Transformation Path
1. Boot loads zefs users once into `zefsAuthUsers`.
2. `liveConfigUsers` reads `system.authentication.user` from the provider.
3. `liveLocalUsers` closes over both sources. Boot calls it once. An error returns from `runYANGConfig` before listeners bind.
4. The boot snapshot feeds API mode selection and the one no-BGP AAA bundle. Optional services, including standalone SSH when compiled, consume that accepted bundle and snapshot.
5. When no BGP reactor exists, `runYANGConfig` builds and publishes one AAA bundle before API/MCP services start. Later infrastructure hook reentry reuses the installed bundle rather than rebuilding it.
6. A non-empty snapshot enables per-user mode and reaches `buildUserAuthenticator` beside the same live closure.
7. `buildUserAuthenticator` wraps `authz.LocalAuthenticator` with the exported AAA profile-authorizing wrapper. Successful authentication binds `AuthResult.Profiles` and the accepted local credential generation to `AuthResult.Authorizer`.
8. REST `withAuth` and gRPC `checkAuth` publish a real username for per-user mode and the reserved shared-API identity for token or no-auth mode.
9. The read-only caller bit rejects no-auth mutations before the reserved identity reaches authorization. Token callers retain full shared-credential authority.
10. `apiAuthed` reaches `checkMgmtListeners` and `markMgmtAuth` unchanged. REST remains loopback-only; gRPC separately enforces TLS for non-loopback binds.
11. REST and gRPC resolve the live accepted `api.Authentication` provider for every request or RPC.
12. On reload, `runReloadContext` places the candidate tree in the provider, builds a fail-closed candidate combining credentials, local policy, retained/configured API token, and external AAA, and keeps it unpublished.
13. `apiAuthReloader` classifies both present and absent API blocks from the candidate users plus the retained accepted token before any listener migration.
14. Listener migration runs against the staged candidate. The accepted identity is published only after every listener and later reload step succeeds. On failure, listener restoration must succeed before the prior identity is restored; restoration failure leaves the candidate unpublished and the surface fail closed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → live provider | `applyLoadedTreeToProvider` writes roots before listener migration | `runReload` producer order and reload tests |
| Provider and zefs → boot snapshot | One `localUsers` call returns merged users or aborts startup | Spawned boot tests cover distinct names, collision precedence, and recovery authority |
| Authentication → authorization | The AAA wrapper binds successful `AuthResult.Profiles` to `AuthResult.Authorizer` | API, concurrent-session, and running zefs recovery tests |
| Shared API credential → authorization | REST/gRPC inject an untypeable reserved identity after token/no-auth classification | Transport tests with an unrelated authorization profile |
| Boot snapshot → AAA and API | `main.go` installs no-BGP AAA and classifies API auth from the same list | No-BGP UI workflow |
| Hub → REST | `apiShared.Authenticator` reaches the loopback REST server | UI correct/wrong credential and reserved-identity assertions |
| Hub → gRPC | The same authenticator reaches `grpcBuildImpl` and a real RPC | gRPC builder integration test |

### Integration Points
- `localUsers` - the one request-time source and producer of the boot snapshot.
- `buildAAABundle`, `installBootAAABundle`, and the infrastructure reentry guard - create one boot-owned AAA bundle and reuse it.
- `aaa.WithProfileAuthorizer` - the reusable authentication choke point used by the AAA registry and API local authentication.
- `aaa.AuthResult.Authorizer` - the immutable authorization view carried by each authenticated request or session.
- `aaa.ReservedSharedAPIUsername` - the untypeable identity REST and gRPC inject only after shared-token or no-auth classification.
- `authz.Store.AuthorizeWithProfiles` - evaluates bound profiles against the live accepted local generation.
- `mgmtAuthInputs.apiUsersLive` - the shared candidate user source carried into reload.
- `buildUserAuthenticator` - receives the accepted decision list and live source, then returns result-scoped authorization.
- `RESTServer.authentication` and `GRPCServer.authentication` - resolve the accepted request-authentication generation at request time.
- `checkMgmtListeners` - unchanged generic guard; consumes the candidate classification.
- `grpcBuildImpl` - proves the shared authenticator on the second transport.
- `listenerMigrator.migrateListeners` - stages listener moves and returns retryable undo for every successfully applied move.
- `runReloadContext` - publishes the accepted identity only after listener migration and all later reload work succeed.
- `plugin.RenderedResponse.TransportComplete` - ends accounting after the accepted action's response reaches its transport, using the same accountant that emitted START.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | User data stays behind `liveLocalUsers`; API and AAA code do not parse YANG paths |
| No unintended coupling | Yes | `main.go` stops reading `SSHExtractedConfig.Users`; the dependent spec removes the remaining field |
| No duplicated functionality | Yes | One boot snapshot and the existing live closure replace second reads and SSH-owned snapshots |
| Zero-copy preserved where applicable | N-A | Startup and reload lists, not a wire hot path |
| Registration over hardcoding | Yes | AAA still builds through its backend registry; the generic guard stays generic |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | `localUsers` exists before no-BGP AAA and API boot resolution | `runYANGConfig` constructs `localUsers` before standalone AAA and API listener/auth resolution | Boot needs a second source | `cmd/ze/hub/main.go` `runYANGConfig` | confirmed |
| A-2 | The reload closure sees the candidate tree | `runReload` calls `applyLoadedTreeToProvider` before `listenerMigrator.reloadListeners` | Reload authenticates against the old tree | `cmd/ze/hub/main_reload.go` `runReload` | confirmed |
| A-3 | A non-empty list gives both transports real authentication | `buildUserAuthenticator` returns non-nil for a non-empty decision list; REST `withAuth` and gRPC `checkAuth` give it precedence over token/no-auth | Guard passes an unauthenticated listener | `cmd/ze/hub/api.go` `buildUserAuthenticator`; REST `withAuth`; gRPC `checkAuth` | confirmed |
| A-4 | A missing `system` root is valid no-user config, not a source error | `liveLocalUsers` handles `errNoSystemConfigRoot` by retaining zefs users and returning no error | No-user configs fail boot | `TestLiveLocalUsersKeepsPowerUsersWithNoSystemRoot` | confirmed |
| A-5 | REST cannot supply the remote guard proof | `NewRESTServer` rejects every address for which `api.IsLoopbackAddr` is false before constructing the server | Planned workflow cannot start | `internal/component/api/rest/server.go` `NewRESTServer` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Boot source error becomes a valid anonymous mode | Daemon starts after the source error | Return a startup error before AAA or listeners are installed |
| R-2 | Reload reads users before provider refresh | Added user appears only on the second reload | Test the producer order through a provider-backed reloader test |
| R-3 | The second zefs read survives | `loadZefsUsers` remains in `apiAuthReloader` or the API boot block | Remove it and `apiZefsUsersOK`; use only the shared snapshot/closure |
| R-4 | No-BGP startup still depends on SSH | API RBAC test allows a denied command | Install AAA outside the SSH branch and wire the live authorizer |
| R-5 | Only successful credentials are tested | A no-op authenticator or absent middleware passes | Assert correct credentials succeed and wrong credentials fail on both transports |
| R-6 | A removed user revives after listener rebuild | Existing reload UI test fails | Keep `UsersLive` on every rebuilt authenticator |
| R-7 | REST is used for remote exposure proof | Server rejects the bind before the guard result is observable | Keep REST loopback and use authenticated TLS gRPC |
| R-8 | The SSH optionality spec edits `main.go` again | File lists overlap | This spec removes every `main.go` field consumer; the dependent spec owns only config-infra |
| R-9 | API local authentication drops zefs recovery profiles | Zefs credentials authenticate but command authorization denies | Bind the authentication result with the same AAA wrapper used by the registry build |
| R-10 | Strict AAA denies shared token or no-auth identity | Token writes or no-auth reads fail when an unrelated profile exists | Inject one reserved shared-API identity only after transport classification; keep the read-only gate before authorization |
| R-11 | A remote per-user caller spoofs the reserved identity | A configured username bypasses profiles | Keep the NUL-prefixed namespace untypeable and reject reserved usernames at the authentication choke point |
| R-12 | Authorization profiles are stored globally by username | Concurrent sessions with one username change each other's authority | Carry `AuthResult.Authorizer` through every command path and keep remote profiles immutable per result |
| R-13 | A recovery session survives a local credential generation change | An old bootstrap session retains break-glass access | Check the accepted local generation on every recovery authorization decision |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | API credentials or authorization can silently disappear, reload can revive access, or remote gRPC can be misclassified. The safe failure is a refused boot or reload with an error |
| How is it reverted? | Single commit revert. No config, storage, or wire migration |
| Who else touches this path? | `spec-ssh-optional-composition` depends on the completed `main.go` cutover and owns no API or `main.go` file |

## Cross-Specification Ownership

| Surface | This spec | SSH optionality spec |
|---------|-----------|----------------------|
| Accepted local identity, API token, external AAA, no-BGP bundle ownership, and reload publication | Owns | Must not edit |
| `cmd/ze/hub/main.go`, `main_reload.go`, `listener_migrate.go`, `api.go`, and API service construction | Owns | Consumes only through optional composition |
| Always-built authentication extraction and base authz YANG | Owns because API boot must parse users without `ze_ssh` | Consumes |
| SSH public-key YANG augment, SSH registration seams, build tags, and absent/present composition tests | Must not edit | Owns |
| Shared command response-completion carrier and direct/socket transport callers | Owns as the accounting and accepted-action boundary | SSH caller changes are dependencies only |
| `SSHExtractedConfig.Users` and transport-only schema cleanup | Must not consume | Owns removal |
| API, AAA, shared authentication, and RBAC documentation | Owns corrected accepted-generation semantics | SSH spec owns only optional transport/build documentation |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze -` boots with config users, REST, no BGP, and no SSH block | → | boot snapshot → no-BGP AAA → REST authenticator/authorizer | `test/ui/api-user-removed-by-reload.ci` |
| Actual boot merges distinct and colliding zefs/config users | → | `runYANGConfig` → shared snapshot → API authentication → recovery authorization | Two boot stages in `test/ui/api-user-removed-by-reload.ci` |
| Successful API local login binds profiles | → | `buildUserAuthenticator` → `aaa.WithProfileAuthorizer` → `AuthResult.Authorizer` → `authz.Store.AuthorizeWithProfiles` | `TestBuildAPIAuthenticationBindsRecoveryProfiles` |
| Shared API callers cross strict AAA | → | REST/gRPC credential classification → reserved identity → authorizer/read-only gate | `TestRESTSharedIdentitySurvivesStrictAuthorization` and `TestGRPCSharedIdentitySurvivesStrictAuthorization` |
| Per-user command denial reaches transport status | → | `buildAPIEngine` maps typed and canonical-response dispatcher denials to `api.ErrUnauthorized` | `TestBuildAPIEngineTranslatesDispatcherAuthorizationDenial`, `TestAPIDispatchErrorTranslatesNilErrorDenialResponse`, and running REST `show system goroutines summary` HTTP 403 |
| gRPC builds from the same no-SSH user source | → | boot snapshot → `apiShared` → `grpcBuildImpl` → real RPC | `TestGRPCBuildAuthenticatesConfigUserWithoutSSH` |
| Boot user source errors | → | boot snapshot resolution → startup refusal | `TestAPIBootUsersFailClosed` |
| API reload after provider user change | → | `apiUsersLive` → `apiAuthReloader` → `UpdateAuth` | `TestAPIAuthReloaderUsesLiveUsersWithoutSSHBlock` |
| API reload when live source errors | → | `apiAuthReloader` fail-closed return | `TestAPIAuthReloaderFailsClosedWhenLiveUsersUnreadable` |
| Concurrent same-name sessions | → | authentication result → caller/session context → command dispatch | AAA, SSH, web, and API isolation tests |
| Recovery credential generation changes | → | accepted identity publication → bound recovery authorizer | recovery expiry tests |
| Running API reload removes a user | → | provider refresh → rebuilt auth → request path | `test/ui/api-user-removed-by-reload.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No-BGP config declares a user, authorization profile, loopback REST, no SSH block, no token, and no zefs user | Boot reports `API auth mode: per-user (1 users)`, installs AAA before REST, and starts |
| AC-2 | Correct and incorrect credentials reach AC-1 REST | Correct credentials can run an allowed command; wrong credentials fail; the assigned profile denies a disallowed command |
| AC-3 | A no-SSH config user reaches the gRPC builder | An actual RPC with correct credentials succeeds and wrong credentials return `Unauthenticated` |
| AC-4 | Authenticated gRPC binds non-loopback with TLS | The generic guard accepts the authenticated classification and the gRPC server accepts the TLS listener |
| AC-5 | Boot user resolution returns an error | Startup reports the error and exits before AAA or any management listener is installed, even when a token exists |
| AC-6 | The actual boot path has a zefs power user and config users, then boots once with distinct names and once with a collision | Distinct users both authenticate and the zefs user can execute through its recovery profile; on collision the config credential and profile win and the zefs credential/recovery grant do not |
| AC-7 | API token exists, no users exist, and an unrelated authorization profile is installed | Token mode and exact stderr remain unchanged; REST and gRPC token callers use the reserved shared-API identity and can execute writes |
| AC-8 | No token or users exist and an unrelated authorization profile is installed | NONE warning remains exact; loopback REST/gRPC reads succeed through the reserved shared-API identity, writes remain read-only denied, and non-loopback management remains refused |
| AC-9 | Reload provider contains a config user and no SSH block | `apiAuthReloader` returns authenticated intent with a working per-user authenticator |
| AC-10 | The live user source errors during reload | Reload fails, says why, and existing running credentials remain installed |
| AC-11 | Reload removes the `api-server` block | API auth reloader returns no intent and leaves credentials unchanged |
| AC-12 | Reload changes or removes a user | The new user authenticates and the removed user stops authenticating without restart |
| AC-13 | Source audit after implementation | `main.go` and API reload contain no `sshCfg.Users`, `ExtractSSHConfig` user read, API-specific `loadZefsUsers`, or `apiZefsUsersOK`; no-BGP AAA uses the shared boot snapshot |
| AC-14 | API guide is checked | It documents zefs/config users, REST loopback, authenticated TLS gRPC, live-source semantics, and exact output |
| AC-15 | Two concurrent remote-authenticated sessions use the same username but resolve different profiles | Each session keeps its own immutable authorization view; a later login does not change the established session; no username-keyed global profile state exists |
| AC-16 | An established zefs recovery session remains open while a new local credential generation becomes accepted | The old session loses the reserved recovery grant on its next authorization decision |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs API without BGP or SSH, logs in, and executes an allowed REST command | config → boot snapshot → AAA → REST auth → authorizer → dispatcher | Strengthened API reload UI workflow |
| 2 | Uses a wrong password or a command denied by the assigned profile | REST auth/authorizer → refusal | Same UI workflow |
| 3 | Uses the zefs recovery user beside and colliding with a config user | zefs/config → `runYANGConfig` → result-scoped authorization → strict authorizer | Two boot stages in the same UI workflow |
| 4 | Uses a shared token or loopback no-auth mode while profiles exist | transport gate → reserved shared-API identity → authorizer/read-only gate | REST and gRPC transport tests |
| 5 | Uses the same config user over gRPC | boot snapshot → shared API state → gRPC interceptor → RPC | `TestGRPCBuildAuthenticatesConfigUserWithoutSSH` |
| 6 | Changes and removes users through reloads | candidate tree → provider → reloader → `UpdateAuth` → REST decisions | Strengthened API reload UI workflow |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAPIBootUsersFailClosed` | `cmd/ze/hub/main_servers_test.go` | AC-5 boot source errors remain distinct from an empty snapshot for the `runYANGConfig` caller | hub package passed |
| `TestBuildAPIAuthenticationBindsRecoveryProfiles` | `cmd/ze/hub/api_test.go` | AC-6 recovery binding through the reusable wrapper | hub package passed |
| `TestBuildAPIEngineTranslatesDispatcherAuthorizationDenial` | `cmd/ze/hub/api_test.go` | AC-2 and AC-6 typed dispatcher denial crosses the shared API seam as a transport permission denial | hub package passed |
| `TestAPIDispatchErrorTranslatesNilErrorDenialResponse` | `cmd/ze/hub/api_test.go` | AC-2 exact canonical denial response with nil Go error maps to `api.ErrUnauthorized`; near misses remain unchanged | hub package passed; running REST denial returned HTTP 403 |
| `TestStoreAuthorizeReservedSharedAPIIdentity` | `internal/component/authz/authz_test.go` | AC-7 and AC-8 strict-store compatibility without weakening ordinary identities | authz package passed |
| Result-authorizer, reserved-name, recovery-expiry, and concurrent-session tests | `internal/component/aaa/login_profiles_test.go`, `internal/component/authz/authz_test.go`, `internal/component/ssh/ssh_test.go`, `internal/component/web/rbac_test.go`, and `cmd/ze/hub/api_test.go` | AC-15 and AC-16 result isolation across every transport path | all listed packages passed |
| `TestRESTSharedIdentitySurvivesStrictAuthorization` | `internal/component/api/rest/server_test.go` | AC-7 and AC-8 with an unrelated profile installed | REST package passed |
| `TestGRPCSharedIdentitySurvivesStrictAuthorization` | `internal/component/api/grpc/server_test.go` | AC-7 token write plus AC-8 no-auth read success and write refusal through real RPCs with strict AAA | gRPC package passed |
| `TestGRPCBuildAuthenticatesConfigUserWithoutSSH` | `cmd/ze/hub/service_grpc_test.go` | AC-3 and AC-4 through a real RPC | hub package passed |
| `TestAPIAuthReloaderUsesLiveUsersWithoutSSHBlock` | `cmd/ze/hub/mgmt_auth_reload_test.go` | AC-9 and working authenticator material | hub package passed |
| `TestAPIAuthReloaderFailsClosedWhenLiveUsersUnreadable` | same | AC-10; replacement for the removed second-zefs-read coverage | hub package passed |
| `TestAPIAuthReloaderProceedsWithTokenAndNoUsers` | same | AC-7, per-user precedence, and valid empty live source | hub package passed |
| `TestAPIAuthReloaderSilentWithoutBlock` | same | AC-11 | hub package passed |
| `TestBuildAAABundleUsesInitialLiveLocalAuthorization`, `TestLiveLocalAuthorizerFollowsStoreSwap`, `TestLiveLocalAuthorizerNilStoreAllows`, and `TestCloseAAABundleClearsLiveLocalAuthorization` | `cmd/ze/hub/aaa_lifecycle_test.go` | Initial local policy, atomic replacement, no-RBAC allow behavior, and package-global state isolation | hub package passed |
| `TestRunReloadSuccessfulReloadSwapsLiveAuthorization` and `TestRunReloadFailedReloadPreservesLiveAuthorization` | `cmd/ze/hub/main_reload_auth_test.go` | Successful reload publishes the parsed policy only at the final success boundary; failed reload retains the old store | hub package passed |
| `TestAPILoginAdmitsPowerAndConfigUsers` | `cmd/ze/hub/auth_e2e_test.go` | Authenticator helper coverage only; does not satisfy AC-6 boot wiring | existing, retain |
| `TestLiveConfigUsers*` and `TestLiveLocalUsers*` | `cmd/ze/hub/main_servers_test.go` | Provider freshness, missing-root behavior, and source errors | existing |

### Boundary Tests
N-A. This change adds no numeric or size input. Empty, non-empty, and failed
user-source states are explicit acceptance cases.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `api-user-removed-by-reload` | `test/ui/api-user-removed-by-reload.ci` | AC-1, AC-2, AC-6, and AC-12: no BGP/SSH block, AAA allow/deny, actual zefs/config boot precedence and recovery, reload removal, listener move, and an unchanged authenticated user losing a command after profile reload | passed in `make ze-functional-ui-test` (169/169, 10 expected skips) |
| `mgmt-guard-api-env-started-settings-survive` | `test/plugin/mgmt-guard-api-env-started-settings-survive.ci` | AC-4, AC-7, AC-8, and absent-block reload classification for environment-started listeners | passed in `make ze-functional-plugin-test` (605/605, 44 expected skips) |

### Interop Tests
N-A. HTTP and gRPC wire formats do not change. The UI workflow and actual gRPC
RPC exercise the changed credential path.

## Files to Modify
- Hub accepted-generation and lifecycle producers: `cmd/ze/hub/aaa_lifecycle.go`, `aaa_lifecycle_test.go`, `aaa_authenticator_web_test.go`, `api.go`, `api_test.go`, `api_infra.go`, `api_infra_test.go`, `auth_e2e_test.go`, `infra_setup.go`, `listener_migrate.go`, `listener_migrate_test.go`, `main.go`, `main_reload.go`, `main_reload_auth_test.go`, `main_reload_pki_test.go`, `main_reload_test.go`, `main_servers.go`, `main_servers_test.go`, `main_servers_webonly_test.go`, `mgmt_auth_reload.go`, `mgmt_auth_reload_test.go`, `service_grpc.go`, `service_grpc_test.go`, `service_rest.go`, `service_web.go`, `service_web_users_test.go`, `service_ssh.go`, `session_factory.go`, and `session_factory_test.go`.
- AAA, authentication, and external-backend producers: `internal/component/aaa/aaa.go`, `build_test.go`, `login_profiles.go`, `login_profiles_test.go`, and `types.go`; `internal/component/authz/auth.go`, `auth_test.go`, `authz.go`, `authz_test.go`, and `register.go`; `internal/component/radius/authenticator.go`, `authenticator_test.go`; and `internal/component/tacacs/authenticator.go`, `authenticator_test.go`, `authorizer.go`, `authorizer_test.go`, and `register.go`.
- Always-built config ownership: `internal/component/config/infra/authz.go`, `authz_test.go`, `authz_no_ssh_test.go`, `hook.go`; and `internal/component/authz/yang/ze-authz-conf.yang`, `schema_test.go`.
- API transports: `internal/component/api/types.go`; `internal/component/api/rest/auth.go`, `auth_test.go`, `server.go`, `server_test.go`; and `internal/component/api/grpc/auth_reload_test.go`, `server.go`, `server_test.go`.
- Accepted-action completion and dispatch: `internal/component/plugin/types.go`, `dispatch.go`, `dispatch_test.go`; `internal/component/plugin/server/benchmark_test.go`, `command.go`, `dispatch.go`, `dispatch_registry.go`, `dispatch_test.go`, `server.go`, `startup_autoload_test.go`, `system.go`, `system_test.go`; and `pkg/plugin/rpc/bridge.go`, `bridge_test.go`, `types.go`.
- Completion-carrier callers and focused tests: `internal/chaos/mcp/tools.go`; `internal/component/cli/client/main.go`, `verb_tree.go`; `internal/component/config/cli/cmd_edit.go`; `internal/component/lg/server.go`; `internal/component/mcp/provider_test.go`, `streamable_tools.go`, `tools.go`, `tools_test.go`; `internal/component/ssh/ssh.go`, `ssh_test.go`; and `internal/component/web/auth.go`, `cli_terminal.go`, `handler_admin.go`, `handler_admin_test.go`, `handler_config.go`, `handler_config_test.go`, `handler_l2tp.go`, `handler_tools.go`, `page_bgp_summary.go`, `page_dashboard.go`, `page_logs.go`, `page_tools.go`, `rbac.go`, `rbac_test.go`, `register_l2tp.go`, `snapshot_views.go`.
- Running-path evidence: `test/ui/api-user-removed-by-reload.ci`, `test/plugin/mgmt-guard-api-env-started-settings-survive.ci`, and `test/weakened.md`.
- Documentation and generated reverse index: `ai/digests/aaa-auth.md`, `ai/CODE-TO-DOCS.md`, `docs/architecture/aaa-tacacs.md`, `docs/architecture/api/architecture.md`, `docs/guide/api.md`, `docs/guide/authentication.md`, and `docs/guide/operator-access-rbac.md`.
- Required closure-gate repairs: `scripts/dev/validate.py`, `validate_test.py`, `verify_wiring_docs.py`, and `verify_wiring_docs_test.py`.
- Closure records: `plan/deferrals/fixit-web-auth-deleted-user-survives-reload.md`, this spec, and only this session's API rows in `plan/journal/component-rebuilt-during-reload.md`, `guard-added-to-one-half-of-a-pair.md`, `rollback-forgets-partial-apply.md`, `gate-verdict-depends-on-the-machine.md`, and `hook-existing-patterns-false-positive.md`.

## Files to Create
- `cmd/ze/hub/infra_setup_auth_test.go` - proves BGP infrastructure reentry reuses the boot-owned AAA bundle.

## Files Explicitly Not Modified
- Pure SSH optional-composition seams and tests: `cmd/ze/hub/build_tag_ssh_absent_test.go`, `build_tag_ssh_present_test.go`, `build_tag_ssh_probe_test.go`, `service_registry.go`, `ssh_infra.go`, `register_ssh.go`, and `ssh_pubkey_live_test.go`.
- SSH-owned extraction and schema augment files: `internal/component/config/infra/ssh.go`, `ssh_test.go`, and `internal/component/ssh/yang/`.
- SSH build inventory and closure material: `scripts/codegen/plugin_imports.go`, `scripts/checks/staticcheck_feature_matrix.go`, SSH-only docs, journals, rules, RFC work, and `plan/spec-ssh-optional-composition.md`.
- `cmd/ze/hub/mgmt_guard.go` - its generic fail-closed producer remains unchanged.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | The always-built `internal/component/authz/yang/ze-authz-conf.yang` owns base authentication users so API boot is independent of `ze_ssh`; SSH augment ownership remains outside this spec |
| YANG validation | Yes | `internal/component/config/infra/authz_test.go` parses base users and profiles through the real schema |
| YANG custom validators | No | None added |
| CLI commands/flags | No | No command or flag changed; existing CLI callers only acknowledge rendered-response delivery |
| CLI grammar | No | None |
| Editor autocomplete | No | None |
| Functional test for API | Yes | `test/ui/api-user-removed-by-reload.ci` drives no-BGP/no-SSH-block boot, REST auth, AAA, reload, listener movement, and policy invalidation; `service_grpc_test.go` drives a real RPC |
| Pipe completeness | Yes | `RenderedResponse.TransportComplete` is propagated through direct and socket-backed command consumers |
| Env var registration | No | No variable added |
| Doctor check | No | No dependency, external socket, file, or listener setting added |
| Prometheus counters | No | None |
| BGP family surface | N-A | None |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature? | No | Corrects credential, authorization, accounting, and reload behavior |
| 2 | Config syntax changed? | No | Existing `system.authentication` syntax moved to its always-built owner without changing accepted text |
| 3 | CLI changed? | No | No command, output, or flag changed |
| 4 | API/RPC changed? | Yes | Authentication behavior in `docs/guide/api.md` and accepted-generation flow in `docs/architecture/api/architecture.md`; endpoints, envelopes, and wire formats are unchanged |
| 5 | Plugin changed? | No user contract | Internal dispatch completion now follows response delivery; RPC payloads and plugin SDK behavior remain unchanged |
| 6 | User guide page? | Yes | `docs/guide/api.md`, `docs/guide/authentication.md`, and `docs/guide/operator-access-rbac.md` |
| 7 | Wire format changed? | No | None |
| 8 | SDK/protocol changed? | No | Completion is an in-process carrier; plugin RPC types preserve the wire contract |
| 9 | RFC behavior changed? | N-A | None |
| 10 | Test infrastructure changed? | No | Existing Go, UI, and plugin test vehicles |
| 11 | Daemon comparison affected? | No | No comparison claim |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md` and `docs/architecture/aaa-tacacs.md` describe one accepted generation, no-BGP bundle ownership, reload publication, and live accounting |
| 13 | Route metadata changed? | No | None |
| 14 | Metrics changed? | No | None |
| 15 | Registered inventory changed? | No | None |
| 16 | Changed source anchors? | Yes | API, AAA, TACACS, authentication, and RBAC documents now anchor the accepted-generation producers |
| 17 | Existing examples cover this area? | Yes | `docs/guide/api.md` corrects per-user sources, transport constraints, live reload semantics, and exact auth-mode output |

## Implementation Steps

1. **Phase: Wiring** - strengthen the UI workflow and add boot-error, zefs/config precedence, shared-identity, gRPC, and reload cases.
   - Verify: each test fails on its intended missing source, authorization binding, AAA policy, transport, or reload behavior.
2. **Phase: API identity and result authorization** - export and reuse the AAA result-authorizing wrapper; add the reserved shared-API identity; inject it from REST and gRPC token/no-auth paths.
   - Verify: zefs recovery authorization; typed and canonical-response dispatcher denial translation; token writes; no-auth reads; read-only mutation refusal; per-user precedence; and reserved-name spoof refusal.
3. **Phase: Boot source and no-BGP AAA** - resolve the shared boot snapshot once, abort on error, install AAA outside the SSH branch, and pass the snapshot to API and standalone SSH.
   - Files: `main.go` and boot tests.
   - Verify: AC-1 through AC-8 and AC-13, including actual boot collision and recovery cases.
4. **Phase: Reload source** - call `apiUsersLive` and remove `apiZefsUsersOK`.
   - Files: `mgmt_auth_reload.go` and tests.
   - Verify: AC-9 through AC-12; provider update precedes reloader use.
5. **Phase: Atomic accepted generation** (complete) - credentials, local policy, API token, external AAA, listener exposure, and accepted bundle publication now form one fail-closed reload transaction. Listener rollback retains retryable undo until restoration succeeds.
   - Evidence: hub race and non-race package tests, REST/gRPC provider tests, and focused listener rollback tests passed.
6. **Phase: Live authorization, accounting, and response completion** (complete) - requests resolve the accepted bundle live; recovery sessions validate the accepted local generation; accounting STOP uses the START producer and fires only after transport delivery.
   - Evidence: focused AAA, TACACS, plugin-dispatch, REST, gRPC, web, MCP, CLI, LG, SSH, and hub tests passed in the combined package and functional suites.
7. **Phase: Documentation, review fixes, and mutation proof** (complete) - documentation reflects the accepted-generation flow. The final review round found 0 BLOCKER and 0 ISSUE after repairs for absent-block exposure, partial listener rollback, duplicate no-BGP AAA construction, session invalidation, split accounting producers, and response-ordering.

### Critical Review Checklist
| Check | What to verify |
|-------|----------------|
| Completeness | AC-1 through AC-16 have named evidence |
| Feature completeness | Boot, zefs/config precedence, recovery authority and expiry, session isolation, no-BGP AAA, token/no-auth compatibility, REST, gRPC, reload, source errors, and user removal are covered |
| Correctness | One boot snapshot feeds AAA, standalone SSH, and API; the live closure validates requests and reloads; every API auth mode reaches the correct authorization identity |
| Naming | No shared user source is named for SSH; reserved API identity names its shared-credential purpose |
| Data flow | Boot uses one merged snapshot; reload uses the same live closure; successful authentication carries result-scoped authorization; no second zefs read |
| Guard | REST remains loopback-only; authenticated TLS gRPC proves remote classification; no-auth writes are denied before reserved-identity authorization |
| Simplicity | No new credential source, config option, environment variable, user snapshot, or compatibility path |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| API and no-BGP AAA stop reading SSH users | LSP references plus source review of `main.go` and `mgmt_auth_reload.go` |
| API-specific second zefs path removed | LSP references for `apiZefsUsersOK`; no API-block `loadZefsUsers` call |
| Hub API construction and boot behavior | `make ze-unit-pkg-test PKG=./cmd/ze/hub` |
| AAA profile wrapper and reserved namespace | `make ze-unit-pkg-test PKG=./internal/component/aaa` and `make ze-unit-pkg-test PKG=./internal/component/authz` |
| REST and gRPC authorization identities | `make ze-unit-pkg-test PKG=./internal/component/api/rest` and `make ze-unit-pkg-test PKG=./internal/component/api/grpc` |
| Running REST, AAA, zefs/config precedence, and reload | `make ze-functional-ui-test` |
| Token-only output preservation | `make ze-functional-plugin-test` |
| API documentation | `make ze-doc-verify` and `make ze-doc-wiring-check` |
| Go quality | `make ze-lint-changed` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Correct and wrong credentials | Same running daemon must accept one and reject the other |
| Failed boot source | Startup aborts before AAA or listeners; token presence does not convert the error into a valid mode |
| Per-user profiles | Successful authentication binds remote and recovery profiles before strict authorization; ordinary local assignments remain live |
| Shared API identity | Only REST/gRPC token or no-auth classification injects it; configured/resolved usernames cannot spoof the reserved namespace |
| Token-only config | A successful empty user read preserves the explicit token mode and write authority with unrelated profiles installed |
| No-auth config | Read-only commands pass strict AAA; every mutation remains denied before authorization |
| Privilege | Config users never receive the zefs recovery profile; config collision removes the zefs recovery grant |
| Reload rollback | Failed user-source resolution leaves previous credentials installed |
| Authorization | No-BGP startup installs the live bundle before API or MCP dispatch |
| Secrets | Logs show only mode and count, never username, password, hash, or token |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Boot functional test stays green before fix | Test is vacuous; prove correct and wrong credentials, profile allow/deny, zefs recovery, and collision precedence |
| Live closure sees old reload tree | Fix producer ordering or test setup; do not restore a snapshot merge |
| User-source error becomes NONE silently | Fix error handling at the call site |
| Zefs user authenticates but authorization denies | Fix the shared result-authorizing wrapper path; do not add a recovery special case in the transport |
| Token or no-auth fails when profiles exist | Fix reserved identity injection or strict-store handling; do not bypass authorization for ordinary usernames |
| Per-user request accepts the shared token | Preserve authenticator precedence in both transports |
| SSH spec needs a default-composition API file | Recheck the boundary; only its spawned no-SSH composition proof can consume this behavior |
| Existing user-removal test fails | Preserve `UsersLive` on rebuilt authenticators |

## Design Insights

- The correct source already exists. The defect is the second list used only to
  decide whether to construct the API authenticator.
- Reload already refreshes the provider before authentication intents run. A
  reloader that reads the live closure gets the candidate users without parsing
  the tree a second way.
- Re-reading zefs during reload conflicts with the established design: zefs
  users are a boot snapshot, while config users are live.
- The same SSH branch that supplied the user snapshot also gated no-BGP AAA.
  Moving the boot snapshot and bundle installation together removes both
  dependencies without a second source.
- API authentication previously had two hidden outputs: a username and resolved
  profiles. The direct local API path kept only the username. Reusing the AAA
  wrapper preserves both without a transport-specific profile store.
- A shared token is not a configured user. A server-injected reserved identity
  preserves its existing authority under strict AAA without making an
  ordinary unassigned username permissive.
- No-auth and shared-token calls can use the same reserved identity because the
  existing `ReadOnly` gate distinguishes their authority before dispatch.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|-------------------------|-----------|
| Resolve one boot snapshot from `localUsers` | Call separate resolvers for AAA, API, and SSH | One read gives every boot consumer the same users and one error policy |
| Abort on boot source error | Continue token-only or anonymous | A failed read is not a valid empty user list |
| Build one no-BGP AAA bundle and reuse it on later infrastructure setup | Keep AAA inside standalone SSH; rebuild on BGP infrastructure reentry | API/MCP authorization must not depend on SSH, and one boot ownership claim prevents rejected infrastructure setup from replacing accepted AAA |
| Use `apiUsersLive` on reload | Re-read zefs and config tree | Provider is already current; one source preserves rollback and deletion behavior |
| Prove REST loopback and gRPC/TLS remote paths separately | Bind REST non-loopback | REST rejects non-loopback by design; gRPC is the valid remote transport |
| Keep the generic guard unchanged | Add an API exception or opt-in leaf | Real per-user auth makes the listener authenticated; no service-specific branch is needed |
| Update all authentication and API guide claims in this closure | Leave shared authentication wording to SSH optionality | The accepted-generation semantics are API-owned; SSH retains only optional transport and build documentation |
| Repurpose obsolete zefs-reread tests and strengthen existing transport tests | Preserve the second database read or add duplicate workflows | Owner approved the shared-source contract and test updates |
| Reuse `aaa.WithProfileAuthorizer` for API local authentication | Bind profiles in the REST/gRPC callback; use the full AAA backend chain | One wrapper keeps reserved-name rejection and result-scoped authorization at the authentication choke point while preserving local-only API credentials |
| Inject `aaa.ReservedSharedAPIUsername` for token/no-auth callers | Exempt printable username `api`; add an API-engine bypass | An untypeable server identity cannot collide with a configured user; the existing read-only gate keeps no-auth mutation denied |
| Keep per-user authenticator precedence over token | Make token an alternative when per-user credentials fail | Existing REST and gRPC producers already choose per-user mode first; fallback would weaken authentication |

## Known Limitations
- gNMI token resolution is separate and unchanged.
- Looking Glass remains an intentionally public surface outside this guard.
- SSH schema, extractor ownership, and `SSHExtractedConfig.Users` removal are handled by the dependent SSH spec.
- A loopback API with no users and no token retains its current no-auth read-only mode through the reserved shared-API identity.

## Checklist

### Goal Gates
- [x] AC-1 through AC-16 demonstrated
- [x] Every user story has a passing test
- [x] Wiring table complete
- [x] Integration and documentation checklists answered
- [x] Architectural and critical review complete
- [x] Every assumption validated during implementation
- [x] Phase 5 atomic accepted-generation and reload tests passed
- [x] Phase 6 live authorization, accounting, and completion tests passed
- [x] Deferral row marked done only at closure

### TDD
- [x] Tests written first
- [x] Tests failed for their intended missing behavior
- [x] Tests pass after implementation
- [x] REST and gRPC credential tests discriminate
- [x] No-BGP AAA allow/deny path discriminates
- [x] Reload user change and removal remain green
- [x] Interop N-A: no wire change

### Closure
- [x] Append and complete `plan/TEMPLATE-CLOSURE.md`
- [x] Independent review gate clean and recorded
- [x] Learned outcome routed to API and AAA architecture documentation
- [x] Commit A contains code, tests, docs, journals, and spec
- [x] `make ze-precommit-verify`
- [x] Commit B removes the spec only after closure

---

## Implementation Summary

### What Was Implemented
- One accepted local identity generation now combines config and zefs credentials, local RBAC, the effective API token, the live external AAA bundle, and a generation-bound request authorizer.
- `runYANGConfig` resolves users before management listeners, installs one no-BGP AAA bundle independently of SSH, and lets later infrastructure setup reuse that boot-owned bundle.
- `runReloadContext` stages a fail-closed API generation, resolves absent API blocks against candidate users plus the retained token, migrates listeners, and publishes only after every fallible reload step succeeds.
- REST and gRPC resolve request authentication through the live accepted provider. Per-user results carry immutable policy, token/no-auth callers use the reserved API identity, and recovery grants expire when the accepted local generation changes.
- Command authorization and accounting resolve through the live accepted bundle. The STOP record uses the accountant that emitted START, and `RenderedResponse.TransportComplete` delays STOP until the direct or socket-backed transport has delivered the response.

### Bugs Found/Fixed
- Absent API blocks could rebuild identity without classifying an environment-started remote gRPC listener. `TestAPIAuthReloaderAbsentBlockUsesCandidateUsersAndRetainedToken` and `TestAPIAuthReloaderAbsentBlockWithoutCredentialsIsKnownUnauthenticated` cover the fix.
- A later listener move failure could discard undo for an earlier successful move. `TestReloadListenersReturnsUndoAfterPartialApplyFailure` and the two fail-closed reload rollback tests cover retained retryable undo.
- BGP infrastructure reentry could rebuild and replace the no-BGP boot AAA bundle. `TestInfraSetupReentryReusesNoBGPBootBundle` covers single ownership.
- An established request/session could retain a stale authorizer after accepted-generation publication. `TestAPIRequestCarriesAuthenticatedAuthorizationGeneration`, `TestLiveLocalAuthorizerFollowsIdentityPublication`, and the recovery generation tests cover live invalidation.
- Accounting START and STOP could resolve through different bundles after a swap. `TestInstallNoBGPAAADispatchPairsAccountingAcrossSwap` and `TestLiveAAABundleAccountantConcurrentSwapKeepsPairs` cover producer retention.
- Command accounting completed before the accepted response reached its transport. Dispatcher, plugin-server, REST, gRPC, web, MCP, LG, CLI, and SSH completion-carrier tests now cover delivery ordering.

### Documentation Updates
- `docs/guide/api.md`, `docs/guide/authentication.md`, and `docs/guide/operator-access-rbac.md` describe config/zefs credentials, reserved token/no-auth identity, profile authority, reload invalidation, transport constraints, and exact auth-mode output.
- `docs/architecture/api/architecture.md` and `docs/architecture/aaa-tacacs.md` describe the accepted-generation transaction, live bundle indirection, result-scoped authorization, and accounting lifetime.
- `ai/digests/aaa-auth.md` records the source-level AAA and API flow.
- `make ze-doc-wiring-check` reached a clean wiring verdict. A HEAD-plus-API-doc scratch snapshot generated a fresh `ai/CODE-TO-DOCS.md` with 2,133 code paths and 508 packages; the normal shared-tree check remains red only because foreign documentation anchors and one baseline `staticcheck_feature_matrix.go` citation are outside this commit.

### Deviations from Plan
- The planned credential-source cutover exposed a wider atomicity boundary. Local policy, API token, external AAA selection, listener exposure, accounting, and response delivery had to move with the accepted credential generation rather than remain independent mutable snapshots.
- The always-built authentication extractor and base authz YANG moved in this spec because API boot without an SSH block consumes them. The dependent SSH spec retains only its public-key augment and optional transport composition.
- Shared CLI, web, MCP, LG, plugin RPC, and SSH caller files changed only to carry the response-completion contract required by accepted-action accounting. Pure SSH compile seams remain outside this closure.
- The source deferral shard remains because its remote-backend reload row is still live. Only the API-authentication row was marked done.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Reload identity and listener exposure were initially treated as separate updates | An absent API block can leave an environment-started listener running, so candidate credentials and exposure classification are one decision | adversarial review of absent-block non-loopback gRPC | candidate users and retained token now drive both classification and final publication |
| approach | Listener migration returned no useful undo when a later move failed | Every applied move remains live until restoration succeeds, so partial failure must retain retryable undo | rollback review and failure injection | `migrateListeners` returns undo for the applied set and `runReloadContext` restores identity only after listener restoration |
| assumption | BGP infrastructure setup was assumed to own a separate AAA construction | No-BGP startup can precede later BGP setup, and rebuilding there replaces accepted AAA | lifecycle review of hook reentry | one boot claim owns AAA construction and reentry reuses it |
| approach | Sessions captured an authorizer without an accepted-generation validity check | Recovery authority must disappear on the next decision after credential generation changes | concurrent-session and recovery security review | request authorizers bind immutable profiles plus the accepted local generation and resolve the live external bundle |
| approach | START and STOP each looked up the current accountant | A bundle swap between them sends STOP to a producer that never emitted START | accounting lifecycle review | task state retains the START accountant through STOP |
| approach | Dispatch return was treated as action completion | The accepted action is not observable until its response reaches the direct or socket-backed transport | daemon shutdown response-ordering review | `RenderedResponse` carries explicit transport completion through every caller |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Resolve users independently of SSH | Done | `cmd/ze/hub/main.go` `runYANGConfig`; `internal/component/config/infra/authz.go` `ExtractAuthUsers` | one boot and live source |
| Fail boot on source error | Done | `cmd/ze/hub/main.go` `runYANGConfig` | before AAA and listeners |
| Install no-BGP AAA once | Done | `cmd/ze/hub/aaa_lifecycle.go` `claimAAABundleBoot`; `cmd/ze/hub/infra_setup.go` `infraSetup` | reentry reuses boot ownership |
| Bind authentication to authorization | Done | `internal/component/aaa/login_profiles.go` `WithProfileAuthorizer`; `cmd/ze/hub/aaa_lifecycle.go` `acceptedLocalGenerationAuthorizer` | immutable profiles and generation |
| Preserve token/no-auth compatibility | Done | REST `withAuth`; gRPC `checkAuth`; `aaa.ReservedSharedAPIUsername` | read-only gate remains first |
| Reload as one accepted generation | Done | `cmd/ze/hub/main_reload.go` `runReloadContext`; `cmd/ze/hub/listener_migrate.go` `migrateListeners` | stage, migrate, publish |
| Keep accounting producer pairs | Done | `cmd/ze/hub/aaa_lifecycle.go` `liveAAABundleAccountant`; plugin dispatcher completion carrier | START/STOP and response lifetime |
| Document and prove running behavior | Done | API/AAA docs; UI and plugin functional scenarios | no wire change |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/ui/api-user-removed-by-reload.ci` | no-BGP, no SSH block, per-user REST and AAA |
| AC-2 | Done | same UI workflow; `TestBuildAPIEngineTranslatesDispatcherAuthorizationDenial` | good/bad password and profile denial |
| AC-3 | Done | `TestGRPCBuildAuthenticatesConfigUserWithoutSSH` | real TLS RPC |
| AC-4 | Done | same gRPC test; `mgmt-guard-api-env-started-settings-survive.ci` | authenticated remote classification |
| AC-5 | Done | `TestAPIBootUsersFailClosed` | source error remains distinct from empty users |
| AC-6 | Done | UI collision stages; `TestBuildAPIAuthenticationBindsRecoveryProfiles` | zefs/config precedence and recovery |
| AC-7 | Done | REST/gRPC shared-identity tests; plugin functional scenario | token mode and write authority |
| AC-8 | Done | REST/gRPC shared-identity tests | no-auth reads, mutation denial, remote refusal |
| AC-9 | Done | `TestRunReloadPublishesAcceptedIdentityAtomically`; `TestAPIAcceptedAuthenticationFollowsIdentityPublication` | candidate live source |
| AC-10 | Done | `TestRunReloadUserSourceErrorPreservesAcceptedAPICredentials` | old accepted identity retained |
| AC-11 | Done | `TestAPIAuthReloaderAbsentBlockKeepsRunningMode` | listener mode retained |
| AC-12 | Done | UI workflow; `TestRunReloadPublishesAcceptedIdentityAtomically` | changed/removed user without restart |
| AC-13 | Done | source audit for `sshCfg.Users`, `apiZefsUsersOK`, and API-specific zefs reads | no forbidden producer remains |
| AC-14 | Done | API/authentication/RBAC guide updates plus doc gates | exact modes and transport constraints |
| AC-15 | Done | `TestStoreAuthorizerBoundProfilesDoNotCrossSessions`; `TestAPIRequestCarriesAuthenticatedAuthorizationGeneration` | immutable same-name sessions |
| AC-16 | Done | `TestProfileAuthorizerExpiresLocalRecoveryGrant`; `TestLocalRecoveryAuthenticationRacingPublicationExpires` | next decision loses recovery |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Boot source and merged users | Done | `main_servers_test.go`, `auth_e2e_test.go` | source errors and precedence |
| API authentication and denial translation | Done | `api_test.go` | profiles, reserved identity, transport sentinel |
| AAA profile and generation binding | Done | `aaa/login_profiles_test.go`, `authz/authz_test.go` | isolation, copying, expiry |
| REST accepted provider | Done | `api/rest/auth_test.go`, `server_test.go` | atomic modes and request behavior |
| gRPC accepted provider | Done | `api/grpc/auth_reload_test.go`, `server_test.go` | real RPC and rejected candidate |
| No-BGP AAA ownership | Done | `infra_setup_auth_test.go`, `aaa_lifecycle_test.go` | one boot bundle and live indirection |
| Reload publication and rollback | Done | `main_reload_auth_test.go`, `listener_migrate_test.go` | stage/publish and retryable undo |
| External AAA authorization/accounting | Done | AAA, RADIUS, TACACS tests | live fallback and producer pairing |
| Dispatch completion | Done | plugin dispatch/server tests | direct and socket delivery |
| Surface completion callers | Done | CLI, web, MCP, LG, SSH focused tests | every carrier completed |
| Running REST/reload | Done | `test/ui/api-user-removed-by-reload.ci` | 169/169 UI suite |
| Environment-started API guard | Done | `test/plugin/mgmt-guard-api-env-started-settings-survive.ci` | 605/605 plugin suite |
| Interop | Done | N-A | no HTTP, gRPC, SSH, or plugin wire change |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Hub API/AAA/reload producers | Changed | expanded to the complete accepted-generation transaction |
| AAA/authz/external backends | Changed | result-scoped profiles, live policy, accounting pairing |
| REST and gRPC | Changed | accepted authentication provider and reserved identity |
| Shared dispatch and callers | Changed | explicit transport completion |
| Config extraction/schema | Changed | always-built API dependency; SSH augment excluded |
| Functional tests and weakening ledger | Changed | running behavior and reviewed replacements |
| Docs, deferral, journals, spec | Changed | durable accepted-generation record |

### Audit Summary
- **Total items:** 44
- **Done:** 37
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 7, all recorded in Deviations from Plan

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| API per-user authentication works without an SSH block | functional | `api-user-removed-by-reload.ci` boots no-BGP/no-SSH-block REST and exercises correct/wrong credentials plus profile allow/deny |
| Boot and reload fail closed around one accepted generation | unit and functional | `TestAPIBootUsersFailClosed`, `TestRunReloadFailurePreservesAcceptedIdentity`, `TestRunReloadListenerRollbackFailureStaysFailClosed`, and the running UI reload stages |
| REST and gRPC preserve secure mode precedence | real transport tests | REST request tests and `TestGRPCBuildAuthenticatesConfigUserWithoutSSH` prove per-user precedence, token/no-auth compatibility, wrong-password denial, and TLS remote classification |
| Established sessions keep immutable authority and recovery expires | concurrency/security | `TestStoreAuthorizerBoundProfilesDoNotCrossSessions`, `TestAPIRequestCarriesAuthenticatedAuthorizationGeneration`, and the recovery generation race tests |
| Authorization and accounting follow the live accepted bundle through response delivery | lifecycle | `TestLiveLocalAuthorizerFollowsIdentityPublication`, `TestInstallNoBGPAAADispatchPairsAccountingAcrossSwap`, `TestLiveAAABundleAccountantConcurrentSwapKeepsPairs`, and completion-carrier surface tests |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| Remote AAA backend rebuild on config reload | deferred | remains live in this shard; remote backend construction is outside the API credential-source closure |
| Web boot serve-or-not users independent of SSH | done | previously completed by `startWebServer` using `localUsersLive` |
| API per-user gate independent of SSH block | done | this spec; accepted boot/reload identity and running UI/gRPC evidence |
| SSH public-key users follow reload | done | previously completed under `spec-ssh-fido2-keys` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/hub-deferred-api-auth-independent-of-ssh-block-95ead384-f7b2-4a4a-9286-268f9021bd63.md` |
| `review_gate.py check` | PASS: fresh clean artifact covers all 129 Commit A paths |
| Rounds | 7. Six product defects were fixed, followed by a final clean review |
| Reviewer lenses used | logic and wiring; reload rollback; authentication and authorization security; lifecycle and concurrency; accounting and transport completion |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Absent API block could leave an environment-started non-loopback gRPC listener unauthenticated | `apiAuthReloader`, `runReloadContext` | classify candidate users plus retained token before listener migration |
| 2 | ISSUE | Partial listener migration lost undo for already-applied moves | `listenerMigrator.migrateListeners` | return retryable undo for the applied set and restore identity only after listener restoration |
| 3 | ISSUE | BGP infrastructure reentry rebuilt the no-BGP boot AAA bundle | `claimAAABundleBoot`, `infraSetup` | one boot construction claim and reuse |
| 4 | ISSUE | Established session authorizers could outlive accepted local policy/credential generation | `acceptedLocalGenerationAuthorizer`, request/session propagation | generation check plus live accepted bundle resolution |
| 5 | ISSUE | Accounting STOP could resolve through a different bundle from START | `liveAAABundleAccountant` | retain the START accountant with task state |
| 6 | ISSUE | Accounting ended before response delivery, so shutdown could lose an accepted action's answer | `Dispatcher.BeginAccounting`, direct/socket callers | explicit `RenderedResponse.TransportComplete` at the transport boundary |
| 7 | CLEAN | Final full fix review found 0 BLOCKER and 0 ISSUE | complete API-owned file set | no further code changes |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/hub/infra_setup_auth_test.go` | yes | file read and included in the review/commit set |
| `test/ui/api-user-removed-by-reload.ci` | yes | file read and exercised by the passing UI suite |
| `test/plugin/mgmt-guard-api-env-started-settings-survive.ci` | yes | file read and exercised by the passing plugin suite |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 through AC-2, AC-6, AC-12 | no-SSH-block REST boot, authz, precedence, and reload | `make ze-functional-ui-test`: 169/169 passed, 10 expected skips |
| AC-3 through AC-5 | gRPC auth/TLS and boot failure | `make ze-unit-pkg-test PKG=./cmd/ze/hub RACE=0` and `RACE=on`: passed |
| AC-7 through AC-8 | token/no-auth compatibility and guard | plugin suite 605/605; REST/gRPC package tests passed |
| AC-9 through AC-13 | candidate source, rollback, removal, source audit | hub race/non-race tests passed; forbidden-symbol grep clean |
| AC-14 | Done | `make ze-doc-wiring-check` reached a clean wiring verdict; commit-scoped `ai/CODE-TO-DOCS.md` generated from HEAD plus API docs is fresh with 2,133 paths |
| AC-15 through AC-16 | session isolation and recovery invalidation | combined AAA/TACACS/API package tests passed |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| no-BGP/no-SSH-block API boot, REST login, policy, listener move, user/profile reload | `test/ui/api-user-removed-by-reload.ci` | passing in 169/169 UI suite |
| environment-started API listener, token/no-auth mode, absent-block reload | `test/plugin/mgmt-guard-api-env-started-settings-survive.ci` | passing in 605/605 plugin suite |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `runYANGConfig` constructs and invokes `localUsers` before AAA and API listener construction; boot tests pass |
| A-2 | confirmed | `runReloadContext` applies candidate provider roots before identity resolution and publishes only at final success |
| A-3 | confirmed | `buildUserAuthenticator`, REST, and real gRPC tests distinguish correct and wrong credentials |
| A-4 | confirmed | `TestLiveLocalUsersKeepsPowerUsersWithNoSystemRoot` and boot no-user modes pass |
| A-5 | confirmed | `NewRESTServer` rejects non-loopback; gRPC/TLS supplies the remote proof |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| API credential sources, precedence, and exact modes | `runYANGConfig`, `buildUserAuthenticator`, REST `withAuth`, gRPC `checkAuth` | `docs/guide/api.md` source anchors updated |
| Accepted-generation reload and rollback | `runReloadContext`, `migrateListeners`, accepted identity provider | `docs/architecture/api/architecture.md` updated |
| AAA authorization and accounting lifetime | `WithProfileAuthorizer`, `acceptedLocalGenerationAuthorizer`, `liveAAABundleAccountant` | AAA/TACACS architecture and digest updated |
| Shared authentication and RBAC behavior | `ExtractAuthUsers`, `Store.AuthorizeWithProfiles`, reserved identities | authentication and operator RBAC guides updated |
| Config/CLI/plugin/wire/RFC categories not otherwise updated | no syntax, command, payload, registered inventory, runtime dependency, or RFC behavior changed | N-A after source-aware review |

## Core Insight

Authentication is not one credential callback. The durable security boundary is the accepted generation that decides credentials, local and external authority, listener exposure, accounting producer, and when an action becomes observable. Publishing or ending any one of those separately recreates the same stale-snapshot defect at a different layer.
