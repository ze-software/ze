# Developer Setup

<!-- source: scripts/dev/dev-setup.py -- tool list and OS detection -->

Set up a Ze development environment with all build, lint, and test dependencies.

## Quick Start

```bash
git clone <repo-url> && cd ze
make ze-setup
```

This detects your OS (macOS with Homebrew or Debian/Ubuntu with apt), installs
missing tools, vendors Go dependencies, and reports what it did.

## Check Mode

Probe the current host without installing anything:

```bash
make ze-setup CHECK=1
```

Exits 0 if all required tools are present, nonzero if any are missing.
Useful as a CI preflight check.

Two of the rows are behaviour, not binaries: `gopls-answers` and
`pyright-answers` run each language server and check that it replies. A server
on PATH that does not answer fails this check, because every LSP call against
it fails the same silent way.

## What It Installs

### Build and Lint

| Tool | Purpose |
|------|---------|
| `go` | Go toolchain |
| `git` | Version control |
| `protobuf` (`protoc`) | Protocol buffer compiler |
| `jq` | JSON processing |
| `golangci-lint` | Go linter (via `go install`) |
| `goimports` | Go import formatter (via `go install`) |
| `gopls` | Go language server behind the agent LSP tool (via `go install`) |
| `python3` | Runs evidence and dev scripts |
| `pipx` | Python tool installer |
| `ruff` | Python linter (via `pipx`) |
| `pyright` | Python language server behind the agent LSP tool (via `pipx`) |

### Appliance and Evidence

| Tool | Purpose |
|------|---------|
| `uv` | Python package runner for SSH probe (`uv run --with paramiko`, via `pipx`) |
| `qemu` | QEMU functional and install gate tests |
| `e2fsprogs` | `mkfs.ext4` and `debugfs` for appliance builds |
| `xorriso` | ISO image creation |
| `grub` | GRUB EFI tooling for ISO builds (Linux only) |

### Optional

| Tool | Purpose |
|------|---------|
| `sshpass` | SSH probe fallback (uv+paramiko is primary) |
| `docker` / `colima` | Container appliance and kernel builds |

## Platform Notes

### macOS

- **e2fsprogs** is keg-only on Homebrew. The Ze code resolves it via the
  Cellar path; no PATH modification is needed after `brew install e2fsprogs`.
- **grub** has no first-party Homebrew formula. ISO builds require Linux or
  a container (colima/docker). The setup script skips grub on macOS.

### Linux

`make ze-setup` installs the apt packages itself, the same way it installs the
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

<!-- source: scripts/dev/dev-setup.py -- privilege_mode, run_privileged, apt_install -->

**uv** is not in the Debian or Ubuntu repositories, so it installs through
`pipx` on both platforms. One route is one thing to fix, and it keeps
`curl | sh` off every dev machine.

**Unprivileged user namespaces.** Ubuntu 23.10+ ships
`kernel.apparmor_restrict_unprivileged_userns=1`, which blocks the sandbox
Chrome relies on and makes the `agent-browser` web functional tests fail to
launch Chrome (`No usable sandbox!`). Setup checks this tunable as
`userns-unrestricted`. When it is restricted, `make ze-setup` (install mode)
echoes and then runs these commands via `sudo` to lift it globally:

```bash
echo "kernel.apparmor_restrict_unprivileged_userns = 0" | sudo tee /etc/sysctl.d/60-ze-userns.conf
sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0
```

The `/etc/sysctl.d` drop-in makes the change survive reboots. If `sudo` is
unavailable it prints the commands to run by hand instead. `make ze-setup
CHECK=1` only reports the state, never changes it.

**KVM device access.** `/dev/kvm` is `root:kvm` mode 0660, so QEMU-backed
evidence (the appliance boot proofs and every `ze-qemu-*` target) needs your
user in the `kvm` group. Without it QEMU does not quietly fall back to
emulation: it refuses to start with `Could not access KVM kernel module:
Permission denied`, and the calling script reports a timeout instead. Setup
checks this as `kvm-access` and, in install mode, runs:

```bash
sudo usermod -aG kvm $USER
```

<!-- source: scripts/dev/dev-setup.py -- kvm_status, apply_kvm_fix -->

Group membership is fixed at login, so an existing shell keeps the old groups
even after the command succeeds. Log out and back in, or run one command with
the new group:

```bash
sg kvm -c 'make ze-vpp-hugepages-qemu-test'
```

Setup distinguishes the two states: `kvm-access` reports `pending` when the
group database lists you but the running session predates it, and `missing`
when the group is not granted at all. A host with no `/dev/kvm` (no hardware
virtualisation, or a VM without nested virt) reports `n/a`: QEMU runs under
`tcg` there, only slower. macOS has no `/dev/kvm` and needs no group; the
evidence scripts select the Apple hypervisor (`hvf`) by platform.

<!-- source: scripts/evidence/effective-vpp-hugepages-qemu.py -- QEMU_ACCEL per-OS selection -->

These two are the only places setup runs `sudo`.

## After Setup

Verify everything works:

```bash
make ze-smoke    # lint + unit tests + build (~2 min)
```

Check that appliance tools are detected:

```bash
bin/ze-setup appliance iso --check
```

## Drift Guard

The dev setup script and `ze doctor` appliance checks share the same tool
list. A Go test (`TestDevSetupMatchesDoctor` in
`internal/appliance/dev_setup_drift_test.go`) fails if they disagree,
preventing the lists from drifting apart.
