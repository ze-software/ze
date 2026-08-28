---
kind: table
level:
stage:
---
| Lab | Docker action | QEMU action | Native producer |
|-----|---------------|-------------|-----------------|
| L2TP (Ze LNS vs xl2tpd) | `./le deployment docker-l2tp-ppp-test` | `./le deployment gokrazy-l2tp-ppp-test` | `internal/le/deployment` |
| PPPoE (Ze client vs accel-ppp) | `./le deployment docker-pppoe-accel-test` | `./le qemu pppoe-accel-test` | `internal/le/qemu/pppoe_accel_linux.go` |
| VRRP (Ze vs keepalived) | `./le integration interop` | `./le qemu vrrp-keepalived-test` | `internal/le/qemu/vrrp_keepalived_linux.go` |
