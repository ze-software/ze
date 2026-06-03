# Installation

Ze provides commands for local installation, remote PXE provisioning, and appliance ISO installer media.

## Local Installation

`ze install local` copies the ze binary to a standard system location,
optionally sets up a systemd service, and creates the config directory.

### Quick Start

```bash
sudo ze install local --no-systemd
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
sudo ze install local --prefix /usr/local --no-systemd
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--prefix` | interactive | Installation prefix (binary goes to `<prefix>/bin/ze`) |
| `--systemd` | | Force systemd service setup |
| `--no-systemd` | | Skip systemd service setup |
| `--dry-run` | | Print what would be done without making changes |

### What It Does

1. Copies the running ze binary to `<prefix>/bin/ze`
2. If systemd is available (auto-detected): creates `/etc/systemd/system/ze.service`, runs `systemctl daemon-reload` and `systemctl enable ze`
3. If no `database.zefs` exists at the config path: creates the config directory

The config directory is resolved from the binary path following GNU prefix
conventions:

| Binary location | Config directory |
|-----------------|-----------------|
| `/usr/local/bin/ze` | `/etc/ze` |
| `/usr/bin/ze` | `/etc/ze` |
| `/opt/ze/bin/ze` | `/opt/ze/etc/ze` |

After installation, run `ze init` to bootstrap the database.

### Systemd Unit

`ze install local` can install a minimal unit as part of binary installation.
For the current service-management path, install the binary with `--no-systemd`,
then use `ze service install` after `ze init`. This avoids creating a legacy
unit first and then having `ze service install` refuse the existing unit.

```bash
sudo ze install local --prefix /usr/local --no-systemd
sudo ze init
sudo ze service install --start
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
WorkingDirectory=<config-dir>
Environment=ZE_CONFIG_DIR=<config-dir>
Environment=XDG_RUNTIME_DIR=/run/ze
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ProtectHome=true
RuntimeDirectory=ze

[Install]
WantedBy=multi-user.target
```

`ze service install` refuses to run unless `<config-dir>/database.zefs` exists.
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

`ze uninstall` reverses a local installation.

```bash
ze uninstall              # remove binary + systemd unit
ze uninstall --purge      # also remove config directory and database
ze uninstall --dry-run    # preview what would be removed
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

To remove only the systemd service unit and keep the binary and config, use:

```bash
sudo ze service uninstall
```

## Installing on Real Hardware (End to End)

This is the bare-metal walkthrough for the PXE install flow. It is exactly what
`make ze-install-qemu-test` exercises in software (build an image, serve it,
boot an installer kernel + initrd that writes the disk, then log in over SSH) —
see [End-to-End QEMU Verification](#end-to-end-qemu-verification) to dry-run the
same chain before touching hardware. The reference subsections below
(Remote Provisioning, Installer Kernel, Installer Initrd, Bootstrap Mode) cover
each piece in detail; this section sequences them.

### 1. Build the disk image

Use the structured appliance builder (full reference:
[appliance guide](appliance.md), "ze install appliance"):

```bash
ze install appliance init prod
# For ARM targets, set image.arch to "arm64" in appliance.json before build.
ze install appliance build prod
```

This produces `~/.config/ze/appliances/prod/ze-<timestamp>.img` with TLS, SSH
credentials, and a seed config baked into its `/perm` zefs. Match
`image.arch` in `appliance.json` to the target CPU.

### 2. Build the installer kernel and initrd

The target PXE-boots a kernel + initrd, *not* the disk image. Build both for the
target architecture:

```bash
make -C tools/installer-kernel ARCH=amd64    # build/Image for x86_64; ARCH=arm64 for ARM
make -C tools/installer-initrd               # build/initrd.img.gz
```

A stock distro kernel will **not** boot the module-free initrd — it needs
virtio/ext4/IP-autoconfig built in. The reference kernel above does; see
[Installer Kernel](#installer-kernel).

### 3. Stage the boot artifacts

`ze install remote` serves from fixed directories on the provisioning device:

| Artifact | Location |
|----------|----------|
| iPXE bootloaders (`ipxe.pxe`, `ipxe.efi`) | `/var/lib/ze/install/tftp/` |
| installer kernel | `/var/lib/ze/install/boot/` |
| installer initrd | `/var/lib/ze/install/boot/` |
| disk image | path passed to `--image` (served at `/install/image/<filename>`) |

The kernel command line is set by **your iPXE script** (ze does not generate
one). After DHCP chainloads iPXE, point it at the kernel/initrd over HTTP and
pass the installer parameters — `ze.image` must match the served image filename:

```ipxe
#!ipxe
kernel http://${next-server}/install/boot/vmlinuz ze.server=${next-server} ze.image=ze-<timestamp>.img ip=dhcp panic=-1
initrd http://${next-server}/install/boot/initrd.img.gz
boot
```

Add `ze.port=<port>` only if the image server does not listen on port 80.

### 4. Start the provisioning server

On a ze device on the (isolated) provisioning network:

```bash
sudo ze install remote \
  --interface eth0 \
  --network 192.168.50.0/24 \
  --image ~/.config/ze/appliances/prod/ze-<timestamp>.img \
  --ssh-username admin \
  --ssh-password 'choose-a-strong-one'
