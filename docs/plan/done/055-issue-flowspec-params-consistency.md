# Issue: FlowSpecParams Type Inconsistency

**Status:** Closed - By Design
**Priority:** N/A
**Created:** 2026-01-01
**Closed:** 2026-01-01

## Summary

`FlowSpecParams` uses `CommunityBytes []byte` while `UnicastParams`/`VPNParams` use `Communities []uint32`. This is intentional.

## Architecture Context

ZeBGP has two paths for UPDATE messages:

### Build Path (Local Origination)
```
Config → FlowSpecRoute{CommunityBytes} → FlowSpecParams → BuildFlowSpec() → Update
```
- For config-originated routes
- FlowSpec pre-packs communities at config load time
- Passes through without repacking at build time

### Forward Path (Route Reflection)
```
Receive UPDATE → Route{wireBytes, sourceCtxID} → zero-copy forward
```
- For received routes
- Uses `Route.wireBytes` cache
- Zero-copy when `sourceCtxID == destCtxID`

## Why It Stays

| Factor | Reality |
|--------|---------|
| FlowSpec volume | ~10-100 rules per config |
| Build frequency | Once at session establishment |
| Forward path | Uses `Route.wireBytes` (separate system) |
| Unify cost | 4+ files, breaking change |
| Unify benefit | Negligible (low-volume path) |

The optimization matters for the Forward path (millions of routes). That's already handled by `Route.wireBytes`. The Build path optimization for FlowSpec is marginal but harmless.

## Resolution

Added documentation comment to `FlowSpecParams` explaining the design choice. No code changes.

## Files Modified

- `internal/bgp/message/update_build.go`: Documentation comment added
