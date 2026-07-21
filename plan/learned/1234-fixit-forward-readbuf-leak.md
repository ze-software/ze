# 1234 -- fixit-forward-readbuf-leak

## Context

The UPDATE forwarding / route-reflection / route-server path leaked a shared read-pool
`BufHandle` on every forwarded UPDATE under common local-AS configs (export-filter wire
override + EBGP prepend, dual-AS/second-localAS cache-ineligible keys, RS-client ASN4->ASN2
transcode). Six sites in `reactor_api_forward.go` / `forward_rs.go` borrowed a handle via
`getReadBuf`, kept only the resulting `*wireu.WireUpdate`, and dropped the handle on the
SUCCESS path -- `ReturnReadBuffer` ran only on error branches. The pool never got the buffer
back, eventually failing all reads daemon-wide (`errReadBufferExhaustedPoolAtMaximum`). Goal:
return every borrowed handle exactly once, at the right point, with no use-after-free.

## Decisions

- **Return at cache eviction, NOT at end of `forwardUpdateCore`** (D-1). The forwarded bodies
  ALIAS the buffer into async writes (`forward_body.go`), so returning when the forward
  function exits is a use-after-free. Eviction (`evictLocked`/`Delete`) is the ONE existing
  return point provably after the last write: each dispatched item holds a `RetainN` released
  only post-write, and eviction requires `totalConsumers() <= 0`. It already returns `poolBuf`
  + both `ebgpSlot` handles, so the adopted-handle drain rides alongside. A pending entry
  cannot evict, so no drain races the mid-call aliasing.
- **Adopt onto the cache entry via a mutex-guarded slice** (D-2). `ReceivedUpdate` gains
  `fwdHandleMu` + `fwdHandles []BufHandle` and an `adoptFwdHandle` method; each site calls it
  immediately after successful wire construction. A slice (not two fixed fields) because sites
  1/3 are per-peer (N handles/call) and sites 2/5 are per-key (map) -- only a slice fits all six.
  Rejected threading a refcounted carrier through `fwdItem`/`releaseItem` (5+ release paths +
  shared-wire refcounts, leak-prone); left `forward_pool.go` out of the diff entirely.
- **Dedicated LEAF mutex** (D-3), not `ebgpMu`: `adoptFwdHandle` takes only `fwdHandleMu`;
  `returnFwdHandles` copies+nils under it, releases it, THEN calls `ReturnReadBuffer` OUTSIDE
  the leaf lock -- so `fwdHandleMu -> bufMux` nesting never occurs and the only nesting is the
  pre-existing `cache.mu -> fwdHandleMu`. Nilling under the mutex makes drain idempotent
  (double-evict / evict-then-Delete return nothing twice).
- **Error paths keep their immediate `ReturnReadBuffer`** (D-5): on failure no wire references
  the buffer. Adopt only on success. The adopted handle is the FULL pool handle (not the
  `[:n]` slice), so `ReturnReadBuffer`'s length-based routing round-trips correctly.
- Plain uniform-AS EBGP rides the atomic `EBGPWire` slot and never adopts (AC-4), so its
  slot-return lines are untouched and cannot double-free.

## Consequences

- Pool in-use returns to baseline after eviction for all six sites; no net leak, no
  `errReadBufferExhaustedPoolAtMaximum` under churn. Proven by pool-`Stats()` before==after
  balance tests (K=1,2,3 multi-handle) + `-race`.
- `adoptFwdHandle` is the idiom for "a reactor forward wire that borrows a read-pool buffer":
  adopt onto the cache entry, drained at eviction. Reusable for future forward-path rewrites.
- Two adjacent leaks were RECORDED, not fixed (spec Notes, out of scope): outgoing peer-pool
  buffer on body-build failure; pool-stopped `DispatchOverflow` missing `releaseItem`.

## Gotchas

- The buffer lifetime is the CACHE ENTRY's, not the forward call's -- bodies alias it into async
  writes long after `forwardUpdateCore` returns. Never return a forwarded read buffer at the
  function boundary; hang it on the entry and drain at eviction (after `totalConsumers()<=0`).
- Sites 1/3 (export-filter override) are only reachable through the external per-peer policy
  chain (`runEgressPolicyChain`, `r.api != nil` + a connected plugin returning a raw/text
  override); no in-process Go test drives them without a plugin-bridge harness. Their fix is the
  byte-identical `adoptFwdHandle(dst)` call as tested sites 2/4, and the adopted handle is the
  read-pool buffer, independent of the override wire's own buffer -- so no distinct bug hides
  there. Mechanism is unit-tested (K=1,2,3); end-to-end is the deferred `.ci`/interop.

## Files

- internal/component/bgp/reactor/received_update.go (`fwdHandleMu`+`fwdHandles`, `adoptFwdHandle`, `returnFwdHandles`)
- internal/component/bgp/reactor/recent_cache.go (`evictLocked`/`Delete` drain `returnFwdHandles`)
- internal/component/bgp/reactor/reactor_api_forward.go (adopt sites 1-4; fixed false comment)
- internal/component/bgp/reactor/forward_rs.go (adopt sites 5-6)
- internal/component/bgp/reactor/forward_readbuf_leak_test.go, received_update_test.go (pool-balance + return-once + -race tests)
