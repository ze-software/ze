# Watchdog Pool Architecture

**Status:** ✅ DONE (2025-12-27)

## Overview

Refactor watchdog to use global route pools indexed by name, enabling dynamic route creation via API.

## Completed

- ✅ WatchdogManager with pool-based route control
- ✅ WatchdogPool, PoolRoute types
- ✅ Pool operations (Add, Remove, Announce, Withdraw)
- ✅ Unit tests

## Original Architecture

```
PeerSettings.WatchdogGroups map[string][]WatchdogRoute  // Per-peer, config-only
```

- Routes defined in config per-peer
- `announce watchdog <name>` looks up peer's local group
- No dynamic route creation

## Proposed Architecture

```
Reactor.watchdogPools map[string]*WatchdogPool  // Global, API-created

type WatchdogPool struct {
    Routes map[string]*PoolRoute  // routeKey → route
    mu     sync.RWMutex
}

type PoolRoute struct {
    Route     StaticRoute
    State     map[string]bool  // peerAddr → announced
}
```

- Routes stored in global pools indexed by name
- API creates routes: `announce route X watchdog <name>`
- `announce watchdog <name>` looks up global pool, sends to all peers

## API Commands

| Command | Action |
|---------|--------|
| `announce route 10.0.0.0/24 next-hop 1.2.3.4 watchdog health` | Add to pool, announce to peers |
| `withdraw route 10.0.0.0/24 watchdog health` | Remove from pool, withdraw from peers |
| `announce watchdog health` | Announce all withdrawn routes in pool |
| `withdraw watchdog health` | Withdraw all announced routes in pool |

## Implementation Phases

### Phase 1: Data Structures

**Files:** `pkg/reactor/watchdog.go` (new)

```go
// WatchdogPool holds routes for a named watchdog group.
type WatchdogPool struct {
    name   string
    routes map[string]*PoolRoute  // routeKey → route
    mu     sync.RWMutex
}

// PoolRoute is a route in a watchdog pool with per-peer state.
type PoolRoute struct {
    StaticRoute
    announced map[string]bool  // peerAddr → isAnnounced
}

// WatchdogManager manages global watchdog pools.
type WatchdogManager struct {
    pools map[string]*WatchdogPool
    mu    sync.RWMutex
}
```

**Tasks:**
- [ ] Create `pkg/reactor/watchdog.go`
- [ ] Define WatchdogPool, PoolRoute, WatchdogManager types
- [ ] Add WatchdogManager to Reactor struct
- [ ] Initialize in Reactor.New()

### Phase 2: Pool Operations

**File:** `pkg/reactor/watchdog.go`

```go
func (m *WatchdogManager) AddRoute(poolName string, route StaticRoute) *PoolRoute
func (m *WatchdogManager) RemoveRoute(poolName, routeKey string) bool
func (m *WatchdogManager) GetPool(name string) *WatchdogPool
func (m *WatchdogManager) AnnouncePool(name string) []*PoolRoute  // returns routes to announce
func (m *WatchdogManager) WithdrawPool(name string) []*PoolRoute  // returns routes to withdraw
```

**Tasks:**
- [ ] Implement AddRoute (creates pool if needed)
- [ ] Implement RemoveRoute
- [ ] Implement GetPool
- [ ] Implement AnnouncePool (returns routes where announced=false)
- [ ] Implement WithdrawPool (returns routes where announced=true)
- [ ] Add unit tests

### Phase 3: Reactor Integration

**File:** `pkg/reactor/reactor.go`

```go
func (r *Reactor) AddWatchdogRoute(route StaticRoute, poolName string) error
func (r *Reactor) RemoveWatchdogRoute(routeKey, poolName string) error
func (r *Reactor) AnnounceWatchdogPool(peerSelector, poolName string) error
func (r *Reactor) WithdrawWatchdogPool(peerSelector, poolName string) error
```

