# Logging

Ze uses hierarchical, structured logging with runtime-configurable levels per subsystem.
<!-- source: internal/core/slogutil/slogutil.go -- Logger, PluginLogger, level definitions -->

## Log Levels

| Level | Description |
|-------|-------------|
| `disabled` | No output |
| `debug` | Detailed diagnostics (FSM transitions, wire events, plugin IPC) |
| `info` | Informational messages (peer up/down, config reload) |
| `warn` | Warnings and errors (default) |
| `err` | Errors only |

## Configuration

### Config File

```
environment {
    log {
        level warn              # base level for all subsystems
        backend stderr          # output destination
        bgp.reactor debug      # subsystem-specific override
        bgp.routes info        # another override
        plugin.relay warn      # plugin stderr relay level
    }
}
```

<!-- source: internal/component/hub/yang/ze-hub-conf.yang -- environment log config block -->

### Environment Variables

Environment variables take precedence over the config file:

```bash
# Set base level
export ZE_LOG=debug

# Set per-subsystem level
export ZE_LOG_BGP_FSM=debug
export ZE_LOG_PLUGIN=info

# All notation forms are equivalent:
ze.log.bgp.fsm    ZE_LOG_BGP_FSM    ze_log_bgp_fsm
```
<!-- source: internal/core/slogutil/slogutil.go -- ze.log, ze.log.<subsystem> env var registration -->

### CLI Flag

```bash
ze -d example.conf             # shorthand for ze.log=debug + ze.log.relay=debug
```

### Runtime Change

Change log levels on a running daemon without restart:

```bash
ze cli -c "request log level bgp.fsm debug"
ze cli -c "request log level bgp.reactor info"
ze cli -c "request log level bgp.reactor disabled"   # silence one subsystem
ze cli -c "show log levels"                          # show current levels
```

`disabled` silences the subsystem until a later `request log level` sets a
level again. `show log levels` reports it as `disabled`, and a subsystem that
started disabled through `ze.log.<subsystem>=disabled` is listed the same way,
so the runtime command can enable it. A level set at runtime is lost on
restart.
<!-- source: internal/plugins/log/cmd/handlers.go -- log show/set RPCs; internal/core/slogutil/slogutil.go -- SetLevel, disabledLevel -->

## Priority Order

Most specific wins, environment beats config:

1. Environment variable `ZE_LOG_BGP_FSM` (most specific)
2. Environment variable `ZE_LOG_BGP` (parent)
3. Environment variable `ZE_LOG` (base)
4. Config file `bgp.fsm debug` (most specific)
5. Config file `level warn` (base)
6. Default: `warn`

## Backends

| Backend | Config | Description |
|---------|--------|-------------|
| `stderr` | `backend stderr` | Standard error (default). Color auto-detected on TTY. |
| `stdout` | `backend stdout` | Standard output. Color auto-detected on TTY. |
| `syslog` | `backend syslog` | Syslog: the local daemon, a Unix socket or a UDP collector, set with `destination`. |

Syslog example:

```
environment {
    log {
        level info
        backend syslog
        destination 192.0.2.100:514
    }
}
```

### Syslog facility and severity

Each syslog record carries the severity of its level: `debug` is sent as
debug, `info` as info, `warn` as warning and `err` as err. A collector can
select records by facility and severity.
<!-- source: internal/core/slogutil/syslog.go -- severityHandler -->

Every subsystem goes out on the `daemon` facility unless the `syslog`
container names another one. The `facility` leaf changes the facility for all
subsystems. Any other name in the container is a subsystem, and its value is
the facility that subsystem and its children go out on. Ze reads the longest
matching name first, as it does for levels, so `bgp` covers `bgp.server` until
`bgp.server` is named too. The name `facility` is reserved for the leaf, so a
subsystem called `facility` cannot be mapped here. Facility names are `kern`, `user`, `mail`,
`daemon`, `auth`, `syslog`, `lpr`, `news`, `uucp`, `cron`, `authpriv`, `ftp`
and `local0` to `local7`.
<!-- source: internal/core/slogutil/syslog.go -- syslogFacilities, facilityFor -->

The level of a facility is the level of the subsystems it carries. To send
everything at `info` on `daemon`, and the BGP subsystems at `debug` on
`local7`:

```
environment {
    log {
        level info
        bgp debug
        backend syslog
        destination 192.0.2.100:514
        syslog {
            bgp local7
        }
    }
}
```

To send everything on one facility, set only the leaf:

```
environment {
    log {
        level info
        backend syslog
        syslog {
            facility local7
        }
    }
}
```

