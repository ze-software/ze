# 277 — RR EBGP Forward (AS-PATH Prepending in ForwardUpdate)

## Objective

Make the reactor's `ForwardUpdate` RFC 4271-compliant by prepending the local ASN to AS-PATH when forwarding to EBGP peers (different AS), while leaving IBGP forwarding unchanged.

## Decisions

- Chose engine-smart ForwardUpdate over a new `forward-ebgp` command or RR plugin changes: the engine already has per-peer information (`PeerSettings.LocalAS`, `PeerAS`, `IsEBGP()`) and already makes per-peer decisions in `ForwardUpdate`. Adding an IBGP/EBGP branch is a single if-statement, not a new command+interface+dispatch chain. This was the key insight from user feedback that simplified the entire spec.
- Two cached EBGP wire versions per `ReceivedUpdate` (`ebgpWireASN4`, `ebgpWireASN2`): the shift from prepending differs depending on whether the destination peer uses 4-byte or 2-byte ASN encoding. The cached versions avoid re-patching on each forward.
- Reused `attribute.ASPathIterator` for parse and `ASPath.WriteToWithASN4` for encode in `RewriteASPath`: these already handle AS_TRANS (23456 for large ASNs in ASN2 mode), segment splitting at 255, and Extended Length headers. No custom byte-level ASN encoding needed.
- `ebgpLocalAS` sourced from `peer.Settings().LocalAS` (not `a.r.config.LocalAS`): avoids nil-config dependency in tests and better supports per-peer LocalAS configuration.
- Pool buffers from `readBufPool4K`/`readBufPool64K` for EBGP wire, same pool tier as original. Returned via `ReturnReadBuffer` in `evictLocked`, `Delete`, and the safety valve — same lifecycle as `poolBuf`.

## Patterns

- Scan/prepare/distribute pattern in `ForwardUpdate`: scan matching peers to detect EBGP variants, pre-generate patched wires once for each variant, then select the correct wire per peer. Avoids re-patching the same UPDATE for N EBGP peers.
- EBGP patched wire preserves `SourceCtxID`: only AS-PATH content changed, not encoding context (ASN4, ADD-PATH). Zero-copy forwarding still applies for EBGP peers with matching context.

## Gotchas

- The shift from `RewriteASPath` can be positive, zero, or negative: ASN4→ASN2 transcoding can shrink existing ASNs more than the prepend adds, yielding a negative shift. The algorithm handles all three cases correctly.
- The original spec assumed a new `forward-ebgp` command. That approach was rejected as unnecessary complexity — the engine already knows peer types. The discovery happened via user feedback before implementation, not during.
- `parsedUpdate` (lazily parsed `*message.Update`) must be separate per wire version: IBGP and EBGP peers use different payloads. Two vars (`ibgpParsed`, `ebgpParsed`) or reset when switching. Using one shared lazy-parsed struct would give EBGP peers IBGP wire bytes.

## Files

- `internal/component/bgp/wireu/aspath_rewrite.go` — `RewriteASPath` byte-patching function (135 lines)
- `internal/component/bgp/wireu/aspath_rewrite_test.go` — 10 unit tests + fuzz
- `internal/component/bgp/reactor/received_update.go` — EBGP fields + `EBGPWire()` + `getReadBuf()`
- `internal/component/bgp/reactor/received_update_test.go` — 4 EBGP wire tests
- `internal/component/bgp/reactor/recent_cache.go` — EBGP pool buffer returns in evict/delete/safety
- `internal/component/bgp/reactor/reactor.go` — `ForwardUpdate` IBGP/EBGP branching
