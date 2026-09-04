# RADIUS admin AAA

Ze authenticates operator logins (SSH and web) against RADIUS servers
(RFC 2865) when the `system.authentication.radius` block is present. Local
bcrypt users keep working as the fallback so an unreachable server cannot lock
you out of the device.

MCP is not on this path. The MCP server authenticates a request against a
bearer-token digest or an OAuth JWT and never builds an AAA request, so a
RADIUS operator cannot log in to it.

<!-- source: internal/component/mcp/bearer.go -- bearerAuthenticator.Authenticate -->
<!-- source: internal/component/mcp/oauth.go -- oauthAuthenticator.Authenticate -->

This is the operator/admin login path. It is separate from the L2TP
**subscriber** RADIUS path (`l2tp.auth.radius`), which authenticates PPP
sessions and lives under the `l2tp` config root. That path also does subscriber
accounting and CoA/DM; see [L2TP](l2tp.md#l2tp-auth-radius) for the attributes an
Accounting-Request carries.

## What it does

| Function | Status | Notes |
|----------|--------|-------|
| Authentication | Production | PAP (RFC 2865 User-Password, hidden per §5.2 with the shared secret), CHAP (§5.3 CHAP-Password with a §5.40 CHAP-Challenge), or EAP with MD5-Challenge or MS-CHAPv2 inside EAP-Message attributes (RFC 3579 §3.1). `auth-method` selects one; the default is `pap`. |
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
            auth-method pap
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
| `radius.server <ip>.key` | string, length 1..max (`ze:sensitive`) | required | Shared secret, stored as `$9$` ciphertext. RFC 2865 §3 forbids an empty secret, so a zero-length key is refused at config load and the RADIUS backend is not built |
| `radius.timeout` | uint16 (1-60) | 3 | Per-server request timeout in seconds |
| `radius.retries` | uint8 (0-10) | 3 | Transmit attempts per server before failover |
| `radius.source-address` | ip-address | none | Local source IP for outbound RADIUS UDP |
| `radius.auth-method` | enum `pap`, `chap`, `eap-md5`, `eap-mschapv2` | `pap` | Credential the Access-Request carries. `chap` and `eap-md5` need the server to hold the password in cleartext; `eap-mschapv2` needs the NT hash (see below) |
| `radius.profile-attribute` | enum `filter-id` | `filter-id` | Access-Accept reply attribute carrying profile name(s) |
| `radius.default-profile` | leaf-list | none | Profiles applied when the Access-Accept carries no `profile-attribute` |

<!-- source: internal/component/radius/yang/ze-radius-conf.yang -- system.authentication.radius -->
<!-- source: internal/component/radius/config.go -- ExtractConfig -->

## Authentication flow

1. An SSH or web client connects with username + password.
2. The daemon's AAA chain calls the RADIUS backend first (priority 50; TACACS+
   is priority 100, local bcrypt is priority 200).
3. The backend builds an Access-Request with User-Name, the credential
   `auth-method` selects, Service-Type=Login, NAS-Identifier (hostname), and
   (when a source-address is set) NAS-IP-Address. A login that carries no username
   sends no User-Name attribute, because RFC 2865 §5 forbids sending text of
   length zero. It sends to the first configured server, retransmitting with
   exponential backoff and failing over to the next server (RFC 2865 §2.5).
   Each server gets a new Identifier and, with it, a new Request Authenticator
   and a re-encoded User-Password (RFC 2865 §4.1).
4. **Access-Accept** -- the login succeeds, unless the reply names a
   Service-Type other than Login-User. Admin login is the one service this path
   provides, so any other Service-Type is treated as an Access-Reject (RFC 2865
   §5.6 and §1.1). Profiles come from the configured reply attribute (see
   below); the session is tagged `source=radius`.
5. **Access-Reject** -- explicit rejection. The chain stops here; local bcrypt
   is NOT tried. This prevents a wrong password against RADIUS from succeeding
   via a stale local hash.
