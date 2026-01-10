# Spec: Labeled-Unicast API Completeness

## SOURCE FILES (read before implementation)

```
┌─────────────────────────────────────────────────────────────────┐
│  Read these source files before implementing:                   │
│                                                                 │
│  1. pkg/bgp/nlri/inet.go - INET NLRI pattern to follow          │
│  2. pkg/bgp/message/update_build.go:816-855 - buildLabeledUnicastNLRIBytes │
│  3. pkg/reactor/reactor.go:167-270 - AnnounceRoute pattern      │
│  4. pkg/reactor/reactor.go:522-630 - Current labeled-unicast    │
│  5. pkg/reactor/peer.go:1341-1439 - buildRIBRouteUpdate (SAFI-aware) │
│  6. pkg/rib/outgoing.go - Adj-RIB-Out methods                   │
│  7. pkg/plugin/types.go:175-183 - LabeledUnicastRoute type         │
│  8. .claude/zebgp/wire/NLRI.md - Label NLRI wire format         │
│  9. .claude/zebgp/api/ARCHITECTURE.md - Route injection flow    │
│                                                                 │
│  RFCs (already fetched):                                        │
│  - RFC 8277: Using BGP to Bind MPLS Labels to Address Prefixes  │
│  - RFC 3032: MPLS Label Stack Encoding (24-bit label in BGP)    │
│  - RFC 7911: ADD-PATH (4-byte path ID prefix)                   │
│                                                                 │
│  ON COMPLETION: Update design docs listed in Documentation      │
│  Impact section to match any design changes made.               │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Complete labeled-unicast API to match AnnounceRoute pattern:
1. **Phase 0 (BLOCKING)**: Create `nlri.LabeledUnicast` type implementing `nlri.NLRI`
2. Add Adj-RIB-Out tracking (MarkSent, QueueAnnounce)
3. Add transaction mode support (InTransaction check)
4. Add queuing for non-established peers
5. Add PathID field to api.LabeledUnicastRoute for ADD-PATH
6. Store ALL PathAttributes in rib.Route (fix attribute loss bug)

## Current State

- Tests: make test PASS, make lint PASS
- Implementation: COMPLETE (pending commit)
- Files changed:
  - `pkg/bgp/nlri/labeled.go` - NEW: nlri.LabeledUnicast type
  - `pkg/bgp/nlri/labeled_test.go` - NEW: NLRI tests
  - `pkg/bgp/message/labeled_wire_test.go` - NEW: Wire consistency tests
  - `pkg/bgp/message/update_build.go` - Fixed ADD-PATH pathID=0 bug
  - `pkg/plugin/types.go` - Added PathID to LabeledUnicastRoute
  - `pkg/plugin/route_keywords.go` - Added path-id keyword
  - `pkg/plugin/route.go` - Added path-id parsing
  - `pkg/plugin/route_parse_test.go` - Added path-id tests
  - `pkg/reactor/reactor.go` - Refactored Announce/Withdraw with 3-way switch

## Context Loaded

```
✅ Context Loading Verification:

Architecture docs:
  - .claude/zebgp/wire/NLRI.md - Label NLRI wire format (3-byte label encoding)
  - .claude/zebgp/api/ARCHITECTURE.md - Route injection flow, Adj-RIB-Out

RFCs:
  - RFC 8277 Section 2: Labeled-Unicast NLRI = Length + Label(s) + Prefix
  - RFC 3032: Label = 20-bit value + 3-bit TC + 1-bit S (BOS), NO TTL in BGP
  - RFC 7911: ADD-PATH prepends 4-byte Path ID

