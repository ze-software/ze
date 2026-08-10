# Authentication

Ze supports multiple SSH login users defined in the daemon's configuration,
in addition to the bootstrap super-admin written to `database.zefs` by
`ze init`. This guide covers adding YANG-configured users, hashing their
passwords, and connecting as them with the `ze` CLI.

## Two sources of users

| Source | Where stored | Created by | Used for |
|--------|-------------|-----------|----------|
| zefs super-admin | `database.zefs` (`meta/auth/local/{username,password}`) | `ze init` | Bootstrap and recovery -- the operator who set the box up |
| YANG users | `system.authentication.user <name>` | Config edit | Day-to-day operators, auditors, scripts |

The daemon merges both sources at config load: any login attempt is checked
against the combined list. YANG users work only when the config is loaded.

When a YANG user has the same name as the zefs super-admin, the YANG entry
takes precedence and the zefs entry is dropped. This lets operators override
the bootstrap password via configuration without a stale zefs hash remaining
as a backdoor.

### Disabling the super-admin

Setting `meta/instance/admin-disabled` to `"true"` in the zefs database
(via `admin-enabled: false` in the appliance config) disables the super-admin
on all surfaces: SSH, web, API, and serial console. The serial console
returns "local admin login disabled" and denies access (fail-closed), unlike
the missing-database case which grants access for emergency recovery.

<!-- source: internal/plugins/init/main.go -- RunWithReader entries -->
<!-- source: cmd/ze/hub/main_servers.go -- usersFromZefsDB -->

## Adding a user

### Step 1: hash a password

`ze passwd` takes plaintext on stdin or via interactive prompt and prints
a bcrypt hash to stdout. The hash uses cost 10 (the same as `ze init`).

```
$ echo "secret" | ze passwd
$2a$10$abcdefghijklmnopqrstuABCDEFGHIJKLMNOPQRSTUVWXYZ012345678
```

Interactive use prompts twice for confirmation.

<!-- source: internal/plugins/passwd/main.go -- runImpl -->

### Step 2: declare the user in YANG

Two equivalent ways to set the password:

| Form | When to use |
|------|------------|
| `password "$2a$10$..."` | You already have a hash (from `ze passwd`, fleet automation, or a backup) |
| `plaintext-password "secret"` | You want to type the plaintext and let the commit hook hash it (Junos style) |

Example using the plaintext form:

```
system {
    authentication {
        user alice {
            plaintext-password "secret"
        }
    }
}
```

After `commit` (or `ze config set`), the persisted file contains
only the bcrypt hash; the `plaintext-password` leaf is removed and never
written to disk. This matches Junos's `plain-text-password` behaviour.

<!-- source: internal/component/ssh/yang/ze-ssh-conf.yang -- system.authentication.user -->
<!-- source: internal/component/config/password_hash.go -- ApplyPasswordHashing -->

### Step 3: reload

The daemon picks up the new user on the next config reload. Existing
sessions are not interrupted.

## Logging in as a YANG user

Any `ze` CLI tool accepts a `--user`/`-u` flag to override the zefs
super-admin username. The password is read from `ze.ssh.password` (env
var) or, if stdin is a terminal, prompted interactively.

```
# As super-admin (default)
ze cli

# As alice, password from env (CI / scripts)
ZE_SSH_PASSWORD=secret ze cli --user alice

# As alice, password prompted (interactive)
ze cli -u alice

# Single command, then exit
ze cli -u alice -c "show version"
```

<!-- source: internal/core/ssh/client/client.go -- ReadCredentialsWithFlags -->

The same flag works on `ze bgp plugin cli`, `ze signal`, `ze config set`,
`ze config edit`, and `ze interface migrate`.

### The stored hash is a credential only over a local connection

