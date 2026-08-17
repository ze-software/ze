---
kind: table
level:
stage:
---
| Protocol area | Test infrastructure | Directory | Make target |
|---------------|--------------------|-----------|----|
| BGP (session, capability, NLRI, community, policy) | Docker: FRR, BIRD, GoBGP | `test/interop/scenarios/` | `make ze-interop-test` |
| IPsec (IKEv2, EAP, MOBIKE) | Docker: strongSwan | `test/interop-ipsec/` | `make ze-interop-ipsec-test` |
| L2TP | Docker | `test/interop-l2tp/` | (L2TP runner) |
| PPPoE (Ze as client) | Docker: accel-ppp | `test/interop-pppoe/` | `make ze-deployment-docker-pppoe-accel-test` |