Source code with line numbers:
  - pkg/bgp/nlri/inet.go:42-247 - INET implements nlri.NLRI (pattern to follow)
    - Line 52-58: NewINET(family, prefix, pathID)
    - Line 166-190: Bytes() returns [pathID?][length][prefix]
    - Line 228-247: Pack(ctx) handles ADD-PATH negotiation
  - pkg/bgp/message/update_build.go:816-855 - buildLabeledUnicastNLRIBytes
    - Line 817-823: Label encoding (20-bit label, BOS=1)
    - Line 838-852: PathID handling for ADD-PATH
  - pkg/reactor/reactor.go:167-227 - AnnounceRoute (reference pattern)
    - Line 189: attrs := []attribute.Attribute{attribute.OriginIGP} ⚠️ ONLY ORIGIN
    - Line 204-207: Transaction check → QueueAnnounce
    - Line 208-219: Established → SendUpdate + MarkSent
    - Line 220-224: Not established → peer.QueueAnnounce
  - pkg/reactor/reactor.go:522-630 - Current labeled-unicast impl
    - Line 537-540: Only skips non-established peers (gap)
    - No transaction check (gap)
    - No Adj-RIB-Out tracking (gap)
  - pkg/reactor/peer.go:1341-1439 - buildRIBRouteUpdate
    - Line 1387-1417: SAFI-aware dispatch (MP_REACH_NLRI for SAFI 4) ✅
    - Line 1419-1432: Copies optional attrs from route.Attributes()
  - pkg/plugin/types.go:175-183 - LabeledUnicastRoute
    - Has: Prefix, NextHop, Labels, PathAttributes
    - Missing: PathID for ADD-PATH (gap)

Patterns identified:
  - INET: stores family, prefix, pathID, hasPath; Pack(ctx) handles ADD-PATH
  - AnnounceRoute uses 3-way switch: InTransaction → Established → QueueAnnounce
  - rib.Route built with NewRouteWithASPath for Adj-RIB-Out
  - MarkSent called after successful SendUpdate
  - buildRIBRouteUpdate IS SAFI-aware (uses route.NLRI().Family() for dispatch)
```

## Problem Analysis

### Critical Issue 1: No nlri.LabeledUnicast Type (BLOCKING)

Cannot build `rib.Route` without an NLRI type implementing `nlri.NLRI`.

Current state:
- `nlri.INET` encodes `[pathID?][length][prefix]` - NO LABEL
- `buildLabeledUnicastNLRIBytes` builds labeled NLRI bytes, but not as NLRI type
- `rib.Route` requires `nlri.NLRI` interface

Required: Create `nlri.LabeledUnicast` that implements `nlri.NLRI`:
```go
type LabeledUnicast struct {
    family  Family
    prefix  netip.Prefix
    labels  []uint32      // Label stack per RFC 3032
    pathID  uint32
    hasPath bool
}
```

### Critical Issue 2: Pre-existing Attribute Loss Bug

In `AnnounceRoute` (reactor.go:189):
```go
attrs := []attribute.Attribute{attribute.OriginIGP}  // ⚠️ ONLY Origin!
ribRoute := rib.NewRouteWithASPath(n, route.NextHop, attrs, asPath)
```

But `buildAnnounceUpdate` uses ALL PathAttributes (MED, Communities, etc.).

**Impact**: When route is queued and replayed via `buildRIBRouteUpdate`, attributes are LOST because `route.Attributes()` only has OriginIGP.

| Path | MED | Communities | LargeCommunities |
|------|-----|-------------|------------------|
| Immediate send | ✅ | ✅ | ✅ |
| Queued → replay | ❌ LOST | ❌ LOST | ❌ LOST |

**Labeled-unicast MUST NOT repeat this bug.** Store ALL attributes in rib.Route.

### Gap Analysis (confirmed)

| Feature | Status | Evidence |
|---------|--------|----------|
| Transaction support | ❌ | reactor.go:537-540 - no InTransaction check |
| Adj-RIB-Out tracking | ❌ | No MarkSent call after SendUpdate |
| Non-established queuing | ❌ | `continue` instead of `peer.QueueAnnounce` |
| PathID in API type | ❌ | api/types.go:178-183 lacks PathID field |
| Full attributes in rib.Route | ❌ | Must fix for labeled-unicast |

### Route Replay Mechanism ✅

`buildRIBRouteUpdate` (peer.go:1387-1417) IS SAFI-aware:
```go
switch {
case family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast:
    // IPv4 unicast: inline NLRI
default:
    // Other families (including SAFI 4): MP_REACH_NLRI
    mpReach := &attribute.MPReachNLRI{
        NLRI: routeNLRI.Pack(ctx),  // Uses NLRI's Pack method ✅
    }
}
```

If `nlri.LabeledUnicast.Pack(ctx)` returns correct bytes, replay works automatically.

## RFC Wire Format (MANDATORY)

### RFC 8277: Labeled-Unicast NLRI

```
Without ADD-PATH:
┌──────────────────────────────────┐
│ Length (1 byte)                  │ = 24*N + prefix_bits
├──────────────────────────────────┤
│ Label 1 (3 bytes, S=0)           │ ← 20-bit label + 3-bit TC + 1-bit S
│ ...                              │
│ Label N (3 bytes, S=1)           │ ← Last label has S=1 (BOS)
├──────────────────────────────────┤
│ Prefix (variable)                │ = ceiling(prefix_bits/8) bytes
└──────────────────────────────────┘

