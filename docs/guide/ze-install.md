# Installation

Ze provides commands for local installation and remote (PXE) provisioning.

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

1. Parses `ze.server` and `ze.image` from the kernel command line
2. Downloads the gokrazy disk image from `http://<server>/install/image/<name>`
3. Writes the image directly to the first non-removable block device
4. Re-reads the partition table, mounts partition 4 (ext4, /perm)
5. Downloads `database.zefs` from `http://<server>/install/database.zefs`
6. Writes it to `/perm/ze/database.zefs`
7. Reboots

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

The bootloader (iPXE) sets these parameters:

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `ze.server` | Yes | | IP address of the ze-install server |
| `ze.image` | No | `ze.img` | Name of the disk image to download |
| `ip=dhcp` | Yes | | Kernel-level network configuration |

### Disk Selection

The init script selects the first non-removable block device via the
sysfs `removable` attribute. Virtual devices (loop, ram, dm, zram) and
optical drives (sr) are skipped.

Supported device types: `/dev/sda` (SATA/SCSI), `/dev/nvme0n1` (NVMe),
`/dev/mmcblk0` (eMMC).

### Error Handling

If the init script encounters an error (missing server IP, no disk found,
download failure after 3 retries), it drops to a shell for debugging
instead of rebooting. This allows the operator to diagnose network or
hardware issues.

### Running Tests

```bash
make -C tools/installer-initrd test
```

This runs the cmdline parsing and disk detection unit tests without
requiring QEMU or real hardware.
