---
kind: table
level:
stage:
---
| Incorrect | Correct | Why |
|-----------|---------|-----|
| `show traffic-stat` | `show traffic stat` | `traffic` is a real namespace (traffic-cmd owns it, trafficusage augments it); `stat` is a member |
| `show bgp-health` | `show bgp health` | `bgp` is the object namespace |
| `show metrics-query` | `show metrics query` | `metrics` is a real namespace |
| `show l2tp session-history` | `show l2tp session history` | `session` is a real container under `l2tp` |
| `resolve peeringdb as-set` | `resolve peeringdb as-set` (unchanged) | `as-set` is one IRR object; no `as` sibling; keep the hyphen |
