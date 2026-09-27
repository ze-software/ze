# Go Standards Rationale

Why: `ai/rules/go-standards.md`

## Logging Subsystem Env Vars

Names follow `<domain>.<component>` convention. Run `ze env` for the full list.

| Variable | Purpose |
|----------|---------|
| `ze.log` | Base level for ALL |
| `ze.log.bgp` | All bgp.* subsystems |
| `ze.log.bgp.config` | Config parsing |
| `ze.log.bgp.filter` | Route filtering |
| `ze.log.bgp.reactor` | All bgp.reactor.* |
| `ze.log.bgp.reactor.peer` | Peer FSM/session |
| `ze.log.bgp.reactor.session` | Session handling |
| `ze.log.bgp.routes` | Route operations |
| `ze.log.plugin` | All plugin.* subsystems |
| `ze.log.plugin.coordinator` | Startup coordinator |
| `ze.log.plugin.relay` | Plugin stderr relay |
| `ze.log.plugin.server` | Plugin RPC server |
| `ze.log.web` | All web.* subsystems |
| `ze.log.backend` | Output: stderr/stdout/syslog |
| `ze.log.destination` | Syslog address |

Shell-compatible: `ze_log_plugin_server` (dot to underscore).

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

## History moved from rule points (2026-09-26)

- `ai/rules/points/go-standards/directives/read-the-ze-style-guide-before-go-design-or-review.md`: (owner directive, 2026-08-18) <!-- doc-links: ignore (history: the point was renamed on 2026-09-27) -->
- 2026-09-27 (owner decision): the trigger changed from every session, before any code, to before a session's first Go edit. The point moved to `read-the-ze-style-guide-before-the-first-go-edit.md`, and `writeStyleGuideRead`, a check in the pre-write hook, enforces it: it refuses a Go write from a session or subagent with no Read or Bash print of the guide on record. Go written through a Bash heredoc bypasses the check.
