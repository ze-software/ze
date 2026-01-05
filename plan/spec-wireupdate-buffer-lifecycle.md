# Spec: wireupdate-buffer-lifecycle

## Task

Implement explicit buffer lifecycle management for `WireUpdate` with reference counting, allowing buffers to be returned to pool when processing completes instead of relying on GC.

## Required Reading

- [x] `.claude/zebgp/MESSAGE_BUFFER_DESIGN.md` - Reference counting, Acquire/Release pattern
- [x] `.claude/zebgp/POOL_ARCHITECTURE.md` - Pool design, buffer management
- [x] `.claude/zebgp/ENCODING_CONTEXT.md` - WireUpdate current design

**Key insights from docs:**

1. **Reference counting pattern** (MESSAGE_BUFFER_DESIGN.md:230-242):
   ```go
   func (m *PassthroughMessage) Acquire() {
       atomic.AddInt32(&meta.RefCount, 1)
   }
   func (m *PassthroughMessage) Release() {
       if atomic.AddInt32(&meta.RefCount, -1) == 0 {
           m.pool.Put(m)
       }
   }
   ```

2. **Buffer returned to originating pool** (MESSAGE_BUFFER_DESIGN.md:467):
   - Buffer returns to source peer's pool when all references released
   - Prevents unbounded memory growth under load

3. **Current problem**:
   - `WireUpdate` takes ownership of buffer but relies on GC for cleanup
   - Under high UPDATE rate, buffers accumulate until GC runs
   - No explicit return path to session's buffer pool

## Current State Analysis

### Current Flow (GC-based)
```
session.readBuf → WireUpdate(slice) → pool.Get() new buffer
                        ↓
                   GC eventually frees
```

### Target Flow (Reference counted)
```
session.readBuf → WireUpdate(buf, pool, refCount=1) → pool.Get() new buffer
                        ↓
                   Acquire() for each holder
                        ↓
                   Release() when done
                        ↓
                   refCount=0 → pool.Put(buf)
```

## Design

### WireUpdate Changes

```go
// pkg/api/wire_update.go
type WireUpdate struct {
    payload     []byte
    sourceCtxID bgpctx.ContextID

    // Buffer lifecycle
    fullBuf  []byte      // Original full buffer (for return to pool)
    pool     *sync.Pool  // Pool to return buffer to (nil = GC managed)
    refCount int32       // Atomic reference count
}

// NewWireUpdate creates WireUpdate that owns buffer (refCount=1)
func NewWireUpdate(payload []byte, ctxID bgpctx.ContextID) *WireUpdate

// NewWireUpdatePooled creates WireUpdate with pool return capability
func NewWireUpdatePooled(payload, fullBuf []byte, pool *sync.Pool, ctxID bgpctx.ContextID) *WireUpdate

// Acquire increments reference count (for sharing)
func (u *WireUpdate) Acquire()

// Release decrements reference count, returns to pool if zero
func (u *WireUpdate) Release()
```

### Session Changes

```go
// pkg/reactor/session.go - processMessage()
if hdr.Type == message.TypeUPDATE {
    // Create pooled WireUpdate with buffer return capability
    wireUpdate := api.NewWireUpdatePooled(
        body,           // payload slice
        s.readBuf,      // full buffer for pool return
        &readBufPool,   // pool reference
        ctxID,
    )

    // Get fresh buffer BEFORE callback (transfer ownership)
    s.readBuf = readBufPool.Get().([]byte)

    // Callback receives WireUpdate with refCount=1
    s.onMessageReceived(..., wireUpdate, ...)

    // Session calls Release() after handleUpdate
    defer wireUpdate.Release()

    return s.handleUpdate(wireUpdate)
}
```

### Reactor/API Changes

```go
// pkg/reactor/reactor.go - notifyMessageReceiver()
if wireUpdate != nil {
    // Acquire for RawMessage (now 2 refs: session + reactor)
    wireUpdate.Acquire()

    msg.WireUpdate = wireUpdate

    // Cache also acquires if storing
    if direction == "received" {
        wireUpdate.Acquire()  // 3 refs
        r.recentUpdates.Add(&ReceivedUpdate{
            WireUpdate: wireUpdate,  // holds reference
            // ...
        })
    }
}

// When RawMessage is done being processed
// → Release() called

// When ReceivedUpdate expires from cache
// → Release() called
```

