# Operations

Running Ze in production: SSH access, signals, health checks, environment variables, and troubleshooting.

## SSH Server

Ze exposes its CLI over SSH. `ze cli`, `ze cli -c`, and `ze signal` commands connect to this server. Most `ze show` subcommands do too, except local ones (version, bgp decode/encode, env, schema, yang, completion) which run in-process.

### Setup

```bash
ze init                        # interactive: prompts for username, password, host, port
```

Defaults: `127.0.0.1:2222`, ED25519 host key auto-generated.

Credentials are stored under `database/` in the resolved configuration folder,
with bcrypt-hashed passwords. Initialization seeds every key before publishing
the directory, so an interrupted initialization cannot expose partial credentials.
<!-- source: internal/plugins/init/main.go -- Run, runInit -->

| Init flag | Effect |
|-----------|--------|
| `--managed` | Enable managed fleet mode |
| `--force` | Replace an existing store, preserving a `.replaced-<stamp>` backup |
| `--yes` | Confirm `--force` without a terminal prompt |
| `--web-cert <address>` | Generate a web TLS certificate for the listen address |
| `--web-cert-name <name>` | Add a DNS name to the generated certificate |
| `--seed` | Build `database.zefs` as an appliance seed artifact, without build-host interface discovery |
<!-- source: internal/plugins/init/main.go -- Run -->

### Reinitializing

Running `ze init` a second time fails with `error: database already exists`. This is intentional -- it prevents accidental credential loss.

To reinitialize:

```bash
ze signal stop                 # daemon must be stopped first
ze init --force                # prompts for confirmation interactively
```

`--force` stages the new store before moving the old tree to
`database.replaced-<stamp>`. An existing blob is backed up as
`database.zefs.replaced-<stamp>`. The backup retains the previous credentials
and configs. Confirmation requires a terminal unless `--yes` is supplied.
Replacement acquires the same ownership lock as the daemon and refuses while
any process owns the store, including a web-only daemon. Changing the SSH target
does not bypass this protection.
<!-- source: internal/plugins/init/main.go -- Run, runInit -->

### Store folder and ownership

An explicit config path selects its parent folder. Without one, clients and
daemons use `ze.config.dir`, then the binary's default configuration folder.
`-` uses the same environment/default resolution. To reach credentials for a
daemon started on a non-default folder, set `ZE_CONFIG_DIR` to that folder.
<!-- source: internal/core/resolve/resolve.go -- StoreDir -->

`ze start <file>` creates `database/` when neither store shape exists and reads
the explicit file on every start. Its daemon commits update that file and stored
history. Bare `ze start` reads the stored active configuration. A `database.zefs`
beside the live location is refused without conversion. The diagnostic names
`ze init --from <blob>`, which imports a local blob; the URL form is storage-2.
Appliance first boot already imports its seed explicitly.
<!-- source: cmd/ze/ze_core_start.go -- openExplicitStore, cmdStart -->
<!-- source: internal/plugins/init/main.go -- Run -->
<!-- source: cmd/ze/ze_core_autoinit.go -- gokrazyAutoInit -->

The tree root and every directory beneath it must be 0700, each frame file 0600,
and all must belong to the caller. Symlinks and non-regular leaves are refused.
Root has no ownership exception. Permission errors name the path and the repair.
Only one process may own the store for writing. Read-only credential lookup and
inspection remain available while that owner runs; offline writers such as
`ze connect add`, `remove`, and `default` refuse until it stops.
<!-- source: internal/component/config/storage/open.go -- Open, OpenReadOnly, lockOwner -->

The owner holds `database.lock` until the store closes. The lock stays outside
the replaceable tree. Removing this file would break writer exclusion.
Initialization uses a private random staging directory and a no-replace rename.
<!-- source: internal/component/config/storage/open.go -- populateOwned, lockOwner -->
<!-- source: internal/component/config/storage/tree.go -- makeStage -->

<!-- source: internal/core/resolve/resolve.go -- StoreDir, StorageFor -->
<!-- source: internal/component/cli/sshclient/client.go -- ResolveStoreDir, openStoreIfReadable -->
<!-- source: internal/plugins/connect/main.go -- AddCredentials, RemoveCredentials, SetDefault -->

### Offline data access

`ze data --path <folder>/database` addresses a tree, while an explicit `.zefs`
file addresses a blob artifact. Run mutating commands only after stopping the
owning daemon and as the store owner. These commands expose decoded values,
never the tree's framing bytes.