The zefs-stored super-admin bcrypt hash may be presented directly as the SSH
password (the on-box `ze` CLI does this), but **only over a local transport**: a
loopback TCP peer (`127.0.0.1`/`::1`, the default `ze` dial) or a unix-socket
peer. Over any remote transport (non-loopback SSH, all web logins, the REST/gRPC
bearer path) the hash is **rejected** as a credential, so a leaked config backup
or zefs copy cannot be replayed as a password from another machine. Remote
logins must supply the real plaintext password (via `ze.ssh.password` or an
interactive prompt); plaintext works from anywhere.

<!-- source: internal/component/ssh/passwordauth.go -- isLocalTransport, authenticatePassword -->
<!-- source: internal/component/authz/auth.go -- CheckPassword allowHashToken gate -->

The stored bcrypt hash is also **masked** in every config display: `show config`,
the web config views, `ze config dump`, and CLI search show `ze:bcrypt` leaves as
`/* SECRET-DATA */`, never the hash and never `$9$`-encoded. The edit-authorized
raw download (`GET /config/download`, gated behind edit permission) keeps the real
hash so a download → edit → upload round-trip is byte-exact; a masked value pasted
back into a commit or upload is rejected with a clear error.

<!-- source: internal/component/config/mask.go -- MaskBcrypt, RejectMaskedBcryptLeaves -->
<!-- source: cmd/ze/hub/service_web.go -- GET /config/download editWrap gate -->

### Remote management listener guard

Ze refuses to start an unauthenticated management service on a non-loopback
listener. The guard runs before any covered listener binds:

| Service | Authentication that permits a remote listener |
|---------|------------------------------------------------|
| Web in insecure mode | Disable insecure mode and configure users |
| MCP | Configure a bearer token or another authenticated `auth-mode` |
| gNMI | Configure `ze.gnmi.token` or `environment.gnmi token` |
| REST and gRPC API | Configure an API token or initialize zefs users |

Literal addresses in `127.0.0.0/8` and `::1` count as loopback. Wildcard
addresses (`0.0.0.0`, `::`, or `:port`), empty addresses, DNS names, and even
the name `localhost` are treated as non-loopback because the guard fails closed
when it cannot prove an address is local.

An unsafe web, MCP, REST, or gRPC listener migration on reload is rejected
before any listener changes; the service keeps its previous addresses. The
public Looking Glass is a separate, intentionally unauthenticated read-only
service. Its deployment guidance is in [Public looking glass](../looking-glass-howto/index.md);
it serves TLS by default and takes an optional bearer token.

`ze doctor --json` and `ze config validate` report the same exposure offline,
so you find it before you deploy. A gNMI listener that is non-loopback with no
token gives `config-gnmi-invalid`, and an MCP block that asks for a remote bind
with no authentication gives `config-mcp-invalid`. Neither command can read
another process's environment, so a listener published only by an environment
variable is caught at startup rather than by these checks.

<!-- source: internal/component/config/validate_semantic.go -- ValidateSemantics gNMI and MCP entries -->
<!-- source: internal/component/config/loader_extract.go -- GNMIListenConfig.Validate -->

<!-- source: cmd/ze/hub/mgmt_guard.go -- checkMgmtListeners, listenAddrIsNonLoopback -->
<!-- source: cmd/ze/hub/main.go -- management listener declarations and remedies -->
<!-- source: cmd/ze/hub/listener_migrate.go -- reload refusal -->

#### Upgrading from a release without the guard

The guard is a boot refusal, so a config that started a daemon yesterday can
stop one today. Two changes break an upgrade. Read them before you upgrade.

**A remote unauthenticated management listener now refuses to boot.** ze exits
with status 1 and prints one line for each offending listener. The line names
the service, the address, and the remedy. Nothing binds, so the daemon does not
half-start. The message looks like this:

```
error: refusing to start gNMI on non-loopback listener "0.0.0.0:9339" without authentication
  set ze.gnmi.token (or environment.gnmi token), or bind to 127.0.0.1/::1 only
```

Apply the remedy from the table above, or bind the service to `127.0.0.1`.

