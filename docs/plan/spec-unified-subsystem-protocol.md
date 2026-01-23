# Spec: Unified Subsystem Protocol

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/architecture.md` - current API architecture
4. `internal/plugin/handler.go` - current registration pattern
5. `internal/plugin/server.go` - current server/coordinator

## Task

Refactor ZeBGP to use a unified subsystem protocol where internal subsystems register their commands via the same 5-stage API protocol as external plugins. This eliminates the distinction between internal and external components, creating cleaner boundaries and enabling dynamic loading.

### Goals

1. **Uniform protocol** - Internal and external subsystems use identical 5-stage startup
2. **Self-registration** - Subsystems declare their own commands via API calls
3. **Transport abstraction** - Same protocol over channels (internal) or pipes (external)
4. **Clear boundaries** - Subsystems communicate only via defined interfaces
5. **Testability** - Each subsystem testable in isolation with mock transport

### Non-Goals

- Breaking external plugin compatibility
- Changing the command syntax users see
- Modifying the wire protocol format

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - [current API design, RIB ownership]
- [ ] `docs/architecture/api/ipc_protocol.md` - [current IPC protocol, 5-stage startup]
- [ ] `docs/architecture/api/capability-contract.md` - [capability negotiation]
- [ ] `docs/architecture/core-design.md` - [engine/plugin split]

### Source Files
- [ ] `internal/plugin/handler.go` - [current central registration]
- [ ] `internal/plugin/server.go` - [current coordinator, stage management]
- [ ] `internal/plugin/command.go` - [dispatcher implementation]
- [ ] `internal/plugin/process.go` - [external plugin process management]

**Key insights:**
- Current architecture has central `RegisterDefaultHandlers()` that knows all subsystems
- External plugins already use 5-stage protocol via stdin/stdout
- Internal handlers directly call `d.Register()` bypassing protocol

## Architecture

### Current State

```
┌─────────────────────────────────────────────────────────────────┐
│                      internal/plugin/                            │
│                                                                  │
│  handler.go (CENTRAL)                                           │
│  └─> RegisterDefaultHandlers(d)                                 │
│      ├─> RegisterCacheHandlers(d)     // direct d.Register()    │
│      ├─> RegisterRouteHandlers(d)     // direct d.Register()    │
│      ├─> RegisterSessionHandlers(d)   // direct d.Register()    │
│      └─> ... (11 explicit calls)                                │
│                                                                  │
│  process.go (EXTERNAL PLUGINS)                                  │
│  └─> 5-stage protocol via stdin/stdout                          │
│      └─> Commands registered in CommandRegistry                 │
└─────────────────────────────────────────────────────────────────┘
```

**Problems:**
- `handler.go` must import and know about all subsystems
- Adding subsystem requires editing central file
- Internal vs external have different registration paths
- Tight coupling prevents independent testing

### Target State

```
┌─────────────────────────────────────────────────────────────────┐
│                         Engine Core                              │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Coordinator                                                 ││
│  │  - Orchestrates 5-stage startup for ALL subsystems          ││
│  │  - No knowledge of specific subsystems                       ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Router (command dispatch)                                   ││
│  │  - Receives registrations via protocol                       ││
│  │  - Routes requests to registered handlers                    ││
│  └─────────────────────────────────────────────────────────────┘│
│                              ▲                                   │
│              Transport abstraction (uniform)                     │
│         ┌────────────────────┼────────────────────┐             │
│         │                    │                    │             │
│    ChannelTransport    ChannelTransport     PipeTransport       │
│         │                    │                    │             │
│  ┌──────┴──────┐     ┌──────┴──────┐     ┌──────┴──────┐       │
│  │   Cache     │     │    Route    │     │  External   │       │
│  │  Subsystem  │     │  Subsystem  │     │   Plugin    │       │
│  └─────────────┘     └─────────────┘     └─────────────┘       │
│    (internal)          (internal)           (external)          │
└─────────────────────────────────────────────────────────────────┘
```

### 5-Stage Protocol (Uniform)

All subsystems follow identical protocol:

| Stage | Direction | Messages | Purpose |
|-------|-----------|----------|---------|
| 1. Declaration | Sub → Engine | `declare cmd <name> <desc>` | Announce commands |
| 2. Config | Engine → Sub | `config {...}` | Send configuration |
| 3. Capability | Sub → Engine | `capability <cap>` | Announce capabilities |
| 4. Registry | Internal | (commands registered) | Engine registers handlers |
| 5. Running | Sub → Engine | `plugin session ready` | Signal operational |

### Transport Abstraction

```go
// internal/subsystem/transport.go

