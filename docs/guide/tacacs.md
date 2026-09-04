# TACACS+ AAA

Ze authenticates SSH logins against TACACS+ servers (RFC 8907) when the
`system.authentication.tacacs` block is present. Local bcrypt users keep
working as the fallback so an unreachable server cannot lock you out of the
device.

## What it does

| Function | Status | Notes |
|----------|--------|-------|
| Authentication | Production | PAP (login over the SSH password callback). RFC 8907 §5. |
| Accounting | Production | START + STOP records around every dispatched CLI command. |
| Authorization | Production | `authorization true` switches per-command authorization on. By default the bridge falls back to local profiles on TACACS+ ERROR; `strict-fallback true` denies instead. |

<!-- source: internal/component/tacacs/authenticator.go -- TacacsAuthenticator.Authenticate -->
<!-- source: internal/component/tacacs/accounting.go -- TacacsAccountant.CommandStart/Stop -->
<!-- source: internal/component/tacacs/authorizer.go -- TacacsAuthorizer.Authorize -->

## Minimal config

```
system {
    authentication {
        tacacs {
            server 10.0.0.1 { port 49; key "$9$encrypted-key"; }
            server 10.0.0.2 { port 49; key "$9$encrypted-key"; }
            timeout 5
        }
        tacacs-profile 15 { profile [ admin ]; }
        tacacs-profile 1  { profile [ read-only ]; }
    }
    authorization {
        profile admin     { run { default-action allow; } edit { default-action allow; } }
        profile read-only { run { default-action allow; } edit { default-action deny;  } }
    }
}
```

| Leaf | Type | Default | Notes |
|------|------|---------|-------|
| `tacacs.server <ip>` | list, ordered-by-user | - | Tried in declaration order on connection failure |
| `tacacs.server <ip>.port` | uint16 | 49 | TCP |
| `tacacs.server <ip>.key` | string (`ze:sensitive`) | required | Shared secret. A server with none disables the whole AAA bundle. See below: what that costs depends on when the bundle is built |
| `tacacs.timeout` | uint16 (1-300) | 5 | Per-server connection timeout in seconds |
| `tacacs.source-address` | ip-address | none | Local source IP for outbound TACACS+ TCP |
| `tacacs.authorization` | boolean | false | Enable per-command TACACS+ authorization |
| `tacacs.strict-fallback` | boolean | false | Deny authorization when TACACS+ is unavailable instead of falling back to local RBAC |
| `tacacs.accounting` | boolean | false | Enable START/STOP accounting records |
| `tacacs-profile <N>.profile` | leaf-list | required | Maps priv-lvl `N` (0-15) to one or more local authz profiles |

<!-- source: internal/component/tacacs/yang/ze-tacacs-conf.yang -- system.authentication.tacacs -->

The key is not optional. RFC 8907 Section 4.5 builds the obfuscation pad from
the shared secret. Section 10.5.2 says "TACACS+ clients MUST NOT set
TAC_PLUS_UNENCRYPTED_FLAG", which is the only honest wire form for an
unobfuscated body. A client with no secret therefore has no conformant packet to
send.

Two refusals follow, and neither one stops the daemon. The AAA build refuses a
server declared without a key and names the address. The packet writer refuses
one too. No path reaches the socket with a cleartext body under a header that
claims otherwise.

What a keyless server costs depends on what else the config declares.

**The tacacs backend is dropped and the rest of the chain composes without it.**
A user who exists locally still logs in against the local backend, and the local
authorization profiles still govern what they run. The daemon logs the drop at
ERROR.

| What the config has | What happens |
|---------------------|--------------|
| A local user as well | that user logs in, and their profiles decide what they run |
| No other backend that builds | nothing authenticates, and ssh is not started |

The chain order is unchanged. TACACS+ is asked first where it built, a reject
stops the chain, and only a failure to ANSWER reaches the local account.

A commit is REFUSED rather than dropped, and only while a chain is already
running. The same error already in the file at boot is logged and startup
continues. So `ze config validate` before a reboot.

The `ze:sensitive` marking hides the key from `show` and from the web editor. It
does NOT encrypt what the commit path writes. Ze decodes a `$9$` value you write
by hand, and the editor stores a key as you typed it. Only `ze config dump`
encodes on the way out, so a dump round-trips and the key stays hidden.

