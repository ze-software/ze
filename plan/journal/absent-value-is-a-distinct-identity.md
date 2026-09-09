# An absent value is a distinct identity, not a wildcard

Two configuration surfaces name the same real thing, and the runtime groups
them by a key built from several fields. One surface leaves an optional field
unset. The reader expects the omission to mean "any", because that is what an
optional field usually means in configuration; the key builder reads it as a
value, so the two surfaces hash apart and the runtime creates two of whatever
the key identifies.

Nothing goes red. Both objects are well formed and both work. The cost is
duplication the operator never asked for, and it is invisible on every surface
that lists objects rather than the keys behind them.

The tell is an OPTIONAL leaf that participates in an identity. Optionality and
identity pull in opposite directions: one says the field may be left out, the
other says leaving it out is a choice with consequences. Where both hold, the
schema owes the reader that sentence, and a test that exercises the sharing has
to build both keys and compare them rather than assert the objects work.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-09 | bgp-bfd-strict | `api.SessionRequest.Key` (`internal/component/bfd/api/events.go`), fed by `bfdRequestFor` (`internal/component/bgp/reactor/peer_bfd.go`) and by the pinned-session parser (`internal/component/bfd/config.go`) | An operator writing a top-level `bfd { single-hop-session 10.0.0.2 { ... } }` AND a BGP peer to 10.0.0.2 reasonably expects ONE BFD session. They silently get TWO, each sending its own Control packets at its own rate, whenever the two blocks disagree about the local address: `Key` includes `Local`, `bfdRequestFor` fills it from the peer's `connection local ip`, and the pinned entry's `local` leaf is optional. An omitted leaf is the zero `netip.Addr`, which is a distinct key rather than a match. RFC 5882 Section 4.4 asks the opposite -- "If multiple control protocols wish to establish BFD sessions with the same remote system for the same data protocol, all MUST share a single BFD session" -- and `TestBFDSharedSessionSameKey` proves ze shares correctly for EQUAL keys, so the gap is entirely in how the two configuration surfaces reach a key. The leaf's own `ze:help` already says "The address is part of the session key", so the schema states it and nothing enforces or reconciles it. Found while writing an interop scenario that claimed to exercise a shared session and quietly created two, which is how it surfaced: the test passed for the wrong reason | FIXED in the same spec, on the owner's answer of 2026-09-09 ("make the two builders agree on the key"). `api.SessionRequest.Canonical` (`internal/component/bfd/api/session_identity.go`) derives the interface and the local address a client left out, from the link the peer is on, and refuses to derive when the link is ambiguous. Every client passes through it: `pluginService.EnsureSession` for the protocol clients and `applyPinned` for the pinned entries (`internal/component/bfd/bfd.go`). Three tests drive the three real builders onto one key, since none of them is exported: `TestPinnedSessionReachesTheSharedKey` (`internal/component/bfd`), `TestStrictPeerRequestReachesTheSharedKey` (`internal/component/bgp/reactor`) and `TestOSPFNeighborRequestReachesTheSharedKey` (`internal/plugins/ospf`). The class this row names still holds: the row stays because the pattern (an optional leaf that participates in an identity) recurs, and because the ambiguous case is still resolved by giving the under-specified request its own session |
