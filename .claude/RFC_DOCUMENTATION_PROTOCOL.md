# RFC Documentation Protocol

**When to read:** Before modifying ANY protocol code (messages, attributes, NLRI, capabilities)
**Prerequisites:** ESSENTIAL_PROTOCOLS.md
**Size:** ~5 KB

---

## Core Principle

**Never modify BGP protocol code without first documenting the wire format from the RFC.**

This applies to:
- NLRI types (wire format, field sizes, encoding)
- Attributes (type codes, flags, encoding)
- Capabilities (negotiation, parameters)
- Messages (OPEN, UPDATE, NOTIFICATION, KEEPALIVE)

---

## Step 0: Check RFC Documentation

Before touching any protocol code:

```bash
# Check if wire format is documented
head -50 pkg/bgp/nlri/<module>.go
grep -n "RFC\|wire format\|Wire format" pkg/bgp/nlri/<module>.go
```

**If NOT documented:**
1. Look up the RFC (see table below)
2. Find the packet format section
3. Document the wire format in the code BEFORE making changes

---

## Common RFCs Reference

| Feature | RFC | ZeBGP Location |
|---------|-----|----------------|
| BGP-4 base | RFC 4271 | `pkg/bgp/message/*.go`, `pkg/bgp/fsm/` |
| MP-BGP (AFI/SAFI) | RFC 4760 | `pkg/bgp/nlri/*.go`, `pkg/bgp/attribute/mpreach.go` |
| VPLS | RFC 4761, RFC 4762 | `pkg/bgp/nlri/vpls.go` |
| EVPN | RFC 7432 | `pkg/bgp/nlri/evpn/` |
| FlowSpec | RFC 5575, RFC 8955 | `pkg/bgp/nlri/flowspec.go` |
| BGP-LS | RFC 7752 | `pkg/bgp/nlri/bgpls/` |
| Add-Path | RFC 7911 | `pkg/bgp/capability/addpath.go` |
| Large Communities | RFC 8092 | `pkg/bgp/attribute/community/large.go` |
| MVPN | RFC 6514 | `pkg/bgp/nlri/mvpn/` |
| RTC | RFC 4684 | `pkg/bgp/nlri/rtc.go` |
| MUP | draft-ietf-dmm-srv6-mobile-uplane | `pkg/bgp/nlri/mup/` |
| Route Refresh | RFC 2918 | `pkg/bgp/message/refresh.go` |
| Graceful Restart | RFC 4724 | `pkg/bgp/capability/graceful.go` |
| 4-byte ASN | RFC 6793 | `pkg/bgp/capability/asn4.go` |
| Extended Communities | RFC 4360 | `pkg/bgp/attribute/community/extended/` |
| PMSI Tunnel | RFC 6514 | `pkg/bgp/attribute/pmsi.go` |
| AIGP | RFC 7311 | `pkg/bgp/attribute/aigp.go` |
| SRv6 | RFC 9252 | `pkg/bgp/nlri/bgpls/srv6.go` |
| Segment Routing | RFC 8669 | `pkg/bgp/attribute/sr/` |
| IP VPN | RFC 4364 | `pkg/bgp/nlri/ipvpn.go` |

---

## Wire Format Documentation Template

**Go Example - VPLS NLRI:**

```go
// VPLS represents a VPLS NLRI (RFC 4761 Section 3.2.2)
//
// Wire format (19 bytes total):
//
//     0                   1                   2                   3
//     0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |           Length (2)          |    Route Distinguisher (8)    |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |                    ... RD continued ...                       |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |          VE ID (2)            |      Label Block Offset (2)   |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |      Label Block Size (2)     |       Label Base (3)          |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Byte offsets (including 2-byte length prefix):
//   [0:2]   - Length (always 17 for VPLS)
//   [2:10]  - Route Distinguisher
//   [10:12] - VE ID (endpoint)
//   [12:14] - Label Block Offset
//   [14:16] - Label Block Size
//   [16:19] - Label Base (20 bits) + flags (4 bits)
type VPLS struct {
    RD              RouteDistinguisher
    VEID            uint16
    LabelBlockOff   uint16
    LabelBlockSize  uint16
    LabelBase       uint32 // 20 bits
}
```