With ADD-PATH (RFC 7911):
┌──────────────────────────────────┐
│ Path ID (4 bytes)                │ ← Prepended when negotiated
├──────────────────────────────────┤
│ Length (1 byte)                  │
├──────────────────────────────────┤
│ Label(s) (3*N bytes)             │
├──────────────────────────────────┤
│ Prefix (variable)                │
└──────────────────────────────────┘
```

### RFC 3032: MPLS Label Encoding (3 bytes in BGP)

```
 0                   1                   2
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Label Value (20 bits)        |TC |S|
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

- Label Value: 20 bits (0-1048575)
- TC: 3 bits (Traffic Class, was Exp)
- S: 1 bit (Stack bit: 0=more labels, 1=bottom of stack)

NOTE: RFC 3032 data plane uses 4 bytes (includes TTL).
      BGP uses 3 bytes (no TTL) per RFC 8277.
```

### RFC 8277: Withdrawal

```
Withdrawal uses MP_UNREACH_NLRI with same NLRI format.
Compatibility field SHOULD be 0x800000 (withdrawal label).
Label MUST match the originally announced label.
```

## Goal Achievement

| Check | Status |
|-------|--------|
| Phase 0: nlri.LabeledUnicast type | ✅ Complete |
| Phase 1: PathID in api type | ✅ Complete |
| Phase 2: Build rib.Route with ALL attrs | ✅ Complete (incl. LocalPref) |
| Phase 3: Transaction support | ✅ Complete (code, no unit test) |
| Phase 4: Adj-RIB-Out tracking | ✅ Complete (code, no unit test) |
| Phase 5: Non-established queuing | ✅ Complete (code, no unit test) |
| Phase 6: Wire format consistency tests | ✅ Complete |
| Tests exist | ✅ NLRI + wire consistency tests |

**Plan achieves goal:** YES - Core implementation complete

## Embedded Rules

- TDD: test must fail before impl
- Verify: make test && make lint before done
- RFC: RFC 8277 for labeled-unicast, RFC 3032 for label encoding, RFC 7911 for ADD-PATH
- Wire format: nlri.LabeledUnicast.Pack() MUST match buildLabeledUnicastNLRIBytes()

## Documentation Impact

- [ ] `.claude/zebgp/wire/NLRI.md` - Add LabeledUnicast section
- [ ] `.claude/zebgp/api/ARCHITECTURE.md` - Add labeled-unicast to route injection flow
- [ ] `docs/plan/CLAUDE_CONTINUATION.md` - Update after implementation

## Implementation Steps

### Phase 0: Create nlri.LabeledUnicast Type (BLOCKING)

**TDD: Write tests first**

1. Test `nlri.LabeledUnicast` interface compliance
2. Test `Pack(ctx)` wire format matches RFC 8277:
   - Single label: `[length][label][prefix]`
   - Label stack: `[length][label1][label2][prefix]` with BOS on last
   - ADD-PATH: `[pathID][length][labels][prefix]`
3. Test wire format consistency with `buildLabeledUnicastNLRIBytes`:
   ```go
   func TestLabeledUnicastWireConsistency(t *testing.T) {
       // Build via UpdateBuilder path
       params := message.LabeledUnicastParams{...}
       expected := ub.buildLabeledUnicastNLRIBytes(params)

       // Build via nlri.LabeledUnicast path
       n := nlri.NewLabeledUnicast(family, prefix, labels, pathID)
       actual := n.Pack(ctx)

       require.Equal(t, expected, actual)
   }
   ```

**Implementation**

Create `pkg/bgp/nlri/labeled.go`:
```go
// LabeledUnicast represents a labeled unicast NLRI (SAFI 4).
// RFC 8277: Using BGP to Bind MPLS Labels to Address Prefixes.
type LabeledUnicast struct {
    family  Family
    prefix  netip.Prefix
    labels  []uint32      // Label stack per RFC 3032 (BOS on last)
    pathID  uint32
    hasPath bool
}

