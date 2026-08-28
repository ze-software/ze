---
kind: table
level:
stage:
---
| Protocol area | Test infrastructure | Directory | Native action |
|---------------|---------------------|-----------|---------------|
| BGP (session, capability, NLRI, community, policy) | Docker: FRR, BIRD, GoBGP | `test/interop/scenarios/` | `./le integration interop` |
| IPsec (IKEv2, EAP, MOBIKE) | Docker: strongSwan | `test/interop-ipsec/` | `./le integration interop-ipsec` |
| L2TP | Docker | `test/interop-l2tp/` | (L2TP runner) |
| PPPoE (Ze as client) | Docker: accel-ppp | `test/interop-pppoe/` | `./le deployment docker-pppoe-accel-test` |
