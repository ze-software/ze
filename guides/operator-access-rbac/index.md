# Operator access with SSH and RBAC

Use this when the box already has Ze installed and `database.zefs` created, and you want real operator accounts instead of a single bootstrap admin.

The example keeps `admin` as a recovery account, adds a read-only NOC user, adds an operator user, enables SSH on TCP/2222, and applies profile-based command authorization.

Ze SSH is an application management plane. The daemon terminates SSH itself and
starts a Ze command session, so a successful login grants access to Ze commands
and the config editor rather than to a Unix shell. Local users, TACACS+, and
RADIUS all map back to Ze authorization profiles, audit, and accounting.

<!-- source: internal/component/authz/yang/ze-authz-conf.yang -- system.authentication.user base fields and system.authorization.profile -->
<!-- source: internal/component/ssh/yang/ze-ssh-conf.yang -- environment.ssh and public-keys augmentation -->
<!-- source: internal/component/authz/authz.go -- built-in profile behavior and fail-closed assignments -->
<!-- source: internal/component/config/password_hash.go -- ApplyPasswordHashing, at commit and at config load -->
<!-- source: docs/guide/tacacs.md -- TACACS+ fallback and profile mapping -->

## 1. Start from the Ubuntu install page

Follow [Build and install Ze on Ubuntu](../ubuntu-build-install/index.md) through `database.zefs` creation. The commands below assume:

| Item | Value |
| --- | --- |
| Ze binary | `/usr/local/bin/ze` |
| Config directory | `/etc/ze` |
| Instance name | `edge-01` |
| Active config | `edge-01.conf` |
| Runtime socket | `/run/ze/ze.socket` |

If your instance name is different, change every `edge-01.conf` command to match it.

## 2. Decide the profiles

This page uses three local profiles.

| Profile | Run commands | Edit commands | Intended user |
| --- | --- | --- | --- |
| `admin` | allow all | allow all | recovery and senior operators |
| `read-only` | allow normal `show`, `monitor`, `resolve`, and help commands | deny all | NOC screens and audit users |
| `operator` | allow run commands | allow config workflow only | operators allowed to change config through the editor |

The entries use prefix matching. `match "debug"` matches `debug ospf ...`, but not `show debug ...`.

## 3. Build and load a set-format update

Use `ze passwd` before writing stored config values. This keeps plaintext out of shell history, process arguments, and the zefs command history.

The commands below read the current active config from zefs, convert it to set format, append the SSH and RBAC changes, render the import file, validate the result, then load it back into zefs with one `ze config import` command. Unrelated existing config sections stay in the candidate because they came from `ze config cat`.

```bash
set -euo pipefail

ADMIN_HASH="$(printf '%s\n' 'CHANGE_ME_BOOTSTRAP' | /usr/local/bin/ze passwd)"
NOC_HASH="$(printf '%s\n' 'CHANGE_ME_NOC' | /usr/local/bin/ze passwd)"
OPERATOR_HASH="$(printf '%s\n' 'CHANGE_ME_OPERATOR' | /usr/local/bin/ze passwd)"

umask 077
CONFIG_SET="$(mktemp)"
CONFIG_IMPORT="$(mktemp)"
trap 'rm -f "$CONFIG_SET" "$CONFIG_IMPORT"' EXIT

sudo /usr/local/bin/ze config cat edge-01.conf | /usr/local/bin/ze config migrate -o "$CONFIG_SET" -

cat >>"$CONFIG_SET" <<EOF
set environment ssh enabled enable
set environment ssh server main ip 0.0.0.0
set environment ssh server main port 2222
set environment ssh idle-timeout 600
set environment ssh max-sessions 32

set system authentication user admin password "$ADMIN_HASH"
set system authentication user admin profile admin
set system authentication user noc password "$NOC_HASH"
set system authentication user noc profile read-only
set system authentication user operator password "$OPERATOR_HASH"
set system authentication user operator profile operator

set system authorization profile admin run default-action allow
set system authorization profile admin edit default-action allow

set system authorization profile read-only run default-action allow
set system authorization profile read-only run entry 10 action deny
set system authorization profile read-only run entry 10 match debug
set system authorization profile read-only run entry 20 action deny
set system authorization profile read-only run entry 20 match clear
set system authorization profile read-only edit default-action deny

set system authorization profile operator run default-action allow
set system authorization profile operator edit default-action deny
set system authorization profile operator edit entry 10 action allow
set system authorization profile operator edit entry 10 match configure
set system authorization profile operator edit entry 20 action allow
set system authorization profile operator edit entry 20 match commit
EOF

/usr/local/bin/ze config migrate -o "$CONFIG_IMPORT" format hierarchical "$CONFIG_SET"
/usr/local/bin/ze config validate "$CONFIG_IMPORT"
sudo /usr/local/bin/ze config import --name edge-01.conf "$CONFIG_IMPORT"
sudo systemctl reload ze.service
```

