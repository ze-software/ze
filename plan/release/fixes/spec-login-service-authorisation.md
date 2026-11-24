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
| Updated | 2026-11-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

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

The owner's design, given 2026-08-11: a new authorisation section, sibling to
`profile`, naming the login services an account may use. A service that offers a
login passes its own name into the authentication request. When the account's
login set has no match for that name, authentication FAILS: the username does
not work on that surface even though the account exists and the password is
right.

This is an AUTHENTICATION gate, not an authorisation one. A refused login never
reaches command authorisation.

Service names are REGISTERED, not spelled in the schema. A plugin that offers a
new login registers its name, and config validation checks a configured value
against the live registry.

**Scope of this spec: every door where a named account logs in.** That is SSH
by password, SSH by public key, the web login form, web Basic Auth, the web
session cookie, REST and gRPC with per-user credentials, and `ze cli`, which is
an SSH login. The section "Login surfaces" lists every door Ze has, and says for
each one where the decision applies or why it does not. The looking glass and
gNMI authenticate with a shared token and learn no username at all, so they
cannot answer the question yet; giving them identity is
`plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md`.

The SSH composition prerequisite closed in `6bbdb9c037` on 2026-08-17.
It remains historical context and no longer blocks this spec.

This spec supplies the login-service gate required by
`plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md`, which
names this spec in its `Depends` row. Both specs sit in the release fixes
bucket, so the gate is first-release work. This spec owns the doors listed
above; the dependent spec owns looking-glass and gNMI identity and their
integration with this gate.

**The code moved after 2026-08-11.** The spec was re-read against the tree on
2026-11-14. What changed under it: the service name already travels in
`aaa.AuthRequest.Service`; five authentication paths exist where the spec
assumed one; remote backends answer before the local list; the undefined-profile
check is now a YANG leafref; REST and gRPC authenticate through one shared
closure. Current Behavior states each one at its producing function.

## Owner decisions open

Each row is a choice no RFC, agreed design or rule in `ai/META-DECISIONS.md`
answers. The design below is complete for every other question. Implementation
of the rows these decisions touch waits for the answer; the rest does not.

| ID | Question | Evidence | Options | What each option changes |
|----|----------|----------|---------|--------------------------|
| OD-1 | What does a login set mean for a user a remote backend authenticated, who has no local record? | `(*backendRegistry).Build` (`internal/component/aaa/types.go`) orders RADIUS 50, TACACS+ 100, local 200, and `(*ChainAuthenticator).Authenticate` returns the first authenticated result. A TACACS+ or RADIUS operator is admitted before the local list is read, and `ExtractAuthUsers` holds no record for that name, so no login set exists to consult | (a) Login sets bind local accounts only; a remote-authenticated user reaches every door. (b) A local record with the same name supplies the set, and a remote user with no record is ungated. (c) The same as (b), and a remote user with no record is refused once any login set exists. (d) The set is carried beside the profiles a backend maps: the TACACS+ privilege mapping (`tacacs-profile`) and its RADIUS equivalent each name a set | (a) AC-12 asserts admission, and the guide says the gate does not cover remote accounts. (b) and (c) need a user entry with no password to be a supported shape, and (c) locks out a remote-only deployment at the first login set. (d) adds a leaf to each remote mapping and a `Files to Modify` row for `tacacs` and `radius`, and it is the only option that restricts a remote user without a local record. Until the answer no code path guesses: AC-12 stays unwritten |
| OD-2 | May a config user that takes the stored recovery user's name be locked out of every door? | `mergeAuthUsers` (`cmd/ze/hub/main_servers.go`) drops the stored user when a config user has its name, and `docs/architecture/api/architecture.md` ("REST and gRPC authentication flow") states the result: "Only surviving stored users retain their recovery profile." The exemption this spec applies is keyed on `aaa.ReservedRecoveryProfile` on the authentication result, the key `liveAAABundleAuthenticator.Authenticate` and `aaa.AuthorizerForResult` already use | (a) Keep the existing key: a shadowing config user is an ordinary account and its login set applies. (b) Key the exemption on the stored username as well, so the name keeps every door whoever declares it. (c) Refuse at commit a login set on a user whose name is the stored recovery user's | (a) changes nothing in `mergeAuthUsers`; the serial console (`loginMain`, `cmd/ze/login.go`) is the only door left on an appliance, and a non-appliance host has none. (b) makes a name, not a credential source, carry a privilege, which departs from the documented design. (c) keeps the documented key and removes the lockout, and adds one validation row and one AC. The design below is written for (a), the documented behavior |
| OD-3 | Does a user naming no login set reach every login service? | No config in the repository or the field carries the section. `docs/guide/authentication.md` documents no per-door restriction today | (a) Yes, absent means every door. (b) No, absent means no door | (a) is AC-7 as written, and every existing config keeps working. (b) inverts AC-7 only, and every existing deployment needs a login set before its users can log in after the upgrade |
| OD-4 | Does MCP stay outside this work? | `(*Streamable).authenticate` (`internal/component/mcp/streamable.go`) returns an `Identity` (`internal/component/mcp/auth.go`) with its own `HasScope` model. It builds no `aaa.AuthRequest` and never reaches `authz.Store` | (a) Outside, as assumed. (b) Inside | (a) is the design below. (b) is a second design that folds two scope models into one, and needs its own spec |

OD-3 and OD-4 are the two items the 2026-08-11 text held under "Assumptions the
owner has not ruled on". They are kept, in this table.

One question is answered by the documented design, so it is not a row. Whether
withdrawing a door ends a session that is already open:
`docs/guide/operator-access-rbac.md`, section 8, states the rule for removing a
user: "Removal governs NEW connections. A session that is already open outlives
it until the connection closes", while the web session cookie is refused at
once. Withdrawing a door follows the same rule, because a login-set edit is a
narrower form of the same act. Departing from it is a departure from a design
page, which needs the owner (`ai/META-DECISIONS.md`, "How agents work"). AC-5
and AC-13 assert the documented rule.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - "Identity and Authorization": how REST and gRPC obtain one accepted authentication generation per request
  → Constraint: users, policy and API authentication publish as one generation (`acceptedLocalIdentityState`). Login sets join that generation; they are never read from the config provider at request time, because the provider holds a rejectable candidate during a reload.
  → Constraint: "A config user replaces a same-name stored user. Only surviving stored users retain their recovery profile."
- [ ] `docs/architecture/config/syntax.md` - the JUNOS-like config syntax: blocks, terminators, comments and inheritance
- [ ] `docs/architecture/config/yang-config-design.md` - YANG (RFC 7950) as the schema language, and "Where a validator actually runs"
  → Constraint: a `ze:validate` runs only in a section the validator walk covers (`ai/doors/config-yang.md`, "Traps"). The new leaf sits under `system`, and the implementation proves the walk reaches it.
