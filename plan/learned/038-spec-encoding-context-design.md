# 038 — Encoding Context Design

## Objective

Design `EncodingContext` and `ContextID` to unify capability-dependent BGP encoding for both receiving and sending, enabling zero-copy route forwarding when source and destination peers share identical encoding capabilities.

## Decisions

- Source context and destination context are the SAME struct — same structure, different use (how a peer encodes what it sends vs. how we encode what we send to it)
- Each peer has TWO contexts (`recvCtx`/`sendCtx`) because ADD-PATH is the only asymmetric capability: a peer may send path IDs without receiving them back
- `ContextID` is uint16 (2 bytes) instead of pointer (8 bytes): saves 6 MB at 1M routes
- Global registry deduplicates identical contexts by hash: 100 RR clients with same caps share one context and one ID
- Four-phase migration: add types → registry + peer integration → SourceCtxID in route storage → zero-copy forwarding; phases 3-4 wait until RIB is implemented

## Patterns

- Zero-copy forwarding check is a single integer comparison: `route.SourceCtxID == peer.sendCtxID` — no struct field iteration at forward time
- Contexts are immutable after session establishment; created once at OPEN negotiation
- `EncodingContext.Hash()` must be deterministic across all fields including map entries

## Gotchas

- ADD-PATH is the only asymmetric BGP capability: ASN4 and Extended Next-Hop are symmetric, but ADD-PATH receive/send modes are negotiated independently per RFC 7911
- Zero-copy requires storing original wire bytes at receive time — if RIB normalises attributes on ingress, zero-copy is broken even when context IDs match
- Design-only spec — no code was written; implementation decisions (package location, registry scope) were deferred

## Files

Design spec only — planned files: `internal/bgp/context/context.go`, `internal/bgp/context/registry.go`
