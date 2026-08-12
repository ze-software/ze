# Spec: fixit-web-auth-deleted-user-survives-reload

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | implementation complete, review round 4 findings fixed |
| Deferral shard | `plan/deferrals/fixit-web-auth-deleted-user-survives-reload.md` |
| Updated | 2026-08-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A web user deleted from the configuration keeps logging in until the daemon
restarts, and the reload that deleted them reports success. Thomas ordered the
fix ahead of the queue on 2026-08-07.

The goal: an operator who removes a user and reloads has removed that user. The
two windows the old fallback existed for must keep working, because both are
real: the web server starts before the AAA chain exists, and a config-file web
user can legitimately be absent from that chain's local backend.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - the guard rules this defect breaks
  → Constraint: a guard fails closed or says something; a permissive branch taken silently is the bug.
  → Decision: enumerated data is DERIVED from one producer, never copied to a second reader.
- [ ] `docs/architecture/hub-architecture.md` - daemon startup and the infra hook
  → Constraint: the hub builds AAA from a parsed tree and hands it to the engine through `infra.Hook`; the engine calls back once its reactor exists.

**Key insights:**
- The shared `*zeconfig.Provider` is the one in-process view of the config the
  daemon is running: every applied reload refreshes it root by root and every
  rolled-back reload restores it.
