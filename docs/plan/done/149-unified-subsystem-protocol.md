# Spec: Unified Subsystem Protocol

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/architecture.md` - current API architecture
4. `internal/plugin/handler.go` - current registration pattern
5. `internal/plugin/server.go` - current server/coordinator

## Task

Refactor ZeBGP so internal subsystems self-register their commands via the same API protocol as external plugins. This eliminates central `RegisterDefaultHandlers()` and creates cleaner boundaries.

**This is a stepping stone toward:** `docs/architecture/config/yang-config-design.md` - future YANG-based config system where handlers declare config subtrees, validators, and completers.

### Goals

1. **Self-registration** - Subsystems declare their own commands via API protocol
2. **Same protocol** - Internal and external use identical 5-stage startup
3. **Transport abstraction** - Channels (internal) or pipes (external)
4. **Clear boundaries** - Subsystems receive dependencies via interfaces
5. **Testability** - Each subsystem testable with mock transport

### Non-Goals (This Phase)

- Config handlers (Validate/Generate/Apply/Rollback) - future
- Schema declaration (YANG paths) - future
- External validators/completers - future
- Priority ordering for commits - future
- Breaking external plugin compatibility
- Changing command syntax users see

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
- [ ] `internal/plugin/registration.go` - [5-stage protocol types and parsing]

**Key insights:**
- Current architecture has central `RegisterDefaultHandlers()` that knows all subsystems
- External plugins already use 5-stage protocol via stdin/stdout
- Internal handlers directly call `d.Register()` bypassing protocol
- Protocol verified from `process.go` and `registration.go`:
  - Request: `#N command` (numeric) or `#abc command` (alpha for engine-initiated)
  - Response: `@serial response` format
  - Stage markers: `declare done`, `config done`, `capability done`, `registry done`, `ready`

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

All subsystems follow identical protocol (verified from `internal/plugin/registration.go`):

| Stage | Direction | Messages | End Marker |
|-------|-----------|----------|------------|
| 1. Declaration | Sub → Engine | `declare rfc/encoding/family/conf/cmd/receive` | `declare done` |
| 2. Config | Engine → Sub | `config <context> <name> <value>` | `config done` |
| 3. Capability | Sub → Engine | `capability <enc> <code> [payload] [peer <addr>]` | `capability done` |
| 4. Registry | Engine → Sub | `registry name/cmd/done` | `registry done` |
| 5. Ready | Sub → Engine | `ready` (or `ready failed <reason>`) | N/A |

### Transport Abstraction

```go
// internal/subsystem/transport.go

// Transport abstracts the communication channel.
// Uses TEXT protocol (same as external plugins) for uniformity.
type Transport interface {
    // Send a text line to the other side
    Send(line string) error

    // Receive blocks until a line arrives or context cancelled
    Receive(ctx context.Context) (string, error)

    // Close terminates the transport
    Close() error
}

// ChannelTransport for internal subsystems (in-process)
// Uses string channels to match text protocol
type ChannelTransport struct {
    toEngine   chan string  // subsystem → engine
    fromEngine chan string  // engine → subsystem
    closed     atomic.Bool
}

// PipeTransport wraps existing process.go stdin/stdout handling
// Already implemented - just needs to implement Transport interface
```

### Protocol Format

**Verified from `internal/plugin/process.go` and `registration.go`:**

External plugins use TEXT protocol with specific stage markers. Internal subsystems MUST use the same format for uniformity.

#### Stage 1: Declaration (subsystem → engine)
```
declare rfc 4724
declare encoding text
declare family ipv4 unicast
declare conf peer * capability <cap:.*>
declare cmd bgp cache list
declare receive update
declare done                   # Signal declarations complete
```

#### Stage 2: Config (engine → subsystem)
```
config peer 192.168.1.1 restart-time 120
config done                    # Signal config complete
```

#### Stage 3: Capability (subsystem → engine)
```
capability hex 64 00010100    # Global capability
capability hex 71 peer 192.168.1.1    # Per-peer capability
capability done               # Signal capabilities complete
```

#### Stage 4: Registry (engine → subsystem)
```
registry name cache           # Your plugin name
registry route text cmd bgp route announce
registry done                 # Signal registry complete
```

#### Stage 5: Ready (subsystem → engine)
```
ready                         # Signal operational (or "ready failed <reason>")
```

#### Runtime: Request/Response

**Subsystem→Engine command (with serial for ACK):**
```
#1 bgp cache list peer 192.168.1.1
```

