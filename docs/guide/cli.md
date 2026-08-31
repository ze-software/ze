---
title: CLI Reference (Guide)
---
# CLI Reference

Ze provides an interactive CLI and single-command execution for runtime queries and control. All CLI access goes through the daemon's SSH server.
<!-- source: internal/component/cli/client/main.go -- Run -->

## Usage

```bash
ze cli                              # Interactive CLI with tab completion
ze cli -c "show bgp peer list"              # Execute single command and exit
ze show bgp peer upstream1 detail           # Read-only query (safe for scripts)
ze cli -c "request peer upstream1 teardown 2" # One-shot command (full access)
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
| `request peer <sel> teardown <code>` | Graceful session closure with NOTIFICATION |
| `delete bgp peer <name>` | Remove peer |
| `request peer <sel> pause` | Pause reading from peer (flow control) |
| `request peer <sel> resume` | Resume reading from peer |
| `show bgp peer <sel> capabilities` | Show negotiated capabilities |
| `show bgp` | BGP summary table |

**Peer selector:** `*` (all), exact IP, glob patterns (`192.168.*.*`), exclusion (`!addr`), or peer name.
<!-- source: internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang -- module ze-peer-cmd -->

## Route Commands

| Command | Description |
|---------|-------------|
| `peer <sel> update text <attrs> nlri <family> <op> <prefix>` | Text-format UPDATE |
| `peer <sel> update hex <hex>` | Hex-format UPDATE |
| `show bgp rib received [peer <selector>] [family <family>]` | Show Adj-RIB-In |
| `show bgp rib advertised [peer <selector>] [family <family>]` | Show Adj-RIB-Out |
| `clear bgp rib in [peer]` | Clear Adj-RIB-In |
| `clear bgp rib out [peer]` | Clear Adj-RIB-Out |
<!-- source: internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang -- show/bgp/rib, clear/bgp/rib; internal/component/bgp/plugins/cmd/rib/rib.go -- route filters -->

See [Route Injection guide](route-injection.md) for UPDATE syntax details.

## Cache Commands

| Command | Description |
|---------|-------------|
| `show cache` | List cached messages |
| `request cache retain <id>` | Prevent cache eviction |
| `request cache release <id>` | Release a cached message |
| `request cache expire <id>` | Remove a cached message immediately |
| `request cache forward <id> <peer>` | Forward a cached message to a peer |
<!-- source: internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang -- module ze-cli-cache-cmd -->

## Event Subscription

| Command | Description |
|---------|-------------|
| `monitor bgp` | Show the live peer dashboard (see [Monitoring guide](monitoring.md)) |
| `monitor event peer <addr> include <type> direction <dir>` | Stream filtered events |
<!-- source: internal/component/bgp/plugins/cmd/monitor/yang/ze-monitor-cmd.yang -- module ze-monitor-cmd; internal/plugins/meta/yang/ze-command-monitor-cmd.yang -- module ze-command-monitor-cmd; internal/component/plugin/server/event_monitor.go -- ParseEventMonitorArgs -->

## Commit Workflow

Named update windows for atomic route changes:

| Command | Description |
|---------|-------------|
| `request commit start <name>` | Begin named update window |
| `request commit end <name>` | End window and send updates |
| `request commit eor <name>` | Send End-of-RIB for window |
| `request commit rollback <name>` | Discard changes |
| `request commit show <name>` | Show commit status |
| `request commit list` | List named commits |

## RPKI Commands

| Command | Description |
|---------|-------------|
| `show bgp rpki` | Validation counters with one row for each cache server |
| `show bgp rpki status` | RTR session count and VRP counts |
| `show bgp rpki cache` | Cache server connection details |
| `show bgp rpki roa` | ROA table summary |
| `show bgp rpki summary` | Validation statistics |
| `show bgp rpki aspa` | ASPA cache, or the providers for a customer AS |
| `request bgp rpki validate <prefix> <origin-asn>` | Validate one prefix against the ROA cache |
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- handleCommand, overviewCommand -->

`show bgp rpki | summary` answers the counters without the cache server rows,
through a pipe alias the plugin declares. It resolves over `ze cli -c "..."` and
over ssh, and NOT inside `ze cli` with no command argument
(`docs/guide/rpki.md`).

## Daemon Control

| Command | Description |
|---------|-------------|
| `request shutdown` | Graceful shutdown |
| `request peer <selector> refresh <family>` | Send a route refresh request |
| `help` | List all commands |
| `show command list` | All commands with descriptions |
| `show command help <name>` | Detailed help for a command |
| `show event list` | List available event types |
<!-- source: internal/component/bgp/plugins/route_refresh/yang/ze-refresh-cmd.yang -- module ze-refresh-cmd; internal/component/bgp/plugins/route_refresh/handler/refresh.go -- handleRefreshMarker -->
<!-- source: internal/plugins/meta/yang/ze-command-meta-cmd.yang -- module ze-command-meta-cmd -->

## Signals

| Command | Description |
|---------|-------------|
| `ze signal reload` | Reload configuration |
| `ze signal stop` | Graceful shutdown (no GR marker) |
| `ze signal restart` | Graceful restart (with GR marker) |
| `ze signal quit` | Goroutine dump and exit |
| `ze status` | Check if daemon is running |
<!-- source: internal/plugins/signal/main.go -- Run, RunStatus -->

## Interactive Features

In `ze cli` interactive mode:

- **Tab completion** for commands, peer names, address families, and log levels
- **Pipe operators** with per-command availability. `match <text>` keeps rows
  containing the text, while a match after a line format reads rendered lines.
  `ze help command --json` publishes the exact operator contract for each
  command. Each row also carries the command's one-line summary under
  `description` and its long explanation under `long-help`. Neither is derived
  from the other, and no row is cut at a sentence or a newline.
  <!-- source: internal/component/command/pipe.go -- applyMatch, applyMatchLines -->
  <!-- source: cmd/ze/help_command.go -- operatorsFor, collectCommands -->
- **History** persisted across sessions
- **Ctrl-C** cancels current command, **Ctrl-D** exits
