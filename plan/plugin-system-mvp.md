# ZeBGP Plugin System - MVP Specification

**Status:** Proposed (derived from plugin-system.md review)
**Date:** 2025-12-22

**Purpose:** Define minimum viable plugin system. Full spec in `plugin-system.md`.

---

## MVP Scope

### In Scope (Phase 1)

| Component | Description |
|-----------|-------------|
| Plugin interface | Base interface for all plugins |
| PeerPlugin interface | Per-peer lifecycle hooks |
| Registry | Plugin loading, dependency resolution |
| RIB plugin | Wrap existing `pkg/rib` |
| ExaBGP API plugin | Wrap existing `pkg/api` |
| Validation layer | Basic RFC compliance checks |
| TOML config | Plugin enable/disable, basic settings |

### Deferred

| Feature | Phase |
|---------|-------|
| gRPC external plugins | 2 |
| Export policy | 2 |
| JSON-RPC transport | 3 |
| Graceful restart handler | 3 |
| Custom capabilities/attributes | 4 |
| State persistence | 4 |
| stdio transport | 4 |

---

## Core Types

```go
package plugin

import (
    "context"
    "log/slog"
    "net/netip"
    "time"

    "github.com/exa-networks/zebgp/pkg/bgp/attribute"
    "github.com/exa-networks/zebgp/pkg/bgp/capability"
    "github.com/exa-networks/zebgp/pkg/bgp/message"
)

// Route represents a BGP route for plugin consumption.
type Route struct {
    Prefix     netip.Prefix
    PathID     uint32              // For ADD-PATH
    NextHop    netip.Addr
    Attributes []attribute.Attribute
    Source     netip.Addr          // Peer that advertised (zero for local)
    Timestamp  time.Time
}

// AddPathMode describes ADD-PATH negotiation result.
type AddPathMode uint8

const (
    AddPathNone AddPathMode = iota
    AddPathReceive
    AddPathSend
    AddPathBoth
)

// PeerInfo describes a peer for plugin consumption.
// This is a COPY - safe to retain and access from any goroutine.
type PeerInfo struct {
    Address      netip.Addr
    LocalAddress netip.Addr
    LocalAS      uint32
    PeerAS       uint32
    RouterID     uint32
    State        string
    Established  time.Time
    Negotiated   NegotiatedParams
    PluginConfig map[string]any // Per-peer plugin config
}

// NegotiatedParams describes negotiated session parameters.
type NegotiatedParams struct {
    FourOctetAS     bool
    ExtendedMessage bool
    MaxMessageSize  int
    AddPath         map[capability.Family]AddPathMode
    Families        []capability.Family
    HoldTime        time.Duration
}
```

---

## Plugin Interface

```go
// Plugin is the base interface all plugins must implement.
type Plugin interface {
    // Name returns unique plugin identifier.
    Name() string

    // Version returns semantic version (e.g., "1.0.0").
    Version() string

    // Dependencies returns plugins this one requires.
    // Returns nil if no dependencies.
    Dependencies() []string

    // Init is called once during startup.
    // Dependencies are guaranteed initialized before this is called.
    Init(ctx context.Context, reactor ReactorAPI, config map[string]any) error

    // Close is called during shutdown (reverse dependency order).
    // Has 5s timeout by default.
    Close() error
}
```

---

## PeerPlugin Interface

```go
// PeerPlugin handles per-peer lifecycle events.
//
// THREAD SAFETY: All methods called from single goroutine per peer.
// Do NOT block for extended periods.
type PeerPlugin interface {
    Plugin

    // GetCapabilities returns capabilities to advertise in OPEN.
    GetCapabilities(peer PeerInfo) []capability.Capability

    // OnOpenReceived is called when peer's OPEN is received.
    // Return non-nil Notification to reject session.
    OnOpenReceived(peer PeerInfo, caps []capability.Capability) *message.Notification

    // OnEstablished is called when session reaches Established.
    // Returns handler for incoming UPDATEs.
    OnEstablished(peer PeerInfo, writer UpdateWriter) UpdateHandler

    // OnClose is called when session leaves Established.
    // Called exactly once per Established session.
    OnClose(peer PeerInfo)
}
```

---

## UpdateHandler

