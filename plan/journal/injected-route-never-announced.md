# A route Ze injects never reaches the BGP peer

The producer logs the injection and the BGP session is up, yet the peer's table
never holds the route.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | route-policy-0-umbrella (found, not caused) | BGP egress of a filter-modified route (`test/plugin` modify suites) | At HEAD 7c45216f1e (macOS) prefix-filter-modify-mp, remove-private-as-export and modify-oversize-suppress fail with their original configs: the filter logs its change but `dest peer was never sent the re-advertised route`. Six more modify tests match; deterministic, not the earlier single flakes | open: bisect to the commit that stopped modified routes reaching egress and fix that producer |
| 2026-10-30 | route-policy-0-umbrella (found, not caused) | BGP route after a config reload (`test/reload`) | At 7c45216f1e (macOS, original configs) bgp-policy-final-reload sees no routes at all (`echoed wire routes=map[]`), and bgp-filter-irr-added-live and bgp-filter-irr-first-add-live never send the route config2 adds after SIGHUP although the IRR refresh logs success | open: likely the same egress loss as the row above; bisect with it and fix that producer |
| 2026-10-17 | - | L2TP subscriber route to BGP (scenario 02-ppp-bgp-redistribute-frr) | Nightly 39015310430 and the one before: Ze logs subscriber route inject 10.100.0.2, FRR is Established, the /32 never reaches FRR, Ze logs nothing between | fixed: Ze sent the UPDATE (path 65001, nhop 172.29.0.2); FRR 10.3.1 logged `DENIED due to: inbound policy missing` (ebgp-requires-policy). The scenario frr.conf now carries `no bgp ebgp-requires-policy` like every other FRR scenario; 02 red before, pass after in docker |