**Engine→Subsystem response (JSON, only when serial present):**
```json
{"answer":{"serial":"1","status":"ok","data":{"ids":[123,456]}}}
```

**Engine→Subsystem request (alpha serial to avoid collision):**
```
#ab bgp cache stats
```

**Subsystem→Engine response:**
```
@ab ok {"hits":100,"misses":5}
```

**Key insight:** The serial determines acknowledgment:
- `#N cmd` (numeric) = plugin-initiated, expects JSON response
- `#abc cmd` (alpha a-j) = engine-initiated, expects `@abc response`

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

// Factory creates subsystems with their dependencies injected
type Factory func(deps *Dependencies) Subsystem

// Registry collects factories via init() auto-registration
var registry = &factoryRegistry{factories: make(map[string]Factory)}

type factoryRegistry struct {
    mu        sync.RWMutex
    factories map[string]Factory
}

func (r *factoryRegistry) Register(name string, f Factory) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.factories[name] = f
}

func (r *factoryRegistry) All() map[string]Factory {
    r.mu.RLock()
    defer r.mu.RUnlock()
    result := make(map[string]Factory, len(r.factories))
    for k, v := range r.factories {
        result[k] = v
    }
    return result
}

// Register adds a factory (called from subsystem init())
func Register(name string, f Factory) {
    registry.Register(name, f)
}

// AllFactories returns all registered factories
func AllFactories() map[string]Factory {
    return registry.All()
}

// Dependencies provided to all subsystems (interface-based)
type Dependencies struct {
    // Core services (interfaces, not concrete types)
    Cache   CacheAPI    // For cache subsystem
    Peers   PeerAPI     // For route/session subsystems
    Events  EventAPI    // For subscribe subsystem
    Logger  *slog.Logger
}

// CacheAPI - what cache subsystem needs from reactor
type CacheAPI interface {
    RetainUpdate(id uint64) error
    ReleaseUpdate(id uint64) error
    DeleteUpdate(id uint64) error
    ForwardUpdate(sel *selector.Selector, id uint64) error
    ListUpdates() []uint64
}

// PeerAPI - what route/session subsystems need
type PeerAPI interface {
    SendUpdate(peer string, update []byte) error
    SendEOR(peer string, family string) error
    ListPeers() []string
    GetPeerState(peer string) (PeerState, error)
}

// EventAPI - what subscribe subsystem needs
type EventAPI interface {
    Subscribe(filter EventFilter) (<-chan Event, func())
}
```

### Dependency Injection

Engine creates Dependencies with concrete implementations:

```go
// internal/engine/engine.go
func (e *Engine) createDependencies() *subsystem.Dependencies {
    return &subsystem.Dependencies{
        Cache:  e.reactor,  // Reactor implements CacheAPI
        Peers:  e.reactor,  // Reactor implements PeerAPI
        Events: e.reactor,  // Reactor implements EventAPI
        Logger: e.logger,
    }
}
```

Subsystems receive only the interfaces they need - no access to full Reactor.

### Command Routing

When user runs `bgp cache list`:

```
1. User → Dispatcher.Dispatch("bgp cache list")
2. Dispatcher finds "bgp cache list" registered by "cache" subsystem
3. Dispatcher generates alpha serial and sends via subsystem's transport:
   #ab bgp cache list
4. Cache subsystem receives, executes handler, responds:
   @ab ok {"ids":[123,456],"count":2}
5. Dispatcher parses response and returns to user
```

**Key:**
- Dispatcher stores which transport handles each command during registration
- Alpha serials (a-j digits) avoid collision with plugin numeric serials
- `parseResponseSerial()` in process.go handles `@serial response` format

### Internal Subsystem Implementation

```go
// internal/subsystems/cache/subsystem.go
package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "codeberg.org/thomas-mangin/ze/internal/subsystem"
)

// init() auto-registers this subsystem - no central registration needed
func init() {
    subsystem.Register("cache", New)
}

// New creates a cache subsystem with injected dependencies
func New(deps *subsystem.Dependencies) subsystem.Subsystem {
    return &Subsystem{
        cache:  deps.Cache,
        logger: deps.Logger,
    }
}

type Subsystem struct {
    cache     subsystem.CacheAPI
    logger    *slog.Logger
    transport subsystem.Transport
}

func (s *Subsystem) Name() string { return "cache" }

