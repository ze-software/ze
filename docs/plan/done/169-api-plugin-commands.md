# Spec: API Plugin Command Registration

## Status: DONE

## Prerequisites

- **spec-api-command-serial.md**: Unified serial numbers for all commands (must be implemented first)

## Problem

External processes cannot extend ZeBGP's command set. All commands are hardcoded in Go. This limits extensibility for:
- Custom operational commands
- Integration with external systems
- Domain-specific tooling

## Goals

1. External processes can register custom commands
2. CLI can discover all commands (builtin + plugin)
3. Commands route to registering process with request/response semantics
4. Clean lifecycle management (process death → unregister)

## Non-Goals

- Hot-reload of builtin commands
- Command override (plugins cannot shadow builtins)
- Multi-process command sharing (one command = one process)

---

## Current Architecture

```
┌─────────┐     Unix Socket      ┌─────────┐
│   CLI   │◄───────────────────►│ Server  │
└─────────┘                      └────┬────┘
                                      │
                                      ▼
                                ┌──────────┐
                                │Dispatcher│ ← Builtin handlers only
                                └──────────┘
                                      │
                                      ▼
                                ┌─────────┐
                                │ Reactor │
                                └─────────┘

┌─────────┐     stdin/stdout     ┌─────────┐
│ Process │◄────────────────────►│ Server  │
└─────────┘                      └─────────┘
    │
    └── stdout: "update text ..." (commands TO ZeBGP)
    └── stdin:  JSON events (notifications FROM ZeBGP)
```

**Current process protocol (stdout → ZeBGP):**
- Text commands: `update text nhop set ... nlri ... add ...`, `update text nlri ... del ...`
- Process is command SOURCE, not handler

**Current process protocol (ZeBGP → stdin):**
- JSON events: `{"type": "update", "peer": {...}, ...}`
- JSON state: `{"type": "state", "peer": {...}, "state": "up"}`

---

## Proposed Architecture

```
┌─────────┐     Unix Socket      ┌─────────┐
│   CLI   │◄───────────────────►│ Server  │
└─────────┘                      └────┬────┘
                                      │
                                      ▼
                                ┌──────────┐
                                │Dispatcher│
                                └────┬─────┘
                                     │
                    ┌────────────────┼────────────────┐
                    │                │                │
                    ▼                ▼                ▼
              ┌─────────┐     ┌──────────┐    ┌──────────────┐
              │ Builtin │     │ Reactor  │    │CommandRegistry│
              │ Handler │     │          │    └───────┬──────┘
              └─────────┘     └──────────┘            │
                                                      ▼
                                               ┌─────────────┐
                                               │PendingRequests│
                                               └──────┬──────┘
                                                      │
                                                      ▼
                                               ┌─────────┐
                                               │ Process │
                                               └─────────┘
```

---

## Protocol Design

### Direction Summary

| Direction | Format | Examples |
|-----------|--------|----------|
| Process → ZeBGP (stdout) | Text | `register command ...`, `response ...` |
| ZeBGP → Process (stdin) | JSON | `{"type": "request", ...}` |

This matches existing pattern: processes send text commands, receive JSON events.

### Process → ZeBGP (Text Commands)

All process commands use the `#N` serial prefix per **spec-api-command-serial.md**:

```
#<serial> <command>
```

#### 1. Registration

```
#N register command "<name>" description "<help>"
#N register command "<name>" description "<help>" args "<usage>"
#N register command "<name>" description "<help>" args "<usage>" completable
#N register command "<name>" description "<help>" timeout <duration>
```

**Examples:**
```
#1 register command "myapp status" description "Show myapp status"
#2 register command "myapp reload" description "Reload myapp config" timeout 60s
#3 register command "myapp check" description "Check component" args "<component>" completable
#4 register command "myapp sync" description "Full sync" timeout 120s
```

**Timeout:**
- Per-command timeout until first response byte
- Format: `<number>s` (seconds) or `<number>ms` (milliseconds)
- Default: 30s if not specified
- Completion requests: fixed 500ms (not configurable)

**Rules:**
- Process MAY send multiple `register` commands (additive)
- Command names MUST be quoted (may contain spaces)
- Command names MUST be lowercase
- Command names MUST NOT contain quote characters
- Command names MUST NOT conflict with builtins (error response)
- Command names MUST NOT conflict with other processes (first wins)

#### 2. Unregistration

```
#N unregister command "<name>"
```

**Example:**
```
#4 unregister command "myapp status"
```

**Rules:**
- Process can only unregister its own commands
- Unregistering non-existent command is no-op

#### 3. Command Response

