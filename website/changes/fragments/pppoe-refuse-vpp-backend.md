---
spec: pppoe-refuse-vpp-backend
topic: PPPoE
feature: commit-time-backend-capability-check
---

## Before

`ze config validate`, an editor commit, and a live reload accepted a
configuration combining `interface backend vpp` with PPPoE access interfaces
even on builds with no VPP PPPoE session data plane. Every subscriber session
then failed at IPCP Opened when the VPP interface backend rejected point-to-point
address installation.

## After

`ze config validate`, editor commits, and SIGHUP reloads now refuse a `pppoe`
access-interface configuration when the running build has no registered PPPoE
session provider for the selected `interface backend`. The refusal names each
affected `/pppoe/interface/<name>` path, the selected backend, the providers
available in the build, the `ze_vpp` build tag needed for VPP PPPoE, and the
exact repair command (`set interface backend netlink`). A refused commit or
reload leaves the running configuration unchanged. Global `pppoe` settings with
no named access interface remain accepted under any backend, and `netlink`
access configurations remain unchanged.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
