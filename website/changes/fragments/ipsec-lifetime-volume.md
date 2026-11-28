---
spec: ipsec-lifetime-volume
topic: IPsec
feature: ipsec-data-model
found-by: rfc-check
---

## Before

IPsec `vpn ipsec esp-group` entries only supported time-based Child SA lifetimes (`lifetime`). High-throughput tunnels could not trigger make-before-break `CREATE_CHILD_SA` rekeys by processed bytes or packets, and `uint64` config leaves above `4294967295` were rejected during config validation.

## After

`vpn ipsec esp-group` now supports `life-bytes` (`0` or `65536..18446744073709551615`, default `0`) and `life-packets` (`0` or `100..18446744073709551615`, default `0`) alongside `lifetime`. When volume limits are configured on Linux XFRM, Ze samples live 64-bit per-SA counters on its 1-second tick, triggers `CREATE_CHILD_SA` rekey on whichever soft limit hits first (with a randomized 5%–10% volume lead), tears down unconditionally on any hard limit, and handles out-of-sync peer counters cleanly. Commits refuse non-zero volume lifetimes on dataplane backends without SA counter readback (such as VPP), and config validation now supports full 64-bit unsigned leaves up to `18446744073709551615`.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
