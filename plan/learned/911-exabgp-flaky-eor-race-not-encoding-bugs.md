# exabgp-flaky-eor-race-not-encoding-bugs

## Context

`known-failures.md` listed `ze-exabgp-test` as "10/40 FAIL -- product bugs:
ze sends wrong UPDATE messages (e.g. MP_UNREACH instead of MP_REACH)". The
brief was to fix ten distinct encoding bugs. The golden `.ci` wire data comes
from real ExaBGP, so byte-exact mismatches against authoritative data were
suspicious from the start.

## What was actually true

Nine of the ten were ONE non-deterministic concurrency race; only one was a
real encoding bug.

- Proof of flakiness: two identical full-suite runs failed disjoint sets
  (run A {20,32,35,39,40} vs run B {1,14,18,25,31,33,35,36,40}). conf-addpath
  passed 5/5 alone but failed under parallel load. A real encoding bug fails
  every run, identically.
- The "MP_UNREACH instead of MP_REACH" symptom was actually a bare End-of-RIB
  marker (`90 0F 00 03 00 01 80`) arriving as the first message, before any
  route NLRI. The mock only accepts EoR once all route options have matched.

## Root causes

1. **EoR race.** On establishment two producers send EoR: reactor
   `sendInitialRoutes` (always, per negotiated family) and the bgp-rs plugin's
   `replayForPeer` goroutine. When bgp-adj-rib-in is absent (the exabgp wrapper
   loads a minimal plugin set) the RS replay fast-fails and sends EoR
   immediately. Announce/withdraw honor `peer.ShouldQueue()` and go through the
   opQueue; `AnnounceEOR` was the one route-op that wrote directly via
   `peer.SendUpdate`, so the plugin EoR raced ahead of the still-queued routes.
   Earlier partial fix `af60758d0` held the write lock only across the
   family-specific route phase, not the static-route phase. Fix `99c943404`:
   `AnnounceEOR` skips peers in initial sync -- the reactor owns the per-family
   EoR -- which also removes the duplicate EoR.
2. **srv6-mup (the only real encoding bug).** `routeattr_prefixsid.go` wrote the
   SRv6 SID Structure Sub-Sub-TLV header as 4 bytes (`0,1,0,len`) instead of the
   RFC 9252 3 bytes (Type 1 octet + Length 2 octets = `1,0,len`). The spurious
   leading reserved byte inflated the inner sub-TLV length by 1. The decode side
   (`srv6sid.go parseSIDStructure`) was already RFC-correct, so ze could not
   round-trip its own SRv6 SID structure.

## Lesson

- A batch of failures sharing one subsystem and labeled "product bugs" is a
  red flag for a single shared cause (often a race), not N independent bugs.
  Re-run the suite 2-3 times and diff the failure SETS before triaging. A set
  that changes run-to-run is a race; trust that over any inherited label.
- Keep route-ops symmetric: if announces/withdraws are queued during initial
  sync for ordering, EoR (and any other establishment-time wire write) must use
  the same gate, or it jumps the queue.
- When external golden data (captured from a reference implementation) says the
  code is wrong, the code is wrong. Find the off-by-one; never touch the
  fixture (`ai/rules/testing.md`).

## Left open (separate, pre-existing)

- `ze-test bgp encode 38 paths-limit`: was a broken test fixture, NOT a ze bug.
  ze correctly sends an ADD-PATH path-id (negotiated via the mirroring ze-peer);
  the expected hex in `test/encode/paths-limit.ci` (`56f48c85f`) omitted it. Same
  trap as the exabgp suite, inverted: here the inherited "failure" was a stale
  fixture, not the code. Fixture corrected (user-authorized); encode suite 53/53.
- `internal/analyze/inject.go:64` goconst lint (`3215ece93`) -- still open,
  unrelated to BGP.

## Files

None recorded.
