# Spec: Convert Static Route Functions to UpdateBuilder

## Status: Partially Obsolete

**See:** `docs/architecture/rib-transition.md` for overall architecture direction.

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
- `.claude/zebgp/POOL_ARCHITECTURE.md` - Pool design

---

**Created:** 2025-12-XX
**Updated:** 2026-01-03 - Marked RIB route conversion as obsolete per Pool+Wire design