When ZeBGP routes a CLI command to a process (via `{"serial": "abc", "type": "request", ...}` on stdin),
the process MUST reply with `@serial` prefix echoing ZeBGP's alpha serial:

```
@<serial> done [json-data]
@<serial> error "<message>"
```

**Examples:**
```
@a done {"component": "web", "healthy": true}
@a done
@b error "component not found: web"
```

#### 3a. Streaming Responses

For commands that return large data (e.g., RIB dump), use continuation marker `+`:

```
@<serial>+ <json-data>     # Partial response, more coming
@<serial> done [json-data] # Final response
@<serial> error "<message>" # Error terminates stream
```

**Examples:**
```
@a+ {"routes": [{"prefix": "10.0.0.0/24", ...}]}
@a+ {"routes": [{"prefix": "10.0.1.0/24", ...}]}
@a done {"total": 2}
```

**Rules:**
- `@serial+` indicates partial data, more chunks follow
- `@serial` (without `+`) is always final (done or error)
- Timeout applies between chunks (same as command timeout, resets on each chunk)
- Error terminates stream immediately

**Flow:**
```
CLI                    ZeBGP                   Process
 │                       │                        │
 │ "myapp status web"    │                        │
 │──────────────────────>│                        │
 │                       │ {"serial":"a",         │
 │                       │  "type":"request",     │
 │                       │  "command":"myapp status", │
 │                       │  "args":["web"]}       │
 │                       │───────────────────────>│
 │                       │                        │ (process handles)
 │                       │ @a done {...}          │
 │                       │<───────────────────────│
 │ {"status":"done",...} │                        │
 │<──────────────────────│                        │
```

**Serial types** (per spec-api-command-serial.md):
- Numeric (`#1`, `#2`, `#123`): Process-initiated commands
- Alpha (`#a`, `#b`, `#bcd`): ZeBGP-initiated requests (0→a, 1→b, ..., 9→j)

**Rules:**
- `serial` MUST match the ZeBGP alpha serial
- Unknown `serial` → logged and ignored (request may have timed out)
- `done` may have optional JSON data (passed to CLI as `data` field)
- `error` message MUST be quoted

### ZeBGP → Process (JSON Messages)

#### 4. Registration Result

Response to `#N register command ...` (echoes numeric serial):

```json
{"serial": "1", "status": "done"}
```

**Or failure:**
```json
{"serial": "1", "status": "error", "error": "conflicts with builtin: daemon status"}
```

#### 5. Command Request (ZeBGP-initiated)

When CLI sends a plugin command, ZeBGP routes to process using alpha serial:

```json
{"serial": "a", "type": "request", "command": "myapp status", "args": ["web"], "peer": "*"}
```

**Fields:**
- `serial`: Alpha serial (a, b, bcd, ...), process echoes in `@serial` response
- `type`: `"request"` for command execution
- `command`: Matched command name
- `args`: Remaining arguments after command match
- `peer`: Peer selector from `neighbor X` prefix (or `*`)

#### 6. Completion Request (ZeBGP-initiated)

```json
{"serial": "b", "type": "complete", "command": "myapp status", "args": [], "partial": "w"}
```

**Fields:**
- `args`: Completed arguments before the partial (context for multi-arg commands)
- `partial`: Current incomplete token being typed

**Example:** `myapp copy file1 f<TAB>`
```json
{"serial": "c", "type": "complete", "command": "myapp copy", "args": ["file1"], "partial": "f"}
```

---

## Command Matching

Commands are matched using **longest-prefix** matching, case-insensitive:

1. All commands (builtin + plugin) sorted by name length descending
2. Input compared against each command prefix
3. First match wins (longest due to sort order)
4. Remaining input becomes args

**Example:**
```
Registered: "peer", "peer status", "peer list"
Input: "peer status web"
Match: "peer status" (longest prefix)
Args:  ["web"]
```

**Conflict rules:**
- Plugins CANNOT shadow builtins (registration rejected)
- Full command name must be unique (first registration wins)
- Shared prefixes allowed: Process A registers `myapp status`, Process B registers `myapp config` ✅
- Duplicate leaf rejected: Two processes register `myapp status` ❌

## Tokenization

Single tokenizer for all parsing (registration and runtime):

- Whitespace separates tokens
- Quoted strings preserve spaces: `"hello world"` → single token `hello world`
- Backslash escaping: `\"` for literal quote, `\\` for literal backslash
- Quotes stripped from result

```
Input:  myapp check "hello world"
Args:   ["hello world"]

Input:  myapp set "value with \"quotes\""
Args:   ["value with \"quotes\""]
```

