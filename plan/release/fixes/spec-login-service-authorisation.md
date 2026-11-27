# Spec: login-service authorisation, a named set of doors an account may use

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P0 |
| Release reason | Mandatory authorization prerequisite for the release login-identity correction; identity without service authorization must not ship. |
| Scope | config |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Design tier | strongest tier |
| Implementation tier | strongest tier |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

Thomas ruled on all four open questions on 2026-11-24 (`OD-1`..`OD-4`). Every ruling is folded into the design, acceptance criteria, test plan, and file list below.

| ID | Decision |
|----|----------|
| OD-1 (RADIUS / TACACS+ users without a local user entry) | Option (d): add a login-service set leaf alongside the profile mapping in RADIUS and TACACS+ config so remote accounts can be restricted by login service too. |
| OD-2 (config user shadowing the stored appliance recovery username) | Option (a): keep the existing `ReservedRecoveryProfile` rule — a config user with that name is treated as an ordinary user whose login set applies (serial console recovery still works). |
| OD-3 (user with no login set configured) | Option (a): absent means allowed on all login services (existing configs keep working). |
| OD-4 (MCP) | Option (b): include MCP in login-service authorization too so operators can restrict MCP access alongside SSH, Web, REST, gRPC, Looking Glass, and gNMI. |

## Task

Ze authenticates a user, then authorises each command that user runs. Nothing
between those two steps asks whether the user may use the surface they arrived
on. `(*Store).Authorize` (`internal/component/authz/authz.go`) is the one
authorisation decision point, and it takes a command path, never a surface name.

The owner's design, given 2026-08-11 and refined by OD-1..OD-4 on 2026-11-24: a
new authorisation section under `system/authorization/`, sibling to `profile`,
naming the login services an account may use. A service that offers a login
passes its own name into the authentication request (or directly into the gate).
When the account's login set has no match for that name, authentication FAILS:
the credential does not work on that surface even though the account exists and
the password, key, or token is right.

This is an AUTHENTICATION gate, not an authorisation one. A refused login never
reaches command authorisation.

Service names are REGISTERED, not spelled in the schema. Each component that
offers a login registers its name at `init()`, and config validation checks a
configured value against the live registry.

**Scope of this spec: every door where a named account logs in today.** That is
SSH by password, SSH by public key, `ze cli` (which is an SSH login), the web
login form, web Basic Auth, the web session cookie, REST and gRPC with per-user
credentials, MCP with named principals (`bearer-list` and `oauth`, per OD-4
option b), and remote TACACS+ and RADIUS accounts via their `login-service`
mapping leaf (per OD-1 option d). The section "Login surfaces" lists every door
Ze has, and says for each one where the decision applies or why it does not.
The looking glass and gNMI currently authenticate with a shared token and learn
no username at all; giving them per-user identity and registering `"looking-glass"`
and `"gnmi"` with this registry is owned by
`plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md`, which
names this spec in its `Depends` row.

The SSH composition prerequisite closed in `6bbdb9c037` on 2026-08-17. It
remains historical context and no longer blocks this spec.

Whether withdrawing a door ends a session that is already open is governed by
the documented rule in `docs/guide/operator-access-rbac.md`, section 8 ("Removal
governs NEW connections. A session that is already open outlives it until the
connection closes", while the web session cookie is refused at once, and open
web SSE streams outlive until disconnect per `docs/architecture/web-interface.md`).
Withdrawing a door follows that exact rule because a login-set edit is a
narrower form of the same act. AC-5 and AC-13 assert it.

This design is orthogonal to `ExplicitAuthorizer` (`AuthorizeExplicit`) in
`aaa`, `authz`, `tacacs`, and `cmd/ze/hub/aaa_lifecycle.go`, and does not touch
command-authorisation interfaces or `(*Store).Authorize`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - "Identity and Authorization": how REST and gRPC obtain one accepted authentication generation per request
  → Constraint: users, policy, login sets, and API authentication publish as one generation (`acceptedLocalIdentityState`). Login sets join that generation; they are never read from the config provider at request time, because the provider holds a rejectable candidate during a reload.
  → Constraint: "A config user replaces a same-name stored user. Only surviving stored users retain their recovery profile."
- [ ] `docs/architecture/core-design.md` - Section 19 "Component Boundaries" and `## Design decisions` (2026-11-26 "Ze always starts")
  → Constraint: `internal/component/config/loginservice` is a stdlib-only leaf package under `config/` (mirroring `config/redistribute`); adding it to §19 preserves independent removability and acyclic downward imports.
- [ ] `docs/architecture/config/syntax.md` - the JUNOS-like config syntax: blocks, terminators, comments, leaf-list lowerings, and inheritance
- [ ] `docs/architecture/config/yang-config-design.md` - YANG (RFC 7950) as the schema language, and "Where a validator actually runs"
  → Constraint: a `ze:validate` runs only in a section `validatedSections` covers (`ai/doors/config-yang.md`, "Traps"). `system` is already in `validatedSections` (`internal/component/config/validate_sections.go`), and the implementation proves the walk reaches the new leaf.
- [ ] `docs/architecture/aaa-tacacs.md` - AAA chain priority, terminal `ErrAuthRejected` vs unreachable fallback, and `tacacs-profile` privilege-level mapping
  → Constraint: a gate refusal returns `ErrAuthRejected` so the chain never falls through to the next backend or opens the web fallback for a non-recovery account.
- [ ] `docs/architecture/mcp/overview.md` - "Authentication": per-request authentication, the four modes (`none`, `bearer`, `bearer-list`, `oauth`), and `Identity`
  → Constraint: MCP stays stateless per request; `none` and `bearer` remain anonymous (ungated), while named `bearer-list` and `oauth` principals pass the login-service gate under `"mcp"` on every POST.
- [ ] `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` - the stored bcrypt hash is a local-only credential, and is masked on display
- [ ] `docs/architecture/web-interface.md` - "Authentication": cookie, Basic Auth, `UserGeneration` per-request re-check, and remote-session handling
  → Constraint: an open SSE stream outlives a removal until the client disconnects. The same holds for a withdrawn door. A remote-backend web session is still never checked against the local user list; only its `LoginSet` (when set) is re-checked against the accepted login-service sets.
- [ ] `docs/architecture/appliance-serial-login.md` - the serial console authenticates in its own process against the stored credential
- [ ] `docs/architecture/plugin/feature-gates.md` - "Refusing a root whose plugin is compiled out"
  → Constraint: a refusal caused by a compiled-out feature says so and names the fix.
- [ ] `docs/guide/operator-access-rbac.md` - section 8, what removing an operator ends and what it leaves open
- [ ] `ai/rules/evidence.md` - the rule this change is governed by: the login set is a guard
  → Constraint: a login set that could not be READ MUST NOT read as "no restriction". A reader that cannot answer MUST deny.
  → Constraint: the test MUST drive the gate from a real login on a running daemon, never from the matching helper alone.
- [ ] `ai/patterns/registration.md` - the registration pattern this copies
  → Decision: the core discovers names through a registry; no central file lists the services.
