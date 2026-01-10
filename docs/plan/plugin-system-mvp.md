# ZeBGP Plugin System - MVP Specification (Revised)

**Status:** Proposed (revised after architecture review)
**Date:** 2026-01-03

**Purpose:** Define minimum viable plugin system aligned with current architecture.

**See:** `docs/architecture/rib-transition.md` for overall architecture direction.

---

## Design Transition Alignment

This plugin system is **compatible** with the Pool + Wire design:

| Plugin Pattern | Pool+Wire Alignment |
|---------------|---------------------|
| `RawMessage.RawBytes` | Wire bytes (compatible with pool storage) |
| `RawMessage.AttrsWire` | Lazy parsing via `AttributesWire` (designed for this) |
| `sendCtx`/`recvCtx` | Zero-copy forwarding check |
| `OnMessage` with raw bytes | No forced parsing on receive path |

### Key Integration Points

```go
// Plugins receive raw bytes - compatible with pool storage
func (p *MyPlugin) OnMessage(peer *Peer, msg RawMessage) {
    // msg.RawBytes = wire bytes (can be stored via pool.Intern)
    // msg.AttrsWire = lazy parsing wrapper

    // Only parse if needed:
    if msg.AttrsWire != nil {
        asPath, _ := msg.AttrsWire.Get(attribute.AttrASPath)
    }
}
```

### No Conflicts

The plugin system operates at a higher layer than pool storage:
- Plugins see `RawMessage` with wire bytes
- Storage layer (pool) handles deduplication internally
- Plugins don't need to know about handles

---

## Design Principles

1. **Wrap, don't replace** - Existing code works; plugins wrap it
2. **Raw bytes first** - Use `RawMessage` pattern, parse on demand
3. **Preserve encoding contexts** - Keep `sendCtx`/`recvCtx` for zero-copy
4. **Incremental phases** - Each phase is independently useful

---

## Phased Approach

| Phase | Scope | Dependency |
|-------|-------|------------|
| 0 | Peer lifecycle callbacks (OnEstablished, OnClose) | None |
| 0.5 | Message callbacks (OnUpdateReceived with RawMessage) | Phase 0 |
| 1 | Plugin interface, Registry, RIB/API wrappers | Phase 0.5 |
| 2 | Capability hooks (GetCapabilities, OnOpenReceived) | Phase 1 |
| 3 | External plugins via gRPC | Phase 2 |

---

## Phase 0: Peer Lifecycle Callbacks

See `docs/plan/phase0-peer-callbacks.md` for full specification.

Summary:
- Add `OnEstablished(peer *Peer)` callback to Reactor
- Add `OnClose(peer *Peer, reason string)` callback to Reactor
- Emit API state messages on these events

---

## Phase 0.5: Message Callbacks

Extend existing `messageCallback` to support plugin pattern:

```go
// MessageObserver receives BGP messages without parsing overhead.
// Multiple observers can be registered; each sees all messages.
type MessageObserver interface {
    // OnMessage is called for every BGP message sent/received.
    // msg contains raw bytes - observer decides if/how to parse.
    // MUST NOT block; use goroutine for slow processing.
    OnMessage(peer *Peer, msg RawMessage)
}

// Reactor.AddMessageObserver registers an observer.
// Observers are called in registration order.
func (r *Reactor) AddMessageObserver(obs MessageObserver)
```

Uses existing `api.RawMessage`:
```go
type RawMessage struct {
    Type      message.MessageType
    RawBytes  []byte              // Wire bytes (no header)
    Timestamp time.Time
    MessageID uint64
    AttrsWire *attribute.AttributesWire // Lazy parsing
    Direction string              // "sent" or "received"
}
```

---

## Phase 1: Plugin System Core

### Plugin Interface

```go
package plugin

// Plugin is the base interface all plugins must implement.
type Plugin interface {
    // Name returns unique plugin identifier.
    Name() string

    // Version returns semantic version.
    Version() string

    // Dependencies returns plugin names this one requires.
    Dependencies() []string

    // Init is called once during startup.
    Init(ctx context.Context, reactor *Reactor) error

    // Close is called during shutdown.
    Close() error
}
```

