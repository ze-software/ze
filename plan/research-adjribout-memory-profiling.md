# Spec: Adj-RIB-Out Memory Profiling

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/pool-architecture.md` - pool system, Memory Profile section
4. `docs/architecture/plugin/rib-storage-design.md` - plugin RIB storage
5. `internal/component/bgp/rib/outgoing.go` - engine OutgoingRIB
6. `internal/component/bgp/plugins/rib/rib.go` - plugin ribOut (handleSent)

## Task

Reduce per-peer route memory in the engine OutgoingRIB and plugin ribOut
by replacing per-peer full-copy storage with shared pool handles.

Measured current cost: 863 bytes/route/peer (478 engine + 385 plugin).
Target: ~4 bytes/route/peer + shared pool.
At 1M routes, 10 peers: 8.4 GB current, ~440 MB target (95% reduction).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/pool-architecture.md` - Memory Profile section (measured data)
  -> Constraint: pool handle pattern (Intern/Get/Release) is the proven dedup mechanism
- [ ] `docs/architecture/plugin/rib-storage-design.md` - how plugin RIB already uses pools
  -> Decision: RouteEntry uses Bundle + ASPath handles (32 B struct, 69 B measured)
- [ ] `docs/architecture/rib-transition.md` - engine vs plugin RIB boundary
  -> Constraint: engine passes wire bytes, plugins own storage
- [ ] `plan/learned/608-perf-1-rib-cache-layout.md` - BundlePool design
  -> Decision: comparable struct as map key for fixed-size dedup