- [ ] `docs/guide/authentication.md` - the operator-facing claim about who authenticates on each surface
  → Constraint: its table becomes incomplete the moment a correct credential can be refused for the surface it arrived on.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8907.md` - the TACACS+ `port` field carries the service name
  → Constraint: RFC 8907 Section 5.1: "The client documentation SHOULD define the values and their meanings for this field." The registered service name IS the value `tacacsAuthenticator.Authenticate` (`internal/component/tacacs/authenticator.go`) sends as `port`, and the value `radiusAuthenticator.exchange` (`internal/component/radius/authenticator.go`) sends as NAS-Port-Id. `docs/guide/tacacs.md` and `docs/guide/radius.md` define `ssh` and `web` today. The summary holds no row for this sentence.
  → Constraint: the names `ssh` and `web` are already on the wire. The registry adopts them unchanged. Renaming either changes what a TACACS+ or RADIUS server receives.

**Key insights:** (minimal context to resume after compaction)
- The registry-backed name check already exists and needs no invention: `RegisterSource` (`internal/component/config/redistribute/registry.go`) is written at `init()`, the leaf is `type string` carrying `ze:validate "redistribute-source"`, and `RedistributeSourceValidator` (`internal/component/config/validators.go`) calls `redistribute.LookupSource` at validate time and hands `redistribute.SourceNames` back as its `CompleteFn`. `CheckAllValidatorsRegistered` (`internal/component/config/yang/validator_registry.go`) fails a `ze:validate` name nobody registered.
- There is ONE local user list. `ExtractAuthUsers` (`internal/component/config/infra/authz.go`) reads `system/authentication/user` and is the only producer of `authz.UserConfig`, which is `aaa.UserCredential` (`internal/component/aaa/types.go`).
- There are SIX authentication paths across Ze's doors today (live AAA chain, SSH public key, private API authenticator, web fallback, SSH with no bundle, and MCP named principals). See Current Behavior.
- `aaa.AuthRequest.Service` already carries the service name. SSH password and web set it. SSH public key builds no request. REST and gRPC set nothing. MCP authenticates via `Streamable.authenticate` and builds no `AuthRequest`.
- `aaa.UserCredential` carries `Name`, `Hash`, `Profiles`, `PublicKeys`, and the runtime-only `UserGeneration` (5 fields, pinned by `TestSameLocalCredentialReadsEveryField`). It carries no `LoginSet` field yet; `aaa.AuthResult` carries no `LoginSet` field yet.
- The profile reference is a YANG leafref with `require-instance true` (`internal/component/authz/yang/ze-authz-conf.yang` and `internal/component/tacacs/yang/ze-tacacs-conf.yang`). The login-set reference on `user`, `tacacs-profile`, and `radius` takes the same leafref shape.
- `system` is already in `validatedSections` (`internal/component/config/validate_sections.go`), so `ze:validate "login-service"` under `system/authorization/login-service` runs on `LoadConfig`, `ze config validate`, and editor commit automatically.

## Current Behavior (MANDATORY)

**Source files read (verified against HEAD on 2026-11-27):**
- [ ] `internal/component/authz/authz.go` - `(*Store).Authorize` is the single command decision point and fails closed on an empty identity. Nothing on this path names a surface.
- [ ] `internal/component/aaa/types.go` - `AuthRequest{Username, Password, RemoteAddr, Service, Local}`; `UserCredential{Name, Hash, Profiles, PublicKeys, UserGeneration}`; `AuthResult{Authenticated, Profiles, Source, UserGeneration, Authorizer}`. `(*ChainAuthenticator).Authenticate` returns the first authenticated result, stops on `ErrAuthRejected`, and on any other error tries the next backend. `(*backendRegistry).Build` orders backends by priority (RADIUS 50, TACACS+ 100, local 200) and wraps the chain with `WithProfileAuthorizer`.
- [ ] `internal/component/aaa/login_profiles.go` - `resultAuthorizingAuthenticator.Authenticate`, the wrapper `WithProfileAuthorizer` returns, refuses a reserved username and binds an authorizer to a successful result. `AuthorizerForResult` keys the recovery grant on `ReservedRecoveryProfile` in a LOCAL result, and strips that profile from a remote result. `acceptedGenerationAuthorizer` also implements `AuthorizeLocalScope` and `AuthorizeExplicit` (orthogonal to this spec).
- [ ] `internal/component/aaa/doctor.go` - owns post-config AAA doctor checks (`aaaLocalFallbackDoctorCheck`, `doctor-aaa-no-local-fallback`, order 730). Owning-package doctor codes register via `diagnostic.Register` from `internal/component/aaa/register.go` (not `internal/core/diagnostic/codes.go`).
- [ ] `internal/component/authz/auth.go` - `(*LocalAuthenticator).Authenticate` matches the name and the bcrypt/SHA-crypt hash and returns `AuthResult{Authenticated, Profiles, Source: SourceLocal, UserGeneration}`. It reads `request.Local` and never reads `request.Service`. An unreadable user list returns that error, which is not `ErrAuthRejected`.
- [ ] `internal/component/config/infra/authz.go` - `ExtractAuthUsers` reads `system/authentication/user` from the resolved `system` map and is the ONE producer of the credential shape. `ValidateAuthzConfig` checks profile regex syntax and reserved names on profiles, users, and `tacacs-profile`; an undefined profile name is refused by the YANG leafref (`Validator.ValidateReferences`).
- [ ] `internal/component/authz/yang/ze-authz-conf.yang` - owns both `system/authentication/user` and `system/authorization/profile`. `leaf-list profile` is a leafref to `../../../authorization/profile/name` with `require-instance true`.
- [ ] `internal/component/tacacs/yang/ze-tacacs-conf.yang`, `internal/component/tacacs/config.go`, `internal/component/tacacs/authenticator.go`, `internal/component/tacacs/register.go` - `tacacs-profile` maps `level` (0..15) to `leaf-list profile` (leafref to `/authz:system/authz:authorization/authz:profile/authz:name`). `tacacsAuthenticator.Authenticate` sends `request.Service` as the START `port` field (falling back to `defaultService` = `"ssh"`), and `handlePass` returns `AuthResult{Authenticated, Profiles, Source: "tacacs"}` with no `LoginSet`.
- [ ] `internal/component/radius/yang/ze-radius-conf.yang`, `internal/component/radius/config.go`, `internal/component/radius/authenticator.go`, `internal/component/radius/aaa.go` - `system/authentication/radius` defines `server`, `auth-method`, `profile-attribute`, and `default-profile`, with no `login-service` leaf and no `ze-authz-conf` import yet. `radiusAuthenticator.exchange` sends `request.Service` as NAS-Port-Id (omitting it when empty), and `result` returns `AuthResult{Authenticated, Profiles, Source: "radius"}` with no `LoginSet`.
- [ ] `internal/component/ssh/yang/ze-ssh-conf.yang` - augments shared users only with SSH public keys and owns `environment.ssh`. It does not own the new login-set reference.
- [ ] `internal/component/ssh/passwordauth.go` - `(*Server).authenticatePasswordResult` calls the configured authenticator with `Service: componentSSH` (`"ssh"`) and `Local` from the accepted socket address.
- [ ] `internal/component/ssh/pubkey.go` - `(*Server).authenticatePublicKeyResult` reads `s.users()` and calls `matchPublicKey`. It calls NO `Authenticator`, builds no `AuthRequest`, and returns a result whose `Source` is `aaa.SourceLocal`.
- [ ] `internal/component/ssh/ssh.go` - the `wish.WithPublicKeyAuth` and `wish.WithPasswordAuth` callbacks are the two SSH login entry points. With no configured authenticator the server builds a bare `LocalAuthenticator` over its own user source, with no wrapper.
- [ ] `cmd/ze/hub/infra_setup.go`, `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go`, `cmd/ze/hub/service_registry.go` - SSH receives `liveAAABundleAuthenticator{}` with no fallback. `main.go` and `main_reload.go` construct `acceptedLocalIdentityState` via `newAcceptedLocalIdentity` and publish it via `publishAcceptedLocalIdentity`. `service_registry.go` defines `mcpServiceDeps` so always-on hub code never imports `internal/component/mcp`.
- [ ] `cmd/ze/hub/service_web.go` - web receives `liveAAABundleAuthenticator` with a `LocalAuthenticator` fallback over the accepted users.
- [ ] `cmd/ze/hub/aaa_lifecycle.go` - `liveAAABundleAuthenticator.Authenticate` asks the live bundle first; when the bundle does not authenticate and a fallback is set, a fallback success is returned for ANY bundle error that is not `ErrAuthRejected`, and after `ErrAuthRejected` only a local result carrying `ReservedRecoveryProfile` is returned. `acceptedLocalIdentityState` holds users, the policy store, `authzGeneration`, and `apiAuthentication`; `publishAcceptedLocalIdentity` stamps `UserGeneration`, publishes `aaa.SetAcceptedLocalUsers`, and stores the state atomically. `sameLocalCredential` compares every `UserCredential` field except `UserGeneration`, and `TestSameLocalCredentialReadsEveryField` (`cmd/ze/hub/rfc8907_aaa_lifecycle_test.go`) pins the field count at 5.
- [ ] `internal/component/web/auth.go` - `LoginHandlerWithAudit` authenticates the form with `Service: authServiceWeb` (`"web"`) and calls `(*SessionStore).createSession`. `AuthMiddlewareWithAudit` accepts a `ze-session` cookie through `(*SessionStore).validateToken`, and otherwise authenticates HTTP Basic Auth with `authServiceWeb` and serves with NO session. `validateToken` re-checks a `LocalAnchored` session via `localUserSessionStatus` against the live user list and `UserGeneration`. `InsecureMiddleware` serves with no login at all.
- [ ] `cmd/ze/hub/api.go` - `buildAPIAuthentication` builds a private authenticator for REST and gRPC: `WithProfileAuthorizer` over a `LocalAuthenticator` holding the accepted users. Its `AuthRequest` sets neither `Service` nor `RemoteAddr`. With no user at all and a token configured, it returns `aaa.ReservedSharedAPIUsername`. With neither, `Required` is false.
- [ ] `internal/component/api/types.go` - `Authentication{Required, Authenticate}`. `Authenticate` takes `authHeader string` and nothing else, so it cannot tell REST from gRPC.
- [ ] `internal/component/api/rest/auth.go` - `(*RESTServer).withAuth` calls `authentication.Authenticate` and sets `caller.Surface = audit.REST` after the fact.
- [ ] `internal/component/api/grpc/server.go` - `(*GRPCServer).checkAuth` calls `authentication.Authenticate` and sets `authenticated.Surface = audit.GRPC`, once per unary RPC and once per stream at open.
- [ ] `internal/component/mcp/auth.go`, `bearer.go`, `oauth.go`, `streamable.go`, `streamable_auth.go`, `streamable_tools.go`, `cmd/ze/hub/service_mcp.go` - `(*Streamable).authenticate` dispatches to `none`, `bearer`, `bearer-list`, or `oauth`. `none` and `bearer` return anonymous `Identity{}` (`Name == ""`); `bearer-list` and `oauth` return a named `Identity{Name, Scopes}` whose `Name` is already passed as `plugin.CallerIdentity.Username` to `s.dispatch.JSON` (reaching `authz.Store`). MCP has no login-service gate and does not register `"mcp"`.
- [ ] `cmd/ze/hub/main_servers.go` - `usersFromStore` reads the stored recovery user and gives it `aaa.ReservedRecoveryProfile`. `mergeAuthUsers` drops a stored user whose name a config user takes. `liveLocalUsers` assembles the candidate list at boot and at each reload.
- [ ] `internal/component/cli/sshclient/client.go` - `readCredentials` resolves the SSH credential `ze cli` presents to the daemon's SSH server, on `127.0.0.1` by default.
- [ ] `cmd/ze/login.go`, `cmd/ze-serial-shell/main.go` - the serial console runs `ze` as the login shell against the stored recovery credential directly; no daemon, no config user, and no AAA backend takes part.
- [ ] `internal/component/lg/auth.go`, `internal/component/gnmi/server.go` - compare one optional shared bearer token and learn no username (covered by sibling spec `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md`).
- [ ] `internal/component/plugin/acceptor.go` - `NewHubAcceptor`: an external plugin authenticates to the hub with a per-plugin token over TLS (process identity, not an operator login).
- [ ] `pkg/plugin/sdk/sdk_identity.go` - exposes `(*Plugin).InstanceName()` only; no external-plugin login-offering contract exists.
- [ ] `internal/component/config/redistribute/registry.go`, `internal/component/config/validators.go`, `internal/component/config/validators_register.go`, `internal/component/config/validate_sections.go` - `RegisterSource` / `RedistributeSourceValidator` / `RegisteredProtocolValidator` / `RegisterValidators` are the exact registry + validator + completion shape to copy; `system` is already in `validatedSections`.

**The six authentication paths today:**

| Path | Entry point | Who answers | Service named | Passes the wrapper |
|------|-------------|-------------|---------------|--------------------|
| Live AAA chain | SSH password; web form; web Basic Auth | `liveAAABundleAuthenticator.Authenticate`, then the bundle: RADIUS (50), TACACS+ (100), local (200) | `ssh`, `web` | Yes |
| SSH public key | `authenticatePublicKeyResult` | `matchPublicKey` over the server's user source; no authenticator is called | none | No |
| Private API authenticator | REST `withAuth`; gRPC `checkAuth` | The closure `buildAPIAuthentication` returns: local users only, never the chain | none | Yes |
| Web fallback | The same web entry points, when the chain did not authenticate | The `LocalAuthenticator` inside `liveAAABundleAuthenticator` | `web` | No |
| SSH with no bundle | SSH password, on a daemon where no backend composed | The bare `LocalAuthenticator` the SSH server builds for itself | `ssh` | No |
| MCP named principals | `(*Streamable).authenticate` (`bearer-list`, `oauth`) | `bearerListAuthenticator` or `oauthAuthenticator`; passes `Identity.Name` as `CallerIdentity.Username` to dispatcher | none | No |

**Behavior to preserve:**
- `(*Store).Authorize` keeps failing closed on an empty identity, and command authorisation (including `AuthorizeLocalScope` and `AuthorizeExplicit`) keeps its current verdicts for every user who is admitted.
- The stored recovery user, while no config user takes its name, keeps authenticating everywhere it does today, including its reserved recovery profile (OD-2 option a).
- The live user source survives: a user deleted from the config still stops authenticating at the next reload without a restart.
- Public-key SSH login keeps working, and the login set gates it identically to a password login.
- Every existing config keeps working with no edit (OD-3 option a).
- `ssh` and `web` stay the values sent to a TACACS+ and a RADIUS server.
- An explicit rejection still stops the chain, and a connection error still moves to the next backend (`ai/doors/access-security.md`, "Boundaries").
- A session that is already open outlives a config edit until its connection closes, while the web cookie does not, and an open web SSE stream survives until disconnect.
- MCP `auth-mode none` (including `le chaos run` Provider mode) and `auth-mode bearer` (single shared token) remain anonymous and unaffected, and per-identity `HasScope` on `tools/call` keeps its exact behavior.

**Behavior to change:**
- A new named list `login-service` under `system/authorization/` names a set of registered login services.
- `system/authentication/user`, `system/authentication/tacacs-profile`, and `system/authentication/radius` each gain an optional `login-service` leafref with `require-instance true` (OD-1 option d).
- Each login surface (`ssh`, `web`, `rest`, `grpc`, `mcp`) registers its own service name at `init()` in its own package, and every authentication path names its calling service: SSH password, SSH public key, web form/Basic/cookie, REST, gRPC, and MCP named principals (`bearer-list`, `oauth`).
- A login whose service name is not in the account's (or remote mapping's) login set is REFUSED on every path, with the same response the surface gives a wrong credential.
- A refusal by the gate is terminal (`aaa.ErrAuthRejected`): it never moves to the next backend and never opens the web fallback for a non-recovery account.
- Editing a login-service set definition or a user's `login-service` reference takes effect on reload without restart, including revoking an open web `ze-session` cookie whose login set no longer includes `web` and enforcing the new set on the next MCP POST.
- A post-config doctor check warns when every configured account is excluded from every registered door, and a Prometheus counter counts login-service refusals labeled by service.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Service name on the authentication request | `internal/component/aaa/types.go` `AuthRequest.Service` | Carries `ssh` or `web` to the backends, which send it to the TACACS+ and RADIUS server | Extends: the gate reads the same field. No second field is added. REST and gRPC start filling it; SSH public key and MCP pass their registered service name directly to the same decision |
| The authentication wrapper | `internal/component/aaa/login_profiles.go` `WithProfileAuthorizer`, `resultAuthorizingAuthenticator.Authenticate` | Wraps the chain and the private API authenticator; refuses a reserved username; binds the authorizer to a successful result | Extends: the login-set decision (`CheckLoginService`) runs here, after a backend authenticated and before the result is returned. The paths outside the wrapper (SSH public key, web fallback, bare SSH local authenticator, MCP) call the same decision function or wrap with `WithProfileAuthorizer` |
| Name registry with a validator | `internal/component/config/redistribute/registry.go` `RegisterSource`; `internal/component/config/validators.go` `RedistributeSourceValidator`, `RegisteredProtocolValidator` | Registers names at `init()`, validates a `ze:validate` leaf against them, completes them in the editor, refuses a conflicting re-registration, and lists offered names on refusal | Extends by copy: a second stdlib-only leaf registry `internal/component/config/loginservice` of the same shape for login-service names. The redistribute registry itself is not reused because its entries carry route-redistribution protocol metadata |
| Reference to a named list entry | `internal/component/authz/yang/ze-authz-conf.yang` `leaf-list profile`, `internal/component/tacacs/yang/ze-tacacs-conf.yang` `tacacs-profile/profile` | Leafrefs with `require-instance true` refuse at commit any reference to an undefined profile and list the profiles that exist | Extends: the `login-service` leaf on `user`, `tacacs-profile`, and `radius` uses the same leafref shape. `ValidateAuthzConfig` adds only reserved-name checks, never a duplicate existence check |
| Remote backend profile mapping | `internal/component/tacacs/config.go` `ExtractedConfig.PrivLvlMap`; `internal/component/radius/config.go` `ExtractedConfig.DefaultProfiles` | Maps TACACS+ `priv-lvl` and RADIUS admin auth to Ze authorization profiles carried on `AuthResult.Profiles` | Extends (OD-1 option d): carries the optional `LoginSet` from `tacacs-profile` and `radius` onto `AuthResult.LoginSet` so remote-authenticated accounts are gated by the exact same decision function |
| One accepted identity generation | `cmd/ze/hub/aaa_lifecycle.go` `acceptedLocalIdentityState`, `publishAcceptedLocalIdentity`, `liveAcceptedLocalUsers` | Publishes users, policy, and API authentication together atomically after every fallible reload step | Extends: the extracted login sets join the generation (`acceptedLocalIdentityState.loginSets` + `aaa.SetAcceptedLoginSets`). The gate never reads `ConfigProvider` |
| Per-user credential generation comparison | `cmd/ze/hub/aaa_lifecycle.go` `sameLocalCredential`, `stampLocalIdentityGeneration` | Advances `UserCredential.UserGeneration` when any credential field differs; `TestSameLocalCredentialReadsEveryField` pins the field count | Extends: `UserCredential.LoginSet` joins the compared fields and the pinned field count moves from 5 to 6 |
| Per-request re-check of a web session | `internal/component/web/auth.go` `(*SessionStore).validateToken`, `localUserSessionStatus` | Refuses a locally anchored session whose user was removed or whose `UserGeneration` changed; leaves remote sessions unchecked against the local user list | Extends: also checks the accepted login-service sets for `"web"` on every request (for both local sessions and remote sessions carrying `LoginSet`), so editing a `login-service` set revokes the cookie on the next request without checking remote users against the local user list |
| MCP per-request authentication | `internal/component/mcp/streamable.go` `(*Streamable).authenticate`; `cmd/ze/hub/service_mcp.go` `buildMCPService` | Authenticates every POST and passes named `Identity.Name` (`bearer-list`, `oauth`) to the command dispatcher as `CallerIdentity.Username` | Extends (OD-4 option b): wires `StreamableConfig.LoginServiceGate` from the hub to enforce the accepted generation's `"mcp"` login-service gate on every non-anonymous `Identity` before dispatch |
| Recovery identity | `internal/component/aaa/reserved.go` `ReservedRecoveryProfile`; `cmd/ze/hub/main_servers.go` `usersFromStore`; `aaa.AuthorizerForResult` | Marks the surviving stored recovery user's local result; lets it through an explicit chain rejection on web; keeps its command rights under a strict policy | Extends: the same mark (`SourceLocal` + `ReservedRecoveryProfile`) exempts the result from the login-set decision. No second key is invented (OD-2 option a) |
| Post-config AAA doctor check | `internal/component/aaa/doctor.go` `aaaLocalFallbackDoctorCheck`; `internal/component/aaa/register.go`; `internal/component/aaa/register_doctor_codes.go` | Checks AAA config readiness in `DoctorPhasePostConfig` and registers its own `doctor-*` code via `diagnostic.Register` | Extends: adds `doctor-aaa-all-accounts-excluded` in `internal/component/aaa/doctor.go`, registers its check in `internal/component/aaa/register.go`, and registers its `CodeMeta` in `internal/component/aaa/register_doctor_codes.go` |
| Refusal that names a compiled-out feature | `docs/architecture/plugin/feature-gates.md`, "Refusing a root whose plugin is compiled out" | Tells the operator the build lacks the feature, and how to fix it | Extends in kind: `LoginServiceValidator` refuses a name this build does not offer, states that this build does not offer it, and lists the names it does offer. Plugin `ConfigNeed` does not apply because a login surface is not a plugin dependency |
| Audit surface names | `internal/core/audit/audit.go` `SSH`, `Web`, `REST`, `GRPC`, `MCP` | Label audit records after the fact, including surfaces with no login | Does not apply: an audit surface set is broader than login surfaces; the login registry holds only surfaces that offer a login (Known Limitations) |
| Command authorisation | `internal/component/authz/authz.go` `(*Store).Authorize`, `AuthorizeLocalScope`, `AuthorizeExplicit` | Decides commands for an admitted user | Does not apply: the login-service gate decides at authentication, before any command exists, and stays orthogonal to `ExplicitAuthorizer` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "Code reaches a gated feature only through a registry or a seam that the feature fills from its own `init()`. A build without the tag then still compiles, and it answers that the feature is absent." | agrees | The stdlib-only `internal/component/config/loginservice` registry lets each gated surface (`ssh`, `web`, `rest`, `grpc`, `mcp`) self-register in `init()`; always-on `config` and `authz` import no gated listener package, so compiling out any `ze_<surface>` tag drops that name cleanly |
| `docs/architecture/core-design.md` | 19. Component Boundaries | anchored to `ze-authz-conf.yang`, `authz/auth.go`, `tacacs/register.go`, `aaa/types.go`, `cmd/ze/hub/main.go`, `main_reload.go`, `aaa_lifecycle.go`; `// Design:` of `aaa/login_profiles.go`; modified by this spec | "The hub resolves configured users over the zefs bootstrap users, pairs that credential set with its authorization store, and publishes one accepted generation. SSH, web, REST, and gRPC read the accepted live users, while command authorization reads the policy from the same generation. Reload retains the previous generation until the candidate succeeds." | agrees | Login-service sets (`LoginSets`) are extracted alongside the authorization store and published atomically in the same `acceptedLocalIdentityState` generation; SSH, web, REST, gRPC, and MCP read that one accepted generation, and reload retains the prior generation until the candidate succeeds. Section 19 is updated for `config/loginservice` and `LoginSets` |
| `ai/doors/access-security.md` | Who depends on it from outside | door covering `authz`, `tacacs`, `radius`, `aaa` | "Login is not an SSH feature. SSH, web, REST and gRPC read one accepted generation of users, and command authorization reads its policy from the same generation" | agrees | Every login surface (SSH password, SSH public key, web form/Basic/cookie, REST, gRPC, and named MCP) enforces `aaa.CheckLoginService` against the same accepted generation; local, TACACS+, and RADIUS feed one uniform `AuthResult.LoginSet` contract while preserving backend registration and fail-closed `ErrAuthRejected` semantics |
| `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` | Decisions | anchored to `authz/auth.go`, `ssh/passwordauth.go`; `// Design:` of `ssh/passwordauth.go` | "Only the SSH password callback sets it, from the accepted socket peer: a unix socket or a loopback TCP connection is local." | agrees | `authenticateUser` and `CheckPassword` keep their required `allowHashToken bool` parameter untouched and copy `u.LoginSet` onto `AuthResult` only after credential verification succeeds; the login-service gate runs after authentication and never relaxes the local-only bcrypt-hash restriction |
| `ai/patterns/registration.md` | The Model | `// Design:` of `tacacs/register.go`, `aaa/types.go`, `aaa/register.go`, `cmd/ze/hub/aaa_lifecycle.go`; modified by this spec | "Every module (plugin, schema, env var, command, validator) follows this model. The core never imports a specific plugin. Plugins never import each other." | agrees | `internal/component/config/loginservice` follows the `init()` self-registration pattern (`loginservice.MustRegister`), queried by `LoginServiceValidator` at runtime so no central file enumerates login surfaces; the page is updated to list `loginservice` in the registration inventory |
| `docs/research/l2tpv2-ze-integration.md` | L2TPv2 Ze Integration Design | `// Design:` of `radius/config.go`, `radius/authenticator.go` | | silent | Covers L2TPv2/BNG subscriber integration and subscriber RADIUS handling; it says nothing about operator management login-service restrictions on `system/authentication/radius` or `AuthResult.LoginSet` |
| `docs/architecture/config/syntax.md` | Custom YANG Extensions | `// Design:` of `config/infra/authz.go`; anchored to `cmd/ze/hub/main.go` | "References a custom validator function" | agrees | `system/authorization/login-service/service` uses the existing `ze:validate` extension (`"login-service"`) with `CompleteFn` derived from `loginservice.Names()`, adding no new YANG extension or config grammar form |
| `docs/architecture/config/yang-config-design.md` | Where a validator actually runs | anchored to `config/infra/authz.go`, `cmd/ze/hub/main.go`; `// Design:` of `config/validators.go`, `validators_register.go` | "Declaring a `ze:validate` is not sufficient to make it run. `ValidateCustomSections` iterates `validatedSections`, a list of top-level section names, and checks only inside those, so an annotation under any other section never executes." | agrees | `LoginServiceValidator` is registered via `RegisterValidator` on `system/authorization/login-service/service` under `system`, which is already in `validatedSections`, and the `login-service` references use native Layer-1 `leafref` (`require-instance true`), so `LoadConfig`, `ze config validate`, and the SSH editor commit share one verdict |
| `ai/doors/config-yang.md` | Boundaries | door covering `config/infra/authz.go`, `config/validators.go`, `validators_register.go`, `config/loginservice/` | "`./le yang glue update` regenerates each `yang/` package's `embed.go` and `register.go`." | agrees | Adding `import ze-authz-conf` to `ze-radius-conf.yang` (plus the additions to `ze-authz-conf.yang` and `ze-tacacs-conf.yang`) regenerates `yang/` glue with `./le yang glue update` rather than hand-editing `embed.go`/`register.go`, and extends schemas through standard YANG modules |
| `docs/architecture/config/system-update.md` | System Update: the backend, and the version check | anchored to `config/validators.go` | | silent | Anchored via `config/validators.go` for update-channel validation; says nothing about authentication, authorization, or login-service allow-lists |
| `docs/architecture/isis/isis-4-component-config.md` | IS-IS Component and Config | anchored to `config/validators.go` | | silent | Anchored via `config/validators.go` for IS-IS config validators; says nothing about operator authentication or login-service restrictions |
| `docs/architecture/ospf/ospf-4-component-config.md` | OSPF plugin registration and config | anchored to `config/validators.go` | | silent | Anchored via `config/validators.go` for OSPF config validators; says nothing about operator authentication or login-service restrictions |
| `docs/architecture/resolve.md` | Resolution Component | anchored to `config/validators.go` | | silent | Anchored via `config/validators.go` for DNS resolver config validators; says nothing about operator authentication or login-service restrictions |
| `docs/architecture/aaa-tacacs.md` | Reject and unreachable are different, and the chain owns the difference | anchored to `aaa/types.go`, `ssh/ssh.go`, `cmd/ze/hub/main_reload.go`, `aaa_lifecycle.go`; `// Design:` of `aaa/doctor.go`; modified by this spec | "The ZeFS super-admin uses the separately assigned reserved recovery profile. On web login, only a successful local result carrying that profile can override a central rejection." | agrees | `CheckLoginService` runs after `ChainAuthenticator.Authenticate` succeeds and returns terminal `ErrAuthRejected`, never turning a central rejection into local fallthrough, while keeping the `SourceLocal` + `ReservedRecoveryProfile` recovery exemption intact; the page is updated for `tacacs-profile/login-service`, `AuthResult.LoginSet`, and `doctor-aaa-all-accounts-excluded` |
| `docs/architecture/api/architecture.md` | REST and gRPC authentication flow | anchored to `aaa/types.go`, `aaa/login_profiles.go`, `ssh/ssh.go`, `cmd/ze/hub/main.go`, `main_reload.go`, `aaa_lifecycle.go`; `// Design:` of `api/types.go`, `cmd/ze/hub/api.go`, `api/rest/auth.go`, `api/grpc/server.go`; modified by this spec | "`newAcceptedLocalIdentity` calls `buildAPIAuthentication` once for each candidate generation. The builder wraps that generation's fixed users with `WithProfileAuthorizer`. A successful result carries the same generation's authorization view in `AuthResult.Authorizer`, beside the authenticated username. REST and gRPC obtain the accepted `api.Authentication` snapshot once for each request." | agrees | `newAcceptedLocalIdentity` carries `LoginSets` alongside the generation's users and authorizer and passes `service` into `AuthRequest.Service` via `buildAPIAuthentication`, so REST (`"rest"`) and gRPC (`"grpc"`) enforce login-service gates per request from the same atomic `api.Authentication` snapshot while leaving shared-token and no-auth paths untouched; the page is updated to document the per-transport login-service gate |
| `ai/doors/commands-cli.md` | Who depends on it from outside | door covering `ssh/passwordauth.go`, `ssh/pubkey.go`, `ssh/ssh.go`, `ssh/register_login_service.go` | "Public-key and password login (`internal/component/ssh` `pubkey.go`, `passwordauth.go`)" | agrees | Both SSH authentication callbacks (`pubkey.go` and `passwordauth.go`) enforce the `"ssh"` login-service gate uniformly while keeping existing SSH config options and the shared command dispatcher contract unchanged; absent `login-service` preserves full SSH access |
| `docs/architecture/system-architecture.md` | Ze System Architecture | `// Design:` of `ssh/pubkey.go`, `ssh/ssh.go`; anchored to `cmd/ze/hub/main.go` | | silent | Describes high-level component layering and BGP/RIB/plugin architecture; it does not cover SSH public-key or password admission or operator login-service restrictions |
| `docs/architecture/api/commands.md` | API Commands | `// Design:` of `ssh/ssh.go`; anchored to `ssh/ssh.go`, `api/grpc/server.go`, `mcp/streamable.go`, `cmd/ze/hub/main.go`, `main_reload.go` | | silent | Governs command dispatch paths, answer shapes, and pipe operators; this spec adds no commands, alters no command grammar or pipe behavior, and acts strictly at authentication before command dispatch |
| `docs/architecture/api/ipc_protocol.md` | Ze IPC Protocol Specification | anchored to `ssh/ssh.go` | | silent | Covers Unix-socket IPC framing between CLI/plugins and daemon; Unix-socket local peer credentials are not an operator login service and are unaffected |
| `docs/architecture/appliance/remote-operations.md` | Remote fleet operations | anchored to `ssh/ssh.go`, `cmd/ze/hub/main.go` | | silent | Covers appliance SSH transport for remote operational scripts and upgrades; it does not specify per-account login-service admission rules, and accounts without `login-service` keep full SSH access |
| `docs/architecture/web-interface.md` | Authentication | `// Design:` of `web/auth.go`; anchored to `web/auth.go`, `cmd/ze/hub/main.go`, `main_reload.go`, `aaa_lifecycle.go`; modified by this spec | "A session also ends when the running configuration stops declaring its user or changes that user's own credentials. The session records both which backend authenticated it (`AuthResult.Source`) and the accepted generation of its own user's record (`AuthResult.UserGeneration`), and `validateToken` re-checks a session the LOCAL backend granted against the live local user list and that user's current `UserGeneration` on every request." | agrees | `UserCredential.LoginSet` joins the `stampLocalIdentityGeneration` equality comparison so changing a user's `login-service` advances `UserGeneration`, and `validateToken` re-checks `aaa.CheckLoginSet` against live `LoginSets` on every request so editing a `login-service` set revokes open sessions immediately while still never checking remote sessions against the local user list; the page is updated for the `"web"` login-service gate |
| `docs/architecture/fleet-interface.md` | Fleet Interface | anchored to `web/auth.go` | | silent | Covers fleet UI routes, alarms, and device views mounted inside the web UI; it inherits `web` authentication and defines no separate login-service policy |
| `ai/doors/api-surfaces.md` | Who depends on it from outside | door covering `web/auth.go`, `api/types.go`, `api/rest/auth.go`, `api/grpc/server.go`, `mcp/streamable.go`, and `register_login_service.go` files | "Each surface carries its own authentication and TLS. A listener that serves without it is a security defect, not a default" | agrees | Each gated API surface (`web`, `rest`, `grpc`, `mcp`) owns its own `register_login_service.go` registration under its own build tag and enforces its own surface name at its authentication boundary without importing other surfaces or leaking into always-on code |
| `ai/doors/hub-engine.md` | Boundaries | door covering `cmd/ze/hub/*` | "Reach a gated service through the construction registry in `cmd/ze/hub/service_registry.go` or a blank import in the generated `internal/component/plugin/all`. Never import it from `cmd/ze/hub` directly" | agrees | `cmd/ze/hub/service_registry.go` threads the hub's `LoginServiceGate` callback through `mcpServiceDeps` into the `ze_mcp`-gated `service_mcp.go`, and `main.go`, `main_reload.go`, and `aaa_lifecycle.go` import no gated surface package |
| `docs/architecture/mcp/overview.md` | Authentication | `// Design:` of `mcp/streamable.go`; anchored to `mcp/streamable.go`, `cmd/ze/hub/service_mcp.go`; modified by this spec | "Every POST authenticates independently. There is no session ID that can stand in for a credential." | agrees | `StreamableConfig.LoginServiceGate` runs inside `handlePOST` step 7 right after the configured MCP authenticator validates a named `bearer-list` or `oauth` identity (`id.Name != ""`), reading the hub's live accepted generation on every POST so login-service edits take effect live on reload without touching static bearer/OAuth config reload rules; anonymous `none`, shared `bearer`, and Provider modes remain unaffected. The page is updated for `"mcp"` login-service gating |
| `ai/rules/plugins.md` | Directives | `// Design:` of `cmd/ze/hub/service_mcp.go`, `cmd/ze/hub/service_registry.go` | "A plugin MUST own its entire feature surface. Removing the plugin MUST make every one of its features disappear; every other plugin and the core MUST keep working." | agrees | Each gated login surface (`ssh`, `web`, `rest`, `grpc`, `mcp`) registers its own service name in its own gated package; compiling out any surface removes its service name from `loginservice.Names()` while every remaining surface and core config validation keep working |
| `docs/architecture/hub-architecture.md` | Hub Architecture | `// Design:` of `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go`; anchored to both | | silent | Documents the plugin-based configuration and external-process hub model (noting in its status header that the standalone orchestrator was removed); it does not cover operator AAA authentication or login-service allow-lists |
| `docs/architecture/api/process-protocol.md` | Plugin Process Protocol | anchored to `cmd/ze/hub/main.go` | | silent | Governs the newline-framed YANG RPC wire protocol between engine and plugins; plugin identities are not operator login services and this spec changes no RPC message or wire frame |
| `docs/architecture/behavior/signals.md` | Signal Handling | anchored to `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go` | | silent | Documents daemon OS signal handling (`SIGTERM`, `SIGHUP`, `SIGUSR1`); `SIGHUP` continues to drive `runReloadContext` unchanged while `runReloadContext` atomically publishes `LoginSets` inside `acceptedLocalIdentityState` |
| `docs/architecture/cli/command-completion.md` | Plugin Commands in Interactive Completion | anchored to `cmd/ze/hub/main.go` | | silent | Covers operational command-tree completion across CLI/SSH/web/MCP; config-editor completion for `login-service` uses standard YANG leafref and `ze:validate` completion (`loginservice.Names()`) |
| `docs/architecture/diagnostics/crash-capture.md` | Crash Capture | anchored to `cmd/ze/hub/main.go` | | silent | Covers stderr capture and panic crash files; unrelated to operator authentication or login-service authorization |
| `docs/architecture/diagnostics/production-diagnostics.md` | Production Diagnostics | anchored to `cmd/ze/hub/main.go` | | silent | Covers live operational diagnostics (`show system triage`, flight recorder, log ring); this spec adds no new RPCs and only registers a standard config-phase doctor check in `internal/component/aaa` |
| `docs/architecture/doctor-and-health-checks.md` | Where each owner registers its check | `// Design:` of `aaa/register_doctor_codes.go`; anchored to `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go` | "`diagnostic.RegisterDoctorCheck()` from the owning component's `init()`" | agrees | `aaaAllAccountsExcludedDoctorCheck` (`DoctorPhasePostConfig`, code `doctor-aaa-all-accounts-excluded`) is registered via `diagnostic.RegisterDoctorCheck` in `internal/component/aaa/register.go` with its `CodeMeta` in `internal/component/aaa/register_doctor_codes.go`, paired with both a unit test and a user-entry-point `doctor.Answer` test |
| `docs/architecture/fleet-config.md` | Managed Configuration | anchored to `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go`, `cmd/ze/hub/aaa_lifecycle.go` | | silent | Covers managed fleet edge/hub configuration delivery and blob layout; unrelated to operator login-service allow-lists |
| `docs/architecture/pki/pki-store.md` | The PKI certificate store | anchored to `cmd/ze/hub/main.go` | | silent | Covers X.509 certificate storage, local CA, and private-key export (`RequireExplicitAuthz`); orthogonal to login-service admission (this spec leaves `ExplicitAuthorizer` and PKI authorization untouched) |
| `docs/architecture/pki/tls-listeners.md` | Serving a PKI Certificate on a TLS Listener | anchored to `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go` | | silent | Covers TLS certificate selection and rotation on web, looking glass, and managed listeners; login-service checks run after TLS and credential authentication without touching listener TLS material |
| `docs/architecture/storage-backends.md` | Configuration storage | anchored to `cmd/ze/hub/main.go` | | silent | Covers `database/` tree storage, locking, and ZeFS backup/restore; login-service sets live in configuration under `system/authorization/login-service` and do not alter storage format or recovery |
| `docs/architecture/testing/ci-format.md` | .ci Test File Format | anchored to `cmd/ze/hub/main.go` | | silent | Covers `.ci` functional test syntax and runner discovery; this spec adds `.ci` files using existing syntax and changes no runner or format behavior |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `cmd/ze/hub/main_reload.go` | "The hub promotes a candidate after runtime reload succeeds. Publication failure returns an error and triggers an attempted runtime restoration." | agrees | Candidate `LoginSets` are built into the unpublished `acceptedLocalIdentityState` during `runReloadContext` and published atomically only after runtime reload succeeds; on failure, the prior generation's `LoginSets` remain active |
| `docs/architecture/plugin/component-boundaries.md` | The four components | anchored to `cmd/ze/hub/main_reload.go` | "The hub rejects a failed reload and calls the Engine again with the restored provider, so that rollback also reaches every registered subsystem." | agrees | `LoginSets` stay unpublished until `runReloadContext` commits the candidate and never leak across a failed reload rollback, and no `pkg/ze` interface or subsystem boundary changes |
| `docs/architecture/appliance/builder.md` | Appliance builder | anchored to `cmd/ze/hub/aaa_lifecycle.go` | | silent | Covers gokrazy appliance image building and installation; the ZeFS bootstrap super-admin retains its `SourceLocal` + `ReservedRecoveryProfile` exemption across login-service restrictions (OD-2) |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec touches only AAA/authz login-service configuration and admission gates; it adds no data-plane feature and leaves Linux-kernel vs VPP backend validation untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec does not touch data-plane mismatch checks; its own config validator (`LoginServiceValidator`) and leafref checks follow the same single-verdict discipline across commit, reload, and `ze config validate` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec touches no firewall or VPP verifier code and adds no `ze:backend` annotations |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec adds no data-plane backend or selector and leaves `kernel`/`vpp` naming untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec touches no BGP session or export-filter code and leaves BGP next-hop rewriting untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec touches no BGP export policy or `policychain` code and leaves both authoring forms untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec touches no BGP engine filter chains or `policychain.Chain` compilation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec touches no BGP UPDATE forwarding or export chain evaluation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec touches no BGP policy or next-hop validation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec touches no BGP route origination or forwarding paths |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec adds no automatic filter-chain members and leaves the implied-filter seam untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | `doctor-aaa-all-accounts-excluded` is a non-blocking `ze doctor` warning (it never causes `ze start` to exit), login-service enforcement runs at login admission rather than preventing any listener or daemon start, and a reload carrying an invalid `login-service` reference or unregistered service name is refused as a whole while running services keep the prior accepted generation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec leaves VPP/kernel mismatch handling untouched and follows the same non-blocking start plus `ze doctor` reporting discipline for its lockout warning |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec does not modify `| implied` rendering or insert automatic config-list members |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | This spec does not change bus participant registration or default message routing; login-service sets govern operator authentication doors, not bus participants |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | This spec touches no bus `send > to` configuration or compilation; note that `login-service` uses the same explicit absent-vs-empty distinction (absent reference allows all, empty `service` list denies all) |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | This spec touches no bus `receive > from` filtering and leaves plugin bus admission untouched |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | This spec touches no bus message allow-lists and leaves bus-level enforcement untouched |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This spec touches no bus filter compilation or conflict checks |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | This spec introduces no bus role arbitration and touches no plugin dissemination rules |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | This spec touches no bus effective-display commands or formatting |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | This spec adds no CLI commands, keywords, or selector words and acts strictly before command dispatch |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec adds no commands or pipe overrides and leaves pipe dispatch untouched |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec adds no pipe operations and leaves command execution semantics untouched |
| `docs/architecture/fleet-interface.md` | Design decisions | decision of 2026-11-25 | "The proposed alarm defaults apply together. A missing relay grant stays visible while the device is offline. A measured alarm uses its existing threshold duration after reconnect. Causes match within one device. Thomas selected “Use all three defaults”." | agrees | This spec touches no fleet alarm state or thresholds; fleet web routes inherit web authentication unchanged |
| `docs/architecture/fleet-interface.md` | Design decisions | decision of 2026-11-25 | "Thomas chose the following rule for Resource high at reconnect. “End the old occurrence. Show a new alarm only if fresh samples qualify.”" | agrees | This spec touches no fleet alarm reconnect handling |
| `docs/architecture/fleet-interface.md` | Design decisions | decision of 2026-11-25 | "When a critical alarm becomes visible again after its cause clears, show its toast to an authorized viewer of the affected device. Thomas selected “Show the critical toast”." | agrees | This spec touches no fleet alarm toast notifications |
| `docs/architecture/fleet-interface.md` | Design decisions | decision of 2026-11-25 | "For Force complete on a staged rollout, Thomas chose “Leave unstarted devices on their prior version”." | agrees | This spec touches no fleet staged rollout logic |
| `docs/architecture/fleet-interface.md` | Design decisions | decision of 2026-11-25 | "Thomas chose “Add the device Alarms tab”. The post-release device page offers the same authorized alarm rows as the device filter on the estate or group Alarms tab." | agrees | This spec touches no fleet device Alarms tab views |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no engine operations and leaves plugin-identity profile authorization untouched |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec touches no engine-to-plugin event delivery or queuing |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | This spec touches no managed-listener TLS certificate installation or reload logic |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | This spec touches no fleet device web Config tab provisioning; authorized web operators retain the same capabilities once admitted through `"web"` |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | This spec touches no looking-glass listener protocol or TLS configuration |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | This spec touches no TLS certificate precedence or listener TLS material loading |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-26 | "A certificate that cannot be presented never stops ze from starting. A web server whose certificate name does not resolve is not started. The looking glass is judged one `server` entry at a time: an entry that says `tls true` with no certificate to present stays down, every entry that asks for TLS stays down when the named certificate does not resolve, and the other entries serve. Each is reported with its fix by an error line in the log under a doctor code and by `ze doctor`. A reload whose config cannot be served stays refused. This supersedes the start half of the row of 2026-11-23 above, which says a `tls true` entry "is refused at start": at start that entry stays down and the others serve, and at reload the config is still refused. Thomas's words, as voice-transcribed: "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; "reload stay refused"; "Should only that endpoint stay down while the others serve: yes"" | agrees | This spec touches no TLS certificate loading or startup failure handling; its doctor warning (`doctor-aaa-all-accounts-excluded`) never stops `ze start` while invalid `login-service` configs stay refused on reload |