```

This runs DHCP (with PXE options), TFTP (bootloaders), and the HTTP image
server on `eth0`. It serves the image at `/install/image/<filename>` and a
credential `database.zefs` (generated from `--ssh-*`, password stored hashed) at
`/install/database.zefs`. See [Remote Provisioning (PXE)](#remote-provisioning-pxe).

### 5. Net-boot the target

Set the target firmware to network boot. It then:

1. DHCPs an address and bootfile, and chainloads iPXE.
2. iPXE loads the installer kernel + initrd with your `ze.server` / `ze.image`.
3. The initrd downloads the image and `database.zefs`, writes the first fixed
   disk (`/dev/sda`, `/dev/nvme0n1`, `/dev/mmcblk0` ...; removable, virtual,
   optical and `mtdblock` flash devices are skipped), and reboots.
4. The target boots ze in [bootstrap mode](#bootstrap-mode) and starts SSH.

### 6. Log in and configure

```bash
ssh admin@<target-ip>      # the password given to --ssh-password
```

ze's SSH endpoint is the network-OS CLI. Configure with `ze config edit` and
commit; the committed config replaces the bootstrap config on the next restart.


### Troubleshooting

- **Installer drops to a shell** instead of rebooting on a bad `ze.server`, no
  writable disk, or a download that fails 3 times — read the serial/VGA console
  for the `[ze-install] FATAL:` line.
- **Wrong disk** written: the initrd picks the first non-removable disk; detach
  extra fixed disks or net-boot into the shell and inspect `/sys/block`.
- **Download stalls / non-standard port**: confirm the image server's port and
  pass `ze.port=` in the iPXE cmdline.
- **Dry-run first**: `make ze-install-qemu-test` reproduces the whole chain in
  QEMU and will surface a broken image, kernel, or initrd without hardware.

## Appliance ISO Install

For appliances built with `ze install appliance build`, ISO media is an offline
install transport for the gokrazy image. The image is gzip-compressed inside the
ISO; the installer initrd decompresses it during installation. Create it with:
<!-- source: cmd/ze/install/appliance/cmd_iso.go -- runIso -->

```bash
ze install appliance build prod
ze install appliance iso prod
make -C tools/installer-kernel ARCH=arm64
ze install appliance iso --kernel tools/installer-kernel/build/Image prod
```

The ISO installer decompresses and writes the embedded image to the target disk.
Unlike PXE provisioning, it does not download `/install/database.zefs` or write a
separate database after the disk image, because the appliance build already
injected `/perm/ze/database.zefs` into the image.
<!-- source: tools/installer-initrd/init -- ZE_SOURCE=iso branch -->
<!-- source: cmd/ze/install/appliance/cmd_build.go -- injectZeFS -->

The ISO bootloader target follows `image.arch`: amd64 images produce
`BOOTX64.EFI`, arm64 images produce `BOOTAA64.EFI`. By default the command looks
for `tools/installer-kernel/build/Image` and pairs that kernel with the shared
initrd. Build the matching architecture before running `ze install appliance
iso`, or pass `--kernel` to keep multiple kernels side by side.
<!-- source: cmd/ze/install/appliance/cmd_iso.go -- isoGRUBTarget, defaultISOKernelPath -->

If the target has more than one fixed disk, create the ISO with an explicit
whole-disk target. The initrd rejects ambiguous implicit disk selection in ISO
mode, excludes the ISO source media from target candidates, and requires a
builder-generated media id match before it trusts a mounted installer volume.
<!-- source: tools/installer-initrd/init -- find_target_disk, find_iso_media -->

```bash
ze install appliance iso --target /dev/vda prod
```

The generated ISO includes the installer kernel, initrd, the selected image,
its checksum, and metadata. It contains the full provisioned appliance image, so
handle it like the `.img` artifact.

ISO installs power off after the disk write so the removable installer media can
be removed before the next boot. They do not auto-reboot while the ISO is still
present.
<!-- source: tools/installer-initrd/init -- ZE_SOURCE=iso branch -->

<!-- source: cmd/ze/install/appliance/cmd_iso.go -- stageISO -->

## Remote Provisioning (PXE)

`ze install remote` is a one-command provisioning server that PXE-boots
target machines with a gokrazy image containing ze.

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
Use `--address` to override.

### Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--interface` | Yes | | Network interface to bind all servers |
| `--network` | Yes | | Provisioning subnet CIDR (/8 to /30) |
| `--image` | Yes | | Path to gokrazy disk image file |
| `--ssh-username` | Yes | | Admin username for the installed target |
| `--ssh-password` | Yes | | Admin password (bcrypt-hashed before embedding) |
| `--address` | No | First IPv4 on interface | Server IP override |

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

