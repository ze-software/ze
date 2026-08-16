# CLI Commands

### Protocol Tools

| Command | Description |
|---------|-------------|
| `ze bgp decode` | Decode BGP message from hex to JSON |
| `ze bgp encode` | Encode text route command to BGP wire hex |

<!-- source: internal/component/bgp/cli/main.go -- bgp decode/encode dispatch -->

### Configuration Management

| Command | Description |
|---------|-------------|
| `ze config validate <file>` | Validate configuration file |
| `ze config edit` | Interactive configuration editor |
| `ze config migrate` | Convert an older ze config to the current format |
| `ze config fmt` | Format and normalize config file |
| `ze config dump` | Dump parsed configuration tree |
| `ze config diff <a> <b>` | Compare two configuration files |
| `ze config set` | Set a configuration value programmatically |
| `ze config import` | Import a configuration file into ze |
| `ze config rename` | Rename a configuration element |
| `ze config archive <name>` | Archive config to a named destination ([guide](../../guides/config-archive/index.md)) |
| `ze config history` | List rollback revisions |
| `ze config rollback <N>` | Restore revision N |

<!-- source: internal/component/config/cli/main.go -- config subcommand dispatch -->
<!-- source: internal/component/config/cli/cmd_archive.go -- archive subcommand -->
<!-- source: internal/component/config/cli/cmd_validate.go -- validate command -->
<!-- source: internal/component/config/cli/cmd_migrate.go -- migrate command -->
<!-- source: internal/component/config/cli/cmd_dump.go -- dump command -->
<!-- source: internal/component/config/cli/cmd_diff.go -- diff command -->

### Schema Discovery

| Command | Description |
|---------|-------------|
| `ze schema list` | List all registered YANG schemas |
| `ze schema show <module>` | Show YANG content for a module |
| `ze schema handlers` | List handler→module mapping |
| `ze schema methods [module]` | List RPCs from YANG modules |
| `ze schema events` | List notifications from YANG |
| `ze schema protocol` | Show protocol version and format info |

<!-- source: internal/component/config/yang/cli/main.go -- schema subcommand dispatch -->

### Daemon Control

| Command | Description |
|---------|-------------|
| `ze <config-file>` | Start daemon with configuration |
| `ze signal reload` | Send SIGHUP and reload configuration |
| `ze signal stop` | Graceful shutdown (no GR marker) |
| `ze signal restart` | Graceful restart (writes GR marker, then shuts down) |
| `ze signal status` | Dump process status (SIGUSR1 equivalent) |
| `ze signal quit` | Send SIGQUIT, dump goroutines, and halt |
| `ze status` | Check if daemon is running |

<!-- source: internal/plugins/signal/main.go -- Commands registry -->

### Runtime Interaction

| Command | Description |
|---------|-------------|
| `ze cli` | Interactive CLI (with `-c <cmd>` for single command) |
| `ze show <command>` | Read-only daemon commands |

**Ping and traceroute:** `show ping` and `show traceroute` run one-shot ICMP checks from the router itself using ze's internal engine -- no daemon required, they work as local handlers. `monitor ping` and `monitor traceroute` open a live, continuously-updating view (Ctrl-C to stop); pipe either through `| log` for scrollback, or add `| resolve` / `| origin` to enrich traceroute hops with reverse DNS or ASN info.
`size` sets the ICMP payload length (1-65507), not the total packet: unlike `ping(8)`'s "64 bytes" (56 payload + 8 header), `size 1400` sends 1400 payload bytes on top of the ICMP and IP headers. Both `show ping` and `monitor ping` take `count` (1-100) and `size`; omitting `count` on `monitor ping` is what makes it stream until Ctrl-C, and it additionally takes `interval` (100ms-30s).
```
ze show ping 8.8.8.8 count 5 timeout 3s
ze show ping 8.8.8.8 size 1400
ze show traceroute 8.8.8.8 max-hops 10 probes 1
ze cli -c "monitor ping 8.8.8.8 interval 500ms"
ze cli -c "monitor ping 8.8.8.8 count 5 size 1400"
ze cli -c "monitor traceroute 8.8.8.8 | log | resolve"
```
<!-- source: internal/component/ping/cmd/register.go -- showPingLocal, monitorPingLocal -->
<!-- source: internal/component/traceroute/cmd/register.go -- showTracerouteLocal -->
<!-- source: internal/component/cli/model_ping_test.go -- parsePingMonitorArgs -->
<!-- source: internal/component/cli/model_traceroute_test.go -- parseTracerouteMonitorArgs -->

