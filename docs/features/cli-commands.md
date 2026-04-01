# CLI Commands

### Protocol Tools

| Command | Description |
|---------|-------------|
| `ze bgp decode` | Decode BGP message from hex to JSON |
| `ze bgp encode` | Encode text route command to BGP wire hex |

<!-- source: cmd/ze/bgp/main.go -- bgp decode/encode dispatch -->

### Configuration Management

| Command | Description |
|---------|-------------|
| `ze config validate <file>` | Validate configuration file |
| `ze config edit` | Interactive configuration editor |
| `ze config migrate` | Convert ExaBGP config to ze format |
| `ze config fmt` | Format and normalize config file |
| `ze config dump` | Dump parsed configuration tree |
| `ze config diff <a> <b>` | Compare two configuration files |
| `ze config set` | Set a configuration value programmatically |
| `ze config import` | Import a configuration file into ze |
| `ze config rename` | Rename a configuration element |
| `ze config archive <name>` | Archive config to a named destination ([guide](guide/config-archive.md)) |
| `ze config history` | List rollback revisions |
| `ze config rollback <N>` | Restore revision N |

<!-- source: cmd/ze/config/main.go -- config subcommand dispatch -->
<!-- source: cmd/ze/config/cmd_archive.go -- archive subcommand -->
<!-- source: cmd/ze/config/cmd_validate.go -- validate command -->
<!-- source: cmd/ze/config/cmd_migrate.go -- migrate command -->
<!-- source: cmd/ze/config/cmd_dump.go -- dump command -->
<!-- source: cmd/ze/config/cmd_diff.go -- diff command -->

### Schema Discovery

| Command | Description |
|---------|-------------|
| `ze schema list` | List all registered YANG schemas |
| `ze schema show <module>` | Show YANG content for a module |
| `ze schema handlers` | List handler→module mapping |
| `ze schema methods [module]` | List RPCs from YANG modules |
| `ze schema events` | List notifications from YANG |
| `ze schema protocol` | Show protocol version and format info |

<!-- source: cmd/ze/yang/main.go -- schema subcommand dispatch -->

### Daemon Control

| Command | Description |
|---------|-------------|
| `ze <config-file>` | Start daemon with configuration |
| `ze signal reload` | Send SIGHUP — reload configuration |
| `ze signal stop` | Graceful shutdown (no GR marker) |
| `ze signal restart` | Graceful restart (writes GR marker, then shuts down) |
| `ze signal status` | Dump daemon status (SIGUSR1 equivalent) |
| `ze signal quit` | Send SIGQUIT — goroutine dump + exit |
| `ze status` | Check if daemon is running |

<!-- source: cmd/ze/signal/main.go -- Commands registry -->

### Runtime Interaction

| Command | Description |
|---------|-------------|
| `ze cli` | Interactive CLI (with `-c <cmd>` for single command) |
| `ze show <command>` | Read-only daemon commands |
| `ze run <command>` | All daemon commands |

**Live peer dashboard:** `monitor bgp` in the interactive CLI enters a live dashboard showing router identity, a sortable color-coded peer table with update rates, and drill-down detail view. Auto-refreshes every 2 seconds. Navigate with j/k, sort with s/S, Enter for detail, Esc to exit.
<!-- source: internal/component/cli/model_dashboard.go -- isDashboardCommand -->

**Commit confirmed:** The editor supports `commit confirmed <seconds>` for safe remote changes. The config is applied immediately but auto-reverts if `confirm` is not issued within the timeout window (1-3600 seconds). Use `abort` to revert manually. Modeled after Junos/VyOS commit confirmed.
<!-- source: internal/component/cli/model_load.go -- cmdCommitConfirmed -->

**Command history persistence:** Both `ze config edit` and `ze cli` persist command history to the zefs blob store. History survives application restarts, is stored per-mode (edit vs command) and per-user, with consecutive dedup and a configurable rolling window (default 100, max 10000). Graceful degradation when no blob store is available (in-memory only).
<!-- source: internal/component/cli/history.go -- History type -->

**Login warnings:** When an operator connects via SSH, ze checks for conditions requiring attention and displays warnings in the welcome area. Each warning includes a message and an actionable command. Currently checks for stale prefix data (peers with `prefix-updated` older than 6 months).
<!-- source: internal/component/ssh/session.go -- createSessionModel login warning collection -->

**Plugin debug shell:** `ze bgp plugin cli` connects to the daemon via SSH, runs the 5-stage plugin handshake, and enters interactive command mode. Developers can test plugin protocol interactions by hand -- sending dispatch-command, subscribe-events, decode-nlri, etc. Accepts defaults (Enter through Q&A) or custom registration parameters (families, plugin name).
<!-- source: cmd/ze/bgp/cmd_plugin.go -- cmdPluginCLI -->

### Other

| Command | Description |
|---------|-------------|
| `ze plugin <name>` | Run a registered plugin |
| `ze exabgp plugin` | Run ExaBGP plugin with ze bridge |
| `ze exabgp migrate` | Convert ExaBGP config to ze |
| `ze completion bash/zsh/fish/nushell` | Generate shell completion scripts |
| `ze --plugins` | List available internal plugins |

<!-- source: cmd/ze/completion/main.go -- completion subcommand -->
<!-- source: cmd/ze/plugin/main.go -- plugin subcommand -->
<!-- source: cmd/ze/exabgp/main.go -- exabgp subcommand -->
