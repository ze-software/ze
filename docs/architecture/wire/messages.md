# BGP Message Wire Format

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Header** | 19 bytes: 16-byte marker (0xFF), 2-byte length, 1-byte type |
| **Types** | 1=OPEN, 2=UPDATE, 3=NOTIFICATION, 4=KEEPALIVE, 5=ROUTE-REFRESH |
| **Max Size** | 4096 bytes standard, 65535 with Extended Message (RFC 8654) |
| **Key Types** | `Message` interface, `MessageType`, `ParseHeader()` |
| **Pattern** | All messages share header; type-specific body follows |

**When to read full doc:** Message parsing, wire format debugging, new message types.

---

**Source:** RFC 4271, ExaBGP `bgp/message/`
**Purpose:** Document wire format for all BGP message types

---

## Message Header (RFC 4271 Section 4.1)

All BGP messages share a common 19-byte header:

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
+                                                               +
|                                                               |
+                            Marker                             +
|                                                               |
+                                                               +
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Length               |      Type     |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Value |
|-------|-------|-------|
| Marker | 16 | All 0xFF (16 bytes of 255) |
| Length | 2 | Total message length (19-4096, or 19-65535 with extended) |
| Type | 1 | Message type code |

### Message Types

| Code | Name | RFC |
|------|------|-----|
| 1 | OPEN | RFC 4271 |
| 2 | UPDATE | RFC 4271 |
| 3 | NOTIFICATION | RFC 4271 |
| 4 | KEEPALIVE | RFC 4271 |
| 5 | ROUTE-REFRESH | RFC 2918 |

### Message Length Constraints

| Message Type | Minimum | Maximum (standard) | Maximum (extended) |
|--------------|---------|--------------------|--------------------|
| OPEN | 29 | 4096 | 65535 |
| UPDATE | 23 | 4096 | 65535 |
| NOTIFICATION | 21 | 4096 | 65535 |
| KEEPALIVE | 19 | 19 | 19 |
| ROUTE-REFRESH | 23 | 23 | 23 |

---

## 1. OPEN Message (Type 1)

RFC 4271 Section 4.2

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|    Version    |     My Autonomous System      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|           Hold Time           |      BGP Identifier           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                         (BGP Identifier cont.)                |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Opt Parm Len  |   Optional Parameters (variable)              |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| Version | 1 | BGP version (always 4) |
| My AS | 2 | 2-byte AS number (use AS_TRANS 23456 if 4-byte) |
| Hold Time | 2 | Hold timer in seconds (0 or >= 3) |
| BGP Identifier | 4 | Router ID (IPv4 format) |
| Opt Parm Len | 1 | Length of optional parameters |
| Optional Parameters | Variable | Capabilities |

### Optional Parameters

```
 0                   1
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Parm. Type    | Parm. Length  |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Parameter Value (variable)           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Parm Type | Name | RFC |
|-----------|------|-----|
| 2 | Capabilities | RFC 5492 |

### Capability TLV (within Optional Parameter Type 2)

```
 0                   1
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Cap. Code     | Cap. Length   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Capability Value (variable)          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

---

## 2. UPDATE Message (Type 2)

RFC 4271 Section 4.3

