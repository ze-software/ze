# Spec: install-0-umbrella

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/init/main.go` - existing bootstrap command
4. `internal/plugins/dhcpserver/` - existing DHCP server
5. `internal/component/iface/discover.go` - interface discovery
6. `internal/component/iface/emit.go` - config emission

## Task

Zero-touch provisioning for ze on bare-metal hardware. `ze install serve`
runs on an existing ze device and PXE-boots target machines with a gokrazy image
containing ze. On first boot, ze enters bootstrap mode: discovers all interfaces,
runs DHCP client on each, starts SSH, and waits for operator configuration.

The goal is: rack a box, power it on, it gets ze over the network, the operator
SSHes in and configures it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/system-architecture.md` - ze init bootstrap command design
  → Decision: ze init creates zefs with SSH creds + interface discovery
  → Constraint: gokrazy appliance model, no systemd
- [ ] `internal/plugins/dhcpserver/handler.go` - existing DHCP packet handling
  → Constraint: buffer-first packet building, RFC 2131 compliance
- [ ] `internal/component/iface/discover.go` - interface discovery
  → Constraint: requires LoadBackend("netlink") before use
- [ ] `cmd/ze/init/main.go` - existing bootstrap command
  → Decision: zefs database with SSH creds, interface discovery, config emission
- [ ] `cmd/ze/main.go` lines 764-793 - first-boot startup path
  → Decision: bootstrapConfigFromTemplate or web-only fallback when no config

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2131.md` - DHCP base protocol (MUST CREATE)
  → Constraint: DORA message exchange, lease lifecycle
- [ ] `rfc/short/rfc2132.md` - DHCP options (MUST CREATE)
  → Constraint: PXE options 43, 60, 66, 67
- [ ] `rfc/short/rfc4578.md` - DHCP PXE options (MUST CREATE)
  → Constraint: option 93 client architecture, BIOS vs UEFI
- [ ] `rfc/short/rfc1350.md` - TFTP protocol (MUST CREATE)
  → Constraint: read-only, 512-byte blocks, simple stop-and-wait

**Key insights:**
- Existing dhcpserver handler has clean buffer-first packet building reusable for PXE
- `ze init` already does interface discovery + config emission + zefs creation
- First-boot path in main.go has template-based bootstrap, extensible for DHCP-all mode
- DHCP client plugin and SSH component already exist, just need automatic wiring

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dhcpserver/handler.go` - RFC 2131 DHCP server with pool/lease management
  → Constraint: PXE is additive; extend buildReply/safeAppendOption, do not fork
- [ ] `internal/plugins/dhcpserver/register.go` - plugin registration, YANG config
- [ ] `internal/plugins/dhcpserver/config.go` - config parsing: serverConfig, sharedNetwork, addressRange structs
  → Constraint: PXE config block must be parsed from same JSON tree under `dhcp-server`
- [ ] `internal/component/iface/discover.go` - DiscoverInterfaces enumerates OS NICs
- [ ] `internal/component/iface/emit.go` - EmitConfig/EmitSetConfig produce ze config syntax
- [ ] `cmd/ze/init/main.go` - ze init creates zefs with SSH creds + interface discovery
- [ ] `cmd/ze/main.go` - first-boot paths: bootstrapConfigFromTemplate, cmdStartManaged
- [ ] `internal/plugins/iface/dhcp/ifacedhcp.go` - DHCP client config struct

**Behavior to preserve:**
- Existing DHCP server plugin non-PXE behavior unchanged (PXE is additive)
- Existing `ze init` interactive/piped flow unchanged
- Existing first-boot paths (template bootstrap, managed mode) unchanged
- Interface discovery and config emission unchanged

**Behavior to change:**
- Extend dhcpserver plugin with PXE option handling (options 43, 60, 66, 67, 93)
- Add new first-boot path: when no config and no template, generate DHCP-on-all-interfaces config
- New TFTP server plugin (`internal/plugins/tftpserver/`)
- New image server plugin (`internal/plugins/imageserver/`)

## Architecture Overview

All protocol implementations live inside ze as plugins with YANG config. This means
TFTP, PXE-extended DHCP, and image serving are reusable by any ze device acting as
a provisioning server via its normal config.

Six components (four protocol pieces, one integration binary, one bootstrap mode):

| Component | What | Where |
|-----------|------|-------|
| DHCP PXE extensions | Extend existing dhcpserver plugin with PXE options | `internal/plugins/dhcpserver/` |
| TFTP server plugin | New plugin: read-only TFTP server (RFC 1350) | `internal/plugins/tftpserver/` |
| Image server plugin | New plugin: HTTP endpoint serving disk images for provisioning | `internal/plugins/imageserver/` |
| ze install subcommand | Thin CLI that generates ze config from flags, forks `ze -` with stdin pipe | `cmd/ze/install/` |
| ze bootstrap mode | First-boot behavior: DHCP on all interfaces, SSH, no BGP | Extension of existing `ze init` + startup path |
| Installer initrd | Minimal Linux that writes gokrazy image to disk | Build artifact, not Go code |

