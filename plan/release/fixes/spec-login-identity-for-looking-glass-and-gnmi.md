# Spec: give the looking glass and gNMI a per-user login

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P0 |
| Release reason | Management access must require the approved explicit public choice or per-user authorization and its login-service gate. |
| Scope | config |
| Depends | `plan/release/fixes/spec-login-service-authorisation.md` |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Two management surfaces authenticate with a shared secret and never learn who
the caller is.

`bearerAuth` (`internal/component/lg/auth.go`) returns the handler unchanged
when the configured token is empty, so a looking glass with no token is fully
public. With a token set, every caller presents the same string. There is no
user, no session and no identity, and `(*LGServer).query`
(`internal/component/lg/server.go`) dispatches every command with a zero-value
caller.

`(*Server).Serve` (`internal/component/gnmi/server.go`) installs its two
authentication interceptors only when a token is configured, and
`(*Server).checkAuth` returns nil for an empty token. So a gNMI server with no
token authenticates nobody, and with a token every caller presents the same
string. `(*Server).Set` (`internal/component/gnmi/set.go`) then enters, edits
and commits a config session under the constant name `gnmi`, whoever called.

The owner's requirements, given 2026-08-11:

- The looking glass MUST carry an explicit choice between open and requiring a
  user. Today "open" is what an operator gets by leaving a leaf empty, which is
  a default rather than a decision.
- gNMI MUST behave like SSH, which means a per-user login against the same
  credentials every other surface uses.

Once each surface knows who the caller is, it registers its login-service name
and takes part in the login-set gate that
`plan/release/fixes/spec-login-service-authorisation.md` builds. That spec
covers the four surfaces that already carry a username; this one supplies the
identity the other two lack.

The spec was reviewed against the tree on 2026-11-14. The code under it had
moved since 2026-08-11, and four things the first version assumed are false
today. They change the design, so they are stated here and detailed below:

1. "gNMI copies the gRPC surface" does not give "gNMI behaves like SSH". The
   gRPC surface authenticates against a fixed local user slice
   (`buildAPIAuthentication`, `cmd/ze/hub/api.go`). SSH authenticates through
   the live AAA chain (`liveAAABundleAuthenticator`,
   `cmd/ze/hub/aaa_lifecycle.go`). The owner's words decide: gNMI uses the
   chain.
2. gNMI serves no commands. Get and Subscribe read the config tree, and Set
   drives a config session. "Passes through `(*Store).Authorize`" therefore
   has to name the command each RPC is authorized as.
3. A non-loopback gNMI listener with no token is already refused, at
   validation and at boot. The criterion and the `.ci` that expected this to
   become newly reachable could never go red against HEAD.
4. The looking glass mode design contradicted itself, and four choices in it
   belong to the owner. They are in "Owner decisions open".

Both this spec and the gate it depends on now sit in `plan/release/fixes/`.
The earlier paragraph that placed them in different buckets described a layout
the tree no longer has. The dependency is unchanged: neither surface is
complete here without joining that gate, so the gate spec lands first.

## Owner decisions open

Each row is a choice no RFC, agreed design or rule in `ai/META-DECISIONS.md`
answers. The design below is written so that each row changes only the cells
named in its last column. No phase that a row blocks starts before the row is
answered.

| ID | Decision | Evidence | Options | What each changes |
|----|----------|----------|---------|-------------------|
| D-1 | What a looking glass does when no auth mode is stated | The owner asked for "an explicit choice between open and requiring a user". Today the absent token leaf means open (`bearerAuth`). The looking glass can start from the environment alone (`ze.looking-glass.enabled`, `runYANGConfig` in `cmd/ze/hub/main.go`), so "no mode stated" is reachable with no config block at all. Pre-release, repository configs are converted directly, so no option needs a compatibility path | (a) no default: a looking glass that starts with no mode is refused, at validation and at boot. (b) default open. (c) default user | (a) every looking-glass config and test in the repository gains the leaf; AC-17 expects a refusal. (b) only configs that set `token` change; AC-17 expects an open looking glass, and "open" stays reachable by omission. (c) a config with no mode and no users serves nobody; AC-17 expects 401 on every route |
| D-2 | The credential a browser presents in user mode | The HTML pages and the `/lg/events` stream are loaded by a browser, which cannot attach a bearer header to a page load or to an event stream. The same limit already applies to today's token mode: with a token set, a plain browser gets 401 on `/lg/` (`bearerAuth` wraps the whole mux, `NewLGServer`). The web UI solves it with a login form and a session cookie (`SessionStore`, `internal/component/web/auth.go`); REST solves it for programs with `Bearer user:password` (`buildAPIAuthentication`) | (a) HTTP Basic on every route: the browser prompts natively, and programs send the same header. (b) a login form and a session cookie as the web UI has, plus a header form for programs. (c) `Bearer user:password` only: programs work, a plain browser does not, as in token mode today | (a) the password is verified on every request, so AC-13's limit and a verification cache carry the cost; no logout. (b) the session store has to be shared with `web` or rebuilt, which the import table (`docs/architecture/core-design.md`, section 19) does not allow today; AC-16 gains a login step and revocation follows the web rule. (c) user mode protects the API only, and the guide has to say the pages need a proxy |
| D-3 | What user mode does to the birdwatcher API | `/api/looking-glass/*` is registered on the same mux as the pages (`(*LGServer).registerRoutes`), so one gate covers both. Alice-LG polls that API as a program. `docs/architecture/api/birdwatcher-compat.md` states no authentication behavior. Whether Alice-LG can send a credential was not read in its source | (a) the API sits under the same per-user gate, and the client presents a user's credential. (b) the API takes the shared token while the pages take users. (c) the API stays open while the pages take users | (a) one gate, and Alice-LG needs an account; settle first whether Alice-LG can send it. (b) two credentials on one listener, and the mode leaf has to express the pair. (c) the gate no longer covers every route, so the route data stays public and only the pages are closed |
| D-4 | Whether a looking-glass user is authorized per command, or only admitted | `(*LGServer).query` dispatches with `plugin.CallerIdentity{}`. `serverDispatcher` (`cmd/ze/hub/main_servers.go`) passes that empty username to the dispatcher's authorizer. Every other surface passes the user, and `(*Store).Authorize` then applies the user's profile | (a) admitted only: every admitted user sees everything the looking glass serves, dispatched under one reserved looking-glass identity. (b) per command: each query is dispatched as the user, and a profile can deny it | (a) a profile has no effect on the looking glass; AC-19 asserts the reserved identity. (b) a profile that lacks the looking glass's `show` commands turns its pages into refusals, so the guide has to list those commands; AC-19 asserts a denied query answers a refusal |
| D-5 | Whether gNMI may serve clear text on loopback | The gNMI authentication text (see Required Reading) says "Fallback to unencrypted communication is prohibited." `(*Server).Serve` serves gRPC without TLS when neither TLS file is named, and `test/plugin/mgmt-guard-loopback-allowed.ci` relies on a loopback listener. This spec requires TLS wherever a credential crosses a non-loopback listener; loopback clear text is the part no rule here settles | (a) keep clear text on loopback. (b) gNMI always serves TLS | (a) nothing more changes, and the guide records the divergence from the OpenConfig text. (b) every loopback gNMI test and local tool carries a certificate, and `none` mode over clear text disappears |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - "Identity and Authorization" and "gNMI Transport"
  → Constraint: REST and gRPC classify a caller three ways, and the table in "REST and gRPC authentication flow" is the contract: per-user (the username and its authorizer), shared token (`ReservedSharedAPIUsername`, read and write), no auth (`ReservedSharedAPIUsername`, read-only).
  → Constraint: a `ze:sensitive` leaf reaches a gNMI client in its `$9$` form. That form is an encoding, not a protection (see Key insights).
- [ ] `docs/architecture/web-interface.md` - "Looking Glass", "LG Bearer Token", "LG Data Access", and the session rule under authentication
  → Constraint: the looking glass holds no RIB or peer import; all data comes through the dispatcher.
  → Decision: a web session the local backend granted is re-checked against the declared users on every request; a session a remote backend granted is not, "because that list never granted it". The stream rules below copy that split.
- [ ] `docs/architecture/api/birdwatcher-compat.md` - the looking-glass JSON contract Alice-LG depends on
  → Constraint: the page is normative for endpoints and bodies and says nothing about authentication. D-3 adds that statement to it.
- [ ] `docs/architecture/core-design.md`, section 19 - allowed imports per component
  → Constraint: `web` may import `aaa`, `audit`, `authz`; `api` may import `audit`. The table has no `lg` or `gnmi` row, so the imports this spec adds are stated in that table in the same work.
- [ ] `ai/doors/access-security.md`, `ai/doors/api-surfaces.md`, `ai/doors/system.md`
  → Constraint: "Login is not an SSH feature." Surfaces read one accepted generation of users, and removing a user takes effect at once on every credential surface.
  → Constraint: "Each surface carries its own authentication and TLS. A listener that serves without it is a security defect, not a default."
  → Constraint: always-on hub code reaches `gnmi` and `lg` only through their gated seams (`gnmi_infra.go`, `service_lg.go`).
- [ ] `ai/rules/evidence.md` - both auth paths are guards
  → Constraint: a surface that cannot resolve its mode MUST refuse to serve rather than serve unauthenticated. What "cannot resolve" covers for the looking glass is D-1.
- [ ] `ai/rules/config.md` - the YANG and env var rules for the new mode leaves
  → Constraint: an `environment/` leaf needs a matching `ze.<service>.<leaf>` registration, as the existing tokens have in `internal/component/config/environment.go`.
- [ ] `ai/META-DECISIONS.md`, "Ze is pre-release"
  → Decision: "A change to Ze's own config syntax converts the configs in the repository directly. No `ze config migrate` transformation, compatibility path or migrate hint is written." So no mode is inferred from the token leaf to keep an old config working.
- [ ] `docs/guide/authentication.md`, `docs/guide/gnmi.md`, `docs/guide/looking-glass.md`, `docs/guide/looking-glass-howto.md`
  → Constraint: each states today's token-or-open behavior and becomes wrong with this change.

