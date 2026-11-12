# Installation

Choose a deployment path for the environment you want to evaluate. Ze is
pre-release software; start with an isolated lab.

## Choose a deployment path

| Goal | Guide | Success check |
|------|-------|---------------|
| Exchange routes on one Linux host | [Two-peer quickstart](quickstart.md#local-two-peer-lab) | Both peers establish and each received RIB contains the other peer's prefix. This checks the control plane. |
| Run a Linux service | [Local installation](#local-installation) and [build prerequisites](ubuntu-build-install.md) | The service is active, authenticated management connects, and repeat installation retains configuration and credentials. |
| Run a daemon in Docker | [Docker Compose](docker.md) | A committed configuration change and the same login survive container recreation on the same named volume. |
| Install a VM or dedicated router | [Appliance build](appliance.md#build-an-image) and [ISO installation](#appliance-iso-install) | After removing installer media, the written disk boots and authenticated SSH reports the selected image. |
| Run a protocol topology | [netlab and containerlab](netlab.md) | The live check must pass adjacencies, kernel routes, traffic and teardown. Live verification remains pending. |
<!-- source: examples/quickstart/r1.conf -- local lab endpoints and originated prefix -->
<!-- source: examples/quickstart/r2.conf -- local lab endpoints and originated prefix -->
<!-- source: internal/plugins/local/native.go -- nativeRuntime.install -->
<!-- source: docker/compose.yaml -- named volume and bare start -->
<!-- source: internal/le/test/qemu/install_iso.go -- executeISO -->
<!-- source: internal/le/test/netlab/live.go -- RunLive -->

Completed deployment checks covered native systemd installation and removal,
two-peer route exchange, and container configuration retention after recreation.
The container check used a prepared runtime image without verified source
identity. A separate private preview run prepared and checked Linux amd64 and
arm64 binaries and archives, plus amd64 deployment and lab container exports,
with source identity and checksums. That artifact check does not extend the
container recreation proof to those images. See [preview preparation](../contributing/preview-artifacts.md);
these pages do not provide a public release download.
<!-- source: internal/test/fixture/register_native_install.go -- nativeInstallLifecycle -->
<!-- source: internal/plugins/systemd/native.go -- nativeService.installSelected, nativeService.Uninstall -->
<!-- source: internal/plugins/systemd/cmd_uninstall.go -- serviceRuntime.cmdUninstall -->
<!-- source: internal/test/fixture/deployment_quickstart.go -- deploymentQuickstart -->
<!-- source: docker/compose.yaml -- named volume and bare start -->
<!-- source: docker/Dockerfile.runtime -- prepared runtime images -->
<!-- source: internal/le/build/preview/preview.go -- prepare -->

The appliance installation and update checks passed on amd64 with KVM. They
covered selected-disk installation, preservation of an extra disk, installed-disk
SSH access, and an image update followed by recovery to the previous root slot.
Configuration and login survived the update and recovery. Arm64 boot and physical
hardware installation remain unverified by these checks. See
[appliance updates](appliance.md#updating) for retention and recovery checks.
<!-- source: internal/le/test/qemu/install_iso.go -- executeISO -->
<!-- source: internal/le/test/qemu/install_ota.go -- executeApplianceOTA -->

## Local Installation

Run `ze install` for the complete Linux installation. It installs the running
binary, initializes absent state, and starts the service when systemd is
available. Existing configuration and credentials are preserved.
<!-- source: internal/plugins/local/native.go -- RunNativeInstall, nativeRuntime.install -->

### Quick Start

```bash
sudo ze install
```

Build the command with the [Linux installation guide](ubuntu-build-install.md),
or use a privately supplied [preview bundle](../contributing/preview-artifacts.md).
From a built source checkout or extracted preview, use `sudo ./ze install`.
The default installed binary is
`/usr/local/bin/ze` and its private state is `/etc/ze`. A fresh installation asks
for management credentials and creates a loopback-only SSH configuration. It
does not configure the host's interfaces. Follow the connection command printed
by the installer, then run `ze doctor` and inspect the service with
`systemctl status ze.service`.
<!-- source: internal/plugins/local/native.go -- nativeRuntime.install, selectNativeTarget -->
<!-- source: internal/plugins/init/main.go -- runInstallInit -->

Choose a prefix or foreground operation explicitly:

```bash
sudo ze install --prefix /opt/ze
sudo ze install --no-service
sudo ze install --dry-run
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--prefix` | `/usr/local` | Installation prefix (binary goes to `<prefix>/bin/ze`) |
| `--no-service` | | Install for foreground operation without a service |
| `--dry-run` | | Print what would be done without making changes |

Without systemd, installation prints the foreground start command. Repeat
`ze install` to replace the binary while keeping state and compatible service
customizations. Conflicting paths or service definitions are refused before
binary replacement. See [Operations](operations.md) for upgrades and recovery.
<!-- source: internal/plugins/local/native.go -- nativeRuntime.install, nativeRuntime.selectedService, nativeTarget.preflight -->

### What It Does

1. Checks the selected paths and service before changing the installation.
2. Initializes the private store only when it is absent.
3. Installs the binary and starts the systemd service when available.
<!-- source: internal/plugins/local/native.go -- nativeRuntime.install -->

The config directory is resolved from the binary path following GNU prefix
conventions:

| Binary location | Config directory |
|-----------------|-----------------|
| `/usr/local/bin/ze` | `/etc/ze` |
| `/usr/bin/ze` | `/etc/ze` |
| `/opt/ze/bin/ze` | `/opt/ze/etc/ze` |

The installer creates a missing config parent with mode `0755`, so the service
account can traverse it after ownership of the private `0700` config directory
transfers. Existing parent permissions are preserved.
<!-- source: internal/plugins/local/native.go -- nativeRuntime.install -->

The explicit `ze install local` command remains available for a binary-only
installation. That lower-level path requires `ze init` and service setup as
separate steps.
<!-- source: internal/plugins/local/cmd_install.go -- cmdInstall -->

### Systemd Service

The default `ze install` command handles systemd setup. For an existing manual
installation, `ze install systemd` remains available:

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
ExecStart=/bin/sh -c 'trap "" HUP; exec <prefix>/bin/ze start'
ExecReload=/bin/sh -c 'kill -HUP $MAINPID'
Restart=on-failure
RestartSec=5
TimeoutStopSec=90
KillMode=mixed
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

`TimeoutStopSec=90` and `KillMode=mixed` serve the stop drain. On a stop, ze
first drains its routing protocols for up to 60 s, then tears down for up to
30 s, so systemd waits 90 s before its SIGKILL. `mixed` sends the stop SIGTERM
to ze alone, so ze can drain its external plugins before it stops them; the
SIGKILL at the timeout still reaches the whole unit. `ze doctor` reads the
values systemd actually applies, drop-ins included, and warns under
`doctor-service-stop` when either is weaker.
<!-- source: internal/plugins/systemd/unit.go -- stopTimeoutSeconds -->
<!-- source: internal/plugins/systemd/doctor.go -- checkServiceStop -->

The shell in `ExecStart` starts ze with SIGHUP ignored, then execs it, so ze
keeps the main PID. A `systemctl reload` during the first few hundred
milliseconds, before ze can register a handler, is then lost instead of killing
the daemon (`docs/architecture/behavior/signals.md`). `ExecReload` sends that
SIGHUP with the same shell's builtin `kill`, so the unit runs no program beyond
`/bin/sh` and ze: `/bin/kill` comes from `procps`, which a minimal host does not
install. `ze doctor` checks the programs `ExecStart` and `ExecReload` run, and
reports a missing one under `doctor-service-executable`.
<!-- source: internal/plugins/systemd/unit.go -- buildUnitFile -->
<!-- source: internal/plugins/systemd/doctor.go -- checkServiceExecutable -->

The install refuses a
binary or config path holding a character that systemd or `/bin/sh` interprets:
a quote, a backslash, `$`, `%`, `;`, `&`, `|`, `<`, `>`, parentheses, a
backtick, or a glob character.

`ze install systemd` requires `<config-dir>/database/` and refuses while another
process owns the store. It creates the `ze` user and group if missing,
recursively transfers the store to `ze:ze`, writes
`/etc/systemd/system/ze.service`, runs `systemctl daemon-reload`, and enables the
service. Storage ownership is strict even for root: after transfer, offline
maintenance runs as `ze` with the daemon stopped. Use `--dry-run` to print the
unit file without root or systemd, `--config <dir>` to override its config
directory, `--force` to overwrite an existing unit, and `--start` to start the
service after enabling it.

The systemd unit sets `XDG_RUNTIME_DIR=/run/ze`, so the daemon socket is
`/run/ze/ze.socket`. For local operator CLI access, configure
`daemon { socket "/run/ze/ze.socket"; }` or export `XDG_RUNTIME_DIR=/run/ze`.

## Uninstalling

Run `ze uninstall` to stop and remove the service before removing its binary.
A stop failure leaves the binary installed. Ordinary removal preserves private
state, credentials and the service account. Use `--purge` only when the selected
installation's state should also be removed.
<!-- source: internal/plugins/local/native.go -- nativeRuntime.uninstall -->

```bash
sudo ze uninstall --dry-run
sudo ze uninstall
```

Choose a matching alternative instead of the ordinary removal above when the
installation uses another prefix or its state must also be removed:

```bash
sudo /opt/ze/bin/ze uninstall --prefix /opt/ze
# Or, for the default prefix with all selected installation state removed:
sudo ze uninstall --purge
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--prefix` | `/usr/local` | Select the installation prefix |
| `--no-service` | | Remove a foreground installation without a service |
| `--purge` | | Also remove config directory and database |
| `--dry-run` | | Print what would be done without making changes |
| `--yes` | | Skip confirmation prompt |

Without `--yes`, uninstall shows what will be removed and asks for
confirmation before proceeding.
<!-- source: internal/plugins/local/native.go -- nativeRuntime.uninstall -->

The lower-level `ze uninstall systemd` and `ze uninstall local` commands remain
available for manual installations. Use the service command first. Unlike the
default removal, `ze uninstall systemd --purge` also removes the service account.
<!-- source: internal/plugins/systemd/cmd_uninstall.go -- cmdUninstall -->
<!-- source: internal/plugins/local/cmd_uninstall.go -- cmdUninstall -->

To check the service status, use `systemctl status ze.service` directly.

## Installing on Real Hardware (End to End)

This bare-metal PXE walkthrough follows the same chain as
`./le test qemu installer`: build an image, serve it, boot an installer kernel
and initrd, write the disk, then log in over SSH. The reference sections below
describe each piece.

### 1. Build the disk image

Use the structured appliance builder (full reference:
[appliance guide](appliance.md), "./ze-host appliance"):

```bash
./le build host-driver
GOROOT="$(./le go stdpatch goroot)"
export GOROOT GOTOOLCHAIN=local PATH="$GOROOT/bin:$PATH"
./ze-host appliance init prod
# For real hardware: set image.kernel-profile to "hardware" and
# image.arch to match the target CPU in appliance.json before build.
# The build runs under Ze's patched Go tree (see below).
./ze-host appliance build prod
```

Run these build-host commands from the source checkout after installing Go and
Git. `./ze-host` contains `appliance` and `install remote`; the normal `./ze`
launcher builds the daemon and does not contain the appliance builder. Keep the
patched environment above for the remaining image-build steps.
<!-- source: internal/le/build/compile/build.go -- HostTags, HostBinary, hostPlan -->
<!-- source: cmd/ze/setup_features_setup.go -- appliance and provisioning imports -->

The image's ze compiles only against Ze's patched standard library, so
`./ze-host appliance build` refuses unless its environment selects the patched
GOROOT: the line before it sets `GOROOT`, `GOTOOLCHAIN=local` and the tree's
`bin` first on `PATH`. Why, and every other build route:
[patching the standard library](../contributing/patching-the-standard-library.md).

This produces `~/.config/ze/appliances/prod/ze-<timestamp>.img` with TLS, SSH
credentials, and a seed config baked into `/perm/ze/database.zefs`. Match
`image.arch` in `appliance.json` to the target CPU.

### 2. Build the installer kernel and initrd

The target PXE-boots a kernel + initrd, *not* the disk image. Build both for the
target architecture:

```bash
./ze-host appliance kernel prod                     # reads arch + profile from appliance.json
./ze-host appliance initrd                          # build/initrd/initrd.img.gz (pure Go)
```

Choose the profile and architecture through the native command:

```bash
./ze-host appliance kernel --profile hardware --arch amd64 prod
./ze-host appliance kernel --profile hardware --arch arm64 prod
./ze-host appliance initrd
```

The installer `PROFILE` selects added driver requests: `qemu` (default, virtio) or
`hardware` (EFI stub, framebuffer, Intel/Realtek/Broadcom/Mellanox NICs,
AHCI, NVMe). A stock distro kernel will **not** boot the module-free initrd;
see [Installer Kernel](#installer-kernel).

`image.kernel-profile` selects this installer profile, not the running appliance
kernel. Runtime images currently use the shared `runtime` profile with both
physical and virtual drivers. Both builders start from Linux `defconfig`, so a
profile fragment is not an exhaustive list of the drivers in the result.
<!-- source: internal/appliance/cmd_kernel.go -- runKernel -->
<!-- source: internal/appliance/runtimekernel.go -- RuntimeKernelTree -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->

### 3. Start the provisioning server

On a ze device on the (isolated) provisioning network:
<!-- source: internal/plugins/provision/main.go -- Run -->

```bash
sudo ./ze-host install remote \
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
5. The initrd writes the selected fixed disk (`/dev/sda`, `/dev/nvme0n1`,
   `/dev/mmcblk0` ...). It preserves a baked `database.zefs` or an existing
   `database/` tree on the written partition. Only a seedless partition receives
   `/install/database.zefs`, then the device reboots.
6. Ze explicitly imports the seed into `/perm/ze/database/`, retains the seed as
   `database.zefs.replaced-*`, and serves its configuration. A credential-only
   seed enters [bootstrap mode](#bootstrap-mode).

A device that already runs Ze can take the same seed without reinstalling:
`ze init --from http://<server>/install/database.zefs --sha256 <hex>` fetches
it with the installer's download helper, checks the digest and every entry
before a key is written, imports it into `database/`, and removes the fetched
copy. A TFTP server works too:
`ze init --from tftp://<server>[:port]/install/database.zefs --sha256 <hex>`
fetches the blob with Ze's own TFTP client, in octet mode, from port 69 unless
you name another. TFTP authenticates nothing, so always give `--sha256` with a
TFTP source. A local path (`ze init --from ./backup.zefs`) imports a file in
place and retires it as `.replaced-<stamp>`, as the appliance does with its
seed.
<!-- source: internal/plugins/init/main.go -- runImport, runFetchImport -->
<!-- source: internal/core/fetch/tftp.go -- tftpToFile, parseTFTPURL -->

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
- **Dry-run first**: `./le test qemu installer` reproduces the whole chain and
  reports a broken image, kernel, or initrd before hardware is touched.

## Appliance ISO Install

For appliances built with `./ze-host appliance build`, ISO media is an offline
install transport for the gokrazy image. The image is gzip-compressed inside the
ISO; the installer initrd decompresses it during installation. Create it with:
<!-- source: internal/appliance/cmd_iso.go -- runIso -->

```bash
./le build host-driver
GOROOT="$(./le go stdpatch goroot)" && export GOROOT GOTOOLCHAIN=local PATH="$GOROOT/bin:$PATH"
./ze-host appliance build prod                       # under the patched Go tree, see "Build the disk image"
./ze-host appliance kernel --profile hardware prod   # download or build installer kernel (reads arch from config)
./ze-host appliance initrd                           # download or build installer initrd
./ze-host appliance iso prod
```

Use `./ze-host appliance iso --check prod` to check the named appliance's prerequisites
before building. This checks the GRUB EFI modules for its target architecture,
as well as the executable, kernel, initrd and xorriso. Omitting the name checks
the host architecture. On Debian or Ubuntu, install `grub-efi-amd64-bin` for
amd64 EFI modules or `grub-efi-arm64-bin` for arm64 EFI modules.
<!-- source: internal/appliance/cmd_iso.go -- runIso, checkISOPrerequisites -->
<!-- source: internal/appliance/iso_modules.go -- checkISOGRUBModules -->

The `kernel` and `initrd` commands
download pre-built artifacts when configured, then fall back to the native
local QEMU or Docker kernel builder and the Go initrd builder. Cached artifacts
are stored under `$XDG_CACHE_HOME/ze/`.

The installer searches optical and removable media for its matching ISO.
Optical devices can supply the image but cannot be selected as the disk to
overwrite. On completion, the installer powers off; remove the installation
media before booting the written disk.
<!-- source: internal/install/disk/iso.go -- findISOMedia, isISOMediaDisk -->
<!-- source: internal/install/disk/run.go -- runISO -->

For arm64 targets, run on an arm64 build host and set `image.arch` to `arm64`
in the named appliance's config first. The kernel version is single-sourced in
`internal/appliance/kernel.version`; `--version` only overrides it and must be a
kernel >= 7:

```bash
./ze-host appliance kernel --profile hardware --arch arm64 prod
./ze-host appliance iso --kernel build/kernel/Image prod
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
./ze-host appliance iso --target /dev/vda prod
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

`./ze-host install remote` is a one-command provisioning server that PXE-boots
target machines with a gokrazy image containing ze.

> **Warning: the provisioning network MUST be isolated.** Run
> `./ze-host install remote` only on a dedicated segment with no other DHCP server. On a shared
> network a foreign DHCP server (for example a corporate one) can win the
> boot-time race and hand the target a lease with no route back to `ze.server`;
> the installer then probes an unreachable server until `ze.wait` expires and
> drops to a debug shell, and the disk is never written. The initrd now
> re-validates the kernel lease and recovers when a reachable interface exists
> (see [Kernel Command Line](#kernel-command-line)), but a second DHCP server on
> the same L2 segment is unsupported and can still break provisioning.

### How It Works

1. The operator runs `sudo ./ze-host install remote` on a Linux provisioning
   host connected to the provisioning network.
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
sudo ./ze-host install remote \
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
staged automatically by `./ze-host install remote`.

The default `--pxe-dir build/pxe` is relative to the working directory. Run
`./ze-host install remote` from the repository root, or pass an absolute
`--pxe-dir`. The command stages bundled iPXE files and the explicit
`--kernel` and `--initrd` artifacts under that root.

### SSH Credentials

The `--ssh-password` is bcrypt-hashed (cost 10) before being embedded
in the generated config as `ssh-password-hash`. The plaintext password
is never written to disk or config. It is visible in the process listing
(`ps aux`) while `./ze-host install remote` is running.

### Generated Config

`./ze-host install remote` generates a standard ze config in brace format and
pipes it to a child `ze` process. The generated config enables three
plugins:

- **dhcpserver** with PXE options, a shared-network, and the computed pool
- **tftpserver** bound to the provisioning interface
- **imageserver** bound to the provisioning interface with SSH credentials

The same provisioning setup can be achieved by writing the config
manually and running `ze <config-file>`.

### Watching Provisioning Activity

`./ze-host install remote` runs the install servers at `info` log level by default, so the
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
build actually running, `./ze-host appliance build` bakes a manifest into the image at
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

SIGTERM or SIGINT sent to `./ze-host install remote` is forwarded to the child
ze process. The child shuts down cleanly (closes listeners, drains
connections). Closing the parent also sends EOF on the stdin pipe,
which ze treats as a shutdown signal.

## Bootstrap Mode

When Ze starts with a live store but no config file and no template, it enters
bootstrap mode automatically. A PXE credential-only seed reaches this state
after the first boot imports it into `/perm/ze/database/`.

### What Happens

1. Ze detects no config in the store (no `file/active/ze.conf`, no `file/template/ze.conf`).
2. Interface discovery enumerates all OS network interfaces.
3. A minimal config is generated: DHCP client enabled on every ethernet
   interface, SSH server enabled.
4. Ze starts with this config. DHCP clients acquire addresses, SSH becomes
   reachable.

### Operator Workflow

1. SSH into the device using the credentials pre-provisioned by `./ze-host install remote`.
2. Configure ze via the CLI (`ze config edit`, then commit).
3. The committed config replaces the bootstrap config. On the next restart,
   ze starts in normal mode.

### Constraints

- Only ethernet interfaces get DHCP. Bridge, veth, dummy, loopback,
  wireguard, and xfrm interfaces are skipped.
- SSH credentials come from the live tree, imported from the seed installed
  by the initrd.
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
5. In HTTP mode only, re-reads the partition table and mounts partition 4
   (ext4, `/perm`). It preserves an existing `database/` tree or non-empty
   `database.zefs`; otherwise it downloads the bootstrap seed to
   `/perm/ze/database.zefs`.
6. In HTTP mode, reboots. In ISO mode, powers off so the operator can remove
   the installer media before the next boot.

### Building the Initrd

Prerequisites: the Go toolchain only. The initrd is built in pure Go, so no
`busybox`, `cpio`, or `gzip` host tools are required.

```bash
./ze-host appliance initrd
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
Existing `./ze-host install remote` deployments need no `ze.source` change because the
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
./le test qemu installer       # HTTP PXE install
./le test qemu installer-iso   # ISO install
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

| Profile | File | Added driver requests | Use case |
|---------|------|---------|----------|
| `qemu` (default) | `qemu.config` | virtio NIC + block | QEMU installer tests |
| `hardware` | `hardware.config` | virtio + EFI + framebuffer + Intel/Realtek/Broadcom/Mellanox NICs + AHCI + NVMe | Bare metal PXE/ISO install |

Both profiles merge onto a shared base (`kernel.config`) that provides IP
autoconfiguration, SCSI, ext4, initramfs, devtmpfs and serial console. The build
starts with Linux `defconfig`, so this table does not enumerate every driver in
the result. These installer profiles do not select a smaller runtime kernel.
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->
<!-- source: internal/appliance/cmd_kernel.go -- runKernel -->

```bash
./ze-host appliance kernel prod
./ze-host appliance kernel --profile hardware --arch amd64
./ze-host appliance kernel --builder qemu --arch arm64 prod
```

Set `image.kernel-profile` to `"hardware"` in `appliance.json` so
`./ze-host appliance kernel <name>` picks it up automatically. The CLI selects Docker
first, then the shared QEMU backend, unless `--builder` forces one path.

`./ze-host appliance kernel` resolves the open profile registry in Go and passes the
fragments to `internal/appliance/kernelbuilder`. The worker merges and checks
the config, then rejects any required symbol that is not built in. Docker and
QEMU are native backends of the same Go driver. Output is cached by target,
architecture, profile, config, and kernel version.

A kernel is built only on a host of its own architecture: an arm64 kernel on an
arm64 machine (a Mac), an amd64 kernel on an amd64 machine. A request for the
other architecture is refused before any backend starts, naming the host's
architecture and where to build it, because a build under CPU emulation runs
for hours and fails late. For the same reason the QEMU backend runs only under
a hardware accelerator, `hvf` on macOS or `kvm` on Linux, and refuses rather
than fall back to the `tcg` emulator. When QEMU itself is missing or fails to
start, the error says so and shows QEMU's own message; install or repair QEMU,
then check that `qemu-system-<arch> -accel help` lists `hvf` or `kvm`. That
listing shows what QEMU was built with, not what this host can use, so the
build also checks the host: on Linux, `/dev/kvm` must open for your user
(when the refusal is a permission error, `sudo usermod -aG kvm $USER`, then
log in again; any other error points at the kernel log); on macOS,
`sysctl kern.hv_support` must print `1`. Each of these questions is bounded at
fifteen seconds, so a hung QEMU or Docker daemon is reported, not waited on.
`ze doctor` reports the same condition.

<!-- source: internal/appliance/kernelbuilder/driver.go -- validateRequest, hostGOARCH -->
<!-- source: internal/appliance/kernelbuilder/qemu.go -- hardwareAccelerator, probeKVM, probeHVF, availableAccelerators -->
<!-- source: internal/appliance/kernelreg.go -- resolveKernelProfile -->
<!-- source: internal/appliance/kernelreq.go -- enforceKernelRequirements -->
<!-- source: internal/appliance/kernelbuilder/driver.go -- Build -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->
<!-- source: internal/appliance/kernelbuilder/qemu.go -- runQEMU -->

## End-to-End QEMU Verification

`./le test qemu installer` builds the initrd and an appliance image, boots the
installer against a blank virtio disk, and transfers the image over HTTP. It
then boots the installed disk and authenticates over SSH. The storage proof
also checks the logged first-boot import, the live tree's seed key values, and
the retired seed. An incomplete staging tree is planted before boot to exercise
restart with an interrupted import.

```bash
ZE_INSTALL_KERNEL=$PWD/build/kernel/Image ./le --name storage-proof test qemu installer
```

`./le test qemu installer-iso` exercises the ISO transport. It creates an ISO
through `./ze-host appliance iso`, boots it, verifies the embedded image is written
without the PXE-only branch, checks safe poweroff and the GPT layout, then logs
in with the embedded ZeFS credentials.
<!-- source: internal/le/test/qemu/actions.go -- Actions -->

```bash
ZE_INSTALL_KERNEL=$PWD/build/kernel/Image ./le test qemu installer-iso
```

The ISO evidence self-skips with `INSTALL-ISO-QEMU: SKIP` when QEMU, a suitable
installer kernel, UEFI firmware, `grub-mkstandalone`/`grub2-mkstandalone`,
`xorriso`, or image-build tooling is unavailable.
<!-- source: internal/le/test/qemu/actions.go -- Answer -->

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
| `ZE_INSTALL_IMAGE` / `ZE_INSTALL_ZEFS` | unset | Reuse a fresh prebuilt image and its matching seed blob instead of building them |

### QEMU Networking Note

The test points the guest at the slirp gateway (`ze.server=10.0.2.2
ze.port=<ephemeral>`) rather than a `guestfwd` forward. A `guestfwd` services
only the first guest connection, which stalls the installer's second and third
downloads (image, zefs); the gateway handles the sequential connections the
installer makes. This is only a test-harness concern; real PXE installs use a
real network and `./ze-host install remote` on port 80.
