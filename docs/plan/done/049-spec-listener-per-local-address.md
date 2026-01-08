# Spec: Listener Per Local Address

## BREAKING CHANGE WARNING

```
╔═══════════════════════════════════════════════════════════════════════════════╗
║                                                                               ║
║   THIS IS A HARD BREAKING CHANGE                                              ║
║                                                                               ║
║   Existing configurations WILL FAIL if:                                       ║
║   1. They rely on TCP.Bind environment variable (being removed)               ║
║   2. Any peer is missing local-address (now mandatory)                        ║
║                                                                               ║
║   There is NO automatic migration. Users MUST update configs before upgrade.  ║
║                                                                               ║
╚═══════════════════════════════════════════════════════════════════════════════╝
```

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. docs/plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. pkg/reactor/reactor.go - Current listener implementation    │
│  6. pkg/reactor/listener.go - Listener type                     │
│  7. pkg/config/environment.go - TCP.Bind removal target         │
│  8. pkg/config/loader.go - Config parsing for LocalAddress      │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Summary

Replace the single listener + explicit `TCP.Bind` configuration with multiple listeners derived automatically from peer `LocalAddress` fields. This reduces configuration redundancy and improves security by only exposing BGP on interfaces where peers are configured.

## Motivation

### Security
- **Current**: `TCP.Bind` defaults to empty, often resulting in binding to `0.0.0.0:179`
- **Problem**: Exposes BGP on ALL interfaces - unnecessary attack surface
- **Solution**: Only listen on addresses where peers are actually configured

### Configuration Simplicity
- **Current**: Must specify `TCP.Bind` AND each peer's `LocalAddress`
- **Problem**: Redundant - the bind addresses are already implicit in peer configs
- **Solution**: Derive listeners from peer `LocalAddress` fields

### RFC Compliance
- BGP sessions require matching local/remote address pairs
- `LocalAddress` should be mandatory, not optional
- This change enforces that requirement

## Design

### Current Architecture

```
Environment:
  TCP.Bind: ["192.168.1.1", "10.0.0.1"]  ← Explicit list

Reactor:
  listener: *Listener  ← Single listener on config.ListenAddr
```

### New Architecture

```
Peers:
  - Address: 192.168.1.2, LocalAddress: 192.168.1.1  ─┐
  - Address: 192.168.1.3, LocalAddress: 192.168.1.1  ─┴─► Listener on 192.168.1.1:179
  - Address: 10.0.0.2,    LocalAddress: 10.0.0.1     ───► Listener on 10.0.0.1:179

Reactor:
  listeners: map[netip.Addr]*Listener  ← One per unique LocalAddress
```

### Connection Flow

```
1. Incoming TCP connection on 192.168.1.1:179
2. Listener accepts, calls handleConnectionWithContext(conn, listenerAddr=192.168.1.1)
3. Extract REMOTE IP from conn.RemoteAddr() → e.g., 10.0.0.2
4. Lookup peer by REMOTE IP: r.peers["10.0.0.2"] → finds correct peer
5. VALIDATES: peer's LocalAddress == listenerAddr (RFC compliance)
6. Hands connection to peer
```

**Multiple peers sharing LocalAddress:**
```
Peer A: Address=10.0.0.2, LocalAddress=192.168.1.1  ─┐
Peer B: Address=10.0.0.3, LocalAddress=192.168.1.1  ─┴─► ONE listener on 192.168.1.1:179

Connection from 10.0.0.2:
  → listenerAddr=192.168.1.1, remoteIP=10.0.0.2
  → r.peers["10.0.0.2"] → Peer A ✓

Connection from 10.0.0.3:
  → listenerAddr=192.168.1.1, remoteIP=10.0.0.3
  → r.peers["10.0.0.3"] → Peer B ✓
```

