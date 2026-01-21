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

ZeBGP uses per-subsystem logging via `internal/slogutil`. Each subsystem has independent enable/disable control.

**Engine subsystems** (use `slogutil.Logger()`):
```go
// In internal/plugin/server.go
var logger = slogutil.Logger("server")  // Reads ze.bgp.log.server

func handleConnection() {
    logger.Debug("connection accepted", "peer", peerAddr)
}
```

**Plugin processes** (use `slogutil.LoggerWithLevel()`):
```go
// In internal/plugin/gr/gr.go
var logger = slogutil.DiscardLogger()  // Disabled by default

func SetLogger(l *slog.Logger) {
    if l != nil { logger = l }
}

// In cmd/ze/bgp/plugin_gr.go
logLevel := fs.String("log-level", "disabled", "Log level")
gr.SetLogger(slogutil.LoggerWithLevel("gr", *logLevel))
```

### Environment Variables

Per-subsystem control via `ze.log.bgp.<subsystem>=<level>`:

| Variable | Purpose |
|----------|---------|
| `ze.log.bgp.server` | Plugin server logging |
| `ze.log.bgp.coordinator` | Startup coordinator |
| `ze.log.bgp.filter` | Filter/NLRI logging |
| `ze.log.bgp.plugin` | Relay plugin stderr (`enabled`/`disabled`) |
| `ze.log.bgp.backend` | Output: `stderr` (default), `stdout`, `syslog` |
| `ze.log.bgp.destination` | Syslog address (when backend=syslog) |

Levels: `disabled`, `debug`, `info`, `warn`, `err` (case-insensitive)

Shell-compatible: `ze_log_bgp_server` also works (dot→underscore)

```bash
ze.log.bgp.server=debug ze bgp server config.conf  # Enable server debug
```

### Debug Logging is Permanent

**BLOCKING:** Do NOT remove `slog.Debug()` calls from the codebase.

Debug logging is essential for troubleshooting production issues. **If you feel the need to add debug output during development or investigation:**

1. **Use slogutil** - Add a logger for the subsystem if one doesn't exist
2. **Use logger.Debug** - Not fmt.Printf, fmt.Fprintf, or temporary print statements
3. **Keep it** - Debug logs are controlled by per-subsystem env vars, never removed
4. **Add context** - Include relevant identifiers (plugin name, peer address, stage, etc.)

```go
// GOOD: Permanent debug logging with subsystem logger
var logger = slogutil.Logger("runner")  // Create once at package level

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
