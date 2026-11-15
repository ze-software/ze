---
spec: bmp-receiver-bind-all-or-none
topic: BMP
feature: bmp-delivery
found-by: review
---

## Before

A BMP receiver could report successful startup or reload after one configured
address failed to bind. Other addresses could remain active, and a failed
replacement could close listeners that were already serving routers.

An invalid explicit plugin hub block could also be ignored at startup, allowing
the daemon to continue.

## After

The receiver binds every added address before closing removed listeners. If a
bind fails, it reports the endpoint and a configuration repair, releases the
new sockets and keeps the previous listener set and session limit. A failed
rollback is reported rather than presented as successful restoration. Existing
accepted sessions continue when their listener is removed. An external plugin
failure continues to follow its declared failure policy.

Ze now refuses invalid explicit plugin hub settings before starting components.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
