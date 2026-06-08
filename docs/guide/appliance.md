# VM Appliance

Build a bootable VM image with Ze baked in using [gokrazy](https://gokrazy.org). The default target is x86_64. The legacy Make workflow uses `GOKRAZY_ARCH=arm64` for native Apple Silicon QEMU images; the structured `ze appliance build` workflow uses `image.arch` in `appliance.json`. The result is a minimal Linux system that runs Ze as its only application, with no package manager, no shell (except emergency serial console), and automatic process supervision.

Suitable for N100-class mini PCs, Proxmox VMs, or QEMU testing.
<!-- source: gokrazy/ze/config.json -- Packages, KernelPackage, Environment -->

## What's in the image

| Component | Purpose |
|-----------|---------|
| Linux kernel | Boot and hardware drivers |
| Gokrazy init | Process supervisor, entropy seeding, watchdog heartbeat |
| Ze | BGP daemon with DHCP client, NTP, and all internal plugins |
| serial-busybox | Emergency shell on serial console (not started by default) |

Ze owns network configuration (DHCP) and time synchronization (NTP). The gokrazy
default DHCP and NTP packages are excluded from the image -- ze handles both via
its config pipeline (`interface { dhcp-auto true }` discovers the first ethernet
and runs DHCP on it; `environment { ntp { enabled true } }` syncs the clock).

The root filesystem is read-only (SquashFS). Persistent data lives on a separate ext4 partition mounted at `/perm`.

## Prerequisites

Install once on the build machine (macOS):

```bash
brew install e2fsprogs    # ext4 filesystem tools
brew install qemu         # VM runtime (testing only)
```

For appliance ISO creation, install `grub-mkstandalone` (or `grub2-mkstandalone`)
plus `xorriso`.
`ze appliance iso` checks those tools before it stages an ISO.
<!-- source: cmd/ze/install/appliance/cmd_iso.go -- resolveISOBuilder -->

The gokrazy build tool (`gok`) is vendored in the repo at `gokrazy/tools/vendor/` and built automatically by Make. No separate install needed.
<!-- source: gokrazy/tools/tools.go -- vendored gok tool -->

## First-time setup

After cloning the repo, download gokrazy system packages (Linux kernel, init, serial console) into the Go module cache. This is a one-time ~42MB download. The exact versions are pinned in `gokrazy/ze/builddir/*/go.mod` (tracked in git, verified by go.sum).

```bash
make ze-gokrazy-deps
```

After this, builds work offline.
<!-- source: gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod -- pinned kernel version -->
<!-- source: gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod -- pinned gokrazy version -->

## L2TP Kernel Support

Ze's L2TP LNS path needs kernel PPPoL2TP support in the appliance kernel:
`CONFIG_PPP`, `CONFIG_PPPOL2TP`, `CONFIG_L2TP`, and `CONFIG_L2TP_NETLINK`.
The pinned upstream gokrazy kernel is not assumed to provide these options.
Build the repo-local kernel before building an appliance intended to terminate
L2TP subscribers:

```bash
make ze-kernel
make ze-gokrazy USER=admin PASS=secret
```

On Apple Silicon, use a native arm64 VM image to avoid x86_64 emulation while
still building the kernel with the same L2TP/PPP options:

```bash
make ze-kernel GOKRAZY_ARCH=arm64
make ze-gokrazy GOKRAZY_ARCH=arm64 USER=admin PASS=secret
make ze-gokrazy-run GOKRAZY_ARCH=arm64 GOKRAZY_QEMU_ACCEL=hvf
```

`make ze-kernel` appends `gokrazy/kernel/l2tp.config.addendum.txt` to the
rtr7 kernel addendum, builds the kernel with gokrazy's rebuild tooling, and
overlays the gitignored module-cache copy used by `make ze-gokrazy`. The first
overlay backs up the pinned cache. Use `make ze-kernel-clean` to restore it.

On a Linux runner with QEMU, `xl2tpd`, `pppd`, `/dev/ppp`, and PPPoL2TP kernel
support, the deployment proof target builds an L2TP-enabled appliance image and
drives a real LAC against it:

```bash
make ze-deployment-gokrazy-l2tp-ppp-test
```

The proof image is built from a temporary gokrazy instance config so the normal
appliance config is left unchanged. It disables IPv6CP in that proof image
because the current static L2TP pool is IPv4-only. Set
`ZE_GOKRAZY_SKIP_BUILD=1` to run against an existing `tmp/gokrazy/ze.img` that
was already built with the L2TP proof template and proof runtime environment.
<!-- source: gokrazy/kernel/l2tp.config.addendum.txt -- Ze L2TP/PPP kernel config -->
<!-- source: Makefile -- ze-kernel target -->
<!-- source: scripts/evidence/effective-gokrazy-l2tp-ppp.py -- appliance L2TP proof -->

## Build an image

First build (creates SSH credentials and a TLS certificate):

```bash
make ze-gokrazy USER=admin PASS=secret
```

Subsequent rebuilds reuse the existing database (same credentials, same TLS cert):

```bash
make ze-gokrazy
```

To use a database from a running instance or another machine:

```bash
make ze-gokrazy ZEFS=/path/to/database.zefs
```

To build with a different first-boot template without editing
`gokrazy/ze/ze.conf`:

```bash
make ze-gokrazy USER=admin PASS=secret GOKRAZY_TEMPLATE=tmp/my-ze.conf
```

The first build:

1. Builds `bin/ze` for the host
2. Runs `ze init` with credentials and generates a self-signed TLS certificate
3. Cross-compiles Ze for linux/`GOKRAZY_ARCH` in the Make workflow, or linux/`image.arch` in the structured `ze appliance build` workflow, and builds a 2GB disk image
4. Formats the persistent `/perm` partition
5. Injects `database.zefs` (credentials + TLS cert) into `/perm/ze/`

The database is kept at `tmp/gokrazy/init/database.zefs` between builds. Browsers that trust the certificate on first use will not prompt again after image rebuilds.

The image lands at `tmp/gokrazy/ze.img`.
<!-- source: Makefile -- ze-gokrazy target -->

## Test in QEMU

```bash
make ze-gokrazy-run
```

This boots the image with port forwarding:

| Host port | Guest service | URL / command |
|-----------|---------------|---------------|
| 18080 | Gokrazy web UI (80) | `http://localhost:18080/` |
| 28080 | Ze web UI (8080) | `http://localhost:28080/` |
| 2222 | Ze SSH CLI (22) | `ssh -p 2222 admin@localhost` |

Quit QEMU with **Ctrl-A X**.

The gokrazy web UI shows process status, stdout/stderr ring buffers, and resource usage. Default credentials are in `gokrazy/ze/config.json` (`Update.HTTPPassword`).

## Deploy to hardware

Write the image to a USB drive or internal disk on your N100 machine:

```bash
# Linux
sudo dd if=tmp/gokrazy/ze.img of=/dev/sdX bs=4M status=progress

# macOS
sudo dd if=tmp/gokrazy/ze.img of=/dev/rdiskN bs=4m
```

Or import into Proxmox:

```bash
qm importdisk <vmid> tmp/gokrazy/ze.img <storage>
```

The machine boots to a serial console (115200 baud). Ze starts automatically, gets a DHCP address, and begins listening for BGP connections according to `/etc/ze/ze.conf`.

## Configuration

### Seed config

The initial Ze config is embedded in the read-only root filesystem at `/etc/ze/ze.conf`. It is baked into the image at build time from the `ExtraFileContents` field in `gokrazy/ze/config.json`:
<!-- source: gokrazy/ze/config.json -- ExtraFileContents /etc/ze/ze.conf -->

```
environment {
    log {
        level info
    }

    web {
        enabled true
        server default {
            ip 0.0.0.0
            port 8080
        }
    }

    ssh {
        enabled true
        server default {
            ip 0.0.0.0
            port 22
        }
    }
}
```

To change the seed config, edit the `ExtraFileContents` value in `gokrazy/ze/config.json` and rebuild.

### Runtime config

Once booted, use `ze config edit` over SSH to modify the running configuration. Changes are stored in `/perm/ze/database.zefs` and persist across reboots and image updates.

### Environment variables

Ze's environment is set in `gokrazy/ze/config.json` under `PackageConfig`:
<!-- source: gokrazy/ze/config.json -- Environment array -->

| Variable | Value | Purpose |
|----------|-------|---------|
| `ze.config.dir` | `/perm/ze` | Persistent storage for database.zefs |
| `ze.bgp.api.socketpath` | `/tmp/ze.socket` | API socket location |
| `ze.bgp.daemon.drop` | `false` | No privilege dropping (no `zeuser` on gokrazy) |
| `ze.log` | `info` | Log level |
| `ze.log.backend` | `stderr` | Logs go to gokrazy ring buffer |

## Updating

Gokrazy supports atomic A/B partition updates over the network:

```bash
bin/ze-setup appliance push <name>
```

This pushes the most recent image to the device. The system reboots into the new version. If the update fails mid-flight, the previous root partition is still intact.

For full image rebuilds (when you also want to update the kernel or partition layout), use `ze appliance build <name>` again and re-flash.

## Architecture notes

### Internal plugins only

Gokrazy has no shell and no PATH. Ze's external plugin mechanism (which uses `/bin/sh -c` to fork processes) does not work. All Ze plugins (bgp-rib, bgp-gr, bgp-adj-rib-in, etc.) are compiled into the ze binary as internal plugins and run as goroutines. This is the default and covers all standard BGP functionality.
<!-- source: internal/component/plugin/process/process.go -- startExternal uses /bin/sh -->

### Process supervision

Gokrazy's init restarts Ze if it exits with a non-zero status (except 125, which means "don't restart"). Ze handles SIGTERM for graceful shutdown. Logs (stdout/stderr) are captured in ring buffers visible through the gokrazy web UI.

### Persistent storage

The `/perm` partition (ext4) survives image updates. Ze stores its database (`database.zefs`), TLS certificates, and config state there via the `ze.config.dir=/perm/ze` environment variable.

## Repo layout

```
gokrazy/
  .gitignore              # excludes *.img
  tools/
    tools.go              # blank import pinning gok version
    go.mod, go.sum        # gok dependency pins
    vendor/               # vendored gok source (~16MB, committed)
  ze/
    config.json           # gokrazy instance config (what to build, how to start)
    builddir/
      codeberg.org/thomas-mangin/ze/
        go.mod            # ze dependency pins + relative replace directive
        go.sum
      github.com/rtr7/kernel/
        go.mod, go.sum    # linux kernel version pin
      github.com/gokrazy/gokrazy/
        go.mod, go.sum    # gokrazy init system version pin
        cmd/dhcp/         # DHCP client
        cmd/ntp/          # NTP client
        cmd/heartbeat/    # watchdog heartbeat
        cmd/randomd/      # entropy seeder
      github.com/gokrazy/serial-busybox/
        go.mod, go.sum    # emergency serial shell
```

The `tools/vendor/` directory contains the gok build tool source (committed to git). The `builddir/` files are small text (go.mod + go.sum, ~27KB). System packages (kernel, init) live in the Go module cache after `make ze-gokrazy-deps`.

## ze-setup binary

Appliance build commands (`ze appliance`) and PXE provisioning (`ze install
remote`) are part of the `ze-setup` binary. This keeps build-host tooling out
of the on-device `ze` binary.

Build ze-setup from the repo root:

```bash
make ze-setup              # produces bin/ze-setup
```

## Building and installing an appliance (end to end)

The full pipeline from source to bootable media:

```bash
# 1. Build ze-setup
make ze-setup

# 2. Create an appliance with its config and secrets
bin/ze-setup appliance init --config appliance.json prod

# 3. Build the full disk image (gokrazy + ZeFS credentials)
bin/ze-setup appliance build prod

# 4. Prepare ISO prerequisites (download or build automatically)
bin/ze-setup appliance iso --check               # see what is ready
bin/ze-setup appliance kernel prod                # download or QEMU-build the installer kernel
bin/ze-setup appliance initrd                    # download or build the installer initrd

# 5. Build the bootable installer ISO
bin/ze-setup appliance iso prod

# 6. Install: either boot the ISO on the target machine, or PXE provision
#    Option A: copy the ISO to a USB stick or mount in a VM
#    Option B: serve over the network with PXE
bin/ze-setup install remote \
  --interface eth0 \
  --network 10.0.0.0/24 \
  --image ~/.config/ze/appliances/prod/ze-*.img \
  --ssh-username admin \
  --ssh-password 'choose-a-strong-one'
```

The `kernel` and `initrd` commands try three sources in order: XDG cache hit,
download from the release server, and local build (QEMU VM for the kernel, make
for the initrd). Once cached, subsequent runs are instant. See
"ISO prerequisites" below for details.

## ze appliance (structured workflow)

The `ze appliance` command provides structured appliance management. Each appliance has its own directory with a JSON config, encrypted secrets, and a TLS certificate.

### Quick start

```bash
bin/ze-setup appliance init lab                  # interactive wizard
bin/ze-setup appliance build lab                 # full image (assemble + gok + ext4)
bin/ze-setup appliance kernel prod                # download or build installer kernel
bin/ze-setup appliance initrd                    # download or build installer initrd
bin/ze-setup appliance iso lab                   # bootable installer ISO from latest image
bin/ze-setup appliance list                      # show all appliances
bin/ze-setup appliance show lab                  # config summary + cert expiry
```

### Appliance directory

By default, appliances live in `~/.config/ze/appliances/`. Override with `--dir` or `ZE_APPLIANCE_DIR`.

```
~/.config/ze/appliances/
  _shared/
    ze.conf                    # optional base config for all appliances
  lab/
    appliance.json             # config (no credentials)
    ze.conf                    # per-device config overrides
    secrets/                   # 0700 permissions
      .encrypted               # marker (present = secrets encrypted)
      tls/
        cert.pem               # public certificate (plaintext)
        key.pem                # private key (encrypted if passphrase set)
      password.hash            # bcrypt hash (encrypted if passphrase set)
      update.token             # gokrazy OTA token (encrypted if passphrase set)
      authorized_keys          # SSH public keys (plaintext)
```

### Encryption

Secrets are encrypted at rest with Argon2id + XChaCha20-Poly1305 when an encryption passphrase is set during `ze appliance init`. The passphrase is never stored on disk. For fleet operations, `ze appliance unlock` starts a passphrase agent (like ssh-agent) that holds the derived key in memory.

```bash
bin/ze-setup appliance unlock                    # start agent (prompts for passphrase)
bin/ze-setup appliance unlock --duration 15m     # auto-expire after 15 minutes
bin/ze-setup appliance unlock --stop             # stop agent
```

### Day-2 operations

```bash
bin/ze-setup appliance passwd lab                # rotate SSH password
bin/ze-setup appliance replace-cert lab          # regenerate self-signed cert
bin/ze-setup appliance replace-cert lab --cert ca.pem --key ca.key   # use CA-signed cert
bin/ze-setup appliance rekey lab                 # change encryption passphrase
bin/ze-setup appliance clone lab lab2            # copy config (not secrets)
```

### Config layering

Set `config-base` in `appliance.json` to share a base config across appliances:

```json
{
  "config-base": "../_shared/ze.conf"
}
```

The base config is read first, then per-appliance `ze.conf` is appended. Later `set` commands override earlier ones; `delete` commands remove settings from the base.

### Commands reference

| Command | Purpose |
|---------|---------|
| `init <name>` | Create appliance with config + encrypted secrets |
| `assemble <name>` | Build ZeFS database only (auto-deletes; use `--keep` to retain) |
| `build <name>` | Full image: assemble + gok + ext4 inject + checksum + manifest |
| `build --all` | Build all appliances |
| `kernel [--arch] [--profile] [--version] [<name>]` | Download or build the installer kernel (reads `kernel-profile` from config) |
| `initrd` | Download or build the installer initrd |
| `iso <name>` | Bootable installer ISO from an existing image |
| `iso --check` | Check ISO prerequisites without building |
| `passwd <name>` | Change SSH password |
| `replace-cert <name>` | Replace TLS cert (regenerate or `--cert`/`--key` for CA) |
| `rekey <name>` | Change encryption passphrase |
| `clone <src> <dst>` | Copy config, not secrets |
| `list` | List appliances with hostname and arch |
| `show <name>` | Show config, cert expiry, managed status |
| `run <name>` | Boot in QEMU with port forwarding |
| `unlock` | Start passphrase agent |
| `push <name>` | Push image to device via gokrazy OTA update |
| `push --all` | Push to all appliances with device.address |
| `config <name> --merged` | Show effective config (base + overlay) |
| `config-push <name>` | Push config to running device via SSH |
| `config-push --all` | Push config to all addressed devices |
| `init --batch <manifest>` | Batch init from JSON manifest |
| `export <name>` | Export appliance to encrypted archive (.ze.enc) |
| `export --all` | Export all appliances to single encrypted archive |
| `import <archive>` | Import appliance from encrypted archive |


### ISO prerequisites

The ISO build requires an installer kernel, an initrd, `grub-mkstandalone`, and
`xorriso`. Use `ze appliance iso --check` to see what is ready and what is
missing. The `kernel` and `initrd` commands handle downloading or building these
artifacts automatically:

    bin/ze-setup appliance iso --check               # report readiness
    bin/ze-setup appliance kernel prod                # download or build kernel (reads profile from config)
    bin/ze-setup appliance kernel --profile hardware prod   # explicit hardware profile
    bin/ze-setup appliance initrd                    # download or build initrd
    bin/ze-setup appliance iso lab                   # build ISO

Both commands try three tiers in order: XDG cache hit, download from a release
server, and local build (QEMU VM for the kernel, make for the initrd). Downloaded
artifacts are cached under `$XDG_CACHE_HOME/ze/` (default `~/.cache/ze/`) and
also copied to `tools/installer-kernel/build/` and `tools/installer-initrd/build/`
so `ze appliance iso` finds them without extra flags.

The download URL defaults to the project release server. Override with the
`ze.appliance.kernel.url` and `ze.appliance.initrd.url` environment variables.

`ze doctor` includes checks for kernel, initrd, grub, xorriso, and e2fsprogs
availability, reporting warnings with actionable hints when prerequisites are
missing.

### ISO installer media

Create an installer ISO from an image already produced by `ze appliance
build`. By default the command selects the latest `ze-*.img` in the appliance
directory, verifies its `.sha256` sidecar, and writes `ze-*.iso` next to the
image. Use `--image` to select a specific image filename and `--output` to write
the ISO elsewhere. The output path must not overwrite the selected `.img`, and
the image filename must stay within `[A-Za-z0-9._-]` so the initrd can pass it
on the kernel command line. By default the installer kernel path is
`tools/installer-kernel/build/Image` or a cached download under
`$XDG_CACHE_HOME/ze/`; build the matching architecture before you
run `ze appliance iso`, or pass `--kernel` to keep multiple kernels
side by side.
<!-- source: cmd/ze/install/appliance/cmd_iso.go -- runIso, resolveISOInput, readRequiredImageChecksum -->

    bin/ze-setup appliance build lab
    bin/ze-setup appliance kernel --profile hardware lab    # build hardware kernel
    bin/ze-setup appliance iso lab
    bin/ze-setup appliance iso --image ze-20260601-120000.img lab
    bin/ze-setup appliance iso --output /path/to/lab.iso lab
    bin/ze-setup appliance iso --kernel tools/installer-kernel/build/Image lab

The ISO is an installer envelope around the existing raw gokrazy image. The image
is gzip-compressed inside the ISO to reduce media size (a 2 GiB image with ~100
MiB of content compresses to roughly 100 MiB). The installer initrd decompresses
the image during installation. The ISO does not rebuild the appliance, regenerate
credentials, fetch a separate ZeFS database, or mutate `/perm` after writing the
disk image. The installed disk receives the selected image bytes, including the
`/perm/ze/database.zefs` that `build` already injected.
<!-- source: cmd/ze/install/appliance/cmd_iso.go -- stageISO -->
<!-- source: tools/installer-initrd/init -- ZE_SOURCE=iso branch -->
<!-- source: cmd/ze/install/appliance/cmd_build.go -- injectZeFS -->

The ISO boot path accepts an optional explicit target disk. If no target is set,
the installer writes only when exactly one non-removable candidate disk remains
after excluding the ISO source media. The initrd also matches the booted ISO by
a builder-generated `ze.media-id` token before it trusts a mounted installer
volume, so identical image filenames on multiple attached installer media do not
confuse the source selection. With multiple fixed disks, pass a whole disk path
such as `/dev/vda` at ISO creation time:
<!-- source: tools/installer-initrd/init -- find_target_disk, find_iso_media, validate_media_id -->

    bin/ze-setup appliance iso --target /dev/vda lab

After the installer writes the disk in ISO mode, it powers off instead of
rebooting. Remove the installer media, then power the target back on so the
firmware boots from the written disk.
<!-- source: tools/installer-initrd/init -- ZE_SOURCE=iso branch -->

The ISO contains the full provisioned appliance image, including the embedded
ZeFS database. Handle the ISO with the same care as the `.img` file.
<!-- source: cmd/ze/install/appliance/cmd_iso.go -- stageISO -->

**USB write method:** the ISO can be written with `dd`, Etcher, or Rufus in DD
mode. Ventoy is also supported when the installer kernel includes loop device
and FAT/exFAT filesystem support (the `hardware` kernel profile has this). The
initrd detects the ISO file on the Ventoy data partition, loop-mounts it, and
proceeds with the installation. When using the `qemu` kernel profile, Ventoy
is not supported.

### Remote operations (push, config-push)

Push a built image to a running gokrazy device via its HTTPS update endpoint:

    bin/ze-setup appliance push lab
    bin/ze-setup appliance push --image ze-20260427-143022.img lab   # rollback to older image
    bin/ze-setup appliance push --all                                # all devices with address
    bin/ze-setup appliance push --all --parallel 4                   # 4 concurrent uploads

Push uses the update token (from `secrets/update.token`) for HTTP basic auth, and verifies the device TLS certificate against the stored `cert.pem`. No system CA pool is consulted.

When `--image` is set, the file name must resolve to a regular file inside the
appliance directory. Path traversal and symlinks escaping that directory are
rejected before any network or TLS work starts.
<!-- source: cmd/ze/install/appliance/cmd_push.go -- resolveImagePath -->

Preview the effective configuration (base + overlay merged) without building:

    bin/ze-setup appliance config lab --merged

Push a config change to a running device without rebuilding the image:

    bin/ze-setup appliance config-push lab
    bin/ze-setup appliance config-push --dry-run lab    # preview only, no SSH connection
    bin/ze-setup appliance config-push --all            # all addressed devices
    bin/ze-setup appliance config-push --all --parallel 4

Config-push uses SSH (operator's key via ssh-agent) to upload the merged config to the device, which validates and applies it. No secrets are transmitted over SSH.

### Device-side config behavior

At boot, the device loads configuration with the following priority:

| Priority | Source | Location |
|----------|--------|----------|
| 1 (highest) | Pushed config | `/perm/ze/config-pushed.conf` |
| 2 | Seed config | `file/template/ze.conf` in ZeFS (bootstrap + interface discovery) |

If a pushed config exists and passes validation (`config.LoadConfig`), the device uses it. If it fails validation, the device deletes it, logs a warning, and falls back to the seed config.

After loading the effective config, the device writes its SHA-256 hash to `/perm/ze/config-active-hash` for fleet drift detection.

**Last-known-good hash:** at build time, `ze appliance build` writes the SHA-256 of the validated seed config to `meta/config/last-known-good` in ZeFS. This is immutable and serves as the integrity baseline.

**Auto-revert after config-push:** when `config-push` applies a new config, the device monitors BGP sessions for 30 seconds. If any session flaps during that window, the device reverts to the previous config (or the seed config if no previous exists). If all sessions remain stable, the new config is confirmed and its hash is written to `/perm/ze/last-known-good-pushed`.

### Batch init

Initialize multiple appliances from a JSON manifest:

    bin/ze-setup appliance init --batch manifest.json

Manifest format (array of entries):

```json
[
  {"name": "edge-01", "hostname": "edge-01.lab", "password": "secret1", "device.address": "10.0.0.1"},
  {"name": "edge-02", "hostname": "edge-02.lab", "password": "generate"}
]
```

Use `"password": "generate"` for per-device random passwords (printed to stdout once, never stored in plaintext). Each appliance receives independent cryptographic state (unique salt/nonce).

### Disaster recovery (export/import)

Export creates an encrypted archive of an appliance directory for offsite backup or bastion migration. Archives include config, secrets, and build metadata, but exclude images and ZeFS databases (both are rebuildable).

Export a single appliance:

    bin/ze-setup appliance export lab
    # creates lab.ze.enc in the current directory

Export all appliances:

    bin/ze-setup appliance export --all
    # creates appliances-YYYYMMDD-HHMMSS.ze.enc

Import restores from an archive:

    bin/ze-setup appliance import lab.ze.enc

Import to a different bastion (migration):

    bin/ze-setup appliance import lab.ze.enc --dir /path/to/new/bastion

Archives are always encrypted using the same Argon2id + XChaCha20-Poly1305 scheme as secrets at rest. The archive passphrase can differ from the secrets passphrase. Use `--force` on import to overwrite existing appliance directories.
