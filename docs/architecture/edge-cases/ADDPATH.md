# ADD-PATH Support (RFC 7911)

**Source:** ExaBGP `bgp/message/open/capability/addpath.py`, `bgp/message/update/nlri/qualifier/path.py`
**Purpose:** Document ADD-PATH capability and path identifier handling

---

## Overview

ADD-PATH allows advertising multiple paths for the same prefix, enabling:
- Path diversity (backup paths)
- Faster convergence
- Better traffic engineering

### Key Concept

Each NLRI can have a **Path Identifier** (4 bytes) prepended:

```
Standard:   [prefix]
ADD-PATH:   [path-id][prefix]
```

---

## Capability Negotiation

### ADD-PATH Capability (Code 69)

```
+---------------------------+
|   Capability Code = 69    |  1 octet
+---------------------------+
|   Capability Length       |  1 octet (N * 4)
+---------------------------+
|   AFI (2) + SAFI (1) + SR |  4 octets per family
+---------------------------+
|   ... more families ...   |
+---------------------------+
```

### Send/Receive Flags

| Value | Name | Description |
|-------|------|-------------|
| 0 | disabled | ADD-PATH not used |
| 1 | receive | Can receive ADD-PATH |
| 2 | send | Can send ADD-PATH |
| 3 | send/receive | Both directions |

### Negotiation Result

For each AFI/SAFI:
- **Local sends** if: local=send AND peer=receive
- **Local receives** if: local=receive AND peer=send

```python
def send(self, afi, safi):
    local = self.sent.get((afi, safi), 0)
    peer = self.received.get((afi, safi), 0)
    return (local & 2) and (peer & 1)  # local can send, peer can receive

def receive(self, afi, safi):
    local = self.sent.get((afi, safi), 0)
    peer = self.received.get((afi, safi), 0)
    return (local & 1) and (peer & 2)  # local can receive, peer can send
```

---

## Path Identifier

### Wire Format

```
+---------------------------+
|   Path ID (4 octets)      |  Network byte order
+---------------------------+
|   Prefix Length (1 octet) |
+---------------------------+
|   Prefix (variable)       |
+---------------------------+
```

### Special Values

| Value | Meaning |
|-------|---------|
| 0x00000000 | NOPATH - ADD-PATH enabled but no specific ID |
| (absent) | DISABLED - ADD-PATH not negotiated |

### Path ID Assignment

Path IDs are locally significant:
- Sender chooses any 32-bit value
- Must be unique per prefix per peer
- Typically: incrementing counter, hash, or peer-derived

---

## UPDATE Message Changes

### With ADD-PATH

NLRI fields include Path ID before each prefix:

```
Withdrawn Routes:
  [path-id-1][prefix-1]
  [path-id-2][prefix-2]

NLRI:
  [path-id-3][prefix-3]
  [path-id-4][prefix-4]
```

### MP_REACH_NLRI / MP_UNREACH_NLRI

Same change - Path ID prepended:

```
MP_REACH_NLRI:
  AFI (2) | SAFI (1) | Next Hop Length (1) | Next Hop | Reserved (1)
  [path-id][nlri-1]
  [path-id][nlri-2]
```

---

## ExaBGP Implementation

### PathInfo Class

```python
class PathInfo:
    LENGTH = 4
    NOPATH: ClassVar['PathInfo']     # ADD-PATH enabled, no ID
    DISABLED: ClassVar['PathInfo']   # ADD-PATH not negotiated

    def __init__(self, packed: Buffer) -> None:
        self._packed = packed
        self._disabled = False

    @classmethod
    def make_from_integer(cls, integer: int) -> 'PathInfo':
        return cls(pack('!I', integer))

    @classmethod
    def make_from_ip(cls, ip: str) -> 'PathInfo':
        # Allow "1.2.3.4" notation for path ID
        return cls(bytes([int(x) for x in ip.split('.')]))

    def __str__(self) -> str:
        if self._disabled:
            return ''
        return '.'.join(str(b) for b in self._packed)

PathInfo.NOPATH = PathInfo(bytes([0, 0, 0, 0]))
PathInfo.DISABLED = PathInfo(bytes([]))
PathInfo.DISABLED._disabled = True
```

### AddPath Capability

```python
@Capability.register()
class AddPath(Capability, dict[FamilyTuple, int]):
    ID = Capability.CODE.ADD_PATH

    string = {
        0: 'disabled',
        1: 'receive',
        2: 'send',
        3: 'send/receive',
    }

    def add_path(self, afi: AFI, safi: SAFI, send_receive: int) -> None:
        self[(afi, safi)] = send_receive

    def extract_capability_bytes(self) -> list[bytes]:
        rs = b''
        for v in self:
            if self[v]:
                rs += v[0].pack_afi() + v[1].pack_safi() + pack('!B', self[v])
        return [rs]
```

### NLRI Packing

