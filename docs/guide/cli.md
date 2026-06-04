# CLI Reference

Ze provides an interactive CLI and single-command execution for runtime queries and control. All CLI access goes through the daemon's SSH server.
<!-- source: internal/component/cli/client/main.go -- Run -->

## Usage

```bash
ze cli                              # Interactive CLI with tab completion
ze cli -c "show bgp peer list"   # Execute single command and exit
ze show peer upstream1 detail         # Read-only query (safe for scripts)
ze cli -c "peer upstream1 teardown 2" # One-shot command (full access)
```

### Modes

| Command | Access | Use Case |
|---------|--------|----------|
| `ze cli` | Interactive, full | Exploring, monitoring, operating |
| `ze show <cmd>` | Read-only | Scripting, monitoring dashboards |
| `ze cli -c <cmd>` | Full | Automation, route injection |

## Peer Commands

| Command | Description |
|---------|-------------|
| `show bgp peer list` | List all peers (brief) |
| `show bgp peer <sel> detail` | Show peer details and statistics |
| `peer <sel> teardown <code>` | Graceful session closure with NOTIFICATION |
| `delete bgp peer <name>` | Remove peer |
| `peer <sel> pause` | Pause reading from peer (flow control) |
| `peer <sel> resume` | Resume reading from peer |
| `show bgp peer <sel> capabilities` | Show negotiated capabilities |
| `show bgp summary` | BGP summary table |

**Peer selector:** `*` (all), exact IP, glob patterns (`192.168.*.*`), exclusion (`!addr`), or peer name.
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- peer command handlers; internal/component/bgp/reactor/reactor_api.go -- getMatchingPeers -->

## Route Commands

| Command | Description |
|---------|-------------|
| `peer <sel> update text <attrs> nlri <family> <op> <prefix>` | Text-format UPDATE |
| `peer <sel> update hex <hex>` | Hex-format UPDATE |
| `show bgp rib received [peer] [family]` | Show Adj-RIB-In |
| `show bgp rib sent [peer] [family]` | Show Adj-RIB-Out |
| `clear bgp rib in [peer]` | Clear Adj-RIB-In |
| `clear bgp rib out [peer]` | Clear Adj-RIB-Out |
<!-- source: internal/component/bgp/plugins/cmd/rib/ -- RIB proxy RPCs; internal/component/bgp/plugins/cmd/update/ -- update RPCs -->

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
| `show rpki status` | RTR session count and VRP counts |
| `show rpki cache` | Cache server connection details |
| `show rpki roa` | ROA table summary |
| `show rpki summary` | Validation statistics |

## Daemon Control

| Command | Description |
|---------|-------------|
| `daemon shutdown` | Graceful shutdown |
| `route-refresh <family>` | Send route refresh request |
| `help` | List all commands |
| `command-list` | All commands with descriptions |
| `command-help <name>` | Detailed help for a command |
<!-- source: internal/component/cmd/meta/ -- help/discovery RPCs; internal/component/bgp/plugins/cmd/cache/ -- cache RPCs -->

## Signals

| Command | Description |
|---------|-------------|
| `ze signal reload` | Reload configuration |
| `ze signal stop` | Graceful shutdown (no GR marker) |
| `ze signal restart` | Graceful restart (with GR marker) |
| `ze signal quit` | Goroutine dump and exit |
| `ze status` | Check if daemon is running |
<!-- source: cmd/ze/signal/main.go -- Run, RunStatus -->

## Interactive Features

In `ze cli` interactive mode:

- **Tab completion** for commands, peer names, address families, and log levels
- **Pipe operators:** `| json`, `| table`, `| match <regex>`, `| count`, `| no-more`
- **History** persisted across sessions
- **Ctrl-C** cancels current command, **Ctrl-D** exits
<!-- source: internal/component/cli/client/main.go -- pipe operators, bubbletea model, history -->
