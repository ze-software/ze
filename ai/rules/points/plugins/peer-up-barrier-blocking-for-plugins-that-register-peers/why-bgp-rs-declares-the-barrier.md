---
kind: note
level:
stage:
---
Reference: `bgp-rs` declares it because `handleState` sets `Up` and captures
`ForwardFrom` in one critical section; an UPDATE taken delivery of before that
lands at or below the peer's cut and belongs to the announce-only Adj-RIB-In
replay, so its withdrawals never reach the peer.