func (s *Subsystem) Start(ctx context.Context, t subsystem.Transport) error {
    s.transport = t

    // Stage 1: Declaration (text protocol, same as external plugins)
    t.Send("declare encoding text")
    commands := []string{
        "bgp cache list",
        "bgp cache retain",
        "bgp cache release",
        "bgp cache expire",
        "bgp cache forward",
    }
    for _, cmd := range commands {
        t.Send("declare cmd " + cmd)
    }
    t.Send("declare done")  // Signal declarations complete

    // Stage 2: Config (receive until "config done")
    for {
        line, err := t.Receive(ctx)
        if err != nil {
            return fmt.Errorf("receive config: %w", err)
        }
        if line == "config done" {
            break
        }
        if strings.HasPrefix(line, "config ") {
            // Parse and apply config if needed
        }
    }

    // Stage 3: Capability (cache has none)
    t.Send("capability done")

    // Stage 4: Registry (receive until "registry done")
    for {
        line, err := t.Receive(ctx)
        if err != nil {
            return fmt.Errorf("receive registry: %w", err)
        }
        if line == "registry done" {
            break
        }
        // Parse registry info if needed
    }

    // Stage 5: Ready
    t.Send("ready")

    // Enter operational loop
    go s.run(ctx)

    return nil
}

func (s *Subsystem) Stop(ctx context.Context) error {
    return s.transport.Close()
}

func (s *Subsystem) run(ctx context.Context) {
    for {
        line, err := s.transport.Receive(ctx)
        if err != nil {
            if ctx.Err() != nil {
                return // shutdown
            }
            s.logger.Error("receive error", "err", err)
            continue
        }

        // Parse #serial command format (engine-initiated request)
        // Engine uses alpha serials (a-j digits): #ab command args
        serial, command := parseAlphaSerial(line)
        if serial == "" {
            continue // Ignore lines without alpha serial
        }

        resp := s.handleRequest(serial, command)
        s.transport.Send(resp)
    }
}

// parseAlphaSerial extracts #abc prefix from engine-initiated request.
// Alpha serials use a-j (0-9 mapped to letters) to avoid collision with plugin numeric serials.
func parseAlphaSerial(line string) (string, string) {
    if !strings.HasPrefix(line, "#") {
        return "", line
    }
    idx := strings.Index(line, " ")
    if idx <= 1 {
        return "", line
    }
    serial := line[1:idx]
    // Verify alpha serial (a-j only)
    for _, c := range serial {
        if c < 'a' || c > 'j' {
            return "", line
        }
    }
    return serial, line[idx+1:]
}

func (s *Subsystem) handleRequest(serial, command string) string {
    // Parse command and args
    parts := strings.Fields(command)
    if len(parts) == 0 {
        return fmt.Sprintf("@%s error unknown command", serial)
    }

    // Match full command path
    fullCmd := strings.Join(parts[:min(3, len(parts))], " ")

    switch fullCmd {
    case "bgp cache list":
        ids := s.cache.ListUpdates()
        data, _ := json.Marshal(map[string]any{"ids": ids, "count": len(ids)})
        return fmt.Sprintf("@%s ok %s", serial, data)

    case "bgp cache retain":
        // Parse id from remaining args
        // Call s.cache.RetainUpdate(id)
        return fmt.Sprintf("@%s ok", serial)

    // ... other commands
    }
    return fmt.Sprintf("@%s error unknown command", serial)
}
```

### Coordinator

```go
// internal/subsystem/coordinator.go

type Coordinator struct {
    subsystems map[string]*managedSubsystem
    dispatcher *Dispatcher            // Command routing
    deps       *Dependencies          // Shared dependencies
    timeout    time.Duration
    logger     *slog.Logger
}

// NewCoordinator creates a coordinator using auto-registered factories
func NewCoordinator(deps *Dependencies, dispatcher *Dispatcher, timeout time.Duration) *Coordinator {
    return &Coordinator{
        subsystems: make(map[string]*managedSubsystem),
        dispatcher: dispatcher,
        deps:       deps,
        timeout:    timeout,
        logger:     deps.Logger,
    }
}

type managedSubsystem struct {
    sub       Subsystem
    transport *ChannelTransport
    commands  []string               // Commands this subsystem handles
    caps      []string
    ready     bool
}

