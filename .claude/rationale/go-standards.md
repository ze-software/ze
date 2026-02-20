# Go Standards Rationale

Why: `.claude/rules/go-standards.md`

## Logging Subsystem Env Vars

| Variable | Purpose |
|----------|---------|
| `ze.log` | Base level for ALL |
| `ze.log.bgp` | All bgp.* |
| `ze.log.bgp.reactor` | All bgp.reactor.* |
| `ze.log.server` | Plugin server |
| `ze.log.coordinator` | Startup coordinator |
| `ze.log.filter` | Filter/NLRI |
| `ze.log.config` | Config parsing |
| `ze.log.bgp.reactor.peer` | Peer FSM/session |
| `ze.log.bgp.reactor.session` | Session handling |
| `ze.log.bgp.routes` | Route operations |
| `ze.log.gr` | GR plugin |
| `ze.log.rib` | RIB plugin |
| `ze.log.relay` | Plugin stderr relay |
| `ze.log.backend` | Output: stderr/stdout/syslog |
| `ze.log.destination` | Syslog address |

Shell-compatible: `ze_log_server` (dot→underscore).

## Config File Logging

```
environment { log { level warn; bgp.routes debug; config info; backend stderr; } }
```

Priority: OS env > config > default (WARN). LazyLogger() for deferred creation.

## Why Debug Logging Is Permanent

```go
// GOOD: Permanent, subsystem-scoped
var logger = slogutil.Logger("test.runner")
logger.Debug("executing command", "binary", binPath, "args", args)

// BAD: Temporary (FORBIDDEN)
fmt.Println("DEBUG:", pluginName)
```

Debug output is controlled by env vars, never removed. Losing diagnostic capability costs more than the lines of code.
