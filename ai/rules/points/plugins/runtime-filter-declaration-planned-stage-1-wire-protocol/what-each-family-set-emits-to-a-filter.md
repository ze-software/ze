---
kind: table
level: MUST
stage:
---
| Family set | Text protocol emits | Filter plugin requirement |
|------------|---------------------|--------------------------|
| CIDR (ipv4/ipv6 unicast, multicast, mpls-label) | `nlri <family> <op> <prefix>...` with the prefixes inlined | `raw=false` is sufficient |
| Non-CIDR (EVPN, Flowspec, VPN, BGP-LS, MVPN, MUP, RTC, and every future non-CIDR family) | `nlri <family> <op>` (marker only, no prefixes) | `raw=true` REQUIRED for per-NLRI decisions, and the plugin parses `FilterUpdateInput.Raw` itself |