If Ze is not managed by systemd yet, start it after importing:

```bash
sudo /usr/local/bin/ze start
```

## 4. Test each account

```bash
export XDG_RUNTIME_DIR=/run/ze

ZE_SSH_PASSWORD='CHANGE_ME_BOOTSTRAP' \
  /usr/local/bin/ze cli --user admin -c "help"

ZE_SSH_PASSWORD='CHANGE_ME_NOC' \
  /usr/local/bin/ze cli --user noc -c "help"

ZE_SSH_PASSWORD='CHANGE_ME_OPERATOR' \
  /usr/local/bin/ze cli --user operator -c "help"
```

Test that NOC is refused a command its profile denies. The `read-only` profile above denies the `clear` prefix:

```bash
ZE_SSH_PASSWORD='CHANGE_ME_NOC' \
  /usr/local/bin/ze cli --user noc -c "clear interface counters"
```

Authorization refuses it, and says so:

```
error: command restricted by access control
```

Check the daemon logs if it succeeds instead:

```bash
journalctl -u ze.service -n 100 --no-pager
```

Pick a real command the profile denies when you write your own check. A command Ze does not have reports `unknown command` and exits non-zero for everyone, authorized or not, so testing with one proves nothing about your profiles: the check would pass even with authorization disabled. `configure` is one of these. It is a mode switch inside the interactive CLI, not a daemon command, so `ze cli -c "configure"` is never an authorization test. To check the edit path, run a command that writes, such as `set system ...`, or log in and try `configure` interactively.

### How authorization decides

Once you define any `system.authorization` profile, authorization is in use and it fails closed:

- A user who authenticates but resolves **no applicable profile** is **denied every command**, not granted access. Assign every account a profile. A profile reaches an account either through `system.authentication.user <name> profile ...` (local users) or through the TACACS+/RADIUS priv-level mapping (remote users).
- A box that defines **no** `system.authorization` profile at all stays fully permissive: with authorization unconfigured there is nothing to enforce.

The daemon log states which rule decided, so you can tell "denied by profile" from "denied because no profile applied":

```bash
journalctl -u ze.service -n 100 --no-pager | grep -i authorize
```

### The recovery account

The `admin` account created by `ze init` is the break-glass identity. It always keeps a path to the box, **even on a box that has authorization profiles configured but no `admin` user of its own** (for example one where the profiles were staged before the operator accounts, or a TACACS+/RADIUS-only box). Two independent recovery paths exist:

- The `ze init` bootstrap admin carries an internal, reserved recovery profile, so it is never locked out by a strict authorization default.
- Defining `set system authentication user admin profile admin` (as this guide does) makes `admin` a normal config account with an explicit profile.

If you never want the bootstrap admin to authenticate, disable it explicitly with `meta/instance/admin-disabled`; accept that you then depend entirely on your configured accounts for access.

### Demo: Prove read-only RBAC enforcement

Run an allowed NOC command, then show Ze explicitly refuse a known state-changing command.

