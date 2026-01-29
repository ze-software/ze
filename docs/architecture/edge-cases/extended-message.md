# Extended Message Support (RFC 8654)

**Source:** ExaBGP `bgp/message/open/capability/extended.py`, `bgp/message/message.py`
**Purpose:** Document extended message handling

---

## Overview

Extended Message capability allows BGP messages larger than 4096 bytes, up to 65535 bytes.

### Key Values

| Constant | Value | Description |
|----------|-------|-------------|
| HEADER_LEN | 19 | BGP header size (fixed) |
| INITIAL_SIZE | 4096 | Standard maximum message size |
| EXTENDED_SIZE | 65535 | Extended maximum message size |

---

## Capability Negotiation

### Extended Message Capability (Code 6)

```
+---------------------------+
|   Capability Code = 6     |  1 octet
+---------------------------+
|   Capability Length = 0   |  1 octet
+---------------------------+
```

The capability has no data - its presence indicates support.

### Negotiation

Extended message is used only if **both** peers advertise the capability:

```python
def msg_size(self) -> int:
    if self.sent.extended_message and self.received.extended_message:
        return ExtendedMessage.EXTENDED_SIZE
    return ExtendedMessage.INITIAL_SIZE
```

---

## Message Header

### Standard Header (RFC 4271)

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
+                                                               +
|                                                               |
+                           Marker                              +
|                         (16 bytes)                            |
+                                                               +
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Length               |      Type     |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

- **Marker:** 16 bytes, all 0xFF
- **Length:** 2 bytes, total message length (19-4096 or 19-65535)
- **Type:** 1 byte, message type code

### Length Field

The length field interpretation depends on negotiation:

| Condition | Valid Range |
|-----------|-------------|
| No extended message | 19 - 4096 |
| Extended message negotiated | 19 - 65535 |

---

## When Extended Messages Are Needed

### Large UPDATE Messages

Common scenarios requiring extended messages:

1. **Full routing table:** Many NLRIs in single UPDATE
2. **Many attributes:** Large community lists
3. **FlowSpec:** Complex flow rules
4. **BGP-LS:** Link-state database dumps

### Example Size Calculation

```
UPDATE with 1000 IPv4 prefixes:
- Header: 19 bytes
- Withdrawn length: 2 bytes
- Path attribute length: 2 bytes
- Attributes: ~100 bytes (typical)
- NLRI: 1000 * 4 bytes avg = 4000 bytes
Total: ~4123 bytes > 4096 → needs extended message
```

---

## ExaBGP Implementation

### Capability Class

```python
@Capability.register()
class ExtendedMessage(Capability):
    ID = Capability.CODE.EXTENDED_MESSAGE
    INITIAL_SIZE = 4096
    EXTENDED_SIZE = 65535

    def extract_capability_bytes(self) -> list[bytes]:
        return [b'']  # Empty capability data

    @classmethod
    def unpack_capability(cls, instance, data, capability):
        return cls()  # No data to parse

    def json(self) -> str:
        return '{ "name": "extended-message", "size": %d }' % self.EXTENDED_SIZE
```

### Negotiated Check

```python
class Negotiated:
    def __init__(self):
        self.sent = Capabilities()
        self.received = Capabilities()

    @property
    def msg_size(self) -> int:
        if self.sent.extended_message and self.received.extended_message:
            return ExtendedMessage.EXTENDED_SIZE
        return ExtendedMessage.INITIAL_SIZE
```

### Message Reading

```python
def read_message(self, negotiated):
    # Read header
    header = self.read_exact(19)
    marker = header[:16]
    length = unpack('!H', header[16:18])[0]
    msg_type = header[18]

    # Validate marker
    if marker != Message.MARKER:
        raise Notify(1, 1, 'Invalid marker')

    # Validate length
    max_size = negotiated.msg_size
    if length < 19 or length > max_size:
        raise Notify(1, 2, f'Invalid length {length}')

    # Read body
    body_len = length - 19
    body = self.read_exact(body_len)

    return msg_type, body
```

### Message Writing

```python
def write_message(self, msg_type, body, negotiated):
    length = 19 + len(body)

    # Check size limit
    max_size = negotiated.msg_size
    if length > max_size:
        raise ValueError(f'Message too large: {length} > {max_size}')

    # Build header
    header = Message.MARKER + pack('!HB', length, msg_type)

    # Send
    self.write(header + body)
```

---

## Fallback Behavior

When extended message not negotiated but UPDATE is large:

### Option 1: Split Updates

```python
def send_updates(self, updates, negotiated):
    max_nlri_per_update = calculate_max_nlri(negotiated.msg_size)

    for chunk in chunks(updates, max_nlri_per_update):
        body = pack_update(chunk, negotiated)
        self.write_message(UPDATE, body, negotiated)
```

### Option 2: Reject

```python
def send_update(self, update, negotiated):
    body = pack_update(update, negotiated)
    if len(body) + 19 > negotiated.msg_size:
        raise ValueError('Update too large for peer capabilities')
    self.write_message(UPDATE, body, negotiated)
```

---

## Wire Examples

### Extended Message Capability in OPEN

```
02                  # Capability code 2 (optional parameters)
02                  # Length 2
06                  # Extended Message capability code
00                  # Capability length 0
```

### Large UPDATE Header

```
FF FF FF FF FF FF FF FF FF FF FF FF FF FF FF FF  # Marker
10 00                                            # Length = 4096 (0x1000)
02                                               # Type = UPDATE
```

### Extended UPDATE Header

```
FF FF FF FF FF FF FF FF FF FF FF FF FF FF FF FF  # Marker
50 00                                            # Length = 20480 (0x5000)
02                                               # Type = UPDATE
```

---

## Ze Implementation Notes

### Capability

```go
type ExtendedMessage struct{}

func (e *ExtendedMessage) Code() uint8 {
    return 6
}

func (e *ExtendedMessage) Pack() []byte {
    return []byte{}  // Empty
}

const (
    InitialMsgSize  = 4096
    ExtendedMsgSize = 65535
)
```

### Negotiated Size

```go
type Negotiated struct {
    SentExtended     bool
    ReceivedExtended bool
}

func (n *Negotiated) MsgSize() int {
    if n.SentExtended && n.ReceivedExtended {
        return ExtendedMsgSize
    }
    return InitialMsgSize
}
```

### Reading Messages

```go
func (c *Conn) ReadMessage(neg *Negotiated) (uint8, []byte, error) {
    // Read header
    header := make([]byte, 19)
    if _, err := io.ReadFull(c.conn, header); err != nil {
        return 0, nil, err
    }

    // Validate marker
    if !bytes.Equal(header[:16], Marker) {
        return 0, nil, ErrInvalidMarker
    }

    length := binary.BigEndian.Uint16(header[16:18])
    msgType := header[18]

    // Validate length
    maxSize := neg.MsgSize()
    if length < 19 || int(length) > maxSize {
        return 0, nil, fmt.Errorf("invalid length %d", length)
    }

    // Read body
    body := make([]byte, length-19)
    if _, err := io.ReadFull(c.conn, body); err != nil {
        return 0, nil, err
    }

    return msgType, body, nil
}
```

---

**Last Updated:** 2025-12-19
