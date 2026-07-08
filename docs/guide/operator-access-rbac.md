# Operator access with SSH and RBAC

Use this when the box already has Ze installed and `database.zefs` created, and you want real operator accounts instead of a single bootstrap admin.

The example keeps `admin` as a recovery account, adds a read-only NOC user, adds an operator user, enables SSH on TCP/2222, and applies profile-based command authorization.

<!-- source: internal/component/ssh/yang/ze-ssh-conf.yang -- system.authentication.user and environment.ssh -->
<!-- source: internal/component/authz/yang/ze-authz-conf.yang -- system.authorization.profile -->
<!-- source: internal/component/authz/authz.go -- built-in profile behavior and fail-closed assignments -->
<!-- source: internal/component/config/password_hash.go -- password hashing on commit -->
<!-- source: docs/guide/tacacs.md -- TACACS+ fallback and profile mapping -->

## 1. Start from the Ubuntu install page

Follow [Build and install Ze on Ubuntu](ubuntu-build-install.md) through `database.zefs` creation. The commands below assume:

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

/usr/local/bin/ze config migrate --format hierarchical -o "$CONFIG_IMPORT" "$CONFIG_SET"
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

Test that NOC cannot enter the edit path:

```bash
ZE_SSH_PASSWORD='CHANGE_ME_NOC' \
  /usr/local/bin/ze cli --user noc -c "configure"
```

The command should be rejected by authorization. Check the daemon logs if it succeeds.

```bash
journalctl -u ze.service -n 100 --no-pager
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

/usr/local/bin/ze config migrate --format hierarchical -o "$CONFIG_IMPORT" "$CONFIG_SET"
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