- [ ] `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` - the stored bcrypt hash is a local-only credential, and is masked on display
- [ ] `docs/architecture/web-interface.md` - "Authentication": cookie, Basic Auth, and the per-request re-check of a locally granted session
  → Constraint: an open SSE stream outlives a removal until the client disconnects. The same holds for a withdrawn door.
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
  → Constraint: its table becomes incomplete the moment a correct password can be refused for the surface it arrived on.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8907.md` - the TACACS+ `port` field carries the service name
  → Constraint: RFC 8907 Section 5.1: "The client documentation SHOULD define the values and their meanings for this field." The registered service name IS the value `tacacsAuthenticator.Authenticate` (`internal/component/tacacs/authenticator.go`) sends as `port`, and the value `radiusAuthenticator.exchange` (`internal/component/radius/authenticator.go`) sends as NAS-Port-Id. `docs/guide/tacacs.md` and `docs/guide/radius.md` define `ssh` and `web` today. The summary holds no row for this sentence.
  → Constraint: the names `ssh` and `web` are already on the wire. The registry adopts them unchanged. Renaming either changes what a TACACS+ or RADIUS server receives.

**Key insights:** (minimal context to resume after compaction)
- The registry-backed name check already exists and needs no invention: `RegisterSource` (`internal/component/config/redistribute/registry.go`) is written at `init()`, the leaf is `type string` carrying `ze:validate "redistribute-source"`, and `RedistributeSourceValidator` (`internal/component/config/validators.go`) calls `redistribute.LookupSource` at validate time and hands `redistribute.SourceNames` back as its `CompleteFn`. `CheckAllValidatorsRegistered` (`internal/component/config/yang/validator_registry.go`) fails a `ze:validate` name nobody registered.
- There is ONE user list. `ExtractAuthUsers` (`internal/component/config/infra/authz.go`) reads `system/authentication/user` and is the only producer of `authz.UserConfig`, which is `aaa.UserCredential` (`internal/component/aaa/types.go`).
- There are FIVE authentication paths over that list, not one. See Current Behavior.
- `aaa.AuthRequest.Service` already carries the service name. SSH password and web set it. SSH public key builds no request. REST and gRPC set nothing.
- `aaa.UserCredential` carries `Name`, `Hash`, `Profiles`, `PublicKeys` and the runtime-only `LocalGeneration`. It carries no login-set field.
- The profile reference is a YANG leafref with `require-instance true` (`internal/component/authz/yang/ze-authz-conf.yang`), not a string compare. The login-set reference is its sibling and takes the same shape.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/authz/authz.go` - `(*Store).Authorize` is the single command decision point and fails closed on an empty identity. Nothing on this path names a surface.
- [ ] `internal/component/aaa/types.go` - `AuthRequest{Username, Password, RemoteAddr, Service, Local}`. `(*ChainAuthenticator).Authenticate` returns the first authenticated result, stops on `ErrAuthRejected`, and on any other error tries the next backend. `(*backendRegistry).Build` orders backends by priority and wraps the chain with `WithProfileAuthorizer`.
- [ ] `internal/component/aaa/login_profiles.go` - `resultAuthorizingAuthenticator.Authenticate`, the wrapper `WithProfileAuthorizer` returns, refuses a reserved username and binds an authorizer to a successful result. Its comment calls it the one choke point every surface passes through. Three paths below do not pass through it. `AuthorizerForResult` keys the recovery grant on `ReservedRecoveryProfile` in a LOCAL result, and strips that profile from a remote result.
- [ ] `internal/component/authz/auth.go` - `(*LocalAuthenticator).Authenticate` matches the name and the bcrypt hash and returns the user's profiles. It reads `request.Local` and never reads `request.Service`. An unreadable user list returns that error, which is not `ErrAuthRejected`.
- [ ] `internal/component/config/infra/authz.go` - `ExtractAuthUsers` reads `system/authentication/user` from the resolved `system` map and is the ONE producer of the credential shape. `ValidateAuthzConfig` checks profile regex syntax and reserved names only; its comment states that an undefined profile name is refused by the leafref (`Validator.ValidateReferences`).
- [ ] `internal/component/authz/yang/ze-authz-conf.yang` - owns both `system/authentication/user` and `system/authorization/profile`. `leaf-list profile` is a leafref to the profile list's key, with `require-instance true`.
- [ ] `internal/component/ssh/yang/ze-ssh-conf.yang` - augments shared users only with SSH public keys and owns `environment.ssh`. It does not own the new login-set reference.
- [ ] `internal/component/ssh/passwordauth.go` - `(*Server).authenticatePasswordResult` calls the configured authenticator with the service `ssh`, a string literal, and `Local` from the accepted socket address.
- [ ] `internal/component/ssh/pubkey.go` - `(*Server).authenticatePublicKeyResult` reads `s.users()` and calls `matchPublicKey`. It calls NO `Authenticator` and builds no `AuthRequest`, and returns a result whose `Source` is `aaa.SourceLocal`.
- [ ] `internal/component/ssh/ssh.go` - the `wish.WithPublicKeyAuth` and `wish.WithPasswordAuth` callbacks are the two SSH login entry points. With no configured authenticator the server builds a bare `LocalAuthenticator` over its own user source, with no wrapper.
- [ ] `cmd/ze/hub/infra_setup.go`, `cmd/ze/hub/main.go` - SSH receives `liveAAABundleAuthenticator{}` with no fallback. `noBGPAAAWiring` hands a daemon without BGP the same value, and a nil authenticator when no bundle composed.
- [ ] `cmd/ze/hub/service_web.go` - web receives `liveAAABundleAuthenticator` with a `LocalAuthenticator` fallback over the accepted users.
- [ ] `cmd/ze/hub/aaa_lifecycle.go` - `liveAAABundleAuthenticator.Authenticate` asks the live bundle first. When the bundle does not authenticate and a fallback is set, a fallback success is returned for ANY bundle error that is not `ErrAuthRejected`; after `ErrAuthRejected` only a local result carrying `ReservedRecoveryProfile` is returned. `acceptedLocalIdentityState` holds users, the policy store and the API authentication of one accepted generation; `publishAcceptedLocalIdentity` installs it. `sameLocalCredential` compares every `UserCredential` field except `LocalGeneration`, and `TestSameLocalCredentialReadsEveryField` pins the field count.
- [ ] `internal/component/web/auth.go` - `LoginHandlerWithAudit` authenticates the form with the service `authServiceWeb` and calls the unexported `(*SessionStore).createSession`. `AuthMiddlewareWithAudit` accepts a `ze-session` cookie through the unexported `(*SessionStore).validateToken`, and otherwise authenticates HTTP Basic Auth with the same service name and serves the request with NO session. `validateToken` re-checks a locally granted session against the live user list on every request, by name only. `InsecureMiddleware` serves with no login at all.
- [ ] `cmd/ze/hub/api.go` - `buildAPIAuthentication` builds a PRIVATE authenticator for REST and gRPC: `WithProfileAuthorizer` over a `LocalAuthenticator` holding the accepted users. It never asks the AAA chain, and its `AuthRequest` sets neither `Service` nor `RemoteAddr`. With no user at all and a token configured, it returns `aaa.ReservedSharedAPIUsername` for a matching token. With neither, authentication is not required.
- [ ] `internal/component/api/types.go` - `Authentication{Required, Authenticate}`. `Authenticate` takes the authorization header and nothing else, so it cannot tell REST from gRPC.
- [ ] `internal/component/api/rest/auth.go` - `(*RESTServer).withAuth` calls `authentication.Authenticate` and sets the audit surface `audit.REST` on the caller after the fact.
- [ ] `internal/component/api/grpc/server.go` - `(*GRPCServer).checkAuth` does the same with `audit.GRPC`, once per RPC and once per stream at open.
- [ ] `cmd/ze/hub/main_servers.go` - `usersFromStore` reads the stored recovery user and gives it the single profile `aaa.ReservedRecoveryProfile`. `mergeAuthUsers` drops a stored user whose name a config user takes. `liveLocalUsers` assembles the candidate list at boot and at each reload. No function named `usersFromZefsDB` exists; the 2026-08-11 text cited one.
- [ ] `internal/component/tacacs/authenticator.go`, `internal/component/tacacs/client.go` - `tacacsAuthenticator.Authenticate` sends `request.Service` as the START `port` field, and `defaultService` (`ssh`) when it is empty.
- [ ] `internal/component/radius/authenticator.go` - `radiusAuthenticator.exchange` sends `request.Service` as NAS-Port-Id, and omits the attribute when the caller names no service.
- [ ] `internal/component/cli/sshclient/client.go` - `readCredentials` resolves the SSH credential `ze cli` presents to the daemon's SSH server, on `127.0.0.1` by default. The stored recovery user presents its hash as the credential; any other user presents a password.
- [ ] `cmd/ze/login.go`, `cmd/ze-serial-shell/main.go` - the serial console runs `ze` as the login shell. `loginMain` opens the credential store itself, compares the typed password with bcrypt against the stored recovery user, and executes the shell. No daemon, no config user and no AAA backend takes part.
- [ ] `internal/component/mcp/streamable.go`, `internal/component/lg/auth.go`, `internal/component/gnmi/server.go` - MCP returns its own `Identity`; the looking glass (`bearerAuth`) and gNMI (`(*Server).checkAuth`) compare one optional shared bearer token and learn no username.
- [ ] `internal/component/plugin/acceptor.go` - `NewHubAcceptor`: an external plugin authenticates to the hub with a per-plugin token over TLS. It is a process identity, not an operator login.
- [ ] `internal/component/config/redistribute/registry.go` - `RegisterSource`, `SourceNames`, `LookupSource`: the registry shape to copy, including `ErrSourceConflict` on a re-registration that disagrees.
- [ ] `internal/component/config/validators.go` - `RedistributeSourceValidator`, the validator shape to copy, including `CompleteFn` for CLI completion. `RegisteredProtocolValidator` beside it shows the refusal that lists the registered names.
- [ ] `internal/component/config/validators_register.go` - `RegisterValidators` wires a validator to the `ze:validate` name.

**The five authentication paths today:**

