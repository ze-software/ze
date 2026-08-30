# Developer Setup

<!-- source: internal/le/setup/actions.go -- Answer -->
<!-- source: internal/le/setup/actions.go -- Answer -->

Set up a Ze development environment with all build, lint, and test dependencies.

## Quick Start

```bash
git clone <repo-url> && cd ze
./le setup
```

This detects your OS (macOS with Homebrew or Debian/Ubuntu with apt), installs
missing tools, vendors Go dependencies, and reports what it did.

`le` is the native development-tool personality built from `cmd/ze`. The root
launcher executes the cached `bin/le` binary and builds it only when absent.
Setup behavior lives in `internal/le/setup.Answer`.

The cache is an existence test, never a freshness test, so a change under
`internal/le/` does not reach `./le` on its own.
<!-- source: le -- the binary existence cache -->
Delete the binary to pick the change up, and the next `./le` call rebuilds it:

```bash
rm bin/le
```

## Check Mode

Probe the current host without installing anything:

```bash
./le setup check
```

The `check` action probes only and changes nothing: it installs no package, edits no
sysctl, and adds no loopback address. It exits 0 if all required tools are
present, nonzero if any are missing. Use it as a CI preflight check. Run
`./le setup install` to install what the probe found missing.

One row is behaviour rather than a binary: `gopls-answers` runs the language
server and checks that it replies. A server on PATH that does not answer fails
this check, because every LSP call against it fails the same way.

### Editor plugin

A language server on PATH is not the whole capability. Claude Code reaches it
through a plugin, and without that plugin the LSP tool refuses every Go file.

`./le setup check` reports a missing plugin as a pending step and names the
command that installs it:

| Plugin | Serves | Install |
|--------|--------|---------|
| `gopls-lsp` | `.go` | `/plugin install gopls-lsp@claude-plugins-official` |

Run the slash command inside a Claude Code session. The plugin and the binary
it names fail the same way and have different fixes, so the report says which
of the two is absent.

## What It Installs

### Build and Lint

| Tool | Purpose |
|------|---------|
| `go` | Go toolchain |
| `git` | Version control |
| `protobuf` (`protoc`) | Protocol buffer compiler |
| `jq` | JSON processing |
| `golangci-lint` | Go linter (via `go install`) |
| `staticcheck` | Feature-tag structural type checker, pinned to 2026.2.1 (via `go install`) |
| `goimports` | Go import formatter (via `go install`) |
| `gopls` | Go language server behind the agent LSP tool (via `go install`) |

Regenerate the checked-in protobuf Go files after you change
`api/proto/ze.proto` or the module path:

```bash
./le setup proto-generate
```

The action builds both protoc plugins from the vendored module versions, runs
`protoc`, and applies explicit `json_name` options to Go struct tags.
<!-- source: internal/le/setup/proto_generate.go -- ProtoGenerator.Run -->

Run the installed checker through the repository gate:

```bash
./le staticcheck-feature-matrix check
```

The target and its checked feature population are documented in
`docs/contributing/testing.md`.

### Appliance

| Tool | Purpose |
|------|---------|
| `qemu` | QEMU functional and install gate tests |
| `e2fsprogs` | `mkfs.ext4` and `debugfs` for appliance builds |
| `xorriso` | ISO image creation |
| `grub` | GRUB EFI tooling for ISO builds (Linux only) |

### Optional

| Tool | Purpose |
|------|---------|
| `sshpass` | Optional SSH probe fallback |
| `docker` / `colima` | Container appliance and kernel builds |


## Platform Notes

### macOS

- **The Homebrew prefix is resolved, never assumed.** It is `/opt/homebrew` on
  Apple Silicon and `/usr/local` on Intel, so a hardcoded path is absent on half
  the Macs. Every consumer asks in the same order: `HOMEBREW_PREFIX` when
  `brew shellenv` has exported it, then the `brew` binary's own location
  (`<prefix>/bin/brew`), then the two documented defaults. The `brew` link is
  not followed: on Intel it points into `<prefix>/Homebrew`, which would answer
  with the wrong prefix.
- **e2fsprogs** is keg-only on Homebrew, so none of it is linked onto `PATH` and
  `which` finds nothing however well it is installed. It is looked for under
  `<prefix>/opt/e2fsprogs/sbin`, the link kept at the current version, and under
  `<prefix>/Cellar/e2fsprogs/<version>/sbin`, where an interrupted upgrade
  leaves it with no link. No PATH modification is needed after
  `brew install e2fsprogs`.
- **grub** has no first-party Homebrew formula. ISO builds require Linux or
  a container (colima/docker). The setup action skips grub on macOS.

<!-- source: internal/appliance/homebrew.go -- brewPrefixes, brewKegDirs -->
<!-- source: internal/le/setup/actions.go -- Answer -->


### Linux

`./le setup` installs the apt packages itself, the same way it installs the
Homebrew ones on macOS. Each command is echoed before it runs. It takes
`apt-get update` once per run, because a container image ships no package
lists, and it sets `DEBIAN_FRONTEND=noninteractive` so a package with a debconf
prompt cannot stop the run.

**How it reaches root.** The answer is decided before any command runs, and
`sudo` is always given `-n`, so no path can stop at a password prompt:

| State | What setup does |
|-------|-----------------|
| You are root (a container build) | Runs the command directly. `sudo` need not be installed |
| `sudo` acts with no password | Runs `sudo -n <command>` |
| `sudo` wants a password, a terminal is attached | Asks once with `sudo -v`, then runs `sudo -n <command>` |
| `sudo` wants a password, no terminal (CI, an agent session) | Prints the command, installs nothing, exits nonzero |

<!-- source: internal/le/setup/actions.go -- Answer -->
<!-- source: internal/le/setup/actions.go -- Answer -->


**GRUB follows your host architecture.** Debian packages one module set per
architecture: an amd64 host takes `grub-efi-amd64-bin`, an arm64 host takes
`grub-efi-arm64-bin`. Asking an arm64 host for the amd64 package installs
nothing at all, `grub-mkstandalone` included. `ze appliance iso` picks its GRUB
target from the architecture of the image it packs, so building an ISO for the
OTHER architecture needs that architecture's set too, through
`dpkg --add-architecture`.

<!-- source: internal/le/setup/actions.go -- Answer -->
<!-- source: internal/appliance/cmd_iso.go -- isoGRUBTarget -->


**Unprivileged user namespaces.** Ubuntu 23.10+ ships
`kernel.apparmor_restrict_unprivileged_userns=1`, which blocks the sandbox
Chrome relies on and makes the `agent-browser` web functional tests fail to
launch Chrome (`No usable sandbox!`). Setup checks this tunable as
`userns-unrestricted`. When it is restricted, `./le setup` (install mode)
echoes and then runs these commands via `sudo` to lift it globally:

```bash
echo "kernel.apparmor_restrict_unprivileged_userns = 0" | sudo tee /etc/sysctl.d/60-ze-userns.conf
sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0
```

The `/etc/sysctl.d` drop-in makes the change survive reboots. It goes through
the same root route as the package installs, so on a root run the echoed lines
carry no `sudo`, and when root is out of reach it prints the commands to run by
hand instead. `./le setup check` only reports the state, never changes it.

**KVM device access.** `/dev/kvm` is `root:kvm` mode 0660, so QEMU-backed
evidence (the appliance boot proofs and every `ze-qemu-*` target) needs your
user in the `kvm` group. Without it QEMU does not quietly fall back to
emulation: it refuses to start with `Could not access KVM kernel module:
Permission denied`, and the calling native QEMU action reports a timeout instead. Setup
checks this as `kvm-access` and, in install mode, runs:

```bash
sudo usermod -aG kvm $USER
```

<!-- source: internal/le/setup/actions.go -- Answer -->

Group membership is fixed at login, so an existing shell keeps the old groups
even after the command succeeds. Log out and back in, or run one command with
the new group:

```bash
sg kvm -c './le qemu vpp-hugepages-test'
```

Setup distinguishes the two states: `kvm-access` reports `pending` when the
group database lists you but the running session predates it, and `missing`
when the group is not granted at all. A host with no `/dev/kvm` (no hardware
virtualisation, or a VM without nested virt) reports `n/a`: QEMU runs under
`tcg` there, only slower. macOS has no `/dev/kvm` and needs no group; the
native QEMU actions select the Apple hypervisor (`hvf`) by platform.

<!-- source: internal/le/qemu/actions.go -- Answer -->

**Loopback addresses.** The functional fixtures give each end of a BGP session
its own address: RFC 4271 Section 5.1.3 forbids a peer its own address as
NEXT_HOP, so a session whose two ends share one address has every originated
route withheld. IPv4 spends 127.0.0.0/8, which Linux already routes to `lo` and
macOS does not, so setup adds 127.0.0.2 through 127.0.0.5 there. IPv6 gives a
host exactly `::1` on every platform, so setup adds `fd00::2` on both. That
address is unique-local (RFC 4193) and never globally routable, so a fixture
cannot leak a packet toward a real destination. Setup checks this as
`loopback-addresses` and, in install mode, runs:

```bash
sudo ifconfig lo0 inet6 fd00::2/128 alias      # macOS
sudo ip -6 addr add fd00::2/128 dev lo         # Linux
```

<!-- source: internal/le/setup/actions.go -- Answer -->

Presence is decided by binding a socket to the address, which is the same
question a fixture asks and a stronger one than reading the interface list: an
IPv6 address is listed while duplicate-address detection still refuses it. The
test runner cannot add either family itself (the ioctl returns EPERM
unprivileged, and the Linux route needs CAP_NET_ADMIN), so a test that binds a
missing address fails at once naming the command above.

<!-- source: internal/test/runner/loopback.go -- the runner's probe and its error -->

Neither addition survives a reboot. Re-run `./le setup` after one;
`./le setup check` says when it is needed. The merge gate adds the IPv6
address the same way, as its own workflow step (`.github/workflows/verify.yml`).

These three, and the apt installs above, are every place setup reaches for root.
All of them go through one helper, so the table of states earlier in this
section governs each of them.

## After Setup

Verify everything works:

```bash
./le verify current mode full
```

Check that appliance tools are detected:

```bash
./le setup check
```

## Drift Guard

The dev setup action and `ze doctor` appliance checks share the same tool
list. A Go test (`TestDevSetupMatchesDoctor` in
`internal/appliance/dev_setup_drift_test.go`) fails if they disagree,
preventing the lists from drifting apart.