6. **Access-Challenge** -- the answer depends on `auth-method`.
   - With `pap` or `chap` it is treated as an Access-Reject and the chain stops.
     Those two send one Access-Request and read one answer, so ze does not
     support challenge/response for them (RFC 2865 §4.4). This is how the
     transports are wired, not a limit of SSH: ze registers password and
     public-key auth only, and SSH keyboard-interactive, which does have a round
     trip back to the operator, is not registered.
   - With `eap-md5` or `eap-mschapv2` it is a round of the EAP conversation. Ze
     answers as the EAP peer, using the password the operator already typed, and
     sends a new Access-Request carrying the peer's EAP-Response and the
     challenge's State attribute unchanged (RFC 2865 §5.24). No round trip to
     the operator is needed, which is why EAP works over SSH password auth and
     the web login form alike. See [EAP](#eap) below.
7. **Timeout / all servers unreachable** -- treated as an infrastructure error,
   so the chain falls through to the next backend (local bcrypt). An
   unreachable RADIUS server never locks the operator out.

The total time one login spends talking to RADIUS is bounded, so a slow or
unreachable server falls through to local rather than hanging the login.

<!-- source: internal/component/aaa/aaa.go -- ChainAuthenticator, ErrAuthRejected -->
<!-- source: internal/component/radius/authenticator.go -- Accept/Reject/error mapping, authBudget -->

## Choosing the credential

`auth-method` selects what the Access-Request carries. RFC 2865 §4.1 permits one
credential and never two, and RFC 3579 §3.3 Note 1 extends the same exclusion to
EAP: "An Access-Request that contains either a User-Password or CHAP-Password or
ARAP-Password or one or more EAP-Message attributes MUST NOT contain more than
one type of those four attributes." Ze sends exactly one.

| | `pap` (default) | `chap` | `eap-md5` | `eap-mschapv2` |
|---|---|---|---|---|
| Attribute | User-Password (2) | CHAP-Password (3) plus CHAP-Challenge (60) | EAP-Message (79) plus Message-Authenticator (80) | EAP-Message (79) plus Message-Authenticator (80) |
| What travels | The password, XORed with MD5(secret + Request Authenticator) (§5.2) | MD5(identifier, password, challenge) (§5.3) | MD5(identifier, password, challenge), inside EAP (RFC 3748 §5.4) | The MS-CHAPv2 NT-Response (RFC 2759 §4) |
| Recoverable from a capture | Yes, by anyone holding the shared secret: the §5.2 hiding is a reversible XOR | No | No | No |
| Server must store | A hash or the cleartext | The cleartext | The cleartext | The NT hash |
| RADIUS round trips | 1 | 1 | 2 or more | 3 or more |

**The cost of `chap` is at the server.** RFC 2865 §2.2: "CHAP requires that the
user's password be available in cleartext to the server so that it can encrypt
the CHAP challenge and compare that to the CHAP response. If the password is not
available in cleartext to the RADIUS server then the server MUST send an
Access-Reject to the client." A server that stores password hashes rejects every
`chap` login, which the `radius-admin-chap-hashed-freeradius` interop scenario
proves against a real FreeRADIUS rather than asserting. Check how your server
stores operator passwords before you select it. Your local bcrypt account is a separate credential and still works, so a
wrong choice is recoverable.

Ze is the NAS and holds the operator's password, so it generates the challenge
and the CHAP Identifier itself, from `crypto/rand`, freshly per login. §5.3 and
§5.40 describe both as coming from a PPP peer; admin login has no PPP peer, and
nothing on the wire differs, because the server verifies the same digest over
the same three inputs. The challenge goes in CHAP-Challenge (60) rather than the
Request Authenticator, which §5.40 permits as a MAY, because the Request
Authenticator already carries the reply verification ze checks on the way back.
A challenge that cannot be generated fails the login as an infrastructure error,
so the chain falls through to the next backend rather than sending a predictable
one.

`ze doctor` probes with a fixed PAP credential whatever `auth-method` says. The
probe tests reachability and the shared secret, and an Access-Reject answers
both.

<!-- source: internal/component/radius/chap.go -- chapCredential, chapResponse -->
<!-- source: internal/component/radius/authenticator.go -- radiusAuthenticator.credential -->

## EAP

`eap-md5` and `eap-mschapv2` run a full RADIUS/EAP conversation (RFC 3579).
**Ze answers EAP itself, as the peer.** The operator types a password into SSH or
the web login, ze holds it, and ze runs the EAP method against the server on the
operator's behalf. Nothing in the login transport changes and no EAP frame
reaches the operator's client, so both methods work over every login surface.

One login is a sequence of Access-Requests:

1. Ze sends its own EAP-Request/Identity to its peer half, and puts the
   EAP-Response/Identity that comes back into EAP-Message attributes. The
   User-Name attribute carries the Type-Data of that Response (RFC 3579 §2.1).
2. The server answers Access-Challenge, carrying its EAP-Request and usually a
   State attribute.
3. Ze feeds the EAP-Request to its peer and sends the peer's EAP-Response in a
   new Access-Request, with the State copied byte for byte. Ze never reads
   State: RFC 2865 §5.24 says "the client MUST NOT interpret the attribute
   locally".
4. The exchange ends on Access-Accept or Access-Reject. Profiles map exactly as
   they do for PAP.

Each round is a **new** Access-Request with its own Identifier and Request
Authenticator, not a retransmission of the previous one.

**Every EAP packet is protected.** RFC 3579 §3.1: "the Message-Authenticator
attribute MUST be used to protect all Access-Request, Access-Challenge,
Access-Accept, and Access-Reject packets containing an EAP-Message attribute."
Ze signs each request with HMAC-MD5 over the whole packet, keyed with the shared
secret and with the signature field taken as sixteen zero octets (§3.2). A reply
whose Message-Authenticator does not verify is **silently discarded**, not
rejected: the Access-Request stays outstanding and the client retransmits, so a
forged packet cannot end a login. If the conversation then runs out of retries
the login fails as an infrastructure error and the chain tries the next backend.

An EAP packet longer than 253 octets is split across consecutive EAP-Message
attributes, and the values of a reply's attributes are concatenated back into
one packet (§3.1). One RADIUS packet carries exactly one EAP packet.

**Two bounds end a conversation the server will not conclude,** and the server
can disable neither. The EAP peer counts every round against its own cap of 20,
and the login carries the same time budget every `auth-method` carries. Either
one ends the login with an infrastructure error, so the chain falls through to
local rather than hanging.

**Choosing between them.** `eap-md5` carries the same cleartext-password
requirement as `chap`, for the same reason: the server recomputes an MD5 over the
password. `eap-mschapv2` needs the NT hash instead, which is what a server backed
by Active Directory or by a `samba` password database holds. EAP-TLS is
implemented in ze's EAP peer and is **not** offered here: it needs an operator
certificate and key, which is a different credential model and a later feature.
Ze does not negotiate among several methods either. It NAKs toward the one
`auth-method` names, so a server that offers something else ends the login.

### Configuring FreeRADIUS for `eap-mschapv2`

A FreeRADIUS 3.2 server needs one `authenticate` section that an operator does
not expect, and the conventional spelling of it is never reached.
`rlm_eap_mschapv2` runs the peer's NT-Response through the `mschap` module as an
inner request. It resolves the section to run with
`dict_valbyname(PW_AUTH_TYPE, 0, "MSCHAP")`, and falls back to `"MS-CHAP"` only
when that lookup misses
(`src/modules/rlm_eap/types/rlm_eap_mschapv2/rlm_eap_mschapv2.c:83` in
`release_3_2_7`). FreeRADIUS registers an Auth-Type value for every module
instance it loads, so `mschap` is already registered and the case-insensitive
first lookup takes it. Name the section after the module instance:

```
authenticate {
    Auth-Type mschap {
        mschap
    }
    eap
}
```

A section written `Auth-Type MS-CHAP { mschap }` loads without complaint and is
never run. The server then answers a correct NT-Response with
`Auth-Type sub-section not found.  Ignoring.` in its log and an EAP-Failure on
the wire, which reads like a wrong password and is not one. Measured on
2026-09-04 against `docker.io/freeradius/freeradius-server:3.2.7`. The lab's
whole virtual server, with the reasoning beside the section, is
`test/interop-radius/site-default`.

<!-- source: test/interop-radius/site-default -- the authenticate sections the lab server runs -->
<!-- source: internal/component/radius/authenticator_eap.go -- authenticateEAP, eapCredential -->
<!-- source: internal/component/radius/eap.go -- appendEAPMessage, eapPacketFrom -->
<!-- source: internal/component/radius/packet.go -- SignMessageAuthenticator -->
<!-- source: internal/core/eap -- NewPeerSession, PeerSession.Process -->

## Profile mapping

Ze's authorization model is name-based, so a RADIUS-accepted user must be
mapped to one or more locally-defined `system.authorization.profile` entries.

- The backend reads the configured `profile-attribute` from the Access-Accept.
  With the default `filter-id`, each Filter-Id attribute (RFC 2865 §5.11)
  supplies one profile name; multiple Filter-Id attributes yield multiple
  profiles. Filter-Id is the only value: RFC 2865 §5.25 says of the Class
  attribute that "the client MUST NOT interpret the attribute locally", so ze
  reads no profile name out of it.
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

## A config the chain cannot build

The order above is what happens when RADIUS ANSWERS. A RADIUS block the daemon
cannot build a client from is a different case. So is a TACACS+ server declared
with no shared secret in the same config. The whole AAA chain then fails to
build, and no backend answers at all.

**Login falls over to the local accounts, on ssh and on web. Authorization does
not: it fails closed.** You log in, see the failure, and repair the box from the
console. With no chain installed there is no policy to consult. A daemon that
cannot build the chain its config describes must not authorize most freely. No
local user means no login either.

A commit is refused rather than failed over, and only while a chain is already
running. The same error already in the file at boot is logged and startup
continues.

`docs/guide/authentication.md` carries the operator view and
`docs/architecture/aaa-tacacs.md` the mechanism.

<!-- source: cmd/ze/hub/infra_setup.go -- the ssh build condition and its authenticator fallback -->
<!-- source: cmd/ze/hub/main_reload.go -- the reload refusal -->

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
| `aaa-radius-chap.ci` | `auth-method chap` -> the server verifies ze's digest and accepts; the mock rejects a request carrying both credentials, so a green run also proves no User-Password was sent |
| `aaa-radius-eap.ci` | `auth-method eap-mschapv2` -> the mock challenges, ze answers with the State copied back, and the login completes. The mock computes the Message-Authenticator itself and DISCARDS a request whose signature does not verify, so a signer covering the wrong octets ends this test in a timeout |

Those four drive `internal/test/mock/radius/radius.go`, which ze wrote. A mock
built beside ze's own encoder agrees with ze by construction, so a second set of
proofs runs the same paths against a real FreeRADIUS server at a pinned tag:

| Interop scenario | Asserts |
|------------------|---------|
| `radius-admin-pap-freeradius` | An operator logs in over ze's SSH listener, FreeRADIUS records `verdict=accept` for the request ze sent, ze's log says `source=radius`, the Filter-Id profile decides which commands run, and the local account's own password is refused rather than accepted by a fall-through |
| `radius-admin-chap-freeradius` | A server ze did not write reproduces ze's CHAP digest from its own `Cleartext-Password` entry, and its record shows a CHAP-Password with no User-Password beside it |
| `radius-admin-chap-hashed-freeradius` | The §2.2 consequence above is real: the same entry stored hashed accepts the same password over PAP and refuses it over CHAP, and ze authenticates the operator through no backend at all |
| `radius-admin-eap-freeradius` | `auth-method eap-mschapv2` completes against a server ze did not write: FreeRADIUS computes the Message-Authenticator, the MS-CHAPv2 challenge and the authenticator response from its own code, and its record shows an EAP-Message with neither password attribute beside it and at least one round that carried the State back |

Run them with `./le integration interop-radius`, or one at a time with
`RADIUS_INTEROP_SCENARIO=<name>`. They need Docker and no kernel module.

Unit coverage lives in `internal/component/radius/{config,authenticator,aaa,chap,doctor}_test.go`.
The `auth-method` enum is pinned against the schema by
`internal/component/config/radius_auth_method_enum_test.go`.

For ad-hoc verification, point the daemon at a real RADIUS server and run any
command via `ze cli -c "show bgp"` -- the daemon log tags the satisfying
backend on every login, e.g.:

```
INFO SSH auth success subsystem=ssh username=alice remote=10.0.0.1:51408 source=radius
```

`source=radius` confirms the chain consulted RADIUS and returned Access-Accept.
`source=local` means RADIUS was unreachable (or unconfigured) and the local
bcrypt user accepted the credentials.

<!-- source: internal/test/mock/radius/radius.go -- ze-test radius-mock for .ci tests -->
<!-- source: internal/test/mock/radius/eap.go -- the mock's EAP branch: buildEAPResponse, verifyRequestSignature -->
<!-- source: internal/le/interoplab/radius/checkers.go -- what each FreeRADIUS scenario asserts -->
<!-- source: test/interop-radius/scenarios/ -- the four interop scenario directories -->

## Operational notes

- **Shared secrets** are stored as `$9$`-encoded ciphertext, never as
  plaintext. The CLI never echoes them; `ze config dump --strip-private`
  replaces them with `/* SECRET-DATA */`.
- **PAP, CHAP, EAP-MD5 or EAP-MSCHAPv2**, selected by `auth-method`, default
  `pap`. EAP-TLS is not offered: it needs an operator certificate and key. The
  L2TP subscriber path handles CHAP and MS-CHAPv2 for PPP sessions through its
  own code, and this leaf does not affect it.
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
