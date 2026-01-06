# Spec: Unified Source Registry

## Task

Create a registry that assigns unique numeric IDs to all message sources (peers, API processes). Store compact IDs in messages instead of IP addresses or strings.

## Motivation

Current state:
- `ReceivedUpdate.SourcePeerIP` stores peer IP (16 bytes for IPv6)
- API-originated messages have no source tracking
- No unified way to identify message origin

Proposed:
- Single `SourceID` (2 bytes) identifies any source
- Registry maps ID → source metadata
- Compact storage, fast comparison, unified model

## Design

### Source Types

```go
type SourceType uint8

const (
    SourceUnknown SourceType = iota
    SourcePeer    // BGP peer
    SourceAPI     // API process
    SourceConfig  // Static routes from config
)
```

### Source Entry

```go
type Source struct {
    Type   SourceType
    Active bool        // false when peer/process removed

    // Type-specific fields
    PeerIP netip.Addr  // SourcePeer
    PeerAS uint32      // SourcePeer
    Name   string      // SourceAPI, SourceConfig
}

func (s Source) String() string {
    switch s.Type {
    case SourcePeer:
        return "peer:" + s.PeerIP.String()
    case SourceAPI:
        return "api:" + s.Name
    case SourceConfig:
        return "config"
    default:
        return "unknown"
    }
}
```

### Source ID

```go
type SourceID uint16

const InvalidSourceID SourceID = 0
```

Using `uint16`:
- 65535 possible sources (plenty for any deployment)
- Matches `bgpctx.ContextID` pattern
- Compact storage (2 bytes)

### Registry

```go
type Registry struct {
    mu      sync.RWMutex
    sources []Source              // indexed by SourceID (0 = invalid)

    // Reverse indexes for O(1) lookup by identifier
    peerIdx map[netip.Addr]SourceID
    apiIdx  map[string]SourceID
}

// Global instance
var DefaultRegistry = NewRegistry()

func NewRegistry() *Registry

// Registration
func (r *Registry) RegisterPeer(ip netip.Addr, as uint32) SourceID
func (r *Registry) RegisterAPI(name string) SourceID
func (r *Registry) RegisterConfig(name string) SourceID

// Lookup by ID (O(1) - slice index)
func (r *Registry) Get(id SourceID) (Source, bool)

// Lookup by identifier (O(1) - map lookup)
func (r *Registry) GetByPeerIP(ip netip.Addr) (SourceID, bool)
func (r *Registry) GetByAPIName(name string) (SourceID, bool)

// Lifecycle
func (r *Registry) Deactivate(id SourceID)  // marks inactive, keeps entry
func (r *Registry) IsActive(id SourceID) bool

// Formatting
func (r *Registry) String(id SourceID) string  // "peer:10.0.0.1"
```

### ID Lifecycle

**Registration:**
- Peers: When peer is created in reactor
- API: When process starts
- Config: When static routes loaded

**Deactivation:**
- Peers: When peer is removed
- API: When process dies
- IDs are **never reused** (keeps historical sources resolvable)

### Integration Points

#### WireUpdate

```go
type WireUpdate struct {
    payload     []byte
    sourceCtxID bgpctx.ContextID
    messageID   uint64
    sourceID    source.SourceID  // NEW
    // ...
}

func (u *WireUpdate) SourceID() source.SourceID
func (u *WireUpdate) SetSourceID(id source.SourceID)
```

#### ReceivedUpdate

```go
type ReceivedUpdate struct {
    // SourcePeerIP netip.Addr  // REMOVED - use WireUpdate.SourceID()
    WireUpdate   *api.WireUpdate
    // ...
}
```

#### Peer

```go
type Peer struct {
    sourceID source.SourceID  // assigned at creation
    // ...
}

func (p *Peer) SourceID() source.SourceID
```

#### Process

```go
type Process struct {
    sourceID source.SourceID  // assigned at start
    // ...
}

func (p *Process) SourceID() source.SourceID
```

### Relationship: MessageID + SourceID

Both IDs are stored in `WireUpdate`:
- `messageID` - unique per message, used for `forward update-id`
- `sourceID` - identifies the sender

When looking up a cached message by ID:
```go
update, ok := cache.Get(messageID)
if ok {
    sourceID := update.WireUpdate.SourceID()
    source := source.Registry.Get(sourceID)
    // Now have both the message and its source
}
```

This enables:
- Forward to all except source: `peer !<source> forward update-id <id>`
- Policy based on source type: treat peer vs API differently

### JSON Format

Add `source` to message wrapper:

```json
{
  "message": {
    "type": "update",
    "id": 123,
    "source": "peer:10.0.0.1"
  },
  "direction": "received",
  ...
}
```

For API-originated:
```json
{
  "message": {
    "type": "update",
    "source": "api:rr-plugin"
  },
  ...
}
```

Source is resolved from ID at output time (not stored as string).

### Package Structure

```
pkg/source/
├── source.go       # SourceType, Source, SourceID
├── registry.go     # Registry implementation
└── registry_test.go
```

## Files to Modify

| File | Changes |
|------|---------|
| `pkg/source/source.go` | NEW: types |
| `pkg/source/registry.go` | NEW: registry |
| `pkg/source/registry_test.go` | NEW: tests |
| `pkg/api/wire_update.go` | Add sourceID field |
| `pkg/api/json.go` | Add source to message wrapper |
| `pkg/api/text.go` | Add source to output |
| `pkg/reactor/peer.go` | Register peer, store sourceID |
| `pkg/reactor/reactor.go` | Set sourceID on WireUpdate |
| `pkg/reactor/received_update.go` | Remove SourcePeerIP |
| `pkg/api/process.go` | Register API, store sourceID |

## Implementation Steps

1. **Create source package**
   - Define types (SourceType, Source, SourceID)
   - Implement Registry with registration/lookup
   - Write tests

2. **Add sourceID to WireUpdate**
   - Add field and methods
   - Keep SourcePeerIP in ReceivedUpdate temporarily

3. **Register peers**
   - Peer stores sourceID
   - Reactor sets sourceID on WireUpdate

4. **Register API processes**
   - Process stores sourceID
   - Set sourceID on API-originated messages

5. **Update formatters**
   - Add source to message wrapper in JSON
   - Resolve ID to string at output time

6. **Remove SourcePeerIP**
   - Remove from ReceivedUpdate
   - Update all usages to use WireUpdate.SourceID()

7. **Update tests**

## TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestSourceTypeString` | `source_test.go` | SourceType.String() |
| `TestSourceString` | `source_test.go` | Source.String() formats |
| `TestRegistryRegisterPeer` | `registry_test.go` | Peer registration |
| `TestRegistryRegisterAPI` | `registry_test.go` | API registration |
| `TestRegistryGet` | `registry_test.go` | Lookup by ID |
| `TestRegistryGetByPeerIP` | `registry_test.go` | Lookup by peer IP |
| `TestRegistryDeactivate` | `registry_test.go` | Deactivation |
| `TestRegistryNeverReuse` | `registry_test.go` | IDs not reused |
| `TestRegistryConcurrent` | `registry_test.go` | Thread safety |
| `TestWireUpdateSourceID` | `wire_update_test.go` | SourceID get/set |
| `TestJSONOutputSource` | `json_test.go` | Source in output |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] `.claude/zebgp/api/` updated