| Command | Effect |
|---------|--------|
| `ze data list` | List keys recursively |
| `ze data cat <key>` | Read a key |
| `ze data write <key> <file>` | Write a file's bytes into a raw key (`-` reads stdin) |
| `ze data import <file>...` | Import files as active configs |
| `ze data rm <key>` | Remove a key |
| `ze data registered` | Show registered key patterns |
| `ze data check` | Check integrity; exits 0 clean, 1 corrupt, 2 unreadable |
| `ze data repair --output <path>` | Copy recoverable keys into a new destination |
| `ze data encode [--crc\|--header] [--cap N] <string\|->` | Encode a netcapstring for inspection |

<!-- source: internal/component/config/storage/cli/main.go -- subcommandHandlers -->

### Connection

```bash
ze cli                         # interactive CLI
ze cli -c "show bgp peer list"  # single command
ze show bgp peer list                # read-only shorthand
ze cli -c "request peer transit teardown 2" # one-shot command
```
<!-- source: internal/component/cli/client/main.go -- Run -->

### Override Host/Port

```bash
# Environment variables
export ZE_SSH_HOST=10.0.0.1
export ZE_SSH_PORT=2222

# Per-command flags
ze signal reload --host 10.0.0.1 --port 2222
```
<!-- source: internal/component/cli/sshclient/client.go -- ze.ssh.host, ze.ssh.port env vars -->

## Signals

### Via SSH (preferred)

| Command | Effect |
|---------|--------|
| `ze signal reload` | Transactionally reload configuration (add/remove/update peers) |
| `ze signal stop` | Graceful shutdown (no GR marker) |
| `ze signal restart` | Graceful restart (writes GR marker for RFC 4724) |
| `ze signal status` | Dump process status |
| `ze signal quit` | Goroutine dump to stderr, then exit |
<!-- source: internal/plugins/signal/main.go -- Commands registry -->

### Via Unix Signals

| Signal | Effect |
|--------|--------|
| `SIGHUP` | Transactionally reload configuration |
| `SIGTERM` / `SIGINT` | Graceful shutdown (NOTIFICATION Cease to all peers) |
| `SIGUSR1` | Dump status to stderr |
<!-- source: cmd/ze/hub/main.go -- signal.Notify, SIGHUP/SIGTERM/SIGINT handling -->

Reload writes the edited config as a candidate version. The active pointer moves
only after verification, apply, and subsystem reload succeed. A failed reload
clears the candidate and keeps the previous active config.

### Exit Codes (signal command)

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Daemon not running |
| 2 | No SSH credentials (run `ze init`) |
| 4 | Signal delivery failed |
<!-- source: internal/plugins/signal/main.go -- ExitSuccess/ExitNotRunning/ExitNoCredentials/ExitSignalFailed -->

## Health Checks

### Liveness

```bash
ze status                      # exit 0 = running, exit 1 = not running
```

This dials the SSH port without completing a handshake. Suitable for systemd watchdog or load balancer TCP check.
<!-- source: internal/plugins/signal/main.go -- RunStatus, net.Dialer -->

### Scripting

```bash
if ze status --host 127.0.0.1 --port 2222; then
    echo "ze is running"
else
    echo "ze is down"
fi
```

### Peer Health

```bash
ze cli -c "show bgp peer list"  # brief peer list with state
ze cli -c "show bgp"            # summary table with uptime and prefix counts
```

## Kernel Capability Refusal

Ze refuses to start when the configuration uses a subsystem this kernel cannot
carry. The refusal prints one line on stderr and exits 1.

```
error: kernel capability: ipsec: the kernel holds no CONFIG_XFRM_USER, which vpn ipsec requires: protocol not supported
```

Every failing subsystem is named, not the first one, so one repair cycle covers
them all. `ze doctor` reports the same verdict before you start, and
`ze config validate` fails on it.

| Subsystem | Kernel feature | What to do |
|-----------|----------------|------------|
| `ipsec` | `CONFIG_XFRM_USER`, and `CONFIG_INET_ESP` to carry the packets | Run a kernel with both. Ze's appliance kernel builds them in |
| `mpls` | `CONFIG_MPLS_ROUTING` and `CONFIG_MPLS_IPTUNNEL`, or the `mpls_router` module | Load the module or run a kernel with both built in |

There is no override. A NOS that half-works on a kernel missing a required
feature is the hazard this removes, and an override is what an operator reaches
for under pressure.

Three answers are possible and only one refuses:

| `ze doctor` reports | Severity | `ze` |
|---------------------|----------|------|
| nothing | - | starts |
| `doctor-<subsystem>-unavailable` | error | refuses, exit 1 |
| `doctor-<subsystem>-unknown` | warning | starts |