## Login surfaces

Every door Ze has, and where the decision applies. "Gated" means the login-set
decision runs before the caller is admitted.

| Surface | Authenticates through | Decision | Reason or place |
|---------|-----------------------|----------|-----------------|
| SSH password | `authenticatePasswordResult`, live AAA chain (RADIUS, TACACS+, local), or the bare local authenticator when no bundle composed | Gated, service `ssh` | In `resultAuthorizingAuthenticator.Authenticate`; the bare SSH local authenticator is wrapped with `WithProfileAuthorizer` too |
| SSH public key | `authenticatePublicKeyResult` | Gated, service `ssh` | Calls `aaa.CheckLoginService` directly after `matchPublicKey` succeeds: it builds no `AuthRequest` |
| SSH subsystems | The SSH login above (`subsystemOptions`, `internal/component/ssh/ssh.go`) | Gated with the login | No login of their own |
| `ze cli` | An SSH password login over loopback (`internal/component/cli/sshclient/client.go`) | Gated, service `ssh` | It is an SSH login. A set that excludes `ssh` also closes `ze cli` for that account, and the guide says so |
| Web login form | `LoginHandlerWithAudit`, live AAA chain (RADIUS, TACACS+, local), then the fallback | Gated, service `web` | In `resultAuthorizingAuthenticator.Authenticate`, and on the fallback's result in `liveAAABundleAuthenticator.Authenticate` |
| Web Basic Auth | `AuthMiddlewareWithAudit`, the same authenticator | Gated, service `web` | The same two places. No session exists to hold |
| Web session cookie | `(*SessionStore).validateToken` | Gated, service `web`, on every request | Re-checks the accepted generation (`UserGeneration` + accepted login sets for local sessions; accepted login sets for remote sessions carrying a `LoginSet`) |
| Web SSE stream already open | Authenticated at connect | Not re-checked | `docs/architecture/web-interface.md`: an open stream outlives a removal until the client disconnects |
| Web insecure mode | `InsecureMiddleware` | Not applicable | No login and no account. The listener guard already confines it |
| REST, per-user credentials | `withAuth`, private API authenticator | Gated, service `rest` | `resultAuthorizingAuthenticator.Authenticate` inside `buildAPIAuthentication`, once `Authentication.Authenticate` takes the service name |
| gRPC, per-user credentials | `checkAuth`, private API authenticator | Gated, service `grpc`, per RPC and at stream open | The same |
| gRPC stream already open | Authenticated at open | Not re-checked | The rule for an open connection (`docs/guide/operator-access-rbac.md`, section 8) |
| REST and gRPC shared token | `buildAPIAuthentication`, token arm | Not applicable | The caller is `aaa.ReservedSharedAPIUsername`, which is no account and can name no set. This mode exists only when no user is declared at all |
| REST and gRPC with no authentication | `Authentication` with `Required` false | Not applicable | Read-only, loopback only, no account |
| MCP (`bearer-list`, `oauth`) | `(*Streamable).authenticate` | Gated, service `mcp`, per POST (OD-4 option b) | After token/JWT verification produces a non-anonymous `Identity`, `LoginServiceGate` checks that principal against the accepted generation for `"mcp"` |
| MCP (`none`, `bearer` single token, Provider mode) | `(*Streamable).authenticate` | Not applicable | Anonymous (`Identity.Name == ""`); no named account to match against a login set |
| gNMI | Optional shared bearer token (`internal/component/gnmi/server.go`) | Outside this spec | No username today. Owned by `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md`, which registers `"gnmi"` with this registry |
| Looking glass | Optional shared bearer token (`internal/component/lg/auth.go`) | Outside this spec | No username today. Owned by the same dependent spec, which registers `"looking-glass"` with this registry |
| Serial console | `loginMain` (`cmd/ze/login.go`) | Not gated, by design | Runs with no daemon and reads no config, so no login set exists where it runs. Admits only the stored recovery user and is the recovery door of an appliance |
| Plugin sockets | Per-plugin token over TLS (`internal/component/plugin/acceptor.go`) | Not applicable | A process identity the hub issued, not an operator login |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator or client presents a credential on one of the gated surfaces above (SSH, Web, REST, gRPC, MCP).
- A config tree read at boot or at reload, carrying `system/authorization/login-service` and `login-service` references under `system/authentication/user`, `system/authentication/tacacs-profile`, and `system/authentication/radius`.
- Each login surface's `init()` registering its service name in `internal/component/config/loginservice`.

