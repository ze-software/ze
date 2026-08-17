# RADIUS admin AAA

Ze authenticates operator logins (SSH, web, MCP) against RADIUS servers
(RFC 2865) when the `system.authentication.radius` block is present. Local
bcrypt users keep working as the fallback so an unreachable server cannot lock
you out of the device.

This is the operator/admin login path. It is separate from the L2TP
**subscriber** RADIUS path (`l2tp.auth.radius`), which authenticates PPP
sessions and lives under the `l2tp` config root. That path also does subscriber
accounting and CoA/DM; see [L2TP](../l2tp/index.md#l2tp-auth-radius) for the attributes an
Accounting-Request carries.

## What it does

| Function | Status | Notes |
|----------|--------|-------|
| Authentication | Production | PAP (RFC 2865 User-Password, hidden per §5.2 with the shared secret). |
| Authorization | Via profiles | An Access-Accept reply attribute (default Filter-Id) maps the user to local RBAC profiles. |
| Accounting | Not in MVP | Admin-session accounting is deferred; subscriber accounting stays in the L2TP path. |

<!-- source: internal/component/radius/authenticator.go -- radiusAuthenticator.Authenticate -->
<!-- source: internal/component/radius/aaa.go -- radiusBackend.Build -->

## Minimal config

```
system {
    authentication {
        radius {
            server 10.0.0.1 { port 1812; key "$9$encrypted-key"; }
            server 10.0.0.2 { port 1812; key "$9$encrypted-key"; }
            timeout 3
            retries 3
            profile-attribute filter-id
            default-profile read-only
        }
    }
    authorization {
        profile admin     { run { default-action allow; } edit { default-action allow; } }
        profile read-only { run { default-action allow; } edit { default-action deny;  } }
    }
}
```

| Leaf | Type | Default | Notes |
|------|------|---------|-------|
| `radius.server <ip>` | list, ordered-by-user | - | IPv4 only (shared udp4 client); tried in declaration order on failure |
| `radius.server <ip>.port` | uint16 (1-65535) | 1812 | UDP authentication port |
| `radius.server <ip>.key` | string (`ze:sensitive`) | required | Shared secret, stored as `$9$` ciphertext |
| `radius.timeout` | uint16 (1-60) | 3 | Per-server request timeout in seconds |
| `radius.retries` | uint8 (0-10) | 3 | Transmit attempts per server before failover |
| `radius.source-address` | ip-address | none | Local source IP for outbound RADIUS UDP |
| `radius.profile-attribute` | enum `filter-id` \| `class` | `filter-id` | Access-Accept reply attribute carrying profile name(s) |
| `radius.default-profile` | leaf-list | none | Profiles applied when the Access-Accept carries no `profile-attribute` |

<!-- source: internal/component/radius/yang/ze-radius-conf.yang -- system.authentication.radius -->
<!-- source: internal/component/radius/config.go -- ExtractConfig -->

## Authentication flow

1. SSH/web/MCP client connects with username + password.
2. The daemon's AAA chain calls the RADIUS backend first (priority 50; TACACS+
   is priority 100, local bcrypt is priority 200).
3. The backend builds an Access-Request with User-Name, the hidden
   User-Password, Service-Type=Login, NAS-Identifier (hostname), and (when a
   source-address is set) NAS-IP-Address. It sends to the first configured
   server, retransmitting with exponential backoff and failing over to the next
   server (RFC 2865 §2.5).
4. **Access-Accept** -- the login succeeds. Profiles come from the configured
   reply attribute (see below); the session is tagged `source=radius`.
5. **Access-Reject** -- explicit rejection. The chain stops here; local bcrypt
   is NOT tried. This prevents a wrong password against RADIUS from succeeding
   via a stale local hash.
6. **Timeout / all servers unreachable** -- treated as an infrastructure error,
   so the chain falls through to the next backend (local bcrypt). An
   unreachable RADIUS server never locks the operator out.

The total time one login spends talking to RADIUS is bounded, so a slow or
unreachable server falls through to local rather than hanging the login.

<!-- source: internal/component/aaa/aaa.go -- ChainAuthenticator, ErrAuthRejected -->
<!-- source: internal/component/radius/authenticator.go -- Accept/Reject/error mapping, authBudget -->

