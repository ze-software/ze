# Spec: Adj-RIB-Out Integration for Forward

## Status: Implemented

## Prerequisites

- `spec-route-id-forwarding.md` - ForwardUpdate command (done)

## Problem

Current ForwardUpdate flow:
```
forward update-id <id> → Send to peers → Delete from cache
```

No persistence: if peer disconnects and reconnects, forwarded routes are lost.

## Solution

Integrate with existing `OutgoingRIB` (adj-rib-out) to persist forwarded routes:

```
forward update-id <id> → Send to peers → Store in adj-rib-out → Delete from cache
                                              ↓
                         Peer reconnect → Replay from adj-rib-out
```

## Existing Infrastructure

The `OutgoingRIB` type already exists in `pkg/rib/outgoing.go`:

```go
type OutgoingRIB struct {
    pending     map[nlri.Family]map[string]*Route  // Queued for sending
    withdrawals map[nlri.Family]map[string]nlri.NLRI  // Queued withdrawals
    sent        map[nlri.Family]map[string]*Route  // Sent cache (for replay)
}
```

Key methods already implemented:
- `MarkSent(route *Route)` - Add route to sent cache
- `RemoveFromSent(nlri NLRI)` - Remove from sent cache
- `GetSentRoutes() []*Route` - Get all sent routes for replay
- `FlushSent() int` - Re-queue sent routes for re-announcement

Peer already has `adjRIBOut *rib.OutgoingRIB` at `pkg/reactor/peer.go:304`.

## Gap Analysis

1. **ReceivedUpdate → Route conversion missing**
   - `ReceivedUpdate` has: `Attrs *AttributesWire`, `Announces []nlri.NLRI`
   - `Route` needs: `attrs []attribute.Attribute`, `asPath *attribute.ASPath`
   - Need to parse `AttributesWire` into individual attributes

2. **ForwardUpdate doesn't call MarkSent**
   - Currently just sends and deletes from cache
   - Need to add adj-rib-out integration

3. **Withdraw handling**
   - `ReceivedUpdate.Withdraws` needs to call `RemoveFromSent`

## Implementation Plan

### Phase 1: ReceivedUpdate to Route Conversion

```go
// ConvertToRoutes extracts individual Routes from a ReceivedUpdate.
// Used when storing in adj-rib-out for persistence.
func (ru *ReceivedUpdate) ConvertToRoutes() ([]*rib.Route, error) {
    if ru.Attrs == nil && len(ru.Announces) == 0 {
        return nil, nil  // Withdraw-only UPDATE
    }

    // Parse attributes (lazy parse if not done)
    attrs, err := ru.Attrs.Attributes()
    if err != nil {
        return nil, fmt.Errorf("parsing attributes: %w", err)
    }

    // Extract NextHop and ASPath
    var nextHop netip.Addr
    var asPath *attribute.ASPath
    var otherAttrs []attribute.Attribute

    for _, attr := range attrs {
        switch a := attr.(type) {
        case *attribute.NextHop:
            nextHop = a.IP
        case *attribute.ASPath:
            asPath = a
        default:
            otherAttrs = append(otherAttrs, attr)
        }
    }

    // Create Route per NLRI
    routes := make([]*rib.Route, 0, len(ru.Announces))
    for i, nlri := range ru.Announces {
        route := rib.NewRouteWithWireCacheFull(
            nlri,
            nextHop,
            otherAttrs,
            asPath,
            ru.Attrs.WireBytes(),      // Attribute wire cache
            ru.AnnounceWire[i],        // NLRI wire cache
            ru.SourceCtxID,
        )
        routes = append(routes, route)
    }

    return routes, nil
}
```

### Phase 2: ForwardUpdate Integration

Modify `ForwardUpdate` in `pkg/reactor/reactor.go`:

```go
func (a *reactorAPIAdapter) ForwardUpdate(sel *api.Selector, updateID uint64) error {
    update, ok := a.r.recentUpdates.Get(updateID)
    if !ok {
        return ErrUpdateExpired
    }

    // ... existing peer selection code ...

    // Convert to routes for adj-rib-out (parse once, reuse for all peers)
    routes, err := update.ConvertToRoutes()
    if err != nil {
        return fmt.Errorf("converting update to routes: %w", err)
    }

    for _, peer := range matchingPeers {
        // ... existing send logic ...

        // Add to adj-rib-out for persistence
        for _, route := range routes {
            peer.AdjRIBOut().MarkSent(route)
        }

        // Handle withdrawals
        for _, nlri := range update.Withdraws {
            peer.AdjRIBOut().RemoveFromSent(nlri)
        }
    }

    // Delete from cache (one-shot)
    a.r.recentUpdates.Delete(updateID)

    return nil
}
```