`ze install` (`cmd/ze/install/`) is a subcommand that generates a ze config
from CLI flags, forks `ze -` (self-fork via `os.Executable()`), and pipes the
config to stdin. Same pattern as `ze-chaos | ze -` but self-contained.

The installer initrd (minimal Linux that writes gokrazy image to disk) is a
build artifact, not Go code.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- PXE client DHCP Discover packet on provisioning network (UDP port 67)
- ze startup with no config in zefs (process entry point)

### Transformation Path
1. DHCP Discover with PXE option 93 arrives at `ze-install` UDP listener
2. `ze-install` DHCP handler parses options, detects PXE client architecture
3. DHCP Offer built with PXE options (bootfile, next-server) added via buffer-first encoding
4. TFTP request for bootloader: file read from configured directory, sent in 512-byte blocks
5. HTTP request for gokrazy image: `http.ServeFile` streams disk image
6. Target reboots into gokrazy+ze, ze detects no config in zefs
7. `iface.DiscoverInterfaces()` enumerates all NICs, generates DHCP-all config
8. ze starts with generated config: DHCP client on all ethernet, SSH server listening

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network -> dhcpserver plugin | UDP DHCP on port 67 (existing listener) | [ ] |
| Network -> tftpserver plugin | UDP TFTP on port 69 | [ ] |
| Network -> imageserver plugin | HTTP on configured port (own listener) | [ ] |
| Plugin -> config | YANG sections for dhcpserver (pxe block), tftpserver, imageserver | [ ] |
| ze startup -> bootstrap mode | No config detected in zefs store | [ ] |
| iface discovery -> config generation | `DiscoverInterfaces()` -> `EmitConfig()` with DHCP blocks | [ ] |

### Integration Points
- `dhcpserver` plugin - extend with PXE option encoding in `buildReply`
- `tftpserver` plugin - new plugin, registers via standard `registry.Register()`
- `imageserver` plugin - new plugin with own HTTP listener for disk images and boot files
- `iface.DiscoverInterfaces()` + `iface.EmitConfig()` - bootstrap config generation
- `zefs.Create()` / `zefs.Open()` - credential storage for pre-provisioned SSH
- `cmd/ze/main.go` startup path - new bootstrap-mode branch alongside existing template/managed paths
- Existing DHCP client plugin - activated by generated bootstrap config
- Existing SSH component - activated by generated bootstrap config

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (plugins communicate via config and bus, not direct imports)
- [ ] No duplicated functionality (PXE extends dhcpserver, does not fork it)
- [ ] Zero-copy preserved where applicable (buffer-first DHCP encoding)

### Provisioning Flow (end-to-end)

| Step | Actor | Action |
|------|-------|--------|
| 1 | Operator | Builds gokrazy image with ze, writes `ze-install` config (ze format: brace or set lines) |
| 2 | Target | PXE ROM sends DHCP Discover with option 93 (client architecture) |
| 3 | `ze-install` | DHCP Offer with IP + PXE options (next-server, bootfile) |
| 4 | Target | DHCP Request/Ack, then TFTP fetch of bootloader |
| 5 | Target | Bootloader (iPXE/GRUB) HTTP-fetches installer kernel + initrd from `ze-install` |
| 6 | Target | Installer initrd downloads gokrazy image via HTTP, writes to first disk, reboots |
| 7 | Target | Boots into gokrazy+ze. No config found. Bootstrap mode. |
| 8 | ze | Discovers all interfaces, enables DHCP client on each, starts SSH |
| 9 | Operator | SSHes into target, configures ze, commits |

### ze install Internal Flow

1. Load ze config (brace format or `set` lines): service dhcpserver with PXE, service tftpserver, service imageserver
2. Plugin startup: dhcpserver, tftpserver, imageserver start via standard ze plugin lifecycle
3. On DHCP Discover with PXE option 93 (client architecture):
   BIOS client: offer TFTP path to iPXE bootloader;
   UEFI client: offer TFTP path to iPXE EFI or GRUB EFI bootloader
4. TFTP request: tftpserver plugin serves bootloader from configured directory
5. HTTP request: imageserver plugin streams gokrazy disk image
6. Bootloader chain-loads installer kernel+initrd via HTTP
7. Installer initrd: download image, write to first disk, reboot

### ze Bootstrap Flow

1. ze starts, zefs exists (written by installer initrd) but no config and no template
2. Discover all interfaces via `iface.DiscoverInterfaces()`
3. Generate config with DHCP client enabled on every ethernet interface
4. Start ze with this config: DHCP client runs, SSH server starts
5. Operator SSHes in, configures, commits
6. ze restarts in normal mode

## Child Specs

