# Plan: Unify MessageID/UpdateID + JSON Format Restructure

## Summary

1. Remove duplicate `UpdateID` from `ReceivedUpdate` - use `WireUpdate.MessageID()`
2. Restructure JSON: common `"message":{"type":...,"id":...,"time":...}` wrapper

**New JSON format (all message types):**
```json
{"message":{"type":"update","id":123,"time":1234567890.123},"announce":{...},"withdraw":{...},"peer":{...}}
{"message":{"type":"open","id":123,"time":...},"version":4,"asn":65001,"hold_time":90,"peer":{...}}
{"message":{"type":"notification","id":123,"time":...},"code":6,"subcode":2,"peer":{...}}
{"message":{"type":"keepalive","id":123,"time":...},"peer":{...}}
```

**`"message"` contains only common fields:** `type`, `id`, `time`
**Message-specific fields stay at top level:** announce, withdraw, code, version, etc.

## Current State (Problem)

```
RawMessage (all msg types)     ReceivedUpdate (UPDATEs only)
├── MessageID uint64  ←─────── UpdateID uint64  (SAME VALUE, DUPLICATED)
├── WireUpdate ───────────────→ WireUpdate
```

- Both use same atomic counter (`msgIDCounter` in received_update.go:17)
- Both assigned same value (reactor.go:2428,2436)
- Redundant storage for UPDATEs

## Target State

```
WireUpdate
├── payload
├── sourceCtxID
└── messageID uint64    ← UPDATE's ID lives here

RawMessage                    ReceivedUpdate
├── MessageID (all types)     └── WireUpdate
├── WireUpdate                     └── .MessageID() for cache key
```

- `RawMessage.MessageID` for all message types (counter still in reactor)
- `WireUpdate.messageID` set from same counter for UPDATEs
- `ReceivedUpdate` uses `WireUpdate.MessageID()` - no separate field
- JSON: `"message":{"type":...,"id":...,"time":...}` common wrapper, specific fields at top level

## Files to Modify

| File | Changes |
|------|---------|
| `pkg/api/wire_update.go` | Add `messageID uint64` field, `MessageID()` accessor, `SetMessageID()` |
| `pkg/reactor/received_update.go` | Remove `UpdateID` field |
| `pkg/reactor/recent_cache.go` | Use `update.WireUpdate.MessageID()` as key |
| `pkg/reactor/reactor.go` | Call `wireUpdate.SetMessageID(messageID)` after creation |
| `pkg/api/json.go` | Restructure all encoders: wrap content in `"message":{...}` with `id`, `time` |
| `pkg/api/text.go` | Update JSON formatting for UPDATEs to use new structure |
| `pkg/api/json_test.go` | Update expected JSON format |
| `pkg/reactor/*_test.go` | Update tests that set `UpdateID` |

## Implementation Steps

### Step 1: Add messageID to WireUpdate

**pkg/api/wire_update.go:**
```go
type WireUpdate struct {
    payload     []byte
    sourceCtxID bgpctx.ContextID
    messageID   uint64              // NEW: set by reactor after creation
    // ... existing fields (attrsOnce, attrs)
}

// MessageID returns the unique identifier for this UPDATE.
func (u *WireUpdate) MessageID() uint64 { return u.messageID }

// SetMessageID sets the message ID (call once, from reactor).
func (u *WireUpdate) SetMessageID(id uint64) { u.messageID = id }
```

Constructor unchanged - ID set by reactor after WireUpdate created.

### Step 2: Set ID in reactor

**pkg/reactor/reactor.go:2421-2440:**
```go
// Zero-copy path for received UPDATE messages
if wireUpdate != nil {
    wireUpdate.SetMessageID(messageID)  // NEW: set ID on WireUpdate

    msg = api.RawMessage{
        Type:       msgType,
        RawBytes:   wireUpdate.Payload(),
        Timestamp:  timestamp,
        Direction:  direction,
        MessageID:  messageID,         // Keep for RawMessage too
        WireUpdate: wireUpdate,
        AttrsWire:  wireUpdate.Attrs(),
    }

    if direction == "received" {
        r.recentUpdates.Add(&ReceivedUpdate{
            // UpdateID removed - use WireUpdate.MessageID()
            WireUpdate:   wireUpdate,
            SourcePeerIP: peerAddr,
            ReceivedAt:   timestamp,
        })
    }
}
```

### Step 3: Remove UpdateID from ReceivedUpdate

**pkg/reactor/received_update.go:28-31:**
```go
type ReceivedUpdate struct {
    // UpdateID uint64  // REMOVED - use WireUpdate.MessageID()
    WireUpdate *api.WireUpdate
    // ... rest unchanged
}
```

### Step 4: Update cache to use WireUpdate.MessageID()

**pkg/reactor/recent_cache.go:71:**
```go
// Before:
c.entries[update.UpdateID] = &cacheEntry{...}

// After:
c.entries[update.WireUpdate.MessageID()] = &cacheEntry{...}
```

### Step 5: Restructure JSON format

**New structure for all message types:**
- `"message":{"type":...,"id":...,"time":...}` - common fields only
- Message-specific fields at top level (announce, withdraw, code, etc.)
- `peer` stays at top level

**Helper function for common message wrapper:**
```go
// messageWrapper creates the common "message" object
func (e *JSONEncoder) messageWrapper(msgType string, msgID uint64) map[string]any {
    return map[string]any{
        "type": msgType,
        "id":   msgID,
        "time": float64(e.timeFunc().UnixNano()) / 1e9,
    }
}
```

**pkg/api/json.go - all message types:**
```go
// Before:
msg := e.message(peer, "notification")
msg["direction"] = direction
if msgID > 0 { msg["msg-id"] = msgID }

// After:
msg := make(map[string]any)
msg["message"] = e.messageWrapper("notification", msgID)
msg["direction"] = direction
// notification-specific fields at top level
msg["code"] = notify.ErrorCode
msg["subcode"] = notify.ErrorSubcode
msg["peer"] = peerObj
```

**Pattern applies to all types:**
- Open: `msg["version"]`, `msg["asn"]`, etc. at top level
- Notification: `msg["code"]`, `msg["subcode"]` at top level
- Keepalive: only `message` and `peer`
- Update: `msg["announce"]`, `msg["withdraw"]` at top level

## Test Updates

| Test File | Changes |
|-----------|---------|
| `pkg/reactor/received_update_test.go` | Remove `UpdateID:` from struct literals |
| `pkg/reactor/recent_cache_test.go` | Use `WireUpdate.MessageID()` where needed |
| `pkg/reactor/forward_split_test.go` | Same |
| `pkg/api/json_test.go` | Update expected JSON: `"message":{"id":...}` structure |
| `pkg/api/text_test.go` | Update expected output strings if needed |
| `pkg/api/wire_update_test.go` | Add tests for `MessageID()` and `SetMessageID()` |

## Verification

```bash
make test && make lint && make functional
```

## Breaking Changes

**JSON API restructure:**
```
Old: {"type":"update","msg-id":123,"time":...,"announce":{...}}
New: {"message":{"type":"update","id":123,"time":...},"announce":{...}}
```

Consumers need to update JSON parsing:
- Access type via `msg.message.type` instead of `msg.type`
- Access ID via `msg.message.id` instead of `msg["msg-id"]`
- Access time via `msg.message.time` instead of `msg.time`
- announce/withdraw/code/etc. stay at top level (unchanged)
