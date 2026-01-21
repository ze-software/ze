# Spec: Update ID Forwarding

## Status: Complete

## Prerequisites

- `spec-attributes-wire.md` - AttributesWire type

## Problem

Current API flow for route reflection:
```
Receive → Parse → Store parsed → API output → Decision → Rebuild UPDATE
```

No way to reference received UPDATEs by ID for efficient forwarding.

## Solution

1. **Update ID** - unique identifier assigned per received UPDATE (immutable snapshot)
2. **One-shot cache** - UPDATEs cached until forwarded or explicitly deleted
3. **Forward command** - `peer <selector> forward update-id <id>` (deletes from cache after send)
4. **Delete command** - `delete update-id <id>` (ack without forwarding)
5. **Negated selector** - `!<ip>` for "all except source"
6. **TTL fallback** - safety net for orphaned entries (60s default)

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| ID scope | Per-UPDATE | One UPDATE with 100 NLRIs gets one ID |
| ID lifetime | Immutable snapshot | Same NLRI with new attrs → new ID |
| Cache strategy | Delete on use | Forward/delete removes entry; TTL is fallback for orphans |
| Expired ID | Fail with error | No fallback scan; controller must use fresh IDs |

## Architecture: ReceivedUpdate vs Route

```
┌─────────────────────────────────────────────────────────────┐
│                     Receive UPDATE                          │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────────┐
│   RecentUpdateCache     │     │           RIB               │
│   (for forwarding)      │     │   (for routing decisions)   │
├─────────────────────────┤     ├─────────────────────────────┤
│ ReceivedUpdate {        │     │ Route per NLRI {            │
│   updateID              │     │   nlri                      │
│   attrs (shared ptr)    │     │   attrs (shared ptr)        │
│   announces []          │     │ }                           │
│   withdraws []          │     │                             │
│ }                       │     │                             │
└─────────────────────────┘     └─────────────────────────────┘
         │                                    │
         │ Deleted on forward/delete          │ Stays until withdrawn
         │ (TTL fallback for orphans)         │
```

**Key:** `attrs` pointer is shared — no duplication of attribute data.

## ReceivedUpdate Type

```go
// ReceivedUpdate represents an immutable snapshot of a received UPDATE.
// Each UPDATE gets a unique ID; updates to same NLRI create new IDs.
type ReceivedUpdate struct {
    updateID      uint64
    attrs         *AttributesWire       // shared with RIB entries
    announces     []nlri.NLRI
    withdraws     []nlri.NLRI           // withdrawals in this UPDATE
    announceWire  [][]byte              // wire bytes per announced NLRI
    withdrawWire  [][]byte              // wire bytes per withdrawn NLRI
    sourcePeerIP  netip.Addr
    sourceCtxID   bgpctx.ContextID
    receivedAt    time.Time
}
```

### Update ID Generation

```go
var updateIDCounter atomic.Uint64

func (r *Reactor) assignUpdateID() uint64 {
    return updateIDCounter.Add(1)
}
```

## One-Shot Cache (with TTL fallback)

```go
type RecentUpdateCache struct {
    mu         sync.RWMutex
    entries    map[uint64]*cacheEntry
    ttl        time.Duration  // default 60s
    maxEntries int            // fixed size, no growth
}

type cacheEntry struct {
    update    *ReceivedUpdate
    expiresAt time.Time
}

func NewRecentUpdateCache(ttl time.Duration, maxEntries int) *RecentUpdateCache {
    return &RecentUpdateCache{
        entries:    make(map[uint64]*cacheEntry, maxEntries),  // pre-allocate
        ttl:        ttl,
        maxEntries: maxEntries,
    }
}

func (c *RecentUpdateCache) Add(update *ReceivedUpdate) {
    c.mu.Lock()
    defer c.mu.Unlock()

    now := time.Now()

    // Lazy cleanup: evict expired entries on each Add
    // BGP keepalives ensure regular Add activity
    for id, e := range c.entries {
        if now.After(e.expiresAt) {
            delete(c.entries, id)
        }
    }

    // Fixed size: drop new entry if still at capacity after eviction
    if len(c.entries) >= c.maxEntries {
        return
    }

    c.entries[update.updateID] = &cacheEntry{
        update:    update,
        expiresAt: now.Add(c.ttl),
    }
}

func (c *RecentUpdateCache) Get(id uint64) (*ReceivedUpdate, bool) {
    c.mu.RLock()
    entry, ok := c.entries[id]
    c.mu.RUnlock()

    if !ok || time.Now().After(entry.expiresAt) {
        return nil, false
    }
    return entry.update, true
}
```