**The looking glass serves TLS by default.** A `looking-glass` block with no
`tls` leaf now serves `https://`, so a plaintext client fails the handshake.
Write `tls false` in the block, or set `ze.looking-glass.tls=false`, to keep
plaintext. One case still serves plaintext on its own: a box with no blob
storage that only inherited the default gets a warning naming `ze init` instead
of a failure, because a hardening default must not remove a working looking
glass. An explicit `tls true` on such a box is an error.

Run `ze config validate` or `ze doctor --json` over the config first. Both
report the same exposure offline. Neither reads the daemon's environment, so
check `ze.gnmi.listen`, `ze.mcp.listen`, and `ze.web.insecure` by hand.

<!-- source: cmd/ze/hub/mgmt_guard.go -- checkMgmtListeners refusal message -->
<!-- source: cmd/ze/hub/service_lg.go -- buildLGService TLS default, explicit-vs-inherited fallback -->

### Authentication on reload

A config reload rebuilds the credentials of the running REST and gRPC servers.
Add a token, rotate it, remove it, or edit the user list, and the change takes
effect on the next reload. The daemon does not restart and the listeners do not
rebind, so open connections keep working.

The web and MCP servers choose their authentication once, when they are built.
A reload that asks either of them for a different mode fails the whole commit:

```
<service> cannot change its authentication while running: it is serving
<mode> and the config asks for <mode>; restart ze to apply it
```

The reload is refused before anything is applied. No listener moves and no
credential changes. Restart ze to apply that edit.

A transport the config does not enable is never built, and a server that does
not run cannot refuse a reload. An `api-server` block that enables REST alone
reloads on the REST server, and says nothing about gRPC.

The listener guard runs again over the pair each service holds once the reload
applies, and it reads the rebuilt authentication rather than the mode the server
started with. A reload that removes the API credentials and moves the same
listener off loopback is refused for that reason. The refusal restores the
credentials the reload had already installed, so the daemon keeps serving the
config it rolled back to.

<!-- source: cmd/ze/hub/mgmt_auth_reload.go -- markMgmtAuth, registerMgmtAuthReloaders -->
<!-- source: cmd/ze/hub/listener_migrate.go -- checkAuthRebuildable, applyAuthIntents, checkReloadExposure -->
<!-- source: internal/component/api/rest/auth.go -- UpdateAuth -->
<!-- source: internal/component/api/grpc/server.go -- UpdateAuth -->

### Tab completion (`ze completion`)

Tab completion runs silently in the shell and does not accept flags. To
have completions resolve as a non-super-admin user, set the env vars in
your shell profile:

```
export ZE_SSH_USERNAME=alice
export ZE_SSH_PASSWORD=...
```

Set **both**, or neither. Completion never prompts for a password: it runs while
you are typing, so a prompt would block the shell instead of asking a question
you could answer. With a username but no password there is no usable credential,
so completion stays silent and you simply get no peer completions. Everything
else keeps working; only dynamic peer names are missing.

If you would rather not keep a password in the environment, leave both unset and
completion resolves as the zefs super-admin, which needs no password.
<!-- source: internal/core/ssh/client/client.go -- LoadCredentialsNoPrompt -->

## SSH public key authentication

Users can authenticate to the SSH server with public keys instead of (or in
addition to) passwords. Each user can have multiple named keys.

### Adding a public key

Extract the base64 key data from an existing SSH public key file. Given a
key file like:

```
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGtK... alice@laptop
```

The three parts are: type, base64 data, comment. Configure the type and
base64 data in the user's `public-keys` block:

```
system {
    authentication {
        user alice {
            password "$2a$10$..."
            profile admin
            public-keys laptop {
                type ssh-ed25519
                key AAAAC3NzaC1lZDI1NTE5AAAAIGtK...
            }
            public-keys workstation {
                type ssh-rsa
                key AAAAB3NzaC1yc2EAAAADAQABAAAB...
            }
        }
    }
}
```

The key name (`laptop`, `workstation`) is an identifier only. It appears in
no protocol exchange but helps distinguish multiple keys for the same user.

### Supported key types

