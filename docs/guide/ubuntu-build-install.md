# Build and install Ze on Ubuntu

This page starts from a blank Ubuntu server and leaves you with an installed `ze` binary, a private `database/` store, an SSH listener, a systemd service, and one place to add Ze features.

The commands assume Ubuntu 24.04 or newer, `sudo`, and an `amd64` or `arm64` host. Replace `edge-01` and every password before running them on a real box.

<!-- source: internal/le/setup/actions.go -- Answer -->
<!-- source: internal/le/repo/featuretags/daemontags.go -- DaemonTags -->
<!-- source: internal/plugins/init/main.go -- Run, runInit, runInitPublished -->
<!-- source: internal/component/authz/yang/ze-authz-conf.yang -- system.authentication.user base fields and system.authorization.profile -->
<!-- source: internal/component/ssh/yang/ze-ssh-conf.yang -- environment.ssh and public-keys augmentation -->

For a container, a VM image or a lab instead, see
[Choose a deployment path](ze-install.md#choose-a-deployment-path).

## 1. Install build tools

Ubuntu's packaged Go may lag the version Ze needs, so install Go from `go.dev` and let `apt` provide the rest.

```bash
sudo apt-get update
sudo apt-get install -y \
  ca-certificates \
  curl \
  git \
  build-essential \
  jq \
  protobuf-compiler
```

Install the exact Go version pinned in this checkout's `go.mod`. The commands
below use `go1.27.3`; update that value when building a checkout with another pin.
<!-- source: go.mod -- Go version -->

```bash
GO_VERSION=1.27.3
GO_ARCH="$(dpkg --print-architecture)"
case "$GO_ARCH" in
  amd64|arm64) ;;
  *) echo "unsupported Go architecture: $GO_ARCH" >&2; exit 1 ;;
esac

curl -fsSLO "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf "go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"

cat >> "$HOME/.profile" <<'EOF'
export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH
EOF
export PATH=/usr/local/go/bin:$HOME/go/bin:$PATH

go version
```

Optional tools for appliance and ISO work:

```bash
sudo apt-get install -y qemu-system-x86 e2fsprogs xorriso grub-efi-amd64-bin
```

On an arm64 host, use `qemu-system-arm` and `grub-efi-arm64-bin` in place of
the two amd64 packages. Debian packages one GRUB module set per architecture;
the amd64 one has no installation candidate on arm64.

Ze also ships a setup checker. It uses the same tool list as the developer and appliance checks.

```bash
git clone https://github.com/ze-software/ze.git
cd ze
./le setup check
```

The `check` action lists anything missing as `[missing] <tool>` and exits non-zero. It changes nothing. `./le setup install` installs missing packages and prints each command first. Commands that need root use `sudo -n`, so nothing waits on a prompt it cannot answer. When sudo wants a password it asks once through `sudo -v`, and only when a terminal is attached; without a terminal it prints the command and exits non-zero.

## 2. Build Ze

From the cloned repository, run the daemon launcher. Its first run builds
`bin/ze` with the feature gates and patched standard library this checkout
requires. Run from the directory where you cloned Ze, rather than assuming it
is under your home directory.
<!-- source: ze -- toolchain pin, feature gates and bootstrap build -->

```bash
./ze version
```

The default feature-tag list is derived from `feature-gates.txt`.
`./le repo feature-tags check` verifies every checked-in consumer of that list, and
`./le repo compiles matrix` shows the shipped build flavors.

For a deliberately smaller custom binary, name its feature tags explicitly:

```bash
GOROOT="$(./le go stdpatch goroot)"
export GOROOT GOTOOLCHAIN=local PATH="$GOROOT/bin:$PATH"
CGO_ENABLED=0 go build -tags 'ze_core ze_distro ze_ssh ze_lg ze_web' -o ze-custom ./cmd/ze
```
<!-- source: internal/le/go/stdpatch/gen/stdpatch.go -- patched standard library -->

Install that custom build with `sudo ./ze-custom install`. The `ze_distro` base
keeps native installation and service management available in the smaller build.
<!-- source: cmd/ze/setup_features_distro.go -- native local and systemd imports -->

## 3. Install and start Ze

The default install command copies the binary to `/usr/local/bin/ze`, initializes
absent state, and installs and starts its systemd service. Choose your operator
username and password when prompted. Keep the default loopback address and port
`2222` for initial access. This installs a daemon on the host; it does not take
over the host's network interfaces.
<!-- source: internal/plugins/local/native.go -- nativeRuntime.install -->
<!-- source: internal/plugins/init/main.go -- runInstallInit and runInitPublishedProfile -->

```bash
sudo ./ze install
systemctl status ze.service --no-pager
sudo -u ze env ZE_CONFIG_DIR=/etc/ze XDG_RUNTIME_DIR=/run/ze /usr/local/bin/ze status
sudo -u ze env ZE_CONFIG_DIR=/etc/ze XDG_RUNTIME_DIR=/run/ze /usr/local/bin/ze cli -c "help"
ssh -p 2222 YOUR_USERNAME@127.0.0.1
```

Use the SSH editor to add peers and review and commit configuration. For a
complete routing example before changing this service, follow the
[local two-peer quickstart](quickstart.md). For additional operator accounts,
use [SSH and RBAC](operator-access-rbac.md).
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- peer configuration -->
<!-- source: internal/component/authz/yang/ze-authz-conf.yang -- operator profiles -->

### Update Ze

To update a standalone installation, retain the previous binary and a compatible
configuration backup, build the selected revision, then repeat the install
command during a maintenance window. Existing state and credentials remain.
Verify login and routing after the restart. Package-managed binaries must be
updated through their package manager. [Upgrading](ze-install.md#upgrading)
gives the command, what the installer refuses, and the checks to run after it.
<!-- source: internal/plugins/local/native.go -- nativeRuntime.install -->

### Remove Ze

```bash
# Remove the service and executable, retaining configuration and credentials.
sudo /usr/local/bin/ze uninstall
```

Removal stops the service before deleting the executable. State is retained by
default; add `--purge` only when you intend to discard the selected installation's
state. [Uninstalling](ze-install.md#uninstalling) lists the flags, the prefix
choice and the package-ownership refusals. For reloads, backups and recovery, see
[Lifecycle](lifecycle.md).
<!-- source: internal/plugins/local/native.go -- nativeRuntime.uninstall -->

## Manual setup: install only the binary

The remaining sections explain the separate steps for operators who need to
control bootstrap and RBAC before first service start. Use this workflow instead
of the default installation above. Do not run fresh `ze init` against an already
initialized service store.
<!-- source: internal/plugins/local/cmd_install.go -- local-only installation -->
<!-- source: internal/plugins/init/main.go -- refusal to replace an existing store -->

Install the built binary into `/usr/local/bin/ze` and create the default config directory.

```bash
sudo ./ze install local --prefix /usr/local
/usr/local/bin/ze version
```

`/usr/local/bin/ze` uses `/etc/ze` as its config directory.

## 4. Create the live store

`ze init` creates `/etc/ze/database/`. It stores the bootstrap admin user, the SSH client defaults, and the instance name. The input lines are:

1. username
2. password
3. SSH host for local CLI credentials, empty means `127.0.0.1`
4. SSH port for local CLI credentials, empty means `2222`
5. instance name, used as the default config filename

```bash
sudo install -d -m 0700 /etc/ze
printf 'admin\nCHANGE_ME_BOOTSTRAP\n\n\nedge-01\n' | sudo /usr/local/bin/ze init
sudo test -d /etc/ze/database
```

This creates the bootstrap admin in an atomically published tree. Its directories
are 0700 and files 0600. Both an existing tree and a `database.zefs` artifact
refuse initialization; `ze init --force --yes` replaces an unowned store and
retains a `.replaced-<stamp>` backup. `ze init --from <source>` imports a blob
from a local path or a URL. Add `--sha256 <hex>` to require a specific digest.
Keep the bootstrap admin as a recovery user until you have tested the configured users below.
<!-- source: internal/plugins/init/main.go -- Run, runInit, runInitPublished, runImport, runFetchImport -->

The initial config is stored under the name the instance name gives it. If that name cannot be read back, `ze init` fails with `name initial config:` rather than writing `ze.conf`.
<!-- source: internal/plugins/init/main.go -- runInitPublished -->

## 5. Create the first stored config

Keep the active configuration inside `database/` for bare `ze start`. Do not
create `/etc/ze/edge-01.conf` as a second source of truth in this workflow.
An explicit `ze start <file>` has different semantics: that file is authoritative
on every start, and daemon commits update it as well as stored history. Build the
candidate with set-format lines, render the import file, validate it, then load
it with one `ze config import` command.

Hash the configured user passwords before writing the candidate. This keeps plaintext out of shell history, process arguments, and stored command history.

```bash
ADMIN_HASH="$(printf '%s\n' 'CHANGE_ME_BOOTSTRAP' | /usr/local/bin/ze passwd)"
NOC_HASH="$(printf '%s\n' 'CHANGE_ME_NOC' | /usr/local/bin/ze passwd)"

umask 077
CONFIG_SET="$(mktemp)"
CONFIG_IMPORT="$(mktemp)"
trap 'rm -f "$CONFIG_SET" "$CONFIG_IMPORT"' EXIT

cat >"$CONFIG_SET" <<EOF
set environment ssh enabled enable
set environment ssh server main ip 0.0.0.0
set environment ssh server main port 2222
set environment ssh idle-timeout 600
set environment ssh max-sessions 32

set system authentication user admin password "$ADMIN_HASH"
set system authentication user admin profile admin
set system authentication user noc password "$NOC_HASH"
set system authentication user noc profile read-only

set system authorization profile admin run default-action allow
set system authorization profile admin edit default-action allow
set system authorization profile read-only run default-action allow
set system authorization profile read-only edit default-action deny
EOF

/usr/local/bin/ze config migrate -o "$CONFIG_IMPORT" format hierarchical "$CONFIG_SET"
/usr/local/bin/ze config validate "$CONFIG_IMPORT"
sudo /usr/local/bin/ze config import --name edge-01.conf "$CONFIG_IMPORT"
sudo /usr/local/bin/ze config list
```

Expected validation output:

```text
configuration valid: /tmp/tmp.XXXXXXXXXX
```

The explicit `admin` user matters. Once any configured user has profile assignments, unassigned users are denied by local RBAC. Defining `admin` in config keeps the bootstrap name usable with an explicit `admin` profile. Bare `ze start` reads the stored active `edge-01.conf`.

## 6. Install and start systemd

```bash
sudo /usr/local/bin/ze install systemd --start
systemctl status ze.service --no-pager
```

The generated unit starts `/usr/local/bin/ze start`, uses `/etc/ze` as
`ZE_CONFIG_DIR`, and sets the runtime directory to `/run/ze`. Installation
transfers the complete tree and ownership lock to `ze` before service startup,
and refuses if a daemon already owns it.
<!-- source: internal/plugins/systemd/cmd_install.go -- serviceRuntime.cmdInstall, serviceRuntime.chownConfig -->

After transfer, even root cannot open the store through Ze. Local commands that
use stored credentials run as the owner:

```bash
sudo -u ze env ZE_CONFIG_DIR=/etc/ze XDG_RUNTIME_DIR=/run/ze /usr/local/bin/ze status
sudo -u ze env ZE_CONFIG_DIR=/etc/ze XDG_RUNTIME_DIR=/run/ze /usr/local/bin/ze cli -c "help"
```

Root still manages systemd and binary installation. Offline store maintenance
requires stopping the daemon first and running the storage command as `ze`.
Read-only credential lookup remains available while the daemon runs.
<!-- source: internal/component/config/storage/open.go -- OpenReadOnly -->
<!-- source: internal/component/config/storage/ownership.go -- TransferOwnership -->

## 7. Test SSH login and RBAC

The `admin` user can run operational and edit commands. The `noc` user can run operational commands but cannot edit config.

```bash
export XDG_RUNTIME_DIR=/run/ze
ZE_SSH_PASSWORD='CHANGE_ME_BOOTSTRAP' /usr/local/bin/ze cli --user admin -c "help"
ZE_SSH_PASSWORD='CHANGE_ME_NOC' /usr/local/bin/ze cli --user noc -c "help"
```

If the server listens on a management address, connect remotely with `--remote host:port`, using the host and port from `environment ssh`.

```bash
ZE_SSH_PASSWORD='CHANGE_ME_NOC' /usr/local/bin/ze cli --remote 192.0.2.10:2222 --user noc -c "help"
```

## 8. Add features

There are two kinds of feature work.

| Feature type | What you change | Example |
| --- | --- | --- |
| Compiled service | Explicit Go build tags | `ze_lg` compiles the looking glass server |
| Runtime feature | Config lines and plugin declarations | `environment looking-glass`, `plugin internal bgp-rr`, `firewall backend nft` |

For normal installs, keep the default binary and add runtime features to the
stored config. The example below exports the current entry, renders and validates
a candidate, then stops the owning daemon for an offline import as the store
owner. The shell supplies the candidate through stdin so its temporary-file
permissions do not need widening.

```bash
set -euo pipefail

umask 077
CONFIG_SET="$(mktemp)"
CONFIG_IMPORT="$(mktemp)"
trap 'rm -f "$CONFIG_SET" "$CONFIG_IMPORT"' EXIT

sudo -u ze env ZE_CONFIG_DIR=/etc/ze /usr/local/bin/ze config cat edge-01.conf | /usr/local/bin/ze config migrate -o "$CONFIG_SET" -

cat >>"$CONFIG_SET" <<'EOF'
set plugin internal bgp-rr use bgp-rr
EOF

/usr/local/bin/ze config migrate -o "$CONFIG_IMPORT" format hierarchical "$CONFIG_SET"
/usr/local/bin/ze config validate "$CONFIG_IMPORT"
sudo systemctl stop ze.service
sudo -u ze env ZE_CONFIG_DIR=/etc/ze /usr/local/bin/ze config import --yes --name edge-01.conf - < "$CONFIG_IMPORT"
sudo systemctl start ze.service
```

Use the pages below as feature-specific starting points:

| Page | Adds |
| --- | --- |
| [Operator access with SSH and RBAC](operator-access-rbac.md) | Local users, profiles, public keys, TACACS+ pointers |
| [FlowSpec route reflector](flowspec-route-reflector.md) | `bgp-rr`, iBGP FlowSpec clients, route-reflector-client flags |
| [FlowSpec protected router](flowspec-protected-router.md) | nft firewall backend, control-plane policing, FlowSpec firewall bridge |
| [Looking glass](looking-glass-howto.md) | public read-only HTTP looking glass and birdwatcher-compatible API |

## 9. Stop or roll back

```bash
sudo systemctl stop ze.service
sudo /usr/local/bin/ze uninstall systemd
sudo /usr/local/bin/ze uninstall local
```

Use `--purge` only when you want to remove `/etc/ze` and its `database/` tree.

To return to an earlier build, install it as [Upgrading](ze-install.md#upgrading)
describes. To roll back configuration, see
[Lifecycle](lifecycle.md#archive-and-rollback).
