# Adding Commands

Plugins can expose commands for runtime interaction via the ZeBGP API.

## Registering Commands

```go
p.OnCommand("my-plugin status", func(ctx *plugin.CommandContext) (any, error) {
    return map[string]any{
        "status": "running",
        "uptime": getUptime(),
    }, nil
})
```

## Command Context

```go
type CommandContext struct {
    Command string   // "my-plugin status"
    Args    []string // Additional arguments
}
```

## Return Values

### Success with Data

```go
return map[string]any{
    "count": 42,
    "items": []string{"a", "b"},
}, nil

// Response: @serial ok {"count":42,"items":["a","b"]}
```

### Success without Data

```go
return nil, nil

// Response: @serial ok
```

### Error

```go
return nil, fmt.Errorf("operation failed: %v", err)

// Response: @serial error operation failed: ...
```

## Naming Conventions

| Pattern | Example | Purpose |
|---------|---------|---------|
| `<plugin> status` | `acme-monitor status` | Get current state |
| `<plugin> stats` | `acme-monitor stats` | Get metrics |
| `<plugin> <action>` | `acme-monitor check` | Perform action |
| `<plugin> list` | `acme-monitor list` | List items |

**Rules:**
- Start with plugin name to avoid conflicts
- Use kebab-case for multi-word commands
- Keep commands short and memorable

## Command Arguments

Arguments are passed as string array:

```go
p.OnCommand("my-plugin get", func(ctx *plugin.CommandContext) (any, error) {
    if len(ctx.Args) < 1 {
        return nil, fmt.Errorf("usage: my-plugin get <key>")
    }
    key := ctx.Args[0]
    return getValue(key), nil
})
```

Invocation:
```
ze bgp run "my-plugin get config.timeout"
```

## Complex Responses

Return structured data for API consumers:

```go
p.OnCommand("monitor metrics", func(ctx *plugin.CommandContext) (any, error) {
    return struct {
        Checks    int     `json:"checks"`
        Failures  int     `json:"failures"`
        Latency   float64 `json:"latency_ms"`
        LastCheck string  `json:"last_check"`
    }{
        Checks:    state.checks,
        Failures:  state.failures,
        Latency:   state.latency,
        LastCheck: state.lastCheck.Format(time.RFC3339),
    }, nil
})
```

## Async Operations

For long-running operations, return immediately and process async:

```go
p.OnCommand("monitor trigger", func(ctx *plugin.CommandContext) (any, error) {
    // Start async operation
    go performCheck()

    return map[string]string{
        "status": "check triggered",
    }, nil
})
```

## Help Text

Provide usage information:

```go
p.OnCommand("my-plugin help", func(ctx *plugin.CommandContext) (any, error) {
    return map[string]any{
        "commands": []string{
            "my-plugin status - Show current status",
            "my-plugin check - Trigger immediate check",
            "my-plugin metrics - Show performance metrics",
        },
    }, nil
})
```
