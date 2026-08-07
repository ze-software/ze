---
kind: table
level:
stage:
---
| Protocol area | Test infrastructure | Directory | Make target |
|---------------|--------------------|-----------|----|
| BGP (session, capability, NLRI, community, policy) | Docker: FRR, BIRD, GoBGP | `test/interop/scenarios/` | `make ze-interop-test` |
| IPsec (IKEv2, EAP, MOBIKE) | Docker: strongSwan | `test/ipsec-interop/` | `make ze-ipsec-interop-test` |
| L2TP | Docker | `test/l2tp-interop/` | (L2TP runner) |
| PPPoE (Ze as client) | Docker: accel-ppp | `test/pppoe-interop/` | `make ze-deployment-pppoe-accel-docker-test` |
