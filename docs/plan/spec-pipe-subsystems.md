# Spec: Pipe-Based Subsystem Infrastructure

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/process.go` - existing pipe communication
4. `internal/plugin/registration.go` - 5-stage protocol
5. `internal/plugin/command.go` - dispatcher and command routing

## Task

Build infrastructure for internal subsystems that run as **separate forked processes** communicating via **pipes**, using the same 5-stage protocol as external plugins.

### Why Forked Processes?

The previous approach (init() self-registration) was wrong because:
- Internal handlers are compiled into the main binary
- They cannot be run separately or replaced at runtime
- No isolation between subsystems

**User requirement:** "We want to have OTHER programs, forked, registering later on and communicating via the plugin API - for that we can not have internal registration, it must be done via a pipe."

### Goals

1. **Forked processes** - Subsystems run as separate processes (fork or separate binary)
2. **Pipe communication** - stdin/stdout pipes like external plugins
3. **Same protocol** - 5-stage startup (Declaration → Config → Capability → Registry → Ready)
4. **Reuse process.go** - Extend existing `Process` infrastructure
5. **Command routing** - Dispatcher routes commands to correct subsystem via pipes

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         ZeBGP Engine                            │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  ProcessManager + Coordinator                                ││
│  │  - Spawns subsystem processes (fork or exec)                ││
│  │  - Manages 5-stage protocol via pipes                       ││
│  │  - Routes commands to correct process                       ││
│  └─────────────────────────────────────────────────────────────┘│
│         │ pipe               │ pipe               │ pipe       │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐      │
│  │ze-subsystem │     │ze-subsystem │     │  External   │      │
│  │   cache     │     │   route     │     │   Plugin    │      │
│  │  (forked)   │     │  (forked)   │     │ (external)  │      │
│  └─────────────┘     └─────────────┘     └─────────────┘      │
└─────────────────────────────────────────────────────────────────┘
```

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - [current API design]
- [ ] `docs/architecture/api/ipc_protocol.md` - [5-stage protocol details]

### Source Files
- [ ] `internal/plugin/process.go` - [pipe communication, Process struct]
- [ ] `internal/plugin/registration.go` - [5-stage protocol parsing]
- [ ] `internal/plugin/command.go` - [Dispatcher, command routing]
- [ ] `internal/plugin/server.go` - [coordinator, startup]

**Key insights:**
- `Process` already handles pipe communication and 5-stage protocol
- `ProcessManager` manages multiple processes
- `Dispatcher` routes commands but currently only for builtin handlers
- Need to extend `Dispatcher` to route to processes via pipes

## Design

### 1. Subsystem Binaries

Each subsystem is a separate binary:

```
cmd/
├── ze/                      # Main ZeBGP binary
│   └── bgp/
│       └── main.go
└── ze-subsystem/            # Subsystem binary
    └── main.go              # Uses --mode=cache|route|... flag
```

Or separate binaries per subsystem:
```
cmd/
├── ze/bgp/main.go
├── ze-cache/main.go         # Cache subsystem
├── ze-route/main.go         # Route subsystem
└── ze-session/main.go       # Session subsystem
```

**Decision:** Single `ze-subsystem` binary with `--mode` flag is simpler for distribution.

### 2. Subsystem Process Spawning

Engine spawns subsystem processes like external plugins:

```go
// In server.go during startup
func (s *Server) startSubsystems(ctx context.Context) error {
    // Spawn cache subsystem
    cacheConfig := PluginConfig{
        Name: "cache",
        Run:  "ze-subsystem --mode=cache",  // Forked process
    }
    cacheProc := NewProcess(cacheConfig)
    if err := cacheProc.StartWithContext(ctx); err != nil {
        return err
    }

    // Process follows 5-stage protocol like external plugins
    // Coordinator handles the stages

    return nil
}
```

### 3. Subsystem Main Loop

Each subsystem binary implements the protocol:

```go
// cmd/ze-subsystem/main.go
func main() {
    mode := flag.String("mode", "", "Subsystem mode: cache|route|session")
    flag.Parse()

    switch *mode {
    case "cache":
        runCacheSubsystem()
    case "route":
        runRouteSubsystem()
    case "session":
        runSessionSubsystem()
    }
}

func runCacheSubsystem() {
    // Stage 1: Declaration
    fmt.Println("declare encoding text")
    fmt.Println("declare cmd bgp cache list")
    fmt.Println("declare cmd bgp cache retain")
    fmt.Println("declare cmd bgp cache release")
    fmt.Println("declare cmd bgp cache expire")
    fmt.Println("declare cmd bgp cache forward")
    fmt.Println("declare done")

    // Stage 2: Read config until "config done"
    scanner := bufio.NewScanner(os.Stdin)
    for scanner.Scan() {
        line := scanner.Text()
        if line == "config done" {
            break
        }
        // Parse config lines
    }

    // Stage 3: Capability (none for cache)
    fmt.Println("capability done")

    // Stage 4: Read registry until "registry done"
    for scanner.Scan() {
        line := scanner.Text()
        if line == "registry done" {
            break
        }
    }

    // Stage 5: Ready
    fmt.Println("ready")

    // Main loop: handle commands
    for scanner.Scan() {
        line := scanner.Text()
        // Parse #serial command
        // Execute and respond with @serial response
    }
}
```

### 4. Command Routing via Pipes

Dispatcher sends commands to subsystem via pipes:

```go
// Extend Dispatcher to track command → process mapping
type Dispatcher struct {
    commands     map[string]*Command      // Builtin handlers (sync)
    processes    map[string]*Process      // Command → process (async via pipe)
    sortedKeys   []string
    registry     *CommandRegistry
    pending      *PendingRequests
}

// RegisterProcess maps a command to a process (called during 5-stage)
func (d *Dispatcher) RegisterProcess(cmdName string, proc *Process) {
    d.processes[strings.ToLower(cmdName)] = proc
}

// Dispatch routes to builtin handler OR process
func (d *Dispatcher) Dispatch(ctx *CommandContext, input string) (*Response, error) {
    // ... existing builtin lookup ...

    // If no builtin, check process mapping
    proc := d.findProcess(input)
    if proc != nil {
        return d.dispatchToProcess(ctx, proc, input)
    }

    // If no process, try plugin registry
    return d.dispatchPlugin(ctx, input, peerSelector)
}

// dispatchToProcess sends command via pipe and waits for response
func (d *Dispatcher) dispatchToProcess(ctx *CommandContext, proc *Process, input string) (*Response, error) {
    // Use Process.SendRequest() which handles #serial command → @serial response
    resp, err := proc.SendRequest(ctx.Context, input)
    if err != nil {
        return &Response{Status: "error", Data: err.Error()}, err
    }

    // Parse response
    return parseResponse(resp)
}
```

### 5. Subsystem Dependencies

Subsystems need access to reactor APIs (cache, peers, events).

**Option A:** Pass via pipes (complex, requires serialization)
**Option B:** Subsystems access reactor via IPC (same as external plugins)
**Option C:** Subsystems are stateless handlers, reactor calls them

**Decision:** Option B - subsystems use the same IPC as external plugins. They send commands to engine (`bgp cache retain 123`) which routes to reactor.

This means:
- Subsystem receives command via stdin: `#ab bgp cache list`
- Subsystem processes locally OR calls engine via command
- Subsystem responds via stdout: `@ab ok {"ids":[1,2,3]}`

For cache subsystem that needs reactor access:
- Engine sends: `#ab bgp cache list`
- Cache subsystem queries... what?

**Problem:** Cache subsystem needs to access cache data, which lives in reactor.

**Solution:** Internal subsystems that need reactor access remain builtin handlers. Only stateless handlers become forked processes.

OR: Subsystems receive data via events, not commands.

**Revised architecture:**

```
Subsystem Types:
1. Stateless handlers → forked processes (parse/format commands)
2. Stateful handlers → builtin (need reactor access)

Example stateless: bgp help, bgp version, bgp config validate
Example stateful: bgp cache list, bgp route announce (needs reactor)
```

This doesn't solve the user's problem. Let me re-think...

**User's actual need:** Future external programs that register commands at runtime.

The init() self-registration achieves this for BUILTIN handlers.
For EXTERNAL programs (forked or separate), they already use the plugin protocol.

**What's missing:** The ability to have "internal" handlers that are forked processes rather than in-process.

**Real solution:**
1. Keep init() self-registration for handlers that need direct reactor access
2. Add ability to spawn "internal" subprocess that uses plugin protocol
3. Internal subprocesses are spawned by engine but work like external plugins

This is essentially what the user wants - forked processes using pipe communication.

### Revised Design: Forked Subsystem Handler