**Go Example - EVPN Type 2:**

```go
// EVPNMACIPRoute represents EVPN MAC/IP Advertisement (RFC 7432 Section 7.2)
//
// Wire format (variable length, 33+ bytes):
//
//     0                   1                   2                   3
//     0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    | Route Type(1) |    Length(1)  |  Route Distinguisher (8)      |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |          Ethernet Segment Identifier (10 bytes)               |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |                    Ethernet Tag ID (4)                        |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    | MAC Addr Len  |           MAC Address (6 bytes)               |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    | IP Addr Len   |           IP Address (0, 4, or 16 bytes)      |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |           MPLS Label 1 (3)    |    MPLS Label 2 (0 or 3)      |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Byte offsets:
//   [0]     - Route Type (always 2)
//   [1]     - Length of following fields
//   [2:10]  - Route Distinguisher
//   [10:20] - Ethernet Segment Identifier
//   [20:24] - Ethernet Tag ID
//   [24]    - MAC Address Length (always 48 = 6 bytes)
//   [25:31] - MAC Address
//   [31]    - IP Address Length (0, 32, or 128 bits)
//   [32:N]  - IP Address (if present)
//   [N:N+3] - MPLS Label 1
//   [N+3:]  - MPLS Label 2 (optional)
type EVPNMACIPRoute struct {
    RD           RouteDistinguisher
    ESI          [10]byte
    EthernetTag  uint32
    MACAddr      [6]byte
    IPAddr       netip.Addr // Optional
    Label1       uint32
    Label2       uint32     // Optional
}
```

---

## The Protocol: RFC-First Development

### Step 1: Identify the RFC

Before modifying any protocol code:
- What RFC defines this feature?
- What section describes the wire format?

### Step 2: Document Wire Format

Add ASCII art packet diagram to the Go type:
- Show all fields with bit positions
- List byte offsets
- Note variable-length fields

### Step 3: Write Tests

```go
func TestVPLSPackUnpack(t *testing.T) {
    // Known wire bytes from RFC example or captured traffic
    wire := []byte{0x00, 0x11, ...}

    // Unpack
    nlri, err := UnpackVPLS(wire)
    require.NoError(t, err)

    // Verify fields
    assert.Equal(t, uint16(0x1234), nlri.VEID)

    // Pack and compare
    packed := nlri.Pack()
    assert.Equal(t, wire, packed)
}
```

### Step 4: Implement

NOW you can implement, with:
- Wire format documented
- Tests ready
- RFC as reference

---

## Pre-Modification Checklist

Copy this for each protocol change:

```markdown
## Pre-Change Checklist: [Feature/File Name]

### RFC/Specification
- [ ] RFC identified: RFC ____
- [ ] Section: ____
- [ ] Wire format documented in code: Yes/No
- [ ] If No: Added wire format diagram
- [ ] Byte offsets verified against RFC

### Tests
- [ ] Test file: pkg/bgp/nlri/<name>_test.go
- [ ] TestPack exists
- [ ] TestUnpack exists
- [ ] TestRoundtrip exists
- [ ] Known wire bytes from RFC/capture

### Ready to Modify
- [ ] RFC documented in code
- [ ] Tests written
- [ ] Tests pass with current code
```

---

## Red Flags - STOP If You See These

1. **No RFC reference** → Find and read the RFC
2. **No wire format diagram** → Add one before changing
3. **No pack/unpack tests** → Write them first
4. **"I'll document later"** → No. RFC first.
5. **Byte offsets unclear** → Draw the packet diagram

---

## Quick Reference

```bash
# Search for RFC references
grep -rn "RFC" pkg/bgp/

# Find undocumented wire formats
for f in pkg/bgp/nlri/*.go; do
    if ! grep -q "Wire format\|wire format" "$f"; then
        echo "Missing wire format: $f"
    fi
done
```

---

## See Also

- zebgp/wire/*.md - Wire format documentation
- zebgp/EXABGP_CODE_MAP.md - ExaBGP reference for compatibility
- TESTING_PROTOCOL.md - Test requirements

---

**Updated:** 2025-12-19