| Type | Algorithm |
|------|-----------|
| `ssh-ed25519` | Ed25519 (recommended) |
| `ssh-rsa` | RSA |
| `ecdsa-sha2-nistp256` | ECDSA P-256 |
| `ecdsa-sha2-nistp384` | ECDSA P-384 |
| `ecdsa-sha2-nistp521` | ECDSA P-521 |

### Connecting with a key

Standard SSH clients work directly:

```
ssh -i ~/.ssh/id_ed25519 -p 2222 alice@router
```

### Password and key coexistence

A user can have both a password and public keys. The SSH server tries public
key authentication first; if no key matches, it falls back to password
authentication. The web UI uses passwords only; public keys are
SSH-specific.

### Scope

Public key configuration applies to YANG-configured users only. The zefs
super-admin (created by `ze init`) authenticates with a password.

<!-- source: internal/component/ssh/pubkey.go -- matchPublicKey -->
<!-- source: internal/component/ssh/ssh.go -- wish.WithPublicKeyAuth -->

## Why two leaves instead of auto-detecting the format

The canonical `password` leaf is marked `ze:bcrypt` -- the parser stores
the value verbatim and never tries to apply the `$9$` reversible
obfuscation used for other sensitive fields. Bcrypt is one-way; mixing
it with `$9$` would be a footgun.

If you write a literal plaintext directly on `password`:

```
user alice {
    password "secret"     # WRONG -- not a bcrypt hash
}
```

then `ze config validate` emits a warning, the daemon logs a warning at
load, and the user cannot authenticate (bcrypt compare fails). Use
`plaintext-password` (auto-hashed) or `ze passwd` (manual) instead.

<!-- source: internal/component/config/password_hash.go -- CheckBcryptLeaves -->

## Notes on plaintext lifetime

While an interactive `ze config edit` session is open, the plaintext value
of `plaintext-password` is held in-memory by the editor and persisted to a
zefs draft blob (mode 0o600) by `SaveDraft`. The plaintext is converted to
the bcrypt hash and the ephemeral leaf is removed at commit; the draft is
deleted afterward. Plaintext never appears in the canonical config file
nor in commit metadata, but does briefly live in the local zefs database
during the editing session.

The bcrypt algorithm only considers the first 72 bytes of input. `ze passwd`
rejects oversize input outright with a clear error so the user does not get
a hash that validates only a prefix of their intended passphrase. The
commit hook accepts oversize input (preserving an existing config) but emits
a `slog.Warn` so the truncation surfaces in daemon logs.

## Things that do NOT work

| Attempt | Why it fails | Use this instead |
|---------|-------------|------------------|
| `ze cli --user alice` from a non-TTY script with no `ZE_SSH_PASSWORD` | No password source, error message names the env var | Set `ZE_SSH_PASSWORD` in CI |
| `--password` flag on `ze cli` | Not implemented -- passwords in argv leak to `ps` and shell history | Env var or interactive prompt |
| Reading the YANG bcrypt hash and passing it as `ZE_SSH_PASSWORD` | The daemon's `CheckPassword` does plaintext-bcrypt comparison and timing-safe equality with the SAME hash; it works for the super-admin only because zefs stores the same bytes the daemon stores | Use the plaintext password; the daemon hashes on receive |
| Forgetting to add the user, then trying to log in as them | Daemon authenticator has no entry, returns `SSH auth failure source=local` | Add user, reload config |

## Reference

| Symbol | Location |
|--------|----------|
| YANG schema | `internal/component/ssh/yang/ze-ssh-conf.yang` |
| Public key matching | `internal/component/ssh/pubkey.go` |
| Commit-time hashing helper | `internal/component/config/password_hash.go` |
| Validator | `internal/component/cli/validator.go` |
| SSH server password handler | `internal/component/ssh/ssh.go` |
| Local authenticator | `internal/component/authz/auth.go` |
| Client credential resolver | `internal/core/ssh/client/client.go` |
| `ze passwd` | `internal/plugins/passwd/main.go` |
