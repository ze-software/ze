# Spec: Peer Lifecycle Callbacks

**Status:** Implemented
**Date:** 2026-01-03
**Depends on:** None
**Enables:** Phase 0.5 (Message Callbacks), Plugin System

**See:** `docs/architecture/rib-transition.md` for overall architecture direction.

---

## Required Reading (MUST complete before implementation)

The following docs MUST be read before starting implementation:

- [x] `docs/architecture/behavior/FSM.md` - FSM states and transitions, callback patterns
- [x] `docs/architecture/api/ARCHITECTURE.md` - API server, message formats, process binding

**Key insights from docs:**

- FSM already has callback mechanism: `session.fsm.SetCallback()` (peer.go:681)
- API output format: `{"type":"state","neighbor":{"address":{"peer":"..."},"state":"up/down"}}`
- ExaBGP uses `fsm` API type with `api['fsm']` config flag
- No existing `EmitPeerState()` or `broadcastJSON()` in api package

---

## Design Transition Alignment

This spec is **independent** of the Pool + Wire design:

| Aspect | Impact |
|--------|--------|
| Observer pattern | No conflict - operates at peer level, not route storage |
| `OnPeerClosed` | Future: trigger `pool.Release()` for peer's routes |
| `RawMessage` in Phase 0.5 | Compatible - uses wire bytes (pool-friendly) |

**No changes needed** - implement as specified.

---

## Goal

Add peer lifecycle callbacks to Reactor that:
1. Notify when peer reaches Established state
2. Notify when peer leaves Established state
3. Emit ExaBGP-compatible API state messages

---

## Current State

### FSM Callback in Peer

```go
// peer.go:681
session.fsm.SetCallback(func(from, to fsm.State) {
    addr := p.settings.Address.String()
    trace.FSMTransition(addr, from.String(), to.String())

    if to == fsm.StateEstablished {
        neg := session.Negotiated()
        p.negotiated.Store(NewNegotiatedCapabilities(neg))
        p.setEncodingContexts(neg)
        p.setState(PeerStateEstablished)
        trace.SessionEstablished(addr, p.settings.LocalAS, p.settings.PeerAS)
        go p.sendInitialRoutes()
    } else if from == fsm.StateEstablished {
        p.negotiated.Store(nil)
        p.clearEncodingContexts()
        p.setState(PeerStateConnecting)
        trace.SessionClosed(addr, "FSM left Established state")
    }
})
```

### Missing

- No callback to Reactor when peer state changes
- API state messages not emitted on these transitions
- No hook point for future plugins

---

## Proposed Changes

### 1. Add PeerLifecycleObserver to Reactor

```go
// reactor.go

// PeerLifecycleObserver receives peer state change notifications.
type PeerLifecycleObserver interface {
    OnPeerEstablished(peer *Peer)
    OnPeerClosed(peer *Peer, reason string)
}

type Reactor struct {
    // ... existing fields ...

    // Peer lifecycle observers (called on state transitions)
    peerObservers []PeerLifecycleObserver
    observersMu   sync.RWMutex
}

// AddPeerObserver registers an observer for peer lifecycle events.
// Observers are called synchronously in registration order.
// MUST NOT block; use goroutine for slow processing.
func (r *Reactor) AddPeerObserver(obs PeerLifecycleObserver) {
    r.observersMu.Lock()
    defer r.observersMu.Unlock()
    r.peerObservers = append(r.peerObservers, obs)
}

// notifyPeerEstablished calls all observers when peer reaches Established.
func (r *Reactor) notifyPeerEstablished(peer *Peer) {
    r.observersMu.RLock()
    observers := r.peerObservers
    r.observersMu.RUnlock()

    for _, obs := range observers {
        obs.OnPeerEstablished(peer)
    }
}

// notifyPeerClosed calls all observers when peer leaves Established.
func (r *Reactor) notifyPeerClosed(peer *Peer, reason string) {
    r.observersMu.RLock()
    observers := r.peerObservers
    r.observersMu.RUnlock()

    for _, obs := range observers {
        obs.OnPeerClosed(peer, reason)
    }
}
```