**Live peer dashboard:** `monitor bgp` in the interactive CLI opens a live dashboard showing router identity, a sortable colour-coded peer table with update rates, and a drill-down detail view. It refreshes every 2 seconds. Use j/k to move, s/S to sort, Enter for detail, and Esc to exit.
<!-- source: internal/component/cli/model_dashboard.go -- isDashboardCommand -->

### Demo: Operate BGP from the live dashboard

Connect to Ze over SSH, open the live BGP dashboard, sort peers, and inspect one session.

[Play the WebM recording](../../assets/demos/cli-dashboard.webm?v=d655c1d08f) · [View the poster](../../assets/demos/cli-dashboard.png?v=6f3f921d23) · [Plain-text transcript](../../assets/demos/cli-dashboard.txt?v=86542601eb)

Recorded with Ze 26.07.18 on macOS and Linux using VHS 0.11.0. Duration: 1 minute 2 seconds.

```console
$ ssh ze-demo
ze# exit
ze> monitor bgp

The dashboard polls three local BGP sessions. Press "s" to sort by the next column, use the arrow keys to select a peer, and press Enter for live session details. Press Escape to return and "q" to leave the dashboard.
```


**Commit confirmed:** The editor supports `commit confirmed <seconds>` for safe remote changes. The config is applied immediately but auto-reverts if `confirm` is not issued within the timeout window (1-3600 seconds). Use `confirm abort` to revert manually. Modeled after Junos commit confirmed.
<!-- source: internal/component/cli/model_load.go -- cmdCommitConfirmed -->

**Command history persistence:** Both `ze config edit` and `ze cli` persist command history to the zefs blob store. History survives application restarts, is stored per-mode (edit vs command) and per-user, with consecutive dedup and a configurable rolling window (default 100, max 10000). Graceful degradation when no blob store is available (in-memory only).
<!-- source: internal/component/cli/history.go -- History type -->

**Login warnings:** When an operator connects via SSH, ze checks for conditions requiring attention and displays warnings in the welcome area. Each warning includes a message and an actionable command. Currently checks for stale prefix data (peers with `prefix-updated` older than 6 months); run `update bgp peer * prefix` to refresh from PeeringDB.
<!-- source: internal/component/ssh/session.go -- createSessionModel login warning collection -->

**Plugin debug shell:** `ze bgp plugin cli` connects to the daemon via SSH, runs the 5-stage plugin handshake, and enters interactive command mode. Developers can test plugin protocol interactions by hand -- sending dispatch-command, subscribe-events, decode-nlri, etc. Accepts defaults (Enter through Q&A) or custom registration parameters (families, plugin name).
<!-- source: internal/component/bgp/cli/cmd_plugin.go -- cmdPluginCLI -->

### Other

| Command | Description |
|---------|-------------|
| `ze plugin <name>` | Run a registered plugin |
| `ze exabgp plugin` | Run ExaBGP plugin with ze bridge |
| `ze exabgp migrate` | Convert ExaBGP config to ze |
| `ze completion bash/zsh/fish/nushell` | Generate shell completion scripts |
| `ze --plugins` | List available internal plugins |

<!-- source: internal/plugins/completion/main.go -- completion subcommand -->
<!-- source: internal/component/plugin/cli/main.go -- plugin subcommand -->
<!-- source: internal/plugins/exabgp/main.go -- exabgp subcommand -->
