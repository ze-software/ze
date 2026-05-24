# Zero-Touch Provisioning

`ze install serve` is a one-command provisioning server that PXE-boots
target machines with a gokrazy image containing ze.

## How It Works

1. The operator runs `ze install serve` on an existing ze device connected
   to the provisioning network.
2. Ze generates a config enabling DHCP (with PXE extensions), TFTP, and
   an HTTP image server, then forks itself (`ze -`) with the config piped
   to stdin.
3. A target machine PXE-boots: DHCP assigns an IP and directs it to the
   TFTP bootloader, which chain-loads the installer kernel and initrd
   via HTTP.
4. The installer writes the gokrazy image to disk and reboots.
5. The target boots into ze in bootstrap mode (future: spec-install-5).

## Quick Start

```bash
ze install serve \
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

## Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--interface` | Yes | | Network interface to bind all servers |
| `--network` | Yes | | Provisioning subnet CIDR (/8 to /30) |
| `--image` | Yes | | Path to gokrazy disk image file |
| `--ssh-username` | Yes | | Admin username for the installed target |
| `--ssh-password` | Yes | | Admin password (bcrypt-hashed before embedding) |
| `--address` | No | First IPv4 on interface | Server IP override |

## DHCP Pool

The DHCP pool range scales with the subnet size. The server IP is
excluded from the pool. Examples:

| Network | Server | Pool Start | Pool Stop |
|---------|--------|------------|-----------|
| 10.0.0.0/24 | 10.0.0.1 | 10.0.0.2 | 10.0.0.254 |
| 192.168.1.0/28 | 192.168.1.1 | 192.168.1.2 | 192.168.1.14 |
| 10.1.1.0/30 | 10.1.1.1 | 10.1.1.2 | 10.1.1.2 |

## PXE Boot

The DHCP server detects PXE clients via option 60 (`PXEClient:`) and
reads option 93 (client architecture) to select the bootfile:

| Architecture | Bootfile |
|-------------|----------|
| BIOS (type 0) | `ipxe.pxe` |
| UEFI (type 6, 7, 9) | `ipxe.efi` |

Bootfiles are served from `/var/lib/ze/install/tftp/` via TFTP.
The installer kernel and initrd are served from `/var/lib/ze/install/boot/`
via HTTP.

## SSH Credentials

The `--ssh-password` is bcrypt-hashed (cost 10) before being embedded
in the generated config as `ssh-password-hash`. The plaintext password
is never written to disk or config. It is visible in the process listing
(`ps aux`) while `ze install serve` is running.

## Generated Config

`ze install serve` generates a standard ze config in brace format and
pipes it to a child `ze` process. The generated config enables three
plugins:

- **dhcpserver** with PXE options, a shared-network, and the computed pool
- **tftpserver** bound to the provisioning interface
- **imageserver** bound to the provisioning interface with SSH credentials

The same provisioning setup can be achieved by writing the config
manually and running `ze <config-file>`.

## Requirements

- Root privileges (DHCP, TFTP, and HTTP bind to privileged ports)
- Bootloader files in `/var/lib/ze/install/tftp/` (iPXE binaries)
- Installer kernel and initrd in `/var/lib/ze/install/boot/`
- Disk image at the path specified by `--image`

## Shutdown

SIGTERM or SIGINT sent to `ze install serve` is forwarded to the child
ze process. The child shuts down cleanly (closes listeners, drains
connections). Closing the parent also sends EOF on the stdin pipe,
which ze treats as a shutdown signal.

## Limitations

- Single image: all targets receive the same gokrazy image
- No per-MAC image selection (future)
- No post-install hooks (future)
- Assumes an isolated provisioning network (no proxy DHCP)
