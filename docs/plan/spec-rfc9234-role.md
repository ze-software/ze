# Spec: RFC 9234 BGP Role for API Policy

## Status: Ready for Implementation

## RFC Reference

`rfc/rfc9234.txt` - Route Leak Prevention and Detection Using Roles in UPDATE and OPEN Messages

## Purpose

RFC 9234 Role enables **API-driven routing policy** without attribute parsing:

```
Customer peer → Receive route → Tag: role=customer → API decides
                                                        ↓
                            "Routes from customers can go to providers"
                                                        ↓
                            Forward to provider peers only
```

**Key insight:** Role tag tells the API the relationship, enabling policy decisions without parsing AS_PATH, communities, etc.

## Use Cases

### Route Server (RS)

```
RS-Client A → Receive → role=rs-client → Forward to other RS-Clients
```

### Transit/Customer Hierarchy

```
Customer → Receive → role=customer → Can send to: Provider, Peer, RS
Provider → Receive → role=provider → Can send to: Customers only
Peer     → Receive → role=peer     → Can send to: Customers only
```

### Policy Examples

```python
# External policy process
if route.tag.role == "customer":
    # Customer routes can go anywhere
    forward_to("peer *")
elif route.tag.role == "provider":
    # Provider routes only to customers
    forward_to("peer [role customer]")
elif route.tag.role == "peer":
    # Peer routes only to customers
    forward_to("peer [role customer]")
```

## RFC 9234 Overview

### Role Capability (Code 9)

Negotiated in OPEN message:
```
Capability Code: 9
Capability Length: 1
Capability Value: Role (0-4)
```

### Role Values

| Value | Role | Description |
|-------|------|-------------|
| 0 | Provider | Upstream transit provider |
| 1 | RS | Route Server |
| 2 | RS-Client | Route Server Client |
| 3 | Customer | Downstream customer |
| 4 | Peer | Settlement-free peer |

### Allowed Relationships

| Local Role | Peer Role | Valid? |
|------------|-----------|--------|
| Provider | Customer | ✅ |
| Customer | Provider | ✅ |
| RS | RS-Client | ✅ |
| RS-Client | RS | ✅ |
| Peer | Peer | ✅ |
| Provider | Provider | ❌ |
| Customer | Customer | ❌ |

### OTC Attribute (Type 35)

Only To Customer - marks routes that should not leak:
- Added by Provider/RS/Peer when sending to Customer
- If present, route cannot be sent to Provider/Peer

## Implementation

### Capability Negotiation

```go
// internal/bgp/capability/role.go

type Role uint8

const (
    RoleProvider  Role = 0
    RoleRS        Role = 1
    RoleRSClient  Role = 2
    RoleCustomer  Role = 3
    RolePeer      Role = 4
)

type RoleCapability struct {
    Role Role
}

func (c *RoleCapability) Code() CapabilityCode { return CapRole }
func (c *RoleCapability) Pack() []byte { return []byte{byte(c.Role)} }

func ParseRoleCapability(data []byte) (*RoleCapability, error) {
    if len(data) != 1 {
        return nil, ErrInvalidLength
    }
    role := Role(data[0])
    if role > RolePeer {
        return nil, ErrInvalidRole
    }
    return &RoleCapability{Role: role}, nil
}
```

### Peer Role Storage

```go
// internal/reactor/peer.go

type Peer struct {
    // ... existing fields ...
    localRole  capability.Role  // Our role in this relationship
    peerRole   capability.Role  // Their role (negotiated)
    roleNegotiated bool         // True if both sides sent Role capability
}

func (p *Peer) onOpenReceived(open *message.Open) {
    // Extract Role capability if present
    if roleCap := open.GetCapability(capability.CapRole); roleCap != nil {
        p.peerRole = roleCap.(*capability.RoleCapability).Role
        p.roleNegotiated = true
    }
}
```

### Route Tag

```go
// internal/rib/route.go

type RouteTag struct {
    SourceRole   capability.Role  // RFC 9234 role of source peer
    SourcePeerIP netip.Addr       // For !<ip> selector
}

type Route struct {
    // ... existing fields ...
    tag RouteTag
}
```

