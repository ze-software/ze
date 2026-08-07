---
kind: directive
level: MUST
stage:
---
**What stays UNGATED on purpose: shared contract leaves.** When other features consume a nil-able seam or value types a gated feature exposes, that contract package stays OFF the manifest and always-on, and only the machinery gates: `bfd/api` (the `SetService`/`GetService` seam BGP/OSPF/static nil-check, plus `bfd/packet` for its State/Diag re-exports) and `ike/dataplane` (the XFRM programming seam OSPF's RFC 4552 authentication also uses). Every consumer of such a seam MUST already handle nil/absent: verify each call site before choosing this shape, and make the absent-build nm needles NAME the gated sub-packages instead of using the subtree prefix.