Note: `Init` receives `*Reactor` directly (not an interface). This is intentional:
- Avoids interface duplication with `api.ReactorInterface`
- Plugins are internal; they can access reactor internals
- External plugins (Phase 3) will use gRPC, not Go interfaces

### PeerObserver Interface

```go
// PeerObserver receives peer lifecycle events.
// Implement this in addition to Plugin for peer-aware plugins.
type PeerObserver interface {
    Plugin

    // OnEstablished is called when peer reaches Established state.
    // peer provides access to SendContext, RecvContext, AdjRIBOut, etc.
    OnEstablished(peer *Peer)

    // OnClose is called when peer leaves Established state.
    OnClose(peer *Peer, reason string)
}
```

### MessageObserver Interface

```go
// MessageObserver receives BGP messages.
// Implement this in addition to Plugin for message-aware plugins.
type MessageObserver interface {
    Plugin

    // OnMessage receives all BGP messages for all peers.
    // Use msg.Direction to filter sent vs received.
    // Use peer.RecvContext()/SendContext() for encoding info.
    OnMessage(peer *Peer, msg RawMessage)
}
```

### Registry

```go
// Registry manages plugin lifecycle.
type Registry struct {
    plugins []Plugin // Ordered by dependency
    mu      sync.RWMutex
}

// Register adds a plugin. Call before Init.
func (r *Registry) Register(p Plugin) error

// Init initializes all plugins in dependency order.
func (r *Registry) Init(ctx context.Context, reactor *Reactor) error

// Close closes all plugins in reverse order.
func (r *Registry) Close() error

// PeerObservers returns plugins implementing PeerObserver.
func (r *Registry) PeerObservers() []PeerObserver

// MessageObservers returns plugins implementing MessageObserver.
func (r *Registry) MessageObservers() []MessageObserver
```

---

## Phase 1: Built-in Plugins

### RIB Plugin

Wraps existing `reactor.ribIn`, `reactor.ribOut`, `reactor.ribStore`:

```go
type RIBPlugin struct {
    ribIn    *rib.IncomingRIB
    ribOut   *rib.OutgoingRIB
    ribStore *rib.RouteStore
}

func (p *RIBPlugin) Name() string         { return "rib" }
func (p *RIBPlugin) Version() string      { return "1.0.0" }
func (p *RIBPlugin) Dependencies() []string { return nil }

func (p *RIBPlugin) Init(ctx context.Context, reactor *Reactor) error {
    // Take ownership of reactor's RIB components
    p.ribIn = reactor.ribIn
    p.ribOut = reactor.ribOut
    p.ribStore = reactor.ribStore
    return nil
}

func (p *RIBPlugin) Close() error { return nil }

// PeerObserver implementation
func (p *RIBPlugin) OnEstablished(peer *Peer) {
    // Initialize per-peer RIB state if needed
}

func (p *RIBPlugin) OnClose(peer *Peer, reason string) {
    // Clear peer's routes from RIB-In
    p.ribIn.RemovePeer(peer.Settings().Address)
}

// MessageObserver implementation
func (p *RIBPlugin) OnMessage(peer *Peer, msg RawMessage) {
    if msg.Type != message.TypeUPDATE || msg.Direction != "received" {
        return
    }
    // Store in RIB-In (lazy parsing via AttrsWire)
    // ...
}
```

### ExaBGP API Plugin

Wraps existing `reactor.api`:

```go
type APIPlugin struct {
    server *api.Server
}

func (p *APIPlugin) Name() string         { return "exabgp-api" }
func (p *APIPlugin) Version() string      { return "1.0.0" }
func (p *APIPlugin) Dependencies() []string { return nil }

func (p *APIPlugin) Init(ctx context.Context, reactor *Reactor) error {
    p.server = reactor.api
    return nil
}

func (p *APIPlugin) Close() error { return nil }

// PeerObserver - emit state messages
func (p *APIPlugin) OnEstablished(peer *Peer) {
    p.server.EmitStateChange(peer.Settings().Address, "established")
}

func (p *APIPlugin) OnClose(peer *Peer, reason string) {
    p.server.EmitStateChange(peer.Settings().Address, "down")
}

// MessageObserver - forward to processes
func (p *APIPlugin) OnMessage(peer *Peer, msg RawMessage) {
    p.server.ForwardMessage(peer, msg)
}
```

