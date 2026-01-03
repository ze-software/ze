# Spec: API Plugin Command Registration

## Status: DRAFT

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
    └── stdout: "announce route ..." (commands TO ZeBGP)
    └── stdin:  JSON events (notifications FROM ZeBGP)
```

**Current process protocol (stdout → ZeBGP):**
- Text commands: `announce route ...`, `withdraw route ...`
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

**Existing commands** (unchanged):
```
announce route 10.0.0.0/24 next-hop 1.2.3.4
withdraw route 10.0.0.0/24
session ack enable
# ... etc
```

**New commands** for plugin system:

#### 1. Registration

```
register command "<name>" description "<help>"
register command "<name>" description "<help>" args "<usage>"
```

**Examples:**
```
register command "myapp status" description "Show myapp status"
register command "myapp status" description "Show myapp status" args "[component]"
register command "myapp reload" description "Reload myapp config"
```

**Rules:**
- Process MAY send multiple `register` commands (additive)
- Command names MUST be quoted (may contain spaces)
- Command names MUST NOT conflict with builtins (error response)
- Command names MUST NOT conflict with other processes (first wins)

#### 2. Unregistration

```
unregister command "<name>"
```

**Example:**
```
unregister command "myapp status"
```

**Rules:**
- Process can only unregister its own commands
- Unregistering non-existent command is no-op

#### 3. Command Response

When ZeBGP routes a CLI command to a process (via `{"type": "request", "serial": N, ...}` on stdin),
the process MUST reply with a `response` command:

```
response <serial> done [json-data]
response <serial> error "<message>"
```

**Examples:**
```
response 42 done {"component": "web", "healthy": true}
response 42 done
response 42 error "component not found: web"
```

**Flow:**
```
CLI                    ZeBGP                   Process
 │                       │                        │
 │ "myapp status web"    │                        │
 │──────────────────────>│                        │
 │                       │ {"type":"request",     │
 │                       │  "serial":42,          │
 │                       │  "command":"myapp status", │
 │                       │  "args":["web"]}       │
 │                       │───────────────────────>│
 │                       │                        │ (process handles)
 │                       │ response 42 done {...} │
 │                       │<───────────────────────│
 │ {"status":"done",...} │                        │
 │<──────────────────────│                        │
```

**Rules:**
- `serial` MUST match the request serial (incrementing uint64)
- Unknown `serial` → logged and ignored (request may have timed out)
- `done` may have optional JSON data (passed to CLI as `data` field)
- `error` message MUST be quoted

### ZeBGP → Process (JSON Messages)

#### 4. Registration Result

```json
{
  "type": "register-result",
  "command": "myapp status",
  "success": true
}
```

**Or failure:**
```json
{
  "type": "register-result",
  "command": "myapp status",
  "success": false,
  "reason": "conflicts with builtin"
}
```

#### 5. Command Request

```json
{
  "type": "request",
  "serial": 42,
  "command": "myapp status",
  "args": ["web"],
  "peer": "*"
}
```

**Fields:**
- `serial`: Incrementing request number (uint64) for response matching
- `command`: Matched command name
- `args`: Remaining arguments after command match
- `peer`: Peer selector from `neighbor X` prefix (or `*`)

---

## Component Design

### Completion Type

```go
// pkg/api/types.go

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
// pkg/api/registry.go

type CommandRegistry struct {
    mu       sync.RWMutex
    commands map[string]*RegisteredCommand  // command name → registration
}

