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
| `python3` | Runs evidence and dev scripts |
| `pipx` | Python tool installer |
| `ruff` | Python linter (via `pipx`) |

### Appliance and Evidence

| Tool | Purpose |
|------|---------|
| `uv` | Python package runner for SSH probe (`uv run --with paramiko`) |
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

The script prints `sudo apt-get install ...` commands but never runs `sudo`
automatically. Copy and run the printed commands to install.

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

The `/etc/sysctl.d` drop-in makes the change survive reboots. This is the one
place setup runs `sudo`; if `sudo` is unavailable it prints the commands to run
by hand instead. `make ze-setup CHECK=1` only reports the state, never changes
it.

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