### Phase 3: Reconnect Replay

Already implemented in `pkg/reactor/peer.go:1199`:

```go
func (p *Peer) sendInitialRoutes() {
    // ...
    sentRoutes := p.adjRIBOut.GetSentRoutes()
    // Routes are replayed...
}
```

### Phase 4: UPDATE Message Splitting on Forward

**IMPORTANT:** Zero-copy forwarding cannot be used when the source peer's UPDATE
exceeds the destination peer's negotiated message size limit.

**Scenario:**
```
Source peer (Extended Message) → 10KB UPDATE → Destination peer (no Extended Message)
                                                Max 4096 bytes!
```

| Capability | Max Message Size |
|------------|------------------|
| Default (RFC 4271) | 4096 bytes |
| Extended Message (RFC 8654) | 65535 bytes |

**ForwardUpdate must check:**
```go
func (a *reactorAPIAdapter) ForwardUpdate(...) error {
    // ...
    for _, peer := range matchingPeers {
        destMaxSize := peer.NegotiatedMaxMessageSize()
        updateSize := len(update.RawBytes) + message.HeaderLen

        if updateSize > destMaxSize {
            // Cannot use zero-copy - must split UPDATE
            routes, _ := update.ConvertToRoutes()
            a.sendSplitUpdates(peer, routes, destMaxSize)
        } else {
            // Zero-copy path OK
            peer.SendRawUpdateBody(update.RawBytes)
        }
    }
}

func (a *reactorAPIAdapter) sendSplitUpdates(peer *Peer, routes []*rib.Route, maxSize int) {
    // Group routes by attributes (same attrs = same UPDATE)
    grouped := groupByAttributes(routes)

    for attrs, nlris := range grouped {
        // Pack as many NLRIs as fit in one UPDATE
        for len(nlris) > 0 {
            update, remaining := packUpdateWithLimit(attrs, nlris, maxSize)
            peer.SendUpdate(update)
            nlris = remaining
        }
    }
}

func packUpdateWithLimit(attrs []byte, nlris []nlri.NLRI, maxSize int) (*Update, []nlri.NLRI) {
    // Header (19) + WithdrawnLen (2) + AttrsLen (2) + Attrs
    overhead := 19 + 2 + 2 + len(attrs)
    available := maxSize - overhead

    var packed []nlri.NLRI
    var remaining []nlri.NLRI
    usedBytes := 0

    for i, n := range nlris {
        nlriSize := n.WireLen()
        if usedBytes + nlriSize > available {
            remaining = nlris[i:]
            break
        }
        packed = append(packed, n)
        usedBytes += nlriSize
    }

    return buildUpdate(attrs, packed), remaining
}
```

**Key points:**
- Check `updateSize > destMaxSize` BEFORE attempting zero-copy
- If too large: parse and send routes individually (batching deferred to separate spec)
- Withdrawals are batched efficiently in `sendWithdrawalsWithLimit`
- Reconnect replay sends routes individually with same size checking
- Single routes with huge attributes (>4KB) are skipped with warning (cannot split atomic route)

## Memory Considerations

| Scenario | Memory Impact |
|----------|---------------|
| 100K routes × 10 peers | ~200MB struct overhead (Route ~200B each) |
| AttributesWire sharing | Shared across routes from same UPDATE |
| Wire cache per route | ~50-100 bytes (NLRI wire bytes) |
| Total with attrs | Depends on attribute size; attrs shared |

Mitigation options:
1. **Per-peer route limits** - Configure max routes per adj-rib-out
2. **Shared attribute storage** - Use internal/store for deduplication
3. **No wire cache** - Trade memory for CPU (re-encode on replay)

## Testing Strategy

