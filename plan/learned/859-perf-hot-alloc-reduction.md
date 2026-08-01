# 859: Hot-Path Allocation Reduction (perf-hot-0 through perf-hot-5)

## Context

Second optimization round after the 771 campaign (91ms->71ms). Profiling with
PPROF=1 on 100K routes identified 17.9M objects / 1.09 GB from string-keyed maps
on the forwarding hot path. Each plugin stage independently converted wire NLRI
bytes to prefix strings for map keys, then discarded the wire bytes. The same
prefix was stringified 4 times per UPDATE across adj-rib-in, rib, and route-server.

## What Changed

All 5 child specs implemented in one commit (f195801fd):

| Spec | Target | Change |
|------|--------|--------|
| perf-hot-1 | rib ribOut string keys | ribOutKey{netip.Prefix, uint32} replaces wirePrefixToString |
| perf-hot-2 | adj-rib-in bgp.Event round-trip | Structured path walks wire bytes directly, bypasses wireNLRIsToAny boxing |
| perf-hot-3 | adj-rib-in RouteKey/pendingKey concat | compactRouteKey/compactPendingKey struct keys replace string concat |
| perf-hot-4 | RS withdrawal string keys | withdrawalKey{family.Family, netip.Prefix} replaces prefix.String() concat |
| perf-hot-5 | rib changesByFamily per-UPDATE map | Single-family fast path + sync.Pool for affected slice |

Shared helper: `nlri.WirePrefixToKey` for zero-alloc wire-to-netip.Prefix conversion.

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Value-type struct keys over interned strings | netip.Prefix is 24 bytes, value-typed, no heap allocation. String interning still allocates on first sighting and adds lifecycle complexity |
| All 5 specs in one commit over phased rollout | Changes touch different files with no overlap. Single commit simplifies bisection (one commit to test, not five) |
| Bypass bgp.Event for structured path (hot-2) | The structured handler built a bgp.Event with wireNLRIsToAny then immediately unboxed it in handleReceived. Direct wire-byte walking eliminates the intermediary without affecting the text/JSON event path |
| sync.Pool for affected slice (hot-5) | changesByFamily allocated a map per UPDATE; single-family UPDATEs (the common case) need only a slice. Pool amortizes the allocation after warmup |

## Gotchas

- Wire-byte map keys must not alias pool buffers that may be recycled. netip.Prefix
  is value-typed (safe), but raw []byte keys from wire buffers would create dangling
  references after pool return. Using netip.Prefix as the key type avoids this entirely.
- The adj-rib-in text/JSON event path (handleReceived) still uses string-based keys.
  Only the structured path was optimized. This is correct: text events are cold-path
  (CLI monitor, external plugins) and must preserve the existing string-based contract.
- Non-unicast families (VPN, EVPN) still use string-based NLRI representations because
  their wire format is complex and not representable as netip.Prefix. These are rare in
  the benchmark workload and not worth optimizing without separate profiling evidence.

## Results

With the RPKI validation gate fix (19464ca9a) and throughput stddev correction
(ec81f5005), 100K IPv4/unicast on 4 GB VM (2026-06-05):

| DUT | Convergence | Throughput |
|-----|-------------|------------|
| ze | 62ms +/- 10ms | 1,612,903 r/s |
| bird | 65ms +/- 0ms | 1,538,461 r/s |

Ze now matches or slightly beats BIRD on convergence in this test configuration.
The full journey: 91ms (pre-771) -> 71ms (post-771) -> 62ms (post-hot-path).

## Files

None recorded.