| Architecture | Bootfile |
|-------------|----------|
| BIOS (type 0) | `ipxe.pxe` |
| UEFI (type 6, 7, 9) | `ipxe.efi` |

Bootfiles are served from `/var/lib/ze/install/tftp/` via TFTP.
The installer kernel and initrd are served from `/var/lib/ze/install/boot/`
via HTTP.

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

### Requirements

- Root privileges (DHCP, TFTP, and HTTP bind to privileged ports)
- Bootloader files in `/var/lib/ze/install/tftp/` (iPXE binaries)
- Installer kernel and initrd in `/var/lib/ze/install/boot/`
- Disk image at the path specified by `--image`

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
  the next startup path (web-only or error).

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

Prerequisites: `busybox-static`, `cpio`, `gzip`.

```bash
make -C tools/installer-initrd
```

This produces `tools/installer-initrd/build/initrd.img.gz`. Copy it
alongside a Linux kernel to the boot directory served by the image
server (`/var/lib/ze/install/boot/`).

To use a custom busybox path:

```bash
make -C tools/installer-initrd BUSYBOX=/usr/bin/busybox-static
```

### Kernel Command Line

The bootloader sets these parameters:

| Parameter | Required | Default | Purpose |
|-----------|----------|---------|---------|
| `ze.source` | No | `http` | Source mode: `http` for PXE or `iso` for local ISO media |
| `ze.server` | HTTP only | | IPv4 address of the ze-install server |
| `ze.port` | No | `80` | TCP port of the install HTTP server (1-65535) |
| `ze.image` | No | `ze.img` | Name of the disk image to install |
| `ze.target` | No | | Explicit whole-disk target such as `/dev/vda` |
| `ze.media-id` | ISO only | | Builder-generated 32-hex token that identifies the booted installer ISO |
| `ip=dhcp` | HTTP only | | Kernel-level network configuration |

`ze.port` exists for install servers that cannot bind the privileged port 80
(for example an unprivileged HTTP server, or a QEMU test harness that serves on
an ephemeral port). ISO mode does not use `ze.server`, `ze.port`, or `ip=dhcp`.
Existing `ze install remote` deployments need no `ze.source` change because the
default source is `http`.
<!-- source: tools/installer-initrd/init -- parse_cmdline, validate_source -->

