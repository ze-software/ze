# Installation

Ze provides commands for local installation, remote PXE provisioning, and appliance ISO installer media.

## Local Installation

`ze install local` copies the ze binary to a standard system location
and creates the config directory.

### Quick Start

```bash
sudo ze install local
```

This presents an interactive menu to select the installation prefix:

```
Select installation prefix:
  1) /usr/local  (recommended)
  2) /usr  (system)
  3) /opt/ze  (self-contained)
Choice [1]:
```

Use `--prefix` for non-interactive use:

```bash
sudo ze install local --prefix /usr/local
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--prefix` | interactive | Installation prefix (binary goes to `<prefix>/bin/ze`) |
| `--dry-run` | | Print what would be done without making changes |

### What It Does

1. Copies the running ze binary to `<prefix>/bin/ze`
2. If no `database.zefs` exists at the config path: creates the config directory

The config directory is resolved from the binary path following GNU prefix
conventions:

| Binary location | Config directory |
|-----------------|-----------------|
| `/usr/local/bin/ze` | `/etc/ze` |
| `/usr/bin/ze` | `/etc/ze` |
| `/opt/ze/bin/ze` | `/opt/ze/etc/ze` |

After installation, run `ze init` to bootstrap the database.

### Systemd Service

After installing the binary and bootstrapping the database, use
`ze install systemd` to set up the systemd service:

```bash
sudo ze install local --prefix /usr/local
sudo ze init
sudo ze install systemd --start
```

The generated service-management unit file:

```ini
[Unit]
Description=Ze Network OS
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=ze
Group=ze
ExecStart=<prefix>/bin/ze start
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
LimitCORE=infinity
LimitMEMLOCK=infinity
WorkingDirectory=<config-dir>
Environment=ZE_CONFIG_DIR=<config-dir>
Environment=XDG_RUNTIME_DIR=/run/ze
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ProtectSystem=true
ProtectHome=true
RuntimeDirectory=ze

[Install]
WantedBy=multi-user.target
```

`ze install systemd` refuses to run unless `<config-dir>/database.zefs` exists.
It creates the `ze` user and group if missing, changes ownership of the config
directory and `database.zefs` to `ze:ze`, writes `/etc/systemd/system/ze.service`,
runs `systemctl daemon-reload`, and enables the service. Use `--dry-run` to
print the unit file without root or systemd, `--config <dir>` to override the
config directory in the unit, `--force` to overwrite an existing unit, and
`--start` to start the service after enabling it.

The systemd unit sets `XDG_RUNTIME_DIR=/run/ze`, so the daemon socket is
`/run/ze/ze.socket`. For local operator CLI access, configure
`daemon { socket "/run/ze/ze.socket"; }` or export `XDG_RUNTIME_DIR=/run/ze`.

## Uninstalling

`ze uninstall systemd` removes the systemd service; `ze uninstall local`
removes the binary and optionally the config directory. Always remove the
service before the binary, so nothing is left running (or trying to restart)
a binary that is gone.

```bash
ze uninstall systemd            # stop, disable, and remove the systemd unit
ze uninstall systemd --purge    # also remove the ze user and group
ze uninstall local              # remove binary
ze uninstall local --purge      # also remove config directory and database
ze uninstall local --dry-run    # preview what would be removed
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--prefix` | detect from running binary | Installation prefix |
| `--purge` | | Also remove config directory and database |
| `--dry-run` | | Print what would be done without making changes |
| `--yes` | | Skip confirmation prompt |

Without `--yes`, uninstall shows what will be removed and asks for
confirmation before proceeding.

To check the service status, use `systemctl status ze.service` directly.

## Installing on Real Hardware (End to End)

This bare-metal PXE walkthrough follows the same chain as
`./le qemu install-test`: build an image, serve it, boot an installer kernel
and initrd, write the disk, then log in over SSH. The reference sections below
describe each piece.

### 1. Build the disk image

Use the structured appliance builder (full reference:
[appliance guide](appliance.md), "ze appliance"):

```bash
ze appliance init prod
# For real hardware: set image.kernel-profile to "hardware" and
# image.arch to match the target CPU in appliance.json before build.
ze appliance build prod
```

This produces `~/.config/ze/appliances/prod/ze-<timestamp>.img` with TLS, SSH
credentials, and a seed config baked into its `/perm` zefs. Match
`image.arch` in `appliance.json` to the target CPU.

### 2. Build the installer kernel and initrd

The target PXE-boots a kernel + initrd, *not* the disk image. Build both for the
target architecture:

```bash
ze appliance kernel prod                     # reads arch + profile from appliance.json
ze appliance initrd                          # build/initrd/initrd.img.gz (pure Go)
```

Choose the profile and architecture through the native command:

```bash
ze appliance kernel --profile hardware --arch amd64 prod
ze appliance kernel --profile hardware --arch arm64 prod
ze appliance initrd
```

The `PROFILE` selects the driver set: `qemu` (default, virtio only) or
`hardware` (EFI stub, framebuffer, Intel/Realtek/Broadcom/Mellanox NICs,
AHCI, NVMe). A stock distro kernel will **not** boot the module-free initrd;
see [Installer Kernel](#installer-kernel).

### 3. Start the provisioning server

On a ze device on the (isolated) provisioning network:
<!-- source: internal/plugins/provision/main.go -- Run -->

```bash
sudo ze install remote \
  --interface eth0 \
  --network 192.168.50.0/24 \
  --image ~/.config/ze/appliances/prod/ze-<timestamp>.img \
  --kernel build/kernel/Image \
  --initrd build/initrd/initrd.img.gz \
  --ssh-username admin \
  --ssh-password 'choose-a-strong-one'
```

`--kernel` and `--initrd` copy the installer files to `build/pxe/boot/`.
Stock iPXE binaries from `tools/ipxe-binaries/` are copied to
`build/pxe/tftp/` if not already present. If the files are already
staged from a previous run, omit `--kernel` and `--initrd`.
<!-- source: internal/plugins/provision/staging.go -- stageArtifacts -->

This runs DHCP (with PXE options and iPXE chainloading), TFTP (bootloaders),
and the HTTP image server on `eth0`. It serves the image at
`/install/image/<filename>`, a dynamically generated iPXE boot script at
`/install/boot/boot.ipxe`, and a credential `database.zefs` (generated from
`--ssh-*`, password stored hashed) at `/install/database.zefs`.
See [Remote Provisioning (PXE)](#remote-provisioning-pxe).
<!-- source: internal/plugins/imageserver/handler.go -- serveBootIPXE -->

### 4. Net-boot the target

Set the target firmware to network boot. It then:

1. DHCPs an address and TFTP bootfile, chainloads iPXE.
2. iPXE sends a second DHCP request; the server detects iPXE via option 77 and
   responds with the HTTP boot script URL instead of the TFTP bootfile.
3. iPXE fetches `boot.ipxe` from the image server, which contains the kernel
   command line with `ze.server`, `ze.image`, and `ip=dhcp`.
4. iPXE loads the installer kernel + initrd via HTTP and boots.
5. The initrd downloads the image and `database.zefs`, writes the first fixed
   disk (`/dev/sda`, `/dev/nvme0n1`, `/dev/mmcblk0` ...; removable, virtual,
   optical and `mtdblock` flash devices are skipped), and reboots.
4. The target boots ze in [bootstrap mode](#bootstrap-mode) and starts SSH.

### 5. Log in and configure

```bash
ssh admin@<target-ip>      # the password given to --ssh-password
```

ze's SSH endpoint is the network-OS CLI. Configure with `ze config edit` and
commit; the committed config replaces the bootstrap config on the next restart.


### Troubleshooting

- **Installer drops to a shell** instead of rebooting on a bad `ze.server`, no
  writable disk, or a download that fails 3 times. Read the serial/VGA console
  for the `[ze-install] FATAL:` line.
- **No disk chosen**: with more than one non-removable disk and no `ze.target`,
  the installer stops and names the candidates rather than picking one. Pass
  `ze.target=/dev/vda` in the cmdline, or detach the extra fixed disks.
- **Download stalls / non-standard port**: confirm the image server's port and
  pass `ze.port=` in the iPXE cmdline.
- **Dry-run first**: `./le qemu install-test` reproduces the whole chain and
  reports a broken image, kernel, or initrd before hardware is touched.

## Appliance ISO Install

For appliances built with `ze appliance build`, ISO media is an offline
install transport for the gokrazy image. The image is gzip-compressed inside the
ISO; the installer initrd decompresses it during installation. Create it with:
<!-- source: internal/appliance/cmd_iso.go -- runIso -->

```bash
ze appliance build prod
ze appliance kernel --profile hardware prod   # download or build installer kernel (reads arch from config)
ze appliance initrd                           # download or build installer initrd
ze appliance iso prod
```

Use `ze appliance iso --check` to verify all prerequisites (kernel, initrd,
grub, xorriso) are available before building. The `kernel` and `initrd` commands
download pre-built artifacts when configured, then fall back to the native
local QEMU or Docker kernel builder and the Go initrd builder. Cached artifacts
are stored under `$XDG_CACHE_HOME/ze/`.

For arm64 targets (the kernel version is single-sourced in
`internal/appliance/kernel.version`; `--version` only overrides it and must be a
kernel >= 7):

```bash
ze appliance kernel --arch arm64
ze appliance iso --kernel build/kernel/Image prod
```

The ISO installer decompresses and writes the embedded image to the target disk.
Unlike PXE provisioning, it does not download `/install/database.zefs` or write a
separate database after the disk image, because the appliance build already
injected `/perm/ze/database.zefs` into the image.
<!-- source: internal/install/disk/run.go -- runISO -->
<!-- source: internal/appliance/cmd_build.go -- injectZeFS -->

The ISO bootloader target follows `image.arch`: amd64 images produce
`BOOTX64.EFI`, arm64 images produce `BOOTAA64.EFI`. By default the command
checks for a cached kernel under `$XDG_CACHE_HOME/ze/installer-kernel/` then
falls back to `build/kernel/Image`. Pass `--kernel` to use a
specific kernel path.
<!-- source: internal/appliance/cmd_iso.go -- isoGRUBTarget, defaultISOKernelPath -->

If the target has more than one fixed disk, create the ISO with an explicit
whole-disk target. The initrd rejects ambiguous implicit disk selection in ISO
mode, excludes the ISO source media from target candidates, and requires a
builder-generated media id match before it trusts a mounted installer volume.
<!-- source: internal/install/disk/detect.go -- findTargetDisk; internal/install/disk/iso.go -- find ISO media -->

```bash
ze appliance iso --target /dev/vda prod
```

The generated ISO includes the installer kernel, initrd, the selected image,
its checksum, and metadata. It contains the full provisioned appliance image, so
handle it like the `.img` artifact.

**USB write method:** the ISO can be written with `dd`, Etcher, or Rufus in DD
mode. Ventoy is also supported when the installer kernel includes loop device
and FAT/exFAT filesystem support (the `hardware` kernel profile has this). The
initrd detects the ISO file on the Ventoy data partition, loop-mounts it, and
proceeds with the installation. When using the `qemu` kernel profile, Ventoy
is not supported.

ISO installs power off after the disk write so the removable installer media can
be removed before the next boot. They do not auto-reboot while the ISO is still
present.
<!-- source: internal/install/disk/run.go -- runISO -->

<!-- source: internal/appliance/cmd_iso.go -- stageISO -->

## Remote Provisioning (PXE)

`ze install remote` is a one-command provisioning server that PXE-boots
target machines with a gokrazy image containing ze.

> **Warning: the provisioning network MUST be isolated.** Run `ze install
> remote` only on a dedicated segment with no other DHCP server. On a shared
> network a foreign DHCP server (for example a corporate one) can win the
> boot-time race and hand the target a lease with no route back to `ze.server`;
> the installer then probes an unreachable server until `ze.wait` expires and
> drops to a debug shell, and the disk is never written. The initrd now
> re-validates the kernel lease and recovers when a reachable interface exists
> (see [Kernel Command Line](#kernel-command-line)), but a second DHCP server on
> the same L2 segment is unsupported and can still break provisioning.

### How It Works

1. The operator runs `ze install remote` on an existing ze device connected
   to the provisioning network.
2. Ze generates a config enabling DHCP (with PXE extensions), TFTP, and
   an HTTP image server, then forks itself (`ze -`) with the config piped
   to stdin.
3. A target machine PXE-boots: DHCP assigns an IP and directs it to the
   TFTP bootloader, which chain-loads the installer kernel and initrd
   via HTTP.
4. The installer writes the gokrazy image to disk and reboots.
5. The target boots into ze in bootstrap mode: discovers all interfaces,
   enables DHCP client on each ethernet NIC, and starts SSH for operator
   access.

### Quick Start

```bash
ze install remote \
  --interface eth0 \
  --network 192.168.1.0/24 \
  --image /path/to/gokrazy.img \
  --ssh-username admin \
  --ssh-password changeme
```

This starts three servers on `eth0`:

| Protocol | Port | Purpose |
|----------|------|---------|
| DHCP | 67/udp | IP assignment with PXE options (bootfile, next-server) |
| TFTP | 69/udp | Bootloader delivery (iPXE for BIOS/UEFI) |
| HTTP | 80/tcp | Disk image and boot file serving |

The server IP is resolved from the interface's first IPv4 address.
If the interface has no IPv4 address, the address from `--network` is
added via netlink and removed on exit (if `--network` is a network
address like `192.168.1.0/24`, the first host `192.168.1.1` is used).
Use `--address` to override.

### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--interface` | Yes | | Network interface to bind all servers |
| `--network` | Yes | | Provisioning subnet CIDR (/8 to /30) |
| `--image` | Yes | | Path to gokrazy disk image file |
| `--ssh-username` | Yes | | Admin username for the installed target |
| `--ssh-password` | Yes | | Admin password (bcrypt-hashed before embedding) |
| `--address` | No | First IPv4 on interface (or from `--network` if none) | Server IP override |
| `--kernel` | No | | Path to installer kernel (copied to boot directory) |
| `--initrd` | No | | Path to installer initrd (copied to boot directory) |
| `--pxe-dir` | No | `build/pxe` | PXE serve root: boot files under `<dir>/boot`, TFTP under `<dir>/tftp` |

### DHCP Pool

The DHCP pool range scales with the subnet size. The server IP is
excluded from the pool. Examples:

| Network | Server | Pool Start | Pool Stop |
|---------|--------|------------|-----------|
| 10.0.0.0/24 | 10.0.0.1 | 10.0.0.2 | 10.0.0.254 |
| 192.168.1.0/28 | 192.168.1.1 | 192.168.1.2 | 192.168.1.14 |
| 10.1.1.0/30 | 10.1.1.1 | 10.1.1.2 | 10.1.1.2 |

### PXE Boot

The DHCP server detects PXE clients via option 60 (`PXEClient:`) and
reads option 93 (client architecture) to select the bootfile:
<!-- source: internal/plugins/dhcpserver/handler.go -- appendPXEOptions -->

| Architecture | Bootfile |
|-------------|----------|
| UEFI x64 (type 7) | `ipxe.efi` |
| Everything else, including BIOS (type 0) and UEFI types 6 and 9 | `ipxe.pxe` |

When the PXE client is iPXE (detected via option 77 user-class prefix "iPXE")
and `boot-script-url` is configured, the DHCP server sends the HTTP boot script
URL as the bootfile instead of the TFTP binary. This two-stage chainload
(firmware -> iPXE via TFTP -> boot.ipxe via HTTP) eliminates the need for
custom-embedded iPXE builds.
<!-- source: internal/plugins/dhcpserver/handler.go -- isIPXE -->

The image server generates `boot.ipxe` dynamically at `/install/boot/boot.ipxe`
with the correct `ze.server`, `ze.image` (lexicographically last `.img` file),
and `ze.port` (when not 80). A static `boot.ipxe` file in the boot directory
takes precedence over dynamic generation for operator customization.
<!-- source: internal/plugins/imageserver/handler.go -- serveBootIPXE -->

Bootfiles are served from `build/pxe/tftp/` via TFTP.
The installer kernel and initrd are served from `build/pxe/boot/`
via HTTP. Stock iPXE binaries are bundled in `tools/ipxe-binaries/` and
staged automatically by `ze install remote`.

The default `--pxe-dir build/pxe` is relative to the working directory. Run
`ze install remote` from the repository root, or pass an absolute
`--pxe-dir`. The command stages bundled iPXE files and the explicit
`--kernel` and `--initrd` artifacts under that root.

### SSH Credentials

The `--ssh-password` is bcrypt-hashed (cost 10) before being embedded
in the generated config as `ssh-password-hash`. The plaintext password
is never written to disk or config. It is visible in the process listing
(`ps aux`) while `ze install remote` is running.

### Generated Config

`ze install remote` generates a standard ze config in brace format and
pipes it to a child `ze` process. The generated config enables three
plugins:

- **dhcpserver** with PXE options, a shared-network, and the computed pool
- **tftpserver** bound to the provisioning interface
- **imageserver** bound to the provisioning interface with SSH credentials

The same provisioning setup can be achieved by writing the config
manually and running `ze <config-file>`.

### Watching Provisioning Activity

`ze install remote` runs the install servers at `info` log level by default, so the
terminal shows the live boot sequence with no extra flags:

- **dhcpserver** logs each lease: `dhcpserver: lease request=DISCOVER reply=OFFER mac=… ip=… bootfile=…`
- **tftpserver** logs each bootloader fetch: `tftpserver: read request file=ipxe.efi client=…`
- **imageserver** logs each HTTP request (`boot.ipxe`, `vmlinuz`, `initrd.img.gz`, the
  disk image, `database.zefs`) and, at startup, the image it will serve with its build
  identity: `imageserver: image to install image=ze-<ts>.img built=<ts> ze-version=… sha256=…`

Set `ze.log` to change verbosity (`ze.log=debug`, `ze.log=warn`); an explicit `ze.log`
or per-subsystem `ze.log.{dhcpserver,tftpserver,imageserver}` overrides the info default.
<!-- source: internal/plugins/provision/main.go -- withInstallLogDefaults -->

If you see the iPXE downloads (`boot.ipxe`, `vmlinuz`, `initrd.img.gz`) but never an
`/install/image/<name>` request, the target booted the kernel but its initrd could not
reach the server to pull the image (commonly a foreign DHCP lease on a non-isolated
network) and the install did not complete.

### Confirming Which Image Is Installed

A failed PXE install never writes the disk, so the target reboots into whatever was on
it before (for example a previous ISO install) and can look unchanged. To confirm the
build actually running, `ze appliance build` bakes a manifest into the image at
`/perm/ze/build.json`, which `ze version` reports:

```
ze 26.06.17 (built …)
  …
  image:    ze-20260617-160000.img (built 20260617-160000)
  appliance: prod
```

The baked manifest omits the image checksum (it would be self-referential, since
baking changes the image); the full `sha256` is in the external `build.json` next
to the image and in the provisioning server's `imageserver: image to install` log.
Compare the `image:`/`built` line on the target with that server log line to verify
the latest image installed.
<!-- source: internal/core/version/version.go -- Extended, ReadManifestFile -->
<!-- source: internal/appliance/cmd_build.go -- injectZeFS baked manifest -->

### Requirements

- An **isolated** provisioning network: no second DHCP server on the L2
  segment, or a foreign lease can leave the target with no route to `ze.server`
- Root privileges (DHCP, TFTP, and HTTP bind to privileged ports;
  interface auto-configuration uses netlink)
- Disk image at the path specified by `--image`
- Installer kernel and initrd: pass `--kernel` and `--initrd` on first run,
  or pre-stage files in `build/pxe/boot/`
- iPXE binaries: bundled in `tools/ipxe-binaries/`, auto-staged to
  `build/pxe/tftp/` if not present

### Shutdown

SIGTERM or SIGINT sent to `ze install remote` is forwarded to the child
ze process. The child shuts down cleanly (closes listeners, drains
connections). Closing the parent also sends EOF on the stdin pipe,
which ze treats as a shutdown signal.

## Bootstrap Mode

When ze starts with a zefs database but no config file and no template,
it enters bootstrap mode automatically. This is the expected state after
a PXE-provisioned device boots for the first time.

### What Happens

1. Ze detects no config in zefs (no `file/active/ze.conf`, no `file/template/ze.conf`).
2. Interface discovery enumerates all OS network interfaces.
3. A minimal config is generated: DHCP client enabled on every ethernet
   interface, SSH server enabled.
4. Ze starts with this config. DHCP clients acquire addresses, SSH becomes
   reachable.

### Operator Workflow

1. SSH into the device using the credentials pre-provisioned by `ze install remote`.
2. Configure ze via the CLI (`ze config edit`, then commit).
3. The committed config replaces the bootstrap config. On the next restart,
   ze starts in normal mode.

### Constraints

- Only ethernet interfaces get DHCP. Bridge, veth, dummy, loopback,
  wireguard, and xfrm interfaces are skipped.
- SSH credentials come from zefs (written by the installer initrd), not
  from the generated config.
- Bootstrap mode is only intended for trusted/provisioning networks.
  SSH is enabled on all interfaces.
- If no ethernet interfaces are found (or the netlink backend is not
  available), bootstrap mode does not activate and ze falls through to
  the next startup path (error; use `--web-only` for standalone web UI).

### Limitations

- Single image: all targets receive the same gokrazy image
- No per-MAC image selection (future)
- No post-install hooks (future)
- Assumes an isolated provisioning network (no proxy DHCP)

## Installer Initrd

The installer initrd is a minimal Linux image that performs the actual
disk write on target hardware. It is the final step in the PXE chain:
the bootloader fetches the kernel and initrd via HTTP, the kernel boots,
and the initrd's init script installs ze.

### What It Does

1. Parses `ze.source`, `ze.server`, `ze.port`, `ze.image`, `ze.target`, and
   `ze.media-id` from the kernel command line
2. In HTTP mode, downloads the gokrazy disk image from
   `http://<server>:<port>/install/image/<name>`
3. In ISO mode, mounts local ISO media read-only and selects the embedded
   compressed image
4. Writes the image to the selected non-removable block device (decompressing
   in ISO mode)
5. In HTTP mode only, re-reads the partition table, mounts partition 4 (ext4,
   `/perm`), downloads `database.zefs`, and writes it to `/perm/ze/database.zefs`
6. In HTTP mode, reboots. In ISO mode, powers off so the operator can remove
   the installer media before the next boot.

### Building the Initrd

Prerequisites: the Go toolchain only. The initrd is built in pure Go, so no
`busybox`, `cpio`, or `gzip` host tools are required.

```bash
ze appliance initrd
```

This cross-compiles `cmd/ze-installer` for the target architecture and packs the
single static binary into `build/initrd/initrd.img.gz` (the `/init` entry of a
pure-Go newc cpio written through `compress/gzip`). Copy it
alongside a Linux kernel to the boot directory served by the image
server (`build/pxe/boot/`).

### Kernel Command Line

The bootloader sets these parameters:

| Parameter | Required | Default | Purpose |
|-----------|----------|---------|---------|
| `ze.source` | No | `http` | Source mode: `http` for PXE or `iso` for local ISO media |
| `ze.server` | HTTP only | | IPv4 address of the ze-install server |
| `ze.port` | No | `80` | TCP port of the install HTTP server (1-65535) |
| `ze.image` | No | `ze.img` | Name of the disk image to install |
| `ze.target` | No | | Explicit whole-disk target such as `/dev/vda` |
| `ze.wait` | No | `30` | Max server probe attempts before giving up (0 = skip probe) |
| `ze.media-id` | ISO only | | Builder-generated 32-hex token that identifies the booted installer ISO |
| `ip=dhcp` | HTTP only | | Kernel-level network configuration (with userspace fallback) |

`ze.port` exists for install servers that cannot bind the privileged port 80
(for example an unprivileged HTTP server, or a QEMU test harness that serves on
an ephemeral port). ISO mode does not use `ze.server`, `ze.port`, or `ip=dhcp`.

On some hardware (e.g. Intel I226-V) the kernel `ip=dhcp` autoconfiguration
races against NIC carrier detection and times out before the link comes up; on a
non-isolated network it can also win a lease from a foreign DHCP server (for
example a corporate one) whose default route cannot reach `ze.server`. The initrd
guards against both. It trusts the kernel-provided default route only when
`ze.server` actually answers an HTTP probe; if there is no default route, or the
route present cannot reach the server, it brings up all non-loopback interfaces,
waits up to 10 seconds for carrier, and runs an in-process DHCP client
(`nclient4`) on each interface,
verifying the server is reachable before accepting the lease. Interfaces that
obtain a lease but cannot route to `ze.server:ze.port` are flushed and skipped.
If no interface produces a working route, the installer drops to the recovery console.
(A foreign DHCP server on a shared segment can still defeat this if it also wins
the per-interface lease race, which is why the provisioning network must be
isolated.) Even after the network is configured, the install server may not be
reachable yet (switch STP port transitions can block traffic for 30-50 seconds
after a reboot). The initrd probes the server with a 2-second timeout, retrying
up to 30 times. Each probe distinguishes "TCP unreachable" from "HTTP error
response" so a server that returns 404 is still considered reachable. Interface
state and routing tables are logged every 10 attempts for diagnostics.
<!-- source: internal/install/disk/network.go -- ensureNetwork -->

The installer fans its progress and `FATAL` lines to every console listed in
`/sys/class/tty/console/active`, not just the single `/dev/console` the kernel
marks preferred. On a headless box the preferred console can be a dead VGA tty
while kernel messages still reach the serial line, which would otherwise hide all
installer output. The generated `boot.ipxe` also selects the console set per
client architecture via iPXE's `${buildarch}`: x86 clients get `console=tty0
console=ttyS0`, while arm64 also keeps `console=ttyAMA0` (the ARM PL011 UART,
which never registers on x86 and can dead-end `/dev/console` if left on an x86
cmdline).
<!-- source: internal/install/disk/console_linux.go -- setupConsoles; internal/plugins/imageserver/handler.go -- serveBootIPXE -->
Existing `ze install remote` deployments need no `ze.source` change because the
default source is `http`.
<!-- source: internal/install/disk/cmdline.go -- parseCmdline -->

The installer selects a non-removable block device via sysfs. Virtual devices
(loop, ram, dm, zram, md), optical drives (sr), floppies (fd), and firmware/CFI
flash (`mtdblock`, the QEMU `virt` machine's pflash) are skipped.

The installer refuses to choose implicitly when more than one fixed candidate
remains, in both HTTP and ISO mode, and names the candidates it found. ISO mode
also excludes the ISO source media before it counts them. Use
`ze.target=/dev/vda` to name an explicit whole disk.

Supported target disk forms include `/dev/sda` (SATA/SCSI), `/dev/vda`
(virtio-blk, used by QEMU/KVM), `/dev/nvme0n1` (NVMe), and `/dev/mmcblk0`
(eMMC).
<!-- source: internal/install/disk/detect.go -- findTargetDisk -->
### Error Handling

If the installer encounters an error (missing server IP, no disk found,
download failure), it does not silently reboot. It opens a Go recovery console
on every active console (preferring a serial console, since headless installs
are driven over serial) so the operator can diagnose network or hardware issues.
The recovery console is a fixed menu (retry network, diagnostics, reboot, power
off), not a shell. Its policy has three branches: with `ze.rescue-auth` set the
console is gated on a rescue token; with no credential on ISO media it opens
ungated (the operator is physically present); with no credential on a network
install it prints the error, waits ~30 seconds, and reboots, so an unattended
box never hangs waiting for a credential nobody can supply.
<!-- source: internal/install/disk/rescue_linux.go -- fatalInitrd, rescueOnConsoles -->

`ze plugin provision` mints the rescue token, prints it once, and writes only
its salted argon2id digest into the generated config as `rescue-auth`. The image
server puts that digest on the installer kernel cmdline as `ze.rescue-auth`, and
the installer derives the typed token under the same salt to compare. Record the
token when provisioning: it is not recoverable afterwards, and without it a
failed network install offers no shell.

The credential is deliberately a dedicated token rather than the admin password.
The iPXE script carrying it is served unauthenticated over plain HTTP on the
provisioning network, so the digest is a public value; committing it to the
admin password would put that password within reach of anyone on that network.

Verifying a typed token costs **64 MiB of RAM** for the duration of the
derivation (measured: 67,120,208 bytes, the configured argon2id arena plus
scratch). The installer initrd must have that much free at the rescue prompt,
which is worth knowing because that prompt appears on a machine that has just
failed to install. A malformed `ze.rescue-auth` is never gated on: the installer
reboots instead, so a typo in the credential can neither open an unauthenticated
shell nor strand an unattended box at a prompt nobody can answer.
<!-- source: internal/core/rescueauth/rescueauth.go -- argonMemory, digestOf -->
<!-- source: internal/install/disk/rescue_linux.go -- selectFatalBranch -->
<!-- source: internal/core/rescueauth/rescueauth.go -- Value, Check, NewValue -->
<!-- source: internal/plugins/imageserver/handler.go -- serveBootIPXE, rescueAuth -->

### Running Tests

The installer logic has Go unit tests alongside each source file in
`internal/install/disk` (cmdline parsing, disk detection, the netlink lease
apply, the stall-timeout download, the partition-node wait, and the recovery
console / fatal policy):

```bash
CGO_ENABLED=0 go test ./internal/install/disk/...
```

End-to-end boot and install are covered by the QEMU evidence harness, which
boots the real Go initrd:

```bash
./le qemu install-test       # HTTP PXE install
./le qemu install-iso-test   # ISO install
```

### No External Binaries

The initrd contains exactly one file: the `cmd/ze-installer` Go binary as
`/init` (PID 1). There is no busybox and no shell. Every operation the old shell
init shelled out to (mount, DHCP, HTTP download, link/address/route, reboot) is
an in-process `golang.org/x/sys/unix` syscall or a vendored library call, so
there is no applet that can be `not found` at install time. See
`ai/rules/platform-linux.md`.

## Installer Kernel

The initrd carries **no kernel modules**, so the kernel it boots alongside must
have NIC drivers, disk drivers, ext4, devtmpfs, initramfs and `ip=dhcp`
autoconfiguration all built in (`=y`). Stock distro/cloud kernels ship these as
modules and cannot boot the initrd. `ze` deliberately ships no installer kernel:
the right kernel is site-specific.

`tools/installer-kernel/` builds a reference kernel with two profiles:

| Profile | File | Drivers | Use case |
|---------|------|---------|----------|
| `qemu` (default) | `qemu.config` | virtio NIC + block | QEMU tests, fast build |
| `hardware` | `hardware.config` | virtio + EFI + framebuffer + Intel/Realtek/Broadcom/Mellanox NICs + AHCI + NVMe | Bare metal PXE/ISO install |

Both profiles merge onto a shared base (`kernel.config`) that provides IP
autoconfiguration, SCSI, ext4, initramfs, devtmpfs and serial console.

```bash
ze appliance kernel prod
ze appliance kernel --profile hardware --arch amd64
ze appliance kernel --builder qemu --arch arm64 prod
```

Set `image.kernel-profile` to `"hardware"` in `appliance.json` so
`ze appliance kernel <name>` picks it up automatically. The CLI selects Docker
first, then the shared QEMU backend, unless `--builder` forces one path.

`ze appliance kernel` resolves the open profile registry in Go and passes the
fragments to `internal/appliance/kernelbuilder`. The worker merges and checks
the config, then rejects any required symbol that is not built in. Docker and
QEMU are native backends of the same Go driver. Output is cached by target,
architecture, profile, config, and kernel version.

<!-- source: internal/appliance/kernelreg.go -- resolveKernelProfile -->
<!-- source: internal/appliance/kernelreq.go -- enforceKernelRequirements -->
<!-- source: internal/appliance/kernelbuilder/driver.go -- Build -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->
<!-- source: internal/appliance/kernelbuilder/qemu.go -- runQEMU -->

## End-to-End QEMU Verification

`./le qemu install-test` exercises the entire chain with no hardware. It builds
the initrd and an appliance image, boots the installer kernel and initrd against
a blank virtio disk, downloads and writes the image and ZeFS over HTTP, then
boots the written disk and logs in over SSH.

```bash
ZE_INSTALL_KERNEL=$PWD/build/kernel/Image ./le qemu install-test
```

`./le qemu install-iso-test` exercises the ISO transport. It creates an ISO
through `ze appliance iso`, boots it, verifies the embedded image is written
without the PXE-only branch, checks safe poweroff and the GPT layout, then logs
in with the embedded ZeFS credentials.
<!-- source: internal/le/qemu/actions.go -- Actions -->

```bash
ZE_INSTALL_KERNEL=$PWD/build/kernel/Image ./le qemu install-iso-test
```

The ISO evidence self-skips with `INSTALL-ISO-QEMU: SKIP` when QEMU, a suitable
installer kernel, UEFI firmware, `grub-mkstandalone`/`grub2-mkstandalone`,
`xorriso`, or image-build tooling is unavailable.
<!-- source: internal/le/qemu/actions.go -- Answer -->

The test self-skips (does not fail) when `ZE_INSTALL_KERNEL` is unset or a
container runtime / `qemu-system-*` is unavailable, because there is no safe
default installer kernel.

### Environment Knobs

| Variable | Default | Purpose |
|----------|---------|---------|
| `ZE_INSTALL_KERNEL` | none; self-skips | Path to the installer kernel `Image`/`vmlinuz` |
| `ZE_INSTALL_ARCH` | host arch (`amd64` for ISO evidence) | Target architecture for QEMU installer evidence and generated appliance config (`arm64`/`amd64`) |
| `ZE_INSTALL_BOOT_TIMEOUT` | `300` | Seconds to wait for the installer to write the disk |
| `ZE_INSTALL_IMAGE_SIZE` | appliance default (2 GiB) | Override image `size-bytes` (must stay large enough for the gokrazy A/B layout) |
| `ZE_INSTALL_SSH_USER` / `ZE_INSTALL_SSH_PASS` | `admin` / `secret` | Power-user credentials provisioned into the image and used for the AC login |
| `ZE_INSTALL_NIC` | `virtio-net-pci` | QEMU NIC model for the installer boot |
| `ZE_INSTALL_KEEP` | unset | Keep the work directory (image, written disk, serial logs) for inspection |
| `ZE_INSTALL_IMAGE` / `ZE_INSTALL_ZEFS` | unset | Reuse a prebuilt image + zefs instead of building one |

### QEMU Networking Note

The test points the guest at the slirp gateway (`ze.server=10.0.2.2
ze.port=<ephemeral>`) rather than a `guestfwd` forward. A `guestfwd` services
only the first guest connection, which stalls the installer's second and third
downloads (image, zefs); the gateway handles the sequential connections the
installer makes. This is only a test-harness concern; real PXE installs use a
real network and `ze install remote` on port 80.