- The AAA bundle is NOT refreshed by a reload. That is a second, separate gap.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/service_web.go` - `startWebServer` computed the web user list once and pinned it into the fallback authenticator.
- [ ] `cmd/ze/hub/aaa_authenticator_web.go` - `liveAAABundleAuthenticator.Authenticate` consults the fallback exactly when the live chain REJECTS, and returns success if the fallback authenticates.
- [ ] `internal/component/authz/auth.go` - `LocalAuthenticator` held `Users []UserConfig` by value, so the slice it was built with was the slice it answered from forever.
- [ ] `internal/component/authz/register.go` - `localBackend.Build` bound that snapshot into the AAA chain, which is consulted BEFORE any fallback.
- [ ] `cmd/ze/hub/infra_setup.go` - `buildAAABundle` passes `LocalUsers` once, at reactor creation.
- [ ] `internal/component/config/infra/ssh.go` - `ExtractSSHConfig` walked the tree for `system/authentication/user`.
- [ ] `internal/component/config/provider.go` - `Provider.Get(root)` returns a shallow copy of the current root, or an empty map when the root is absent.
- [ ] `cmd/ze/hub/main_reload.go` - `doReload` calls `applyLoadedTreeToProvider(cp, newTree)` after `s.ReloadConfig` succeeds, and `restoreProviderSnapshot` on rollback.
- [ ] `internal/component/plugin/server/reload.go` - `(*Server).ReloadConfig` diffs the tree and drives verify/apply RPCs.
- [ ] `internal/component/bgp/plugin/register.go` - `runBGPEngine` builds the reactor from `p.OnConfigure`, the plugin protocol's stage-2 startup callback.
- [ ] `internal/component/bgp/config/loader_create.go` - `CreateReactorFromTree` is the only non-test caller of `infra.Run`.
- [ ] `cmd/ze/hub/infra_setup.go` - `infraSetup` is the infra hook and the only reload-reachable caller of `swapAAABundle`.

**Behavior to preserve:**
- The pre-bundle window: before the AAA chain exists, web login still works.
- The unknown-to-the-chain window: a config-file user the chain's local backend
  does not carry still authenticates.
- The zefs power user authenticates whatever the config says, including a config
  that declares no users at all.
- A config user overrides a same-named zefs power user (`mergeAuthUsers`).
- `ExtractSSHConfig` reports the same users it always did.

**Behavior to change:**
- The AAA chain's local backend answers from the RUNNING config instead of a
  startup snapshot, and so does the web fallback behind it. Both, because the
  chain answers first: fixing only the fallback changes nothing observable.
- A user the running config no longer declares is refused.
- A user the running config newly declares is accepted, with no restart.
- A running config that cannot be read refuses the login and says so.
- Config users reach AUTHENTICATION whether or not the config declares an
  `environment { ssh { } }` block. The per-login reader goes to the `system`
  root directly rather than through `ExtractSSHConfig`, whose early return made
  config users invisible to web and API auth on an ssh-less config. Only the
  boot-time serve-or-not check stays coupled (deferral shard).

## Data Flow (MANDATORY)

### Entry Point
An HTTP request to a protected web route or `POST /login`, carrying a username
and password (Basic auth header or login form).

### Transformation Path
1. `zeweb.AuthMiddlewareWithAudit` / `zeweb.LoginHandlerWithAudit` call `webAuth.Authenticate`.
2. `liveAAABundleAuthenticator` reads the `aaaBundle` atomic slot and tries the live chain.
3. The chain's local backend is an `authz.LocalAuthenticator` with `UsersFunc` set, so it answers from the running config.
4. On rejection (or no bundle), the web fallback runs. It is the same type with the same kind of `UsersFunc`.
5. Both funcs are `liveLocalUsers(...)` → `liveConfigUsers(configProvider)` → `Provider.Root("system")` → `infra.ExtractAuthUsers`, merged with the zefs power users.

A request carrying a `ze-session` cookie takes the shorter path, and reaches the
same producer: `zeweb.AuthMiddlewareWithAudit` → `SessionStore.ValidateToken` →
`SessionStore.localUserDeclared` → the same `liveLocalUsers(...)` func the
fallback authenticator holds. `startWebServer` builds that func once and gives
it to both, so a cookie and a password cannot disagree about who exists.

Config enters the same provider by the other path: `doReload` → `applyLoadedTreeToProvider`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| hub ↔ config | `*zeconfig.Provider.Root("system")`, read per login | Yes |
| hub ↔ config/infra | `infra.ExtractAuthUsers(map[string]any)` | Yes |
| always-on hub ↔ ze_web factory | `ServiceDeps.LocalUsersLive func() ([]authz.UserConfig, error)`, beside `ServiceDeps.PowerUsers []authz.UserConfig` | Yes |

### Integration Points
- `ServiceDeps` (`cmd/ze/hub/service_registry.go`) carries the live reader, so
  no `internal/component/web` type crosses the registry boundary.
- `infra.ExtractSSHConfig` derives its `Users` from the same extractor.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Auth reads the config through the same Provider the reload writes; nothing re-reads the file or the blob store |
| No unintended coupling | Yes | The gated web factory receives a `func`, not a `*Provider` |
| No duplicated functionality | Yes | `ExtractSSHConfig` now derives its users from `ExtractAuthUsers`; the second tree-walk is DELETED, not layered (`ai/rules/no-layering.md`) |
| Zero-copy preserved | N-A | Login is not a hot path |
| Registration over hardcoding | Yes | No new per-feature field in a core package; the seam is one `ServiceDeps` func |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The ConfigProvider is populated before the web server is built | `runYANGConfig` fills it in Phase 2, `buildServices` runs later in the same function | The pre-bundle window would refuse every config user | `test/ui/web-user-removed-by-reload.ci` "before" stage authenticates two config users | confirmed |
| A-2 | Every applied reload refreshes the `system` root | `doReload` calls `applyLoadedTreeToProvider` unconditionally after `ReloadConfig` succeeds | A deleted user would survive the reload anyway | Same functional test, "after" stage | confirmed |
| A-3 | No `ReloadConfig` path rebuilds the AAA bundle | `go list -deps ./internal/component/plugin/server` carries no `bgp/config`; the BGP plugin registers only `OnConfigure` (stage 2 of plugin STARTUP) | The SIGHUP half would need no work | Dependency closure + handler grep | confirmed |
| A-5 | Only one functional fixture depended on config users being invisible without an ssh block | Swept every `.ci` under `test/plugin/`, `test/ui/`, `test/reload/` matching `authentication` and ran all 60 | Other tests would break on landing | The sweep: 59 pass unchanged, `rbac-web-config-deny` corrected | confirmed |
| A-4 | `Tree.ToMap()` and the tree walk describe the same users | `Get` and `ToMap` both read `t.values` directly, and the loader prunes inactive nodes before either runs | Boot and login would disagree about who exists | `TestExtractAuthUsersAgreesWithExtractSSHConfig` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Refusing on an unreadable config locks the operator out | Web login fails for everyone at once | The zefs power user is a separate snapshot the config cannot remove, so the break-glass account is unaffected by a config read failure only when the read succeeds and returns nothing; a hard read failure is a wiring fault, is logged, and cannot arise from operator action |
| R-2 | Reading the provider per login is slow | Login latency | `Provider.Get` is a shallow copy of one root under an RLock; logins are interactive and rare |
| R-3 | AC-10 widened that read from per login to per request carrying a session cookie | Web page latency under a config with many users | Same shallow copy plus one walk of `system/authentication`, on human-paced requests only: `/assets/` is not behind the auth wrapper, and a session a remote backend granted is not anchored and skips the read entirely. Caching it would reintroduce the snapshot this spec exists to delete |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Web login. Too strict locks operators out of the UI; too loose is the defect being fixed |
| How is it reverted? | Single commit revert; no config migration, no on-disk format change |
| Who else touches this path? | `spec-fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild` owns `listener_migrate.go` and `mgmt_auth_reload.go`. This spec touches neither |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| HTTP request with Basic auth, after a SIGHUP reload | → | `liveAAABundleAuthenticator.Authenticate` → `authz.LocalAuthenticator.UsersFunc` → `liveConfigUsers` → `infra.ExtractAuthUsers` | `test/ui/web-user-removed-by-reload.ci` |
| `buildWebService` builds the web server | → | `startWebServer` installs `&authz.LocalAuthenticator{UsersFunc: localUsersLive}` as the fallback (`service_web.go`) | `TestWebAuthRejectsRemovedUserWhenChainInstalled` |
| HTTP request carrying a `ze-session` cookie, after a reload removed its user | → | `AuthMiddlewareWithAudit` cookie branch → the live user check | `test/ui/web-user-removed-by-reload.ci`, cookie stage |
| SSH public key offered after a reload removed its user | → | `Server.Start` public-key handler → live user source | `TestPublicKeyAuthFollowsRunningConfig` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config user is removed and the config reloaded | The user is refused at the next web login, with no restart |
| AC-2 | A config user is kept across the same reload | The user still authenticates |
| AC-3 | A config user is added by a reload | The user authenticates, with no restart |
| AC-4 | No AAA bundle is installed yet (pre-bundle window) | Config users and the zefs power user authenticate |
| AC-5 | An AAA chain is installed and rejects a config user it does not carry | The user still authenticates while the config declares them |
| AC-6 | The running config cannot be read | The login is refused and the failure is logged |
| AC-7 | The config declares no users | The zefs power user still authenticates |
| AC-8 | The same user name is in both zefs and the config | The config entry decides, and follows the running config |
| AC-9 | An AAA chain is installed and its LOCAL backend is asked about a deleted user | The chain itself refuses, without relying on a later fallback |
| AC-10 | A user holds a valid `ze-session` cookie and a reload removes them from the config | The next request carrying that cookie is refused, with no restart and no wait for the 24h TTL |
| AC-11 | A user removed by a reload presents their configured SSH public key | The key is refused, and the refusal is recorded like any other SSH auth failure |
| AC-12 | A user the reload KEEPS holds a cookie, or presents their key | Both keep working: no session churn and no key refusal for a user the running config still declares |
| AC-13 | A user removed by a reload presents `Bearer <user>:<pass>` to the REST or gRPC API | The request is refused, with no restart |

AC-13 added 2026-08-08 from the independent review of AC-10 to AC-12. It is a
THIRD credential surface on the boot snapshot, and no deferral row named it.
`buildUserAuthenticator` (`cmd/ze/hub/api.go`) builds
`&authz.LocalAuthenticator{Users: users}` with no `UsersFunc`, from an
`apiUsers` list assembled once at boot. REST and gRPC accept
`Bearer <user>:<pass>` and dispatch commands through `serverDispatcher`, so the
spec's own defect is live there. The goal, "an operator who removes a user and
reloads has removed that user", does not hold while it stands.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Deletes a web user and reloads, then checks the account is dead | HTTP → AuthMiddleware → liveAAABundleAuthenticator → `authz.LocalAuthenticator{UsersFunc: localUsersLive}` → `liveLocalUsers` → `liveConfigUsers` → `Provider.Root` | `test/ui/web-user-removed-by-reload.ci` |
| 2 | Keeps working in the UI as another user across that reload | Same path, surviving user | Same test, `keepuser` control |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWebAuthRejectsUserRemovedFromRunningConfig` | `cmd/ze/hub/aaa_authenticator_web_test.go` | AC-1 with no bundle | pass |
| `TestWebAuthRejectsRemovedUserWhenChainInstalled` | same | AC-1 and AC-5 on the chain path | pass |
| `TestWebAuthFollowsConfigInBothDirections` | same | AC-2, AC-3 | pass |
| `TestWebAuthDeniesWhenRunningConfigUnreadable` | same | AC-6 | pass |
| `TestWebAuthDeniesWithNoUserSourceWired` | same | AC-6, unwired seam | pass |
| `TestWebAuthPowerUserSurvivesEmptyConfig` | same | AC-7 | pass |
| `TestWebAuthConfigUserOverridesPowerUser` | same | AC-8 | pass |
| `TestNoConfigUsersLeavesPowerUserOnly` | same | AC-4 for web-only mode | pass |
| `TestAAABundleLocalBackendFollowsRunningConfig` | same | AC-9, the chain with no fallback at all | pass |
| `TestLocalAuthenticatorUsersFuncIsReadPerCall` | `internal/component/authz/auth_test.go` | AC-1 at the authenticator | pass |
| `TestLocalAuthenticatorUsersFuncReplacesUsers` | same | a snapshot cannot survive beside the live reader | pass |
| `TestLocalAuthenticatorUsersFuncErrorFailsClosed` | same | AC-6 at the authenticator | pass |
| `TestWebAuthFallsBackWhenNoBundle` | same | AC-4, pre-existing | pass |
| `TestLiveConfigUsersFollowsTheProvider` | `cmd/ze/hub/main_servers_test.go` | AC-1, AC-3 at the provider | pass |
| `TestLiveConfigUsersEmptyWithoutSystemRoot` | same | AC-7 | pass |
| `TestLiveConfigUsersNilProviderIsAnError` | same | AC-6 | pass |
| `TestExtractAuthUsersAgreesWithExtractSSHConfig` | `internal/component/config/infra/ssh_test.go` | A-4, single producer | pass |
| `TestExtractAuthUsersLeafListShapes` | same | profile survives every map shape | pass |
| `TestExtractAuthUsersMissingSections` | same | a shapeless subtree authenticates nobody | pass |
| `TestPublicKeyAuthFollowsRunningConfig` | `internal/component/ssh/pubkey_test.go` | AC-11, AC-12 at the SSH entry point: a real client handshake against a live server | pass |
| `TestPublicKeyAuthDeniesWhenRunningConfigUnreadable` | same | AC-6 on the key path: an unreadable config refuses the key the boot snapshot still holds | pass |
| `TestInfraSetupSSHPublicKeyFollowsRunningConfig` | `cmd/ze/hub/ssh_pubkey_live_test.go` | AC-11, AC-12 through the daemon's own wiring (`infraSetup` -> `sshBuild`) | pass |
| `TestSSHStandaloneBuildPublicKeyFollowsRunningConfig` | same | AC-11, AC-12 on the no-bgp{} path, the second `zessh.NewServer` site | pass |
| `TestPublicKeyMatchUserWithoutProfiles` | `internal/component/ssh/pubkey_test.go` | a key match is a match, not the non-emptiness of the profile list | pass |
| `TestSessionCookieFollowsTheRunningConfig` | `internal/component/web/auth_session_revocation_test.go` | AC-10 and AC-12 at the web entry point: the real login handler issues the cookie and the real middleware refuses it after the removal | pass |
| `TestSessionOfRemoteBackendUserSurvivesLocalRemoval` | same | AC-12 for a RADIUS/TACACS+ operator the local list never declared | pass |
| `TestSessionRefusedWhenLiveUserListUnreadable` | same | AC-6 on the cookie path: an unreadable list does not renew a session it granted | pass |
| `TestSessionAnchoredWhenAReloadLandsDuringLogin` | same | the anchor is the authenticator's answer: a reload landing between `Authenticate` and `CreateSession` cannot make a local session un-revocable | pass |
| `TestSessionOfRemoteBackendUserSurvivesWhenTheLocalListDeclaresThemToo` | same | AC-12 for a name BOTH backends know: membership is not the grant | pass |
| `TestSessionStoreWithoutLiveSourceRefusesALocalSession` | same | the documented meaning of `NewSessionStore(nil)`: it refuses what it cannot check | pass |
| `TestValidateTokenRefusalLeavesANewerSessionAlone` | same | invalidation is token-scoped: a revoked cookie arriving after a re-login does not kill the new session | pass |
| `TestInvalidateSessionIsTokenScoped` | `internal/component/web/auth_test.go` | the same rule at the store, without the concurrency | pass |
| `TestValidateTokenReadsTheListPerRequest` | `internal/component/web/auth_session_revocation_test.go` | AC-10's "no restart and no wait for the TTL": the list is read per request, never cached | pass |
| `TestLocalAuthenticatorAlwaysNamesItsSource` | `internal/component/authz/auth_test.go` | every return path of the local backend reports `SourceLocal`, which is what the session anchor depends on | pass |
| `TestLocalBackendNameMatchesTheSourceItReports` | same | the registry name and the reported source are one string | pass |
| `TestGrantedByLocalBackend` | `internal/component/aaa/chain_test.go` | an empty `Source` is NOT local: silence is not a claim | pass |
| `TestProviderRootReportsAbsence` | `internal/component/config/provider_test.go` | an absent root is distinguishable from a root that holds nothing | pass |
| `TestProviderGetAnswersFromRoot` | same | `Get` keeps its contract while deriving from `Root` | pass |
| `TestLiveConfigUsersReportsAnAbsentSystemRoot` | `cmd/ze/hub/main_servers_test.go` | the reachable fault is reported, not silent | pass |
| `TestLiveLocalUsersKeepsPowerUsersWithNoSystemRoot` | same | AC-7 through the sentinel: no `system` block still authenticates the power user | pass |
| `TestAPIUserAuthenticatorFollowsTheRunningConfig` | same | AC-13: `Bearer <user>:<pass>` over REST and gRPC follows the running config | pass |
| `TestAPIUserAuthenticatorWithoutLiveSourceUsesItsList` | same | the nil branch stays on its caller's list rather than authenticating nobody | pass |
| `TestWebServerUsesTheCallersCredentialsWhenZefsIsUnreadable` | `cmd/ze/hub/service_web_users_test.go` | the web server reads zefs no second time: with the database unreadable it serves on the caller's power user, logs that user in through the real login handler, and the session survives its own next request | pass |
| `TestBootPowerUsersSaysSoWhenZefsIsUnreadable` | same | the boot read of zefs reports its failure at a level the DEFAULT logger prints. It is the only producer of that diagnostic now, so a daemon with no `api-server` block and an unreadable database is otherwise silent about the break-glass account | pass |
| `TestWebServerRefusesToServeWithNoUserSourceWired` | same | `startWebServer` returns a nil server AND says why when `LocalUsersLive` is nil, rather than serving on the `PowerUsers` snapshot beside it | pass |
| `TestWebServerRefusesToServeWhenTheUserSourceCannotBeRead` | same | the same refusal when the source returns an error, carrying that error into the diagnostic | pass |

