# Developer Setup

<!-- source: internal/le/setup/actions.go -- Answer -->
<!-- source: internal/le/setup/actions.go -- Answer -->

Set up a Ze development environment with all build, lint, and test dependencies.

## Quick Start

```bash
git clone <repo-url> && cd ze
./le setup install
```

This detects your OS (macOS with Homebrew or Debian/Ubuntu with apt), installs
missing tools, vendors Go dependencies, and reports what it did.

`le` is the native development-tool personality built from `cmd/ze`. The root
launcher executes the cached `bin/le` binary and builds it only when absent.
Setup behavior lives in `internal/le/setup.Answer`.

The cache is an existence test, never a freshness test, so a change under
`internal/le/` does not reach `./le` on its own.
<!-- source: le -- the binary existence cache -->

The launcher does not rebuild by itself, because a peer session's half-written
source would then fail every call in every session. It tells you instead. About
one call in sixteen compares the binary with the build inputs and prints one
line on stderr when a COMMITTED file is newer:

```
le: bin/le is older than committed sources; refresh it with './le --update'
```

A file that git holds as modified is never counted, so a peer's work in progress
says nothing. The check runs after your command, so it never delays the answer
you asked for.
<!-- source: le -- warn_when_stale, committed_change_is_newer -->

`./le --update` builds the working tree into `bin/le`:

```bash
./le --update
```

It renames the new binary into place rather than writing through the file. A peer
executing the old one therefore keeps the inode it started with. A failed build
leaves `bin/le` as it was and says so. With a command after it, the option
updates first and then runs that command against the new binary.

`--update` is the answer to a stale shared binary. `--name` below is the answer
to reading a result back from your own uncommitted edits.
<!-- source: le -- update_le -->

Do not delete `bin/le`. Several sessions share one checkout and one of them can
be executing that file right now. Ask for a build of your own instead:

```bash
./le --name mywork verify current mode full
```

`--name` comes first, before the command, and the launcher consumes it. Every
argument after it reaches the binary unchanged. The build lands in
`bin/le-mywork/le`, which `bin/` already keeps out of git, and it runs on every
call, so the toolchain decides what to recompile and the binary carries your
edits. The shared `bin/le` is never written and never waits for a peer.
<!-- source: le -- the --name option -->

A name accepts letters, digits, dot, underscore and hyphen. A name carrying `/`
or `..` is refused with a message and exit 2, never repaired into something safe.

The launcher exports the name to the process it runs and to that process's
children, so a script that calls `./le` inside a named session reaches the same
build without repeating the option. A binary that is not the one you named
refuses to answer and reports both names:

```
le: --name asked for 'mywork' and this process is 'le' (/home/you/ze/bin/le).
    Reach the named build through ./le --name mywork, never through a hardcoded binary path.
```
<!-- source: cmd/ze/le_build_name.go -- refuseWrongBuildName -->

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
| `govulncheck` | Dependency vulnerability scanner for the verification gate, built from the vendored copy (via `go install`) |

The linter's module dependency and setup install target move together. Use the
pinned release: its exhaustive analyzer checks enum switches through type
aliases as well as directly named types. An existing `golangci-lint` on PATH is
only presence-probed by setup, so changing the pin alone does not replace that
binary.
<!-- source: internal/le/setup/tools.go -- GolangCIVersion, golangciTarget -->
<!-- source: internal/le/setup/probes.go -- Setup.Probe -->

Regenerate the checked-in protobuf Go files after you change
`api/proto/ze.proto` or the module path:

```bash
./le setup proto-generate
```

The action builds both protoc plugins from the vendored module versions, runs
`protoc`, and applies explicit `json_name` options to Go struct tags.
<!-- source: internal/le/setup/proto_generate.go -- protoGenerator.run -->

Run the installed checker through the repository gate:

```bash
./le go staticcheck check
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

### Docker labs

A Docker lab runs Ze in containers, so the kernel it tests is the Docker host's,
and that kernel has to carry every feature Ze enrolls. A lab is never judged on a
kernel that lacks one.

- **macOS:** the labs run inside the Alpine QEMU guest booted on Ze's own arm64
  runtime kernel under HVF, never in colima or Docker Desktop, whose Linux VMs
  lack features Ze enrolls. Build the kernel cache entry once with
  `./ze appliance kernel --target runtime --arch arm64`, then run a lab as
  `./le test qemu docker-lab lab "test integration interop-ipsec"`. With no
  `lab` the action stops after the guest's Docker kernel check.
- **Linux:** the labs run on the host's own Docker, on a kernel that carries
  every registered feature. `./le setup docker-kernel check` asks the
  Docker daemon in hand and names each missing feature. It cross-builds the
  linux `ze` it probes with for the daemon's architecture
  (`tmp/qemu/linux-<arch>/ze`); `ze <path>` names another, which must be a
  linux executable for that architecture.
  `./le setup docker-kernel install` puts Ze's cached runtime kernel under
  `/boot` and makes it GRUB's default, one stated `sudo` step at a time. It
  needs `GRUB_DEFAULT=saved` in `/etc/default/grub`, never reboots, and refuses
  on macOS. When the daemon applies AppArmor (Ubuntu's default), the check runs its
  probe under Ze's `ze-kernel-probe` profile, because Docker's `docker-default`
  denies what the probe does; `./le setup docker-kernel apparmor confirm
  ze-kernel-probe` installs and loads it, one stated `sudo` step at a time. The
  VRRP interop scenarios run ze under `ze-lab-vrrp` the same way, loaded with
  `./le setup docker-kernel apparmor confirm ze-lab-vrrp`; the action with no
  `confirm` lists every profile and the steps each one runs.

`docs/architecture/testing/qemu-integration.md`, "Docker labs in the Ze-kernel
guest", is the guest path; the scheduled nightly runs the same path on amd64
under KVM (`docs/architecture/testing/ci-workflows.md`).

<!-- source: internal/le/test/qemu/dockerlab.go -- runDockerLabHere, dockerLabKernel -->
<!-- source: internal/le/setup/dockerkernel.go -- runDockerKernelCheck, runDockerKernelInstall -->

<!-- source: internal/appliance/homebrew.go -- brewPrefixes, brewKegDirs -->
<!-- source: internal/le/setup/actions.go -- Answer -->


### Linux

`./le setup install` installs the apt packages itself, the same way it installs the
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
`userns-unrestricted`. When it is restricted, `./le setup install`
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
sg kvm -c './le test qemu vpp-hugepages-test'
```

Setup distinguishes the two states: `kvm-access` reports `pending` when the
group database lists you but the running session predates it, and `missing`
when the group is not granted at all. A host with no `/dev/kvm` (no hardware
virtualisation, or a VM without nested virt) reports `n/a`: QEMU runs under
`tcg` there, only slower. macOS has no `/dev/kvm` and needs no group; the
native QEMU actions select the Apple hypervisor (`hvf`) by platform.

<!-- source: internal/le/test/qemu/actions.go -- Answer -->

**Loopback addresses.** The functional fixtures give each end of a BGP session
its own address: RFC 4271 Section 5.1.3 forbids a peer its own address as
NEXT_HOP, so a session whose two ends share one address has every originated
route withheld. IPv4 spends 127.0.0.0/8, which Linux already routes to `lo` and
macOS does not, so setup adds 127.0.0.2 through 127.0.0.5 there. IPv6 gives a
host exactly `::1` on every platform. Setup adds `fd00::2/128` for the speaker
and next-hop owner, `fd00::3/127` for the recipient, and `fe80::1/128` as the
next-hop owner's link-local. The one connected prefix `fd00::2/127` contains
exactly the two unique-local addresses. This makes the RFC2545 compatibility
fixture's common-subnet condition real without routing all of `fd00::/64`
locally. Setup checks this as `loopback-addresses` and, in install mode, runs:

```bash
sudo ifconfig lo0 inet6 fd00::2/128 alias      # macOS
sudo ifconfig lo0 inet6 fd00::3/127 alias
sudo ifconfig lo0 inet6 fe80::1/128 alias
sudo ip -6 addr add fd00::2/128 dev lo         # Linux
sudo ip -6 addr add fd00::3/127 dev lo
sudo ip -6 addr add fe80::1/128 dev lo
```

<!-- source: internal/le/setup/actions.go -- Answer -->

Presence requires a successful socket bind. IPv6 additionally requires the
exact address and prefix on `lo` (`lo0` on macOS); an address elsewhere, or
`fd00::3/128`, does not establish this fixture's common subnet. The link-local
bind probe includes the interface zone. An unreadable interface fails the
ownership check closed. Setup never silently removes an existing address with
a different prefix: resolve a conflicting assignment explicitly if the add
command fails.

These changes require an explicit `setup install` or the commands above.
`setup check` and test runners do not change host networking. The compatibility
runner's local-address preflight checks bindability, not this complete topology;
run setup check before the suite.

<!-- source: internal/test/runner/loopback.go -- the runner's probe and its error -->

These additions do not survive a reboot. Re-run `./le setup install` after one;
`./le setup check` says when it is needed. The merge gate adds the same IPv6
addresses and prefix in `.github/workflows/verify.yml`.

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