**Design notes:**
- **One-shot:** entries deleted immediately after `forward` or `delete` command
- **TTL fallback:** orphaned entries (controller never responded) cleaned up lazily on Add()
- No background goroutine — TTL cleanup triggered lazily on Add()
- Fixed size, hint capacity — no growth beyond maxEntries
- If full after eviction, new entry dropped (indicates maxEntries too small)
- **Performance:** O(n) scan on each Add(). Keep maxEntries reasonable (1000-10000) for acceptable latency

### Lookup Flow

```go
func (r *Reactor) GetUpdateByID(id uint64) (*ReceivedUpdate, error) {
    if update, ok := r.recentUpdates.Get(id); ok {
        return update, nil
    }
    return nil, ErrUpdateExpired
}

var ErrUpdateExpired = errors.New("update-id expired or not found")
```

## Configuration

```yaml
rib:
  recent-update-ttl: 60s       # default, how long update-ids remain valid
  recent-update-max: 100000    # optional safety cap, 0 = unlimited
```

## API Output

```json
{
    "type": "update",
    "update-id": 12345,
    "peer": { "address": "10.0.0.1" },
    "announce": {
        "nlri": { "ipv4/unicast": ["192.168.1.0/24", "10.0.0.0/8"] }
    },
    "withdraw": {
        "nlri": { "ipv4/unicast": ["172.16.0.0/16"] }
    }
}
```

**Note:** All NLRIs in one UPDATE share the same `update-id`.

## Forward Command

### Syntax

```
peer <selector> forward update-id <id>
```

### Examples

```
# Forward to specific peer
peer 10.0.0.2 forward update-id 12345

# Forward to all peers except source
peer !10.0.0.1 forward update-id 12345

# Forward to all peers (unusual but valid)
peer * forward update-id 12345
```

### Implementation

```go
func (d *Dispatcher) handleForward(ctx *Context, selector string, args string) error {
    updateID, err := parseUpdateID(args)
    if err != nil {
        return err
    }

    update, err := d.reactor.GetUpdateByID(updateID)
    if err != nil {
        return fmt.Errorf("update-id %d: %w", updateID, err)
    }

    peers := d.reactor.GetMatchingPeers(selector)
    if len(peers) == 0 {
        return fmt.Errorf("no peers match selector %q", selector)
    }

    // Forward to all matching peers, collect errors
    var errs []error
    for _, peer := range peers {
        if err := d.forwardToPeer(peer, update); err != nil {
            errs = append(errs, fmt.Errorf("peer %s: %w", peer.addr, err))
        }
    }

    if len(errs) > 0 {
        return errors.Join(errs...)
    }
    return nil
}

func (d *Dispatcher) forwardToPeer(peer *Peer, update *ReceivedUpdate) error {
    // Pack attributes for target peer's context
    // Note: attrs is nil for withdraw-only UPDATEs
    var attrBytes []byte
    if update.attrs != nil {
        attrBytes = update.attrs.PackFor(peer.sendCtxID)
    }

    // Pack NLRIs - reuse wire bytes if contexts match, re-encode otherwise
    announceBytes := packNLRIsFor(update.announces, update.announceWire,
                                   update.sourceCtxID, peer.sendCtxID)
    withdrawBytes := packNLRIsFor(update.withdraws, update.withdrawWire,
                                   update.sourceCtxID, peer.sendCtxID)

    // Build and send UPDATE message
    return peer.SendUpdate(withdrawBytes, attrBytes, announceBytes)
}

// packNLRIsFor returns wire bytes for NLRIs, reusing wire bytes when possible.
func packNLRIsFor(nlris []nlri.NLRI, wireBytes [][]byte,
                  srcCtx, dstCtx bgpctx.ContextID) []byte {
    if srcCtx == dstCtx {
        // Reuse wire bytes (avoids re-encoding, copies during join)
        return bytes.Join(wireBytes, nil)
    }
    // Re-encode for different context
    var buf bytes.Buffer
    for _, n := range nlris {
        n.WriteTo(&buf, dstCtx)
    }
    return buf.Bytes()
}
```

## Negated Peer Selector

### Syntax

```
peer <ip>      # Specific peer
peer *         # All peers
peer !<ip>     # All peers EXCEPT this IP
```

### Invalid Combinations

```
peer !*                    # Error: cannot exclude all
peer !10.0.0.1 !10.0.0.2   # Error: only single exclude supported
peer 10.0.0.1 !10.0.0.1    # Error: contradictory
```

### Implementation

```go
func ParseSelector(s string) (*Selector, error) {
    s = strings.TrimSpace(s)

    if s == "*" {
        return &Selector{All: true}, nil
    }

    if s == "!*" {
        return nil, fmt.Errorf("invalid selector: cannot exclude all peers")
    }

    if strings.HasPrefix(s, "!") {
        ip, err := netip.ParseAddr(s[1:])
        if err != nil {
            return nil, fmt.Errorf("invalid exclude IP %q: %w", s[1:], err)
        }
        return &Selector{Exclude: ip}, nil
    }

    ip, err := netip.ParseAddr(s)
    if err != nil {
        return nil, fmt.Errorf("invalid peer IP %q: %w", s, err)
    }
    return &Selector{IP: ip}, nil
}

type Selector struct {
    All     bool
    IP      netip.Addr
    Exclude netip.Addr
}

func (r *Reactor) GetMatchingPeers(selector *Selector) []*Peer {
    var result []*Peer

    if selector.All {
        return r.allPeers()
    }

    if selector.IP.IsValid() {
        if p := r.getPeer(selector.IP); p != nil {
            return []*Peer{p}
        }
        return nil
    }

    if selector.Exclude.IsValid() {
        for _, p := range r.peers {
            if p.addr != selector.Exclude {
                result = append(result, p)
            }
        }
        return result
    }

    return nil
}
```

## Lifecycle

| Event | Behavior |
|-------|----------|
| Receive UPDATE (announce) | Assign new update-id, add to cache, send to API |
| Receive UPDATE (withdraw) | Assign new update-id, add to cache, send to API |
| Same NLRI, new attrs | New UPDATE → new update-id |
| Forward command | Send to peers, **delete from cache** |
| Delete command | **Delete from cache** (no forwarding) |
| Cache TTL expires | Fallback cleanup for orphaned entries |

**Key:** Entries are one-shot — deleted after forward or explicit delete. TTL is safety net only.

## Flow

```
Peer A → Receive UPDATE → Assign update-id 12345 → Add to cache + Store NLRIs in RIB
                                    ↓
                        API output: { update-id: 12345, ... }
                                    ↓
                        External process decides action
                                    ↓
              ┌─────────────────────┴─────────────────────┐
              ↓                                           ↓
    forward update-id 12345                     delete update-id 12345
              ↓                                           ↓
    Cache lookup → Send → Delete                   Delete from cache
```

## Test Plan