**Key insights:**
- Plugin RIB already solved this problem: 69 B/route via pool handles vs 478-741 B in the other layers
- The engine OutgoingRIB and plugin ribOut both duplicate full route data per peer
- The same pool-handle pattern can be extended to both outgoing layers
- OutgoingRIB routes exist for replay only; parsed attributes are never queried after storage

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/rib/route.go` - 160 B Route struct with parsed attrs + wire caches
- [ ] `internal/component/bgp/rib/outgoing.go` - OutgoingRIB with map[Family]map[string]*Route
- [ ] `internal/component/bgp/route.go` - 288 B bgp.Route struct with string-heavy parsed fields
- [ ] `internal/component/bgp/plugins/rib/rib.go:697-790` - handleSent stores per-peer *Route
- [ ] `internal/component/bgp/plugins/rib/storage/routeentry.go` - 32 B RouteEntry (reference)

**Behavior to preserve:**
- OutgoingRIB.GetSentRoutes() returns routes for replay on session re-establishment
- OutgoingRIB transaction semantics (Begin/Commit/Rollback)
- Plugin ribOut supports `show rib out` with per-peer per-family route display
- Plugin ribOut supports GR/LLGR stale propagation via StaleLevel
- Plugin ribOut supports `rib clear out` per peer, per family

**Behavior to change:**
- Internal storage representation (map values become handles instead of full structs)
- Memory footprint per route per peer (863 B -> ~4 B + shared pool)

## Measured Results (2026-05-25)

All measurements at 100K IPv4/32 routes, Apple M4 Max, Go 1.26.
Method: `runtime.MemStats.TotalAlloc` delta (cumulative, GC-immune).

### Per-Route Memory by Layer

| Layer | Struct | Measured | Allocs | Per-peer? |
|-------|--------|----------|--------|-----------|
| Plugin RIB (RouteEntry + BART) | 32 B | **69 B** | 1.0 | No |
| Engine OutgoingRIB (rib.Route) | 160 B | **478 B** | 10.0 | Yes |
| Plugin ribOut (bgp.Route typical) | 288 B | **385 B** | 6.1 | Yes |
| Plugin ribOut (bgp.Route full) | 288 B | **741 B** | 10.1 | Yes |

### Scaling

| Scenario | Plugin RIB | Engine Out | Plugin ribOut | Total |
|----------|-----------|------------|---------------|-------|
| 100K routes, 1 peer | 7 MB | 46 MB | 37 MB | 90 MB |
| 100K routes, 10 peers | 7 MB | 456 MB | 367 MB | 830 MB |
| 1M routes, 10 peers | 66 MB | 4.6 GB | 3.7 GB | 8.4 GB |

### Where the Bytes Go

**Engine rib.Route (478 B):**
Route struct(160) + NLRI(16) + attrs(48) + ASPath(80) + wireBytes(40) +
nlriWireBytes(4) + indexCache(15) + map key(20) + map bucket(50) = ~450 B

**Plugin bgp.Route (385 B typical):**
Route struct(288) + Prefix string(15) + NextHop string(12) + ASPath slice(36) +
Origin ptr(9) + MED/LP ptrs(16) + Communities(32) + RawAttrs(44) +
SourcePeer(8) + map key(15) = ~385 B

## Data Flow (MANDATORY)

### Entry Point
- Engine receives UPDATE wire bytes from peer
- Engine forwards to destination peer(s) and emits "sent" event to plugins

### Current Transformation Path
1. Engine parses UPDATE, creates `rib.Route` with parsed attrs + wire cache
2. Engine calls `OutgoingRIB.MarkSent(route)` -> stores `*Route` in `sent` map per peer
3. Plugin receives "sent" event, `handleSent()` creates `bgp.Route` from parsed fields
4. Plugin stores `*bgp.Route` in `ribOut[peer][family][key]` map

### Proposed Transformation Path
1. Engine parses UPDATE, creates `rib.Route` (unchanged for forwarding)
2. Engine calls `OutgoingRIB.MarkSent(route)` -> interns wire bytes, stores handle per peer
3. Plugin receives "sent" event with raw-attributes field
4. Plugin interns wire bytes from event, stores handle in `ribOut[peer][family][key]`
5. On replay or `show rib out`: reconstruct Route from pool.Get(handle) -> parse on demand

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> Plugin | JSON event with raw-attributes | [ ] |
| Pool -> OutgoingRIB | Handle stored instead of *Route | [ ] |
| Pool -> Plugin ribOut | Handle stored instead of *bgp.Route | [ ] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 100K routes stored in OutgoingRIB | Measured bytes/route < 100 (vs 478 current) |
| AC-2 | 100K routes stored in plugin ribOut | Measured bytes/route < 100 (vs 385 current) |
| AC-3 | Session re-establishes | GetSentRoutes() reconstructs full routes from handles |
| AC-4 | `show rib out` command | Displays same output as current (parsed on demand) |
| AC-5 | `rib clear out !peer` | Releases handles, frees pool entries with refcount 0 |
| AC-6 | GR stale propagation | StaleLevel preserved through handle storage |
| AC-7 | OutgoingRIB transaction | Begin/Commit/Rollback work with handles |
| AC-8 | 10 peers, same route set | Single pool copy, 10 handles (not 10 full copies) |

## Design

### Phase 1: Engine OutgoingRIB (primary target, 478 B -> ~70 B)

Replace `map[Family]map[string]*Route` with `map[Family]map[string]OutgoingEntry`:

```go
type OutgoingEntry struct {
    AttrHandle    attrpool.Handle  // 4 B -> shared wire bytes in pool
    NLRIBytes     [5]byte          // 5 B inline (IPv4/32 worst case)
    NLRILen       uint8            // 1 B
    NextHop       netip.Addr       // 24 B inline (no heap alloc)
    SourceCtxID   bgpctx.ContextID // 2 B
}
// ~36 bytes stored by value (no pointer, no heap alloc for the entry itself)
// + map overhead (~50 B)
// Total: ~86 B vs 478 B current
```

Pool stores the wire attribute bytes (deduplicated across peers).
On replay: `pool.Get(handle)` retrieves wire bytes, reconstruct Route.

**Why inline NLRI:** IPv4 prefixes are 1-5 bytes. Storing them inline
avoids a separate allocation per route. IPv6/VPN routes need a different
approach (pool handle or `[]byte`), but IPv4 is the 99% case for full tables.

### Phase 2: Plugin ribOut (secondary target, 385 B -> ~12 B)

Replace `map[string]*Route` with `map[string]RibOutEntry`:

```go
type RibOutEntry struct {
    AttrHandle  attrpool.Handle  // 4 B -> shared wire bytes
    MsgID       uint64           // 8 B (needed for dedup)
    StaleLevel  uint8            // 1 B (GR/LLGR)
}
// ~16 B stored by value (with padding)
// Prefix is the map key (already exists)
// NextHop, communities, etc reconstructed from pool.Get(handle)
```

On `show rib out`: parse wire bytes from pool on demand.
On replay: reconstruct `FormatAnnounceCommand` from wire bytes.

### Phase 3: Per-Peer Route Limits (safety)

Add `MaxRoutes` to both layers. Log warning at 80%, reject at 100%.

## Files to Modify

- `internal/component/bgp/rib/outgoing.go` - OutgoingEntry type, MarkSent/GetSentRoutes
- `internal/component/bgp/rib/outgoing_test.go` - update for new storage
- `internal/component/bgp/plugins/rib/rib.go` - RibOutEntry type, handleSent
- `internal/component/bgp/plugins/rib/rib_commands.go` - reconstruct for show
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - iterate with reconstruction
- `docs/architecture/pool-architecture.md` - update Memory Profile section

## Out of Scope

- Plugin RIB RouteEntry optimization (already done)
- Wire encoding allocations (covered by learned 603, 604)
- Hot-path CPU optimization (covered by learned 771)
- Loc-RIB best-path memory (separate concern)
- IPv6/VPN NLRI inline storage (follow-up if needed)

---

## Memory Formulas (Measured, Pre-Optimization)

```
engine_outgoing_mem  = routes * peers * 478 bytes
plugin_ribout_mem    = routes * peers * (385..741) bytes
plugin_ribin_mem     = routes * 69 bytes
total_rib_mem        = engine + plugin_ribout + plugin_ribin
```

---

**Created:** 2026-01-01
**Revised:** 2026-05-25 (research complete, promoted to design)