### 2. Add Reactor Reference to Peer

```go
// peer.go

type Peer struct {
    // ... existing fields ...

    // reactor is set when peer is added to reactor.
    // Used to notify reactor of state changes.
    reactor *Reactor
}

// SetReactor sets the reactor reference.
// Called by Reactor.AddPeer().
func (p *Peer) SetReactor(r *Reactor) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.reactor = r
}
```

### 3. Modify FSM Callback to Notify Reactor

```go
// peer.go:681 - modified
session.fsm.SetCallback(func(from, to fsm.State) {
    addr := p.settings.Address.String()
    trace.FSMTransition(addr, from.String(), to.String())

    if to == fsm.StateEstablished {
        neg := session.Negotiated()
        p.negotiated.Store(NewNegotiatedCapabilities(neg))
        p.setEncodingContexts(neg)
        p.setState(PeerStateEstablished)
        trace.SessionEstablished(addr, p.settings.LocalAS, p.settings.PeerAS)

        // NEW: Notify reactor
        p.mu.RLock()
        reactor := p.reactor
        p.mu.RUnlock()
        if reactor != nil {
            reactor.notifyPeerEstablished(p)
        }

        go p.sendInitialRoutes()

    } else if from == fsm.StateEstablished {
        p.negotiated.Store(nil)
        p.clearEncodingContexts()
        p.setState(PeerStateConnecting)

        // Determine reason
        reason := "session closed"
        if to == fsm.StateIdle {
            reason = "connection lost"
        }

        // NEW: Notify reactor
        p.mu.RLock()
        reactor := p.reactor
        p.mu.RUnlock()
        if reactor != nil {
            reactor.notifyPeerClosed(p, reason)
        }

        trace.SessionClosed(addr, reason)
    }
})
```

### 4. Set Reactor in AddPeer

```go
// reactor.go - AddPeer modified

func (r *Reactor) AddPeer(settings *PeerSettings) (*Peer, error) {
    // ... existing validation ...

    peer := NewPeer(settings)
    peer.SetReactor(r)  // NEW
    peer.SetGlobalWatchdog(r.watchdog)

    // ... rest of existing code ...
}
```

### 5. API State Observer

```go
// reactor.go

// apiStateObserver emits ExaBGP-compatible state messages.
type apiStateObserver struct {
    server *api.Server
}

func (o *apiStateObserver) OnPeerEstablished(peer *Peer) {
    if o.server == nil {
        return
    }
    o.server.EmitPeerState(peer.Settings().Address, "up")
}

func (o *apiStateObserver) OnPeerClosed(peer *Peer, reason string) {
    if o.server == nil {
        return
    }
    o.server.EmitPeerState(peer.Settings().Address, "down")
}
```

### 6. Add EmitPeerState to API Server

```go
// api/server.go

// EmitPeerState sends a peer state change to all subscribed processes.
// State is "up" or "down".
func (s *Server) EmitPeerState(peerAddr netip.Addr, state string) {
    // Build ExaBGP-compatible JSON message
    msg := map[string]any{
        "exabgp": "4.0.1",
        "time":   time.Now().Unix(),
        "host":   hostname(),
        "pid":    os.Getpid(),
        "ppid":   os.Getppid(),
        "type":   "state",
        "neighbor": map[string]any{
            "address": map[string]string{
                "peer": peerAddr.String(),
            },
            "state": state,
        },
    }

    s.broadcastJSON(msg)
}
```

### 7. Register Observer on Reactor Init

```go
// reactor.go - in NewReactor or Run

func (r *Reactor) Run(ctx context.Context) error {
    // ... existing setup ...

    // Register API state observer
    if r.api != nil {
        r.AddPeerObserver(&apiStateObserver{server: r.api})
    }

    // ... rest of run logic ...
}
```

---

## API Message Format

### State Up (Established)

```json
{
  "exabgp": "4.0.1",
  "time": 1704307200,
  "host": "router1",
  "pid": 12345,
  "ppid": 1,
  "type": "state",
  "neighbor": {
    "address": {
      "peer": "192.0.2.1"
    },
    "state": "up"
  }
}
```