The init script selects a non-removable block device via sysfs. Virtual devices
(loop, ram, dm, zram, md), optical drives (sr), floppies (fd), and firmware/CFI
flash (`mtdblock`, the QEMU `virt` machine's pflash) are skipped.

In HTTP mode, the installer preserves the existing first-candidate behavior. In
ISO mode, it also excludes the ISO source media and refuses to choose
implicitly when more than one fixed candidate remains. Use `ze.target=/dev/vda`
to name an explicit whole disk.

Supported target disk forms include `/dev/sda` (SATA/SCSI), `/dev/vda`
(virtio-blk, used by QEMU/KVM), `/dev/nvme0n1` (NVMe), and `/dev/mmcblk0`
(eMMC).
<!-- source: tools/installer-initrd/init -- find_target_disk, validate_target_path -->
### Error Handling

If the init script encounters an error (missing server IP, no disk found,
download failure after 3 retries), it drops to a shell for debugging
instead of rebooting. This allows the operator to diagnose network or
hardware issues.

### Running Tests

```bash
make -C tools/installer-initrd test
```

This runs the cmdline parsing, disk detection, ISO media discovery, and image
write unit tests without requiring QEMU or real hardware.

### Busybox Applets

The initrd is a single static busybox plus symlinks. The Makefile symlinks
every applet the init script uses (`sh cat mount umount mkdir sleep wget dd
sync reboot poweroff blockdev basename rm mktemp mkfifo sha256sum tee` ...), and `/init`
also runs `busybox --install -s /bin` at boot as defence in depth. A missing
applet would otherwise surface only at install time as a `not found` error and
a kernel panic, so the init avoids non-essential externals (for example it
parses the checksum file with the shell `read` builtin rather than `cut`).

## Installer Kernel

The initrd carries **no kernel modules**, so the kernel it boots alongside must
have virtio-net, virtio-blk, ext4, devtmpfs, initramfs and `ip=dhcp`
autoconfiguration all built in (`=y`). Stock distro/cloud kernels ship these as
modules and cannot boot the initrd. `ze` deliberately ships no installer kernel:
the right kernel is site-specific.

`tools/installer-kernel/` builds a reference kernel that satisfies these
requirements (and is what the end-to-end QEMU test boots):

```bash
make -C tools/installer-kernel                 # build/Image for arm64
make -C tools/installer-kernel ARCH=amd64      # build/Image for x86_64
make -C tools/installer-kernel LINUX_VERSION=6.12.9
```

`build.sh` verifies the required options resolved to `=y` before building and
fails loudly if any did not. Output is `build/Image` (the kernel) and
`build/config` (the resolved config). See `tools/installer-kernel/README.md`
for the full rationale and the list of forced options.

## End-to-End QEMU Verification

`make ze-install-qemu-test` exercises the entire chain in QEMU with no hardware:
it builds the initrd, builds a real appliance image with `ze install appliance`
(see the [appliance guide](appliance.md)), boots the installer kernel + initrd
against a blank virtio disk, has the initrd download and write the image and
zefs over HTTP, then boots the **written disk** and logs in over SSH as the
provisioned power user. That final login is the regression test for credential
loading from the installed zefs.

```bash
ZE_INSTALL_KERNEL=$PWD/tools/installer-kernel/build/Image make ze-install-qemu-test
```

`make ze-install-iso-qemu-test` exercises the appliance ISO transport. It builds
the initrd and appliance image, creates an ISO through `ze install appliance
iso`, boots that ISO in QEMU, verifies the embedded image is written without the
PXE-only ZeFS download branch, verifies the installer powers off safely, inspects
the written GPT layout, and logs in over SSH using credentials from the embedded
ZeFS database.
<!-- source: scripts/evidence/effective-install-iso-qemu.py -- main -->

```bash
ZE_INSTALL_KERNEL=$PWD/tools/installer-kernel/build/Image make ze-install-iso-qemu-test
```

The ISO evidence self-skips with `INSTALL-ISO-QEMU: SKIP` when QEMU, a suitable
installer kernel, UEFI firmware, `grub-mkstandalone`/`grub2-mkstandalone`,
`xorriso`, static busybox, or image-build tooling is unavailable.
<!-- source: scripts/evidence/effective-install-iso-qemu.py -- main, skip -->

The test self-skips (does not fail) when `ZE_INSTALL_KERNEL` is unset or a
container runtime / `qemu-system-*` is unavailable, because there is no safe
default installer kernel.

### Environment Knobs

| Variable | Default | Purpose |
|----------|---------|---------|
| `ZE_INSTALL_KERNEL` | (none — self-skips) | Path to the installer kernel `Image`/`vmlinuz` |
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
installer makes. This is purely a test-harness concern — real PXE installs use a
real network and `ze install remote` on port 80.
