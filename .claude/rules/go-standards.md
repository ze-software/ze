---
paths:
  - "**/*.go"
---

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

ZeBGP uses per-subsystem logging via `pkg/slogutil`. Each subsystem has independent enable/disable control.

**Engine subsystems** (use `slogutil.Logger()`):
```go
// In pkg/plugin/server.go
var logger = slogutil.Logger("server")  // Reads zebgp.log.server

func handleConnection() {
    logger.Debug("connection accepted", "peer", peerAddr)
}
```

**Plugin processes** (use `slogutil.LoggerWithLevel()`):
```go
// In pkg/plugin/gr/gr.go
var logger = slogutil.DiscardLogger()  // Disabled by default

func SetLogger(l *slog.Logger) {
    if l != nil { logger = l }
}

// In cmd/zebgp/plugin_gr.go
logLevel := fs.String("log-level", "disabled", "Log level")
gr.SetLogger(slogutil.LoggerWithLevel("gr", *logLevel))
```

### Environment Variables

Per-subsystem control via `zebgp.log.<subsystem>=<level>`:

| Variable | Purpose |
|----------|---------|
| `zebgp.log.server` | Plugin server logging |
| `zebgp.log.coordinator` | Startup coordinator |
| `zebgp.log.filter` | Filter/NLRI logging |
| `zebgp.log.plugin` | Relay plugin stderr (`enabled`/`disabled`) |
| `zebgp.log.backend` | Output: `stderr` (default), `stdout`, `syslog` |
| `zebgp.log.destination` | Syslog address (when backend=syslog) |

Levels: `disabled`, `debug`, `info`, `warn`, `err` (case-insensitive)

Shell-compatible: `zebgp_log_server` also works (dot→underscore)

```bash
zebgp.log.server=debug zebgp server config.conf  # Enable server debug
```

### Debug Logging is Permanent

**BLOCKING:** Do NOT remove `slog.Debug()` calls from the codebase.

Debug logging is essential for troubleshooting production issues. When you add debug logging for investigation:

1. **Keep it** - Debug logs are controlled by per-subsystem env vars
2. **Use logger.Debug** - Not fmt.Printf or temporary print statements
3. **Add context** - Include relevant identifiers (plugin name, peer address, stage, etc.)

```go
// GOOD: Permanent debug logging with subsystem logger
logger.Debug("stageTransition START", "plugin", pluginName, "complete", completeStage)

// BAD: Temporary debugging (FORBIDDEN)
fmt.Println("DEBUG:", pluginName)  // Will be removed - use logger.Debug instead
```

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
