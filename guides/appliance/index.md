# VM Appliance

Build a bootable VM image with Ze baked in using [gokrazy](https://gokrazy.org/). The default target is x86_64, and `image.arch` in `appliance.json` selects another architecture for `ze appliance build`. At runtime, the appliance is minimal: Linux kernel, gokrazy init, and Ze as the only application, with no package manager, no general shell (except authenticated emergency serial console), no unused distro daemons, and automatic process supervision.

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

The build needs `mkfs.ext4`, `debugfs`, and `e2fsck` from e2fsprogs. The native
builder resolves each tool independently from the Homebrew keg, standard sbin
directories, and `PATH`.
<!-- source: internal/appliance/cmd_build.go -- resolveE2FSTool -->

For appliance ISO creation, install `grub-mkstandalone` (or `grub2-mkstandalone`)
plus `xorriso`.
`ze appliance iso` checks those tools before it stages an ISO.
<!-- source: internal/appliance/cmd_iso.go -- resolveISOBuilder -->

The vendored gokrazy command lives at `cmd/ze-gok`; the appliance builder calls
it in process. No separate gokrazy installation or first-party script is
required.
<!-- source: internal/appliance/cmd_build.go -- runGokInProcess -->

## First-time setup

Initialize the named appliance before its first build:

```bash
ze appliance init edge-01
```

The build resolves pinned system packages through the Go module graph.
<!-- source: gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod -- pinned gokrazy version -->

## Runtime Kernel Requirements

The runtime kernel is pinned by two manifests, `gokrazy/kernel/kernel.require`
and `gokrazy/kernel/runtime.require`, holding 59 symbols between them. Each is
checked against the resolved config after the build, and `enforce_required_symbols`
accepts `=y` alone. A Kconfig answer of `m` therefore fails the BUILD rather than
shipping an appliance where a feature Ze accepts in config cannot work. A module
is unreachable in the QEMU test VM in any case: that VM boots this kernel beside
Alpine's own modules, built for another version.

Each symbol has a producer in Ze rather than a test that wanted it.

| Group | Symbols | Producer in Ze |
|-------|---------|----------------|
| Subscriber | `PPP`, `PPPOL2TP`, `PPPOE`, `L2TP`, `L2TP_V3` | The L2TP LNS, and the PPPoE server and client |
| Firewall | `NF_TABLES`, `NF_TABLES_INET`, `NF_TABLES_IPV4`, `NF_TABLES_IPV6`, `NF_CONNTRACK`, `NF_NAT`, `NFT_CT`, `NFT_NAT`, `NFT_MASQ`, `NFT_REDIR`, `NFT_LIMIT`, `NFT_LOG`, `NFT_REJECT` | The nftables backend, and `translatePolicy` (`internal/plugins/copp/translate.go`), which returns `FamilyInet` unconditionally. Without `NF_TABLES_INET` the kernel answers EOPNOTSUPP, `Apply`'s flush fails, the firewall plugin fails startup, and the daemon exits |
| Tunnels | `NET_IPGRE_DEMUX`, `NET_IPGRE`, `IPV6_GRE`, `NET_IPIP`, `IPV6_TUNNEL`, `IPV6_SIT`, `VXLAN` | The nine tunnel kinds `ze-iface-conf.yang` models. Six symbols plus the GRE demux gate. Measured cost: vmlinuz 16352256 to 16450560 bytes, +0.60% |
| Policy routing | `IP_MULTIPLE_TABLES`, `IPV6_MULTIPLE_TABLES`, `IP_ROUTE_MULTIPATH` | The policy-routing engine. Without `IPV6_MULTIPLE_TABLES` the kernel folds every table id into the main table and says nothing |
| Traffic control | `NET_SCH_HTB`, `TBF`, `FQ_CODEL`, `HFSC`, `FQ`, `SFQ`, `NETEM`, `PRIO`, `INGRESS`, `NET_CLS_U32`, `NET_CLS_FW`, `NET_CLS_MATCHALL`, `NET_ACT_MIRRED` | Class of service, rate limiting, and mirroring |
| Interfaces and VPN | `DUMMY`, `VETH`, `MACVLAN`, `VLAN_8021Q`, `INET_ESP`, `INET6_ESP`, `XFRM_STATISTICS`, `HUGETLBFS`, `BPF_SYSCALL`, `BPF_JIT` | Interface creation, IPsec, VPP hugepages, and the eBPF surfaces |

`CONFIG_NET_UDP_TUNNEL`, `CONFIG_WIREGUARD` and `CONFIG_TUN` are `=y` in the
config fragment and are not pinned in a manifest.
<!-- source: gokrazy/kernel/runtime.require -- runtime requirements -->
<!-- source: gokrazy/kernel/kernel.require -- base requirements -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- enforceRequiredSymbols -->

## L2TP Kernel Support

Ze's L2TP LNS path needs kernel PPPoL2TP support in the appliance runtime
kernel: `CONFIG_PPP`, `CONFIG_PPPOL2TP`, `CONFIG_L2TP`, and
`CONFIG_L2TP_V3`. The shared runtime proof kernel also keeps `CONFIG_PPPOE`
built in for PPPoE evidence. The pinned upstream gokrazy kernel is not assumed
to provide these options.
Build the repo-local kernel before building an appliance intended to terminate
L2TP subscribers:

```bash
ze appliance kernel --target runtime --arch amd64
ze appliance build edge-01
```

Select the QEMU builder or arm64 explicitly when needed:

```bash
ze appliance kernel --target runtime --arch arm64 --builder qemu
ze appliance build edge-arm64
```

`ze appliance kernel` calls the Go driver in
`internal/appliance/kernelbuilder`. The driver reads
`internal/appliance/kernel.version`, selects Docker or QEMU, resolves the
tracked config fragments and `.require` manifests, and writes the runtime
kernel tree. The Go worker enforces every required symbol before the artifact
can be used.
<!-- source: internal/appliance/cmd_kernel.go -- runKernel -->
<!-- source: internal/appliance/kernelbuilder/driver.go -- Build -->
<!-- source: internal/appliance/kernelbuilder/worker.go -- RunWorker -->

On a Linux runner with QEMU, `xl2tpd`, `pppd`, `/dev/ppp`, and PPPoL2TP kernel
support, the deployment proof target builds an L2TP-enabled appliance image and
drives a real LAC against it:

```bash
./le deployment gokrazy-l2tp-ppp-test
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
<!-- source: internal/appliance/cmd_kernel.go -- runKernel -->
<!-- source: internal/le/deployment/actions.go -- Answer -->

## Build an image

Create a named appliance once, then build it:

```bash
ze appliance init edge-01
ze appliance build edge-01
```

The named appliance keeps its config and secrets between builds. The build
assembles its ZeFS database, builds the disk image through vendored gokrazy,
formats the persistent partition, injects the database, and writes the build
manifest.

```bash
ze appliance show edge-01
ze appliance config edge-01 --merged
```

<!-- source: internal/appliance/main.go -- applianceCommands -->
<!-- source: internal/appliance/cmd_build.go -- runBuild, buildOne -->

## Test in QEMU

```bash
ze appliance run edge-01
```

The command boots the named image in QEMU. Quit QEMU with **Ctrl-A X**.
The Gokrazy management UI is exposed through Ze's authenticated web UI at
`/gokrazy/`; the proxy reads Gokrazy's password from its standard password-file
locations.
<!-- source: internal/appliance/cmd_run.go -- runRun -->
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

The initial Ze config is stored as the seed template in
`gokrazy/ze/ze.conf`. `ze appliance assemble` uses it when no base or
per-appliance overlay exists. The seed database holds no active config that can
shadow the template, so the appliance builds its effective config on first
boot.
<!-- source: gokrazy/ze/ze.conf -- seed template -->
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

To change the seed config, edit `gokrazy/ze/ze.conf`, or use the structured
workflow's `config-base` and per-appliance `ze.conf` files.

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
ze appliance push <name>
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
  main.go                   # vendored gokrazy command wrapper
```

The gok source is vendored under `vendor/github.com/gokrazy/`. The small
`builddir/` modules pin the system-package versions.

### Builds never run from this directory

The tree above is the build *input*; no build runs inside it. Every image build
first copies `gokrazy/ze` to a fresh directory under the project `tmp/`,
including the whole `builddir`, rewriting each filesystem-path `replace` to an
absolute path so it still resolves from the new depth. gok is then pointed at the
copy, and the copy is deleted afterwards.

<!-- source: internal/appliance/instance/prepare.go -- Prepare, copyBuildDir, absolutizeReplaces -->

`ze appliance build` prepares an isolated copy through
`resolveBuildParentDir`. A build leaves the working tree unchanged, and two
builds in one checkout use separate prepared instances.

<!-- source: internal/appliance/kernelargs.go -- resolveBuildParentDir -->
<!-- source: internal/appliance/instance/prepare.go -- Prepare -->

Build a verified local runtime kernel before the image when required:

```
ze appliance kernel --target runtime --arch amd64
ze appliance build edge-01
```

The kernel replacement is written into the prepared copy only, so nothing in
the source tree needs to be reverted.
<!-- source: internal/appliance/instance/prepare.go -- replaceKernel -->


## Build-host command

Appliance build commands (`ze appliance`) and PXE provisioning
(`ze install remote`) are registered in the Ze binary. Build it from the
repository root when it is not already installed:

```bash
go build -tags ze_setup -o bin/ze ./cmd/ze
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

Build the full ISO through the structured appliance actions:

```bash
ze appliance init --config prod.json prod
ze appliance kernel prod
ze appliance initrd
ze appliance build prod
ze appliance iso prod
```

The named appliance retains its config and secrets for subsequent builds.

### Manual steps

Use the same commands individually when you need to inspect an intermediate
artifact:

```bash
# Create the appliance with config and secrets.
ze appliance init --config prod.json prod

# Check and prepare ISO prerequisites.
ze appliance iso --check
ze appliance kernel prod
ze appliance initrd

# Build the disk image and installer ISO.
ze appliance build prod
ze appliance iso prod

# Provision the resulting image over the network when needed.
ze install remote \
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
ze appliance init lab                  # interactive wizard
ze appliance build lab                 # full image
ze appliance kernel lab                # installer kernel
ze appliance initrd                    # installer initrd
ze appliance iso lab                   # bootable installer ISO
ze appliance list                      # show all appliances
ze appliance show lab                  # config summary and cert expiry
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
ze appliance unlock                    # start agent
ze appliance unlock --duration 15m     # auto-expire after 15 minutes
ze appliance unlock --stop             # stop agent
```

### Day-2 operations

```bash
ze appliance passwd lab
ze appliance replace-cert lab
ze appliance replace-cert lab --cert ca.pem --key ca.key
ze appliance rekey lab
ze appliance clone lab lab2
```

`replace-cert` validates the material before it writes anything. It refuses a
certificate and a key that are not a pair. It refuses a file that holds no PEM
data. It refuses a certificate that is past its not-after date, and the message
gives both validity dates. A certificate whose validity starts in the future is
accepted, because a staged renewal is copied into an image that boots later.
`--cert` and `--key` must be given together.

A refusal leaves `cert.pem` and `key.pem` byte-identical to what the appliance
already held. Both files are written through a temp file and a rename, so an
interrupted run leaves neither file truncated. When the key write fails after
the certificate write, the command puts the previous certificate back and
reports the restore. `ze appliance init` validates and writes the same way.
<!-- source: internal/appliance/cmd_cert.go -- validateTLSPair, writeTLSPair -->
<!-- source: internal/appliance/cmd_init.go -- writeTLSSecrets -->

A `ze` older than this validation could store a certificate and a key that do
not load as a pair. On such an appliance the web listener does not start. Run
`ze doctor` to find it: the stored pair is reported as `doctor-tls-invalid`,
"certificate and key in storage are not a usable pair". `replace-cert` fixes it.
<!-- source: internal/component/doctor/checks_tls.go -- checkWebTLSPair -->

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
| `replace-cert <name>` | Replace TLS cert (regenerate or `--cert`/`--key` for CA); refuses material that is not a valid pair |
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

    ze appliance iso --check
    ze appliance kernel lab
    ze appliance kernel --profile hardware lab
    ze appliance kernel --builder qemu --arch arm64 lab
    ze appliance kernel --target runtime
    ze appliance initrd
    ze appliance iso lab

`ze appliance kernel` defaults to the installer target. `--target runtime`
builds the gokrazy runtime kernel tree and enforces the runtime requirement
floor. The installer target tries the cache, then an optional configured
artifact URL, then the local Go driver. The driver selects Docker when
available and otherwise QEMU; `--builder docker` and `--builder qemu` force one
backend. Resolved artifacts are cached under `$XDG_CACHE_HOME/ze/`.
<!-- source: internal/appliance/cmd_kernel.go -- runKernel, kernelTargetFor -->
<!-- source: internal/appliance/kernelbuilder/driver.go -- Build -->

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
variant metadata matches the appliance architecture, profile, and version.
Pass `--kernel` to select a specific installer kernel.

The kernel cache key includes target, architecture, profile, config, and
version. Repeating the same request reuses the verified artifact; a different
request gets a separate cache entry.
<!-- source: internal/appliance/cmd_kernel.go -- runKernel -->
<!-- source: internal/appliance/cmd_iso.go -- runIso, resolveISOInput, readRequiredImageChecksum -->

    ze appliance build lab
    ze appliance kernel --builder docker --profile hardware lab
    ze appliance iso lab
    ze appliance iso --image ze-20260601-120000.img lab
    ze appliance iso --output /path/to/lab.iso lab
    ze appliance iso --kernel build/kernel/Image lab
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

    ze appliance iso --target /dev/vda lab

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

    ze appliance push lab
    ze appliance push --image ze-20260427-143022.img lab
    ze appliance push --all
    ze appliance push --all --parallel 4

Push uses the update token (from `secrets/update.token`) for HTTP basic auth, and verifies the device TLS certificate against the stored `cert.pem`. No system CA pool is consulted.
<!-- source: internal/appliance/cmd_push.go -- loadDeviceTLS, authTransport -->

When `--image` is set, the file name must resolve to a regular file inside the
appliance directory. Path traversal and symlinks escaping that directory are
rejected after the local update token is read, but before TLS setup or any
network request starts.
<!-- source: internal/appliance/cmd_push.go -- resolveImagePath -->

Preview the effective configuration (base + overlay merged) without building:

    ze appliance config lab --merged

Push a config change to a running device without rebuilding the image:

    ze appliance config-push lab
    ze appliance config-push --dry-run lab
    ze appliance config-push --all
    ze appliance config-push --all --parallel 4

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

    ze appliance init --batch manifest.json

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

    ze appliance export lab
    # creates lab.ze.enc in the current directory

Export all appliances:

    ze appliance export --all
    # creates appliances-YYYYMMDD-HHMMSS.ze.enc

Import restores from an archive:

    ze appliance import lab.ze.enc

Import to a different bastion (migration):

    ze appliance import lab.ze.enc --dir /path/to/new/bastion

Archives are always encrypted using the same Argon2id + XChaCha20-Poly1305 scheme as secrets at rest. The archive passphrase can differ when provided separately from the secrets passphrase. `import --force` overwrites files present in the archive, but it does not delete extra files already present in existing appliance directories.
<!-- source: internal/appliance/crypto.go -- Encrypt, Decrypt -->
<!-- source: internal/appliance/cmd_import.go -- importArchive, extractTar -->