| Path | Entry point | Who answers | Service named | Passes the wrapper |
|------|-------------|-------------|---------------|--------------------|
| Live AAA chain | SSH password; web form; web Basic Auth | `liveAAABundleAuthenticator.Authenticate`, then the bundle: RADIUS, TACACS+, local, in that order | `ssh`, `web` | Yes |
| SSH public key | `authenticatePublicKeyResult` | `matchPublicKey` over the server's user source. No authenticator is called | none | No |
| Private API authenticator | REST `withAuth`; gRPC `checkAuth` | The closure `buildAPIAuthentication` returns: local users only, never the chain | none | Yes |
| Web fallback | The same web entry points, when the chain did not authenticate | The `LocalAuthenticator` inside `liveAAABundleAuthenticator` | `web` | No |
| SSH with no bundle | SSH password, on a daemon where no backend composed | The bare `LocalAuthenticator` the SSH server builds for itself | `ssh` | No |

**Behavior to preserve:**
- `(*Store).Authorize` keeps failing closed on an empty identity, and command authorisation keeps its current verdicts for every user who is admitted.
- The stored recovery user, while no config user takes its name, keeps authenticating everywhere it does today, including its reserved recovery profile. It is the recovery path.
- The live user source survives: a user deleted from the config still stops authenticating at the next reload without a restart.
- Public-key SSH login keeps working, and the login set gates it identically to a password login.
- Every existing config keeps working with no edit, per OD-3 option (a).
- `ssh` and `web` stay the values sent to a TACACS+ and a RADIUS server.
- An explicit rejection still stops the chain, and a connection error still moves to the next backend (`ai/doors/access-security.md`, "Boundaries").
- A session that is already open outlives a config edit until its connection closes, and the web cookie does not.

**Behavior to change:**
- A new named list under `system/authorization/` names a set of login services.
- `system/authentication/user` gains a reference to that list, a leafref with `require-instance true`, the shape the profile reference already has.
- Each login service registers its name, and every authentication path names the service of the login it decides: the public-key path and the REST and gRPC path included.
- A login whose service name is not in the account's set is REFUSED on every path, with the same response the surface gives a wrong password.
- A refusal by the gate is terminal. It never moves to the next backend and never opens the web fallback.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Service name on the authentication request | `internal/component/aaa/types.go` `AuthRequest.Service` | Carries `ssh` or `web` to the backends, which send it to the TACACS+ and RADIUS server | Extends: the gate reads the same field. No second field is added. REST and gRPC start filling it, and the public-key path names its service to the gate directly |
| The authentication wrapper | `internal/component/aaa/login_profiles.go` `WithProfileAuthorizer`, `resultAuthorizingAuthenticator.Authenticate` | Wraps the chain and the private API authenticator; refuses a reserved username; binds the authorizer to a successful result | Extends: the login-set decision runs here, after a backend authenticated and before the result is returned. The three paths that do not pass through it call the same decision function |
| Name registry with a validator | `internal/component/config/redistribute/registry.go` `RegisterSource`; `internal/component/config/validators.go` `RedistributeSourceValidator` | Registers names at `init()`, validates a `ze:validate` leaf against them, completes them in the editor, refuses a conflicting re-registration | Extends by copy: a second registry of the same shape for login-service names. The redistribute registry itself is not reused, because its entries carry a protocol and a source meaning |
| Reference to a named list entry | `internal/component/authz/yang/ze-authz-conf.yang` `leaf-list profile`, a leafref with `require-instance true` | Refuses at commit a user naming a profile nobody defines, and names the profiles that exist | Extends: the login-set reference is a leafref of the same form. No string compare is added to `ValidateAuthzConfig` |
| One accepted identity generation | `cmd/ze/hub/aaa_lifecycle.go` `acceptedLocalIdentityState`, `publishAcceptedLocalIdentity`, `liveAcceptedLocalUsers` | Publishes users, policy and API authentication together, after every fallible reload step | Extends: the login sets and each user's reference join the generation. The gate never reads the config provider |
| Credential comparison that revokes | `cmd/ze/hub/aaa_lifecycle.go` `sameLocalCredential` | Advances the generation when any credential field differs | Extends: the login-set reference is a compared field, and its pinned field count moves with it |
| Per-request re-check of a web session | `internal/component/web/auth.go` `(*SessionStore).validateToken`, `localUserDeclared` | Refuses a locally granted session whose user the accepted list no longer declares | Extends: the same re-check also refuses a session whose user may no longer use the web service |
| Recovery identity | `internal/component/aaa/reserved.go` `ReservedRecoveryProfile`; `cmd/ze/hub/main_servers.go` `usersFromStore`; `aaa.AuthorizerForResult` | Marks the stored recovery user's result; lets it through an explicit chain rejection on web; keeps its command rights under a strict policy | Extends: the same mark exempts the result from the login-set decision. No second key is invented (OD-2) |
| Refusal that names a compiled-out feature | `docs/architecture/plugin/feature-gates.md`, "Refusing a root whose plugin is compiled out" | Tells the operator the build lacks the feature, and how to fix it | Extends in kind: the validator's refusal of a name this build does not offer says so and lists the names it does offer. The plugin `ConfigNeed` machinery does not apply, because a login surface is not a plugin dependency |
| Audit surface names | `internal/core/audit/audit.go` `SSH`, `Web`, `REST`, `GRPC` | Name the surface in an audit record | Does not apply: an audit name labels a record after the fact and includes surfaces with no login. The login registry is narrower on purpose (Known Limitations) |
| Command authorisation | `internal/component/authz/authz.go` `(*Store).Authorize` | Decides a command for an admitted user | Does not apply: the gate decides before any command exists |

## Login surfaces

Every door Ze has, and where the decision applies. "Gated" means the login-set
decision runs before the caller is admitted.

| Surface | Authenticates through | Decision | Reason or place |
|---------|-----------------------|----------|-----------------|
| SSH password | `authenticatePasswordResult`, live AAA chain, or the bare local authenticator when no bundle composed | Gated, service `ssh` | In the authentication wrapper; the bare authenticator is wrapped the same way |
| SSH public key | `authenticatePublicKeyResult` | Gated, service `ssh` | The function calls the decision itself: it builds no request |
| SSH subsystems | The SSH login above (`subsystemOptions`, `internal/component/ssh/ssh.go`) | Gated with the login | No login of their own |
| `ze cli` | An SSH password login over loopback (`internal/component/cli/sshclient/client.go`) | Gated, service `ssh` | It is an SSH login. A set that excludes `ssh` also closes `ze cli` for that account, and the guide says so |
| Web login form | `LoginHandlerWithAudit`, live AAA chain, then the fallback | Gated, service `web` | In the wrapper, and on the fallback's result |
| Web Basic Auth | `AuthMiddlewareWithAudit`, the same authenticator | Gated, service `web` | The same two places. No session exists to hold |
| Web session cookie | `validateToken` | Gated, service `web`, on every request | A cookie is a credential accepted earlier. The per-request re-check reads the accepted generation |
| Web SSE stream already open | Authenticated at connect | Not re-checked | `docs/architecture/web-interface.md`: an open stream outlives a removal until the client disconnects |
| Web insecure mode | `InsecureMiddleware` | Not applicable | No login and no account. The listener guard already confines it |
| REST, per-user credentials | `withAuth`, private API authenticator | Gated, service `rest` | The wrapper inside `buildAPIAuthentication`, once `Authenticate` is told the service |
| gRPC, per-user credentials | `checkAuth`, private API authenticator | Gated, service `grpc`, per RPC and at stream open | The same |
| gRPC stream already open | Authenticated at open | Not re-checked | The rule for an open connection |
| REST and gRPC shared token | `buildAPIAuthentication`, token arm | Not applicable | The caller is `aaa.ReservedSharedAPIUsername`, which is no account and can name no set. This mode exists only when no user is declared at all, the stored recovery user included |
| REST and gRPC with no authentication | `Authentication` with `Required` false | Not applicable | Read-only, loopback only, no account |
| MCP | `(*Streamable).authenticate` | Outside (OD-4) | Its own identity and scope model; no `aaa.AuthRequest` |
| gNMI | Optional shared bearer token (`internal/component/gnmi/server.go`) | Outside this spec | No username. `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` |
| Looking glass | Optional shared bearer token (`internal/component/lg/auth.go`) | Outside this spec | No username. The same dependent spec |
| Serial console | `loginMain` (`cmd/ze/login.go`) | Not gated, by design | It runs with no daemon and reads no config, so no login set exists where it runs. It admits only the stored recovery user and is the recovery door of an appliance |
| Plugin sockets | Per-plugin token over TLS (`internal/component/plugin/acceptor.go`) | Not applicable | A process identity the hub issued, not an operator login |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator presents a credential on one of the gated surfaces above.
- A config tree read at boot or at reload, carrying `system/authorization/` and `system/authentication/user`.
- A login surface's `init()` registering its service name.