// NewLabeledUnicast creates a new labeled unicast NLRI.
// Labels are encoded per RFC 3032: 20-bit label + 3-bit TC + 1-bit S.
// The last label has S=1 (Bottom of Stack).
func NewLabeledUnicast(family Family, prefix netip.Prefix, labels []uint32, pathID uint32) *LabeledUnicast

// Implement nlri.NLRI interface:
// - Family() Family
// - PathID() uint32
// - HasPathID() bool
// - Bytes() []byte
// - Pack(ctx *PackContext) []byte
// - Len() int
// - String() string
```

### Phase 1: Add PathID to API Type

1. Add `PathID uint32` to `api.LabeledUnicastRoute` (types.go)
2. Wire PathID through to `LabeledUnicastParams` in `AnnounceLabeledUnicast`
3. Test PathID encoding in wire output

### Phase 2: Build rib.Route with ALL Attributes

Create helper that stores ALL PathAttributes (not just Origin):

```go
func (a *reactorAPIAdapter) buildLabeledUnicastRIBRoute(
    route api.LabeledUnicastRoute,
    isIBGP bool,
) *rib.Route {
    // 1. Build NLRI
    family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
    if route.Prefix.Addr().Is6() {
        family.AFI = nlri.AFIIPv6
    }
    label := uint32(0)
    if len(route.Labels) > 0 {
        label = route.Labels[0]
    }
    n := nlri.NewLabeledUnicast(family, route.Prefix, []uint32{label}, route.PathID)

    // 2. Build attributes - MUST INCLUDE ALL (unlike AnnounceRoute bug)
    var attrs []attribute.Attribute

    // Origin
    if route.Origin != nil {
        attrs = append(attrs, attribute.Origin(*route.Origin))
    } else {
        attrs = append(attrs, attribute.OriginIGP)
    }

    // MED
    if route.MED != nil {
        attrs = append(attrs, attribute.MED(*route.MED))
    }

    // Communities
    if len(route.Communities) > 0 {
        comms := make(attribute.Communities, len(route.Communities))
        for i, c := range route.Communities {
            comms[i] = attribute.Community(c)
        }
        attrs = append(attrs, comms)
    }

    // LargeCommunities
    if len(route.LargeCommunities) > 0 {
        lcs := make(attribute.LargeCommunities, len(route.LargeCommunities))
        for i, lc := range route.LargeCommunities {
            lcs[i] = attribute.LargeCommunity{
                GlobalAdmin: lc.GlobalAdmin,
                LocalData1:  lc.LocalData1,
                LocalData2:  lc.LocalData2,
            }
        }
        attrs = append(attrs, lcs)
    }

    // ExtendedCommunities
    if len(route.ExtendedCommunities) > 0 {
        attrs = append(attrs, attribute.ExtendedCommunities(route.ExtendedCommunities))
    }

    // 3. Build AS-PATH
    var asPath *attribute.ASPath
    if len(route.ASPath) > 0 {
        asPath = &attribute.ASPath{
            Segments: []attribute.ASPathSegment{
                {Type: attribute.ASSequence, ASNs: route.ASPath},
            },
        }
    } else if isIBGP {
        asPath = &attribute.ASPath{Segments: nil}
    } else {
        asPath = &attribute.ASPath{
            Segments: []attribute.ASPathSegment{
                {Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
            },
        }
    }

    return rib.NewRouteWithASPath(n, route.NextHop, attrs, asPath)
}
```

### Phase 3: Refactor AnnounceLabeledUnicast

Refactor to match AnnounceRoute pattern:

```go
func (a *reactorAPIAdapter) AnnounceLabeledUnicast(peerSelector string, route api.LabeledUnicastRoute) error {
    peers := a.getMatchingPeers(peerSelector)
    if len(peers) == 0 {
        return errors.New("no peers match selector")
    }

    var lastErr error
    for _, peer := range peers {
        isIBGP := peer.Settings().IsIBGP()

        // Build rib.Route with ALL attributes
        ribRoute := a.buildLabeledUnicastRIBRoute(route, isIBGP)

        switch {
        case peer.AdjRIBOut().InTransaction():
            // Queue to Adj-RIB-Out (will be sent on commit)
            peer.AdjRIBOut().QueueAnnounce(ribRoute)

        case peer.State() == PeerStateEstablished:
            // Send immediately and track
            family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
            if route.Prefix.Addr().Is6() {
                family.AFI = nlri.AFIIPv6
            }
            ctx := peer.packContext(family)

            // Build UPDATE using existing builder (for immediate send)
            ub := message.NewUpdateBuilder(a.r.config.LocalAS, isIBGP, ctx)
            params := a.buildLabeledUnicastParams(route)
            update := ub.BuildLabeledUnicast(params)

            if err := peer.SendUpdate(update); err != nil {
                lastErr = err
            } else {
                // Track sent route for re-announcement on reconnect
                peer.AdjRIBOut().MarkSent(ribRoute)
            }

        default:
            // Session not established: queue to peer's operation queue
            peer.QueueAnnounce(ribRoute)
        }
    }
    return lastErr
}
```

### Phase 4: Refactor WithdrawLabeledUnicast

Same pattern for withdrawals:

```go
func (a *reactorAPIAdapter) WithdrawLabeledUnicast(peerSelector string, route api.LabeledUnicastRoute) error {
    // Build nlri.LabeledUnicast for withdrawal
    // RFC 8277: withdrawal MUST include original label

    switch {
    case peer.AdjRIBOut().InTransaction():
        peer.AdjRIBOut().QueueWithdraw(n)
    case peer.State() == PeerStateEstablished:
        // Send immediately
        if err := peer.SendUpdate(update); err != nil {
            lastErr = err
        } else {
            peer.AdjRIBOut().RemoveFromSent(n)
        }
    default:
        peer.QueueWithdraw(n)
    }
}
```

### Phase 5: Wire PathID Through

1. Map `route.PathID` to `params.PathID` in `buildLabeledUnicastParams`
2. Map PathID to `nlri.LabeledUnicast` construction
3. Test ADD-PATH encoding with PathID

### Phase 6: Verification

```bash
make test && make lint
```

## Wire Format Consistency Tests (CRITICAL)

Two code paths must produce identical wire format:
1. **Immediate send**: `BuildLabeledUnicast(params)` → `buildLabeledUnicastNLRIBytes`
2. **Queued replay**: `buildRIBRouteUpdate(route)` → `nlri.LabeledUnicast.Pack(ctx)`

Test cases:
- Single label, no ADD-PATH
- Single label, with ADD-PATH
- Label stack (2-3 labels), with BOS
- IPv4 and IPv6 prefixes
- Various prefix lengths (0, 8, 24, 32, 128)

## Dependencies

- `nlri.LabeledUnicast` must exist before Phases 2-5 can proceed
- Phase 2 (rib.Route builder) must be complete before Phases 3-4

## Out of Scope

**Pre-existing bug**: `AnnounceRoute` also has attribute loss bug (only stores OriginIGP).
- File as separate issue
- Fix independently (same pattern as Phase 2)

## Checklist

- [x] Phase 0: nlri.LabeledUnicast type created
- [x] Phase 0: Wire format tests pass
- [x] Phase 0: Consistency tests pass (Pack vs buildLabeledUnicastNLRIBytes)
- [x] Phase 1: PathID field added to api.LabeledUnicastRoute
- [x] Phase 2: buildLabeledUnicastRIBRoute stores ALL attributes
- [x] Phase 3: AnnounceLabeledUnicast uses 3-way switch
- [x] Phase 3: Transaction mode works (QueueAnnounce) - code complete, no unit test
- [x] Phase 3: Adj-RIB-Out tracking works (MarkSent) - code complete, no unit test
- [x] Phase 3: Non-established queuing works (peer.QueueAnnounce) - code complete, no unit test
- [x] Phase 4: WithdrawLabeledUnicast uses 3-way switch
- [x] Phase 4: Withdraw transaction mode works - code complete, no unit test
- [x] Phase 4: Withdraw RemoveFromSent works - code complete, no unit test
- [x] Phase 5: PathID passed to builder
- [x] Phase 6: make test passes
- [x] Phase 6: make lint passes
- [x] Tests fail first (TDD) - for nlri.LabeledUnicast
- [x] Tests pass after impl
- [x] Goal achieved
- [ ] Documentation updated
- [ ] Spec moved to docs/plan/done/
- [ ] docs/plan/README.md updated

## Additional Fixes Applied

- [x] Fixed buildLabeledUnicastNLRIBytes ADD-PATH pathID=0 bug (RFC 7911 compliance)
- [x] Added LocalPref to buildLabeledUnicastRIBRoute (was missing)

## Known Limitations (Pre-existing)

- LabeledUnicastParams only supports single label (not label stack)
- Immediate send uses single label; queued replay preserves full stack via nlri.LabeledUnicast
