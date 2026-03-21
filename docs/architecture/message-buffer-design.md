# Message Buffer Design

Zero-copy message handling with capability-based forwarding.

---

> **Current vs. Future Design**
>
> This document describes a **future design** with `PassthroughMessage`, ref counting,
> and capability masks. The current implementation uses a simpler approach:
>
> - **Ownership transfer** - callback returns `kept=true` to take buffer ownership
> - **Two size-appropriate pools** - 4K (pre-OPEN) and 64K (Extended Message)
> - **Zero-copy cache** - cache owns buffer, `Take()` transfers ownership
> - **No ref counting** - single owner at a time (session → cache → caller)
> - **Critical ordering** - callback executes BEFORE cache (prevents use-after-free)
>
> **Cache API:**
> - `Add()` - cache takes ownership
> - `Take(id)` - removes entry, transfers ownership (caller must `Release()`)
> - `Contains(id)` - check existence without ownership transfer
> - `Delete(id)` - remove and return buffer to pool
>
> See `ENCODING_CONTEXT.md` and `UPDATE_BUILDING.md` for current implementation.
> This design may be implemented when high-volume route reflection requires
> true zero-copy forwarding to multiple peers simultaneously.

---

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Peer A (incoming)                         │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ PeerBufferPool (sized after OPEN negotiation)           │    │
│  │   maxSize = 4096 (or 65535 if extended message)         │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     PassthroughMessage                           │
│  ┌──────────────────┬────────┬──────┬────────────────────────┐  │
│  │ Marker (16 bytes)│ Length │ Type │ Payload                │  │
│  │ → MessageMeta    │ (2)    │ (1)  │ (variable)             │  │
│  │   after read     │        │      │                        │  │
│  └──────────────────┴────────┴──────┴────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
│ Peer B           │ │ Peer C           │ │ Peer D           │
│ caps: ASN4+AP    │ │ caps: ASN4       │ │ caps: ASN2       │
│ ✓ compatible     │ │ ✓ compatible     │ │ ✗ needs repack   │
└──────────────────┘ └──────────────────┘ └──────────────────┘
```

---

## Capability Mask

Instead of tracking which peers to forward to, we track what capabilities a message requires. Peers are checked for compatibility at forward time.

```go
type CapabilityMask uint64

const (
    // Session type (mutually exclusive: 0=IBGP, 1=EBGP)
    CapEBGP CapabilityMask = 1 << 0

    // ASN encoding (mutually exclusive: 0=ASN2, 1=ASN4)
    CapASN4 CapabilityMask = 1 << 1

    // Message size
    CapExtendedMessage CapabilityMask = 1 << 2  // >4096 bytes

    // ADD-PATH per family
    CapAddPathIPv4Unicast   CapabilityMask = 1 << 8
    CapAddPathIPv4Multicast CapabilityMask = 1 << 9
    CapAddPathIPv6Unicast   CapabilityMask = 1 << 10
    CapAddPathIPv6Multicast CapabilityMask = 1 << 11
    CapAddPathVPNv4         CapabilityMask = 1 << 12
    CapAddPathVPNv6         CapabilityMask = 1 << 13
    CapAddPathEVPN          CapabilityMask = 1 << 14
    CapAddPathFlowSpec      CapabilityMask = 1 << 15

    // Address families (for MP-BGP)
    CapIPv4Unicast   CapabilityMask = 1 << 24
    CapIPv4Multicast CapabilityMask = 1 << 25
    CapIPv6Unicast   CapabilityMask = 1 << 26
    CapIPv6Multicast CapabilityMask = 1 << 27
    CapVPNv4         CapabilityMask = 1 << 28
    CapVPNv6         CapabilityMask = 1 << 29
    CapEVPN          CapabilityMask = 1 << 30
    CapFlowSpecv4    CapabilityMask = 1 << 31
    CapFlowSpecv6    CapabilityMask = 1 << 32
    CapBGPLS         CapabilityMask = 1 << 33

    // Future expansion: bits 3-7, 16-23, 34-63 available
)

// Helper functions for exclusive capabilities
func (c CapabilityMask) IsEBGP() bool { return c&CapEBGP != 0 }
func (c CapabilityMask) IsIBGP() bool { return c&CapEBGP == 0 }
func (c CapabilityMask) IsASN4() bool { return c&CapASN4 != 0 }
func (c CapabilityMask) IsASN2() bool { return c&CapASN4 == 0 }

// Peer's negotiated capabilities
type PeerCapabilities struct {
    mask CapabilityMask
}

// Masks for exclusive capability groups
const (
    // Exclusive capabilities must match exactly
    maskExclusive = CapEBGP | CapASN4

    // Additive capabilities: peer must have if message requires
    maskAdditive = CapExtendedMessage |
        CapAddPathIPv4Unicast | CapAddPathIPv4Multicast |
        CapAddPathIPv6Unicast | CapAddPathIPv6Multicast |
        CapAddPathVPNv4 | CapAddPathVPNv6 |
        CapAddPathEVPN | CapAddPathFlowSpec |
        CapIPv4Unicast | CapIPv4Multicast |
        CapIPv6Unicast | CapIPv6Multicast |
        CapVPNv4 | CapVPNv6 | CapEVPN |
        CapFlowSpecv4 | CapFlowSpecv6 | CapBGPLS
)