<!-- source: internal/component/tacacs/register.go -- tacacsBackend.Build -->
<!-- source: cmd/ze/hub/main_reload.go -- the reload refusal -->
<!-- source: cmd/ze/hub/main.go -- the boot warning, noBGPAAAWiring -->
<!-- source: internal/component/ssh/ssh.go -- the LocalAuthenticator fallback -->
<!-- source: internal/component/tacacs/packet.go -- MarshalInto, ErrNoSharedSecret -->

## Authentication flow

1. SSH client connects with username + password.
2. Daemon's AAA chain calls `TacacsAuthenticator` first (priority 100; local
   bcrypt is priority 200).
3. The client opens TCP to the first configured server and sends a PAP
   AUTHEN START. The body is XOR-encrypted with the MD5 pseudo-pad keyed
   on the shared secret.
4. **PASS** -- the server's reply data byte is the priv-lvl. The
   authenticator looks up `tacacs-profile <priv-lvl>.profile`. A matching
   entry yields the authz profiles attached to the SSH session. An
   unmapped priv-lvl rejects the login (AC-18) so adding new TACACS+
   levels in the upstream server does not accidentally grant access.
5. **FAIL** -- explicit rejection. The chain stops here. Local bcrypt is
   NOT tried. This prevents a wrong password against TACACS+ from
   succeeding via a stale local hash.
6. **Connection error / ERROR status** -- the next server in the list is
   tried. When every server is unreachable (or all return ERROR) the
   chain falls through to the local bcrypt authenticator.

<!-- source: internal/component/aaa/aaa.go -- ChainAuthenticator, ErrAuthRejected -->
<!-- source: internal/component/tacacs/authenticator.go -- handlePass, AuthenStatusFail handling -->

## Privilege level mapping

TACACS+ servers send a numeric priv-lvl (0-15) in the AUTHEN REPLY. Ze's
internal authorization model is name-based, so each priv-lvl must be
mapped to one or more locally-defined `system.authorization.profile`
entries.

| priv-lvl | Common convention | Example mapping |
|----------|-------------------|-----------------|
| 15 | full administrator | `profile [ admin ]` |
| 5  | site operator | `profile [ operator ]` |
| 1  | read-only / NOC | `profile [ read-only ]` |
| 0  | minimal access | rarely used; map only if the upstream server returns it |
| 2-14 | site-defined | only map the levels your TACACS+ server actually returns |

Levels not present in `tacacs-profile`, and levels mapped to an empty profile
list, reject the login. Look for `TACACS+ unmapped privilege level` in the daemon
log when extending the upstream config. A TACACS+ user therefore always reaches
authorization with at least one resolved profile, or does not authenticate at
all: there is no path where an authenticated TACACS+ user has no profile.

Even so, authorization fails closed at the second layer: if a session ever reaches
authorization resolving no applicable profile, every command is denied rather than
allowed. A priv-level whose mapping names only profiles you have not defined
resolves nothing and is denied, not granted admin. Local break-glass accounts
(the `ze init` bootstrap admin, or any `system.authentication.user` you keep) are
the way back into a box whose mapping is wrong.

### What the mapped profiles govern

The mapped profiles decide every command the session may run after login
succeeds. Ze resolves them once at authentication and authorizes each command
against them, so `tacacs-profile 1 { profile [ read-only ]; }` gives that
session exactly what the local `read-only` profile allows and refuses the rest
with `command restricted by access control`.

Only the profile *names* are fixed at login. Each command is evaluated against
the profile as it is defined at that moment, so editing `read-only` and
committing applies to sessions already open, without a reconnect.

A local `system.authentication.user` block with the same username takes
precedence over the mapped profiles: an explicit local assignment is a stated
intent for that name, so it is not widened or narrowed by how the user logged in.

Verify the mapping with a command the profile denies, not with one Ze does not
have. A command that does not exist reports `unknown command` and exits non-zero
for everyone, so it passes whatever the mapping says and proves nothing.