```go
// internal/plugin/subsystem.go

// SubsystemHandler is a forked process that handles a subset of commands.
// Unlike external plugins, subsystems are built into ZeBGP and spawned by engine.
type SubsystemHandler struct {
    proc     *Process
    commands []string  // Commands this subsystem handles
}

// NewSubsystemHandler creates a handler backed by a forked process.
func NewSubsystemHandler(name string, commands []string) *SubsystemHandler {
    return &SubsystemHandler{
        commands: commands,
    }
}

// Start spawns the subsystem process.
func (h *SubsystemHandler) Start(ctx context.Context) error {
    config := PluginConfig{
        Name: "subsystem-" + h.name,
        Run:  fmt.Sprintf("ze-subsystem --mode=%s", h.name),
    }
    h.proc = NewProcess(config)
    return h.proc.StartWithContext(ctx)
}

// Handle routes a command to the forked process.
func (h *SubsystemHandler) Handle(ctx context.Context, cmd string) (*Response, error) {
    resp, err := h.proc.SendRequest(ctx, cmd)
    if err != nil {
        return nil, err
    }
    return parseResponse(resp)
}
```

But this still has the problem: what does the forked process DO? If it needs reactor access, it can't have it (separate process).

**Final insight:** The forked process needs to communicate BACK to the engine for reactor operations.

```
Engine                    Subsystem (forked)
  │                            │
  │ ─────#ab bgp cache list───→│
  │                            │
  │                            │ (needs reactor.ListUpdates())
  │                            │
  │ ←───#1 internal cache list─│  (subsystem calls back to engine!)
  │                            │
  │ ────@1 ok {"ids":[...]}───→│  (engine responds)
  │                            │
  │ ←────@ab ok {"ids":[...]}──│  (subsystem forwards response)
```

This is bidirectional communication. The forked subsystem:
1. Receives commands from engine
2. Calls back to engine for operations it can't do locally
3. Returns response to original request

This is complex but achievable with existing Process infrastructure (it already supports bidirectional serial-based communication).

## Simplified Implementation Plan

### Phase 1: Infrastructure

1. **Create subsystem binary scaffold**
   - `cmd/ze-subsystem/main.go`
   - `--mode=cache|route|...` flag
   - Basic 5-stage protocol implementation

2. **Add subsystem spawning to server**
   - Config option to spawn internal subsystems as processes
   - Reuse existing Process/ProcessManager

3. **Bidirectional command flow**
   - Subsystem can send commands back to engine
   - Engine routes subsystem commands like API commands

### Phase 2: First Subsystem

4. **Migrate cache handlers to forked process**
   - Create cache subsystem mode
   - Implement cache commands in subprocess
   - Engine routes `bgp cache *` to subprocess
   - Subprocess calls back for reactor operations

### Phase 3: Remaining Subsystems

5. **Migrate other subsystems** (if beneficial)

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSubsystemSpawn` | `internal/plugin/subsystem_test.go` | Engine spawns subsystem process | |
| `TestSubsystemProtocol` | `internal/plugin/subsystem_test.go` | 5-stage protocol completes | |
| `TestSubsystemCommand` | `internal/plugin/subsystem_test.go` | Command routed to subprocess | |
| `TestSubsystemCallback` | `internal/plugin/subsystem_test.go` | Subprocess calls back to engine | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `subsystem-spawn` | `test/data/plugin/subsystem-spawn.ci` | Subprocess starts and completes 5-stage | |
| `subsystem-command` | `test/data/plugin/subsystem-command.ci` | Command executed via subprocess | |

## Files to Create

```
cmd/
└── ze-subsystem/
    └── main.go              # Subsystem binary entry point

internal/plugin/
├── subsystem.go             # SubsystemHandler (forked process wrapper)
└── subsystem_test.go        # Tests
```

## Files to Modify

- `internal/plugin/server.go` - Add subsystem spawning during startup
- `internal/plugin/command.go` - Add process-based command routing

## Implementation Steps

**Self-Critical Review:** After each step, review for issues and fix before proceeding.

1. **Write unit tests** - Create tests BEFORE implementation
2. **Run tests** - Verify FAIL
3. **Create subsystem binary** - cmd/ze-subsystem with 5-stage protocol
4. **Create SubsystemHandler** - wrapper that spawns and routes to process
5. **Integrate with server** - spawn subsystems during startup
6. **Run tests** - Verify PASS
7. **Verify all** - `make lint && make test && make functional`

## Questions to Resolve

1. **Which subsystems benefit from forking?**
   - Stateless handlers (help, version) - simple
   - Stateful handlers (cache, route) - complex bidirectional needed

2. **Bidirectional complexity worth it?**
   - For truly external programs, yes
   - For internal subsystems, maybe not

3. **Alternative: Just external plugins?**
   - Keep init() for internal handlers
   - Use plugin protocol for external programs
   - This is already working

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes
