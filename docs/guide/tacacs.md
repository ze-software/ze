# TACACS+ AAA

Ze authenticates SSH logins against TACACS+ servers (RFC 8907) when the
`system.authentication.tacacs` block is present. Local bcrypt users keep
working as the fallback so an unreachable server cannot lock you out of the
device.

## What it does

| Function | Status | Notes |
|----------|--------|-------|
| Authentication | Production | PAP login followed by shell-session authorization for profile assignment. RFC 8907 sections 5 and 9. |
| Accounting | Production | START and STOP records for every entered command, including denied and unknown commands. |
| Authorization | Production | `authorization true` enables per-command authorization. ERROR tries configured backups before local fallback; `strict-fallback true` denies when all servers are unavailable. |

<!-- source: internal/component/tacacs/authenticator.go -- TacacsAuthenticator.Authenticate -->
<!-- source: internal/component/tacacs/accounting.go -- TacacsAccountant.CommandStart/Stop -->
<!-- source: internal/component/tacacs/authorizer.go -- TacacsAuthorizer.Authorize -->

## Minimal config

```
system {
    authentication {
        tacacs {
            server 10.0.0.1 { port 49; key "unique-secret-for-the-first-server"; }
            server 10.0.0.2 { port 49; key "unique-secret-for-the-backup-server"; }
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
| `tacacs.server <ip>` | list, ordered-by-user | - | Tried in declaration order on connection failure or ERROR |
| `tacacs.server <ip>.port` | uint16 | 49 | TCP |
| `tacacs.server <ip>.key` | string (`ze:sensitive`) | required | Shared secret. A missing key refuses the TACACS+ backend; other successfully built backends remain available at boot |
| `tacacs.timeout` | uint16 (1-300) | 5 | Per-server connection timeout in seconds |
| `tacacs.source-address` | ip-address | none | Local source IP for outbound TACACS+ TCP. An address that does not parse is refused at load with the address named, the same way a keyless server is, rather than binding the wildcard |
| `tacacs.authorization` | boolean | false | Enable per-command TACACS+ authorization |
| `tacacs.strict-fallback` | boolean | false | Deny authorization when TACACS+ is unavailable instead of falling back to local RBAC |
| `tacacs.accounting` | boolean | false | Enable START/STOP accounting records |
| `tacacs-profile <N>.profile` | leaf-list | required | Maps priv-lvl `N` (0-15) to one or more local authz profiles |

<!-- source: internal/component/tacacs/yang/ze-tacacs-conf.yang -- system.authentication.tacacs -->

Enabling authorization or accounting requires at least one server. A selected
capability with no destination is refused at build, rather than silently omitted.

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

`$9$` is reversible obfuscation, so configuration storage needs credential-grade
access protection. Diagnostic strings, structured logs and JSON configuration
rendering redact the extracted shared key. `ze doctor` warns when a key has fewer
than sixteen characters; keys of thirty-two characters or longer are supported.

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
   AUTHEN START. The MD5 pseudo-pad obfuscates the body using the shared secret.
4. **PASS** -- Ze requests shell-session authorization with `service=shell` and
   an empty `cmd`. The server returns a decimal `priv-lvl` argument, which maps
   through `tacacs-profile <priv-lvl>.profile`. Missing privilege uses level one.
   Unmapped levels and mandatory policy that Ze cannot enforce reject the login.
   Authentication reply data never supplies a privilege level.
5. **FAIL** -- explicit rejection. The chain stops here. Local bcrypt is
   NOT tried. This prevents a wrong password against TACACS+ from
   succeeding via a stale local hash.
6. **Connection error / ERROR status** -- the next server in the list is
   tried. When every server is unreachable (or all return ERROR) the
   chain falls through to the local bcrypt authenticator.

RESTART, FOLLOW and a reply whose unencrypted flag disagrees with the configured
shared secret are terminal denials. A backup server or local password cannot
override them.

The web login fallback follows the same terminal-rejection rule for ordinary
local accounts. The separate ZeFS super-admin remains a recovery path through
its reserved local profile; assigning a normal `admin` profile does not grant
that exception.

Usernames are width-mapped and NFC-normalized without case folding under the
RFC 8265 UsernameCasePreserved profile. PAP passwords and other text fields use
printable US-ASCII; invalid fields are refused without sending them.

Internal plugin/RPC and shared-token API callers keep their trusted local
identities, but use printable wire usernames of the form
`~ze~r:<base64url identity bytes>` without padding. Human usernames beginning
`~ze~` after normalization use the separate form
`~ze~u:<base64url username bytes>` in authentication, authorization and accounting.
Configure server policy for these wire names; a human lookalike cannot acquire
the internal caller's identity. The wire username limit is 255 bytes, so either
encoded form accepts at most 186 input bytes. Longer names are refused rather
than hashed or truncated.
<!-- source: internal/component/tacacs/text.go -- prepareWireUsername -->

<!-- source: internal/component/aaa/aaa.go -- ChainAuthenticator, ErrAuthRejected -->
<!-- source: internal/component/tacacs/authenticator.go -- handlePass, AuthenStatusFail handling -->

## Privilege level mapping

TACACS+ servers return numeric `priv-lvl` values from zero through fifteen in the
shell-session AUTHOR response. Ze maps each value to locally defined
`system.authorization.profile` entries. The parser checks numeric length before
conversion; an oversized mandatory value denies login.

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
With per-command TACACS+ authorization enabled, an existing SSH password
session also uses the current server configuration on its next command.

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

PASS_ADD retains the requested command arguments and applies response attributes.
PASS_REPL uses only the response attributes. The complete effective `service`,
`cmd` and ordered `cmd-arg` values must still describe the command Ze will execute.
A server cannot approve one command while replacing its arguments with another.
Unknown mandatory attributes, session ACLs or timers Ze cannot enforce, and
per-command privilege changes deny authorization. Unsupported optional attributes
may be ignored.

<!-- source: internal/component/tacacs/authenticator.go -- handlePass priv-lvl lookup -->

## Accounting

When `accounting true` is set, each command entering dispatch or the API/SSH
streaming path emits a paired record set:

| Flag | When | Args |
|------|------|------|
| START (0x02) | Before lookup or authorization | `task_id`, `start_time`, `service=shell`, `cmd`, ordered `cmd-arg` values |
| STOP (0x04) | After refusal, handler return or stream completion | Matching `task_id`, `stop_time`, the same service and command arguments |

Command values use reversible Go string escapes without surrounding quotes.
For example, `café` is sent as `caf\u00e9`; an actual backslash is doubled.
The command handler still receives the original argument. TACACS+ command
authorization uses this same lossless representation and denies a request that
cannot fit the protocol's field/count limits.

Accounting alone bounds oversized displays to 255 bytes per argument and 255
arguments per packet. Such a record explicitly includes `ze-command-truncated=1`
and `ze-command-sha256=<digest>` before `service` and `cmd`. Long fields end in
`...`, and excess arguments are omitted. The digest identifies the complete
redacted argument sequence, not the secret values; it and the task ID match
between START and STOP. This preserves evidence that the command was entered
without silently losing the record or altering execution.
<!-- source: internal/component/tacacs/command_text.go -- accountingArguments -->

Records pass through a bounded queue to one background worker. A full queue waits
for capacity, and shutdown drains accepted records. Network failures are logged
and never refuse command execution. Requests after the accountant has stopped
are refused and counted; queue saturation no longer discards records.

A configuration reload keeps each active command's original accountant and
server configuration alive until STOP. Only then can the retired bundle close.
Task identifiers remain unique across those overlapping generations.

Use `ze show aaa accounting` to inspect the counter:

```
ze show aaa accounting
```

The response includes `dropped-records`, the count of records refused after
shutdown. Delivery failures at the network or server are logged separately.

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

<!-- source: internal/test/mock/tacacs/tacacs.go -- le test tacacs-mock for .ci tests -->

## Operational notes

- **Shared secrets** are sensitive data. `$9$` values are obfuscated, not
  encrypted; a committed configuration can contain the original text.
  `ze config dump --strip-private` replaces secrets with `/* SECRET-DATA */`.
- **VRF**: when the SSH server runs in a non-default VRF, TACACS+ TCP
  connections inherit the same VRF context.
- **Single-connect mode** (RFC 8907 section 4.3) is tested via `tacacs-singleconnect.ci`.
- **Operational tooling**: `ze tacacs show <config>` probes every TACACS+
  server the config names and reports whether it answers, with no daemon
  running. Runtime `ze show aaa accounting` exposes local accounting queue
  drops.
- **Transport security**: RFC 8907 section 10.5 requires privacy, integrity and
  separation from other traffic. The MD5 pseudo-pad does not provide these.
  A successful reachability probe does not establish a secure deployment.
- **Authentication methods**: the supported method is PAP. ASCII interaction,
  CHAP, MS-CHAP and ENABLE workflows are outside the selected product scope.

## RFC reference

- RFC 8907 -- The TACACS+ Protocol (formalises the original Cisco draft).
  Local summary: `rfc/short/rfc8907.md`.