// Transport abstracts the communication channel
type Transport interface {
    // Send a message to the other side
    Send(msg *Message) error

    // Receive blocks until a message arrives or context cancelled
    Receive(ctx context.Context) (*Message, error)

    // Close terminates the transport
    Close() error
}

// ChannelTransport for internal subsystems (in-process)
type ChannelTransport struct {
    toEngine   chan *Message  // subsystem → engine
    fromEngine chan *Message  // engine → subsystem
}

// PipeTransport for external plugins (stdin/stdout)
type PipeTransport struct {
    stdin  io.WriteCloser
    stdout *bufio.Scanner
    stderr io.ReadCloser
}
```

### Subsystem Interface

```go
// internal/subsystem/subsystem.go

// Subsystem defines the contract for all components
type Subsystem interface {
    // Name returns the subsystem identifier (e.g., "cache", "route")
    Name() string

    // Start begins the subsystem with the given transport
    // Subsystem must complete 5-stage protocol via transport
    Start(ctx context.Context, transport Transport) error

    // Stop gracefully shuts down the subsystem
    Stop(ctx context.Context) error
}
```

### Message Protocol

```go
// internal/subsystem/message.go

type Message struct {
    Type string `json:"type"`

    // Declaration stage
    Command     string `json:"command,omitempty"`
    Description string `json:"description,omitempty"`

    // Config stage
    Config json.RawMessage `json:"config,omitempty"`

    // Capability stage
    Capability string `json:"capability,omitempty"`

    // Request/Response (operational)
    Serial  string         `json:"serial,omitempty"`
    Args    []string       `json:"args,omitempty"`
    Status  string         `json:"status,omitempty"`
    Data    any            `json:"data,omitempty"`
    Error   string         `json:"error,omitempty"`
}
```

### Internal Subsystem Implementation

```go
// internal/subsystems/cache/subsystem.go
package cache

import (
    "context"
    "codeberg.org/thomas-mangin/ze/internal/subsystem"
)

type Subsystem struct {
    store  CacheStore
    logger *slog.Logger
}

func New(store CacheStore) *Subsystem {
    return &Subsystem{store: store}
}

func (s *Subsystem) Name() string { return "cache" }

func (s *Subsystem) Start(ctx context.Context, t subsystem.Transport) error {
    // Stage 1: Declaration
    commands := []struct{ name, desc string }{
        {"bgp cache list", "List cached msg-ids"},
        {"bgp cache retain", "Prevent eviction of cached message"},
        {"bgp cache release", "Allow eviction (reset TTL)"},
        {"bgp cache expire", "Remove from cache immediately"},
        {"bgp cache forward", "Forward cached UPDATE to peers"},
    }
    for _, cmd := range commands {
        if err := t.Send(&subsystem.Message{
            Type:        "declare",
            Command:     cmd.name,
            Description: cmd.desc,
        }); err != nil {
            return fmt.Errorf("declare %s: %w", cmd.name, err)
        }
    }

    // Stage 2: Config (receive and apply)
    msg, err := t.Receive(ctx)
    if err != nil {
        return fmt.Errorf("receive config: %w", err)
    }
    if msg.Type != "config" {
        return fmt.Errorf("expected config, got %s", msg.Type)
    }
    // Apply configuration if needed

    // Stage 3: Capability (cache has none)

    // Stage 4: Registry (engine handles)

    // Stage 5: Running
    if err := t.Send(&subsystem.Message{Type: "ready"}); err != nil {
        return fmt.Errorf("send ready: %w", err)
    }

    // Enter operational loop
    go s.run(ctx, t)

    return nil
}

