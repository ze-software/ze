# Command Reference

Ze commands fall into two categories: **shell commands** that run locally
and **runtime commands** sent to the running daemon via SSH.
<!-- source: cmd/ze/main.go -- main dispatch -->

## Shell Commands

Run directly from the terminal. No daemon required (except `ze signal`, `ze status`,
`ze show`, `ze run`, `ze cli`).

### ze

Start the daemon or access subcommands.

```
ze <config-file>                 # Start daemon with config
ze start                         # Start daemon from database
```

| Flag | Purpose |
|------|---------|
| `-d`, `--debug` | Enable debug logging |
| `-f <file>` | Use filesystem directly, bypass blob store |
| `--plugin <name>` | Load plugin before starting (repeatable) |
| `--plugins` | List available internal plugins |
| `--pprof <addr:port>` | Start pprof HTTP server |
| `-V`, `--version` | Show version |
| `--chaos-seed <N>` | Enable chaos self-test mode |
| `--chaos-rate <0-1>` | Fault probability per operation |
| `--server <host:port>` | Override hub address for managed mode |
| `--name <name>` | Override client name for managed mode |
| `--token <token>` | Override auth token for managed mode |
<!-- source: cmd/ze/main.go -- global flag parsing -->

### ze config validate

Validate a configuration file without starting the daemon.

```
ze config validate <config-file>
ze config validate -q <config-file>     # Quiet: exit code only
ze config validate --json <config-file> # JSON output
```

| Flag | Purpose |
|------|---------|
| `-v` | Verbose output |
| `-q` | Quiet mode (exit code only) |
| `--json` | JSON output |

Exit codes: 0 = valid, 1 = invalid, 2 = file not found.
<!-- source: cmd/ze/config/cmd_validate.go -- cmdValidate -->

### ze config

Configuration management.

**Editing:**

```
ze config edit [file]            # Interactive editor
ze config set <file> <path> <value>
```

**Storage:**

```
ze config import <file>...       # Import files into the database
ze config import --name <n> <file>  # Import under a different name
ze config rename <old> <new>     # Rename a config in the database
ze config ls [prefix]            # List files in database
ze config cat <key>              # Print database entry
```

**Inspection:**

```
ze config validate <file>        # Validate configuration file
ze config dump <file>            # Dump parsed configuration
ze config diff <f1> <f2>         # Compare two configs
ze config diff <N> <file>        # Compare with rollback revision
ze config fmt <file>             # Format and normalize
```

**History:**

```
ze config history <file>         # List rollback revisions
ze config rollback <N> <file>    # Restore revision N
ze config archive <name> <file>  # Archive config
```

**Migration:**

```
ze config migrate <file>         # Convert old format to current
```
<!-- source: cmd/ze/config/main.go -- subcommandHandlers, storageHandlers -->

| Flag | Purpose |
|------|---------|
| `-f` | Bypass database, use filesystem directly |
| `-o <output>` | Output file (migrate) |

### ze signal

Send commands to the running daemon via SSH.

```
ze signal reload                 # Reload configuration
ze signal stop                   # Graceful shutdown (no GR marker)
ze signal restart                # Graceful restart (with GR marker)
ze signal quit                   # Immediate exit + goroutine dump
```

| Flag | Purpose |
|------|---------|
| `--host` | SSH host (default: 127.0.0.1 or `ze_ssh_host`) |
| `--port` | SSH port (default: 2222 or `ze_ssh_port`) |

Exit codes: 0 = ok, 1 = not running, 4 = command failed.
<!-- source: cmd/ze/signal/main.go -- Run, ExitSuccess/ExitNotRunning/ExitNoCredentials/ExitSignalFailed -->

### ze status

Check if the daemon is running.

```
ze status
```

| Flag | Purpose |
|------|---------|
| `--host` | SSH host |
| `--port` | SSH port |

Exit codes: 0 = running, 1 = not running.
<!-- source: cmd/ze/signal/main.go -- RunStatus -->

### ze bgp

BGP protocol tools (offline, no daemon needed).

```
ze bgp decode <hex>              # Decode BGP message hex to JSON
ze bgp encode <route-command>    # Encode route command to BGP hex
ze bgp plugin                    # Interactive plugin simulator
```

**decode flags:**

| Flag | Purpose |
|------|---------|
| `--open` | Decode as OPEN message |
| `--update` | Decode as UPDATE message |
| `--nlri <family>` | Decode as NLRI for family |
| `-f <family>` | Address family |
| `--json` | JSON output |
| `--plugin <name>` | Load plugin (repeatable) |

**encode flags:**

| Flag | Purpose |
|------|---------|
| `-f <family>` | Address family (default: ipv4/unicast) |
| `-a <asn>` | Local ASN (default: 65533) |
| `-z <asn>` | Peer ASN (default: 65533) |
| `-i` | Enable feature |
| `-n` | Dry run |
| `--no-header` | Exclude BGP header |
| `--asn4` | 4-byte ASN (default: true) |
<!-- source: cmd/ze/bgp/main.go -- Run; cmd/ze/bgp/decode.go -- cmdDecode; cmd/ze/bgp/encode.go -- cmdEncode -->

### ze exabgp

ExaBGP compatibility tools.

```
ze exabgp plugin <cmd> [args]    # Run ExaBGP plugin with ze
ze exabgp migrate <file>         # Convert ExaBGP config to ze
```

**plugin flags:**

| Flag | Purpose |
|------|---------|
| `--family <family>` | Address family (repeatable) |
| `--route-refresh` | Enable route-refresh |
| `--add-path <mode>` | ADD-PATH mode: receive, send, both |
<!-- source: cmd/ze/exabgp/main.go -- Run, cmdPlugin, cmdMigrate -->

### ze schema

Schema discovery.

```
ze schema list                   # List registered schemas
ze schema show <module>          # Show YANG module content
ze schema handlers               # List handler-to-module mapping
ze schema methods [module]       # List RPCs from YANG
ze schema events [module]        # List notifications
ze schema protocol               # Show protocol version
```

All subcommands accept `--json`.
<!-- source: cmd/ze/schema/main.go -- Run -->

### ze yang

YANG analysis.

```
ze yang completion               # Detect prefix collisions
ze yang tree                     # Print unified tree
ze yang doc [command]            # Command documentation
```

| Flag | Purpose |
|------|---------|
| `--json` | JSON output |
| `--commands` | Show command tree (tree) |
| `--config` | Show config tree (tree) |
| `--min-prefix <N>` | Minimum prefix length (completion, default: 1) |
| `--list` | List commands (doc) |
<!-- source: cmd/ze/yang/main.go -- Run -->

### ze init

Bootstrap the database (interactive or piped).

```
ze init                          # Interactive setup
ze init -managed                 # Fleet mode
ze init -force                   # Replace existing database
```

Prompts for: username, password, host (127.0.0.1), port (2222), name (hostname).
<!-- source: cmd/ze/init/main.go -- Run, defaultHost, defaultPort -->

### ze start --web

Add the HTTPS web interface alongside the BGP daemon. The web server runs on a
separate port and provides configuration viewing, editing, and admin commands.

```
ze start --web                                            # Start daemon + web on default port
ze start --web --listen 127.0.0.1:8443                    # Web on specific address
ze start --web --insecure-web --listen 127.0.0.1:8443     # No authentication (testing only)
```

| Flag | Purpose |
|------|---------|
| `--web` | Enable the web interface alongside the daemon |
| `--listen <addr>` | Web server listen address (default: `0.0.0.0:8443`) |
| `--insecure-web` | Disable authentication (requires `--listen 127.0.0.1:*`) |

The web server uses a self-signed ECDSA P-256 certificate (persisted in zefs) with SANs
for localhost, 127.0.0.1, ::1, and the listen address.

See [Web Interface Guide](web-interface.md) for full usage documentation.
<!-- source: cmd/ze/main.go -- cmdStart, cmd/ze/hub/main.go -- startWebServer -->

### ze data

Low-level blob store management.

```
ze data import <file>...           # Import files into blob
ze data rm <key>...                # Remove entries
ze data ls [prefix]                # List entries
ze data cat <key>                  # Print entry content
```

| Flag | Purpose |
|------|---------|
| `--path <store>` | Blob store path |
<!-- source: cmd/ze/data/main.go -- Run, subcommandHandlers -->

### ze plugin

Plugin management.

```
ze plugin <name> [args]          # Run plugin CLI handler
ze plugin test                   # Test plugin schema/config
```
<!-- source: cmd/ze/plugin/main.go -- Run -->

### ze completion

Generate shell completion scripts for bash, zsh, fish, and nushell. The scripts provide tab completion for subcommands, flags, plugin names, YANG schema modules, and `show`/`run` command trees.

```
ze completion bash
ze completion zsh
ze completion fish
ze completion nushell
```

#### Installation

| Shell | Quick (current session) | Persistent |
|-------|------------------------|------------|
| Bash | `eval "$(ze completion bash)"` | `ze completion bash > /etc/bash_completion.d/ze` |
| Zsh | `eval "$(ze completion zsh)"` | `ze completion zsh > ~/.zsh/completions/_ze && autoload -Uz compinit && compinit` |
| Fish | `ze completion fish \| source` | `ze completion fish > ~/.config/fish/completions/ze.fish` |
| Nushell | `ze completion nushell \| save -f ($nu.default-config-dir \| path join "completions" "ze.nu")` | Add `source completions/ze.nu` to `config.nu` |
<!-- source: cmd/ze/completion/main.go -- Run -->

### ze env

Environment variable management.

```
ze env                           # Show all env vars
ze env list                      # Show all with current values
ze env get <key>                 # Show single env var
```

| Flag | Purpose |
|------|---------|
| `-v`, `--verbose` | Verbose output (list) |
<!-- source: cmd/ze/environ/main.go -- Run -->

### ze-perf

BGP propagation latency benchmark tool. Separate binary from `ze`.

<!-- source: cmd/ze-perf/main.go -- ze-perf CLI entry point -->

```
ze-perf <command> [flags]
```

| Command | Purpose |
|---------|---------|
| `run` | Run benchmark against a BGP DUT |
| `report` | Generate comparison report from result files |
| `track` | Track performance history and detect regressions |

#### ze-perf run

Run a BGP propagation benchmark against a device under test (DUT). Establishes
sender and receiver sessions with the DUT, injects routes from the sender, and
measures how quickly they propagate through to the receiver.

<!-- source: cmd/ze-perf/run.go -- run subcommand -->

```
ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000
ze-perf run --dut-addr 172.31.0.5 --dut-asn 65000 --dut-name gobgp --routes 10000 --json
ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000 --family ipv6/unicast
ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000 --force-mp --repeat 10
```

**DUT flags:**

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--dut-addr` | string | (required) | DUT BGP address |
| `--dut-port` | int | 179 | DUT BGP port |
| `--dut-asn` | int | (required) | DUT autonomous system number |
| `--dut-name` | string | `unknown` | DUT implementation name (appears in results) |
| `--dut-version` | string | | DUT version string |

**Sender/receiver flags:**

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--sender-addr` | string | `127.0.0.1` | Sender local address |
| `--sender-asn` | int | `65001` | Sender autonomous system number |
| `--sender-port` | int | `0` | DUT port for sender (0 = use `--dut-port`) |
| `--receiver-addr` | string | `127.0.0.2` | Receiver local address |
| `--receiver-asn` | int | `65002` | Receiver autonomous system number |
| `--receiver-port` | int | `0` | DUT port for receiver (0 = use `--dut-port`) |