### ReceivedUpdate Changes

```go
// pkg/reactor/received_update.go
type ReceivedUpdate struct {
    UpdateID     uint64
    WireUpdate   *api.WireUpdate  // Holds reference (replaces RawBytes)
    // Remove: RawBytes []byte
    // Remove: Attrs *attribute.AttributesWire
    SourcePeerIP netip.Addr
    SourceCtxID  bgpctx.ContextID
    ReceivedAt   time.Time
}

// Derived accessors
func (r *ReceivedUpdate) RawBytes() []byte {
    return r.WireUpdate.Payload()
}

func (r *ReceivedUpdate) Attrs() *attribute.AttributesWire {
    return r.WireUpdate.Attrs()
}
```

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestWireUpdate_RefCount` | `pkg/api/wire_update_test.go` | Acquire/Release counting |
| `TestWireUpdate_PoolReturn` | `pkg/api/wire_update_test.go` | Buffer returned to pool on Release |
| `TestWireUpdate_MultipleAcquire` | `pkg/api/wire_update_test.go` | Multiple holders work correctly |
| `TestWireUpdate_NoPool` | `pkg/api/wire_update_test.go` | GC path still works (pool=nil) |

### Integration Tests

| Test | File | Validates |
|------|------|-----------|
| `TestSession_BufferReturn` | `pkg/reactor/session_test.go` | Buffer returned after UPDATE |
| `TestNotifyMessageReceiver_RefCount` | `pkg/reactor/reactor_test.go` | Proper ref counting through callback |

## Files to Modify

| File | Changes |
|------|---------|
| `pkg/api/wire_update.go` | Add refCount, pool, Acquire(), Release() |
| `pkg/api/wire_update_test.go` | Add ref counting tests |
| `pkg/reactor/session.go` | Use NewWireUpdatePooled, call Release() |
| `pkg/reactor/reactor.go` | Call Acquire() when storing WireUpdate |
| `pkg/reactor/received_update.go` | Store WireUpdate instead of RawBytes |
| `pkg/reactor/recent_updates.go` | Call Release() on eviction |

## Implementation Steps

1. **Write tests** - Create ref counting tests (expect FAIL)
2. **Run tests** - Verify FAIL
3. **Implement WireUpdate changes** - Add refCount, pool, Acquire/Release
4. **Run tests** - Verify PASS
5. **Write session tests** - Buffer return tests (expect FAIL)
6. **Implement session changes** - Use NewWireUpdatePooled
7. **Run tests** - Verify PASS
8. **Write reactor tests** - Ref counting through callback (expect FAIL)
9. **Implement reactor changes** - Acquire/Release calls
10. **Run tests** - Verify PASS
11. **Update ReceivedUpdate** - Store WireUpdate, add Release on eviction
12. **Verify all** - `make test && make lint && make functional`

## Reference Counting Rules

| Event | RefCount Change |
|-------|-----------------|
| `NewWireUpdatePooled()` | = 1 |
| Session stores in callback arg | (no change, session's ref) |
| Reactor stores in RawMessage | +1 (Acquire) |
| Reactor stores in ReceivedUpdate cache | +1 (Acquire) |
| Session's handleUpdate returns | -1 (Release) |
| RawMessage processing completes | -1 (Release) |
| ReceivedUpdate evicted from cache | -1 (Release) |
| RefCount reaches 0 | buffer → pool.Put() |

## Checklist

### 🧪 TDD
- [ ] Unit tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] Update `.claude/zebgp/ENCODING_CONTEXT.md` with new lifecycle
- [ ] Update `.claude/zebgp/UPDATE_BUILDING.md` receive path

### Completion
- [ ] Spec moved to `plan/done/NNN-wireupdate-buffer-lifecycle.md`

---

**Created:** 2026-01-05
**Status:** Planning