func (s *Subsystem) run(ctx context.Context, t subsystem.Transport) {
    for {
        msg, err := t.Receive(ctx)
        if err != nil {
            if ctx.Err() != nil {
                return // shutdown
            }
            s.logger.Error("receive error", "err", err)
            continue
        }

        if msg.Type == "request" {
            resp := s.handleRequest(msg)
            t.Send(resp)
        }
    }
}

func (s *Subsystem) handleRequest(msg *subsystem.Message) *subsystem.Message {
    // Dispatch based on msg.Command
    // Return response with same Serial
}
```

### Coordinator

```go
// internal/subsystem/coordinator.go

type Coordinator struct {
    subsystems map[string]managedSubsystem
    router     *Router
    timeout    time.Duration
    logger     *slog.Logger
}

type managedSubsystem struct {
    sub       Subsystem
    transport Transport
    commands  []string
    caps      []string
    ready     bool
}

func (c *Coordinator) StartAll(ctx context.Context) error {
    // Start all subsystems concurrently
    var wg sync.WaitGroup
    errCh := make(chan error, len(c.subsystems))

    for name, ms := range c.subsystems {
        wg.Add(1)
        go func(name string, ms managedSubsystem) {
            defer wg.Done()
            if err := ms.sub.Start(ctx, ms.transport); err != nil {
                errCh <- fmt.Errorf("subsystem %s: %w", name, err)
            }
        }(name, ms)
    }

    // Collect declarations (Stage 1) with timeout
    stageCtx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel()

    if err := c.collectDeclarations(stageCtx); err != nil {
        return fmt.Errorf("stage 1 declaration: %w", err)
    }

    // Send configs (Stage 2)
    if err := c.sendConfigs(ctx); err != nil {
        return fmt.Errorf("stage 2 config: %w", err)
    }

    // Collect capabilities (Stage 3)
    stageCtx, cancel = context.WithTimeout(ctx, c.timeout)
    defer cancel()

    if err := c.collectCapabilities(stageCtx); err != nil {
        return fmt.Errorf("stage 3 capability: %w", err)
    }

    // Register commands (Stage 4)
    c.registerCommands()

    // Wait for ready (Stage 5)
    stageCtx, cancel = context.WithTimeout(ctx, c.timeout)
    defer cancel()

    if err := c.waitReady(stageCtx); err != nil {
        return fmt.Errorf("stage 5 ready: %w", err)
    }

    return nil
}
```

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestChannelTransportSendReceive` | `internal/subsystem/transport_test.go` | Channel transport works bidirectionally | |
| `TestChannelTransportClose` | `internal/subsystem/transport_test.go` | Close terminates Receive | |
| `TestCoordinatorStartAll` | `internal/subsystem/coordinator_test.go` | All stages complete in order | |
| `TestCoordinatorTimeout` | `internal/subsystem/coordinator_test.go` | Stage timeout triggers error | |
| `TestCoordinatorPartialFailure` | `internal/subsystem/coordinator_test.go` | One subsystem failure aborts all | |
| `TestCacheSubsystemProtocol` | `internal/subsystems/cache/subsystem_test.go` | Cache follows 5-stage protocol | |
| `TestCacheSubsystemCommands` | `internal/subsystems/cache/subsystem_test.go` | All cache commands work via transport | |
| `TestRouteSubsystemProtocol` | `internal/subsystems/route/subsystem_test.go` | Route follows 5-stage protocol | |
| `TestRouterDispatch` | `internal/subsystem/router_test.go` | Commands route to correct subsystem | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Stage timeout | 1ms-10m | 10m | 0 | >10m |
| Command name length | 1-256 | 256 chars | empty | 257 chars |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `subsystem-startup` | `test/plugin/subsystem-startup.ci` | Internal subsystem completes 5-stage | |
| `subsystem-mixed` | `test/plugin/subsystem-mixed.ci` | Internal + external coexist | |
| `subsystem-timeout` | `test/plugin/subsystem-timeout.ci` | Slow subsystem times out | |

### Future (deferred)

