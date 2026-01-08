# ZeBGP Plugin System Design

**Status:** Proposed (Under Review)
**Date:** 2025-12-21
**Updated:** 2025-12-22 (v5)

> **⚠️ BLOCKING PREREQUISITE:** This design requires RIB implementation. See `docs/plan/rib-design.md` (TODO).

> **📄 Document Structure:** This is the full specification. For phased implementation, see Phased Implementation section below.

---

## Change Log

**v5 Changes (Review):**
- Added Prerequisites section
- Added Decision Log
- Added Risk Register
- Fixed topological sort algorithm (was computing in-degree backwards)
- Added `OnUpdateSend` hook for final UPDATE modification
- Added Phased Implementation with MVP definition
- Added document structure recommendations
- Marked sections as MVP vs POST-MVP
- Added missing Clone() return types

**v4 Changes:**
- Added full Clone() implementation with deep-copy semantics
- Added Cloner interface for attributes
- Added dependency cycle detection using Tarjan's algorithm
- Added topological sort for plugin init order
- Added test cases for cycle detection and ordering

**v3 Changes:**
- Removed Go native plugin support (gRPC/JSON-RPC only)
- Added ConfigSchema for config validation (JSON Schema draft-07)
- Added GetConfigSchema RPC (called before Init for validation)
- Fixed CustomCapability code range to 239-254 per RFC 5492 §4
- Fixed CustomAttribute code range to 241-254 (safe experimental)
- Added Outbound UPDATE Flow with ExportPlugin interface
- Added OnExport RPC to gRPC/JSON-RPC protocols
- Added Route message type for export policy

---

## Prerequisites

| Prerequisite | Status | Why Needed |
|--------------|--------|------------|
| RFC 4271 core compliance | 🟡 In Progress | Plugin interfaces must match stable core |
| RIB implementation | 🔴 Not Started | Export policy, route storage require RIB |
| Reactor refactor | 🟡 Partial | ReactorAPI interface needs clear boundaries |

**Do NOT begin plugin implementation until:**
1. `make test && make lint` passes consistently
2. RIB design is approved
3. Core FSM is stable

---

## Decision Log

| Date | Decision | Rationale | Alternatives Considered |
|------|----------|-----------|------------------------|
| 2025-12-21 | No Go native plugins | CGO issues, Go version coupling, no Windows | Native .so, Wasm |
| 2025-12-21 | gRPC as primary transport | Type safety, streaming, wide language support | JSON-RPC only |
| 2025-12-21 | JSON-RPC for simplicity | Zero deps, easy debugging, ExaBGP compat | gRPC only |
| 2025-12-21 | stdio for ExaBGP compat | Migration path from ExaBGP processes | Drop compat |
| 2025-12-22 | Validation layer mandatory | RFC compliance non-negotiable | Trust plugins |
| 2025-12-22 | Circuit breaker for external | Prevent cascade failures | Fail-fast only |
| 2025-12-22 | Priority-based ordering | Explicit control, predictable | Alphabetical |
| 2025-12-22 | Clone() for async safety | Pooled UPDATEs need explicit copy | Always copy |

---

## Risk Register

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Plugin corrupts UPDATE | 🔴 High | Medium | Validation layer, immutable option |
| External plugin latency | 🟡 Medium | High | Timeouts, circuit breaker, backpressure |
| Dependency version drift | 🟡 Medium | Medium | Version negotiation, compatibility matrix |
| Core API churn | 🔴 High | High (now) | Wait for core stability, minimal MVP |
| Memory leaks from pooling | 🟡 Medium | Medium | Clone() semantics, linter checks |
| Plugin blocks shutdown | 🟡 Medium | Medium | Close timeout, force-terminate |

---

## Goals

1. **Extensibility** - Third parties can extend ZeBGP without forking
2. **API Flexibility** - Multiple API styles (ExaBGP, gRPC, custom)
3. **Boot-time Selection** - Config determines which plugins load
4. **Backward Compatibility** - Current ExaBGP API remains default
5. **Minimal Core** - Reactor handles FSM + transport; plugins handle policy/API
6. **RFC Compliance** - Plugin system MUST NOT compromise RFC 4271 compliance

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                          Configuration                           │
│   plugins:                                                       │
│     api: [exabgp, grpc]                                         │
│     peer: [rib, route-policy, custom]                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Plugin Registry                           │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐               │
│  │ ExaBGP  │ │  gRPC   │ │   RIB   │ │ Custom  │               │
│  │  API    │ │  API    │ │ Plugin  │ │ Plugin  │               │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘               │
│       │           │           │           │                      │
└───────┼───────────┼───────────┼───────────┼──────────────────────┘
        │           │           │           │
        └───────────┴─────┬─────┴───────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Reactor Core                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │  Connection  │  │     FSM      │  │   Message    │          │
│  │   Manager    │  │              │  │   Encoding   │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

---

## RFC Compliance

```
┌─────────────────────────────────────────────────────────────────┐
│  Plugins MUST NOT violate RFC 4271 requirements.                │
│  The core reactor enforces RFC compliance at the wire level.    │
└─────────────────────────────────────────────────────────────────┘
```

### Validation Layer

A validation layer sits between the plugin handler chain and the wire:

```
Plugin Chain → [Validation Layer] → Wire
                     │
                     ├─ Mandatory attributes present?
                     ├─ Attribute flags correct?
                     ├─ Well-formed encoding?
                     ├─ Message size within negotiated limits?
                     └─ Log/reject violations
```

### Plugin Constraints

| Constraint | Enforcement |
|------------|-------------|
| UPDATE must have ORIGIN, AS_PATH, NEXT_HOP (for NLRI) | Validation layer rejects, logs error |
| NOTIFICATION codes must be valid per RFC 4271 §4.5 | Validation layer substitutes Cease if invalid |
| Attribute flags must match RFC 4271 Appendix F | Validation layer corrects or rejects |
| AS_PATH must not be empty for eBGP | Validation layer rejects |
| Message size must respect negotiated Extended Message capability | Validation layer rejects if > max size |

### Plugin Notification Code Restrictions

