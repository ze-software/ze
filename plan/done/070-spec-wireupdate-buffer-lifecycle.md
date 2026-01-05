# Spec: wireupdate-buffer-lifecycle

## Task

Implement clean buffer lifecycle: get from pool, read, process, return to pool.

## Current State (Needs Refactor)

```go
// Current: persistent s.readBuf with swap
s.readBuf exists
conn.Read(s.readBuf)
oldBuf := s.readBuf
s.readBuf = pool.Get()
process(oldBuf)
pool.Put(oldBuf)
```

**Problems:**
- Confusing swap logic
- Persistent field when not needed
- Pool acts as "next buffer" source, not direct recycling

## Target Design

```go
// Zero-copy: get from pool, cache takes ownership or return
buf := s.getReadBuffer()    // from pool, sized appropriately

conn.Read(buf)
err, kept := process(buf)   // callback returns kept=true if caching

if !kept {
    s.returnReadBuffer(buf) // return if not cached
}
// If cached: cache returns to pool on eviction
```

## Implementation

### 1. Separate Pools by Size

```go
// pkg/reactor/session.go
var (
    readBufPool4K = sync.Pool{
        New: func() any { return make([]byte, message.MaxMsgLen) },  // 4096
    }
    readBufPool64K = sync.Pool{
        New: func() any { return make([]byte, message.ExtMsgLen) }, // 65535
    }
)
```

### 2. Session Buffer Methods

```go
type Session struct {
    // ... existing fields ...

    // Remove: readBuf []byte (no longer persistent)

    // Track negotiated size for pool selection.
    // Thread safety: only accessed from session's read goroutine:
    //   negotiate() ← handleOpen() ← processMessage() ← readAndProcessMessage()
    // No synchronization needed.
    extendedMessage bool
}

// getReadBuffer gets appropriately-sized buffer from pool
func (s *Session) getReadBuffer() []byte {
    if s.extendedMessage {
        return readBufPool64K.Get().([]byte)
    }
    return readBufPool4K.Get().([]byte)
}

// returnReadBuffer returns buffer to appropriate pool based on capacity
func (s *Session) returnReadBuffer(buf []byte) {
    if cap(buf) >= message.ExtMsgLen {  // NOTE: cap(), not len()
        readBufPool64K.Put(buf)
    } else {
        readBufPool4K.Put(buf)
    }
}
```

### 3. Clean readAndProcessMessage

```go
func (s *Session) readAndProcessMessage(conn net.Conn) error {
    // Get buffer from pool
    buf := s.getReadBuffer()

    // Read header
    _, err := io.ReadFull(conn, buf[:message.HeaderLen])
    if err != nil {
        s.returnReadBuffer(buf)
        if errors.Is(err, io.EOF) {
            s.handleConnectionClose()
            return ErrConnectionClosed
        }
        return err
    }

    hdr, err := message.ParseHeader(buf[:message.HeaderLen])
    if err != nil {
        s.returnReadBuffer(buf)
        _ = s.fsm.Event(fsm.EventBGPHeaderErr)
        return fmt.Errorf("parse header: %w", err)
    }

    // Validate length...
    // (existing validation code)

    // Read body
    bodyLen := int(hdr.Length) - message.HeaderLen
    if bodyLen > 0 {
        _, err = io.ReadFull(conn, buf[message.HeaderLen:hdr.Length])
        if err != nil {
            s.returnReadBuffer(buf)
            return fmt.Errorf("read body: %w", err)
        }
    }

    // Process message
    err = s.processMessage(&hdr, buf[message.HeaderLen:hdr.Length])

    // Return buffer to pool
    s.returnReadBuffer(buf)

    return err
}
```

### 4. Update negotiate() for Extended Message

```go
func (s *Session) negotiate() {
    // ... existing negotiation ...

    // Track extended message for pool selection
    if s.negotiated.ExtendedMessage {
        s.extendedMessage = true
    }

    // Remove: s.readBuf resize logic (no longer needed)
}
```

## Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    RECEIVE PATH (Ownership Transfer)            │
│                                                                 │
│  buf := getReadBuffer()    ← Get from appropriate pool          │
│         ↓                                                       │
│  conn.Read(buf)            ← Read directly into pool buffer     │
│         ↓                                                       │
│  receiver.OnMessageReceived() ← Callback FIRST (buf valid)      │
│         ↓                                                       │
│  Cache.Add(buf)?                                                │
│         │                                                       │
│    ┌────┴────┐                                                  │
│    │         │                                                  │
│   YES       NO (cache full)                                     │
│    │         │                                                  │
│    ↓         ↓                                                  │
│  kept=true  kept=false                                          │
│  cache owns  session returns buf after handleUpdate()           │
│    │                                                            │
│    ↓                                                            │
│  Take() removes entry, transfers ownership to caller            │
│    ↓                                                            │
│  caller.Release() → pool                                        │
└─────────────────────────────────────────────────────────────────┘
```

**Critical design points:**
1. Callback executes BEFORE cache (prevents use-after-free when cache full)
2. `Add()` returns false if cache full - session keeps buffer ownership
3. `Take()` removes entry and transfers ownership (prevents race condition)
4. `Release()` method on ReceivedUpdate for caller to return buffer

## Files Modified

| File | Changes |
|------|---------|
| `pkg/reactor/session.go` | Remove `readBuf` field, add `extendedMessage` bool, add `getReadBuffer()`/`returnReadBuffer()`/`ReturnReadBuffer()`, `MessageCallback` returns bool, `processMessage` returns `(error, kept)` |
| `pkg/reactor/reactor.go` | `notifyMessageReceiver` accepts `buf`, returns `kept`, callback BEFORE cache, zero-copy (no copy) |
| `pkg/reactor/received_update.go` | Add `poolBuf []byte` field, add `Release()` method |
| `pkg/reactor/recent_cache.go` | `Get()` → `Take()` (removes entry), add `Contains()`, return buf on eviction/delete |

**Note:** `ReceivedUpdate.Announces`, `Withdraws`, `AnnounceWire`, `WithdrawWire`, and `ConvertToRoutes()`
are being removed in a separate refactor. See `spec-wireupdate-split.md`.

## Design Notes

### Initial Read Before OPEN Negotiation

Before OPEN is received, `extendedMessage` is false, so 4K pool is used. This is correct:
- OPEN message is always ≤4096 bytes (RFC 4271)
- Extended Message capability (RFC 8654) only applies to UPDATE messages
- After `negotiate()` sets `extendedMessage = true`, subsequent reads use 64K pool

### Why Two Pools Instead of One?

Single pool with mixed sizes would:
- Return 64K buffers to sessions that only need 4K (memory waste)
- Or require size checking on every Get() (complexity)

Separate pools ensure each session gets appropriately-sized buffers.

## Benefits

1. **Zero-copy cache** - no `make`/`copy` when caching UPDATE
2. **Clear ownership** - single owner (session or cache), never both
3. **No confusing swap** - ownership transfer via `kept` return value
4. **Simpler state** - no persistent `readBuf` field
5. **Correct pool usage** - size-appropriate pools (4K/64K)

## Checklist

### Implementation
- [x] Add separate 4K and 64K pools
- [x] Add `extendedMessage` field to Session
- [x] Add `getReadBuffer()` / `returnReadBuffer()` methods
- [x] Refactor `readAndProcessMessage()` to use get/return pattern
- [x] Update `negotiate()` to set `extendedMessage` flag
- [x] Remove `readBuf` field from Session
- [x] Update `NewSession()` to not allocate readBuf

### Verification
- [x] `make test` passes (pkg/reactor tests pass; unrelated pkg/api/process_test.go failures)
- [x] `make lint` passes (no session.go lint errors)
- [x] `make functional` passes (37 tests)

### Documentation
- [x] Update `ENCODING_CONTEXT.md` with clean flow
- [x] Update `MESSAGE_BUFFER_DESIGN.md` with clean flow

## Related Specs

- `spec-wireupdate-split.md` - Wire-level UPDATE splitting (removes ConvertToRoutes)

## Review Findings (2025-01-05)

Issues found and fixed in this spec:
1. ~~Section 5 showed obsolete copy code~~ → Deleted (implementation is zero-copy)
2. ~~`len(buf)` in returnReadBuffer~~ → Fixed to `cap(buf)`
3. ~~Flow diagram incomplete for cache-full~~ → Updated with both paths
4. `ReceivedUpdate.Announces/Withdraws` never populated → Moving to `spec-wireupdate-split.md`

---

**Created:** 2025-01-05
**Completed:** 2025-01-05
**Reviewed:** 2025-01-05
**Status:** ✅ Implemented (with cleanup pending in spec-wireupdate-split.md)
**Approach:** Clean get/return pool pattern with size-appropriate pools