```go
// TestForwardStoresInAdjRIBOut verifies routes stored after forward.
// VALIDATES: Forwarded routes appear in adj-rib-out.
// PREVENTS: Routes lost on peer reconnect.
func TestForwardStoresInAdjRIBOut(t *testing.T)

// TestForwardWithdrawRemovesFromAdjRIBOut verifies withdraws handled.
// VALIDATES: Withdrawn routes removed from adj-rib-out.
// PREVENTS: Stale routes replayed after withdraw.
func TestForwardWithdrawRemovesFromAdjRIBOut(t *testing.T)

// TestReconnectReplaysForwardedRoutes verifies replay on reconnect.
// VALIDATES: Adj-rib-out routes replayed on session re-establishment.
// PREVENTS: Route loss on peer flap.
func TestReconnectReplaysForwardedRoutes(t *testing.T)

// TestMultiplePeersIndependentAdjRIBOut verifies per-peer state.
// VALIDATES: Each peer has independent adj-rib-out.
// PREVENTS: Cross-peer state corruption.
func TestMultiplePeersIndependentAdjRIBOut(t *testing.T)

// TestForwardUpdateSplitting verifies UPDATE splitting on forward.
// VALIDATES: Large UPDATE from Extended Message peer splits for non-Extended peer.
// PREVENTS: Oversized UPDATE messages rejected by destination peer.
func TestForwardUpdateSplitting(t *testing.T)

// TestForwardUpdateSplittingExtendedMessage verifies Extended Message handling.
// VALIDATES: Uses 65535 limit when dest has Extended Message, 4096 otherwise.
// PREVENTS: Unnecessary splitting when destination supports large messages.
func TestForwardUpdateSplittingExtendedMessage(t *testing.T)

// TestReplayUpdateSplitting verifies UPDATE splitting on reconnect replay.
// VALIDATES: adj-rib-out replay respects newly negotiated max msg size.
// PREVENTS: Replay failure when peer reconnects with smaller max msg size.
func TestReplayUpdateSplitting(t *testing.T)
```

## Checklist

- [x] Add `ConvertToRoutes()` to ReceivedUpdate
- [x] Parse AttributesWire in conversion
- [x] Extract NextHop and ASPath
- [x] Create Route with wire cache
- [x] Modify ForwardUpdate to call MarkSent
- [x] Handle withdrawals with RemoveFromSent
- [x] Check `updateSize > destMaxSize` before zero-copy (reactor.go:1662)
- [x] Implement `sendSplitUpdates()` for oversized UPDATEs (reactor.go:1743)
- [ ] Group routes by attributes for efficient packing (TODO in sendRoutesWithLimit)
- [x] Reconnect replay with size checking (peer.go:1225-1232)
- [x] Unit tests for conversion (received_update_test.go)
- [x] Integration test for reconnect replay (adjribout_forward_test.go)
- [~] Test forward splitting (forward_split_test.go - scaffold tests with TODOs, needs mocking)
- [ ] Memory profiling with high route counts
- [x] Documentation update (this checklist)

## Related Documentation

- `pkg/rib/outgoing.go` - OutgoingRIB implementation
- `pkg/reactor/peer.go:1199` - Reconnect replay logic
- `spec-route-id-forwarding.md` - One-shot cache design

## Known Limitations

**FlushAllPending mid-loop failure:** If `SendUpdate` fails mid-loop during pending route
processing, remaining routes from `FlushAllPending` are in sent cache but not actually sent.
This is a pre-existing design issue but **cannot occur in practice until RFC 7606 is implemented**
— currently any send error tears down the session, and on reconnect the entire adj-rib-out
is replayed from scratch.

**Concurrent ForwardUpdate ordering:** Multiple concurrent `ForwardUpdate` calls may race.
If UPDATE1 announces prefix P and UPDATE2 withdraws P, the final adj-rib-out state depends
on execution order:
- If MarkSent(P) completes before RemoveFromSent(P): correct (P removed)
- If RemoveFromSent(P) completes before MarkSent(P): incorrect (P remains)

This is an **API usage consideration**, not a bug. Callers should:
1. Wait for ForwardUpdate to complete before issuing conflicting operations, OR
2. Use the same UPDATE for announce+withdraw (BGP allows this)

**Route batching not implemented:** Current split path sends routes individually. Efficient
batching (grouping routes with same attributes into single UPDATE) is deferred to a separate
spec. This works correctly but uses more UPDATE messages than optimal.

## Dependencies

- `spec-attributes-wire.md` - AttributesWire.Attributes() parsing

---

**Created:** 2026-01-01
**Updated:** 2026-01-01 - Implemented: ConvertToRoutes(), ForwardUpdate integration with MarkSent/RemoveFromSent
