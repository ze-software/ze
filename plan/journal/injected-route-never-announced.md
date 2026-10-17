# A route Ze injects never reaches the BGP peer

The producer logs the injection and the BGP session is up, yet the peer's table
never holds the route.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-17 | - | L2TP subscriber route to BGP (scenario 02-ppp-bgp-redistribute-frr) | Nightly 39015310430 and the one before: Ze logs subscriber route inject 10.100.0.2, FRR is Established, the /32 never reaches FRR, Ze logs nothing between | open: reproduce scenario 02 and trace the inject through the redistribute orchestrator to the UPDATE |