### Transformation Path
1. A component that offers a login registers its service name at `init()`, from its own package, so a build without the surface registers nothing for it.
2. Config validation resolves each configured service name against the registry (the `ze:validate` validator), and each user's login-set reference against the configured sets (the leafref).
3. `ExtractAuthUsers` produces the credential list, now carrying the login-set reference. The login sets are extracted beside the policy store.
4. Boot, and each reload after its last fallible step, publishes users, policy and login sets as one accepted generation.
5. A login arrives on one surface. The surface names its service: in the request for the paths that build one, and directly to the decision for the public-key path.
6. A backend authenticates the credential. Only then does the decision run, on the authenticated result: it reads the accepted generation, finds the account's set, and answers admit or refuse.
7. The decision refuses when the service is not in the set, when the request names no service or one nobody registered, and when it cannot read the generation or resolve the set. Every refusal is `aaa.ErrAuthRejected`, logged with its reason.
8. A result carrying the reserved recovery profile from the local backend is admitted without reading a set.
9. A refusal returns the surface's existing unauthorised response, unchanged in shape, and the surface records a failed login.
10. An admitted login proceeds, and `(*Store).Authorize` decides commands exactly as it does today.
11. On each later web request, `validateToken` asks the same decision for the session's user and the web service.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Login surface → name registry | A registration call at `init()` in the surface's own package; the registry lives in the daemon process | Yes: the shape `RegisterSource` (`internal/component/config/redistribute/registry.go`) already uses |
| External plugin → name registry | Not crossed. An `init()` in a forked plugin writes that process's own copy (`ai/doors/system.md`, "A plugin can run in another process"), and the plugin SDK holds no contract for a plugin to offer an operator login | Yes for the door's rule. The absence of an SDK login contract rests on a search of `pkg/plugin` only (A-7) |
| Config tree → credential | `ExtractAuthUsers` (`internal/component/config/infra/authz.go`) stays the one producer; the login-set reference joins the existing fields | Yes: one reader, no second parser of `system.authentication` |
| Config → accepted generation | `publishAcceptedLocalIdentity` (`cmd/ze/hub/aaa_lifecycle.go`) | Yes: the function exists and is the only publication point |
| Surface → authentication request | `aaa.AuthRequest.Service`, which exists | Yes for SSH password and web. REST and gRPC need `api.Authentication.Authenticate` to carry the service (A-2) |
| Authentication → authorisation | Unchanged. A refused login never reaches `(*Store).Authorize` | Yes: the gate sits before the authorisation call |
| Ze → TACACS+ and RADIUS server | The service name in the `port` field and in NAS-Port-Id | Yes: values unchanged, see the census |

### Changed-Wire Census

No byte on any wire changes. The census is kept because the registered name is
the value two wires already carry, so a rename in the registry is a wire change
the next editor must see.

**Search evidence:** scope `internal/`, `cmd/` and `pkg/`, non-test Go files. Terms: `Service:` as a struct field setter, and `.Service` read in `internal/component/tacacs`, `internal/component/radius`, `internal/component/aaa` and `internal/component/authz`. Positive control: the search finds the `ssh` setter in `internal/component/ssh/passwordauth.go`, the known producer. Findings: two setters of `AuthRequest.Service` (SSH password, and web at two call sites) and two readers that put it on a wire (TACACS+, RADIUS). REST and gRPC set nothing, and their private authenticator reaches neither reader.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| TACACS+ port field | writer | `internal/component/tacacs/authenticator.go` `tacacsAuthenticator.Authenticate` | Reads `request.Service`, falls back to `defaultService` (`internal/component/tacacs/client.go`); the session authorization carries the same value | Preserve: `ssh` and `web` unchanged. The fallback stays, and after this spec no gated path sends an empty name |
| TACACS+ port field | reader | The operator's TACACS+ server, outside the repository | RFC 8907 Section 5.1, free-format text | Preserve: the server sees the same values. `docs/guide/tacacs.md` stays their definition |
| RADIUS NAS-Port-Id | writer | `internal/component/radius/authenticator.go` `radiusAuthenticator.exchange`, called from `Authenticate` and from the EAP path in `internal/component/radius/authenticator_eap.go` | Passes `request.Service` to `AppendTextAttr` as NAS-Port-Id | Preserve: values unchanged |
| RADIUS NAS-Port-Id | reader | The operator's RADIUS server, outside the repository | `docs/guide/radius.md`, the admin login attributes | Preserve |