| Spec | Scope | Depends |
|------|-------|---------|
| `spec-install-1-dhcp-pxe.md` | Extend dhcpserver plugin with PXE options (43, 60, 66, 67, 93) | - |
| `spec-install-2-tftpserver.md` | New tftpserver plugin (RFC 1350, read-only, YANG config) | - |
| `spec-install-3-image-server.md` | New imageserver plugin (own HTTP listener, disk images + boot files) | - |
| `spec-install-4-ze-install-binary.md` | `ze install` subcommand: config gen + fork `ze -` | spec-install-1, 2, 3 |
| `spec-install-5-bootstrap-mode.md` | ze first-boot: auto-init, DHCP-all, SSH-only mode | - |
| `spec-install-6-installer-initrd.md` | Build system for the minimal installer image | spec-install-4 |

## Component 1 (spec-install-1): DHCP PXE Extensions

### Extend Existing Plugin

The dhcpserver plugin (`internal/plugins/dhcpserver/`) already implements RFC 2131
with pool management, lease tracking, and buffer-first packet building. PXE support
is additive: new options in the DHCP Offer/Ack, new matching logic on Discover.

PXE-specific DHCP options to add:

| Option | RFC | Purpose |
|--------|-----|---------|
| 43 (Vendor-Specific) | RFC 2132 | PXE boot menu, boot item |
| 60 (Class-Identifier) | RFC 2132 | PXE client detection ("PXEClient:Arch:...") |
| 66 (TFTP Server Name) | RFC 2132 | Next-server hostname/IP |
| 67 (Bootfile Name) | RFC 2132 | Path to bootloader binary |
| 93 (Client System Architecture) | RFC 4578 | BIOS vs UEFI detection |
| 94 (Client Network Interface) | RFC 4578 | NIC identifier |
| 97 (Client Machine Identifier) | RFC 4578 | UUID |

### YANG Config Addition

Add a `pxe` container under the dhcpserver YANG schema:

| Leaf | Type | Purpose |
|------|------|---------|
| `pxe/enabled` | boolean | Enable PXE option injection |
| `pxe/tftp-server` | inet:ipv4-address | Next-server IP (option 66) |
| `pxe/bootfile-bios` | string | Bootfile for BIOS clients (option 67) |
| `pxe/bootfile-uefi` | string | Bootfile for UEFI clients (option 67) |

When `pxe/enabled` is true and a DHCP Discover contains option 60 starting with
"PXEClient:", the handler reads option 93 (client architecture), selects the
appropriate bootfile, and adds options 66/67 to the Offer/Ack.

### Implementation

Extend `dhcpHandler.buildReply()` to:
1. Set `siaddr` (bytes 20-23 of DHCP header) to the PXE TFTP server IP. Some PXE
   ROMs only check `siaddr` and ignore option 66. The existing code already writes
   `serverIP` into `siaddr` at `handler.go:225-226`; for PXE, override with the
   configured `pxe/tftp-server` address.
2. Append PXE options 66/67 via existing `safeAppendOption()`.

Add `parsePXEArch()` to read option 93 from the Discover, and `isPXEClient()` to
check option 60 for the "PXEClient:" prefix.

## Component 2 (spec-install-2): TFTP Server Plugin

### New Plugin

`internal/plugins/tftpserver/` following standard plugin registration pattern.
Read-only TFTP server per RFC 1350.

| File | Purpose |
|------|---------|
| `register.go` | Plugin registration, YANG schema, `RunEngine` |
| `handler.go` | TFTP packet handling: RRQ, DATA, ACK, ERROR |
| `schema/` | YANG schema for tftpserver config |

### YANG Config

| Leaf | Type | Purpose |
|------|------|---------|
| `enabled` | boolean | Enable/disable TFTP server |
| `listen-interface` | leafref | Interface to bind (or all) |
| `root-directory` | string | Directory to serve files from |

### Protocol

RFC 1350: read-only, 512-byte data blocks, stop-and-wait ACK.
Simple enough to implement directly (no external dependencies).
Write requests (WRQ) are rejected with ERROR packet.

## Component 3 (spec-install-3): Image Server Plugin

### New Plugin

`internal/plugins/imageserver/` following standard plugin registration pattern.
Separate from the web component: own HTTP listener on a configurable port.
Image serving should not affect web UI performance, and has a different lifecycle
(can be enabled without the web UI).

| Path | Content |
|------|---------|
| `/install/image/<name>` | Gokrazy disk image |
| `/install/boot/vmlinuz` | Installer kernel |
| `/install/boot/initrd` | Installer initrd |
| `/install/boot/ipxe.cfg` | iPXE script (chain-loads kernel+initrd via HTTP) |

Uses `http.ServeFile` for Range request support.

### YANG Config

| Leaf | Type | Purpose |
|------|------|---------|
| `enabled` | boolean | Enable image server |
| `listen-interface` | leafref | Interface to bind |
| `listen-port` | uint16 | HTTP port (default 80, standard HTTP for PXE clients that have no config) |
| `image-directory` | string | Directory containing disk images |
| `boot-directory` | string | Directory containing installer kernel/initrd |
| `ssh-username` | string | Admin username for installed target (written to served zefs) |
| `ssh-password-hash` | string | bcrypt hash of admin password (written to served zefs) |