The third answer means ze could not reach the probe's evidence, most often
because the process lacked a privilege. No kernel rebuild fixes that, so ze warns
and starts. Read the reason in the message: `ze explain doctor-ipsec-xfrm-unknown`
carries the rest.

A configuration that does not use the subsystem is never probed and never
reported. An empty `vpn { ipsec { } }` block installs no Security Association, so
it is not IPsec in use.

A reload is refused the same way, and a refused reload leaves the running
configuration serving. The operator sees the refusal through `ze config commit`.

`ze config validate` answers about the host running the command. A config written
for another machine and validated on a workstation is judged against the
workstation.

<!-- source: internal/component/kernelcap/kernelcap.go -- Refuse, Evaluate -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig, the kernel capability refusal -->

## Environment Variables

Ze environment variables use dot notation (`ze.log.bgp`) and are case-insensitive. All forms are equivalent: `ze.log.bgp`, `ZE_LOG_BGP`, `ze_log_bgp`.

List all registered variables:

```bash
ze env list                    # all vars with types and defaults
ze env list -v                 # include current values
ze env get ze.log              # details for one var
```

### Key Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `ze.log` | `warn` | Base log level |
| `ze.log.<subsystem>` | -- | Per-subsystem log level |
| `ze.log.backend` | `stderr` | Log output: stderr, stdout, syslog, kmsg, or a comma-separated list of them |
| `ze.log.destination` | -- | Syslog address (when backend=syslog) |
| `ze.log.relay` | `warn` | Plugin stderr relay threshold |
| `ze.ssh.host` | -- | Override SSH host for CLI commands |
| `ze.ssh.port` | -- | Override SSH port for CLI commands |
| `ze.config.dir` | -- | Override config directory |
<!-- source: internal/core/slogutil/slogutil.go -- ze.log registration -->
<!-- source: internal/component/cli/sshclient/client.go -- ze.ssh.host/port -->
<!-- source: internal/core/paths/paths.go -- ze.config.dir -->

## CLI Flags

| Flag | Purpose |
|------|---------|
| `-d`, `--debug` | Enable debug logging (sets `ze.log=debug`) |
| `ze config edit -f <file>` | Edit a loose file without opening its store |
| `--plugin <name>` | Load additional plugin for YANG/native configs (repeatable). Hub/orchestrator configs reject this; use `plugin { internal ... }` or `plugin { external ... }` in the config instead. |
| `--pprof <addr:port>` | Start pprof HTTP server for profiling |
| `--chaos-seed <N>` | Chaos testing seed (0=off, -1=time-based) |
| `--chaos-rate <0-1>` | Chaos fault probability |
| `-V`, `--version` | Show version |
<!-- source: cmd/ze/main.go -- global flag parsing -->

## systemd

Use `ze install systemd` on standard Linux hosts:

```bash
sudo ze init
sudo ze install systemd --start
```

This writes `/etc/systemd/system/ze.service`, creates the `ze` user/group if
missing, and transfers the complete store tree and its ownership lock to that
account before enabling or starting the service. A running store owner refuses
the transfer. Root performs the system changes; after transfer, storage
maintenance runs as `ze`.

Inspect the generated unit without writing anything:

```bash
ze install systemd --dry-run --config /etc/ze
```

Run doctor after installation to verify the service account and binary path:

```bash
sudo -u ze env ZE_CONFIG_DIR=/etc/ze ze doctor
```

When `/etc/systemd/system/ze.service` exists, doctor checks that the unit's
`ExecStart` binary exists and is executable, and that the configured `User` and
`Group` exist on the host.

The service runs with `XDG_RUNTIME_DIR=/run/ze`, so the daemon socket is
`/run/ze/ze.socket`. Local commands using stored credentials run as its owner:

```bash
sudo -u ze env ZE_CONFIG_DIR=/etc/ze XDG_RUNTIME_DIR=/run/ze ze cli
```

Other users can supply `--user`, `ze.ssh.password`, and an explicit remote
address instead of reading the service account's credentials. Offline
maintenance first stops `ze.service` as root, then runs the storage command
with `sudo -u ze`; restarting the service is a root system operation.

Remove only the service unit:

```bash
sudo ze uninstall systemd
```

Generated unit shape:

```ini
[Unit]
Description=Ze Network OS
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=ze
Group=ze
ExecStart=/usr/local/bin/ze start
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
LimitCORE=infinity
LimitMEMLOCK=infinity
WorkingDirectory=/etc/ze
Environment=ZE_CONFIG_DIR=/etc/ze
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
<!-- source: internal/plugins/systemd/cmd_install.go -- cmdInstall -->
<!-- source: internal/plugins/systemd/unit.go -- buildUnitFile -->

## Troubleshooting

### Peer Won't Connect

```
Symptom: peer stays in Connect/Active state
```

1. **Check connectivity:** Can you reach the peer IP on port 179?
   ```bash
   nc -zv 10.0.0.2 179
   ```

2. **Check config:** Validate before debugging network issues
   ```bash
   ze config validate config.conf
   ```

3. **Enable debug logging:**
   ```bash
   ze cli -c "request log level bgp.fsm debug"
   ze cli -c "bgp monitor event state"
   ```

4. **Common causes:**
   - Firewall blocking TCP 179
   - Peer not configured for your address
   - AS number mismatch (check `local { as }` and `remote { as }`)
   - Hold time rejected (must be 0 or >= 3)

### Peer Drops After OPEN

```
Symptom: session establishes briefly then NOTIFICATION received
```

1. Check the NOTIFICATION code in monitor output:

   | Code | Subcode | Meaning |
   |------|---------|---------|
   | 2 | 2 | Bad Peer AS -- AS number mismatch |
   | 2 | 3 | Bad BGP Identifier -- duplicate router-id |
   | 2 | 6 | Unacceptable Hold Time |
   | 2 | 11 | Role Mismatch (RFC 9234) |
   | 6 | 2 | Administrative Shutdown |
   | 6 | 5 | Connection Rejected |

2. **Role mismatch:** If using `role { import customer; strict true; }`, both peers must advertise compatible roles.

3. **Capability issues:** Enable debug to see negotiated vs offered capabilities:
   ```bash
   ze cli -c "request log level bgp.reactor debug"
   ```

### Daemon Won't Start

1. **Config parse error:** Run `ze config validate config.conf` first
2. **Port in use:** Check if another ze instance or BGP daemon holds port 179
3. **SSH port conflict:** Default SSH is 2222. Check with `netstat -tlnp | grep 2222`
4. **Missing credentials:** Run `ze init` before starting
<!-- source: internal/component/config/cli/cmd_validate.go -- cmdValidate; internal/component/ssh/ -- SSH server -->

### Plugin Not Working

1. **Check plugin is loaded:**
   ```bash
   ze show plugin list               # list compiled-in plugins
   ```

2. **Check plugin is bound to peer:** Config must have `attach process <name> { receive [...] }` on the peer

3. **Check plugin logs:** Set `ZE_LOG_PLUGIN_RELAY=debug` to see plugin stderr output

4. **Plugin not reaching Ready state:** Enable `ZE_LOG_PLUGIN=debug` and look for startup stage failures
<!-- source: internal/component/plugin/server/server.go -- plugin stage timeout; internal/component/plugin/registry/registry.go -- plugin registration -->

### No Routes in RIB

1. **Check RIB plugin is loaded and bound:**
   ```bash
   ze cli -c "show bgp rib status"
   ```

2. **Check peer is sending updates:**
   ```bash
   ze cli -c "bgp monitor peer transit-a event update direction received"
   ```

3. **Check RPKI validation:** If bgp-rpki is loaded, routes may be pending validation. Check:
   ```bash
   ze cli -c "show bgp rpki status"
   ```

### Crash Capture

Ze automatically captures panic stack traces and forwards them to syslog. On
restart after a crash, `show crashes` displays saved crash reports with ring
buffer context (the last 64 log entries before the panic).

**Configuration (environment variables):**

| Variable | Default | Description |
|----------|---------|-------------|
| `ze.crash.dir` | autodetect | Override crash file directory |
| `ze.crash.keep` | `5` | Number of crash files to retain (1-100) |

Crash dir autodetection probes in order: `/perm/ze/crash/` (gokrazy),
`<config-dir>/crash/`, `/var/lib/ze/crash/`, `/tmp/ze-crash/`. The resolved
path is printed at startup.

Syslog forwarding uses `ze.log.destination` (no separate config). Crash
messages use `LOG_CRIT` facility for filtering.

**CLI (online, daemon running):**

```
show crashes           # list crash files with timestamps
show crashes latest    # display most recent crash report
```

**CLI (works with or without a daemon; falls back to in-process read when the daemon is down):**

```
ze show crashes          # list crash files (JSON)
ze show crashes latest   # display most recent crash report
```

### Collecting Debug Information

For bug reports, collect:

```bash
ze --version                   # ze version
ze config validate config.conf        # config validity
ze status                      # daemon state
ze cli -c "show bgp peer list"  # peer states
ze cli -c "show bgp"            # session summary
ze env list -v                 # effective configuration
ze signal quit                 # goroutine dump (kills daemon!)
```
