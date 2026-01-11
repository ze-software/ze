# Documentation vs Code Issues Report

**Generated:** 2026-01-11
**Status:** Major and medium issues require discussion before fixing

---

## Major Issues (P0)

### 1. Pool Architecture Documents Phase 5 Target, Not Current Implementation

**Location:** `docs/architecture/pool-architecture.md`

**Problem:** The entire pool-architecture.md describes a sophisticated double-buffer pool design that is NOT the current implementation:

| Documented | Actual |
|------------|--------|
| `pkg/pool/Pool` with MSB handle bits | `internal/store/AttributeStore[T]` |
| Global `CompactionScheduler` | Per-type worker goroutines |
| Double-buffer alternation | Hash-map with collision chaining |
| `PassthroughBuffer`, `PackedMessageCache` | Not implemented |

**Evidence:**
- `pkg/pool/pool.go:61-63` has unused fields: `_state PoolState // nolint:unused // Phase 5: compaction`
- `pkg/rib/store.go:18-22` uses `attrStores map[AttributeCode]*attrStore` not `Pool`
- `internal/store/attribute.go:32` defines `AttributeStore[T]` with worker goroutines

**Impact:** Developers following the docs will search for patterns that don't exist.

**Recommended Fix Options:**
1. **Option A:** Add prominent header: "This documents Phase 5 target architecture. Current implementation uses `internal/store/` types."
2. **Option B:** Rewrite to document actual `AttributeStore`/`FamilyStore` implementation
3. **Option C:** Keep as-is if Phase 5 implementation is imminent

---

### 2. "announce route" Command Documented But Removed

**Location:** `docs/architecture/api/commands.md:127-136`

**Problem:** Documentation shows:
```
peer <selector> announce route <prefix> next-hop <ip> [attributes...]
```

But code at `pkg/plugin/route.go:229` explicitly states:
```go
// NOTE: "announce route" removed in favor of "update text" (new-syntax.md)
```

**Impact:** Users following documented syntax get "unknown command" errors.

**Recommended Fix Options:**
1. **Option A:** Remove `announce route` examples from commands.md entirely
2. **Option B:** Add deprecation notice pointing to `update text` syntax
3. **Option C:** Keep examples but add "DEPRECATED" label with migration path

---

## Medium Issues (P1)

### 3. msg-id Cache Control Commands Not Implemented

**Location:** `docs/architecture/api/capability-contract.md:71-96`

**Problem:** Documentation shows these commands for API programs to control message cache:
```
msg-id 123 retain    # Keep until released
msg-id 123 release   # Allow eviction
msg-id 123 expire    # Remove immediately
msg-id list          # List cached msg-ids
```

**Reality:** No handlers registered for these commands. Search for `msg-id` in `pkg/plugin/` returns zero handler matches.

**Impact:** API programs cannot prevent route eviction across peer restarts.

**Recommended Fix Options:**
1. **Option A:** Implement the handlers
2. **Option B:** Mark section as "Future - Not Yet Implemented"
3. **Option C:** Remove from docs until implemented

---

### 4. borr/eorr Markers Not Implemented

**Location:** `docs/architecture/api/capability-contract.md:135-149`

**Problem:** Documentation shows Enhanced Route Refresh markers:
```json
{"type": "borr", "peer": "192.168.1.1", "afi": "ipv4", "safi": "unicast"}
{"type": "eorr", "peer": "192.168.1.1", "afi": "ipv4", "safi": "unicast"}
```

**Reality:**
- API can *receive* `refresh` events (implemented)
- API cannot *send* `borr`/`eorr` markers (not implemented)

**Impact:** Enhanced Route Refresh (RFC 7313) flow incomplete.

**Recommended Fix Options:**
1. **Option A:** Implement borr/eorr handlers
2. **Option B:** Mark as "Future - Receive only implemented"

---

### 5. Negotiated Capabilities Struct Simplified in Docs

**Location:** `CLAUDE.md`, `docs/architecture/core-design.md`

**Problem:** Documentation shows simplified types:
```go
AddPath         map[Family]bool   // Docs
ExtendedNextHop bool              // Docs
Families        []Family          // Docs
```

**Actual types** at `pkg/bgp/capability/negotiated.go:43-82`:
```go
addPath         map[Family]AddPathMode  // More specific enum
extendedNextHop map[Family]AFI          // Per-family mapping
families        map[Family]bool         // Map, not slice
GracefulRestart *GracefulRestart        // Not mentioned in docs
RouteRefresh    bool                    // Not mentioned in docs
```

**Impact:** Type mismatches when developers code against documentation.

**Recommended Fix Options:**
1. **Option A:** Update docs to show actual types
2. **Option B:** Keep simplified view but add "See code for full struct" note
3. **Option C:** Add separate "Negotiated Capabilities Reference" document

---

## Summary

| Issue | Severity | Effort to Fix |
|-------|----------|---------------|
| Pool architecture mismatch | Major | High (rewrite or add header) |
| announce route removed | Major | Low (update docs) |
| msg-id commands missing | Medium | Medium (implement) or Low (mark future) |
| borr/eorr missing | Medium | Medium (implement) or Low (mark future) |
| Negotiated struct simplified | Medium | Low (update examples) |

---

## Minor Issues (Fixed)

The following minor issues have been fixed in this commit:

1. **ContextID type** - Changed from `uint32` to `uint16` in docs
2. **Type 6 OPERATIONAL** - Removed from messages.md (never implemented)
3. **Watchdog limitation** - Added note that `watchdog set` in `update text` returns error