### Provisioning Tracking

The dhcpserver plugin already tracks leases. PXE provisioning events can be
logged via the bus (new event type) for observability. Optional "provision once"
mode: dhcpserver stops offering PXE options to MACs that have completed installation.

## Component 4 (spec-install-4): ze install Subcommand

### Architecture

`ze install` is a subcommand of ze at `cmd/ze/install/`. It generates a ze config
string from CLI flags, finds its own binary via `os.Executable()`, forks
`ze -` (exec.Command with StdinPipe), and pipes the config + NUL sentinel to
stdin. Same pattern as `ze-chaos | ze -` but internal: one command starts
everything. No hub.Run() import, no ephemeral zefs creation.

The same provisioning can be achieved with `ze` and a hand-written config.
`ze install serve` makes the common case trivial:

`ze install serve --interface eth0 --image /path/to/gokrazy.img --network 192.168.1.0/24`

The server's own IP is derived from the interface address (first IPv4 address on
the interface). An explicit `--address` flag overrides this. The derived IP is
used for DHCP `siaddr`, option 54 (server-identifier), `default-router`, and
the TFTP/HTTP server addresses in PXE options.

### CLI to Config Mapping

The binary translates CLI flags into a ze config that enables:
- dhcpserver with PXE on the specified interface and network
- tftpserver on the same interface, serving from an auto-created temp directory
- imageserver on the same interface, serving the specified image

Additionally, `--ssh-username` and `--ssh-password` (required) specify the admin
credentials for the installed target. These are bcrypt-hashed and written into the
zefs database served by imageserver at `/install/database.zefs`.

### Sample Generated Config (brace format)

```
service {
    dhcp-server {
        listen-interface eth0;
        shared-network install {
            subnet 192.168.1.0/24 {
                range pool1 {
                    start 192.168.1.100;
                    stop 192.168.1.200;
                }
                default-router 192.168.1.1;
            }
        }
        pxe {
            enabled true;
            tftp-server 192.168.1.1;
            bootfile-bios ipxe.pxe;
            bootfile-uefi ipxe.efi;
        }
    }
    tftp-server {
        enabled true;
        listen-interface eth0;
        root-directory /var/lib/ze-install/tftp;
    }
    image-server {
        enabled true;
        listen-interface eth0;
        image-directory /var/lib/ze-install/images;
        boot-directory /var/lib/ze-install/boot;
        ssh-username admin;
        ssh-password-hash "$2a$10$...";
    }
}
```

### Sample Config (set format)

```
set service dhcp-server listen-interface eth0
set service dhcp-server shared-network install subnet 192.168.1.0/24 range pool1 start 192.168.1.100
set service dhcp-server shared-network install subnet 192.168.1.0/24 range pool1 stop 192.168.1.200
set service dhcp-server shared-network install subnet 192.168.1.0/24 default-router 192.168.1.1
set service dhcp-server pxe enabled true
set service dhcp-server pxe tftp-server 192.168.1.1
set service dhcp-server pxe bootfile-bios ipxe.pxe
set service dhcp-server pxe bootfile-uefi ipxe.efi
set service tftp-server enabled true
set service tftp-server listen-interface eth0
set service tftp-server root-directory /var/lib/ze-install/tftp
set service image-server enabled true
set service image-server listen-interface eth0
set service image-server image-directory /var/lib/ze-install/images
set service image-server boot-directory /var/lib/ze-install/boot
set service image-server ssh-username admin
set service image-server ssh-password-hash "$2a$10$..."
```

## Component 5 (spec-install-5): ze Bootstrap Mode

### Existing Building Blocks

| Feature | Exists | Where |
|---------|--------|-------|
| Interface discovery | Yes | `iface.DiscoverInterfaces()` in `internal/component/iface/discover.go` |
| Config emission from discovery | Yes | `iface.EmitConfig()` in `internal/component/iface/emit.go` |
| `ze init` (create zefs, creds, discovery) | Yes | `cmd/ze/init/main.go:Run()` |
| Bootstrap from template | Yes | `cmd/ze/main.go:bootstrapConfigFromTemplate()` |
| DHCP client plugin | Yes | `internal/plugins/iface/dhcp/` |
| SSH server component | Yes | `internal/component/ssh/` |
| First-boot managed mode | Yes | `cmd/ze/main.go:cmdStartManaged()` |

### What Needs to Change

The existing first-boot path (`bootstrapConfigFromTemplate`) assumes a template
exists in zefs. The new bootstrap mode needs a "no template, no config, just
make it reachable" path:

1. **Detect bootstrap condition**: zefs exists (written by installer initrd) but has neither
   `file/active/ze.conf` nor `file/template/ze.conf`. No new marker needed.
   This extends the existing startup path in `cmd/ze/main.go` lines 764-774
   which already checks for config and template existence.

