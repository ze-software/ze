# Spec: Convert Static Route Functions to UpdateBuilder

## Status: Partially Obsolete

**See:** `docs/architecture/rib-transition.md` for overall architecture direction.

---

## Why This Spec Was Not Fully Completed

**Original scope:** Convert 3 route-building functions to UpdateBuilder pattern.

**What was completed:**
- `buildStaticRouteUpdate` → converted to UpdateBuilder ✅
- `buildGroupedUpdate` → deleted, UpdateBuilder used directly ✅

**What was NOT completed:**
- `buildRIBRouteUpdate` → NOT converted to UpdateBuilder

**Why `buildRIBRouteUpdate` conversion was cancelled:**

The Pool + Wire architecture was finalized AFTER this spec was written. This changed the correct approach for RIB routes:

| Route Type | Has Wire Bytes? | Correct Approach |
|------------|-----------------|------------------|
| Static routes | No (from config) | UpdateBuilder constructs from scratch |
| RIB routes | Yes (received from peers) | Zero-copy from pool |

**The key insight:**

| Step | UpdateBuilder Approach | Pool + Wire Approach |
|------|------------------------|----------------------|
| Receive | Parse wire → structs | Store wire bytes |
| Store | Parsed attributes | pool.Handle → wire bytes |
| Forward | Rebuild wire from structs | pool.Get() → same wire bytes |
| CPU cost | Parse + rebuild | None (zero-copy) |

Using UpdateBuilder for RIB routes would **add** unnecessary work:
1. Parse wire bytes into structured attributes
2. Pass to UpdateBuilder
3. Re-encode back to wire bytes

Pool + Wire skips all of this - just forward the original bytes.

**Conclusion:** This task was not "failed" or "abandoned". It was **superseded** by a better architectural design. The correct solution for RIB routes is zero-copy forwarding, which is tracked in separate specs.

---

## Design Transition Impact

This spec was written before the Pool + Wire lazy parsing design was finalized.

### What's Still Relevant

| Function | Status | Notes |
|----------|--------|-------|
| `buildStaticRouteUpdate` | ✅ DONE | Replaced by `buildStaticRouteUpdateNew` using UpdateBuilder |
| `buildGroupedUpdate` | ✅ DONE | Deleted, UpdateBuilder used directly |

### What's Now Obsolete

| Function | Status | Notes |
|----------|--------|-------|
| `buildRIBRouteUpdate` | ❌ OBSOLETE | Will be replaced by pool-based zero-copy forwarding, NOT UpdateBuilder |

**Why `buildRIBRouteUpdate` won't use UpdateBuilder:**

1. Pool + Wire design stores routes as `pool.Handle` → wire bytes
2. Forwarding uses `pool.Get(handle)` → zero-copy (no parsing)
3. UpdateBuilder assumes parsed attributes → re-packs them
4. This is wasted work when we already have wire bytes

**Replacement path:**

```go
// OLD: buildRIBRouteUpdate (parse → re-pack)
update := buildRIBRouteUpdate(route, localAS, isIBGP, ctx)

// NEW: Zero-copy from pool (no parsing at all)
if route.SourceCtxID() == peer.SendCtxID() {
    attrBytes := pool.Get(route.AttrHandle())  // Zero-copy
    nlriBytes := pool.Get(route.NLRIHandle())  // Zero-copy
} else {
    // Re-encode only when contexts differ
    attrs := NewAttributesWire(pool.Get(route.AttrHandle()), route.SourceCtxID())
    attrBytes, _ := attrs.PackFor(peer.SendCtxID())
}
```

---

## Completed Work (Reference Only)

### Static Route Conversion ✅

`buildStaticRouteUpdateNew` now uses UpdateBuilder pattern:

```go
func buildStaticRouteUpdateNew(route StaticRoute, ...) *message.Update {
    ub := message.NewUpdateBuilder(localAS, isIBGP, ctx)
    if route.IsVPN() {
        return ub.BuildVPN(toStaticRouteVPNParams(route))
    }
    if route.IsLabeledUnicast() {
        return ub.BuildLabeledUnicast(toStaticRouteLabeledUnicastParams(route))
    }
    return ub.BuildUnicast(toStaticRouteUnicastParams(route, sendCtx))
}
```

### Why UpdateBuilder is Correct for Static Routes

Static routes are **locally originated** - they don't have existing wire bytes:
- Parameters come from config
- No pre-existing wire format to preserve
- UpdateBuilder constructs from scratch (correct approach)

### Why UpdateBuilder is Wrong for RIB Routes

RIB routes are **received** - they already have wire bytes:
- Wire bytes stored via pool.Handle
- Forwarding should use existing bytes (zero-copy)
- Re-parsing and re-packing is wasted CPU

---

## Action Items

- [x] Convert `buildStaticRouteUpdate` → `buildStaticRouteUpdateNew` with UpdateBuilder
- [x] Delete `buildGroupedUpdate` (UpdateBuilder used directly)
- [ ] ~~Convert `buildRIBRouteUpdate` to UpdateBuilder~~ **CANCELLED - use pool forwarding instead**

**Next Step:** Implement `spec-pool-handle-migration.md` which will:
1. Store routes with `pool.Handle` instead of parsed attributes
2. Enable zero-copy forwarding path
3. Delete `buildRIBRouteUpdate` entirely

---

## References

- `docs/architecture/rib-transition.md` - Overall architecture direction
- `docs/plan/spec-pool-handle-migration.md` - Pool integration that replaces buildRIBRouteUpdate
- `docs/architecture/POOL_ARCHITECTURE.md` - Pool design

---

**Created:** 2025-12-XX
**Updated:** 2026-01-03 - Marked RIB route conversion as obsolete per Pool+Wire design