---

## Preserved Patterns

### Encoding Contexts

Plugins access via Peer methods:

```go
// Zero-copy forwarding check
if srcPeer.SendContextID() == dstPeer.RecvContextID() {
    // Contexts match - can forward raw bytes
    dstPeer.SendRawUpdateBody(msg.RawBytes)
} else {
    // Must re-encode
    // ...
}
```

### Transactions

Plugins access via Peer's AdjRIBOut:

```go
func (p *MyPlugin) OnMessage(peer *Peer, msg RawMessage) {
    adjRIB := peer.AdjRIBOut()

    if adjRIB.InTransaction() {
        // Queue for commit
        adjRIB.QueueAnnounce(route)
    } else {
        // Send immediately
        peer.SendUpdate(update)
        adjRIB.MarkSent(route)
    }
}
```

### Watchdog

Plugins access via Reactor's WatchdogManager:

```go
func (p *MyPlugin) Init(ctx context.Context, reactor *Reactor) error {
    wm := reactor.WatchdogManager()

    // Create pool
    wm.CreatePool("my-routes")

    // Add route to pool
    wm.AddRoute("my-routes", route)

    return nil
}
```

### RawMessage with Lazy Parsing

```go
func (p *MyPlugin) OnMessage(peer *Peer, msg RawMessage) {
    if msg.Type != message.TypeUPDATE {
        return
    }

    // Option 1: Use lazy-parsed attributes
    if msg.AttrsWire != nil {
        origin := msg.AttrsWire.Origin()
        asPath := msg.AttrsWire.ASPath()
    }

    // Option 2: Full parse if needed
    update, err := message.ParseUpdate(msg.RawBytes, peer.RecvContext())
    if err != nil {
        return
    }
}
```

---

## Integration Points

### Reactor Changes

```go
type Reactor struct {
    // ... existing fields ...

    plugins *plugin.Registry
}

func (r *Reactor) Run(ctx context.Context) error {
    // Initialize plugins
    if err := r.plugins.Init(ctx, r); err != nil {
        return err
    }
    defer r.plugins.Close()

    // ... existing run logic ...
}
```

### Peer Changes

```go
// In Peer.run(), after FSM callback:
if to == fsm.StateEstablished {
    // Notify plugin observers
    for _, obs := range r.plugins.PeerObservers() {
        obs.OnEstablished(p)
    }
}
```

### Session Changes

```go
// In Session.processMessage(), after existing callback:
if s.onMessageReceived != nil {
    s.onMessageReceived(s.settings.Address, hdr.Type, body, direction)
}

// Plugin observers are called via Reactor, not Session
// (Session doesn't know about plugins)
```

---

## Configuration

```toml
[zebgp.plugins]
rib = true          # Enable RIB plugin (default: true)
exabgp-api = true   # Enable ExaBGP API plugin (default: true)

# Future: external plugins
# [zebgp.plugins.external]
# my-plugin = { path = "/usr/lib/zebgp/my-plugin.so" }
```

---

## Implementation Checklist

### Phase 0 (see phase0-peer-callbacks.md)
- [ ] Add PeerLifecycleCallback to Reactor
- [ ] Call from Peer FSM callback
- [ ] Emit API state messages

### Phase 0.5
- [ ] Add MessageObserver interface
- [ ] Reactor.AddMessageObserver()
- [ ] Wire into existing messageCallback flow

### Phase 1
- [ ] Create `pkg/plugin/` package
- [ ] Define Plugin, PeerObserver, MessageObserver interfaces
- [ ] Implement Registry with dependency resolution
- [ ] Create RIBPlugin wrapper
- [ ] Create APIPlugin wrapper
- [ ] Add plugin init to Reactor.Run()
- [ ] Add TOML configuration

---

## Migration Path

1. Phase 0-0.5: No breaking changes, adds callbacks
2. Phase 1: RIB/API become plugins, but behavior unchanged
3. Existing code using `reactor.ribIn` continues to work
4. Plugins are opt-in via config

---

## References

- Current architecture: `pkg/reactor/`
- API types: `pkg/plugin/types.go`
- RIB: `pkg/rib/`
- Encoding contexts: `pkg/bgp/context/`
