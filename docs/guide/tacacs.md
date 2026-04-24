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
| Authorization | Wired (no `.ci` test) | `authorization true` switches per-command authorization on; the bridge falls back to local profiles on TACACS+ ERROR. |

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
| `tacacs.server <ip>.key` | string (`ze:sensitive`) | required | Shared secret, stored as `$9$` ciphertext |
| `tacacs.timeout` | uint16 (1-300) | 5 | Per-server connection timeout in seconds |
| `tacacs.source-address` | ip-address | none | Local source IP for outbound TACACS+ TCP |
| `tacacs.authorization` | boolean | false | Enable per-command TACACS+ authorization |
| `tacacs.accounting` | boolean | false | Enable START/STOP accounting records |
| `tacacs-profile <N>.profile` | leaf-list | required | Maps priv-lvl `N` (0-15) to one or more local authz profiles |

<!-- source: internal/component/tacacs/schema/ze-tacacs-conf.yang -- system.authentication.tacacs -->

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

Levels not present in `tacacs-profile` reject the login. Look for
`TACACS+ unmapped privilege level` in the daemon log when extending the
upstream config.

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
queued after `Stop()` are dropped silently.

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
| `tacacs-show.ci` | `ze tacacs show` offline config display |

For ad-hoc verification, point the daemon at a real TACACS+ server and
run any command via `ze cli -c "summary"` -- the daemon log tags the
satisfying backend on every login, e.g.:

```
INFO SSH auth success subsystem=ssh username=alice remote=10.0.0.1:51408 source=tacacs
```

`source=tacacs` confirms the chain consulted TACACS+ and returned PASS.
`source=local` means TACACS+ was unreachable (or unconfigured) and the
local bcrypt user accepted the credentials.

<!-- source: cmd/ze-test/tacacs_mock.go -- ze-test tacacs-mock for .ci tests -->

## Operational notes

- **Shared secrets** are stored as `$9$`-encoded ciphertext, never as
  plaintext. The CLI never echoes them; `ze config dump --strip-secrets`
  replaces them with `/* SECRET-DATA */`.
- **VRF**: when the SSH server runs in a non-default VRF, TACACS+ TCP
  connections inherit the same VRF context.
- **Single-connect mode** (RFC 8907 §4.4) is tested via `tacacs-singleconnect.ci`.
- **Operational tooling**: `ze tacacs show <config>` displays the parsed
  TACACS+ configuration offline. Runtime `ze show tacacs` per-server
  reachability and counters are tracked in `plan/deferrals.md`.

## RFC reference

- RFC 8907 -- The TACACS+ Protocol (formalises the original Cisco draft).
  Local summary: `rfc/short/rfc8907.md`.