2. **Generate bootstrap config**: new function `EmitBootstrapConfig()` in
   `internal/component/iface/emit.go` that calls `EmitConfig()` for interface
   stanzas then adds DHCP client config inside each ethernet interface block.
   DHCP client is per-interface in the YANG tree: `interface/<type>/<name>/unit
   default/ipv4/dhcp/enabled true`. The emitted config looks like:

   ```
   interface {
       ethernet eth0 {
           os-name eth0;
           unit default {
               ipv4 {
                   dhcp {
                       enabled true;
                   }
               }
           }
       }
   }
   ```

   `EmitConfig()` itself stays unchanged, avoiding regression risk in
   `ze init` and `ze interface scan --config`.

3. **Start in limited mode**: no BGP, no web, no plugins beyond iface-dhcp.
   SSH + DHCP client only. SSH is enabled because the bootstrap config includes
   an SSH block, and credentials are already in zefs (written by the installer
   initrd from the pre-provisioned zefs database). The bootstrap config emits:

   ```
   environment {
       ssh {
           enabled true;
       }
   }
   ```

   The SSH server reads username/password from zefs at startup, so no
   credentials need to appear in the config file itself.

4. **Exit bootstrap**: when the operator commits a real config via SSH, ze's
   existing config commit/reload mechanism applies the new config. The bootstrap
   config is replaced by the committed config in zefs. On next restart, ze finds
   a real config and starts in normal mode. No special bootstrap exit logic needed.

### SSH Credential Flow

The `ze-install` config includes admin credentials for the target installation.
These are written into a zefs database that the installer initrd places on the
target's /perm partition. SSH on the target reads creds from zefs at startup
(`cmd/ze/hub/main_servers.go:491-503` reads `KeySSHUsername`/`KeySSHPassword`
from zefs). The host key is auto-generated on first boot if not present in zefs.

| Mode | How | Credential source |
|------|-----|-------------------|
| Pre-provisioned | Installer initrd writes gokrazy image + zefs to disk. zefs contains admin username + bcrypt password hash. | `ze-install` config `ssh-username` / `ssh-password-hash` fields |

The `ze-install` config must include:

| Field | Purpose |
|-------|---------|
| `ssh-username` | Admin username for the installed ze (stored in zefs) |
| `ssh-password-hash` | bcrypt hash of admin password (stored in zefs) |

These are written into the zefs database served by the imageserver at
`/install/database.zefs`. The installer initrd downloads this and writes it to
`/perm/ze/database.zefs` on the target.

### Gokrazy Partition Layout

| Partition | Mount | Content |
|-----------|-------|---------|
| 1 | /boot | Firmware, kernel, cmdline |
| 2 | / (A) | Read-only root with Go binaries |
| 3 | / (B) | A/B update slot |
| 4 | /perm | Persistent user data (ext4) |

The zefs database lives on /perm. The installer initrd flow for pre-provisioned:
1. Write gokrazy image to disk
2. Mount partition 4 (/perm)
3. Download zefs database from `ze-install` HTTP endpoint
4. Write to `/perm/ze/database.zefs`
5. Unmount, reboot

## Component 6 (spec-install-6): Installer Initrd

Out of scope for Go implementation. This is a Linux build artifact.

### Requirements

- Minimal Linux environment (u-root, Alpine initramfs, or custom)
- HTTP client (wget/curl or Go binary)
- Block device write (`dd` equivalent)
- Knows the ze-install server IP (from DHCP, passed via kernel cmdline)
- Flow: download image, write to first available non-removable disk, reboot

### Build System

A Makefile target or script that:
1. Takes a gokrazy disk image as input
2. Builds the installer initrd with the download+write logic
3. Packages the TFTP bootloader + kernel + initrd for `ze-install` to serve

Could use u-root (Go-based initramfs tooling) for a pure-Go solution,
or a minimal Alpine-based initrd with busybox.

## Design Decisions

