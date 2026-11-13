---
spec: route-policy-1-gate-proof-and-forked-sysrib
topic: Plugins
feature: route-policy
found-by: review
---

## Before

An external system RIB lacked the daemon's selected routing state. It could
not provide the internal mode's administrative distances, next-hop resolution,
ECMP and candidate commands.

## After

Internal and external system RIBs now consume the daemon's routes after import
policy and selection. Both use the same distance updates, next-hop resolution,
ECMP and candidate commands. A lost plugin connection withdraws its published
routes before replacement.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