## Peer Selector

The `peer` field in requests uses the selector syntax from `internal/plugin/selector.go`:

| Pattern | Meaning |
|---------|---------|
| `*` | All peers |
| `<ip>` | Specific peer (e.g., `192.0.2.1`) |
| `!<ip>` | All peers except this IP |

Invalid: `!*` (cannot exclude all), empty selector.

---

## Component Design

### Completion Type

```go
// internal/plugin/types.go

// Completion represents a single completion suggestion.
// Used for both command and argument completion.
type Completion struct {
    Value  string `json:"value"`            // The completion text
    Help   string `json:"help,omitempty"`   // Optional description
    Source string `json:"source,omitempty"` // "builtin" or process name (verbose mode)
}
```

### CommandRegistry

```go
// internal/plugin/registry.go

type CommandRegistry struct {
    mu       sync.RWMutex
    commands map[string]*RegisteredCommand  // command name → registration
}

type RegisteredCommand struct {
    Name         string
    Description  string
    Args         string         // Usage hint (e.g., "<component>")
    Completable  bool           // Process handles arg completion
    Timeout      time.Duration  // Per-command timeout (default: 30s)
    Process      *Process       // Owning process
    RegisteredAt time.Time
}

func (r *CommandRegistry) Register(proc *Process, cmds []CommandDef) []RegisterResult
func (r *CommandRegistry) Unregister(proc *Process, names []string)
func (r *CommandRegistry) UnregisterAll(proc *Process)  // Called on process death
func (r *CommandRegistry) Lookup(name string) *RegisteredCommand
func (r *CommandRegistry) All() []*RegisteredCommand
func (r *CommandRegistry) Complete(partial string) []Completion  // For command completion
```

### PendingRequests

```go
// internal/plugin/pending.go

const (
    DefaultCommandTimeout    = 30 * time.Second
    CompletionTimeout        = 500 * time.Millisecond
    MaxPendingPerProcess     = 100  // Prevent memory exhaustion from stuck process
)

type PendingRequests struct {
    mu       sync.RWMutex
    next     uint64                        // Next serial number (encoded as alpha)
    requests map[string]*PendingRequest    // alpha serial → pending
    byProcess map[*Process]int             // Count per process for limit enforcement
}

type PendingRequest struct {
    Serial    string       // Alpha serial (a, b, bcd, ...)
    Command   string
    Process   *Process
    Client    *Client      // Socket client waiting for response
    CreatedAt time.Time
    Timer     *time.Timer  // Timeout cancellation
}

func (p *PendingRequests) Add(req *PendingRequest) string  // Returns assigned alpha serial
func (p *PendingRequests) Complete(serial string, resp *Response) bool
func (p *PendingRequests) Timeout(serial string)
func (p *PendingRequests) CancelAll(proc *Process)  // Called on process death
```

### Dispatcher Changes

```go
// internal/plugin/command.go

type Dispatcher struct {
    commands   map[string]*Command      // Builtin commands
    sortedKeys []string
    registry   *CommandRegistry         // NEW: Plugin commands
    pending    *PendingRequests         // NEW: In-flight requests
}

func (d *Dispatcher) Dispatch(ctx *CommandContext, input string) (*Response, error) {
    // 1. Try builtin commands (existing logic)
    // 2. Try registry lookup
    // 3. If found in registry, create pending request and route to process
    // 4. Return immediately (async) or block until response (sync)
}
```

### Process Changes

```go
// internal/plugin/process.go

type Process struct {
    // ... existing fields ...

    registeredCommands []string  // NEW: Track for cleanup
}

// handleOutput processes stdout lines (all text commands)
func (p *Process) handleOutput(line string) {
    // Check for @serial response (to ZeBGP-initiated request)
    if serial, rest, ok := parseResponseSerial(line); ok {
        // @serial done [...] | @serial error "..."
        p.handleResponse(serial, rest)
        return
    }

    // Check for #N serial prefix (process-initiated)
    serial, cmd := parseSerial(line)

    // Parse command
    tokens := tokenize(cmd)
    if len(tokens) == 0 {
        return
    }

    switch tokens[0] {
    case "register":
        // #N register command "<name>" description "<help>" [args "<usage>"]
        p.handleRegister(serial, tokens[1:])
    case "unregister":
        // #N unregister command "<name>"
        p.handleUnregister(serial, tokens[1:])
    default:
        // Existing command handling (announce, withdraw, etc.)
        p.handleCommand(serial, cmd)
    }
}
```

---

## CLI Commands

### Query: `system command list`

Returns all commands (builtin + registered):

