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

### Debug Logging is Permanent

**BLOCKING:** Do NOT remove `slog.Debug()` calls from the codebase.

Debug logging is essential for troubleshooting production issues. When you add debug logging for investigation:

1. **Keep it** - Debug logs are controlled by SLOG_LEVEL environment variable
2. **Use slog.Debug** - Not fmt.Printf or temporary print statements
3. **Add context** - Include relevant identifiers (plugin name, peer address, stage, etc.)

```go
// GOOD: Permanent debug logging
slog.Debug("server: stageTransition START", "plugin", pluginName, "complete", completeStage)

// BAD: Temporary debugging (FORBIDDEN)
fmt.Println("DEBUG:", pluginName)  // Will be removed - use slog.Debug instead
```

### SLOG_LEVEL Configuration

ZeBGP reads `SLOG_LEVEL` environment variable:
- `DEBUG` - All logs including debug
- `INFO` - Default level
- `WARN` - Warnings and errors only
- `ERROR` - Errors only

```bash
SLOG_LEVEL=DEBUG zebgp server config.conf  # Enable debug output
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