<!-- source: internal/component/aaa/login_profiles.go -- login-resolved profiles reaching authorization -->
<!-- source: internal/component/authz/authz.go -- Store.Authorize, config assignment precedence -->
<!-- source: test/plugin/tacacs-readonly.ci -- priv-lvl 1 allowed a read, refused a write -->
<!-- source: test/plugin/tacacs-author.ci -- per-command AUTHOR REQUEST, FAIL blocks the command -->
<!-- source: internal/component/tacacs/authorizer.go -- per-command authorization when `authorization true` -->

Setting `authorization true` moves the per-command decision to the TACACS+ server
itself: Ze sends an AUTHOR REQUEST per command, and the profiles above apply only
as the fallback when the server is unreachable, unless `strict-fallback true`
makes that case deny.

<!-- source: internal/component/tacacs/authenticator.go -- handlePass priv-lvl lookup -->

## Accounting

When `accounting true` is set, every command dispatched through the CLI
emits two records:

| Flag | When | Args |
|------|------|------|
| START (0x02) | Just after authorization passes, before the handler runs | `task_id`, `service=shell`, `cmd=<input>`, `start_time` |
| STOP (0x04)  | After the handler returns, regardless of outcome | `task_id`, `service=shell`, `cmd=<input>`, `stop_time` |

Records are queued to a single long-lived background worker. The worker
sends one record at a time over the same TACACS+ client used for
authentication, with the same server failover. Accounting failures are
logged (`TACACS+ accounting failed`) and never block the command. Records
that cannot be queued increment the local drop counter.

Use `ze show aaa accounting` to inspect the counter:

```
ze show aaa accounting
```

The response includes `dropped-records`. A non-zero value means at least one
START/STOP record was lost locally before the TACACS+ client could send it.

<!-- source: internal/component/plugin/server/command.go -- Dispatcher accountant hook -->
<!-- source: internal/component/tacacs/accounting.go -- worker, processOne, enqueue -->

## Verification

The `.ci` tests in `test/plugin/` cover the main behaviours:

| Test | Asserts |
|------|---------|
| `tacacs-auth.ci` | TACACS+ PASS + priv-lvl 15 -> admin profile, no local fallback consulted |
| `tacacs-author.ci` | TACACS+ command authorization PASS/FAIL with local fallback |
| `tacacs-fallback.ci` | Server unreachable -> local bcrypt accepted, log shows `source=local` |
| `tacacs-local-only.ci` | No `tacacs` block -> existing local-only auth path unchanged |
| `tacacs-readonly.ci` | Read-only profile restricts write commands |
| `tacacs-acct.ci` | `accounting true` -> mock receives ACCT START followed by STOP |
| `tacacs-singleconnect.ci` | Single-connect mode TCP reuse |
| `tacacs-show.ci` | `ze tacacs show` reachability probe, in its table and `| json` renderings |

Strict fallback is covered by `TestExtractConfigStrictFallback` and
`TestTacacsAuthorizerStrictFallbackDeniesUnreachable` in the TACACS+ unit tests.

For ad-hoc verification, point the daemon at a real TACACS+ server and
run any command via `ze cli -c "show bgp"` -- the daemon log tags the
satisfying backend on every login, e.g.:

```
INFO SSH auth success subsystem=ssh username=alice remote=10.0.0.1:51408 source=tacacs
```

`source=tacacs` confirms the chain consulted TACACS+ and returned PASS.
`source=local` means TACACS+ was unreachable (or unconfigured) and the
local bcrypt user accepted the credentials.

<!-- source: internal/test/mock/tacacs/tacacs.go -- ze-test tacacs-mock for .ci tests -->

## Operational notes

- **Shared secrets** are stored as `$9$`-encoded ciphertext, never as
  plaintext. The CLI never echoes them; `ze config dump --strip-private`
  replaces them with `/* SECRET-DATA */`.
- **VRF**: when the SSH server runs in a non-default VRF, TACACS+ TCP
  connections inherit the same VRF context.
- **Single-connect mode** (RFC 8907 §4.4) is tested via `tacacs-singleconnect.ci`.
- **Operational tooling**: `ze tacacs show <config>` probes every TACACS+
  server the config names and reports whether it answers, with no daemon
  running. Runtime `ze show aaa accounting` exposes local accounting queue
  drops.

## RFC reference

- RFC 8907 -- The TACACS+ Protocol (formalises the original Cisco draft).
  Local summary: `rfc/short/rfc8907.md`.