### OTC Attribute Parsing

```go
// internal/bgp/attribute/otc.go

// OTC represents the Only To Customer attribute (RFC 9234).
// Type 35, optional transitive.
type OTC uint32  // AS number that added OTC

const AttrOTC AttributeCode = 35

func (o OTC) Code() AttributeCode   { return AttrOTC }
func (o OTC) Flags() AttributeFlags { return FlagOptional | FlagTransitive }
func (o OTC) Len() int              { return 4 }
func (o OTC) Pack() []byte {
    buf := make([]byte, 4)
    binary.BigEndian.PutUint32(buf, uint32(o))
    return buf
}

func ParseOTC(data []byte) (OTC, error) {
    if len(data) != 4 {
        return 0, ErrInvalidLength
    }
    return OTC(binary.BigEndian.Uint32(data)), nil
}
```

### API Output

```json
{
    "type": "update",
    "route-id": 12345,
    "tag": {
        "role": "customer",
        "source": "10.0.0.1"
    },
    "peer": { "address": "10.0.0.1" },
    "announce": {
        "nlri": { "ipv4/unicast": ["192.168.1.0/24"] }
    }
}
```

### Peer Selector by Role

```
peer [role customer]    # All customers
peer [role provider]    # All providers
peer [role peer]        # All peers
peer [role rs-client]   # All RS-clients
```

Implementation:
```go
func (r *Reactor) GetMatchingPeers(selector *Selector) []*Peer {
    if selector.RoleFilter != nil {
        var result []*Peer
        for _, p := range r.peers {
            if p.peerRole == *selector.RoleFilter {
                result = append(result, p)
            }
        }
        return result
    }
    // ... existing matching
}
```

## Config

### Peer Role Configuration

```
peer 10.0.0.1 {
    local-as 65001;
    peer-as 65002;
    role customer;  # Our role: we are their customer
}
```

This means:
- We send Role=Customer in OPEN
- We expect them to send Role=Provider
- Routes from them tagged as "from provider"

## Test Plan

```go
// TestRoleCapabilityParsing verifies capability parsing.
// VALIDATES: All 5 role values parsed correctly.
// PREVENTS: Role misidentification.
func TestRoleCapabilityParsing(t *testing.T)

// TestRoleNegotiation verifies OPEN exchange.
// VALIDATES: Roles stored after negotiation.
// PREVENTS: Lost role information.
func TestRoleNegotiation(t *testing.T)

// TestRouteTagging verifies tag on receive.
// VALIDATES: Routes tagged with source role.
// PREVENTS: Missing role tags.
func TestRouteTagging(t *testing.T)

// TestOTCParsing verifies OTC attribute parsing.
// VALIDATES: OTC value extracted correctly.
// PREVENTS: OTC handling errors.
func TestOTCParsing(t *testing.T)

// TestRoleSelector verifies peer [role X] selector.
// VALIDATES: Correct peers matched by role.
// PREVENTS: Wrong peer selection.
func TestRoleSelector(t *testing.T)

// TestAPIOutputRole verifies role in API output.
// VALIDATES: Role tag in JSON output.
// PREVENTS: Missing role for policy decisions.
func TestAPIOutputRole(t *testing.T)
```

## Checklist

- [ ] Role capability type (code 9)
- [ ] Role capability parsing
- [ ] Role capability packing
- [ ] Peer.localRole, peerRole fields
- [ ] Role negotiation in OPEN handling
- [ ] RouteTag with SourceRole
- [ ] OTC attribute parsing (type 35)
- [ ] API output includes role tag
- [ ] `peer [role X]` selector
- [ ] Config `role` keyword in peer block
- [ ] Tests pass
- [ ] `make test && make lint` pass
- [ ] Functional test

## Dependencies

None (independent of other specs)

## Notes

- RFC 9234 is optional - works without negotiation (no role tag)
- OTC attribute is for route-leak prevention, but we expose it for API policy
- Role tag enables zero-parse policy decisions

---

**Created:** 2026-01-01