```go
// UpdateHandler processes incoming UPDATE messages.
//
// WARNING: update is pooled. Do NOT retain after handler returns.
// Use update.Clone() for async processing.
type UpdateHandler func(peer PeerInfo, update *message.Update) HandlerResult

// HandlerResult specifies handler outcome.
type HandlerResult struct {
    Action       HandlerAction
    Notification *message.Notification // Only for ActionCloseSession
}

type HandlerAction int

const (
    ActionContinue     HandlerAction = iota // Pass to next handler
    ActionDrop                              // Silently discard UPDATE
    ActionCloseSession                      // Send notification, close session
)

// Convenience constructors
func Continue() HandlerResult { return HandlerResult{Action: ActionContinue} }
func Drop() HandlerResult     { return HandlerResult{Action: ActionDrop} }
func CloseSession(n *message.Notification) HandlerResult {
    return HandlerResult{Action: ActionCloseSession, Notification: n}
}
```

---

## UpdateWriter

```go
// UpdateWriter sends UPDATE messages to a peer.
//
// LIFECYCLE: Valid from OnEstablished until OnClose.
// THREAD SAFETY: All methods safe for concurrent use.
type UpdateWriter interface {
    // WriteUpdate sends an UPDATE message.
    // Passes through validation layer before wire.
    WriteUpdate(update *message.Update) error

    // Closed returns true if session has ended.
    Closed() bool

    // Negotiated returns negotiated session parameters.
    Negotiated() NegotiatedParams

    // Peer returns peer info for this writer.
    Peer() PeerInfo
}

var ErrSessionClosed = errors.New("session closed")
```

---

## ReactorAPI

```go
// ReactorAPI provides plugin access to reactor functionality.
//
// THREAD SAFETY: All methods safe for concurrent use.
type ReactorAPI interface {
    // Peer access
    Peers() []PeerInfo
    GetPeer(addr netip.Addr) (PeerInfo, bool)

    // Peer control
    TeardownPeer(addr netip.Addr, reason string) error

    // Service registry
    RegisterService(name string, service any) error
    GetService(name string) (any, bool)

    // Logging (scoped to plugin name)
    Logger() *slog.Logger
}
```

---

## Registry

```go
// Registry manages plugin lifecycle.
type Registry struct {
    plugins map[string]Plugin
    order   []Plugin // Initialization order
    mu      sync.RWMutex
}

// Register adds a plugin.
func (r *Registry) Register(p Plugin) error

// Get returns a plugin by name.
func (r *Registry) Get(name string) (Plugin, bool)

// InitAll initializes plugins in dependency order.
func (r *Registry) InitAll(ctx context.Context, reactor ReactorAPI) error

// CloseAll closes plugins in reverse order.
func (r *Registry) CloseAll() error

// PeerPlugins returns all PeerPlugin implementations.
func (r *Registry) PeerPlugins() []PeerPlugin
```

### Dependency Resolution (Fixed)

```go
// resolveDependencies returns plugins in initialization order.
// Dependencies are initialized BEFORE dependents.
func (r *Registry) resolveDependencies() ([]Plugin, error) {
    // Build adjacency list: plugin -> its dependencies
    deps := make(map[string][]string)
    for name, p := range r.plugins {
        deps[name] = p.Dependencies()
    }

    // Detect cycles using DFS
    if cycle := detectCycle(deps); cycle != nil {
        return nil, fmt.Errorf("dependency cycle: %s", strings.Join(cycle, " → "))
    }

    // Topological sort (Kahn's algorithm)
    // inDegree[X] = number of plugins that X depends on
    inDegree := make(map[string]int)
    dependents := make(map[string][]string) // dep -> plugins that depend on it

    for name := range r.plugins {
        inDegree[name] = len(deps[name])
        for _, dep := range deps[name] {
            dependents[dep] = append(dependents[dep], name)
        }
    }

    // Start with plugins that have no dependencies
    var queue []string
    for name, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, name)
        }
    }

    var result []Plugin
    for len(queue) > 0 {
        // Sort queue for deterministic order
        sort.Strings(queue)
        name := queue[0]
        queue = queue[1:]

        result = append(result, r.plugins[name])

        // Decrement inDegree of dependents
        for _, dependent := range dependents[name] {
            inDegree[dependent]--
            if inDegree[dependent] == 0 {
                queue = append(queue, dependent)
            }
        }
    }

    // Check for unresolved dependencies
    if len(result) != len(r.plugins) {
        var missing []string
        for name, degree := range inDegree {
            if degree > 0 {
                missing = append(missing, name)
            }
        }
        return nil, fmt.Errorf("unresolved dependencies: %v", missing)
    }

    return result, nil
}

func detectCycle(deps map[string][]string) []string {
    const (
        unvisited = 0
        visiting  = 1
        visited   = 2
    )

    state := make(map[string]int)
    parent := make(map[string]string)

    var dfs func(node string) []string
    dfs = func(node string) []string {
        if state[node] == visited {
            return nil
        }
        if state[node] == visiting {
            // Found cycle - reconstruct path
            cycle := []string{node}
            for cur := parent[node]; cur != node; cur = parent[cur] {
                cycle = append([]string{cur}, cycle...)
            }
            return append(cycle, node)
        }

        state[node] = visiting
        for _, dep := range deps[node] {
            parent[dep] = node
            if cycle := dfs(dep); cycle != nil {
                return cycle
            }
        }
        state[node] = visited
        return nil
    }

    for node := range deps {
        if cycle := dfs(node); cycle != nil {
            return cycle
        }
    }
    return nil
}
```

