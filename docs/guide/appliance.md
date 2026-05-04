# VM Appliance

Build a bootable VM image with Ze baked in using [gokrazy](https://gokrazy.org). The default target is x86_64; set `GOKRAZY_ARCH=arm64` for a native Apple Silicon QEMU image. The result is a minimal Linux system that runs Ze as its only application, with no package manager, no shell (except emergency serial console), and automatic process supervision.

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
3. Cross-compiles Ze for linux/`GOKRAZY_ARCH` and builds a 2GB disk image
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
GOARCH=amd64 bin/gok --parent_dir gokrazy -i ze update
```

This rebuilds and pushes the new root filesystem without touching `/perm`. The system reboots into the new version. If the update fails mid-flight, the previous root partition is still intact.

For full image rebuilds (when you also want to update the kernel or partition layout), use `make ze-gokrazy USER=admin PASS=secret` again and re-flash.

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
