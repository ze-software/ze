# Immutable appliance A/B lifecycle and safe rollback

Build a self-contained read-only Ze appliance image on gokrazy, deploy OS updates to the inactive A/B root slot with `--testboot` before committing permanently, protect live configuration changes with timed `commit confirmed` rollback windows, and back up both the live ZeFS store and encrypted bastion state.

## Role and topology

```text
Operator workstation (./ze-host)                Ze gokrazy appliance (10.12.104.10)
  ├── appliance.json + encrypted DR export        ├── Root slot A (active, read-only)
  ├── HTTPS OTA push (--testboot / permanent) ──► ├── Root slot B (inactive, next image)
  └── SSH config-push + commit confirmed      ──► └── /perm (persistent ZeFS store & certs)
```

## Appliance manifest (`appliance.json`)

Define credentials, management listeners, TLS identity, target device address, and image geometry in one manifest (based on `docs/guide/appliance.md`):

```json
{
    "credentials": { "username": "exa", "admin-enabled": true },
    "ssh":   { "host": "0.0.0.0", "port": "2222" },
    "web":   { "enabled": true, "host": "0.0.0.0", "port": "8080" },
    "tls":   { "cert-name": "router.local", "validity-years": 10 },
    "identity": { "hostname": "ze-prod" },
    "device": { "address": "10.12.104.10", "update-port": 443 },
    "image": { "arch": "amd64", "size-bytes": 2147483648, "kernel-profile": "hardware-kms" }
}
```

## Automatic config archiving on commit

Retain local commit history and ship snapshots to `/perm` or an HTTPS archive endpoint automatically on every commit (based on `docs/guide/config-archive.md`):

```text
system {
    host router1;
    domain dc1.example.com;
    commit-revisions 20;

    archive local-backup {
        location file:///perm/backups/ze;
        trigger commit;
    }

    archive offsite {
        location https://archive.example.com/configs;
        trigger daily;
        on-change true;
        timeout 10s;
        filename "{host}-{date}-{time}";
    }
}
```

*(For writable Linux/systemd hosts rather than read-only gokrazy appliances, Ze also supports `system { update-check { url ...; maintenance-window { ... }; } }` with `update system firmware check|download|apply|restart|rollback`; see the [self-update guide](../../guides/self-update/).)*

## Build, A/B testboot, and promote workflow

1. Build the host driver and appliance disk/ISO image from your workstation:

```bash
./le build host-driver
./ze-host appliance init ze-prod
./ze-host appliance build ze-prod
./ze-host appliance iso ze-prod
```

2. Take a live ZeFS backup on the appliance before upgrading:

```text
request data backup path /perm/ze/pre-upgrade.zefs
```

3. Push the new image to the inactive root slot as a one-shot testboot (a subsequent reboot falls back to the previous root slot automatically):

```bash
./ze-host appliance push --testboot ze-prod
```

4. Verify health and running version over SSH before making the new slot permanent:

```text
show version --extended
show system platform
show doctor
show config running
```

5. Promote the tested slot permanently (or push across a fleet with `--all --parallel 4`):

```bash
./ze-host appliance push ze-prod
```

## Safe live config changes (`commit confirmed`)

In the SSH or web configuration editor, bare `commit` is refused. Use `commit confirmed <seconds>` (1 to 3600 seconds) so a bad change that locks out management reverts automatically when the timer expires:

```text
commit verify
commit confirmed 120
# Verify connectivity and routing, then confirm permanently:
commit accept
# Or revert immediately:
commit abort
```

To restore a previous store snapshot on the running appliance:

```text
request data restore path /perm/ze/pre-upgrade.zefs config
# Or replace the full store (clean shutdown + restart):
request data restore path /perm/ze/pre-upgrade.zefs full
```

For workstation disaster recovery of the appliance credentials and manifest, export an Argon2id + XChaCha20-Poly1305 encrypted bundle:

```bash
./ze-host appliance export ze-prod
# Restore on another workstation:
./ze-host appliance import ze-prod.ze.enc
```

## Lab evidence

- **Real gokrazy A/B OTA in QEMU (image build, HTTPS push, reboot, version check, `/perm` persistence):** `./le test qemu appliance-ota` (`internal/le/test/qemu/install_ota.go`)
- **SSH `config-push` + reboot persistence in QEMU:** `./le test qemu appliance-config` (`internal/le/test/qemu/install_config_push.go`)
- **Appliance functional tests:** `./le test functional appliance` (`test/appliance/`)
- **Commit-confirmed timeout, accept, abort, disconnect, and restart tests:** `./le test bgp plugin --pattern session-editor-commit-confirmed` (`test/plugin/session-editor-commit-confirmed-*.ci`)
- **Live and offline backup/restore tests:** `test/plugin/data-backup.ci`, `test/plugin/data-backup-live.ci`, `test/plugin/data-backup-refused-live.ci`

## Current limits

- **Shared boot partition and persistent `/perm`:** Gokrazy maintains two root slots, while the boot partition is shared and `/perm` persists across root-slot rollbacks. Rolling back a `--testboot` image does not revert config edits saved in `/perm`.
- **Operator verification required on `--testboot` and `config-push`:** Neither `--testboot` nor `./ze-host appliance config-push` runs automatic routing health checks; operators verify SSH and routing state before promoting.
- **No external shell plugins on gokrazy:** Read-only gokrazy images have no `/bin/sh` or `PATH`, so only built-in internal plugins run on the appliance.
- **Separate backups for workstation vs. live store:** `./ze-host appliance export` backs up the bastion manifest and keys, not disk images or the appliance's live ZeFS database (keep `request data backup` separately).
- **`update system firmware` is Linux/systemd only:** Self-update is not supported on read-only gokrazy filesystems; use `./ze-host appliance push`.

## Related guides

- [Appliance guide](../../guides/appliance/)
- [Appliance install lab](../../labs/appliance-install/)
- [Config editor guide](../../guides/config-editor/)
- [Config archive guide](../../guides/config-archive/)
- [Self-update guide](../../guides/self-update/)