### Integration Points
- `ExtractAuthUsers` (`internal/component/config/infra/authz.go`) - the one credential producer. The login-set reference is read here or nowhere.
- `resultAuthorizingAuthenticator.Authenticate` (`internal/component/aaa/login_profiles.go`) - where the decision runs for the chain and for the private API authenticator.
- `authenticatePublicKeyResult` (`internal/component/ssh/pubkey.go`) - calls the decision after `matchPublicKey` succeeds.
- The bare local authenticator in `internal/component/ssh/ssh.go` - takes the same wrapper, so a daemon with no bundle is gated too.
- `liveAAABundleAuthenticator.Authenticate` (`cmd/ze/hub/aaa_lifecycle.go`) - the fallback's result passes the same decision, and a gate refusal from the bundle is terminal.
- `(*SessionStore).validateToken` (`internal/component/web/auth.go`) - the per-request re-check.
- `buildAPIAuthentication` (`cmd/ze/hub/api.go`) and `api.Authentication` (`internal/component/api/types.go`) - the closure learns which transport called.
- `publishAcceptedLocalIdentity` and `sameLocalCredential` (`cmd/ze/hub/aaa_lifecycle.go`) - the generation carries the sets, and the comparison reads the new field.
- `RegisterValidators` (`internal/component/config/validators_register.go`) - where the new `ze:validate` name is wired.
- `(*Store).Authorize` (`internal/component/authz/authz.go`) - NOT edited. Command authorisation is a separate question and stays one.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes, by design | One decision function. The wrapper covers two paths, and the three paths outside it are named and call it. AC-2 proves each door separately |
| No unintended coupling (components stay isolated) | Yes | The registry is a leaf beside the redistribute one. `aaa` learns no transport type: it reads a string it already carries |
| No duplicated functionality (extends existing, does not recreate) | Yes | See Existing Mechanisms: every part extends something that exists |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Login path, no wire buffers |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | A surface adds one registration call in its own package. No shared file lists the names |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes, with one stated exception | Searched: the audit constants (`internal/core/audit/audit.go`), `registerService` (`cmd/ze/hub/service_registry.go`), `DiscoverListenerServices` (`internal/component/config/listener.go`), `defaultService` (`internal/component/tacacs/client.go`). None learns a new name. The exception is `defaultService`, an existing literal `ssh` for a caller that names no service; it stays, and its doc row is in Files to Modify |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The registry-backed name check needs no new mechanism | `RegisterSource` / `RedistributeSourceValidator` / `RegisterValidators` are read and are exactly this shape | The spec invents a mechanism the repo already has, against `ai/rules/simplicity.md` | The implementation's validator is a copy of the redistribute one, reviewed side by side | unvalidated |
| A-2 | REST and gRPC can name themselves by passing a string into `api.Authentication.Authenticate`, with no transport type crossing into `aaa` | The closure is called from exactly two places, `withAuth` and `checkAuth`, each of which already knows its audit surface | The threading grows a parameter through unrelated layers | A design step that names the changed signature before any surface is edited, and the two call sites compile against it | unvalidated |
| A-3 | The default in OD-3 option (a) keeps every field config working | No config in the field carries a section that does not exist yet | An upgrade locks operators out of their own daemon, including out of the surface they would use to fix it | AC-7, and a functional test on a config carrying no login section at all | unvalidated |
| A-4 | The stored recovery user is recognized by `aaa.ReservedRecoveryProfile` on a local result, and no config user can carry that profile | `usersFromStore` (`cmd/ze/hub/main_servers.go`) is the only producer of that profile on a user; `ValidateAuthzConfig` refuses a user that names a reserved profile | A config user gains the exemption, or the recovery user loses it and the recovery path becomes lockable | AC-8 and AC-14, driven from a running daemon | unvalidated |
| A-5 | Registration completes before the first config validation reads the registry | Every existing registry of this shape is written at `init()`, and the composition root is generated | A name registered later is rejected at validate time | The design step states the ordering explicitly, as `RegisterListenerProtocols` already does for its own readers | unvalidated |
| A-6 | Every surface maps a gate refusal to the response it gives a wrong credential | Each entry point tests for a nil error and an authenticated result, and has one failure branch | A refusal is distinguishable from a wrong password, which leaks that the account exists | AC-2 compares the two responses on each door | unvalidated |
| A-7 | No login-offering contract exists for an external plugin | A search of `pkg/plugin` for an authentication callback found identity helpers and no login offer | An external plugin's login is ungated, or its name is refused at validate | Read `pkg/plugin/sdk/sdk_identity.go` in the design step and record the answer here | unvalidated |
| A-8 | The validator walk reaches a `ze:validate` leaf under `system/authorization` | `ai/doors/config-yang.md`, "Traps": the walk covers only its listed sections | The validator is declared and never runs, so an unregistered name is accepted | `test/parse/login-service-unregistered-name.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->, and `TestEveryValidatorSectionIsWalkedOrExcused` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator locks themselves out of every surface, including the one they would use to undo it | A config that names a login set excluding every service | The stored recovery user is exempt while no config user takes its name (A-4, OD-2), and the serial console does not go through this gate. AC-8 proves it |
| R-2 | The gate is added to some paths and forgotten on another, so one door silently stays open | A path that returns an authenticated result without calling the decision | AC-2 covers every gated door separately. The five paths are named in Current Behavior, and the wiring test drives each one |
| R-3 | A read failure resolves to an empty login set, which the matcher reads as "no restriction" | An empty set reaching the matcher with a nil error | AC-6: a set that could not be read denies and says so. Empty from a FAILED read is never the permissive answer |
| R-4 | The gate is placed after command authorisation, so a refused user still runs a command | A test that asserts the response code but not the side effect | The gate sits in the authentication path. AC-3 asserts that a refused login performs no command |
| R-5 | The login set is checked at boot only, so a reload cannot tighten it | The gate passes after a config edit that should have closed it | The sets live in the accepted generation. AC-5 drives a reload |
| R-6 | A name registered by two components with different meanings | The registry accepts a silent overwrite | Copy the conflict error `RegisterSource` already returns on a re-registration that disagrees |
| R-7 | A gate error that is not `ErrAuthRejected` moves the chain to the next backend, or opens the web fallback, and the login is admitted there | A refused login that succeeds on a daemon with two backends, or on web | The decision returns `ErrAuthRejected` for every refusal, the unreadable case included, and runs outside the chain. The fallback's result passes the decision too. AC-6 and AC-15 |
| R-8 | REST and gRPC share one closure, so one of them is gated under the other's name | An account allowed on REST only that is admitted on gRPC | A-2. AC-17 tests the pair with a set that holds exactly one of the two |
| R-9 | A registry rename changes the value a TACACS+ or RADIUS server receives | A server-side policy keyed on the port value stops matching | The census. `ssh` and `web` are adopted unchanged, and a unit test pins both |
| R-10 | A build without a surface's tag refuses a config that names it, and the operator cannot tell why | "unknown login service" on a name the documentation lists | The refusal says this build does not offer the name and lists the names it does. AC-9 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Two directions, both bad and not symmetric. Too permissive: a surface that should refuse an account admits it, which is the status quo and is at least no worse. Too strict: an operator is locked out of every management surface of a running router, with no way in except physical access. The second is the one to design against |
| How is it reverted? | Single commit revert. The new config section becomes unknown, so a config carrying it must be edited before the older daemon accepts it. Ze is pre-release, so no migration is owed (`ai/META-DECISIONS.md`, "Ze is pre-release") |
| Who else touches this path? | The closed `spec-hub-deferred-api-auth-independent-of-ssh-block` work corrected where the API reads its user list from, in the same area. It is separable and landed first. `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` extends the same design to two more surfaces. `plan/journal/guard-added-to-one-half-of-a-pair.md` holds a row dated 2026-10-31 on `liveAAABundleAuthenticator.Authenticate`; another spec is its key, and this spec edits the same function |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A config declaring a login set and a user referencing it, validated at load | → | the new `ze:validate` validator wired in `RegisterValidators` (`internal/component/config/validators_register.go`) | `test/parse/login-service-unregistered-name.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| An SSH password login by a user whose set excludes ssh | → | `(*Server).authenticatePasswordResult` (`internal/component/ssh/passwordauth.go`), then the wrapper | `test/plugin/login-service-ssh-password.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| An SSH public-key login by the same user | → | `(*Server).authenticatePublicKeyResult` (`internal/component/ssh/pubkey.go`) | `test/plugin/login-service-ssh-publickey.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A `ze cli -c` command by the same user | → | the SSH password path | `test/plugin/login-service-ze-cli.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A web form login by a user whose set excludes web | → | `LoginHandlerWithAudit` (`internal/component/web/auth.go`) | `test/plugin/login-service-web-form.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A web Basic Auth request by the same user | → | `AuthMiddlewareWithAudit` (`internal/component/web/auth.go`) | `test/plugin/login-service-web-basic.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A web request with a cookie issued before a reload removed web from the set | → | `(*SessionStore).validateToken` (`internal/component/web/auth.go`) | `test/plugin/login-service-web-cookie-reload.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A REST request by a user whose set excludes rest | → | `(*RESTServer).withAuth` (`internal/component/api/rest/auth.go`) | `test/plugin/login-service-rest.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A gRPC call by a user whose set excludes grpc | → | `(*GRPCServer).checkAuth` (`internal/component/api/grpc/server.go`) | `test/plugin/login-service-grpc.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## Acceptance Criteria

