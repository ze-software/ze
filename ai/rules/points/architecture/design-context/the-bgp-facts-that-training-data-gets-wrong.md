---
kind: table
level:
stage:
---
| Fact | Why it matters |
|------|---------------|
| NEXT_HOP is set at the eBGP border router; all IBGP routes share a small set of next-hops (the L3 device originating the prefix or the eBGP peer) | Attribute byte overlap across IBGP peers is high, not low |
| MED, LOCAL_PREF, communities are policy-set by the sender and tend to be identical across many routes from the same peer | Same-peer attribute reuse is very high |
| AS_PATH is identical for all routes learned from the same external source; IBGP does not prepend | Cross-peer attribute overlap within an AS is significant |
| BGP UPDATE packing groups NLRIs with identical attributes into one message, but convergence events and incremental announcements spread them across multiple UPDATEs | Attribute reuse across UPDATEs from a single peer is common |
