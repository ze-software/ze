# Spec: Route ID Forwarding

## Status: Ready for Implementation

## Prerequisites

- `spec-attributes-wire.md` - AttributesWire type

## Problem

Current API flow for route reflection:
```
Receive → Parse → Store parsed → API output → Decision → Rebuild UPDATE
```

No way to reference received routes by ID for efficient forwarding.

## Solution

1. **Route ID** - unique identifier assigned on receive
2. **Forward command** - `peer <selector> forward route-id <id>`
3. **Negated selector** - `!<ip>` for "all except source"

## Route Changes

```go
type Route struct {
    nlri          nlri.NLRI
    attrs         *AttributesWire    // From spec-attributes-wire.md
    nlriWireBytes []byte
    sourceCtxID   bgpctx.ContextID
    routeID       uint64             // NEW: unique identifier
    sourcePeerIP  netip.Addr         // NEW: for !<ip> selector
}
```

### Route ID Generation

```go
var routeIDCounter atomic.Uint64

func (r *Reactor) assignRouteID(route *Route) {
    route.routeID = routeIDCounter.Add(1)
}
```

### Route Lookup

```go
// RIB maintains route-id index
type RIB struct {
    byID map[uint64]*Route
    // ...
}

func (r *RIB) GetByID(id uint64) (*Route, bool)
```

## API Output

```json
{
    "type": "update",
    "route-id": 12345,
    "peer": { "address": "10.0.0.1" },
    "announce": {
        "nlri": { "ipv4 unicast": ["192.168.1.0/24"] }
    }
}
```

## Forward Command

### Syntax

```
peer <selector> forward route-id <id>
```

### Examples

```
# Forward to specific peer
peer 10.0.0.2 forward route-id 12345

# Forward to all peers except source
peer !10.0.0.1 forward route-id 12345

# Forward to all peers (unusual but valid)
peer * forward route-id 12345
```

### Implementation

```go
func (d *Dispatcher) handleForward(ctx *Context, selector string, args string) error {
    // Parse route-id from args
    routeID, err := parseRouteID(args)
    if err != nil {
        return err
    }

    // Lookup route
    route, ok := d.reactor.RIB().GetByID(routeID)
    if !ok {
        return fmt.Errorf("route-id %d not found", routeID)
    }

    // Get matching peers
    peers := d.reactor.GetMatchingPeers(selector)

    // Forward to each peer
    for _, peer := range peers {
        // Use AttributesWire.PackFor() for zero-copy when possible
        attrBytes := route.attrs.PackFor(peer.sendCtxID)
        nlriBytes := route.PackNLRIFor(peer.sendCtxID)
        // Build and send UPDATE
    }
    return nil
}
```

## Negated Peer Selector

### Syntax

```
peer !<ip>     # All peers EXCEPT this IP
```

### Implementation

```go
func ParseSelector(s string) (*Selector, error) {
    if strings.HasPrefix(s, "!") {
        ip, err := netip.ParseAddr(s[1:])
        if err != nil {
            return nil, err
        }
        return &Selector{Exclude: ip}, nil
    }
    // ... existing selector parsing
}

type Selector struct {
    All     bool
    IP      netip.Addr
    Exclude netip.Addr    // NEW: for !<ip>
    Filters map[string]string
}

func (r *Reactor) GetMatchingPeers(selector *Selector) []*Peer {
    if selector.Exclude.IsValid() {
        // Return all peers except the excluded one
        var result []*Peer
        for _, p := range r.peers {
            if p.addr != selector.Exclude {
                result = append(result, p)
            }
        }
        return result
    }
    // ... existing matching logic
}
```

## Flow

```
Peer A → Receive UPDATE → Assign route-id 12345 → Store in RIB
                                    ↓
                        API output: { route-id: 12345, ... }
                                    ↓
                        External process: forward to B, C
                                    ↓
                        API command: peer !10.0.0.1 forward route-id 12345
                                    ↓
                        Lookup route by ID → PackFor(peer.sendCtxID) → Send
```

## Test Plan

```go
// TestRouteIDAssignment verifies unique ID generation.
// VALIDATES: Each route gets unique ID.
// PREVENTS: ID collisions causing wrong route forwarding.
func TestRouteIDAssignment(t *testing.T)

// TestRIBGetByID verifies route lookup by ID.
// VALIDATES: Route can be retrieved by ID.
// PREVENTS: Lost routes, broken forwarding.
func TestRIBGetByID(t *testing.T)

// TestForwardCommand verifies forward parsing and execution.
// VALIDATES: Forward command works end-to-end.
// PREVENTS: Command parsing errors, forwarding failures.
func TestForwardCommand(t *testing.T)

// TestNegatedSelector verifies !<ip> parsing.
// VALIDATES: !<ip> matches all except specified.
// PREVENTS: Wrong peer selection, route loops.
func TestNegatedSelector(t *testing.T)

// TestForwardZeroCopy verifies zero-copy when contexts match.
// VALIDATES: Same context uses Packed() directly.
// PREVENTS: Unnecessary re-encoding.
func TestForwardZeroCopy(t *testing.T)
```

## Checklist

- [ ] Route ID field added to Route
- [ ] Route ID generation (atomic counter)
- [ ] RIB byID index
- [ ] GetByID() method
- [ ] API output includes route-id
- [ ] `forward route-id` command parsing
- [ ] Forward command execution
- [ ] `!<ip>` selector parsing
- [ ] GetMatchingPeers with exclude
- [ ] Tests pass
- [ ] `make test && make lint` pass

## Dependencies

- `spec-attributes-wire.md` - AttributesWire.PackFor()

## Dependents

- `spec-rfc9234-role.md` - Uses RouteTag for policy (separate from route-id)

---

**Created:** 2026-01-01
