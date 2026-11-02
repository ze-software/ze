# A deny-by-default change blocks Ze's own traffic

A default turns to deny (a firewall default, an XFRM catch-all, a policy
default reject), and a Ze subsystem whose own traffic or input crosses it is
not exempted or adapted in the same work. It fails later with a permission
error that reads as its own defect.

Captured: 2026-11-02 rule protocol/directives/a-deny-by-default-change-adapts-ze-s-own-traffic

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-01 | - | MOBIKE route-source lookup under `vpn ipsec unmatched discard` | The catch-all discard (default since 2026-10-18) made the UDP connect used as a route lookup fail with EPERM, so a moved initiator never announced its new address | fixed: d6ef7093a8 asks the routing table directly; `TestMOBIKERouteSourceUnderTheCatchAllDiscard` |