Each row is driven through the real entry point on a running daemon unless it
names config validation. "Excluded door" and "included door" mean two cases on
ONE daemon with ONE account: the same credential is refused on the door the set
leaves out and admitted on a door it holds.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A component offering a login registers its service name at startup | The name is in the registry before any config validation runs, and the editor completes it. A second registration of the same name by another owner returns an error | A repeat registration by the same owner with the same meaning is accepted, as `RegisterSource` accepts it |
| AC-2 | An account whose login set excludes a door presents a CORRECT credential to it. One case per gated door: SSH password, SSH public key, `ze cli`, web form, web Basic Auth, REST, gRPC | Each door answers with the same status and body it gives the same account with a wrong credential, and the same account with the same credential is admitted on a door its set includes | The stored recovery user (AC-8). A login already open (AC-13) |
| AC-3 | The refused login of AC-2, on a door that would have dispatched a command or created a session | The audit log holds one failed-login record for the attempt, and the daemon's state is as before it: `show` output and the config match their state before the attempt, and the web answer is the login page | None: a refusal that leaves any effect is the defect |
| AC-4 | The same account presents the same credential to a door its set includes | The login succeeds, and a command the account's profile allows returns its normal answer while one it denies is refused as today | None beyond today's command verdicts |
| AC-5 | A running daemon, a config edit removing a door from an account's set, then a reload | The next login on that door is refused with the wrong-credential response, with no restart, and a login on a door still in the set succeeds | A connection opened before the reload (AC-13) |
| AC-6 | The login set of an account cannot be resolved at login: the accepted generation is unreadable, or the reference names a set the generation does not hold | The login is refused with the wrong-credential response, and the daemon log names the account, the service and the reason, once | The stored recovery user, whose result is admitted before any set is read |
| AC-7 | A user naming no login set at all | Every gated door admits it with a correct credential, so an untouched config keeps working after an upgrade (OD-3) | None |
| AC-8 | The stored recovery user, with no config user of its name, on a config where every config user's set excludes every door | It authenticates on SSH by password and by key, on `ze cli`, on web, REST and gRPC, and runs a command on each | None |
| AC-9 | A config naming a login service that this build did not register | Config validation rejects it at load and at commit. The message names the value, says this build does not offer it, lists the names it does offer, and the editor completes only those names | A name registered by a surface compiled into this build |
| AC-10 | A user referencing a login set that is not defined | The commit is rejected by the leafref and the message names the sets that exist, as an undefined profile reference is rejected today | None |
| AC-11 | A login set that names no service, referenced by a user | The config is accepted, and that user is refused on every gated door. This is distinct from AC-7 | The stored recovery user |
| AC-12 | A user a remote backend authenticates (TACACS+ or RADIUS), with no local record, on a daemon that has login sets | Decided by OD-1. The row is written when the owner answers, and it names the door, the backend and the expected answer | Decided by OD-1 |
| AC-13 | An account with an open SSH session, an open gRPC stream and a web session, then a reload removing those doors from its set | The open SSH session and the open gRPC stream keep answering until their connection closes, and the next web request with the old cookie receives the unauthenticated response and loses its session | An open web SSE stream, until the client disconnects (`docs/architecture/web-interface.md`) |
| AC-14 | A config user that takes the stored recovery user's name and references a set excluding ssh | The account is refused on SSH and admitted on a door its set holds: it is a config account (OD-2 option (a)) | None under option (a). The row changes if OD-2 is answered otherwise |
| AC-15 | A daemon with a remote backend and the local backend, and a local account whose set excludes web, presenting its correct password to the web form | The login is refused with the wrong-credential response, and the daemon log shows the gate's reason exactly once for the attempt | The stored recovery user |
| AC-16 | REST or gRPC in shared-token mode, on a config that defines login sets | A request with the token is admitted as today under the shared identity | None: the mode has no account to restrict, and it exists only while no user is declared |
| AC-17 | An account whose set holds `rest` and not `grpc`, and a second account with the reverse | Each account is admitted on its one transport and refused on the other | None |
| AC-18 | The service name sent to a remote backend for an SSH and for a web login | A TACACS+ START carries `ssh` and `web` in its `port` field, and a RADIUS Access-Request carries the same in NAS-Port-Id, as before this spec | None |
| AC-19 | A daemon on which no AAA bundle composed, so SSH answers from its own local authenticator, and an account whose set excludes ssh | The SSH password login is refused with the wrong-credential response, and the stored recovery user is admitted | The stored recovery user |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | creates a monitoring account allowed on the API only, and tries to SSH in with it | config → accepted generation → SSH login → wrapper → decision | `test/plugin/login-service-ssh-password.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 2 | uses that same account against the REST API | config → accepted generation → REST `withAuth` → private authenticator → decision → dispatcher | `test/plugin/login-service-rest.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 3 | mistypes a service name in the config | config load → the new validator → rejection naming the value and the names offered | `test/parse/login-service-unregistered-name.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 4 | removes a door from an account's set on a running router | reload → published generation → next login refused, open cookie refused | `test/plugin/login-service-reload-tightens.ci`, `test/plugin/login-service-web-cookie-reload.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 5 | locks every config account out, then recovers | stored recovery user → decision exempts the result → any door | `test/plugin/login-service-recovery-user.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## 🧪 TDD Test Plan

### Unit Tests
No test sits on `(*Store).Authorize`, which this spec does not edit. The
decision's tests sit beside the decision.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoginServiceRegistryRejectsConflictingRegistration` | `internal/component/config/loginservice/registry_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-1: a second registration that disagrees is an error, not an overwrite | |
| `TestLoginServiceValidatorRejectsUnregisteredName` | `internal/component/config/validators_test.go` | AC-9: an unregistered name fails validation, the message lists the offered names, and completion lists only registered names | |
| `TestExtractAuthUsersCarriesLoginSet` | `internal/component/config/infra/authz_test.go` | The reference reaches the credential from each shape the map form produces | |
| `TestLoginDecisionAbsentSetAdmits` | `internal/component/aaa/login_service_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-7 | |
| `TestLoginDecisionEmptySetRefuses` | the same file | AC-11: an empty set and an absent reference are different values | |
| `TestLoginDecisionUnreadableRefusesAsRejection` | the same file | AC-6: every failure to answer is `ErrAuthRejected`, never another error | |
| `TestLoginDecisionEmptyOrUnregisteredServiceRefuses` | the same file | A request that names no service, or an unregistered one, is refused for an account with a set | |
| `TestLoginDecisionAdmitsRecoveryResult` | the same file | AC-8: a local result carrying the recovery profile is admitted before a set is read, and a remote result carrying the same profile name is not | |
| `TestWrapperGateIsTerminalInChain` | `internal/component/aaa/login_profiles_test.go` | AC-15: after a gate refusal no later backend is called | |
| `TestLiveBundleFallbackPassesDecision` | `cmd/ze/hub/aaa_authenticator_web_test.go` | AC-15: the web fallback's result is gated, and a gate refusal from the bundle opens the fallback for the recovery result only | |
| `TestSSHPublicKeyLoginHonoursLoginSet` | `internal/component/ssh/pubkey_test.go` | AC-2: the gate is not password-specific | |
| `TestSSHLocalAuthenticatorWithoutBundleIsGated` | `internal/component/ssh/passwordauth_test.go` | AC-19 | |
| `TestValidateTokenRefusesWithdrawnWebService` | `internal/component/web/auth_test.go` | AC-13: the per-request re-check | |
| `TestAPIAuthenticationNamesTransport` | `cmd/ze/hub/api_infra_test.go` | AC-17: the closure gates REST and gRPC under their own names | |
| `TestSameLocalCredentialReadsEveryField` | `cmd/ze/hub/rfc8907_aaa_lifecycle_test.go` (existing) | The pinned field count moves with the new field, and a changed reference advances the generation | |
| `TestRegisteredNamesMatchWireValues` | `internal/component/ssh/passwordauth_test.go`, `internal/component/web/auth_test.go` | AC-18: the registered names are `ssh` and `web` | |
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
| `login-service-ssh-publickey` | `test/plugin/login-service-ssh-publickey.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2: the configured key is refused on SSH; with ssh in the set it is admitted | |
| `login-service-ze-cli` | `test/plugin/login-service-ze-cli.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2: `ze cli -c` as that account is refused; as an account whose set holds ssh it answers | |
| `login-service-web-form` | `test/plugin/login-service-web-form.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2, AC-3: the form answers as for a wrong password; an allowed account receives a session cookie | |
| `login-service-web-basic` | `test/plugin/login-service-web-basic.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2: Basic Auth is refused for the excluded account and served for the included one | |
| `login-service-web-cookie-reload` | `test/plugin/login-service-web-cookie-reload.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-13: a cookie issued before the reload is refused after it | |
| `login-service-rest` | `test/plugin/login-service-rest.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2, AC-3, AC-17: refused on REST, admitted on gRPC, no command effect | |
| `login-service-grpc` | `test/plugin/login-service-grpc.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-2, AC-17: the reverse pair, unary and stream open | |
| `login-service-reload-tightens` | `test/plugin/login-service-reload-tightens.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-5, AC-13: a reload closes a door for the next login and an open SSH session keeps answering | |
| `login-service-recovery-user` | `test/plugin/login-service-recovery-user.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-8: the stored recovery user logs in on every door while every config account is refused | |
| `login-service-shadowed-recovery-name` | `test/plugin/login-service-shadowed-recovery-name.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-14 | |
| `login-service-absent-section` | `test/plugin/login-service-absent-section.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-7: a config with no login section admits its user on every door | |
| `login-service-empty-set` | `test/plugin/login-service-empty-set.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-11 | |
| `login-service-chain-terminal` | `test/plugin/login-service-chain-terminal.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-15, AC-6: with a remote backend configured and unreachable, the excluded local account is still refused on web | |
| `login-service-shared-token` | `test/plugin/login-service-shared-token.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-16 | |
| `login-service-unregistered-name` | `test/parse/login-service-unregistered-name.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-9: the refusal text | |
| `login-service-undefined-set` | `test/parse/login-service-undefined-set.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> | AC-10: the leafref refusal names the sets that exist | |

### Interop Tests (Scope: protocol)
No byte on a wire changes (see the census), so no interop scenario is owed for
the design as written. OD-1 can change that: under options (b), (c) or (d) Ze
refuses a login a remote server accepted, which the server observes as an
accepted authentication with no session after it. The row below is owed only
then, and its peer behavior is read in FreeRADIUS's source before the check is
written (`ai/rules/interop-and-goal-validation.md`).

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `radius-admin-login-service-refused-freeradius` | `test/interop-radius/scenarios/` | FreeRADIUS | Owed only under OD-1 (b), (c) or (d): a RADIUS-accepted admin is refused on the door its set excludes and admitted on one it holds | |

