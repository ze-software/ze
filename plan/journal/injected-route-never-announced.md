# A route Ze injects never reaches the BGP peer

The producer logs the injection and the BGP session is up, yet the peer's table
never holds the route.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-17 | - | L2TP subscriber route to BGP (scenario 02-ppp-bgp-redistribute-frr) | Nightly 39015310430 and the one before: Ze logs subscriber route inject 10.100.0.2, FRR is Established, the /32 never reaches FRR, Ze logs nothing between | fixed: Ze sent the UPDATE (path 65001, nhop 172.29.0.2); FRR 10.3.1 logged `DENIED due to: inbound policy missing` (ebgp-requires-policy). The scenario frr.conf now carries `no bgp ebgp-requires-policy` like every other FRR scenario; 02 red before, pass after in docker |