- Hot reload of subsystems (requires more infrastructure)
- Subsystem dependency ordering (if needed)

## Files to Modify

- `internal/plugin/server.go` - Use Coordinator instead of direct registration
- `internal/plugin/handler.go` - Remove `RegisterDefaultHandlers()`, keep as compatibility shim during migration
- `internal/plugin/process.go` - Use PipeTransport abstraction

## Files to Create

- `internal/subsystem/subsystem.go` - Subsystem interface
- `internal/subsystem/transport.go` - Transport interface + ChannelTransport
- `internal/subsystem/message.go` - Protocol messages
- `internal/subsystem/coordinator.go` - 5-stage orchestration
- `internal/subsystem/router.go` - Command routing
- `internal/subsystems/cache/subsystem.go` - Cache subsystem (migrated)
- `internal/subsystems/route/subsystem.go` - Route subsystem (migrated)
- `internal/subsystems/session/subsystem.go` - Session subsystem (migrated)
- `internal/subsystems/rib/subsystem.go` - RIB subsystem (migrated)
- `internal/subsystems/bgp/subsystem.go` - BGP namespace subsystem (migrated)
- `internal/subsystems/subscribe/subsystem.go` - Subscription subsystem (migrated)
- `internal/subsystems/refresh/subsystem.go` - Refresh subsystem (migrated)
- `internal/subsystems/commit/subsystem.go` - Commit subsystem (migrated)
- `internal/subsystems/raw/subsystem.go` - Raw subsystem (migrated)

## Files to Delete

After migration complete:
- `internal/plugin/cache.go` → moved to `internal/subsystems/cache/`
- `internal/plugin/route.go` → moved to `internal/subsystems/route/`
- `internal/plugin/session.go` → moved to `internal/subsystems/session/`
- (etc. for all subsystem files)

## Implementation Steps

### Phase 1: Infrastructure (Week 1)

1. **Create subsystem package**
   - Write `internal/subsystem/subsystem.go` with interface
   - Write `internal/subsystem/transport.go` with ChannelTransport
   - Write `internal/subsystem/message.go` with protocol types
   - Write tests, verify FAIL
   - Implement, verify PASS

2. **Create coordinator**
   - Write `internal/subsystem/coordinator.go`
   - Write `internal/subsystem/router.go`
   - Write tests for 5-stage orchestration
   - Implement, verify PASS

### Phase 2: First Subsystem Migration (Week 2)

3. **Migrate cache subsystem**
   - Create `internal/subsystems/cache/subsystem.go`
   - Implement Subsystem interface
   - Write tests using mock transport
   - Verify cache commands work via protocol
   - Run `make test && make lint && make functional`

4. **Integration**
   - Update `internal/plugin/server.go` to use Coordinator
   - Keep old registration as fallback
   - Verify external plugins still work
   - Run full test suite

### Phase 3: Remaining Subsystems (Week 3-4)

5. **Migrate remaining subsystems** (one at a time)
   - route → `internal/subsystems/route/`
   - session → `internal/subsystems/session/`
   - rib → `internal/subsystems/rib/`
   - bgp → `internal/subsystems/bgp/`
   - subscribe → `internal/subsystems/subscribe/`
   - refresh → `internal/subsystems/refresh/`
   - commit → `internal/subsystems/commit/`
   - raw → `internal/subsystems/raw/`

6. **Each migration:**
   - Write subsystem with 5-stage protocol
   - Write tests
   - Verify functional tests pass
   - Remove old handler file

### Phase 4: Cleanup (Week 5)

7. **Remove legacy code**
   - Remove `RegisterDefaultHandlers()` and related
   - Remove old handler files from `internal/plugin/`
   - Update architecture documentation

8. **Final verification**
   - `make test && make lint && make functional`
   - Manual testing with external plugins
   - Update CLAUDE.md with new architecture

## RFC Documentation

N/A - This is internal architecture, not BGP protocol.

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered during implementation]

### Design Insights
- [Key learnings that should be documented elsewhere]

### Deviations from Plan
- [Any differences from original plan and why]

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] Architecture docs updated with new design

### Completion (after tests pass)
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
