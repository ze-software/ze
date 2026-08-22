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
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (status string, data any, err error) {
    switch command {
    case "my-plugin status":
        return "done", map[string]any{
            "status": "running",
            "uptime": 3600,
        }, nil
    case "my-plugin check":
        if len(args) < 1 {
            return "error", map[string]string{
                "error": "usage: my-plugin check <target>",
            }, nil
        }
        result := performCheck(args[0])
        return "done", result, nil
    default:
        return "error", map[string]string{
            "error": "unknown command: " + command,
        }, nil
    }
})
```

## Wire Format

Commands are delivered to the plugin as `execute-command` RPCs over the MuxConn connection. The wire format uses `#<len>:<id> <verb> [<json>]` framing.
<!-- source: pkg/plugin/rpc/message.go -- AppendRequest, AppendResult, AppendError -->
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandInput, ExecuteCommandOutput -->

### Request (engine to plugin)

```
#2:17 ze-plugin-callback:execute-command {"serial":"abc123","command":"my-plugin status","args":[],"peer":""}
```

### Success Response (plugin to engine)

The answer is a head, its records and a terminator, on every connection. The
frame is the same whatever the payload is, so a handler that built one value
takes the same three lines as a handler that walked a table. The field after the
id is a three-byte kind token saying which of the three a line is: `top`, `row`
and `end`, with `bad` for a rejected row.

```
#2:17 top status=done type=json
#2:17 row item={"status":"running","uptime":3600}
#2:17 end count=1
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- executeCommandAnswer -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteDocumentAnswer -->

The engine reads that sequence back into one `ExecuteCommandOutput`: the head's
`status=` is its `status`, and the record is its `data`.
<!-- source: internal/component/plugin/ipc/rpc.go -- ExecuteCommandValue -->
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandOutput -->

### Error Response (plugin to engine)

```
#2:17 error {"message":"execute-command not supported"}
```

## Return Values

The `OnExecuteCommand` handler returns three values: `(status string, data any, err error)`.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand, executeCommandOutput -->

### Success with Data

```go
return "done", map[string]any{
    "count": 42,
    "items": []string{"a", "b"},
}, nil
```

The SDK marshals this value once and sends it as the one record of the answer:
```
#2:17 top status=done type=json
#2:17 row item={"count":42,"items":["a","b"]}
#2:17 end count=1
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- executeCommandOutput, executeCommandAnswer -->

### Success without Data

```go
return "done", nil, nil
```

Response. A command that reported nothing writes no record, and the terminator
says so. Nothing is not the same answer as an empty collection:
```
#2:17 top status=done type=json
#2:17 end count=0
```
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteDocumentAnswer, writeDocumentLines -->

### Success with Rows

A command that walks a large collection returns an `sdk.Records` rather than a
built value. The SDK writes one line for each row, so neither the plugin nor the
engine holds the whole answer:

```go
return "done", sdk.Records{Key: "sessions", Rows: sessionRows()}, nil
```

```
#2:17 top status=done type=ndjson key=sessions
#2:17 row item={"id":1,"peer":"10.0.0.1"}
#2:17 row item={"id":2,"peer":"10.0.0.2"}
#2:17 end count=2
```

`Key` names the envelope the rows belong under. A handler MUST NOT name it
`errors`, because that name holds the rows the walk rejected. `Rows` is walked
once, before the handler's call returns. A handler MUST NOT store the sequence,
and MUST keep whatever it reads alive until then. A row that no wire message can
carry is reported as a rejected row, and the walk continues.

A walk of 256 rows or fewer collapses to one `type=json` document, which is the
JSON the command answered with before it produced rows at all. The encoder
decides that from the walk, and the handler states nothing about the wire.
<!-- source: pkg/plugin/records.go -- Row, Record, Records, Records.WriteAnswer -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteRecordAnswer, boundedRecord -->
<!-- source: pkg/plugin/rpc/collapse.go -- CollapseRecords, AnswerErrorsKey -->

### Handler Error

If the handler returns a non-nil error, the SDK sends an error response:

```go
return "", nil, fmt.Errorf("operation failed: database timeout")

```
Response:
```
#2:17 error {"message":"operation failed: database timeout"}
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
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
    if command == "my-plugin get" {
        if len(args) < 1 {
            return "error", map[string]string{
                "error": "usage: my-plugin get <key>",
            }, nil
        }
        key := args[0]
        value := getValue(key)
        return "done", map[string]any{"key": key, "value": value}, nil
    }
    return "error", map[string]string{"error": "unknown command"}, nil
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

## Naming a Pipe Alias for Your Command

A pipe alias is the word an operator types after the pipe character, standing
for an operator chain they would otherwise type in full. Declare one in the
`Pipes` list of the same `Registration`:
<!-- source: pkg/plugin/sdk/sdk_types.go -- PipeDecl, Registration -->

```go
err := p.Run(ctx, sdk.Registration{
    Commands: []sdk.CommandDecl{
        {Name: "my-plugin status", Description: "Show current status"},
    },
    Pipes: []sdk.PipeDecl{{
        Command:     "my-plugin status",
        Name:        "totals",
        Description: "The counters, without the per-session rows",
        Expansion:   "display sessions-total sessions-established",
    }},
})
```

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `Command` | string | Yes | Command path the alias sits on. MUST be one of your own declared commands |
| `Name` | string | Yes | The word typed after the pipe character (kebab-case) |
| `Description` | string | No | The line completion and `command help` show beside the name |
| `Expansion` | string | Yes | The operator chain the name stands for |

Three rules decide whether your command CAN have an alias.
<!-- source: internal/component/command/alias.go -- RegisterPluginAliases, checkAlias -->

- **The pipe layer selects. It does not compute.** `display` keeps named keys and
  drops the rest, and no operator renames a key, adds two numbers, or counts
  matching rows. Your handler MUST emit the aggregate fields beside the detail
  rows, as siblings at one level. `show bgp rpki` is the worked example.
- **The name MUST be free.** It cannot be a built-in operator name, a pipe filter
  name on an overlapping command path, or an alias name the exact same command
  path already carries. One refusal fails your whole Stage 1 registration.
- **The alias takes no argument, and names no other alias.** Its expansion parses
  to built-in operators alone.

The engine stops the alias reaching a command below the one it sits on. Declare
`my-plugin status detail` in the same message and it inherits nothing.

`ze cli` with no command argument expands the chain in the client process, where
your alias is not registered, so it answers
`pipe error: unknown pipe operator: totals` there. It resolves over
`ze cli -c "<command>"` and in the interactive session a plain ssh client
reaches.
<!-- source: internal/component/cli/model_mode.go -- executeOperationalCommand -->

## Complex Responses

Return structured JSON data for API consumers:

```go
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
    if command == "monitor metrics" {
        data := struct {
            Checks    int     `json:"checks"`
            Failures  int     `json:"failures"`
            LatencyMs float64 `json:"latency-ms"`
            LastCheck string  `json:"last-check"`
        }{
            Checks:    state.checks,
            Failures:  state.failures,
            LatencyMs: state.latency,
            LastCheck: state.lastCheck.Format(time.RFC3339),
        }
        return "done", data, nil
    }
    return "error", map[string]string{"error": "unknown command"}, nil
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
    return "done", map[string]any{
        "commands": []string{
            "my-plugin status - Show current status",
            "my-plugin check <target> - Trigger immediate check",
            "my-plugin metrics - Show performance metrics",
        },
    }, nil
}
```
<!-- source: pkg/plugin/sdk/sdk_types.go -- CommandDecl -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->
