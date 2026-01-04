# Configuration Routes and End-of-RIB

**Status:** ⏭️ Superseded
**Created:** 2025-12-21
**Superseded by:** `unified-commit-system.md`

---

## Problem Statement

### Current Behavior

When ZeBGP establishes a BGP session:
1. Static routes from config are sent individually or grouped (if `group-updates` enabled)
2. No End-of-RIB (EOR) marker is sent after initial routes
3. No "implicit commit" - routes sent as they're processed

### Issues

1. **No EOR** - Peer doesn't know when initial sync is complete
2. **Non-deterministic ordering** - Routes may arrive in different order
3. **No batching guarantee** - Config routes may not be optimally grouped

### RFC Requirements

**RFC 4724 (Graceful Restart):**
> The End-of-RIB marker is a BGP UPDATE message with no reachable NLRI and empty withdrawn NLRI.
> For IPv4 unicast: UPDATE with Withdrawn Routes Length = 0 and Total Path Attribute Length = 0

**RFC 7313 (Enhanced Route Refresh):**
> Defines Beginning-of-RIB (BoRR) and End-of-RIB (EoRR) markers for route refresh.

---

## Proposed Solution

### Implicit Commit for Config Routes

All static routes defined in the configuration file should be treated as a single implicit transaction:

```
Session ESTABLISHED
       │
       ▼
┌──────────────────────────────────┐
│  IMPLICIT COMMIT START           │
│  (all config routes)             │
└──────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────┐
│  Group routes by attributes      │
│  Build minimal UPDATE messages   │
│  Send UPDATEs                    │
└──────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────┐
│  IMPLICIT COMMIT END             │
│  Send EOR for each family        │
└──────────────────────────────────┘
       │
       ▼
  Session ready for API routes
```

### EOR Behavior

After all configuration routes are sent, ZeBGP sends EOR for each negotiated address family:

| Family | EOR Message |
|--------|-------------|
| IPv4 Unicast | UPDATE with withdrawn=0, path_attrs=0, nlri=0 |
| IPv6 Unicast | UPDATE with MP_UNREACH_NLRI (AFI=2, SAFI=1, no withdraws) |
| IPv4 Flow | UPDATE with MP_UNREACH_NLRI (AFI=1, SAFI=133, no withdraws) |
| etc. | MP_UNREACH_NLRI for each AFI/SAFI |

---

## Configuration

### New Options

```
neighbor 192.168.1.1 {
    # Send EOR after config routes (default: true)
    send-eor true;

    # Delay before sending EOR (allows API to add routes first)
    # 0 = send immediately after config routes
    eor-delay 0;
}
```

### Global RIB Option

```
rib {
    # Implicit commit for config routes (default: true)
    # When true: all config routes grouped and sent as batch
    # When false: routes sent individually as processed
    config-commit true;
}
```

---

## Implementation

### Phase 1: EOR Support

**1.1 EOR Message Generation**

```go
// pkg/bgp/message/eor.go

// BuildEOR creates an End-of-RIB marker for the given family
func BuildEOR(fam family.Family) *Update {
    if fam == family.IPv4Unicast {
        // RFC 4724: Empty UPDATE for IPv4 unicast
        return &Update{
            WithdrawnRoutesLen:   0,
            TotalPathAttrLen:     0,
            WithdrawnRoutes:      nil,
            PathAttributes:       nil,
            NLRI:                 nil,
        }
    }

    // RFC 4724: MP_UNREACH_NLRI with no withdrawn NLRIs
    mpUnreach := &attribute.MPUnreachNLRI{
        AFI:  fam.AFI(),
        SAFI: fam.SAFI(),
        // No withdrawn NLRIs = EOR
    }

    return &Update{
        WithdrawnRoutesLen: 0,
        TotalPathAttrLen:   uint16(mpUnreach.Len()),
        PathAttributes:     []attribute.Attribute{mpUnreach},
    }
}
```

**1.2 EOR Detection (Receiving)**

```go
// pkg/bgp/message/update.go

// IsEOR returns true if this UPDATE is an End-of-RIB marker
func (u *Update) IsEOR() (bool, family.Family) {
    // IPv4 Unicast EOR: completely empty UPDATE
    if u.WithdrawnRoutesLen == 0 && u.TotalPathAttrLen == 0 && len(u.NLRI) == 0 {
        return true, family.IPv4Unicast
    }

    // Other families: MP_UNREACH_NLRI with no withdraws
    for _, attr := range u.PathAttributes {
        if mpUnreach, ok := attr.(*attribute.MPUnreachNLRI); ok {
            if len(mpUnreach.WithdrawnRoutes) == 0 {
                fam := family.New(mpUnreach.AFI, mpUnreach.SAFI)
                return true, fam
            }
        }
    }

    return false, family.Family{}
}
```

### Phase 2: Config Route Commit

**2.1 Collect Config Routes**

```go
// pkg/reactor/peer.go

func (p *Peer) sendInitialRoutes() error {
    // Collect all static routes from config
    routes := p.neighbor.StaticRoutes

    if len(routes) == 0 {
        // No config routes, just send EOR
        return p.sendEORForNegotiatedFamilies()
    }

    // Group by attributes (implicit commit)
    groups := rib.GroupByAttributes(routes)

    // Send grouped UPDATEs
    for _, group := range groups {
        update := buildGroupedUpdate(group, p.negotiated)
        if err := p.SendUpdate(update); err != nil {
            return err
        }
    }

    // Send EOR for each family that had routes
    return p.sendEORForNegotiatedFamilies()
}
```

**2.2 Track Families for EOR**

```go
// pkg/reactor/peer.go

func (p *Peer) sendEORForNegotiatedFamilies() error {
    if !p.neighbor.SendEOR {
        return nil
    }

    // Wait for eor-delay if configured
    if p.neighbor.EORDelay > 0 {
        time.Sleep(p.neighbor.EORDelay)
    }

    // Send EOR for each negotiated family
    for _, fam := range p.negotiated.Families {
        eor := message.BuildEOR(fam)
        if err := p.SendUpdate(eor); err != nil {
            return fmt.Errorf("send EOR for %s: %w", fam, err)
        }
        p.log.Debug("sent EOR", "family", fam)
    }

    return nil
}
```

### Phase 3: Integration with API Commits

When API routes are sent via `commit end`, they should NOT trigger EOR (EOR is only for initial sync):

```go
// pkg/rib/outgoing.go

type CommitOptions struct {
    Label       string
    SendEOR     bool  // Only true for config route commit
}

func (r *OutgoingRIB) CommitTransaction(opts CommitOptions) (CommitStats, error) {
    // ... group and send routes ...

    if opts.SendEOR {
        // Send EOR after this batch
        for _, fam := range r.affectedFamilies {
            r.peer.SendEOR(fam)
        }
    }

    return stats, nil
}
```

---

## Session Establishment Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           SESSION ESTABLISHMENT                              │
└─────────────────────────────────────────────────────────────────────────────┘

  ZeBGP                                                              Peer
    │                                                                  │
    │◄─────────────────── TCP Connection ─────────────────────────────►│
    │                                                                  │
    │──────────────────────── OPEN ───────────────────────────────────►│
    │◄─────────────────────── OPEN ────────────────────────────────────│
    │                                                                  │
    │◄──────────────────── KEEPALIVE ──────────────────────────────────│
    │───────────────────── KEEPALIVE ─────────────────────────────────►│
    │                                                                  │
    │                    [SESSION ESTABLISHED]                         │
    │                                                                  │
    │  ┌─────────────────────────────────────────────────────────────┐ │
    │  │ IMPLICIT CONFIG COMMIT                                      │ │
    │  │                                                             │ │
    │  │ 1. Collect static routes from config                        │ │
    │  │ 2. Group by attributes                                      │ │
    │  │ 3. Build minimal UPDATEs                                    │ │
    │  └─────────────────────────────────────────────────────────────┘ │
    │                                                                  │
    │──── UPDATE (routes 1-5, same attrs) ────────────────────────────►│
    │──── UPDATE (routes 6-8, same attrs) ────────────────────────────►│
    │──── UPDATE (routes 9-10, same attrs) ───────────────────────────►│
    │                                                                  │
    │  ┌─────────────────────────────────────────────────────────────┐ │
    │  │ END-OF-RIB MARKERS                                          │ │
    │  │                                                             │ │
    │  │ For each negotiated family:                                 │ │
    │  │   - IPv4 Unicast: empty UPDATE                              │ │
    │  │   - IPv6 Unicast: MP_UNREACH(AFI=2,SAFI=1)                 │ │
    │  │   - etc.                                                    │ │
    │  └─────────────────────────────────────────────────────────────┘ │
    │                                                                  │
    │──── UPDATE (EOR IPv4 Unicast) ──────────────────────────────────►│
    │──── UPDATE (EOR IPv6 Unicast) ──────────────────────────────────►│
    │                                                                  │
    │                    [INITIAL SYNC COMPLETE]                       │
    │                                                                  │
    │  API routes now accepted...                                      │
    │                                                                  │
```

---

## Test Cases

### Unit Tests

```go
// pkg/bgp/message/eor_test.go

func TestBuildEOR_IPv4Unicast(t *testing.T) {
    eor := message.BuildEOR(family.IPv4Unicast)

    assert.Equal(t, uint16(0), eor.WithdrawnRoutesLen)
    assert.Equal(t, uint16(0), eor.TotalPathAttrLen)
    assert.Empty(t, eor.NLRI)

    // Verify wire format: marker + len(23) + type(2) + 0000 + 0000
    expected := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000"
    assert.Equal(t, expected, hex.EncodeToString(eor.Bytes()))
}

func TestBuildEOR_IPv6Unicast(t *testing.T) {
    eor := message.BuildEOR(family.IPv6Unicast)

    assert.Len(t, eor.PathAttributes, 1)
    mpUnreach := eor.PathAttributes[0].(*attribute.MPUnreachNLRI)
    assert.Equal(t, uint16(2), mpUnreach.AFI)
    assert.Equal(t, uint8(1), mpUnreach.SAFI)
    assert.Empty(t, mpUnreach.WithdrawnRoutes)
}

func TestIsEOR(t *testing.T) {
    tests := []struct {
        name     string
        update   *message.Update
        isEOR    bool
        family   family.Family
    }{
        {
            name:   "IPv4 Unicast EOR",
            update: message.BuildEOR(family.IPv4Unicast),
            isEOR:  true,
            family: family.IPv4Unicast,
        },
        {
            name:   "IPv6 Unicast EOR",
            update: message.BuildEOR(family.IPv6Unicast),
            isEOR:  true,
            family: family.IPv6Unicast,
        },
        {
            name:   "Regular UPDATE",
            update: &message.Update{NLRI: []nlri.NLRI{...}},
            isEOR:  false,
        },
    }
    // ...
}
```

### Integration Tests

```go
// pkg/reactor/peer_test.go

func TestPeer_SendsEORAfterConfigRoutes(t *testing.T) {
    // Setup peer with 5 static routes
    // Verify: 5 routes sent in grouped UPDATEs
    // Verify: EOR sent for each negotiated family
}

func TestPeer_EORDelayRespected(t *testing.T) {
    // Setup peer with eor-delay: 100ms
    // Verify: EOR sent after delay
}

func TestPeer_NoEORWhenDisabled(t *testing.T) {
    // Setup peer with send-eor: false
    // Verify: No EOR sent
}
```

### Self-Check Tests

Add `.ci` test cases for EOR:

```
# test/data/encode/eor-ipv4.ci
option:file:eor-ipv4.conf
1:cmd:announce eor ipv4/unicast
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0017:02:00000000

# test/data/encode/eor-ipv6.ci
option:file:eor-ipv6.conf
1:cmd:announce eor ipv6/unicast
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:001E:02:00000007900F0003000201
```

---

## .ci File Updates

Existing tests need EOR expectations added:

```
# test/data/encode/attributes.ci (updated)
option:file:attributes.conf
1:cmd:announce route 10.0.0.7/32 next-hop 255.255.255.255 ...
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:004C:02:...
# NEW: EOR after config routes
1:cmd:announce eor ipv4/unicast
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0017:02:00000000
1:cmd:announce eor ipv6/unicast
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:001E:02:00000007900F0003000201
```

---

## Relationship to API Commit Batching

| Feature | Config Routes | API Routes |
|---------|---------------|------------|
| Trigger | Session ESTABLISHED | `commit end` command |
| Grouping | Automatic (implicit commit) | Automatic (explicit commit) |
| EOR | Yes, after all config routes | Yes, for families in commit |
| Timing | Immediate | On commit |

**Both** config routes and API commits send EOR for the families involved.

### Commit EOR Behavior

When `commit end` is called:
1. Group routes by attributes
2. Send UPDATE messages
3. **Send EOR for each family that had routes in this commit**

```
commit start batch1
announce route 10.0.0.0/24 next-hop 1.2.3.4        # IPv4 Unicast
announce route 2001:db8::/32 next-hop 2001:db8::1  # IPv6 Unicast
commit end batch1
```

Results in:
```
UPDATE (IPv4 route)
UPDATE (IPv6 route)
UPDATE (EOR IPv4 Unicast)   ← EOR for families in commit
UPDATE (EOR IPv6 Unicast)   ← EOR for families in commit
```

### Response Includes EOR Info

```json
{
  "status": "ok",
  "updates_sent": 2,
  "routes_announced": 2,
  "routes_withdrawn": 0,
  "eor_sent": ["ipv4/unicast", "ipv6/unicast"],
  "transaction": "batch1"
}
```

---

## Files to Create/Modify

### New Files
- `pkg/bgp/message/eor.go` - EOR message building
- `pkg/bgp/message/eor_test.go` - EOR tests

### Modified Files
- `pkg/bgp/message/update.go` - Add `IsEOR()` method
- `pkg/reactor/peer.go` - Add `sendInitialRoutes()`, `sendEORForNegotiatedFamilies()`
- `pkg/reactor/neighbor.go` - Add `SendEOR`, `EORDelay` fields
- `pkg/config/bgp.go` - Add `send-eor`, `eor-delay` to schema
- `pkg/config/loader.go` - Load EOR config
- `test/data/encode/*.ci` - Add EOR expectations

---

## Success Criteria

1. EOR sent for each negotiated family after config routes
2. Config routes grouped (implicit commit) before EOR
3. `send-eor` config option works
4. `eor-delay` config option works
5. All existing tests updated with EOR expectations
6. New EOR-specific tests pass

---

## References

- RFC 4724: Graceful Restart Mechanism for BGP
- RFC 7313: Enhanced Route Refresh Capability for BGP-4
- ExaBGP EOR: `../src/exabgp/bgp/message/update/eor.py`
- Related plan: `plan/api-commit-batching.md`
