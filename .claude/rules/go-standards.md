# Go Coding Standards

## Required
- Go 1.21+ features (slog, generics)
- `golangci-lint` must pass
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Context for cancellation: `context.Context` as first param

## Logging

Use `log/slog` for all logging. Do NOT use the legacy `log` package.

```go
// GOOD: structured logging with slog
slog.Warn("unexpected format", "peer", peerAddr, "expected", "[]any", "got", fmt.Sprintf("%T", v))

// BAD: legacy log package
log.Printf("unexpected format for peer %s", peerAddr)  // FORBIDDEN
```

Prefer structured key-value pairs over formatted strings.

### Per-Subsystem Logging

Ze uses per-subsystem logging via `internal/slogutil`. Each subsystem has independent enable/disable control with hierarchical inheritance.

**Engine subsystems** (use `slogutil.Logger()`):
```go
// In internal/plugin/server.go
var logger = slogutil.Logger("server")  // Reads ze.log.server

func handleConnection() {
    logger.Debug("connection accepted", "peer", peerAddr)
}
```

**Plugin processes** (use `slogutil.PluginLogger()`):
```go
// In internal/plugin/gr/gr.go
var logger = slogutil.DiscardLogger()  // Disabled by default

func SetLogger(l *slog.Logger) {
    if l != nil { logger = l }
}

// In cmd/ze/bgp/plugin_gr.go
logLevel := fs.String("log-level", "disabled", "Log level")
gr.SetLogger(slogutil.PluginLogger("gr", *logLevel))  // CLI flag OR env var
```

### Environment Variables

Hierarchical logging via `ze.log.<path>=<level>`:

| Variable | Purpose |
|----------|---------|
| `ze.log` | Base level for ALL subsystems |
| `ze.log.bgp` | Level for all bgp.* subsystems |
| `ze.log.bgp.reactor` | Level for all bgp.reactor.* subsystems |
| `ze.log.server` | Plugin server logging |
| `ze.log.coordinator` | Startup coordinator |
| `ze.log.filter` | Filter/NLRI logging |
| `ze.log.config` | Config parsing/loading |
| `ze.log.bgp.reactor.peer` | Peer FSM/session events |
| `ze.log.bgp.reactor.session` | Session handling |
| `ze.log.bgp.routes` | Route operations |
| `ze.log.gr` | GR plugin |
| `ze.log.rib` | RIB plugin |
| `ze.log.relay` | Plugin stderr relay level |
| `ze.log.backend` | Output: `stderr` (default), `stdout`, `syslog` |
| `ze.log.destination` | Syslog address (when backend=syslog) |

### Hierarchical Priority

Most specific wins. Priority order (highest to lowest):
1. CLI flag `--log-level` (plugin processes only)
2. Specific env var (dot): `ze.log.bgp.fsm`
3. Specific env var (underscore): `ze_log_bgp_fsm`
4. Parent env var (dot): `ze.log.bgp`
5. Parent env var (underscore): `ze_log_bgp`
6. ... up to `ze.log` / `ze_log`
7. Default: **WARN** (shows warnings and errors)

To silence all logging: `ze.log=disabled`

Levels: `disabled`, `debug`, `info`, `warn`, `err` (case-insensitive)

Shell-compatible: `ze_log_server` also works (dot→underscore)

```bash
ze.log=info ze bgp server config.conf              # All subsystems at INFO
ze.log.bgp=debug ze bgp server config.conf         # All bgp.* at DEBUG
ze.log.bgp.reactor.peer=warn ze bgp server ...     # Peer at WARN only
```

### Config File Support

Log levels can also be set via the config file `environment { log { } }` block:

```
environment {
    log {
        level warn;            # Base level (ze.log=warn)
        bgp.routes debug;      # Subsystem (ze.log.bgp.routes=debug)
        config info;           # Subsystem (ze.log.config=info)
        backend stderr;        # Output: stderr | stdout | syslog
        destination localhost; # Syslog address (when backend=syslog)
        relay warn;            # Plugin stderr relay level
    }
}
```

**Priority:** OS env var > config file > default (WARN)

If an OS env var is set, config file settings for that key are ignored.

Engine loggers use `LazyLogger()` for deferred creation, allowing config file settings
to be applied before first use.

### Debug Logging is Permanent

**BLOCKING:** Do NOT remove `slog.Debug()` calls from the codebase.

Debug logging is essential for troubleshooting production issues. **If you feel the need to add debug output during development or investigation:**

1. **Use slogutil** - Add a logger for the subsystem if one doesn't exist
2. **Use logger.Debug** - Not fmt.Printf, fmt.Fprintf, or temporary print statements
3. **Keep it** - Debug logs are controlled by per-subsystem env vars, never removed
4. **Add context** - Include relevant identifiers (plugin name, peer address, stage, etc.)

```go
// GOOD: Permanent debug logging with subsystem logger
var logger = slogutil.Logger("test.runner")  // Create once at package level

logger.Debug("executing command", "binary", binPath, "args", args)

// BAD: Temporary debugging (FORBIDDEN)
fmt.Println("DEBUG:", pluginName)           // FORBIDDEN - temporary
fmt.Fprintf(os.Stderr, "DEBUG: %s\n", msg)  // FORBIDDEN - temporary
```

**Rationale:** Temporary debug statements get removed, losing valuable diagnostic capability. Using slogutil means the debug output is always available when needed (via env var) but silent by default.

## Error Handling

```go
// ALWAYS wrap errors with context
if err != nil {
    return fmt.Errorf("parsing header: %w", err)
}

// NEVER ignore errors
f, _ := os.Open(path)  // FORBIDDEN
```

## Fail-Early Rule
Configuration/parsing errors MUST propagate immediately.
Never silently ignore parse failures.

```go
// GOOD: fail early
if prefix == "" {
    return nil, fmt.Errorf("missing required prefix")
}

// BAD: silent ignore
if prefix == "" {
    prefix = "0.0.0.0/0"  // FORBIDDEN
}
```

## Forbidden
- `panic()` for error handling
- `f, _ := func()` (ignoring errors)
- Global mutable state
- `init()` functions (except registry patterns)

## Verification
```bash
make lint  # Must pass before commit
```
