# Build and install Ze on Ubuntu

This page starts from a blank Ubuntu server and leaves you with an installed `ze` binary, a `database.zefs`, an SSH listener, a systemd service, and one place to add Ze features.

The commands assume Ubuntu 24.04 or newer, `sudo`, and an `amd64` or `arm64` host. Replace `edge-01` and every password before running them on a real box.

<!-- source: internal/le/setup/actions.go -- Answer -->
<!-- source: internal/le/setup/actions.go -- Answer -->
<!-- source: internal/le/featuretags/daemontags.go -- DaemonTags -->
<!-- source: internal/plugins/init/main.go -- ze init input format and database.zefs creation -->
<!-- source: internal/component/authz/yang/ze-authz-conf.yang -- system.authentication.user base fields and system.authorization.profile -->
<!-- source: internal/component/ssh/yang/ze-ssh-conf.yang -- environment.ssh and public-keys augmentation -->

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

Install Go 1.26. Pick the current 1.26 patch release from <https://go.dev/dl/> if a newer patch exists.

```bash
GO_VERSION=1.26.0
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

On an arm64 host, ask for `grub-efi-arm64-bin` instead: Debian packages one GRUB
module set per architecture, and the amd64 one has no installation candidate
there.

Ze also ships a setup checker. It uses the same tool list as the developer and appliance checks.

```bash
git clone https://github.com/ze-software/ze.git
cd ze
./le setup check
```

The `check` action lists anything missing as `[missing] <tool>` and exits non-zero. It changes nothing. `./le setup install` installs missing packages and prints each command first. Commands that need root use `sudo -n`, so nothing waits on a prompt it cannot answer. When sudo wants a password it asks once through `sudo -v`, and only when a terminal is attached; without a terminal it prints the command and exits non-zero.

## 2. Build Ze

Build the daemon directly with Go:

```bash
cd ~/ze
CGO_ENABLED=0 go build -tags 'ze_core ze_distro ze_anomaly ze_as112 ze_bfd ze_bgp ze_bmp ze_copp ze_cos ze_ddos ze_dhcpserver ze_exabgp ze_flowexport ze_geodns ze_gnmi ze_grpc ze_ike ze_isis ze_l2tp ze_ldp ze_lg ze_mcp ze_mpls ze_mrt ze_ntp ze_ospf ze_policyroute ze_pxe ze_radius ze_rest ze_rsvpte ze_ssh ze_tacacs ze_telemetry ze_trafficusage ze_vpp ze_vrrp ze_web' -o bin/ze ./cmd/ze
```

The default feature-tag list is derived from `feature-gates.txt`.
`./le feature-tags check` verifies every checked-in consumer of that list, and
`./le repository-tracked-build matrix` shows the shipped build flavors.

For a deliberately smaller custom binary, name its feature tags explicitly:

```bash
go build -tags 'ze_core ze_ssh ze_lg ze_web' -o bin/ze ./cmd/ze
```

## 3. Install the binary

Install the built binary into `/usr/local/bin/ze` and create the default config directory.

```bash
sudo ./bin/ze install local --prefix /usr/local
/usr/local/bin/ze version
```

`/usr/local/bin/ze` uses `/etc/ze` as its config directory.

## 4. Create `database.zefs`

`ze init` creates `/etc/ze/database.zefs`. It stores the bootstrap admin user, the SSH client defaults, and the instance name. The input lines are:

1. username
2. password
3. SSH host for local CLI credentials, empty means `127.0.0.1`
4. SSH port for local CLI credentials, empty means `2222`
5. instance name, used as the default config filename

```bash
sudo install -d -m 0700 /etc/ze
printf 'admin\nCHANGE_ME_BOOTSTRAP\n\n\nedge-01\n' | sudo /usr/local/bin/ze init
sudo test -s /etc/ze/database.zefs
```

This creates the zefs bootstrap admin. Keep it as a recovery user until you have tested the configured users below.

## 5. Create the first config in zefs

Keep the active configuration inside `database.zefs`. Do not create `/etc/ze/edge-01.conf` as a second source of truth. Build the candidate with set-format lines, render the import file, validate it, then load it into zefs with one `ze config import` command.

Hash the configured user passwords before writing the candidate. This keeps plaintext out of shell history, process arguments, and the zefs command history.

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

/usr/local/bin/ze config migrate --format hierarchical -o "$CONFIG_IMPORT" "$CONFIG_SET"
/usr/local/bin/ze config validate "$CONFIG_IMPORT"
sudo /usr/local/bin/ze config import --name edge-01.conf "$CONFIG_IMPORT"
sudo /usr/local/bin/ze config ls
```

Expected validation output:

```text
configuration valid: /tmp/tmp.XXXXXXXXXX
```

The explicit `admin` user matters. Once any configured user has profile assignments, unassigned users are denied by local RBAC. Defining `admin` in config keeps the bootstrap name usable with an explicit `admin` profile. `ze start` reads the active `edge-01.conf` config from zefs.

## 6. Install and start systemd

```bash
sudo /usr/local/bin/ze install systemd --start
systemctl status ze.service --no-pager
```

The generated unit starts `/usr/local/bin/ze start`, uses `/etc/ze` as `ZE_CONFIG_DIR`, and sets the runtime directory to `/run/ze`.

For local operator commands from the shell, either run as root with the same config directory, or point the CLI at the systemd runtime socket:

```bash
export XDG_RUNTIME_DIR=/run/ze
/usr/local/bin/ze status
/usr/local/bin/ze cli -c "help"
```

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

For normal installs, keep the default binary and add runtime features to the active zefs config. The feature pages use this pattern: read the current zefs entry, normalize it to set format, append set-format lines, render a checked import file, validate, import, and reload.

```bash
set -euo pipefail

umask 077
CONFIG_SET="$(mktemp)"
CONFIG_IMPORT="$(mktemp)"
trap 'rm -f "$CONFIG_SET" "$CONFIG_IMPORT"' EXIT

sudo /usr/local/bin/ze config cat edge-01.conf | /usr/local/bin/ze config migrate -o "$CONFIG_SET" -

cat >>"$CONFIG_SET" <<'EOF'
set plugin internal bgp-rr use bgp-rr
EOF

/usr/local/bin/ze config migrate --format hierarchical -o "$CONFIG_IMPORT" "$CONFIG_SET"
/usr/local/bin/ze config validate "$CONFIG_IMPORT"
sudo /usr/local/bin/ze config import --name edge-01.conf "$CONFIG_IMPORT"
sudo systemctl reload ze.service
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

Use `--purge` only when you want to remove `/etc/ze` and `database.zefs`.