```go
// TestUpdateIDAssignment verifies unique ID generation.
// VALIDATES: Each UPDATE gets unique ID.
// PREVENTS: ID collisions causing wrong forwarding.
func TestUpdateIDAssignment(t *testing.T)

// TestRecentUpdateCacheAdd verifies cache insertion.
// VALIDATES: Updates are cached and retrievable.
// PREVENTS: Lost updates, broken forwarding.
func TestRecentUpdateCacheAdd(t *testing.T)

// TestRecentUpdateCacheExpiry verifies TTL expiration.
// VALIDATES: Expired entries return not found.
// PREVENTS: Stale data being forwarded.
func TestRecentUpdateCacheExpiry(t *testing.T)

// TestRecentUpdateCacheLazyCleanup verifies expired entries evicted on Add.
// VALIDATES: Expired entries removed during Add().
// PREVENTS: Unbounded memory growth.
func TestRecentUpdateCacheLazyCleanup(t *testing.T)

// TestRecentUpdateCacheMaxEntries verifies fixed size limit.
// VALIDATES: Cache rejects new entries when full after eviction.
// PREVENTS: Memory exhaustion under high load.
func TestRecentUpdateCacheMaxEntries(t *testing.T)

// TestRecentUpdateCacheConcurrency verifies thread safety.
// VALIDATES: Concurrent Add/Get are safe.
// PREVENTS: Race conditions, data corruption.
func TestRecentUpdateCacheConcurrency(t *testing.T)

// TestAPIOutputIncludesUpdateID verifies API JSON has update-id field.
// VALIDATES: API output contains update-id for received UPDATEs.
// PREVENTS: Controller can't reference updates for forwarding.
func TestAPIOutputIncludesUpdateID(t *testing.T)

// TestForwardCommand verifies forward parsing and execution.
// VALIDATES: Forward command works end-to-end.
// PREVENTS: Command parsing errors, forwarding failures.
func TestForwardCommand(t *testing.T)

// TestForwardExpiredID verifies error on expired ID.
// VALIDATES: Expired IDs return clear error.
// PREVENTS: Silent failures, undefined behavior.
func TestForwardExpiredID(t *testing.T)

// TestForwardPartialFailure verifies error collection.
// VALIDATES: Failures to some peers don't stop others.
// PREVENTS: One bad peer blocking all forwarding.
func TestForwardPartialFailure(t *testing.T)

// TestForwardWithWithdrawals verifies withdrawal forwarding.
// VALIDATES: Withdrawals in UPDATE are forwarded.
// PREVENTS: Lost withdrawals, routing inconsistency.
func TestForwardWithWithdrawals(t *testing.T)

// TestNegatedSelector verifies !<ip> parsing.
// VALIDATES: !<ip> matches all except specified.
// PREVENTS: Wrong peer selection, route loops.
func TestNegatedSelector(t *testing.T)

// TestSelectorEdgeCases verifies invalid selectors rejected.
// VALIDATES: !*, contradictions return errors.
// PREVENTS: Undefined behavior on bad input.
func TestSelectorEdgeCases(t *testing.T)

// TestForwardWireByteReuse verifies wire byte reuse when contexts match.
// VALIDATES: Same context reuses wire bytes (avoids re-encoding).
// PREVENTS: Unnecessary re-encoding overhead.
func TestForwardWireByteReuse(t *testing.T)

// TestPackNLRIsForContextMismatch verifies re-encoding on context mismatch.
// VALIDATES: Different contexts trigger re-encode.
// PREVENTS: Sending wrong wire format to peer.
func TestPackNLRIsForContextMismatch(t *testing.T)
```

## Checklist

- [x] ReceivedUpdate type (with withdrawals)
- [x] Update ID generation (atomic counter)
- [x] RecentUpdateCache with TTL and lazy cleanup
- [x] Cache fixed max-entries (pre-allocated)
- [x] GetUpdateByID() method (via cache.Get)
- [x] Config: `recent-update-ttl`, `recent-update-max`
- [x] API output includes update-id
- [x] `forward update-id` command parsing
- [x] Forward command execution (parse + send to peers)
- [x] Forward auto-delete from cache after send
- [x] `delete update-id` command (ack without forwarding)
- [x] Cache Delete() method
- [x] Cache ResetTTL() method
- [x] Zero-copy forwarding (SendRawUpdateBody when contexts match)
- [x] adj-rib-out integration (ConvertToRoutes + MarkSent/RemoveFromSent)
- [x] Partial failure error collection (errors.Join)
- [x] `!<ip>` selector parsing
- [x] Selector edge case validation
- [x] GetMatchingPeers with exclude (Selector.Matches)
- [x] Tests pass
- [x] `make test && make lint` pass

## Related Documentation

- `docs/architecture/api/ARCHITECTURE.md` - Route Reflection via API section (design overview)
- `docs/architecture/api/COMMANDS.md` - Forward command syntax
- `docs/architecture/api/JSON_FORMAT.md` - JSON output format with update-id
- `docs/architecture/UPDATE_BUILDING.md` - Build vs Forward path
- `docs/architecture/ENCODING_CONTEXT.md` - Context system for zero-copy

## Dependencies

- `spec-attributes-wire.md` - AttributesWire.PackFor()

## Dependents

- `spec-rfc9234-role.md` - Uses RouteTag for policy (separate from update-id)

---

**Created:** 2026-01-01
**Updated:** 2026-01-01 - Per-UPDATE ID, time-based cache, immutable snapshot semantics
**Updated:** 2026-01-01 - Added withdrawals, max-entries, simplified lifecycle docs
**Updated:** 2026-01-01 - Lazy cleanup on Add(), no background goroutine, fixed terminology
**Updated:** 2026-01-01 - Fixed nil attrs for withdraw-only, O(n) performance note, added API output test
**Updated:** 2026-01-01 - Cross-references to design docs, updated .claude docs to use update-id terminology
**Updated:** 2026-01-01 - Changed to one-shot cache (delete on forward/delete), TTL as fallback only
**Updated:** 2026-01-01 - Implemented zero-copy forwarding, adj-rib-out integration complete
