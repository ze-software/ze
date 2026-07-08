# Operator access with SSH and RBAC

Use this when the box already has Ze installed and `database.zefs` created, and you want real operator accounts instead of a single bootstrap admin.

The example keeps `admin` as a recovery account, adds a read-only NOC user, adds an operator user, enables SSH on TCP/2222, and applies profile-based command authorization.

<!-- source: internal/component/ssh/yang/ze-ssh-conf.yang -- system.authentication.user and environment.ssh -->
<!-- source: internal/component/authz/yang/ze-authz-conf.yang -- system.authorization.profile -->
<!-- source: internal/component/authz/authz.go -- built-in profile behavior and fail-closed assignments -->
<!-- source: internal/component/config/password_hash.go -- password hashing on commit -->
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

## 3. Hash passwords

Use `ze passwd` before writing a static config file. This keeps plaintext out of `/etc/ze/edge-01.conf`.

```bash
ADMIN_HASH="$(printf '%s\n' 'CHANGE_ME_BOOTSTRAP' | /usr/local/bin/ze passwd)"
NOC_HASH="$(printf '%s\n' 'CHANGE_ME_NOC' | /usr/local/bin/ze passwd)"
OPERATOR_HASH="$(printf '%s\n' 'CHANGE_ME_OPERATOR' | /usr/local/bin/ze passwd)"
```

If you use the interactive config editor later, `plaintext-password "secret"` is safe there because the commit hook stores only the bcrypt `password` leaf. For a file you import directly, use the hash form shown here.

## 4. Write the config

```bash
sudo tee /etc/ze/edge-01.conf >/dev/null <<EOF
environment {
    ssh {
        enabled true
        server main {
            ip 0.0.0.0
            port 2222
        }
        idle-timeout 600
        max-sessions 32
    }
}

system {
    authentication {
        user admin {
            password "$ADMIN_HASH"
            profile [ admin ]
        }
        user noc {
            password "$NOC_HASH"
            profile [ read-only ]
        }
        user operator {
            password "$OPERATOR_HASH"
            profile [ operator ]
        }
    }
    authorization {
        profile admin {
            run {
                default-action allow
            }
            edit {
                default-action allow
            }
        }
        profile read-only {
            run {
                default-action allow
                entry 10 {
                    action deny
                    match "debug"
                }
                entry 20 {
                    action deny
                    match "clear"
                }
            }
            edit {
                default-action deny
            }
        }
        profile operator {
            run {
                default-action allow
            }
            edit {
                default-action deny
                entry 10 {
                    action allow
                    match "configure"
                }
                entry 20 {
                    action allow
                    match "commit"
                }
            }
        }
    }
}
EOF
sudo chmod 0600 /etc/ze/edge-01.conf
```

Keep the explicit `admin` stanza. Once `system.authorization.profile` exists and users have assignments, RBAC denies unassigned users. If you leave the zefs bootstrap admin outside the config, it can authenticate but fail authorization.

## 5. Validate, import, and reload

```bash
sudo /usr/local/bin/ze config validate /etc/ze/edge-01.conf
sudo /usr/local/bin/ze config import --name edge-01.conf /etc/ze/edge-01.conf
sudo systemctl reload ze.service
```

If Ze is not managed by systemd yet, start it after importing:

```bash
sudo /usr/local/bin/ze start
```

## 6. Test each account

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

## 7. Add SSH public keys

Each public key has a local name, a key type, and the base64 body of the SSH public key. The key value is the middle field of a line from `~/.ssh/id_ed25519.pub`.

```bash
ALICE_KEY="$(awk '{print $2}' /home/alice/.ssh/id_ed25519.pub)"

sudo tee -a /etc/ze/edge-01.conf >/dev/null <<EOF

# Add inside: system authentication user operator
# public-keys alice-laptop {
#     type ssh-ed25519
#     key "$ALICE_KEY"
# }
EOF
```

For a real edit, put the block inside the `user operator` stanza:

```text
public-keys alice-laptop {
    type ssh-ed25519
    key "AAAAC3NzaC1lZDI1NTE5AAAA..."
}
```

Then validate, import, and reload again.

## 8. Add TACACS+ later

Local users stay useful as break-glass accounts. TACACS+ can be added under `system.authentication.tacacs` and mapped back to the same local authorization profiles.

```text
system {
    authentication {
        tacacs {
            server 10.0.0.1 {
                port 49
                key "$9$encrypted-key"
            }
            timeout 5
            authorization true
            accounting true
        }
        tacacs-profile 15 {
            profile [ admin ]
        }
        tacacs-profile 1 {
            profile [ read-only ]
        }
    }
}
```

TACACS+ authentication is tried before local bcrypt. A server rejection stops the chain. A server outage falls back to local users unless `strict-fallback true` is set.

## 9. Operational checks

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze show audit
/usr/local/bin/ze show aaa accounting
/usr/local/bin/ze show warnings
```

Use `admin` to recover from a broken profile, then correct the config, validate it, import it, and reload.
