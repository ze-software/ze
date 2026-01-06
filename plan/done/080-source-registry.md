# Spec: Unified Source Registry

## Task

Create a registry that assigns unique numeric IDs to all message sources (peers, API processes). Store compact IDs in messages instead of IP addresses or strings.

## Required Reading

- [ ] `.claude/zebgp/api/ARCHITECTURE.md` - API process lifecycle, message flow
- [ ] `.claude/zebgp/api/JSON_FORMAT.md` - JSON output format for source field
- [ ] `.claude/zebgp/ENCODING_CONTEXT.md` - WireUpdate structure, zero-copy patterns
- [ ] `.claude/zebgp/UPDATE_BUILDING.md` - Message flow, Build vs Forward paths

**Key insights:**
- WireUpdate already has `sourceCtxID` and `messageID` fields - add `sourceID` following same pattern
- API processes need lifecycle tracking (start/stop) - registry must handle deactivation
- JSON output resolves IDs to strings at output time (not stored)

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

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestSourceTypeString` | `pkg/source/source_test.go` | SourceType.String() |
| `TestSourceString` | `pkg/source/source_test.go` | Source.String() formats |
| `TestRegistryRegisterPeer` | `pkg/source/registry_test.go` | Peer registration |
| `TestRegistryRegisterAPI` | `pkg/source/registry_test.go` | API registration |
| `TestRegistryGet` | `pkg/source/registry_test.go` | Lookup by ID |
| `TestRegistryGetByPeerIP` | `pkg/source/registry_test.go` | Lookup by peer IP |
| `TestRegistryDeactivate` | `pkg/source/registry_test.go` | Deactivation |
| `TestRegistryNeverReuse` | `pkg/source/registry_test.go` | IDs not reused |
| `TestRegistryConcurrent` | `pkg/source/registry_test.go` | Thread safety |
| `TestWireUpdateSourceID` | `pkg/api/wire_update_test.go` | SourceID get/set |
| `TestJSONOutputSource` | `pkg/api/json_test.go` | Source in output |

### Functional Tests

| Test | Location | Scenario |
|------|----------|----------|
| N/A | - | No new functional tests needed (existing tests cover message flow) |

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

1. **Write tests** - Create `pkg/source/source_test.go` and `pkg/source/registry_test.go`
2. **Run tests** - Verify FAIL (paste output)
3. **Implement** - Create `pkg/source/` package with types and registry
4. **Run tests** - Verify PASS (paste output)
5. **Add WireUpdate integration** - Add sourceID field, write test, implement
6. **Register peers** - Peer stores sourceID, reactor sets on WireUpdate
7. **Register API processes** - Process stores sourceID
8. **Update formatters** - Add source to JSON/text output
9. **Remove SourcePeerIP** - Migrate usages to WireUpdate.SourceID()
10. **Verify all** - `make lint && make test && make functional`

## RFC Documentation

N/A - Source registry is internal implementation, not BGP protocol.

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified)
- [x] Implementation complete
- [x] Tests PASS (20 tests)

### Verification
- [x] `make lint` passes (source package clean)
- [x] `make test` passes
- [x] `make functional` passes (18 tests)

### Documentation
- [x] Required docs read
- [x] RFC references added (N/A)

### Completion
- [x] Implementation complete

## Final Implementation

### SourceID Design (uint32, self-describing)
```
0:        config (singleton)
1-99999:  peer
100000:   reserved (gap)
100001+:  api
MaxUint32: invalid
```

### Features Implemented
- `pkg/source/source.go` - SourceID, SourceType, Source types
- `pkg/source/registry.go` - Thread-safe registry with O(1) lookups
- `SourceID.String()` - Returns "type:n" (1-based): "peer:42", "api:1", "config:1"
- `ParseSourceID()` - Parses "type:n" with overflow protection
- Convenience: `IsValid()`, `IsPeer()`, `IsAPI()`, `IsConfig()`
- `Get()` returns Source by value (safe, no data races)
- `WireUpdate.SourceID()` / `SetSourceID()`
- `SplitWireUpdate` preserves sourceID on split chunks
- Peer registration at creation, sourceID set on received UPDATEs

### Deferred (follow-up)
- API process registration
- JSON/text formatter updates
- Remove `ReceivedUpdate.SourcePeerIP`
