# Documentation vs Code Issues Report

**Generated:** 2026-01-12
**Status:** All known issues resolved

---

## Major Issues (P0)

None.

---

## Minor Issues (Fixed)

The following issues have been resolved:

1. **ContextID type** - Changed from `uint32` to `uint16` in docs
2. **Type 6 OPERATIONAL** - Removed from messages.md (never implemented)
3. **Watchdog limitation** - Added note that `watchdog set` in `update text` returns error
4. **borr/eorr RFC 7313** - Full implementation complete (spec 107)
5. **Pool architecture mismatch** - `pool-architecture.md` now has context header clarifying it's for API programs, not the engine
6. **Negotiated struct simplified** - `CLAUDE.md` now shows correct types (`map[Family]Mode`, `map[Family]AFI`, `GracefulRestart`, `RouteRefresh`)
7. **"announce route" syntax** - Removed from all active docs, migrated to `update text` syntax (spec 108)