| Decision | Chosen | Over | Reason |
|----------|--------|------|--------|
| Protocols as ze plugins | YANG-configured plugins | Standalone binary code | Reusable: ze device can serve as provisioning server via normal config |
| Config format | ze format (brace + set lines) | TOML/KDL/standalone | Consistent across all ze programs; single parser, both syntaxes |
| PXE as dhcpserver extension | Extend existing plugin | New PXE-specific DHCP server | No duplication; PXE is additive options on standard DHCP |
| TFTP as separate plugin | `internal/plugins/tftpserver/` | Embed in dhcpserver | Different protocol, different config, different lifecycle |
| Image server as plugin | `internal/plugins/imageserver/` | Web component endpoint | Own HTTP listener, isolation from web UI, independent lifecycle |
| `ze install` subcommand | `ze install` generates config, forks `ze -` with stdin pipe | Separate binary calling hub.Run() | No hub import, no ephemeral zefs. ze-chaos pattern, proven. Single binary. |
| Bootstrap detection | No config AND no template in zefs | Explicit bootstrap marker | Extends existing startup check, no new state to manage |
| Bootstrap config | New `EmitBootstrapConfig()` wrapping `EmitConfig()` | Modify `EmitConfig()` directly | Avoids regression risk in `ze init` and `ze interface scan` |
| Zefs injection | Installer initrd downloads zefs from imageserver, writes to /perm | Embed creds in gokrazy image | Clean separation: image is generic, creds are per-deployment |
| ze install server storage | ze handles its own storage via stdin config path | Ephemeral zefs with hub.Run() | Fork pattern: ze install is a launcher, ze handles storage |
| PXE boot with initrd | Option A (initrd writes image) | Option B (kexec into gokrazy) | Gokrazy owns the partition table. Clean write is safer than live migration. |
| DHCP approach | Full DHCP server | Proxy DHCP | PXE network is isolated. Proxy mode can be added later. |
| TFTP scope | Bootloader only | Full file serving | TFTP is slow. Kernel, initrd, and image go over HTTP. |
| Bootstrap SSH creds | Pre-provisioned only (creds in ze-install config) | Auto-generate on first boot | Creds must be known to the operator; auto-generate requires console access. |
| Disk target | First non-removable disk | Configurable | Simple for v1. Operator controls hardware. |

## Existing Code Reuse

| Existing | Reuse in |
|----------|----------|
| `dhcpserver/handler.go` buildReply, safeAppendOption | PXE extension: add options to existing reply builder |
| `dhcpserver/register.go` plugin lifecycle | PXE: same plugin, extended YANG schema |
| `dhcpserver/config.go` config parsing | PXE: add pxe block to existing config struct |
| `iface.DiscoverInterfaces()` + `iface.EmitConfig()` | Bootstrap mode: generate DHCP-enabled config for all interfaces |
| `ze init` zefs creation logic (`zefs.Create`, `KeySSH*` writes) | `ze-install` builds target zefs with admin creds for imageserver to serve |
| SSH component | Bootstrap mode: already starts when config has SSH enabled |
| DHCP client plugin | Bootstrap mode: already runs when config has DHCP on interfaces |
| Plugin registration pattern | tftpserver: same `registry.Register()` pattern |
| Plugin registration pattern | imageserver: same `registry.Register()` pattern |

## RFC Summaries Needed

| RFC | Topic | Status |
|-----|-------|--------|
| RFC 2131 | DHCP | Used in code (dhcpserver), summary needed in `rfc/short/` |
| RFC 2132 | DHCP Options | Used in code, summary needed |
| RFC 4578 | DHCP PXE Options | New, summary needed |
| RFC 1350 | TFTP | New, summary needed |
| RFC 5970 | DHCPv6 PXE (UEFI HTTP Boot) | Future, not for v1 |

## Security Considerations

| Concern | Mitigation |
|---------|------------|
| DHCP on untrusted network | `ze-install` runs only on isolated provisioning networks |
| SSH credentials in image | Pre-provisioned creds are bcrypt-hashed (same as `ze init`). Operator changes password after first login. |
| TFTP has no auth | TFTP serves only the bootloader (public, no secrets). Image goes over HTTP. |
| Image integrity | Future: SHA256 checksum verification in the installer initrd |
| Rogue PXE server | Out of scope for v1. Operator controls the provisioning network. |
| Privileged ports | DHCP (67), TFTP (69), HTTP (80) require root on Linux. On gokrazy ze runs as root. On other Linux hosts, `ze-install` must run as root. |

## Implementation Order

1. **spec-install-1: DHCP PXE extensions** -- extend dhcpserver with PXE options.
   Smallest change, builds on existing working plugin.

2. **spec-install-2: TFTP server plugin** -- new plugin, independent of PXE.
   Can be tested standalone. Useful beyond provisioning.

3. **spec-install-3: image server** -- new imageserver plugin with own HTTP listener.
   Independent of web component, follows standard plugin registration pattern.

4. **spec-install-4: ze install subcommand** -- config gen + fork `ze -`.
   Depends on 1-3 being functional.

5. **spec-install-5: bootstrap mode** -- ze first-boot changes. Mostly wiring existing
   building blocks (interface discovery + DHCP client + SSH) into an automatic path.

6. **spec-install-6: installer initrd** -- build system for the minimal Linux image.
   Depends on ze install being functional.

## Scope Boundaries (v1)

| Limitation | v1 behavior | Future |
|------------|-------------|--------|
| Disk selection | First non-removable disk (`/dev/sda` or `/dev/nvme0n1`) | Configurable via kernel cmdline |
| DHCP coexistence | Full DHCP server, assumes isolated provisioning network | Proxy DHCP mode alongside existing DHCP |
| Image count | One image, all targets get the same | Per-MAC image selection |
| Post-install hook | None, same image for all | Per-device hostname, zefs, post-install script |

## Wiring Test (MANDATORY -- NOT deferrable)

