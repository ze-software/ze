# 1242 -- fixit-authz-admin-fallthrough

## Context

`authz.Store.Authorize` used to fall through to `BuiltinAdminProfile()` (allow-all)
whenever a box had authorization profiles but the caller resolved to no applicable
profile and the store had no local users (`hasUsers == false`). That "no profile ->
admin" default is a privilege-escalation footgun: staging profiles before operator
accounts, or a TACACS+/RADIUS-only box where a user authenticates but maps to zero
profiles, silently granted admin. The fix removes `hasUsers` as a decision input and
makes every unresolved path fail closed (Deny), while preserving a break-glass path
for the bootstrap admin and keeping internal plugin RPC working.

## Decisions

- **Fail closed, always.** S-1 (no auth), S-2 (no applicable profile), and S-3
  (assignment names only undefined profiles) all `return Deny` (`authz.go:387/440/484`).
  `BuiltinAdminProfile()` now has zero production callers -- the allow-all default is
  gone. An empty username at `Authorize` is Deny, not a wildcard.
- **Break-glass via a RESERVED recovery profile, not a config assignment.** The `ze init`
  bootstrap admin carries `aaa.ReservedRecoveryProfile` -- a NUL-prefixed (`\x00ze:`),
  un-typeable name outside the config namespace -- delivered ONLY through login-resolved
  profiles (`UserCredential.Profiles -> AuthResult.Profiles -> RecordLoginProfiles`,
  `main_servers.go:144`), never via `AssignProfiles`. `Authorize` honors this reserved
  name regardless of the store's profiles, so a wrong/partial authorization config can
  never lock the bootstrap admin out. The older "define a config `admin` user" route
  survives as a second, operator-controlled recovery path.
- **Internal RPC dispatch injects a RESERVED internal identity.** All five internal
  CommandContext constructions in `plugin/server` (`opUpdateRoute`
  dispatch_registry.go:247; `handleUpdateRouteSelDirect` dispatch.go:478;
  `dispatchCommandArgs` :541; `dispatchCommand` :576; `wrapHandler` server.go:137)
  set `Username: internalPluginIdentity(proc.Name())`, which authorizes via a reserved
  trusted profile. These engineOps are reachable ONLY over the plugin->engine RPC socket
  (a connected plugin process), never an operator/CLI/remote surface, so the trusted
  grant is not remotely reachable. Without this, removing the admin default broke route
  propagation (empty username -> Deny).

## Gotchas

- **The reserved-name mechanism is only as safe as its wire-ingress filtering.** The
  security review found a BLOCKER: a hostile TACACS+/RADIUS server could return a
  `\x00ze:` reserved profile name in its reply and have `Authorize` short-circuit to
  Allow. Fix: every UNTRUSTED wire backend drops reserved names before assembling
  profiles -- `radius mapProfiles` (authenticator.go:203 `IsReservedName`),
  `tacacs handlePass` (authenticator.go:114 `FilterReservedNames`). `AuthResult` has no
  Username field, so a hostile server cannot inject an identity via the reply, only a
  profile -- which is now filtered.
- **Do NOT centralize the reserved-profile filter at the auth choke point.** The obvious
  "robust by construction" move -- `FilterReservedNames(result.Profiles)` in
  `profileRecordingAuthenticator.Authenticate` -- LOCKS OUT THE RECOVERY ADMIN. The
  TRUSTED local backend legitimately delivers the reserved recovery profile through that
  exact `result.Profiles` path (`main_servers.go` usersFromZefsDB), so a central strip
  erases the break-glass grant. Filtering therefore lives in each untrusted backend,
  which never has a legitimate reason to emit a reserved name. An anti-pattern comment
  at `login_profiles.go:92` documents this so a future agent does not "helpfully"
  centralize it. (A Go unit test seeding `LoginProfiles` directly passes either way --
  only the full-login `.ci` catches the lockout.)
- **The reserved USERNAME is a separate spoof surface.** `profileRecordingAuthenticator`
  (the single choke point every ssh/web/api surface passes through) rejects
  `IsReservedName(request.Username)` with `ErrAuthRejected` BEFORE any backend runs, so
  no backend can make an attacker-chosen reserved username Authenticated. Server-injected
  internal identities bypass authentication entirely and are unaffected. Config
  tokenization + `ValidateAuthzConfig` reject NUL/reserved names, so no local reserved
  user can exist either.
- **`reactor.ExecuteCommand` (reactor.go:740) builds an empty-username CommandContext.**
  Pre-existing, no production caller, can only fail closed -- left as-is (a NOTE, not a
  hole).

## Files

- internal/component/authz/authz.go (S-1/S-2/S-3 -> Deny; `hasUsers` removed; `authzLogger`)
- internal/component/aaa/reserved.go (NEW: reserved prefix, recovery profile, `IsReservedName`/`FilterReservedNames`)
- internal/component/aaa/login_profiles.go (reserved-username reject at choke point; anti-central-filter invariant comment)
- internal/component/radius/authenticator.go, internal/component/tacacs/authenticator.go (drop reserved names on wire ingress)
- internal/component/plugin/server/{dispatch_registry.go, dispatch.go, server.go} (reserved internal identity injection)
- internal/component/bgp/config/loader.go (`ValidateAuthzConfig` rejects reserved names; store-existence)
- cmd/ze/hub/main_servers.go (recovery profile delivered via login-profiles)
- test/plugin/authz-{recovery-admin,no-applicable-profile,rpc-identity}.ci
- docs/guide/operator-access-rbac.md, docs/guide/tacacs.md
