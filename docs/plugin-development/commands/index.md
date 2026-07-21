# Adding Commands

Plugins can expose commands for runtime interaction via the ze API.

## Declaring Commands

Commands are declared in the `Registration` struct passed to `Run`. The engine learns about them during Stage 1 (declare-registration).
<!-- source: pkg/plugin/rpc/types.go -- CommandDecl, DeclareRegistrationInput -->

```go
err := p.Run(ctx, sdk.Registration{
    Commands: []sdk.CommandDecl{
        {Name: "my-plugin status", Description: "Show current status"},
        {Name: "my-plugin check", Description: "Trigger immediate check", Args: []string{"target"}},
    },
})
```
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->
<!-- source: pkg/plugin/sdk/sdk_types.go -- Registration, CommandDecl -->

## Handling Commands

Register a handler with `OnExecuteCommand` before calling `Run`. The handler receives the command serial, command name, arguments, and peer selector.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->

```go
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (status, data string, err error) {
    switch command {
    case "my-plugin status":
        return "done", `{"status":"running","uptime":3600}`, nil
    case "my-plugin check":
        if len(args) < 1 {
            return "error", "usage: my-plugin check <target>", nil
        }
        result := performCheck(args[0])
        return "done", result, nil
    default:
        return "error", "unknown command: " + command, nil
    }
})
```

## Wire Format

Commands are delivered to the plugin as `execute-command` RPCs over the MuxConn connection. The wire format uses `#<id> <verb> [<json>]` framing.
<!-- source: pkg/plugin/rpc/message.go -- FormatRequest, FormatResult, FormatError -->
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandInput, ExecuteCommandOutput -->

### Request (engine to plugin)

```
#17 ze-plugin-callback:execute-command {"serial":"abc123","command":"my-plugin status","args":[],"peer":""}
```

### Success Response (plugin to engine)

```
#17 ok {"status":"done","data":"{\"status\":\"running\"}"}
```

### Error Response (plugin to engine)

```
#17 error {"message":"execute-command not supported"}
```

## Return Values

The `OnExecuteCommand` handler returns three values: `(status, data string, err error)`.
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- handleExecuteCommand -->

### Success with Data

```go
return "done", `{"count":42,"items":["a","b"]}`, nil
```

The SDK wraps this into an `ExecuteCommandOutput` and sends:
```
#17 ok {"status":"done","data":"{\"count\":42,\"items\":[\"a\",\"b\"]}"}
```
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandOutput -->

### Success without Data

```go
return "done", "", nil
```

Response:
```
#17 ok {"status":"done"}
```
<!-- source: pkg/plugin/rpc/message.go -- FormatResult, FormatOK -->

### Handler Error

If the handler returns a non-nil error, the SDK sends an error response:

```go
return "", "", fmt.Errorf("operation failed: database timeout")
```

Response:
```
#17 error {"message":"operation failed: database timeout"}
```
<!-- source: pkg/plugin/rpc/message.go -- FormatError, NewErrorPayload -->

## ExecuteCommandInput Fields

The engine sends these fields in the `execute-command` RPC:
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandInput -->

| Field | Type | Purpose |
|-------|------|---------|
| `serial` | string | Correlation ID for the request |
| `command` | string | Command name (e.g., `"my-plugin status"`) |
| `args` | []string | Additional arguments (may be empty) |
| `peer` | string | Peer selector (may be empty) |

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

Arguments arrive in the `args` parameter of the handler:

```go
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
    if command == "my-plugin get" {
        if len(args) < 1 {
            return "error", "usage: my-plugin get <key>", nil
        }
        key := args[0]
        value := getValue(key)
        return "done", value, nil
    }
    return "error", "unknown command", nil
})
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->

Invocation:
```
ze bgp run "my-plugin get config.timeout"
```

## CommandDecl Fields

Commands are declared with these fields:
<!-- source: pkg/plugin/rpc/types.go -- CommandDecl -->

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `Name` | string | Yes | Command name (e.g., `"my-plugin status"`) |
| `Description` | string | No | Human-readable description |
| `Args` | []string | No | Expected argument names (for help/completion) |
| `Completable` | bool | No | Whether the command supports tab completion |

## Complex Responses

Return structured JSON data for API consumers:

```go
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
    if command == "monitor metrics" {
        data, _ := json.Marshal(struct {
            Checks    int     `json:"checks"`
            Failures  int     `json:"failures"`
            LatencyMs float64 `json:"latency-ms"`
            LastCheck string  `json:"last-check"`
        }{
            Checks:    state.checks,
            Failures:  state.failures,
            LatencyMs: state.latency,
            LastCheck: state.lastCheck.Format(time.RFC3339),
        })
        return "done", string(data), nil
    }
    return "error", "unknown command", nil
})
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandOutput -->

Note: JSON keys use kebab-case per ze conventions (`"latency-ms"`, not `"latency_ms"`).

## Dispatching Commands to Other Plugins

Plugins can invoke commands on other plugins through the engine's command dispatcher:
<!-- source: pkg/plugin/sdk/sdk_engine.go -- DispatchCommand -->

```go
p.OnStarted(func(ctx context.Context) error {
    status, data, err := p.DispatchCommand(ctx, "rib show-in ipv4/unicast")
    if err != nil {
        return err
    }
    fmt.Printf("rib response: status=%s data=%s\n", status, data)
    return nil
})
```

The engine routes the command by longest-match registry lookup and returns the full `{status, data}` response from the target handler.
<!-- source: pkg/plugin/rpc/types.go -- DispatchCommandInput, DispatchCommandOutput -->

## Help Text

Provide usage information via a dedicated command:

```go
// In Registration:
sdk.CommandDecl{Name: "my-plugin help", Description: "Show available commands"},

// In handler:
if command == "my-plugin help" {
    data, _ := json.Marshal(map[string]any{
        "commands": []string{
            "my-plugin status - Show current status",
            "my-plugin check <target> - Trigger immediate check",
            "my-plugin metrics - Show performance metrics",
        },
    })
    return "done", string(data), nil
}
```
<!-- source: pkg/plugin/sdk/sdk_types.go -- CommandDecl -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->