```
system command list
```

Response:
```json
{
  "status": "done",
  "data": {
    "commands": [
      {"value": "daemon status", "help": "Show daemon status"},
      {"value": "peer list", "help": "List all peers"},
      {"value": "myapp status", "help": "Show myapp status"}
    ]
  }
}
```

### Query: `system command list verbose`

Returns all commands with source info:

```
system command list verbose
```

Response:
```json
{
  "status": "done",
  "data": {
    "commands": [
      {"value": "daemon status", "help": "Show daemon status", "source": "builtin"},
      {"value": "myapp status", "help": "Show myapp status", "source": "myapp-controller"}
    ]
  }
}
```

### Query: `system command help <name>`

Returns detailed help for a specific command:

```
system command help "myapp status"
```

Response:
```json
{
  "status": "done",
  "data": {
    "command": "myapp status",
    "description": "Show myapp component status",
    "args": "<component>",
    "source": "myapp-controller",
    "timeout": "5s"
  }
}
```

For unknown command:
```json
{
  "status": "error",
  "error": "unknown command: myapp foo"
}
```

### Query: `system command complete <partial>`

CLI sends partial input, ZeBGP returns matching completions:

```
system command complete "myapp st"
```

Response:
```json
{
  "status": "done",
  "data": {
    "completions": [
      {"value": "myapp status", "help": "Show myapp status"},
      {"value": "myapp start", "help": "Start myapp"}
    ]
  }
}
```

Empty input returns all commands:
```
system command complete ""
```

### Query: `system command complete <command> args <partial>`

For argument completion, CLI can ask process for suggestions:

```
system command complete "myapp status" args "w"
```

ZeBGP routes to owning process:
```json
{"serial": "b", "type": "complete", "command": "myapp status", "args": [], "partial": "w"}
```

Process responds (text):
```
@b done {"completions":[{"value":"web"},{"value":"worker"},{"value":"websocket"}]}
```

ZeBGP returns to CLI:
```json
{
  "status": "done",
  "data": {
    "completions": [
      {"value": "web"},
      {"value": "worker"},
      {"value": "websocket"}
    ]
  }
}
```

Process MAY include `help` field for richer completions:
```
@a done {"completions":[{"value":"web","help":"Web server"},{"value":"worker","help":"Background worker"}]}
```

### Registration with Completion Support

Process can declare it supports argument completion:

```
register command "myapp status" description "Show status" args "<component>" completable
```

The `completable` flag tells ZeBGP to route `complete ... args` requests to this process.

Without `completable`, ZeBGP returns empty completions for arguments.

---

## Lifecycle Management

### Process Startup

1. ZeBGP spawns process
2. Process sends registration commands on stdout:
   ```
   #1 register command "myapp status" description "Show status" args "<component>" completable
   #2 register command "myapp reload" description "Reload config"
   ```
3. ZeBGP validates and registers commands
4. ZeBGP sends results on stdin (serial as string):
   ```json
   {"serial": "1", "status": "done"}
   {"serial": "2", "status": "done"}
   ```
5. Process is ready to handle commands

### Command Execution

1. CLI sends `myapp status web`
2. Dispatcher checks builtins (no match)
3. Dispatcher checks registry (match: `myapp status` → Process)
4. Create PendingRequest, assign alpha serial (e.g., `a`)
5. Send to process stdin:
   ```json
   {"serial": "a", "type": "request", "command": "myapp status", "args": ["web"], "peer": "*"}
   ```
6. Process handles, sends on stdout:
   ```
   @a done {"component": "web", "status": "healthy"}
   ```
7. PendingRequests.Complete() routes response to CLI client
8. CLI receives response

### Timeout

1. PendingRequest timer fires (default: 30s)
2. Remove from pending map
3. Send error response to CLI: `{"status": "error", "error": "command timed out"}`
4. Log warning (process may be stuck)

### Process Death

1. Process exits (crash or normal)
2. Server detects EOF on stdout
3. Call `registry.UnregisterAll(process)`
4. Call `pending.CancelAll(process)` → error responses to waiting CLIs
5. Respawn if configured

### Process Respawn

1. New process instance starts
2. Must re-register commands (registry is cleared on death)
3. Old pending requests already canceled

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Register builtin conflict | Reject with reason in `register-result` |
| Register process conflict | First registration wins, later rejected |
| Unknown request ID in response | Log warning, ignore |
| Response after timeout | Log warning, ignore |
| Malformed JSON from process | Log error, ignore line |
| Process death mid-request | Error response to CLI |
| Max pending limit reached | Reject request with error, log warning |
| Completion timeout (500ms) | Return empty completions |

