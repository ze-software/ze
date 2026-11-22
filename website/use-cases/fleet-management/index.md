# Fleet configuration management

Manage a fleet of Ze routers from a central Ze hub over authenticated TLS connections: enroll devices against the hub's CA, push live configuration edits with zero daemon restart, prevent accidental out-of-band drift while managed, and resolve emergency local edits through audited adopt or revert workflows.

## Role and topology

```text
                    Central Ze Hub (10.0.0.1:1791, TLS)
                    stores client-edge-01.conf, client-edge-02.conf
                          /                              \
       TLS + per-client secret                    TLS + per-client secret
                        /                                  \
                 edge-01 (managed)                   edge-02 (managed)
                 local edits frozen                  local edits frozen
                 boots from cache if hub down        boots from cache if hub down
```

## Hub configuration

The hub exposes a dedicated `server` listener for managed clients on a separate port from its local plugin listener. Every secret must be at least 32 characters (based on `docs/guide/fleet-config.md`):

```text
plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-plugin-secret-of-32-characters";
        }
        server central {
            ip 0.0.0.0;
            port 1791;
            secret "remote-plugin-secret-of-32-character";
            client edge-01 { secret "edge-01-secret-of-at-least-32-chars"; }
            client edge-02 { secret "edge-02-secret-of-at-least-32-chars"; }
        }
    }
}
```

## Managed client configuration (stored on the hub)

Save each device's full configuration on the hub as `client-<name>.conf` (for example `client-edge-01.conf`). Keep the `client edge-01` block inside the pushed config so the device stays bound to the hub after applying updates:

```text
plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
            secret "local-secret-of-at-least-32-characters";
        }
        client edge-01 {
            host 10.0.0.1;
            port 1791;
            secret "edge-01-secret-of-at-least-32-chars";
        }
    }
}

bgp {
    ...
}
```

## Enrollment and live rollout workflow

1. Export the hub's local CA certificate and import the client configuration on the running hub:

```bash
ze show pki local-ca pem > hub-root.pem
ze data import client-edge-01.conf
```

2. Provision and start the managed device (or pass `--hub-trust-first-use` for initial lab setup):

```bash
ze init --managed --hub-ca hub-root.pem
ze start
```

3. Edit and push live configuration changes from the hub without restarting the remote daemon:

```bash
ze config edit client-edge-01.conf
# or re-import an updated file:
ze data import client-edge-01.conf
```

Committing on the hub sends `config-changed` over TLS to `edge-01`, which validates and applies the new running configuration immediately. If the hub is unreachable at boot, the managed device boots from its last cached configuration.

## Emergency local edits and divergence resolution

While a device is managed, local commits, imports, restores, and rollbacks are frozen and refused. During an incident where the hub cannot be reached, an operator can unfreeze the device locally, make an emergency change, and reconnect later:

```bash
# On edge-01: leave managed mode to unfreeze local edits, then rejoin when ready
ze request fleet disable
# ... emergency local edit ...
ze request fleet enable
```

When `edge-01` reconnects with a different active hash, the hub holds automatic pushes and records the divergence for explicit resolution:

```bash
# On the hub: inspect the masked diff and device/hub versions
ze show fleet divergence list
ze show fleet divergence name edge-01

# Either adopt the device's emergency edit into the hub's authoritative copy:
ze update fleet divergence name edge-01 adopt version <device-version>

# Or revert the device back to the hub's authoritative copy:
ze request fleet divergence name edge-01 revert version <device-version> hub-version <hub-version>
```

Both `adopt` and `revert` are hash-pinned against concurrent edits and recorded in `show audit`.

## Verification commands

```text
ze show fleet status
ze show fleet divergence list
ze show fleet divergence name edge-01
ze config history client-edge-01.conf
ze doctor
```

## Lab evidence

- **Managed fleet functional suite (`test/managed/`, 32 `.ci` tests):** `./le test functional managed`
- Key scenarios include `client-first-boot.ci`, `client-cached-boot-hub-down.ci`, `fleet-config-change-live.ci`, `fleet-freeze-refuses-commit.ci`, `fleet-divergence-adopt.ci`, `fleet-divergence-revert.ci`, `fleet-status.ci`, and `data-restore-client-live.ci`.

## Current limits

- **Full per-device files (no templates or canary waves):** Each managed client has its own complete `client-<name>.conf`; Ze does not render config templates or run staged canary rollouts across groups.
- **1 MiB divergence upload cap:** Configurations larger than 1 MiB are not uploaded automatically on divergence and must be reconciled manually via `ze data cat` and `ze data import`.
- **Running hub required for adopt/revert:** Divergence resolution commands require a running hub so audit records are always written.
- **Keep the client block in pushed configs:** Pushing a configuration that omits `plugin { hub { client <name> { ... } } }` disconnects the device from future hub updates.

## Related guides

- [Fleet management](../../features/fleet-management/)
- [Fleet configuration guide](../../guides/fleet-config/)
- [Audit log guide](../../guides/audit/)
