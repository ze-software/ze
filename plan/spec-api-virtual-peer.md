# Spec: API as Virtual Peer (Revised)

## Task

Add EncodingContext support for API to enable zero-copy route forwarding.
More complex than a single context because:

1. API can send routes with OR without ADD-PATH (need multiple contexts)
2. AS-PATH determines iBGP/eBGP (no AS-PATH = iBGP, first ASN = origin)
3. Routes TO API need normalization (always ASN4), not zero-copy
4. Adj-RIB-In must store encoding context for later zero-copy redistribution

## Current State (verified)

```
make test   - PASS
make lint   - PASS (0 issues)
functional  - 24 passed, 13 failed [6, 7, 8, J, L, N, Q, S, T, U, V, Z, a]
Last commit: 9d144af (docs: add API architecture docs and virtual peer spec)
```

## Problem Statement

### Routes FROM API (API → ZeBGP → Peers)

1. **No sourceCtxID** - Routes from API have no context ID
2. **No wire cache** - Cannot do zero-copy forwarding from API routes
3. **Mixed capabilities** - Same API sends ADD-PATH and non-ADD-PATH routes
4. **AS-PATH semantics** - Need to infer iBGP/eBGP from AS-PATH content

### Routes TO API (Peers → ZeBGP → API)

1. **Mixed encodings** - Peers use different ASN sizes (2-byte vs 4-byte)
2. **Need normalization** - API should receive consistent ASN4 format
3. **No zero-copy** - Always re-encode for API (that's OK)

### Adj-RIB-In

1. **No encoding tracking** - Routes stored without sourceCtxID
2. **Cannot zero-copy** - When redistributing, must always re-encode
3. **Need context preservation** - Store how route was received

## Embedded Protocol Requirements

### Default Rules (ALL tasks)

- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `plan/CLAUDE_CONTINUATION.md` for current state
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof

### From ESSENTIAL_PROTOCOLS.md

- TDD: Write test, see FAIL, implement, see PASS
- Self-review after completion: fix critical/medium issues, report minor
- Understand before implementing: explore codebase first
- For multi-file work, use Task tool agents

### From CODING_STANDARDS.md

- Go 1.21+, idiomatic Go
- Error handling: NEVER ignore errors
- Use context.Context for cancellation
- No panic() for normal error handling

## Design

### Key Design Decisions

1. **Routes carry their context** - sourceCtxID stored in Route struct
2. **API context pool** - Lazy-create contexts for API routes (per family+addpath)
3. **AS-PATH determines iBGP** - Empty = iBGP, first ASN = origin (wire encoding only)
4. **JSON raw mode** - Include context fields for decoder (per-route, per-family)

### Context Lookup (No Virtual Peer Needed)

Routes already store their `sourceCtxID`. To get the encoding context, just look it up:

```go
// Route already has everything we need:
type Route struct {
    wireBytes     []byte
    nlriWireBytes []byte
    sourceCtxID   bgpctx.ContextID  // How it was encoded
}

// Get context when needed:
func getSourceContext(route *rib.Route) *bgpctx.EncodingContext {
    return bgpctx.Registry.Get(route.SourceCtxID())
}
```

**No virtual peer needed** - the Route carries its encoding context ID.
Just look up the context from the Registry when formatting JSON.

### For API-Originated Routes

When API sends a route (not mirroring a peer), use fixed context:

```go
var apiOriginatedContext = &bgpctx.EncodingContext{
    ASN4:    true,   // API always uses 4-byte ASN
    AddPath: nil,    // Derived from route's path-id presence
    IsIBGP:  true,   // API routes are local
}

func (api *API) contextForRoute(route RouteSpec) *bgpctx.EncodingContext {
    ctx := *apiOriginatedContext  // Copy
    if route.PathID != 0 {
        ctx.AddPath = map[bgpctx.Family]bool{route.Family(): true}
    }
    return &ctx
}
```

### Code Path for Routes TO API

Simple - no virtual peer needed:

```go
func sendToAPI(route *rib.Route, family bgpctx.Family, wantRaw bool) {
    if wantRaw {
        // Raw output: use original wire bytes (zero-copy)
        ctx := bgpctx.Registry.Get(route.SourceCtxID())
        json := RawRouteJSON{
            ASN4:       ctx.ASN4,
            AddPath:    ctx.AddPath[family],
            Attributes: base64Encode(route.WireBytes()),
            NLRI:       base64Encode(route.NLRIWireBytes()),
        }
        api.write(json)
    } else {
        // Parsed output: format already-parsed data
        json := ParsedRouteJSON{
            NextHop: route.NextHop(),
            ASPath:  route.ASPath().ASNs(),
            Origin:  route.Origin(),
            // ... already normalized, no context needed
        }
        api.write(json)
    }
}
```

### Routes TO API (JSON with Context)

JSON output has two modes:

1. **Parsed output** - Parsed attributes, normalized (default)
2. **Raw output** - Original wire bytes + context for decoding (zero-copy)

**Parsed JSON (default):**

No context needed - data is already parsed and normalized:

```json
{
  "type": "update",
  "neighbor": {"ip": "192.168.1.1", "asn": 65001},
  "announce": {
    "ipv4 unicast": {
      "10.0.0.0/24": {
        "next-hop": "192.168.1.1",
        "as-path": [65001, 65002],
        "origin": "igp"
      }
    }
  }
}
```

**Raw JSON (when wire bytes requested):**

Context is per-route (ADD-PATH is per-family):

```json
{
  "type": "update",
  "neighbor": {"ip": "192.168.1.1", "asn": 65001},
  "announce": {
    "ipv4 unicast": {
      "10.0.0.0/24": {
        "asn4": true,
        "addpath": false,
        "attributes": "QAEBAEACBgICAP3pAP3q",
        "nlri": "GAoAAA=="
      }
    }
  }
}
```

**Context fields (per-route):**
- `asn4`: true = 4-byte ASN, false = 2-byte ASN in wire
- `addpath`: true = NLRI has 4-byte path-id prefix
- `attributes`: base64-encoded path attributes
- `nlri`: base64-encoded NLRI

```go
func (api *API) sendRoute(route *rib.Route, family bgpctx.Family, includeWire bool) {
    if !includeWire {
        // Parsed output - already normalized, no context needed
        api.writeJSON(parsedRouteMessage(route))
        return
    }

    // Raw output - include context for decoding
    ctx := bgpctx.Registry.Get(route.SourceCtxID())

    msg := RawRouteMessage{
        ASN4:       ctx.ASN4,
        AddPath:    ctx.AddPath[family],
        Attributes: base64.StdEncoding.EncodeToString(route.WireBytes()),
        NLRI:       base64.StdEncoding.EncodeToString(route.NLRIWireBytes()),
    }
    api.writeJSON(msg)
}
```

### Decoding Path Clarification

| Flow | Decoder | Input | Uses |
|------|---------|-------|------|
| Peer → ZeBGP | UPDATE parser | Wire bytes | peer.recvCtx |
| ZeBGP → API (parsed) | None | Already parsed | N/A |
| ZeBGP → API (raw) | API client | Wire bytes | JSON context |
| API → ZeBGP | Text parser | Text command | N/A |

**Key insight:** ZeBGP never decodes for API. Either:
- Sends parsed data (already decoded when received from peer)
- Sends raw bytes + context (API client decodes if needed)

### Context Tracking (No Real Adj-RIB-In)

Routes already store their encoding context via `sourceCtxID` field (from Phase 2 work).
No separate Adj-RIB-In structure needed - the Route itself carries the context.

```go
// Already exists in Route struct:
type Route struct {
    // ...
    wireBytes     []byte
    nlriWireBytes []byte
    sourceCtxID   bgpctx.ContextID  // How route was encoded
}
```

When receiving from peer, route is created with peer's context:

```go
// In UPDATE parsing:
route := rib.NewRouteWithWireCacheFull(
    nlri, nextHop, attrs, asPath,
    attrWireBytes,    // Original wire bytes
    nlriWireBytes,    // Original NLRI bytes
    peer.recvCtxID,   // Peer's receive context
)
```

**Redistribution is API-controlled**, not automatic:

```go
// API command triggers redistribution
// "announce route ... to <peer-selector>"

func handleAnnounceRoute(route *rib.Route, peerSelector string) {
    for _, peer := range matchingPeers(peerSelector) {
        // Zero-copy if contexts match
        attrBytes := route.PackAttributesFor(peer.sendCtxID)
        nlriBytes := route.PackNLRIFor(peer.sendCtxID)
        peer.sendUpdate(attrBytes, nlriBytes)
    }
}
```

### Route Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                        ROUTES FROM API                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  API Client                                                          │
│      │ "announce route 10.0.0.0/24 path-info 42 as-path 65001"      │
│      ▼                                                               │
│  Text Parser                                                         │
│      │ PathID=42, ASPath=[65001]                                    │
│      ▼                                                               │
│  Get API Context (from pool)                                         │
│      │ ASN4=true, AddPath={family: pathID!=0}                       │
│      ▼                                                               │
│  Create Route with Wire Cache                                        │
│      │ wireBytes = encode(attrs, apiCtx)                            │
│      │ sourceCtxID = apiCtxID                                       │
│      ▼                                                               │
│  Forward to Peers (per "announce ... to <selector>")                 │
│      │ PackAttributesFor(peer.sendCtxID)                            │
│      │ → zero-copy if apiCtxID == peer.sendCtxID                    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                        ROUTES TO API                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Peer Route Received                                                 │
│      │ parsed with peer.recvCtx                                     │
│      │ wireBytes preserved                                          │
│      │ sourceCtxID = peer.recvCtxID                                 │
│      ▼                                                               │
│  Send to API (no virtual peer needed)                                │
│      │                                                               │
│      ├─ Parsed output (default)                                     │
│      │      → format route.NextHop(), route.ASPath(), etc.         │
│      │      → no context in JSON                                    │
│      │                                                               │
│      └─ Raw output (wire bytes requested)                           │
│           → ctx = Registry.Get(route.SourceCtxID())                │
│           → JSON: {asn4, addpath, attributes, nlri}                │
│           → zero-copy: use route.WireBytes() directly              │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                    API-CONTROLLED REDISTRIBUTION                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  API Command: "announce route <prefix> to <peer-selector>"          │
│      │                                                               │
│      │ route.sourceCtxID from original source (peer or API)        │
│      ▼                                                               │
│  Forward to Matching Peers                                           │
│      │ PackAttributesFor(peer.sendCtxID)                            │
│      │ PackNLRIFor(peer.sendCtxID)                                  │
│      │                                                               │
│      ├─ sourceCtxID == peer.sendCtxID?                              │
│      │      YES → zero-copy (return cached bytes)                   │
│      │      NO  → re-encode with peer's context                     │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Codebase Context

### Files to Read First

| File | Purpose |
|------|---------|
| `pkg/api/process.go` | Process struct, where to add context |
| `pkg/api/route.go` | Route announcement flow |
| `pkg/api/command.go` | Dispatcher, add new commands |
| `pkg/bgp/context/context.go` | EncodingContext struct |
| `pkg/rib/route.go` | NewRouteWithWireCacheFull |

### Patterns to Follow

- Session state in Process (like `ackEnabled`, `syncEnabled`)
- Command handlers in `pkg/api/` with `handle*` pattern
- Context registration via `bgpctx.Registry.Register()`

## Implementation Phases

### Phase 1: API Context Pool

**Goal:** Create context pool that derives context from route characteristics.

1. Create `APIContextPool` struct
2. Implement `contextFor(route)` - derive from path-id, AS-PATH
3. Lazy context registration with Registry

**Files:**
- `pkg/api/context.go` (new)

**Tests:**
- Route with path-id gets ADD-PATH context
- Route without path-id gets non-ADD-PATH context
- Same characteristics reuse context ID
- Empty AS-PATH → isIBGP=true
- First ASN = localAS → isIBGP=true

### Phase 2: API Route Creation with Wire Cache

**Goal:** Routes from API get wire bytes and sourceCtxID.

1. Modify `announceRouteImpl()` to use context pool
2. Create routes with `NewRouteWithWireCacheFull`
3. Pack attributes using derived context

**Files:**
- `pkg/api/route.go` (modify)
- `pkg/reactor/reactor.go` (modify AnnounceRoute)

**Tests:**
- API routes have wireBytes
- API routes have nlriWireBytes
- API routes have sourceCtxID from pool
- Forward to matching peer uses zero-copy

### Phase 3: Peer Route Reception with Wire Cache

**Goal:** Routes from peers get wire cache for redistribution.

1. Modify UPDATE parsing to preserve wire bytes
2. Create routes with `NewRouteWithWireCacheFull`
3. Set sourceCtxID from peer's recvCtxID

**Files:**
- `pkg/reactor/update.go` (modify)
- `pkg/bgp/message/update.go` (wire preservation)

**Tests:**
- Peer routes have wireBytes from parsing
- Peer routes have sourceCtxID
- Redistribution to same-context peer uses zero-copy

### Phase 4: JSON Output with Context

**Goal:** JSON output includes encoding context for raw decoding.

1. Add `context` field to JSON route messages
2. Include asn4, addpath, source_context_id
3. Optional `wire` field with base64 raw bytes

**Files:**
- `pkg/api/json.go` (modify)
- `pkg/api/types.go` (add ContextInfo struct)

**Tests:**
- JSON includes context fields
- Context matches source peer's capabilities
- Wire bytes base64 encoded when requested

## Verification Checklist

### Phase 1: API Context Pool
- [ ] Tests for contextFor with path-id
- [ ] Tests for contextFor without path-id
- [ ] Tests for isIBGP from AS-PATH
- [ ] Tests for context reuse
- [ ] `make test && make lint` passes

### Phase 2: API Route Wire Cache
- [ ] Tests for API route with wireBytes
- [ ] Tests for API route with sourceCtxID
- [ ] Tests for zero-copy to matching peer
- [ ] `make test && make lint` passes

### Phase 3: Peer Route Wire Cache
- [ ] Tests for peer routes with wireBytes
- [ ] Tests for peer routes with sourceCtxID
- [ ] Tests for zero-copy redistribution
- [ ] `make test && make lint` passes

### Phase 4: JSON with Context
- [ ] Tests for context in JSON output
- [ ] Tests for wire bytes encoding
- [ ] Functional tests still pass
- [ ] `make test && make lint` passes

## Open Questions (Resolved)

1. **Per-connection or global context?**
   - **Decision:** Context pool per API server (shared)
   - **Rationale:** Context is derived from route, not connection. Same route
     characteristics = same context regardless of which connection sent it.

2. **Should API context be pre-configured?**
   - **Decision:** No - derive from route content
   - **Rationale:** Simpler, no commands needed. Route with path-id gets
     ADD-PATH context automatically.

3. **How to determine iBGP for API routes?**
   - **Decision:** Infer from AS-PATH
   - **Rules:**
     - Empty AS-PATH → iBGP (locally originated)
     - First ASN = localAS → iBGP
     - First ASN ≠ localAS → eBGP

4. **Zero-copy TO API?**
   - **Decision:** No - always normalize
   - **Rationale:** API needs consistent format (always ASN4).
     Re-encoding is acceptable for API output.

5. **What about Adj-RIB-In memory?**
   - **Decision:** Store routes with wire cache
   - **Trade-off:** More memory for routes, but enables zero-copy redistribution
   - **Mitigation:** Wire bytes shared via route reference counting

## Dependencies

| Phase | Depends On |
|-------|------------|
| Phase 1 | Route wire cache (done in a317ea9) |
| Phase 2 | Phase 1 |
| Phase 3 | Route wire cache (done), independent of Phase 1-2 |
| Phase 4 | Phase 3 (need sourceCtxID on routes) |

**Note:** Phase 3 (peer routes) can be done in parallel with Phase 1-2 (API routes).

## Metrics to Track

```go
type ZeroCopyMetrics struct {
    Hits      uint64  // Zero-copy path used
    Misses    uint64  // Re-encoding required
    APIRoutes uint64  // Routes from API
    PeerRoutes uint64 // Routes from peers
}
```

---

**Created:** 2025-12-29
**Revised:** 2025-12-29 (multiple contexts, AS-PATH iBGP, Adj-RIB-In)
**Status:** Ready for phased implementation
**Prerequisites:** Phase 3 zero-copy forwarding (completed in a317ea9)
