---
title: CLI Reference (Guide)
---
# CLI Reference

Ze provides an interactive CLI and single-command execution for runtime queries and control. `ze cli` reaches the daemon through its SSH server. `ze start --cli` attaches a console to the daemon it starts, in the same process.
<!-- source: internal/component/cli/client/main.go -- Run, RunAttached -->

## Usage

```bash
ze cli                              # Interactive CLI with tab completion
ze cli -c "show bgp peer list"              # Execute single command and exit
ze show bgp peer upstream1 detail           # Read-only query (safe for scripts)
ze cli -c "request peer upstream1 teardown 2" # One-shot command (full access)
ze start --cli                              # Start the daemon and attach a console
```

### Modes

| Command | Access | Use Case |
|---------|--------|----------|
| `ze cli` | Interactive, full | Exploring, monitoring, operating |
| `ze show <cmd>` | Read-only | Scripting, monitoring dashboards |
| `ze cli -c <cmd>` | Full | Automation, route injection |
| `ze start --cli` | Interactive, full | Running the daemon and operating it from one terminal |

With `ze.cli.transcript` enabled, `ze cli -c` records the command and its rendered
answer under `$XDG_DATA_HOME/ze/transcripts/`. This includes commands served
locally, which need no SSH credentials or running daemon. See
[transcript recording](configuration.md#environment-block) for file ownership and defaults.
<!-- source: internal/component/cli/client/main.go -- runBGP, runOne, openOneShotTranscript -->

### SSH idle timeout

`environment ssh idle-timeout` sets how many seconds an SSH connection can stay
silent. The default is 600 seconds. Receiving or sending data restarts the idle
period, so an active terminal or monitor can stay connected beyond that duration.
<!-- source: internal/component/ssh/ssh.go -- Server.Start -->
<!-- source: internal/component/ssh/yang/ze-ssh-conf.yang -- environment/ssh/idle-timeout -->

### The attached console

`ze start --cli` starts the daemon and attaches an interactive console to it.
The console dispatches each command in the daemon process, so it needs no SSH
server and it works in a build with SSH compiled out.

The console opens at the operational prompt. Type `configure` to reach
configuration mode, and `commit now` there reloads the running daemon, as it does
over SSH. Type `exit`, or press Ctrl-D, to detach the console and leave the
daemon running.

The console authenticates nobody, so it names the change author from the `USER`
environment variable, and `unknown` when that variable is empty. The name is
what `show | blame` reports for the changes the console commits.
<!-- source: internal/component/cli/client/main.go -- RunAttached, newAttachedModel -->
<!-- source: cmd/ze/hub/session_editor.go -- attachedConsoleEditor, attachedConsoleUser -->

## Peer Commands

| Command | Description |
|---------|-------------|
| `show bgp peer list` | List all peers (brief) |
| `show bgp peer <sel> detail` | Show peer details and statistics |
| `request peer <sel> teardown <code>` | Graceful session closure with NOTIFICATION |
| `delete bgp peer <name>` | Remove peer |
| `update bgp config` | Write the running peer set to the configuration file |
| `request peer <sel> pause` | Pause reading from peer (flow control) |
| `request peer <sel> resume` | Resume reading from peer |
| `show bgp peer <sel> capabilities` | Show negotiated capabilities |
| `show bgp` | BGP summary table |

**Peer selector:** `*` (all), exact IP, glob patterns (`192.168.*.*`), exclusion (`!addr`), or peer name.
<!-- source: internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang -- module ze-peer-cmd -->

## Route Commands

| Command | Description |
|---------|-------------|
| `send bgp <sel> update text <attrs> (nlri | mp <family>) <op> <prefix>` | Text-format UPDATE |
| `send bgp <sel> update hex <hex>` | Hex-format UPDATE |
| `show bgp rib received [peer <selector>] [family <family>]` | Show Adj-RIB-In |
| `show bgp rib advertised [peer <selector>] [family <family>]` | Show Adj-RIB-Out |
| `show bgp rib received covering <address>` | Show every received route whose prefix contains the address, most specific first within each peer (IPv4 and IPv6 prefix families) |
| `show bgp rib received exact <prefix>` | Show the received routes of exactly that prefix, every ADD-PATH path of it, from every selected peer (IPv4 and IPv6 prefix families) |
| `show bgp rib path <pattern>` | Show the routes whose AS path matches the pattern: AS numbers joined by commas, with `^` first to tie them to the start of the path and `$` last to tie them to its end |
| `clear bgp rib in <selector>` | Clear Adj-RIB-In (`*` for all peers) |
| `clear bgp rib out <selector> [family <family>]` | Resend Adj-RIB-Out (`*` for all peers) |
<!-- source: internal/component/bgp/plugins/cmd/rib/yang/ze-rib-cmd.yang -- show/bgp/rib, clear/bgp/rib; internal/component/bgp/plugins/cmd/rib/rib.go -- route filters -->
<!-- source: internal/component/bgp/plugins/rib/rib_pipeline_covering.go -- receivedWalk, parseCoveringAddress, parseExactPrefix, walkConflictRefusal, refuseCoveringScope, refuseExactScope -->

`covering` takes one address with no mask: `show bgp rib received covering
192.0.2.1` lists `192.0.2.0/24`, then `192.0.0.0/22`, then `0.0.0.0/0` when a
peer holds all three. It reads received routes only, so on `sent` or on the
default scope it is refused with the received form to type.

`exact` takes one prefix with its mask and answers that prefix alone: `show bgp
rib received exact 192.0.0.0/2` lists the routes of `192.0.0.0/2` and not
`192.0.0.0/24`. A bare address is refused with the `covering` form to type, and
a prefix with host bits set is refused with its masked form (`10.0.0.1/8` names
`10.0.0.0/8`). It reads received routes only, like `covering`, and one command
takes `covering` or `exact`, never both.

`prefix` stays a text filter on the prefix string, so it can also match longer
prefixes that share its leading text: `prefix 10.0` matches `10.0.0.0/24` and
not `10.10.0.0/16`, and `prefix 192.0.0.0/2` matches `192.0.0.0/24` too. Use
`exact` to ask for one prefix.

`path` takes AS numbers joined by commas, each in any spelling of an AS number
(`65546` or `1.10`). It lists the routes whose AS path holds those numbers in
that order with nothing between them, anywhere in the path: `path 65010,65001`.
Two anchors tie the run to an end of the path.

| Pattern | Routes listed |
|---------|---------------|
| `path 65001` | AS 65001 is anywhere in the path |
| `path 65010,65001` | 65010 is directly followed by 65001, anywhere in the path |
| `path ^65010` | The path starts with 65010: the routes that neighbour AS sent |
| `path 65001$` | The path ends with 65001: the routes AS 65001 originates. A route that only crosses AS 65001 is not listed |
| `path ^65010,65001$` | The path is exactly 65010 then 65001. A longer path that holds the pair is not listed |

The `$` is read only once, directly after the last AS number. A pattern that
holds it anywhere else (`65001$,65002`, `65001 $`, `$` alone) is refused, and
the refusal names the pattern.

The comparison is literal, and prepending stays as received. `path 65001$`
lists a route whose path ends `65001 65001 65001`, because its last AS is
65001. `path ^65010,65001$` does not list the path `65010 65001 65001`: write
each repeat, `path ^65010,65001,65001$`.

The filter reads the AS path as one list of AS numbers in the order received,
the members of an AS_SET and of a confederation segment included. A path that
ends with an AS_SET therefore ends, for `$`, with the last member of the set.
<!-- source: internal/component/bgp/plugins/rib/rib_pipeline.go -- parsePathPattern, pathPattern.matches -->

See [Route Injection guide](route-injection.md) for UPDATE syntax details.

## Cache Commands

| Command | Description |
|---------|-------------|
| `show cache` | List cached messages |
| `request cache retain <id>` | Prevent cache eviction |
| `request cache release <id>` | Release a cached message |
| `request cache expire <id>` | Remove a cached message immediately |
| `send bgp <peer> cached <id>` | Forward a cached message to a peer |
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

## Resolution Commands

| Command | Description |
|---------|-------------|
| `show resolve rir <asn>` | Which Regional Internet Registry holds an AS number, and its whois host |
| `update resolve rir` | Refresh the RIR delegation table from the five registry delegation files |
<!-- source: internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang -- show > resolve > rir, update > resolve > rir -->

The lookup reads a table the binary ships as embedded data, so it answers with
no network. `update resolve rir` stores a fresh table under
`meta/rir/delegation`, and the lookup prefers that copy while its generation
date is later than the shipped table's. The refresh is all or nothing: a
registry that does not answer leaves the previous table in place and the error
names the file it could not read. The same lookup runs on the host with no
daemon, as `ze resolve rir <asn>`.

An appliance behind a mirror names where each file is read from, under
`system/rir`, one `delegation-source` block per registry. A registry with no
block is read from the file it publishes, so mirroring one blocked registry
takes one block. A mirror is HTTPS, or plain HTTP when it runs on the router
itself.
<!-- source: internal/component/config/system/yang/ze-system-conf.yang -- system/rir/delegation-source -->
<!-- source: internal/component/resolve/irr/stored.go -- preferStoredDelegation -->
<!-- source: internal/component/resolve/cmd/rir.go -- handleRIRASN, handleRIRRefresh -->

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

- **Tab completion** for commands, peer names, address families, and log levels.
  A command whose argument only the running daemon knows asks the daemon at
  each Tab: `request plugin restart` offers every configured plugin, each
  marked as running or not running
  <!-- source: internal/component/cli/client/inject.go -- wireDaemonArgCompletion -->

- **Pipe operators** with per-command availability. `match <text>` keeps rows
  containing the text, while a match after a line format reads rendered lines.
  It reads the payload numbers as written, so a 64-bit counter matches on its
  exact digits rather than on a rounded value.
  `ze help command --json` publishes the exact operator contract for each
  command. Each row also carries the command's one-line summary under
  `short-help` and its long explanation under `description`. Neither is derived
  from the other, and no row is cut at a sentence or a newline.
  <!-- source: internal/component/command/pipe.go -- applyMatch, applyMatchLines -->
  <!-- source: internal/plugins/help/help_command.go -- operatorsFor, collectCommands -->
- **History** is immediately browsable and published across sessions without
  blocking input. Graceful session close drains queued history; abrupt process
  death can lose commands still waiting for publication.
  Storage failures remain visible in the session's close result; reconnect after
  checking the disk holding the configuration store to restore saved recall.
  <!-- source: internal/component/cli/history.go -- Save, Close -->
- **Ctrl-C** cancels current command, **Ctrl-D** exits

### Keys that reveal help

Every command declares two help texts: a one-line summary and a long
explanation. Tab and `?` reach both, so an operator reads what a command does
and stays at the prompt. Two message lines sit above the prompt, and the second
one carries the summary.

| Key | What it does |
|-----|--------------|
| Tab, with more to complete | Completes the command. Two candidates or more open the menu |
| Up or Down | Moves the selection in the menu. The second message line shows the selected command's summary |
| Tab, with nothing left to complete | Shows the command's long explanation in a box above the prompt |
| Tab, on a command that declares no explanation | The second message line reads `<command>: no explanation is declared` |
| `?`, with a candidate highlighted | Shows that candidate's long explanation, in the box Tab uses. The menu stays open under it |
| `?`, on a config key in configuration mode | Shows that key's whole YANG description in the box. A description is often a paragraph, and the message line holds one row |
| `?`, on a candidate that declares no text | The second message line reads `<command>: no explanation is declared` |
| `?`, with no candidate highlighted | Completes, as Tab does |
| `?`, inside a double-quoted value | Types a `?`. Quote any value that carries one, such as a URL with a query string: `description "https://example.net/a?b=1"` |
| Enter, with the menu open | Puts the selected command in the input |
| Enter, with the explanation on the screen | Runs the command as typed |
| Escape, with the explanation on the screen | Removes the explanation. The typed command stays, and so does a menu under it |
| Escape, with the menu open | Closes the menu |
| Escape, with nothing revealed | Clears the typed command. An empty input asks to quit |
| Any text key, or Backspace | Removes the menu or the explanation. The key still reaches the input |
<!-- source: internal/component/cli/model_keys.go -- handleTab, revealExplanation, revealCandidateExplanation, handleEnter, cursorInQuote -->
<!-- source: internal/component/cli/model_render.go -- warningText -->

Escape removes one thing for each press. The explanation goes first, and the
typed command stays until the next press.
<!-- source: internal/component/cli/model_keys.go -- handleKeyMsg, the Escape descent -->

A menu row is the command name alone. The summary has the second message line to
itself, so no width cuts it. Ze invents no explanation: a command that declares
none says so.
<!-- source: internal/component/cli/model_render.go -- renderDropdownBox, renderExplanationBox -->
