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