// CanPassThrough checks if peer can receive message unchanged
func (p *PeerCapabilities) CanPassThrough(required CapabilityMask) bool {
    // Exclusive capabilities must match exactly
    // (EBGP↔EBGP, IBGP↔IBGP, ASN4↔ASN4, ASN2↔ASN2)
    if (p.mask & maskExclusive) != (required & maskExclusive) {
        return false
    }

    // Additive capabilities: peer must have all that message requires
    reqAdditive := required & maskAdditive
    if (p.mask & reqAdditive) != reqAdditive {
        return false
    }

    return true
}

// Example scenarios:
//
// Message from EBGP+ASN4 peer with ADD-PATH:
//   required = CapEBGP | CapASN4 | CapAddPathIPv4Unicast
//
// Peer B: EBGP+ASN4+ADD-PATH → CanPassThrough = true
// Peer C: EBGP+ASN4          → CanPassThrough = false (missing ADD-PATH)
// Peer D: IBGP+ASN4+ADD-PATH → CanPassThrough = false (EBGP≠IBGP)
// Peer E: EBGP+ASN2+ADD-PATH → CanPassThrough = false (ASN4≠ASN2)
```

---

## Message Metadata (Marker Overlay)

After validating the 16-byte marker, overlay metadata for processing:

```go
// BGP Marker (RFC 4271)
var Marker = [16]byte{
    0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
    0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

// MessageMeta overlays the marker space after validation
// Total: 16 bytes (fits exactly in marker space)
type MessageMeta struct {
    RefCount     int32          // 4 bytes: peers still holding reference
    Flags        uint32         // 4 bytes: processing flags
    RequiredCaps CapabilityMask // 8 bytes: capabilities needed to receive unchanged
}

// Processing flags
const (
    FlagParsed      uint32 = 1 << 0  // Message has been parsed
    FlagHasWithdraw uint32 = 1 << 1  // UPDATE contains withdrawals
    FlagHasAnnounce uint32 = 1 << 2  // UPDATE contains announcements
    FlagEOR         uint32 = 1 << 3  // End-of-RIB marker
)
```

---

## Passthrough Message

```go
type PassthroughMessage struct {
    data   []byte          // full message buffer
    pool   *PeerBufferPool // return here when done
    length int             // actual message length (may be < len(data))
}

// Meta returns pointer to metadata overlay (zero allocation)
func (m *PassthroughMessage) Meta() *MessageMeta {
    return (*MessageMeta)(unsafe.Pointer(&m.data[0]))
}

// Length returns BGP message length from header
func (m *PassthroughMessage) Length() uint16 {
    return binary.BigEndian.Uint16(m.data[16:18])
}

// Type returns BGP message type
func (m *PassthroughMessage) Type() MessageType {
    return MessageType(m.data[18])
}

// Payload returns message payload (after header)
func (m *PassthroughMessage) Payload() []byte {
    return m.data[19:m.length]
}

// InitAfterRead validates marker and initializes metadata
func (m *PassthroughMessage) InitAfterRead(fromPeer *PeerCapabilities) error {
    // Validate marker
    if !bytes.Equal(m.data[0:16], Marker[:]) {
        return ErrInvalidMarker
    }

    // Validate length
    msgLen := m.Length()
    if msgLen < 19 || int(msgLen) > len(m.data) {
        return ErrInvalidLength
    }
    m.length = int(msgLen)

    // Initialize metadata (overwrites marker)
    meta := m.Meta()
    meta.RefCount = 0
    meta.Flags = 0
    meta.RequiredCaps = fromPeer.mask  // Inherit sender's capabilities as requirements

    return nil
}

// WriteTo restores marker and writes message
func (m *PassthroughMessage) WriteTo(w io.Writer) (int64, error) {
    // Restore marker before writing
    copy(m.data[0:16], Marker[:])

    n, err := w.Write(m.data[:m.length])
    return int64(n), err
}

// Acquire increments reference count
func (m *PassthroughMessage) Acquire() {
    meta := m.Meta()
    atomic.AddInt32(&meta.RefCount, 1)
}

// Release decrements reference count, returns to pool if zero
func (m *PassthroughMessage) Release() {
    meta := m.Meta()
    if atomic.AddInt32(&meta.RefCount, -1) == 0 {
        m.pool.Put(m)
    }
}
```

---

## Peer Buffer Pool

Each peer has its own buffer pool, sized after OPEN negotiation:

```go
type PeerBufferPool struct {
    maxSize int        // negotiated max message size
    pool    sync.Pool  // buffer recycling
}

// NewPeerBufferPool creates pool after OPEN negotiation
func NewPeerBufferPool(extendedMessage bool) *PeerBufferPool {
    maxSize := 4096  // standard BGP max
    if extendedMessage {
        maxSize = 65535  // RFC 8654
    }

    p := &PeerBufferPool{maxSize: maxSize}
    p.pool.New = func() interface{} {
        return &PassthroughMessage{
            data: make([]byte, maxSize),
            pool: p,
        }
    }
    return p
}

// Get retrieves a buffer for reading
func (p *PeerBufferPool) Get() *PassthroughMessage {
    msg := p.pool.Get().(*PassthroughMessage)
    msg.length = 0
    return msg
}

// Put returns a buffer to the pool
func (p *PeerBufferPool) Put(msg *PassthroughMessage) {
    // Clear metadata before recycling
    meta := msg.Meta()
    meta.RefCount = 0
    meta.Flags = 0
    meta.RequiredCaps = 0

    p.pool.Put(msg)
}
```

---

## Forwarding Logic

```go
type Router struct {
    peers map[string]*Peer
}

// ForwardMessage sends message to compatible peers
func (r *Router) ForwardMessage(msg *PassthroughMessage, fromPeer *Peer) {
    required := msg.Meta().RequiredCaps

    for _, peer := range r.peers {
        if peer == fromPeer {
            continue  // don't send back to sender
        }

        if peer.caps.CanReceive(required) {
            // Pass-through: peer has all required capabilities
            msg.Acquire()
            peer.SendPassthrough(msg)
        } else {
            // Repack needed: peer lacks some capabilities
            peer.SendRepacked(msg, required)
        }
    }
}

// Peer.SendPassthrough queues message for unchanged forwarding
func (p *Peer) SendPassthrough(msg *PassthroughMessage) {
    p.outQueue <- msg  // msg.Release() called after write
}

// Peer.SendRepacked converts message for peer's capabilities
func (p *Peer) SendRepacked(msg *PassthroughMessage, originalCaps CapabilityMask) {
    // Check cache first
    repacked := p.repackCache.Get(msg, p.caps.mask)
    if repacked != nil {
        p.outQueue <- repacked
        return
    }

    // Parse and repack
    repacked = p.repackMessage(msg, originalCaps)
    p.repackCache.Put(msg, p.caps.mask, repacked)
    p.outQueue <- repacked
}
```

---

## Repack Cache

Cache repacked messages by capability combination:

```go
type RepackCache struct {
    mu    sync.RWMutex
    cache map[repackKey]*PassthroughMessage
}

type repackKey struct {
    originalHash uint64         // hash of original message
    targetCaps   CapabilityMask // target peer capabilities
}

func (c *RepackCache) Get(original *PassthroughMessage, targetCaps CapabilityMask) *PassthroughMessage {
    key := repackKey{
        originalHash: hashMessage(original),
        targetCaps:   targetCaps,
    }

    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.cache[key]
}

func (c *RepackCache) Put(original *PassthroughMessage, targetCaps CapabilityMask, repacked *PassthroughMessage) {
    key := repackKey{
        originalHash: hashMessage(original),
        targetCaps:   targetCaps,
    }

    c.mu.Lock()
    defer c.mu.Unlock()
    c.cache[key] = repacked
}
```

---

## Read Flow

```go
func (p *Peer) readLoop() {
    for {
        // Get buffer from peer's pool
        msg := p.bufferPool.Get()

        // Read message
        n, err := io.ReadFull(p.conn, msg.data[:19])  // header
        if err != nil {
            p.bufferPool.Put(msg)
            return
        }

        // Read payload
        payloadLen := msg.Length() - 19
        if payloadLen > 0 {
            _, err = io.ReadFull(p.conn, msg.data[19:19+payloadLen])
            if err != nil {
                p.bufferPool.Put(msg)
                return
            }
        }

        // Initialize metadata
        if err := msg.InitAfterRead(&p.caps); err != nil {
            p.bufferPool.Put(msg)
            continue
        }

        // Process
        p.handleMessage(msg)
    }
}
```

---

## Write Flow

```go
func (p *Peer) writeLoop() {
    for msg := range p.outQueue {
        // WriteTo restores marker and writes
        _, err := msg.WriteTo(p.conn)

        // Release our reference
        msg.Release()

        if err != nil {
            return
        }
    }
}
```

---

## Memory Flow Summary

```
1. Peer A connects, OPEN negotiated
   → PeerBufferPool created (maxSize=4096 or 65535)

2. Read message from Peer A
   → Get buffer from A's pool
   → Read into buffer
   → Validate marker, overlay metadata

3. Forward decision
   → Check each peer's capabilities vs message requirements
   → Compatible: pass-through (Acquire + queue)
   → Incompatible: repack (check cache, or parse/repack)

4. Write to Peer B
   → Restore marker
   → Write to connection
   → Release reference

5. All peers done
   → RefCount hits 0
   → Buffer returned to Peer A's pool

6. Peer A disconnects
   → Pool garbage collected
```

---

## Related Specs

- `plan/spec-wireupdate-buffer-lifecycle.md` - Buffer pool get/return lifecycle (current impl)
- `plan/spec-wireupdate-split.md` - Wire-level UPDATE splitting (TODO)

---

**Last Updated:** 2025-01-05