```python
def pack_nlri(self, negotiated: Negotiated) -> Buffer:
    send_addpath = negotiated.addpath.send(self.AFI, self.SAFI)

    if send_addpath:
        if self._has_addpath:
            return self._packed  # Already has path ID
        # Prepend NOPATH (4 zero bytes)
        return bytes([0, 0, 0, 0]) + self._packed
    else:
        if self._has_addpath:
            return self._packed[4:]  # Strip path ID
        return self._packed
```

---

## Configuration

### ExaBGP Config

```
peer 192.168.1.2 {
    capability {
        add-path receive;       # Receive only
        add-path send;          # Send only
        add-path send/receive;  # Both (per family below)
    }

    add-path {
        ipv4/unicast;           # Both send and receive
        ipv4/unicast send;      # Send only
        ipv4/unicast receive;   # Receive only
    }
}
```

### API Commands

```
# Announce with path ID
announce route 10.0.0.0/8 next-hop 1.1.1.1 path-information 0.0.0.1

# Withdraw specific path
withdraw route 10.0.0.0/8 path-information 0.0.0.1
```

---

## Wire Examples

### ADD-PATH Capability

```
45                  # Capability code 69
08                  # Length 8 (2 families)
00 01 01 03         # IPv4 unicast, send/receive
00 02 01 01         # IPv6 unicast, receive only
```

### UPDATE with ADD-PATH

```
IPv4 unicast with path ID:

Withdrawn Routes Length: 0x0000
Path Attributes Length: 0x0018
  ... attributes ...
NLRI:
  00 00 00 01       # Path ID = 1
  18                # /24
  0A 00 01          # 10.0.1.0
  00 00 00 02       # Path ID = 2
  18                # /24
  0A 00 02          # 10.0.2.0
```

### MP_REACH with ADD-PATH

```
90                  # Flags: Optional, Transitive
0E                  # MP_REACH_NLRI (14)
00 12               # Length 18
00 01               # AFI = IPv4
01                  # SAFI = unicast
04                  # Next Hop Length
C0 A8 01 01         # Next Hop = 192.168.1.1
00                  # Reserved
00 00 00 01         # Path ID = 1
18                  # /24
0A 00 01            # 10.0.1.0
```

---

## ZeBGP Implementation Notes

### PathInfo Type

```go
type PathInfo struct {
    id       uint32
    disabled bool
}

var (
    NoPath       = PathInfo{id: 0, disabled: false}
    DisabledPath = PathInfo{id: 0, disabled: true}
)

func NewPathInfo(id uint32) PathInfo {
    return PathInfo{id: id, disabled: false}
}

func (p PathInfo) Pack() []byte {
    if p.disabled {
        return nil
    }
    buf := make([]byte, 4)
    binary.BigEndian.PutUint32(buf, p.id)
    return buf
}

func (p PathInfo) String() string {
    if p.disabled {
        return ""
    }
    // Format as IP-like for readability
    return fmt.Sprintf("%d.%d.%d.%d",
        (p.id>>24)&0xFF, (p.id>>16)&0xFF,
        (p.id>>8)&0xFF, p.id&0xFF)
}
```

### AddPath Capability

```go
type AddPath map[Family]uint8

const (
    AddPathDisabled = 0
    AddPathReceive  = 1
    AddPathSend     = 2
    AddPathBoth     = 3
)

func (a AddPath) CanSend(f Family, peer AddPath) bool {
    local := a[f]
    remote := peer[f]
    return (local&AddPathSend != 0) && (remote&AddPathReceive != 0)
}

func (a AddPath) CanReceive(f Family, peer AddPath) bool {
    local := a[f]
    remote := peer[f]
    return (local&AddPathReceive != 0) && (remote&AddPathSend != 0)
}

func (a AddPath) Pack() []byte {
    var buf bytes.Buffer
    for family, sr := range a {
        if sr != 0 {
            buf.Write(family.AFI.Pack())
            buf.Write(family.SAFI.Pack())
            buf.WriteByte(sr)
        }
    }
    return buf.Bytes()
}
```

### NLRI Encoding (Phase 3+ Simplification)

After ADD-PATH simplification, NLRI types store **payload only** (no path ID in Len/WriteTo).
Path ID handling is centralized in `WriteNLRI()`:

```go
// NLRI interface - payload only
type NLRI interface {
    Len() int                                    // Payload length (no path ID)
    WriteTo(buf []byte, off int, ctx) int        // Write payload only
    PathID() uint32                              // Stored path ID (0 if unset)
}

// Encoding with ADD-PATH handling
func encodeNLRI(n nlri.NLRI, ctx *nlri.PackContext) []byte {
    size := nlri.LenWithContext(n, ctx)  // +4 when ctx.AddPath=true
    buf := make([]byte, size)
    nlri.WriteNLRI(n, buf, 0, ctx)       // Prepends path ID when AddPath=true
    return buf
}
```

**WriteNLRI behavior (RFC 7911 Section 3):**
- `ctx.AddPath=true`: writes `[4-byte pathID][payload]`
- `ctx.AddPath=false` or `ctx=nil`: writes `[payload]` only

**Path ID value:**
- Uses `n.PathID()` (stored value, 0 if unset)
- Value 0 is valid per RFC 7911 (NOPATH)

---

**Last Updated:** 2026-01-04