type RegisteredCommand struct {
    Name         string
    Description  string
    Args         string      // Usage hint (e.g., "<component>")
    Completable  bool        // Process handles arg completion
    Process      *Process    // Owning process
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
// pkg/api/pending.go

type PendingRequests struct {
    mu       sync.RWMutex
    next     uint64                        // Next serial number
    requests map[uint64]*PendingRequest    // serial → pending
    timeout  time.Duration
}

type PendingRequest struct {
    Serial    uint64
    Command   string
    Process   *Process
    Client    *Client      // Socket client waiting for response
    CreatedAt time.Time
    Timer     *time.Timer  // Timeout cancellation
}

func (p *PendingRequests) Add(req *PendingRequest) uint64  // Returns assigned serial
func (p *PendingRequests) Complete(serial uint64, resp *Response) bool
func (p *PendingRequests) Timeout(serial uint64)
func (p *PendingRequests) CancelAll(proc *Process)  // Called on process death
```

### Dispatcher Changes

```go
// pkg/api/command.go

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
// pkg/api/process.go

type Process struct {
    // ... existing fields ...

    registeredCommands []string  // NEW: Track for cleanup
}

// handleOutput processes stdout lines (all text commands)
func (p *Process) handleOutput(line string) {
    tokens := tokenize(line)
    if len(tokens) == 0 {
        return
    }

    switch tokens[0] {
    case "register":
        // register command "<name>" description "<help>" [args "<usage>"]
        p.handleRegister(tokens[1:])
    case "unregister":
        // unregister command "<name>"
        p.handleUnregister(tokens[1:])
    case "response":
        // response <id> done [json] | response <id> error "<msg>"
        p.handleResponse(tokens[1:])
    default:
        // Existing command handling (announce, withdraw, etc.)
        p.handleCommand(line)
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
{
  "type": "complete",
  "serial": 43,
  "command": "myapp status",
  "partial": "w"
}
```

Process responds (text):
```
response 43 done {"completions":[{"value":"web"},{"value":"worker"},{"value":"websocket"}]}
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
response 43 done {"completions":[{"value":"web","help":"Web server"},{"value":"worker","help":"Background worker"}]}
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
2. Process sends `{"type": "register", "commands": [...]}` on stdout
3. ZeBGP validates and registers commands
4. ZeBGP sends `{"type": "register-result", ...}` on stdin
5. Process is ready to handle commands

### Command Execution

1. CLI sends `myapp status web`
2. Dispatcher checks builtins (no match)
3. Dispatcher checks registry (match: `myapp status` → Process)
4. Create PendingRequest, assign next serial (e.g., 42)
5. Send `{"type": "request", "serial": 42, ...}` to process stdin
6. Process handles, sends `response 42 done {...}` on stdout
7. PendingRequests.Complete(42) routes response to CLI client
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
3. Old pending requests already cancelled

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

### Phase 1: Core Infrastructure
- [ ] `CommandRegistry` type
- [ ] `PendingRequests` type (with serial counter)
- [ ] Wire into `Dispatcher`
- [ ] Tests for registry operations

### Phase 2: Process Protocol
- [ ] Text command parsing in `handleOutput` (`register`, `unregister`, `response`)
- [ ] `register` / `unregister` handling
- [ ] `register-result` JSON sending
- [ ] `request` JSON sending
- [ ] `response` text parsing
- [ ] Tests for protocol

### Phase 3: Lifecycle
- [ ] Timeout handling
- [ ] Process death cleanup
- [ ] Pending request cancellation
- [ ] Integration tests

### Phase 4: CLI Query & Completion
- [ ] `system command list` handler
- [ ] `system command list verbose` handler
- [ ] `system command complete <partial>` handler
- [ ] `system command complete <cmd> args <partial>` handler (routed to process)
- [ ] `completable` flag in registration
- [ ] Update `system help` to use dispatcher

---

## Example: Plugin Process

```python
#!/usr/bin/env python3
import json
import sys

COMPONENTS = ["web", "worker", "websocket", "database"]

# Register commands at startup (text format)
# "completable" flag enables argument completion
print('register command "hello" description "Say hello" args "[name]"')
print('register command "myapp status" description "Show component status" args "<component>" completable')
sys.stdout.flush()

# Handle incoming messages (JSON from ZeBGP)
for line in sys.stdin:
    msg = json.loads(line)
    serial = msg["serial"]

    if msg["type"] == "request":
        # Handle command execution
        if msg["command"] == "hello":
            name = msg["args"][0] if msg["args"] else "stranger"
            data = json.dumps({"greeting": f"Hello, {name}!"})
            print(f"response {serial} done {data}")

        elif msg["command"] == "myapp status":
            comp = msg["args"][0] if msg["args"] else "all"
            data = json.dumps({"component": comp, "status": "healthy"})
            print(f"response {serial} done {data}")

        else:
            print(f'response {serial} error "unknown command"')

    elif msg["type"] == "complete":
        # Handle argument completion
        partial = msg.get("partial", "")
        matches = [{"value": c} for c in COMPONENTS if c.startswith(partial)]
        data = json.dumps({"completions": matches})
        print(f"response {serial} done {data}")

    sys.stdout.flush()
```

---

## Open Questions

1. **Sync vs async dispatch?**
   - Current: CLI blocks until response
   - Alternative: Return immediately, poll for result
   - Recommendation: Sync (simpler, matches current behavior)

2. **Command namespacing?**
   - Current: Flat namespace, conflicts rejected
   - Alternative: `process:command` namespacing
   - Recommendation: Flat (simpler, plugins choose unique names)

3. **Multiple processes per command?**
   - Current: One process per command
   - Alternative: Load balancing / failover
   - Recommendation: One (simpler, can add later)

4. **Capability negotiation?**
   - Should process declare protocol version?
   - Recommendation: Add `"version": 1` to register message for future compat