The environment variable `ze.log.syslog.facility` overrides the leaf, and
`ze.log.syslog.<subsystem>` overrides a subsystem name (`ZE_LOG_SYSLOG_BGP=local7`).
A word that names no facility is refused with a warning on stderr, and the
subsystem keeps the facility of the next shorter name.
<!-- source: internal/core/slogutil/slogutil.go -- syslogConfigEntries -->

## Plugin Stderr Relay

External plugins (forked processes) write to stderr. Ze captures this output and relays it through the logging system tagged with subsystem `plugin.relay`.

```bash
export ZE_LOG_PLUGIN_RELAY=debug      # see all plugin stderr
export ZE_LOG_PLUGIN_RELAY=disabled   # silence plugin output
```

Default relay level: `warn`.
<!-- source: internal/component/plugin/process/process.go -- stderrLogger -->

The relay keeps the LAST line a plugin wrote. Ze attaches a parent-owned pipe to
the child and drains it after reaping the process, with a bounded grace period,
so a verdict written immediately before the plugin exits still reaches the log. A
grandchild holding the descriptor open cannot hold the drain open forever.
<!-- source: internal/component/plugin/process/process.go -- monitorCmd, parent-owned stderr pipe -->

A quoted value in a relayed line arrives decoded. The parser reads the escapes
the structured logger wrote, so a backslash no longer doubles at every relay hop.
A value that ends in a backslash no longer runs the scan past its own closing
quote and into the attributes after it.
<!-- source: internal/core/slogutil/parse.go -- unquoteValue, findClosingQuote -->

A fatal diagnostic from a dying process goes to the real stderr rather than to
the crash-capture pipe. Crash capture redirects `os.Stderr` into a pipe whose
reader goroutine dies with the process, so a fatal message written there existed,
was written, and reached no one. The last thing a dying process says now reaches
the terminal.
<!-- source: internal/core/env/env.go -- fatalOut, SetFatalOutput -->
<!-- source: internal/core/crashlog/crashlog.go -- SetFatalOutput called with the pre-redirect descriptor -->

## Naming Convention

Subsystem names follow `<domain>.<component>[.<concern>]`:

| Domain | Covers |
|--------|--------|
| `bgp` | BGP protocol (config, filter, reactor, routes, server) |
| `chaos` | Chaos fault injection |
| `cli` | CLI and editor |
| `hub` | Hub process management |
| `plugin` | Plugin infrastructure |
| `web` | Web UI |

Setting a parent level applies to all children. `ze.log.bgp=debug` enables debug for every `bgp.*` subsystem. `ze.log.plugin=info` enables info for every `plugin.*` subsystem.
<!-- source: internal/core/slogutil/slogutil.go -- getLogEnv hierarchical lookup -->

## Subsystems

Run `ze env` to see the full list with descriptions. Below is the complete inventory:

| Subsystem | Description |
|-----------|-------------|
| `bgp.config` | Configuration parsing and loading |
| `bgp.filter` | Route filtering (AS loop, originator-ID) |
| `bgp.filter.community` | Community-based route filtering plugin |
| `bgp.gr` | Graceful restart marker handling |
| `bgp.reactor` | Reactor event loop and peer management |
| `bgp.reactor.cache` | UPDATE cache gap-based eviction |
| `bgp.reactor.forward` | Per-peer forward worker pool |
| `bgp.reactor.peer` | Peer lifecycle and FSM transitions |
| `bgp.reactor.session` | Session wire protocol handling |
| `bgp.routes` | Route processing and announcements |
| `bgp.server` | TCP server and connection handling |
| `bgp.watchdog` | Watchdog timer plugin |
| `chaos` | Chaos fault injection orchestration |
| `chaos.peer` | Chaos testing simulated peers |
| `cli.editor.draft` | Config editor draft persistence |
| `hub.managed` | Managed mode client connection |
| `hub.reload` | Configuration reload handling |
| `plugin` | Plugin process lifecycle and event delivery |
| `plugin.coordinator` | Plugin startup stage coordination |
| `plugin.manager` | Multi-plugin coordination and respawn |
| `plugin.relay` | Plugin stderr relay to engine log |
| `plugin.server` | Plugin RPC server and stage protocol |
| `web.auth` | Web UI authentication |
| `web.server` | Web UI HTTP server |
<!-- source: internal/core/slogutil/slogutil.go -- subsystemDescriptions map -->

## Inspecting Current Configuration

```bash
ze env list -v                 # show all env vars with current values
ze env get ze.log              # show specific var details
ze cli -c "show log levels"    # show runtime log levels
```
