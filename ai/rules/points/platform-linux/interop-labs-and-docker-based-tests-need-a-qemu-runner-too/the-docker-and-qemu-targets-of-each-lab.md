---
kind: table
level:
stage:
---
| Lab | Docker target | QEMU target | Netns script |
|-----|---------------|-------------|--------------|
| L2TP (Ze LNS vs xl2tpd) | `ze-deployment-l2tp-ppp-docker-test` | `ze-qemu-l2tp-ppp-test` | `effective-l2tp-ppp.py` |
| PPPoE (Ze client vs accel-ppp) | `ze-deployment-pppoe-accel-docker-test` | `ze-qemu-pppoe-accel-test` | `effective-pppoe-accel.py` |
| VRRP (Ze vs keepalived) | `ze-interop-test INTEROP_SCENARIO=vrrp-*-keepalived` | `ze-qemu-vrrp-keepalived-test` | `effective-vrrp-keepalived.py` |
