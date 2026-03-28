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
        relay warn             # plugin stderr relay level
    }
}
```

<!-- source: internal/component/hub/schema/ze-hub-conf.yang -- environment log config block -->

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
ze run bgp log set bgp.fsm debug
ze run bgp log set bgp.reactor info
ze cli --run "bgp log levels"         # show current levels
```
<!-- source: internal/component/cmd/log/ -- log show/set RPCs -->

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
| `syslog` | `backend syslog` | Remote syslog. Set address with `destination`. |

Syslog example:

```
environment {
    log {
        level info
        backend syslog
        destination 10.0.0.100:514
    }
}
```

## Plugin Stderr Relay

External plugins (forked processes) write to stderr. Ze captures this output and relays it through the logging system tagged with subsystem `relay`.

```bash
export ZE_LOG_RELAY=debug      # see all plugin stderr
export ZE_LOG_RELAY=disabled   # silence plugin output
```

Default relay level: `warn`.
<!-- source: internal/component/plugin/process/delivery.go -- plugin delivery/relay -->

## Common Subsystems

| Subsystem | What it covers |
|-----------|---------------|
| `bgp` | All BGP activity |
| `bgp.reactor` | Event loop, peer management |
| `bgp.fsm` | FSM state transitions |
| `bgp.routes` | Route processing |
| `bgp.wire` | Wire encoding/decoding |
| `plugin` | Plugin lifecycle |
| `hub` | Plugin hub, IPC |
| `gr` | Graceful restart |
| `relay` | Plugin stderr relay |
<!-- source: internal/core/slogutil/slogutil.go -- subsystem logger creation -->

## Inspecting Current Configuration

```bash
ze env list -v                 # show all env vars with current values
ze env get ze.log              # show specific var details
ze cli --run "bgp log levels"  # show runtime log levels
```
