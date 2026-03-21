# CLI Reference

Ze provides an interactive CLI and single-command execution for runtime queries and control. All CLI access goes through the daemon's SSH server.

## Usage

```bash
ze cli                              # Interactive CLI with tab completion
ze cli --run "peer list"            # Execute single command and exit
ze show peer upstream1 detail         # Read-only query (safe for scripts)
ze run peer upstream1 teardown 2     # Full access including destructive commands
```

### Modes

| Command | Access | Use Case |
|---------|--------|----------|
| `ze cli` | Interactive, full | Exploring, monitoring, operating |
| `ze show <cmd>` | Read-only | Scripting, monitoring dashboards |
| `ze run <cmd>` | Full | Automation, route injection |

## Peer Commands

| Command | Description |
|---------|-------------|
| `peer list` | List all peers (brief) |
| `peer * show` | Show peer details and statistics |
| `peer <sel> teardown <code>` | Graceful session closure with NOTIFICATION |
| `peer <sel> add <config>` | Dynamic peer addition |
| `peer <sel> remove` | Remove peer |
| `peer <sel> pause` | Pause reading from peer (flow control) |
| `peer <sel> resume` | Resume reading from peer |
| `peer <sel> capabilities` | Show negotiated capabilities |
| `bgp summary` | BGP summary table |

**Peer selector:** `*` (all), exact IP, glob patterns (`192.168.*.*`), exclusion (`!addr`), or peer name.

## Route Commands

| Command | Description |
|---------|-------------|
| `peer <sel> update text <attrs> nlri <family> <op> <prefix>` | Text-format UPDATE |
| `peer <sel> update hex <hex>` | Hex-format UPDATE |
| `rib routes received [peer] [family]` | Show Adj-RIB-In |
| `rib routes sent [peer] [family]` | Show Adj-RIB-Out |
| `rib clear received [peer]` | Clear Adj-RIB-In |
| `rib clear sent [peer]` | Clear Adj-RIB-Out |

See [Route Injection guide](route-injection.md) for UPDATE syntax details.

## Cache Commands

| Command | Description |
|---------|-------------|
| `cache list` | List cached messages |
| `cache forward <id> <peer>` | Forward cached message to peer |
| `cache release <id>` | Release message from cache |

## Event Subscription

| Command | Description |
|---------|-------------|
| `bgp monitor` | Stream live events (see [Monitoring guide](monitoring.md)) |
| `bgp monitor peer <addr> event <type> direction <dir>` | Filtered stream |

## Commit Workflow

Named update windows for atomic route changes:

| Command | Description |
|---------|-------------|
| `commit start <name>` | Begin named update window |
| `commit end <name>` | End window and send updates |
| `commit eor <name>` | Send End-of-RIB for window |
| `commit rollback <name>` | Discard changes |
| `commit show <name>` | Show commit status |
| `commit list` | List named commits |

## RPKI Commands

| Command | Description |
|---------|-------------|
| `rpki status` | RTR session count and VRP counts |
| `rpki cache` | Cache server connection details |
| `rpki roa` | ROA table summary |
| `rpki summary` | Validation statistics |

## Daemon Control

| Command | Description |
|---------|-------------|
| `daemon shutdown` | Graceful shutdown |
| `route-refresh <family>` | Send route refresh request |
| `help` | List all commands |
| `command-list` | All commands with descriptions |
| `command-help <name>` | Detailed help for a command |

## Signals

| Command | Description |
|---------|-------------|
| `ze signal reload` | Reload configuration |
| `ze signal stop` | Graceful shutdown (no GR marker) |
| `ze signal restart` | Graceful restart (with GR marker) |
| `ze signal quit` | Goroutine dump and exit |
| `ze status` | Check if daemon is running |

## Interactive Features

In `ze cli` interactive mode:

- **Tab completion** for commands and peer names
- **Pipe operators:** `| json`, `| table`, `| match <regex>`, `| count`, `| no-more`
- **History** persisted across sessions
- **Ctrl-C** cancels current command, **Ctrl-D** exits