## Files to Modify
- `internal/component/authz/yang/ze-authz-conf.yang` - add the named login-set list under `system/authorization` and its user reference, a leafref with `require-instance true`, beside `leaf-list profile`
- `internal/component/config/infra/authz.go` - carry the reference in `ExtractAuthUsers`; extract the login sets beside the policy store; refuse a reserved name
- `internal/component/config/validators.go` - the new validator, copying `RedistributeSourceValidator`
- `internal/component/config/validators_register.go` - wire the new `ze:validate` name
- `internal/component/aaa/types.go` - the login-set reference on `UserCredential`
- `internal/component/aaa/login_profiles.go` - the decision, and its call in `resultAuthorizingAuthenticator.Authenticate`
- `internal/component/ssh/passwordauth.go` - name the service from the registration, not a literal
- `internal/component/ssh/pubkey.go` - call the decision after the key matched
- `internal/component/ssh/ssh.go` - the bare local authenticator takes the wrapper
- `internal/component/web/auth.go` - the per-request re-check in `validateToken`; the service name from the registration
- `internal/component/api/types.go` - `Authentication.Authenticate` learns the calling service
- `cmd/ze/hub/api.go` - `buildAPIAuthentication` passes the service into the request
- `internal/component/api/rest/auth.go`, `internal/component/api/grpc/server.go` - each names itself to `Authenticate`
- `cmd/ze/hub/aaa_lifecycle.go` - the login sets in `acceptedLocalIdentityState`; `sameLocalCredential` reads the new field; `liveAAABundleAuthenticator.Authenticate` gates the fallback result and treats a gate refusal as terminal
- `cmd/ze/hub/rfc8907_aaa_lifecycle_test.go` - `TestSameLocalCredentialReadsEveryField` pins the new field count. The edit raises what the test proves, and is named here because the test pins a number
- `docs/guide/authentication.md` - who may use which door; `ze cli` is an SSH login; what an open session keeps
- `docs/guide/authorization.md` - the new section beside `profile`
- `docs/guide/configuration.md` - the new config section
- `docs/guide/operator-access-rbac.md` - section 8 gains the withdrawn-door case
- `docs/guide/tacacs.md`, `docs/guide/radius.md` - state that the access-channel value is the registered login-service name
- `docs/architecture/api/architecture.md` - "Identity and Authorization": the generation carries login sets, and the closure names its transport
- `docs/architecture/web-interface.md` - "Authentication": the re-check covers the web door
- `docs/architecture/core-design.md` - section 19: the allowed-import row for the new registry package, which `ssh`, `web`, `api` and `aaa` import. The page is declared by the `// Design:` header of `internal/component/authz/authz.go` and `internal/component/aaa/login_profiles.go`
- `docs/architecture/system-architecture.md` - declared by the `// Design:` header of `internal/component/ssh/pubkey.go`. Its account of SSH public-key login gains the gate. The page was not read for this revision, so the implementing phase reads it first and names it unaffected, with the reason, if it says nothing the gate changes
- `rfc/short/rfc8907.md` - a row for the Section 5.1 sentence on documenting the `port` values, pointing at `docs/guide/tacacs.md`

## Files to Create
- `internal/component/config/loginservice/registry.go` - the name registry, copying `internal/component/config/redistribute/registry.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
- `internal/component/config/loginservice/registry_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
- `internal/component/aaa/login_service_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
- one registration per login surface, in the surface's own package: SSH, web, REST, gRPC
- the seventeen `.ci` files named in Functional Tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | The new list and the user reference, both in `internal/component/authz/yang/ze-authz-conf.yang`. The SSH module is not touched |
| YANG validation constraints | Yes | The user reference is a leafref with `require-instance true`. The service leaf is `type string` with `ze:validate`; the registry holds the value set, not the schema |
| YANG custom validators | Yes | The new validator, wired by name in `RegisterValidators`, with `CompleteFn` |
| CLI commands/flags | No | No new verb. The config editor reaches the new section through the schema |
| CLI grammar (keyword before value) | No | No new command |
| Editor autocomplete | Yes | Automatic once the validator supplies `CompleteFn`, as the redistribute source leaf does |
| Functional test for new RPC/API | Yes | The `.ci` files in Functional Tests |
| Pipe completeness | No | No new command output |
| Env var registration | No | No `environment/` leaf. The section lives under `system` |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module, or binary |
| Doctor code per problem class | Yes | A config in which every config account is excluded from every door is a problem an operator can hit. One doctor check reports it with the command that fixes it (`ai/META-DECISIONS.md`, "Goal and scope"); its code and phase are named in the design step |
| Prometheus counters/metrics | Yes | A refused-by-login-set counter, labeled by service, so an operator can see a misconfigured set rather than hunt for it in logs |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The feature declaration that renders `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/guide/authorization.md` |
| 3 | CLI command added/changed? | No | No verb changes |
| 4 | API/RPC added/changed? | No | The surfaces are unchanged. Only who may reach them changes |
| 5 | Plugin added/changed? | No | No plugin is added |
| 6 | Has a user guide page? | Yes | `docs/guide/authentication.md`, `docs/guide/operator-access-rbac.md` |
| 7 | Wire format changed? | No | See the census: the values on the wire are unchanged |
| 8 | Plugin SDK/protocol changed? | No | The registry is in the daemon. No SDK contract is added for an external plugin to offer a login (Known Limitations) |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8907.md`: the Section 5.1 sentence on documenting the `port` values, met by `docs/guide/tacacs.md` |
| 10 | Test infrastructure changed? | No | Existing `test/plugin` and `test/parse` suites |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it claims parity on per-user access control |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md`, `docs/architecture/web-interface.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | The refused-by-login-set counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | A new registry. Add it to the registration inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/release/fixes/spec-login-service-authorisation.md` lists them. Known from reading: `docs/guide/operator-access-rbac.md` and `docs/architecture/web-interface.md` anchor `validateToken`; `docs/architecture/api/architecture.md` anchors `buildAPIAuthentication`, `mergeAuthUsers` and `api.Authentication`; `docs/guide/authentication.md` anchors the REST and gRPC authentication files |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/authentication.md` shows user config examples that now have a further dimension |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove a login reaches a decision point that can refuse it
   - Tests: the nine wiring `.ci` files, written to fail
   - Files: those `.ci` files
   - Verify: each fails because the login SUCCEEDS today, not on a setup error