**Benchmark flags:**

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--routes` | int | `1000` | Number of routes to inject |
| `--family` | string | `ipv4/unicast` | Address family (`ipv4/unicast` or `ipv6/unicast`) |
| `--force-mp` | bool | `false` | Force MP_REACH_NLRI for IPv4 unicast |
| `--seed` | uint64 | `0` | Deterministic seed (0 = random) |
| `--warmup` | duration | `2s` | Warmup delay after session establishment |
| `--connect-timeout` | duration | `10s` | TCP connection timeout |
| `--duration` | duration | `60s` | Maximum time to wait for convergence per iteration |

**Iteration flags:**

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--repeat` | int | `5` | Number of benchmark iterations |
| `--warmup-runs` | int | `1` | Warmup iterations (discarded from results) |
| `--iter-delay` | duration | `3s` | Delay between iterations |
| `--batch-size` | int | `0` | UPDATE batch size (0 = single UPDATE per prefix) |

**Output flags:**

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--json` | bool | `false` | JSON output |
| `--output` | string | | Output file path (implies `--json`) |

Exit codes: 0 = success, 1 = error (missing flags, validation failure, benchmark failure).

#### ze-perf report

Generate a comparison report from one or more result JSON files.

<!-- source: cmd/ze-perf/report.go -- report subcommand -->

```
ze-perf report result-ze.json result-gobgp.json
ze-perf report --html result-ze.json result-gobgp.json > report.html
```

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--md` | bool | `true` | Markdown output |
| `--html` | bool | `false` | HTML output (overrides `--md`) |

Reads result JSON files produced by `ze-perf run --json` and generates a
side-by-side comparison table.

#### ze-perf track

Track performance history and detect regressions from an NDJSON file.

<!-- source: cmd/ze-perf/track.go -- track subcommand -->