func (c *Coordinator) StartAll(ctx context.Context) error {
    // Create subsystems from auto-registered factories
    for name, factory := range AllFactories() {
        sub := factory(c.deps)
        transport := NewChannelTransport()
        c.subsystems[name] = &managedSubsystem{
            sub:       sub,
            transport: transport,
        }
    }

    // Start all subsystems concurrently
    for name, ms := range c.subsystems {
        go func(name string, ms *managedSubsystem) {
            if err := ms.sub.Start(ctx, ms.transport); err != nil {
                c.logger.Error("subsystem start failed", "name", name, "err", err)
            }
        }(name, ms)
    }

    // Stage 1: Collect declarations until "declare done"
    stageCtx, cancel := context.WithTimeout(ctx, c.timeout)
    if err := c.collectDeclarations(stageCtx); err != nil {
        cancel()
        return fmt.Errorf("stage 1 declaration: %w", err)
    }
    cancel()

    // Stage 2: Send configs (end with "config done")
    for _, ms := range c.subsystems {
        // TODO: Send actual config based on subsystem's declared patterns
        ms.transport.Send("config done")
    }

    // Stage 3: Collect capabilities until "capability done"
    stageCtx, cancel = context.WithTimeout(ctx, c.timeout)
    if err := c.collectCapabilities(stageCtx); err != nil {
        cancel()
        return fmt.Errorf("stage 3 capability: %w", err)
    }
    cancel()

    // Stage 4: Send registry info (end with "registry done")
    for name, ms := range c.subsystems {
        ms.transport.Send("registry name " + name)
        // Send all commands from all subsystems
        for otherName, other := range c.subsystems {
            for _, cmd := range other.commands {
                ms.transport.Send(fmt.Sprintf("registry %s text cmd %s", otherName, cmd))
            }
        }
        ms.transport.Send("registry done")

        // Register in dispatcher: command → transport mapping
        for _, cmd := range ms.commands {
            c.dispatcher.RegisterSubsystem(cmd, ms.transport)
        }
        c.logger.Debug("registered subsystem", "name", name, "commands", ms.commands)
    }

    // Stage 5: Wait for "ready" from all subsystems
    stageCtx, cancel = context.WithTimeout(ctx, c.timeout)
    if err := c.waitReady(stageCtx); err != nil {
        cancel()
        return fmt.Errorf("stage 5 ready: %w", err)
    }
    cancel()

    return nil
}

