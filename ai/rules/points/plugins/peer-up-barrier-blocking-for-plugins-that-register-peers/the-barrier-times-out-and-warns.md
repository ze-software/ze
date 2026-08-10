---
kind: directive
level: MUST NOT
stage:
---
**Bounded, and it says so.** A plugin that never acknowledges delays the marker
to `peerUpBarrierTimeout` (2 s), which releases it with a WARN naming the peer
and the shortfall. Establishment MUST NOT be blocked.