```
ze-perf track history.ndjson
ze-perf track --check history.ndjson
ze-perf track --html history.ndjson > trend.html
ze-perf track --check --threshold-convergence 15 history.ndjson
```

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--md` | bool | `true` | Markdown output |
| `--html` | bool | `false` | HTML output (overrides `--md`) |
| `--check` | bool | `false` | Check for regressions (exit 1 on regression) |
| `--last` | int | `0` | Only consider last N entries (0 = all) |
| `--threshold-convergence` | int | `20` | Convergence regression threshold (%) |
| `--threshold-throughput` | int | `20` | Throughput regression threshold (%) |
| `--threshold-p99` | int | `30` | P99 latency regression threshold (%) |

<!-- source: internal/perf/regression.go -- regression detection thresholds -->

Exit codes: 0 = no regression (or report mode), 1 = regression detected or error.

---

## Runtime Commands

Commands sent to the running daemon. Access through three entry points:

| Entry | Access | Usage |
|-------|--------|-------|
| `ze cli` | Full (interactive) | Exploration, monitoring |
| `ze show <cmd>` | Read-only | Scripting, dashboards |
| `ze run <cmd>` | Full | Automation, route injection |

`ze cli` accepts `--run <command>` for single-shot execution and
`--format <format>` (default: yaml).
<!-- source: cmd/ze/cli/main.go -- Run; cmd/ze/show/main.go -- Run; cmd/ze/run/main.go -- Run -->

### Peer Selector

Many commands take a `peer <selector>` argument:

| Selector | Example | Description |
|----------|---------|-------------|
| `*` | `peer *` | All peers |
| Name | `peer upstream1` | By configured peer name |
| IP address | `peer 10.0.0.1` | By peer IP |
| ASN | `peer as65001` | By remote ASN, case-insensitive (matches all peers with that ASN) |
| Glob | `peer 192.168.*.*` | Pattern match |
| Exclusion | `peer !10.0.0.1` | All except this peer |
| ASN exclusion | `peer !as65001` | All except peers with this ASN |
<!-- source: internal/component/bgp/reactor/reactor_api.go -- selectPeers; internal/component/bgp/plugins/cmd/peer/peer.go -- peer command handler -->

### Peer Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `peer list` | read-only | List all peers (IP, ASN, state, uptime) |
| `peer <sel> detail` | read-only | Detailed peer info (config, state, counters, `prefix-updated` date, `prefix-stale` warning) |
| `peer <sel> capabilities` | read-only | Negotiated capabilities |
| `peer <sel> statistics` | read-only | Per-peer update statistics with rates |
| `bgp summary` | read-only | BGP summary table |
| `peer <sel> pause` | write | Pause read loop (flow control) |
| `peer <sel> resume` | write | Resume read loop |
| `peer <sel> teardown [<code>] [<msg>]` | write | Graceful close with NOTIFICATION |
| `peer <sel> flush` | write | Block until all queued updates for peer are on the wire |
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- peer command handlers; internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang -->

### Set Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `set bgp peer <name> with <config>` | write | Create peer with configuration |
| `set bgp peer <sel> save` | write | Save running peers to config |

#### Peer Config Keys

Config keys are parsed from the YANG `peer-fields` schema via `ParseInlineArgs`. Container prefixes (`remote`, `local`) scope sub-keys. The parser walks the YANG tree to determine how many tokens each field consumes (leaf = name + value, container = name + recurse into children).

| Key | Value | Required | Description |
|-----|-------|----------|-------------|
| `remote ip` | IP address | Yes | Peer remote IP address |
| `remote as` | ASN (uint32) | Yes | Peer AS number |
| `local as` | ASN (uint32) | No | Local AS override |
| `local ip` | IP address | No | Local IP for this session |
| `router-id` | IPv4 address | No | Router ID override |
| `timer hold-time` | seconds (0-86400) | No | Hold time (default: 90) |
| `timer connect-retry` | seconds | No | Connect retry interval (default: 120) |
| `local connect` | true/false | No | Initiate outbound connections (default: true) |
| `remote accept` | true/false | No | Accept inbound connections (default: true) |
| `description` | text | No | Peer description |
| `link-local` | IPv6 address | No | Link-local next-hop |
| `port` | 1-65535 | No | Per-peer listen port |
| `group-updates` | enable/disable | No | UPDATE grouping |

Example: `set bgp peer upstream1 with remote ip 10.0.0.1 remote as 65001 local as 65000 timer hold-time 90 local connect false`

<!-- source: internal/component/config/setparser_inline.go -- ParseInlineArgs YANG-driven parser -->
<!-- source: internal/component/plugin/server/node_with.go -- HandleNodeWith generic set handler -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- HandleBgpPeerWith, preparePeerTree -->

### Del Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `del bgp peer <sel>` | write | Remove peer |
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- del peer handler -->

### Update Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `update bgp peer <sel> prefix` | write | Update prefix maximums from PeeringDB |
<!-- source: internal/component/cmd/update/update.go -- update verb RPC registration; internal/component/cmd/update/schema/ze-cli-update-cmd.yang -->

### Route Injection

```
peer <sel> update text <attrs> nlri <family> <op> <prefixes>
peer <sel> update hex <hex-data>
peer <sel> update b64 <b64-data>
peer <sel> raw [<type>] <encoding> <data>
```

Text format attributes:

| Attribute | Syntax |
|-----------|--------|
| `origin` | `origin set igp` / `egp` / `incomplete` |
| `nhop` | `nhop set 192.168.1.1` or `nhop set self` |
| `med` | `med set 100` |
| `local-preference` | `local-preference set 200` |
| `as-path` | `as-path set [ 65001 65002 ]` |
| `community` | `community set [ 65000:100 no-export ]` |
| `large-community` | `large-community set [ 65000:1:1 ]` |
| `extended-community` | `extended-community set [ rt:65000:100 ]` |

NLRI operations: `nlri <family> add <prefixes>`, `nlri <family> del <prefixes>`,
`nlri <family> eor`.
<!-- source: internal/component/bgp/plugins/cmd/update/ -- update text/hex/b64 parsing; internal/component/bgp/plugins/cmd/raw/ -- raw message injection -->

### RIB Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `rib status` | read-only | RIB summary (peer count, routes, families) |
| `rib routes [received\|sent] [peer] [family]` | read-only | Adj-RIB-In/Out inspection |
| `rib best [<prefix>]` | read-only | Best-path per prefix |
| `rib best status` | read-only | Best-path computation status |
| `rib clear in <selector>` | write | Clear Adj-RIB-In (`*` for all peers) |
| `rib clear out <selector> [family]` | write | Regenerate and re-advertise Adj-RIB-Out (`*` for all peers, optional family filter) |
<!-- source: internal/component/bgp/plugins/cmd/rib/ -- RIB proxy RPCs; internal/component/bgp/plugins/rib/ -- RIB plugin -->

### Commit (Atomic Updates)

| Command | Access | Purpose |
|---------|--------|---------|
| `commit <name> start [peer]` | write | Begin named update window |
| `commit <name> end` | write | Flush queued updates |
| `commit <name> eor` | write | Flush updates and send End-of-RIB |
| `commit <name> show` | read-only | Show queue status |
| `commit <name> rollback` | write | Discard queued updates |
| `commit <name> withdraw route <prefix>` | write | Withdraw prefix from window |
| `commit list` | read-only | List active commits |
<!-- source: internal/component/cmd/commit/ -- commit command RPCs -->

### Cache Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `cache list` | read-only | List cached message IDs |
| `cache <id> retain` | write | Pin in cache (prevent eviction) |
| `cache <id> release` | write | Release from cache |
| `cache <id> expire` | write | Remove immediately |
| `cache <id> forward <peer-sel>` | write | Re-inject UPDATE to peer(s) |

Batch operations: `cache <id1>,<id2> <action> [args]`.
<!-- source: internal/component/cmd/cache/ -- cache command RPCs -->

### Event Monitoring

```
bgp monitor [peer <sel>] [event <types>] [direction <dir>]
```

| Filter | Values |
|--------|--------|
| `peer` | IP address, `*` |
| `event` | update, open, notification, keepalive, refresh, state, negotiated (comma-separated) |
| `direction` | sent, received |

Streaming command: use in interactive `ze cli` or via SSH.
<!-- source: internal/component/bgp/plugins/cmd/monitor/ -- monitor streaming RPCs -->

### Metrics

| Command | Access | Purpose |
|---------|--------|---------|
| `metrics show` | read-only | Prometheus text format metrics |
| `metrics list` | read-only | List metric names |
<!-- source: internal/component/cmd/metrics/ -- metrics show/list RPCs -->

### Logging

| Command | Access | Purpose |
|---------|--------|---------|
| `log show` | read-only | List subsystems with current log levels |
| `log set <subsystem> <level>` | write | Set log level at runtime |

Levels: debug, info, warn, err, disabled.
<!-- source: internal/component/cmd/log/ -- log show/set RPCs; internal/core/slogutil/slogutil.go -- level definitions -->

### Plugin Configuration (from plugin context)

| Command | Access | Purpose |
|---------|--------|---------|
| `bgp plugin encoding <json\|text>` | write | Set event encoding |
| `bgp plugin format <hex\|base64\|parsed\|full>` | write | Set wire format display |
| `bgp plugin ack <sync\|async>` | write | Set ACK timing |
<!-- source: internal/component/cmd/subscribe/ -- subscribe/unsubscribe RPCs -->

### Discovery

| Command | Access | Purpose |
|---------|--------|---------|
| `help` | read-only | List available subcommands |
| `command-list` | read-only | List all commands with descriptions |
| `command-help <name>` | read-only | Detailed help for a command |
| `event-list` | read-only | List available event types |
<!-- source: internal/component/cmd/meta/ -- help/discovery RPCs -->

---

## Interactive CLI Features

Inside `ze cli`:

| Feature | Syntax |
|---------|--------|
| Pipe: filter lines | `peer list \| match established` |
| Pipe: count | `peer list \| count` |
| Pipe: table format | `rib routes \| table` |
| Pipe: JSON pretty | `peer list \| json` |
| Pipe: JSON compact | `peer list \| json compact` |
| Pipe: disable paging | `peer list \| no-more` |
| Tab completion | Contextual command/argument completion |
<!-- source: cmd/ze/cli/main.go -- pipe operators, interactive model -->

---

## Signal Handling

The daemon handles these Unix signals directly:

| Signal | Effect |
|--------|--------|
| `SIGHUP` | Reload configuration |
| `SIGTERM` / `SIGINT` | Graceful shutdown |
| `SIGUSR1` | Dump status to stderr |
<!-- source: cmd/ze/hub/main.go -- signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP) -->