**Tasks:**
- [ ] Add WatchdogManager field to Reactor
- [ ] Implement AddWatchdogRoute (add to pool + send to peers)
- [ ] Implement RemoveWatchdogRoute (remove from pool + withdraw from peers)
- [ ] Modify AnnounceWatchdog to check global pools first, then per-peer
- [ ] Modify WithdrawWatchdog to check global pools first, then per-peer
- [ ] Handle peer connect: send announced routes from pools
- [ ] Handle peer disconnect: update pool state

### Phase 4: API Integration

**File:** `pkg/plugin/route.go`

Modify route parsing to detect `watchdog <name>` suffix:

```go
// announce route 10.0.0.0/24 next-hop 1.2.3.4 watchdog health
// → ParseRoute returns route + watchdogName

func handleAnnounceRoute(...) {
    route, watchdogName := parseRouteWithWatchdog(args)
    if watchdogName != "" {
        return ctx.Reactor.AddWatchdogRoute(route, watchdogName)
    }
    // existing flow
}
```

**Tasks:**
- [ ] Modify ParseRoute to extract watchdog name
- [ ] Update handleAnnounceRoute to call AddWatchdogRoute
- [ ] Update handleWithdrawRoute to call RemoveWatchdogRoute
- [ ] Add API tests

### Phase 5: Config Integration (Optional)

Allow config-defined routes to populate global pools at startup:

```
watchdog health {
    route 10.0.0.0/24 next-hop self;
    route 10.0.1.0/24 next-hop self;
}

peer 10.0.0.1 {
    subscribe watchdog health;
}
```

**Tasks:**
- [ ] Add watchdog block to config schema
- [ ] Parse watchdog blocks into WatchdogManager
- [ ] Add peer subscription mechanism
- [ ] Resolve `next-hop self` per-peer

## State Management

### Per-Peer Announced State

Each route tracks which peers have it announced:

```go
type PoolRoute struct {
    StaticRoute
    announced map[string]bool  // "10.0.0.1" → true
}
```

### On Peer Connect

```go
func (r *Reactor) onPeerEstablished(peer *Peer) {
    // Send all announced routes from pools
    for _, pool := range r.watchdog.pools {
        for _, route := range pool.routes {
            if route.announced[peer.Address()] {
                peer.SendUpdate(buildUpdate(route))
            }
        }
    }
}
```

### On Peer Disconnect

State preserved - routes remain "announced" for that peer. On reconnect, they'll be re-sent.

## Next-Hop Handling

| Config Value | Behavior |
|--------------|----------|
| `next-hop 1.2.3.4` | Fixed IP, same for all peers |
| `next-hop self` | Replace with peer's local-address at send time |

```go
func resolveNextHop(route StaticRoute, peer *Peer) netip.Addr {
    if route.NextHopSelf {
        return peer.Settings().LocalAddress
    }
    return route.NextHop
}
```

## Compatibility

| Feature | Config-based (current) | Pool-based (new) |
|---------|----------------------|------------------|
| Per-peer routes | ✅ WatchdogGroups | ❌ Global only |
| Dynamic creation | ❌ | ✅ API |
| State persistence | ✅ Per-peer | ✅ Global + per-peer |
| Mixed use | N/A | ✅ Check pools first, then per-peer |

## Testing

### Unit Tests
- [ ] WatchdogManager.AddRoute creates pool
- [ ] WatchdogManager.RemoveRoute handles missing
- [ ] PoolRoute state tracking per-peer
- [ ] AnnouncePool returns correct routes
- [ ] WithdrawPool returns correct routes

### Integration Tests
- [ ] API: announce route with watchdog
- [ ] API: withdraw watchdog pool
- [ ] Peer reconnect re-sends pool routes
- [ ] Mixed config + API watchdog groups

## Migration

No breaking changes:
1. Existing per-peer WatchdogGroups continue to work
2. New global pools are additive
3. `announce watchdog <name>` checks both locations