[Download the asciicast recording](../../assets/demos/rbac.cast?v=5f85fa730f) · [Plain-text transcript](../../assets/demos/rbac.txt?v=939addc51a)

Recorded with Ze 26.08.31 on macOS and Linux using Ze recorder. Duration: 1 minute 11 seconds.

```console
$ ze config show rbac.conf system authorization profile read-only
default-action allow
entry 20 {
   action deny
   match clear
}

$ ze config show rbac.conf system authentication user noc profile
profile read-only

$ ze cli --user noc -c 'show version'
version: ze 26.07.18

$ ze cli --user noc -c 'clear interface counters'
error: command restricted by access control

The recording displays the command restriction and the NOC user's profile binding before exercising both paths. The daemon allows `show version`, then rejects the matching `clear` command before execution.
```


## 5. Add SSH public keys

Each public key has a local name, a key type, and the base64 body of the SSH public key. The key value is the middle field of a line from `~/.ssh/id_ed25519.pub`.

```bash
set -euo pipefail

ALICE_KEY="$(cut -d' ' -f2 /home/alice/.ssh/id_ed25519.pub)"

umask 077
CONFIG_SET="$(mktemp)"
CONFIG_IMPORT="$(mktemp)"
trap 'rm -f "$CONFIG_SET" "$CONFIG_IMPORT"' EXIT

sudo /usr/local/bin/ze config cat edge-01.conf | /usr/local/bin/ze config migrate -o "$CONFIG_SET" -

cat >>"$CONFIG_SET" <<EOF
set system authentication user operator public-keys alice-laptop type ssh-ed25519
set system authentication user operator public-keys alice-laptop key "$ALICE_KEY"
EOF

/usr/local/bin/ze config migrate -o "$CONFIG_IMPORT" format hierarchical "$CONFIG_SET"
/usr/local/bin/ze config validate "$CONFIG_IMPORT"
sudo /usr/local/bin/ze config import --name edge-01.conf "$CONFIG_IMPORT"
sudo systemctl reload ze.service
```

## 6. Add TACACS+ later

Local users stay useful as break-glass accounts. TACACS+ can be added under `system.authentication.tacacs` and mapped back to the same local authorization profiles. Use the same zefs update pattern as above and append these set-format lines:

```text
set system authentication tacacs server 10.0.0.1 port 49
set system authentication tacacs server 10.0.0.1 key SHARED_SECRET
set system authentication tacacs timeout 5
set system authentication tacacs authorization enable
set system authentication tacacs accounting enable
set system authentication tacacs-profile 15 profile admin
set system authentication tacacs-profile 1 profile read-only
```

TACACS+ authentication is tried before local bcrypt. A server rejection stops the chain. A server outage falls back to local authentication, so break-glass local users can still log in. `strict-fallback true` does not block that login; it makes command *authorization* fail closed (deny) during a TACACS outage instead of falling back to the local RBAC profiles.

## 7. Operational checks

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze show audit
/usr/local/bin/ze show aaa accounting
/usr/local/bin/ze show warnings
```

Use `admin` to recover from a broken profile, then correct the config in zefs, validate it, and reload.

## 8. Remove an operator

Delete the user from `system.authentication.user` and reload. The removal takes
effect at once on every credential surface: the web password, the web session
cookie the browser still holds, the SSH password, the SSH public key, and
`Bearer <user>:<pass>` over REST and gRPC. The daemon does not restart, and the
24-hour web session TTL is a ceiling rather than the only test.

Removal governs NEW connections. A session that is already open outlives it until
the connection closes, because an operator may be editing their own account. A
session a TACACS+ or RADIUS backend granted is not revoked by the local user
list, which never authenticated it.
<!-- source: cmd/ze/hub/aaa_lifecycle.go -- liveAcceptedLocalUsers -->
<!-- source: internal/component/web/auth.go -- SessionStore.validateToken, webSession -->
<!-- source: internal/component/ssh/pubkey.go -- authenticatePublicKeyResult -->