### Boundary Tests (numeric inputs)
N-A. This change reads names and hashes; it introduces no numeric field.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-user-removed-by-reload` | `test/ui/web-user-removed-by-reload.ci` | Delete a web user, reload, and the account is dead on BOTH credentials -- a password and the session cookie the browser tab is still holding -- while another user keeps working on both | pass |
| `api-user-removed-by-reload` | `test/ui/api-user-removed-by-reload.ci` | AC-13. Delete a user, reload, and `Bearer <user>:<pass>` stops dispatching commands over REST. Covers BOTH construction sites of `buildUserAuthenticator`: a reload with the `api-server` block absent leaves the BOOT-built authenticator in place and asserts against it, then a reload that restores the block on a new port rebuilds it and a later block-absent reload asserts against the REBUILT one | pass |

The `.ci` sits in `test/ui/` beside `web-user-removed-by-reload.ci` rather than in
`test/reload/`. Both are the same defect on two credential surfaces, both need a
request placed strictly AFTER a SIGHUP, and the `.ci` grammar cannot order an
`http=` step after a signal, so both are shell drivers that own the whole
sequence. `test/reload/` holds the runner-driven peer+plugin form, which cannot
express that ordering.

### Interop Tests (Scope: config)
N-A. No wire-visible protocol behavior changes.

## Files to Modify
- `internal/component/authz/auth.go` - `LocalAuthenticator.UsersFunc`: the credentials valid right now, read per call, replacing the snapshot
- `internal/component/authz/register.go` - the local AAA backend prefers `LocalUsersFunc`
- `internal/component/aaa/types.go` - `BuildParams.LocalUsersFunc`
- `internal/component/config/infra/ssh.go` - add `ExtractAuthUsers`; `ExtractSSHConfig` derives its users from it
- `cmd/ze/hub/main_servers.go` - add `liveConfigUsers`, `liveLocalUsers`, `errNoLiveConfigProvider`
- `cmd/ze/hub/infra_setup.go` - `buildAAABundle` and `infraSetup` carry the live user source
- `cmd/ze/hub/aaa_authenticator_web.go` - add `noConfigUsers`
- `cmd/ze/hub/service_web.go` - thread the live reader into `startWebServer` and install the live fallback
- `cmd/ze/hub/service_registry.go` - add the live seam (named `ServiceDeps.ConfigUsersLive` at this point; renamed to `LocalUsersLive` by the one-zefs-reader change below)
- `cmd/ze/hub/main.go` - build the live source once and give it to both the chain and the web factory
- `internal/component/web/auth.go` - the cookie branch of `AuthMiddlewareWithAudit` stops being a bypass of the authenticator (AC-10)
- `internal/component/ssh/ssh.go` - `Config.UsersFunc` and `(*Server).users()`: the same shape and the same precedence as `authz.LocalAuthenticator`, so the key path and the password fallback answer from one source (AC-11)
- `internal/component/ssh/pubkey.go` - `(*Server).authenticatePublicKey` reads that source, denies and audits on a read failure; the wish callback now only calls it
- `cmd/ze/hub/ssh_infra.go` - `UsersFunc` on `sshBuildInputs` and `sshStandaloneInputs`
- `cmd/ze/hub/service_ssh.go` - both `zessh.NewServer` sites set `UsersFunc`
- `cmd/ze/hub/infra_setup.go` - the bgp{} path threads `liveUsers` into `sshBuild`
- `cmd/ze/hub/main.go` - the no-bgp{} path threads `localUsers` into `sshBuildStandalone`
- `internal/component/aaa/types.go` - `SourceLocal` and `AuthResult.GrantedByLocalBackend()`: the local backend's name beside the field it fills, and the predicate that reads it (an empty `Source` is NOT local)
- `internal/component/authz/auth.go`, `register.go` - both spell `aaa.SourceLocal` instead of the literal
- `internal/component/web/auth.go` - `CreateSession` takes the `AuthResult` and records the anchor from it; `InvalidateUser` is REPLACED by the token-scoped `invalidateSession`
- `internal/component/config/provider.go` - `Root` reports root presence; `Get` derives from it
- `cmd/ze/hub/main_servers.go` - `errNoSystemConfigRoot`; `liveConfigUsers` reads through `Root`; `liveLocalUsers` classifies the sentinel as a configuration, not a fault
- `cmd/ze/hub/api.go`, `api_infra.go`, `main.go` - `apiBuildInputs.UsersLive` reaches `buildUserAuthenticator`, so REST and gRPC follow the running config (AC-13)
- `cmd/ze/hub/mgmt_auth_reload.go` - `mgmtAuthInputs.apiUsersLive`, so a listener-changing reload cannot put the API authenticator back on a snapshot
- `cmd/ze/hub/service_web.go`, `service_registry.go`, `main.go` - ONE reader of zefs for the web surface. `ServiceDeps.PowerUsers` and `ServiceDeps.LocalUsersLive` replace `ConfigUsersLive` and `ConfigUsers`, and `startWebServer` no longer calls `loadZefsUsers` or composes its own `liveLocalUsers`: it takes both from the hub, which already built them for the AAA chain. `runWebOnly` keeps its own read because that process has no chain to share one with. The serve-or-not check asks `LocalUsersLive` once at construction rather than merging a second snapshot, which also closes the `ConfigUsers`/ssh-block deferral row, and it fails closed on a nil or erroring source
- `docs/architecture/web-components.md` - the session is re-checked against the running configuration on every request, not only against its 24h TTL; and `startWebServer` is handed its credentials rather than reading them

## Files to Create
- `cmd/ze/hub/main_servers_test.go`
- `cmd/ze/hub/ssh_pubkey_live_test.go`
- `cmd/ze/hub/service_web_users_test.go`
- `test/ui/web-user-removed-by-reload.ci`
- `test/ui/api-user-removed-by-reload.ci`

### Fixture Correction
`test/plugin/rbac-web-config-deny.ci` declared a config user `readonly` whose
bcrypt hash matched no known plaintext, and logged in with the ZEFS power user's
password for the same name. That worked only because config users were invisible
to web auth without an ssh block. With authentication reading the running config,
`mergeAuthUsers` gives the config entry precedence, as it is documented to, and
the unknown password stopped working. The fixture now carries the `testpass`
hash already used by `ssh-user-login-yang.ci` and `authz-allow.ci`, verified
against its plaintext. Every assertion is unchanged, and the identity under test
is now the config user carrying `profile [ readweb ]`, which is what the test
says it is testing. All 59 other auth-touching functional tests were re-run and
pass unchanged.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | No new leaf; `system/authentication/user` already exists |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | No new leaf |
| CLI commands/flags | No | No CLI surface changes |
| CLI grammar | N-A | No new command |
| Editor autocomplete | No | No new leaf |
| Functional test for new RPC/API | Yes | `test/ui/web-user-removed-by-reload.ci` |
| Pipe completeness | N-A | No command output |
| Env var registration | No | No new `environment/` leaf |
| Doctor check for runtime dependencies | No | No new path, socket, port, or binary |
| Prometheus counters/metrics | No | No new observable counter; the refusal is logged |
| BGP family surface | N-A | Not a BGP change |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Behavior correction, not a feature |
| 2 | Config syntax changed? | No | Same leaves, same shape |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/web.md` if it states when a user change takes effect -- checked at closure |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior changed? | N-A | No RFC governs this |
| 10 | Test infrastructure changed? | No | Uses the existing `.ci` shell-driver pattern |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/hub-architecture.md` if it describes web auth wiring -- checked at closure |
| 13 | Route metadata keys? | No | - |
| 14 | Prometheus counters? | No | - |
| 15 | Registered plugin/command/capability? | No | - |
| 16 | Changed file referenced by doc source anchors? | Yes | Grep `docs/` for the six modified files -- checked at closure |
| 17 | Existing docs show examples for this area? | Yes | Same check |

## Implementation Steps

1. **Phase: Wiring** -- add the live seam to `ServiceDeps` (`ConfigUsersLive` then, `LocalUsersLive` now), populate it in `main.go`, consume it in `startWebServer`.
   - Verify: `TestWebAuthRejectsRemovedUserWhenChainInstalled` reaches the new authenticator.
2. **Phase: One producer** -- add `infra.ExtractAuthUsers`, delete the duplicate tree-walk from `ExtractSSHConfig`.
   - Verify: `TestExtractAuthUsersAgreesWithExtractSSHConfig`.
3. **Phase: The guard** -- `liveLocalUsers` refuses on a read failure and on an unwired source, and logs both.
   - Verify: `TestWebAuthDeniesWhenRunningConfigUnreadable`, `TestWebAuthDeniesWithNoUserSourceWired`.
4. **Phase: Functional proof** -- the `.ci` driver starts the daemon, logs in, deletes a user, reloads, and logs in again.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a named test that goes red when the behavior is reverted |
| Correctness | The fallback is consulted on chain REJECTION, so the fallback's answer must never be more permissive than the current config |
| Data flow | Auth reads the same Provider the reload writes; no second copy of the user list survives anywhere |
| Naming | `PowerUsers` (the zefs snapshot, which NAMES accounts for the UI) and `LocalUsersLive` (the live list, which decides both serve-or-not and every login) are distinguishable at every call site |
| Rule: `ai/rules/evidence.md` | Every failure path denies AND logs; no zero value reads as a valid answer |
| Rule: `ai/rules/no-layering.md` | The old tree-walk is deleted, not kept beside the new reader |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One producer of the user shape | `grep -rn 'authentication' internal/component/config/infra/ssh.go` shows a single reader |
| The live seam is reachable | `grep -rn 'LocalUsersLive' cmd/ze/hub` shows producer and consumer |
| Unit tests green | `make ze-test-pkg PKG=./cmd/ze/hub`, `make ze-test-pkg PKG=./internal/component/config/infra` |
| Functional test green | `make ze-ui-test` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authorization failing open | The fallback must deny on every path it cannot answer: nil source, read error, empty user list |
| Credential comparison | Unchanged: `authz.LocalAuthenticator` keeps the timing-safe bcrypt path, including the dummy-hash branch for unknown users |
| Lockout | The zefs power user is a separate snapshot no config reload can remove |
| Log leakage | The refusal logs the username and the error, never the password or the hash |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Functional test fails at the "before" stage | The harness, not the fix: check the web server started and the config users reached it |
| Functional test fails at the "after" stage | The fix or the reload: read `daemon.log`, which the driver prints |

## Design Insights

- The distinction the old code could not draw is "the chain does not know this
  user" versus "the config no longer declares this user". A snapshot cannot draw
  it, because both look like absence. Only a reader of the current config can.
- There were TWO snapshots, not one, and the second was in front of the first.
  Fixing the fallback and running the functional test is what found it: the
  probe user that existed only in the reloaded config authenticated, proving the
  reload had landed, while the deleted user still authenticated, proving
  something ahead of the fallback was answering. A unit test could not have
  found this, because it was a wiring fact, not a logic fact.
- A fallback consulted on REJECTION is a privilege path, not a convenience path.
  Anything more permissive than the primary answer overturns it.
- The `.ci` grammar cannot order an `http=` step after a peer-driven
  `action=sighup`: the two live in separate phases. A shell driver as the last
  `cmd=foreground` owns the whole sequence and is the only way to assert
  before-and-after around a signal.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fix the AAA chain's local backend, not only the web fallback | Fix the fallback alone; have the web layer re-check a `Source == "local"` success against the config | The fallback is consulted only when the chain REJECTS. The chain's local backend held the same snapshot and answered FIRST, so fixing the fallback alone left the deleted user logging in: `test/ui/web-user-removed-by-reload.ci` proved it. Re-checking the chain's answer afterwards would patch a stale producer from outside and would leave SSH and the API stale |
| One live authenticator (`LocalAuthenticator.UsersFunc`) | A separate `configUsersAuthenticator` type at the web edge | Two mechanisms for one question is the drift this spec exists to remove. The web-edge type was written first and DELETED when the backend gained the capability (`ai/rules/no-layering.md`) |
| Read the shared ConfigProvider per login | Re-parse the config file per login; refresh a snapshot from a reload hook | Re-parsing costs a full YANG parse per login attempt and disagrees with the tree during a transaction. A reload hook would have to live in `main_reload.go`, which another spec owns, and a watcher-fed snapshot has a window where a login lands before the watcher runs |
| `ExtractSSHConfig` derives from `ExtractAuthUsers` | Keep both walkers and add a test that they agree | Agreement by construction beats agreement by assertion. The map reader is the one the guard uses, so it is the one that must be canonical |
| Keep the zefs power user as a startup snapshot | Re-read zefs per login | zefs is not the config: no reload adds or removes that account, and `admin-disabled` is written only at image assembly (`internal/appliance/cmd_assemble.go`). Re-opening the database per login would add an I/O failure mode with a lockout attached |
| A `func` in `ServiceDeps`, not a `*Provider` | Pass the provider | Keeps `internal/component/config` out of the gated factory's contract and lets tests drive the seam without building a provider |
| REPORT the anchor: `CreateSession` takes the `AuthResult` and records `GrantedByLocalBackend()`; `ValidateToken` re-checks an anchored session per request | (a) re-derive the anchor by reading the live user list inside `CreateSession` (the first implementation, REPLACED 2026-08-08 after review); (b) drive invalidation from the reload, computing the config users that disappeared and calling an invalidation for each | (a) shipped and was wrong in both directions. It asked a SECOND question, of a list that moves, after the authenticator had already answered. A reload landing between `Authenticate` returning and that read yielded `(declared=false, err=nil)` -- silent, since only the error branch logged -- so a locally-authenticated session recorded itself un-revocable for the full 24h TTL: the guard failed open on the one event it exists to catch. In the other direction, a name held by BOTH the local list and a remote backend anchored on membership, so a local deletion logged out an operator the remote backend had admitted. Membership is not the grant. The authenticator already reports the grant on `AuthResult.Source`, every `LocalAuthenticator` return path sets it (`TestLocalAuthenticatorAlwaysNamesItsSource`), and the chain and `liveAAABundleAuthenticator` pass the result through unchanged. (b) needs the previous user list kept somewhere to diff against, which is the startup snapshot this spec deleted, covers only removals that arrive by reload, and cannot fail closed when the diff itself fails |
| The comparison lives behind `aaa.AuthResult.GrantedByLocalBackend()`, beside `aaa.SourceLocal` | The web package comparing `result.Source == "local"` at the call site | The original objection to `Source` was real and is answered rather than ignored: the web package must not spell a backend name the AAA registry owns, and an empty `Source` from a future backend must not read as a valid answer. The constant sits beside the field it fills, `localBackend.Name()` returns it too (`TestLocalBackendNameMatchesTheSourceItReports`), and the predicate treats an empty `Source` as NOT local. Silence is not a claim: reading it as local would attach the local list's revocation to a session another backend granted, which is the exact regression the second decision below exists to prevent |
| Invalidation is scoped to the TOKEN that failed, never to the username | `InvalidateUser(username)`, deleting whatever token the user holds now | `ValidateToken` releases the read lock before it reads the live list, so a request carrying a revoked cookie can reach the delete after the user has been re-added and has logged in again. Deleting by username then destroys the NEWER session on behalf of the older one. The store already drops the previous token in `CreateSession`, so a session that is no longer `s.users[username]` has nothing left to remove. `InvalidateUser` was deleted, not kept beside the new method (`ai/rules/no-layering.md`) |
| `Provider.Root(name) (map[string]any, bool)` reports root presence; `Get` derives from it | Leave `liveConfigUsers` on `Get` and keep the nil-provider claim | `Get` answers a missing root with an empty map and a nil error, so "the daemon lost the `system` root" and "the operator declares no users" were one answer. The nil-provider branch guarded a state production cannot reach while the reachable fault arrived silently. `liveConfigUsers` now reports `errNoSystemConfigRoot` beside the empty list, and `liveLocalUsers` classifies it as a configuration rather than a read failure, so the zefs power user keeps authenticating (AC-7) |
| A session a remote backend granted is never revoked by the local list | Re-check every session against the live user list | A RADIUS or TACACS+ operator never appears in the config, so re-checking every session would log every one of them out on their next request. `TestSessionOfRemoteBackendUserSurvivesLocalRemoval` goes red on that change |
| An unreadable user list does not anchor a session it could not have granted | Anchor anyway and refuse later; refuse to create the session at all | `authz.LocalAuthenticator` denies while its read fails, so a session created during a read fault came from a remote backend. Anchoring it would lock out the only operators who can still log in during a configuration fault (R-1) |

## Known Limitations

- The live AAA bundle is still not rebuilt on reload, so RADIUS and TACACS
  SERVER changes (address, secret, backend added or removed) need a restart.
  The bundle's LOCAL backend is now live, which is what this defect turned on.
  Homed in the deferral shard with its trace.
- ~~SSH public-key authentication still reads the startup `Users` snapshot.~~
  **Withdrawn 2026-08-07.** This was parking, not a limitation: a deleted user's
  key still yielded a shell with config-edit rights, which is the same defect on
  a more privileged surface, so the spec's goal did not hold while it stood
  (`ai/rules/rule-precedence.md`). It is AC-11.
- Session cookies already issued are covered by AC-10, implemented. A cookie is
  credential material the middleware accepted before the reload, and
  `ValidateToken` tested only the 24h TTL, so the removed user kept full rights
  for the rest of it. It now re-checks the session against the same live user
  list the local authenticator answers from, on every request.
- An SSE stream that is ALREADY open survives the removal until the client
  disconnects. `(*EventBroker).ServeHTTP` (`internal/component/web/sse.go`)
  authenticates at connect and then blocks on the client's context for the life
  of the connection, so no later request exists for `AuthMiddlewareWithAudit` to
  refuse. What survives is a read channel (config-change notifications and the
  log stream), not an edit right: every mutation route is a fresh request and is
  refused. Cancelling an in-flight stream needs the broker to learn which
  username each subscriber belongs to, which is a mechanism this spec does not
  build. HOMED 2026-08-08 by Thomas's ruling on B-14, recorded in
  `plan/deferrals/fixit-connection-management-command.md`: removal affects NEW
  connections only, because a reload that cut the editing operator's own session
  would be worse than the window it closes. What is owed is the operator command
  that closes a live connection deliberately, and it needs its own spec.
- The live user list is read on EVERY request carrying an anchored session, not
  only at login: `SessionStore.ValidateToken` → `localUserDeclared` →
  `Provider.Root` → `ExtractAuthUsers` → `mergeAuthUsers`, so `/fragment/*` and
  `/show/` pay it too. One RLock, one shallow root copy, one map walk and two
  slice allocations, plus the sort `ExtractAuthUsers` does over the users and
  over each user's public keys (its callers merge and log the result, and the
  map form has no order). No I/O and no parse. Correct and accepted: the sort is
  over the configured users, web requests are human-paced, and this is not a
  path `ai/rules/performance.md` governs. It is NOT cached, because a cache is the snapshot the per-request
  re-check exists to delete. Recorded on `localUserDeclared`
  (`internal/component/web/auth.go`), where the next reader meets it.
- `resolveAuthIntents`'s `if _, known := m.runningAuth(name); !known` branch
  (`cmd/ze/hub/listener_migrate.go`) reaches no service today:
  `registerMgmtAuthReloaders` registers web, mcp, rest and grpc, and
  `markMgmtAuth` classifies those same four. Its comment claimed the branch
  covered the looking glass, which it never did: LG gets no reloader, so the
  loop never visits it. The branch is kept as the guard for a reloader
  registered against a name the boot guard does not classify, and the comment
  now says that instead. Not a defect: no behavior depends on it.
- ~~`ServiceDeps.ConfigUsers` (the boot serve-or-not check) still comes from
  `ExtractSSHConfig`, so it is empty when the config declares users but no
  `environment { ssh { } }` block.~~ **Closed 2026-08-08.** `ConfigUsers` is
  deleted; the check asks `LocalUsersLive`, the same source the login path uses.
  It fell out of removing the second zefs reader: with one live list there was no
  second list left for this check to read.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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

### Discrimination Evidence

Each row names the change made to the implementation, and the failure the named
tests produced with that change in place.

| Implementation reverted to | Tests that went red | Observed failure |
|----------------------------|--------------------|------------------|
| `LocalAuthenticator.users()` caching its first answer | `TestWebAuthRejectsUserRemovedFromRunningConfig`, `TestWebAuthDeniesWhenRunningConfigUnreadable`, `TestAAABundleLocalBackendFollowsRunningConfig` | "An error is expected but got nil": the deleted user still authenticated, at the fallback AND at the chain |
| An earlier, web-only fallback fix, with the chain left on its snapshot | `web-user-removed-by-reload` | "FAIL: webuser is refused once the reload removes them from the config", while "OK: newuser authenticates, so the reload read the rewritten config" passed in the same run. This is what proved the chain was the authoritative stale copy |
| Swallowing the config read error | `TestWebAuthDeniesWhenRunningConfigUnreadable` | "An error is expected but got nil ... a guard that cannot read the config must deny" |
| Treating an unwired user source as an empty config | `TestWebAuthDeniesWithNoUserSourceWired` | "Expected error with \"no live config provider\" in chain but got nil" |
| `liveConfigUsers` caching its first answer | `TestLiveConfigUsersFollowsTheProvider`, `TestLiveConfigUsersEmptyWithoutSystemRoot` | "expected: [alice] actual: [alice bob]": the removed user survived |
| `ExtractAuthUsers` dropping profiles | `TestExtractAuthUsersAgreesWithExtractSSHConfig`, `TestExtractAuthUsersLeafListShapes` | "expected: [admin] actual: []string(nil)" |
| The whole fix, in a running daemon | `web-user-removed-by-reload` | "FAIL: webuser is refused once the reload removes them from the config" |
| `(*ssh.Server).users()` returning `s.config.Users`, ignoring `UsersFunc` | `TestPublicKeyAuthFollowsRunningConfig`, `TestPublicKeyAuthDeniesWhenRunningConfigUnreadable`, `TestInfraSetupSSHPublicKeyFollowsRunningConfig` | "An error is expected but got nil ... AC-11: a user the reload removed must be refused when presenting their configured key", and the audit query returned `[]`. The daemon log in the same run shows "SSH auth success username=goneuser source=public-key" AFTER the removal |
| `sshBuildImpl` not setting `UsersFunc` (the mechanism intact) | `TestInfraSetupSSHPublicKeyFollowsRunningConfig` only | Same failure, while the whole `internal/component/ssh` suite stayed green. The two tests discriminate different faults: the mechanism and its wiring |
| `sshBuildStandaloneImpl` not setting `UsersFunc` | `TestSSHStandaloneBuildPublicKeyFollowsRunningConfig` only | "AC-11: the no-bgp{} path must refuse a user the reload removed", while the infraSetup test stayed green. This is why both `zessh.NewServer` sites carry a test |
| `authenticatePublicKey` reading the key match off `profiles == nil` | `TestPublicKeyAuthFollowsRunningConfig`, `TestPublicKeyMatchUserWithoutProfiles` | "Received unexpected error ... AC-12: a kept user with no profile leaf authenticates too". A legal profile-less user was refused on every connection |
| `SessionStore.ValidateToken` returning the session straight after the TTL test, as it did before AC-10 | `TestSessionCookieFollowsTheRunningConfig`, `TestSessionRefusedWhenLiveUserListUnreadable`, `TestValidateTokenReadsTheListPerRequest` | "expected: 401 actual: 200 ... a cookie whose user the running config no longer declares must be refused", and "expected: 4 actual: 1": the list was read once, at login |
| The same revert, in a running daemon | `web-user-removed-by-reload`, cookie stage | "FAIL: webuser's session cookie is refused once the reload removes them from the config", while every password assertion in the same run passed, "OK: webuser is refused once the reload removes them from the config" among them. That contrast is why the `.ci` needed the cookie stage: the AC-1 fix left this path untouched and the old test could not see it |
| Re-checking EVERY session against the live list, ignoring `WebSession.LocalAnchored` (the design the trap invites) | `TestSessionOfRemoteBackendUserSurvivesLocalRemoval`, `TestSessionStoreWithoutLiveSourceRefusesALocalSession` | "Expected value not to be nil ... the local list cannot revoke a session it never granted": a RADIUS-authenticated operator was logged out on their next request |
| `CreateSession` re-deriving the anchor with a second read of the live list, as the first implementation did (`LocalAnchored: func() bool { d, _ := s.localUserDeclared(username); return d }()`) | `TestSessionAnchoredWhenAReloadLandsDuringLogin`, `TestSessionOfRemoteBackendUserSurvivesWhenTheLocalListDeclaresThemToo` | "expected: 401 actual: 200 ... a session the local backend granted must stay revocable when the removal lands during login", with the surviving session printed as `LocalAnchored:false`; and, in the other direction, "expected: 200 actual: 401 ... the local list must not revoke a session the remote backend granted, even to a name it also declares". One revert, two opposite failures: the second read was wrong about who granted the session in both directions |
| Invalidation deleting `s.users[username]` instead of the token that failed | `TestValidateTokenRefusalLeavesANewerSessionAlone`, `TestInvalidateSessionIsTokenScoped` | "Expected value not to be nil ... a refused cookie must not invalidate the session created after it", and "... invalidating a superseded session must leave the current one alone" |
| `liveConfigUsers` back on `Provider.Get`, which answers a missing root with an empty map and a nil error | `TestLiveConfigUsersReportsAnAbsentSystemRoot` | "Expected error with \"the running configuration declares no system root\" in chain but got nil ... no system root at all must say so". `TestLiveLocalUsersKeepsPowerUsersWithNoSystemRoot` stayed green through the revert, which is the point: the sentinel adds a distinction without denying AC-7 |
| `buildUserAuthenticator` ignoring its live source, as it did before AC-13 (`&authz.LocalAuthenticator{Users: users}`) | `TestAPIUserAuthenticatorFollowsTheRunningConfig` | "Should be false ... a user the running config no longer declares must lose API access with no restart": the removed operator kept dispatching commands over REST and gRPC |
| `buildAPIShared` (`cmd/ze/hub/api.go`) passing `nil` for the live source, so the BOOT-built authenticator keeps its snapshot | `api-user-removed-by-reload` | "FAIL: bootuser is refused by the BOOT-built authenticator once the reload removes them", under a daemon that printed `sighup reload complete`. The four assertions before it passed, so the credentials were being checked |
| `apiAuthReloader` (`cmd/ze/hub/mgmt_auth_reload.go`) passing `nil` for the live source, so the REBUILT authenticator freezes the user list of the reload that built it | `api-user-removed-by-reload` | "FAIL: reloaduser is refused by the RELOAD-built authenticator once a later reload removes them", ten assertions later than the revert above, "OK: bootuser is refused by the BOOT-built authenticator once the reload removes them" among the passing ones. One test, two reverts, two different stages: that contrast is what proves each site is covered separately |
| `startWebServer` (`cmd/ze/hub/service_web.go`) reading zefs for itself instead of taking the caller's power users | `TestWebServerUsesTheCallersCredentialsWhenZefsIsUnreadable` | "Expected value not to be nil ... the web server must serve on the caller's power user", with the daemon printing "warning: web power-user auth unavailable" and then "warning: web server disabled: no authenticatable users". The web read failing while the caller's succeeded is the break-glass lockout: the chain admits the account and the session check does not declare it |
| `bootPowerUsers` (`cmd/ze/hub/main.go`) logging the failed zefs read at Debug, which is what the daemon path did once the web factory's stderr warning was deleted | `TestBootPowerUsersSaysSoWhenZefsIsUnreadable` | "\"\" does not contain \"zefs power user unavailable\" ... an unreadable zefs database must be reported at a level the default logger prints". `slogutil.Logger` defaults to WARN, so at Debug the buffer is empty: the guard fails closed and says nothing |
| `startWebServer` merging the `PowerUsers` snapshot into the serve-or-not test instead of refusing on a nil or erroring `LocalUsersLive` | `TestWebServerRefusesToServeWithNoUserSourceWired`, `TestWebServerRefusesToServeWhenTheUserSourceCannotBeRead` | "Expected nil, but got: &web.WebServer{... bound:[]string{\"127.0.0.1:64588\"}}", and "\"web UI default: workbench\\nweb server listening on https://127.0.0.1:64588/\\n\" does not contain \"no user source wired\"". One revert, both branches: the server bound a listener on a user list nobody could produce |

---

## Implementation Summary

### What Was Implemented

- `authz.LocalAuthenticator.UsersFunc` (`internal/component/authz/auth.go`):
  the credentials valid right now, read per call by `(*LocalAuthenticator).users()`.
  It REPLACES the `Users` snapshot when set.
- The AAA chain's local backend takes it: `aaa.BuildParams.LocalUsersFunc`
  (`internal/component/aaa/types.go`) reaches `localBackend.Build`
  (`internal/component/authz/register.go`).
- One live producer in the hub: `liveConfigUsers` -> `liveLocalUsers`
  (`cmd/ze/hub/main_servers.go`), built once in `main.go` and given to the AAA
  chain, the web fallback, the session store, the SSH server and the API
  authenticator. It denies and logs on a nil source and on a read failure, and
  classifies an absent `system` root (`errNoSystemConfigRoot`) as a
  configuration so the zefs power user keeps authenticating.
- One producer of the user shape: `infra.ExtractAuthUsers`
  (`internal/component/config/infra/ssh.go`); `ExtractSSHConfig` derives from it.
- `Provider.Root(name) (map, bool)` (`internal/component/config/provider.go`)
  reports root presence; `Get` derives from it.
- Session revocation: `SessionStore.ValidateToken` re-checks an anchored session
  against the live list per request; the anchor is `AuthResult.GrantedByLocalBackend()`
  recorded at `CreateSession`; invalidation is token-scoped (`invalidateSession`).
- SSH: `Config.UsersFunc` and `(*Server).users()` (`internal/component/ssh/ssh.go`),
  read by `authenticatePublicKey` (`internal/component/ssh/pubkey.go`); both
  `zessh.NewServer` sites set it (`cmd/ze/hub/service_ssh.go`).
- API: `buildUserAuthenticator(users, usersLive)` (`cmd/ze/hub/api.go`), fed at
  boot by `apiBuildInputs.UsersLive` and on reload by `mgmtAuthInputs.apiUsersLive`.
- Web: `startWebServer` takes `PowerUsers` and `LocalUsersLive` from the hub,
  reads zefs no second time, and refuses to serve on a nil or erroring source.

### Bugs Found/Fixed

- TWO snapshots, not one: the AAA chain's local backend answered before the web
  fallback, so fixing the fallback alone changed nothing observable. Found by the
  functional test, not by a unit test: it was a wiring fact.
- The first AC-10 implementation re-derived the session anchor with a SECOND read
  of the live list, which failed open in one direction and logged remote-backend
  operators out in the other. Replaced by reporting the anchor from `AuthResult`.
- `InvalidateUser(username)` destroyed a NEWER session on behalf of an older
  revoked cookie. Replaced by token-scoped `invalidateSession`.
- AC-13 was found by the independent review: REST and gRPC were a third
  credential surface on the boot snapshot that no deferral row named.

### Documentation Updates

- `docs/architecture/web-components.md`: the session revocation row and the
  one-live-list paragraph landed with the implementation (lines 202-205, 215).
- `docs/guide/authentication.md`, "Step 3: reload": ADDED at closure. The page
  said only "Existing sessions are not interrupted", which reads as a general
  claim and is now false for a REMOVED user. The new paragraph names every
  credential surface removal now reaches, and says a connection already open
  outlives it. Three source anchors added: `liveLocalUsers`,
  `SessionStore.ValidateToken`, `authenticatePublicKey`, each verified present.
- `make ze-doc-test` NOT run in this closure session (see Pre-Commit Verification).

### Deviations from Plan

- `ServiceDeps.ConfigUsersLive`/`ConfigUsers` were replaced by `PowerUsers` +
  `LocalUsersLive` mid-flight, which closed a deferral row as a side effect.
- AC-10 to AC-13 were added after the original spec was approved, all from
  review findings on more credential surfaces holding the same snapshot.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The web fallback was fixed first, on its own | The AAA chain's local backend answered FIRST and held the same snapshot | `test/ui/web-user-removed-by-reload.ci` failed with the probe user passing | Fixed the authenticator, not the edge |
| approach | `CreateSession` re-derived the local anchor by reading the live list again | The authenticator had already reported the grant on `AuthResult.Source` | Independent review round 4 | Anchor is REPORTED, not re-derived |
| approach | Invalidation was keyed on the username | The token is what failed; the username may hold a newer session | Review round 4 | `invalidateSession` is token-scoped |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| An operator who removes a user and reloads has removed that user | Done | `cmd/ze/hub/main_servers.go` `liveLocalUsers` | Reached from every credential surface below |
| The pre-bundle window keeps working | Done | `cmd/ze/hub/service_web.go:522` fallback | `liveAAABundleAuthenticator.fallback` carries `UsersFunc` |
| A config user unknown to the chain keeps working | Done | same | The fallback is the same live authenticator |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `(*LocalAuthenticator).users()` reads `UsersFunc` per call (`internal/component/authz/auth.go:54`) | `TestWebAuthRejectsUserRemovedFromRunningConfig` |
| AC-2 | Done | same | `TestWebAuthFollowsConfigInBothDirections` |
| AC-3 | Done | same | same test |
| AC-4 | Done | `service_web.go:522` fallback on the no-bundle path | `TestWebAuthFallsBackWhenNoBundle` |
| AC-5 | Done | same | `TestWebAuthRejectsRemovedUserWhenChainInstalled` |
| AC-6 | Done | `liveLocalUsers` err branch returns the error (`main_servers.go:182-187`) | Denies AND logs |
| AC-7 | Done | `errNoSystemConfigRoot` branch sets `current = nil` and merges power users (`main_servers.go:172-181`) | `TestLiveLocalUsersKeepsPowerUsersWithNoSystemRoot` |
| AC-8 | Done | `mergeAuthUsers(zefsUsers, current)` (`main_servers.go:188`) | Config entry wins |
| AC-9 | Done | `localBackend.Build` sets `UsersFunc: params.LocalUsersFunc` (`authz/register.go:65`), fed by `infra_setup.go:46` | `TestAAABundleLocalBackendFollowsRunningConfig` |
| AC-10 | Done | `SessionStore.ValidateToken` -> `localUserDeclared` (`internal/component/web/auth.go:251-283`) | Anchored sessions only |
| AC-11 | Done | `authenticatePublicKey` -> `(*Server).users()` -> `Config.UsersFunc` (`ssh/pubkey.go:35`, `ssh/ssh.go:256`) | Both `NewServer` sites wired |
| AC-12 | Done | `!session.LocalAnchored` early return (`web/auth.go:265`); kept users match | `TestSessionOfRemoteBackendUserSurvivesLocalRemoval` |
| AC-13 | Done | `buildUserAuthenticator` prefers `usersLive` (`cmd/ze/hub/api.go:83`), fed at boot (`main.go:1208`) and on reload (`mgmt_auth_reload.go:178`) | Both construction sites |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| 44 unit tests listed in the TDD plan | Done | `cmd/ze/hub`, `internal/component/{authz,aaa,web,ssh,config,config/infra}` | `make ze-test-pkg PKG=./cmd/ze/hub` green in this session |
| `web-user-removed-by-reload` | Done | `test/ui/web-user-removed-by-reload.ci` | Present; not re-run in this session |
| `api-user-removed-by-reload` | Done | `test/ui/api-user-removed-by-reload.ci` | Present; not re-run in this session |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Every file in "Files to Modify" and "Files to Create" | Done | All present and committed at HEAD before this closure |
| `cmd/ze/hub/mgmt_auth_reload.go` | Changed at closure | The spec-path citation became a bare stem, because commit B removes the spec |
| `docs/guide/authentication.md` | Changed at closure | Documentation review found a claim removal made false |

### Audit Summary

- **Total items:** 13 AC + 3 task requirements
- **Done:** 16
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 files edited at closure (recorded in Deviations and above)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator who removes a user and reloads has removed that user | functional | `test/ui/web-user-removed-by-reload.ci` drives a live daemon: login, delete, SIGHUP, login again, on BOTH a password and the `ze-session` cookie. Discrimination recorded: with the fix reverted it prints "FAIL: webuser is refused once the reload removes them from the config" while the probe user passes in the same run |
| The same holds on the API | functional | `test/ui/api-user-removed-by-reload.ci` covers BOTH `buildUserAuthenticator` sites: reverting `buildAPIShared`'s live source fails the boot-built stage, reverting `apiAuthReloader`'s fails the reload-built stage ten assertions later |
| The same holds on SSH keys | unit, real handshake | `TestPublicKeyAuthFollowsRunningConfig` runs a real client handshake against a live server; `TestInfraSetupSSHPublicKeyFollowsRunningConfig` and `TestSSHStandaloneBuildPublicKeyFollowsRunningConfig` each go red for a different missing wiring |
| The pre-bundle and unknown-to-the-chain windows keep working | unit | `TestWebAuthFallsBackWhenNoBundle`, `TestWebAuthRejectsRemovedUserWhenChainInstalled` |
| No guard fails open | unit | `TestWebAuthDeniesWhenRunningConfigUnreadable`, `TestWebAuthDeniesWithNoUserSourceWired`, `TestSessionRefusedWhenLiveUserListUnreadable`, `TestWebServerRefusesToServeWhenTheUserSourceCannotBeRead` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| Rebuild the live AAA bundle on config reload (remote backends) | deferred | `spec-fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild`, which owns the reload-time rebuild seam |
| Web boot serve-or-not list independent of the ssh block | done 2026-08-08 | `ServiceDeps.ConfigUsers` deleted; `startWebServer` asks `LocalUsersLive` |
| API per-user gate independent of the ssh block | deferred | `spec-hub-deferred-api-auth-independent-of-ssh-block` (present in `plan/`) |
| SSH public-key auth on the startup snapshot | done 2026-08-08 | Implemented here as AC-11 |
| SSE stream open at the moment of removal | homed, not this spec | Thomas's B-14 ruling, `plan/deferrals/fixit-connection-management-command.md`: removal affects NEW connections; the connection-closing command needs its own spec |

The shard is NOT removed: two rows are still live (`deferred`), so it outlives
its source spec (`ai/rules/planning.md`).

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-web-auth-deleted-user-survives-reload-640fa955-f03a-45e8-a58f-4b367f5859e6.md` (16 files) |
| `review_gate.py check` | clean |
| Rounds | 1 at closure. The implementation sessions ran four review rounds before this one, recorded in Findings fixed |
| Reviewer lenses used | AC-to-producer verification at HEAD (every AC traced to its producing function), guard fail-closed audit, citation and documentation audit |

This closure reviewed code ALREADY AT HEAD. The working tree carried no
uncommitted change over any path this spec names, verified with
`git status --porcelain` over `internal/component/{authz,aaa,web,ssh,config}`,
`cmd/ze/hub`, `test/ui` and `docs/architecture/web-components.md` before the
review began. The two edits this closure made are the citation repoint and the
documentation correction listed above.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The web fallback was fixed while the AAA chain's local backend answered first from a snapshot | `internal/component/authz/register.go` | `LocalUsersFunc` on the backend |
| 2 | BLOCKER | SSH public-key auth kept a deleted user's shell | `internal/component/ssh/pubkey.go` | AC-11 |
| 3 | BLOCKER | A session cookie survived removal for the 24h TTL | `internal/component/web/auth.go` | AC-10 |
| 4 | BLOCKER | REST and gRPC `Bearer` auth was a third snapshot | `cmd/ze/hub/api.go` | AC-13 |
| 5 | BLOCKER | The session anchor was re-derived by a second read and failed open | `internal/component/web/auth.go` | Anchor reported from `AuthResult` |
| 6 | ISSUE | Invalidation by username destroyed a newer session | `internal/component/web/auth.go` | Token-scoped `invalidateSession` |
| 7 | ISSUE | `startWebServer` read zefs a second time and could serve on a list nobody else agreed with | `cmd/ze/hub/service_web.go` | One reader; refuse on nil or erroring source |
| 8 | NOTE | The spec claimed the SSE limitation was "not homed yet" | this spec, Known Limitations | Corrected: it is homed by Thomas's B-14 ruling |
| 9 | NOTE | `docs/guide/authentication.md` said only "existing sessions are not interrupted" | `docs/guide/authentication.md` | Removal paragraph added with three source anchors |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/hub/main_servers_test.go` | Yes | `ls -1` printed it |
| `cmd/ze/hub/ssh_pubkey_live_test.go` | Yes | same |
| `cmd/ze/hub/service_web_users_test.go` | Yes | same |
| `test/ui/web-user-removed-by-reload.ci` | Yes | same |
| `test/ui/api-user-removed-by-reload.ci` | Yes | same |
| `internal/component/web/auth_session_revocation_test.go` | Yes | same |
| `internal/component/ssh/pubkey_test.go` | Yes | same |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-3 | The authenticator reads the live list per call | `internal/component/authz/auth.go:54-57`: `func (a *LocalAuthenticator) users()` returns `a.UsersFunc()` when set |
| AC-4, AC-5 | The fallback is the same live authenticator | `cmd/ze/hub/service_web.go:522`: `liveAAABundleAuthenticator{fallback: &authz.LocalAuthenticator{UsersFunc: localUsersLive}}` |
| AC-6 | A read failure denies and logs | `cmd/ze/hub/main_servers.go:182-187`: `case err != nil:` logs `cannot read running config users` then `return nil, err` |
| AC-7 | No `system` root still authenticates the power user | `main_servers.go:172-181` sets `current = nil` and falls through to `mergeAuthUsers` |
| AC-8 | Config entry wins | `main_servers.go:188`: `mergeAuthUsers(zefsUsers, current)` |
| AC-9 | The CHAIN's local backend is live | `internal/component/authz/register.go:65`: `UsersFunc: params.LocalUsersFunc`, fed by `cmd/ze/hub/infra_setup.go:46` |
| AC-10, AC-12 | Anchored sessions re-checked per request, others untouched | `internal/component/web/auth.go:265` `if !session.LocalAnchored { return session }`, then `:269` `localUserDeclared` |
| AC-11 | The key path reads the live source | `internal/component/ssh/pubkey.go:35` `users, err := s.users()`; `ssh/ssh.go:256` prefers `UsersFunc`; `cmd/ze/hub/service_ssh.go:48,220` set it at both `NewServer` sites |
| AC-13 | REST and gRPC follow the running config | `cmd/ze/hub/api.go:83` `auth = &authz.LocalAuthenticator{UsersFunc: usersLive}`; `main.go:1208` `UsersLive: localUsers`; `mgmt_auth_reload.go:178` `buildUserAuthenticator(users, in.apiUsersLive)` |
| All | The package the wiring lives in is green | `make ze-test-pkg PKG=./cmd/ze/hub`: `ok github.com/ze-software/ze/cmd/ze/hub 106.382s` |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| HTTP Basic auth after a SIGHUP reload | `test/ui/web-user-removed-by-reload.ci` | Yes: the file carries 31 lines naming the cookie / `ze-session` path beside the password stages |
| `buildWebService` installs the live fallback | none (unit) | Yes: `service_web.go:75-76` passes `deps.PowerUsers` and `deps.LocalUsersLive`, populated at `main.go:1131,1139` |
| `ze-session` cookie after removal | `test/ui/web-user-removed-by-reload.ci` | Yes, same file |
| SSH public key after removal | none (unit, real handshake) | Yes: `internal/component/ssh/pubkey_test.go` plus `cmd/ze/hub/ssh_pubkey_live_test.go` |
| `Bearer <user>:<pass>` after removal | `test/ui/api-user-removed-by-reload.ci` | Yes: 3 `Bearer` occurrences, one per credential stage |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `main.go:615-617` builds the live source in the same function that later builds the services |
| A-2 | confirmed | `doReload` calls `applyLoadedTreeToProvider` after `ReloadConfig` succeeds |
| A-3 | confirmed | Dependency closure recorded in the deferral shard row 1, and the row stays live for that reason |
| A-4 | confirmed | `TestExtractAuthUsersAgreesWithExtractSSHConfig`; `ExtractSSHConfig` now DERIVES from `ExtractAuthUsers` (`infra/ssh.go:74`), so agreement is by construction |
| A-5 | confirmed | `test/plugin/rbac-web-config-deny.ci` corrected; the other 59 pass unchanged |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/web-components.md` session revocation row | `internal/component/web/auth.go:251-283` `ValidateToken` re-checks anchored sessions | Yes, read at HEAD |
| `docs/guide/authentication.md` "Step 3: reload" | `cmd/ze/hub/main_servers.go` `liveLocalUsers`, `internal/component/web/auth.go` `SessionStore.ValidateToken`, `internal/component/ssh/pubkey.go` `authenticatePublicKey` all present | Yes, each symbol grepped at HEAD before the anchor was written |
| Anchors pointing at changed files | `docs/guide/authentication.md:32` cites `usersFromZefsDB`, still present at `main_servers.go:211` | Yes, no drift |
| Every other category | No new leaf, command, RPC, plugin, wire format, RFC behavior or metric | Yes, the Documentation Update Checklist rows stand |

## Gates Not Run in This Closure Session

`make ze-verify`, `ze-verify-changed`, `ze-doc-test`, `ze-ui-test`, and every
`.ci` suite. The operator scoped this session to review and closure of code
already at HEAD and forbade running a test suite. The one gate run is
`make ze-test-pkg PKG=./cmd/ze/hub`, green. `make ze-tracked-build-check` runs
after commit A, which carries one Go comment change.

## Core Insight

A fallback consulted on REJECTION is a privilege path. The stale answer was not
in the fallback the defect report named: it was in the authenticator the chain
asked FIRST, and both looked identical from the outside, because a snapshot
cannot distinguish "the chain does not know this user" from "the config no
longer declares this user". Fixing the producer, once, made five credential
surfaces correct at the same time. Fixing each edge would have left four.