### Transformation Path
1. Each component that offers a login (`ssh`, `web`, `api/rest`, `api/grpc`, `mcp`) registers its service name at `init()`, from its own package, so a build without a gated surface (`ze_ssh`, `ze_web`, `ze_rest`, `ze_grpc`, `ze_mcp`) registers nothing for it.
2. Config validation resolves each configured service name in `system/authorization/login-service[*]/service` against the registry (`ze:validate "login-service"`, walked by `ValidateCustomSections` because `system` is in `validatedSections`), and each `login-service` leafref on `user`, `tacacs-profile`, and `radius` against the configured sets (`require-instance true`). `ValidateAuthzConfig` rejects reserved names.
3. `ExtractAuthUsers` produces `[]authz.UserConfig` carrying `LoginSet`; `ExtractLoginServices` produces `map[string][]string` (preserving empty sets as present keys with empty slices); `tacacs.ExtractConfig` produces `PrivLvlLoginSet`; `radius.ExtractConfig` produces `LoginSet`.
4. Boot (`cmd/ze/hub/main.go`) and each reload after its last fallible step (`cmd/ze/hub/main_reload.go`) publish users, policy, login sets (`aaa.SetAcceptedLoginSets`), and API authentication as one accepted generation via `publishAcceptedLocalIdentity`. Reload also swaps the rebuilt AAA bundle carrying the updated TACACS+/RADIUS `LoginSet` mappings.
5. A login arrives on one surface. The surface names its registered service: in `AuthRequest.Service` for SSH password, web form/Basic, REST, and gRPC; directly to `aaa.CheckLoginService` for SSH public key; and via `LoginServiceGate(identity.Name, "mcp")` for named MCP principals.
6. The backend authenticates the credential and returns `AuthResult` carrying `LoginSet` (from `UserCredential.LoginSet`, `tacacs-profile`, or `radius`). Only then does `aaa.CheckLoginService` run, on the authenticated result: it checks the recovery exemption, checks absent `LoginSet`, reads the accepted login-sets generation, and answers admit or refuse.
7. The decision refuses when the service is not in the set, when the set is empty, when the request names no service or an unregistered one, and when it cannot read the accepted generation or resolve the set. Every refusal is `aaa.ErrAuthRejected`, logged once with account, service, and reason, and increments the refused-by-login-set Prometheus counter labeled by service.
8. A local result carrying `aaa.ReservedRecoveryProfile` is admitted before reading any set (OD-2 option a); a remote result carrying that profile name is never exempt.
9. A refusal returns the surface's existing unauthorised response, unchanged in shape, and the surface records a failed login in the audit log.
10. An admitted login proceeds, and `(*Store).Authorize` (or MCP's `HasScope` + dispatcher RBAC) decides commands exactly as it does today.
11. On each later web request, `(*SessionStore).validateToken` re-checks the session against the accepted generation (`UserGeneration` + login-set check for `"web"` on local sessions; login-set check for `"web"` on remote sessions carrying a `LoginSet`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Login surface → name registry | A registration call at `init()` in the surface's own package (`ssh`, `web`, `api/rest`, `api/grpc`, `mcp`); the registry lives in `internal/component/config/loginservice` | Yes: copies `RegisterSource` (`internal/component/config/redistribute/registry.go`) |
| External plugin → name registry | Not crossed. An `init()` in a forked plugin writes that process's own copy (`ai/doors/system.md`, "A plugin can run in another process"), and `pkg/plugin/sdk/sdk_identity.go` holds no contract for an external plugin to offer an operator login | Yes: verified by reading `pkg/plugin/sdk/sdk_identity.go` (A-7) |
| Config tree → credential & login sets | `ExtractAuthUsers` and `ExtractLoginServices` (`internal/component/config/infra/authz.go`); `tacacs.ExtractConfig` and `radius.ExtractConfig` for remote mappings | Yes: one reader per config subtree, no duplicate parser |
| Config → accepted generation | `publishAcceptedLocalIdentity` (`cmd/ze/hub/aaa_lifecycle.go`) publishes `state.users`, `state.loginSets`, `aaa.SetAcceptedLocalUsers`, and `aaa.SetAcceptedLoginSets` atomically | Yes: single publication point in `aaa_lifecycle.go` |
| Surface → authentication request | `aaa.AuthRequest.Service` for SSH password, web, REST, gRPC; direct decision call for SSH public key; `StreamableConfig.LoginServiceGate` for MCP | Yes: no transport type crosses into `aaa` (A-2) |
| Authentication → authorisation | Unchanged. A refused login never reaches `(*Store).Authorize` or MCP `runMethod` | Yes: the gate sits in authentication before authorisation |
| Ze → TACACS+ and RADIUS server | The service name in TACACS+ `port` and RADIUS NAS-Port-Id | Yes: values `ssh` and `web` unchanged (see census) |

### Changed-Wire Census

No byte on any wire changes. The census is kept because the registered name is
the value two wires already carry, so a rename in the registry is a wire change
the next editor must see. Under OD-1 option (d), the remote backend's
`login-service` check runs locally inside Ze after the remote server's reply
arrives and adds no attribute or field to TACACS+ or RADIUS packets.

**Search evidence:** scope `internal/`, `cmd/`, and `pkg/`, non-test Go files. Terms: `Service:` as a struct field setter, and `.Service` read in `internal/component/tacacs`, `internal/component/radius`, `internal/component/aaa`, and `internal/component/authz`. Positive control: the search finds `Service: componentSSH` in `internal/component/ssh/passwordauth.go`, the known producer. Findings: two existing setters of `AuthRequest.Service` (SSH password, and web at two call sites) and two readers that put it on a wire (TACACS+ `port`, RADIUS `NAS-Port-Id`). REST and gRPC use the local-only private API authenticator and reach neither remote wire reader; MCP uses `LoginServiceGate` over the accepted local generation and reaches neither remote wire reader.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| TACACS+ port field | writer | `internal/component/tacacs/authenticator.go` `tacacsAuthenticator.Authenticate` | Reads `request.Service`, falls back to `defaultService` (`internal/component/tacacs/client.go`); the session authorization carries the same value | Preserve: `ssh` and `web` unchanged. The fallback stays, and after this spec no gated path sends an empty name |
| TACACS+ port field | reader | The operator's TACACS+ server, outside the repository | RFC 8907 Section 5.1, free-format text | Preserve: the server sees the same values. `docs/guide/tacacs.md` stays their definition |
| RADIUS NAS-Port-Id | writer | `internal/component/radius/authenticator.go` `radiusAuthenticator.exchange`, called from `Authenticate` and from the EAP path in `internal/component/radius/authenticator_eap.go` | Passes `request.Service` to `AppendTextAttr` as NAS-Port-Id | Preserve: values unchanged |
| RADIUS NAS-Port-Id | reader | The operator's RADIUS server, outside the repository | `docs/guide/radius.md`, the admin login attributes | Preserve |

### Integration Points
- `ExtractAuthUsers`, `ExtractLoginServices`, `ValidateAuthzConfig` (`internal/component/config/infra/authz.go`) - the one credential and login-set producer.
- `tacacs.ExtractConfig`, `tacacsAuthenticator.handlePass` (`internal/component/tacacs/config.go`, `authenticator.go`, `register.go`) - carries `tacacs-profile[*].login-service` onto `AuthResult.LoginSet` (OD-1 option d).
- `radius.ExtractConfig`, `radiusAuthenticator.result` (`internal/component/radius/config.go`, `authenticator.go`) - carries `radius.login-service` onto `AuthResult.LoginSet` (OD-1 option d).
- `(*LocalAuthenticator).Authenticate` (`internal/component/authz/auth.go`) - copies `u.LoginSet` onto `AuthResult.LoginSet`.
- `resultAuthorizingAuthenticator.Authenticate`, `CheckLoginService`, `CheckLoginSet`, `SetAcceptedLoginSets` (`internal/component/aaa/login_profiles.go`) - the one fail-closed decision point for the chain and the private API authenticator.
- `authenticatePublicKeyResult` (`internal/component/ssh/pubkey.go`) - calls `aaa.CheckLoginService` after `matchPublicKey` succeeds.
- The bare local authenticator in `internal/component/ssh/ssh.go` - wrapped with `aaa.WithProfileAuthorizer`, so a daemon with no bundle is gated too.
- `liveAAABundleAuthenticator.Authenticate` (`cmd/ze/hub/aaa_lifecycle.go`) - the web fallback's result passes `aaa.CheckLoginService`, and a gate refusal from the bundle is terminal.
- `(*SessionStore).validateToken` (`internal/component/web/auth.go`) - per-request re-check of `UserGeneration` and the `web` login-service gate.
- `buildAPIAuthentication` (`cmd/ze/hub/api.go`) and `api.Authentication` (`internal/component/api/types.go`) - `Authenticate(authHeader, service string)` learns whether REST or gRPC called.
- `(*Streamable).authenticate` (`internal/component/mcp/streamable.go`), `buildMCPService` (`cmd/ze/hub/service_mcp.go`), `mcpServiceDeps` (`cmd/ze/hub/service_registry.go`), and `checkAcceptedLocalLoginService` (`cmd/ze/hub/aaa_lifecycle.go`) - gates named MCP principals under `"mcp"` without coupling `mcp` to `aaa` or always-on hub code to `mcp` (OD-4 option b).
- `publishAcceptedLocalIdentity` and `sameLocalCredential` (`cmd/ze/hub/aaa_lifecycle.go`), `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go` - the accepted generation carries `loginSets` and `sameLocalCredential` compares `LoginSet`.
- `RegisterValidators` (`internal/component/config/validators_register.go`) - wires `ze:validate "login-service"`.
- `internal/component/aaa/doctor.go` and `register.go` - registers the post-config doctor check `doctor-aaa-all-accounts-excluded`.
- `(*Store).Authorize` (`internal/component/authz/authz.go`) - NOT edited. Command authorisation and `ExplicitAuthorizer` stay separate and untouched.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes, by design | One decision function (`aaa.CheckLoginService` / `CheckLoginSet`). The wrapper covers the AAA chain (local, TACACS+, RADIUS) and the private API authenticator; the four paths outside it (SSH public key, bare SSH local authenticator, web fallback, MCP named principals) all route through the same decision or wrapper. AC-2, AC-12, AC-19, and AC-20 prove every door separately |
| No unintended coupling (components stay isolated) | Yes | `internal/component/config/loginservice` is a stdlib-only leaf beside `config/redistribute`. `aaa` learns no transport type (reads `AuthRequest.Service` string). `mcp` imports only `config/loginservice` (stdlib leaf) and takes a hub-wired `LoginServiceGate` callback, preserving `ze_mcp` compile-out |
| No duplicated functionality (extends existing, does not recreate) | Yes | See Existing Mechanisms: every part extends an existing mechanism (`AuthRequest.Service`, `WithProfileAuthorizer`, `redistribute` registry/validator pattern, YANG leafrefs, `acceptedLocalIdentityState`, `validateToken`, `aaa/doctor.go`) |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Login path, no wire buffers |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Each login surface (`ssh`, `web`, `api/rest`, `api/grpc`, `mcp`) adds one registration call in its own package. No shared file lists the service names |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes, with one stated exception | Searched: audit constants (`internal/core/audit/audit.go`), `registerService` (`cmd/ze/hub/service_registry.go`), `DiscoverListenerServices` (`internal/component/config/listener.go`), `defaultService` (`internal/component/tacacs/client.go`). None learns a new name; `LoginServiceValidator` and editor completion derive their values from `loginservice.Names()`. The exception is `defaultService`, an existing literal `"ssh"` for an unnamed TACACS+ caller; it stays |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The registry-backed name check needs no new mechanism | `RegisterSource` / `RedistributeSourceValidator` / `RegisteredProtocolValidator` / `RegisterValidators` were read and match this shape exactly | The spec invents a mechanism the repo already has, against `ai/rules/simplicity.md` | The implementation's validator is a direct copy of `RedistributeSourceValidator` + `RegisteredProtocolValidator`, reviewed side by side | confirmed |
| A-2 | REST and gRPC can name themselves by passing a string into `api.Authentication.Authenticate`, with no transport type crossing into `aaa` | The closure is called from exactly two places (`withAuth` and `checkAuth`), each of which already knows its transport | The threading grows a parameter through unrelated layers | Signature change `Authenticate func(authHeader, service string) (CallerIdentity, bool)` compiles against both call sites and tests | confirmed |
| A-3 | OD-3 option (a) keeps every existing config working | No config in the repository or field carries `login-service` yet | An upgrade locks operators out of their own daemon | AC-7 and `test/plugin/login-service-absent-section.ci` | confirmed |
| A-4 | The stored recovery user is recognized by `aaa.ReservedRecoveryProfile` on a local result, and no config user or remote backend can carry that profile | `usersFromStore` (`cmd/ze/hub/main_servers.go`) is the only producer; `ValidateAuthzConfig` rejects reserved names; `tacacs`, `radius`, and `AuthorizerForResult` strip reserved names from remote results | A config or remote user gains the exemption, or the recovery user loses it | AC-8, AC-14, and `TestLoginDecisionAdmitsRecoveryResult` | confirmed |
| A-5 | Registration completes before the first config validation reads the registry | Every surface package's `init()` runs before `main()` via generated `plugin/all` and hub imports | A name registered later is rejected at validate time | `TestLoginServiceRegistryRejectsConflictingRegistration` and `test/parse/login-service-unregistered-name.ci` | confirmed |
| A-6 | Every surface maps a gate refusal to the response it gives a wrong credential | Each entry point tests for `err == nil && result.Authenticated` (or `LoginServiceGate` returning `nil` on MCP) and shares one failure branch | A refusal is distinguishable from a wrong credential, leaking account existence | AC-2 and AC-20 compare refused-by-gate vs wrong-credential responses on every door | confirmed |
| A-7 | No login-offering contract exists for an external plugin | Read `pkg/plugin/sdk/sdk_identity.go` on HEAD (2026-11-27): exposes only `(*Plugin).InstanceName()` | An external plugin's login is ungated | Confirmed by reading `pkg/plugin/sdk/sdk_identity.go` (re-verified at closure) | confirmed |
| A-8 | The validator walk reaches a `ze:validate` leaf under `system/authorization` | Read `internal/component/config/validate_sections.go` on HEAD (2026-11-27): `"system"` is already in `validatedSections` | The validator is declared and never runs | `test/parse/login-service-unregistered-name.ci` and `TestEveryValidatorSectionIsWalkedOrExcused` | confirmed |
| A-9 | MCP named principals (`bearer-list` identity name, `oauth` JWT `sub`) share the username namespace with `system/authentication/user` | `internal/component/mcp/tools.go` already passes `scope.Identity.Name` as `plugin.CallerIdentity.Username` to `pluginserver.Dispatcher` / `authz.Store` | An MCP identity name never matches a local user's `login-service` set | AC-20 and `test/plugin/login-service-mcp.ci` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator locks themselves out of every surface, including the one they would use to undo it | A config where every account's login set excludes every service | The stored recovery user is exempt while unshadowed (OD-2 option a, AC-8), the serial console is outside this gate, and `doctor-aaa-all-accounts-excluded` warns with a fix command |
| R-2 | The gate is added to some paths and forgotten on another, so one door silently stays open | A path that returns an authenticated result without calling the decision | AC-2, AC-12, AC-19, and AC-20 cover every gated door separately; the six paths are named in Current Behavior and driven by the wiring tests |
| R-3 | A read failure resolves to an empty login set or absent reference, which reads as "no restriction" | A failed/uninitialized generation reaching the matcher with a nil error | AC-6: `acceptedLoginSets.Load() == nil` or a missing set name always returns `ErrAuthRejected`. Absent reference (`""`) is distinct from empty set (`[]string{}`) and from missing/unreadable state |
| R-4 | The gate is placed after command authorisation, so a refused user still runs a command or holds a session | A test asserting status code without checking side effects | The gate runs inside authentication (`resultAuthorizingAuthenticator`, `authenticatePublicKeyResult`, `Streamable.authenticate`). AC-3 and AC-20 assert zero side effects |
| R-5 | Editing a `login-service` set without editing the `user` entry leaves open web cookies alive | `UserGeneration` only compares `UserCredential.LoginSet` (the set name), not the set's member list | `(*SessionStore).validateToken` re-checks `aaa.CheckLoginSet` against the live accepted login sets on every request (for both local and remote web sessions), in addition to `UserGeneration` |
| R-6 | A name registered by two components with different meanings overwrites silently | Second `Register` silently replaces the first | Return a conflict error on conflicting re-registration (copying `redistribute.ErrSourceConflict`) and panic with `BUG:` in `init()` |
| R-7 | A gate error that is not `ErrAuthRejected` moves the chain to the next backend or opens the web fallback | A refused login succeeds on a daemon with two backends, or on web | Every refusal from `CheckLoginService` returns `aaa.ErrAuthRejected` and runs outside `ChainAuthenticator`. The web fallback passes `CheckLoginService` too. AC-6 and AC-15 |
| R-8 | REST and gRPC share one closure, so one of them is gated under the other's name | An account allowed on REST only is admitted on gRPC | A-2 and AC-17 test the pair with complementary sets |
| R-9 | A registry rename changes the value a TACACS+ or RADIUS server receives | A server-side policy keyed on `port` / `NAS-Port-Id` stops matching | The census pins `ssh` and `web` unchanged; `TestRegisteredNamesMatchWireValues` enforces both |
| R-10 | A build without a surface's tag (`ze_web`, `ze_mcp`, etc.) refuses a config naming it without saying why | "unknown login service" on a documented name | `LoginServiceValidator` names the value, states that this build does not offer it, and lists the names this build offers (AC-9) |
| R-11 | Adding `import ze-authz-conf` to `ze-radius-conf.yang` breaks a test binary that imports `radius/yang` without `authz/yang` | `TestThisBinaryLinksEveryModuleItsSchemaImports` fails | Run `./le yang glue update` to regenerate `internal/component/radius/yang/register.go` with the blank import for `authz/yang` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Two asymmetric directions. Too permissive: a surface admits an account that should be refused (the status quo). Too strict: an operator is locked out of every management surface of a running router with no remote way back. Every AC tests BOTH refused and admitted cases on one daemon, and the stored recovery user stays exempt (OD-2 option a) |
| How is it reverted? | Single commit revert. Any config using the new `login-service` section or leaf must remove it before an older binary accepts it. Ze is pre-release, so no migration is owed (`ai/META-DECISIONS.md`, "Ze is pre-release") |
| Who else touches this path? | `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` depends on this spec to register `"gnmi"` and `"looking-glass"` into `internal/component/config/loginservice`. `spec-pki-show-private-key` adds `ExplicitAuthorizer` to `aaa`, `authz`, `tacacs`, and `cmd/ze/hub/aaa_lifecycle.go`; this spec is cleanly orthogonal and touches none of the `Authorize*` command-authorisation methods |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A config declaring a login set and a user referencing it, validated at load | → | `LoginServiceValidator` wired in `RegisterValidators` (`internal/component/config/validators_register.go`) | `test/parse/login-service-unregistered-name.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| An SSH password login by a user whose set excludes `ssh` | → | `(*Server).authenticatePasswordResult` (`internal/component/ssh/passwordauth.go`), then `resultAuthorizingAuthenticator.Authenticate` | `test/plugin/login-service-ssh-password.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| An SSH public-key login by the same user | → | `(*Server).authenticatePublicKeyResult` (`internal/component/ssh/pubkey.go`) → `aaa.CheckLoginService` | `test/plugin/login-service-ssh-publickey.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A `ze cli -c` command by the same user | → | the SSH password path | `test/plugin/login-service-ze-cli.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A web form login by a user whose set excludes `web` | → | `LoginHandlerWithAudit` (`internal/component/web/auth.go`) → wrapper + fallback gate | `test/plugin/login-service-web-form.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A web Basic Auth request by the same user | → | `AuthMiddlewareWithAudit` (`internal/component/web/auth.go`) | `test/plugin/login-service-web-basic.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A web request with a cookie issued before a reload removed `web` from the set | → | `(*SessionStore).validateToken` (`internal/component/web/auth.go`) | `test/plugin/login-service-web-cookie-reload.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A REST request by a user whose set excludes `rest` | → | `(*RESTServer).withAuth` (`internal/component/api/rest/auth.go`) → `buildAPIAuthentication` | `test/plugin/login-service-rest.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A gRPC call by a user whose set excludes `grpc` | → | `(*GRPCServer).checkAuth` (`internal/component/api/grpc/server.go`) → `buildAPIAuthentication` | `test/plugin/login-service-grpc.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| An MCP `bearer-list` / `oauth` POST by a named principal whose set excludes `mcp` | → | `(*Streamable).authenticate` (`internal/component/mcp/streamable.go`) → `checkAcceptedLocalLoginService` | `test/plugin/login-service-mcp.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A TACACS+ login whose `tacacs-profile` names a login set excluding `ssh` | → | `tacacsAuthenticator.handlePass` (`internal/component/tacacs/authenticator.go`) → `resultAuthorizingAuthenticator.Authenticate` | `test/plugin/login-service-tacacs.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A RADIUS login whose `radius` block names a login set excluding `web` | → | `radiusAuthenticator.result` (`internal/component/radius/authenticator.go`) → `resultAuthorizingAuthenticator.Authenticate` | `test/plugin/login-service-radius.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## Acceptance Criteria

Each row is driven through the real entry point on a running daemon unless it
names config validation. "Excluded door" and "included door" mean two cases on
ONE daemon with ONE account: the same credential is refused on the door the set
leaves out and admitted on a door it holds.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A component offering a login registers its service name at startup | The name is present in `loginservice.Names()` before any config validation runs, and editor completion offers it. A second registration of the same name by a conflicting owner returns `ErrConflict` | A repeat registration with the same owner and description is accepted idempotently, matching `redistribute.RegisterSource` |
| AC-2 | An account whose login set excludes a door presents a CORRECT credential to it. One case per gated door: SSH password, SSH public key, `ze cli`, web form, web Basic Auth, REST, gRPC, MCP (`bearer-list` / `oauth`) | Each door answers with the exact same status and body it gives a wrong credential, and the same account with the same credential is admitted on a door its set includes | The stored recovery user (AC-8). A connection already open before reload (AC-13) |
| AC-3 | The refused login of AC-2, on a door that would have dispatched a command or created a session | The audit log holds one failed-login record for the attempt, the refused-by-login-set Prometheus counter increments for that service, and the daemon's state matches its state before the attempt (`show` output and config unchanged; web returns the login page; MCP runs no tool) | None: a refusal that leaves any side effect is the defect |
| AC-4 | The same account presents the same credential to a door its set includes | The login succeeds, and a command the account's profile allows returns its normal answer while one it denies is refused as today | None beyond today's command verdicts |
| AC-5 | A running daemon, a config edit removing a door from an account's set (either by editing the set's `service` list or by changing the user's `login-service` reference), then a reload | The next login on that door is refused with the wrong-credential response, with no restart, and a login on a door still in the set succeeds | A connection opened before the reload (AC-13) |
| AC-6 | The login set of an account cannot be resolved at login: the accepted generation is unreadable/uninitialized, or the reference names a set the generation does not hold | The login is refused with the wrong-credential response (`ErrAuthRejected`), and the daemon log names the account, the service, and the reason once | The stored recovery user, whose local result is admitted before any set is read (OD-2 option a) |
| AC-7 | A user (or remote backend mapping) naming no `login-service` set at all | Every gated door admits it with a correct credential, so an untouched config keeps working after an upgrade (OD-3 option a) | None |
| AC-8 | The stored recovery user, with no config user of its name, on a config where every config user's set excludes every door | It authenticates on SSH by password and by key, on `ze cli`, on web, REST, and gRPC, and runs a command on each | None |
| AC-9 | A config naming a login service that this build did not register (for example a typo, or a compiled-out surface such as `mcp` in a `!ze_mcp` build) | Config validation rejects it at load, `ze config validate`, and commit. The message names the value, states that this build does not offer it, lists the names this build does offer, and editor completion offers only registered names | Any name registered by a surface compiled into this build |
| AC-10 | A `user`, `tacacs-profile`, or `radius` entry referencing a `login-service` set that is not defined under `system/authorization/login-service` | Validation and commit reject it via the YANG leafref (`require-instance true`) and list the sets that exist, as an undefined profile reference is rejected today | None |
| AC-11 | A `login-service` set that names no `service` entry at all, referenced by a user or remote mapping | The config is accepted, and that account is refused on every gated door. This is distinct from AC-7 (absent reference) | The stored recovery user |
| AC-12 | A user authenticated by TACACS+ (whose matched `tacacs-profile` names a `login-service` set) or by RADIUS (whose `system/authentication/radius` block names a `login-service` set), with no local user record (OD-1 option d) | On an excluded door, the login is refused terminally with the wrong-credential response (`ErrAuthRejected`, never falling through to local); on an included door, the login succeeds with the mapped profiles. When `login-service` is absent on `tacacs-profile` or `radius`, the remote user is admitted on every door (OD-3 option a) | None |
| AC-13 | An account with an open SSH session, an open gRPC stream, and a web session cookie, then a reload removing those doors from its set (whether by editing the user's `login-service` leaf or editing the set's `service` list) | The open SSH session and the open gRPC stream keep answering until their connection closes, while the next web request with the old cookie receives HTTP 401 and loses its session (both for local and remote sessions carrying a `LoginSet`) | An open web SSE stream, until the client disconnects (`docs/architecture/web-interface.md`) |
| AC-14 | A config user that takes the stored recovery user's name and references a set excluding `ssh` | The account is refused on SSH and admitted on a door its set holds: it is an ordinary config account (OD-2 option a), while the appliance serial console still works | None under OD-2 option (a) |
| AC-15 | A daemon with a remote backend and the local backend, and a local account whose set excludes `web`, presenting its correct password to the web form (and a remote-authenticated account whose set excludes `web`) | The login is refused with the wrong-credential response, the web fallback does not admit the refused non-recovery account, and the daemon log shows the gate's reason once for the attempt | The stored recovery user (local `ReservedRecoveryProfile`) |
| AC-16 | REST or gRPC in shared-token mode, on a config that defines login sets | A request with the token is admitted as today under `ReservedSharedAPIUsername` | None: shared-token mode carries no account to restrict and exists only while no user is declared |
| AC-17 | An account whose set holds `rest` and not `grpc`, and a second account with the reverse | Each account is admitted on its one transport and refused on the other | None |
| AC-18 | The service name sent to a remote backend for an SSH and for a web login | A TACACS+ START carries `ssh` and `web` in its `port` field, and a RADIUS Access-Request carries the same in NAS-Port-Id, byte for byte as before this spec | None |
| AC-19 | A daemon on which no AAA bundle composed, so SSH answers from its own local authenticator, and an account whose set excludes `ssh` | The SSH password login is refused with the wrong-credential response, and the stored recovery user is admitted | The stored recovery user |
| AC-20 | An MCP POST in `bearer-list` or `oauth` mode carrying a named principal (`Identity.Name`) whose `system/authentication/user` record references a `login-service` set excluding `mcp` (OD-4 option b) | MCP answers HTTP 401 with the same `Bearer error="invalid_token"` challenge as a bad token, records one `audit.ActionAuthFail`, and executes no method or tool; adding `mcp` to the set and reloading admits the next POST with no restart | Anonymous MCP modes (`none`, `bearer` single shared token, Provider mode) and named principals with no `login-service` set (OD-3 option a) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | creates a monitoring account allowed on the API only, and tries to SSH in with it | config → accepted generation → SSH login → wrapper → `CheckLoginService` | `test/plugin/login-service-ssh-password.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 2 | uses that same account against the REST API | config → accepted generation → REST `withAuth` → private authenticator → `CheckLoginService` → dispatcher | `test/plugin/login-service-rest.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 3 | mistypes a service name in the config | config load → `LoginServiceValidator` → rejection naming the value and the names offered | `test/parse/login-service-unregistered-name.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 4 | removes a door from an account's set on a running router | reload → published generation → next login refused, open web cookie refused | `test/plugin/login-service-reload-tightens.ci`, `test/plugin/login-service-web-cookie-reload.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 5 | locks every config account out, runs `ze doctor`, then recovers | `doctor-aaa-all-accounts-excluded` reports fix; stored recovery user → `CheckLoginService` exempts local recovery result → any door | `test/plugin/login-service-recovery-user.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 6 | restricts a TACACS+ privilege level or RADIUS admin backend to SSH only and tries to log in on Web | config → `tacacs-profile`/`radius` `login-service` → `AuthResult.LoginSet` → `CheckLoginService` → terminal `ErrAuthRejected` | `test/plugin/login-service-tacacs.ci`, `test/plugin/login-service-radius.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 7 | restricts a named operator account from MCP while keeping SSH open, then adds `mcp` on reload | config → accepted generation → MCP `Streamable.authenticate` → `LoginServiceGate` → 401 then 200 after reload | `test/plugin/login-service-mcp.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## 🧪 TDD Test Plan

### Unit Tests
No test sits on `(*Store).Authorize`, which this spec does not edit. The
decision's tests sit beside the decision.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoginServiceRegistryRejectsConflictingRegistration` | `internal/component/config/loginservice/registry_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-1: a second registration that disagrees returns `ErrConflict`, not an overwrite | |
| `TestLoginServiceValidatorRejectsUnregisteredName` | `internal/component/config/validators_test.go` | AC-9: an unregistered name fails validation, the message names the offered services, and completion lists only registered names | |
| `TestExtractAuthUsersAndLoginServices` | `internal/component/config/infra/authz_test.go` | `LoginSet` reaches `UserCredential`; `ExtractLoginServices` preserves empty sets distinct from absent sets; reserved names refused | |
| `TestLoginDecisionAbsentSetAdmits` | `internal/component/aaa/login_service_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-7: empty `LoginSet` reference admits every service | |
| `TestLoginDecisionEmptySetRefuses` | `internal/component/aaa/login_service_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-11: an empty set (`[]string{}`) and an absent reference (`""`) are distinct | |
| `TestLoginDecisionUnreadableRefusesAsRejection` | `internal/component/aaa/login_service_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-6: nil generation or missing set name returns `ErrAuthRejected` | |
| `TestLoginDecisionEmptyOrUnregisteredServiceRefuses` | `internal/component/aaa/login_service_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | A request naming no service or an unregistered service is refused for any account with a set | |
| `TestLoginDecisionAdmitsRecoveryResult` | `internal/component/aaa/login_service_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-8: a local result carrying `ReservedRecoveryProfile` is admitted before reading any set; a remote result carrying that profile name is not | |
| `TestAAADoctorCheckAllAccountsExcluded` | `internal/component/aaa/doctor_test.go` | `doctor-aaa-all-accounts-excluded` warns with fix when every configured account is excluded from every door, and passes when at least one account can reach a door | |
| `TestDoctorAnswerReportsAllAccountsExcluded` | `internal/component/doctor/doctor_test.go` | User-entry-point doctor check (`ze doctor --json config <file>` / `doctor.Answer`) exposes `doctor-aaa-all-accounts-excluded` end to end (`docs/architecture/doctor-and-health-checks.md`, "The two tests a new check carries") | |
| `TestWrapperGateIsTerminalInChain` | `internal/component/aaa/login_profiles_test.go` | AC-15: after a gate refusal no later backend is called, and the refusal counter increments | |
| `TestTacacsProfileCarriesLoginSet` | `internal/component/tacacs/authenticator_test.go` | AC-12: `tacacs-profile[*].login-service` reaches `AuthResult.LoginSet` | |
| `TestRadiusConfigCarriesLoginSet` | `internal/component/radius/authenticator_test.go` | AC-12: `system/authentication/radius/login-service` reaches `AuthResult.LoginSet` | |
| `TestLiveBundleFallbackPassesDecision` | `cmd/ze/hub/aaa_authenticator_web_test.go` | AC-15: the web fallback's result is gated, and a gate refusal from the bundle opens the fallback for the local recovery result only | |
| `TestSSHPublicKeyLoginHonoursLoginSet` | `internal/component/ssh/pubkey_test.go` | AC-2: the gate is not password-specific | |
| `TestSSHLocalAuthenticatorWithoutBundleIsGated` | `internal/component/ssh/passwordauth_test.go` | AC-19: bare SSH local authenticator uses `WithProfileAuthorizer` | |
| `TestValidateTokenRefusesWithdrawnWebService` | `internal/component/web/auth_test.go` | AC-13: per-request re-check refuses local and remote sessions when `web` is removed from their set | |
| `TestAPIAuthenticationNamesTransport` | `cmd/ze/hub/api_infra_test.go` | AC-17: the closure gates REST and gRPC under `"rest"` and `"grpc"` | |
| `TestMCPStreamableLoginServiceGate` | `internal/component/mcp/streamable_auth_test.go` | AC-20: named `bearer-list`/`oauth` principals call `LoginServiceGate` under `"mcp"`, anonymous `none`/`bearer` skip it, refusal returns HTTP 401 `invalid_token` | |
| `TestSameLocalCredentialReadsEveryField` | `cmd/ze/hub/rfc8907_aaa_lifecycle_test.go` (existing) | Pinned field count moves from 5 to 6, and changing `LoginSet` advances `UserGeneration` | |
| `TestRegisteredNamesMatchWireValues` | `internal/component/ssh/passwordauth_test.go`, `internal/component/web/auth_test.go` | AC-18: the registered names are `"ssh"` and `"web"` | |
| `TestPowerUserIgnoresLoginSets` | `cmd/ze/hub/auth_e2e_test.go` | AC-8 through the hub's own assembly | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| services named in one login set | 0 to the registry size | 1, the smallest set that admits anything | N-A: 0 is valid config and admits nothing (AC-11) | N-A: naming every registered service admits every gated door |

### Functional Tests
Each surface file holds the negative and the positive case on one daemon.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `login-service-ssh-password` | `test/plugin/login-service-ssh-password.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2, AC-3, AC-4: a correct password is refused on SSH and the same account succeeds on REST | |
| `login-service-ssh-publickey` | `test/plugin/login-service-ssh-publickey.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2: the configured key is refused on SSH; with `ssh` in the set it is admitted | |
| `login-service-ze-cli` | `test/plugin/login-service-ze-cli.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2: `ze cli -c` as that account is refused; as an account whose set holds `ssh` it answers | |
| `login-service-web-form` | `test/plugin/login-service-web-form.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2, AC-3: the form answers as for a wrong password; an allowed account receives a session cookie | |
| `login-service-web-basic` | `test/plugin/login-service-web-basic.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2: Basic Auth is refused for the excluded account and served for the included one | |
| `login-service-web-cookie-reload` | `test/plugin/login-service-web-cookie-reload.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-13: a cookie issued before a reload removing `web` from the set is refused on the next request | |
| `login-service-rest` | `test/plugin/login-service-rest.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2, AC-3, AC-17: refused on REST, admitted on gRPC, no command effect | |
| `login-service-grpc` | `test/plugin/login-service-grpc.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2, AC-17: the reverse pair, unary and stream open | |
| `login-service-mcp` | `test/plugin/login-service-mcp.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-20: named MCP principal refused with 401 when `mcp` is excluded, admitted after reload adds `mcp`; anonymous `bearer` unaffected | |
| `login-service-tacacs` | `test/plugin/login-service-tacacs.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-12 (OD-1 option d): TACACS+ `tacacs-profile` `login-service` refuses excluded door terminally and admits included door | |
| `login-service-radius` | `test/plugin/login-service-radius.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-12 (OD-1 option d): RADIUS `login-service` refuses excluded door terminally and admits included door | |
| `login-service-reload-tightens` | `test/plugin/login-service-reload-tightens.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-5, AC-13: a reload closes a door for the next login and an open SSH session keeps answering | |
| `login-service-recovery-user` | `test/plugin/login-service-recovery-user.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-8: the stored recovery user logs in on every door while every config account is refused | |
| `login-service-shadowed-recovery-name` | `test/plugin/login-service-shadowed-recovery-name.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-14 (OD-2 option a): config user taking the recovery username is gated as an ordinary account | |
| `login-service-absent-section` | `test/plugin/login-service-absent-section.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-7 (OD-3 option a): a config with no `login-service` section admits its user on every door | |
| `login-service-empty-set` | `test/plugin/login-service-empty-set.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-11: an empty `login-service` set refuses every gated door | |
| `login-service-chain-terminal` | `test/plugin/login-service-chain-terminal.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-15, AC-6: with a remote backend configured and unreachable, the excluded local account is still refused on web | |
| `login-service-shared-token` | `test/plugin/login-service-shared-token.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-16: REST/gRPC shared-token mode stays admitted | |
| `login-service-unregistered-name` | `test/parse/login-service-unregistered-name.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-9: the refusal text names the bad value and the offered services | |
| `login-service-undefined-set` | `test/parse/login-service-undefined-set.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-10: the leafref refusal on `user`, `tacacs-profile`, and `radius` names the sets that exist | |

### Interop Tests (Scope: protocol)
No byte on a wire changes (see the census). Under OD-1 option (d), Ze locally
refuses a door after FreeRADIUS returns `Access-Accept`, which FreeRADIUS
observes as an accepted authentication with no session after it. The FreeRADIUS
interop scenario below verifies this against a real FreeRADIUS server after
reading FreeRADIUS's behavior (`ai/rules/interop-and-goal-validation.md`).

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `radius-admin-login-service-refused-freeradius` | `test/interop-radius/scenarios/` | FreeRADIUS | OD-1 option (d): a FreeRADIUS-accepted admin is refused on the door `system/authentication/radius/login-service` excludes and admitted on a door it holds, while `NAS-Port-Id` remains unchanged | |

## Files to Modify
- `internal/component/authz/yang/ze-authz-conf.yang` - add `list login-service` under `system/authorization` (`leaf name`, `leaf-list service` with `ze:validate "login-service"`) and `leaf login-service` (leafref with `require-instance true`) under `system/authentication/user`
- `internal/component/authz/auth.go` - copy `u.LoginSet` onto `AuthResult.LoginSet` in `(*LocalAuthenticator).Authenticate`
- `internal/component/tacacs/yang/ze-tacacs-conf.yang` - add optional `leaf login-service` (leafref to `/authz:system/authz:authorization/authz:login-service/authz:name` with `require-instance true`) under `system/authentication/tacacs-profile` (OD-1 option d)
- `internal/component/tacacs/config.go` - extract `PrivLvlLoginSet` from `system/authentication/tacacs-profile[*]/login-service`
- `internal/component/tacacs/authenticator.go` - carry `privLvlLoginSet` and set `AuthResult.LoginSet` in `handlePass`
- `internal/component/tacacs/register.go` - pass `cfg.PrivLvlLoginSet` to `newTacacsAuthenticator`
- `internal/component/radius/yang/ze-radius-conf.yang` - import `ze-authz-conf` and add optional `leaf login-service` (leafref to `/authz:system/authz:authorization/authz:login-service/authz:name` with `require-instance true`) under `system/authentication/radius` (OD-1 option d), and regenerate `internal/component/radius/yang/register.go` via `./le yang glue update`
- `internal/component/radius/config.go` - extract `LoginSet` from `system/authentication/radius/login-service`
- `internal/component/radius/authenticator.go` - store `loginSet` and set `AuthResult.LoginSet` in `result`
- `internal/component/config/infra/authz.go` - carry `LoginSet` in `ExtractAuthUsers`; add `ExtractLoginServices` beside `ExtractAuthzStore`; refuse reserved names on `login-service` definitions and on `user`, `tacacs-profile`, and `radius` references in `ValidateAuthzConfig`
- `internal/component/config/validators.go` - add `LoginServiceValidator`, copying `RedistributeSourceValidator` and listing offered services on refusal as `RegisteredProtocolValidator` does
- `internal/component/config/validators_register.go` - wire `ze:validate "login-service"` in `RegisterValidators`
- `internal/component/aaa/types.go` - add `LoginSet string` to `UserCredential` and `AuthResult`
- `internal/component/aaa/login_profiles.go` - add `SetAcceptedLoginSets`, `CheckLoginService`, `CheckLoginSet`, the refusal Prometheus counter hook, and the call in `resultAuthorizingAuthenticator.Authenticate`
- `internal/component/aaa/doctor.go` - add post-config doctor check `doctor-aaa-all-accounts-excluded` reporting when every configured account is excluded from every registered door, with its fix command
- `internal/component/aaa/register.go` - register `aaaAllAccountsExcludedDoctorCheck` via `diagnostic.RegisterDoctorCheck`
- `internal/component/aaa/register_doctor_codes.go` - register `doctor-aaa-all-accounts-excluded` `CodeMeta` via `diagnostic.Register`
- `internal/component/ssh/passwordauth.go` - name the service from the `"ssh"` registration constant
- `internal/component/ssh/pubkey.go` - call `aaa.CheckLoginService` after `matchPublicKey` succeeds
- `internal/component/ssh/ssh.go` - wrap the bare local authenticator with `aaa.WithProfileAuthorizer`
- `internal/component/web/auth.go` - carry `LoginSet` on `webSession`; re-check login-service permission for `"web"` in `(*SessionStore).validateToken`; name the service from the `"web"` registration constant
- `internal/component/api/types.go` - `Authentication.Authenticate` takes `(authHeader, service string)`
- `cmd/ze/hub/api.go` - `buildAPIAuthentication` passes `service` into `AuthRequest.Service`
- `internal/component/api/rest/auth.go` - pass the registered `"rest"` service name to `authentication.Authenticate`
- `internal/component/api/grpc/server.go` - pass the registered `"grpc"` service name to `authentication.Authenticate`
- `internal/component/mcp/streamable.go` - add `LoginServiceGate func(username, service string) error` to `StreamableConfig` and enforce it in `(*Streamable).authenticate` for non-anonymous `Identity` (`bearer-list`, `oauth`) under `"mcp"` (OD-4 option b)
- `cmd/ze/hub/service_mcp.go` - pass `deps.MCP.LoginServiceGate` into `StreamableConfig` (behind `ze_mcp`)
- `cmd/ze/hub/service_registry.go` - add `LoginServiceGate` to `mcpServiceDeps`
- `cmd/ze/hub/main.go` - attach extracted login sets to boot `acceptedLocalIdentityState` and wire `checkAcceptedLocalLoginService` into `deps.MCP.LoginServiceGate`
- `cmd/ze/hub/main_reload.go` - attach extracted login sets to reload candidate `acceptedLocalIdentityState`
- `cmd/ze/hub/aaa_lifecycle.go` - add `loginSets map[string][]string` to `acceptedLocalIdentityState` (with `withLoginSets` helper so existing test builders remain intact); publish `aaa.SetAcceptedLoginSets` in `publishAcceptedLocalIdentity` (and `nil` in `closeAAABundle`); compare `LoginSet` in `sameLocalCredential`; gate the fallback result in `liveAAABundleAuthenticator.Authenticate` and treat a gate refusal from the bundle as terminal; provide `checkAcceptedLocalLoginService` for MCP
- `cmd/ze/hub/rfc8907_aaa_lifecycle_test.go` - update `TestSameLocalCredentialReadsEveryField` to pin the new field count (6) and assert `LoginSet` changes advance `UserGeneration`
- `docs/guide/authentication.md` - who may use which door (including MCP and remote backends); `ze cli` is an SSH login; what an open session keeps
- `docs/guide/authorization.md` - the new `login-service` section beside `profile`
- `docs/guide/configuration.md` - the new config section
- `docs/guide/operator-access-rbac.md` - section 8 gains the withdrawn-door case
- `docs/guide/tacacs.md`, `docs/guide/radius.md` - state that the access-channel value is the registered login-service name, and document the optional `login-service` leaf on `tacacs-profile` and `radius` (OD-1 option d)
- `docs/architecture/api/architecture.md` - "Identity and Authorization": the generation carries login sets, and `api.Authentication.Authenticate` names its transport (`rest`/`grpc`)
- `docs/architecture/web-interface.md` - "Authentication": `validateToken` re-checks the `web` login-service gate for local and remote sessions
- `docs/architecture/core-design.md` - section 19: allowed-import rows for `config/loginservice` (`aaa`, `api`, `mcp`, `ssh`, `web`) and login-service sets in the accepted identity generation
- `docs/architecture/aaa-tacacs.md` - document the optional `login-service` leaf on `tacacs-profile` and terminal `ErrAuthRejected` gate behavior
- `docs/architecture/mcp/overview.md` - "Authentication": document the `"mcp"` login-service gate for named `bearer-list` and `oauth` principals (OD-4 option b)
- `ai/patterns/registration.md` - add `internal/component/config/loginservice` to the registration inventory
- `rfc/short/rfc8907.md` - a row for the Section 5.1 sentence on documenting the `port` values, pointing at `docs/guide/tacacs.md`

## Files to Create
- `internal/component/config/loginservice/registry.go` - the login-service name registry, copying `internal/component/config/redistribute/registry.go`
- `internal/component/config/loginservice/registry_test.go`
- `internal/component/aaa/login_service_test.go`
- `internal/component/ssh/register_login_service.go` - registers `"ssh"` at `init()` (named `register_login_service.go` per `./le go style check` `init()` rule)
- `internal/component/web/register_login_service.go` - registers `"web"` at `init()`
- `internal/component/api/rest/register_login_service.go` - registers `"rest"` at `init()`
- `internal/component/api/grpc/register_login_service.go` - registers `"grpc"` at `init()`
- `internal/component/mcp/register_login_service.go` - registers `"mcp"` at `init()`
- the twenty `.ci` files named in Functional Tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `list login-service` and `user/login-service` in `internal/component/authz/yang/ze-authz-conf.yang`; optional `login-service` leafref in `internal/component/tacacs/yang/ze-tacacs-conf.yang` and `internal/component/radius/yang/ze-radius-conf.yang` (OD-1 option d). `ze-ssh-conf.yang` and `ze-mcp-conf.yang` are untouched |
| YANG validation constraints | Yes | Each `login-service` user/mapping reference is a `leafref` with `require-instance true`. The `service` leaf-list is `type string` with `ze:validate "login-service"`; the live registry holds the value set |
| YANG custom validators | Yes | `LoginServiceValidator` in `internal/component/config/validators.go`, wired by name in `RegisterValidators` (`validators_register.go`), with `CompleteFn` |
| CLI commands/flags | No | No new verb. The config editor reaches the new section through the YANG schema |
| CLI grammar (keyword before value) | No | No new command |
| Editor autocomplete | Yes | Automatic once `LoginServiceValidator` supplies `CompleteFn` (`loginservice.Names`), and leafref completion automatically offers defined `login-service` set names |
| Functional test for new RPC/API | Yes | The twenty `.ci` files in Functional Tests |
| Pipe completeness | No | No new command output |
| Env var registration | No | No `environment/` leaf; the section lives under `system` |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module, or binary |
| Doctor code per problem class | Yes | `doctor-aaa-all-accounts-excluded` in `internal/component/aaa/doctor.go` (`DoctorPhasePostConfig`, order 735), check registered in `internal/component/aaa/register.go`, `CodeMeta` registered via `diagnostic.Register` in `internal/component/aaa/register_doctor_codes.go`, warning with its fix command when every configured account is excluded from every registered door |
| Prometheus counters/metrics | Yes | `ze_aaa_login_service_refused_total{service}` counter incremented on every login-service refusal so operators can observe misconfigured sets directly |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The feature declaration under `features/` that renders `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/guide/authorization.md` |
| 3 | CLI command added/changed? | No | No verb changes |
| 4 | API/RPC added/changed? | No | The REST, gRPC, and MCP RPC surfaces are unchanged; only authentication admission per surface changes |
| 5 | Plugin added/changed? | No | No plugin is added |
| 6 | Has a user guide page? | Yes | `docs/guide/authentication.md`, `docs/guide/operator-access-rbac.md`, `docs/guide/tacacs.md`, `docs/guide/radius.md` |
| 7 | Wire format changed? | No | See the census: TACACS+ `port` and RADIUS `NAS-Port-Id` values (`ssh`, `web`) are unchanged |
| 8 | Plugin SDK/protocol changed? | No | The registry lives in the daemon; no SDK contract is added for an external plugin to offer a login (Known Limitations) |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8907.md`: Section 5.1 sentence on documenting `port` values, met by `docs/guide/tacacs.md` |
| 10 | Test infrastructure changed? | No | Uses existing `test/plugin`, `test/parse`, and `test/interop-radius` suites |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it claims parity on per-user access control |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md`, `docs/architecture/web-interface.md`, `docs/architecture/core-design.md`, `docs/architecture/aaa-tacacs.md`, `docs/architecture/mcp/overview.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` / `docs/guide/monitoring.md` for `ze_aaa_login_service_refused_total` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/patterns/registration.md` (add `loginservice` to the registration inventory) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Architecture pages in `Files to Modify` (`core-design.md`, `aaa-tacacs.md`, `api/architecture.md`, `web-interface.md`, `mcp/overview.md`, `ai/patterns/registration.md`) are updated. Remaining DECLARED `// Design:` pages are unaffected: `docs/architecture/api/commands.md` (command dispatch/pipes unchanged; gate runs at authentication), `docs/architecture/hub-architecture.md` (external-process plugin model unchanged), `docs/architecture/system-architecture.md` (does not document SSH pubkey/password admission), `docs/research/l2tpv2-ze-integration.md` (L2TPv2 subscriber RADIUS unchanged; only operator admin RADIUS `login-service` is added), `docs/architecture/doctor-and-health-checks.md` (new check follows existing `diagnostic.RegisterDoctorCheck` + `CodeMeta` pattern without changing doctor mechanics), `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` (local-only bcrypt hash restriction unchanged), `docs/architecture/config/syntax.md` (uses existing `ze:validate` extension), `docs/architecture/config/yang-config-design.md` (uses existing Layer-1 `leafref` and Layer-2 `ze:validate` under already-walked `system`), `ai/rules/plugins.md` (no plugin rule changed) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/authentication.md`, `docs/guide/authorization.md`, `docs/guide/tacacs.md`, `docs/guide/radius.md` show user and AAA config examples updated for `login-service` |

## Design presentation

### What changes

Operators can define named `login-service` sets under `system authorization` listing which login surfaces (`ssh`, `web`, `rest`, `grpc`, `mcp`) an account may authenticate into, and reference a set from any local user (`system authentication user`), TACACS+ privilege-level mapping (`system authentication tacacs-profile`), or RADIUS admin config (`system authentication radius`). Each login surface registers its own name at `init()` in its own package (`internal/component/config/loginservice`), so a build without a surface does not register it and config validation rejects unknown or compiled-out names while listing the names the binary offers. When an account presents a valid credential on an excluded surface, authentication fails terminally with the surface's normal wrong-credential response before command authorisation is ever reached, while an unconfigured `login-service` leaf continues to allow every surface and the unshadowed stored recovery user stays exempt.

### Before and after

1. **Restricting a local user to REST and gRPC only (excluding SSH and Web):**
   - Before: Any valid password or SSH key in `system authentication user` works on SSH, `ze cli`, Web form, Web Basic Auth, REST, and gRPC alike; there is no way to separate an automation account from an interactive shell account.
   - After: Defining `system authorization login-service api-only` with `service [ rest grpc ]` and assigning `login-service api-only` to `user monitor` admits `monitor` on REST and gRPC and refuses `monitor` on SSH (password and public key), `ze cli`, and Web (form and Basic Auth) with the same unauthorised response as a wrong password, recording one failed-login audit entry and incrementing `ze_aaa_login_service_refused_total`.
2. **Restricting remote TACACS+ and RADIUS operators by door (OD-1 option d):**
   - Before: A TACACS+ or RADIUS operator authenticated without a local user entry can log in on every surface that consults the AAA chain (`ssh` and `web`).
   - After: Setting `login-service ssh-only` on `system authentication tacacs-profile 15` or `system authentication radius` restricts those remote logins to `ssh` and terminally rejects `web` (`ErrAuthRejected`, never falling through to local).
3. **Restricting a named MCP principal (OD-4 option b):**
   - Before: A named MCP `bearer-list` or `oauth` principal cannot be restricted by `system authorization login-service`.
   - After: `mcp` registers its service name at `init()`; if a named MCP principal matches a `system authentication user` whose `login-service` set excludes `mcp`, every MCP POST is refused with HTTP 401 `Bearer error="invalid_token"` until a reload adds `mcp` back.
4. **Tightening a login set on a live router via reload:**
   - Before: No login-service set exists to reload.
   - After: Removing `web` from a set and reloading immediately rejects the next web request carrying an existing `ze-session` cookie with HTTP 401, while already-open SSH sessions, gRPC streams, and web SSE streams survive until they close.
5. **Typing an unknown or compiled-out service name in config:**
   - Before: No `login-service` section exists.
   - After: `ze config validate`, commit, and load reject the value naming the offending service, stating that this build does not offer it, and listing the exact registered services this binary carries (`ssh`, `web`, `rest`, `grpc`, `mcp`).

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Add a `login-service` field onto `system authorization profile` | Owner ruling (2026-08-11): profiles govern commands and login sets govern doors. Separating them lets two accounts share one command profile while differing on which doors they may use |
| Enforce the door check inside `(*Store).Authorize` after login | Owner ruling (2026-08-11): the credential must fail at authentication. Admitting a session that denies every command still holds session state, logs a successful login, and leaks account existence |
| Spell the service names in a YANG `enumeration` | Owner ruling (2026-08-11) and registration principle: a schema enum forces a central schema edit for every new surface and accepts compiled-out surfaces instead of reflecting the live binary's registered surfaces |
| Restrict local accounts only and leave RADIUS/TACACS+ ungated | Set aside by Thomas on 2026-11-24 (OD-1 option d): remote accounts must be restrictable via a `login-service` leaf beside their profile mapping |
| Leave MCP outside login-service authorization | Set aside by Thomas on 2026-11-24 (OD-4 option b): MCP must be restrictable alongside SSH, Web, REST, gRPC, Looking Glass, and gNMI |
| Default an absent `login-service` reference to deny-all | Set aside by Thomas on 2026-11-24 (OD-3 option a): deny-by-default would break every existing deployment on upgrade |
| Exempt a shadowing config user by matching the stored recovery username | Set aside by Thomas on 2026-11-24 (OD-2 option a): keeps the documented `ReservedRecoveryProfile` rule so privilege follows credential source, not username |
| Read login sets from `ConfigProvider` at login time | `ConfigProvider` holds a rejectable candidate during reload; reading it would enforce unaccepted candidates. Publishing login sets inside `acceptedLocalIdentityState` keeps users, policy, and login sets atomic |

### Documents departed from

none

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| OD-1 | How login sets apply to remote RADIUS / TACACS+ users without a local user entry | (a) local accounts only; (b) match local record if present; (c) match local record and refuse unmatched; (d) add a `login-service` leaf alongside the profile mapping in RADIUS and TACACS+ config | Option (d) — approved by Thomas on 2026-11-24 |
| OD-2 | Config user shadowing the stored appliance recovery username | (a) keep `ReservedRecoveryProfile` rule (config user is ordinary, login set applies, serial console still works); (b) exempt by stored username; (c) refuse at commit | Option (a) — approved by Thomas on 2026-11-24 |
| OD-3 | Account with no `login-service` configured | (a) absent means allowed on all login services; (b) absent means allowed on none | Option (a) — approved by Thomas on 2026-11-24 |
| OD-4 | Whether MCP is included in login-service authorization | (a) outside; (b) include MCP alongside SSH, Web, REST, gRPC, Looking Glass, and gNMI | Option (b) — approved by Thomas on 2026-11-24 |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove a login reaches a decision point that can refuse it on every door
   - Tests: the twelve wiring `.ci` files from the Wiring Test table, written to fail
   - Files: those `.ci` files
   - Verify: each fails because the login SUCCEEDS today (or the unregistered name is accepted), not on a setup error
2. **Phase: The registry** -- names register from each surface's own package, and nothing central lists them
   - Tests: `TestLoginServiceRegistryRejectsConflictingRegistration`, `TestRegisteredNamesMatchWireValues`
   - Files: `internal/component/config/loginservice/registry.go`, `internal/component/ssh/register_login_service.go`, `internal/component/web/register_login_service.go`, `internal/component/api/rest/register_login_service.go`, `internal/component/api/grpc/register_login_service.go`, `internal/component/mcp/register_login_service.go`
   - Verify: AC-1, AC-18. No file outside the registry enumerates the service names
3. **Phase: Config surface (local, TACACS+, RADIUS)** -- the section, the leafrefs, and their validation
   - Tests: `TestLoginServiceValidatorRejectsUnregisteredName`, `TestExtractAuthUsersAndLoginServices`, `TestTacacsProfileCarriesLoginSet`, `TestRadiusConfigCarriesLoginSet`, the two `test/parse` `.ci` files
   - Files: `ze-authz-conf.yang`, `ze-tacacs-conf.yang`, `ze-radius-conf.yang` (plus `./le yang glue update`), `validators.go`, `validators_register.go`, `config/infra/authz.go`, `tacacs/config.go`, `radius/config.go`
   - Verify: AC-9, AC-10, A-8, and CLI completion offers registered names only
4. **Phase: The generation carries the sets** -- one producer, one publication
   - Tests: `TestSameLocalCredentialReadsEveryField`
   - Files: `internal/component/aaa/types.go`, `cmd/ze/hub/aaa_lifecycle.go`, `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go`
   - Verify: a changed `LoginSet` advances `UserGeneration`; a rejected reload publishes nothing
5. **Phase: The decision** -- one fail-closed function in `aaa`
   - Tests: the five `TestLoginDecision...` unit tests
   - Files: `internal/component/aaa/login_profiles.go`
   - Verify: AC-6, AC-7, AC-8, AC-11. Every refusal returns `ErrAuthRejected`
6. **Phase: The gate, on all six paths at once** -- never one path at a time
   - Tests: the surface `.ci` files, `TestWrapperGateIsTerminalInChain`, `TestLiveBundleFallbackPassesDecision`, `TestSSHPublicKeyLoginHonoursLoginSet`, `TestSSHLocalAuthenticatorWithoutBundleIsGated`, `TestAPIAuthenticationNamesTransport`, `TestMCPStreamableLoginServiceGate`
   - Files: `authz/auth.go`, `tacacs/authenticator.go`, `tacacs/register.go`, `radius/authenticator.go`, `aaa/login_profiles.go`, `ssh/passwordauth.go`, `ssh/pubkey.go`, `ssh/ssh.go`, `api/types.go`, `cmd/ze/hub/api.go`, `api/rest/auth.go`, `api/grpc/server.go`, `mcp/streamable.go`, `cmd/ze/hub/service_mcp.go`, `cmd/ze/hub/service_registry.go`, `cmd/ze/hub/aaa_lifecycle.go`
   - Verify: AC-2, AC-3, AC-4, AC-12, AC-15, AC-16, AC-17, AC-19, AC-20
7. **Phase: Reload, open sessions, and recovery**
   - Tests: `login-service-reload-tightens`, `login-service-web-cookie-reload`, `TestValidateTokenRefusesWithdrawnWebService`, `login-service-recovery-user`, `login-service-shadowed-recovery-name`, `TestPowerUserIgnoresLoginSets`
   - Files: `internal/component/web/auth.go`
   - Verify: AC-5, AC-8, AC-13, AC-14
8. **Phase: Docs, metric, doctor check, FreeRADIUS interop, and discrimination**
   - Tests: `TestAAADoctorCheckAllAccountsExcluded`, `TestDoctorAnswerReportsAllAccountsExcluded`, `radius-admin-login-service-refused-freeradius`, and every new `.ci` run once with the gate reverted by the main thread
   - Files: `internal/component/aaa/doctor.go`, `internal/component/aaa/register.go`, `internal/component/aaa/register_doctor_codes.go`, the documentation set, `rfc/short/rfc8907.md`
   - Verify: reverting the gate turns every new `.ci` red

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every gated door in "Login surfaces" (SSH password, SSH pubkey, `ze cli`, Web form, Web Basic, Web cookie, REST, gRPC, MCP, plus TACACS+ and RADIUS) is proven separately |
| Feature completeness | Boot AND reload. Editing either the `login-service` set definition or the account's reference takes effect on reload without restart |
| Correctness | A refused login is indistinguishable from a wrong credential to the caller, and distinguishable in the log and metric to the operator |
| Naming | The section names LOGIN (`login-service`), not command permission. It decides which door, never which command |
| Data flow | One credential producer, one registry, one decision function, one published generation. No central list of service names anywhere |
| Operator errors and doctor codes | `LoginServiceValidator` names the value, the names offered, and the fix. `doctor-aaa-all-accounts-excluded` carries its fix command and registers via `diagnostic.Register` in `internal/component/aaa` |
| Rule: `ai/rules/evidence.md` | An unreadable login set denies (`ErrAuthRejected`). An empty set from a failed read never reaches the matcher as permissive |
| Rule: `ai/rules/simplicity.md` | The registry and validator copy existing shapes (`redistribute`); MCP reuses its existing username link to `system/authentication/user` with no new MCP YANG leaves |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No central list of service names | Grep the tree for any file enumerating the names outside `loginservice` and its registrants |
| Every gated door refuses and admits | All twenty `.ci` files pass, and each fails with the gate reverted |
| Remote backends honor `login-service` (OD-1 option d) | `login-service-tacacs.ci`, `login-service-radius.ci`, and `radius-admin-login-service-refused-freeradius` |
| MCP honors `login-service` (OD-4 option b) | `login-service-mcp.ci` and `TestMCPStreamableLoginServiceGate` |
| The recovery path survives | `login-service-recovery-user.ci`, on a running daemon |
| An untouched config still works | `login-service-absent-section.ci` |
| The wire values did not move | `TestRegisteredNamesMatchWireValues` |
| Lint and architecture checks | `./le go lint run scope "<pkg>"`, `./le arch tier check`, `./le yang glue check`, `./le doc check all`, `./le cli grammar` (run by main thread) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| What a wrong landing exposes | Permissive: an account reaches a door it should not. Strict: an operator is locked out of every surface with no remote way back. Design against the second |
| What proves it did not | Each `.ci` asserts BOTH halves on one daemon: refused where excluded, admitted where included |
| Fail closed | An unresolvable login set or nil accepted generation denies with `ErrAuthRejected`. Trace every error return of `CheckLoginService` and `CheckLoginSet` |
| No fall-through | A gate refusal reaches neither the next backend in `ChainAuthenticator` nor the web fallback in `liveAAABundleAuthenticator` |
| Every path | SSH public key, private API authenticator, web fallback, bare SSH local authenticator, TACACS+, RADIUS, and MCP named principals are all gated |
| Empty is not "no restriction" | A user naming an EMPTY set (`[]string{}`) is denied everywhere (AC-11). A user naming NO set (`""`) is admitted everywhere (AC-7). These never collapse |
| Guard driven from its entry point | Real logins on a running daemon, never unit tests of the matcher alone |
| Privilege | The exemption is keyed on `ReservedRecoveryProfile` from `SourceLocal` only. No config user or remote backend can carry it into the exemption |
| No user enumeration | Refused login returns the same status, body, and timing shape as a wrong credential (decision runs after credential verification) |
| Accounting | A refused login is recorded as `audit.ActionAuthFail` and increments `ze_aaa_login_service_refused_total`, never a successful login |
| Stored credential | A web cookie issued before a withdrawal is refused on its next request, whether the withdrawal edited the user's leaf or the set's service list |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A new `.ci` passes with the gate reverted | The test is vacuous. Assert the credential decision, not the response code alone |
| The service name cannot reach the API closure without a transport type crossing into `aaa` | STOP. That is A-2 broken. Report the signature you reached and ask which way |
| Another authentication path is found | STOP adding gates. Add the path to Current Behavior and to "Login surfaces", then gate it |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The mechanism the owner asked for exists. Route redistribution registers source names at `init()`, tags the leaf `ze:validate "redistribute-source"`, and lets one validator answer both "is this real" and "what may I offer in completion".
- The hard half is not the registry; it is that six authentication paths exist across Ze's doors, and four of them never pass through the live AAA chain wrapper.
- The service name was already on the wire for TACACS+ (`port`) and RADIUS (`NAS-Port-Id`), making `"ssh"` and `"web"` wire-preserving values.
- Carrying `LoginSet` on `AuthResult` lets local users, TACACS+ `tacacs-profile` mappings, and RADIUS admin logins all pass through the exact same fail-closed decision in `WithProfileAuthorizer`.
- MCP already links named `bearer-list` and `oauth` principals to `system/authentication/user` via `CallerIdentity.Username` -> `authz.Store`; using that same link for `LoginServiceGate` delivers live-reloadable MCP login-service enforcement with zero new MCP YANG leaves and zero restart requirement.
- A refusal has to be terminal (`ErrAuthRejected`), because `ChainAuthenticator` and the web fallback treat any other error as an unreachable backend and try the next one.

## Owned journal rows
No open journal row describes a defect this spec fixes. `plan/journal/` was
searched for the functions this spec edits; the one row found on
`liveAAABundleAuthenticator.Authenticate` (`plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-10-31) is keyed to the spec that found it.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A separate named `login-service` section under `system/authorization`, not a field on `profile` | Add a service list to `profile` | Owner instruction (2026-08-11): profiles govern commands and login sets govern doors; separating them lets accounts share command profiles while differing on doors |
| Refuse at authentication, not at command authorisation | Admit the login and deny every command | Owner instruction (2026-08-11): the credential must not work on an excluded surface, avoiding open sessions, false login accounting, and user enumeration |
| Names come from a registry (`internal/component/config/loginservice`), never from a YANG enum | YANG `enumeration` listing the services | Owner instruction (2026-08-11) and registration principle: surfaces self-register at `init()` and compiled-out surfaces vanish from validation and completion |
| Add optional `login-service` leafref to `tacacs-profile` and `radius`, carried on `AuthResult.LoginSet` | Local-only gate, or dummy local user records for remote accounts | Owner ruling OD-1 option (d) (2026-11-24): restricts remote accounts cleanly through the same `CheckLoginService` decision without requiring local shadow accounts |
| Gate named MCP principals (`bearer-list`, `oauth`) via hub-wired `LoginServiceGate` over `acceptedLocalIdentityState` | Leave MCP outside, or add restart-only `login-service` leaves to `ze-mcp-conf.yang` | Owner ruling OD-4 option (b) (2026-11-24): matches MCP's existing `Identity.Name` → `authz.Store` link, keeps `ze_mcp` compile-out clean, and applies reloads live without restarting MCP |
| A user or remote mapping naming no set reaches every service | Deny by default | Owner ruling OD-3 option (a) (2026-11-24): preserves every existing config on upgrade |
| One decision function (`aaa.CheckLoginService` / `CheckLoginSet`), called on every path | Per-surface custom checks | Six paths exist; centralising the fail-closed check prevents drift between doors |
| Every gate refusal is `ErrAuthRejected` | Return underlying read error | Any other error falls through `ChainAuthenticator` or opens the web fallback |
| Registered names are `ssh`, `web`, `rest`, `grpc`, `mcp` | New names | `ssh` and `web` are already on TACACS+/RADIUS wires; `rest`, `grpc`, and `mcp` match Ze's surface names |
| The user and remote-mapping references are YANG leafrefs (`require-instance true`) | String compare in `ValidateAuthzConfig` | Matches `profile` leafrefs and lists existing sets on commit refusal |
| Login sets live in `acceptedLocalIdentityState` and publish via `aaa.SetAcceptedLoginSets` | Read `ConfigProvider` at login | `ConfigProvider` holds rejectable reload candidates; atomic generation publication guarantees consistency |
| An open SSH/gRPC/SSE connection outlives a withdrawn door; a web cookie does not | Drop all live connections on reload | Matches `docs/guide/operator-access-rbac.md` section 8 and `docs/architecture/web-interface.md` |
| Exemption keyed on `SourceLocal` + `ReservedRecoveryProfile` | Exempt by stored username | Owner ruling OD-2 option (a) (2026-11-24): privilege follows credential source, not username |
| `ze cli` is gated as `ssh` | Separate service name | `ze cli` is an SSH login over loopback; the daemon cannot distinguish it from another loopback SSH client |
| Serial console is not gated | Read config from `loginMain` | Runs with no daemon and must work when config is broken |

## Known Limitations
- The looking glass and gNMI are not covered here because neither has per-user identity yet; both are owned by `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md`, which registers `"looking-glass"` and `"gnmi"` with this registry.
- REST and gRPC authenticate against local users only (`buildAPIAuthentication`, `cmd/ze/hub/api.go`). Extending REST/gRPC to TACACS+/RADIUS is not in scope here.
- MCP `auth-mode none` and `auth-mode bearer` (single shared token) carry no username and cannot be gated per account; only named `bearer-list` and `oauth` principals are gated.
- An external plugin cannot offer a gated login: the registry lives in the daemon process and `pkg/plugin/sdk/sdk_identity.go` carries no login-offer contract across the plugin boundary.
- A build without a surface's tag (`ze_web`, `ze_mcp`, etc.) refuses a config naming that surface's service (AC-9). A config shared across builds names only services both builds carry.
- Three other service-name lists exist (`internal/core/audit/audit.go`, `cmd/ze/hub/service_registry.go`, `internal/component/config/listener.go`). This registry intentionally holds LOGIN services only; reconciling the other three is out of scope.

## RFC Documentation (Scope: protocol)

The code that sends the service name to a TACACS+ server carries, above the
statement, RFC 8907 Section 5.1: "The client documentation SHOULD define the
values and their meanings for this field." The comment names
`docs/guide/tacacs.md` as that documentation.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 8907 Section 5.1 (`port` field) | TACACS+ START carries `"ssh"` or `"web"` (default `"ssh"` when unnamed), documented in `docs/guide/tacacs.md` | Local `tacacs-profile[*].login-service` gate runs after server reply with zero wire change | `TestRegisteredNamesMatchWireValues` | `login-service-tacacs.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| RFC 2865 / RFC 2869 Section 5.17 (`NAS-Port-Id`) | RADIUS Access-Request carries `"ssh"` or `"web"` in `NAS-Port-Id` (omitted when empty), documented in `docs/guide/radius.md` | Local `radius.login-service` gate runs after Access-Accept with zero wire change | `TestRegisteredNamesMatchWireValues` | `login-service-radius.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->, `radius-admin-login-service-refused-freeradius` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-20 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests of `aaa`, `authz`, `tacacs`, `radius`, `ssh`, `web`, `api`, `mcp`, `config`, and `cmd/ze/hub`, and the named `.ci` files through `./le test functional plugin` and `./le test functional parse` plus the FreeRADIUS interop scenario. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated on every gated door in "Login surfaces", not test-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-user behavior
- [ ] Interop test `radius-admin-login-service-refused-freeradius` passes against FreeRADIUS

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-login-service-authorisation.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- **Stdlib-only login-service registry (`internal/component/config/loginservice/registry.go`):** `Register`, `MustRegister`, `ErrConflict`, `Names`, `Registered`, `Lookup`, `Snapshot`, `Restore`. Five surfaces self-register at `init()` in `register_login_service.go` (`internal/component/{ssh,web,api/rest,api/grpc,mcp}/register_login_service.go`) with zero central enumeration.
- **YANG schema & validators:** Added `system/authorization/login-service` list (`service` with `ze:validate "login-service"`) and `require-instance true` `login-service` leafrefs under `system/authentication/user` (`ze-authz-conf.yang`), `system/authentication/tacacs-profile` (`ze-tacacs-conf.yang`), and `system/authentication/radius` (`ze-radius-conf.yang`, regenerated glue via `./le yang glue update`). Added `LoginServiceValidator` (`internal/component/config/validators.go`, wired in `validators_register.go`) and `ExtractLoginServices` / `LoginSet` extraction + reserved-name checks (`internal/component/config/infra/authz.go`).
- **Fail-closed AAA gate, metric & doctor check:** Added `LoginSet` to `aaa.UserCredential` and `aaa.AuthResult` (`aaa/types.go`), immutable `SetAcceptedLoginSets`, fail-closed `CheckLoginService` / `CheckLoginSet` returning terminal `ErrAuthRejected`, structured `Info` refusal log, Prometheus counter `ze_aaa_login_service_refused_total{service}` (`aaa/login_profiles.go`), and post-config `doctor-aaa-all-accounts-excluded` check (`aaa/doctor.go`, `aaa/register.go`, `aaa/register_doctor_codes.go`). Wired `AuthResult.LoginSet` in `authz/auth.go`, `tacacs/{config,authenticator,register}.go`, and `radius/{config,authenticator}.go`.
- **All six authentication paths & hub lifecycle:** Gated SSH password + pubkey + bare local authenticator (`ssh/{passwordauth,pubkey,ssh}.go`), Web form + Basic + per-request `(*SessionStore).validateToken` re-check for local and remote sessions (`web/auth.go`), REST (`"rest"`) & gRPC (`"grpc"`) via `api.Authentication.Authenticate(authHeader, service string)` (`api/types.go`, `rest/auth.go`, `grpc/server.go`, `cmd/ze/hub/api.go`), MCP named principals (`bearer-list`/`oauth`) via `StreamableConfig.LoginServiceGate` (`mcp/streamable.go`, `cmd/ze/hub/{service_registry,service_mcp,aaa_lifecycle,main}.go`), atomic generation publication of `loginSets` + `sameLocalCredential` 6-field pinning + web fallback gating (`cmd/ze/hub/{aaa_lifecycle,main,main_reload}.go`).
- **Tests & Docs:** Added all 22 spec unit tests, 20 `.ci` functional tests (`2` parse + `18` plugin with compiled Go fixture drivers in `internal/test/fixture/`), 1 FreeRADIUS interop scenario (`radius-admin-login-service-refused-freeradius`), 17 updated docs/architecture/RFC/feature files, `features/login-service-authorization.md`, and `website/changes/fragments/login-service-authorisation.md`.

### Bugs Found/Fixed
- `internal/component/doctor/doctor_test.go`: `TestDoctorAnswerReportsAllAccountsExcluded` initially invoked `Answer(...)` directly (daemon route) instead of `command.AnswerLocal(...)` (local CLI route); fixed and verified green.
- `internal/test/fixture/login_service_doors.go`: `login-service-grpc` initially passed unary `"show version"` to gRPC `Stream` after the gate admitted `grpconly`; switched to streaming `"monitor event"` and verified 5/5 stress pass.
- `test/interop-radius/Dockerfile.ze` & `internal/le/interoplab/radius/{radius.go,check_login_service.go}`: initialized `/var/lib/ze-web-store` for the web HTTPS listener's self-signed TLS cert (mounting `ze.conf` at `/var/lib/ze-web-store/ze.conf` because `StoreDir(configPath)` outranks `ZE_CONFIG_DIR`) and raised curl `--max-time` from 5s to 15s (exceeding the 6s silent-RADIUS timeout for `localop`); verified `radius-admin-login-service-refused-freeradius` PASS against real FreeRADIUS.

### Documentation Updates
- Updated guides: `docs/guide/{authentication,authorization,configuration,operator-access-rbac,tacacs,radius,monitoring}.md`, `docs/plugin-development/metrics.md`.
- Updated architecture & pattern docs: `docs/architecture/{api/architecture,web-interface,core-design,aaa-tacacs,mcp/overview}.md`, `ai/patterns/registration.md`.
- Updated RFC & features & news: `rfc/short/rfc8907.md` (`[RFC8907-5.1-2]` + resealed 19 shifted `rfc8907` audit verdicts via `./le rfc reseal stem rfc8907` and `./le rfc update`), `features/{login-service-authorization,tacacs-aaa,radius-admin-aaa}.md`, `website/changes/fragments/login-service-authorisation.md`.
- `./le doc ste check` passed with 0 issues across all changed docs; `./le doc check all` reported 0 issues in our YANG leaves, source anchors, or changed documentation.

### Deviations from Plan
- The five surface registration files are named `register_login_service.go` (instead of `login_service.go`) because `./le go style check` enforces that newly added `init()` functions live in `register*.go`. Updated all doc and spec references to match.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | First RADIUS interop run left `ze` without an initialized store and used `curl --max-time 5` | Web HTTPS requires a store at `StoreDir(configPath)` (`filepath.Dir(configPath)`), and silent RADIUS `timeout 2; retries 2` takes 6s before local fallback | Real FreeRADIUS interop run | Pre-initialized `/var/lib/ze-web-store` in `Dockerfile.ze`, mounted `ze.conf` there for `scenarioLoginServiceRefused`, and set `--max-time 15` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `system/authorization/login-service` list + leafrefs on `user`, `tacacs-profile`, `radius` | Done | `internal/component/{authz,tacacs,radius}/yang/ze-*-conf.yang` | `./le yang glue check` clean |
| Self-registering service names (`ssh`, `web`, `rest`, `grpc`, `mcp`) with no central enum | Done | `internal/component/config/loginservice/registry.go`, `*/register_login_service.go` | Zero central lists |
| Fail-closed gate at authentication across all 6 paths | Done | `internal/component/aaa/login_profiles.go`, `ssh/`, `web/`, `api/`, `mcp/`, `cmd/ze/hub/` | Verified by all 20 `.ci` tests + 16/16 discrimination check |
| Atomic generation publication & live reload (`ze-session` cookie + MCP immediate, open SSH/gRPC/SSE survive) | Done | `cmd/ze/hub/{aaa_lifecycle,main,main_reload}.go`, `internal/component/web/auth.go` | `login-service-reload-tightens.ci`, `login-service-web-cookie-reload.ci`, `login-service-mcp.ci` |
| Recovery exemption (`SourceLocal` + `ReservedRecoveryProfile`), OD-1d, OD-2a, OD-3a, OD-4b | Done | `internal/component/aaa/login_profiles.go`, `tacacs/`, `radius/`, `mcp/` | All four owner rulings verified |
| Post-config doctor check & refusal Prometheus counter | Done | `internal/component/aaa/{doctor,register,register_doctor_codes,login_profiles}.go` | `doctor-aaa-all-accounts-excluded`, `ze_aaa_login_service_refused_total{service}` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestLoginServiceRegistryRejectsConflictingRegistration`, `TestLoginServiceValidatorRejectsUnregisteredName` | Conflict returns `ErrConflict`; idempotent repeat accepted |
| AC-2 | Done | `login-service-{ssh-password,ssh-publickey,ze-cli,web-form,web-basic,rest,grpc,mcp}.ci` | Every door refuses identically to wrong credential & admits included door |
| AC-3 | Done | `login-service-{ssh-password,web-form,rest,mcp}.ci` | Audit `ActionAuthFail`, metric increment, zero state/command side effects |
| AC-4 | Done | `login-service-ssh-password.ci`, `login-service-rest.ci` | Admitted door runs allowed commands and keeps profile denials |
| AC-5 | Done | `login-service-reload-tightens.ci`, `login-service-web-cookie-reload.ci`, `login-service-mcp.ci` | Reload closes withdrawn doors without restart |
| AC-6 | Done | `TestLoginDecisionUnreadableRefusesAsRejection`, `login-service-chain-terminal.ci` | Nil generation or missing set returns terminal `ErrAuthRejected` |
| AC-7 | Done | `TestLoginDecisionAbsentSetAdmits`, `login-service-absent-section.ci` | Absent `login-service` admits every door |
| AC-8 | Done | `TestLoginDecisionAdmitsRecoveryResult`, `TestPowerUserIgnoresLoginSets`, `login-service-recovery-user.ci` | Unshadowed local recovery user admitted everywhere |
| AC-9 | Done | `TestLoginServiceValidatorRejectsUnregisteredName`, `login-service-unregistered-name.ci` | Names bad value, states build does not offer it, lists offered names |
| AC-10 | Done | `login-service-undefined-set.ci` | Leafref refusal on `user`, `tacacs-profile`, and `radius` |
| AC-11 | Done | `TestLoginDecisionEmptySetRefuses`, `login-service-empty-set.ci` | Empty `[]string{}` denies all doors, distinct from absent `""` |
| AC-12 | Done | `TestTacacsProfileCarriesLoginSet`, `TestRadiusConfigCarriesLoginSet`, `login-service-{tacacs,radius}.ci`, `radius-admin-login-service-refused-freeradius` | OD-1 option d proven against mocks and real FreeRADIUS |
| AC-13 | Done | `TestValidateTokenRefusesWithdrawnWebService`, `login-service-web-cookie-reload.ci`, `login-service-reload-tightens.ci` | Open SSH survives reload; web cookie refused HTTP 401 immediately |
| AC-14 | Done | `login-service-shadowed-recovery-name.ci` | OD-2 option a: shadowing config user gated as ordinary account |
| AC-15 | Done | `TestWrapperGateIsTerminalInChain`, `TestLiveBundleFallbackPassesDecision`, `login-service-chain-terminal.ci` | Gate refusal is terminal `ErrAuthRejected`, never falls through |
| AC-16 | Done | `TestAPIAuthenticationNamesTransport`, `login-service-shared-token.ci` | Shared-token mode stays admitted |
| AC-17 | Done | `TestAPIAuthenticationNamesTransport`, `login-service-rest.ci`, `login-service-grpc.ci` | Independent `"rest"` and `"grpc"` gating |
| AC-18 | Done | `TestRegisteredNamesMatchWireValues` (`ssh`, `web`), `radius-admin-login-service-refused-freeradius` | Wire values `"ssh"` and `"web"` unchanged |
| AC-19 | Done | `TestSSHLocalAuthenticatorWithoutBundleIsGated` | Bare SSH local authenticator wrapped with `WithProfileAuthorizer` |
| AC-20 | Done | `TestMCPStreamableLoginServiceGate`, `login-service-mcp.ci` | OD-4 option b: named MCP principals gated with HTTP 401 `invalid_token`, anonymous unaffected |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All 22 Unit Tests | Done | `config/loginservice`, `config`, `config/infra`, `aaa`, `doctor`, `tacacs`, `radius`, `ssh`, `web`, `mcp`, `cmd/ze/hub` | All pass `-race` |
| 2 Parse `.ci` Tests | Done | `test/parse/login-service-{unregistered-name,undefined-set}.ci` | 2/2 PASS (1.7s) |
| 18 Plugin `.ci` Tests | Done | `test/plugin/login-service-*.ci` | 18/18 PASS (7.6s), 90/90 stress pass, 16/16 gating discrimination pass |
| 1 FreeRADIUS Interop | Done | `test/interop-radius/scenarios/radius-admin-login-service-refused-freeradius/` | PASS (47s) against real FreeRADIUS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All modified files in `Files to Modify` | Done | Plus regenerated `internal/component/radius/yang/register.go`, `rfc/audit/rfc8907/*.json`, `features/login-service-authorization.md`, `website/changes/fragments/login-service-authorisation.md` |
| All created files in `Files to Create` | Done | Surface init files named `register_login_service.go` per style gate |

### Audit Summary
- **Total items:** 20 ACs, 22 unit tests, 20 `.ci` tests, 1 FreeRADIUS interop scenario, 19 doc/RFC/feature/news files
- **Done:** All
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (registration filenames `register_login_service.go`, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Named `login-service` sets restrict every door where a named account logs in at authentication time | Functional `.ci` + discrimination check | All 20 `.ci` tests PASS (`2/2` parse, `18/18` plugin, `90/90` x5 stress); disabling `CheckLoginSet` turned `16/16` gating tests FAIL RED while only the 2 non-regression control tests (`absent-section`, `shared-token`) passed |
| Service names self-register with compile-out safety | Unit + cross-tag lint | `TestLoginServiceRegistryRejectsConflictingRegistration`, `login-service-unregistered-name.ci`, `./le go lint run` compile-out tags clean |
| Remote TACACS+ and RADIUS accounts restricted locally with zero wire change (OD-1d) | Functional `.ci` + real FreeRADIUS interop | `login-service-tacacs.ci` PASS, `login-service-radius.ci` PASS, `RADIUS_INTEROP_SCENARIO=radius-admin-login-service-refused-freeradius ./le test integration interop-radius` PASS (47s) |
| MCP named principals restricted with live reload (OD-4b) | Unit + functional `.ci` | `TestMCPStreamableLoginServiceGate` PASS, `login-service-mcp.ci` PASS |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Per-user identity and `"looking-glass"` / `"gnmi"` registration | Looking glass and gNMI currently authenticate via shared bearer token with no username; explicitly out of scope per Task and Known Limitations | `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/login-service-authorisation-b5e50005-5555-4000-8000-000000000005.md` |
| `./le spec review check` | clean (136 files, verdict=clean) |
| Rounds | 2 (Round 1 found 3 non-blocker issues + 1 note; Round 2 verified all fixes CLEAN) |
| Reviewer lenses used | security + fail-closed audit, spec completeness (AC-1..AC-20), wiring & exports, RFC/docs alignment |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Missing RFC 8907 §5.1 verbatim quote and `docs/guide/tacacs.md` comment above TACACS+ `port` assignment | `internal/component/tacacs/authenticator.go:62-64` | Added RFC 8907 §5.1 quote and `docs/guide/tacacs.md` reference |
| 2 | ISSUE | Exported `loginservice.Lookup` had zero production callers | `internal/component/config/validators.go:758` | Switched `LoginServiceValidator` to `loginservice.Lookup` |
| 3 | ISSUE | Spec/feature/guide doc drift (`register_login_service.go`, 20 `.ci` tests + FreeRADIUS interop listing) | `spec-login-service-authorisation.md`, `features/{login-service-authorization,radius-admin-aaa}.md`, `docs/guide/{radius,tacacs}.md` | Aligned filenames and added promoted `.ci` and interop rows |
| 4 | NOTE | Retired-generation MCP refusal bypassed `aaa.refuseLoginService` log & metric | `cmd/ze/hub/aaa_lifecycle.go:368-371` | Routed `state == nil` through `aaa.CheckLoginService(username, nil, service)` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/config/loginservice/{registry.go,registry_test.go}` | Yes | Created & tested (`1 package ok`) |
| `internal/component/{ssh,web,api/rest,api/grpc,mcp}/register_login_service.go` | Yes | All 5 files exist, pass `./le go style check`, register `ssh`/`web`/`rest`/`grpc`/`mcp` |
| `internal/component/aaa/login_service_test.go`, `internal/component/radius/authenticator_test.go`, `internal/component/mcp/streamable_auth_test.go` | Yes | All pass `-race` |
| `test/parse/login-service-{unregistered-name,undefined-set}.ci` | Yes | `2/2 PASS` in live `test/parse/` |
| `test/plugin/login-service-*.ci` (18 files) | Yes | `18/18 PASS` in live `test/plugin/` (`90/90` stress pass) |
| `test/interop-radius/scenarios/radius-admin-login-service-refused-freeradius/{users,ze.conf}` | Yes | `PASS (47s)` against real FreeRADIUS |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-20 | All 20 acceptance criteria pass on live unit, `.ci`, and FreeRADIUS runs | `./le test unit pkg` passed across all 14 affected packages (`-race`); `./le test bgp parse --pattern login-service` `2/2 PASS`; `./le test bgp plugin --pattern login-service` `18/18 PASS` (`90/90` stress, `16/16` discrimination); `radius-admin-login-service-refused-freeradius` `PASS` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Config validation (`LoginServiceValidator`, leafrefs) | `test/parse/login-service-{unregistered-name,undefined-set}.ci` | Yes (`2/2 PASS`) |
| SSH password, SSH pubkey, `ze cli` | `test/plugin/login-service-{ssh-password,ssh-publickey,ze-cli}.ci` | Yes (`3/3 PASS`, fail RED without gate) |
| Web form, Basic Auth, cookie reload | `test/plugin/login-service-{web-form,web-basic,web-cookie-reload}.ci` | Yes (`3/3 PASS`, fail RED without gate) |
| REST, gRPC, MCP | `test/plugin/login-service-{rest,grpc,mcp}.ci` | Yes (`3/3 PASS`, fail RED without gate) |
| TACACS+, RADIUS, FreeRADIUS interop | `test/plugin/login-service-{tacacs,radius}.ci`, `radius-admin-login-service-refused-freeradius` | Yes (`3/3 PASS`, fail RED without gate) |
| Reload, recovery, shadowed recovery, absent, empty, chain-terminal, shared-token | `test/plugin/login-service-{reload-tightens,recovery-user,shadowed-recovery-name,absent-section,empty-set,chain-terminal,shared-token}.ci` | Yes (`7/7 PASS`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1..A-9 | Confirmed | All 9 assumptions verified in code and tests: `LoginServiceValidator` mirrors `RedistributeSourceValidator`, `api.Authentication.Authenticate(authHeader, service)` keeps `aaa` transport-free, `login-service-absent-section.ci` preserves existing configs, `ReservedRecoveryProfile` exemption is local-only, registrations run before validation, refusal parity matches wrong credentials across all doors, `pkg/plugin/sdk/sdk_identity.go` exposes only `InstanceName()`, `TestEveryValidatorSectionIsWalkedOrExcused` passes, and `login-service-mcp.ci` confirms MCP principal linkage |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Guides (`authentication`, `authorization`, `configuration`, `operator-access-rbac`, `tacacs`, `radius`, `monitoring`, `metrics`) | Matched against `ze-*-conf.yang`, `login_profiles.go`, `doctor.go`, `streamable.go`, `web/auth.go`; `./le doc ste check` 0 issues | Yes |
| Architecture & patterns (`core-design`, `api/architecture`, `web-interface`, `aaa-tacacs`, `mcp/overview`, `ai/patterns/registration.md`) | Matched against `register_login_service.go`, `aaa_lifecycle.go`, `service_registry.go`; `./le doc check all` 0 issues in our anchors | Yes |
| RFC & Features (`rfc/short/rfc8907.md`, `features/{login-service-authorization,tacacs-aaa,radius-admin-aaa}.md`, `website/changes/fragments/login-service-authorisation.md`) | `./le rfc reseal stem rfc8907` + `./le rfc update` + `./le feature check` (0 issues on `login-service-authorization`) | Yes |