Umbrella spec. Wiring tests defined in child specs.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| DHCP Discover with PXE option 60 | → | dhcpserver PXE reply with options 66/67 | `spec-install-1` |
| TFTP RRQ packet | → | tftpserver file read + DATA response | `spec-install-2` |
| HTTP GET `/install/image/<name>` | → | imageserver plugin serves file | `spec-install-3` |
| `ze install serve` CLI | -> | fork `ze -` with generated install config | `spec-install-4` |
| ze startup (no config, no template) | → | Bootstrap mode DHCP-all + SSH | `spec-install-5` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze install serve` with valid flags | DHCP (with PXE), TFTP, and HTTP servers listen on configured interface |
| AC-2 | PXE client sends DHCP Discover with option 93 | `ze-install` responds with Offer containing bootfile path matching client architecture (BIOS/UEFI) |
| AC-3 | PXE client TFTP-fetches bootloader | Bootloader served from `ze-install` TFTP directory |
| AC-4 | Bootloader HTTP-fetches installer kernel+initrd | Files served from `ze-install` HTTP server |
| AC-5 | Installer initrd runs on target | Downloads gokrazy image via HTTP, writes to first disk, reboots |
| AC-6 | ze starts on target with zefs but no config | Bootstrap mode: discovers interfaces, DHCP client on all ethernet, SSH starts |
| AC-7 | Operator SSHes to bootstrapped target | SSH accessible on DHCP-acquired addresses, CLI available for configuration |
| AC-8 | `ze install serve` with --ssh-username and --ssh-password | SSH creds bcrypt-hashed and embedded in generated config |
| AC-9 | Installer initrd fetches `/install/database.zefs` | zefs written to target's /perm/ze/database.zefs, ze boots with working SSH |

## 🧪 TDD Test Plan

### Unit Tests

Umbrella spec. Detailed test plans in child specs.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPXEOptionEncoding` | `internal/plugins/dhcpserver/handler_test.go` | PXE options 66/67/93 correctly encoded | |
| `TestTFTPReadRequest` | `internal/plugins/tftpserver/handler_test.go` | Bootloader file served over TFTP | |
| `TestImageServerZefsEndpoint` | `internal/plugins/imageserver/handler_test.go` | `/install/database.zefs` contains correct SSH creds from config | |
| `TestEmitBootstrapConfig` | `internal/component/iface/emit_test.go` | DHCP-on-all config emitted from discovery | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| DHCP IP range | 1-254 host part | .254 | .0 (network) | .255 (broadcast) |
| TFTP block number | 1-65535 | 65535 | 0 (invalid) | N/A (wraps) |
| TFTP data block | fixed 512 bytes (RFC 1350) | 512 | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ze-install-dhcp-pxe` | `test/install/dhcp-pxe.ci` | PXE client gets DHCP offer with bootfile path and correct siaddr | |
| `test-ze-install-tftp-boot` | `test/install/tftp-boot.ci` | TFTP client fetches bootloader file from tftpserver | |
| `test-ze-install-http-image` | `test/install/http-image.ci` | HTTP client downloads gokrazy image from imageserver | |
| `test-ze-install-http-zefs` | `test/install/http-zefs.ci` | HTTP client downloads database.zefs with pre-provisioned SSH creds | |
| `test-ze-bootstrap-ssh` | `test/install/bootstrap-ssh.ci` | Fresh ze enters bootstrap mode, SSH reachable with provisioned creds | |

### Future (if deferring any tests)
- QEMU-based PXE boot integration test (requires QEMU with PXE ROM)
- Multi-image serving test (v2 feature)

## Files to Modify

Umbrella spec. Detailed file lists in child specs.

- `internal/plugins/dhcpserver/` - PXE extensions (spec-install-1)
- `internal/plugins/dhcpserver/schema/` - YANG additions for PXE config (spec-install-1)
- `internal/plugins/imageserver/` - new image server plugin (spec-install-3)
- `cmd/ze/install/` - new subcommand (spec-install-4), `cmd/ze/main.go` dispatch
- `cmd/ze/main.go` - add bootstrap-mode path (spec-install-5)
- `internal/component/iface/emit.go` - extend EmitConfig for DHCP blocks (spec-install-5)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (dhcpserver PXE) | Yes | `internal/plugins/dhcpserver/schema/` |
| YANG schema (tftpserver) | Yes | `internal/plugins/tftpserver/schema/` |
| YANG schema (imageserver) | Yes | `internal/plugins/imageserver/schema/` |
| CLI commands/flags | Yes | `cmd/ze/install/main.go` |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Plugin registration | Yes | `internal/plugins/tftpserver/register.go`, `internal/plugins/imageserver/register.go` |
| Plugin all.go import | Yes | `make generate` (all.go is code-generated by `scripts/codegen/plugin_imports.go`) |
| Functional test for new RPC/API | Yes | `test/install/*.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - ze-install provisioning |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - ze-install |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - tftpserver, imageserver plugins; dhcpserver PXE extension |
| 6 | Has a user guide page? | Yes | `docs/guide/ze-install.md` - provisioning guide |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc4578.md` - PXE options |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create

- `internal/plugins/tftpserver/register.go` - plugin registration + YANG + RunEngine
- `internal/plugins/tftpserver/handler.go` - TFTP packet handling (RRQ, DATA, ACK, ERROR)
- `internal/plugins/tftpserver/handler_test.go` - TFTP unit tests
- `internal/plugins/tftpserver/schema/` - YANG schema for tftpserver
- `internal/plugins/imageserver/register.go` - plugin registration + YANG + RunEngine
- `internal/plugins/imageserver/handler.go` - HTTP file serving with Range support
- `internal/plugins/imageserver/handler_test.go` - image server unit tests
- `internal/plugins/imageserver/schema/` - YANG schema for imageserver
- `cmd/ze/install/main.go` - subcommand: generates ze config from CLI flags, forks `ze -`
- `test/install/dhcp-pxe.ci` - PXE DHCP functional test
- `test/install/tftp-boot.ci` - TFTP bootloader fetch functional test
- `test/install/http-image.ci` - image server functional test
- `test/install/http-zefs.ci` - zefs download functional test
- `test/install/bootstrap-ssh.ci` - bootstrap mode functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + child specs |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table per child spec |
| 4. Implement (TDD) | Child specs in order: install-1, install-2, install-3 |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test` |
| 7. Critical review | Critical Review Checklist |
| 8-13. Fix/verify loop | Per child spec |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- PXE option stubs in dhcpserver, tftpserver skeleton
   - Tests: dhcpserver PXE config parses, tftpserver registers
   - Files: `dhcpserver/handler.go`, `tftpserver/register.go`
   - Verify: plugins load with new YANG config, PXE stub returns empty

2. **Phase: DHCP PXE extensions** -- PXE option encoding in dhcpserver
   - Tests: PXE option encoding unit tests, DHCP Discover/Offer with PXE round-trip
   - Files: `dhcpserver/handler.go`, `dhcpserver/config.go`, `dhcpserver/schema/`
   - Verify: unit tests pass, DHCP handler adds PXE options when configured

3. **Phase: TFTP server** -- read-only TFTP plugin
   - Tests: TFTP RRQ/DATA/ACK unit test, WRQ rejected
   - Files: `tftpserver/handler.go`, `tftpserver/register.go`, `tftpserver/schema/`
   - Verify: TFTP serves file from configured directory

4. **Phase: Image server** -- imageserver plugin with own HTTP listener
   - Tests: HTTP GET image endpoint, Range request support, zefs endpoint
   - Files: `imageserver/register.go`, `imageserver/handler.go`, `imageserver/schema/`
   - Verify: image served via HTTP with correct Content-Type, zefs download works

5. **Phase: ze install subcommand** -- config gen + fork pattern
   - Tests: subcommand dispatches, forks ze with install config
   - Files: `cmd/ze/install/main.go`, `cmd/ze/install/serve.go`, `cmd/ze/main.go`
   - Verify: `ze install serve` starts DHCP+PXE, TFTP, HTTP via forked ze

6. **Phase: Bootstrap mode** -- ze first-boot DHCP-all + SSH
   - Tests: bootstrap config generation test, bootstrap detection test
   - Files: `cmd/ze/main.go`, `internal/component/iface/emit.go`
   - Verify: ze enters bootstrap mode when no config, SSH reachable

7. **Functional tests** -- create after feature works
8. **RFC refs** -- add RFC comments for DHCP/PXE/TFTP code
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | PXE option values match RFC 4578, DHCP complies with RFC 2131 |
| Naming | YANG leaves use kebab-case, Go types follow ze conventions |
| Data flow | DHCP->TFTP->HTTP chain works end-to-end |
| Rule: buffer-first | DHCP/TFTP packet building uses append, no fmt.Sprintf on wire path |
| Rule: registration | tftpserver uses standard registry.Register(), blank import in all.go |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| dhcpserver PXE options in Offer | Unit test with PXE option 93 in Discover |
| tftpserver serves file | Unit test with TFTP RRQ |
| HTTP serves image | Unit test with HTTP GET `/install/image/` |
| `ze install serve -h` prints usage | Run ze binary and check output |
| Bootstrap mode activates | Unit test: no config triggers DHCP-all config |
| YANG schemas validate | `make ze-lint` passes with new schemas |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | DHCP packets from network: validate lengths, option bounds before parsing |
| Path traversal | TFTP/HTTP file serving: reject paths outside configured directory |
| Resource exhaustion | Limit concurrent DHCP/TFTP/HTTP connections |
| Credential handling | SSH password hash in config: never log plaintext, bcrypt only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

## Implementation Summary

### What Was Implemented
- [Umbrella: see child specs]

### Bugs Found/Fixed
- [None yet]

### Documentation Updates
- [None yet]

### Deviations from Plan
- [None yet]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`cmd/ze/install/*` + `cmd/ze/main.go` dispatch)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
