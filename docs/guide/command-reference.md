# Command Reference

Ze commands fall into two categories: **shell commands** that run locally
and **runtime commands** sent to the running daemon via SSH.

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

### ze init

Bootstrap the database (interactive or piped).

```
ze init                          # Interactive setup
ze init -managed                 # Fleet mode
ze init -force                   # Replace existing database
```

Prompts for: username, password, host (127.0.0.1), port (2222), name (hostname).

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

### ze plugin

Plugin management.

```
ze plugin <name> [args]          # Run plugin CLI handler
ze plugin test                   # Test plugin schema/config
```

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

### Peer Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `peer list` | read-only | List all peers (IP, ASN, state, uptime) |
| `peer <sel> detail` | read-only | Detailed peer info (config, state, counters) |
| `peer <sel> capabilities` | read-only | Negotiated capabilities |
| `peer <sel> statistics` | read-only | Per-peer update statistics with rates |
| `bgp summary` | read-only | BGP summary table |
| `peer <sel> add <config>` | write | Add peer dynamically |
| `peer <sel> remove` | write | Remove peer |
| `peer <sel> pause` | write | Pause read loop (flow control) |
| `peer <sel> resume` | write | Resume read loop |
| `peer <sel> teardown [<code>] [<msg>]` | write | Graceful close with NOTIFICATION |
| `peer <sel> save` | write | Save running peers to config |

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

### RIB Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `rib status` | read-only | RIB summary (peer count, routes, families) |
| `rib routes [received\|sent] [peer] [family]` | read-only | Adj-RIB-In/Out inspection |
| `rib best [<prefix>]` | read-only | Best-path per prefix |
| `rib best status` | read-only | Best-path computation status |
| `rib clear in [peer]` | write | Clear Adj-RIB-In |
| `rib clear out [peer]` | write | Regenerate and re-advertise Adj-RIB-Out |

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

### Cache Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `cache list` | read-only | List cached message IDs |
| `cache <id> retain` | write | Pin in cache (prevent eviction) |
| `cache <id> release` | write | Release from cache |
| `cache <id> expire` | write | Remove immediately |
| `cache <id> forward <peer-sel>` | write | Re-inject UPDATE to peer(s) |

Batch operations: `cache <id1>,<id2> <action> [args]`.

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

### Metrics

| Command | Access | Purpose |
|---------|--------|---------|
| `metrics show` | read-only | Prometheus text format metrics |
| `metrics list` | read-only | List metric names |

### Logging

| Command | Access | Purpose |
|---------|--------|---------|
| `log show` | read-only | List subsystems with current log levels |
| `log set <subsystem> <level>` | write | Set log level at runtime |

Levels: debug, info, warn, err, disabled.

### Plugin Configuration (from plugin context)

| Command | Access | Purpose |
|---------|--------|---------|
| `bgp plugin encoding <json\|text>` | write | Set event encoding |
| `bgp plugin format <hex\|base64\|parsed\|full>` | write | Set wire format display |
| `bgp plugin ack <sync\|async>` | write | Set ACK timing |

### Discovery

| Command | Access | Purpose |
|---------|--------|---------|
| `help` | read-only | List available subcommands |
| `command-list` | read-only | List all commands with descriptions |
| `command-help <name>` | read-only | Detailed help for a command |
| `event-list` | read-only | List available event types |

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

---

## Signal Handling

The daemon handles these Unix signals directly:

| Signal | Effect |
|--------|--------|
| `SIGHUP` | Reload configuration |
| `SIGTERM` / `SIGINT` | Graceful shutdown |
| `SIGUSR1` | Dump status to stderr |
