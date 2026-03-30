# Operations

Running Ze in production: SSH access, signals, health checks, environment variables, and troubleshooting.

## SSH Server

Ze exposes its CLI over SSH. `ze cli`, `ze run`, and `ze signal` commands connect to this server. Most `ze show` subcommands do too, except local ones (version, bgp decode/encode, env, schema, yang, completion) which run in-process.

### Setup

```bash
ze init                        # interactive: prompts for username, password, host, port
```

Defaults: `127.0.0.1:2222`, ED25519 host key auto-generated.

Credentials are stored in the ze database (`database.zefs`) with bcrypt-hashed passwords.
<!-- source: cmd/ze/init/main.go -- keyUsername/keyPassword/keyHost/keyPort, defaultHost, defaultPort -->

### Reinitializing

Running `ze init` a second time fails with `error: database already exists`. This is intentional -- it prevents accidental credential loss.

To reinitialize:

```bash
ze signal stop                 # daemon must be stopped first
ze init --force                # prompts for confirmation interactively
```

`--force` moves the old database to `database.zefs.replaced-<date>` as a backup before creating a new one. The backup contains your previous SSH credentials and any stored configs. Non-interactive use (piped stdin) is rejected for safety -- `--force` requires interactive confirmation.
<!-- source: cmd/ze/init/main.go -- forceFlag -->

### Connection

```bash
ze cli                         # interactive CLI
ze cli --run "peer list"       # single command
ze show peer list              # read-only shorthand
ze run peer transit teardown 2 # read-write shorthand
```
<!-- source: cmd/ze/cli/main.go -- Run; cmd/ze/show/main.go -- Run; cmd/ze/run/main.go -- Run -->

### Override Host/Port

```bash
# Environment variables
export ZE_SSH_HOST=10.0.0.1
export ZE_SSH_PORT=2222

# Per-command flags
ze signal reload --host 10.0.0.1 --port 2222
```
<!-- source: cmd/ze/internal/ssh/client/client.go -- ze.ssh.host, ze.ssh.port env vars -->

## Signals

### Via SSH (preferred)

| Command | Effect |
|---------|--------|
| `ze signal reload` | Reload configuration (add/remove/update peers) |
| `ze signal stop` | Graceful shutdown (no GR marker) |
| `ze signal restart` | Graceful restart (writes GR marker for RFC 4724) |
| `ze signal quit` | Goroutine dump to stderr, then exit |
<!-- source: cmd/ze/signal/main.go -- Run -->

### Via Unix Signals

| Signal | Effect |
|--------|--------|
| `SIGHUP` | Reload configuration |
| `SIGTERM` / `SIGINT` | Graceful shutdown (NOTIFICATION Cease to all peers) |
| `SIGUSR1` | Dump status to stderr |
<!-- source: cmd/ze/hub/main.go -- signal.Notify, SIGHUP/SIGTERM/SIGINT handling -->

### Exit Codes (signal command)

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Daemon not running |
| 2 | No SSH credentials (run `ze init`) |
| 4 | Signal delivery failed |
<!-- source: cmd/ze/signal/main.go -- ExitSuccess/ExitNotRunning/ExitNoCredentials/ExitSignalFailed -->

## Health Checks

### Liveness

```bash
ze status                      # exit 0 = running, exit 1 = not running
```

This dials the SSH port without completing a handshake. Suitable for systemd watchdog or load balancer TCP check.
<!-- source: cmd/ze/signal/main.go -- RunStatus, net.Dialer -->

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
ze cli --run "peer list"       # brief peer list with state
ze cli --run "bgp summary"    # summary table with uptime and prefix counts
```

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
| `ze.log.backend` | `stderr` | Log output: stderr, stdout, syslog |
| `ze.log.destination` | -- | Syslog address (when backend=syslog) |
| `ze.log.relay` | `warn` | Plugin stderr relay threshold |
| `ze.ssh.host` | -- | Override SSH host for CLI commands |
| `ze.ssh.port` | -- | Override SSH port for CLI commands |
| `ze.config.dir` | -- | Override config directory |
| `ze.storage.blob` | `true` | Use blob storage (false = filesystem) |
<!-- source: internal/core/slogutil/slogutil.go -- ze.log registration; cmd/ze/internal/ssh/client/client.go -- ze.ssh.host/port; cmd/ze/main.go -- ze.storage.blob, ze.config.dir -->

## CLI Flags

| Flag | Purpose |
|------|---------|
| `-d`, `--debug` | Enable debug logging (sets `ze.log=debug`) |
| `-f <file>` | Use filesystem storage (bypass blob database) |
| `--plugin <name>` | Load additional plugin (repeatable) |
| `--pprof <addr:port>` | Start pprof HTTP server for profiling |
| `--chaos-seed <N>` | Chaos testing seed (0=off, -1=time-based) |
| `--chaos-rate <0-1>` | Chaos fault probability |
| `-V`, `--version` | Show version |
| `--plugins` | List registered plugins (with optional `--json`) |
<!-- source: cmd/ze/main.go -- global flag parsing -->

## systemd

Example unit file:

```ini
[Unit]
Description=Ze BGP Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/ze /etc/ze/config.conf
ExecReload=/usr/local/bin/ze signal reload
ExecStop=/usr/local/bin/ze signal stop
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

# Logging goes to journald via stderr
StandardError=journal
SyslogIdentifier=ze

[Install]
WantedBy=multi-user.target
```

Initialize credentials before starting:

```bash
sudo ze init
sudo systemctl enable --now ze
```

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
   ze run bgp log set bgp.fsm debug
   ze cli --run "bgp monitor event state"
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
   ze run bgp log set bgp.reactor debug
   ```

### Daemon Won't Start

1. **Config parse error:** Run `ze config validate config.conf` first
2. **Port in use:** Check if another ze instance or BGP daemon holds port 179
3. **SSH port conflict:** Default SSH is 2222. Check with `netstat -tlnp | grep 2222`
4. **Missing credentials:** Run `ze init` before starting
<!-- source: cmd/ze/config/cmd_validate.go -- cmdValidate; internal/component/ssh/ -- SSH server -->

### Plugin Not Working

1. **Check plugin is loaded:**
   ```bash
   ze --plugins                  # list compiled-in plugins
   ```

2. **Check plugin is bound to peer:** Config must have `process <name> { receive [...] }` on the peer

3. **Check plugin logs:** Set `ZE_LOG_PLUGIN_RELAY=debug` to see plugin stderr output

4. **Plugin not reaching Ready state:** Enable `ZE_LOG_PLUGIN=debug` and look for startup stage failures
<!-- source: internal/component/plugin/server/server.go -- plugin stage timeout; internal/component/plugin/registry/registry.go -- plugin registration -->

### No Routes in RIB

1. **Check RIB plugin is loaded and bound:**
   ```bash
   ze cli --run "rib status"
   ```

2. **Check peer is sending updates:**
   ```bash
   ze cli --run "bgp monitor peer transit-a event update direction received"
   ```

3. **Check RPKI validation:** If bgp-rpki is loaded, routes may be pending validation. Check:
   ```bash
   ze cli --run "rpki status"
   ```

### Collecting Debug Information

For bug reports, collect:

```bash
ze --version                   # ze version
ze config validate config.conf        # config validity
ze status                      # daemon state
ze cli --run "peer list"       # peer states
ze cli --run "bgp summary"    # session summary
ze env list -v                 # effective configuration
ze signal quit                 # goroutine dump (kills daemon!)
```