2. **Phase: The registry** -- names register, and nothing central lists them
   - Tests: `TestLoginServiceRegistryRejectsConflictingRegistration`, `TestRegisteredNamesMatchWireValues`
   - Files: `internal/component/config/loginservice/registry.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->, one registration per surface
   - Verify: AC-1, AC-18. No file outside the registry enumerates the service names
3. **Phase: Config surface** -- the section, the reference, and their validation
   - Tests: `TestLoginServiceValidatorRejectsUnregisteredName`, the two `test/parse` files
   - Files: the authz YANG module, the validator file, the validator registration file, the authz extraction file
   - Verify: AC-9, AC-10, A-8, and CLI completion offers registered names only
4. **Phase: The generation carries the sets** -- one producer, one publication
   - Tests: `TestExtractAuthUsersCarriesLoginSet`, `TestSameLocalCredentialReadsEveryField`
   - Files: `internal/component/config/infra/authz.go`, `internal/component/aaa/types.go`, `cmd/ze/hub/aaa_lifecycle.go`
   - Verify: a changed reference advances the generation; a rejected reload publishes nothing
5. **Phase: The decision** -- one function, fail closed
   - Tests: the five `TestLoginDecision...` tests
   - Files: `internal/component/aaa/login_profiles.go`
   - Verify: AC-6, AC-7, AC-11. Every refusal is `ErrAuthRejected`
6. **Phase: The gate, on all five paths at once** -- never one path at a time
   - Tests: the surface `.ci` files, `TestWrapperGateIsTerminalInChain`, `TestLiveBundleFallbackPassesDecision`, `TestSSHPublicKeyLoginHonoursLoginSet`, `TestSSHLocalAuthenticatorWithoutBundleIsGated`, `TestAPIAuthenticationNamesTransport`
   - Files: the wrapper, the two SSH files, the API types file, `cmd/ze/hub/api.go`, the REST and gRPC call sites, `liveAAABundleAuthenticator.Authenticate`
   - Verify: AC-2, AC-3, AC-4, AC-15, AC-16, AC-17, AC-19. A path left ungated is the failure this phase exists to prevent
7. **Phase: Reload, open sessions and recovery**
   - Tests: `login-service-reload-tightens`, `login-service-web-cookie-reload`, `TestValidateTokenRefusesWithdrawnWebService`, `login-service-recovery-user`, `login-service-shadowed-recovery-name`, `TestPowerUserIgnoresLoginSets`
   - Files: `internal/component/web/auth.go`
   - Verify: AC-5, AC-8, AC-13, AC-14
8. **Phase: Docs, metric, doctor check and discrimination**
   - Tests: every `.ci` above, each run once with the gate reverted, by the main thread
   - Files: the documentation set, the counter, the doctor check, `rfc/short/rfc8907.md`
   - Verify: reverting the gate turns every new `.ci` red. A test that stays green proves nothing

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every gated door in "Login surfaces" is proven separately. One door missing is a hole, not progress |
| Feature completeness | Boot AND reload. A gate a reload cannot tighten is half a gate |
| Correctness | A refused login is indistinguishable from a wrong password to the caller, and distinguishable in the log to the operator |
| Naming | The section names LOGIN, not permission. It decides which door, never which command |
| Data flow | One credential producer, one registry, one decision function, one published generation. No central list of service names anywhere |
| Operator errors and doctor codes | The validator's refusal names the value, the names offered and the fix. The doctor check for "every config account is excluded everywhere" carries its fix command |
| Rule: `ai/rules/evidence.md` | An unreadable login set denies. An empty set from a failed read never reaches the matcher as permissive |
| Rule: `ai/rules/simplicity.md` | The registry and the validator are copies of shapes that exist. A new mechanism here is the smell |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No central list of service names | Grep the tree for any file enumerating the names outside the registry and its registrants |
| Every gated door refuses and admits | The surface `.ci` files pass, and each fails with the gate reverted |
| The recovery path survives | `login-service-recovery-user`, on a running daemon |
| An untouched config still works | `login-service-absent-section` |
| The wire values did not move | `TestRegisteredNamesMatchWireValues` |
| Lint | `./le go lint run scope "<pkg>"` for each package the change edits, run by the main thread |
| Schema and docs | `./le doc check all`, `./le cli grammar` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| What a wrong landing exposes | Permissive: the status quo, an account reaching a surface it should not. Strict: an operator locked out of a running router with no remote way back. Design against the second |
| What proves it did not | Each `.ci` asserts BOTH halves on one daemon: refused where excluded, admitted where included. A refusal-only test cannot tell a working gate from a broken login |
| Fail closed | An unresolvable login set denies. Trace every error return of the decision and confirm each is `ErrAuthRejected` |
| No fall-through | A gate refusal reaches neither the next backend in `ChainAuthenticator` nor the fallback in `liveAAABundleAuthenticator` |
| Every path | SSH public key, the private API authenticator, the web fallback and the SSH server's own local authenticator are gated. None goes through the wrapped live chain |
| Empty is not "no restriction" | A user naming an EMPTY set is denied everywhere. A user naming NO set is admitted everywhere. These must not collapse into one value |
| Guard driven from its entry point | Real logins on a running daemon. Calling the matcher with a hand-made set proves the helper, never the wiring |
| Privilege | The exemption is keyed on the reserved recovery profile from the local backend. Confirm no config edit can give that profile to a user, and that a remote backend's result never carries it into the exemption |
| No user enumeration | A refused login MUST NOT reveal that the account exists. Same status, same body, same timing shape as a wrong credential: the decision runs after the credential check, so the bcrypt cost is paid either way |
| Accounting | A refused login is recorded as a failed login, never as a successful one |
| Stored credential | A cookie issued before a withdrawal is refused on its next request |

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

- The mechanism the owner asked for exists. Route redistribution registers
  source names at `init()`, tags the leaf `ze:validate "redistribute-source"`,
  and lets one validator answer both "is this real" and "what may I offer in
  completion". The schema-enum-versus-validator question is therefore already
  settled in this repository, in favour of the validator.
- The hard half is not the registry. It is that no path asks the question at
  all today, and that there are five paths. The wrapper in
  `internal/component/aaa/login_profiles.go` describes itself as the one choke
  point, and three of the five paths never reach it.
- The service name was already being carried, for a different reader. It went
  to the TACACS+ and RADIUS server as an access-channel label. That makes the
  name a wire value, which the 2026-08-11 text did not know.
- The two halves of "who are you" and "what may you run" are cleanly separated
  today, and this gate belongs in the first. Putting it in the second would let a
  refused user hold a session.
- A refusal has to be the terminal kind. `ChainAuthenticator` and the web
  fallback both read any error other than `ErrAuthRejected` as "this backend
  could not answer, ask another", which is the right reading for a dead server
  and the wrong one for a guard.

## Owned journal rows
No open journal row describes a defect this spec fixes. `plan/journal/` was
searched on 2026-11-14 for the functions this spec edits; the one row found, on
`liveAAABundleAuthenticator.Authenticate`, is keyed to the spec that found it.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A separate named section, not a field on `profile` | Add a service list to the existing profile | The owner's instruction of 2026-08-11. The two answer different questions, and a profile is about commands. Keeping them separate lets one account share a command policy with another while differing on doors |
| Refuse at authentication, not at authorisation | Admit the login and deny every command | The owner's instruction: the username does not work. An admitted session that can do nothing still holds state, still appears in accounting, and still tells an attacker the account exists |
| Names come from a registry, never from the schema | A YANG enum listing the services | A schema enum makes every new login surface a schema edit. The registry is what the owner asked for and what the repo already does |
| A user naming no set reaches every service | Deny by default | Denying breaks every config in the field at upgrade, including locking the operator out of the surface they would use to fix it. OD-3 holds it for the owner to invert |
| One decision function, called on every path | A check in each surface's login function | Five paths exist. A check per surface is the shape that leaves one out, and the wrapper already exists for two of them |
| Every gate refusal is `ErrAuthRejected` | Return the read error as it is | Any other error moves the chain to the next backend and opens the web fallback, which admits the login the gate refused. Fail closed is plain correctness, not a choice |
| The registered names are `ssh`, `web`, `rest`, `grpc` | New names | `ssh` and `web` are already sent to TACACS+ and RADIUS servers and documented in the guides. `rest` and `grpc` match the audit surface names an operator already reads |
| The user reference is a leafref | A string compare in `ValidateAuthzConfig` | The profile reference moved to a leafref since this spec was written, and its refusal names the entries that exist. The sibling takes the same form |
| The sets live in the accepted identity generation | Read the config provider at login | The provider holds a rejectable candidate during a reload (`liveConfigUsers`, `cmd/ze/hub/main_servers.go`). A gate that read it would enforce a config the reload then refuses |
| An open session outlives a withdrawn door; the web cookie does not | End every live session of the account at reload | `docs/guide/operator-access-rbac.md`, section 8, states this rule for removing a user, and a withdrawn door is a narrower form of the same edit. Departing from the page needs the owner |
| The exemption is keyed on the reserved recovery profile | Key it on the stored username | The key exists and three functions read it. The documented design says only a surviving stored user holds it. OD-2 asks about the one consequence |
| `ze cli` is gated as `ssh` | A service name of its own | It is an SSH login on the SSH listener: the daemon cannot tell it from another SSH client on loopback, and a separate name would be a claim the server cannot check |
| The serial console is not gated | Read the config from `loginMain` | It runs with no daemon and must work when the config is the thing that is broken |

## Known Limitations
- The looking glass and gNMI are not covered. Both authenticate with a shared
  token and learn no username, so they cannot answer the question until they
  have identity. That is
  `plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md`.
- MCP keeps its own `Identity` scope model (OD-4). Unifying the two is not
  attempted here.
- REST and gRPC authenticate against local users only. A TACACS+ or RADIUS
  operator has no REST or gRPC login today, with or without this spec
  (`buildAPIAuthentication`, `cmd/ze/hub/api.go`). This spec does not change
  that.
- A user a remote backend authenticated is covered only as far as OD-1 decides.
- An external plugin cannot offer a gated login. The registry is in the daemon
  process, and no SDK contract carries a login offer across the plugin
  boundary. Adding one is a protocol change with its own design.
- A build without a surface's tag refuses a config that names that surface's
  service. The refusal says why (AC-9). A config shared between two builds
  names only what both offer.
- Three other lists of service names exist and disagree: the audit constants
  (`internal/core/audit/audit.go`), `registerService`
  (`cmd/ze/hub/service_registry.go`), and `DiscoverListenerServices`
  (`internal/component/config/listener.go`). This spec adds a registry that is
  narrower on purpose, holding LOGIN services only. Reconciling the other three
  is not in scope and MUST NOT be attempted here.

## RFC Documentation (Scope: protocol)

The code that sends the service name to a TACACS+ server carries, above the
statement, RFC 8907 Section 5.1: "The client documentation SHOULD define the
values and their meanings for this field." The comment names
`docs/guide/tacacs.md` as that documentation.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-19 all demonstrated, AC-12 once OD-1 is answered
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests of `aaa`, `authz`, `ssh`, `web`, `api`, `config` and `cmd/ze/hub`, through `./le test unit pkg path <dir>/`, and the named files through `./le test functional plugin` and `./le test functional parse`. No agent runs a whole-tree verification: GitHub CI verifies the pushed commit
- [ ] Feature code integrated on every gated door in "Login surfaces", not test-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] The owner has ruled on every row under "Owner decisions open"
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A unless OD-1 selects option (b), (c) or (d))

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-login-service-authorisation.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