### Protocol texts (no RFC; the specification is OpenConfig's)
- [ ] OpenConfig gNMI authentication, `rpc/gnmi/gnmi-authentication.md` in the `openconfig/reference` repository. The text is not in this repository. It was fetched on 2026-11-14 through a fetch tool that returns extracts, so each quote below MUST be re-read against the stored text before a code comment quotes it. The implementation phase stores the text first (`reference/README.md` governs where).
  → Constraint: "All communication between gRPC client and server must be encrypted by TLS (TLS 1.2)."
  → Constraint: "Fallback to unencrypted communication is prohibited."
  → Constraint: "Configuration operations carried over the encrypted connection will carry credentials (username/password) in the metadata."
  → Constraint: "Telemetry Subscribe() RPCs require a username/password credential inside the metadata."
  → Constraint: "Each telemetry message will not be authenticated with a username/password." The stream is authenticated when it opens, which is why revocation needs its own criterion (AC-9).
- [ ] gnmic 0.49.1, the client the lab runs (`gnmicImage`, `internal/component/gnmi/gnmic_docker_test.go`). `(*Target).appendCredentials` (`pkg/api/target/target.go` in the gnmic repository, read through the same fetch tool) adds the metadata keys `username` and `password` when `--username` and `--password` are given and no `--auth-scheme` is set. `--token` sends `authorization: Bearer <token>`, which `TestGnmicInterop` already proves against Ze.
  → Decision: the per-user credential on gNMI is the pair of metadata keys `username` and `password`. The gNMI text asks for it and gnmic sends it.

