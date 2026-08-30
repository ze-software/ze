# CLI Commands

### Every command starts with its verb

`show`, `clear`, `monitor`, `request`, `set`, and `delete` are the six verbs. The
words after the verb are the YANG path. A bare form with no verb is not in the
command tree, and the dispatcher answers `unknown command` for it. `daemon
reload` is now `request reload`, `daemon status` is `show status`, `daemon quit`
is `request halt`, `daemon shutdown` is `request shutdown`, and `bgp summary` is
`show bgp`.

`stop`, `restart`, and `reboot` are the exception, and they keep their bare
spelling. The SSH exec middleware intercepts those three lifecycle verbs before
the dispatcher, which registers no key for them.
<!-- source: internal/plugins/signal/main.go -- Commands table ExecCommand column -->
<!-- source: internal/component/ssh/ssh.go -- execMiddleware lifecycle interception -->

Every subcommand of `ze show`, `ze clear`, `ze monitor`, `ze request`, `ze set`,
and `ze delete` resolves against the daemon's own registrations. The verb-relative
tree is built from the same registration set the dispatcher is keyed on, so the
client and the daemon cannot disagree about which words are a command.
<!-- source: internal/component/cli/client/verb_tree.go -- BuildVerbCommandTree, AbsoluteVerbPath -->

A local in-process handler is refused for any argv that reaches a declared
command below it. `show interface` is registered at two words and takes an
interface name, so `brief`, `scan`, `type`, `errors`, `rate`, and the two
`name <name> ...` forms all go to the daemon rather than being read as interface
names.
<!-- source: internal/component/command/registry/registry.go -- LookupLocal prefix refusal -->

### Peer selectors on a destructive command

One resolver answers the peer selector for every peer-scoped command. It accepts
a name, an address, an ASN (`as65001`), or a prefix, so a selector that works on
one verb works on all of them.

A command that acts on ONE peer refuses a wildcard (`*` or empty), refuses an
exclusion selector (`!edge1`, `!as65001`, `!10.0.0.0/24`), and refuses a selector
that matches more than one peer. It never guesses which peer was meant. An
unresolvable selector is an error with the selector quoted, rather than a no-op
reported as success. A `show` command that narrows a list still accepts an
exclusion, because a complement is a good answer when the command filters rather
than acts.
<!-- source: internal/component/plugin/server/command.go -- ResolveSinglePeer, errExcludePeerSelector -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- handleTeardown, handleBgpPeerFlush -->

### Positional arguments

Every declared argument kind takes part in positional matching, typed kinds
included, and mandatory definitions are offered a token before optional ones. A
`uint16` port no longer skips its token and then fails with `required argument
missing`, and an optional string can no longer starve a required argument of its
value. One spare token becomes the peer selector when the command declares that
it requires one, no selector arrived out of band, and exactly one token is spare,
which is what makes `delete bgp peer 127.0.0.1` work.
<!-- source: internal/component/plugin/server/command.go -- positionalDef, matchCommandTokens -->

### A command declares what its answer holds

A command declares whether its answer holds rows or one document, and which of
its fields hold an IP address. The CLI refuses an operator that declaration
cannot support, by name, before the command runs:

```
show bgp rib status | count
count cannot apply here: this command answers one document, and count acts on rows
```

`ze help command "<path>" --json` lists the operators a declared command
supports, and it reads the same declaration, so the published list and the
runtime cannot disagree. `| resolve` and `| origin` are listed only where the
command declares a field that holds an address.

Every `show bgp` command declares one. Go compiled into the daemon declares
sixteen paths. A plugin process declares the other eleven in its startup
message: six under `show bgp rpki`, two under `show bgp rs`, two under
`show bgp adj-rib-in`, and `show bgp healthcheck`. An undeclared command still
refuses what it cannot support, from the answer it has in hand, after it runs.
<!-- source: internal/component/command/pipe.go -- validateDeclaredShape -->
<!-- source: internal/component/command/answer_shape.go -- RegisterShape, RegisterAddressFields, RegisterPluginShapes -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- registerShapes -->
<!-- source: internal/component/plugin/server/startup.go -- registerPluginShapes -->

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
| `ze config archive <name>` | Archive config to a named destination ([guide](../guide/config-archive.md)) |
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

Each `ze signal` subcommand sends one SSH exec command to the daemon. `reload`
sends `request reload`, `status` sends `show status`, and `quit` sends `request
halt`. `stop`, `restart`, and `reboot` keep their bare spelling because the SSH
exec middleware intercepts them before the dispatcher.
<!-- source: internal/plugins/signal/main.go -- Commands registry, ExecCommand column -->

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

**Live peer dashboard:** `monitor bgp` in the interactive CLI opens a live dashboard showing router identity, a sortable colour-coded peer table with update rates, and a drill-down detail view. It refreshes every 2 seconds. Use j/k to move, s/S to sort, Enter for detail, and Esc to exit. The state column renders green for `established`, yellow for the transitional states, and red for `stopped`, `idle`, and `idle-hold`. `idle-hold` is the state a prefix limit leaves a peer in when the family that overflowed asked for no reconnect, so it needs an operator and is coloured like the other down states.
<!-- source: internal/component/cli/model_dashboard.go -- isDashboardCommand -->
<!-- source: internal/component/cli/model_dashboard_render.go -- stateStyled -->

<!-- terminal-demo: cli-dashboard -->

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
| `ze show plugins` | List the plugins compiled into this binary |

<!-- source: internal/plugins/completion/main.go -- completion subcommand -->
<!-- source: internal/component/plugin/cli/main.go -- plugin subcommand -->
<!-- source: internal/plugins/exabgp/main.go -- exabgp subcommand -->