## Profile mapping

Ze's authorization model is name-based, so a RADIUS-accepted user must be
mapped to one or more locally-defined `system.authorization.profile` entries.

- The backend reads the configured `profile-attribute` from the Access-Accept.
  With the default `filter-id`, each Filter-Id attribute (RFC 2865 §5.11)
  supplies one profile name; multiple Filter-Id attributes yield multiple
  profiles. `class` reads the Class attribute (§5.25) the same way.
- When the Access-Accept carries no such attribute, the `default-profile`
  leaf-list applies.
- When neither is present the login resolves to no profile and is **rejected**,
  with a `RADIUS admin auth rejected: no profiles resolved` warning naming the
  user. Ze never authorizes a user it cannot attach a profile to: an
  authenticated login always carries at least one profile name. The rejection
  stops the chain, exactly as an Access-Reject does, because the server did
  answer -- this is not the unreachable-server case that falls through to local.

Configure your RADIUS server to return `Filter-Id = admin` (or your profile
name) for operators, and define a matching `system.authorization.profile`. If
your server does not send a profile attribute, set `default-profile` instead;
otherwise no RADIUS user can log in.

<!-- source: internal/component/radius/authenticator.go -- mapProfiles, Authenticate Access-Accept branch -->

## Chain order with TACACS+ and local

When RADIUS, TACACS+, and local users are all configured, the chain is tried in
priority order: RADIUS (50), then TACACS+ (100), then local bcrypt (200). A
reject from any backend stops the chain; an infrastructure error falls through
to the next.

<!-- source: internal/component/radius/aaa.go -- aaaPriority -->
<!-- source: internal/component/tacacs/register.go -- Priority 100 -->

## Readiness check

`ze doctor` probes every configured RADIUS admin server with an Access-Request
before the daemon starts. When none answers a verifiable response it emits
`doctor-radius-admin-unreachable` (a warning: local fallback still works). A
missing or wrong shared key counts as unreachable.

```
ze doctor --json router.conf
ze explain doctor-radius-admin-unreachable
```

<!-- source: internal/component/radius/doctor.go -- checkRadiusAdminServers -->

## Verification

The `.ci` tests in `test/plugin/` cover the main behaviours:

| Test | Asserts |
|------|---------|
| `aaa-radius-admin.ci` | Access-Accept + Filter-Id -> admin profile, log shows `source=radius`, no local fallback consulted |
| `aaa-radius-fallback.ci` | Server unreachable -> local bcrypt accepted, log shows `source=local`, RADIUS did not silently succeed |

Unit coverage lives in `internal/component/radius/{config,authenticator,aaa,doctor}_test.go`.

For ad-hoc verification, point the daemon at a real RADIUS server and run any
command via `ze cli -c "show bgp summary"` -- the daemon log tags the satisfying
backend on every login, e.g.:

```
INFO SSH auth success subsystem=ssh username=alice remote=10.0.0.1:51408 source=radius
```

`source=radius` confirms the chain consulted RADIUS and returned Access-Accept.
`source=local` means RADIUS was unreachable (or unconfigured) and the local
bcrypt user accepted the credentials.

<!-- source: internal/test/mock/radius/radius.go -- ze-test radius-mock for .ci tests -->

## Operational notes

- **Shared secrets** are stored as `$9$`-encoded ciphertext, never as
  plaintext. The CLI never echoes them; `ze config dump --strip-private`
  replaces them with `/* SECRET-DATA */`.
- **PAP only** in the MVP. CHAP and EAP admin auth are follow-up work; the
  L2TP subscriber path already handles CHAP/MS-CHAPv2 for PPP sessions.
- **Same client** as the L2TP subscriber path (`internal/component/radius`),
  so retransmit, failover, and Response-Authenticator verification behave
  identically. That client is `udp4`, so RADIUS admin servers must be IPv4.
- **Fail-open on misconfig.** If the RADIUS client cannot start (for example an
  unbindable `source-address`), the backend contributes nothing and logs an
  error rather than failing the whole AAA bundle, so local (and TACACS+) still
  authenticate and you are never locked out.

## RFC reference

- RFC 2865 -- Remote Authentication Dial In User Service (RADIUS).
  Local summary: `rfc/short/rfc2865.md`.