---

## Security Considerations

1. **Command shadowing**: Plugins CANNOT override builtins (enforced)
2. **Process isolation**: Each process can only unregister its own commands
3. **No code execution**: Commands are routed, not eval'd
4. **Timeout protection**: Stuck processes don't block CLI indefinitely

---

## Configuration

No config changes required. Process decides what to register at runtime.

Optional future config:
```
process myapp {
    run "/usr/bin/myapp";
    allow-commands yes;     # Default: yes
    command-timeout 60s;    # Default: 30s
}
```

---

## Implementation Phases

### Phase 1: Core Infrastructure ✅
- [x] `CommandRegistry` type (`internal/plugin/registry.go`)
- [x] `PendingRequests` type with serial counter (`internal/plugin/pending.go`)
- [x] Wire into `Dispatcher` (`internal/plugin/command.go`)
- [x] Tests for registry operations (`internal/plugin/registry_test.go`)

### Phase 2: Process Protocol ✅
- [x] Text command parsing (`register`, `unregister`, `@serial response`)
- [x] `register` / `unregister` handling (`internal/plugin/server.go`)
- [x] Response JSON sending to process
- [x] `request` JSON sending to process
- [x] `@serial done/error` response parsing (`internal/plugin/plugin.go`)
- [x] Tests for protocol (`internal/plugin/plugin_test.go`)

### Phase 3: Lifecycle ✅
- [x] Timeout handling (`PendingRequests` timer)
- [x] Process death cleanup (`cleanupProcess()`)
- [x] Pending request cancellation (`CancelAll()`)
- [x] Streaming response collection

### Phase 4: CLI Query & Completion ✅
- [x] `system command list` handler
- [x] `system command list verbose` handler
- [x] `system command help <name>` handler
- [x] `system command complete <partial>` handler
- [x] `system command complete <cmd> args <partial>` handler (routed to process)
- [x] `completable` flag in registration
- [x] Update `system help` to use dispatcher

---

## Example: Plugin Process

```python
#!/usr/bin/env python3
import json
import sys

COMPONENTS = ["web", "worker", "websocket", "database"]

# Track our numeric serial for process-initiated commands
next_serial = 1
pending = {}  # serial -> command (to track our requests)

def send_command(cmd):
    global next_serial
    serial = str(next_serial)
    pending[serial] = cmd
    print(f"#{serial} {cmd}")
    next_serial += 1
    sys.stdout.flush()

# Register commands at startup (with #N serial prefix)
send_command('register command "hello" description "Say hello" args "[name]"')
send_command('register command "myapp status" description "Show component status" args "<component>" completable')
send_command('register command "myapp dump" description "Dump all data" timeout 120s')

# Handle incoming messages (JSON from ZeBGP)
for line in sys.stdin:
    msg = json.loads(line)
    serial = msg.get("serial", "")

    # Response to our command (numeric serial we sent)
    if serial in pending:
        del pending[serial]
        if msg.get("status") == "error":
            print(f"Command failed: {msg.get('error')}", file=sys.stderr)
        continue

    # ZeBGP-initiated request (alpha serial)
    if msg.get("type") == "request":
        if msg["command"] == "hello":
            name = msg["args"][0] if msg["args"] else "stranger"
            data = json.dumps({"greeting": f"Hello, {name}!"})
            print(f"@{serial} done {data}")

        elif msg["command"] == "myapp status":
            comp = msg["args"][0] if msg["args"] else "all"
            data = json.dumps({"component": comp, "status": "healthy"})
            print(f"@{serial} done {data}")

        elif msg["command"] == "myapp dump":
            # Streaming response example
            for i, comp in enumerate(COMPONENTS):
                data = json.dumps({"component": comp, "index": i})
                print(f"@{serial}+ {data}")  # + indicates more coming
                sys.stdout.flush()
            print(f"@{serial} done")  # Final response (no +)

        else:
            print(f'@{serial} error "unknown command"')

    elif msg.get("type") == "complete":
        # Handle argument completion
        partial = msg.get("partial", "")
        matches = [{"value": c} for c in COMPONENTS if c.startswith(partial)]
        data = json.dumps({"completions": matches})
        print(f"@{serial} done {data}")

    sys.stdout.flush()
```

---

## Design Decisions

1. **Sync dispatch** - CLI blocks until response. Async can be added later if needed.

2. **Shared prefix tree, unique leaf** - Commands can share prefixes (`myapp status`, `myapp config`) but full command names must be unique across all processes.

3. **One process per command** - Simple ownership model. Load balancing/failover can be added later if needed.
