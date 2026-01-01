# Spec: Peer Encoding Cleanup

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. plan/CLAUDE_CONTINUATION.md - Current state                 │
│  3. THIS SPEC FILE - Design requirements                        │
│  4. pkg/bgp/message/update_build.go - New implementation        │
│  5. pkg/reactor/peer.go - Old code to remove                    │
│  6. pkg/reactor/wire_compat_test.go - Existing wire tests       │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

1. Fix production bug: ORIGINATOR_ID/CLUSTER_LIST silently dropped in non-grouped paths
2. Fix grouping bug: `routeGroupKey` missing fields causes silent data loss
3. Complete migration from old UPDATE builders to `UpdateBuilder`
4. Delete ~500 LOC of dead code

## CRITICAL: Production Bug (Worse Than Initially Documented)

**Bug:** ORIGINATOR_ID and CLUSTER_LIST are ONLY encoded in `buildGroupedUpdate`. Both the old `buildStaticRouteUpdate` AND the new path are broken.

```
StaticRoute.OriginatorID  →  toStaticRouteUnicastParams()  →  UnicastParams (NO FIELD!)
                                        ↓
                              DATA LOST HERE

StaticRoute.OriginatorID  →  buildStaticRouteUpdate()  →  (NOT ENCODED!)
                                        ↓
                              ALSO LOST HERE (old code too!)
```

### Corrected Bug Matrix

| Code Path | Function | Location | ORIGINATOR_ID/CLUSTER_LIST |
|-----------|----------|----------|---------------------------|
| `GroupUpdates=true` | `buildGroupedUpdate` | peer.go:1995-2008 | ✅ Encoded |
| `GroupUpdates=false` (old) | `buildStaticRouteUpdate` | peer.go:1635-1790 | ❌ **NEVER HAD IT** |
| `GroupUpdates=false` (new) | `buildStaticRouteUpdateNew` | Uses BuildUnicast | ❌ Missing |
| VPN routes | `BuildVPN` | update_build.go:448-462 | ✅ Encoded |
| LabeledUnicast | `BuildLabeledUnicast` | update_build.go:710-724 | ✅ Encoded |

**Key insight:** The bug predates the migration. The new path inherited the bug from `buildStaticRouteUpdate`, which never encoded these fields. Only `buildGroupedUpdate` got it right.

### Additional Bug: `routeGroupKey` Missing Fields

`routeGroupKey` (peer.go:1847-1862) does NOT include `OriginatorID`, `ClusterList`, or `RawAttributes` in the grouping key:

```go
// Current key includes:
// NextHop, Origin, LocalPreference, MED, Communities, LargeCommunities,
// ExtCommunityBytes, RD, Is4, prefixKey, ASPath, AtomicAggregate, AggregatorASN, AggregatorIP
//
// MISSING: OriginatorID, ClusterList, RawAttributes!
```

**Result:** Routes with DIFFERENT `OriginatorID` values can be grouped together. `BuildGroupedUnicast` uses only the first route's attributes → **Silent data loss for remaining routes!**

## Background

Most extraction work is **already done**:
- `pkg/bgp/message/update_build.go` (1535 LOC) exists with `UpdateBuilder`
- `BuildUnicast`, `BuildVPN`, `BuildLabeledUnicast`, `BuildMVPN`, `BuildVPLS`, `BuildFlowSpec`, `BuildMUP` all exist
- `buildStaticRouteUpdateNew` uses `UpdateBuilder` and is already in production
- Conversion helpers exist: `toStaticRouteUnicastParams`, `toStaticRouteVPNParams`, `toStaticRouteLabeledUnicastParams`
- **wire_compat_test.go exists** (220 LOC) - but compares two broken implementations!

**What remains:**
1. Fix missing attributes in UnicastParams (ORIGINATOR_ID, CLUSTER_LIST)
2. Fix `routeGroupKey` to include these fields
3. Migrate `buildGroupedUpdate` to use `UpdateBuilder`
4. Delete old functions

## Known Limitations (Out of Scope)

These pre-existing issues are NOT addressed by this spec:

1. **VPN grouping bug** - `buildGroupedUpdate` creates multiple MP_REACH_NLRI attributes when grouping VPN routes with same RD. Code has comment: "VPN routes need separate handling - for now, just use first."

2. **LabeledUnicast in grouped path** - LabeledUnicast routes fall through to unicast handling in `buildGroupedUpdate`, missing SAFI=4 MP_REACH_NLRI encoding.

3. **VPNParams missing RawAttributes** - VPN routes cannot pass through custom attributes. Future work.

4. **RawAttributes not in routeGroupKey** - Routes differing only in RawAttributes could be grouped. Low risk since custom attributes are rare and typically unique.

Both grouping bugs are latent issues in rarely-used code paths (GroupUpdates + VPN/LabeledUnicast).

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- Verify before claiming: run commands, paste output as proof

## Current State Analysis

### Attribute Support Matrix

| Params Struct | ORIGINATOR_ID | CLUSTER_LIST | RawAttributes |
|---------------|---------------|--------------|---------------|
| UnicastParams | ❌ Missing | ❌ Missing | ✅ Has |
| VPNParams | ✅ Has | ✅ Has | ❌ Missing |
| LabeledUnicastParams | ✅ Has | ✅ Has | ✅ Has |
| MVPNParams | ❌ Not needed | ❌ Not needed | ❌ Not needed |
| VPLSParams | ✅ Has | ✅ Has | ❌ Not needed |
| FlowSpecParams | ❌ Not needed | ❌ Not needed | ❌ Not needed |
| MUPParams | ❌ Not needed | ❌ Not needed | ❌ Not needed |

**Type confirmed:** `attribute.ClusterList` is `[]uint32` (simple.go:241), so `copy()` works directly.

### Functions to DELETE (after migration)

| Function | Used By |
|----------|---------|
| `buildStaticRouteUpdate` | wire_compat_test.go, peer_test.go only |
| `buildGroupedUpdate` | `sendInitialRoutes` (to be migrated) |
| `buildMPReachNLRI` | above functions |
| `buildVPNNLRIBytes` | `buildMPReachNLRI` |
| `buildMPReachNLRIExtNHUnicast` | `buildStaticRouteUpdate` |
| `buildMPReachNLRIUnicast` | above functions |

### Functions to KEEP in reactor

| Function | Reason |
|----------|--------|
| `routeGroupKey` | Uses `StaticRoute` (reactor type) - needs update |
| `groupRoutesByAttributes` | Uses `StaticRoute` (reactor type) |
| `toStaticRoute*Params` | Conversion helpers |
| `packRawAttribute` | Used by conversion helpers |

## Verified Correct (From Code Review)

The following aspects have been verified correct:

1. **Context threading for ADD-PATH**: `packContext` → `NewUpdateBuilder` → `ub.Ctx` → `inet.Pack(ctx)` correctly propagates ADD-PATH flag
2. **VPN routes excluded from grouping**: `routeGroupKey` includes `r.RD`, so VPN routes get unique keys
3. **IPv6 routes excluded from grouping**: `routeGroupKey` includes prefix in key for IPv6
4. **Attribute ordering**: Both paths sort by type code before packing (RFC 4271 Appendix F.3)
5. **Single-route groups**: Correctly fallback to `buildStaticRouteUpdateNew`
6. **Error handling**: Matches existing behavior (break on error)

## Implementation Notes

### Attribute Packing

The implementation uses `attribute.PackAttributesOrdered(attrs)`. Verify this function exists in `pkg/bgp/attribute/`. If not, use the existing packing pattern from `BuildUnicast`.

### Format String Count

When updating `routeGroupKey`, verify the format string has correct number of `%` specifiers:
- Original: 14 specifiers
- Updated: 16 specifiers (adding OriginatorID, ClusterList)

### Future Refactoring Opportunity

`BuildGroupedUnicast` duplicates ~90% of `BuildUnicast`. Consider extracting common attribute building into a shared helper:

```go
func (ub *UpdateBuilder) buildCommonAttrs(p UnicastParams) []attribute.Attribute { ... }
```

This is out of scope for this spec but would reduce maintenance burden.

## Implementation Steps

### Phase 1: Add Fields to UnicastParams

**Note on TDD:** Traditional TDD requires a failing test before implementation. However, we cannot write a test that references `UnicastParams.OriginatorID` until the field exists (won't compile). Therefore, Phase 1 adds the struct fields, and Phase 2 writes the failing test.

**File: `pkg/bgp/message/update_build.go`**

1. Add to `UnicastParams` struct (after `RawAttributeBytes`):
   ```go
   // ORIGINATOR_ID (RFC 4456) - 0 means not set
   OriginatorID uint32

   // CLUSTER_LIST (RFC 4456)
   ClusterList []uint32
   ```

2. Run `make test` - should pass (fields added but not used yet)

**File: `pkg/reactor/peer.go`**

3. Update `toStaticRouteUnicastParams` to copy the new fields:
   ```go
   return message.UnicastParams{
       // ... existing fields ...
       RawAttributeBytes:  rawAttrs,
       OriginatorID:       r.OriginatorID,  // ADD
       ClusterList:        r.ClusterList,   // ADD
   }
   ```

4. Add unit test to verify copying works:

**File: `pkg/reactor/peer_test.go`**
   ```go
   // TestToStaticRouteUnicastParams_CopiesReflectorAttrs verifies RFC 4456 fields.
   //
   // VALIDATES: OriginatorID and ClusterList are copied to UnicastParams.
   // PREVENTS: Silent data loss for route reflector attributes.
   func TestToStaticRouteUnicastParams_CopiesReflectorAttrs(t *testing.T) {
       route := StaticRoute{
           Prefix:       netip.MustParsePrefix("10.0.0.0/24"),
           NextHop:      netip.MustParseAddr("192.168.1.1"),
           OriginatorID: 0xC0A80101,
           ClusterList:  []uint32{0xC0A80102, 0xC0A80103},
       }
       nf := &NegotiatedFamilies{IPv4Unicast: true}

       params := toStaticRouteUnicastParams(route, nf)

       if params.OriginatorID != route.OriginatorID {
           t.Errorf("OriginatorID not copied: got %x, want %x",
               params.OriginatorID, route.OriginatorID)
       }
       if len(params.ClusterList) != len(route.ClusterList) {
           t.Fatalf("ClusterList length mismatch: got %d, want %d",
               len(params.ClusterList), len(route.ClusterList))
       }
       for i, v := range route.ClusterList {
           if params.ClusterList[i] != v {
               t.Errorf("ClusterList[%d] mismatch: got %x, want %x",
                   i, params.ClusterList[i], v)
           }
       }
   }
   ```

5. Run `make test` - should pass

### Phase 2: Fix BuildUnicast with Expected-Bytes Test (TDD)

**IMPORTANT:** Cannot compare old `buildStaticRouteUpdate` vs new - both are broken!
Must use expected bytes or compare against `buildGroupedUpdate` (only correct implementation).

**File: `pkg/bgp/message/update_build_test.go`**

1. Add test that verifies ORIGINATOR_ID/CLUSTER_LIST are encoded:
   ```go
   // TestBuildUnicast_EncodesReflectorAttrs verifies RFC 4456 attribute encoding.
   //
   // VALIDATES: ORIGINATOR_ID and CLUSTER_LIST are encoded in PathAttributes.
   // PREVENTS: Data loss for route reflector configurations.
   func TestBuildUnicast_EncodesReflectorAttrs(t *testing.T) {
       ctx := &nlri.PackContext{ASN4: true}
       ub := NewUpdateBuilder(65001, true, ctx)

       params := UnicastParams{
           Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
           NextHop:         netip.MustParseAddr("192.168.1.1"),
           Origin:          attribute.OriginIGP,
           LocalPreference: 100,
           OriginatorID:    0xC0A80101, // 192.168.1.1
           ClusterList:     []uint32{0xC0A80102, 0xC0A80103},
       }

       update := ub.BuildUnicast(params)

       // ORIGINATOR_ID: flags=0x80 (optional), type=0x09, len=0x04, value=C0A80101
       expectedOriginator := []byte{0x80, 0x09, 0x04, 0xC0, 0xA8, 0x01, 0x01}
       if !bytes.Contains(update.PathAttributes, expectedOriginator) {
           t.Errorf("ORIGINATOR_ID not found in PathAttributes\ngot: %x\nwant to contain: %x",
               update.PathAttributes, expectedOriginator)
       }

       // CLUSTER_LIST: flags=0x80, type=0x0A, len=0x08, values=C0A80102 C0A80103
       expectedClusterType := []byte{0x80, 0x0A, 0x08}
       if !bytes.Contains(update.PathAttributes, expectedClusterType) {
           t.Errorf("CLUSTER_LIST not found in PathAttributes\ngot: %x",
               update.PathAttributes)
       }
   }

   // TestBuildUnicast_eBGP_NoLocalPref verifies LOCAL_PREF omitted for eBGP.
   //
   // VALIDATES: LOCAL_PREF not present in eBGP UPDATE.
   // PREVENTS: RFC violation - LOCAL_PREF is iBGP only.
   func TestBuildUnicast_eBGP_NoLocalPref(t *testing.T) {
       ctx := &nlri.PackContext{ASN4: true}
       ub := NewUpdateBuilder(65001, false, ctx) // isIBGP=false

       params := UnicastParams{
           Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
           NextHop:         netip.MustParseAddr("192.168.1.1"),
           Origin:          attribute.OriginIGP,
           LocalPreference: 200, // Should be ignored for eBGP
       }

       update := ub.BuildUnicast(params)

       // LOCAL_PREF (type 5) should NOT be present for eBGP
       // Attribute header: flags (1 byte) + type 0x05
       if bytes.Contains(update.PathAttributes, []byte{0x40, 0x05}) {
           t.Error("LOCAL_PREF should not be present in eBGP UPDATE")
       }
   }
   ```

2. Run test - **MUST FAIL** (BuildUnicast doesn't encode these attrs yet)
   ```bash
   go test -run TestBuildUnicast_EncodesReflectorAttrs ./pkg/bgp/message/...
   ```
   Paste failure output.

3. **Fix:** Add encoding to `BuildUnicast` in `update_build.go`:

   After COMMUNITIES section (around line 175), add:
   ```go
   // 9. ORIGINATOR_ID (RFC 4456)
   if p.OriginatorID != 0 {
       origIP := netip.AddrFrom4([4]byte{
           byte(p.OriginatorID >> 24), byte(p.OriginatorID >> 16),
           byte(p.OriginatorID >> 8), byte(p.OriginatorID),
       })
       attrs = append(attrs, attribute.OriginatorID(origIP))
   }

   // 10. CLUSTER_LIST (RFC 4456)
   if len(p.ClusterList) > 0 {
       cl := make(attribute.ClusterList, len(p.ClusterList))
       copy(cl, p.ClusterList)
       attrs = append(attrs, cl)
   }
   ```

4. Run test - **MUST PASS**
   ```bash
   go test -run TestBuildUnicast_EncodesReflectorAttrs ./pkg/bgp/message/...
   ```

5. Run full test suite:
   ```bash
   make test && make functional
   ```

### Phase 3: Fix routeGroupKey (Prevent Silent Data Loss)

**File: `pkg/reactor/peer.go`**

1. Add test for grouping behavior:

**File: `pkg/reactor/peer_test.go`**
   ```go
   // TestRouteGroupKey_IncludesReflectorAttrs verifies grouping key includes RFC 4456 fields.
   //
   // VALIDATES: Routes with different OriginatorID get different keys.
   // PREVENTS: Silent data loss when grouping routes with different reflector attrs.
   func TestRouteGroupKey_IncludesReflectorAttrs(t *testing.T) {
       route1 := StaticRoute{
           Prefix:       netip.MustParsePrefix("10.0.0.0/24"),
           NextHop:      netip.MustParseAddr("192.168.1.1"),
           OriginatorID: 0xC0A80101,
       }
       route2 := StaticRoute{
           Prefix:       netip.MustParsePrefix("10.0.1.0/24"),
           NextHop:      netip.MustParseAddr("192.168.1.1"),
           OriginatorID: 0xC0A80102, // Different!
       }

       key1 := routeGroupKey(route1)
       key2 := routeGroupKey(route2)

       if key1 == key2 {
           t.Errorf("Routes with different OriginatorID should have different keys\nkey1: %s\nkey2: %s",
               key1, key2)
       }
   }

   // TestRouteGroupKey_IncludesClusterList verifies ClusterList affects grouping.
   func TestRouteGroupKey_IncludesClusterList(t *testing.T) {
       route1 := StaticRoute{
           Prefix:      netip.MustParsePrefix("10.0.0.0/24"),
           NextHop:     netip.MustParseAddr("192.168.1.1"),
           ClusterList: []uint32{0xC0A80101},
       }
       route2 := StaticRoute{
           Prefix:      netip.MustParsePrefix("10.0.1.0/24"),
           NextHop:     netip.MustParseAddr("192.168.1.1"),
           ClusterList: []uint32{0xC0A80101, 0xC0A80102}, // Different!
       }

       key1 := routeGroupKey(route1)
       key2 := routeGroupKey(route2)

       if key1 == key2 {
           t.Error("Routes with different ClusterList should have different keys")
       }
   }
   ```

2. Run tests - **MUST FAIL**

3. Update `routeGroupKey` to include the missing fields (around line 1847):
   ```go
   func routeGroupKey(r StaticRoute) string {
       // ... existing sorting code for comms, lcs ...

       // Key includes all attributes that affect UPDATE encoding
       prefixKey := ""
       if !r.Prefix.Addr().Is4() {
           prefixKey = r.Prefix.String()
       }

       // 16 format specifiers total
       return fmt.Sprintf("%s|%d|%d|%d|%v|%v|%s|%s|%v|%s|%v|%v|%d|%v|%d|%v",
           r.NextHop.String(),
           r.Origin,
           r.LocalPreference,
           r.MED,
           comms,
           lcs,
           hex.EncodeToString(r.ExtCommunityBytes),
           r.RD,
           r.Prefix.Addr().Is4(),
           prefixKey,
           r.ASPath,
           r.AtomicAggregate,
           r.AggregatorASN,
           r.AggregatorIP,
           r.OriginatorID,  // ADD (specifier 15)
           r.ClusterList,   // ADD (specifier 16)
       )
   }
   ```

   Note: RawAttributes could also be included, but routes with custom attributes are rare and typically unique anyway. Adding OriginatorID/ClusterList covers the route reflector use case.

4. Run tests - **MUST PASS**

5. Run `make test`

### Phase 4: Implement BuildGroupedUnicast

**File: `pkg/bgp/message/update_build.go`**

1. Add stub function:
   ```go
   // BuildGroupedUnicast builds an UPDATE with multiple IPv4 unicast NLRIs.
   //
   // RFC 4271 Section 4.3 - Multiple NLRI can share path attributes.
   // Uses first route's attributes; other routes contribute only prefixes.
   //
   // Precondition: All routes MUST be IPv4 unicast (caller validates via routeGroupKey).
   func (ub *UpdateBuilder) BuildGroupedUnicast(routes []UnicastParams) *Update {
       return nil // TDD stub
   }
   ```

2. Write tests - **MUST FAIL** (returns nil):

**File: `pkg/bgp/message/update_build_test.go`**
   ```go
   // TestBuildGroupedUnicast_MultipleNLRIs verifies grouped UPDATE encoding.
   //
   // VALIDATES: Multiple prefixes packed into single UPDATE with shared attributes.
   // PREVENTS: Regression in GroupUpdates=true performance optimization.
   func TestBuildGroupedUnicast_MultipleNLRIs(t *testing.T) {
       ctx := &nlri.PackContext{ASN4: true}
       ub := NewUpdateBuilder(65001, true, ctx)

       routes := []UnicastParams{
           {
               Prefix:          netip.MustParsePrefix("10.0.0.0/24"),
               NextHop:         netip.MustParseAddr("192.168.1.1"),
               Origin:          attribute.OriginIGP,
               LocalPreference: 100,
               Communities:     []uint32{0xFFFF0001},
           },
           {
               Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
               NextHop: netip.MustParseAddr("192.168.1.1"),
               Origin:  attribute.OriginIGP,
           },
           {
               Prefix:  netip.MustParsePrefix("10.0.2.0/24"),
               NextHop: netip.MustParseAddr("192.168.1.1"),
               Origin:  attribute.OriginIGP,
           },
       }

       update := ub.BuildGroupedUnicast(routes)

       if update == nil {
           t.Fatal("BuildGroupedUnicast returned nil")
       }

       // Verify NLRI contains all 3 prefixes (each /24 = 4 bytes: 1 len + 3 prefix)
       expectedNLRILen := 3 * 4
       if len(update.NLRI) != expectedNLRILen {
           t.Errorf("NLRI length: got %d, want %d", len(update.NLRI), expectedNLRILen)
       }

       // Verify attributes from first route are present (COMMUNITIES)
       if !bytes.Contains(update.PathAttributes, []byte{0xFF, 0xFF, 0x00, 0x01}) {
           t.Error("First route's communities not found in PathAttributes")
       }
   }

   // TestBuildGroupedUnicast_IncludesReflectorAttrs verifies RFC 4456 fields.
   //
   // VALIDATES: ORIGINATOR_ID and CLUSTER_LIST from first route are encoded.
   // PREVENTS: Data loss for route reflector attributes in grouped updates.
   func TestBuildGroupedUnicast_IncludesReflectorAttrs(t *testing.T) {
       ctx := &nlri.PackContext{ASN4: true}
       ub := NewUpdateBuilder(65001, true, ctx)

       routes := []UnicastParams{
           {
               Prefix:       netip.MustParsePrefix("10.0.0.0/24"),
               NextHop:      netip.MustParseAddr("192.168.1.1"),
               Origin:       attribute.OriginIGP,
               OriginatorID: 0xC0A80101,
               ClusterList:  []uint32{0xC0A80102, 0xC0A80103},
               RawAttributeBytes: [][]byte{{0xC0, 0x63, 0x01, 0xAB}}, // Custom attr
           },
           {
               Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
               NextHop: netip.MustParseAddr("192.168.1.1"),
               Origin:  attribute.OriginIGP,
           },
       }

       update := ub.BuildGroupedUnicast(routes)
       if update == nil {
           t.Fatal("BuildGroupedUnicast returned nil")
       }

       // Verify ORIGINATOR_ID (type 9) present
       if !bytes.Contains(update.PathAttributes, []byte{0x80, 0x09, 0x04, 0xC0, 0xA8, 0x01, 0x01}) {
           t.Error("ORIGINATOR_ID not encoded")
       }

       // Verify CLUSTER_LIST (type 10) present
       if !bytes.Contains(update.PathAttributes, []byte{0x80, 0x0A}) {
           t.Error("CLUSTER_LIST not encoded")
       }

       // Verify RawAttributes appended
       if !bytes.Contains(update.PathAttributes, []byte{0xC0, 0x63, 0x01, 0xAB}) {
           t.Error("RawAttributes not appended")
       }
   }

   // TestBuildGroupedUnicast_WithAddPath verifies ADD-PATH encoding (RFC 7911).
   //
   // VALIDATES: PathID is encoded when ADD-PATH is negotiated.
   // PREVENTS: Missing path identifiers in grouped updates.
   func TestBuildGroupedUnicast_WithAddPath(t *testing.T) {
       ctx := &nlri.PackContext{ASN4: true, AddPath: true}
       ub := NewUpdateBuilder(65001, true, ctx)

       routes := []UnicastParams{
           {
               Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
               NextHop: netip.MustParseAddr("192.168.1.1"),
               Origin:  attribute.OriginIGP,
               PathID:  1,
           },
           {
               Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
               NextHop: netip.MustParseAddr("192.168.1.1"),
               Origin:  attribute.OriginIGP,
               PathID:  2,
           },
       }

       update := ub.BuildGroupedUnicast(routes)
       if update == nil {
           t.Fatal("BuildGroupedUnicast returned nil")
       }

       // With ADD-PATH: each NLRI = 4-byte PathID + 1-byte len + 3-byte prefix = 8 bytes
       // 2 routes = 16 bytes
       expectedNLRILen := 16
       if len(update.NLRI) != expectedNLRILen {
           t.Errorf("NLRI length with ADD-PATH: got %d, want %d", len(update.NLRI), expectedNLRILen)
       }
   }

   // TestBuildGroupedUnicast_EmptySlice verifies empty input handling.
   func TestBuildGroupedUnicast_EmptySlice(t *testing.T) {
       ctx := &nlri.PackContext{ASN4: true}
       ub := NewUpdateBuilder(65001, true, ctx)

       update := ub.BuildGroupedUnicast(nil)

       if update == nil {
           t.Fatal("BuildGroupedUnicast returned nil for empty input")
       }
       if len(update.PathAttributes) != 0 || len(update.NLRI) != 0 {
           t.Error("Expected empty update for empty input")
       }
   }
   ```

3. Implement `BuildGroupedUnicast`:
   ```go
   func (ub *UpdateBuilder) BuildGroupedUnicast(routes []UnicastParams) *Update {
       if len(routes) == 0 {
           return &Update{}
       }

       // Use first route for all attributes
       first := routes[0]
       var attrs []attribute.Attribute

       // 1. ORIGIN
       attrs = append(attrs, first.Origin)

       // 2. AS_PATH
       asPath := ub.buildASPath(first.ASPath)
       attrs = append(attrs, asPath)

       // 3. NEXT_HOP (IPv4 only for grouped updates)
       if first.NextHop.Is4() {
           attrs = append(attrs, &attribute.NextHop{Addr: first.NextHop})
       }

       // 4. MED
       if first.MED > 0 {
           attrs = append(attrs, attribute.MED(first.MED))
       }

       // 5. LOCAL_PREF
       if ub.IsIBGP {
           lp := first.LocalPreference
           if lp == 0 {
               lp = 100
           }
           attrs = append(attrs, attribute.LocalPref(lp))
       }

       // 6. ATOMIC_AGGREGATE
       if first.AtomicAggregate {
           attrs = append(attrs, attribute.AtomicAggregate{})
       }

       // 7. AGGREGATOR
       if first.HasAggregator {
           attrs = append(attrs, &attribute.Aggregator{
               ASN:     first.AggregatorASN,
               Address: netip.AddrFrom4(first.AggregatorIP),
           })
       }

       // 8. COMMUNITIES
       if len(first.Communities) > 0 {
           sorted := make([]uint32, len(first.Communities))
           copy(sorted, first.Communities)
           sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
           comms := make(attribute.Communities, len(sorted))
           for i, c := range sorted {
               comms[i] = attribute.Community(c)
           }
           attrs = append(attrs, comms)
       }

       // 9. ORIGINATOR_ID (RFC 4456)
       if first.OriginatorID != 0 {
           origIP := netip.AddrFrom4([4]byte{
               byte(first.OriginatorID >> 24), byte(first.OriginatorID >> 16),
               byte(first.OriginatorID >> 8), byte(first.OriginatorID),
           })
           attrs = append(attrs, attribute.OriginatorID(origIP))
       }

       // 10. CLUSTER_LIST (RFC 4456)
       if len(first.ClusterList) > 0 {
           cl := make(attribute.ClusterList, len(first.ClusterList))
           copy(cl, first.ClusterList)
           attrs = append(attrs, cl)
       }

       // 16. EXTENDED_COMMUNITIES
       if len(first.ExtCommunityBytes) > 0 {
           attrs = append(attrs, &rawAttribute{
               flags: attribute.FlagOptional | attribute.FlagTransitive,
               code:  attribute.AttrExtCommunity,
               data:  first.ExtCommunityBytes,
           })
       }

       // 32. LARGE_COMMUNITIES
       if len(first.LargeCommunities) > 0 {
           lcs := make(attribute.LargeCommunities, len(first.LargeCommunities))
           for i, lc := range first.LargeCommunities {
               lcs[i] = attribute.LargeCommunity{
                   GlobalAdmin: lc[0],
                   LocalData1:  lc[1],
                   LocalData2:  lc[2],
               }
           }
           attrs = append(attrs, lcs)
       }

       // Sort and pack attributes
       sort.Slice(attrs, func(i, j int) bool {
           return attrs[i].Code() < attrs[j].Code()
       })
       attrBytes := attribute.PackAttributesOrdered(attrs)

       // Append raw attributes from first route (pass-through from config)
       for _, raw := range first.RawAttributeBytes {
           attrBytes = append(attrBytes, raw...)
       }

       // Build NLRI for all routes
       ctx := ub.Ctx
       if ctx == nil {
           ctx = &nlri.PackContext{}
       }
       var nlriBytes []byte
       for _, r := range routes {
           inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, r.Prefix, r.PathID)
           nlriBytes = append(nlriBytes, inet.Pack(ctx)...)
       }

       return &Update{
           PathAttributes: attrBytes,
           NLRI:           nlriBytes,
       }
   }
   ```

4. Run tests - **MUST PASS**

5. Update `sendInitialRoutes` in `peer.go`:

   **Location:** Replace the inner loop at lines 1090-1101 (inside `if p.settings.GroupUpdates {` block):

   ```go
   // Replace lines 1090-1101 with:
   for _, routes := range groups {
       if len(routes) == 1 {
           // Single-route group (IPv6, VPN, LabeledUnicast, or solo IPv4)
           ctx := p.packContext(routeFamily(routes[0]))
           update := buildStaticRouteUpdateNew(routes[0], p.settings.LocalAS, p.settings.IsIBGP(), ctx, nf)
           if err := p.SendUpdate(update); err != nil {
               trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
               break
           }
           trace.RouteSent(addr, routes[0].Prefix.String(), routes[0].NextHop.String())
       } else {
           // Multi-route group - IPv4 unicast only (routeGroupKey ensures this)
           ctx := p.packContext(routeFamily(routes[0]))
           ub := message.NewUpdateBuilder(p.settings.LocalAS, p.settings.IsIBGP(), ctx)
           params := make([]message.UnicastParams, len(routes))
           for i, r := range routes {
               params[i] = toStaticRouteUnicastParams(r, nf)
           }
           update := ub.BuildGroupedUnicast(params)
           if err := p.SendUpdate(update); err != nil {
               trace.Log(trace.Routes, "peer %s: send error: %v", addr, err)
               break
           }
           for _, route := range routes {
               trace.RouteSent(addr, route.Prefix.String(), route.NextHop.String())
           }
       }
   }
   ```

6. Run `make test && make functional`

### Phase 5: Migrate Wire Compat Tests

Now that all code changes are complete, migrate wire compat tests from "old vs new comparison" to "expected bytes" format.

**File: `pkg/reactor/wire_compat_test.go`**

1. For each test that uses `buildStaticRouteUpdate`:
   - Run test once to capture expected bytes from current output
   - Convert to expected-bytes assertion (like `TestWireCompat_VPNIPv4` already does)
   - Remove call to `buildStaticRouteUpdate`

   Example migration:
   ```go
   // BEFORE (compares old vs new - both were broken for reflector attrs!)
   oldUpdate := buildStaticRouteUpdate(route, 65001, true, ctx, nf)
   newUpdate := ub.BuildUnicast(params)
   if !bytes.Equal(oldUpdate.PathAttributes, newUpdate.PathAttributes) { ... }

   // AFTER (expected bytes from correct implementation)
   expected, _ := hex.DecodeString("40010100400200...") // captured
   newUpdate := ub.BuildUnicast(params)
   if !bytes.Equal(newUpdate.PathAttributes, expected) { ... }
   ```

2. Run `make test` - verify all tests still pass

### Phase 6: Delete Dead Code

Delete these functions from `peer.go`:
- `buildStaticRouteUpdate` (now unused after Phase 5 test migration)
- `buildGroupedUpdate` (replaced by BuildGroupedUnicast)
- `buildMPReachNLRI`
- `buildVPNNLRIBytes`
- `buildMPReachNLRIExtNHUnicast`
- `buildMPReachNLRIUnicast`

Run `make test && make lint`

### Phase 7: Rename (Optional)

If desired for consistency:
- Rename `buildStaticRouteUpdateNew` → `buildStaticRouteUpdate`

This is cosmetic - the "New" suffix is no longer misleading since the old function is gone.

## Verification Checklist

- [ ] Phase 1: Fields added to UnicastParams, toStaticRouteUnicastParams copies them
- [ ] Phase 1: Unit test for copying passes
- [ ] Phase 2: BuildUnicast test FAILS before fix (expected bytes not found)
- [ ] Phase 2: BuildUnicast test PASSES after fix
- [ ] Phase 2: eBGP LOCAL_PREF test passes
- [ ] Phase 2: `make test && make functional` pass
- [ ] Phase 3: routeGroupKey tests FAIL before fix
- [ ] Phase 3: routeGroupKey tests PASS after fix
- [ ] Phase 4: BuildGroupedUnicast tests FAIL before implementation
- [ ] Phase 4: BuildGroupedUnicast tests PASS after implementation
- [ ] Phase 4: ADD-PATH test passes
- [ ] Phase 4: sendInitialRoutes migrated (lines 1090-1101)
- [ ] Phase 4: ORIGINATOR_ID/CLUSTER_LIST/RawAttributes verified in grouped path
- [ ] Phase 5: Wire compat tests migrated to expected-bytes format
- [ ] Phase 6: Dead code deleted (~500 LOC)
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make functional` passes

## Effort Estimate

| Phase | Effort | Risk |
|-------|--------|------|
| 1. Add fields to UnicastParams | 30m | Low |
| 2. Fix BuildUnicast (expected-bytes test) | 45m | Low |
| 3. Fix routeGroupKey | 30m | Low |
| 4. BuildGroupedUnicast + sendInitialRoutes | 2.5h | Medium |
| 5. Migrate wire compat tests | 45m | Low |
| 6. Delete dead code | 30m | Low |
| 7. Rename (optional) | 15m | Low |
| **Total** | **~6h** | |

## Backward Compatibility Note

Adding ORIGINATOR_ID/CLUSTER_LIST to unicast routes changes the wire format for routes that have these attributes configured. This is a **bug fix** - peers will now receive attributes they should have been receiving all along.

Note: ORIGINATOR_ID and CLUSTER_LIST are RFC 4456 route reflector attributes, typically used in iBGP. The code encodes them for eBGP too if configured - this matches ExaBGP behavior and allows config flexibility, though some peers may reject them.

## Success Criteria

1. ORIGINATOR_ID and CLUSTER_LIST encoded for ALL unicast routes (grouped and non-grouped)
2. `routeGroupKey` prevents grouping routes with different reflector attributes
3. Wire format verified via expected-bytes tests
4. Grouping optimization preserved with BuildGroupedUnicast
5. RawAttributes supported in grouped path (was missing in old buildGroupedUpdate)
6. Old functions deleted (~500 LOC removed from peer.go)
7. All tests pass