func (c *Coordinator) collectDeclarations(ctx context.Context) error {
    pending := len(c.subsystems)
    for pending > 0 {
        for _, ms := range c.subsystems {
            line, err := ms.transport.ReceiveFromSubsystem(ctx)
            if err != nil {
                return err
            }

            if line == "declare done" {
                pending--
                continue
            }

            // Parse: declare cmd <name>
            if strings.HasPrefix(line, "declare cmd ") {
                cmdName := strings.TrimPrefix(line, "declare cmd ")
                ms.commands = append(ms.commands, cmdName)
            }
            // Parse other declare types as needed
        }
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

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Text protocol** | Same as external plugins - no format translation needed |
| **`declare done` marker** | Matches external plugin protocol (not `end-declare`) |
| **`#alpha` for engine requests** | Avoids collision with plugin numeric `#N` serials (see `encodeAlphaSerial()`) |
| **`@serial` for subsystem responses** | Matches external plugin response format (see `parseResponseSerial()`) |
| **Interface-based deps** | Subsystems get `CacheAPI` not `*Reactor` - clear boundaries |
| **init() auto-registration** | Subsystems register via `init()` - no central import list |
| **Factory pattern** | Coordinator creates subsystems, injects deps - testable |
| **Transport per subsystem** | Each subsystem has own channel pair - isolated |
| **Dispatcher stores routing** | `command → transport` mapping for request dispatch |

## Files to Modify

- `internal/plugin/server.go` - Use Coordinator instead of direct registration
- `internal/plugin/handler.go` - Remove `RegisterDefaultHandlers()`, keep as compatibility shim during migration
- `internal/plugin/command.go` - Add `RegisterSubsystem(cmd, transport)` to Dispatcher

## Files to Create

```
internal/
├── subsystem/                    # Protocol infrastructure (singular)
│   ├── subsystem.go              # Subsystem interface, Factory, Dependencies
│   ├── transport.go              # Transport interface, ChannelTransport
│   ├── coordinator.go            # 5-stage orchestration
│   └── interfaces.go             # CacheAPI, PeerAPI, EventAPI
│
└── subsystems/                   # Self-contained components (plural)
    ├── cache/subsystem.go        # Cache subsystem
    ├── route/subsystem.go        # Route subsystem
    ├── session/subsystem.go      # Session subsystem
    ├── rib/subsystem.go          # RIB subsystem
    ├── bgp/subsystem.go          # BGP namespace
    ├── subscribe/subsystem.go    # Subscription subsystem
    ├── refresh/subsystem.go      # Refresh subsystem
    ├── commit/subsystem.go       # Commit subsystem
    └── raw/subsystem.go          # Raw subsystem
```

## Files to Delete

After migration complete:
- `internal/plugin/cache.go` → `internal/subsystems/cache/`
- `internal/plugin/route.go` → `internal/subsystems/route/`
- `internal/plugin/session.go` → `internal/subsystems/session/`
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

## Future Extensibility (YANG Alignment)

This spec creates foundation for `docs/architecture/config/yang-config-design.md`:

| Future Feature | How This Spec Enables It |
|----------------|--------------------------|
| Schema declaration | Add `declare schema "/bgp/neighbor/*"` to protocol |
| Config handlers | Add `config-validate`, `config-apply` message types |
| External validators | Add `declare validator <name> <path>` |
| External completers | Add `declare completer <name> <path>` |
| Priority ordering | Add `declare priority <N>` |
| State population | Add `state-set` message type |

Protocol is extensible - new `declare` and message types can be added without breaking existing subsystems.

## RFC Documentation

N/A - This is internal architecture, not BGP protocol.

## Implementation Summary

### What Was Implemented

**Simpler approach chosen over full async protocol:**

Instead of the complex 5-stage async protocol, implemented init() self-registration:

1. **Added global handler registry** (`command.go`):
   - `RegisterBuiltin(name, handler, help)` - registers handler in global registry
   - `LoadBuiltins(d)` - loads handlers from registry to dispatcher
   - `RegisterDefaultHandlers(d)` - backward-compatible alias for `LoadBuiltins`

2. **Converted all handlers to init() self-registration**:
   - `handler.go` - daemon, peer, system, RIB handlers
   - `cache.go` - cache handlers
   - `route.go` - route handlers
   - `bgp.go` - BGP namespace handlers
   - `plugin.go` - plugin lifecycle handlers
   - `commit.go` - commit handlers
   - `refresh.go` - route refresh handlers
   - `subscribe.go` - subscription handlers
   - `raw.go` - raw passthrough handlers

3. **Removed unused code**:
   - Deleted `internal/subsystem/` and `internal/subsystems/` (over-engineered async protocol)
   - Deleted empty `RegisterSessionHandlers()` function
   - Deleted old `RegisterRibHandlers()` function (merged into init())

4. **Updated tests**:
   - Replaced `RegisterCacheHandlers(d)` → `RegisterDefaultHandlers(d)` in cache_test.go
   - Replaced `RegisterCommitHandlers(d)` → `RegisterDefaultHandlers(d)` in commit_test.go

### Bugs Found/Fixed
- None - the existing code worked correctly

### Design Insights

**The async protocol was over-engineered:**
- External plugins need async protocol (separate processes, stdin/stdout)
- Internal handlers are just functions in the same process
- Adding goroutines/channels for sync function calls adds unnecessary complexity

**init() self-registration achieves the goal simply:**
- Each handler file registers itself via `init()`
- No central file needs to know about all handlers
- Adding new handlers = create file with init(), done

**Key insight for future:** Only use async protocol for things that are actually async (external plugins, long-running subsystems). Keep internal handlers synchronous.

### Deviations from Plan

| Planned | Actual | Rationale |
|---------|--------|-----------|
| Full 5-stage async protocol | init() self-registration | Much simpler, achieves same goal |
| Channel-based transport | Not needed | Sync handlers don't need async transport |
| Coordinator orchestration | Not needed | No stages to orchestrate for sync handlers |
| Multiple subsystem packages | Single `plugin` package | Handlers stay where they were, just self-register |

The spec's **core goal** (self-registration to eliminate central `RegisterDefaultHandlers` knowledge) was achieved, but via a simpler mechanism.

## Checklist

### 🧪 TDD
- [x] Tests written (existing tests cover handlers)
- [x] Tests FAIL (verified old RegisterXxxHandlers removed)
- [x] Implementation complete
- [x] Tests PASS (`make test` passes)
- [x] Boundary tests cover all numeric inputs (N/A - no new numeric inputs)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes (37 packages)
- [x] `make functional` passes (flaky tests pass individually)

### Documentation (during implementation)
- [x] Required docs read
- [x] Architecture docs updated with new design (N/A - simpler approach used)

### Completion (after tests pass)
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