```
+-----------------------------------------------------+
|   Withdrawn Routes Length (2 octets)                |
+-----------------------------------------------------+
|   Withdrawn Routes (variable)                       |
+-----------------------------------------------------+
|   Total Path Attribute Length (2 octets)            |
+-----------------------------------------------------+
|   Path Attributes (variable)                        |
+-----------------------------------------------------+
|   Network Layer Reachability Information (variable) |
+-----------------------------------------------------+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| Withdrawn Routes Length | 2 | Length of withdrawn routes section |
| Withdrawn Routes | Variable | IPv4 prefixes being withdrawn |
| Path Attribute Length | 2 | Length of path attributes section |
| Path Attributes | Variable | Path attributes (see ATTRIBUTES.md) |
| NLRI | Variable | IPv4 prefixes being announced |

### Withdrawn Routes / NLRI Format (IPv4)

```
+---------------------------+
|   Length (1 octet)        |
+---------------------------+
|   Prefix (variable)       |
+---------------------------+
```

Prefix bytes = ceil(Length / 8)

Example: 10.0.0.0/24 = `18 0A 00 00` (length=24, 3 prefix bytes)

---

## 3. NOTIFICATION Message (Type 3)

RFC 4271 Section 4.5

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Error code    | Error subcode |   Data (variable)             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| Error Code | 1 | Error category |
| Error Subcode | 1 | Specific error |
| Data | Variable | Error-specific data |

### Error Codes

| Code | Name | Subcodes |
|------|------|----------|
| 1 | Message Header Error | 1=Not Synchronized, 2=Bad Length, 3=Bad Type |
| 2 | OPEN Message Error | 1=Unsupported Version, 2=Bad Peer AS, 3=Bad BGP ID, 4=Unsupported Param, 6=Unacceptable Hold Time, 7=Unsupported Capability |
| 3 | UPDATE Message Error | 1=Malformed Attr List, 2=Unrecognized Well-known Attr, 3=Missing Well-known Attr, 4=Attr Flags Error, 5=Attr Length Error, 6=Invalid ORIGIN, 7=AS Routing Loop, 8=Invalid NEXT_HOP, 9=Optional Attr Error, 10=Invalid Network Field, 11=Malformed AS_PATH |
| 4 | Hold Timer Expired | 0=Unspecific |
| 5 | FSM Error | 1=OpenSent, 2=OpenConfirm, 3=Established |
| 6 | Cease | 1=Max Prefixes, 2=Admin Shutdown, 3=Peer Deconfigured, 4=Admin Reset, 5=Connection Rejected, 6=Config Change, 7=Connection Collision, 8=Out of Resources |

### Shutdown Communication (RFC 8203/9003)

For Cease/Admin Shutdown (6,2) and Cease/Admin Reset (6,4):

```
+-------------------+
| Length (1 octet)  |  UTF-8 string length (0-128 legacy, 0-255 extended)
+-------------------+
| UTF-8 Message     |  Human-readable shutdown reason
+-------------------+
```

---

## 4. KEEPALIVE Message (Type 4)

RFC 4271 Section 4.4

```
[Header only - no body]
```

KEEPALIVE has no message body. Total length is always 19 bytes (header only).

---

## 5. ROUTE-REFRESH Message (Type 5)

RFC 2918

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|            AFI                | Reserved      |     SAFI      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| AFI | 2 | Address Family Identifier |
| Reserved | 1 | 0=Query, 1=BoRR, 2=EoRR (RFC 7313) |
| SAFI | 1 | Subsequent Address Family Identifier |

### Reserved Field Values (RFC 7313)

| Value | Name | Description |
|-------|------|-------------|
| 0 | ROUTE_REFRESH_QUERY | Normal route refresh request |
| 1 | ROUTE_REFRESH_BEGIN | Beginning of Route Refresh (BoRR) |
| 2 | ROUTE_REFRESH_END | End of Route Refresh (EoRR) |

---

## Extended Message Support (RFC 8654)

When Extended Message capability is negotiated:
- Maximum message length increases from 4096 to 65535 bytes
- Affects UPDATE messages primarily (large attribute sets)
- Header Length field can contain values > 4096

---

## Go Implementation Notes

### Message Interface

Message embeds WireWriter for zero-allocation encoding:

```go
// WireWriter in internal/bgp/context/context.go (not wire package due to import cycle)
type WireWriter interface {
    Len(ctx *EncodingContext) int
    WriteTo(buf []byte, off int, ctx *EncodingContext) int
}

// Message in internal/bgp/message/message.go
type Message interface {
    context.WireWriter
    Type() MessageType
}

type MessageType uint8

const (
    TypeOPEN         MessageType = 1
    TypeUPDATE       MessageType = 2
    TypeNOTIFICATION MessageType = 3
    TypeKEEPALIVE    MessageType = 4
    TypeROUTEREFRESH MessageType = 5
)

// Context-independent messages ignore context
func (k *Keepalive) Len(_ *context.EncodingContext) int { return HeaderLen }
func (k *Keepalive) WriteTo(buf []byte, off int, _ *context.EncodingContext) int {
    // write 19-byte header...
}

// Context-dependent messages use context for encoding decisions
func (u *Update) Len(ctx *context.EncodingContext) int {
    // Size depends on ASN4, ADD-PATH in context
}
```

### Header Parsing

```go
const (
    MarkerLen = 16
    HeaderLen = 19
    MaxMsgLen = 4096
    ExtMsgLen = 65535
)

var Marker = [16]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
                      0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

func ParseHeader(data []byte) (length uint16, msgType MessageType, err error) {
    if len(data) < HeaderLen {
        return 0, 0, ErrShortRead
    }
    if !bytes.Equal(data[:16], Marker[:]) {
        return 0, 0, ErrInvalidMarker
    }
    length = binary.BigEndian.Uint16(data[16:18])
    msgType = MessageType(data[18])
    return length, msgType, nil
}
```

---

**Created:** 2025-12-19
**Last Updated:** 2026-01-13
