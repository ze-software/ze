# 770: Pre-Computation Optimization Critical Review

## Context

Reviewed PRECOMPUTATION_OPTIMIZATION_REPORT.md (7 proposals, P0-P2) against actual source code
to validate whether each optimization targets real costs or imagined ones.

## Key Finding: Most "Repeated Derivations" Are Trivially Cheap

Five parallel source investigations revealed that the report inflated the cost of many operations:

| Report claim | Reality |
|-------------|---------|
| `peer.Settings().IsEBGP()` is repeated work | Two `uint32` field comparison, inlineable |
| `peer.Settings().PeerKey()` is repeated work | Value-type construction, no heap allocation |
| `applyNextHopMod` allocates `make([]byte, 32)` | Only for IPv6+link-local (rare); IPv4 uses stack arrays |
| `applySendCommunityFilter` is expensive | 1-4 string equality checks, no allocation |
| Cluster ID to bytes is repeated | `var clBuf [4]byte` is stack-allocated |
| Monitor delivery strings are costly | 3 uncontended RLocks at ~50-75ns total |
| Subscription scan is O(n) | 15-25 integer comparisons, faster than a hash lookup |

## What Was Actually Worth Implementing

1. **RIB parse-once** (P0): `ParseAttributes` at 57.7% of RIB allocation was genuinely per-NLRI when it should be per-UPDATE. Real dominant cost.
2. **Peer forwarding facts** (P0): Only `SendContextID()` (mutex per peer) and `PeerKey()` (2-3x redundant construction) justified a snapshot. The 20-field struct proposal was over-engineered; a mid-weight version was correct.
3. **Egress mod precomputation** (P0): Only justified as part of the facts struct (marginal cost on top). Standalone it was not worth the lifecycle complexity.
4. **Batch destination resolution** (P1): Real peer-map re-walk per ID, but batch sizes average <16. Worth doing for route-server competitive parity.
5. **Atomic monitor count** (P1): One-line `atomic.Int64` replacing an RLock. Typed API was over-engineered.
6. **Event raw bytes** (P1): Hex decode on fallback path. Secondary priority.
7. **Static JSON fragments** (P2): `UnmarshalText([]byte("sent"))` was a real per-sent-UPDATE allocation. Array-indexed `String()` replaces switch for `MessageDirection` and `EventKind`.

## Design Principles Validated

- **Profile before optimizing**: without validation, 3 of 7 proposals would have added complexity for no measurable gain.
- **Sum of small decisions**: when competing with BIRD (2x faster), even 20ns operations matter at 200 peers x 100K UPDATEs/sec. The "too small to matter" dismissal was wrong in aggregate.
- **Resolve at lifecycle boundaries**: the correct design pattern is to compute decisions (EBGP/IBGP, next-hop mode, send-community mask) at session setup, not per-UPDATE. This is what C implementations do naturally.
- **Dead API parameters**: `AppendMessage`'s `overrideDir string` had zero non-empty callers across the entire codebase. Removing dead parameters is part of the optimization, not cleanup.

## Traps

- `RouteEntry.Clone` does not copy `AttrFingerprint` or `AttrLen`. Must fix before using Clone for parse-once sharing.
- Dynamic peers mutate `PeerSettings.PeerAS` after OPEN. Any cached facts including `isIBGP`/`isEBGP` must refresh after `resolveDynamicPeerSettings`.
- `replace_all` edits need exact string match. When test files use different variable names (`rawContent`, `hexContent`, `textContent`, `jsonContent`) each variant needs its own pass.

## Files

None recorded.