The peer lookup is by **remote IP** (peer's Address), not by listener address. The listener address is only used for validation.

## Configuration Constraints

The following configurations are **invalid** and MUST be rejected:

| Constraint | Example | Reason |
|------------|---------|--------|
| Missing LocalAddress | `address: 10.0.0.2` (no local-address) | Cannot determine listener |
| Self-referential | `address: 10.0.0.1, local-address: 10.0.0.1` | Peer pointing at itself |
| Duplicate peer Address | Two peers with same `address` | Map key collision, ambiguous matching |
| Link-local IPv6 | `local-address: fe80::1` | Requires zone ID, not portable |

### Validation Errors

```go
// Missing LocalAddress
"peer 10.0.0.2: local-address is required"

// Self-referential
"peer 10.0.0.1: address cannot equal local-address"

// Duplicate Address (wraps ErrPeerExists for errors.Is compatibility)
"peer 10.0.0.2: peer already exists"

// Link-local IPv6
"peer 10.0.0.2: link-local addresses not supported for local-address"

// Port already in use
"listen on 192.168.1.1: bind: address already in use"

// Port not configured
"config.Port must be set (use 179 for standard BGP)"
```

## ExaBGP Compatibility

> **Intentional Divergence**: This design removes support for ExaBGP's `tcp { bind [...] }`
> configuration. ZeBGP derives listen addresses from peer `local-address` fields for security.
>
> **Migration**: ExaBGP users must ensure all neighbors have `local-address` configured.
> The derived listeners will match the previous `bind` list if configurations were consistent.

## Changes Required

### 1. Remove TCP.Bind from Environment

**File**: `pkg/config/environment.go`

```go
// BEFORE
type TCPEnv struct {
    Attempts int
    Delay    int
    Bind     []string  // REMOVE
    Port     int
    ACL      bool
}

// AFTER
type TCPEnv struct {
    Attempts int
    Delay    int
    Port     int
    ACL      bool
}
```

Remove:
- `TCPEnv.Bind` field
- `getEnvStringList("tcp", "bind", ...)` call in `loadFromEnv()`

### 2. Make LocalAddress Mandatory in PeerSettings

**File**: `pkg/reactor/peersettings.go`

```go
// Document the requirement
type PeerSettings struct {
    // Address is the peer's IP address.
    Address netip.Addr

    // LocalAddress is our local IP for this session.
    // REQUIRED: Used to determine which interface to listen on.
    LocalAddress netip.Addr

    // ...
}
```

**File**: `pkg/config/loader.go` (or wherever peer config is validated)

Add validation:
```go
if !settings.LocalAddress.IsValid() {
    return fmt.Errorf("peer %s: local-address is required", settings.Address)
}
```

### 3. Multi-Listener Support in Reactor

**File**: `pkg/reactor/reactor.go`

```go
// BEFORE
type Reactor struct {
    listener *Listener
    // ...
}

// AFTER
type Reactor struct {
    listeners map[netip.Addr]*Listener  // Keyed by local address
    // ...
}
```

#### Constructor Update

```go
// Initialize map in New() for safety (reading nil map is safe, but be explicit)
func New(config *Config) *Reactor {
    return &Reactor{
        config:    config,
        peers:     make(map[string]*Peer),
        listeners: make(map[netip.Addr]*Listener),  // ADD THIS
        ribIn:     rib.NewIncomingRIB(),
        ribOut:    rib.NewOutgoingRIB(),
        ribStore:  rib.NewRouteStore(100),
        watchdog:  NewWatchdogManager(),
    }
}
```

#### Startup Logic

```go
func (r *Reactor) StartWithContext(ctx context.Context) error {
    // Collect unique local addresses from peers
    localAddrs := make(map[netip.Addr]struct{})
    for _, peer := range r.peers {
        localAddrs[peer.Settings().LocalAddress] = struct{}{}
    }

    // Handle no peers case
    if len(localAddrs) == 0 {
        log.Warn("no peers configured, not listening for BGP connections")
        // Reactor starts successfully, can accept peers via AddPeer later
    }

    // Create listener for each unique address
    r.listeners = make(map[netip.Addr]*Listener)
    for addr := range localAddrs {
        if err := r.startListenerForAddress(ctx, addr); err != nil {
            // Cleanup already-started listeners
            for _, l := range r.listeners {
                l.Stop()
            }
            return err
        }
    }

    // ... rest of startup
}

// startListenerForAddress creates and starts a listener for the given local address.
func (r *Reactor) startListenerForAddress(ctx context.Context, addr netip.Addr) error {
    if r.config.Port == 0 {
        return errors.New("config.Port must be set (use 179 for standard BGP)")
    }
    listenAddr := net.JoinHostPort(addr.String(), strconv.Itoa(r.config.Port))
    listener := NewListener(listenAddr)

    // Capture addr in closure so handleConnection knows which listener accepted
    localAddr := addr
    listener.SetHandler(func(conn net.Conn) {
        r.handleConnectionWithContext(conn, localAddr)
    })

    if err := listener.StartWithContext(ctx); err != nil {
        return fmt.Errorf("listen on %s: %w", addr, err)
    }

    log.Info("started BGP listener on %s:%d", addr, r.config.Port)
    r.listeners[addr] = listener
    return nil
}
```

#### Dynamic Peer Addition

```go
func (r *Reactor) AddPeer(settings *PeerSettings) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Validate LocalAddress is set
    if !settings.LocalAddress.IsValid() {
        return fmt.Errorf("peer %s: local-address is required", settings.Address)
    }

    // Validate not self-referential
    if settings.Address == settings.LocalAddress {
        return fmt.Errorf("peer %s: address cannot equal local-address", settings.Address)
    }

    // Validate not link-local IPv6
    if settings.LocalAddress.Is6() && settings.LocalAddress.IsLinkLocalUnicast() {
        return fmt.Errorf("peer %s: link-local addresses not supported for local-address", settings.Address)
    }

    // Check for duplicate peer
    key := settings.Address.String()
    if _, exists := r.peers[key]; exists {
        return fmt.Errorf("peer %s: %w", settings.Address, ErrPeerExists)
    }

    // Check if we need a new listener
    if _, hasListener := r.listeners[settings.LocalAddress]; !hasListener {
        if r.running {
            if err := r.startListenerForAddress(r.ctx, settings.LocalAddress); err != nil {
                return err
            }
        }
    }

    // Add peer (existing logic)
    peer := NewPeer(settings)
    // ...
}
```

#### Connection Handler with Context

```go
// handleConnectionWithContext handles an incoming TCP connection with listener context.
// listenerAddr is the local address the listener is bound to.
func (r *Reactor) handleConnectionWithContext(conn net.Conn, listenerAddr netip.Addr) {
    remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
    if !ok {
        _ = conn.Close()
        return
    }
    peerIP, _ := netip.AddrFromSlice(remoteAddr.IP)
    peerIP = peerIP.Unmap()

    r.mu.RLock()
    peer, exists := r.peers[peerIP.String()]
    cb := r.connCallback
    r.mu.RUnlock()

    if !exists {
        // Unknown peer
        _ = conn.Close()
        return
    }

    // RFC compliance: verify connection arrived on expected listener
    if peer.Settings().LocalAddress != listenerAddr {
        // Connection to wrong local address - this shouldn't happen normally
        // but could indicate misconfiguration or attack
        log.Warn("peer %s connected to %s but configured for %s",
            peerIP, listenerAddr, peer.Settings().LocalAddress)
        _ = conn.Close()
        return
    }

    // ... rest of connection handling (collision detection, callback, accept)
}
```

#### Cleanup

```go
func (r *Reactor) cleanup() {
    // Stop all listeners
    for addr, listener := range r.listeners {
        log.Info("stopping BGP listener on %s", addr)
        listener.Stop()
        waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        _ = listener.Wait(waitCtx)
        cancel()
        delete(r.listeners, addr)
    }

    // ... rest of cleanup
}
```

#### Dynamic Peer Removal

```go
func (r *Reactor) RemovePeer(addr netip.Addr) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    peer, exists := r.peers[addr.String()]
    if !exists {
        return ErrPeerNotFound
    }

    localAddr := peer.Settings().LocalAddress
    peer.Stop()
    delete(r.peers, addr.String())

    // Check if any other peer uses this LocalAddress
    stillUsed := false
    for _, p := range r.peers {
        if p.Settings().LocalAddress == localAddr {
            stillUsed = true
            break
        }
    }

    // Stop listener if no longer needed
    if !stillUsed {
        if listener, ok := r.listeners[localAddr]; ok {
            log.Info("stopping BGP listener on %s (no more peers)", localAddr)
            listener.Stop()
            delete(r.listeners, localAddr)
        }
    }

    return nil
}
```

### 4. Update Reactor Config

**File**: `pkg/reactor/reactor.go`

```go
// BEFORE
type Config struct {
    ListenAddr    string  // REMOVE - no longer single address
    RouterID      uint32
    LocalAS       uint32
    APISocketPath string
    // ...
}

// AFTER
type Config struct {
    Port          int     // BGP port (default 179)
    RouterID      uint32
    LocalAS       uint32
    APISocketPath string
    // ...
}
```

### 5. Update ListenAddr Helper

```go
// ListenAddrs returns all addresses the reactor is listening on.
func (r *Reactor) ListenAddrs() []net.Addr {
    r.mu.RLock()
    defer r.mu.RUnlock()

    addrs := make([]net.Addr, 0, len(r.listeners))
    for _, l := range r.listeners {
        if addr := l.Addr(); addr != nil {
            addrs = append(addrs, addr)
        }
    }
    return addrs
}
```

## Edge Cases

### 1. No Peers Configured
- No listeners started
- Reactor runs but accepts no connections
- Log warning: "no peers configured, not listening for BGP connections"

### 2. IPv4 and IPv6 Peers
- Separate listeners for IPv4 and IPv6 addresses
- Each listener only accepts connections on its address family

### 3. Port Conflicts
- If another process holds a local address's port, startup fails with clear error
- Partial startup: cleanup already-started listeners before returning error

### 4. Peer Address Collision with Local Address
- Validation: peer's `Address` must not equal its own `LocalAddress`
- Prevents self-referential peer config

### 5. Connection to Wrong Listener
Scenario where this check triggers:
```
Peer A: Address=10.0.0.2, LocalAddress=192.168.1.1 → listener on 192.168.1.1
Peer B: Address=10.0.0.3, LocalAddress=10.0.0.1   → listener on 10.0.0.1

If host 10.0.0.2 mistakenly connects to 10.0.0.1:179:
  - listenerAddr = 10.0.0.1
  - Peer lookup finds Peer A (by remote IP 10.0.0.2)
  - Peer A's LocalAddress = 192.168.1.1 ≠ 10.0.0.1
  - Connection REJECTED with warning log
```
This catches remote misconfiguration or routing anomalies.

## Implementation Notes

### Logging
The spec uses `log.Info()` and `log.Warn()`. Verify ZeBGP's actual logging package before implementing:
- Check `pkg/trace/` or similar for logging API
- Adjust format strings if needed (e.g., structured logging)

### Config File
The `tcp { bind [...] }` syntax exists ONLY in environment variables (`TCP.Bind`), not in config files. No config file migration needed.

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `docs/plan/CLAUDE_CONTINUATION.md` for current state
- **FIRST:** Read `.claude/ESSENTIAL_PROTOCOLS.md` for session rules
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof

### From ESSENTIAL_PROTOCOLS.md
- TDD is BLOCKING: Tests must exist and fail before implementation
- Check ExaBGP for reference implementation patterns
- RFC compliance is NON-NEGOTIABLE

### RFCs to Reference
- RFC 4271 Section 8: BGP FSM, connection handling
- RFC 5765: Security considerations for BGP

## Implementation Steps

### Phase 1: Add Multi-Listener Support (Non-Breaking)
1. Add `listeners map[netip.Addr]*Listener` to Reactor
2. Write tests for multi-listener startup - MUST FAIL
3. Implement listener-per-address logic in `StartWithContext`
4. Keep existing `ListenAddr` config working (backward compat)
5. Run tests - MUST PASS
6. Run `make test && make lint`

### Phase 2: LocalAddress Validation
1. Write tests for missing LocalAddress rejection - MUST FAIL
2. Add validation in `AddPeer` and config loading
3. Run tests - MUST PASS
4. Run `make test && make lint`

### Phase 3: Dynamic Listener Management
1. Write tests for dynamic peer add/remove with listener lifecycle - MUST FAIL
2. Implement listener start on new LocalAddress
3. Implement listener stop when last peer removed
4. Run tests - MUST PASS
5. Run `make test && make lint`

### Phase 4: Remove TCP.Bind
1. Remove `TCPEnv.Bind` field
2. Remove `ListenAddr` from `reactor.Config`
3. Update all callers
4. Run `make test && make lint`

### Phase 5: Documentation
1. Update any docs referencing `TCP.Bind`
2. Document `LocalAddress` requirement
3. Update example configs

## Test Matrix

| Test Case | Setup | Expected Result |
|-----------|-------|-----------------|
| Two peers, same LocalAddress | A: 10.0.0.2/192.168.1.1, B: 10.0.0.3/192.168.1.1 | One listener on 192.168.1.1 |
| Two peers, different LocalAddresses | A: 10.0.0.2/192.168.1.1, B: 10.0.0.3/10.0.0.1 | Two listeners |
| Add peer, new LocalAddress, running | Reactor running, add peer with new LocalAddress | New listener created |
| Add peer, existing LocalAddress | Reactor running, add peer sharing LocalAddress | No new listener |
| Remove last peer for LocalAddress | Remove only peer using LocalAddress | Listener stopped |
| Remove peer, others share LocalAddress | Remove one of multiple peers on same LocalAddress | Listener stays |
| Add peer, missing LocalAddress | `LocalAddress: netip.Addr{}` | Error: "local-address is required" |
| Add peer, self-referential | `Address == LocalAddress` | Error: "cannot equal local-address" |
| Add peer, duplicate Address | Same Address as existing peer | ErrPeerExists |
| Add peer, link-local IPv6 | `LocalAddress: fe80::1` | Error: "link-local not supported" |
| Connection to correct listener | Peer connects to its LocalAddress | Connection accepted |
| Connection to wrong listener | Peer connects to different LocalAddress (see Edge Case 5) | Connection rejected, logged |
| Port already in use | Another process on LocalAddress:179 | Clear error, no partial state |
| No peers at startup | Empty peer list | Warning logged, no listeners, reactor runs |
| IPv4 peer | LocalAddress is IPv4 | IPv4 listener only |
| IPv6 peer | LocalAddress is IPv6 (non-link-local) | IPv6 listener only |
| Mixed IPv4/IPv6 | One IPv4 peer, one IPv6 peer | Two listeners, separate families |
| Port zero rejected | `Config.Port = 0` | Error: "config.Port must be set" |
| Custom port | `Config.Port = 1179` | Listeners on :1179 |

### Test File Locations

| Component | Test File |
|-----------|-----------|
| Multi-listener startup | `pkg/reactor/reactor_test.go` |
| LocalAddress validation | `pkg/reactor/peer_test.go` |
| Dynamic listener lifecycle | `pkg/reactor/reactor_test.go` |
| Connection validation | `pkg/reactor/reactor_test.go` |
| Config validation | `pkg/config/loader_test.go` |

## Verification Checklist

- [ ] Tests written FIRST, shown to FAIL
- [ ] Multi-listener startup works (one per unique LocalAddress)
- [ ] LocalAddress validation rejects empty/invalid
- [ ] LocalAddress validation rejects self-referential (Address == LocalAddress)
- [ ] LocalAddress validation rejects link-local IPv6
- [ ] Dynamic peer add creates listener if needed
- [ ] Dynamic peer add reuses existing listener if LocalAddress shared
- [ ] Dynamic peer remove stops unused listener
- [ ] Dynamic peer remove keeps listener if other peers share it
- [ ] Connection validation checks listener matches peer's LocalAddress
- [ ] Connection to wrong listener rejected with warning log
- [ ] IPv4 and IPv6 handled correctly (separate listeners)
- [ ] Port=0 rejected with clear error
- [ ] Custom port works correctly
- [ ] Port conflicts produce clear errors, no partial state
- [ ] No peers at startup: warning logged, reactor runs, accepts AddPeer
- [ ] `TCP.Bind` removed from environment
- [ ] `reactor.Config.ListenAddr` removed
- [ ] `listeners` map initialized in New()
- [ ] `handleConnectionWithContext` replaces `handleConnection`
- [ ] Error messages include peer address (wrapped errors)
- [ ] Logging API verified against ZeBGP's actual logging package
- [ ] All callers updated
- [ ] `make test` passes
- [ ] `make lint` passes

## Migration

### Breaking Changes
1. `TCP.Bind` environment variable no longer recognized
2. `LocalAddress` now required for all peers (was optional)
3. `reactor.Config.ListenAddr` removed

### Migration Path
1. Users with `TCP.Bind` set: Remove it, ensure all peers have `LocalAddress`
2. Users without `LocalAddress` on peers: Add it to each peer config
3. The derived listeners will match previous `TCP.Bind` if configs were consistent

## Priority

High - Security improvement, reduces attack surface, enforces RFC compliance.