---

## Validation Layer

```go
// Validator checks RFC compliance before wire encoding.
type Validator struct {
    mode ValidatorMode
}

type ValidatorMode int

const (
    ValidatorEnforce ValidatorMode = iota // Reject violations
    ValidatorWarn                         // Log and allow
    ValidatorDisable                      // Skip validation
)

// ValidateUpdate checks UPDATE for RFC 4271 compliance.
func (v *Validator) ValidateUpdate(u *message.Update, neg NegotiatedParams) error {
    // Check mandatory attributes for announcements
    if len(u.NLRI) > 0 || u.MPReachNLRI != nil {
        if !hasAttribute(u.Attributes, attribute.AttrOrigin) {
            return fmt.Errorf("missing ORIGIN attribute")
        }
        if !hasAttribute(u.Attributes, attribute.AttrASPath) {
            return fmt.Errorf("missing AS_PATH attribute")
        }
        // NEXT_HOP required for IPv4 unicast NLRI
        if len(u.NLRI) > 0 && !hasAttribute(u.Attributes, attribute.AttrNextHop) {
            return fmt.Errorf("missing NEXT_HOP attribute")
        }
    }

    // Check message size
    size := u.Len()
    if size > neg.MaxMessageSize {
        return fmt.Errorf("UPDATE size %d exceeds max %d", size, neg.MaxMessageSize)
    }

    return nil
}

func hasAttribute(attrs []attribute.Attribute, code attribute.Code) bool {
    for _, a := range attrs {
        if a.Code() == code {
            return true
        }
    }
    return false
}
```

---

## Clone Support

Add to `pkg/bgp/message/update.go`:

```go
// Clone creates a deep copy of the UPDATE message.
// Use when retaining UPDATE beyond handler return (e.g., async processing).
func (u *Update) Clone() *Update {
    if u == nil {
        return nil
    }

    clone := &Update{
        PathID: u.PathID,
    }

    // Deep copy slices
    if len(u.Withdrawn) > 0 {
        clone.Withdrawn = make([]netip.Prefix, len(u.Withdrawn))
        copy(clone.Withdrawn, u.Withdrawn)
    }

    if len(u.NLRI) > 0 {
        clone.NLRI = make([]netip.Prefix, len(u.NLRI))
        copy(clone.NLRI, u.NLRI)
    }

    // Deep copy attributes
    if len(u.Attributes) > 0 {
        clone.Attributes = make([]attribute.Attribute, len(u.Attributes))
        for i, attr := range u.Attributes {
            clone.Attributes[i] = attr.Clone()
        }
    }

    // Deep copy MP-NLRI
    if u.MPReachNLRI != nil {
        clone.MPReachNLRI = u.MPReachNLRI.Clone()
    }
    if u.MPUnreachNLRI != nil {
        clone.MPUnreachNLRI = u.MPUnreachNLRI.Clone()
    }

    return clone
}
```

---

## Configuration

