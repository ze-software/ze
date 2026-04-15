# Authentication

Ze supports multiple SSH login users defined in the daemon's configuration,
in addition to the bootstrap super-admin written to `database.zefs` by
`ze init`. This guide covers adding YANG-configured users, hashing their
passwords, and connecting as them with the `ze` CLI.

## Two sources of users

| Source | Where stored | Created by | Used for |
|--------|-------------|-----------|----------|
| zefs super-admin | `database.zefs` (`meta/ssh/{username,password}`) | `ze init` | Bootstrap and recovery -- the operator who set the box up |
| YANG users | `system.authentication.user <name>` | Config edit | Day-to-day operators, auditors, scripts |

The daemon merges both sources at config load: any login attempt is checked
against the combined list. The super-admin always works; YANG users work
only when the config is loaded.

<!-- source: cmd/ze/hub/infra_setup.go -- infraSetup user merge -->

## Adding a user

### Step 1: hash a password

`ze passwd` takes plaintext on stdin or via interactive prompt and prints
a bcrypt hash to stdout. The hash uses cost 10 (the same as `ze init`).

```
$ echo "secret" | ze passwd
$2a$10$abcdefghijklmnopqrstuABCDEFGHIJKLMNOPQRSTUVWXYZ012345678
```

Interactive use prompts twice for confirmation.

<!-- source: cmd/ze/passwd/main.go -- runImpl -->

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

After `commit` (or `ze config set --no-reload`), the persisted file contains
only the bcrypt hash; the `plaintext-password` leaf is removed and never
written to disk. This matches Junos's `plain-text-password` behaviour.

<!-- source: internal/component/ssh/schema/ze-ssh-conf.yang -- system.authentication.user -->
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

<!-- source: cmd/ze/internal/ssh/client/client.go -- ReadCredentialsWithFlags -->

The same flag works on `ze bgp plugin cli`, `ze signal`, `ze config set`,
`ze config edit`, and `ze interface migrate`.

### Tab completion (`ze completion`)

Tab completion runs silently in the shell and does not accept flags. To
have completions resolve as a non-super-admin user, set the env var in
your shell profile:

```
export ZE_SSH_USERNAME=alice
export ZE_SSH_PASSWORD=...   # or use a key-locked secret store
```

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
| YANG schema | `internal/component/ssh/schema/ze-ssh-conf.yang` |
| Commit-time hashing helper | `internal/component/config/password_hash.go` |
| Validator | `internal/component/cli/validator.go` |
| SSH server password handler | `internal/component/ssh/ssh.go` |
| Local authenticator | `internal/component/authz/auth.go` |
| Client credential resolver | `cmd/ze/internal/ssh/client/client.go` |
| `ze passwd` | `cmd/ze/passwd/main.go` |
