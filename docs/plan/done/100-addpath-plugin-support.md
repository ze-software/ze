# Spec: ADD-PATH Support in Plugin System

## Task

Enable ADD-PATH (RFC 7911) path-id to be preserved in plugin events so RIB and other plugins can distinguish multiple paths to the same prefix.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/edge-cases/ADDPATH.md` - ADD-PATH design
- [x] `docs/architecture/api/ARCHITECTURE.md` - Plugin event format

### RFC Summaries
- [x] `docs/rfc/rfc7911.md` - ADD-PATH specification

**Key insights:**
- ADD-PATH prepends 4-byte path-id to each NLRI when negotiated
- Negotiation is per AFI/SAFI and directional (send/receive independent)
- Path-id is locally significant, opaque to receiver

## Implementation Status

### Completed

| Component | Status | Implementation |
|-----------|--------|----------------|
| Context propagation | ✅ Done | Via `AttributesWire.SourceContext()` → `EncodingContext` |
| `MPReachWire.NLRIs(hasAddPath)` | ✅ Done | `internal/plugin/mpwire.go` |
| `MPUnreachWire.NLRIs(hasAddPath)` | ✅ Done | `internal/plugin/mpwire.go` |
| `IPv4Reach.NLRIs(hasAddPath)` | ✅ Done | `internal/plugin/mpwire.go` |
| `IPv4Withdraw.NLRIs(hasAddPath)` | ✅ Done | `internal/plugin/mpwire.go` |
| `FamilyNLRI.NLRIs` field | ✅ Done | Replaced `Prefixes []netip.Prefix` |
| `AnnouncedByFamily(ctx)` | ✅ Done | Takes `*EncodingContext`, uses `ctx.AddPathFor()` |
| `WithdrawnByFamily(ctx)` | ✅ Done | Takes `*EncodingContext`, uses `ctx.AddPathFor()` |
| JSON structured format | ✅ Done | `{"prefix":"...","path-id":N}` |
| RIB `routeKey` with path-id | ✅ Done | `family:prefix:path-id` when path-id > 0 |
| RIB `parseNLRIValue` | ✅ Done | Handles both string and object formats |
| RIB `Route.PathID` field | ✅ Done | Stored and included in JSON output |

### Design Note: Context Propagation

Original spec proposed adding `AddPathReceive map[string]bool` to `RawMessage`.
Actual implementation uses existing infrastructure:

```
Reactor                          Plugin System
   │                                  │
   ├─ FromNegotiatedRecv(neg)         │
   │  └─ ctx.AddPath[family] = true   │
   │                                  │
   ├─ Registry.Register(ctx) → ctxID  │
   │                                  │
   ├─ AttributesWire.sourceCtxID ─────┼─► wire.SourceContext()
   │                                  │      │
   │                                  │      ▼
   │                                  │   Registry.Get(ctxID)
   │                                  │      │
   │                                  │      ▼
   │                                  │   ctx.AddPathFor(family)
   │                                  │      │
   │                                  │      ▼
   │                                  │   NLRIs(hasAddPath)
```

No changes to `RawMessage` required.

## Remaining Work (All Complete)

All items below have been implemented.

### 1. ✅ Remove stale limitation comment
**File:** `internal/plugin/rib/rib.go:4-5` now reads:
```go
// RFC 7911: ADD-PATH path-id is included in route keys when present.
// Multiple paths to the same prefix with different path-ids are stored separately.
```

### 2. ✅ Update replayRoutes for path-id
**File:** `internal/plugin/rib/rib.go:377-382` - path-id included when non-zero

### 3. ✅ Fix formatNLRIJSON - use type assertion
**File:** `internal/plugin/text.go:252-258` - uses `prefixer` interface

### 4. ✅ Add unit tests

| Test | File | Status |
|------|------|--------|
| `TestRIBRouteKeyWithPathID` | `internal/plugin/rib/rib_test.go` | ✅ Added |
| `TestRIBParseStructuredJSON` | `internal/plugin/rib/rib_test.go` | ✅ Added |
| `TestReplayRoutesWithPathID` | `internal/plugin/rib/rib_test.go` | ✅ Added |
| `TestFormatNLRIJSONWithPathID` | `internal/plugin/text_test.go` | ✅ Added |
| `TestFormatNLRIJSONNoPathID` | `internal/plugin/text_test.go` | ✅ Added |
| `TestFormatNLRIJSONPathIDMax` | `internal/plugin/text_test.go` | ✅ Added |

## 🧪 TDD Test Plan (Complete)

All tests have been added and pass. See Section 4 above for the complete list.

## Checklist

### 🧪 TDD (Core Implementation)
- [x] Tests written (`TestMPReachWireNLRIs`, etc.)
- [x] Tests FAIL (verified)
- [x] Implementation complete
- [x] Tests PASS (verified)

### Verification (Core)
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Remaining Items (All Complete)
- [x] Remove stale limitation comment from rib.go
- [x] Update replayRoutes for path-id
- [x] Fix formatNLRIJSON to use type assertion
- [x] Add `TestRIBRouteKeyWithPathID`
- [x] Add `TestRIBParseStructuredJSON`
- [x] Add `TestFormatJSONWithPathID` (+ `TestFormatNLRIJSONPathIDMax`)
- [x] Add `TestFormatJSONNoPathID`
- [x] Final verification passes

### Completion
- [x] Spec moved to `docs/plan/done/100-addpath-plugin-support.md`