```toml
# etc/zebgp/zebgp.toml

[zebgp.plugin]
rib = true          # Enable RIB plugin (default: true)
exabgp-api = true   # Enable ExaBGP API plugin (default: true)

[zebgp.validation]
mode = "enforce"    # enforce, warn, disable
```

---

## Built-in Plugins

### RIB Plugin

```go
type RIBPlugin struct {
    ribIn  *rib.Table
    ribOut *rib.Table
    log    *slog.Logger
}

func (p *RIBPlugin) Name() string         { return "rib" }
func (p *RIBPlugin) Version() string      { return "1.0.0" }
func (p *RIBPlugin) Dependencies() []string { return nil }

func (p *RIBPlugin) Init(ctx context.Context, reactor ReactorAPI, cfg map[string]any) error {
    p.log = reactor.Logger()
    p.ribIn = rib.NewTable()
    p.ribOut = rib.NewTable()
    reactor.RegisterService("rib.in", p.ribIn)
    reactor.RegisterService("rib.out", p.ribOut)
    return nil
}

func (p *RIBPlugin) Close() error { return nil }

func (p *RIBPlugin) GetCapabilities(peer PeerInfo) []capability.Capability {
    return nil // RIB doesn't add capabilities
}

func (p *RIBPlugin) OnOpenReceived(peer PeerInfo, caps []capability.Capability) *message.Notification {
    return nil // Accept all
}

func (p *RIBPlugin) OnEstablished(peer PeerInfo, writer UpdateWriter) UpdateHandler {
    return func(peer PeerInfo, update *message.Update) HandlerResult {
        // Store in Adj-RIB-In
        p.ribIn.ProcessUpdate(peer.Address, update)
        return Continue()
    }
}

func (p *RIBPlugin) OnClose(peer PeerInfo) {
    // Remove peer's routes from RIB
    p.ribIn.RemovePeer(peer.Address)
}
```

### ExaBGP API Plugin

```go
type ExaBGPAPIPlugin struct {
    server *api.Server
    log    *slog.Logger
}

func (p *ExaBGPAPIPlugin) Name() string         { return "exabgp-api" }
func (p *ExaBGPAPIPlugin) Version() string      { return "1.0.0" }
func (p *ExaBGPAPIPlugin) Dependencies() []string { return nil }

func (p *ExaBGPAPIPlugin) Init(ctx context.Context, reactor ReactorAPI, cfg map[string]any) error {
    p.log = reactor.Logger()
    socketPath := "/var/run/zebgp.sock"
    if path, ok := cfg["socket"].(string); ok {
        socketPath = path
    }
    p.server = api.NewServer(socketPath)
    return p.server.Start()
}

func (p *ExaBGPAPIPlugin) Close() error {
    return p.server.Stop()
}

// ExaBGP API doesn't need PeerPlugin hooks - it uses ReactorAPI
```

---

## Implementation Checklist

### Phase 1 Tasks

- [ ] Create `pkg/plugin/` package
- [ ] Define core interfaces (Plugin, PeerPlugin, UpdateHandler, etc.)
- [ ] Define types (Route, PeerInfo, NegotiatedParams, etc.)
- [ ] Implement Registry with dependency resolution
- [ ] Implement Validator (basic)
- [ ] Add Clone() to message.Update
- [ ] Add Clone() to attribute types
- [ ] Wrap existing RIB as RIBPlugin
- [ ] Wrap existing API as ExaBGPAPIPlugin
- [ ] Integrate plugin system into reactor
- [ ] Add TOML configuration
- [ ] Write tests for:
  - [ ] Dependency cycle detection
  - [ ] Topological ordering
  - [ ] Handler chaining
  - [ ] Validation layer

### Tests Required

```go
func TestDependencyCycle(t *testing.T)
func TestTopologicalOrder(t *testing.T)
func TestHandlerChain(t *testing.T)
func TestValidatorEnforce(t *testing.T)
func TestUpdateClone(t *testing.T)
```

---

## Migration Path

1. **No breaking changes** - existing code continues to work
2. Plugin system wraps existing `pkg/api` and `pkg/rib`
3. Reactor delegates to plugin registry for peer events
4. Future: external plugins via gRPC (Phase 2)

---

## References

- Full spec: `plan/plugin-system.md`
- RFC 4271: BGP-4
- RFC 8654: Extended Message
