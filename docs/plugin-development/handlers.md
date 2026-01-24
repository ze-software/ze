# Verify and Apply Handlers

Handlers process configuration changes. Verify handlers validate changes; Apply handlers execute them.

## Handler Flow

```
User changes config
        │
        ▼
┌──────────────────┐
│  YANG Validation │  Type checking, ranges, patterns
└────────┬─────────┘
        │
        ▼
┌──────────────────┐
│  Verify Handler  │  Semantic validation (your code)
└────────┬─────────┘
        │
        ▼
┌──────────────────┐
│  Apply Handler   │  Execute the change (your code)
└──────────────────┘
```

## Verify Handlers

Verify handlers validate config **before** it's applied. Return an error to reject.

```go
p.OnVerify("my-prefix", func(ctx *plugin.VerifyContext) error {
    // ctx.Action: "create", "modify", or "delete"
    // ctx.Path: "my-prefix.item[key=value]"
    // ctx.Data: JSON config data

    if ctx.Action == "delete" {
        return nil  // Always allow delete
    }

    // Parse and validate
    var cfg Config
    if err := json.Unmarshal([]byte(ctx.Data), &cfg); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }

    // Semantic validation
    if cfg.Endpoint == "" {
        return fmt.Errorf("endpoint is required")
    }

    if !strings.HasPrefix(cfg.Endpoint, "https://") {
        return fmt.Errorf("endpoint must use HTTPS")
    }

    return nil  // Config valid
})
```

### What Verify Should Check

| Check | Example |
|-------|---------|
| Required fields present | `if cfg.Name == "" { error }` |
| Values make sense | `if cfg.Timeout < cfg.Interval { error }` |
| References exist | `if !exists(cfg.PeerGroup) { error }` |
| No conflicts | `if alreadyExists(cfg.Key) { error }` |

### What Verify Should NOT Do

- Start services
- Open connections
- Modify state
- Write files

Verify may be called multiple times or rolled back.

## Apply Handlers

Apply handlers execute validated changes. Called only after all verify passes.

```go
p.OnApply("my-prefix", func(ctx *plugin.ApplyContext) error {
    // ctx.Action: "create", "modify", or "delete"
    // ctx.Path: "my-prefix.item[key=value]"

    switch ctx.Action {
    case "create":
        // Start monitoring, open connections, etc.
        return startMonitoring(ctx.Path)

    case "modify":
        // Update existing configuration
        return updateMonitoring(ctx.Path)

    case "delete":
        // Stop monitoring, close connections
        return stopMonitoring(ctx.Path)
    }

    return nil
})
```

### Apply Handler Responsibilities

- Start/stop services
- Open/close connections
- Update runtime state
- Allocate/free resources

## Handler Routing

Handlers are matched by **longest prefix**:

```go
p.OnVerify("monitor", handleMonitor)
p.OnVerify("monitor.alert", handleAlert)

// Path "monitor.alert.email" → handleAlert
// Path "monitor.interval" → handleMonitor
```

## Context Types

### VerifyContext

```go
type VerifyContext struct {
    Action string  // "create", "modify", "delete"
    Path   string  // Full path with predicates
    Data   string  // JSON config data
}
```

### ApplyContext

```go
type ApplyContext struct {
    Action string  // "create", "modify", "delete"
    Path   string  // Full path with predicates
}
```

## Path Parsing

Paths include YANG predicates for list keys:

```
my-prefix.endpoint[name=api]
```

Extract the key:

```go
func extractKey(path string) string {
    start := strings.Index(path, "[")
    end := strings.Index(path, "]")
    if start < 0 || end < 0 {
        return ""
    }
    kv := path[start+1:end]  // "name=api"
    parts := strings.SplitN(kv, "=", 2)
    return parts[1]  // "api"
}
```

## Error Handling

Return clear, actionable errors:

```go
// Good: tells user what's wrong and how to fix
return fmt.Errorf("interval must be at least 10 seconds, got %d", cfg.Interval)

// Bad: cryptic
return fmt.Errorf("validation failed")
```

## State Management

Maintain plugin state for runtime behavior:

```go
var (
    monitors = make(map[string]*Monitor)
    mu       sync.Mutex
)

p.OnApply("monitor", func(ctx *ApplyContext) error {
    mu.Lock()
    defer mu.Unlock()

    switch ctx.Action {
    case "create":
        monitors[ctx.Path] = newMonitor(ctx)
    case "delete":
        if m := monitors[ctx.Path]; m != nil {
            m.Stop()
            delete(monitors, ctx.Path)
        }
    }
    return nil
})
```
