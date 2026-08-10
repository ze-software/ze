# VM Appliance

Build a bootable VM image with Ze baked in using [gokrazy](https://gokrazy.org/). The default target is x86_64. The legacy Make workflow uses `GOKRAZY_ARCH=arm64` for native Apple Silicon QEMU images; the structured `ze appliance build` workflow uses `image.arch` in `appliance.json`. At runtime, the appliance is minimal: Linux kernel, gokrazy init, and Ze as the only application, with no package manager, no general shell (except authenticated emergency serial console), no unused distro daemons, and automatic process supervision.

Suitable for N100-class mini PCs, Proxmox VMs, or QEMU testing.
<!-- source: gokrazy/ze/config.json -- Packages, KernelPackage, Environment -->

## What's in the image

| Component | Purpose |
|-----------|---------|
| Linux kernel | Boot and hardware drivers |
| [gokrazy](https://gokrazy.org/) init | Starts Ze, supervises it, seeds entropy, sends watchdog heartbeat |
| Ze | BGP daemon with DHCP client and all internal plugins |
| ze-serial-shell | Authenticated emergency shell on serial console (login required) |

Ze owns network configuration in the appliance. The gokrazy default DHCP and
NTP packages are excluded from the image; the shipped Ze seed template enables
interface DHCP auto-discovery with `set interface dhcp-auto true` and leaves Ze
NTP disabled (`set environment ntp enabled false`) until the operator enables it
in Ze config.

The root filesystem is read-only (SquashFS). Persistent data lives on a separate ext4 partition mounted at `/perm`.

## Prerequisites

Install once on the build machine.

macOS:

```bash
brew install e2fsprogs    # ext4 filesystem tools
brew install qemu         # VM runtime (testing only)
```

Linux:

```bash
sudo apt-get install -y e2fsprogs qemu-system-x86   # Debian, Ubuntu
sudo dnf install -y e2fsprogs qemu-system-x86       # Fedora
```

The build needs BOTH `mkfs.ext4` and `debugfs` from e2fsprogs: it formats
`/perm` with the first and injects the seed database with the second. The build
finds them by itself, in `/usr/sbin`, `/sbin`, `/usr/local/sbin`, then the
homebrew Cellar, and it takes the first directory holding both. Pass
`make ze-gokrazy E2FS=/path/to/sbin` to name the directory instead. An empty
`E2FS=` is not an override and does not resume the search.
<!-- source: mk/gokrazy.mk -- E2FS autodetect and the ze-gokrazy e2fsprogs guard -->

For appliance ISO creation, install `grub-mkstandalone` (or `grub2-mkstandalone`)
plus `xorriso`.
`ze appliance iso` checks those tools before it stages an ISO.
<!-- source: internal/appliance/cmd_iso.go -- resolveISOBuilder -->

The gokrazy build tool (`gok`) is vendored in the repo at `vendor/github.com/gokrazy/` and built automatically by Make. No separate install needed.
<!-- source: cmd/ze-gok/main.go -- vendored gok tool -->

## First-time setup

After cloning the repo, download gokrazy system packages (Linux kernel, init, serial console) into the Go module cache. This is a one-time ~42MB download. The exact versions are pinned in `gokrazy/ze/builddir/*/go.mod` (tracked in git, verified by go.sum).

```bash
make ze-gokrazy-deps
```

After this, builds work offline.
<!-- source: gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod -- pinned kernel version -->
<!-- source: gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod -- pinned gokrazy version -->

## L2TP Kernel Support

Ze's L2TP LNS path needs kernel PPPoL2TP support in the appliance runtime
kernel: `CONFIG_PPP`, `CONFIG_PPPOL2TP`, `CONFIG_L2TP`, and
`CONFIG_L2TP_V3`. The shared runtime proof kernel also keeps `CONFIG_PPPOE`
built in for PPPoE evidence. The pinned upstream gokrazy kernel is not assumed
to provide these options.
Build the repo-local kernel before building an appliance intended to terminate
L2TP subscribers:

```bash
make ze-kernel                                   # default runtime build: docker, amd64
make ze-kernel KERNEL_BUILDER=qemu               # force the shared QEMU backend
make ze-kernel KERNEL_ARCH=arm64                 # runtime arm64 kernel
make ze-kernel KERNEL_ARCH=arm64 KERNEL_BUILDER=qemu
make ze-gokrazy USER=admin PASS=secret
```

On Apple Silicon, use a native arm64 VM image to avoid x86_64 emulation while
still building the kernel with the same L2TP/PPP options:

```bash
make ze-kernel KERNEL_ARCH=arm64                 # default builder is docker
make ze-kernel KERNEL_ARCH=arm64 KERNEL_BUILDER=qemu
make ze-gokrazy GOKRAZY_ARCH=arm64 USER=admin PASS=secret
make ze-gokrazy-run GOKRAZY_ARCH=arm64 GOKRAZY_QEMU_ACCEL=hvf
```

`make ze-kernel` delegates to `gokrazy/kernel/Makefile`, which calls the single
shared driver `tools/kernel-builder/run.py`. The driver reads the kernel version
from `internal/appliance/kernel.version`, selects the Docker backend by default
(or the QEMU backend with `KERNEL_BUILDER=qemu`), resolves the tracked
`gokrazy/kernel/kernel.config` + `runtime.config` fragments (plus the shared
`# ze-include: efi-console` console fragment) and matching `.require` manifests,
and emits `vmlinuz`, `lib/modules/`, and DTBs. `make ze-kernel` then assembles
those into an out-of-tree kernel package (`tmp/kernel/pkg`, a copy of the pinned
`rtr7/kernel` module with our artifacts overlaid) and points `gok` at it via a
`go.mod` `replace`. The pinned module cache is never mutated in place and there
is no backup to restore; `make ze-kernel-clean` drops the `replace` and removes
`tmp/kernel`.
<!-- source: mk/gokrazy.mk -- ze-kernel -->
<!-- source: gokrazy/kernel/Makefile -- all -->
<!-- source: tools/kernel-builder/run.py -- main -->

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
was already built with the L2TP proof template, the proof runtime environment,
and an L2TP-capable kernel: skip-build bypasses the proof's own kernel
resolution, and an image on the pinned rtr7 kernel (which has no l2tp support)
crash-loops at first boot instead of serving.
<!-- source: gokrazy/kernel/runtime.config -- Ze L2TP/PPP kernel config -->
<!-- source: mk/gokrazy.mk -- ze-kernel -->
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

The legacy Make first build:

1. Builds `bin/ze` for the host
2. Runs `ze init --seed` with credentials and generates a self-signed TLS certificate. `--seed` skips baking the build host's discovered interfaces into the active config; otherwise that active config would hold the wrong host's NICs and shadow the seed template, leaving the appliance without web/L2TP. The appliance instead builds its active config at first boot from the template merged with its own on-device discovery.
3. Cross-compiles Ze for linux/`GOKRAZY_ARCH` and builds a 2GB disk image
4. Formats the persistent `/perm` partition
5. Injects `database.zefs` (credentials + TLS cert) into `/perm/ze/`

The database is kept at `tmp/gokrazy/init/database.zefs` between builds. Browsers
that trust the certificate on first use will not prompt again after image
rebuilds. Structured `ze appliance build` also writes a build manifest into
`/perm/ze/build.json`; the legacy `make ze-gokrazy` flow does not.

The image lands at `tmp/gokrazy/ze.img`.
<!-- source: mk/gokrazy.mk -- ze-gokrazy -->

## Test in QEMU

```bash
make ze-gokrazy-run
```

This boots the image with these legacy Make forwards:

| Host URL / command | Guest service |
|--------------------|---------------|
| `https://localhost:28080/` | Ze web UI (8080) |
| `https://localhost:28080/gokrazy/` | Gokrazy management UI proxied by Ze |
| `ssh -p 2222 admin@localhost` | Ze SSH CLI (22) |

Quit QEMU with **Ctrl-A X**.

The Gokrazy management UI shows process status, stdout/stderr ring buffers, and
resource usage. In appliance mode it is exposed under Ze's authenticated web UI
at `/gokrazy/`; the proxy reads Gokrazy's password from the same password-file
locations Gokrazy uses when it needs to inject upstream Basic Auth.
<!-- source: mk/gokrazy.mk -- ze-gokrazy-run -->
<!-- source: internal/component/web/register_gokrazy.go -- /gokrazy route -->
<!-- source: internal/core/gokrazyutil/gokrazyutil.go -- ReadPassword -->

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

The machine boots to a serial console (115200 baud). Ze starts automatically, gets a DHCP address, and loads its active configuration from `/perm/ze/database.zefs` (bootstrapped from the seed template on first boot). The serial console requires authentication with the local admin credentials before granting shell access. If the credentials database is missing or unreadable, access is granted without authentication for emergency recovery. When `admin-enabled: false` is set in the appliance config, the serial console denies the built-in admin (fail-closed) and prints "local admin login disabled".
<!-- source: cmd/ze/login.go -- loginMain, fail-open path, admin-disabled check -->

## Configuration

### Seed config

The initial Ze config is stored as the seed template in `gokrazy/ze/ze.conf`.
Legacy Make writes that file into `file/template/ze.conf` in ZeFS during
`make ze-gokrazy`; structured `ze appliance assemble` uses the same default when
no base or per-appliance overlay config is present. Because `make ze-gokrazy`
runs `ze init --seed`, the seed DB has no `file/active/ze.conf` to shadow the
template, so the template becomes the effective config on first boot (`ze
appliance assemble` never wrote an active config, so it was already correct).
<!-- source: gokrazy/ze/ze.conf -- seed template -->
<!-- source: mk/gokrazy.mk -- GOKRAZY_TEMPLATE write, ze init --seed -->
<!-- source: internal/plugins/init/main.go -- runInit seed skips file/active write -->
<!-- source: internal/appliance/cmd_assemble.go -- resolveSeedConfig -->

```bash
set environment log level info
set environment web enabled true
set environment web server default ip 0.0.0.0
set environment web server default port 8080
set environment ssh enabled true
set environment ssh server default ip 0.0.0.0
set environment ssh server default port 22
set environment ntp enabled false
set interface dhcp-auto true
```

To change the seed config, edit `gokrazy/ze/ze.conf`, pass
`GOKRAZY_TEMPLATE=/path/to/ze.conf` to the legacy Make workflow, or use the
structured workflow's `config-base` and per-appliance `ze.conf` files.

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
| `ze.log.backend` | `kmsg,stderr` | Logs go to kmsg and gokrazy ring buffers |
| `ze.gokrazy.enabled` | `true` | Enables appliance auto-init fallback and the `/gokrazy/` management proxy |

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
  ze/
    config.json           # gokrazy instance config (what to build, how to start)
    builddir/
      github.com/ze-software/ze/
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
cmd/ze-serial-shell/        # serial console login gate (replaces serial-busybox)
  main.go                   # gokrazy wrapper: symlink + DontStartOnBoot
  _gokrazy/                 # renamed busybox extrafiles per arch
cmd/ze-gok/
  main.go                   # gok wrapper (built by make bin/gok)
```

The gok build tool source is vendored in the main `vendor/github.com/gokrazy/` directory. The `builddir/` files are small text (go.mod + go.sum, ~27KB). System packages (kernel, init) live in the Go module cache after `make ze-gokrazy-deps`.

### Builds never run from this directory

The tree above is the build *input*; no build runs inside it. Every image build
first copies `gokrazy/ze` to a fresh directory under the project `tmp/`,
including the whole `builddir`, rewriting each filesystem-path `replace` to an
absolute path so it still resolves from the new depth. gok is then pointed at the
copy, and the copy is deleted afterwards.

<!-- source: internal/appliance/instance/prepare.go -- Prepare, copyBuildDir, absolutizeReplaces -->

Both entry points do this: `ze appliance build` through `resolveBuildParentDir`,
and `make ze-gokrazy` through `bin/gok`, which rewrites `--parent_dir` before gok
sees it. As a result a build leaves the working tree unchanged, two builds in one
checkout use isolated prepared instances, and a build that would have to resolve
packages over the network fails instead of silently using unpinned versions. The
one shared mutable path left is `tmp/kernel/pkg`: every `make ze-kernel` rewrites
it (starting with a delete), so concurrent kernel materializations for different
architectures do collide there. The L2TP boot proof therefore consumes a per-run
copy of the package, never the shared path.

<!-- source: internal/appliance/kernelargs.go -- resolveBuildParentDir -->
<!-- source: cmd/ze-gok/main.go -- prepareArgs -->

To build against a locally built kernel, pass it per build:

```
make ze-kernel                                   # builds tmp/kernel/pkg
make ze-gokrazy KERNEL_PKG=tmp/kernel/pkg USER=admin PASS=secret
```

The `replace` is written into the prepared copy only, so nothing needs reverting
afterwards and a later build without `KERNEL_PKG` uses the pinned kernel.

<!-- source: mk/gokrazy.mk -- KERNEL_PKG, ze-kernel -->
<!-- source: internal/appliance/instance/prepare.go -- replaceKernel -->


## ze-setup binary

Appliance build commands (`ze appliance`) and PXE provisioning (`ze install
remote`) are part of the `ze-setup` binary. This keeps build-host tooling out
of the on-device `ze` binary.

Build ze-setup from the repo root:

```bash
make bin/ze-setup          # produces bin/ze-setup
```

## Building and installing an appliance (end to end)

### From a JSON config (recommended)

Write an appliance config file (arch, kernel profile, credentials, networking):

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

#### Reserving hugepages for VPP

When the appliance runs VPP, reserve hugepages at boot by adding `image.hugepages`
to the config. `ze appliance build` bakes `default_hugepagesz`/`hugepagesz`/`hugepages`
into the boot cmdline (via a derived gokrazy instance config; the checked-in
`gokrazy/ze/config.json` is never modified). Declare `image.memory` so the build
rejects a reservation over 50% of target RAM and `ze appliance run` sizes QEMU's
`-m` to match. Sizes are byte-size strings (`10b`, `512mb`, `1gb`, `1tb`;
case-insensitive, 1024-based):

```json
"image": {
    "arch": "amd64",
    "size-bytes": 2147483648,
    "memory": "8gb",
    "hugepages": { "size": "1gb", "page-size": "2mb" }
}
```

`hugepages.size` is the total reservation and `page-size` is `2mb` or `1gb`; the
page count is `size / page-size` (so `size` must be a whole multiple of
`page-size`). The reservation is bounded to 512 GiB and, when `memory` is set, to
50% of it. 1gb pages need CPU `pdpe1gb` support and `CONFIG_HUGETLBFS` in the
kernel profile (both surfaced by `ze doctor`).
<!-- source: internal/appliance/config.go -- ImageConfig.Hugepages, validateImageMemory -->

Build the full ISO in one command:

```bash
make ze-iso CONFIG=prod.json SSH_PASSWORD='choose-a-strong-one'
```

This runs the entire pipeline: init, kernel build, initrd, disk image, and ISO.
The appliance name is derived from the config filename (`prod.json` creates
appliance `prod`). Subsequent builds with the same config reinitialize from
scratch.

Rebuild after code changes (appliance config unchanged):

```bash
make ze-iso-build NAME=prod
```

Set up PXE boot (optional, after the ISO build):

```bash
make ze-pxe NAME=prod
```

See `make help-deploy` for all variables (`APPLIANCE_BUILDER`, `PXE_DIR`, etc.).
<!-- source: mk/appliance.mk -- ze-iso, ze-iso-build, ze-pxe -->

### Manual steps

The Makefile targets call these `ze-setup` commands under the hood. Run them
individually when you need finer control:

```bash
# 1. Build bin/ze-setup
make bin/ze-setup

# 2. Create an appliance with its config and secrets
env ze.appliance.ssh.password='choose-a-strong-one' \
  bin/ze-setup appliance init --config prod.json prod

# 3. Optional readiness check for ISO prerequisites
bin/ze-setup appliance iso --check

# 4. Prepare ISO prerequisites
bin/ze-setup appliance kernel prod                # download or build the installer kernel
bin/ze-setup appliance initrd                    # download or build the installer initrd

# 5. Build the full disk image (gokrazy + ZeFS credentials)
bin/ze-setup appliance build prod

# 6. Build the bootable installer ISO
bin/ze-setup appliance iso prod

# 7. Install: either boot the ISO on the target machine, or PXE provision
#    Option A: copy the ISO to a USB stick or mount in a VM
#    Option B: serve over the network with PXE
bin/ze-setup install remote \
  --interface eth0 \
  --network 10.0.0.0/24 \
  --image ~/.config/ze/appliances/prod/ze-*.img \
  --ssh-username admin \
  --ssh-password 'choose-a-strong-one'
```

The `kernel` and `initrd` commands first check the XDG cache. If
`ze.appliance.kernel.url` or `ze.appliance.initrd.url` is set, the matching
command then tries that configured prebuilt-artifact URL; otherwise it builds
locally. Kernel local builds use the shared Docker-or-QEMU builder selection, and
initrd local builds compile and pack `cmd/ze-installer`. Once cached, subsequent
runs are instant. See "ISO prerequisites" below for details.

## ze appliance (structured workflow)

The `ze appliance` command provides structured appliance management. Each
appliance has its own directory with a JSON config, secrets that are optionally
encrypted at rest, and a TLS certificate.
<!-- source: internal/appliance/resolve.go -- ResolveDir, ConfigPath, SecretsDir -->
<!-- source: internal/appliance/cmd_init.go -- runInit, WriteSecret -->
<!-- source: internal/appliance/crypto.go -- WriteSecret, Encrypt -->

### Quick start

```bash
bin/ze-setup appliance init lab                  # interactive wizard
bin/ze-setup appliance build lab                 # full image (assemble + gok + ext4)
bin/ze-setup appliance kernel lab                 # download or build installer kernel
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
<!-- source: internal/appliance/crypto.go -- Encrypt, WriteSecret, ResolvePassphrase -->
<!-- source: internal/appliance/agent.go -- passphrase agent -->

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
<!-- source: internal/appliance/cmd_assemble.go -- resolveSeedConfig -->

### Commands reference

| Command | Purpose |
|---------|---------|
| `init <name>` | Create appliance with config + secrets (encrypted when a passphrase is set) |
| `assemble [--keep] <name>` | Build ZeFS database only (auto-deletes; use `--keep` to retain) |
| `build <name>` | Full image: assemble + gok + ext4 inject + checksum + manifest |
| `build --all` | Build all appliances |
| `kernel [--target] [--arch] [--profile] [--builder] [--version] [<name>]` | Download or build an installer or runtime kernel; with `<name>`, reads arch/profile from appliance config |
| `initrd` | Download or build the installer initrd |
| `iso [--image] [--output] [--kernel] [--initrd] [--target] [--builder] [<name>]` | Bootable installer ISO from an existing image |
| `iso --check` | Check ISO prerequisites without building |
| `passwd <name>` | Change SSH password |
| `replace-cert <name>` | Replace TLS cert (regenerate or `--cert`/`--key` for CA) |
| `rekey <name>` | Change encryption passphrase |
| `clone <src> <dst>` | Copy config, not secrets |
| `list` | List appliances with hostname and arch |
| `show <name>` | Show config, cert expiry, managed status |
| `run <name>` | Boot in QEMU with port forwarding |
| `unlock` | Start passphrase agent |
| `push [--image] [--testboot] [--no-reboot] <name>` | Push image to device via gokrazy OTA update |
| `push --all [--parallel N]` | Push to all appliances with device.address |
| `config <name> --merged` | Show effective config (base + overlay) |
| `config-push <name>` | Push config to running device via SSH |
| `config-push --all [--parallel N]` | Push config to all addressed devices |
| `init --batch <manifest>` | Batch init from JSON manifest |
| `export <name>` | Export appliance to encrypted archive (.ze.enc) |
| `export --all` | Export all appliances to single encrypted archive |
| `import [--force] [--dir <path>] <archive>` | Import appliance from encrypted archive |


### ISO prerequisites

The ISO build requires an installer kernel, an initrd, `grub-mkstandalone`, and
`xorriso`. Use `ze appliance iso --check` to see what is ready and what is
missing. The `kernel` and `initrd` commands handle downloading or building these
artifacts automatically:

    bin/ze-setup appliance iso --check               # report readiness
    bin/ze-setup appliance kernel lab                 # reads arch/profile from appliance config
    bin/ze-setup appliance kernel --profile hardware lab   # explicit hardware profile
    bin/ze-setup appliance kernel --builder qemu --arch arm64 lab
    bin/ze-setup appliance kernel --target runtime    # build the verified runtime kernel tree
    bin/ze-setup appliance initrd                    # download or build initrd
    bin/ze-setup appliance iso lab                   # build ISO

`ze appliance kernel` defaults to `--target installer` (the monolithic PXE
`Image`); `--target runtime` builds the gokrazy runtime kernel tree (modules +
`vmlinuz`) from `gokrazy/kernel/` with the runtime requirement floor enforced.
The command reports the target it built (`kernel ready: ... (target=installer,
profile=qemu, version=7.1.4)`). The installer target tries cache first, then a
configured prebuilt-artifact URL if `ze.appliance.kernel.url` is set, then local
build. Every local build runs through the shared driver
`tools/kernel-builder/run.py`, which selects Docker when available and falls back
to QEMU; use `--builder docker` or `--builder qemu` to force one path. Resolved
installer artifacts are cached under `$XDG_CACHE_HOME/ze/` (default
`~/.cache/ze/`) and copied to `build/kernel/` when appropriate.
<!-- source: internal/appliance/cmd_kernel.go -- runKernel, kernelTargetFor -->
<!-- source: tools/kernel-builder/run.py -- main -->

`ze appliance initrd` uses the same cache, optional configured URL, then local
build pattern for the initrd artifact. The download URL has no built-in release
server default; set `ze.appliance.kernel.url` or `ze.appliance.initrd.url` to use
prebuilt artifacts.
<!-- source: internal/appliance/cmd_initrd.go -- resolveInitrd -->

`ze doctor` includes checks for kernel, initrd, grub, xorriso, and e2fsprogs
availability, reporting warnings with actionable hints when prerequisites are
missing.
<!-- source: internal/appliance/doctor_checks.go -- applianceDoctorChecks -->

### ISO installer media

Create an installer ISO from an image already produced by `ze appliance
build`. By default the command selects the latest `ze-*.img` in the appliance
directory, verifies its `.sha256` sidecar, and writes `ze-*.iso` next to the
image. Use `--image` to select a specific image filename and `--output` to write
the ISO elsewhere. The output path must not overwrite the selected `.img`, and
the image filename must stay within `[A-Za-z0-9._-]` so the initrd can pass it
on the kernel command line. By default, `ze appliance iso` resolves a matching
installer kernel from the cache, or from `build/kernel/Image` only when its
variant metadata matches the appliance arch/profile/version. `ze appliance
kernel` and `make -C tools/installer-kernel` both delegate to
`tools/kernel-builder/`; pass `--kernel` to keep multiple kernels side by side.

The `make -C tools/installer-kernel` build is incremental. It rebuilds when a
config fragment, a builder file, the Makefile, or
`internal/appliance/kernel.version` is newer than `build/kernel/Image`. It also
rebuilds when the requested arch, profile, or builder is different from the last
build, which it records in `build/kernel/.request`. A repeated build with the
same request does no work. `ze appliance kernel` is keyed on the cache instead.
It deletes `build/kernel/.request` on every installer-target run, so the next
`make` rebuilds rather than trust a record it did not write.
<!-- source: internal/appliance/cmd_iso.go -- runIso, resolveISOInput, readRequiredImageChecksum -->
<!-- source: tools/installer-kernel/Makefile -- all -->

    bin/ze-setup appliance build lab
    bin/ze-setup appliance kernel --builder docker --profile hardware lab
    bin/ze-setup appliance iso lab
    bin/ze-setup appliance iso --image ze-20260601-120000.img lab
    bin/ze-setup appliance iso --output /path/to/lab.iso lab
    bin/ze-setup appliance iso --kernel build/kernel/Image lab

The ISO is an installer envelope around the existing raw gokrazy image. The image
is gzip-compressed inside the ISO to reduce media size (a 2 GiB image with ~100
MiB of content compresses to roughly 100 MiB). The installer initrd decompresses
the image during installation. The ISO does not rebuild the appliance, regenerate
credentials, fetch a separate ZeFS database, or mutate `/perm` after writing the
disk image. The installed disk receives the selected image bytes, including the
`/perm/ze/database.zefs` and `/perm/ze/build.json` manifest that `build` already
injected.
<!-- source: internal/appliance/cmd_iso.go -- stageISO -->
<!-- source: internal/install/disk/run.go -- runISO -->
<!-- source: internal/appliance/cmd_build.go -- injectZeFS -->

The ISO boot path accepts an optional explicit target disk. If no target is set,
the installer writes only when exactly one non-removable candidate disk remains
after excluding the ISO source media. The initrd also matches the booted ISO by
a builder-generated `ze.media-id` token before it trusts a mounted installer
volume, so identical image filenames on multiple attached installer media do not
confuse the source selection. With multiple fixed disks, pass a whole disk path
such as `/dev/vda` at ISO creation time:
<!-- source: internal/install/disk/detect.go -- findTargetDisk; internal/install/disk/iso.go -- media-id match -->

    bin/ze-setup appliance iso --target /dev/vda lab

After the installer writes the disk in ISO mode, it powers off instead of
rebooting. Remove the installer media, then power the target back on so the
firmware boots from the written disk.
<!-- source: internal/install/disk/run.go -- runISO -->

The ISO contains the full provisioned appliance image, including the embedded
ZeFS database. Handle the ISO with the same care as the `.img` file.
<!-- source: internal/appliance/cmd_iso.go -- stageISO -->

**USB write method:** the ISO can be written with `dd`, Etcher, or Rufus in DD
mode. Ventoy is also supported when the installer kernel includes loop device
and FAT/exFAT filesystem support (the `hardware` kernel profile has this). The
initrd detects the ISO file on the Ventoy data partition, loop-mounts it, and
proceeds with the installation. When using the `qemu` kernel profile, Ventoy
is not supported.
<!-- source: internal/install/disk/run.go -- runISO Ventoy fallback -->
<!-- source: internal/install/disk/iso.go -- tryVentoyISO -->
<!-- source: tools/installer-kernel/hardware.config -- Ventoy-capable profile -->

### Remote operations (push, config-push)

Push a built image to a running gokrazy device via its HTTPS update endpoint:

    bin/ze-setup appliance push lab
    bin/ze-setup appliance push --image ze-20260427-143022.img lab   # rollback to older image
    bin/ze-setup appliance push --all                                # all devices with address
    bin/ze-setup appliance push --all --parallel 4                   # 4 concurrent uploads

Push uses the update token (from `secrets/update.token`) for HTTP basic auth, and verifies the device TLS certificate against the stored `cert.pem`. No system CA pool is consulted.
<!-- source: internal/appliance/cmd_push.go -- loadDeviceTLS, authTransport -->

When `--image` is set, the file name must resolve to a regular file inside the
appliance directory. Path traversal and symlinks escaping that directory are
rejected after the local update token is read, but before TLS setup or any
network request starts.
<!-- source: internal/appliance/cmd_push.go -- resolveImagePath -->

Preview the effective configuration (base + overlay merged) without building:

    bin/ze-setup appliance config lab --merged

Push a config change to a running device without rebuilding the image:

    bin/ze-setup appliance config-push lab
    bin/ze-setup appliance config-push --dry-run lab    # preview only, no SSH connection
    bin/ze-setup appliance config-push --all            # all addressed devices
    bin/ze-setup appliance config-push --all --parallel 4

Config-push uses SSH (operator's key via ssh-agent) to upload the merged config to the device, which validates and applies it. No secrets are transmitted over SSH.
<!-- source: internal/appliance/cmd_config_push.go -- configPushOne -->

### Device-side config behavior

At boot, unmanaged devices resolve the active config in ZeFS. If no active
config exists, Ze bootstraps one from the seed template or interface discovery.
If `/perm/ze/config-pushed.conf` exists and parses as Ze config, Ze writes it
over the active config. If the pushed config fails validation, Ze deletes that
pushed file and continues with the existing active config.

| Stage | Source | Location |
|-------|--------|----------|
| 1 | Existing or bootstrapped active config | `file/active/ze.conf` in ZeFS |
| 2 | Seed template, only when active config is missing | `file/template/ze.conf` in ZeFS |
| 3 | Valid pushed config, applied over active config | `/perm/ze/config-pushed.conf` |

After loading the effective config, the device writes its SHA-256 hash to
`/perm/ze/config-active-hash` for fleet drift detection.
<!-- source: cmd/ze/ze_core_start.go -- cmdStart config bootstrap order -->
<!-- source: cmd/ze/pushed_config.go -- checkPushedConfig, writeConfigActiveHash -->

**Last-known-good hash:** at build time, `ze appliance build` writes the SHA-256
of the assembled seed config to `meta/config/last-known-good` in ZeFS. This
serves as the build-time integrity baseline.
<!-- source: internal/appliance/cmd_assemble.go -- assembleZeFS last-known-good -->

**Config push and health monitor:** `config-push` connects over SSH, stages the
merged config, validates it, and applies it on the device. The source-backed
health monitor is armed when a pushed config file is consumed at boot; it watches
BGP peer close events for 30 seconds. If a peer closes during that window, the
device reverts to the previous config, or to the seed config if no previous
config was saved. If the window completes, the active config hash is written to
`/perm/ze/last-known-good-pushed`.
<!-- source: internal/appliance/cmd_config_push.go -- configPushOne -->
<!-- source: cmd/ze/health_revert.go -- HealthRevert -->

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

Use `"password": "generate"` for per-device random passwords (printed to stdout once, never stored in plaintext). When an encryption passphrase is set, each encrypted secret write receives a fresh random salt and nonce.

### Disaster recovery (export/import)

Export creates an encrypted archive of an appliance directory for offsite backup or bastion migration. Archives include config, secrets, and build metadata, but exclude images and ZeFS databases (both are rebuildable).
<!-- source: internal/appliance/cmd_export.go -- tarApplianceInto, shouldExcludeFromExport -->

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

Archives are always encrypted using the same Argon2id + XChaCha20-Poly1305 scheme as secrets at rest. The archive passphrase can differ when provided separately from the secrets passphrase. `import --force` overwrites files present in the archive, but it does not delete extra files already present in existing appliance directories.
<!-- source: internal/appliance/crypto.go -- Encrypt, Decrypt -->
<!-- source: internal/appliance/cmd_import.go -- importArchive, extractTar -->