**Key insights:** (minimal context to resume after compaction)
- `ExtractAuthUsers` is in `internal/component/config/infra/authz.go`, not `ssh.go`, and it is a pure extractor over a resolved `system` map. It is not a live source. The live sources are the atomic AAA bundle slot that `liveAAABundleAuthenticator` reads on every call, and `LocalUsersFunc` (`internal/component/aaa/types.go`).
- `aaa.AuthRequest` already carries `Service`. SSH sets `"ssh"` (`internal/component/ssh/passwordauth.go`) and web sets `authServiceWeb` (`internal/component/web/auth.go`); TACACS+ and RADIUS read it. gNMI and the looking glass each need only to set their own name.
- Identity is established when a stream opens and never again: `(*Server).authStreamInterceptor` for gNMI, the wrapper around the mux for `/lg/events`. A stream opened before a user is removed keeps delivering.
- The `$9$` form is reversible by anyone. `internal/component/config/secret` documents it as "an obfuscation (not encryption)", `secret.Decode` takes no key, and `reversibleSecret` (`internal/component/config/secret_text.go`) sends every `ze:sensitive` leaf that is not `ze:bcrypt` through it. So an admitted gNMI reader recovers every such secret in clear. Reading config over gNMI is therefore authorized as reading the running config, never as a harmless read.
- No audit surface exists for either: `internal/core/audit/audit.go` names `api`, `cli`, `grpc`, `mcp`, `rest`, `ssh`, `system` and `web`. The looking glass dispatches through the web dispatcher (`webDispatch`, `runYANGConfig`), so its queries are recorded as `web`.
- Neither surface re-reads its authentication on reload. `gnmiBuildImpl` (`cmd/ze/hub/service_gnmi.go`) and `NewLGServer` read the token once, gNMI has no auth reloader, and the looking glass is deliberately absent from `registerMgmtAuthReloaders` (`cmd/ze/hub/mgmt_auth_reload.go`). Read, not run: a reload that changes either token is accepted and not applied.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/lg/auth.go` - `bearerAuth` returns `next` unchanged for an empty token; otherwise it compares sha256 digests with `subtle.ConstantTimeCompare` and answers 401 with `WWW-Authenticate: Bearer realm="looking glass"`. `bearerTokenMatches` fails closed on a missing header, a wrong scheme and a bare token. No username is parsed.
- [ ] `internal/component/lg/server.go` - `NewLGServer` builds one handler chain, security headers over `errorfragment.Middleware` over `bearerAuth` over the mux, so the gate covers the pages, the assets, `/lg/events` and `/api/looking-glass/*` alike. `(*LGServer).query` dispatches with `plugin.CallerIdentity{}`. `LGConfig.Token` is read once at construction; `Reconfigure` moves listen addresses only.
- [ ] `internal/component/lg/handler_ui.go` - `(*LGServer).handleUIEvents` holds one long-lived response and pushes peer state every five seconds until the request context ends. Nothing re-checks the caller after the first request.
- [ ] `internal/component/lg/yang/ze-lg-conf.yang` - `environment/looking-glass` holds `enabled`, a `server` list, `tls` (default true), `certificate` and `token`. The token description states "An empty token leaves the looking glass open, and that is the default".
- [ ] `cmd/ze/hub/service_lg.go` - `buildLGService` passes `deps.LGToken` and the web dispatcher. When TLS is on by default, no certificate is named and the daemon has no store, it prints a warning and serves plaintext; an explicit `tls true` in that state is an error. `tls false` serves plaintext on purpose.
- [ ] `cmd/ze/hub/service_registry.go`, `cmd/ze/hub/register_lg.go` - `serviceDeps` hands every factory generic inputs: the dispatcher, `Recorder`, `Authorizer`, `LocalUsersLive`, and for the looking glass `LGAddrs`, `LGTLS`, `LGToken`, `LGCertificate`. The `ze_lg` registration wires the listener migrator and the certificate rotation, and no authentication reloader.
- [ ] `cmd/ze/hub/main.go` - `runYANGConfig` resolves the looking-glass token from `ze.looking-glass.token`, then from the block (`ExtractLGSettings`). It declares gNMI to the boot guard with `authenticated: gnmiToken != ""`, so a non-loopback gNMI listener with no token already stops the daemon. gNMI is absent from `markMgmtAuth` because it has no listener-migrator slot. The looking glass is never declared to the guard.
- [ ] `cmd/ze/hub/mgmt_guard.go` - `checkMgmtListeners` refuses an unauthenticated non-loopback declaration and names no service. It judges authentication only, not TLS.
- [ ] `cmd/ze/hub/mgmt_auth_reload.go` - `registerMgmtAuthReloaders` registers web, MCP, REST and gRPC. `mcpAuthReloader` refuses a reload that changes MCP authentication or TLS, naming the restart. gNMI and the looking glass have no reloader.
- [ ] `internal/component/gnmi/server.go` - `Config` holds `ListenAddr`, `Token`, `CertFile`, `KeyFile`. `Serve` adds `authUnaryInterceptor` and `authStreamInterceptor` only when the token is not empty, and serves gRPC without TLS when neither TLS file is named. `checkAuth` requires `authorization: Bearer <token>`. The server holds no authorizer, no audit recorder and no identity.
- [ ] `internal/component/gnmi/tls.go` - `loadTLSCredentials` refuses a half-named pair, an unreadable file and an invalid pair with an operator error, before anything listens.
- [ ] `internal/component/gnmi/set.go` - `Set` calls `Enter`, `SetSegments`, `DeleteSegments` and `Commit` on the `api.ConfigSessionManager` under the constant `gnmiUsername`. It consults no authorizer. It does not reach the plugin `Dispatcher`, so the dispatcher's accounting hook (`(*Dispatcher).BeginAccounting`) never sees a gNMI write.
- [ ] `internal/component/gnmi/get.go`, `internal/component/gnmi/subscribe.go` - `Get` and `handleSubscribeOnce` read the running tree through `zeconfig.EncodeSecrets`. `handleSubscribeStream` forwards every `ChangeNotifier` notification until the stream context ends. Neither takes the caller into account.
- [ ] `internal/component/gnmi/metrics.go`, `internal/component/gnmi/show.go` - three metrics, `ze_gnmi_requests_total`, `ze_gnmi_errors_total` and `ze_gnmi_subscribe_active`; `show gnmi` reports `token-set` and `tls-configured` from `(*Server).Status`.
- [ ] `cmd/ze/hub/gnmi_infra.go`, `cmd/ze/hub/service_gnmi.go` - `resolveGNMIListeners` is the one resolver of gNMI's address, token and start, used by the guard and by `gnmiBuildImpl`. `gnmiBuildInputs` carries the tree, the store, the config path and the reload hook, and no authentication input. Only the first `server` entry is bound.
- [ ] `internal/component/config/loader_extract.go` - `GNMIListenConfig.Validate` refuses a non-loopback listener with no token and requires nothing else. `extractGNMIBlock` and `extractLGBlock` parse the blocks; `LGListenConfig` has no validation.
- [ ] `internal/component/config/validate_semantic.go` - `ValidateSemantics` reports the MCP and gNMI validations as `config-mcp-invalid` and `config-gnmi-invalid` (`internal/core/diagnostic/codes.go`), which `ze config validate` and `ze doctor` print. It has no looking-glass entry.
- [ ] `internal/component/config/environment.go` - registers `ze.gnmi.enabled`, `listen`, `token`, `tls.cert`, `tls.key` and `ze.looking-glass.listen`, `enabled`, `tls`, `token`, `certificate`. No mode variable exists.
- [ ] `internal/component/gnmi/yang/ze-gnmi-conf.yang` - `environment/gnmi` holds `enabled`, `token`, `tls { cert; key }` and a `server` list. The token description states "An empty token means that the gNMI server does no authentication."
- [ ] `internal/component/api/grpc/server.go` - `checkGRPCListenAddr` requires authentication and TLS on a non-loopback listener. `(*GRPCServer).checkAuth` reads one `api.Authentication` generation per RPC, marks a no-auth caller `ReadOnly`, and records an `auth-fail` audit entry on refusal. `(*zeConfigServiceImpl).requireWriteAccess` refuses a read-only caller, then asks the caller's authorizer about `api.ConfigAuthEdit`, `ConfigAuthSet`, `ConfigAuthDelete`, `ConfigAuthCommit` or `ConfigAuthDiscard`. `CommitSession` records a `config-commit` audit entry under the caller's name. `GetRunningConfig` dispatches `show config running` as the caller.
- [ ] `cmd/ze/hub/api.go` - `buildAPIAuthentication` builds an `authz.LocalAuthenticator` over a fixed user slice and accepts `Bearer user:password`, or compares one shared token and returns `ReservedSharedAPIUsername`. It does not use the AAA chain, so a RADIUS or TACACS+ user cannot authenticate on REST or gRPC.
- [ ] `cmd/ze/hub/aaa_lifecycle.go`, `cmd/ze/hub/infra_setup.go`, `cmd/ze/hub/service_web.go` - `liveAAABundleAuthenticator` reads the atomic bundle slot on every call. SSH is built with it and no fallback; web is built with it and a local fallback over the live local users. `liveAAABundleAuthorizer` fails closed when no bundle exists.
- [ ] `internal/component/config/infra/authz.go`, `internal/component/config/infra/ssh.go` - `ExtractAuthUsers` is in `authz.go`: it reads `system/authentication/user` from a resolved map and returns sorted credentials, and a shapeless input returns no users. `ssh.go` holds `ExtractSSHConfig`, the listener settings, and no user extraction.
- [ ] `internal/component/authz/authz.go` - `(*Store).Authorize` denies an empty username and allows `ReservedSharedAPIUsername`. Read, not run: on a daemon whose bundle carries a profile store, a looking-glass query, which carries an empty username, reaches that denial. A-6 settles it by test.
- [ ] `internal/component/authz/auth.go` - `(*LocalAuthenticator).Authenticate` runs a bcrypt comparison against a dummy hash for an unknown user, so the unknown-user and wrong-password paths cost the same. No package on the login path limits or delays repeated failures.
- [ ] `internal/core/audit/audit.go` - the surface names are eight constants, and `Entry` carries actor, remote address, surface, action, detail and outcome.
- [ ] `internal/component/gnmi/gnmic_docker_test.go` - `TestGnmicInterop` drives gnmic 0.49.1 over TLS with `--token`, and asserts a missing and a wrong token are refused. No test sends `--username`.
- [ ] `internal/test/fixture/lg_pki_fixture.go`, `internal/test/fixture/plugin_fixture_09_shell.go` - both assert the token gate with no header, a wrong token and the right token on `/api/looking-glass/status` and `/lg/peers`. The second embeds its own daemon config.

**Behavior to preserve:**
- The looking glass stays able to be genuinely public, because that is what it is for. The change makes that a stated choice, never a removed capability.
- The shared-token mode stays available on both surfaces. This spec adds a per-user mode; it does not delete the token.
- The constant-time digest comparison stays on both token paths.
- A non-loopback gNMI listener with no authentication stays refused at validation and at boot (`test/plugin/mgmt-guard-gnmi-nonloopback-refused.ci`).
- `checkMgmtListeners` keeps its verdicts for every surface it already judges, and still names no service.
- gNMI TLS keeps its rule: naming either file asks for TLS, and the server serves TLS or nothing (`internal/component/gnmi/tls.go`).
- The birdwatcher endpoints, envelope and bodies are unchanged in every mode in which a request is admitted.
- A `ze:sensitive` leaf keeps reaching a gNMI client in its `$9$` form, and a Set notification keeps the same form.

**Behavior to change:**
- Each surface gains an explicit `auth-mode` leaf and its environment variable. No mode is inferred from the token leaf.
- gNMI gains a per-user mode that authenticates through the live AAA chain, as SSH does, with the metadata keys `username` and `password`. It is the mode an unset leaf selects.
- A gNMI caller has an identity in every mode, and each RPC is authorized against it.
- A gNMI caller that presented no credential, which only a loopback listener in `none` mode admits, is read-only. Today that caller can Set and commit.
- A credential, token or password, never crosses a non-loopback listener in clear text on either surface.
- A gNMI commit is audited under the caller's name, and a refused login is audited as a failure.
- A stream opened by a user who is later removed ends.
- A reload that changes either surface's authentication is applied or refused, never accepted and ignored.
- Both surfaces register a login-service name and take part in the login-set gate.
- The looking glass gains a per-user mode. Its credential form, its default, its effect on the birdwatcher API and its authorization depth are D-1 to D-4.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The live AAA chain | `cmd/ze/hub/aaa_lifecycle.go` `liveAAABundleAuthenticator`, `liveAAABundleAuthorizer` | Authenticates SSH and web logins against RADIUS, TACACS+ and local users, re-reading the accepted bundle on every call, so a removed user or a rotated backend takes effect with no restart | Extends: gNMI user mode and looking-glass user mode authenticate through it. This is what "behave like SSH" means, and it is why a removed user stops authenticating on both with no new reload code |
| The service name on a login | `internal/component/aaa/types.go` `AuthRequest.Service` | Carries `ssh` or `web` to the backends, which report the channel to RADIUS and TACACS+ | Extends: gNMI and the looking glass set their own names. The login-set gate of the sibling spec reads the same field |
| The API authentication generation | `internal/component/api/types.go` `Authentication`, `AuthenticationProvider`; `cmd/ze/hub/api.go` `buildAPIAuthentication` | One immutable generation per request for REST and gRPC, over a fixed local user slice or one shared token | Does not apply to user mode: it cannot reach RADIUS or TACACS+ users, and its `Authenticate` takes one header string where gNMI carries two metadata keys. Its shared-token branch is the model for token mode's identity |
| The caller identity | `internal/component/plugin/dispatch.go` `CallerIdentity` (`api.CallerIdentity`) | Carries `Username`, `RemoteAddr`, `Surface`, `ReadOnly` and the accepted `Authorizer` from the transport to dispatch | Extends: each gNMI RPC and each looking-glass query carries one. It replaces the constant `gnmiUsername` and the zero-value caller in `(*LGServer).query` |
| The three caller classes | `docs/architecture/api/architecture.md`, "REST and gRPC authentication flow"; `(*GRPCServer).checkAuth` | Per-user, shared token as `ReservedSharedAPIUsername` with read and write, no auth as the same name read-only | Extends: gNMI's three modes map to the same three classes, so one table describes REST, gRPC and gNMI |
| Config write authorization | `internal/component/api/grpc/server.go` `requireWriteAccess`; `internal/component/api/types.go` `ConfigAuthSet`, `ConfigAuthDelete`, `ConfigAuthCommit` | Refuses a read-only caller, then authorizes the named config command against the caller's authorizer | Extends: gNMI Set asks the same question with the same command names. The decision moves to `internal/component/api`, beside the names, so gRPC and gNMI call one function |
| Running-config read authorization | `(*zeConfigServiceImpl).GetRunningConfig` | Dispatches `show config running` as the caller, so the dispatcher authorizes and accounts the read | Extends: gNMI Get and Subscribe are authorized as that command. The command name is declared once in `internal/component/api`, beside the config names |
| Non-loopback listener rule for gRPC | `internal/component/api/grpc/server.go` `checkGRPCListenAddr` | Requires authentication and TLS on a non-loopback listener | Extends: gNMI and the looking glass apply the same two conditions. gNMI applies only the first today, and the looking glass applies neither |
| The boot exposure guard | `cmd/ze/hub/mgmt_guard.go` `checkMgmtListeners`; `runYANGConfig` declarations | Refuses an unauthenticated non-loopback management listener before anything binds | Extends: gNMI's declaration computes `authenticated` from the resolved mode. The guard function is not edited |
| The reload exposure reloaders | `cmd/ze/hub/mgmt_auth_reload.go` `registerMgmtAuthReloaders`, `mcpAuthReloader` | Re-answer each surface's authentication from a reloaded tree; MCP refuses a change it cannot apply | Extends: gNMI and the looking glass register one each, so a mode or token change is refused with the restart instruction. The comment that calls the looking glass deliberately absent is corrected |
| The generic service inputs | `cmd/ze/hub/service_registry.go` `serviceDeps`; `cmd/ze/hub/gnmi_infra.go` `gnmiBuildInputs` | Hand each gated factory generic values: the dispatcher, the audit recorder, the authorizer, the live local users | Extends: the looking-glass factory reads the recorder and the live users that are already there and gains the chain function; `gnmiBuildInputs` gains the chain function and the recorder. No field names a surface type |
| MCP's auth-mode leaf | `internal/component/mcp/yang/`, `MCPListenConfig.Validate` (`internal/component/config/loader_extract.go`), `mcpListenerAuthenticated` | An enumeration that selects the authenticator, validated against the token and the bind address | Extends the shape: an enumeration leaf, validated against its companion leaves. Does not copy MCP's inference of `bearer` from a set token, which exists "for backward compatibility" that pre-release rules out |
| The audit recorder | `internal/core/audit` `Recorder`; `(*GRPCServer).recordAuthFailure`, `(*zeConfigServiceImpl).recordAudit` | Records `auth-fail` and `config-commit` entries with actor, remote address and surface | Extends: gNMI holds a recorder and writes the same two entries under a `gnmi` surface. The looking glass writes `auth-fail` under its own surface |
| Web session re-check | `internal/component/web/auth.go` `(*SessionStore).validateToken`, `localUserDeclared` | Ends a locally granted session when its user leaves the running config; leaves a remotely granted one alone | Extends the rule to streams: gNMI STREAM and `/lg/events` re-check a locally granted identity. The session store itself applies only under D-2 option (b) |
| The gNMI change notifier | `internal/component/gnmi/subscribe.go` `ChangeNotifier.NotifyConfigReload` | Sends every STREAM subscriber a notification after any config reload | Extends: that delivery is the point at which a stream's identity is re-checked, so a reload that removes a user ends that user's stream |
| Dummy-hash comparison | `internal/component/authz/auth.go` `(*LocalAuthenticator).Authenticate` | Makes an unknown user cost the same as a wrong password | Does not change. Both surfaces reach it through the chain, so neither may answer before calling the chain |
| gNMI config validation | `GNMIListenConfig.Validate`; `ValidateSemantics` (`internal/component/config/validate_semantic.go`); doctor code `config-gnmi-invalid` | Reports a non-loopback tokenless gNMI listener offline | Extends: it judges the mode, its companion leaves and TLS. The looking glass gains the equivalent validation and entry, which it lacks |
| The login-service registry | `plan/release/fixes/spec-login-service-authorisation.md` (not built yet) | Will hold the registered login-service names and gate a login on the account's set | Extends: this spec registers `gnmi` and `looking-glass`. It builds no registry of its own |
| The `$9$` secret form | `internal/component/config/secret_text.go` `EncodeSecrets`, `SecretValueAtPath` | Publishes each `ze:sensitive` value in a reversible encoding | Does not change. The spec treats a gNMI read as a read of the running config for authorization, because the encoding hides nothing |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A gRPC call to a gNMI service method, carrying call metadata: `username` and `password`, or `authorization: Bearer <token>`, or nothing.
- An HTTP request to a looking-glass route, carrying the credential D-2 selects, a bearer token, or nothing.
- A config tree read at boot or at reload, and the environment, carrying each surface's `auth-mode`.

### Transformation Path
1. Each surface's resolver answers the mode from the environment variable first and the config leaf second, the precedence the token already has (`resolveGNMIListeners`; the looking-glass block in `runYANGConfig`). gNMI with no mode stated resolves to `user`. The looking glass with no mode stated follows D-1.
2. Validation judges the resolved settings before anything binds: an unknown mode, a token leaf without token mode, token mode without a token, `none` on a non-loopback listener, and a credential mode on a non-loopback listener without TLS are each refused with the fix.
3. The hub hands the gated builder an authentication function over the live AAA chain and an audit recorder. Neither surface holds a user list or imports a backend.
4. A request arrives. In user mode the surface builds an authentication request naming itself as the service and calls the chain. In token mode it compares digests and takes the shared API identity. In `none` (gNMI) or `open` (looking glass) it takes a read-only identity without a credential.
5. A refused login answers the surface's one refusal for every cause, writes an `auth-fail` audit entry with the attempted name, and counts toward the refused-login limit for that remote address.
6. An admitted gNMI call is authorized per RPC: Capabilities needs admission only; Get and Subscribe are authorized as a read of the running config; Set is authorized for every operation it carries and for the commit, before the session is entered.
7. A gNMI Set enters, edits and commits its session under the caller's name, and the commit is audited under that name.
8. A looking-glass query is dispatched with a named identity, never an empty one. Which identity is D-4.
9. An open stream re-checks its identity each time it delivers: gNMI STREAM at each notification, `/lg/events` at each tick. A locally granted identity whose user has left the running config ends the stream.
10. A reload that changes a surface's mode, token or TLS is refused, naming the restart.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Environment and config tree → resolved mode | The one resolver each surface already has for its token | Yes: `resolveGNMIListeners`; `ExtractLGSettings` and the env reads in `runYANGConfig` |
| Hub → gated surface | The authentication function and the recorder ride the existing build inputs (`gnmiBuildInputs`, `serviceDeps`), as generic types | Yes for the seam; the chain function is a new field on both, and the recorder is new on `gnmiBuildInputs` |
| Surface → AAA chain | `aaa.AuthRequest` with `Service` set, through `liveAAABundleAuthenticator` | Yes: the path SSH and web use |
| Surface → login-set gate | The service name on the request | Depends on the sibling spec landing first |
| gNMI → authorization | The caller's authorizer, asked about the config command names in `internal/component/api` | The decision function exists for gRPC; gNMI calling it is new |
| gNMI → audit | `audit.Recorder` entries with surface `gnmi` | New: no `gnmi` surface constant exists |
| Looking glass → dispatcher | `plugin.CallerIdentity` with a username | New: the caller is zero-value today |

### Changed-Wire Census

**Search evidence:** searched `test`, `docs` and `etc` for a `gnmi` or `looking-glass` config block (36 files, listed in Files to Modify by directory); `internal`, `cmd` and `pkg` for `NewGNMIClient` (the fixture and four test files); `internal` and `cmd` for `ze.gnmi.token`, `ze.looking-glass.token` and `LGToken` (seven non-test files); `internal/test` for a looking-glass `Bearer` header (two fixtures). Positive control: the search for `NewGNMIClient` finds `internal/test/fixture/plugin_fixture_config_refusal.go`, which `test/plugin/gnmi-set-enum-refusal.ci` runs.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| gNMI call metadata credential | reader | `internal/component/gnmi/server.go` `(*Server).checkAuth` | Reads `authorization` only, and only when a token is set | Change: reads `username` and `password` in user mode, `authorization` in token mode, nothing in `none` |
| gNMI call metadata credential | writer | gnmic 0.49.1 `(*Target).appendCredentials`, outside the repository | Sends `username` and `password` for `--username`/`--password`; `--token` sends a bearer header | Preserve: Ze accepts what the client sends. Proven by the gnmic interop test |
| gNMI call metadata credential | writer | `internal/component/gnmi/gnmic_docker_test.go` `runGnmic` | Passes `--token` | Change: token cases select token mode; new cases pass `--username` and `--password` |
| gNMI call metadata credential | writer | `internal/test/fixture/plugin_fixture_config_refusal.go` | Dials gNMI for `gnmi-set-enum-refusal.ci`, whose config sets `token "secret"` | Change: the config states `auth-mode token`, and the fixture is unchanged |
| gNMI call metadata credential | writer | `internal/component/gnmi/server_test.go`, `integration_test.go`, `secret_test.go`, `tls_test.go` | Build `Config` with or without a token | Change: each states the mode it means |
| Looking-glass HTTP credential | reader | `internal/component/lg/auth.go` `bearerAuth` | Reads `Authorization: Bearer <token>` when a token is set | Change: one gate that reads the mode. The user-mode form is D-2 |
| Looking-glass HTTP credential | writer | Browsers and birdwatcher clients such as Alice-LG, outside the repository | Send nothing today unless a proxy adds a header | Per D-2 and D-3 |
| Looking-glass HTTP credential | writer | `internal/test/fixture/lg_pki_fixture.go`, `internal/test/fixture/plugin_fixture_09_shell.go` | Send no header, a wrong token and the right token | Preserve under token mode: the configs they run, the one embedded in the second fixture included, state `auth-mode token` |
| `environment gnmi` and `environment looking-glass` config | reader | `internal/component/config/loader_extract.go` `extractGNMIBlock`, `extractLGBlock`; `resolveGNMIListeners`; `runYANGConfig` | Read `token`, never a mode | Change: read `auth-mode`; validation judges it |
| `environment gnmi` and `environment looking-glass` config | writer | The repository's configs: the `.ci` files under `test/plugin`, `test/parse`, `test/ui`, `test/reload`, `test/stress/scenarios/05-profile-1m/ze.conf`, and the examples in `docs/guide/gnmi.md`, `looking-glass.md`, `looking-glass-howto.md`, `configuration.md`, `docs/config-reference.md` | 36 files carry one of the two blocks | Change: each is converted directly to state the mode it relies on. No migration is written |
| Environment variables | reader | `internal/component/config/environment.go` registrations; `resolveGNMIListeners`; `runYANGConfig` | `ze.gnmi.token` and `ze.looking-glass.token` are registered; no mode variable exists | Change: `ze.gnmi.auth-mode` and `ze.looking-glass.auth-mode` are registered and read |
| Environment variables | writer | `.ci` files that start a surface from the environment, such as `test/plugin/mgmt-guard-gnmi-env-started-token-survives.ci` | Set the token variable alone | Change: each sets the mode variable it relies on |
| Audit entries | writer | `internal/component/gnmi`, `internal/component/lg` | Neither writes an entry | Change: `auth-fail` on both, `config-commit` on gNMI, under new surface names |
| Audit entries | reader | `show audit` and the audit log (`docs/guide/audit.md`) | Lists entries by surface | Preserve the format; the surface vocabulary grows by two |

### Integration Points
- `liveAAABundleAuthenticator` (`cmd/ze/hub/aaa_lifecycle.go`) - the authenticator both surfaces receive from the hub.
- `gnmiBuildInputs` (`cmd/ze/hub/gnmi_infra.go`) and `serviceDeps` (`cmd/ze/hub/service_registry.go`) - the seams the authenticator and the recorder ride. No field names a concrete surface type.
- `resolveGNMIListeners` (`cmd/ze/hub/gnmi_infra.go`) - gains the mode, so the guard and the builder still agree by construction.
- `GNMIListenConfig.Validate` and a new `LGListenConfig` validation (`internal/component/config/loader_extract.go`), both reported by `ValidateSemantics` - the offline refusals.
- `requireWriteAccess` (`internal/component/api/grpc/server.go`) - its decision moves to `internal/component/api` and gains gNMI as a caller.
- `registerMgmtAuthReloaders` (`cmd/ze/hub/mgmt_auth_reload.go`) - gains the two reloaders.
- The login-service registry of the sibling spec - gains two names.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Both surfaces authenticate through the chain every login surface uses, and gNMI writes are authorized by the decision gRPC uses |
| No unintended coupling (components stay isolated) | Yes | Neither surface imports a backend or another surface. The hub hands each a function. The section 19 table gains the `lg` and `gnmi` rows |
| No duplicated functionality (extends existing, does not recreate) | Yes | No user list, no second write-authorization rule, no registry. The Existing Mechanisms table names what each piece extends |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Login path, no wire buffers |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Each surface registers its login-service name and its reloader from its own gated file. `checkMgmtListeners` and the chain name no surface |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | The audit surface names are a hand-written constant list (`internal/core/audit/audit.go`), and this spec adds two. The sibling spec's Known Limitations records that list as one of three that disagree and leaves reconciling them out of scope. `ValidateSemantics` names each surface it validates and gains a looking-glass entry. The login-service names derive from the sibling's registry |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | gnmic sends the per-user credential as the metadata keys `username` and `password` | `(*Target).appendCredentials` in gnmic 0.49.1, read through a fetch tool, and the gNMI authentication text | Ze reads the wrong keys and no real client can log in | AC-11, the gnmic interop test with `--username` and `--password` | unvalidated |
| A-2 | The hub can hand the live chain to the gated gNMI builder without the always-on code naming a gNMI type | `gnmiBuildInputs` already carries generic inputs across the seam | The seam grows a concrete type and the compile-out breaks | `./le arch tier check`, and the `ze_gnmi`-absent build tag test in `cmd/ze/hub` | unvalidated |
| A-3 | The bundle exists before gNMI and the looking glass accept a request | Both are built in `runYANGConfig` after infra setup publishes the boot bundle | An early request finds no chain. The design answer is refusal, never admission | A unit test that calls the gNMI authentication function with an empty bundle slot and expects a refusal | unvalidated |
| A-4 | A gNMI reload reloader can refuse a change although gNMI has no listener-migrator slot | `resolveAuthIntents` skips a name with no handle, per the comment on `markMgmtAuth` | The refusal needs a different hook, such as the config verify step; the criterion (AC-12) does not change | The implementation names the hook before Phase 6 and proves AC-12 through a real reload | unvalidated |
| A-5 | Converting every repository config to state a mode leaves no test relying on inference | The census lists 36 files | A test passes only because the new behavior removed what it checked | Each converted `.ci` is run, and the functional suites the change reaches are run once | unvalidated |
| A-6 | A looking-glass query with an empty username is denied on a daemon that configures authorization profiles | `(*Store).Authorize` denies an empty username; read, not run | The reserved identity of Transformation step 8 is still required for audit, and AC-14's red-before expectation changes to green | `lg-open-mode-with-authorization-profiles.ci`, run against HEAD first | unvalidated |
| A-7 | The refused-login limit can key on the remote address | Both surfaces see the peer address (`peer.FromContext`, the HTTP remote address) | Behind a proxy every client shares one address, and the limit throttles everyone | The guide states the proxy case; AC-13 is proven on a direct connection | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The public looking glass is lost, and a public route viewer starts refusing | A looking-glass suite test fails on a config that selects open | AC-14 pins open as a selectable mode that serves every route with no credential |
| R-2 | gNMI gains a username but no authorization, so any admitted user may write | An admitted caller's Set succeeds under a profile that denies it | AC-3 and AC-4 assert the refusal and the unchanged config |
| R-3 | A password crosses the network in clear | A user-mode listener on a non-loopback address without TLS starts | AC-8 refuses it at validation and at boot, on both surfaces |
| R-4 | A removed user keeps a stream | A STREAM or `/lg/events` connection outlives the user's removal | AC-9 and AC-18 |
| R-5 | Per-request password verification is an unthrottled cost and an online oracle on a surface meant to face the internet | A burst of wrong passwords from one address each costs a bcrypt comparison | AC-13 limits refused logins per remote address |
| R-6 | A read-only gNMI user decodes every `$9$` secret | A profile intended as read-only is granted the running-config read | The guide states that the read exposes every reversible secret, and AC-4 proves a profile can deny it |
| R-7 | The two surfaces are done separately and only one lands, leaving the login-set gate with a hole | A merged change covering one surface | Both are in this spec on purpose. gNMI can land first only if the looking-glass decisions are still open, and the spec then stays open |
| R-8 | A reload changes the mode in the config while the listener keeps the old one | `show gnmi` reports a mode the config no longer states | AC-12 |
| R-9 | The post-release TACACS+ accounting work assumes gNMI writes have no user | `plan/post-release/features/spec-tacacs-chap-mschap-login.md` AC-18 plans a `NOT_SET` accounting record for a gNMI Set | That spec owns the journal row and the accounting. Its premise changes when this lands, and Known Limitations names it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Too strict: a monitoring system loses gNMI, or a public looking glass starts refusing; both are visible at once. Too permissive is quieter: a local process keeps writing config with no credential, a password crosses a network in clear, or a removed user keeps a stream |
| How is it reverted? | Single commit revert per landed piece. A config that states the new `auth-mode` leaf must be edited before an older daemon accepts it |
| Who else touches this path? | `plan/release/fixes/spec-login-service-authorisation.md` builds the gate and lands first; its Files to Modify still plans to add the service name to the authentication request, which `aaa.AuthRequest.Service` already carries. `plan/post-release/features/spec-tacacs-chap-mschap-login.md` owns gNMI Set accounting. `plan/release/fixes/spec-improve-2-gnmi-state.md` edits `internal/component/gnmi/get.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A gNMI call with `username` and `password` metadata on a daemon in user mode | → | `(*Server).checkAuth` and the hub's chain function | `test/plugin/gnmi-user-login.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A gNMI Set by a user whose profile denies config writes | → | the shared write decision in `internal/component/api`, called from `(*Server).Set` | `test/plugin/gnmi-user-set-denied.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A gNMI Set with no credential on a loopback listener in `none` mode | → | the read-only identity in `(*Server).checkAuth` | `test/plugin/gnmi-none-mode-read-only.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A daemon started with gNMI in user mode on a non-loopback address without TLS | → | `GNMIListenConfig.Validate` and the boot check | `test/plugin/gnmi-user-mode-requires-tls.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A reload that removes a user holding a gNMI STREAM | → | the re-check in `(*Server).handleSubscribeStream` | `test/reload/gnmi-stream-ends-on-user-removal.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| gnmic with `--username` and `--password` against a user-mode server | → | `(*Server).checkAuth` | `TestGnmicInterop` (`internal/component/gnmi/gnmic_docker_test.go`), user-mode subtests |
| A looking-glass request against a server configured open | → | the resolved mode in the looking-glass gate (`internal/component/lg/auth.go`) | `test/plugin/lg-open-mode-stays-public.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A looking-glass request against a server configured for users | → | the user branch of the looking-glass gate | `test/plugin/lg-user-login.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| A reload that removes a user holding `/lg/events` | → | the re-check in `(*LGServer).handleUIEvents` | `test/reload/lg-events-ends-on-user-removal.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## Acceptance Criteria

"Red at HEAD" says what the test shows before the change. A row with no such
note has no HEAD behavior to compare, because the mode it needs does not exist.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | gNMI in user mode over TLS. A Capabilities, a Get and a Subscribe ONCE, each with a configured user's `username` and `password` metadata | Each is answered. The daemon log names that user and the `gnmi` surface for the call | Token and `none` modes, when the config selects them: an operator may still run a shared-token collector or a loopback reader |
| AC-2 | The same server. A call with an unknown username, one with a known username and a wrong password, one with no metadata, and one with a bearer token | Each ends `Unauthenticated` with one message, identical across the four. Each writes an `auth-fail` audit entry with surface `gnmi`, the attempted name where one was sent, and the remote address. `ze_gnmi_login_refused_total` rises by four. Red at HEAD: the two keys are not read | None. In user mode no other credential admits a caller |
| AC-3 | User mode. A Set by a user whose profile denies `config set`, then the same Set by a user whose profile permits it | The first ends `PermissionDenied`, the running config is byte-identical, and no config session remains. The second commits. Red at HEAD: no authorizer is consulted | The denied user's reads, where the profile permits them: write and read are separate grants |
| AC-4 | User mode. A Get and a Subscribe by a user whose profile denies `show config running` | Both end `PermissionDenied` and return no value. A user whose profile permits it is answered | Capabilities for the denied user: it publishes models and encodings, not config |
| AC-5 | User mode. User `alice` commits a Set | `show audit` lists one `config-commit` entry with actor `alice`, surface `gnmi`, her remote address and the diff. The commit's log lines carry `user=alice`. The constant actor `gnmi` appears in neither. Red at HEAD: the actor is the constant `gnmi` and no audit entry is written | None |
| AC-6 | gNMI with `auth-mode token` and a token, over TLS. A Get and a Set with the bearer token; a Get with a wrong token | The first two are answered and the Set commits, audited under the shared API identity. The wrong token ends `Unauthenticated` | Per-user credentials are refused in this mode: one mode is in force at a time |
| AC-7 | gNMI with `auth-mode none` on a loopback listener. A Get, a Subscribe ONCE and a Set, with no credential | The Get and the Subscribe are answered. The Set ends `PermissionDenied` and the running config is byte-identical. Red at HEAD: the Set commits | Unauthenticated reads on loopback: that is what the mode is for, and a local process can read the config file anyway |
| AC-8 | Six configs, each through `ze config validate` and through a daemon start: `none` with a non-loopback listener; user mode on a non-loopback listener with no TLS; token mode on a non-loopback listener with no TLS; a `token` leaf under user mode; token mode with no token; an unknown mode value | Each is refused before anything binds. `ze config validate` reports `config-gnmi-invalid`, and both name the leaf and the fix. Red at HEAD for the second to sixth; the first is today's refusal, kept | A loopback listener in any mode without TLS, pending D-5 |
| AC-9 | User mode. Local user `bob` holds a Subscribe STREAM and local user `alice` holds another. A reload removes `bob` | `bob`'s stream ends `Unauthenticated` before the reload notification reaches it. `alice`'s stream receives that notification. Red at HEAD: both receive it | A stream whose user a remote backend (RADIUS, TACACS+) granted is not ended by a local config change, as the web rule already holds |
| AC-10 | A user whose login set excludes `gnmi` presents a correct password; then a config naming `gnmi` and `looking-glass` in a login set is validated | The login is refused exactly as AC-2's wrong password is. The config validates, and CLI completion offers both names. Depends on the sibling spec | A user naming no login set, per that spec's default |
| AC-11 | gnmic 0.49.1 with `--username` and `--password` against a user-mode server over TLS: capabilities, get, subscribe once, set; then a wrong password; then `--token` against a token-mode server | The first four succeed with the answers `TestGnmicInterop` already asserts. The wrong password fails with `Unauthenticated`. The token case still succeeds | None |
| AC-12 | A running daemon. A reload whose config changes gNMI's `auth-mode` or `token`; then the same for the looking glass | Each reload is refused, the message names the leaf and `ze signal restart`, and the running listener still enforces the mode it started with. Red at HEAD: the reload is accepted and the listener keeps the old setting | A reload that changes neither surface's authentication is applied as today |
| AC-13 | One remote address sends refused logins up to the fixed limit within the fixed window, on gNMI and then on the looking glass in user mode | The next attempt inside the window is refused with the surface's rate-limit answer (`ResourceExhausted`; HTTP 429 with `Retry-After`), and the password it carries is never compared. A correct login from another address is admitted at the same moment. The limit and window are one constant each, and the test reads them | Refused attempts below the limit: they cost a comparison each, which is the price of telling a user their password is wrong |
| AC-14 | A looking glass with `auth-mode open`, on a daemon that also configures `system authorization` profiles | Every page and every `/api/looking-glass/*` route answers with data and with no credential | A-6 decides whether this is red at HEAD |
| AC-15 | A looking glass with `auth-mode token`. A request with the token, one with a wrong token, one with none | The first is served; the others get 401 with the `WWW-Authenticate` header, as `lg-token-gate.ci` asserts today | None |
| AC-16 | A looking glass in user mode, over TLS. A request with a configured user's credential in the form D-2 selects | The page is served, and the daemon log names that user and the looking-glass surface. Blocked by D-2 | Open and token modes when selected |
| AC-17 | A looking glass that starts with no `auth-mode` stated, from a config block and then from the environment alone | The behavior D-1 selects, the same from both sources. Blocked by D-1 | None in the mode not chosen |
| AC-18 | User mode. Local user `bob` holds `/lg/events`. A reload removes `bob` | The stream closes at the next tick, and that tick's event is the last it could have sent. Blocked by D-2 for how the stream is opened | A stream of a remotely granted user, as in AC-9 |
| AC-19 | User mode. A query by an admitted user whose profile lacks the command the page dispatches | The answer D-4 selects: the page's data under the reserved identity, or the looking glass's refusal body. Blocked by D-4 | None in the option not chosen |
| AC-20 | User mode. A birdwatcher request to `/api/looking-glass/status` with and without a credential | The answer D-3 selects. An admitted request's body is byte-identical to open mode's. Blocked by D-3 | None in the option not chosen |
| AC-21 | Looking-glass validation: user or token mode on a non-loopback listener that will serve plaintext; a `token` leaf without token mode; token mode with no token; an unknown mode | Each is refused by `ze config validate` and at start, naming the leaf and the fix. The warned fallback to plaintext in `buildLGService` becomes that refusal in a credential mode. Red at HEAD: a token over plaintext on a non-loopback address starts | `open` mode over plaintext: it carries no credential, and a proxy in front may serve TLS |
| AC-22 | `ze.gnmi.auth-mode` and `ze.looking-glass.auth-mode` set in the environment of a daemon whose config states another mode | The environment value is in force, as it is for each surface's token | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | points a gNMI collector at the router with a named account | metadata → chain → identity → read authorization → answer | `test/plugin/gnmi-user-login.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 2 | gives an account read-only rights and tries to Set with it | metadata → chain → write decision → refusal, config unchanged | `test/plugin/gnmi-user-set-denied.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 3 | asks who changed the config over gNMI | Set → session under the user → audit entry → `show audit` | `test/plugin/gnmi-user-commit-audited.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 4 | removes an account while its collector is streaming | reload → notifier → stream re-check → stream ends | `test/reload/gnmi-stream-ends-on-user-removal.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 5 | runs a local script against loopback gNMI with no credential | `none` mode → read-only identity → Get answered, Set refused | `test/plugin/gnmi-none-mode-read-only.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 6 | runs a public route viewer and says so in the config | config → open mode → unauthenticated routes | `test/plugin/lg-open-mode-stays-public.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 7 | puts the looking glass behind named accounts | config → user mode → chain → served request | `test/plugin/lg-user-login.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| 8 | writes a user-mode listener on a public address and forgets TLS | validation → refusal naming the fix | `test/plugin/gnmi-user-mode-requires-tls.ci`, `test/plugin/lg-credential-mode-requires-tls.ci` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGNMIUserLoginReadsUsernameAndPasswordMetadata` | `internal/component/gnmi/server_test.go` | AC-1, AC-2: the two keys reach the authentication function with service `gnmi`; the four refusals share one status and message | |
| `TestGNMIUserLoginAlwaysCallsTheChain` | `internal/component/gnmi/server_test.go` | AC-2: an unknown user and an empty password still reach the authentication function, so the chain's equal-cost path is never skipped | |
| `TestGNMIRefusesWhenNoChainIsInstalled` | `internal/component/gnmi/server_test.go` | A-3: a nil authentication function refuses in user mode | |
| `TestGNMISetAuthorizesEveryOperationBeforeTheSession` | `internal/component/gnmi/set_test.go` | AC-3: a denied delete, update or commit ends the Set with no session entered | |
| `TestGNMIReadAuthorizedAsRunningConfigRead` | `internal/component/gnmi/get_test.go` | AC-4 | |
| `TestGNMINoneModeCallerIsReadOnly` | `internal/component/gnmi/set_test.go` | AC-7 | |
| `TestGNMITokenModeUsesSharedAPIIdentity` | `internal/component/gnmi/server_test.go` | AC-6 | |
| `TestGNMIStreamEndsWhenLocalUserLeaves` | `internal/component/gnmi/subscribe_test.go` | AC-9, including the remotely granted stream that stays | |
| `TestGNMIListenConfigValidateModes` | `internal/component/config/loader_extract_test.go` | AC-8: one case per refused config and one per accepted mode | |
| `TestLGListenConfigValidateModes` | `internal/component/config/loader_extract_test.go` | AC-21 | |
| `TestResolveGNMIListenersMode` | `cmd/ze/hub/gnmi_infra_test.go` | AC-22, and the unset default `user` | |
| `TestConfigWriteDecisionSharedByGRPCAndGNMI` | `internal/component/api/config_auth_test.go` | The moved decision keeps `requireWriteAccess`'s verdicts for a read-only caller, a nil authorizer and a denying authorizer | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `TestRefusedLoginLimit` | beside the limiter, whose package the implementation names | AC-13: the limit, the window, and the independence of two addresses | |
| `TestLGOpenModeIsExplicit` | `internal/component/lg/auth_test.go` | AC-14: open is a resolved mode; an empty token no longer selects it | |
| `TestLGUserLogin` | `internal/component/lg/auth_test.go` | AC-16, and the single 401 for every refusal cause | |
| `TestLGEventsEndsWhenLocalUserLeaves` | `internal/component/lg/handler_ui_test.go` | AC-18 | |
| `TestLGQueryCarriesAnIdentity` | `internal/component/lg/server_test.go` | AC-14, AC-19: no query is dispatched with an empty username | |
| `TestBothSurfacesRegisterTheirLoginName` | `internal/component/config/loginservice/registry_test.go` | AC-10 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `TestDeletedUserLosesBothSurfaces` | `cmd/ze/hub/auth_e2e_test.go` | A removed user is refused on both surfaces through the hub's real chain wiring, with no restart | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| gNMI `auth-mode` | one of `user`, `token`, `none` | each of the three | an unset leaf resolves to `user` and is never an error | an unrecognized value fails validation |
| looking-glass `auth-mode` | one of `open`, `token`, `user` | each of the three | an unset leaf follows D-1 | an unrecognized value fails validation |
| refused logins per address in one window | 0 to the limit | the limit: that attempt is still verified | N-A | the limit plus one: refused unverified |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gnmi-user-login` | `test/plugin/gnmi-user-login.ci` | AC-1 and AC-2 on one daemon: a named account reads, and an unknown user, a wrong password and a bearer token are each refused | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-user-set-denied` | `test/plugin/gnmi-user-set-denied.ci` | AC-3: the denied Set leaves the active config key unchanged; the permitted Set changes it | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-user-read-denied` | `test/plugin/gnmi-user-read-denied.ci` | AC-4 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-user-commit-audited` | `test/plugin/gnmi-user-commit-audited.ci` | AC-5: `show audit` names the user and the surface | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-token-mode` | `test/plugin/gnmi-token-mode.ci` | AC-6 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-none-mode-read-only` | `test/plugin/gnmi-none-mode-read-only.ci` | AC-7 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-user-mode-requires-tls` | `test/plugin/gnmi-user-mode-requires-tls.ci` | AC-8, the daemon start half: exit 1 before anything binds, naming the fix | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-auth-mode-invalid` | `test/parse/gnmi-auth-mode-invalid.ci` | AC-8, the `ze config validate` half, one case per refused config | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-stream-ends-on-user-removal` | `test/reload/gnmi-stream-ends-on-user-removal.ci` | AC-9 on a live daemon | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-deleted-user-refused` | `test/reload/gnmi-deleted-user-refused.ci` | A user who logged in, then was removed by a reload, is refused on the next call with no restart | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-login-set-excludes` | `test/plugin/gnmi-login-set-excludes.ci` | AC-10: an excluded user with a correct password is refused, and an included one is served | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-auth-reload-refused` | `test/reload/gnmi-auth-reload-refused.ci` | AC-12, gNMI half | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `gnmi-refused-login-limit` | `test/plugin/gnmi-refused-login-limit.ci` | AC-13, gNMI half | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `lg-open-mode-stays-public` | `test/plugin/lg-open-mode-stays-public.ci` | AC-14 without authorization profiles | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `lg-open-mode-with-authorization-profiles` | `test/plugin/lg-open-mode-with-authorization-profiles.ci` | AC-14 with profiles; run against HEAD first to settle A-6 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `lg-user-login` | `test/plugin/lg-user-login.ci` | AC-16: a named account is served, and a wrong password, an unknown user and an excluded user each get the same 401, on one daemon | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `lg-auth-mode-unset` | `test/plugin/lg-auth-mode-unset.ci` | AC-17, from the block and from the environment | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `lg-events-ends-on-user-removal` | `test/reload/lg-events-ends-on-user-removal.ci` | AC-18 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `lg-user-query-authorization` | `test/plugin/lg-user-query-authorization.ci` | AC-19 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `lg-user-mode-birdwatcher` | `test/plugin/lg-user-mode-birdwatcher.ci` | AC-20 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `lg-credential-mode-requires-tls` | `test/plugin/lg-credential-mode-requires-tls.ci` | AC-21 | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `lg-auth-reload-refused` | `test/reload/lg-auth-reload-refused.ci` | AC-12, looking-glass half | <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `mgmt-guard-gnmi-nonloopback-refused` | `test/plugin/mgmt-guard-gnmi-nonloopback-refused.ci` | Existing and green at HEAD. Its config and expected remedy are converted to the mode wording, and it stays the proof that the old refusal survived | |
| `lg-token-gate` | `test/plugin/lg-token-gate.ci` | Existing. Its config states `auth-mode token`; AC-15 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `TestGnmicInterop`, user-mode subtests | `internal/component/gnmi/gnmic_docker_test.go` (tag `docker`; `docs/architecture/testing/interop.md` describes the run) | gnmic 0.49.1 | AC-11: a real client's `--username` and `--password` log in over TLS for capabilities, get, subscribe once and set; a wrong password is refused; `--token` still works in token mode. The vacuity check is the wrong-password subtest plus a run with the user branch reverted, which turns the four success subtests red | |

No interop test is planned for the looking glass until D-3 says what Alice-LG
presents. Before that test expects Alice-LG to send a credential, its source is
read for the function that sends it.

## Files to Modify
- `internal/component/gnmi/server.go` - `Config` gains the mode, the authentication function and the recorder; `Serve` always installs the authentication interceptors; `checkAuth` returns an identity for every mode
- `internal/component/gnmi/set.go` - the constant `gnmiUsername` is deleted; the session runs under the caller, after the write decision; the commit is audited
- `internal/component/gnmi/get.go`, `internal/component/gnmi/subscribe.go` - the read authorization, and the stream re-check
- `internal/component/gnmi/metrics.go` - `ze_gnmi_login_refused_total`
- `internal/component/gnmi/show.go` and `ServerStatus` - report the mode in place of `token-set` alone
- `internal/component/gnmi/yang/ze-gnmi-conf.yang` - the `auth-mode` enumeration; the token description no longer says an empty token means no authentication
- `internal/component/lg/auth.go` - one gate over the mux that reads the mode
- `internal/component/lg/server.go` - `LGConfig` gains the mode, the authentication function and the recorder; `query` carries an identity
- `internal/component/lg/handler_ui.go` - the stream re-check in `handleUIEvents`
- `internal/component/lg/yang/ze-lg-conf.yang` - the `auth-mode` enumeration; the token description no longer says an empty token leaves the looking glass open
- `internal/component/config/loader_extract.go` - both blocks read the mode; `GNMIListenConfig.Validate` judges it; `LGListenConfig` gains its validation
- `internal/component/config/validate_semantic.go` - the looking-glass entry beside the gNMI and MCP ones
- `internal/component/config/environment.go` - `ze.gnmi.auth-mode`, `ze.looking-glass.auth-mode`
- `internal/component/api/types.go` and a new file beside it - the running-config read command name, and the write decision moved from gRPC
- `internal/component/api/grpc/server.go` - `requireWriteAccess` calls the moved decision
- `internal/core/audit/audit.go` - the two surface names
- `internal/core/diagnostic/codes.go` - the looking-glass validation code, and the `config-gnmi-invalid` description, which today names only the tokenless case
- `cmd/ze/hub/gnmi_infra.go`, `cmd/ze/hub/service_gnmi.go` - the resolver returns the mode; the build inputs carry the chain function and the recorder
- `cmd/ze/hub/service_lg.go`, `cmd/ze/hub/service_registry.go`, `cmd/ze/hub/register_lg.go` - the same for the looking glass; no plaintext fallback in a credential mode
- `cmd/ze/hub/main.go` - gNMI's guard declaration reads the mode; the looking-glass mode is resolved beside its token
- `cmd/ze/hub/mgmt_auth_reload.go` - the two reloaders, and the corrected comment
- `internal/component/gnmi/gnmic_docker_test.go`, `internal/test/fixture/lg_pki_fixture.go`, `internal/test/fixture/plugin_fixture_09_shell.go` - modes stated; user-mode subtests
- The 36 repository configs the census found, under `test/plugin`, `test/parse`, `test/ui`, `test/reload`, `test/stress/scenarios/05-profile-1m` and `docs` - each states the mode it relies on
- `docs/architecture/api/architecture.md` - "gNMI Transport" and the caller-class table, which gains gNMI (declared by the `// Design:` header of the gNMI server, Get, Set and Subscribe files)
- `docs/architecture/web-interface.md` - "LG Bearer Token" and "LG Data Access" (declared by `internal/component/lg/auth.go`)
- `docs/architecture/api/birdwatcher-compat.md` - the authentication statement D-3 selects
- `docs/architecture/core-design.md`, section 19 - the `lg` and `gnmi` rows
- `docs/guide/gnmi.md` (declared by `internal/component/gnmi/tls.go`), `docs/guide/authentication.md`, `docs/guide/looking-glass.md`, `docs/guide/looking-glass-howto.md`, `docs/guide/configuration.md`, `docs/guide/audit.md`
- `docs/guide/environment-variables.md` - the two mode variables; `docs/guide/config-reload.md` - the refused authentication change and its restart instruction
- `features/gnmi.md`, `features/looking-glass.md` - the description, and the Defect review line that calls this spec "design scope, not a defect"

Three more pages are declared by the `// Design:` headers of files in this list
and are not edited, each for the reason given:
- `docs/architecture/config/syntax.md` (declared by `loader_extract.go`, `validate_semantic.go` and the `config/infra` files) - it describes the file grammar and mentions gNMI only for list-key paths and `Tree.ToMap`; an enumeration leaf adds no grammar
- `docs/architecture/hub-architecture.md` (declared by `cmd/ze/hub/main.go`) - its one gNMI mention is the stop order at shutdown, which does not change
- `docs/features/ai-first.md` (declared by `internal/core/diagnostic/codes.go`) - it names neither surface; the new diagnostic code is listed by the generated code catalog, not by this page

## Files to Create
- The new `.ci` files named in Functional Tests
- `internal/component/api/config_auth.go` - the shared write decision <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
- The refused-login limiter, in the package the implementation names after reading where both surfaces can reach it without a new cross-component import

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | An `auth-mode` leaf in `ze-gnmi-conf.yang` and in `ze-lg-conf.yang` |
| YANG validation constraints | Yes | Each is an enumeration, so an unrecognized value fails at parse. The companion-leaf rules are semantic validation |
| YANG custom validators | No | The value set is fixed. The login-service names are the sibling spec's validator |
| CLI commands/flags | No | No new verb. `show gnmi` gains a field |
| CLI grammar (keyword before value) | No | No new command |
| Editor autocomplete | Yes | Automatic for an enumeration leaf |
| Functional test for new RPC/API | Yes | The `.ci` files in Functional Tests |
| Pipe completeness | No | No new command output |
| Env var registration | Yes | `ze.gnmi.auth-mode` and `ze.looking-glass.auth-mode` in `internal/component/config/environment.go` |
| Doctor check for runtime dependencies | No | No new file, socket, port or binary. TLS material already has `gnmi-tls-material` |
| Doctor code per problem class | Yes | `config-gnmi-invalid` covers the gNMI refusals. The looking glass gains its own validation code, reported by `ze doctor` and `ze config validate` before boot refuses. A refused reload names the same code |
| Prometheus counters/metrics | Yes | `ze_gnmi_login_refused_total{reason}` and `ze_lg_login_refused_total{reason}`, with `reason` one of `credential`, `service`, `limit` |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/gnmi.md`, `features/looking-glass.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`; `docs/config-reference.md` if it is generated, by its generator |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: `show gnmi` reports the mode |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/architecture.md`: gNMI's authentication contract and per-RPC authorization |
| 5 | Plugin added/changed? | No | Both are components |
| 6 | Has a user guide page? | Yes | `docs/guide/authentication.md`, `docs/guide/gnmi.md`, `docs/guide/looking-glass.md`, `docs/guide/looking-glass-howto.md` |
| 7 | Wire format changed? | No | gNMI metadata and HTTP headers are not Ze wire formats under `docs/architecture/wire/` |
| 8 | Plugin SDK/protocol changed? | No | No SDK type crosses this seam |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | The governing text is OpenConfig's, and no RFC summary is enrolled for it |
| 10 | Test infrastructure changed? | No | Existing suites and the existing gnmic lab client |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, where it describes gNMI or looking-glass access control |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` section 19; `docs/architecture/web-interface.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/monitoring.md`: the two counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Two login-service registrations, in the inventory the sibling spec names |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by `// Design:` headers: `docs/architecture/api/architecture.md` (gNMI, API), `docs/architecture/web-interface.md` (looking glass) and `docs/guide/gnmi.md` (gNMI TLS), all named above. `./le spec citation anchors spec plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` lists the rest at implementation time |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Every gNMI and looking-glass example in the four guide pages gains a mode, and the gnmic examples in `docs/guide/gnmi.md` gain the `--username` form |

## Implementation Steps

The gNMI phases need no owner decision except D-5, which changes only whether
loopback clear text survives. The looking-glass phases wait for D-1 to D-4.

1. **Phase: Wiring (MANDATORY FIRST)** -- prove today's behavior before changing it
   - Tests: `gnmi-user-login`, `gnmi-none-mode-read-only`, `gnmi-user-set-denied`, `lg-open-mode-with-authorization-profiles`
   - Files: those `.ci` files
   - Verify: each gNMI test fails for the reason its AC names as red at HEAD, not on a setup error. The looking-glass test settles A-6
2. **Phase: Modes and validation** -- the leaves, the variables, the resolvers, the refusals
   - Tests: `TestGNMIListenConfigValidateModes`, `TestLGListenConfigValidateModes`, `TestResolveGNMIListenersMode`, `gnmi-auth-mode-invalid`, `gnmi-user-mode-requires-tls`
   - Files: the two YANG modules, `loader_extract.go`, `validate_semantic.go`, `environment.go`, `gnmi_infra.go`, `main.go`
   - Verify: AC-8, AC-22, and the gNMI half of the repository configs converted in the same piece of work
3. **Phase: gNMI identity** -- the chain, the three caller classes, the audit of refusals
   - Tests: `TestGNMIUserLoginReadsUsernameAndPasswordMetadata`, `TestGNMIUserLoginAlwaysCallsTheChain`, `TestGNMIRefusesWhenNoChainIsInstalled`, `TestGNMITokenModeUsesSharedAPIIdentity`, `gnmi-user-login`, `gnmi-token-mode`
   - Files: `gnmi/server.go`, `service_gnmi.go`, `gnmi_infra.go`, `audit.go`
   - Verify: AC-1, AC-2, AC-6
4. **Phase: gNMI authorization and audit**
   - Tests: `TestConfigWriteDecisionSharedByGRPCAndGNMI`, `TestGNMISetAuthorizesEveryOperationBeforeTheSession`, `TestGNMIReadAuthorizedAsRunningConfigRead`, `TestGNMINoneModeCallerIsReadOnly`, `gnmi-user-set-denied`, `gnmi-user-read-denied`, `gnmi-user-commit-audited`, `gnmi-none-mode-read-only`
   - Files: `api/config_auth.go`, `grpc/server.go`, `gnmi/set.go`, `gnmi/get.go`, `gnmi/subscribe.go`
   - Verify: AC-3, AC-4, AC-5, AC-7. The constant `gnmiUsername` no longer exists
5. **Phase: gNMI streams, removal and the refused-login limit**
   - Tests: `TestGNMIStreamEndsWhenLocalUserLeaves`, `TestRefusedLoginLimit`, `gnmi-stream-ends-on-user-removal`, `gnmi-deleted-user-refused`, `gnmi-refused-login-limit`
   - Files: `gnmi/subscribe.go`, the limiter
   - Verify: AC-9, AC-13 for gNMI
6. **Phase: gNMI reload and the gnmic interop**
   - Tests: `gnmi-auth-reload-refused`, `TestGnmicInterop` user-mode subtests
   - Files: `mgmt_auth_reload.go`, `gnmic_docker_test.go`
   - Verify: AC-11, AC-12 for gNMI. A-4 is settled before this phase starts
7. **Phase: Looking-glass modes and identity** -- after D-1 to D-4
   - Tests: the looking-glass unit tests and `.ci` files
   - Files: `lg/auth.go`, `lg/server.go`, `lg/handler_ui.go`, `service_lg.go`, `service_registry.go`, `register_lg.go`, `mgmt_auth_reload.go`
   - Verify: AC-14 to AC-21, AC-12 and AC-13 for the looking glass
8. **Phase: Join the login-set gate** -- after the sibling spec lands
   - Tests: `TestBothSurfacesRegisterTheirLoginName`, `TestDeletedUserLosesBothSurfaces`, `gnmi-login-set-excludes`
   - Files: the two registration sites
   - Verify: AC-10
9. **Phase: Docs and discrimination**
   - Tests: every new `.ci`, each re-run with its producing change reverted
   - Files: the documentation set
   - Verify: every new `.ci` goes red without the change, and the red is recorded

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC that is not blocked by an open decision has an implementation and a `.ci`. A blocked AC is named as blocked, never dropped |
| Feature completeness | User AND token AND the unauthenticated mode on each surface, each explicitly selectable. Dropping one changes what an operator can express |
| Correctness | Every refusal cause on a surface produces one answer. gNMI Set checks every operation before it enters a session |
| Naming | `auth-mode` on both, as MCP has. `open` on the looking glass says public; `none` on gNMI says no credential, as MCP's `none` does |
| Data flow | One chain, handed in by the hub. Neither surface builds a user list, and neither reads `system authentication` |
| Operator errors and doctor codes | Every validation refusal is an operator error with its fix on the last line and names `config-gnmi-invalid` or the looking-glass code. The reload refusal names `ze signal restart` |
| Rule: `ai/rules/evidence.md` | No path resolves a mode it cannot read to the unauthenticated one. A nil authentication function refuses |
| Rule: `ai/rules/no-layering.md` | The constant `gnmiUsername`, the zero-value caller in `query` and the token-means-mode inference are deleted, not kept beside their replacements |
| Rule: `ai/META-DECISIONS.md`, pre-release | Repository configs are converted. No migration, hint or inferred mode exists |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| gNMI resolves an identity in every mode | `gnmi-user-login`, `gnmi-token-mode`, `gnmi-none-mode-read-only` pass, and each fails with its change reverted |
| A gNMI write is authorized and audited | `gnmi-user-set-denied`, `gnmi-user-commit-audited` |
| No constant gNMI actor | `gopls references` on `gnmiUsername` finds no symbol, and `gnmi-user-commit-audited` asserts the actor |
| No credential in clear on a non-loopback listener | `gnmi-user-mode-requires-tls`, `lg-credential-mode-requires-tls`, `gnmi-auth-mode-invalid` |
| Streams end with their user | `gnmi-stream-ends-on-user-removal`, `lg-events-ends-on-user-removal` |
| A real client can log in | `TestGnmicInterop` user-mode subtests |
| The public looking glass survives | `lg-open-mode-stays-public`, `lg-open-mode-with-authorization-profiles` |
| Lint | `./le go lint run scope "<pkg>"` for each package changed |
| Schema and docs | `./le doc check all`, `./le cli dispatch check`, `./le yang glue check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| What a wrong landing exposes | A loopback process that still commits config with no credential; a password or token in clear on a network; a removed user's stream still delivering; a looking glass that resolves to open when the operator asked for a gate |
| What proves it did not | Each `.ci` asserts both halves on one daemon: admitted with the right credential, refused with the wrong one, and for writes the active config key unchanged |
| Fail closed | An unreadable mode, a nil chain and an empty bundle slot each refuse. Trace every error return in both gates and confirm none admits |
| Unset is a stated behavior | gNMI unset is `user`. The looking glass unset is D-1. Neither is "whatever the token leaf is" |
| No user enumeration | One status and one message for every refusal cause. The surface calls the chain for an unknown user, so the chain's dummy comparison runs |
| Online guessing | The refused-login limit holds per remote address, and a refused attempt above it verifies nothing |
| Privilege | A read-only caller cannot Set. A profile that denies `config set`, `config delete` or `config commit` denies the gNMI Set that needs it |
| Secrets on read | A gNMI read returns every reversible secret. Confirm the read is gated by the running-config authorization, and that the guide says what the `$9$` form is |
| Accounting | A refused login is an `auth-fail` entry, never a success. A commit names its user |
| Transport | A credential mode on a non-loopback listener never starts without TLS, on either surface, including the looking glass's warned fallback |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A new `.ci` passes with the change reverted | The test is vacuous. Assert the identity or the unchanged config, not the response code alone |
| gnmic does not send `username` and `password` as read | STOP. A-1 is broken. Report what the client sends, and which key Ze has to read |
| The chain cannot cross the gated seam without a concrete type | STOP. A-2 is broken. Report the signature reached |
| A looking-glass phase is reached with D-1 to D-4 unanswered | STOP. Land the gNMI phases, keep the spec open, and report which decision blocks |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The two surfaces look alike and are not. A looking glass with no token is
  public ON PURPOSE, and the owner asked to keep that while making it a stated
  choice. A gNMI server with no token authenticates nobody by ACCIDENT.
- The gRPC API is the model for gNMI's authorization and audit, and the wrong
  model for its authentication. It shares gNMI's transport and its config
  session manager, so the write decision, the read command and the audit entry
  carry over. Its login is a fixed local user slice, which is not what SSH does.
- The first version's "no timing signal" criterion could not be measured. The
  property it wanted already exists in one place, the dummy comparison in
  `(*LocalAuthenticator).Authenticate`, and the testable obligation on a
  surface is to always call the chain.
- A criterion that HEAD already satisfies proves nothing when it passes. The
  non-loopback tokenless refusal is kept as preserved behavior with its
  existing test, and the criteria that replace it are each red at HEAD for a
  stated reason.
- Splitting this from `plan/release/fixes/spec-login-service-authorisation.md`
  is deliberate. That spec checks a fact the daemon already knows; this one
  teaches it a fact it has never known. They fail differently and test
  differently.

## Owned journal rows

This spec owns no open journal row. The row that describes its gNMI defect,
`plan/journal/invariant-enforced-by-an-absent-call-site.md` dated 2026-10-31
("gNMI Set neither session-authorized nor accounted"), is listed by
`plan/post-release/features/spec-tacacs-chap-mschap-login.md`.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An explicit `auth-mode` leaf on each surface | Infer the mode from whether a token or users are configured | Inference is what produces today's surprise, where an empty leaf silently disables a gate. The owner asked for an option, which means a stated choice |
| No mode is inferred from an existing token leaf | Infer token mode when the token is set and the mode is absent, as MCP does | MCP's inference exists for backward compatibility. Ze is pre-release, so repository configs are converted directly and no compatibility path is written |
| gNMI's unset mode is `user` | Require the leaf; default to `token` or `none` | The owner's words: gNMI behaves like SSH, and SSH needs no leaf to require a user. A default of `none` would keep today's accident as the default |
| gNMI user mode authenticates through the live AAA chain | Copy gRPC's `buildAPIAuthentication` | The owner's words again. gRPC's path reaches local users only, so a RADIUS or TACACS+ operator who can use SSH could not use gNMI |
| The credential is the metadata keys `username` and `password` | `Bearer user:password`, as REST and gRPC take | The gNMI authentication text asks for username and password in the metadata, and gnmic sends those two keys. A form no gNMI client produces cannot be proven with a real client |
| Keep the shared-token mode on both | Replace it with per-user | A token is the right credential for an unattended collector, and removing a capability is the owner's call, not this spec's |
| One mode in force per surface | Accept a token and users on one listener | One gate with one answer per request. D-3 option (b) is the one place the looking glass might need a pair, and it is the owner's to choose |
| A `none`-mode gNMI caller is read-only | Keep full access on loopback | Every surface gives the same result: gRPC already marks its no-auth caller read-only (`(*GRPCServer).checkAuth`), and an unauthenticated config write is a fail-open, not a mode |
| A credential mode on a non-loopback listener requires TLS, on both surfaces | Require it for user mode only | gRPC already holds this rule for any authentication (`checkGRPCListenAddr`), for the reason it states: the credential that satisfies the gate would cross the network in clear. A token is such a credential too |
| A reload that changes authentication is refused with the restart instruction | Apply the change live | MCP already does this (`mcpAuthReloader`), and it keeps the one-time construction both servers have. What is not acceptable is today's third answer, accepted and ignored |
| gNMI reads are authorized as `show config running` | Admit every authenticated reader | The `$9$` form is reversible by anyone, so a gNMI read is a read of the running config with its secrets |
| Both surfaces in one spec | One spec each | Either alone leaves the login-set gate with a hole, and the hole is invisible from the surface that did land |

## Known Limitations
- MCP keeps its own `Identity` scope model and is not covered here.
- This spec depends on `plan/release/fixes/spec-login-service-authorisation.md`
  for the gate itself. AC-10 and Phase 8 wait for it.
- TACACS+ accounting of a gNMI Set (RFC 8907) is not done here.
  `plan/post-release/features/spec-tacacs-chap-mschap-login.md` owns it as its
  AC-18. That criterion plans a `NOT_SET` record for a write with no user;
  after this spec a user-mode write has one.
- REST and gRPC keep authenticating against the fixed local user slice. Moving
  them to the chain is not part of the owner's requirement for this spec.
- gNMI binds only the first `server` entry. That defect has its own journal
  row and is not changed here.
- A stream whose user a remote backend granted is not ended by a config
  change, on gNMI or the looking glass, as the web UI already behaves.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-22 all demonstrated, each blocked one after its decision is recorded here
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path <dir>/` for each of `internal/component/gnmi`, `internal/component/lg`, `internal/component/api`, `internal/component/api/grpc`, `internal/component/config` and `cmd/ze/hub`; `./le test functional plugin`, `./le test functional parse` and `./le test functional reload` for the `.ci` files this spec names and converts; and any further suite `./le repo changed scope print both` reaches. The whole-tree verification is not part of this gate: GitHub CI runs it on the pushed commit, and no agent starts it
- [ ] Feature code integrated on both surfaces, not test-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] D-1 to D-5 each answered by the owner and the answer written into this spec
- [ ] Every item this spec did not do is a spec of its own, named in Known Limitations

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features: the gNMI per-user credential proven with gnmic

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-login-identity-for-looking-glass-and-gnmi.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