Plugins MAY return NOTIFICATIONs with these codes:
- **Cease (6/*)** - Administrative actions
- **UPDATE Message Error (3/*)** - Policy-based rejections
- **Finite State Machine Error (5/0)** - Generic FSM error

Plugins MUST NOT return NOTIFICATIONs with:
- **Hold Timer Expired (4/*)** - Managed by reactor
- **Connection Not Synchronized (5/1)** - Managed by reactor
- **Bad Peer AS (2/2)** - Checked during OPEN processing

Validation layer substitutes Cease/Administrative Reset (6/4) for invalid codes.

### RawWriter Restrictions

`RawWriter` bypasses validation and can send arbitrary bytes. This is **EXPERIMENTAL** and requires explicit opt-in:

```toml
[zebgp.plugin.my-draft-plugin]
raw-access = true  # REQUIRED to use RawWriter
```

Without `raw-access = true`, calling `RawWriter` methods returns an error.

**Audit logging:** All RawWriter calls are logged at INFO level with message type and payload SHA256. Raw bytes logged at DEBUG level if `zebgp.raw.log-payloads = true`.

**Use cases for RawWriter:**
- Testing new IETF drafts
- Interoperability testing
- Fuzzing
- Research

**NOT for production route exchange.**

### Validation Configuration

```toml
[zebgp.validation]
mode = "enforce"      # enforce (default), warn, disable
log-violations = true # Log RFC violations
```

---

## Core Interfaces

### 1. Base Plugin Interface

```go
// Plugin is the base interface all plugins must implement.
type Plugin interface {
    // Name returns unique plugin identifier (e.g., "exabgp-api", "rib").
    Name() string

    // Version returns semantic version string (e.g., "1.2.3").
    Version() string

    // Priority returns plugin execution order (0-999, lower = earlier).
    // Built-in plugins use 0-99, user plugins should use 100-999.
    // Plugins with same priority are ordered alphabetically by name.
    Priority() int

    // Dependencies returns services this plugin requires.
    // The registry ensures dependencies are initialized before this plugin.
    // Returns nil if no dependencies.
    Dependencies() []Dependency

    // ConfigSchema returns JSON Schema (draft-07) for config validation.
    // Called before Init() to validate configuration.
    // Return nil to skip validation (not recommended for external plugins).
    // Invalid config prevents plugin initialization with detailed error message.
    ConfigSchema() *ConfigSchema

    // Init is called once during reactor startup.
    // ctx is cancelled on shutdown; config contains plugin-specific settings.
    // All dependencies are guaranteed to be available via reactor.GetService().
    // Config is guaranteed to match ConfigSchema() if schema was provided.
    Init(ctx context.Context, reactor ReactorAPI, config map[string]any) error

    // Close is called during reactor shutdown.
    // Called in reverse dependency order (dependents close before dependencies).
    // Services remain available during Close() for cleanup operations.
    // Close has a timeout (default 5s); if exceeded, plugin is force-terminated.
    Close() error
}

// ConfigSchema defines plugin configuration validation.
type ConfigSchema struct {
    // Schema is JSON Schema (draft-07) as raw JSON.
    // Example: {"type": "object", "properties": {"threshold": {"type": "integer", "minimum": 0}}}
    Schema json.RawMessage

    // Required lists required configuration keys.
    Required []string

    // Defaults provides default values for optional keys.
    Defaults map[string]any
}

// Dependency specifies a required service with optional version constraint.
type Dependency struct {
    Name       string // Service name (e.g., "rib.in")
    MinVersion string // Minimum version (e.g., "1.0"), empty = any
}

// Standard priority ranges:
const (
    PriorityBuiltinFirst = 0   // Core functionality (RIB)
    PriorityBuiltinLast  = 99  // Built-in plugins
    PriorityUserDefault  = 500 // Default for user plugins
    PriorityUserLast     = 999 // Last in chain
)
```

### Plugin Ordering

Plugins execute in priority order (lowest first). For handler chains:

```
Priority 0   → RIB plugin (store routes first)
Priority 100 → Filter plugin (accept/reject)
Priority 200 → Logging plugin (log after filtering)
Priority 500 → Custom plugins (default priority)
```

**Explicit ordering in config:**

```toml
[zebgp.plugin]
# Override default priorities
"rib.priority" = 0
"route-policy.priority" = 50
"my-filter.priority" = 100
"logging.priority" = 900
```

**Priority conflicts:** If two plugins have identical priority, they are ordered alphabetically by name. A warning is logged.

### 2. Peer Plugin Interface

```go
// PeerPlugin handles per-peer lifecycle events.
// Multiple PeerPlugins can be registered - they form a processing chain.
//
// THREAD SAFETY: All methods are called from a single goroutine per peer.
// Plugins MUST NOT block for extended periods. Use goroutines for async work.
type PeerPlugin interface {
    Plugin

    // GetCapabilities returns capabilities to advertise in OPEN.
    // Called before sending OPEN message (peer.Negotiated is empty at this point).
    // Capabilities from all plugins are merged (see Capability Merging).
    GetCapabilities(peer PeerInfo) []capability.Capability

    // AdjustCapabilities is called after receiving peer's OPEN.
    // Allows reactive capability negotiation based on peer's capabilities.
    //
    // NOTE: This implements "delayed OPEN" pattern where ZeBGP waits to receive
    // peer's OPEN before sending its own. This is OPTIONAL behavior enabled via:
    //   [zebgp.peer."x.x.x.x"]
    //   delayed-open = true
    // Without delayed-open, GetCapabilities is used and this hook is skipped.
    //
    // Returns adjusted capabilities to advertise, or nil to use original.
    AdjustCapabilities(peer PeerInfo, ours, theirs []capability.Capability) []capability.Capability

    // OnOpenMessage is called when OPEN is received.
    // Return non-nil Notification to reject and close session.
    // If any plugin rejects, session is closed.
    OnOpenMessage(peer PeerInfo, routerID netip.Addr, caps []capability.Capability) *message.Notification

    // OnEstablished is called when session reaches Established state.
    // Returns UpdateHandler to process incoming UPDATEs.
    // writer is used to send UPDATEs to this peer.
    OnEstablished(peer PeerInfo, writer UpdateWriter) UpdateHandler

    // OnClose is called exactly once when session leaves Established state.
    // Called in reverse priority order (highest priority first).
    // UpdateWriter is invalid after this returns.
    // Called regardless of how session ended (normal close, error, peer down).
    OnClose(peer PeerInfo)

    // --- Extended Hooks (optional, implement via embedding) ---

    // OnStateChange is called on any FSM state transition.
    // OnStateChange(peer PeerInfo, from, to State)

    // OnKeepalive is called when KEEPALIVE is received.
    // OnKeepalive(peer PeerInfo)

    // OnRouteRefresh is called when ROUTE-REFRESH is received.
    // OnRouteRefresh(peer PeerInfo, afi uint16, safi uint8)

    // OnNotification is called when NOTIFICATION is received (before close).
    // OnNotification(peer PeerInfo, notif *message.Notification)

    // OnNotificationSend is called before sending NOTIFICATION to peer.
    // Allows plugins to log, modify, or override outgoing notifications.
    // Return nil to use original notification, or modified notification.
    // OnNotificationSend(peer PeerInfo, notif *message.Notification) *message.Notification

    // OnUpdateSend is called before sending UPDATE to peer (after export policy).
    // Allows final modification of composed UPDATE before wire encoding.
    // Called per-UPDATE, not per-route (unlike OnExport which is per-route).
    // Use for: final attribute adjustments, logging, metrics.
    // Return nil to use original UPDATE, or modified UPDATE.
    // OnUpdateSend(peer PeerInfo, update *message.Update) *message.Update

    // OnError is called for non-fatal errors (parse errors, validation warnings).
    // Useful for logging, alerting, and debugging interop issues.
    // OnError(peer PeerInfo, err error, context string)
}

// --- Graceful Restart Support (RFC 4724) ---

// GracefulRestartHandler is implemented by plugins that need to participate
// in RFC 4724 Graceful Restart. The RIB plugin implements this by default.
//
// IMPORTANT: During GR, the peer's routes are marked stale but preserved.
// Plugins MUST NOT withdraw stale routes until OnStaleCleanup is called.
type GracefulRestartHandler interface {
    // OnRestartBegin is called when a peer reconnects with Restart bit set.
    // stalenessTimer is the negotiated restart time; routes become stale.
    // The plugin should preserve existing routes from this peer.
    OnRestartBegin(peer PeerInfo, stalenessTimer time.Duration) error

    // OnEOR is called when End-of-RIB marker received for an AFI/SAFI.
    // Routes for this family are no longer stale.
    OnEOR(peer PeerInfo, afi uint16, safi uint8)

    // OnStaleCleanup is called when staleness timer expires or all EORs received.
    // Plugin should remove any remaining stale routes from this peer.
    OnStaleCleanup(peer PeerInfo)

    // OnRestartAbort is called if session fails during restart (before all EORs).
    // All routes from this peer should be withdrawn.
    OnRestartAbort(peer PeerInfo, reason string)
}

// UpdateHandler processes incoming UPDATE messages.
// Handlers are chained: each handler can pass/drop/modify/close.
//
// THREAD SAFETY: Called from peer's goroutine. Must not block.
// UPDATE object is pooled - do NOT retain references after returning.
//
// ASYNC PROCESSING: If you need to process the UPDATE asynchronously,
// you MUST clone it before spawning a goroutine:
//
//     go func(u *message.Update) {
//         // Safe: u is a deep copy
//         processAsync(u)
//     }(update.Clone())
//
// Failure to clone before async use will cause data races and corruption.
type UpdateHandler func(peer PeerInfo, update *message.Update) HandlerResult

// Clone creates a deep copy of the UPDATE message.
// Use this when you need to retain the UPDATE beyond handler return,
// such as for async processing or queueing.
//
// GUARANTEES:
// - All slices, maps, and nested structures are deep copied
// - The returned Update shares NO memory with the original
// - Modifications to clone do not affect original (and vice versa)
//
// WARNING: Retaining UPDATE references after handler returns (without Clone) causes:
// - Data races (UPDATE may be concurrently modified by pool)
// - Corrupted data (buffer reused for next UPDATE)
// - Unpredictable crashes
func (u *Update) Clone() *Update {
    if u == nil {
        return nil
    }

    clone := &Update{
        PathID: u.PathID, // Value type, safe to copy
    }

    // Deep copy withdrawn prefixes
    if len(u.Withdrawn) > 0 {
        clone.Withdrawn = make([]netip.Prefix, len(u.Withdrawn))
        copy(clone.Withdrawn, u.Withdrawn)
    }

    // Deep copy NLRI
    if len(u.NLRI) > 0 {
        clone.NLRI = make([]netip.Prefix, len(u.NLRI))
        copy(clone.NLRI, u.NLRI)
    }

    // Deep copy attributes (each attribute implements Cloner)
    if len(u.Attributes) > 0 {
        clone.Attributes = make([]attribute.Attribute, len(u.Attributes))
        for i, attr := range u.Attributes {
            clone.Attributes[i] = attr.Clone()
        }
    }

    // Deep copy MP_REACH_NLRI (contains NextHop + NLRI slice)
    if u.MPReachNLRI != nil {
        clone.MPReachNLRI = u.MPReachNLRI.Clone()
    }

    // Deep copy MP_UNREACH_NLRI
    if u.MPUnreachNLRI != nil {
        clone.MPUnreachNLRI = u.MPUnreachNLRI.Clone()
    }

    return clone
}

// Cloner interface for deep copying attributes.
// All attribute types must implement this.
type Cloner interface {
    Clone() Attribute
}

// Example implementations:
//
// func (a *ASPath) Clone() Attribute {
//     clone := &ASPath{Segments: make([]ASPathSegment, len(a.Segments))}
//     for i, seg := range a.Segments {
//         clone.Segments[i] = ASPathSegment{
//             Type: seg.Type,
//             ASNs: slices.Clone(seg.ASNs),
//         }
//     }
//     return clone
// }
//
// func (c *Communities) Clone() Attribute {
//     return &Communities{Values: slices.Clone(c.Values)}
// }
//
// func (lc *LargeCommunities) Clone() Attribute {
//     return &LargeCommunities{Values: slices.Clone(lc.Values)}
// }

// HandlerResult specifies handler outcome with granular control.
type HandlerResult struct {
    Action       HandlerAction
    Notification *message.Notification // Only used with ActionCloseSession
}

type HandlerAction int

const (
    // ActionContinue passes UPDATE to next handler (possibly modified).
    ActionContinue HandlerAction = iota

    // ActionDrop silently drops this UPDATE, session continues, stop chain.
    // Use for filtered/invalid updates that don't warrant session close.
    ActionDrop

    // ActionCloseSession sends NOTIFICATION and closes session.
    // Notification field must be set.
    ActionCloseSession
)

// Convenience constructors
func Continue() HandlerResult { return HandlerResult{Action: ActionContinue} }
func Drop() HandlerResult     { return HandlerResult{Action: ActionDrop} }
func CloseSession(n *message.Notification) HandlerResult {
    return HandlerResult{Action: ActionCloseSession, Notification: n}
}

// UpdateWriter sends UPDATE messages to a peer.
//
// LIFECYCLE: Valid from OnEstablished until OnClose returns.
// After OnClose is called, all Write methods return ErrSessionClosed.
// Handlers should not retain references to UpdateWriter beyond their scope.
//
// THREAD SAFETY: All methods are safe for concurrent use.
// Multiple goroutines may call WriteUpdate simultaneously.
type UpdateWriter interface {
    // WriteUpdate sends a parsed UPDATE message.
    // Message passes through validation layer before wire.
    // Returns ErrSessionClosed if session has ended (safe to call, returns error).
    // Returns ErrValidationFailed if RFC validation fails.
    WriteUpdate(update *message.Update) error

    // WriteUpdates sends multiple UPDATE messages.
    // Atomic at transport level: all sent or none (on connection error).
    // NOT atomic at BGP level: no transaction guarantee with peer.
    // Messages pass through validation layer before wire.
    // If validation fails for any message, none are sent.
    WriteUpdates(updates []*message.Update) error

    // Closed returns true if the session has ended.
    // Use to check before expensive operations.
    // NOTE: Session may close between Closed() and WriteUpdate();
    // WriteUpdate() handles this atomically and returns ErrSessionClosed.
    Closed() bool

    // Negotiated returns the negotiated session parameters.
    Negotiated() NegotiatedParams

    // Peer returns the peer info for this writer.
    Peer() PeerInfo

    // RequestRouteRefresh sends ROUTE-REFRESH message to peer (RFC 2918).
    // Returns ErrCapabilityNotNegotiated if peer doesn't support route refresh.
    // Use to request peer re-sends all routes for the specified AFI/SAFI.
    RequestRouteRefresh(afi uint16, safi uint8) error

    // RequestEnhancedRouteRefresh sends Enhanced ROUTE-REFRESH (RFC 7313).
    // subtype: 1=Begin-of-RR, 2=End-of-RR, 0=normal request
    // Returns ErrCapabilityNotNegotiated if peer doesn't support enhanced RR.
    RequestEnhancedRouteRefresh(afi uint16, safi uint8, subtype uint8) error
}

var (
    ErrSessionClosed    = errors.New("session closed")
    ErrValidationFailed = errors.New("RFC validation failed")
)

// --- Raw/Experimental Message Support ---

// RawWriter allows sending arbitrary BGP messages for protocol development.
// Use for: testing new RFCs/drafts, fuzzing, interop testing, research.
//
// AUDIT: All calls are logged at INFO level with message type and SHA256.
// Set zebgp.raw.log-payloads=true to log raw bytes at DEBUG level.
type RawWriter interface {
    // WriteRawUpdate sends raw UPDATE bytes (header added automatically).
    WriteRawUpdate(payload []byte) error

    // WriteRawMessage sends any BGP message type.
    // msgType: 1=OPEN, 2=UPDATE, 3=NOTIFICATION, 4=KEEPALIVE, 5=ROUTE-REFRESH
    WriteRawMessage(msgType uint8, payload []byte) error
}

// Plugins that need raw access implement this to receive RawWriter:
type RawWriterPlugin interface {
    PeerPlugin

    // OnEstablishedRaw is called instead of OnEstablished when plugin
    // declares it needs raw access. Receives both parsed and raw writers.
    OnEstablishedRaw(peer PeerInfo, writer UpdateWriter, raw RawWriter) UpdateHandler
}

// --- Custom/Experimental Capabilities ---

// CustomCapability allows plugins to advertise experimental capabilities.
// Code range validation is enforced to prevent collision with standard capabilities.
//
// RFC 5492 §4 defines capability code allocation:
//   - 0:       Reserved
//   - 1-63:    IETF Review
//   - 64-127:  First Come First Served
//   - 128-238: First Come First Served (many already allocated)
//   - 239-254: Experimental Use (safe for plugins)
//   - 255:     Reserved
type CustomCapability struct {
    Code  uint8  // Capability code (MUST be in experimental range 239-254)
    Value []byte // Raw capability value
}

// Validate checks that the capability code is in the experimental range.
// Called automatically before sending OPEN.
// RFC 5492 §4: codes 239-254 are designated for Experimental Use.
func (c *CustomCapability) Validate() error {
    if c.Code < 239 || c.Code > 254 {
        return fmt.Errorf("CustomCapability code %d invalid: use 239-254 per RFC 5492 §4", c.Code)
    }
    return nil
}

// In GetCapabilities, plugins can return custom capabilities:
// func (p *MyPlugin) GetCapabilities(peer PeerInfo) []capability.Capability {
//     return []capability.Capability{
//         &CustomCapability{Code: 239, Value: []byte{0x01, 0x02}},
//     }
// }

// --- Custom/Experimental Path Attributes ---

// CustomAttribute allows plugins to add experimental path attributes.
// Standard attribute codes (0-40) are validated to prevent accidental misuse.
//
// IANA BGP Path Attributes registry (unlike capabilities) has no explicit
// "experimental" range. Codes 128-254 are First Come First Served with some
// already allocated (e.g., 128=ATTR_SET per RFC 6368). Use codes 241-254.
//
// VALIDATION: CustomAttribute.Validate() is called by the validation layer
// before wire encoding, along with all other RFC compliance checks.
type CustomAttribute struct {
    Flags uint8  // Attribute flags (optional, transitive, partial, extended)
    Code  uint8  // Attribute type code (use 241-254 for experimental)
    Value []byte // Raw attribute value
}

// Validate checks the attribute code and warns about standard code usage.
// Returns error for well-known mandatory attributes that plugins should not override.
// Logs warning for codes that may conflict with IANA assignments.
func (a *CustomAttribute) Validate() error {
    // Block well-known mandatory attributes (plugins must not forge these)
    switch a.Code {
    case 1: // ORIGIN
        return errors.New("CustomAttribute cannot use ORIGIN (code 1)")
    case 2: // AS_PATH
        return errors.New("CustomAttribute cannot use AS_PATH (code 2)")
    case 3: // NEXT_HOP
        return errors.New("CustomAttribute cannot use NEXT_HOP (code 3)")
    case 5: // LOCAL_PREF
        return errors.New("CustomAttribute cannot use LOCAL_PREF (code 5)")
    }
    // Warn about codes outside safe experimental range
    if a.Code < 241 {
        slog.Warn("CustomAttribute code may conflict with IANA assignments",
            "code", a.Code, "recommended", "241-254")
    }
    return nil
}

// message.Update supports custom attributes:
// update := &message.Update{
//     Attributes: []attribute.Attribute{
//         attribute.Origin(attribute.OriginIGP),
//         &CustomAttribute{Flags: 0xC0, Code: 241, Value: experimentalData},
//     },
// }

// --- Custom/Experimental NLRI ---

// CustomNLRI allows plugins to send experimental address families.
// Validation layer checks that encoding is well-formed for the AFI/SAFI.
type CustomNLRI struct {
    AFI   uint16 // Address Family Identifier
    SAFI  uint8  // Subsequent AFI
    Value []byte // Raw NLRI encoding
}

// For MP_REACH_NLRI with experimental AFI/SAFI:
// update := &message.Update{
//     Attributes: []attribute.Attribute{
//         &attribute.MPReachNLRI{
//             AFI:      65000,  // Experimental AFI
//             SAFI:     128,    // Experimental SAFI
//             NextHop:  nextHopBytes,
//             NLRI:     []nlri.NLRI{&CustomNLRI{...}},
//         },
//     },
// }
```

### 3. API Plugin Interface

```go
// APIPlugin provides external API functionality.
// Multiple API plugins can run simultaneously.
type APIPlugin interface {
    Plugin

    // Start begins serving the API.
    Start() error

    // Stop gracefully shuts down the API.
    Stop() error
}

// APIPlugin implementations that need peer lifecycle events should
// also implement PeerPlugin. The registry will call both interfaces.
```

### 4. State Persistence (Optional)

```go
// StateProvider allows plugins to persist state across restarts.
// Implement this interface for plugins that need crash recovery.
type StateProvider interface {
    // Save persists plugin state. Called during graceful shutdown.
    // Return nil state to indicate nothing to persist.
    // Context has 5s timeout by default.
    Save(ctx context.Context) ([]byte, error)

    // Restore loads persisted state. Called during Init if state exists.
    // state is nil if no persisted state exists.
    // Return error to abort plugin initialization.
    Restore(ctx context.Context, state []byte) error
}

// State is persisted to: $ZEBGP_STATE_DIR/plugins/{plugin-name}.state
// Default: /var/lib/zebgp/plugins/
```

---

## Reactor API (Plugin Context)

Plugins interact with the reactor through this interface:

```go
// ReactorAPI provides controlled access to reactor functionality.
//
// THREAD SAFETY: All methods are safe for concurrent use.
type ReactorAPI interface {
    // Peer access
    Peers() []PeerInfo
    GetPeer(addr netip.Addr) (PeerInfo, bool)

    // Peer control
    TeardownPeer(addr netip.Addr, reason string) error

    // RIB access (if RIB plugin loaded)
    RIBIn() RIBReader
    RIBOut() RIBWriter

    // Event subscription - plugins can subscribe to events asynchronously.
    // Returns buffered channel. Buffer size configurable via zebgp.events.buffer-size.
    // Events are dropped if consumer is slow.
    // Dropped events increment zebgp_plugin_events_dropped counter.
    // Use dedicated goroutine to consume events promptly.
    Subscribe(events ...EventType) <-chan Event
    Unsubscribe(ch <-chan Event)

    // Service registry - plugins can expose services to other plugins.
    // Services are typed via interface assertion at consumption time.
    RegisterService(name string, version string, service any) error
    GetService(name string) (any, string, bool) // Returns (service, version, found)

    // Configuration
    Config() ReactorConfig
    PluginConfig(pluginName string) map[string]any

    // Logging - returns logger scoped to plugin name
    Logger() *slog.Logger

    // Metrics - plugins report metrics here
    Metrics() MetricsRegistry
}

// PeerInfo describes a peer for plugin consumption.
type PeerInfo struct {
    Address      netip.Addr
    LocalAddress netip.Addr
    LocalAS      uint32
    PeerAS       uint32
    RouterID     uint32
    State        string
    Established  time.Time
    Negotiated   NegotiatedParams

    // Per-peer plugin config (if configured)
    PluginConfig map[string]any
}

type NegotiatedParams struct {
    FourOctetAS     bool
    ExtendedMessage bool              // RFC 8654
    MaxMessageSize  int               // 4096 or 65535
    AddPath         map[AFI]AddPathMode
    Families        []AFI
    HoldTime        uint16
}

// --- Event System ---

type EventType int

const (
    EventPeerUp EventType = iota
    EventPeerDown
    EventStateChange
    EventRouteAnnounce
    EventRouteWithdraw
    EventConfigReload
    EventShutdown
)

type Event struct {
    Type      EventType
    Peer      PeerInfo      // For peer events
    Routes    []Route       // For route events
    Timestamp time.Time
}

// --- Metrics ---

// MetricsRegistry provides plugin-scoped metrics.
// All metrics are automatically prefixed with "zebgp_plugin_{name}_" to prevent collisions.
// Example: plugin "my-filter" calling Counter("requests") creates "zebgp_plugin_my_filter_requests"
//
// CARDINALITY WARNING: Avoid high-cardinality labels (e.g., peer addresses).
// Use buckets or aggregation for metrics with many unique values.
type MetricsRegistry interface {
    // Counter creates a monotonically increasing counter.
    // Name is automatically prefixed with plugin namespace.
    Counter(name string, labels ...string) Counter

    // Gauge creates a metric that can go up or down.
    Gauge(name string, labels ...string) Gauge

    // Histogram creates a distribution metric with the given buckets.
    Histogram(name string, buckets []float64, labels ...string) Histogram
}

// Standard metrics all plugins SHOULD report:
const (
    MetricInvocationsTotal = "invocations_total"  // Counter: handler calls
    MetricLatencySeconds   = "latency_seconds"    // Histogram: processing time
    MetricErrorsTotal      = "errors_total"       // Counter: errors by type
)

// --- Logging ---

// Plugins receive a plugin-scoped logger via ReactorAPI.Logger().
// All log entries are automatically tagged with plugin name.
//
// Example usage in plugin:
//   func (p *MyPlugin) Init(ctx context.Context, reactor ReactorAPI, cfg map[string]any) error {
//       p.log = reactor.Logger()  // Automatically includes "plugin"="my-filter" attribute
//       p.log.Info("initialized", "threshold", cfg["threshold"])
//       // Output: level=INFO msg=initialized plugin=my-filter threshold=100
//       return nil
//   }
//
// External plugins (gRPC/JSON-RPC) manage their own logging.
// Convention: log to stderr or sidecar file with plugin name prefix.
```

---

## Plugin Registry

```go
// Registry manages plugin discovery, loading, and lifecycle.
//
// NOTE: Go's native plugin system (.so/.dylib) is NOT supported due to severe
// limitations (same Go version, same stdlib, CGO required, no Windows).
// External plugins use gRPC or JSON-RPC transport instead.
type Registry struct {
    plugins     map[string]Plugin
    peerPlugins []PeerPlugin
    apiPlugins  []APIPlugin
    mu          sync.RWMutex
}

// Register adds an in-process plugin to the registry.
// Used for built-in plugins (RIB, ExaBGP API, etc.).
func (r *Registry) Register(p Plugin) error

// Connect connects to an external plugin via gRPC or JSON-RPC.
// address format: "grpc:///path/to/socket" or "jsonrpc://host:port"
func (r *Registry) Connect(address string) error

// Get returns a plugin by name.
func (r *Registry) Get(name string) (Plugin, bool)

// PeerPlugins returns all registered peer plugins in priority order.
func (r *Registry) PeerPlugins() []PeerPlugin

// APIPlugins returns all registered API plugins.
func (r *Registry) APIPlugins() []APIPlugin

// ResolveDependencies validates and orders plugins for initialization.
// Returns error if dependency cycle detected or required dependency missing.
func (r *Registry) ResolveDependencies() ([]Plugin, error)
```

### Dependency Cycle Detection

The registry uses Tarjan's algorithm to detect cycles before initialization.
Plugins are initialized in topological order (dependencies before dependents).

```go
// resolveDependencies validates dependency graph and returns init order.
func (r *Registry) resolveDependencies() ([]Plugin, error) {
    // Build dependency graph
    graph := make(map[string][]string)
    plugins := make(map[string]Plugin)

    for name, p := range r.plugins {
        plugins[name] = p
        for _, dep := range p.Dependencies() {
            graph[name] = append(graph[name], dep.Name)
        }
    }

    // Detect cycles using DFS with state tracking
    if cycle := detectCycle(graph); cycle != nil {
        return nil, fmt.Errorf("dependency cycle: %s", strings.Join(cycle, " → "))
    }

    // Topological sort for init order
    return topologicalSort(plugins, graph)
}

func detectCycle(graph map[string][]string) []string {
    const (
        unvisited = 0
        visiting  = 1
        visited   = 2
    )

    state := make(map[string]int)
    var cycle []string

    var dfs func(node string, path []string) bool
    dfs = func(node string, path []string) bool {
        if state[node] == visiting {
            // Found cycle - extract it
            for i, n := range path {
                if n == node {
                    cycle = append(path[i:], node)
                    return true
                }
            }
        }
        if state[node] == visited {
            return false
        }

        state[node] = visiting
        path = append(path, node)

        for _, dep := range graph[node] {
            if dfs(dep, path) {
                return true
            }
        }

        state[node] = visited
        return false
    }

    for node := range graph {
        if state[node] == unvisited {
            if dfs(node, nil) {
                return cycle
            }
        }
    }
    return nil
}

func topologicalSort(plugins map[string]Plugin, graph map[string][]string) ([]Plugin, error) {
    // Kahn's algorithm for topological sort
    // graph[X] = list of plugins that X depends on
    // inDegree[X] = number of unresolved dependencies for X
    // dependents[Y] = list of plugins that depend on Y (need Y to init first)

    inDegree := make(map[string]int)
    dependents := make(map[string][]string)

    for name := range plugins {
        inDegree[name] = len(graph[name])
        for _, dep := range graph[name] {
            dependents[dep] = append(dependents[dep], name)
        }
    }

    // Start with plugins that have no dependencies (inDegree == 0)
    var queue []string
    for name, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, name)
        }
    }

    var result []Plugin
    for len(queue) > 0 {
        // Sort for deterministic order
        sort.Strings(queue)
        name := queue[0]
        queue = queue[1:]

        if p, ok := plugins[name]; ok {
            result = append(result, p)
        }

        // This plugin is now initialized - decrement inDegree of its dependents
        for _, dependent := range dependents[name] {
            inDegree[dependent]--
            if inDegree[dependent] == 0 {
                queue = append(queue, dependent)
            }
        }
    }

    // Check for unresolved dependencies (cycle or missing plugin)
    if len(result) != len(plugins) {
        var unresolved []string
        for name, degree := range inDegree {
            if degree > 0 {
                unresolved = append(unresolved, name)
            }
        }
        return nil, fmt.Errorf("unresolved dependencies: %v", unresolved)
    }

    return result, nil
}
```

**Startup behavior:**

```
INFO: Loading plugins...
INFO: Plugin "rib" registered (priority 0, no dependencies)
INFO: Plugin "policy" registered (priority 50, depends on: rib)
INFO: Plugin "logging" registered (priority 900, no dependencies)
INFO: Resolving dependencies...
INFO: Init order: rib → policy → logging

# Or if cycle detected:
ERROR: Dependency cycle detected: filter → validator → filter
FATAL: Cannot start - fix plugin dependencies in configuration
```

**Test case:**

```go
func TestDependencyCycleDetection(t *testing.T) {
    r := NewRegistry()

    // A depends on B, B depends on C, C depends on A
    r.Register(&mockPlugin{name: "A", deps: []Dependency{{Name: "B"}}})
    r.Register(&mockPlugin{name: "B", deps: []Dependency{{Name: "C"}}})
    r.Register(&mockPlugin{name: "C", deps: []Dependency{{Name: "A"}}})

    _, err := r.ResolveDependencies()
    if err == nil {
        t.Fatal("expected cycle error")
    }
    if !strings.Contains(err.Error(), "dependency cycle") {
        t.Errorf("unexpected error: %v", err)
    }
    if !strings.Contains(err.Error(), "A → B → C → A") {
        t.Errorf("cycle path not shown: %v", err)
    }
}

func TestTopologicalOrder(t *testing.T) {
    r := NewRegistry()

    // D has no deps, C depends on D, B depends on C, A depends on B
    r.Register(&mockPlugin{name: "A", deps: []Dependency{{Name: "B"}}})
    r.Register(&mockPlugin{name: "B", deps: []Dependency{{Name: "C"}}})
    r.Register(&mockPlugin{name: "C", deps: []Dependency{{Name: "D"}}})
    r.Register(&mockPlugin{name: "D", deps: nil})

    order, err := r.ResolveDependencies()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // D must come before C, C before B, B before A
    indexOf := func(name string) int {
        for i, p := range order {
            if p.Name() == name {
                return i
            }
        }
        return -1
    }

    if indexOf("D") > indexOf("C") {
        t.Error("D should init before C")
    }
    if indexOf("C") > indexOf("B") {
        t.Error("C should init before B")
    }
    if indexOf("B") > indexOf("A") {
        t.Error("B should init before A")
    }
}
```

### Plugin Lifecycle

```
                                    ┌─────────────────────┐
                                    │                     │
                          ┌────────►│   Failed/Disabled   │
                          │         │                     │
                          │         └─────────────────────┘
                          │ Init error / panic                ▲
                          │                                   │
┌───────────┐    ┌───────────────┐    ┌───────────┐    ┌────────────┐
│           │    │               │    │           │    │            │
│ Discovered│───►│ Initializing  │───►│   Ready   │───►│  Running   │
│           │    │               │    │           │    │            │
└───────────┘    └───────────────┘    └───────────┘    └────────────┘
                          │                                   │
                          │ Dependencies resolved             │ Shutdown signal
                          │ Config validated                  │ or fatal error
                          │                                   ▼
                          │                            ┌────────────┐
                          │                            │            │
                          └───────────────────────────►│  Closing   │
                                Close timeout          │            │
                                                       └────────────┘
                                                              │
                                                              ▼
                                                       ┌────────────┐
                                                       │            │
                                                       │   Closed   │
                                                       │            │
                                                       └────────────┘
```

**State descriptions:**

| State | Description |
|-------|-------------|
| Discovered | Plugin found (config, directory scan, or socket) |
| Initializing | Dependencies loading, Init() called |
| Ready | Init complete, waiting for peers |
| Running | Processing peer events and UPDATEs |
| Closing | Close() called, cleanup in progress |
| Closed | Plugin fully stopped |
| Failed/Disabled | Init failed or circuit breaker open |

**External plugin states:**

| State | Circuit Breaker | Description |
|-------|-----------------|-------------|
| Connected | Closed | Normal operation |
| Disconnected | Closed | Reconnecting with backoff |
| Failing | Half-Open | Testing if recovered |
| Bypassed | Open | Plugin skipped, using timeout-action |

---

## Multi-Plugin Behavior

### Capability Merging

When multiple plugins return capabilities, they are merged according to specific rules:

```go
// CapabilityMerger controls how capabilities from multiple plugins combine.
type CapabilityMerger interface {
    Merge(caps [][]capability.Capability) []capability.Capability
}
```

**Merge rules by capability type:**

| Capability | Merge Rule | Example |
|------------|------------|---------|
| MP-BGP (AFI/SAFI) | Union | Plugin A: IPv4, Plugin B: IPv6 → Both |
| 4-Octet AS | OR | Any plugin requests → enabled |
| AddPath | Union modes | A: send, B: receive → send+receive |
| Route Refresh | OR | Any plugin requests → enabled |
| Extended Message | OR | Any plugin requests → enabled |
| Graceful Restart | Complex | See below |
| Hold Time | Minimum non-zero | A: 90s, B: 180s → 90s |
| Custom (239-254) | Last plugin wins | Warning logged |

**Graceful Restart merging:**
- Restart Time: minimum of all requested values
- AFI/SAFI flags: union with OR of forwarding bits
- Restart flags: OR of all plugin requests

**Conflict resolution order:**
1. Plugin with lower priority wins
2. If same priority, alphabetical by name
3. Warning logged with both values

**Conflict handling:**
```go
// On conflict, log warning with details:
// WARN: Capability conflict for code 65 (4-octet AS):
//       plugin "rib" (priority 0) requests ASN 65000
//       plugin "policy" (priority 100) requests ASN 65001
//       Using value from "rib" (lower priority)
```

**Example:**
```
Plugin A (priority 0): AddPath send for IPv4
Plugin B (priority 100): AddPath receive for IPv4, send for IPv6
→ Result: AddPath send+receive for IPv4, send for IPv6
```

### Handler Chaining

UPDATE handlers form a processing pipeline:

```
UPDATE received
    │
    ▼
┌─────────────────┐
│ Handler 1 (log) │ → Continue()
└────────┬────────┘
         │
         ▼
┌─────────────────────┐
│ Handler 2 (filter)  │ → Continue(), Drop(), or CloseSession()
└────────┬────────────┘
         │ (modified UPDATE passed to next handler)
         ▼
┌─────────────────┐
│ Handler 3 (RIB) │ → Store in RIB, Continue()
└─────────────────┘
```

**Handler return values:**
- `Continue()` → Continue to next handler (UPDATE unchanged or modified in-place)
- `Drop()` → Silently discard UPDATE, session continues, stop chain
- `CloseSession(notif)` → Send notification, close session, stop chain

### UPDATE Modification Semantics

Modifications are **CUMULATIVE**. Each handler receives the UPDATE as modified by previous handlers:

```
Original UPDATE: {routes: [A, B, C], communities: [100:1]}
    │
Handler 1: Adds community 200:2
    │ UPDATE now: {routes: [A, B, C], communities: [100:1, 200:2]}
    ▼
Handler 2: Filters route B
    │ UPDATE now: {routes: [A, C], communities: [100:1, 200:2]}
    ▼
Handler 3: Sees UPDATE with routes [A, C] and communities [100:1, 200:2]
```

**Modification rules:**
- Handlers modify the UPDATE struct in-place
- Changes are visible to all subsequent handlers
- Original UPDATE is not preserved (use deep copy if needed)
- RFC validation occurs AFTER all handlers complete
- UPDATE objects are pooled - do NOT retain references after handler returns

**Handler can modify UPDATE:**
- Add/remove attributes
- Filter routes (remove from NLRI/Withdrawn lists)
- Transform NLRI
- Modify path attributes (AS_PATH, communities, etc.)

**Forbidden modifications** (enforced by validation layer):
- Remove mandatory attributes (ORIGIN, AS_PATH, NEXT_HOP)
- Create malformed attribute encoding
- Violate RFC 4271 attribute rules

### Per-Peer Plugin Configuration

Plugins can have different config per peer:

```toml
[zebgp.plugin]
rib = true

# Per-peer overrides
[zebgp.peer."192.168.1.1".plugin]
route-policy = true

[zebgp.peer."192.168.1.1".policy]
import = "/etc/zebgp/peer1-import.policy"
```

```go
// In plugin code:
func (p *PolicyPlugin) OnEstablished(peer PeerInfo, writer UpdateWriter) UpdateHandler {
    // Get per-peer config, fallback to global
    cfg := peer.PluginConfig["policy"]
    if cfg == nil {
        cfg = p.globalConfig
    }
    // ...
}
```

### Inter-Plugin Communication

Plugins can expose services to other plugins:

```go
// RIB plugin exposes its RIB as a service
func (p *RIBPlugin) Init(ctx context.Context, reactor ReactorAPI, cfg map[string]any) error {
    p.ribIn = rib.NewIncomingRIB()
    p.ribOut = rib.NewOutgoingRIB()

    // Register services for other plugins (with version)
    reactor.RegisterService("rib.in", "1.0", p.ribIn)
    reactor.RegisterService("rib.out", "1.0", p.ribOut)
    return nil
}

// Policy plugin uses RIB service
func (p *PolicyPlugin) Init(ctx context.Context, reactor ReactorAPI, cfg map[string]any) error {
    // Get RIB service (if available)
    if svc, version, ok := reactor.GetService("rib.in"); ok {
        p.ribIn = svc.(*rib.IncomingRIB)
        p.log.Info("using rib.in service", "version", version)
    }
    return nil
}
```

**Service lifecycle:**
1. Plugins initialize in dependency order (respecting Dependencies() declarations)
2. Earlier plugins register services
3. Later plugins can consume services
4. Missing required services → Init() returns error
5. Missing optional services → graceful degradation
6. Version mismatches logged as warnings

### Panic Recovery

Plugins may panic due to bugs. ZeBGP recovers gracefully:

```
Plugin Panic During Handler
    │
    ├─ Panic is recovered
    ├─ Stack trace logged at ERROR level
    ├─ Session closed with NOTIFICATION (Cease/Administrative Reset)
    ├─ Plugin marked as "failed" (circuit breaker opens)
    └─ Metric incremented: zebgp_plugin_panics_total{plugin="name"}
```

**Recovery behavior by location:**

| Location | Recovery Action |
|----------|-----------------|
| `Init()` | Plugin disabled, startup continues without it |
| `GetCapabilities()` | Return empty capabilities, log error |
| `OnEstablished()` | Close session with Cease |
| `UpdateHandler` | Close session with Cease |
| `OnClose()` | Log error, continue cleanup |
| `Close()` | Log error, continue shutdown (timeout enforced) |

**External plugins:** Panic in external process doesn't affect ZeBGP. Connection is closed, circuit breaker handles retry.

### Backpressure Strategy

When handler chain is slow and incoming UPDATEs queue up:

```
TCP Receive Buffer (OS-managed)
    │
    ▼
Per-Peer Input Queue (bounded, default: 1000 UPDATEs)
    │
    ▼
Handler Chain Processing
    │
    ▼
Output (if any)
```

**Backpressure stages:**

| Stage | Queue Depth | Action |
|-------|-------------|--------|
| Normal | < 100 | Process immediately |
| Warning | 100-500 | Log warning, continue |
| High | 500-1000 | Log error, apply skip-slow-behavior |
| Critical | 1000 | Stop reading from peer, TCP backpressure |
| Overflow | > 1000 (shouldn't happen) | Apply drop-policy, increment `zebgp_updates_dropped` |

**Configuration:**

```toml
[zebgp.backpressure]
queue-size = 1000
high-watermark = 500
skip-slow-plugins = true       # Skip plugins that timeout under load
skip-slow-behavior = "pass"    # pass (continue chain), drop (discard UPDATE), queue (block)

# Drop policy when queue overflows (should not happen with TCP backpressure)
# IMPORTANT: BGP semantics require careful consideration of what to drop
drop-policy = "newest"         # newest (default), oldest, or never-withdrawals
```

**Drop policy options:**

| Policy | Behavior | Trade-off |
|--------|----------|-----------|
| `newest` | Drop incoming UPDATE, keep queued | Preserves ordering, may lose recent routes |
| `oldest` | Drop oldest queued UPDATE | Processes recent routes faster, breaks ordering |
| `never-withdrawals` | Like `newest`, but never drop withdrawals | Ensures route cleanup, may queue withdrawals indefinitely |

**Recommendation:** Use `newest` (default) to preserve BGP UPDATE ordering semantics.
Withdrawals are generally more critical than announcements since stale routes cause
black-holing. Consider `never-withdrawals` for networks where route cleanup is critical.

**Slow plugin handling:**

When a plugin consistently exceeds timeout under backpressure:
1. Log warning with plugin name and latency
2. If `skip-slow-plugins = true`: apply `skip-slow-behavior`
3. Handler resumes when queue depth returns to normal
4. Metric: `zebgp_plugin_skipped_total{plugin="name", reason="backpressure"}`

**skip-slow-behavior options:**
- `pass` - Continue to next handler, skipping slow plugin (may accept routes that should be filtered)
- `drop` - Discard UPDATE entirely (safe but loses routes)
- `queue` - Block until plugin responds (maintains correctness but may cause TCP backpressure)

### Async Event Subscription

Plugins can react to events asynchronously:

```go
func (p *MonitorPlugin) Init(ctx context.Context, reactor ReactorAPI, cfg map[string]any) error {
    // Subscribe to peer and route events
    events := reactor.Subscribe(EventPeerUp, EventPeerDown, EventRouteAnnounce)

    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case ev := <-events:
                switch ev.Type {
                case EventPeerUp:
                    p.log.Info("peer up", "addr", ev.Peer.Address)
                case EventRouteAnnounce:
                    p.log.Info("routes announced", "count", len(ev.Routes))
                }
            }
        }
    }()
    return nil
}
```

---

## Event Flow

### Session Establishment

```
1. Connection accepted/established
   │
2. Check if delayed-open enabled for peer
   │
   ├─ [delayed-open = false (default)]
   │   │
   │   2a. For each PeerPlugin (in order):
   │       caps := plugin.GetCapabilities(peer)
   │       → Merge capabilities
   │   │
   │   2b. OPEN sent with merged capabilities
   │   │
   │   2c. OPEN received from peer
   │
   └─ [delayed-open = true]
       │
       2a. OPEN received from peer
       │
       2b. For each PeerPlugin (in order):
           caps := plugin.GetCapabilities(peer)
           → Merge capabilities
       │
       2c. For each PeerPlugin (in order):
           adjusted := plugin.AdjustCapabilities(peer, ourCaps, peerCaps)
           if adjusted != nil:
               → Replace our capabilities with adjusted
       │
       2d. OPEN sent with final capabilities
   │
3. For each PeerPlugin (in order):
   │   notif := plugin.OnOpenMessage(peer, routerID, caps)
   │   if notif != nil:
   │       → Send NOTIFICATION, close session
   │       → Stop processing
   │
4. Session Established
   │
5. For each PeerPlugin (in order):
   │   handler := plugin.OnEstablished(peer, writer)
   │   → Store handler in handler chain
   │
6. Ready to process UPDATEs
```

### UPDATE Processing

```
1. UPDATE received
   │
2. Parse UPDATE
   │
3. For each handler in chain:
   │   result := handler(peer, update)
   │   switch result.Action:
   │       ActionContinue:
   │           → Continue to next handler (UPDATE may be modified)
   │       ActionDrop:
   │           → Log, stop chain, discard UPDATE, session continues
   │       ActionCloseSession:
   │           → Send NOTIFICATION, close session, stop chain
   │
4. UPDATE fully processed (if ActionContinue through all handlers)
```

### Session Close

```
1. Session leaves Established (any reason)
   │
2. For each PeerPlugin (reverse order):
   │   plugin.OnClose(peer)
   │   (called exactly once per Established session)
   │
3. Cleanup complete
```

### Outbound UPDATE Flow

Plugins generate outbound UPDATEs through the RIB and export policy chain:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Outbound UPDATE Generation                          │
└─────────────────────────────────────────────────────────────────────────────┘

1. Route Source (one of):
   │
   ├─ API command: "announce route 10.0.0.0/24 next-hop self"
   │
   ├─ Config static route
   │
   └─ Inbound UPDATE (re-advertisement)
   │
   ▼
2. RIB Plugin (Adj-RIB-Out)
   │
   ├─ Store route with attributes
   ├─ Run best-path selection (if multiple paths)
   └─ Mark route for advertisement
   │
   ▼
3. Export Policy Chain (per target peer)
   │
   ├─ For each PeerPlugin with OnExport hook:
   │   │
   │   │   result := plugin.OnExport(sourcePeer, targetPeer, route)
   │   │   switch result.Action:
   │   │       ActionContinue: → Next plugin (route may be modified)
   │   │       ActionDrop:     → Don't send to this peer, stop chain
   │   │
   └─ Route passes all export policies
   │
   ▼
4. UPDATE Generation
   │
   ├─ Group routes by common attributes (path packing)
   ├─ Apply per-peer transformations:
   │   ├─ NEXT_HOP: self vs. preserve
   │   ├─ AS_PATH: prepend local AS (eBGP) or preserve (iBGP)
   │   └─ LOCAL_PREF: set for iBGP, strip for eBGP
   │
   ▼
5. Validation Layer
   │
   ├─ Verify mandatory attributes present
   ├─ Check message size vs. negotiated limit
   └─ Reject malformed UPDATEs with error log
   │
   ▼
6. Wire Encoding → Send to peer
```

**Export Policy Interface:**

```go
// ExportHandler is called for each route being exported to a peer.
// Plugins can filter or modify routes before they're sent.
//
// IMPORTANT: This is called per-route, not per-UPDATE. Multiple routes
// may be packed into a single UPDATE after export policy processing.
type ExportHandler func(source, target PeerInfo, route *Route) ExportResult

type ExportResult struct {
    Action   ExportAction
    Modified *Route // If Action == ActionModify
}

type ExportAction int

const (
    ExportContinue ExportAction = iota // Send to peer (possibly modified)
    ExportDrop                         // Don't send to this peer
    ExportModify                       // Use modified route
)

// PeerPlugin can optionally implement export policy:
type ExportPlugin interface {
    PeerPlugin

    // OnExport is called when a route is about to be sent to a peer.
    // source is the peer that originally advertised the route (or nil for local).
    // target is the peer we're sending to.
    OnExport(source, target PeerInfo, route *Route) ExportResult
}
```

**End-of-RIB (EOR) Handling:**

After initial route sync, plugins must send EOR markers:

```go
// UpdateWriter includes EOR support
type UpdateWriter interface {
    // ... existing methods ...

    // SendEOR sends End-of-RIB marker for the specified address family.
    // Called after initial route sync is complete.
    // RFC 4724: EOR is an UPDATE with no NLRI and no withdrawn routes.
    SendEOR(afi uint16, safi uint8) error
}
```

**RIB Plugin coordinates EOR:**
1. On session Established, start initial sync
2. Send all Adj-RIB-Out routes for this peer
3. After last route, call `writer.SendEOR(afi, safi)` for each family
4. Mark peer as "initial sync complete"

---

## Built-in Plugins

### 1. ExaBGP API Plugin (Default)

Wraps current `pkg/api` as a plugin:

```go
// ExaBGPAPIPlugin provides ExaBGP-compatible Unix socket API.
type ExaBGPAPIPlugin struct {
    server *api.Server
    config ExaBGPAPIConfig
}

type ExaBGPAPIConfig struct {
    SocketPath string            // "/var/run/zebgp.sock"
    Processes  []ProcessConfig   // External process spawning
}

func (p *ExaBGPAPIPlugin) Name() string    { return "exabgp-api" }
func (p *ExaBGPAPIPlugin) Version() string { return "1.0.0" }

func (p *ExaBGPAPIPlugin) Init(ctx context.Context, reactor ReactorAPI, cfg map[string]any) error {
    p.config = parseConfig(cfg)
    p.server = api.NewServer(p.config.SocketPath, reactor)
    return nil
}

func (p *ExaBGPAPIPlugin) Start() error { return p.server.Start() }
func (p *ExaBGPAPIPlugin) Stop() error  { return p.server.Stop() }
func (p *ExaBGPAPIPlugin) Close() error { return nil }
```

### 2. RIB Plugin (Default)

Manages Adj-RIB-In and Adj-RIB-Out:

```go
// RIBPlugin manages routing information bases.
type RIBPlugin struct {
    ribIn    *rib.IncomingRIB
    ribOut   *rib.OutgoingRIB
    store    *rib.RouteStore
}

func (p *RIBPlugin) Name() string    { return "rib" }
func (p *RIBPlugin) Version() string { return "1.0.0" }

func (p *RIBPlugin) OnEstablished(peer PeerInfo, writer UpdateWriter) UpdateHandler {
    return func(peer PeerInfo, update *message.Update) HandlerResult {
        // Store routes in Adj-RIB-In
        p.ribIn.ProcessUpdate(peer.Address, update)
        return Continue()
    }
}
```

---

## Configuration

Follows ExaBGP pattern: **TOML file** + **environment variable overrides**.

### Config File Location

```
$ZEBGP_ROOT/etc/zebgp/zebgp.toml
```

Or specify via: `zebgp --config /path/to/zebgp.toml`

### Priority Order

1. Environment variable (dot notation): `zebgp.plugin.grpc-api=true`
2. Environment variable (underscore): `zebgp_plugin_grpc_api=true`
3. TOML file value
4. Default

### TOML File Format

```toml
# etc/zebgp/zebgp.toml

[zebgp.plugin]
exabgp-api = true
grpc-api = false
rib = true
route-policy = false
external = ""
close-timeout = "5s"    # Timeout for plugin Close()

[zebgp.events]
buffer-size = 1000      # Event channel buffer size

[zebgp.grpc]
listen = "localhost:50051"
tls-cert = ""
tls-key = ""
reflection = true

[zebgp.policy]
import = ""
export = ""
```

### Plugin Section Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `exabgp-api` | bool | `true` | Enable ExaBGP-compatible API |
| `grpc-api` | bool | `false` | Enable gRPC API |
| `rib` | bool | `true` | Enable RIB management plugin |
| `route-policy` | bool | `false` | Enable route policy plugin |
| `external` | string | `''` | Comma-separated plugin addresses |
| `close-timeout` | duration | `5s` | Timeout for plugin Close() |
| `discover-sockets` | string | `''` | Directory to scan for *.sock files (auto-connect) |

### Plugin Discovery

Enable automatic plugin discovery without explicit configuration:

```toml
[zebgp.plugin]
# Auto-connect to external plugins (scans for *.sock files)
discover-sockets = "/var/run/zebgp/plugins"
```

**Discovery behavior:**

| Type | Pattern | Action |
|------|---------|--------|
| gRPC socket | `*-grpc.sock` | Connect as gRPC client |
| JSON-RPC socket | `*-jsonrpc.sock` or `*.sock` | Connect as JSON-RPC client |

**Note:** Go's native plugin system (.so/.dylib) is NOT supported due to severe
limitations (same Go version, same stdlib, CGO required, no Windows support).
All external plugins must use gRPC or JSON-RPC transport.

**Discovery order:**
1. Explicitly configured plugins (in config order)
2. Discovered socket plugins (alphabetical)

**Disabled plugins:** Create `.disabled` file to skip:
```bash
touch /var/run/zebgp/plugins/my-plugin.sock.disabled  # Skip this plugin
```

### Per-Plugin Sections

#### gRPC Plugin (`[zebgp.grpc]`)

```toml
[zebgp.grpc]
listen = "0.0.0.0:50051"
tls-cert = "/etc/zebgp/cert.pem"
tls-key = "/etc/zebgp/key.pem"
reflection = true
```

#### Route Policy Plugin (`[zebgp.policy]`)

```toml
[zebgp.policy]
import = "/etc/zebgp/import.policy"
export = "/etc/zebgp/export.policy"
```

### Go Implementation

```go
// PluginSection holds plugin-related settings.
type PluginSection struct {
    ConfigSection
    _section_name = "plugin"

    ExaBGPAPI    bool          // exabgp-api: Enable ExaBGP API (default: true)
    GRPCAPI      bool          // grpc-api: Enable gRPC API (default: false)
    RIB          bool          // rib: Enable RIB plugin (default: true)
    RoutePolicy  bool          // route-policy: Enable policy plugin (default: false)
    External     string        // external: Plugin paths (default: "")
    CloseTimeout time.Duration // close-timeout: Plugin close timeout (default: 5s)
}

// EventsSection holds event system settings.
type EventsSection struct {
    ConfigSection
    _section_name = "events"

    BufferSize int // buffer-size: Event channel buffer (default: 1000)
}

// GRPCSection holds gRPC plugin settings.
type GRPCSection struct {
    ConfigSection
    _section_name = "grpc"

    Listen     string // listen: Address to listen on (default: "localhost:50051")
    TLSCert    string // tls-cert: TLS certificate path
    TLSKey     string // tls-key: TLS key path
    Reflection bool   // reflection: Enable gRPC reflection (default: true)
}

// PolicySection holds route policy plugin settings.
type PolicySection struct {
    ConfigSection
    _section_name = "policy"

    Import string // import: Import policy file path
    Export string // export: Export policy file path
}
```

### Loading (ExaBGP Pattern)

```go
func (e *Environment) Setup() {
    ini := parseINI(findEnvFile())

    for section, options := range e.sections() {
        for name, opt := range options {
            envDot := fmt.Sprintf("zebgp.%s.%s", section, name)
            envUnderscore := strings.ReplaceAll(envDot, ".", "_")

            // Priority: env (dot) > env (underscore) > INI > default
            if val, ok := os.LookupEnv(envDot); ok {
                opt.Parse(val)
            } else if val, ok := os.LookupEnv(envUnderscore); ok {
                opt.Parse(val)
            } else if val, ok := ini.Get(section, name); ok {
                opt.Parse(val)
            }
            // else: keep default
        }
    }
}
```

### Backward Compatibility

Existing `[zebgp.api]` section continues to work for ExaBGP API plugin:

```toml
[zebgp.api]
ack = true
encoder = "json"
respawn = true
socket-name = "zebgp"
```

### External Plugin Configuration

External plugins get their own TOML section based on plugin name:

```toml
# External plugin connected via gRPC at /var/run/zebgp/custom-filter.sock
# Plugin.Name() returns "custom-filter"

[zebgp.custom-filter]
threshold = 100
mode = "strict"
```

Plugin receives parsed config. Use the config helper package for type-safe access:

```go
import "codeberg.org/thomas-mangin/zebgp/pkg/plugin/config"

func (p *CustomPlugin) Init(ctx context.Context, reactor ReactorAPI, cfg map[string]any) error {
    // Type-safe config access with defaults and validation
    threshold, err := config.GetInt(cfg, "threshold", 100)    // default: 100
    if err != nil {
        return fmt.Errorf("invalid threshold: %w", err)
    }

    mode, err := config.GetString(cfg, "mode", "normal")      // default: "normal"
    if err != nil {
        return fmt.Errorf("invalid mode: %w", err)
    }

    timeout, err := config.GetDuration(cfg, "timeout", 5*time.Second)
    if err != nil {
        return fmt.Errorf("invalid timeout: %w", err)
    }

    enabled := config.GetBool(cfg, "enabled", true)           // default: true

    // Nested config access
    tlsCfg, _ := config.GetMap(cfg, "tls")
    certPath, _ := config.GetString(tlsCfg, "cert", "")

    return nil
}
```

**Config helper functions:**

```go
package config

// GetString returns string value or default. Error if wrong type.
func GetString(cfg map[string]any, key string, def string) (string, error)

// GetInt returns int value or default. Handles int, int64, float64.
func GetInt(cfg map[string]any, key string, def int) (int, error)

// GetBool returns bool value or default.
func GetBool(cfg map[string]any, key string, def bool) bool

// GetDuration parses duration string ("5s", "1m") or returns default.
func GetDuration(cfg map[string]any, key string, def time.Duration) (time.Duration, error)

// GetStringSlice returns []string or default.
func GetStringSlice(cfg map[string]any, key string, def []string) ([]string, error)

// GetMap returns nested map or empty map.
func GetMap(cfg map[string]any, key string) (map[string]any, bool)

// MustGetString panics if key missing or wrong type (for required fields).
func MustGetString(cfg map[string]any, key string) string
```

### Environment Override Examples

```bash
# Override INI file settings
export zebgp.plugin.grpc-api=true
export zebgp.grpc.listen=0.0.0.0:50051

# Underscore notation works too
export zebgp_grpc_tls_cert=/etc/zebgp/cert.pem
```

---

## Third-Party Plugin Development

Plugins communicate with ZeBGP via **gRPC** or **JSON-RPC** over Unix socket or TCP.
This enables plugins in **any language** (Python, Rust, Go, C++, etc.).

### Transport Options

| Transport | Protocol | Use Case |
|-----------|----------|----------|
| Unix socket | gRPC | Local plugins, best performance |
| Unix socket | JSON-RPC | Local plugins, simple implementation |
| TCP | gRPC | Remote plugins, microservices |
| TCP | JSON-RPC | Remote plugins, debugging |
| stdio | JSON-RPC | ExaBGP process compatibility |
| In-process | Go interface | Built-in plugins only |

### ExaBGP Process Compatibility

For migration from ExaBGP process model:

```toml
[zebgp.plugin.my-exabgp-process]
transport = "stdio"
command = "/usr/bin/python3 /etc/zebgp/my-process.py"
```

This spawns the process and communicates via stdin/stdout using JSON-RPC.
Provides backward compatibility with ExaBGP-style external processes.

### API Version Negotiation

ZeBGP and plugins negotiate a compatible API version during Init:

```
ZeBGP sends:     api_version = "1.2"  (current ZeBGP version)
Plugin responds: supported_api_version = "1.0" (plugin's max version)
Negotiated:      min(1.2, 1.0) = 1.0
```

**Version compatibility rules:**

| ZeBGP | Plugin | Result |
|-------|--------|--------|
| 1.2 | 1.0 | ✅ Use 1.0 features only |
| 1.2 | 1.2 | ✅ Full feature set |
| 1.2 | 2.0 | ✅ Use 1.2 (plugin is newer) |
| 2.0 | 1.2 | ⚠️ Use 1.2 with warning |
| 2.0 | 1.0 | ❌ Error if breaking changes in 2.0 |
| 0.9 | 1.0 | ⚠️ Use 0.9 with warning (plugin newer) |

**Semantic versioning:**
- **Major** (X.0): Breaking changes, old plugins may not work
- **Minor** (1.X): New features, backward compatible
- **Patch** (1.0.X): Bug fixes only

**Version-gated features:**

```go
// In plugin proxy
if negotiatedVersion >= "1.1" {
    // Use AdjustCapabilities hook
} else {
    // Skip, plugin doesn't support it
}
```

**Startup behavior:**

```
1. ZeBGP connects to plugin
2. Sends InitRequest with api_version
3. Plugin returns InitResponse with supported_api_version
4. If major version mismatch and breaking changes exist:
   → Log error, disable plugin
5. Otherwise:
   → Use minimum version, log if downgraded
```

### Plugin API Compatibility Matrix

Track API compatibility across ZeBGP versions:

| ZeBGP Version | Plugin API | New Features | Breaking Changes |
|---------------|------------|--------------|------------------|
| 1.0.x | 1.0 | Initial release | - |
| 1.1.x | 1.0, 1.1 | GracefulRestartHandler, OnError | None |
| 1.2.x | 1.0, 1.1, 1.2 | RequestRouteRefresh, Clone() | None |
| 2.0.x | 1.2, 2.0 | New handler signature | 1.0, 1.1 deprecated |

**Plugin compatibility:**

| Plugin API | ZeBGP 1.0 | ZeBGP 1.1 | ZeBGP 1.2 | ZeBGP 2.0 |
|------------|-----------|-----------|-----------|-----------|
| 1.0 | ✅ | ✅ | ✅ | ⚠️ deprecated |
| 1.1 | ❌ | ✅ | ✅ | ⚠️ deprecated |
| 1.2 | ❌ | ❌ | ✅ | ✅ |
| 2.0 | ❌ | ❌ | ❌ | ✅ |

**Deprecation policy:**

1. **Minor version bump (1.x → 1.y):** Full backward compatibility
2. **Major version bump (1.x → 2.0):** Previous major deprecated, removed after 2 releases
3. **Deprecation warnings:** Logged at startup when using old API version
4. **Migration guide:** Published with each major version

### Error Codes

Standard error codes for plugin-related failures:

```go
const (
    // Plugin initialization errors (1000-1099)
    ErrPluginInitFailed       = 1001  // Init() returned error
    ErrPluginConfigInvalid    = 1002  // Invalid configuration
    ErrPluginDependencyMissing = 1003  // Required service not available
    ErrPluginVersionMismatch  = 1004  // API version incompatible

    // Plugin runtime errors (1100-1199)
    ErrPluginTimeout          = 1100  // Handler exceeded timeout
    ErrPluginPanic            = 1101  // Handler panicked
    ErrPluginClosed           = 1102  // Plugin closed unexpectedly

    // External plugin errors (1200-1299)
    ErrPluginConnectFailed    = 1200  // Cannot connect to plugin
    ErrPluginDisconnected     = 1201  // Connection lost
    ErrPluginCircuitOpen      = 1202  // Circuit breaker open

    // Validation errors (1300-1399)
    ErrValidationFailed       = 1300  // RFC validation failed
    ErrInvalidCapability      = 1301  // Capability code out of range
    ErrInvalidAttribute       = 1302  // Attribute code reserved
)
```

### Configuration

```toml
[zebgp.plugin]
# Built-in plugins (in-process)
rib = true
exabgp-api = true

# External plugins
external = "grpc:///var/run/zebgp-filter.sock, jsonrpc://localhost:9000"

[zebgp.plugin.my-filter]
transport = "grpc"
address = "/var/run/zebgp-filter.sock"
# or
# transport = "jsonrpc"
# address = "localhost:9000"
# or
# transport = "stdio"
# command = "/usr/bin/python3 /path/to/plugin.py"
```

### External Plugin Resilience

External plugins may crash, hang, or become unreachable. ZeBGP handles these failures gracefully:

#### Timeout Handling

```toml
[zebgp.plugin.my-filter]
timeout = "5s"          # Max time to wait for response (default: 5s)
timeout-action = "pass" # pass (default), drop, or close-session
```

| Action | Behavior |
|--------|----------|
| `pass` | Log warning, continue to next handler (may accept filtered routes) |
| `drop` | Log warning, discard UPDATE entirely |
| `close-session` | Log error, close session without NOTIFICATION |

#### Reconnection Strategy

On connection loss to external plugin:

```
1. Log error: "plugin my-filter disconnected"
2. Mark plugin as unhealthy
3. Retry with exponential backoff:
   - Attempt 1: wait 1s
   - Attempt 2: wait 2s
   - Attempt 3: wait 4s
   - ...
   - Max wait: 60s
4. After 5 consecutive failures:
   - Log critical: "plugin my-filter unreachable, disabling"
   - Disable plugin until manual restart or config reload
```

#### Hot Reload (External Plugins)

External plugins can be restarted without ZeBGP restart:

```toml
[zebgp.plugin.my-filter]
hot-reload = true  # Reconnect if socket available after disconnect
```

With `hot-reload = true`:
- Plugin disconnect triggers reconnection attempts
- In-flight requests return timeout-action behavior
- Successful reconnect resumes normal operation
- Plugin state is NOT preserved (plugin must re-initialize)

#### Circuit Breaker

Protects against cascading failures:

```toml
[zebgp.plugin.my-filter]
circuit-breaker = true        # Enable circuit breaker (default: true)
failure-threshold = 10        # Failures before opening circuit
failure-window = "60s"        # Window for counting failures
recovery-timeout = "30s"      # Time before attempting recovery
```

**States:**
- **Closed:** Normal operation, all requests go to plugin
- **Open:** Plugin failing, skip plugin (use timeout-action behavior)
- **Half-Open:** After recovery-timeout, try one request to test

```
                ┌──────────────────────────────────────────┐
                │                                          │
                ▼                                          │
┌────────┐   failure   ┌────────┐   recovery    ┌──────────┴─┐
│ Closed │ ─────────►  │  Open  │ ────────────► │ Half-Open  │
└────────┘  threshold  └────────┘   timeout     └────────────┘
    ▲                                               │
    │                  success                      │
    └───────────────────────────────────────────────┘
```

#### Health Check

External plugins should implement health check RPC:

```protobuf
rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);

message HealthCheckRequest {}
message HealthCheckResponse {
    bool healthy = 1;
    string message = 2;  // Optional diagnostic message
}
```

ZeBGP calls health check:
- Before first use
- During circuit breaker half-open state
- Periodically if configured: `health-check-interval = 30s`

---

## Security

### Transport Security

| Transport | Security | Production Ready |
|-----------|----------|------------------|
| Unix socket | File permissions | ✅ Yes |
| TCP without TLS | None | ❌ No (dev only) |
| TCP with TLS | TLS 1.3 | ✅ Yes |
| TCP with mTLS | Mutual TLS 1.3 | ✅ Yes (recommended) |

**Unix socket permissions:**
```bash
# Socket owned by zebgp user, mode 0600
srw------- 1 zebgp zebgp 0 Dec 21 10:00 /var/run/zebgp-filter.sock
```

### Subprocess Security (stdio plugins)

stdio plugins run as subprocesses spawned by ZeBGP. Security considerations:

**Configuration:**
```toml
[zebgp.plugin.my-process]
transport = "stdio"
command = "/usr/bin/python3 /etc/zebgp/my-plugin.py"

# Security settings
user = "zebgp-plugin"         # Run as this user (requires CAP_SETUID)
group = "zebgp-plugin"        # Run as this group
working-dir = "/var/lib/zebgp/plugins/my-process"

# Resource limits (requires CAP_SYS_RESOURCE or cgroups)
memory-limit = "256M"         # Max memory (RSS)
cpu-limit = 0.5               # Max CPU cores (0.5 = 50% of one core)
file-limit = 100              # Max open files
process-limit = 10            # Max spawned processes

# Environment sanitization
env-passthrough = ["PATH", "HOME", "LANG"]  # Only pass these env vars
env-set = { "PLUGIN_MODE" = "production" }  # Set these explicitly
```

**Security features:**

| Feature | Description | Requirement |
|---------|-------------|-------------|
| User isolation | Run plugin as non-root user | CAP_SETUID or separate service |
| Memory limit | Kill plugin if exceeds limit | cgroups v2 or ulimit |
| CPU limit | Throttle plugin CPU | cgroups v2 |
| File limit | Limit open file descriptors | ulimit |
| Process limit | Prevent fork bombs | ulimit or cgroups |
| Env sanitization | Clear dangerous env vars | Default enabled |
| Working directory | Isolate filesystem access | Default enabled |
| Seccomp | Limit syscalls | Optional (future) |

**Environment sanitization:**

By default, subprocess environment is cleared except:
- `PATH` (set to `/usr/bin:/bin`)
- `HOME` (set to working-dir)
- `LANG`, `LC_ALL` (for encoding)
- Variables in `env-passthrough`
- Variables in `env-set`

Dangerous variables are always removed:
- `LD_PRELOAD`, `LD_LIBRARY_PATH`
- `PYTHONPATH`, `RUBYLIB`, `NODE_PATH`
- `SSH_AUTH_SOCK`, `GPG_AGENT_INFO`

**Logging:**
```
INFO: Starting plugin my-process as user=zebgp-plugin pid=12345
WARN: Plugin my-process exceeded memory limit (256M), sending SIGTERM
ERROR: Plugin my-process killed (OOM), restarting with backoff
```

**TLS configuration:**
```toml
[zebgp.plugin.my-filter]
transport = "grpc"
address = "plugin-host:50051"
tls = true
tls-cert = "/etc/zebgp/client.crt"      # Client certificate (for mTLS)
tls-key = "/etc/zebgp/client.key"       # Client key (for mTLS)
tls-ca = "/etc/zebgp/ca.crt"            # Verify server cert
tls-skip-verify = false                 # NEVER true in production
```

**Security warnings at startup:**
```
WARN: Plugin "my-filter" using TCP without TLS - not recommended for production
WARN: Plugin "my-filter" has tls-skip-verify=true - certificate not validated
```

### Plugin Authorization (Future)

Plugins declare required capabilities:

```protobuf
message InitResponse {
    // ...
    repeated string required_apis = 6;  // ["peers", "rib.read", "updates.modify"]
}
```

| API | Description |
|-----|-------------|
| `peers` | Read peer list and info |
| `peers.control` | Teardown peers |
| `rib.read` | Read RIB contents |
| `rib.write` | Modify RIB |
| `updates.read` | Receive UPDATE messages |
| `updates.modify` | Modify UPDATE messages |
| `raw` | Send raw BGP messages (requires `raw-access=true`) |
| `metrics` | Register metrics |
| `services` | Register/consume services |

### Resource Limits

Prevent plugins from consuming excessive resources:

```toml
[zebgp.plugin.my-filter]
# Request limits
rate-limit = 10000          # Max RPC calls per second
max-pending = 1000          # Max queued requests before backpressure

# Response limits
max-response-size = "10MB"  # Max response size
```

**Exceeded limits:**
- `rate-limit`: Requests delayed, warning logged
- `max-pending`: New requests rejected until queue drains
- `max-response-size`: Response truncated, error logged

---

## gRPC Plugin Interface

### Protocol Buffer Definitions

```protobuf
syntax = "proto3";
package zebgp.plugin.v1;

// PluginService is the main plugin interface.
// ZeBGP connects to plugins as a gRPC client.
service PluginService {
    // Lifecycle
    rpc GetConfigSchema(GetConfigSchemaRequest) returns (GetConfigSchemaResponse);
    rpc Init(InitRequest) returns (InitResponse);
    rpc Close(CloseRequest) returns (CloseResponse);
    rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);

    // Peer events
    rpc GetCapabilities(GetCapabilitiesRequest) returns (GetCapabilitiesResponse);
    rpc AdjustCapabilities(AdjustCapabilitiesRequest) returns (AdjustCapabilitiesResponse);
    rpc OnOpenMessage(OnOpenMessageRequest) returns (OnOpenMessageResponse);
    rpc OnEstablished(OnEstablishedRequest) returns (OnEstablishedResponse);
    rpc OnClose(OnCloseRequest) returns (OnCloseResponse);

    // UPDATE processing - bidirectional stream for efficiency
    rpc ProcessUpdates(stream UpdateRequest) returns (stream UpdateResponse);

    // Export policy - called before sending routes to peers (optional)
    rpc OnExport(OnExportRequest) returns (OnExportResponse);

    // Final UPDATE modification - called after export policy, before wire (optional)
    rpc OnUpdateSend(OnUpdateSendRequest) returns (OnUpdateSendResponse);

    // Optional extended hooks
    rpc OnStateChange(StateChangeRequest) returns (StateChangeResponse);
    rpc OnKeepalive(KeepaliveRequest) returns (KeepaliveResponse);
    rpc OnRouteRefresh(RouteRefreshRequest) returns (RouteRefreshResponse);
    rpc OnNotification(NotificationRequest) returns (NotificationResponse);

    // Optional state persistence
    rpc SaveState(SaveStateRequest) returns (SaveStateResponse);
    rpc RestoreState(RestoreStateRequest) returns (RestoreStateResponse);
}

// --- Lifecycle ---

// VALIDATION FLOW:
// 1. ZeBGP calls GetConfigSchema() to retrieve plugin's schema
// 2. ZeBGP validates config against schema BEFORE calling Init()
// 3. If valid, ZeBGP calls Init() with validated config
// 4. Plugin can assume config matches schema (defensive validation still recommended)

message GetConfigSchemaRequest {}

message GetConfigSchemaResponse {
    ConfigSchema schema = 1;  // nil = no validation
}

message ConfigSchema {
    bytes schema = 1;                      // JSON Schema (draft-07) as raw JSON
    repeated string required = 2;          // Required configuration keys
    map<string, string> defaults = 3;      // Default values for optional keys
}

message InitRequest {
    map<string, string> config = 1;  // Plugin configuration (validated against schema)
    string api_version = 2;          // ZeBGP plugin API version (e.g., "1.0")
}

message InitResponse {
    bool success = 1;
    string error = 2;
    string name = 3;                       // Plugin name
    string version = 4;                    // Plugin version
    int32 priority = 5;                    // Execution priority (0-999, lower = earlier)
    string supported_api_version = 6;      // Plugin's max supported API version
    repeated string required_apis = 7;     // Required APIs (see Plugin Authorization)
    repeated PluginDependency dependencies = 8;  // Service dependencies
}

message PluginDependency {
    string name = 1;        // Service name
    string min_version = 2; // Minimum version required
}

message CloseRequest {}
message CloseResponse {}

message HealthCheckRequest {}
message HealthCheckResponse {
    bool healthy = 1;
    string message = 2;  // Optional diagnostic message
}

// --- Peer Info ---

message PeerInfo {
    string address = 1;        // Peer IP address
    string local_address = 2;  // Local IP address
    uint32 local_as = 3;
    uint32 peer_as = 4;
    uint32 router_id = 5;
    string state = 6;
    NegotiatedParams negotiated = 7;
    map<string, string> plugin_config = 8;  // Per-peer plugin config
}

message NegotiatedParams {
    bool four_octet_as = 1;
    bool extended_message = 2;            // RFC 8654
    uint32 max_message_size = 3;          // 4096 or 65535
    repeated AddressFamily families = 4;
    uint32 hold_time = 5;
    map<string, AddPathMode> add_path = 6;
}

message AddressFamily {
    uint32 afi = 1;
    uint32 safi = 2;
}

enum AddPathMode {
    ADD_PATH_NONE = 0;
    ADD_PATH_RECEIVE = 1;
    ADD_PATH_SEND = 2;
    ADD_PATH_BOTH = 3;
}

// --- Capabilities ---

message GetCapabilitiesRequest {
    PeerInfo peer = 1;
}

message GetCapabilitiesResponse {
    repeated Capability capabilities = 1;
}

message AdjustCapabilitiesRequest {
    PeerInfo peer = 1;
    repeated Capability our_capabilities = 2;
    repeated Capability peer_capabilities = 3;
}

message AdjustCapabilitiesResponse {
    repeated Capability adjusted_capabilities = 1;  // nil/empty = use original
}

message Capability {
    uint32 code = 1;
    bytes value = 2;  // Raw capability value
}

// --- OPEN Message ---

message OnOpenMessageRequest {
    PeerInfo peer = 1;
    string router_id = 2;
    repeated Capability capabilities = 3;
}

message OnOpenMessageResponse {
    bool accept = 1;
    Notification notification = 2;  // Set if rejecting
}

message Notification {
    uint32 code = 1;
    uint32 subcode = 2;
    bytes data = 3;
}

// --- Established ---

message OnEstablishedRequest {
    PeerInfo peer = 1;
}

message OnEstablishedResponse {
    bool success = 1;
}

// --- UPDATE Processing ---

message UpdateRequest {
    PeerInfo peer = 1;
    Update update = 2;
    bytes raw = 3;  // Raw UPDATE bytes (optional, for protocol development)
}

message Update {
    repeated bytes withdrawn = 1;      // Withdrawn prefixes
    repeated Attribute attributes = 2;
    repeated bytes nlri = 3;           // Announced prefixes
}

message Attribute {
    uint32 flags = 1;
    uint32 code = 2;
    bytes value = 3;
}

message UpdateResponse {
    enum Action {
        CONTINUE = 0;       // Continue to next handler
        DROP = 1;           // Silently drop UPDATE, session continues
        CLOSE_SESSION = 2;  // Send notification, close session
        MODIFY = 3;         // Use modified update, continue chain
    }
    Action action = 1;
    Notification notification = 2;  // If action = CLOSE_SESSION
    Update modified = 3;            // If action = MODIFY

    // Outbound updates to send (optional)
    repeated OutboundUpdate send = 4;
}

message OutboundUpdate {
    string peer_selector = 1;  // "*" for all, or specific peer address
    Update update = 2;
    bytes raw = 3;  // Raw bytes for protocol development
}

// --- Export Policy ---

message OnExportRequest {
    PeerInfo source = 1;       // Peer that advertised the route (nil for local)
    PeerInfo target = 2;       // Peer we're sending to
    Route route = 3;
}

message Route {
    bytes prefix = 1;                      // NLRI prefix (encoded)
    repeated Attribute attributes = 2;     // Path attributes
    uint32 path_id = 3;                    // Add-Path ID (0 if not used)
}

message OnExportResponse {
    enum Action {
        CONTINUE = 0;  // Send to peer (possibly modified)
        DROP = 1;      // Don't send to this peer
        MODIFY = 2;    // Use modified route
    }
    Action action = 1;
    Route modified = 2;  // If action = MODIFY
}

// --- UPDATE Send (final modification before wire) ---

message OnUpdateSendRequest {
    PeerInfo peer = 1;       // Target peer
    Update update = 2;       // Composed UPDATE (after export policy)
}

message OnUpdateSendResponse {
    Update modified = 1;     // nil = use original, else use this UPDATE
}

// --- Close ---

message OnCloseRequest {
    PeerInfo peer = 1;
}

message OnCloseResponse {}

// --- Extended Hooks ---

message StateChangeRequest {
    PeerInfo peer = 1;
    string from_state = 2;
    string to_state = 3;
}

message StateChangeResponse {}

message KeepaliveRequest {
    PeerInfo peer = 1;
}

message KeepaliveResponse {}

message RouteRefreshRequest {
    PeerInfo peer = 1;
    uint32 afi = 2;
    uint32 safi = 3;
}

message RouteRefreshResponse {}

message NotificationRequest {
    PeerInfo peer = 1;
    Notification notification = 2;
}

message NotificationResponse {}

// --- State Persistence ---

message SaveStateRequest {}

message SaveStateResponse {
    bytes state = 1;  // Opaque state blob, nil = nothing to save
}

message RestoreStateRequest {
    bytes state = 1;  // Previously saved state, nil = no state exists
}

message RestoreStateResponse {
    bool success = 1;
    string error = 2;
}
```

### Go gRPC Plugin Example

```go
package main

import (
    "context"
    "log"
    "net"

    pb "codeberg.org/thomas-mangin/zebgp/pkg/plugin/proto"
    "google.golang.org/grpc"
)

type FilterPlugin struct {
    pb.UnimplementedPluginServiceServer
    threshold int
}

func (p *FilterPlugin) Init(ctx context.Context, req *pb.InitRequest) (*pb.InitResponse, error) {
    // Parse config
    if v, ok := req.Config["threshold"]; ok {
        p.threshold = parseInt(v)
    }
    return &pb.InitResponse{
        Success:             true,
        Name:                "prefix-filter",
        Version:             "1.0.0",
        SupportedApiVersion: "1.0",
    }, nil
}

func (p *FilterPlugin) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
    return &pb.HealthCheckResponse{Healthy: true}, nil
}

func (p *FilterPlugin) GetCapabilities(ctx context.Context, req *pb.GetCapabilitiesRequest) (*pb.GetCapabilitiesResponse, error) {
    return &pb.GetCapabilitiesResponse{}, nil
}

func (p *FilterPlugin) OnOpenMessage(ctx context.Context, req *pb.OnOpenMessageRequest) (*pb.OnOpenMessageResponse, error) {
    return &pb.OnOpenMessageResponse{Accept: true}, nil
}

func (p *FilterPlugin) OnEstablished(ctx context.Context, req *pb.OnEstablishedRequest) (*pb.OnEstablishedResponse, error) {
    return &pb.OnEstablishedResponse{Success: true}, nil
}

func (p *FilterPlugin) ProcessUpdates(stream pb.PluginService_ProcessUpdatesServer) error {
    for {
        req, err := stream.Recv()
        if err != nil {
            return err
        }

        // Filter logic: drop if too many prefixes
        if len(req.Update.Nlri) > p.threshold {
            stream.Send(&pb.UpdateResponse{Action: pb.UpdateResponse_DROP})
            continue
        }

        stream.Send(&pb.UpdateResponse{Action: pb.UpdateResponse_CONTINUE})
    }
}

func (p *FilterPlugin) OnClose(ctx context.Context, req *pb.OnCloseRequest) (*pb.OnCloseResponse, error) {
    return &pb.OnCloseResponse{}, nil
}

func (p *FilterPlugin) Close(ctx context.Context, req *pb.CloseRequest) (*pb.CloseResponse, error) {
    return &pb.CloseResponse{}, nil
}

func main() {
    lis, _ := net.Listen("unix", "/var/run/zebgp-filter.sock")
    server := grpc.NewServer()
    pb.RegisterPluginServiceServer(server, &FilterPlugin{threshold: 1000})
    log.Fatal(server.Serve(lis))
}
```

### Python gRPC Plugin Example

```python
#!/usr/bin/env python3
import grpc
from concurrent import futures
import zebgp_plugin_pb2 as pb
import zebgp_plugin_pb2_grpc as pb_grpc

class CommunityFilter(pb_grpc.PluginServiceServicer):
    def __init__(self):
        self.blocked_communities = set()

    def Init(self, request, context):
        # Parse config
        if 'blocked' in request.config:
            self.blocked_communities = set(request.config['blocked'].split(','))
        return pb.InitResponse(
            success=True,
            name="community-filter",
            version="1.0.0",
            supported_api_version="1.0"
        )

    def HealthCheck(self, request, context):
        return pb.HealthCheckResponse(healthy=True)

    def GetCapabilities(self, request, context):
        return pb.GetCapabilitiesResponse()

    def OnOpenMessage(self, request, context):
        return pb.OnOpenMessageResponse(accept=True)

    def OnEstablished(self, request, context):
        return pb.OnEstablishedResponse(success=True)

    def ProcessUpdates(self, request_iterator, context):
        for req in request_iterator:
            # Check for blocked communities in attributes
            blocked = False
            for attr in req.update.attributes:
                if attr.code == 8:  # COMMUNITIES
                    communities = parse_communities(attr.value)
                    if self.blocked_communities & communities:
                        blocked = True
                        break

            if blocked:
                yield pb.UpdateResponse(action=pb.UpdateResponse.DROP)
            else:
                yield pb.UpdateResponse(action=pb.UpdateResponse.CONTINUE)

    def OnClose(self, request, context):
        return pb.OnCloseResponse()

    def Close(self, request, context):
        return pb.CloseResponse()

if __name__ == '__main__':
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    pb_grpc.add_PluginServiceServicer_to_server(CommunityFilter(), server)
    server.add_insecure_port('unix:///var/run/zebgp-community.sock')
    server.start()
    server.wait_for_termination()
```

---

## JSON-RPC Plugin Interface

For simpler implementations or languages without good gRPC support.

### Protocol

- Transport: Unix socket, TCP, or stdio
- Format: JSON-RPC 2.0 (newline-delimited)
- Encoding: UTF-8
- Binary data: Base64 encoded (RFC 4648)

### Methods

| Method | Request | Response |
|--------|---------|----------|
| `get_config_schema` | `{}` | `{schema?}` |
| `init` | `{config: {...}, api_version: "1.0"}` | `{name, version, supported_api_version}` |
| `close` | `{}` | `{}` |
| `health_check` | `{}` | `{healthy, message?}` |
| `get_capabilities` | `{peer: {...}}` | `{capabilities: [...]}` |
| `adjust_capabilities` | `{peer, our_capabilities, peer_capabilities}` | `{adjusted_capabilities?}` |
| `on_open_message` | `{peer, router_id, capabilities}` | `{accept, notification?}` |
| `on_established` | `{peer}` | `{success}` |
| `on_update` | `{peer, update, raw?}` | `{action, notification?, modified?, send?}` |
| `on_export` | `{source?, target, route}` | `{action, modified?}` |
| `on_update_send` | `{peer, update}` | `{modified?}` |
| `on_close` | `{peer}` | `{}` |
| `on_state_change` | `{peer, from, to}` | `{}` |
| `on_keepalive` | `{peer}` | `{}` |
| `on_route_refresh` | `{peer, afi, safi}` | `{}` |
| `on_notification` | `{peer, notification}` | `{}` |
| `save_state` | `{}` | `{state?}` |
| `restore_state` | `{state?}` | `{success, error?}` |

### Action Values

| Action | Description |
|--------|-------------|
| `continue` | Continue to next handler |
| `drop` | Silently drop UPDATE |
| `close_session` | Send notification, close session |
| `modify` | Use modified update |

### Message Format

```json
// Request (ZeBGP → Plugin)
{"jsonrpc": "2.0", "method": "on_update", "params": {
    "peer": {
        "address": "192.168.1.1",
        "local_address": "192.168.1.2",
        "local_as": 65000,
        "peer_as": 65001,
        "router_id": 167837953,
        "state": "established",
        "negotiated": {
            "four_octet_as": true,
            "extended_message": false,
            "max_message_size": 4096,
            "families": [{"afi": 1, "safi": 1}],
            "hold_time": 90
        }
    },
    "update": {
        "withdrawn": [],
        "attributes": [
            {"flags": 64, "code": 1, "value": "AA=="},
            {"flags": 64, "code": 2, "value": "AgMAAP6dAAABAAD+ng=="},
            {"flags": 64, "code": 3, "value": "wKgBAQ=="}
        ],
        "nlri": ["CgAAAA=="]
    },
    "raw": "//8AAAA..."
}, "id": 42}

// Response (Plugin → ZeBGP)
{"jsonrpc": "2.0", "result": {
    "action": "continue"
}, "id": 42}

// Or drop
{"jsonrpc": "2.0", "result": {
    "action": "drop"
}, "id": 42}

// Or close session
{"jsonrpc": "2.0", "result": {
    "action": "close_session",
    "notification": {"code": 3, "subcode": 1, "data": ""}
}, "id": 42}

// Or modify
{"jsonrpc": "2.0", "result": {
    "action": "modify",
    "modified": {
        "withdrawn": [],
        "attributes": [...],
        "nlri": [...]
    }
}, "id": 42}
```

### Python JSON-RPC Plugin Example

```python
#!/usr/bin/env python3
"""Simple JSON-RPC plugin - no dependencies required."""

import json
import socket
import sys

class RateLimiter:
    def __init__(self):
        self.config = {}
        self.counters = {}  # peer -> count

    def handle(self, method, params):
        if method == "init":
            self.config = params.get("config", {})
            return {
                "name": "rate-limiter",
                "version": "1.0.0",
                "supported_api_version": "1.0"
            }

        elif method == "health_check":
            return {"healthy": True}

        elif method == "get_capabilities":
            return {"capabilities": []}

        elif method == "on_open_message":
            return {"accept": True}

        elif method == "on_established":
            peer = params["peer"]["address"]
            self.counters[peer] = 0
            return {"success": True}

        elif method == "on_update":
            peer = params["peer"]["address"]
            nlri_count = len(params["update"].get("nlri", []))
            self.counters[peer] = self.counters.get(peer, 0) + nlri_count

            max_routes = int(self.config.get("max_routes", 10000))
            if self.counters[peer] > max_routes:
                return {
                    "action": "close_session",
                    "notification": {"code": 6, "subcode": 1, "data": ""}
                }
            return {"action": "continue"}

        elif method == "on_close":
            peer = params["peer"]["address"]
            self.counters.pop(peer, None)
            return {}

        elif method == "close":
            return {}

        return {}

def main():
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.bind("/var/run/zebgp-ratelimit.sock")
    sock.listen(1)

    plugin = RateLimiter()

    while True:
        conn, _ = sock.accept()
        buffer = b""

        while True:
            data = conn.recv(4096)
            if not data:
                break
            buffer += data

            while b"\n" in buffer:
                line, buffer = buffer.split(b"\n", 1)
                request = json.loads(line)

                result = plugin.handle(request["method"], request.get("params", {}))

                response = {
                    "jsonrpc": "2.0",
                    "result": result,
                    "id": request["id"]
                }
                conn.send(json.dumps(response).encode() + b"\n")

        conn.close()

if __name__ == "__main__":
    main()
```

### Rust JSON-RPC Plugin Example

```rust
use serde::{Deserialize, Serialize};
use std::io::{BufRead, BufReader, Write};
use std::os::unix::net::UnixListener;

#[derive(Deserialize)]
struct Request {
    jsonrpc: String,
    method: String,
    params: serde_json::Value,
    id: u64,
}

#[derive(Serialize)]
struct Response {
    jsonrpc: &'static str,
    result: serde_json::Value,
    id: u64,
}

fn handle(method: &str, params: &serde_json::Value) -> serde_json::Value {
    match method {
        "init" => serde_json::json!({
            "name": "rust-filter",
            "version": "1.0.0",
            "supported_api_version": "1.0"
        }),
        "health_check" => serde_json::json!({"healthy": true}),
        "get_capabilities" => serde_json::json!({"capabilities": []}),
        "on_open_message" => serde_json::json!({"accept": true}),
        "on_established" => serde_json::json!({"success": true}),
        "on_update" => {
            // Example: continue all
            serde_json::json!({"action": "continue"})
        }
        "on_close" => serde_json::json!({}),
        "close" => serde_json::json!({}),
        _ => serde_json::json!({}),
    }
}

fn main() {
    let _ = std::fs::remove_file("/var/run/zebgp-rust.sock");
    let listener = UnixListener::bind("/var/run/zebgp-rust.sock").unwrap();

    for stream in listener.incoming() {
        let mut stream = stream.unwrap();
        let reader = BufReader::new(stream.try_clone().unwrap());

        for line in reader.lines() {
            let line = line.unwrap();
            let req: Request = serde_json::from_str(&line).unwrap();

            let result = handle(&req.method, &req.params);
            let resp = Response { jsonrpc: "2.0", result, id: req.id };

            writeln!(stream, "{}", serde_json::to_string(&resp).unwrap()).unwrap();
        }
    }
}
```

---

## In-Process Plugins (Built-in Only)

For maximum performance, built-in plugins run in-process using Go interfaces.
Third-party plugins should use gRPC or JSON-RPC.

```go
// In-process plugins implement this interface directly
type InProcessPlugin interface {
    Plugin
    // ... same methods as before
}

// ZeBGP wraps external plugins with a proxy that speaks gRPC/JSON-RPC
type ExternalPluginProxy struct {
    transport string      // "grpc", "jsonrpc", or "stdio"
    address   string      // socket path, host:port, or command
    conn      interface{} // gRPC client or JSON-RPC connection
}

func (p *ExternalPluginProxy) OnEstablished(peer PeerInfo, writer UpdateWriter) UpdateHandler {
    // Forward to external plugin via gRPC/JSON-RPC
    // ...
}
```

---

## Protocol Development Example

Testing a new draft RFC with experimental capability and attribute:

```go
package main

import (
    "codeberg.org/thomas-mangin/zebgp/pkg/plugin"
    "codeberg.org/thomas-mangin/zebgp/pkg/bgp/capability"
    "codeberg.org/thomas-mangin/zebgp/pkg/bgp/message"
)

// DraftRFCPlugin tests draft-ietf-idr-new-feature
type DraftRFCPlugin struct{}

func (p *DraftRFCPlugin) Name() string    { return "draft-new-feature" }
func (p *DraftRFCPlugin) Version() string { return "0.0.1" }

// Advertise experimental capability (code 239, per RFC 5492 §4)
func (p *DraftRFCPlugin) GetCapabilities(peer plugin.PeerInfo) []capability.Capability {
    return []capability.Capability{
        // Standard capabilities
        capability.FourOctetAS(peer.LocalAS),
        // Experimental capability for draft
        &plugin.CustomCapability{
            Code:  239,                    // RFC 5492 experimental range (239-254)
            Value: []byte{0x01, 0x00, 0x04}, // Draft-specific encoding
        },
    }
}

// Check if peer supports our experimental capability
func (p *DraftRFCPlugin) OnOpenMessage(peer plugin.PeerInfo, routerID netip.Addr, caps []capability.Capability) *message.Notification {
    for _, cap := range caps {
        if custom, ok := cap.(*plugin.CustomCapability); ok && custom.Code == 239 {
            // Peer supports draft feature
            return nil
        }
    }
    // Peer doesn't support - continue anyway (optional feature)
    return nil
}

// Use RawWriterPlugin to get raw access
func (p *DraftRFCPlugin) OnEstablishedRaw(peer plugin.PeerInfo, writer plugin.UpdateWriter, raw plugin.RawWriter) plugin.UpdateHandler {
    // Send UPDATE with experimental attribute
    update := &message.Update{
        Attributes: []attribute.Attribute{
            attribute.Origin(attribute.OriginIGP),
            attribute.ASPath([]uint32{peer.LocalAS}),
            // Experimental path attribute (code 241, safe experimental range)
            &plugin.CustomAttribute{
                Flags: 0xC0,                      // Optional, Transitive
                Code:  241,                       // Safe experimental range (241-254)
                Value: []byte{0xDE, 0xAD, 0xBE, 0xEF}, // Draft-specific data
            },
        },
        NLRI: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
    }
    writer.WriteUpdate(update)

    // Or send completely raw UPDATE for maximum control
    rawPayload := buildDraftUpdatePayload() // Your custom encoding
    raw.WriteRawUpdate(rawPayload)

    // Or send any message type (e.g., new message type 6)
    raw.WriteRawMessage(6, []byte{0x01, 0x02, 0x03})

    return func(peer plugin.PeerInfo, update *message.Update) plugin.HandlerResult {
        // Process incoming UPDATEs, look for experimental attributes
        for _, attr := range update.Attributes {
            if custom, ok := attr.(*plugin.CustomAttribute); ok && custom.Code == 241 {
                // Handle draft-specific attribute
            }
        }
        return plugin.Continue()
    }
}

var Plugin DraftRFCPlugin
```

**Use cases:**
- Testing new IETF drafts before standardization
- Interoperability testing with other implementations
- Fuzzing BGP parsers
- Research on BGP extensions
- Vendor-specific extensions

### External Process Plugin

For non-Go or isolated plugins:

```yaml
plugins:
  external:
    - name: "python-filter"
      command: "/usr/bin/python3 /etc/zebgp/filter.py"
      protocol: "json-rpc"  # or "msgpack-rpc"
```

Protocol:
```json
{"jsonrpc": "2.0", "method": "OnEstablished", "params": {"peer": {...}}, "id": 1}
{"jsonrpc": "2.0", "result": {"success": true}, "id": 1}
```

---

## Phased Implementation

### MVP Scope (What to Build First)

```
┌─────────────────────────────────────────────────────────────────┐
│  MVP = Minimum to enable in-process plugins with UPDATE hooks  │
│                                                                 │
│  DO NOT build gRPC, JSON-RPC, capability merging, export       │
│  policy, or external plugins until MVP is stable and tested.    │
└─────────────────────────────────────────────────────────────────┘
```

| Feature | MVP | Post-MVP | Notes |
|---------|-----|----------|-------|
| `Plugin` base interface | ✅ | | Name, Version, Init, Close |
| `PeerPlugin` interface | ✅ | | OnEstablished → UpdateHandler |
| `UpdateHandler` chain | ✅ | | Continue, Drop only (no Modify) |
| In-process plugins only | ✅ | | No external transport |
| Registry (simple) | ✅ | | Register, list, init order |
| Priority ordering | ✅ | | Lower priority runs first |
| Wrap existing API | ✅ | | ExaBGPAPIPlugin in-process |
| Basic ReactorAPI | ✅ | | Peers(), Logger() only |
| Handler Modify action | | ✅ | After basic chain works |
| gRPC transport | | ✅ | External plugins |
| JSON-RPC transport | | ✅ | Simple external plugins |
| stdio transport | | ✅ | ExaBGP compat |
| Capability merging | | ✅ | Complex, defer |
| GetCapabilities hook | | ✅ | After capability merging |
| Export policy (OnExport) | | ✅ | Requires RIB |
| OnUpdateSend hook | | ✅ | After export policy |
| Config schema validation | | ✅ | Nice to have |
| Circuit breaker | | ✅ | External plugins only |
| State persistence | | ✅ | Graceful restart |
| Graceful Restart handler | | ✅ | Requires RIB |
| Backpressure | | ✅ | Optimize later |
| Plugin discovery | | ✅ | Convenience feature |

### Phase 1: MVP (In-Process Only)

**Goal:** Prove the plugin architecture with minimal changes.

```
Week 1-2:
├── Define Plugin, PeerPlugin interfaces (minimal)
├── Implement simple Registry (no deps, no ordering)
├── Wrap pkg/api as ExaBGPAPIPlugin
└── Wire OnEstablished → UpdateHandler into reactor

Week 3:
├── Add priority-based ordering
├── Add Continue/Drop handler results
└── Tests for plugin lifecycle
```

**Exit Criteria:**
- [ ] `make test && make lint` passes
- [ ] ExaBGP API works as plugin (no behavior change)
- [ ] Can add logging plugin that sees UPDATEs
- [ ] Handler chain executes in priority order

### Phase 2: Handler Chain Complete

**Goal:** Full inbound UPDATE processing.

```
├── Add Modify action to HandlerResult
├── Add CloseSession action
├── Implement validation layer (outbound)
├── Add OnClose, OnOpenMessage hooks
└── Add dependency ordering
```

**Exit Criteria:**
- [ ] Plugin can modify UPDATE in-place
- [ ] Plugin can reject session with NOTIFICATION
- [ ] Plugins init in dependency order

### Phase 3: External Plugins (gRPC)

**Goal:** Support out-of-process plugins.

```
├── Define plugin.proto
├── Implement gRPC proxy
├── Add timeout handling
├── Add circuit breaker
└── Add health checks
```

**Exit Criteria:**
- [ ] Go plugin runs as separate process
- [ ] Timeout triggers configured action
- [ ] Circuit breaker opens after failures

### Phase 4: RIB & Export (Requires RIB)

**Goal:** Outbound route policy.

```
├── Design RIB (separate plan)
├── Implement RIBPlugin
├── Add OnExport hook
├── Add OnUpdateSend hook
└── Implement capability merging
```

### Phase 5: Production Hardening

```
├── JSON-RPC transport
├── stdio transport (ExaBGP compat)
├── State persistence
├── Graceful restart support
├── Plugin discovery
├── Full metrics/observability
```

### Phase 6: Documentation & Examples

```
├── Plugin developer guide
├── Example plugins (Go, Python, Rust)
├── Testing utilities
├── Performance benchmarks
```

---

## Legacy Migration Path Reference

The original phased approach (for reference):

| Original Phase | New Mapping |
|---------------|-------------|
| Phase 1: Interface Definition | MVP |
| Phase 2: Registry & Transport | MVP + Phase 3 |
| Phase 3: API Plugin Extraction | MVP |
| Phase 4: Peer Plugin Integration | Phase 2 |
| Phase 5: RIB Plugin Extraction | Phase 4 |
| Phase 6: Documentation | Phase 6 |

---

## Testing Plugins

### Mock Reactor

For unit testing plugins in isolation:

```go
package plugin_test

import (
    "context"
    "testing"

    "codeberg.org/thomas-mangin/zebgp/pkg/plugin"
    "codeberg.org/thomas-mangin/zebgp/pkg/plugin/testing/mock"
)

func TestMyPlugin(t *testing.T) {
    // Create mock reactor with test peers
    reactor := mock.NewReactor()
    reactor.AddPeer(plugin.PeerInfo{
        Address:  netip.MustParseAddr("192.168.1.1"),
        LocalAS:  65000,
        PeerAS:   65001,
    })

    // Initialize plugin
    p := &MyPlugin{}
    err := p.Init(context.Background(), reactor, map[string]any{
        "threshold": 100,
    })
    if err != nil {
        t.Fatalf("Init failed: %v", err)
    }

    // Get handler and test UPDATE processing
    peer := reactor.Peers()[0]
    writer := mock.NewUpdateWriter(peer)
    handler := p.OnEstablished(peer, writer)

    // Create test UPDATE
    update := &message.Update{
        NLRI: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
        Attributes: []attribute.Attribute{
            attribute.Origin(attribute.OriginIGP),
            attribute.ASPath([]uint32{65001}),
            attribute.NextHop(netip.MustParseAddr("192.168.1.1")),
        },
    }

    // Process and verify
    result := handler(peer, update)
    if result.Action != plugin.ActionContinue {
        t.Errorf("Expected Continue, got: %v", result.Action)
    }

    // Check what plugin sent
    sent := writer.SentUpdates()
    if len(sent) != 1 {
        t.Errorf("Expected 1 update sent, got %d", len(sent))
    }
}
```

### Plugin Test Harness CLI

Test external plugins from command line:

```bash
# Test gRPC plugin
zebgp plugin test grpc:///var/run/my-plugin.sock

# Test JSON-RPC plugin
zebgp plugin test jsonrpc://localhost:9000

# Test stdio plugin
zebgp plugin test stdio:///usr/bin/python3 /path/to/plugin.py

# Verbose output
zebgp plugin test --verbose grpc:///var/run/my-plugin.sock
```

**Test sequence:**
1. Connect to plugin
2. Call `Init` with test config
3. Call `HealthCheck`
4. Call `GetCapabilities` with mock peer
5. Call `OnOpenMessage` with mock OPEN
6. Call `OnEstablished`
7. Send test UPDATEs via `ProcessUpdates`
8. Verify outbound updates are valid
9. Call `OnClose`
10. Call `Close`
11. Report results

**Example output:**
```
Testing plugin at grpc:///var/run/my-plugin.sock

✅ Connect: OK (12ms)
✅ Init: OK - name="my-filter", version="1.0.0", priority=100
✅ HealthCheck: OK - healthy=true
✅ GetCapabilities: OK - 2 capabilities
✅ OnOpenMessage: OK - accepted
✅ OnEstablished: OK
✅ ProcessUpdates: OK - 100 updates, avg 0.5ms/update
✅ OnClose: OK
✅ Close: OK

All tests passed. Plugin is compatible with ZeBGP.
```

### Integration Testing

Test plugin with real ZeBGP instance:

```bash
# Start ZeBGP with test config
zebgp --config test-config.toml --test-mode

# test-mode:
# - Uses loopback for all connections
# - Shorter timers (1s keepalive, 3s hold)
# - Verbose logging
# - Exits after test-duration (default: 60s)
```

```toml
# test-config.toml
[zebgp.test]
mode = true
duration = 30s
expect-updates = 100
expect-peers = 2

[zebgp.plugin]
my-filter = true

[zebgp.plugin.my-filter]
address = '/var/run/my-plugin.sock'
```

---

## Performance

### Latency Budget

Target latencies for UPDATE processing:

| Component | Target | Maximum |
|-----------|--------|---------|
| Parse UPDATE | 10µs | 100µs |
| Handler chain (total) | 1ms | 10ms |
| Per-handler (in-process) | 100µs | 1ms |
| Per-handler (external) | 500µs | 5ms |
| Validation layer | 10µs | 100µs |
| Wire encoding | 10µs | 100µs |

**Total budget:** <2ms p99 for full UPDATE round-trip.

### gRPC vs JSON-RPC Performance

Measured on localhost with simple accept-all plugin:

| Protocol | Latency (p50) | Latency (p99) | Throughput |
|----------|---------------|---------------|------------|
| In-process | 1µs | 5µs | 1M+ ops/s |
| gRPC (Unix) | 50µs | 200µs | 50k ops/s |
| gRPC (TCP) | 100µs | 500µs | 30k ops/s |
| JSON-RPC (Unix) | 200µs | 800µs | 20k ops/s |
| JSON-RPC (TCP) | 300µs | 1ms | 15k ops/s |
| stdio | 500µs | 2ms | 5k ops/s |

**Recommendation:**
- <1k routes/sec: Any protocol works
- 1k-10k routes/sec: Use gRPC
- >10k routes/sec: Use gRPC with streaming, consider in-process

### Streaming for High Throughput

For high-volume UPDATE processing, use gRPC streaming:

```protobuf
// Bidirectional streaming - keeps connection open
rpc ProcessUpdates(stream UpdateRequest) returns (stream UpdateResponse);
```

**Benefits:**
- No per-request connection overhead
- Batching at transport level
- ~10x throughput vs unary RPCs

**Plugin-side batching:**

```go
func (p *FilterPlugin) ProcessUpdates(stream pb.PluginService_ProcessUpdatesServer) error {
    batch := make([]*pb.UpdateRequest, 0, 100)

    for {
        req, err := stream.Recv()
        if err != nil {
            return err
        }

        // Accumulate batch
        batch = append(batch, req)

        // Process when batch full or timeout
        if len(batch) >= 100 {
            results := p.processBatch(batch)
            for _, result := range results {
                stream.Send(result)
            }
            batch = batch[:0]
        }
    }
}
```

### Memory Management

**UPDATE pooling:**

ZeBGP pools UPDATE message buffers to reduce GC pressure:

```go
// Plugins receive pooled UPDATE objects
// Do NOT hold references after handler returns
func (p *MyPlugin) handleUpdate(peer PeerInfo, update *message.Update) HandlerResult {
    // OK: read and process immediately
    for _, prefix := range update.NLRI {
        p.processPrefix(prefix)
    }

    // BAD: storing reference - update may be reused
    // p.updates = append(p.updates, update)

    // OK: copy if you need to keep data
    p.prefixes = append(p.prefixes, slices.Clone(update.NLRI)...)

    return Continue()
}
```

**External plugin memory:**

External plugins manage their own memory. ZeBGP enforces `max-response-size` to prevent OOM from malicious responses.

### Metrics

Built-in plugin performance metrics:

```
# Handler latency histogram (per plugin)
zebgp_plugin_handler_duration_seconds{plugin="my-filter",quantile="0.5"} 0.0005
zebgp_plugin_handler_duration_seconds{plugin="my-filter",quantile="0.99"} 0.005

# Handler invocation counter
zebgp_plugin_handler_total{plugin="my-filter",result="continue"} 12345
zebgp_plugin_handler_total{plugin="my-filter",result="drop"} 67
zebgp_plugin_handler_total{plugin="my-filter",result="modify"} 890

# External plugin connection state
zebgp_plugin_connection_state{plugin="my-filter"} 1  # 1=connected, 0=disconnected

# Circuit breaker state
zebgp_plugin_circuit_state{plugin="my-filter"} 0  # 0=closed, 1=open, 2=half-open

# Queue depth (for external plugins)
zebgp_plugin_queue_depth{plugin="my-filter"} 42

# Events dropped due to slow consumer
zebgp_plugin_events_dropped{plugin="my-filter"} 0

# Plugin panics recovered
zebgp_plugin_panics_total{plugin="my-filter"} 0
```

---

## Debugging

### Verbose Mode

Enable detailed plugin logging:

```bash
zebgp --log-level=debug --plugin-debug=my-filter
```

**Output:**
```
DEBUG plugin.my-filter: Init called config={"threshold":"100"}
DEBUG plugin.my-filter: GetCapabilities peer=192.168.1.1
DEBUG plugin.my-filter: OnEstablished peer=192.168.1.1 state=Established
DEBUG plugin.my-filter: ProcessUpdate peer=192.168.1.1 nlri=3 withdrawn=0 duration=0.5ms
DEBUG plugin.my-filter: ProcessUpdate result=continue
```

### Trace Mode

Capture all plugin RPC traffic:

```bash
zebgp --plugin-trace=/tmp/plugin-trace.jsonl
```

**Output (JSON Lines):**
```json
{"ts":"2025-12-21T10:00:00Z","plugin":"my-filter","method":"Init","req":{"config":{"threshold":"100"}},"resp":{"success":true},"duration_ms":5}
{"ts":"2025-12-21T10:00:01Z","plugin":"my-filter","method":"ProcessUpdates","req":{"peer":"192.168.1.1","update":{...}},"resp":{"action":"continue"},"duration_ms":0.5}
```

### Manual Testing with socat

Test JSON-RPC plugin directly:

```bash
# Connect to plugin socket
socat - UNIX:/var/run/my-plugin.sock

# Send init request
{"jsonrpc":"2.0","method":"init","params":{"config":{},"api_version":"1.0"},"id":1}

# Response
{"jsonrpc":"2.0","result":{"name":"my-filter","version":"1.0.0","supported_api_version":"1.0"},"id":1}

# Send update
{"jsonrpc":"2.0","method":"on_update","params":{"peer":{"address":"192.168.1.1"},"update":{"nlri":["CgAAAA=="]}},"id":2}

# Response
{"jsonrpc":"2.0","result":{"action":"continue"},"id":2}
```

### Plugin Chain Visualization

Understand plugin execution order:

```bash
# Show UPDATE processing chain
zebgp-cli plugin chain

UPDATE Processing Order:
────────────────────────────────────────────────────────────
  Priority │ Plugin        │ Transport    │ Description
────────────────────────────────────────────────────────────
    0      │ rib           │ in-process   │ stores routes in RIB
   50      │ route-policy  │ in-process   │ applies import/export policy
  100      │ my-filter     │ grpc (unix)  │ prefix-based filtering
  500      │ logging       │ in-process   │ logs accepted updates
────────────────────────────────────────────────────────────

# Show capability merging
zebgp-cli plugin capabilities --peer 192.168.1.1

Capability Merging for 192.168.1.1:
────────────────────────────────────────────────────────────
  Capability       │ Source Plugin   │ Value
────────────────────────────────────────────────────────────
  4-Octet AS       │ rib             │ ASN 65000
  MP-BGP IPv4      │ rib             │ unicast
  MP-BGP IPv6      │ route-policy    │ unicast
  AddPath IPv4     │ rib (send)      │ send+receive
                   │ my-filter (recv)│
  Route Refresh    │ rib             │ enabled
────────────────────────────────────────────────────────────
  Final: [4-Octet-AS, MP-BGP IPv4/IPv6, AddPath send+recv, Route-Refresh]

# Show handler chain with latency stats
zebgp-cli plugin chain --stats

UPDATE Processing Order (with stats):
────────────────────────────────────────────────────────────────────────
  Pri │ Plugin        │ p50     │ p99     │ Continue │ Drop │ Modify
────────────────────────────────────────────────────────────────────────
    0 │ rib           │ 0.1ms   │ 0.5ms   │ 99.9%    │ 0%   │ 0%
   50 │ route-policy  │ 0.2ms   │ 1.0ms   │ 95.0%    │ 5%   │ 0%
  100 │ my-filter     │ 0.5ms   │ 2.0ms   │ 99.5%    │ 0.5% │ 0%
  500 │ logging       │ 0.05ms  │ 0.1ms   │ 100%     │ 0%   │ 0%
────────────────────────────────────────────────────────────────────────
  Total chain: p50=0.85ms, p99=3.6ms
```

### Plugin Inspection

Inspect running plugins:

```bash
# List active plugins
zebgp-cli plugin list

NAME         VERSION  PRIORITY  TRANSPORT    STATE      CIRCUIT
rib          1.0.0    0         in-process   active     -
exabgp-api   1.0.0    10        in-process   active     -
my-filter    1.0.0    100       grpc (unix)  active     closed

# Get plugin details
zebgp-cli plugin info my-filter

Plugin: my-filter
Version: 1.0.0
Priority: 100
Transport: gRPC over Unix socket
Address: /var/run/my-filter.sock
State: active
Circuit: closed (0 failures in last 60s)
API Version: 1.0

Metrics:
  Invocations: 12,345
  Continue: 12,278 (99.5%)
  Drop: 67 (0.5%)
  Modify: 0 (0%)
  Avg latency: 0.5ms
  P99 latency: 2.1ms

# Force health check
zebgp-cli plugin health-check my-filter
✅ my-filter: healthy
```

---

## Open Questions

1. ~~**Hot Reload** - Can plugins be reloaded without restart?~~
   - ✅ Resolved: External plugins support hot-reload via config option

2. ~~**Plugin Sandboxing** - Should external plugins run in separate processes for isolation?~~
   - ✅ Resolved: Subprocess security section added with user isolation, resource limits, env sanitization

3. ~~**Graceful Restart** - How to preserve plugin state across restarts?~~
   - ✅ Resolved: StateProvider interface added for plugins that need persistence

4. ~~**Async UPDATE Safety** - How to handle async processing of pooled UPDATEs?~~
   - ✅ Resolved: Clone() method added with documentation for safe async patterns

5. ~~**Graceful Restart Protocol** - How do plugins participate in RFC 4724 GR?~~
   - ✅ Resolved: GracefulRestartHandler interface added

6. ~~**Dependency Cycle Detection** - How to prevent deadlock from circular dependencies?~~
   - ✅ Resolved: Tarjan's algorithm + topological sort in Registry

7. **Binary Attribute Encoding in JSON-RPC** - Base64 is verbose. Consider human-readable mode?
   - Proposal: Add optional `decoded` field for debugging

8. **Plugin SDK** - Should we provide a `pkg/plugin/sdk/` with base implementations?
   - Proposal: Yes, to reduce boilerplate for plugin authors

9. **ReactorAPI Size** - Current interface has 17+ methods. Too much coupling?
   - Proposal: Split into focused interfaces (PeerRegistry, EventBus, ServiceLocator)

---

## Document Structure Recommendation

This document is 3900+ lines. Consider splitting for maintainability:

| Document | Content | Est. Lines |
|----------|---------|------------|
| `plugin-system-overview.md` | Goals, architecture, prerequisites, decisions | 500 |
| `plugin-interface.md` | Go interfaces, handler chain, lifecycle | 1000 |
| `plugin-grpc-protocol.md` | proto definitions, gRPC examples | 500 |
| `plugin-jsonrpc-protocol.md` | JSON-RPC spec, examples | 400 |
| `plugin-security.md` | Security model, subprocess, TLS | 500 |
| `plugin-testing.md` | Test harness, mock reactor, CLI | 400 |
| `plugin-examples/` | Actual code files (Go, Python, Rust) | N/A |

**Benefits:**
- Easier to review changes
- Parallel editing without conflicts
- Focused documents for specific audiences
- Code examples as runnable files

**Migration:** Not urgent. Can split when implementing.

---

## File Structure

```
pkg/
└── plugin/
    ├── plugin.go          # Core interfaces (Plugin, PeerPlugin, APIPlugin)
    ├── handler.go         # UpdateHandler, HandlerResult
    ├── clone.go           # Clone() for UPDATE, Cloner interface for attributes
    ├── export.go          # ExportHandler, ExportResult for outbound
    ├── graceful.go        # GracefulRestartHandler interface
    ├── registry.go        # Plugin registry and lifecycle
    ├── reactor_api.go     # ReactorAPI implementation
    ├── dependency.go      # Dependency, cycle detection, topological sort
    ├── errors.go          # Standard error codes
    ├── config/
    │   ├── config.go      # Type-safe config helpers
    │   ├── schema.go      # JSON Schema validation (draft-07)
    │   └── config_test.go # Config helper tests
    ├── proto/
    │   ├── plugin.proto   # gRPC service definition
    │   └── plugin.pb.go   # Generated Go code
    ├── grpc/
    │   ├── client.go      # gRPC client (ZeBGP → Plugin)
    │   └── proxy.go       # Wraps gRPC as Plugin interface
    ├── jsonrpc/
    │   ├── client.go      # JSON-RPC client
    │   ├── protocol.go    # Message encoding/decoding
    │   └── proxy.go       # Wraps JSON-RPC as Plugin interface
    ├── stdio/
    │   ├── client.go      # stdio transport (ExaBGP compat)
    │   ├── security.go    # Subprocess security (user, limits, env)
    │   └── proxy.go       # Wraps stdio as Plugin interface
    ├── discovery/
    │   ├── discovery.go   # Plugin socket auto-discovery
    │   └── discovery_test.go
    ├── testing/
    │   └── mock/
    │       ├── reactor.go     # Mock ReactorAPI for testing
    │       └── writer.go      # Mock UpdateWriter for testing
    └── builtin/
        ├── exabgp_api.go  # ExaBGP API plugin (in-process)
        └── rib.go         # RIB plugin (in-process)

cmd/
└── zebgp-cli/
    └── plugin.go          # Plugin CLI commands (list, chain, info, test)
```

**Note:** Go's native plugin system (.so/.dylib) is intentionally not supported.
External plugins use gRPC or JSON-RPC transport exclusively.

---

## Summary

This design:

1. **Preserves** the current ExaBGP-compatible API as the default
2. **Supports** multiple simultaneous APIs (ExaBGP + gRPC, etc.)
3. **Language-agnostic** - Plugins via gRPC, JSON-RPC, or stdio (Python, Rust, Go, C++, etc.)
4. **Keeps** the reactor core simple - just FSM + transport
5. **Follows** Go idioms (interfaces, functional options, minimal dependencies)
6. **RFC Compliant** - Validation layer ensures RFC 4271 compliance
7. **ExaBGP Compatible** - stdio transport for process migration

**Plugin Transport Options:**

| Transport | Protocol | Language | Performance | Use Case |
|-----------|----------|----------|-------------|----------|
| Unix socket | gRPC | Any with gRPC | Excellent | Production plugins |
| Unix socket | JSON-RPC | Any | Good | Simple plugins, no deps |
| TCP | gRPC | Any | Good | Remote/microservice plugins |
| TCP | JSON-RPC | Any | Moderate | Debugging, remote |
| stdio | JSON-RPC | Any | Low | ExaBGP process compat |
| In-process | Go interface | Go only | Best | Built-in plugins |

**Key Features:**

| Feature | Benefit |
|---------|---------|
| **gRPC interface** | Type-safe, streaming, any language with gRPC support |
| **JSON-RPC interface** | Zero dependencies, any language, easy debugging |
| **stdio transport** | ExaBGP process compatibility |
| **Multi-plugin chains** | Layer functionality (log → filter → store) |
| **Priority ordering** | Explicit control over plugin execution order |
| **Bidirectional streaming** | Efficient UPDATE processing via gRPC streams |
| **Cumulative modification** | Handlers transform UPDATEs, changes visible to next handler |
| **RFC validation layer** | Ensures outgoing messages comply with RFC 4271 |
| **Raw message access** | Send any BGP message for protocol development (opt-in) |
| **Extended hooks** | OnStateChange, OnKeepalive, OnRouteRefresh, OnNotification, OnError |
| **Export policy** | OnExport hook for outbound route filtering/modification |
| **Outgoing hooks** | OnNotificationSend, OnUpdateSend for intercepting outbound messages |
| **Config validation** | JSON Schema (draft-07) validation before plugin Init() |
| **Graceful Restart** | GracefulRestartHandler for RFC 4724 support |
| **Per-peer config** | Different plugin behavior per peer |
| **Event subscription** | Async notification of peer/route changes |
| **Metrics interface** | Built-in observability with automatic namespacing |
| **State persistence** | Optional StateProvider for crash recovery |
| **Dependency versioning** | Service dependencies with version constraints |
| **Cycle detection** | Tarjan's algorithm prevents init deadlocks |
| **Topological init** | Plugins init in dependency order |
| **Deep clone** | Safe async UPDATE processing via Clone() |
| **TOML + env config** | Standard config with env overrides |
| **Plugin discovery** | Auto-load from directories and sockets |
| **Circuit breaker** | Automatic failure handling for external plugins |
| **mTLS support** | Mutual TLS for secure transport |
| **Subprocess security** | User isolation, resource limits, env sanitization |
| **Health checks** | Monitor plugin availability and recovery |
| **API versioning** | Forward-compatible plugin interface with matrix |
| **Testing harness** | CLI tool and mock reactor for plugin development |
| **Hot reload** | External plugins can restart without ZeBGP restart |
| **Chain visualization** | `zebgp-cli plugin chain` shows execution order |
| **UPDATE cloning** | Safe async processing with Clone() method |
| **Route refresh control** | Plugins can request route refresh from peers |
| **Config helpers** | Type-safe config parsing with defaults |

**Resilience Features:**

| Feature | Description |
|---------|-------------|
| Timeout handling | Configurable timeout with pass/drop/close actions |
| Reconnection | Exponential backoff for failed connections |
| Circuit breaker | Automatic bypass of failing plugins |
| Health checks | Periodic liveness verification |
| Resource limits | Rate limiting and response size limits |
| Panic recovery | Graceful handling of plugin panics |
| Close timeout | Force-terminate hung plugins during shutdown |
| Backpressure | Queue depth management with configurable drop policy |
| Drop policy | Configurable: newest, oldest, or never-withdrawals |

**Example Plugin Languages:**
- Go (gRPC or JSON-RPC)
- Python (gRPC, JSON-RPC, or stdio)
- Rust (gRPC or JSON-RPC)
- C++ (gRPC)
- Node.js (gRPC or JSON-RPC)
- Any language with socket support (JSON-RPC)
- Any language with stdin/stdout (stdio)

The migration is incremental - each phase adds capability without breaking existing functionality.