### State Down (Closed)

```json
{
  "exabgp": "4.0.1",
  "time": 1704307260,
  "host": "router1",
  "pid": 12345,
  "ppid": 1,
  "type": "state",
  "neighbor": {
    "address": {
      "peer": "192.0.2.1"
    },
    "state": "down"
  }
}
```

---

## Thread Safety

1. **Observer registration**: Protected by `observersMu`
2. **Observer iteration**: Copy slice under read lock, iterate unlocked
3. **Peer.reactor access**: Protected by `peer.mu`
4. **Callback execution**: Synchronous, must not block

---

## Testing

### Test Cases

```go
// reactor_test.go

func TestPeerLifecycleCallbacks(t *testing.T) {
    // VALIDATES: OnPeerEstablished called when peer reaches Established
    // PREVENTS: Missing state notifications to plugins/API
}

func TestPeerLifecycleCallbackOrder(t *testing.T) {
    // VALIDATES: Observers called in registration order
    // PREVENTS: Non-deterministic callback ordering
}

func TestPeerClosedReason(t *testing.T) {
    // VALIDATES: Correct reason passed to OnPeerClosed
    // PREVENTS: Misleading close reasons in logs/API
}

func TestAPIStateEmission(t *testing.T) {
    // VALIDATES: JSON state messages emitted on transitions
    // PREVENTS: Missing ExaBGP-compatible state output
}
```

### Manual Test

```bash
# Start ze bgp with API process
zebgp -c test.toml

# In another terminal, watch API output
cat /var/run/ze-bgp.sock

# Should see state messages when peer connects/disconnects:
# {"exabgp":"4.0.1","type":"state","neighbor":{"address":{"peer":"192.0.2.1"},"state":"up"}}
```

---

## Implementation Checklist

- [ ] Add `PeerLifecycleObserver` interface to reactor.go
- [ ] Add `peerObservers` slice and mutex to Reactor
- [ ] Add `AddPeerObserver()` method
- [ ] Add `notifyPeerEstablished()` and `notifyPeerClosed()` methods
- [ ] Add `reactor` field to Peer struct
- [ ] Add `SetReactor()` method to Peer
- [ ] Modify FSM callback to call reactor notify methods
- [ ] Modify `AddPeer()` to call `SetReactor()`
- [ ] Add `apiStateObserver` implementation
- [ ] Add `EmitPeerState()` to api.Server
- [ ] Register apiStateObserver in Reactor.Run()
- [ ] Write tests for callback ordering
- [ ] Write tests for API state emission
- [ ] Update functional tests to verify state messages

---

## Files Modified

| File | Changes |
|------|---------|
| `internal/reactor/reactor.go` | Add observer interface, methods, apiStateObserver |
| `internal/reactor/peer.go` | Add reactor field, SetReactor, modify FSM callback |
| `internal/plugin/server.go` | Add EmitPeerState method |
| `internal/reactor/reactor_test.go` | Add callback tests |

---

## Risks

1. **Performance**: Synchronous callbacks add latency to state transitions
   - Mitigation: Document "must not block" requirement
   - Future: Consider async dispatch if needed

2. **Ordering**: Callbacks before sendInitialRoutes()
   - Intentional: Plugins should see Established before routes sent
   - Document this ordering guarantee

3. **Deadlock**: Callback holding reactor lock while accessing peer
   - Mitigation: Copy observer slice under lock, iterate unlocked
   - Peer accesses reactor via copied reference

---

## Future Extensions

Phase 0.5 will add:
- `OnMessage(peer *Peer, msg RawMessage)` callback
- Reuse same observer pattern

Phase 1 will:
- Wrap observers in Plugin interface
- Add dependency ordering
- Add Registry for plugin management

Pool+Wire integration (after `spec-pool-handle-migration.md`):
- `OnPeerClosed` will trigger `pool.Release()` for all routes from that peer
- RIB cleanup uses observer pattern established here
